# Sprint 2 - Low Priority Issues

## Overview

Low priority issues identified during Sprint 2 development. These issues don't block core functionality but should be addressed for improved maintainability and user experience.

**Status Summary:**
- Total Issues: 3
- Open: 3 (100%)
- In Progress: 0 (0%)
- Resolved: 0 (0%)

---

## L-01: Inconsistent Error Messages Expose Internal Details

**Severity:** LOW  
**Category:** Security/UX  
**Status:** Open  
**Effort:** 2-3 hours  

### Impact
Error messages inconsistently expose internal implementation details (database errors, file paths) to API consumers, creating potential security risks and poor user experience.

### Problem
```go
// Current inconsistent error handling
return fmt.Errorf("failed to connect to database: %v", err)  // Exposes DB details
return errors.New("invalid request")  // Too generic
```

### Fix
Standardize error responses with user-friendly messages and internal logging:

```go
// pkg/errors/errors.go
func NewAPIError(code string, message string, internal error) *APIError {
    return &APIError{Code: code, Message: message, internal: internal}
}

// Usage
if err := db.Connect(); err != nil {
    log.Error("database connection failed", "error", err)
    return NewAPIError("DB_ERROR", "Service temporarily unavailable", err)
}
```

### Verification
- [ ] All API endpoints return consistent error format
- [ ] Internal details logged but not exposed
- [ ] Error codes documented in API spec

---

## L-02: Missing Request ID Tracking for Log Correlation

**Severity:** LOW  
**Category:** Observability  
**Status:** Open  
**Effort:** 3-4 hours  

### Impact
Difficult to trace requests across service boundaries and correlate logs for debugging, especially in multi-tenant scenarios.

### Problem
No request correlation mechanism makes troubleshooting complex workflows challenging.

### Fix
Add request ID middleware and propagate through context:

```go
// pkg/middleware/request_id.go
func RequestID(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        id := r.Header.Get("X-Request-ID")
        if id == "" {
            id = uuid.New().String()
        }
        ctx := context.WithValue(r.Context(), "request_id", id)
        w.Header().Set("X-Request-ID", id)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

### Verification
- [ ] All HTTP responses include X-Request-ID header
- [ ] Logs include request ID when available
- [ ] Request ID propagated to downstream calls

---

## L-03: Hardcoded Timeouts Not Configurable

**Severity:** LOW  
**Category:** Configuration  
**Status:** Open  
**Effort:** 2-3 hours  

### Impact
Hardcoded timeout values prevent tuning for different deployment environments and network conditions.

### Problem
```go
// Current hardcoded timeouts
client := &http.Client{Timeout: 30 * time.Second}  // Fixed timeout
ctx, cancel := context.WithTimeout(ctx, 5*time.Second)  // Fixed context timeout
```

### Fix
Move timeouts to configuration with sensible defaults:

```go
// configs/timeouts.go
type Timeouts struct {
    HTTP     time.Duration `yaml:"http" default:"30s"`
    Database time.Duration `yaml:"database" default:"5s"`
    MDM      time.Duration `yaml:"mdm" default:"60s"`
}

// Usage
client := &http.Client{Timeout: cfg.Timeouts.HTTP}
```

### Verification
- [ ] All timeouts configurable via config file
- [ ] Default values maintain current behavior
- [ ] Configuration validation prevents invalid values

---

## Notes

These low priority issues can be addressed incrementally without impacting Sprint 2 core deliverables. Consider batching fixes during maintenance windows or assigning to junior developers for learning opportunities.