# tasks-backlog.md

> Status: Active
> Updated: 23.02.2026

Bugs and small improvements that don't belong to a module scope.
For deferred and acknowledged issues see `tech-debt.md`.

Prefixes: BUG — something broken, CHORE — small tweak or cleanup, IMPROVEMENT — small enhancement.

---

## Open

<!-- Add new items here. Format:
- [ ] PREFIX — description (noticed DD.MM.YYYY)
-->

- [ ] IMPROVEMENT — Show solar system name instead of raw location ID in the blueprints table location
  column. Requires resolving `location_id` via ESI `POST /universe/names/` (bulk) or
  `GET /universe/systems/{system_id}` and caching results in a new `eve_locations` table
  (similar to the existing lazy-resolution pattern for `eve_types`). (noticed 23.02.2026)

- [ ] IMPROVEMENT — Display all dates in the blueprints table (e.g. research end date) using the
  browser's regional locale format (`toLocaleDateString` / `toLocaleString`) instead of a hard-coded
  ISO/UTC string, so the format matches the user's OS locale automatically. (noticed 23.02.2026)

- [ ] IMPROVEMENT — When sorting by a date column, always treat a missing date (displayed as `—` in the
  UI) as the lowest priority regardless of sort direction — i.e. rows with no date must always appear
  at the bottom of the table, whether the sort is ascending or descending. Currently null/empty dates
  sort to the top on descending sort. Fix in the TanStack Table `sortingFn` for date columns.
  (noticed 23.02.2026)

- [ ] IMPROVEMENT — Add UI controls for removing tracked characters and corporations. The backend
  `DELETE /api/characters/{id}` and `DELETE /api/corporations/{id}` endpoints are fully implemented
  (cascade-delete blueprints, jobs, sync_state, then the owner row; return 204). The API client
  helpers `deleteCharacter(id)` and `deleteCorporation(id)` in `client.js` also exist but are never
  called. What is missing:
  - A delete button per row in `CharactersSection` (currently display-only).
  - A corporations management section (`CorporationsSection`) with a per-row delete action and,
    ideally, an add-corporation form (Corp ID, name, delegate character dropdown) so corporations
    can be both added and removed from the UI.
  After a successful delete, both sections should reload their data and trigger `postSync()` to
  keep the blueprints table consistent. (noticed 23.02.2026)

- [ ] IMPROVEMENT — In the Category column show the category of the item *produced* by the blueprint,
  not the category of the blueprint item itself (which is always "Blueprint"). Requires looking up
  `product_type_id` for each BPO (available from ESI blueprint data or `GET /universe/types/{id}`
  `produced_by` field) and resolving its category chain via `eve_types` → `eve_groups` →
  `eve_categories`. (noticed 23.02.2026)

- [ ] BUG — Corporation blueprints never appear in the table: corporations are never inserted into
  the database because no UI exists to add them. The sync worker correctly iterates `ListCorporations`
  and the `POST /api/corporations` backend endpoint is implemented, but there is no frontend surface
  to call it. Root cause: OAuth callback → only character saved; `runCycle` → `ListCorporations`
  returns `[]` → corp blueprint sync loop never executes. Fix: add `CorporationsSection` component
  with a tracked-corporations table (delete) and an add-corporation form (Corp ID, name, delegate
  character dropdown); after a successful add call `postSync()` so blueprints appear without waiting
  for the next background tick. (noticed 23.02.2026)

---

## Closed

<!-- Items move here when fixed. Format:
- [x] PREFIX — description — fixed commit {hash} (DD.MM.YYYY)
-->
