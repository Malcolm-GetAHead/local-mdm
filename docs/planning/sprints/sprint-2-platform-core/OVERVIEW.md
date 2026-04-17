# Sprint 2: Platform Core — Enrollment & Inventory

**Duration**: 2-3 weeks
**Goal**: All three platforms can enroll devices and report inventory
**Depends on**: Sprint 1 complete

## Tasks Overview

| ID | Task | Parallel? | Dependencies | Estimated Effort |
|---|---|---|---|---|
| S2-01 | [macOS: NanoMDM Integration & Enrollment](S2-01-macos-nanomdm-enrollment.md) | ✅ Yes | S1 complete | 4-5 days |
| S2-02 | [macOS: NanoDEP Integration](S2-02-macos-nanodep.md) | ✅ Yes | S1 complete | 3-4 days |
| S2-03 | [Windows: Discovery & Enrollment (MS-MDE2)](S2-03-windows-discovery-enrollment.md) | ✅ Yes | S1 complete | 5-6 days |
| S2-04 | [Windows: OMA-DM Sync & DeviceInfo CSP](S2-04-windows-omadm-sync.md) | ⛔ Sequential | S2-03 | 4-5 days |
| S2-05 | [Android: Management API Client & Enrollment](S2-05-android-enrollment.md) | ✅ Yes | S1 complete | 4-5 days |
| S2-06 | [Device Service Layer](S2-06-device-service.md) | ⚠️ Partial | S1-01, starts parallel, integrates with S2-01/03/05 | 3-4 days |

## Dependency Graph

```
Sprint 1 (all complete)
    │
    ├── S2-01 (macOS NanoMDM) ──────┐
    ├── S2-02 (macOS NanoDEP) ──────┤
    ├── S2-03 (Windows Discovery) ──┤──→ S2-04 (Windows OMA-DM)
    ├── S2-05 (Android Enrollment) ─┤
    └── S2-06 (Device Service) ─────┘
                                    │
                              Integration
                              & testing
```

S2-01, S2-02, S2-03, S2-05, and S2-06 can all start in parallel. S2-04 requires S2-03 because OMA-DM sync only works after a device has enrolled via MS-MDE2.

## Service-Level Dependencies

| This Sprint Produces | Consumed By |
|---|---|
| NanoMDM webhook handler (check-in events) | Sprint 3 (macOS commands), Sprint 4 (PSSO profile push) |
| NanoDEP device sync + auto-assign | Sprint 3 (macOS zero-touch) |
| Windows enrollment + SyncML session | Sprint 3 (Windows CSPs, policies) |
| Android Management API client | Sprint 3 (Android policies, apps) |
| Unified DeviceService (create, get, list, update status) | Sprint 3, 4, 5 (everything) |
| APNs push integration | Sprint 3 (macOS commands) |

## Definition of Done

- [ ] macOS device enrolls via configuration profile, appears in device list
- [x] macOS DEP profile can be defined and assigned to serial numbers
- [x] Windows device discovers MDM server and completes enrollment
- [x] Windows device reports DeviceInfo via OMA-DM sync
- [ ] Android device enrolls via QR code, appears in device list
- [x] `GET /api/v1/devices` returns enrolled devices from all platforms
- [x] `GET /api/v1/devices/{id}` returns platform-specific inventory data
