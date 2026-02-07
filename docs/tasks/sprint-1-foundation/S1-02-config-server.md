# S1-02: Configuration & Server Bootstrap

**Sprint**: 1 — Foundation
**Parallel**: ✅ No blockers — can start immediately
**Effort**: 2-3 days

## Objective

Configuration loading, validation, and the core server lifecycle (startup, graceful shutdown, signal handling).

## Tasks

### 1. Configuration System
- YAML config file loading (`configs/config.yaml`)
- Environment variable overrides (12-factor friendly)
- Validation on startup (required fields, valid ranges)
- Nested config structs: server, database, keycloak, certificates, logging
- Files: `internal/config/config.go`, `configs/config.example.yaml`

### 2. Server Bootstrap
- HTTP server with configurable listen address and timeouts
- Graceful shutdown on SIGINT/SIGTERM
- Startup banner with version, listen address, config summary
- Files: `cmd/server/main.go`

### 3. Docker Compose
- PostgreSQL 15 service
- Keycloak service (with realm import for dev)
- SCEP server service (micromdm/scep)
- Adminer for DB inspection (dev only)
- Persistent volumes for data
- Files: `docker-compose.yml`, `docker/keycloak/realm-export.json`

### 4. Makefile
- `run`, `build`, `test`, `lint`
- `docker-up`, `docker-down`
- `migrate-up`, `migrate-down`, `migrate-create`
- `dev` (docker-up + migrate-up + run)
- Files: `Makefile`

### 5. Structured Logging
- JSON structured logging (slog or zerolog)
- Log level from config
- Request ID propagation
- Files: `internal/logging/logger.go`

## Config Structure

```yaml
server:
  host: "0.0.0.0"
  port: 8080
  read_timeout: 30s
  write_timeout: 30s

database:
  host: localhost
  port: 5432
  name: localmdm
  user: localmdm
  password: localmdm
  max_connections: 25

keycloak:
  url: "http://localhost:8180"
  realm: "localmdm"
  client_id: "localmdm-api"

certificates:
  ca_path: "./data/ca"
  scep_url: "http://localhost:2016/scep"

logging:
  level: "info"
  format: "json"
```

## Acceptance Criteria

- [ ] Server starts with valid config, fails fast with clear error on invalid config
- [ ] Environment variables override YAML values
- [ ] `docker-compose up` brings up PostgreSQL + Keycloak + SCEP
- [ ] Graceful shutdown completes in-flight requests
- [ ] Structured logs include timestamp, level, request_id, caller
