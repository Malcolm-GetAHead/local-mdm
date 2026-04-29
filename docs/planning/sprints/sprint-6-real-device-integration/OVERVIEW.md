# Sprint 6: Real Device Integration

**Status**: 🔲 Not Started
**Branch**: `sprint-6/real-device-integration`
**Duration**: 1-2 weeks (iterative test-fix cycles, not feature development)
**Goal**: Enroll real devices (Windows VM, macOS VM, Android) into Local MDM and verify the full management loop end-to-end
**Depends on**: Sprint 5g (quality polish) — merged to main

---

## Context

Sprints 1-5 built the MDM platform against mocks, fixtures, and protocol specs. The mdmb simulator in Sprint 5c proved SCEP + check-in works, but no real device has ever enrolled. This sprint closes that gap.

This is an **iterative sprint** — the workflow is: attempt enrollment → diagnose failure → fix code → rebuild → reset VM → retry. Expect multiple cycles per platform. The deliverable is not a feature list but a working enrollment-to-compliance loop for each platform.

---

## Prerequisites (Owner Setup)

### VM Infrastructure
- [ ] Install UTM (or raw QEMU) on macOS host
- [ ] Create Windows 11 ARM VM template:
  - Enable PowerShell Remoting (`Enable-PSRemoting -Force`)
  - Enable WinRM over HTTP for local network (`winrm set winrm/config/service @{AllowUnencrypted="true"}`)
  - Note VM IP address
  - Create "clean" snapshot after setup
- [ ] Create macOS VM template (UTM supports macOS guests on Apple Silicon):
  - Enable SSH (`sudo systemsetup -setremotelogin on`)
  - Note VM IP address
  - Create "clean" snapshot after setup
- [ ] (Optional) Android: Google Cloud project with Android Management API enabled, service account key

### Network
- [ ] Local MDM Docker stack accessible from VMs (host IP on bridge network)
- [ ] NanoMDM accessible from macOS VM (for Apple MDM protocol)

---

## Phase 1: Windows (Highest Protocol Complexity)

### S6-01: Windows VM Enrollment Loop
**Approach**: Iterative — attempt enrollment, fix failures, reset VM, retry.

1. Start Docker stack (`make prod-up`)
2. Boot Windows VM from clean snapshot
3. Import Local MDM CA certificate into Windows trust store:
   ```powershell
   certutil -addstore Root \\host\share\ca.crt
   ```
4. Trigger MDM enrollment via PowerShell:
   ```powershell
   # Or via Settings > Accounts > Access work or school > Enroll only in device management
   Add-MdmRegistration -ServerUrl "https://<host-ip>:8080/enrollment/windows"
   ```
5. Monitor Event Viewer: `Applications and Services Logs → Microsoft → Windows → DeviceManagement-Enterprise-Diagnostics-Provider`
6. Monitor Local MDM server logs for incoming requests
7. Diagnose and fix protocol issues (MS-MDE2 discovery, WSTEP enrollment, OMA-DM sync)
8. Rebuild container, restore VM snapshot, retry

**Expected issues** (based on code review):
- Discovery endpoint SOAP envelope format may not match what Windows expects
- Certificate enrollment exchange (WSTEP) may have encoding issues
- OMA-DM SyncML session initialization may fail on first sync
- CSP URIs for device info collection may not match Windows version

**Verification**: Device appears in dashboard with platform_data populated from OMA-DM sync.

### S6-02: Windows Policy Push & Compliance
Once enrollment works:
1. Assign a security policy (password, encryption) to the enrolled device
2. Trigger OMA-DM sync (device polls on schedule, or force via `mdm /sync`)
3. Verify policy translates to correct CSP commands in SyncML
4. Verify compliance evaluation runs against real platform_data
5. Verify dashboard shows real compliance status

**Verification**: Policy assigned → device syncs → compliance evaluated → dashboard accurate.

---

## Phase 2: macOS (Depends on NanoMDM)

### S6-03: macOS VM Enrollment Loop
1. Boot macOS VM from clean snapshot
2. Import CA certificate into System keychain:
   ```bash
   sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain ca.crt
   ```
3. Install enrollment profile:
   ```bash
   sudo profiles -I -F enrollment.mobileconfig
   ```
4. Monitor: `log stream --predicate 'subsystem == "com.apple.ManagedClient"'`
5. Monitor NanoMDM and Local MDM logs
6. Diagnose SCEP certificate enrollment, check-in, TokenUpdate flow
7. Fix, rebuild, restore snapshot, retry

**Expected issues**:
- Enrollment profile format/signing may need adjustment
- SCEP challenge validation timing
- NanoMDM webhook forwarding to Local MDM may have payload format issues
- Without APNs, server cannot push commands — device must initiate check-in

**Verification**: Device appears in dashboard with serial, model, OS version from TokenUpdate.

