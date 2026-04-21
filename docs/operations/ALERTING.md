# Alerting Guide

## Prometheus Alert Rules

### Service Health
```yaml
groups:
  - name: localmdm
    rules:
      - alert: ServiceDown
        expr: up{job="localmdm"} == 0
        for: 1m
        labels: { severity: critical }
        annotations: { summary: "Local MDM service is down" }

      - alert: HighErrorRate
        expr: rate(http_requests_total{status=~"5.."}[5m]) / rate(http_requests_total[5m]) > 0.05
        for: 5m
        labels: { severity: warning }
        annotations: { summary: "Error rate above 5%" }

      - alert: SlowAPI
        expr: histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m])) > 0.5
        for: 5m
        labels: { severity: warning }
        annotations: { summary: "API p95 latency above 500ms" }
```

### Database
```yaml
      - alert: DatabaseDown
        expr: db_open_connections == 0
        for: 30s
        labels: { severity: critical }
        annotations: { summary: "Database connection pool empty" }

      - alert: HighDBConnections
        expr: db_open_connections / 25 > 0.8
        for: 5m
        labels: { severity: warning }
        annotations: { summary: "DB connection pool above 80%" }
```

### Certificates
```yaml
      - alert: CertExpiringSoon
        expr: certificates_expiring_soon{days="30"} > 0
        for: 1h
        labels: { severity: warning }
        annotations: { summary: "Certificates expiring within 30 days" }

      - alert: CertExpiringCritical
        expr: certificates_expiring_soon{days="7"} > 0
        for: 1h
        labels: { severity: critical }
        annotations: { summary: "Certificates expiring within 7 days" }
```

### Enrollment
```yaml
      - alert: EnrollmentFailures
        expr: rate(enrollments_total{status="failed"}[15m]) > 0.1
        for: 5m
        labels: { severity: warning }
        annotations: { summary: "Enrollment failure rate elevated" }

      - alert: SlowEnrollment
        expr: histogram_quantile(0.95, rate(enrollment_duration_seconds_bucket[15m])) > 30
        for: 5m
        labels: { severity: warning }
        annotations: { summary: "Enrollment p95 above 30 seconds" }
```

## Prometheus Scrape Config

```yaml
scrape_configs:
  - job_name: localmdm
    scrape_interval: 15s
    static_configs:
      - targets: ['localhost:9090']
```

For ECS Fargate, use CloudWatch Agent sidecar to scrape `localhost:9090/metrics` and forward to CloudWatch Metrics.

## Key Metrics

| Metric | Type | Labels | Purpose |
|--------|------|--------|---------|
| `http_requests_total` | Counter | method, path, status | Request volume and error rates |
| `http_request_duration_seconds` | Histogram | method, path | API latency |
| `devices_total` | Gauge | platform, status | Device inventory |
| `enrollments_total` | Counter | platform, status | Enrollment tracking |
| `enrollment_duration_seconds` | Histogram | platform | Enrollment performance |
| `certificates_expiring_soon` | Gauge | days | Certificate lifecycle |
| `db_open_connections` | Gauge | — | Connection pool health |
| `commands_queued_total` | Counter | command_type | Command dispatch volume |

## Health Endpoints

- `GET /health` — Liveness probe (returns 200 if process is running, checks DB)
- `GET /health/ready` — Readiness probe (checks DB + Keycloak with per-dependency latency)
