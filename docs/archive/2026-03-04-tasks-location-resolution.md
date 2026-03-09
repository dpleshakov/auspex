# 2026-03-04-tasks-location-resolution.md

**Status:** Archived

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

### TASK-03 `location-resolution-corp-offices`
**Type:** Regular
**Description:** Fix silent resolution failure for corporation office/hangar location IDs.

**Root cause:** Corporation blueprints stored in NPC station offices have a `location_id` that refers to the **office/hangar item** within the station (an EVE "item ID" in the ~100M–999M range), not the station itself. These IDs are below the `npcStationThreshold` (1T), so the code treats them as NPC stations and sends them to `POST /universe/names/`. That endpoint only recognises proper station IDs (60,000,000–64,000,000) and silently omits unrecognised IDs from the response — no error, no log, no entry in `eve_locations`. The blueprint stays in "Resolving…" forever.

**Second issue:** when `PostUniverseNames` returns fewer entries than requested, there is no log warning — the discrepancy is invisible.

Steps:
1. Narrow the NPC station ID range check: treat an ID as a resolvable NPC station only if it falls in `60_000_000 ≤ id < 64_000_000`. IDs in the range `[64_000_000, 1_000_000_000_000)` are corporation office/hangar item IDs that cannot be resolved via `/universe/names/`.
2. For unresolvable office/hangar IDs (range `[64_000_000, 1T)`): skip resolution and store the entry in `eve_locations` with a sentinel name (e.g. `"Corporation Hangar"`) so the UI shows something meaningful instead of "Resolving…" forever.
3. After calling `PostUniverseNames`, add a warning log if `len(entries) < len(npcToResolve)`, listing the IDs that were not returned by ESI. This makes future silent omissions visible in the log.
4. Add/update tests to cover:
   - Corporation office IDs are not sent to `PostUniverseNames`
   - Corporation office IDs get a sentinel name stored in `eve_locations`
   - Warning is logged when ESI returns fewer entries than requested

**Definition of done:** working code + tests + committed

**Notes:**
- The exact NPC station ID range in EVE is 60,000,000–64,000,000. IDs outside this range but below 1T are not station IDs recognised by `/universe/names/`.
- Sentinel name for office IDs ("Corporation Hangar") is intentionally generic — resolving the actual station from an office item ID requires additional ESI calls (`POST /corporations/{id}/assets/locations/`) which is out of scope for this fix.
- Do not change 403-handling for player structures — this task only affects the ID classification for sub-1T IDs.

**Status:** ✅ Done

---

### TASK-04 `review`
**Type:** Review
**Covers:** TASK-01, TASK-02
**Description:**
- Code: security, error handling, readability, obvious performance issues
- Security: input validation, no tokens in logs, errors do not expose internal details, dependency vulnerability check
- Documentation: verify `technical-reference.md` matches reality — update if not; verify `architecture.md` — update if needed
**Status:** ✅ Done

### TASK-05 `docs`
**Type:** Docs
**Description:**
- Update user-facing documentation (README, help, guides) if behaviour visible to the user has changed
- Verify `technical-reference.md` is up to date — all API and schema changes introduced by this feature must be reflected
- Update `CHANGELOG.md` — only user-visible changes, following the format in `process-changelog.md`
**Status:** ✅ Done
