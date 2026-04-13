package ws

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/penguin/agent-hive/internal/container"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// resizeMsg is sent from the client to resize the terminal.
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
		defer conn.Close()

		session := c.Session()

		// PTY -> WebSocket
		go func() {
			buf := make([]byte, 4096)
			for {
				n, err := session.Read(buf)
				if err != nil {
					if err != io.EOF {
						log.Printf("pty read error: %v", err)
					}
					conn.WriteMessage(websocket.CloseMessage,
						websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
					return
				}
				if err := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
					log.Printf("ws write error: %v", err)
					return
				}
			}
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
					if err := session.Resize(resize.Rows, resize.Cols); err != nil {
						log.Printf("pty resize error: %v", err)
					}
					continue
				}
			}

			if _, err := session.Write(msg); err != nil {
				log.Printf("pty write error: %v", err)
				return
			}
		}
	}
}
