# Sprint 5b: EventBus & Compliance Wiring

**Status**: 🔲 Not Started  
**Duration**: 3-5 days  
**Goal**: Build the Go-side LISTEN/NOTIFY EventBus listener, wire compliance auto-evaluation into device and policy lifecycle events, fix device state parsing in check-in handlers  
**Depends on**: Sprint 5f complete (5f changes `NewCAManager` to fail on missing files — tests that relied on auto-generation need updating first)

---

## Why This Sprint

Sprint 4 installed 6 PostgreSQL triggers that fire `pg_notify` on device, policy, command, assignment, and compliance events. Sprint 5 built the compliance evaluation engine with real policy checks. But nothing connects them — compliance only evaluates when explicitly called via the API (`POST /devices/{id}/compliance/evaluate`). The EventBus listener is the missing piece that makes compliance reactive.

The dashboard (Sprint 5d) depends on live compliance data. Without the EventBus, the compliance view shows stale results.

---

## Current State of Triggers

All 6 triggers fire on a **single channel `mdm_events`** with JSON payload `{type, id, device_id, table, op}`:

| Trigger | Table | Fires On | Event Type |
|---------|-------|----------|------------|
| `device_enrolled_event` | devices | AFTER INSERT | `device.enrolled` |
| `device_updated_event` | devices | AFTER UPDATE OF **status** only | `device.status_changed` |
| `policy_updated_event` | policies | AFTER UPDATE | `policy.updated` |
| `command_created_event` | device_commands | AFTER INSERT | `command.created` |
| `policy_assigned_event` | policy_assignments | AFTER INSERT | `policy.assigned` |
| `compliance_evaluated_event` | compliance_results | AFTER INSERT OR UPDATE | `compliance.evaluated` |

### Missing Triggers (need new migration)

| Trigger Needed | Table | Fires On | Why |
|----------------|-------|----------|-----|
| `device_info_updated` | devices | AFTER UPDATE OF platform_data | **Most critical** — device reports new state during check-in, compliance must re-evaluate |
| `policy_unassigned` | policy_assignments | AFTER DELETE | Policy removed from device/group — compliance results should be cleaned up |
| `group_member_added` | group_memberships | AFTER INSERT | Device added to group — effective policies change, compliance must re-evaluate |
| `group_member_removed` | group_memberships | AFTER DELETE | Device removed from group — effective policies change |

---

## Tasks

| ID | Task | Effort | Dependencies |
|---|---|---|---|
| S5b-01 | EventBus LISTEN/NOTIFY Go listener | 1-2 days | Migration 000007 triggers |
| S5b-02 | New triggers migration (platform_data, unassign, group membership) | 0.5 day | — |
| S5b-03 | Wire compliance evaluation into EventBus subscribers | 1 day | S5b-01 |
| S5b-04 | Device state parsing in check-in handlers | 1 day | S5b-03 |
| S5b-05 | Register lifecycle hooks + fix Android lifecycle gap | 0.5 day | S5b-01 |
| S5b-06 | Load testing framework (k6 scenarios) | 0.5 day | All API endpoints |
| S5b-07 | Reporting integration test coverage (67.9% → 80%+) | 0.5 day | DB_HOST fix in 5e got it to 67.9%; add ComplianceReport, EnrollmentReport tests |

### S5b-01: EventBus LISTEN/NOTIFY Listener

Build `internal/service/eventbus.go`:

```go
type EventBus struct {
    dsn         string          // Writer pool DSN (read replicas don't relay NOTIFY)
    subscribers map[string][]EventHandler
    logger      *slog.Logger
}

type MDMEvent struct {
    Type     string    `json:"type"`      // e.g. "device.enrolled", "policy.updated"
    ID       uuid.UUID `json:"id"`        // entity ID
    DeviceID *uuid.UUID `json:"device_id"` // nullable
    Table    string    `json:"table"`
    Op       string    `json:"op"`        // INSERT, UPDATE, DELETE
}

type EventHandler func(ctx context.Context, event MDMEvent) error
```

