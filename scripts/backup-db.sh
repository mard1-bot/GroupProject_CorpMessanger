#!/bin/bash
# Backup script for PostgreSQL database
# This script dumps the database to a file outside of Docker volumes

set -e

# Configuration
BACKUP_DIR="/home/arseniyv/Рабочий стол/GroupProject_CorpMessanger/backups"
CONTAINER_NAME="corp-postgres"
DB_NAME="corpmessenger"
DB_USER="corpmessenger"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
BACKUP_FILE="$BACKUP_DIR/corpmessenger_backup_$TIMESTAMP.sql"

# Create backup directory if it doesn't exist
mkdir -p "$BACKUP_DIR"

echo "Starting database backup..."
echo "Backup file: $BACKUP_FILE"

# Dump the database
docker exec "$CONTAINER_NAME" pg_dump -U "$DB_USER" "$DB_NAME" > "$BACKUP_FILE"

# Compress the backup
gzip "$BACKUP_FILE"
BACKUP_FILE="${BACKUP_FILE}.gz"

echo "Backup completed: $BACKUP_FILE"

# Keep only the last 7 days of backups
find "$BACKUP_DIR" -name "corpmessenger_backup_*.sql.gz" -type f -mtime +7 -delete

echo "Old backups (older than 7 days) removed"
