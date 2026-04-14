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

// Listener receives PTY output and disconnect events via buffered channel.
type Listener struct {
	ch   chan []byte
	done chan struct{}

	OnDisconnect func()
}

// NewListener creates a listener with a buffered output channel.
func NewListener(onOutput func([]byte), onDisconnect func()) *Listener {
	l := &Listener{
		ch:           make(chan []byte, 64),
		done:         make(chan struct{}),
		OnDisconnect: onDisconnect,
	}
	// Goroutine drains the channel and calls onOutput outside any lock
	go func() {
		for {
			select {
			case data, ok := <-l.ch:
				if !ok {
					return
				}
				onOutput(data)
			case <-l.done:
				return
			}
		}
	}()
	return l
}

// Send queues data for the listener. Non-blocking: drops if buffer full.
func (l *Listener) Send(data []byte) {
	select {
	case l.ch <- data:
	default:
		// drop if buffer full — better than blocking all listeners
	}
}

// Close stops the listener goroutine.
func (l *Listener) Close() {
	select {
	case <-l.done:
	default:
		close(l.done)
	}
}

// Container represents a project container with an optional PTY session.
type Container struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Connected bool      `json:"connected"`
	CreatedAt time.Time `json:"createdAt"`

	mu        sync.Mutex
	session   *ptypkg.Session
	logFile   *os.File
	listeners map[*Listener]bool
}

// AddListener registers a listener.
func (c *Container) AddListener(l *Listener) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.listeners == nil {
		c.listeners = make(map[*Listener]bool)
	}
	c.listeners[l] = true
}

// RemoveListener unregisters and closes a listener.
func (c *Container) RemoveListener(l *Listener) {
	c.mu.Lock()
	delete(c.listeners, l)
	c.mu.Unlock()
	l.Close()
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
		listeners: make(map[*Listener]bool),
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
		listeners: make(map[*Listener]bool),
	}

	m.mu.Lock()
	m.containers[id] = c
	m.mu.Unlock()

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

// pumpOutput reads from PTY and broadcasts to all listeners + log file.
// IMPORTANT: callbacks are called OUTSIDE the lock to prevent deadlocks.
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
			data := make([]byte, n)
			copy(data, buf[:n])

			// Write to log file under lock
			c.mu.Lock()
			if c.logFile != nil {
				c.logFile.Write(data)
			}
			// Copy listener list under lock
			listeners := make([]*Listener, 0, len(c.listeners))
			for l := range c.listeners {
				listeners = append(listeners, l)
			}
			c.mu.Unlock()

			// Send to listeners OUTSIDE the lock via non-blocking channel
			for _, l := range listeners {
				l.Send(data)
			}
		}
		if err != nil {
			break
		}
	}

	// Process exited — mark disconnected, notify all listeners
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
	listeners := make([]*Listener, 0, len(c.listeners))
	for l := range c.listeners {
		listeners = append(listeners, l)
	}
	c.mu.Unlock()

	for _, l := range listeners {
		if l.OnDisconnect != nil {
			l.OnDisconnect()
		}
	}
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

// ReadHistory reads the last portion of terminal output log (max 64KB).
func (m *Manager) ReadHistory(id string) ([]byte, error) {
	const maxBytes = 16 * 1024

	f, err := os.Open(m.terminalLogPath(id))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}

	size := info.Size()
	if size <= maxBytes {
		return os.ReadFile(m.terminalLogPath(id))
	}

	// Read only the tail
	buf := make([]byte, maxBytes)
	_, err = f.ReadAt(buf, size-maxBytes)
	if err != nil {
		return nil, err
	}
	return buf, nil
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
