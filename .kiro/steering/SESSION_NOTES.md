# Session Notes — Working Preferences & Project Knowledge

**Last Updated**: 2026-04-30  
**Purpose**: Guidance for AI agents working on this codebase. Keep this file lean — patterns and conventions that apply to every session. One-shot implementation details belong in sprint docs, not here.

## Current State

- **Sprint 6**: ✅ COMPLETE — all cleanup items resolved, Windows OMA-DM fully operational with real device
- **Sprint 7**: 🔲 Not Started — macOS Platform SSO (Java/Swift, requires Apple Developer account)

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
- **During retrospective, the owner probes deeply.** Casual-sounding questions are deliberate investigation. They consistently lead to significant findings. Investigate fully.
- **When presenting analysis at sprint start, separate decisions from context.** Put "decisions I need from you" as a numbered list at the top. Put analysis and tradeoffs below. The owner makes decisions quickly when clearly framed.
- **Don't run the retro autonomously.** "Ready for retro?" is a status check, not a go signal. The owner drives each section with explicit prompts. Wait for them.

## Lessons for Future Agents

### Investigation & Debugging

- **`go test` passing does not mean the feature works.** Rebuild the Docker container (`docker compose build localmdm && docker compose up -d localmdm`) and hit the real endpoint with curl or a browser after every sub-task. Templates are embedded via `//go:embed` — any template or static file change requires a rebuild to take effect.
- **Don't accept "acceptable for now" without verifying.** If coverage is low or tests use mocks for database code, investigate *why*. Run the actual tests, check what's uncovered, and look at the uncovered code.
- **Verify claims from earlier sprints.** Sprint completion docs may say "deferred — not critical for MVP" about things that are now critical. Check deferred items against current requirements.
- **Don't trust bug diagnoses from previous sessions without reproducing.** When a sprint plan gives you a root cause, treat it as a hypothesis until you've confirmed it yourself.
- **"Is this happening anywhere else?" should be your reflex after every bug fix.** Search the full codebase after every fix.
- **When you hit a test/tooling issue, check if the tool has unreleased fixes.** Always check the repo's recent commits, not just the latest release tag.
- **When a bug has a complex explanation, check the simple one first.** The "pkcs7 library incompatibility" theory was believed for two sprints. The actual cause was a wrong file path.
- **When debugging protocol issues, compare with a known working implementation byte-by-byte.** Fleet DM's source code revealed the namespace bug, the correct EnrollmentVersion, and the proper XML marshaling approach.
- **Don't claim something is blocked without checking.** Before saying something is blocked, actually look at the infrastructure and tooling available.
- **Verify CSP/protocol values on real devices, not just docs.** MS doc labels for BitLocker DeviceEncryptionStatus bitmask didn't match observed behavior. Three commits were wasted guessing from docs before SSH'ing into the VM and testing each state. Always test on the device first.
- **Check if the running container has your code.** After pushing code, the Docker container still runs the old image until rebuilt. If something "doesn't work" after a code change, check `docker ps` creation time before investigating.
- **When a mock-based test passes but the feature doesn't work, write an integration test.** The device ID resolution passed all mock tests but failed in production because `deviceRepository.Update()` didn't include `device_id` in the SQL UPDATE. The integration test caught it instantly.

### Code Quality

