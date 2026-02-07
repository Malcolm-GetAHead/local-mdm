# Week 1 Action Plan - Critical Security Fixes

**Goal**: Fix all CRITICAL issues to make system minimally deployable  
**Duration**: 5 working days (40 hours)  
**Team**: 1 developer  
**Status**: 🔴 **BLOCKING** - Cannot deploy without completion

---

## Day 1: Authentication & Secrets (8 hours)

### Task 1.1: Fix Authentication Bypass (C-01) ✅ COMPLETED
**Time**: 2 hours  
**Priority**: 🔴 CRITICAL  
**Status**: ✅ COMPLETED (2026-02-07)

**Steps**:
1. ✅ Modified `internal/api/server.go:New()` to return error if auth fails
2. ✅ Updated `cmd/server/main.go` to handle error
3. ✅ Added test: `TestServerStartupFailsWithInvalidKeycloak`
4. ✅ Verified: Start server with invalid Keycloak URL, fails as expected

**Code Changes**:
```bash
# Files modified
internal/api/server.go - Made auth initialization mandatory
cmd/server/main.go - Handle error from api.New()
internal/api/server_auth_test.go - Added comprehensive tests (5 tests)
```

**Verification**:
```bash
# Server refuses to start with invalid Keycloak
✅ KEYCLOAK_URL=http://invalid:9999 ./server
# Error: CRITICAL: Cannot start server without authentication

# Server starts with valid Keycloak
✅ KEYCLOAK_URL=http://localhost:8180 ./server
# Server starts successfully

# All tests pass
✅ go test -race ./...
# PASS - No race conditions
```

**Documentation**: `reviews/PRD_RDY_REVIEW/1/C-01_AUTH_BYPASS_FIX.md`

---

### Task 1.2: Remove Hardcoded Secrets (C-02) ✅ COMPLETED
**Time**: 4 hours  
**Priority**: 🔴 CRITICAL  
**Status**: ✅ COMPLETED (2026-02-07)

**Steps**:
1. ✅ Updated `internal/config/config.go:Validate()` to reject default secrets
2. ✅ Updated `configs/config.yaml` to use environment variable placeholders
3. ✅ Created `.env.example` with required variables
4. ✅ Updated `README.md` with environment variable documentation
5. ✅ Added test: `TestSecretValidation` (10 test cases)

**Code Changes**:
```bash
# Files modified
internal/config/config.go - Added validateSecrets() method
configs/config.yaml - Removed hardcoded secrets
configs/config.example.yaml - Removed hardcoded secrets
.env.example - Created with documentation
internal/config/config_test.go - Added 11 comprehensive tests
```

**Verification**:
```bash
# Server refuses to start with default secrets
✅ ./server --config configs/config.yaml
# Error: CRITICAL: jwt_secret must be changed from default value

# Server starts with strong secrets from environment
✅ export DB_PASSWORD="$(openssl rand -base64 24)"
✅ export JWT_SECRET="$(openssl rand -base64 48)"
✅ export KEYCLOAK_CLIENT_SECRET="real-secret"
✅ ./server --config configs/config.yaml
# Server starts successfully

# All tests pass with 98.1% coverage
✅ go test -race -cover ./internal/config/...
# PASS - coverage: 98.1% of statements
```

**Documentation**: `reviews/PRD_RDY_REVIEW/1/C-02_HARDCODED_SECRETS_FIX.md`

---

### Task 1.3: Enforce TLS (C-07) ✅ COMPLETED
**Time**: 2 hours  
**Priority**: 🔴 CRITICAL  
**Status**: ✅ COMPLETED (2026-02-07)

**Steps**:
1. ✅ Added `Environment` field to config (development, staging, production)
2. ✅ Added `validateEnvironment()` method
3. ✅ Added `validateTLS()` method to enforce TLS in production/staging
4. ✅ Updated config files with environment field
5. ✅ Added test: `TestEnvironmentValidation` (5 test cases)
6. ✅ Added test: `TestTLSValidation` (9 test cases)

**Code Changes**:
```bash
# Files modified
internal/config/config.go - Added environment and TLS validation
configs/config.yaml - Added environment field
configs/config.example.yaml - Added environment field
.env.example - Added ENVIRONMENT variable
internal/config/config_test.go - Added 15 comprehensive tests
```

**Verification**:
```bash
# Production refuses to start without TLS
✅ ENVIRONMENT=production ./server --config configs/config.yaml
# Error: CRITICAL: TLS must be enabled in production environment

# Staging refuses to start without TLS
✅ ENVIRONMENT=staging ./server --config configs/config.yaml
# Error: CRITICAL: TLS must be enabled in staging environment

# Development allows HTTP
✅ ENVIRONMENT=development ./server --config configs/config.yaml
# Server starts successfully

# All tests pass with 98.7% coverage
✅ go test -race -cover ./internal/config/...
# PASS - coverage: 98.7% of statements
```

**Documentation**: `reviews/PRD_RDY_REVIEW/1/C-07_TLS_ENFORCEMENT_FIX.md`

