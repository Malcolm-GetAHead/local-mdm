# Sprint 4: Policy Abstraction & Identity Integration

**Duration**: 2 weeks
**Goal**: Unified policy system across platforms + Keycloak Platform SSO for macOS
**Depends on**: Sprint 3 complete (platform commands/profiles working)

## Tasks Overview

| ID | Task | Parallel? | Dependencies | Estimated Effort |
|---|---|---|---|---|
| S4-01 | [Unified Policy Model & Translators](S4-01-unified-policy.md) | ✅ Yes | S3-01, S3-02, S3-03 | 5-6 days |
| S4-02 | [Policy Assignment & Groups](S4-02-policy-assignment.md) | ⚠️ Partial | S4-01 (model), S2-06 (device service) | 3-4 days |
| S4-03 | [Compliance Engine](S4-03-compliance.md) | ⚠️ Partial | S4-01, S4-02 | 3-4 days |
| S4-05 | [Device Lifecycle Hooks](S4-05-keycloak-device-sync.md) | ✅ Yes | None | 1-2 days |

> **Note**: S4-04 (macOS Platform SSO) moved to Sprint 4b. S4-05 rescoped 2026-04-20: Keycloak PSSO admin client, device registry sync, and reconciliation moved to Sprint 4b. S4-05 now covers only the lifecycle hook infrastructure (interface + wiring CheckOut/wipe/delete handlers).

## Dependency Graph

```
S3 complete
    │
    ├── S4-01 (Unified Policy) ──→ S4-02 (Assignment) ──→ S4-03 (Compliance)
    │
    └── S4-05 (Lifecycle Hooks) — parallel, no dependencies
```

## Architecture Decision: Service Layer

**Decision (2026-04-20):** Sprint 4 introduces `internal/service/` as a business logic layer between HTTP handlers and repositories.

**Pattern:**
```
HTTP Handler (thin: parse request + format response)
    → Service (business logic, reusable)
        → Repository (DB queries)
        → Command Dispatcher (async platform dispatch)
```

**Why:** Sprint 4's policy assignment, compliance evaluation, and lifecycle hooks involve multi-step business logic that would bloat handlers and can't be reused from the CLI (Sprint 5). The service layer keeps handlers thin and makes business logic testable and reusable.

**Rules:**
- New Sprint 4+ business logic goes in `internal/service/`
- Existing handlers (CRUD, lock/wipe/restart) stay as-is — no refactor for its own sake
- Services accept repository interfaces via constructor (dependency injection)
- Services do NOT import `net/http` — they are transport-agnostic
- Handlers call services; services call repos and the command dispatcher

**New packages:**
- `internal/service/policy.go` — policy translation, deployment, templates
- `internal/service/compliance.go` — evaluation, reporting, remediation
- `internal/service/lifecycle.go` — device lifecycle hook management
- `internal/service/groups.go` — device group business logic

## Service-Level Dependencies

| This Sprint Produces | Consumed By |
|---|---|
| Unified policy model + translators | Sprint 5 (policy UI) |
| Policy assignment to devices/groups | Sprint 5 (group management UI) |
| Compliance engine | Sprint 5 (compliance dashboard) |
| Keycloak ↔ MDM device sync | Device lifecycle management |

## Definition of Done

- [ ] Define a policy once, deploy to all three platforms with correct translation
- [ ] Assign policy to device group, all devices in group receive it
- [ ] Compliance engine reports which devices are non-compliant and why
- [ ] Device unenrollment/wipe/delete triggers lifecycle hooks (Keycloak integration in 4b)
- [ ] Idempotency-Key header support on all POST endpoints

## Additional Items

### Idempotency-Key Support
Added from Sprint 2 (M-05). Sprint 2 implemented simple duplicate prevention via DB unique constraints + 409 Conflict responses. Full `Idempotency-Key` header support (store key + cached response in Redis, return cached response on duplicate key) should be implemented in this sprint when the policy assignment system creates more complex multi-step operations that benefit from idempotent retries.
