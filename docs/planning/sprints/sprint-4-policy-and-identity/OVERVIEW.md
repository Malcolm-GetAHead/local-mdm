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

## Architecture Decision: Event Bus via PostgreSQL LISTEN/NOTIFY

**Decision (2026-04-20):** Sprint 4 implements an event bus using PostgreSQL's built-in `LISTEN`/`NOTIFY` for decoupled event-driven communication between services.

**Why:** The compliance engine (S4-03) needs to fire on device check-in, policy change, and manual trigger. Future features (F-07 outbound webhooks, notifications) need the same trigger points. Rather than adding direct calls in every handler, an event bus decouples producers from consumers.

**Why PostgreSQL over in-process channels or Redis:**
- Multi-instance support from day one (both server instances receive events)
- Transactional — notifications only fire on commit, so consumers never see uncommitted data
- No new infrastructure — PostgreSQL is already the primary datastore
- `lib/pq` (already in go.mod) supports LISTEN/NOTIFY natively

**Pattern:**
```
Handler/Service writes to DB
    → PostgreSQL trigger fires pg_notify('mdm_events', payload)
    → Go EventBus listener receives notification
    → Dispatches to registered subscribers
```

**Implementation:**
```sql
-- Trigger function (reusable across tables):
CREATE FUNCTION notify_event() RETURNS TRIGGER AS $$
BEGIN
    PERFORM pg_notify('mdm_events', json_build_object(
        'type', TG_ARGV[0],
        'id', NEW.id,
        'device_id', COALESCE(NEW.device_id, NEW.id)
    )::text);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Attach to tables that produce events:
CREATE TRIGGER device_command_event AFTER INSERT ON device_commands
    FOR EACH ROW EXECUTE FUNCTION notify_event('command.created');
CREATE TRIGGER policy_assignment_event AFTER INSERT ON policy_assignments
    FOR EACH ROW EXECUTE FUNCTION notify_event('policy.assigned');
CREATE TRIGGER compliance_result_event AFTER INSERT OR UPDATE ON compliance_results
    FOR EACH ROW EXECUTE FUNCTION notify_event('compliance.evaluated');
```

```go
// internal/service/eventbus.go
type EventBus struct {
    subscribers map[string][]EventHandler
}

type EventHandler func(ctx context.Context, event Event)

type Event struct {
    Type     string    // "command.created", "policy.assigned", etc.
    ID       uuid.UUID
    DeviceID uuid.UUID
}

func (bus *EventBus) Subscribe(eventType string, handler EventHandler)
func (bus *EventBus) listen(ctx context.Context, db *sql.DB) // background goroutine
```

**Event types (Sprint 4):**
- `command.created` — new command queued (triggers: compliance check if device_info result)
- `policy.assigned` — policy assigned to device/group (triggers: compliance evaluation)
- `policy.updated` — policy config changed (triggers: re-evaluate all assigned devices)
- `device.checkin` — device completed sync (triggers: compliance evaluation)
- `device.enrolled` — new device enrolled (triggers: push applicable policies)
- `device.unenrolled` — device unenrolled (triggers: lifecycle hooks)

**Subscribers (Sprint 4):**
- `ComplianceService.OnPolicyAssigned` — evaluate device compliance
- `ComplianceService.OnDeviceCheckin` — re-evaluate after state update
- `PolicyService.OnDeviceEnrolled` — push applicable policies to new device
- `LifecycleService.OnDeviceUnenrolled` — call lifecycle hooks

**Future subscribers (F-07):**
- `WebhookService.OnAnyEvent` — deliver outbound webhook notifications
- `NotificationService.OnComplianceChange` — alert admin on non-compliance

**Constraints:**
- Payload limit: 8KB per notification (sufficient — payloads are `{type, id, device_id}`, subscribers look up details from DB)
- Latency: ~1-5ms (acceptable for async reactions)
- Delivery: at-most-once within a connection (if listener disconnects, events during downtime are missed — acceptable for compliance which also runs on manual trigger)

**Files:**
- `internal/service/eventbus.go` — EventBus, subscriber registration, LISTEN loop
- `migrations/000007_event_triggers.up.sql` — pg_notify trigger functions

## Service-Level Dependencies

> **Sprint 5 Note (2026-04-20):** Sprint 4 uses direct service calls for compliance evaluation and policy push triggers. The PostgreSQL LISTEN/NOTIFY event triggers are in place (migration 000007) but the Go-side EventBus listener is deferred to Sprint 5, where outbound webhooks need async event-driven dispatch. Sprint 5 should: (1) implement `internal/service/eventbus.go` with persistent LISTEN connection, reconnection, graceful shutdown; (2) migrate direct compliance calls to EventBus subscribers; (3) add webhook subscriber for outbound notifications.

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

### Prerequisite: Remove Redis, Use PostgreSQL
**Decision (2026-04-20):** Replace Redis with PostgreSQL for all caching/storage needs. Redis is currently used only for auth token caching (`internal/auth/token_cache.go`). PostgreSQL is sufficient at our scale and simplifies the stack to a single datastore. Scaling path: PostgreSQL → Aurora PostgreSQL with read replicas.

Tasks:
- Replace Redis token cache with PostgreSQL `token_cache` table (token_hash, user_data JSONB, expires_at) + periodic cleanup
- Build Idempotency-Key middleware using PostgreSQL table with `expires_at` + periodic cleanup
- Remove `go-redis/v9` from `go.mod`
- Remove Redis from `docker-compose.yml`
- Update config to remove Redis settings

### Prerequisite: Read/Write Database Pools
**Decision (2026-04-20):** Moved to Sprint 4b as a standalone sub-sprint. See [Sprint 4b: DB Pools](../sprint-4b-db-pools/OVERVIEW.md).

> **Sprint renaming (2026-04-20):** Former Sprint 4b (Platform SSO) is now Sprint 4c. Sprint 4b is Read/Write Database Pools. See [Sprint 4c: Platform SSO](../sprint-4c-platform-sso/OVERVIEW.md).
