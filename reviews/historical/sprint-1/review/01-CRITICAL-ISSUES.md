# Critical Issues - Sprint 1 Code Review

**Priority**: 🔴 CRITICAL - Must fix before Sprint 2  
**Impact**: Data corruption, security breaches, service outages

---

## 1. No Transaction Management ✅ RESOLVED

**Status**: ✅ COMPLETED (2026-02-07)  
**Resolution**: Implemented comprehensive transaction management system

### Original Issue
Repository methods performed multiple database operations without transactions, leading to potential data corruption and orphaned records.

### Resolution Summary
Created a `Transactor` interface and updated all repositories to support transactions:
- Implemented `WithTransaction()` method for atomic operations
- Updated all repository methods to use `executor` interface
- Added support for nested transactions
- Implemented panic recovery with automatic rollback
- Created comprehensive test suite with 7 tests (all passing)

### Files Modified
- `internal/repository/transaction.go` (created)
- `internal/repository/transaction_test.go` (created)
- `internal/repository/device.go` (updated)
- `internal/repository/enterprise.go` (updated)
- `internal/repository/policy.go` (updated)

### Validation
```bash
$ go test ./internal/repository/... -v
PASS - All tests passing (including 7 new transaction tests)
```

### Documentation
See `TASK-001-TRANSACTION-MANAGEMENT.md` for complete implementation details.

---

### Issue
Repository methods perform multiple database operations without transactions, leading to potential data corruption and orphaned records.

### Location
- `internal/repository/device.go`
- `internal/repository/enterprise.go`
- `internal/repository/policy.go`

### Example Problem
```go
// In policy.go - AssignToDevice
func (r *policyRepository) AssignToDevice(ctx context.Context, deviceID, policyID uuid.UUID) error {
    query := `INSERT INTO device_policies (device_id, policy_id) VALUES ($1, $2)`
    _, err := r.db.ExecContext(ctx, query, deviceID, policyID)
    return err
}
```

**Problem**: If this fails after a related operation succeeds, data becomes inconsistent.

### Impact
- Data corruption in production
- Orphaned records
- Inconsistent state between related entities
- No rollback capability
- Race conditions in concurrent operations

### Reproduction
```go
// This will create orphaned records:
enterprise := &models.Enterprise{Name: "Test"}
repo.Create(ctx, enterprise) // Succeeds

device := &models.Device{EnterpriseID: enterprise.ID}
repo.Create(ctx, device) // Fails - but enterprise already created!
```

### Fix Required
```go
// Add transaction support to repository interface
type Repository interface {
    WithTransaction(ctx context.Context, fn func(context.Context) error) error
}

// Implement in base repository
func (r *baseRepository) WithTransaction(ctx context.Context, fn func(context.Context) error) error {
    tx, err := r.db.BeginTx(ctx, nil)
    if err != nil {
        return fmt.Errorf("begin transaction: %w", err)
    }
    
    defer func() {
        if p := recover(); p != nil {
            tx.Rollback()
            panic(p)
        }
    }()
    
    // Create context with transaction
    txCtx := context.WithValue(ctx, txKey, tx)
    
    if err := fn(txCtx); err != nil {
        if rbErr := tx.Rollback(); rbErr != nil {
            return fmt.Errorf("rollback failed: %v (original error: %w)", rbErr, err)
        }
        return err
    }
    
    if err := tx.Commit(); err != nil {
        return fmt.Errorf("commit transaction: %w", err)
    }
    
    return nil
}

// Usage in service layer
func (s *DeviceService) EnrollDevice(ctx context.Context, req *EnrollRequest) error {
    return s.repo.WithTransaction(ctx, func(txCtx context.Context) error {
        // Create device
        device := &models.Device{...}
        if err := s.deviceRepo.Create(txCtx, device); err != nil {
            return err
        }
        
        // Create certificate
        cert := &models.Certificate{DeviceID: device.ID}
        if err := s.certRepo.Create(txCtx, cert); err != nil {
            return err // Rolls back device creation
        }
        
        // Create audit log
        log := &models.AuditLog{ResourceID: device.ID}
        if err := s.auditRepo.Create(txCtx, log); err != nil {
            return err // Rolls back everything
        }
        
        return nil
    })
}
```

