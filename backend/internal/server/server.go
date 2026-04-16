package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/penguin/agent-hive/internal/auth"
	"github.com/penguin/agent-hive/internal/container"
	"github.com/penguin/agent-hive/internal/static"
	"github.com/penguin/agent-hive/internal/store"
	"github.com/penguin/agent-hive/internal/ws"
)

// New creates the HTTP handler with container and todo APIs.
func New(devMode bool, mgr *container.Manager, db *store.Store, am *auth.Manager) http.Handler {
	mux := http.NewServeMux()

	// Auth API
	mux.HandleFunc("/api/auth/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		handleLogin(am, w, r)
	})
	mux.HandleFunc("/api/auth/check", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		handleAuthCheck(am, w, r)
	})

	// Event broadcast WebSocket
	mux.HandleFunc("/ws/notify", ws.HandleNotify(am))

	// Terminal WebSocket
	mux.HandleFunc("/ws/terminal", ws.HandleTerminal(mgr))

	// Container REST API
	mux.HandleFunc("/api/containers", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			listContainers(mgr, w)
		case http.MethodPost:
			createContainer(mgr, db, w, r)
			am.Broadcast([]byte(`{"type":"containers-changed"}`))
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/containers/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path[len("/api/containers/"):]
		if path == "" {
			http.Error(w, "missing container id", http.StatusBadRequest)
			return
		}

		// /api/containers/{id}/reopen
		if strings.HasSuffix(path, "/reopen") && r.Method == http.MethodPost {
			id := strings.TrimSuffix(path, "/reopen")
			reopenContainer(mgr, id, w)
			return
		}

		// /api/containers/{id}/terminals[/{tid}[/has-process]]
		parts := strings.SplitN(path, "/", 3)
		if len(parts) >= 2 && parts[1] == "terminals" {
			containerID := parts[0]
			broadcastTerminalChange := func() {
				am.Broadcast([]byte(`{"type":"terminals-changed","containerId":"` + containerID + `"}`))
			}

			if len(parts) == 2 {
				// /api/containers/{id}/terminals
				switch r.Method {
				case http.MethodGet:
					listTerminals(mgr, containerID, w)
				case http.MethodPost:
					createTerminalHandler(mgr, containerID, w)
					broadcastTerminalChange()
				default:
					http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				}
				return
			}

			// /api/containers/{id}/terminals/{tid}[/has-process]
			tidPath := parts[2]
			if strings.HasSuffix(tidPath, "/has-process") && r.Method == http.MethodGet {
				tid := strings.TrimSuffix(tidPath, "/has-process")
				hasProcessHandler(mgr, containerID, tid, w)
				return
			}

			tid := tidPath
			switch r.Method {
			case http.MethodDelete:
				deleteTerminalHandler(mgr, containerID, tid, w)
				broadcastTerminalChange()
			default:
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			}
			return
		}

		id := parts[0]
		switch r.Method {
		case http.MethodDelete:
			deleteContainer(mgr, db, id, w)
			am.Broadcast([]byte(`{"type":"containers-changed"}`))
		case http.MethodPatch:
			renameContainer(mgr, db, id, w, r)
			am.Broadcast([]byte(`{"type":"containers-changed"}`))
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Layout REST API
	mux.HandleFunc("/api/layout", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			getLayout(db, w)
		case http.MethodPut:
			updateLayout(db, w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Mobile Layout REST API
	mux.HandleFunc("/api/mobile-layout", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			getMobileLayout(db, w)
		case http.MethodPut:
			updateMobileLayout(db, w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Todo REST API: /api/todos/{containerID}[/{todoID}]
	mux.HandleFunc("/api/todos/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/todos/")
		parts := strings.SplitN(path, "/", 2)
		containerID := parts[0]
		if containerID == "" {
			http.Error(w, "missing container id", http.StatusBadRequest)
			return
		}

		broadcastTodoChange := func() {
			am.Broadcast([]byte(`{"type":"todos-updated","containerId":"` + containerID + `"}`))
		}

		if len(parts) == 1 || parts[1] == "" {
			switch r.Method {
			case http.MethodGet:
				listTodos(db, containerID, w)
			case http.MethodPost:
				createTodo(db, containerID, w, r)
				broadcastTodoChange()
			default:
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			}
			return
		}

		todoPath := parts[1]

		if todoPath == "reorder" && r.Method == http.MethodPut {
			reorderTodos(db, w, r)
			broadcastTodoChange()
			return
		}

		todoID, err := strconv.ParseInt(todoPath, 10, 64)
		if err != nil {
			http.Error(w, "invalid todo id", http.StatusBadRequest)
			return
		}

		switch r.Method {
		case http.MethodPatch:
			updateTodo(db, todoID, w, r)
			broadcastTodoChange()
		case http.MethodDelete:
			deleteTodo(db, todoID, w)
			broadcastTodoChange()
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	if devMode {
		viteURL, _ := url.Parse("http://localhost:5173")
		proxy := httputil.NewSingleHostReverseProxy(viteURL)
		mux.Handle("/", proxy)
	} else {
		mux.Handle("/", static.Handler())
	}

	return am.Middleware(mux)
}

// --- Auth handlers ---

type loginReq struct {
	Password  string `json:"password"`
	MachineID string `json:"machineId"`
}

func handleLogin(am *auth.Manager, w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	token, err := am.Login(req.Password, req.MachineID)
	if err != nil {
		status := http.StatusUnauthorized
		if err == auth.ErrMachineNotAllowed {
			status = http.StatusForbidden
		}
		http.Error(w, err.Error(), status)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": token})
}

func handleAuthCheck(am *auth.Manager, w http.ResponseWriter, r *http.Request) {
	if !am.Enabled() {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"enabled": false})
		return
	}
	token := r.URL.Query().Get("token")
	if token == "" {
		token = r.Header.Get("X-Auth-Token")
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"enabled": true,
		"valid":   am.ValidateToken(token),
	})
}

// --- Container handlers ---

type createContainerReq struct {
	Name string `json:"name"`
}

func createContainer(mgr *container.Manager, db *store.Store, w http.ResponseWriter, r *http.Request) {
	var req createContainerReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		req.Name = "New Project"
	}

	c, err := mgr.Create(req.Name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Persist metadata and layout
	_ = db.SaveContainer(c.ID, c.Name)
	page, pos, _ := db.NextAvailableSlot()
	_ = db.AddLayoutEntry(c.ID, page, pos)
	_ = db.AddMobileLayoutEntry(c.ID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(c)
}

func listContainers(mgr *container.Manager, w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(mgr.List())
}

func deleteContainer(mgr *container.Manager, db *store.Store, id string, w http.ResponseWriter) {
	if !mgr.Delete(id) {
		http.Error(w, "container not found", http.StatusNotFound)
		return
	}
	_ = db.DeleteContainerMeta(id)
	_ = db.DeleteTodosByContainer(id)
	_ = db.DeleteTerminalsByContainer(id)
	_ = db.RemoveLayoutEntry(id)
	_ = db.RemoveMobileLayoutEntry(id)
	w.WriteHeader(http.StatusNoContent)
}

type renameReq struct {
	Name string `json:"name"`
}

func renameContainer(mgr *container.Manager, db *store.Store, id string, w http.ResponseWriter, r *http.Request) {
	var req renameReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if !mgr.Rename(id, req.Name) {
		http.Error(w, "container not found", http.StatusNotFound)
		return
	}
	_ = db.RenameContainer(id, req.Name)
	w.WriteHeader(http.StatusNoContent)
}

func reopenContainer(mgr *container.Manager, id string, w http.ResponseWriter) {
	if err := mgr.Reopen(id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Todo handlers ---

func listTodos(db *store.Store, containerID string, w http.ResponseWriter) {
	todos, err := db.ListTodos(containerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if todos == nil {
		todos = []store.Todo{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(todos)
}

type createTodoReq struct {
	Content string `json:"content"`
}

func createTodo(db *store.Store, containerID string, w http.ResponseWriter, r *http.Request) {
	var req createTodoReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	todo, err := db.CreateTodo(containerID, req.Content)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(todo)
}

type updateTodoReq struct {
	Content *string `json:"content"`
	Done    *bool   `json:"done"`
}

func updateTodo(db *store.Store, todoID int64, w http.ResponseWriter, r *http.Request) {
	var req updateTodoReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// We need to get current values for fields not being updated.
	// For simplicity, require both fields (frontend always sends both).
	content := ""
	done := false
	if req.Content != nil {
		content = *req.Content
	}
	if req.Done != nil {
		done = *req.Done
	}

	if err := db.UpdateTodo(todoID, content, done); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func deleteTodo(db *store.Store, todoID int64, w http.ResponseWriter) {
	if err := db.DeleteTodo(todoID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type reorderReq struct {
	IDs []int64 `json:"ids"`
}

func reorderTodos(db *store.Store, w http.ResponseWriter, r *http.Request) {
	var req reorderReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := db.ReorderTodos(req.IDs); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Layout handlers ---

func getLayout(db *store.Store, w http.ResponseWriter) {
	entries, err := db.GetLayout()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if entries == nil {
		entries = []store.LayoutEntry{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

func updateLayout(db *store.Store, w http.ResponseWriter, r *http.Request) {
	var entries []store.LayoutEntry
	if err := json.NewDecoder(r.Body).Decode(&entries); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := db.SetLayout(entries); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Mobile Layout handlers ---

func getMobileLayout(db *store.Store, w http.ResponseWriter) {
	entries, err := db.GetMobileLayout()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if entries == nil {
		entries = []store.MobileLayoutEntry{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

func updateMobileLayout(db *store.Store, w http.ResponseWriter, r *http.Request) {
	var entries []store.MobileLayoutEntry
	if err := json.NewDecoder(r.Body).Decode(&entries); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := db.SetMobileLayout(entries); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Terminal handlers ---

type terminalResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	IsDefault bool   `json:"isDefault"`
	Connected bool   `json:"connected"`
}

func listTerminals(mgr *container.Manager, containerID string, w http.ResponseWriter) {
	c, ok := mgr.Get(containerID)
	if !ok {
		http.Error(w, "container not found", http.StatusNotFound)
		return
	}

	terms := c.ListTerminals()
	resp := make([]terminalResponse, 0, len(terms))
	for _, t := range terms {
		resp = append(resp, terminalResponse{
			ID:        t.ID,
			Name:      t.Name,
			IsDefault: t.IsDefault,
			Connected: t.Connected,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func createTerminalHandler(mgr *container.Manager, containerID string, w http.ResponseWriter) {
	term, err := mgr.CreateTerminal(containerID)
	if err != nil {
		switch {
		case errors.Is(err, container.ErrTerminalLimit):
			http.Error(w, err.Error(), http.StatusBadRequest)
		case errors.Is(err, container.ErrContainerNotFound):
			http.Error(w, err.Error(), http.StatusNotFound)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(terminalResponse{
		ID:        term.ID,
		Name:      term.Name,
		IsDefault: term.IsDefault,
		Connected: term.Connected,
	})
}

func deleteTerminalHandler(mgr *container.Manager, containerID, terminalID string, w http.ResponseWriter) {
	err := mgr.DeleteTerminal(containerID, terminalID)
	if err != nil {
		switch {
		case errors.Is(err, container.ErrDefaultTerminal):
			http.Error(w, err.Error(), http.StatusBadRequest)
		case errors.Is(err, container.ErrContainerNotFound), errors.Is(err, container.ErrTerminalNotFound):
			http.Error(w, err.Error(), http.StatusNotFound)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func hasProcessHandler(mgr *container.Manager, containerID, terminalID string, w http.ResponseWriter) {
	c, ok := mgr.Get(containerID)
	if !ok {
		http.Error(w, "container not found", http.StatusNotFound)
		return
	}

	term, ok := c.GetTerminal(terminalID)
	if !ok {
		http.Error(w, "terminal not found", http.StatusNotFound)
		return
	}

	hasChild := HasChildProcess(term.ProcessPID())
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"hasProcess": hasChild})
}

// HasChildProcess checks if a process has child processes by reading /proc/{pid}/children.
func HasChildProcess(pid int) bool {
	if pid <= 0 {
		return false
	}
	// Try /proc/{pid}/task/{pid}/children first (Linux-specific)
	childrenPath := fmt.Sprintf("/proc/%d/task/%d/children", pid, pid)
	data, err := os.ReadFile(childrenPath)
	if err != nil {
		// Fallback: if we can't read, assume process has children (safer to confirm)
		return true
	}
	return len(strings.TrimSpace(string(data))) > 0
}
