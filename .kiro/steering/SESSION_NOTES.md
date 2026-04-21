# Session Notes — Working Preferences & Project Knowledge

**Last Updated**: 2026-04-20  
**Purpose**: Guidance for AI agents working on this codebase, based on observed preferences and project-specific knowledge from Sprint 2 through Sprint 4 implementation.

---

## Working Style

- Ask clarifying questions before implementation starts, not during
- Owner trusts the agent to make reasonable decisions — don't ask for approval on every detail
- Keep documentation up to date as work happens, not as an afterthought
- Test coverage matters — run tests frequently, flag real bugs found by tests
- "Do it all" requests are common — batch work efficiently rather than asking for ordering preferences
- When unsure about a technical decision (e.g. protocol format, library API), research it and recommend an approach rather than asking the owner to decide. Owner will redirect if needed.
- After completing a sprint, do a retrospective: check scope alignment, audit for gaps, verify docs are current, and update steering files with learnings. Owner values this.
- When the owner asks "are we missing anything?" — do a thorough audit (scope vs delivered, future sprint dependencies, codebase health). Don't just say "looks good."
- Clean up after yourself: delete merged branches, remove dead code, push to origin. Don't leave loose ends.

## Owner Background

- **DevOps engineer** by trade — understands infrastructure, deployment, and operations deeply
- **Career path**: Windows/Linux sysadmin → DevOps. Understands systems management, policies, and MDM concepts from the admin side
- **Languages**: Python and PostgreSQL at intermediate level (utility, not primary craft). No Go experience — don't assume the owner reads Go code directly
- **Strengths**: Architecture patterns, infrastructure decisions, tradeoff analysis, knowing when something smells wrong
- **Communication style**: Reviews agent outputs, asks clarifying questions, gives directional nudges on implementation decisions. Prefers to discuss architecture before code changes.
- **When explaining**: Use plain language for Go-specific patterns (interfaces, goroutines, channels, etc.). Relate to concepts the owner already knows (Python equivalents, infrastructure analogies). The owner will ask if they don't understand — don't over-simplify, just be clear.

## Git Workflow

- **Always create a feature branch before starting work.** Branch naming: `sprint-{id}/{short-description}` (e.g. `sprint-2a/gap-closure`). Never commit directly to main.
- **Commit after each completed sub-task**, not at the end of the sprint. Each commit should represent a logical, working unit (e.g. "S2a-01: Add missing CRUD endpoints" as one commit, "S2a-02: Wire platform services" as the next). This lets the owner review progress incrementally.
- **Commit messages should reference the task ID** from the sprint plan (e.g. `S2a-01:`, `S2a-03:`) so commits map back to the plan.
- **Run tests before each commit** — don't commit broken code. If tests pass with `-race`, commit.
- **Push periodically** so work isn't lost if the session ends unexpectedly. Push after each sub-task commit at minimum.
- Don't squash or amend unless asked — the owner wants to see the progression.

## Implementation Preferences

- Follow existing patterns exactly — constructor style, error handling, repository pattern, test structure. Don't introduce new patterns.
- Repos accept `interface{}` and type-switch on `*sql.DB` or `executor`. All use `getExecutor(ctx, r.db)` for transaction awareness.
- Error detection for not-found uses `strings.Contains(err.Error(), "not found")` — repos return plain `fmt.Errorf`, not sentinel errors. This is a known pattern.
- Audit logging via `s.logAudit(r, action, resourceType, resourceID, details)` on all mutations.
- Handler tests use mock repos in `handler_test_helpers_test.go` — no infrastructure needed.
- Integration tests need Docker services: `docker compose up -d` then `migrate up`.

## Project-Specific Knowledge

