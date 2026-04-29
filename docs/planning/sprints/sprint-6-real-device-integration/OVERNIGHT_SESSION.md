`# Sprint 6 Remainder: Autonomous Overnight Session

## Context
Branch: `main`. All tests should pass before you start — run `make dev-test` first to confirm. Read `docs/planning/sprints/sprint-6-real-device-integration/GAPS.md` for full context, especially the "Retro Items" and "Should Fix" sections.

**IMPORTANT**: `make dev-test` destroys real device data in the database. This is expected. Run it freely — the owner will re-enroll devices after reviewing your work.

## Your Mission (fully autonomous — owner is asleep)

Work through these tasks in order. Commit after each task with `S6-XX:` prefix. Push after each commit. Run `make dev-test` after every code change — all 19 packages must pass before committing. If a task is blocked, skip it and move to the next. Do NOT modify any steering files.

### Task 1: Run `make dev-test` baseline
Confirm all 19 packages pass before making any changes. If anything fails, fix it first.

### Task 2: XML marshaling refactor
Replace `fmt.Sprintf` string templates in `internal/platform/windows/discovery.go` `GenerateDiscoverResponse` with proper Go struct XML marshaling (like Fleet DM does). This prevents namespace/formatting bugs. Update the test in `service_test.go` to match. The response must be byte-identical in semantics (same elements, same namespaces, same values) — just generated via `xml.Marshal` instead of string formatting.

### Task 3: Unit tests for Windows enrollment fixes
Add tests to the appropriate `_test.go` files for:
- Discovery response: verify namespace has no trailing slash, version is 4.0, no AuthenticationServiceUrl, Content-Length would be set
- CSR fallback: test `SignRawCSR` with a CSR containing non-PrintableString subject characters
- Enterprise ID from email: test UUID extraction from `00000000-0000-0000-0000-000000000001@localmdm.local`, fallback for `admin@localmdm.local`, edge cases
- Duplicate device upsert: test that re-enrollment with same hardware ID updates instead of failing

### Task 4: Fix enterprise ID fallback
In `internal/api/platform_handlers.go`, the Windows enrollment handler falls back to a hardcoded UUID. Change it to use `s.config.MacOS.DefaultEnterpriseID` (yes, it's in MacOS config — that's where it was added). If that's empty, fall back to the hardcoded UUID. This is a one-line change.

### Task 5: Fix `HandleSyncML` double-parse
In `internal/api/platform_handlers.go`, the OMA-DM sync handler calls `HandleSyncML` then separately calls `ExtractDeviceIDFromSyncML` which re-parses the same XML. Refactor `HandleSyncML` to return the device ID alongside the response bytes, eliminating the double parse.

### Task 6: Generate unique ActivityId
In `internal/platform/windows/discovery.go`, the ActivityId in the discovery response is hardcoded. Generate a fresh `uuid.New()` for each request.

### Task 7: Integration tests for webhook flow
In `internal/platform/macos/webhook_test.go` or a new file, add integration-style tests (using mocks, not Docker) for:
- Authenticate webhook → device created with name/serial/model from plist
- TokenUpdate webhook → device status set to enrolled, push_magic stored
- Acknowledge with SecurityInfo → platform_data updated with FileVault/firewall

### Task 8: Add nginx to `make prod-up`
In the Makefile, update the `prod-up` target to include the nginx-tls service.

### Task 9: Run `make dev-test` final verification
All 19 packages must pass. Check coverage for `internal/platform/windows` and `internal/platform/macos` — report the numbers.

### Task 10: Update CHANGELOG.md
Add entries for whatever you completed in this session under a "Sprint 6 (continued)" heading.

## Rules
- Git: commit per task, `S6-XX:` prefix, push after each commit, never commit to main directly — wait, you ARE on main. Create branch `sprint-6b/cleanup` first, work there, and leave it for the owner to review and merge.
- If `make dev-test` fails after a change, fix it before moving on.
- Do not modify `.kiro/steering/` files.
- Do not start Docker services or restart containers — tests run in their own Docker context via `make dev-test`.
- Do not SSH into VMs or interact with real devices.
- If you finish all tasks, stop. Do not invent new work.
- 