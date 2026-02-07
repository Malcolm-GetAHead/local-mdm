# High Priority Issues (Should Fix Before Production)

**Priority**: HIGH  
**Total Issues**: 8  
**Estimated Effort**: 3-4 days  
**Risk Level**: Significant operational/security concerns

---

## H-01: No Circuit Breaker for Keycloak Dependency

**Severity**: HIGH  
**Category**: Reliability  
**Impact**: Complete service outage when Keycloak is down  
**Effort**: 0.5 days

### Problem
The authentication system has a hard dependency on Keycloak with no circuit breaker. If Keycloak becomes unavailable:
- All authentication requests fail immediately
- No graceful degradation
- Service becomes completely unusable
- No cached token validation fallback

**Location**: `internal/auth/oidc.go:52-60`, `internal/api/handlers.go:52-58`

### Fix
Implement circuit breaker pattern with cached token validation.

```go
// internal/auth/circuit_breaker.go
package auth

import (
    "context"
    "errors"
    "sync"
    "time"
)

type CircuitState int

const (
    StateClosed CircuitState = iota
    StateOpen
    StateHalfOpen
)

type CircuitBreaker struct {
    maxFailures  int
    timeout      time.Duration
    state        CircuitState
    failures     int
    lastFailTime time.Time
    mu           sync.RWMutex
}

func NewCircuitBreaker(maxFailures int, timeout time.Duration) *CircuitBreaker {
    return &CircuitBreaker{
        maxFailures: maxFailures,
        timeout:     timeout,
        state:       StateClosed,
    }
}

func (cb *CircuitBreaker) Call(fn func() error) error {
    cb.mu.RLock()
    state := cb.state
    cb.mu.RUnlock()
    
    if state == StateOpen {
        if time.Since(cb.lastFailTime) > cb.timeout {
            cb.mu.Lock()
            cb.state = StateHalfOpen
            cb.mu.Unlock()
        } else {
            return errors.New("circuit breaker is open")
        }
    }
    
    err := fn()
    
    cb.mu.Lock()
    defer cb.mu.Unlock()
    
    if err != nil {
        cb.failures++
        cb.lastFailTime = time.Now()
        
        if cb.failures >= cb.maxFailures {
            cb.state = StateOpen
        }
        return err
    }
    
    // Success - reset
    if cb.state == StateHalfOpen {
        cb.state = StateClosed
    }
    cb.failures = 0
    
    return nil
}

// Enhanced OIDCValidator with circuit breaker
type OIDCValidator struct {
    // ... existing fields ...
    circuitBreaker *CircuitBreaker
    tokenCache     *TokenCache
}

func NewOIDCValidator(issuerURL, clientID string) (*OIDCValidator, error) {
    v := &OIDCValidator{
        issuerURL:      issuerURL,
        clientID:       clientID,
        jwksURL:        fmt.Sprintf("%s/protocol/openid-connect/certs", issuerURL),
        refreshEvery:   1 * time.Hour,
        circuitBreaker: NewCircuitBreaker(5, 30*time.Second),
        tokenCache:     NewTokenCache(10000, 5*time.Minute),
    }
    
    if err := v.refreshJWKS(); err != nil {
        return nil, fmt.Errorf("failed to fetch JWKS: %w", err)
    }
    
    return v, nil
}

func (v *OIDCValidator) ValidateToken(tokenString string) (*AuthUser, error) {
    // Check cache first
    if user, found := v.tokenCache.Get(tokenString); found {
        return user, nil
    }
    
    // Try validation with circuit breaker
    var user *AuthUser
    err := v.circuitBreaker.Call(func() error {
        var err error
        user, err = v.validateTokenInternal(tokenString)
        return err
    })
    
    if err != nil {
        // Circuit breaker open - try cached validation
        if errors.Is(err, errors.New("circuit breaker is open")) {
            return v.validateTokenCached(tokenString)
        }
        return nil, err
    }
    
    // Cache successful validation
    v.tokenCache.Set(tokenString, user)
    
    return user, nil
}

// Token cache for graceful degradation
type TokenCache struct {
    cache    map[string]*cacheEntry
    maxSize  int
    ttl      time.Duration
    mu       sync.RWMutex
}

type cacheEntry struct {
    user      *AuthUser
    expiresAt time.Time
}

func NewTokenCache(maxSize int, ttl time.Duration) *TokenCache {
    return &TokenCache{
        cache:   make(map[string]*cacheEntry),
        maxSize: maxSize,
        ttl:     ttl,
    }
}

func (tc *TokenCache) Get(token string) (*AuthUser, bool) {
    tc.mu.RLock()
    defer tc.mu.RUnlock()
    
    entry, exists := tc.cache[token]
    if !exists || time.Now().After(entry.expiresAt) {
        return nil, false
    }
    
    return entry.user, true
}

func (tc *TokenCache) Set(token string, user *AuthUser) {
    tc.mu.Lock()
    defer tc.mu.Unlock()
    
    // Evict oldest if at capacity
    if len(tc.cache) >= tc.maxSize {
        var oldestToken string
        var oldestTime time.Time
        
        for token, entry := range tc.cache {
            if oldestTime.IsZero() || entry.expiresAt.Before(oldestTime) {
                oldestToken = token
                oldestTime = entry.expiresAt
            }
        }
        
        delete(tc.cache, oldestToken)
    }
    
    tc.cache[token] = &cacheEntry{
        user:      user,
        expiresAt: time.Now().Add(tc.ttl),
    }
}
```

