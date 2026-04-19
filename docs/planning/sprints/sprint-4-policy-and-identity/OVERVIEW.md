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
| S4-05 | [Keycloak Device Lifecycle Sync](S4-05-keycloak-device-sync.md) | ✅ Yes | S2-06 | 2-3 days |

> **Note**: S4-04 (macOS Platform SSO) and its dependency S4-05's PSSO-specific work have been moved to Sprint 4b to avoid context-switching between Go backend work and the Java/Swift external dependencies that PSSO requires.

## Dependency Graph

```
S3 complete
    │
    ├── S4-01 (Unified Policy) ──→ S4-02 (Assignment) ──→ S4-03 (Compliance)
    │
    └── S4-05 (Keycloak Device Sync) — parallel with S4-01
```

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
- [ ] Device unenrollment triggers Keycloak device removal
- [ ] Idempotency-Key header support on all POST endpoints

## Additional Items

### Idempotency-Key Support
Added from Sprint 2 (M-05). Sprint 2 implemented simple duplicate prevention via DB unique constraints + 409 Conflict responses. Full `Idempotency-Key` header support (store key + cached response in Redis, return cached response on duplicate key) should be implemented in this sprint when the policy assignment system creates more complex multi-step operations that benefit from idempotent retries.
