#!/usr/bin/env bash
set -euo pipefail

if [[ "$#" -ne 1 ]]; then
  echo "usage: scripts/apply-sql-file.sh <sql-file>" >&2
  exit 2
fi

sql_file="$1"
if [[ ! -f "$sql_file" ]]; then
  echo "SQL file not found: $sql_file" >&2
  exit 2
fi

COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.dev.yml}"
POSTGRES_USER="${POSTGRES_USER:-phoenix}"
POSTGRES_DB="${POSTGRES_DB:-phoenix_feed}"

compose_args=(compose)
if [[ -n "${COMPOSE_ENV_FILE:-}" ]]; then
  compose_args+=(--env-file "$COMPOSE_ENV_FILE")
fi
compose_args+=(
  -f "$COMPOSE_FILE"
  exec -T db psql
  -v ON_ERROR_STOP=1
  --single-transaction
  -U "$POSTGRES_USER"
  -d "$POSTGRES_DB"
)

docker "${compose_args[@]}" < "$sql_file"
