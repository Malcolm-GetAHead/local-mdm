# S5-11: User Management CRUD & API Token Authentication

**Sprint**: 5 — Backend Polish  
**Parallel**: ✅ Yes (must complete before S5-08 CLI Tools)  
**Depends on**: Sprint 4 (service layer pattern), existing `users` and `api_tokens` tables  
**Effort**: 2-3 days

## Problem

The CLI (S5-08) and web dashboard (S5b) both assume user management and API token endpoints exist. The database tables (`users`, `api_tokens`) are in the schema since migration 000001, but no server-side handlers, services, or repositories exist for them. The CLI literally can't authenticate without API token support.

## Scope

Build the minimum needed for CLI and dashboard auth. Keycloak user provisioning integration (syncing users from Keycloak to the local DB) is deferred to F-07.

## Tasks

### 1. UserService & UserRepository
- `internal/service/user.go` — CRUD, role validation, enterprise scoping
- `internal/repository/user.go` — DB queries against existing `users` table
- Methods: `Create`, `Get`, `List` (by enterprise), `Update` (role, active status), `Deactivate` (soft delete)
- Password hashing not needed — auth is via Keycloak OIDC or API tokens

### 2. API Token Service & Repository
- `internal/service/token.go` — generate, validate, revoke
- `internal/repository/token.go` — DB queries against existing `api_tokens` table
- Token generation: cryptographically random (32 bytes, base64url encoded)
- Storage: hash with pgcrypto `crypt()`/`gen_salt()` — plaintext returned once at creation, never stored
- Validation: hash incoming token, compare against stored hash
- Scoping: enterprise_id, role (inherited from creating user), optional expiration

### 3. Token Auth Middleware
- Alternative to OIDC JWT — check `Authorization: Bearer <token>` against `api_tokens` table
- Falls through to OIDC validation if token not found in DB (supports both auth methods)
- Update `internal/auth/middleware.go` to try token auth before OIDC

### 4. API Handlers
```
GET    /api/v1/users                  — List users (enterprise-scoped)
POST   /api/v1/users                  — Create user (admin+)
GET    /api/v1/users/{id}             — Get user
PUT    /api/v1/users/{id}             — Update user (role, active)
DELETE /api/v1/users/{id}             — Deactivate user

POST   /api/v1/tokens                 — Generate API token (returns plaintext once)
GET    /api/v1/tokens                 — List tokens (metadata only)
DELETE /api/v1/tokens/{id}            — Revoke token
```

### 5. Tests
- UserService unit tests (CRUD, role validation, enterprise scoping)
- TokenService unit tests (generate, validate, revoke, expiration)
- Token auth middleware tests (valid token, expired token, revoked token, fallthrough to OIDC)
- Handler tests with mock repos

## Out of Scope (stays in F-07)

- Keycloak user provisioning (auto-sync users from Keycloak realm)
- Token scoping beyond enterprise + role (fine-grained API scopes)
- Token rotation / refresh

## Acceptance Criteria

- [ ] User CRUD endpoints work with enterprise scoping
- [ ] API token generated and returned once at creation
- [ ] Token validated on subsequent requests (hash comparison)
- [ ] Token revocation immediately prevents access
- [ ] Both OIDC JWT and API token auth work on all protected endpoints
- [ ] S5-08 CLI can authenticate with an API token

## Reference

Detailed spec in [F-07 §9 (User Management)](../../future/F-07-advanced-features.md) and [F-07 §10 (API Token Auth)](../../future/F-07-advanced-features.md). This task implements the core subset needed for CLI and dashboard.

---

*Created 2026-04-20 during Sprint 4 forward look.*
