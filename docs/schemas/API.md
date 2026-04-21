# API Documentation

**Version**: 1.0  
**Base URL**: `https://your-mdm-server.com/api/v1`  
**Last Updated**: 2026-04-20

## Overview

The Local MDM API is a RESTful API that provides unified device management across Windows, macOS, and Android platforms.

## Authentication

All API requests require authentication using JWT tokens or API keys.

### JWT Authentication

```http
Authorization: Bearer <jwt_token>
```

### API Key Authentication

```http
X-API-Key: <api_key>
```

### Login

```http
POST /api/v1/auth/login
Content-Type: application/json

{
  "email": "admin@example.com",
  "password": "password"
}
```

**Response**:
```json
{
  "access_token": "eyJhbGc...",
  "refresh_token": "eyJhbGc...",
  "expires_in": 3600,
  "token_type": "Bearer"
}
```

### Refresh Token

```http
POST /api/v1/auth/refresh
Content-Type: application/json

{
  "refresh_token": "eyJhbGc..."
}
```

## Response Format

### Success Response

```json
{
  "data": { ... },
  "meta": {
    "timestamp": "2026-02-05T10:30:00Z"
  }
}
```

### Error Response

```json
{
  "error": {
    "code": "DEVICE_NOT_FOUND",
    "message": "Device with ID abc123 not found",
    "details": {}
  },
  "meta": {
    "timestamp": "2026-02-05T10:30:00Z"
  }
}
```

### Pagination

List endpoints support pagination:

```http
GET /api/v1/devices?page=1&per_page=50
```

**Response**:
```json
{
  "data": [...],
  "meta": {
    "page": 1,
    "per_page": 50,
    "total": 150,
    "total_pages": 3
  }
}
```

## Endpoints

### Health Check

#### Get System Health

```http
GET /health
```

**Response**:
```json
{
  "status": "healthy",
  "version": "1.0.0",
  "database": "connected",
  "timestamp": "2026-02-05T10:30:00Z"
}
```

---

## Enterprises

### List Enterprises

```http
GET /api/v1/enterprises
```

**Auth**: admin, super_admin

**Query Parameters**:
- `limit` (optional): Items per page (default: 100)
- `offset` (optional): Offset for pagination

### Get Enterprise

```http
GET /api/v1/enterprises/:id
```

### Create Enterprise

```http
POST /api/v1/enterprises
Content-Type: application/json

{
  "name": "Acme Corp",
  "slug": "acme",
  "settings": {}
}
```

**Auth**: super_admin + IP allowlist

### Update Enterprise

```http
PUT /api/v1/enterprises/:id
Content-Type: application/json

{
  "name": "Acme Corporation",
  "settings": {"timezone": "US/Eastern"}
}
```

**Auth**: admin, super_admin. Fields are optional — only provided fields are updated.

### Delete Enterprise

```http
DELETE /api/v1/enterprises/:id
```

**Auth**: super_admin + IP allowlist. Soft-deletes the enterprise.

**Response**: `204 No Content`

---

## Devices

### List Devices

```http
GET /api/v1/devices
```

**Query Parameters**:
- `platform` (optional): Filter by platform (windows, macos, android)
- `status` (optional): Filter by status (enrolled, pending, unenrolled)
- `page` (optional): Page number (default: 1)
- `per_page` (optional): Items per page (default: 50, max: 100)

**Response**:
```json
{
  "data": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "enterprise_id": "660e8400-e29b-41d4-a716-446655440000",
      "platform": "windows",
      "device_id": "WIN-ABC123",
      "serial_number": "SN123456",
      "name": "John's Laptop",
      "model": "ThinkPad X1 Carbon",
      "os_version": "Windows 11 Pro 23H2",
      "enrollment_date": "2026-02-01T10:00:00Z",
      "last_seen": "2026-02-05T10:25:00Z",
      "status": "enrolled",
      "created_at": "2026-02-01T10:00:00Z",
      "updated_at": "2026-02-05T10:25:00Z"
    }
  ],
  "meta": {
    "page": 1,
    "per_page": 50,
    "total": 1,
    "total_pages": 1
  }
}
```

### Get Device

```http
GET /api/v1/devices/:id
```

