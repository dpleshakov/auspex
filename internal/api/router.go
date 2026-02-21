// Package api contains the Chi router and all HTTP handlers.
// Handlers never call ESI directly — they read only from the store package.
// The router serves the embedded frontend static files alongside the JSON API.
//
// Middleware stack: Logger, Recoverer, CORS, Content-Type: application/json (API routes only).
package api

// router.go: assembles the Chi router, registers middleware, mounts all handlers.
