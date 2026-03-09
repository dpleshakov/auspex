.PHONY: build lint check test release release-notes versioninfo clean clean-all

# ── Build ─────────────────────────────────────────────────────────────────────

# Development build: generate code, build frontend, vet, test, compile.
# Safe to run before committing — no diff checks.
build:
	sqlc generate
	cd cmd/auspex/web && npm ci && npm run build
	go vet ./...
	go test ./...
	go build ./cmd/auspex/

# Static analysis: npm audit, go mod tidy, golangci-lint. Run before pushing.
lint:
	cd cmd/auspex/web && npm audit --audit-level=high
	go mod tidy
	golangci-lint run

# Full CI-equivalent check: build + lint + diff verification. Requires a clean worktree.
check: build lint test
	git diff --exit-code internal/store/ go.mod go.sum

# ── Test ──────────────────────────────────────────────────────────────────────

# Runs Go tests with coverage — prints per-function table and enforces the 80% threshold.
# internal/store is excluded (100% sqlc-generated); cmd/auspex is excluded (wire-up main only).
test:
	go test -coverprofile=coverage.out \
	    ./internal/config/... \
	    ./internal/db/... \
	    ./internal/esi/... \
	    ./internal/auth/... \
	    ./internal/sync/... \
	    ./internal/api/...
	go tool cover -func=coverage.out
	go run tools/check-coverage.go 80

# ── Release ───────────────────────────────────────────────────────────────────

# Internal target — used by .goreleaser.yaml hooks, do not call directly.
frontend:
	cd cmd/auspex/web && npm ci && npm run build

# Internal target — used by .goreleaser.yaml hooks, do not call directly.
sqlc:
	sqlc generate

# Generates cmd/auspex/versioninfo.json and cmd/auspex/*.syso.
# VERSION must be set: make versioninfo VERSION=0.1.0
versioninfo:
	go run tools/gen-versioninfo.go $(VERSION)
	goversioninfo -64 -o cmd/auspex/resource_windows_amd64.syso cmd/auspex/versioninfo.json
	goversioninfo -arm -o cmd/auspex/resource_windows_arm64.syso cmd/auspex/versioninfo.json

# Extracts the release notes for VERSION from CHANGELOG.md → docs/release-notes.md.
# Can be called standalone to verify output: make release-notes VERSION=0.1.0
VERSION ?= Unreleased
release-notes:
	go run tools/release-notes.go $(VERSION)

# Builds all platforms locally without publishing — for local testing.
release: release-notes
	goreleaser release --snapshot --clean --release-notes docs/release-notes.md --release-footer docs/release-footer.md

# ── Clean ─────────────────────────────────────────────────────────────────────

clean:
	go run tools/rm.go auspex auspex.exe
	go run tools/rm.go docs/release-notes.md
	go run tools/rm.go cmd/auspex/resource_windows_amd64.syso cmd/auspex/resource_windows_arm64.syso cmd/auspex/versioninfo.json
	go run tools/rm.go -r cmd/auspex/web/dist
	go run tools/rm.go -r dist
	go run tools/touch.go cmd/auspex/web/dist/.gitkeep

# Removes the database — all characters must be re-added via OAuth after this.
clean-all: clean
	go run tools/rm.go auspex.db auspex.yaml
