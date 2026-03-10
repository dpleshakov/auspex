package sync

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dpleshakov/auspex/internal/db"
	"github.com/dpleshakov/auspex/internal/esi"
	"github.com/dpleshakov/auspex/internal/store"
)

// newIntegrationDB opens an in-memory SQLite database, applies all migrations,
// and registers t.Cleanup(db.Close). Fails the test immediately on any error.
func newIntegrationDB(t *testing.T) *sql.DB {
	t.Helper()
	sqlDB, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("newIntegrationDB: open: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return sqlDB
}

// newIntegrationWorker constructs a Worker backed by a real store and a real
// ESI httpClient. All outgoing HTTP requests are redirected to the test server
// at esiServerURL via hostOverrideTransport, so no real network calls are made.
func newIntegrationWorker(t *testing.T, sqlDB *sql.DB, esiServerURL string) *Worker {
	t.Helper()
	q := store.New(sqlDB)
	transport := &hostOverrideTransport{target: esiServerURL}
	esiClient := esi.NewClient(&http.Client{Transport: transport})
	return New(q, esiClient, time.Minute)
}

// newESIServer starts a test HTTP server that serves fixture files from testdata/.
// routes maps URL path patterns (e.g. "/latest/characters/90000001/blueprints")
// to fixture file names relative to testdata/. Every response includes an
// Expires header 10 minutes in the future. The server is closed via t.Cleanup.
func newESIServer(t *testing.T, routes map[string]string) *httptest.Server {
	t.Helper()
	expires := time.Now().Add(10 * time.Minute).UTC().Format(http.TimeFormat)
	mux := http.NewServeMux()
	for pattern, fixture := range routes {
		fixturePath := filepath.Join("testdata", fixture)
		mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			data, err := os.ReadFile(fixturePath)
			if err != nil {
				http.Error(w, "fixture not found: "+fixturePath, http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Expires", expires)
			_, _ = w.Write(data)
		})
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// hostOverrideTransport rewrites the scheme and host of every outgoing request
// to the configured target, preserving the path, query, and all headers.
// This lets a real esi.Client be pointed at a local httptest.Server without
// changing the base URL baked into the client.
type hostOverrideTransport struct {
	target string // e.g. "http://127.0.0.1:54321"
}

func (h *hostOverrideTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	targetURL, err := url.Parse(h.target)
	if err != nil {
		return nil, fmt.Errorf("hostOverrideTransport: parsing target %q: %w", h.target, err)
	}
	cloned := req.Clone(req.Context())
	cloned.URL.Scheme = targetURL.Scheme
	cloned.URL.Host = targetURL.Host
	return http.DefaultTransport.RoundTrip(cloned)
}

// seedIntegrationCharacter inserts a minimal character row.
// corpID may be 0 (no corporation). Fails the test on any error.
func seedIntegrationCharacter(t *testing.T, sqlDB *sql.DB, id int64, corpID int64) {
	t.Helper()
	_, err := sqlDB.Exec(
		`INSERT INTO characters
		 (id, name, access_token, refresh_token, token_expiry, corporation_id, corporation_name)
		 VALUES (?, ?, 'tok', 'rtok', ?, ?, '')`,
		id, fmt.Sprintf("Character%d", id), time.Now().Add(time.Hour), corpID,
	)
	if err != nil {
		t.Fatalf("seedIntegrationCharacter %d: %v", id, err)
	}
}

// seedIntegrationCorporation inserts a minimal corporation row.
// delegateID must reference an existing character row. Fails the test on any error.
func seedIntegrationCorporation(t *testing.T, sqlDB *sql.DB, id int64, delegateID int64) {
	t.Helper()
	_, err := sqlDB.Exec(
		`INSERT INTO corporations (id, name, delegate_id) VALUES (?, ?, ?)`,
		id, fmt.Sprintf("Corporation%d", id), delegateID,
	)
	if err != nil {
		t.Fatalf("seedIntegrationCorporation %d: %v", id, err)
	}
}

// TestIntegrationHarness_Smoke verifies that all harness helpers construct and
// clean up without error. It is not a sync test — it only validates the infrastructure.
func TestIntegrationHarness_Smoke(t *testing.T) {
	sqlDB := newIntegrationDB(t)
	seedIntegrationCharacter(t, sqlDB, 90000001, 0)
	seedIntegrationCorporation(t, sqlDB, 99000001, 90000001)
	srv := newESIServer(t, map[string]string{})
	_ = newIntegrationWorker(t, sqlDB, srv.URL)
}
