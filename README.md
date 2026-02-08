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
- **Certificate management**: Automated PKI for device authentication
- **REST API**: Comprehensive API for integration and automation
- **Multi-tenant**: Support multiple enterprises/organizations

## Project Status

✅ **Sprint 1 Complete** - 23/24 issues resolved (95.8%)
- See [Sprint 1 Review](docs/reviews/sprint-1/) for completion status
- See [Implementation Docs](docs/implementation/sprint-1/) for completed features
- Ready to begin Sprint 2 (Platform Core)

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
# Prerequisites: Go 1.21+, PostgreSQL 15+

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

- **Language**: Go 1.21+
- **Database**: PostgreSQL 15+
- **Protocols**: OMA-DM, MS-MDE2, Apple MDM, Android Management API
- **Dependencies**: nanoMDM, Google API client

## License

MIT License - See [LICENSE](LICENSE) for details

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development guidelines.
