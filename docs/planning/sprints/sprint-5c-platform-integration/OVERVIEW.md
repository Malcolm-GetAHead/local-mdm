# Sprint 5c: Platform Integration Fixes

**Status**: 🔲 Not Started  
**Duration**: 5-7 days  
**Goal**: Fix the three critical platform integration blockers that prevent real device management, fix SCEP protocol compliance, and increase service layer test coverage  
**Depends on**: Sprint 5b complete (EventBus provides reactive compliance evaluation)

---

## Why This Sprint

The project has 87+ API routes, a policy engine, compliance evaluation, and a CLI — but **no real device can complete enrollment on any platform**. Three structural issues exist since Sprint 2 that were verified through mock-based tests but would fail with real hardware:

1. macOS: protocol mismatch (handlers expect NanoMDM JSON, devices send Apple plist)
2. Windows: enrollment completes but no device record is created
3. Android: webhook events are acknowledged but silently dropped

These must be fixed before the dashboard (Sprint 5d) is useful — a dashboard showing zero devices isn't helpful.

---

## Testing Strategy — Simulated vs. Real Devices

This sprint frontloads all structural and integration work that can be verified **without real hardware**. The goal is to get the architecture correct and all code paths exercised with simulated payloads, so that the real device testing phase (F-01) is focused on edge cases rather than structural rework.

### mdmb — Apple Device Simulator

