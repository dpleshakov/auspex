package auth_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/oauth2"

	"github.com/dpleshakov/auspex/internal/auth"
	"github.com/dpleshakov/auspex/internal/esi"
	"github.com/dpleshakov/auspex/internal/store"
)

// ---------------------------------------------------------------------------
// mock store
// ---------------------------------------------------------------------------

// mockQuerier implements store.Querier for testing.
// Only GetCharacter, GetCorporation, and UpsertCharacter are implemented;
// any other call panics to make unexpected usage obvious.
type mockQuerier struct {
	store.Querier // embed to satisfy interface; unimplemented methods panic

	characters   map[int64]store.Character
	corporations map[int64]store.Corporation
	upsertCalls  []store.UpsertCharacterParams
}

func (m *mockQuerier) GetCharacter(_ context.Context, id int64) (store.Character, error) {
	c, ok := m.characters[id]
	if !ok {
		return store.Character{}, fmt.Errorf("character %d not found", id)
	}
	return c, nil
}

func (m *mockQuerier) GetCorporation(_ context.Context, id int64) (store.Corporation, error) {
	c, ok := m.corporations[id]
	if !ok {
		return store.Corporation{}, fmt.Errorf("corporation %d not found", id)
	}
	return c, nil
}

func (m *mockQuerier) UpsertCharacter(_ context.Context, arg store.UpsertCharacterParams) error {
	m.upsertCalls = append(m.upsertCalls, arg)
	return nil
}

// ---------------------------------------------------------------------------
// mock ESI inner client
// ---------------------------------------------------------------------------

// mockESI records the token passed to each call and returns canned data.
type mockESI struct {
	blueprints []esi.Blueprint
	cacheUntil time.Time
	tokenSeen  string
}

func (m *mockESI) GetCharacterBlueprints(_ context.Context, _ int64, token string) ([]esi.Blueprint, time.Time, error) {
	m.tokenSeen = token
	return m.blueprints, m.cacheUntil, nil
}

func (m *mockESI) GetCorporationBlueprints(_ context.Context, _ int64, token string) ([]esi.Blueprint, time.Time, error) {
	m.tokenSeen = token
	return m.blueprints, m.cacheUntil, nil
}

func (m *mockESI) GetCharacterJobs(_ context.Context, _ int64, token string) ([]esi.Job, time.Time, error) {
	m.tokenSeen = token
	return nil, m.cacheUntil, nil
}

func (m *mockESI) GetCorporationJobs(_ context.Context, _ int64, token string) ([]esi.Job, time.Time, error) {
	m.tokenSeen = token
	return nil, m.cacheUntil, nil
}

func (m *mockESI) GetUniverseType(_ context.Context, _ int64) (esi.UniverseType, error) {
	return esi.UniverseType{}, nil
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// newTokenServer starts an httptest.Server that serves OAuth2 token refresh
// responses with the given access and refresh tokens.
func newTokenServer(t *testing.T, newAccess, newRefresh string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"access_token":  newAccess,
			"refresh_token": newRefresh,
			"token_type":    "Bearer",
			"expires_in":    3600,
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, "encode error", http.StatusInternalServerError)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// newConf returns a minimal oauth2.Config pointed at the given token URL.
func newConf(tokenURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Endpoint: oauth2.Endpoint{
			TokenURL:  tokenURL,
			AuthStyle: oauth2.AuthStyleInParams,
		},
	}
}

// ---------------------------------------------------------------------------
// tests
// ---------------------------------------------------------------------------

