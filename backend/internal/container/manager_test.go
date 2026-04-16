package container

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadHistoryDefaultTerminalLimit(t *testing.T) {
	mgr := newTestManager(t)
	cid, tid := "c-1", "t-1"

	// Create container with default terminal
	mgr.containers[cid] = &Container{
		ID:   cid,
		Name: "Test",
		terminals: map[string]*Terminal{
			tid: {ID: tid, IsDefault: true},
		},
	}

	var b strings.Builder
	for i := 0; i < 1200; i++ {
		b.WriteString("line\n")
	}
	writeTestHistory(t, mgr, cid, tid, b.String())

	history, err := mgr.ReadHistory(cid, tid)
	if err != nil {
		t.Fatalf("ReadHistory returned error: %v", err)
	}

	expected := strings.Repeat("line\n", defaultHistoryLineLimit)
	if string(history) != expected {
		t.Fatalf("expected last %d lines, got %d bytes", defaultHistoryLineLimit, len(history))
	}
}

func TestReadHistoryExtraTerminalLimit(t *testing.T) {
	mgr := newTestManager(t)
	cid, tid := "c-1", "t-2"

	// Create container with extra (non-default) terminal
	mgr.containers[cid] = &Container{
		ID:   cid,
		Name: "Test",
		terminals: map[string]*Terminal{
			"t-1": {ID: "t-1", IsDefault: true},
			tid:   {ID: tid, IsDefault: false},
		},
	}

	var b strings.Builder
	for i := 0; i < 400; i++ {
		b.WriteString("line\n")
	}
	writeTestHistory(t, mgr, cid, tid, b.String())

	history, err := mgr.ReadHistory(cid, tid)
	if err != nil {
		t.Fatalf("ReadHistory returned error: %v", err)
	}

	expected := strings.Repeat("line\n", extraHistoryLineLimit)
	if string(history) != expected {
		t.Fatalf("expected last %d lines, got %d bytes", extraHistoryLineLimit, len(history))
	}
}

func TestReadHistoryCapsByBytes(t *testing.T) {
	mgr := newTestManager(t)
	cid, tid := "c-1", "t-1"

	mgr.containers[cid] = &Container{
		ID:   cid,
		Name: "Test",
		terminals: map[string]*Terminal{
			tid: {ID: tid, IsDefault: true},
		},
	}

	content := strings.Repeat("x", int(historyReplayByteLimit)+1024)
	writeTestHistory(t, mgr, cid, tid, content)

	history, err := mgr.ReadHistory(cid, tid)
	if err != nil {
		t.Fatalf("ReadHistory returned error: %v", err)
	}

	if len(history) != int(historyReplayByteLimit) {
		t.Fatalf("expected %d bytes, got %d", historyReplayByteLimit, len(history))
	}
}

func TestContainerHasDefaultTerminal(t *testing.T) {
	c := &Container{
		ID:   "c-1",
		Name: "Test",
		terminals: map[string]*Terminal{
			"t-1": {ID: "t-1", IsDefault: true, Name: "Terminal 1"},
			"t-2": {ID: "t-2", IsDefault: false, Name: "Terminal 2"},
		},
	}

	dt := c.GetDefaultTerminal()
	if dt == nil {
		t.Fatal("GetDefaultTerminal returned nil")
	}
	if dt.ID != "t-1" {
		t.Errorf("GetDefaultTerminal ID = %s, want t-1", dt.ID)
	}
}

func TestContainerGetTerminal(t *testing.T) {
	c := &Container{
		ID:   "c-1",
		Name: "Test",
		terminals: map[string]*Terminal{
			"t-1": {ID: "t-1", IsDefault: true},
			"t-2": {ID: "t-2", IsDefault: false},
		},
	}

	term, ok := c.GetTerminal("t-2")
	if !ok {
		t.Fatal("GetTerminal(t-2) returned false")
	}
	if term.ID != "t-2" {
		t.Errorf("terminal ID = %s, want t-2", term.ID)
	}

	_, ok = c.GetTerminal("nonexistent")
	if ok {
		t.Error("GetTerminal(nonexistent) should return false")
	}
}

func TestContainerListTerminals(t *testing.T) {
	c := &Container{
		ID:   "c-1",
		Name: "Test",
		terminals: map[string]*Terminal{
			"t-1": {ID: "t-1", IsDefault: true},
			"t-2": {ID: "t-2", IsDefault: false},
		},
	}

	list := c.ListTerminals()
	if len(list) != 2 {
		t.Errorf("ListTerminals len = %d, want 2", len(list))
	}
}

func TestDeleteContainerCleansAllTerminals(t *testing.T) {
	mgr := newTestManager(t)

	c := &Container{
		ID:   "c-1",
		Name: "Test",
		terminals: map[string]*Terminal{
			"t-1": {ID: "t-1", IsDefault: true, listeners: make(map[*Listener]bool)},
			"t-2": {ID: "t-2", IsDefault: false, listeners: make(map[*Listener]bool)},
		},
	}

	mgr.containers["c-1"] = c

	if !mgr.Delete("c-1") {
		t.Fatal("Delete returned false")
	}

	if _, ok := mgr.Get("c-1"); ok {
		t.Error("container should be deleted")
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

func writeTestHistory(t *testing.T, mgr *Manager, containerID, terminalID, content string) {
	t.Helper()

	dir := filepath.Join(mgr.dataDir, "terminals", containerID)
	os.MkdirAll(dir, 0755)
	path := mgr.terminalLogPath(containerID, terminalID)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write history: %v", err)
	}
}
