package container

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Regression: after a server restart the user clicks reopen, which drives
// reopenTerminal — and the terminal's log file on disk must be preserved so
// ReadHistory can replay prior scrollback. Prior to this fix, reopenTerminal
// truncated the log, wiping history and making the reopened terminal look blank.
//
// We test the file-opening helper directly (rather than reopenTerminal) because
// the full path also spawns a real PTY. The helper is the one decision point
// that governs "truncate vs. preserve", so covering it locks the guarantee in.
func TestOpenTerminalLogFilePreservesPriorContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "t-1.log")

	prior := "previous session output\nline two\n"
	if err := os.WriteFile(path, []byte(prior), 0644); err != nil {
		t.Fatalf("seed log: %v", err)
	}

	f, err := openTerminalLogFile(path)
	if err != nil {
		t.Fatalf("openTerminalLogFile: %v", err)
	}
	// Simulate the new session emitting more output post-reopen.
	if _, err := f.WriteString("new session line\n"); err != nil {
		t.Fatalf("write: %v", err)
	}
	f.Close()

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	want := prior + "new session line\n"
	if string(got) != want {
		t.Errorf("content after reopen-style open mismatch\n got: %q\nwant: %q", got, want)
	}
}

// Edge (边界值): opening a non-existent log must create it empty — the reopen
// path should never fail just because the file happens to be missing (e.g. the
// user wiped data/terminals out-of-band).
func TestOpenTerminalLogFileCreatesWhenMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fresh.log")

	f, err := openTerminalLogFile(path)
	if err != nil {
		t.Fatalf("openTerminalLogFile on missing: %v", err)
	}
	f.Close()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Size() != 0 {
		t.Errorf("fresh file size = %d, want 0", info.Size())
	}
}

// Regression (通过 ReadHistory 的视角): the full loop — seed log, reopen-style
// open, write more — ends with ReadHistory returning both old and new content.
// This guards the user-visible behavior (not just the file-mode flag).
func TestReadHistoryAfterReopenIncludesPriorSession(t *testing.T) {
	mgr := newTestManager(t)
	cid, tid := "c-1", "t-1"

	mgr.containers[cid] = &Container{
		ID:   cid,
		Name: "Test",
		terminals: map[string]*Terminal{
			tid: {ID: tid, IsDefault: true, listeners: make(map[*Listener]bool)},
		},
	}

	// Simulate a prior session's output on disk (what Restore sees after a
	// server restart).
	writeTestHistory(t, mgr, cid, tid, "prior session output\n")

	// Simulate what reopenTerminal does on the filesystem side: open the log
	// for writing, then append more output as the new shell produces it.
	path := mgr.terminalLogPath(cid, tid)
	f, err := openTerminalLogFile(path)
	if err != nil {
		t.Fatalf("openTerminalLogFile: %v", err)
	}
	if _, err := f.WriteString("post-reopen output\n"); err != nil {
		t.Fatalf("write: %v", err)
	}
	f.Close()

	history, err := mgr.ReadHistory(cid, tid)
	if err != nil {
		t.Fatalf("ReadHistory: %v", err)
	}
	if !strings.Contains(string(history), "prior session output") {
		t.Errorf("history lost prior session output: %q", history)
	}
	if !strings.Contains(string(history), "post-reopen output") {
		t.Errorf("history missing post-reopen output: %q", history)
	}
}

// Smoke: reconnect marker contains a timestamp and the "terminal reconnected"
// phrase — so the user can see in scrollback where the seam is.
func TestFormatReconnectMarkerIncludesTimestamp(t *testing.T) {
	fixed := time.Date(2026, 4, 23, 16, 50, 30, 0, time.UTC)
	got := string(formatReconnectMarker(fixed))
	if !strings.Contains(got, "2026-04-23 16:50:30") {
		t.Errorf("marker missing timestamp: %q", got)
	}
	if !strings.Contains(got, "终端已重连") {
		t.Errorf("marker missing reconnect label: %q", got)
	}
}

// Edge (边界值): marker begins and ends with CRLF so it never runs together
// with the last character of the previous session's output or the first
// character of the new one.
func TestFormatReconnectMarkerIsLineBounded(t *testing.T) {
	got := string(formatReconnectMarker(time.Now()))
	if !strings.HasPrefix(got, "\r\n") {
		t.Errorf("marker must start with CRLF, got %q", got)
	}
	if !strings.HasSuffix(got, "\r\n") {
		t.Errorf("marker must end with CRLF, got %q", got)
	}
}

// Edge (异常恢复): ANSI dim sequences surround the human text so the marker is
// visually distinct from regular shell output. The SGR reset at the end
// prevents dim-mode from leaking into subsequent output — historically one of
// the easier ways to garble a terminal.
func TestFormatReconnectMarkerResetsSGR(t *testing.T) {
	got := string(formatReconnectMarker(time.Now()))
	if !strings.Contains(got, "\x1b[2m") {
		t.Errorf("marker missing dim intro: %q", got)
	}
	if !strings.Contains(got, "\x1b[0m") {
		t.Errorf("marker missing SGR reset — dim would leak into later output: %q", got)
	}
}
