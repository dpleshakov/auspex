# Auspex — Technical Reference

> Date: 04.03.2026
> Status: Current

---

## Database Schema

```sql
-- Location name cache (NPC stations and player structures, populated lazily on first encounter)
CREATE TABLE eve_locations (
    id          INTEGER PRIMARY KEY,  -- EVE location_id (station or structure)
    name        TEXT NOT NULL,
    resolved_at DATETIME NOT NULL     -- last successful resolution timestamp
);

-- EVE universe reference data (populated lazily on first encounter)
CREATE TABLE eve_categories (
    id    INTEGER PRIMARY KEY,  -- EVE category_id
    name  TEXT NOT NULL
);

CREATE TABLE eve_groups (
    id          INTEGER PRIMARY KEY,  -- EVE group_id
    category_id INTEGER NOT NULL REFERENCES eve_categories(id),
    name        TEXT NOT NULL
);

CREATE TABLE eve_types (
    id       INTEGER PRIMARY KEY,  -- EVE type_id
    group_id INTEGER NOT NULL REFERENCES eve_groups(id),
    name     TEXT NOT NULL
);

-- Authorized characters (one OAuth token per character)
CREATE TABLE characters (
    id               INTEGER PRIMARY KEY,  -- EVE character_id
    name             TEXT NOT NULL,
    access_token     TEXT NOT NULL,
    refresh_token    TEXT NOT NULL,
    token_expiry     DATETIME NOT NULL,
    created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    corporation_id   INTEGER NOT NULL DEFAULT 0,
    corporation_name TEXT NOT NULL DEFAULT ''
);

-- Tracked corporations (accessed via delegate character)
CREATE TABLE corporations (
    id           INTEGER PRIMARY KEY,  -- EVE corporation_id
    name         TEXT NOT NULL,
    delegate_id  INTEGER NOT NULL REFERENCES characters(id),
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- BPO library (all characters + corporations combined)
CREATE TABLE blueprints (
    id            INTEGER PRIMARY KEY,  -- EVE item_id
    owner_type    TEXT NOT NULL,        -- 'character' | 'corporation'
    owner_id      INTEGER NOT NULL,
    type_id       INTEGER NOT NULL REFERENCES eve_types(id),
    location_id   INTEGER NOT NULL,
    location_flag TEXT NOT NULL DEFAULT '',  -- ESI location_flag (e.g. 'CorpSAG3', 'Hangar')
    me_level      INTEGER NOT NULL DEFAULT 0,
    te_level      INTEGER NOT NULL DEFAULT 0,
    updated_at    DATETIME NOT NULL
);

-- Active and ready research jobs
CREATE TABLE jobs (
    id           INTEGER PRIMARY KEY,  -- EVE job_id
    blueprint_id INTEGER NOT NULL REFERENCES blueprints(id),
    owner_type   TEXT NOT NULL,        -- 'character' | 'corporation'
    owner_id     INTEGER NOT NULL,
    installer_id INTEGER NOT NULL,     -- character_id who started the job
    activity     TEXT NOT NULL,        -- 'me_research' | 'te_research' | 'copying'
    status       TEXT NOT NULL,        -- 'active' | 'ready'
    start_date   DATETIME NOT NULL,
    end_date     DATETIME NOT NULL,
    updated_at   DATETIME NOT NULL
);

-- Corp assets cache (OfficeFolder entries — maps office item_id to real station/structure)
CREATE TABLE corp_assets (
    item_id       INTEGER PRIMARY KEY,  -- Corporation Office Item ID (= location_id in corp blueprints)
    owner_id      INTEGER NOT NULL,     -- corporation_id
    location_id   INTEGER NOT NULL,     -- real station or structure ID
    location_type TEXT NOT NULL
);

-- ESI cache state per subject per endpoint
CREATE TABLE sync_state (
    owner_type  TEXT NOT NULL,
    owner_id    INTEGER NOT NULL,
    endpoint    TEXT NOT NULL,      -- 'blueprints' | 'jobs'
    last_sync   DATETIME NOT NULL,
    cache_until DATETIME NOT NULL,
    last_error  TEXT,               -- last sync error message; NULL when last sync succeeded
    PRIMARY KEY (owner_type, owner_id, endpoint)
);
```

---

## API Reference

All API endpoints are served under `http://localhost:PORT` (default port: 8080).

- `/api/*` — JSON REST API (all responses include `Content-Type: application/json`)
- `/auth/*` — EVE SSO OAuth2 flow (redirect-based, not JSON)
- `/*` — React SPA (serves `index.html` for any unmatched path)

