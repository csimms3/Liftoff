#!/bin/bash
# Project-local PostgreSQL for development and tests: data in .pgdata/ (gitignored),
# TCP on localhost only, no password (trust auth: fine on a single-user dev machine,
# not reachable from other hosts). Uses the installed postgres binaries (e.g.
# `brew install postgresql@16`, which is keg-only: put its bin/ on PATH); leaves any
# other Postgres alone.
#
#   scripts/dev-db.sh start    start it (creating it the first time) and the
#                              `liftoff` and `liftoff_test` databases
#   scripts/dev-db.sh stop
#   scripts/dev-db.sh status
#   scripts/dev-db.sh url      print DATABASE_URL (add `test` for the test database)
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DATA="$ROOT/.pgdata"
PORT="${LIFTOFF_DEV_DB_PORT:-55433}"

url() { echo "postgres://postgres@localhost:$PORT/$1?sslmode=disable"; }

need() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "$1 not found. Install PostgreSQL (e.g. brew install postgresql@16) and put its bin/ on PATH." >&2
    exit 1
  fi
}

running() { pg_ctl -D "$DATA" status >/dev/null 2>&1; }

# A running cluster must be on the port we'd report; otherwise say so rather than
# hand out a URL that doesn't work.
check_port() {
  local live
  live=$(sed -n 4p "$DATA/postmaster.pid" 2>/dev/null || true)
  if [ -n "$live" ] && [ "$live" != "$PORT" ]; then
    echo "The dev database is running on port $live, not $PORT (LIFTOFF_DEV_DB_PORT). Stop it first: $0 stop" >&2
    exit 1
  fi
}

case "${1:-}" in
  start)
    need initdb; need pg_ctl; need createdb; need psql
    if [ ! -f "$DATA/PG_VERSION" ]; then
      initdb -D "$DATA" -U postgres --auth=trust >/dev/null
      echo "Created dev database cluster in .pgdata/"
    fi
    if running; then
      check_port
    else
      # -k '': no Unix socket (TCP only); listen on localhost only.
      pg_ctl -D "$DATA" -o "-p $PORT -k '' -c listen_addresses=localhost" -l "$DATA/server.log" -w start >/dev/null
    fi
    for db in liftoff liftoff_test; do
      if ! psql -h localhost -p "$PORT" -U postgres -d postgres -Atc "SELECT 1 FROM pg_database WHERE datname = '$db'" | grep -q 1; then
        createdb -h localhost -p "$PORT" -U postgres "$db"
      fi
    done
    echo "Dev Postgres running: $(url liftoff)"
    ;;
  stop)
    need pg_ctl
    if running; then pg_ctl -D "$DATA" -m fast stop >/dev/null && echo "Stopped"; else echo "Not running"; fi
    ;;
  status)
    need pg_ctl
    if running; then check_port; echo "Running on port $PORT"; else echo "Not running"; exit 1; fi
    ;;
  url)
    if [ "${2:-}" = test ]; then url liftoff_test; else url liftoff; fi
    ;;
  *)
    echo "usage: $0 start|stop|status|url [test]" >&2
    exit 2
    ;;
esac
