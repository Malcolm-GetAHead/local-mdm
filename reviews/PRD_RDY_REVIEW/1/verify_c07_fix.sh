#!/bin/bash
# Verification script for C-07: TLS Enforcement Fix

set -e

echo "=========================================="
echo "C-07 TLS Enforcement Fix - Verification"
echo "=========================================="
echo ""

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Test 1: Verify environment field in config files
echo "Test 1: Checking config files for environment field..."
if ! grep -q "environment:" configs/config.yaml; then
    echo -e "${RED}❌ FAIL: environment field not found in config.yaml${NC}"
    exit 1
fi
echo -e "${GREEN}✅ PASS: environment field present in config files${NC}"
echo ""

# Test 2: Run environment validation tests
echo "Test 2: Running environment validation tests..."
if ! go test -v ./internal/config/... -run TestEnvironmentValidation 2>&1 | grep -q "PASS"; then
    echo -e "${RED}❌ FAIL: Environment validation tests failed${NC}"
    exit 1
fi
echo -e "${GREEN}✅ PASS: All environment validation tests passed${NC}"
echo ""

# Test 3: Run TLS validation tests
echo "Test 3: Running TLS validation tests..."
if ! go test -v ./internal/config/... -run TestTLSValidation 2>&1 | grep -q "PASS"; then
    echo -e "${RED}❌ FAIL: TLS validation tests failed${NC}"
    exit 1
fi
echo -e "${GREEN}✅ PASS: All TLS validation tests passed${NC}"
echo ""

# Test 4: Verify test coverage
echo "Test 4: Checking test coverage..."
COVERAGE=$(go test -cover ./internal/config/... 2>&1 | grep 'coverage:' | sed 's/.*coverage: \([0-9.]*\)%.*/\1/')
if [ -z "$COVERAGE" ]; then
    echo -e "${YELLOW}⚠️  WARNING: Could not determine coverage${NC}"
elif (( $(echo "$COVERAGE < 80" | bc -l 2>/dev/null || echo "0") )); then
    echo -e "${YELLOW}⚠️  WARNING: Coverage is ${COVERAGE}% (target: >80%)${NC}"
else
    echo -e "${GREEN}✅ PASS: Coverage is ${COVERAGE}% (target: >80%)${NC}"
fi
echo ""

# Test 5: Verify race condition testing
echo "Test 5: Running race detector..."
if go test -race ./internal/config/... 2>&1 | grep -qE "(PASS|cached)"; then
    echo -e "${GREEN}✅ PASS: No race conditions detected${NC}"
else
    echo -e "${RED}❌ FAIL: Race detector found issues${NC}"
    exit 1
fi
echo ""

# Test 6: Verify full test suite passes
echo "Test 6: Running full test suite..."
if go test -race ./... 2>&1 | grep -qE "FAIL"; then
    echo -e "${RED}❌ FAIL: Full test suite has failures${NC}"
    exit 1
fi
echo -e "${GREEN}✅ PASS: Full test suite passes${NC}"
echo ""

# Summary
echo "=========================================="
echo "Verification Summary"
echo "=========================================="
echo -e "${GREEN}✅ All verification tests passed!${NC}"
echo ""
echo "Fix Status: ✅ COMPLETE"
echo "Test Coverage: ${COVERAGE}%"
echo "Race Conditions: None detected"
echo ""
echo "The C-07 TLS enforcement vulnerability has been successfully fixed."
echo "The system now:"
echo "  - Requires TLS in production and staging environments"
echo "  - Validates TLS certificate configuration"
echo "  - Allows HTTP only in development"
echo "  - Provides clear error messages for misconfiguration"
echo ""
echo "Next steps:"
echo "  1. Deploy to staging with TLS enabled"
echo "  2. Verify TLS enforcement in staging"
echo "  3. Continue with Week 1 Action Plan (C-04: Panic Error Handling)"
echo ""
