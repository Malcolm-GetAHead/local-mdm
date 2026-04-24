#!/bin/bash
# run_and_record.sh — Run a k6 scenario and append results to the history CSV.
#
# Usage: ./tests/load/run_and_record.sh <scenario.js> [sprint_label]
# Example: ./tests/load/run_and_record.sh tests/load/steady_state.js "Sprint 5b"
#
# Results are appended to tests/load/results_history.csv

set -euo pipefail

SCENARIO="${1:?Usage: $0 <scenario.js> [sprint_label]}"
SPRINT="${2:-$(git branch --show-current)}"
GIT_REF=$(git rev-parse --short HEAD)
TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
SCENARIO_NAME=$(basename "$SCENARIO" .js)
RESULTS_DIR="tests/load/results"
HISTORY_FILE="tests/load/results_history.csv"

mkdir -p "$RESULTS_DIR"

# Run k6 with JSON summary output
JSON_FILE="$RESULTS_DIR/${SCENARIO_NAME}_${GIT_REF}_$(date +%s).json"
k6 run --summary-export="$JSON_FILE" "$SCENARIO" || true

# Extract metrics from the JSON summary
if [ ! -f "$JSON_FILE" ]; then
    echo "ERROR: k6 did not produce summary output"
    exit 1
fi

# Parse key metrics using python (available on macOS and most Linux)
read -r P50 P95 P99 AVG REQS ERRORS DURATION <<< $(python3 -c "
import json, sys
with open('$JSON_FILE') as f:
    d = json.load(f)
m = d.get('metrics', {})
dur = m.get('http_req_duration', {}).get('values', {})
failed = m.get('http_req_failed', {}).get('values', {})
iters = m.get('iterations', {}).get('values', {})
print(
    dur.get('med', 0),
    dur.get('p(95)', 0),
    dur.get('p(99)', 0),
    dur.get('avg', 0),
    int(iters.get('count', 0)),
    round(failed.get('rate', 0) * 100, 2),
    round(iters.get('count', 0) / max(d.get('state', {}).get('testRunDurationMs', 1) / 1000, 1), 1),
)
")

# Create header if file doesn't exist
if [ ! -f "$HISTORY_FILE" ]; then
    echo "timestamp,git_ref,sprint,scenario,p50_ms,p95_ms,p99_ms,avg_ms,total_requests,error_rate_pct,rps,json_file" > "$HISTORY_FILE"
fi

# Append results
echo "${TIMESTAMP},${GIT_REF},${SPRINT},${SCENARIO_NAME},${P50},${P95},${P99},${AVG},${REQS},${ERRORS},${DURATION},${JSON_FILE}" >> "$HISTORY_FILE"

echo ""
echo "=== Results recorded ==="
echo "  Sprint:    $SPRINT"
echo "  Git ref:   $GIT_REF"
echo "  Scenario:  $SCENARIO_NAME"
echo "  p50:       ${P50}ms"
echo "  p95:       ${P95}ms"
echo "  p99:       ${P99}ms"
echo "  Requests:  $REQS"
echo "  Error %:   $ERRORS"
echo "  RPS:       $DURATION"
echo "  Raw JSON:  $JSON_FILE"
echo ""
echo "History: $HISTORY_FILE"
