package pty

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"sync"
	"syscall"

	"github.com/creack/pty"
	"github.com/penguin/agent-hive/internal/config"
)

// SessionOptions configures a PTY session.
type SessionOptions struct {
	User  string // target username (empty = current user)
	Shell string // shell path (empty = auto-detect)
}

// Session wraps a PTY process.
type Session struct {
	ptmx *os.File
	cmd  *exec.Cmd
	mu   sync.Mutex
}

// NewSession starts a new shell in a PTY.
// If opts is nil, defaults to the current user's shell.
func NewSession(opts *SessionOptions) (*Session, error) {
	shell, env, sysAttr, dir, err := resolveSessionParams(opts)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(shell)
	cmd.Env = env
	cmd.Dir = dir
	if sysAttr != nil {
		cmd.SysProcAttr = sysAttr
	}

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, err
	}

	return &Session{
		ptmx: ptmx,
		cmd:  cmd,
	}, nil
}

func resolveSessionParams(opts *SessionOptions) (shell string, env []string, sysAttr *syscall.SysProcAttr, dir string, err error) {
	if opts == nil {
		opts = &SessionOptions{}
	}

	isRoot := os.Getuid() == 0
	targetUser := opts.User
	shell = opts.Shell

	if targetUser != "" && isRoot {
		u, lookupErr := user.Lookup(targetUser)
		if lookupErr != nil {
			return "", nil, nil, "", fmt.Errorf("lookup user %q: %w", targetUser, lookupErr)
		}

		uid, _ := strconv.Atoi(u.Uid)
		gid, _ := strconv.Atoi(u.Gid)

		sysAttr = &syscall.SysProcAttr{
			Credential: &syscall.Credential{
				Uid: uint32(uid),
				Gid: uint32(gid),
			},
		}

		dir = u.HomeDir
		if shell == "" {
			shell = config.LookupUserShell(targetUser)
		}

		env = buildUserEnv(u, shell)
	} else {
		// Non-root or no target user: use current process settings
		if shell == "" {
			shell = os.Getenv("SHELL")
			if shell == "" {
				shell = "/bin/bash"
			}
		}
		dir = ""
		env = append(os.Environ(), "TERM=xterm-256color")
	}

	return shell, env, sysAttr, dir, nil
}

func buildUserEnv(u *user.User, shell string) []string {
	return []string{
		"HOME=" + u.HomeDir,
		"USER=" + u.Username,
		"LOGNAME=" + u.Username,
		"SHELL=" + shell,
		"TERM=xterm-256color",
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
	}
}

// Read reads from the PTY output.
func (s *Session) Read(p []byte) (int, error) {
	return s.ptmx.Read(p)
}

// Write writes to the PTY input.
func (s *Session) Write(p []byte) (int, error) {
	return s.ptmx.Write(p)
}

// Resize changes the PTY window size.
func (s *Session) Resize(rows, cols uint16) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return pty.Setsize(s.ptmx, &pty.Winsize{
		Rows: rows,
		Cols: cols,
	})
}

// Close terminates the PTY session.
func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	return s.ptmx.Close()
}

// Wait waits for the process to exit.
func (s *Session) Wait() error {
	return s.cmd.Wait()
}

// Reader returns an io.Reader for the PTY output.
func (s *Session) Reader() io.Reader {
	return s.ptmx
}
