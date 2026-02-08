# F-06: User Documentation & Training Materials

**Priority**: Low  
**Effort**: 2-3 days  
**Score Impact**: +0.07 points  
**Status**: Post-launch

---

## Gap Analysis

### Current State
- Technical documentation (architecture, API, database)
- Developer setup guide (S1-02)
- Deployment guide (S5-04)
- API documentation (S5-03)

### Missing
- End-user enrollment guides (with screenshots)
- Video tutorials for enrollment
- Admin user guide (not just API docs)
- Troubleshooting FAQ
- Migration guide from other MDMs (step-by-step)
- Release notes template
- Changelog automation

### Impact
Without user documentation:
- Higher support burden
- Slower user adoption
- More enrollment failures
- Frustrated administrators
- Difficult migrations from other MDMs

---

## Proposed Solution

### 1. End-User Enrollment Guides

**Windows Enrollment Guide**:
```markdown
# Enrolling Your Windows Device

## Prerequisites
- Windows 10 version 1903 or later, or Windows 11
- Administrator access to your device
- Enrollment email from your IT administrator

## Step-by-Step Instructions

### Step 1: Open Settings
1. Click the Start menu
2. Click the Settings icon (gear icon)
3. Click "Accounts"

[Screenshot: Windows Settings with Accounts highlighted]

### Step 2: Access Work or School
1. Click "Access work or school" in the left sidebar
2. Click "Connect"

[Screenshot: Access work or school page with Connect button]

### Step 3: Enter Your Email
1. Enter your work email address
2. Click "Next"

[Screenshot: Email entry dialog]

### Step 4: Complete Enrollment
1. Follow the on-screen prompts
2. Enter your password when prompted
3. Accept the terms and conditions
4. Wait for enrollment to complete (1-2 minutes)

[Screenshot: Enrollment progress]

### Step 5: Verify Enrollment
1. You should see "Connected to [Organization]'s MDM"
2. Your device is now managed

[Screenshot: Successfully enrolled device]

## Troubleshooting

**Problem**: "Can't connect to the server"
**Solution**: Check your internet connection and try again

**Problem**: "Invalid email address"
**Solution**: Verify you're using your work email, not personal email

**Problem**: "Enrollment failed"
**Solution**: Contact your IT administrator with error code
```

**macOS Enrollment Guide**:
```markdown
# Enrolling Your Mac

## Prerequisites
- macOS 12 (Monterey) or later
- Administrator access
- Enrollment profile from IT administrator

## Step-by-Step Instructions

### Step 1: Download Enrollment Profile
1. Open the enrollment email from your IT administrator
2. Click "Download Enrollment Profile"
3. Save the .mobileconfig file to your Downloads folder

[Screenshot: Email with download link]

### Step 2: Install Profile
1. Double-click the downloaded .mobileconfig file
2. System Preferences will open automatically
3. Click "Continue" to install the profile

[Screenshot: Profile installation dialog]

### Step 3: Authenticate
1. Enter your Mac password when prompted
2. Click "Install"
3. Enter your password again to confirm

[Screenshot: Password prompt]

### Step 4: Complete Enrollment
1. Wait for enrollment to complete (30-60 seconds)
2. You'll see "Profile Installed" confirmation

[Screenshot: Success message]

### Step 5: Verify Enrollment
1. Open System Preferences
2. Click "Profiles"
3. You should see "MDM Profile" listed

[Screenshot: Profiles pane with MDM profile]

## Troubleshooting

**Problem**: "Profile cannot be installed"
**Solution**: Ensure you're running macOS 12 or later

**Problem**: "Profile is not signed"
**Solution**: Contact IT administrator for a new profile

**Problem**: "Installation failed"
**Solution**: Check System Preferences > Profiles for error details
```

