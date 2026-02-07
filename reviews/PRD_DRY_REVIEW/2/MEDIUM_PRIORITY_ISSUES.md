# Medium Priority Issues (Fix Within First Month)

**Priority**: MEDIUM  
**Total Issues**: 12  
**Estimated Effort**: 4-5 days  
**Risk Level**: Performance, maintainability concerns

---

## M-01: No Query Result Caching
**Severity**: MEDIUM | **Category**: Performance | **Effort**: 0.5 days

Frequently accessed data (enterprises, policies) is queried repeatedly without caching, causing unnecessary database load.

**Fix**: Implement Redis caching layer with TTL.

---

## M-02: Missing Compression Middleware
**Severity**: MEDIUM | **Category**: Performance | **Effort**: 0.25 days

API responses are not compressed, wasting bandwidth for large payloads (device lists, audit logs).

**Fix**: Add gzip middleware.

```go
import "github.com/gorilla/handlers"

s.router.Use(handlers.CompressHandler)
```

---

## M-03: No Database Query Logging
**Severity**: MEDIUM | **Category**: Observability | **Effort**: 0.5 days

Slow queries cannot be identified without query logging.

**Fix**: Add query logging middleware with duration tracking.

---

## M-04: Missing Health Check Details
**Severity**: MEDIUM | **Category**: Observability | **Effort**: 0.25 days

Health endpoint only checks database, not Keycloak or other dependencies.

**Fix**: Add comprehensive health checks.

```go
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
    health := map[string]string{
        "database": "unknown",
        "keycloak": "unknown",
    }
    
    // Check database
    if err := s.db.Health(ctx); err == nil {
        health["database"] = "healthy"
    } else {
        health["database"] = "unhealthy"
    }
    
    // Check Keycloak
    if err := s.authMiddleware.validator.Health(ctx); err == nil {
        health["keycloak"] = "healthy"
    } else {
        health["keycloak"] = "unhealthy"
    }
    
    status := http.StatusOK
    if health["database"] != "healthy" {
        status = http.StatusServiceUnavailable
    }
    
    respondJSON(w, r, status, health)
}
```

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

## M-08: Inefficient JSONB Validation
**Severity**: MEDIUM | **Category**: Performance | **Effort**: 0.5 days

JSONB validation parses entire JSON on every request. Should validate size first.

**Fix**: Check size before parsing.

```go
func ValidateJSONB(data json.RawMessage, maxDepth int) error {
    // Check size first (cheap)
    if len(data) > MaxJSONBSize {
        return fmt.Errorf("JSONB exceeds maximum size")
    }
    
    // Then parse and validate depth (expensive)
    return validateDepth(data, maxDepth)
}
```

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
