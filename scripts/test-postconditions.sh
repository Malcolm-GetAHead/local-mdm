#!/bin/bash
# test-postconditions.sh — Verify test run didn't corrupt shared data.
# Runs inside the test-runner container after go test completes.
set -e

DB_HOST="${DB_HOST:-localhost}"
DB_PASSWORD="${DB_PASSWORD:-postgres-dev-password-1234}"
PGPASSWORD="$DB_PASSWORD"
export PGPASSWORD

SEED_ENT="00000000-0000-0000-0000-000000000001"
TEST_ENT="99999999-9999-9999-9999-999999999999"
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

# Clean up test data under the test enterprise (child rows only, never the enterprise itself)
for tbl in compliance_results policy_assignments device_group_members device_commands device_certificates enrollment_tokens devices policies device_groups users; do
  psql -h "$DB_HOST" -U postgres -d localmdm -t -A -c \
    "DELETE FROM $tbl WHERE enterprise_id = '$TEST_ENT';" 2>/dev/null || true
done
# Clean up audit logs created by tests under the test enterprise
psql -h "$DB_HOST" -U postgres -d localmdm -t -A -c \
  "DELETE FROM audit_logs WHERE enterprise_id = '$TEST_ENT';" 2>/dev/null || true

# Clean up devices leaked into seed enterprise by NanoMDM webhooks during mdmb e2e tests.
# mdmb generates serial numbers like "SN" + 8 hex chars. The running localmdm-server
# container receives NanoMDM webhooks and creates these devices under default_enterprise_id.
LEAKED=$(psql -h "$DB_HOST" -U postgres -d localmdm -t -A -c \
  "DELETE FROM devices WHERE enterprise_id = '$SEED_ENT' AND serial_number ~ '^SN[0-9a-f]{8}$' AND status = 'pending' RETURNING id;" 2>/dev/null | wc -l | tr -d ' ')
if [ "$LEAKED" -gt 0 ]; then
  echo "  Cleaned up $LEAKED leaked mdmb test device(s) from seed enterprise"
fi

# Clean up policies leaked by Playwright tests (non-seed UUIDs in seed enterprise)
LEAKED_POLICIES=$(psql -h "$DB_HOST" -U postgres -d localmdm -t -A -c \
  "DELETE FROM policies WHERE enterprise_id = '$SEED_ENT' AND id::text NOT LIKE 'e0000000-%' RETURNING id;" 2>/dev/null | wc -l | tr -d ' ')
if [ "$LEAKED_POLICIES" -gt 0 ]; then
  echo "  Cleaned up $LEAKED_POLICIES leaked test policy(ies) from seed enterprise"
fi

check "No leaked test enterprises" "0" \
  "SELECT count(*) FROM enterprises WHERE id NOT IN ('$SEED_ENT', '$TEST_ENT');"

check "Seed enterprise exists" "1" \
  "SELECT count(*) FROM enterprises WHERE id = '$SEED_ENT' AND deleted_at IS NULL;"

check "Test enterprise exists" "1" \
  "SELECT count(*) FROM enterprises WHERE id = '$TEST_ENT' AND deleted_at IS NULL;"

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