### Estimated Fix Time
- 4-6 hours
- Affects all repository methods
- Requires service layer refactoring

---

## 2. SQL Injection Vulnerabilities ✅ RESOLVED

**Status**: ✅ COMPLETED (2026-02-07)  
**Resolution**: Implemented column whitelists for defense-in-depth

### Original Issue
While current code uses parameterized queries (no actual vulnerability), there was a risk that future developers might add dynamic ORDER BY clauses without proper validation.

### Resolution Summary
Created column whitelists and validation functions as defense-in-depth:
- Implemented `DeviceOrderColumns`, `EnterpriseOrderColumns`, `PolicyOrderColumns` whitelists
- Created `ValidateOrderColumn` function for whitelist validation
- Added `DefaultOrderColumn` function for safe defaults
- Created comprehensive test suite with 8 tests (22 sub-tests, all passing)
- Tests include 7 common SQL injection patterns
- 100% coverage on SQL safety code

### Files Modified
- `internal/repository/sql_safety.go` (created - whitelists and validation)
- `internal/repository/sql_safety_test.go` (created - 8 tests)

### Validation
```bash
$ go test ./internal/repository/... -v -run TestSQL
PASS - All 8 SQL safety tests passing
Coverage: 100% on sql_safety.go
```

### Documentation
See `TASK-002-SQL-INJECTION-PREVENTION.md` for complete implementation details.

---

### Issue
While most queries use parameterized statements, there are potential SQL injection points in dynamic query construction.

### Location
- `internal/repository/device.go:List()` - ORDER BY clause
- Any future dynamic filtering

### Example Problem
```go
// Potential vulnerability if ORDER BY is made dynamic
func (r *deviceRepository) List(ctx context.Context, enterpriseID uuid.UUID, orderBy string, limit, offset int) {
    query := fmt.Sprintf(`
        SELECT * FROM devices 
        WHERE enterprise_id = $1 
        ORDER BY %s  -- VULNERABLE!
        LIMIT $2 OFFSET $3`, orderBy)
    // ...
}
```

### Impact
- Complete database compromise
- Data exfiltration
- Data manipulation
- Privilege escalation

### Reproduction
```go
// Attacker provides:
orderBy := "name; DROP TABLE devices; --"
// Results in:
// SELECT * FROM devices WHERE enterprise_id = $1 ORDER BY name; DROP TABLE devices; -- LIMIT $2 OFFSET $3
```

### Fix Required
```go
// Whitelist approach for dynamic columns
var allowedOrderColumns = map[string]string{
    "name":       "name",
    "created_at": "created_at",
    "updated_at": "updated_at",
    "status":     "status",
}

func (r *deviceRepository) List(ctx context.Context, enterpriseID uuid.UUID, orderBy string, limit, offset int) ([]*models.Device, int, error) {
    // Validate and sanitize orderBy
    column, ok := allowedOrderColumns[orderBy]
    if !ok {
        column = "created_at" // Default
    }
    
    query := fmt.Sprintf(`
        SELECT * FROM devices 
        WHERE enterprise_id = $1 AND deleted_at IS NULL
        ORDER BY %s DESC
        LIMIT $2 OFFSET $3`, column) // Safe - from whitelist
    
    // ...
}
```

### Estimated Fix Time
- 2-3 hours
- Add validation for all dynamic query parts
- Create whitelist constants

---

## 3. Missing Context Timeout Enforcement ✅ RESOLVED

**Status**: ✅ COMPLETED (2026-02-07)  
**Resolution**: Implemented timeout middleware and configuration

### Original Issue
Database operations and HTTP handlers didn't enforce context timeouts, leading to hanging requests and resource exhaustion.

### Resolution Summary
Created timeout middleware and added configuration support:
- Implemented `timeoutMiddleware` with configurable duration
- Applied middleware first in chain to protect all endpoints
- Added `RequestTimeout` (30s) and `QueryTimeout` (10s) configuration
- Context properly propagates to all database operations
- Created comprehensive test suite with 3 tests (all passing)

