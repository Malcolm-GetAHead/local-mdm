#!/bin/bash
# test-postconditions.sh — Verify test run didn't corrupt shared data.
# Runs inside the test-runner container after go test completes.
set -e

DB_HOST="${DB_HOST:-localhost}"
DB_PASSWORD="${DB_PASSWORD:-postgres-dev-password-1234}"
PGPASSWORD="$DB_PASSWORD"
export PGPASSWORD

SEED_ENT="00000000-0000-0000-0000-000000000001"
FAIL=0

check() {
  local label="$1" expected="$2" query="$3"
  actual=$(psql -h "$DB_HOST" -U postgres -d localmdm -t -A -c "$query" 2>/dev/null)
  if [ "$actual" != "$expected" ]; then
    echo "FAIL: $label — expected $expected, got $actual"
    FAIL=1
  else
    echo "  OK: $label"
  fi
}

echo ""
echo "=== Test Postconditions ==="

check "No leaked test enterprises" "0" \
  "SELECT count(*) FROM enterprises WHERE id != '$SEED_ENT';"

check "Seed enterprise exists" "1" \
  "SELECT count(*) FROM enterprises WHERE id = '$SEED_ENT' AND deleted_at IS NULL;"

check "Seed policies intact" "8" \
  "SELECT count(*) FROM policies WHERE enterprise_id = '$SEED_ENT' AND deleted_at IS NULL;"

check "No orphaned audit logs from this run" "0" \
  "SELECT count(*) FROM audit_logs WHERE enterprise_id IS NULL AND created_at > NOW() - INTERVAL '10 minutes';"

echo ""
if [ $FAIL -ne 0 ]; then
  echo "POSTCONDITIONS FAILED — tests leaked data or corrupted seed state"
  exit 1
fi
echo "All postconditions passed."
