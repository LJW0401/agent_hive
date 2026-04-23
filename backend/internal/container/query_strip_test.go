package container

import (
	"bytes"
	"strings"
	"testing"
)

// Regression: TUI anchor + 1MB tail expansion made each reopen replay dozens
// of shell-prompt probes (DA1 `\x1b[c`, DECRQM `\x1b[?2026$p`, ...) back into
// xterm.js, which answered every single one. The answers (prefixed by
// `\x1b[?`) were written to the PTY as input, then zsh's ZLE consumed the
// `ESC-[` and inserted the tail as literal text at the prompt — the user
// saw `2026;2$y1;2c` strings repeated at their cursor.
//
// Fix: strip terminal query sequences from replay bytes so xterm.js has no
// queries to answer; responses (different SGR shape) pass through.
func TestStripTerminalQueriesRemovesDA1AndDECRQM(t *testing.T) {
	// Simulated shell prompt probe: DA1 + DECRQM mode 2026 query.
	in := []byte("prompt-start\x1b[c\x1b[?2026$p\x1b[0m user-text\n")
	got := stripTerminalQueries(in)
	if bytes.Contains(got, []byte("\x1b[c")) {
		t.Errorf("DA1 query not stripped: %q", got)
	}
	if bytes.Contains(got, []byte("\x1b[?2026$p")) {
		t.Errorf("DECRQM query not stripped: %q", got)
	}
	// Surrounding content and non-query SGR must survive.
	if !bytes.Contains(got, []byte("prompt-start")) {
		t.Errorf("stripping ate non-query content: %q", got)
	}
	if !bytes.Contains(got, []byte("user-text")) {
		t.Errorf("stripping ate trailing text: %q", got)
	}
	if !bytes.Contains(got, []byte("\x1b[0m")) {
		t.Errorf("SGR reset mistakenly stripped: %q", got)
	}
}

// Edge (相似序列区分): responses and queries share a family but differ in
// syntax (responses have `?` prefix or `$y` suffix that queries lack). Strip
// must NOT touch responses — otherwise legitimate terminal state reporting
// seen in, e.g., a `tput` command's output would be lost.
func TestStripTerminalQueriesPreservesResponses(t *testing.T) {
	cases := []string{
		"\x1b[?1;2c",           // DA1 response
		"\x1b[?6c",             // DA1 alt response
		"\x1b[?2026;2$y",       // DECRQM mode set response
		"\x1b[?2004;1$y",       // DECRQM bracketed paste state response
		"\x1b[0n",              // DSR 5 OK response
		"\x1b[25;80R",          // DSR 6 cursor position report
		"\x1b[>0;276;0c",       // DA2 response
	}
	for _, r := range cases {
		got := stripTerminalQueries([]byte(r))
		if !bytes.Equal(got, []byte(r)) {
			t.Errorf("response %q mutated → %q", r, got)
		}
	}
}

// Edge (同家族其他查询): DA2, DA3, DSR 5/6, XT version must all be stripped —
// any of them could show up in real prompt setups and produce garbled input
// on replay.
func TestStripTerminalQueriesCoversCommonFamilies(t *testing.T) {
	cases := []string{
		"\x1b[c",     // DA1 (no params)
		"\x1b[0c",    // DA1 (explicit 0)
		"\x1b[>c",    // DA2
		"\x1b[>0c",   // DA2 (with param)
		"\x1b[=c",    // DA3
		"\x1b[5n",    // DSR 5 (status)
		"\x1b[6n",    // DSR 6 (cursor position)
		"\x1b[25$p",  // DECRQM public mode 25
		"\x1b[?7$p",  // DECRQM private mode 7
		"\x1b[>q",    // XT version
		"\x1b[>0q",   // XT version with 0
	}
	for _, q := range cases {
		got := stripTerminalQueries([]byte(q))
		if len(got) != 0 {
			t.Errorf("query %q not fully stripped → %q", q, got)
		}
	}
}

// Edge (边界值): empty and nil inputs are safe no-ops.
func TestStripTerminalQueriesEmpty(t *testing.T) {
	if got := stripTerminalQueries(nil); len(got) != 0 {
		t.Errorf("nil input: got %q", got)
	}
	if got := stripTerminalQueries([]byte{}); len(got) != 0 {
		t.Errorf("empty input: got %q", got)
	}
}

// Edge (非法输入 / 类似字节): raw "c", "n", "p", "q" characters in plain
// text — anywhere `\x1b[` is absent — must never be stripped.
func TestStripTerminalQueriesPlainTextUntouched(t *testing.T) {
	in := []byte("abc 123 $p >q some-file.c\n")
	got := stripTerminalQueries(in)
	if !bytes.Equal(got, in) {
		t.Errorf("plain text got touched: %q → %q", in, got)
	}
}

// Integration: ReadHistory must produce a replay stream that would not retrigger
// terminal query responses when fed to xterm.js.
func TestReadHistoryStripsTerminalQueriesFromReplay(t *testing.T) {
	mgr := newTestManager(t)
	cid, tid := "c-1", "t-1"
	mgr.containers[cid] = &Container{
		ID:   cid,
		Name: "Test",
		terminals: map[string]*Terminal{
			tid: {ID: tid, IsDefault: true, listeners: make(map[*Listener]bool)},
		},
	}
	// Simulate 5 prompt renders, each emitting a probe pair.
	var b strings.Builder
	for i := 0; i < 5; i++ {
		b.WriteString("user@host ~ $ cmd\r\n")
		b.WriteString("\x1b[c\x1b[?2026$p")
	}
	writeTestHistory(t, mgr, cid, tid, b.String())

	history, err := mgr.ReadHistory(cid, tid)
	if err != nil {
		t.Fatalf("ReadHistory: %v", err)
	}
	if bytes.Contains(history, []byte("\x1b[c")) {
		t.Errorf("replay still contains DA1 probe (would retrigger response): %q", history)
	}
	if bytes.Contains(history, []byte("\x1b[?2026$p")) {
		t.Errorf("replay still contains DECRQM probe: %q", history)
	}
	if !bytes.Contains(history, []byte("user@host")) {
		t.Errorf("non-probe content lost: %q", history)
	}
}
