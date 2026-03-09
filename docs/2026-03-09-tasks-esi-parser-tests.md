## 2026-03-09-tasks-esi-parser-tests.md

**Status:** Active

### Contracts

No API or DB schema changes. All changes are internal to `internal/esi/`.

**Pagination contract (new internal behaviour):**
- `GetCharacterBlueprints`, `GetCorporationBlueprints`, `GetCharacterJobs`, `GetCorporationJobs`
  now fetch all pages when ESI returns `X-Pages > 1`.
- The functions' signatures and return types are unchanged.
- Page results are concatenated before filtering (BPC filter, status filter) is applied.
- The `cacheUntil` returned is taken from the *first* page response (consistent with current behaviour).

---

### TASK-01 `blueprints-pagination`
**Type:** Regular
**Description:**
Implement multi-page fetching for `GetCharacterBlueprints` and `GetCorporationBlueprints` in
`internal/esi/blueprints.go`.

ESI returns an `X-Pages` response header on paginated endpoints. When `X-Pages > 1`, the client
must fetch pages 2, 3, … N by appending `?page=N` to the URL, then concatenate all results before
applying the BPC filter.

Implementation notes:
- Read `X-Pages` from the first response header. If absent or `"1"`, do nothing extra.
- Use a helper (e.g. `parseXPages(header string) int`) — test this helper in isolation.
- Pages 2..N are fetched sequentially (not concurrently) to keep the implementation simple.
- `cacheUntil` comes from the first page response only.
- After all raw items are collected, pass them through the existing `parseBlueprints` logic.

Tests to write (in `blueprints_test.go`):
- `TestGetCharacterBlueprints_MultiPage`: server returns `X-Pages: 2`; page 1 and page 2 each
  serve one BPO; assert 2 BPOs returned and that page 2 was requested.
- `TestGetCharacterBlueprints_XPagesAbsent`: no `X-Pages` header; assert only 1 request made.
- `TestGetCharacterBlueprints_XPagesOne`: `X-Pages: 1`; assert only 1 request made.
- `TestGetCorporationBlueprints_MultiPage`: same multi-page scenario for the corp endpoint.
- `TestParseXPages_*`: table-driven unit tests for the `parseXPages` helper
  (valid int, absent header, malformed string, "1").

**Definition of done:** working code + tests + `make test` passes + committed
**Status:** ⬜ Pending

---

### TASK-02 `jobs-pagination`
**Type:** Regular
**Description:**
Implement multi-page fetching for `GetCharacterJobs` and `GetCorporationJobs` in
`internal/esi/jobs.go`, following the exact same pattern established in TASK-01.

Reuse the `parseXPages` helper introduced in TASK-01.

After all raw job items are collected across all pages, apply the existing status and activity
filters in one pass.

Tests to write (in `jobs_test.go`):
- `TestGetCharacterJobs_MultiPage`: `X-Pages: 2`; page 1 has an active job, page 2 has an active
  job; assert both returned.
- `TestGetCharacterJobs_XPagesAbsent`: no `X-Pages` header; assert only 1 request made.
- `TestGetCorporationJobs_MultiPage`: same scenario for corp endpoint.

**Definition of done:** working code + tests + `make test` passes + committed
**Status:** ⬜ Pending

---

### TASK-03 `parser-coverage`
**Type:** Regular
**Description:**
Fill the remaining parser test coverage gaps across all `internal/esi/` test files.
No new implementation code — tests only.

Gaps to address, per the testing strategy checklist:

**blueprints_test.go:**
- `TestGetCharacterBlueprints_AllBPCsReturnsEmpty`: server returns only BPC items (quantity > 0);
  assert returned slice is empty (not an error).
- `TestGetCharacterBlueprints_LocationFlagAbsent`: blueprint JSON omits `location_flag` field;
  assert `LocationFlag == ""` (Go zero value, not an error).
- `TestGetCharacterBlueprints_ZeroMETE`: BPO with `material_efficiency: 0` and
  `time_efficiency: 0`; assert both survive filtering and parse to 0.

**jobs_test.go:**
- `TestGetCharacterJobs_EmptyList`: server returns `[]`; assert empty slice, no error.
  (Mirrors `TestGetCharacterBlueprints_EmptyList` which already exists.)
- `TestGetCorporationJobs_EmptyList`: same for corp endpoint.

**offices_test.go:**
- `TestGetCorporationOffices_EmptyList`: server returns `[]`; assert empty slice, no error.
- Verify (read the code) whether the `/corporations/{id}/offices/` endpoint also supports
  `X-Pages`. If yes, add a follow-up note in the review task; do not implement pagination for
  offices in this task.

**Definition of done:** all new tests pass + `make test` passes + committed
**Status:** ⬜ Pending

---

### TASK-04 `review`
**Type:** Review
**Covers:** TASK-01, TASK-02, TASK-03
**Description:**
- Code: security, error handling, readability, obvious performance issues.
  Pay special attention to: loop termination in pagination (guard against infinite loop if ESI
  returns a nonsensical `X-Pages` value), integer overflow in `parseXPages`, request counting.
- Security: no tokens in logs, errors do not expose internal details, dependency vulnerability
  check (`go mod tidy`, `golangci-lint`).
- Documentation: verify `technical-reference.md` matches reality — update if pagination behaviour
  is documented there; verify `architecture.md` — no module responsibility changes expected.
**Status:** ⬜ Pending

---

### TASK-05 `docs`
**Type:** Docs
**Description:**
- Update user-facing documentation (README) only if the pagination fix is user-visible (e.g.,
  previously characters with >1000 BPOs silently lost data — this is worth noting).
- Verify `technical-reference.md` is up to date — pagination is an internal client detail; if
  the ESI client section exists, add a note that all list endpoints now support multi-page fetching.
- Update `CHANGELOG.md` — only user-visible changes, following `process-changelog.md` format.
  The pagination fix is user-visible: characters/corporations with large blueprint libraries
  (>1000 BPOs) were previously showing incomplete data.
**Status:** ⬜ Pending
