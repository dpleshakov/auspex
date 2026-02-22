# Auspex — tech-debt.md

Known issues and deferred decisions. Updated after each layer review.

---

## Layer 1 Review — Foundation (config, db, store)

### TD-01 `flag.Parse()` inside `config.Load()`

**File:** `internal/config/config.go`

`Load()` calls `flag.Parse()` internally. This is an anti-pattern for library code: the caller cannot register additional flags before parsing happens, and calling `Load()` in tests requires careful flag management. For a single-binary MVP this works, but would become a problem if `main.go` ever needs its own flags defined after the call.

**Acceptable for MVP.** If `main.go` gains additional flags, move `flag.Parse()` to `main.go` and accept the config path as a parameter to `Load(path string)`.

---

### TD-02 `esi.callback_url` not validated as a URL

**File:** `internal/config/config.go`

`validate()` only checks that `callback_url` is non-empty. A malformed value like `"not-a-url"` passes validation and will cause a confusing error later during the OAuth flow when EVE SSO rejects the redirect URI.

**Acceptable for MVP** — misconfiguration produces a clear EVE SSO error at runtime, not a silent failure. Fix: parse with `url.Parse()` and verify the scheme is `http` or `https`.

---

### TD-03 SQLite single-connection limitation

**File:** `internal/db/db.go`

`SetMaxOpenConns(1)` limits the database to one concurrent connection. This is necessary because `PRAGMA foreign_keys` is a per-connection setting and SQLite does not support concurrent writes. For a local single-user desktop tool this is correct. If Auspex were ever deployed as a multi-user server, this would need to be revisited (move to PostgreSQL or use a WAL-tuned SQLite setup with proper connection-level PRAGMA hooks).

**Not a problem for MVP.**

---
