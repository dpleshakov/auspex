package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dpleshakov/auspex/internal/store"
)

// --- GET /api/blueprints ---

func TestGetBlueprints_Empty(t *testing.T) {
	mux := NewRouter(&mockQuerier{}, nil, nil, testFS())

	req := httptest.NewRequest(http.MethodGet, "/api/blueprints", http.NoBody)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var got []any
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty array, got %d items", len(got))
	}
}

func TestGetBlueprints_NullJobWhenNoJob(t *testing.T) {
	mux := NewRouter(&mockQuerier{
		ListBlueprintsFn: func(_ context.Context, _ store.ListBlueprintsParams) ([]store.ListBlueprintsRow, error) {
			return []store.ListBlueprintsRow{
				{
					ID: 1, OwnerType: "character", OwnerID: 100,
					OwnerName: "Alpha", TypeID: 200, TypeName: "Blueprint Alpha",
					CategoryID: 5, CategoryName: "Ship", LocationID: 300,
					MeLevel: 10, TeLevel: 20,
					// JobID.Valid == false → no job
				},
			}, nil
		},
	}, nil, nil, testFS())

	req := httptest.NewRequest(http.MethodGet, "/api/blueprints", http.NoBody)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var got []map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 blueprint, got %d", len(got))
	}
	if got[0]["job"] != nil {
		t.Fatalf("expected null job, got %v", got[0]["job"])
	}
}

func TestGetBlueprints_NonNullJobWhenJobExists(t *testing.T) {
	startDate := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2026, 2, 2, 0, 0, 0, 0, time.UTC)

	mux := NewRouter(&mockQuerier{
		ListBlueprintsFn: func(_ context.Context, _ store.ListBlueprintsParams) ([]store.ListBlueprintsRow, error) {
			return []store.ListBlueprintsRow{
				{
					ID: 1, OwnerType: "character", OwnerID: 100,
					OwnerName: "Alpha", TypeID: 200, TypeName: "Blueprint Alpha",
					CategoryID: 5, CategoryName: "Ship", LocationID: 300,
					MeLevel: 10, TeLevel: 20,
					JobID:        sql.NullInt64{Int64: 42, Valid: true},
					JobActivity:  sql.NullString{String: "research_me", Valid: true},
					JobStatus:    sql.NullString{String: "active", Valid: true},
					JobStartDate: sql.NullTime{Time: startDate, Valid: true},
					JobEndDate:   sql.NullTime{Time: endDate, Valid: true},
				},
			}, nil
		},
	}, nil, nil, testFS())

	req := httptest.NewRequest(http.MethodGet, "/api/blueprints", http.NoBody)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var got []map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 blueprint, got %d", len(got))
	}
	jobField := got[0]["job"]
	if jobField == nil {
		t.Fatal("expected non-null job, got null")
	}
	job, ok := jobField.(map[string]any)
	if !ok {
		t.Fatalf("expected job object, got %T", jobField)
	}
	if job["activity"] != "research_me" {
		t.Errorf("job.activity = %v, want research_me", job["activity"])
	}
	if job["status"] != "active" {
		t.Errorf("job.status = %v, want active", job["status"])
	}
}

func TestGetBlueprints_FilterOwnerType(t *testing.T) {
	var captured store.ListBlueprintsParams
	mux := NewRouter(&mockQuerier{
		ListBlueprintsFn: func(_ context.Context, arg store.ListBlueprintsParams) ([]store.ListBlueprintsRow, error) {
			captured = arg
			return nil, nil
		},
	}, nil, nil, testFS())

	req := httptest.NewRequest(http.MethodGet, "/api/blueprints?owner_type=corporation", http.NoBody)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if captured.OwnerType != "corporation" {
		t.Errorf("OwnerType = %v, want corporation", captured.OwnerType)
	}
}

func TestGetBlueprints_FilterStatus(t *testing.T) {
	var captured store.ListBlueprintsParams
	mux := NewRouter(&mockQuerier{
		ListBlueprintsFn: func(_ context.Context, arg store.ListBlueprintsParams) ([]store.ListBlueprintsRow, error) {
			captured = arg
			return nil, nil
		},
	}, nil, nil, testFS())

	req := httptest.NewRequest(http.MethodGet, "/api/blueprints?status=idle", http.NoBody)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if captured.Status != "idle" {
		t.Errorf("Status = %v, want idle", captured.Status)
	}
}