---
3. Reject HTTP in production mode
4. Add test: `TestHTTPRejectedInProduction`

**Code Changes**:
```bash
# Files to modify
internal/api/server.go
internal/config/config.go
```

**Verification**:
```bash
# Development: HTTP allowed
ENVIRONMENT=development TLS_ENABLED=false ./server

# Production: HTTP rejected
ENVIRONMENT=production TLS_ENABLED=false ./server
# Should exit with error
```

---

## Day 2: Error Handling & HTTP Safety (8 hours)

### Task 2.1: Remove Panic Error Handling (C-04)
**Time**: 4 hours  
**Priority**: 🔴 CRITICAL

**Steps**:
1. Find all usages of `MustUserFromContext`
2. Replace with `UserFromContext` + error handling
3. Remove `MustUserFromContext` function
4. Add test: `TestHandlerWithoutUser`

**Code Changes**:
```bash
# Find all usages
grep -r "MustUserFromContext" --include="*.go"

# Files to modify (likely)
internal/api/handlers.go
internal/auth/context.go
```

**Verification**:
```bash
# All tests should pass
go test -race ./...

# No panics in codebase (except test panics)
grep -r "panic(" --include="*.go" | grep -v "_test.go"
```

---

### Task 2.2: Fix HTTP Client Timeouts (C-09)
**Time**: 2 hours  
**Priority**: 🔴 CRITICAL

**Steps**:
1. Create `internal/auth/http_client.go` with safe client
2. Update `internal/auth/oidc.go` to use safe client
3. Add URL validation function
4. Add test: `TestJWKSURLValidation`

**Code Changes**:
```bash
# Files to modify
internal/auth/oidc.go
internal/auth/http_client.go (new)
internal/auth/oidc_test.go
```

**Verification**:
```bash
# Test with slow endpoint
go test -v -run TestOIDCValidatorTimeout ./internal/auth

# Test with internal IP
go test -v -run TestJWKSURLValidation ./internal/auth
```

---

### Task 2.3: Add Database Connection Limits (C-10)
**Time**: 2 hours  
**Priority**: 🔴 CRITICAL

**Steps**:
1. Update `internal/db/db.go:New()` to validate limits
2. Add reasonable defaults if not specified
3. Add connection pool metrics
4. Add test: `TestDatabaseConnectionLimits`

**Code Changes**:
```bash
# Files to modify
internal/db/db.go
internal/db/db_test.go
```

**Verification**:
```bash
# Should reject invalid limits
MAX_OPEN_CONNS=0 ./server
MAX_OPEN_CONNS=10000 ./server

# Should accept valid limits
MAX_OPEN_CONNS=25 ./server
```

---

## Day 3: Rate Limiting & Audit Logging (8 hours)

### Task 3.1: Implement Redis Rate Limiting (C-05)
**Time**: 6 hours  
**Priority**: 🔴 CRITICAL

**Steps**:
1. Add Redis dependency to `go.mod`
2. Create `internal/api/ratelimit_redis.go`
3. Update `internal/api/server.go` to use Redis limiter
4. Add configuration for Redis URL
5. Add test: `TestRedisRateLimiter`
6. Add fallback to in-memory for development

**Code Changes**:
```bash
# Files to create/modify
go.mod
internal/api/ratelimit_redis.go (new)
internal/api/server.go
internal/config/config.go
internal/api/ratelimit_redis_test.go (new)
```

**Verification**:
```bash
# Start Redis
docker run -d -p 6379:6379 redis:7-alpine

# Test rate limiting
go test -v -run TestRedisRateLimiter ./internal/api

# Load test with 10K IPs
k6 run --vus 10000 --duration 1m rate-limit-test.js
```

---

### Task 3.2: Implement Audit Logging (C-06)
**Time**: 2 hours (basic implementation)  
**Priority**: 🔴 CRITICAL

**Steps**:
1. Create `internal/audit/audit.go`
2. Add audit logging to authentication events
3. Add audit logging to authorization failures
4. Add test: `TestAuditLogging`

**Code Changes**:
```bash
# Files to create/modify
internal/audit/audit.go (new)
internal/audit/audit_test.go (new)
internal/auth/middleware.go
internal/api/server.go
```

**Verification**:
```bash
# Test audit logging
go test -v -run TestAuditLogging ./internal/audit

# Verify logs written to database
psql -c "SELECT * FROM audit_logs ORDER BY created_at DESC LIMIT 10"
```

---

## Day 4: Testing & Validation (8 hours)

### Task 4.1: Security Test Suite
**Time**: 4 hours  
**Priority**: 🔴 CRITICAL

**Steps**:
1. Create `tests/security/` directory
2. Add authentication bypass tests
3. Add SQL injection tests
4. Add rate limiting tests
5. Add cross-tenant access tests

**Code Changes**:
```bash
# Files to create
tests/security/auth_test.go (new)
tests/security/injection_test.go (new)
tests/security/ratelimit_test.go (new)
tests/security/isolation_test.go (new)
```

