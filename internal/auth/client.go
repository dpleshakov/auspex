package auth

// client.go: auth.Client wraps the esi package and automatically injects
// a fresh access token into every request, refreshing via OAuth2 if needed.
// Reads and writes tokens via the store package.
