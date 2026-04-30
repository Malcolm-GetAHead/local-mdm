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

---

## Session 1: Enrollment Token System

**ID**: `AUT-01`
**Effort**: ~1 day
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

Tasks:
1. Create migration: `enrollment_tokens` table (id, enterprise_id FK, token VARCHAR(64) UNIQUE,
   description, max_uses, uses_remaining, expires_at, created_by, created_at, revoked_at).
2. Repository: `EnrollmentTokenRepository` interface + PostgreSQL implementation following
   Writer/Reader pool pattern. Methods: Create, GetByToken, List, Revoke, DecrementUses.
3. API endpoints (behind auth middleware):
   - POST /api/v1/enrollment-tokens — create token (enterprise_id, description, max_uses, expires_in)
   - GET /api/v1/enrollment-tokens — list tokens for enterprise
   - DELETE /api/v1/enrollment-tokens/{id} — revoke token
4. Modify Windows enrollment handler (`handleWindowsDiscoveryService` or enrollment path):
   extract token from email local part, validate against enrollment_tokens table (not expired,
   uses_remaining > 0, not revoked), decrement uses_remaining. Reject with clear error if invalid.
5. Modify macOS enrollment profile handler: accept optional `?token=` query param, validate
   same way. Include token in SCEP challenge metadata so it can be validated during SCEP enrollment.
6. Dashboard page: "Enrollment Tokens" nav item. List tokens (token, enterprise, uses remaining,
   expires, created by, status). Create form (description, max uses, expiry). Revoke button.
   Copy-to-clipboard for the enrollment email address. Use HTMX patterns matching existing pages.
7. Tests: repository integration tests (Docker PostgreSQL), handler unit tests with mock repo,
   token validation edge cases (expired, exhausted, revoked, valid).
8. Audit logging on create, use, and revoke actions.

Acceptance criteria:
- Creating a token returns a copyable email address (e.g., `abc123@localmdm.local`)
- Enrolling with a valid token succeeds and decrements uses_remaining
- Enrolling with expired/exhausted/revoked token returns a clear error
- Dashboard shows token list with create/revoke actions
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

Implement dynamic device groups — groups whose membership is automatically computed from
filter rules instead of manual assignment. Static groups (S4-02) already exist with manual
membership. Dynamic groups extend this with a `type` column and `rules` JSONB column.

Branch: `feature/aut-02-dynamic-groups` from `main`.
Commit per sub-task with `AUT-02:` prefix. Push after each commit.
Run `go test -race ./...` after every change.

Tasks:
1. Migration: add `type VARCHAR(10) DEFAULT 'static'` and `rules JSONB` columns to `device_groups`.
   Add CHECK constraint: type IN ('static', 'dynamic'). Dynamic groups must have non-null rules.
2. Rule engine (`internal/service/dynamic_groups.go`): define Rule struct (Field, Operator, Value).
   Supported fields: platform, os_version, status, model, name (from device columns).
   Supported operators: equals, not_equals, contains, starts_with, greater_than, less_than, in.
   Evaluate function takes a device and a list of rules, returns bool (AND logic — all must match).
3. Evaluation service: query all devices for an enterprise, evaluate each against group rules,
   compute adds/removes vs current membership, apply changes. Log membership changes to audit log.
4. EventBus subscriber: register a subscriber that re-evaluates dynamic groups when devices are
   created, updated, or deleted. Also add a periodic re-evaluation (every 15 minutes) using a
   background goroutine with context cancellation.
5. API: extend existing group endpoints:
   - POST /api/v1/groups — accept `type` and `rules` fields
   - GET /api/v1/groups/{id} — return rules in response
   - PUT /api/v1/groups/{id} — allow updating rules (triggers re-evaluation)
   - POST /api/v1/groups/{id}/evaluate — manual trigger for re-evaluation
6. Dashboard: extend group create form with type toggle (static/dynamic). When dynamic is selected,
   show a rule builder UI (field dropdown, operator dropdown, value input, add/remove rule buttons).
   Group detail page shows current rules and a "Re-evaluate Now" button. Use HTMX.
7. Tests: rule evaluation unit tests (all operators, edge cases), integration tests for membership
   computation, handler tests for CRUD with rules, EventBus subscriber test.

Acceptance criteria:
- Creating a dynamic group with rules `[{field: "platform", operator: "equals", value: "macos"}]`
  auto-populates membership with all macOS devices
