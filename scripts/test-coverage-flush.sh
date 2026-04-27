#!/bin/bash
# Minimal test: verify instrumented binary writes coverage on SIGINT
# Usage: ./scripts/test-coverage-flush.sh
# Requires: Docker services running (postgres, keycloak)

set -e

COVER_DIR="/tmp/lmdm-coverage-test"
BINARY="/tmp/localmdm-cover-test"
CONFIG="configs/config.local.yaml"

rm -rf "$COVER_DIR"
mkdir -p "$COVER_DIR"

echo "1. Building instrumented binary..."
go build -cover -coverpkg=./internal/... -o "$BINARY" ./cmd/server/ || exit 1

echo "2. Starting server..."
GOCOVERDIR="$COVER_DIR" CONFIG_PATH="$CONFIG" "$BINARY" > /dev/null 2>&1 &
PID=$!
sleep 3

if ! curl -sf http://localhost:8080/health > /dev/null 2>&1; then
    echo "ERROR: Server failed to start"
    kill -9 $PID 2>/dev/null
    exit 1
fi

echo "3. Sending one request..."
curl -sf http://localhost:8080/health > /dev/null

echo "4. Sending SIGINT..."
kill -INT $PID
for i in $(seq 1 10); do
    if ! kill -0 $PID 2>/dev/null; then break; fi
    sleep 1
done

echo "5. Checking coverage files..."
FILES=$(ls "$COVER_DIR"/ 2>/dev/null | wc -l | tr -d ' ')
if [ "$FILES" -gt 0 ]; then
    echo "✅ SUCCESS: $FILES coverage file(s) written"
    ls -la "$COVER_DIR"/
    echo ""
    echo "Coverage flush works. Run 'make coverage-combined' for the full report."
else
    echo "❌ FAIL: No coverage files written"
    echo "The os.Exit fix may not have been applied to cmd/server/main.go"
    exit 1
fi

rm -f "$BINARY"
rm -rf "$COVER_DIR"