func TestGetBlueprints_InvalidOwnerID(t *testing.T) {
	mux := NewRouter(&mockQuerier{}, nil, nil, testFS())

	req := httptest.NewRequest(http.MethodGet, "/api/blueprints?owner_id=notanumber", http.NoBody)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestGetBlueprints_InvalidCategoryID(t *testing.T) {
	mux := NewRouter(&mockQuerier{}, nil, nil, testFS())

	req := httptest.NewRequest(http.MethodGet, "/api/blueprints?category_id=xyz", http.NoBody)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestGetBlueprints_DBError(t *testing.T) {
	mux := NewRouter(&mockQuerier{
		ListBlueprintsFn: func(_ context.Context, _ store.ListBlueprintsParams) ([]store.ListBlueprintsRow, error) {
			return nil, errors.New("db error")
		},
	}, nil, nil, testFS())

	req := httptest.NewRequest(http.MethodGet, "/api/blueprints", http.NoBody)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

// --- GET /api/jobs/summary ---

func TestGetJobsSummary_Counts(t *testing.T) {
	mux := NewRouter(&mockQuerier{
		CountIdleBlueprintsFn:    func(_ context.Context) (int64, error) { return 5, nil },
		CountOverdueJobsFn:       func(_ context.Context) (int64, error) { return 3, nil },
		CountCompletingTodayFn:   func(_ context.Context) (int64, error) { return 1, nil },
		ListCharacterSlotUsageFn: func(_ context.Context) ([]store.ListCharacterSlotUsageRow, error) { return nil, nil },
	}, nil, nil, testFS())

	req := httptest.NewRequest(http.MethodGet, "/api/jobs/summary", http.NoBody)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var got summaryJSON
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.IdleBlueprints != 5 {
		t.Errorf("idle_blueprints = %d, want 5", got.IdleBlueprints)
	}
	if got.OverdueJobs != 3 {
		t.Errorf("overdue_jobs = %d, want 3", got.OverdueJobs)
	}
	if got.CompletingToday != 1 {
		t.Errorf("completing_today = %d, want 1", got.CompletingToday)
	}
}

func TestGetJobsSummary_IncludesCharacters(t *testing.T) {
	mux := NewRouter(&mockQuerier{
		ListCharacterSlotUsageFn: func(_ context.Context) ([]store.ListCharacterSlotUsageRow, error) {
			return []store.ListCharacterSlotUsageRow{
				{ID: 1, Name: "Alice", UsedSlots: 3},
				{ID: 2, Name: "Bob", UsedSlots: 0},
			}, nil
		},
	}, nil, nil, testFS())

	req := httptest.NewRequest(http.MethodGet, "/api/jobs/summary", http.NoBody)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var got summaryJSON
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Characters) != 2 {
		t.Fatalf("expected 2 characters, got %d", len(got.Characters))
	}
	if got.Characters[0].Name != "Alice" || got.Characters[0].UsedSlots != 3 {
		t.Errorf("characters[0] = %+v, want {Alice, 3}", got.Characters[0])
	}
	if got.Characters[1].Name != "Bob" || got.Characters[1].UsedSlots != 0 {
		t.Errorf("characters[1] = %+v, want {Bob, 0}", got.Characters[1])
	}
}

func TestGetJobsSummary_DBErrorOnIdle(t *testing.T) {
	mux := NewRouter(&mockQuerier{
		CountIdleBlueprintsFn: func(_ context.Context) (int64, error) {
			return 0, errors.New("db error")
		},
	}, nil, nil, testFS())

	req := httptest.NewRequest(http.MethodGet, "/api/jobs/summary", http.NoBody)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

func TestGetJobsSummary_DBErrorOnSlotUsage(t *testing.T) {
	mux := NewRouter(&mockQuerier{
		ListCharacterSlotUsageFn: func(_ context.Context) ([]store.ListCharacterSlotUsageRow, error) {
			return nil, errors.New("db error")
		},
	}, nil, nil, testFS())

	req := httptest.NewRequest(http.MethodGet, "/api/jobs/summary", http.NoBody)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}
