package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// testFS builds a minimal in-memory FS that mimics the embedded frontend dist.
func testFS() fstest.MapFS {
	return fstest.MapFS{
		"index.html":    {Data: []byte("<html>app</html>")},
		"assets/app.js": {Data: []byte("// js")},
	}
}

// --- jsonContentType middleware ---

func TestJSONContentType(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := jsonContentType(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want %q", got, "application/json")
	}
}

// --- CORS middleware ---

func TestCORSMiddleware_SetsHeaders(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := corsMiddleware(next)

	req := httptest.NewRequest(http.MethodGet, "/api/blueprints", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("Access-Control-Allow-Origin = %q, want *", got)
	}
}

func TestCORSMiddleware_Options(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Should not be called for OPTIONS.
		w.WriteHeader(http.StatusOK)
	})
	handler := corsMiddleware(next)

	req := httptest.NewRequest(http.MethodOptions, "/api/blueprints", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("OPTIONS: expected 204, got %d", rr.Code)
	}
}

// --- Panic recovery ---

func TestPanicRecovery(t *testing.T) {
	// Verify that middleware.Recoverer (used in NewRouter) turns panics into 500s.
	mux := chi.NewRouter()
	mux.Use(middleware.Recoverer)
	mux.Get("/boom", func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("panic: expected 500, got %d", rr.Code)
	}
}

// --- Assembled router: Content-Type on /api routes ---

func TestRouter_APIContentType(t *testing.T) {
	mux := NewRouter(nil, nil, nil, testFS())

	req := httptest.NewRequest(http.MethodGet, "/api/characters", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("/api/characters Content-Type = %q, want application/json", got)
	}
}

// --- Assembled router: static file serving ---

func TestRouter_StaticKnownFile(t *testing.T) {
	mux := NewRouter(nil, nil, nil, testFS())

	req := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("GET /assets/app.js: expected 200, got %d", rr.Code)
	}
}

func TestRouter_SPAFallback(t *testing.T) {
	mux := NewRouter(nil, nil, nil, testFS())

	// /some/spa/route does not exist as a file → must return index.html.
	req := httptest.NewRequest(http.MethodGet, "/some/spa/route", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("SPA fallback: expected 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "<html>app</html>") {
		t.Errorf("SPA fallback: expected index.html content, got %q", rr.Body.String())
	}
}

func TestRouter_IndexHTML(t *testing.T) {
	mux := NewRouter(nil, nil, nil, testFS())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("GET /: expected 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "<html>app</html>") {
		t.Errorf("GET /: expected index.html content, got %q", rr.Body.String())
	}
}
