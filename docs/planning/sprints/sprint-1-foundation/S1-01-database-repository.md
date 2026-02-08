# S1-01: Database & Repository Layer

**Sprint**: 1 — Foundation
**Parallel**: ✅ No blockers — can start immediately
**Effort**: 3-4 days

## Objective

Establish the PostgreSQL schema, migrations, connection pooling, and repository pattern that all services build on.

## Tasks

### 1. Database Connection & Pooling
- Connection pool with configurable max connections, idle timeout
- Health check query
- Graceful shutdown (drain connections)
- Files: `internal/db/connection.go`, `internal/db/health.go`

### 2. Migration System
- golang-migrate integration
- Initial schema migration (enterprises, users, devices, policies, certificates, audit_logs, api_tokens)
- NanoMDM schema tables (devices, users, enrollments, commands — see `docs/dependencies/nanomdm/schema-pgsql.sql`)
- NanoDEP schema table (dep_names — see `docs/dependencies/nanodep/schema-pgsql.sql`)
- Makefile targets: `migrate-up`, `migrate-down`, `migrate-create`
- Files: `migrations/000001_initial_schema.up.sql`, `migrations/000001_initial_schema.down.sql`

### 3. Transaction Support
- Transaction wrapper with commit/rollback
- Context-aware (pass `*sql.Tx` through context)
- Files: `internal/db/transaction.go`

### 4. Repository Interfaces & Implementations
- Base repository interface with pagination, filtering, sorting helpers
- `EnterpriseRepository` — CRUD for enterprises/tenants
- `UserRepository` — CRUD, find by email, list by enterprise
- `DeviceRepository` — CRUD, find by serial/UDID, list by enterprise/platform
- `PolicyRepository` — CRUD, list by enterprise, assignment operations
- `CertificateRepository` — CRUD, find by serial, revocation
- `AuditLogRepository` — append-only insert, list with filters
- Files: `internal/repository/*.go`

### 5. Query Helpers
- Pagination (limit/offset from query params)
- Filtering (column-based, JSONB path queries)
- Sorting (multi-column, direction)
- Files: `internal/db/query.go`

## Interfaces to Export

```go
type DeviceRepository interface {
    Create(ctx context.Context, device *models.Device) error
    GetByID(ctx context.Context, id uuid.UUID) (*models.Device, error)
    GetBySerial(ctx context.Context, enterpriseID uuid.UUID, serial string) (*models.Device, error)
    List(ctx context.Context, enterpriseID uuid.UUID, filters Filters) ([]*models.Device, int, error)
    Update(ctx context.Context, device *models.Device) error
    Delete(ctx context.Context, id uuid.UUID) error
}
```

Similar pattern for all repositories.

## Acceptance Criteria

- [ ] Migrations apply cleanly to a fresh PostgreSQL 15 instance
- [ ] Migrations roll back cleanly
- [ ] All repository CRUD operations work with integration tests
- [ ] Pagination returns correct total counts
- [ ] Soft deletes filter correctly (`deleted_at IS NULL`)
- [ ] Enterprise isolation enforced (all queries scoped to enterprise_id)
