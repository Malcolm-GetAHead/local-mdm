# Test & Verification Plan

**Purpose**: Verify all fixes before production deployment  
**Timeline**: 2 days (parallel with implementation)  
**Owner**: QA Team + DevOps

---

## Test Categories

### 1. Unit Tests (Automated)
**Coverage Target**: 80%+  
**Execution**: On every commit

```bash
# Run all tests with race detection
go test -race -cover ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html

# Verify coverage threshold
go test -cover ./... | awk '/coverage:/ {if ($2 < 80.0) exit 1}'
```

**Critical Paths to Test**:
- [ ] Authentication flow (token validation, role checks)
- [ ] Rate limiting (IP-based, account-based)
- [ ] Circuit breaker (open, half-open, closed states)
- [ ] Error sanitization (no internal details leaked)
- [ ] Pagination validation (max limits enforced)
- [ ] JSONB validation (size, depth limits)
- [ ] Audit logging (async, queue overflow)
- [ ] Database transactions (isolation, rollback)

---

### 2. Integration Tests (Automated)
**Execution**: On every PR

```bash
# Start test dependencies
docker-compose -f docker-compose.test.yml up -d

# Wait for services
sleep 30

# Run integration tests
go test -tags=integration ./tests/integration/...

# Cleanup
docker-compose -f docker-compose.test.yml down
```

**Test Scenarios**:

#### Authentication Integration
```go
func TestAuthenticationFlow(t *testing.T) {
    // 1. Login with valid credentials
    token, err := client.Login("admin@example.com", "password")
    require.NoError(t, err)
    
    // 2. Access protected endpoint
    devices, err := client.ListDevices(token)
    require.NoError(t, err)
    
    // 3. Login with invalid credentials
    _, err = client.Login("admin@example.com", "wrong")
    require.Error(t, err)
    
    // 4. Verify rate limiting after 10 attempts
    for i := 0; i < 10; i++ {
        client.Login("admin@example.com", "wrong")
    }
    _, err = client.Login("admin@example.com", "wrong")
    require.Equal(t, http.StatusTooManyRequests, err.StatusCode)
}
```

#### Circuit Breaker Integration
```go
func TestCircuitBreakerKeycloak(t *testing.T) {
    // 1. Authenticate successfully
    token, err := client.Login("admin@example.com", "password")
    require.NoError(t, err)
    
    // 2. Stop Keycloak
    stopKeycloak()
    
    // 3. Verify cached token still works
    devices, err := client.ListDevices(token)
    require.NoError(t, err)
    
    // 4. Verify new logins fail gracefully
    _, err = client.Login("user@example.com", "password")
    require.Error(t, err)
    require.Contains(t, err.Error(), "service unavailable")
    
    // 5. Restart Keycloak
    startKeycloak()
    time.Sleep(30 * time.Second)
    
    // 6. Verify service recovers
    _, err = client.Login("user@example.com", "password")
    require.NoError(t, err)
}
```

#### Database Backup/Restore
```go
func TestDatabaseBackupRestore(t *testing.T) {
    // 1. Create test data
    enterprise := createTestEnterprise(t)
    device := createTestDevice(t, enterprise.ID)
    
    // 2. Perform backup
    backupFile, err := runBackup()
    require.NoError(t, err)
    
    // 3. Modify data
    deleteDevice(t, device.ID)
    
    // 4. Restore from backup
    err = runRestore(backupFile)
    require.NoError(t, err)
    
    // 5. Verify data restored
    restored, err := getDevice(t, device.ID)
    require.NoError(t, err)
    require.Equal(t, device.Name, restored.Name)
}
```

---

### 3. Load Tests (Manual + Automated)
**Tool**: k6 or Apache JMeter  
**Execution**: Before production deployment

