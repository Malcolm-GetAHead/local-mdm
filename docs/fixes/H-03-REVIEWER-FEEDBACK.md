# Reviewer Feedback - Configuration Improvements

**Date**: 2026-02-08  
**Status**: ✅ RESOLVED  
**Type**: Non-Critical Improvements

---

## Issues Identified

### 1. Buffer Size Hard-coded ℹ️
**Location**: `internal/api/server.go`

**Before**:
```go
audit.NewAsyncLogger(database.DB, 1000, logger)  // Hard-coded
```

**Concern**: Could be configurable, though 1000 is reasonable default.

---

### 2. Worker Count Hard-coded ℹ️
**Location**: `internal/audit/async_logger.go`

**Before**:
```go
for i := 0; i < 3; i++ {  // Hard-coded
    go al.worker(i)
}
```

**Concern**: Could be configurable, though 3 workers is reasonable.

---

## Resolution

### Configuration Added

**File**: `internal/config/config.go`

```go
// AuditLogConfig holds audit log configuration
type AuditLogConfig struct {
    BufferSize  int `yaml:"buffer_size"`
    WorkerCount int `yaml:"worker_count"`
}

type AuthConfig struct {
    // ... existing fields ...
    AuditLog AuditLogConfig `yaml:"audit_log"`
}
```

### Function Signature Updated

**File**: `internal/audit/async_logger.go`

```go
func NewAsyncLogger(db *sql.DB, bufferSize, workerCount int, logger *slog.Logger) *AsyncLogger {
    if bufferSize <= 0 {
        bufferSize = 1000 // Default
    }
    if workerCount <= 0 {
        workerCount = 3 // Default
    }
    
    // ... create logger ...
    
    // Start configurable number of workers
    for i := 0; i < workerCount; i++ {
        al.wg.Add(1)
        go al.worker(i)
    }
    
    return al
}
```

### Server Integration

**File**: `internal/api/server.go`

```go
func New(cfg *config.Config, database *db.DB, logger *slog.Logger) (*Server, error) {
    // Get audit log config with defaults
    bufferSize := cfg.Auth.AuditLog.BufferSize
    if bufferSize == 0 {
        bufferSize = 1000 // Default
    }
    workerCount := cfg.Auth.AuditLog.WorkerCount
    if workerCount == 0 {
        workerCount = 3 // Default
    }

    s := &Server{
        // ...
        auditLogger: audit.NewAsyncLogger(database.DB, bufferSize, workerCount, logger),
    }
    // ...
}
```

### Configuration File

**File**: `configs/config.example.yaml`

```yaml
auth:
  jwt_secret: "REPLACE_WITH_ENV_VAR"
  access_token_duration: 1h
  refresh_token_duration: 168h
  circuit_breaker:
    max_failures: 5
    timeout: 30s
  token_cache:
    ttl: 5m
  audit_log:
    buffer_size: 1000    # Async audit log queue size
    worker_count: 3      # Number of background workers
```

---

## Benefits

✅ **Configurable per Environment**
- Dev: Small buffer (100), 1 worker
- Staging: Medium buffer (500), 2 workers
- Production: Large buffer (1000+), 3+ workers

✅ **Sensible Defaults**
- Buffer: 1000 events (handles bursts)
- Workers: 3 (good parallelism without overhead)

✅ **Backward Compatible**
- Defaults applied if not configured
- No breaking changes

✅ **Tunable Performance**
- High load: Increase buffer + workers
- Low load: Decrease to save resources

---

## Configuration Examples

### High-Load Production
```yaml
audit_log:
  buffer_size: 5000    # Handle large bursts
  worker_count: 5      # More parallelism
```

### Low-Load Development
```yaml
audit_log:
  buffer_size: 100     # Small buffer
  worker_count: 1      # Single worker
```

### Default (Omitted)
```yaml
# audit_log not specified - uses defaults
# buffer_size: 1000
# worker_count: 3
```

---

## Test Results

```bash
✅ All tests passing
✅ No regressions
✅ Configuration validated

ok      internal/audit    1.993s
ok      internal/api      44.520s
ok      internal/config   1.637s
```

---

## Files Modified

1. `internal/config/config.go` - Added `AuditLogConfig`
2. `internal/audit/async_logger.go` - Accept `workerCount` parameter
3. `internal/api/server.go` - Use config values with defaults
4. `configs/config.example.yaml` - Added audit_log section
5. `internal/audit/async_logger_test.go` - Updated test calls

---

## Summary

**Before**:
- ❌ Buffer size hard-coded (1000)
- ❌ Worker count hard-coded (3)
- ⚠️ Not tunable per environment

**After**:
- ✅ Buffer size configurable
- ✅ Worker count configurable
- ✅ Sensible defaults (1000, 3)
- ✅ Tunable per environment
- ✅ Backward compatible

---

**Status**: ✅ **RESOLVED**

Both configuration parameters are now tunable with sensible defaults.