### Files Modified
- `internal/api/server.go` (added timeout middleware)
- `internal/api/timeout_test.go` (created - 3 tests)
- `internal/config/config.go` (added timeout configuration)
- `configs/config.yaml` (added timeout values)
- `configs/config.example.yaml` (added timeout values)

### Validation
```bash
$ go test ./internal/api/... -v -run TestTimeout
PASS - All 3 timeout tests passing
```

### Documentation
See `TASK-003-CONTEXT-TIMEOUTS.md` for complete implementation details.

---

### Issue
Database operations and HTTP handlers don't enforce context timeouts, leading to hanging requests and resource exhaustion.

### Location
- All repository methods
- All HTTP handlers
- `internal/api/handlers.go`

### Example Problem
```go
// Handler doesn't set timeout
func (s *Server) handleListDevices(w http.ResponseWriter, r *http.Request) {
    // Uses request context directly - no timeout!
    devices, _, err := s.deviceRepo.List(r.Context(), enterpriseID, 100, 0)
    // If database hangs, this request hangs forever
}
```

### Impact
- Resource exhaustion (goroutines, connections)
- Service unavailability
- Cascading failures
- No protection against slow queries

### Reproduction
```bash
# Simulate slow query
BEGIN;
LOCK TABLE devices IN ACCESS EXCLUSIVE MODE;
# Now all device queries hang forever
```

### Fix Required
```go
// Add timeout middleware
func timeoutMiddleware(timeout time.Duration) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            ctx, cancel := context.WithTimeout(r.Context(), timeout)
            defer cancel()
            
            done := make(chan struct{})
            go func() {
                next.ServeHTTP(w, r.WithContext(ctx))
                close(done)
            }()
            
            select {
            case <-done:
                return
            case <-ctx.Done():
                http.Error(w, "Request timeout", http.StatusGatewayTimeout)
            }
        })
    }
}

// Apply to all routes
s.router.Use(timeoutMiddleware(30 * time.Second))

// Also enforce in repository
func (r *deviceRepository) List(ctx context.Context, ...) ([]*models.Device, int, error) {
    // Enforce maximum timeout
    ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
    defer cancel()
    
    // Use ctx for all operations
    rows, err := r.db.QueryContext(ctx, query, ...)
    // ...
}
```

### Estimated Fix Time
- 3-4 hours
- Add timeout middleware
- Update all repository methods
- Add timeout configuration

---

## 4. No Rate Limiting Implementation ✅ RESOLVED

**Status**: ✅ COMPLETED (2026-02-07)  
**Resolution**: Applied existing rate limiting middleware with configuration support

### Original Issue
`internal/api/ratelimit.go` existed but was never used. API was vulnerable to abuse and DDoS.

### Resolution Summary
Applied the existing rate limiter as middleware with configuration support:
- Rate limiting now active on all endpoints
- Default: 100 requests/minute per IP
- Configurable via config.yaml
- Can be enabled/disabled
- Comprehensive test suite (10 tests, all passing)

### Files Modified
- `internal/api/server.go` (applied middleware)
- `internal/api/ratelimit_test.go` (created - 10 tests)
- `internal/config/config.go` (added RateLimitConfig)
- `configs/config.yaml` (added rate_limit section)
- `configs/config.example.yaml` (added rate_limit section)

### Implementation
```go
func (s *Server) setupMiddleware() {
	// Rate limiting - apply early to protect all endpoints
	if s.config.Server.RateLimit.Enabled {
		limit := s.config.Server.RateLimit.RequestsPerMin
		window := s.config.Server.RateLimit.Window
		globalLimiter := newRateLimiter(limit, window)
		s.router.Use(rateLimitMiddleware(globalLimiter))
	}
	// ... other middleware
}
```

### Configuration
```yaml
server:
  rate_limit:
    enabled: true
    requests_per_min: 100
    window: 1m
```

### Validation
```bash
$ go test ./internal/api/... -v -run TestRateLimit
PASS - All 10 tests passing
```

