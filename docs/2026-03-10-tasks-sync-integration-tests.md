# 2026-03-10-tasks-sync-integration-tests.md

**Status:** Active

### Context

`testing-strategy.md` defines "3. Sync Integration Tests" as the next layer after the API contract tests.
The existing `internal/sync/*_test.go` tests use `mockQuerier` + `mockESIClient` — fast and focused but they only prove the worker calls the right interfaces with the right arguments.
What is missing is verification that the **full pipeline** works end-to-end: real ESI fixture → real ESI HTTP client → real SQLite (in-memory) → correct database state.

The new tests live in `internal/sync/` (same package, so white-box access is preserved).
They are normal `go test` tests — no build tags, no external dependencies, run as part of `make build`.

### Exclusions

Items from `testing-strategy.md` section 3 "What to cover" that are deliberately excluded:

- *Force-refresh bypasses cache_until* — already covered by `TestForceRefresh_IgnoresFreshCache` in `worker_test.go`.
- *Token refresh via `auth.TokenRefresher`* — the sync worker does not perform OAuth token refresh; it reads tokens from the DB. Belongs to the `auth` package.

---

### TASK-01 `sync-integration-harness`
**Type:** Regular
**Description:**

Create the shared infrastructure required by all sync integration tests.

**Fixtures** — `internal/sync/testdata/`:

| File | Contents |
|------|----------|
| `character_blueprints.json` | 2 BPOs (`quantity=-1`), type_ids 5000 + 5001, NPC station `location_id=60003760` |
| `corporation_blueprints.json` | 1 BPO with `location_flag=CorpSAG1`, `location_id=1052718829000` (office item ID) |
| `character_jobs.json` | 1 `active` job + 1 `ready` job + 1 `delivered` job; `blueprint_id` values match the character blueprints fixture |
| `corporation_jobs.json` | 1 `active` job; `blueprint_id` matches the corporation blueprint fixture |
| `universe_type_5000.json` | Universe type response for type_id 5000 (TypeName, GroupID, GroupName, CategoryID, CategoryName) |
| `universe_type_5001.json` | Same shape for type_id 5001 |

Type IDs 5000/5001 are chosen because they are guaranteed absent from a fresh DB, so the type resolution path is always exercised.

**Helpers** — `internal/sync/integration_helpers_test.go`:

- `newIntegrationDB(t *testing.T) *sql.DB` — calls `db.Open(":memory:")`, registers `t.Cleanup(db.Close)`.
- `newIntegrationWorker(t *testing.T, sqlDB *sql.DB, esiServerURL string) *Worker` — constructs real `*store.Queries` + `esi.NewClient()` wired with a URL-rewriting `http.RoundTripper` (`hostOverrideTransport`) that replaces the host in every outgoing request with the test server address while preserving the path. Returns the `*Worker`.
- `newESIServer(t *testing.T, routes map[string]string) *httptest.Server` — starts a `httptest.NewServer` whose mux serves fixture files as `application/json`. `routes` maps URL path prefixes (e.g. `"/latest/characters/90000001/blueprints/"`) to fixture file paths read from `testdata/`. Sets an `Expires` header 10 minutes in the future on every response. Registers `t.Cleanup(srv.Close)`.
- `hostOverrideTransport` (unexported struct) implements `http.RoundTripper`; rewrites `req.URL.Scheme` to `"http"` and `req.URL.Host` to the test server host:port. Preserves all other request fields.
- `seedIntegrationCharacter(t, db, id, corpID)` and `seedIntegrationCorporation(t, db, id, delegateID)` — thin wrappers around raw SQL inserts (mirror the pattern in `internal/api/testdb_test.go`).

**Definition of done:** fixtures present, helpers compile, `go test ./internal/sync/...` still passes (no regressions).

**Status:** ✅ Done

---

### TASK-02 `sync-integration-blueprints`
**Type:** Regular
**Description:**

End-to-end blueprint sync tests using the harness from TASK-01.
Each test seeds rows, starts a fake ESI server serving the relevant fixtures, calls `w.syncSubject(ctx, ownerType, ownerID, "blueprints")`, and queries the real SQLite DB to verify state.

**Tests:**

