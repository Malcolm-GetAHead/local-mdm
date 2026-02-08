# Sprint 2 Pre-Flight Check - External Dependencies

**Date**: 2026-02-08  
**Sprint**: Sprint 2 - Platform Core  
**Purpose**: Identify required external accounts, credentials, and setup

---

## Summary

### ✅ Can Start Without External Accounts
Sprint 2 can begin with **local development/testing** using simulators and mock services.

### ⚠️ Real Device Testing Requires External Accounts
To test with **real devices**, you'll need platform-specific accounts (deferred to F-01).

---

## By Platform

### 🍎 macOS (S2-01, S2-02)

#### For Development/Testing ✅
**No external accounts required** - Can use:
- Local NanoMDM integration (library)
- Self-signed certificates for testing
- Simulator/VM for basic testing

#### For Real Device Enrollment ⚠️
**Required** (deferred to F-01: Real Device Testing):
1. **Apple Developer Account** ($99/year)
   - Purpose: APNs push certificate
   - Required for: Sending push notifications to devices
   - Alternative: Can develop without, but can't wake devices

2. **Apple Business Manager (ABM)** (Free, requires business verification)
   - Purpose: DEP/ADE automated enrollment
   - Required for: Zero-touch enrollment (S2-02)
   - Alternative: Manual enrollment works without ABM (S2-01)

**Setup Steps** (when ready):
```
1. Apple Developer Account:
   - Sign up at developer.apple.com
   - Create APNs push certificate via MDM CSR
   - Download .p12 certificate

2. Apple Business Manager:
   - Sign up at business.apple.com
   - Verify business (can take days/weeks)
   - Add MDM server
   - Assign devices
```

---

### 🪟 Windows (S2-03, S2-04)

#### For Development/Testing ✅
**No external accounts required** - Can use:
- Windows 10/11 VM or physical machine
- Local MS-MDE2 protocol implementation
- Self-signed certificates for testing

#### For Real Device Enrollment ✅
**No special accounts required**:
- Windows MDM uses standard protocols (MS-MDE2, OMA-DM)
- No cloud service dependencies
- Works with local MDM server

**Requirements**:
- Windows 10 Pro/Enterprise or Windows 11 Pro/Enterprise
- Device must trust your MDM server certificate
- Network connectivity to MDM server

---

### 🤖 Android (S2-05)

#### For Development/Testing ⚠️
**Google Cloud Account Required** (Free tier available):

1. **Google Cloud Project** (Free)
   - Purpose: Android Management API access
   - Required for: All Android management
   - Cost: Free tier sufficient for development

2. **Service Account** (Free)
   - Purpose: API authentication
   - Required for: API calls
   - Setup: Create in Google Cloud Console

**Setup Steps**:
```
1. Create Google Cloud Project:
   - Go to console.cloud.google.com
   - Create new project
   - Enable Android Management API

2. Create Service Account:
   - IAM & Admin → Service Accounts
   - Create service account
   - Grant "Android Management User" role
   - Create JSON key file
   - Download and store securely
```

#### For Real Device Enrollment ⚠️
**Google Workspace Account** (Optional, $6-18/user/month):
- Purpose: Work profile management, enterprise binding
- Required for: Work profiles, advanced features
- Alternative: Can use personal Google account for testing
- Note: Fully managed devices don't require Workspace

**Device Requirements**:
- Android 5.0+ (Lollipop)
- Factory reset for fully managed enrollment
- Google Play Services installed

---

## Required Now vs. Later

### ✅ Can Start Sprint 2 Now (No Accounts Needed)

**Development Approach**:
1. **macOS**: Build NanoMDM integration, test with self-signed certs
2. **Windows**: Build MS-MDE2 protocol, test with local VM
3. **Android**: Build API client structure, mock API responses

**Testing Strategy**:
- Unit tests (no external services)
- Integration tests with mocks
- Local protocol testing
- Simulator/VM testing

### ⚠️ Needed for Real Device Testing (F-01)

**When you're ready to test with real devices**:

