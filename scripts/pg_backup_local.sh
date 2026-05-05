#!/bin/bash
# Daily Postgres backup to local droplet disk.
#
# Runs pg_dump against the running db container, gzips the output, rotates
# the directory to keep the most recent 7 daily backups, and logs success.
#
# Cost: $0 (uses existing droplet disk). Complement to DigitalOcean's
# weekly volume snapshot — finer granularity, no extra service.
#
# Install:
#   sudo install -m 0755 scripts/pg_backup_local.sh /usr/local/bin/pg_backup_local.sh
#   sudo install -m 0644 scripts/pg-backup.service /etc/systemd/system/
#   sudo install -m 0644 scripts/pg-backup.timer   /etc/systemd/system/
#   sudo systemctl daemon-reload
#   sudo systemctl enable --now pg-backup.timer
#
# Manual run:
#   sudo /usr/local/bin/pg_backup_local.sh

set -euo pipefail

APP_DIR="/opt/phoenix-feed/app"
BACKUP_DIR="/var/backups/phoenix-feed"
RETENTION_DAYS=7
TIMESTAMP="$(date -u +%Y-%m-%dT%H-%M-%SZ)"
DEST="${BACKUP_DIR}/phoenix_feed-${TIMESTAMP}.sql.gz"

mkdir -p "${BACKUP_DIR}"

# Source env so POSTGRES_USER/DB are available.
set -a
. "${APP_DIR}/.env"
set +a

echo "[$(date -u +%FT%TZ)] starting pg_dump -> ${DEST}"

cd "${APP_DIR}"
docker compose --env-file .env -f docker-compose.prod.yml exec -T db \
    pg_dump --no-owner --no-privileges --format=plain \
        -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" \
    | gzip -9 > "${DEST}.tmp"

# Atomic move so partial dumps never appear with the final name.
mv "${DEST}.tmp" "${DEST}"

SIZE_KB=$(du -k "${DEST}" | cut -f1)
echo "[$(date -u +%FT%TZ)] dump complete, ${SIZE_KB} KB"

# Rotate: delete dumps older than RETENTION_DAYS.
deleted=0
while IFS= read -r f; do
    rm -f "$f"
    deleted=$((deleted + 1))
    echo "[$(date -u +%FT%TZ)] rotated old: $f"
done < <(find "${BACKUP_DIR}" -maxdepth 1 -name 'phoenix_feed-*.sql.gz' -mtime +${RETENTION_DAYS})

echo "[$(date -u +%FT%TZ)] done; ${deleted} old dump(s) rotated"

# Sanity check: at least one backup exists and the latest is non-trivially sized.
LATEST=$(ls -1t "${BACKUP_DIR}"/phoenix_feed-*.sql.gz 2>/dev/null | head -1 || true)
if [ -z "${LATEST}" ]; then
    echo "[$(date -u +%FT%TZ)] FAIL: no backups present after run"
    exit 1
fi
LATEST_SIZE=$(stat -c '%s' "${LATEST}")
if [ "${LATEST_SIZE}" -lt 1024 ]; then
    echo "[$(date -u +%FT%TZ)] FAIL: latest backup ${LATEST} is suspiciously small (${LATEST_SIZE} bytes)"
    exit 1
fi

echo "[$(date -u +%FT%TZ)] OK"
