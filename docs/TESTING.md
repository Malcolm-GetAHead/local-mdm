# Testing Guide

**Last Updated**: 2026-04-29

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

### Integration Tests (need Docker PostgreSQL)

All integration tests use `testutil.ConnectDB(t)` or `testutil.ConnectRawDB(t)` for database connections. These helpers:
- Read `DB_HOST` / `DB_PASSWORD` env vars (set by Docker Compose) with localhost fallback
- Set pool limits (2 open / 1 idle) to prevent connection exhaustion
- Call `t.Cleanup()` for automatic close — no `defer db.Close()` needed
- Skip the test with `t.Skipf` if the database is unavailable

```go
func TestDeviceRepository_Create(t *testing.T) {
    db := testutil.ConnectDB(t)  // skips if no DB, auto-closes
    repo := NewDeviceRepository(db.Writer, db.Reader)

    device := &Device{Name: "Test"}
    err := repo.Create(context.Background(), device)
    require.NoError(t, err)
}
```

For tests that need a raw `*sql.DB` (reporting, SCEP challenges, token cache):
```go
func TestTokenCache(t *testing.T) {
    db := testutil.ConnectRawDB(t)  // returns *sql.DB, skips if no DB
    cache := NewTokenCache(db)
    // ...
}
```

**Never create database connections inline in test files.** Always use `testutil.ConnectDB(t)` or `testutil.ConnectRawDB(t)`.

### Test Enterprise Cleanup

Every integration test that creates an enterprise **must** register cleanup. Use the shared helper:

```go
enterpriseID := testutil.CreateTestEnterprise(t, db.Writer, "Test Enterprise")
// CASCADE FK deletes all child rows (devices, policies, groups, etc.) on cleanup
```

For tests that create enterprises via the repository layer instead of raw SQL:
```go
err := entRepo.Create(ctx, enterprise)
require.NoError(t, err)
t.Cleanup(func() { db.Writer.Exec("DELETE FROM enterprises WHERE id = $1", enterprise.ID) })
```

**Rules:**
- Always use hard `DELETE FROM enterprises WHERE id = $1` — never `entRepo.Delete()` (that's a soft delete, the row stays)
- Don't use `defer db.Close()` alongside `t.Cleanup()` — `defer` runs before `t.Cleanup()`, closing the connection before cleanup can execute. `testutil.ConnectDB(t)` already registers its own close via `t.Cleanup()`
- Never scope cleanup by enterprise name — always by ID

### File Paths in Tests

`go test` changes the working directory to the package directory. Use these patterns:
- `t.TempDir()` for generated files (CA certs in tests)
- `projectPath(t, "path/from/root")` for project-root files (defined in `tests/e2e/helpers_test.go`)
- Never use bare relative paths like `"internal/api/certs/ca.crt"` in tests

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

## Browser Tests (Playwright)

The dashboard UI is tested with Playwright via a markdown-based playbook DSL.

### Running Browser Tests
```bash
make seed          # Reset test data (run before each test)
make browser-test  # Run all 199 Playwright tests headless
```

For visual debugging:
```bash
cd tests/browser && node run-playbook.js --headed
```

Run a specific section:
```bash
cd tests/browser && node run-playbook.js --section "Groups"
```

### Playbook DSL

Tests are defined in `tests/browser/browser-playbook.md` using a simple DSL:
- `Visit /path` — navigate to URL
- `Navigate to "Link"` — click a sidebar/nav link
- `Fill: field=value` — fill form fields
- `Click "Button"` — click a button or link
- `Select "Option" from "dropdown"` — select dropdown option
- `Verify "text" is visible` / `is not visible` — assert text presence
- `Wait Ns` — pause for timing-sensitive operations

### Key Notes
- Tests run against the live Docker stack (localhost:8080)
- Real Keycloak login — no cookie bypass
- Console errors and HTTP 4xx/5xx are tracked and fail the run
- Seed data enterprise_id differs from Keycloak user — tests create their own data
- Run `make seed` before each test run to clean up mutations from previous runs

## Error State Tests

A separate Playwright test suite (`tests/browser/run-error-tests.js`) validates UI behavior when backend services are unavailable or return errors. These tests verify that error pages, fallback messages, and retry prompts render correctly.

```bash
cd tests/browser && node run-error-tests.js
```

## Combined Coverage

The `make coverage-combined` target merges Go unit/integration coverage with Playwright-driven coverage (collected via `-cover` instrumented binary) into a single report. This gives a realistic picture of total code exercised across both test layers.

```bash
make coverage-combined  # Outputs combined HTML report
```

## Current Coverage (Sprint 6)

> **Note**: Coverage numbers below are from Docker runs (`make dev-test`) where integration tests have access to PostgreSQL and Keycloak. Running locally without Docker shows lower numbers for packages with integration tests (repository, reporting, audit, certs, auth, macos).

| Package | Coverage | Notes |
|---------|----------|-------|
| apperrors | 100.0% | |
| models | 100.0% | |
| metrics | 97.5% | |
| validation | 96.6% | |
| audit | 95.2% | Async logger, structured logging, shutdown |
| config | 91.6% | |
| auth | 90.7% | Keycloak integration tests |
| android | 90.0% | |
| tracing | 86.7% | |
| reporting | 86.0% | |
| windows | 85.2% | |
| db | 82.4% | With integration tests |
| service | 81.0% | |
| certs | 78.0% | |
| macos | 77.9% | |
| repository | 77.9% | |
| scep | 75.9% | |
| api | 58.6% | ⚠️ Below 70% handler target — web handlers need route registration work |

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

## Real Device Testing (Sprint 6)

### Windows Enrollment Testing

Windows 11 devices enroll via Settings UI — no agent or Azure AD required.

**Prerequisites on Windows VM:**
1. Import CA cert: `certutil -addstore Root ca.crt`
2. Add hosts entry: `echo 192.168.1.102 enterpriseenrollment.localmdm.local >> C:\Windows\System32\drivers\etc\hosts`

**Enrollment:** Settings → Accounts → Access work or school → "Enroll only in device management" → `admin@localmdm.local` → any credentials

**Verify:** Device appears in dashboard under Acme Corp, OMA-DM sync updates `last_seen`

### macOS Enrollment Testing

macOS devices enroll via Safari profile download. Reboot triggers NanoMDM check-in → webhook → auto-queue 9 commands.

### Important: Test Database Conflict

Tests and real enrolled devices share the same PostgreSQL database. Test cleanup is now scoped to test-created enterprise IDs (CASCADE deletes child rows), so real device data is no longer broadly deleted. However, the shared database means test failures or interrupted runs could leave orphaned data. **Run tests BEFORE enrolling real devices when possible**, or plan to verify real device data after testing.

### TLS Requirements for Windows

The server cert must have a CRL Distribution Point extension. Without it, Windows schannel rejects TLS connections with `CRYPT_E_NO_REVOCATION_CHECK`. The CRL is served at `http://<server>:8080/crl/ca.crl`.