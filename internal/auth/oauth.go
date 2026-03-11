// Package auth implements the EVE SSO OAuth2 flow.
// Responsibilities: generate authorization URL, exchange code for tokens,
// refresh tokens on expiry, verify character identity via /verify.
// Uses golang.org/x/oauth2.
package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"

	"golang.org/x/oauth2"

	"github.com/dpleshakov/auspex/internal/store"
)

// ErrInvalidState is returned by HandleCallback when the OAuth state parameter
// does not match any pending authorization request.
var ErrInvalidState = errors.New("invalid or expired OAuth state")

// oauth.go: authorization URL generation, code→token exchange, /verify call.

//nolint:gosec // G101: these are public EVE SSO endpoints, not credentials
const (
	eveAuthURL   = "https://login.eveonline.com/v2/oauth/authorize"
	eveTokenURL  = "https://login.eveonline.com/v2/oauth/token"
	eveVerifyURL = "https://esi.evetech.net/verify"
	esiBaseURL   = "https://esi.evetech.net/latest"
)

// eveScopes are the ESI OAuth2 scopes required for Auspex MVP.
var eveScopes = []string{
	"esi-assets.read_corporation_assets.v1",
	"esi-characters.read_blueprints.v1",
	"esi-corporations.read_blueprints.v1",
	"esi-corporations.read_facilities.v1",
	"esi-industry.read_character_jobs.v1",
	"esi-industry.read_corporation_jobs.v1",
	"esi-universe.read_structures.v1",
}

// verifyResponse is the JSON payload returned by EVE SSO /verify.
type verifyResponse struct {
	CharacterID   int64  `json:"CharacterID"`
	CharacterName string `json:"CharacterName"`
}

// characterInfoResponse is the relevant subset of GET /characters/{id}/.
type characterInfoResponse struct {
	CorporationID int64 `json:"corporation_id"`
}

// corporationInfoResponse is the relevant subset of GET /corporations/{id}/.
type corporationInfoResponse struct {
	Name string `json:"name"`
}

// Provider manages the EVE SSO OAuth2 authorization code flow.
// It is safe for concurrent use.
type Provider struct {
	conf       *oauth2.Config
	store      store.Querier
	httpClient *http.Client
	verifyURL  string
	esiBaseURL string
	states     map[string]struct{}
	mu         sync.Mutex
}

// NewProvider constructs a Provider using the given EVE SSO credentials and store.
// httpClient is used for the /verify call and injected into OAuth2 token exchange.
// Pass nil to use http.DefaultClient.
func NewProvider(clientID, clientSecret, callbackURL string, q store.Querier, httpClient *http.Client) *Provider {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Provider{
		conf: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  callbackURL,
			Scopes:       eveScopes,
			Endpoint: oauth2.Endpoint{
				AuthURL:  eveAuthURL,
				TokenURL: eveTokenURL,
			},
		},
		store:      q,
		httpClient: httpClient,
		verifyURL:  eveVerifyURL,
		esiBaseURL: esiBaseURL,
		states:     make(map[string]struct{}),
	}
}

// GenerateAuthURL returns the EVE SSO authorization URL and the random state value.
// The state is stored internally and consumed exactly once by HandleCallback.
func (p *Provider) GenerateAuthURL() (string, error) {
	state, err := randomState()
	if err != nil {
		return "", fmt.Errorf("generating OAuth state: %w", err)
	}
	p.mu.Lock()
	p.states[state] = struct{}{}
	p.mu.Unlock()
	return p.conf.AuthCodeURL(state), nil
}

// HandleCallback validates the OAuth2 state, exchanges the authorization code
// for tokens, verifies the character via ESI /verify, and upserts the character
// in the store. Returns the character ID on success.
func (p *Provider) HandleCallback(ctx context.Context, code, state string) (int64, error) {
	p.mu.Lock()
	_, valid := p.states[state]
	if valid {
		delete(p.states, state)
	}
	p.mu.Unlock()

	if !valid {
		return 0, ErrInvalidState
	}

	// Inject the custom HTTP client so oauth2 uses it for the token exchange.
	// This enables testing without real network calls.
	ctx = context.WithValue(ctx, oauth2.HTTPClient, p.httpClient)

	token, err := p.conf.Exchange(ctx, code)
	if err != nil {
		return 0, fmt.Errorf("exchanging authorization code: %w", err)
	}

	char, err := p.callVerify(ctx, token.AccessToken)
	if err != nil {
		return 0, fmt.Errorf("verifying character: %w", err)
	}

	charInfo, err := p.callCharacterInfo(ctx, char.CharacterID)
	if err != nil {
		return 0, fmt.Errorf("fetching character info for %d: %w", char.CharacterID, err)
	}

	corpInfo, err := p.callCorporationInfo(ctx, charInfo.CorporationID)
	if err != nil {
		return 0, fmt.Errorf("fetching corporation info for %d: %w", charInfo.CorporationID, err)
	}

	if err := p.store.UpsertCharacter(ctx, store.UpsertCharacterParams{
		ID:              char.CharacterID,
		Name:            char.CharacterName,
		AccessToken:     token.AccessToken,
		RefreshToken:    token.RefreshToken,
		TokenExpiry:     token.Expiry,
		CorporationID:   charInfo.CorporationID,
		CorporationName: corpInfo.Name,
	}); err != nil {
		return 0, fmt.Errorf("saving character %d: %w", char.CharacterID, err)
	}

	if !isNPCCorporation(charInfo.CorporationID) {
		if err := p.store.InsertOrIgnoreCorporation(ctx, store.InsertOrIgnoreCorporationParams{
			ID:         charInfo.CorporationID,
			Name:       corpInfo.Name,
			DelegateID: char.CharacterID,
		}); err != nil {
			return 0, fmt.Errorf("saving corporation %d: %w", charInfo.CorporationID, err)
		}
	}

	return char.CharacterID, nil
}

