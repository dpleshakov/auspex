package db

import (
	"path/filepath"
	"testing"
)

var expectedTables = []string{
	"eve_categories", "eve_groups", "eve_types",
	"characters", "corporations",
	"blueprints", "jobs", "sync_state",
}

func TestOpen_TablesCreated(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() {
		if err = db.Close(); err != nil {
			t.Fatalf("close: %v", err)
		}
	}()

	for _, table := range expectedTables {
		var name string
		err := db.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?", table,
		).Scan(&name)
		if err != nil {
			t.Errorf("table %q not found after migration: %v", table, err)
		}
	}
}

func TestOpen_Idempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	for i := range 3 {
		db, err := Open(path)
		if err != nil {
			t.Fatalf("Open call %d: %v", i+1, err)
		}
		if err = db.Close(); err != nil {
			t.Fatalf("close: %v", err)
		}
	}
}

func TestOpen_MigrationRecorded(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() {
		if err = db.Close(); err != nil {
			t.Fatalf("close: %v", err)
		}
	}()

	var count int
	if err := db.QueryRow(
		"SELECT COUNT(*) FROM schema_migrations WHERE filename = '001_initial.sql'",
	).Scan(&count); err != nil {
		t.Fatalf("querying schema_migrations: %v", err)
	}
	if count != 1 {
		t.Errorf("migration recorded %d times, want 1", count)
	}
}
