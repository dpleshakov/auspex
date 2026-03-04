# Auspex — tech-debt.md

Known issues and deferred decisions.

---

### Active

#### TD-03 `SQLite single-connection limitation`
- Problem: `SetMaxOpenConns(1)` limits the database to one concurrent connection. This is necessary because `PRAGMA foreign_keys` is a per-connection setting and SQLite does not support concurrent writes. For a local single-user desktop tool this is correct. If Auspex were ever deployed as a multi-user server, this would need to be revisited (move to PostgreSQL or use a WAL-tuned SQLite setup with proper connection-level PRAGMA hooks).
- Why deferred: Not a problem for MVP.
- Trigger: Multi-user or server deployment scenario.
- File: `internal/db/db.go`
- Added: 2026-02-26

#### TD-04 `No pagination for blueprints and jobs endpoints`
- Problem: ESI pagination endpoints (`/characters/{id}/blueprints`, `/corporations/{id}/blueprints`, `/characters/{id}/industry/jobs`, `/corporations/{id}/industry/jobs`) support multiple pages via `?page=N` and signal the total page count via the `X-Pages` response header. The current client fetches only the first page. Characters or corporations with very large BPO libraries (typically large corporations) will have data silently truncated — no error is returned, only the first page of results is stored.
- Why deferred: Not a problem for MVP — personal characters and small/medium corps fit within a single page.
- Trigger: Supporting large fleet-scale corporations. Fix: after each successful response, check `X-Pages`; if `> 1`, fetch remaining pages and concatenate.
- Files: `internal/esi/blueprints.go`, `internal/esi/jobs.go`
- Added: 2026-02-26

#### TD-05 `Response body size not limited`
- Problem: `io.ReadAll(resp.Body)` reads the entire response body without an upper bound. A misbehaving server (or network interception) could return an arbitrarily large body, causing unbounded memory growth.
- Why deferred: Not a problem for MVP — all requests go to the official ESI API over HTTPS, which returns bounded, well-formed JSON.
- Trigger: Extension to support third-party or user-configurable API endpoints. Fix: wrap with `io.LimitReader(resp.Body, maxBodyBytes)`.
- File: `internal/esi/client.go`
- Added: 2026-02-26

#### TD-07 `Provider.states map grows without bound`
- Problem: Every call to `GenerateAuthURL()` adds an entry to `p.states`. Abandoned OAuth sessions (user opens login URL but never completes the flow) are never evicted. For a local desktop tool with one user and infrequent logins this is inconsequential.
- Why deferred: Not a problem for MVP.
- Trigger: High-frequency or automated login scenarios. Fix: add a TTL-based eviction (store `time.Time` alongside the state and sweep entries older than N minutes in a background goroutine).
- File: `internal/auth/oauth.go`
- Added: 2026-02-27

#### TD-09 `tokenForCharacter issues a store read on every ESI call`
- Problem: Each ESI call (blueprints, jobs) triggers a `store.GetCharacter` round-trip to SQLite to retrieve the current token. A single sync cycle for one character makes 2 such reads (blueprints + jobs), and for a corporation 4 reads (2 character queries + 2 corporation→delegate queries). With SQLite on a local disk this is fast, but at scale (many characters, frequent syncs) it adds up.
- Why deferred: Not a problem for MVP.
- Trigger: Many characters with frequent syncs. Fix: cache the token in memory with a short TTL (shorter than the access token lifetime) to avoid repeated store reads within a single sync cycle.
- File: `internal/auth/client.go`
- Added: 2026-02-27

#### TD-10 `syncBlueprints does not prune stale rows`
- Problem: `syncBlueprints` only upserts incoming blueprints; it never removes rows for blueprints that have left the owner's ESI response (sold, destroyed, moved to an untracked owner). By contrast, `syncJobs` explicitly compares incoming job IDs against stored IDs and deletes the difference. A blueprint transferred to an untracked owner or destroyed remains in the DB indefinitely and shows as "Idle" on the dashboard.
- Why deferred: Not a problem for MVP — blueprint transfers are uncommon, and the item is at most displayed as a phantom "Idle" entry.
- Trigger: Blueprint sold/destroyed/transferred to untracked owner. Fix: mirror the `syncJobs` approach — collect the set of incoming `item_id`s, compare against stored IDs, and delete the difference.
- File: `internal/sync/worker.go`
- Added: 2026-02-28

