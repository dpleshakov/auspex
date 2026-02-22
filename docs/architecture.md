# Auspex — architecture.md

> Phase 4: Architecture
> Date: 21.02.2026
> Status: Current

---

## High-Level Diagram

```
┌─────────────────────────────────────────────────────────┐
│                     Auspex Binary                       │
│                                                         │
│  ┌──────────┐    ┌──────────┐    ┌──────────────────┐  │
│  │  config  │    │    db    │    │      embed       │  │
│  │          │    │ SQLite   │    │  (static files)  │  │
│  └────┬─────┘    └────┬─────┘    └────────┬─────────┘  │
│       │               │                   │             │
│  ┌────▼───────────────▼───────────────────▼─────────┐  │
│  │                     api                          │  │
│  │              Chi router + handlers               │  │
│  └────────────────────┬─────────────────────────────┘  │
│                        │                                │
│  ┌─────────────────────▼──────────────────────────┐    │
│  │                    store                       │    │
│  │            sqlc-generated queries              │    │
│  └─────────────────────┬──────────────────────────┘    │
│                         │                               │
│  ┌──────────────────────▼─────────────────────────┐    │
│  │                     sync                       │    │
│  │         background worker + scheduler          │    │
│  └──────────┬───────────────────────┬─────────────┘    │
│             │                       │                   │
│  ┌──────────▼──────┐   ┌────────────▼──────────────┐   │
│  │      auth       │   │           esi             │   │
│  │   OAuth2 flow   │   │       ESI HTTP client     │   │
│  └─────────────────┘   └───────────────────────────┘   │
│                                                         │
└─────────────────────────────────────────────────────────┘
         │                                    │
         ▼                                    ▼
   Browser (localhost)               EVE ESI API
```

---

## Modules and Responsibilities

### `config`
Reads and validates configuration at startup. Sources: command-line flags and a config file. Provides other packages with a typed config struct.

Parameters: server port, database file path, auto-refresh interval, ESI client_id and client_secret, callback URL.

### `db`
Initializes the SQLite connection. Runs schema migrations at startup (up-only, no rollback for MVP). Provides `*sql.DB` to other packages.

### `store`
sqlc-generated code — typed functions for all database queries. Contains no business logic, only CRUD. Not imported directly by `esi` or `auth` — only by `sync` and `api`.

### `esi`
HTTP client for the ESI API. Responsibility: make HTTP requests to ESI and return typed structs. Has no knowledge of the database.

Respects ESI cache headers: reads `Expires` from the response and returns it to the caller. Handles ESI HTTP errors (429, 5xx) with retry logic.

Endpoints used:
- `GET /characters/{id}/blueprints`
- `GET /characters/{id}/industry/jobs`
- `GET /corporations/{id}/blueprints`
- `GET /corporations/{id}/industry/jobs`
- `GET /universe/types/{type_id}`
- `POST /universe/names/` (bulk resolve)

### `auth`
OAuth2 flow for EVE SSO. Responsibility: generate the authorization URL, exchange code for tokens, refresh tokens on expiry, verify the character via `/verify`.

Uses `golang.org/x/oauth2`. Saves and reads tokens via `store`. Provides `auth.Client` — a wrapper around `esi` that automatically injects a fresh token into every request.

### `sync`
Background worker and sync scheduler. Responsibility: knows when and what needs to be updated; coordinates `auth`/`esi` and `store`.

Starts as a goroutine at application startup. A ticker fires every N minutes (from config). On each tick, iterates over all subjects (characters + corporations), checks `sync_state.cache_until`, skips if the cache is still fresh.

Receives a force-refresh signal via a channel from `api` — in this case ignores `cache_until`.

After a successful sync, updates `sync_state` and triggers lazy resolution of any new `type_id`s via `esi`.

### `api`
Chi router and HTTP handlers. Responsibility: accept HTTP requests, read data from `store`, return JSON responses. Never calls ESI directly.

Serves frontend static files via `embed`.

Middleware stack: `Logger`, `Recoverer`, `CORS`, `Content-Type: application/json` for API routes.

---

## Key Interfaces

Dependency injection via interfaces is the mechanism that makes unit testing possible without real ESI or SQLite. The following interfaces must be defined:

**`esi.Client` interface** — used by `sync` and `auth`. Allows substituting a mock ESI client in tests:

```go
type Client interface {
    GetCharacterBlueprints(ctx context.Context, characterID int64, token string) ([]Blueprint, time.Time, error)
    GetCharacterJobs(ctx context.Context, characterID int64, token string) ([]Job, time.Time, error)
    GetCorporationBlueprints(ctx context.Context, corporationID int64, token string) ([]Blueprint, time.Time, error)
    GetCorporationJobs(ctx context.Context, corporationID int64, token string) ([]Job, time.Time, error)
    GetUniverseType(ctx context.Context, typeID int64) (UniverseType, error)
}
```

