# Autonomous Agent Sessions — Implementation Plan

**Created**: 2026-04-29
**Purpose**: Self-contained sessions that an agentic coder can execute start-to-finish without human interaction.
**Estimated Total**: ~7 days of agent work across 8 sessions.

## Prerequisites (All Sessions)

- Read `.kiro/steering/STEERING.md` and `.kiro/steering/SESSION_NOTES.md` before starting
- Run `make dev-test` to establish a green baseline before any changes
- Branch: `feature/{session-id}-{short-name}` from `main`
- Commit per sub-task with `{session-id}:` prefix
- Push after each commit
- Run `go test -race ./...` after every change

## Mandatory Verification Gates (All Sessions)

These gates were added after AUT-01 revealed that `go test` alone misses real bugs.
Every session MUST follow these — no exceptions.

### After every sub-task:
1. `go test -race ./...` — unit/mock tests pass
2. `docker compose build localmdm && docker compose up -d localmdm` — rebuild with new code (migrations auto-apply on container start)
3. **Hit the real endpoint** — curl the API, open the dashboard page in a browser. If it's a UI change, visually confirm it renders.

### After all sub-tasks complete:
4. `make dev-test` — full integration test suite in Docker (scoped cleanup only — never `DELETE FROM <table>` without a WHERE clause scoped to test data)
5. Run Playwright browser tests: `cd tests/browser && node run-playbook.js` — all existing tests must still pass, plus new steps for the feature you implemented. Compare pass/fail count before and after your changes.
6. `git diff main --stat` — sanity check the changeset. Verify no unexpected files were modified.
7. Verify documentation is updated: DATABASE.md, API.md, openapi.yaml

### Test priority:
Both Go integration tests (against live PostgreSQL + Keycloak) and Playwright browser tests are
**high priority**. These catch real bugs that mock-based tests miss. If either suite regresses,
fix it before declaring the session complete.

### Test requirements:
- **Repository tests**: MUST be integration tests against Docker PostgreSQL (not mocks). Use `testutil.ConnectDB(t)`.
- **Handler tests**: Mock-based unit tests for logic, PLUS at least one curl/API test against the running server.
- **Dashboard tests**: Go-level tests using `newTestServerWithTemplates` + `doWithSession`.
- **Playwright tests**: Add steps to `tests/browser/browser-playbook.md` for every new dashboard page or feature. Run the full playbook (`node run-playbook.js`) and confirm your new steps pass. The playbook is the source of truth for UI verification.
- **No inline JavaScript** in templates — CSP blocks it. Use event delegation in `app.js`.
- **Documentation updates are part of the task**, not a cleanup step.

### Test data cleanup:
- Integration tests create their own enterprise via `testutil.CreateTestEnterprise(t, db, name)`.
  Each test gets a unique enterprise with a `test-` prefixed slug. `t.Cleanup()` CASCADE-deletes
  the enterprise and all child rows automatically. This provides full per-test isolation.
- `testutil.TestEnterpriseID` (`99999999-9999-9999-9999-999999999999`) is a permanent enterprise
  in seed data. Use it for Playwright browser tests or code that needs a stable enterprise reference.
  Do NOT use it for Go integration test data — use `CreateTestEnterprise` instead.
- `make dev-test` runs a preflight step that cleans up orphaned `test-%` enterprises from
  crashed previous runs, then runs tests, then runs postconditions.
- NEVER run `DELETE FROM <table>` without a WHERE clause — it destroys real device/token data.
- Playwright browser tests run against the real enterprise (`00000000-0000-0000-0000-000000000001`) — this is intentional so the owner can see test results in the dashboard.

## Live Device Infrastructure

Two real devices are enrolled and available for testing. Use them to validate that
code changes produce real effects on managed devices.

### macOS VM
- **SSH**: `ssh testuser@192.168.64.4` (password: `testuser`)
- **OS**: macOS 26.2, UTM virtual machine
- **Status**: Enrolled in MDM via NanoMDM, FileVault enabled
- **Reboot**: `ssh testuser@192.168.64.4 "echo 'testuser' | sudo -S shutdown -r now"`
- **MDM check-in**: macOS checks in on reboot (no APNs push available). **Reboot is required for new policies/profiles to be applied.**
- **Useful commands**:
  - `profiles show -all` — list installed MDM profiles
  - `system_profiler SPHardwareDataType` — hardware info
  - `fdesetup status` — FileVault status
  - `/usr/libexec/mdmclient QueryDeviceInformation` — trigger MDM device info query

### Windows VM
- **SSH**: `ssh testuser@192.168.65.2` (password: `testuser`)
- **OS**: Windows 11 Pro ARM64, UTM virtual machine
- **Status**: Enrolled via Settings UI, OMA-DM syncing
- **Reboot**: `ssh testuser@192.168.65.2 "shutdown /r /t 5"`
- **MDM sync**: Windows syncs on its own schedule. Force sync: `ssh testuser@192.168.65.2 "deviceenroller.exe /c /AutoEnrollMDM"`
- **Useful commands**:
  - `manage-bde -status` — BitLocker status
  - `netsh advfirewall show allprofiles` — firewall status
  - `dsregcmd /status` — MDM enrollment status
  - `Get-MdmDiagnosticLog` — MDM diagnostic logs (PowerShell)

### MDM Server Access
- **Dashboard**: `http://localhost:8080/dashboard/` (or `https://192.168.1.102:8443`)
- **API**: `http://localhost:8080/api/v1/`
- **Login**: admin / admin123

### Validation Pattern
When implementing features that affect device state (policies, commands, compliance):
1. Apply the change via API or dashboard
2. Trigger a device check-in (reboot macOS VM, or wait for Windows sync)
3. SSH into the device and verify the change took effect
4. Check the dashboard to confirm the device data updated
5. Include the SSH verification commands and expected output in the retrospective

---

## Session 1: Enrollment Token System ✅

**ID**: `AUT-01`
**Status**: COMPLETE
**Effort**: ~1 day (actual: ~1 day implementation + ~0.5 day review/fixes)
**Source**: F-07 §Enrollment Token System
**Branch**: `feature/aut-01-enrollment-tokens`

### Prompt

```
Read `.kiro/steering/STEERING.md` and `.kiro/steering/SESSION_NOTES.md` for project conventions.
Read `docs/planning/future/F-07-advanced-features.md` §Enrollment Token System for the full spec.
Read `docs/planning/future/autonomous_sessions.md` Session 1 for task list.

Implement an enrollment token system that controls who can enroll devices. Currently anyone
who knows the server hostname can enroll. Enrollment tokens are short-lived, limited-use
codes that authorize enrollment to a specific enterprise.

Branch: `feature/aut-01-enrollment-tokens` from `main`.
Commit per sub-task with `AUT-01:` prefix. Push after each commit.
Run `go test -race ./...` after every change.
Follow the Mandatory Verification Gates in the Prerequisites section — rebuild Docker,
hit real endpoints, and verify dashboard rendering after every sub-task.

Tasks:
1. Create migration: `enrollment_tokens` table (id, enterprise_id FK, token VARCHAR(64) UNIQUE,
   description, max_uses, uses_remaining, expires_at, created_by, created_at, revoked_at,
   status VARCHAR(20) DEFAULT 'active').
   Apply migration to running Docker PostgreSQL and verify with `\d enrollment_tokens`.
2. Repository: `EnrollmentTokenRepository` interface + PostgreSQL implementation following
   Writer/Reader pool pattern. Methods: Create, GetByToken, List, Revoke, DecrementUses,
   SetStatus, ExpireTokens.
3. API endpoints (behind auth middleware):
   - POST /api/v1/enrollment-tokens — create token (enterprise_id, description, max_uses, expires_in)
   - GET /api/v1/enrollment-tokens — list tokens for enterprise
   - DELETE /api/v1/enrollment-tokens/{id} — revoke token
   Response must include: token, email (Windows), macos_enroll_url, status.
   After implementing, curl each endpoint against the running server to verify.
4. Modify Windows enrollment handler: extract token from email local part, validate against
   enrollment_tokens table (status=active, not expired, uses_remaining > 0), decrement
   uses_remaining. Reject with SOAP fault if invalid. Enrollment MUST fail without a valid token.
5. Modify macOS enrollment profile handler: require `?token=` query param, validate same way.
   Enrollment MUST fail without a valid token (403). Include token in SCEP challenge metadata.
6. Dashboard page: "Enrollment Tokens" nav item. Inline create form (matching groups page pattern,
   NOT a modal). List tokens with Win/Mac enrollment instructions and copy buttons.
   Status badge from DB column (active/expired/revoked). Revoke button for active tokens.
   No inline JavaScript — use event delegation in app.js. Verify in browser after implementing.
7. Periodic expiry: add status column, periodic cleanup job (hourly) that sets status='expired'
   for active tokens past expires_at. On-access expiry: if an active token is time-expired,
   set status='expired' before rejecting.
8. Tests:
   - Repository integration tests against Docker PostgreSQL (not mocks) using testutil.ConnectDB(t)
   - Handler unit tests with mock repo covering all validation paths
   - Go-level dashboard tests using newTestServerWithTemplates + doWithSession
   - Token validation edge cases (expired, exhausted, revoked, valid, unlimited, already-expired-status)
   - Windows enrollment tests (valid token, expired, revoked, no token)
   - Add Playwright playbook steps for token create, list, revoke
9. Audit logging on create, use, and revoke actions. Populate created_by from auth context
   (check user exists in local DB before setting FK).
10. Update DATABASE.md, API.md, and openapi.yaml with new tables/columns/endpoints.

Acceptance criteria:
- Creating a token returns a copyable email address AND macOS enrollment URL
- Enrolling with a valid token succeeds and decrements uses_remaining
- Enrolling WITHOUT a token fails (403 for macOS, SOAP fault for Windows)
- Enrolling with expired/exhausted/revoked token returns a clear error
- Expired tokens show status='expired' in dashboard (not 'active')
- Dashboard shows token list with inline create form, Win/Mac copy buttons, revoke
- All tests pass with `go test -race ./...`
- `make dev-test` passes clean
- Playwright browser tests pass for enrollment token pages
- DATABASE.md, API.md, openapi.yaml are updated

When complete, perform this retrospective:
1. List every task where you made a shortcut, used a stub, skipped a test, or deviated from the spec.
2. Does everything align with the existing codebase? Any dependencies broken or gaps?
3. How's test coverage and documentation? Are they still accurate? Any skipped integration tests?

After the retrospective, provide:
- Summary of work completed
- Checklist of how the owner can verify the work (specific URLs to visit, commands to run, behaviors to test)
```

