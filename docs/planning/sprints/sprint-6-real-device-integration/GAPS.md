# Sprint 6: Gaps & Technical Debt

**Created**: 2026-04-28
**Purpose**: Honest accounting of shortcuts, missing tests, and documentation gaps from Sprint 6.

---

## Shortcuts & Stubs

1. **Windows PPKG generator** — produces invalid .ppkg (ZIP with only Customizations.xml, missing DPP metadata folder and catalog file). Needs Windows ADK research or proper format implementation.
2. **macOS device record created manually via SQL INSERT** — instead of fixing the plist parsing bug first. Bug was later fixed but the manual insert masked it.
3. **NanoMDM schema created manually via psql** — instead of fixing the init script first. Init script updated later.
4. **`ActiveNSExtensions` added to auto-queue without checking platform support** — failed on macOS, removed after. Should have checked Apple docs first.
5. **Enterprise ID hardcoded to `00000000-0000-0000-0000-000000000001`** in macOS Authenticate webhook handler. Breaks multi-tenant. Should come from enrollment profile or NanoMDM params.
6. **Windows enrollment handler fallback `enterpriseRepo.List(ctx, 1, 0)`** — picks first enterprise by created_at, which may be a test enterprise. Fixed by using enterprise ID in URL, but fallback logic is still wrong.
7. **No unit tests for new webhook parsing code** — SecurityInfo, DeviceInformation, ProfileList, AppList, CertificateList, UserList, OSUpdates parsing all verified only against real device.
8. **`formatBytes` template function** — never verified renders correctly in browser.
9. **Command status tracking** — commands created as `sent` immediately, skipping `pending` → `sent` transition. If NanoMDM rejects, DB says "sent" but it wasn't.
10. **Empty state SVG centering** — 6 iterations (S6-13a through S6-13h) without testing in browser. Should have inspected Tailwind preflight CSS first.
11. **Windows PPKG** — gave up after one failed attempt instead of researching the valid format. The spec is public and buildable.
12. **Debug log lines left in production code** — `h.logger.Debug("authenticate raw payload"...)` and `h.logger.Debug("raw acknowledge payload"...)` in webhook.go.
13. **Stale test enterprises** — cleaned with broad `DELETE FROM enterprises WHERE id != '...'` which cascade-deleted devices, groups, policies. Should have been more surgical.

---

## Test Coverage Gaps

### Zero Coverage (new code, no tests)

| Function | File | What it does |
|----------|------|-------------|
| `handleAcknowledge` | webhook.go:298 | Parses command results, updates platform_data |
| `processCommandResult` | webhook.go:347 | Core data pipeline — SecurityInfo, DeviceInfo, ProfileList, AppList, CertList, UserList, OSUpdates parsing |
| `maybeAutoQueue` | webhook.go:698 | Auto-queue logic with 15min cooldown |
| `parseUUID` | webhook.go:762 | UUID parsing helper |

### Coverage by Package

| Package | Coverage | Notes |
|---------|----------|-------|
| `internal/platform/macos` | 54.1% | Dragged down by new webhook code |
| `internal/platform/windows` | 69.1% | New SOAP format changes not fully tested |
| `internal/api` | 57.3% | New dashboard handlers (checkin, tabs) untested |
| `internal/certs` | 78.1% | Good — CA key usage change covered |

### Integration Tests We Should Have

- Webhook with base64-encoded Authenticate plist → device record created with name/model/serial/OS
- Webhook with base64-encoded TokenUpdate plist → device status set to enrolled, push_magic stored
- processCommandResult with SecurityInfo plist → FileVaultEnabled, firewall_enabled in platform_data
- processCommandResult with DeviceInformation plist → 35 fields parsed correctly
- processCommandResult with ManagedApplicationList dict format (not array)
- maybeAutoQueue cooldown prevents re-queuing within 15 minutes
- maybeAutoQueue sends 8 commands to NanoMDM API
- Command status transitions: created as sent → marked completed on Acknowledged
- Windows discovery SOAP envelope parsing with real Windows 11 request format
- Windows enrollment response contains base64 provisioning XML (not raw cert)

### Skipped Tests

- `jsonb_validation_test.go` — 3 integration tests skip unconditionally with `t.Skip("skipping integration test")`. Should run in Docker.
- `auth_coverage_test.go` — 6 tests skip when Keycloak unavailable. Run in Docker (fixed this sprint) but skip locally.

---

## Documentation Issues

### Missing Documentation

1. **CHANGELOG.md** — no Sprint 6 entry
2. **No VM setup guide** — how to create/configure macOS and Windows VMs for testing
3. **No enrollment guide** — how to enroll macOS (Safari → profile download → install), what the auto-queue does
4. **No data pipeline documentation** — what commands are auto-queued, what fields flow into platform_data, how compliance evaluates them
5. **No nginx TLS proxy documentation** — why it exists, how to configure, cert generation

