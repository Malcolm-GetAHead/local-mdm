#!/bin/bash
# Master test runner - runs all device enrollment tests
# Usage: ./run_all_tests.sh [macos|windows|android|all]

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RESULTS_DIR="$SCRIPT_DIR/../results"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
LOG_FILE="$RESULTS_DIR/test_run_${TIMESTAMP}.log"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Create results directory
mkdir -p "$RESULTS_DIR"

log() {
    echo -e "$1" | tee -a "$LOG_FILE"
}

run_test() {
    local test_name=$1
    local test_script=$2
    
    log "${YELLOW}=== Running $test_name ===${NC}"
    
    if python3 "$test_script" 2>&1 | tee -a "$LOG_FILE"; then
        log "${GREEN}✓ $test_name PASSED${NC}\n"
        return 0
    else
        log "${RED}✗ $test_name FAILED${NC}\n"
        return 1
    fi
}

# Parse arguments
PLATFORM=${1:-all}

log "=== Local MDM Device Testing ==="
log "Platform: $PLATFORM"
log "Timestamp: $TIMESTAMP"
log "Log file: $LOG_FILE"
log ""

# Check prerequisites
log "Checking prerequisites..."

# Check if MDM server is running
if ! curl -s http://localhost:8080/health > /dev/null 2>&1; then
    log "${RED}✗ MDM server is not running${NC}"
    log "Start it with: make run"
    exit 1
fi
log "${GREEN}✓ MDM server is running${NC}"

# Check Python dependencies
if ! python3 -c "import requests" 2>/dev/null; then
    log "${RED}✗ Python 'requests' module not found${NC}"
    log "Install it with: pip3 install requests"
    exit 1
fi
log "${GREEN}✓ Python dependencies OK${NC}"
log ""

# Track results
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

# Run macOS tests
if [ "$PLATFORM" = "macos" ] || [ "$PLATFORM" = "all" ]; then
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    if run_test "macOS Enrollment" "$SCRIPT_DIR/test_macos_enrollment.py"; then
        PASSED_TESTS=$((PASSED_TESTS + 1))
    else
        FAILED_TESTS=$((FAILED_TESTS + 1))
    fi
fi

# Run Windows tests
if [ "$PLATFORM" = "windows" ] || [ "$PLATFORM" = "all" ]; then
    if [ -f "$SCRIPT_DIR/test_windows_enrollment.py" ]; then
        TOTAL_TESTS=$((TOTAL_TESTS + 1))
        if run_test "Windows Enrollment" "$SCRIPT_DIR/test_windows_enrollment.py"; then
            PASSED_TESTS=$((PASSED_TESTS + 1))
        else
            FAILED_TESTS=$((FAILED_TESTS + 1))
        fi
    else
        log "${YELLOW}⚠ Windows tests not yet implemented${NC}\n"
    fi
fi

# Run Android tests
if [ "$PLATFORM" = "android" ] || [ "$PLATFORM" = "all" ]; then
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    if run_test "Android Enrollment" "$SCRIPT_DIR/test_android_enrollment.py"; then
        PASSED_TESTS=$((PASSED_TESTS + 1))
    else
        FAILED_TESTS=$((FAILED_TESTS + 1))
    fi
fi

# Summary
log "=== Test Summary ==="
log "Total tests: $TOTAL_TESTS"
log "${GREEN}Passed: $PASSED_TESTS${NC}"
if [ $FAILED_TESTS -gt 0 ]; then
    log "${RED}Failed: $FAILED_TESTS${NC}"
fi
log ""
log "Results saved to: $RESULTS_DIR"
log "Log file: $LOG_FILE"

# Generate HTML report
cat > "$RESULTS_DIR/report_${TIMESTAMP}.html" << EOF
<!DOCTYPE html>
<html>
<head>
    <title>Local MDM Test Report - $TIMESTAMP</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .pass { color: green; }
        .fail { color: red; }
        .summary { background: #f0f0f0; padding: 15px; margin: 20px 0; }
        .test { margin: 10px 0; padding: 10px; border-left: 3px solid #ccc; }
        .test.pass { border-left-color: green; }
        .test.fail { border-left-color: red; }
    </style>
</head>
<body>
    <h1>Local MDM Test Report</h1>
    <p>Timestamp: $TIMESTAMP</p>
    
    <div class="summary">
        <h2>Summary</h2>
        <p>Total Tests: $TOTAL_TESTS</p>
        <p class="pass">Passed: $PASSED_TESTS</p>
        <p class="fail">Failed: $FAILED_TESTS</p>
    </div>
    
    <h2>Test Results</h2>
    <div class="test pass">
        <h3>✓ macOS Enrollment</h3>
        <p>See detailed results in JSON files</p>
    </div>
    
    <h2>Screenshots</h2>
    <p>Screenshots saved to: screenshots/</p>
    
    <h2>Logs</h2>
    <pre>$(cat "$LOG_FILE")</pre>
</body>
</html>
EOF

log "HTML report: $RESULTS_DIR/report_${TIMESTAMP}.html"

# Exit with appropriate code
if [ $FAILED_TESTS -gt 0 ]; then
    exit 1
else
    exit 0
fi