### Deliverables
- Migration file in `migrations/`
- `internal/repository/enrollment_token.go` + tests
- API handlers + tests
- Dashboard page (`templates/pages/enrollment_tokens.html`)
- Modified enrollment handlers for token validation

---

## Session 2: Dynamic Device Groups

**ID**: `AUT-02`
**Effort**: ~1 day
**Source**: F-07 §Dynamic Device Groups
**Branch**: `feature/aut-02-dynamic-groups`
**Depends on**: None (extends existing static groups)

### Prompt

```
Read `.kiro/steering/STEERING.md` and `.kiro/steering/SESSION_NOTES.md` for project conventions.
Read `docs/planning/future/F-07-advanced-features.md` §Dynamic Device Groups for the full spec.
Read `docs/planning/future/autonomous_sessions.md` Session 2 for task list.

Problem: Static groups require manual membership management. Admins want groups that
auto-populate based on device attributes (e.g., "all macOS devices", "all devices running
OS < 14.0"). The existing group system (S4-02) has manual membership only.

Branch: `feature/aut-02-dynamic-groups` from `main`.
Commit per sub-task with `AUT-02:` prefix. Push after each commit.
Run `go test -race ./...` after every change.
Follow the Mandatory Verification Gates in the Prerequisites section.

### Phase 1: Investigation & Design (commit as AUT-02: design doc)

Before writing any feature code, investigate and produce a design document
(`docs/planning/sprints/aut-02-design.md`) covering:

1. Read `internal/repository/group.go` and `internal/service/group.go` — document the
   current GroupRepository interface, how membership works (AddMember/RemoveMember/ListMembers),
   and how groups are used by policy assignments.
2. Read `migrations/000011_eventbus_triggers.up.sql` — list every event type string that
   DB triggers actually fire. Map these to the device lifecycle events that should trigger
   re-evaluation (create, update, delete). If the event types don't match what you need,
   document what's missing.
3. Propose a rule schema. Consider:
   - Should rules support OR logic (any rule matches) or only AND (all must match)?
   - Should rules filter on JSONB PlatformData fields (e.g., FileVaultEnabled) or only
     top-level device columns?
   - Should evaluation happen in Go (load all devices, filter in memory) or in SQL
     (translate rules to WHERE clauses)? What are the tradeoffs at 100 vs 10,000 devices?
   - What operators make sense? (equals, contains, greater_than, etc.)
4. Propose the migration schema (columns on device_groups, or a separate rules table?).
5. Propose the evaluation trigger strategy: EventBus subscriber, periodic background job,
   or both? What interval for periodic? What happens if evaluation is slow?

**Stop after Phase 1.** Commit the design doc and present it for review. Do not proceed
to Phase 2 until the owner has reviewed the design.

### Phase 2: Implementation (after design review)

Implement the approved design. The tasks below are a starting point — adjust based on
the design decisions made in Phase 1.

1. Migration: add columns to `device_groups` per approved schema.
2. Rule engine: implement evaluation logic per approved design.
3. Evaluation service: compute membership changes, apply, audit log.
4. EventBus subscriber and/or periodic evaluator per approved strategy.
5. API: extend group CRUD to accept type and rules. Add manual evaluate endpoint.
6. Dashboard: type toggle, rule builder UI, re-evaluate button. HTMX, no inline JS.
7. Tests: rule evaluation unit tests, repo integration tests, handler tests, dashboard
   tests, EventBus subscriber test, Playwright steps.
8. Update DATABASE.md, API.md, openapi.yaml.

Acceptance criteria:
- Creating a dynamic group with platform=macos rule auto-populates with macOS devices
- Adding a new macOS device triggers re-evaluation and adds it to the group
- Manual re-evaluation via API and dashboard works
- Static groups are unaffected
- All tests pass with `go test -race ./...`

When complete, perform the standard retrospective.
```

### Deliverables
- Design document with rule schema, evaluation strategy, and migration plan
- Migration adding dynamic group support
- Rule engine + evaluation service + tests
- EventBus subscriber and/or periodic evaluator
- Extended API handlers and dashboard UI

---

## Session 3a: Custom Device Tags

**ID**: `AUT-03a`
**Effort**: ~0.5 day
**Source**: F-07 §Custom Device Attributes
**Branch**: `feature/aut-03a-device-tags`
**Depends on**: None

### Prompt

```
Read `.kiro/steering/STEERING.md` and `.kiro/steering/SESSION_NOTES.md` for project conventions.
Read `docs/planning/future/F-07-advanced-features.md` §Custom Device Attributes.
Read `docs/planning/future/autonomous_sessions.md` Session 3a for task list.

Implement custom device tags — key-value metadata on devices for filtering and organization.

Branch: `feature/aut-03a-device-tags` from `main`.
Commit per sub-task with `AUT-03a:` prefix. Push after each commit.
Run `go test -race ./...` after every change.
Follow the Mandatory Verification Gates in the Prerequisites section.

Tasks:
1. Migration: create `device_tags` table (device_id FK, key VARCHAR(64), value VARCHAR(256),
   PRIMARY KEY (device_id, key)). Index on (key, value) for filtering.
   After rebuild, verify migration applied with `\d` in psql.
2. Repository: `DeviceTagRepository` — SetTags(ctx, deviceID, tags map[string]string),
   GetTags(ctx, deviceID), DeleteTag(ctx, deviceID, key), ListDevicesByTag(ctx, enterpriseID, key, value).
3. API endpoints:
   - PUT /api/v1/devices/{id}/tags — set tags (merge with existing)
   - GET /api/v1/devices/{id}/tags — get all tags
   - DELETE /api/v1/devices/{id}/tags/{key} — remove a tag
   - Extend GET /api/v1/devices with `?tag=key:value` filter parameter
   After implementing, curl each endpoint against the running server to verify.
4. Dashboard: add a "Tags" section to device detail page. Inline edit with add/remove.
   Add tag filter to device list page (text input, format `key:value`).
   No inline JavaScript — use event delegation in app.js. Verify in browser after implementing.
5. Tests: repository integration tests against Docker PostgreSQL (not mocks), handler tests,
   filter tests. Go-level dashboard tests using newTestServerWithTemplates + doWithSession.
   Add Playwright playbook steps for tag CRUD and tag filtering.
6. Update DATABASE.md, API.md, and openapi.yaml.

Acceptance criteria:
- Tags can be set, viewed, and removed on device detail page
- Device list can be filtered by tag (e.g., `?tag=department:engineering`)
- All tests pass with `go test -race ./...`

When complete, perform the standard retrospective.
```