Errors are returned as:

```json
{ "error": "human-readable message" }
```

---

### Authentication

Auspex uses EVE SSO OAuth2. Authentication is browser-based — there are no API keys or session tokens for the REST API. The backend stores OAuth tokens per character in SQLite and uses them internally for ESI requests.

---

### Characters

#### `GET /api/characters`

Returns all characters that have been added via the OAuth2 flow.

**Response `200 OK`:**

```json
[
  {
    "id": 12345678,
    "name": "My Character",
    "corporation_id": 98765432,
    "corporation_name": "Center for Advanced Studies",
    "is_delegate": true,
    "sync_error": null,
    "created_at": "2026-02-21T10:00:00Z"
  }
]
```

| Field | Type | Description |
|-------|------|-------------|
| `id` | integer | EVE character ID |
| `name` | string | Character name |
| `corporation_id` | integer | EVE corporation ID the character belongs to |
| `corporation_name` | string | EVE corporation name (stored at character save time; used for NPC corporations not in the `corporations` table) |
| `is_delegate` | boolean | Whether this character is the delegate for its corporation |
| `sync_error` | string or `null` | Last sync error for this character's corporation (only when `is_delegate = true` and last sync failed); `null` otherwise |
| `created_at` | ISO 8601 datetime | When the character was added |

Returns an empty array `[]` if no characters have been added.

---

#### `DELETE /api/characters/{id}`

Removes a character and all associated data (blueprints, jobs, sync state).

If the deleted character is the last member of a player corporation, the corporation and all its data (blueprints, jobs, sync state) are deleted first. If other characters share the same corporation and the deleted character is the current delegate, the delegate is reassigned to the first remaining character. NPC corporations (ID range 1000000–2000000) are never in the `corporations` table, so no corporation logic applies.

**Path parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `id` | integer | EVE character ID |

**Responses:**

| Status | Description |
|--------|-------------|
| `204 No Content` | Character and all associated data deleted |
| `400 Bad Request` | `id` is not a valid integer |
| `500 Internal Server Error` | Database error |

> **Note:** Deletion is non-atomic (no transaction). In the unlikely event of a mid-operation crash, orphaned rows may remain. This is a known MVP limitation (TD-14).

---

### Corporations

#### `GET /api/corporations`

Returns all tracked corporations.

**Response `200 OK`:**

```json
[
  {
    "id": 98765432,
    "name": "My Corporation",
    "delegate_id": 12345678,
    "delegate_name": "My Character",
    "created_at": "2026-02-21T10:00:00Z"
  }
]
```

| Field | Type | Description |
|-------|------|-------------|
| `id` | integer | EVE corporation ID |
| `name` | string | Corporation name |
| `delegate_id` | integer | EVE character ID used to fetch corporation ESI data |
| `delegate_name` | string | Name of the delegate character |
| `created_at` | ISO 8601 datetime | When the corporation was added |

---

#### `POST /api/corporations`

Adds a corporation to be tracked. The delegate character must already be added via OAuth.

**Request body:**

```json
{
  "id": 98765432,
  "name": "My Corporation",
  "delegate_id": 12345678
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | integer | yes | EVE corporation ID |
| `name` | string | yes | Corporation name |
| `delegate_id` | integer | yes | EVE character ID that has the required corporation ESI scopes |

**Responses:**

| Status | Description |
|--------|-------------|
| `201 Created` | Corporation added |
| `400 Bad Request` | Missing required fields, or `delegate_id` does not refer to a known character |
| `500 Internal Server Error` | Database error |

---

#### `PATCH /api/corporations/{id}/delegate`

Updates the delegate character for a corporation.

**Path parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `id` | integer | EVE corporation ID |

**Request body:**

```json
{ "character_id": 12345678 }
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `character_id` | integer | yes | EVE character ID to set as delegate; must belong to this corporation |

**Responses:**

| Status | Description |
|--------|-------------|
| `204 No Content` | Delegate updated |
| `400 Bad Request` | `character_id` missing, or character does not belong to this corporation |
| `404 Not Found` | Corporation not found |
| `500 Internal Server Error` | Database error |

---

#### `DELETE /api/corporations/{id}`

Removes a corporation and all associated data (blueprints, jobs, sync state).

**Path parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `id` | integer | EVE corporation ID |

**Responses:**

