// Package esi is the HTTP client for the EVE ESI API.
// Responsibilities: make HTTP requests, return typed structs, respect ESI cache headers.
// Has no knowledge of the database.
// Reads the Expires header from ESI responses and returns cache_until to callers.
// Handles ESI errors (429, 5xx) with retry logic.
package esi
