# Session Notes — Working Preferences & Project Knowledge

**Last Updated**: 2026-04-18  
**Purpose**: Guidance for AI agents working on this codebase, based on observed preferences and project-specific knowledge from Sprint 2 implementation.

---

## Working Style

- Ask clarifying questions before implementation starts, not during
- Owner trusts the agent to make reasonable decisions — don't ask for approval on every detail
- Keep documentation up to date as work happens, not as an afterthought
- Test coverage matters — run tests frequently, flag real bugs found by tests
- "Do it all" requests are common — batch work efficiently rather than asking for ordering preferences

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
