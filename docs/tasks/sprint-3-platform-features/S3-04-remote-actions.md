# S3-04: Remote Actions (Lock, Wipe, Restart)

**Sprint**: 3 — Platform Features
**Parallel**: ⚠️ Can start once any one platform's command infra works
**Depends on**: S3-01, S3-02, or S3-03 (at least one)
**Effort**: 3-4 days

## Objective

Unified API for remote device actions that dispatches to the correct platform handler.

## Tasks

### 1. Action Dispatcher
- `POST /api/v1/devices/{id}/lock` → routes to macOS/Windows/Android handler
- `POST /api/v1/devices/{id}/wipe` → routes to platform handler
- `POST /api/v1/devices/{id}/restart` → routes to platform handler (macOS/Android only)
- Platform detection from device record
- Files: `internal/service/actions.go`

### 2. Action Tracking
- Store action requests in DB (who, what, when, status)
- Update status as platform reports result
- `GET /api/v1/devices/{id}/actions` — action history
- Files: `internal/service/action_tracking.go`

### 3. API Handlers
- Files: `internal/api/handlers/actions.go`

## Acceptance Criteria

- [ ] `POST /api/v1/devices/{id}/lock` works for all three platforms
- [ ] `POST /api/v1/devices/{id}/wipe` works for all three platforms
- [ ] Action history shows status progression
- [ ] Audit log entry created for each action
