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

// Regression: the user saw "── 终端已重连 … ──^[[O%" after reopen. `^[[O` is
// \x1b[O, xterm.js's focus-out event — emitted only because the replay
// activated focus-tracking mode (\x1b[?1004h) that an earlier shell had
// enabled. When the new shell's tty is briefly in cooked+echo mode during
// startup, line discipline echoes the incoming \x1b[O back to the screen as
// visible `^[[O`. Strip focus/mouse mode toggles from replay so xterm.js
// stays in default (disabled) state — the fresh shell will re-enable what
// it wants via the live stream.
func TestStripTerminalQueriesRemovesFocusAndMouseModes(t *testing.T) {
	cases := []string{
		"\x1b[?1000h", "\x1b[?1000l", // X10 mouse
		"\x1b[?1001h", "\x1b[?1001l", // highlight mouse
		"\x1b[?1002h", "\x1b[?1002l", // button-event mouse
		"\x1b[?1003h", "\x1b[?1003l", // any-event mouse
		"\x1b[?1004h", "\x1b[?1004l", // focus events
		"\x1b[?1005h", "\x1b[?1005l", // UTF-8 mouse
		"\x1b[?1006h", "\x1b[?1006l", // SGR mouse
		"\x1b[?1015h", "\x1b[?1015l", // urxvt mouse
		"\x1b[?1016h", "\x1b[?1016l", // SGR-pixels mouse
	}
	for _, seq := range cases {
		got := stripTerminalQueries([]byte(seq))
		if len(got) != 0 {
			t.Errorf("mode toggle %q not stripped → %q", seq, got)
		}
	}
}

// Edge (区分邻近序列): alt-screen toggles (?47 / ?1047 / ?1049) must NOT be
// stripped — the anchor logic depends on them. Bracketed paste, cursor
// visibility, and other mode toggles are also preserved since they don't
// cause xterm.js to emit reverse input.
func TestStripTerminalQueriesPreservesAltScreenAndBenignModes(t *testing.T) {
	cases := []string{
		"\x1b[?1049h", "\x1b[?1049l", // alt-screen (anchored on)
		"\x1b[?1047h", "\x1b[?1047l", // alt-screen variant
		"\x1b[?47h", "\x1b[?47l", // old alt-screen
		"\x1b[?2004h", "\x1b[?2004l", // bracketed paste
		"\x1b[?25h", "\x1b[?25l", // cursor visibility
		"\x1b[?7h", "\x1b[?7l", // line wrap
	}
	for _, seq := range cases {
		got := stripTerminalQueries([]byte(seq))
		if !bytes.Equal(got, []byte(seq)) {
			t.Errorf("benign/anchor toggle %q mutated → %q", seq, got)
		}
	}
}

// Regression: the full reconnect scenario — log ends with a shell that
// enabled focus tracking, replay must not carry that enable forward.
func TestReadHistoryStripsFocusModeFromReplay(t *testing.T) {
	mgr := newTestManager(t)
	cid, tid := "c-1", "t-1"
	mgr.containers[cid] = &Container{
		ID:   cid,
		Name: "Test",
		terminals: map[string]*Terminal{
			tid: {ID: tid, IsDefault: true, listeners: make(map[*Listener]bool)},
		},
	}
	writeTestHistory(t, mgr, cid, tid,
		"prompt line\r\n\x1b[?1004h\x1b[?1002h ready\r\n",
	)
	history, err := mgr.ReadHistory(cid, tid)
	if err != nil {
		t.Fatalf("ReadHistory: %v", err)
	}
	if bytes.Contains(history, []byte("\x1b[?1004h")) {
		t.Errorf("focus mode still in replay — xterm.js would activate it and emit focus-out on refresh: %q", history)
	}
	if bytes.Contains(history, []byte("\x1b[?1002h")) {
		t.Errorf("mouse mode still in replay: %q", history)
	}
	if !bytes.Contains(history, []byte("prompt line")) {
		t.Errorf("non-mode content lost: %q", history)
	}
}

