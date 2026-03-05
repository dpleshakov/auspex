# 2026-03-04-tasks-product-category.md

**Status:** Active

### Contracts

**`GET /api/blueprints` response change:**
- Fields `category_id` and `category_name` currently reflect the blueprint item's own category (always "Blueprint"). After the fix they must reflect the category of the *produced* item.
- New field `product_type_id` (integer) added to the response to expose the product type for reference.

**DB schema:** no new tables required — the existing `eve_types` → `eve_groups` → `eve_categories` chain is used. The `blueprints` table may need a `product_type_id` column to cache the product type per blueprint.

---

### TASK-01 `product-category`
**Type:** Regular
**Description:** In the Category column show the category of the item *produced* by the blueprint, not the category of the blueprint item itself (which is always "Blueprint"). Requires looking up `product_type_id` for each BPO (available from ESI blueprint data or `GET /universe/types/{id}` `produced_by` field) and resolving its category chain via `eve_types` → `eve_groups` → `eve_categories`. Steps: add `product_type_id` to `blueprints` table (migration), populate during sync, update `ListBlueprints` query to join on the product type's category chain, update `GET /api/blueprints` response, update `BlueprintTable` component to render the product category.
**Definition of done:** working code + tests + committed
**Status:** ⬜ Pending

### TASK-02 `review`
**Type:** Review
**Covers:** TASK-01
**Description:**
- Code: security, error handling, readability, obvious performance issues
- Security: input validation, no tokens in logs, errors do not expose internal details, dependency vulnerability check
- Documentation: verify `technical-reference.md` matches reality — update if not; verify `architecture.md` — update if needed
**Status:** ⬜ Pending

### TASK-03 `docs`
**Type:** Docs
**Description:**
- Update user-facing documentation (README, help, guides) if behaviour visible to the user has changed
- Verify `technical-reference.md` is up to date — all API and schema changes introduced by this feature must be reflected
- Update `CHANGELOG.md` — only user-visible changes, following the format in `process-changelog.md`
**Status:** ⬜ Pending
