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
)

// Client is the base ESI HTTP client. It handles Authorization headers,
// Expires header parsing, and retry logic for 429 and 5xx responses.
type Client struct {
	http  *http.Client
	sleep func(time.Duration) // injectable for testing; defaults to time.Sleep
}

// NewClient constructs a Client using the provided *http.Client.
// Passing a custom *http.Client (e.g. from httptest.NewServer) enables unit testing.
func NewClient(httpClient *http.Client) *Client {
	return &Client{
		http:  httpClient,
		sleep: time.Sleep,
	}
}

// do executes a GET request to url with the given Bearer token.
// It retries on 429 (honouring Retry-After) and 5xx (exponential backoff),
// up to maxRetries times. Returns the raw response body and the parsed Expires header.
// A 4xx response other than 429 is returned as an error without retrying.
func (c *Client) do(ctx context.Context, url, token string) ([]byte, time.Time, error) {
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

		case resp.StatusCode >= 500:
			if attempt == maxRetries {
				return nil, cacheUntil, fmt.Errorf("ESI status %d after %d retries", resp.StatusCode, maxRetries)
			}
			// Exponential backoff: 1s, 2s, 4s for attempts 0, 1, 2.
			c.sleep(time.Duration(1<<uint(attempt)) * time.Second)

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
	return time.Duration(secs) * time.Second
}
