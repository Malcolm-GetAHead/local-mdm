# S3-02: Windows — Core CSPs

**Sprint**: 3 — Platform Features
**Parallel**: ✅ Yes
**Depends on**: S2-04 (OMA-DM sync working)
**Effort**: 5-6 days

## Tasks

### 1. CSP Framework
- Base CSP handler interface (Get, Set, Execute nodes)
- SyncML command generation for CSP operations (Add, Replace, Get, Delete)
- CSP registry (map URI paths to handlers)
- Files: `internal/platform/windows/csp/framework.go`

### 2. Policy CSP
- Password policies (length, complexity, expiration)
- Device lock settings (timeout, max failed attempts)
- Encryption requirements
- Files: `internal/platform/windows/csp/policy.go`

### 3. WiFi CSP
- WiFi profile deployment (SSID, security type, credentials)
- 802.1X support
- Profile removal
- Files: `internal/platform/windows/csp/wifi.go`

### 4. VPN CSP
- VPN profile deployment (IKEv2, SSTP, L2TP)
- Always-on VPN
- Profile removal
- Files: `internal/platform/windows/csp/vpn.go`

### 5. DeviceLock CSP
- Remote lock command
- Remote wipe (full reset)
- PIN reset
- Files: `internal/platform/windows/csp/devicelock.go`

### 6. EnterpriseModernAppManagement CSP
- App inventory query
- App installation (MSI, MSIX, appx)
- App removal
- Files: `internal/platform/windows/csp/app.go`

## Acceptance Criteria

- [ ] Password policy deployed and enforced on Windows device
- [ ] WiFi profile deployed, device connects to configured network
- [ ] VPN profile deployed
- [ ] Remote lock command locks Windows device
- [ ] App inventory returned via CSP query
