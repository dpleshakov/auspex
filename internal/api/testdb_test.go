package api

import (
	"context"
	"database/sql"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dpleshakov/auspex/internal/db"
	"github.com/dpleshakov/auspex/internal/store"
)

// --- Stubs ---

type noopWorker struct{}

func (noopWorker) ForceRefresh() {}

type noopAuth struct{}

func (noopAuth) GenerateAuthURL() (string, error) { return "", nil }
func (noopAuth) HandleCallback(_ context.Context, _, _ string) (int64, error) {
	return 0, nil
}

// --- Test infrastructure ---

// newContractDB opens an in-memory SQLite database and applies all migrations.
// Fails the test immediately on any error.
func newContractDB(t *testing.T) *sql.DB {
	t.Helper()
	sqlDB, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("newContractDB: open: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return sqlDB
}

// newContractServer wires the real store, a no-op WorkerRefresher, and a no-op
// AuthProvider into the Chi router and starts a test HTTP server.
// The server is closed automatically via t.Cleanup.
func newContractServer(t *testing.T, sqlDB *sql.DB) *httptest.Server {
	t.Helper()
	q := store.New(sqlDB)
	mux := NewRouter(q, noopWorker{}, noopAuth{}, testFS())
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// --- Seed helpers ---

// seedCharacter inserts a character row. corpID may be 0 (no corporation).
func seedCharacter(t *testing.T, sqlDB *sql.DB, id int64, name string, corpID int64) {
	t.Helper()
	_, err := sqlDB.Exec(
		`INSERT INTO characters
		 (id, name, access_token, refresh_token, token_expiry, corporation_id, corporation_name)
		 VALUES (?, ?, 'tok', 'rtok', ?, ?, '')`,
		id, name, time.Now().Add(time.Hour), corpID,
	)
	if err != nil {
		t.Fatalf("seedCharacter %d (%s): %v", id, name, err)
	}
}

// seedCorporation inserts a corporation row.
// delegateID must reference an existing character row.
func seedCorporation(t *testing.T, sqlDB *sql.DB, id int64, name string, delegateID int64) {
	t.Helper()
	_, err := sqlDB.Exec(
		`INSERT INTO corporations (id, name, delegate_id) VALUES (?, ?, ?)`,
		id, name, delegateID,
	)
	if err != nil {
		t.Fatalf("seedCorporation %d (%s): %v", id, name, err)
	}
}

// seedSyncState inserts a sync_state row.
// Pass lastError="" to store NULL; any non-empty string is stored as the error value.
func seedSyncState(t *testing.T, sqlDB *sql.DB, ownerType string, ownerID int64, endpoint string, cacheUntil time.Time, lastError string) {
	t.Helper()
	var le any
	if lastError != "" {
		le = lastError
	}
	_, err := sqlDB.Exec(
		`INSERT INTO sync_state (owner_type, owner_id, endpoint, last_sync, cache_until, last_error)
		 VALUES (?, ?, ?, CURRENT_TIMESTAMP, ?, ?)`,
		ownerType, ownerID, endpoint, cacheUntil, le,
	)
	if err != nil {
		t.Fatalf("seedSyncState %s/%d/%s: %v", ownerType, ownerID, endpoint, err)
	}
}

// BlueprintSeed holds the fields needed to insert a blueprint row.
// seedBlueprint also ensures the required EVE type hierarchy rows exist
// (INSERT OR IGNORE), so tests only need to specify the IDs they care about.
type BlueprintSeed struct {
	ID         int64
	OwnerType  string // "character" | "corporation"; defaults to "character"
	OwnerID    int64
	TypeID     int64 // defaults to 1
	CategoryID int64 // defaults to 1
	GroupID    int64 // defaults to 1
	LocationID int64
	MeLevel    int64
	TeLevel    int64
}

// seedBlueprint inserts a blueprint and the minimum EVE universe rows it depends on.
func seedBlueprint(t *testing.T, sqlDB *sql.DB, b BlueprintSeed) {
	t.Helper()
	if b.OwnerType == "" {
		b.OwnerType = "character"
	}
	if b.TypeID == 0 {
		b.TypeID = 1
	}
	if b.CategoryID == 0 {
		b.CategoryID = 1
	}
	if b.GroupID == 0 {
		b.GroupID = 1
	}

	if _, err := sqlDB.Exec(
		`INSERT OR IGNORE INTO eve_categories (id, name) VALUES (?, 'Category')`,
		b.CategoryID,
	); err != nil {
		t.Fatalf("seedBlueprint ensure category %d: %v", b.CategoryID, err)
	}
	if _, err := sqlDB.Exec(
		`INSERT OR IGNORE INTO eve_groups (id, category_id, name) VALUES (?, ?, 'Group')`,
		b.GroupID, b.CategoryID,
	); err != nil {
		t.Fatalf("seedBlueprint ensure group %d: %v", b.GroupID, err)
	}
	if _, err := sqlDB.Exec(
		`INSERT OR IGNORE INTO eve_types (id, group_id, name) VALUES (?, ?, 'Type')`,
		b.TypeID, b.GroupID,
	); err != nil {
		t.Fatalf("seedBlueprint ensure type %d: %v", b.TypeID, err)
	}

	if _, err := sqlDB.Exec(
		`INSERT INTO blueprints
		 (id, owner_type, owner_id, type_id, location_id, me_level, te_level, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		b.ID, b.OwnerType, b.OwnerID, b.TypeID, b.LocationID, b.MeLevel, b.TeLevel,
	); err != nil {
		t.Fatalf("seedBlueprint %d: %v", b.ID, err)
	}
}

// JobSeed holds the fields needed to insert a job row.
type JobSeed struct {
	ID          int64
	BlueprintID int64
	OwnerType   string // "character" | "corporation"; defaults to "character"
	OwnerID     int64
	InstallerID int64
	Activity    string // defaults to "me_research"
	Status      string // defaults to "active"
	StartDate   time.Time
	EndDate     time.Time
}

// seedJob inserts a job row. InstallerID must reference an existing character.
func seedJob(t *testing.T, sqlDB *sql.DB, j JobSeed) {
	t.Helper()
	if j.OwnerType == "" {
		j.OwnerType = "character"
	}
	if j.Activity == "" {
		j.Activity = "me_research"
	}
	if j.Status == "" {
		j.Status = "active"
	}
	if j.EndDate.IsZero() {
		j.EndDate = time.Now().Add(time.Hour)
	}
	_, err := sqlDB.Exec(
		`INSERT INTO jobs
		 (id, blueprint_id, owner_type, owner_id, installer_id, activity, status, start_date, end_date, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		j.ID, j.BlueprintID, j.OwnerType, j.OwnerID, j.InstallerID,
		j.Activity, j.Status, j.StartDate, j.EndDate,
	)
	if err != nil {
		t.Fatalf("seedJob %d: %v", j.ID, err)
	}
}

// --- Assertion helpers ---

// assertField verifies key exists in m and its value has type T.
// In JSON decoded to map[string]any, numbers are float64, booleans are bool, strings are string.
func assertField[T any](t *testing.T, m map[string]any, key string) {
	t.Helper()
	v, ok := m[key]
	if !ok {
		t.Errorf("key %q missing from response", key)
		return
	}
	if _, ok := v.(T); !ok {
		var zero T
		t.Errorf("key %q: want %T, got %T (%v)", key, zero, v, v)
	}
}

// assertNull verifies key exists in m and its value is JSON null (nil in Go).
func assertNull(t *testing.T, m map[string]any, key string) {
	t.Helper()
	v, ok := m[key]
	if !ok {
		t.Errorf("key %q missing from response", key)
		return
	}
	if v != nil {
		t.Errorf("key %q: want null, got %v (%T)", key, v, v)
	}
}

// --- Smoke test ---

func TestContractDB_MigrationsApply(t *testing.T) {
	sqlDB := newContractDB(t)
	var name string
	if err := sqlDB.QueryRow(
		`SELECT name FROM sqlite_master WHERE type='table' AND name='blueprints'`,
	).Scan(&name); err != nil {
		t.Fatalf("blueprints table not found after migrations: %v", err)
	}
}
