# Auspex — tech-debt.md

Known issues and deferred decisions. Updated after each layer review.

---

## Layer 1 Review — Foundation (config, db, store)

### TD-01 `flag.Parse()` inside `config.Load()` — ✅ Fixed

**File:** `internal/config/config.go`

`Load()` called `flag.Parse()` internally, preventing the caller from
registering flags beforehand — an anti-pattern for library code.

**Fix:** `Load()` changed to `Load(path string)`. Flag registration and
`flag.Parse()` moved to `main.go`. Config package no longer imports `flag`.

---

### TD-02 `esi.callback_url` not validated as a URL — ✅ Fixed

**File:** `internal/config/config.go`

`validate()` only checked that `callback_url` was non-empty.

**Fix:** `url.Parse()` added; validation rejects any value whose scheme is
not `http` or `https`. Test `TestLoadFromFile_InvalidCallbackURL` added.

---

---

## Layer 2 Review — ESI HTTP Client

### TD-04 No pagination for blueprints and jobs endpoints — ⏭ Won't fix (MVP)

**Files:** `internal/esi/blueprints.go`, `internal/esi/jobs.go`

ESI pagination endpoints (`/characters/{id}/blueprints`, `/corporations/{id}/blueprints`,
`/characters/{id}/industry/jobs`, `/corporations/{id}/industry/jobs`) support multiple
pages via `?page=N` and signal the total page count via the `X-Pages` response header.
The current client fetches only the first page.

Characters or corporations with very large BPO libraries (typically large corporations)
will have data silently truncated — no error is returned, only the first page of results
is stored.

**Not a problem for MVP** — personal characters and small/medium corps fit within a single
page. Must be addressed before supporting large fleet-scale corporations. Fix: after each
successful response, check `X-Pages`; if `> 1`, fetch remaining pages and concatenate.

---

### TD-05 Response body size not limited — ⏭ Won't fix (MVP)

**File:** `internal/esi/client.go`

`io.ReadAll(resp.Body)` reads the entire response body without an upper bound.
A misbehaving server (or network interception) could return an arbitrarily large body,
causing unbounded memory growth.

**Not a problem for MVP** — all requests go to the official ESI API over HTTPS, which
returns bounded, well-formed JSON. Fix if auspex is ever extended to support third-party
or user-configurable API endpoints: wrap with `io.LimitReader(resp.Body, maxBodyBytes)`.

---

### TD-03 SQLite single-connection limitation — ⏭ Won't fix (MVP)

**File:** `internal/db/db.go`

`SetMaxOpenConns(1)` limits the database to one concurrent connection. This
is necessary because `PRAGMA foreign_keys` is a per-connection setting and
SQLite does not support concurrent writes. For a local single-user desktop
tool this is correct. If Auspex were ever deployed as a multi-user server,
this would need to be revisited (move to PostgreSQL or use a WAL-tuned
SQLite setup with proper connection-level PRAGMA hooks).

**Not a problem for MVP.**

---

## Layer 3 Review — Auth (oauth.go, client.go)

### TD-06 `callVerify`: status check after `io.ReadAll` — ✅ Fixed

**File:** `internal/auth/oauth.go`

The original code called `io.ReadAll(resp.Body)` before checking
`resp.StatusCode`. A misbehaving server returning a large error body would
exhaust memory before the status check could reject it. Additionally, the
raw error body was interpolated into the error message, which could expose
sensitive content in logs.

**Fix:** Status check moved before `io.ReadAll`. Error message for non-200
responses no longer includes the response body (only the status code).
Test `TestCallVerify_NonOKStatus` continues to pass unchanged.

---

### TD-07 `Provider.states` map grows without bound — ⏭ Won't fix (MVP)

**File:** `internal/auth/oauth.go`

Every call to `GenerateAuthURL()` adds an entry to `p.states`. Abandoned
OAuth sessions (user opens login URL but never completes the flow) are never
evicted. For a local desktop tool with one user and infrequent logins this
is inconsequential. Fix if needed: add a TTL-based eviction (store
`time.Time` alongside the state and sweep entries older than N minutes in
a background goroutine).

**Not a problem for MVP.**

---

### TD-08 `callVerify` did not validate `CharacterID > 0` — ✅ Fixed

**File:** `internal/auth/oauth.go`

