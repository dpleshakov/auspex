# 2026-03-05-tasks-characters-page.md

**Status:** Archived

### Contracts

**DB schema additions:**

```sql
-- Migration: add corporation_id to characters
ALTER TABLE characters ADD COLUMN corporation_id INTEGER NOT NULL DEFAULT 0;

-- Migration: add corporation_name to characters
ALTER TABLE characters ADD COLUMN corporation_name TEXT NOT NULL DEFAULT '';

-- Migration: add last_error to sync_state
ALTER TABLE sync_state ADD COLUMN last_error TEXT;
```

**`GET /api/characters` response change:**

```json
[
  {
    "id": 12345678,
    "name": "My Character",
    "corporation_id": 98765432,
    "corporation_name": "Center for Advanced Studies",
    "is_delegate": true,
    "sync_error": null,
    "created_at": "2026-02-21T10:00:00Z"
  }
]
```

| Field | Type | Description |
|-------|------|-------------|
| `corporation_id` | integer | EVE corporation ID the character belongs to |
| `corporation_name` | string | EVE corporation name (stored at character save time; used for NPC corporations that are not in the `corporations` table) |
| `is_delegate` | boolean | Whether this character is the delegate for its corporation |
| `sync_error` | string or `null` | Last sync error for this character's corporation (only when `is_delegate = true` and last sync failed); `null` otherwise |

**`DELETE /api/characters/{id}` behaviour change:**
- If the deleted character is the last member of a player corporation (no other characters share the same `corporation_id`) — delete the corporation and all its data (blueprints, jobs, sync_state) before deleting the character.
- If other characters share the same `corporation_id` and the deleted character is the current delegate — reassign `delegate_id` to the first remaining character from that corporation.
- NPC corporations (ID in range 1000000–2000000) are never added to the `corporations` table, so no corporation logic applies.

**New endpoint:**

```
PATCH /api/corporations/{id}/delegate
Body: { "character_id": 12345678 }
```

| Status | Description |
|--------|-------------|
| `204 No Content` | Delegate updated |
| `400 Bad Request` | `character_id` missing, or character does not belong to this corporation |
| `404 Not Found` | Corporation not found |
| `500 Internal Server Error` | Database error |

---

### TASK-01 `characters-schema`
**Type:** Regular
**Description:** Add migration adding `corporation_id INTEGER NOT NULL DEFAULT 0` and `corporation_name TEXT NOT NULL DEFAULT ''` to the `characters` table, and `last_error TEXT` to `sync_state`. Update the OAuth callback (`internal/auth/oauth.go`) to populate `corporation_id` and `corporation_name` from the `/verify` response when saving a character. Update sqlc queries: `CreateCharacter` / `UpsertCharacter` to include `corporation_id` and `corporation_name`; add `ListCharactersByCorporation(corporation_id)` for use in delete logic; add `UpdateSyncStateError(owner_type, owner_id, endpoint, error)` for recording sync failures.
**Definition of done:** working code + tests + committed
**Status:** ✅ Done

### TASK-02 `auto-add-corporation`
**Type:** Regular
**Description:** In the OAuth callback, after saving a character: if `corporation_id` is outside the NPC range (1000000–2000000), insert the corporation into the `corporations` table with this character as delegate (`INSERT OR IGNORE` — if the corporation already exists, leave the existing delegate unchanged). Then trigger an immediate sync for the corporation. Update `internal/auth/oauth.go` and the relevant store queries.
**Definition of done:** working code + tests + committed
**Status:** ✅ Done

### TASK-03 `delete-character-logic`
**Type:** Regular
**Description:** Update `DELETE /api/characters/{id}` in `internal/api/characters.go`. Before deleting the character: (1) count other characters sharing the same `corporation_id`; (2) if count is zero and the corporation exists in the `corporations` table — delete the corporation's blueprints, jobs, sync_state, and the corporation row first; (3) if count is greater than zero and the character is the current delegate — reassign `delegate_id` to the first other character from `ListCharactersByCorporation`. Then delete the character's own blueprints, jobs, sync_state, and the character row.
**Definition of done:** working code + tests + committed
**Status:** ✅ Done

### TASK-04 `delegate-endpoint`
**Type:** Regular
**Description:** Implement `PATCH /api/corporations/{id}/delegate` in `internal/api/corporations.go`. Validate that the corporation exists and that the supplied `character_id` belongs to the corporation (via `corporation_id` field on the character row). Update `delegate_id`. Return 400 if the character does not belong to this corporation, 404 if the corporation is not found.
**Definition of done:** working code + tests + committed
**Status:** ✅ Done