- Adding a new macOS device triggers re-evaluation and adds it to the group
- Changing a device's platform removes it from the group on next evaluation
- Manual re-evaluation via API and dashboard button works
- Static groups are unaffected — no behavioral changes
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
- Migration adding `type` and `rules` columns
- `internal/service/dynamic_groups.go` + rule engine + tests
- EventBus subscriber for auto-evaluation
- Extended API handlers and dashboard UI
- Background periodic evaluator with graceful shutdown

---

## Session 3: Custom Device Tags & Bulk Operations

**ID**: `AUT-03`
**Effort**: ~1 day
**Source**: F-07 §Custom Device Attributes, §Bulk Operations UI
**Branch**: `feature/aut-03-tags-bulk-ops`
**Depends on**: None

### Prompt

```
Read `.kiro/steering/STEERING.md` and `.kiro/steering/SESSION_NOTES.md` for project conventions.
Read `docs/planning/future/F-07-advanced-features.md` §Custom Device Attributes and §Bulk Operations UI.
Read `docs/planning/future/autonomous_sessions.md` Session 3 for task list.

Implement two related features: custom device tags (key-value metadata on devices) and
bulk operations (multi-select devices and perform actions on all selected).

Branch: `feature/aut-03-tags-bulk-ops` from `main`.
Commit per sub-task with `AUT-03:` prefix. Push after each commit.
Run `go test -race ./...` after every change.

### Part A: Custom Device Tags

Tasks:
1. Migration: create `device_tags` table (device_id FK, key VARCHAR(64), value VARCHAR(256),
   PRIMARY KEY (device_id, key)). Index on (key, value) for filtering.
2. Repository: `DeviceTagRepository` — SetTags(ctx, deviceID, tags map[string]string),
   GetTags(ctx, deviceID), DeleteTag(ctx, deviceID, key), ListDevicesByTag(ctx, enterpriseID, key, value).
3. API endpoints:
   - PUT /api/v1/devices/{id}/tags — set tags (merge with existing)
   - GET /api/v1/devices/{id}/tags — get all tags
   - DELETE /api/v1/devices/{id}/tags/{key} — remove a tag
   - Extend GET /api/v1/devices with `?tag=key:value` filter parameter
4. Dashboard: add a "Tags" section to device detail page. Inline edit with add/remove.
   Add tag filter to device list page (text input, format `key:value`).
5. Tests: repository integration tests, handler tests, filter tests.

### Part B: Bulk Operations

Tasks:
6. Dashboard device list: add checkboxes to each row. "Select All" checkbox in header.
   Selected count indicator. Actions dropdown appears when ≥1 device selected.
7. Actions dropdown: Lock Selected, Wipe Selected, Add Tag, Remove Tag, Assign Policy.
   Each action shows a confirmation modal (HTMX) listing affected devices.
   Destructive actions (lock, wipe) require typing the action name to confirm.
8. API endpoint: POST /api/v1/devices/bulk — accepts {action, device_ids[], params}.
   Actions: lock, wipe, add_tag, remove_tag, assign_policy. Returns per-device results.
9. Implementation: bulk handler iterates device_ids, calls existing service methods
   (deviceService.Lock, deviceService.Wipe, etc.), collects results. Audit log per device.
10. Tests: bulk handler tests (partial success, all fail, all succeed), confirmation UI tests.

Acceptance criteria:
- Tags can be set, viewed, and removed on device detail page
- Device list can be filtered by tag (e.g., `?tag=department:engineering`)
- Selecting multiple devices shows action dropdown
- Bulk lock sends lock command to all selected devices
- Confirmation modal shows device list and requires typed confirmation for destructive actions
- Partial failures return per-device results (some succeed, some fail)
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
- Migration for `device_tags` table
- Tag repository + API + dashboard UI
- Bulk operations API endpoint + dashboard multi-select UI
- Confirmation modals with typed confirmation for destructive actions

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

Tasks:
1. Migration: `webhooks` table (id, enterprise_id FK, url, events TEXT[], secret VARCHAR(128)
   for HMAC-SHA256 signing, active BOOLEAN, description, failure_count INT DEFAULT 0,
   last_failure_at, created_at, updated_at). `webhook_deliveries` table (id, webhook_id FK,
   event_type, payload JSONB, status VARCHAR(20), response_code INT, attempts INT, next_retry_at,
   created_at).
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
6. Dashboard page: "Webhooks" under settings or as nav item. List webhooks with status
   (active/disabled/failing). Create form with URL, event checkboxes, secret field.
   Detail page shows recent deliveries with status codes. Test button. Enable/disable toggle.
7. Event types to support: device.enrolled, device.unenrolled, device.wiped, policy.assigned,
   policy.removed, compliance.changed, command.completed.
8. Tests: dispatcher unit tests (mock HTTP server), retry logic tests, HMAC signature
   verification test, handler tests, auto-disable after consecutive failures test.

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

Implement two features: policy dry-run (simulate what would happen if a policy were applied)
and scheduled deployment (apply policies at a future time with optional canary rollout).

Branch: `feature/aut-05-dryrun-scheduling` from `main`.
Commit per sub-task with `AUT-05:` prefix. Push after each commit.
Run `go test -race ./...` after every change.

### Part A: Policy Dry-Run

Tasks:
1. Service method: `PolicyService.DryRun(ctx, policyID, targetType, targetID)` — evaluates
   the policy against target devices (device, group, or enterprise) WITHOUT deploying.
   Returns per-device results: current state, what would change, predicted compliance status.
2. API endpoint: POST /api/v1/policies/{id}/dry-run — body: {target_type, target_id}.
   Returns: {devices: [{device_id, name, current_compliance, predicted_compliance, changes: [...]}]}.
3. Dashboard: "Dry Run" button on policy detail page. Opens a modal to select target
   (device, group, or all). Shows results in a table: device name, current vs predicted
   compliance, list of changes that would be applied. Green/red indicators.
4. Tests: dry-run service tests with mock repos, handler tests, edge cases (empty group,
   already-compliant devices, policy with no applicable settings for platform).

### Part B: Scheduled Deployment

Tasks:
5. Migration: `scheduled_deployments` table (id, policy_id FK, target_type, target_id,
   scheduled_at TIMESTAMPTZ, strategy VARCHAR(20) — 'immediate' or 'canary',
   canary_percentage INT, status VARCHAR(20) — 'pending'/'in_progress'/'completed'/'cancelled',
   created_by, created_at, completed_at).
6. Repository: ScheduledDeploymentRepository — Create, List, GetByID, Cancel,
   GetPendingBefore(time), MarkInProgress, MarkCompleted.
7. Scheduler worker: background goroutine that polls every 60 seconds for deployments
   where scheduled_at <= now() and status = 'pending'. Executes deployment using existing
   policy assignment logic. For canary: deploy to N% of target devices first, mark as
   in_progress, deploy remainder on next cycle (or after manual approval — for autonomous
   implementation, auto-complete after 1 hour if no failures).
8. API endpoints:
   - POST /api/v1/policies/{id}/schedule — create scheduled deployment
   - GET /api/v1/scheduled-deployments — list all scheduled deployments
   - DELETE /api/v1/scheduled-deployments/{id} — cancel a pending deployment
9. Dashboard: "Schedule" button on policy assign page. Date/time picker, strategy toggle
   (immediate/canary with percentage slider). Scheduled deployments list page showing
   upcoming and past deployments with status.
10. Tests: scheduler worker tests (mock time), canary logic tests, cancellation tests,
    handler tests.

Acceptance criteria:
- Dry-run shows accurate predicted compliance without modifying any data
- Scheduling a deployment for 5 minutes in the future executes it automatically
- Canary deployment applies to the configured percentage first
- Cancelling a pending deployment prevents execution
- Dashboard shows scheduled deployments with status
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
- Dry-run service method + API + dashboard modal
- Migration for `scheduled_deployments` table
- Scheduler worker with canary support
- API endpoints + dashboard scheduling UI

---

## Session 6: Security Hardening

**ID**: `AUT-06`
**Effort**: ~1 day
**Source**: F-03 §mTLS, §Security Audit Logging, §Compliance Reporting, §Vulnerability Scanning, §Recovery Key Escrow
**Branch**: `feature/aut-06-security-hardening`
**Depends on**: None

### Prompt

```
Read `.kiro/steering/STEERING.md` and `.kiro/steering/SESSION_NOTES.md` for project conventions.
Read `docs/planning/future/F-03-advanced-security.md` for the full security spec.
Read `docs/planning/future/autonomous_sessions.md` Session 6 for task list.

