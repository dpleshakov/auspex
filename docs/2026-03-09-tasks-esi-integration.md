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
**Status:** ✅ Done

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
**Status:** ✅ Done

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
**Status:** ✅ Done

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
**Status:** ✅ Done

---

### TASK-04a `esi-integration-rethink`
**Type:** Regular
**Description:** Redesign what the integration tests actually verify.

**Problem with the original design:**
`compareFixture` compared the live ESI response (parsed and re-serialized) against a saved
golden file. This comparison is not meaningful: live data changes daily, so any diff is a
false positive. The comparison neither validates parsing logic (that is covered by the unit
tests in `blueprints_test.go`, `jobs_test.go`, etc., which use `httptest.Server` with
inline JSON) nor gives useful signal in CI.

**What integration tests should do:**
- Call the real ESI endpoint with a live token
- Verify the call succeeded (no error)
- Verify structurally valid results: non-zero IDs on the first element where real data is
  guaranteed to exist (blueprints always has items; universe type 34 always has a name)
- Do nothing else — no file comparison, no exhaustive field checks

**What changed from the original plan:**
The `-dump` flag approach was **not implemented in the integration tests**. Instead, JSON
dumping was extracted to a standalone tool: `tools/esi-dump.go` (build tag `//go:build ignore`,
run via `go run tools/esi-dump.go`). The tests themselves are now purely assertive — they
never write any files. `saveFixture` is retained in `integration_test.go` as an unexported
helper but is not called by any test function.

**Important constraint — what the dump tool saves:**
`tools/esi-dump.go` marshals the **parsed Go struct** back to JSON. This is NOT the same as
the raw HTTP response body from ESI. The raw bytes are consumed inside each `Get*` method and
are not available at the call site. The files in `testdata/` are therefore reference snapshots
of what the parser currently returns for real data, not raw ESI payloads.

**Changes to `integration_test.go`:**

1. Remove `-dump` / `-update` flags entirely. No package-level flag variables.

2. Remove `compareFixture` — no file comparison logic remains in the tests.

3. Wrap element-level field assertions in a length guard so empty responses do not fail:
```go
if len(bps) > 0 {
    if bps[0].ItemID == 0 {
        t.Error("GetCharacterBlueprints: bps[0].ItemID is 0")
    }
    if bps[0].TypeID == 0 {
        t.Error("GetCharacterBlueprints: bps[0].TypeID is 0")
    }
}
```
   Empty slice is a valid ESI response (character may have no active jobs, corporation may
   have no blueprints). Assertions at element level only apply where data is guaranteed.

4. Remove the `github.com/google/go-cmp/cmp` import — it is no longer used in
   `integration_test.go`. The unit tests (`blueprints_test.go`, etc.) use manual field
   comparisons and do not import `go-cmp` either, so the dependency can be dropped from
   `go.mod` if it has no other direct users. Verify with `go mod tidy`.

**New file: `tools/esi-dump.go`** (build tag `//go:build ignore`)
Standalone dump utility. Flags: `-out`, `-char`, `-corp`, `-type`. With no flags, all
endpoints are attempted. Writes `{outDir}/{name}.json` for each fetched endpoint. Exits
with a non-zero code on any error (no `t.Skip` semantics — all env vars are required).

**Definition of done:**
- Normal run (`go test -tags integration ...`): verifies no ESI error and structurally
  valid parser output; no file is read or written
- Dump run (`go run tools/esi-dump.go`): saves re-serialized struct JSON to `testdata/`
  for human inspection; completely separate from the test binary
- No test fails on a valid empty ESI response (empty slice, zero active jobs)
- `go-cmp` import removed from `integration_test.go`; `go mod tidy` does not add it back
- `compareFixture` removed from `integration_test.go`
**Status:** ✅ Done

---

### TASK-05 `review`
**Type:** Review
**Covers:** TASK-01, TASK-02, TASK-03, TASK-04, TASK-04a
**Description:**
- Code: error handling, readability, no dead code
- Security: `ESI_ACCESS_TOKEN` value is never logged or written into fixture files; fixture JSON contains only parsed struct fields (re-serialized from the Go struct), not raw HTTP response details
- Test correctness: element-level assertions are guarded by `len > 0`; no test fails on a valid empty ESI response
- Verify `go-cmp` is removed from `go.mod` / `go.sum` after `go mod tidy` (TASK-04a drops the last direct import)
- Verify `tools/esi-dump.go` works end-to-end: `go run tools/esi-dump.go` writes files to `testdata/`; normal test run writes nothing
- Verify `saveFixture` in `integration_test.go` is not called by any test function (dead helper — acceptable, or remove)
- Documentation: verify `technical-reference.md` is unaffected (no API or schema changes in this feature)
**Status:** ✅ Done

---

### TASK-06 `docs`
**Type:** Docs
**Description:**
- No user-visible changes → no CHANGELOG entry
- Add a "Running Integration Tests" section to `docs/testing-strategy.md` documenting:
  - The three env vars (`ESI_ACCESS_TOKEN`, `ESI_CHARACTER_ID`, `ESI_CORPORATION_ID`) and their skip behaviour
  - The `tools/esi-dump.go` standalone tool and its role: saves re-serialized parser output to `testdata/` for human inspection; does not capture raw ESI HTTP response bytes; not part of the test binary
  - The two run commands (normal test run + dump tool)
  - Where snapshot files live (`internal/esi/testdata/`) and what they contain (parsed struct JSON, not raw ESI JSON)
  - Note that parser unit tests (`blueprints_test.go`, etc.) use inline JSON with `httptest.Server` and do not depend on these files
- Verify `technical-reference.md` is unaffected
**Status:** ⬜ Pending

---

## Run commands

```bash
# Normal run — skips tests whose env vars are absent:
ESI_ACCESS_TOKEN=eyJ... ESI_CHARACTER_ID=12345 ESI_CORPORATION_ID=67890 \
  go test -tags integration ./internal/esi/...

# Dump current parser output to testdata/ (standalone tool, not part of tests):
ESI_ACCESS_TOKEN=eyJ... ESI_CHARACTER_ID=12345 ESI_CORPORATION_ID=67890 \
  go run tools/esi-dump.go

# Dump only a specific subset:
ESI_ACCESS_TOKEN=eyJ... ESI_CHARACTER_ID=12345 go run tools/esi-dump.go -char
ESI_ACCESS_TOKEN=eyJ... ESI_CORPORATION_ID=67890 go run tools/esi-dump.go -corp
go run tools/esi-dump.go -type 34
```

## Env vars

| Variable | Required by | Skip behaviour |
|----------|-------------|----------------|
| `ESI_ACCESS_TOKEN` | all authed tests | `t.Skip` if empty |
| `ESI_CHARACTER_ID` | character tests | `t.Skip` if empty |
| `ESI_CORPORATION_ID` | corporation tests | `t.Skip` if empty |

## Snapshot files (produced by `go run tools/esi-dump.go`)

Tests never write any files. Snapshots are produced exclusively by the standalone dump tool.

| File | Produced by |
|------|-------------|
| `internal/esi/testdata/character_blueprints.json` | `tools/esi-dump.go` (`-char`) |
| `internal/esi/testdata/character_jobs.json` | `tools/esi-dump.go` (`-char`) |
| `internal/esi/testdata/corporation_blueprints.json` | `tools/esi-dump.go` (`-corp`) |
| `internal/esi/testdata/corporation_jobs.json` | `tools/esi-dump.go` (`-corp`) |
| `internal/esi/testdata/universe_type_34.json` | `tools/esi-dump.go` (`-type 34`) |