**Android Enrollment Guide**:
```markdown
# Enrolling Your Android Device

## Prerequisites
- Android 11 or later
- QR code from IT administrator
- Factory reset device (for fully managed mode)

## Step-by-Step Instructions (Work Profile)

### Step 1: Open Camera
1. Open your device's camera app
2. Point camera at QR code from IT administrator

[Screenshot: QR code scanning]

### Step 2: Tap Notification
1. Tap the notification that appears
2. Tap "Set up work profile"

[Screenshot: Setup notification]

### Step 3: Install Work Apps
1. Follow on-screen prompts
2. Install required work apps
3. Wait for setup to complete (2-3 minutes)

[Screenshot: App installation progress]

### Step 4: Complete Setup
1. Set up work profile password (if required)
2. Accept work policies
3. Tap "Done"

[Screenshot: Setup complete]

### Step 5: Verify Enrollment
1. Look for briefcase icon on work apps
2. Swipe down to see "Work profile is on"

[Screenshot: Work profile indicator]

## Troubleshooting

**Problem**: "QR code not recognized"
**Solution**: Ensure good lighting and hold camera steady

**Problem**: "Device not compatible"
**Solution**: Check Android version (must be 11+)

**Problem**: "Enrollment failed"
**Solution**: Factory reset device and try again
```

### 2. Video Tutorials

**Scripts**:

**Windows Enrollment (2-3 minutes)**:
```
[0:00] Introduction
"Hi, I'm going to show you how to enroll your Windows device in your organization's MDM system."

[0:10] Prerequisites
"Before we start, make sure you have Windows 10 or 11, and you've received an enrollment email from your IT team."

[0:20] Step 1: Open Settings
[Screen recording: Opening Settings > Accounts]

[0:40] Step 2: Connect
[Screen recording: Clicking Connect, entering email]

[1:20] Step 3: Complete Enrollment
[Screen recording: Following prompts, entering password]

[2:00] Verification
"And that's it! Your device is now enrolled and managed."

[2:10] Troubleshooting
"If you run into any issues, check the troubleshooting guide or contact IT support."

[2:20] Outro
"Thanks for watching!"
```

**Production**:
- Screen recording with voiceover
- Captions for accessibility
- 1080p resolution
- Upload to YouTube, Vimeo, or internal portal

### 3. Admin User Guide

**Table of Contents**:
```markdown
# Local MDM Administrator Guide

## Getting Started
- Logging in
- Dashboard overview
- Navigation

## Device Management
- Viewing device inventory
- Device details and status
- Enrolling devices
- Unenrolling devices
- Remote actions (lock, wipe)

## Policy Management
- Creating policies
- Policy types (WiFi, VPN, security)
- Assigning policies to devices
- Policy compliance monitoring

## User Management
- Creating admin users
- Assigning roles and permissions
- Managing API tokens

## Reporting
- Device inventory reports
- Compliance reports
- Audit logs
- Exporting data

## Troubleshooting
- Common issues and solutions
- Viewing device logs
- Contacting support

## Appendix
- Keyboard shortcuts
- API reference
- Glossary
```

**Example Section**:
```markdown
## Creating a WiFi Policy

A WiFi policy allows you to automatically configure WiFi settings on managed devices.

### Step 1: Navigate to Policies
1. Click "Policies" in the left sidebar
2. Click "Create Policy" button

[Screenshot: Policies page]

### Step 2: Select Policy Type
1. Select "WiFi" from the policy type dropdown
2. Click "Next"

[Screenshot: Policy type selection]

### Step 3: Configure WiFi Settings
1. Enter SSID (network name)
2. Select security type (WPA2-Enterprise recommended)
3. Enter authentication details
4. Click "Create"

[Screenshot: WiFi configuration form]

### Step 4: Assign to Devices
1. Select devices or device groups
2. Click "Assign Policy"
3. Policy will be deployed within 5 minutes

[Screenshot: Device assignment]

### Verification
Devices will automatically connect to the configured WiFi network.

### Troubleshooting
- **Device not connecting**: Verify SSID and password are correct
- **Authentication failed**: Check certificate configuration
- **Policy not applied**: Check device compliance status
```

