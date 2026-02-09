# Critical Issues (Must Fix Before Production)

**Priority**: CRITICAL  
**Total Issues**: 7  
**Estimated Effort**: 4-5 days  
**Risk Level**: Production blockers

---

## C-01: DoS via Unbounded Request Body Reading

**Severity**: CRITICAL  
**Category**: Security/Availability  
**Impact**: Memory exhaustion, server crash, denial of service  
**Effort**: 0.5 days  
**Status**: 🔴 **Open** (0% progress)

### Problem
HTTP handlers read request bodies without size limits, allowing attackers to exhaust server memory.

**Location**: `internal/api/handlers.go` (multiple endpoints)

### Exploit Scenario
1. Attacker sends POST request with 1GB+ body to `/api/v1/devices`
2. Server attempts to read entire body into memory
3. Memory exhaustion causes OOM kill or server crash
4. Service becomes unavailable for legitimate users

### Fix
Implement request body size limits using `http.MaxBytesReader`:

```go
// internal/api/middleware.go
func requestSizeLimitMiddleware(maxBytes int64) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            if r.ContentLength > maxBytes {
                http.Error(w, "Request too large", http.StatusRequestEntityTooLarge)
                return
            }
            r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
            next.ServeHTTP(w, r)
        })
    }
}

// Apply to all API routes
api.Use(requestSizeLimitMiddleware(1 << 20)) // 1MB limit
```

### Test Cases
1. Send 2MB request body - should return 413
2. Send exactly 1MB - should process normally
3. Verify memory usage stays bounded under load
4. Test with concurrent large requests

### Verification Steps
1. Load test with large payloads
2. Monitor memory usage during attack
3. Verify 413 responses for oversized requests
4. Confirm server remains responsive

---

## C-02: Hardcoded SCEP Challenge Password

**Severity**: CRITICAL  
**Category**: Security  
**Impact**: Unauthorized device enrollment, certificate compromise  
**Effort**: 0.5 days  
**Status**: 🔴 **Open** (0% progress)

### Problem
SCEP challenge password is hardcoded in source code, allowing anyone to enroll devices.

**Location**: `internal/scep/server.go:45`

### Exploit Scenario
1. Attacker discovers hardcoded challenge in source code
2. Creates malicious device enrollment request
3. Obtains valid client certificate from SCEP server
4. Uses certificate to authenticate as legitimate device
5. Gains access to MDM commands and enterprise data

### Fix
Generate dynamic challenge passwords with expiration:

```go
// internal/scep/challenge.go
type ChallengeManager struct {
    challenges map[string]*Challenge
    mu         sync.RWMutex
}

type Challenge struct {
    Password  string
    ExpiresAt time.Time
    DeviceID  string
    Used      bool
}

func (cm *ChallengeManager) GenerateChallenge(deviceID string) string {
    cm.mu.Lock()
    defer cm.mu.Unlock()
    
    password := generateSecurePassword(32)
    cm.challenges[password] = &Challenge{
        Password:  password,
        ExpiresAt: time.Now().Add(5 * time.Minute),
        DeviceID:  deviceID,
        Used:      false,
    }
    return password
}

func (cm *ChallengeManager) ValidateChallenge(password string) (string, bool) {
    cm.mu.Lock()
    defer cm.mu.Unlock()
    
    challenge, exists := cm.challenges[password]
    if !exists || challenge.Used || time.Now().After(challenge.ExpiresAt) {
        return "", false
    }
    
    challenge.Used = true
    return challenge.DeviceID, true
}
```

### Test Cases
1. Generate challenge - should return unique password
2. Use challenge twice - second attempt should fail
3. Wait for expiration - expired challenge should fail
4. Validate with wrong password - should fail

### Verification Steps
1. Verify no hardcoded passwords in codebase
2. Test challenge generation and validation
3. Confirm challenges expire properly
4. Test concurrent challenge operations

---

## C-03: Weak Random Number Generation

**Severity**: CRITICAL  
**Category**: Security  
**Impact**: Predictable tokens, session hijacking, cryptographic weakness  
**Effort**: 0.5 days  
**Status**: 🔴 **Open** (0% progress)

### Problem
Using `math/rand` for security-sensitive random generation instead of `crypto/rand`.

**Location**: `internal/auth/tokens.go:23`, `internal/scep/challenge.go:67`

### Exploit Scenario
1. Attacker analyzes sequence of generated tokens/challenges
2. Identifies predictable pattern in PRNG output
3. Predicts future token values
4. Generates valid session tokens for other users
5. Gains unauthorized access to admin accounts

### Fix
Replace all `math/rand` usage with `crypto/rand`:

```go
// internal/crypto/random.go
package crypto

import (
    "crypto/rand"
    "encoding/base64"
    "fmt"
)

func GenerateSecureBytes(length int) ([]byte, error) {
    bytes := make([]byte, length)
    if _, err := rand.Read(bytes); err != nil {
        return nil, fmt.Errorf("failed to generate random bytes: %w", err)
    }
    return bytes, nil
}

func GenerateSecureString(length int) (string, error) {
    bytes, err := GenerateSecureBytes(length)
    if err != nil {
        return "", err
    }
    return base64.URLEncoding.EncodeToString(bytes)[:length], nil
}

func GenerateSecureToken() (string, error) {
    return GenerateSecureString(32)
}
```