### Verification
1. Start service with Keycloak running
2. Authenticate and cache tokens
3. Stop Keycloak
4. Verify cached tokens still work
5. Verify circuit breaker opens after 5 failures
6. Restart Keycloak
7. Verify circuit breaker closes and service recovers

---

## H-02: Error Messages Leak Internal Details

**Severity**: HIGH  
**Category**: Security  
**Impact**: Information disclosure aids attackers  
**Effort**: 0.5 days

### Problem
Error messages expose internal implementation details, database structure, and file paths.

**Examples**:
```go
// internal/repository/device.go:45
return nil, fmt.Errorf("failed to get device %s: %w", id, err)
// Exposes: "failed to get device <uuid>: pq: relation "devices" does not exist"

// internal/config/config.go:195
return fmt.Errorf("failed to read config file: %w", err)
// Exposes: "failed to read config file: open /etc/local-mdm/config.yaml: permission denied"
```

### Fix
Sanitize error messages for external responses.

```go
// internal/api/errors.go
package api

import (
    "errors"
    "fmt"
    "log/slog"
    "net/http"
)

type ErrorCode string

const (
    ErrCodeNotFound        ErrorCode = "not_found"
    ErrCodeUnauthorized    ErrorCode = "unauthorized"
    ErrCodeForbidden       ErrorCode = "forbidden"
    ErrCodeValidation      ErrorCode = "validation_failed"
    ErrCodeInternal        ErrorCode = "internal_error"
    ErrCodeServiceUnavailable ErrorCode = "service_unavailable"
)

type AppError struct {
    Code       ErrorCode
    Message    string
    Internal   error
    StatusCode int
}

func (e *AppError) Error() string {
    return e.Message
}

func NewNotFoundError(resource string) *AppError {
    return &AppError{
        Code:       ErrCodeNotFound,
        Message:    fmt.Sprintf("%s not found", resource),
        StatusCode: http.StatusNotFound,
    }
}

func NewValidationError(message string) *AppError {
    return &AppError{
        Code:       ErrCodeValidation,
        Message:    message,
        StatusCode: http.StatusBadRequest,
    }
}

func NewInternalError(err error) *AppError {
    return &AppError{
        Code:       ErrCodeInternal,
        Message:    "An internal error occurred",
        Internal:   err,
        StatusCode: http.StatusInternalServerError,
    }
}

func HandleError(w http.ResponseWriter, r *http.Request, err error, logger *slog.Logger) {
    var appErr *AppError
    if !errors.As(err, &appErr) {
        appErr = NewInternalError(err)
    }
    
    // Log internal error details
    if appErr.Internal != nil {
        requestID, _ := r.Context().Value(requestIDKey).(string)
        logger.Error("Request failed",
            "request_id", requestID,
            "error", appErr.Internal,
            "path", r.URL.Path,
        )
    }
    
    // Return sanitized error to client
    respondError(w, r, appErr.StatusCode, string(appErr.Code), appErr.Message)
}
```

**Update repository methods**:
```go
// internal/repository/device.go
func (r *deviceRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Device, error) {
    // ... query code ...
    
    if err == sql.ErrNoRows {
        return nil, NewNotFoundError("device")
    }
    if err != nil {
        return nil, NewInternalError(fmt.Errorf("database query failed: %w", err))
    }
    
    return device, nil
}
```

### Verification
1. Trigger various error conditions
2. Verify external responses don't leak internals
3. Verify internal errors are logged
4. Test with invalid UUIDs, missing resources, etc.

---

## H-03: No Graceful Degradation for Non-Critical Features

**Severity**: HIGH  
**Category**: Reliability  
**Impact**: Complete outage for partial failures  
**Effort**: 0.5 days

### Problem
If audit logging fails, the entire request fails. Non-critical features should degrade gracefully.

