# Sprint 2: Platform Core - Autonomous Implementation

**Sprint Goal**: Implement device enrollment for macOS, Windows, and Android platforms  
**Target**: Complete S2-01, S2-03, and S2-05 (parallel tasks)  
**Timeline**: Overnight autonomous development  
**Quality Bar**: Production-ready code with comprehensive tests

---

## CONTEXT

### Sprint 1 Status
✅ **Complete** (95.8% - 23/24 issues resolved)
- Database & repository layer
- Configuration & server setup
- Certificate & PKI management
- Keycloak OIDC authentication
- API framework & middleware
- Security hardening (rate limiting, circuit breaker, error sanitization)
- Testing framework (730 tests, 70.7% coverage)

### Sprint 2 Scope
**Objective**: All three platforms can enroll devices and report inventory

**Tasks to Implement** (in parallel):
1. **S2-01**: macOS NanoMDM Integration & Enrollment (4-5 days)
2. **S2-03**: Windows Discovery & Enrollment (5-6 days)
3. **S2-05**: Android Management API & Enrollment (4-5 days)

**Deferred** (sequential dependency):
- S2-02: macOS NanoDEP (requires S2-01)
- S2-04: Windows OMA-DM Sync (requires S2-03)
- S2-06: Device Service Layer (integrates with all)

---

## IMPLEMENTATION STRATEGY

### Phase 1: Foundation (1-2 hours)
Create platform-specific packages and interfaces:

```
internal/platform/
├── macos/
│   ├── nanomdm_service.go      # NanoMDM integration
│   ├── enrollment.go            # Profile generation
│   ├── apns.go                  # Push notifications
│   ├── webhook.go               # Event handling
│   └── *_test.go                # Comprehensive tests
├── windows/
│   ├── discovery.go             # MS-MDE2 discovery
│   ├── enrollment.go            # MS-MDE2 enrollment
│   ├── protocol.go              # Protocol handlers
│   ├── syncml.go                # SyncML parsing
│   └── *_test.go                # Comprehensive tests
└── android/
    ├── client.go                # Google API client
    ├── auth.go                  # Service account auth
    ├── enterprise.go            # Enterprise binding
    ├── enrollment.go            # Token generation
    ├── webhook.go               # Event handling
    └── *_test.go                # Comprehensive tests
```

### Phase 2: Platform Implementations (Parallel)

#### S2-01: macOS (Priority 1)
**Dependencies**: NanoMDM library, PostgreSQL, SCEP
**External Accounts**: None required for development

**Implementation**:
1. Import NanoMDM as Go library dependency
2. Configure with PostgreSQL storage (shared DB)
3. Wire NanoMDM HTTP handlers into router
4. Generate `.mobileconfig` enrollment profiles
5. Implement APNs push integration
6. Create webhook handler for device events
7. Add routes: `/mdm`, `/checkin`, `/api/v1/macos/enroll/{enterprise_id}`

**Acceptance Criteria**:
- [ ] NanoMDM library integrated
- [ ] Enrollment profile generation working
- [ ] APNs push certificate loading (from S1-03)
- [ ] Webhook creates device records
- [ ] Device appears in `GET /api/v1/devices` with platform=macos
- [ ] 80%+ test coverage
- [ ] All tests pass with -race

**Reference**: `docs/planning/sprints/sprint-2-platform-core/S2-01-macos-nanomdm-enrollment.md`

#### S2-03: Windows (Priority 2)
**Dependencies**: None (self-contained protocol)
**External Accounts**: None required

**Implementation**:
1. Implement MS-MDE2 discovery service
2. Handle discovery request (XML parsing)
3. Return discovery response with enrollment URL
4. Implement MS-MDE2 enrollment protocol
5. Handle RequestSecurityToken (RST)
6. Issue device certificate via SCEP
7. Create device record on successful enrollment
8. Add routes: `/EnrollmentServer/Discovery.svc`, `/EnrollmentServer/Enrollment.svc`

