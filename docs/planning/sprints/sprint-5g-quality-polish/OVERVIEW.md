# Sprint 5g: Quality Polish

**Status**: 🔲 Not Started
**Branch**: `sprint-5g/quality-polish`
**Duration**: ~1 day
**Goal**: Fix real issues surfaced by external code review — N+1 queries, missing loading states, empty state UX, error test coverage, and developer workflow
**Depends on**: Sprint 5d (web dashboard) — merged to main

---

## Context

Three external assessment documents (antigravity_be/fe/tests) were reviewed against the actual codebase. Of 18 claims, 8 were inaccurate (contradicted project decisions or misread the code), 8 were partially accurate, and 1 was fully accurate. This sprint implements the valid kernels extracted from the partially accurate and accurate findings.

### Rejected Recommendations (with rationale)

| Recommendation | Why Rejected |
|---|---|
| Redis Streams for EventBus | Redis was deliberately removed in Sprint 4. EventBus uses a single `pq.Listener` connection — does not exhaust pools. |
| json-iterator | Premature optimization. Enterprise MDM manages hundreds–thousands of devices, not millions. No profiling evidence of JSON bottleneck. |
| Circuit breakers on DB calls | Already exist (`internal/auth/circuit_breaker.go`). Health/readiness probes also exist. |
| Glassmorphism UI overhaul | Already has 10 themes × 2 modes with varied visual styles, shadows, custom radii. |
| Google Fonts | System fonts are deliberate — zero GDPR risk, zero FOUT, zero external dependency. Critical for enterprise/air-gapped deployments. |
| Alpine.js migration | Event delegation in `app.js` is architecturally required for CSP nonce + HTMX body swap compatibility. Alpine directives would conflict with CSP. |
| mockery for mock generation | Hand-written mocks are the established convention (600+ lines in `handler_test_helpers_test.go`, per-service mocks in `internal/service/`). More readable, support error injection. |
| Google Wire for DI | Server init is 293 lines of linear, readable Go. Wire adds code generation complexity for ~15 repos and ~10 services — not warranted. |

---

## Tasks

### S5g-01: Fix N+1 Queries in Web Handlers
**Effort**: ~2 hours
**Files**: `internal/api/web_handlers.go`, `web_handlers_pages.go`, `internal/repository/`

Three web handlers have N+1 query patterns:

1. **`handleWebDeviceDetail`** (web_handlers.go:260–265): Loops over `effectiveAssignments` calling `s.policyRepo.GetByID()` per assignment.
   - Fix: Add `ListByIDs(ctx, []uuid.UUID) ([]*Policy, error)` to PolicyRepository. Single `WHERE id = ANY($1)` query.

2. **`handleWebGroups`** (web_handlers_pages.go:525): Loops over groups calling `s.groupService.ListMembers()` per group to get member counts.
   - Fix: Add `CountMembersByGroupIDs(ctx, []uuid.UUID) (map[uuid.UUID]int, error)` to GroupRepository. Single `SELECT group_id, COUNT(*) FROM group_members WHERE group_id = ANY($1) GROUP BY group_id`.

