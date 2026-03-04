# 2026-03-04-tasks-location-resolution.md

**Status:** Active

### Contracts

**DB schema additions:**
```sql
CREATE TABLE eve_locations (
    id   INTEGER PRIMARY KEY,  -- EVE location_id (station or structure)
    name TEXT NOT NULL
);
```

**`GET /api/blueprints` response change:**
- Field `location_id` (integer) supplemented with `location_name` (string, nullable — `null` while not yet resolved)

---

### TASK-01 `location-resolution`
**Type:** Regular
**Description:** Show solar system name instead of raw location ID in the blueprints table location column. Requires resolving `location_id` via ESI `POST /universe/names/` (bulk) or `GET /universe/systems/{system_id}` and caching results in a new `eve_locations` table (similar to the existing lazy-resolution pattern for `eve_types`). Steps: add `eve_locations` migration, add `store` queries, add resolver in `sync` worker, expose `location_name` in `GET /api/blueprints` response, render in `BlueprintTable` component.
**Definition of done:** working code + tests + committed
**Status:** ⬜ Pending

### TASK-02 `review`
**Type:** Review
**Covers:** TASK-01
**Status:** ⬜ Pending

### TASK-03 `docs`
**Type:** Docs
**Status:** ⬜ Pending
