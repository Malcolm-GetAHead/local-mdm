# Development Steering Guide

**Project**: Local MDM  
**Last Updated**: 2026-02-07  
**Purpose**: Guide for AI agents and developers working on this codebase

---

## Core Principles

### 1. Minimal Implementation
- Write only code that directly solves the problem
- Avoid over-engineering and premature optimization
- Don't add features "just in case" - implement when needed

### 2. Test-Driven Development
- Run `go test -race ./...` after EVERY change
- Maintain test coverage > 60% (increase over time)
- Add tests BEFORE fixing bugs when possible

### 3. Design Decisions Respect
- **Local secrets** in `secrets/` directory (dev only, AWS SSM in production)
- **CA keys** on filesystem (dev only, AWS Secrets Manager in production)
- **Docker Compose** for local dev (Kubernetes for production)
- **Basic monitoring** sufficient for dev (advanced for production)

### Credential Storage Strategy
- **Service credentials** (WNS keys, API tokens, service account keys): Store in `secrets/` directory for dev, AWS SSM Parameters in production. Reference via config file paths.
- **Device-bound secrets** (DEP tokens, per-device certificates, enrollment tokens): Encrypt with pgcrypto (`pgp_sym_encrypt`/`pgp_sym_decrypt`) and store in the database. This is the existing pattern for DEP tokens.
- **User-issued secrets** (admin API tokens for CLI/integrations): Store in the database using pgcrypto `crypt()`/`gen_salt()` for hash-based verification. The plaintext token is returned once at creation and never stored. This is the same table (`api_tokens`) already in the schema.
- **Never** store secrets in config YAML values directly — use file paths or environment variables that point to the secret.

### Testing Approach
- **Mock-first**: All platform-specific code (NanoMDM, Android Management API, Windows WNS) should be tested with mock services and simulated device responses. No real devices or external APIs required for unit/integration tests.
- **Real device testing** is a future phase (F-01) — Windows VMs, macOS VMs, Android emulators. Not required for sprint work.
- **Handler tests** use mock repos in `handler_test_helpers_test.go` — no infrastructure needed.
- **Platform tests** use testify mocks for repository interfaces.
- **Integration tests** need Docker services (PostgreSQL, Keycloak) but not real MDM devices.

---

## Development Workflow

### Before Making Changes
```bash
# 1. Ensure tests pass
go test ./...

# 2. Check for race conditions
go test -race ./...

# 3. Verify coverage baseline
go test -cover ./... | grep -v "no test files"
```

### After Making Changes
```bash
# 1. Run tests with race detector
go test -race ./...

# 2. Check coverage improved or maintained
go test -cover ./...

# 3. Verify no vet warnings
go vet ./...

# 4. Run full test suite
make test
```

---

## Code Standards

### Error Handling
```go
// ✅ GOOD - Return errors, add context
func GetDevice(id uuid.UUID) (*Device, error) {
    device, err := repo.GetByID(ctx, id)
    if err != nil {
        return nil, fmt.Errorf("failed to get device %s: %w", id, err)
    }
    return device, nil
}

// ❌ BAD - Panic
func GetDevice(id uuid.UUID) *Device {
    device, err := repo.GetByID(ctx, id)
    if err != nil {
        panic(err)  // NEVER DO THIS
    }
    return device
}
```

### Context Handling
```go
// ✅ GOOD - Check context before expensive operations
func List(ctx context.Context) ([]*Device, error) {
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    default:
    }
    
    // Expensive database query
    rows, err := db.QueryContext(ctx, query)
    // ...
}

// ❌ BAD - Ignore context
func List(ctx context.Context) ([]*Device, error) {
    // No context check
    rows, err := db.Query(query)  // Wrong - use QueryContext
    // ...
}
```

### Concurrency Safety
```go
// ✅ GOOD - Use atomic operations for lock-free reads
type Cache struct {
    data atomic.Pointer[CacheData]
}

func (c *Cache) Get() *CacheData {
    return c.data.Load()  // Lock-free read
}

// ❌ BAD - Check-then-act race condition
type Cache struct {
    data *CacheData
    mu   sync.RWMutex
}

func (c *Cache) Refresh() {
    if time.Since(c.lastUpdate) > interval {  // Race here
        c.update()  // Multiple goroutines can enter
    }
}
```

