# Critical Reliability Issues

**Priority**: 🔴 **CRITICAL**  
**Must Fix**: Before any production deployment

---

## CRITICAL-07: Unbounded Memory Growth in Rate Limiter

### Location
`internal/api/ratelimit.go:18-60`

### Issue
The rate limiter has multiple memory leak vectors:

```go
type rateLimiter struct {
    requests map[string][]time.Time  // Unbounded growth
    lru      *list.List               // Unbounded growth
    lruMap   map[string]*list.Element // Unbounded growth
    // ...
}
```

**Problems**:
1. `maxSize` is set to 10,000 but not enforced consistently
2. Each IP can accumulate unlimited timestamps in `requests[ip]`
3. Cleanup runs only every 1 minute (too infrequent)
4. No memory limits on individual entries

### Attack Vector
```bash
# Attacker rotates through 10,000 IPs
for i in 1..10000; do
  # Each IP makes 100 requests
  for j in 1..100; do
    curl -H "X-Forwarded-For: 10.0.$((i/256)).$((i%256))" http://target/api
  done
done

# Result: 10,000 IPs × 100 timestamps × 24 bytes = 24 MB
# Repeat every minute = unbounded growth
```

### Impact
- **Severity**: CRITICAL
- **Exploitability**: High
- **Impact**: Memory exhaustion, service crash

### Fix

```go
const (
    maxRateLimiterEntries = 10000
    maxTimestampsPerKey   = 1000  // NEW: Limit per IP
    cleanupInterval       = 10 * time.Second  // More frequent
)

type rateLimiter struct {
    requests    map[string]*timestampRing  // Use ring buffer
    lru         *list.List
    lruMap      map[string]*list.Element
    mu          sync.RWMutex
    limit       int
    window      time.Duration
    maxSize     int
    stopChan    chan struct{}
    stopped     bool
    memoryLimit int64  // NEW: Total memory limit
}

// Ring buffer for timestamps (bounded memory)
type timestampRing struct {
    timestamps []time.Time
    head       int
    size       int
    capacity   int
}

func newTimestampRing(capacity int) *timestampRing {
    return &timestampRing{
        timestamps: make([]time.Time, capacity),
        capacity:   capacity,
    }
}

func (r *timestampRing) Add(t time.Time) {
    r.timestamps[r.head] = t
    r.head = (r.head + 1) % r.capacity
    if r.size < r.capacity {
        r.size++
    }
}

func (r *timestampRing) CountSince(cutoff time.Time) int {
    count := 0
    for i := 0; i < r.size; i++ {
        if r.timestamps[i].After(cutoff) {
            count++
        }
    }
    return count
}

func (rl *rateLimiter) allow(key string) bool {
    rl.mu.Lock()
    defer rl.mu.Unlock()
    
    now := time.Now()
    cutoff := now.Add(-rl.window)
    
    // Enforce max size
    if _, exists := rl.requests[key]; !exists {
        if len(rl.requests) >= rl.maxSize {
            rl.evictOldest()
        }
        rl.requests[key] = newTimestampRing(maxTimestampsPerKey)
    }
    
    // Update LRU
    if element, ok := rl.lruMap[key]; ok {
        rl.lru.MoveToFront(element)
    } else {
        element := rl.lru.PushFront(key)
        rl.lruMap[key] = element
    }
    
    // Count recent requests
    ring := rl.requests[key]
    recentCount := ring.CountSince(cutoff)
    
    if recentCount >= rl.limit {
        return false
    }
    
    // Add new request
    ring.Add(now)
    return true
}

// More aggressive cleanup
func (rl *rateLimiter) cleanup() {
    rl.mu.Lock()
    defer rl.mu.Unlock()
    
    now := time.Now()
    cutoff := now.Add(-rl.window * 2)
    
    for key, ring := range rl.requests {
        if ring.CountSince(cutoff) == 0 {
            delete(rl.requests, key)
            if element, ok := rl.lruMap[key]; ok {
                rl.lru.Remove(element)
                delete(rl.lruMap, key)
            }
        }
    }
}
```

### Test Case
```go
func TestRateLimiterMemoryBound(t *testing.T) {
    rl := newRateLimiter(100, time.Minute)
    defer rl.Stop()
    
    // Simulate 100,000 unique IPs
    for i := 0; i < 100000; i++ {
        key := fmt.Sprintf("ip-%d", i)
        rl.allow(key)
    }
    
    // Should never exceed maxSize
    assert.LessOrEqual(t, len(rl.requests), maxRateLimiterEntries)
}
```

