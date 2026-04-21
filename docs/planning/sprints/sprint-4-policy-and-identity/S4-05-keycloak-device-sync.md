# S4-05: Device Lifecycle Hooks

**Sprint**: 4 — Policy & Identity
**Parallel**: ✅ Yes (no external dependencies)
**Effort**: 1-2 days

## Objective

Wire device lifecycle events (unenrollment, wipe, delete) to extensible hook points so that Sprint 4c's Keycloak PSSO sync (and future integrations) can plug in without modifying handler code.

> **Rescoped 2026-04-20**: Keycloak PSSO admin client, device registry sync, and reconciliation moved to Sprint 4c. This task now focuses only on the hook infrastructure and making CheckOut/wipe/delete handlers call it.

## Tasks

### 1. Device Lifecycle Hook Interface
- Define a `DeviceLifecycleHook` interface: `OnUnenroll(ctx, device)`, `OnWipe(ctx, device)`, `OnDelete(ctx, device)`
- Register hooks as EventBus subscribers for `device.unenrolled`, `device.wiped`, `device.deleted`
- No-op by default — Sprint 4c adds the Keycloak hook implementation
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

## What Moved to Sprint 4c

The following are now part of Sprint 4c (Platform SSO):
- Keycloak PSSO admin client (`internal/auth/keycloak_client.go`)
- `DELETE /device/{serial}` calls to Keycloak
- Keycloak ↔ MDM reconciliation job
- All Keycloak service account configuration

## Acceptance Criteria

- [x] `DeviceLifecycleHook` interface defined with `OnUnenroll`, `OnWipe`, `OnDelete`
- [x] macOS CheckOut event updates device status and calls hooks
- [x] Wipe handler calls hooks after status update
- [x] Delete handler calls hooks after deletion
- [x] Audit log entry for each lifecycle event
- [x] Hook slice is empty by default (no external calls until 4c adds Keycloak hook)
