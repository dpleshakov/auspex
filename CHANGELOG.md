# Changelog

All notable changes to Auspex are documented here.

---

## [0.1.0] — 2026-02-23

Initial MVP release.

### Added

**Layer 1 — Foundation**
- `internal/config`: config loader reading from YAML file and CLI flags; validates required ESI credentials and callback URL format (c9b1725)
- `internal/db`: SQLite connection via `modernc.org/sqlite` (pure Go, no CGO); up-only schema migrations applied automatically at startup (e89b1d7)
- `internal/store`: complete database schema (`blueprints`, `jobs`, `characters`, `corporations`, `sync_state`, `eve_types`, `eve_groups`, `eve_categories`); sqlc-generated typed query functions (c64fc94)

**Layer 2 — ESI HTTP Client**
- `internal/esi`: base HTTP client with `Authorization` header injection, `Expires` response header parsing into `cache_until`, retry on 429 with `Retry-After`, and exponential backoff on 5xx (2ff10f9)
- `internal/esi`: endpoints for character and corporation blueprints and industry jobs; BPCs and non-active/ready jobs filtered out at the client layer (e13ed37)
- `internal/esi`: endpoints for EVE universe reference data — `/universe/types/{id}`, `/universe/groups/{id}`, `/universe/categories/{id}` (e820dff)

**Layer 3 — Auth**
- `internal/auth`: EVE SSO OAuth2 Authorization Code flow; state parameter generation and validation (CSRF protection); character verification via `/verify` endpoint; token and character saved to SQLite on successful callback (7ae5bd3)
- `internal/auth`: automatic access token refresh; `auth.Client` wraps `esi.Client` and injects a fresh token before every ESI call; transparent to the sync worker (1877334)

**Layer 4 — Sync Worker**
- `internal/sync`: background goroutine with configurable ticker; iterates all characters and corporations, checks ESI cache expiry per subject per endpoint, skips fresh subjects (74cb4d9)
- `internal/sync`: force-refresh channel — `Worker.ForceRefresh()` bypasses cache expiry and triggers an immediate full sync cycle (74cb4d9)
- `internal/sync`: full sync cycle — fetches blueprints and jobs from ESI, upserts to store, removes stale job rows, updates `sync_state` with ESI `Expires` header (f367d4a)
- `internal/sync`: lazy universe type resolution — after each sync, unknown `type_id`s are resolved via ESI and inserted into `eve_types`, `eve_groups`, `eve_categories` (068a7dc)

**Layer 5 — API**
- `internal/api`: Chi router with Logger, Recoverer, and CORS middleware; embedded React SPA served on all non-API routes with `index.html` fallback (551022d)
- `internal/api`: character management endpoints — `GET /api/characters`, `DELETE /api/characters/{id}` with cascade delete of all associated data (c54451c)
- `internal/api`: corporation management endpoints — `GET /api/corporations`, `POST /api/corporations` (validates delegate character), `DELETE /api/corporations/{id}` (c54451c)
- `internal/api`: blueprint library endpoint — `GET /api/blueprints` with optional filters by `status`, `owner_type`, `owner_id`, `category_id`; each blueprint includes a nested `job` object or `null` (d1d5bca)
- `internal/api`: jobs summary endpoint — `GET /api/jobs/summary` returning idle blueprint count, overdue job count, completing-today count, and per-character slot usage (d1d5bca)
- `internal/api`: sync control endpoints — `POST /api/sync` (202 Accepted immediately), `GET /api/sync/status` with owner names (e8c198a)
- `internal/api`: EVE SSO OAuth2 HTTP handlers — `GET /auth/eve/login`, `GET /auth/eve/callback`; successful callback triggers immediate sync and redirects to frontend (a76f324)

**Layer 6 — Main + Build**
- `cmd/auspex/main.go`: application entry point wiring all packages; graceful shutdown on SIGINT/SIGTERM — drains HTTP requests (10s), waits for sync worker to finish current cycle (60441ef)
- `auspex.example.yaml`: documented configuration template with all fields and required EVE Developer App scopes (60441ef)
- `scripts/build.sh`, `scripts/build.cmd`: full build scripts for macOS/Linux and Windows; enforce `npm build → sqlc generate → go build` order; fail clearly if any step fails (5d71a4b)

**Layer 7 — Frontend**
- React + Vite project scaffold in `cmd/auspex/web/`; Vite dev proxy for `/api` and `/auth`; `src/api/client.js` with typed fetch wrappers for all backend endpoints (f2d520e)
- `SummaryBar` component: aggregate counts from `GET /api/jobs/summary`; overdue count highlighted red, idle count highlighted when non-zero (1ef5773)
- `CharactersSection` component: per-character slot usage table; rows highlighted when free slots are available (501a4ef)
- `BlueprintTable` component: BPO table using TanStack Table v8; all columns rendered with human-readable status (Idle / ME Research / TE Research / Copying / Ready) (ca373e6)
- `BlueprintTable` sorting and filtering: default sort by priority (Overdue → Ready → Idle → Active) then `end_date`; filter dropdowns for status, owner, category; "Clear filters" button (e87a1fd)
- `BlueprintTable` row highlighting: red for overdue jobs (`status = ready` + `end_date < now`), yellow for jobs completing today (`status = active` + `end_date` is today) (5d0560f)
- `App` component: assembles all components; auto-polling every 10 minutes; force-refresh flow with "Refreshing…" indicator and 60-second timeout guard (6caff01)

### Fixed (during layer reviews)

- `config`: `flag.Parse()` moved out of `config.Load()` to `main.go` (anti-pattern for library code) — TD-01
- `config`: `callback_url` validated as a proper HTTP/HTTPS URL, not just non-empty — TD-02
- `auth/oauth.go`: HTTP status check moved before `io.ReadAll` in `callVerify`; error message no longer includes raw response body — TD-06
- `auth/oauth.go`: `callVerify` now validates `CharacterID > 0` to reject malformed SSO responses — TD-08
- `api/response.go`: `Content-Type: application/json` moved into `writeJSON` itself so auth error responses also carry the correct header — TD-13
- `index.css`: blueprint table sticky header `top` offset corrected to `51px` to account for the sticky app header height — TD-18
- `App.jsx`: force-refresh poll now has a 60-second deadline to prevent infinite polling when no characters are added or sync never completes — TD-19

### Known Limitations

- ESI pagination not implemented — only the first page of blueprints/jobs is fetched; large corporation libraries may be silently truncated (TD-04)
- Location IDs displayed as raw integers; human-readable station/structure names are a post-MVP feature (TD-21)
- `free_research_slots` is always `0`; requires per-character skill data from ESI (not in MVP scope)
- Leaf components (`SummaryBar`, `CharactersSection`, `BlueprintTable`) retain unreachable internal-fetch code paths from their initial implementation (TD-20)

See `docs/tech-debt.md` for the full list with rationale and fix guidance.
