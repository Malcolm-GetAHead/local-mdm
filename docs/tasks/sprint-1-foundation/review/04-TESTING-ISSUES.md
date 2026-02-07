# Testing Issues - Sprint 1 Code Review

**Priority**: 🟠 HIGH  
**Impact**: Quality, Reliability, Maintainability

---

## Overview

Current test coverage is **45.8%** overall, with significant gaps in critical areas. Tests exist but have quality issues that reduce their effectiveness.

### Coverage by Package

| Package | Coverage | Status | Issues |
|---------|----------|--------|--------|
| config | 93.1% | ✅ Excellent | None |
| validation | 95.0% | ✅ Excellent | None |
| repository | 81.1% | ✅ Good | Integration only |
| certs | 69.4% | ✅ Good | No error paths |
| auth | 60.7% | ⚠️ Acceptable | Depends on Keycloak |
| api | 0.0% | 🔴 Critical | No tests |
| db | 0.0% | 🔴 Critical | No tests |
| cmd/server | 0.0% | 🔴 Critical | No tests |

---

## Critical Testing Issues

### 1. No API Handler Tests (0% Coverage)

**Impact**: Cannot verify API behavior, regressions will go undetected

**Problem**:
```go
// internal/api/handlers.go has 0% coverage
func (s *Server) handleListDevices(w http.ResponseWriter, r *http.Request) {
    respondNotImplemented(w, r)  // No tests verify this
}
```

**Fix**:
```go
// internal/api/handlers_test.go
func TestHandleListDevices(t *testing.T) {
    // Setup
    server := setupTestServer(t)
    token := getTestToken(t)
    
    // Test authenticated request
    req := httptest.NewRequest("GET", "/api/v1/devices", nil)
    req.Header.Set("Authorization", "Bearer "+token)
    w := httptest.NewRecorder()
    
    server.router.ServeHTTP(w, req)
    
    assert.Equal(t, http.StatusNotImplemented, w.Code)
}

func TestHandleListDevices_Unauthorized(t *testing.T) {
    server := setupTestServer(t)
    
    req := httptest.NewRequest("GET", "/api/v1/devices", nil)
    w := httptest.NewRecorder()
    
    server.router.ServeHTTP(w, req)
    
    assert.Equal(t, http.StatusUnauthorized, w.Code)
}
```

---

### 2. Tests Depend on External Services

**Impact**: Tests fail if Keycloak or PostgreSQL not running, slow test execution

**Problem**:
```go
// Tests require running services
func TestKeycloakLogin(t *testing.T) {
    kc := auth.NewKeycloakClient(
        "http://localhost:8180/realms/localmdm",  // Must be running!
        "localmdm-api",
        "localmdm-api-secret",
    )
    
    tokenResp, err := kc.Login("admin", "admin123")
    // Fails if Keycloak not running
}
```

**Fix**:
```go
// Use httptest to mock Keycloak
func TestKeycloakLogin(t *testing.T) {
    // Mock Keycloak server
    mockKeycloak := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.URL.Path == "/realms/localmdm/protocol/openid-connect/token" {
            json.NewEncoder(w).Encode(map[string]interface{}{
                "access_token":  "mock_token",
                "refresh_token": "mock_refresh",
                "expires_in":    3600,
                "token_type":    "Bearer",
            })
            return
        }
        w.WriteHeader(http.StatusNotFound)
    }))
    defer mockKeycloak.Close()
    
    kc := auth.NewKeycloakClient(
        mockKeycloak.URL+"/realms/localmdm",
        "test-client",
        "test-secret",
    )
    
    tokenResp, err := kc.Login("admin", "admin123")
    assert.NoError(t, err)
    assert.Equal(t, "mock_token", tokenResp.AccessToken)
}
```

---

### 3. No Negative Test Cases

**Impact**: Error paths untested, bugs in error handling

**Problem**:
```go
// Only happy path tested
func TestDeviceRepository(t *testing.T) {
    device := &models.Device{...}
    err := repo.Create(ctx, device)
    assert.NoError(t, err)  // What if it fails?
}
```

