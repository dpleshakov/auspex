# Auspex — architecture.md

> Date: 21.02.2026
> Status: Current

---

## Tech Stack

### Selected Technologies

#### Backend

| Component | Solution | Version |
|-----------|----------|---------|
| Language | Go | 1.26+ |
| HTTP routing | Chi | v5 |
| Database | SQLite | — |
| SQLite driver | modernc.org/sqlite | latest |
| SQL code generation | sqlc | v2 |
| OAuth2 | golang.org/x/oauth2 | latest |
| HTTP client (ESI) | standard net/http | — |
| Static file embedding | standard embed | — |
| Testing | standard testing | — |
| Mocks | testify/mock | latest |

#### Frontend

| Component | Solution | Version |
|-----------|----------|---------|
| Framework | React | 18+ |
| Build tool | Vite | latest |
| Tables | TanStack Table | v8 |
| HTTP client | fetch API (standard) | — |
| Styles | CSS (no framework) | — |

#### Infrastructure

| Component | Solution |
|-----------|----------|
| Database | SQLite, single file next to the binary |
| Distribution | Single Go binary, static files embedded via embed |
| Configuration | Launch flags + config file (port, refresh interval) |

---

### Rationale

#### Go

Driven by requirements: a single binary with no external dependencies, cross-platform compilation (Windows / macOS / Linux), embedding static files via `embed`, future compatibility with Wails. Go compiles to a single executable that requires no runtime on the user's machine.

#### Chi

A lightweight HTTP router fully compatible with the standard `net/http` — no custom context, no vendor lock-in. Solves a specific problem: Auspex has 10+ endpoints and several middleware (logging, recover, CORS). Chi provides a middleware stack with route grouping without the boilerplate of the standard library. Built-in middleware (`Logger`, `Recoverer`) is ready to use out of the box. Compatibility with the standard `http.Handler` is important for a future migration to Wails.

#### SQLite

Driven by requirements: a single-user local application, a single file, no external dependencies. SQLite performance is more than sufficient — ESI data updates every 10+ minutes and the record count is in the hundreds, not millions.

#### modernc.org/sqlite (driver)

A pure Go implementation of SQLite — compiles without CGO. This is critical for cross-platform single-binary builds: CGO-dependent drivers require a C toolchain on the build machine for each target platform. modernc.org/sqlite eliminates this problem entirely.

#### sqlc

A code generator: takes a SQL schema and SQL queries, generates typed Go code. Solves the schema/code desync problem — when the schema changes, `sqlc generate` fails if queries are not updated. No manual `Scan`, the compiler checks types. SQL remains clean SQL with no ORM magic. Important for future modules (manufacturing, analytics) where queries will be more complex.

#### golang.org/x/oauth2

The official extended Go library for OAuth2. Handles the Authorization Code flow, automatic token refresh on expiry, and token storage. EVE SSO uses standard OAuth2 — the library fits without adaptation. The alternative (manual implementation) is possible but x/oauth2 already handles edge cases and is battle-tested in production.

#### testify/mock

Go's standard `testing` package handles test execution and assertions. `testify/mock` adds interface-based mocking — necessary for testing `sync` and `api` in isolation without real ESI or SQLite. The alternative (manual mock structs) works but requires significant boilerplate for every interface. testify/mock generates this automatically and integrates cleanly with the standard `testing` package.

#### React + Vite + TanStack Table

A BPO table with sorting, filters, highlighting, and periodic data refresh is exactly the class of problem TanStack Table was built for. Vanilla JS would require manual implementation of most of TanStack Table's functionality. React provides a component model (BPO table, characters section, summary row are natural components) and state management. Vite removes build complexity: `npm run build` produces static files that embed into the Go binary via `embed` just as easily as vanilla files.

---

### Considered Alternatives and Reasons for Rejection

