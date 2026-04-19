# Sprint 2a: Gap Closure — Pre-Sprint 3 Cleanup

**Duration**: 2-3 days  
**Goal**: Close gaps between Sprint 2 deliverables and Sprint 3 dependencies  
**Priority**: Must complete before starting Sprint 3

---

## Why This Sprint Exists

Sprint 2 delivered the platform enrollment flows, OMA-DM sync, DEP integration, and wired up all CRUD handlers. However, several pieces are partially connected — repositories exist without routes, platform services exist without being wired into the server, and stub handlers need to be replaced with real implementations. Sprint 3 (Commands, Profiles & Apps) depends on these being solid.

---

## S2a-01: Add Missing CRUD API Endpoints

**Effort**: 0.5 days  
**Blocks**: S3-01 (macOS profiles need policy assignment), S3-04 (remote actions need device updates), S4-02 (policy groups)

### Problem

The repository layer has full Update and Delete methods for enterprises, devices, and policies, but no API routes expose them. The PolicyRepository has `AssignToDevice` and `UnassignFromDevice` methods but no endpoints. Sprint 3 needs these — you can't deploy a policy to a device if there's no way to assign it.

### What to Add

#### Enterprise Update and Delete
```
PUT    /api/v1/enterprises/{id}     → handleUpdateEnterprise
DELETE /api/v1/enterprises/{id}     → handleDeleteEnterprise
```

**Where to look**:
- `internal/repository/enterprise.go` — `Update(ctx, enterprise)` updates name and settings; `Delete(ctx, id)` soft-deletes
- `internal/api/handlers.go` — add handlers following the same pattern as `handleCreateEnterprise`
- `internal/api/server.go` `setupRoutes()` — add routes with appropriate auth (super_admin for delete, admin for update)

**Example handler**:
```go
func (s *Server) handleUpdateEnterprise(w http.ResponseWriter, r *http.Request) {
    id, err := parseUUIDParam(r, "id")
    // ... parse JSON body with name/settings ...
    // ... call s.enterpriseRepo.Update(ctx, enterprise) ...
    // ... logAudit "enterprise.update" ...
}
```

#### Device Update and Delete
```
PUT    /api/v1/devices/{id}         → handleUpdateDevice
DELETE /api/v1/devices/{id}         → handleDeleteDevice
```

**Where to look**:
- `internal/repository/device.go` — `Update(ctx, device)` updates name, model, os_version, last_seen, status, platform_data; `Delete(ctx, id)` soft-deletes
- Roles: admin/operator for update, admin for delete

**Why it matters**: Sprint 3 remote actions (S3-04) will need to update device status after lock/wipe. The current lock/wipe handlers already do this, but there's no general-purpose device update endpoint for things like renaming a device or updating its metadata.

#### Policy Update, Delete, Assign, Unassign
```
PUT    /api/v1/policies/{id}                        → handleUpdatePolicy
DELETE /api/v1/policies/{id}                        → handleDeletePolicy
POST   /api/v1/policies/{id}/assign                 → handleAssignPolicy
DELETE /api/v1/policies/{id}/assign/{device_id}     → handleUnassignPolicy
```

**Where to look**:
- `internal/repository/policy.go` — `Update` changes name, description, policy_config, is_active; `Delete` soft-deletes; `AssignToDevice` inserts into device_policies with ON CONFLICT DO NOTHING; `UnassignFromDevice` hard-deletes from device_policies
- The `device_policies` junction table (migration 000001) tracks assignment status: pending, applied, failed, removed

**Why it matters**: This is the most critical gap. Sprint 3 is entirely about deploying policies/profiles to devices. Without assign/unassign endpoints, there's no way to target a policy at a device through the API. S3-01 (macOS profiles), S3-02 (Windows CSPs), and S3-03 (Android policies) all need this.

