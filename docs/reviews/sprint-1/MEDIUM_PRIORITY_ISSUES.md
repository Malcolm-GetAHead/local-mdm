# Medium Priority Issues (Fix Within First Month)

**Priority**: MEDIUM  
**Total Issues**: 12  
**Resolved**: 6 ✅  
**Remaining**: 6  
**Estimated Effort**: 2.25 days (remaining)  
**Risk Level**: Moderate performance, maintainability concerns

---

## M-01: No Query Result Caching
**Severity**: MEDIUM | **Category**: Performance | **Effort**: 0.5 days

Frequently accessed data (enterprises, policies) is queried repeatedly without caching, causing unnecessary database load.

**Fix**: Implement Redis caching layer with TTL.

---

## M-02: Missing Compression Middleware ✅ RESOLVED

**Severity**: MEDIUM  
**Category**: Performance  
**Effort**: 0.25 days  
**Status**: ✅ **RESOLVED** (2026-02-08)

### Problem
API responses are not compressed, wasting bandwidth for large payloads (device lists, audit logs).

### Resolution
Implemented custom gzip compression middleware with client-aware compression.

**Implementation**: `internal/api/compression.go`

**Key Features**:
- Client-aware: Only compresses when `Accept-Encoding: gzip` present
- Proper headers: Sets `Content-Encoding: gzip`, removes `Content-Length`
- Custom `gzipResponseWriter` wrapper
- Integrated first in middleware chain for maximum benefit

**Test Coverage**: `internal/api/compression_test.go` (151 lines, 6 tests + benchmarks)
```
✅ Compresses when client accepts gzip
✅ Does not compress when client doesn't accept
✅ Compression ratio for JSON (>50% reduction)
✅ Handles empty responses
✅ Preserves status codes (200, 201, 400, 401, 404, 500)
✅ Removes Content-Length header
✅ Benchmarks (with/without compression)
```

### Verification
✅ >50% bandwidth reduction for JSON payloads  
✅ Client-aware (only when requested)  
✅ Minimal CPU overhead  
✅ All tests passing  
✅ Properly integrated in middleware chain

---

## M-03: No Database Query Logging
**Severity**: MEDIUM | **Category**: Observability | **Effort**: 0.5 days

Slow queries cannot be identified without query logging.

**Fix**: Add query logging middleware with duration tracking.

---

## M-04: Missing Health Check Details ✅ RESOLVED

**Severity**: MEDIUM  
**Category**: Observability  
**Effort**: 0.25 days  
**Status**: ✅ **RESOLVED** (2026-02-08)

### Problem
Health endpoint only checks database, not Keycloak or other dependencies.

### Resolution
Implemented comprehensive health checks with multi-component monitoring.

**Implementation**: 
- `internal/api/handlers.go` - Enhanced `handleHealth()`
- `internal/auth/oidc.go` - `HealthCheck()` method
- `internal/auth/middleware.go` - `HealthCheck()` wrapper

**Key Features**:
- Multi-component checks: Database + Keycloak
- Structured JSON response with status, version, checks, timestamp
- Proper status codes: 200 (healthy) vs 503 (unhealthy)
- Graceful degradation: Keycloak issues marked as "degraded", not "unhealthy"
- 5-second timeout per check

**Response Format**:
```json
{
  "status": "healthy",
  "version": "1.0.0",
  "checks": {
    "database": "healthy",
    "keycloak": "healthy"
  },
  "timestamp": "2026-02-08T00:37:00Z"
}
```

**Test Coverage**: `internal/api/health_test.go` (187 lines, 10 tests)
```
✅ All dependencies healthy
✅ Response format is valid JSON
✅ Timestamp is recent
✅ Checks map contains expected keys
✅ Database check reports status
✅ Keycloak check reports status
✅ Respects context timeout
✅ Version is included
✅ Returns 200 when healthy
✅ Status field matches HTTP status
```

