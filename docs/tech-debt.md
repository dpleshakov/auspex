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
