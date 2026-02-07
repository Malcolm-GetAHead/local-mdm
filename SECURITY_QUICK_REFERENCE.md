# Security Quick Reference Card

**Last Updated**: 2026-02-07

---

## 🚨 NEVER DO THIS

```go
// ❌ NEVER: Panic in HTTP handlers
func handler(w http.ResponseWriter, r *http.Request) {
    user := auth.MustUserFromContext(r.Context()) // CRASHES SERVER
}

// ❌ NEVER: Hardcode secrets
const jwtSecret = "my-secret-key"

// ❌ NEVER: Use http.DefaultClient
resp, err := http.DefaultClient.Get(url) // No timeout!

// ❌ NEVER: Allow nil auth middleware
if s.authMiddleware != nil {
    // Auth is optional - SECURITY HOLE
}

// ❌ NEVER: Dynamic SQL without whitelist
query := fmt.Sprintf("SELECT * FROM devices ORDER BY %s", userInput)

// ❌ NEVER: Expose internal errors
http.Error(w, err.Error(), 500) // Leaks SQL errors, file paths
```

---

## ✅ ALWAYS DO THIS

```go
// ✅ ALWAYS: Proper error handling
user, err := auth.UserFromContext(r.Context())
if err != nil {
    respondError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
    return
}

// ✅ ALWAYS: Load secrets from environment
jwtSecret := os.Getenv("JWT_SECRET")
if jwtSecret == "" {
    log.Fatal("JWT_SECRET environment variable required")
}

// ✅ ALWAYS: Use client with timeout
client := &http.Client{Timeout: 10 * time.Second}
resp, err := client.Get(url)

// ✅ ALWAYS: Require auth middleware
if s.authMiddleware == nil {
    return fmt.Errorf("CRITICAL: auth middleware required")
}

// ✅ ALWAYS: Whitelist SQL columns
safeColumn, ok := ValidateOrderColumn(userInput, whitelist)
if !ok {
    return fmt.Errorf("invalid column")
}

// ✅ ALWAYS: Sanitize error messages
if err != nil {
    log.Error("Database error", "error", err)
    respondError(w, r, 500, "internal_error", "An error occurred")
}
```

---

## 🔒 Security Checklist for New Code

### Before Committing

- [ ] No hardcoded secrets (passwords, keys, tokens)
- [ ] No panics in HTTP handlers
- [ ] All errors properly handled (no silent failures)
- [ ] Input validation on all user inputs
- [ ] SQL queries use parameterized statements
- [ ] HTTP clients have timeouts
- [ ] Sensitive data not logged
- [ ] Tests include error paths
- [ ] Tests pass with `-race` flag

### Before Deploying

- [ ] All secrets loaded from environment
- [ ] TLS enabled in production
- [ ] Rate limiting enabled
- [ ] Audit logging implemented
- [ ] Health checks working
- [ ] Metrics endpoint exposed
- [ ] Load tests passed
- [ ] Security scan passed

---

## 🛡️ Common Vulnerabilities

### SQL Injection
```go
// ❌ VULNERABLE
query := fmt.Sprintf("SELECT * FROM users WHERE id = %s", userID)

// ✅ SAFE
query := "SELECT * FROM users WHERE id = $1"
row := db.QueryRowContext(ctx, query, userID)
```

### XSS (Cross-Site Scripting)
```go
// ❌ VULNERABLE
fmt.Fprintf(w, "<div>%s</div>", userInput)

// ✅ SAFE
import "html/template"
tmpl.Execute(w, userInput) // Auto-escapes
```

### SSRF (Server-Side Request Forgery)
```go
// ❌ VULNERABLE
resp, _ := http.Get(userProvidedURL)

// ✅ SAFE
if err := validateURL(userProvidedURL); err != nil {
    return err
}
client := &http.Client{Timeout: 10 * time.Second}
resp, _ := client.Get(userProvidedURL)
```

### Authentication Bypass
```go
// ❌ VULNERABLE
if authMiddleware != nil {
    authMiddleware.RequireAuth(handler)
}

// ✅ SAFE
if authMiddleware == nil {
    panic("auth middleware required")
}
authMiddleware.RequireAuth(handler)
```

### Information Disclosure
```go
// ❌ VULNERABLE
log.Info("User login", "password", password)

// ✅ SAFE
log.Info("User login", "user_id", userID)
```

---

## 📊 Logging Best Practices

### What to Log
- ✅ Authentication attempts (success/failure)
- ✅ Authorization failures
- ✅ Data access (who accessed what)
- ✅ Configuration changes
- ✅ Errors with context
- ✅ Performance metrics

### What NOT to Log
- ❌ Passwords
- ❌ JWT tokens (full)
- ❌ API keys
- ❌ Credit card numbers
- ❌ Personal data (PII)
- ❌ SQL queries with sensitive data