**Acceptance Criteria**:
- [ ] Discovery service responds correctly
- [ ] Enrollment protocol implemented
- [ ] Certificate issuance working
- [ ] Device record created
- [ ] Device appears in `GET /api/v1/devices` with platform=windows
- [ ] 80%+ test coverage
- [ ] All tests pass with -race

**Reference**: `docs/planning/sprints/sprint-2-platform-core/S2-03-windows-discovery-enrollment.md`

#### S2-05: Android (Priority 3)
**Dependencies**: Google Cloud credentials (✅ verified)
**External Accounts**: ✅ Google Cloud configured

**Implementation**:
1. Create Google API client with service account auth
2. Implement enterprise creation/binding
3. Generate enrollment tokens (QR code and NFC)
4. Generate QR code images
5. Implement webhook handler for device events
6. Add polling reconciliation (backup for webhooks)
7. Add routes: `/api/v1/android/enterprise`, `/api/v1/android/enrollment-token`, `/api/v1/android/webhook`

**Acceptance Criteria**:
- [ ] Google API client working
- [ ] Enterprise binding functional
- [ ] Enrollment token generation working
- [ ] QR code generation working
- [ ] Webhook handler processes events
- [ ] Device appears in `GET /api/v1/devices` with platform=android
- [ ] 80%+ test coverage
- [ ] All tests pass with -race

**Reference**: `docs/planning/sprints/sprint-2-platform-core/S2-05-android-enrollment.md`

### Phase 3: Integration & Testing (1-2 hours)
1. Update device repository for platform-specific fields
2. Add platform enum to device model
3. Update API responses to include platform info
4. Integration tests for each platform
5. End-to-end enrollment flow tests

### Phase 4: Documentation (30 min)
1. Update API documentation with new endpoints
2. Document enrollment flows for each platform
3. Create implementation summary for each task
4. Update Sprint 2 progress tracking

---

## CRITICAL REQUIREMENTS

### Code Quality Standards
- **Minimal code**: Write only what's needed, no over-engineering
- **Production-ready**: No TODOs, no placeholders, complete implementations
- **Error handling**: Comprehensive error handling with proper wrapping
- **Logging**: Structured logging for all operations
- **Security**: Input validation, sanitization, auth checks
- **Testing**: 80%+ coverage, race detection, edge cases

### Testing Requirements
- **Unit tests**: Every function, happy path + error paths
- **Integration tests**: Full enrollment flows
- **Race detection**: All tests must pass with `-race`
- **Edge cases**: Invalid inputs, concurrent operations, timeouts
- **Mocking**: Mock external services (NanoMDM, Google API) for tests

### Architecture Patterns (from Sprint 1)
- **Repository pattern**: Data access through repositories
- **Error wrapping**: Use `fmt.Errorf("context: %w", err)`
- **Structured logging**: Use slog with context
- **Constants**: No magic numbers, use constants package
- **Validation**: Input validation before processing
- **Context propagation**: Pass context through all layers

### Security Requirements
- **Input validation**: Validate all inputs (XML, JSON, query params)
- **SQL injection**: Use parameterized queries (already in place)
- **XSS prevention**: Sanitize outputs
- **Auth checks**: Verify authentication on all endpoints
- **Rate limiting**: Apply to enrollment endpoints
- **Audit logging**: Log all enrollment events

---

## IMPLEMENTATION GUIDELINES

### 1. Start with Interfaces
Define clear interfaces before implementation:

```go
// Platform-agnostic enrollment interface
type EnrollmentService interface {
    GenerateEnrollmentProfile(ctx context.Context, enterpriseID uuid.UUID) ([]byte, error)
    HandleEnrollment(ctx context.Context, req EnrollmentRequest) (*Device, error)
    HandleCheckIn(ctx context.Context, deviceID uuid.UUID) error
}
```

### 2. Use Existing Patterns
Follow Sprint 1 patterns:
- Repository pattern for data access
- Middleware for auth, logging, rate limiting
- Error types from `internal/apperrors`
- Validation from `internal/validation`
- Logging from `internal/logging`

