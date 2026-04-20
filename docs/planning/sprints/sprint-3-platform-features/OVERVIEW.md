# Sprint 3: Platform Features — Commands, Profiles & Apps

**Duration**: 2-3 weeks
**Goal**: Deploy policies, push profiles/commands, manage apps on all platforms
**Depends on**: Sprint 2 complete (devices can enroll)

## Tasks Overview

| ID | Task | Parallel? | Dependencies | Estimated Effort |
|---|---|---|---|---|
| S3-01 | [macOS: MDM Commands & Profiles](S3-01-macos-commands-profiles.md) | ✅ Yes | S2-01 | 4-5 days |
| S3-02 | [Windows: Core CSPs (Policy, WiFi, VPN, DeviceLock)](S3-02-windows-csps.md) | ✅ Yes | S2-04 | 5-6 days |
| S3-03 | [Android: Policies & App Management](S3-03-android-policies-apps.md) | ✅ Yes | S2-05 | 4-5 days |
| S3-04 | [Remote Actions (Lock, Wipe, Restart)](S3-04-remote-actions.md) | ⚠️ Partial | S3-01, S3-02, S3-03 (needs command infra per platform) | 3-4 days |
| S3-05 | [App Management Service](S3-05-app-management.md) | ⚠️ Partial | S3-01, S3-03 | 3-4 days |
| S3-06 | [Windows Provisioning Packages (.ppkg)](S3-06-windows-ppkg.md) | ⚠️ Partial | S2-03 | 2-3 days |

## Dependency Graph

```
S2 complete
    │
    ├── S3-01 (macOS commands) ──────┐
    ├── S3-02 (Windows CSPs) ────────┤──→ S3-04 (Remote Actions)
    ├── S3-03 (Android policies) ────┤
    │                                │
    ├── S3-01 ───────────────────────┤──→ S3-05 (App Management)
    └── S3-03 ───────────────────────┘
```

S3-01, S3-02, S3-03 are fully parallel. S3-04 and S3-05 can start once at least one platform's command infrastructure is working, then expand to other platforms as they complete.

## Definition of Done

- [x] macOS: InstallProfile, RemoveProfile, DeviceInformation, DeviceLock commands work
- [x] macOS: WiFi, VPN, Certificate configuration profiles can be generated and pushed
- [x] Windows: Policy, WiFi, VPN, DeviceLock CSPs deployed via OMA-DM
- [x] Android: Security policies enforced, apps installed from managed Play
- [x] Remote lock works on all three platforms via unified API
- [x] Remote wipe works on all three platforms via unified API
- [x] App install/remove works on macOS and Android

## Completion Notes

**Sprint 3 completed**: 2026-04-20

All 6 tasks delivered plus gap closure (PPKG signing with dev cert, platform dispatch wiring). Key deliverables:
- 12 macOS command builders with NanoMDM HTTP API integration
- Windows CSP framework with WNS push and SyncML Replace
- Android policy translation to Management API format
- Unified remote actions with command tracking and platform dispatch
- App management service with full CRUD and deploy API
- Windows provisioning packages with ICD XML generation and signing