| Component | Alternative | Reason for rejection |
|-----------|-------------|----------------------|
| Chi | `net/http` (stdlib) | No middleware stack; boilerplate when grouping routes with different middleware |
| Chi | Gin | Custom `gin.Context` instead of standard — friction during integration and future Wails migration |
| Chi | Echo | Same issues as Gin — custom context, vendor lock-in |
| sqlc | `database/sql` (stdlib) | Manual `Scan` for every query; schema and code easily fall out of sync; significant boilerplate |
| sqlc | GORM | Hides SQL behind ORM magic; hard to know what query actually runs; overkill for Auspex's predictable schema |
| modernc.org/sqlite | mattn/go-sqlite3 | Requires CGO — complicates cross-platform builds |
| React | Vue | Smaller ecosystem; TanStack Table is React-oriented; no meaningful advantage for this task |
| React | Vanilla JS | An interactive table with filters, state, and polling is exactly the class of task where vanilla JS turns into spaghetti |

---

### Known Stack Risks

**ESI API as an external dependency** — the entire project depends on ESI stability. CCP may change endpoints or restrict scopes. Mitigation: isolate the ESI client in a dedicated package so changes don't spread through the entire codebase.

**sqlc and schema** — when the DB schema changes, `sqlc generate` must be run and queries updated. This is an easy step to forget. Mitigation: add `sqlc generate` to the Makefile as a mandatory step before building.

**modernc.org/sqlite performance** — the pure Go implementation is slower than the CGO variant. For Auspex this is irrelevant (hundreds of records, updates every 10+ minutes), but worth keeping in mind if data volume were to grow significantly.

**React bundle size** — the frontend is embedded in the binary. React + TanStack Table will add ~200–300 KB gzip to the binary size. For a desktop application this is acceptable.

---

### References and Versions

- Go: https://go.dev/doc/ (1.26+)
- Chi: https://github.com/go-chi/chi (v5)
- sqlc: https://docs.sqlc.dev (v2)
- modernc.org/sqlite: https://pkg.go.dev/modernc.org/sqlite
- golang.org/x/oauth2: https://pkg.go.dev/golang.org/x/oauth2
- testify/mock: https://github.com/stretchr/testify
- EVE ESI: https://esi.evetech.net/ui/
- EVE SSO OAuth2: https://developers.eveonline.com/blog/article/sso-to-authenticated-calls
- React: https://react.dev (v18+)
- Vite: https://vitejs.dev
- TanStack Table: https://tanstack.com/table/v8

---

## Architecture

### High-Level Diagram

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

### Modules and Responsibilities

#### `config`
Reads and validates configuration at startup. Sources: command-line flags and a config file. Provides other packages with a typed config struct.

Parameters: server port, database file path, auto-refresh interval, ESI client_id and client_secret, callback URL.

#### `db`
Initializes the SQLite connection. Runs schema migrations at startup (up-only, no rollback for MVP). Provides `*sql.DB` to other packages.

#### `store`
sqlc-generated code — typed functions for all database queries. Contains no business logic, only CRUD. Not imported directly by `esi` or `auth` — only by `sync` and `api`.

#### `esi`
HTTP client for the ESI API. Responsibility: make HTTP requests to ESI and return typed structs. Has no knowledge of the database.

Respects ESI cache headers: reads `Expires` from the response and returns it to the caller. Handles ESI HTTP errors (429, 5xx) with retry logic.

Endpoints used:
- `GET /characters/{id}/blueprints`
- `GET /characters/{id}/industry/jobs`
- `GET /corporations/{id}/blueprints`
- `GET /corporations/{id}/industry/jobs`
- `GET /universe/types/{type_id}`
- `POST /universe/names/` (bulk resolve)

#### `auth`
OAuth2 flow for EVE SSO. Responsibility: generate the authorization URL, exchange code for tokens, refresh tokens on expiry, verify the character via `/verify`.

Uses `golang.org/x/oauth2`. Saves and reads tokens via `store`. Provides `auth.Client` — a wrapper around `esi` that automatically injects a fresh token into every request.

#### `sync`
Background worker and sync scheduler. Responsibility: knows when and what needs to be updated; coordinates `auth`/`esi` and `store`.

Starts as a goroutine at application startup. A ticker fires every N minutes (from config). On each tick, iterates over all subjects (characters + corporations), checks `sync_state.cache_until`, skips if the cache is still fresh.

Receives a force-refresh signal via a channel from `api` — in this case ignores `cache_until`.

After a successful sync, updates `sync_state` and triggers lazy resolution of any new `type_id`s via `esi`.

#### `api`
Chi router and HTTP handlers. Responsibility: accept HTTP requests, read data from `store`, return JSON responses. Never calls ESI directly.

