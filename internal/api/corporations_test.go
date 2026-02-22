package api

import (
	"bytes"
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

func TestGetCorporations_ReturnsJSON(t *testing.T) {
	createdAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	mock := &mockQuerier{
		ListCorporationsFn: func(ctx context.Context) ([]store.ListCorporationsRow, error) {
			return []store.ListCorporationsRow{
				{ID: 100, Name: "Goonswarm", DelegateID: 1, DelegateName: "Alpha", CreatedAt: createdAt},
			}, nil
		},
	}
	mux := NewRouter(mock, nil, nil, testFS())

	req := httptest.NewRequest(http.MethodGet, "/api/corporations", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var got []map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 corporation, got %d", len(got))
	}
	if got[0]["name"] != "Goonswarm" {
		t.Errorf("expected name Goonswarm, got %v", got[0]["name"])
	}
	if got[0]["delegate_name"] != "Alpha" {
		t.Errorf("expected delegate_name Alpha, got %v", got[0]["delegate_name"])
	}
}

func TestGetCorporations_DBError(t *testing.T) {
	mock := &mockQuerier{
		ListCorporationsFn: func(ctx context.Context) ([]store.ListCorporationsRow, error) {
			return nil, errors.New("db error")
		},
	}
	mux := NewRouter(mock, nil, nil, testFS())

	req := httptest.NewRequest(http.MethodGet, "/api/corporations", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

func TestAddCorporation_OK(t *testing.T) {
	var inserted store.InsertCorporationParams
	mock := &mockQuerier{
		InsertCorporationFn: func(ctx context.Context, arg store.InsertCorporationParams) error {
			inserted = arg
			return nil
		},
	}
	mux := NewRouter(mock, nil, nil, testFS())

	body := `{"id":100,"name":"Goonswarm","delegate_id":1}`
	req := httptest.NewRequest(http.MethodPost, "/api/corporations", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	if inserted.ID != 100 || inserted.Name != "Goonswarm" || inserted.DelegateID != 1 {
		t.Errorf("inserted params = %+v, want {100 Goonswarm 1}", inserted)
	}
}

func TestAddCorporation_InvalidDelegate(t *testing.T) {
	mock := &mockQuerier{
		GetCharacterFn: func(ctx context.Context, id int64) (store.Character, error) {
			return store.Character{}, sql.ErrNoRows
		},
	}
	mux := NewRouter(mock, nil, nil, testFS())

	body := `{"id":100,"name":"Goonswarm","delegate_id":999}`
	req := httptest.NewRequest(http.MethodPost, "/api/corporations", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestAddCorporation_MissingFields(t *testing.T) {
	mux := NewRouter(&mockQuerier{}, nil, nil, testFS())

	cases := []struct {
		name string
		body string
	}{
		{"missing id", `{"name":"Corp","delegate_id":1}`},
		{"missing name", `{"id":100,"delegate_id":1}`},
		{"missing delegate_id", `{"id":100,"name":"Corp"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/corporations", bytes.NewBufferString(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, req)
			if rr.Code != http.StatusBadRequest {
				t.Errorf("%s: expected 400, got %d", tc.name, rr.Code)
			}
		})
	}
}

func TestAddCorporation_InvalidBody(t *testing.T) {
	mux := NewRouter(&mockQuerier{}, nil, nil, testFS())

	req := httptest.NewRequest(http.MethodPost, "/api/corporations", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestDeleteCorporation_OK(t *testing.T) {
	var calls []string
	mock := &mockQuerier{
		DeleteBlueprintsByOwnerFn: func(ctx context.Context, arg store.DeleteBlueprintsByOwnerParams) error {
			calls = append(calls, "blueprints:"+arg.OwnerType)
			return nil
		},
		DeleteJobsByOwnerFn: func(ctx context.Context, arg store.DeleteJobsByOwnerParams) error {
			calls = append(calls, "jobs:"+arg.OwnerType)
			return nil
		},
		DeleteSyncStateByOwnerFn: func(ctx context.Context, arg store.DeleteSyncStateByOwnerParams) error {
			calls = append(calls, "sync_state:"+arg.OwnerType)
			return nil
		},
		DeleteCorporationFn: func(ctx context.Context, id int64) error {
			calls = append(calls, "corporation")
			return nil
		},
	}
	mux := NewRouter(mock, nil, nil, testFS())

	req := httptest.NewRequest(http.MethodDelete, "/api/corporations/100", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}
	want := []string{"blueprints:corporation", "jobs:corporation", "sync_state:corporation", "corporation"}
	if len(calls) != len(want) {
		t.Fatalf("cascade calls = %v, want %v", calls, want)
	}
	for i, c := range calls {
		if c != want[i] {
			t.Errorf("cascade call[%d] = %q, want %q", i, c, want[i])
		}
	}
}

func TestDeleteCorporation_InvalidID(t *testing.T) {
	mux := NewRouter(&mockQuerier{}, nil, nil, testFS())

	req := httptest.NewRequest(http.MethodDelete, "/api/corporations/notanumber", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}
