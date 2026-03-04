# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**Auspex** is a local desktop tool for EVE Online industry players who manage multiple manufacturing characters. It pulls data from the EVE ESI API via OAuth2 and presents a unified dashboard of BPO library and research job status across all characters and corporations.

Delivered as a **single Go binary** with the React frontend embedded via `go:embed`. Users run the binary and open `localhost:PORT` in their browser. No external dependencies — no Docker, no PostgreSQL, no Redis. SQLite database stored as a single file next to the binary.

## Development Processes

| Document | Purpose |
|----------|---------|
| [`docs/process-project-start.md`](docs/process-project-start.md) | Starting a brand-new project from scratch. Runs once; produces the foundational documents (`project-brief.md`, `architecture.md`, repository skeleton). Not used in ongoing development. |
| [`docs/process-feature.md`](docs/process-feature.md) | Adding any new feature — from the first MVP tasks to all subsequent post-MVP modules. This is the main development loop. |
| [`docs/process-maintenance.md`](docs/process-maintenance.md) | Working with tech debt (`tech-debt.md`) and general principles for effective AI-assisted development. |
| [`docs/process-changelog.md`](docs/process-changelog.md) | Format and rules for maintaining `CHANGELOG.md`. |

---

## Language Standard

**All code must be in English.** This applies without exception to:
- Identifiers: variable names, function names, type names, constants, package names
- Comments (both inline and doc comments)
- Commit messages
- Documentation files (`*.md`)
- SQL schema, table names, column names
- API field names, error messages, log messages

The only permitted exceptions are EVE Online proper nouns (ship names, item names, etc.) that appear as data values, not as identifiers.

---

## Expected Commands

Once implemented, the standard workflow will be:

```bash
# Run before committing — lint + tests + full build must all pass
make check

# Full build (frontend → sqlc → Go binary)
make build            # macOS/Linux; on Windows requires make to be installed separately

# Frontend only (in cmd/auspex/web/)
npm install
npm run dev           # dev server with HMR, proxies /api and /auth to localhost:8080
npm run build         # produces dist/ that gets embedded into the Go binary

# Backend only
sqlc generate         # regenerate internal/store/ after schema or query changes
go build -o auspex ./cmd/auspex/
go run ./cmd/auspex/

# Run a specific Go test
go test ./internal/esi/...
go test ./internal/sync/... -run TestSyncWorker
```

Build order matters: `npm run build` → `sqlc generate` → `go build`. The Makefile enforces this. `sqlc generate` must be re-run after any change to `internal/db/migrations/` or `internal/db/queries/`.

## Architecture

### Go Package Layout

```
cmd/auspex/          # main entry point — wires everything together; embeds web/dist
cmd/auspex/web/      # React frontend (Vite, TanStack Table); lives here so //go:embed works
internal/
  config/            # CLI flags + config file → typed Config struct
  db/                # SQLite init, schema migrations (up-only), provides *sql.DB
  store/             # sqlc-generated typed query functions (CRUD only, no business logic)
  esi/               # ESI HTTP client — fetches data, returns typed structs, respects cache headers
  auth/              # EVE SSO OAuth2 flow; wraps esi with auto-refreshing token injection
  sync/              # background worker + scheduler; coordinates auth/esi + store
  api/               # Chi router, HTTP handlers, serves embedded static files
```

### Key Architectural Constraints

**API handlers never call ESI directly.** All ESI traffic goes through the `sync` background worker. Handlers only read from SQLite. This keeps UI response instant regardless of ESI availability.

**`store` is only imported by `sync` and `api`** — not by `esi` or `auth`. This keeps the data access layer isolated.

**Polymorphic ownership**: `blueprints` and `jobs` tables use `owner_type` (`'character'` | `'corporation'`) + `owner_id` instead of separate FK columns. Integrity is enforced at the application layer.

**Lazy EVE universe resolution**: `eve_types`, `eve_groups`, `eve_categories` are populated on first encounter with a new `type_id` and never re-fetched — this data is stable.

### Data Flow

- **OAuth add character**: `/auth/eve/login` → EVE SSO → `/auth/eve/callback` → store token → trigger immediate sync → redirect to frontend
- **Background sync** (ticker every N minutes): for each character/corp, check `sync_state.cache_until`, skip if still fresh, otherwise fetch from ESI, upsert to SQLite, resolve unknown `type_id`s, update `sync_state`
- **Force refresh**: `POST /api/sync` sends signal to sync worker via channel → 202 immediately; frontend polls `GET /api/sync/status` every 2s watching `last_sync` timestamp
- **Frontend reads**: always hits SQLite via API handlers; polling interval matches the configured auto-refresh interval

### Database Schema Key Points

- `sync_state` table tracks ESI cache expiry per `(owner_type, owner_id, endpoint)` — the sync worker reads `Expires` response header and writes it here
- Blueprint `status` is derived, not stored: a blueprint is `idle` if it has no associated job in the `jobs` table with `status IN ('active', 'ready')`
- `jobs` table only stores active/ready jobs; completed/canceled jobs from ESI are ignored

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Backend language | Go 1.26+ |
| HTTP router | Chi v5 |
| Database | SQLite via `modernc.org/sqlite` (pure Go, no CGO) |
| SQL codegen | sqlc v2 |
| OAuth2 | `golang.org/x/oauth2` |
| Frontend framework | React 18 |
| Build tool | Vite |
| Table component | TanStack Table v8 |
| Styling | Plain CSS (no framework) |

**Why `modernc.org/sqlite` over `mattn/go-sqlite3`**: avoids CGO, enabling cross-platform single-binary compilation without a C toolchain per target platform.

## MVP Scope

MVP = BPO library + research slot monitoring only.

**In MVP**: ESI OAuth, multi-character + multi-corp support, BPO table (Name, Category, ME%, TE%, status, owner, location, end date), summary row (idle/overdue/completing today/free slots), visual highlights (red = overdue, yellow = completing today), sort/filter, auto-refresh + manual refresh.

**Post-MVP**: manufacturing slot monitoring, BPC library, profitability analytics, mineral tracking, Wails desktop wrapper, external alerts.

## ESI API Endpoints Used

- `GET /characters/{id}/blueprints`
- `GET /characters/{id}/industry/jobs`
- `GET /corporations/{id}/blueprints`
- `GET /corporations/{id}/industry/jobs`
- `GET /universe/types/{type_id}`
- `POST /universe/names/` (bulk resolve)
- `GET /verify` (character verification after OAuth)

ESI reference: https://esi.evetech.net/ui/
EVE SSO docs: https://developers.eveonline.com/blog/article/sso-to-authenticated-calls