### Deliverables
- Migration for `device_tags` table
- Tag repository + API + dashboard UI
- Tag filtering on device list

---

## Session 3b: Bulk Device Operations

**ID**: `AUT-03b`
**Effort**: ~0.5 day
**Source**: F-07 §Bulk Operations UI
**Branch**: `feature/aut-03b-bulk-ops`
**Depends on**: AUT-03a (uses tags for Add Tag / Remove Tag actions)

### Prompt

```
Read `.kiro/steering/STEERING.md` and `.kiro/steering/SESSION_NOTES.md` for project conventions.
Read `docs/planning/future/F-07-advanced-features.md` §Bulk Operations UI.
Read `docs/planning/future/autonomous_sessions.md` Session 3b for task list.

Problem: Admins need to perform actions on multiple devices at once (lock all devices in
a department, wipe all decommissioned devices, tag a batch of new devices). Currently
every action is one device at a time.

Branch: `feature/aut-03b-bulk-ops` from `main`.
Commit per sub-task with `AUT-03b:` prefix. Push after each commit.
Run `go test -race ./...` after every change.
Follow the Mandatory Verification Gates in the Prerequisites section.

### Phase 1: Design the confirmation UX (commit as AUT-03b: design doc)

Before implementing, investigate and document:

1. Read the device list page template and `app.js` — understand the current table structure
   and how HTMX interactions work on this page.
2. Propose the multi-select UX: how do checkboxes interact with pagination? If the admin
   selects 5 devices on page 1 and navigates to page 2, are the selections preserved?
   Document the tradeoffs of client-side vs server-side selection tracking.
3. Propose the confirmation modal pattern for destructive actions (lock, wipe). The spec
   suggests typed confirmation — is this the right pattern for this dashboard? Look at how
   other modals work in the existing templates.
4. Document the proposed API shape for the bulk endpoint.

Commit the design as `docs/planning/sprints/aut-03b-design.md`. **Stop and present for review.**

### Phase 2: Implementation (after design review)

1. Dashboard device list: add checkboxes, select all, selected count, actions dropdown.
   No inline JavaScript — use event delegation in app.js.
2. Actions: Lock, Wipe, Add Tag, Remove Tag, Assign Policy. Confirmation modal via HTMX.
   Destructive actions use the approved confirmation pattern from Phase 1.
3. API: POST /api/v1/devices/bulk — {action, device_ids[], params}. Returns per-device results.
4. Implementation: iterate device_ids, call existing service methods, collect results, audit log.
5. Tests: bulk handler (partial success, all fail, all succeed), confirmation UI, dashboard
   tests, Playwright steps.
6. Update DATABASE.md, API.md, openapi.yaml.

Acceptance criteria:
- Selecting multiple devices shows action dropdown
- Bulk lock sends lock command to all selected devices
- Confirmation modal shows device list and requires confirmation for destructive actions
- Partial failures return per-device results
- All tests pass with `go test -race ./...`

When complete, perform the standard retrospective.
```

### Deliverables
- Design doc for multi-select UX and confirmation pattern
- Bulk operations API endpoint + dashboard multi-select UI
- Confirmation modals for destructive actions

---

## Session 4: Outbound Webhook System

**ID**: `AUT-04`
**Effort**: ~0.5 day
**Source**: F-07 §Webhook System
**Branch**: `feature/aut-04-webhooks`
**Depends on**: None (uses existing EventBus)

### Prompt

```
Read `.kiro/steering/STEERING.md` and `.kiro/steering/SESSION_NOTES.md` for project conventions.
Read `docs/planning/future/F-07-advanced-features.md` §Webhook System for the spec.
Read the existing EventBus implementation in `internal/service/` — webhooks subscribe to it.
Read `docs/planning/future/autonomous_sessions.md` Session 4 for task list.

Implement an outbound webhook system. When events occur (device enrolled, policy applied,
compliance changed, etc.), HTTP POST notifications are sent to configured webhook URLs.
The system uses the existing PostgreSQL LISTEN/NOTIFY EventBus.

Branch: `feature/aut-04-webhooks` from `main`.
Commit per sub-task with `AUT-04:` prefix. Push after each commit.
Run `go test -race ./...` after every change.
Follow the Mandatory Verification Gates in the Prerequisites section — rebuild Docker,
hit real endpoints, and verify dashboard rendering after every sub-task.

Tasks:
0. **Investigation (before any code):** Read `migrations/000011_eventbus_triggers.up.sql` and
   list every event type string that DB triggers actually fire. Compare with the 7 event types
   below. If they don't match, adjust the event type list to match what the EventBus actually
   produces. Also read `internal/service/eventbus.go` to understand the Subscribe() pattern
   and MDMEvent struct. Commit findings as a comment in the first commit message.
1. Migration: `webhooks` table (id, enterprise_id FK, url, events TEXT[], secret VARCHAR(128)
   for HMAC-SHA256 signing, active BOOLEAN, description, failure_count INT DEFAULT 0,
   last_failure_at, created_at, updated_at). `webhook_deliveries` table (id, webhook_id FK,
   event_type, payload JSONB, status VARCHAR(20), response_code INT, attempts INT, next_retry_at,
   created_at).
   After rebuild, verify migration applied with `\d` in psql.
2. Repository: WebhookRepository — Create, List, GetByID, Update, Delete, ListActiveByEvent,
   RecordDelivery, GetPendingRetries.
3. Webhook dispatcher (`internal/service/webhook_dispatcher.go`): subscribes to EventBus events.
   On event: find all active webhooks subscribed to that event type, build JSON payload,
   sign with HMAC-SHA256 (secret in `X-Webhook-Signature` header), POST to URL with 5s timeout.
   On failure: increment failure_count, schedule retry (exponential backoff: 1m, 5m, 30m, max 3 attempts).
   After 10 consecutive failures, auto-disable webhook and log warning.
4. Retry worker: background goroutine that polls webhook_deliveries for pending retries,
   re-attempts delivery. Runs every 60 seconds.
5. API endpoints:
   - POST /api/v1/webhooks — create webhook (url, events[], secret, description)
   - GET /api/v1/webhooks — list webhooks for enterprise
   - PUT /api/v1/webhooks/{id} — update webhook
   - DELETE /api/v1/webhooks/{id} — delete webhook
   - POST /api/v1/webhooks/{id}/test — send a test event to verify connectivity
   - GET /api/v1/webhooks/{id}/deliveries — list recent deliveries (last 50)
   After implementing, curl each endpoint against the running server to verify.
6. Dashboard page: "Webhooks" under settings or as nav item. List webhooks with status
   (active/disabled/failing). Create form with URL, event checkboxes, secret field.
   Detail page shows recent deliveries with status codes. Test button. Enable/disable toggle.
   No inline JavaScript — use event delegation in app.js. Verify in browser after implementing.
7. Event types to support: device.enrolled, device.unenrolled, device.wiped, policy.assigned,
   policy.removed, compliance.changed, command.completed.
8. Tests: dispatcher unit tests (use `httptest.NewServer` as the webhook target — never
   configure webhooks pointing to external hosts), retry logic tests, HMAC signature
   verification test, handler tests, auto-disable after consecutive failures test.
   Repository integration tests against Docker PostgreSQL (not mocks).
   Go-level dashboard tests using newTestServerWithTemplates + doWithSession for webhook pages.
   Add Playwright playbook steps for webhook create, test, and delivery history.
9. Webhook sink container: add an nginx service to docker-compose (`webhook-sink`) that
   captures inbound POST/GET requests and logs full payloads. Configuration:
   - Listen on port 9999
   - Custom `log_format` that captures `$request_body`, `$http_x_webhook_signature`, method, URI
   - Set `client_body_buffer_size 64k` to capture full payloads without truncation
   - Log to a mounted volume (`./logs/webhook-sink/`) for inspection
   - Use `return 200` for all requests (always succeed so retry logic can be tested separately)
   This container is used for deployment-level validation — configure a webhook pointing to
   `http://webhook-sink:9999/hooks` and trigger events to verify the full outbound payload.
10. Update DATABASE.md, API.md, and openapi.yaml with new tables/columns/endpoints.

Acceptance criteria:
- Creating a webhook and enrolling a device sends a POST to the configured URL
- Payload is signed with HMAC-SHA256, verifiable with the shared secret
- Failed deliveries are retried with exponential backoff (up to 3 attempts)
- Webhook auto-disables after 10 consecutive failures
- Test endpoint sends a ping event and shows the result
- Dashboard shows delivery history with status codes
- All tests pass with `go test -race ./...`

