# Auspex — tasks.md

> Phase 6: Task Breakdown
> Date: 21.02.2026
> Status: Draft

---

## Overview

Total: 26 tasks across 7 layers. Order is bottom-up: each layer depends on the previous one.

| Layer | Tasks | Description |
|-------|-------|-------------|
| 1 | TASK-01 – TASK-03 | Foundation: config, db, store |
| 2 | TASK-04 – TASK-06 | ESI HTTP client |
| 3 | TASK-07 – TASK-08 | Auth: OAuth2 flow, token refresh |
| 4 | TASK-09 – TASK-12 | Sync worker |
| 5 | TASK-13 – TASK-17 | API: router, handlers |
| 6 | TASK-18 – TASK-19 | Main wiring + build scripts |
| 7 | TASK-20 – TASK-26 | Frontend |

---

## Layer 1 — Foundation

### TASK-01 `config`
**Status:** ✅ Done — commit c9b1725

**Description:** `Config` struct and loader. Sources: CLI flags and `auspex.yaml`. Fields: port, db_path, refresh_interval, ESI client_id, client_secret, callback_url. Validation of required fields at startup.

**Definition of done:**
- `config.Load()` reads from flags and yaml file
- Missing required fields (client_id, client_secret) return a descriptive error
- Default values applied for optional fields (port: 8080, refresh_interval: 10m)
- Tests: valid config loads correctly, missing required fields return error, default values are applied

**Dependencies:** none

---

### TASK-02 `db`
**Status:** ✅ Done — commit e89b1d7

**Description:** SQLite connection initialization using `modernc.org/sqlite`. Runs migration files in order at startup. Returns `*sql.DB`.

**Definition of done:**
- `db.Open(path)` opens the SQLite connection
- Migrations in `internal/db/migrations/` are applied in filename order
- Repeated startup is idempotent (migrations do not re-run)
- Tests: migrations apply without error, repeated calls are idempotent

**Dependencies:** TASK-01

---

### TASK-03 `store`
**Status:** ✅ Done — commit fc63e54

**Description:** Full DB schema in `migrations/001_initial.sql`. sqlc queries for all tables: characters, corporations, blueprints, jobs, sync_state, eve_types, eve_groups, eve_categories. Run `sqlc generate` to produce `internal/store/`.

**Definition of done:**
- `001_initial.sql` contains complete schema matching `architecture.md`
- `sqlc.yaml` configured correctly (schema path, queries path, output path)
- `sqlc generate` completes without errors
- All query files cover CRUD needed by `sync` and `api`
- Tests: not required (generated code)

**Dependencies:** TASK-02

---

## Layer 2 — ESI HTTP Client

### TASK-04 `esi` base client
**Status:** ✅ Done — commit 3907a42

**Description:** Base HTTP client struct. Executes requests with Authorization header, parses `Expires` response header into `time.Time`, implements retry logic on 429 (respect `Retry-After`) and 5xx (exponential backoff, max 3 retries).

**Definition of done:**
- `esi.NewClient(httpClient)` constructor accepts `*http.Client` for testability
- `Expires` header parsed correctly into `cache_until`
- 429 response retried after `Retry-After` delay
- 5xx response retried with exponential backoff
- Tests: cache header parsing, retry on 429, retry on 5xx, no retry on 4xx (via `httptest.NewServer`)

**Dependencies:** none

---

### TASK-05 `esi` blueprints + jobs
**Description:** ESI endpoints for blueprints and industry jobs. Character and corporation variants.

**Endpoints:**
- `GET /characters/{id}/blueprints`
- `GET /corporations/{id}/blueprints`
- `GET /characters/{id}/industry/jobs`
- `GET /corporations/{id}/industry/jobs`

**Definition of done:**
- Typed response structs for blueprints and jobs
- BPC filtered out (quantity != -1 means BPC — skip)
- Jobs filtered to status `active` and `ready` only
- `esi.Client` interface defined covering all methods
- Tests: response parsing via `httptest`, BPC filtering, job status filtering

**Dependencies:** TASK-04

---

### TASK-06 `esi` universe
**Description:** ESI endpoints for EVE universe reference data.

**Endpoints:**
- `GET /universe/types/{type_id}` — returns type name, group_id
- `GET /universe/groups/{group_id}` — returns group name, category_id
- `GET /universe/categories/{category_id}` — returns category name

**Definition of done:**
- Typed response structs for type, group, category
- Tests: response parsing via `httptest`

**Dependencies:** TASK-04

---

## Layer 3 — Auth

