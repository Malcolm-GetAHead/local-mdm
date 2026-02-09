# Quick Reference - Issues for Sprint 2

**For**: Engineering team  
**Purpose**: Quick action list for Sprint 2 Platform Core  
**Scope**: Core MDM platform functionality  
**Status**: 🔴 **NOT STARTED** - 0/20 issues resolved (0%)

---

## Issue Lookup Table

| ID | Title | Priority | Effort | Status |
|----|-------|----------|--------|--------|
| C-01 | No Device Authentication | CRITICAL | 2d | 🔴 Open |
| C-02 | No Policy Enforcement | CRITICAL | 3d | 🔴 Open |
| C-03 | No Certificate Management | CRITICAL | 2d | 🔴 Open |
| C-04 | No Device Enrollment | CRITICAL | 3d | 🔴 Open |
| C-05 | No Command Dispatch | CRITICAL | 2d | 🔴 Open |
| C-06 | No Platform Abstraction | CRITICAL | 2d | 🔴 Open |
| C-07 | No Device State Tracking | CRITICAL | 1.5d | 🔴 Open |
| H-01 | No Bulk Operations | HIGH | 1.5d | 🔴 Open |
| H-02 | No Policy Validation | HIGH | 1d | 🔴 Open |
| H-03 | No Device Groups | HIGH | 2d | 🔴 Open |
| H-04 | No Compliance Reporting | HIGH | 1.5d | 🔴 Open |
| H-05 | No Event Notifications | HIGH | 1d | 🔴 Open |
| M-01 | No Device Search | MEDIUM | 1d | 🔴 Open |
| M-02 | No Policy Templates | MEDIUM | 1d | 🔴 Open |
| M-03 | No Device History | MEDIUM | 1.5d | 🔴 Open |
| M-04 | No Certificate Rotation | MEDIUM | 1.5d | 🔴 Open |
| M-05 | No Command Scheduling | MEDIUM | 1d | 🔴 Open |
| L-01 | No Device Tagging | LOW | 0.5d | 🔴 Open |
| L-02 | No Export Functionality | LOW | 0.5d | 🔴 Open |
| L-03 | No Device Statistics | LOW | 0.5d | 🔴 Open |

**Total**: 28 days effort, 0% complete

---

## Common Patterns

### Request Body Reading
```go
// Bounded body reading
func readRequestBody(w http.ResponseWriter, r *http.Request, maxBytes int64) ([]byte, error) {
    r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
    defer r.Body.Close()
    return io.ReadAll(r.Body)
}
```

### Error Handling
```go
// Consistent error responses
func handleError(w http.ResponseWriter, err error, code int) {
    log.Error().Err(err).Msg("operation failed")
    http.Error(w, apperrors.UserMessage(err), code)
}
```

### Device Authentication
```go
// Certificate validation
func validateDeviceCert(cert *x509.Certificate) error {
    if time.Now().After(cert.NotAfter) {
        return errors.New("certificate expired")
    }
    return nil
}
```

### Policy Enforcement
```go
// Policy application
func applyPolicy(device *Device, policy *Policy) error {
    for _, rule := range policy.Rules {
        if err := enforceRule(device, rule); err != nil {
            return fmt.Errorf("rule %s failed: %w", rule.ID, err)
        }
    }
    return nil
}
```

---

## Code Snippets for Frequent Fixes

### Device Enrollment Flow
```go
func enrollDevice(w http.ResponseWriter, r *http.Request) {
    // 1. Validate enrollment request
    req, err := parseEnrollmentRequest(r)
    if err != nil {
        handleError(w, err, http.StatusBadRequest)
        return
    }
    
    // 2. Generate device certificate
    cert, err := generateDeviceCert(req.CSR)
    if err != nil {
        handleError(w, err, http.StatusInternalServerError)
        return
    }
    
    // 3. Store device record
    device := &Device{
        ID:          generateDeviceID(),
        Certificate: cert,
        Platform:    req.Platform,
        Status:      "enrolled",
    }
    
    if err := deviceRepo.Create(device); err != nil {
        handleError(w, err, http.StatusInternalServerError)
        return
    }
    
    writeJSON(w, device)
}
```

### Command Dispatch
```go
func dispatchCommand(deviceID string, cmd *Command) error {
    device, err := deviceRepo.GetByID(deviceID)
    if err != nil {
        return fmt.Errorf("device not found: %w", err)
    }
    
    // Platform-specific dispatch
    dispatcher := getDispatcher(device.Platform)
    return dispatcher.Send(device, cmd)
}
```

### Policy Validation
```go
func validatePolicy(policy *Policy) error {
    if policy.Name == "" {
        return errors.New("policy name required")
    }
    
    for _, rule := range policy.Rules {
        if err := validateRule(rule); err != nil {
            return fmt.Errorf("invalid rule %s: %w", rule.ID, err)
        }
    }
    
    return nil
}
```

---

## Testing Commands

### Unit Tests
```bash
# Run all tests
make test

# Test specific package
go test ./internal/device/...

# Test with race detection
go test -race ./...

# Test with coverage
go test -cover ./internal/...
```

### Integration Tests
```bash
# Device enrollment
make test-enrollment

# Policy enforcement
make test-policies

# Certificate management
make test-certs

# Command dispatch
make test-commands
```

### Load Testing
```bash
# Device enrollment load test
go run cmd/loadtest/enrollment.go -devices=1000

# Policy application test
go run cmd/loadtest/policies.go -concurrent=50

# Command dispatch test
go run cmd/loadtest/commands.go -rate=100
```

---

## Verification Checklist

### Core Platform
- [ ] Device enrollment works for all platforms
- [ ] Certificate generation and validation
- [ ] Policy creation and enforcement
- [ ] Command dispatch to devices
- [ ] Device state synchronization
- [ ] Platform abstraction layer

### Security
- [ ] Device authentication required
- [ ] Certificate validation enforced
- [ ] Policy permissions checked
- [ ] Command authorization verified
- [ ] Audit logging enabled

### Performance
- [ ] Bulk operations handle 1000+ devices
- [ ] Policy enforcement under 100ms
- [ ] Command dispatch under 500ms
- [ ] Certificate operations under 200ms
- [ ] Database queries optimized

### Reliability
- [ ] Graceful error handling
- [ ] Transaction rollbacks work
- [ ] Connection retry logic
- [ ] Circuit breaker protection
- [ ] Health checks pass

---

## Links to Detailed Documents

### Sprint 2 Reviews
- [Executive Summary](EXECUTIVE_SUMMARY.md) - Sprint 2 overview
- [Critical Issues](CRITICAL_ISSUES.md) - Must-fix security issues
- [High Priority Issues](HIGH_PRIORITY_ISSUES.md) - Important features
- [Medium Priority Issues](MEDIUM_PRIORITY_ISSUES.md) - Nice-to-have features
- [Low Priority Issues](LOW_PRIORITY_ISSUES.md) - Optional enhancements
- [Issue Tracking](ISSUE_TRACKING.md) - Progress tracking
- [Action Items](ACTION_ITEMS.md) - Implementation tasks

### Implementation Docs
- `docs/implementation/sprint-2/` - Feature implementations (TBD)
- `docs/architecture/PLATFORM_CORE.md` - Core platform design
- `docs/schemas/DEVICE.md` - Device data models
- `docs/schemas/POLICY.md` - Policy definitions

### Testing & Deployment
- `docs/TESTING.md` - Testing guidelines
- `docs/dev/SETUP.md` - Development setup
- `docs/dev/QUICK_REFERENCE.md` - Common commands