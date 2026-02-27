package db

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Open opens a SQLite database at path, runs any pending migrations, and
// returns the connection. Enables WAL journal mode and foreign key enforcement.
//
// MaxOpenConns is set to 1 because PRAGMA foreign_keys is a per-connection
// setting in SQLite: with a pool of connections, new connections would not
// have FK enforcement. A single connection also avoids concurrent-write
// contention, which SQLite does not support well.
func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	var setupErr error
	defer func() {
		if setupErr != nil {
			_ = db.Close() //nolint:errcheck // cleanup after setup failure
		}
	}()

	// Limit to a single connection so PRAGMAs set below apply to every query.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if setupErr := db.Ping(); setupErr != nil {
		return nil, fmt.Errorf("pinging database: %w", setupErr)
	}

	if _, setupErr := db.Exec("PRAGMA journal_mode=WAL"); setupErr != nil {
		return nil, fmt.Errorf("setting WAL mode: %w", setupErr)
	}

	if _, setupErr := db.Exec("PRAGMA foreign_keys=ON"); setupErr != nil {
		return nil, fmt.Errorf("enabling foreign keys: %w", setupErr)
	}

	if setupErr := runMigrations(db); setupErr != nil {
		return nil, fmt.Errorf("running migrations: %w", setupErr)
	}

	return db, nil
}

func runMigrations(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			filename   TEXT PRIMARY KEY,
			applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("creating schema_migrations table: %w", err)
	}

	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("reading migrations directory: %w", err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()

		var count int
		if err := db.QueryRow(
			"SELECT COUNT(*) FROM schema_migrations WHERE filename = ?", filename,
		).Scan(&count); err != nil {
			return fmt.Errorf("checking migration %s: %w", filename, err)
		}
		if count > 0 {
			continue
		}

		content, err := migrationsFS.ReadFile("migrations/" + filename)
		if err != nil {
			return fmt.Errorf("reading migration %s: %w", filename, err)
		}

		if err := applyMigration(db, filename, string(content)); err != nil {
			return err
		}
	}

	return nil
}

// applyMigration executes a single migration inside a transaction so that a
// partially-applied migration cannot leave the schema in an inconsistent state.
func applyMigration(db *sql.DB, filename, content string) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction for migration %s: %w", filename, err)
	}
	defer tx.Rollback() //nolint:errcheck // no way to do anything

	if _, err := tx.Exec(content); err != nil {
		return fmt.Errorf("executing migration %s: %w", filename, err)
	}

	if _, err := tx.Exec(
		"INSERT INTO schema_migrations (filename) VALUES (?)", filename,
	); err != nil {
		return fmt.Errorf("recording migration %s: %w", filename, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing migration %s: %w", filename, err)
	}

	return nil
}