**Fix**:
```go
func TestDeviceRepository_Create_DuplicateSerial(t *testing.T) {
    device1 := &models.Device{
        EnterpriseID: enterpriseID,
        SerialNumber: "DUPLICATE123",
    }
    err := repo.Create(ctx, device1)
    assert.NoError(t, err)
    
    // Try to create duplicate
    device2 := &models.Device{
        EnterpriseID: enterpriseID,
        SerialNumber: "DUPLICATE123",  // Same serial
    }
    err = repo.Create(ctx, device2)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "duplicate")
}

func TestDeviceRepository_Create_InvalidEnterpriseID(t *testing.T) {
    device := &models.Device{
        EnterpriseID: uuid.New(),  // Non-existent enterprise
        SerialNumber: "TEST123",
    }
    err := repo.Create(ctx, device)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "foreign key")
}

func TestDeviceRepository_GetByID_NotFound(t *testing.T) {
    device, err := repo.GetByID(ctx, uuid.New())
    assert.Error(t, err)
    assert.Nil(t, device)
}

func TestDeviceRepository_GetByID_Deleted(t *testing.T) {
    device := createTestDevice(t)
    repo.Delete(ctx, device.ID)
    
    // Should not return deleted device
    found, err := repo.GetByID(ctx, device.ID)
    assert.Error(t, err)
    assert.Nil(t, found)
}
```

---

### 4. Tests Not Isolated (Shared Database)

**Impact**: Tests interfere with each other, flaky tests

**Problem**:
```go
func TestEnterpriseRepository(t *testing.T) {
    // Creates enterprise in shared database
    enterprise := &models.Enterprise{Name: "Test"}
    repo.Create(ctx, enterprise)
    
    // Never cleaned up!
}

func TestDeviceRepository(t *testing.T) {
    // Sees enterprises from previous test
    enterprises, _, _ := enterpriseRepo.List(ctx, 10, 0)
    // Count is unpredictable!
}
```

**Fix**:
```go
func TestEnterpriseRepository(t *testing.T) {
    // Use transaction that rolls back
    tx, err := db.Begin()
    require.NoError(t, err)
    defer tx.Rollback()
    
    repo := repository.NewEnterpriseRepository(tx)
    
    enterprise := &models.Enterprise{Name: "Test"}
    err = repo.Create(ctx, enterprise)
    assert.NoError(t, err)
    
    // Transaction rolls back automatically
}

// Or use test database per test
func setupTestDB(t *testing.T) *sql.DB {
    dbName := "test_" + uuid.New().String()
    
    // Create test database
    adminDB, _ := sql.Open("postgres", "postgres://postgres:postgres@localhost/postgres")
    adminDB.Exec("CREATE DATABASE " + dbName)
    
    // Connect to test database
    testDB, _ := sql.Open("postgres", "postgres://postgres:postgres@localhost/"+dbName)
    
    // Run migrations
    migrate.Up(testDB)
    
    // Cleanup on test end
    t.Cleanup(func() {
        testDB.Close()
        adminDB.Exec("DROP DATABASE " + dbName)
        adminDB.Close()
    })
    
    return testDB
}
```

---

### 5. No Boundary Condition Testing

**Impact**: Edge cases cause crashes

**Missing Tests**:
```go
// Test limits
func TestDeviceRepository_List_EmptyResult(t *testing.T) {
    devices, total, err := repo.List(ctx, enterpriseID, 10, 0)
    assert.NoError(t, err)
    assert.Equal(t, 0, total)
    assert.Empty(t, devices)
}

func TestDeviceRepository_List_LargeOffset(t *testing.T) {
    devices, total, err := repo.List(ctx, enterpriseID, 10, 1000000)
    assert.NoError(t, err)
    assert.Empty(t, devices)
}

func TestDeviceRepository_List_NegativeLimit(t *testing.T) {
    devices, total, err := repo.List(ctx, enterpriseID, -1, 0)
    assert.Error(t, err)
}

func TestDeviceRepository_List_ZeroLimit(t *testing.T) {
    devices, total, err := repo.List(ctx, enterpriseID, 0, 0)
    assert.NoError(t, err)
    assert.Empty(t, devices)
}

// Test string limits
func TestDeviceRepository_Create_LongName(t *testing.T) {
    device := &models.Device{
        Name: strings.Repeat("A", 256),  // Exceeds VARCHAR(255)
    }
    err := repo.Create(ctx, device)
    assert.Error(t, err)
}

// Test null handling
func TestDeviceRepository_Create_NullOptionalFields(t *testing.T) {
    device := &models.Device{
        EnterpriseID: enterpriseID,
        Platform:     "macos",
        DeviceID:     "test",
        // Name, Model, OSVersion are null
    }
    err := repo.Create(ctx, device)
    assert.NoError(t, err)
}
```

