# Testing Guide

**Last Updated**: 2026-04-23

## Running Tests

All development and testing runs inside Docker containers for environment parity.

### Full Test Suite (canonical command)
```bash
make dev-test
```
Runs all 19 test packages in Docker with race detector. This is the command to use for all testing.

### Pre-Commit Verification
```bash
make prod-test
```
Builds a clean production container and runs the full suite against it.

### Coverage Summary
```bash
docker compose --profile test run --rm test-runner go test -cover -p 4 ./...
```

### Specific Package
```bash
docker compose --profile test run --rm test-runner go test -race -v ./internal/api/...
docker compose --profile test run --rm test-runner go test -race -v ./internal/platform/macos/...
```

### Static Analysis
```bash
go vet ./...
```

## Test Structure

```
internal/
├── api/
│   ├── handlers_test.go              # Handler unit tests (CRUD, auth, platform)
│   ├── handler_test_helpers_test.go   # Mock repos, test server, helpers
│   ├── health_test.go                # Health endpoint tests
│   ├── server_auth_test.go           # Auth middleware tests
│   ├── compression_test.go           # Compression middleware
│   ├── cors_test.go                  # CORS middleware
│   ├── ratelimit_test.go             # Rate limiting
│   ├── auth_ratelimit_test.go        # Auth-specific rate limiting
│   ├── ip_allowlist_middleware_test.go # IP allowlist
│   ├── request_id_test.go            # Request ID middleware
│   ├── request_size_limit_test.go    # Request size limits
│   ├── timeout_test.go              # Timeout middleware
│   ├── tracing_middleware_test.go    # Tracing middleware
│   └── error_handler_test.go        # Error handling
├── audit/
│   └── audit_test.go                # Async audit logger tests
├── auth/
│   └── auth_test.go                 # OIDC validator, circuit breaker
├── certs/
│   └── certs_test.go                # CA manager, certificate service
├── config/
│   └── config_test.go               # Config loading, validation, secrets
├── db/
│   └── db_test.go                   # Database connection, health
├── models/
│   └── models_test.go               # Model validation
├── platform/
│   ├── android/
│   │   └── service_test.go          # Android service tests
│   ├── macos/
│   │   ├── service_test.go          # macOS service, NanoMDM handler tests
│   │   ├── dep_test.go              # DEP service, sync callback, lifecycle
│   │   └── webhook_test.go          # Checkin/Command webhook handler tests
│   └── windows/                     # Windows platform tests
├── repository/
│   ├── *_test.go                    # Repository unit tests
│   ├── app_integration_test.go      # App repo integration tests
│   ├── compliance_integration_test.go # Compliance repo integration tests
│   ├── group_integration_test.go    # Group repo integration tests
│   ├── policy_assignment_integration_test.go # Policy assignment integration tests
│   ├── policy_version_integration_test.go   # Policy version integration tests
│   └── sprint12_gaps_integration_test.go    # Sprint 1/2 gap coverage
├── service/
│   ├── policy_service_test.go       # Policy service tests
│   ├── group_service_test.go        # Group service tests
│   ├── compliance_service_test.go   # Compliance service tests
│   └── lifecycle_service_test.go    # Lifecycle hooks tests
├── scep/
│   └── scep_test.go                 # SCEP challenge tests
├── tracing/
│   └── tracing_test.go              # Tracing tests
└── validation/
    └── validation_test.go           # JSONB validation, pagination
```

## Test Patterns

### Handler Tests (unit, no infrastructure)

Handler tests use mock repos defined in `handler_test_helpers_test.go`. No database or external services needed.

```go
func TestHandleUpdateDevice(t *testing.T) {
    t.Run("updates device fields", func(t *testing.T) {
        ts := newTestServer(t)
        id := uuid.New()
        ts.deviceRepo.devices = append(ts.deviceRepo.devices, &models.Device{
            BaseModel: models.BaseModel{ID: id},
            Platform:  models.PlatformWindows,
            Name:      "Old",
        })

        body := jsonBody(t, map[string]string{"name": "New Name"})
        req := httptest.NewRequest("PUT", "/api/v1/devices/"+id.String(), body)
        w := ts.do(req)

        assert.Equal(t, http.StatusOK, w.Code)
        assert.Equal(t, "New Name", ts.deviceRepo.devices[0].Name)
    })
}
```

Key helpers:
- `newTestServer(t)` — creates a Server with mock repos, no auth middleware
- `ts.do(req)` — executes request, returns recorder
- `ts.doWithAuth(req, user)` — executes with authenticated user context
- `jsonBody(t, v)` — marshals to JSON reader
- `decodeResponse(t, w)` — decodes standard Response struct

Mock repos have error fields for testing failure paths:
- `mockDeviceRepo{updateErr: fmt.Errorf("db error")}`
- `mockPolicyRepo{assignErr: fmt.Errorf("constraint violation")}`

### Integration Tests

All integration tests run inside Docker via `make dev-test`. No manual service startup needed — the test-runner container depends on PostgreSQL and connects via Docker networking.

```bash
make dev-test     # Runs everything, including integration tests
```

### Platform Tests

Platform tests use testify mocks (`MockDeviceRepository`) for the service layer:

```go
func TestService_CreateDevice(t *testing.T) {
    repo := new(MockDeviceRepository)
    repo.On("Create", mock.Anything, mock.Anything).Return(nil)
    service := NewService(repo)

    device, err := service.CreateDevice(ctx, enterpriseID, "udid", "serial")
    require.NoError(t, err)
    assert.Equal(t, models.PlatformMacOS, device.Platform)
}
```

## Current Coverage (Sprint 4b)

| Package | Coverage | Notes |
|---------|----------|-------|
| apperrors | 100.0% | |
| models | 100.0% | |
| validation | 96.6% | |
| audit | 95.2% | |
| config | 93.1% | |
| scep | 93.3% | |
| tracing | 86.7% | |
| db | 82.4% | With integration tests |
| certs | 78.4% | |
| windows | 69.7% | |
| auth | 68.0% | |
| service | 67.5% | |
| metrics | 65.0% | |
| android | 61.9% | |
| macos | 79.8% | Above target — DEP storage integration tests added |
| api | 56.5% | Below 70% handler target |
| repository | 87.3% | Integration tests need Docker PostgreSQL |

## Coverage Goals

- **Sprint 1**: 35%+ ✅ (achieved)
- **Sprint 2**: 50%+ ✅ (achieved)
- **Sprint 2a**: 60%+ ✅ (achieved — most packages well above)
- **Sprint 3**: 65%+ ✅ (achieved)
- **Sprint 4/4b**: 60%+ ✅ (achieved — 15 of 17 packages above target)
- **Sprint 5**: 70%+ (production ready target)

## Prerequisites

### Local Testing (unit tests only)
```bash
make dev-test         # Runs all tests in Docker
```

### Full Integration Testing
```bash
make dev-test         # All 19 packages, race detector, Docker networking
```

## Tips

- **Always use `-race`** — the race detector catches real bugs
- **Run `go vet ./...`** after changes — catches common mistakes
- **Check mock stubs** before writing handler tests — some mock methods may be no-ops that need to be made functional
- **Grep for existing tests** before changing a function's behavior — fleshing out a stub handler will break tests that send nil/empty bodies
- **New endpoints need test routes** — add them to `newTestServer()` in `handler_test_helpers_test.go`
