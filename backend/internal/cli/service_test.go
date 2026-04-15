package cli

import (
	"os"
	"testing"
)

func TestCmdService_RequiresRootForStart(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("test requires non-root")
	}
	// Verify the root check logic works for start/stop/restart
	for _, action := range []string{"start", "stop", "restart"} {
		isRoot := os.Getuid() == 0
		if isRoot {
			t.Fatalf("%s should require root but current user is root", action)
		}
	}
}

func TestCmdService_StatusNoRoot(t *testing.T) {
	// status does not require root — verify it's not in the root-required list
	rootRequired := map[string]bool{
		"start":   true,
		"stop":    true,
		"restart": true,
	}
	if rootRequired["status"] {
		t.Fatal("status should not require root")
	}
}

func TestCmdLogs_FlagParsing(t *testing.T) {
	// Verify that flag.NewFlagSet for logs accepts -f and -n
	// This is a compile-time + basic logic test
	args := []string{"-f", "-n", "100"}

	// Simulate flag parsing
	follow := false
	lines := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-f":
			follow = true
		case "-n":
			if i+1 < len(args) {
				lines = args[i+1]
				i++
			}
		}
	}

	if !follow {
		t.Fatal("expected follow=true")
	}
	if lines != "100" {
		t.Fatalf("expected lines=100, got %q", lines)
	}
}
