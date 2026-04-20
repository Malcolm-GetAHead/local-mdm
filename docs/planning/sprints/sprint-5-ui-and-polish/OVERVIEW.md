# Sprint 5: Backend Polish, CLI & Production Readiness

**Duration**: 2 weeks  
**Goal**: API documentation, CLI tools, E2E testing, observability, performance, deployment guide  
**Depends on**: Sprint 4 complete

> **Note**: Web Dashboard (S5-01) has been moved to [Sprint 5b](../sprint-5b-web-dashboard/OVERVIEW.md) as a separate frontend effort.

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

**Total effort**: ~20-25 days (most tasks parallel; critical path ~8-10 days)

## Notes on Overlap with Sprint 2

Sprint 2 already delivered:
- Prometheus metrics on separate internal port (partial S5-06)
- Structured logging with slog (partial S5-06)
- Request ID propagation (partial S5-06)

S5-06 should be scoped down to: enhanced health checks (/health/ready with dependency checks), alerting documentation, and any gaps in the existing observability setup. The Prometheus metrics and structured logging work is done.

## Notes from Sprint 4

- **EventBus LISTEN/NOTIFY listener**: Sprint 4 installed PostgreSQL triggers (migration 000007) that fire `pg_notify` on device, policy, command, assignment, and compliance events. The Go-side listener (`internal/service/eventbus.go`) was deferred to Sprint 5. S5-09 should wire compliance evaluation as the first EventBus subscriber. Future subscribers: outbound webhooks (F-07), notifications.
- **Compliance engine placeholder**: `evaluatePolicy()` returns "unknown" — S5-09 adds real evaluation logic.
- **Policy deployment on assignment**: Policies are recorded but not pushed on assignment. Devices pick up policies on next check-in. EventBus can trigger push notifications in the future.

## Notes from Sprint 2a Audit

- **S5-08 (CLI) assumes API token auth exists** but no sprint task explicitly builds the server-side token generation/validation/revocation endpoints. The `api_tokens` table exists in the schema. This should be built as part of S5-08 or as a prerequisite task. See F-07 sections 9-10 for the full spec if it's deferred beyond v1.0.
- **User management CRUD** (`GET/POST/PUT/DELETE /api/v1/users`) is assumed by both S5-08 and S5b (dashboard) but not explicitly tasked. Same recommendation — build as part of S5-08 or add a dedicated task. See F-07 section 9.

## Dependency Graph

```
S4 complete
    │
    ├── S5-09 (Device State) ───────┐
    ├── S5-10 (Service Migration) ──┤──→ S5-08 (CLI Tools)
    ├── S5-02 (Reporting) ──────────┤ (S5-02 depends on S5-09)
    ├── S5-03 (API Docs) ───────────┤
    ├── S5-04 (Deployment Guide) ───┤──→ S5-05 (E2E Testing)
    ├── S5-06 (Observability) ──────┤
    └── S5-07 (Performance) ────────┘
```

## Definition of Done

- [ ] OpenAPI 3.0 spec matches all implemented endpoints
- [ ] CLI tools: device/policy/group management, enrollment commands, table/JSON output
- [ ] Reports: device inventory, compliance summary, audit log export
- [ ] Deployment guide covers Docker, Docker Compose, and bare metal
- [ ] E2E tests cover enrollment → policy → compliance flow for all platforms
- [ ] Performance targets met (API p95 <200ms, 50 concurrent enrollments)
- [ ] No critical or high severity bugs open

---

*Updated: 2026-04-18 — Dashboard moved to Sprint 5b*
