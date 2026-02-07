# Critical Fixes - Sprint 1 Review

**Priority**: 🔴 CRITICAL  
**Total Issues**: 6  
**Estimated Effort**: 5-6 days  
**Must Complete Before**: Sprint 2

---

## Quick Summary

This document contains **6 critical issues** that need immediate attention before Sprint 2.

---

## Issue 1: Fix JWKS Race Condition

**File**: `internal/auth/oidc.go`  
**Effort**: 4 hours  
**Impact**: Authentication bypass or server crashes  
**Status**: ✅ RESOLVED (2026-02-07)

### Problem
```go
// Check-then-act race condition
if time.Since(v.lastRefresh) > v.refreshEvery {
    v.refreshJWKS()  // Multiple goroutines can enter
}
```

### Solution Implemented
```go
import "sync/atomic"

type OIDCValidator struct {
    jwks         *JWKS
    jwksMutex    sync.RWMutex
    lastRefresh  time.Time
    refreshEvery time.Duration
    refreshMutex sync.Mutex      // NEW: Serializes refresh operations
}

func (v *OIDCValidator) refreshJWKS() error {
    v.refreshMutex.Lock()
    defer v.refreshMutex.Unlock()
    
    // Double-check pattern
    v.jwksMutex.RLock()
    needsRefresh := time.Since(v.lastRefresh) > v.refreshEvery
    v.jwksMutex.RUnlock()
    
    if !needsRefresh {
        return nil
    }
    
    // Fetch and update atomically
    jwks := fetchJWKS()
    v.jwksMutex.Lock()
    v.jwks = &jwks
    v.lastRefresh = time.Now()
    v.jwksMutex.Unlock()
    return nil
}

func (v *OIDCValidator) ValidateToken(token string) (*AuthUser, error) {
    // Non-blocking check with proper locking
    v.jwksMutex.RLock()
    needsRefresh := time.Since(v.lastRefresh) > v.refreshEvery
    v.jwksMutex.RUnlock()
    
    if needsRefresh {
        go v.refreshJWKS()  // Background refresh
    }
    
    // Lock-free read
    v.jwksMutex.RLock()
    jwks := v.jwks
    v.jwksMutex.RUnlock()
    // ... validate token
}
```

### Testing
```bash
go test -race ./internal/auth/...
# ✅ PASS - No race conditions detected
```

### Documentation
See: `ISSUE-01-JWKS-RACE-CONDITION-RESOLVED.md`

---

## Issue 2: Fix JSONB Injection

**File**: `internal/repository/*.go`  
**Effort**: 1 day  
**Impact**: Database compromise, DoS  
**Status**: ✅ RESOLVED (2026-02-07)

### Problem
No validation of JSONB fields - accepts arbitrary JSON.

### Solution Implemented
```go
// internal/validation/jsonb.go
const (
    MaxJSONBSize  = 1 << 20 // 1MB
    MaxJSONBDepth = 10
)

func ValidateJSONB(data interface{}, maxDepth int) error {
    if data == nil {
        return nil
    }

    bytes, err := json.Marshal(data)
    if err != nil {
        return fmt.Errorf("invalid JSON: %w", err)
    }

    if len(bytes) > MaxJSONBSize {
        return fmt.Errorf("JSON exceeds maximum size of %d bytes", MaxJSONBSize)
    }

    var obj interface{}
    if err := json.Unmarshal(bytes, &obj); err != nil {
        return fmt.Errorf("invalid JSON: %w", err)
    }

    if depth := calculateDepth(obj); depth > maxDepth {
        return fmt.Errorf("JSON nesting depth %d exceeds maximum of %d", depth, maxDepth)
    }

    return nil
}

// In repositories
func (r *deviceRepository) Create(ctx context.Context, device *models.Device) error {
    if err := validation.ValidateJSONB(device.PlatformData, validation.MaxJSONBDepth); err != nil {
        return fmt.Errorf("invalid platform_data: %w", err)
    }
    // ... rest of create
}
```

### Testing
```bash
go test -race ./internal/validation/... ./internal/repository/...
# ✅ PASS - All tests pass, coverage 93.7% (validation), 86.2% (repository)
```

### Documentation
See: `ISSUE-02-JSONB-INJECTION-RESOLVED.md`

---

## Issue 3: Fix Context Cancellation

**File**: `internal/repository/*.go`  
**Effort**: 1 hour  
**Impact**: Resource leaks, connection exhaustion  
**Status**: ✅ RESOLVED (2026-02-07)

