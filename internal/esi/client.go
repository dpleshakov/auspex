// Package esi is the HTTP client for the EVE ESI API.
// Responsibilities: make HTTP requests, return typed structs, respect ESI cache headers.
// Has no knowledge of the database.
// Reads the Expires header from ESI responses and returns cache_until to callers.
// Handles ESI errors (429, 5xx) with retry logic.
package esi

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const (
	// BaseURL is the root of the ESI API.
	BaseURL = "https://esi.evetech.net/latest"

	// maxRetries is the maximum number of retry attempts after the initial request.
	maxRetries = 3

	// maxRetryAfterDelay caps the Retry-After sleep to prevent the client from
	// blocking indefinitely on a misbehaving or adversarial ESI response.
	maxRetryAfterDelay = 60 * time.Second
)

// Client is the interface used by the sync and auth packages.
// It allows substituting a mock ESI client in tests without a real network.
//
// Note: GetUniverseType is implemented in TASK-06 (universe.go).
// The compile-time assertion var _ Client = (*httpClient)(nil) is added there.
type Client interface {
	GetCharacterBlueprints(ctx context.Context, characterID int64, token string) ([]Blueprint, time.Time, error)
	GetCorporationBlueprints(ctx context.Context, corporationID int64, token string) ([]Blueprint, time.Time, error)
	GetCharacterJobs(ctx context.Context, characterID int64, token string) ([]Job, time.Time, error)
	GetCorporationJobs(ctx context.Context, corporationID int64, token string) ([]Job, time.Time, error)
	GetUniverseType(ctx context.Context, typeID int64) (UniverseType, error)
}

// httpClient is the concrete implementation of Client.
type httpClient struct {
	http    *http.Client
	sleep   func(time.Duration) // injectable for testing; defaults to time.Sleep
	baseURL string              // defaults to BaseURL; overridden in tests
}

// NewClient constructs an httpClient using the provided *http.Client.
// The returned *httpClient satisfies the Client interface once all methods are implemented.
// Passing a custom *http.Client (e.g. from httptest.NewServer) enables unit testing.
func NewClient(h *http.Client) *httpClient {
	return &httpClient{
		http:    h,
		sleep:   time.Sleep,
		baseURL: BaseURL,
	}
}

// do executes a GET request to url with the given Bearer token.
// It retries on 429 (honouring Retry-After) and 5xx (exponential backoff),
// up to maxRetries times. Returns the raw response body and the parsed Expires header.
// A 4xx response other than 429 is returned as an error without retrying.
func (c *httpClient) do(ctx context.Context, url, token string) ([]byte, time.Time, error) {
	for attempt := 0; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, time.Time{}, fmt.Errorf("building request: %w", err)
		}
		req.Header.Set("Accept", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}

		resp, err := c.http.Do(req)
		if err != nil {
			return nil, time.Time{}, fmt.Errorf("sending request: %w", err)
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, time.Time{}, fmt.Errorf("reading response body: %w", err)
		}

		cacheUntil := parseExpires(resp.Header.Get("Expires"))

		switch {
		case resp.StatusCode == http.StatusTooManyRequests:
			if attempt == maxRetries {
				return nil, cacheUntil, fmt.Errorf("ESI 429 after %d retries", maxRetries)
			}
			c.sleep(parseRetryAfter(resp.Header.Get("Retry-After")))
			if ctx.Err() != nil {
				return nil, time.Time{}, ctx.Err()
			}

		case resp.StatusCode >= 500:
			if attempt == maxRetries {
				return nil, cacheUntil, fmt.Errorf("ESI status %d after %d retries", resp.StatusCode, maxRetries)
			}
			// Exponential backoff: 1s, 2s, 4s for attempts 0, 1, 2.
			c.sleep(time.Duration(1<<uint(attempt)) * time.Second)
			if ctx.Err() != nil {
				return nil, time.Time{}, ctx.Err()
			}

		case resp.StatusCode >= 400:
			return nil, cacheUntil, fmt.Errorf("ESI status %d: %s", resp.StatusCode, body)

		default:
			return body, cacheUntil, nil
		}
	}

	// Unreachable: every path in the last iteration returns explicitly.
	return nil, time.Time{}, fmt.Errorf("ESI request failed after %d retries", maxRetries)
}

// parseExpires parses the RFC1123 Expires header returned by ESI.
// If the header is missing or malformed, time.Now() is returned so the
// caller treats the response as already expired.
func parseExpires(s string) time.Time {
	if s == "" {
		return time.Now()
	}
	t, err := http.ParseTime(s)
	if err != nil {
		return time.Now()
	}
	return t
}

// parseRetryAfter parses the Retry-After header (integer seconds).
// Falls back to 1 second when the header is absent or cannot be parsed.
func parseRetryAfter(s string) time.Duration {
	if s == "" {
		return time.Second
	}
	secs, err := strconv.Atoi(s)
	if err != nil || secs < 0 {
		return time.Second
	}
	d := time.Duration(secs) * time.Second
	if d > maxRetryAfterDelay {
		return maxRetryAfterDelay
	}
	return d
}
