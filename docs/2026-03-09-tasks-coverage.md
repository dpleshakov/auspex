# 2026-03-09-tasks-coverage.md

**Status:** Active

### Contracts

No API or schema changes. Build tooling only.

---

### TASK-01 `tools/check-coverage.go`
**Type:** Regular
**Description:** Create `tools/check-coverage.go` — a Go script tagged `//go:build ignore` that runs `go test -coverprofile=coverage.out ./...`, parses the total coverage percentage from `go tool cover -func` output, and exits with a non-zero code if the total is below the threshold. Threshold is passed as a CLI argument; default is 70. The script must work on Windows, macOS, and Linux without external dependencies.
**Definition of done:** working code + committed
**Status:** ✅ Done

### TASK-02 `make test`
**Type:** Regular
**Description:** Update the `test` target in Makefile to: run `go test -coverprofile=coverage.out ./...`, print per-function coverage via `go tool cover -func=coverage.out`, and invoke `go run tools/check-coverage.go 60` to enforce the threshold. Replaces the current bare `go test ./...`. Also add `/coverage.out` to `.gitignore` so the generated file does not dirty the worktree (which would break `make check`'s clean-worktree assertion).
**Definition of done:** working code + committed
**Status:** ✅ Done

### TASK-03 `make check`
**Type:** Regular
**Description:** Add `test` as a dependency of the `check` target. Current: `check: build lint` → updated: `check: build lint test`.
**Definition of done:** working code + committed
**Status:** ✅ Done

### TASK-04 `smoke`
**Type:** Smoke test
**Description:** Run `make test` and confirm: tests pass, per-function coverage table is printed, threshold check passes (current total: 61.3%, threshold: 60%). Finally, temporarily lower the threshold argument below 61 to verify that `make test` exits with a non-zero code, then restore the threshold.
**Status:** ✅ Done

### TASK-05 `review`
**Type:** Review
**Covers:** TASK-01, TASK-02, TASK-03, TASK-04
**Description:**
- Code: error handling in `check-coverage.go` (missing `total:` line, malformed percentage, `go test` failure), readability, obvious issues
- Security: no tokens in logs, no sensitive data involved
- Documentation: verify `technical-reference.md` matches reality — update if not; verify `architecture.md` — update if needed
**Status:** ✅ Done

### TASK-06 `docs`
**Type:** Docs
**Description:**
- Update user-facing documentation (README, help, guides) if behaviour visible to the user has changed
- Verify `technical-reference.md` is up to date — all changes introduced by this feature must be reflected
- Update `CHANGELOG.md` — only user-visible changes, following the format in `process-changelog.md`
**Status:** ⬜ Pending
