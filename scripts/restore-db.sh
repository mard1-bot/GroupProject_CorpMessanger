#!/bin/bash
# Restore script for PostgreSQL database
# This script restores a database backup from a file

set -e

# Configuration
BACKUP_DIR="/home/arseniyv/Рабочий стол/GroupProject_CorpMessanger/backups"
CONTAINER_NAME="corp-postgres"
DB_NAME="corpmessenger"
DB_USER="corpmessenger"

# Check if backup file is provided
if [ -z "$1" ]; then
    echo "Usage: $0 <backup_file.sql.gz>"
    echo "Available backups:"
    ls -lh "$BACKUP_DIR"/*.sql.gz 2>/dev/null || echo "No backups found"
    exit 1
fi

BACKUP_FILE="$1"

# Check if backup file exists
if [ ! -f "$BACKUP_FILE" ]; then
    # Try with full path
    BACKUP_FILE="$BACKUP_DIR/$1"
    if [ ! -f "$BACKUP_FILE" ]; then
        echo "Backup file not found: $1"
        exit 1
    fi
fi

echo "Restoring database from: $BACKUP_FILE"
echo "WARNING: This will overwrite all existing data!"
read -p "Are you sure? (yes/no): " confirm

if [ "$confirm" != "yes" ]; then
    echo "Restore cancelled"
    exit 0
fi

# Decompress and restore
if [[ $BACKUP_FILE == *.gz ]]; then
    gunzip -c "$BACKUP_FILE" | docker exec -i "$CONTAINER_NAME" psql -U "$DB_USER" "$DB_NAME"
else
    docker exec -i "$CONTAINER_NAME" psql -U "$DB_USER" "$DB_NAME" < "$BACKUP_FILE"
fi

echo "Restore completed successfully"