- The Sprint 2 security review docs (docs/reviews/sprint-2/) contain some false claims from automated review (SQL injection, memory leaks, deadlocks that don't exist). Trust the code, not the review narratives.
- H-05 (TLS validation) was closed as invalid — `InsecureSkipVerify` doesn't exist in the codebase.
- DEP tokens are encrypted with pgcrypto (`pgp_sym_encrypt`/`pgp_sym_decrypt`), encryption key from `DEP_ENCRYPTION_KEY` env var or `secrets/dep_encryption.key`.
- The `secrets/` directory is gitignored and for dev only — production will use AWS SSM.
- Idempotency-Key header support implemented in Sprint 4 — PostgreSQL-backed middleware on all POST/PUT/PATCH endpoints with 24h TTL and periodic cleanup.
- Prometheus metrics run on a separate internal port (default 127.0.0.1:9090), not the public API port.
- macOS Platform SSO is in Sprint 4c (separate from Sprint 4 policy work) due to Java/Swift dependencies.
- Web dashboard is in Sprint 5b (separate from Sprint 5 backend polish). Uses Go templates + HTMX + Tailwind CSS, not React.
- S4-02 (Policy Groups) is scoped to static groups only; dynamic groups are in F-07.
- Redis has been fully removed from the project (Sprint 4). Token cache uses PostgreSQL. No Redis in docker-compose or config.

## What NOT to Do

- Don't add Redis caching or abstractions unless the task specifically calls for it. Redis was removed in Sprint 4 — PostgreSQL handles all caching needs.
- Don't change the secrets storage approach (file-based for dev is intentional).
- Don't build things "just in case" — the steering guide is explicit about minimal implementation.
- Don't trust the Sprint 2 review docs at face value — verify claims against actual code.

## Sprint 2a Learnings (Testing & Mocks)

- Mock repos in `handler_test_helpers_test.go` may have no-op stub methods (e.g. `Update` returning nil without doing anything). When adding handlers that exercise those methods, make the mocks functional first — add error fields (`updateErr`, `deleteErr`, `assignErr`) and real logic.
- Always grep for existing tests of a function before changing its behavior. Fleshing out a stub handler (e.g. going from `w.WriteHeader(200)` to parsing JSON) will break tests that send nil/empty bodies.
- The `newTestServer()` helper in `handler_test_helpers_test.go` registers routes without auth middleware. New endpoints must be added there too, or handler tests won't route.

## Sprint 2a Learnings (Platform Architecture)

- **NanoMDM is webhook-based**: Apple devices send raw XML plist to NanoMDM. NanoMDM forwards events to Local MDM as JSON webhooks. The `/checkin` and `/mdm` endpoints receive NanoMDM webhook JSON (see `WebhookEvent` struct in `webhook.go`), NOT raw Apple plist.
- **DEPStorageInterface satisfies multiple nanodep interfaces**: Our `DEPStorageInterface` already implements `godep.ClientStorage`, `sync.CursorStorage`, and `sync.AssignerProfileRetriever`. No adapters or wrappers needed — pass it directly.
- **nanodep/sync requires nanolib transitive dep**: Not in go.sum by default. Run `go get github.com/micromdm/nanodep/sync@v0.7.0` to pull it in. The sync package uses `github.com/micromdm/nanolib/log` (not `slog`) — skip `WithLogger` option and let it use its internal `NopLogger`; our callback already logs via `slog`.
- **Android webhook handlers are in the same package as service**: `webhook.go` can access `h.service.enterpriseRepo` (unexported fields) directly. No getter methods needed.

## Sprint 2a Learnings (Config & Feature Flags)

- `FeaturesConfig.EnableMetrics` was redundant with `MetricsConfig.Enabled` — removed in Sprint 2a. When adding feature flags, check if the functionality already has a config toggle elsewhere.
- `FeaturesConfig.EnableAuditLog` now controls whether the async audit logger or a `NopAuditLogger` is initialized. Previously it was always-on.
- `FeaturesConfig.EnableWebhooks` exists but is intentionally unwired until Sprint 3.
- `config.MacOS.DEPSyncInterval` exists in the config struct but had no runtime consumer until Sprint 2a wired the DEP sync loop.
- `config.Windows.ManagementURL` is used for the pre-created `ManagementHandler`. Falls back to `host:port` if empty.

## Sprint 3 Learnings (Platform Commands & Profiles)

- **NanoMDM HTTP API for command push**: Commands are sent to NanoMDM via its HTTP API (`POST /v1/push/{udid}` and `POST /v1/enqueue/{udid}`), not via webhooks. The webhook flow is inbound only (device → NanoMDM → Local MDM). Outbound commands go Local MDM → NanoMDM HTTP API → APNs → device.
- **macOS command builders are stateless**: Each of the 12 command builders (DeviceLock, EraseDevice, InstallProfile, RemoveProfile, DeviceInformation, etc.) produces a plist command payload. NanoMDM handles queuing and delivery.
- **Windows CSP framework uses SyncML Replace**: Pushing configuration to Windows devices uses `<Replace>` elements in SyncML (not `<Add>`). The CSP framework generates OMA-URI paths per CSP type (Policy, WiFi, VPN, DeviceLock, App).
- **WNS push client triggers device sync**: Windows devices don't poll — WNS push notification tells the device to initiate an OMA-DM sync session, at which point queued commands/CSPs are delivered.
- **Android policy translation is declarative**: Policies are translated to the Management API JSON format and applied via `enterprises.policies.patch`. The device picks up changes on next sync. DeviceCommander uses `devices.issueCommand` for immediate actions (lock/wipe/reboot).
- **Unified remote actions use platform dispatch**: The `/devices/{id}/commands` endpoint determines platform from the device record and dispatches to the appropriate platform service. Command tracking in `device_commands` table provides unified status regardless of platform.
- **App management is catalog-based**: The `apps` table is an enterprise-scoped catalog. Deployment is a separate action (`/apps/{id}/deploy`) that triggers platform-specific install flows (NanoMDM InstallApplication, Android Management API app policy, Windows CSP app install).
- **PPKG generation uses ICD XML**: Windows provisioning packages are built from ICD (Imaging and Configuration Designer) XML templates, then packaged and optionally signed with a dev certificate for trusted installation.
- **`FeaturesConfig.EnableWebhooks`** was wired in Sprint 3 for outbound webhook notifications on command completion events.

## Sprint 3 Session Observations (2026-04-20)

- **Retrospective pattern is high-value**: backward look (scope audit, code quality, docs, git) + forward look (dependency analysis, architecture decisions) between sprints catches gaps before they become expensive. Owner expects this after every sprint.
- **Architecture discussions before code**: Owner prefers to talk through design decisions (service layer, event bus, Redis removal, read/write pools) before any code is written. Present options with tradeoffs, let owner choose, then implement.
- **Owner asks excellent "why" questions**: Questions like "can we use PostgreSQL instead of Redis?" and "should this be an enum instead of null?" consistently lead to simpler, better solutions. Don't rush past these — they're the most productive part of the session.
- **Sprint 4 complete**: All 5 tasks + 3 prerequisites delivered. Service layer, policy system, compliance engine, lifecycle hooks, Redis removal, idempotency-key middleware. Retrospective caught versioning bug (handlers bypassing PolicyService) and Redis remnants in config/docker-compose.
- **Dashboard changed to HTMX**: Sprint 5b uses Go templates + HTMX + Tailwind CSS instead of React. No separate frontend build pipeline. Decision driven by simplicity and owner's skillset.

## Sprint 4 Session Observations (2026-04-20)

- **"Ask questions upfront, then run with it"** is the owner's preferred workflow. Batch all clarifying questions before implementation starts. Once answers are given, execute autonomously — don't ask for approval on each sub-task. The owner trusts the agent to make reasonable decisions within the agreed scope.
- **Owner has strong architectural instincts** — when presenting options, the owner often already has a leaning. Watch for signals like "would it make more sense to..." or "how about we..." — these aren't questions, they're directional decisions. Confirm and execute rather than re-arguing the tradeoff.
- **Retrospectives are non-negotiable.** After every sprint: backward look (scope audit, dead code, stale docs, test coverage) then forward look (next 2 sprints + future roadmap alignment). The owner will prompt for this if you don't offer it. The backward look consistently catches real bugs (Sprint 4: handlers bypassing PolicyService, Redis remnants in docker-compose).
- **Owner values honest pushback.** When asked "should we add this as Sprint 4d?", the agent pushed back with reasoning (wrong home, better as S5-09) and the owner agreed. Don't just say yes — explain why an alternative is better. The owner redirects when they disagree.
- **Sprint sub-numbering convention**: 4b, 4c, etc. are used for standalone work that's related to but separate from the main sprint. Owner prefers clean separation over cramming everything into one sprint. If a task touches a different domain or toolchain, suggest splitting it out.
- **Documentation updates happen during the sprint, not after.** The owner checks docs accuracy during retrospective and flags gaps. Keep README, DATABASE.md, API.md, and steering files current as you go.
- **Three sessions over two days produced Sprints 2a, 3, and 4.** The pace is fast. The owner expects full sprint delivery per session including retrospective and forward planning. Plan accordingly — don't spend time on unnecessary discussion when the path is clear.
- **Per-task commits are recovery checkpoints, not review artifacts.** The owner trusts the agent's output and doesn't review individual commits during the session. The granular commit history exists so that if context is lost mid-sprint, the next session can identify the last clean state and continue. Always push after each commit — work must survive session loss.

## Sprint 4 Learnings (Policy & Identity)

### Service Layer Pattern
- `internal/service/` introduced in Sprint 4 as business logic layer between handlers and repos.
- **Services are transport-agnostic** — no `net/http` imports. Handlers parse requests, call services, format responses.
- Sprint 4 services: `PolicyService`, `GroupService`, `ComplianceService`, `LifecycleService`.
- **Codebase has two patterns**: Sprint 2/3 handlers call repos directly; Sprint 4+ handlers call services. S5-10 will migrate the remaining handlers. Don't extend the old pattern — new business logic goes in services.
- Services accept repository interfaces via constructor (dependency injection). Mock repos in tests.

### Redis Removal
- Redis fully removed in Sprint 4. Token cache uses PostgreSQL `token_cache` table (SHA-256 hashed tokens, TTL-based expiry).
- `go-redis/v9` removed from `go.mod`. No Redis in `docker-compose.yml` or config.
- `RedisConfig` struct removed from `config.go`.
- Decision rationale: PostgreSQL is already the primary datastore, token cache doesn't need the performance of Redis at our scale, and PostgreSQL works across multiple server instances (sync.Map wouldn't).

