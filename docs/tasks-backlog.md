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

---

## Closed

<!-- Items move here when fixed. Format:
- [x] PREFIX — description — fixed commit {hash} (DD.MM.YYYY)
-->

- [x] BUG — Corporation blueprints never appeared in the table: corporations were never inserted into
  the database because no UI existed for it. The sync worker correctly iterates `ListCorporations`
  and the `POST /api/corporations` backend endpoint was already implemented, but there was no
  frontend surface to call it. Root cause confirmed by tracing the full chain:
  OAuth callback → only character saved; `runCycle` → `ListCorporations` returns `[]` → corp
  blueprint sync loop never executes. Fix: added `CorporationsSection` component with a tracked-
  corporations table (delete) and an add-corporation form (Corp ID, name, delegate character
  dropdown); after a successful add the component calls `postSync()` so blueprints appear without
  waiting for the next background tick — fixed commit TBD (23.02.2026)