### 4. Troubleshooting FAQ

```markdown
# Frequently Asked Questions

## Enrollment Issues

**Q: Windows enrollment fails with "Can't connect to the server"**
A: This usually indicates a network issue. Check:
- Internet connection is active
- Firewall isn't blocking MDM server
- MDM server URL is correct
- Try again in a few minutes

**Q: macOS profile installation fails**
A: Common causes:
- Profile is expired (request new profile from IT)
- macOS version too old (upgrade to macOS 12+)
- Another MDM profile already installed (remove it first)

**Q: Android QR code not scanning**
A: Try these steps:
- Ensure good lighting
- Hold camera steady
- Clean camera lens
- Try manual entry instead

## Policy Issues

**Q: WiFi policy not applying to device**
A: Check:
- Device is enrolled and compliant
- Policy is assigned to device
- Device has checked in recently (< 24 hours)
- WiFi settings are correct

**Q: Device shows as non-compliant**
A: View compliance details:
1. Click on device
2. Go to "Compliance" tab
3. Review failed checks
4. Remediate issues or adjust policy

## Command Issues

**Q: Lock command not working**
A: Verify:
- Device is online and connected
- Device has checked in recently
- Command was sent successfully (check command history)
- Device supports lock command

**Q: Wipe command failed**
A: This is a critical operation. Check:
- Device is online
- Wipe command was confirmed
- Device has sufficient battery
- Contact support if issue persists

## General Issues

**Q: Device not checking in**
A: Possible causes:
- Device is offline
- Device was unenrolled
- Network connectivity issues
- MDM profile was removed

**Q: Dashboard not loading**
A: Try:
- Refresh browser (Ctrl+F5 or Cmd+Shift+R)
- Clear browser cache
- Try different browser
- Check if server is online (status.example.com)
```

### 5. Migration Guides

**From Jamf Pro to Local MDM**:
```markdown
# Migrating from Jamf Pro

## Overview
This guide helps you migrate macOS devices from Jamf Pro to Local MDM.

## Prerequisites
- Local MDM server set up and running
- Admin access to Jamf Pro
- Communication plan for end users

## Migration Steps

### Phase 1: Preparation (1 week)
1. Export device inventory from Jamf Pro
2. Import devices into Local MDM (CSV import)
3. Recreate policies in Local MDM
4. Test enrollment with pilot devices (5-10 devices)

### Phase 2: Pilot (1-2 weeks)
1. Unenroll pilot devices from Jamf Pro
2. Enroll pilot devices in Local MDM
3. Verify policies apply correctly
4. Gather feedback from pilot users
5. Adjust policies as needed

### Phase 3: Rollout (2-4 weeks)
1. Communicate migration plan to all users
2. Unenroll devices from Jamf Pro in batches
3. Enroll devices in Local MDM
4. Monitor for issues
5. Provide support for users

### Phase 4: Cleanup (1 week)
1. Verify all devices migrated
2. Decommission Jamf Pro server
3. Update documentation
4. Conduct post-migration review

## Policy Mapping

| Jamf Pro | Local MDM |
|----------|-----------|
| Configuration Profile | Configuration Profile |
| Restrictions | Security Policy |
| WiFi | WiFi Policy |
| VPN | VPN Policy |
| App Deployment | App Management |

## Troubleshooting

**Issue**: Device won't unenroll from Jamf
**Solution**: Use Jamf Pro's "Remove MDM Profile" command

**Issue**: Policies not applying after migration
**Solution**: Verify policy assignments in Local MDM

**Issue**: Users can't enroll in Local MDM
**Solution**: Ensure Jamf profile is fully removed first
```

### 6. Release Notes Template

