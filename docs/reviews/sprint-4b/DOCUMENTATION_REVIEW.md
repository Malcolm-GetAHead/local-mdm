# Documentation Review Findings — Post Sprint 4b

**Date**: 2026-04-21  
**Scope**: Full audit of all documentation against codebase state after Sprint 4b  
**Branch**: `sprint-4b/read-write-pools`

---

## Test Coverage Summary

| Package | Coverage (-short) | Coverage (full) | Target | Status |
|---------|------------------|-----------------|--------|--------|
| apperrors | 100% | — | 60% | ✅ |
| models | 100% | — | 60% | ✅ |
| validation | 96.6% | — | 60% | ✅ |
| audit | 95.2% | — | 60% | ✅ |
| config | 93.1% | — | 60% | ✅ |
| scep | 93.3% | — | 60% | ✅ |
| tracing | 86.7% | — | 60% | ✅ |
| certs | 78.4% | — | 60% | ✅ |
| auth | 68.0% | — | 60% | ✅ |
| windows | 69.7% | — | 60% | ✅ |
| service | 67.5% | — | 60% | ✅ |
| metrics | 65.0% | — | 60% | ✅ |
| android | 61.9% | — | 60% | ✅ |
| macos | 59.0% | — | 60% | ⚠️ Borderline |
| api | 56.5% | — | 70% | ⚠️ Below target |
| repository | 47.8% | 49.1% | 80% | ⚠️ Below target (integration tests need Docker) |
| db | 29.4% | 82.4% | 60% | ✅ (with integration tests) |

---

## HIGH Priority — Actively Misleading or Significantly Incomplete

### QUICK_REFERENCE.md (`docs/dev/QUICK_REFERENCE.md`)

- [ ] **Missing all Sprint 4 API endpoints** — 15 routes not listed (groups CRUD, group members, policy versions, policy rollback, policy translate, policy templates, policy assignments, effective policies, compliance summary, device compliance, compliance evaluate)
- [ ] **Missing 8+ database tables** — device_groups, group_memberships, policy_assignments, compliance_results, device_apps, policy_versions, token_cache, idempotency_keys. Also missing users, api_tokens (from migration 000001). Table listed as `commands` should be `device_commands`
- [ ] **Missing 4 directories from project structure** — `internal/service/`, `internal/apperrors/`, `internal/logging/`, `internal/constants/`. Also `internal/certs/` description says "SCEP" but SCEP is in `internal/scep/` (separate)

### API.md (`docs/schemas/API.md`)

- [ ] **7 phantom routes** documented but don't exist in code:
  - `POST /api/v1/devices/enroll` — generic enrollment doesn't exist; enrollment is platform-specific
  - `GET/POST/PUT/DELETE /api/v1/users` — no user management handlers exist (Keycloak handles users; S5-11 will build these)
  - `GET /api/v1/openapi.yaml` — no OpenAPI spec endpoint
  - `GET /api/v1/docs` — no Swagger UI endpoint
- [ ] **3 routes in code but missing from docs**:
  - `GET /version`
  - `GET /api/v1/certificates`
  - `GET /api/v1/audit-logs`