When complete, perform this retrospective:
1. List every task where you made a shortcut, used a stub, skipped a test, or deviated from the spec.
2. Does everything align with the existing codebase? Any dependencies broken or gaps?
3. How's test coverage and documentation? Are they still accurate? Any skipped integration tests?

After the retrospective, provide:
- Summary of work completed
- Checklist of how the owner can verify the work (specific URLs to visit, commands to run, behaviors to test)
```

### Deliverables
- Migrations for `webhooks` and `webhook_deliveries` tables
- Webhook repository + dispatcher service + retry worker
- EventBus subscriber registration
- API endpoints + dashboard page
- HMAC signature signing and verification
- `webhook-sink` nginx container in docker-compose for payload inspection

---

## Session 5: Policy Dry-Run & Scheduled Deployment

**ID**: `AUT-05`
**Effort**: ~1 day
**Source**: F-07 §Policy Dry-Run Mode, §Scheduled Policy Deployment
**Branch**: `feature/aut-05-dryrun-scheduling`
**Depends on**: None

### Prompt

```
Read `.kiro/steering/STEERING.md` and `.kiro/steering/SESSION_NOTES.md` for project conventions.
Read `docs/planning/future/F-07-advanced-features.md` §Policy Dry-Run Mode and §Scheduled Policy Deployment.
Read the existing compliance service in `internal/service/compliance.go` and policy service
in `internal/service/policy.go` to understand the current evaluation flow.
Read `docs/planning/future/autonomous_sessions.md` Session 5 for task list.

Problem: Admins want to preview the impact of a policy before deploying it (dry-run), and
schedule policy deployments for a future time with optional gradual rollout (canary).

Branch: `feature/aut-05-dryrun-scheduling` from `main`.
Commit per sub-task with `AUT-05:` prefix. Push after each commit.
Run `go test -race ./...` after every change.
Follow the Mandatory Verification Gates in the Prerequisites section.

### Phase 1: Investigation & Design (commit as AUT-05: design doc)

Before writing any feature code, investigate and produce a design document
(`docs/planning/sprints/aut-05-design.md`) covering:

1. Read `internal/service/compliance.go` — find the method that evaluates a single device
   against a policy. Is it exported? What does it return? Can it be called without persisting
   results (dry-run mode)? If not, what changes are needed to support dry-run?
2. Read `internal/service/policy.go` — find how policies are assigned to targets (device,
   group, enterprise). What method handles assignment? What service methods exist?
3. Read `internal/repository/policy_assignment.go` — understand GetEffectivePolicies and
   how priority resolution works. Dry-run needs to simulate "what if this policy were added."
4. Propose the dry-run implementation: how does it hook into existing compliance evaluation
   without persisting results? What's the response shape?
5. For scheduled deployment: propose the canary strategy. Should canary auto-complete, or
   require manual approval? What happens if a canary deployment has failures — auto-rollback,
   pause, or continue? Document tradeoffs.
6. Propose the migration schema for `scheduled_deployments`.

**Stop after Phase 1.** Commit the design doc and present it for review. Do not proceed
to Phase 2 until the owner has reviewed the design.

### Phase 2: Dry-Run Implementation (after design review)

1. Implement dry-run service method per approved design.
2. API: POST /api/v1/policies/{id}/dry-run.
3. Dashboard: "Dry Run" button on policy detail, modal with results.
4. Tests: service tests, handler tests, edge cases, dashboard tests, Playwright.

### Phase 3: Scheduled Deployment (after dry-run is complete)

5. Migration: `scheduled_deployments` table per approved schema.
6. Repository: CRUD + GetPendingBefore, MarkInProgress, MarkCompleted.
7. Scheduler worker: background goroutine per approved strategy.
8. API: schedule, list, cancel endpoints.
9. Dashboard: schedule button, deployments list page.
10. Tests: scheduler, canary logic, cancellation, handler, repo integration, Playwright.
11. Update DATABASE.md, API.md, openapi.yaml.

Acceptance criteria:
- Dry-run shows accurate predicted compliance without modifying any data
- Scheduling a deployment for 5 minutes in the future executes it automatically
- Cancelling a pending deployment prevents execution
- Dashboard shows scheduled deployments with status
- All tests pass with `go test -race ./...`

When complete, perform the standard retrospective.
```

### Deliverables
- Dry-run service method + API + dashboard modal
- Migration for `scheduled_deployments` table
- Scheduler worker with canary support
- API endpoints + dashboard scheduling UI

---

## Session 6a: Security Audit Logging & CI Scanning

**ID**: `AUT-06a`
**Effort**: ~0.5 day
**Source**: F-03 §Security Audit Logging, §Vulnerability Scanning
**Branch**: `feature/aut-06a-security-audit`
**Depends on**: None

### Prompt

```
Read `.kiro/steering/STEERING.md` and `.kiro/steering/SESSION_NOTES.md` for project conventions.
Read `docs/planning/future/F-03-advanced-security.md` for the full security spec.

Implement security audit logging (separate from operational audit_logs) and CI vulnerability
scanning. These are mechanical, low-risk features.

Branch: `feature/aut-06a-security-audit` from `main`.
Commit per sub-task with `AUT-06a:` prefix. Push after each commit.
Run `go test -race ./...` after every change.
Follow the Mandatory Verification Gates in the Prerequisites section.

Tasks:
1. Security audit log: create `security_audit_logs` table (separate from operational audit_logs).
   Fields: id, timestamp, event_type (auth_failure, permission_denied, token_revoked,
   cert_issued, cert_revoked, admin_action), severity (info/warning/critical), actor,
   source_ip, details JSONB. Repository + service. Wire into auth middleware (failed logins),
   cert service (issuance/revocation), and admin actions (user role changes).
2. Tests: security audit log repository integration tests, service tests, verify auth
   middleware integration. Playwright steps if dashboard exposure is added.
3. CI vulnerability scanning: create `.github/workflows/security.yml` with:
   - `govulncheck ./...` (Go vulnerability database)
   - `trivy fs --scanners vuln,secret .` (dependency + secret scanning)
   - Run on push to main and on PRs
   - Fail the build on HIGH/CRITICAL vulnerabilities
4. Update DATABASE.md, API.md, openapi.yaml.

Acceptance criteria:
- Failed login attempts appear in security audit log
- CI workflow YAML is valid (verify with `act` or manual inspection)
- All tests pass with `go test -race ./...`

When complete, perform the standard retrospective.
```

### Deliverables
- Security audit log table + repository + service + middleware integration
- `.github/workflows/security.yml` CI workflow

---

## Session 6b: Compliance Report Generation

**ID**: `AUT-06b`
**Effort**: ~0.5 day
**Source**: F-03 §Compliance Reporting
**Branch**: `feature/aut-06b-compliance-reports`
**Depends on**: None

### Prompt

```
Read `.kiro/steering/STEERING.md` and `.kiro/steering/SESSION_NOTES.md` for project conventions.
Read `docs/planning/future/F-03-advanced-security.md` §Compliance Reporting.

Problem: Admins need exportable compliance reports for auditors and management. Currently
compliance data is only viewable in the dashboard.

Branch: `feature/aut-06b-compliance-reports` from `main`.
Commit per sub-task with `AUT-06b:` prefix. Push after each commit.
Run `go test -race ./...` after every change.
Follow the Mandatory Verification Gates in the Prerequisites section.

Before implementing, read `internal/reporting/` to understand the existing reporting
package. Check if a ReportService already exists and what it covers. Build on existing
patterns rather than creating parallel structures.

Tasks:
1. `ReportService.GenerateComplianceReport(ctx, enterpriseID, format)` — produces a
   structured report with: executive summary (total devices, compliant %, non-compliant
   count), per-policy breakdown, per-device detail. Output formats: JSON and CSV.
2. API: GET /api/v1/reports/compliance?format=json|csv.
3. Dashboard: "Export Report" button on compliance page.
4. Tests: report generation tests (verify CSV output format), handler tests, dashboard tests.
   Repository integration tests against Docker PostgreSQL (not mocks).
5. Update DATABASE.md, API.md, openapi.yaml.

Acceptance criteria:
- Compliance report exports as JSON and CSV with accurate data
- Dashboard export button triggers download
- All tests pass with `go test -race ./...`

When complete, perform the standard retrospective.
```

### Deliverables
- Compliance report generation (JSON + CSV) + API + dashboard export button

---

## Session 6c: Certificate Renewal & Recovery Key Escrow

**ID**: `AUT-06c`
**Effort**: ~1 day
**Source**: F-03 §Certificate Auto-Renewal, §Recovery Key Escrow
**Branch**: `feature/aut-06c-cert-recovery`
**Depends on**: None

### Prompt

```
Read `.kiro/steering/STEERING.md` and `.kiro/steering/SESSION_NOTES.md` for project conventions.
Read `docs/planning/future/F-03-advanced-security.md` §Certificate Auto-Renewal and §Recovery Key Escrow.

