package pty

import (
	"os"
	"os/user"
	"testing"
)

func TestResolveSessionParams_NoOpts(t *testing.T) {
	shell, env, sysAttr, _, err := resolveSessionParams(nil)
	if err != nil {
		t.Fatalf("resolveSessionParams: %v", err)
	}

	// Should not set credential when not root (or no user specified)
	if os.Getuid() != 0 {
		if sysAttr != nil {
			t.Fatal("expected nil SysProcAttr for non-root")
		}
	}

	if shell == "" {
		t.Fatal("expected non-empty shell")
	}

	hasTermEnv := false
	for _, e := range env {
		if e == "TERM=xterm-256color" {
			hasTermEnv = true
		}
	}
	if !hasTermEnv {
		t.Fatal("expected TERM=xterm-256color in env")
	}
}

func TestResolveSessionParams_WithShellOverride(t *testing.T) {
	shell, _, _, _, err := resolveSessionParams(&SessionOptions{Shell: "/bin/sh"})
	if err != nil {
		t.Fatalf("resolveSessionParams: %v", err)
	}
	if shell != "/bin/sh" {
		t.Fatalf("expected /bin/sh, got %q", shell)
	}
}

func TestResolveSessionParams_NonRootIgnoresUser(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("test requires non-root")
	}

	u, _ := user.Current()
	_, _, sysAttr, _, err := resolveSessionParams(&SessionOptions{User: u.Username})
	if err != nil {
		t.Fatalf("resolveSessionParams: %v", err)
	}

	// Non-root should not set SysProcAttr even when User is specified
	if sysAttr != nil {
		t.Fatal("expected nil SysProcAttr for non-root even with User set")
	}
}

