# Sprint 4b: Read/Write Database Pools

**Duration**: 1-2 days  
**Goal**: Split single DB connection into separate Writer and Reader pools to prepare for Aurora read replicas  
**Depends on**: Sprint 4 complete

> **Created 2026-04-20.** Extracted from Sprint 4 prerequisites to keep the pool refactor isolated from feature work.

---

## Why a Separate Sprint

Read/Write pool splitting touches every repository constructor and the server initialization. Doing it as a standalone sub-sprint means:

- Sprint 4 lands clean with all policy/compliance/lifecycle feature work
- This refactor gets its own commit history for easy bisection if anything regresses
- No merge noise in the middle of Sprint 4's feature development

---

## Tasks

| ID | Task | Effort |
|---|---|---|
| S4b-01 | Update `internal/db/db.go`: `DB` struct holds `Writer *sql.DB` and `Reader *sql.DB` | 0.5 day |
| S4b-02 | Add `writer_dsn` and `reader_dsn` to `DatabaseConfig` (fall back to single DSN if reader not set) | 0.5 day |
| S4b-03 | Update all repository constructors to accept both pools — writes use Writer, reads use Reader | 0.5 day |
| S4b-04 | Update server.go wiring, tests, verify all pass with `-race` | 0.5 day |

### Design

- `getExecutor(ctx, db)` pattern continues to work for transaction-aware writes
- In dev, both pools point to the same PostgreSQL instance
- In production, Reader points to Aurora read replica
- Existing `Transactor` uses Writer pool exclusively

## Definition of Done

- [ ] `DB` struct exposes `Writer` and `Reader` pools
- [ ] Config supports `writer_dsn` / `reader_dsn` with single-DSN fallback
- [ ] All repository reads use Reader pool, writes use Writer pool
- [ ] All tests pass with `-race`
- [ ] No functional change — purely structural prep
