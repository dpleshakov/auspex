# Auspex — tech-debt.md

Known issues and deferred decisions. Updated after each layer review.

---

## Layer 1 Review — Foundation (config, db, store)

### TD-01 `flag.Parse()` inside `config.Load()` — ✅ Fixed

**File:** `internal/config/config.go`

`Load()` called `flag.Parse()` internally, preventing the caller from
registering flags beforehand — an anti-pattern for library code.

**Fix:** `Load()` changed to `Load(path string)`. Flag registration and
`flag.Parse()` moved to `main.go`. Config package no longer imports `flag`.

---

### TD-02 `esi.callback_url` not validated as a URL — ✅ Fixed

**File:** `internal/config/config.go`

`validate()` only checked that `callback_url` was non-empty.

**Fix:** `url.Parse()` added; validation rejects any value whose scheme is
not `http` or `https`. Test `TestLoadFromFile_InvalidCallbackURL` added.

---

---

## Layer 2 Review — ESI HTTP Client

### TD-04 No pagination for blueprints and jobs endpoints — ⏭ Won't fix (MVP)

**Files:** `internal/esi/blueprints.go`, `internal/esi/jobs.go`

ESI pagination endpoints (`/characters/{id}/blueprints`, `/corporations/{id}/blueprints`,
`/characters/{id}/industry/jobs`, `/corporations/{id}/industry/jobs`) support multiple
pages via `?page=N` and signal the total page count via the `X-Pages` response header.
The current client fetches only the first page.

Characters or corporations with very large BPO libraries (typically large corporations)
will have data silently truncated — no error is returned, only the first page of results
is stored.

**Not a problem for MVP** — personal characters and small/medium corps fit within a single
page. Must be addressed before supporting large fleet-scale corporations. Fix: after each
successful response, check `X-Pages`; if `> 1`, fetch remaining pages and concatenate.

---

### TD-05 Response body size not limited — ⏭ Won't fix (MVP)

**File:** `internal/esi/client.go`

`io.ReadAll(resp.Body)` reads the entire response body without an upper bound.
A misbehaving server (or network interception) could return an arbitrarily large body,
causing unbounded memory growth.

**Not a problem for MVP** — all requests go to the official ESI API over HTTPS, which
returns bounded, well-formed JSON. Fix if auspex is ever extended to support third-party
or user-configurable API endpoints: wrap with `io.LimitReader(resp.Body, maxBodyBytes)`.

---

### TD-03 SQLite single-connection limitation — ⏭ Won't fix (MVP)

**File:** `internal/db/db.go`

`SetMaxOpenConns(1)` limits the database to one concurrent connection. This
is necessary because `PRAGMA foreign_keys` is a per-connection setting and
SQLite does not support concurrent writes. For a local single-user desktop
tool this is correct. If Auspex were ever deployed as a multi-user server,
this would need to be revisited (move to PostgreSQL or use a WAL-tuned
SQLite setup with proper connection-level PRAGMA hooks).

**Not a problem for MVP.**

---

## Layer 3 Review — Auth (oauth.go, client.go)

### TD-06 `callVerify`: status check after `io.ReadAll` — ✅ Fixed

**File:** `internal/auth/oauth.go`

The original code called `io.ReadAll(resp.Body)` before checking
`resp.StatusCode`. A misbehaving server returning a large error body would
exhaust memory before the status check could reject it. Additionally, the
raw error body was interpolated into the error message, which could expose
sensitive content in logs.

**Fix:** Status check moved before `io.ReadAll`. Error message for non-200
responses no longer includes the response body (only the status code).
Test `TestCallVerify_NonOKStatus` continues to pass unchanged.

---

### TD-07 `Provider.states` map grows without bound — ⏭ Won't fix (MVP)

**File:** `internal/auth/oauth.go`

Every call to `GenerateAuthURL()` adds an entry to `p.states`. Abandoned
OAuth sessions (user opens login URL but never completes the flow) are never
evicted. For a local desktop tool with one user and infrequent logins this
is inconsequential. Fix if needed: add a TTL-based eviction (store
`time.Time` alongside the state and sweep entries older than N minutes in
a background goroutine).

**Not a problem for MVP.**

---

### TD-08 `callVerify` did not validate `CharacterID > 0` — ✅ Fixed

**File:** `internal/auth/oauth.go`

A malformed or empty response from EVE SSO (e.g. `{"CharacterID": 0}`)
would be stored silently, corrupting the `characters` table with an invalid
ID=0 row.

**Fix:** Added validation `v.CharacterID <= 0 → error` after JSON parsing.
Test `TestCallVerify_ZeroCharacterID` added.

---

### TD-09 `tokenForCharacter` issues a store read on every ESI call — ⏭ Won't fix (MVP)

**File:** `internal/auth/client.go`

Each ESI call (blueprints, jobs) triggers a `store.GetCharacter` round-trip
to SQLite to retrieve the current token. A single sync cycle for one
character makes 2 such reads (blueprints + jobs), and for a corporation 4
reads (2 character queries + 2 corporation→delegate queries). With SQLite
on a local disk this is fast, but at scale (many characters, frequent
syncs) it adds up.

**Not a problem for MVP.** Fix if needed: cache the token in memory with
a short TTL (shorter than the access token lifetime) to avoid repeated
store reads within a single sync cycle.
