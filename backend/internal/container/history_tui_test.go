package container

import (
	"bytes"
	"strings"
	"testing"
)

// Smoke: latest toggle wins across the three historical alt-screen variants.
func TestFindLastAltScreenAnchorPrefersLatest(t *testing.T) {
	buf := []byte("AAA\x1b[?47hBBB\x1b[?1047hCCC\x1b[?1049lDDD")
	got := findLastAltScreenAnchor(buf)
	want := bytes.LastIndex(buf, []byte("\x1b[?1049l"))
	if got != want {
		t.Errorf("anchor = %d, want %d", got, want)
	}
}

// Edge (非法输入): empty buf and buf with no toggle both return -1.
func TestFindLastAltScreenAnchorNoMatch(t *testing.T) {
	if got := findLastAltScreenAnchor(nil); got != -1 {
		t.Errorf("nil buf = %d, want -1", got)
	}
	if got := findLastAltScreenAnchor([]byte("plain shell output")); got != -1 {
		t.Errorf("no-toggle buf = %d, want -1", got)
	}
}

// Edge (边界值): buf with fewer newlines than the limit is returned whole.
func TestTrimToLastLinesUnderLimit(t *testing.T) {
	buf := []byte("a\nb\nc\n")
	got := trimToLastLines(buf, 100)
	if !bytes.Equal(got, buf) {
		t.Errorf("got %q, want %q", got, buf)
	}
}

// Edge (边界值): lineLimit <= 0 is a defensive no-op, not a full truncate.
func TestTrimToLastLinesZeroLimit(t *testing.T) {
	buf := []byte("a\nb\nc\n")
	got := trimToLastLines(buf, 0)
	if !bytes.Equal(got, buf) {
		t.Errorf("got %q, want buf unchanged", got)
	}
}

// Smoke: trim keeps the tail matching the limit.
func TestTrimToLastLinesAtLimit(t *testing.T) {
	buf := []byte("a\nb\nc\nd\n")
	got := trimToLastLines(buf, 2)
	want := []byte("c\nd\n")
	if !bytes.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

// Regression: when a TUI (Claude Code, vim, less, …) was running before
// disconnect, the log contains an \x1b[?1049h "enter alternate screen" toggle
// followed by many KB of cursor-addressable redraws with almost no newlines.
// The old byte-tail readHistoryFile frequently chopped the 1049h toggle off
// the front, so replay in xterm.js landed in the main screen with garbled
// escape remnants — the user saw only a few dozen visible lines.
//
// After fix: when an alt-screen toggle exists anywhere in a reasonable
// look-back window, readHistoryFile must anchor replay to that toggle so
// xterm.js can reconstruct the TUI's screen state.
func TestReadHistoryAnchorsOnAltScreenToggle(t *testing.T) {
	mgr := newTestManager(t)
	cid, tid := "c-1", "t-1"

	mgr.containers[cid] = &Container{
		ID:   cid,
		Name: "Test",
		terminals: map[string]*Terminal{
			tid: {ID: tid, IsDefault: true, listeners: make(map[*Listener]bool)},
		},
	}

	// Build a log that blows past the naive byte tail:
	//   pre — 50KB of ordinary shell output
	//   enter — \x1b[?1049h entering alt-screen
	//   tui — ~500KB of TUI repaints (cursor moves, no newlines)
	// Total ~550KB. Naive tail (≤1MB) that ignores toggles would still include
	// the enter here, but in the 256KB-only world it would be truncated.
	// We want the anchor logic to reliably keep the toggle even for much
	// larger tui payloads, so we push past the old limit.
	pre := strings.Repeat("shell prompt output\n", 50*1024/20) // ~50KB with newlines
	enter := "\x1b[?1049h"
	tui := strings.Repeat("\x1b[H\x1b[2K redraw ", 500*1024/15) // ~500KB, no \n
	content := pre + enter + tui
	writeTestHistory(t, mgr, cid, tid, content)

	history, err := mgr.ReadHistory(cid, tid)
	if err != nil {
		t.Fatalf("ReadHistory: %v", err)
	}

	if !bytes.Contains(history, []byte(enter)) {
		t.Errorf("history lost alt-screen enter toggle — xterm.js would replay to the wrong screen")
	}
	if !bytes.Contains(history, []byte("redraw")) {
		t.Errorf("history missing TUI body")
	}
}

// Edge: the most recent 1049l (exit alt-screen) should win over an earlier
// 1049h, so a shell that briefly entered then exited a TUI replays cleanly on
// the main screen.
func TestReadHistoryAnchorsOnLatestToggle(t *testing.T) {
	mgr := newTestManager(t)
	cid, tid := "c-1", "t-1"
	mgr.containers[cid] = &Container{
		ID:   cid,
		Name: "Test",
		terminals: map[string]*Terminal{
			tid: {ID: tid, IsDefault: true, listeners: make(map[*Listener]bool)},
		},
	}

	content := "" +
		strings.Repeat("x", 10*1024) +
		"\x1b[?1049h" + "TUI-PAYLOAD" + "\x1b[?1049l" +
		"post-tui shell output\n"
	writeTestHistory(t, mgr, cid, tid, content)

	history, err := mgr.ReadHistory(cid, tid)
	if err != nil {
		t.Fatalf("ReadHistory: %v", err)
	}
	// The 1049l (exit) is the last toggle, so history should start at/after it,
	// meaning the TUI enter from earlier is NOT in the returned bytes.
	if bytes.Contains(history, []byte("\x1b[?1049h")) {
		t.Errorf("history should anchor past the later 1049l, dropping the earlier 1049h")
	}
	if !bytes.Contains(history, []byte("\x1b[?1049l")) {
		t.Errorf("history missing 1049l anchor")
	}
	if !bytes.Contains(history, []byte("post-tui shell output")) {
		t.Errorf("history missing post-tui content")
	}
}

// Edge (非法输入 / 边界值): when there is no alt-screen toggle at all, the
// byte cap still applies but at the new (larger) limit. Existing non-TUI
// shells should not regress — they should get more history than before.
func TestReadHistoryNoToggleUsesByteCap(t *testing.T) {
	mgr := newTestManager(t)
	cid, tid := "c-1", "t-1"
	mgr.containers[cid] = &Container{
		ID:   cid,
		Name: "Test",
		terminals: map[string]*Terminal{
			tid: {ID: tid, IsDefault: true, listeners: make(map[*Listener]bool)},
		},
	}

	// ~2MB of byte soup, no newlines, no toggles — the byte cap governs.
	content := strings.Repeat("x", 2*1024*1024)
	writeTestHistory(t, mgr, cid, tid, content)

	history, err := mgr.ReadHistory(cid, tid)
	if err != nil {
		t.Fatalf("ReadHistory: %v", err)
	}
	if int64(len(history)) != historyReplayByteLimit {
		t.Errorf("history size = %d, want %d (byte cap)", len(history), historyReplayByteLimit)
	}
}
