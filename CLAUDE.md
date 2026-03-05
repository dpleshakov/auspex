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

## Commands

```bash
# Before every commit — both must pass.
make build     # generate, compile, test
make lint      # npm audit, go mod tidy, golangci-lint

# Full CI-equivalent check — requires a clean worktree (all changes committed).
make check

# Build release distribution (goreleaser snapshot).
make release

# During development — quick feedback
make test                                    # Go tests only
go build ./cmd/auspex/                         # run the server locally
cd cmd/auspex/web && npm run dev             # frontend dev server with HMR (proxies /api and /auth to :8080)
```

`make build` runs in order: `sqlc generate`, `npm ci && npm run build`, `go vet`, `go test`, `go build`.
Re-run after any change to `internal/db/migrations/` or `internal/db/queries/`.

---

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

For data flow, database schema, tech stack, MVP scope, and ESI endpoints — see [`docs/architecture.md`](docs/architecture.md) and [`docs/technical-reference.md`](docs/technical-reference.md).
