package container

import (
	"testing"

	ptypkg "github.com/penguin/agent-hive/internal/pty"
)

// --- reopenOpts --- //

// Smoke: inherited cwd wins over the base Dir.
func TestReopenOptsInheritsCWD(t *testing.T) {
	base := &ptypkg.SessionOptions{User: "u", Shell: "/bin/sh", Dir: "/default"}
	got := reopenOpts(base, "/tmp/work")
	if got.Dir != "/tmp/work" {
		t.Errorf("Dir = %q, want /tmp/work", got.Dir)
	}
	if got.User != "u" || got.Shell != "/bin/sh" {
		t.Errorf("non-Dir fields should be preserved, got %+v", got)
	}
}

// Edge (边界值): empty inherited cwd leaves the base Dir untouched.
func TestReopenOptsEmptyCWDKeepsBaseDir(t *testing.T) {
	base := &ptypkg.SessionOptions{Dir: "/default"}
	got := reopenOpts(base, "")
	if got.Dir != "/default" {
		t.Errorf("Dir = %q, want /default", got.Dir)
	}
}

// Edge (非法输入): nil base must not panic.
func TestReopenOptsNilBase(t *testing.T) {
	got := reopenOpts(nil, "/x")
	if got.Dir != "/x" {
		t.Errorf("Dir = %q, want /x", got.Dir)
	}
	got = reopenOpts(nil, "")
	if got.Dir != "" {
		t.Errorf("Dir should default to empty, got %q", got.Dir)
	}
}

// Edge (并发竞态防御): mutating the returned opts must not pollute base —
// otherwise two concurrent reopens would stomp on each other's Dir.
func TestReopenOptsIsCopy(t *testing.T) {
	base := &ptypkg.SessionOptions{Dir: "/default"}
	got := reopenOpts(base, "/a")
	got.Dir = "/mutated"
	if base.Dir != "/default" {
		t.Errorf("base mutated: %q", base.Dir)
	}
}

// --- observeCWD --- //

// Smoke: first observation records the cwd and reports a change.
func TestObserveCWDFirstSampleRecords(t *testing.T) {
	term := &Terminal{ID: "t-1"}
	if !term.observeCWD("/home/u") {
		t.Fatal("first non-empty sample should report a change")
	}
	if term.LastCWD() != "/home/u" {
		t.Errorf("LastCWD = %q, want /home/u", term.LastCWD())
	}
}

// Edge (边界值): a repeated identical sample does not report a change — the
// DB write path skips on !changed, so this is what keeps sqlite idle.
func TestObserveCWDRepeatedIsNoop(t *testing.T) {
	term := &Terminal{ID: "t-1"}
	term.observeCWD("/a")
	if term.observeCWD("/a") {
		t.Errorf("repeated identical sample should not report a change")
	}
}

// Edge (非法输入): an empty read (e.g. /proc/<pid>/cwd gone) must not erase the
// cached value — stale but non-empty is better than blank for a later reopen.
func TestObserveCWDEmptyIgnored(t *testing.T) {
	term := &Terminal{ID: "t-1", lastCWD: "/earlier"}
	if term.observeCWD("") {
		t.Errorf("empty sample should not report a change")
	}
	if term.LastCWD() != "/earlier" {
		t.Errorf("LastCWD overwritten by empty read: %q", term.LastCWD())
	}
}

// Edge (异常恢复): transitioning from a bad sample back to a good one must
// work — we do not want a single empty read to latch-off the poller.
func TestObserveCWDRecoversAfterEmpty(t *testing.T) {
	term := &Terminal{ID: "t-1"}
	term.observeCWD("/a")
	term.observeCWD("") // ignored
	if !term.observeCWD("/b") {
		t.Fatal("new cwd after an empty sample should register as a change")
	}
	if term.LastCWD() != "/b" {
		t.Errorf("LastCWD = %q, want /b", term.LastCWD())
	}
}

// --- sessionPID --- //

// Edge (非法输入): a terminal with no session returns PID 0, which the poller
// uses as its exit signal.
func TestSessionPIDOnDisconnectedTerminal(t *testing.T) {
	term := &Terminal{ID: "t-1"}
	if pid := term.sessionPID(); pid != 0 {
		t.Errorf("disconnected terminal PID = %d, want 0", pid)
	}
}