### Stale/Inaccurate Documentation

6. **README.md** — says "All backend features complete through Sprint 5g" — doesn't mention Sprint 6 macOS data pipeline
7. **TESTING.md** — only 4 mentions of macOS/NanoMDM/webhook — doesn't document webhook testing or real device testing approach
8. **ARCHITECTURE.md** — doesn't mention nginx TLS proxy, auto-queue pipeline, or NanoMDM webhook data flow
9. **config.docker.yaml** — `nanomdm_url` hardcoded to `192.168.1.229:9000` (host-specific IP, won't work on another developer's machine)

### Configuration Issues

10. **nginx service not in `make prod-up`** — manual addition, not part of standard startup
11. **`howett.net/plist` dependency** — added but not documented in dependency notes
12. **CA cert persistence** — volume mount added to docker-compose but not documented (why it's needed, what happens without it)

---

## Action Items (Priority Order)

### Must Fix (before merge to main)
- [ ] Write unit tests for `processCommandResult` (SecurityInfo + DeviceInfo at minimum)
- [ ] Write unit test for `maybeAutoQueue` cooldown logic
- [ ] Remove debug log lines from webhook.go
- [ ] Fix hardcoded enterprise ID in Authenticate handler
- [ ] Add Sprint 6 entry to CHANGELOG.md

### Should Fix (soon after merge)
- [ ] Write integration tests for full webhook flow (Authenticate → TokenUpdate → Connect → Acknowledged)
- [ ] Fix `nanomdm_url` in config to use Docker hostname with env var override
- [ ] Add nginx to `make prod-up` target
- [ ] Update README.md with Sprint 6 status
- [ ] Update ARCHITECTURE.md with data pipeline flow
- [ ] Research and fix PPKG format for valid Windows provisioning packages

### Nice to Have
- [ ] Fix empty state SVG centering properly (rebuild Tailwind CSS)
- [ ] Add enrollment guide documentation
- [ ] Add VM setup guide
- [ ] Fix command status `pending` → `sent` → `completed` transitions
- [ ] Fix `enterpriseRepo.List` fallback in Windows enrollment handler

---

*This document is the output of the Sprint 6 retrospective, 2026-04-28.*

---

## Session Handoff Context

### VM Infrastructure
- **macOS VM**: `ssh testuser@192.168.64.4` — macOS 26.2, UTM, enrolled in MDM, checking in on reboot. Password: `testuser`. FileVault enabled.
- **Windows VM**: `ssh testuser@192.168.65.2` — Windows 11 Pro ARM64 Build 26200, UTM. Password: `testuser`. CA cert trusted. Hosts entry: `192.168.1.229 enterpriseenrollment.localmdm.local`. NOT enrolled (native enrollment blocked).
- **MDM Server**: `http://192.168.1.229:8080` (HTTP) / `https://192.168.1.229:8443` (HTTPS via nginx)
- **VM templates**: `LocalMDM-macOS-Template`, `LocalMDM-Windows-Template` — clean snapshots for reset via `restore_vms.sh`
- **Start VMs**: `utmctl start "LocalMDM-macOS-Test"` / `utmctl start "LocalMDM-Windows-Test"`

### Docker Stack
- `localmdm` — Go server on port 8080, CA certs mounted from `./internal/api/certs/`
- `nanomdm` — Apple MDM protocol handler on port 9000, CA cert mounted from same dir
- `nginx-tls` — TLS proxy on ports 443 and 8443, server cert signed by our CA
- `keycloak` — OIDC on port 8180, admin/admin. **v23 — should upgrade to latest (noted, not done)**
- `postgres` — port 5432, password `postgres-dev-password-1234`

### Config Notes
- `configs/config.docker.yaml` has `nanomdm_url: "http://192.168.1.229:9000"` — host-specific, needs env var override for portability
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
- Server protocol verified: Discovery, Policy, Enrollment, OMA-DM all work over HTTPS
- Native enrollment blocked: `RegisterDeviceWithManagement` COM error, Settings UI requires Azure AD, PPKG format invalid
- **PPKG findings**: A ZIP containing only `Customizations.xml` returns `0x80070057 E_INVALIDARG`. The correct CSP path for bulk enrollment is `Provisioning/Enrollments/{UPN}/DiscoveryServiceFullURL` (confirmed from Microsoft docs). The XML content is correct but the package format needs additional metadata files that Windows ICD generates. `provtool.exe` and `Install-ProvisioningPackage` cmdlet both fail. Next step: install Windows ADK on the VM and use `icd.exe` CLI to build a valid .ppkg from our Customizations.xml.
