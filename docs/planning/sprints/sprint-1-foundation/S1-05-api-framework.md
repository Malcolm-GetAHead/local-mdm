# S1-05: API Framework & Middleware

**Sprint**: 1 — Foundation
**Parallel**: ⚠️ Partial — route stubs can start immediately, auth middleware needs S1-04
**Depends on**: S1-02 (server bootstrap), S1-04 (OIDC middleware)
**Effort**: 3-4 days

## Objective

HTTP routing, middleware stack, request/response helpers, and API endpoint stubs for all planned resources.

## Tasks

### 1. Router Setup
- gorilla/mux (or chi) with versioned API prefix `/api/v1/`
- Subrouters per resource group
- Platform-specific routes outside `/api/v1/` (e.g., `/mdm`, `/discovery`, `/checkin`)
- Files: `internal/api/server.go`, `internal/api/routes.go`

### 2. Middleware Stack
- Request ID generation (UUID per request, propagate in context + response header)
- Structured request logging (method, path, status, duration, request_id)
- CORS (configurable origins)
- Auth middleware (from S1-04)
- Rate limiting (basic, per-IP or per-token)
- Recovery (panic → 500 with log)
- Files: `internal/api/middleware.go`

### 3. Request/Response Helpers
- Standard JSON response envelope: `{ "data": ..., "meta": { "total": N, "page": N } }`
- Standard error response: `{ "error": { "code": "...", "message": "..." } }`
- Request body parsing with validation
- Pagination params extraction from query string
- Files: `internal/api/response.go`, `internal/api/request.go`

### 4. Endpoint Stubs
All return 501 Not Implemented initially. Implemented in later sprints.

**Core Resources**:
- `GET/POST /api/v1/enterprises`
- `GET/PUT/DELETE /api/v1/enterprises/{id}`
- `GET/POST /api/v1/devices`
- `GET/PUT/DELETE /api/v1/devices/{id}`
- `POST /api/v1/devices/{id}/lock`
- `POST /api/v1/devices/{id}/wipe`
- `GET/POST /api/v1/policies`
- `GET/PUT/DELETE /api/v1/policies/{id}`
- `GET/POST /api/v1/certificates`
- `GET /api/v1/audit-logs`

**Operational**:
- `GET /health` (implemented — no auth)
- `GET /version` (implemented — no auth)

Files: `internal/api/handlers/*.go` (one file per resource group)

### 5. OpenAPI Spec Sync
- Ensure route definitions match `docs/schemas/API.md`
- Add any new routes from Keycloak integration
- Files: update `docs/schemas/API.md`

## Dependencies

| Dependency | Status | Workaround |
|---|---|---|
| S1-02 Config/Server | Need server bootstrap | Can develop handlers in isolation, wire up later |
| S1-04 OIDC Middleware | Need auth middleware | Use a no-op middleware stub, swap in real one when ready |

## Acceptance Criteria

- [ ] All planned routes registered and return 501 with correct response envelope
- [ ] Auth middleware rejects unauthenticated requests on protected routes
- [ ] Health and version endpoints work without auth
- [ ] Request logging shows method, path, status, duration, request_id
- [ ] CORS headers present for configured origins
- [ ] Panic in handler returns 500, not crash
- [ ] Pagination params parsed correctly from `?page=2&per_page=25`
