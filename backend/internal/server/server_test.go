package server

import (
	"testing"

	"github.com/penguin/agent-hive/internal/fileutil"
)

func TestFileTypeDetection(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"main.go", "text"},
		{"README.md", "markdown"},
		{"photo.png", "image"},
		{"doc.pdf", "pdf"},
		{"binary.exe", "binary"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fileutil.FileType(tt.name)
			if got != tt.want {
				t.Errorf("FileType(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestGetMaxLinesDefault(t *testing.T) {
	if defaultMaxLines != 1000 {
		t.Errorf("defaultMaxLines = %d, want 1000", defaultMaxLines)
	}
}

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
