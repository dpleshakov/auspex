# 2026-03-10-tasks-location.md

**Status:** Archived

### Contracts

**New ESI endpoints:**
- `GET /corporations/{id}/assets/?page=N` — requires scope `esi-assets.read_corporation_assets.v1`
- `GET /universe/stations/{id}/` — public, no token required

**New DB table:**
```sql
CREATE TABLE corp_assets (
    item_id       INTEGER PRIMARY KEY,  -- Corporation Office Item ID (= location_id in corp blueprints)
    owner_id      INTEGER NOT NULL,     -- corporation_id
    location_id   INTEGER NOT NULL,     -- real station or structure ID
    location_type TEXT NOT NULL
);
```

**Modified DB table:**
```sql
-- blueprints: new column
location_flag TEXT NOT NULL DEFAULT ''
```

**New sync endpoint key:** `corp_assets` (per corporation, synced before `blueprints`)

---

### TASK-01 `failing-test`
**Type:** Regular
**Description:** Update `TestSyncIntegration_CorporationBlueprints_RowsMatchFixture` to document
the correct expected behavior — resolving a corp CorpSAG blueprint location to a real station name
via corp assets rather than offices. Steps:

1. Update `corporation_blueprints.json` to use `item_id=1052548174037` (keep `type_id=5000`,
   `location_flag="CorpSAG3"`, `location_id=1052718829566`).
2. Remove the `/latest/corporations/99000001/offices/` route from the test server route map.
3. Add `/latest/corporations/99000001/assets` route → new fixture `corporation_assets_officefolders.json`:
   `[{"item_id": 1052718829566, "location_flag": "OfficeFolder", "location_id": 60015146, "location_type": "station"}]`
4. Add `/latest/universe/stations/60015146` route → new fixture `universe_station_60015146.json`:
   `{"station_id": 60015146, "name": "Ibura IX - Moon 11 - Spacelane Patrol Testing Facilities"}`
5. Change the assertion from "not sentinel" to an exact name check:
   `locName == "Ibura IX - Moon 11 - Spacelane Patrol Testing Facilities"`.

After this task the test **intentionally fails** (red TDD state). `make build` will be broken until
TASK-05 completes. This is expected — the test documents the desired behavior before the fix exists.
**Definition of done:** test written, fixtures added, committed; `go test` fails on this test case
**Status:** ✅ Done

---

### TASK-02 `esi-assets-station`
**Type:** Regular
**Description:** Add two new ESI methods and register them in the `Client` interface.

`GetCorporationAssets(ctx, corpID int64, token string, page int) ([]CorpAsset, int, error)`:
- `GET /corporations/{id}/assets/?page=N`
- New type `CorpAsset { ItemID, LocationID int64; LocationFlag, LocationType string }`
- Read `X-Pages` response header and return totalPages so the caller can paginate
- Caller loops all pages and filters `LocationFlag == "OfficeFolder"`; returning raw records keeps the ESI layer simple

`GetStation(ctx, stationID int64) (string, error)`:
- `GET /universe/stations/{id}/` — no auth token required
- Parse `name` field from the response JSON
- Add to `internal/esi/universe.go` alongside existing universe helpers

Update `Client` interface in `client.go` to include both new methods. Keep `GetCorporationOffices`
in the interface for now (removed in TASK-05). Add unit tests for both methods.

Update `technical-reference.md` to reflect new ESI endpoints.
**Definition of done:** working code + tests + committed
**Status:** ✅ Done

---

### TASK-03 `db-corp-assets`
**Type:** Regular
**Description:** Add schema and store support for corp assets and blueprint location flag.

1. Migration `internal/db/migrations/004_corp_assets.sql`:
   - `CREATE TABLE corp_assets` (see Contracts above)
   - `ALTER TABLE blueprints ADD COLUMN location_flag TEXT NOT NULL DEFAULT ''`

2. sqlc queries (`internal/db/queries/`):
   - `UpsertCorpAsset` — `INSERT OR REPLACE INTO corp_assets`
   - `GetCorpAsset(item_id)` — returns `(location_id, location_type)` for the given item_id
   - `DeleteCorpAssetsByOwner(owner_id)` — prune stale assets before re-inserting
   - Update `UpsertBlueprint` to include `location_flag`
   - Add `ListBlueprintLocationsByOwner(owner_type, owner_id)` returning `(location_id, location_flag)` pairs (replaces `ListBlueprintLocationIDsByOwner` in the resolver)

