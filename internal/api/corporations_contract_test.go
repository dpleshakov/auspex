package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestContract_GetCorporations_EmptyDB(t *testing.T) {
	sqlDB := newContractDB(t)
	srv := newContractServer(t, sqlDB)

	resp, err := http.Get(srv.URL + "/api/corporations")
	if err != nil {
		t.Fatalf("GET /api/corporations: %v", err)
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

func TestContract_GetCorporations_OneCorporation(t *testing.T) {
	sqlDB := newContractDB(t)
	seedCharacter(t, sqlDB, 2001, "Delegate", 88888)
	seedCorporation(t, sqlDB, 88888, "Test Corp", 2001)
	srv := newContractServer(t, sqlDB)

	resp, err := http.Get(srv.URL + "/api/corporations")
	if err != nil {
		t.Fatalf("GET /api/corporations: %v", err)
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
		t.Fatalf("expected 1 corporation, got %d", len(items))
	}
	corp := items[0]

	assertField[float64](t, corp, "id")
	assertField[string](t, corp, "name")
	assertField[float64](t, corp, "delegate_id")
	assertField[string](t, corp, "delegate_name")
	assertField[string](t, corp, "created_at")
}

func TestContract_PostCorporations_ValidRequest(t *testing.T) {
	sqlDB := newContractDB(t)
	seedCharacter(t, sqlDB, 2002, "Delegate", 0)
	srv := newContractServer(t, sqlDB)

	body := `{"id": 77777, "name": "New Corp", "delegate_id": 2002}`
	resp, err := http.Post(
		srv.URL+"/api/corporations",
		"application/json",
		strings.NewReader(body),
	)
	if err != nil {
		t.Fatalf("POST /api/corporations: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Handler returns 201 with no body on success.
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
}

func TestContract_PatchCorporationDelegate_ValidRequest(t *testing.T) {
	sqlDB := newContractDB(t)
	// Both characters belong to the same corporation.
	const corpID int64 = 66666
	seedCharacter(t, sqlDB, 2003, "OldDelegate", corpID)
	seedCharacter(t, sqlDB, 2004, "NewDelegate", corpID)
	seedCorporation(t, sqlDB, corpID, "Corp Beta", 2003)
	srv := newContractServer(t, sqlDB)

	patchBody := `{"character_id": 2004}`
	req, err := http.NewRequest(
		http.MethodPatch,
		srv.URL+"/api/corporations/66666/delegate",
		strings.NewReader(patchBody),
	)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH /api/corporations/66666/delegate: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Handler returns 204 No Content on success.
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
}