**`store.Querier` interface** — generated by sqlc automatically. Used by `sync` and `api`. Allows substituting a mock store in tests without a real SQLite file.

**`auth.TokenRefresher` interface** — used by `sync`. Allows testing sync logic without a real OAuth2 flow:

```go
type TokenRefresher interface {
    FreshToken(ctx context.Context, characterID int64) (string, error)
}
```

Packages that depend on these interfaces receive them via constructor arguments, never instantiate them internally.

---

## Database Schema

```sql
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
    id            INTEGER PRIMARY KEY,  -- EVE character_id
    name          TEXT NOT NULL,
    access_token  TEXT NOT NULL,
    refresh_token TEXT NOT NULL,
    token_expiry  DATETIME NOT NULL,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
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
    id          INTEGER PRIMARY KEY,  -- EVE item_id
    owner_type  TEXT NOT NULL,        -- 'character' | 'corporation'
    owner_id    INTEGER NOT NULL,
    type_id     INTEGER NOT NULL REFERENCES eve_types(id),
    location_id INTEGER NOT NULL,
    me_level    INTEGER NOT NULL DEFAULT 0,
    te_level    INTEGER NOT NULL DEFAULT 0,
    updated_at  DATETIME NOT NULL
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

-- ESI cache state per subject per endpoint
CREATE TABLE sync_state (
    owner_type  TEXT NOT NULL,
    owner_id    INTEGER NOT NULL,
    endpoint    TEXT NOT NULL,      -- 'blueprints' | 'jobs'
    last_sync   DATETIME NOT NULL,
    cache_until DATETIME NOT NULL,
    PRIMARY KEY (owner_type, owner_id, endpoint)
);
```

---

## API Contracts

### Characters

**`GET /api/characters`**
```json
[
  {
    "id": 12345,
    "name": "TinkerBear",
    "token_expiry": "2026-02-21T12:00:00Z",
    "created_at": "2026-02-01T10:00:00Z"
  }
]
```

**`DELETE /api/characters/{id}`**
```
204 No Content
```

---

### OAuth Flow

**`GET /auth/eve/login`**
```
302 redirect → EVE SSO authorization URL
```

**`GET /auth/eve/callback?code=...&state=...`**
```
302 redirect → / (frontend)
```

---

### Corporations

**`GET /api/corporations`**
```json
[
  {
    "id": 99999,
    "name": "Bear Industries",
    "delegate_id": 12345,
    "delegate_name": "TinkerBear",
    "created_at": "2026-02-01T10:00:00Z"
  }
]
```

**`POST /api/corporations`**
```json
// request
{ "corporation_id": 99999, "delegate_id": 12345 }

// response 201 Created
{ "id": 99999, "name": "Bear Industries", "delegate_id": 12345 }
```

**`DELETE /api/corporations/{id}`**
```
204 No Content
```

---

### Blueprints

**`GET /api/blueprints?status=idle&owner_id=12345&owner_type=character&category_id=6`**

Query parameters (all optional):
- `status` — `idle` | `active` | `ready`
- `owner_id` — filter by character or corporation
- `owner_type` — `character` | `corporation`
- `category_id` — filter by category

```json
[
  {
    "id": 1000000001,
    "owner_type": "character",
    "owner_id": 12345,
    "owner_name": "TinkerBear",
    "type_id": 670,
    "type_name": "Capsule Blueprint",
    "category_id": 6,
    "category_name": "Ship",
    "group_id": 29,
    "group_name": "Capsule",
    "location_id": 60000004,
    "me_level": 10,
    "te_level": 20,
    "job": {
      "id": 555,
      "activity": "me_research",
      "status": "active",
      "start_date": "2026-02-20T10:00:00Z",
      "end_date": "2026-02-22T10:00:00Z",
      "installer_id": 12345,
      "installer_name": "TinkerBear"
    }
  },
  {
    "id": 1000000002,
    "owner_type": "corporation",
    "owner_id": 99999,
    "owner_name": "Bear Industries",
    "type_id": 588,
    "type_name": "Rifter Blueprint",
    "category_id": 6,
    "category_name": "Ship",
    "group_id": 25,
    "group_name": "Frigate",
    "location_id": 60000004,
    "me_level": 0,
    "te_level": 0,
    "job": null
  }
]
```

---

### Summary

**`GET /api/jobs/summary`**
```json
{
  "idle_blueprints": 5,
  "overdue_jobs": 2,
  "completing_today": 3,
  "free_research_slots": 4,
  "characters": [
    {
      "id": 12345,
      "name": "TinkerBear",
      "used_slots": 4,
      "total_slots": 5,
      "available_slots": 1
    }
  ]
}
```

