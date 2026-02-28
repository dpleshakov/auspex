.PHONY: build test release-local release clean clean-all

# ── Build ─────────────────────────────────────────────────────────────────────

# Full check + build: consistency checks, linters, tests, compilation.
# Run before every push. Identical to what CI runs.
build:
	sqlc generate && git diff --exit-code internal/store/
	cd cmd/auspex/web && npm audit --audit-level=high
	cd cmd/auspex/web && npm ci && npm run build
	go mod tidy && git diff --exit-code go.mod go.sum
	go vet ./...
	golangci-lint run
	go test ./...
	go build -o auspex ./cmd/auspex/

# ── Test ──────────────────────────────────────────────────────────────────────

# Runs Go tests only — quick feedback during development.
test:
	go test ./...

# ── Release ───────────────────────────────────────────────────────────────────

# Internal target — used by .goreleaser.yaml hooks, do not call directly.
frontend:
	cd cmd/auspex/web && npm ci && npm run build

# Internal target — used by .goreleaser.yaml hooks, do not call directly.
sqlc:
	sqlc generate

# Builds all platforms locally without publishing — for local testing.
release:
	goreleaser release --snapshot --clean

# ── Clean ─────────────────────────────────────────────────────────────────────

clean:
	go run tools/rm.go auspex auspex.exe
	go run tools/rm.go -r cmd/auspex/web/dist
	go run tools/touch.go cmd/auspex/web/dist/.gitkeep

# Removes the database — all characters must be re-added via OAuth after this.
clean-all: clean
	go run tools/rm.go auspex.db
