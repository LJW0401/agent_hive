package container

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadHistoryReturnsLast500Lines(t *testing.T) {
	mgr := newTestManager(t)

	var b strings.Builder
	for i := 0; i < 600; i++ {
		b.WriteString("line\n")
	}
	writeTestHistory(t, mgr, "lines", b.String())

	history, err := mgr.ReadHistory("lines")
	if err != nil {
		t.Fatalf("ReadHistory returned error: %v", err)
	}

	expected := strings.Repeat("line\n", historyReplayLineLimit)
	if string(history) != expected {
		t.Fatalf("expected last %d lines, got %d bytes", historyReplayLineLimit, len(history))
	}
}

func TestReadHistoryCapsSparseLogsByBytes(t *testing.T) {
	mgr := newTestManager(t)

	content := strings.Repeat("x", int(historyReplayByteLimit)+1024)
	writeTestHistory(t, mgr, "sparse", content)

	history, err := mgr.ReadHistory("sparse")
	if err != nil {
		t.Fatalf("ReadHistory returned error: %v", err)
	}

	if len(history) != int(historyReplayByteLimit) {
		t.Fatalf("expected %d bytes, got %d", historyReplayByteLimit, len(history))
	}
	if string(history) != content[len(content)-int(historyReplayByteLimit):] {
		t.Fatalf("expected byte-capped tail content")
	}
}

func newTestManager(t *testing.T) *Manager {
	t.Helper()

	dataDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dataDir, "terminals"), 0755); err != nil {
		t.Fatalf("mkdir terminals: %v", err)
	}

	return &Manager{
		containers: make(map[string]*Container),
		dataDir:    dataDir,
	}
}

func writeTestHistory(t *testing.T, mgr *Manager, id, content string) {
	t.Helper()

	if err := os.WriteFile(mgr.terminalLogPath(id), []byte(content), 0644); err != nil {
		t.Fatalf("write history: %v", err)
	}
}
