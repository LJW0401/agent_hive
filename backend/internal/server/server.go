package server

import (
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/penguin/agent-hive/internal/ws"
)

// New creates the HTTP handler. In dev mode it proxies to the Vite dev server.
func New(devMode bool) http.Handler {
	mux := http.NewServeMux()

	// WebSocket endpoint for terminal
	mux.HandleFunc("/ws/terminal", ws.HandleTerminal)

	if devMode {
		// Proxy all other requests to Vite dev server
		viteURL, _ := url.Parse("http://localhost:5173")
		proxy := httputil.NewSingleHostReverseProxy(viteURL)
		mux.Handle("/", proxy)
	} else {
		// Serve embedded static files (will be added in Phase 7)
		mux.Handle("/", http.FileServer(http.Dir("../frontend/dist")))
	}

	return mux
}