Problem: Device certificates expire without warning, and recovery keys (BitLocker/FileVault)
are not centrally stored. Both require understanding platform-specific MDM responses.

Branch: `feature/aut-06c-cert-recovery` from `main`.
Commit per sub-task with `AUT-06c:` prefix. Push after each commit.
Run `go test -race ./...` after every change.
Follow the Mandatory Verification Gates in the Prerequisites section.

### Phase 1: Investigation (commit as AUT-06c: investigation)

Before implementing, investigate and document:

1. Read `internal/certs/` — understand the certificate lifecycle: how certs are issued,
   stored, and what fields track expiry. What repository methods exist for querying by
   expiry date?
2. Read `internal/platform/macos/` — find where SecurityInfo command responses are parsed.
   Is the FileVault recovery key already extracted? If not, what does the response look like?
3. Read `internal/platform/windows/` — find where OMA-DM sync responses are parsed. Is the
   BitLocker recovery key already extracted? What CSP path contains it?
4. Check how DEP tokens are encrypted with pgcrypto — this is the pattern for recovery key
   encryption. Find the encrypt/decrypt calls.
5. Document findings and proposed approach in the first commit message.

### Phase 2: Certificate Renewal

1. `CertRenewalService`: query devices where cert expires within 30 days. For each, queue
   a re-enrollment command (platform-specific). Background worker runs daily.
2. Dashboard: "Expiring Certificates" widget on home page (7/30/90 day counts).
3. Tests: cert renewal scheduling tests with mock data.

### Phase 3: Recovery Key Escrow

4. Migration: `recovery_keys` table (id, device_id FK, key_type VARCHAR, encrypted_key TEXT
   pgcrypto-encrypted, escrowed_at, rotated_at).
5. Repository + API endpoint to retrieve key (admin only, audit logged).
6. Parse recovery keys from platform responses (macOS SecurityInfo, Windows OMA-DM).
7. Key rotation endpoint: POST /api/v1/devices/{id}/recovery-key/rotate.
8. Dashboard: "Recovery Key" button on device detail (admin only), modal with copy.
   Every access is audit logged.
9. **Logging safety**: Recovery key values may appear at DEBUG level during development.
   Ensure production log level (INFO+) never includes key values.
10. Tests: encryption/decryption round-trip, repo integration, dashboard, Playwright.
11. Update DATABASE.md, API.md, openapi.yaml.

Acceptance criteria:
- Cert renewal worker identifies expiring certs (test with mock data)
- Recovery keys are stored encrypted and retrievable by admins only
- Every recovery key access is audit logged
- All tests pass with `go test -race ./...`

When complete, perform the standard retrospective.
```

### Deliverables
- Certificate renewal service + background worker + dashboard widget
- Recovery key escrow table + encrypted storage + admin retrieval UI
- Platform-specific response parsing for recovery keys

---

## Session 7: Accessibility (WCAG 2.1 AA) & i18n Framework

**ID**: `AUT-07`
**Effort**: ~1 day
**Source**: F-08 §WCAG 2.1 AA Compliance, §Internationalization
**Branch**: `feature/aut-07-a11y-i18n`
**Depends on**: None

### Prompt

```
Read `.kiro/steering/STEERING.md` and `.kiro/steering/SESSION_NOTES.md` for project conventions.
Read `docs/planning/future/F-08-internationalization-accessibility.md` for the full spec.
Read `docs/planning/future/autonomous_sessions.md` Session 7 for task list.

Implement WCAG 2.1 AA accessibility compliance and an i18n framework for the dashboard.
The dashboard uses Go templates + HTMX + Tailwind CSS. No React.

Branch: `feature/aut-07-a11y-i18n` from `main`.
Commit per sub-task with `AUT-07:` prefix. Push after each commit.
Run `go test -race ./...` after every change.
Follow the Mandatory Verification Gates in the Prerequisites section — rebuild Docker,
hit real endpoints, and verify dashboard rendering after every sub-task.

### Part A: WCAG 2.1 AA Compliance

Tasks:
1. Audit all templates in `internal/api/templates/` for accessibility issues. Fix:
   - Add `role` and `aria-label` attributes to navigation, tables, forms, modals
   - Add `aria-live="polite"` to HTMX swap targets (so screen readers announce updates)
   - Add `<label>` elements for all form inputs (some may use placeholder-only)
   - Add skip-to-content link as first focusable element in base template
   - Ensure all interactive elements are keyboard-accessible (tab order, Enter/Space activation)
   - Add `aria-expanded` to collapsible sections and dropdowns
   - Verify color contrast ratios meet 4.5:1 for text, 3:1 for large text (check Tailwind classes)
   **Important**: After modifying each template, rebuild and verify that specific page renders
   correctly in the browser before moving to the next template. Do not batch all template
   changes and test at the end — a broken template breaks the entire dashboard.
   **Verification**: Take a Playwright screenshot of each modified page and save to
   `docs/planning/sprints/aut-07-screenshots/`. This proves each page was individually verified.
2. Add focus-visible styles: ensure keyboard focus is visually distinct (Tailwind `focus-visible:ring-2`)
   on all buttons, links, and form controls.
3. Add `alt` text to all SVG icons (or `aria-hidden="true"` for decorative ones).
4. Ensure data tables have proper `<thead>`, `<th scope="col">`, and `<caption>` elements.
5. Test with Pa11y: create `tests/a11y/pa11y.config.js` that tests key pages (dashboard,
   devices, device detail, policies, groups, compliance, audit). Add `make a11y-test` target.

### Part B: i18n Framework

Tasks:
6. Create i18n middleware (`internal/api/i18n.go`): reads `Accept-Language` header or
   `?lang=` query param. Stores locale in request context. Defaults to `en`.
7. Create locale files: `internal/api/locales/en.json` — extract all user-facing strings
   from templates into key-value pairs. Create `internal/api/locales/es.json` as a
   second language (use machine translation, mark as draft).
8. Template function: `{{t .Lang "key"}}` that looks up the translation. Falls back to
   English if key is missing in the requested locale. Falls back to the key itself if
   missing in English (never breaks rendering).
9. Update base template to include a language switcher (dropdown in header, sets `?lang=` cookie).
10. Convert 2-3 representative pages (dashboard home, devices list, device detail) to use
    `{{t}}` calls instead of hardcoded English strings. Leave remaining pages for future sessions.
    No inline JavaScript — use event delegation in app.js. Verify in browser after implementing.
11. Tests: i18n middleware tests (header parsing, cookie, fallback), translation lookup tests
    (missing key, missing locale), Pa11y test runner.
    Go-level dashboard tests using newTestServerWithTemplates + doWithSession for i18n rendering.
    Add Playwright playbook steps for language switcher and a11y verification.
12. Translation playbook: create a SEPARATE Playwright playbook (`tests/browser/i18n-playbook.md`)
    that loads every converted page in each supported locale and verifies:
    - No raw translation keys visible (e.g., `nav.devices` instead of "Devices")
    - Language switcher changes the locale
    - Fallback to English works for missing keys
    This playbook runs via `node run-i18n-tests.js`, NOT as part of the main `run-playbook.js`.
    It is slow (loads every page twice per locale) and should not block general testing.
13. Update DATABASE.md, API.md, and openapi.yaml with new tables/columns/endpoints.

Acceptance criteria:
- All pages pass Pa11y with zero WCAG 2.1 AA errors
- Keyboard navigation works through all pages (tab through nav, tables, forms, modals)
- HTMX swaps announce content changes to screen readers
- Language switcher changes dashboard language to Spanish (on converted pages)
- Missing translation keys fall back gracefully (never show blank or error)
- All tests pass with `go test -race ./...`

When complete, perform this retrospective:
1. List every task where you made a shortcut, used a stub, skipped a test, or deviated from the spec.
2. Does everything align with the existing codebase? Any dependencies broken or gaps?
3. How's test coverage and documentation? Are they still accurate? Any skipped integration tests?