**Response**:
```json
{
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "enterprise_id": "660e8400-e29b-41d4-a716-446655440000",
    "platform": "windows",
    "device_id": "WIN-ABC123",
    "serial_number": "SN123456",
    "name": "John's Laptop",
    "model": "ThinkPad X1 Carbon",
    "os_version": "Windows 11 Pro 23H2",
    "enrollment_date": "2026-02-01T10:00:00Z",
    "last_seen": "2026-02-05T10:25:00Z",
    "status": "enrolled",
    "platform_data": {
      "windows_edition": "Pro",
      "tpm_version": "2.0",
      "bitlocker_enabled": true
    },
    "policies": [
      {
        "id": "770e8400-e29b-41d4-a716-446655440000",
        "name": "Corporate WiFi",
        "status": "applied"
      }
    ],
    "created_at": "2026-02-01T10:00:00Z",
    "updated_at": "2026-02-05T10:25:00Z"
  }
}
```

### Create Enrollment Token

```http
POST /api/v1/devices/enroll
Content-Type: application/json

{
  "platform": "windows",
  "name": "New Device",
  "metadata": {}
}
```

**Response**:
```json
{
  "data": {
    "enrollment_token": "eyJhbGc...",
    "enrollment_url": "https://mdm.example.com/enroll?token=...",
    "qr_code": "data:image/png;base64,...",
    "expires_at": "2026-02-06T10:30:00Z"
  }
}
```

### Update Device

```http
PUT /api/v1/devices/:id
Content-Type: application/json

{
  "name": "Updated Laptop Name",
  "model": "ThinkPad X1 Carbon Gen 11",
  "os_version": "Windows 11 Pro 24H2",
  "status": "enrolled",
  "platform_data": {}
}
```

**Auth**: admin, operator. Fields are optional — only provided fields are updated.

### Delete Device

```http
DELETE /api/v1/devices/:id
```

**Auth**: admin. Soft-deletes the device.

**Response**: `204 No Content`

### Lock Device

```http
POST /api/v1/devices/:id/lock
Content-Type: application/json

{
  "message": "This device has been locked by IT",
  "pin": "1234"
}
```

**Response**:
```json
{
  "data": {
    "command_id": "880e8400-e29b-41d4-a716-446655440000",
    "status": "pending",
    "created_at": "2026-02-05T10:30:00Z"
  }
}
```

### Wipe Device

```http
POST /api/v1/devices/:id/wipe
Content-Type: application/json

{
  "wipe_type": "full",
  "confirmation": "WIPE"
}
```

**Wipe Types**:
- `full`: Complete device wipe
- `enterprise`: Remove only enterprise data (Android work profile)

**Response**:
```json
{
  "data": {
    "command_id": "990e8400-e29b-41d4-a716-446655440000",
    "status": "pending",
    "created_at": "2026-02-05T10:30:00Z"
  }
}
```

---

## Policies

### List Policies

```http
GET /api/v1/policies
```

**Query Parameters**:
- `platform` (optional): Filter by platform
- `policy_type` (optional): Filter by type
- `is_active` (optional): Filter by active status

**Response**:
```json
{
  "data": [
    {
      "id": "770e8400-e29b-41d4-a716-446655440000",
      "enterprise_id": "660e8400-e29b-41d4-a716-446655440000",
      "name": "Corporate WiFi",
      "description": "Main office WiFi configuration",
      "platform": "all",
      "policy_type": "wifi",
      "is_active": true,
      "device_count": 45,
      "created_at": "2026-02-01T09:00:00Z",
      "updated_at": "2026-02-01T09:00:00Z"
    }
  ]
}
```

### Get Policy

```http
GET /api/v1/policies/:id
```

**Response**:
```json
{
  "data": {
    "id": "770e8400-e29b-41d4-a716-446655440000",
    "enterprise_id": "660e8400-e29b-41d4-a716-446655440000",
    "name": "Corporate WiFi",
    "description": "Main office WiFi configuration",
    "platform": "all",
    "policy_type": "wifi",
    "policy_config": {
      "ssid": "CorpWiFi",
      "security_type": "WPA2-Enterprise",
      "eap_type": "PEAP",
      "auto_join": true
    },
    "is_active": true,
    "created_at": "2026-02-01T09:00:00Z",
    "updated_at": "2026-02-01T09:00:00Z"
  }
}
```

### Create Policy

```http
POST /api/v1/policies
Content-Type: application/json

{
  "name": "Corporate WiFi",
  "description": "Main office WiFi configuration",
  "platform": "all",
  "policy_type": "wifi",
  "policy_config": {
    "ssid": "CorpWiFi",
    "security_type": "WPA2-Enterprise",
    "eap_type": "PEAP",
    "auto_join": true
  },
  "is_active": true
}
```

