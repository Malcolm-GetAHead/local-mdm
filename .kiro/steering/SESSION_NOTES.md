# Session Notes — Working Preferences & Project Knowledge

**Last Updated**: 2026-04-23  
**Purpose**: Guidance for AI agents working on this codebase. Keep this file lean — patterns and conventions that apply to every session. One-shot implementation details belong in sprint docs, not here.

## Current State

- **Sprint 5e**: ✅ COMPLETE, on branch `sprint-5e/cert-verification` (not yet merged to main)
- **Sprint 5c**: ✅ COMPLETE, merged to main
- **Retrospective**: In progress
- **Next sprint**: **5f** (API hardening + test hygiene), then 5b (EventBus), then 5d (dashboard)

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

## Known Issues

- **EventBus Go listener not built** — triggers exist (migration 000007), no listener. Tracked in Sprint 5b.
- **Production CheckinHandler only extracts UDID** — The enriched handler (Serial, Name, Model, OS, Build, status=enrolled on TokenUpdate) exists only in `tests/e2e/mdmb_enrollment_test.go`. Must be ported to `internal/platform/macos/webhook.go` CheckinHandler. Tracked in Sprint 5b S5b-04.
- **NewCAManager silently generates CAs** when file path is wrong — footgun that caused the Sprint 5e cert verification bug. Tracked in Sprint 5f (S5f-01) — make generation explicit via CLI command.

## Project-Specific Knowledge

- **Redis is gone.** Fully removed in Sprint 4. Token cache and idempotency keys use PostgreSQL. No Redis in docker-compose, config, or go.mod. Don't add it back.
- **Secrets**: `secrets/` directory for dev (gitignored), AWS SSM for production. DEP tokens encrypted with pgcrypto in the database.
- **Idempotency-Key**: PostgreSQL-backed middleware on all POST/PUT/PATCH. 24h TTL, hourly cleanup.
- **Prometheus metrics**: separate internal port (127.0.0.1:9090), not the public API port.
- **EventBus**: PostgreSQL triggers in place (migration 000007). Go-side LISTEN/NOTIFY listener not yet built — **Sprint 5b** owns this. Must use dedicated connection on Writer pool DSN (read replicas don't relay NOTIFY).
- **Compliance engine**: real evaluation logic in Sprint 5 (security: password, encryption, firewall; restrictions: camera). Returns "unknown" when device has no `platform_data`. Auto-evaluation on check-in deferred to Sprint 5b (EventBus).
- **Policy deployment**: assignments recorded, devices pick up on next check-in. No immediate push (intentional).
- **Sprint 2 security review docs** contain false claims. Trust the code, not the review narratives.
- **Dashboard**: Go templates + HTMX + Tailwind CSS (Sprint 5d). Not React.
- **macOS Platform SSO**: Sprint 6 (Java + Swift, separate from Go work).

## Sprint Status

| Sprint | Status | Key Deliverables |
|--------|--------|-----------------|
| 1 | ✅ Complete | Foundation, schema, auth, certs, enrollment stubs |
| 2 | ✅ Complete | API handlers, platform enrollment, DEP, metrics |
| 2a | ✅ Complete | Gap closure, CRUD, webhooks, DEP sync |
| 3 | ✅ Complete | Commands, profiles, apps, CSPs, PPKG |
| 4 | ✅ Complete | Policy system, compliance, groups, lifecycle, Redis removal |
| 4b | ✅ Complete | Writer/Reader DB pools, repo constructor refactor |
| 4c | 🔲 Not Started | macOS Platform SSO (Java/Swift) — renamed to Sprint 6 |
| 5 | ✅ Complete | Backend polish, CLI, observability, performance |
| 5c | ✅ Complete | Platform integration fixes (macOS/Windows/Android), SCEP, service tests |
| 5e | ✅ Complete | NanoMDM cert verification fix (path bug), assert.ErrorIs migration, SCEP tests, coverage improvements |
| 5f | 🔲 Not Started | API hardening, explicit CA generation, test DB helper consolidation |
| 5b | 🔲 Not Started | EventBus listener, compliance wiring, load testing |
| 5d | 🔲 Not Started | Web dashboard (HTMX) |
| 6 | 🔲 Not Started | macOS Platform SSO (Java/Swift) — requires Apple Developer account |
