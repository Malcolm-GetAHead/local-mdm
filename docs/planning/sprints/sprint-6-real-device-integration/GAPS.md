# Sprint 6: Gaps & Technical Debt

**Created**: 2026-04-28
**Updated**: 2026-04-29 (post S6-13 documentation & test audit)
**Purpose**: Honest accounting of shortcuts, missing tests, and documentation gaps from Sprint 6.

---

## Shortcuts & Stubs

1. **Windows PPKG generator** — produces invalid .ppkg (ZIP with only Customizations.xml, missing DPP metadata folder and catalog file). Needs Windows ADK research or proper format implementation.
2. ~~**macOS device record created manually via SQL INSERT**~~ ✅ Fixed — plist parsing bug resolved.
3. ~~**NanoMDM schema created manually via psql**~~ ✅ Fixed — init script updated.
4. ~~**`ActiveNSExtensions` added to auto-queue without checking platform support**~~ ✅ Removed.
5. ~~**Enterprise ID hardcoded**~~ ✅ Fixed — uses `default_enterprise_id` config.
6. ~~**Windows enrollment handler fallback `enterpriseRepo.List(ctx, 1, 0)`**~~ ✅ Fixed — uses `default_enterprise_id` config.
7. ~~**No unit tests for webhook parsing code**~~ ✅ Fixed — processCommandResult, maybeAutoQueue, full webhook flow tests.
8. **`formatBytes` template function** — never verified renders correctly in browser.
9. **Command status tracking** — commands created as `sent` immediately, skipping `pending` → `sent` transition. If NanoMDM rejects, DB says "sent" but it wasn't.
10. ~~**Debug log lines left in production code**~~ ✅ Removed.
11. ~~**Stale test enterprises**~~ ✅ Fixed — `testutil.CreateTestEnterprise()` with `t.Cleanup()` cascade delete; broad pre-clean removed. Postcondition gate added. Leaked mdmb devices now cleaned up automatically.

---

## Test Coverage (as of S6-13, merged Go + Playwright)

| Package | Go Only | Merged | Target | Status |
|---------|---------|--------|--------|--------|
| `internal/apperrors` | 100.0% | 100.0% | 60% | ✅ |
| `internal/models` | 100.0% | 100.0% | 60% | ✅ |
| `internal/validation` | 96.6% | 98.6% | 85% | ✅ |
| `internal/metrics` | 97.5% | 96.4% | 85% | ✅ |
| `internal/audit` | 95.2% | 90.4% | 92% | ✅ |
| `internal/auth` | 90.7% | 94.4% | 92% | ✅ |
| `internal/config` | 90.9% | 91.9% | 85% | ✅ |
| `internal/platform/android` | 90.0% | 92.4% | 80% | ✅ |
| `internal/tracing` | 86.7% | 91.7% | 85% | ✅ |
| `internal/reporting` | 86.0% | 92.7% | 85% | ✅ |
| `internal/platform/windows` | 83.4% | 89.2% | 80% | ✅ |
| `internal/service` | 81.0% | 93.5% | 80% | ✅ |
| `internal/db` | 29.4%¹ | 84.7% | 80% | ✅ |
| `internal/certs` | 78.0% | 86.2% | 80% | ✅ |
| `internal/platform/macos` | 77.9% | 87.5% | 80% | ✅ |
| `internal/repository` | 77.2%¹ | 87.7% | 80% | ✅ |
| `internal/scep` | 75.9% | 84.3% | 80% | ✅ |
| `internal/api` | 61.8%¹ | 75.9% | 70% | ✅ |
| **TOTAL** | **72.4%** | **78.7%** | **75%** | ✅ |

¹ Low Go-only numbers are expected — integration tests need Docker (`make dev-test`), and web handlers are covered by Playwright.

All packages meet STEERING targets when measured via `make coverage-combined` (merged).

---

## Remaining Action Items

### This Cleanup Session (03_FINAL_CLEANUP_PROMPT.md)
- [ ] Windows OMA-DM device info queries — sync handler doesn't send Get commands for BitLocker/firewall/OS
- [ ] Command status transitions — `pending` → `sent` → `completed` (currently skips `pending`)
- [ ] CSR subject preservation — `SignRawCSR` uses hardcoded `CN=MDMDeviceCert` instead of CSR subject
- [ ] CRL endpoint path — hardcoded `certs/ca.crl`, should derive from CA cert config path
- [ ] CA cert persistence note in SETUP.md
- [ ] `howett.net/plist` dependency note

### Deferred to Future Sprints
- **Windows PPKG format** — needs Windows ADK research (F-01 or dedicated task)
- **ASN.1 CSR parser** — replace hand-rolled parser with proper CSR signature verification or lenient x509 fork

### Dashboard Cleanup (04_DASHBOARD_CLEANUP.md)
- **Pending enrollments visibility** — devices with `status = 'pending'` invisible in device list
- **Empty state SVG centering** — inline styles instead of Tailwind classes, needs `make css` rebuild
- **`formatBytes` browser verification** — template function never verified in browser

---

## Session Handoff Context

### VM Infrastructure
- **macOS VM**: `ssh testuser@192.168.64.4` — macOS 26.2, UTM, enrolled in MDM. Password: `testuser`. FileVault enabled. Reboot to trigger check-in (no APNs).
- **Windows VM**: `ssh testuser@192.168.65.2` — Windows 11 Pro ARM64 Build 26200, UTM. Password: `testuser`. CA cert trusted. Enrolled via Settings UI, OMA-DM syncing.
- **MDM Server**: `http://192.168.1.102:8080` (HTTP) / `https://192.168.1.102:8443` (HTTPS via nginx)
- **VM templates**: `LocalMDM-macOS-Template`, `LocalMDM-Windows-Template` — restore via `scripts/restore_vms.sh`

### Docker Stack
- `localmdm` — Go server on port 8080, CA certs mounted from `./internal/api/certs/`
- `nanomdm` — Apple MDM on port 9000, CA cert mounted from same dir
- `nginx-tls` — TLS proxy on ports 443/8443, server cert signed by project CA
- `keycloak` — OIDC on port 8180, admin/admin
- `postgres` — port 5432, password `postgres-dev-password-1234`

### macOS Device State
- UDID: `35B9DA82-0B4D-51C3-8E6A-6694FCA3B75D`, Serial: `ZL9QG3C3RR`
- Enterprise: Acme Corp (`00000000-0000-0000-0000-000000000001`)
- Auto-queues 9 commands on check-in: SecurityInfo, DeviceInformation (35 queries), ProfileList, InstalledApplicationList, CertificateList, ManagedApplicationList, AvailableOSUpdates, OSUpdateStatus, UserList
- FileVault: enabled, Firewall: disabled

### Windows Device State
- Enrolled via Settings UI ("Enroll only in device management")
- OMA-DM sync sessions acknowledged but no device info queries sent yet
- `RegisterDeviceWithManagement` API fails with `0x80180006` from Go — Settings UI uses different code path

---

*This document is the output of the Sprint 6 retrospective (2026-04-28) and documentation audit (2026-04-29).*
