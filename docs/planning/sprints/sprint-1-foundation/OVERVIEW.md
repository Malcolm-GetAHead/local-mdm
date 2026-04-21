# Sprint 1: Foundation

**Status**: ✅ Complete  
**Duration**: 2 weeks
**Goal**: Core infrastructure that all subsequent sprints depend on

## Tasks Overview

| ID | Task | Parallel? | Dependencies | Estimated Effort |
|---|---|---|---|---|
| S1-01 | [Database & Repository Layer](S1-01-database-repository.md) | ✅ Yes | None | 3-4 days |
| S1-02 | [Configuration & Server Bootstrap](S1-02-config-server.md) | ✅ Yes | None | 2-3 days |
| S1-03 | [Certificate Infrastructure (PKI)](S1-03-certificate-pki.md) | ✅ Yes | None | 3-4 days |
| S1-04 | [Keycloak Setup & OIDC Integration](S1-04-keycloak-oidc.md) | ✅ Yes | None | 3-4 days |
| S1-05 | [API Framework & Middleware](S1-05-api-framework.md) | ⚠️ Partial | S1-02, S1-04 (auth middleware needs OIDC) | 3-4 days |
| S1-06 | [Security Hardening & Secrets Management](S1-06-security-hardening.md) | ⚠️ Partial | S1-02, S1-05 | 2-3 days |
| S1-07 | [Testing Framework Setup](S1-07-testing-framework.md) | ✅ Yes | None | 1-2 days |

## Dependency Graph

```
S1-01 (DB/Repo) ─────────────────────┐
S1-02 (Config/Server) ───┐           │
S1-03 (PKI/Certs) ───────┤           │
S1-04 (Keycloak/OIDC) ───┤           │
                          ▼           ▼
                    S1-05 (API Framework)
```

S1-01 through S1-04 can all start in parallel on day 1. S1-05 depends on S1-02 for the server bootstrap and S1-04 for the OIDC token validation middleware, but route stubs and response formatting can begin immediately.

## Service-Level Dependencies

| This Sprint Produces | Consumed By |
|---|---|
| PostgreSQL connection pool + migrations | Every subsequent sprint |
| Repository interfaces (Device, User, Policy, Cert) | Sprint 2, 3, 4 |
| Configuration loading (YAML + env vars) | Every subsequent sprint |
| SCEP server integration | Sprint 2 (macOS enrollment) |
| CA cert generation + device cert signing | Sprint 2 (all platform enrollment) |
| Keycloak OIDC token validation middleware | Sprint 2+ (all API endpoints) |
| RBAC middleware (role extraction from Keycloak tokens) | Sprint 2+ (protected routes) |
| HTTP server with routing, logging, CORS | Sprint 2+ (platform endpoints) |
| Health check + version endpoints | Ops/monitoring |

## Definition of Done

- [x] `make docker-up` starts PostgreSQL + Keycloak
- [x] `make migrate-up` applies all schema migrations
- [x] `make run` starts the server with health check passing
- [x] OIDC login flow works against local Keycloak
- [x] Protected API endpoint rejects unauthenticated requests
- [x] CA certificate can be generated and stored
- [x] Device certificate can be signed from a CSR
- [x] All repository CRUD operations have integration tests
