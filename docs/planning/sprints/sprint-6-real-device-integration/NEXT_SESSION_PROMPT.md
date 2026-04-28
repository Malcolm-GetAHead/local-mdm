# Sprint 6 Continuation: Windows Enrollment + Cleanup

## Context
Sprint 6 is on branch `sprint-6/real-device-integration`. macOS enrollment is fully working with a real device (auto-queuing 9 commands, full data pipeline, compliance evaluation). Windows server-side protocol is verified but native device enrollment is blocked. Read `docs/planning/sprints/sprint-6-real-device-integration/GAPS.md` thoroughly before starting — it has the full handoff context including VM IPs, credentials, Docker stack details, and every Windows enrollment approach we tried.

## Your Mission (be autonomous — the owner is away and depending on you)

### Phase 1: Windows Enrollment via Agent (HIGH PRIORITY)
Fleet DM (Apache 2.0, `github.com/fleetdm/fleet`) enrolls Windows without Azure AD by using an agent that calls `RegisterDeviceWithManagement` from within its own STA COM process. Study their approach:
- `server/mdm/microsoft/` — protocol implementation
- Their fleetd agent handles the enrollment call

Build a minimal Windows enrollment agent:
1. Write a C# Windows Service or console app with `[STAThread]` that:
   - Calls `RegisterDeviceWithManagement(upn, discoveryUrl, "")` 
   - The COM issue is that .NET's `Main` with `[STAThread]` wasn't enough — Fleet's agent likely initializes COM explicitly via `CoInitializeEx(COINIT_APARTMENTTHREADED)` before the P/Invoke call
   - Try: explicit `CoInitializeEx` before `RegisterDeviceWithManagement`
2. Compile it on the Windows VM (C# compiler at `C:\Windows\Microsoft.NET\Framework64\v4.0.30319\csc.exe`, C# 5 syntax only)
3. Run it — if it works, the device will natively enroll and show in Settings → Access work or school
4. The discovery URL is: `https://192.168.1.229:8443/EnrollmentServer/00000000-0000-0000-0000-000000000001/Discovery.svc`

**VM Access:**
- Windows VM: `ssh testuser@192.168.65.2` (password: testuser)
- macOS VM: `ssh testuser@192.168.64.4` (password: testuser)  
- Start VMs: `utmctl start "LocalMDM-Windows-Test"` / `utmctl start "LocalMDM-macOS-Test"`
- Restore from template if needed: `utmctl stop "LocalMDM-Windows-Test"` then restore via UTM GUI or `restore_vms.sh`
- The Windows VM has: CA cert trusted, hosts entry for `enterpriseenrollment.localmdm.local`, Windows ADK/ICD installed
- MDM server: `http://192.168.1.229:8080` (HTTP) / `https://192.168.1.229:8443` (HTTPS via nginx)

**If the agent approach works**, verify:
- Device shows in Settings → Access work or school
- Device appears in Local MDM dashboard under Acme Corp
- OMA-DM sync happens on schedule (check provisioning XML ADDR points to `https://192.168.1.229:8443/ManagementServer/MDM.svc`)

### Phase 2: Fix "Must Fix" Items from GAPS.md (before merge)
1. Write unit tests for `processCommandResult` in `webhook.go` — at minimum SecurityInfo and DeviceInformation parsing
2. Write unit test for `maybeAutoQueue` cooldown logic
3. Remove debug log lines from `webhook.go` (`h.logger.Debug("authenticate raw payload"...)` and `h.logger.Debug("raw acknowledge payload"...)`)
4. Fix hardcoded enterprise ID `00000000-0000-0000-0000-000000000001` in macOS Authenticate webhook handler — should be configurable or derived from enrollment
5. Add Sprint 6 entry to CHANGELOG.md
6. Run `make dev-test` — all 19 packages must pass

### Phase 3: Documentation
1. Update README.md with Sprint 6 status
2. Update ARCHITECTURE.md with: nginx TLS proxy, NanoMDM webhook data pipeline, auto-queue flow
3. Fix `nanomdm_url` in `configs/config.docker.yaml` — add env var override (`NANOMDM_URL`) with Docker hostname default

### Important Notes
- **CA certs are volume-mounted** from `./internal/api/certs/` — do NOT delete these files or all enrolled devices lose trust
- **NanoMDM schema** is in `docker/postgres/init-nanomdm-schema.sh` — includes `enrollment_queue` and `cert_auth_associations`
- **macOS device UDID**: `35B9DA82-0B4D-51C3-8E6A-6694FCA3B75D`, Serial: `ZL9QG3C3RR` — reboot VM to trigger check-in
- **Keycloak**: v23 on port 8180, admin/admin — upgrade to latest is tracked but NOT for this session
- **Docker stack**: `docker compose up -d` starts everything. `docker compose build localmdm && docker compose up -d localmdm` for rebuilds
- **Tests**: `make dev-test` runs all 19 packages in Docker with race detector
- **Git**: commit per sub-task with `S6-XX:` prefix, push after each commit, never commit to main

### What Success Looks Like
- Windows VM natively enrolled and visible in dashboard with real device data
- All "Must Fix" items from GAPS.md resolved
- Full test suite passes
- Documentation updated
- Branch ready for PR to main
