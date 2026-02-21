// Package db initializes the SQLite connection and runs schema migrations on startup.
// Provides *sql.DB to other packages. Uses modernc.org/sqlite (pure Go, no CGO).
// Migrations are up-only (no rollback) for MVP.
package db
