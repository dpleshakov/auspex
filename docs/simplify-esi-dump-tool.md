# Simplify esi-dump tool

## Goal

`go run tools/esi-dump.go` with no arguments produces a full snapshot of all
ESI endpoints into `internal/esi/testdata/`. No manual token copying, no flags
to remember.

## Changes

### Remove flags

Remove `-char`, `-corp`, `-out` flags. Keep `-type` (default: 34) for cases
where a specific type ID is needed.

### Automatic authentication

Read `auspex.yaml` (config path: `./auspex.yaml`, same default as the app) to
get `client_id`, `client_secret`, and `callback_url` (token URL is derived the
same way as in `internal/auth`).

Read the first character from `auspex.db` (db path from config). Use their
`access_token` / `refresh_token` to obtain a valid token, refreshing via
OAuth2 if expired. Persist the refreshed token back to the DB (same as
`auth.Client` does today).

The corporation ID is taken from that character's `corporation_id` field in the
`characters` table.

### Error behaviour

- Config file not found → fatal with hint: "copy auspex.example.yaml to
  auspex.yaml and fill in credentials"
- No characters in DB → fatal with hint: "add a character via the app first"
- Token refresh fails → fatal with ESI error message
- Corporation ID is an NPC corporation (1000000–2000000) → skip corporation
  endpoints, print a notice

## What is NOT changing

- Output format: raw ESI JSON, pretty-printed, same as today
- Output directory: `internal/esi/testdata/` (hardcoded)
- Build tag: `//go:build ignore`
- Corporation endpoints still use the same character's token (delegate pattern)