### Documentation
See `TASK-004-RATE-LIMITING.md` for complete implementation details.

---
    rateLimiter := NewRateLimiter(100, time.Minute) // 100 req/min
    s.router.Use(rateLimiter.Middleware())
    
    s.router.Use(securityHeadersMiddleware)
    s.router.Use(corsMiddleware)
}

// Different limits for different endpoints
func (s *Server) setupRoutes() {
    // Strict limit for auth endpoints
    authLimiter := NewRateLimiter(5, time.Minute)
    api.HandleFunc("/auth/login", authLimiter.Wrap(s.handleLogin))
    
    // More lenient for read operations
    readLimiter := NewRateLimiter(100, time.Minute)
    api.Handle("/devices", readLimiter.Wrap(s.authMiddleware.RequireAuth(
        http.HandlerFunc(s.handleListDevices),
    )))
}
```

### Estimated Fix Time
- 2-3 hours
- Apply existing rate limiter
- Configure per-endpoint limits
- Add Redis backend for distributed rate limiting (future)

---

## 5. No Input Validation on API Handlers 🔴 CRITICAL

### Issue
API handlers accept and process input without validation, leading to invalid data in database and potential exploits.

### Location
- `internal/api/handlers.go` - all handlers
- No validation layer between HTTP and repository

### Example Problem
```go
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
    var req auth.LoginRequest
    if err := parseJSONBody(r, &req); err != nil {
        respondError(w, r, http.StatusBadRequest, "invalid_request", err.Error())
        return
    }
    
    // ❌ No validation of username/password format
    // ❌ No length checks
    // ❌ No sanitization
    
    tokenResp, err := kc.Login(req.Username, req.Password)
    // ...
}
```

### Impact
- Invalid data in database
- Buffer overflow potential
- XSS attacks
- SQL injection (if validation bypassed)
- Service crashes from malformed input

### Reproduction
```bash
# Send invalid data
curl -X POST http://localhost:8080/api/v1/auth/login \
    -d '{"username":"' $(python -c 'print("A"*10000)') '","password":"test"}'
# No validation - processes 10KB username
```

### Fix Required
```go
// Create validation layer
package validation

type LoginRequest struct {
    Username string `json:"username" validate:"required,email,max=255"`
    Password string `json:"password" validate:"required,min=8,max=128"`
}

func (r *LoginRequest) Validate() error {
    if err := validator.Struct(r); err != nil {
        return fmt.Errorf("validation failed: %w", err)
    }
    
    // Additional business logic validation
    if strings.Contains(r.Username, "..") {
        return errors.New("invalid username format")
    }
    
    return nil
}

// In handler
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
    var req validation.LoginRequest
    if err := parseJSONBody(r, &req); err != nil {
        respondError(w, r, http.StatusBadRequest, "invalid_request", err.Error())
        return
    }
    
    // Validate input
    if err := req.Validate(); err != nil {
        respondError(w, r, http.StatusBadRequest, "validation_failed", err.Error())
        return
    }
    
    // Sanitize
    req.Username = validation.SanitizeEmail(req.Username)
    
    // Now safe to process
    tokenResp, err := kc.Login(req.Username, req.Password)
    // ...
}
```

### Validation Rules Needed
```go
// Device enrollment
type EnrollDeviceRequest struct {
    Platform     string `validate:"required,oneof=windows macos android"`
    DeviceID     string `validate:"required,max=255"`
    SerialNumber string `validate:"required,max=255,alphanum"`
    Name         string `validate:"required,max=255"`
    Model        string `validate:"max=255"`
    OSVersion    string `validate:"max=100"`
}

// Policy creation
type CreatePolicyRequest struct {
    Name         string `validate:"required,min=3,max=255"`
    Description  string `validate:"max=1000"`
    Platform     string `validate:"required,oneof=windows macos android"`
    PolicyType   string `validate:"required,oneof=wifi vpn security app restriction compliance"`
    PolicyConfig map[string]interface{} `validate:"required"`
}

