# S1-04: Keycloak Setup & OIDC Integration

**Sprint**: 1 — Foundation
**Parallel**: ✅ No blockers — can start immediately
**Effort**: 3-4 days

## Objective

Replace custom JWT auth with Keycloak as the OIDC identity provider. Set up Keycloak for development, implement token validation, and map Keycloak roles to Local MDM RBAC.

## Tasks

### 1. Keycloak Development Setup
- Docker Compose service for Keycloak (port 8180 to avoid conflict with 8080)
- Dev realm `localmdm` with auto-import on startup
- Pre-configured clients:
  - `localmdm-api` — confidential client for backend API
  - `localmdm-dashboard` — public client for future web UI (PKCE)
  - `psso` — public client for macOS Platform SSO (future, but create now)
- Pre-configured realm roles: `super_admin`, `admin`, `operator`, `viewer`
- Pre-configured client scope: `urn:apple:platformsso`
- Seed admin user for development
- Files: `docker/keycloak/realm-export.json`

### 2. OIDC Token Validation
- Fetch Keycloak JWKS (JSON Web Key Set) on startup, cache with refresh
- Validate access tokens: signature, expiry, issuer, audience
- Extract claims: `sub`, `email`, `realm_access.roles`, `azp`
- Files: `internal/auth/oidc.go`

### 3. RBAC Role Mapping
- Map Keycloak realm roles to Local MDM permissions
- Permission model: `resource:action` (e.g., `devices:read`, `policies:write`)
- Role → permission mapping (configurable or hardcoded initially)
- Enterprise isolation: extract enterprise_id from token custom claim or user attribute
- Files: `internal/auth/rbac.go`, `internal/auth/permissions.go`

### 4. Auth Context
- Middleware extracts validated token into request context
- `auth.UserFromContext(ctx)` returns current user info (ID, email, roles, enterprise)
- Files: `internal/auth/context.go`

### 5. API Token Support (Service Accounts)
- Keycloak service account client credentials flow for machine-to-machine
- Or: Local MDM-issued API tokens stored in DB, validated locally
- Decision: use Keycloak client credentials for now, simpler
- Files: `internal/auth/api_token.go`

## Role → Permission Mapping

```
super_admin → all permissions
admin       → devices:*, policies:*, users:read, users:write, certs:*, audit:read
operator    → devices:*, policies:read, policies:write, certs:read, audit:read
viewer      → devices:read, policies:read, certs:read, audit:read
```

## Dependencies on Other Sprint 1 Tasks

| Dependency | Required For | Can Stub? |
|---|---|---|
| S1-02 Config | Keycloak URL, realm, client_id | Yes — hardcode defaults |
| S1-02 Docker Compose | Running Keycloak instance | No — need this for integration testing |

## Interfaces to Export

```go
type AuthMiddleware func(next http.Handler) http.Handler
type RequireRole func(roles ...string) func(http.Handler) http.Handler

type AuthUser struct {
    ID           string
    Email        string
    Roles        []string
    EnterpriseID uuid.UUID
}

func UserFromContext(ctx context.Context) (*AuthUser, error)
```

## Acceptance Criteria

- [ ] `docker-compose up` starts Keycloak with pre-configured realm
- [ ] Can obtain access token from Keycloak via password grant (dev only) or auth code flow
- [ ] Middleware validates token and rejects expired/invalid tokens with 401
- [ ] Middleware extracts roles and makes them available via context
- [ ] `RequireRole("admin")` middleware returns 403 for insufficient permissions
- [ ] JWKS cache refreshes automatically