---

### Sync

**`POST /api/sync`**
```
202 Accepted

{ "message": "sync started" }
```

**`GET /api/sync/status`**
```json
[
  {
    "owner_type": "character",
    "owner_id": 12345,
    "owner_name": "TinkerBear",
    "endpoint": "blueprints",
    "last_sync": "2026-02-21T11:50:00Z",
    "cache_until": "2026-02-21T12:05:00Z"
  },
  {
    "owner_type": "corporation",
    "owner_id": 99999,
    "owner_name": "Bear Industries",
    "endpoint": "jobs",
    "last_sync": "2026-02-21T11:50:00Z",
    "cache_until": "2026-02-21T12:05:00Z"
  }
]
```

---

## Data Flows

### Flow 1 — Adding a Character (OAuth)

```
User → GET /auth/eve/login
     → 302 redirect to EVE SSO
     → User authenticates on CCP site
     → EVE SSO → GET /auth/eve/callback?code=...
     → auth: exchange code for access_token + refresh_token
     → auth: GET /verify → character_id + name
     → store: INSERT INTO characters
     → sync: trigger immediate sync for new character
     → 302 redirect to frontend
```

### Flow 2 — Background Sync

```
sync worker (ticker every N minutes)
  → store: SELECT all characters + corporations
  → for each subject:
      → store: SELECT sync_state WHERE owner = subject
      → if cache_until > now: skip
      → auth: ensure token is fresh (refresh if needed)
      → esi: GET /characters/{id}/blueprints (or /corporations/{id}/blueprints)
      → esi: GET /characters/{id}/industry/jobs
      → store: UPSERT blueprints
      → store: UPSERT jobs (only status: active | ready)
      → for each new type_id not in eve_types:
          → esi: GET /universe/types/{type_id}
          → store: INSERT INTO eve_types + eve_groups + eve_categories
      → store: UPDATE sync_state (last_sync, cache_until from Expires header)
```

### Flow 3 — Frontend Reading Data

```
Frontend (auto-poll every N minutes or manual refresh button)
  → GET /api/blueprints?filters...
  → api handler: store.ListBlueprints(filters)
      → JOIN blueprints + jobs + eve_types + eve_groups + eve_categories
  → return JSON array (blueprint with nested job object or null)

  → GET /api/jobs/summary
  → api handler: store.GetSummary()
      → aggregate counts: idle, overdue, completing_today
      → per-character slot counts
  → return JSON summary object
```

### Flow 4 — Force Refresh

```
User clicks "Refresh" button
  → POST /api/sync
  → api: send signal to sync worker via channel
  → return 202 Accepted immediately (no waiting)

sync worker receives signal
  → ignore cache_until for all subjects
  → run full sync (same as Flow 2)

Frontend polls GET /api/sync/status every 2s
  → checks last_sync timestamp
  → when last_sync updated → re-fetch /api/blueprints
  → stop polling
```

---

## Key Architectural Decisions

**The backend never calls ESI synchronously during an HTTP request.** All ESI requests go exclusively through the background `sync` worker. `api` handlers read only from SQLite. This guarantees instant UI response regardless of ESI availability.

**Polymorphic ownership.** Blueprints and jobs belong to a subject via `owner_type` + `owner_id` instead of separate foreign keys. Integrity is enforced at the application layer.

**Lazy EVE universe data resolution.** The `eve_types`, `eve_groups`, `eve_categories` tables are populated on first encounter with a new `type_id` and never updated again — this data is stable.

**Separation of business logic and UI.** The backend is a clean REST API. The frontend is static files. Replacing the UI (Wails, native) does not touch the backend.

---

## Security Decisions

**Trust boundary.** The only external system that receives user data is the EVE ESI API — OAuth2 tokens are sent there and nowhere else. Everything else (SQLite, config file) stays local on the user's machine.

**Tokens never enter logs.** Chi Logger records method, URL, HTTP status, and response time. OAuth2 tokens travel in the `Authorization` header and never appear in URLs, so they are safe from accidental logging. Error responses from handlers must not include token values — errors are logged server-side only.

**Input validation at the API boundary.** All user-supplied values arriving via HTTP — `corporation_id`, `delegate_id`, query parameters — are validated in `api` handlers before being passed to `store`. The `esi` package validates ESI responses before returning them to `sync`.

**OAuth2 state parameter.** The `auth` package generates a random `state` value for each login flow and validates it on callback. This prevents CSRF attacks on the OAuth2 flow.

**Credentials never in git.** ESI `client_id` and `client_secret` live in `auspex.yaml` which is gitignored from day one. The repository contains only `auspex.example.yaml` with placeholder values.
