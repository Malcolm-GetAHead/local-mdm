# S5-09: Device State Collection & Compliance Evaluation

**Sprint**: 5 — Backend Polish  
**Parallel**: ⚠️ Must complete before S5-02 (Reporting & Audit)  
**Depends on**: S4-03 (Compliance Engine infrastructure)  
**Effort**: 3-4 days

## Problem

Sprint 4's compliance engine has all the plumbing (policy resolution, evaluation loop, result storage, API endpoints) but `evaluatePolicy()` always returns `status: "unknown"` because there's no device state to compare against. Check-in handlers (Sprint 2/3) receive device state but only log it or update `last_seen` — they don't parse security posture into queryable fields.

## Tasks

### 1. Device State Parsing
Parse security-relevant fields from check-in responses and store in `platform_data` JSONB:

- **macOS**: SecurityInfo response → password policy compliance, FileVault status, firewall status, SIP status
- **Windows**: DevDetail CSP + Policy CSP results → password policy, BitLocker status, Windows Update status
- **Android**: Management API device report → password compliance, encryption status, policy compliance state

Update the existing check-in/sync handlers to extract and store these fields.

### 2. Structured Device State Schema
Define a common schema within `platform_data` for security state:

```json
{
  "security_state": {
    "password_compliant": true,
    "password_length": 12,
    "encryption_enabled": true,
    "firewall_enabled": true,
    "os_up_to_date": true,
    "last_security_scan": "2026-04-20T12:00:00Z"
  }
}
```

Platform-specific fields live alongside in `platform_data`; the `security_state` key is the common subset the compliance engine reads.

### 3. Real Compliance Evaluation Logic
Replace the placeholder `evaluatePolicy()` in `internal/service/compliance.go` with actual comparison:

- **Security policies**: compare `min_password_length` against `security_state.password_length`, `require_encryption` against `security_state.encryption_enabled`, etc.
- **WiFi policies**: check if expected SSID profile is installed (from profile list response)
- **Restriction policies**: check if restricted features are disabled (camera, screen capture, etc.)
- Return `compliant` / `non_compliant` with specific field-level details in the `details` JSONB

### 4. Wire Evaluation into Check-in Handlers
After device state is parsed and stored, trigger compliance evaluation:

- Direct service call from check-in handler (Sprint 5 approach)
- EventBus subscriber (when LISTEN/NOTIFY listener is wired in Sprint 5)

## Acceptance Criteria

- [ ] macOS SecurityInfo response parsed into `platform_data.security_state`
- [ ] Windows DevDetail CSP response parsed into `platform_data.security_state`
- [ ] Android device report parsed into `platform_data.security_state`
- [ ] `evaluatePolicy()` returns real `compliant`/`non_compliant` status with reasons
- [ ] Compliance evaluation runs after device check-in
- [ ] S5-02 compliance reports show real data

---

*Created 2026-04-20 during Sprint 4 retrospective.*
