# Sprint 6: Gaps & Technical Debt

**Created**: 2026-04-28
**Updated**: 2026-04-29
**Purpose**: Honest accounting of shortcuts, missing tests, and documentation gaps from Sprint 6.

---

## Shortcuts & Stubs

1. **Windows PPKG generator** — produces invalid .ppkg (ZIP with only Customizations.xml, missing DPP metadata folder and catalog file). Needs Windows ADK research or proper format implementation.
2. **macOS device record created manually via SQL INSERT** — instead of fixing the plist parsing bug first. Bug was later fixed but the manual insert masked it.
3. **NanoMDM schema created manually via psql** — instead of fixing the init script first. Init script updated later.
4. **`ActiveNSExtensions` added to auto-queue without checking platform support** — failed on macOS, removed after. Should have checked Apple docs first.
5. ~~**Enterprise ID hardcoded to `00000000-0000-0000-0000-000000000001`** in macOS Authenticate webhook handler.~~ ✅ Fixed — uses `default_enterprise_id` config.
6. ~~**Windows enrollment handler fallback `enterpriseRepo.List(ctx, 1, 0)`**~~ ✅ Fixed — uses `default_enterprise_id` config with hardcoded UUID fallback.
7. ~~**No unit tests for new webhook parsing code**~~ ✅ Fixed — processCommandResult, maybeAutoQueue, and full webhook flow integration tests added.
8. **`formatBytes` template function** — never verified renders correctly in browser.
9. **Command status tracking** — commands created as `sent` immediately, skipping `pending` → `sent` transition. If NanoMDM rejects, DB says "sent" but it wasn't.
10. **Empty state SVG centering** — 6 iterations (S6-13a through S6-13h) without testing in browser. Should have inspected Tailwind preflight CSS first.
11. **Windows PPKG** — gave up after one failed attempt instead of researching the valid format. The spec is public and buildable.
12. ~~**Debug log lines left in production code**~~ ✅ Removed.
13. ~~**Stale test enterprises** — cleaned with broad `DELETE FROM enterprises WHERE id != '...'` which cascade-deleted devices, groups, policies. Should have been more surgical.~~ ✅ Fixed — `testutil.CreateTestEnterprise()` with `t.Cleanup()` cascade delete; broad pre-clean removed from `connectAndCleanDB`.

---

## Test Coverage Gaps

### Coverage by Package (as of S6-12 cleanup)

| Package | Coverage | Notes |
|---------|----------|-------|
| `internal/apperrors` | 100.0% | |
| `internal/models` | 100.0% | |
| `internal/metrics` | 97.5% | Up from 65.0% — DB metrics, mux middleware, server lifecycle |
| `internal/validation` | 96.6% | |
| `internal/audit` | 95.2% | Up from 10.7% — async logger, structured logging, shutdown tests |
| `internal/config` | 91.6% | |
| `internal/auth` | 90.7% | Keycloak integration tests |
| `internal/platform/android` | 90.0% | Up from 57.1% — webhook data paths, Google API wrappers |
| `internal/tracing` | 86.7% | |
| `internal/reporting` | 86.0% | Up from 17.0% — integration tests with Docker PostgreSQL |
| `internal/platform/windows` | 85.2% | Up from 69.1% — enrollment, management, CSP tests |
| `internal/db` | 82.4% | With integration tests |
| `internal/service` | 81.0% | Up from 67.5% — translate, device actions, compliance |
| `internal/certs` | 78.0% | CRL generation, SignCSRPEM, loadCA error paths |
| `internal/platform/macos` | 77.9% | Up from 54.1% — webhook integration tests |
| `internal/repository` | 77.9% | Up from 7.9% — integration tests with Docker PostgreSQL |
| `internal/scep` | 75.9% | |
| `internal/api` | 58.6% | ⚠️ Below 70% handler target — web handlers need route registration work |

### Integration Tests — Status