**Architecture decisions**:
- **Single channel**: All triggers fire on `mdm_events`. The listener demuxes by `event.Type` to dispatch to the correct subscribers.
- **Dedicated connection**: Use `sql.DB.Conn(ctx)` to acquire a single long-lived connection from the Writer pool. Call `LISTEN mdm_events` on it. This connection must NOT be returned to the pool.
- **Keep-alive**: Periodic `SELECT 1` ping every 30 seconds to prevent connection timeout.
- **Reconnect**: If the connection drops, log an error and reconnect with exponential backoff.
- **Graceful shutdown**: Context cancellation stops the listener loop.
- **Multi-instance safe**: All server instances receive all events (PostgreSQL NOTIFY is fan-out). Subscribers must be idempotent.
- **Error handling**: Subscriber errors are logged but don't stop the listener. Failed events are not retried (fire-and-forget).

**Wire into server**:
- Create EventBus in `Server.New()` using `cfg.Database.DSN()` (Writer pool DSN)
- Start listener in `Server.Start()` alongside other background services
- Stop in `Server.Shutdown()`

### S5b-02: New Triggers Migration

Create `migrations/000011_eventbus_triggers.up.sql`:

```sql
-- Trigger for platform_data changes (device state updates from check-in)
CREATE TRIGGER device_info_updated_event
    AFTER UPDATE OF platform_data ON devices
    FOR EACH ROW
    WHEN (OLD.platform_data IS DISTINCT FROM NEW.platform_data)
    EXECUTE FUNCTION notify_mdm_event('device.info_updated');

-- Trigger for policy unassignment
CREATE TRIGGER policy_unassigned_event
    AFTER DELETE ON policy_assignments
    FOR EACH ROW
    EXECUTE FUNCTION notify_mdm_event('policy.unassigned');

-- Trigger for group membership changes
CREATE TRIGGER group_member_added_event
    AFTER INSERT ON group_memberships
    FOR EACH ROW
    EXECUTE FUNCTION notify_mdm_event('group.member_added');

CREATE TRIGGER group_member_removed_event
    AFTER DELETE ON group_memberships
    FOR EACH ROW
    EXECUTE FUNCTION notify_mdm_event('group.member_removed');
```

**Note**: The `notify_mdm_event()` function uses `NEW.id` and `NEW.device_id` — for DELETE triggers, these will be `OLD.id` and `OLD.device_id`. The function needs a minor update to handle `TG_OP = 'DELETE'` by using `OLD` instead of `NEW`.

### S5b-03: Wire Compliance into EventBus Subscribers

Register these subscribers at startup:

| Event Type | Subscriber Action |
|------------|-------------------|
| `device.enrolled` | Evaluate new device against enterprise-wide policies |
| `device.info_updated` | Re-evaluate device against all assigned policies (device just reported new state) |
| `policy.updated` | Re-evaluate all devices assigned to this policy |
| `policy.assigned` | Evaluate the target device (or all devices in target group/enterprise) |
| `policy.unassigned` | Clean up compliance results for the removed assignment |
| `group.member_added` | Evaluate the device against the group's assigned policies |
| `group.member_removed` | Re-evaluate the device (it may have lost policies) |

**Implementation**: Each subscriber calls `complianceService.EvaluateDevice()` for the affected device(s). For policy-level events (`policy.updated`, `policy.assigned`), the subscriber needs to resolve which devices are affected (via `groupService.GetEffectivePolicies` in reverse — find all devices assigned to a policy).

**New method needed**: `ComplianceService.EvaluateAllForPolicy(ctx, policyID)` — finds all devices assigned to this policy and evaluates each one.

### S5b-04: Device State Parsing in Check-in Handlers

Enhance check-in and webhook handlers to extract security-relevant fields into `platform_data`.

**Architecture note (Sprint 5c change)**: macOS devices check in to NanoMDM, which forwards events to Local MDM via JSON webhook. The `CheckinHandler` in `internal/platform/macos/webhook.go` processes these webhooks. Sprint 5c's E2E test (`mdmb_enrollment_test.go`) already demonstrates enriched parsing of Authenticate/TokenUpdate messages — this logic needs to be moved into the production `CheckinHandler`.

