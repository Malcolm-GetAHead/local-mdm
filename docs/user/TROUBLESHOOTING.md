# Troubleshooting

## Enrollment Failures

### macOS

**Symptom:** Device shows "Profile Failed to Install" during enrollment.

- **SCEP challenge expired:** Challenges have a configurable TTL (`certificates.scep_challenge_ttl`, default 10m). If enrollment takes too long, the challenge expires. Check logs for `SCEP challenge not found or expired`.
- **APNs certificate mismatch:** The push topic in the enrollment profile must match the APNs certificate. Verify `macos.push_topic` in config matches the certificate's UID field.
- **NanoMDM unreachable:** Local MDM forwards `/checkin` and `/mdm` to NanoMDM. Verify `macos.nanomdm_url` is reachable from the server.

```bash
# Test NanoMDM connectivity
curl -s http://localhost:9000/version

# Check SCEP endpoint
curl -s "http://localhost:8080/scep?operation=GetCACert" | openssl x509 -inform DER -noout -subject
```

### Windows

**Symptom:** "Something went wrong" during MDM enrollment in Settings.

- **Discovery URL mismatch:** Windows enrollment starts with a discovery request. Verify `windows.discovery_url` matches the URL the device is configured to reach.
- **Certificate signing failure:** Check that `certificates.ca_cert_path` and `certificates.ca_key_path` are readable and the CA cert is valid.
- **TLS required:** Windows MDM enrollment requires HTTPS in production. In dev, ensure the device trusts the self-signed cert or use HTTP with `environment: development`.

```bash
# Test discovery endpoint
curl -s https://mdm.example.com/windows/discovery

# Verify CA cert validity
openssl x509 -in certs/ca.crt -noout -dates
```

### Android

**Symptom:** Enterprise enrollment fails or device shows "managed by" but no policies apply.

- **Service account JSON missing:** Verify `android.service_account_json` points to a valid Google service account key file.
- **Project ID mismatch:** The `android.project_id` must match the Google Cloud project linked to Android Management API.
- **Webhook URL unreachable:** Google sends status callbacks to `android.webhook_url`. This must be publicly reachable with valid TLS.

---

## Certificate Issues

### "certificate has expired" in logs

```bash
# Check CA cert expiry
openssl x509 -in certs/ca.crt -noout -dates

# Check device cert expiry (if you have it)
openssl x509 -in device.crt -noout -dates

# Monitor via Prometheus
curl -s http://127.0.0.1:9090/metrics | grep certificates_expiring_soon
```

The `certificates.expiration_monitor` runs every `check_interval` (default 24h) and sets the `certificates_expiring_soon` metric for certs within `warning_threshold` (default 30 days).

### "x509: certificate signed by unknown authority"

The client doesn't trust the CA that signed the server or device certificate.

- **Dev:** Import `certs/ca.crt` into the device/browser trust store.
- **Production:** Use a publicly trusted CA (ACM on ALB) for the server. Device certs signed by the internal CA are validated server-side only.

### SCEP certificate request fails

```bash
# Verify SCEP endpoint returns CA cert
curl -s "http://localhost:8080/scep?operation=GetCACert" -o /dev/null -w "%{http_code}"
# Should return 200

# Check CA key permissions
ls -la certs/ca.key
# Should be readable by the server process
```

---

## Database Connectivity

### "connection refused" on startup

```bash
# Check PostgreSQL is running
docker ps | grep postgres
# or
pg_isready -h localhost -p 5432

# Check connection string
psql "postgres://postgres:postgres@localhost:5432/localmdm?sslmode=disable" -c "SELECT 1"
```

### "too many connections"

The default `max_open_conns` is 25. With multiple instances, total connections = instances × max_open_conns.

```bash
# Check current connections
psql -c "SELECT count(*) FROM pg_stat_activity WHERE datname = 'localmdm'"

# Check max connections
psql -c "SHOW max_connections"
```

