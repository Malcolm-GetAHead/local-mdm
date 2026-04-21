# S5-06: Observability & Operations (Reduced Scope)

**Sprint**: 5 — Backend Polish  
**Parallel**: ✅ Yes  
**Effort**: 1-1.5 days (reduced from 3-4 days)

## What Sprint 2 Already Delivered

- ✅ Prometheus metrics on separate internal port (127.0.0.1:9090)
  - `http_requests_total`, `http_request_duration_seconds`, `http_active_requests`
  - `enrollments_total` by platform/status
  - `commands_queued_total`, `commands_pending`
  - `db_open_connections`, `db_idle_connections`, `db_wait_count_total`
  - Middleware auto-instruments all HTTP requests using mux route templates
- ✅ Structured logging via slog (JSON output, configurable level)
- ✅ Request ID middleware (X-Request-ID header, propagated to audit log details)
- ✅ `/health` endpoint with DB and Keycloak dependency checks
- ✅ Certificate expiration monitor (background goroutine, configurable threshold)

## Remaining Work

### 1. Add `/health/ready` Endpoint (0.25 days)

The existing `/health` endpoint checks dependencies but doesn't distinguish between liveness and readiness. Load balancers (ALB, Kubernetes) and container orchestrators need both:

- `/health` (liveness) — "is the process running?" Always returns 200 unless the server is broken
- `/health/ready` (readiness) — "can this instance serve traffic?" Checks DB, Keycloak

```go
// Add to setupRoutes:
s.router.HandleFunc("/health/ready", s.handleHealthReady).Methods("GET")
```

Response should include latency per dependency and overall status (available/degraded/unavailable).

### 2. Add Missing Metrics (0.25 days)

Small additions to the existing `internal/metrics/metrics.go`:

| Metric | Type | Why |
|--------|------|-----|
| `devices_total` | GaugeVec (platform, status) | Dashboard "total enrolled devices" |
| `certificates_expiring_soon` | GaugeVec (days: 7, 30, 90) | Alert on upcoming expirations |
| `enrollment_duration_seconds` | HistogramVec (platform) | Track enrollment performance |

These are 3 new collectors added to the existing `Metrics` struct. The cert expiry gauge can be updated by the existing `ExpirationMonitor`.

### 3. Alerting Documentation (0.5 days)

Create `docs/operations/ALERTING.md` with recommended Prometheus alert rules:

| Alert | Condition | Severity |
|-------|-----------|----------|
| Service Down | `up{job="local-mdm"} == 0` | Critical |
| Database Down | `/health/ready` returns unhealthy for DB | Critical |
| High Error Rate | `rate(http_requests_total{status=~"5.."}[5m]) > 0.05` | High |
| Certificate Expiring | `certificates_expiring_soon{days="7"} > 0` | High |
| Enrollment Failures | `rate(enrollments_total{status="failure"}[5m]) > 0.1` | Medium |
| Slow API | `histogram_quantile(0.95, http_request_duration_seconds) > 2` | Medium |

Include a basic Prometheus scrape config and Grafana dashboard JSON.

### 4. Backup Documentation (0.25 days)

Create `docs/operations/BACKUP.md` documenting what needs backup beyond PostgreSQL:

- `secrets/` directory (DEP encryption key, any file-based secrets)
- CA certificate and private key (if file-based, per steering guide)
- `configs/config.yaml`
- APNs certificates (if stored on filesystem)

Note: Database backup/restore is PostgreSQL infrastructure, not application-level.

## Out of Scope (Deferred to F-05)

The following are planned for [F-05: Advanced Monitoring](../../future/F-05-advanced-monitoring.md):
- Distributed tracing (OpenTelemetry spans across services)
- APM integration
- SLO definition and tracking
- Error budget monitoring
- Grafana dashboard templates

## Acceptance Criteria

- [ ] `/health/ready` checks all dependencies with latency
- [ ] `devices_total`, `certificates_expiring_soon`, `enrollment_duration_seconds` metrics exposed
- [ ] Alerting documentation with recommended rules and thresholds
- [ ] Backup documentation listing all non-database items

---

*Updated: 2026-04-18 — Reduced scope to reflect Sprint 2 observability work*