**Example assign handler**:
```go
func (s *Server) handleAssignPolicy(w http.ResponseWriter, r *http.Request) {
    policyID, _ := parseUUIDParam(r, "id")
    var req struct {
        DeviceIDs []uuid.UUID `json:"device_ids"`
    }
    parseJSONBody(r, &req)
    for _, deviceID := range req.DeviceIDs {
        s.policyRepo.AssignToDevice(ctx, deviceID, policyID)
    }
    // audit log, respond
}
```

### Tests to Add
- Update/delete for each resource type (success, not found, validation)
- Policy assign/unassign (success, duplicate assignment idempotent, not found)
- Auth/role checks on each new endpoint

---

## S2a-02: Wire Platform Services into Server Struct

**Effort**: 0.5 days  
**Blocks**: S3-01 (macOS commands), S3-02 (Windows CSP delivery), S3-03 (Android policy enforcement)

### Problem

Each platform has a `Service` struct with `CreateDevice` and `UpdateDeviceStatus` methods, but none are stored on the `Server` struct or initialized in the constructor. They exist as standalone code that nothing calls from the API layer.

Additionally, the `ManagementHandler` in `handleWindowsManagementSync` is created fresh on every request instead of being reused. This works but is wasteful and prevents the handler from maintaining any state (like caching device lookups).

### What to Wire Up

#### macOS Service
**Where**: `internal/platform/macos/service.go` — `NewService(deviceRepo)` returns `*Service`  
**Add to Server struct**: `macosService *macos.Service`  
**Initialize in constructor**: `s.macosService = macos.NewService(s.deviceRepo)`  
**Used by**: macOS enrollment (create device record after profile download), NanoMDM webhook handlers (update device status on checkin events)

#### Windows Service + ManagementHandler
**Where**: `internal/platform/windows/service.go` — `NewService(deviceRepo)` returns `*Service`  
**Where**: `internal/platform/windows/management.go` — `NewManagementHandler(serverURI, deviceRepo, cmdRepo, logger)` returns `*ManagementHandler`  
**Add to Server struct**: `windowsService *windows.Service`, `windowsMgmtHandler *windows.ManagementHandler`  
**Initialize in constructor**: Create once, reuse across requests  
**Fix in `handleWindowsManagementSync`**: Replace per-request handler creation with `s.windowsMgmtHandler.HandleSyncML()`

Current code (wasteful):
```go
func (s *Server) handleWindowsManagementSync(w http.ResponseWriter, r *http.Request) {
    serverURI := fmt.Sprintf("https://%s/ManagementServer/MDM.svc", r.Host)
    handler := windows.NewManagementHandler(serverURI, s.deviceRepo, s.cmdRepo, s.logger) // created every request
    resp, err := handler.HandleSyncML(r.Context(), body)
```

Should be:
```go
// In constructor:
s.windowsMgmtHandler = windows.NewManagementHandler(serverURI, s.deviceRepo, s.cmdRepo, s.logger)

// In handler:
func (s *Server) handleWindowsManagementSync(w http.ResponseWriter, r *http.Request) {
    resp, err := s.windowsMgmtHandler.HandleSyncML(r.Context(), body)
```

#### Android Service + Client
**Where**: `internal/platform/android/service.go` — `NewService(deviceRepo, enterpriseRepo, projectID, serviceAccount)`  
**Where**: `internal/platform/android/client.go` — `NewClient(ctx, projectID, serviceAccountJSON, logger)` — requires Google credentials  
**Add to Server struct**: `androidService *android.Service`  
**Initialize in constructor**: Service always; Client only if `config.Android.ServiceAccountJSON` is configured (graceful degradation like CertificateService)

**Why it matters**: Sprint 3 (S3-03 Android policies) needs the Android Client to call Google Management API for policy deployment. Without it wired up, there's no way to push policies to Android devices.

### Tests to Add
- Verify platform services are non-nil after server construction
- Verify ManagementHandler is reused (not a test per se, but code review item)

---

## S2a-03: Replace NanoMDM Stubs with Real Handlers

**Effort**: 0.5 days  
**Blocks**: S3-01 (macOS MDM commands depend on checkin/command flow working)

### Problem

Two routes in `server.go` are inline stubs:

