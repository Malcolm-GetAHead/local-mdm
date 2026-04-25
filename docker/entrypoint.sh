#!/bin/sh
set -e

# Run database migrations
echo "Running database migrations..."
migrate -path /app/migrations -database "postgres://${DB_USER:-postgres}:${DB_PASSWORD:-postgres-dev-password-1234}@${DB_HOST:-postgres}:${DB_PORT:-5432}/${DB_NAME:-localmdm}?sslmode=disable" up || true

echo "Starting Local MDM server..."
exec /app/localmdm "$@"
