#!/bin/bash
# Boot the full Liftoff app: backend (Go) + frontend (Vite)
# Press Ctrl+C to stop both

set -e
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

cleanup() {
  if [ -n "$BACKEND_PID" ]; then
    kill "$BACKEND_PID" 2>/dev/null || true
  fi
  exit 0
}
trap cleanup SIGINT SIGTERM

echo "Starting backend (port 8080)..."
cd "$ROOT/backend"
go run . &
BACKEND_PID=$!
cd "$ROOT"

# Wait for backend to be ready
echo "Waiting for backend..."
until curl -sf http://localhost:8080/health >/dev/null 2>&1; do
  sleep 0.5
done
echo "Backend ready."

echo "Starting frontend (port 5173)..."
cd "$ROOT/frontend"
pnpm dev