// TestClient_FreshToken verifies that a non-expired token is forwarded as-is
// to the inner ESI client without calling the token endpoint.
func TestClient_FreshToken(t *testing.T) {
	// The token server must NOT be called; if it is, it would return "should-not-be-used".
	srv := newTokenServer(t, "should-not-be-used", "")

	q := &mockQuerier{
		characters: map[int64]store.Character{
			42: {
				ID:           42,
				Name:         "Test Pilot",
				AccessToken:  "fresh-token",
				RefreshToken: "refresh-token",
				TokenExpiry:  time.Now().Add(1 * time.Hour),
			},
		},
	}

	inner := &mockESI{}
	client := auth.NewClient(inner, q, newConf(srv.URL), srv.Client())

	_, _, err := client.GetCharacterBlueprints(context.Background(), 42, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if inner.tokenSeen != "fresh-token" {
		t.Errorf("inner ESI received token %q, want %q", inner.tokenSeen, "fresh-token")
	}
	if len(q.upsertCalls) != 0 {
		t.Errorf("UpsertCharacter called %d times, want 0 (token was fresh)", len(q.upsertCalls))
	}
}

// TestClient_ExpiredTokenRefreshed verifies that an expired token is refreshed
// via OAuth2, the inner ESI client receives the new token, and the new credentials
// are persisted to the store.
func TestClient_ExpiredTokenRefreshed(t *testing.T) {
	srv := newTokenServer(t, "new-access-token", "new-refresh-token")

	q := &mockQuerier{
		characters: map[int64]store.Character{
			42: {
				ID:           42,
				Name:         "Test Pilot",
				AccessToken:  "old-access-token",
				RefreshToken: "old-refresh-token",
				TokenExpiry:  time.Now().Add(-1 * time.Hour), // expired
			},
		},
	}

	inner := &mockESI{}
	client := auth.NewClient(inner, q, newConf(srv.URL), srv.Client())

	_, _, err := client.GetCharacterBlueprints(context.Background(), 42, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Inner client must receive the new token, not the old one.
	if inner.tokenSeen != "new-access-token" {
		t.Errorf("inner ESI received token %q, want %q", inner.tokenSeen, "new-access-token")
	}

	// New credentials must be persisted exactly once.
	if len(q.upsertCalls) != 1 {
		t.Fatalf("UpsertCharacter called %d times, want 1", len(q.upsertCalls))
	}
	saved := q.upsertCalls[0]
	if saved.ID != 42 {
		t.Errorf("saved character ID = %d, want 42", saved.ID)
	}
	if saved.AccessToken != "new-access-token" {
		t.Errorf("saved AccessToken = %q, want %q", saved.AccessToken, "new-access-token")
	}
	if saved.RefreshToken != "new-refresh-token" {
		t.Errorf("saved RefreshToken = %q, want %q", saved.RefreshToken, "new-refresh-token")
	}
}

// TestClient_RefreshedTokenSavedForJobs verifies the refresh-and-save path
// also works for GetCharacterJobs (not just blueprints).
func TestClient_RefreshedTokenSavedForJobs(t *testing.T) {
	srv := newTokenServer(t, "jobs-new-token", "jobs-new-refresh")

	q := &mockQuerier{
		characters: map[int64]store.Character{
			7: {
				ID:           7,
				Name:         "Industrialist",
				AccessToken:  "jobs-old-token",
				RefreshToken: "jobs-old-refresh",
				TokenExpiry:  time.Now().Add(-30 * time.Minute),
			},
		},
	}

	inner := &mockESI{}
	client := auth.NewClient(inner, q, newConf(srv.URL), srv.Client())

	_, _, err := client.GetCharacterJobs(context.Background(), 7, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if inner.tokenSeen != "jobs-new-token" {
		t.Errorf("inner ESI received token %q, want %q", inner.tokenSeen, "jobs-new-token")
	}
	if len(q.upsertCalls) != 1 {
		t.Fatalf("UpsertCharacter called %d times, want 1", len(q.upsertCalls))
	}
	if q.upsertCalls[0].AccessToken != "jobs-new-token" {
		t.Errorf("saved token = %q, want %q", q.upsertCalls[0].AccessToken, "jobs-new-token")
	}
}

// TestClient_CorporationUsesDelegate verifies that corporation endpoints
// use the delegate character's token (not a per-corporation token).
func TestClient_CorporationUsesDelegate(t *testing.T) {
	srv := newTokenServer(t, "should-not-be-used", "")

	q := &mockQuerier{
		characters: map[int64]store.Character{
			99: {
				ID:           99,
				Name:         "Director",
				AccessToken:  "delegate-token",
				RefreshToken: "delegate-refresh",
				TokenExpiry:  time.Now().Add(2 * time.Hour), // fresh
			},
		},
		corporations: map[int64]store.Corporation{
			77: {ID: 77, Name: "Test Corp", DelegateID: 99},
		},
	}

	inner := &mockESI{}
	client := auth.NewClient(inner, q, newConf(srv.URL), srv.Client())

	_, _, err := client.GetCorporationBlueprints(context.Background(), 77, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if inner.tokenSeen != "delegate-token" {
		t.Errorf("inner ESI received token %q, want %q", inner.tokenSeen, "delegate-token")
	}
	// Fresh delegate token: no refresh, no upsert.
	if len(q.upsertCalls) != 0 {
		t.Errorf("UpsertCharacter called %d times, want 0", len(q.upsertCalls))
	}
}

// TestClient_CorporationExpiredDelegate verifies that an expired delegate token
// is refreshed and saved before the corporation endpoint is called.
func TestClient_CorporationExpiredDelegate(t *testing.T) {
	srv := newTokenServer(t, "new-delegate-token", "new-delegate-refresh")

	q := &mockQuerier{
		characters: map[int64]store.Character{
			99: {
				ID:           99,
				Name:         "Director",
				AccessToken:  "old-delegate-token",
				RefreshToken: "old-delegate-refresh",
				TokenExpiry:  time.Now().Add(-2 * time.Hour), // expired
			},
		},
		corporations: map[int64]store.Corporation{
			77: {ID: 77, Name: "Test Corp", DelegateID: 99},
		},
	}

	inner := &mockESI{}
	client := auth.NewClient(inner, q, newConf(srv.URL), srv.Client())

	_, _, err := client.GetCorporationJobs(context.Background(), 77, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if inner.tokenSeen != "new-delegate-token" {
		t.Errorf("inner ESI received token %q, want %q", inner.tokenSeen, "new-delegate-token")
	}
	if len(q.upsertCalls) != 1 {
		t.Fatalf("UpsertCharacter called %d times, want 1", len(q.upsertCalls))
	}
	if q.upsertCalls[0].ID != 99 {
		t.Errorf("saved character ID = %d, want 99 (delegate)", q.upsertCalls[0].ID)
	}
}

// TestClient_UniverseTypePassthrough verifies that GetUniverseType
// is forwarded directly to the inner client without any token injection.
func TestClient_UniverseTypePassthrough(t *testing.T) {
	srv := newTokenServer(t, "should-not-be-used", "") // must not be called

	q := &mockQuerier{}
	inner := &mockESI{}
	client := auth.NewClient(inner, q, newConf(srv.URL), srv.Client())

	_, err := client.GetUniverseType(context.Background(), 12345)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// No store calls, no token seen (universe is public endpoint).
	if inner.tokenSeen != "" {
		t.Errorf("tokenSeen = %q, want empty (universe endpoint requires no token)", inner.tokenSeen)
	}
}