### Test Cases
1. Generate 1000 tokens - verify no duplicates
2. Statistical randomness test (chi-square)
3. Verify entropy meets cryptographic standards
4. Performance test vs math/rand

### Verification Steps
1. Audit all random generation usage
2. Replace math/rand with crypto/rand
3. Run statistical randomness tests
4. Verify no predictable patterns

---

## C-04: No Authentication on Admin Endpoints

**Severity**: CRITICAL  
**Category**: Security  
**Impact**: Complete system compromise, unauthorized admin access  
**Effort**: 1 day  
**Status**: 🔴 **Open** (0% progress)

### Problem
Admin endpoints like `/api/v1/admin/users` have no authentication middleware.

**Location**: `internal/api/server.go:89-95`

### Exploit Scenario
1. Attacker discovers admin endpoints through enumeration
2. Directly accesses `/api/v1/admin/users` without authentication
3. Retrieves list of all admin users and their details
4. Uses information for targeted attacks
5. Creates new admin accounts via `/api/v1/admin/users` POST

### Fix
Implement JWT authentication middleware for admin routes:

```go
// internal/api/auth_middleware.go
func requireAuthMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        token := extractBearerToken(r)
        if token == "" {
            respondError(w, r, http.StatusUnauthorized, "missing_token", "Authorization token required")
            return
        }
        
        claims, err := validateJWT(token)
        if err != nil {
            respondError(w, r, http.StatusUnauthorized, "invalid_token", "Invalid or expired token")
            return
        }
        
        ctx := context.WithValue(r.Context(), "user_id", claims.UserID)
        ctx = context.WithValue(ctx, "user_role", claims.Role)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

func requireAdminMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        role := r.Context().Value("user_role").(string)
        if role != "admin" {
            respondError(w, r, http.StatusForbidden, "insufficient_privileges", "Admin access required")
            return
        }
        next.ServeHTTP(w, r)
    })
}

// Apply to admin routes
adminRoutes := api.PathPrefix("/admin").Subrouter()
adminRoutes.Use(requireAuthMiddleware)
adminRoutes.Use(requireAdminMiddleware)
```

### Test Cases
1. Access admin endpoint without token - should return 401
2. Access with invalid token - should return 401
3. Access with non-admin token - should return 403
4. Access with valid admin token - should succeed

### Verification Steps
1. Audit all admin endpoints for auth middleware
2. Test authentication bypass attempts
3. Verify role-based access control
4. Test token validation edge cases

---

## C-05: No Webhook Signature Verification

**Severity**: CRITICAL  
**Category**: Security  
**Impact**: Command injection, data manipulation, unauthorized actions  
**Effort**: 0.5 days  
**Status**: 🔴 **Open** (0% progress)

### Problem
Webhook endpoints accept unsigned requests, allowing attackers to forge device commands.

**Location**: `internal/api/webhooks.go:34-56`

### Exploit Scenario
1. Attacker identifies webhook endpoint URL
2. Crafts malicious webhook payload with device commands
3. Sends unsigned request to webhook endpoint
4. Server processes fake webhook as legitimate
5. Executes unauthorized commands on managed devices

### Fix
Implement HMAC signature verification for webhooks:

```go
// internal/api/webhook_auth.go
func verifyWebhookSignature(payload []byte, signature string, secret []byte) bool {
    expectedMAC := hmac.New(sha256.New, secret)
    expectedMAC.Write(payload)
    expectedSignature := hex.EncodeToString(expectedMAC.Sum(nil))
    
    return hmac.Equal([]byte(signature), []byte(expectedSignature))
}

func webhookAuthMiddleware(secret []byte) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            signature := r.Header.Get("X-Webhook-Signature")
            if signature == "" {
                http.Error(w, "Missing signature", http.StatusUnauthorized)
                return
            }
            
            body, err := io.ReadAll(r.Body)
            if err != nil {
                http.Error(w, "Cannot read body", http.StatusBadRequest)
                return
            }
            r.Body = io.NopCloser(bytes.NewReader(body))
            
            if !verifyWebhookSignature(body, signature, secret) {
                http.Error(w, "Invalid signature", http.StatusUnauthorized)
                return
            }
            
            next.ServeHTTP(w, r)
        })
    }
}
```

### Test Cases
1. Send webhook without signature - should return 401
2. Send webhook with invalid signature - should return 401
3. Send webhook with valid signature - should process
4. Test signature with modified payload - should fail

### Verification Steps
1. Configure webhook secrets in all environments
2. Test signature verification with known payloads
3. Verify replay attack protection
4. Test with various payload sizes

---

## C-06: No Input Validation on API Endpoints

