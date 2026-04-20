# S5-10: Migrate Remaining Handlers to Service Layer

**Sprint**: 5 — Backend Polish  
**Parallel**: ✅ Yes (should complete before S5-08 CLI Tools)  
**Depends on**: Sprint 4 (service layer pattern established)  
**Effort**: 2-3 days

## Problem

Sprint 4 introduced `internal/service/` with PolicyService, GroupService, ComplianceService, and LifecycleService. New Sprint 4 handlers call services. But Sprint 2/3 handlers still call repositories directly:

- Device CRUD (create, get, list, update, delete)
- Device actions (lock, wipe, restart) — partially migrated (wipe/delete call lifecycle hooks)
- App management (CRUD, deploy)
- Enrollment flows (macOS, Windows, Android)
- Certificate listing

This means the CLI (S5-08) can't reuse business logic without duplicating handler code or importing `net/http`.

## Tasks

### 1. DeviceService (new)
Extract from handlers into `internal/service/device.go`:
- `Create`, `Get`, `List`, `Update`, `Delete`
- `Lock`, `Wipe`, `Restart` (command creation + dispatch + lifecycle hooks)
- Handlers become thin: parse request → call service → format response

### 2. AppService (new)
Extract from handlers into `internal/service/app.go`:
- `Create`, `Get`, `List`, `Update`, `Delete`
- `Deploy` (platform-specific install flow)

### 3. Update Handlers
Rewrite existing handlers to call services instead of repos. No behavior change — purely structural.

### 4. Verify CLI Compatibility
Confirm all service methods are transport-agnostic (no `net/http` imports in service package).

## What NOT to Migrate

- Enrollment flows — these are platform-specific and already in platform service packages
- Certificate operations — simple repo calls, no business logic to extract
- Auth handlers (login/refresh) — Keycloak-specific, not reusable from CLI

## Acceptance Criteria

- [ ] DeviceService and AppService created in `internal/service/`
- [ ] All device and app handlers call services, not repos
- [ ] No `net/http` imports in service package
- [ ] All existing tests pass (no behavior change)
- [ ] S5-08 CLI can import and call services directly

---

*Created 2026-04-20 during Sprint 4 retrospective.*
