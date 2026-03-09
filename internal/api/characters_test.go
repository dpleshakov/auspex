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
	syncErrMsg := "ESI 503: service unavailable"
	mock := &mockQuerier{
		ListCharactersWithMetaFn: func(_ context.Context) ([]store.ListCharactersWithMetaRow, error) {
			return []store.ListCharactersWithMetaRow{
				{
					ID: 1, Name: "Alpha", CorporationID: 98765432,
					CorporationName: "Test Corp", IsDelegate: 1,
					SyncError: syncErrMsg,
					CreatedAt: createdAt,
				},
				{
					ID: 2, Name: "Beta", CorporationID: 98765432,
					CorporationName: "Test Corp", IsDelegate: 0,
					SyncError: nil,
					CreatedAt: createdAt,
				},
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
	if got[0]["is_delegate"] != true {
		t.Errorf("expected is_delegate=true for Alpha, got %v", got[0]["is_delegate"])
	}
	if got[0]["sync_error"] != syncErrMsg {
		t.Errorf("expected sync_error=%q, got %v", syncErrMsg, got[0]["sync_error"])
	}
	if got[1]["is_delegate"] != false {
		t.Errorf("expected is_delegate=false for Beta, got %v", got[1]["is_delegate"])
	}
	if got[1]["sync_error"] != nil {
		t.Errorf("expected sync_error=null for Beta, got %v", got[1]["sync_error"])
	}
	// Tokens must never appear in the response.
	for _, field := range []string{"access_token", "refresh_token", "token_expiry"} {
		if _, ok := got[0][field]; ok {
			t.Errorf("field %q must not be exposed in API response", field)
		}
	}
}

func TestGetCharacters_EmptyList(t *testing.T) {
	mock := &mockQuerier{
		ListCharactersWithMetaFn: func(_ context.Context) ([]store.ListCharactersWithMetaRow, error) {
			return nil, nil
		},
	}
	mux := NewRouter(mock, nil, nil, testFS())

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
		ListCharactersWithMetaFn: func(_ context.Context) ([]store.ListCharactersWithMetaRow, error) {
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
		// CorporationID=0 → no corp logic, only character cascade.
		GetCharacterFn: func(_ context.Context, _ int64) (store.Character, error) {
			return store.Character{ID: 7, CorporationID: 0}, nil
		},
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

func TestDeleteCharacter_LastInCorp(t *testing.T) {
	const charID int64 = 10
	const corpID int64 = 98765432
	var calls []string
	mock := &mockQuerier{
		GetCharacterFn: func(_ context.Context, _ int64) (store.Character, error) {
			return store.Character{ID: charID, CorporationID: corpID}, nil
		},
		ListCharactersByCorporationFn: func(_ context.Context, _ int64) ([]store.Character, error) {
			// Only the character being deleted — no others.
			return []store.Character{{ID: charID}}, nil
		},
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
		DeleteCorporationFn: func(_ context.Context, _ int64) error {
			calls = append(calls, "corporation")
			return nil
		},
		DeleteCharacterFn: func(_ context.Context, _ int64) error {
			calls = append(calls, "character")
			return nil
		},
	}
	mux := NewRouter(mock, nil, nil, testFS())

	req := httptest.NewRequest(http.MethodDelete, "/api/characters/10", http.NoBody)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}
	want := []string{
		"blueprints:corporation", "jobs:corporation", "sync_state:corporation", "corporation",
		"blueprints:character", "jobs:character", "sync_state:character", "character",
	}
	if len(calls) != len(want) {
		t.Fatalf("cascade calls = %v, want %v", calls, want)
	}
	for i, c := range calls {
		if c != want[i] {
			t.Errorf("cascade call[%d] = %q, want %q", i, c, want[i])
		}
	}
}

func TestDeleteCharacter_DelegateReassignment(t *testing.T) {
	const charID int64 = 10
	const otherID int64 = 11
	const corpID int64 = 98765432
	var newDelegateID int64
	mock := &mockQuerier{
		GetCharacterFn: func(_ context.Context, _ int64) (store.Character, error) {
			return store.Character{ID: charID, CorporationID: corpID}, nil
		},
		ListCharactersByCorporationFn: func(_ context.Context, _ int64) ([]store.Character, error) {
			return []store.Character{{ID: charID}, {ID: otherID}}, nil
		},
		GetCorporationFn: func(_ context.Context, _ int64) (store.Corporation, error) {
			return store.Corporation{ID: corpID, DelegateID: charID}, nil
		},
		UpdateCorporationDelegateFn: func(_ context.Context, arg store.UpdateCorporationDelegateParams) error {
			newDelegateID = arg.DelegateID
			return nil
		},
	}
	mux := NewRouter(mock, nil, nil, testFS())

	req := httptest.NewRequest(http.MethodDelete, "/api/characters/10", http.NoBody)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}
	if newDelegateID != otherID {
		t.Errorf("expected delegate reassigned to %d, got %d", otherID, newDelegateID)
	}
}

func TestDeleteCharacter_NotDelegate_NoReassignment(t *testing.T) {
	const charID int64 = 10
	const delegateID int64 = 99
	const corpID int64 = 98765432
	reassigned := false
	mock := &mockQuerier{
		GetCharacterFn: func(_ context.Context, _ int64) (store.Character, error) {
			return store.Character{ID: charID, CorporationID: corpID}, nil
		},
		ListCharactersByCorporationFn: func(_ context.Context, _ int64) ([]store.Character, error) {
			return []store.Character{{ID: charID}, {ID: delegateID}}, nil
		},
		GetCorporationFn: func(_ context.Context, _ int64) (store.Corporation, error) {
			// Character 10 is NOT the delegate.
			return store.Corporation{ID: corpID, DelegateID: delegateID}, nil
		},
		UpdateCorporationDelegateFn: func(_ context.Context, _ store.UpdateCorporationDelegateParams) error {
			reassigned = true
			return nil
		},
	}
	mux := NewRouter(mock, nil, nil, testFS())

	req := httptest.NewRequest(http.MethodDelete, "/api/characters/10", http.NoBody)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}
	if reassigned {
		t.Error("delegate must not be reassigned when character is not the delegate")
	}
}

func TestDeleteCharacter_NPCCorp_NoCorpLogic(t *testing.T) {
	const charID int64 = 10
	const npcCorpID int64 = 1_000_002 // NPC range
	corpDataDeleted := false
	mock := &mockQuerier{
		GetCharacterFn: func(_ context.Context, _ int64) (store.Character, error) {
			return store.Character{ID: charID, CorporationID: npcCorpID}, nil
		},
		DeleteCorporationFn: func(_ context.Context, _ int64) error {
			corpDataDeleted = true
			return nil
		},
	}
	mux := NewRouter(mock, nil, nil, testFS())

	req := httptest.NewRequest(http.MethodDelete, "/api/characters/10", http.NoBody)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}
	if corpDataDeleted {
		t.Error("NPC corporation data must not be deleted")
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

func TestDeleteCharacter_GetCharacterError(t *testing.T) {
	mock := &mockQuerier{
		GetCharacterFn: func(_ context.Context, _ int64) (store.Character, error) {
			return store.Character{}, errors.New("db error")
		},
	}
	mux := NewRouter(mock, nil, nil, testFS())

	req := httptest.NewRequest(http.MethodDelete, "/api/characters/42", http.NoBody)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

func TestDeleteCharacter_DeleteBlueprintsError(t *testing.T) {
	mock := &mockQuerier{
		// CorporationID=0 → skip corp logic, go straight to character cascade.
		GetCharacterFn: func(_ context.Context, _ int64) (store.Character, error) {
			return store.Character{ID: 42, CorporationID: 0}, nil
		},
		DeleteBlueprintsByOwnerFn: func(_ context.Context, _ store.DeleteBlueprintsByOwnerParams) error {
			return errors.New("db error")
		},
	}
	mux := NewRouter(mock, nil, nil, testFS())

	req := httptest.NewRequest(http.MethodDelete, "/api/characters/42", http.NoBody)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

func TestDeleteCharacter_DeleteJobsError(t *testing.T) {
	mock := &mockQuerier{
		GetCharacterFn: func(_ context.Context, _ int64) (store.Character, error) {
			return store.Character{ID: 42, CorporationID: 0}, nil
		},
		DeleteJobsByOwnerFn: func(_ context.Context, _ store.DeleteJobsByOwnerParams) error {
			return errors.New("db error")
		},
	}
	mux := NewRouter(mock, nil, nil, testFS())

	req := httptest.NewRequest(http.MethodDelete, "/api/characters/42", http.NoBody)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

func TestDeleteCharacter_DeleteSyncStateError(t *testing.T) {
	mock := &mockQuerier{
		GetCharacterFn: func(_ context.Context, _ int64) (store.Character, error) {
			return store.Character{ID: 42, CorporationID: 0}, nil
		},
		DeleteSyncStateByOwnerFn: func(_ context.Context, _ store.DeleteSyncStateByOwnerParams) error {
			return errors.New("db error")
		},
	}
	mux := NewRouter(mock, nil, nil, testFS())

	req := httptest.NewRequest(http.MethodDelete, "/api/characters/42", http.NoBody)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

func TestDeleteCharacter_DeleteCharacterError(t *testing.T) {
	mock := &mockQuerier{
		GetCharacterFn: func(_ context.Context, _ int64) (store.Character, error) {
			return store.Character{ID: 42, CorporationID: 0}, nil
		},
		DeleteCharacterFn: func(_ context.Context, _ int64) error {
			return errors.New("db error")
		},
	}
	mux := NewRouter(mock, nil, nil, testFS())

	req := httptest.NewRequest(http.MethodDelete, "/api/characters/42", http.NoBody)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}
