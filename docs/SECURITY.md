# Security Hardening

## Implemented Security Measures

### 1. Security Headers ✅

All HTTP responses include security headers:

- **X-Content-Type-Options: nosniff** - Prevents MIME type sniffing
- **X-Frame-Options: DENY** - Prevents clickjacking
- **X-XSS-Protection: 1; mode=block** - XSS protection
- **Content-Security-Policy: default-src 'self'** - Restricts resource loading
- **Referrer-Policy: strict-origin-when-cross-origin** - Controls referrer information
- **Strict-Transport-Security** - HSTS (when TLS enabled)

### 2. Input Validation ✅

Validation helpers in `internal/validation/sanitize.go`:

```go
// HTML sanitization
clean := validation.SanitizeHTML(userInput)

// Path traversal prevention
safePath := validation.SanitizePath(filePath)

// Email validation
if !validation.ValidateEmail(email) {
    return errors.New("invalid email")
}

// UUID validation
if !validation.ValidateUUID(id) {
    return errors.New("invalid UUID")
}
```

### 3. SQL Injection Prevention ✅

**All database queries use parameterized statements:**

```go
// ✅ SAFE - Parameterized query
query := "SELECT * FROM devices WHERE id = $1"
db.QueryContext(ctx, query, deviceID)

// ❌ UNSAFE - String concatenation (never do this)
query := "SELECT * FROM devices WHERE id = '" + deviceID + "'"
```

**Repository layer enforces prepared statements:**
- All queries use `$1, $2, $3` placeholders
- No string concatenation in SQL
- Database driver handles escaping

### 4. Rate Limiting ✅

In-memory rate limiter (production uses AWS WAF on ALB; in-memory remains as fallback):

```go
// Create rate limiter: 100 requests per minute
limiter := newRateLimiter(100, 1*time.Minute)

// Apply to routes
router.Use(rateLimitMiddleware(limiter))
```

**Default Limits:**
- API endpoints: 100 req/min per IP
- Can be customized per route

### 5. Secrets Management ✅

**Development:**
- Secrets stored in `secrets/` directory (gitignored)
- Environment variables override file-based secrets
- Never commit secrets to version control

**Production:**
- Use AWS Secrets Manager or SSM Parameter Store
- Rotate secrets regularly
- Audit secret access

See `secrets/README.md` for setup instructions.

### 6. Authentication & Authorization ✅

- **OIDC token validation** via Keycloak
- **Role-based access control** (RBAC)
- **JWT signature verification** using JWKS
- **Token expiry validation**
- **Enterprise isolation** enforced

### 7. HTTPS/TLS ✅

**Configuration:**
```yaml
server:
  tls:
    enabled: true
    cert_file: "/path/to/cert.pem"
    key_file: "/path/to/key.pem"
```

