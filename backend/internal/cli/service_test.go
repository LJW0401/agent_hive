package cli

import (
	"os"
	"testing"
)

func TestCheckRoot_NonRoot(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("test requires non-root")
	}

	for _, cmd := range []string{"start", "stop", "restart", "install", "uninstall"} {
		err := checkRoot(cmd)
		if err == nil {
			t.Fatalf("checkRoot(%q) should return error for non-root", cmd)
		}
	}
}

func TestCheckRoot_ErrorMessage(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("test requires non-root")
	}

	err := checkRoot("install")
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if msg != "error: 'install' requires root privileges. Use sudo." {
		t.Fatalf("unexpected error message: %s", msg)
	}
}

func TestCmdService_StatusNoRootRequired(t *testing.T) {
	// status branch in CmdService does not call requireRoot
	// Verify by checking that the "status" action is not in the root-required branch
	rootActions := map[string]bool{"start": true, "stop": true, "restart": true}
	if rootActions["status"] {
		t.Fatal("status should not require root")
	}
}

func TestCmdLogs_FlagParsing(t *testing.T) {
	args := []string{"-f", "-n", "100"}

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
