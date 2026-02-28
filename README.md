# Auspex

A local desktop tool for EVE Online industry players who manage multiple manufacturing characters. Auspex pulls data from the EVE ESI API via OAuth2 and presents a unified dashboard of your BPO library and research job status across all characters and corporations.

## Features

- Multi-character support via EVE SSO OAuth2
- Unified BPO table with ME%, TE%, status, owner, location, and job end date
- Visual row highlighting: red for overdue jobs, yellow for jobs completing today
- Summary bar: idle BPOs / overdue / completing today / free research slots
- Per-character slot usage table
- Sort by any column; filter by status, owner, and category
- Auto-refresh on a configurable interval
- Manual force-refresh with live polling feedback
- Single binary — no Docker, no PostgreSQL, no Redis required

## Known Limitations

- **Corporation support is not yet available via the UI.** The backend endpoint `POST /api/corporations` exists but there is no frontend for it — tracked in [docs/tasks-backlog.md](docs/tasks-backlog.md).
- Only the first page of ESI results is fetched — large corporation BPO libraries (>1000 items) will be truncated silently.
- Location IDs are displayed as raw numbers; human-readable station/structure names are not yet implemented.
- Free research slots count is always 0 (requires per-character skill data from ESI, not yet implemented).

See [docs/tech-debt.md](docs/tech-debt.md) and [docs/tasks-backlog.md](docs/tasks-backlog.md) for the full list of known issues and deferred decisions.

## Prerequisites

| Tool | Version | Notes |
|------|---------|-------|
| Go | 1.26+ | [golang.org](https://golang.org/dl/) |
| Node.js | 18+ | [nodejs.org](https://nodejs.org/) |
| make | any | Bundled with macOS/Linux. Windows: install separately. |
| sqlc | v2 | `go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest` |
| golangci-lint | v2 | `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest` — required for `make check` |
| goreleaser | v2 | `go install github.com/goreleaser/goreleaser/v2@latest` — required for `make release-local` / `make release` |
| EVE Developer App | — | [developers.eveonline.com](https://developers.eveonline.com/) |

## Quick Start

### 1. Register a EVE Developer Application

Go to [https://developers.eveonline.com/](https://developers.eveonline.com/) and create a new application with the following settings:

- **Connection Type:** Authentication & API Access
- **Callback URL:** `http://localhost:8080/auth/eve/callback`
- **Scopes:**
  - `esi-characters.read_blueprints.v1`
  - `esi-corporations.read_blueprints.v1`
  - `esi-industry.read_character_jobs.v1`
  - `esi-industry.read_corporation_jobs.v1`

Copy the **Client ID** and **Secret Key** — you will need them in the next step.

### 2. Create the config file

```bash
cp auspex.example.yaml auspex.yaml
```

Edit `auspex.yaml` and fill in your credentials:

```yaml
esi:
  client_id: "your-client-id"
  client_secret: "your-client-secret"
  callback_url: "http://localhost:8080/auth/eve/callback"
```

### 3. Build

```bash
make build
```

Runs `npm run build` → `sqlc generate` → `go build` in the correct order and produces an `auspex` (or `auspex.exe`) binary in the project root.

> **Windows:** `make` is not included with Windows by default and must be installed separately.

### 4. Run

```bash
./auspex          # macOS/Linux
auspex.exe        # Windows
```

The server starts on port 8080 by default. Open your browser and go to:

```
http://localhost:8080
```

### 5. Add a character

Navigate to:

```
http://localhost:8080/auth/eve/login
```

Complete the EVE SSO flow. After a successful login, Auspex immediately triggers a sync and redirects you to the dashboard.

Repeat for each character.

## Configuration

All settings live in `auspex.yaml` (see `auspex.example.yaml` for a commented template).

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `port` | integer | `8080` | TCP port the HTTP server listens on |
| `db_path` | string | `auspex.db` | Path to the SQLite database file |
| `refresh_interval` | integer | `10` | Background sync interval, in minutes |
| `esi.client_id` | string | — | EVE SSO Client ID (required) |
| `esi.client_secret` | string | — | EVE SSO Client Secret (required) |
| `esi.callback_url` | string | — | OAuth2 callback URL (required); must match the Developer App setting |

The config file path can be overridden with the `-config` flag:

```bash
./auspex -config /path/to/custom.yaml
```

## Development

### Running the frontend dev server

The Vite dev server proxies `/api` and `/auth` to the Go backend, so you can work on the frontend with hot module replacement while the backend serves real data.

```bash
# Start the backend (builds frontend once to satisfy go:embed, then runs)
./auspex

# In a separate terminal, start the frontend dev server
cd cmd/auspex/web
npm install
npm run dev
```

Open `http://localhost:5173` — changes to `.jsx` / `.css` files hot-reload instantly.

### Running tests

```bash
go test ./...
```

### Regenerating the store after schema changes

```bash
sqlc generate
```

Run this after any change to `internal/db/migrations/` or `internal/db/queries/`.

### Build order

The correct build order is enforced by the Makefile:

1. `npm run build` — produces `cmd/auspex/web/dist/` (embedded into the binary)
2. `sqlc generate` — regenerates `internal/store/` from SQL queries
3. `go build ./cmd/auspex/` — compiles the binary with the embedded frontend

## Project Structure

```
cmd/auspex/          # main entry point; embeds web/dist
cmd/auspex/web/      # React frontend (Vite, TanStack Table)
internal/
  config/            # CLI flags + config file → typed Config struct
  db/                # SQLite init and up-only migrations
  store/             # sqlc-generated typed query functions
  esi/               # ESI HTTP client with retry and cache-header parsing
  auth/              # EVE SSO OAuth2 flow and token auto-refresh
  sync/              # Background worker; coordinates esi + store
  api/               # Chi router and HTTP handlers
docs/                # Architecture, requirements, task breakdown, tech debt
Makefile             # build, test, lint, clean targets
```

See [docs/project-structure.md](docs/project-structure.md) for a detailed description of every file.

## Architecture Notes

- **API handlers never call ESI directly.** All ESI traffic goes through the background sync worker. Handlers only read from SQLite — UI responses are always instant regardless of ESI availability.
- **Polymorphic ownership:** blueprints and jobs use `owner_type` (`'character'` | `'corporation'`) + `owner_id` instead of separate FK columns.
- **Lazy universe resolution:** `eve_types`, `eve_groups`, and `eve_categories` are populated on first encounter and never re-fetched.

See [docs/architecture.md](docs/architecture.md) for the full architecture description.

## License

This project is not affiliated with CCP Games. EVE Online and all related marks are property of CCP hf.
