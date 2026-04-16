package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/penguin/agent-hive/internal/auth"
	"github.com/penguin/agent-hive/internal/container"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type resizeMsg struct {
	Type string `json:"type"`
	Rows uint16 `json:"rows"`
	Cols uint16 `json:"cols"`
}

// HandleNotify creates a handler for event broadcasts (todo sync, etc.).
func HandleNotify(am *auth.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("notify ws upgrade error: %v", err)
			return
		}

		am.RegisterNotifyWS(conn)
		defer am.UnregisterNotifyWS(conn)

		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				break
			}
		}
	}
}

// HandleTerminal connects a WebSocket to a container's terminal PTY.
// Query params: id (container ID, required), tid (terminal ID, optional — defaults to default terminal).
func HandleTerminal(mgr *container.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		containerID := r.URL.Query().Get("id")
		if containerID == "" {
			http.Error(w, "missing container id", http.StatusBadRequest)
			return
		}

		c, ok := mgr.Get(containerID)
		if !ok {
			http.Error(w, "container not found", http.StatusNotFound)
			return
		}

		// Resolve terminal
		terminalID := r.URL.Query().Get("tid")
		var term *container.Terminal
		if terminalID != "" {
			term, ok = c.GetTerminal(terminalID)
			if !ok {
				http.Error(w, "terminal not found", http.StatusNotFound)
				return
			}
		} else {
			// Default terminal (backward compat)
			term = c.GetDefaultTerminal()
			if term == nil {
				http.Error(w, "no default terminal", http.StatusNotFound)
				return
			}
			terminalID = term.ID
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("websocket upgrade error: %v", err)
			return
		}

		var wsMu sync.Mutex
		writeMsg := func(msgType int, data []byte) error {
			wsMu.Lock()
			defer wsMu.Unlock()
			return conn.WriteMessage(msgType, data)
		}

		// Send terminal history
		history, err := mgr.ReadHistory(containerID, terminalID)
		if err == nil && len(history) > 0 {
			writeMsg(websocket.BinaryMessage, history)
		}

		// If terminal disconnected, send status and close
		if !term.Connected {
			writeMsg(websocket.TextMessage, []byte(`{"type":"status","connected":false}`))
			conn.Close()
			return
		}

		// Register listener on the terminal (not container)
		listener := container.NewListener(
			func(data []byte) {
				if err := writeMsg(websocket.BinaryMessage, data); err != nil {
					log.Printf("ws write error: %v", err)
				}
			},
			func() {
				writeMsg(websocket.TextMessage, []byte(`{"type":"status","connected":false}`))
				conn.Close()
			},
		)
		term.AddListener(listener)

		defer func() {
			term.RemoveListener(listener)
			conn.Close()
		}()

		// WebSocket -> PTY
		for {
			msgType, msg, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
					log.Printf("ws read error: %v", err)
				}
				return
			}

			if msgType == websocket.TextMessage {
				var resize resizeMsg
				if err := json.Unmarshal(msg, &resize); err == nil && resize.Type == "resize" {
					if err := term.ResizePTY(resize.Rows, resize.Cols); err != nil {
						log.Printf("pty resize error: %v", err)
					}
					continue
				}
			}

			if _, err := term.WriteToPTY(msg); err != nil {
				log.Printf("pty write error: %v", err)
				return
			}
		}
	}
}
