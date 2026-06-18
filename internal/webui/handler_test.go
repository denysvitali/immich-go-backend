package webui

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newDir builds a temporary directory containing the supplied files
// (key: relative path, value: contents) and returns the path. The dir is
// auto-cleaned via t.Cleanup.
func newDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		full := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	return dir
}

func TestHandler_EmptyDir_PassesThrough(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusTeapot)
	})

	h := Handler("", next)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if !called {
		t.Fatal("next handler was not invoked when WebUIDir is empty")
	}
	if rec.Code != http.StatusTeapot {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusTeapot)
	}
}

func TestHandler_MissingDir_PassesThrough(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	h := Handler("/definitely/does/not/exist", next)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if !called {
		t.Fatal("next handler was not invoked when WebUIDir is missing")
	}
}

func TestHandler_ServesStaticFile(t *testing.T) {
	dir := newDir(t, map[string]string{
		"index.html": "<html><body>hello</body></html>",
		"app.js":     "console.log('hi')",
	})
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("next should not be called for %s", r.URL.Path)
	})

	h := Handler(dir, next)

	req := httptest.NewRequest(http.MethodGet, "/app.js", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "console.log") {
		t.Fatalf("body = %q, want it to contain 'console.log'", rec.Body.String())
	}
}

func TestHandler_SPAFallbackToIndex(t *testing.T) {
	dir := newDir(t, map[string]string{
		"index.html": "<html>SPA</html>",
	})
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNotFound)
	})

	h := Handler(dir, next)

	// Deep link with no extension → SPA fallback serves index.html.
	req := httptest.NewRequest(http.MethodGet, "/photos/123", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if called {
		t.Fatal("next should not be called when index.html fallback applies")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "SPA") {
		t.Fatalf("body = %q, want it to contain 'SPA'", rec.Body.String())
	}
}

func TestHandler_AssetMissingFallsThrough(t *testing.T) {
	dir := newDir(t, map[string]string{
		"index.html": "<html>SPA</html>",
	})
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNotFound)
	})

	h := Handler(dir, next)

	// .css extension and missing file → next handler (real 404).
	req := httptest.NewRequest(http.MethodGet, "/assets/missing.css", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if !called {
		t.Fatal("next should be called for missing assets with extensions")
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestHandler_NonGETPassesThrough(t *testing.T) {
	dir := newDir(t, map[string]string{"index.html": "<html></html>"})
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusTeapot)
	})

	h := Handler(dir, next)

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if !called {
		t.Fatal("POST should pass through to next handler")
	}
	if rec.Code != http.StatusTeapot {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusTeapot)
	}
}

func TestHandler_PathTraversalBlocked(t *testing.T) {
	dir := newDir(t, map[string]string{"index.html": "<html></html>"})
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("next should not be called for traversal %s", r.URL.Path)
	})

	h := Handler(dir, next)

	req := httptest.NewRequest(http.MethodGet, "/../etc/passwd", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}