After the retrospective, provide:
- Summary of work completed
- Checklist of how the owner can verify the work (specific URLs to visit, commands to run, behaviors to test)
```

### Deliverables
- WCAG fixes across all templates
- Pa11y test config + `make a11y-test` target
- i18n middleware + locale files (en, es)
- `{{t}}` template function + language switcher
- 2-3 pages converted to i18n

---

## Session 8: User Documentation & OpenTelemetry Instrumentation

**ID**: `AUT-08`
**Effort**: ~1 day
**Source**: F-06 §User Documentation, F-05 §OpenTelemetry
**Branch**: `feature/aut-08-docs-otel`
**Depends on**: None

### Prompt

```
Read `.kiro/steering/STEERING.md` and `.kiro/steering/SESSION_NOTES.md` for project conventions.
Read `docs/planning/future/F-06-user-documentation.md` and `docs/planning/future/F-05-advanced-monitoring.md`.
Read `docs/planning/future/autonomous_sessions.md` Session 8 for task list.

Implement user-facing documentation and OpenTelemetry instrumentation. The documentation
should be written for IT admins who manage devices, not developers. OTel instrumentation
adds tracing spans to handlers, services, and DB calls.

Branch: `feature/aut-08-docs-otel` from `main`.
Commit per sub-task with `AUT-08:` prefix. Push after each commit.
Run `go test -race ./...` after every change.
Follow the Mandatory Verification Gates in the Prerequisites section — rebuild Docker,
hit real endpoints, and verify dashboard rendering after every sub-task.

### Part A: User Documentation

Tasks:
1. Create `docs/user-guide/` directory with:
   - `enrollment-windows.md` — step-by-step Windows enrollment via Settings UI.
     Include: prerequisites (CA cert trust), exact Settings navigation path, expected
     dashboard result. Reference the nginx HTTPS endpoint.
   - `enrollment-macos.md` — step-by-step macOS enrollment via Safari profile download.
     Include: CA cert trust via Keychain Access, Safari URL for profile, System Preferences
     profile installation, expected dashboard result.
   - `enrollment-android.md` — placeholder noting Google Cloud project requirement,
     link to F-07 for future implementation.
   - `admin-guide.md` — dashboard walkthrough: devices (filtering, detail, actions),
     policies (create, assign, compliance), groups (static, membership), compliance
     (evaluation, per-setting results), audit log (filtering, detail expansion).
   - `troubleshooting.md` — FAQ format, minimum 15 entries covering: enrollment failures,
     certificate trust issues, device not appearing, compliance showing unknown, SCEP errors,
     NanoMDM connectivity, Keycloak login issues, Docker stack issues, VM setup issues.
   - `configuration.md` — full config reference. Read `internal/config/config.go` and
     document every config option with: description, type, default value (if any), and
     whether it is required or optional. Group by section (server, database, keycloak,
     certificates, macos, windows, features, rate_limiting, metrics, audit_log, etc.).
     Include a `config.example.minimal.yaml` showing only the required fields needed to
     get a working instance (roughly 10-15 fields). Include a `config.example.full.yaml`
     showing all options with comments.
2. Take screenshots using Playwright scripts (login, navigate, screenshot) for key pages.
   Save to `docs/user-guide/images/`. Reference in markdown docs.
3. Create `docs/user-guide/README.md` as table of contents.

### Part B: OpenTelemetry Instrumentation

Tasks:
4. **Investigation (before adding dependencies):** Verify the service methods listed for
   instrumentation actually exist. Read `internal/service/policy.go`, `internal/service/compliance.go`,
   and `internal/service/device.go`. List the actual exported method signatures for:
   PolicyService.AssignToGroup (may not exist — check GroupService), ComplianceService.EvaluateDeviceByID,
   DeviceService.Lock, DeviceService.Wipe. If method names differ, use the actual names.
   Also check `internal/tracing/` — a tracing package already exists. Understand what it does
   before adding OTel alongside it. Commit findings in the first commit message.
5. Add OpenTelemetry SDK dependency: `go.opentelemetry.io/otel` and the OTLP exporter.
   Pin exact versions in go.mod (not open ranges). Run `go mod tidy` and verify the
   dependency count is reasonable. Check that the binary size increase is acceptable
   (compare `ls -la` of the built binary before and after).
   Use `otelhttp` middleware for automatic HTTP handler instrumentation.
5. Create `internal/tracing/otel.go`: initialize TracerProvider with OTLP exporter
   (configurable endpoint via `tracing.endpoint` config, disabled by default).
   Graceful shutdown on context cancellation.
6. Instrument key paths with custom spans:
   - HTTP middleware: wrap router with `otelhttp.NewHandler` (automatic span per request)
   - Service layer: add spans to PolicyService.AssignToGroup, ComplianceService.EvaluateDeviceByID,
     DeviceService.Lock/Wipe (the business logic entry points)
   - Database: add spans to repository methods via a tracing wrapper or context propagation
7. Add trace ID to structured log output (correlate logs with traces).
8. Config: add `tracing.enabled`, `tracing.endpoint`, `tracing.sample_rate` to config.
   Default: disabled. When enabled, exports to configured OTLP endpoint.
9. Tests: verify tracing doesn't break when disabled (default), verify spans are created
   when enabled (use in-memory exporter in tests).
   Go-level dashboard tests using newTestServerWithTemplates + doWithSession to verify no regressions.
   Add Playwright playbook steps for screenshot generation and user guide link verification.
10. Update DATABASE.md, API.md, and openapi.yaml with new tables/columns/endpoints.

Acceptance criteria:
- User guide docs are complete and reference correct URLs/paths for the current setup
- Screenshots are current (taken via Playwright against running stack)
- Troubleshooting FAQ has ≥15 entries covering common issues
- OTel instrumentation is disabled by default (zero overhead)
- When enabled, HTTP requests produce traces with service/handler/DB spans
- Trace ID appears in structured log output
- All tests pass with `go test -race ./...`

When complete, perform this retrospective:
1. List every task where you made a shortcut, used a stub, skipped a test, or deviated from the spec.
2. Does everything align with the existing codebase? Any dependencies broken or gaps?
3. How's test coverage and documentation? Are they still accurate? Any skipped integration tests?

After the retrospective, provide:
- Summary of work completed
- Checklist of how the owner can verify the work (specific URLs to visit, commands to run, behaviors to test)
```

### Deliverables
- `docs/user-guide/` with enrollment guides, admin guide, troubleshooting FAQ
- Playwright-generated screenshots
- OpenTelemetry SDK integration with OTLP exporter
- Handler, service, and DB instrumentation
- Trace ID in structured logs

---

## Session 0: Test Enterprise Isolation ✅

**ID**: `AUT-00`
**Status**: COMPLETE
**Effort**: ~2 hours (actual: ~1.5 hours including a design pivot)
**Source**: AUT-01 retrospective finding — tests and real data share the same database
**Branch**: `feature/aut-00-test-enterprise`
**Depends on**: None. **Run this before all other sessions.**

### Design Decision

The original spec called for a shared test enterprise that all integration tests use.
During implementation, this was found to reduce per-test isolation — tests sharing one
enterprise can't run in parallel and depend on cleanup between test functions not missing
any tables. The design was revised to keep per-test enterprise creation (the original
codebase pattern) with convention-based postconditions:

- **Integration tests** create their own enterprise via `testutil.CreateTestEnterprise(t, db, name)`.
  Each gets a unique `test-{uuid}` slug. `t.Cleanup()` CASCADE-deletes the enterprise and all
  child rows. Full isolation, parallel-safe.
- **`TestEnterpriseID`** (`99999999-...`) exists in seed data for Playwright browser tests and
  any code needing a stable enterprise reference. Integration tests do NOT use it for data.
- **Preflight cleanup** (`make dev-test` runs `test-postconditions.sh --preflight` before tests)
  deletes orphaned `test-%` enterprises from crashed previous runs.
- **Postconditions** use slug-based leak detection: `test-%` enterprises are expected, anything
  else (excluding seed enterprise) is flagged as a leak.

### Prompt

```
This session is COMPLETE. The deliverables below describe what was implemented.
If re-running or extending, read the Design Decision section above — do NOT migrate
integration tests to a shared enterprise pattern. The per-test enterprise pattern
with CASCADE cleanup is the correct approach.
```

### Deliverables
- Test enterprise (`99999999-...`) in seed data (for Playwright / stable references)
- `testutil.TestEnterpriseID` constant + `EnsureTestEnterprise` helper
- `testutil.CreateTestEnterprise` as the primary integration test pattern (per-test isolation)
- Preflight cleanup in `test-postconditions.sh --preflight` (cleans orphaned `test-%` enterprises)
- Slug-based leak detection in postconditions (test enterprises are expected, not flagged)
- `make dev-test` and `make prod-test` run preflight before tests

---

## Session 9: Service Layer Consolidation

**ID**: `AUT-09`
**Effort**: ~1 day (3 parts, ~2-3 hours each)
**Source**: Maintainability analysis (2026-04-30)
**Branch**: `feature/aut-09-service-consolidation`
**Depends on**: None (refactor only — no new features)

### Prompt

```
Read `.kiro/steering/STEERING.md` and `.kiro/steering/SESSION_NOTES.md` for project conventions.
Read `docs/planning/future/autonomous_sessions.md` Session 9 for task list.