### Input Validation
```go
// ✅ GOOD - Validate early
func Create(device *Device) error {
    if err := validation.ValidateJSONB(device.PlatformData, 1<<20, 10); err != nil {
        return fmt.Errorf("invalid platform_data: %w", err)
    }
    return repo.Create(ctx, device)
}

// ❌ BAD - No validation
func Create(device *Device) error {
    return repo.Create(ctx, device)  // Accepts anything
}
```

---

## Architecture Guidelines

### Repository Pattern
```go
// ✅ Repositories handle data access only
type DeviceRepository interface {
    Create(ctx context.Context, device *Device) error
    GetByID(ctx context.Context, id uuid.UUID) (*Device, error)
    List(ctx context.Context, enterpriseID uuid.UUID, limit, offset int) ([]*Device, int, error)
}
```

### Service Layer (Sprint 4+)
```go
// ✅ Services handle business logic — reusable from handlers, CLI, background jobs
type PolicyService struct {
    policyRepo  PolicyRepository
    deviceRepo  DeviceRepository
    groupRepo   GroupRepository
    cmdRepo     CommandRepository
    dispatcher  CommandDispatcher
    logger      *slog.Logger
}

// Services do NOT import net/http — they are transport-agnostic
func (s *PolicyService) AssignToGroup(ctx context.Context, policyID, groupID uuid.UUID) error {
    // Multi-step business logic: look up devices, translate, push
}

// ✅ Handlers are thin — parse request, call service, format response
func (s *Server) handleAssignPolicyToGroup(w http.ResponseWriter, r *http.Request) {
    policyID, _ := parseUUIDParam(r, "id")
    groupID, _ := parseUUIDParam(r, "group_id")
    err := s.policyService.AssignToGroup(r.Context(), policyID, groupID)
    // respond
}

// ❌ BAD — business logic in handler (pre-Sprint 4 pattern, don't extend)
func (s *Server) handleAssignPolicyToGroup(w http.ResponseWriter, r *http.Request) {
    // 50 lines of business logic mixed with HTTP concerns
}
```

**Rules:**
- New business logic (Sprint 4+) goes in `internal/service/`
- Existing simple handlers (CRUD, lock/wipe) stay as-is — no refactor for its own sake
- Services accept interfaces via constructor (dependency injection)
- Services never import `net/http`

### Event Bus (Sprint 4+)
- PostgreSQL `LISTEN`/`NOTIFY` for decoupled event-driven communication
- Handlers/services write to DB → PostgreSQL triggers fire `pg_notify` → Go `EventBus` listener dispatches to subscribers
- Subscribers are registered at startup (compliance evaluation, policy deployment, lifecycle hooks, future webhooks)
- Transactional: events only fire on commit
- Multi-instance safe: all server instances receive notifications

### Transaction Usage
```go
// ✅ Use SERIALIZABLE for critical operations
err := transactor.WithTransactionIsolation(ctx, IsolationSerializable, func(txCtx context.Context) error {
    // Create enterprise and first admin atomically
    return nil
})

// ✅ Use READ COMMITTED for simple reads
err := transactor.WithTransactionIsolation(ctx, IsolationReadCommitted, func(txCtx context.Context) error {
    // Read-only operations
    return nil
})
```

### Dependency Injection
```go
// ✅ GOOD - Accept interfaces, return concrete types
func NewServer(cfg *Config, db Database, logger Logger) *Server {
    return &Server{cfg: cfg, db: db, logger: logger}
}

// ❌ BAD - Global state
var globalDB *sql.DB

func GetDevice(id string) *Device {
    return globalDB.Query(...)  // Hard to test
}
```

---

## Testing Standards

### Test Structure
```go
func TestDeviceRepository_Create(t *testing.T) {
    // Setup
    db := testutil.SetupTestDB(t)
    defer db.Close()
    repo := NewDeviceRepository(db)
    
    // Execute
    device := &Device{Name: "Test"}
    err := repo.Create(context.Background(), device)
    
    // Assert
    require.NoError(t, err)
    assert.NotEqual(t, uuid.Nil, device.ID)
    
    // Verify
    fetched, err := repo.GetByID(context.Background(), device.ID)
    require.NoError(t, err)
    assert.Equal(t, device.Name, fetched.Name)
}
```

### Test Coverage Requirements
- **Critical paths**: 100% (auth, transactions, security)
- **Repositories**: 80%+
- **Handlers**: 70%+
- **Utilities**: 60%+
- **Overall**: Maintain or improve current coverage