// Enterprise creation
type CreateEnterpriseRequest struct {
    Name string `validate:"required,min=3,max=255"`
    Slug string `validate:"required,min=3,max=100,alphanum"`
}
```

### Estimated Fix Time
- 6-8 hours
- Add validation package
- Create validation structs for all requests
- Update all handlers
- Add validation tests

---

## 6. CORS Wildcard Configuration ✅ RESOLVED

**Status**: ✅ COMPLETED (2026-02-07)  
**Resolution**: Replaced wildcard CORS with origin whitelist validation

### Original Issue
CORS middleware allowed all origins (`*`), enabling XSS and CSRF attacks from any website.

### Resolution Summary
Replaced wildcard CORS with proper origin validation:
- Origin whitelist configuration
- Exact match and wildcard subdomain support
- Configurable methods, headers, credentials
- Proper preflight handling
- Comprehensive test suite (11 tests, all passing)

### Files Modified
- `internal/api/server.go` (replaced wildcard CORS)
- `internal/api/cors_test.go` (created - 11 tests)
- `internal/config/config.go` (added CORSConfig)
- `configs/config.yaml` (added cors section)
- `configs/config.example.yaml` (added cors section)

### Implementation
```go
func corsMiddleware(cfg config.CORSConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			
			// Check if origin is allowed
			if origin != "" && isAllowedOrigin(origin, cfg.AllowedOrigins) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				// ... set other headers
			}
			// ... handle preflight
		})
	}
}
```

### Configuration
```yaml
server:
  cors:
    allowed_origins:
      - "http://localhost:3000"
      - "http://localhost:8080"
    allowed_methods:
      - "GET"
      - "POST"
      - "PUT"
      - "DELETE"
      - "OPTIONS"
    allowed_headers:
      - "Content-Type"
      - "Authorization"
    allow_credentials: true
    max_age: 3600
```

### Validation
```bash
$ go test ./internal/api/... -v -run TestCORS
PASS - All 11 tests passing
```

### Documentation
See `TASK-006-CORS-CONFIGURATION.md` for complete implementation details.

---
            w.Header().Set("Access-Control-Allow-Methods", strings.Join(cfg.AllowedMethods, ", "))
            w.Header().Set("Access-Control-Allow-Headers", strings.Join(cfg.AllowedHeaders, ", "))
            
            if cfg.AllowCredentials {
                w.Header().Set("Access-Control-Allow-Credentials", "true")
            }
            
            if cfg.MaxAge > 0 {
                w.Header().Set("Access-Control-Max-Age", strconv.Itoa(cfg.MaxAge))
            }
            
            if r.Method == "OPTIONS" {
                w.WriteHeader(http.StatusOK)
                return
            }
            
            next.ServeHTTP(w, r)
        })
    }
}

func isAllowedOrigin(origin string, allowed []string) bool {
    for _, o := range allowed {
        if o == origin || o == "*" {
            return true
        }
        // Support wildcard subdomains: *.example.com
        if strings.HasPrefix(o, "*.") {
            domain := strings.TrimPrefix(o, "*.")
            if strings.HasSuffix(origin, domain) {
                return true
            }
        }
    }
    return false
}
```

### Estimated Fix Time
- 2-3 hours
- Add CORS configuration
- Implement origin validation
- Update tests

---

## 7. No Audit Logging Implementation 🔴 CRITICAL

### Issue
Audit log table exists in database but is never used. No tracking of who did what and when.

### Location
- `migrations/000001_initial_schema.up.sql` - table defined
- No audit logging service
- No audit log creation in handlers

### Impact
- No compliance (SOC2, HIPAA, GDPR)
- No forensics capability
- No accountability
- Can't track security incidents
- Can't detect unauthorized access

