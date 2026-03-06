# 2026-03-04-tasks-location-resolution.md

**Status:** Active

### Contracts

**DB schema additions:**
```sql
CREATE TABLE eve_locations (
    id          INTEGER PRIMARY KEY,  -- EVE location_id (station or structure)
    name        TEXT NOT NULL,
    resolved_at DATETIME NOT NULL     -- last successful resolution timestamp
);
```

**`GET /api/blueprints` response change:**
- Field `location_id` (integer) supplemented with `location_name` (string, nullable — `null` while not yet resolved)

**Frontend rendering:**
- `location_name != null` → render the name
- `location_name == null` → render `"Resolving…"` placeholder

---

### TASK-01 `location-resolution-npc`
**Type:** Regular
**Description:** Resolve `location_id` for NPC stations and return a human-readable name in the blueprints API response. NPC station IDs are in the range `< 1_000_000_000_000` and can be resolved via `POST /universe/names/` (bulk, no auth required). The response includes the full station name with system context (e.g. `"Jita IV - Moon 4 - Caldari Navy Assembly Plant"`), so no separate system lookup is needed.

Steps:
1. Add `eve_locations` migration (schema above)
2. Add `store` queries: `GetLocation`, `InsertLocation` (sqlc)
3. Add `resolveLocationIDs` in `sync` worker — after each successful blueprint sync, collect all `location_id`s from new/updated blueprints, skip IDs already present in `eve_locations`, bulk-resolve via `POST /universe/names/`, insert results with `resolved_at = now`
4. Add `PostUniverseNames` method to `esi.Client` interface (reuse if already present from type resolution)
5. Expose `location_name` in `GET /api/blueprints` response via JOIN in `ListBlueprints` query
6. Render in `BlueprintTable`: show name when resolved, `"Resolving…"` when `null`

**Definition of done:** working code + tests + committed

**Notes:**
- Player-owned structures (`location_id >= 1_000_000_000_000`) are intentionally out of scope — handled in TASK-02
- `resolved_at` enables future cache invalidation; no invalidation logic required in this task
- On first run, all `location_name` values will be `null` until the first sync cycle completes — this is expected and handled by the `"Resolving…"` placeholder

**Status:** ✅ Done

---

### TASK-02 `location-resolution-structures`
**Type:** Regular
**Description:** Extend location resolution to cover player-owned structures. Structure IDs are in the range `>= 1_000_000_000_000` and require an authenticated request: `GET /universe/structures/{structure_id}/` with a valid character token. The response includes both the structure name and `solar_system_id`; the displayed value should be `"<system name> — <structure name>"` (e.g. `"Perimeter — Tranquility Trading Tower"`), because structure names alone lack geographic context.

Steps:
1. Extend `resolveLocationIDs` in `sync` worker to split IDs by range: IDs below threshold go to `POST /universe/names/` (existing), IDs above threshold go to `GET /universe/structures/{id}/`
2. For each structure ID: fetch structure via authenticated ESI call (use any available character token), fetch system name via `GET /universe/systems/{system_id}/`, compose display name as `"<system> — <structure>"`
3. Add `GetUniverseStructure` and `GetUniverseSystem` methods to `esi.Client` interface
4. Structures that return 403 (no access) should be stored with name `null` and retried on next sync — do not cache failed lookups

**Definition of done:** working code + tests + committed

**Notes:**
- 403 is a normal response: a character may not have docking access to a structure they previously visited. Store `null`, do not cache the failure permanently.
- System name resolution adds one extra ESI call per unique system; cache system names in `eve_locations` using the system ID as key to avoid redundant lookups

**Status:** ✅ Done

---

### TASK-03 `review`
**Type:** Review
**Covers:** TASK-01, TASK-02
**Description:**
- Code: security, error handling, readability, obvious performance issues
- Security: input validation, no tokens in logs, errors do not expose internal details, dependency vulnerability check
- Documentation: verify `technical-reference.md` matches reality — update if not; verify `architecture.md` — update if needed
**Status:** ⬜ Pending

### TASK-04 `docs`
**Type:** Docs
**Description:**
- Update user-facing documentation (README, help, guides) if behaviour visible to the user has changed
- Verify `technical-reference.md` is up to date — all API and schema changes introduced by this feature must be reflected
- Update `CHANGELOG.md` — only user-visible changes, following the format in `process-changelog.md`
**Status:** ⬜ Pending
