package server

import (
	"testing"
)

func TestHasChildProcessZeroPID(t *testing.T) {
	if HasChildProcess(0) {
		t.Error("HasChildProcess(0) should return false")
	}
}

func TestHasChildProcessNegativePID(t *testing.T) {
	if HasChildProcess(-1) {
		t.Error("HasChildProcess(-1) should return false")
	}
}

func TestHasChildProcessNonExistentPID(t *testing.T) {
	// PID 999999999 should not exist; fallback returns true (safer)
	if !HasChildProcess(999999999) {
		t.Error("HasChildProcess(nonexistent) should fallback to true")
	}
}

func TestHasChildProcessCurrentProcess(t *testing.T) {
	// Current test process PID=1 is init — skip if not predictable.
	// Just test that it doesn't panic with PID 1.
	_ = HasChildProcess(1)
}
