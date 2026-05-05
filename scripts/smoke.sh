#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.dev.yml}"
DB_DSN="${DATABASE_URL:-postgres://phoenix:phoenix@localhost:5432/phoenix_feed?sslmode=disable}"
API_URL="${API_URL:-http://localhost:8080}"
CLIENT_ID="${CLIENT_ID:-smoke-device}"

API_PID=""
INGESTER_PID=""

cleanup() {
  if [[ -n "${API_PID}" ]]; then kill "${API_PID}" >/dev/null 2>&1 || true; fi
  if [[ -n "${INGESTER_PID}" ]]; then kill "${INGESTER_PID}" >/dev/null 2>&1 || true; fi
}
trap cleanup EXIT

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 1
  fi
}

wait_for_db() {
  for _ in {1..60}; do
    if docker compose -f "$COMPOSE_FILE" exec -T db pg_isready -U phoenix -d phoenix_feed >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  echo "database did not become healthy" >&2
  exit 1
}

apply_sql_file() {
  local file="$1"
  docker compose -f "$COMPOSE_FILE" exec -T db psql -U phoenix -d phoenix_feed < "$file"
}

apply_schema() {
  apply_sql_file db/schema.sql
  shopt -s nullglob
  for migration in db/migrations/*.sql; do
    apply_sql_file "$migration"
  done
}

wait_for_api() {
  for _ in {1..60}; do
    if curl -fsS "$API_URL/v1/health" >/tmp/phx-health.json 2>/dev/null; then
      return 0
    fi
    sleep 1
  done
  echo "api did not become healthy" >&2
  cat /tmp/phx-api.log >&2 || true
  exit 1
}

expect_active_meta_and_rate_limit() {
  local first_status second_status body
  body="$(mktemp)"
  first_status="$(curl -sS -o "$body" -w "%{http_code}" -H "X-Client-ID: $CLIENT_ID" "$API_URL/v1/incidents/active")"
  if [[ "$first_status" != "200" ]]; then
    echo "first active request returned $first_status" >&2
    cat "$body" >&2
    exit 1
  fi
  grep -q '"disclaimer":"Not for emergency use; call 911"' "$body"
  grep -q '"attribution":"Data via City of Phoenix Fire Department"' "$body"
  grep -q '"refresh_min_seconds":600' "$body"
  grep -q '"tier":"free"' "$body"

  second_status="$(curl -sS -o /tmp/phx-active-second.json -w "%{http_code}" -H "X-Client-ID: $CLIENT_ID" "$API_URL/v1/incidents/active")"
  if [[ "$second_status" != "429" ]]; then
    echo "second active request returned $second_status, want 429" >&2
    cat /tmp/phx-active-second.json >&2 || true
    exit 1
  fi
}

require_cmd docker
require_cmd go
require_cmd curl

docker compose -f "$COMPOSE_FILE" up -d db
wait_for_db
apply_schema

DATABASE_URL="$DB_DSN" POLL_INTERVAL=1s POLL_JITTER=0s go run ./cmd/ingester >/tmp/phx-ingester.log 2>&1 &
INGESTER_PID="$!"
sleep 30
kill "$INGESTER_PID" >/dev/null 2>&1 || true
INGESTER_PID=""

DATABASE_URL="$DB_DSN" HTTP_ADDR=:8080 go run ./cmd/api >/tmp/phx-api.log 2>&1 &
API_PID="$!"
wait_for_api
expect_active_meta_and_rate_limit

echo "smoke ok"
