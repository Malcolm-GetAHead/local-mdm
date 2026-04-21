# Operations Guide

## Backup

### PostgreSQL

The `localmdm` and `keycloak` databases share the same PostgreSQL instance.

```bash
# Full backup (both databases)
pg_dump -h localhost -U postgres -d localmdm -F c -f localmdm_$(date +%Y%m%d).dump
pg_dump -h localhost -U postgres -d keycloak -F c -f keycloak_$(date +%Y%m%d).dump

# Restore
pg_restore -h localhost -U postgres -d localmdm localmdm_20260421.dump

# Production (RDS) — use automated snapshots + manual before migrations
aws rds create-db-snapshot \
  --db-instance-identifier localmdm-primary \
  --db-snapshot-identifier localmdm-pre-migration-$(date +%Y%m%d)
```

**What to back up:**
- `localmdm` database — all application data, device records, policies, DEP tokens (encrypted), audit logs
- `keycloak` database — user accounts, realm config, client settings

### CA Certificates

The CA private key is critical — losing it means re-enrolling all devices.

```bash
# Dev: files in certs/ directory
cp certs/ca.crt certs/ca.key /secure-backup/

# Production: stored in AWS Secrets Manager or filesystem
# Back up to encrypted S3 bucket
aws s3 cp certs/ca.key s3://localmdm-backups/certs/ca.key --sse aws:kms
aws s3 cp certs/ca.crt s3://localmdm-backups/certs/ca.crt --sse aws:kms
```

### Secrets

```bash
# Dev: secrets/ directory (gitignored)
tar czf secrets_backup_$(date +%Y%m%d).tar.gz secrets/

# Production: SSM parameters — export for disaster recovery
aws ssm get-parameters-by-path \
  --path /localmdm/prod/ \
  --with-decryption \
  --query 'Parameters[*].[Name,Value]' \
  --output text > ssm_backup.txt
# Store ssm_backup.txt in encrypted, offline storage — then delete it
```

### Keycloak Realm

```bash
# Export realm config (run inside Keycloak container)
docker exec localmdm-keycloak /opt/keycloak/bin/kc.sh export \
  --dir /tmp/export --realm localmdm

# Copy export out
docker cp localmdm-keycloak:/tmp/export/localmdm-realm.json ./backups/
```

---

## Certificate Renewal

### APNs Certificate (macOS Push)

APNs certificates expire annually. Renewal requires an Apple Developer account.

1. Generate a new CSR from the existing CA key
2. Upload to Apple Push Certificates Portal
3. Download the renewed `.pem` certificate
4. Convert to `.p12` and update `macos.apns_cert_path` in config
5. Restart the service — no device re-enrollment needed if the push topic is unchanged

### CA Certificate

The internal CA signs device certificates. Default validity: configurable via `certificates.device_cert_validity`.

**Rotation strategy:**
1. Generate a new CA keypair
2. Update `certificates.ca_cert_path` and `certificates.ca_key_path`
3. New enrollments use the new CA; existing devices continue until their certs expire
4. Monitor expiring certs via the `certificates_expiring_soon` Prometheus metric

### TLS Certificate

- **ALB (production):** ACM certificates auto-renew. No action needed.
- **nginx (self-hosted):** Use certbot/Let's Encrypt with auto-renewal cron.
- **Dev:** TLS disabled by default (`server.tls.enabled: false`).

---

## Monitoring

### Prometheus Metrics

Metrics are served on a separate internal port (not exposed to the internet):

```
http://127.0.0.1:9090/metrics
```

### Key Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `http_requests_total` | Counter | Requests by method, path, status |
| `http_request_duration_seconds` | Histogram | Request latency by method, path |
| `http_active_requests` | Gauge | In-flight requests |
| `db_open_connections` | Gauge | Open database connections |
| `db_idle_connections` | Gauge | Idle database connections |
| `db_wait_count_total` | Gauge | Connections waited for (pool exhaustion) |
| `enrollments_total` | Counter | Enrollment attempts by platform, status |
| `commands_queued_total` | Counter | Commands queued by type |
| `commands_pending` | Gauge | Pending command queue depth |
| `devices_total` | Gauge | Devices by platform, status |
| `certificates_expiring_soon` | Gauge | Certs expiring within threshold (by days bucket) |
| `enrollment_duration_seconds` | Histogram | Enrollment time by platform |

