# Remaining Tasks - Priority Order

**Date**: 2026-02-08  
**Status**: H-01 Complete, Ready for Next Tasks

---

## HIGH PRIORITY (3 Remaining)

### H-03: No Graceful Degradation for Non-Critical Features
**Effort**: 0.5 days | **Impact**: Complete outage for partial failures

**Problem**: Audit logging failures block requests instead of degrading gracefully.

**Solution**: Implement async audit logging with buffering
- Background workers (3 goroutines)
- Buffered channel (1000 events)
- Drop events if buffer full (log warning)
- Graceful shutdown with flush

**Files to Create**:
- `internal/audit/async_logger.go` (new)
- `internal/audit/async_logger_test.go` (new)

**Files to Modify**:
- `internal/auth/middleware.go` - Use async logger
- `internal/api/server.go` - Initialize async logger

---

### H-06: Audit Logs Unbounded (No Archival)
**Effort**: 0.5 days | **Impact**: Database growth, performance degradation

**Problem**: Audit logs grow indefinitely, causing:
- Table size → hundreds of GB
- Query slowdown
- Disk space exhaustion

**Solution**: Implement table partitioning + archival
- Monthly partitions
- Archive partitions >90 days to S3
- Drop archived partitions
- Automated via cron job

**Files to Create**:
- `migrations/000XXX_audit_log_partitioning.up.sql` (new)
- `migrations/000XXX_audit_log_partitioning.down.sql` (new)
- `scripts/archive_audit_logs.sh` (new)

**Configuration**:
```yaml
audit:
  retention_days: 90
  archive_enabled: true
  archive_bucket: "mdm-audit-logs"
```

---

### H-07: No Distributed Tracing
**Effort**: 1 day | **Impact**: Difficult to debug production issues

**Problem**: Cannot track requests across services or identify slow operations.

**Solution**: Implement OpenTelemetry tracing
- Jaeger or AWS X-Ray integration
- Automatic HTTP instrumentation
- Database span tracking
- Configurable sampling

**Files to Create**:
- `internal/tracing/tracing.go` (new)
- `internal/tracing/tracing_test.go` (new)

**Files to Modify**:
- `cmd/server/main.go` - Initialize tracer
- `internal/api/server.go` - Add tracing middleware
- `internal/config/config.go` - Add tracing config

**Configuration**:
```yaml
tracing:
  enabled: true
  endpoint: "localhost:4317"  # OTLP gRPC endpoint
  sampling_rate: 1.0          # 100% for dev, 0.1 for prod
```

---

## MEDIUM PRIORITY (8 Remaining)

### M-01: No Query Result Caching
**Effort**: 0.5 days | **Impact**: Unnecessary database load

**Problem**: Frequently accessed data (enterprises, policies) queried repeatedly.

**Solution**: Redis caching layer with TTL
- Cache enterprises (TTL: 1 hour)
- Cache policies (TTL: 5 minutes)
- Cache-aside pattern
- Invalidation on updates

**Files to Create**:
- `internal/cache/cache.go` (new)
- `internal/cache/cache_test.go` (new)

**Files to Modify**:
- `internal/repository/enterprise.go` - Add caching
- `internal/repository/policy.go` - Add caching

---

### M-03: No Database Query Logging
**Effort**: 0.5 days | **Impact**: Cannot identify slow queries

**Problem**: No visibility into query performance.

**Solution**: Query logging middleware
- Log queries >100ms
- Include query, duration, rows affected
- Structured logging with context

**Files to Create**:
- `internal/db/query_logger.go` (new)
- `internal/db/query_logger_test.go` (new)

**Files to Modify**:
- `internal/db/db.go` - Add query logging wrapper

---

### M-05: No Metrics Collection
**Effort**: 0.5 days | **Impact**: No operational visibility

**Problem**: No metrics for monitoring (request rate, latency, errors).

**Solution**: Prometheus metrics
- HTTP request metrics (rate, duration, status)
- Database metrics (connections, query duration)
- Circuit breaker metrics (state, failures)
- Cache metrics (hits, misses)

**Files to Create**:
- `internal/metrics/metrics.go` (new)
- `internal/api/metrics_middleware.go` (new)

**Files to Modify**:
- `internal/api/server.go` - Add metrics middleware
- `internal/auth/circuit_breaker.go` - Add metrics