// Regression: post-v1.6.0 the user saw `10;rgb:e5e5/e7e7/ebeb11;rgb:1111/1111/1414`
// repeated at their prompt. These are OSC 10 / OSC 11 color *responses* emitted
// by xterm.js because the replay contained OSC 10/11 *queries* from the prior
// shell prompt (p10k / oh-my-zsh / vim / tmux probe fg/bg color). The query
// shape is `\x1b]<num>;?<terminator>`; the response shape is
// `\x1b]<num>;rgb:.../.../...<terminator>`. Queries must be stripped from
// replay so xterm.js has no queries to answer.
func TestStripTerminalQueriesRemovesOSCColorQueries(t *testing.T) {
	// BEL terminator (\x07) — xterm's de facto form.
	cases := []string{
		"\x1b]10;?\x07",   // fg color query
		"\x1b]11;?\x07",   // bg color query
		"\x1b]12;?\x07",   // cursor color query
		"\x1b]17;?\x07",   // highlight bg query
		"\x1b]19;?\x07",   // tek bg query
		"\x1b]4;1;?\x07",  // palette index 1 query
		"\x1b]4;15;?\x07", // palette index 15 query
		"\x1b]708;?\x07",  // urxvt border color query
	}
	for _, q := range cases {
		got := stripTerminalQueries([]byte(q))
		if len(got) != 0 {
			t.Errorf("OSC query %q not stripped → %q", q, got)
		}
	}
}

// Edge (ST terminator): same queries but terminated by ESC `\` (7-bit ST)
// instead of BEL. Must also be stripped — some shells/terminals use ST.
func TestStripTerminalQueriesRemovesOSCColorQueriesWithST(t *testing.T) {
	cases := []string{
		"\x1b]10;?\x1b\\",
		"\x1b]11;?\x1b\\",
		"\x1b]4;2;?\x1b\\",
	}
	for _, q := range cases {
		got := stripTerminalQueries([]byte(q))
		if len(got) != 0 {
			t.Errorf("OSC query (ST-terminated) %q not stripped → %q", q, got)
		}
	}
}

// Edge (区分响应与 active set): OSC 10/11 responses (with concrete rgb: values)
// and OSC 0/1/2 (window / icon title sets — not queries, different number
// range) must survive. Otherwise a legitimate title-set during replay would
// be erased, or a tput-style response capture would be mangled.
func TestStripTerminalQueriesPreservesOSCSetsAndResponses(t *testing.T) {
	cases := []string{
		"\x1b]10;rgb:e5e5/e7e7/ebeb\x07", // OSC 10 response / active set
		"\x1b]11;rgb:1111/1111/1414\x07", // OSC 11 response / active set
		"\x1b]10;#ffffff\x07",            // OSC 10 active set (short form)
		"\x1b]0;window-title\x07",        // OSC 0 window title set
		"\x1b]2;terminal-title\x07",      // OSC 2 title set
		"\x1b]1;icon-name\x07",           // OSC 1 icon set
		"\x1b]4;3;rgb:ffff/0000/0000\x07", // OSC 4 palette set (not query)
	}
	for _, seq := range cases {
		got := stripTerminalQueries([]byte(seq))
		if !bytes.Equal(got, []byte(seq)) {
			t.Errorf("OSC set/response %q mutated → %q", seq, got)
		}
	}
}

// Regression (full pipeline): ReadHistory must strip OSC color probes, the
// exact scenario that produced `10;rgb:...11;rgb:...` on the user's prompt.
func TestReadHistoryStripsOSCColorQueriesFromReplay(t *testing.T) {
	mgr := newTestManager(t)
	cid, tid := "c-1", "t-1"
	mgr.containers[cid] = &Container{
		ID:   cid,
		Name: "Test",
		terminals: map[string]*Terminal{
			tid: {ID: tid, IsDefault: true, listeners: make(map[*Listener]bool)},
		},
	}
	var b strings.Builder
	for i := 0; i < 5; i++ {
		b.WriteString("user@host ~ $ cmd\r\n")
		b.WriteString("\x1b]10;?\x07\x1b]11;?\x07") // p10k-style fg+bg probe
	}
	writeTestHistory(t, mgr, cid, tid, b.String())

	history, err := mgr.ReadHistory(cid, tid)
	if err != nil {
		t.Fatalf("ReadHistory: %v", err)
	}
	if bytes.Contains(history, []byte("\x1b]10;?")) {
		t.Errorf("replay still contains OSC 10 query: %q", history)
	}
	if bytes.Contains(history, []byte("\x1b]11;?")) {
		t.Errorf("replay still contains OSC 11 query: %q", history)
	}
	if !bytes.Contains(history, []byte("user@host")) {
		t.Errorf("non-probe content lost: %q", history)
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