---

## CRITICAL-08: Goroutine Leak in Rate Limiter

### Location
`internal/api/ratelimit.go:42-54`

### Issue
The cleanup goroutine is never stopped in most code paths:

```go
func newRateLimiter(limit int, window time.Duration) *rateLimiter {
    rl := &rateLimiter{...}
    
    // Goroutine starts but may never stop
    go func() {
        ticker := time.NewTicker(1 * time.Minute)
        defer ticker.Stop()
        for {
            select {
            case <-ticker.C:
                rl.cleanup()
            case <-rl.stopChan:
                return
            }
        }
    }()
    
    return rl
}
```

**Problem**: `Stop()` is never called in `internal/api/server.go`

### Impact
- **Severity**: CRITICAL
- **Exploitability**: Automatic
- **Impact**: Goroutine leak, memory leak, eventual crash

### Fix

1. **Add cleanup to server shutdown**:

```go
// internal/api/server.go
type Server struct {
    router         *mux.Router
    db             *db.DB
    config         *config.Config
    logger         *slog.Logger
    authMiddleware *auth.Middleware
    server         *http.Server
    rateLimiter    *rateLimiter  // NEW: Store reference
}

func (s *Server) setupMiddleware() {
    // ...
    if s.config.Server.RateLimit.Enabled {
        s.rateLimiter = newRateLimiter(limit, window)
        s.router.Use(rateLimitMiddleware(s.rateLimiter))
    }
}

func (s *Server) Shutdown(ctx context.Context) error {
    // Stop rate limiter first
    if s.rateLimiter != nil {
        s.rateLimiter.Stop()
    }
    
    // Then shutdown HTTP server
    return s.server.Shutdown(ctx)
}
```

2. **Add finalizer as safety net**:

```go
func newRateLimiter(limit int, window time.Duration) *rateLimiter {
    rl := &rateLimiter{...}
    
    // Set finalizer to ensure cleanup
    runtime.SetFinalizer(rl, func(r *rateLimiter) {
        r.Stop()
    })
    
    go func() {
        // ... cleanup goroutine
    }()
    
    return rl
}
```

---

## CRITICAL-09: Missing Transaction Rollback on Context Cancellation

### Location
`internal/repository/transaction.go:66-140`

### Issue
Transactions don't check for context cancellation before commit:

```go
func (t *transactor) WithTransactionIsolation(ctx context.Context, isolation IsolationLevel, fn func(context.Context) error) error {
    // ... begin transaction ...
    
    // Execute function
    err = fn(txCtx)
    
    if err != nil {
        // Rollback on error
        if rbErr := tx.Rollback(); rbErr != nil {
            return fmt.Errorf("rollback failed: %v (original error: %w)", rbErr, err)
        }
        return err
    }
    
    // PROBLEM: Commits even if context was cancelled!
    if err := tx.Commit(); err != nil {
        return fmt.Errorf("commit transaction: %w", err)
    }
    
    return nil
}
```

### Attack Vector
```go
// Client cancels request mid-transaction
ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
defer cancel()

// Transaction takes 2 seconds
err := transactor.WithTransaction(ctx, func(txCtx context.Context) error {
    // Long-running operation
    time.Sleep(2 * time.Second)
    return repo.Create(txCtx, device)
})

// Context cancelled but transaction still commits!
```

### Impact
- **Severity**: CRITICAL
- **Exploitability**: Medium
- **Impact**: Data corruption, partial writes, resource leaks

### Fix

```go
func (t *transactor) WithTransactionIsolation(ctx context.Context, isolation IsolationLevel, fn func(context.Context) error) error {
    // Check context before starting
    select {
    case <-ctx.Done():
        return ctx.Err()
    default:
    }
    
    // ... begin transaction ...
    
    // Execute function
    err = fn(txCtx)
    
    // Check context after function execution
    select {
    case <-ctx.Done():
        // Context cancelled - rollback
        _ = tx.Rollback()
        return ctx.Err()
    default:
    }
    
    if err != nil {
        if rbErr := tx.Rollback(); rbErr != nil {
            return fmt.Errorf("rollback failed: %v (original error: %w)", rbErr, err)
        }
        return err
    }
    
    // Final context check before commit
    select {
    case <-ctx.Done():
        _ = tx.Rollback()
        return ctx.Err()
    default:
    }
    
    // Commit transaction
    if err := tx.Commit(); err != nil {
        return fmt.Errorf("commit transaction: %w", err)
    }
    
    return nil
}
```

