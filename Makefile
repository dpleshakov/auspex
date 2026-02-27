.PHONY: build test release-local release clean clean-all

# ── Build ─────────────────────────────────────────────────────────────────────

# Full check + build: consistency checks, linters, tests, compilation.
# Run before every push. Identical to what CI runs.
build:
	go mod tidy && git diff --exit-code go.mod go.sum
	sqlc generate && git diff --exit-code internal/store/
	go vet ./...
	golangci-lint run
	cd cmd/auspex/web && npm audit --audit-level=high
	go test ./...
	cd cmd/auspex/web && npm ci && npm run build
	go build -o auspex ./cmd/auspex/

# ── Test ──────────────────────────────────────────────────────────────────────

# Runs Go tests only — quick feedback during development.
test:
	go test ./...

# ── Release ───────────────────────────────────────────────────────────────────

# Internal targets — used by .goreleaser.yaml hooks, do not call directly.
frontend:
	cd cmd/auspex/web && npm ci && npm run build

sqlc:
	sqlc generate

# Builds all platforms locally without publishing — for local testing.
release-local:
	goreleaser release --snapshot --clean

# Builds all platforms and publishes to GitHub Releases.
# Triggered automatically by CI on git tag push — do not run manually.
release:
ifndef CI
	$(error this target should only run in CI. Use 'make release-local' for local testing)
endif
	goreleaser release --clean

# ── Clean ─────────────────────────────────────────────────────────────────────

clean:
	rm -f auspex auspex.exe
	rm -rf cmd/auspex/web/dist/*
	touch cmd/auspex/web/dist/.gitkeep

# Removes the database — all characters must be re-added via OAuth after this.
clean-all: clean
	rm -f auspex.db
