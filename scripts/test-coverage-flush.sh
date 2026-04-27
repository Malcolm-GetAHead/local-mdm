#!/bin/bash
# Test if the instrumented Local MDM binary writes coverage on shutdown
# Run from project root: ./scripts/test-coverage-flush.sh
set -e

COVER_DIR="/tmp/lmdm-coverage-debug"
BINARY="/tmp/localmdm-cover-debug"
CONFIG="configs/config.local.yaml"

rm -rf "$COVER_DIR"
mkdir -p "$COVER_DIR"

echo "1. Building instrumented binary..."
echo "   go build -cover -covermode=atomic -coverpkg=./cmd/server/...,./internal/... -o $BINARY ./cmd/server/"
go build -cover -covermode=atomic -coverpkg=./cmd/server/...,./internal/... -o "$BINARY" ./cmd/server/

echo "   Checking binary instrumentation..."
go tool covdata debugdump -i=/dev/null 2>&1 | head -1 || true
echo "   Binary size: $(du -h "$BINARY" | cut -f1)"

echo ""
echo "2. Starting server..."
GOCOVERDIR="$COVER_DIR" CONFIG_PATH="$CONFIG" "$BINARY" > /tmp/lmdm-cover.log 2>&1 &
PID=$!
sleep 4

if ! curl -sf http://localhost:8080/health > /dev/null 2>&1; then
    echo "   ❌ Server failed to start. Log:"
    tail -10 /tmp/lmdm-cover.log
    kill -9 $PID 2>/dev/null || true
    exit 1
fi
echo "   Server running (PID $PID)"

echo ""
echo "3. Sending test requests..."
curl -sf http://localhost:8080/health > /dev/null
curl -sf http://localhost:8080/health/ready > /dev/null 2>&1 || true

echo ""
echo "4. Checking GOCOVERDIR from /proc..."
echo "   GOCOVERDIR=$COVER_DIR"
ls -la "$COVER_DIR"/

echo ""
echo "5. Sending SIGTERM (not SIGINT)..."
kill -TERM $PID

echo "   Waiting for shutdown..."
for i in $(seq 1 15); do
    if ! kill -0 $PID 2>/dev/null; then
        echo "   Server exited after ${i}s"
        break
    fi
    sleep 1
done

if kill -0 $PID 2>/dev/null; then
    echo "   ⚠️  Still running after 15s — killing"
    kill -9 $PID 2>/dev/null || true
fi
wait $PID 2>/dev/null || true

echo ""
echo "=== Server log (last 10 lines) ==="
tail -10 /tmp/lmdm-cover.log

echo ""
echo "=== Coverage directory ==="
ls -la "$COVER_DIR"/ 2>/dev/null || echo "(empty)"

FILES=$(ls "$COVER_DIR"/ 2>/dev/null | wc -l | tr -d ' ')
if [ "$FILES" -gt 0 ]; then
    echo ""
    echo "✅ SUCCESS: $FILES coverage file(s) written"
    echo ""
    echo "Generate report with:"
    echo "  go tool covdata textfmt -i=$COVER_DIR -o=/tmp/coverage.out"
    echo "  go tool cover -func=/tmp/coverage.out | tail -1"
else
    echo ""
    echo "❌ FAIL: No coverage files written"
    echo ""
    echo "Trying alternative: build with go test -c..."
    BINARY2="/tmp/localmdm-cover-test"
    rm -rf "$COVER_DIR"
    mkdir -p "$COVER_DIR"
    # Build as test binary — this uses a different coverage mechanism
    go test -cover -covermode=atomic -coverpkg=./internal/... -c -o "$BINARY2" ./cmd/server/ 2>/dev/null && {
        echo "   Test binary built. Starting..."
        GOCOVERDIR="$COVER_DIR" CONFIG_PATH="$CONFIG" "$BINARY2" -test.run=^$ -test.coverprofile="$COVER_DIR/coverage.out" > /tmp/lmdm-cover2.log 2>&1 &
        PID2=$!
        sleep 4
        curl -sf http://localhost:8080/health > /dev/null 2>&1 && {
            echo "   Server running (PID $PID2), sending SIGTERM..."
            kill -TERM $PID2
            wait $PID2 2>/dev/null || true
            echo "   Coverage files:"
            ls -la "$COVER_DIR"/
        } || {
            echo "   Test binary server failed to start"
            kill -9 $PID2 2>/dev/null || true
        }
        rm -f "$BINARY2"
    } || echo "   go test -c failed (expected if no _test.go in cmd/server)"
fi

rm -f "$BINARY"
