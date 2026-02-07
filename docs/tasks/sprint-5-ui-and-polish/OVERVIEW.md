# Sprint 5: UI, Reporting & Polish

**Duration**: 2-3 weeks
**Goal**: Web dashboard, reporting, documentation, and production readiness
**Depends on**: Sprint 4 complete

## Tasks Overview

| ID | Task | Parallel? | Dependencies | Estimated Effort |
|---|---|---|---|---|
| S5-01 | [Web Dashboard](S5-01-web-dashboard.md) | ✅ Yes | All API endpoints from S1-S4 | 6-8 days |
| S5-02 | [Reporting & Audit](S5-02-reporting-audit.md) | ✅ Yes | S2-06, S4-03 | 4-5 days |
| S5-03 | [API Documentation & OpenAPI Spec](S5-03-api-docs.md) | ✅ Yes | All API endpoints | 2-3 days |
| S5-04 | [Deployment & Operations Guide](S5-04-deployment.md) | ✅ Yes | All components | 2-3 days |
| S5-05 | [End-to-End Testing & Hardening](S5-05-e2e-testing.md) | ⚠️ Partial | S5-01 (for UI tests), all platform tasks | 4-5 days |
| S5-06 | [Observability & Operations](S5-06-observability.md) | ✅ Yes | All components | 3-4 days |
| S5-07 | [Performance Testing & Optimization](S5-07-performance-testing.md) | ⚠️ Partial | All sprints complete | 2-3 days |
| S5-08 | [CLI Tools for Administration](S5-08-cli-tools.md) | ✅ Yes | All API endpoints | 2-3 days |

## Dependency Graph

```
S4 complete
    │
    ├── S5-01 (Dashboard) ──────────┐
    ├── S5-02 (Reporting) ──────────┤
    ├── S5-03 (API Docs) ───────────┤──→ S5-05 (E2E Testing)
    └── S5-04 (Deployment Guide) ───┘
```

All tasks except S5-05 can run fully in parallel. S5-05 benefits from having the dashboard and docs ready but can start with API-level E2E tests immediately.

## Definition of Done

- [ ] Web dashboard: login, device list, device detail, policy management, compliance view
- [ ] Reports: device inventory, compliance summary, audit log export
- [ ] OpenAPI 3.0 spec matches all implemented endpoints
- [ ] Deployment guide covers Docker, Docker Compose, and bare metal
- [ ] E2E tests cover enrollment → policy → compliance flow for all platforms
- [ ] No critical or high severity bugs open
