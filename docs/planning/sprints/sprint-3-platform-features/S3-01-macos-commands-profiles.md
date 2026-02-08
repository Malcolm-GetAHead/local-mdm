# S3-01: macOS — MDM Commands & Configuration Profiles

**Sprint**: 3 — Platform Features
**Parallel**: ✅ Yes
**Depends on**: S2-01 (NanoMDM enrollment working)
**Effort**: 4-5 days

## Tasks

### 1. MDM Command Sending
- Queue raw Plist commands via NanoMDM enqueue API
- DeviceInformation, SecurityInfo, CertificateList, ProfileList, InstalledApplicationList
- InstallProfile, RemoveProfile
- InstallApplication, RemoveApplication
- DeviceLock, EraseDevice, RestartDevice, ShutDownDevice
- Files: `internal/platform/macos/commands.go`

### 2. Configuration Profile Generation
- WiFi profile (WPA2/WPA3, 802.1X)
- VPN profile (IKEv2, IPSec)
- Certificate profile (PKCS12, SCEP)
- Email profile (IMAP/Exchange)
- Restrictions profile
- Files: `internal/platform/macos/profiles/*.go`

### 3. Command Response Processing
- Parse command results from NanoMDM webhook (mdm.Connect events)
- Update device inventory from DeviceInformation responses
- Track profile installation status
- Track app installation status
- Files: `internal/platform/macos/responses.go`

### 4. Routes
- `POST /api/v1/devices/{id}/commands` — send command
- `GET /api/v1/devices/{id}/commands` — list command history
- `POST /api/v1/devices/{id}/profiles` — install profile
- `DELETE /api/v1/devices/{id}/profiles/{profile_id}` — remove profile

## Acceptance Criteria

- [ ] DeviceInformation command returns and updates device record
- [ ] WiFi profile installs successfully on macOS device
- [ ] Profile can be removed via RemoveProfile command
- [ ] Command history shows status (pending, acknowledged, error)