A malformed or empty response from EVE SSO (e.g. `{"CharacterID": 0}`)
would be stored silently, corrupting the `characters` table with an invalid
ID=0 row.

**Fix:** Added validation `v.CharacterID <= 0 → error` after JSON parsing.
Test `TestCallVerify_ZeroCharacterID` added.

---

### TD-09 `tokenForCharacter` issues a store read on every ESI call — ⏭ Won't fix (MVP)

**File:** `internal/auth/client.go`

Each ESI call (blueprints, jobs) triggers a `store.GetCharacter` round-trip
to SQLite to retrieve the current token. A single sync cycle for one
character makes 2 such reads (blueprints + jobs), and for a corporation 4
reads (2 character queries + 2 corporation→delegate queries). With SQLite
on a local disk this is fast, but at scale (many characters, frequent
syncs) it adds up.

**Not a problem for MVP.** Fix if needed: cache the token in memory with
a short TTL (shorter than the access token lifetime) to avoid repeated
store reads within a single sync cycle.

---

## Layer 4 Review — Sync Worker (worker.go)

### TD-10 `syncBlueprints` does not prune stale rows — ⏭ Won't fix (MVP)

**File:** `internal/sync/worker.go`

`syncBlueprints` only upserts incoming blueprints; it never removes rows
for blueprints that have left the owner's ESI response (sold, destroyed,
moved to an untracked owner). By contrast, `syncJobs` explicitly compares
incoming job IDs against stored IDs and deletes the difference.

Because `UpsertBlueprint` uses `ON CONFLICT(id) DO UPDATE SET owner_*`,
a blueprint transferred to another *tracked* owner is updated correctly —
the problem only arises when the item moves to an untracked owner or is
destroyed. In those cases the row remains indefinitely with the original
owner and shows as "Idle" on the dashboard.

**Not a problem for MVP** — blueprint transfers are uncommon, and the item
is at most displayed as a phantom "Idle" entry. Fix when needed: mirror the
`syncJobs` approach — collect the set of incoming `item_id`s, compare
against `ListBlueprintTypeIDsByOwner` (or a dedicated
`ListBlueprintIDsByOwner`), and delete the difference.

---

### TD-11 `resolveTypeIDs` makes N sequential ESI calls — ⏭ Won't fix (MVP)

**File:** `internal/sync/worker.go`

For each unknown `type_id`, `resolveTypeIDs` calls `esi.GetUniverseType`
synchronously. The sync worker is single-threaded per cycle, so on a
character's first sync with 200 unique BPO types, the worker is blocked for
200 sequential network round-trips before it can move on to the next subject.

ESI offers `POST /universe/names/` for bulk resolution (up to 1000 IDs per
request), but it was not implemented in TASK-06. During the initial sync of
a large corp library this can take tens of seconds to a few minutes.

**Not a problem for MVP** — personal characters and small/medium corps have
far fewer unique BPO types, and the delay is one-time. Fix before supporting
large fleet-scale corporations: collect all unknown `type_id`s across all
owners, batch them into `POST /universe/names/` requests, then upsert
results. Alternatively, resolve concurrently with a bounded goroutine pool.

---

### TD-12 Blueprints with unresolved `type_id` silently excluded from dashboard — ⏭ Won't fix (MVP)

**File:** `internal/db/queries/blueprints.sql`

`ListBlueprints` uses `JOIN eve_types t ON t.id = b.type_id`. If
`resolveTypeIDs` fails or is interrupted for a particular `type_id`
(e.g. ESI 404, DB error, context cancellation), no row exists in
`eve_types` for that ID. The blueprint is silently excluded from all
query results with no error or warning.

**Not a problem for MVP** — `resolveTypeIDs` errors are already logged,
and the retry on the next tick will usually succeed. Fix if needed: change
to `LEFT JOIN eve_types` and use `COALESCE(t.name, 'Unknown Type ' || b.type_id)`
as the fallback name so the blueprint always appears on the dashboard.

---

## Layer 5 Review — API (router, handlers)

### TD-13 `writeJSON` did not set `Content-Type` — ✅ Fixed

**File:** `internal/api/response.go`

`writeJSON` relied on the `jsonContentType` middleware to set
`Content-Type: application/json`, but that middleware is only applied to
the `/api/*` route group. Error responses from `/auth/eve/login` and
`/auth/eve/callback` were sent as JSON without the correct header.

