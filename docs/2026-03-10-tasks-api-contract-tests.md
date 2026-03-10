## 2026-03-10-tasks-api-contract-tests.md

**Status:** Active

### Contracts

No new API endpoints or schema changes — this feature adds test coverage for existing contracts.

Endpoints covered:
- `GET /api/characters`
- `DELETE /api/characters/{id}`
- `GET /api/corporations`
- `POST /api/corporations`
- `DELETE /api/corporations/{id}`
- `PATCH /api/corporations/{id}/delegate`
- `GET /api/blueprints`
- `GET /api/jobs/summary`
- `POST /api/sync`
- `GET /api/sync/status`

---

### TASK-01 `test-db-helper`
**Type:** Regular
**Description:**
Create `internal/api/testdb_test.go` with test infrastructure shared by all contract tests:
- `newContractDB(t *testing.T) *sql.DB` — opens an in-memory SQLite database (`:memory:`) and applies all migrations from `internal/db/migrations/` in order; fails the test on any migration error
- `newContractServer(t *testing.T, db *sql.DB) *httptest.Server` — wires `store.New(db)`, a no-op `WorkerRefresher` stub, and an empty `auth.Provider` stub; constructs the Chi router via `api.NewRouter`; starts and registers `t.Cleanup` to close the server
- Seed helpers (package-level functions in the same file):
  - `seedCorporation(t, db, id, name, delegateID)` — inserts a row into `corporations`
  - `seedCharacter(t, db, id, name, corpID)` — inserts a row into `characters`
  - `seedSyncState(t, db, ownerType, ownerID, endpoint, cacheUntil, lastError)` — inserts a row into `sync_state`
  - `seedBlueprint(t, db, b BlueprintSeed)` — inserts a row into `blueprints`; `BlueprintSeed` is a local struct with all required columns and sensible zero-value defaults
  - `seedJob(t, db, j JobSeed)` — inserts a row into `jobs`; `JobSeed` is a local struct

All seed helpers call `t.Helper()` and `t.Fatal` on insert failure.

Write a minimal smoke test `TestContractDB_MigrationsApply` that calls `newContractDB` and verifies the `blueprints` table exists.

**Definition of done:** working code + tests + committed
**Status:** ✅ Done

---

### TASK-02 `characters-corps-contract`
**Type:** Regular
**Description:**
Create `internal/api/characters_contract_test.go` and `internal/api/corporations_contract_test.go`
using the helpers from TASK-01.

**Characters (`GET /api/characters`):**
- Empty DB → 200, body is `[]` (not `null`)
- Single character with no corporation → 200, response contains fields: `id` (number), `name` (string), `corporation_id` (null), `corporation_name` (null), `is_delegate` (bool), `sync_error` (null), `created_at` (string)
- Character with a corporation and a non-null `sync_error` → verify `corporation_id`, `corporation_name`, `sync_error` are non-null with correct types

**Corporations (`GET /api/corporations`):**
- Empty DB → 200, body is `[]`
- One corporation → response contains fields: `id` (number), `name` (string), `delegate_id` (number)

**Corporations (`POST /api/corporations`):**
- Valid request with existing delegate character → 201, response body matches `corporationJSON` shape: `id`, `name`, `delegate_id`

**Corporations (`PATCH /api/corporations/{id}/delegate`):**
- Valid delegate update → 200, response body matches `corporationJSON` shape

For each test: decode response into `map[string]any`, assert required keys exist with correct Go types. Do not assert on specific values — only shape and nullability.

**Definition of done:** working code + tests + committed
**Status:** ⬜ Pending

---

### TASK-03 `blueprints-jobs-sync-contract`
**Type:** Regular
**Description:**
Create `internal/api/blueprints_contract_test.go` and `internal/api/sync_contract_test.go`
using the helpers from TASK-01.

**Blueprints (`GET /api/blueprints`):**
- Empty DB → 200, body is `[]`
- Blueprint with no active job → response item contains: `id`, `type_id`, `type_name`, `owner_type`, `owner_id`, `runs`, `material_efficiency`, `time_efficiency`, `location_id`, `location_name`, `status`; job-related fields (`job_id`, `activity`, `end_date`, `probability`) are `null`
- Blueprint with an associated active job → job-related fields are non-null with correct types: `job_id` (number), `activity` (string), `end_date` (string), `probability` (number or null)
- Filtering: `?owner_type=character&owner_id=X` returns only matching blueprints; unknown `status` value returns 400

**Jobs summary (`GET /api/jobs/summary`):**
- Empty DB → 200, response contains: `idle_blueprints` (number), `overdue_jobs` (number), `completing_today` (number), `characters` (array, may be empty)
- With seeded characters and jobs → `characters` array items contain: `character_id` (number), `character_name` (string), `used_slots` (number), `max_slots` (number)

**Sync status (`GET /api/sync/status`):**
- Empty DB → 200, body is `[]`
- With seeded sync_state rows → items contain: `owner_type` (string), `owner_id` (number), `owner_name` (string or null), `endpoint` (string), `cache_until` (string or null), `last_synced_at` (string or null), `last_error` (string or null)

**Post sync (`POST /api/sync`):**
- → 202 Accepted, empty body

**Definition of done:** working code + tests + committed
**Status:** ⬜ Pending

---

### TASK-04 `review`
**Type:** Review
**Covers:** TASK-01, TASK-02, TASK-03
**Description:**
- Code: readability of test helpers and seed functions; no duplication between contract tests and existing mock-based tests; helper functions call `t.Helper()`; JSON shape assertions are clear and intentional
- Security: no real credentials or tokens in test fixtures; test files are `_test.go` only (not compiled into production binary)
- Documentation: verify `technical-reference.md` still accurately describes all API response shapes — update if any discrepancy is found between the documented contract and what the tests actually assert; verify `architecture.md` — no changes expected
**Status:** ⬜ Pending

---

### TASK-05 `docs`
**Type:** Docs
**Description:**
- Update user-facing documentation (README) only if any externally visible behaviour changed — no changes expected for a test-only feature
- Verify `technical-reference.md` is up to date — confirm all JSON response shapes for the 10 endpoints above are documented; add or correct any missing fields found during TASK-02 and TASK-03
- Update `CHANGELOG.md` following the format in `process-changelog.md` — add one entry under the current release section noting that API contract tests were added
**Status:** ⬜ Pending