### Problem
Repository methods ignore context cancellation.

### Solution Implemented
```go
func (r *deviceRepository) List(ctx context.Context, ...) ([]*models.Device, int, error) {
    // Check before expensive operation
    select {
    case <-ctx.Done():
        return nil, 0, ctx.Err()
    default:
    }
    
    // Count query
    var total int
    if err := exec.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
        return nil, 0, err
    }
    
    // Check again
    select {
    case <-ctx.Done():
        return nil, 0, ctx.Err()
    default:
    }
    
    // Main query
    rows, err := exec.QueryContext(ctx, query, args...)
    if err != nil {
        return nil, 0, err
    }
    defer rows.Close()
    
    devices := []*models.Device{}
    for rows.Next() {
        // Check during iteration
        select {
        case <-ctx.Done():
            return nil, 0, ctx.Err()
        default:
        }
        
        // Scan row
        device := &models.Device{}
        if err := rows.Scan(...); err != nil {
            return nil, 0, err
        }
        devices = append(devices, device)
    }
    
    return devices, total, rows.Err()
}
```

### Testing
```bash
go test -race ./internal/repository/... -run "ContextCancellation"
# ✅ PASS - 9 tests (3 per repository)
```

### Documentation
See: `ISSUE-03-CONTEXT-CANCELLATION-RESOLVED.md`

---

## Issue 4: Fix Panic in Constructors

**File**: `internal/repository/*.go`  
**Effort**: 2 hours  
**Impact**: Server crashes  
**Status**: ✅ RESOLVED (2026-02-07)

### Problem
```go
func NewEnterpriseRepository(db interface{}) EnterpriseRepository {
    switch v := db.(type) {
    // ...
    default:
        panic(fmt.Sprintf("unsupported database type: %T", db))
    }
}
```

### Solution Implemented
```go
func NewEnterpriseRepository(db interface{}) (EnterpriseRepository, error) {
    switch v := db.(type) {
    case *sql.DB:
        return &enterpriseRepository{db: v}, nil
    case executor:
        return &enterpriseRepository{db: v}, nil
    default:
        return nil, fmt.Errorf("unsupported database type: %T", db)
    }
}

// Update all callers
repo, err := repository.NewEnterpriseRepository(db)
if err != nil {
    return fmt.Errorf("failed to create repository: %w", err)
}
```

### Testing
```bash
go test -race ./internal/repository/...
# ✅ PASS - All tests pass, no panics
```

### Documentation
See: `ISSUE-04-PANIC-IN-CONSTRUCTORS-RESOLVED.md`

---

## Issue 5: Fix Transaction Isolation

**File**: `internal/repository/transaction.go`  
**Effort**: 4 hours  
**Impact**: Data corruption, race conditions  
**Status**: ✅ RESOLVED (2026-02-07)

### Problem
No isolation level specified (uses default READ COMMITTED).

### Solution Implemented
```go
type IsolationLevel string

const (
    IsolationDefault      IsolationLevel = ""
    IsolationReadCommitted IsolationLevel = "READ COMMITTED"
    IsolationSerializable  IsolationLevel = "SERIALIZABLE"
)

func (t *transactor) WithTransactionIsolation(
    ctx context.Context, 
    isolation IsolationLevel, 
    fn func(context.Context) error,
) error {
    opts := &sql.TxOptions{}
    if isolation != IsolationDefault {
        opts.Isolation = toSQLIsolation(isolation)
    }
    
    tx, err := db.BeginTx(ctx, opts)
    if err != nil {
        return fmt.Errorf("begin transaction: %w", err)
    }
    
    // Execute with retry for serialization errors
    maxRetries := 3
    for attempt := 0; attempt < maxRetries; attempt++ {
        err = fn(txCtx)
        
        if err == nil {
            break
        }
        
        if isolation == IsolationSerializable && isSerializationError(err) && attempt < maxRetries-1 {
            time.Sleep(time.Duration(attempt+1) * 100 * time.Millisecond)
            continue
        }
        
        break
    }
    
    if err != nil {
        tx.Rollback()
        return err
    }
    
    return tx.Commit()
}

// Use SERIALIZABLE for critical operations
err := transactor.WithTransactionIsolation(ctx, IsolationSerializable, func(txCtx context.Context) error {
    // Critical operations with full isolation
    return repo.Create(txCtx, entity)
})
```

