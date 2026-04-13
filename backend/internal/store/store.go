package store

import (
	"database/sql"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

// Store wraps the SQLite database.
type Store struct {
	db *sql.DB
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

	return &Store{db: db}, nil
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

		CREATE TABLE IF NOT EXISTS layouts (
			container_id TEXT PRIMARY KEY,
			page         INTEGER NOT NULL DEFAULT 0,
			position     INTEGER NOT NULL DEFAULT 0
		);
	`)
	return err
}
