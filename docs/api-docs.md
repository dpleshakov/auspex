# Auspex — API Documentation

All API endpoints are served under `http://localhost:PORT` (default port: 8080).

- `/api/*` — JSON REST API (all responses include `Content-Type: application/json`)
- `/auth/*` — EVE SSO OAuth2 flow (redirect-based, not JSON)
- `/*` — React SPA (serves `index.html` for any unmatched path)

Errors are returned as:

```json
{ "error": "human-readable message" }
```

---

## Authentication

Auspex uses EVE SSO OAuth2. Authentication is browser-based — there are no API keys or session tokens for the REST API. The backend stores OAuth tokens per character in SQLite and uses them internally for ESI requests.

---

## Characters

### `GET /api/characters`

Returns all characters that have been added via the OAuth2 flow.

**Response `200 OK`:**

```json
[
  {
    "id": 12345678,
    "name": "My Character",
    "created_at": "2026-02-21T10:00:00Z"
  }
]
```

| Field | Type | Description |
|-------|------|-------------|
| `id` | integer | EVE character ID |
| `name` | string | Character name |
| `created_at` | ISO 8601 datetime | When the character was added |

Returns an empty array `[]` if no characters have been added.

---

### `DELETE /api/characters/{id}`

Removes a character and all associated data (blueprints, jobs, sync state).

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

## Corporations

### `GET /api/corporations`

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

### `POST /api/corporations`

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

### `DELETE /api/corporations/{id}`

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

## Blueprints

### `GET /api/blueprints`

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
| `location_id` | integer | ESI location ID (raw; human-readable names are a post-MVP feature) |
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

## Jobs Summary

### `GET /api/jobs/summary`

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

## Sync

### `POST /api/sync`

Sends a force-refresh signal to the background sync worker. Returns immediately without waiting for the sync to complete.

**Response `202 Accepted`** — no body.

Use `GET /api/sync/status` to poll for completion.

---

### `GET /api/sync/status`

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

## OAuth2 (EVE SSO)

These endpoints are browser-navigation endpoints, not JSON API endpoints.

### `GET /auth/eve/login`

Redirects the browser to the EVE SSO authorization page. The user selects a character and approves the requested scopes.

**Response:** `302 Found` → EVE SSO authorization URL

---

### `GET /auth/eve/callback`

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
3. Saves the character and tokens to SQLite
4. Triggers an immediate background sync for the new character
5. Redirects to `/` (the React dashboard)
