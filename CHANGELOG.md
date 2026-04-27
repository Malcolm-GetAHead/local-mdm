# Changelog

Sprint-by-sprint development history for Local MDM. For current status, see [README.md](README.md).

---

## Sprint 5g — Quality Polish
- Fix N+1 queries in web handlers (batch `ListByIDs`, `CountMembersByGroupIDs`)
- HTMX loading indicator (CSS progress bar)
- Enhanced empty states with SVG icons and contextual guidance
- Playwright error state tests (`page.route()` interception)
- `make verify` target (vet + lint + unit + integration + browser)
- CertificateService and ReportingService refactored to use interfaces
- Blueprint assessment fixes: Swagger CDN → vendored (air-gap), rate limiter X-Forwarded-For, DSN escaping, EventBus retry max from DB, ListTemplates repo method, `fmt.Sprintf` JSON → `json.Marshal`, toast theme colors, CSP violation detection in Playwright
- Coverage pipeline fixed: instrumented binary now flushes on SIGTERM
- Test DB password defaults aligned with Docker Compose

## Sprint 5d — Web Dashboard
- SPA-like navigation with HTMX content swaps, 10 visual themes with dark mode
- Device management: list with DB-level filtering, detail with Platform Details tab, lock/wipe/unenroll/delete
- Policy management: CRUD with settings catalog, assign/unassign with pagination
- Groups: CRUD, inline edit, member toggle with filter preservation
- Compliance: per-setting pass/fail (keyed violations), card toggle filters
- Audit log: expandable details, action/date filters
- EventBus retry queue (migration 000013), dedicated session secret, CSRF protection
- 199 Playwright browser tests, 17 web handler Go tests

## Sprint 5b — EventBus & Compliance Wiring
- EventBus LISTEN/NOTIFY listener using `pq.Listener` with pre-flight check, reconnect, keepalive
- Migration 000011: 4 new triggers (platform_data, unassign, group membership)
- Compliance auto-evaluation wired to EventBus (7 subscribers)
- macOS CheckinHandler enriched (serial, name, model, OS, build on TokenUpdate)
- Windows security CSP URIs, Android webhook data persistence
- ComplianceCleanupHook on device unenroll/wipe/delete
- k6 load testing framework

## Sprint 5f — API Hardening & Test Hygiene
- `NewCAManager` fails on missing files (no silent CA generation)
- API handler test coverage: 48.8% → 67.8%
- Auth package coverage: 73.0% → 90.6%
- Consolidated duplicate `setupTestDB` into shared `testutil.ConnectDB(t)`

## Sprint 5e — Cert Verification + Test Hygiene
- Root cause: NanoMDM cert verification failure was a CA file path bug
- 20 `assert.Contains` → `assert.ErrorIs` migration
- SCEP handler coverage: 49.6% → 75.9%
- `NewCAManagerFromPEM` + env var support for production

## Sprint 5c — Platform Integration Fixes
- NanoMDM v0.9.0 deployed as Docker service
- macOS/Windows/Android enrollment flows verified end-to-end
- SCEP protocol compliance with PKCS#7 envelopes
- Sentinel error `apperrors.ErrNotFound` replaces `strings.Contains`
- `handlers.go` split into 10 domain-specific files
- Full mdmb device simulator enrollment verified
- All development moved to Docker containers

## Sprint 5 — Backend Polish, CLI, Observability
- Service layer: DeviceService, AppService, UserService, TokenService, ReportingService
- User management API with role validation
- API token system with `lmdm_` prefix, SHA-256 hashing
- Reporting endpoints with CSV export
- SCEP server with challenge-based enrollment
- Readiness probe with per-dependency latency checks
- Real compliance evaluation (OS version, encryption, password, apps)

## Sprint 4b — Read/Write Database Pools
- DB struct split into Writer and Reader pools (Aurora read replica ready)
- ReaderConfig with field-level fallback
- Transactor uses Writer pool exclusively

## Sprint 4 — Policy & Identity
- Unified policy model with platform translators
- Policy versioning, rollback, templates
- Static device groups with membership management
- Compliance engine with per-device evaluation
- Device lifecycle hooks
- Redis removed — all caching via PostgreSQL
- Idempotency-Key header support
- PostgreSQL event triggers for LISTEN/NOTIFY

## Sprint 3 — Commands, Profiles & Apps
- macOS MDM commands (12 builders), configuration profiles
- Windows CSP framework, WNS push client, SyncML Replace
- Android policy translation, DeviceCommander, AppManager
- Unified remote actions with command tracking
- App management service
- Windows provisioning packages (.ppkg)

## Sprint 2a — Gap Closure
- Full CRUD endpoints for enterprises, devices, policies
- NanoMDM webhook handlers, DEP sync loop
- Platform services wired into server

## Sprint 2 — Platform Core
- All API handlers wired to repository layer
- Platform enrollment flows: macOS, Windows, Android
- Windows OMA-DM sync with SyncML protocol
- macOS DEP integration with encrypted token storage
- Prometheus metrics

## Sprint 1 — Foundation
- Schema, auth, certs, enrollment stubs
- 23/24 issues resolved (95.8%)
