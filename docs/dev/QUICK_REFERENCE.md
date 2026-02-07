# Local MDM - Quick Reference

## Project Location
```
~/Documents/GitRepos/Malcolm-GetAHead/local-mdm
```

## Quick Start Commands

```bash
# Start everything
make dev

# Or step by step:
make docker-up      # Start PostgreSQL
make migrate-up     # Run migrations
make run            # Start server
```

## Essential Commands

| Command | Description |
|---------|-------------|
| `make help` | Show all commands |
| `make run` | Start server |
| `make build` | Build binary |
| `make test` | Run tests |
| `make docker-up` | Start PostgreSQL |
| `make docker-down` | Stop PostgreSQL |
| `make migrate-up` | Run migrations |
| `make migrate-down` | Rollback migrations |
| `make migrate-create NAME=xxx` | Create new migration |

## Important URLs

| Service | URL |
|---------|-----|
| API Server | http://localhost:8080 |
| Health Check | http://localhost:8080/health |
| Adminer (DB UI) | http://localhost:8081 |
| API Docs | http://localhost:8080/api/v1/docs (future) |

## Database Access

**Adminer (Web)**:
- URL: http://localhost:8081
- System: PostgreSQL
- Server: postgres
- User: postgres
- Password: postgres
- Database: localmdm

**psql (CLI)**:
```bash
psql -h localhost -U postgres -d localmdm
```

## Configuration

**File**: `configs/config.yaml`

**Environment Variables**:
```bash
export DB_HOST=localhost
export DB_PORT=5432
export DB_USER=postgres
export DB_PASSWORD=postgres
export DB_NAME=localmdm
export JWT_SECRET=your-secret-key
```

## Project Structure

```
local-mdm/
├── cmd/server/          # Main application
├── internal/
│   ├── api/            # HTTP handlers
│   ├── config/         # Configuration
│   ├── db/             # Database
│   ├── models/         # Data models
│   └── platform/       # Platform modules
├── migrations/         # Database migrations
├── configs/           # Config files
└── docs/              # Documentation
```

## Documentation

| Document | Purpose |
|----------|---------|
| [README.md](../../README.md) | Project overview |
| [SCOPE.md](../scope/SCOPE.md) | Project scope |
| [SETUP.md](SETUP.md) | Setup guide |
| [DATABASE.md](../schemas/DATABASE.md) | Database schema |
| [API.md](../schemas/API.md) | API reference |
| [ARCHITECTURE.md](ARCHITECTURE.md) | System design |
| [PROGRESS.md](PROGRESS.md) | Development progress |

## API Endpoints (Current)

### Working
- `GET /health` - Health check ✅

### Stubbed (Not Implemented Yet)
- `POST /api/v1/auth/login`
- `POST /api/v1/auth/refresh`
- `GET /api/v1/devices`
- `GET /api/v1/devices/:id`
- `POST /api/v1/devices/enroll`
- `DELETE /api/v1/devices/:id`
- `POST /api/v1/devices/:id/lock`
- `POST /api/v1/devices/:id/wipe`
- `GET /api/v1/policies`
- `POST /api/v1/policies`
- `GET /api/v1/policies/:id`
- `PUT /api/v1/policies/:id`
- `DELETE /api/v1/policies/:id`
- `POST /api/v1/policies/:id/assign`

## Database Tables

| Table | Purpose |
|-------|---------|
| `enterprises` | Organizations/tenants |
| `users` | Admin users |
| `devices` | Enrolled devices |
| `policies` | Management policies |
| `device_policies` | Device-policy links |
| `certificates` | PKI certificates |
| `api_tokens` | API access tokens |
| `audit_logs` | Audit trail |

## Common Tasks

### Create New Migration
```bash
make migrate-create NAME=add_device_groups
```

### Reset Database
```bash
make migrate-down  # Rollback all
make migrate-up    # Apply all
```

### View Logs
```bash
make docker-logs   # Docker logs
tail -f logs/app.log  # App logs (future)
```

### Run Tests
```bash
make test              # All tests
make test-coverage     # With coverage report
```

### Build for Production
```bash
make build
./bin/local-mdm
```

## Troubleshooting

### Port 8080 in use
```bash
lsof -i :8080
kill -9 <PID>
```

### Database connection failed
```bash
docker ps  # Check if running
make docker-up  # Start if not
```

### Migration stuck
```bash
make migrate-force VERSION=1
```

### Go module issues
```bash
go clean -modcache
make deps
```

## Development Workflow

1. **Start session**
   ```bash
   cd ~/Documents/GitRepos/Malcolm-GetAHead/local-mdm
   make docker-up
   make run
   ```

2. **Make changes**
   - Edit code
   - Server auto-restarts (future: add hot reload)

3. **Test changes**
   ```bash
   make test
   curl http://localhost:8080/health
   ```

4. **Create migration** (if needed)
   ```bash
   make migrate-create NAME=description
   # Edit migration files
   make migrate-up
   ```

5. **Update docs**
   - Update PROGRESS.md with changes
   - Document design decisions
   - Update API.md if endpoints changed

6. **End session**
   ```bash
   Ctrl+C  # Stop server
   make docker-down  # Stop PostgreSQL
   ```

## Next Steps

See [PROGRESS.md](PROGRESS.md) for current status and next tasks.

**Current Phase**: Foundation (40% complete)

**Next Tasks**:
1. Implement authentication (JWT)
2. Create repository layer
3. Create service layer
4. Add certificate infrastructure
5. Write tests

## Getting Help

- Check [PROGRESS.md](PROGRESS.md) for known issues
- Review [SETUP.md](SETUP.md) for detailed setup
- See [ARCHITECTURE.md](ARCHITECTURE.md) for design
- Read [API.md](API.md) for API details

---

**Last Updated**: 2026-02-05
