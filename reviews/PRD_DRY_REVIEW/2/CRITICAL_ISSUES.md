# Critical Issues (Must Fix Before Production)

**Priority**: CRITICAL  
**Total Issues**: 3  
**Estimated Effort**: 2-3 days  
**Risk Level**: Production blockers

---

## C-01: CA Private Key Stored on Filesystem

**Severity**: CRITICAL (for production)  
**Category**: Security  
**Impact**: Complete security breach if server compromised  
**Effort**: 1 day  
**Status**: ⚠️ **DEFERRED TO F-03** (Advanced Security - post-v1.0)

### Problem
The CA private key is stored on the filesystem in `secrets/ca.key` with only file permissions (0600) protecting it.

**Location**: `internal/certs/ca.go:23-27`

### Current Assessment
**For local POC/development**: ✅ **ACCEPTABLE**
- File permissions (0600) adequate for local development
- Documented in `secrets/README.md` as dev-only approach
- Explicitly marked for AWS Secrets Manager in production (F-03)

**For production deployment**: ❌ **MUST FIX**
- Move to AWS Secrets Manager or HSM
- Covered in F-03: Advanced Security Features
- Timeline: Post-v1.0 (before production deployment)

### Recommendation
**No action required for v1.0 POC**. This is intentionally deferred to F-03 (Advanced Security) which includes:
- HSM integration (AWS CloudHSM or PKCS#11)
- AWS Secrets Manager integration
- Key rotation procedures

See `docs/tasks/future/F-03-advanced-security.md` for complete implementation plan.

---

## C-02: No Rate Limiting on Authentication Endpoints

**Severity**: CRITICAL  
**Category**: Security  
**Impact**: Brute force attacks, credential stuffing, DoS  
**Effort**: 0.5 days

### Problem
The `/api/v1/auth/login` endpoint has no rate limiting, allowing unlimited authentication attempts. An attacker can:
- Brute force user passwords
- Perform credential stuffing attacks
- DoS the authentication service
- Exhaust Keycloak connection pool

**Location**: `internal/api/server.go:71-72`

```go
// Auth routes (no auth required)
api.HandleFunc("/auth/login", s.handleLogin).Methods("POST")  // ❌ No rate limit
api.HandleFunc("/auth/refresh", s.handleRefresh).Methods("POST")
```

### Exploit Scenario
1. Attacker obtains list of email addresses (data breach, OSINT)
2. Runs credential stuffing attack with 1000 req/sec
3. Tries common passwords against all accounts
4. Gains access to admin accounts
5. Compromises entire MDM infrastructure

### Fix
Add aggressive rate limiting to authentication endpoints.

```go
// internal/api/server.go
func (s *Server) setupRoutes() {
    // ... existing code ...
    
    // Auth routes with strict rate limiting
    authLimiter := newRateLimiter(10, time.Minute) // 10 attempts per minute per IP
    api.Handle("/auth/login", 
        rateLimitMiddleware(authLimiter)(
            http.HandlerFunc(s.handleLogin),
        ),
    ).Methods("POST")
    
    api.Handle("/auth/refresh",
        rateLimitMiddleware(authLimiter)(
            http.HandlerFunc(s.handleRefresh),
        ),
    ).Methods("POST")
}
```

**Enhanced rate limiter with account lockout**:

```go
// internal/api/auth_ratelimit.go
package api

import (
    "net/http"
    "sync"
    "time"
)

type authRateLimiter struct {
    ipAttempts      map[string]*attemptTracker
    accountAttempts map[string]*attemptTracker
    mu              sync.RWMutex
}

type attemptTracker struct {
    count      int
    firstAttempt time.Time
    lockedUntil  time.Time
}

func newAuthRateLimiter() *authRateLimiter {
    return &authRateLimiter{
        ipAttempts:      make(map[string]*attemptTracker),
        accountAttempts: make(map[string]*attemptTracker),
    }
}

func (rl *authRateLimiter) checkAndRecord(ip, account string) (allowed bool, retryAfter time.Duration) {
    rl.mu.Lock()
    defer rl.mu.Unlock()
    
    now := time.Now()
    
    // Check IP-based rate limit (10 attempts per minute)
    if tracker, exists := rl.ipAttempts[ip]; exists {
        if now.Before(tracker.lockedUntil) {
            return false, tracker.lockedUntil.Sub(now)
        }
        
        if now.Sub(tracker.firstAttempt) < time.Minute {
            tracker.count++
            if tracker.count > 10 {
                tracker.lockedUntil = now.Add(5 * time.Minute)
                return false, 5 * time.Minute
            }
        } else {
            tracker.count = 1
            tracker.firstAttempt = now
        }
    } else {
        rl.ipAttempts[ip] = &attemptTracker{
            count:        1,
            firstAttempt: now,
        }
    }
    
    // Check account-based rate limit (5 attempts per 5 minutes)
    if tracker, exists := rl.accountAttempts[account]; exists {
        if now.Before(tracker.lockedUntil) {
            return false, tracker.lockedUntil.Sub(now)
        }
        
        if now.Sub(tracker.firstAttempt) < 5*time.Minute {
            tracker.count++
            if tracker.count > 5 {
                tracker.lockedUntil = now.Add(15 * time.Minute)
                return false, 15 * time.Minute
            }
        } else {
            tracker.count = 1
            tracker.firstAttempt = now
        }
    } else {
        rl.accountAttempts[account] = &attemptTracker{
            count:        1,
            firstAttempt: now,
        }
    }
    
    return true, 0
}

func authRateLimitMiddleware(limiter *authRateLimiter) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            ip := getClientIP(r)
            
            // Extract account from request body (requires buffering)
            var req struct {
                Username string `json:"username"`
            }
            // ... parse request body ...
            
            allowed, retryAfter := limiter.checkAndRecord(ip, req.Username)
            if !allowed {
                w.Header().Set("Retry-After", fmt.Sprintf("%d", int(retryAfter.Seconds())))
                respondError(w, r, http.StatusTooManyRequests, "rate_limit_exceeded", 
                    fmt.Sprintf("Too many login attempts. Try again in %v", retryAfter))
                return
            }
            
            next.ServeHTTP(w, r)
        })
    }
}
```

### Verification
1. Test 10 failed logins from same IP - should block
2. Test 5 failed logins to same account - should block
3. Verify Retry-After header is set
4. Test successful login resets counter
5. Load test with 1000 req/sec - should throttle

---

## C-03: No Database Backup/Restore Procedures

**Severity**: CRITICAL (for production)  
**Category**: Reliability  
**Impact**: Data loss, no disaster recovery  
**Effort**: 1 day  
**Status**: ⚠️ **DEFERRED TO F-04** (Disaster Recovery - post-v1.0)

### Problem
There are no documented or automated database backup procedures.

**Location**: Missing from `docs/operations/`

### Current Assessment
**For local POC/development**: ✅ **ACCEPTABLE**
- Local development uses Docker Compose with volume mounts
- Data can be recreated from migrations
- No production data at risk

**For production deployment**: ❌ **MUST FIX**
- Automated backup/restore required
- Covered in F-04: Disaster Recovery & Business Continuity
- Timeline: Post-v1.0 (before production deployment)

### Recommendation
**No action required for v1.0 POC**. This is intentionally deferred to F-04 (Disaster Recovery) which includes:
- Automated backup scripts with S3 upload
- Point-in-time recovery procedures
- Backup verification and testing
- Disaster recovery runbooks

See `docs/tasks/future/F-04-disaster-recovery.md` for complete implementation plan.

### Temporary Workaround for Development
For local development, manual backups can be performed:

**Backup Script**:
```bash
#!/bin/bash
# scripts/backup-database.sh

set -euo pipefail

BACKUP_DIR="/var/backups/local-mdm"
RETENTION_DAYS=30
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="${BACKUP_DIR}/localmdm_${TIMESTAMP}.sql.gz"

# Create backup directory
mkdir -p "${BACKUP_DIR}"

# Perform backup
pg_dump -h "${DB_HOST}" -U "${DB_USER}" -d "${DB_NAME}" \
    --format=custom \
    --compress=9 \
    --file="${BACKUP_FILE}"

# Upload to S3
aws s3 cp "${BACKUP_FILE}" "s3://${BACKUP_BUCKET}/database/${TIMESTAMP}/"

# Verify backup
pg_restore --list "${BACKUP_FILE}" > /dev/null

# Clean old backups
find "${BACKUP_DIR}" -name "*.sql.gz" -mtime +${RETENTION_DAYS} -delete

echo "Backup completed: ${BACKUP_FILE}"
```

**Restore Script**:
```bash
#!/bin/bash
# scripts/restore-database.sh

set -euo pipefail

BACKUP_FILE="$1"

if [ -z "${BACKUP_FILE}" ]; then
    echo "Usage: $0 <backup-file>"
    exit 1
fi

# Download from S3 if needed
if [[ "${BACKUP_FILE}" == s3://* ]]; then
    LOCAL_FILE="/tmp/restore_$(date +%s).sql.gz"
    aws s3 cp "${BACKUP_FILE}" "${LOCAL_FILE}"
    BACKUP_FILE="${LOCAL_FILE}"
fi

# Verify backup integrity
pg_restore --list "${BACKUP_FILE}" > /dev/null

# Create restore database
psql -h "${DB_HOST}" -U "${DB_USER}" -c "DROP DATABASE IF EXISTS ${DB_NAME}_restore;"
psql -h "${DB_HOST}" -U "${DB_USER}" -c "CREATE DATABASE ${DB_NAME}_restore;"

# Restore
pg_restore -h "${DB_HOST}" -U "${DB_USER}" -d "${DB_NAME}_restore" \
    --no-owner \
    --no-acl \
    "${BACKUP_FILE}"

echo "Restore completed to ${DB_NAME}_restore"
echo "Verify data, then rename database to activate"
```

**Automated Backup (Kubernetes CronJob)**:
```yaml
# k8s/cronjob-backup.yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: database-backup
spec:
  schedule: "0 2 * * *"  # 2 AM daily
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: backup
            image: postgres:15
            command: ["/scripts/backup-database.sh"]
            env:
            - name: DB_HOST
              valueFrom:
                secretKeyRef:
                  name: database-credentials
                  key: host
            - name: DB_USER
              valueFrom:
                secretKeyRef:
                  name: database-credentials
                  key: username
            - name: DB_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: database-credentials
                  key: password
            - name: BACKUP_BUCKET
              value: "local-mdm-backups"
            volumeMounts:
            - name: scripts
              mountPath: /scripts
          volumes:
          - name: scripts
            configMap:
              name: backup-scripts
              defaultMode: 0755
          restartPolicy: OnFailure
```

### Verification
1. Run backup script manually
2. Verify backup file created
3. Verify backup uploaded to S3
4. Run restore script to test database
5. Verify all data restored correctly
6. Test point-in-time recovery
7. Document restore procedure in runbook

### References
- PostgreSQL Backup Best Practices
- AWS RDS Automated Backups
- Disaster Recovery Planning