| Status | Description |
|--------|-------------|
| `204 No Content` | Corporation and all associated data deleted |
| `400 Bad Request` | `id` is not a valid integer |
| `500 Internal Server Error` | Database error |

---

### Blueprints

#### `GET /api/blueprints`

Returns the BPO library. All query parameters are optional and combinable.

**Query parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `status` | string | Filter by derived status: `idle`, `active`, `ready` |
| `owner_type` | string | Filter by owner type: `character` or `corporation` |
| `owner_id` | integer | Filter by owner ID (character or corporation ID) |
| `category_id` | integer | Filter by EVE category ID |

**Response `200 OK`:**

```json
[
  {
    "id": 1000000001,
    "owner_type": "character",
    "owner_id": 12345678,
    "owner_name": "My Character",
    "type_id": 2047,
    "type_name": "Tritanium Blueprint",
    "category_id": 9,
    "category_name": "Blueprint",
    "location_id": 60003760,
    "location_name": "Jita IV - Moon 4 - Caldari Navy Assembly Plant",
    "me_level": 10,
    "te_level": 20,
    "job": null
  },
  {
    "id": 1000000002,
    "owner_type": "character",
    "owner_id": 12345678,
    "owner_name": "My Character",
    "type_id": 34,
    "type_name": "Tritanium",
    "category_id": 9,
    "category_name": "Blueprint",
    "location_id": 60003760,
    "location_name": null,
    "me_level": 8,
    "te_level": 16,
    "job": {
      "id": 500000001,
      "activity": "me_research",
      "status": "active",
      "start_date": "2026-02-20T12:00:00Z",
      "end_date": "2026-02-25T12:00:00Z"
    }
  }
]
```

**Blueprint fields:**

| Field | Type | Description |
|-------|------|-------------|
| `id` | integer | EVE item ID of the blueprint |
| `owner_type` | string | `"character"` or `"corporation"` |
| `owner_id` | integer | EVE character or corporation ID |
| `owner_name` | string | Display name of the owner |
| `type_id` | integer | EVE type ID |
| `type_name` | string | Resolved type name (e.g. `"Rifter Blueprint"`) |
| `category_id` | integer | EVE category ID |
| `category_name` | string | Resolved category name (e.g. `"Blueprint"`) |
| `location_id` | integer | ESI location ID |
| `location_name` | string or `null` | Human-readable location name; `null` while not yet resolved (shows "Resolving…" in the UI) |
| `me_level` | integer | Material Efficiency level (0–10) |
| `te_level` | integer | Time Efficiency level (0–20) |
| `job` | object or `null` | Currently active or ready research job, or `null` if idle |

**Job fields** (when `job` is not null):

| Field | Type | Description |
|-------|------|-------------|
| `id` | integer | EVE job ID |
| `activity` | string | `"me_research"`, `"te_research"`, or `"copying"` |
| `status` | string | `"active"` (running) or `"ready"` (finished, not yet collected) |
| `start_date` | ISO 8601 datetime | When the job started |
| `end_date` | ISO 8601 datetime | When the job completes (or completed) |

**Derived status rules** (used by the frontend for display and filtering):

| Display status | Condition |
|----------------|-----------|
| Idle | `job = null` |
| Active | `job.status = "active"` |
| Ready | `job.status = "ready"` AND `end_date >= now` |
| Overdue | `job.status = "ready"` AND `end_date < now` |
| Completing today | `job.status = "active"` AND `end_date` is today |

---

### Jobs Summary

#### `GET /api/jobs/summary`

Returns aggregate counts and per-character slot usage for the dashboard summary bar.

**Response `200 OK`:**

```json
{
  "idle_blueprints": 12,
  "overdue_jobs": 2,
  "completing_today": 3,
  "free_research_slots": 0,
  "characters": [
    {
      "id": 12345678,
      "name": "My Character",
      "used_slots": 5
    }
  ]
}
```

| Field | Type | Description |
|-------|------|-------------|
| `idle_blueprints` | integer | BPOs with no active or ready job |
| `overdue_jobs` | integer | Jobs with `status = "ready"` and `end_date < now` |
| `completing_today` | integer | Jobs with `status = "active"` and `end_date` is today |
| `free_research_slots` | integer | Always `0` in MVP (requires per-character skill data) |
| `characters` | array | Per-character slot usage |
| `characters[].id` | integer | EVE character ID |
| `characters[].name` | string | Character name |
| `characters[].used_slots` | integer | Number of currently active research jobs |

---

### Sync

#### `POST /api/sync`