---

### 6. No Concurrent Access Testing

**Impact**: Race conditions in production

**Missing Tests**:
```go
func TestDeviceRepository_ConcurrentCreate(t *testing.T) {
    var wg sync.WaitGroup
    errors := make(chan error, 10)
    
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func(n int) {
            defer wg.Done()
            device := &models.Device{
                EnterpriseID: enterpriseID,
                SerialNumber: fmt.Sprintf("SERIAL%d", n),
            }
            if err := repo.Create(ctx, device); err != nil {
                errors <- err
            }
        }(i)
    }
    
    wg.Wait()
    close(errors)
    
    // Should have no errors
    for err := range errors {
        t.Errorf("Concurrent create failed: %v", err)
    }
    
    // Verify all devices created
    devices, total, _ := repo.List(ctx, enterpriseID, 100, 0)
    assert.Equal(t, 10, total)
    assert.Len(t, devices, 10)
}

func TestDeviceRepository_ConcurrentUpdate(t *testing.T) {
    device := createTestDevice(t)
    
    var wg sync.WaitGroup
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func(n int) {
            defer wg.Done()
            device.Name = fmt.Sprintf("Name%d", n)
            repo.Update(ctx, device)
        }(i)
    }
    
    wg.Wait()
    
    // Verify device still valid (no corruption)
    updated, err := repo.GetByID(ctx, device.ID)
    assert.NoError(t, err)
    assert.NotEmpty(t, updated.Name)
}
```

---

### 7. No Timeout Testing

**Impact**: Hanging operations not detected

**Missing Tests**:
```go
func TestDeviceRepository_GetByID_Timeout(t *testing.T) {
    // Create context with short timeout
    ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
    defer cancel()
    
    // Simulate slow query
    time.Sleep(10 * time.Millisecond)
    
    device, err := repo.GetByID(ctx, uuid.New())
    assert.Error(t, err)
    assert.True(t, errors.Is(err, context.DeadlineExceeded))
    assert.Nil(t, device)
}

func TestAPI_RequestTimeout(t *testing.T) {
    server := setupTestServer(t)
    
    // Handler that takes too long
    server.router.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
        time.Sleep(35 * time.Second)  // Exceeds 30s timeout
        w.WriteHeader(http.StatusOK)
    })
    
    req := httptest.NewRequest("GET", "/slow", nil)
    w := httptest.NewRecorder()
    
    server.router.ServeHTTP(w, req)
    
    assert.Equal(t, http.StatusGatewayTimeout, w.Code)
}
```

---

### 8. No Performance Benchmarks

**Impact**: Performance regressions not detected

**Missing Tests**:
```go
func BenchmarkDeviceRepository_Create(b *testing.B) {
    repo := setupTestRepo(b)
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        device := &models.Device{
            EnterpriseID: enterpriseID,
            SerialNumber: fmt.Sprintf("BENCH%d", i),
        }
        repo.Create(context.Background(), device)
    }
}

func BenchmarkDeviceRepository_List(b *testing.B) {
    repo := setupTestRepo(b)
    
    // Create test data
    for i := 0; i < 1000; i++ {
        device := &models.Device{
            EnterpriseID: enterpriseID,
            SerialNumber: fmt.Sprintf("BENCH%d", i),
        }
        repo.Create(context.Background(), device)
    }
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        repo.List(context.Background(), enterpriseID, 100, 0)
    }
}

func BenchmarkAPI_ListDevices(b *testing.B) {
    server := setupTestServer(b)
    token := getTestToken(b)
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        req := httptest.NewRequest("GET", "/api/v1/devices", nil)
        req.Header.Set("Authorization", "Bearer "+token)
        w := httptest.NewRecorder()
        
        server.router.ServeHTTP(w, req)
    }
}
```

