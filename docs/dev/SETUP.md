# Setup Guide

This guide will help you set up the Local MDM development environment.

## Prerequisites

- **Go**: 1.25 or higher ([download](https://go.dev/dl/))
- **PostgreSQL**: 15 or higher
- **Docker & Docker Compose**: For local development (optional but recommended)
- **golang-migrate**: For database migrations
- **`/etc/hosts` entry**: Add `127.0.0.1 keycloak` to `/etc/hosts` — required for dashboard login (Keycloak OIDC redirect)

## Quick Start

### 1. Clone the Repository

```bash
cd ~/Documents/GitRepos/Malcolm-GetAHead/local-mdm
```

### 2. Install Development Tools

```bash
make install-tools
```

This installs:
- `golang-migrate` - Database migration tool
- `golangci-lint` - Go linter

### 3. Start PostgreSQL

Using Docker Compose (recommended):

```bash
make docker-up
```

This starts:
- PostgreSQL on port 5432
- Keycloak (OIDC IdP) on port 8180
- NanoMDM (Apple MDM protocol handler) on port 9000
- Adminer (database UI) on port 8081

Or install PostgreSQL locally:
```bash
# macOS
brew install postgresql@15
brew services start postgresql@15

# Create database
createdb localmdm
```

### 4. Configure the Application

```bash
cp configs/config.example.yaml configs/config.yaml
```

Edit `configs/config.yaml` and update:
- Database credentials (if not using defaults)
- JWT secret (required for production)
- Platform-specific settings

**Important**: Change the JWT secret from the default value!

### 5. Run Database Migrations

```bash
make migrate-up
```

This creates all necessary database tables.

### 6. Seed Development Data

```bash
make seed
```

This loads sample enterprises, devices, policies, and groups for development.

### 7. Start the Server

```bash
make run
```

The server will start on `http://localhost:8080`

The **web dashboard** is available at http://localhost:8080 — log in with your Keycloak credentials (requires the `/etc/hosts` entry from Prerequisites).

### 8. Verify Installation

```bash
curl http://localhost:8080/health
```

Expected response:
```json
{
  "data": {
    "status": "healthy",
    "version": "1.0.0",
    "checks": {
      "database": "healthy",
      "keycloak": "healthy"
    },
    "timestamp": "2026-02-05T10:30:00Z"
  },
  "meta": {
    "timestamp": "2026-02-05T10:30:00Z",
    "request_id": "..."
  }
}
```

## Development Workflow

### Running the Server

```bash
# Run directly
make run

# Or build and run binary
make build
./bin/local-mdm
```

### Database Migrations

```bash
# Create a new migration
make migrate-create NAME=add_device_groups

# Run migrations
make migrate-up

# Rollback last migration
make migrate-down

# Force migration version (if stuck)
make migrate-force VERSION=1
```

### Testing

```bash
# Run all tests in Docker (canonical — runs all 19 packages with race detector)
make dev-test

# Run tests on host (skips integration tests that need Docker)
make test

# Run tests with coverage
make test-coverage
```

### Code Quality

```bash
# Format code
make fmt

# Run linter
make lint
```

### Docker Commands

```bash
# Start containers
make docker-up

# Stop containers
make docker-down

# View logs
make docker-logs
```

## Configuration

### Environment Variables

You can override configuration with environment variables:

```bash
export ENVIRONMENT=development
export DB_HOST=localhost
export DB_PORT=5432
export DB_USER=postgres
export DB_PASSWORD=postgres-dev-password-1234
export DB_NAME=localmdm
export JWT_SECRET=your-secret-key
export KEYCLOAK_CLIENT_SECRET=your-keycloak-secret
export DEP_ENCRYPTION_KEY=your-dep-key
# Reader pool overrides (optional, for read replicas)
export DB_READER_HOST=replica.example.com
export DB_READER_PORT=5432
```

### Database Connection

Default connection string (used by Makefile for migrations):
```
postgres://postgres:postgres-dev-password-1234@localhost:5432/localmdm?sslmode=disable
```

**Note**: The Go application reads database config from `configs/config.yaml` and environment variables (`DB_HOST`, `DB_PORT`, etc.), not from `DB_URL`. The `DB_URL` variable is only used by the Makefile for running migrations.

**Important**: Config validation rejects passwords shorter than 16 characters and common defaults like "postgres". For local development, the docker-compose PostgreSQL is pre-configured to accept the default password. For production, use a strong password.

## Platform-Specific Setup

### Windows MDM

No additional setup required for development. For production:
1. Obtain a valid TLS certificate
2. Configure discovery/enrollment URLs in config
3. Ensure server is accessible from Windows devices

### macOS MDM

macOS device management is handled by NanoMDM (included in Docker Compose). No additional setup required for development — NanoMDM handles the Apple MDM protocol and sends webhooks to Local MDM.

For real device enrollment (beyond the mdmb simulator):
1. Devices must be able to reach the server (use host IP, not localhost)
2. Install the enrollment profile via Safari: `http://<host-ip>:8080/enrollment/macos/profile`
3. APNs push certificate is a future requirement (F-01) — without it, devices only sync on reboot/manual check-in

### Android MDM

Requires Google Cloud project:

1. Create Google Cloud project
2. Enable Android Management API
3. Create service account
4. Download service account JSON
5. Place in `secrets/google-service-account.json`
6. Update config with project ID

## Troubleshooting

### Database Connection Failed

```bash
# Check if PostgreSQL is running
docker ps  # if using Docker
# or
pg_isready  # if installed locally

# Check connection
psql -h localhost -U postgres -d localmdm
```

### Migration Errors

```bash
# Check current migration version
migrate -path ./migrations -database "$DB_URL" version

# Force to specific version
make migrate-force VERSION=1

# Start fresh (WARNING: deletes all data)
make migrate-down
make migrate-up
```

### Port Already in Use

```bash
# Find process using port 8080
lsof -i :8080

# Kill process
kill -9 <PID>

# Or change port in config.yaml
```

### Go Module Issues

```bash
# Clean module cache
go clean -modcache

# Re-download dependencies
make deps
```

## Database Access

### Using Adminer (Web UI)

1. Start Docker containers: `make docker-up`
2. Open http://localhost:8081
3. Login with:
   - System: PostgreSQL
   - Server: postgres
   - Username: postgres
   - Password: postgres-dev-password-1234
   - Database: localmdm

### Using psql (CLI)

```bash
psql -h localhost -U postgres -d localmdm

# List tables
\dt

# Describe table
\d devices

# Query
SELECT * FROM devices;
```

## Next Steps

1. Review [API Documentation](../schemas/API.md)
2. Check [Database Schema](../schemas/DATABASE.md)
3. Read [Architecture](../architecture/ARCHITECTURE.md)
4. Follow [Progress](../planning/PROGRESS.md) for implementation status

## Common Make Commands

```bash
make help          # Show all available commands
make build         # Build the application
make run           # Run the application
make test          # Run tests
make clean         # Clean build artifacts
make docker-up     # Start Docker containers
make migrate-up    # Run database migrations
make dev           # Start full dev environment
```

## Production Deployment

See [DEPLOYMENT.md](../user/DEPLOYMENT.md) for production deployment guide.

## Getting Help

- Check [PROGRESS.md](../planning/PROGRESS.md) for known issues
- Review logs via `docker compose logs localmdm` or server stdout
- Enable debug logging in `config.yaml`

---

**Last Updated**: 2026-04-29
