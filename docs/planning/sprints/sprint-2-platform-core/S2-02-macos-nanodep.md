# S2-02: macOS — NanoDEP Integration

**Sprint**: 2 — Platform Core
**Parallel**: ✅ Can start immediately after Sprint 1
**Effort**: 3-4 days

## Objective

Integrate NanoDEP for Apple DEP/ADE automated device enrollment. Devices assigned in ABM/ASM can auto-enroll.

## Tasks

### 1. NanoDEP Library Integration
- Import `github.com/micromdm/nanodep` as Go dependency
- Configure with PostgreSQL storage backend (dep_names table from S1-01 migration)
- Wire depserver API handlers or embed as library
- Files: `internal/platform/macos/nanodep.go`

### 2. DEP Token Management API
- Expose endpoints for DEP token PKI exchange
- Generate keypair, return certificate for ABM/ASM upload
- Accept encrypted token file, decrypt and store OAuth tokens
- Files: `internal/platform/macos/dep_tokens.go`

### 3. DEP Profile Management
- Define DEP profiles (JSON) with MDM server URL
- Assign profiles to serial numbers
- Query device details from Apple
- Files: `internal/platform/macos/dep_profiles.go`

### 4. Device Syncer
- Run depsyncer logic (embedded or as goroutine)
- Sync devices from Apple DEP on configurable interval
- Auto-assign DEP profile to newly synced devices
- Webhook or internal event on new device sync
- Files: `internal/platform/macos/dep_sync.go`

### 5. Routes
- `GET/PUT /api/v1/dep/{name}/tokenpki` — token PKI exchange
- `PUT /api/v1/dep/{name}/tokens` — upload decrypted tokens
- `GET/PUT /api/v1/dep/{name}/profile` — define/get DEP profile
- `POST /api/v1/dep/{name}/assign` — assign profile to serials
- `GET /api/v1/dep/{name}/devices` — list synced devices

## Key Reference Docs
- [NanoDEP Operations Guide](../../dependencies/nanodep/operations-guide.md)
- [NanoDEP OpenAPI Spec](../../dependencies/nanodep/openapi.yaml)
- [NanoDEP PostgreSQL Schema](../../dependencies/nanodep/schema-pgsql.sql)

## Acceptance Criteria

- [x] DEP token PKI exchange completes successfully
- [x] DEP profile can be defined and assigned to serial numbers
- [x] Device syncer fetches devices from Apple (or depsim for testing)
- [x] Auto-assigner assigns profile to newly synced devices
- [x] Synced devices visible via API

## Implementation Notes (2026-04-17)

Actual file locations:
- `internal/platform/macos/dep_storage.go` — Encrypted DEP token storage (pgcrypto)
- `internal/platform/macos/dep_service.go` — DEP service (profiles, sync, assignment)
- `internal/platform/macos/dep_test.go` — Unit tests with mock storage
- `migrations/000004_dep_names.up.sql` — dep_names + dep_devices tables

API endpoints:
- `GET/PUT /api/v1/dep/{name}/tokenpki` — Token PKI exchange
- `GET/PUT /api/v1/dep/{name}/assigner` — Assigner profile management
- `GET /api/v1/dep/{name}/devices` — List synced devices

OAuth tokens encrypted at rest with pgp_sym_encrypt. Encryption key from config
(DEP_ENCRYPTION_KEY env var, secrets/dep_encryption.key for dev).
