# S2-04: Windows — OMA-DM Sync & DeviceInfo CSP

**Sprint**: 2 — Platform Core
**Parallel**: ⛔ Sequential — requires S2-03 (enrollment must work first)
**Effort**: 4-5 days

## Objective

Handle OMA-DM SyncML sessions from enrolled Windows devices. Collect device inventory via DeviceInfo CSP.

## Tasks

### 1. SyncML Parser/Generator
- Parse SyncML XML messages (SyncHdr, SyncBody, Alert, Status, Results, Replace, Add, Get)
- Generate SyncML response messages
- Handle session state (SessionID, MsgID, CmdID sequencing)
- Files: `internal/platform/windows/protocol/syncml.go`

### 2. OMA-DM Session Handler
- `POST /ManagementServer/MDM.svc` endpoint
- Session initialization (pkg 1/2 exchange)
- Client authentication via certificate
- Command queue: dequeue pending commands, send to device
- Process device responses (Status, Results)
- Files: `internal/platform/windows/management.go`, `internal/platform/windows/protocol/session.go`

### 3. DeviceInfo CSP
- Query device information nodes:
  - `./DevDetail/Ext/Microsoft/DeviceName`
  - `./DevDetail/Ext/Microsoft/OSPlatform`
  - `./DevDetail/Ext/Microsoft/ProcessorArchitecture`
  - `./DevDetail/Ext/Microsoft/TotalRAM`
  - `./DevDetail/Ext/Microsoft/TotalStorage`
  - `./DevDetail/FwV` (firmware version)
  - `./DevDetail/HwV` (hardware version)
  - `./DevDetail/SwV` (software version)
- Parse results and update device record
- Files: `internal/platform/windows/csp/deviceinfo.go`

### 4. Command Queue
- Store pending commands per device in database
- Dequeue on next sync session
- Track command status (pending, sent, acknowledged, error)
- Files: `internal/platform/windows/command_queue.go`

### 5. Routes
- `POST /ManagementServer/MDM.svc` — OMA-DM sync endpoint

## Acceptance Criteria

- [x] Enrolled Windows device initiates OMA-DM sync session
- [x] Server correctly handles SyncML pkg 1/2 exchange
- [x] DeviceInfo CSP queries return device inventory
- [x] Device record updated with hardware/software info
- [x] Command queue delivers pending commands during sync
- [x] Command responses processed and status updated

## Implementation Notes (2026-04-17)

Actual file locations differ from plan:
- `internal/platform/windows/syncml.go` — SyncML XML types, parser, generator
- `internal/platform/windows/management.go` — Session handler, DevDetail CSP, command delivery
- `internal/repository/command.go` — Command queue repository
- `migrations/000003_device_commands.up.sql` — device_commands table
- Route: `POST /ManagementServer/MDM.svc`