**Location**: `internal/auth/middleware.go:37-50`

```go
if m.auditLogger != nil {
    _ = m.auditLogger.Log(r.Context(), audit.Event{...})  // ❌ Ignores error but blocks
}
```

### Fix
Make audit logging asynchronous with buffering.

```go
// internal/audit/async_logger.go
package audit

import (
    "context"
    "log/slog"
    "time"
)

type AsyncLogger struct {
    logger     *Logger
    eventQueue chan Event
    slogger    *slog.Logger
}

func NewAsyncLogger(db *sql.DB, bufferSize int, logger *slog.Logger) *AsyncLogger {
    al := &AsyncLogger{
        logger:     NewLogger(db),
        eventQueue: make(chan Event, bufferSize),
        slogger:    logger,
    }
    
    // Start background workers
    for i := 0; i < 3; i++ {
        go al.worker()
    }
    
    return al
}

func (al *AsyncLogger) Log(ctx context.Context, event Event) {
    select {
    case al.eventQueue <- event:
        // Queued successfully
    default:
        // Queue full - log warning but don't block request
        al.slogger.Warn("Audit log queue full, dropping event",
            "action", event.Action,
            "resource_type", event.ResourceType,
        )
    }
}

func (al *AsyncLogger) worker() {
    for event := range al.eventQueue {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        
        if err := al.logger.Log(ctx, event); err != nil {
            al.slogger.Error("Failed to write audit log",
                "error", err,
                "action", event.Action,
            )
        }
        
        cancel()
    }
}

func (al *AsyncLogger) Close() error {
    close(al.eventQueue)
    return nil
}
```

### Verification
1. Fill audit log queue to capacity
2. Verify requests still succeed
3. Verify warnings logged
4. Stop database
5. Verify service continues (audit logs dropped)

---

## H-04: Missing Database Connection Retry on Startup

**Severity**: HIGH  
**Category**: Reliability  
**Impact**: Service fails to start if DB temporarily unavailable  
**Effort**: 0.25 days

### Problem
If the database is not ready when the service starts, it fails immediately with no retry.

**Location**: `cmd/server/main.go:47-51`

```go
database, err := db.New(cfg.Database)
if err != nil {
    logger.Error("Failed to connect to database", "error", err)
    os.Exit(1)  // ❌ No retry
}
```

### Fix
Add exponential backoff retry logic.

```go
// cmd/server/main.go
func connectDatabaseWithRetry(cfg config.DatabaseConfig, logger *slog.Logger) (*db.DB, error) {
    maxRetries := 10
    baseDelay := 1 * time.Second
    maxDelay := 30 * time.Second
    
    for attempt := 0; attempt < maxRetries; attempt++ {
        database, err := db.New(cfg)
        if err == nil {
            logger.Info("Database connection established", "attempt", attempt+1)
            return database, nil
        }
        
        if attempt == maxRetries-1 {
            return nil, fmt.Errorf("failed to connect after %d attempts: %w", maxRetries, err)
        }
        
        delay := time.Duration(1<<uint(attempt)) * baseDelay
        if delay > maxDelay {
            delay = maxDelay
        }
        
        logger.Warn("Database connection failed, retrying",
            "attempt", attempt+1,
            "max_retries", maxRetries,
            "retry_in", delay,
            "error", err,
        )
        
        time.Sleep(delay)
    }
    
    return nil, fmt.Errorf("unreachable")
}

func main() {
    // ... existing code ...
    
    logger.Info("Connecting to database", "host", cfg.Database.Host, "port", cfg.Database.Port)
    database, err := connectDatabaseWithRetry(cfg.Database, logger)
    if err != nil {
        logger.Error("Failed to connect to database", "error", err)
        os.Exit(1)
    }
    defer database.Close()
    
    // ... rest of main ...
}
```

### Verification
1. Start service with database stopped
2. Verify retry attempts logged
3. Start database during retry window
4. Verify service connects and starts successfully

---

## H-05: No Query Timeout Enforcement

**Severity**: HIGH  
**Category**: Performance  
**Impact**: Slow queries can exhaust connection pool  
**Effort**: 0.25 days

### Problem
While context timeouts exist, there's no database-level query timeout enforcement. A slow query can hold a connection indefinitely.

### Fix
Add statement timeout to all database connections.