### TASK-05 `characters-api-response`
**Type:** Regular
**Description:** Update `GET /api/characters` handler and the underlying store query. Add to each character in the response: `corporation_id` and `corporation_name` (from the characters table); `is_delegate` (true if a row in `corporations` has `delegate_id = character.id`); `sync_error` (value of `sync_state.last_error` for `owner_type = 'corporation'` and `owner_id = character.corporation_id` where `character.id = corporations.delegate_id`, otherwise `null`). Update `internal/sync/worker.go` to write the error message to `sync_state.last_error` on a failed sync and clear it on success.
**Definition of done:** working code + tests + committed
**Status:** ✅ Done

### TASK-06 `review-backend`
**Type:** Review
**Covers:** TASK-01, TASK-02, TASK-03, TASK-04, TASK-05
**Description:**
- Code: security, error handling, readability, obvious performance issues
- Security: input validation, no tokens in logs, errors do not expose internal details, dependency vulnerability check
- Documentation: verify `technical-reference.md` matches reality — update if not; verify `architecture.md` — update if needed
**Status:** ✅ Done

### TASK-07 `characters-page`
**Type:** Regular
**Description:** Implement the Characters tab content in `App.jsx` (replacing the placeholder added in `tab-navigation`). Extract into a new `CharactersPage` component. The component fetches `GET /api/characters` and `GET /api/corporations`, then groups characters by `corporation_id`. For each player corporation group: corporation name as the group header (from corporations response); each character row shows name, delegate indicator (● if `is_delegate`, otherwise ○ — clicking ○ calls `PATCH /api/corporations/{id}/delegate` and reloads), a `⚠ no access` warning next to the delegate indicator if `sync_error != null`, and a [Delete] button. For each NPC corporation group (ID in range 1000000–2000000): use `corporation_name` from the character data as the group header (NPC corporations are not in the `GET /api/corporations` response); character rows show only name and [Delete] button — no delegate indicator. Add `patchDelegate(corporationId, characterId)` to `cmd/auspex/web/src/api/client.js`. [+ Add character] button at the bottom linking to `/auth/eve/login`.
**Definition of done:** working code + committed
**Status:** ✅ Done

### TASK-08 `delete-confirmation`
**Type:** Regular
**Description:** Add a confirmation dialog to the [Delete] button on `CharactersPage`. Default message: "Delete character X? All their data will be removed." When the character is the last one in a player corporation: "Delete character X? This is the last character in corporation Y — the corporation and all its blueprints will also be deleted." Determine which message to show client-side: check if any other character in the loaded data shares the same `corporation_id`. On confirmation, call `DELETE /api/characters/{id}` and reload the page data.
**Definition of done:** working code + committed
**Status:** ✅ Done

### TASK-09 `review-frontend`
**Type:** Review
**Covers:** TASK-07, TASK-08
**Description:**
- Code: security, error handling, readability, obvious performance issues
- Security: input validation, no tokens in logs, errors do not expose internal details, dependency vulnerability check
- Documentation: verify `technical-reference.md` matches reality — update if not; verify `architecture.md` — update if needed
**Status:** ✅ Done

### TASK-10 `docs`
**Type:** Docs
**Description:**
- Update user-facing documentation (README, help, guides) if behaviour visible to the user has changed
- Verify `technical-reference.md` is up to date — all API and schema changes introduced by this feature must be reflected
- Update `CHANGELOG.md` — only user-visible changes, following the format in `process-changelog.md`
**Status:** ✅ Done

### TASK-11 `characters-page-polish`
**Type:** Regular
**Description:** Polish the Characters page layout and usability. Changes:
- Remove the `+ Add character` button at the bottom of the list; keep only the one in the top-right corner. When no characters have been added yet, show a centered empty state with a single `+ Add character` button and a short explanation (e.g. "No characters added yet. Add a character to get started.").
- Replace the delegate dot indicator with a filled circle ● + text label `Delegate` for the delegate character (non-interactive). For all other characters in the same player corporation group, show an empty circle ○ + a `Make delegate` button — clicking it calls `PATCH /api/corporations/{id}/delegate` and reloads the page.
- Add two columns to each character row: blueprint count (number of blueprints owned by this character, from already loaded blueprint data or a dedicated query) and last sync time (from `GET /api/sync/status` for this character).
- Improve corporation group visual separation: add a bottom margin between groups and a subtle divider line so groups are clearly distinct when there are multiple corporations.
**Definition of done:** working code + committed
**Status:** ✅ Done

### TASK-12 `characters-page-polish-2`
**Type:** Regular
**Description:** Further polish the Characters page. Changes:
- Add a table header row above the character list with column labels: `Name`, `Role`, `Blueprints`, `Last Sync`. Header row should be visually distinct (e.g. smaller, muted color, uppercase).
- Make corporation group headers more visually prominent: increase font size, apply the amber/gold accent color (matching the active tab style), and add sufficient top margin before each group to clearly separate it from the previous one.
**Definition of done:** working code + committed
**Status:** ⬜ Pending
