# Development Steering Guide

**Project**: Local MDM  
**Last Updated**: 2026-04-20  
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
- **Docker Compose** for local dev (ECS Fargate for production, Kubernetes as alternative)
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
- **Real device testing** uses UTM VMs (macOS 26, Windows 11 ARM). VMs have passwordless SSH. Enrollment requires GUI interaction (macOS: Safari profile download, Windows: Settings → "Enroll only in device management"). Always restore from template snapshot before testing. See `docs/planning/sprints/sprint-6-real-device-integration/GAPS.md` for VM details and current state.

---

## Development Workflow

### Before Making Changes
```bash
# Establish green baseline
make dev-test
```

### After Every Sub-Task
```bash
# 1. Unit/mock tests with race detector
go test -race ./...

# 2. Rebuild container (embedded templates/static files require this)
docker compose build localmdm && docker compose up -d localmdm

# 3. Hit the real endpoint — curl the API or open the dashboard page.
#    go test passing does NOT mean the feature works.
```

### After All Sub-Tasks Complete
```bash
# 1. Full integration test suite in Docker
make dev-test

# 2. Playwright browser tests (if UI changes)
cd tests/browser && node run-playbook.js

# 3. Sanity check the changeset
git diff main --stat

# 4. Verify no vet warnings
go vet ./...
```

See `docs/planning/future/autonomous_sessions.md` §Mandatory Verification Gates for the
full checklist including test requirements, test data cleanup rules, and Playwright conventions.

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
    repo := NewDeviceRepository(db.Writer, db.Reader)
    
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

Measured via `make coverage-combined` (merged Go + Playwright coverage):

- **Critical paths**: 92%+ (auth, audit, transactions, security)
- **Repositories**: 80%+
- **Handlers**: 70%+ (merged — Go-only undercounts `api` because Playwright covers web handlers)
- **Utilities**: 85%+
- **Overall**: 75%+

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
├── apperrors/    - Structured application errors
├── audit/        - Async audit logging
├── auth/         - OIDC authentication
├── certs/        - Certificate management
├── config/       - Configuration loading
├── constants/    - Shared constants
├── db/           - Database connection (Writer/Reader pools)
├── logging/      - Structured logging helpers
├── metrics/      - Prometheus metrics server
├── models/       - Data models
├── platform/     - Platform-specific code (macos/, windows/, android/)
├── repository/   - Data access layer
├── scep/         - SCEP challenge management
├── service/      - Business logic layer (Sprint 4+)
├── tracing/      - Request tracing
├── validation/   - Input validation
└── testutil/     - Test helpers

docs/planning/
├── sprints/              - Sprint plans and task breakdowns
│   ├── sprint-1-foundation/
│   ├── sprint-2-platform-core/
│   ├── sprint-2a-gap-closure/
│   ├── sprint-3-platform-features/
│   ├── sprint-4-policy-and-identity/
│   ├── sprint-4b-db-pools/
│   ├── sprint-5-ui-and-polish/
│   ├── sprint-5b-eventbus/
│   ├── sprint-5c-platform-integration/
│   ├── sprint-5d-web-dashboard/
│   ├── sprint-5e-cert-verification/
│   ├── sprint-5f-api-hardening/
│   ├── sprint-5g-quality-polish/
│   ├── sprint-6-real-device-integration/
│   └── sprint-7-platform-sso/
└── future/               - Future enhancements (F-01 to F-08)
```

### Key Commands
```bash
# Development
make run                    # Start server
make dev-test               # Full test suite in Docker (use this, not make test)
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
- `docs/planning/sprints/` - Sprint plans and task breakdowns
- `docs/planning/future/` - Future enhancements roadmap

### Finding Information
- **Current work**: Check `docs/planning/sprints/` for active tasks
- **Future plans**: Check `docs/planning/future/` for roadmap
- **Code reviews**: Check `docs/reviews/` for review findings
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

## Owner Background

- **DevOps engineer** — Windows/Linux sysadmin → DevOps. Understands infrastructure, deployment, MDM concepts from the admin side.
- **Languages**: Python and PostgreSQL at intermediate level. No Go experience — don't assume the owner reads Go code directly.
- **Strengths**: Architecture patterns, infrastructure decisions, tradeoff analysis, knowing when something smells wrong.
- **When explaining**: Use plain language for Go-specific patterns. Relate to concepts the owner already knows. Don't over-simplify, just be clear.

