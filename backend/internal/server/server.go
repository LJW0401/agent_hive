package server

import (
	"encoding/json"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/penguin/agent-hive/internal/container"
	"github.com/penguin/agent-hive/internal/ws"
)

// New creates the HTTP handler with container management APIs.
func New(devMode bool, mgr *container.Manager) http.Handler {
	mux := http.NewServeMux()

	// WebSocket endpoint for terminal (per container)
	mux.HandleFunc("/ws/terminal", ws.HandleTerminal(mgr))

	// REST API
	mux.HandleFunc("/api/containers", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			listContainers(mgr, w, r)
		case http.MethodPost:
			createContainer(mgr, w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/containers/", func(w http.ResponseWriter, r *http.Request) {
		// Extract container ID from path: /api/containers/{id}
		id := r.URL.Path[len("/api/containers/"):]
		if id == "" {
			http.Error(w, "missing container id", http.StatusBadRequest)
			return
		}

		switch r.Method {
		case http.MethodDelete:
			deleteContainer(mgr, id, w)
		case http.MethodPatch:
			renameContainer(mgr, id, w, r)
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

type createReq struct {
	Name string `json:"name"`
}

func createContainer(mgr *container.Manager, w http.ResponseWriter, r *http.Request) {
	var req createReq
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

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(c)
}

func listContainers(mgr *container.Manager, w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(mgr.List())
}

func deleteContainer(mgr *container.Manager, id string, w http.ResponseWriter) {
	if !mgr.Delete(id) {
		http.Error(w, "container not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type renameReq struct {
	Name string `json:"name"`
}

func renameContainer(mgr *container.Manager, id string, w http.ResponseWriter, r *http.Request) {
	var req renameReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if !mgr.Rename(id, req.Name) {
		http.Error(w, "container not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