```go
s.router.HandleFunc("/mdm", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)  // does nothing
})).Methods("PUT")
s.router.HandleFunc("/checkin", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)  // does nothing
})).Methods("PUT")
```

Meanwhile, `internal/platform/macos/webhook.go` has `CheckinHandler` and `CommandHandler` types with `ServeHTTP` methods that are ready to be used — they just aren't wired in. And `NanoMDMService` in `nanomdm_service.go` has `HandleCommand` and `HandleCheckin` methods that are placeholders but have the right signatures.

### What to Do

1. **Initialize NanoMDMService** in the server constructor:
   ```go
   nanomdmSvc, err := macos.NewNanoMDMService(database.DB, logger)
   ```

2. **Create CheckinHandler and CommandHandler** using the initialized services:
   ```go
   checkinHandler := macos.NewCheckinHandler(nanomdmSvc, s.macosService, logger)
   commandHandler := macos.NewCommandHandler(nanomdmSvc, logger)
   ```

3. **Replace the inline stubs** in `setupRoutes()`:
   ```go
   s.router.Handle("/mdm", commandHandler).Methods("PUT")
   s.router.Handle("/checkin", checkinHandler).Methods("PUT")
   ```

4. **Flesh out the handler logic** — at minimum:
   - `CheckinHandler.ServeHTTP`: Parse the plist body, extract UDID and MessageType, call `NanoMDMService.HandleCheckin`, create/update device record via `macosService.CreateDevice` on Authenticate events
   - `CommandHandler.ServeHTTP`: Parse the plist body, extract command response, call `NanoMDMService.HandleCommand`

**Where to look**:
- `internal/platform/macos/webhook.go` lines 97-140 — `CheckinHandler` and `CommandHandler` structs
- `internal/platform/macos/nanomdm_service.go` — `HandleCommand` and `HandleCheckin` (currently just log)
- Apple MDM protocol: devices PUT to `/checkin` on enrollment events (Authenticate, TokenUpdate, CheckOut) and PUT to `/mdm` to report command results

**Why it matters**: The macOS enrollment profile we generate (in `enrollment.go`) tells devices to check in at `{serverURL}/checkin` and send command responses to `{serverURL}/mdm`. If these return 200 with no logic, the device thinks everything is fine but nothing actually happens. Sprint 3 (S3-01) needs the command flow working to send InstallProfile, DeviceInformation, etc.

### Tests to Add
- CheckinHandler receives Authenticate plist → creates device record
- CheckinHandler receives TokenUpdate → updates device
- CommandHandler receives command response → logs/processes
- Invalid plist body → returns error

---

## S2a-04: Wire DEP Sync Loop

**Effort**: 0.25 days  
**Blocks**: S2-02 acceptance criteria (device syncer fetches devices), DEP auto-assignment

### Problem

The DEP service has `SyncDevicesCallbackForName` which stores synced devices, and the nanoDEP library has a `sync.Syncer` that handles the fetch/sync cursor loop. But nothing starts the sync loop. The DEP service is initialized in the server constructor, but no background goroutine runs the periodic sync.

### What to Do

1. **Add a `StartDEPSync` method** to `DEPService` or create a wrapper that:
   - Creates a `godep.Client` using the stored auth tokens
   - Creates a `sync.Syncer` with the client, cursor storage, and callback
   - Creates a `sync.Assigner` with the client and assigner profile storage
   - Runs the syncer on the configured interval (`config.MacOS.DEPSyncInterval`, default 30 minutes)

2. **Start the sync in `Server.Start()`** (similar to how certMonitor is started):
   ```go
   if s.depService != nil {
       s.depSyncer = s.depService.StartSync(depName, syncInterval)
   }
   ```

3. **Stop the sync in `Server.Shutdown()`**

**Where to look**:
- `github.com/micromdm/nanodep/sync` — `Syncer` struct, `NewSyncer`, `Run` method
- `github.com/micromdm/nanodep/godep` — `NewClient` needs `ClientStorage` (our `DEPStorage` implements the required interfaces)
- `internal/platform/macos/dep_service.go` — add StartSync method
- `internal/platform/macos/dep_storage.go` — already implements `RetrieveAuthTokens` and `RetrieveConfig` which `godep.Client` needs

