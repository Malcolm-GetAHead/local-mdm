# Sprint 5b: EventBus & Compliance Wiring

**Status**: 🔲 Not Started  
**Duration**: 3-5 days  
**Goal**: Build the Go-side LISTEN/NOTIFY EventBus listener, wire compliance auto-evaluation into check-in flows, and add load testing infrastructure  
**Depends on**: Sprint 5 complete (PostgreSQL triggers exist since migration 000007, compliance engine has real evaluation logic)

---

## Why This Sprint

Sprint 4 installed PostgreSQL triggers that fire `pg_notify` on device, policy, command, assignment, and compliance events. Sprint 5 built the compliance evaluation engine with real policy checks. But nothing connects them — compliance only evaluates when explicitly called via the API. The EventBus listener is the missing piece that makes compliance reactive.

The dashboard (Sprint 5c) depends on live compliance data. Without the EventBus, the compliance view shows stale results.

---

## Tasks

| ID | Task | Effort | Dependencies |
|---|---|---|---|
| S5b-01 | EventBus LISTEN/NOTIFY Go listener | 1-2 days | Migration 000007 triggers |
| S5b-02 | Wire compliance evaluation into check-in handlers | 1 day | S5-09 compliance engine |
| S5b-03 | Device state parsing in check-in handlers | 1 day | S5b-02 |
| S5b-04 | Load testing framework (k6 scenarios) | 0.5-1 day | All API endpoints |

### S5b-01: EventBus LISTEN/NOTIFY Listener

Build `internal/service/eventbus.go`:
- Dedicated long-lived connection using `sql.DB.Conn(ctx)` on **Writer pool DSN** (read replicas don't relay NOTIFY)
- Keep-alive with periodic ping to prevent connection timeout
- Subscribe to channels: `device_events`, `policy_events`, `command_events`, `compliance_events`
- Dispatch to registered subscriber functions
- Graceful shutdown (context cancellation)
- Multi-instance safe: all instances receive all events (fan-out)

**Technical note from Sprint 4**: `LISTEN` requires a dedicated connection — standard pool connections are recycled and would drop the subscription. Use `sql.DB.Conn(ctx)` or a separate `pgx` connection.

### S5b-02: Wire Compliance into Check-in

After a device check-in updates `platform_data`:
- Call `complianceService.EvaluateDevice()` for the device
- Can be done as a direct service call (simpler) or as an EventBus subscriber on `device_events` (decoupled)
- Recommendation: direct call first, migrate to EventBus subscriber later

### S5b-03: Device State Parsing

Enhance check-in handlers to extract security-relevant fields:
- **macOS**: Parse SecurityInfo response → `password_present`, `FileVaultEnabled`, `FirewallEnabled`, `SIPEnabled`
- **Windows**: Parse DevDetail/Policy CSP → `encryption_enabled`, `firewall_enabled`, `password_present`, `bitlocker_status`
- **Android**: Parse Management API device report → `encryption_enabled`, `password_present`

Store in `platform_data` with keys that the compliance engine already checks.

### S5b-04: Load Testing Framework

Create `tests/load/` with k6 scenarios:
- Enrollment burst: 100 devices in 2 minutes
- Steady state: 1000 devices checking in over 1 hour
- Admin dashboard: 10 concurrent admin sessions
- Policy deployment: assign policy to 1000 devices

---

## Definition of Done

- [ ] EventBus listener receives NOTIFY events from PostgreSQL triggers
- [ ] Compliance auto-evaluates after device check-in
- [ ] macOS/Windows/Android check-in handlers populate security state in platform_data
- [ ] Load test scenarios exist and run against local environment
- [ ] All existing tests pass

---

*Created: 2026-04-22 — Split from Sprint 5 deferred items*
