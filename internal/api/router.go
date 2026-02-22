// Package api contains the Chi router and all HTTP handlers.
// Handlers never call ESI directly — they read only from the store package.
// The router serves the embedded frontend static files alongside the JSON API.
//
// Middleware stack: Logger, Recoverer, CORS (global), Content-Type: application/json (API routes only).
package api

import (
	"context"
	"io/fs"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/dpleshakov/auspex/internal/store"
)

// WorkerRefresher is the interface the api package uses to communicate with the sync worker.
type WorkerRefresher interface {
	ForceRefresh()
}

// AuthProvider is the interface the api package uses for EVE SSO OAuth2 operations.
type AuthProvider interface {
	GenerateAuthURL() (string, error)
	HandleCallback(ctx context.Context, code, state string) (int64, error)
}

// router holds shared dependencies for all HTTP handlers.
type router struct {
	q      store.Querier
	worker WorkerRefresher
	auth   AuthProvider
}

// NewRouter assembles and returns the application Chi router.
// staticFS must be rooted at the frontend dist directory ("index.html" at top level).
// In production, pass fs.Sub(staticFiles, "web/dist") from main.go.
func NewRouter(q store.Querier, worker WorkerRefresher, authProv AuthProvider, staticFS fs.FS) *chi.Mux {
	rt := &router{q: q, worker: worker, auth: authProv}

	mux := chi.NewRouter()
	mux.Use(middleware.Logger)
	mux.Use(middleware.Recoverer)
	mux.Use(corsMiddleware)

	mux.Route("/api", func(api chi.Router) {
		api.Use(jsonContentType)

		api.Get("/characters", rt.handleGetCharacters)
		api.Delete("/characters/{id}", rt.handleDeleteCharacter)

		api.Get("/corporations", rt.handleGetCorporations)
		api.Post("/corporations", rt.handleAddCorporation)
		api.Delete("/corporations/{id}", rt.handleDeleteCorporation)

		api.Get("/blueprints", rt.handleGetBlueprints)
		api.Get("/jobs/summary", rt.handleGetJobsSummary)

		api.Post("/sync", rt.handlePostSync)
		api.Get("/sync/status", rt.handleGetSyncStatus)
	})

	// EVE SSO OAuth2 routes — implemented in TASK-17.
	mux.Get("/auth/eve/login", rt.handleLogin)
	mux.Get("/auth/eve/callback", rt.handleCallback)

	// All remaining routes serve the embedded React SPA.
	mux.Handle("/*", newSPAHandler(staticFS))

	return mux
}

// jsonContentType sets Content-Type: application/json for all /api responses.
// Must be applied before the handler writes anything so handlers can safely
// call w.WriteHeader() without resetting the header.
func jsonContentType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

// corsMiddleware sets permissive CORS headers for the local dev environment.
// The app is a local desktop tool; cross-origin requests only occur during
// development when the Vite dev server proxies /api and /auth to the backend.
// Placed globally so OPTIONS preflight requests are handled before route matching.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
