package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/dpleshakov/auspex/internal/store"
)

// mockQuerier implements store.Querier for tests.
// Embed the interface so unimplemented methods panic clearly if called.
type mockQuerier struct {
	store.Querier
	upsertCalled             bool
	upsertParams             store.UpsertCharacterParams
	insertOrIgnoreCorpCalled bool
	insertOrIgnoreCorpParams store.InsertOrIgnoreCorporationParams
}

// addCharCorpHandlers registers /characters/{id}/ and /corporations/{id}/ handlers
// on the given mux so that HandleCallback can fetch corporation info.
func addCharCorpHandlers(t *testing.T, mux *http.ServeMux, corporationID int64, corporationName string) {
	t.Helper()
	mux.HandleFunc("/characters/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(characterInfoResponse{CorporationID: corporationID}); err != nil {
			t.Fatalf("encode character info: %v", err)
		}
	})
	mux.HandleFunc("/corporations/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(corporationInfoResponse{Name: corporationName}); err != nil {
			t.Fatalf("encode corporation info: %v", err)
		}
	})
}

func (m *mockQuerier) UpsertCharacter(ctx context.Context, arg store.UpsertCharacterParams) error {
	m.upsertCalled = true
	m.upsertParams = arg
	return nil
}

func (m *mockQuerier) InsertOrIgnoreCorporation(_ context.Context, arg store.InsertOrIgnoreCorporationParams) error {
	m.insertOrIgnoreCorpCalled = true
	m.insertOrIgnoreCorpParams = arg
	return nil
}

// newTestProvider creates a Provider with the given httpClient and querier.
// Defaults are used when arguments are nil.
func newTestProvider(t *testing.T, httpClient *http.Client, q store.Querier) *Provider {
	t.Helper()
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	if q == nil {
		q = &mockQuerier{}
	}
	return NewProvider("test-client-id", "test-client-secret", "http://localhost/callback", q, httpClient)
}

// TestGenerateAuthURL_ContainsState verifies that the returned URL contains a
// non-empty state parameter and that the same state is recorded internally.
func TestGenerateAuthURL_ContainsState(t *testing.T) {
	p := newTestProvider(t, nil, nil)

	authURL, err := p.GenerateAuthURL()
	if err != nil {
		t.Fatalf("GenerateAuthURL error: %v", err)
	}

	u, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("returned URL is not parseable: %v", err)
	}

	state := u.Query().Get("state")
	if state == "" {
		t.Fatal("state parameter missing from auth URL")
	}

	p.mu.Lock()
	_, stored := p.states[state]
	p.mu.Unlock()

	if !stored {
		t.Fatal("state value not stored internally after GenerateAuthURL")
	}
}

// TestGenerateAuthURL_UniqueStates verifies that two calls produce different states.
func TestGenerateAuthURL_UniqueStates(t *testing.T) {
	p := newTestProvider(t, nil, nil)

	url1, _ := p.GenerateAuthURL()
	url2, _ := p.GenerateAuthURL()

	if url1 == url2 {
		t.Fatal("two GenerateAuthURL calls returned identical URLs; states must be unique")
	}
}

// TestHandleCallback_InvalidState verifies that an unrecognized state returns an error.
func TestHandleCallback_InvalidState(t *testing.T) {
	p := newTestProvider(t, nil, nil)

	_, err := p.HandleCallback(context.Background(), "anycode", "not-a-valid-state")
	if err == nil {
		t.Fatal("expected error for invalid state, got nil")
	}
}

