package config

import (
	"os"
	"os/user"
	"testing"
)

func TestLookupUserShell(t *testing.T) {
	u, err := user.Current()
	if err != nil {
		t.Skip("cannot get current user")
	}

	shell := LookupUserShell(u.Username)
	if shell == "" {
		t.Fatal("expected non-empty shell")
	}
	if shell[0] != '/' {
		t.Fatalf("expected absolute path, got %q", shell)
	}
}

func TestLookupUserShell_NonExistent(t *testing.T) {
	shell := LookupUserShell("nonexistent_user_xyz_99999")
	if shell != "/bin/bash" {
		t.Fatalf("expected /bin/bash fallback, got %q", shell)
	}
}

func TestLoad_WithUserShell(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/config.yaml"

	data := []byte("port: 9090\nuser: testuser\nshell: /bin/zsh\n")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.User != "testuser" {
		t.Fatalf("expected user 'testuser', got %q", cfg.User)
	}
	if cfg.Shell != "/bin/zsh" {
		t.Fatalf("expected shell '/bin/zsh', got %q", cfg.Shell)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	cfg, err := Load("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != 8090 {
		t.Fatalf("expected default port 8090, got %d", cfg.Port)
	}
}

func TestLoad_InfersUserFromFileOwner(t *testing.T) {
	u, err := user.Current()
	if err != nil {
		t.Skip("cannot get current user")
	}

	dir := t.TempDir()
	path := dir + "/config.yaml"
	if err := os.WriteFile(path, []byte("port: 8090\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// The file owner should be the current user
	if cfg.User != u.Username {
		t.Fatalf("expected user %q (file owner), got %q", u.Username, cfg.User)
	}
	if cfg.Shell == "" {
		t.Fatal("expected non-empty shell from inference")
	}
}

func TestFallbackToCurrentUser(t *testing.T) {
	u, err := user.Current()
	if err != nil {
		t.Skip("cannot get current user")
	}

	cfg := &Config{}
	cfg.fallbackToCurrentUser()

	if cfg.User != u.Username {
		t.Fatalf("expected user %q, got %q", u.Username, cfg.User)
	}
	if cfg.Shell == "" {
		t.Fatal("expected non-empty shell from fallback")
	}
}

func TestFallbackToCurrentUser_PartialFill(t *testing.T) {
	cfg := &Config{User: "already-set"}
	cfg.fallbackToCurrentUser()

	if cfg.User != "already-set" {
		t.Fatalf("expected user not overwritten, got %q", cfg.User)
	}
	if cfg.Shell == "" {
		t.Fatal("expected shell to be filled by fallback")
	}
}