### Example
```go
// ❌ BAD
log.Info("Login attempt", "username", username, "password", password)

// ✅ GOOD
log.Info("Login attempt", 
    "username", username,
    "ip", r.RemoteAddr,
    "user_agent", r.UserAgent(),
    "success", true,
)
```

---

## 🔐 Secrets Management

### Development
```bash
# .env file (gitignored)
export DB_PASSWORD="dev-password"
export JWT_SECRET="dev-secret-at-least-32-chars"
export KEYCLOAK_CLIENT_SECRET="dev-client-secret"
```

### Production
```bash
# AWS Secrets Manager
aws secretsmanager get-secret-value --secret-id prod/mdm/db-password

# Kubernetes Secrets
kubectl create secret generic mdm-secrets \
  --from-literal=db-password='...' \
  --from-literal=jwt-secret='...'
```

### Code
```go
// Load from environment
dbPassword := os.Getenv("DB_PASSWORD")
if dbPassword == "" {
    log.Fatal("DB_PASSWORD environment variable required")
}

// Validate strength
if len(dbPassword) < 16 {
    log.Fatal("DB_PASSWORD must be at least 16 characters")
}
```

---

## 🧪 Testing Security

### Unit Tests
```go
func TestAuthenticationRequired(t *testing.T) {
    req := httptest.NewRequest("GET", "/api/v1/devices", nil)
    // No Authorization header
    
    rr := httptest.NewRecorder()
    handler.ServeHTTP(rr, req)
    
    if rr.Code != http.StatusUnauthorized {
        t.Errorf("Expected 401, got %d", rr.Code)
    }
}
```

### Integration Tests
```go
func TestCrossTenantAccessBlocked(t *testing.T) {
    // Create two enterprises
    enterprise1 := createTestEnterprise(t)
    enterprise2 := createTestEnterprise(t)
    
    // Create device in enterprise1
    device := createTestDevice(t, enterprise1.ID)
    
    // Try to access from enterprise2 user
    token := getTokenForEnterprise(t, enterprise2.ID)
    
    req := httptest.NewRequest("GET", "/api/v1/devices/"+device.ID.String(), nil)
    req.Header.Set("Authorization", "Bearer "+token)
    
    rr := httptest.NewRecorder()
    handler.ServeHTTP(rr, req)
    
    // Should be forbidden
    if rr.Code != http.StatusForbidden {
        t.Errorf("Cross-tenant access should be blocked")
    }
}
```

### Load Tests
```bash
# Using k6
k6 run --vus 1000 --duration 10m load-test.js

# Check for:
# - No memory leaks
# - Stable latency
# - No errors
# - Rate limiting works
```

---

## 🚀 Deployment Checklist

### Pre-Deployment
```bash
# 1. Run all tests
go test -race -cover ./...

# 2. Security scan
gosec ./...
trivy fs .

# 3. Validate configuration
./server --validate-config

# 4. Check secrets
grep -r "password\|secret\|key" configs/ # Should find none

# 5. Load test
k6 run load-test.js
```

### Post-Deployment
```bash
# 1. Health check
curl https://api.example.com/health

# 2. Verify TLS
curl -v https://api.example.com 2>&1 | grep "TLS"

# 3. Test authentication
curl -H "Authorization: Bearer invalid" https://api.example.com/api/v1/devices
# Should return 401

# 4. Check metrics
curl https://api.example.com/metrics

# 5. Verify audit logs
psql -c "SELECT COUNT(*) FROM audit_logs WHERE created_at > NOW() - INTERVAL '1 hour'"
```

---

## 📞 Incident Response

### Security Incident Detected

1. **Isolate**: Take affected systems offline
2. **Assess**: Determine scope of breach
3. **Contain**: Block attacker access
4. **Eradicate**: Remove attacker presence
5. **Recover**: Restore from clean backups
6. **Review**: Analyze audit logs
7. **Report**: Notify affected parties
8. **Improve**: Fix vulnerabilities

### Audit Log Queries
```sql
-- Failed login attempts
SELECT * FROM audit_logs 
WHERE action = 'auth.login.failure' 
AND created_at > NOW() - INTERVAL '1 hour'
ORDER BY created_at DESC;

-- Unauthorized access attempts
SELECT * FROM audit_logs 
WHERE action = 'authz.access.denied'
AND created_at > NOW() - INTERVAL '1 hour'
ORDER BY created_at DESC;

-- Data modifications by user
SELECT * FROM audit_logs 
WHERE user_id = '...'
AND action LIKE '%.update' OR action LIKE '%.delete'
ORDER BY created_at DESC;
```

---

## 📚 Resources

- **OWASP Top 10**: https://owasp.org/www-project-top-ten/
- **Go Security**: https://go.dev/doc/security/
- **CWE Top 25**: https://cwe.mitre.org/top25/
- **NIST Guidelines**: https://csrc.nist.gov/publications/

---

**Remember**: Security is not a feature, it's a requirement. When in doubt, ask for a security review.
