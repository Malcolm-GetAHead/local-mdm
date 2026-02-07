# Sprint 1 Remediation Tasks

**Purpose**: Ordered list of tasks to fix all critical and high-priority issues  
**Format**: Similar to sprint-1-foundation task structure  
**Estimated Total Time**: 7-11 days

---

## Task Organization

Tasks are organized by priority and dependency:

- **Phase 1**: Critical Security (Days 1-3)
- **Phase 2**: Critical Reliability (Days 3-5)
- **Phase 3**: High Priority (Days 5-8)
- **Phase 4**: Medium Priority (Days 8-11)

---

## Phase 1: Critical Security Fixes (Days 1-3)

### TASK-01: Fix Insecure Default Configuration
**Priority**: 🔴 CRITICAL  
**Estimated Time**: 2 hours  
**Dependencies**: None

#### Acceptance Criteria
- [ ] Remove all secrets from config files
- [ ] Use environment variables for all sensitive data
- [ ] Add configuration validation
- [ ] Create separate dev/staging/prod configs
- [ ] Update documentation

#### Implementation Steps

1. **Update config.yaml**:
```yaml
# configs/config.yaml
server:
  host: "${SERVER_HOST:-127.0.0.1}"
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
  password: "${DB_PASSWORD}"
  database: "${DB_NAME:-localmdm}"
  sslmode: "${DB_SSLMODE:-require}"
  max_open_conns: "${DB_MAX_OPEN_CONNS:-25}"
  max_idle_conns: "${DB_MAX_IDLE_CONNS:-5}"
  conn_max_lifetime: "${DB_CONN_MAX_LIFETIME:-5m}"

keycloak:
  url: "${KEYCLOAK_URL}"
  realm: "${KEYCLOAK_REALM:-localmdm}"
  client_id: "${KEYCLOAK_CLIENT_ID}"
  client_secret: "${KEYCLOAK_CLIENT_SECRET}"
```

2. **Add validation**:
```go
// internal/config/config.go
func (c *Config) Validate() error {
    // Require secrets
    if c.Database.Password == "" {
        return fmt.Errorf("DB_PASSWORD must be set")
    }
    if c.Keycloak.ClientSecret == "" {
        return fmt.Errorf("KEYCLOAK_CLIENT_SECRET must be set")
    }
    
    // Enforce security in production
    env := os.Getenv("ENV")
    if env == "production" {
        if !c.Server.TLS.Enabled {
            return fmt.Errorf("TLS must be enabled in production")
        }
        if !c.Server.RateLimit.Enabled {
            return fmt.Errorf("rate limiting must be enabled in production")
        }
        if c.Database.SSLMode != "require" && c.Database.SSLMode != "verify-full" {
            return fmt.Errorf("database SSL must be enabled in production")
        }
    }
    
    return nil
}
```

3. **Create environment-specific configs**:
```bash
# configs/config.dev.yaml - Development
# configs/config.staging.yaml - Staging
# configs/config.prod.yaml - Production
# configs/config.example.yaml - Template
```

4. **Update .env.example**:
```bash
# .env.example
ENV=development

# Server
SERVER_HOST=127.0.0.1
SERVER_PORT=8080
TLS_ENABLED=false
TLS_CERT_FILE=
TLS_KEY_FILE=

# Database
DB_HOST=localhost
DB_PORT=5432
DB_USER=localmdm
DB_PASSWORD=changeme
DB_NAME=localmdm
DB_SSLMODE=disable

# Keycloak
KEYCLOAK_URL=http://localhost:8180
KEYCLOAK_REALM=localmdm
KEYCLOAK_CLIENT_ID=localmdm-api
KEYCLOAK_CLIENT_SECRET=changeme
```

#### Testing
```bash
# Test that server fails without required env vars
unset DB_PASSWORD
go run cmd/server/main.go
# Should fail with "DB_PASSWORD must be set"

# Test with valid config
export DB_PASSWORD=test
export KEYCLOAK_CLIENT_SECRET=test
go run cmd/server/main.go
# Should start successfully
```

---

### TASK-02: Add Comprehensive Input Validation
**Priority**: 🔴 CRITICAL  
**Estimated Time**: 1 day  
**Dependencies**: None

#### Acceptance Criteria
- [ ] Create validation structs for all API requests
- [ ] Implement validation methods
- [ ] Add validation to all handlers
- [ ] Add tests for validation
- [ ] Document validation rules

#### Implementation Steps

