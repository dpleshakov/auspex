# 2026-03-04-tasks-owner-management.md

**Status:** Active

### Contracts

No new API endpoints or schema changes. Uses existing endpoints:
- `DELETE /api/characters/{id}` — already implemented
- `GET /api/corporations` — already implemented
- `POST /api/corporations` — already implemented (request: `{ "corporation_id": int, "delegate_id": int }`)
- `DELETE /api/corporations/{id}` — already implemented

API client helpers `deleteCharacter(id)` and `deleteCorporation(id)` in `client.js` already exist but are not called from the UI.

---

### TASK-01 `delete-ui`
**Type:** Regular
**Description:** Add UI controls for removing tracked characters and corporations. The backend `DELETE /api/characters/{id}` and `DELETE /api/corporations/{id}` endpoints are fully implemented (cascade-delete blueprints, jobs, sync_state, then the owner row; return 204). The API client helpers `deleteCharacter(id)` and `deleteCorporation(id)` in `client.js` also exist but are never called. What is missing: a delete button per row in `CharactersSection` (currently display-only); a corporations management section (`CorporationsSection`) with a per-row delete action. After a successful delete, both sections should reload their data and trigger `postSync()` to keep the blueprints table consistent.
**Definition of done:** working code + tests + committed
**Status:** ⬜ Pending

### TASK-02 `add-corporation-form`
**Type:** Regular
**Description:** Add an add-corporation form to `CorporationsSection` (Corp ID, name, delegate character dropdown) so corporations can be added from the UI. The `POST /api/corporations` backend endpoint is implemented. Root cause of the current bug: OAuth callback saves only the character; `runCycle` → `ListCorporations` returns `[]` → corp blueprint sync loop never executes. Fix: add the form, and after a successful add call `postSync()` so blueprints appear without waiting for the next background tick.
**Definition of done:** working code + tests + committed
**Status:** ⬜ Pending

### TASK-03 `review`
**Type:** Review
**Covers:** TASK-01, TASK-02
**Status:** ⬜ Pending

### TASK-04 `docs`
**Type:** Docs
**Status:** ⬜ Pending