---

### M-06: Request ID Not Propagated ✅ RESOLVED
**Status**: ✅ Complete

---

### M-07: No Structured Logging for Audit Events ✅ RESOLVED
**Effort**: 0.25 days | **Impact**: Difficult to query audit logs  
**Status**: ✅ **COMPLETE** (2026-02-08)

**Problem**: Audit logs stored as JSONB but not structured in application logs.

**Solution Implemented**: Structured audit logging
- Added slog integration to audit logger
- Log audit events with structured fields (enterprise_id, user_id, action, resource_type, resource_id, ip_address)
- Log both success and failure events
- Searchable in log aggregation tools

**Files Modified**:
- `internal/audit/audit.go` - Added logger field and structured logging
- `internal/audit/structured_logging_test.go` (NEW) - 237 lines of tests

---

### M-08: Missing Index on audit_logs.created_at ✅ RESOLVED
**Effort**: 0.1 days | **Impact**: Slow audit log queries  
**Status**: ✅ **COMPLETE** (2026-02-08)

**Problem**: Queries by date range with `ORDER BY created_at DESC` are slow.

**Solution Implemented**: Add optimized descending index
```sql
CREATE INDEX idx_audit_logs_created_at_desc ON audit_logs(created_at DESC);
```

**Benefits**:
- Optimized for `ORDER BY created_at DESC` queries
- Faster pagination through recent logs
- Better query planner decisions

**Files Created**:
- `migrations/000002_audit_log_index_optimization.up.sql` (NEW)
- `migrations/000002_audit_log_index_optimization.down.sql` (NEW)

---

### M-09: No Connection Pool Monitoring
**Effort**: 0.25 days | **Impact**: Cannot detect pool exhaustion

**Problem**: No visibility into connection pool usage.

**Solution**: Pool metrics
- Expose pool stats (open, idle, in-use, wait count)
- Log warnings when pool >80% utilized
- Prometheus metrics

---

### M-10: Missing Request Size Limits
**Effort**: 0.25 days | **Impact**: DoS via large payloads

**Problem**: No limit on request body size.

**Solution**: Add middleware
```go
http.MaxBytesReader(w, r.Body, 10<<20)  // 10MB limit
```

---

### M-11: No Graceful Shutdown
**Effort**: 0.5 days | **Impact**: In-flight requests dropped on restart

**Problem**: Server stops immediately, dropping active requests.

**Solution**: Graceful shutdown
- Catch SIGTERM/SIGINT
- Stop accepting new requests
- Wait for in-flight requests (30s timeout)
- Close database connections
- Flush audit logs

---

### M-12: Missing API Versioning
**Effort**: 0.5 days | **Impact**: Breaking changes affect all clients

**Problem**: No API versioning strategy.

**Solution**: URL-based versioning
- `/api/v1/devices`
- Support multiple versions simultaneously
- Deprecation warnings in headers

---

## RECOMMENDED ORDER

### Phase 1: Reliability (1.5 days)
1. **H-03**: Async audit logging (0.5 days)
2. **M-11**: Graceful shutdown (0.5 days)
3. **M-10**: Request size limits (0.25 days)
4. **M-08**: Audit log index (0.1 days)

### Phase 2: Observability (2 days)
5. **H-07**: Distributed tracing (1 day)
6. **M-05**: Metrics collection (0.5 days)
7. **M-03**: Query logging (0.5 days)

### Phase 3: Performance (1.5 days)
8. **M-01**: Query result caching (0.5 days)
9. **H-06**: Audit log archival (0.5 days)
10. **M-09**: Connection pool monitoring (0.25 days)

### Phase 4: API Quality (0.75 days)
11. **M-12**: API versioning (0.5 days)
12. **M-07**: Structured audit logging (0.25 days)

---

## QUICK WINS (Start Here)

These can be done quickly and provide immediate value:

1. **M-08**: Add audit log index (10 minutes)
2. **M-10**: Request size limits (30 minutes)
3. **M-07**: Structured audit logging (1 hour)

---

## SUMMARY

**Total Remaining**: 11 issues  
**Total Effort**: ~6 days  
**High Priority**: 3 issues (2 days)  
**Medium Priority**: 8 issues (4 days)

**Recommended Start**: Phase 1 (Reliability) - Quick wins + critical reliability fixes