1. **Create validation package**:
```go
// internal/api/validation.go
package api

import (
    "fmt"
    "github.com/malcolm-getahead/local-mdm/internal/models"
    "github.com/malcolm-getahead/local-mdm/internal/validation"
)

// Request validation structs
type CreateEnterpriseRequest struct {
    Name     string                 `json:"name"`
    Slug     string                 `json:"slug"`
    Settings map[string]interface{} `json:"settings"`
}

func (r *CreateEnterpriseRequest) Validate() error {
    v := validation.New()
    
    v.Required("name", r.Name)
    v.MinLength("name", r.Name, 3)
    v.MaxLength("name", r.Name, 255)
    
    v.Required("slug", r.Slug)
    v.Pattern("slug", r.Slug, `^[a-z0-9-]+$`)
    v.MinLength("slug", r.Slug, 3)
    v.MaxLength("slug", r.Slug, 100)
    
    if r.Settings != nil {
        if err := validation.ValidateJSONB(r.Settings, validation.MaxJSONBDepth); err != nil {
            v.errors = append(v.errors, fmt.Sprintf("settings: %v", err))
        }
    }
    
    if !v.Valid() {
        return fmt.Errorf("validation failed: %s", v.Error())
    }
    
    return nil
}

type CreateDeviceRequest struct {
    Platform     string                 `json:"platform"`
    DeviceID     string                 `json:"device_id"`
    SerialNumber string                 `json:"serial_number"`
    Name         string                 `json:"name"`
    Model        string                 `json:"model"`
    OSVersion    string                 `json:"os_version"`
    PlatformData map[string]interface{} `json:"platform_data"`
}

func (r *CreateDeviceRequest) Validate() error {
    v := validation.New()
    
    v.Required("platform", r.Platform)
    v.OneOf("platform", r.Platform, []string{
        models.PlatformWindows,
        models.PlatformMacOS,
        models.PlatformAndroid,
    })
    
    v.Required("device_id", r.DeviceID)
    v.MaxLength("device_id", r.DeviceID, 255)
    
    v.Required("serial_number", r.SerialNumber)
    v.MaxLength("serial_number", r.SerialNumber, 255)
    
    v.MaxLength("name", r.Name, 255)
    v.MaxLength("model", r.Model, 255)
    v.MaxLength("os_version", r.OSVersion, 100)
    
    if r.PlatformData != nil {
        if err := validation.ValidateJSONB(r.PlatformData, validation.MaxJSONBDepth); err != nil {
            v.errors = append(v.errors, fmt.Sprintf("platform_data: %v", err))
        }
    }
    
    if !v.Valid() {
        return fmt.Errorf("validation failed: %s", v.Error())
    }
    
    return nil
}

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
    
    v.MaxLength("description", r.Description, 1000)
    
    v.Required("platform", r.Platform)
    v.OneOf("platform", r.Platform, []string{
        models.PlatformWindows,
        models.PlatformMacOS,
        models.PlatformAndroid,
    })
    
    v.Required("policy_type", r.PolicyType)
    v.OneOf("policy_type", r.PolicyType, []string{
        models.PolicyTypeWiFi,
        models.PolicyTypeVPN,
        models.PolicyTypeSecurity,
        models.PolicyTypeApp,
        models.PolicyTypeRestriction,
        models.PolicyTypeCompliance,
    })
    
    v.Required("policy_config", r.PolicyConfig)
    if err := validation.ValidateJSONB(r.PolicyConfig, validation.MaxJSONBDepth); err != nil {
        v.errors = append(v.errors, fmt.Sprintf("policy_config: %v", err))
    }
    
    if !v.Valid() {
        return fmt.Errorf("validation failed: %s", v.Error())
    }
    
    return nil
}
```

2. **Update handlers**:
```go
// internal/api/handlers.go
func (s *Server) handleCreateEnterprise(w http.ResponseWriter, r *http.Request) {
    var req CreateEnterpriseRequest
    if err := parseJSONBody(r, &req); err != nil {
        respondError(w, r, http.StatusBadRequest, "invalid_json", err.Error())
        return
    }
    
    if err := req.Validate(); err != nil {
        respondError(w, r, http.StatusBadRequest, "validation_failed", err.Error())
        return
    }
    
    // Get user from context
    user, err := auth.UserFromContext(r.Context())
    if err != nil {
        respondError(w, r, http.StatusUnauthorized, "unauthorized", "User not found in context")
        return
    }
    
    // Create enterprise
    enterprise := &models.Enterprise{
        Name:     req.Name,
        Slug:     req.Slug,
        Settings: req.Settings,
    }
    
    repo, err := repository.NewEnterpriseRepository(s.db)
    if err != nil {
        s.logger.Error("Failed to create repository", "error", err)
        respondError(w, r, http.StatusInternalServerError, "internal_error", "Failed to create repository")
        return
    }
    
    if err := repo.Create(r.Context(), enterprise); err != nil {
        s.logger.Error("Failed to create enterprise", "error", err)
        respondError(w, r, http.StatusInternalServerError, "create_failed", "Failed to create enterprise")
        return
    }
    
    // Audit log
    s.auditLogger.LogFromContext(r.Context(), "enterprise.create", "enterprise", &enterprise.ID, models.JSONB{
        "name": enterprise.Name,
        "slug": enterprise.Slug,
    })
    
    respondJSON(w, r, http.StatusCreated, enterprise)
}
```

