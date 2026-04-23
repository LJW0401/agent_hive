package container

import (
	"os"
	"sync"

	ptypkg "github.com/penguin/agent-hive/internal/pty"
)

// Terminal represents a single PTY terminal within a container.
type Terminal struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	IsDefault bool   `json:"isDefault"`
	Connected bool   `json:"connected"`

	mu        sync.Mutex
	session   *ptypkg.Session
	logFile   *os.File
	listeners map[*Listener]bool
	// lastCWD is the last observed working directory of the shell process.
	// Captured by the periodic CWD poller so that reopening the terminal can
	// start in the same directory even after the shell has exited.
	lastCWD string
}

// LastCWD returns the last observed working directory (may be empty).
func (t *Terminal) LastCWD() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.lastCWD
}

// SetLastCWD updates the cached last working directory.
func (t *Terminal) SetLastCWD(cwd string) {
	t.mu.Lock()
	t.lastCWD = cwd
	t.mu.Unlock()
}

// AddListener registers a listener for this terminal.
func (t *Terminal) AddListener(l *Listener) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.listeners == nil {
		t.listeners = make(map[*Listener]bool)
	}
	t.listeners[l] = true
}

// RemoveListener unregisters and closes a listener for this terminal.
func (t *Terminal) RemoveListener(l *Listener) {
	t.mu.Lock()
	delete(t.listeners, l)
	t.mu.Unlock()
	l.Close()
}

// WriteToPTY writes data to this terminal's PTY session.
func (t *Terminal) WriteToPTY(data []byte) (int, error) {
	t.mu.Lock()
	s := t.session
	t.mu.Unlock()
	if s == nil {
		return 0, os.ErrClosed
	}
	return s.Write(data)
}

// ResizePTY resizes this terminal's PTY.
func (t *Terminal) ResizePTY(rows, cols uint16) error {
	t.mu.Lock()
	s := t.session
	t.mu.Unlock()
	if s == nil {
		return os.ErrClosed
	}
	return s.Resize(rows, cols)
}

// close shuts down the terminal's PTY session and log file.
func (t *Terminal) close() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.session != nil {
		t.session.Close()
		t.session = nil
	}
	if t.logFile != nil {
		t.logFile.Close()
		t.logFile = nil
	}
	t.Connected = false
}

// ProcessPID returns the PID of the shell process, or 0 if not connected.
func (t *Terminal) ProcessPID() int {
	t.mu.Lock()
	s := t.session
	t.mu.Unlock()
	if s == nil {
		return 0
	}
	return s.PID()
}
