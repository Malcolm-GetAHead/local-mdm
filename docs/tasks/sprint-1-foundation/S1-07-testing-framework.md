# S1-07: Testing Framework Setup

**Sprint**: 1 — Foundation
**Parallel**: ✅ Yes (can start immediately)
**Effort**: 1-2 days

## Objective

Establish testing infrastructure, frameworks, and CI/CD integration for unit, integration, and E2E tests.

## Tasks

### 1. Unit Test Framework
- Install testify for assertions and test suites
- Install mockery for generating mocks from interfaces
- Create test helpers package
- Files: `internal/testutil/helpers.go`, `Makefile` (test targets)

### 2. Integration Test Setup
- Test database setup (separate from dev database)
- Database fixtures and seed data
- Transaction rollback per test
- Files: `internal/testutil/db.go`, `testdata/fixtures/`

### 3. Mock Generation
- Configure mockery for all repository interfaces
- Configure mockery for all service interfaces
- Generate initial mocks
- Files: `.mockery.yaml`, `internal/mocks/` (generated)

### 4. Test Coverage Enforcement
- Configure coverage reporting
- Set minimum coverage threshold (70%)
- Coverage report generation (HTML, JSON)
- Files: `Makefile`, `.github/workflows/test.yml`

### 5. CI/CD Integration
- GitHub Actions workflow for tests
- Run tests on PR
- Coverage report as PR comment
- Files: `.github/workflows/test.yml`, `.github/workflows/coverage.yml`

## Dependencies

```bash
go get github.com/stretchr/testify
go install github.com/vektra/mockery/v2@latest
```

## Makefile Targets

```makefile
.PHONY: test test-unit test-integration test-coverage mocks

test: test-unit test-integration

test-unit:
	go test -v -race -short ./...

test-integration:
	go test -v -race -run Integration ./...

test-coverage:
	go test -v -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -html=coverage.out -o coverage.html
	go tool cover -func=coverage.out

mocks:
	mockery --all --dir internal/repository --output internal/mocks/repository
	mockery --all --dir internal/service --output internal/mocks/service
```

## Test Structure

```
internal/
├── repository/
│   ├── device.go
│   └── device_test.go          # Unit tests
├── service/
│   ├── device.go
│   └── device_test.go          # Unit tests with mocked repos
├── api/
│   ├── handlers.go
│   └── handlers_test.go        # Integration tests
├── testutil/
│   ├── helpers.go              # Test helpers
│   ├── db.go                   # Test DB setup
│   └── fixtures.go             # Test data builders
└── mocks/                      # Generated mocks
    ├── repository/
    └── service/
```

## Test Naming Conventions

- Unit tests: `TestFunctionName`
- Integration tests: `TestIntegration_FunctionName`
- Table-driven tests: `TestFunctionName_Cases`

## Example Test

```go
func TestDeviceService_Create(t *testing.T) {
    // Arrange
    mockRepo := mocks.NewDeviceRepository(t)
    service := service.NewDeviceService(mockRepo)
    device := &models.Device{Name: "Test Device"}
    
    mockRepo.On("Create", mock.Anything, device).Return(nil)
    
    // Act
    err := service.Create(context.Background(), device)
    
    // Assert
    assert.NoError(t, err)
    mockRepo.AssertExpectations(t)
}
```

## Integration Test Database

```go
// internal/testutil/db.go
func SetupTestDB(t *testing.T) *sql.DB {
    db := connectToTestDB()
    runMigrations(db)
    t.Cleanup(func() { cleanupTestDB(db) })
    return db
}
```

## Acceptance Criteria

- [ ] `make test` runs all tests successfully
- [ ] `make test-coverage` generates coverage report
- [ ] `make mocks` generates mocks for all interfaces
- [ ] Coverage threshold enforced (70% minimum)
- [ ] CI/CD runs tests on every PR
- [ ] Test database isolated from development database

## Future Enhancements

- E2E tests with real VMs (Sprint 2+)
- Load testing framework (Sprint 5)
- Chaos engineering tests (post-v1.0)
