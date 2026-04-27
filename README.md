# Local MDM - Unified Multi-Platform Mobile Device Management

A unified, open-source MDM platform supporting Windows, macOS, and Android devices with minimal agent requirements.

## Overview

Local MDM provides enterprise device management capabilities across multiple platforms:
- **Windows 10/11**: Agent-less enrollment using native MDM client
- **macOS**: Integration with nanoMDM for Apple device management
- **Android**: Google Android Management API integration

## Architecture

```
Unified Control Plane (Go)
├── Windows Module (Custom OMA-DM/MS-MDE2)
├── macOS Module (nanoMDM wrapper)
└── Android Module (Google Management API)
```

## Key Features

- **Multi-platform support**: Manage Windows, macOS, and Android from a single interface
- **Web dashboard**: HTMX-powered admin UI with 10 themes and dark mode
- **Policy abstraction**: Define policies once, deploy across platforms
- **Compliance engine**: Real-time evaluation with per-setting pass/fail
- **Remote actions**: Unified lock, wipe, and restart across all platforms
- **Certificate management**: Automated PKI with SCEP enrollment
- **REST API**: Comprehensive API for integration and automation
- **Multi-tenant**: Support multiple enterprises/organizations
- **Air-gap ready**: No external CDN dependencies, strict CSP

## Quick Start

```bash
# Prerequisites: Docker, Go 1.25+

# Start infrastructure
cp .env.example .env
make docker-up
sleep 45

# Run migrations and seed data
make migrate-up
make seed

# Start server
make run

# Access dashboard (requires /etc/hosts entry: 127.0.0.1 keycloak)
open http://localhost:8080
```

## Technology Stack

- **Language**: Go 1.25+
- **Database**: PostgreSQL 15+ (Writer/Reader pool split for Aurora)
- **Frontend**: Go templates + HTMX v2.0.9 + Tailwind CSS v4
- **Auth**: Keycloak OIDC + API tokens
- **Protocols**: OMA-DM, MS-MDE2, Apple MDM, Android Management API, SCEP
- **Testing**: 199 Playwright browser tests, Go unit/integration tests, k6 load tests

## Project Status

All backend features complete through Sprint 5g. Web dashboard operational with full device/policy/group/compliance management.

**Current**: Sprint 6 — Real device integration (Windows VM, macOS VM, Android)

See [CHANGELOG.md](CHANGELOG.md) for detailed sprint history.

## Documentation

- [Setup Guide](docs/dev/SETUP.md) — Development environment setup
- [Quick Reference](docs/dev/QUICK_REFERENCE.md) — Common commands and workflows
- [Testing Guide](docs/TESTING.md) — Testing guidelines and best practices
- [Architecture](docs/architecture/ARCHITECTURE.md) — System design and components
- [API Documentation](docs/schemas/API.md) — REST API reference
- [Database Schema](docs/schemas/DATABASE.md) — Data model and migrations
- [Sprint Planning](docs/planning/sprints/) — Sprint-based development plans
- [Future Roadmap](docs/planning/future/) — Post-v1.0 enhancements

## License

MIT License - See [LICENSE](LICENSE) for details

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development guidelines.
