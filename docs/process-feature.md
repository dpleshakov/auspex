# Feature Development

This process repeats for every feature — including the initial MVP features. There is no special MVP mode: the first features are developed exactly the same way as all subsequent ones.

---

## Overview

| Step | Conversation | Output |
|------|-------------|--------|
| 1 | Task breakdown | `YYYY-MM-DD-tasks-{feature}.md` |
| 2–N | One conversation per task | Working committed code + updated `technical-reference.md` |

---

## Step 1 — Task Breakdown

**Input:** `architecture.md` + `technical-reference.md` (if it exists)

If `technical-reference.md` does not yet exist, it is created during this conversation. It contains API endpoints, database schema, and any other technical details that need to be stable before tasks can be written.

**What to do:**
In conversation with AI, break the feature into tasks. AI proposes the task structure, you agree or adjust. The goal is a complete, ordered list where every task is atomic — achievable in one AI conversation (roughly 30–100 lines of code).

One conversation may produce more than one tasks file if the feature naturally splits into independent parts. This is preferable to forcing everything into one file.

**Output:** `YYYY-MM-DD-tasks-{feature}.md`

The date prefix keeps files ordered and prevents name collisions. Example: `2026-03-04-tasks-auth.md`.

### Task types

Every tasks file contains four types of tasks. AI places them during breakdown — you confirm their placement.

**Regular tasks** — implementation: code + tests. Each ends with a commit.

If a regular task involves changes to the database schema or API contracts, its Description must include an explicit line: "Update `technical-reference.md` to reflect these changes." This is part of the definition of done for that task — not deferred to review.

**Smoke test tasks** — placed at natural checkpoints before continuing (e.g. after backend, before frontend). Goal: verify the mechanism works end-to-end with minimal effort — logs, a primitive HTML page, a curl call. Not a full test, just a confidence check.

**Review tasks** — placed after each logical block (e.g. after all files in `auth/` are done). Each review task must explicitly list the following checklist items in its Description:
- Code: security, error handling, readability, obvious performance issues
- Security: input validation, no tokens in logs, errors do not expose internal details, dependency vulnerability check (`npm audit`, `go audit`, or equivalent for your stack)
- Documentation: verify `technical-reference.md` matches what was actually built — update if not; verify `architecture.md` — update if module responsibilities or interactions changed

**Docs task** — always the last task in the file. Its Description must explicitly list:
- Update user-facing documentation (README, help, guides) if behaviour visible to the user has changed
- Verify `technical-reference.md` is up to date — all API changes, schema changes, and new endpoints introduced by this feature must be reflected
- Update `CHANGELOG.md` following the format in `process-changelog.md` — only changes visible to the user; no technical details, no refactoring, no infrastructure changes

### Tasks file structure

```markdown
## YYYY-MM-DD-tasks-{feature}.md

**Status:** Active

### Contracts
[API endpoints and DB schema planned for this feature — filled during breakdown]

---

### TASK-01 `name`
**Type:** Regular
**Description:** ...
**Definition of done:** working code + tests + committed
**Status:** ⬜ Pending

### TASK-02 `name`
**Type:** Smoke test
**Description:** ...
**Status:** ⬜ Pending

### TASK-03 `name`
**Type:** Review
**Covers:** TASK-01, TASK-02
**Description:**
- Code: security, error handling, readability, obvious performance issues
- Security: input validation, no tokens in logs, errors do not expose internal details, dependency vulnerability check
- Documentation: verify `technical-reference.md` matches reality — update if not; verify `architecture.md` — update if needed
**Status:** ⬜ Pending

...

### TASK-NN `docs`
**Type:** Docs
**Description:**
- Update user-facing documentation (README, help, guides) if behaviour visible to the user has changed
- Verify `technical-reference.md` is up to date — all API and schema changes introduced by this feature must be reflected
- Update `CHANGELOG.md` — only user-visible changes, following the format in `process-changelog.md`
**Status:** ⬜ Pending
```

---

## Step 2–N — Task Execution

Each task is a separate conversation with AI.

**Input for every conversation:** the specific task + relevant source files (only what is needed for this task — not the entire project)

### Regular task

1. Give AI the task and relevant context
2. Receive code — understand it, do not accept blindly
3. Write tests in the same conversation
4. Run code and tests, test manually
5. If something is wrong — fix in the same conversation
6. Mark the task status as `✅ Done` in the tasks file and commit everything — code, tests, and the updated tasks file — in a single commit. Do not make a separate commit just to update the status.
7. If this was the last task in the file — move the file to `docs/archive/` in the same commit and update the file header status to `Archived`.

### Smoke test task

Build the minimal thing that demonstrates the mechanism works. This is not a deliverable — it is a checkpoint. Log output, a primitive page, a curl call — whatever is fastest. Commit if useful, discard if not.

### Review task

Work through every item listed in the task Description. Commit any fixes and documentation updates. Mark the task as `✅ Done` and include the updated tasks file in the same commit.

### Docs task

Work through every item listed in the task Description. Commit all documentation changes. Mark the task as `✅ Done` and include the updated tasks file in the same commit — and move the file to `docs/archive/` since this is always the last task.

---

## Task status values

| Symbol | Meaning |
|--------|---------|
| ⬜ | Pending |
| 🔄 | In progress |
| ✅ | Done |
| ⏭️ | Skipped — reason noted |

When all tasks are done, update the file header status to `Archived` and move the file to `docs/archive/`. Both happen in the same commit as the last task.