3. **Add tests**:
```go
// internal/api/validation_test.go
func TestCreateEnterpriseRequestValidation(t *testing.T) {
    tests := []struct {
        name    string
        req     CreateEnterpriseRequest
        wantErr bool
    }{
        {
            name: "valid request",
            req: CreateEnterpriseRequest{
                Name: "Test Enterprise",
                Slug: "test-enterprise",
            },
            wantErr: false,
        },
        {
            name: "missing name",
            req: CreateEnterpriseRequest{
                Slug: "test-enterprise",
            },
            wantErr: true,
        },
        {
            name: "invalid slug",
            req: CreateEnterpriseRequest{
                Name: "Test Enterprise",
                Slug: "Test Enterprise!",  // Invalid characters
            },
            wantErr: true,
        },
        {
            name: "name too short",
            req: CreateEnterpriseRequest{
                Name: "AB",
                Slug: "ab",
            },
            wantErr: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.req.Validate()
            if (err != nil) != tt.wantErr {
                t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

#### Testing
```bash
go test ./internal/api/... -v -run TestValidation
```

---

### TASK-03: Secure Certificate Private Keys
**Priority**: 🔴 CRITICAL  
**Estimated Time**: 4 hours  
**Dependencies**: TASK-01 (config)

#### Acceptance Criteria
- [ ] Encrypt private keys at rest
- [ ] Use secure file permissions (0400)
- [ ] Add passphrase support
- [ ] Document key management
- [ ] Add key rotation mechanism

#### Implementation Steps

1. **Update CA manager**:
```go
// internal/certs/ca.go
func (m *CAManager) saveEncryptedKey(key *rsa.PrivateKey, passphrase []byte) error {
    keyBytes := x509.MarshalPKCS1PrivateKey(key)
    
    // Encrypt with AES-256
    encrypted, err := x509.EncryptPEMBlock(
        rand.Reader,
        "RSA PRIVATE KEY",
        keyBytes,
        passphrase,
        x509.PEMCipherAES256,
    )
    if err != nil {
        return fmt.Errorf("encrypt key: %w", err)
    }
    
    // Create file with restrictive permissions
    keyFile, err := os.OpenFile(m.keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0400)
    if err != nil {
        return fmt.Errorf("create key file: %w", err)
    }
    defer keyFile.Close()
    
    if err := pem.Encode(keyFile, encrypted); err != nil {
        return fmt.Errorf("encode key: %w", err)
    }
    
    return nil
}

func (m *CAManager) loadEncryptedKey(passphrase []byte) error {
    keyPEM, err := os.ReadFile(m.keyPath)
    if err != nil {
        return fmt.Errorf("read key file: %w", err)
    }
    
    block, _ := pem.Decode(keyPEM)
    if block == nil {
        return fmt.Errorf("failed to decode key PEM")
    }
    
    // Check if encrypted
    if x509.IsEncryptedPEMBlock(block) {
        decrypted, err := x509.DecryptPEMBlock(block, passphrase)
        if err != nil {
            return fmt.Errorf("decrypt key: %w", err)
        }
        
        key, err := x509.ParsePKCS1PrivateKey(decrypted)
        if err != nil {
            return fmt.Errorf("parse key: %w", err)
        }
        
        m.caKey = key
    } else {
        // Unencrypted (for backward compatibility)
        key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
        if err != nil {
            return fmt.Errorf("parse key: %w", err)
        }
        
        m.caKey = key
    }
    
    return nil
}
```

2. **Add passphrase to config**:
```yaml
certificates:
  ca_cert_path: "${CA_CERT_PATH:-./secrets/ca.crt}"
  ca_key_path: "${CA_KEY_PATH:-./secrets/ca.key}"
  ca_key_passphrase: "${CA_KEY_PASSPHRASE}"  # Required for encrypted keys
  key_size: "${CA_KEY_SIZE:-4096}"
  validity_years: "${CA_VALIDITY_YEARS:-10}"
```

3. **Update initialization**:
```go
// cmd/server/main.go or certs service initialization
passphrase := []byte(cfg.Certificates.CAKeyPassphrase)
if len(passphrase) == 0 {
    logger.Warn("CA key passphrase not set - using unencrypted key")
}

caManager, err := certs.NewCAManager(
    cfg.Certificates.CACertPath,
    cfg.Certificates.CAKeyPath,
    passphrase,
)
```

#### Testing
```bash
# Test encrypted key generation
go test ./internal/certs/... -v -run TestEncryptedKey

