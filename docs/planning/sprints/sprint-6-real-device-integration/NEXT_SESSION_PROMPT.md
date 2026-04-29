# Sprint 6 — Next Session Context

## Current State (2026-04-28)

**Both platforms enrolled and syncing:**
- macOS VM: Full data pipeline (9 auto-queued commands, 35+ fields, compliance evaluation)
- Windows VM: Enrolled via Settings UI, OMA-DM sync working, enterprise-scoped, compliance policies applied

## What's Working
- Windows enrollment: Settings → Access work or school → "Enroll only in device management" → `admin@localmdm.local`
- Windows OMA-DM sync: Device checks in on schedule, last_seen updates
- macOS webhook pipeline: Authenticate → TokenUpdate → auto-queue → command results → platform_data → compliance
- Enterprise assignment: Email local part as UUID, or default to Acme Corp
- CRL distribution point on server cert for Windows schannel compatibility

## What Needs Work Next

### Windows OMA-DM Device Info Queries (HIGH PRIORITY)
The OMA-DM sync handler acknowledges sessions but doesn't query device state. Need to add CSP queries to the sync response:
- `./Device/Vendor/MSFT/BitLocker/Status/DeviceEncryptionStatus` → encryption compliance
- `./Device/Vendor/MSFT/Firewall/MdmStore/Global/EnableFirewall` → firewall compliance
- `./DevDetail/SwV` → OS version
- `./DevInfo/DevName` → device name
- `./DevDetail/Ext/Microsoft/DeviceName` → friendly name

Results should update `platform_data` JSONB, triggering compliance re-evaluation via EventBus.

### Remaining GAPS.md Items
- [ ] Integration tests for full webhook flow
- [ ] Add nginx to `make prod-up` target
- [ ] VM setup guide documentation
- [ ] Fix command status `pending` → `sent` → `completed` transitions

### VM Access
- Windows VM: `ssh testuser@192.168.65.2` (password: testuser)
- macOS VM: `ssh testuser@192.168.64.4` (password: testuser)
- MDM server: `http://192.168.1.229:8080` / `https://192.168.1.229:8443`

### Key Lesson: `make dev-test` Destroys Real Device Data
Tests share the same database. Run tests BEFORE enrolling real devices, or use a separate test database.
