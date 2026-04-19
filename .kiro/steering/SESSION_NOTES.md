# Session Notes — Working Preferences & Project Knowledge

**Last Updated**: 2026-04-19  
**Purpose**: Guidance for AI agents working on this codebase, based on observed preferences and project-specific knowledge from Sprint 2 and 2a implementation.

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

## Git Workflow

- **Always create a feature branch before starting work.** Branch naming: `sprint-{id}/{short-description}` (e.g. `sprint-2a/gap-closure`). Never commit directly to main.
- **Commit after each completed sub-task**, not at the end of the sprint. Each commit should represent a logical, working unit (e.g. "S2a-01: Add missing CRUD endpoints" as one commit, "S2a-02: Wire platform services" as the next). This lets the owner review progress incrementally.
- **Commit messages should reference the task ID** from the sprint plan (e.g. `S2a-01:`, `S2a-03:`) so commits map back to the plan.
- **Run tests before each commit** — don't commit broken code. If tests pass with `-race`, commit.
- **Push periodically** so work isn't lost if the session ends unexpectedly. Push after each sub-task commit at minimum.
- Don't squash or amend unless asked — the owner wants to see the progression.
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
- Full Idempotency-Key support is deferred to Sprint 4; Sprint 2 uses DB unique constraints + 409 Conflict.
- Prometheus metrics run on a separate internal port (default 127.0.0.1:9090), not the public API port.
- macOS Platform SSO is in Sprint 4b (separate from Sprint 4 policy work) due to Java/Swift dependencies.
- Web dashboard is in Sprint 5b (separate from Sprint 5 backend polish) due to React/frontend skillset.
- S4-02 (Policy Groups) is scoped to static groups only; dynamic groups are in F-07.

## What NOT to Do

- Don't add Redis caching, service layers, or abstractions unless the task specifically calls for it.
- Don't change the secrets storage approach (file-based for dev is intentional).
- Don't modify Docker Compose or Kubernetes configs.
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
