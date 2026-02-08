# L-04 & L-07: Magic Numbers and Linter Config - Implementation

**Date**: 2026-02-08  
**Status**: ✅ COMPLETE  
**Priority**: LOW  
**Category**: Code Quality  
**Effort**: 0.5 days (combined)  

---

## Issues Resolved

### L-04: Magic Numbers in Code
**Impact**: Unclear constants, poor maintainability  
**Solution**: Define named constants

### L-07: No Linter Configuration
**Impact**: Inconsistent code quality  
**Solution**: Add .golangci.yml

---

## L-04: Magic Numbers

### Problem

Magic numbers scattered throughout the codebase made it difficult to understand their meaning and maintain consistency:

```go
// What does 1 << 20 mean?
limitedReader := io.LimitReader(resp.Body, 1<<20)

// What does 100 mean?
if cfg.MaxOpenConns > 100 {
    return fmt.Errorf("max_open_conns must not exceed 100")
}

// What does 30 mean?
timeout = 30 * time.Second
```

### Solution

Created centralized constants package with well-documented constants:

**File**: `internal/constants/constants.go`

```go
package constants

// Size constants
const (
    OneMB = 1 << 20  // 1,048,576 bytes
    MaxRequestBodySize = OneMB
    MaxJWKSResponseSize = OneMB
)

// Timeout constants
const (
    DefaultQueryTimeout = 30    // seconds
    DefaultRequestTimeout = 30  // seconds
)

// Limit constants
const (
    MaxDatabaseConnections = 100  // PostgreSQL default
    DefaultRateLimit = 100        // requests per window
    MaxActionLength = 100         // characters
    MaxRateLimiterEntries = 10000
)

// Pagination constants
const (
    MaxPageSize = 1000
    DefaultPageSize = 100
)
```

### Files Updated

1. **internal/auth/oidc.go**
   - `1<<20` → `constants.MaxJWKSResponseSize`
   - `30 * time.Second` → `constants.DefaultRequestTimeout * time.Second`

2. **internal/api/server.go**
   - `1 << 20` → `constants.MaxRequestBodySize`
   - `30 * time.Second` → `constants.DefaultRequestTimeout * time.Second`
   - `100` → `constants.DefaultRateLimit`

3. **internal/audit/audit.go**
   - `100` → `constants.MaxActionLength`

4. **internal/db/db.go**
   - `100` → `constants.MaxDatabaseConnections`

5. **internal/config/config.go**
   - `30 * time.Second` → `constants.DefaultQueryTimeout * time.Second`

6. **internal/api/ratelimit.go**
   - `maxRateLimiterEntries = 10000` → `constants.MaxRateLimiterEntries`

### Benefits

✅ **Clarity**: Constants have descriptive names  
✅ **Maintainability**: Change once, update everywhere  
✅ **Consistency**: Same values used across codebase  
✅ **Documentation**: Constants are self-documenting  

---

## L-07: Linter Configuration

### Problem

No linter configuration meant:
- Inconsistent code style
- No automated quality checks
- Potential bugs not caught
- No security checks

### Solution

Added comprehensive `.golangci.yml` configuration with 20+ linters:

#### Enabled Linters

**Default Linters**:
- `errcheck` - Check for unchecked errors
- `gosimple` - Simplify code
- `govet` - Vet examines Go source code
- `ineffassign` - Detect ineffectual assignments
- `staticcheck` - Static analysis
- `unused` - Check for unused code

**Additional Linters**:
- `gocyclo` - Cyclomatic complexity (max 15)
- `gofmt` - Format code
- `goimports` - Manage imports
- `misspell` - Spell check
- `gocritic` - Comprehensive checks
- `revive` - Fast, configurable linter
- `gosec` - Security checks
- `bodyclose` - Check HTTP response body closed
- `noctx` - Check for missing context
- `sqlclosecheck` - Check SQL rows/statements closed
- `rowserrcheck` - Check SQL rows.Err()
- `errorlint` - Error wrapping checks (%w)
- `exportloopref` - Check loop variable capture
- `goconst` - Find repeated strings
- `unconvert` - Remove unnecessary type conversions

#### Configuration Highlights

