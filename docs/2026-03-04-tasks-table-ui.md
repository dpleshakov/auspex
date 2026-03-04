# 2026-03-04-tasks-table-ui.md

**Status:** Active

### Contracts

No API or schema changes. Both tasks are frontend-only.

---

### TASK-01 `locale-dates`
**Type:** Regular
**Description:** Display all dates in the blueprints table (e.g. research end date) using the browser's regional locale format (`toLocaleDateString` / `toLocaleString`) instead of a hard-coded ISO/UTC string, so the format matches the user's OS locale automatically.
**Definition of done:** working code + committed
**Status:** ⬜ Pending

### TASK-02 `null-dates-sort`
**Type:** Regular
**Description:** When sorting by a date column, always treat a missing date (displayed as `—` in the UI) as the lowest priority regardless of sort direction — i.e. rows with no date must always appear at the bottom of the table, whether the sort is ascending or descending. Currently null/empty dates sort to the top on descending sort. Fix in the TanStack Table `sortingFn` for date columns.
**Definition of done:** working code + committed
**Status:** ⬜ Pending

### TASK-03 `review`
**Type:** Review
**Covers:** TASK-01, TASK-02
**Status:** ⬜ Pending

### TASK-04 `docs`
**Type:** Docs
**Status:** ⬜ Pending