Consolidate the codebase so all handlers use the service layer (`internal/service/`)
instead of calling repositories directly. Currently, pre-Sprint 4 handlers call repos
from the handler; post-Sprint 4 handlers go through services. This creates two patterns
that confuse new developers.

Branch: `feature/aut-09-service-consolidation` from `main`.
Commit per sub-task with `AUT-09:` prefix. Push after each commit.
Run `go test -race ./...` after every change.
Follow the Mandatory Verification Gates in the Prerequisites section — rebuild Docker,
hit real endpoints, and verify dashboard rendering after every sub-task.

CRITICAL: This is a pure refactor. No API behavior changes. No database changes.
Every existing test must continue to pass. If a test breaks, the refactor is wrong — fix
the refactor, not the test.

### Phase 0: Verify call counts (before any code changes)

The repo call counts below were estimated at spec time and may be stale. Before starting,
grep each handler file for direct repo calls and update the counts. If a handler file has
already been migrated to services (by a previous session), skip it. Commit the verified
counts as `AUT-09: verified repo call counts` before proceeding.

### Part A: New Services + Easy Handlers (~2 hours)

1. Create `internal/service/enterprise.go` — `EnterpriseService` wrapping enterprise repo
   with CRUD methods (Create, Get, List, Update, Delete). Follow existing service patterns
   (accept repo interface via constructor, no net/http imports).
   Write service-level tests in `internal/service/enterprise_test.go`.
2. Create `internal/service/enrollment_token.go` — `EnrollmentTokenService` wrapping
   enrollment token repo (Create, List, Revoke, GetByToken, Validate, DecrementUses, SetStatus).
   Write service-level tests.
3. Create `internal/service/command.go` — `CommandService` wrapping command repo and device
   repo (ListByDevice, Create) and cert repo (List for device certs). Note: DeviceService
   already has Lock/Wipe/Restart — CommandService handles the lower-level command record
   CRUD and cert listing.
   Write service-level tests.
4. Extend `internal/service/report.go` (or create it) — wrap audit log repo Search/List.
   Write service-level tests.
5. Rewire `handlers_enterprise.go` — replace all 6 `s.enterpriseRepo.*` calls with
   `s.enterpriseService.*` calls. Update Server struct and constructor.
6. Rewire `handlers_command.go` — replace all 8 repo calls with service calls.
7. Rewire `handlers_enrollment_token.go` — replace all 6 repo calls with service calls.
8. Rewire `handlers_report.go` — replace 2 repo calls with service calls.
9. Clean up: remove repo fields from Server struct that are no longer referenced directly.
   Verify: `go test -race ./...` must pass. Rebuild Docker and hit each affected endpoint.

### Part B: Mixed Handlers (~3 hours)

10. Add missing read methods to `PolicyService`: Get, List, Delete, ListTemplates, and any
    other repo methods called directly from `handlers_policy.go`. Write tests.
11. Rewire `handlers_policy.go` — replace all 9 remaining repo calls with service calls.
12. Add missing read method to `DeviceService` if needed (check `handlers_device.go`).
    Rewire the 1 remaining repo call.
13. Rewire `handlers_compliance.go` — replace 1 repo call with service call.
14. Rewire `web_handlers.go` — replace all 10 repo calls with service calls. These are
    dashboard handlers that should call the same services the API handlers use.
15. Rewire `web_handlers_pages.go` — replace all 16 repo calls with service calls.
    Verify: rebuild Docker, visit every dashboard page, confirm rendering is identical.

### Part C: Platform Handlers (~2 hours)

16. Create `internal/service/enrollment.go` — `EnrollmentService` encapsulating the
    multi-step enrollment flow: enterprise lookup, device creation/update, token validation
    and decrement. This is the trickiest part — enrollment handlers have protocol-specific
    logic interleaved with business logic. Extract only the business logic (enterprise
    validation, device record management, token handling). Leave protocol parsing (SOAP XML,
    SCEP) in the handlers.
    Write service-level tests.
17. Rewire `platform_handlers.go` — replace 8 repo calls with enrollment service calls.
    Verify: rebuild Docker, test Windows discovery endpoint, test macOS enrollment profile
    endpoint. If real VMs are available, do a full enrollment test.
18. Final cleanup: verify no handler file imports `internal/repository` directly (they
    should only import `internal/service` and `internal/models`). Remove any unused repo
    fields from Server struct.

Acceptance criteria:
- Zero handler files call repo methods directly — all go through services
- All existing tests pass with `go test -race ./...` (no test modifications allowed
  except updating mock setup in handler test helpers)
- API behavior is identical — same request/response for every endpoint
- Dashboard renders identically — every page loads without errors
- Server struct holds services, not repos (repos are internal to services)
- New service tests exist for every new service created

When complete, perform this retrospective:
1. List every task where you made a shortcut, used a stub, skipped a test, or deviated from the spec.
2. Does everything align with the existing codebase? Any dependencies broken or gaps?
3. How's test coverage and documentation? Are they still accurate?
4. Did any handler need logic beyond "parse request → call service → format response"?
   If so, should that logic move to the service?
5. Are there any repo methods that are now only called from services and could have their
   visibility reduced?

After the retrospective, provide:
- Summary of work completed
- Checklist of how the owner can verify the work (specific URLs to visit, commands to run)
- Before/after count: how many handler files still import repository directly (should be 0)
```

### Deliverables
- New services: `EnterpriseService`, `EnrollmentTokenService`, `CommandService`, `ReportService`, `EnrollmentService`
- Extended services: `PolicyService` (read methods), `DeviceService` (if needed)
- All handlers rewired to use services exclusively
- Service-level tests for every new service
- Server struct cleaned up — holds services, not repos
- Zero behavior changes to API or dashboard

---

## Session 10: HTTP Client Timeouts & Context Propagation

**ID**: `AUT-10`
**Status**: COMPLETE
**Effort**: ~2 hours (actual: ~2 hours implementation + ~1 hour codebase audit follow-up)
**Source**: Code audit (2026-04-30)
**Branch**: `feature/aut-10-http-timeouts`
**Depends on**: None

### Prompt

```
Read `.kiro/steering/STEERING.md` and `.kiro/steering/SESSION_NOTES.md` for project conventions.
Read `docs/planning/future/autonomous_sessions.md` Session 10 for task list.

Fix HTTP calls that lack timeouts and database calls that ignore request context.
These are production safety issues — if an upstream service (Keycloak, NanoMDM) hangs,
goroutines block forever.

**Note:** Line numbers below are from the 2026-04-30 code audit and may have shifted.
Search for the pattern (e.g., `http.Post(`, `context.Background()`) if the line has moved.

Branch: `feature/aut-10-http-timeouts` from `main`.
Commit per sub-task with `AUT-10:` prefix. Push after each commit.
Run `go test -race ./...` after every change.

Tasks:
1. `internal/auth/keycloak.go` lines 75 and 103: two bare `http.Post()` calls with no
   context or timeout. These are user-facing login and token refresh. Replace with
   `http.NewRequestWithContext` using the caller's context, and use a client with a
   30-second timeout. The functions need a `ctx context.Context` parameter added.
   Update all callers.
2. `internal/api/web_session.go` line 218: bare `http.PostForm()` for OAuth callback
   token exchange. Replace with `http.NewRequestWithContext` using `r.Context()` and
   a client with a 30-second timeout.
3. `internal/auth/oidc.go` line 402: `http.DefaultClient.Do(req)` in HealthCheck.
   Replace with a package-level client that has a 10-second timeout. The request
   already uses `NewRequestWithContext` so this is just the client-level fallback.
4. `internal/scep/challenge.go`: all three methods (`GenerateChallenge`,
   `ValidateChallenge`, `CleanupExpired`) use `context.Background()`. Add
   `ctx context.Context` as the first parameter to each. Update the `ChallengeStore`
   interface, the concrete implementation, all callers in `scep/handler.go` and
   `platform_handlers.go` and `server.go`, and any test mocks.
5. `internal/service/token.go` line 90: fire-and-forget goroutine with
   `context.Background()` and discarded error. Add a 5-second timeout context and
   log errors instead of discarding them.

Acceptance criteria:
- No bare `http.Post`, `http.PostForm`, or `http.DefaultClient` usage in non-test code
- No `context.Background()` in code that has a request context available
- All existing tests pass with `go test -race ./...`
- Verify: `grep -rn 'http\.Post\b\|http\.PostForm\|http\.DefaultClient' --include='*.go' | grep -v _test.go | grep -v vendor` returns nothing