```javascript
// load-test.js
import http from 'k6/http';
import { check, sleep } from 'k6';

export let options = {
    stages: [
        { duration: '2m', target: 100 },  // Ramp up to 100 users
        { duration: '5m', target: 100 },  // Stay at 100 users
        { duration: '2m', target: 500 },  // Ramp up to 500 users
        { duration: '5m', target: 500 },  // Stay at 500 users
        { duration: '2m', target: 1000 }, // Ramp up to 1000 users
        { duration: '5m', target: 1000 }, // Stay at 1000 users
        { duration: '2m', target: 0 },    // Ramp down
    ],
    thresholds: {
        http_req_duration: ['p(95)<500', 'p(99)<1000'],
        http_req_failed: ['rate<0.01'],
    },
};

export default function() {
    // Login
    let loginRes = http.post('http://localhost:8080/api/v1/auth/login', JSON.stringify({
        username: 'admin@example.com',
        password: 'password',
    }), {
        headers: { 'Content-Type': 'application/json' },
    });
    
    check(loginRes, {
        'login successful': (r) => r.status === 200,
    });
    
    let token = loginRes.json('data.access_token');
    
    // List devices
    let devicesRes = http.get('http://localhost:8080/api/v1/devices', {
        headers: { 'Authorization': `Bearer ${token}` },
    });
    
    check(devicesRes, {
        'devices listed': (r) => r.status === 200,
    });
    
    sleep(1);
}
```

**Run Load Test**:
```bash
k6 run --vus 1000 --duration 30m load-test.js
```

**Success Criteria**:
- [ ] 1000 concurrent users sustained
- [ ] P95 latency < 500ms
- [ ] P99 latency < 1000ms
- [ ] Error rate < 1%
- [ ] No memory leaks
- [ ] Connection pool stable
- [ ] Database queries < 100ms p95

---

### 4. Security Tests (Manual + Automated)
**Tool**: OWASP ZAP, Burp Suite  
**Execution**: Before production deployment

#### Automated Security Scan
```bash
# Run OWASP ZAP baseline scan
docker run -t owasp/zap2docker-stable zap-baseline.py \
    -t http://localhost:8080 \
    -r zap-report.html

# Run Trivy vulnerability scan
trivy image local-mdm:latest

# Run gosec static analysis
gosec ./...
```

#### Manual Security Tests

**Authentication Bypass**:
```bash
# Test 1: Missing token
curl -X GET http://localhost:8080/api/v1/devices
# Expected: 401 Unauthorized

# Test 2: Invalid token
curl -X GET http://localhost:8080/api/v1/devices \
    -H "Authorization: Bearer invalid"
# Expected: 401 Unauthorized

# Test 3: Expired token
curl -X GET http://localhost:8080/api/v1/devices \
    -H "Authorization: Bearer <expired-token>"
# Expected: 401 Unauthorized

# Test 4: Tampered token
curl -X GET http://localhost:8080/api/v1/devices \
    -H "Authorization: Bearer <tampered-token>"
# Expected: 401 Unauthorized
```

**SQL Injection**:
```bash
# Test 1: Device name injection
curl -X POST http://localhost:8080/api/v1/devices \
    -H "Authorization: Bearer $TOKEN" \
    -d '{"name": "test'; DROP TABLE devices; --"}'
# Expected: Device created with literal name (no SQL execution)

# Test 2: UUID injection
curl -X GET "http://localhost:8080/api/v1/devices/'; DROP TABLE devices; --"
# Expected: 400 Bad Request (invalid UUID)
```

**Rate Limiting**:
```bash
# Test 1: Brute force protection
for i in {1..15}; do
    curl -X POST http://localhost:8080/api/v1/auth/login \
        -d '{"username":"admin","password":"wrong"}'
done
# Expected: 429 Too Many Requests after 10 attempts

# Test 2: Rate limit bypass attempt
for i in {1..15}; do
    curl -X POST http://localhost:8080/api/v1/auth/login \
        -H "X-Forwarded-For: 1.2.3.$i" \
        -d '{"username":"admin","password":"wrong"}'
done
# Expected: Each IP gets 10 attempts (no bypass)
```

