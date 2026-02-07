# Testing Guide

## Running Tests

### All Tests
```bash
make test
```

### Unit Tests Only
```bash
make test-unit
```

### Integration Tests Only
```bash
make test-integration
```

### Coverage Report
```bash
make test-coverage
# Opens coverage.html in browser
```

### Coverage Summary
```bash
make test-coverage-summary
# Prints: Total coverage: XX.X%
```

## Test Structure

```
internal/
├── auth/
│   └── auth_test.go           # Auth integration tests
├── certs/
│   └── certs_test.go          # Certificate tests
├── repository/
│   └── repository_test.go     # Repository integration tests
└── testutil/
    ├── db.go                  # Database test helpers
    └── helpers.go             # Test data factories
```

## Test Helpers

### Database Setup
```go
import "github.com/malcolm-getahead/local-mdm/internal/testutil"

func TestSomething(t *testing.T) {
    db := testutil.SetupTestDB(t)
    defer testutil.CleanupTestDB(t, db)
    
    // Your test code
}
```

### Test Data Factories
```go
// Create test enterprise
enterprise := testutil.NewTestEnterprise(t)

// Create test device
device := testutil.NewTestDevice(t, enterprise.ID)

// Create test policy
policy := testutil.NewTestPolicy(t, enterprise.ID)
```

### Assertions
```go
// Use testify assertions
testutil.AssertNoError(t, err)
testutil.AssertEqual(t, expected, actual)
testutil.AssertNotNil(t, value)
```

## Prerequisites

### Local Testing
- PostgreSQL running on localhost:5432
- Keycloak running on localhost:8180
- Run `make docker-up` to start services

### CI/CD
- GitHub Actions automatically starts PostgreSQL and Keycloak
- Runs on every push and pull request
- Coverage reports uploaded to Codecov

## Coverage Goals

- **Sprint 1**: 35%+ (foundation code, many stubs)
- **Sprint 2**: 50%+ (platform implementations)
- **Sprint 3**: 60%+ (policy management)
- **Sprint 4**: 70%+ (production ready)

## Current Coverage

| Package | Coverage |
|---------|----------|
| auth | 60.7% |
| certs | 69.4% |
| repository | 53.5% |
| **Total** | **37.5%** |

## Writing Tests

### Integration Test Example
```go
func TestDeviceRepository(t *testing.T) {
    db := testutil.SetupTestDB(t)
    defer testutil.CleanupTestDB(t, db)
    
    repo := repository.NewDeviceRepository(db.DB)
    enterprise := testutil.NewTestEnterprise(t)
    device := testutil.NewTestDevice(t, enterprise.ID)
    
    // Test Create
    err := repo.Create(context.Background(), device)
    testutil.AssertNoError(t, err)
    
    // Test Get
    fetched, err := repo.GetByID(context.Background(), device.ID)
    testutil.AssertNoError(t, err)
    testutil.AssertEqual(t, device.Name, fetched.Name)
}
```

### Unit Test Example (with mocks - future)
```go
func TestDeviceService(t *testing.T) {
    mockRepo := mocks.NewDeviceRepository(t)
    service := NewDeviceService(mockRepo)
    
    // Setup mock expectations
    mockRepo.On("GetByID", mock.Anything, mock.Anything).
        Return(&models.Device{Name: "Test"}, nil)
    
    // Test
    device, err := service.GetDevice(context.Background(), uuid.New())
    testutil.AssertNoError(t, err)
    testutil.AssertEqual(t, "Test", device.Name)
    
    // Verify mock was called
    mockRepo.AssertExpectations(t)
}
```

## Continuous Integration

Tests run automatically on:
- Every push to main/develop
- Every pull request
- Coverage reports generated
- Failures block merges

See `.github/workflows/test.yml` for CI configuration.
