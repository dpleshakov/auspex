package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dpleshakov/auspex/internal/auth"
)

// mockAuthProvider implements AuthProvider for tests.
type mockAuthProvider struct {
	GenerateAuthURLFn  func() (string, error)
	HandleCallbackFn   func(ctx context.Context, code, state string) (int64, error)
}

func (m *mockAuthProvider) GenerateAuthURL() (string, error) {
	if m.GenerateAuthURLFn != nil {
		return m.GenerateAuthURLFn()
	}
	return "https://login.eveonline.com/auth?state=teststate", nil
}

func (m *mockAuthProvider) HandleCallback(ctx context.Context, code, state string) (int64, error) {
	if m.HandleCallbackFn != nil {
		return m.HandleCallbackFn(ctx, code, state)
	}
	return 12345, nil
}

// --- GET /auth/eve/login ---

func TestHandleLogin_RedirectsToEVESSO(t *testing.T) {
	authURL := "https://login.eveonline.com/v2/oauth/authorize?state=abc"
	mux := NewRouter(&mockQuerier{}, nil, &mockAuthProvider{
		GenerateAuthURLFn: func() (string, error) {
			return authURL, nil
		},
	}, testFS())

	req := httptest.NewRequest(http.MethodGet, "/auth/eve/login", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rr.Code)
	}
	if got := rr.Header().Get("Location"); got != authURL {
		t.Errorf("Location = %q, want %q", got, authURL)
	}
}

func TestHandleLogin_500WhenGenerateFails(t *testing.T) {
	mux := NewRouter(&mockQuerier{}, nil, &mockAuthProvider{
		GenerateAuthURLFn: func() (string, error) {
			return "", errors.New("rng failure")
		},
	}, testFS())

	req := httptest.NewRequest(http.MethodGet, "/auth/eve/login", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

// --- GET /auth/eve/callback ---

func TestHandleCallback_ValidStateRedirectsToRoot(t *testing.T) {
	worker := &mockWorker{}
	mux := NewRouter(&mockQuerier{}, worker, &mockAuthProvider{
		HandleCallbackFn: func(ctx context.Context, code, state string) (int64, error) {
			return 12345, nil
		},
	}, testFS())

	req := httptest.NewRequest(http.MethodGet, "/auth/eve/callback?code=mycode&state=mystate", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rr.Code)
	}
	if got := rr.Header().Get("Location"); got != "/" {
		t.Errorf("Location = %q, want /", got)
	}
}

func TestHandleCallback_InvalidStateReturns400(t *testing.T) {
	mux := NewRouter(&mockQuerier{}, nil, &mockAuthProvider{
		HandleCallbackFn: func(ctx context.Context, code, state string) (int64, error) {
			return 0, auth.ErrInvalidState
		},
	}, testFS())

	req := httptest.NewRequest(http.MethodGet, "/auth/eve/callback?code=mycode&state=badstate", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestHandleCallback_MissingParams_Returns400(t *testing.T) {
	mux := NewRouter(&mockQuerier{}, nil, &mockAuthProvider{}, testFS())

	for _, url := range []string{
		"/auth/eve/callback",
		"/auth/eve/callback?code=x",
		"/auth/eve/callback?state=y",
	} {
		req := httptest.NewRequest(http.MethodGet, url, nil)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("GET %s: expected 400, got %d", url, rr.Code)
		}
	}
}

func TestHandleCallback_CallbackError_Returns500(t *testing.T) {
	mux := NewRouter(&mockQuerier{}, nil, &mockAuthProvider{
		HandleCallbackFn: func(ctx context.Context, code, state string) (int64, error) {
			return 0, errors.New("token exchange failed")
		},
	}, testFS())

	req := httptest.NewRequest(http.MethodGet, "/auth/eve/callback?code=x&state=y", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestHandleCallback_SuccessTriggersForceRefresh(t *testing.T) {
	worker := &mockWorker{}
	mux := NewRouter(&mockQuerier{}, worker, &mockAuthProvider{
		HandleCallbackFn: func(ctx context.Context, code, state string) (int64, error) {
			return 12345, nil
		},
	}, testFS())

	req := httptest.NewRequest(http.MethodGet, "/auth/eve/callback?code=mycode&state=mystate", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rr.Code)
	}
	if !worker.forceRefreshCalled {
		t.Error("expected ForceRefresh to be called after successful OAuth callback")
	}
}
