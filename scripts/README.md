# Database Backup Scripts

This directory contains scripts for backing up and restoring the PostgreSQL database outside of Docker volumes.

## Scripts

### backup-db.sh
Creates a compressed SQL backup of the corpmessenger database.

**Usage:**
```bash
./scripts/backup-db.sh
```

**Features:**
- Creates timestamped backups in `../backups/` directory
- Compresses backups with gzip
- Automatically removes backups older than 7 days
- Stores backups outside of Docker volumes

**Output:**
- Backup file: `../backups/corpmessenger_backup_YYYYMMDD_HHMMSS.sql.gz`

### restore-db.sh
Restores the database from a backup file.

**Usage:**
```bash
# List available backups
./scripts/restore-db.sh

# Restore from a specific backup
./scripts/restore-db.sh corpmessenger_backup_20240428_120000.sql.gz
```

**Warning:** This will overwrite all existing data in the database!

## Setup Automatic Backups

To set up automatic daily backups using cron:

```bash
# Edit crontab
crontab -e

# Add this line to backup daily at 2 AM
0 2 * * * /home/arseniyv/Рабочий стол/GroupProject_CorpMessanger/scripts/backup-db.sh >> /home/arseniyv/Рабочий стол/GroupProject_CorpMessanger/backups/backup.log 2>&1
```

## Backup Location

Backups are stored in: `../backups/` (outside of Docker volumes)

This ensures data persistence even if Docker volumes are deleted.

## Manual Backup Example

```bash
# Navigate to project root
cd /home/arseniyv/Рабочий стол/GroupProject_CorpMessanger

# Run backup
./scripts/backup-db.sh

# Check backup
ls -lh backups/
```

## Manual Restore Example

```bash
# Navigate to project root
cd /home/arseniyv/Рабочий стол/GroupProject_CorpMessanger

# List available backups
ls -lh backups/

# Restore from backup (you'll be prompted for confirmation)
./scripts/restore-db.sh corpmessenger_backup_20240428_120000.sql.gz
```