### Race Condition Testing
```bash
# ALWAYS run with race detector
go test -race ./...

# For specific packages
go test -race -v ./internal/auth/...
go test -race -v ./internal/repository/...
```

---

## Common Patterns

### Constants Over Magic Numbers
```go
// ✅ GOOD
const (
    MaxJSONBSize = 1 << 20  // 1MB
    MaxPageSize  = 1000
    MaxDepth     = 10
)

// ❌ BAD
if len(data) > 1048576 {  // What is this number?
```

### Structured Logging
```go
// ✅ GOOD
logger.Info("Device created",
    "device_id", device.ID,
    "enterprise_id", device.EnterpriseID,
    "platform", device.Platform,
)

// ❌ BAD
log.Printf("Device created: %s", device.ID)  // Unstructured
```

### Error Wrapping
```go
// ✅ GOOD - Preserve error chain
if err != nil {
    return fmt.Errorf("failed to create device: %w", err)
}

// ❌ BAD - Lose error context
if err != nil {
    return fmt.Errorf("failed to create device: %v", err)  // %v not %w
}
```

---

## What NOT to Do

### ❌ Don't Add Without Planning
- Redis caching (Redis was removed in Sprint 4 — use PostgreSQL for all caching)
- Distributed tracing (check monitoring requirements)
- Kubernetes manifests (check deployment plans)
- HSM integration (check security roadmap)
- Advanced monitoring (check observability plans)

### ❌ Don't Change These (Design Decisions)
- Secrets in local directory (intentional for dev)
- CA key storage approach (intentional for dev)
- Docker Compose setup (intentional for dev)
- Basic monitoring (sufficient for dev)

### ❌ Don't Over-Engineer
- Don't build a full JSON schema validator - just check size and depth
- Don't add caching everywhere - add when needed
- Don't create abstractions for single use cases
- Don't add configuration for everything - use sensible defaults

---

## Quick Reference

### File Locations
```
internal/
├── api/          - HTTP handlers and middleware
├── auth/         - OIDC authentication
├── certs/        - Certificate management
├── config/       - Configuration loading
├── db/           - Database connection
├── models/       - Data models
├── platform/     - Platform-specific code (macos/, windows/, android/)
├── repository/   - Data access layer
├── service/      - Business logic layer (Sprint 4+)
├── validation/   - Input validation
└── testutil/     - Test helpers

docs/planning/
├── sprints/              - Sprint plans and task breakdowns
│   ├── sprint-3-platform-features/
│   ├── sprint-4-policy-and-identity/
│   ├── sprint-4b-db-pools/
│   ├── sprint-4c-platform-sso/
│   ├── sprint-5-ui-and-polish/
│   └── sprint-5b-web-dashboard/
└── future/               - Future enhancements (F-01 to F-08)
```

### Key Commands
```bash
# Development
make run                    # Start server
make test                   # Run tests
make test-coverage          # Generate coverage report

# Testing
go test -race ./...         # Race detection (ALWAYS USE)
go test -cover ./...        # Coverage check
go test -v ./internal/...   # Verbose output

# Quality
go vet ./...                # Static analysis
golangci-lint run           # Linting (if installed)
```

### Environment Setup
```bash
# Start dependencies
make docker-up

# Wait for services
sleep 45

# Run migrations
make migrate-up

# Run tests
make test

# Start server
make run
```

---

## Getting Help

### Documentation Structure
- `README.md` - Project overview
- `GETTING_STARTED.md` - Setup guide
- `docs/TESTING.md` - Testing guide
- `docs/SECURITY.md` - Security guidelines
- `docs/tasks/` - Task tracking and planning
- `docs/tasks/future/` - Future enhancements roadmap

### Finding Information
- **Current work**: Check `docs/tasks/` for active tasks
- **Future plans**: Check `docs/tasks/future/` for roadmap
- **Code reviews**: Check task directories for review findings
- **Architecture**: Check `docs/architecture/` for design docs

---

## Decision Log Template

When making significant decisions, document them here:

### YYYY-MM-DD: Decision Title
- **Decision**: What was decided
- **Rationale**: Why this decision was made
- **Impact**: What this affects
- **Alternatives Considered**: What else was considered

---

## Version History

- **2026-02-07**: Initial steering guide created

---

**Remember**: Keep it simple, test thoroughly, respect design decisions, and consult documentation before major changes.