**Fix:** `w.Header().Set("Content-Type", "application/json")` moved into
`writeJSON` itself, before `w.WriteHeader(status)`. The `jsonContentType`
middleware is kept for documentation intent and is now a no-op (idempotent
`Header.Set`). Test `TestHandleLogin_500WhenGenerateFails` extended to
assert the header is present.

---

### TD-14 Cascade delete is non-atomic — ⏭ Won't fix (MVP)

**Files:** `internal/api/characters.go`, `internal/api/corporations.go`

`handleDeleteCharacter` and `handleDeleteCorporation` each perform four
consecutive `store.Querier` calls (delete blueprints → jobs → sync_state →
entity) with no wrapping transaction. A crash or context cancellation
between any two calls leaves the database in a partially deleted state:
for example, blueprints deleted but the character row still present, or
vice-versa.

**Not a problem for MVP** — the app is a local single-user desktop tool
with SQLite. A mid-delete crash is rare and the worst outcome is cosmetic
(a phantom character with no data, or orphaned data rows). Fix before
exposing delete endpoints to concurrent or networked use: wrap the four
calls in a `sql.Tx` and surface a `WithTx(*sql.Tx) store.Querier` method
from the store package so handlers can participate in transactions.

---

### TD-15 Duplicate corporation insert returns 500 instead of 409 — ⏭ Won't fix (MVP)

**File:** `internal/api/corporations.go`

`InsertCorporation` uses `INSERT OR IGNORE` (or similar), so inserting a
corporation with an `id` that already exists silently succeeds. If the
underlying SQL used `INSERT` without `OR IGNORE`, a unique constraint
violation would propagate as a generic 500. The current schema and sqlc
query should be verified; if the handler ever surfaces constraint errors,
they should be mapped to 409 Conflict rather than 500.

**Not a problem for MVP** — re-adding a known corporation is an uncommon
user action and the current behaviour is at worst confusing.

---

### TD-16 EVE SSO user-cancel produces unhelpful 400 — ⏭ Won't fix (MVP)

**File:** `internal/api/oauth.go`

When the user clicks "Cancel" on the EVE SSO authorization page, EVE
redirects to the callback URL with `?error=access_denied` (no `code`
parameter). The current handler checks `if code == "" || state == ""` and
returns 400 "missing code or state" — technically correct but unhelpful.

**Not a problem for MVP** — the user sees the browser's raw 400 page
for a moment before the tab can be closed. Fix: check `req.URL.Query().Get("error")`
first and redirect to `/?auth_error=canceled` so the frontend can show a
friendly message.

---

### TD-17 CORS wildcard applies to auth endpoints — ⏭ Won't fix (MVP)

**File:** `internal/api/router.go`

The `corsMiddleware` sets `Access-Control-Allow-Origin: *` globally,
including on `/auth/eve/login` and `/auth/eve/callback`. In theory this
allows any website open in the same browser to initiate or interfere with
the OAuth flow. In practice the risk is negligible: the OAuth callback
validates a random `state` parameter (CSRF mitigation), and the app only
listens on `localhost`.

**Not a problem for MVP** — this is a local desktop tool. Fix if the
app is ever exposed to a non-localhost network: restrict the allowed origin
to the frontend origin (e.g. `http://localhost:5173` in dev, `http://localhost:PORT`
in production), or remove the global CORS middleware and apply it only to
`/api/*`.

---

## Layer 7 Review — Frontend (React)

### TD-18 Sticky blueprint table header overlaps sticky app header — ✅ Fixed

**File:** `cmd/auspex/web/src/index.css`

`.bp-table__th` had `position: sticky; top: 0`, which caused the table header
to scroll behind (and visually under) the sticky `.app-header` when the user
scrolled down. The app header has `z-index: 10`, so the table header was hidden
rather than overlapping on top, but it was effectively invisible at the stuck
position.

**Fix:** Changed `top: 0` to `top: 51px` (approximate rendered height of
`.app-header`: button ~28px content + 2×10px padding + 1px border). Added
`z-index: 1` so the table header renders above table rows during horizontal
scroll. If the header height changes (e.g. after a UI redesign), this value
must be updated manually.

---

### TD-19 Force-refresh poll runs indefinitely when sync_state is empty — ✅ Fixed

**File:** `cmd/auspex/web/src/App.jsx`

