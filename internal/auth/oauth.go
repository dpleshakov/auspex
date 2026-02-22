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
	"fmt"
	"io"
	"net/http"
	"sync"

	"golang.org/x/oauth2"

	"github.com/dpleshakov/auspex/internal/store"
)

// oauth.go: authorization URL generation, code→token exchange, /verify call.

const (
	eveAuthURL   = "https://login.eveonline.com/v2/oauth/authorize"
	eveTokenURL  = "https://login.eveonline.com/v2/oauth/token"
	eveVerifyURL = "https://esi.evetech.net/verify"
)

// eveScopes are the ESI OAuth2 scopes required for Auspex MVP.
var eveScopes = []string{
	"esi-characters.read_blueprints.v1",
	"esi-corporations.read_blueprints.v1",
	"esi-industry.read_character_jobs.v1",
	"esi-industry.read_corporation_jobs.v1",
}

// verifyResponse is the JSON payload returned by EVE SSO /verify.
type verifyResponse struct {
	CharacterID   int64  `json:"CharacterID"`
	CharacterName string `json:"CharacterName"`
}

// Provider manages the EVE SSO OAuth2 authorization code flow.
// It is safe for concurrent use.
type Provider struct {
	conf       *oauth2.Config
	store      store.Querier
	httpClient *http.Client
	verifyURL  string
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
		return 0, fmt.Errorf("invalid or expired OAuth state")
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

	if err := p.store.UpsertCharacter(ctx, store.UpsertCharacterParams{
		ID:           char.CharacterID,
		Name:         char.CharacterName,
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		TokenExpiry:  token.Expiry,
	}); err != nil {
		return 0, fmt.Errorf("saving character %d: %w", char.CharacterID, err)
	}

	return char.CharacterID, nil
}

// callVerify calls EVE SSO /verify with the access token and returns character info.
func (p *Provider) callVerify(ctx context.Context, accessToken string) (verifyResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.verifyURL, nil)
	if err != nil {
		return verifyResponse{}, fmt.Errorf("building verify request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return verifyResponse{}, fmt.Errorf("calling /verify: %w", err)
	}
	defer resp.Body.Close()

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

// randomState generates a cryptographically random 32-character hex string.
func randomState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
