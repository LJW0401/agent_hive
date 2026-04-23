package store

import (
	"database/sql"
	"path/filepath"
	"testing"
)

// Smoke: round-trip — UpdateTerminalCWD should be visible through GetTerminal.
func TestUpdateTerminalCWDRoundTrip(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.CreateTerminal("c-1", "t-1", "Agent", true); err != nil {
		t.Fatalf("CreateTerminal: %v", err)
	}
	if err := s.UpdateTerminalCWD("t-1", "/home/u/work"); err != nil {
		t.Fatalf("UpdateTerminalCWD: %v", err)
	}
	got, err := s.GetTerminal("t-1")
	if err != nil {
		t.Fatalf("GetTerminal: %v", err)
	}
	if got.LastCWD != "/home/u/work" {
		t.Errorf("LastCWD = %q, want /home/u/work", got.LastCWD)
	}
}

// Edge (边界值): a freshly created terminal has empty LastCWD.
func TestFreshTerminalHasEmptyLastCWD(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.CreateTerminal("c-1", "t-1", "Agent", true); err != nil {
		t.Fatalf("CreateTerminal: %v", err)
	}
	got, err := s.GetTerminal("t-1")
	if err != nil {
		t.Fatalf("GetTerminal: %v", err)
	}
	if got.LastCWD != "" {
		t.Errorf("expected empty LastCWD, got %q", got.LastCWD)
	}
}

// Edge (非法输入): updating an unknown terminal is a no-op (no error).
func TestUpdateTerminalCWDUnknownIDIsNoop(t *testing.T) {
	s := newTestStore(t)
	if err := s.UpdateTerminalCWD("missing", "/tmp"); err != nil {
		t.Fatalf("UpdateTerminalCWD on missing id: %v", err)
	}
}

// Edge (异常恢复): opening an existing DB created *without* the last_cwd column
// should transparently migrate — the migration must be idempotent.
func TestMigrationAddsLastCWDColumnToLegacySchema(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "agent_hive.db")

	// Hand-craft a legacy schema missing last_cwd, then close.
	raw, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open raw: %v", err)
	}
	if _, err := raw.Exec(`
		CREATE TABLE terminals (
			id           TEXT PRIMARY KEY,
			container_id TEXT NOT NULL,
			name         TEXT NOT NULL DEFAULT 'Agent',
			is_default   INTEGER NOT NULL DEFAULT 0,
			sort_order   INTEGER NOT NULL DEFAULT 0,
			created_at   DATETIME NOT NULL DEFAULT (datetime('now'))
		);
		INSERT INTO terminals (id, container_id, name, is_default, sort_order)
		VALUES ('t-legacy', 'c-1', 'Agent', 1, 0);
	`); err != nil {
		t.Fatalf("seed legacy: %v", err)
	}
	raw.Close()

	// Now New() should add the last_cwd column without dropping data.
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer s.Close()

	got, err := s.GetTerminal("t-legacy")
	if err != nil {
		t.Fatalf("GetTerminal after migration: %v", err)
	}
	if got.LastCWD != "" {
		t.Errorf("legacy row should have empty LastCWD, got %q", got.LastCWD)
	}
	if err := s.UpdateTerminalCWD("t-legacy", "/opt/app"); err != nil {
		t.Fatalf("UpdateTerminalCWD after migration: %v", err)
	}
	got, _ = s.GetTerminal("t-legacy")
	if got.LastCWD != "/opt/app" {
		t.Errorf("post-migration update LastCWD = %q, want /opt/app", got.LastCWD)
	}
}

// Edge (异常恢复): running migration twice is a no-op.
func TestMigrationIdempotent(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("first New: %v", err)
	}
	s.Close()
	s2, err := New(dir)
	if err != nil {
		t.Fatalf("second New: %v", err)
	}
	s2.Close()
}
