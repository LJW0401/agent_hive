package cli

import (
	"os/user"
	"testing"

	"github.com/penguin/agent-hive/internal/config"
)

func TestInitDetectsCurrentUser(t *testing.T) {
	u, err := user.Current()
	if err != nil {
		t.Skip("cannot get current user")
	}

	shell := config.LookupUserShell(u.Username)
	if shell == "" {
		t.Fatal("expected non-empty shell for current user")
	}
	if shell[0] != '/' {
		t.Fatalf("expected absolute shell path, got %q", shell)
	}
}

func TestInitInvalidUser(t *testing.T) {
	_, err := user.Lookup("nonexistent_user_xyz_99999")
	if err == nil {
		t.Fatal("expected error for non-existent user")
	}
}
