# Local MDM - Unified Multi-Platform Mobile Device Management

A unified, open-source MDM platform supporting Windows, macOS, and Android devices with minimal agent requirements.

## Overview

Local MDM provides enterprise device management capabilities across multiple platforms:
- **Windows 10/11**: Agent-less enrollment using native MDM client
- **macOS**: Integration with nanoMDM for Apple device management
- **Android**: Google Android Management API integration

## Architecture

The system consists of a unified Go-based control plane that orchestrates platform-specific MDM implementations:

```
Unified Control Plane (Go)
├── Windows Module (Custom OMA-DM/MS-MDE2)
├── macOS Module (nanoMDM wrapper)
└── Android Module (Google Management API)
```

## Key Features

- **Multi-platform support**: Manage Windows, macOS, and Android from a single interface
- **Agent-less enrollment**: Windows and macOS use native OS capabilities
- **Policy abstraction**: Define policies once, deploy across platforms
- **Remote actions**: Unified lock, wipe, and restart across all platforms
- **App management**: Deploy and manage applications across device fleets
- **Configuration profiles**: WiFi, VPN, certificates, and restrictions
- **Certificate management**: Automated PKI for device authentication
- **REST API**: Comprehensive API for integration and automation
- **Multi-tenant**: Support multiple enterprises/organizations

## Project Status

✅ **Sprint 1 Complete** - 23/24 issues resolved (95.8%)
- See [Sprint 1 Review](docs/reviews/sprint-1/) for completion status
- See [Implementation Docs](docs/implementation/sprint-1/) for completed features

✅ **Sprint 2 Complete** - 6/6 tasks done, 19/20 review issues resolved (95%)
- All API handlers wired to repository layer (zero 501 stubs)
- Platform enrollment flows: macOS, Windows (with cert signing), Android (with HMAC)
- Windows OMA-DM sync with SyncML protocol, DevDetail CSP, command queue
- macOS DEP integration with encrypted token storage (pgcrypto)
- Prometheus metrics on separate internal port
- See [Sprint 2 Review](docs/reviews/sprint-2/) for issue tracking
- See [Implementation Docs](docs/implementation/sprint-2/) for completed features

✅ **Sprint 2a Complete** - 5/5 tasks done (Gap Closure)
- Full CRUD endpoints for enterprises, devices, policies (update/delete)
- Policy assign/unassign endpoints for device targeting
- Platform services (macOS, Windows, Android) wired into server
- NanoMDM webhook handlers replace stubs (`/checkin`, `/mdm`)
- DEP sync loop with nanodep Syncer (configurable interval)
- Feature flag cleanup, Android webhook wiring, binary/file cleanup
- See [Sprint 2a Plan](docs/planning/sprints/sprint-2a-gap-closure/OVERVIEW.md)

✅ **Sprint 3 Complete** - 6/6 tasks done + gap closure (Commands, Profiles & Apps)
- macOS MDM commands (12 command builders), configuration profiles (WiFi/VPN/Cert/Restrictions), NanoMDM HTTP API integration
- Windows CSP framework (Policy/WiFi/VPN/DeviceLock/App CSPs), WNS push client, SyncML Replace support
- Android policy translation (security/restrictions/WiFi/apps → Management API), DeviceCommander, AppManager
- Unified remote actions (lock/wipe/restart) with command tracking, platform dispatch
- App management service (catalog model, CRUD API, deploy to devices)
- Windows provisioning packages (.ppkg) with ICD XML generation, signing, templates
- See [Sprint 3 Plan](docs/planning/sprints/sprint-3-platform-features/OVERVIEW.md)

✅ **Sprint 4 Complete** - 5/5 tasks done + prerequisites (Policy & Identity)
- Unified policy model with platform translators (macOS/Windows/Android)
- Policy versioning (full snapshots), rollback to any version, templates
- Static device groups with membership management
- Policy assignment to devices, groups, or enterprises (priority-based)
- Compliance engine with per-device evaluation and enterprise summary
- Device lifecycle hooks (unenroll/wipe/delete) with extensible hook interface
- Redis removed — token cache and idempotency keys use PostgreSQL
- Idempotency-Key header support on all POST/PUT/PATCH endpoints
- PostgreSQL event triggers for LISTEN/NOTIFY (Go listener in Sprint 5b)
- See [Sprint 4 Plan](docs/planning/sprints/sprint-4-policy-and-identity/OVERVIEW.md)