// TestHandleCallback_StateConsumedOnce verifies that a state cannot be reused.
func TestHandleCallback_StateConsumedOnce(t *testing.T) {
	// Set up a server that handles token exchange, /verify, and ESI endpoints.
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "tok",
			"token_type":    "Bearer",
			"refresh_token": "ref",
			"expires_in":    1200,
		})
		if err != nil {
			t.Fatalf("encode: %v", err)
		}
	})
	mux.HandleFunc("/verify", func(w http.ResponseWriter, r *http.Request) {
		err := json.NewEncoder(w).Encode(verifyResponse{CharacterID: 1, CharacterName: "X"})
		if err != nil {
			t.Fatalf("encode: %v", err)
		}
	})
	addCharCorpHandlers(t, mux, 500001, "TestCorp")
	ts := httptest.NewServer(mux)
	defer ts.Close()

	mq := &mockQuerier{}
	p := newTestProvider(t, ts.Client(), mq)
	p.conf.Endpoint.TokenURL = ts.URL + "/token"
	p.verifyURL = ts.URL + "/verify"
	p.esiBaseURL = ts.URL

	_, err := p.GenerateAuthURL()
	if err != nil {
		t.Fatal(err)
	}

	p.mu.Lock()
	var state string
	for s := range p.states {
		state = s
	}
	p.mu.Unlock()

	// First call: should succeed.
	if _, err := p.HandleCallback(context.Background(), "code", state); err != nil {
		t.Fatalf("first callback failed: %v", err)
	}

	// Second call with same state: must fail.
	if _, err := p.HandleCallback(context.Background(), "code", state); err == nil {
		t.Fatal("expected error on second use of same state, got nil")
	}
}

// TestCallVerify_ParsesResponse verifies that callVerify correctly parses the
// JSON response from the /verify endpoint.
func TestCallVerify_ParsesResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify that the Authorization header was sent.
		if auth := r.Header.Get("Authorization"); auth != "Bearer test-access-token" {
			http.Error(w, "missing auth header", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(verifyResponse{
			CharacterID:   12345,
			CharacterName: "TinkerBear",
		})
		if err != nil {
			t.Fatalf("encode: %v", err)
		}
	}))
	defer ts.Close()

	p := newTestProvider(t, ts.Client(), nil)
	p.verifyURL = ts.URL

	got, err := p.callVerify(context.Background(), "test-access-token")
	if err != nil {
		t.Fatalf("callVerify error: %v", err)
	}
	if got.CharacterID != 12345 {
		t.Errorf("CharacterID = %d, want 12345", got.CharacterID)
	}
	if got.CharacterName != "TinkerBear" {
		t.Errorf("CharacterName = %q, want %q", got.CharacterName, "TinkerBear")
	}
}

// TestCallVerify_NonOKStatus verifies that a non-200 response from /verify
// is returned as an error.
func TestCallVerify_NonOKStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer ts.Close()

	p := newTestProvider(t, ts.Client(), nil)
	p.verifyURL = ts.URL

	_, err := p.callVerify(context.Background(), "bad-token")
	if err == nil {
		t.Fatal("expected error for non-200 verify response, got nil")
	}
}

// TestCallVerify_ZeroCharacterID verifies that a response with CharacterID == 0
// is rejected as invalid.
func TestCallVerify_ZeroCharacterID(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(verifyResponse{CharacterID: 0, CharacterName: ""})
		if err != nil {
			t.Fatalf("encode: %v", err)
		}
	}))
	defer ts.Close()

	p := newTestProvider(t, ts.Client(), nil)
	p.verifyURL = ts.URL

	_, err := p.callVerify(context.Background(), "some-token")
	if err == nil {
		t.Fatal("expected error for zero CharacterID, got nil")
	}
}

