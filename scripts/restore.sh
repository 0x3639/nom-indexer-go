#!/bin/bash
#
# Restore the nom-indexer PostgreSQL database from a backup
#
# Usage: ./scripts/restore.sh <backup_file>
#
# Supports both .sql and .sql.gz files

set -e

CONTAINER_NAME="nom-indexer-postgres"
DB_NAME="nom_indexer"
DB_USER="postgres"

# Check arguments
if [ -z "$1" ]; then
    echo "Usage: $0 <backup_file>"
    echo ""
    echo "Available backups:"
    ls -lh ./backups/*.sql* 2>/dev/null || echo "  No backups found in ./backups/"
    exit 1
fi

BACKUP_FILE="$1"

# Check if backup file exists
if [ ! -f "${BACKUP_FILE}" ]; then
    echo "Error: Backup file '${BACKUP_FILE}' not found."
    exit 1
fi

# Check if container is running
if ! docker ps --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
    echo "Error: Container '${CONTAINER_NAME}' is not running."
    echo "Start it with: docker-compose up -d postgres"
    exit 1
fi

echo "WARNING: This will drop and recreate the '${DB_NAME}' database."
echo "All existing data will be lost!"
echo ""
read -p "Are you sure you want to continue? (yes/no): " CONFIRM

if [ "${CONFIRM}" != "yes" ]; then
    echo "Restore cancelled."
    exit 0
fi

echo ""
echo "Stopping indexer if running..."
docker stop nom-indexer 2>/dev/null || true

echo "Dropping and recreating database..."
docker exec "${CONTAINER_NAME}" psql -U "${DB_USER}" -c "DROP DATABASE IF EXISTS ${DB_NAME};"
docker exec "${CONTAINER_NAME}" psql -U "${DB_USER}" -c "CREATE DATABASE ${DB_NAME};"

echo "Restoring from ${BACKUP_FILE}..."

# Handle compressed vs uncompressed files
if [[ "${BACKUP_FILE}" == *.gz ]]; then
    gunzip -c "${BACKUP_FILE}" | docker exec -i "${CONTAINER_NAME}" psql -U "${DB_USER}" "${DB_NAME}"
else
    docker exec -i "${CONTAINER_NAME}" psql -U "${DB_USER}" "${DB_NAME}" < "${BACKUP_FILE}"
fi

echo ""
echo "Restore completed successfully!"
echo ""
echo "You can now restart the indexer with: docker-compose up -d indexer"
