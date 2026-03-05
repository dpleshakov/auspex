# Auspex Project Memory

## Process

- Before starting any task, read `docs/process-feature.md` — it is the main development loop.
- When completing a task, update its `**Status:**` to `✅ Done` in the task file (e.g. `docs/2026-03-05-tasks-characters-page.md`) BEFORE committing.
- Include the task file in the commit (it is part of the Definition of Done).

## Build Commands

- `make build` — sqlc generate + frontend build + go vet + go test + go build. Safe before commit, no diff checks.
- `make lint` — npm audit, go mod tidy, golangci-lint. Run before committing.
- `make check` — lint + build + git diff verification. Requires clean worktree (after commit).
- `make test` — Go tests only, quick feedback.

## Store / sqlc

- sqlc-generated files live in `internal/store/`. Run `sqlc generate` (or `make build`) after changing `internal/db/queries/`.
- sqlc uses `interface{}` for CASE subquery expressions (e.g. nullable computed columns). Handle with type assertion in handlers: `s, ok := val.(string)`.
- `ListCharactersWithMetaRow.SyncError` is `interface{}` — `nil` for NULL, `string` for text value.

## Architecture Notes

- `GET /api/characters` uses `ListCharactersWithMeta` query (JOIN with corporations + sync_state subquery).
- sync worker writes `last_error` to sync_state on failure, clears it on success via `UpdateSyncStateError`.