- **Integration tests find real bugs that mocks hide.** Every time we wrote integration tests against live PostgreSQL, we found bugs. When the owner asks "why don't we have tests for this?", the answer should be "let me write them now."
- **Inline `onclick` and inline JS in templates are blocked by CSP.** The dashboard uses a strict Content Security Policy with nonces. All JavaScript must go in `web/static/js/app.js` using event delegation. Never use `onclick`, `hx-on::`, or inline `<script>` in templates.
- **`created_by` and other FK columns referencing `users` table: Keycloak user IDs don't always exist in the local `users` table.** Check with `userService.Get()` before setting a FK. Silent INSERT failures are hard to debug.
- **Don't duplicate structs across handlers.** If you need the same view struct in multiple handlers, extract it to a shared type and helper function immediately. The owner will call out copy-paste.
- **Documentation updates (DATABASE.md, API.md, openapi.yaml) are part of the implementation, not a cleanup step.** Update docs as you write the code, not after.
- **Scoped test cleanup only.** Never `DELETE FROM <table>` without a WHERE clause. Always scope to test-created rows (e.g., `WHERE description LIKE 'inttest-%'`). Blanket deletes destroy real device and token data.
- **NULL column scan bugs are a recurring pattern.** Every nullable TEXT/VARCHAR column scanned into a Go `string` will fail. Use `COALESCE(col, '')` or `sql.NullString`.
- **Don't shim around test failures — fix the root cause.** When a test fails, write an isolated test that reproduces the failure, fix the underlying issue, then verify.
- **Don't overestimate effort on test coverage.** Before claiming something is expensive, actually look at the uncovered lines and count them.
- **Establish a test baseline before starting work.** Run `make dev-test` before the first change to confirm the branch is green.
- **`make dev-test` destroys real device data.** ~~Tests share the production database and do broad deletes.~~ Test cleanup is now scoped to test enterprise IDs (CASCADE), but tests and real data still share the same database. Run tests BEFORE enrolling real devices, or use a separate test database.
- **Every test that creates an enterprise must clean it up.** Use `testutil.CreateTestEnterprise(t, db, name)` for raw SQL, or `t.Cleanup(func() { db.Writer.Exec("DELETE FROM enterprises WHERE id = $1", id) })` after `entRepo.Create()`. Never use `entRepo.Delete()` for cleanup — that's a soft delete.
- **`defer db.Close()` breaks `t.Cleanup()`.** `defer` runs before `t.Cleanup()`, closing the connection before cleanup can execute. `testutil.ConnectDB(t)` already registers close via `t.Cleanup()` — don't add `defer db.Close()` on top of it.
- **A trailing slash on an XML namespace is a different namespace.** Always compare namespaces character-by-character against the spec or a known working implementation.
- **MS-MDE2 requires non-chunked HTTP responses.** Always set Content-Length on SOAP responses.
- **Go map iteration order is random.** Always sort before rendering.
- **`go test` changes the working directory to the package directory.** Use `projectPath(t, ...)` for project-root files.

### Owner Interaction

- **The owner thinks in infrastructure, not code.** Frame solutions in terms of services, data flow, failure modes, and deployment topology — not Go function signatures.
- **The owner doesn't read Go.** This is an experiment in LLM-driven development. The owner provides architecture, troubleshooting, and domain knowledge — not code review. Write clear "why" comments in code, not just "what" comments. Frame implementation summaries in terms of behavior and failure modes.
- **The owner asks probing questions that look simple but aren't.** Take these questions seriously — investigate fully before answering. Questions like "how does this work for macOS?" or "can someone share that file?" consistently uncover real design gaps.
- **The owner's retro questions escalate.** "Is there anything else?" means "I suspect there's more — find it." Each round should involve actual investigation.
- **The owner wants honest, critical feedback — not softened positives reframed as criticism.** Give actual negatives. They act on real feedback.
- **The owner has strong opinions about test realism.** Tests that use mock DBs when a real Docker PostgreSQL is available will get called out.
- **When the owner says "lets fix all these items properly" they mean now, in this session.** Don't propose deferring to a future sprint.
- **When the owner prescribes a technical approach, validate it against the codebase before agreeing.** If a suggestion doesn't fit the code structure, explain why and propose the alternative.
- **The owner context-switches during sessions.** Architecture brainstorming may happen mid-implementation. Acknowledge the idea, finish the current task, then address it. Don't interrupt code work to write docs.
- **The owner has domain knowledge they don't always share upfront.** If they mention a specific tool or project (Fleet DM, dev-deployer), they're telling you to look at it. Ask for details immediately rather than discovering them later.

### Process

