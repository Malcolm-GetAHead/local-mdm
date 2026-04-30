#!/bin/bash
# test-postconditions.sh — Preflight cleanup + post-test verification.
# Runs inside the test-runner container before and after go test.
#
# Usage:
#   sh test-postconditions.sh              # postconditions (default)
#   sh test-postconditions.sh --preflight  # preflight cleanup only
set -e

DB_HOST="${DB_HOST:-localhost}"
DB_PASSWORD="${DB_PASSWORD:-postgres-dev-password-1234}"
PGPASSWORD="$DB_PASSWORD"
export PGPASSWORD

SEED_ENT="00000000-0000-0000-0000-000000000001"
TEST_ENT="99999999-9999-9999-9999-999999999999"

# === Preflight: clean up orphaned test enterprises from crashed previous runs ===
# Integration tests create enterprises with slug LIKE 'test-%'. CASCADE deletes all child rows.
# The permanent test enterprise (TEST_ENT, slug 'test-enterprise') is preserved.
preflight() {
  echo ""
  echo "=== Preflight Cleanup ==="
  ORPHANED=$(psql -h "$DB_HOST" -U postgres -d localmdm -t -A -c \
    "DELETE FROM enterprises WHERE slug LIKE 'test-%' AND id != '$TEST_ENT' RETURNING id;" 2>/dev/null | wc -l | tr -d ' ')
  if [ "$ORPHANED" -gt 0 ]; then
    echo "  Cleaned up $ORPHANED orphaned test enterprise(s) from previous run"
  else
    echo "  No orphaned test enterprises found"
  fi
}

# === Postconditions: verify test run didn't corrupt shared data ===
postconditions() {
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

  # Clean up devices leaked into seed enterprise by NanoMDM webhooks during mdmb e2e tests.
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

  # Clean up any test enterprises whose t.Cleanup fired successfully — should be 0 already.
  # If any remain, CASCADE delete handles child rows.
  LEFTOVER=$(psql -h "$DB_HOST" -U postgres -d localmdm -t -A -c \
    "DELETE FROM enterprises WHERE slug LIKE 'test-%' AND id != '$TEST_ENT' RETURNING id;" 2>/dev/null | wc -l | tr -d ' ')
  if [ "$LEFTOVER" -gt 0 ]; then
    echo "  Cleaned up $LEFTOVER leftover test enterprise(s)"
  fi

  # No non-test, non-seed enterprises should exist
  check "No leaked non-test enterprises" "0" \
    "SELECT count(*) FROM enterprises WHERE id NOT IN ('$SEED_ENT', '$TEST_ENT') AND slug NOT LIKE 'test-%';"

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
}

# Dispatch
case "${1:-}" in
  --preflight) preflight ;;
  *)           postconditions ;;
esac
