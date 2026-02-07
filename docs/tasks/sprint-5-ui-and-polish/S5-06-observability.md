# S5-06: Observability & Operations

**Sprint**: 5 — UI & Polish
**Parallel**: ✅ Yes
**Effort**: 3-4 days

## Objective

Implement comprehensive observability with health checks, Prometheus metrics, structured logging, and operational documentation.

## Tasks

### 1. Enhanced Health Checks
- `/health` - Basic liveness check (service is running)
- `/health/ready` - Readiness check (service + all dependencies available)
- Dependency polling with status reporting
- Files: `internal/health/health.go`, `internal/health/checks.go`

**Health Check Response**:
```json
{
  "status": "available",
  "service": "up",
  "dependencies": {
    "database": {
      "status": "up",
      "latency_ms": 2,
      "last_check": "2026-02-06T10:30:00Z"
    },
    "keycloak": {
      "status": "up",
      "latency_ms": 15,
      "last_check": "2026-02-06T10:30:00Z"
    },
    "scep": {
      "status": "up",
      "latency_ms": 5,
      "last_check": "2026-02-06T10:30:00Z"
    }
  },
  "version": "1.0.0",
  "uptime_seconds": 3600
}
```

**Dependency Checks**:
- Database: `SELECT version()` every 30 seconds
- Keycloak: `GET /.well-known/openid-configuration` every 60 seconds
- SCEP: `GET /health` every 60 seconds
- APNs: Connection test every 5 minutes (macOS only)

**Status Values**:
- `up` - Dependency is healthy
- `degraded` - Dependency is slow (>1s latency)
- `down` - Dependency is unreachable

**Service Status**:
- `available` - Service and all dependencies are up
- `degraded` - Service is up but some dependencies are degraded/down
- `unavailable` - Service cannot function (critical dependency down)

### 2. Prometheus Metrics
- `/metrics` - Prometheus-formatted metrics endpoint
- Custom metrics for MDM operations
- Standard Go runtime metrics
- Files: `internal/metrics/metrics.go`, `internal/metrics/collectors.go`

**Metrics to Expose**:

```
# Enrollment metrics
mdm_enrollments_total{platform="windows|macos|android",status="success|failure"}
mdm_enrollment_duration_seconds{platform="windows|macos|android"}

# Device metrics
mdm_devices_total{platform="windows|macos|android",status="enrolled|unenrolled"}
mdm_device_last_seen_seconds{platform="windows|macos|android"}

# Command metrics
mdm_commands_total{platform="windows|macos|android",command="lock|wipe|install_app",status="success|failure"}
mdm_command_duration_seconds{platform="windows|macos|android",command="lock|wipe|install_app"}

# Policy metrics
mdm_policy_deployments_total{platform="windows|macos|android",status="success|failure"}
mdm_compliance_status{platform="windows|macos|android",status="compliant|non_compliant"}

# API metrics
http_requests_total{method="GET|POST|PUT|DELETE",path="/api/v1/devices",status="200|400|500"}
http_request_duration_seconds{method="GET|POST|PUT|DELETE",path="/api/v1/devices"}

# Database metrics
db_connections_open
db_connections_in_use
db_query_duration_seconds{query="device_list|device_get|policy_list"}

# Certificate metrics
mdm_certificates_issued_total{type="device|apns"}
mdm_certificates_expiring_soon{days="7|30|90"}
```

### 3. Structured Logging
- JSON-formatted logs to stdout
- Correlation IDs for request tracing
- Log levels: DEBUG, INFO, WARN, ERROR
- Contextual logging (enterprise_id, device_id, user_id)
- Files: `internal/logging/logger.go`, `internal/logging/context.go`

**Log Format**:
```json
{
  "timestamp": "2026-02-06T10:30:00Z",
  "level": "INFO",
  "message": "Device enrolled successfully",
  "correlation_id": "req-123e4567-e89b-12d3-a456-426614174000",
  "enterprise_id": "ent-123e4567-e89b-12d3-a456-426614174000",
  "device_id": "dev-123e4567-e89b-12d3-a456-426614174000",
  "platform": "windows",
  "duration_ms": 234,
  "user_id": "user-123e4567-e89b-12d3-a456-426614174000"
}
```

### 4. Alerting Documentation
- Document recommended alerts based on metrics
- Alert thresholds and severity levels
- Runbook for common issues
- Files: `docs/operations/ALERTING.md`, `docs/operations/RUNBOOK.md`

**Recommended Alerts**:

| Alert | Metric | Threshold | Severity |
|-------|--------|-----------|----------|
| Service Down | `up{job="local-mdm"}` | `== 0` | Critical |
| Database Down | `mdm_dependency_status{name="database"}` | `== 0` | Critical |
| High Error Rate | `rate(http_requests_total{status=~"5.."}[5m])` | `> 0.05` | High |
| Enrollment Failures | `rate(mdm_enrollments_total{status="failure"}[5m])` | `> 0.1` | Medium |
| Certificate Expiring | `mdm_certificates_expiring_soon{days="7"}` | `> 0` | High |
| Slow API Response | `histogram_quantile(0.95, http_request_duration_seconds)` | `> 2` | Medium |
| Device Not Seen | `mdm_device_last_seen_seconds` | `> 604800` (7 days) | Low |

### 5. Backup Documentation
- Document what needs backup (outside database)
- Backup procedures for file-based secrets
- Certificate backup procedures
- Files: `docs/operations/BACKUP.md`

**Items Requiring Backup** (non-database):
- `secrets/` directory (if using file-based secrets)
- APNs certificates (if stored on filesystem)
- Root CA private key (if stored on filesystem)
- Configuration files (`configs/config.yaml`)

**Note**: Database backup/restore is handled by PostgreSQL infrastructure (not in scope).

## Configuration Changes

Update `configs/config.yaml`:

```yaml
observability:
  health_checks:
    enabled: true
    interval_seconds: 30
    timeout_seconds: 5
  
  metrics:
    enabled: true
    path: "/metrics"
  
  logging:
    level: "INFO"  # DEBUG, INFO, WARN, ERROR
    format: "json"  # json, text
    output: "stdout"
```

## Acceptance Criteria

- [ ] `/health` returns service status
- [ ] `/health/ready` checks all dependencies
- [ ] `/metrics` exposes Prometheus metrics
- [ ] Logs output in JSON format to stdout
- [ ] Correlation IDs present in all logs
- [ ] Alerting documentation complete with thresholds
- [ ] Backup documentation complete

## Integration with Monitoring Stack

**Prometheus** (scrape config):
```yaml
scrape_configs:
  - job_name: 'local-mdm'
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: '/metrics'
    scrape_interval: 15s
```

**Grafana Dashboard** (future):
- Device enrollment rate
- API request latency (p50, p95, p99)
- Error rate by endpoint
- Database connection pool usage
- Certificate expiration timeline

**Log Aggregation** (ELK/Loki):
- Logs already in JSON format
- Correlation IDs enable request tracing
- Structured fields enable filtering/aggregation