**Response**:
```json
{
  "data": {
    "id": "770e8400-e29b-41d4-a716-446655440000",
    "enterprise_id": "660e8400-e29b-41d4-a716-446655440000",
    "name": "Corporate WiFi",
    "description": "Main office WiFi configuration",
    "platform": "all",
    "policy_type": "wifi",
    "policy_config": {
      "ssid": "CorpWiFi",
      "security_type": "WPA2-Enterprise",
      "eap_type": "PEAP",
      "auto_join": true
    },
    "is_active": true,
    "created_at": "2026-02-05T10:30:00Z",
    "updated_at": "2026-02-05T10:30:00Z"
  }
}
```

### Update Policy

```http
PUT /api/v1/policies/:id
Content-Type: application/json

{
  "name": "Updated WiFi Policy",
  "is_active": false
}
```

### Delete Policy

```http
DELETE /api/v1/policies/:id
```

### Assign Policy to Device

```http
POST /api/v1/policies/:id/assign
Content-Type: application/json

{
  "device_ids": [
    "550e8400-e29b-41d4-a716-446655440000",
    "551e8400-e29b-41d4-a716-446655440001"
  ]
}
```

**Response**:
```json
{
  "data": {
    "assigned": 2,
    "failed": 0
  }
}
```

### Unassign Policy from Device

```http
DELETE /api/v1/policies/:id/assign/:device_id
```

**Auth**: admin, operator

**Response**: `204 No Content`

---

## Applications

### List Applications

```http
GET /api/v1/apps
```

**Query Parameters**:
- `platform` (optional): Filter by platform

**Response**:
```json
{
  "data": [
    {
      "id": "aa0e8400-e29b-41d4-a716-446655440000",
      "enterprise_id": "660e8400-e29b-41d4-a716-446655440000",
      "name": "Microsoft Office",
      "platform": "windows",
      "identifier": "com.microsoft.office",
      "version": "2024",
      "install_type": "required",
      "app_config": {},
      "created_at": "2026-02-01T09:00:00Z",
      "updated_at": "2026-02-01T09:00:00Z"
    }
  ]
}
```

### Create Application

```http
POST /api/v1/apps
Content-Type: application/json

{
  "name": "Microsoft Office",
  "platform": "windows",
  "identifier": "com.microsoft.office",
  "version": "2024",
  "install_type": "required",
  "app_config": {}
}
```

**Response**: `201 Created`
```json
{
  "data": {
    "id": "aa0e8400-e29b-41d4-a716-446655440000",
    "enterprise_id": "660e8400-e29b-41d4-a716-446655440000",
    "name": "Microsoft Office",
    "platform": "windows",
    "identifier": "com.microsoft.office",
    "version": "2024",
    "install_type": "required",
    "app_config": {},
    "created_at": "2026-04-01T09:00:00Z",
    "updated_at": "2026-04-01T09:00:00Z"
  }
}
```

### Get Application

```http
GET /api/v1/apps/:id
```

### Update Application

```http
PUT /api/v1/apps/:id
Content-Type: application/json

{
  "name": "Microsoft Office 365",
  "version": "2025"
}
```

### Delete Application

```http
DELETE /api/v1/apps/:id
```

**Response**: `204 No Content`

### Deploy Application

```http
POST /api/v1/apps/:id/deploy
Content-Type: application/json

{
  "device_ids": [
    "550e8400-e29b-41d4-a716-446655440000"
  ],
  "install_type": "required"
}
```

**Install Types**:
- `required`: Force install
- `available`: Available in company portal
- `uninstall`: Remove application

**Response**:
```json
{
  "data": {
    "deployed": 1,
    "failed": 0
  }
}
```

---

## Device Commands & Profiles

### Send Command

```http
POST /api/v1/devices/:id/commands
Content-Type: application/json

{
  "command_type": "DeviceLock",
  "parameters": {
    "message": "Locked by IT",
    "pin": "1234"
  }
}
```

**Response**: `201 Created`
```json
{
  "data": {
    "command_id": "880e8400-e29b-41d4-a716-446655440000",
    "command_type": "DeviceLock",
    "status": "pending",
    "created_at": "2026-04-05T10:30:00Z"
  }
}
```

### List Command History

```http
GET /api/v1/devices/:id/commands
```