### Alerts to Configure

```yaml
# High error rate
- alert: HighErrorRate
  expr: rate(http_requests_total{status=~"5.."}[5m]) > 0.1

# Database connection pool exhaustion
- alert: DBPoolExhausted
  expr: db_open_connections > 20  # max_open_conns default is 25

# Slow requests
- alert: SlowRequests
  expr: histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m])) > 5

# Certificates expiring
- alert: CertsExpiringSoon
  expr: certificates_expiring_soon{days="7"} > 0

# Enrollment failures
- alert: EnrollmentFailures
  expr: rate(enrollments_total{status="failure"}[15m]) > 0.5
```

### Production (CloudWatch)

In ECS Fargate, use a CloudWatch Agent sidecar to scrape Prometheus metrics from `localhost:9090` and forward to CloudWatch Metrics. No separate Prometheus server needed.

---

## Log Management

### Format

Logs use Go's `slog` with structured JSON output:

```json
{
  "time": "2026-04-21T16:00:00.000Z",
  "level": "INFO",
  "msg": "Device created",
  "device_id": "550e8400-e29b-41d4-a716-446655440000",
  "enterprise_id": "660e8400-e29b-41d4-a716-446655440001",
  "platform": "windows"
}
```

### Configuration

```yaml
logging:
  level: "info"      # debug, info, warn, error
  format: "json"     # json or text
  output: "stdout"   # stdout or file
  file_path: "./logs/app.log"  # only used when output: file
```

### Production (ECS)

Container stdout goes to CloudWatch Logs via the `awslogs` driver:

```json
"logConfiguration": {
  "logDriver": "awslogs",
  "options": {
    "awslogs-group": "/ecs/localmdm",
    "awslogs-region": "us-east-1",
    "awslogs-stream-prefix": "localmdm"
  }
}
```

Use CloudWatch Logs Insights to query:

```
# Errors in the last hour
fields @timestamp, @message
| filter @message like /ERROR/
| sort @timestamp desc
| limit 50

# Slow database queries
fields @timestamp, msg, latency
| filter msg = "query completed" and latency > 1000
| sort latency desc
```

### Audit Logs

Mutation operations (create, update, delete) are logged asynchronously via the audit subsystem. Audit entries are stored in PostgreSQL (`audit_logs` table) and include:

- Action performed
- Resource type and ID
- Actor (from JWT)
- Timestamp
- Request details

---

## Scaling

### Stateless Design

Local MDM is stateless by design — all shared state lives in PostgreSQL:

- Token cache → PostgreSQL (not Redis)
- Idempotency keys → PostgreSQL (24h TTL, hourly cleanup)
- SCEP challenges → PostgreSQL
- Session state → none (JWT-based auth)

Any instance can handle any request. No sticky sessions required.

### Horizontal Scaling (ECS)

```bash
# Scale localmdm tasks
aws ecs update-service \
  --cluster localmdm \
  --service localmdm \
  --desired-count 4

# Auto-scaling based on CPU
aws application-autoscaling register-scalable-target \
  --service-namespace ecs \
  --resource-id service/localmdm/localmdm \
  --scalable-dimension ecs:service:DesiredCount \
  --min-capacity 2 \
  --max-capacity 10
```

### Database Scaling

- **Read replicas:** Set `DB_READER_HOST` to point at an Aurora read replica. All read queries (outside transactions) automatically route to the reader pool.
- **Connection tuning:** Adjust `database.max_open_conns` per instance. With 4 instances at 25 connections each = 100 connections to the primary. RDS default max is ~80 for db.t3.micro — size accordingly.
- **Write scaling:** Single primary handles writes. If write throughput becomes a bottleneck, scale up the RDS instance class.

### Multi-Instance Considerations

| Component | Behavior | Notes |
|-----------|----------|-------|
| Rate limiter | Per-instance (in-memory) | AWS WAF provides global rate limiting |
| JWKS cache | Per-instance | Correct — each instance caches Keycloak public keys independently |
| Circuit breaker | Per-instance | Correct — independent failure detection |
| Idempotency keys | Shared (PostgreSQL) | Safe across instances |
| Audit log buffer | Per-instance | Flushed to PostgreSQL asynchronously |