| Platform | Account | Cost | Setup Time | Priority |
|----------|---------|------|------------|----------|
| Android | Google Cloud (free tier) | Free | 30 min | High |
| Android | Google Workspace (optional) | $6-18/user/mo | 1 hour | Low |
| macOS | Apple Developer | $99/year | 1 hour | Medium |
| macOS | Apple Business Manager | Free | 1-2 weeks | Low |
| Windows | None | Free | N/A | N/A |

---

## Recommended Approach

### Phase 1: Sprint 2 Development (Now)
**No external accounts needed**
- Build all platform integrations
- Write comprehensive tests
- Use mocks and simulators
- Validate protocol implementations

**Deliverables**:
- NanoMDM integration complete
- Windows MDM protocol working
- Android API client built
- All unit/integration tests passing

### Phase 2: Real Device Testing (F-01)
**Set up external accounts**
- Create Google Cloud project (Android)
- Sign up for Apple Developer (macOS push)
- Optional: Apple Business Manager (DEP)
- Optional: Google Workspace (work profiles)

**Deliverables**:
- Real Android device enrolled
- Real macOS device enrolled (manual)
- Real Windows device enrolled
- Optional: DEP/ADE enrollment

---

## Cost Summary

### Immediate (Sprint 2)
**Total: $0** - No accounts required for development

### Future (F-01 - Real Device Testing)

**Required**:
- Google Cloud: **Free** (free tier sufficient)

**Optional**:
- Apple Developer: **$99/year** (for APNs push)
- Apple Business Manager: **Free** (for DEP/ADE)
- Google Workspace: **$6-18/user/month** (for work profiles)

**Minimum to test all platforms**: **$0** (can test without paid accounts)
**Recommended for full features**: **$99/year** (Apple Developer only)

---

## Action Items

### Before Starting Sprint 2
- [ ] ✅ **None required** - Can start immediately

### During Sprint 2 (Optional)
- [ ] Create Google Cloud project (free, 30 min)
- [ ] Enable Android Management API (free, 5 min)
- [ ] Create service account and download JSON key (free, 10 min)

### Before F-01 (Real Device Testing)
- [ ] Sign up for Apple Developer ($99/year, 1 hour)
- [ ] Create APNs push certificate (30 min)
- [ ] Optional: Sign up for Apple Business Manager (free, 1-2 weeks)
- [ ] Optional: Sign up for Google Workspace ($6-18/user/month, 1 hour)

---

## Testing Strategy by Phase

### Sprint 2 (Development)
```
macOS:
  ✅ NanoMDM library integration
  ✅ Self-signed certificate testing
  ✅ Protocol validation
  ❌ Real device enrollment (needs APNs)

Windows:
  ✅ MS-MDE2 protocol implementation
  ✅ Local VM testing
  ✅ Real device enrollment (no accounts needed)

Android:
  ✅ API client development
  ✅ Mock API responses
  ✅ Protocol validation
  ⚠️ Real device enrollment (needs Google Cloud - free)
```

### F-01 (Real Device Testing)
```
macOS:
  ✅ Real device enrollment with APNs
  ✅ DEP/ADE enrollment (if ABM set up)

Android:
  ✅ Real device enrollment with QR code
  ✅ Work profile enrollment (if Workspace)
```

---

## Recommendation

### ✅ **Start Sprint 2 Now**

**No blockers** - All external accounts are optional for development.

**Optional during Sprint 2**:
- Set up Google Cloud (free, 30 min) for Android API testing
- This is the only account that's free and quick to set up

**Defer to F-01**:
- Apple Developer account ($99/year)
- Apple Business Manager (free but slow verification)
- Google Workspace (paid, optional)

---

## Questions?

**Q: Can we test Android without Google Cloud?**  
A: Yes, with mocks. But setting up Google Cloud (free) is recommended.

**Q: Can we test macOS without Apple Developer?**  
A: Yes, but can't send push notifications. Devices won't wake up automatically.

**Q: Can we test Windows without any accounts?**  
A: Yes, fully. Windows MDM is self-contained.

**Q: What's the minimum to test all three platforms?**  
A: $0 - Google Cloud is free, Windows needs nothing, macOS works without push.

---

**Status**: ✅ **Ready to start Sprint 2 without any external accounts**

**Recommendation**: Start Sprint 2 now, optionally set up Google Cloud (free) during development.
