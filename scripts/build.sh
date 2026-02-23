#!/bin/bash
set -e

# Run tests with: go test ./...

echo "==> Building frontend..."
cd cmd/auspex/web
npm install
npm run build

if [ ! -f "dist/index.html" ]; then
    echo "ERROR: Frontend build failed — dist/index.html not found."
    exit 1
fi

cd ../../..

echo "==> Generating store (sqlc)..."
sqlc generate

echo "==> Building binary..."
go build -o auspex ./cmd/auspex/

echo "Done. Run ./auspex to start."