Serves frontend static files via `embed`.

Middleware stack: `Logger`, `Recoverer`, `CORS`, `Content-Type: application/json` for API routes.

---

### Key Interfaces

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

### Database Schema

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

### API Contracts

See `api-docs.md` for the full API reference.

---

### Data Flows

#### Flow 1 — Adding a Character (OAuth)

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

#### Flow 2 — Background Sync

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

#### Flow 3 — Frontend Reading Data

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

#### Flow 4 — Force Refresh

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

### Key Architectural Decisions

**The backend never calls ESI synchronously during an HTTP request.** All ESI requests go exclusively through the background `sync` worker. `api` handlers read only from SQLite. This guarantees instant UI response regardless of ESI availability.

**Polymorphic ownership.** Blueprints and jobs belong to a subject via `owner_type` + `owner_id` instead of separate foreign keys. Integrity is enforced at the application layer.

**Lazy EVE universe data resolution.** The `eve_types`, `eve_groups`, `eve_categories` tables are populated on first encounter with a new `type_id` and never updated again — this data is stable.

**Separation of business logic and UI.** The backend is a clean REST API. The frontend is static files. Replacing the UI (Wails, native) does not touch the backend.

---

### Security Decisions

**Trust boundary.** The only external system that receives user data is the EVE ESI API — OAuth2 tokens are sent there and nowhere else. Everything else (SQLite, config file) stays local on the user's machine.

**Tokens never enter logs.** Chi Logger records method, URL, HTTP status, and response time. OAuth2 tokens travel in the `Authorization` header and never appear in URLs, so they are safe from accidental logging. Error responses from handlers must not include token values — errors are logged server-side only.

**Input validation at the API boundary.** All user-supplied values arriving via HTTP — `corporation_id`, `delegate_id`, query parameters — are validated in `api` handlers before being passed to `store`. The `esi` package validates ESI responses before returning them to `sync`.

**OAuth2 state parameter.** The `auth` package generates a random `state` value for each login flow and validates it on callback. This prevents CSRF attacks on the OAuth2 flow.

**Credentials never in git.** ESI `client_id` and `client_secret` live in `auspex.yaml` which is gitignored from day one. The repository contains only `auspex.example.yaml` with placeholder values.

---

## Project Structure

### Directory Layout

```
auspex/
├── cmd/
│   └── auspex/
│       ├── main.go
│       └── web/
│           ├── dist/               # gitignored (built by npm run build)
│           │   └── .gitkeep
│           ├── src/
│           │   ├── api/
│           │   │   └── client.js
│           │   ├── components/
│           │   │   ├── BlueprintTable.jsx
│           │   │   ├── CharactersSection.jsx
│           │   │   └── SummaryBar.jsx
│           │   ├── App.jsx
│           │   └── main.jsx
│           ├── index.html
│           ├── package.json
│           └── vite.config.js
├── docs
│   ├── architecture.md
│   ├── api-docs.md
│   ├── project-brief.md
│   ├── tech-debt.md
│   └── archive/
│       └── <archived_tasks.md>
├── internal/
│   ├── api/
│   │   ├── blueprints.go
│   │   ├── characters.go
│   │   ├── corporations.go
│   │   ├── router.go
│   │   ├── static.go
│   │   └── sync.go
│   ├── auth/
│   │   ├── client.go
│   │   └── oauth.go
│   ├── config/
│   │   └── config.go
│   ├── db/
│   │   ├── db.go
│   │   ├── migrations/
│   │   │   └── 001_initial.sql
│   │   └── queries/
│   │       ├── blueprints.sql
│   │       ├── characters.sql
│   │       ├── corporations.sql
│   │       ├── jobs.sql
│   │       ├── sync_state.sql
│   │       └── universe.sql
│   ├── esi/
│   │   ├── blueprints.go
│   │   ├── client.go
│   │   ├── jobs.go
│   │   └── universe.go
│   ├── store/                      # generated by sqlc — do not edit manually
│   │   └── .gitkeep
│   └── sync/
│       └── worker.go
├── tools/
│   ├── rm.go               # cross-platform file/dir removal (go run tools/rm.go)
│   └── touch.go            # cross-platform file touch/create (go run tools/touch.go)
├── .gitignore
├── go.mod
├── Makefile
└── sqlc.yaml
```

