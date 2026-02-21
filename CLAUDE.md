# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**Auspex** is a local desktop tool for EVE Online industry players who manage multiple manufacturing characters. It pulls data from the EVE ESI API via OAuth2 and presents a unified dashboard of BPO library and research job status across all characters and corporations.

Delivered as a **single Go binary** with the React frontend embedded via `go:embed`. Users run the binary and open `localhost:PORT` in their browser. No external dependencies — no Docker, no PostgreSQL, no Redis. SQLite database stored as a single file next to the binary.

## Development Process

Development follows a strict phase-by-phase process. Do not skip phases or jump ahead. Each phase has defined inputs and outputs — the output of one phase is the input of the next.

**Current active phase: Phase 5 — Project Structure**

### Phase Overview

| # | Phase | Input | Output |
|---|-------|-------|--------|
| 1 | Discovery | Raw idea | `idea.md` |
| 2 | Requirements | `idea.md` | `requirements.md` |
| 3 | Tech Stack | `requirements.md` | `tech-stack.md` |
| 4 | Architecture | `requirements.md` + `tech-stack.md` | `architecture.md` |
| **5** | **Project Structure** ← current | `architecture.md` | File structure + `project-structure.md` |
| 6 | Task Breakdown | `requirements.md` + `architecture.md` | `tasks.md` |
| 7 | Iterative Development | Task + context | Working, committed code |
| 8 | Review & Refactor | Completed modules | Improved code + `tech-debt.md` |
| 9 | Documentation | Finished product | `README.md`, `api-docs.md`, `deployment.md` |

---

### Phase 1 — Discovery

**Input:** raw idea

**What to do:**
In conversation with AI, articulate what the product is, who it's for, what problem it solves, and how it differs from existing solutions. AI asks clarifying questions, helps identify weak spots and constraints.

**Output:** `idea.md`
- Problem description
- Proposed solution
- Target audience
- Key success metrics
- Risks and constraints

---

### Phase 2 — Requirements

**Input:** `idea.md`

**What to do:**
Together with AI, write out functional and non-functional requirements. Create a list of user stories or use cases. Define the MVP — what is mandatory, what can be cut.

**Output:** `requirements.md`
- Feature list with priorities (must / should / could)
- User stories
- Non-functional requirements (performance, security, scalability)
- Constraints (platform, audience, budget)
- Clear MVP boundary

---

### Phase 3 — Tech Stack

**Input:** `requirements.md`

**What to do:**
Discuss stack options with AI, taking into account requirements, your skills, and constraints. Justify the choice of each tool, consider alternatives and risks.

**Output:** `tech-stack.md`
- Selected technologies with rationale
- Considered alternatives and reasons for rejection
- Known stack risks
- Links to documentation, tool versions

---

### Phase 4 — Architecture

**Input:** `requirements.md` + `tech-stack.md`

**What to do:**
Design the high-level architecture — modules, their responsibilities, and interactions. Describe the database schema, API contracts (at least the main endpoints), and data flows.

**Output:** `architecture.md`
- Diagrams (Mermaid or ASCII)
- Module descriptions and responsibilities
- Database schema
- Main API contracts (endpoints, methods, formats)
- Data flows between components

---

### Phase 5 — Project Structure ← CURRENT

**Input:** `architecture.md`

**What to do:**
Ask AI to generate the project file structure. Create a skeleton — directories, empty files, base configs, `.env.example`.

**Output:**
- Actual file structure in the repository
- `project-structure.md` — description of the purpose of each directory/file
- Base configs (linter, formatter, `.gitignore`, `.env.example`)

---

### Phase 6 — Task Breakdown

**Input:** `requirements.md` + `architecture.md`

**What to do:**
Break the project into tasks — small, atomic, each resulting in working code. Each task should be completable in one AI conversation (30–100 lines of code). Set dependencies and order.

**Output:** `tasks.md` or tasks in a tracker
- Task list with descriptions
- Definition of done for each task
- Dependencies between tasks
- Execution order

---

### Phase 7 — Iterative Development (main loop)

Each task goes through a mini-cycle:

**Input:** specific task + relevant context (required files, API contracts)

**What to do:**
1. Give AI the task + context (not the entire project, only what's needed)
2. Receive code, understand it
3. Run and test manually
4. If something is wrong — correct in conversation
5. Commit working code

**Output:**
- Working, committed code
- Updated `tasks.md` (mark completed tasks)

---

### Phase 8 — Review & Refactor

**Input:** completed modules or finished features

**What to do:**
Ask AI to do a code review — find security, performance, and readability issues. Refactor before the next major feature, not at the end.

**Output:**
- Improved, committed code
- `tech-debt.md` — list of known issues and deferred decisions

---

### Phase 9 — Documentation

**Input:** finished product or module

**What to do:**
AI helps write the README, API documentation, and startup/deployment instructions.

**Output:**
- `README.md` — project description, quick start
- `api-docs.md` — API documentation
- `deployment.md` — deployment instructions

---

### Core Principles for Working with AI

**Context is everything.** AI does not remember past sessions. Documents from previous phases are your shared memory. Pass the relevant files at the start of each conversation.

**Small tasks beat large ones.** One task = one conversation. "Build the entire backend" works worse than "implement the `POST /users` endpoint with validation and database write".

**Never accept code blindly.** Understand what AI generates — otherwise you'll be helpless when the first bug appears.

**Commit often.** After each working task — commit. This makes it easy to roll back if AI "helped" break something that was working.

**Refactor before, not after.** Don't accumulate technical debt until the end of the project. A small review after each module is better than a big one at the end.

---

## Expected Commands

Once implemented, the standard workflow will be:

```bash
# Full build (frontend → sqlc → Go binary)
scripts/build.sh      # macOS/Linux
scripts/build.cmd     # Windows

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

Build order matters: `npm run build` → `sqlc generate` → `go build`. The build scripts enforce this. `sqlc generate` must be re-run after any change to `internal/db/migrations/` or `internal/db/queries/`.

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
- `jobs` table only stores active/ready jobs; completed/cancelled jobs from ESI are ignored

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Backend language | Go 1.22+ |
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