### S6-04: macOS Compliance Evaluation
1. Assign policy to enrolled macOS device
2. Wait for device check-in (or trigger manually)
3. Verify platform_data contains real security posture (FileVault, firewall, password)
4. Verify compliance engine evaluates correctly against real data

**Note**: Pushing profiles/commands requires APNs certificate. This phase focuses on enrollment + check-in + compliance. Command push is deferred until APNs is available.

---

## Phase 3: Android (If Google Cloud Project Available)

### S6-05: Android Enterprise Setup
1. Configure Google Cloud project credentials in Local MDM config
2. Create enterprise binding via Android Management API
3. Generate enrollment token / QR code
4. Enroll physical device or emulator
5. Verify webhook events (enrollment, status report) reach Local MDM
6. Verify device appears in dashboard

### S6-06: Android Policy & Compliance
1. Assign policy to enrolled Android device
2. Verify policy translates to Management API format
3. Verify status report webhook updates platform_data
4. Verify compliance evaluation against real device state

---

## Bug Tracking

Each protocol fix discovered during testing should be committed individually with descriptive messages. Expected categories:

- **Protocol format bugs** — SOAP/SyncML/mobileconfig format mismatches
- **Certificate chain bugs** — trust, encoding, or path issues
- **Data mapping bugs** — platform_data fields not matching what real devices report
- **Compliance logic bugs** — evaluation assumptions that don't hold with real data
- **Timing/ordering bugs** — enrollment steps happening in unexpected order

---

## Success Criteria

| Platform | Enrolled | Syncs Data | Policy Applied | Compliance Evaluated |
|----------|----------|------------|----------------|---------------------|
| Windows  | ⚠️ Protocol verified (SOAP), native enrollment blocked by Azure AD requirement on Win11 | ⚠️ OMA-DM sync verified via SOAP | ☐ | ☐ |
| macOS    | ✅ Real device via SCEP + NanoMDM | ✅ 9 auto-queued commands, full data pipeline | ✅ Check-in only (no APNs push) | ✅ Real FileVault/Firewall evaluation |
| Android  | ☐ Not attempted (requires Google Cloud project) | ☐ | ☐ | ☐ |

### What was actually delivered (beyond original plan)

- **macOS full data pipeline**: SecurityInfo, DeviceInformation (35 queries), ProfileList, InstalledApplicationList, CertificateList, ManagedApplicationList, AvailableOSUpdates, OSUpdateStatus, UserList — all auto-queued on check-in with 15min cooldown
- **Dashboard tabs**: Compliance, Policies, Commands, Profiles, Applications, Certificates, Updates, Users, Extensions, Platform Details — all populated from real device data
- **Compliance against real data**: FileVault enabled → encryption pass, Firewall disabled → firewall fail
- **Command tracking**: Auto-queued commands show in Commands tab with sent → completed status
- **Force Check-in button**: Ready for APNs (shows friendly error without push cert)
- **Persistent CA**: Mounted from host, survives container rebuilds
- **nginx TLS proxy**: Port 443/8443 with CA-signed cert for Windows HTTPS requirement
- **NanoMDM v0.9.0 webhook integration**: Full plist parsing with base64 decode
- **Windows protocol fixes**: SOAP envelope wrapping, RSTRC wrapper, provisioning XML with certs, device ID mapping

### Windows enrollment blockers (for future sprint)

**RESOLVED** — Windows enrollment working via Settings UI ("Enroll only in device management"). Key fixes:
- XML namespace trailing slash (`enrollment/` → `enrollment`)
- Non-chunked HTTP responses (Content-Length headers required by MS-MDE2)
- CRL Distribution Point on server cert (Windows schannel requires revocation checking)
- Lenient CSR signing for Windows ASN.1 PrintableString characters
- Enterprise ID resolution from email local part (UUID) with Acme Corp default

**Remaining Windows work** (next sprint):
- OMA-DM device info queries (BitLocker, firewall, OS version CSPs)
- Windows compliance evaluation from real device data
- Enrollment token system (F-07) for secure enrollment

1. `RegisterDeviceWithManagement` API has COM threading requirement — fails with `0x80010106` from any programmatic context
2. Windows 11 Settings UI "Access work or school" requires Azure AD identity validation before MDM enrollment
3. PPKG format requires Windows ADK tooling (`icd.exe`) to produce valid packages — our ZIP-based generator is incomplete
4. Server-side protocol is fully verified and correct — the blocker is entirely device-side

---

## Out of Scope

- **APNs push certificate** — required for macOS command push, deferred until Apple Developer account is available
- **WNS push notifications** — Windows devices poll on schedule, push notifications are a future optimization
- **Production deployment** — this sprint uses local Docker stack + local VMs
- **CI/CD pipeline** — still local verification only
- **Platform SSO** — Sprint 7

---

*Created: 2026-04-27*
