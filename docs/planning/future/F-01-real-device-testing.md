# F-01: Real Device Testing

**Priority**: High  
**Effort**: 3-4 days  
**Score Impact**: +0.30 points  
**Status**: Deferred for future discussion

---

## Gap Analysis

### Current State
- Unit tests with mocks (S1-07)
- Integration tests with test database
- E2E tests via API calls (S5-05)
- No testing with actual devices or VMs

### Missing
- Windows VM enrollment testing
- macOS VM enrollment and DEP testing
- Android emulator/device testing
- Cross-platform policy deployment verification
- Real device command execution (lock, wipe, restart)
- Device compatibility matrix

### Impact
Without real device testing:
- Protocol implementation bugs may go undetected
- Platform-specific edge cases missed
- Enrollment flows not validated end-to-end
- Policy translation errors not caught
- Command execution failures discovered in production

---

## Proposed Solution

### 1. Test Lab Setup
- Windows 11 VM (Hyper-V or VMware)
- macOS VM (VMware Fusion or Parallels)
- Android emulator (Android Studio AVD)
- Physical test devices (optional): 1 Windows laptop, 1 MacBook, 1 Android phone

### 2. Automated Device Tests
- Enrollment automation scripts
- Policy deployment verification
- Command execution validation
- Compliance checking
- Unenrollment cleanup

### 3. Test Scenarios

**Windows**:
- Manual enrollment via Settings → Access work or school
- Certificate-based authentication
- WiFi profile deployment
- VPN profile deployment
- Device lock command
- Device wipe command
- .ppkg enrollment (S3-06)

**macOS**:
- Manual enrollment via .mobileconfig profile
- DEP/ADE enrollment simulation
- Configuration profile installation
- App installation (VPP app)
- Device lock command
- Erase device command
- Platform SSO profile (S4-04)

**Android**:
- QR code enrollment
- Work profile creation
- Fully managed device enrollment
- Policy enforcement (password, restrictions)
- App installation from managed Play
- Device lock command
- Factory reset command

### 4. Compatibility Matrix

| OS | Versions | Enrollment | Policies | Commands | Status |
|----|----------|------------|----------|----------|--------|
| Windows 10 | 21H2, 22H2 | ✅ | ✅ | ✅ | Tested |
| Windows 11 | 22H2, 23H2 | ✅ | ✅ | ✅ | Tested |
| macOS | 12, 13, 14 | ✅ | ✅ | ✅ | Tested |
| Android | 11, 12, 13, 14 | ✅ | ✅ | ✅ | Tested |

---

## Implementation Tasks

### Task 1: Test Infrastructure (1 day)
- Set up Windows 11 VM
- Set up macOS VM (if possible)
- Set up Android emulator
- Configure network access to Local MDM server
- Install test certificates

### Task 2: Windows Device Tests (1 day)
- Automated enrollment script
- Policy deployment tests (WiFi, VPN, DeviceLock)
- Command execution tests (lock, wipe)
- .ppkg enrollment test
- Validation scripts

### Task 3: macOS Device Tests (1 day)

**Already completed in Sprint 5c** (mdmb device simulator):
- ✅ SCEP certificate enrollment (full PKCS#7 PKCSReq/CertRep)
- ✅ Enrollment profile generation and parsing (PayloadContent, SignMessage)
- ✅ Authenticate + TokenUpdate check-in with Mdm-Signature verification
- ✅ Device record creation with Serial, Name, Model, OS, Build
- ✅ 5 concurrent device enrollments (race-detector clean)

**Remaining for real Apple devices:**
- DEP/ADE enrollment (requires Apple Business Manager account)
- APNs push notification delivery (requires Apple Push Certificate)
- Profile deployment tests (WiFi, VPN, Certificate) on real hardware
- Command execution tests (lock, erase, app install) on real hardware
- Platform SSO test
- Edge cases mdmb doesn't simulate (user enrollment, Shared iPad)

### Task 4: Android Device Tests (1 day)
- **Configure Google Android Management API client** (prerequisite):
  - Enable Android Management API in GCP project
  - Create service account with Android Management API role
  - Generate JSON key file → `secrets/android-service-account.json`
  - Set `android.project_id` and `android.service_account_json` in config.yaml
  - Verify `android.NewClient()` initializes successfully
- QR code enrollment
- Policy enforcement tests
- App installation tests
- Command execution tests
- Webhook event validation (verify Google → Local MDM webhook delivery with real events)
- **Android platform unit test coverage** (currently 61.6%): Add HTTP client mocks for `client.go` (CreateEnterprise, CreateEnrollmentToken, CreatePolicy) and `commands.go` (LockDevice, WipeDevice, RebootDevice) — these are 0% because they make real API calls. Use `httptest.NewServer` to mock Google's API responses.

---

## Acceptance Criteria

- [ ] Windows device enrolls successfully via Settings
- [ ] Windows device receives and applies WiFi profile
- [ ] Windows device responds to lock command
- [ ] macOS device enrolls via .mobileconfig profile
- [ ] macOS device installs configuration profiles
- [ ] macOS device responds to lock and erase commands
- [ ] Android device enrolls via QR code
- [ ] Android device enforces password policy
- [ ] Android device installs managed app
- [ ] All three platforms report inventory correctly
- [ ] Compatibility matrix documented

---

## Tools & Frameworks

**VM Management**:
- Vagrant for automated VM provisioning
- Terraform for cloud-based test VMs (AWS, Azure)

**Test Automation**:
- Python scripts for enrollment automation
- Selenium for web-based enrollment flows
- ADB for Android device control
- PowerShell for Windows automation
- AppleScript/osascript for macOS automation

**CI/CD Integration**:
- GitHub Actions with self-hosted runners
- Scheduled nightly device tests
- Test result reporting

---

## Cost Considerations

**Free Options**:
- Windows 11 VM (evaluation license, 90 days)
- Android emulator (free with Android Studio)
- macOS VM (requires Mac hardware, licensing restrictions)

**Paid Options**:
- VMware Fusion/Workstation ($200-300)
- Physical test devices ($500-2000)
- Cloud-based device testing (AWS Device Farm, BrowserStack)

---

## Risks

**High Risk**:
- macOS VM licensing (Apple restricts virtualization to Mac hardware)
- DEP testing requires Apple Business Manager account
- Android emulator may not fully replicate device behavior

**Medium Risk**:
- VM performance issues (slow enrollment, timeouts)
- Network configuration complexity
- Certificate trust issues

**Mitigation**:
- Use physical Mac for macOS testing if VM not possible
- Partner with organization that has ABM for DEP testing
- Use physical Android device for final validation

---

## Future Enhancements

- Continuous device testing in CI/CD
- Device farm integration (AWS Device Farm, BrowserStack)
- Automated regression testing on every commit
- Performance testing with multiple devices enrolling simultaneously
- Chaos testing (network failures, server restarts during enrollment)

---

## References

- [S1-07: Testing Framework Setup](../sprint-1-foundation/S1-07-testing-framework.md)
- [S5-05: E2E Testing](../sprint-5-ui-and-polish/S5-05-e2e-testing.md)
- [Windows VM Setup Guide](https://developer.microsoft.com/en-us/windows/downloads/virtual-machines/)
- [Android Emulator Documentation](https://developer.android.com/studio/run/emulator)
