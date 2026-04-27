# Session Notes — Working Preferences & Project Knowledge

**Last Updated**: 2026-04-26  
**Purpose**: Guidance for AI agents working on this codebase. Keep this file lean — patterns and conventions that apply to every session. One-shot implementation details belong in sprint docs, not here.

## Current State

- **Sprint 5d**: ✅ COMPLETE, merged to main (tagged v0.5.4-sprint5d)
- **Sprint 5b**: ✅ COMPLETE, merged to main (via sprint-5d branch)
- **Sprint 5f**: ✅ COMPLETE, merged to main (via sprint-5d branch)
- **Sprint 5g**: ✅ COMPLETE, merged to main
- **Sprint 6**: 🔲 Not Started (Real device integration)
- **Retrospective**: Pending (Sprint 5b)
---

## Working Style

- Ask clarifying questions before implementation starts, not during. Batch all questions upfront.
- Owner trusts the agent to make reasonable decisions — don't ask for approval on every detail.
- "Do it all" requests are common — batch work efficiently rather than asking for ordering preferences.
- When unsure about a technical decision, research it and recommend an approach. Owner will redirect if needed.
- Architecture discussions before code — present options with tradeoffs, let owner choose, then implement.
- Owner has strong architectural instincts. Watch for signals like "would it make more sense to..." — these are directional decisions, not questions. Confirm and execute.
- Owner values honest pushback. Explain why an alternative is better. The owner redirects when they disagree.
- **Owner's workflow is: analyze sprint → clarify decisions → autonomous implementation → output review → structured retrospective.** Don't slow down implementation with mid-task checkpoints. The retrospective IS the quality gate.
- **During retrospective, the owner probes deeply.** Casual-sounding questions ("are these items captured anywhere?", "does nanomdm support simulated events?") are deliberate investigation. They consistently lead to significant findings. Investigate fully.
- **Owner juggles multiple projects.** Keep the "Current State" block at the top of this file updated after each major milestone so anyone can re-orient instantly.
- **When presenting analysis at sprint start, separate decisions from context.** Put "decisions I need from you" as a numbered list at the top. Put analysis and tradeoffs below. The owner makes decisions quickly when clearly framed.

## Lessons for Future Agents