### Test Case
```go
func TestTransactionRollbackOnContextCancellation(t *testing.T) {
    db := setupTestDB(t)
    transactor := NewTransactor(db)
    repo := NewDeviceRepository(db)
    
    ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
    defer cancel()
    
    err := transactor.WithTransaction(ctx, func(txCtx context.Context) error {
        // Create device
        device := &models.Device{...}
        if err := repo.Create(txCtx, device); err != nil {
            return err
        }
        
        // Simulate long operation
        time.Sleep(200 * time.Millisecond)
        
        return nil
    })
    
    // Should return context.DeadlineExceeded
    assert.ErrorIs(t, err, context.DeadlineExceeded)
    
    // Device should NOT be in database
    _, err = repo.GetByID(context.Background(), device.ID)
    assert.Error(t, err, "device should not exist")
}
```

---

## CRITICAL-10: Missing Database Connection Pool Limits

### Location
`internal/db/db.go`

### Issue
Database connection pool has no limits configured:

```go
func New(cfg config.DatabaseConfig) (*DB, error) {
    db, err := sql.Open("postgres", cfg.DSN())
    if err != nil {
        return nil, err
    }
    
    // MISSING: Connection pool configuration!
    // db.SetMaxOpenConns(...)
    // db.SetMaxIdleConns(...)
    // db.SetConnMaxLifetime(...)
    
    return &DB{DB: db}, nil
}
```

### Impact
- **Severity**: CRITICAL
- **Exploitability**: High (under load)
- **Impact**: Database connection exhaustion, service unavailability

### Fix

```go
func New(cfg config.DatabaseConfig) (*DB, error) {
    db, err := sql.Open("postgres", cfg.DSN())
    if err != nil {
        return nil, err
    }
    
    // Configure connection pool
    maxOpen := cfg.MaxOpenConns
    if maxOpen == 0 {
        maxOpen = 25  // Reasonable default
    }
    db.SetMaxOpenConns(maxOpen)
    
    maxIdle := cfg.MaxIdleConns
    if maxIdle == 0 {
        maxIdle = 5  // Reasonable default
    }
    db.SetMaxIdleConns(maxIdle)
    
    connLifetime := cfg.ConnMaxLifetime
    if connLifetime == 0 {
        connLifetime = 5 * time.Minute  // Reasonable default
    }
    db.SetConnMaxLifetime(connLifetime)
    
    // Set connection timeout
    db.SetConnMaxIdleTime(1 * time.Minute)
    
    // Verify connection
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    if err := db.PingContext(ctx); err != nil {
        return nil, fmt.Errorf("database ping failed: %w", err)
    }
    
    return &DB{DB: db}, nil
}
```

Update config:

```yaml
database:
  max_open_conns: 25      # Maximum open connections
  max_idle_conns: 5       # Maximum idle connections
  conn_max_lifetime: 5m   # Connection lifetime
  query_timeout: 30s      # Query timeout
```

---

## CRITICAL-11: Panic in Repository Constructors

### Location
`internal/repository/transaction.go:176-186`

### Issue
The `getExecutor` function panics on invalid input:

```go
func getExecutor(ctx context.Context, db interface{}) executor {
    if tx := getTx(ctx); tx != nil {
        return tx
    }
    
    switch v := db.(type) {
    case *sql.DB:
        return v
    case executor:
        return v
    default:
        panic(fmt.Sprintf("unsupported database type: %T", db))  // PANIC!
    }
}
```

### Impact
- **Severity**: CRITICAL
- **Exploitability**: Low (requires programming error)
- **Impact**: Server crash

### Fix

```go
func getExecutor(ctx context.Context, db interface{}) (executor, error) {
    if tx := getTx(ctx); tx != nil {
        return tx, nil
    }
    
    switch v := db.(type) {
    case *sql.DB:
        return v, nil
    case executor:
        return v, nil
    default:
        return nil, fmt.Errorf("unsupported database type: %T", db)
    }
}

// Update all repository methods to handle error
func (r *deviceRepository) Create(ctx context.Context, device *models.Device) error {
    if err := validation.ValidateJSONB(device.PlatformData, validation.MaxJSONBDepth); err != nil {
        return fmt.Errorf("invalid platform_data: %w", err)
    }

    query := `...`
    
    exec, err := getExecutor(ctx, r.db)
    if err != nil {
        return fmt.Errorf("get executor: %w", err)
    }
    
    return exec.QueryRowContext(ctx, query, ...).Scan(...)
}
```