---

## Test Quality Improvements Needed

### 1. Add Table-Driven Tests

**Current**:
```go
func TestSanitizeHTML(t *testing.T) {
    result := SanitizeHTML("<script>alert('xss')</script>")
    assert.Equal(t, "&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;", result)
}
```

**Better**:
```go
func TestSanitizeHTML(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
    }{
        {"plain text", "hello", "hello"},
        {"script tag", "<script>alert('xss')</script>", "&lt;script&gt;..."},
        {"img tag", "<img src=x onerror=alert(1)>", "&lt;img..."},
        {"empty", "", ""},
        {"null bytes", "test\x00test", "testtest"},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := SanitizeHTML(tt.input)
            assert.Equal(t, tt.expected, result)
        })
    }
}
```

### 2. Add Test Helpers

**Create**:
```go
// internal/testutil/fixtures.go
func CreateTestEnterprise(t *testing.T, db *sql.DB) *models.Enterprise {
    t.Helper()
    enterprise := &models.Enterprise{
        Name: "Test Enterprise " + uuid.New().String()[:8],
        Slug: "test-" + uuid.New().String()[:8],
    }
    repo := repository.NewEnterpriseRepository(db)
    err := repo.Create(context.Background(), enterprise)
    require.NoError(t, err)
    return enterprise
}

func CreateTestDevice(t *testing.T, db *sql.DB, enterpriseID uuid.UUID) *models.Device {
    t.Helper()
    device := &models.Device{
        EnterpriseID: enterpriseID,
        Platform:     "macos",
        DeviceID:     "test-" + uuid.New().String(),
        SerialNumber: "SN" + uuid.New().String()[:8],
        Name:         "Test Device",
        Status:       "enrolled",
    }
    repo := repository.NewDeviceRepository(db)
    err := repo.Create(context.Background(), device)
    require.NoError(t, err)
    return device
}
```

### 3. Add Integration Test Suite

**Create**:
```go
// tests/integration/enrollment_test.go
func TestEnrollmentFlow(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }
    
    // Start test server
    server := startTestServer(t)
    defer server.Stop()
    
    // Create enterprise
    enterprise := createEnterprise(t, server)
    
    // Enroll device
    device := enrollDevice(t, server, enterprise.ID)
    
    // Verify device in database
    found := getDevice(t, server, device.ID)
    assert.Equal(t, device.ID, found.ID)
    
    // Verify certificate issued
    cert := getCertificate(t, server, device.ID)
    assert.NotNil(t, cert)
    
    // Verify audit log
    logs := getAuditLogs(t, server, enterprise.ID)
    assert.Contains(t, logs, "device.enroll")
}
```

---

## Recommended Test Structure

```
tests/
├── unit/                    # Fast, isolated unit tests
│   ├── repository/
│   ├── validation/
│   └── config/
├── integration/             # Tests with database
│   ├── api/
│   ├── enrollment/
│   └── policy/
├── e2e/                     # End-to-end tests
│   ├── enrollment_flow_test.go
│   └── policy_flow_test.go
└── fixtures/                # Test data
    ├── enterprises.json
    ├── devices.json
    └── policies.json
```

---

## Summary

**Current State**: 45.8% coverage, quality issues  
**Target State**: 80%+ coverage, high quality  
**Estimated Effort**: 20-30 hours

**Priority Fixes**:
1. Add API handler tests (8-10h)
2. Mock external dependencies (6-8h)
3. Add negative test cases (4-6h)
4. Isolate tests (4-6h)
5. Add boundary tests (2-3h)
6. Add concurrent tests (2-3h)
7. Add benchmarks (2-3h)
