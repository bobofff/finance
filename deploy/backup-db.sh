#!/usr/bin/env bash
set -euo pipefail

if [[ -z "${DB_HOST:-}" || -z "${DB_USER:-}" || -z "${DB_NAME:-}" ]]; then
  echo "Missing DB_HOST/DB_USER/DB_NAME env vars. Source deploy/.env first."
  exit 1
fi

BACKUP_DIR="${BACKUP_DIR:-./backups}"
mkdir -p "$BACKUP_DIR"

TS="$(date +%Y%m%d_%H%M%S)"
OUT="${BACKUP_DIR}/finance_${TS}.sql.gz"

echo "==> Backing up to ${OUT}"
PGPASSWORD="${DB_PASSWORD:-}" pg_dump -h "$DB_HOST" -U "$DB_USER" -p "${DB_PORT:-5432}" "$DB_NAME" | gzip > "$OUT"

echo "==> Done"