- **Think critically about the spec, don't just implement it literally.** If a spec says a feature is "optional" but the feature's entire purpose is access control, flag the contradiction. Ask "what would an admin actually expect?" — not just "what does the task list say?"
- **Push features to completion, not just "working."** Before calling a feature done, ask: "what would an admin expect to see?" not just "does the test pass?"
- **Scope expansion without test checkpoints creates debt.** When features keep getting added, pause to test what exists before adding more.
- **Sprint effort estimates for agent work are 10-25x too high.** Mechanical, well-scoped changes compress dramatically. Investigation and debugging don't compress as much.
- **Don't defer items that are in the sprint plan without a real blocker.** Find a way through the difficulty instead of deferring.
- **Independent sprint tasks can run in parallel via subagents.** Use subagents for parallel implementation when tasks touch different files with no shared state.
- **Don't run commands that can hang.** The shell tool cannot handle background processes. If you need a server running while tests execute, write a self-contained script.
- **Batch UI feedback requests.** Ask the owner to collect all feedback in one pass rather than reporting issues one at a time.
- **Restore VMs from template at the start of device testing.** Stale enrollment state from previous sessions causes confusing failures.

## Known Issues

- **Enrollment requires tokens (AUT-01)** — both Windows and macOS enrollment require a valid enrollment token. Create tokens via dashboard or API. macOS uses_remaining decrements at SCEP certificate issuance, not profile download. VM re-enrollment after snapshot restore requires a fresh token.
- **CRL is static** — served from a file generated once. Cert revocations won't be reflected until CRL regeneration is implemented.
- **NanoMDM config uses host IP** — `configs/config.docker.yaml` has `nanomdm_url` with env var override (`NANOMDM_URL`), but enrollment profiles use the host IP which must be reachable from VMs.
- **macOS webhook enterprise ID is hardcoded** — Authenticate handler uses configurable `default_enterprise_id`. Multi-tenant requires passing enterprise ID through the enrollment flow.
- **Container rebuilds regenerate the CA unless certs are volume-mounted.** Without the `./internal/api/certs:/app/certs` mount, every `docker compose build` creates a new CA, breaking all enrolled devices.

## Project-Specific Knowledge

- **Redis is gone.** Fully removed in Sprint 4. Don't add it back.
- **Secrets**: `secrets/` directory for dev (gitignored), AWS SSM for production.
- **EventBus**: PostgreSQL LISTEN/NOTIFY. Subscribers: compliance auto-evaluation, lifecycle cleanup.
- **Dashboard**: Go templates + HTMX + Tailwind CSS. Not React.
- **macOS Platform SSO**: Sprint 7 (Java + Swift, separate from Go work).
- **Default to ECS Fargate, not Kubernetes.**
- **Autonomous sessions**: `docs/planning/future/autonomous_sessions.md` contains self-contained session prompts (AUT-00 through AUT-08) with mandatory verification gates. Read the Prerequisites section before executing any session.
- **Microsoft MDM reference**: `MicrosoftDocs/memdocs` repo is indexed as a knowledge base. Search it for Windows CSP definitions (BitLocker, Firewall, DeviceLock, WiFi, VPN, app management, certificate store, Windows Update), Intune compliance policy logic, enrollment flows, and OMA-DM protocol details. The BitLocker CSP bitmask was verified against real device testing — MS doc labels don't always match observed behavior, so verify with actual devices when possible.
- **Apple MDM reference**: `apple/device-management` repo indexed as KB. Contains MDM command schemas, profile payload definitions, check-in protocol, declarative management specs. Machine-readable YAML — the definitive source for command fields and supported OS versions.
- **Android MDM reference**: `googleapis/google-api-nodejs-client` (androidmanagement directory) indexed as KB. Contains full API type definitions with JSDoc descriptions for policies, devices, compliance, enrollment tokens, app management. The `v1.ts` file is the complete API reference in code form.

## Sprint Status

| Sprint | Status | Key Deliverables |
|--------|--------|-----------------|
| 1-4 | ✅ Complete | Foundation through policy/compliance/groups/lifecycle |
| 4b | ✅ Complete | Writer/Reader DB pools |
| 5-5g | ✅ Complete | Backend polish, platform integration, web dashboard, quality |
| 6 | ✅ Complete | macOS full data pipeline, Windows enrolled via Settings UI, OMA-DM sync, nginx TLS with CRL |
| 7 | 🔲 Not Started | macOS Platform SSO (Java/Swift) |
