package esi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// noopSleep replaces time.Sleep so retry tests complete instantly.
func noopSleep(_ time.Duration) {}

// newTestClient builds an httpClient pointed at srv with retries disabled for time.
func newTestClient(srv *httptest.Server) *httpClient {
	c := NewClient(srv.Client())
	c.baseURL = srv.URL
	c.sleep = noopSleep
	return c
}

// --- parseExpires ---

func TestParseExpires_Valid(t *testing.T) {
	s := "Wed, 22 Feb 2026 12:00:00 GMT"
	got := parseExpires(s)
	want, _ := http.ParseTime(s)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseExpires_Empty(t *testing.T) {
	before := time.Now()
	got := parseExpires("")
	after := time.Now()
	if got.Before(before) || got.After(after) {
		t.Errorf("expected time.Now() fallback, got %v", got)
	}
}

func TestParseExpires_Malformed(t *testing.T) {
	before := time.Now()
	got := parseExpires("not-a-date")
	after := time.Now()
	if got.Before(before) || got.After(after) {
		t.Errorf("expected time.Now() fallback for malformed header, got %v", got)
	}
}

// --- parseRetryAfter ---

func TestParseRetryAfter_Valid(t *testing.T) {
	got := parseRetryAfter("30")
	if got != 30*time.Second {
		t.Errorf("got %v, want 30s", got)
	}
}

func TestParseRetryAfter_Empty(t *testing.T) {
	if got := parseRetryAfter(""); got != time.Second {
		t.Errorf("got %v, want 1s fallback", got)
	}
}

func TestParseRetryAfter_Malformed(t *testing.T) {
	if got := parseRetryAfter("soon"); got != time.Second {
		t.Errorf("got %v, want 1s fallback", got)
	}
}

func TestParseRetryAfter_Negative(t *testing.T) {
	if got := parseRetryAfter("-5"); got != time.Second {
		t.Errorf("got %v, want 1s fallback", got)
	}
}

// --- do: success path ---

func TestDo_Success(t *testing.T) {
	const expiresHeader = "Wed, 22 Feb 2026 12:00:00 GMT"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Expires", expiresHeader)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	body, cacheUntil, err := c.do(context.Background(), srv.URL, "tok123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(body) != `{"ok":true}` {
		t.Errorf("body: got %q, want %q", body, `{"ok":true}`)
	}
	want, _ := http.ParseTime(expiresHeader)
	if !cacheUntil.Equal(want) {
		t.Errorf("cacheUntil: got %v, want %v", cacheUntil, want)
	}
}

func TestDo_AuthorizationHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	c.do(context.Background(), srv.URL, "mytoken")
	if gotAuth != "Bearer mytoken" {
		t.Errorf("Authorization: got %q, want %q", gotAuth, "Bearer mytoken")
	}
}

func TestDo_NoAuthHeader_WhenTokenEmpty(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	c.do(context.Background(), srv.URL, "")
	if gotAuth != "" {
		t.Errorf("expected no Authorization header, got %q", gotAuth)
	}
}

// --- do: retry on 429 ---

func TestDo_Retry429_SucceedsAfterRetries(t *testing.T) {
	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n < 3 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	var slept []time.Duration
	c := NewClient(srv.Client())
	c.sleep = func(d time.Duration) { slept = append(slept, d) }

	_, _, err := c.do(context.Background(), srv.URL, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls.Load() != 3 {
		t.Errorf("expected 3 calls, got %d", calls.Load())
	}
	if len(slept) != 2 {
		t.Errorf("expected 2 sleeps, got %d", len(slept))
	}
	for _, d := range slept {
		if d != time.Second {
			t.Errorf("expected Retry-After=1s sleep, got %v", d)
		}
	}
}

func TestDo_Retry429_ExhaustedReturnsError(t *testing.T) {
	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, _, err := c.do(context.Background(), srv.URL, "")
	if err == nil {
		t.Fatal("expected error after exhausted retries, got nil")
	}
	if got := calls.Load(); got != maxRetries+1 {
		t.Errorf("expected %d calls, got %d", maxRetries+1, got)
	}
}

// --- do: retry on 5xx ---

func TestDo_Retry5xx_SucceedsAfterRetries(t *testing.T) {
	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	var slept []time.Duration
	c := NewClient(srv.Client())
	c.sleep = func(d time.Duration) { slept = append(slept, d) }

	_, _, err := c.do(context.Background(), srv.URL, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls.Load() != 3 {
		t.Errorf("expected 3 calls, got %d", calls.Load())
	}
	// Delays must follow exponential backoff: 1s, 2s.
	wantDelays := []time.Duration{1 * time.Second, 2 * time.Second}
	if len(slept) != len(wantDelays) {
		t.Fatalf("expected %d sleeps, got %d", len(wantDelays), len(slept))
	}
	for i, want := range wantDelays {
		if slept[i] != want {
			t.Errorf("sleep[%d]: got %v, want %v", i, slept[i], want)
		}
	}
}

func TestDo_Retry5xx_ExhaustedReturnsError(t *testing.T) {
	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, _, err := c.do(context.Background(), srv.URL, "")
	if err == nil {
		t.Fatal("expected error after exhausted retries, got nil")
	}
	if got := calls.Load(); got != maxRetries+1 {
		t.Errorf("expected %d calls, got %d", maxRetries+1, got)
	}
}

// --- do: no retry on 4xx ---

func TestDo_NoRetry4xx(t *testing.T) {
	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"not found"}`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, _, err := c.do(context.Background(), srv.URL, "")
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
	if calls.Load() != 1 {
		t.Errorf("expected exactly 1 call (no retry), got %d", calls.Load())
	}
}
