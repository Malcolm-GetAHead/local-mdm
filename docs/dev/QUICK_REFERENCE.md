# Local MDM - Quick Reference

**Last Updated**: 2026-04-20

## Project Location
```
~/Documents/GitRepos/Malcolm-GetAHead/local-mdm
```

## Quick Start Commands

```bash
# Start everything
make dev

# Or step by step:
make docker-up      # Start PostgreSQL + Keycloak
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
| `make docker-up` | Start PostgreSQL + Keycloak |
| `make docker-down` | Stop services |
| `make migrate-up` | Run migrations |
| `make migrate-down` | Rollback migrations |
| `make migrate-create NAME=xxx` | Create new migration |
| `go test -race ./...` | Run tests with race detector |
| `go test -cover ./...` | Coverage summary |
| `go vet ./...` | Static analysis |

## Important URLs

| Service | URL |
|---------|-----|
| API Server | http://localhost:8080 |
| Health Check | http://localhost:8080/health |
| Version | http://localhost:8080/version |
| Prometheus Metrics | http://127.0.0.1:9090/metrics (internal only) |
| Adminer (DB UI) | http://localhost:8081 |

## Project Structure

```
local-mdm/
├── cmd/server/              # Main application entry point
├── internal/
│   ├── api/                 # HTTP handlers, middleware, routing
│   ├── apperrors/           # Structured application errors
│   ├── audit/               # Async audit logging
│   ├── auth/                # OIDC authentication (Keycloak)
│   ├── certs/               # Certificate management, CA
│   ├── config/              # Configuration loading & validation
│   ├── constants/           # Shared constants
│   ├── db/                  # Database connection & health (Writer/Reader pools)
│   ├── logging/             # Structured logging helpers
│   ├── metrics/             # Prometheus metrics server
│   ├── models/              # Data models (Device, Policy, etc.)
│   ├── platform/
│   │   ├── android/         # Android Management API integration
│   │   ├── macos/           # NanoMDM, DEP, enrollment profiles
│   │   └── windows/         # OMA-DM, SyncML, MS-MDE2
│   ├── repository/          # Data access layer (PostgreSQL)
│   ├── scep/                # SCEP challenge management
│   ├── service/             # Business logic layer (Sprint 4+)
│   ├── testutil/            # Test helpers
│   ├── tracing/             # Request tracing
│   └── validation/          # Input validation (JSONB, pagination)
├── migrations/              # Database migrations
├── configs/                 # Config files
├── secrets/                 # Dev-only secrets (gitignored)
└── docs/                    # Documentation
```

## API Endpoints

### Public (no auth)
| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check |
| GET | `/version` | Version info |

### Auth
| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/auth/login` | Login (returns JWT) |
| POST | `/api/v1/auth/refresh` | Refresh token |

### Enterprises
| Method | Path | Role | Description |
|--------|------|------|-------------|
| GET | `/api/v1/enterprises` | admin, super_admin | List enterprises |
| POST | `/api/v1/enterprises` | super_admin + IP allowlist | Create enterprise |
| GET | `/api/v1/enterprises/{id}` | any authenticated | Get enterprise |
| PUT | `/api/v1/enterprises/{id}` | admin, super_admin | Update enterprise |
| DELETE | `/api/v1/enterprises/{id}` | super_admin + IP allowlist | Delete enterprise |

### Devices
| Method | Path | Role | Description |
|--------|------|------|-------------|
| GET | `/api/v1/devices` | any authenticated | List devices |
| GET | `/api/v1/devices/{id}` | any authenticated | Get device |
| PUT | `/api/v1/devices/{id}` | admin, operator | Update device |
| DELETE | `/api/v1/devices/{id}` | admin | Delete device |
| POST | `/api/v1/devices/{id}/lock` | admin, operator | Lock device |
| POST | `/api/v1/devices/{id}/wipe` | admin + IP allowlist | Wipe device |

### Policies
| Method | Path | Role | Description |
|--------|------|------|-------------|
| GET | `/api/v1/policies` | any authenticated | List policies |
| POST | `/api/v1/policies` | admin, operator | Create policy |
| GET | `/api/v1/policies/{id}` | any authenticated | Get policy |
| PUT | `/api/v1/policies/{id}` | admin, operator | Update policy |
| DELETE | `/api/v1/policies/{id}` | admin | Delete policy |
| POST | `/api/v1/policies/{id}/assign` | admin, operator | Assign to devices |
| DELETE | `/api/v1/policies/{id}/assign/{device_id}` | admin, operator | Unassign from device |

### Certificates & Audit
| Method | Path | Role | Description |
|--------|------|------|-------------|
| GET | `/api/v1/certificates` | any authenticated | List certificates |
| GET | `/api/v1/audit-logs` | admin, super_admin | List audit logs |

### macOS
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/macos/enroll/{enterprise_id}` | Enrollment profile download |
| GET/PUT | `/api/v1/dep/{name}/tokenpki` | DEP token PKI management |
| GET/PUT | `/api/v1/dep/{name}/assigner` | DEP assigner profile |
| GET | `/api/v1/dep/{name}/devices` | List DEP-synced devices |
| PUT | `/checkin` | NanoMDM checkin webhook |
| PUT | `/mdm` | NanoMDM command webhook |

### Windows
| Method | Path | Description |
|--------|------|-------------|
| GET/POST | `/EnrollmentServer/Discovery.svc` | MS-MDE2 discovery |
| POST | `/EnrollmentServer/Policy.svc` | Enrollment policy |
| POST | `/EnrollmentServer/Enrollment.svc` | Device enrollment |
| POST | `/ManagementServer/MDM.svc` | OMA-DM SyncML sync |

### Android
| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/android/enrollment-token/{enterprise_id}` | Create enrollment token |
| GET | `/api/v1/android/enrollment-token/{token_id}/qr` | Get enrollment QR code |
| POST | `/api/v1/android/webhook` | Google Management API webhook |

