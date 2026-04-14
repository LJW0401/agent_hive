package auth

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// Manager handles password authentication and event broadcasting.
type Manager struct {
	password string
	machines []string

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

// Login validates password and optional machine whitelist.
func (m *Manager) Login(password, machineID string) (string, error) {
	if m.password != "" && password != m.password {
		return "", ErrInvalidPassword
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
			return "", ErrMachineNotAllowed
		}
	}

	return generateToken(), nil
}

// ValidateToken checks if a token is non-empty (any logged-in token is valid).
func (m *Manager) ValidateToken(token string) bool {
	return token != ""
}

// RegisterNotifyWS registers a WebSocket for event broadcasts.
func (m *Manager) RegisterNotifyWS(conn *websocket.Conn) {
	m.notifyMu.Lock()
	m.notifyWS[conn] = true
	m.notifyMu.Unlock()
}

// UnregisterNotifyWS removes a WebSocket.
func (m *Manager) UnregisterNotifyWS(conn *websocket.Conn) {
	m.notifyMu.Lock()
	delete(m.notifyWS, conn)
	m.notifyMu.Unlock()
}

// Broadcast sends a message to all registered notify WebSockets.
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

func generateToken() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

type AuthError string

func (e AuthError) Error() string { return string(e) }

const (
	ErrInvalidPassword   AuthError = "invalid password"
	ErrMachineNotAllowed AuthError = "machine not allowed"
)

// Middleware enforces authentication on API/WS routes.
func (m *Manager) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth endpoints
		if r.URL.Path == "/api/auth/login" || r.URL.Path == "/api/auth/check" {
			next.ServeHTTP(w, r)
			return
		}

		if !m.Enabled() {
			next.ServeHTTP(w, r)
			return
		}

		// Skip static resources
		if !isAPIorWS(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		token := r.URL.Query().Get("token")
		if token == "" {
			token = r.Header.Get("X-Auth-Token")
		}

		if !m.ValidateToken(token) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func isAPIorWS(path string) bool {
	return len(path) >= 4 && (path[:4] == "/api" || path[:3] == "/ws")
}