### 3. Leverage Existing Infrastructure
Reuse Sprint 1 components:
- Database connection and transactions
- SCEP server for certificate issuance
- Auth middleware for endpoint protection
- Rate limiting middleware
- Audit logging
- Health checks

### 4. Test-Driven Development
Write tests first or alongside implementation:
1. Define interface
2. Write test cases
3. Implement to pass tests
4. Refactor for clarity
5. Add edge case tests

### 5. Incremental Progress
Implement in small, testable chunks:
1. Basic structure and interfaces
2. Core functionality
3. Error handling
4. Edge cases
5. Integration tests
6. Documentation

---

## EXTERNAL DEPENDENCIES

### NanoMDM (macOS)
```go
import (
    nanomdmsvc "github.com/micromdm/nanomdm/service/nanomdm"
    "github.com/micromdm/nanomdm/storage/pgsql"
    nanomdmhttp "github.com/micromdm/nanomdm/http/mdm"
)
```

**Database**: Uses existing PostgreSQL (tables from S1-01 migration)  
**Reference**: `docs/dependencies/nanomdm/`

### Google Android Management API (Android)
```go
import (
    "google.golang.org/api/androidmanagement/v1"
    "google.golang.org/api/option"
)
```

**Credentials**: `secrets/google-service-account.json` (✅ verified)  
**Reference**: `docs/planning/sprints/sprint-2-platform-core/GOOGLE_CLOUD_VERIFIED.md`

### Windows MDM Protocol
**No external dependencies** - Implement MS-MDE2 protocol directly  
**Reference**: Microsoft MS-MDE2 specification

---

## VERIFICATION CHECKLIST

After implementation, verify:

### Functionality
- [ ] macOS enrollment profile downloads
- [ ] Windows discovery service responds
- [ ] Android enrollment token generates
- [ ] All platforms create device records
- [ ] Devices appear in unified device list
- [ ] Platform-specific data stored correctly

### Testing
- [ ] All tests pass: `go test ./...`
- [ ] Race detection clean: `go test -race ./...`
- [ ] Coverage ≥80%: `go test -cover ./...`
- [ ] No vet warnings: `go vet ./...`
- [ ] Benchmarks run: `go test -bench=. ./...`

### Code Quality
- [ ] No TODO/FIXME comments
- [ ] All errors handled
- [ ] All inputs validated
- [ ] Structured logging in place
- [ ] Constants used (no magic numbers)
- [ ] Code documented (godoc comments)

### Security
- [ ] Auth required on all endpoints
- [ ] Input validation comprehensive
- [ ] SQL injection protected
- [ ] XSS prevention in place
- [ ] Rate limiting applied
- [ ] Audit logging complete

### Integration
- [ ] Server starts successfully
- [ ] Health checks pass
- [ ] API endpoints respond
- [ ] Database migrations applied
- [ ] No breaking changes to Sprint 1

---

## DELIVERABLES

### Code
1. **Platform packages**: `internal/platform/{macos,windows,android}/`
2. **Tests**: Comprehensive test suites (80%+ coverage)
3. **Routes**: New API endpoints integrated
4. **Models**: Updated device model for platforms
5. **Migrations**: Any new database schema changes

### Documentation
1. **Implementation summaries**: One per task (S2-01, S2-03, S2-05)
2. **API documentation**: Updated with new endpoints
3. **Enrollment guides**: How to enroll each platform
4. **Sprint 2 progress**: Updated tracking document

### Verification
1. **Test report**: Coverage, pass/fail, race detection
2. **Integration test results**: End-to-end enrollment flows
3. **Performance benchmarks**: Enrollment operation timings
4. **Security review**: Input validation, auth checks

---

## SUCCESS CRITERIA

### Minimum (Must Have)
- [ ] All three platforms have enrollment endpoints
- [ ] Device records created on enrollment
- [ ] Unified device list shows all platforms
- [ ] 80%+ test coverage
- [ ] All tests pass with -race
- [ ] No security vulnerabilities
- [ ] No breaking changes to Sprint 1