When complete, provide a summary and verification checklist.
```

### Deliverables
- All HTTP calls use clients with explicit timeouts
- SCEP `ChallengeStore` interface accepts context
- `KeycloakClient.Login`/`RefreshToken` accept context
- `OIDCValidator.ValidateToken` accepts context; middleware passes `r.Context()`
- Token update goroutine has timeout and error logging
- Command dispatcher has 30s timeout context
- SCEP PostIssueHook has 5s timeout context
- `refreshGaugeMetrics` accepts parent context
- Timeout verification tests for Keycloak client and command dispatcher
- Full codebase audit: all remaining `context.Background()` verified as legitimate
- Zero behavior changes to API or dashboard

---

## Session 11: Device Re-enrollment & Soft-Delete Fix

**ID**: `AUT-11`
**Status**: COMPLETE
**Effort**: ~2 hours (actual: ~3 hours including follow-up fixes for NULL scan, RemoveProfile-on-delete, real device testing)
**Source**: Code audit (2026-04-30) — unique constraint blocks re-enrollment of deleted devices
**Branch**: `feature/aut-11-reenrollment-fix`
**Depends on**: None

### Problem

The `devices` table has `UNIQUE(enterprise_id, platform, device_id)` but soft-deletes set
`deleted_at` without removing the row. When a device is deleted from the dashboard and then
re-enrolled into the same enterprise, the INSERT fails because the soft-deleted row still
occupies the unique slot. The device silently fails to create, and the macOS Authenticate
webhook falls back to `defaultEnterpriseID`, potentially placing the device in the wrong
enterprise or failing entirely.

Additionally, the macOS Authenticate webhook always uses `defaultEnterpriseID` for new
devices. If a device record doesn't exist at Authenticate time (deleted, or SCEP creation
failed), the device gets assigned to the wrong enterprise.

### Prompt

```
Read `.kiro/steering/STEERING.md` and `.kiro/steering/SESSION_NOTES.md` for project conventions.
Read `docs/planning/future/autonomous_sessions.md` Session 11 for task list.

Fix device re-enrollment after soft-delete and close the macOS multi-tenant gap.

Branch: `feature/aut-11-reenrollment-fix` from `main`.
Commit per sub-task with `AUT-11:` prefix. Push after each commit.
Run `go test -race ./...` after every change.

Tasks:
1. In the device repository `Create` method: if the INSERT fails due to a unique constraint
   violation, query for a soft-deleted device with the same (enterprise_id, platform, device_id).
   If found, restore it: clear `deleted_at`, reset `status` to 'enrolled', update
   `enrollment_date` to NOW(), and return the restored device. This preserves the device UUID
   and audit trail. If no soft-deleted record is found, return the original error.
   Write an integration test: create device → soft-delete → create again with same IDs →
   verify same UUID is returned with cleared deleted_at.

2. In `internal/platform/macos/webhook.go` Authenticate handler (around line 211): when
   `GetDeviceByUDID` fails (device not found), before falling back to `defaultEnterpriseID`,
   attempt to look up the device including soft-deleted records. Add a repository method
   `GetByPlatformIDIncludeDeleted(ctx, platform, deviceID)` that queries WITHOUT the
   `deleted_at IS NULL` filter. If a soft-deleted record is found, use its `enterprise_id`
   instead of the default. If NO record exists at all (not even soft-deleted), do NOT create
   a device in the default enterprise — log a warning with the UDID and return without
   creating. A device with a valid cert but no record is an anomaly. Remove the
   `defaultEnterpriseID` field and `SetDefaultEnterpriseID` method entirely — they are no
   longer needed.
   Write unit tests:
   - Mock GetDeviceByUDID returns not-found, GetByPlatformIDIncludeDeleted returns a
     soft-deleted device → verify correct enterprise ID is used for restoration.
   - Mock both lookups return not-found → verify no device is created and a warning is logged.

3. Write an end-to-end integration test covering the full flow:
   - Create enterprise and device
   - Soft-delete the device
   - Simulate re-enrollment (call Create with same enterprise/platform/device_id)
   - Verify device is restored with same UUID, correct enterprise, enrolled status
   - Verify a second enterprise can enroll the same UDID independently

4. Update DATABASE.md to document the soft-delete re-enrollment behavior.

Acceptance criteria:
- Deleting a device and re-enrolling it into the same enterprise restores the original record
- Deleting a device and enrolling it into a different enterprise creates a new record
- macOS Authenticate handler uses the correct enterprise for re-authenticating deleted devices
- macOS Authenticate handler rejects devices with no record (active or deleted) — logs warning, does not create
- `defaultEnterpriseID` is removed — no silent fallback to a default tenant
- All existing tests pass with `go test -race ./...`
- No migration needed (fix is in application logic, not schema)

When complete, provide a summary and verification checklist.
```

### Deliverables
- Device repository handles re-enrollment of soft-deleted devices (unique constraint → restore)
- `GetByPlatformIDIncludeDeleted` added to DeviceRepository interface
- macOS webhook uses correct enterprise for deleted device re-authentication; `defaultEnterpriseID` removed
- Unknown devices (no record active or deleted) rejected with warning log
- NULL column scan fix: `COALESCE` on nullable VARCHAR columns via shared `deviceSelectColumns` constant
- `DeviceService.Delete` sends `RemoveProfile` to NanoMDM for macOS devices before soft-deleting
- `RemoveProfile` added to command dispatcher's macOS switch
- Integration tests: re-enrollment after soft-delete, duplicate active rejection, multi-enterprise UDID, `GetByPlatformIDIncludeDeleted`
- E2E test: full re-enrollment lifecycle (create → delete → re-enroll → verify UUID preserved)
- DATABASE.md: soft-delete re-enrollment behavior, Delete vs Unenroll flows documented
- Real device verified: delete from dashboard → VM reboot → RemoveProfile delivered → device unenrolled

---

## Session Dependency Graph

```
AUT-00  (Test Enterprise)        — complete ✅
AUT-01  (Enrollment Tokens)      — complete ✅
AUT-02  (Dynamic Groups)         — design-first: Phase 1 produces design doc for review
AUT-03a (Device Tags)            — independent, autonomous
AUT-03b (Bulk Ops)               — depends on AUT-03a, design-first for confirmation UX
AUT-04  (Webhooks)               — independent, investigation step then autonomous
AUT-05  (Dry-Run + Scheduling)   — design-first: Phase 1 produces design doc for review
AUT-06a (Security Audit + CI)    — independent, autonomous
AUT-06b (Compliance Reports)     — independent, autonomous
AUT-06c (Cert Renewal + Escrow)  — investigation-first, then autonomous
AUT-07  (A11y + i18n)            — independent, screenshot gate per template
AUT-08  (Docs + OTel)            — investigation step then autonomous
AUT-09  (Service Consolidation)  — run BEFORE feature sessions (reduces merge conflicts)
AUT-10  (HTTP Timeouts)          — complete ✅
AUT-11  (Re-enrollment Fix)      — complete ✅
```

Sessions marked "design-first" will stop after Phase 1 and wait for owner review.
Sessions marked "investigation step" will commit findings before proceeding autonomously.
Sessions marked "autonomous" can run start-to-finish without human interaction.

## Recommended Execution Order

0. **AUT-00** (Test Enterprise Isolation) ✅
1. **AUT-01** (Enrollment Tokens) ✅
2. **AUT-10** (HTTP Timeouts) ✅
3. **AUT-11** (Re-enrollment Fix) ✅
4. **AUT-09** (Service Consolidation) — autonomous, clean up before features
5. **AUT-06a** (Security Audit + CI) — autonomous, low risk
6. **AUT-06b** (Compliance Reports) — autonomous, moderate risk
7. **AUT-03a** (Device Tags) — autonomous, highest admin UX value
8. **AUT-04** (Webhooks) — investigation step then autonomous
9. **AUT-03b** (Bulk Ops) — design-first, needs UX review
10. **AUT-02** (Dynamic Groups) — design-first, needs rule engine review
11. **AUT-05** (Dry-Run + Scheduling) — design-first, needs compliance integration review
12. **AUT-06c** (Cert Renewal + Escrow) — investigation-first, needs platform knowledge
13. **AUT-07** (A11y + i18n) — autonomous with screenshot gates
14. **AUT-08** (Docs + OTel) — investigation step then autonomous

Autonomous sessions (2-7 above) can run overnight. Design-first sessions (9-12) need
a morning review of the design doc before implementation proceeds.