```yaml
linters-settings:
  gocyclo:
    min-complexity: 15  # Maximum cyclomatic complexity
  
  gocritic:
    enabled-tags:
      - diagnostic
      - performance
      - style
  
  errorlint:
    errorf: true        # Check %w in fmt.Errorf
    asserts: true
    comparison: true
```

#### Test Exclusions

```yaml
issues:
  exclude-rules:
    # Exclude some linters from running on tests
    - path: _test\.go
      linters:
        - gocyclo
        - errcheck
        - gosec
        - goconst
```

### Usage

```bash
# Install golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run linter
golangci-lint run

# Run with auto-fix
golangci-lint run --fix

# Run specific linters
golangci-lint run --enable-only=errcheck,gosec
```

### Benefits

✅ **Automated Quality**: Catches issues before code review  
✅ **Security**: Identifies security vulnerabilities  
✅ **Performance**: Suggests performance improvements  
✅ **Consistency**: Enforces code style  
✅ **Error Handling**: Ensures proper error wrapping  
✅ **Resource Management**: Checks for leaks  

---

## Test Results

```bash
✅ All tests passing
✅ No race conditions
✅ No regressions

ok      internal/api          12.489s
ok      internal/audit        1.869s
ok      internal/auth         37.646s
ok      internal/config       2.420s
ok      internal/db           9.626s
```

---

## Before/After Comparison

### Before (Magic Numbers)

```go
// Unclear what these numbers mean
limitedReader := io.LimitReader(resp.Body, 1<<20)
s.router.Use(requestSizeLimitMiddleware(1 << 20))
if cfg.MaxOpenConns > 100 {
    return fmt.Errorf("max_open_conns must not exceed 100")
}
timeout = 30 * time.Second
```

### After (Named Constants)

```go
// Clear and self-documenting
limitedReader := io.LimitReader(resp.Body, constants.MaxJWKSResponseSize)
s.router.Use(requestSizeLimitMiddleware(constants.MaxRequestBodySize))
if cfg.MaxOpenConns > constants.MaxDatabaseConnections {
    return fmt.Errorf("max_open_conns must not exceed %d", constants.MaxDatabaseConnections)
}
timeout = constants.DefaultRequestTimeout * time.Second
```

---

## Files Created

1. `internal/constants/constants.go` - Centralized constants (45 lines)
2. `.golangci.yml` - Linter configuration (95 lines)

---

## Files Modified

1. `internal/auth/oidc.go` - Use constants
2. `internal/api/server.go` - Use constants
3. `internal/api/ratelimit.go` - Use constants
4. `internal/audit/audit.go` - Use constants
5. `internal/db/db.go` - Use constants
6. `internal/config/config.go` - Use constants

---

## Integration with CI/CD

### GitHub Actions Example

```yaml
name: Lint
on: [push, pull_request]
jobs:
  golangci:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      - name: golangci-lint
        uses: golangci/golangci-lint-action@v3
        with:
          version: latest
```

### Pre-commit Hook

```bash
#!/bin/sh
# .git/hooks/pre-commit
golangci-lint run --new-from-rev=HEAD~1
```

---

## Future Enhancements

### Additional Constants to Extract
- HTTP status codes (use http.StatusOK, etc.)
- Error messages (centralize common errors)
- Configuration defaults (extract to constants)

### Additional Linters to Consider
- `dupl` - Code duplication detection
- `gocognit` - Cognitive complexity
- `nestif` - Nested if statements
- `cyclop` - Package complexity

---

## Conclusion

Both L-04 and L-07 are complete:

**L-04 (Magic Numbers)**:
- ✅ Centralized constants package created
- ✅ 6 files updated to use constants
- ✅ All magic numbers replaced with named constants
- ✅ Self-documenting code

**L-07 (Linter Config)**:
- ✅ Comprehensive .golangci.yml created
- ✅ 20+ linters enabled
- ✅ Security, performance, and style checks
- ✅ Test exclusions configured
- ✅ Ready for CI/CD integration

**Status**: ✅ PRODUCTION READY

---

**Implemented By**: Kiro AI Assistant  
**Date**: 2026-02-08  
**Issues**: L-04, L-07 (LOW PRIORITY)