`handleRefresh()` starts a `setInterval` that polls `GET /api/sync/status` and
stops when it finds a `last_sync` timestamp newer than when Refresh was clicked.
If no characters are added yet, `sync_state` is empty and the API returns `[]`.
`statuses.some(...)` is always false, so the interval never clears and
`isRefreshing` stays `true` forever — the button shows "Refreshing…" until
page reload.

**Fix:** Added a `SYNC_POLL_MAX_MS = 60_000` deadline. If no completion signal
arrives within 60 seconds, the poll clears itself, calls `loadData()`, and resets
`isRefreshing`. This covers: no characters added, persistent ESI errors, or any
other case where sync never writes a fresh `last_sync`.

---

### TD-20 Dual-fetch dead code in leaf components — ⏭ Won't fix (MVP)

**Files:** `src/components/SummaryBar.jsx`, `src/components/CharactersSection.jsx`,
`src/components/BlueprintTable.jsx`

Each component was built in two phases: first with internal data fetching
(TASK-21–23), then adapted to accept props from `App` (TASK-26). The internal
fetch paths (`getJobsSummary()` / `getBlueprints()` called inside `useEffect`)
remain in the code but are unreachable in the current wiring — `App` always
passes non-`undefined` props, so the `if (externalXxx !== undefined)` branch
always fires immediately.

The dead code adds ~15 lines per component and two redundant `useState` pairs
(`loading`, `error`) that are set but then immediately overwritten.

**Not a problem for MVP** — the dead paths are harmless and the components
still work correctly. Fix in post-MVP cleanup: remove internal fetch logic,
accept data purely via props, lift error/loading state entirely into `App`.

---

### TD-21 Location column shows raw numeric ESI `location_id` — ⏭ Won't fix (MVP)

**File:** `src/components/BlueprintTable.jsx`

The "Location" column renders the raw `location_id` integer from the API response
(e.g. `60003760`). Resolving this to a human-readable station or structure name
requires `POST /universe/names/` bulk resolution — the same pattern as type
resolution, but location IDs change more frequently (structures can be destroyed
or renamed).

**Not a problem for MVP** — experienced EVE players recognize common station IDs.
Fix post-MVP: add a `locations` table and a background resolver similar to
`resolveTypeIDs`, then join in `ListBlueprints` query.

---

### TD-22 esbuild moderate vulnerability in dev dependency — ⏭ Won't fix (MVP)

**File:** `cmd/auspex/web/package.json` (transitively via `vite`)

`npm audit` reports a moderate-severity vulnerability in `esbuild ≤ 0.24.2`
(GHSA-67mh-4wv8-2f99): the esbuild dev server can be made to proxy arbitrary
requests from any website open in the same browser. The fix requires upgrading
to `vite@7` (breaking change).

**Not a problem for MVP** — the vulnerability affects only the Vite dev server
(`npm run dev`), not the production build embedded in the binary. End users
never run the dev server. Fix before onboarding external contributors: upgrade
to `vite@7` and resolve any breaking changes in the Vite config.

---

### TD-23 Uncovered test cases — ⏭ Won't fix (MVP)

The following scenarios are not covered by tests. All of them are second-order
edge cases; critical paths and happy paths are fully tested.

**`internal/api/blueprints_test.go`**
- Valid numeric `owner_id` and `category_id` are only checked for invalid format
  (400), but not that a valid value is propagated into
  `ListBlueprintsParams.OwnerID/CategoryID`.
- A combination of multiple filters applied simultaneously is not tested.

**`internal/api/characters_test.go`, `corporations_test.go`**
- `DELETE /api/characters/{id}` and `DELETE /api/corporations/{id}` with a
  non-existent ID are not covered — it is unclear whether the handler returns
  404 or 204 (No Content).

**`internal/sync/sync_subject_test.go`**
- ESI returns an empty jobs list (`[]`) — in this case all existing jobs in the
  DB for the given owner should be deleted. A test for the partially-stale case
  exists, but an empty list is not tested.

**`internal/config/config_test.go`**
- A syntactically invalid YAML file (not semantically invalid, but truly
  malformed YAML). A parse error is expected, but the behaviour is not specified
  by a test.

**Not a problem for MVP** — all described cases are either handled correctly by
default (standard library errors) or affect rare scenarios. Fix when next
touching these handlers: add targeted test cases.