Implement security hardening features that don't require external services (no HSM, no
cloud accounts). Focus on: security audit logging, compliance report generation, CI
vulnerability scanning, certificate auto-renewal, and encryption recovery key escrow.

Branch: `feature/aut-06-security-hardening` from `main`.
Commit per sub-task with `AUT-06:` prefix. Push after each commit.
Run `go test -race ./...` after every change.

Tasks:
1. Security audit log: create `security_audit_logs` table (separate from operational audit_logs).
   Fields: id, timestamp, event_type (auth_failure, permission_denied, token_revoked,
   cert_issued, cert_revoked, admin_action), severity (info/warning/critical), actor,
   source_ip, details JSONB. Repository + service. Wire into auth middleware (failed logins),
   cert service (issuance/revocation), and admin actions (user role changes).
2. Compliance report generation: `ReportService.GenerateComplianceReport(ctx, enterpriseID, format)`
   — produces a structured report with: executive summary (total devices, compliant %,
   non-compliant count), per-policy breakdown, per-device detail, trend data (if historical
   compliance results exist). Output formats: JSON and CSV. API endpoint:
   GET /api/v1/reports/compliance?format=json|csv. Dashboard: "Export Report" button on
   compliance page.
3. CI vulnerability scanning: create `.github/workflows/security.yml` with:
   - `govulncheck ./...` (Go vulnerability database)
   - `trivy fs --scanners vuln,secret .` (dependency + secret scanning)
   - Run on push to main and on PRs
   - Fail the build on HIGH/CRITICAL vulnerabilities