**Fix:** Reduce `database.max_open_conns` per instance, or increase PostgreSQL `max_connections`. For RDS, this is tied to instance class.

### Migration failures

```bash
# Check current migration version
migrate -path ./migrations -database "postgres://postgres:postgres@localhost:5432/localmdm?sslmode=disable" version

# Force to a specific version after a failed migration
make migrate-force VERSION=7

# Re-run migrations
make migrate-up
```

### "NULL column scan" errors

Some columns are nullable in the schema but scanned into non-pointer Go types. Known issue with `command.error_message` (nullable TEXT → `string`). Check logs for the specific column and table.

---

## Keycloak Authentication Issues

### "OIDC discovery failed" on startup

The server validates Keycloak connectivity at startup and will fail if Keycloak is unreachable.

```bash
# Check Keycloak is running
curl -s http://localhost:8180/health/ready

# Check OIDC discovery endpoint
curl -s http://localhost:8180/realms/localmdm/.well-known/openid-configuration | jq .issuer

# Verify config
grep -A4 "keycloak:" configs/config.yaml
```

### "token validation failed" / 401 on API requests

- **Expired token:** Access tokens default to 1h. Use the refresh endpoint: `POST /api/v1/auth/refresh`.
- **Wrong realm:** Verify `keycloak.realm` is `localmdm` (or your custom realm name).
- **Client secret mismatch:** Ensure `KEYCLOAK_CLIENT_SECRET` env var matches the client secret in Keycloak admin console → Clients → localmdm-api → Credentials.

```bash
# Get a fresh token
curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "admin"}' | jq .

# Decode a JWT to check claims (paste token at jwt.io or use)
echo "$TOKEN" | cut -d. -f2 | base64 -d 2>/dev/null | jq .
```

### "circuit breaker open" in logs

The auth middleware has a circuit breaker (default: 5 failures, 30s timeout). If Keycloak is down, the circuit opens and requests fail fast.

- Check Keycloak health: `curl http://localhost:8180/health/ready`
- The circuit auto-recovers after the timeout period
- Health endpoint reports Keycloak as `degraded` (not `unhealthy`) — the server stays up

---

## SCEP Challenge Failures

### "challenge not found or expired"

SCEP challenges are single-use and time-limited (default TTL from `certificates.scep_challenge_ttl`).

```bash
# Check challenge count in database
psql -d localmdm -c "SELECT count(*), min(expires_at), max(expires_at) FROM scep_challenges"

# Check for expired challenges
psql -d localmdm -c "SELECT count(*) FROM scep_challenges WHERE expires_at < NOW()"
```

**Common causes:**
- Device took too long to complete enrollment (increase `scep_challenge_ttl`)
- Challenge was already used (they are single-use)
- Clock skew between server and database

### "SCEP CA cert not found"

The SCEP handler serves the CA certificate from `certificates.ca_cert_path`.

```bash
# Verify file exists and is readable
ls -la certs/ca.crt

# Verify it's a valid certificate
openssl x509 -in certs/ca.crt -noout -text | head -20
```

---

## General Debugging

### Enable debug logging

```yaml
logging:
  level: "debug"
  format: "json"
```

Or set at runtime if using environment override (not currently supported — requires config file change and restart).

### Check server status

```bash
# Liveness
curl -s http://localhost:8080/health | jq .

# Readiness with latency
curl -s http://localhost:8080/health/ready | jq .

# Version
curl -s http://localhost:8080/version | jq .

# Metrics
curl -s http://127.0.0.1:9090/metrics | head -20
```

### Common HTTP status codes

| Code | Meaning | Common Cause |
|------|---------|--------------|
| 401  | Unauthorized | Missing or expired JWT token |
| 403  | Forbidden | Valid token but insufficient permissions |
| 409  | Conflict | Duplicate Idempotency-Key (request already processed) |
| 429  | Too Many Requests | Rate limit exceeded |
| 503  | Service Unavailable | Database unreachable |
