package store

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Store wraps the SQLite database.
type Store struct {
	db      *sql.DB
	dataDir string
}

// New opens (or creates) the SQLite database and runs migrations.
func New(dataDir string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}

	dbPath := filepath.Join(dataDir, "agent_hive.db")
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}

	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}

	s := &Store{db: db, dataDir: dataDir}

	if err := s.migrateTerminals(); err != nil {
		log.Printf("warning: terminal migration failed: %v", err)
	}

	return s, nil
}

// Close closes the database.
func (s *Store) Close() error {
	return s.db.Close()
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS todos (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			container  TEXT    NOT NULL,
			content    TEXT    NOT NULL DEFAULT '',
			done       INTEGER NOT NULL DEFAULT 0,
			sort_order INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT (datetime('now'))
		);
		CREATE INDEX IF NOT EXISTS idx_todos_container ON todos(container);

		CREATE TABLE IF NOT EXISTS containers (
			id         TEXT PRIMARY KEY,
			name       TEXT NOT NULL DEFAULT 'New Project',
			created_at DATETIME NOT NULL DEFAULT (datetime('now'))
		);

		CREATE TABLE IF NOT EXISTS layouts (
			container_id TEXT PRIMARY KEY,
			page         INTEGER NOT NULL DEFAULT 0,
			position     INTEGER NOT NULL DEFAULT 0
		);

		CREATE TABLE IF NOT EXISTS mobile_layouts (
			container_id TEXT PRIMARY KEY,
			sort_order   INTEGER NOT NULL DEFAULT 0
		);

		CREATE TABLE IF NOT EXISTS terminals (
			id           TEXT PRIMARY KEY,
			container_id TEXT NOT NULL,
			name         TEXT NOT NULL DEFAULT 'Terminal 1',
			is_default   INTEGER NOT NULL DEFAULT 0,
			sort_order   INTEGER NOT NULL DEFAULT 0,
			created_at   DATETIME NOT NULL DEFAULT (datetime('now'))
		);
		CREATE INDEX IF NOT EXISTS idx_terminals_container ON terminals(container_id);
	`)
	return err
}

// migrateTerminals migrates old single-terminal containers to the multi-terminal model.
// If the terminals table is empty but containers exist, it creates a default terminal
// for each container and moves log files from {cid}.log to {cid}/{tid}.log.
func (s *Store) migrateTerminals() error {
	var termCount int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM terminals`).Scan(&termCount); err != nil {
		return err
	}
	if termCount > 0 {
		return nil // already migrated
	}

	containers, err := s.ListContainerMeta()
	if err != nil {
		return err
	}
	if len(containers) == 0 {
		return nil // fresh install
	}

	termDir := filepath.Join(s.dataDir, "terminals")

	for _, c := range containers {
		tid := fmt.Sprintf("t-%s-0", c.ID)
		now := time.Now()

		_, err := s.db.Exec(
			`INSERT INTO terminals (id, container_id, name, is_default, sort_order, created_at)
			 VALUES (?, ?, ?, 1, 0, ?)`,
			tid, c.ID, "Terminal 1", now,
		)
		if err != nil {
			log.Printf("warning: failed to create default terminal for %s: %v", c.ID, err)
			continue
		}

		// Migrate log file: {cid}.log → {cid}/{tid}.log
		oldPath := filepath.Join(termDir, c.ID+".log")
		newDir := filepath.Join(termDir, c.ID)
		newPath := filepath.Join(newDir, tid+".log")

		if _, err := os.Stat(oldPath); err == nil {
			if err := os.MkdirAll(newDir, 0755); err != nil {
				log.Printf("warning: failed to create dir %s: %v", newDir, err)
				continue
			}
			if err := os.Rename(oldPath, newPath); err != nil {
				log.Printf("warning: failed to migrate log %s → %s: %v", oldPath, newPath, err)
			}
		}
	}

	return nil
}
