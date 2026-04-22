# S4-03: Compliance Engine

**Sprint**: 4 — Policy & Identity
**Parallel**: ⚠️ Needs S4-01 (policy model) and S4-02 (assignments)
**Effort**: 3-4 days

## Tasks

### 1. Compliance Evaluation
- Compare device state against assigned policies
- Per-device compliance status: compliant, non-compliant, unknown
- Per-policy compliance: which specific rules pass/fail
- Triggered via EventBus subscribers:
  - `device.checkin` → re-evaluate after device state update
  - `policy.assigned` / `policy.updated` → evaluate affected devices
  - Manual trigger via API endpoint
- Files: `internal/service/compliance.go`

### 2. Compliance Reporting
- `GET /api/v1/compliance` — enterprise-wide compliance summary
- `GET /api/v1/devices/{id}/compliance` — per-device compliance detail
- Non-compliant device list with reasons
- Handlers are thin — call ComplianceService, format response
- Files: `internal/api/handlers.go` (new handler methods)

### 3. Remediation Actions (basic)
- Notify admin (audit log entry)
- Auto-push missing policy on next check-in
- Mark device as non-compliant (future: block access)
- Files: `internal/service/compliance.go` (remediation methods on ComplianceService)

## Acceptance Criteria

- [ ] Device with correct password policy shows as compliant (deferred to S5-09 — needs device state parsing)
- [ ] Device missing required WiFi profile shows as non-compliant with reason (deferred to S5-09)
- [x] Compliance summary shows counts per status
- [ ] Compliance re-evaluated on policy change (deferred to Sprint 5b — EventBus listener)
