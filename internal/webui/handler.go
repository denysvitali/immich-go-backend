// Package webui serves a static frontend (typically the Immich web build)
// from a configurable directory. The handler is intentionally permissive:
// if dir is empty or a file isn't found, requests fall through to the
// wrapped handler so API and websocket routes keep working.
//
// SPA fallback: GET requests for paths without a file extension that
// don't match a file on disk fall back to index.html, letting the
// frontend router take over (typical SvelteKit/Vite behaviour).
package webui

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Handler returns an http.Handler that serves files from dir. Unmatched
// requests are forwarded to next, except for extension-less GETs which
// fall back to index.html when present.
//
// When dir is empty, the handler is a transparent passthrough — useful
// for local development where the API is hit directly.
func Handler(dir string, next http.Handler) http.Handler {
	if strings.TrimSpace(dir) == "" {
		return next
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		absDir = dir
	}
	if info, err := os.Stat(absDir); err != nil || !info.IsDir() {
		// Frontend dir missing — fall through to API.
		return next
	}
	root := http.FileServer(http.Dir(absDir))
	indexPath := filepath.Join(absDir, "index.html")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only intercept GET / HEAD — everything else (POST/PUT/etc.)
		// belongs to the API.
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			next.ServeHTTP(w, r)
			return
		}

		// API routes must always pass through to the API mux. SPA
		// fallback to index.html otherwise turns /api/server/ping,
		// /api/server/config, ... into HTML responses, which causes
		// the SvelteKit client to spin its loading flower forever
		// waiting for JSON that never comes. Same for websocket
		// upgrade probes under /api/socket.io/.
		if isAPIPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		// Path traversal guard — must run on the raw URL path because
		// path.Clean resolves `..` segments away, hiding the attempt.
		if strings.Contains(r.URL.Path, "..") || strings.HasPrefix(r.URL.Path, "/.") {
			http.NotFound(w, r)
			return
		}

		clean := path.Clean(r.URL.Path)
		// Reject any path that escapes the root after cleaning.
		if !strings.HasPrefix(clean, "/") {
			http.NotFound(w, r)
			return
		}

		full := filepath.Join(absDir, clean)
		// Verify the resolved path is still under absDir — defense in depth
		// against symlinks or weird operating systems where filepath.Join
		// might allow escape.
		rel, err := filepath.Rel(absDir, full)
		if err != nil || strings.HasPrefix(rel, "..") || rel == ".." {
			http.NotFound(w, r)
			return
		}

		if info, err := os.Stat(full); err == nil && !info.IsDir() {
			root.ServeHTTP(w, r)
			return
		}

		// SPA fallback: serve index.html for extension-less GETs so the
		// frontend router takes over. Asset paths with extensions (e.g.
		// /assets/foo.css) get a 404 to surface real missing assets.
		if !hasExt(clean) {
			if _, err := os.Stat(indexPath); err == nil {
				http.ServeFile(w, r, indexPath)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func hasExt(p string) bool {
	ext := filepath.Ext(p)
	return ext != "" && ext != "."
}

// isAPIPath reports whether a URL path is owned by the backend REST /
// websocket API and must not be intercepted by the SPA fallback.
// /api/* (grpc-gateway) and the websocket upgrade paths under
// /api/socket.io/* would otherwise have index.html served in their
// place, breaking the frontend.
func isAPIPath(p string) bool {
	// Trim trailing slashes so /api and /api/ are treated identically;
	// http.FileServer treats them as the same path too.
	p = strings.TrimRight(p, "/")
	if p == "" {
		// "GET /" — that's the SPA index, not the API.
		return false
	}
	return strings.HasPrefix(p, "/api/")
}
