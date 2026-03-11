## 2026-03-11-tasks-healthcheck.md

**Status:** Active

### Contracts

**New ESI endpoint consumed:**
- `GET /latest/status/` — public, no token — ESI reachability check
  - New method `GetStatus() error` added to `Client` interface (`internal/esi/client.go`)
  - Implemented in new `internal/esi/status.go`

**New HTTP endpoints exposed:**
- `GET /healthz` — liveness; always returns `200 OK`, no dependencies checked
- `GET /readyz` — readiness; checks DB (`SELECT 1`) and ESI (`GetStatus()`); returns JSON `200` / `503`

```json
{
  "status": "ok",
  "checks": {
    "database": "ok",
    "esi": "ok"
  }
}
```

**New CLI subcommand:**
- `auspex healthcheck` — detected via `os.Args[1]` check in `main.go` before flag parsing; fully self-contained

---

### TASK-01 `esi-status`
**Type:** Regular
**Description:**
- Add `GetStatus() error` to the `Client` interface in `internal/esi/client.go`
- Implement in new `internal/esi/status.go`: call `GET /latest/status/`; return `nil` if response is `200`, error otherwise
- Unit tests in `internal/esi/status_test.go`
- Update `docs/technical-reference.md` to document the new ESI endpoint
**Definition of done:** working code + tests + committed
**Status:** ⬜ Pending

---

### TASK-02 `health-endpoints`
**Type:** Regular
**Description:**
- Create `internal/api/health.go` with `liveHandler` and `readyHandler`
- `readyHandler` runs `SELECT 1` against the DB and calls `GetStatus()` on the ESI client; returns structured JSON (`200` all OK, `503` any failure)
- Add ESI `Client` to the `Server` struct in `internal/api/` (currently holds `worker`, `store`, `auth`)
- Register `/healthz` and `/readyz` routes in `internal/api/router.go` before the wildcard static-file handler
- Unit tests for both handlers (mock ESI client + in-memory DB)
- Update `docs/technical-reference.md` to document the two new HTTP endpoints
**Definition of done:** working code + tests + committed
**Status:** ⬜ Pending

---

### TASK-03 `healthcheck-cmd`
**Type:** Regular
**Description:**
- In `cmd/auspex/main.go`: detect `os.Args[1] == "healthcheck"` before flag parsing; if matched, delegate to `runHealthcheck()` and `os.Exit` with its return code
- Create `cmd/auspex/healthcheck.go` implementing `runHealthcheck() int`:
  - Load and validate config (required fields present, `callback_url` well-formed)
  - Open DB, run `SELECT 1`
  - Call `GetStatus()` to check ESI reachability
  - List characters from store; for each: refresh token via auth, parse JWT `scp` claim, verify required character scopes (`esi-industry-character-jobs-v1`, `esi-characters-blueprints-v1`)
  - List corporations from store; for each: find delegate character, refresh token, verify required corporation scopes (`esi-industry-corporation-jobs-v1`, `esi-corporations-blueprints-v1`, `esi-corporations-read-corporation-membership-v1`)
  - Print human-readable report matching the format in `docs/healthcheck-strategy.md`
  - Return `0` if all checks pass, `1` if any fail
- Unit test for scope-comparison logic
**Definition of done:** working code + tests + committed
**Status:** ⬜ Pending

---

### TASK-04 `smoke-test`
**Type:** Smoke test
**Description:**
- Run `go build ./cmd/auspex/`
- Start the server; `curl localhost:8080/healthz` → `200 OK`
- `curl localhost:8080/readyz` → JSON body with `"status": "ok"`
- `./auspex healthcheck` → all checks pass, exit code `0`
**Status:** ⬜ Pending

---

### TASK-05 `review`
**Type:** Review
**Covers:** TASK-01, TASK-02, TASK-03, TASK-04
**Description:**
- Code: security, error handling, readability, obvious performance issues
- Security: input validation, no tokens in logs, errors do not expose internal details, dependency vulnerability check (`npm audit`, `go mod tidy`, `golangci-lint`)
- Documentation: verify `docs/technical-reference.md` matches what was actually built — update if not; verify `docs/architecture.md` — update if module responsibilities or interactions changed
**Status:** ⬜ Pending

---

### TASK-06 `docs`
**Type:** Docs
**Description:**
- Update user-facing documentation (README, help, guides) if behaviour visible to the user has changed
- Verify `docs/technical-reference.md` is up to date — all new endpoints (`/healthz`, `/readyz`, `GET /latest/status/`) and the `healthcheck` subcommand must be reflected
- Update `CHANGELOG.md` — one entry covering the `auspex healthcheck` command and the `/healthz`/`/readyz` HTTP endpoints, following the format in `docs/process-changelog.md`
**Status:** ⬜ Pending