### Policy System Architecture
- **Unified policy model**: define once, translate to macOS (plist profiles), Windows (SyncML CSP commands), Android (Management API JSON) via `PolicyService.Translate()`.
- **Versioning**: full config snapshots on every create/update. Rollback to any version. Stored in `policy_versions` table.
- **Templates**: `is_template` flag on policies. Clone template to create enterprise-specific policy.
- **Assignment**: priority-based (lower number = higher priority). Assign to device, group, or enterprise. Resolved via `GetEffectivePolicies()`.
- **Policy deployment**: assignments are recorded, devices pick up policies on next check-in. No immediate push — this is intentional. Lock/wipe/restart are immediate commands via a separate path.

### Compliance Engine
- Infrastructure complete (evaluation loop, result storage, API endpoints) but `evaluatePolicy()` returns "unknown" until S5-09 adds device state parsing.
- Direct service calls for Sprint 4. EventBus LISTEN/NOTIFY listener deferred to Sprint 5 — PostgreSQL triggers are in place (migration 000007), Go-side listener not yet built.

### Lifecycle Hooks
- `DeviceLifecycleHook` interface: `OnUnenroll`, `OnWipe`, `OnDelete`. Registered on `LifecycleService`.
- Hook errors are logged but don't block the operation. Multiple hooks supported.
- Currently no hooks registered (empty slice). Sprint 4c adds Keycloak hook.

### Idempotency-Key
- PostgreSQL-backed middleware on all POST/PUT/PATCH. Caches response for 24h.
- Periodic cleanup goroutine runs hourly to remove expired keys.

### Sprint Renaming
- Sprint 4b = Read/Write Database Pools (standalone refactor)
- Sprint 4c = macOS Platform SSO with Keycloak (formerly 4b)

### Testing Observations
- `handler_test_helpers_test.go` now includes mock repos for groups, policy assignments, compliance, and policy versions. New Sprint 4+ endpoints must be registered in `newTestServer()`.
- Service-layer tests use their own mocks (in `*_test.go` files within `internal/service/`), separate from handler test mocks.
- Repository tests for new Sprint 4 repos (group, compliance, policy_version) need integration tests with Docker PostgreSQL — unit tests only cover the service layer above them.
- Flaky test: `TestDeviceRepository_List_ContextCancellation` occasionally fails in full suite runs due to shared mock DB state. Pre-existing, not Sprint 4.