**Verification**:
```bash
# All security tests should pass
go test -v ./tests/security/...

# Run with race detector
go test -race ./tests/security/...
```

---

### Task 4.2: Load Testing
**Time**: 2 hours  
**Priority**: 🔴 CRITICAL

**Steps**:
1. Create `tests/load/` directory
2. Write k6 load test script
3. Test with 1000 concurrent users
4. Verify no memory leaks
5. Verify rate limiting works

**Code Changes**:
```bash
# Files to create
tests/load/load-test.js (new)
tests/load/README.md (new)
```

**Verification**:
```bash
# Run load test
k6 run --vus 1000 --duration 10m tests/load/load-test.js

# Check metrics
# - p95 latency < 500ms
# - Error rate < 0.1%
# - Memory stable
```

---

### Task 4.3: Configuration Validation
**Time**: 2 hours  
**Priority**: 🔴 CRITICAL

**Steps**:
1. Add `--validate-config` flag to server
2. Test all configuration combinations
3. Document required environment variables
4. Create deployment checklist

**Code Changes**:
```bash
# Files to modify
cmd/server/main.go
docs/DEPLOYMENT.md (new)
```

**Verification**:
```bash
# Validate configuration
./server --validate-config

# Should fail with helpful errors
TLS_ENABLED=false ENVIRONMENT=production ./server --validate-config
```

---

## Day 5: Documentation & Final Verification (8 hours)

### Task 5.1: Update Documentation
**Time**: 3 hours  
**Priority**: 🔴 CRITICAL

**Steps**:
1. Update `README.md` with security requirements
2. Create `docs/SECURITY.md` with security guidelines
3. Update `GETTING_STARTED.md` with environment variables
4. Create deployment runbook

**Code Changes**:
```bash
# Files to modify/create
README.md
docs/SECURITY.md
GETTING_STARTED.md
docs/DEPLOYMENT_RUNBOOK.md (new)
```

---

### Task 5.2: End-to-End Testing
**Time**: 3 hours  
**Priority**: 🔴 CRITICAL

**Steps**:
1. Deploy to staging environment
2. Run full test suite
3. Perform manual security testing
4. Verify all critical issues fixed
5. Document any remaining issues

**Verification Checklist**:
- [ ] Server starts with valid configuration
- [ ] Server fails to start with invalid Keycloak URL
- [ ] Server rejects default secrets
- [ ] HTTP rejected in production mode
- [ ] No panics in error handling
- [ ] HTTP clients have timeouts
- [ ] Database connection limits enforced
- [ ] Rate limiting works with Redis
- [ ] Audit logs written for security events
- [ ] All tests pass with `-race`
- [ ] Load test passes (1000 users)
- [ ] Security scan shows no CRITICAL issues

---

### Task 5.3: Create Release Notes
**Time**: 2 hours  
**Priority**: 🔴 CRITICAL

**Steps**:
1. Document all changes made
2. List breaking changes
3. Update migration guide
4. Create deployment checklist

**Code Changes**:
```bash
# Files to create
CHANGELOG.md
docs/MIGRATION_GUIDE.md (new)
docs/DEPLOYMENT_CHECKLIST.md (new)
```

---

## Success Criteria

### All Tests Pass
```bash
# Unit tests
go test -race -cover ./...

# Security tests
go test -v ./tests/security/...

# Load tests
k6 run --vus 1000 --duration 10m tests/load/load-test.js
```

### Security Scan Clean
```bash
# No CRITICAL issues
gosec ./...
trivy fs .
```

### Configuration Validated
```bash
# All required environment variables documented
# Default secrets rejected
# TLS enforced in production
./server --validate-config
```

### Audit Logging Working
```bash
# All security events logged
psql -c "SELECT COUNT(*) FROM audit_logs WHERE created_at > NOW() - INTERVAL '1 hour'"
```

---

## Rollback Plan

If any critical issue is discovered:

1. **Stop deployment** immediately
2. **Document the issue** in GitHub issue
3. **Revert changes** if necessary
4. **Fix the issue** before proceeding
5. **Re-test** before continuing

---

## Daily Standup Template

### What I did yesterday
- [ ] Task completed
- [ ] Tests written
- [ ] Documentation updated

### What I'm doing today
- [ ] Current task
- [ ] Expected completion time

### Blockers
- [ ] Any issues preventing progress

---

## End of Week 1 Deliverables

1. ✅ All CRITICAL issues fixed
2. ✅ Security test suite passing
3. ✅ Load tests passing
4. ✅ Documentation updated
5. ✅ Deployment checklist created
6. ✅ Configuration validated
7. ✅ Audit logging implemented
8. ✅ Rate limiting working

**Status**: 🟢 **READY FOR WEEK 2** (High priority fixes)

---

## Next Steps (Week 2)

See `WEEK_2_ACTION_PLAN.md` for:
- Health checks for dependencies
- CORS fixes
- Token blacklist
- Enterprise isolation audit
- Metrics endpoint
- Error message sanitization

---

**Remember**: Quality over speed. It's better to take an extra day and do it right than to rush and introduce new vulnerabilities.