**Response**:
```json
{
  "data": [
    {
      "command_id": "880e8400-e29b-41d4-a716-446655440000",
      "command_type": "DeviceLock",
      "status": "completed",
      "created_at": "2026-04-05T10:30:00Z",
      "completed_at": "2026-04-05T10:30:05Z"
    }
  ]
}
```

### Install Profile

```http
POST /api/v1/devices/:id/profiles
Content-Type: application/json

{
  "profile_type": "wifi",
  "payload": {
    "ssid": "CorpWiFi",
    "security_type": "WPA2-Enterprise",
    "eap_type": "PEAP",
    "auto_join": true
  }
}
```

**Response**: `201 Created`
```json
{
  "data": {
    "command_id": "990e8400-e29b-41d4-a716-446655440000",
    "status": "pending",
    "created_at": "2026-04-05T10:31:00Z"
  }
}
```

### Remove Profile

```http
DELETE /api/v1/devices/:id/profiles/:profile_id
```

**Response**: `204 No Content`

### Restart Device

```http
POST /api/v1/devices/:id/restart
```

**Response**:
```json
{
  "data": {
    "command_id": "aa1e8400-e29b-41d4-a716-446655440000",
    "status": "pending",
    "created_at": "2026-04-05T10:32:00Z"
  }
}
```

---

## Windows Provisioning Packages

### Generate Provisioning Package

```http
POST /api/v1/windows/ppkg
Content-Type: application/json

{
  "template": "basic_enrollment",
  "parameters": {
    "enrollment_url": "https://mdm.example.com/enroll",
    "wifi_ssid": "CorpWiFi",
    "wifi_security": "WPA2-Enterprise"
  }
}
```

**Response**: Binary `.ppkg` file download with `Content-Type: application/octet-stream`.

### List Provisioning Package Templates

```http
GET /api/v1/windows/ppkg/templates
```

**Response**:
```json
{
  "data": [
    {
      "name": "basic_enrollment",
      "description": "Basic MDM enrollment with WiFi"
    },
    {
      "name": "full_provisioning",
      "description": "Full device provisioning with policies"
    }
  ]
}
```

---

## Users

### List Users

```http
GET /api/v1/users
```

### Create User

```http
POST /api/v1/users
Content-Type: application/json

{
  "email": "newadmin@example.com",
  "password": "SecurePassword123!",
  "full_name": "New Admin",
  "role": "admin"
}
```

### Update User

```http
PUT /api/v1/users/:id
```

### Delete User

```http
DELETE /api/v1/users/:id
```

---

## Platform-Specific Endpoints

### Windows

#### Discovery Service

```http
GET|POST /EnrollmentServer/Discovery.svc
```

MS-MDE2 discovery endpoint used by Windows devices during enrollment.

#### Policy Service

```http
POST /EnrollmentServer/Policy.svc
```

Returns enrollment policy to Windows devices.

#### Enrollment Service

```http
POST /EnrollmentServer/Enrollment.svc
```

Handles Windows device enrollment with certificate signing.

#### Management Sync

```http
POST /ManagementServer/MDM.svc
```

OMA-DM SyncML endpoint for device management sync sessions.

### macOS

#### Enrollment Profile

```http
GET /api/v1/macos/enroll/:enterprise_id
```

Returns `.mobileconfig` enrollment profile for download.

#### DEP Token PKI

```http
GET|PUT /api/v1/dep/:name/tokenpki
```

Generate or retrieve DEP token PKI certificate for Apple portal exchange.

#### DEP Assigner Profile

```http
GET|PUT /api/v1/dep/:name/assigner
```

Get or set the DEP auto-assign profile UUID.

#### DEP Devices

```http
GET /api/v1/dep/:name/devices
```

List devices synced from Apple DEP.

#### Check-in (NanoMDM webhook)

```http
PUT /checkin
```

Receives NanoMDM webhook JSON for device check-in events (Authenticate, TokenUpdate, CheckOut).

#### Command (NanoMDM webhook)

```http
PUT /mdm
```

Receives NanoMDM webhook JSON for command result events.

### Android

#### Create Enrollment Token

```http
POST /api/v1/android/enrollment-token/:enterprise_id
```

Creates an enrollment token for Android device enrollment.

#### Enrollment QR Code

```http
GET /api/v1/android/enrollment-token/:token_id/qr
```

Returns QR code image for device enrollment.

#### Webhook

```http
POST /api/v1/android/webhook
```

Receives events from Google Android Management API (enrollment, unenrollment, compliance, status).

---

## Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `UNAUTHORIZED` | 401 | Authentication required |
| `FORBIDDEN` | 403 | Insufficient permissions |
| `NOT_FOUND` | 404 | Resource not found |
| `VALIDATION_ERROR` | 400 | Invalid request data |
| `DEVICE_NOT_FOUND` | 404 | Device not found |
| `POLICY_NOT_FOUND` | 404 | Policy not found |
| `ENROLLMENT_FAILED` | 500 | Device enrollment failed |
| `COMMAND_FAILED` | 500 | Device command failed |
| `INTERNAL_ERROR` | 500 | Internal server error |

---

## Rate Limiting

- **Authenticated requests**: 1000 requests per hour
- **Unauthenticated requests**: 100 requests per hour

Rate limit headers:
```http
X-RateLimit-Limit: 1000
X-RateLimit-Remaining: 999
X-RateLimit-Reset: 1612540800
```

---

## Webhooks

Configure webhooks to receive real-time events.

### Event Types

- `device.enrolled`
- `device.unenrolled`
- `device.status_changed`
- `policy.applied`
- `policy.failed`
- `command.completed`
- `command.failed`

### Webhook Payload

```json
{
  "event": "device.enrolled",
  "timestamp": "2026-02-05T10:30:00Z",
  "data": {
    "device_id": "550e8400-e29b-41d4-a716-446655440000",
    "platform": "windows"
  }
}
```

---

## OpenAPI Specification

Full OpenAPI 3.0 specification available at:
```
GET /api/v1/openapi.yaml
```

Interactive documentation (Swagger UI):
```
GET /api/v1/docs
```

---

**Next Steps**:
1. Generate OpenAPI spec from code
2. Set up Swagger UI
3. Add request/response examples
4. Document authentication flows

---

## Sprint 4: Policy & Identity Endpoints

### Policy Versioning

#### List Policy Versions
```http
GET /api/v1/policies/{id}/versions?limit=20&offset=0
```
Returns version history for a policy (newest first).

#### Rollback Policy
```http
POST /api/v1/policies/{id}/rollback
Content-Type: application/json

{"version": 1}
```
Restores policy to a previous version. Creates a new version entry.

#### Translate Policy
```http
GET /api/v1/policies/{id}/translate?platform=macos
```
Returns platform-specific translation. Omit `platform` to get all three.

### Policy Templates

#### List Templates
```http
GET /api/v1/policy-templates
```

#### Clone Template
```http
POST /api/v1/policy-templates/{id}/clone
Content-Type: application/json

{"name": "My Security Policy"}
```

### Device Groups

#### CRUD
```http
GET    /api/v1/groups
POST   /api/v1/groups                    {"name": "Engineering", "description": "..."}
GET    /api/v1/groups/{id}
PUT    /api/v1/groups/{id}               {"name": "Updated Name"}
DELETE /api/v1/groups/{id}
```

#### Group Membership
```http
GET    /api/v1/groups/{id}/members
POST   /api/v1/groups/{id}/members       {"device_id": "uuid"}
DELETE /api/v1/groups/{id}/members/{device_id}
```

### Policy Assignments

#### Assign Policy to Target
```http
POST /api/v1/policies/{id}/assignments
Content-Type: application/json

{"target_type": "group", "target_id": "group-uuid", "priority": 10}
```
`target_type`: `device`, `group`, or `enterprise`. Lower priority number = higher precedence.

#### List Policy Assignments
```http
GET /api/v1/policies/{id}/assignments
```

#### Remove Assignment
```http
DELETE /api/v1/policy-assignments/{assignment_id}
```

#### Get Device Effective Policies
```http
GET /api/v1/devices/{id}/effective-policies
```
Returns all policies that apply to a device (via direct, group, and enterprise assignments), ordered by priority.

### Compliance

#### Enterprise Compliance Summary
```http
GET /api/v1/compliance
```
Returns: `{"compliant": 42, "non_compliant": 3, "unknown": 5, "error": 0, "total": 50}`

#### Device Compliance
```http
GET /api/v1/devices/{id}/compliance
```

#### Trigger Compliance Evaluation
```http
POST /api/v1/devices/{id}/compliance/evaluate
```

### Idempotency-Key Header

All `POST`, `PUT`, and `PATCH` requests support the `Idempotency-Key` header. If provided, the server caches the response for 24 hours. Subsequent requests with the same key return the cached response without re-executing the operation.

```http
POST /api/v1/policies
Idempotency-Key: unique-client-key-123
Content-Type: application/json

{"name": "My Policy", ...}
```
