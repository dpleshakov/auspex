.PHONY: build frontend sqlc test lint lint-go lint-js check build-release release clean clean-all tidy

# ── Build ─────────────────────────────────────────────────────────────────────

build: frontend sqlc
	go build -o auspex ./cmd/auspex/

frontend:
	cd cmd/auspex/web && npm install && npm run build

sqlc:
	sqlc generate

# ── Test & Lint ───────────────────────────────────────────────────────────────

test:
	go test ./...

lint: lint-go lint-js

lint-go:
	go vet ./...
	golangci-lint run

lint-js:
	cd cmd/auspex/web && npm audit --audit-level=high

# ── CI (run before pushing) ───────────────────────────────────────────────────

check: lint test build

# ── Release ───────────────────────────────────────────────────────────────────

# Builds all platforms locally without publishing — for local testing.
release-local:
	goreleaser release --snapshot --clean

# Builds all platforms and publishes to GitHub Releases.
# Triggered automatically by CI on git tag push — do not run manually.
release:
	goreleaser release --clean

# ── Clean ─────────────────────────────────────────────────────────────────────

clean:
	rm -f auspex auspex.exe
	rm -rf cmd/auspex/web/dist/*
	touch cmd/auspex/web/dist/.gitkeep

# Removes the database — all characters must be re-added via OAuth after this.
clean-all: clean
	rm -f auspex.db