### Fix Required
```go
// Create audit service
package audit

type Service struct {
    db     *sql.DB
    logger *slog.Logger
}

type LogEntry struct {
    EnterpriseID *uuid.UUID
    UserID       *uuid.UUID
    Action       string
    ResourceType string
    ResourceID   *uuid.UUID
    Details      map[string]interface{}
    IPAddress    string
    UserAgent    string
}

func (s *Service) Log(ctx context.Context, entry *LogEntry) error {
    query := `
        INSERT INTO audit_logs (
            enterprise_id, user_id, action, resource_type, resource_id,
            details, ip_address, user_agent
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
    
    details, _ := json.Marshal(entry.Details)
    
    _, err := s.db.ExecContext(ctx, query,
        entry.EnterpriseID, entry.UserID, entry.Action,
        entry.ResourceType, entry.ResourceID, details,
        entry.IPAddress, entry.UserAgent,
    )
    
    if err != nil {
        s.logger.Error("Failed to create audit log", "error", err)
        // Don't fail the request if audit logging fails
    }
    
    return nil
}

// Middleware to automatically log all requests
func (s *Service) Middleware() func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            start := time.Now()
            wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
            
            next.ServeHTTP(wrapped, r)
            
            // Log after request completes
            user, _ := auth.UserFromContext(r.Context())
            
            entry := &LogEntry{
                Action:       fmt.Sprintf("%s %s", r.Method, r.URL.Path),
                ResourceType: "http_request",
                IPAddress:    r.RemoteAddr,
                UserAgent:    r.UserAgent(),
                Details: map[string]interface{}{
                    "method":      r.Method,
                    "path":        r.URL.Path,
                    "status_code": wrapped.statusCode,
                    "duration_ms": time.Since(start).Milliseconds(),
                },
            }
            
            if user != nil {
                entry.UserID = &user.ID
                entry.EnterpriseID = &user.EnterpriseID
            }
            
            s.Log(r.Context(), entry)
        })
    }
}

// Usage in handlers
func (s *Server) handleCreateDevice(w http.ResponseWriter, r *http.Request) {
    var req CreateDeviceRequest
    // ... validation ...
    
    device, err := s.deviceService.Create(r.Context(), &req)
    if err != nil {
        return
    }
    
    // Audit log
    user, _ := auth.UserFromContext(r.Context())
    s.auditService.Log(r.Context(), &audit.LogEntry{
        EnterpriseID: &user.EnterpriseID,
        UserID:       &user.ID,
        Action:       "device.create",
        ResourceType: "device",
        ResourceID:   &device.ID,
        Details: map[string]interface{}{
            "platform":      device.Platform,
            "serial_number": device.SerialNumber,
        },
        IPAddress: r.RemoteAddr,
        UserAgent: r.UserAgent(),
    })
    
    respondJSON(w, r, http.StatusCreated, device)
}
```

### Actions to Audit
```go
const (
    // Authentication
    ActionLogin         = "auth.login"
    ActionLogout        = "auth.logout"
    ActionLoginFailed   = "auth.login_failed"
    
    // Devices
    ActionDeviceEnroll  = "device.enroll"
    ActionDeviceUpdate  = "device.update"
    ActionDeviceDelete  = "device.delete"
    ActionDeviceLock    = "device.lock"
    ActionDeviceWipe    = "device.wipe"
    
    // Policies
    ActionPolicyCreate  = "policy.create"
    ActionPolicyUpdate  = "policy.update"
    ActionPolicyDelete  = "policy.delete"
    ActionPolicyAssign  = "policy.assign"
    
    // Enterprises
    ActionEnterpriseCreate = "enterprise.create"
    ActionEnterpriseUpdate = "enterprise.update"
    ActionEnterpriseDelete = "enterprise.delete"
    
    // Certificates
    ActionCertIssue     = "cert.issue"
    ActionCertRevoke    = "cert.revoke"
)
```

### Estimated Fix Time
- 4-6 hours
- Create audit service
- Add audit middleware
- Update all handlers
- Add audit log queries

---

## 8. Missing Error Context and Wrapping 🔴 CRITICAL

### Issue
Errors are returned without context, making debugging impossible in production.

### Location
- All repository methods
- All service methods
- All handlers

### Example Problem
```go
func (r *deviceRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Device, error) {
    // ...
    if err == sql.ErrNoRows {
        return nil, fmt.Errorf("device not found")  // ❌ No context!
    }
    return device, err  // ❌ No wrapping!
}