---

### File Descriptions

#### Root

| File | Purpose |
|------|---------|
| `go.mod` | Go module definition: `github.com/dpleshakov/auspex`, requires Go 1.26+ |
| `sqlc.yaml` | sqlc v2 config: reads schema from `internal/db/migrations/`, queries from `internal/db/queries/`, outputs typed Go code to `internal/store/` |
| `auspex.example.yaml` | **Created in Phase 7** when `internal/config/` is implemented. Field names are determined by the `Config` struct. The real `auspex.yaml` (with credentials) is gitignored. |
| `.gitignore` | Ignores binary, `web/dist/*` (except `.gitkeep`), `node_modules/`, `*.db`, `auspex.yaml` |
| `Makefile` | Build automation: `build`, `frontend`, `sqlc`, `test`, `lint`, `check`, `clean`, `clean-all` targets. On Windows requires `make` to be installed separately. |
| `tools/` | Go helper scripts for cross-platform build tasks. Tagged `//go:build ignore` — excluded from normal builds, invoked via `go run tools/<name>.go`. |

---

#### `cmd/auspex/`

Entry point for the Auspex binary.

| File | Purpose |
|------|---------|
| `main.go` | Wires all packages together: loads config, initializes DB, starts sync worker, starts HTTP server. Contains `//go:embed all:web/dist` to embed the built frontend into the binary. |

---

#### `cmd/auspex/web/`

React frontend source and build output. Lives under `cmd/auspex/` for a specific reason: Go's `//go:embed` directive cannot reference paths above the source file's directory (no `..` allowed). Placing `web/` here allows `main.go` to embed `web/dist` directly.

| File/Dir | Purpose |
|----------|---------|
| `index.html` | Vite HTML entry point |
| `package.json` | NPM dependencies: React 18, TanStack Table v8, Vite |
| `vite.config.js` | Vite config. In dev mode proxies `/api` and `/auth` to `localhost:8080` so the frontend talks to the running Go backend. |
| `dist/` | Build output — **not committed** (gitignored). Produced by `npm run build`. Embedded into the Go binary at compile time via `//go:embed`. |
| `dist/.gitkeep` | Committed to keep the `dist/` directory in the repo so `//go:embed` compiles on a fresh clone before `npm run build` has been run. |
| `src/main.jsx` | React root: mounts `<App />` into `#root` |
| `src/App.jsx` | Top-level component: composes `SummaryBar`, `CharactersSection`, `BlueprintTable` |
| `src/api/client.js` | All `fetch` calls to the backend API. Components import functions from here and never call `fetch` directly. |
| `src/components/SummaryBar.jsx` | Summary row: idle BPO count / overdue jobs / completing today / free research slots |
| `src/components/CharactersSection.jsx` | Per-character slot usage: Used / Total / Available |
| `src/components/BlueprintTable.jsx` | Main BPO table with sorting, filtering, and row highlighting via TanStack Table v8 |

---

#### `internal/config/`

| File | Purpose |
|------|---------|
| `config.go` | `Config` struct and loader. Sources: CLI flags and `auspex.yaml`. Fields: port, db_path, refresh_interval, ESI client_id/client_secret/callback_url. |

---

#### `internal/db/`

| File/Dir | Purpose |
|----------|---------|
| `db.go` | Opens the SQLite connection (`modernc.org/sqlite`, pure Go, no CGO). Runs migrations in order on startup. Returns `*sql.DB`. |
| `migrations/001_initial.sql` | Full DB schema: `eve_categories`, `eve_groups`, `eve_types`, `characters`, `corporations`, `blueprints`, `jobs`, `sync_state`. Migrations are up-only for MVP (no rollback). |
| `queries/blueprints.sql` | sqlc query definitions for the `blueprints` table |
| `queries/characters.sql` | sqlc query definitions for the `characters` table |
| `queries/corporations.sql` | sqlc query definitions for the `corporations` table |
| `queries/jobs.sql` | sqlc query definitions for the `jobs` table |
| `queries/sync_state.sql` | sqlc query definitions for the `sync_state` table |
| `queries/universe.sql` | sqlc query definitions for `eve_types`, `eve_groups`, `eve_categories` |