```go
// internal/db/db.go
func New(cfg config.DatabaseConfig) (*DB, error) {
    // ... existing validation ...
    
    db, err := sql.Open("postgres", cfg.DSN())
    if err != nil {
        return nil, fmt.Errorf("failed to open database: %w", err)
    }
    
    // Configure connection pool
    db.SetMaxOpenConns(cfg.MaxOpenConns)
    db.SetMaxIdleConns(cfg.MaxIdleConns)
    db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
    db.SetConnMaxIdleTime(10 * time.Minute)
    
    // Set statement timeout for all connections
    queryTimeout := cfg.QueryTimeout
    if queryTimeout == 0 {
        queryTimeout = 30 * time.Second
    }
    
    _, err = db.Exec(fmt.Sprintf("SET statement_timeout = %d", queryTimeout.Milliseconds()))
    if err != nil {
        db.Close()
        return nil, fmt.Errorf("failed to set statement timeout: %w", err)
    }
    
    // Test connection
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    if err := db.PingContext(ctx); err != nil {
        db.Close()
        return nil, fmt.Errorf("failed to ping database: %w", err)
    }
    
    return &DB{db}, nil
}
```

**Configuration**:
```yaml
# configs/config.yaml
database:
  query_timeout: 30s  # Kill queries after 30 seconds
```

### Verification
1. Run slow query (e.g., `SELECT pg_sleep(60)`)
2. Verify query is killed after timeout
3. Verify connection is returned to pool
4. Monitor connection pool metrics

---

## H-06: Audit Logs Unbounded (No Archival)

**Severity**: HIGH  
**Category**: Performance  
**Impact**: Database growth, query performance degradation  
**Effort**: 0.5 days

### Problem
Audit logs grow indefinitely with no archival or partitioning strategy. Over time:
- Table size grows to hundreds of GB
- Queries slow down
- Backups take longer
- Disk space exhausted

### Fix
Implement audit log archival with table partitioning.

```sql
-- migrations/000002_audit_log_partitioning.up.sql

-- Convert audit_logs to partitioned table
BEGIN;

-- Rename existing table
ALTER TABLE audit_logs RENAME TO audit_logs_old;

-- Create partitioned table
CREATE TABLE audit_logs (
    id UUID DEFAULT gen_random_uuid(),
    enterprise_id UUID REFERENCES enterprises(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    action VARCHAR(100) NOT NULL,
    resource_type VARCHAR(50) NOT NULL,
    resource_id UUID,
    details JSONB DEFAULT '{}',
    ip_address INET,
    user_agent TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
) PARTITION BY RANGE (created_at);

-- Create partitions for current and next 3 months
CREATE TABLE audit_logs_2026_02 PARTITION OF audit_logs
    FOR VALUES FROM ('2026-02-01') TO ('2026-03-01');

CREATE TABLE audit_logs_2026_03 PARTITION OF audit_logs
    FOR VALUES FROM ('2026-03-01') TO ('2026-04-01');

CREATE TABLE audit_logs_2026_04 PARTITION OF audit_logs
    FOR VALUES FROM ('2026-04-01') TO ('2026-05-01');

-- Copy data from old table
INSERT INTO audit_logs SELECT * FROM audit_logs_old;

-- Drop old table
DROP TABLE audit_logs_old;

-- Create indexes on partitions
CREATE INDEX idx_audit_logs_enterprise_id ON audit_logs(enterprise_id);
CREATE INDEX idx_audit_logs_user_id ON audit_logs(user_id);
CREATE INDEX idx_audit_logs_action ON audit_logs(action);
CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at);

COMMIT;
```

**Archival script**:
```bash
#!/bin/bash
# scripts/archive-audit-logs.sh

RETENTION_DAYS=90
ARCHIVE_BUCKET="local-mdm-audit-archives"

# Find partitions older than retention period
OLD_PARTITIONS=$(psql -h "${DB_HOST}" -U "${DB_USER}" -d "${DB_NAME}" -t -c "
    SELECT tablename 
    FROM pg_tables 
    WHERE schemaname = 'public' 
    AND tablename LIKE 'audit_logs_%'
    AND tablename < 'audit_logs_' || to_char(NOW() - INTERVAL '${RETENTION_DAYS} days', 'YYYY_MM')
")

for partition in ${OLD_PARTITIONS}; do
    echo "Archiving partition: ${partition}"
    
    # Export to CSV
    psql -h "${DB_HOST}" -U "${DB_USER}" -d "${DB_NAME}" -c "
        COPY (SELECT * FROM ${partition}) TO STDOUT WITH CSV HEADER
    " | gzip > "/tmp/${partition}.csv.gz"
    
    # Upload to S3
    aws s3 cp "/tmp/${partition}.csv.gz" "s3://${ARCHIVE_BUCKET}/audit-logs/${partition}.csv.gz"
    
    # Drop partition
    psql -h "${DB_HOST}" -U "${DB_USER}" -d "${DB_NAME}" -c "DROP TABLE ${partition}"
    
    rm "/tmp/${partition}.csv.gz"
done
```