#### TD-11 `resolveTypeIDs makes N sequential ESI calls`
- Problem: For each unknown `type_id`, `resolveTypeIDs` calls `esi.GetUniverseType` synchronously. The sync worker is single-threaded per cycle, so on a character's first sync with 200 unique BPO types, the worker is blocked for 200 sequential network round-trips before it can move on to the next subject. During the initial sync of a large corp library this can take tens of seconds to a few minutes.
- Why deferred: Not a problem for MVP — personal characters and small/medium corps have far fewer unique BPO types, and the delay is one-time.
- Trigger: Large fleet-scale corporations. Fix: collect all unknown `type_id`s across all owners, batch them into `POST /universe/names/` requests, then upsert results. Alternatively, resolve concurrently with a bounded goroutine pool.
- File: `internal/sync/worker.go`
- Added: 2026-02-28

#### TD-12 `Blueprints with unresolved type_id silently excluded from dashboard`
- Problem: `ListBlueprints` uses `JOIN eve_types t ON t.id = b.type_id`. If `resolveTypeIDs` fails or is interrupted for a particular `type_id` (e.g. ESI 404, DB error, context cancellation), no row exists in `eve_types` for that ID. The blueprint is silently excluded from all query results with no error or warning.
- Why deferred: Not a problem for MVP — `resolveTypeIDs` errors are already logged, and the retry on the next tick will usually succeed.
- Trigger: Persistent ESI errors for specific type IDs. Fix: change to `LEFT JOIN eve_types` and use `COALESCE(t.name, 'Unknown Type ' || b.type_id)` as the fallback name so the blueprint always appears on the dashboard.
- File: `internal/db/queries/blueprints.sql`
- Added: 2026-02-28

#### TD-14 `Cascade delete is non-atomic`
- Problem: `handleDeleteCharacter` and `handleDeleteCorporation` each perform four consecutive `store.Querier` calls (delete blueprints → jobs → sync_state → entity) with no wrapping transaction. A crash or context cancellation between any two calls leaves the database in a partially deleted state.
- Why deferred: Not a problem for MVP — the app is a local single-user desktop tool with SQLite. A mid-delete crash is rare and the worst outcome is cosmetic.
- Trigger: Exposing delete endpoints to concurrent or networked use. Fix: wrap the four calls in a `sql.Tx` and surface a `WithTx(*sql.Tx) store.Querier` method from the store package so handlers can participate in transactions.
- Files: `internal/api/characters.go`, `internal/api/corporations.go`
- Added: 2026-03-01

#### TD-15 `Duplicate corporation insert returns 500 instead of 409`
- Problem: If the underlying SQL uses `INSERT` without `OR IGNORE`, a unique constraint violation would propagate as a generic 500. The current schema and sqlc query should be verified; if the handler ever surfaces constraint errors, they should be mapped to 409 Conflict rather than 500.
- Why deferred: Not a problem for MVP — re-adding a known corporation is an uncommon user action.
- Trigger: User attempts to add a corporation that is already tracked. Fix: detect unique constraint errors and return 409 Conflict.
- File: `internal/api/corporations.go`
- Added: 2026-03-01

#### TD-16 `EVE SSO user-cancel produces unhelpful 400`
- Problem: When the user clicks "Cancel" on the EVE SSO authorization page, EVE redirects to the callback URL with `?error=access_denied` (no `code` parameter). The current handler checks `if code == "" || state == ""` and returns 400 "missing code or state" — technically correct but unhelpful.
- Why deferred: Not a problem for MVP — the user sees the browser's raw 400 page for a moment before the tab can be closed.
- Trigger: User-facing polish pass. Fix: check `req.URL.Query().Get("error")` first and redirect to `/?auth_error=canceled` so the frontend can show a friendly message.
- File: `internal/api/oauth.go`
- Added: 2026-03-01

