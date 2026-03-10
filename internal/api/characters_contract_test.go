package api

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func TestContract_GetCharacters_EmptyDB(t *testing.T) {
	sqlDB := newContractDB(t)
	srv := newContractServer(t, sqlDB)

	resp, err := http.Get(srv.URL + "/api/characters")
	if err != nil {
		t.Fatalf("GET /api/characters: %v", err)
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

func TestContract_GetCharacters_NoCorpNoSyncError(t *testing.T) {
	sqlDB := newContractDB(t)
	seedCharacter(t, sqlDB, 1001, "Alpha", 0)
	srv := newContractServer(t, sqlDB)

	resp, err := http.Get(srv.URL + "/api/characters")
	if err != nil {
		t.Fatalf("GET /api/characters: %v", err)
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
		t.Fatalf("expected 1 character, got %d", len(items))
	}
	c := items[0]

	// Required fields with expected Go types after JSON decode.
	assertField[float64](t, c, "id")
	assertField[string](t, c, "name")
	// corporation_id is int64 serialized as number — zero value when not set.
	assertField[float64](t, c, "corporation_id")
	// corporation_name is string — empty string when not set.
	assertField[string](t, c, "corporation_name")
	assertField[bool](t, c, "is_delegate")
	assertNull(t, c, "sync_error")
	assertField[string](t, c, "created_at")
}

func TestContract_GetCharacters_WithCorpAndSyncError(t *testing.T) {
	sqlDB := newContractDB(t)
	const charID int64 = 1002
	const corpID int64 = 55555
	// Character is delegate of the corporation and shares the same corp ID.
	seedCharacter(t, sqlDB, charID, "Delegate", corpID)
	seedCorporation(t, sqlDB, corpID, "Corp Alpha", charID)
	seedSyncState(t, sqlDB, "corporation", corpID, "blueprints",
		time.Now().Add(time.Hour), "ESI 503: service unavailable")
	srv := newContractServer(t, sqlDB)

	resp, err := http.Get(srv.URL + "/api/characters")
	if err != nil {
		t.Fatalf("GET /api/characters: %v", err)
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
		t.Fatalf("expected 1 character, got %d", len(items))
	}
	c := items[0]

	// corporation_id must be non-zero.
	if id, ok := c["corporation_id"].(float64); !ok || id == 0 {
		t.Errorf("corporation_id: want non-zero float64, got %v (%T)", c["corporation_id"], c["corporation_id"])
	}
	assertField[string](t, c, "corporation_name")
	// sync_error must be a non-null string.
	if _, ok := c["sync_error"].(string); !ok {
		t.Errorf("sync_error: want string, got %v (%T)", c["sync_error"], c["sync_error"])
	}
	// Character is the delegate of the corporation.
	if is, ok := c["is_delegate"].(bool); !ok || !is {
		t.Errorf("is_delegate: want true, got %v", c["is_delegate"])
	}
}
