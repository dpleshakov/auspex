# Auspex — Testing Strategy

> Date: March 2026
> Status: Agreed, pending implementation

---

## Context and Motivation

Two distinct problems drove this discussion:

**Problem 1 — No protection against ESI API changes.**
If EVE ESI silently changes a field name, drops a field, or changes a type, the application will
break in unexpected places at runtime. There are no tests that encode our expectations about the
ESI response format, so breakage is only discovered when a user sees something wrong in the UI.

**Problem 2 — Bugs can only be verified by a human in the UI.**
When a bug is reported, a fix is implemented, but there is no way to confirm the fix without
running the full application and checking the UI manually. Complex scenarios cannot be
reproduced programmatically, which means no regression protection either.

---

## What We Decided Not To Do

### go-vcr (HTTP record/replay)

`go-vcr` intercepts real HTTP calls on first run, saves them to a "cassette" file, and replays
them on subsequent runs. It is popular in projects like Terraform providers and AWS SDK clients.

**Why we rejected it for Auspex:**

- Our ESI client makes simple, independent GET requests. Cassettes for these are trivial to
  write by hand and offer no automation benefit.
- For **parser tests**, the primary value is *transparency* — a hand-written JSON fixture is
  a readable, explicit assertion about what ESI returns. A vcr cassette obscures this.
- For **regression tests**, fixtures come from the user (copied from DevTools or logs). They are
  not automatically recorded — they are specific data from a specific broken scenario.
- go-vcr would add a dependency and complicate test comprehension without meaningful payoff.

**One exception to revisit:** if ESI introduces pagination (`X-Pages` header) and we need to
test multi-page request sequences, manual fixtures become tedious and go-vcr would help.

---

## Test Taxonomy

### 1. ESI Parser Tests (fixture-based)

**Location:** `internal/esi/`
**Test files:** alongside the code they test, e.g. `blueprints_test.go`, `jobs_test.go`, `universe_test.go`
**Fixtures:** `internal/esi/testdata/` — hand-written JSON files

**Idea:**
Capture a snapshot of the real ESI response format as JSON fixture files and write tests that
parse them. These tests serve two purposes: verify that our parser handles the known response
shape correctly, and protect against accidental regressions in our own parsing code.

**Important limitation:**
These tests do NOT detect changes in the real ESI API. They only verify that our parser is
stable against a fixed snapshot. If ESI changes its format, the fixture stays unchanged and
the test keeps passing — until the fixture is updated manually. Detection of real ESI changes
requires integration tests with a live token (see item 8).

**Goal:**
- Protect our parsing code from regressions during refactoring
- Document exactly what response shape we currently depend on
- Provide a fast, network-free foundation for the test suite

**Infrastructure already in place:**
- `httpClient.baseURL` is overridable (set in `NewClient`, replaceable in tests)
- `NewClient` accepts `*http.Client`, so `httptest.NewServer` can substitute ESI

**What to cover:**
- All response structs: blueprints, jobs, universe type/group/category
- Field presence and types (especially nullable fields)
- Edge cases: empty array response, `null` fields, zero values
- Business logic tied to parsing: BPC filter (`quantity == -1`), job status filter (`active`/`ready`)
- `X-Pages` header parsing for paginated endpoints

**Fixtures to create:**
- `character_blueprints.json` — mix of BPOs and BPCs
- `corporation_blueprints.json`
- `character_jobs.json` — mix of `active`, `ready`, `delivered` statuses
- `corporation_jobs.json`
- `universe_type.json`, `universe_group.json`, `universe_category.json`

**Arguments for:**
- Fixtures double as living documentation of the ESI schema we currently depend on
- Zero external dependencies — standard `httptest` package
- Fast: no network, no disk I/O beyond reading fixture files

---

### 2. API Response Contract Tests

**Location:** `internal/api/`
**Test files:** `handlers_test.go` or per-handler files

**Idea:**
The frontend depends on the shape of JSON responses from our own API handlers. If a handler
changes and renames a field, the frontend breaks silently. These tests spin up the Chi router
with a test SQLite database, make real HTTP requests, and assert on the JSON response structure.

**Goal:**
- Protect the backend↔frontend contract
- Catch handler regressions (renamed fields, missing fields, wrong types)
- Make handler behavior explicit and reviewable