# Verify file permissions
ls -la secrets/ca.key
# Should show: -r-------- (0400)
```

---

### TASK-04: Fix JSONB Depth Calculation
**Priority**: 🔴 CRITICAL  
**Estimated Time**: 2 hours  
**Dependencies**: None

#### Implementation
See `01-CRITICAL-SECURITY-ISSUES.md` CRITICAL-03 for full implementation.

#### Testing
```bash
go test ./internal/validation/... -v -run TestJSONBDepth
```

---

### TASK-05: Audit and Fix SQL Injection Vectors
**Priority**: 🔴 CRITICAL  
**Estimated Time**: 4 hours  
**Dependencies**: None

#### Implementation
See `01-CRITICAL-SECURITY-ISSUES.md` CRITICAL-02 for full implementation.

#### Testing
```bash
# Audit all SQL queries
grep -rn "fmt.Sprintf.*SELECT\|UPDATE\|DELETE\|INSERT" internal/

# Run SQL injection tests
go test ./internal/repository/... -v -run TestSQLInjection
```

---

### TASK-06: Fix JWKS Refresh Race Condition
**Priority**: 🔴 CRITICAL  
**Estimated Time**: 2 hours  
**Dependencies**: None

#### Implementation
See `01-CRITICAL-SECURITY-ISSUES.md` CRITICAL-01 for full implementation.

#### Testing
```bash
go test ./internal/auth/... -v -run TestJWKSRefresh -race
```

---

## Phase 2: Critical Reliability Fixes (Days 3-5)

### TASK-07: Add Audit Logging
**Priority**: 🔴 CRITICAL  
**Estimated Time**: 1 day  
**Dependencies**: TASK-02 (handlers)

#### Implementation
See `02-CRITICAL-RELIABILITY-ISSUES.md` CRITICAL-12 for full implementation.

---

### TASK-08: Configure Database Connection Pool
**Priority**: 🔴 CRITICAL  
**Estimated Time**: 1 hour  
**Dependencies**: TASK-01 (config)

#### Implementation
See `02-CRITICAL-RELIABILITY-ISSUES.md` CRITICAL-10 for full implementation.

---

### TASK-09: Fix Transaction Context Handling
**Priority**: 🔴 CRITICAL  
**Estimated Time**: 3 hours  
**Dependencies**: None

#### Implementation
See `02-CRITICAL-RELIABILITY-ISSUES.md` CRITICAL-09 for full implementation.

---

### TASK-10: Fix Rate Limiter Goroutine Leak
**Priority**: 🔴 CRITICAL  
**Estimated Time**: 1 hour  
**Dependencies**: None

#### Implementation
See `02-CRITICAL-RELIABILITY-ISSUES.md` CRITICAL-08 for full implementation.

---

### TASK-11: Fix Rate Limiter Memory Leak
**Priority**: 🔴 CRITICAL  
**Estimated Time**: 4 hours  
**Dependencies**: TASK-10

#### Implementation
See `02-CRITICAL-RELIABILITY-ISSUES.md` CRITICAL-07 for full implementation.

---

### TASK-12: Remove Panics from Repository Code
**Priority**: 🔴 CRITICAL  
**Estimated Time**: 2 hours  
**Dependencies**: None

#### Implementation
See `02-CRITICAL-RELIABILITY-ISSUES.md` CRITICAL-11 for full implementation.

---

## Summary

### Phase 1: Critical Security (Days 1-3)
- TASK-01: Fix insecure defaults (2h)
- TASK-02: Add input validation (1d)
- TASK-03: Secure private keys (4h)
- TASK-04: Fix JSONB depth (2h)
- TASK-05: Fix SQL injection (4h)
- TASK-06: Fix JWKS race (2h)

**Total**: ~3 days

### Phase 2: Critical Reliability (Days 3-5)
- TASK-07: Add audit logging (1d)
- TASK-08: Configure DB pool (1h)
- TASK-09: Fix transaction context (3h)
- TASK-10: Fix goroutine leak (1h)
- TASK-11: Fix memory leak (4h)
- TASK-12: Remove panics (2h)

**Total**: ~2 days

### Grand Total: 5 days for all critical fixes

---

## Testing Strategy

After completing all tasks:

1. **Unit Tests**: All new code must have tests
2. **Integration Tests**: End-to-end API tests
3. **Security Tests**: Penetration testing
4. **Load Tests**: Performance under load
5. **Chaos Tests**: Failure injection

---

## Next Steps

1. Review and approve this remediation plan
2. Assign tasks to developers
3. Create tracking issues in project management tool
4. Begin implementation in priority order
5. Conduct code reviews for each task
6. Run comprehensive test suite after each phase
7. Security review before Sprint 2
