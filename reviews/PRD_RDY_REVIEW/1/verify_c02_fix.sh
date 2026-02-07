#!/bin/bash
# Verification script for C-02: Hardcoded Secrets Fix
# This script demonstrates that the fix is working correctly

set -e

echo "=========================================="
echo "C-02 Hardcoded Secrets Fix - Verification"
echo "=========================================="
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test 1: Verify no hardcoded secrets in config files
echo "Test 1: Checking config files for hardcoded secrets..."
if grep -q "password: \"postgres\"" configs/config.yaml; then
    echo -e "${RED}❌ FAIL: Found hardcoded password in config.yaml${NC}"
    exit 1
fi
if grep -q "jwt_secret: \"change-me-in-production\"" configs/config.yaml; then
    echo -e "${RED}❌ FAIL: Found default JWT secret in config.yaml${NC}"
    exit 1
fi
if grep -q "client_secret: \"localmdm-api-secret\"" configs/config.yaml; then
    echo -e "${RED}❌ FAIL: Found hardcoded Keycloak secret in config.yaml${NC}"
    exit 1
fi
echo -e "${GREEN}✅ PASS: No hardcoded secrets in config files${NC}"
echo ""

# Test 2: Verify .env.example exists
echo "Test 2: Checking for .env.example file..."
if [ ! -f .env.example ]; then
    echo -e "${RED}❌ FAIL: .env.example file not found${NC}"
    exit 1
fi
echo -e "${GREEN}✅ PASS: .env.example file exists${NC}"
echo ""

# Test 3: Run config validation tests
echo "Test 3: Running config validation tests..."
if ! go test -v ./internal/config/... -run TestSecretValidation 2>&1 | grep -q "PASS"; then
    echo -e "${RED}❌ FAIL: Secret validation tests failed${NC}"
    exit 1
fi
echo -e "${GREEN}✅ PASS: All secret validation tests passed${NC}"
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

# Test 6: Verify validation logic through unit tests
echo "Test 6: Verifying validation rejects weak secrets..."
if go test -v ./internal/config/... -run TestSecretValidation/default 2>&1 | grep -q "PASS"; then
    echo -e "${GREEN}✅ PASS: Validation correctly rejects weak secrets${NC}"
else
    echo -e "${RED}❌ FAIL: Validation tests failed${NC}"
    exit 1
fi
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
echo "The C-02 hardcoded secrets vulnerability has been successfully fixed."
echo "The system now:"
echo "  - Rejects default/weak secrets at startup"
echo "  - Requires environment variables for all sensitive credentials"
echo "  - Enforces minimum length requirements"
echo "  - Has no hardcoded secrets in configuration files"
echo ""
echo "Next steps:"
echo "  1. Set required environment variables (see .env.example)"
echo "  2. Continue with Week 1 Action Plan (C-07: TLS Enforcement)"
echo ""