```markdown
# Local MDM v1.1.0 Release Notes

**Release Date**: 2026-03-15  
**Type**: Minor Release

## What's New

### Features
- **Windows Provisioning Packages**: Generate .ppkg files for bulk enrollment
- **Enhanced Compliance Reporting**: New compliance dashboard with drill-down
- **API Rate Limiting**: Configurable rate limits per tenant

### Improvements
- Faster device enrollment (30% reduction in enrollment time)
- Improved error messages for failed policy deployments
- Better handling of offline devices

### Bug Fixes
- Fixed issue where WiFi policies weren't applying to Windows 11 devices
- Resolved memory leak in Android webhook handler
- Corrected timezone handling in audit logs

## Breaking Changes
None

## Upgrade Instructions

### Docker Compose
```bash
docker-compose pull
docker-compose up -d
```

### Kubernetes
```bash
helm upgrade localmdm ./helm/localmdm --version 1.1.0
```

## Known Issues
- macOS Platform SSO requires Keycloak 22+ (will be fixed in v1.2.0)
- Android work profile enrollment may fail on Samsung devices with Knox (investigating)

## Deprecations
- `/api/v1/devices/legacy` endpoint will be removed in v2.0.0 (use `/api/v1/devices` instead)

## Security Updates
- Updated dependencies to address CVE-2026-1234
- Improved input validation for policy configurations

## Documentation
- [Upgrade Guide](https://docs.localmdm.io/upgrade/v1.1.0)
- [API Changes](https://docs.localmdm.io/api/v1.1.0)
- [Migration Guide](https://docs.localmdm.io/migration/v1.1.0)

## Support
- [GitHub Issues](https://github.com/malcolm-getahead/local-mdm/issues)
- [Community Forum](https://community.localmdm.io)
- [Email Support](mailto:support@localmdm.io)
```

---

## Implementation Tasks

### Task 1: Enrollment Guides (1 day)
- Write Windows enrollment guide with screenshots
- Write macOS enrollment guide with screenshots
- Write Android enrollment guide with screenshots
- Create PDF versions

### Task 2: Video Tutorials (1 day)
- Script videos
- Record screen captures
- Add voiceover
- Edit and produce
- Upload to video platform

### Task 3: Admin User Guide (0.5 days)
- Write comprehensive admin guide
- Add screenshots for each section
- Create searchable HTML version
- Generate PDF version

### Task 4: FAQ & Troubleshooting (0.5 days)
- Compile common issues
- Write solutions
- Organize by category
- Add search functionality

### Task 5: Migration Guides (0.5 days)
- Write Jamf Pro migration guide
- Write Intune migration guide
- Write Workspace ONE migration guide
- Create migration checklists

---

## Acceptance Criteria

- [ ] Enrollment guides created for all three platforms
- [ ] Video tutorials produced and published
- [ ] Admin user guide covers all major features
- [ ] FAQ has at least 20 common questions
- [ ] Migration guides for top 3 MDM platforms
- [ ] Release notes template created
- [ ] All documentation reviewed for accuracy
- [ ] Documentation published to docs site

---

## Documentation Site

**Structure**:
```
docs.localmdm.io/
├── getting-started/
│   ├── windows-enrollment
│   ├── macos-enrollment
│   └── android-enrollment
├── admin-guide/
│   ├── device-management
│   ├── policy-management
│   └── reporting
├── troubleshooting/
│   ├── faq
│   └── common-issues
├── migration/
│   ├── from-jamf
│   ├── from-intune
│   └── from-workspace-one
├── api/
│   └── reference
└── release-notes/
    └── v1.0.0
```

**Technology**: MkDocs, Docusaurus, or GitBook

---

## Future Enhancements

- Interactive tutorials (guided walkthroughs)
- In-app help and tooltips
- Chatbot for common questions
- Community forum
- Knowledge base with search
- Localization (multiple languages)

---

## References

- [S5-03: API Documentation](../sprint-5-ui-and-polish/S5-03-api-docs.md)
- [S5-04: Deployment Guide](../sprint-5-ui-and-polish/S5-04-deployment.md)