**What to cover:**
- All API endpoints: `/api/blueprints`, `/api/jobs`, `/api/characters`, `/api/sync/status`, etc.
- Response field names and types
- HTTP status codes for normal and error cases
- Empty state responses (no characters added yet, no data synced)

**Arguments for:**
- The frontend is the only consumer of our API — but it has no tests of its own
- Field renames are a common source of silent breakage during refactoring
- Chi router + in-memory SQLite makes this straightforward to set up

---

### 3. Sync Integration Tests

**Location:** `internal/sync/`
**Fixtures:** `internal/sync/testdata/`

**Idea:**
Test the full sync pipeline: ESI fixture → sync worker → SQLite → verify database state.
Uses `httptest.NewServer` to serve ESI fixtures and an in-memory SQLite instance
(`modernc.org/sqlite` supports `:memory:`).

**Goal:**
- Verify that sync correctly transforms ESI responses into database records
- Cover business logic in sync: deduplication, upsert behavior, cache_until handling,
  type resolution triggering
- Provide a harness for regression tests (see below)

**What to cover:**
- Character blueprint sync: records created/updated correctly
- Corporation blueprint sync
- Job sync: only `active` and `ready` jobs stored
- `sync_state.cache_until` updated from ESI `Expires` header
- Force-refresh signal bypasses cache_until check
- Token refresh triggered when token is expired (via `auth.TokenRefresher` mock)
- New `type_id`s trigger lazy universe type resolution

**Arguments for:**
- ESI parser tests verify parsing; sync integration tests verify what we *do* with the data
- In-memory SQLite is fast and requires no setup
- These tests are the natural home for regression scenarios

---

### 4. Regression Scenario Tests

**Location:** `internal/sync/testdata/` (scenarios as subdirectories) or a dedicated
`internal/testscenarios/` package if they span multiple layers

**Idea:**
When a bug is found in the UI, the user provides the raw ESI response (from DevTools Network
tab or application logs). That JSON becomes a fixture. A test is written that reproduces the
exact scenario, initially fails (confirming the bug is reproduced), then passes after the fix.
The test stays in the repository permanently as a regression guard.

**Workflow:**
1. Bug observed in UI
2. User copies raw ESI JSON from DevTools → shares in conversation
3. Fixture saved to `testdata/<scenario_name>/`
4. Test written: `TestRegression_<ScenarioName>`
5. Test fails → confirms reproduction → fix applied → test passes
6. Test remains as permanent regression guard

**Naming convention:**
```
testdata/
  corp_jobs_missing_after_sync/
    esi_jobs.json
    esi_blueprints.json
  blueprint_quantity_overflow/
    esi_blueprints.json
```

**Goal:**
- Allow bugs to be reproduced and verified without the UI
- Eliminate the manual verification round-trip for every fix
- Build a growing suite of real-world scenarios that protect against regression
- Allow async collaboration: user provides data, fix can be verified independently

**Arguments for:**
- Directly solves Problem 2
- Each test is a documented history of a real bug — valuable institutional knowledge
- Real production data catches edge cases that synthetic fixtures miss
- Cost per test is low once the sync integration harness exists

---

### 5. Smoke Test

**Location:** `cmd/auspex/` or a dedicated `smoke_test.go`

**Idea:**
Start the full application binary with an in-memory SQLite database and verify that it
initializes correctly and the HTTP server responds to a basic health check. This is a
startup/wiring test, not a functional test.

**Goal:**
- Catch nil pointer panics and initialization order bugs in `main.go`
- Verify that all dependencies wire together without errors
- Provide a minimal confidence check after any structural refactoring

**What to cover:**
- Application starts without error
- `/healthz` returns 200
- Basic API route is reachable

**Arguments for:**
- Extremely cheap to write and maintain
- Catches an entire class of bugs (wiring errors) that unit tests miss entirely
- Fast: in-memory SQLite, no ESI calls

---

### 6. Database Migration Tests

**Location:** `internal/db/`

**Idea:**
Apply all migrations in order against a fresh in-memory SQLite database and verify the result.
Also verify idempotency: running migrations twice does not produce errors or duplicate state.

**Goal:**
- Ensure migrations are always in a valid, runnable state
- Catch SQL syntax errors and schema conflicts before they reach users
- Verify that sqlc-generated queries compile against the actual schema

**What to cover:**
- All migrations apply from scratch without error
- Repeated application is idempotent
- Final schema matches the expected table/column structure