**SSRF Protection**:
```bash
# Test 1: Internal IP
# (Requires Keycloak config access - test in staging)
# Set JWKS URL to http://169.254.169.254/latest/meta-data/
# Expected: Server refuses to start (JWKS URL validation fails)

# Test 2: Private IP
# Set JWKS URL to http://192.168.1.1/
# Expected: Server refuses to start
```

**Error Message Disclosure**:
```bash
# Test 1: Invalid UUID
curl -X GET http://localhost:8080/api/v1/devices/invalid-uuid
# Expected: Generic error (no database details)

# Test 2: Non-existent resource
curl -X GET http://localhost:8080/api/v1/devices/00000000-0000-0000-0000-000000000000
# Expected: "device not found" (no SQL error)

# Test 3: Database error
# (Stop database, make request)
curl -X GET http://localhost:8080/api/v1/devices
# Expected: "internal error" (no connection string)
```

---

### 5. Chaos Tests (Manual)
**Tool**: Chaos Mesh or manual  
**Execution**: In staging environment

#### Database Failure
```bash
# 1. Start load test
k6 run --vus 100 --duration 10m load-test.js &

# 2. Kill database connection
docker-compose stop postgres

# 3. Observe behavior
# Expected: 
# - Health check fails
# - Requests return 503 Service Unavailable
# - No panics or crashes
# - Logs show connection errors

# 4. Restart database
docker-compose start postgres

# 5. Verify recovery
# Expected:
# - Service reconnects within 30s
# - Health check passes
# - Requests succeed
```

#### Keycloak Failure
```bash
# 1. Authenticate and cache tokens
curl -X POST http://localhost:8080/api/v1/auth/login \
    -d '{"username":"admin","password":"password"}'

# 2. Stop Keycloak
docker-compose stop keycloak

# 3. Test cached token
curl -X GET http://localhost:8080/api/v1/devices \
    -H "Authorization: Bearer $TOKEN"
# Expected: Request succeeds (cached validation)

# 4. Test new login
curl -X POST http://localhost:8080/api/v1/auth/login \
    -d '{"username":"user","password":"password"}'
# Expected: 503 Service Unavailable (circuit breaker open)

# 5. Restart Keycloak
docker-compose start keycloak

# 6. Wait for circuit breaker to close (30s)
sleep 30

# 7. Test new login
curl -X POST http://localhost:8080/api/v1/auth/login \
    -d '{"username":"user","password":"password"}'
# Expected: Login succeeds (circuit breaker closed)
```

#### Disk Space Exhaustion
```bash
# 1. Fill disk to 95%
dd if=/dev/zero of=/tmp/fillfile bs=1M count=10000

# 2. Attempt to write audit logs
curl -X POST http://localhost:8080/api/v1/devices \
    -H "Authorization: Bearer $TOKEN" \
    -d '{"name":"test"}'

# Expected:
# - Request succeeds (audit log dropped)
# - Warning logged about disk space
# - Alert triggered

# 3. Clean up
rm /tmp/fillfile
```

#### Pod Restart
```bash
# 1. Start load test
k6 run --vus 100 --duration 5m load-test.js &

# 2. Kill pod
kubectl delete pod local-mdm-xxx

# 3. Observe behavior
# Expected:
# - Some requests fail (503)
# - New pod starts within 30s
# - Load balancer routes to new pod
# - Error rate < 5%
```

---

### 6. Backup/Restore Tests (Manual)
**Execution**: Weekly in staging, before production deployment

#### Full Backup/Restore
```bash
# 1. Create test data
./scripts/create-test-data.sh

# 2. Record checksums
psql -c "SELECT COUNT(*) FROM enterprises" > /tmp/before.txt
psql -c "SELECT COUNT(*) FROM devices" >> /tmp/before.txt
psql -c "SELECT COUNT(*) FROM audit_logs" >> /tmp/before.txt

# 3. Perform backup
./scripts/backup-database.sh

# 4. Verify backup file
ls -lh /var/backups/local-mdm/
pg_restore --list /var/backups/local-mdm/latest.sql.gz

# 5. Drop database
psql -c "DROP DATABASE localmdm"

# 6. Restore from backup
./scripts/restore-database.sh /var/backups/local-mdm/latest.sql.gz

# 7. Verify data
psql -c "SELECT COUNT(*) FROM enterprises" > /tmp/after.txt
psql -c "SELECT COUNT(*) FROM devices" >> /tmp/after.txt
psql -c "SELECT COUNT(*) FROM audit_logs" >> /tmp/after.txt

# 8. Compare checksums
diff /tmp/before.txt /tmp/after.txt
# Expected: No differences
```

