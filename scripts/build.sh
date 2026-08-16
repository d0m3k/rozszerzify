#!/bin/bash
# build.sh — build the Go backend (linux/amd64 for the VPS) + Preact PWA frontend
set -euo pipefail
cd "$(dirname "$0")/.."

echo "=== Building backend (linux/amd64) ==="
cd backend
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o ../build/rozszerzify ./cmd/rozszerzify/
echo "Backend built: build/rozszerzify"

echo "=== Building frontend ==="
cd ../frontend
npm install --no-fund --no-audit --silent
npm run build
echo "Frontend built: frontend/dist/"

echo "=== Done ==="