**Arguments for:**
- Migrations are easy to break (typos, wrong column names, missing semicolons)
- Partially done already; formalizing it costs little
- Gives confidence when adding new migrations

---

### 7. Fuzz Tests for ESI Parsing

**Location:** `internal/esi/`

**Idea:**
Use Go's built-in fuzzing (`testing/fuzz`, introduced in Go 1.18) to feed unexpected inputs
to ESI response parsers. The fuzzer explores inputs that hand-written tests would not think to
cover: negative integers where positive are expected, empty strings, deeply nested nulls.

**Goal:**
- Discover panics and unexpected errors in JSON parsing code
- Harden the ESI client against malformed or adversarial responses
- Catch integer overflow, nil dereference, and type assertion failures

**What to cover:**
- `json.Unmarshal` of blueprint response
- `json.Unmarshal` of job response
- Header parsing (e.g., malformed `Expires` value)

**Arguments for:**
- Go's fuzzer is built-in, no extra dependencies
- ESI is an external API — we do not control its responses
- Fuzz tests run as normal unit tests with a fixed seed; extended fuzzing is opt-in

**Priority note:** Lower priority than 1–6. Worth adding after the core test suite is stable.

---

### 8. ESI Live Integration Tests

**Location:** `internal/esi/`
**Build tag:** `//go:build integration`

**Idea:**
Tests that make real HTTP requests to the live ESI API using a valid access token provided
via environment variable. Unlike fixture-based parser tests, these tests talk to the actual
ESI and will fail if ESI changes its response format or drops a field our parser depends on.
This is the only test category that directly detects real ESI API changes.

**What these tests verify:**
- The ESI call succeeds (no HTTP or parsing error)
- Structurally valid parser output: non-zero IDs on the first element where real data is
  guaranteed (blueprints always has items; universe type 34 always has a name and category)
- Empty slice responses are accepted — no element-level assertions are made on potentially
  empty endpoints (character jobs, corporation blueprints, corporation jobs)

These tests do **not** compare against saved fixtures or write any files. They are purely
assertive: call ESI, check the result looks valid, done.

**Env vars and skip behaviour:**

| Variable | Required by | Skip behaviour |
|----------|-------------|----------------|
| `ESI_ACCESS_TOKEN` | all authed tests | `t.Skip` if empty |
| `ESI_CHARACTER_ID` | character tests | `t.Skip` if empty |
| `ESI_CORPORATION_ID` | corporation tests | `t.Skip` if empty |

`TestIntegration_GetUniverseType` requires no env vars — it uses hardcoded type ID 34
(Tritanium), which is a stable EVE item that will always exist.

**What to cover:**
- `GET /characters/{id}/blueprints` — parses without error, required fields non-zero
- `GET /characters/{id}/industry/jobs` — parses without error
- `GET /corporations/{id}/blueprints` — parses without error
- `GET /corporations/{id}/industry/jobs` — parses without error
- `GET /universe/types/{type_id}` — TypeName non-empty, CategoryID non-zero

**Fixture snapshots (separate tool):**
JSON snapshots of parsed parser output can be saved using the standalone dump tool
`tools/esi-dump.go` (build tag `//go:build ignore`). These files are for human inspection
only — no test reads or writes them. See the "Running Integration Tests" section for
run commands and details.

**Goal:**
- Directly solves Problem 1 — the only reliable way to know ESI has changed is to ask ESI
- Skips gracefully in CI where no token is available
- Complements fixture-based tests: those protect our code day-to-day, these protect against
  ESI drift when run manually

**Implementation order note:**
Although this test has low urgency for daily CI use, it should be written **before** parser
tests — because it provides the structural confidence that the parser handles the live ESI
format correctly, and the dump tool can produce fixture files for parser tests to use.

---

## Running Integration Tests

Integration tests live in `internal/esi/integration_test.go` behind the `//go:build integration`
build tag. They are not included in a normal `go test ./...` run.

### Run commands

```bash
# Normal run — skips any test whose required env var is absent:
ESI_ACCESS_TOKEN=eyJ... ESI_CHARACTER_ID=12345 ESI_CORPORATION_ID=67890 \
  go test -tags integration ./internal/esi/...

# Universe type test requires no env vars:
go test -tags integration -run TestIntegration_GetUniverseType ./internal/esi/...
```

Tests call `t.Skip` (not `t.Fatal`) when a required env var is missing, so running
without any env vars set will report all authed tests as skipped — not failed.

