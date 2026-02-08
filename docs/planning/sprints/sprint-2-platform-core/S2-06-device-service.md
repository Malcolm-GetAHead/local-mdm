# S2-06: Device Service Layer

**Sprint**: 2 — Platform Core
**Parallel**: ⚠️ Starts in parallel, integrates with platform tasks as they complete
**Effort**: 3-4 days

## Objective

Unified device service that abstracts platform differences. All platform modules register devices through this service.

## Tasks

### 1. Device Service
- Create device (called by platform enrollment handlers)
- Get device by ID, serial, UDID
- List devices with filtering (platform, enterprise, status, compliance)
- Update device info (called by platform inventory handlers)
- Update device status (enrolled, compliant, non-compliant, unenrolled)
- Soft delete device
- Files: `internal/service/device.go`

### 2. Device API Handlers
- Implement `GET /api/v1/devices` (list with pagination, filtering)
- Implement `GET /api/v1/devices/{id}` (detail with platform-specific data in JSONB)
- Implement `DELETE /api/v1/devices/{id}` (soft delete + trigger platform unenroll)
- Files: `internal/api/handlers/devices.go`

### 3. Enrollment Service
- Orchestrates enrollment across platforms
- Generates enrollment URLs/tokens per platform
- Tracks enrollment state
- Files: `internal/service/enrollment.go`

### 4. Audit Logging
- Log device lifecycle events (enroll, unenroll, lock, wipe, policy change)
- Structured audit entries with actor, action, target, timestamp
- Files: `internal/service/audit.go`

## Interfaces

```go
type DeviceService interface {
    Create(ctx context.Context, device *models.Device) error
    GetByID(ctx context.Context, id uuid.UUID) (*models.Device, error)
    List(ctx context.Context, enterpriseID uuid.UUID, filters DeviceFilters) ([]*models.Device, int, error)
    UpdateInfo(ctx context.Context, id uuid.UUID, info map[string]interface{}) error
    UpdateStatus(ctx context.Context, id uuid.UUID, status string) error
    Delete(ctx context.Context, id uuid.UUID) error
    Lock(ctx context.Context, id uuid.UUID) error
    Wipe(ctx context.Context, id uuid.UUID) error
}
```

## Acceptance Criteria

- [ ] Devices from all three platforms appear in unified device list
- [ ] Filtering by platform, status, enterprise works correctly
- [ ] Device detail includes platform-specific data in JSONB field
- [ ] Audit log entries created for all device lifecycle events
- [ ] Deleting a device triggers platform-specific unenrollment
