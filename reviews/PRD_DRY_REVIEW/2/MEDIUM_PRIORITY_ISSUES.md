# Medium Priority Issues (Fix Within First Month)

**Priority**: MEDIUM  
**Total Issues**: 12  
**Resolved**: 3 ✅  
**Remaining**: 9  
**Estimated Effort**: 3.5 days (remaining)  
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

## M-06: Missing Request ID Propagation
**Severity**: MEDIUM | **Category**: Observability | **Effort**: 0.25 days

Request IDs are generated but not propagated to database queries or external calls.

**Fix**: Add request ID to context and log it in all operations.

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

## M-09: No Graceful Shutdown for Background Workers
**Severity**: MEDIUM | **Category**: Reliability | **Effort**: 0.5 days

Async audit logger workers don't gracefully shutdown, potentially losing events.

**Fix**: Add shutdown signal handling.

```go
func (al *AsyncLogger) Shutdown(ctx context.Context) error {
    close(al.eventQueue)
    
    // Wait for workers to drain queue
    done := make(chan struct{})
    go func() {
        al.wg.Wait()
        close(done)
    }()
    
    select {
    case <-done:
        return nil
    case <-ctx.Done():
        return ctx.Err()
    }
}
```

---

## M-10: Missing Index on audit_logs.created_at
**Severity**: MEDIUM | **Category**: Performance | **Effort**: 0.1 days

Audit log queries by date range are slow without index.

**Fix**: Already exists in schema ✅ (verify in production)

---

## M-11: No Certificate Expiration Monitoring
**Severity**: MEDIUM | **Category**: Reliability | **Effort**: 0.5 days

Device certificates can expire without warning.

**Fix**: Add background job to check expiring certificates.

```go
func (s *CertService) CheckExpiringCerts(ctx context.Context) error {
    threshold := time.Now().Add(30 * 24 * time.Hour)
    
    certs, err := s.repo.GetExpiringBefore(ctx, threshold)
    if err != nil {
        return err
    }
    
    for _, cert := range certs {
        s.logger.Warn("Certificate expiring soon",
            "device_id", cert.DeviceID,
            "expires_at", cert.ExpiresAt,
        )
        // Send alert
    }
    
    return nil
}
```

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
