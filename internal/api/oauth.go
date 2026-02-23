package api

import (
	"errors"
	"net/http"

	"github.com/dpleshakov/auspex/internal/auth"
)

// Handles:
//
//	GET /auth/eve/login
//	GET /auth/eve/callback?code=...&state=...

func (r *router) handleLogin(w http.ResponseWriter, req *http.Request) {
	url, err := r.auth.GenerateAuthURL()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate authorization URL")
		return
	}
	http.Redirect(w, req, url, http.StatusFound)
}

func (r *router) handleCallback(w http.ResponseWriter, req *http.Request) {
	code := req.URL.Query().Get("code")
	state := req.URL.Query().Get("state")

	if code == "" || state == "" {
		writeError(w, http.StatusBadRequest, "missing code or state")
		return
	}

	_, err := r.auth.HandleCallback(req.Context(), code, state)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidState) {
			writeError(w, http.StatusBadRequest, "invalid or expired OAuth state")
			return
		}
		writeError(w, http.StatusInternalServerError, "OAuth callback failed")
		return
	}

	r.worker.ForceRefresh()
	http.Redirect(w, req, "/", http.StatusFound)
}
