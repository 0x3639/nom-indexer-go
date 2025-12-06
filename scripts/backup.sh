#!/bin/bash
#
# Backup the nom-indexer PostgreSQL database
#
# Usage: ./scripts/backup.sh [output_file]
#
# If no output file is specified, creates a timestamped backup in ./backups/

set -e

CONTAINER_NAME="nom-indexer-postgres"
DB_NAME="nom_indexer"
DB_USER="postgres"
BACKUP_DIR="./backups"

# Check if container is running
if ! docker ps --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
    echo "Error: Container '${CONTAINER_NAME}' is not running."
    echo "Start it with: docker-compose up -d postgres"
    exit 1
fi

# Determine output file
if [ -n "$1" ]; then
    OUTPUT_FILE="$1"
else
    mkdir -p "${BACKUP_DIR}"
    TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
    OUTPUT_FILE="${BACKUP_DIR}/nom_indexer_${TIMESTAMP}.sql.gz"
fi

echo "Backing up database '${DB_NAME}' from container '${CONTAINER_NAME}'..."

# Create backup with compression
docker exec "${CONTAINER_NAME}" pg_dump -U "${DB_USER}" "${DB_NAME}" | gzip > "${OUTPUT_FILE}"

# Get file size
FILE_SIZE=$(ls -lh "${OUTPUT_FILE}" | awk '{print $5}')

echo "Backup completed: ${OUTPUT_FILE} (${FILE_SIZE})"