- [ ] **~10 endpoints missing role requirements** — code enforces roles but docs don't list them:
  - `POST /devices/{id}/lock` — requires `admin, operator`
  - `POST /devices/{id}/restart` — requires `admin, operator`
  - `POST /devices/{id}/wipe` — requires `admin` + IP allowlist (docs don't mention IP allowlist)
  - `POST /policies` — requires `admin, operator`
  - `PUT /policies/{id}` — requires `admin, operator`
  - `DELETE /policies/{id}` — requires `admin`
  - `POST /policies/{id}/assign` — requires `admin, operator`
  - Sprint 4 endpoints also missing role annotations
- [ ] **Unprotected endpoint flag** — `GET /windows/ppkg/templates` has no auth middleware in code (unlike `POST /windows/ppkg` which requires `admin, operator`). Either add auth or document as intentionally public.

### DATABASE.md (`docs/schemas/DATABASE.md`)

- [ ] **3 tables completely undocumented**:
  - `device_commands` (migration 000003) — no CREATE TABLE, no column definitions, no description
  - `dep_names` (migration 000004) — DEP OAuth tokens (pgcrypto encrypted), PKI certs, syncer cursor, assigner profile
  - `dep_devices` (migration 000004) — DEP-synced device tracking with serial numbers, profile status
- [ ] **Header stale** — says "Version: 1.0, Last Updated: 2026-02-05" — hasn't been updated for Sprint 3/4/4b

### ARCHITECTURE.md (`docs/architecture/ARCHITECTURE.md`)

- [ ] **Wrong package names** — `internal/services` should be `internal/service/` (singular), `internal/repositories` should be `internal/repository/` (singular)
- [ ] **"Planned" labels on implemented code** — Services and repos are fully implemented since Sprint 1-4. Sub-lists don't match what was actually built
- [ ] **Phantom package** — `internal/webhooks` section describes a package that doesn't exist. Webhook handling is in `internal/platform/android/webhook.go` and `internal/platform/macos/webhook.go`
- [ ] **Stale "Next Steps" section** — says "1. Implement service layer, 2. Add repository layer, 3. Implement authentication, 4. Begin Windows module" — all done since Sprint 1-4. Should be removed or replaced with actual next steps (Sprint 4c/5)
- [ ] **Missing 7 packages from Component Overview** — `audit/`, `metrics/`, `scep/`, `tracing/`, `constants/`, `apperrors/`, `logging/`

### SECURITY.md (`docs/SECURITY.md`)

- [ ] **Line ~58** — "In-memory rate limiter (production should use Redis)" — Redis removed in Sprint 4. Should say PostgreSQL-backed or external rate limiting
- [ ] **Line ~154** — "[ ] Redis-backed rate limiting" in production TODO checklist — remove Redis reference

---

## MEDIUM Priority — Outdated but Not Blocking

### SETUP.md (`docs/dev/SETUP.md`)

- [ ] **Wrong environment variables listed**:
  - `CONFIG_PATH` — does not exist in config.go
  - `DB_URL` — only used by Makefile for migrations, not by the Go app
- [ ] **Missing environment variables**:
  - `ENVIRONMENT` — config.go reads this
  - `KEYCLOAK_CLIENT_SECRET` — config.go reads this
  - `DEP_ENCRYPTION_KEY` — config.go reads this
  - `DB_READER_HOST`, `DB_READER_PORT` — added in Sprint 4b
- [ ] **Missing Keycloak** — docker-compose starts Keycloak on port 8180 but SETUP.md doesn't mention it
- [ ] **Password validation gap** — example config uses default password `postgres` but config validation rejects passwords < 16 chars and rejects "postgres" specifically. Quick Start will fail without noting this
- [ ] **Go version** — says "Go 1.21 or higher" but go.mod declares 1.25
- [ ] **Last Updated** — says 2026-02-05

### DATABASE.md (`docs/schemas/DATABASE.md`)

- [ ] **`policies` table main section** missing `is_template BOOLEAN NOT NULL DEFAULT false` column (added in migration 000006). Sprint 4 section mentions it but the main CREATE TABLE block is stale
- [ ] **"Performance Considerations"** says "Read replicas for reporting queries (future enhancement)" — Sprint 4b implemented Writer/Reader pools, this is no longer future
- [ ] **No mention of Writer/Reader pool architecture** from Sprint 4b

### TESTING.md (`docs/TESTING.md`)

- [ ] **Coverage table** says "Current Coverage (Sprint 2a)" — should be updated to Sprint 4b with current numbers
- [ ] **Test structure tree** missing Sprint 3/4/4b test files:
  - `internal/service/*_test.go` (policy, groups, compliance, lifecycle)
  - `internal/api/idempotency_test.go`, `command_dispatcher_test.go`
  - `internal/config/reader_config_test.go`
  - `internal/db/dual_pool_test.go`
  - `internal/repository/read_executor_test.go`, `new_repos_test.go`
- [ ] **Sprint 3/4 coverage goals** still shown as targets, not achieved

### STEERING.md (`.kiro/steering/STEERING.md`)

- [ ] **"Getting Help" section** — references `docs/tasks/` and `docs/tasks/future/` which don't exist. Actual paths: `docs/planning/sprints/` and `docs/planning/future/`
- [ ] **File Locations tree** missing 6 packages: `audit/`, `metrics/`, `scep/`, `tracing/`, `constants/`, `apperrors/`

### SESSION_NOTES.md (`.kiro/steering/SESSION_NOTES.md`)

- [ ] **"Implementation Preferences" section** — says "Repos accept `interface{}` and type-switch on `*sql.DB` or `executor`. All use `getExecutor(ctx, r.db)` for transaction awareness." Post-Sprint 4b, repos accept two `interface{}` args (writer, reader). The `r.db` field no longer exists — it's `r.writer` and `r.reader`
- [ ] **No Sprint 4b Learnings section** — should document: Writer/Reader pool pattern, `getReadExecutor` for reads, `resolveExecutor` helper, ReaderConfig fallback, non-repo consumers use Writer pool

---

## LOW Priority — Minor Inaccuracies

### README.md

- [ ] **Go version** — says "Go 1.21+" but go.mod declares 1.25. Should say "Go 1.25+"

### ARCHITECTURE.md

- [ ] **Metrics section** says "(Future)" but Prometheus metrics are implemented in `internal/metrics/` since Sprint 2

---

## Notes

- Historical sprint docs (Sprint 1/2a implementation reviews) contain old `database.DB` code snippets. These are point-in-time snapshots and don't need updating.
- The `api` package coverage (56.5%) is below the 70% handler target. Most handler tests exist but the package has grown significantly with Sprint 3/4 endpoints.
- The `repository` package coverage (47.8% with `-short`) is misleading — integration tests that need Docker PostgreSQL bring it to 49.1%. Many repo methods are only tested via integration tests.
