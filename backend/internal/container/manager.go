package container

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	ptypkg "github.com/penguin/agent-hive/internal/pty"
)

// Container represents a project container with a PTY session.
type Container struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
	session   *ptypkg.Session
}

// Session returns the PTY session for this container.
func (c *Container) Session() *ptypkg.Session {
	return c.session
}

// Manager manages multiple containers.
type Manager struct {
	mu         sync.RWMutex
	containers map[string]*Container
	nextID     atomic.Int64
}

// NewManager creates a new container manager.
func NewManager() *Manager {
	return &Manager{
		containers: make(map[string]*Container),
	}
}

// Create creates a new container with a PTY session.
func (m *Manager) Create(name string) (*Container, error) {
	session, err := ptypkg.NewSession()
	if err != nil {
		return nil, err
	}

	id := fmt.Sprintf("c-%d", m.nextID.Add(1))

	c := &Container{
		ID:        id,
		Name:      name,
		CreatedAt: time.Now(),
		session:   session,
	}

	m.mu.Lock()
	m.containers[id] = c
	m.mu.Unlock()

	// Watch for process exit and clean up
	go func() {
		_ = session.Wait()
	}()

	return c, nil
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

	_ = c.session.Close()
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