### Target (Should Have)
- [ ] Full enrollment flows working
- [ ] Webhook/event handlers implemented
- [ ] Platform-specific data captured
- [ ] Integration tests for each platform
- [ ] Documentation complete
- [ ] Performance benchmarks established

### Stretch (Nice to Have)
- [ ] S2-02 (macOS DEP) started
- [ ] S2-04 (Windows OMA-DM) started
- [ ] S2-06 (Device Service) started
- [ ] Real device testing guide
- [ ] Troubleshooting documentation

---

## AUTONOMOUS EXECUTION INSTRUCTIONS

### Step 1: Read Sprint 2 Documentation
- `docs/planning/sprints/sprint-2-platform-core/OVERVIEW.md`
- `docs/planning/sprints/sprint-2-platform-core/S2-01-macos-nanomdm-enrollment.md`
- `docs/planning/sprints/sprint-2-platform-core/S2-03-windows-discovery-enrollment.md`
- `docs/planning/sprints/sprint-2-platform-core/S2-05-android-enrollment.md`

### Step 2: Implement in Parallel
Start with all three platforms simultaneously:
1. Create package structure
2. Define interfaces
3. Implement core functionality
4. Add comprehensive tests
5. Integrate with server
6. Verify functionality

### Step 3: Test Continuously
After each implementation:
```bash
go test -race ./...
go test -cover ./...
go vet ./...
```

### Step 4: Document Progress
Create implementation summaries:
- `docs/implementation/sprint-2/S2-01-MACOS-ENROLLMENT.md`
- `docs/implementation/sprint-2/S2-03-WINDOWS-ENROLLMENT.md`
- `docs/implementation/sprint-2/S2-05-ANDROID-ENROLLMENT.md`

### Step 5: Final Verification
Run complete verification checklist and generate report:
- `docs/reviews/sprint-2/IMPLEMENTATION_REPORT.md`

---

## CRITICAL RULES

### DO:
✅ Write minimal, production-ready code  
✅ Test everything (80%+ coverage)  
✅ Handle all errors comprehensively  
✅ Validate all inputs  
✅ Use existing Sprint 1 patterns  
✅ Document as you go  
✅ Commit frequently with clear messages  

### DON'T:
❌ Leave TODO/FIXME comments  
❌ Skip error handling  
❌ Skip tests  
❌ Over-engineer solutions  
❌ Break Sprint 1 functionality  
❌ Commit secrets or credentials  
❌ Ignore race conditions  

---

## STOP CONDITIONS

**Stop immediately and report if**:
- Critical security vulnerability found
- Fundamental architecture flaw discovered
- Sprint 1 functionality breaks
- Cannot achieve 80% test coverage
- Race conditions cannot be resolved
- External dependency unavailable

---

## EXPECTED OUTCOME

By tomorrow morning:
- ✅ S2-01 (macOS) implemented and tested
- ✅ S2-03 (Windows) implemented and tested
- ✅ S2-05 (Android) implemented and tested
- ✅ All tests passing (730+ tests from Sprint 1 + new tests)
- ✅ 80%+ coverage maintained
- ✅ Documentation complete
- ✅ Ready for real device testing (F-01)

**Estimated**: 60-70% of Sprint 2 complete (3 of 6 tasks)

---

## REFERENCE DOCUMENTS

- Sprint 1 completion: `docs/reviews/sprint-1/`
- Sprint 2 planning: `docs/planning/sprints/sprint-2-platform-core/`
- Dependencies: `docs/dependencies/`
- Architecture: `docs/architecture/ARCHITECTURE.md`
- Testing guide: `docs/TESTING.md`
- Security guide: `docs/SECURITY.md`
- Steering guide: `.kiro/steering/STEERING.md`

---

**BEGIN AUTONOMOUS IMPLEMENTATION NOW**

Focus on: S2-01 (macOS), S2-03 (Windows), S2-05 (Android)  
Quality bar: Production-ready, 80%+ coverage, comprehensive tests  
Timeline: Overnight (8-12 hours)  
Expected result: 60-70% of Sprint 2 complete
