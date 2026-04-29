# F-01: Real Device Testing

**Priority**: High  
**Effort**: Remaining ~2 days (Android + Windows policy/command verification)  
**Status**: Partially complete — Sprint 6 delivered macOS full pipeline + Windows enrollment

---

## Completed (Sprint 5c + Sprint 6)

### Test Infrastructure ✅
- UTM VMs: macOS 26 and Windows 11 ARM, with template snapshots
- Passwordless SSH to macOS VM, WinRM to Windows VM
- nginx TLS proxy for HTTPS (port 8443), CA cert trusted by VMs
- `scripts/restore_vms.sh` for snapshot restore between test runs
- See `tests/device-testing/VM_SETUP.md` for full setup guide

### macOS ✅
- Real macOS VM enrolled via Safari profile download
- SCEP certificate enrollment (full PKCS#7 PKCSReq/CertRep)
- Authenticate + TokenUpdate check-in with Mdm-Signature verification
- 9 auto-queued commands on each check-in (SecurityInfo, DeviceInformation with 35 queries, ProfileList, InstalledApplicationList, CertificateList, ManagedApplicationList, AvailableOSUpdates, OSUpdateStatus, UserList)
- SecurityInfo parsing → compliance engine (FileVault, firewall, passcode)
- 35+ device fields populated in platform_data
- mdmb simulator: 5 concurrent enrollments (race-detector clean)

### Windows ✅ (partial)
- Windows 11 VM enrolled via Settings → "Enroll only in device management"
- MS-MDE2 protocol: Discovery, Policy, Enrollment over HTTPS
- OMA-DM SyncML sync sessions acknowledged
- Enterprise assignment via API
- Device appears in dashboard with certificate info

### Simulated Testing ✅
- mdmb device simulator (Sprint 5c): enrollment, SCEP, check-in
- Go e2e tests: full enrollment pipeline with NanoMDM in Docker
- 199 Playwright browser tests: dashboard CRUD, policy/group management

---

## Remaining

### Windows — OMA-DM Device Info Queries
- Sync handler acknowledges sessions but doesn't send CSP queries
- Need: BitLocker status, firewall state, OS version, device name via OMA-DM
- Without this, Windows compliance shows "unknown" for all settings
- Effort: ~0.5 day

### Android — Real Device/Emulator Testing
- **Prerequisites**: GCP project with Android Management API enabled, service account key in `secrets/google-service-account.json`
- QR code enrollment on emulator or physical device
- Policy enforcement verification (password, restrictions)
- App installation from managed Play
- Webhook event validation (Google → Local MDM)
- Effort: ~1 day

### macOS — APNs Push (Blocked)
- Requires Apple Push Certificate (Apple Developer account)
- Without APNs, server cannot push commands — devices only sync on reboot
- DEP/ADE enrollment requires Apple Business Manager account
- Not required for MVP — manual check-in via reboot is sufficient

### Cross-Platform
- Policy deployment verification across all three platforms
- Command execution tests (lock, wipe) on real devices
- Device compatibility matrix (currently untested: Windows 10, macOS < 26, Android < 14)

---

## Acceptance Criteria

- [x] macOS device enrolls via .mobileconfig profile
- [x] macOS device reports inventory (35+ fields)
- [x] macOS compliance evaluates correctly (FileVault, firewall, passcode)
- [x] Windows device enrolls via Settings UI
- [x] Windows device completes OMA-DM sync
- [ ] Windows device reports BitLocker/firewall/OS via OMA-DM queries
- [ ] Android device enrolls via QR code
- [ ] Android device enforces password policy
- [ ] All three platforms respond to lock command
- [ ] Compatibility matrix documented with tested versions

---

## References

- [VM Setup Guide](../../tests/device-testing/VM_SETUP.md)
- [Device Testing Quickstart](../../tests/device-testing/QUICKSTART.md)
- [Sprint 6 Plan](../sprints/sprint-6-real-device-integration/)
