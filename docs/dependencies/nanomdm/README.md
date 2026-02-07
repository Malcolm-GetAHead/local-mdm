# NanoMDM

> Source: [github.com/micromdm/nanomdm](https://github.com/micromdm/nanomdm)

NanoMDM is a minimalist Apple MDM server and Go library. It is the core macOS/iOS MDM component that Local MDM wraps.

## Role in Local MDM

NanoMDM handles the Apple MDM protocol for our macOS module. We integrate with it as a Go library and use its PostgreSQL storage backend. Key integration points:

- **MDM check-in handling** (Authenticate, TokenUpdate, CheckOut)
- **Command queuing and delivery** via the enqueue API
- **APNs push notifications** to trigger device check-ins
- **Webhook callbacks** for reacting to MDM events
- **PostgreSQL storage** for enrollment and command data

## What NanoMDM Does NOT Include

These are things Local MDM must provide separately:

- **SCEP server** — we use micromdm/scep (see [../scep/](../scep/))
- **TLS termination** — reverse proxy / load balancer required
- **DEP API access** — we use nanodep (see [../nanodep/](../nanodep/))
- **Enrollment profiles** — we must create and serve these ourselves
- **Blueprints / auto-commands** — no automatic command sending on enrollment
- **JSON command API** — commands are raw Plist only (see cmdr.py tool)

## Architecture

NanoMDM is a thin composable layer:

1. **HTTP handlers** — standard Go HTTP handlers for MDM and API requests (`http` package)
2. **Service layer** — composable interface for processing MDM requests (`service` package)
3. **Storage layer** — interfaces and implementations for enrollment/command data (`storage` package)

## Key Features

- Horizontal scaling with minimal local state
- Multiple APNs topics (multi-tenant potential)
- Multi-command targeting (same command to multiple enrollments)
- Migration endpoint for moving enrollments between backends
- MicroMDM-compatible webhook callbacks
- Enrollment-certificate authorization

## Documentation

- [Quickstart Guide](quickstart.md) — Get NanoMDM running with ngrok
- [Operations Guide](operations-guide.md) — CLI flags, HTTP endpoints, APIs
- [PostgreSQL Schema](schema-pgsql.sql) — Database tables for our backend
- [Webhook Event Schema](webhook-event.json) — JSON schema for webhook callbacks
