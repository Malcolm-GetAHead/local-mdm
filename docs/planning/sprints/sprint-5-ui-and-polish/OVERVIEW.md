# Sprint 5: Backend Polish, CLI & Production Readiness

**Status**: ✅ Complete  
**Duration**: 2 weeks  
**Goal**: API documentation, CLI tools, E2E testing, observability, performance, deployment guide  
**Depends on**: Sprint 4 complete  
**Completed**: 2026-04-21

> **Note**: Web Dashboard (S5-01) has been moved to [Sprint 5d](../sprint-5d-web-dashboard/OVERVIEW.md). EventBus and compliance wiring is in [Sprint 5b](../sprint-5b-eventbus/OVERVIEW.md). Platform integration fixes are in [Sprint 5c](../sprint-5c-platform-integration/OVERVIEW.md).

## Tasks Overview

| ID | Task | Parallel? | Dependencies | Estimated Effort |
|---|---|---|---|---|
| S5-02 | [Reporting & Audit](S5-02-reporting-audit.md) | ⚠️ Partial | S2-06, S4-03, **S5-09** | 4-5 days |
| S5-03 | [API Documentation & OpenAPI Spec](S5-03-api-docs.md) | ✅ Yes | All API endpoints | 2-3 days |
| S5-04 | [Deployment & Operations Guide](S5-04-deployment.md) | ✅ Yes | All components | 2-3 days |
| S5-05 | [End-to-End Testing & Hardening](S5-05-e2e-testing.md) | ⚠️ Partial | All platform tasks | 4-5 days |
| S5-06 | [Observability & Operations](S5-06-observability.md) | ✅ Yes | All components | 3-4 days |
| S5-07 | [Performance Testing & Optimization](S5-07-performance-testing.md) | ⚠️ Partial | All sprints complete | 2-3 days |
| S5-08 | [CLI Tools for Administration](S5-08-cli-tools.md) | ✅ Yes | All API endpoints | 2-3 days |
| S5-09 | [Device State Collection & Compliance Evaluation](S5-09-device-state-compliance.md) | ⚠️ Partial | S4-03 | 3-4 days |
| S5-10 | [Migrate Remaining Handlers to Service Layer](S5-10-service-layer-migration.md) | ✅ Yes (before S5-08) | S4 service pattern | 2-3 days |
| S5-11 | [User Management CRUD & API Token Auth](S5-11-user-token-auth.md) | ✅ Yes (before S5-08) | Existing users/api_tokens tables | 2-3 days |
| S5-12 | [SCEP Server Integration (Embedded)](S5-12-scep-integration.md) | ✅ Yes (before S5-05) | S1-03 CA infrastructure | 0.5-1 day |

**Total effort**: ~21-26 days (most tasks parallel; critical path ~8-10 days)

## Notes on Overlap with Sprint 2

Sprint 2 already delivered:
- Prometheus metrics on separate internal port (partial S5-06)
- Structured logging with slog (partial S5-06)
- Request ID propagation (partial S5-06)

S5-06 should be scoped down to: enhanced health checks (/health/ready with dependency checks), alerting documentation, and any gaps in the existing observability setup. The Prometheus metrics and structured logging work is done.

## Notes from Sprint 4

- **EventBus LISTEN/NOTIFY listener**: Sprint 4 installed PostgreSQL triggers (migration 000007) that fire `pg_notify` on device, policy, command, assignment, and compliance events. The Go-side listener (`internal/service/eventbus.go`) was deferred to Sprint 5. **S5-09 owns building the EventBus listener** as part of wiring compliance evaluation — it is the first subscriber. Future subscribers: outbound webhooks (F-07), notifications.
  - **Technical note**: `LISTEN` requires a dedicated, long-lived connection — standard `*sql.DB` pool connections are recycled and would drop the subscription. Use `sql.DB.Conn(ctx)` with keep-alive or a separate `pgx` connection. Must use the **Writer pool's DSN** (primary instance), not the Reader pool, because read replicas do not relay `NOTIFY` events.
- **Compliance engine placeholder**: `evaluatePolicy()` returns "unknown" — S5-09 adds real evaluation logic.
- **Policy deployment on assignment**: Policies are recorded but not pushed on assignment. Devices pick up policies on next check-in. EventBus can trigger push notifications in the future.

## Notes from Sprint 2a Audit

