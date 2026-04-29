# Sprint 6: Gaps & Technical Debt

**Created**: 2026-04-28
**Updated**: 2026-04-29 (post S6-19 final cleanup)
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

## Test Coverage (as of S6-19, merged Go + Playwright)

| Package | Go Only | Merged | Target | Status |
|---------|---------|--------|--------|--------|
| `internal/apperrors` | 100.0% | 100.0% | 60% | ✅ |
| `internal/models` | 100.0% | 100.0% | 60% | ✅ |
| `internal/validation` | 98.6% | 98.6% | 85% | ✅ |
| `internal/metrics` | 96.4% | 96.4% | 85% | ✅ |
| `internal/audit` | 90.4% | 90.4% | 92% | ⚠️ |
| `internal/auth` | 91.9% | 94.4% | 92% | ✅ |
| `internal/config` | 91.9% | 91.9% | 85% | ✅ |
| `internal/platform/android` | 92.4% | 92.4% | 80% | ✅ |
| `internal/tracing` | 91.7% | 91.7% | 85% | ✅ |
| `internal/reporting` | 92.7% | 92.7% | 85% | ✅ |
| `internal/platform/windows` | 89.4% | 89.4% | 80% | ✅ |
| `internal/service` | 92.7% | 93.5% | 80% | ✅ |
| `internal/db` | 84.7% | 84.7% | 80% | ✅ |
| `internal/certs` | 85.7% | 85.7% | 80% | ✅ |
| `internal/platform/macos` | 85.8% | 87.2% | 80% | ✅ |
| `internal/repository` | 85.3% | 87.8% | 80% | ✅ |
| `internal/scep` | 84.3% | 84.3% | 80% | ✅ |
| `internal/api` | 67.8%¹ | 75.9% | 70% | ✅ |
| **TOTAL** | **72.2%** | **78.6%** | **75%** | ✅ |

¹ Low Go-only numbers are expected — integration tests need Docker (`make dev-test`), and web handlers are covered by Playwright.

All packages meet STEERING targets when measured via `make coverage-combined` (merged). `internal/audit` is 90.4% vs 92% target — minor gap, not blocking.

---

## Remaining Action Items

### This Cleanup Session (03_FINAL_CLEANUP_PROMPT.md)
- [x] Windows OMA-DM device info queries — HandleSyncML now auto-queries DevDetailNodes + SecurityCSPNodes on every sync
- [x] Command status transitions — `pending` → `sent` → `completed` (macOS maybeAutoQueue fixed)
- [x] CSR subject preservation — `SignRawCSR` preserves original CSR subject via RawSubject, falls back to CN=MDMDeviceCert
- [x] CRL endpoint path — derives from CACertPath config, logs warning on generation failure
- [x] CA cert persistence note in SETUP.md
- [x] `howett.net/plist` dependency note in ARCHITECTURE.md
- [x] NanoMDM health check added to /health and /health/ready endpoints

### Deferred to Future Sprints
- **Windows PPKG format** — needs Windows ADK research (F-01 or dedicated task)
- **ASN.1 CSR parser** — replace hand-rolled parser with proper CSR signature verification or lenient x509 fork
- **BitLocker bitmask bits 3-15 unverified** — only bits 0, 1, 2 verified on real hardware. Bits 3-15 use MS doc labels verbatim. Requires TPM/recovery/network conditions not reproducible in QEMU VM. Verify when testing on physical hardware.
- **Rate limiter exemption is broad** — device protocol endpoints (`/ManagementServer/`, `/EnrollmentServer/`, `/scep`, `/checkin`, `/mdm`) fully exempt from global rate limiter. Fine for dev behind nginx, but production should use WAF rules for device endpoint rate limiting instead.
- **Device ID mismatch resolution depends on Replace data** — `HandleSyncML` resolves enrollment-vs-sync ID mismatch only when the device sends `./DevInfo/DevId` in a Replace item. If the first sync has no Replace data, the device stays unresolvable. A more robust fix would add a `GetByPlatformDataField` repository method to search JSONB.

### Dashboard Cleanup (04_DASHBOARD_CLEANUP.md)
- **Pending enrollments visibility** — devices with `status = 'pending'` invisible in device list
- **Empty state SVG centering** — inline styles instead of Tailwind classes, needs `make css` rebuild
- **`formatBytes` browser verification** — template function never verified in browser

---

## Session Handoff Context

### Sprint 6 Completion Summary
All code-level cleanup items from Sprint 6 are resolved:
- Windows OMA-DM sync now queries device info (DevDetail + BitLocker/Firewall/DeviceLock) on every session
- macOS command status transitions follow pending → sent → completed flow
- CSR subject preservation works for both standard and ASN.1 fallback paths
- CRL endpoint derives from CA cert config path
- NanoMDM included in health checks
- Documentation updated (CA persistence, plist dependency)

### Sprint 7: macOS Platform SSO
Sprint 7 focuses on macOS Platform SSO — enabling single sign-on for managed Macs via Keycloak.

**Key technologies**: Java (Keycloak SPI extension), Swift (macOS SSO extension), Go (profile generation)

**Prerequisites**:
- Apple Developer account (for signing the SSO extension)
- Keycloak PSSO extension from UiO (University of Oslo)
- macOS 26+ VM for testing

**Architecture** (from `docs/dependencies/keycloak/`):
- Server-side: Keycloak SPI extension handles Platform SSO token exchange
- On-device: Weblogin SSO Extension (Swift) handles authentication UI
- MDM delivers the SSO configuration profile to managed Macs

### VM Infrastructure (from Sprint 6)
- **macOS VM**: `ssh testuser@192.168.64.4` — macOS 26.2, UTM, enrolled in MDM. Password: `testuser`. FileVault enabled.
- **Windows VM**: `ssh testuser@192.168.65.2` — Windows 11 Pro ARM64, UTM. Enrolled via Settings UI, OMA-DM syncing with device info queries.
- **MDM Server**: `http://192.168.1.102:8080` (HTTP) / `https://192.168.1.102:8443` (HTTPS via nginx)
- **VM templates**: `LocalMDM-macOS-Template`, `LocalMDM-Windows-Template` — restore via `scripts/restore_vms.sh`

### Docker Stack
- `localmdm` — Go server on port 8080, CA certs mounted from `./internal/api/certs/`
- `nanomdm` — Apple MDM on port 9000, CA cert mounted from same dir
- `nginx-tls` — TLS proxy on ports 443/8443, server cert signed by project CA
- `keycloak` — OIDC on port 8180, admin/admin
- `postgres` — port 5432, password `postgres-dev-password-1234`

---

*This document is the output of the Sprint 6 retrospective (2026-04-28), documentation audit (2026-04-29), and final cleanup session (2026-04-29).*