#### TD-17 `CORS wildcard applies to auth endpoints`
- Problem: The `corsMiddleware` sets `Access-Control-Allow-Origin: *` globally, including on `/auth/eve/login` and `/auth/eve/callback`. In theory this allows any website open in the same browser to initiate or interfere with the OAuth flow. In practice the risk is negligible: the OAuth callback validates a random `state` parameter (CSRF mitigation), and the app only listens on `localhost`.
- Why deferred: Not a problem for MVP — this is a local desktop tool.
- Trigger: Exposing the app to a non-localhost network. Fix: restrict the allowed origin to the frontend origin, or remove the global CORS middleware and apply it only to `/api/*`.
- File: `internal/api/router.go`
- Added: 2026-03-01

#### TD-20 `Dual-fetch dead code in leaf components`
- Problem: Each component was built in two phases: first with internal data fetching (TASK-21–23), then adapted to accept props from `App` (TASK-26). The internal fetch paths remain in the code but are unreachable in the current wiring. The dead code adds ~15 lines per component and two redundant `useState` pairs (`loading`, `error`) that are set but then immediately overwritten.
- Why deferred: Not a problem for MVP — the dead paths are harmless and the components still work correctly.
- Trigger: Post-MVP cleanup. Fix: remove internal fetch logic, accept data purely via props, lift error/loading state entirely into `App`.
- Files: `src/components/SummaryBar.jsx`, `src/components/CharactersSection.jsx`, `src/components/BlueprintTable.jsx`
- Added: 2026-03-03

#### TD-21 `Location column shows raw numeric ESI location_id`
- Problem: The "Location" column renders the raw `location_id` integer from the API response (e.g. `60003760`). Resolving this to a human-readable station or structure name requires `POST /universe/names/` bulk resolution — the same pattern as type resolution, but location IDs change more frequently (structures can be destroyed or renamed).
- Why deferred: Not a problem for MVP — experienced EVE players recognize common station IDs.
- Trigger: Post-MVP UX polish. Fix: add a `locations` table and a background resolver similar to `resolveTypeIDs`, then join in `ListBlueprints` query.
- File: `src/components/BlueprintTable.jsx`
- Added: 2026-03-03

#### TD-22 `esbuild moderate vulnerability in dev dependency`
- Problem: `npm audit` reports a moderate-severity vulnerability in `esbuild ≤ 0.24.2` (GHSA-67mh-4wv8-2f99): the esbuild dev server can be made to proxy arbitrary requests from any website open in the same browser. The fix requires upgrading to `vite@7` (breaking change).
- Why deferred: Not a problem for MVP — the vulnerability affects only the Vite dev server (`npm run dev`), not the production build embedded in the binary. End users never run the dev server.
- Trigger: Onboarding external contributors. Fix: upgrade to `vite@7` and resolve any breaking changes in the Vite config.
- File: `cmd/auspex/web/package.json` (transitively via `vite`)
- Added: 2026-03-03

#### TD-23 `Uncovered test cases`
- Problem: The following scenarios are not covered by tests. All of them are second-order edge cases; critical paths and happy paths are fully tested. `internal/api/blueprints_test.go`: valid numeric `owner_id` and `category_id` propagation not tested; combination of multiple filters not tested. `internal/api/characters_test.go`, `corporations_test.go`: `DELETE` with a non-existent ID not covered — unclear whether handler returns 404 or 204. `internal/sync/sync_subject_test.go`: ESI returning an empty jobs list not tested. `internal/config/config_test.go`: syntactically invalid YAML file not tested.
- Why deferred: Not a problem for MVP — all described cases are either handled correctly by default or affect rare scenarios.
- Trigger: Next time these handlers are touched. Fix: add targeted test cases for each scenario described above.
- Added: 2026-03-03

