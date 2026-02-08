# S2-05: Android — Management API Client & Enrollment

**Sprint**: 2 — Platform Core
**Parallel**: ✅ Can start immediately after Sprint 1
**Effort**: 4-5 days

## Objective

Integrate with Google Android Management API for device enrollment via QR code and token.

## Tasks

### 1. Google API Client
- Service account authentication (JSON key file)
- Android Management API v1 client wrapper
- Enterprise creation/binding
- Error handling and retry logic
- Files: `internal/platform/android/client.go`, `internal/platform/android/auth.go`

### 2. Enterprise Setup
- Create enterprise via signup URL flow
- Store enterprise binding (Google enterprise name ↔ Local MDM enterprise_id)
- Files: `internal/platform/android/enterprise.go`

### 3. Enrollment Token Generation
- Create enrollment tokens (QR code and NFC)
- Generate QR code image from token
- Token expiration handling
- Support: fully managed, work profile, dedicated device
- Files: `internal/platform/android/enrollment.go`, `internal/platform/android/qr.go`

### 4. Webhook Handler
- Pub/Sub or pull-based status reports
- Process device state changes (enrollment, policy compliance, etc.)
- Create/update device records in DeviceRepository
- Verify webhook signatures for security
- Files: `internal/platform/android/webhook.go`, `internal/platform/android/events.go`

### 5. Polling Reconciliation
- Background job runs every 15 minutes
- Polls Android Management API for all device statuses
- Reconciles differences (in case webhook was missed or service was offline)
- Updates device records with latest state
- Files: `internal/platform/android/poller.go`

**Reconciliation Strategy**:
- Webhooks provide real-time updates (primary method)
- Polling catches missed webhooks (backup method)
- Compare `last_status_report_time` to detect drift
- Log discrepancies for monitoring and alerting

### 6. Routes
- `POST /api/v1/android/enterprise` — bind enterprise
- `POST /api/v1/android/enrollment-token` — generate enrollment token
- `GET /api/v1/android/enrollment-token/{id}/qr` — QR code image
- `POST /api/v1/android/webhook` — Google callback endpoint

## Acceptance Criteria

- [ ] Enterprise bound to Google Android Management
- [ ] Enrollment token generated with QR code
- [ ] Android device scans QR code and enrolls
- [ ] Webhook receives enrollment event
- [ ] Device appears in `GET /api/v1/devices` with platform=android
