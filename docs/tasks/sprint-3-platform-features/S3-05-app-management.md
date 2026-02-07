# S3-05: App Management Service

**Sprint**: 3 — Platform Features
**Parallel**: ⚠️ Can start once macOS or Android app commands work
**Depends on**: S3-01 (macOS), S3-03 (Android)
**Effort**: 3-4 days

## Objective

Unified app management: deploy, remove, and inventory apps across platforms.

## Tasks

### 1. App Catalog
- Store app definitions (name, platform, identifier, version, install type)
- Required vs available apps
- Per-enterprise app catalog
- Files: `internal/service/apps.go`, `internal/repository/app.go`

### 2. App Deployment
- Assign app to device or group
- Dispatch to platform handler (macOS InstallApplication, Android managed Play, Windows CSP)
- Track installation status
- Files: `internal/service/app_deploy.go`

### 3. App Inventory
- Collect installed apps from device reports
- Compare against required apps for compliance
- Files: `internal/service/app_inventory.go`

### 4. API Handlers
- `GET/POST /api/v1/apps` — catalog CRUD
- `POST /api/v1/apps/{id}/deploy` — deploy to devices/groups
- `GET /api/v1/devices/{id}/apps` — installed apps on device
- Files: `internal/api/handlers/apps.go`

## Acceptance Criteria

- [ ] App added to catalog
- [ ] App deployed to macOS device via InstallApplication
- [ ] App deployed to Android device via managed Play
- [ ] Installed app list collected from devices
- [ ] Missing required app flagged as non-compliant