// callCharacterInfo fetches the character's public info (corporation_id) from ESI.
func (p *Provider) callCharacterInfo(ctx context.Context, characterID int64) (characterInfoResponse, error) {
	url := fmt.Sprintf("%s/characters/%d/", p.esiBaseURL, characterID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return characterInfoResponse{}, fmt.Errorf("building character info request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req) //nolint:gosec // URL is constructed from a hardcoded base URL + validated character ID
	if err != nil {
		return characterInfoResponse{}, fmt.Errorf("calling character info: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // closing response body on read path

	if resp.StatusCode != http.StatusOK {
		return characterInfoResponse{}, fmt.Errorf("character info returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return characterInfoResponse{}, fmt.Errorf("reading character info response: %w", err)
	}

	var v characterInfoResponse
	if err := json.Unmarshal(body, &v); err != nil {
		return characterInfoResponse{}, fmt.Errorf("parsing character info response: %w", err)
	}

	return v, nil
}

// callCorporationInfo fetches the corporation's public info (name) from ESI.
func (p *Provider) callCorporationInfo(ctx context.Context, corporationID int64) (corporationInfoResponse, error) {
	url := fmt.Sprintf("%s/corporations/%d/", p.esiBaseURL, corporationID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return corporationInfoResponse{}, fmt.Errorf("building corporation info request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req) //nolint:gosec // URL is constructed from a hardcoded base URL + validated corporation ID
	if err != nil {
		return corporationInfoResponse{}, fmt.Errorf("calling corporation info: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // closing response body on read path

	if resp.StatusCode != http.StatusOK {
		return corporationInfoResponse{}, fmt.Errorf("corporation info returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return corporationInfoResponse{}, fmt.Errorf("reading corporation info response: %w", err)
	}

	var v corporationInfoResponse
	if err := json.Unmarshal(body, &v); err != nil {
		return corporationInfoResponse{}, fmt.Errorf("parsing corporation info response: %w", err)
	}

	return v, nil
}

// callVerify calls EVE SSO /verify with the access token and returns character info.
func (p *Provider) callVerify(ctx context.Context, accessToken string) (verifyResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.verifyURL, http.NoBody)
	if err != nil {
		return verifyResponse{}, fmt.Errorf("building verify request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req) //nolint:gosec // URL is sourced from validated config, not user input
	if err != nil {
		return verifyResponse{}, fmt.Errorf("calling /verify: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // closing response body on read path, error is not actionable

	if resp.StatusCode != http.StatusOK {
		return verifyResponse{}, fmt.Errorf("verify returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return verifyResponse{}, fmt.Errorf("reading verify response: %w", err)
	}

	var v verifyResponse
	if err := json.Unmarshal(body, &v); err != nil {
		return verifyResponse{}, fmt.Errorf("parsing verify response: %w", err)
	}

	if v.CharacterID <= 0 {
		return verifyResponse{}, fmt.Errorf("verify returned invalid CharacterID %d", v.CharacterID)
	}

	return v, nil
}

// OAuthConfig returns the underlying *oauth2.Config so that auth.NewClient
// can share the same credentials for automatic token refresh.
func (p *Provider) OAuthConfig() *oauth2.Config {
	return p.conf
}

// isNPCCorporation reports whether the given EVE corporation ID belongs to an
// NPC corporation. NPC corp IDs occupy the range 1000000–2000000 inclusive.
// Player corps fall outside this range and must be tracked in the corporations table.
func isNPCCorporation(corpID int64) bool {
	return corpID >= 1_000_000 && corpID <= 2_000_000
}

// randomState generates a cryptographically random 32-character hex string.
func randomState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
