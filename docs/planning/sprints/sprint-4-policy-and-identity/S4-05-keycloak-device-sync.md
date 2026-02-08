# S4-05: Keycloak Device Lifecycle Sync

**Sprint**: 4 — Policy & Identity
**Parallel**: ⚠️ Needs S4-04 (PSSO working) and S2-06 (device service)
**Effort**: 2-3 days

## Objective

Sync device lifecycle events between Local MDM and Keycloak's PSSO device registry.

## Tasks

### 1. Keycloak Admin Client
- Service account with `psso-admin` client + `mac-admin` role
- HTTP client for Keycloak PSSO device API
- `GET /device` — list devices
- `GET /device/{serial}` — query device
- `DELETE /device/{serial}` — remove device
- Files: `internal/auth/keycloak_client.go`

### 2. Device Unenrollment Hook
- When macOS device unenrolls (CheckOut event or admin delete):
  - Call Keycloak `DELETE /device/{serial}` to revoke PSSO registration
  - Log action in audit trail
- Files: update `internal/platform/macos/webhook.go`, `internal/service/device.go`

### 3. Device Wipe Hook
- When macOS device is wiped via MDM:
  - Call Keycloak `DELETE /device/{serial}`
- Files: update `internal/service/actions.go`

### 4. Reconciliation (optional)
- Periodic job: compare MDM enrolled devices vs Keycloak registered devices
- Flag orphaned Keycloak registrations (device unenrolled from MDM but still in Keycloak)
- Files: `internal/service/keycloak_reconcile.go`

## Acceptance Criteria

- [ ] Unenrolling macOS device removes it from Keycloak PSSO
- [ ] Wiping macOS device removes it from Keycloak PSSO
- [ ] Audit log records Keycloak device removal
- [ ] Reconciliation identifies orphaned registrations
