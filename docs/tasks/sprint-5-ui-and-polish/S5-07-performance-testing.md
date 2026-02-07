# S5-07: Performance Testing & Optimization

**Sprint**: 5 — UI & Polish
**Parallel**: ⚠️ Partial (needs other sprints complete for realistic testing)
**Effort**: 2-3 days

## Objective

Define performance targets, conduct load testing, optimize database queries, and tune system for production scale.

## Performance Targets

### API Latency (p95)
- `GET /api/v1/devices` (list): < 200ms
- `GET /api/v1/devices/{id}` (detail): < 100ms
- `POST /api/v1/devices/{id}/lock` (command): < 500ms
- `POST /api/v1/policies` (create): < 150ms
- `GET /api/v1/policies` (list): < 200ms

### Enrollment Latency
- Windows enrollment: < 30s (discovery → enrolled)
- macOS enrollment: < 20s (profile install → enrolled)
- Android enrollment: < 15s (QR scan → enrolled)

### Command Dispatch
- Command queued: < 5s
- Command delivered to device: < 30s (depends on device check-in)

### Throughput
- Concurrent enrollments: 50 devices/minute
- API requests: 1000 req/s per server instance
- Device check-ins: 10,000 devices/hour

### Scale Targets
- Total devices: 10,000+ per enterprise
- Concurrent users: 100+ admins
- Enterprises: 100+ tenants

### Database Performance
- Query latency (p95): < 50ms
- Connection pool utilization: < 80%
- Index hit rate: > 95%

## Tasks

### 1. Load Testing Framework
- Install k6 or Locust for load testing
- Create test scenarios for each endpoint
- Simulate realistic device enrollment patterns
- Files: `tests/load/`, `tests/load/scenarios/`

**Test Scenarios**:
- Enrollment burst (100 devices in 2 minutes)
- Steady state (1000 devices checking in over 1 hour)
- Admin dashboard load (10 admins browsing devices)
- Policy deployment (push policy to 1000 devices)
- Command execution (lock 100 devices simultaneously)

### 2. Database Query Optimization
- Analyze slow queries with `EXPLAIN ANALYZE`
- Add missing indexes
- Optimize JSONB queries
- Review connection pool settings
- Files: `migrations/000XXX_add_performance_indexes.up.sql`

**Indexes to Review**:
```sql
-- Device queries
CREATE INDEX idx_devices_enterprise_platform ON devices(enterprise_id, platform);
CREATE INDEX idx_devices_last_seen_status ON devices(last_seen, status);
CREATE INDEX idx_devices_platform_data_gin ON devices USING gin(platform_data);

-- Policy queries
CREATE INDEX idx_policies_enterprise_active ON policies(enterprise_id, is_active);
CREATE INDEX idx_device_policies_device_status ON device_policies(device_id, status);

-- Audit log queries
CREATE INDEX idx_audit_logs_enterprise_created ON audit_logs(enterprise_id, created_at DESC);
CREATE INDEX idx_audit_logs_actor_action ON audit_logs(actor_id, action);
```

### 3. Connection Pool Tuning
- Benchmark different pool sizes
- Configure idle timeout
- Configure max lifetime
- Monitor pool metrics
- Files: `internal/db/connection.go`

**Recommended Settings**:
```yaml
database:
  max_open_conns: 25
  max_idle_conns: 10
  conn_max_lifetime: 5m
  conn_max_idle_time: 2m
```

### 4. Caching Strategy
- Identify cacheable data (device status, policy definitions)
- Implement in-memory cache with TTL
- Cache invalidation strategy
- Optional: Redis for distributed caching
- Files: `internal/cache/cache.go`

**Cache Candidates**:
- Device list (per enterprise, TTL: 30s)
- Policy definitions (TTL: 5m)
- User permissions (TTL: 5m)
- Keycloak JWKS (TTL: 1h)

### 5. Performance Profiling
- Add pprof endpoints for CPU/memory profiling
- Profile under load
- Identify bottlenecks
- Optimize hot paths
- Files: `internal/api/server.go` (pprof routes)

**Profiling Endpoints** (dev only):
```
GET /debug/pprof/
GET /debug/pprof/heap
GET /debug/pprof/goroutine
GET /debug/pprof/profile?seconds=30
```

## Load Testing Example (k6)

```javascript
// tests/load/enrollment_burst.js
import http from 'k6/http';
import { check, sleep } from 'k6';

export let options = {
  stages: [
    { duration: '30s', target: 50 },  // Ramp up to 50 VUs
    { duration: '1m', target: 50 },   // Stay at 50 VUs
    { duration: '30s', target: 0 },   // Ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<500'], // 95% of requests < 500ms
    http_req_failed: ['rate<0.01'],   // Error rate < 1%
  },
};

export default function () {
  let res = http.post('http://localhost:8080/EnrollmentServer/Enrollment.svc', 
    enrollmentPayload, 
    { headers: { 'Content-Type': 'application/soap+xml' } }
  );
  
  check(res, {
    'status is 200': (r) => r.status === 200,
    'enrollment successful': (r) => r.body.includes('ProvisioningDoc'),
  });
  
  sleep(1);
}
```

## Benchmarking Commands

```bash
# Run load test
k6 run tests/load/enrollment_burst.js

# Profile CPU for 30 seconds
curl http://localhost:8080/debug/pprof/profile?seconds=30 > cpu.prof
go tool pprof cpu.prof

# Profile memory
curl http://localhost:8080/debug/pprof/heap > heap.prof
go tool pprof heap.prof

# Analyze slow queries
psql -d localmdm -c "SELECT query, mean_exec_time, calls FROM pg_stat_statements ORDER BY mean_exec_time DESC LIMIT 10;"
```

## Performance Optimization Checklist

- [ ] All queries use appropriate indexes
- [ ] JSONB queries use GIN indexes
- [ ] Connection pool sized correctly
- [ ] Caching implemented for hot paths
- [ ] No N+1 query problems
- [ ] Pagination enforced on list endpoints
- [ ] Request timeouts configured
- [ ] Database prepared statements used
- [ ] Goroutine leaks checked
- [ ] Memory allocations minimized in hot paths

## Acceptance Criteria

- [ ] All performance targets met under load
- [ ] Load tests pass with 50 concurrent enrollments
- [ ] API latency p95 < 200ms for list endpoints
- [ ] Database query latency p95 < 50ms
- [ ] No memory leaks detected in 1-hour load test
- [ ] Connection pool utilization < 80% under load
- [ ] Performance documentation complete

## Performance Monitoring (Ongoing)

Add to Grafana dashboard:
- API latency percentiles (p50, p95, p99)
- Enrollment success rate
- Database query latency
- Connection pool usage
- Cache hit rate
- Goroutine count
- Memory usage

## Future Optimizations

- Read replicas for device list queries
- Horizontal scaling with load balancer
- CDN for web dashboard static assets
- Database partitioning for audit logs
- Message queue for async command processing
