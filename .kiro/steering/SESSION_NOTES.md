# Session Notes — Working Preferences & Project Knowledge

**Last Updated**: 2026-04-22  
**Purpose**: Guidance for AI agents working on this codebase. Keep this file lean — patterns and conventions that apply to every session. One-shot implementation details belong in sprint docs, not here.

## Current State

- **Sprint 5c**: ✅ COMPLETE, on branch `sprint-5c/platform-integration` (not yet merged to main)
- **Sprint 5**: ✅ COMPLETE, merged to main
- **Retrospective**: In progress (backward look)
- **Next sprint**: **5b** (EventBus), then 5d (dashboard)
- **Pending**: merge sprint-5c/platform-integration to main

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

## Session Closeout Process

After every sprint, the owner expects:

1. **Retrospective (backward look)**: scope audit, dead code scan, stale docs, test coverage. Check ALL sprint OVERVIEWs (not just current). First action: update current sprint's OVERVIEW (mark complete, check DoD boxes).
2. **Forward look**: next 2 sprints + future roadmap alignment. Flag broken dependencies, stale assumptions, gaps.
3. **Doc & test audit**: Owner will prompt with: *"How's our test coverage and documentation? Are they still accurate? Are we skipping any integration tests we shouldn't? Please review documentation holistically for the project to ensure we haven't missed anything outside of the sprint."* — Do a thorough audit. Check API.md matches routes, DATABASE.md matches migrations, ARCHITECTURE.md matches packages. Don't just say "looks good."
4. **Clean up**: delete merged branches, remove dead code, push to origin.
5. **Owner asks for feedback**: At session end, the owner asks what they could do differently. Be honest and specific — they act on it. Also offer to update session notes before context is cleared.

## Owner Background

- **DevOps engineer** — Windows/Linux sysadmin → DevOps. Understands infrastructure, deployment, MDM concepts from the admin side.
- **Languages**: Python and PostgreSQL at intermediate level. No Go experience — don't assume the owner reads Go code directly.
- **Strengths**: Architecture patterns, infrastructure decisions, tradeoff analysis, knowing when something smells wrong.
- **When explaining**: Use plain language for Go-specific patterns. Relate to concepts the owner already knows. Don't over-simplify, just be clear.

## Git Workflow

- **Feature branch per sprint**: `sprint-{id}/{short-description}`. Never commit directly to main.
- **Commit per sub-task**, referencing task ID (e.g. `S4b-01: ...`). Run tests with `-race` before each commit.
- **Push after each commit** — work must survive session loss. Per-task commits are recovery checkpoints.
- Don't squash or amend unless asked. Owner wants to see progression.

## Implementation Patterns (Current as of Sprint 4b)

### Repository Pattern (Writer/Reader Pools)
- **Constructors take two args**: `NewXxxRepository(writer, reader interface{})`. Both resolve via `resolveExecutor()`.
- **Write methods** (Create/Update/Delete): `getExecutor(ctx, r.writer)` — transaction-aware via context.
- **Read methods** (Get/List): `getReadExecutor(ctx, r.reader)` — returns tx if active (reads see uncommitted writes in tx), otherwise uses reader pool.
- **Non-repo consumers** (audit, idempotency, certs, metrics, auth, DEP): use Writer pool directly (`*sql.DB`).
- **Transactor**: uses Writer pool exclusively. `NewTransactor(database.Writer)`.
- In dev, both pools point to the same PostgreSQL. In production, Reader points to Aurora read replica.

### Service Layer (Sprint 4+)
- `internal/service/` — business logic between handlers and repos. Transport-agnostic (no `net/http`).
- Services: `PolicyService`, `GroupService`, `ComplianceService`, `LifecycleService`, `DeviceService`, `AppService`, `UserService`, `TokenService`.
- **Sprint 5 completed the migration**: all device and app handlers now call services. No handlers call repos directly for business logic.
- Services accept repository interfaces via constructor (dependency injection).

### Error Handling
- Not-found detection: `errors.Is(err, apperrors.ErrNotFound)` — repos wrap with `fmt.Errorf("xxx not found: %w", apperrors.ErrNotFound)`, handlers check with `errors.Is()`. Replaced `strings.Contains` pattern in S5c-07.
- Audit logging: `s.logAudit(r, action, resourceType, resourceID, details)` on all mutations.

### Testing
- **Handler tests**: mock repos in `handler_test_helpers_test.go`. New endpoints must be registered in `newTestServer()`.
- **Service tests**: own mocks in `*_test.go` within `internal/service/`.
- **Integration tests**: need Docker services (`docker compose up -d` then `migrate up`). PostgreSQL is always available — don't use `-short` flag to skip integration tests.
- **Always run `go test -race -p 4 ./...`** (no `-short`) — the Docker stack is running. The `-p 4` limits parallel packages to prevent PostgreSQL connection pool exhaustion.

