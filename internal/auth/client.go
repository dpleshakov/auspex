package auth

// client.go: auth.Client wraps the esi package and automatically injects
// a fresh access token into every request, refreshing via OAuth2 if needed.
// Reads and writes tokens via the store package.

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/oauth2"

	"github.com/dpleshakov/auspex/internal/esi"
	"github.com/dpleshakov/auspex/internal/store"
)

// Compile-time assertion: *Client must satisfy esi.Client.
var _ esi.Client = (*Client)(nil)

// Client wraps an esi.Client and automatically injects a valid access token
// into every authenticated ESI request, refreshing via OAuth2 when needed.
// It implements esi.Client transparently so that the sync worker can treat
// auth.Client and esi.Client interchangeably.
type Client struct {
	inner      esi.Client
	store      store.Querier
	conf       *oauth2.Config
	httpClient *http.Client // injected into OAuth2 context for testability
}

// NewClient returns an auth.Client that wraps inner.
// conf must be the same *oauth2.Config used for the initial authorization flow
// (same client credentials and token URL) so that refresh calls succeed.
// httpClient is used for token refresh HTTP calls; pass nil to use http.DefaultClient.
func NewClient(inner esi.Client, q store.Querier, conf *oauth2.Config, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		inner:      inner,
		store:      q,
		conf:       conf,
		httpClient: httpClient,
	}
}

// GetCharacterBlueprints fetches blueprints for the given character,
// refreshing the access token via OAuth2 if it has expired.
// The token parameter is ignored; the token is sourced from the store.
func (c *Client) GetCharacterBlueprints(ctx context.Context, characterID int64, _ string) ([]esi.Blueprint, time.Time, error) {
	token, err := c.tokenForCharacter(ctx, characterID)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("getting token for character %d: %w", characterID, err)
	}
	return c.inner.GetCharacterBlueprints(ctx, characterID, token)
}

// GetCorporationBlueprints fetches blueprints for the given corporation,
// using the delegate character's token (refreshed if needed).
// The token parameter is ignored.
func (c *Client) GetCorporationBlueprints(ctx context.Context, corporationID int64, _ string) ([]esi.Blueprint, time.Time, error) {
	token, err := c.tokenForCorporation(ctx, corporationID)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("getting token for corporation %d: %w", corporationID, err)
	}
	return c.inner.GetCorporationBlueprints(ctx, corporationID, token)
}

// GetCharacterJobs fetches active and ready industry jobs for the given character.
// The token parameter is ignored.
func (c *Client) GetCharacterJobs(ctx context.Context, characterID int64, _ string) ([]esi.Job, time.Time, error) {
	token, err := c.tokenForCharacter(ctx, characterID)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("getting token for character %d: %w", characterID, err)
	}
	return c.inner.GetCharacterJobs(ctx, characterID, token)
}

// GetCorporationJobs fetches active and ready industry jobs for the given corporation,
// using the delegate character's token (refreshed if needed).
// The token parameter is ignored.
func (c *Client) GetCorporationJobs(ctx context.Context, corporationID int64, _ string) ([]esi.Job, time.Time, error) {
	token, err := c.tokenForCorporation(ctx, corporationID)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("getting token for corporation %d: %w", corporationID, err)
	}
	return c.inner.GetCorporationJobs(ctx, corporationID, token)
}

// GetUniverseType delegates to the inner ESI client without token injection.
// Universe endpoints are public and require no authorization.
func (c *Client) GetUniverseType(ctx context.Context, typeID int64) (esi.UniverseType, error) {
	return c.inner.GetUniverseType(ctx, typeID)
}

// tokenForCharacter returns a valid access token for the character.
// If the stored token is expired it is refreshed via OAuth2 and the updated
// credentials are persisted to the store before being returned.
func (c *Client) tokenForCharacter(ctx context.Context, characterID int64) (string, error) {
	char, err := c.store.GetCharacter(ctx, characterID)
	if err != nil {
		return "", fmt.Errorf("loading character %d from store: %w", characterID, err)
	}

	t := &oauth2.Token{
		AccessToken:  char.AccessToken,
		RefreshToken: char.RefreshToken,
		Expiry:       char.TokenExpiry,
	}

	// Inject the HTTP client so oauth2 uses it for the token refresh call.
	// This makes the refresh path testable without real network calls.
	ctx = context.WithValue(ctx, oauth2.HTTPClient, c.httpClient)

	// TokenSource returns t unchanged if t.Valid() (not expired).
	// If expired, it calls the token URL to obtain a new access token.
	ts := c.conf.TokenSource(ctx, t)
	newToken, err := ts.Token()
	if err != nil {
		return "", fmt.Errorf("refreshing token for character %d: %w", characterID, err)
	}

	// Persist only when the token actually changed (i.e., a refresh occurred).
	if newToken.AccessToken != char.AccessToken {
		if err := c.store.UpsertCharacter(ctx, store.UpsertCharacterParams{
			ID:           char.ID,
			Name:         char.Name,
			AccessToken:  newToken.AccessToken,
			RefreshToken: newToken.RefreshToken,
			TokenExpiry:  newToken.Expiry,
		}); err != nil {
			return "", fmt.Errorf("saving refreshed token for character %d: %w", characterID, err)
		}
	}

	return newToken.AccessToken, nil
}

// tokenForCorporation returns a valid access token for the delegate character
// of the given corporation, refreshing via OAuth2 if needed.
func (c *Client) tokenForCorporation(ctx context.Context, corporationID int64) (string, error) {
	corp, err := c.store.GetCorporation(ctx, corporationID)
	if err != nil {
		return "", fmt.Errorf("loading corporation %d from store: %w", corporationID, err)
	}
	return c.tokenForCharacter(ctx, corp.DelegateID)
}