#### Point-in-Time Recovery
```bash
# 1. Note current time
RESTORE_TIME=$(date -u +"%Y-%m-%d %H:%M:%S")

# 2. Create data before restore point
./scripts/create-test-data.sh

# 3. Wait 1 minute
sleep 60

# 4. Create data after restore point (to be lost)
./scripts/create-more-test-data.sh

# 5. Restore to point in time
./scripts/restore-database.sh --time "$RESTORE_TIME"

# 6. Verify only data before restore point exists
# Expected: First dataset present, second dataset absent
```

---

### 7. Migration Tests (Automated)
**Execution**: On every migration change

```bash
# Test forward migration
migrate -path migrations -database "$DATABASE_URL" up

# Verify schema
psql -c "\d+ audit_logs"

# Test rollback
migrate -path migrations -database "$DATABASE_URL" down 1

# Verify schema reverted
psql -c "\d+ audit_logs"

# Test idempotency (run twice)
migrate -path migrations -database "$DATABASE_URL" up
migrate -path migrations -database "$DATABASE_URL" up
# Expected: Second run is no-op
```

---

## Verification Checklist

### Pre-Deployment
- [ ] All unit tests pass
- [ ] All integration tests pass
- [ ] Load test passes (1000 users, 30 min)
- [ ] Security scan passes (no high/critical issues)
- [ ] Chaos tests pass (database failure, Keycloak failure)
- [ ] Backup/restore tested successfully
- [ ] Migration rollback tested
- [ ] Code coverage > 80%
- [ ] No race conditions detected
- [ ] Performance benchmarks meet targets

### Post-Deployment (Staging)
- [ ] Smoke tests pass
- [ ] Health check returns healthy
- [ ] Authentication works
- [ ] Rate limiting works
- [ ] Circuit breaker works
- [ ] Audit logs written
- [ ] Metrics exported
- [ ] Traces visible in Jaeger
- [ ] Alerts configured
- [ ] Backup automation works

### Post-Deployment (Production)
- [ ] Canary deployment successful (10% traffic)
- [ ] Error rate < 0.1%
- [ ] P99 latency < 1s
- [ ] No critical errors in logs
- [ ] All services healthy
- [ ] Gradual rollout to 100%
- [ ] Monitor for 24 hours
- [ ] Post-deployment review completed

---

## Rollback Verification

### Trigger Rollback If:
- Error rate > 5%
- P99 latency > 5s
- Health check fails
- Critical security issue
- Data corruption detected

### Rollback Procedure:
```bash
# 1. Stop new deployments
kubectl rollout pause deployment/local-mdm

# 2. Rollback to previous version
kubectl rollout undo deployment/local-mdm

# 3. Verify rollback
kubectl rollout status deployment/local-mdm

# 4. Run smoke tests
./scripts/smoke-test.sh

# 5. Monitor for 30 minutes
# Expected: Error rate < 0.1%, latency normal

# 6. Investigate root cause
# 7. Fix and redeploy
```

---

## Continuous Testing

### Daily (Automated)
- Unit tests on every commit
- Integration tests on every PR
- Security scan on every build
- Dependency vulnerability scan

### Weekly (Automated)
- Load test in staging
- Backup/restore test
- Chaos test (database failure)
- Performance regression test

### Monthly (Manual)
- Full security audit
- Penetration testing
- Disaster recovery drill
- Compliance review

### Quarterly (Manual)
- External security audit
- Load test in production (off-hours)
- Full disaster recovery test
- Architecture review
