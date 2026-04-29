#!/bin/bash
# Combined coverage: Go unit tests + Playwright browser tests
# Usage: ./scripts/coverage-combined.sh
# Requires: Docker services running (postgres, keycloak, nanomdm), port 8080 free

COVER_DIR="/tmp/lmdm-coverage"
BINARY="/tmp/localmdm-cover"
CONFIG="configs/config.local.yaml"
SERVER_PID=""

cleanup() {
    if [ -n "$SERVER_PID" ] && kill -0 "$SERVER_PID" 2>/dev/null; then
        kill -TERM "$SERVER_PID" 2>/dev/null
        sleep 3
        kill -9 "$SERVER_PID" 2>/dev/null
    fi
    docker compose start localmdm > /dev/null 2>&1 || true
}
trap cleanup EXIT

rm -rf "$COVER_DIR"
mkdir -p "$COVER_DIR/browser"

echo "=== Building instrumented binary ==="
go build -cover -covermode=atomic -coverpkg=./cmd/server/...,./internal/... -o "$BINARY" ./cmd/server/ || exit 1

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
(cd tests/browser && npm install --silent 2>/dev/null && node run-playbook.js 2>&1 | tail -1)

echo "=== Stopping server ==="
kill -TERM "$SERVER_PID" 2>/dev/null || true
# Wait for graceful shutdown and coverage flush
for i in $(seq 1 10); do
    if ! kill -0 "$SERVER_PID" 2>/dev/null; then break; fi
    sleep 1
done
kill -9 "$SERVER_PID" 2>/dev/null || true
wait "$SERVER_PID" 2>/dev/null || true
SERVER_PID=""
sleep 1

echo "=== Coverage files ==="
ls -la "$COVER_DIR/browser/"

# Check if coverage data was written
if [ ! "$(ls -A "$COVER_DIR/browser/" 2>/dev/null)" ]; then
    echo ""
    echo "WARNING: No coverage data written by server."
    echo "The Go binary may not flush coverage on SIGINT."
    echo "Falling back to Go unit test coverage only."
    echo ""
fi

echo "=== Generating reports ==="
# Playwright coverage (if available)
if [ "$(ls -A "$COVER_DIR/browser/" 2>/dev/null)" ]; then
    go tool covdata textfmt -i="$COVER_DIR/browser" -o="$COVER_DIR/playwright.out" 2>/dev/null
fi

# Go unit test coverage
go test -coverprofile="$COVER_DIR/unit.out" -p 4 ./... > /dev/null 2>&1 || true

# Merge profiles: combine Go + Playwright into a single profile for true union coverage
MERGED_OUT="$COVER_DIR/merged.out"
if [ -f "$COVER_DIR/playwright.out" ]; then
    head -1 "$COVER_DIR/unit.out" > "$MERGED_OUT"
    tail -n +2 "$COVER_DIR/unit.out" >> "$MERGED_OUT"
    tail -n +2 "$COVER_DIR/playwright.out" >> "$MERGED_OUT"
else
    cp "$COVER_DIR/unit.out" "$MERGED_OUT"
fi

echo ""
echo "╔═══════════════════════════════════════════════════════════════════════════════╗"
echo "║                           Coverage Comparison                                ║"
echo "╠════════════════════════════╦══════════╦════════════╦════════════╦════════════╣"
echo "║ Package                    ║ Go Tests ║ Playwright ║   Merged   ║   Target   ║"
echo "╠════════════════════════════╬══════════╬════════════╬════════════╬════════════╣"

PW_OUT="$COVER_DIR/playwright.out"
[ ! -f "$PW_OUT" ] && PW_OUT="/dev/null"

for pkg in api auth audit certs config db metrics platform/android platform/macos platform/windows reporting repository scep service tracing validation; do
    go_pct=$(go tool cover -func="$COVER_DIR/unit.out" 2>/dev/null | grep "internal/$pkg/" | awk '{gsub(/%/,"",$NF); sum+=$NF; n++} END {if(n>0) printf "%.1f", sum/n; else print "0.0"}')
    pw_pct="n/a"
    if [ -f "$COVER_DIR/playwright.out" ]; then
        pw_pct=$(go tool cover -func="$COVER_DIR/playwright.out" 2>/dev/null | grep "internal/$pkg/" | awk '{gsub(/%/,"",$NF); sum+=$NF; n++} END {if(n>0) printf "%.1f", sum/n; else print "0.0"}')
    fi
    merged_pct=$(go tool cover -func="$MERGED_OUT" 2>/dev/null | grep "internal/$pkg/" | awk '{gsub(/%/,"",$NF); sum+=$NF; n++} END {if(n>0) printf "%.1f", sum/n; else print "0.0"}')
    printf "║ %-26s ║  %5s%%  ║   %5s%%   ║   %5s%%   ║            ║\n" "$pkg" "$go_pct" "$pw_pct" "$merged_pct"
done

echo "╠════════════════════════════╬══════════╬════════════╬════════════╬════════════╣"
go_total=$(go tool cover -func="$COVER_DIR/unit.out" 2>/dev/null | tail -1 | awk '{print $NF}')
pw_total="n/a"
[ -f "$COVER_DIR/playwright.out" ] && pw_total=$(go tool cover -func="$COVER_DIR/playwright.out" 2>/dev/null | tail -1 | awk '{print $NF}')
merged_total=$(go tool cover -func="$MERGED_OUT" 2>/dev/null | tail -1 | awk '{print $NF}')
printf "║ %-26s ║  %6s  ║   %6s   ║   %6s   ║            ║\n" "TOTAL" "$go_total" "$pw_total" "$merged_total"
echo "╚════════════════════════════╩══════════╩════════════╩════════════╩════════════╝"
echo ""
[ -f "$COVER_DIR/playwright.out" ] && echo "Playwright HTML: go tool cover -html=$COVER_DIR/playwright.out -o /tmp/playwright-coverage.html"
echo "Merged HTML:     go tool cover -html=$MERGED_OUT -o /tmp/merged-coverage.html"
echo "Unit test HTML:  go tool cover -html=$COVER_DIR/unit.out -o /tmp/unit-coverage.html"
