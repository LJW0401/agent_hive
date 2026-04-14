package static

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed dist/*
var embeddedFS embed.FS

// Handler returns an http.Handler that serves the embedded frontend files.
func Handler() http.Handler {
	sub, err := fs.Sub(embeddedFS, "dist")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// No cache for all resources (binary is rebuilt on each deploy)
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

		if path == "/" {
			fileServer.ServeHTTP(w, r)
			return
		}

		f, err := sub.Open(path[1:])
		if err != nil {
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
			return
		}
		f.Close()
		fileServer.ServeHTTP(w, r)
	})
}