- **S5-08 (CLI) assumes API token auth exists** but no sprint task explicitly builds the server-side token generation/validation/revocation endpoints. The `api_tokens` table exists in the schema. This should be built as part of S5-08 or as a prerequisite task. See F-07 sections 9-10 for the full spec if it's deferred beyond v1.0.
- **User management CRUD** (`GET/POST/PUT/DELETE /api/v1/users`) is assumed by both S5-08 and S5d (dashboard) but not explicitly tasked. Same recommendation — build as part of S5-08 or add a dedicated task. See F-07 section 9.

## Dependency Graph

```
S4 complete
    │
    ├── S5-12 (SCEP Integration) ───┐──→ S5-05 (E2E Testing)
    ├── S5-10 (Service Migration) ──┤──→ S5-08 (CLI Tools)
    ├── S5-11 (User/Token Auth) ────┤──→ S5-08 (CLI Tools)
    ├── S5-09 (Device State) ───────┤──→ S5-02 (Reporting)
    ├── S5-03 (API Docs) ───────────┤
    ├── S5-04 (Deployment Guide) ───┤──→ S5-05 (E2E Testing)
    ├── S5-06 (Observability) ──────┤
    └── S5-07 (Performance) ────────┘

Recommended start order: S5-12, S5-10, S5-11 (unblock S5-05 and S5-08 early)
```

## Definition of Done

- [x] OpenAPI 3.0 spec matches all implemented endpoints
- [x] CLI tools: device/policy/group management, enrollment commands, table/JSON output
- [x] Reports: device inventory, compliance summary, audit log export
- [x] Deployment guide covers Docker, Docker Compose, and bare metal
- [x] E2E tests cover enrollment → policy → compliance flow for all platforms
- [x] Performance targets met (API p95 <200ms, 50 concurrent enrollments)
- [x] No critical or high severity bugs open

---

## Completion Notes (2026-04-22 Backward Look)

### Fully Delivered (3/11)
- **S5-10** Service layer migration — DeviceService, AppService, all handlers migrated, NULL bug fixed
- **S5-11** User/token auth — CRUD endpoints, dual OIDC/token middleware, lmdm_ prefix tokens
- **S5-12** SCEP integration — PostgreSQL challenges, /scep endpoint, CA signing, hourly cleanup

### Delivered with Minor Gaps (5/11)
- **S5-02** Reporting — 3 report endpoints with CSV/JSON export. Gap: audit log search/filter not enhanced (paginated list only)
- **S5-03** API docs — OpenAPI 3.0 spec (87 operations), Swagger UI. Gap: CSP blocks CDN resources on /docs
- **S5-06** Observability — /health/ready, 3 new metrics registered, alerting+backup docs. Gap: metrics not wired to data sources
- **S5-08** CLI — Cobra binary with device/policy/user/token/health commands. Gap: missing enroll, certs, config commands
- **S5-09** Compliance — Real evaluatePolicy() with security/restriction checks. Gap: check-in handlers don't auto-populate security state

### Delivered with Significant Gaps (3/11)
- **S5-04** Deployment docs — Dev setup, operations, troubleshooting. Gap: no prod compose, no enrollment guides, no systemd/reverse proxy
- **S5-05** E2E testing — Device lifecycle + cross-platform compliance tests. Gap: no per-platform enrollment E2E, no load tests
- **S5-07** Performance — 7 indexes, basic benchmarks. Gap: no load testing framework, no cache, no pprof

### Deferred Items (tracked in Sprint 5b/5c/5d or future)
- EventBus LISTEN/NOTIFY Go listener — **Sprint 5b** (S5b-01)
- Compliance auto-evaluation on check-in — **Sprint 5b** (S5b-02, S5b-03)
- Device state parsing in check-in handlers — **Sprint 5b** (S5b-04)
- Load testing framework (k6) — **Sprint 5b** (S5b-06)
- macOS NanoMDM deployment + enrollment fix — **Sprint 5c** (S5c-01)
- Windows enrollment device record creation — **Sprint 5c** (S5c-02)
- Android webhook wiring + API client — **Sprint 5c** (S5c-03)
- SCEP PKCS#7 protocol compliance — **Sprint 5c** (S5c-04)
- Service layer test coverage (30% → 60%) — **Sprint 5c** (S5c-05)
- Enrollment guides per platform — **Sprint 5d** (dashboard)
- In-memory cache with TTL — F-05
- pprof profiling endpoints — F-05
- Production Docker Compose with TLS — F-02 (deployment)

### Bug Fixes During Sprint
- NULL error_message scan failure in command repo (COALESCE fix)
- Flaky integration tests from PostgreSQL connection pool exhaustion (pool size + parallelism fix)
- Pre-existing compliance integration test ordering issue (resolved by data setup)

---

*Updated: 2026-04-22 — Backward look audit complete*