### Verification
1. Create partitions for current month
2. Insert test audit logs
3. Run archival script
4. Verify old partitions archived to S3
5. Verify old partitions dropped
6. Test query performance

---

## H-07: No Distributed Tracing

**Severity**: HIGH  
**Category**: Observability  
**Impact**: Difficult to debug production issues  
**Effort**: 1 day

### Problem
No distributed tracing makes it impossible to:
- Track requests across services
- Identify slow operations
- Debug production issues
- Understand system behavior under load

### Fix
Implement OpenTelemetry tracing.

```go
// internal/tracing/tracing.go
package tracing

import (
    "context"
    "fmt"
    
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
    "go.opentelemetry.io/otel/sdk/resource"
    "go.opentelemetry.io/otel/sdk/trace"
    semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

func InitTracer(serviceName, endpoint string) (*trace.TracerProvider, error) {
    ctx := context.Background()
    
    exporter, err := otlptracegrpc.New(ctx,
        otlptracegrpc.WithEndpoint(endpoint),
        otlptracegrpc.WithInsecure(),
    )
    if err != nil {
        return nil, fmt.Errorf("failed to create exporter: %w", err)
    }
    
    res, err := resource.New(ctx,
        resource.WithAttributes(
            semconv.ServiceName(serviceName),
            semconv.ServiceVersion("1.0.0"),
        ),
    )
    if err != nil {
        return nil, fmt.Errorf("failed to create resource: %w", err)
    }
    
    tp := trace.NewTracerProvider(
        trace.WithBatcher(exporter),
        trace.WithResource(res),
        trace.WithSampler(trace.AlwaysSample()),
    )
    
    otel.SetTracerProvider(tp)
    
    return tp, nil
}
```

**Add to server**:
```go
// cmd/server/main.go
import (
    "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func main() {
    // ... existing code ...
    
    // Initialize tracing
    if cfg.Tracing.Enabled {
        tp, err := tracing.InitTracer("local-mdm", cfg.Tracing.Endpoint)
        if err != nil {
            logger.Error("Failed to initialize tracing", "error", err)
        } else {
            defer tp.Shutdown(context.Background())
            logger.Info("Tracing initialized", "endpoint", cfg.Tracing.Endpoint)
        }
    }
    
    // ... rest of main ...
}

// internal/api/server.go
func (s *Server) setupMiddleware() {
    // Add tracing middleware first
    s.router.Use(otelhttp.NewMiddleware("local-mdm"))
    
    // ... existing middleware ...
}
```

### Verification
1. Deploy Jaeger or AWS X-Ray
2. Generate test traffic
3. View traces in UI
4. Verify spans for all operations
5. Test trace sampling

---

## H-08: No Pagination Limit Enforcement

**Severity**: HIGH  
**Category**: Performance  
**Impact**: Memory exhaustion, DoS  
**Effort**: 0.25 days

### Problem
Repository List methods accept arbitrary limit values. An attacker could request millions of records.

**Location**: `internal/repository/device.go:110-145`

```go
func (r *deviceRepository) List(ctx context.Context, enterpriseID uuid.UUID, limit, offset int) ([]*models.Device, int, error) {
    // ❌ No validation on limit
    query := `... LIMIT $2 OFFSET $3`
    rows, err := exec.QueryContext(ctx, query, enterpriseID, limit, offset)
```

### Fix
Enforce maximum pagination limits.

```go
// internal/repository/pagination.go
package repository

import "fmt"

const (
    MaxPageSize     = 1000
    DefaultPageSize = 100
)

func ValidatePagination(limit, offset int) (int, int, error) {
    if limit <= 0 {
        limit = DefaultPageSize
    }
    
    if limit > MaxPageSize {
        return 0, 0, fmt.Errorf("limit exceeds maximum of %d", MaxPageSize)
    }
    
    if offset < 0 {
        return 0, 0, fmt.Errorf("offset must be non-negative")
    }
    
    return limit, offset, nil
}

// Update all List methods
func (r *deviceRepository) List(ctx context.Context, enterpriseID uuid.UUID, limit, offset int) ([]*models.Device, int, error) {
    limit, offset, err := ValidatePagination(limit, offset)
    if err != nil {
        return nil, 0, fmt.Errorf("invalid pagination: %w", err)
    }
    
    // ... rest of method ...
}
```

### Verification
1. Request limit=1000000 - should return error
2. Request limit=0 - should default to 100
3. Request limit=-1 - should return error
4. Request offset=-1 - should return error
