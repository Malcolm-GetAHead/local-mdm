#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

# Find most recent backup, or use argument
if [ $# -ge 1 ]; then
  BACKUP_DIR="$1"
else
  BACKUP_DIR=$(ls -dt ~/localmdm-backup-* 2>/dev/null | head -1)
fi

if [ -z "$BACKUP_DIR" ] || [ ! -d "$BACKUP_DIR" ]; then
  echo "Usage: $0 [backup-dir]"
  echo "No backup found at ~/localmdm-backup-*"
  exit 1
fi

echo "=== Local MDM Restore ==="
echo "Project: $PROJECT_DIR"
echo "Backup:  $BACKUP_DIR"
echo ""

# 1. Certificates (before starting containers)
echo "Restoring certificates..."
mkdir -p "$PROJECT_DIR/internal/api/certs" "$PROJECT_DIR/certs"
cp "$BACKUP_DIR"/certs/{ca.key,ca.crt,ca.crl,server.crt,server.key} "$PROJECT_DIR/internal/api/certs/" 2>/dev/null || true
cp "$BACKUP_DIR/certs/root-ca.crl" "$PROJECT_DIR/certs/ca.crl" 2>/dev/null || true
echo "  ✓ certs"

# 2. Config
echo "Restoring config..."
mkdir -p "$PROJECT_DIR/configs"
cp "$BACKUP_DIR/configs/config.yaml" "$PROJECT_DIR/configs/" 2>/dev/null || true
echo "  ✓ config.yaml"

# 3. Secrets
echo "Restoring secrets..."
mkdir -p "$PROJECT_DIR/secrets"
cp -R "$BACKUP_DIR"/secrets/* "$PROJECT_DIR/secrets/" 2>/dev/null || true
echo "  ✓ secrets"

# 4. Environment files
for envfile in .env .env.local; do
  if [ -f "$BACKUP_DIR/$envfile" ]; then
    cp "$BACKUP_DIR/$envfile" "$PROJECT_DIR/"
    echo "  ✓ $envfile"
  fi
done

# 5. Start postgres and restore databases
echo "Starting postgres..."
docker compose -f "$PROJECT_DIR/docker-compose.yml" up -d postgres
echo "Waiting for postgres to be ready..."
until docker exec localmdm-postgres pg_isready -U postgres > /dev/null 2>&1; do
  sleep 2
done

echo "Restoring databases..."
for db in localmdm keycloak nanomdm; do
  if [ -f "$BACKUP_DIR/db/$db.sql" ]; then
    docker exec -i localmdm-postgres psql -U postgres "$db" < "$BACKUP_DIR/db/$db.sql"
    echo "  ✓ $db"
  else
    echo "  ⚠ $db.sql not found, skipping"
  fi
done

# 6. Untracked files
if [ -f "$BACKUP_DIR/Sprint prompts.md" ]; then
  cp "$BACKUP_DIR/Sprint prompts.md" "$PROJECT_DIR/"
  echo "  ✓ Sprint prompts.md"
fi

# 7. Start everything
echo ""
echo "Starting all services..."
docker compose -f "$PROJECT_DIR/docker-compose.yml" up -d
echo ""
echo "=== Restore complete ==="
echo "Run 'make dev-test' to verify."
