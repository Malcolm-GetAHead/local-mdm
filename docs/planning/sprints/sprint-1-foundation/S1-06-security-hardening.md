# S1-06: Security Hardening & Secrets Management

**Sprint**: 1 — Foundation
**Parallel**: ⚠️ Partial (can start early, integrates with other tasks)
**Effort**: 2-3 days

## Objective

Implement security hardening measures including secrets management, rate limiting, input validation, and SQL injection prevention.

## Tasks

### 1. Secrets Management
- Create `secrets/` directory for local development secrets (gitignored)
- Secrets loader that reads from `secrets/` directory
- Environment variable override support
- Files: `internal/config/secrets.go`

**Secrets to manage**:
- Database credentials (`secrets/db_password`)
- JWT signing key (`secrets/jwt_secret`)
- APNs certificate and key (`secrets/apns_cert.pem`, `secrets/apns_key.pem`)
- SCEP CA private key (`secrets/ca_key.pem`)
- Keycloak client secret (`secrets/keycloak_client_secret`)

**Future State**: 
- Production: AWS Secrets Manager or SSM Parameter Store
- Migration path: Same secret keys, different loader implementation
- Document in `docs/deployment/SECRETS.md`

### 2. Rate Limiting Middleware
- Per-tenant rate limiting (configurable limits per enterprise)
- Per-endpoint rate limiting (enrollment endpoints more restrictive)
- Per-IP rate limiting for unauthenticated endpoints
- Redis-backed rate limiter (optional, fallback to in-memory)
- Files: `internal/api/middleware/ratelimit.go`

**Limits**:
- Enrollment endpoints: 10 req/min per IP
- API endpoints: 100 req/min per user
- Admin endpoints: 1000 req/min per enterprise

### 3. Input Validation Framework
- Request validation middleware using struct tags
- Sanitization helpers (HTML, SQL, path traversal)
- Max request size enforcement (10MB default)
- Content-Type validation
- Files: `internal/api/middleware/validation.go`, `internal/validation/sanitize.go`

### 4. SQL Injection Prevention
- Audit all database queries for parameterization
- Enforce prepared statements in repository layer
- Add linter rule to catch string concatenation in SQL
- Files: `.golangci.yml` (linter config)

### 5. Additional Security Headers
- HSTS (Strict-Transport-Security)
- X-Content-Type-Options: nosniff
- X-Frame-Options: DENY
- Content-Security-Policy
- Files: `internal/api/middleware/security_headers.go`

## Secrets Directory Structure

```
secrets/
├── .gitignore              # Ignore all files in this directory
├── README.md               # Instructions for local setup
├── db_password             # PostgreSQL password
├── jwt_secret              # JWT signing key (256-bit)
├── keycloak_client_secret  # Keycloak OIDC client secret
├── apns_cert.pem           # APNs certificate (macOS push)
├── apns_key.pem            # APNs private key
└── ca_key.pem              # Root CA private key
```

## Configuration Changes

Update `configs/config.yaml`:

```yaml
security:
  rate_limiting:
    enabled: true
    enrollment_rpm: 10
    api_rpm: 100
    admin_rpm: 1000
  max_request_size: 10485760  # 10MB
  
secrets:
  source: "file"  # file, env, aws_secrets_manager, aws_ssm
  file_path: "./secrets"
```

## Acceptance Criteria

- [ ] Secrets loaded from `secrets/` directory in development
- [ ] Rate limiting blocks excessive requests (test with curl loop)
- [ ] Input validation rejects malformed requests
- [ ] All SQL queries use parameterized statements (audit complete)
- [ ] Security headers present in all responses
- [ ] Documentation created for production secrets migration

## Future Migration Path

**Development** (current):
```go
secrets, _ := config.LoadSecretsFromFile("./secrets")
```

**Production** (future):
```go
secrets, _ := config.LoadSecretsFromAWS(region, secretPrefix)
```

Document in `docs/deployment/SECRETS.md`:
- How to migrate from file-based to AWS Secrets Manager
- Required IAM permissions
- Secret naming conventions in AWS
- Rotation procedures