**⚠️ CRITICAL**: The production `CheckinHandler` in `internal/platform/macos/webhook.go` currently only extracts UDID from Authenticate messages. The enriched handler that extracts Serial, DeviceName, Model, OSVersion, BuildVersion, and updates status to "enrolled" on TokenUpdate exists ONLY in `tests/e2e/mdmb_enrollment_test.go`. This must be ported to production code in this task. The test handler is the reference implementation — copy the plist struct and switch/case logic into `CheckinHandler.ServeHTTP()`.

**macOS** (`internal/platform/macos/webhook.go` — CheckinHandler):
- On Authenticate: extract SerialNumber, DeviceName, ProductName, OSVersion, BuildVersion into device record (pattern from Sprint 5c E2E test)
- On TokenUpdate: set status to enrolled, store PushMagic and token presence
- On SecurityInfo command response (via NanoMDM command webhook): parse `FDE_Enabled` → `FileVaultEnabled`, `FirewallEnabled`, `IsPasscodePresent` → `password_present`

**Windows** (`internal/platform/windows/management.go`):
- Add CSP URIs to the `fieldMap`:
  - `./Vendor/MSFT/BitLocker/Status/DeviceEncryptionStatus` → `bitlocker_status`
  - `./Vendor/MSFT/Firewall/MdmStore/Global/EnableFirewall` → `firewall_enabled`
  - `./Vendor/MSFT/DeviceLock/DevicePasswordEnabled` → `password_present`

**Android** (`internal/platform/android/webhook.go`):
- In `handleStatusReport`: parse `device.SecurityPosture` from the Google API response (requires Google client — deferred until F-01 GCP setup)
- In `handleComplianceReport`: persist compliance data to `platform_data`
- Actually persist the data: call `h.service.UpdateDevice()` to save to `platform_data`

### S5b-05: Register Lifecycle Hooks + Fix Android Gap

**Problem**: `LifecycleService.RegisterHook()` is never called in production code. The lifecycle service has zero subscribers. Android's `handleUnenrollment` calls `UpdateDeviceStatus` (wired in Sprint 5c) but doesn't call lifecycle hooks.

**Fix**:
1. Create a `ComplianceCleanupHook` that implements `DeviceLifecycleHook`:
   - `OnUnenroll`: clear compliance results for the device
   - `OnWipe`: clear compliance results
   - `OnDelete`: clear compliance results
2. Register it in `Server.New()`: `s.lifecycleService.RegisterHook(complianceCleanupHook)`
3. In the Android `WebhookHandler.handleUnenrollment()`: add `h.lifecycle.OnUnenroll(ctx, device)` call (WebhookHandler was wired in Sprint 5c, needs LifecycleService added to its constructor)

### S5b-06: Load Testing Framework

Create `tests/load/` with k6 scenarios:
- `enrollment_burst.js` — 100 device enrollments in 2 minutes
- `steady_state.js` — 1000 devices checking in over 1 hour
- `admin_dashboard.js` — 10 concurrent admin sessions (list devices, policies, compliance)
- `policy_deploy.js` — assign policy to 1000 devices, verify compliance evaluation

Include a `README.md` with setup instructions and performance targets.

**Docker**: Add k6 as a docker-compose service (profile: `load-test`) or install in the dev container. Tests should target `http://localmdm:8080` (Docker networking). All dev/test runs in Docker per Sprint 5c convention.

---

## Definition of Done

- [ ] EventBus listener receives NOTIFY events from PostgreSQL triggers
- [ ] New triggers fire on platform_data changes, policy unassignment, group membership changes
- [ ] Compliance auto-evaluates after device check-in (platform_data update)
- [ ] Compliance re-evaluates when policy is updated or assigned
- [ ] macOS/Windows/Android check-in handlers populate security state in platform_data
- [ ] At least one lifecycle hook registered (compliance cleanup)
- [ ] Load test scenarios exist and run against local environment
- [ ] All existing tests pass

---

*Created: 2026-04-22 — Split from Sprint 5 deferred items*  
*Updated: 2026-04-22 — Added missing triggers, compliance subscribers, lifecycle hooks, Android gap*
