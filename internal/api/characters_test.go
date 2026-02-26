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

func TestGetCharacters_ReturnsJSON(t *testing.T) {
	createdAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	mock := &mockQuerier{
		ListCharactersFn: func(_ context.Context) ([]store.Character, error) {
			return []store.Character{
				{ID: 1, Name: "Alpha", CreatedAt: createdAt},
				{ID: 2, Name: "Beta", CreatedAt: createdAt},
			}, nil
		},
	}
	mux := NewRouter(mock, nil, nil, testFS())

	req := httptest.NewRequest(http.MethodGet, "/api/characters", http.NoBody)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var got []map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 characters, got %d", len(got))
	}
	if got[0]["name"] != "Alpha" {
		t.Errorf("expected name Alpha, got %v", got[0]["name"])
	}
	// Tokens must never appear in the response.
	for _, field := range []string{"access_token", "refresh_token", "token_expiry"} {
		if _, ok := got[0][field]; ok {
			t.Errorf("field %q must not be exposed in API response", field)
		}
	}
}

func TestGetCharacters_EmptyList(t *testing.T) {
	mux := NewRouter(&mockQuerier{}, nil, nil, testFS())

	req := httptest.NewRequest(http.MethodGet, "/api/characters", http.NoBody)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	// Body must be a JSON array (empty list, not null).
	var got []any
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty array, got %v", got)
	}
}

func TestGetCharacters_DBError(t *testing.T) {
	mock := &mockQuerier{
		ListCharactersFn: func(_ context.Context) ([]store.Character, error) {
			return nil, errors.New("db error")
		},
	}
	mux := NewRouter(mock, nil, nil, testFS())

	req := httptest.NewRequest(http.MethodGet, "/api/characters", http.NoBody)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

func TestDeleteCharacter_OK(t *testing.T) {
	var gotDeletedID int64
	mock := &mockQuerier{
		DeleteCharacterFn: func(_ context.Context, id int64) error {
			gotDeletedID = id
			return nil
		},
	}
	mux := NewRouter(mock, nil, nil, testFS())

	req := httptest.NewRequest(http.MethodDelete, "/api/characters/42", http.NoBody)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", rr.Code)
	}
	if gotDeletedID != 42 {
		t.Errorf("expected deleted id=42, got %d", gotDeletedID)
	}
}

func TestDeleteCharacter_CascadeOrder(t *testing.T) {
	var calls []string
	mock := &mockQuerier{
		DeleteBlueprintsByOwnerFn: func(_ context.Context, arg store.DeleteBlueprintsByOwnerParams) error {
			calls = append(calls, "blueprints:"+arg.OwnerType)
			return nil
		},
		DeleteJobsByOwnerFn: func(_ context.Context, arg store.DeleteJobsByOwnerParams) error {
			calls = append(calls, "jobs:"+arg.OwnerType)
			return nil
		},
		DeleteSyncStateByOwnerFn: func(_ context.Context, arg store.DeleteSyncStateByOwnerParams) error {
			calls = append(calls, "sync_state:"+arg.OwnerType)
			return nil
		},
		DeleteCharacterFn: func(_ context.Context, _ int64) error {
			calls = append(calls, "character")
			return nil
		},
	}
	mux := NewRouter(mock, nil, nil, testFS())

	req := httptest.NewRequest(http.MethodDelete, "/api/characters/7", http.NoBody)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}
	want := []string{"blueprints:character", "jobs:character", "sync_state:character", "character"}
	if len(calls) != len(want) {
		t.Fatalf("cascade calls = %v, want %v", calls, want)
	}
	for i, c := range calls {
		if c != want[i] {
			t.Errorf("cascade call[%d] = %q, want %q", i, c, want[i])
		}
	}
}

func TestDeleteCharacter_InvalidID(t *testing.T) {
	mux := NewRouter(&mockQuerier{}, nil, nil, testFS())

	req := httptest.NewRequest(http.MethodDelete, "/api/characters/notanumber", http.NoBody)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}
