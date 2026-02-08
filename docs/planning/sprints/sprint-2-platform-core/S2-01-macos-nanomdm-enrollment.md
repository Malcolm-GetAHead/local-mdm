# S2-01: macOS — NanoMDM Integration & Enrollment

**Sprint**: 2 — Platform Core
**Parallel**: ✅ Can start immediately after Sprint 1
**Effort**: 4-5 days

## Objective

Integrate NanoMDM as a Go library for Apple MDM protocol handling. Devices can enroll and check in.

## Tasks

### 1. NanoMDM Library Integration
- Import nanoMDM packages as Go library dependencies:
  - `github.com/micromdm/nanomdm/service/nanomdm` - Core MDM service
  - `github.com/micromdm/nanomdm/storage/pgsql` - PostgreSQL storage backend
  - `github.com/micromdm/nanomdm/http/mdm` - HTTP handlers for MDM protocol
  - `github.com/micromdm/nanomdm/push/nanopush` - APNs push service
- Configure with PostgreSQL storage backend (shared DB, nanomdm tables from S1-01 migration)
- Wire NanoMDM HTTP handlers into our router
- Files: `internal/platform/macos/nanomdm_service.go`

**Integration Approach**:
```go
// Use nanoMDM as library, not standalone server
import (
    nanomdmsvc "github.com/micromdm/nanomdm/service/nanomdm"
    "github.com/micromdm/nanomdm/storage/pgsql"
    nanomdmhttp "github.com/micromdm/nanomdm/http/mdm"
)

// Create nanoMDM service with PostgreSQL storage
storage := pgsql.New(db)
mdmService := nanomdmsvc.New(storage, logger)

// Wrap nanoMDM HTTP handlers
mdmHandler := nanomdmhttp.CheckinAndCommandHandler(mdmService, logger)
router.Handle("/mdm", mdmHandler)
```

### 2. Enrollment Profile Generation
- Generate `.mobileconfig` enrollment profiles
- Embed SCEP payload (URL, challenge)
- Embed MDM payload (ServerURL, Topic, CheckInURL)
- Sign profile with server certificate
- Serve profile at download endpoint
- Files: `internal/platform/macos/enrollment.go`, `internal/platform/macos/profile.go`

### 3. APNs Push Integration
- Load APNs push certificate from storage (S1-03)
- Send push notifications to enrolled devices
- Handle push errors and feedback
- Files: `internal/platform/macos/apns.go`

### 4. Webhook Handler
- Receive NanoMDM webhook callbacks (Authenticate, TokenUpdate, CheckOut, Connect)
- On Authenticate: create/update device in DeviceRepository
- On CheckOut: mark device as unenrolled
- On Connect: update last_seen_at, process command responses
- Files: `internal/platform/macos/webhook.go`

### 5. Routes
- `PUT /mdm` — NanoMDM MDM endpoint
- `PUT /checkin` — NanoMDM check-in endpoint (if separate)
- `GET /api/v1/macos/enroll/{enterprise_id}` — download enrollment profile
- `PUT /api/v1/macos/pushcert` — upload APNs push certificate

## Key Reference Docs
- [NanoMDM Operations Guide](../../dependencies/nanomdm/operations-guide.md)
- [NanoMDM PostgreSQL Schema](../../dependencies/nanomdm/schema-pgsql.sql)
- [SCEP README](../../dependencies/scep/README.md)

## Acceptance Criteria

- [ ] macOS device installs enrollment profile and completes MDM enrollment
- [ ] Authenticate and TokenUpdate webhook events create device record
- [ ] APNs push triggers device check-in
- [ ] CheckOut event marks device unenrolled
- [ ] Device appears in `GET /api/v1/devices` with platform=macos
