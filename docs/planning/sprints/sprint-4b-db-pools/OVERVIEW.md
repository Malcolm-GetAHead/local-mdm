# Sprint 4b: Read/Write Database Pools

**Status**: ✅ Complete  
**Duration**: 1 session  
**Goal**: Split single DB connection into separate Writer and Reader pools to prepare for Aurora read replicas  
**Depends on**: Sprint 4 complete  
**Branch**: `sprint-4b/read-write-pools`

> **Created 2026-04-20.** Extracted from Sprint 4 prerequisites to keep the pool refactor isolated from feature work.

---

## Why a Separate Sprint

Read/Write pool splitting touches every repository constructor and the server initialization. Doing it as a standalone sub-sprint means:

- Sprint 4 lands clean with all policy/compliance/lifecycle feature work
- This refactor gets its own commit history for easy bisection if anything regresses
- No merge noise in the middle of Sprint 4's feature development

---

## Tasks

| ID | Task | Status |
|---|---|---|
| S4b-01 | Update `internal/db/db.go`: `DB` struct holds `Writer *sql.DB` and `Reader *sql.DB` | ✅ |
| S4b-02 | Add `ReaderConfig` to `DatabaseConfig` with field-level fallback | ✅ |
| S4b-03 | Update all 11 repository constructors to accept `(writer, reader interface{})` | ✅ |
| S4b-04 | Update server.go wiring, non-repo consumers, all tests pass with `-race` | ✅ |

### Design

- `getExecutor(ctx, r.writer)` for write methods — transaction-aware via context
- `getReadExecutor(ctx, r.reader)` for read methods — returns tx if active (reads see uncommitted writes within transactions), otherwise uses reader pool
- `resolveExecutor` helper for DRY constructor validation
- Non-repo consumers (audit, idempotency, certs, metrics, auth, DEP) use Writer pool
- In dev, both pools point to the same PostgreSQL instance
- In production, Reader points to Aurora read replica
- Existing `Transactor` uses Writer pool exclusively

### Config

```yaml
database:
  # Base config (used for writer pool, and reader pool if no overrides)
  host: "localhost"
  port: 5432
  user: "postgres"
  password: "secure-password"
  database: "localmdm"
  sslmode: "disable"
  max_open_conns: 25
  max_idle_conns: 5
  conn_max_lifetime: 5m
  query_timeout: 30s

  # Optional: reader pool overrides (for Aurora read replica)
  # Unset fields inherit from base config
  # reader:
  #   host: "replica.example.com"
  #   max_open_conns: 10
  #   max_idle_conns: 3
```

Environment variable overrides: `DB_READER_HOST`, `DB_READER_PORT`

## Definition of Done

- [x] `DB` struct exposes `Writer` and `Reader` pools
- [x] Config supports `ReaderConfig` with field-level fallback to base config
- [x] All repository reads use Reader pool, writes use Writer pool
- [x] All tests pass with `-race`
- [x] No functional change — purely structural prep