- **Don't accept "acceptable for now" without verifying.** If coverage is low or tests use mocks for database code, investigate *why*. Run the actual tests, check what's uncovered, and look at the uncovered code. In this session, "macos 59% — borderline" turned into 2 real JSONB serialization bugs that would have broken DEP sync in production. The owner will push you on this — get ahead of it.
- **Verify claims from earlier sprints.** Sprint completion docs may say "deferred — not critical for MVP" about things that are now critical. The SCEP server was deferred in Sprint 1 and nobody tracked it since. Check deferred items against current requirements.
- **The owner thinks in infrastructure, not code.** When presenting solutions, frame them in terms of services, data flow, failure modes, and deployment topology — not Go function signatures. The owner catches architectural gaps (missing NanoMDM in diagrams, no Keycloak service, SCEP not wired up) that code-focused analysis misses.
- **Don't default to Kubernetes.** The owner's instinct is toward lighter orchestration (ECS Fargate). If a doc or plan assumes K8s, flag it and propose the simpler alternative.
- **Integration tests find real bugs that mocks hide.** Every time we wrote integration tests against live PostgreSQL, we found bugs — NULL column scans in command repo, JSONB serialization in DEP storage. When the owner asks "why don't we have tests for this?", the answer should be "let me write them now" not "it's tested through mocks."
- **The owner asks probing questions that look simple but aren't.** "Don't we have real PostgreSQL?" and "Is this SCEP work detailed anywhere?" both led to significant findings. Take these questions seriously — investigate fully before answering.
- **Don't trust Sprint 2/3 platform integration claims.** The macOS, Windows, and Android enrollment flows were built against mocks and JSON fixtures. The actual protocol-level integration has never been tested with real protocols. Sprint 5c exists to fix this. Until 5c is complete, treat all platform features as "works in tests, unverified with real protocols."
- **NULL column scan bugs are a recurring pattern.** Every nullable TEXT/VARCHAR column scanned into a Go `string` will fail. Three bugs found so far: `error_message` in commands, `full_name` in users, `user_agent` in audit logs. When writing new repository code, audit all `Scan()` calls against the schema for nullable columns. Use `COALESCE(col, '')` or `sql.NullString`.
- **Connection pool exhaustion causes flaky tests.** 18 test packages × N connections can exceed PostgreSQL's 100 limit. Always use `-p 4` when running the full suite. Test pool sizes should be 2 open / 1 idle. If tests fail randomly with different tests each run, check connection counts before debugging individual tests.
- **`mdmb`** (github.com/jessepeterson/mdmb) is a device simulator by the NanoMDM author. It exercises the full Apple MDM protocol stack (SCEP, check-in, commands). Use it for macOS E2E testing in Sprint 5c.
- **The `strings.Contains(err.Error(), "not found")` pattern is fragile.** ✅ RESOLVED in S5c-07. All 36 handler checks + 29 repo returns now use `apperrors.ErrNotFound` sentinel + `errors.Is()`. 16 integration test assertions still use `assert.Contains` (works, low priority).
- **The retrospective process finds real issues.** Don't rush it. The backward look, forward look, and holistic doc/test review consistently find bugs, stale docs, and architectural gaps. Budget time for it.
- **Don't defer items that are in the sprint plan without a real blocker.** In Sprint 5c, mdmb integration and SCEP verification were deferred twice before the owner caught it. The "blocker" was just a test configuration issue (wrong port). If the sprint plan says to do it, do it — find a way through the difficulty instead of deferring. The owner will call you on it.
- **When you hit a test/tooling issue, check if the tool has unreleased fixes.** mdmb v0.1.0 (2021 release) had build issues. The `main` branch (updated weekly) had fixes. Always check the repo's recent commits, not just the latest release tag.
- **The owner's instinct to move to Docker was correct and should have been proposed by the agent.** When debugging cross-platform issues, the first question should be "are we running on the same platform?" not "what's different about the crypto implementation?" Infrastructure-level solutions beat code-level workarounds.
- **Push features to completion, not just "working."** The owner asked for enriched enrollment data, concurrent testing, and full protocol verification — none of which were in the original sprint plan. These made the feature production-ready instead of just "tests pass." When implementing a feature, ask: "what would an admin expect to see?" not just "does the test pass?"
- **Patch upstream bugs locally rather than working around them.** The mdmb Load() bug was a 3-line fix. Patching it in the Dockerfile was faster and more correct than adjusting our test assertions to accept empty fields.
- **When a bug has a complex explanation, check the simple one first.** The "pkcs7 library incompatibility" theory was believed for two sprints. The actual cause was a wrong file path — `go test` changes the working directory, `NewCAManager` generated a new CA, NanoMDM had the old CA. The clue: `go test -c` + run from project root passed, `go test -run` failed. Same binary, different working directory. Simple explanation, 10-second fix once found.
- **Hardcoded connection strings in test files are a recurring pattern.** Three separate test files had `host=localhost` hardcoded, preventing them from running in Docker. Every new integration test file risks the same bug. Consolidate into a shared helper (tracked in S5f-03).
- **Don't trust bug diagnoses from previous sessions without reproducing.** The Sprint 5e plan stated "pkcs7 library incompatibility" as fact. It was a theory that nobody verified. The actual cause was a wrong file path. When a sprint plan gives you a root cause, treat it as a hypothesis until you've confirmed it yourself.
- **"Is this happening anywhere else?" should be your reflex after every bug fix.** In Sprint 5e, the CA path bug existed in 3 locations, the hardcoded localhost bug in 3 files, and the wrong database name in 2 files. The owner will ask — get ahead of it by searching the full codebase after every fix.
- **`go test` changes the working directory to the package directory.** Any relative path in a test resolves from the package dir, not the project root. Use `projectPath(t, ...)` (defined in `tests/e2e/helpers_test.go`) for project-root files, or `t.TempDir()` for generated files. Never use bare relative paths like `"internal/api/certs/ca.crt"` in tests.
- **Tests that skip on DB connection failure can hide wrong database names.** Two tests referenced `localmdm_test` (doesn't exist in Docker) instead of `localmdm`. They silently skipped via `t.Skipf` on connection error. When un-skipping or fixing integration tests, verify the database name matches the Docker setup.
- **When the owner asks "does that also cover X?" they already suspect it doesn't.** The CA env var support covered Local MDM but not NanoMDM (separate binary, file-based CA loading). The owner caught this immediately. Always think about the full system — all services, not just the Go code.
- **Independent sprint tasks can run in parallel via subagents.** Mechanical tasks (assert.ErrorIs migration, SCEP test coverage) don't depend on investigation tasks. Use subagents for parallel implementation when tasks touch different files with no shared state.
- **Don't run the retro autonomously.** "Ready for retro?" is a status check, not a go signal. The owner drives each section with explicit prompts. Wait for them.
- **When the owner prescribes a technical approach, validate it against the codebase before agreeing.** The owner thinks in infrastructure ("we have real Keycloak, use it") but may not know which test patterns can access it. If a suggestion doesn't fit the code structure, explain why and propose the alternative — don't just silently do something different.
- **Don't overestimate effort on test coverage.** "Half a day" and "diminishing returns" turned into 5 minutes and +7% coverage. Before claiming something is expensive, actually look at the uncovered lines and count them. The owner will ask you to do it anyway, and you'll look bad when it's trivial.
- **The owner wants honest, critical feedback — not softened positives reframed as criticism.** When asked "anything I could do differently?", give actual negatives. The owner will push back if you're being too nice. They act on real feedback.
- **Establish a test baseline before starting work.** Run `make dev-test` before the first change to confirm the branch is green. The steering guide says to do this. If you skip it and something was already broken, you'll waste time debugging pre-existing failures mixed with your changes.
- **Don't shim around test failures — fix the root cause.** The EventBus `if cfg.Database.Host != ""` guard was a shim to avoid a test hang. The real fix was a pre-flight connection check. When a test fails, write an isolated test that reproduces the failure, fix the underlying issue, then verify. The owner will catch shims.
- **pq.Listener spawns an uncontrollable reconnect goroutine.** Never create a `pq.Listener` without first verifying the DSN works via `sql.Open` + `Ping`. Add `connect_timeout=5` to the DSN. Make `Shutdown()` safe when `Start()` was never called.
- **Sprint plan effort estimates are consistently 3-5x too high.** Sprint 5b was estimated at 3-5 days, took <1 day. Before accepting estimates, look at the actual code and count the changes needed. The owner's roadmap depends on accurate estimates.
- **The owner's casual questions surface major gaps.** "Do we capture BitLocker keys?" → recovery key escrow (not in roadmap). "Does this cover iOS?" → entire platform missing. "Any other MDM features?" → 12 new roadmap items. Take these questions as seriously as explicit requirements.
- **The owner reviews your work by asking questions, not reading code.** They won't spot a bad implementation by reading Go — they'll spot it by asking "does this work when the database is down?" or "what happens in production with multiple instances?" Frame your implementation summaries in terms of failure modes and operational behavior, not code structure. If you shimmed something, they'll find it through questions, not code review.
- **The owner has strong opinions about test realism.** Tests that use mock DBs when a real Docker PostgreSQL is available will get called out. Tests that skip on connection failure when the connection should work will get called out. The expectation is: if Docker is running, tests use real infrastructure. The `testutil.ConnectDB(t)` pattern exists for this reason. Don't create new test patterns that bypass it.
- **UI work requires a fundamentally different approach than backend work.** The backend sprint plans had clear interfaces and testable outputs. UI work needs user workflows ("as an admin, I want to..."), not feature lists ("device list with pagination"). Build one page, show it, get feedback, then build the next. Don't mark 7 tasks complete at once without showing output.
- **Run Playwright after every change, not at the end.** In Sprint 5d, the first Playwright run found 5 bugs that would have been caught incrementally. The Playwright suite IS the verification step for UI work — treat it like `go test -race` for backend work.
- **The owner tests like a user, not like a developer.** They click every button, try every workflow, resize the window, check the audit log. They will find gaps you missed because you verified "does the page render" not "can I accomplish my goal." Before calling a page done, click through every action a user would take.
- **Batch UI feedback requests.** The first 4 hours of Sprint 5d were one-issue-at-a-time ping-pong. When the owner gave a batched list of 30+ items, productivity tripled. If you're building UI, ask the owner to collect all feedback in one pass rather than reporting issues one at a time.
- **CSP nonces break in non-obvious ways.** Go's `html/template` HTML-escapes `+` to `&#43;` in attributes, which breaks CSP nonce matching. Use `base64.RawURLEncoding` (no `+` or `/` characters) for nonces. Inline event handlers (`onclick`, `oninput`) are blocked by CSP even with nonces — only `<script>` tags get nonce support. Move all JS to nonce'd script blocks with `addEventListener`.
- **Go map iteration order is random.** Any data from a Go map displayed in the UI will shuffle on every page load. Always sort before rendering.
- **Docker containers don't auto-run migrations unless you add an entrypoint script.** The Dockerfile had `migrate` binary but never executed it. Migration 000011 wasn't applied, causing EventBus compliance auto-evaluation to silently fail. Always verify migrations are current.
- **The owner's Keycloak setup uses `/etc/hosts` for hostname resolution.** Keycloak runs on port 8180 (same internal and external). Don't try to rewrite URLs between Docker-internal and browser-facing — align the ports instead.
- **SVG text scales with the viewBox.** For consistent font sizing in charts, use HTML text outside the SVG and keep the SVG for shapes only.
- **The owner expects compliance to show individual setting checks, not policy-level pass/fail.** "Require Encryption: Fail" is actionable. "Corporate Security Baseline: Non-Compliant" is not.
- **The owner thinks across projects.** They'll reference patterns from other codebases (dev-deployer Playwright runner, infrastructure patterns from work). When they mention another project, look at it — they're telling you to use that approach, not just mentioning it casually. Ask for the path upfront if they don't provide it.
- **The owner values completeness over speed.** They'd rather you take an extra 10 minutes to wire the `policy.unassigned` subscriber properly than ship a no-op with a comment saying "will evaluate on next check-in." The retro will catch incomplete work and you'll end up doing it anyway. Do it right the first time.
- **When the owner says "lets fix all these items properly" they mean now, in this session.** Don't propose deferring to a future sprint. Don't ask "want me to fix this now or track it?" Just fix it. The only valid deferral is a genuine technical blocker (needs real hardware, needs a third-party API key).
- **The owner's retro questions escalate.** "Is there anything else?" means "I suspect there's more — find it." They'll ask this multiple times until you say no. Each round should involve actual investigation (grep the codebase, check docs, think about edge cases), not just "I think we're good." The owner asked "is there anything else?" three times in Sprint 5b and each time there was more to find.

- **Don't run commands that can hang.** The shell tool cannot handle background processes (`&`). If you need a server running while tests execute, write a self-contained script and tell the owner to run it. Don't try to background a process interactively — you'll burn 30+ minutes on workarounds that don't work.
- **When the owner says "get creative" they mean it.** Changing accent colors is not a theme. Changing backgrounds, card styles, sidebar colors, border radius, shadows, and overall feel IS a theme. The owner has high visual standards. If a UI task feels too easy, you're probably not going deep enough.
- **The owner's retro question "list every shortcut" is a real engineering gate.** Don't soften it. List actual shortcuts, stubs, and deviations. The owner will turn each one into an action item with a time estimate. This is how quality gets enforced — take it seriously and be thorough.
- **Playwright tests passing doesn't mean the UI works.** The tests check text visibility and click success. They don't check layout integrity, CSS rendering, or UX flow. The owner tests like a user — clicking through every page, switching themes, filtering then toggling. Add tests that verify structural integrity (sidebar visible after nav, header present, filter preserved after action) not just "does the text appear."
- **Don't claim something is blocked without checking.** "Needs real Keycloak" was wrong — Keycloak was running in Docker the whole time. "Needs custom runner support" was wrong — the runner already supported select dropdowns. Before saying something is blocked, actually look at the infrastructure and tooling available.
- **CSP nonces and HTMX body swaps are incompatible.** Inline `<script nonce="...">` blocks in swapped content won't execute because the CSP header nonce was set on the original page load. The fix: move ALL page JS to an external `app.js` using event delegation. This is a hard architectural constraint — don't try hx-boost or body swaps until all inline scripts are eliminated.
- **The seed data `ON CONFLICT DO NOTHING` pattern silently loses data.** Soft-deleted rows (deleted_at set) still exist, so the INSERT doesn't fire. The seed must explicitly reset `deleted_at` for all seed rows AND use `DO UPDATE` for the enterprise. Every new seed table needs this treatment.
- **Go map iteration order is random — sort before rendering.** This was a known lesson but it applies to the compliance engine too. `map[string]string` violations render in random order. Always `sort.Strings(configKeys)` before iterating.

## Known Issues

- **Android SecurityPosture parsing deferred to F-01** — handleStatusReport persists webhook Data to platform_data but doesn't parse Google SecurityPosture (requires Google API client). TODO comment in code.
- **CA manager failure is now fatal** (Sprint 5b) — server won't start if CA cert/key can't be loaded. All test configs must include valid Certificates config.
- **pq.Listener spawns uncontrollable reconnect goroutine** — EventBus.Start() does a pre-flight sql.Open+Ping before creating the listener to avoid hangs. If you see EventBus connection issues, check the DSN has `connect_timeout`.
- **F-07 expanded significantly in Sprint 5b** — 12 new features added (iOS, kiosk, lost mode, selective wipe, OS updates, inventory, zero-touch, alerting, self-service, app store, conditional access sync, SCIM). Review before planning next sprints.
- **Recovery key escrow tracked in F-03** — gap analysis with 7 specific items (migration, repo, profile payloads, response parsing, API endpoint). Depends on F-01.

## Project-Specific Knowledge

- **Redis is gone.** Fully removed in Sprint 4. Token cache and idempotency keys use PostgreSQL. No Redis in docker-compose, config, or go.mod. Don't add it back.
- **Secrets**: `secrets/` directory for dev (gitignored), AWS SSM for production. DEP tokens encrypted with pgcrypto in the database.
- **Idempotency-Key**: PostgreSQL-backed middleware on all POST/PUT/PATCH. 24h TTL, hourly cleanup.
- **Prometheus metrics**: separate internal port (127.0.0.1:9090), not the public API port.
- **EventBus**: PostgreSQL triggers in place (migration 000007 + 000011). Go-side pq.Listener built in Sprint 5b. Subscribers: compliance auto-evaluation on device/policy/group events, lifecycle cleanup on unenroll/wipe/delete. Must use Writer pool DSN (read replicas don't relay NOTIFY).
- **Compliance engine**: real evaluation logic in Sprint 5 (security: password, encryption, firewall; restrictions: camera). Returns "unknown" when device has no `platform_data`. Auto-evaluation on check-in deferred to Sprint 5b (EventBus).
- **Policy deployment**: assignments recorded, devices pick up on next check-in. No immediate push (intentional).
- **Sprint 2 security review docs** contain false claims. Trust the code, not the review narratives.
- **Dashboard**: Go templates + HTMX + Tailwind CSS (Sprint 5d). Not React.
- **macOS Platform SSO**: Sprint 7 (Java + Swift, separate from Go work).

## Sprint Status

| Sprint | Status | Key Deliverables |
|--------|--------|-----------------|
| 1 | ✅ Complete | Foundation, schema, auth, certs, enrollment stubs |
| 2 | ✅ Complete | API handlers, platform enrollment, DEP, metrics |
| 2a | ✅ Complete | Gap closure, CRUD, webhooks, DEP sync |
| 3 | ✅ Complete | Commands, profiles, apps, CSPs, PPKG |
| 4 | ✅ Complete | Policy system, compliance, groups, lifecycle, Redis removal |
| 4b | ✅ Complete | Writer/Reader DB pools, repo constructor refactor |
| 4c | 🔲 Not Started | macOS Platform SSO (Java/Swift) — renamed to Sprint 7 |
| 5 | ✅ Complete | Backend polish, CLI, observability, performance |
| 5c | ✅ Complete | Platform integration fixes (macOS/Windows/Android), SCEP, service tests |
| 5e | ✅ Complete | NanoMDM cert verification fix (path bug), assert.ErrorIs migration, SCEP tests, coverage improvements |
| 5f | ✅ Complete | API hardening, explicit CA generation, test DB helper consolidation |
| 5b | ✅ Complete | EventBus listener, compliance wiring, lifecycle hooks, k6 load tests |
| 5d | ✅ Complete | Web dashboard (HTMX), 196 Playwright tests |
| 5g | ✅ Complete | Quality polish: N+1 fixes, loading indicator, empty states, error tests, interface refactor |
| 6 | 🔲 Not Started | Real device integration (Windows VM, macOS VM, Android) |
| 7 | 🔲 Not Started | macOS Platform SSO (Java/Swift) — requires Apple Developer account |