`TestSyncIntegration_CharacterBlueprints_RowsMatchFixture`
- Seed: character ID=90000001 (no corporation)
- ESI routes: blueprint endpoint → `character_blueprints.json`; universe type endpoints → `universe_type_5000.json`, `universe_type_5001.json`
- Assert:
  - `SELECT COUNT(*) FROM blueprints WHERE owner_type='character' AND owner_id=90000001` = 2
  - `me_level`, `te_level`, `location_id` match fixture values for each blueprint
  - `SELECT COUNT(*) FROM eve_types` = 2 (type resolution ran)
  - `SELECT cache_until FROM sync_state WHERE owner_type='character' AND endpoint='blueprints'` is in the future

`TestSyncIntegration_CharacterBlueprints_SecondSync_Upserts`
- Run sync once (me_level=10 in fixture), then modify the in-memory fixture server to return me_level=5, run sync again
- Assert: blueprint row shows me_level=5 (upsert, not duplicate)

`TestSyncIntegration_CorporationBlueprints_RowsMatchFixture`
- Seed: character ID=90000001 + corporation ID=99000001 (delegate=90000001)
- ESI routes: corp blueprint endpoint → `corporation_blueprints.json`; universe type; `/corporations/99000001/offices/` → offices response with officeID→stationID mapping; `/universe/names/` → station name response
- Assert: blueprint row exists; `eve_locations` has a row for the office item ID with a real station name

`TestSyncIntegration_TypeResolution_FKIntegrity`
- Before sync: DB is empty
- After character blueprint sync: verify that `eve_categories`, `eve_groups`, `eve_types` rows all exist and FK constraints hold (SELECT with JOIN)

**Definition of done:** all new tests pass, `go test ./internal/sync/...` green.

**Status:** ✅ Done

---

### TASK-03 `sync-integration-jobs`
**Type:** Regular
**Description:**

End-to-end job sync tests using the harness from TASK-01.
Blueprints must exist before jobs can be inserted (FK constraint); seed them via `seedBlueprint`-style helper or a preceding `syncSubject` call for blueprints.

**Tests:**

`TestSyncIntegration_CharacterJobs_OnlyActiveAndReadyStored`
- Seed: character ID=90000001 + blueprint rows for IDs referenced in `character_jobs.json`
- ESI route: character jobs endpoint → `character_jobs.json` (contains 1 active + 1 ready + 1 delivered)
- Assert: `SELECT COUNT(*) FROM jobs WHERE owner_type='character'` = 2 (delivered filtered by ESI client, never reaches DB)

`TestSyncIntegration_CharacterJobs_SyncStateUpdated`
- Same setup as above
- Assert: `sync_state.cache_until` for `endpoint='jobs'` is in the future

`TestSyncIntegration_StaleJobsDeleted`
- Seed: character + blueprint rows + two job rows (IDs 700000001 and 700000002) directly in DB
- ESI route: fixture returning only job 700000001
- Assert after sync: `SELECT COUNT(*) FROM jobs WHERE owner_type='character'` = 1; job 700000002 absent

`TestSyncIntegration_CorporationJobs_Stored`
- Seed: character + corporation + corp blueprint row
- ESI route: `corporation_jobs.json`
- Assert: job row exists with `owner_type='corporation'` and correct field values

**Definition of done:** all new tests pass, `go test ./internal/sync/...` green.

**Status:** ⬜ Pending

---

### TASK-04 `review`
**Type:** Review
**Covers:** TASK-01, TASK-02, TASK-03
**Description:**
- Code: correctness, test isolation (no shared mutable state between tests), naming consistency with existing `_test.go` files in the package
- Security: no real tokens in fixtures, no outbound network calls, `t.Cleanup` registered for every server and DB
- Documentation: verify `technical-reference.md` and `architecture.md` still reflect reality; update only if something changed
- Run `make build` and `make lint` and confirm both pass clean
**Status:** ⬜ Pending

---

### TASK-05 `docs`
**Type:** Docs
**Description:**
- These are internal tests with no user-visible behavior change → no CHANGELOG entry needed; confirm this explicitly
- Verify `testing-strategy.md` section 3 still matches what was implemented; update if any scope diverged
- Update `MEMORY.md` active task pointer after this file is archived
**Status:** ⬜ Pending
