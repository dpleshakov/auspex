## 2026-03-11-tasks-simplify-esi-dump.md

**Status:** Active

### Contracts

No new API endpoints or DB schema changes. Dev tooling only.

The tool reuses existing packages:
- `internal/config` — reads `auspex.yaml`
- `internal/db` — opens `auspex.db`
- `internal/store` — reads the first character row
- `internal/auth` — refreshes token, persists back to DB

Output directory (`internal/esi/testdata/`) and output format (raw pretty-printed ESI JSON) are unchanged.

---

### TASK-01 `simplify-esi-dump`
**Type:** Regular
**Description:**
- Remove `-char`, `-corp`, `-out` flags from `tools/esi-dump.go`; keep `-type` (default: 34)
- Read `auspex.yaml` via `internal/config`; fatal with hint if not found: `"copy auspex.example.yaml to auspex.yaml and fill in credentials"`
- Open `auspex.db` (path from config); read the first character via `store.ListCharacters` (or equivalent); fatal with hint if none: `"add a character via the app first"`
- Obtain a valid token: use `auth.Client` to refresh if expired; persist refreshed token back to DB
- Derive `corporation_id` from the character row; if it falls in the NPC range (1000000–2000000), skip corporation endpoints and print a notice
- All other error paths (token refresh failure, config parse error) fatal with the ESI error message
**Definition of done:** working code + committed
**Status:** ⬜ Pending

---

### TASK-02 `smoke-test`
**Type:** Smoke test
**Description:**
- Run `go run tools/esi-dump.go` with no arguments against a local dev environment (real `auspex.yaml` + populated `auspex.db`)
- Verify `internal/esi/testdata/` is populated with fresh JSON files
- Verify that running again does not fail (token persisted correctly)
- If the character belongs to an NPC corp, verify the notice is printed and corp files are not written
**Status:** ⬜ Pending

---

### TASK-03 `review`
**Type:** Review
**Covers:** TASK-01, TASK-02
**Description:**
- Code: error handling completeness (all fatal paths covered), readability, no flags left behind
- Security: no tokens logged or written anywhere outside DB; `auspex.db` access uses the same path resolution as the app
- Documentation: verify `docs/technical-reference.md` reflects the updated tool usage if it documents `esi-dump`; verify `docs/architecture.md` — no structural changes expected but confirm
**Status:** ⬜ Pending

---

### TASK-04 `docs`
**Type:** Docs
**Description:**
- Update user-facing documentation (README, help, guides) if the tool is documented there
- Verify `docs/technical-reference.md` is up to date — update `esi-dump` usage description if present
- Update `CHANGELOG.md` — one entry if the simplification is user-visible (developer UX change); follow the format in `docs/process-changelog.md`
**Status:** ⬜ Pending
