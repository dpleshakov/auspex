# 2026-03-05-tasks-tab-navigation.md

**Status:** Complete

### Contracts

No API or schema changes. Frontend-only.

---

### TASK-01 `tab-bar`
**Type:** Regular
**Description:** Restructure `App.jsx` header and add tab navigation. Changes: (1) move `[ Refresh ]` button to the left, next to the `AUSPEX` logo; (2) add a tab bar to the right of the logo and Refresh button with two tabs: `Blueprints` and `Characters`; (3) add `useState('blueprints')` for the active tab; (4) render tab content conditionally — Blueprints tab shows the existing layout unchanged, Characters tab shows a placeholder (e.g. `<p>Characters</p>`) until the Characters page is implemented. Add corresponding CSS: tab items inline in the header, active tab highlighted with a bottom border in the same amber/gold colour used by the "IDLE BPOS" summary card.
**Definition of done:** working code + committed
**Status:** ✅ Done

### TASK-02 `review`
**Type:** Review
**Covers:** TASK-01
**Status:** ✅ Done

### TASK-03 `docs`
**Type:** Docs
**Status:** ✅ Done
