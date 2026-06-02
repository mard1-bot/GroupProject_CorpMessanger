#!/usr/bin/env bash
# PostgreSQL backup script for CorpMessenger
# Run via cron: 0 2 * * * /path/to/scripts/backup-postgres.sh
# Keeps 7 daily backups, 4 weekly backups, 12 monthly backups

set -euo pipefail

BACKUP_DIR="$(cd "$(dirname "$0")/.." && pwd)/backups/postgres"
SECRETS_DIR="$(cd "$(dirname "$0")/.." && pwd)/.secrets"
mkdir -p "$BACKUP_DIR"
chmod 700 "$BACKUP_DIR"

# Load database URL
if [ ! -f "$SECRETS_DIR/database_url" ]; then
  echo "❌ Error: .secrets/database_url not found"
  exit 1
fi

DATABASE_URL=$(cat "$SECRETS_DIR/database_url")

# Extract components from DATABASE_URL
# Format: postgres://user:pass@host:port/dbname?sslmode=require
DB_HOST=$(echo "$DATABASE_URL" | sed -n 's/.*@\([^:]*\):.*/\1/p')
DB_PORT=$(echo "$DATABASE_URL" | sed -n 's/.*:\([0-9]*\)\/.*/\1/p')
DB_NAME=$(echo "$DATABASE_URL" | sed -n 's/.*\/\([^?]*\).*/\1/p')
DB_USER=$(echo "$DATABASE_URL" | sed -n 's/\/\/\([^:]*\):.*/\1/p')
DB_PASS=$(cat "$SECRETS_DIR/postgres_password")

# Set PGPASSWORD for pg_dump
export PGPASSWORD="$DB_PASS"

TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="$BACKUP_DIR/corpmessenger_$TIMESTAMP.sql.gz"

echo "[backup-postgres] Starting backup at $(date)"

# Perform backup
pg_dump -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" | gzip > "$BACKUP_FILE"

if [ $? -eq 0 ]; then
  echo "✓ Backup completed: $BACKUP_FILE"
  SIZE=$(du -h "$BACKUP_FILE" | cut -f1)
  echo "  Size: $SIZE"
else
  echo "❌ Backup failed!"
  exit 1
fi

# Clean up old backups
# Keep last 7 daily backups
find "$BACKUP_DIR" -name "corpmessenger_*.sql.gz" -type f -mtime +7 -delete

# Keep weekly backups (every Sunday)
if [ "$(date +%u)" -eq 7 ]; then
  WEEKLY_BACKUP="$BACKUP_DIR/weekly/corpmessenger_weekly_$(date +%Y%m%d).sql.gz"
  mkdir -p "$BACKUP_DIR/weekly"
  cp "$BACKUP_FILE" "$WEEKLY_BACKUP"
  find "$BACKUP_DIR/weekly" -name "corpmessenger_weekly_*.sql.gz" -type f -mtime +28 -delete
  echo "✓ Weekly backup created: $WEEKLY_BACKUP"
fi

# Keep monthly backups (first day of month)
if [ "$(date +%d)" -eq 01 ]; then
  MONTHLY_BACKUP="$BACKUP_DIR/monthly/corpmessenger_monthly_$(date +%Y%m).sql.gz"
  mkdir -p "$BACKUP_DIR/monthly"
  cp "$BACKUP_FILE" "$MONTHLY_BACKUP"
  find "$BACKUP_DIR/monthly" -name "corpmessenger_monthly_*.sql.gz" -type f -mtime +365 -delete
  echo "✓ Monthly backup created: $MONTHLY_BACKUP"
fi

# Clean up PGPASSWORD
unset PGPASSWORD

echo "[backup-postgres] Backup finished at $(date)"
