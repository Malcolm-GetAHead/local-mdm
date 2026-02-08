# S4-03: Compliance Engine

**Sprint**: 4 — Policy & Identity
**Parallel**: ⚠️ Needs S4-01 (policy model) and S4-02 (assignments)
**Effort**: 3-4 days

## Tasks

### 1. Compliance Evaluation
- Compare device state against assigned policies
- Per-device compliance status: compliant, non-compliant, unknown
- Per-policy compliance: which specific rules pass/fail
- Run on: device check-in, policy change, manual trigger
- Files: `internal/policy/compliance.go`

### 2. Compliance Reporting
- `GET /api/v1/compliance` — enterprise-wide compliance summary
- `GET /api/v1/devices/{id}/compliance` — per-device compliance detail
- Non-compliant device list with reasons
- Files: `internal/api/handlers/compliance.go`

### 3. Remediation Actions (basic)
- Notify admin (audit log entry)
- Auto-push missing policy on next check-in
- Mark device as non-compliant (future: block access)
- Files: `internal/policy/remediation.go`

## Acceptance Criteria

- [ ] Device with correct password policy shows as compliant
- [ ] Device missing required WiFi profile shows as non-compliant with reason
- [ ] Compliance summary shows counts per status
- [ ] Compliance re-evaluated on policy change