**Note**: The sync loop will fail without valid Apple DEP tokens (which require ABM access). For now, it should start gracefully and log errors when tokens aren't configured, rather than crashing. This is the same pattern as the cert monitor — start if configured, skip if not.

### Tests to Add
- Sync callback stores devices correctly (already tested)
- Sync loop starts and stops cleanly (integration test with mock Apple server, or just verify goroutine lifecycle)

---

## S2a-05: Clean Up Disconnected Code

**Effort**: 0.25 days  
**Blocks**: Nothing directly, but reduces confusion for Sprint 3 developers

### Items

#### 1. Remove .bak files in repository/
**Where**: `internal/repository/*.bak` — 9 backup files from iterative development  
**Action**: Delete them. They're noise.

#### 2. Remove committed binaries
**Where**: `server` (14MB) and `bin/local-mdm` (9.8MB) in repo root  
**Action**: Delete and add to `.gitignore`

#### 3. Feature flags not checked
**Where**: `internal/config/config.go` — `FeaturesConfig` has `EnableAuditLog`, `EnableWebhooks`, `EnableMetrics`  
**Problem**: `EnableAuditLog` is never checked (audit logger always initializes). `EnableWebhooks` is never checked. `EnableMetrics` exists but the server uses `config.Metrics.Enabled` instead.  
**Action**: Either wire the flags up or remove the duplicates. Recommendation: remove `EnableMetrics` from FeaturesConfig (redundant with MetricsConfig.Enabled), wire `EnableAuditLog` to conditionally initialize the async logger, leave `EnableWebhooks` for Sprint 3.

#### 4. SQL safety columns unused
**Where**: `internal/repository/sql_safety.go` — `DeviceOrderColumns`, `EnterpriseOrderColumns`, `PolicyOrderColumns` defined but never used  
**Problem**: All List queries hardcode `ORDER BY created_at DESC`  
**Action**: Either add `sort` query parameter support to list endpoints (using `ValidateOrderColumn`) or leave as-is with a TODO for Sprint 5 (API polish). The whitelists are correct and ready to use.

#### 5. Android webhook handlers are logging stubs
**Where**: `internal/platform/android/webhook.go` — `handleEnrollment`, `handleComplianceReport`, `handleStatusReport`, `handleUnenrollment`  
**Problem**: These log but don't create/update device records  
**Action**: Wire `handleEnrollment` to call `service.CreateDevice` and `handleUnenrollment` to call `service.UpdateDeviceStatus(ctx, id, "unenrolled")`. The others can remain logging stubs until Sprint 3 (S3-03).

---

## Summary

| Task | Effort | Sprint 3 Dependency |
|------|--------|-------------------|
| S2a-01: Missing CRUD endpoints | 0.5 days | S3-01, S3-02, S3-03, S3-04, S4-02 |
| S2a-02: Wire platform services | 0.5 days | S3-01, S3-02, S3-03 |
| S2a-03: NanoMDM real handlers | 0.5 days | S3-01 |
| S2a-04: DEP sync loop | 0.25 days | S2-02 completion |
| S2a-05: Clean up disconnected code | 0.25 days | Code quality |
| **Total** | **2 days** | |

## Definition of Done

- [ ] All repository CRUD methods have corresponding API endpoints
- [ ] Policy assign/unassign endpoints work
- [ ] Platform services stored on Server struct, initialized in constructor
- [ ] ManagementHandler created once, reused across requests
- [ ] `/mdm` and `/checkin` routes use real handlers (not inline stubs)
- [ ] DEP sync loop starts when configured, stops on shutdown
- [ ] .bak files and committed binaries removed
- [ ] Feature flags either wired or cleaned up
- [ ] All existing tests still pass
- [ ] New endpoints have tests

---

*Created: 2026-04-18*  
*Sprint: 2a — Gap Closure*