// In handler
device, err := s.deviceRepo.GetByID(ctx, id)
if err != nil {
    // Error is just "device not found" - no idea which device, which user, which enterprise
    respondError(w, r, http.StatusNotFound, "not_found", err.Error())
    return
}
```

### Impact
- Impossible to debug production issues
- No error tracing
- Can't identify root cause
- No correlation between errors
- Support tickets take hours to resolve

### Fix Required
```go
// Use error wrapping
func (r *deviceRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Device, error) {
    query := `SELECT ... FROM devices WHERE id = $1 AND deleted_at IS NULL`
    
    device := &models.Device{}
    err := r.db.QueryRowContext(ctx, query, id).Scan(...)
    if err == sql.ErrNoRows {
        return nil, fmt.Errorf("device not found: id=%s: %w", id, ErrNotFound)
    }
    if err != nil {
        return nil, fmt.Errorf("query device: id=%s: %w", id, err)
    }
    
    return device, nil
}

// In service layer
func (s *DeviceService) GetDevice(ctx context.Context, id uuid.UUID) (*models.Device, error) {
    user, err := auth.UserFromContext(ctx)
    if err != nil {
        return nil, fmt.Errorf("get user from context: %w", err)
    }
    
    device, err := s.repo.GetByID(ctx, id)
    if err != nil {
        return nil, fmt.Errorf("get device: user=%s enterprise=%s device=%s: %w",
            user.ID, user.EnterpriseID, id, err)
    }
    
    // Verify enterprise access
    if device.EnterpriseID != user.EnterpriseID {
        return nil, fmt.Errorf("access denied: user=%s enterprise=%s device_enterprise=%s: %w",
            user.ID, user.EnterpriseID, device.EnterpriseID, ErrForbidden)
    }
    
    return device, nil
}

// In handler
func (s *Server) handleGetDevice(w http.ResponseWriter, r *http.Request) {
    id, err := uuid.Parse(mux.Vars(r)["id"])
    if err != nil {
        respondError(w, r, http.StatusBadRequest, "invalid_id", "Invalid device ID format")
        return
    }
    
    device, err := s.deviceService.GetDevice(r.Context(), id)
    if err != nil {
        // Log full error with context
        s.logger.Error("Failed to get device",
            "error", err,
            "device_id", id,
            "path", r.URL.Path,
            "method", r.Method,
        )
        
        // Return user-friendly error
        if errors.Is(err, ErrNotFound) {
            respondError(w, r, http.StatusNotFound, "not_found", "Device not found")
        } else if errors.Is(err, ErrForbidden) {
            respondError(w, r, http.StatusForbidden, "forbidden", "Access denied")
        } else {
            respondError(w, r, http.StatusInternalServerError, "internal_error", "Internal server error")
        }
        return
    }
    
    respondJSON(w, r, http.StatusOK, device)
}

// Define sentinel errors
var (
    ErrNotFound   = errors.New("not found")
    ErrForbidden  = errors.New("forbidden")
    ErrConflict   = errors.New("conflict")
    ErrBadRequest = errors.New("bad request")
)
```

### Estimated Fix Time
- 6-8 hours
- Update all error returns
- Add error wrapping
- Define sentinel errors
- Update error handling in handlers

---

## Summary of Critical Issues

| Issue | Impact | Fix Time | Priority |
|-------|--------|----------|----------|
| No Transaction Management | Data Corruption | 4-6h | P0 |
| SQL Injection | Security Breach | 2-3h | P0 |
| No Context Timeouts | Service Outage | 3-4h | P0 |
| No Rate Limiting | DDoS Vulnerability | 2-3h | P0 |
| No Input Validation | Data Integrity | 6-8h | P0 |
| CORS Wildcard | Security Breach | 2-3h | P0 |
| No Audit Logging | Compliance Failure | 4-6h | P1 |
| Missing Error Context | Debugging Impossible | 6-8h | P1 |

**Total Estimated Fix Time**: 29-43 hours (4-6 days)

**Recommendation**: Fix P0 issues (1-6) before Sprint 2. P1 issues (7-8) can be done in parallel with Sprint 2 but should be completed within the sprint.
