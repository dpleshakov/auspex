# Auspex — Deployment

Auspex is a local desktop tool. It runs as a single self-contained binary on the same machine where you open the browser. There is no server-side deployment, no cloud infrastructure, and no external database.

---

## Prerequisites

### Runtime (end users)

No prerequisites. Download the binary and run it.

### Build time (developers or self-compilers)

| Tool | Version | Install |
|------|---------|---------|
| Go | 1.22+ | [golang.org/dl](https://golang.org/dl/) |
| Node.js | 18+ | [nodejs.org](https://nodejs.org/) |
| npm | 9+ | Bundled with Node.js |
| make | any | Bundled with macOS/Linux. Windows: install separately. |
| sqlc | v2 | `go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest` |
| golangci-lint | v2 | `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest` — required for `make check` |

### Release (maintainers only)

| Tool | Version | Install |
|------|---------|---------|
| goreleaser | v2 | `go install github.com/goreleaser/goreleaser/v2@latest` |

---

## Building from Source

Build order is critical and enforced by the Makefile:

1. **`npm run build`** — compiles the React frontend into `cmd/auspex/web/dist/`
2. **`sqlc generate`** — regenerates `internal/store/` from SQL query files
3. **`go build`** — compiles the Go binary with the embedded frontend

### Using the Makefile (recommended)

```bash
make build
```

> **Windows:** `make` is not included with Windows by default and must be installed separately.

### Manual build

```bash
# 1. Build the frontend
cd cmd/auspex/web
npm install
npm run build
cd ../../..

# 2. Regenerate the store
sqlc generate

# 3. Build the binary
go build -o auspex ./cmd/auspex/        # macOS/Linux
go build -o auspex.exe ./cmd/auspex/    # Windows
```

### Cross-compilation

Because Auspex uses `modernc.org/sqlite` (pure Go, no CGO), you can cross-compile for any target platform from any host without a C toolchain.

```bash
# Build a Linux amd64 binary from macOS or Windows
GOOS=linux GOARCH=amd64 go build -o auspex-linux-amd64 ./cmd/auspex/

# Build a Windows amd64 binary from macOS or Linux
GOOS=windows GOARCH=amd64 go build -o auspex-windows-amd64.exe ./cmd/auspex/

# Build a macOS arm64 binary (Apple Silicon) from any host
GOOS=darwin GOARCH=arm64 go build -o auspex-darwin-arm64 ./cmd/auspex/
```

> **Important:** The frontend must be built first (`npm run build`) on the build host regardless of the target platform, because `web/dist/` is embedded into the Go binary at compile time.

---

## Configuration

1. Copy the example config:

   ```bash
   cp auspex.example.yaml auspex.yaml
   ```

2. Open `auspex.yaml` and set your EVE SSO credentials:

   ```yaml
   port: 8080
   db_path: auspex.db
   refresh_interval: 10

   esi:
     client_id: "your-client-id"
     client_secret: "your-client-secret"
     callback_url: "http://localhost:8080/auth/eve/callback"
   ```

3. Make sure `auspex.yaml` is in the working directory when you run the binary, or use the `-config` flag:

   ```bash
   ./auspex -config /path/to/auspex.yaml
   ```

`auspex.yaml` is listed in `.gitignore` and must never be committed to version control.

---

## Running

```bash
./auspex            # macOS/Linux
auspex.exe          # Windows
```

The binary expects `auspex.yaml` (or the path passed via `-config`) in the current working directory. The SQLite database (`auspex.db` by default) is created automatically on first run in the same directory.

Open your browser and navigate to `http://localhost:8080`.

---

## File Layout at Runtime

```
auspex(.exe)       # the binary
auspex.yaml        # config file with credentials (not in git)
auspex.db          # SQLite database (created automatically)
```

The database stores all character tokens, blueprint data, and sync state. Backing up `auspex.db` preserves all data. Deleting it returns Auspex to a clean state (all characters need to be re-added via OAuth).

---

## Startup Sequence

On startup, Auspex:

1. Reads and validates `auspex.yaml`
2. Opens (or creates) the SQLite database at `db_path`
3. Applies any pending schema migrations
4. Creates ESI HTTP client, auth provider, sync worker
5. Starts the background sync worker (runs an initial cycle immediately)
6. Starts the HTTP server on `port`

On `SIGINT` or `SIGTERM` (Ctrl+C):

1. Stops accepting new HTTP requests (10-second drain for in-flight requests)
2. Signals the sync worker to stop after its current cycle completes
3. Exits cleanly

---

## Updating

1. Pull the latest source
2. Run `make build` again — it handles all steps in the correct order
3. Replace the binary
4. Restart

The database schema uses up-only migrations. New versions add migrations automatically on startup; no manual SQL is required.

---

## Releasing

Releases are built with [goreleaser](https://goreleaser.com/). The release config lives in `.goreleaser.yaml` — it builds binaries for Linux, macOS, and Windows (amd64 + arm64) and uploads archives to GitHub Releases.

### Test a release build locally (no publish)

```bash
make release-local
```

Runs `goreleaser release --snapshot --clean`. Produces archives under `dist/` without creating a GitHub release.

### Publish a release

```bash
git tag v1.2.3
git push origin v1.2.3
```

The `make release` target (`goreleaser release --clean`) is intended for CI. It requires a `GITHUB_TOKEN` environment variable with `repo` scope and a clean git tag.

> **Note:** goreleaser runs `make frontend` and `make sqlc` automatically before building (defined in `.goreleaser.yaml` `before.hooks`). Node.js and sqlc must still be installed on the release host.

---

## Running Tests

```bash
go test ./...
```

Tests are pure Go unit tests with no external dependencies. They use `httptest` for ESI client tests and mock interfaces for sync and store tests.

---

## Troubleshooting

**`config: esi.client_id is required`**
The config file was not found, or `esi.client_id` is empty. Check the file path and that credentials are filled in.

**`db: ...`**
The SQLite database could not be opened. Check that the directory specified by `db_path` exists and is writable.

**`preparing static files: ...`**
The binary was compiled without running `npm run build` first. The `web/dist/` directory was empty. Rebuild using `make build`.

**OAuth callback fails with "invalid or expired OAuth state"**
The browser completed the OAuth flow in a different server session (e.g. after a restart). Restart the login flow from `/auth/eve/login`.

**Blueprint data is missing or stale**
Click the "Refresh" button on the dashboard to trigger a force sync, or wait for the next automatic sync (interval configured by `refresh_interval`). Check `/api/sync/status` to see when each subject was last synced.

**Large corporation — only partial data shown**
ESI pagination is not implemented in the MVP. Only the first page (up to ~1000 items) is fetched. See TD-04 in `docs/tech-debt.md`.