## Git Workflow

- **Feature branch per sprint**: `sprint-{id}/{short-description}`. Never commit directly to main.
- **Commit per sub-task**, referencing task ID (e.g. `S4b-01: ...`). Run tests with `-race` before each commit.
- **Push after each commit** — work must survive session loss. Per-task commits are recovery checkpoints.
- Don't squash or amend unless asked. Owner wants to see progression.

## Session Closeout

The owner drives a structured retrospective after each sprint. Wait for explicit prompts — don't run the retro autonomously. The retro typically covers: backward look (scope, dead code, stale docs), forward look (next sprints, roadmap alignment), doc/test audit, cleanup, and feedback.

## Production Architecture (decided Sprint 4b session)

### Deployment: AWS ECS Fargate (not Kubernetes)
- Single Go binary per service — no sidecar proxies or service mesh needed.
- ALB for TLS termination (ACM certificates, auto-renewing) and path-based routing.

### Services (3 ECS services + RDS)
- **localmdm** — the Go application (API, policy engine, enrollment). Multiple tasks behind ALB.
- **nanomdm** — Apple MDM protocol handler (separate ECS service). Receives `/checkin` and `/mdm` via ALB path routing. Uses same RDS instance, separate `nanomdm` database. Local MDM sends commands to NanoMDM via HTTP API (`nanomdm_url` config).
- **keycloak** — OIDC identity provider (separate ECS service). Admin login, JWT issuance, RBAC. Uses same RDS instance, separate database.
- **RDS PostgreSQL** — primary (Writer pool) + read replica (Reader pool). NanoMDM and Keycloak use primary only.

### Multi-Instance Safety
- **Stateless by design** — all shared state is in PostgreSQL (token cache, idempotency keys, SCEP challenges).
- **In-memory rate limiters** are per-instance (imprecise across instances, acceptable as fallback behind WAF).
- **No sticky sessions required** — any instance can handle any request.

### What NOT to do
- Don't add Redis or any external cache — PostgreSQL handles all caching.
- Don't default to Kubernetes — ECS Fargate is the target unless explicitly changed.
- Don't assume single-instance — all code must work behind a load balancer with multiple instances.

## Implementation Patterns

### Repository Pattern (Writer/Reader Pools)
- **Constructors take two args**: `NewXxxRepository(writer, reader interface{})`. Both resolve via `resolveExecutor()`.
- **Write methods** (Create/Update/Delete): `getExecutor(ctx, r.writer)` — transaction-aware via context.
- **Read methods** (Get/List): `getReadExecutor(ctx, r.reader)` — returns tx if active, otherwise uses reader pool.
- **Non-repo consumers** (audit, idempotency, certs, metrics, auth, DEP): use Writer pool directly (`*sql.DB`).
- **Transactor**: uses Writer pool exclusively. `NewTransactor(database.Writer)`.

### Service Layer
- `internal/service/` — business logic between handlers and repos. Transport-agnostic (no `net/http`).
- Services accept repository interfaces via constructor (dependency injection).
- New business logic goes in services. Existing simple handlers (CRUD) stay as-is.

### Error Handling
- Not-found detection: `errors.Is(err, apperrors.ErrNotFound)` — repos wrap with `fmt.Errorf("xxx not found: %w", apperrors.ErrNotFound)`, handlers check with `errors.Is()`.
- Audit logging: `s.logAudit(r, action, resourceType, resourceID, details)` on all mutations.

### Testing
- **Handler tests**: mock repos in `handler_test_helpers_test.go`.
- **Service tests**: hand-written mock repos in `*_test.go` within `internal/service/`.
- **Integration tests**: run in Docker. All test DB connections use `DB_HOST`/`DB_PASSWORD` env vars with localhost fallback.
- **Always run `make dev-test`** — runs all 19 test packages in Docker with race detector.
- **Pre-commit**: run `make prod-test` — builds a clean production container and runs the full suite.

## Version History

- **2026-02-07**: Initial steering guide created
- **2026-04-20**: Updated for Sprint 4b (Writer/Reader pool pattern, repo constructor changes)
- **2026-04-23**: Consolidated stable sections from SESSION_NOTES (owner background, git workflow, session closeout, production architecture, implementation patterns)

---

**Remember**: Keep it simple, test thoroughly, respect design decisions, and consult documentation before major changes.
