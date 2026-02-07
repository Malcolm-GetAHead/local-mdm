---
name: final-verification
description: Comprehensive verification that code is ready for next phase
---

Perform final verification before proceeding to the next phase.

Specify what phase comes next (e.g., "Sprint 2", "Production Deployment", "Staging") and minimum test coverage percentage (default: 80%).

### VERIFICATION CHECKLIST:

#### 1. CODE QUALITY:
- [ ] All linters pass (golangci-lint, gosec, etc.)
- [ ] No TODO/FIXME/HACK comments
- [ ] No commented-out code
- [ ] Consistent style and naming
- [ ] All functions documented
- [ ] No code duplication

#### 2. TESTING:
- [ ] All tests pass
- [ ] No race conditions (go test -race)
- [ ] Coverage meets minimum threshold for critical paths
- [ ] Integration tests exist and pass
- [ ] Error paths tested
- [ ] Edge cases covered
- [ ] Performance tests pass

#### 3. SECURITY:
- [ ] No secrets in code/config
- [ ] All inputs validated
- [ ] SQL queries parameterized
- [ ] Authentication/authorization on all endpoints
- [ ] Audit logging for sensitive operations
- [ ] Secure defaults
- [ ] Dependencies scanned (no known vulnerabilities)

#### 4. RELIABILITY:
- [ ] All errors handled
- [ ] Resources cleaned up (defer, Close(), Stop())
- [ ] Context cancellation checked
- [ ] Transactions handle failures
- [ ] No unbounded loops/maps/channels
- [ ] Graceful degradation

#### 5. PRODUCTION READINESS:
- [ ] Configuration externalized
- [ ] Metrics/logging added
- [ ] Health checks work
- [ ] Deployment tested
- [ ] Rollback tested
- [ ] Documentation complete

### RUN THESE COMMANDS:
```bash
# Tests
go test ./... -v -race -cover -timeout 30s

# Coverage
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | grep total

# Linting
golangci-lint run ./...
gosec ./...

# Security scan
nancy go.sum

# Build
go build ./...
```

### DELIVERABLE:
- Pass/Fail for each checklist item
- List of any remaining issues
- Recommendation: **Proceed** or **Fix First**

### CRITICAL REQUIREMENT:
If ANY critical item fails, DO NOT proceed.
