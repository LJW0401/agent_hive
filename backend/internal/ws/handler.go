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

		// Disconnected terminals never have pumpOutput running, so there is no
		// live stream — just send history (may be large) and close. The race
		// that SubscribeWithSnapshot solves only exists for connected streams.
		if !term.Connected {
			history, _ := mgr.ReadHistory(containerID, terminalID)
			if len(history) > 0 {
				bundle := make([]byte, 0, len(history)+2)
				bundle = append(bundle, 0x1b, 'c')
				bundle = append(bundle, history...)
				writeMsg(websocket.BinaryMessage, bundle)
			}
			writeMsg(websocket.TextMessage, []byte(`{"type":"status","connected":false}`))
			conn.Close()
			return
		}

		// Atomic snapshot + subscribe. This closes the race in which large
		// replay bundles (up to 10MB after the TUI anchor fix) leave a
		// hundreds-of-millisecond window between "history byte captured" and
		// "listener attached" — pumpOutput writes during that window were
		// silently dropped and the user saw replay truncate at the running
		// command.
		//
		// Listener callbacks start buffering into `pending` until the history
		// bundle has been fully sent, then we drain `pending` in order and
		// flip `historySent` so subsequent callbacks write through directly.
		// This keeps the wire format ordered: history first, then live bytes.
		var pendingMu sync.Mutex
		var pending [][]byte
		historySent := false

		history, listener, err := mgr.SubscribeWithSnapshot(
			containerID,
			terminalID,
			func(data []byte) {
				pendingMu.Lock()
				if historySent {
					pendingMu.Unlock()
					if werr := writeMsg(websocket.BinaryMessage, data); werr != nil {
						log.Printf("ws write error: %v", werr)
					}
					return
				}
				// Copy — the caller owns `data` only for this call.
				pending = append(pending, append([]byte(nil), data...))
				pendingMu.Unlock()
			},
			func() {
				writeMsg(websocket.TextMessage, []byte(`{"type":"status","connected":false}`))
				conn.Close()
			},
		)
		if err != nil {
			log.Printf("subscribe error: %v", err)
			conn.Close()
			return
		}

		defer func() {
			term.RemoveListener(listener)
			conn.Close()
		}()

		// Send terminal history. We prepend \x1bc (RIS, full terminal reset) so
		// xterm.js starts from a clean state: pending SGR / alt-screen / charset
		// flags from a prior partial connection can't bleed into the replay and
		// produce a corrupted scrollback.
		if len(history) > 0 {
			bundle := make([]byte, 0, len(history)+2)
			bundle = append(bundle, 0x1b, 'c')
			bundle = append(bundle, history...)
			writeMsg(websocket.BinaryMessage, bundle)
		}

		// Drain any live bytes that arrived while we were sending history.
		// Loop until the pending queue is empty, then flip historySent; after
		// the flip, listener writes bypass the queue.
		for {
			pendingMu.Lock()
			if len(pending) == 0 {
				historySent = true
				pendingMu.Unlock()
				break
			}
			batch := pending
			pending = nil
			pendingMu.Unlock()
			for _, b := range batch {
				if werr := writeMsg(websocket.BinaryMessage, b); werr != nil {
					log.Printf("ws write error: %v", werr)
					break
				}
			}
		}

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