**Production Requirements:**
- Use valid TLS certificates (Let's Encrypt, AWS ACM)
- TLS 1.2+ only
- Strong cipher suites
- HSTS enabled

### 8. Request Size Limits ✅

**HTTP server configuration:**
```go
server := &http.Server{
    ReadTimeout:  30 * time.Second,
    WriteTimeout: 30 * time.Second,
    IdleTimeout:  120 * time.Second,
    MaxHeaderBytes: 1 << 20, // 1 MB
}
```

### 9. Panic Recovery ✅

Recovery middleware catches panics:
- Logs error with request ID
- Returns 500 Internal Server Error
- Prevents server crash

### 10. Request ID Tracking ✅

Every request gets unique UUID:
- Propagated in context
- Included in logs
- Returned in response headers
- Enables request tracing

## Security Checklist

### Development
- [x] Security headers enabled
- [x] Input validation helpers
- [x] SQL injection prevention (parameterized queries)
- [x] Rate limiting implemented
- [x] Secrets management setup
- [x] OIDC authentication
- [x] RBAC authorization
- [x] Panic recovery
- [x] Request ID tracking
- [x] API token authentication (lmdm_ prefix, SHA-256 hashed)
- [x] SCEP challenge-based enrollment
- [x] Idempotency-Key deduplication (PostgreSQL-backed)

### Production (TODO)
- [ ] TLS certificates configured
- [ ] AWS Secrets Manager integration
- [ ] AWS WAF rate-based rules on ALB (primary rate limiting; in-memory remains as fallback)
- [ ] DDoS protection (CloudFlare, AWS Shield)
- [ ] Security scanning (Snyk, Dependabot)
- [ ] Penetration testing
- [ ] Security audit
- [ ] Incident response plan

## Common Vulnerabilities Addressed

### OWASP Top 10

1. **Broken Access Control** ✅
   - RBAC with role checking
   - Enterprise isolation
   - Token validation

2. **Cryptographic Failures** ✅
   - TLS support
   - Secure password hashing (Keycloak)
   - JWT signature verification

3. **Injection** ✅
   - Parameterized SQL queries
   - Input sanitization
   - No eval() or exec()

4. **Insecure Design** ✅
   - Security by design
   - Principle of least privilege
   - Defense in depth

5. **Security Misconfiguration** ✅
   - Security headers
   - Secure defaults
   - Configuration validation

6. **Vulnerable Components** ✅
   - Dependency scanning (TODO)
   - Regular updates
   - Minimal dependencies

7. **Authentication Failures** ✅
   - OIDC with Keycloak
   - Token expiry
   - No password storage

8. **Software and Data Integrity** ✅
   - Code signing (TODO)
   - Audit logging
   - Immutable infrastructure

9. **Logging Failures** ✅
   - Structured logging
   - Request ID tracking
   - Error logging

10. **SSRF** ✅
    - Input validation
    - URL sanitization
    - Network segmentation (TODO)

### 11. API Token Authentication ✅

Programmatic API access via long-lived tokens (Sprint 5):

- **Token format**: `lmdm_` prefix followed by 32 random bytes (base64url-encoded)
- **Storage**: Only the SHA-256 hash is stored in the `api_tokens` table — the plaintext token is returned once at creation and never stored
- **Authentication**: `Authorization: Bearer lmdm_...` header
- **Dual auth support**: Endpoints accept either OIDC JWT or API token — middleware tries both and uses the first that succeeds
- **Token revocation**: `DELETE /api/v1/tokens/{id}` immediately invalidates the token
- **Scoping**: Tokens are scoped to an enterprise and carry a role (same RBAC as OIDC users)

### 12. SCEP Server Security ✅

Simple Certificate Enrollment Protocol server for device certificate issuance (Sprint 5):

- **Challenge-based enrollment**: Devices must present a valid one-time-use challenge to obtain a certificate
- **One-time-use challenges**: Each challenge can only be used once — replay attacks are rejected
- **PostgreSQL-backed**: Challenges stored in `scep_challenges` table, safe across multiple server instances (no in-memory state)
- **Hourly cleanup**: Expired challenges are purged every hour via background goroutine
- **Enterprise-scoped**: Challenges are tied to an enterprise, preventing cross-tenant enrollment

## Certificate Revocation

The server TLS certificate includes a CRL Distribution Point (`http://<server>:8080/crl/ca.crl`). This is required for Windows schannel TLS validation. The CRL is currently static (generated once, empty). Future work:
- Regenerate CRL on certificate revocation
- Automate CRL refresh on a schedule
- Consider OCSP as an alternative for real-time revocation checking

## Enrollment Security (Future)

Currently, any device that knows the MDM server hostname and email format can enroll. Planned improvements (tracked in F-07):
- Enrollment tokens with expiry and use limits
- Per-enterprise enrollment codes
- Enrollment audit trail

See `docs/planning/future/F-07-advanced-features.md` for the enrollment token system design.

### 13. Idempotency ✅

Request deduplication for safe retries on all mutating endpoints (Sprint 4):

- **Idempotency-Key header**: Required on all `POST`, `PUT`, `PATCH` requests
- **PostgreSQL-backed**: Keys and cached responses stored in `idempotency_keys` table — safe across multiple server instances
- **24-hour TTL**: Keys expire after 24 hours; hourly cleanup removes expired entries
- **Replay protection**: Duplicate requests within the TTL window return the original response (status code + body) without re-executing the operation

### 14. Production Rate Limiting ✅

Two-layer rate limiting strategy:

- **Primary — AWS WAF**: Rate-based rules attached to the ALB. Handles volumetric attacks before traffic reaches the application. Configurable per-IP thresholds.
- **Fallback — In-memory per-instance**: The Go application's built-in rate limiter (100 req/min per IP) remains active as defense-in-depth. In a multi-instance deployment behind a load balancer, each instance tracks rates independently (imprecise but acceptable as a secondary layer).

## Reporting Security Issues

**DO NOT** open public GitHub issues for security vulnerabilities.

Instead, email: security@localmdm.dev (TODO: setup)

Include:
- Description of vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if any)

## Security Updates

Check for security updates regularly:

```bash
# Check for vulnerable dependencies
go list -json -m all | nancy sleuth

# Update dependencies
go get -u ./...
go mod tidy
```

## Additional Resources

- [OWASP Top 10](https://owasp.org/www-project-top-ten/)
- [Go Security Best Practices](https://golang.org/doc/security/)
- [NIST Cybersecurity Framework](https://www.nist.gov/cyberframework)
