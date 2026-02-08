# S3-03: Android — Policies & App Management

**Sprint**: 3 — Platform Features
**Parallel**: ✅ Yes
**Depends on**: S2-05 (Android enrollment working)
**Effort**: 4-5 days

## Tasks

### 1. Policy Translation
- Translate unified policy model → Android Management API policy JSON
- Password requirements, screen lock, encryption
- Device restrictions (camera, USB, Bluetooth)
- Network configuration (WiFi, VPN)
- Files: `internal/platform/android/policy.go`, `internal/platform/android/translator.go`

### 2. Policy Deployment
- Create/update policies via Android Management API
- Apply policy to enrolled devices
- Policy compliance status from device reports
- Files: `internal/platform/android/policy_deploy.go`

### 3. App Management
- Add apps from managed Google Play
- Deploy required apps to devices
- Remove apps
- Managed app configuration (key-value)
- Files: `internal/platform/android/apps.go`

### 4. Device Commands
- Lock device
- Wipe device (full / work profile only)
- Reboot device
- Files: `internal/platform/android/commands.go`

## Acceptance Criteria

- [ ] Password policy enforced on Android device
- [ ] App installed from managed Google Play
- [ ] App removed from device
- [ ] Device lock command works
- [ ] Policy compliance status reported
