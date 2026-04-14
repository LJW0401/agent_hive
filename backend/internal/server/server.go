package server

import (
	"encoding/json"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"

	"github.com/penguin/agent-hive/internal/container"
	"github.com/penguin/agent-hive/internal/store"
	"github.com/penguin/agent-hive/internal/ws"
)

// New creates the HTTP handler with container and todo APIs.
func New(devMode bool, mgr *container.Manager, db *store.Store) http.Handler {
	mux := http.NewServeMux()

	// WebSocket endpoint for terminal (per container)
	mux.HandleFunc("/ws/terminal", ws.HandleTerminal(mgr))

	// Container REST API
	mux.HandleFunc("/api/containers", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			listContainers(mgr, w)
		case http.MethodPost:
			createContainer(mgr, db, w, r)
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

		id := path
		switch r.Method {
		case http.MethodDelete:
			deleteContainer(mgr, db, id, w)
		case http.MethodPatch:
			renameContainer(mgr, db, id, w, r)
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

	// Todo REST API: /api/todos/{containerID}[/{todoID}]
	mux.HandleFunc("/api/todos/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/todos/")
		parts := strings.SplitN(path, "/", 2)
		containerID := parts[0]
		if containerID == "" {
			http.Error(w, "missing container id", http.StatusBadRequest)
			return
		}

		if len(parts) == 1 || parts[1] == "" {
			// /api/todos/{containerID}
			switch r.Method {
			case http.MethodGet:
				listTodos(db, containerID, w)
			case http.MethodPost:
				createTodo(db, containerID, w, r)
			default:
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			}
			return
		}

		todoPath := parts[1]

		// /api/todos/{containerID}/reorder
		if todoPath == "reorder" && r.Method == http.MethodPut {
			reorderTodos(db, w, r)
			return
		}

		// /api/todos/{containerID}/{todoID}
		todoID, err := strconv.ParseInt(todoPath, 10, 64)
		if err != nil {
			http.Error(w, "invalid todo id", http.StatusBadRequest)
			return
		}

		switch r.Method {
		case http.MethodPatch:
			updateTodo(db, todoID, w, r)
		case http.MethodDelete:
			deleteTodo(db, todoID, w)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	if devMode {
		viteURL, _ := url.Parse("http://localhost:5173")
		proxy := httputil.NewSingleHostReverseProxy(viteURL)
		mux.Handle("/", proxy)
	} else {
		mux.Handle("/", http.FileServer(http.Dir("../frontend/dist")))
	}

	return mux
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
	_ = db.RemoveLayoutEntry(id)
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