- [x] Webhook with base64-encoded Authenticate plist → device record created with name/model/serial/OS
- [x] Webhook with base64-encoded TokenUpdate plist → device status set to enrolled, push_magic stored
- [x] processCommandResult with SecurityInfo plist → FileVaultEnabled, firewall_enabled in platform_data
- [ ] processCommandResult with DeviceInformation plist → 35 fields parsed correctly
- [ ] processCommandResult with ManagedApplicationList dict format (not array)
- [x] maybeAutoQueue cooldown prevents re-queuing within 15 minutes
- [ ] maybeAutoQueue sends 8 commands to NanoMDM API
- [ ] Command status transitions: created as sent → marked completed on Acknowledged
- [x] Windows discovery SOAP envelope parsing with real Windows 11 request format
- [x] Windows enrollment response contains base64 provisioning XML (not raw cert)

### Skipped Tests

- `jsonb_validation_test.go` — 3 integration tests skip with `testing.Short()`. They run normally in Docker (`make dev-test` does not pass `-short`).
- `auth_coverage_test.go` — 6 tests skip when Keycloak unavailable. Run in Docker (fixed this sprint) but skip locally.

---

## Documentation Issues

### Missing Documentation

1. ~~**CHANGELOG.md** — no Sprint 6 entry~~ ✅ Added
2. ~~**No VM setup guide**~~ ✅ Added — see `tests/device-testing/VM_SETUP.md`
3. ~~**No enrollment guide** — how to enroll macOS~~ ✅ Added
4. **No data pipeline documentation** — what commands are auto-queued, what fields flow into platform_data, how compliance evaluates them
5. **No nginx TLS proxy documentation** — why it exists, how to configure, cert generation

### Stale/Inaccurate Documentation

6. ~~**README.md** — says "All backend features complete through Sprint 5g"~~ ✅ Updated with Sprint 6 status
7. **TESTING.md** — only 4 mentions of macOS/NanoMDM/webhook — doesn't document webhook testing or real device testing approach
8. ~~**ARCHITECTURE.md** — doesn't mention nginx TLS proxy, auto-queue pipeline, or NanoMDM webhook data flow~~ ✅ Updated
9. ~~**config.docker.yaml** — `nanomdm_url` hardcoded to `192.168.1.102:9000`~~ ✅ Fixed with env var override

### Configuration Issues

