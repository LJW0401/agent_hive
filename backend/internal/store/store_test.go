package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewCreatesTerminalsTable(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer s.Close()

	// Verify terminals table exists by inserting a row
	_, err = s.db.Exec(
		`INSERT INTO terminals (id, container_id, name, is_default, sort_order) VALUES ('t-1', 'c-1', 'Terminal 1', 1, 0)`,
	)
	if err != nil {
		t.Fatalf("terminals table not created: %v", err)
	}

	var count int
	s.db.QueryRow(`SELECT COUNT(*) FROM terminals`).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 terminal, got %d", count)
	}
}

func TestMigrateCreatesDefaultTerminals(t *testing.T) {
	dir := t.TempDir()
	termDir := filepath.Join(dir, "terminals")
	os.MkdirAll(termDir, 0755)

	// Create a legacy log file
	os.WriteFile(filepath.Join(termDir, "c-1.log"), []byte("hello"), 0644)

	// First: create DB without terminals table to simulate old schema
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer s.Close()

	// Insert a container record (simulating old data)
	_, err = s.db.Exec(`INSERT INTO containers (id, name) VALUES ('c-1', 'Test')`)
	if err != nil {
		t.Fatalf("insert container failed: %v", err)
	}
	s.Close()

	// Re-open: migrateTerminals should detect container without terminal and create one
	// But since we already have the terminals table (created by migrate()), and it's empty,
	// and we have containers, migrateTerminals should run.
	// However, migrateTerminals already ran on first New() — at that point containers table was empty.
	// We need to close and re-open to trigger migration with the container present.
	s2, err := New(dir)
	if err != nil {
		t.Fatalf("New() second time failed: %v", err)
	}
	defer s2.Close()

	// Check that a default terminal was created for c-1
	var count int
	s2.db.QueryRow(`SELECT COUNT(*) FROM terminals WHERE container_id = 'c-1' AND is_default = 1`).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 default terminal for c-1, got %d", count)
	}

	// Check log file was migrated
	oldPath := filepath.Join(termDir, "c-1.log")
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Errorf("old log file should not exist after migration")
	}

	// Find the new log file
	var tid string
	s2.db.QueryRow(`SELECT id FROM terminals WHERE container_id = 'c-1'`).Scan(&tid)
	newPath := filepath.Join(termDir, "c-1", tid+".log")
	data, err := os.ReadFile(newPath)
	if err != nil {
		t.Fatalf("migrated log file not found at %s: %v", newPath, err)
	}
	if string(data) != "hello" {
		t.Errorf("migrated log content = %q, want %q", string(data), "hello")
	}
}

func TestMigrateSkipsWhenTerminalsExist(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// Insert a terminal manually
	_, err = s.db.Exec(
		`INSERT INTO terminals (id, container_id, name, is_default, sort_order) VALUES ('t-1', 'c-1', 'T1', 1, 0)`,
	)
	if err != nil {
		t.Fatalf("insert terminal failed: %v", err)
	}

	// Insert a container
	_, err = s.db.Exec(`INSERT INTO containers (id, name) VALUES ('c-2', 'Test2')`)
	if err != nil {
		t.Fatalf("insert container failed: %v", err)
	}
	s.Close()

	// Re-open: migration should skip because terminals table is not empty
	s2, err := New(dir)
	if err != nil {
		t.Fatalf("New() second time failed: %v", err)
	}
	defer s2.Close()

	// c-2 should NOT have a terminal created (migration skipped)
	var count int
	s2.db.QueryRow(`SELECT COUNT(*) FROM terminals WHERE container_id = 'c-2'`).Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 terminals for c-2 (migration should skip), got %d", count)
	}
}
