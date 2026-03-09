# 2026-03-09-tasks-esi-integration.md

**Status:** Active

### Context

ESI live integration tests are the first item in the implementation order because they generate
golden file fixtures that all subsequent fixture-based parser tests depend on. These tests make
real HTTP calls to the live ESI API using a token supplied via environment variable, skip
gracefully when no token is present, and save parsed Go structs as JSON fixtures in
`internal/esi/testdata/`.

---

### TASK-01 `esi-integration-infra`
**Type:** Regular
**Description:** Test infrastructure — shared helpers used by all integration tests.

- Create `internal/esi/testdata/.gitkeep` to anchor the fixtures directory in git
- Create `internal/esi/integration_test.go` with build tag `//go:build integration`
- Declare `var update = flag.Bool("update", false, "overwrite golden fixtures")` at package level
- Add `TestMain(m *testing.M)` that calls `flag.Parse()` then `os.Exit(m.Run())`
- `mustToken(t *testing.T) string` — reads `ESI_ACCESS_TOKEN`; calls `t.Skip` if empty
- `mustEnvID(t *testing.T, name string) int64` — reads a named env var, parses as int64; calls `t.Skip` if empty or unparseable
- `saveFixture(t *testing.T, name string, v any)` — marshals struct to indented JSON, writes to `testdata/{name}.json`; calls `t.Fatal` on error
- `compareFixture(t *testing.T, name string, got any)` — if `*update`: calls `saveFixture`; else: reads `testdata/{name}.json`, unmarshals into a value of the same type as `got`, compares with `cmp.Diff`; calls `t.Errorf` on diff

**Definition of done:** file compiles with `-tags integration`; all helpers present

---

### TASK-02 `esi-integration-character`
**Type:** Regular
**Description:** Integration tests for character-scoped ESI endpoints.

Two test functions in `internal/esi/integration_test.go`:

- `TestIntegration_GetCharacterBlueprints`
  - Calls `mustToken(t)` and `mustEnvID(t, "ESI_CHARACTER_ID")`
  - Calls `client.GetCharacterBlueprints(ctx, characterID, token)`
  - Asserts no error
  - Asserts returned slice is non-empty
  - Asserts `bps[0].ItemID != 0` and `bps[0].TypeID != 0`
  - Calls `compareFixture(t, "character_blueprints", bps)`

- `TestIntegration_GetCharacterJobs`
  - Same pattern with `mustEnvID(t, "ESI_CHARACTER_ID")`
  - Calls `client.GetCharacterJobs(ctx, characterID, token)`
  - Asserts no error (empty slice is acceptable — character may have no active jobs)
  - Calls `compareFixture(t, "character_jobs", jobs)`

**Definition of done:** both tests compile and skip when env vars are absent

---

### TASK-03 `esi-integration-corporation`
**Type:** Regular
**Description:** Integration tests for corporation-scoped ESI endpoints.

Two test functions in `internal/esi/integration_test.go`:

- `TestIntegration_GetCorporationBlueprints`
  - Calls `mustToken(t)` and `mustEnvID(t, "ESI_CORPORATION_ID")`
  - Calls `client.GetCorporationBlueprints(ctx, corporationID, token)`
  - Asserts no error
  - Calls `compareFixture(t, "corporation_blueprints", bps)`

- `TestIntegration_GetCorporationJobs`
  - Same pattern
  - Calls `client.GetCorporationJobs(ctx, corporationID, token)`
  - Asserts no error
  - Calls `compareFixture(t, "corporation_jobs", jobs)`

**Definition of done:** both tests compile and skip when env vars are absent

---

### TASK-04 `esi-integration-universe`
**Type:** Regular
**Description:** Integration test for the universe type endpoint. No token required.

- `TestIntegration_GetUniverseType`
  - Uses hardcoded `typeID = 34` (Tritanium — a stable, well-known item)
  - Calls `client.GetUniverseType(ctx, 34)`
  - Asserts no error
  - Asserts `ut.TypeName != ""`
  - Asserts `ut.CategoryID != 0`
  - Calls `compareFixture(t, "universe_type_34", ut)`

**Definition of done:** test compiles and runs without any env vars set

---

### TASK-05 `review`
**Type:** Review
**Covers:** TASK-01, TASK-02, TASK-03, TASK-04
**Description:**
- Code: error handling, readability, no dead code
- Security: `ESI_ACCESS_TOKEN` value is never logged or written into fixture files; fixture JSON contains only parsed struct fields, not raw request details
- Test correctness: `compareFixture` comparison is on parsed struct, not raw bytes (new ESI fields we don't parse are silently ignored)
- Verify `go.mod` / `go.sum` are consistent after adding `github.com/google/go-cmp` as direct dependency
- Documentation: verify `technical-reference.md` is unaffected (no API or schema changes in this feature)
**Status:** ⬜ Pending

---

### TASK-06 `docs`
**Type:** Docs
**Description:**
- No user-visible changes → no CHANGELOG entry
- Add a "Running Integration Tests" section to `docs/testing-strategy.md` documenting:
  - The three env vars (`ESI_ACCESS_TOKEN`, `ESI_CHARACTER_ID`, `ESI_CORPORATION_ID`) and their skip behaviour
  - The `-update` flag and its role in refreshing fixture files
  - The two run commands (normal run + fixture refresh)
  - Where fixture files live and how they feed into parser tests
- Verify `technical-reference.md` is unaffected
**Status:** ⬜ Pending

---

## Run commands

```bash
# Normal run — skips tests whose env vars are absent:
ESI_ACCESS_TOKEN=eyJ... ESI_CHARACTER_ID=12345 ESI_CORPORATION_ID=67890 \
  go test -tags integration ./internal/esi/...

# Refresh golden fixtures:
ESI_ACCESS_TOKEN=eyJ... ESI_CHARACTER_ID=12345 ESI_CORPORATION_ID=67890 \
  go test -tags integration -args -update ./internal/esi/...
```

## Env vars

| Variable | Required by | Skip behaviour |
|----------|-------------|----------------|
| `ESI_ACCESS_TOKEN` | all authed tests | `t.Skip` if empty |
| `ESI_CHARACTER_ID` | character tests | `t.Skip` if empty |
| `ESI_CORPORATION_ID` | corporation tests | `t.Skip` if empty |

## Fixture files produced

| File | Produced by |
|------|-------------|
| `internal/esi/testdata/character_blueprints.json` | `TestIntegration_GetCharacterBlueprints` |
| `internal/esi/testdata/character_jobs.json` | `TestIntegration_GetCharacterJobs` |
| `internal/esi/testdata/corporation_blueprints.json` | `TestIntegration_GetCorporationBlueprints` |
| `internal/esi/testdata/corporation_jobs.json` | `TestIntegration_GetCorporationJobs` |
| `internal/esi/testdata/universe_type_34.json` | `TestIntegration_GetUniverseType` |
