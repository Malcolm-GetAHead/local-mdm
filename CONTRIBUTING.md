# Contributing to Local MDM

## Development Setup

See [docs/dev/SETUP.md](docs/dev/SETUP.md) for full environment setup.

```bash
make docker-up      # Start infrastructure (PostgreSQL, Keycloak, NanoMDM)
make migrate-up     # Run migrations
make run            # Start server on host
```

## Testing

```bash
make dev-test       # Canonical: all 19 packages in Docker with race detector
go test -race ./... # Quick: host-only (skips integration tests)
```

Run `make dev-test` before every commit. All tests must pass with the race detector enabled.

## Code Standards

- Follow patterns in [.kiro/steering/STEERING.md](.kiro/steering/STEERING.md)
- Parameterized SQL queries only — no string concatenation
- Wrap errors with `fmt.Errorf("context: %w", err)`
- Add `t.Cleanup()` for all test resources (enterprises, DB connections)
- Use `testutil.CreateTestEnterprise()` or manual cleanup with `DELETE FROM enterprises WHERE id = $1`

## Git Workflow

- Feature branch per sprint: `sprint-{id}/{short-description}`
- One commit per logical unit, referencing task IDs
- Run `go test -race ./...` before each commit
- Push after each commit

## Documentation

- [Architecture](docs/architecture/ARCHITECTURE.md)
- [API Reference](docs/schemas/API.md)
- [Database Schema](docs/schemas/DATABASE.md)
- [Testing Guide](docs/TESTING.md)
- [Security](docs/SECURITY.md)