### TASK-07 `auth` OAuth flow
**Description:** EVE SSO OAuth2 Authorization Code flow. Generate authorization URL with state parameter, exchange code for tokens, verify character via `/verify`, save tokens to store.

**Definition of done:**
- `auth.GenerateAuthURL()` returns URL with random state, state saved to verify callback
- `auth.HandleCallback(code, state)` validates state, exchanges code, calls `/verify`, saves character to store
- State mismatch returns error
- Tests: URL generation includes state, `/verify` response parsed correctly, invalid state rejected

**Dependencies:** TASK-03, TASK-04

---

### TASK-08 `auth` token refresh
**Description:** Automatic token refresh when access token is expired. `auth.Client` wraps `esi.Client` and injects a fresh token before every request. Implements `auth.TokenRefresher` interface.

**Definition of done:**
- `auth.Client` calls `store.GetCharacter` to check token expiry before each request
- Expired token triggers refresh via `golang.org/x/oauth2`, updated token saved to store
- `auth.Client` implements `esi.Client` interface (transparent to `sync`)
- Tests: fresh token used when valid, refresh called when expired, refreshed token saved to store (via mock store)

**Dependencies:** TASK-05, TASK-07

---

## Layer 4 — Sync Worker

### TASK-09 `sync` worker skeleton
**Description:** Background goroutine with ticker. On each tick iterates all characters and corporations from store, checks `sync_state.cache_until` per subject per endpoint, skips if cache is still fresh.

**Definition of done:**
- Worker starts as goroutine, stops cleanly on context cancellation
- Ticker interval read from config
- `cache_until > now` → subject skipped
- `cache_until <= now` → subject queued for sync
- Tests: subject skipped when cache fresh, subject processed when cache expired (via mock store)

**Dependencies:** TASK-03, TASK-08

---

### TASK-10 `sync` full sync cycle
**Description:** Full sync for one subject: fetch blueprints and jobs from ESI, upsert to store, update sync_state with `cache_until` from ESI `Expires` header.

**Definition of done:**
- Blueprints upserted (insert or update on conflict)
- Jobs upserted, stale jobs (no longer in ESI response) deleted
- `sync_state` updated with `last_sync = now` and `cache_until` from response
- Tests: upsert called with correct data, stale jobs removed, sync_state updated (via mock esi.Client and store.Querier)

**Dependencies:** TASK-09

---

### TASK-11 `sync` lazy type resolution
**Description:** After each successful sync, collect all `type_id`s from new blueprints. For each `type_id` not present in `eve_types`, fetch from ESI and insert into `eve_types`, `eve_groups`, `eve_categories`.

**Definition of done:**
- New type_ids detected after upsert
- ESI `/universe/types`, `/universe/groups`, `/universe/categories` called in sequence
- Data inserted into all three tables
- Already-known type_ids skipped (no redundant ESI calls)
- Tests: new type_id triggers ESI call and insert, known type_id skipped (via mocks)

**Dependencies:** TASK-10, TASK-06

---

### TASK-12 `sync` force refresh
**Description:** Channel-based signal from `api` to sync worker. When signal received, ignore `cache_until` for all subjects and run full sync immediately.

**Definition of done:**
- `sync.Worker` exposes `ForceRefresh()` method that sends to internal channel
- Worker receives signal and runs sync cycle ignoring `cache_until`
- `api` calls `ForceRefresh()` on `POST /api/sync`
- Tests: force refresh triggers sync despite fresh cache (via mock store and esi.Client)

**Dependencies:** TASK-10

---

## Layer 5 — API

### TASK-13 `api` router + middleware
**Description:** Chi router assembly. Middleware stack for all routes: Logger, Recoverer. Additional middleware for `/api` group: `Content-Type: application/json`, CORS. Static file serving via `embed.FS` for non-API routes with SPA fallback to `index.html`.

**Definition of done:**
- `/api/*` routes return `Content-Type: application/json`
- Panic in handler returns 500, not crash
- Non-API routes serve embedded static files
- Unknown non-API routes fall through to `index.html`
- Tests: middleware applied correctly, static fallback works, panic recovery returns 500

**Dependencies:** TASK-03

---

### TASK-14 `api` characters + corporations
**Description:** HTTP handlers for character and corporation management.

**Endpoints:**
- `GET /api/characters` — list all characters
- `DELETE /api/characters/{id}` — remove character and their data
- `GET /api/corporations` — list all corporations with delegate name
- `POST /api/corporations` — add corporation by id + delegate_id
- `DELETE /api/corporations/{id}` — remove corporation