3. Update `internal/store/` generated files (manually if sqlc unavailable, following existing
   `*.sql.go` pattern). Add `CorpAsset` model to `models.go`.

4. Update `syncBlueprints` call site in `worker.go` to pass `bp.LocationFlag` to `UpsertBlueprint`.

Update `technical-reference.md` to reflect schema changes.
**Definition of done:** working code + tests + committed
**Status:** ✅ Done

---

### TASK-04 `sync-corp-assets`
**Type:** Regular
**Description:** Sync corp assets as a separate endpoint before blueprint sync.

1. Add constant `endpointCorpAssets = "corp_assets"`.
2. In `runCycle`, for each corporation iterate `[endpointCorpAssets, endpointBlueprints, endpointJobs]`
   so assets are always fresh before location resolution runs.
3. New method `syncCorpAssets(ctx, corpID int64) (time.Time, error)`:
   - Loop all pages via `GetCorporationAssets` (use `totalPages` returned by the ESI method)
   - Collect all OfficeFolder entries
   - Call `DeleteCorpAssetsByOwner` then upsert each entry
   - Return the ESI cache expiry from page 1
4. Wire into `syncSubject` switch: new `endpointCorpAssets` case calls `syncCorpAssets`.

Add integration test `TestSyncIntegration_CorpAssetsSync_StoresOfficeFolders` that verifies
corp_assets rows are inserted with correct `item_id` and `location_id` after sync.
**Definition of done:** working code + tests + committed
**Status:** ✅ Done

---

### TASK-05 `location-resolution`
**Type:** Regular
**Description:** Replace the `/offices/`-based location resolution with the new corp-assets approach,
delete all dead code, and make the TASK-01 test go green.

1. In `worker.go`, rewrite `resolveLocationIDs`:
   - Use `ListBlueprintLocationsByOwner` (added in TASK-03) to get `(location_id, location_flag)` pairs
   - For entries where `location_flag` is in `corpHangarFlags`:
     - Look up `location_id` in `corp_assets` via `GetCorpAsset`
     - If found → resolve the real `location_id` from `corp_assets`:
       - NPC station (60 000 000–64 000 000): call `GetStation` → cache name in `eve_locations`
       - Player structure (≥ 1 000 000 000 000): call `GetUniverseStructure` (existing logic, unchanged)
     - If not found in `corp_assets` → store `corpHangarSentinel` (will resolve next cycle)
   - For entries where `location_flag == "Hangar"` (character blueprints with direct station IDs):
     - NPC station range: call `GetStation`
     - Structure range: call `GetUniverseStructure`
   - Skip IDs already cached in `eve_locations`

2. Remove `resolveCorpOfficeLocations` function and its call site in `syncBlueprints`.

3. Delete dead code:
   - `internal/esi/offices.go`
   - `internal/esi/offices_test.go`
   - Remove `GetCorporationOffices` from `Client` interface in `client.go`

4. Update `location_resolution_test.go`: replace unit tests for the old corp-hangar-sentinel path
   with tests for the new corp-assets lookup path.

After this task `go test ./...` passes, including the TASK-01 integration test.
**Definition of done:** working code + tests + committed; `go test ./...` green
**Status:** ✅ Done

---

### TASK-06 `review`
**Type:** Review
**Covers:** TASK-01, TASK-02, TASK-03, TASK-04, TASK-05
**Description:**
- Code: error handling in paginated assets fetch; correct fallback when `corp_assets` not yet synced; no duplicate location cache writes
- Security: no tokens logged, no internal error details exposed in API responses, dependency vulnerability check (`go mod tidy`, `golangci-lint`, `npm audit`)
- Documentation: verify `technical-reference.md` matches what was actually built — update if not; verify `architecture.md` — update if module responsibilities changed
**Status:** ✅ Done

---

### TASK-07 `docs`
**Type:** Docs
**Description:**
- Update user-facing documentation (README, `auspex.example.yaml`) with the new required ESI scope `esi-assets.read_corporation_assets.v1`
- Verify `technical-reference.md` is up to date — new ESI endpoints, schema changes (`corp_assets` table, `blueprints.location_flag`), new sync endpoint `corp_assets`
- Update `CHANGELOG.md` — one entry: corp blueprint location names now resolve correctly to real station/structure names; following the format in `process-changelog.md`
**Status:** ✅ Done