### Saving parser output snapshots

The standalone tool `tools/esi-dump.go` (build tag `//go:build ignore`, not part of the
test binary) fetches live ESI data and writes the re-serialized parsed structs to
`internal/esi/testdata/` as JSON files. These files are for human inspection; no test
reads or writes them.

```bash
# Dump all endpoints (requires all three env vars):
ESI_ACCESS_TOKEN=eyJ... ESI_CHARACTER_ID=12345 ESI_CORPORATION_ID=67890 \
  go run tools/esi-dump.go

# Dump only character endpoints:
ESI_ACCESS_TOKEN=eyJ... ESI_CHARACTER_ID=12345 go run tools/esi-dump.go -char

# Dump only corporation endpoints:
ESI_ACCESS_TOKEN=eyJ... ESI_CORPORATION_ID=67890 go run tools/esi-dump.go -corp

# Dump universe type (no token required):
go run tools/esi-dump.go -type 34

# Write to a custom directory:
go run tools/esi-dump.go -out /tmp/esi-snapshots
```

### Snapshot files

| File | Produced by |
|------|-------------|
| `internal/esi/testdata/character_blueprints.json` | `go run tools/esi-dump.go -char` |
| `internal/esi/testdata/character_jobs.json` | `go run tools/esi-dump.go -char` |
| `internal/esi/testdata/corporation_blueprints.json` | `go run tools/esi-dump.go -corp` |
| `internal/esi/testdata/corporation_jobs.json` | `go run tools/esi-dump.go -corp` |
| `internal/esi/testdata/universe_type_34.json` | `go run tools/esi-dump.go -type 34` |

**Important:** these files contain the **parsed Go struct** re-serialized to JSON — not the
raw HTTP response bytes from ESI. The raw bytes are consumed inside each `Get*` method and
are not available at the call site.

### Relationship to parser unit tests

Parser unit tests (`blueprints_test.go`, `jobs_test.go`, `universe_test.go`) use inline JSON
strings served by `httptest.NewServer`. They do not read files from `internal/esi/testdata/`
and have no dependency on the integration tests or the dump tool.

---

## Priority Tables

Two separate dimensions: **implementation order** (what to write first) and **daily CI value**
(how much the test contributes to ongoing confidence).

### Implementation Order

Write in this order because each layer depends on the previous one:

| Step | Type | Reason |
|------|------|--------|
| 1 | ESI live integration tests | Generates fixtures for all other ESI tests |
| 2 | ESI parser tests (fixture-based) | Depends on fixtures from step 1 |
| 3 | Migration tests | Foundation for any test using SQLite |
| 4 | Sync integration tests | Depends on migrations; provides harness for regressions |
| 5 | Regression scenario tests | Depends on sync integration harness |
| 6 | API response contract tests | Depends on migrations and store |
| 7 | Smoke test | Depends on everything being wired |
| 8 | Fuzz tests | Standalone; add after core suite is stable |

### Daily CI Value

How much each test contributes to ongoing automated confidence (no token, no network):

| # | Type | Effort | Solves | Notes |
|---|------|--------|--------|-------|
| 1 | ESI parser tests (fixture-based) | Low | Parser regressions | Fast, no network |
| 2 | API response contract tests | Low | Frontend stability | Field rename protection |
| 3 | Sync integration tests | Medium | Problem 2 (harness) | Enables regression tests |
| 4 | Regression scenario tests | Low* | Problem 2 (regression) | Real bug data |
| 5 | Smoke test | Very low | Wiring bugs | Cheapest test in the suite |
| 6 | Migration tests | Very low | Schema integrity | Partially exists already |
| 7 | Fuzz tests | Low | Parser robustness | After core suite is stable |
| 8 | ESI live integration tests | Low | Problem 1 | Manual only; needs token |

*Low effort per test once the sync integration harness (step 4) exists.

---

## How to Get Fixture Data

For ESI parser tests: fixtures are generated automatically by running ESI live integration
tests (item 8) with a valid token. No manual fixture creation needed.

For regression tests: when a bug is observed in the UI, open DevTools → Network tab,
find the relevant ESI response or the `/api/...` response, copy the raw JSON, and share it.
The fixture will be committed alongside the test.

For ESI live integration tests: take a fresh access token from the running application
(visible in DevTools or extractable from the SQLite database) and pass it via
`ESI_ACCESS_TOKEN` environment variable.
