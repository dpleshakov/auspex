package api

import "net/http"

// Handles:
//
//	GET /auth/eve/login
//	GET /auth/eve/callback?code=...&state=...
func (r *router) handleLogin(w http.ResponseWriter, req *http.Request) {
	// TODO(TASK-17): implement — redirect to EVE SSO authorization URL.
	http.Redirect(w, req, "/", http.StatusFound)
}

func (r *router) handleCallback(w http.ResponseWriter, req *http.Request) {
	// TODO(TASK-17): implement — validate state, exchange code, save character.
	http.Redirect(w, req, "/", http.StatusFound)
}
