# Session Notes — Working Preferences & Project Knowledge

**Last Updated**: 2026-04-21  
**Purpose**: Guidance for AI agents working on this codebase. Keep this file lean — patterns and conventions that apply to every session. One-shot implementation details belong in sprint docs, not here.

---

## Working Style

- Ask clarifying questions before implementation starts, not during. Batch all questions upfront.
- Owner trusts the agent to make reasonable decisions — don't ask for approval on every detail.
- "Do it all" requests are common — batch work efficiently rather than asking for ordering preferences.
- When unsure about a technical decision, research it and recommend an approach. Owner will redirect if needed.
- Architecture discussions before code — present options with tradeoffs, let owner choose, then implement.
- Owner has strong architectural instincts. Watch for signals like "would it make more sense to..." — these are directional decisions, not questions. Confirm and execute.
- Owner values honest pushback. Explain why an alternative is better. The owner redirects when they disagree.

## Lessons for Future Agents

- **Don't accept "acceptable for now" without verifying.** If coverage is low or tests use mocks for database code, investigate *why*. Run the actual tests, check what's uncovered, and look at the uncovered code. In this session, "macos 59% — borderline" turned into 2 real JSONB serialization bugs that would have broken DEP sync in production. The owner will push you on this — get ahead of it.
- **Verify claims from earlier sprints.** Sprint completion docs may say "deferred — not critical for MVP" about things that are now critical. The SCEP server was deferred in Sprint 1 and nobody tracked it since. Check deferred items against current requirements.
- **The owner thinks in infrastructure, not code.** When presenting solutions, frame them in terms of services, data flow, failure modes, and deployment topology — not Go function signatures. The owner catches architectural gaps (missing NanoMDM in diagrams, no Keycloak service, SCEP not wired up) that code-focused analysis misses.
- **Don't default to Kubernetes.** The owner's instinct is toward lighter orchestration (ECS Fargate). If a doc or plan assumes K8s, flag it and propose the simpler alternative.
- **Integration tests find real bugs that mocks hide.** Every time we wrote integration tests against live PostgreSQL, we found bugs — NULL column scans in command repo, JSONB serialization in DEP storage. When the owner asks "why don't we have tests for this?", the answer should be "let me write them now" not "it's tested through mocks."
- **The owner asks probing questions that look simple but aren't.** "Don't we have real PostgreSQL?" and "Is this SCEP work detailed anywhere?" both led to significant findings. Take these questions seriously — investigate fully before answering.

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
- Services: `PolicyService`, `GroupService`, `ComplianceService`, `LifecycleService`.
- **Two patterns coexist**: Sprint 2/3 handlers call repos directly; Sprint 4+ handlers call services. S5-10 will migrate the rest. Don't extend the old pattern — new business logic goes in services.
- Services accept repository interfaces via constructor (dependency injection).

### Error Handling
- Not-found detection: `strings.Contains(err.Error(), "not found")` — repos return `fmt.Errorf`, not sentinel errors.
- Audit logging: `s.logAudit(r, action, resourceType, resourceID, details)` on all mutations.

### Testing
- **Handler tests**: mock repos in `handler_test_helpers_test.go`. New endpoints must be registered in `newTestServer()`.
- **Service tests**: own mocks in `*_test.go` within `internal/service/`.
- **Integration tests**: need Docker services (`docker compose up -d` then `migrate up`). PostgreSQL is always available — don't use `-short` flag to skip integration tests.
- **Always run `go test -race ./...`** (no `-short`) — the Docker stack is running.

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
- **nanomdm** — Apple MDM protocol handler (separate ECS service). Receives `/checkin` and `/mdm` via ALB path routing. Shares RDS database. Local MDM sends commands to NanoMDM via HTTP API (`nanomdm_url` config).
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
- **EventBus**: PostgreSQL triggers in place (migration 000007). Go-side LISTEN/NOTIFY listener not yet built — S5-09 owns this. Must use dedicated connection on Writer pool DSN (read replicas don't relay NOTIFY).
- **Compliance engine**: infrastructure complete but `evaluatePolicy()` returns "unknown" until S5-09.
- **Policy deployment**: assignments recorded, devices pick up on next check-in. No immediate push (intentional).
- **Sprint 2 security review docs** contain false claims. Trust the code, not the review narratives.
- **Dashboard**: Go templates + HTMX + Tailwind CSS (Sprint 5b). Not React.
- **macOS Platform SSO**: Sprint 4c (Java + Swift, separate from Go work).

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
- **command.GetByID and ListByDevice bug** — `error_message` column is nullable TEXT but scanned into `string` (not `*string`). Fails for pending/sent commands where error_message is NULL. Documented in `sprint12_gaps_integration_test.go`. Fix needed in Sprint 5.

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
| 4c | 🔲 Not Started | macOS Platform SSO (Java/Swift) |
| 5 | 🔲 Not Started | Backend polish, CLI, observability, performance |
| 5b | 🔲 Not Started | Web dashboard (HTMX) |
