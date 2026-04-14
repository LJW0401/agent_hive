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

// HandleNotify creates a handler for session-level notifications (preemption).
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

// HandleTerminal creates a handler that connects a WebSocket to a container's PTY.
func HandleTerminal(mgr *container.Manager, am *auth.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		containerID := r.URL.Query().Get("id")
		if containerID == "" {
			http.Error(w, "missing container id", http.StatusBadRequest)
			return
		}

		// Check auth and read-only status
		readOnly := false
		if am.Enabled() {
			token := r.URL.Query().Get("token")
			var ok bool
			readOnly, ok = am.Validate(token)
			if !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}

		c, ok := mgr.Get(containerID)
		if !ok {
			http.Error(w, "container not found", http.StatusNotFound)
			return
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
		history, err := mgr.ReadHistory(containerID)
		if err == nil && len(history) > 0 {
			writeMsg(websocket.BinaryMessage, history)
		}

		// Send read-only status
		if readOnly {
			writeMsg(websocket.TextMessage, []byte(`{"type":"readonly","readOnly":true}`))
		}

		// If terminal disconnected, send status and close
		if !c.Connected {
			writeMsg(websocket.TextMessage, []byte(`{"type":"status","connected":false}`))
			conn.Close()
			return
		}

		// Register listener: PTY output -> this WebSocket
		listener := &container.Listener{
			OnOutput: func(data []byte) {
				if err := writeMsg(websocket.BinaryMessage, data); err != nil {
					log.Printf("ws write error: %v", err)
				}
			},
			OnDisconnect: func() {
				writeMsg(websocket.TextMessage, []byte(`{"type":"status","connected":false}`))
				conn.Close()
			},
		}
		c.AddListener(listener)

		defer func() {
			c.RemoveListener(listener)
			conn.Close()
		}()

		// WebSocket -> PTY (skip writes in read-only mode)
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
					if !readOnly {
						if err := c.ResizePTY(resize.Rows, resize.Cols); err != nil {
							log.Printf("pty resize error: %v", err)
						}
					}
					continue
				}
			}

			if readOnly {
				continue
			}

			if _, err := c.WriteToPTY(msg); err != nil {
				log.Printf("pty write error: %v", err)
				return
			}
		}
	}
}
