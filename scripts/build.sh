#!/bin/bash
set -e

echo "==> Building frontend..."
cd cmd/auspex/web
npm install
npm run build
cd ../../..

echo "==> Generating store (sqlc)..."
sqlc generate

echo "==> Building binary..."
go build -o auspex ./cmd/auspex/

echo "Done. Run ./auspex to start."