---

### Closed

#### TD-01 `flag.Parse() inside config.Load()`
- Fixed: 2026-02-25
- `Load()` called `flag.Parse()` internally, preventing the caller from registering flags beforehand. Fix: `Load()` changed to `Load(path string)`; flag registration and `flag.Parse()` moved to `main.go`.
- File: `internal/config/config.go`

#### TD-02 `esi.callback_url not validated as a URL`
- Fixed: 2026-02-25
- `validate()` only checked that `callback_url` was non-empty. Fix: `url.Parse()` added; validation rejects any value whose scheme is not `http` or `https`. Test `TestLoadFromFile_InvalidCallbackURL` added.
- File: `internal/config/config.go`

#### TD-06 `callVerify: status check after io.ReadAll`
- Fixed: 2026-02-27
- The original code called `io.ReadAll(resp.Body)` before checking `resp.StatusCode`. A misbehaving server returning a large error body would exhaust memory before the status check could reject it. Additionally, the raw error body was interpolated into the error message, which could expose sensitive content in logs. Fix: status check moved before `io.ReadAll`. Error message for non-200 responses no longer includes the response body (only the status code).
- File: `internal/auth/oauth.go`

#### TD-08 `callVerify did not validate CharacterID > 0`
- Fixed: 2026-02-27
- A malformed or empty response from EVE SSO (e.g. `{"CharacterID": 0}`) would be stored silently, corrupting the `characters` table with an invalid ID=0 row. Fix: added validation `v.CharacterID <= 0 → error` after JSON parsing. Test `TestCallVerify_ZeroCharacterID` added.
- File: `internal/auth/oauth.go`

#### TD-13 `writeJSON did not set Content-Type`
- Fixed: 2026-03-01
- `writeJSON` relied on the `jsonContentType` middleware to set `Content-Type: application/json`, but that middleware is only applied to the `/api/*` route group. Error responses from `/auth/eve/login` and `/auth/eve/callback` were sent as JSON without the correct header. Fix: `w.Header().Set("Content-Type", "application/json")` moved into `writeJSON` itself, before `w.WriteHeader(status)`.
- File: `internal/api/response.go`

#### TD-18 `Sticky blueprint table header overlaps sticky app header`
- Fixed: 2026-03-04 (previous fix reverted — introduced regression)
- `.bp-table__th` has `position: sticky; top: 0` and `z-index: 1`. The `z-index` ensures the header renders above cells during horizontal scroll. `top: 0` is correct: because `.bp-table-wrapper` has `overflow-x: auto`, it becomes the sticky scroll container for `<th>` per CSS spec. The wrapper does not scroll vertically, so sticky never activates and the element behaves as `position: relative; top: 0` (no offset). Changing `top` to `51px` caused a regression — the header was permanently shifted 51px down into data rows. The scenario of the header conflicting with `.app-header` does not occur in this layout.
- Note: sticky table header does not actually activate during page scroll due to `overflow-x: auto` on the wrapper. If true sticky-header behaviour is wanted, the layout needs to be restructured (e.g. make the wrapper height-constrained and scroll vertically, or hoist `overflow-x` to a higher container).
- File: `cmd/auspex/web/src/index.css`

#### TD-19 `Force-refresh poll runs indefinitely when sync_state is empty`
- Fixed: 2026-03-03
- `handleRefresh()` starts a `setInterval` that polls `GET /api/sync/status` and stops when it finds a `last_sync` timestamp newer than when Refresh was clicked. If no characters are added yet, `sync_state` is empty and the API returns `[]`. `statuses.some(...)` is always false, so the interval never clears and `isRefreshing` stays `true` forever. Fix: added a `SYNC_POLL_MAX_MS = 60_000` deadline. If no completion signal arrives within 60 seconds, the poll clears itself, calls `loadData()`, and resets `isRefreshing`.
- File: `cmd/auspex/web/src/App.jsx`
