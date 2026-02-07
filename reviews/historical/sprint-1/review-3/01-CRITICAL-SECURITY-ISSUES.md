# Critical Security Issues

**Priority**: 🔴 **CRITICAL**  
**Must Fix**: Before any production deployment or Sprint 2

---

## CRITICAL-01: Authentication Bypass via JWKS Refresh Race Condition

### Location
`internal/auth/oidc.go:115-125`

### Issue
The JWKS refresh mechanism has a race condition that can be exploited:

```go
// Current vulnerable code
if needsRefresh {
    go v.refreshJWKS()  // Non-blocking refresh
}

// Token validation continues with potentially stale JWKS
```

### Attack Vector
1. Attacker waits for JWKS to become stale (after 1 hour)
2. Attacker sends malicious token during refresh window
3. Token is validated against old JWKS before new keys are loaded
4. If old keys are compromised, attacker gains access

### Impact
- **Severity**: CRITICAL
- **Exploitability**: Medium (requires timing)
- **Impact**: Complete authentication bypass

### Fix
```go
// Validate token with fresh JWKS
func (v *OIDCValidator) ValidateToken(tokenString string) (*AuthUser, error) {
    // Check if refresh is needed
    v.jwksMutex.RLock()
    needsRefresh := time.Since(v.lastRefresh) > v.refreshEvery
    v.jwksMutex.RUnlock()
    
    // BLOCKING refresh if needed
    if needsRefresh {
        if err := v.refreshJWKS(); err != nil {
            // Log error but continue with cached JWKS
            // Only fail if JWKS is completely missing
            v.jwksMutex.RLock()
            hasKeys := v.jwks != nil && len(v.jwks.Keys) > 0
            v.jwksMutex.RUnlock()
            
            if !hasKeys {
                return nil, fmt.Errorf("JWKS unavailable: %w", err)
            }
        }
    }
    
    // Continue with token validation...
}
```

### Test Case
```go
func TestJWKSRefreshBlocking(t *testing.T) {
    // Test that token validation blocks during JWKS refresh
    // Test that stale tokens are rejected after refresh
}
```

---

## CRITICAL-02: SQL Injection via ORDER BY Column

### Location
`internal/repository/sql_safety.go:36-41`

### Issue
The `ValidateOrderColumn` function uses a whitelist but doesn't prevent all injection vectors:

```go
func ValidateOrderColumn(column string, allowed []string) (string, error) {
    for _, a := range allowed {
        if column == a {
            return column, nil
        }
    }
    return DefaultOrderColumn(), fmt.Errorf("invalid order column: %s", column)
}
```

**Problem**: This is called but the result is directly interpolated into SQL:

```go
// In repository code (not shown but implied)
query := fmt.Sprintf("SELECT * FROM devices ORDER BY %s", orderColumn)
```

### Attack Vector
If any code path allows user-controlled `ORDER BY` without using this validator, SQL injection is possible.

### Impact
- **Severity**: CRITICAL
- **Exploitability**: High (if exposed via API)
- **Impact**: Database compromise, data exfiltration

### Fix
1. **Never use string interpolation for ORDER BY**
2. **Use a map-based approach**:

```go
// Safe ORDER BY implementation
type OrderByConfig struct {
    Column    string
    Direction string // "ASC" or "DESC"
}

var allowedOrderColumns = map[string]string{
    "created_at": "created_at",
    "updated_at": "updated_at",
    "name":       "name",
    "status":     "status",
}

func SafeOrderBy(userColumn, userDirection string) (string, string, error) {
    // Validate column
    safeColumn, ok := allowedOrderColumns[userColumn]
    if !ok {
        return "created_at", "DESC", fmt.Errorf("invalid column")
    }
    
    // Validate direction
    safeDirection := "DESC"
    if strings.ToUpper(userDirection) == "ASC" {
        safeDirection = "ASC"
    }
    
    return safeColumn, safeDirection, nil
}

// Usage in repository
column, direction, _ := SafeOrderBy(userInput, userDirection)
query := fmt.Sprintf("SELECT * FROM devices ORDER BY %s %s", column, direction)
```

### Audit Required
Search entire codebase for:
```bash
grep -rn "ORDER BY.*%s" internal/
grep -rn "fmt.Sprintf.*ORDER" internal/
```

---

## CRITICAL-03: JSONB Injection via Deeply Nested Objects

### Location
`internal/validation/jsonb.go:14-38`

### Issue
The JSONB validation has a flaw in depth calculation:

```go
func calculateDepth(v interface{}) int {
    switch val := v.(type) {
    case map[string]interface{}:
        if len(val) == 0 {
            return 0  // Empty map returns 0, not 1!
        }
        // ...
    }
}
```