### Device Commands & Profiles (Sprint 3)
| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/devices/{id}/commands` | Send command to device |
| GET | `/api/v1/devices/{id}/commands` | List command history |
| POST | `/api/v1/devices/{id}/profiles` | Install profile |
| DELETE | `/api/v1/devices/{id}/profiles/{profile_id}` | Remove profile |
| POST | `/api/v1/devices/{id}/restart` | Restart device |

### Apps (Sprint 3)
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/apps` | List apps |
| POST | `/api/v1/apps` | Create app |
| GET | `/api/v1/apps/{id}` | Get app |
| PUT | `/api/v1/apps/{id}` | Update app |
| DELETE | `/api/v1/apps/{id}` | Delete app |
| POST | `/api/v1/apps/{id}/deploy` | Deploy app to devices |

### Windows Provisioning (Sprint 3)
| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/windows/ppkg` | Generate provisioning package |
| GET | `/api/v1/windows/ppkg/templates` | List ppkg templates |

### Device Groups (Sprint 4)
| Method | Path | Role | Description |
|--------|------|------|-------------|
| GET | `/api/v1/groups` | any authenticated | List groups |
| POST | `/api/v1/groups` | admin, operator | Create group |
| GET | `/api/v1/groups/{id}` | any authenticated | Get group |
| PUT | `/api/v1/groups/{id}` | admin, operator | Update group |
| DELETE | `/api/v1/groups/{id}` | admin | Delete group |
| GET | `/api/v1/groups/{id}/members` | any authenticated | List group members |
| POST | `/api/v1/groups/{id}/members` | admin, operator | Add device to group |
| DELETE | `/api/v1/groups/{id}/members/{device_id}` | admin, operator | Remove device from group |

### Policy Versioning & Templates (Sprint 4)
| Method | Path | Role | Description |
|--------|------|------|-------------|
| GET | `/api/v1/policies/{id}/versions` | any authenticated | List policy versions |
| POST | `/api/v1/policies/{id}/rollback` | admin, operator | Rollback to version |
| GET | `/api/v1/policies/{id}/translate` | any authenticated | Translate policy to platform |
| GET | `/api/v1/policy-templates` | any authenticated | List policy templates |
| POST | `/api/v1/policy-templates/{id}/clone` | admin, operator | Clone template |

### Policy Assignments (Sprint 4)
| Method | Path | Role | Description |
|--------|------|------|-------------|
| GET | `/api/v1/policies/{id}/assignments` | any authenticated | List policy assignments |
| POST | `/api/v1/policies/{id}/assignments` | admin, operator | Assign policy to target |
| DELETE | `/api/v1/policy-assignments/{assignment_id}` | admin, operator | Remove assignment |
| GET | `/api/v1/devices/{id}/effective-policies` | any authenticated | Get device effective policies |

### Compliance (Sprint 4)
| Method | Path | Role | Description |
|--------|------|------|-------------|
| GET | `/api/v1/compliance` | any authenticated | Enterprise compliance summary |
| GET | `/api/v1/devices/{id}/compliance` | any authenticated | Device compliance |
| POST | `/api/v1/devices/{id}/compliance/evaluate` | admin, operator | Trigger compliance evaluation |

## Database Tables

| Table | Purpose |
|-------|---------|
| `enterprises` | Organizations/tenants |
| `users` | Admin users (Keycloak-managed) |
| `devices` | Enrolled devices |
| `policies` | Management policies |
| `device_policies` | Device-policy assignments (Sprint 1/2) |
| `certificates` | PKI certificates |
| `api_tokens` | API tokens for programmatic access |
| `device_commands` | Device command queue |
| `apps` | Application catalog |
| `device_apps` | App installation state per device |
| `audit_logs` | Audit trail |
| `dep_names` | DEP server configurations (encrypted tokens) |
| `dep_devices` | DEP-synced device inventory |
| `device_groups` | Static device groups |
| `group_memberships` | Device-to-group mapping |
| `policy_assignments` | Policy targeting (device/group/enterprise) |
| `compliance_results` | Per-device compliance state |
| `policy_versions` | Policy version snapshots |
| `token_cache` | PostgreSQL-backed token cache |
| `idempotency_keys` | Idempotency-Key response cache |

## Development Workflow

1. **Start session**
   ```bash
   cd ~/Documents/GitRepos/Malcolm-GetAHead/local-mdm
   make docker-up && sleep 45 && make migrate-up
   make run
   ```

2. **Make changes** — edit code, run tests frequently

3. **Test changes**
   ```bash
   go test -race ./...
   go vet ./...
   ```

4. **Commit** — one commit per logical unit, reference task IDs

5. **End session**
   ```bash
   Ctrl+C          # Stop server
   make docker-down  # Stop services
   ```

## Current Phase

**Sprint 4b complete** — Read/Write Database Pools. DB struct split into Writer/Reader pools, all repository constructors updated, ReaderConfig with field-level fallback. Ready for Sprint 4c (macOS Platform SSO) or Sprint 5 (Backend Polish).

See [Sprint Planning](../planning/sprints/) for roadmap.

---
