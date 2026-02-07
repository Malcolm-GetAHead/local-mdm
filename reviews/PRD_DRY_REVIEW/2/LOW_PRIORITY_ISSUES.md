# Low Priority Issues (Technical Debt)

**Priority**: LOW  
**Total Issues**: 7  
**Estimated Effort**: 2-3 days  
**Risk Level**: Code quality, future improvements

---

## L-01: Inconsistent Error Wrapping
**Severity**: LOW | **Category**: Code Quality | **Effort**: 0.5 days

Some errors use `%w` (correct), others use `%v` (loses error chain).

**Fix**: Audit all error returns and use `%w` consistently.

---

## L-02: Missing Code Comments on Public Functions
**Severity**: LOW | **Category**: Maintainability | **Effort**: 0.5 days

Many exported functions lack godoc comments.

**Fix**: Add comments to all exported functions.

```go
// GetByID retrieves a device by its unique identifier.
// Returns ErrNotFound if the device does not exist or has been deleted.
func (r *deviceRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Device, error) {
```

---

## L-03: No Structured Logging in Some Packages
**Severity**: LOW | **Category**: Observability | **Effort**: 0.25 days

Some packages use `fmt.Printf` instead of structured logging.

**Fix**: Replace all `fmt.Printf` with `logger.Info/Warn/Error`.

---

## L-04: Magic Numbers in Code
**Severity**: LOW | **Category**: Maintainability | **Effort**: 0.25 days

Some constants are hardcoded (e.g., `1 << 20` for 1MB).

**Fix**: Define named constants.

```go
const (
    MaxRequestBodySize = 1 << 20  // 1MB
    MaxJSONBSize       = 1 << 20  // 1MB
    MaxJWKSSize        = 1 << 20  // 1MB
)
```

---

## L-05: No Benchmark Tests
**Severity**: LOW | **Category**: Performance | **Effort**: 0.5 days

No benchmark tests to track performance regressions.

**Fix**: Add benchmark tests for critical paths.

```go
func BenchmarkDeviceList(b *testing.B) {
    db := setupTestDB(b)
    repo := NewDeviceRepository(db)
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, _, _ = repo.List(context.Background(), enterpriseID, 100, 0)
    }
}
```

---

## L-06: Duplicate Code in Repository Methods
**Severity**: LOW | **Category**: Maintainability | **Effort**: 0.5 days

All repository List methods have identical pagination logic.

**Fix**: Extract common pagination helper.

```go
func executePaginatedQuery[T any](
    ctx context.Context,
    exec executor,
    countQuery string,
    dataQuery string,
    scanFn func(*sql.Rows) (T, error),
    args ...interface{},
) ([]T, int, error) {
    // Common pagination logic
}
```

---

## L-07: No Linter Configuration
**Severity**: LOW | **Category**: Code Quality | **Effort**: 0.25 days

No golangci-lint configuration for consistent code quality.

**Fix**: Add `.golangci.yml`.

```yaml
linters:
  enable:
    - errcheck
    - gosimple
    - govet
    - ineffassign
    - staticcheck
    - unused
    - gocyclo
    - gofmt
    - misspell
    - gocritic

linters-settings:
  gocyclo:
    min-complexity: 15
  gocritic:
    enabled-tags:
      - diagnostic
      - performance
      - style
```