✅ **Sprint 4b Complete** - 4/4 tasks done (Read/Write Database Pools)
- DB struct split into Writer and Reader pools for Aurora read replica readiness
- All repository reads use Reader pool, writes use Writer pool
- ReaderConfig with field-level fallback (zero config change needed for dev)
- Transactor uses Writer pool exclusively; reads within transactions see uncommitted writes
- See [Sprint 4b Plan](docs/planning/sprints/sprint-4b-db-pools/OVERVIEW.md)

✅ **Sprint 5 Complete** - Backend Polish, CLI, Observability & Performance
- Service layer expansion: DeviceService, AppService, UserService, TokenService, ReportingService
- User management API (CRUD) with role validation and privilege escalation prevention
- API token system with `lmdm_` prefix, SHA-256 hashing, create/validate/revoke lifecycle
- Reporting endpoints: device inventory, compliance summary, enrollment trends (with CSV export)
- SCEP server with challenge-based certificate enrollment (`scep_challenges` table)
- Readiness probe (`GET /health/ready`) with per-dependency latency checks
- Audit log search with `action`, `start_date`, `end_date` query filters
- ComplianceService `evaluatePolicy()` now performs real policy evaluation (OS version, encryption, password, apps)
- Performance indexes (migration 000009) for common enterprise-scoped query patterns
- See [Sprint 5 Plan](docs/planning/sprints/sprint-5-ui-and-polish/OVERVIEW.md)

✅ **Sprint 5c Complete** - 10/10 tasks done (Platform Integration Fixes)
- NanoMDM v0.9.0 deployed as Docker service (Apple MDM protocol handler)
- macOS enrollment profile points to NanoMDM, webhook endpoint receives forwarded events
- Windows enrollment creates device records (enterprise ID in URL path)
- Android webhook handler wired (enrollment, status, unenrollment events processed)
- SCEP protocol compliance with PKCS#7 envelopes (go.mozilla.org/pkcs7)
- Sentinel error `apperrors.ErrNotFound` replaces fragile `strings.Contains` pattern
- `password_hash` nullable (migration 000010), "oidc-managed" placeholder removed
- `handlers.go` split into 10 domain-specific files
- Service layer test coverage: 30.5% → 61.9%
- E2E integration tests for macOS and Android webhook flows
- Full mdmb device simulator enrollment verified (SCEP + check-in + device record)
- 5 concurrent device enrollments tested (race-detector clean)
- All development and testing moved to Docker containers (Alpine Linux, environment parity)
- See [Sprint 5c Plan](docs/planning/sprints/sprint-5c-platform-integration/OVERVIEW.md)

✅ **Sprint 5e Complete** - 4/4 tasks done + bonus coverage work (Cert Verification + Test Hygiene)
- Root cause: NanoMDM cert verification failure was a CA file path bug, not a pkcs7 library incompatibility
- Fix: `projectPath()` helper for E2E tests, `.gitignore` for stale CA certs
- 20 `assert.Contains` → `assert.ErrorIs` migration across 10 files
- SCEP handler coverage: 49.6% → 75.9%, service coverage: 61.9% → 77.3%
- Fixed 3 hardcoded `localhost` test files preventing Docker execution
- Added `NewCAManagerFromPEM` + `CA_CERT_PEM`/`CA_KEY_PEM` env var support for production
- Overall test coverage: 63.8% → 65.7%
- See [Sprint 5e Plan](docs/planning/sprints/sprint-5e-cert-verification/OVERVIEW.md)

