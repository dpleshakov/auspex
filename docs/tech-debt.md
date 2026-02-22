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