---

## CRITICAL-12: Missing Audit Logging

### Location
Multiple files (system-wide issue)

### Issue
No audit logging for sensitive operations:

- User authentication (login/logout)
- Device enrollment/unenrollment
- Policy creation/modification/deletion
- Certificate issuance/revocation
- Configuration changes
- Administrative actions

### Impact
- **Severity**: CRITICAL
- **Exploitability**: N/A
- **Impact**: Compliance violations, no forensic trail, cannot detect breaches

### Fix

1. **Create audit logger**:

```go
// internal/audit/logger.go
package audit

import (
    "context"
    "database/sql"
    "time"
    
    "github.com/google/uuid"
    "github.com/malcolm-getahead/local-mdm/internal/auth"
    "github.com/malcolm-getahead/local-mdm/internal/models"
)

type Logger struct {
    db *sql.DB
}

func New(db *sql.DB) *Logger {
    return &Logger{db: db}
}

type Event struct {
    EnterpriseID *uuid.UUID
    UserID       *uuid.UUID
    Action       string
    ResourceType string
    ResourceID   *uuid.UUID
    Details      models.JSONB
    IPAddress    string
    UserAgent    string
}

func (l *Logger) Log(ctx context.Context, event Event) error {
    query := `
        INSERT INTO audit_logs (
            enterprise_id, user_id, action, resource_type, resource_id,
            details, ip_address, user_agent, created_at
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())`
    
    _, err := l.db.ExecContext(ctx, query,
        event.EnterpriseID,
        event.UserID,
        event.Action,
        event.ResourceType,
        event.ResourceID,
        event.Details,
        event.IPAddress,
        event.UserAgent,
    )
    
    return err
}

// Helper to extract user from context
func (l *Logger) LogFromContext(ctx context.Context, action, resourceType string, resourceID *uuid.UUID, details models.JSONB) error {
    user, _ := auth.UserFromContext(ctx)
    
    event := Event{
        Action:       action,
        ResourceType: resourceType,
        ResourceID:   resourceID,
        Details:      details,
    }
    
    if user != nil {
        event.UserID = &user.ID
        event.EnterpriseID = &user.EnterpriseID
    }
    
    // Extract IP and User-Agent from context if available
    // ...
    
    return l.Log(ctx, event)
}
```

2. **Add audit logging to all sensitive operations**:

```go
// internal/api/handlers.go
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
    var req auth.LoginRequest
    if err := parseJSONBody(r, &req); err != nil {
        respondError(w, r, http.StatusBadRequest, "invalid_request", err.Error())
        return
    }
    
    // ... authentication logic ...
    
    // AUDIT LOG
    s.auditLogger.Log(r.Context(), audit.Event{
        Action:       "user.login",
        ResourceType: "user",
        Details: models.JSONB{
            "username": req.Username,
            "success":  true,
        },
        IPAddress: r.RemoteAddr,
        UserAgent: r.UserAgent(),
    })
    
    respondJSON(w, r, http.StatusOK, tokenResp)
}
```

3. **Add audit log retention policy**:

```sql
-- migrations/000002_audit_log_retention.up.sql
CREATE INDEX idx_audit_logs_created_at_desc ON audit_logs(created_at DESC);

-- Function to clean old audit logs (keep 1 year)
CREATE OR REPLACE FUNCTION cleanup_old_audit_logs()
RETURNS void AS $$
BEGIN
    DELETE FROM audit_logs
    WHERE created_at < NOW() - INTERVAL '1 year';
END;
$$ LANGUAGE plpgsql;

-- Schedule cleanup (requires pg_cron extension)
SELECT cron.schedule('cleanup-audit-logs', '0 2 * * *', 'SELECT cleanup_old_audit_logs()');
```

---

## Summary

All 6 critical reliability issues must be fixed before production deployment.

**Estimated effort**: 2-3 days for all fixes + comprehensive testing.

**Priority order**:
1. CRITICAL-12: Add audit logging (1 day)
2. CRITICAL-10: Configure connection pool (1 hour)
3. CRITICAL-09: Fix transaction context handling (3 hours)
4. CRITICAL-08: Fix goroutine leak (1 hour)
5. CRITICAL-07: Fix rate limiter memory leak (4 hours)
6. CRITICAL-11: Remove panics (2 hours)