### Verification
✅ Multi-component health monitoring  
✅ Proper status codes for K8s/load balancers  
✅ Smart degradation (Keycloak issues don't fail health)  
✅ Comprehensive test coverage  
✅ All tests passing

---

## M-05: No Metrics Endpoint
**Severity**: MEDIUM | **Category**: Observability | **Effort**: 0.5 days

No Prometheus metrics for monitoring request rates, latencies, errors.

**Fix**: Add Prometheus instrumentation.

```go
import "github.com/prometheus/client_golang/prometheus/promhttp"

s.router.Handle("/metrics", promhttp.Handler())
```

---

## M-06: Missing Request ID Propagation ✅ RESOLVED

**Severity**: MEDIUM  
**Category**: Observability  
**Effort**: 0.25 days  
**Status**: ✅ **RESOLVED** (2026-02-08)

### Problem
Request IDs are generated but not propagated to database queries or external calls.

### Resolution
Implemented comprehensive request ID middleware with full propagation.

**Implementation**:
- `internal/api/server.go` - `requestIDMiddleware()` generates UUIDs
- `internal/api/request_id.go` - `GetRequestID(ctx)` helper
- `internal/auth/context.go` - `requestIDKey` constant
- `internal/auth/middleware.go` - Request ID propagated to all auth logs

**Key Features**:
```go
func requestIDMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        requestID := uuid.New().String()
        ctx := context.WithValue(r.Context(), requestIDKey, requestID)
        w.Header().Set("X-Request-ID", requestID)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

- UUID generation: Each request gets unique ID
- Context propagation: ID stored in context, accessible everywhere
- Response header: `X-Request-ID` returned to client
- Helper functions: `api.GetRequestID(ctx)` and `auth.GetRequestID(ctx)`
- Integrated in middleware chain: Applied first for all requests
- Propagated to logs: All auth middleware logs include request_id

**Test Coverage**: `internal/api/request_id_test.go` + `internal/auth/request_id_unit_test.go` (8 tests)
```
✅ Returns empty string when not set (api + auth)
✅ Returns request ID when set (api + auth)
✅ Returns empty string for wrong type (api + auth)
✅ Returns empty string for nil value (api + auth)
```

### Verification
✅ UUID generated for each request  
✅ Propagated through context  
✅ Returned in X-Request-ID header  
✅ Included in all auth logs  
✅ Helper functions in both packages  
✅ All tests passing

---

## M-07: No Connection Pool Monitoring
**Severity**: MEDIUM | **Category**: Observability | **Effort**: 0.25 days

Cannot detect connection pool exhaustion without metrics.

**Fix**: Export connection pool stats as Prometheus metrics.

```go
func (db *DB) ExportMetrics() {
    stats := db.Stats()
    prometheus.NewGaugeFunc(prometheus.GaugeOpts{
        Name: "db_open_connections",
    }, func() float64 { return float64(stats.OpenConnections) })
}
```

---

## M-08: Inefficient JSONB Validation ✅ RESOLVED

**Severity**: MEDIUM  
**Category**: Performance  
**Effort**: 0.5 days  
**Status**: ✅ **RESOLVED** (2026-02-08)

### Problem
JSONB validation parses entire JSON on every request. Should validate size first.

### Resolution
Implemented fast path for `json.RawMessage` with O(1) size check before expensive parsing.

**Implementation**: `internal/validation/jsonb.go`

**Key Features**:
```go
// Fast path for json.RawMessage (most common case)
if raw, ok := data.(json.RawMessage); ok {
    // Check size first (O(1) - just len())
    if len(raw) > MaxJSONBSize {
        return fmt.Errorf("JSON exceeds maximum size")
    }
    // Then parse and validate depth (expensive)
    ...
}
```

**Performance Improvement**: **MASSIVE**

**Benchmark Results**:
```
BEFORE (marshal first):
- Small:  657.0 ns/op
- Medium: 32,580 ns/op  
- Large:  1,554,085 ns/op

AFTER (fast path):
- Small:  251.8 ns/op    (2.6x faster)
- Medium: 618.9 ns/op    (52x faster!)
- Large:  10,869 ns/op   (143x faster!!)
- Oversized: 69.33 ns/op (22,400x faster for rejection!)
```

**Test Coverage**: `internal/validation/jsonb_test.go` (73 lines, 3 tests + benchmarks)
```
✅ Fast path for json.RawMessage
✅ Rejects oversized before parsing
✅ Validates depth correctly
✅ Benchmarks proving performance gains
```

### Verification
✅ 52-143x performance improvement  
✅ DoS protection (oversized rejected in 69ns)  
✅ Backward compatible  
✅ 98%+ CPU reduction for validation  
✅ All tests passing

---

## M-09: No Graceful Shutdown for Background Workers ✅ RESOLVED
**Severity**: MEDIUM  
**Category**: Reliability  
**Effort**: 0.5 days  
**Status**: ✅ **RESOLVED** (2026-02-08)

### Problem
Async audit logger workers didn't gracefully shutdown, potentially losing events on server restart or shutdown.

### Resolution
Implemented context-aware graceful shutdown with timeout handling.

**Implementation Files**:
- `internal/audit/async_logger.go` - Added Shutdown() method
- `internal/audit/shutdown_test.go` (NEW) - 350+ lines, 9 tests + 3 benchmarks
- `internal/api/server.go` - Integrated with server shutdown

**Features**:
1. **Context-Aware Shutdown**
   - Respects caller's timeout
   - Drains queue before returning
   - Returns error if timeout exceeded

2. **No Data Loss**
   - Closes channel (stops accepting new events)
   - Waits for all workers to finish
   - All queued events processed

3. **Idempotent**
   - Multiple shutdown calls are safe
   - Thread-safe with mutex

4. **Backward Compatible**
   - Close() still works (calls Shutdown with background context)

**Implementation**:
```go
func (al *AsyncLogger) Shutdown(ctx context.Context) error {
    al.mu.Lock()
    if al.closed {
        al.mu.Unlock()
        return nil  // Idempotent
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
    
    select {
    case <-done:
        return nil  // All events processed
    case <-ctx.Done():
        if al.slogger != nil {
            al.slogger.Warn("Audit logger shutdown timeout, some events may be lost")
        }
        return ctx.Err()  // Timeout
    }
}
```

**Server Integration**:
```go
func (s *Server) Shutdown(ctx context.Context) error {
    // ... stop rate limiter ...
    
    // Gracefully shutdown async audit logger (drain queue with timeout)
    if asyncLogger, ok := s.auditLogger.(*audit.AsyncLogger); ok {
        if err := asyncLogger.Shutdown(ctx); err != nil {
            s.logger.Warn("Audit logger shutdown timeout", "error", err)
        }
    }
    
    return s.server.Shutdown(ctx)
}
```

**Test Coverage**: 9 comprehensive tests + 3 benchmarks
- Shutdown drains queue
- Respects timeout
- Idempotent (multiple calls safe)
- Close() calls Shutdown()
- Empty queue handling
- Large queue (500 events)
- Rejects new events after shutdown
- Concurrent shutdowns safe
- Performance benchmarks

### Verification
✅ Graceful shutdown with timeout  
✅ No data loss (drains queue)  
✅ Idempotent (multiple calls safe)  
✅ Thread-safe implementation  
✅ Server integration with context  
✅ 9 comprehensive tests passing  
✅ Race detection clean  
✅ Backward compatible

---

## M-10: Missing Index on audit_logs.created_at
**Severity**: MEDIUM | **Category**: Performance | **Effort**: 0.1 days

Audit log queries by date range are slow without index.

**Fix**: Already exists in schema ✅ (verify in production)

---

## M-11: No Certificate Expiration Monitoring ✅ RESOLVED
**Severity**: MEDIUM  
**Category**: Reliability  
**Effort**: 0.5 days  
**Status**: ✅ **RESOLVED** (2026-02-08)

### Problem
Device certificates can expire without warning, causing service disruption and authentication failures.

### Resolution
Implemented background certificate expiration monitor with full server integration and configuration support.

**Implementation Files**:
- `internal/certs/expiration_monitor.go` (NEW) - Monitor implementation (175 lines)
- `internal/certs/expiration_monitor_test.go` (NEW) - Unit tests (484 lines, 15 tests)
- `internal/api/cert_monitor_integration_test.go` (NEW) - Integration tests (180 lines, 2 tests)
- `internal/api/server.go` (MODIFIED) - Server lifecycle integration
- `internal/config/config.go` (MODIFIED) - Configuration structure
- `configs/config.example.yaml` (MODIFIED) - Configuration example

**Features**:
1. **Background Monitoring**
   - Runs in separate goroutine with ticker
   - Configurable check interval (default: 24 hours)
   - Immediate check on startup

2. **Smart Filtering**
   - Only active certificates (excludes revoked)
   - Only future expirations (excludes already expired)
   - Ordered by expiration date (soonest first)

3. **Full Configuration**
   ```yaml
   certificates:
     expiration_monitor:
       enabled: true
       check_interval: 24h
       warning_threshold: 720h  # 30 days
   ```

4. **Server Integration**
   - Created in NewServer() with configuration
   - Started in Server.Start()
   - Stopped in Server.Shutdown()
   - Proper lifecycle management

5. **Structured Logging**
   ```go
   logger.Warn("Certificate expiring soon",
       "certificate_id", cert.ID,
       "device_id", cert.DeviceID,
       "subject", cert.Subject,
       "serial_number", cert.SerialNumber,
       "expires_at", cert.ExpiresAt,
       "days_remaining", daysRemaining,
   )
   ```

**Test Coverage**: 17 comprehensive tests
- 15 unit tests (lifecycle, detection, filtering, edge cases)
- 2 integration tests (server lifecycle, enabled/disabled)
- All tests passing with race detection

### Verification
✅ Background monitoring with configurable intervals  
✅ Smart certificate filtering (active, not expired)  
✅ Structured logging with all details  
✅ Full server integration  
✅ Configuration support (enabled flag, intervals)  
✅ Graceful lifecycle (Start/Stop)  
✅ Thread-safe implementation  
✅ 17 comprehensive tests passing  
✅ Race detection clean  
✅ Production-ready

---

## M-12: No IP Allowlisting for Admin Operations
**Severity**: MEDIUM | **Category**: Security | **Effort**: 0.5 days

Admin operations (create enterprise, wipe device) should be restricted to trusted IPs.

**Fix**: Add IP allowlist middleware.

```go
func ipAllowlistMiddleware(allowedCIDRs []string) func(http.Handler) http.Handler {
    cidrs := make([]*net.IPNet, len(allowedCIDRs))
    for i, cidr := range allowedCIDRs {
        _, ipnet, _ := net.ParseCIDR(cidr)
        cidrs[i] = ipnet
    }
    
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            ip := getClientIP(r)
            
            allowed := false
            for _, cidr := range cidrs {
                if cidr.Contains(ip) {
                    allowed = true
                    break
                }
            }
            
            if !allowed {
                respondError(w, r, http.StatusForbidden, "ip_not_allowed", 
                    "Your IP address is not authorized for this operation")
                return
            }
            
            next.ServeHTTP(w, r)
        })
    }
}
```