### Attack Vector
Attacker can create deeply nested structures that bypass depth check:

```json
{
  "a": {},
  "b": {"c": {}},
  "d": {"e": {"f": {}}}
}
```

Each empty object returns depth 0, so total depth appears shallow.

### Impact
- **Severity**: CRITICAL
- **Exploitability**: High
- **Impact**: Database storage exhaustion, query performance degradation

### Fix
```go
func calculateDepth(v interface{}) int {
    switch val := v.(type) {
    case map[string]interface{}:
        // Empty map still has depth 1
        maxDepth := 0
        for _, item := range val {
            if d := calculateDepth(item); d > maxDepth {
                maxDepth = d
            }
        }
        return maxDepth + 1  // Always add 1 for this level
        
    case []interface{}:
        // Empty array still has depth 1
        maxDepth := 0
        for _, item := range val {
            if d := calculateDepth(item); d > maxDepth {
                maxDepth = d
            }
        }
        return maxDepth + 1  // Always add 1 for this level
        
    default:
        return 1  // Primitives have depth 1, not 0
    }
}
```

### Test Case
```go
func TestJSONBDepthWithEmptyObjects(t *testing.T) {
    // Test that empty nested objects count toward depth
    deeply := map[string]interface{}{
        "a": map[string]interface{}{
            "b": map[string]interface{}{
                "c": map[string]interface{}{},
            },
        },
    }
    
    err := ValidateJSONB(deeply, 3)
    assert.Error(t, err, "Should reject depth > 3")
}
```

---

## CRITICAL-04: Certificate Private Key Insecure Storage

### Location
`internal/certs/ca.go:138-149`

### Issue
Private keys are stored with insufficient protection:

```go
keyFile, err := os.OpenFile(m.keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
```

**Problems**:
1. File permissions 0600 are insufficient (should be 0400)
2. No encryption at rest
3. No HSM/KMS integration
4. Keys stored in plain text on disk
5. No key rotation mechanism

### Impact
- **Severity**: CRITICAL
- **Exploitability**: Medium (requires file system access)
- **Impact**: Complete PKI compromise, all device certificates compromised

### Fix

#### Short-term (Immediate)
```go
// 1. Use 0400 permissions (read-only)
keyFile, err := os.OpenFile(m.keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0400)

// 2. Encrypt private key with passphrase
func (m *CAManager) saveEncryptedKey(key *rsa.PrivateKey, passphrase string) error {
    keyBytes := x509.MarshalPKCS1PrivateKey(key)
    
    // Encrypt with AES-256
    block, err := aes.NewCipher([]byte(passphrase))
    if err != nil {
        return err
    }
    
    // Use PKCS#8 encrypted format
    encrypted, err := x509.EncryptPEMBlock(
        rand.Reader,
        "RSA PRIVATE KEY",
        keyBytes,
        []byte(passphrase),
        x509.PEMCipherAES256,
    )
    if err != nil {
        return err
    }
    
    return pem.Encode(keyFile, encrypted)
}
```

#### Long-term (Production)
```go
// Use AWS KMS or HashiCorp Vault
type KMSKeyManager struct {
    kmsClient *kms.Client
    keyID     string
}

func (k *KMSKeyManager) SignCSR(csr *x509.CertificateRequest) (*x509.Certificate, error) {
    // Sign using KMS without exposing private key
    // ...
}
```

---

## CRITICAL-05: Missing Input Validation on API Handlers

### Location
`internal/api/handlers.go` (all handlers)

### Issue
API handlers have NO input validation:

```go
func (s *Server) handleCreatePolicy(w http.ResponseWriter, r *http.Request) {
    respondNotImplemented(w, r)  // Stub - no validation!
}
```

Even implemented handlers lack validation:

```go
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
    var req auth.LoginRequest
    if err := parseJSONBody(r, &req); err != nil {
        // Only checks JSON parsing, not content!
        respondError(w, r, http.StatusBadRequest, "invalid_request", err.Error())
        return
    }
    // Missing: length checks, character validation, rate limiting per user
}
```

### Impact
- **Severity**: CRITICAL
- **Exploitability**: High
- **Impact**: Various injection attacks, DoS, data corruption

### Fix

Create comprehensive input validation:

