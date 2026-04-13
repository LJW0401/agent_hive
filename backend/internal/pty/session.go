package pty

import (
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/creack/pty"
)

// Session wraps a PTY process.
type Session struct {
	ptmx *os.File
	cmd  *exec.Cmd
	mu   sync.Mutex
}

// NewSession starts a new shell in a PTY.
func NewSession() (*Session, error) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}

	cmd := exec.Command(shell)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, err
	}

	return &Session{
		ptmx: ptmx,
		cmd:  cmd,
	}, nil
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
