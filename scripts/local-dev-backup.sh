#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
BACKUP_DIR=~/localmdm-backup-$(date +%Y%m%d)

mkdir -p "$BACKUP_DIR"/{db,certs,configs,secrets}

echo "=== Local MDM Backup ==="
echo "Project: $PROJECT_DIR"
echo "Backup:  $BACKUP_DIR"
echo ""

# 1. Database dumps
echo "Dumping databases..."
docker compose -f "$PROJECT_DIR/docker-compose.yml" up -d postgres
echo "Waiting for postgres to be ready..."
until docker exec localmdm-postgres pg_isready -U postgres > /dev/null 2>&1; do
  sleep 2
done
for db in localmdm keycloak nanomdm; do
  docker exec localmdm-postgres pg_dump --clean --if-exists -U postgres "$db" > "$BACKUP_DIR/db/$db.sql"
  if [ ! -s "$BACKUP_DIR/db/$db.sql" ]; then
    echo "  ✗ WARNING: $db.sql is empty!"
  else
    echo "  ✓ $db"
  fi
done

# 2. Certificates
echo "Backing up certificates..."
cp "$PROJECT_DIR"/internal/api/certs/{ca.key,ca.crt,ca.crl,server.crt,server.key} "$BACKUP_DIR/certs/" 2>/dev/null || true
cp "$PROJECT_DIR/certs/ca.crl" "$BACKUP_DIR/certs/root-ca.crl" 2>/dev/null || true
echo "  ✓ certs"

# 3. Config
echo "Backing up config..."
cp "$PROJECT_DIR/configs/config.yaml" "$BACKUP_DIR/configs/" 2>/dev/null || true
echo "  ✓ config.yaml"

# 4. Secrets
echo "Backing up secrets..."
cp -R "$PROJECT_DIR/secrets/" "$BACKUP_DIR/secrets/"
echo "  ✓ secrets"

# 5. Environment files
for envfile in .env .env.local; do
  if [ -f "$PROJECT_DIR/$envfile" ]; then
    cp "$PROJECT_DIR/$envfile" "$BACKUP_DIR/"
    echo "  ✓ $envfile"
  fi
done

# 6. Untracked files
if [ -f "$PROJECT_DIR/Sprint prompts.md" ]; then
  cp "$PROJECT_DIR/Sprint prompts.md" "$BACKUP_DIR/"
  echo "  ✓ Sprint prompts.md (untracked)"
fi

echo ""
echo "=== Backup complete ==="
find "$BACKUP_DIR" -type f | sort
echo ""
echo "Safe to delete project folder and Docker volumes."
