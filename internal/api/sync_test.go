package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dpleshakov/auspex/internal/store"
)

// mockWorker implements WorkerRefresher for tests.
type mockWorker struct {
	forceRefreshCalled bool
}

func (m *mockWorker) ForceRefresh() {
	m.forceRefreshCalled = true
}

// --- POST /api/sync ---

func TestPostSync_Returns202(t *testing.T) {
	worker := &mockWorker{}
	mux := NewRouter(&mockQuerier{}, worker, nil, testFS())

	req := httptest.NewRequest(http.MethodPost, "/api/sync", http.NoBody)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rr.Code)
	}
}

func TestPostSync_CallsForceRefresh(t *testing.T) {
	worker := &mockWorker{}
	mux := NewRouter(&mockQuerier{}, worker, nil, testFS())

	req := httptest.NewRequest(http.MethodPost, "/api/sync", http.NoBody)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if !worker.forceRefreshCalled {
		t.Fatal("expected ForceRefresh to be called")
	}
}

// --- GET /api/sync/status ---

func TestGetSyncStatus_Empty(t *testing.T) {
	mux := NewRouter(&mockQuerier{}, nil, nil, testFS())

	req := httptest.NewRequest(http.MethodGet, "/api/sync/status", http.NoBody)
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

func TestGetSyncStatus_IncludesOwnerNames(t *testing.T) {
	lastSync := time.Date(2026, 2, 20, 10, 0, 0, 0, time.UTC)
	cacheUntil := time.Date(2026, 2, 20, 11, 0, 0, 0, time.UTC)

	mux := NewRouter(&mockQuerier{
		ListSyncStatusFn: func(_ context.Context) ([]store.ListSyncStatusRow, error) {
			return []store.ListSyncStatusRow{
				{
					OwnerType:  "character",
					OwnerID:    101,
					OwnerName:  "Alice",
					Endpoint:   "blueprints",
					LastSync:   lastSync,
					CacheUntil: cacheUntil,
				},
				{
					OwnerType:  "corporation",
					OwnerID:    200,
					OwnerName:  "AlphaCorp",
					Endpoint:   "jobs",
					LastSync:   lastSync,
					CacheUntil: cacheUntil,
				},
			}, nil
		},
	}, nil, nil, testFS())

	req := httptest.NewRequest(http.MethodGet, "/api/sync/status", http.NoBody)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var got []syncStatusItemJSON
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 items, got %d", len(got))
	}

	if got[0].OwnerType != "character" || got[0].OwnerID != 101 || got[0].OwnerName != "Alice" || got[0].Endpoint != "blueprints" {
		t.Errorf("item[0] = %+v, want character/101/Alice/blueprints", got[0])
	}
	if got[1].OwnerType != "corporation" || got[1].OwnerID != 200 || got[1].OwnerName != "AlphaCorp" || got[1].Endpoint != "jobs" {
		t.Errorf("item[1] = %+v, want corporation/200/AlphaCorp/jobs", got[1])
	}
}

func TestGetSyncStatus_DBError(t *testing.T) {
	mux := NewRouter(&mockQuerier{
		ListSyncStatusFn: func(_ context.Context) ([]store.ListSyncStatusRow, error) {
			return nil, errors.New("db error")
		},
	}, nil, nil, testFS())

	req := httptest.NewRequest(http.MethodGet, "/api/sync/status", http.NoBody)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}