4. Certificate auto-renewal: `CertRenewalService` that checks device certificate expiry.
   Query devices where cert expires within 30 days. For each, queue a re-enrollment command
   (platform-specific: macOS InstallProfile with new SCEP payload, Windows certificate renewal
   CSP). Background worker runs daily. Dashboard: "Expiring Certificates" widget on home page
   showing count of certs expiring in 7/30/90 days.
5. Recovery key escrow: migration adding `recovery_keys` table (id, device_id FK, key_type
   VARCHAR — 'bitlocker' or 'filevault', encrypted_key TEXT — pgcrypto encrypted, escrowed_at).
   Repository methods. API endpoint to retrieve key (admin only, audit logged).
   For macOS: parse FileVault recovery key from SecurityInfo command response and store.
   For Windows: parse BitLocker recovery key from OMA-DM response and store.
   Dashboard: "Recovery Key" button on device detail page (admin only), shows key in a
   modal with copy button. Every access is audit logged.
6. Tests: security audit log tests, report generation tests (verify CSV output format),
   cert renewal scheduling tests, recovery key encryption/decryption round-trip tests.

Acceptance criteria:
- Failed login attempts appear in security audit log
- Compliance report exports as JSON and CSV with accurate data
- CI workflow runs govulncheck and trivy (verify YAML is valid)
- Cert renewal worker identifies expiring certs (test with mock data)
- Recovery keys are stored encrypted and retrievable by admins only
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
- Security audit log table + repository + service + middleware integration
- Compliance report generation (JSON + CSV) + API + dashboard export button
- `.github/workflows/security.yml` CI workflow
- Certificate renewal service + background worker
- Recovery key escrow table + encrypted storage + admin retrieval UI

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
11. Tests: i18n middleware tests (header parsing, cookie, fallback), translation lookup tests
    (missing key, missing locale), Pa11y test runner.

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
2. Take screenshots using Playwright scripts (login, navigate, screenshot) for key pages.
   Save to `docs/user-guide/images/`. Reference in markdown docs.
3. Create `docs/user-guide/README.md` as table of contents.

### Part B: OpenTelemetry Instrumentation

Tasks:
4. Add OpenTelemetry SDK dependency: `go.opentelemetry.io/otel` and the OTLP exporter.
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

## Session Dependency Graph

```
AUT-01 (Enrollment Tokens)     — independent
AUT-02 (Dynamic Groups)        — independent
AUT-03 (Tags + Bulk Ops)       — independent
AUT-04 (Webhooks)              — independent
AUT-05 (Dry-Run + Scheduling)  — independent
AUT-06 (Security Hardening)    — independent
AUT-07 (A11y + i18n)           — independent
AUT-08 (Docs + OTel)           — independent
```

All sessions are independent and can run in any order. If running multiple sessions
sequentially, each should merge to `main` before the next starts to avoid conflicts.

## Recommended Execution Order

1. **AUT-01** (Enrollment Tokens) — highest security value, closes a real gap
2. **AUT-03** (Tags + Bulk Ops) — highest admin UX value
3. **AUT-04** (Webhooks) — enables external integrations
4. **AUT-02** (Dynamic Groups) — builds on tags for auto-membership
5. **AUT-05** (Dry-Run + Scheduling) — policy management maturity
6. **AUT-06** (Security Hardening) — production readiness
7. **AUT-07** (A11y + i18n) — compliance and reach
8. **AUT-08** (Docs + OTel) — operational maturity