Sends a force-refresh signal to the background sync worker. Returns immediately without waiting for the sync to complete.

**Response `202 Accepted`** — no body.

Use `GET /api/sync/status` to poll for completion.

---

#### `GET /api/sync/status`

Returns the current sync state for all tracked subjects (characters and corporations) and endpoints.

**Response `200 OK`:**

```json
[
  {
    "owner_type": "character",
    "owner_id": 12345678,
    "owner_name": "My Character",
    "endpoint": "blueprints",
    "last_sync": "2026-02-23T09:00:00Z",
    "cache_until": "2026-02-23T09:05:00Z"
  },
  {
    "owner_type": "character",
    "owner_id": 12345678,
    "owner_name": "My Character",
    "endpoint": "jobs",
    "last_sync": "2026-02-23T09:00:00Z",
    "cache_until": "2026-02-23T09:05:00Z"
  }
]
```

| Field | Type | Description |
|-------|------|-------------|
| `owner_type` | string | `"character"` or `"corporation"` |
| `owner_id` | integer | EVE character or corporation ID |
| `owner_name` | string | Display name of the owner |
| `endpoint` | string | `"blueprints"` or `"jobs"` |
| `last_sync` | ISO 8601 datetime | When this subject/endpoint was last successfully synced |
| `cache_until` | ISO 8601 datetime | ESI cache expiry — the sync worker will not re-fetch before this time |

Returns an empty array `[]` if no characters have been added yet.

---

### OAuth2 (EVE SSO)

These endpoints are browser-navigation endpoints, not JSON API endpoints.

#### `GET /auth/eve/login`

Redirects the browser to the EVE SSO authorization page. The user selects a character and approves the requested scopes.

**Response:** `302 Found` → EVE SSO authorization URL

---

#### `GET /auth/eve/callback`

OAuth2 callback endpoint. EVE SSO redirects here after the user completes authorization.

**Query parameters** (set by EVE SSO, not by the caller):

| Parameter | Description |
|-----------|-------------|
| `code` | Authorization code |
| `state` | CSRF state token (must match the value from the login redirect) |

**Responses:**

| Status | Description |
|--------|-------------|
| `302 Found` → `/` | Authorization successful; character saved; immediate sync triggered |
| `400 Bad Request` | Missing `code`/`state`, or state mismatch (CSRF check failed) |
| `500 Internal Server Error` | Token exchange or character verification failed |

After a successful callback, Auspex:
1. Exchanges the authorization code for access and refresh tokens
2. Calls `GET /verify` to resolve the character ID and name
3. Calls ESI `GET /characters/{id}/` to resolve the character's corporation
4. Saves the character (with `corporation_id` and `corporation_name`) to SQLite
5. If the corporation is a player corporation (ID outside 1000000–2000000), inserts it into the `corporations` table with this character as delegate (`INSERT OR IGNORE` — if already tracked, the existing delegate is preserved)
6. Triggers an immediate background sync for the new character
7. Redirects to `/` (the React dashboard)

---

## ESI Endpoints Used

| Endpoint | Auth | Scope | Purpose |
|----------|------|-------|---------|
| `GET /characters/{id}/blueprints` | Bearer | `esi-blueprints.read_character_blueprints.v1` | Character BPO library |
| `GET /corporations/{id}/blueprints` | Bearer | `esi-blueprints.read_corporation_blueprints.v1` | Corporation BPO library |
| `GET /characters/{id}/industry/jobs/` | Bearer | `esi-industry.read_character_jobs.v1` | Character research jobs |
| `GET /corporations/{id}/industry/jobs/` | Bearer | `esi-industry.read_corporation_jobs.v1` | Corporation research jobs |
| `GET /corporations/{id}/assets/?page=N` | Bearer | `esi-assets.read_corporation_assets.v1` | Corp assets — used to resolve CorpSAG blueprint locations to real station/structure IDs via OfficeFolder entries |
| `GET /universe/types/{id}/` | None | — | Item type name and group |
| `GET /universe/groups/{id}/` | None | — | Group name and category |
| `GET /universe/categories/{id}/` | None | — | Category name |
| `GET /universe/stations/{id}/` | None | — | NPC station name (station IDs 60 000 000–64 000 000) |
| `GET /universe/structures/{id}/` | Bearer | `esi-universe.read_structures.v1` | Player structure name (IDs ≥ 1 000 000 000 000) |
| `GET /universe/systems/{id}/` | None | — | Solar system name |
| `POST /universe/names/` | None | — | Batch ID-to-name resolution |