3. **`buildComplianceRows`** (web_handlers.go:392): Loops over compliance results calling `s.policyRepo.GetByID()` per result.
   - Fix: Collect unique policy IDs, use `ListByIDs` (same as #1), build lookup map.

**Verification**: Run `make dev-test`, confirm Playwright passes, spot-check pages load correctly.

---

### S5g-02: Add HTMX Loading Indicators
**Effort**: ~1 hour
**Files**: `internal/api/templates/base.html`, `web/static/css/input.css`, `web/static/js/app.js`

No loading indicators exist during HTMX content swaps. For a local admin tool responses are fast, but the UI should acknowledge in-progress requests.

Implementation (simple, not skeleton screens):
1. Add a thin progress bar at the top of the page (NProgress-style, CSS-only).
2. Use HTMX events `htmx:beforeRequest` / `htmx:afterRequest` to show/hide via class toggle in `app.js`.
3. CSS: `position: fixed; top: 0; width: 100%; height: 3px; background: var(--accent);` with a CSS animation.

**Not doing**: Full skeleton screens (over-engineering for <50ms responses on local network).

**Verification**: Navigate between pages, confirm bar appears briefly. Run Playwright — no regressions.

---

### S5g-03: Enhance Empty States
**Effort**: ~1.5 hours
**Files**: `internal/api/templates/devices.html`, `policies.html`, `groups.html`, `compliance.html`, `audit.html`

Current empty states are styled text (`text-center text-gray-500 py-8`) but lack visual weight or guidance. Enhance with:

1. Inline SVG illustration per entity (device/policy/group/compliance/audit) — simple line-art icons, ~6 lines of SVG each. No external dependencies (no unDraw CDN).
2. Contextual subtext explaining what to do next (e.g., "Enroll a device to get started").
3. CTA button where applicable (e.g., "Create Policy" on empty policies page).

Keep dark mode support (`dark:` variants). Keep theme compatibility (use `var(--accent)` for CTA buttons).

**Verification**: Temporarily clear seed data, confirm each empty state renders. Run Playwright.

---

### S5g-04: Playwright Error State Tests
**Effort**: ~2 hours
**Files**: `tests/browser/browser-playbook.md` or new `tests/browser/error-states.spec.js`

The 196 existing Playwright tests are all happy-path. No error states are tested.

**Decision needed**: Extend the playbook DSL with error simulation syntax, or write separate standard Playwright spec files for error scenarios. Recommendation: **separate spec file** — error testing requires `page.route()` interception which doesn't fit the declarative playbook DSL.

Test scenarios:
1. Server returns 500 on device list → verify error message displayed (not blank page)
2. Server returns 500 on policy create → verify form preserves input and shows error
3. Server returns 404 on device detail → verify "not found" message
4. Delete action fails → verify toast shows error, item still in list

Implementation:
- `tests/browser/error-states.spec.js` using standard Playwright API
- `page.route()` to intercept HTMX requests and return error responses
- Update `make browser-test` to run both playbook and spec files

**Verification**: `make browser-test` runs both test suites.

---

### S5g-05: Unified `make verify` Target
**Effort**: ~30 minutes
**Files**: `Makefile`

Individual targets exist (`test`, `lint`, `dev-test`, `browser-test`, `load-test`) but no single command runs the full local verification suite. Add:

```makefile
verify: ## Run full local verification: lint + unit tests + integration tests + browser tests
	@echo "=== Lint ==="
	@$(MAKE) lint
	@echo "=== Unit Tests ==="
	@$(MAKE) test-unit
	@echo "=== Integration Tests (Docker) ==="
	@$(MAKE) dev-test
	@echo "=== Browser Tests ==="
	@$(MAKE) browser-test
	@echo "✅ All checks passed"
```

Excludes `load-test` (requires running server + takes 5+ minutes — run separately before merge). This is a convenience wrapper, not a replacement for `prod-test` which remains the pre-merge gate.

**Verification**: Run `make verify`, confirm all stages execute in sequence.

---

### S5g-06: Refactor CertificateService and ReportingService to Use Interfaces
**Effort**: ~1.5 hours
**Files**: `internal/certs/service.go`, `internal/reporting/reports.go`, `internal/repository/`, `internal/api/server.go`

Two infrastructure utilities accept raw `*sql.DB` instead of repository interfaces. The service layer in `internal/service/` properly uses interfaces, but these two predate that pattern.

1. **CertificateService** (`internal/certs/service.go:24`): `NewCertificateService(ca *CAManager, db *sql.DB)` → extract `CertificateRepository` interface, create implementation in `internal/repository/`.

2. **ReportingService** (`internal/reporting/reports.go:22`): `NewService(db *sql.DB)` → extract `ReportingRepository` interface for the raw SQL queries (compliance report, enrollment trends, device inventory), create implementation in `internal/repository/`.

Update `server.go` wiring to pass repository implementations instead of `db.Writer`.

**Not refactoring**: AuditLogger, TokenCache, DEPStorage, SCEP ChallengeManager, Metrics, EventBus — these are infrastructure-level components where raw `*sql.DB` is the documented convention (SESSION_NOTES: "Non-repo consumers use Writer pool directly").

**Verification**: `make dev-test` passes. No behavior change — pure refactor.

---

## Effort Summary

| Task | Effort | Risk |
|---|---|---|
| S5g-01: Fix N+1 queries | ~2h | Low — additive repo methods, no behavior change |
| S5g-02: Loading indicators | ~1h | Low — CSS + 5 lines of JS |
| S5g-03: Empty states | ~1.5h | Low — template-only changes |
| S5g-04: Error state tests | ~2h | Low — new test files, no production code changes |
| S5g-05: `make verify` | ~30m | None — Makefile addition |
| S5g-06: Interface refactor | ~1.5h | Low — pure refactor, same behavior |
| **Total** | **~8.5h** | |

---

## Out of Scope

- **data-testid attributes**: Has merit but doesn't fit the playbook DSL architecture. Would require extending the custom runner. Track for future sprint if playbook maintenance becomes painful.
- **Server init refactoring**: 293 lines is long but linear and readable. Not worth the complexity of Wire or similar tools at this scale.
- **Performance optimization**: No evidence of bottlenecks. k6 framework exists for when it's needed.

---

*Created: 2026-04-27*
