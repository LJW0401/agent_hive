package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
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

// HandleTerminal creates a handler that connects a WebSocket to a container's PTY.
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

		// Send terminal history first
		history, err := mgr.ReadHistory(containerID)
		if err == nil && len(history) > 0 {
			writeMsg(websocket.BinaryMessage, history)
		}

		// If disconnected, send status and close
		if !c.Connected {
			writeMsg(websocket.TextMessage, []byte(`{"type":"status","connected":false}`))
			conn.Close()
			return
		}

		// Set up output callback: PTY output -> WebSocket
		done := make(chan struct{})
		c.SetCallbacks(
			func(data []byte) {
				if err := writeMsg(websocket.BinaryMessage, data); err != nil {
					log.Printf("ws write error: %v", err)
				}
			},
			func() {
				// Terminal process exited
				writeMsg(websocket.TextMessage, []byte(`{"type":"status","connected":false}`))
				conn.Close()
				close(done)
			},
		)

		defer func() {
			c.ClearCallbacks()
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
					if err := c.ResizePTY(resize.Rows, resize.Cols); err != nil {
						log.Printf("pty resize error: %v", err)
					}
					continue
				}
			}

			if _, err := c.WriteToPTY(msg); err != nil {
				log.Printf("pty write error: %v", err)
				return
			}
		}
	}
}
