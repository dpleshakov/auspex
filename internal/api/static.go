package api

import (
	"io/fs"
	"net/http"
	"strings"
)

// spaHandler serves the embedded React frontend.
// Requests for files that exist in the FS (assets, favicon, etc.) are served directly.
// All other paths fall through to index.html so the React router handles them.
type spaHandler struct {
	root   fs.FS
	server http.Handler
}

func newSPAHandler(root fs.FS) *spaHandler {
	return &spaHandler{
		root:   root,
		server: http.FileServer(http.FS(root)),
	}
}

func (h *spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Resolve path relative to the FS root (strip the leading /).
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}

	// If the file exists and is not a directory, serve it directly.
	f, err := h.root.Open(path)
	if err == nil {
		stat, statErr := f.Stat()
		_ = f.Close()
		if statErr == nil && !stat.IsDir() {
			h.server.ServeHTTP(w, r)
			return
		}
	}

	// SPA fallback: serve index.html and let the React router handle the path.
	r = r.Clone(r.Context())
	r.URL.Path = "/"
	h.server.ServeHTTP(w, r)
}