**Severity**: CRITICAL  
**Category**: Security  
**Impact**: SQL injection, XSS, data corruption, system compromise  
**Effort**: 1 day  
**Status**: 🔴 **Open** (0% progress)

### Problem
API endpoints accept user input without validation, sanitization, or size limits.

**Location**: `internal/api/devices.go`, `internal/api/policies.go` (all handlers)

### Exploit Scenario
1. Attacker sends malicious JSON payload to device creation endpoint
2. Payload contains SQL injection in device name field
3. Unsanitized input passed directly to database query
4. Attacker gains database access and extracts sensitive data
5. Modifies device policies to compromise entire fleet

### Fix
Implement comprehensive input validation:

```go
// internal/validation/validator.go
type DeviceRequest struct {
    Name     string `json:"name" validate:"required,min=1,max=100,alphanum"`
    Platform string `json:"platform" validate:"required,oneof=windows macos android"`
    SerialNo string `json:"serial_number" validate:"required,min=8,max=50,alphanum"`
}

func ValidateStruct(s interface{}) error {
    validate := validator.New()
    return validate.Struct(s)
}

func validateAndBind(r *http.Request, v interface{}) error {
    if err := json.NewDecoder(r.Body).Decode(v); err != nil {
        return fmt.Errorf("invalid JSON: %w", err)
    }
    
    if err := ValidateStruct(v); err != nil {
        return fmt.Errorf("validation failed: %w", err)
    }
    
    return nil
}

// Usage in handlers
func (s *Server) handleCreateDevice(w http.ResponseWriter, r *http.Request) {
    var req DeviceRequest
    if err := validateAndBind(r, &req); err != nil {
        respondError(w, r, http.StatusBadRequest, "validation_error", err.Error())
        return
    }
    
    // Sanitize input
    req.Name = html.EscapeString(strings.TrimSpace(req.Name))
    
    // Process validated request
    device, err := s.deviceService.Create(req)
    // ...
}
```

### Test Cases
1. Send invalid JSON - should return 400
2. Send missing required fields - should return validation error
3. Send oversized strings - should reject
4. Send SQL injection payloads - should sanitize
5. Send XSS payloads - should escape

### Verification Steps
1. Add validation to all API endpoints
2. Test with malicious payloads
3. Verify SQL injection protection
4. Test XSS prevention
5. Validate all input edge cases

---

## C-07: No Rate Limiting on API Endpoints

**Severity**: CRITICAL  
**Category**: Security/Availability  
**Impact**: DoS attacks, resource exhaustion, service degradation  
**Effort**: 1 day  
**Status**: 🔴 **Open** (0% progress)

### Problem
API endpoints have no rate limiting, allowing unlimited requests that can overwhelm the server.

**Location**: `internal/api/server.go` (all routes)

### Exploit Scenario
1. Attacker identifies API endpoints through reconnaissance
2. Launches high-volume request flood (1000+ req/sec)
3. Server resources (CPU, memory, DB connections) exhausted
4. Legitimate users cannot access service
5. Database becomes unresponsive under load

### Fix
Implement tiered rate limiting based on endpoint sensitivity:

```go
// internal/api/rate_limiter.go
type RateLimiter struct {
    limiters map[string]*rate.Limiter
    mu       sync.RWMutex
}

func NewRateLimiter(rps int, burst int) *RateLimiter {
    return &RateLimiter{
        limiters: make(map[string]*rate.Limiter),
    }
}

func (rl *RateLimiter) getLimiter(key string, rps int, burst int) *rate.Limiter {
    rl.mu.Lock()
    defer rl.mu.Unlock()
    
    limiter, exists := rl.limiters[key]
    if !exists {
        limiter = rate.NewLimiter(rate.Limit(rps), burst)
        rl.limiters[key] = limiter
    }
    return limiter
}

func rateLimitMiddleware(rps, burst int) func(http.Handler) http.Handler {
    rl := NewRateLimiter(rps, burst)
    
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            ip := getClientIP(r)
            limiter := rl.getLimiter(ip, rps, burst)
            
            if !limiter.Allow() {
                w.Header().Set("Retry-After", "60")
                respondError(w, r, http.StatusTooManyRequests, "rate_limit_exceeded", 
                    "Too many requests")
                return
            }
            
            next.ServeHTTP(w, r)
        })
    }
}

// Apply different limits to different endpoint groups
api.Use(rateLimitMiddleware(100, 200))  // General API: 100 req/sec
authRoutes.Use(rateLimitMiddleware(10, 20))  // Auth: 10 req/sec
adminRoutes.Use(rateLimitMiddleware(50, 100)) // Admin: 50 req/sec
```

### Test Cases
1. Send requests at limit rate - should process normally
2. Exceed rate limit - should return 429
3. Test burst capacity handling
4. Verify rate limit resets over time
5. Test with multiple concurrent clients

### Verification Steps
1. Load test each endpoint group
2. Monitor response times under load
3. Verify 429 responses include Retry-After
4. Test rate limit effectiveness
5. Confirm server stability under attack