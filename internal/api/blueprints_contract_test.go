package api

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func TestContract_GetBlueprints_EmptyDB(t *testing.T) {
	sqlDB := newContractDB(t)
	srv := newContractServer(t, sqlDB)

	resp, err := http.Get(srv.URL + "/api/blueprints")
	if err != nil {
		t.Fatalf("GET /api/blueprints: %v", err)
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

func TestContract_GetBlueprints_NoJob(t *testing.T) {
	sqlDB := newContractDB(t)
	seedCharacter(t, sqlDB, 3001, "Manufacturer", 0)
	seedBlueprint(t, sqlDB, BlueprintSeed{ID: 9001, OwnerType: "character", OwnerID: 3001})
	srv := newContractServer(t, sqlDB)

	resp, err := http.Get(srv.URL + "/api/blueprints")
	if err != nil {
		t.Fatalf("GET /api/blueprints: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var items []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 blueprint, got %d", len(items))
	}
	bp := items[0]

	assertField[float64](t, bp, "id")
	assertField[string](t, bp, "owner_type")
	assertField[float64](t, bp, "owner_id")
	assertField[string](t, bp, "owner_name")
	assertField[float64](t, bp, "type_id")
	assertField[string](t, bp, "type_name")
	assertField[float64](t, bp, "category_id")
	assertField[string](t, bp, "category_name")
	assertField[float64](t, bp, "location_id")
	assertField[float64](t, bp, "me_level")
	assertField[float64](t, bp, "te_level")
	// No active job → job field is null.
	assertNull(t, bp, "job")
}

func TestContract_GetBlueprints_WithJob(t *testing.T) {
	sqlDB := newContractDB(t)
	seedCharacter(t, sqlDB, 3002, "Builder", 0)
	seedBlueprint(t, sqlDB, BlueprintSeed{ID: 9002, OwnerType: "character", OwnerID: 3002})
	seedJob(t, sqlDB, JobSeed{
		ID:          7001,
		BlueprintID: 9002,
		OwnerType:   "character",
		OwnerID:     3002,
		InstallerID: 3002,
		Activity:    "me_research",
		Status:      "active",
		EndDate:     time.Now().Add(time.Hour),
	})
	srv := newContractServer(t, sqlDB)

	resp, err := http.Get(srv.URL + "/api/blueprints")
	if err != nil {
		t.Fatalf("GET /api/blueprints: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var items []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 blueprint, got %d", len(items))
	}
	bp := items[0]

	// job field must be a non-null object with correct field types.
	jobRaw, ok := bp["job"]
	if !ok {
		t.Fatalf("key \"job\" missing from response")
	}
	job, ok := jobRaw.(map[string]any)
	if !ok {
		t.Fatalf("\"job\": want object, got %T", jobRaw)
	}
	assertField[float64](t, job, "id")
	assertField[string](t, job, "activity")
	assertField[string](t, job, "status")
	assertField[string](t, job, "start_date")
	assertField[string](t, job, "end_date")
}

func TestContract_GetBlueprints_FilterByOwner(t *testing.T) {
	sqlDB := newContractDB(t)
	seedCharacter(t, sqlDB, 3003, "CharA", 0)
	seedCharacter(t, sqlDB, 3004, "CharB", 0)
	seedBlueprint(t, sqlDB, BlueprintSeed{ID: 9003, OwnerType: "character", OwnerID: 3003})
	seedBlueprint(t, sqlDB, BlueprintSeed{ID: 9004, OwnerType: "character", OwnerID: 3004})
	srv := newContractServer(t, sqlDB)

	resp, err := http.Get(srv.URL + "/api/blueprints?owner_type=character&owner_id=3003")
	if err != nil {
		t.Fatalf("GET /api/blueprints with filter: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var items []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("expected 1 blueprint for owner 3003, got %d", len(items))
	}
}