---

#### `internal/store/`

**Generated by sqlc — do not edit manually.**

Regenerate after any change to `internal/db/migrations/` or `internal/db/queries/`:
```
sqlc generate
```

Imported only by `internal/api/` and `internal/sync/`. Never imported by `internal/esi/` or `internal/auth/`.

---

#### `internal/esi/`

ESI HTTP client. Has no knowledge of the database.

| File | Purpose |
|------|---------|
| `client.go` | Base HTTP client: executes requests, parses the `Expires` header into `cache_until`, handles 429/5xx with retry logic |
| `blueprints.go` | `GET /characters/{id}/blueprints`, `GET /corporations/{id}/blueprints` |
| `jobs.go` | `GET /characters/{id}/industry/jobs`, `GET /corporations/{id}/industry/jobs` |
| `universe.go` | `GET /universe/types/{type_id}`, `POST /universe/names/` (bulk name resolve) |

---

#### `internal/auth/`

EVE SSO OAuth2 flow.

| File | Purpose |
|------|---------|
| `oauth.go` | Authorization URL generation, code → token exchange, character verification via `GET /verify` |
| `client.go` | `auth.Client`: wraps `esi` with automatic token refresh before each request. Reads/writes tokens via `store`. |

---

#### `internal/sync/`

Background sync worker and scheduler. Coordinates `auth`/`esi` and `store`.

| File | Purpose |
|------|---------|
| `worker.go` | Ticker fires every N minutes (from config). Iterates characters + corporations, checks `sync_state.cache_until`, skips if still fresh, otherwise fetches from ESI and upserts to DB. Accepts force-refresh signal via channel from `api`. After each successful sync, triggers lazy resolution of unknown `type_id`s. |

Note: if stdlib `sync` is needed inside this package, alias it: `import stdsync "sync"`.

---

#### `internal/api/`

Chi router and HTTP handlers. Never calls ESI directly — reads only from `store`.

| File | Purpose |
|------|---------|
| `router.go` | Assembles the Chi router, registers middleware (Logger, Recoverer, CORS, JSON content-type for `/api` routes), mounts all handlers |
| `static.go` | Serves the embedded frontend (`embed.FS` passed in from `main.go`). Non-API routes fall through to `index.html` (SPA routing). |
| `characters.go` | `GET /api/characters`, `DELETE /api/characters/{id}` |
| `corporations.go` | `GET /api/corporations`, `POST /api/corporations`, `DELETE /api/corporations/{id}` |
| `blueprints.go` | `GET /api/blueprints` (filters: status, owner_id, owner_type, category_id), `GET /api/jobs/summary` |
| `sync.go` | `POST /api/sync` (sends signal to sync worker), `GET /api/sync/status` |

---

#### `tools/`

Go helper scripts for cross-platform build automation. Each file carries `//go:build ignore`
so it is excluded from `go build ./...` but can be invoked directly via `go run`:

| File | Purpose |
|------|---------|
| `rm.go` | Removes files or directories. Flag `-r` enables recursive removal. Missing paths are silently skipped (like `rm -f`). |
| `touch.go` | Creates a file (and any missing parent directories). Updates mtime if the file already exists. |

---

#### `Makefile`

Top-level build automation. Targets:

| Target | Action |
|--------|--------|
| `build` | Full build: `frontend` → `sqlc` → `go build -o auspex ./cmd/auspex/` |
| `frontend` | `npm install && npm run build` inside `cmd/auspex/web/` |
| `sqlc` | `sqlc generate` |
| `test` | `go test ./...` |
| `lint` | `lint-go` + `lint-js` |
| `lint-go` | `go vet ./...` + `golangci-lint run` |
| `lint-js` | `npm audit --audit-level=high` |
| `check` | `lint` + `test` + `build` (run before pushing) |
| `clean` | Removes binary and rebuilds `web/dist/` with only `.gitkeep`. Uses `go run tools/rm.go` and `go run tools/touch.go` for cross-platform compatibility. |
| `clean-all` | `clean` + removes `auspex.db` |

**Build order matters:** the frontend must be built before `go build` so that `web/dist/` contains real files for `//go:embed`. `sqlc generate` must run before `go build` so that `internal/store/` contains generated code.
