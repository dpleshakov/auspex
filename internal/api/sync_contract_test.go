package api

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func TestContract_GetJobsSummary_EmptyDB(t *testing.T) {
	sqlDB := newContractDB(t)
	srv := newContractServer(t, sqlDB)

	resp, err := http.Get(srv.URL + "/api/jobs/summary")
	if err != nil {
		t.Fatalf("GET /api/jobs/summary: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	assertField[float64](t, body, "idle_blueprints")
	assertField[float64](t, body, "ready_jobs")
	assertField[float64](t, body, "free_research_slots")

	// characters must be present and be an array (may be empty).
	charsRaw, ok := body["characters"]
	if !ok {
		t.Fatalf("key \"characters\" missing from response")
	}
	if _, ok := charsRaw.([]any); !ok {
		t.Errorf("\"characters\": want array, got %T", charsRaw)
	}
}

func TestContract_GetJobsSummary_WithCharacter(t *testing.T) {
	sqlDB := newContractDB(t)
	seedCharacter(t, sqlDB, 4001, "Worker", 0)
	seedBlueprint(t, sqlDB, BlueprintSeed{ID: 9010, OwnerType: "character", OwnerID: 4001})
	seedJob(t, sqlDB, JobSeed{
		ID:          8001,
		BlueprintID: 9010,
		OwnerType:   "character",
		OwnerID:     4001,
		InstallerID: 4001,
		Activity:    "me_research",
		Status:      "active",
		EndDate:     time.Now().Add(time.Hour),
	})
	srv := newContractServer(t, sqlDB)

	resp, err := http.Get(srv.URL + "/api/jobs/summary")
	if err != nil {
		t.Fatalf("GET /api/jobs/summary: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	charsRaw, ok := body["characters"]
	if !ok {
		t.Fatalf("key \"characters\" missing")
	}
	chars, ok := charsRaw.([]any)
	if !ok || len(chars) == 0 {
		t.Fatalf("expected non-empty characters array, got %T len=%d", charsRaw, len(chars))
	}
	item, ok := chars[0].(map[string]any)
	if !ok {
		t.Fatalf("characters[0]: want object, got %T", chars[0])
	}
	assertField[float64](t, item, "id")
	assertField[string](t, item, "name")
	assertField[float64](t, item, "used_slots")
}

func TestContract_GetSyncStatus_EmptyDB(t *testing.T) {
	sqlDB := newContractDB(t)
	srv := newContractServer(t, sqlDB)

	resp, err := http.Get(srv.URL + "/api/sync/status")
	if err != nil {
		t.Fatalf("GET /api/sync/status: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var body []any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(body) != 0 {
		t.Errorf("expected empty array, got %d items", len(body))
	}
}

func TestContract_GetSyncStatus_WithData(t *testing.T) {
	sqlDB := newContractDB(t)
	seedCharacter(t, sqlDB, 4002, "SyncOwner", 0)
	seedSyncState(t, sqlDB, "character", 4002, "blueprints", time.Now().Add(time.Hour), "")
	srv := newContractServer(t, sqlDB)

	resp, err := http.Get(srv.URL + "/api/sync/status")
	if err != nil {
		t.Fatalf("GET /api/sync/status: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var items []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(items) == 0 {
		t.Fatalf("expected at least 1 sync status item, got 0")
	}
	item := items[0]

	assertField[string](t, item, "owner_type")
	assertField[float64](t, item, "owner_id")
	assertField[string](t, item, "owner_name")
	assertField[string](t, item, "endpoint")
	assertField[string](t, item, "last_sync")
	assertField[string](t, item, "cache_until")
}

func TestContract_PostSync_Returns202(t *testing.T) {
	sqlDB := newContractDB(t)
	srv := newContractServer(t, sqlDB)

	resp, err := http.Post(srv.URL+"/api/sync", "application/json", nil)
	if err != nil {
		t.Fatalf("POST /api/sync: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}
}