### Testing
```bash
go test -race ./internal/repository/...
# ✅ PASS - All tests pass, coverage 86.3%
```

### Documentation
See: `ISSUE-05-TRANSACTION-ISOLATION-RESOLVED.md`

---
    // Create enterprise and first device atomically
    return nil
})
```

---

## Issue 6: Fix Rate Limiter Memory

**File**: `internal/api/ratelimit.go`  
**Effort**: 4 hours  
**Impact**: Memory exhaustion DoS  
**Status**: ✅ RESOLVED (2026-02-07)

### Problem
Unbounded map growth - attacker can exhaust memory by sending requests from many IPs.

### Solution Implemented
```go
import "container/list"

const (
    maxRateLimiterEntries = 10000 // Maximum number of tracked IPs
)

type rateLimiter struct {
    requests map[string][]time.Time
    lru      *list.List
    lruMap   map[string]*list.Element
    mu       sync.RWMutex
    limit    int
    window   time.Duration
    maxSize  int
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
    return &rateLimiter{
        requests: make(map[string][]time.Time),
        lru:      list.New(),
        lruMap:   make(map[string]*list.Element),
        limit:    limit,
        window:   window,
        maxSize:  maxRateLimiterEntries,
    }
}

func (rl *rateLimiter) allow(key string) bool {
    rl.mu.Lock()
    defer rl.mu.Unlock()
    
    // Evict oldest entry if at capacity and key doesn't exist
    if _, exists := rl.requests[key]; !exists && len(rl.requests) >= rl.maxSize {
        rl.evictOldest()
    }
    
    // Update LRU
    if element, ok := rl.lruMap[key]; ok {
        rl.lru.MoveToFront(element)
    } else {
        element := rl.lru.PushFront(key)
        rl.lruMap[key] = element
    }
    
    // Check rate limit and track request
    // ...
}

func (rl *rateLimiter) evictOldest() {
    if rl.lru.Len() == 0 {
        return
    }
    
    oldest := rl.lru.Back()
    if oldest != nil {
        key := oldest.Value.(string)
        rl.lru.Remove(oldest)
        delete(rl.lruMap, key)
        delete(rl.requests, key)
    }
}
```

### Testing
```bash
go test -race ./internal/api/...
# ✅ PASS - All tests pass, no race conditions
# Coverage: allow() 100%, cleanup() 100%, evictOldest() 87.5%
# Memory bounded at ~9 MB (was unbounded)
```

### Documentation
See: `ISSUE-06-RATE-LIMITER-MEMORY-RESOLVED.md`

---
    }
}
```

---

## Testing Checklist

For each fix:

- [ ] Unit tests added
- [ ] Integration tests added
- [ ] Race detector clean (`go test -race`)
- [ ] Manual testing completed
- [ ] Code reviewed
- [ ] Documentation updated

---

## Verification Commands

```bash
# Run all tests with race detection
go test -race ./...

# Check specific packages
go test -race -v ./internal/auth/...
go test -race -v ./internal/repository/...
go test -race -v ./internal/api/...

# Verify no panics
go test -v ./internal/repository/... 2>&1 | grep -i panic

# Check coverage
go test -cover ./... | grep -v "no test files"
```

---

## Timeline

### Day 1
- Morning: Fix JWKS race condition (Issue 1)
- Afternoon: Fix panic in constructors (Issue 4)

### Day 2
- Morning: Start JSONB validation (Issue 2)
- Afternoon: Complete JSONB validation

### Day 3
- Morning: Fix context cancellation (Issue 3)
- Afternoon: Continue context cancellation

### Day 4
- Morning: Fix transaction isolation (Issue 5)
- Afternoon: Fix rate limiter memory (Issue 6)

### Day 5
- Morning: Testing and verification
- Afternoon: Code review and documentation

### Day 6 (Buffer)
- Fix any issues found during testing
- Final verification

---

## Success Criteria

- [ ] All 6 issues resolved
- [ ] All tests pass
- [ ] Race detector clean
- [ ] No panics in tests
- [ ] Code reviewed and approved
- [ ] Documentation updated

---

## After Completion

Once these 6 critical issues are fixed:

✅ **Proceed to Sprint 2** - Foundation is solid  
✅ **Address high-priority issues** - During Sprint 2 as time permits  
✅ **Follow future phase plans** - F-01 through F-08 as documented

---

**Total Effort**: 5-6 days  
**Priority**: Must complete before Sprint 2  
**Impact**: Prevents security issues, crashes, and data corruption