[`mdmb`](https://github.com/jessepeterson/mdmb) is a device simulator built by the same author as NanoMDM, specifically for testing MDM servers. It simulates the full Apple MDM protocol:

- Creates fake devices with UDIDs, serial numbers, simulated keychains
- Enrolls through the complete flow: SCEP certificate enrollment → MDM check-in (Authenticate, TokenUpdate)
- Connects to check command queues (simulates APNs push receipt)
- Responds to MDM commands (DeviceInformation, ProfileList, InstallProfile, etc.)
- Supports load testing: create N devices, enroll all, connect all with worker pools

**This means we can test the entire macOS → NanoMDM → Local MDM flow end-to-end without a real device**, including SCEP certificate enrollment. Add `mdmb` to the dev toolchain.

Usage:
```bash
# Create simulated devices
./mdmb devices-create -n 10

# Enroll through NanoMDM (uses our enrollment profile, hits our SCEP server)
./mdmb -uuids all devices-profiles-install -f enroll.mobileconfig

# Simulate devices checking in (triggers NanoMDM → Local MDM webhooks)
./mdmb -uuids all devices-connect
```

### What we CAN verify without real devices:
- **macOS full flow**: mdmb → NanoMDM → SCEP cert → check-in → webhook → device record creation → command dispatch → command response (complete E2E)
- **SCEP protocol**: mdmb's SCEP client exercises the real PKCS#7 envelope flow
- **Windows SOAP enrollment**: existing XML test fixtures + curl
- **Android webhook processing**: JSON payloads via curl + integration tests
- **All service layer business logic**: unit tests with mocks

### What REQUIRES real devices (deferred to F-01):
- Windows MS-MDE2 discovery → enrollment → management sync with a real Windows device
- Android Management API webhook delivery from Google's servers
- Apple-specific edge cases that mdmb doesn't simulate (APNs delivery, DEP enrollment, user enrollment)
- End-to-end lock/wipe verification on real hardware

The expectation is that F-01 will surface protocol-level edge cases (unexpected field values, timing issues, platform-specific quirks) but NOT architectural issues (wrong service wiring, missing database records, broken data flow).

---

## Tasks

| ID | Task | Effort | Dependencies |
|---|---|---|---|
| S5c-01 | macOS: Deploy NanoMDM + fix enrollment flow | 2-3 days | Docker, NanoMDM docs |
| S5c-02 | Windows: Fix enrollment to create device records | 0.5 day | — |
| S5c-03 | Android: Wire webhook handler + initialize API client | 1 day | Google service account |
| S5c-04 | SCEP: Fix protocol compliance (PKCS#7 envelopes) | 1 day | S5c-01 (macOS needs SCEP) |
| S5c-05 | Service layer test coverage (target: 60%+) | 1-2 days | S5c-01 through S5c-04 |
| S5c-06 | End-to-end integration tests (all platforms, local stack) | 1-2 days | S5c-01 through S5c-04 |

---

### S5c-01: macOS — Deploy NanoMDM + Fix Enrollment Flow

**Problem**: The enrollment profile tells Apple devices to POST to `/checkin` and `/mdm` on the Local MDM server. But the handlers at those routes (`CheckinHandler`, `CommandHandler`) expect NanoMDM's JSON webhook format — they call `json.NewDecoder(r.Body).Decode(&event)`. Real Apple devices send plist XML (Authenticate, TokenUpdate, CheckOut) and CMS-signed plist (command responses). Every real device check-in returns 400 Bad Request.

The intended architecture (per session notes) is:
- NanoMDM runs as a **separate service** handling the raw Apple MDM protocol
- Devices connect to NanoMDM's `/checkin` and `/mdm` endpoints
- NanoMDM forwards events to Local MDM via JSON webhooks
- Local MDM's handlers process the webhook JSON (which they already do correctly)

But NanoMDM is not deployed, not in docker-compose, and not configured.

**Fix — 4 sub-tasks**:

#### S5c-01a: Add NanoMDM to docker-compose.yml

```yaml
nanomdm:
  image: ghcr.io/micromdm/nanomdm:latest
  container_name: localmdm-nanomdm
  ports:
    - "9000:9000"
  environment:
    NANO_STORAGE_DSN: "postgres://postgres:postgres@postgres:5432/localmdm?sslmode=disable"
    NANO_WEBHOOK_URL: "http://localmdm:8080/api/v1/macos/webhook"
  depends_on:
    postgres:
      condition: service_healthy
```

NanoMDM shares the same PostgreSQL database (it has its own tables with a `nano_` prefix). It forwards check-in and command events to Local MDM via webhook.

**Research needed**: Verify the exact NanoMDM Docker image, environment variables, and webhook configuration. The `micromdm/nanomdm` repo has documentation. Key config:
- `NANO_STORAGE` — storage backend (postgres)
- `NANO_WEBHOOK_URL` — where to POST webhook events
- `NANO_API` — API key for the command API (Local MDM uses this to send commands)

#### S5c-01b: Create webhook receiver endpoint

The current `/checkin` and `/mdm` routes are registered as NanoMDM webhook receivers but they're on the wrong path. NanoMDM sends webhooks to a configurable URL. Create a dedicated webhook endpoint:

```
POST /api/v1/macos/webhook  — receives NanoMDM webhook JSON
```

This replaces the current `/checkin` and `/mdm` routes for webhook processing. The `/checkin` and `/mdm` routes should either be removed (devices talk to NanoMDM, not Local MDM) or proxied to NanoMDM.

**Current handler code that works correctly** (just needs the right route):
- `CheckinHandler.ServeHTTP` — correctly parses `WebhookEvent` JSON, creates device on Authenticate, updates status on CheckOut
- `CommandHandler.ServeHTTP` — correctly parses `CommandWebhookEvent` JSON, processes command responses

#### S5c-01c: Fix enrollment profile URLs

In `internal/platform/macos/enrollment.go`, the `GenerateEnrollmentProfile` function currently sets:
```go
ServerURL:  serverURL + "/mdm"
CheckInURL: serverURL + "/checkin"
```

These must point to NanoMDM instead:
```go
ServerURL:  nanomdmURL + "/mdm"
CheckInURL: nanomdmURL + "/checkin"
```

The `nanomdmURL` should come from config (`cfg.MacOS.NanoMDMURL`). The config struct already has this field — it's just never set in config.yaml.

#### S5c-01d: Update config.yaml with NanoMDM settings

```yaml
macos:
  nanomdm_url: "http://localhost:9000"    # NanoMDM server URL (for enrollment profile + command API)
  nanomdm_api_key: "localmdm-api-key"     # API key for NanoMDM command API
  push_topic: "com.example.mdm"
```

**Acceptance criteria**:
- [ ] NanoMDM starts in docker-compose and connects to PostgreSQL
- [ ] Enrollment profile points devices at NanoMDM's `/checkin` and `/mdm`
- [ ] NanoMDM forwards check-in events to Local MDM webhook endpoint
- [ ] Local MDM creates device record on Authenticate event
- [ ] Local MDM updates device status on CheckOut event
- [ ] Commands sent via `NanoMDMService.SendCommand()` reach NanoMDM's API
- [ ] **mdmb simulated device completes full enrollment** (SCEP cert + Authenticate + TokenUpdate)
- [ ] **mdmb device-connect triggers webhook → device appears in `GET /api/v1/devices`**

> **mdmb replaces the need for real macOS devices for protocol testing.** It exercises the full Apple MDM protocol stack (SCEP, check-in, command queue) against NanoMDM, which forwards to our webhook handlers. F-01 is still needed for APNs delivery, DEP enrollment, and Apple-specific edge cases that mdmb doesn't simulate.

---

### S5c-02: Windows — Fix Enrollment to Create Device Records

**Problem**: `handleWindowsEnrollmentService` in `internal/api/platform_handlers.go` generates a UUID (`deviceID := uuid.New()`), signs a certificate for it, and returns the enrollment response — but **never calls `deviceRepo.Create()`**. The device gets a valid certificate but no record exists in the database. Enrolled Windows devices are invisible to the system.

The comment in the code says "Create a pending device record" but the code only generates a UUID.

**Fix**: After signing the certificate and before returning the response, create the device record:

```go
// After CSR signing, before response generation:
device := &models.Device{
    BaseModel:    models.BaseModel{ID: deviceID},
    EnterpriseID: enterpriseID, // need to extract from request or config
    Platform:     models.PlatformWindows,
    DeviceID:     deviceID.String(),
    Status:       models.DeviceStatusEnrolled,
    PlatformData: models.JSONB{},
}
if err := s.deviceRepo.Create(r.Context(), device); err != nil {
    s.logger.Error("failed to create device record", "error", err)
    // Don't fail enrollment — device has a valid cert, record can be created on first sync
}
```

**Challenge**: The Windows enrollment SOAP request doesn't include an enterprise ID. Options:
1. Use a default enterprise (configured in config.yaml)
2. Extract from the enrollment URL (add enterprise_id as a path parameter)
3. Create a "pending" device and associate with enterprise on first management sync

Option 2 is cleanest — change the enrollment URL to include the enterprise ID, similar to macOS enrollment (`/macos/enroll/{enterprise_id}`).

**Also needed**: The `windowsService.CreateDevice()` method exists in `internal/platform/windows/service.go` and correctly calls `deviceRepo.Create()`. The handler should call this instead of duplicating the logic.

**Acceptance criteria**:
- [ ] Windows enrollment creates a device record in the database
- [ ] Device appears in `GET /api/v1/devices` after enrollment
- [ ] Device has correct platform, status, and enterprise association
- [ ] Existing Windows enrollment tests still pass

---

### S5c-03: Android — Wire Webhook Handler + Initialize API Client

**Problem (two issues)**:

**Issue 1: Webhook handler is a no-op.** `handleAndroidWebhook` in `platform_handlers.go` reads the body, verifies HMAC, logs "received android webhook", returns 200 OK, and does nothing. The `android.WebhookHandler` struct in `internal/platform/android/webhook.go` has full logic for enrollment, compliance, status, and unenrollment — but it's never instantiated or called.

Current code:
```go
func (s *Server) handleAndroidWebhook(w http.ResponseWriter, r *http.Request) {
    body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
    // ... HMAC verification ...
    s.logger.Info("received android webhook", "body_size", len(body))
    w.WriteHeader(http.StatusOK) // <-- That's it. No processing.
}
```

**Issue 2: Android API client never initialized.** `android.NewClient()` creates a real Google Android Management API client using service account credentials. But `server.go` only creates `android.NewService()` (repos + config) — it never creates `android.NewClient()`. Without the client, `handleStatusReport` can't fetch device details from Google, and enrollment tokens can't be created via the real API.

**Fix — 3 sub-tasks**:

#### S5c-03a: Instantiate WebhookHandler and wire into route

In `server.go` constructor:
```go
// Create Android webhook handler (only if credentials configured)
if cfg.Android.ServiceAccountJSON != "" {
    androidClient, err := android.NewClient(ctx, cfg.Android.ProjectID, cfg.Android.ServiceAccountJSON, logger)
    if err != nil {
        logger.Warn("Android client not available", "error", err)
    } else {
        s.androidWebhookHandler = android.NewWebhookHandler(s.androidService, androidClient, logger)
    }
}
```

#### S5c-03b: Replace no-op handler with real dispatch

Replace the no-op `handleAndroidWebhook` with delegation to the `WebhookHandler`:

```go
func (s *Server) handleAndroidWebhook(w http.ResponseWriter, r *http.Request) {
    // HMAC verification stays here (it reads the body)
    // ...

    if s.androidWebhookHandler != nil {
        s.androidWebhookHandler.HandleWebhook(w, r)
        return
    }
    // Fallback if client not configured
    s.logger.Warn("android webhook received but handler not configured")
    w.WriteHeader(http.StatusOK)
}
```

**Note**: The HMAC verification currently reads the body, but `HandleWebhook` also reads the body. Need to restructure so the body is read once and passed to both.

#### S5c-03c: Fix webhook handler stubs

In `internal/platform/android/webhook.go`:

- `handleComplianceReport`: Parse `event.Data` for compliance fields, update device `platform_data` with security posture, call `complianceService.EvaluateDevice()`
- `handleStatusReport`: After fetching device from Google API, actually persist the state to the local database via `h.service.UpdateDevice()`
- `handleUnenrollment`: Add `h.lifecycle.OnUnenroll(ctx, device)` call (requires adding `LifecycleService` to `WebhookHandler`)

**Acceptance criteria**:
- [ ] Android webhook events are parsed and dispatched to the correct handler
- [ ] Enrollment events create device records in the database
- [ ] Status report events update device state in the database
- [ ] Unenrollment events update device status and call lifecycle hooks
- [ ] Google API client initializes when credentials are configured
- [ ] Graceful degradation when credentials are not configured (log warning, skip)

---

### S5c-04: SCEP — Fix Protocol Compliance

**Problem**: The SCEP handler (`internal/scep/handler.go`) has two protocol issues:

1. **Response format**: `pkiOperation` returns the signed certificate as raw DER bytes with `Content-Type: application/x-x509-ca-cert`. Real SCEP clients (including Apple's) expect a **PKCS#7 (CMS) degenerate certificates-only envelope** wrapping the signed cert + CA cert chain. Apple's SCEP client will likely reject the raw DER response.

2. **Request format**: The handler tries to parse the request body as a raw DER or base64-encoded CSR. Real SCEP clients send a **PKCS#7 SignedData envelope** containing the CSR, encrypted with the CA's public key. The handler can't unwrap this.

**Fix**:

For the response, wrap the signed cert in a PKCS#7 degenerate envelope:
```go
// Use go.mozilla.org/pkcs7 or build manually
// The degenerate envelope contains: [signed cert, CA cert] with no signers
```

For the request, the simplest approach is to use the `github.com/smallstep/pkcs7` library (or `go.mozilla.org/pkcs7`) to:
1. Decrypt the PKCS#7 envelope using the CA private key
2. Extract the inner CSR
3. Sign it (existing logic)
4. Wrap the response in a PKCS#7 degenerate envelope

**Alternative**: If full PKCS#7 support is too complex, embed the `micromdm/scep` library as originally planned in S5-12. It handles all the CMS envelope wrapping/unwrapping. The trade-off is an additional dependency vs. correctness.

**Acceptance criteria**:
- [ ] SCEP GetCACert returns CA cert in PKCS#7 degenerate envelope (or raw DER — both are valid per RFC 8894)
- [ ] SCEP PKIOperation accepts PKCS#7-wrapped CSR requests
- [ ] SCEP PKIOperation returns signed cert in PKCS#7 SignedData envelope
- [ ] Verified with `openssl` CLI: generate CSR, submit to `/scep`, parse response
- [ ] **mdmb enrollment completes SCEP certificate exchange successfully** (this is the real test — mdmb uses a proper SCEP client)
- [ ] Integration test covers the full challenge → CSR → signed cert flow

> **mdmb's SCEP client is the definitive test.** It implements the same SCEP protocol flow as Apple devices (PKCS#7 envelopes, challenge passwords, CA cert fetching). If mdmb can enroll through our SCEP server, real Apple devices almost certainly can too. F-01 covers any Apple-specific SCEP quirks.

---

### S5c-05: Service Layer Test Coverage

**Problem**: `internal/service/` has 30.4% test coverage. This package contains all business logic — policy translation, compliance evaluation, device lifecycle, app deployment, user management, token auth. The low coverage means bugs in business logic won't be caught.

**Target**: 60%+ coverage for the service package.

**Priority areas** (by risk):
1. `token.go` — Token generation, validation, hash comparison. Security-critical. Currently 0% coverage.
2. `user.go` — User creation with role validation, deactivation. Currently 0% coverage.
3. `device.go` — Lock/Wipe/Restart with lifecycle hooks, command dispatch. Currently 0% coverage.
4. `app.go` — Deploy to multiple devices with error handling. Currently 0% coverage.
5. `compliance.go` — evaluatePolicy with security/restriction checks. Has some coverage from Sprint 4 tests but the new Sprint 5 logic (checkSecurityPolicy, checkRestrictionPolicy) is untested.

**Approach**: Write unit tests with mock repos (same pattern as existing `policy_test.go`, `groups_test.go`, `compliance_test.go`). No integration tests needed — the service layer is transport-agnostic.

**Acceptance criteria**:
- [ ] `go test -cover ./internal/service/` reports 60%+ coverage
- [ ] Token generation/validation/revocation tested
- [ ] User CRUD with role validation tested
- [ ] Device Lock/Wipe/Restart tested (verify lifecycle hooks called, command created)
- [ ] App Deploy tested (verify commands created for each device)
- [ ] Compliance security/restriction checks tested with various platform_data scenarios

---

### S5c-06: End-to-End Integration Tests (All Platforms, Local Stack)

**Goal**: Verify the full data flow across all services running locally — not just isolated unit tests, but the actual communication paths between Local MDM, NanoMDM, SCEP, and simulated devices.

**Test infrastructure** (all running in docker-compose):
- PostgreSQL (shared database)
- Keycloak (OIDC auth)
- NanoMDM (Apple MDM protocol handler, added in S5c-01)
- Local MDM (our server)
- mdmb (Apple device simulator, run as a test tool)

#### macOS E2E Flow (mdmb → NanoMDM → Local MDM)

Full enrollment-to-management lifecycle using mdmb:

```bash
# 1. Generate enrollment profile from Local MDM
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/macos/enroll/$ENTERPRISE_ID > enroll.mobileconfig

# 2. Create simulated device
mdmb devices-create

# 3. Enroll device (hits NanoMDM SCEP + check-in endpoints)
mdmb -uuids all devices-profiles-install -f enroll.mobileconfig

# 4. Verify device appeared in Local MDM (NanoMDM webhook → device record)
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/devices
# → should contain the enrolled device

# 5. Send a command via Local MDM → NanoMDM API
curl -X POST -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/devices/$DEVICE_ID/lock

# 6. Simulate device checking in (picks up command from NanoMDM)
mdmb -uuids all devices-connect

# 7. Verify command completed
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/devices/$DEVICE_ID/commands
# → lock command should show status: completed
```

**Automate as a Go integration test** in `tests/e2e/macos_enrollment_test.go`:
- Skip if NanoMDM not available (`t.Skipf`)
- Use `os/exec` to run mdmb commands
- Assert device appears in API after enrollment
- Assert command lifecycle (pending → sent → completed)

#### Windows E2E Flow (SOAP → Local MDM)

Simulate the MS-MDE2 enrollment flow with HTTP requests:

```
1. POST /EnrollmentServer/Discovery.svc → verify discovery response
2. POST /EnrollmentServer/Policy.svc → verify policy response
3. POST /EnrollmentServer/Enrollment.svc (with CSR) → verify cert + provisioning XML
4. GET /api/v1/devices → verify device record was created (S5c-02 fix)
5. POST /ManagementServer/MDM.svc (SyncML) → verify command delivery
```

**Automate as a Go integration test** in `tests/e2e/windows_enrollment_test.go`:
- Build SOAP XML payloads matching MS-MDE2 spec
- Verify each step returns correct response
- Assert device appears in database after enrollment

#### Android E2E Flow (webhook → Local MDM)

Simulate Google Management API webhook delivery:

```
1. POST /api/v1/android/enroll (get enrollment token)
2. POST /api/v1/android/webhook (ENROLLMENT event with HMAC signature)
   → verify device record created
3. POST /api/v1/android/webhook (STATUS_REPORT event)
   → verify device state updated in platform_data
4. POST /api/v1/android/webhook (COMPLIANCE_REPORT event)
   → verify compliance data stored
5. POST /api/v1/android/webhook (UNENROLLMENT event)
   → verify device status updated, lifecycle hooks called
6. GET /api/v1/devices → verify device shows correct status
```

**Automate as a Go integration test** in `tests/e2e/android_enrollment_test.go`:
- Build webhook JSON payloads matching Google's format
- Compute HMAC signatures
- Assert device lifecycle through all states

#### SCEP E2E Flow (mdmb exercises this as part of macOS enrollment)

mdmb's enrollment includes SCEP certificate enrollment, which tests:
- `GET /scep?operation=GetCACert` → CA cert in correct format
- `POST /scep?operation=PKIOperation` → CSR with challenge → signed cert in PKCS#7

If mdmb enrollment succeeds, SCEP is proven to work with a real SCEP client implementation.

#### Cross-Service Communication Test

Verify the webhook path works end-to-end:
```
1. Start Local MDM with NanoMDM webhook URL configured
2. Simulate NanoMDM sending an Authenticate webhook (JSON POST to /api/v1/macos/webhook)
3. Verify device record created in database
4. Simulate NanoMDM sending a CheckOut webhook
5. Verify device status updated to unenrolled
6. Verify lifecycle hooks fired
```

This can run without mdmb — just curl/Go HTTP client sending the JSON payloads that NanoMDM would send.

**Acceptance criteria**:
- [ ] macOS: mdmb device enrolls through NanoMDM, appears in Local MDM device list, receives and responds to a command
- [ ] Windows: simulated SOAP enrollment creates device record, management sync delivers commands
- [ ] Android: simulated webhook events create device, update status, trigger lifecycle hooks
- [ ] SCEP: mdmb enrollment proves PKCS#7 envelope handling works
- [ ] All tests run with `go test ./tests/e2e/...` when docker-compose stack is up
- [ ] Tests skip gracefully when NanoMDM/mdmb not available

---

## Definition of Done

- [ ] macOS enrollment profile points to NanoMDM, not Local MDM
- [ ] NanoMDM deployed in docker-compose, forwarding webhooks to Local MDM
- [ ] **mdmb simulated devices complete full enrollment and appear in device list**
- [ ] **mdmb device receives a command via NanoMDM and responds**
- [ ] Windows enrollment creates device records in the database
- [ ] **Windows E2E: SOAP enrollment → device record → management sync (automated test)**
- [ ] Android webhook events are processed (not silently dropped)
- [ ] **Android E2E: webhook enrollment → status update → unenrollment (automated test)**
- [ ] SCEP protocol works with mdmb's SCEP client and openssl
- [ ] **Cross-service webhook test: NanoMDM JSON → Local MDM → device record (automated test)**
- [ ] Service layer test coverage ≥ 60%
- [ ] All E2E tests run with `go test ./tests/e2e/...` and skip gracefully when services unavailable
- [ ] All existing tests pass
- [ ] Real device edge cases tracked as F-01 dependency (APNs, DEP, Windows MS-MDE2, Google webhook delivery)

---

*Created: 2026-04-22 — Critical platform integration blockers identified during Sprint 5 backward look*
