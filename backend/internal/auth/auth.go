package auth

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Session represents an authenticated device session.
type Session struct {
	Token     string    `json:"token"`
	MachineID string    `json:"machineId,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	ReadOnly  bool      `json:"readOnly"`
}

// Manager handles authentication and device session tracking.
type Manager struct {
	mu       sync.RWMutex
	password string
	machines []string

	activeSession *Session

	// All WebSocket connections for the active session (notify + terminal)
	notifyMu sync.Mutex
	notifyWS map[*websocket.Conn]bool
}

// NewManager creates an auth manager.
func NewManager(password string, machines []string) *Manager {
	return &Manager{
		password: password,
		machines: machines,
		notifyWS: make(map[*websocket.Conn]bool),
	}
}

// Enabled returns whether authentication is configured.
func (m *Manager) Enabled() bool {
	return m.password != ""
}

// Login validates credentials and creates a session.
func (m *Manager) Login(password, machineID string) (*Session, error) {
	if m.password != "" && password != m.password {
		return nil, ErrInvalidPassword
	}

	if len(m.machines) > 0 && machineID != "" {
		allowed := false
		for _, mid := range m.machines {
			if mid == machineID {
				allowed = true
				break
			}
		}
		if !allowed {
			return nil, ErrMachineNotAllowed
		}
	}

	token := generateToken()
	session := &Session{
		Token:     token,
		MachineID: machineID,
		CreatedAt: time.Now(),
	}

	m.mu.Lock()
	hadOld := m.activeSession != nil
	m.activeSession = session
	m.mu.Unlock()

	if hadOld {
		m.notifyAllPreempted()
	}

	return session, nil
}

// Claim promotes an existing token to the active session, preempting others.
// Used when a device refreshes and wants to re-take control.
func (m *Manager) Claim(token string) bool {
	m.mu.Lock()
	if m.activeSession != nil && m.activeSession.Token == token {
		m.mu.Unlock()
		return true // already active
	}
	// Set this token as active
	m.activeSession = &Session{
		Token:     token,
		CreatedAt: time.Now(),
	}
	m.mu.Unlock()

	m.notifyAllPreempted()
	return true
}

// Validate checks if a session token is valid and returns whether it's read-only.
func (m *Manager) Validate(token string) (readOnly bool, ok bool) {
	if !m.Enabled() {
		return false, true
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.activeSession == nil {
		return false, false
	}

	if m.activeSession.Token == token {
		return false, true
	}

	// Token doesn't match active — it's been preempted, allow read-only
	return true, true
}

// RegisterNotifyWS registers a WebSocket to receive session notifications.
func (m *Manager) RegisterNotifyWS(conn *websocket.Conn) {
	m.notifyMu.Lock()
	m.notifyWS[conn] = true
	m.notifyMu.Unlock()
}

// UnregisterNotifyWS removes a WebSocket from notifications.
func (m *Manager) UnregisterNotifyWS(conn *websocket.Conn) {
	m.notifyMu.Lock()
	delete(m.notifyWS, conn)
	m.notifyMu.Unlock()
}

// Broadcast sends a message to all registered notify WebSockets (without closing them).
func (m *Manager) Broadcast(message []byte) {
	m.notifyMu.Lock()
	conns := make([]*websocket.Conn, 0, len(m.notifyWS))
	for c := range m.notifyWS {
		conns = append(conns, c)
	}
	m.notifyMu.Unlock()

	for _, c := range conns {
		c.WriteMessage(websocket.TextMessage, message)
	}
}

// notifyAllPreempted sends preemption message to all registered WebSockets and clears them.
func (m *Manager) notifyAllPreempted() {
	m.notifyMu.Lock()
	conns := make([]*websocket.Conn, 0, len(m.notifyWS))
	for c := range m.notifyWS {
		conns = append(conns, c)
	}
	m.notifyWS = make(map[*websocket.Conn]bool)
	m.notifyMu.Unlock()

	for _, c := range conns {
		c.WriteMessage(websocket.TextMessage, []byte(`{"type":"preempted"}`))
		c.Close()
	}
}

func generateToken() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Errors
type AuthError string

func (e AuthError) Error() string { return string(e) }

const (
	ErrInvalidPassword   AuthError = "invalid password"
	ErrMachineNotAllowed AuthError = "machine not allowed"
)

// Middleware returns an HTTP middleware that enforces authentication.
func (m *Manager) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/auth/login" || r.URL.Path == "/api/auth/check" || r.URL.Path == "/api/auth/claim" {
			next.ServeHTTP(w, r)
			return
		}

		if !m.Enabled() {
			next.ServeHTTP(w, r)
			return
		}

		if !isAPIorWS(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		token := r.URL.Query().Get("token")
		if token == "" {
			token = r.Header.Get("X-Auth-Token")
		}

		_, ok := m.Validate(token)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func isAPIorWS(path string) bool {
	return len(path) >= 4 && (path[:4] == "/api" || path[:3] == "/ws")
}
