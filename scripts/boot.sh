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

# Kill any existing backend on 8080 so we get fresh routes
if command -v lsof >/dev/null 2>&1; then
  lsof -ti:8080 | xargs kill -9 2>/dev/null || true
  sleep 1
fi

# The backend refuses to start without a JWT_SECRET of 32+ characters. For local
# dev, generate one into backend/.env (gitignored), replacing an empty or short one
# (e.g. from a copied .env.example).
ENV_FILE=backend/.env
if [ -z "$JWT_SECRET" ] && ! grep -qsE '^JWT_SECRET=.{32,}' "$ENV_FILE"; then
  if [ -f "$ENV_FILE" ]; then
    # Rewrite in place (not mv) so the file keeps its permissions.
    kept=$(grep -v '^JWT_SECRET=' "$ENV_FILE" || true)
    printf '%s' "$kept" > "$ENV_FILE"
    # Don't glue the new line onto a last line that lacks a newline.
    [ -s "$ENV_FILE" ] && [ -n "$(tail -c1 "$ENV_FILE")" ] && echo >> "$ENV_FILE"
  fi
  (umask 077; echo "JWT_SECRET=$(openssl rand -hex 32)" >> "$ENV_FILE")
  echo "Generated a local JWT_SECRET in $ENV_FILE"
fi

# Database: the project-local Postgres (scripts/dev-db.sh), unless DATABASE_URL is
# already set in the environment or backend/.env.
if [ -z "$DATABASE_URL" ] && ! grep -qs '^DATABASE_URL=' "$ENV_FILE"; then
  ./scripts/dev-db.sh start
  [ -s "$ENV_FILE" ] && [ -n "$(tail -c1 "$ENV_FILE")" ] && echo >> "$ENV_FILE"
  (umask 077; echo "DATABASE_URL=$(./scripts/dev-db.sh url)" >> "$ENV_FILE")
  echo "Using the dev database; DATABASE_URL added to $ENV_FILE"
elif grep -qs "^DATABASE_URL=$(./scripts/dev-db.sh url)$" "$ENV_FILE"; then
  ./scripts/dev-db.sh start
fi

echo "Starting backend (port 8080)..."
cd "$ROOT/backend"
go run . &
BACKEND_PID=$!
cd "$ROOT"

# Wait for backend to be ready
echo "Waiting for backend..."
until curl -sf http://localhost:8080/health >/dev/null 2>&1; do
  if ! kill -0 "$BACKEND_PID" 2>/dev/null; then
    echo "Backend exited during startup; see the output above." >&2
    exit 1
  fi
  sleep 0.5
done
echo "Backend ready."

echo "Starting frontend (port 5173)..."
cd "$ROOT/frontend"
pnpm dev
