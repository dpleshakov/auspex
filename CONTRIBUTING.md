# Contributing to Auspex

## Prerequisites

| Tool | Version | Install |
|------|---------|---------|
| Go | 1.26+ | [golang.org/dl](https://golang.org/dl/) |
| Node.js | 18+ | [nodejs.org](https://nodejs.org/) |
| make | any | Bundled with macOS/Linux. Windows: install separately. |
| sqlc | v2 | `go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest` |
| golangci-lint | v2 | `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest` |
| goreleaser | v2 | `go install github.com/goreleaser/goreleaser/v2@latest` — required for release builds only |
| goversioninfo | latest | `go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest` — required for Windows PE metadata |

## Building from Source

Build order is critical and enforced by the Makefile:

1. `npm run build` — compiles the React frontend into `cmd/auspex/web/dist/`
2. `sqlc generate` — regenerates `internal/store/` from SQL query files
3. `go build` — compiles the Go binary with the embedded frontend

```bash
make build
```

This runs all three steps in the correct order and produces an `auspex` (or `auspex.exe`) binary in the project root.

### Cross-compilation

Because Auspex uses `modernc.org/sqlite` (pure Go, no CGO), you can cross-compile for any target platform without a C toolchain:

```bash
GOOS=linux   GOARCH=amd64 go build -o auspex-linux-amd64   ./cmd/auspex/
GOOS=windows GOARCH=amd64 go build -o auspex-windows-amd64.exe ./cmd/auspex/
GOOS=darwin  GOARCH=arm64 go build -o auspex-darwin-arm64  ./cmd/auspex/
```

> The frontend must be built first (`npm run build`) on the build host regardless of target platform, because `web/dist/` is embedded into the Go binary at compile time.

## Running Tests

```bash
make test
# or directly:
go test ./...
```

Tests are pure Go unit tests with no external dependencies. They use `httptest` for ESI client tests and mock interfaces for sync and store tests.

## Frontend Dev Server

The Vite dev server proxies `/api` and `/auth` to the Go backend, giving you hot module replacement while the backend serves real data:

```bash
# Start the backend (builds frontend once to satisfy go:embed, then runs)
./auspex

# In a separate terminal
cd cmd/auspex/web
npm install
npm run dev
```

Open `http://localhost:5173`.

## Makefile Targets

| Target | Action |
|--------|--------|
| `build` | Full build: frontend → sqlc → go build |
| `test` | `go test ./...` |
| `clean` | Remove binary and rebuild `web/dist/` with only `.gitkeep` |
| `clean-all` | `clean` + remove `auspex.db` |
| `release-notes` | Extract release notes for a version from `CHANGELOG.md` |
| `versioninfo` | Generate `cmd/auspex/versioninfo.json` and `resource.syso` for Windows builds |
| `release` | Local snapshot build via goreleaser (no publish) |

## Schema Changes

After any change to `internal/db/migrations/` or `internal/db/queries/`, regenerate the store:

```bash
sqlc generate
```

The Makefile `build` target does this automatically.

## Releasing

Releases are built with [goreleaser](https://goreleaser.com/). The config lives in `.goreleaser.yaml` — it builds binaries for Linux, macOS, and Windows (amd64 + arm64) and uploads archives to GitHub Releases as a draft.

### Test a release build locally

```bash
make release
```

Produces archives under `dist/` without creating a GitHub release.

### Publish a release

1. Update `CHANGELOG.md`: rename `[Unreleased]` to the new version with today's date, add a fresh empty `[Unreleased]` section above it.
2. Commit: `chore: release vX.Y.Z`.
3. Tag and push:

```bash
git tag vX.Y.Z
git push origin vX.Y.Z
```

goreleaser runs `make frontend`, `make sqlc`, `make versioninfo`, and `make release-notes` automatically before building (defined in `.goreleaser.yaml` `before.hooks`). Node.js, sqlc, and goversioninfo must be installed on the release host. A `GITHUB_TOKEN` environment variable with `repo` scope is required.

The release is created as a draft — review and publish manually on GitHub.

## Project Structure

See [docs/project-structure.md](docs/project-structure.md) for a detailed description of every directory and file.

## Architecture

See [docs/architecture.md](docs/architecture.md) for the high-level architecture, module responsibilities, database schema, and key design decisions.
