# Sprint 5f: API Hardening & Test Hygiene

**Status**: 🔲 Not Started  
**Duration**: 1-2 days  
**Goal**: Harden the API layer and fix design-level issues before the dashboard (5d) builds on top of it  
**Depends on**: Sprint 5e complete

---

## Why This Sprint

Sprint 5e's investigation revealed two systemic issues:

1. **NewCAManager silently generates CAs** when the file path is wrong. This is convenient for first-run dev setup but dangerous — a misconfigured production server would silently create an untrusted CA and start issuing useless certs. The auto-generate behavior should be explicit, not a fallback.

2. **API handler test coverage is 48.8%** — the lowest of any package. Handlers for groups, health, compliance, users, and half the policy endpoints have zero tests. Sprint 5d (dashboard) will wire a UI to these endpoints. Bugs found during dashboard work will slow that sprint down.

Fixing these before 5d means the dashboard builds on a tested, predictable API.

---

## Tasks

| ID | Task | Effort |
|---|---|---|
| S5f-01 | Make CA generation explicit — fail on missing files, add CLI command | 0.5 day |
| S5f-02 | API handler tests for untested endpoints | 1 day |
| S5f-03 | Consolidate test DB helpers into shared utility | 0.5 day |

### S5f-01: Explicit CA Generation

**Current behavior**: `NewCAManager(path, path)` → file not found → silently generates new CA → writes to disk.

**Target behavior**:
- `NewCAManager` fails with a clear error if files don't exist
- `NewCAManagerFromPEM` (added in 5e) fails with a clear error if PEM is invalid
- New CLI command: `localmdm-cli certs init` — explicitly generates CA cert/key at configured paths
- First-run dev setup: `make setup` or docs tell you to run `localmdm-cli certs init`
- Existing tests that rely on auto-generation use `t.TempDir()` + explicit generation helper

### S5f-02: API Handler Test Coverage

Target: API package ≥ 65% (currently 48.8%).

Untested handler groups (all 0%):
- `handlers_group.go` — CRUD + membership (8 handlers)
- `handlers_health.go` — health, ready, version, login, refresh (5 handlers)
- `handlers_compliance.go` — summary, device, evaluate (3 handlers)
- `handlers_user.go` — list, get, update, deactivate, create/list/revoke tokens (7 handlers)
- `handlers_policy.go` (partial) — versions, rollback, translate, templates, clone, target assign/unassign/list (9 handlers)

These all follow the existing mock-repo pattern in `handler_test_helpers_test.go`. Mechanical work — no design decisions needed.

### S5f-03: Consolidate Test DB Helpers

**Current state**: 5+ different `setupTestDB` / `setupDB` / `getTestDB` functions across test files, each with slightly different implementations. Three had hardcoded `localhost` (fixed in 5e).

**Target**: Single `testutil.ConnectDB(t)` function that:
- Reads `DB_HOST` / `DB_PASSWORD` env vars with localhost fallback
- Sets pool limits (2 open / 1 idle) to prevent connection exhaustion
- Calls `t.Cleanup()` for automatic close
- Skips test if DB unavailable

All integration tests import this instead of rolling their own.

---

## Definition of Done

- [ ] `NewCAManager` returns error when cert/key files don't exist (no silent generation)
- [ ] `localmdm-cli certs init` generates CA cert/key at configured paths
- [ ] API handler test coverage ≥ 65%
- [ ] Single shared `testutil.ConnectDB(t)` used by all integration tests
- [ ] All tests pass (`make dev-test`)

---

*Created: 2026-04-23 — Split from Sprint 5e review findings*