// TestHandleCallback_ValidFlow exercises the full happy path:
// valid state → token exchange → /verify → character info → corporation info → UpsertCharacter.
func TestHandleCallback_ValidFlow(t *testing.T) {
	mq := &mockQuerier{}

	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "access-abc",
			"token_type":    "Bearer",
			"refresh_token": "refresh-xyz",
			"expires_in":    3600,
		})
		if err != nil {
			t.Fatalf("encode: %v", err)
		}
	})
	mux.HandleFunc("/verify", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(verifyResponse{
			CharacterID:   99999,
			CharacterName: "BearPilot",
		})
		if err != nil {
			t.Fatalf("encode: %v", err)
		}
	})
	addCharCorpHandlers(t, mux, 98000001, "Caldari State")
	ts := httptest.NewServer(mux)
	defer ts.Close()

	p := newTestProvider(t, ts.Client(), mq)
	p.conf.Endpoint.TokenURL = ts.URL + "/token"
	p.verifyURL = ts.URL + "/verify"
	p.esiBaseURL = ts.URL

	// Seed a valid state.
	if _, err := p.GenerateAuthURL(); err != nil {
		t.Fatal(err)
	}
	p.mu.Lock()
	var state string
	for s := range p.states {
		state = s
	}
	p.mu.Unlock()

	charID, err := p.HandleCallback(context.Background(), "auth-code", state)
	if err != nil {
		t.Fatalf("HandleCallback error: %v", err)
	}
	if charID != 99999 {
		t.Errorf("returned charID = %d, want 99999", charID)
	}
	if !mq.upsertCalled {
		t.Fatal("UpsertCharacter was not called")
	}
	if mq.upsertParams.Name != "BearPilot" {
		t.Errorf("upserted name = %q, want %q", mq.upsertParams.Name, "BearPilot")
	}
	if mq.upsertParams.AccessToken != "access-abc" {
		t.Errorf("upserted access token = %q, want %q", mq.upsertParams.AccessToken, "access-abc")
	}
	if mq.upsertParams.RefreshToken != "refresh-xyz" {
		t.Errorf("upserted refresh token = %q, want %q", mq.upsertParams.RefreshToken, "refresh-xyz")
	}
	if mq.upsertParams.CorporationID != 98000001 {
		t.Errorf("upserted corporation_id = %d, want 98000001", mq.upsertParams.CorporationID)
	}
	if mq.upsertParams.CorporationName != "Caldari State" {
		t.Errorf("upserted corporation_name = %q, want %q", mq.upsertParams.CorporationName, "Caldari State")
	}

	// State must be consumed — second call must fail.
	p.mu.Lock()
	_, stillPresent := p.states[state]
	p.mu.Unlock()
	if stillPresent {
		t.Fatal("state was not removed after successful callback")
	}

	// 98000001 is a player corp — InsertOrIgnoreCorporation must be called.
	if !mq.insertOrIgnoreCorpCalled {
		t.Fatal("InsertOrIgnoreCorporation was not called for player corporation")
	}
	if mq.insertOrIgnoreCorpParams.ID != 98000001 {
		t.Errorf("corp ID = %d, want 98000001", mq.insertOrIgnoreCorpParams.ID)
	}
	if mq.insertOrIgnoreCorpParams.DelegateID != 99999 {
		t.Errorf("delegate ID = %d, want 99999 (the character ID)", mq.insertOrIgnoreCorpParams.DelegateID)
	}
	if mq.insertOrIgnoreCorpParams.Name != "Caldari State" {
		t.Errorf("corp name = %q, want %q", mq.insertOrIgnoreCorpParams.Name, "Caldari State")
	}
}

// TestHandleCallback_NPCCorporation verifies that NPC corporations (IDs 1000000–2000000)
// are not inserted into the corporations table.
func TestHandleCallback_NPCCorporation(t *testing.T) {
	mq := &mockQuerier{}

	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "access-npc",
			"token_type":    "Bearer",
			"refresh_token": "refresh-npc",
			"expires_in":    3600,
		})
		if err != nil {
			t.Fatalf("encode: %v", err)
		}
	})
	mux.HandleFunc("/verify", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(verifyResponse{CharacterID: 11111, CharacterName: "NPCChar"})
		if err != nil {
			t.Fatalf("encode: %v", err)
		}
	})
	// NPC corp ID within 1000000–2000000.
	addCharCorpHandlers(t, mux, 1000182, "Center for Advanced Studies")
	ts := httptest.NewServer(mux)
	defer ts.Close()

	p := newTestProvider(t, ts.Client(), mq)
	p.conf.Endpoint.TokenURL = ts.URL + "/token"
	p.verifyURL = ts.URL + "/verify"
	p.esiBaseURL = ts.URL

	if _, err := p.GenerateAuthURL(); err != nil {
		t.Fatal(err)
	}
	p.mu.Lock()
	var state string
	for s := range p.states {
		state = s
	}
	p.mu.Unlock()

	if _, err := p.HandleCallback(context.Background(), "npc-code", state); err != nil {
		t.Fatalf("HandleCallback error: %v", err)
	}

	if mq.insertOrIgnoreCorpCalled {
		t.Fatal("InsertOrIgnoreCorporation must not be called for NPC corporation")
	}
}
