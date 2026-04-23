# Getting Started with Local MDM

A quick guide to get the Local MDM server running locally and make your first API calls.

## Prerequisites

- **Go 1.25+** — [download](https://go.dev/dl/)
- **Docker & Docker Compose** — for PostgreSQL, Keycloak, NanoMDM, and Adminer
- **golang-migrate** — install with `make install-tools`

## Quick Start

### 1. Start Services

```bash
docker compose up -d
```

This starts:
- **PostgreSQL 15** on port 5432 (databases: `localmdm`, `keycloak`, `nanomdm`)
- **Keycloak 23** on port 8180 (OIDC identity provider)
- **NanoMDM v0.9.0** on port 9000 (Apple MDM protocol handler, webhooks to Local MDM)
- **Adminer** on port 8081 (database UI)

Wait for Keycloak to finish starting (~30–45 seconds):

```bash
docker compose logs -f keycloak
# Wait until you see: "Listening on: http://0.0.0.0:8080"
```

### 2. Run Migrations

```bash
make migrate-up
```

### 3. Copy Config

```bash
cp configs/config.example.yaml configs/config.yaml
```

The defaults work out of the box with the Docker Compose services. The Keycloak client secret for dev is `localmdm-api-secret` — set it in your config or via environment variable:

```bash
export KEYCLOAK_CLIENT_SECRET=localmdm-api-secret
```

### 4. Start the Server

```bash
make run
```

The server starts on `http://localhost:8080`.

## Verify It Works

### Health Check

```bash
curl -s http://localhost:8080/health | jq
```

```json
{
  "data": {
    "status": "healthy",
    "version": "1.0.0",
    "checks": {
      "database": "healthy",
      "keycloak": "healthy"
    },
    "timestamp": "2026-04-22T12:00:00Z"
  },
  "meta": { "timestamp": "2026-04-22T12:00:00Z", "request_id": "..." }
}
```

### Readiness Check (with latency)

```bash
curl -s http://localhost:8080/health/ready | jq
```

## Authenticate

All API endpoints require a Bearer token. Local MDM supports two auth methods:

1. **Keycloak JWT** — for interactive use and bootstrapping
2. **API token** (`lmdm_` prefix) — for CLI and automation

### Get a Keycloak Token

The dev Keycloak realm ships with a default admin user:

```bash
TOKEN=$(curl -s -X POST \
  http://localhost:8180/realms/localmdm/protocol/openid-connect/token \
  -d "grant_type=password" \
  -d "client_id=localmdm-api" \
  -d "client_secret=localmdm-api-secret" \
  -d "username=admin" \
  -d "password=admin123" | jq -r '.access_token')
```

Use this token for all subsequent requests:

```bash
AUTH="Authorization: Bearer $TOKEN"
```

## Create Your First Enterprise

```bash
curl -s -X POST http://localhost:8080/api/v1/enterprises \
  -H "$AUTH" \
  -H "Content-Type: application/json" \
  -d '{"name": "Acme Corp", "slug": "acme"}' | jq
```

Save the enterprise ID from the response — you'll need it for the Keycloak user's `enterprise_id` attribute.

> **Note**: The default Keycloak admin user has `enterprise_id` set to `00000000-0000-0000-0000-000000000001`. If your enterprise gets a different ID, update the user's attribute in Keycloak Admin Console at http://localhost:8180/admin (login: `admin` / `admin`).

## Create a User

```bash
curl -s -X POST http://localhost:8080/api/v1/users \
  -H "$AUTH" \
  -H "Content-Type: application/json" \
  -d '{"email": "operator@acme.dev", "full_name": "Ops User", "role": "operator"}' | jq
```

## Generate an API Token

API tokens are prefixed with `lmdm_` and work with both the CLI and direct API calls. The plaintext token is shown once at creation — save it.

```bash
curl -s -X POST http://localhost:8080/api/v1/tokens \
  -H "$AUTH" \
  -H "Content-Type: application/json" \
  -d '{"name": "cli-token"}' | jq
```

Save the `plaintext` value from the response:

```bash
export LOCALMDM_TOKEN="lmdm_..."
```

## CLI Quickstart

Build the CLI:

```bash
go build -o bin/localmdm-cli ./cmd/cli
```

The CLI reads `LOCALMDM_TOKEN` from the environment and defaults to `http://localhost:8080`.

```bash
# Check server health
./bin/localmdm-cli health

# List devices
./bin/localmdm-cli devices list

# List policies
./bin/localmdm-cli policies list

# List users
./bin/localmdm-cli users list

# Create an API token
./bin/localmdm-cli tokens create "my-token"

# JSON output
./bin/localmdm-cli devices list -o json
```

Override the server URL or token per-command:

```bash
./bin/localmdm-cli --server http://mdm.example.com --token lmdm_... devices list
```

## Development Commands

```bash
make help             # Show all available commands
make run              # Start the server
make test             # Run tests with race detector
make test-coverage    # Generate coverage report
make docker-up        # Start Docker services
make docker-down      # Stop Docker services
make migrate-up       # Run database migrations
make migrate-down     # Rollback migrations
make dev              # docker-up + migrate-up + run
```

## Next Steps

- [Development Setup](docs/dev/SETUP.md) — full dev environment details, environment variables, troubleshooting
- [API Reference](docs/schemas/API.md) — complete REST API documentation
- [Database Schema](docs/schemas/DATABASE.md) — tables, migrations, and data model
- [Architecture](docs/architecture/ARCHITECTURE.md) — system design and component overview
- [Deployment Guide](docs/user/DEPLOYMENT.md) — production deployment on AWS ECS Fargate