10. ~~**nginx service not in `make prod-up`**~~ ✅ Added (S6-08)
11. **`howett.net/plist` dependency** — added but not documented in dependency notes
12. **CA cert persistence** — volume mount added to docker-compose but not documented (why it's needed, what happens without it)

---

## Action Items (Priority Order)

### Must Fix (before merge to main)
- [x] Write unit tests for `processCommandResult` (SecurityInfo + DeviceInfo at minimum)
- [x] Write unit test for `maybeAutoQueue` cooldown logic
- [x] Remove debug log lines from webhook.go
- [x] Fix hardcoded enterprise ID in Authenticate handler
- [x] Add Sprint 6 entry to CHANGELOG.md

### Should Fix (soon after merge)
- [x] Write integration tests for full webhook flow (Authenticate → TokenUpdate → Connect → Acknowledged)
- [x] Fix `nanomdm_url` in config to use Docker hostname with env var override
- [x] Add nginx to `make prod-up` target
- [x] Update README.md with Sprint 6 status
- [x] Update ARCHITECTURE.md with data pipeline flow
- [ ] Research and fix PPKG format for valid Windows provisioning packages

### Nice to Have
- [ ] Fix empty state SVG centering properly (rebuild Tailwind CSS)
- [x] Add enrollment guide documentation
- [x] Add VM setup guide
- [ ] Fix command status `pending` → `sent` → `completed` transitions
- [x] Fix `enterpriseRepo.List` fallback in Windows enrollment handler

### Retro Items (Session 2, 2026-04-28)
- [x] Add unit tests for Windows enrollment fixes: namespace, Content-Length, CSR fallback, duplicate device upsert, enterprise ID from email, last_seen on OMA-DM sync
- [ ] Replace hand-rolled ASN.1 CSR parser with proper CSR signature verification (or use a lenient x509 fork)
- [ ] CSR fallback should preserve original subject from CSR instead of generic `CN=MDMDeviceCert`
- [x] Use `default_enterprise_id` config value for Windows enrollment fallback instead of hardcoded UUID
- [x] Generate unique ActivityId per discovery response instead of hardcoded UUID
- [x] Refactor Windows discovery response from `fmt.Sprintf` template to Go struct XML marshaling (prevents namespace/formatting bugs)
- [x] Return device ID from `HandleSyncML` instead of re-parsing XML in `ExtractDeviceIDFromSyncML`
- [x] Make CRL auto-generated alongside CA cert; CRL endpoint still hardcoded path (configurable is nice-to-have)
- [x] Run `make dev-test` to verify all fixes pass the full test suite

---

*This document is the output of the Sprint 6 retrospective, 2026-04-28.*

---

## Session Handoff Context

### VM Infrastructure
- **macOS VM**: `ssh testuser@192.168.64.4` — macOS 26.2, UTM, enrolled in MDM, checking in on reboot. Password: `testuser`. FileVault enabled.
- **Windows VM**: `ssh testuser@192.168.65.2` — Windows 11 Pro ARM64 Build 26200, UTM. Password: `testuser`. CA cert trusted. Hosts entry for enterpriseenrollment.localmdm.local. Enrolled via Settings UI, OMA-DM syncing.
- **MDM Server**: `http://192.168.1.102:8080` (HTTP) / `https://192.168.1.102:8443` (HTTPS via nginx)
- **VM templates**: `LocalMDM-macOS-Template`, `LocalMDM-Windows-Template` — clean snapshots for reset via `restore_vms.sh`
- **Start VMs**: `utmctl start "LocalMDM-macOS-Test"` / `utmctl start "LocalMDM-Windows-Test"`

### Docker Stack
- `localmdm` — Go server on port 8080, CA certs mounted from `./internal/api/certs/`
- `nanomdm` — Apple MDM protocol handler on port 9000, CA cert mounted from same dir
- `nginx-tls` — TLS proxy on ports 443 and 8443, server cert signed by our CA
- `keycloak` — OIDC on port 8180, admin/admin. **v23 — should upgrade to latest (noted, not done)**
- `postgres` — port 5432, password `postgres-dev-password-1234`

### Config Notes
- `configs/config.docker.yaml` has `nanomdm_url: "http://192.168.1.102:9000"` — host-specific, needs env var override for portability
- CA certs persist in `./internal/api/certs/` via Docker volume mount — do NOT delete these or all enrolled devices lose trust
- NanoMDM database schema is in `docker/postgres/init-nanomdm-schema.sh` — includes `enrollment_queue` and `cert_auth_associations` tables added this sprint

### macOS Device State
- UDID: `35B9DA82-0B4D-51C3-8E6A-6694FCA3B75D`
- Serial: `ZL9QG3C3RR`
- Enterprise: Acme Corp (`00000000-0000-0000-0000-000000000001`)
- Auto-queues 8 commands on check-in: SecurityInfo, DeviceInformation (35 queries), ProfileList, InstalledApplicationList, CertificateList, ManagedApplicationList, AvailableOSUpdates, OSUpdateStatus
- Reboot VM to trigger check-in (no APNs push)
- FileVault: enabled, Firewall: disabled

### Windows Enrollment Status

**Windows Enrollment Status**: RESOLVED. Enrollment works via Settings UI ("Enroll only in device management"). Key fixes: XML namespace trailing slash, non-chunked HTTP responses (Content-Length), CRL Distribution Point on server cert. The `RegisterDeviceWithManagement` API still fails with `0x80180006` from Go — the Settings UI uses a different code path.
