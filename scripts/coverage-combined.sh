#!/bin/bash
# Combined coverage: Go unit tests + Playwright browser tests
# Usage: ./scripts/coverage-combined.sh
# Requires: Docker services running (postgres, keycloak, nanomdm), port 8080 free
set -e

COVER_DIR="/tmp/lmdm-coverage"
BINARY="/tmp/localmdm-cover"
CONFIG="configs/config.local.yaml"

cleanup() {
    echo "Cleaning up..."
    [ -n "$SERVER_PID" ] && kill -INT "$SERVER_PID" 2>/dev/null && wait "$SERVER_PID" 2>/dev/null
    docker compose start localmdm > /dev/null 2>&1 || true
}
trap cleanup EXIT

rm -rf "$COVER_DIR"
mkdir -p "$COVER_DIR/browser"

echo "=== Building instrumented binary ==="
go build -cover -coverpkg=./internal/... -o "$BINARY" ./cmd/server/

echo "=== Stopping Docker localmdm ==="
docker compose stop localmdm > /dev/null 2>&1 || true
sleep 2

echo "=== Seeding database ==="
docker compose exec -T postgres psql -U postgres -d localmdm -f /dev/stdin < migrations/seed_data.sql > /dev/null 2>&1

echo "=== Starting instrumented server ==="
GOCOVERDIR="$COVER_DIR/browser" CONFIG_PATH="$CONFIG" "$BINARY" > /dev/null 2>&1 &
SERVER_PID=$!
sleep 4

if ! curl -sf http://localhost:8080/health > /dev/null; then
    echo "ERROR: Server failed to start"
    exit 1
fi
echo "Server running (PID $SERVER_PID)"

echo "=== Running Playwright ==="
cd tests/browser
npm install --silent 2>/dev/null
node run-playbook.js 2>&1 | tail -1
cd ../..

echo "=== Stopping server ==="
kill -INT "$SERVER_PID"
wait "$SERVER_PID" 2>/dev/null
SERVER_PID=""
sleep 1

echo "=== Generating report ==="
go tool covdata textfmt -i="$COVER_DIR/browser" -o="$COVER_DIR/playwright.out"

# Also get Go unit test coverage
go test -coverprofile="$COVER_DIR/unit.out" -p 4 ./... > /dev/null 2>&1 || true

echo ""
echo "╔══════════════════════════════════════════════════════════════════╗"
echo "║                    Coverage Comparison                          ║"
echo "╠════════════════════════════╦══════════╦════════════╦════════════╣"
echo "║ Package                    ║ Go Tests ║ Playwright ║  Combined  ║"
echo "╠════════════════════════════╬══════════╬════════════╬════════════╣"

for pkg in api auth audit certs config db logging metrics models apperrors platform/android platform/macos platform/windows reporting repository scep service tracing validation; do
    go_pct=$(go tool cover -func="$COVER_DIR/unit.out" 2>/dev/null | grep "internal/$pkg/" | awk '{gsub(/%/,"",$NF); sum+=$NF; n++} END {if(n>0) printf "%.1f", sum/n; else print "0.0"}')
    pw_pct=$(go tool cover -func="$COVER_DIR/playwright.out" 2>/dev/null | grep "internal/$pkg/" | awk '{gsub(/%/,"",$NF); sum+=$NF; n++} END {if(n>0) printf "%.1f", sum/n; else print "0.0"}')
    # Combined = max of the two (simplified — real merge would be line-level)
    combined=$(echo "$go_pct $pw_pct" | awk '{if($1>$2) print $1; else print $2}')
    printf "║ %-26s ║  %5s%%  ║   %5s%%   ║   %5s%%   ║\n" "$pkg" "$go_pct" "$pw_pct" "$combined"
done

echo "╠════════════════════════════╬══════════╬════════════╬════════════╣"
go_total=$(go tool cover -func="$COVER_DIR/unit.out" 2>/dev/null | tail -1 | awk '{print $NF}')
pw_total=$(go tool cover -func="$COVER_DIR/playwright.out" 2>/dev/null | tail -1 | awk '{print $NF}')
printf "║ %-26s ║  %6s  ║   %6s   ║            ║\n" "TOTAL" "$go_total" "$pw_total"
echo "╚════════════════════════════╩══════════╩════════════╩════════════╝"
echo ""
echo "Playwright report: go tool cover -html=$COVER_DIR/playwright.out -o /tmp/playwright-coverage.html"
echo "Unit test report:  go tool cover -html=$COVER_DIR/unit.out -o /tmp/unit-coverage.html"
