# 2026-03-05-tasks-makefile-build.md

**Status:** Archived

### Contracts

No API or schema changes. Infrastructure-only.

---

### TASK-01 `makefile-restructure`
**Type:** Regular
**Description:** Restructure Makefile build targets to fix the fundamental conflict between
code generation (which modifies files) and diff checks (which require no modified files).

**Current problem:** `make build` runs `sqlc generate` and then immediately checks
`git diff --exit-code internal/store/`. This makes `make build` fail whenever there are
uncommitted changes — i.e. during development, exactly when it is needed most.

**Solution:** split into three targets with distinct purposes:

**`build`** — fast development build, safe to run at any point before committing:
```makefile
build:
    sqlc generate
    cd cmd/auspex/web && npm run build
    go vet ./...
    go test ./...
    go build ./cmd/auspex/
```

**`lint`** — static analysis without compilation, run before pushing:
```makefile
lint:
    cd cmd/auspex/web && npm audit --audit-level=high
    go mod tidy
    golangci-lint run
```

**`check`** — full CI-equivalent validation, requires a clean worktree:
```makefile
check: lint build
    git diff --exit-code internal/store/ go.mod go.sum
```

**Rationale for diff checks staying only in `check`:** diff checks are only meaningful
in CI where the worktree is always clean. Locally, they only block the developer.
CI is the right place to catch forgotten-to-commit generated files.

**Note on `npm ci` vs `npm run build`:** in `build`, use `npm run build` (no `npm ci`)
to avoid re-downloading node_modules on every run. `npm ci` is appropriate only in CI.
The `check` target inherits `build`, so CI gets `npm run build`; `npm ci` in CI is
handled by the setup-node action cache or can be added as a separate step if needed.

**Also update `.PHONY`:** add `lint` and `check` to the `.PHONY` declaration.

**Definition of done:** `make build` succeeds with uncommitted changes present; `make lint`
runs golangci-lint and npm audit standalone; `make check` runs everything including diff
checks; committed.
**Status:** ✅ Done

---

### TASK-02 `ci-update`
**Type:** Regular
**Description:** Update `.github/workflows/ci.yml` to use `make check` instead of `make build`.

**Current state:**
```yaml
- name: Build
  run: make build
```

**Target state:**
```yaml
- name: Check
  run: make check
```

This is the only required change. CI already has a clean worktree (fresh checkout), so
`make check` with its `git diff --exit-code` will correctly catch any generated files
that were not committed.

**Definition of done:** `ci.yml` uses `make check`; committed.
**Status:** ✅ Done

---

### TASK-03 `docs-update`
**Type:** Regular
**Description:** Update all documentation that references the old `make build` semantics
or the expected command list.

**Files to update:**

**`CLAUDE.md` — "Expected Commands" section:**

The section currently has `make check` marked as "once implemented" and `make build`
described as a full build. After this task, the descriptions should match reality:

```markdown
# Run during development — generate, compile, test. Safe before committing.
make build

# Static analysis — linters and dependency audit. Run before pushing.
make lint

# Full CI-equivalent check — requires a clean worktree (all changes committed).
make check
```

Remove the "once implemented" qualifier from `make check`.

**`CONTRIBUTING.md` — "Makefile Targets" table:**

Add `lint` and `check` rows; update the description of `build`:

| Target | Action |
|--------|--------|
| `build` | Development build: sqlc → frontend → vet → test → go build. Safe to run before committing. |
| `lint`  | Static analysis: npm audit, go mod tidy, golangci-lint. Run before pushing. |
| `check` | Full CI check: lint + build + git diff consistency verification. Requires clean worktree. |

**Definition of done:** both files updated and accurately describe the three targets; committed.
**Status:** ✅ Done

---

### TASK-04 `review`
**Type:** Review
**Covers:** TASK-01, TASK-02, TASK-03
**Checklist:**
- Run `make build` with uncommitted changes — must succeed
- Run `make lint` standalone — must run golangci-lint and npm audit
- Run `make check` with clean worktree — must pass
- Run `make check` with uncommitted generated files — must fail on git diff
- Verify CI workflow file is valid YAML
- Verify CONTRIBUTING.md and CLAUDE.md accurately describe the three targets
**Status:** ✅ Done