**Definition of done:**
- All endpoints return correct HTTP status codes
- `DELETE /api/characters/{id}` cascades: removes character's blueprints, jobs, sync_state
- `POST /api/corporations` validates delegate_id exists in characters table
- Tests: correct status codes, cascade delete, invalid delegate_id returns 400 (via mock store.Querier)

**Dependencies:** TASK-13

---

### TASK-15 `api` blueprints + summary
**Description:** Blueprint library endpoint with filters and summary endpoint.

**Endpoints:**
- `GET /api/blueprints?status=&owner_id=&owner_type=&category_id=` — filtered BPO list with nested job
- `GET /api/jobs/summary` — aggregate counts and per-character slot usage

**Definition of done:**
- All four filter params are optional and combinable
- Each blueprint includes nested `job` object or `null`
- Summary counts: idle_blueprints, overdue_jobs (end_date < now AND status = ready), completing_today, free_research_slots
- Per-character slot counts included in summary
- Tests: filters applied correctly, overdue logic correct, summary counts accurate (via mock store.Querier)

**Dependencies:** TASK-13

---

### TASK-16 `api` sync endpoints
**Description:** Sync control endpoints.

**Endpoints:**
- `POST /api/sync` — send force-refresh signal to sync worker, return 202 immediately
- `GET /api/sync/status` — return sync_state rows with owner names

**Definition of done:**
- `POST /api/sync` returns 202 without waiting for sync to complete
- `GET /api/sync/status` joins sync_state with characters/corporations for owner_name
- Tests: 202 returned immediately, status response includes owner names (via mock store and mock worker)

**Dependencies:** TASK-13, TASK-12

---

### TASK-17 `api` OAuth handlers
**Description:** EVE SSO OAuth2 HTTP handlers.

**Endpoints:**
- `GET /auth/eve/login` — redirect to EVE SSO authorization URL
- `GET /auth/eve/callback?code=...&state=...` — exchange code, save character, redirect to frontend

**Definition of done:**
- `/auth/eve/login` returns 302 to EVE SSO URL
- `/auth/eve/callback` validates state, saves character, redirects to `/`
- Invalid state returns 400
- After successful callback, immediate sync triggered for new character
- Tests: redirect URL correct, invalid state rejected, character saved on valid callback (via mock auth)

**Dependencies:** TASK-13, TASK-07

---

## Layer 6 — Main + Build

### TASK-18 `main.go` + example config
**Description:** Application entry point. Wires all packages together: load config, open DB, run migrations, create store, create esi client, create auth client, start sync worker, start HTTP server. Graceful shutdown on SIGINT/SIGTERM. Create `auspex.example.yaml`.

**Definition of done:**
- All packages initialized in correct order
- Sync worker started as goroutine before HTTP server
- Graceful shutdown: stop accepting requests, wait for sync worker to finish current cycle
- `auspex.example.yaml` documents all config fields with comments
- Manual smoke test: binary starts, opens in browser, OAuth flow completes

**Dependencies:** TASK-17, TASK-16

---

### TASK-19 Build scripts
**Description:** Finalize `scripts/build.sh` and `scripts/build.cmd` for full cross-platform builds.

**Scripts:**
- `scripts/build.sh` — macOS/Linux: `npm install` + `npm run build` → `sqlc generate` → `go build`
- `scripts/build.cmd` — Windows CMD: same sequence

**Definition of done:**
- `scripts/build.sh` produces working binary on macOS/Linux
- `scripts/build.cmd` produces working binary on Windows
- Both scripts fail with a clear error if any step fails
- Build fails clearly if `web/dist/` is empty (frontend not built)
- `go test ./...` documented as the standard way to run tests

**Dependencies:** TASK-18

---

## Layer 7 — Frontend

### TASK-20 Vite + React scaffold
**Description:** Initialize React + Vite project in `cmd/auspex/web/`. Configure Vite proxy for `/api` and `/auth` to backend in dev mode. Implement `src/api/client.js` with all fetch functions.

**Definition of done:**
- `npm run dev` starts frontend with proxy to `localhost:8080`
- `npm run build` produces files in `web/dist/`
- `api/client.js` exports functions for all backend endpoints: `getCharacters`, `deleteCharacter`, `getCorporations`, `addCorporation`, `deleteCorporation`, `getBlueprints`, `getJobsSummary`, `postSync`, `getSyncStatus`
- No component logic in `client.js` — pure fetch wrappers only

**Dependencies:** TASK-18

---

### TASK-21 `SummaryBar` component
**Description:** Top summary bar showing aggregate counts from `GET /api/jobs/summary`.

