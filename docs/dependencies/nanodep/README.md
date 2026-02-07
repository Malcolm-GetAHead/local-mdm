# NanoDEP

> Source: [github.com/micromdm/nanodep](https://github.com/micromdm/nanodep)

NanoDEP provides tools and a Go library for communicating with Apple's Device Enrollment Program (DEP) API.

## Role in Local MDM

NanoDEP handles Apple DEP/ADE integration for our macOS module. Key integration points:

- **DEP token management** — PKI exchange with Apple ABM/ASM portals
- **DEP profile definition & assignment** — configure what happens at device setup
- **Device sync** — continuously fetch newly assigned devices from Apple
- **Transparently authenticating reverse proxy** — talk to Apple DEP APIs without session management
- **PostgreSQL storage backend** — stores DEP config, tokens, sync cursors

## Key Components

| Component | Purpose |
|---|---|
| `depserver` | Configuration API + authenticating reverse proxy to Apple DEP |
| `depsyncer` | Continuous device sync + auto-assignment tool |
| `deptokens` | Standalone token decryption utility (optional) |
| `godep` Go package | High-level Go methods for DEP API endpoints |
| `client` Go package | Low-level auth primitives and session management |

## Documentation

- [Quickstart Guide](quickstart.md) — Get NanoDEP running end-to-end
- [Operations Guide](operations-guide.md) — CLI flags, API endpoints, tools, depsyncer
- [OpenAPI Spec](openapi.yaml) — Full API specification for depserver
- [PostgreSQL Schema](schema-pgsql.sql) — Database table for DEP names
