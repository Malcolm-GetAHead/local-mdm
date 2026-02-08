# Bug Fix: AsyncLogger Nil Context Panic

**Issue**: Pre-existing test failure in `TestHandleHealth_Integration`  
**Root Cause**: `AsyncLogger.Shutdown()` panicked when called with nil context  
**Severity**: HIGH (causes panic in production if shutdown called with nil context)  
**Status**: ✅ FIXED

## Problem

The `TestHandleHealth_Integration` test was failing with a panic:

```
panic: runtime error: invalid memory address or nil pointer dereference
[signal SIGSEGV: segmentation violation code=0x2 addr=0x20 pc=0x102623ec4]

github.com/malcolm-getahead/local-mdm/internal/audit.(*AsyncLogger).Shutdown(0x1400018c040, {0x0, 0x0})
    /Users/malcolm/Documents/GitRepos/Malcolm-GetAHead/local-mdm/internal/audit/async_logger.go:129 +0x1c4
```

## Root Cause

**File**: `internal/audit/async_logger.go`  
**Line**: 129  
**Issue**: `ctx.Done()` called on nil context

The test was calling `server.Shutdown(nil)`, which passed nil context to `AsyncLogger.Shutdown(ctx)`, causing a nil pointer dereference when trying to access `ctx.Done()`.

**Call Chain:**
1. `health_test.go:158` - `defer server.Shutdown(nil)`
2. `server.go:272` - `asyncLogger.Shutdown(ctx)` (ctx is nil)
3. `async_logger.go:129` - `case <-ctx.Done():` (panic on nil)

## Fix

Modified `AsyncLogger.Shutdown()` to handle nil context gracefully:

```go
// Shutdown gracefully shuts down the async logger with timeout
// Waits for workers to drain queue or context timeout
// If ctx is nil, waits indefinitely for workers to finish
func (al *AsyncLogger) Shutdown(ctx context.Context) error {
    al.mu.Lock()
    if al.closed {
        al.mu.Unlock()
        return nil
    }
    al.closed = true
    al.mu.Unlock()

    close(al.eventQueue)

    // Wait for workers with timeout
    done := make(chan struct{})
    go func() {
        al.wg.Wait()
        close(done)
    }()

    // If no context provided, wait indefinitely
    if ctx == nil {
        <-done
        return nil
    }

    select {
    case <-done:
        return nil
    case <-ctx.Done():
        if al.slogger != nil {
            al.slogger.Warn("Audit logger shutdown timeout, some events may be lost")
        }
        return ctx.Err()
    }
}
```

**Key Changes:**
1. Added nil context check before select statement
2. If ctx is nil, wait indefinitely for workers to finish
3. Updated godoc to document nil context behavior

## Test Added

Added `TestAsyncLogger_Shutdown_NilContext` to verify nil context handling:

```go
func TestAsyncLogger_Shutdown_NilContext(t *testing.T) {
    db := setupTestDB(t)
    defer db.Close()

    logger := NewAsyncLogger(db.DB, 100, 2, nil)

    // Create test enterprise and user
    enterpriseID := createTestEnterprise(t, db)
    userID := createTestUser(t, db, enterpriseID)

    // Log an event
    err := logger.Log(context.Background(), Event{
        EnterpriseID: enterpriseID,
        UserID:       userID,
        Action:       "test.action",
        ResourceType: "test",
        ResourceID:   uuid.New(),
    })
    require.NoError(t, err)

    // Shutdown with nil context should not panic
    err = logger.Shutdown(nil)
    assert.NoError(t, err)

    // Verify event was logged
    var count int
    err = db.DB.QueryRow("SELECT COUNT(*) FROM audit_logs WHERE action = 'test.action' AND user_id = $1", userID).Scan(&count)
    require.NoError(t, err)
    assert.Equal(t, 1, count)
}
```

## Verification

### Before Fix
```bash
$ go test -v ./internal/api/... -run TestHandleHealth_Integration
panic: runtime error: invalid memory address or nil pointer dereference
FAIL
```

### After Fix
```bash
$ go test -v ./internal/api/... -run TestHandleHealth_Integration
--- PASS: TestHandleHealth_Integration (2.12s)
    --- PASS: TestHandleHealth_Integration/all_dependencies_healthy (0.00s)
    --- PASS: TestHandleHealth_Integration/response_format_is_valid_JSON (0.00s)
    --- PASS: TestHandleHealth_Integration/timestamp_is_recent (0.00s)
    --- PASS: TestHandleHealth_Integration/checks_map_contains_expected_keys (0.00s)
    --- PASS: TestHandleHealth_Integration/database_check_reports_status (0.00s)
    --- PASS: TestHandleHealth_Integration/keycloak_check_reports_status (0.00s)
    --- PASS: TestHandleHealth_Integration/respects_context_timeout (0.00s)
    --- PASS: TestHandleHealth_Integration/version_is_included (0.00s)
PASS
ok      github.com/malcolm-getahead/local-mdm/internal/api  2.356s
```

### Full Test Suite
```bash
$ go test -race ./...
ok      github.com/malcolm-getahead/local-mdm/internal/api      48.879s
ok      github.com/malcolm-getahead/local-mdm/internal/audit    3.045s
ok      github.com/malcolm-getahead/local-mdm/internal/auth     (cached)
ok      github.com/malcolm-getahead/local-mdm/internal/certs    (cached)
...
✅ ALL TESTS PASSING
```

## Impact

### Before
- ❌ Test suite had 1 failing test
- ❌ Potential production panic if shutdown called with nil context
- ❌ Unclear behavior for nil context

### After
- ✅ All tests passing
- ✅ No panic on nil context
- ✅ Clear documented behavior (waits indefinitely)
- ✅ Defensive programming

## Files Modified

1. **Modified** `internal/audit/async_logger.go` - Added nil context handling
2. **Modified** `internal/audit/shutdown_test.go` - Added test for nil context

## Why This Matters

### Production Safety
- Prevents panic if shutdown called incorrectly
- Graceful degradation instead of crash
- Defensive programming best practice

### Test Reliability
- Eliminates flaky test
- Clear test expectations
- Better error messages

### API Clarity
- Documents nil context behavior
- Makes API more forgiving
- Reduces cognitive load for callers

## Lessons Learned

1. **Always handle nil contexts**: Even if API expects non-nil, defensive code prevents panics
2. **Test edge cases**: Nil inputs are common edge cases that should be tested
3. **Fix pre-existing issues**: Don't bypass failing tests - they indicate real problems

---

**Fixed**: 2026-02-08  
**Effort**: 15 minutes  
**Impact**: HIGH (prevents production panic)  
**Test Coverage**: Added 1 test for nil context handling