```go
// internal/api/validation.go
package api

import (
    "fmt"
    "github.com/malcolm-getahead/local-mdm/internal/validation"
)

type CreatePolicyRequest struct {
    Name         string                 `json:"name"`
    Description  string                 `json:"description"`
    Platform     string                 `json:"platform"`
    PolicyType   string                 `json:"policy_type"`
    PolicyConfig map[string]interface{} `json:"policy_config"`
}

func (r *CreatePolicyRequest) Validate() error {
    v := validation.New()
    
    v.Required("name", r.Name)
    v.MinLength("name", r.Name, 3)
    v.MaxLength("name", r.Name, 255)
    
    v.Required("platform", r.Platform)
    v.OneOf("platform", r.Platform, []string{"windows", "macos", "android"})
    
    v.Required("policy_type", r.PolicyType)
    v.OneOf("policy_type", r.PolicyType, []string{
        "wifi", "vpn", "security", "app", "restriction", "compliance",
    })
    
    // Validate JSONB
    if err := validation.ValidateJSONB(r.PolicyConfig, validation.MaxJSONBDepth); err != nil {
        v.errors = append(v.errors, fmt.Sprintf("policy_config: %v", err))
    }
    
    if !v.Valid() {
        return fmt.Errorf("validation failed: %s", v.Error())
    }
    
    return nil
}

// Usage in handler
func (s *Server) handleCreatePolicy(w http.ResponseWriter, r *http.Request) {
    var req CreatePolicyRequest
    if err := parseJSONBody(r, &req); err != nil {
        respondError(w, r, http.StatusBadRequest, "invalid_json", err.Error())
        return
    }
    
    if err := req.Validate(); err != nil {
        respondError(w, r, http.StatusBadRequest, "validation_failed", err.Error())
        return
    }
    
    // Proceed with validated input...
}
```

---

## CRITICAL-06: Insecure Default Configuration

### Location
`configs/config.yaml`

### Issue
Default configuration has insecure settings:

```yaml
server:
  host: "0.0.0.0"  # Binds to all interfaces
  tls:
    enabled: false  # No TLS by default!
  rate_limit:
    enabled: false  # No rate limiting!

database:
  password: "postgres"  # Default password in config file!
  sslmode: "disable"    # No SSL to database!

keycloak:
  client_secret: "localmdm-api-secret"  # Hardcoded secret!
```

### Impact
- **Severity**: CRITICAL
- **Exploitability**: High (if deployed as-is)
- **Impact**: Complete system compromise

### Fix

1. **Remove secrets from config files**:

```yaml
# config.yaml - NO SECRETS
server:
  host: "${SERVER_HOST:-127.0.0.1}"  # Default to localhost
  port: "${SERVER_PORT:-8080}"
  tls:
    enabled: "${TLS_ENABLED:-true}"
    cert_file: "${TLS_CERT_FILE}"
    key_file: "${TLS_KEY_FILE}"
  rate_limit:
    enabled: "${RATE_LIMIT_ENABLED:-true}"
    requests_per_min: "${RATE_LIMIT_RPM:-100}"

database:
  host: "${DB_HOST:-localhost}"
  port: "${DB_PORT:-5432}"
  user: "${DB_USER:-localmdm}"
  password: "${DB_PASSWORD}"  # MUST be set via env
  database: "${DB_NAME:-localmdm}"
  sslmode: "${DB_SSLMODE:-require}"  # Require SSL by default

keycloak:
  url: "${KEYCLOAK_URL}"
  realm: "${KEYCLOAK_REALM:-localmdm}"
  client_id: "${KEYCLOAK_CLIENT_ID}"
  client_secret: "${KEYCLOAK_CLIENT_SECRET}"  # MUST be set via env
```

2. **Add configuration validation**:

```go
func (c *Config) Validate() error {
    // Require secrets to be set
    if c.Database.Password == "" {
        return fmt.Errorf("DB_PASSWORD must be set")
    }
    if c.Keycloak.ClientSecret == "" {
        return fmt.Errorf("KEYCLOAK_CLIENT_SECRET must be set")
    }
    
    // Enforce TLS in production
    if os.Getenv("ENV") == "production" && !c.Server.TLS.Enabled {
        return fmt.Errorf("TLS must be enabled in production")
    }
    
    // Enforce rate limiting
    if !c.Server.RateLimit.Enabled {
        return fmt.Errorf("rate limiting must be enabled")
    }
    
    return nil
}
```

3. **Create separate config files**:

```
configs/
  config.dev.yaml      # Development (insecure OK)
  config.staging.yaml  # Staging (secure)
  config.prod.yaml     # Production (secure)
  config.example.yaml  # Template (no secrets)
```

---

## Summary

All 6 critical security issues must be fixed before any production deployment or Sprint 2 work.

**Estimated effort**: 2-3 days for all fixes + comprehensive testing.

**Priority order**:
1. CRITICAL-06: Fix insecure defaults (1 hour)
2. CRITICAL-05: Add input validation (1 day)
3. CRITICAL-04: Secure private keys (4 hours)
4. CRITICAL-03: Fix JSONB depth calculation (2 hours)
5. CRITICAL-02: Audit and fix SQL injection (4 hours)
6. CRITICAL-01: Fix JWKS race condition (2 hours)
