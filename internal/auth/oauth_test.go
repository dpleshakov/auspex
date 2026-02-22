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
// Only UpsertCharacter is overridden because it is the only method called by auth.
type mockQuerier struct {
	store.Querier
	upsertCalled bool
	upsertParams store.UpsertCharacterParams
}

func (m *mockQuerier) UpsertCharacter(ctx context.Context, arg store.UpsertCharacterParams) error {
	m.upsertCalled = true
	m.upsertParams = arg
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
	// Set up a server that handles token exchange and /verify.
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "tok",
			"token_type":    "Bearer",
			"refresh_token": "ref",
			"expires_in":    1200,
		})
	})
	mux.HandleFunc("/verify", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(verifyResponse{CharacterID: 1, CharacterName: "X"})
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	mq := &mockQuerier{}
	p := newTestProvider(t, ts.Client(), mq)
	p.conf.Endpoint.TokenURL = ts.URL + "/token"
	p.verifyURL = ts.URL + "/verify"

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
		json.NewEncoder(w).Encode(verifyResponse{
			CharacterID:   12345,
			CharacterName: "TinkerBear",
		})
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

// TestHandleCallback_ValidFlow exercises the full happy path:
// valid state → token exchange → /verify → UpsertCharacter.
func TestHandleCallback_ValidFlow(t *testing.T) {
	mq := &mockQuerier{}

	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "access-abc",
			"token_type":    "Bearer",
			"refresh_token": "refresh-xyz",
			"expires_in":    3600,
		})
	})
	mux.HandleFunc("/verify", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(verifyResponse{
			CharacterID:   99999,
			CharacterName: "BearPilot",
		})
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	p := newTestProvider(t, ts.Client(), mq)
	p.conf.Endpoint.TokenURL = ts.URL + "/token"
	p.verifyURL = ts.URL + "/verify"

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

	// State must be consumed — second call must fail.
	p.mu.Lock()
	_, stillPresent := p.states[state]
	p.mu.Unlock()
	if stillPresent {
		t.Fatal("state was not removed after successful callback")
	}
}