**Displays:** Idle BPOs / Overdue jobs / Completing today / Free research slots. Each as a labeled count, visually distinct for non-zero overdue and idle values.

**Definition of done:**
- Data fetched from `getJobsSummary()`
- Overdue count highlighted red when > 0
- Idle count highlighted when > 0
- Loading and error states handled

**Dependencies:** TASK-20

---

### TASK-22 `CharactersSection` component
**Description:** Per-character slot usage table from `GET /api/jobs/summary`.

**Displays:** Character name / Used slots / Total slots / Available slots. Row highlighted when available slots > 0.

**Definition of done:**
- Data from `summary.characters` array
- Row highlighted when `available_slots > 0`
- Empty state shown when no characters added

**Dependencies:** TASK-20

---

### TASK-23 `BlueprintTable` scaffold
**Description:** Basic BPO table using TanStack Table v8. Fetches data from `GET /api/blueprints`. Renders all columns without sorting, filtering, or highlighting.

**Columns:** Name / Category / Assigned (owner name) / Location / ME% / TE% / Status / Date End

**Definition of done:**
- Table renders with all columns
- `job.activity` + `job.status` combined into human-readable Status cell (Idle / ME Research / TE Research / Copying / Ready)
- `job.end_date` formatted as local datetime
- Loading and error states handled

**Dependencies:** TASK-20

---

### TASK-24 `BlueprintTable` sorting + filtering
**Description:** Add sorting and filtering to the blueprint table.

**Sorting:** Default sort: status priority (Overdue → Ready → Idle → Active), then end_date ascending. User can override by clicking column headers.

**Filtering:** Filter controls above table for: status (dropdown), owner (dropdown populated from data), category (dropdown populated from data).

**Definition of done:**
- Default sort puts overdue and idle at top
- All three filter dropdowns work independently and in combination
- Filters and sort state preserved on data refresh
- "Clear filters" button resets to defaults

**Dependencies:** TASK-23

---

### TASK-25 `BlueprintTable` row highlighting
**Description:** Visual highlighting of rows based on job status and end date.

**Rules:**
- Red: `status = ready` AND `end_date < now` (overdue — job finished but not collected)
- Yellow: `status = active` AND `end_date` is today
- Neutral label "Idle": `job = null`

**Definition of done:**
- Red rows visible for overdue jobs
- Yellow rows visible for jobs completing today
- Idle rows have distinct visual label
- Highlighting updates correctly on data refresh

**Dependencies:** TASK-23

---

### TASK-26 `App` + auto-polling
**Description:** Top-level `App` component assembling all components. Auto-polling logic and force-refresh flow.

**Auto-polling:** Fetch `getBlueprints()` and `getJobsSummary()` every N minutes (read from config endpoint or hardcode default 10 min for MVP).

**Force-refresh flow:**
- "Refresh" button calls `postSync()`
- Start polling `getSyncStatus()` every 2 seconds
- When `last_sync` timestamp changes — re-fetch blueprints and summary
- Stop polling

**Definition of done:**
- All three components rendered: SummaryBar, CharactersSection, BlueprintTable
- Auto-polling updates data without page reload
- Force-refresh button triggers sync and re-fetches data when complete
- "Refreshing..." indicator shown during force-refresh polling

**Dependencies:** TASK-21, TASK-22, TASK-25

---

## Dependency Graph

```
TASK-01 (config)
  └── TASK-02 (db)
        └── TASK-03 (store)
              ├── TASK-07 (auth oauth)
              │     └── TASK-08 (auth refresh)
              │           └── TASK-09 (sync worker)
              │                 └── TASK-10 (sync full)
              │                       ├── TASK-11 (sync types)
              │                       └── TASK-12 (sync force)
              └── TASK-13 (api router)
                    ├── TASK-14 (api characters/corps)
                    ├── TASK-15 (api blueprints)
                    ├── TASK-16 (api sync) ← TASK-12
                    └── TASK-17 (api oauth) ← TASK-07

TASK-04 (esi base)
  ├── TASK-05 (esi blueprints/jobs) ← TASK-07, TASK-08
  └── TASK-06 (esi universe) ← TASK-11

TASK-18 (main) ← TASK-17, TASK-16
  └── TASK-19 (build scripts)

TASK-18 → TASK-20 (frontend scaffold)
  ├── TASK-21 (SummaryBar)
  ├── TASK-22 (CharactersSection)
  └── TASK-23 (BlueprintTable scaffold)
        ├── TASK-24 (sorting/filtering)
        └── TASK-25 (highlighting)
              └── TASK-26 (App + polling)
```
