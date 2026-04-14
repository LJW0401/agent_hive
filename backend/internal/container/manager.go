package container

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	ptypkg "github.com/penguin/agent-hive/internal/pty"
)

// Container represents a project container with an optional PTY session.
type Container struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Connected bool      `json:"connected"`
	CreatedAt time.Time `json:"createdAt"`

	mu           sync.Mutex
	session      *ptypkg.Session
	logFile      *os.File
	onOutput     func([]byte)
	onDisconnect func()
}

// Session returns the PTY session (nil if disconnected).
func (c *Container) Session() *ptypkg.Session {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.session
}

// Manager manages multiple containers.
type Manager struct {
	mu         sync.RWMutex
	containers map[string]*Container
	nextID     atomic.Int64
	dataDir    string
}

// NewManager creates a new container manager.
func NewManager(dataDir string) *Manager {
	termDir := filepath.Join(dataDir, "terminals")
	os.MkdirAll(termDir, 0755)
	return &Manager{
		containers: make(map[string]*Container),
		dataDir:    dataDir,
	}
}

func (m *Manager) terminalLogPath(id string) string {
	return filepath.Join(m.dataDir, "terminals", id+".log")
}

// Create creates a new container with a PTY session.
func (m *Manager) Create(name string) (*Container, error) {
	id := fmt.Sprintf("c-%d", m.nextID.Add(1))

	session, err := ptypkg.NewSession()
	if err != nil {
		return nil, err
	}

	logFile, err := os.OpenFile(m.terminalLogPath(id), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		session.Close()
		return nil, err
	}

	c := &Container{
		ID:        id,
		Name:      name,
		Connected: true,
		CreatedAt: time.Now(),
		session:   session,
		logFile:   logFile,
	}

	m.mu.Lock()
	m.containers[id] = c
	m.mu.Unlock()

	go m.pumpOutput(c)

	return c, nil
}

// Restore adds a container from persisted metadata without a PTY (disconnected).
func (m *Manager) Restore(id, name string, createdAt time.Time) {
	c := &Container{
		ID:        id,
		Name:      name,
		Connected: false,
		CreatedAt: createdAt,
	}

	m.mu.Lock()
	m.containers[id] = c
	m.mu.Unlock()

	// Update nextID to avoid collisions
	// Parse numeric part from "c-123"
	var num int64
	fmt.Sscanf(id, "c-%d", &num)
	for {
		cur := m.nextID.Load()
		if num <= cur {
			break
		}
		if m.nextID.CompareAndSwap(cur, num) {
			break
		}
	}
}

// Reopen creates a new PTY session for an existing disconnected container.
func (m *Manager) Reopen(id string) error {
	m.mu.RLock()
	c, ok := m.containers[id]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("container not found")
	}

	c.mu.Lock()
	if c.session != nil {
		c.mu.Unlock()
		return fmt.Errorf("container already connected")
	}

	session, err := ptypkg.NewSession()
	if err != nil {
		c.mu.Unlock()
		return err
	}

	// Truncate old log and start fresh
	logFile, err := os.OpenFile(m.terminalLogPath(id), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		session.Close()
		c.mu.Unlock()
		return err
	}

	c.session = session
	c.logFile = logFile
	c.Connected = true
	c.mu.Unlock()

	go m.pumpOutput(c)

	return nil
}

// pumpOutput reads from PTY and writes to the log file.
// When the PTY exits, the container is marked as disconnected.
func (m *Manager) pumpOutput(c *Container) {
	buf := make([]byte, 4096)
	for {
		c.mu.Lock()
		s := c.session
		c.mu.Unlock()
		if s == nil {
			return
		}

		n, err := s.Read(buf)
		if n > 0 {
			c.mu.Lock()
			if c.logFile != nil {
				c.logFile.Write(buf[:n])
			}
			c.mu.Unlock()

			// Notify attached WebSocket readers via the broadcast mechanism
			c.mu.Lock()
			if c.onOutput != nil {
				c.onOutput(buf[:n])
			}
			c.mu.Unlock()
		}
		if err != nil {
			break
		}
	}

	// Process exited — mark disconnected
	c.mu.Lock()
	if c.session != nil {
		c.session.Close()
		c.session = nil
	}
	if c.logFile != nil {
		c.logFile.Close()
		c.logFile = nil
	}
	c.Connected = false
	if c.onDisconnect != nil {
		c.onDisconnect()
	}
	c.mu.Unlock()
}

// Get returns a container by ID.
func (m *Manager) Get(id string) (*Container, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.containers[id]
	return c, ok
}

// Delete destroys a container and its PTY session.
func (m *Manager) Delete(id string) bool {
	m.mu.Lock()
	c, ok := m.containers[id]
	if !ok {
		m.mu.Unlock()
		return false
	}
	delete(m.containers, id)
	m.mu.Unlock()

	c.mu.Lock()
	if c.session != nil {
		c.session.Close()
		c.session = nil
	}
	if c.logFile != nil {
		c.logFile.Close()
		c.logFile = nil
	}
	c.mu.Unlock()

	// Remove log file
	os.Remove(m.terminalLogPath(id))

	return true
}

// List returns all containers.
func (m *Manager) List() []*Container {
	m.mu.RLock()
	defer m.mu.RUnlock()
	list := make([]*Container, 0, len(m.containers))
	for _, c := range m.containers {
		list = append(list, c)
	}
	return list
}

// Rename updates a container's name.
func (m *Manager) Rename(id, name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.containers[id]
	if !ok {
		return false
	}
	c.Name = name
	return true
}

// ReadHistory reads the terminal output log for a container.
func (m *Manager) ReadHistory(id string) ([]byte, error) {
	return os.ReadFile(m.terminalLogPath(id))
}

// SetCallbacks sets the output and disconnect callbacks for a container.
func (c *Container) SetCallbacks(onOutput func([]byte), onDisconnect func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onOutput = onOutput
	c.onDisconnect = onDisconnect
}

// ClearCallbacks clears the callbacks.
func (c *Container) ClearCallbacks() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onOutput = nil
	c.onDisconnect = nil
}

// WriteToPTY writes data to the PTY session.
func (c *Container) WriteToPTY(data []byte) (int, error) {
	c.mu.Lock()
	s := c.session
	c.mu.Unlock()
	if s == nil {
		return 0, io.ErrClosedPipe
	}
	return s.Write(data)
}

// ResizePTY resizes the PTY.
func (c *Container) ResizePTY(rows, cols uint16) error {
	c.mu.Lock()
	s := c.session
	c.mu.Unlock()
	if s == nil {
		return io.ErrClosedPipe
	}
	return s.Resize(rows, cols)
}