### Config (Database)
```yaml
database:
  host: "localhost"       # Base config = writer pool
  port: 5432
  # Optional reader overrides (for read replicas)
  # Unset fields inherit from base config
  # reader:
  #   host: "replica.example.com"
```
- `ReaderDSN()` falls back to `DSN()` when no reader config.
- Env overrides: `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `DB_READER_HOST`, `DB_READER_PORT`.

## Production Architecture (decided Sprint 4b session)

### Deployment: AWS ECS Fargate (not Kubernetes)
- **ECS Fargate** is the primary deployment target. Kubernetes manifests exist as an appendix in F-02 but are not the default.
- Single Go binary per service — no sidecar proxies or service mesh needed.
- ALB for TLS termination (ACM certificates, auto-renewing) and path-based routing.

### Services (3 ECS services + RDS)
- **localmdm** — the Go application (API, policy engine, enrollment). Multiple tasks behind ALB.
- **nanomdm** — Apple MDM protocol handler (separate ECS service). Receives `/checkin` and `/mdm` via ALB path routing. Uses same RDS instance, separate `nanomdm` database. Local MDM sends commands to NanoMDM via HTTP API (`nanomdm_url` config).
- **keycloak** — OIDC identity provider (separate ECS service). Admin login, JWT issuance, RBAC. Uses same RDS instance, separate database.
- **RDS PostgreSQL** — primary (Writer pool) + read replica (Reader pool). NanoMDM and Keycloak use primary only.

### Supporting AWS Services
- **SSM Parameter Store** — all secrets (DB password, JWT secret, Keycloak secret, DEP encryption key, NanoMDM API key). Injected as env vars at task launch.
- **CloudWatch Logs** — ECS `awslogs` driver for container stdout/stderr.
- **CloudWatch Metrics** — CloudWatch Agent sidecar in each localmdm task scrapes Prometheus metrics from localhost:9090 and forwards to CloudWatch. No separate Prometheus server needed.
- **AWS WAF** — rate-based rules on ALB for production rate limiting. In-memory rate limiters in the Go app remain as defense-in-depth fallback.
- **ACM** — TLS certificates (free, auto-renewing, attached to ALB).

### Multi-Instance Safety
- **Stateless by design** — all shared state is in PostgreSQL (token cache, idempotency keys, SCEP challenges after S5-12).
- **In-memory rate limiters** are per-instance (imprecise across instances, acceptable as fallback behind WAF).
- **JWKS cache** and **circuit breaker** are per-instance by design (correct behavior — each instance independently caches Keycloak public keys).
- **No sticky sessions required** — any instance can handle any request.

### What NOT to do
- Don't add Redis or any external cache — PostgreSQL handles all caching.
- Don't default to Kubernetes — ECS Fargate is the target unless explicitly changed.
- Don't assume single-instance — all code must work behind a load balancer with multiple instances.

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

## What NOT to Do

- Don't add Redis — PostgreSQL handles all caching.
- Don't change secrets storage approach (file-based for dev is intentional).
- Don't build things "just in case" — minimal implementation only.
- Don't trust Sprint 2 review docs at face value.
- Don't use `-short` flag when running tests — integration tests should run.
- Don't use the old single-arg repo constructor pattern `NewXxxRepository(db)` — always use `(writer, reader)`.

## Known Issues

- **Doc debt tracked in `docs/reviews/sprint-4b/DOCUMENTATION_REVIEW.md`** — being resolved on `review/documentation-fixes` branch.
- **Repo integration test gap tracked in `docs/reviews/sprint-4b/REPOSITORY_TEST_COVERAGE.md`** — resolved on `review/repo-integration-tests` branch. All 5 test files written.
- **`GET /windows/ppkg/templates`** — auth being added on `review/documentation-fixes` branch.
- **command.GetByID and ListByDevice bug** — ✅ FIXED in Sprint 5 (COALESCE). Tests updated in `sprint12_gaps_integration_test.go`.
- **Service layer test coverage at 30.4%** — ✅ FIXED in S5c-05 (now 61.9%).
- **`strings.Contains` error detection** — ✅ FIXED in S5c-07. `apperrors.ErrNotFound` sentinel + `errors.Is()` in all handlers.
- **NanoMDM not deployed** — ✅ FIXED in S5c-01. NanoMDM v0.9.0 in docker-compose, separate `nanomdm` database, webhook forwarding.
- **Windows enrollment doesn't create device records** — ✅ FIXED in S5c-02. Enterprise ID in URL path.
- **Android webhook is a no-op** — ✅ FIXED in S5c-03. WebhookHandler wired, graceful degradation without Google client.
- **EventBus Go listener not built** — triggers exist (migration 000007), no listener. Tracked in Sprint 5b.
- **mdmb device simulator not yet integrated** — NanoMDM is deployed but mdmb not installed in dev toolchain. Tracked in F-01.
- **Windows SOAP E2E test not written** — device record creation tested, full SOAP flow deferred to F-01.
- **16 repo integration tests still use `assert.Contains(err.Error(), "not found")`** — works but should use `assert.ErrorIs()` for consistency. Low priority.

## Sprint 4b Learnings

- **Writer/Reader pool pattern**: `resolveExecutor()` helper resolves `interface{}` to concrete `executor` at construction time. `getExecutor(ctx, r.writer)` for writes (transaction-aware), `getReadExecutor(ctx, r.reader)` for reads (uses tx if active, otherwise reader pool).
- **ReaderConfig fallback**: `ReaderDSN()` returns writer DSN when no reader config is set. Zero config change needed for dev — both pools point to the same PostgreSQL.
- **Non-repo consumers** (audit, idempotency, certs, metrics, auth, DEP) use Writer pool directly as `*sql.DB`. They don't need the Writer/Reader split because they either write or need to see their own writes immediately.
- **Integration tests found real bugs**: NULL column scan failures in command repo. Integration tests against live PostgreSQL catch issues that mock-based tests miss — column type mismatches, FK constraints, JSONB serialization.

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
| 5b | 🔲 Not Started | EventBus listener, compliance wiring, load testing |
| 5d | 🔲 Not Started | Web dashboard (HTMX) |
| 6 | 🔲 Not Started | macOS Platform SSO (Java/Swift) — requires Apple Developer account |
