# 2026-03-06-tasks-table-ui.md

**Status:** Archived

### Contracts

No API schema changes. One API field renamed, one removed. All other changes are frontend-only.

**`GET /api/jobs/summary` response changes:**
- Field `overdue_jobs` renamed to `ready_jobs` — counts all jobs with `status = ready` (previously counted only `status = ready AND end_date < now`)
- Field `completing_today` removed

---

### TASK-01 `ready-jobs-summary`
**Type:** Regular
**Description:** Fix the summary bar count for completed jobs and remove the obsolete "completing today" count.

Backend:
- Rename `CountOverdueJobs` → `CountReadyJobs` in `internal/db/queries/jobs.sql`; update query to `WHERE status = 'ready'` (drop the `end_date < datetime('now')` condition)
- Remove `CountCompletingToday` from `internal/db/queries/jobs.sql`
- Regenerate store with `sqlc generate`
- Update `handleGetJobsSummary` in `internal/api/blueprints.go`: rename field `overdue_jobs` → `ready_jobs`, remove `completing_today` field
- Update `summaryJSON` struct accordingly
- Update `technical-reference.md` to reflect the renamed and removed fields

Frontend:
- Update `SummaryBar` component: rename `overdue_jobs` → `ready_jobs`, remove "Completing today" tile

**Definition of done:** working code + tests + committed
**Status:** ✅ Done

---

### TASK-02 `row-highlighting`
**Type:** Regular
**Description:** Rework row highlighting logic in `BlueprintTable` to match the new semantics.

Rules:
- **Red** — `job.status === 'ready'` (job is done, needs collection). No date comparison.
- **Yellow** — `job.status === 'active'` AND `end_date <= now + 24h`. The 24-hour threshold is hardcoded as a named constant `ALARM_HOURS = 24`.

Remove `isToday()` function — no longer needed.

Rewrite `getFullStatusLabel()` and `getRowClass()` accordingly. The label "Overdue" is removed; `status = ready` is displayed as "Ready" regardless of date.

**Definition of done:** working code + committed
**Status:** ✅ Done

---

### TASK-03 `date-format`
**Type:** Regular
**Description:** Fix AM/PM display in all date cells and standardise date formatting.

- Add `hour12: false` to `formatLocalDate()` in `BlueprintTable.jsx` to force 24-hour output regardless of OS locale
- Dates are displayed in UTC (browser's `new Date(isoStr)` already parses UTC correctly; no timezone conversion is applied)
- `formatLocalDate` is the single place for date formatting — no inline `toLocaleString` calls elsewhere

**Definition of done:** working code + committed
**Status:** ✅ Done

---

### TASK-04 `null-dates-sort`
**Type:** Regular
**Description:** When sorting by a date column, rows without a date (Idle blueprints, displayed as `—`) must always appear at the bottom of the table regardless of sort direction.

Fix in the TanStack Table `sortingFn` for the `end_date` column: treat `null`/`undefined`/empty values as lowest priority in both ascending and descending sort.

**Definition of done:** working code + committed
**Status:** ✅ Done

---

### TASK-05 `review`
**Type:** Review
**Covers:** TASK-01, TASK-02, TASK-03, TASK-04
**Description:**
- Code: security, error handling, readability, obvious performance issues
- Security: input validation, no tokens in logs, errors do not expose internal details, dependency vulnerability check
- Documentation: verify `technical-reference.md` matches reality — update if not; verify `architecture.md` — update if needed
**Status:** ✅ Done

---

### TASK-06 `docs`
**Type:** Docs
**Description:**
- Update user-facing documentation (README, help, guides) if behaviour visible to the user has changed
- Verify `technical-reference.md` is up to date — all API and schema changes introduced by this feature must be reflected
- Update `CHANGELOG.md` — only user-visible changes, following the format in `process-changelog.md`
**Status:** ✅ Done

---

## Future configuration options

The following behaviour is hardcoded in this feature and is a candidate for `auspex.yaml` in a future settings feature:

- `alarm_hours: 24` — threshold in hours for yellow row highlighting ("completing soon")
- `display_timezone: utc` — whether dates are shown in UTC or the browser's local timezone (`utc` | `local`)
