# S4-05: Device Lifecycle Hooks

**Sprint**: 4 — Policy & Identity
**Parallel**: ✅ Yes (no external dependencies)
**Effort**: 1-2 days

## Objective

Wire device lifecycle events (unenrollment, wipe, delete) to extensible hook points so that Sprint 4b's Keycloak PSSO sync (and future integrations) can plug in without modifying handler code.

> **Rescoped 2026-04-20**: Keycloak PSSO admin client, device registry sync, and reconciliation moved to Sprint 4b. This task now focuses only on the hook infrastructure and making CheckOut/wipe/delete handlers call it.

## Tasks

### 1. Device Lifecycle Hook Interface
- Define a `DeviceLifecycleHook` interface: `OnUnenroll(ctx, device)`, `OnWipe(ctx, device)`, `OnDelete(ctx, device)`
- Server holds a slice of hooks, iterates on each event
- No-op by default — Sprint 4b adds the Keycloak hook implementation
- Files: `internal/service/lifecycle.go`

### 2. Wire CheckOut Handler
- macOS `handleCheckOut()` in `webhook.go` currently just logs
- Update to: set device status to `unenrolled` + call lifecycle hooks
- Files: update `internal/platform/macos/webhook.go`

### 3. Wire Wipe/Delete Handlers
- `handleWipeDevice` and `handleDeleteDevice` in `handlers.go` already update status
- Add lifecycle hook calls after status update
- Files: update `internal/api/handlers.go`

### 4. Audit Integration
- Each hook call logs to audit trail with action `device.lifecycle.{event}`
- Files: update `internal/api/handlers.go`

## What Moved to Sprint 4b

The following are now part of Sprint 4b (Platform SSO):
- Keycloak PSSO admin client (`internal/auth/keycloak_client.go`)
- `DELETE /device/{serial}` calls to Keycloak
- Keycloak ↔ MDM reconciliation job
- All Keycloak service account configuration

## Acceptance Criteria

- [ ] `DeviceLifecycleHook` interface defined with `OnUnenroll`, `OnWipe`, `OnDelete`
- [ ] macOS CheckOut event updates device status and calls hooks
- [ ] Wipe handler calls hooks after status update
- [ ] Delete handler calls hooks after deletion
- [ ] Audit log entry for each lifecycle event
- [ ] Hook slice is empty by default (no external calls until 4b adds Keycloak hook)