✅ **Sprint 5f Complete** - 3/3 tasks done (API Hardening & Test Hygiene)
- `NewCAManager` no longer silently generates CAs — fails on missing files, explicit `GenerateCA()` + `localmdm-cli certs init`
- API handler test coverage: 48.8% → 67.8% (80+ new test cases across groups, compliance, users, tokens, policy versioning, reports)
- Auth package coverage: 73.0% → 90.6% (circuit breaker, SSRF validation, audit logging, cache fallback)
- Consolidated 10+ duplicate `setupTestDB` functions into shared `testutil.ConnectDB(t)` / `testutil.ConnectRawDB(t)`
- See [Sprint 5f Plan](docs/planning/sprints/sprint-5f-api-hardening/OVERVIEW.md)

✅ **Sprint 5b Complete** - 7/7 tasks done (EventBus & Compliance Wiring)

✅ **Sprint 5d Complete** - Web Dashboard (Go templates + HTMX + Tailwind CSS)
- SPA-like navigation with HTMX content swaps, 10 visual themes with dark mode
- Device management: list with DB-level filtering, detail with Platform Details tab, lock/wipe/unenroll/delete
- Policy management: CRUD with settings catalog, assign/unassign with pagination
- Groups: CRUD, inline edit, member toggle with filter preservation
- Compliance: per-setting pass/fail (keyed violations), card toggle filters
- Audit log: expandable details, action/date filters
- EventBus retry queue (migration 000013), dedicated session secret, CSRF protection
- 196 Playwright browser tests, 17 web handler Go tests
- See [Sprint 5d Plan](docs/planning/sprints/sprint-5d-web-dashboard/OVERVIEW.md)
- EventBus LISTEN/NOTIFY listener using `pq.Listener` with pre-flight check, reconnect, keepalive
- Migration 000011: 4 new triggers (platform_data, unassign, group membership) + `extra` context in payload
- Compliance auto-evaluation wired to EventBus (7 subscribers: device/policy/group events)
- macOS CheckinHandler enriched (serial, name, model, OS, build, enrolled status on TokenUpdate)
- Windows security CSP URIs (BitLocker, firewall, password), Android webhook data persistence
- ComplianceCleanupHook on device unenroll/wipe/delete, Android lifecycle hooks wired
- k6 load testing framework with results history tracking (`results_history.csv`)
- See [Sprint 5b Plan](docs/planning/sprints/sprint-5b-eventbus/OVERVIEW.md)

## Documentation

### Getting Started
- [Setup Guide](docs/dev/SETUP.md) - Development environment setup
- [Quick Reference](docs/dev/QUICK_REFERENCE.md) - Common commands and workflows
- [Testing Guide](docs/TESTING.md) - Testing guidelines and best practices

### Architecture & Design
- [Architecture](docs/architecture/ARCHITECTURE.md) - System design and components
- [Project Scope](docs/scope/SCOPE.md) - Requirements and goals
- [API Documentation](docs/schemas/API.md) - REST API reference
- [Database Schema](docs/schemas/DATABASE.md) - Data model and migrations

### Development
- [Implementation Docs](docs/implementation/) - Feature implementations and fixes
- [Code Reviews](docs/reviews/) - Code review findings and tracking
- [Sprint Planning](docs/planning/sprints/) - Sprint-based development plans
- [Future Roadmap](docs/planning/future/) - Post-v1.0 enhancements

### Dependencies
- [External Dependencies](docs/dependencies/) - NanoMDM, NanoDEP, SCEP, Keycloak docs

## Quick Start

```bash
# Prerequisites: Go 1.25+, PostgreSQL 15+

# Clone and setup
cd local-mdm
cp configs/config.example.yaml configs/config.yaml

# Run migrations
make migrate-up

# Start server
make run

# Access dashboard
open http://localhost:8080
```

## Technology Stack

- **Language**: Go 1.25+
- **Database**: PostgreSQL 15+
- **Protocols**: OMA-DM, MS-MDE2, Apple MDM, Android Management API
- **Dependencies**: nanoMDM, Google API client

## License

MIT License - See [LICENSE](LICENSE) for details

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development guidelines.
