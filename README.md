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
- PostgreSQL event triggers for LISTEN/NOTIFY (Go listener in Sprint 5)
- See [Sprint 4 Plan](docs/planning/sprints/sprint-4-policy-and-identity/OVERVIEW.md)

✅ **Sprint 4b Complete** - 4/4 tasks done (Read/Write Database Pools)
- DB struct split into Writer and Reader pools for Aurora read replica readiness
- All repository reads use Reader pool, writes use Writer pool
- ReaderConfig with field-level fallback (zero config change needed for dev)
- Transactor uses Writer pool exclusively; reads within transactions see uncommitted writes
- See [Sprint 4b Plan](docs/planning/sprints/sprint-4b-db-pools/OVERVIEW.md)

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
