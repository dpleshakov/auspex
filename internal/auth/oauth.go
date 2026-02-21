// Package auth implements the EVE SSO OAuth2 flow.
// Responsibilities: generate authorization URL, exchange code for tokens,
// refresh tokens on expiry, verify character identity via /verify.
// Uses golang.org/x/oauth2.
package auth

// oauth.go: authorization URL generation, code→token exchange, /verify call.
