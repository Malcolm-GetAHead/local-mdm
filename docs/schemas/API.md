# API Documentation

**Version**: 1.0  
**Base URL**: `https://your-mdm-server.com/api/v1`  
**Last Updated**: 2026-04-22

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

#### Get Version

```http
GET /version
```

**Response**:
```json
{
  "version": "1.0.0",
  "build_time": "2026-04-21T00:00:00Z"
}
```

#### API Documentation

```http
GET /docs
GET /docs/openapi.yaml
```

Swagger UI and OpenAPI spec. No authentication required.

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

**Auth**: admin, operator

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

**Auth**: admin + IP allowlist

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

**Auth**: admin, operator

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

**Auth**: admin, operator

### Delete Policy

```http
DELETE /api/v1/policies/:id
```

**Auth**: admin

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

**Auth**: admin, operator

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

**Auth**: admin, operator

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

**Auth**: admin, operator

### Delete Application

```http
DELETE /api/v1/apps/:id
```

**Auth**: admin

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

**Auth**: admin, operator

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

**Auth**: admin, operator

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

**Auth**: admin, operator

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

**Auth**: admin, operator

**Response**: `204 No Content`

### Restart Device

```http
POST /api/v1/devices/:id/restart
```

**Auth**: admin, operator

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

## Certificates

### List Certificates

```http
GET /api/v1/certificates
```

**Auth**: any authenticated

**Query Parameters**:
- `device_id` (optional): Filter by device
- `limit` (optional): Items per page
- `offset` (optional): Offset for pagination

---

## Audit Logs

### List Audit Logs

```http
GET /api/v1/audit-logs
```

**Auth**: admin, super_admin

**Query Parameters**:
- `limit` (optional): Items per page
- `offset` (optional): Offset for pagination
- `action` (optional): Filter by action type
- `start_date` (optional): Filter from date (RFC 3339)
- `end_date` (optional): Filter to date (RFC 3339)

---

## User Management

### List Users

```http
GET /api/v1/users
```

**Auth**: admin, super_admin

**Query Parameters**:
- `limit` (optional): Items per page (default: 100)
- `offset` (optional): Offset for pagination

**Response**:
```json
{
  "data": [
    {
      "id": "bb0e8400-e29b-41d4-a716-446655440000",
      "enterprise_id": "660e8400-e29b-41d4-a716-446655440000",
      "email": "admin@example.com",
      "full_name": "Admin User",
      "role": "admin",
      "is_active": true,
      "last_login_at": "2026-04-20T10:00:00Z",
      "created_at": "2026-02-01T09:00:00Z",
      "updated_at": "2026-04-20T10:00:00Z"
    }
  ]
}
```

### Create User

```http
POST /api/v1/users
Content-Type: application/json

{
  "email": "newuser@example.com",
  "password": "securepassword",
  "full_name": "New User",
  "role": "operator"
}
```

**Auth**: admin, super_admin

**Response**: `201 Created`

### Get User

```http
GET /api/v1/users/:id
```

**Auth**: admin, super_admin

### Update User

```http
PUT /api/v1/users/:id
Content-Type: application/json

{
  "full_name": "Updated Name",
  "role": "admin",
  "is_active": true
}
```

**Auth**: admin, super_admin. Fields are optional — only provided fields are updated.

### Delete User

```http
DELETE /api/v1/users/:id
```

**Auth**: admin, super_admin. Soft-deletes the user.

**Response**: `204 No Content`

---

## API Tokens

### Create API Token

```http
POST /api/v1/tokens
Content-Type: application/json

{
  "name": "CI/CD Token",
  "scopes": ["devices:read", "policies:read"],
  "expires_in_days": 90
}
```

**Auth**: admin, super_admin

**Response**: `201 Created`
```json
{
  "data": {
    "id": "cc0e8400-e29b-41d4-a716-446655440000",
    "name": "CI/CD Token",
    "token": "lmdm_abc123...",
    "scopes": ["devices:read", "policies:read"],
    "expires_at": "2026-07-20T10:00:00Z",
    "created_at": "2026-04-22T10:00:00Z"
  }
}
```

**Note**: The `token` field (with `lmdm_` prefix) is returned only once at creation. It is stored as a SHA-256 hash and cannot be retrieved again.

### List API Tokens

```http
GET /api/v1/tokens
```

**Auth**: admin, super_admin

**Response**:
```json
{
  "data": [
    {
      "id": "cc0e8400-e29b-41d4-a716-446655440000",
      "name": "CI/CD Token",
      "scopes": ["devices:read", "policies:read"],
      "last_used_at": "2026-04-21T15:00:00Z",
      "expires_at": "2026-07-20T10:00:00Z",
      "created_at": "2026-04-22T10:00:00Z"
    }
  ]
}
```

### Delete (Revoke) API Token

```http
DELETE /api/v1/tokens/:id
```

**Auth**: admin, super_admin

**Response**: `204 No Content`

---

## Reports

### Device Inventory Report

```http
GET /api/v1/reports/devices
```

**Auth**: admin, super_admin

**Query Parameters**:
- `platform` (optional): Filter by platform (windows, macos, android)
- `format` (optional): Response format — `json` (default) or `csv`

**Response** (JSON):
```json
{
  "data": {
    "total_devices": 150,
    "by_platform": {"windows": 80, "macos": 50, "android": 20},
    "by_status": {"enrolled": 140, "pending": 5, "unenrolled": 5},
    "devices": [...]
  }
}
```

**Response** (CSV): Returns `Content-Type: text/csv` with device inventory rows.

### Compliance Report

```http
GET /api/v1/reports/compliance
```

**Auth**: admin, super_admin

**Query Parameters**:
- `format` (optional): Response format — `json` (default) or `csv`

**Response** (JSON):
```json
{
  "data": {
    "compliant": 120,
    "non_compliant": 15,
    "unknown": 10,
    "error": 5,
    "total": 150,
    "by_policy": [...]
  }
}
```

### Enrollment Report

```http
GET /api/v1/reports/enrollments
```

**Auth**: admin, super_admin

**Query Parameters**:
- `days` (optional): Number of days to look back (default: 30)

**Response**:
```json
{
  "data": {
    "period_days": 30,
    "total_enrollments": 25,
    "by_platform": {"windows": 12, "macos": 8, "android": 5},
    "by_day": [
      {"date": "2026-04-21", "count": 3},
      {"date": "2026-04-20", "count": 1}
    ]
  }
}
```

---

## SCEP

Simple Certificate Enrollment Protocol endpoint for device certificate enrollment.

### Get CA Capabilities

```http
GET /scep?operation=GetCACaps
```

**Auth**: none (device enrollment)

**Response**: Plain text list of CA capabilities.
```
POSTPKIOperation
SHA-256
AES
DES3
SCEPStandard
```

### Get CA Certificate

```http
GET /scep?operation=GetCACert
```

**Auth**: none (device enrollment)

**Response**: DER-encoded CA certificate (`application/x-x509-ca-cert`).

### PKI Operation

```http
POST /scep?operation=PKIOperation
Content-Type: application/x-pki-message
```

**Auth**: SCEP challenge password (validated against `scep_challenges` table)

**Request Body**: DER-encoded PKCS#7 CSR envelope.

**Response**: DER-encoded PKCS#7 signed certificate response (`application/x-pki-message`).

---

## Health Check (Extended)

### Readiness Probe

```http
GET /health/ready
```

**Response**:
```json
{
  "status": "ready",
  "checks": {
    "database": {"status": "up", "latency_ms": 2},
    "database_reader": {"status": "up", "latency_ms": 1},
    "migrations": {"status": "up"}
  },
  "timestamp": "2026-04-22T10:00:00Z"
}
```

Returns `200 OK` when all dependencies are healthy, `503 Service Unavailable` otherwise. Each check includes per-dependency latency for diagnostics.

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

**Auth**: admin, operator

**Response**: Binary `.ppkg` file download with `Content-Type: application/octet-stream`.

### List Provisioning Package Templates

```http
GET /api/v1/windows/ppkg/templates
```

**Auth**: any authenticated

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

## Enrollment Tokens

Enrollment tokens control who can enroll devices. Admins create tokens with optional use limits and expiry. The response includes platform-specific enrollment instructions:

- **Windows**: Email address (`<token>@localmdm.local`) — enter in Settings → "Enroll only in device management"
- **macOS**: Enrollment URL — open in Safari to download the enrollment profile

Tokens have three statuses: `active`, `expired`, `revoked`. Expired tokens are updated automatically by a periodic cleanup job (hourly) and on-access when a device attempts to use an expired token.

### Create Enrollment Token

```http
POST /api/v1/enrollment-tokens
Content-Type: application/json

{
  "enterprise_id": "00000000-0000-0000-0000-000000000001",
  "description": "IT onboarding batch May 2026",
  "max_uses": 5,
  "expires_in": "168h"
}
```

**Auth**: admin, super_admin

**Fields**:
- `enterprise_id` (required): Enterprise the token enrolls devices into
- `description` (optional): Human-readable label
- `max_uses` (optional): Maximum enrollments allowed. Omit for unlimited.
- `expires_in` (optional): Duration string (e.g., `1h`, `24h`, `168h`). Default: `24h`.

**Response**: `201 Created`
```json
{
  "data": {
    "id": "c39f3d45-ce69-423d-ad8f-b99e84eb2bc0",
    "enterprise_id": "00000000-0000-0000-0000-000000000001",
    "token": "5370aa74b9005ce7b52398a2951a3a1b",
    "email": "5370aa74b9005ce7b52398a2951a3a1b@localmdm.local",
    "macos_enroll_url": "https://192.168.1.102:8443/api/v1/macos/enroll/00000000-0000-0000-0000-000000000001?token=5370aa74b9005ce7b52398a2951a3a1b",
    "description": "IT onboarding batch May 2026",
    "max_uses": 5,
    "uses_remaining": 5,
    "status": "active",
    "expires_at": "2026-05-07T10:00:00Z",
    "created_at": "2026-04-30T10:00:00Z"
  }
}
```

### List Enrollment Tokens

```http
GET /api/v1/enrollment-tokens?enterprise_id=00000000-0000-0000-0000-000000000001
```

**Auth**: admin, super_admin

**Query Parameters**:
- `enterprise_id` (required): Filter by enterprise
- `limit`, `offset`: Pagination (default: 100, 0)

**Response**: `200 OK` — paginated list of tokens with `status`, `max_uses`, `uses_remaining`, `expires_at`, `revoked_at`.

### Revoke Enrollment Token

```http
DELETE /api/v1/enrollment-tokens/{id}
```

**Auth**: admin, super_admin

**Response**: `204 No Content`

Revoked tokens immediately stop accepting new enrollments (status changes to `revoked`). Existing enrolled devices are not affected.

## Platform-Specific Endpoints

### Windows

#### Discovery Service

```http
GET|POST /EnrollmentServer/Discovery.svc
GET|POST /EnrollmentServer/:enterprise_id/Discovery.svc
```

MS-MDE2 discovery endpoint used by Windows devices during enrollment. The `enterprise_id` variant propagates the enterprise context to the enrollment URL for multi-tenant support.

#### Policy Service

```http
POST /EnrollmentServer/Policy.svc
```

Returns enrollment policy to Windows devices.

#### Enrollment Service

```http
POST /EnrollmentServer/Enrollment.svc
POST /EnrollmentServer/:enterprise_id/Enrollment.svc
```

Handles Windows device enrollment with certificate signing. When `enterprise_id` is present in the URL, a device record is created in the database upon successful enrollment.

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

**Auth**: admin, super_admin. Generate or retrieve DEP token PKI certificate for Apple portal exchange.

#### DEP Assigner Profile

```http
GET|PUT /api/v1/dep/:name/assigner
```

**Auth**: admin, super_admin. Get or set the DEP auto-assign profile UUID.

#### DEP Devices

```http
GET /api/v1/dep/:name/devices
```

List devices synced from Apple DEP.

#### NanoMDM Webhook

```http
POST /api/v1/macos/webhook
```

Receives NanoMDM webhook JSON for device check-in events (Authenticate, TokenUpdate, CheckOut) and command result events. NanoMDM forwards these automatically when configured with `NANOMDM_WEBHOOK_URL`.

Legacy endpoints `PUT /checkin` and `PUT /mdm` are still registered for backward compatibility.

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

## Sprint 4: Policy & Identity Endpoints

### Policy Versioning

#### List Policy Versions
```http
GET /api/v1/policies/{id}/versions?limit=20&offset=0
```
**Auth**: any authenticated. Returns version history for a policy (newest first).

#### Rollback Policy
```http
POST /api/v1/policies/{id}/rollback
Content-Type: application/json

{"version": 1}
```
**Auth**: admin, operator. Restores policy to a previous version. Creates a new version entry.

#### Translate Policy
```http
GET /api/v1/policies/{id}/translate?platform=macos
```
**Auth**: any authenticated. Returns platform-specific translation. Omit `platform` to get all three.

### Policy Templates

#### List Templates
```http
GET /api/v1/policy-templates
```
**Auth**: any authenticated.

#### Clone Template
```http
POST /api/v1/policy-templates/{id}/clone
Content-Type: application/json

{"name": "My Security Policy"}
```
**Auth**: admin, operator.

### Device Groups

#### CRUD
```http
GET    /api/v1/groups                                              # Auth: any authenticated
POST   /api/v1/groups                    {"name": "Engineering"}   # Auth: admin, operator
GET    /api/v1/groups/{id}                                         # Auth: any authenticated
PUT    /api/v1/groups/{id}               {"name": "Updated Name"}  # Auth: admin, operator
DELETE /api/v1/groups/{id}                                         # Auth: admin
```

#### Group Membership
```http
GET    /api/v1/groups/{id}/members                                 # Auth: any authenticated
POST   /api/v1/groups/{id}/members       {"device_id": "uuid"}    # Auth: admin, operator
DELETE /api/v1/groups/{id}/members/{device_id}                     # Auth: admin, operator
```

### Policy Assignments

#### Assign Policy to Target
```http
POST /api/v1/policies/{id}/assignments
Content-Type: application/json

{"target_type": "group", "target_id": "group-uuid", "priority": 10}
```
**Auth**: admin, operator. `target_type`: `device`, `group`, or `enterprise`. Lower priority number = higher precedence.

#### List Policy Assignments
```http
GET /api/v1/policies/{id}/assignments
```
**Auth**: any authenticated.

#### Remove Assignment
```http
DELETE /api/v1/policy-assignments/{assignment_id}
```
**Auth**: admin, operator.

#### Get Device Effective Policies
```http
GET /api/v1/devices/{id}/effective-policies
```
**Auth**: any authenticated. Returns all policies that apply to a device (via direct, group, and enterprise assignments), ordered by priority.

### Compliance

#### Enterprise Compliance Summary
```http
GET /api/v1/compliance
```
**Auth**: any authenticated. Returns: `{"compliant": 42, "non_compliant": 3, "unknown": 5, "error": 0, "total": 50}`

#### Device Compliance
```http
GET /api/v1/devices/{id}/compliance
```
**Auth**: any authenticated.

#### Trigger Compliance Evaluation
```http
POST /api/v1/devices/{id}/compliance/evaluate
```
**Auth**: admin, operator.

### Idempotency-Key Header

All `POST`, `PUT`, and `PATCH` requests support the `Idempotency-Key` header. If provided, the server caches the response for 24 hours. Subsequent requests with the same key return the cached response without re-executing the operation.

```http
POST /api/v1/policies
Idempotency-Key: unique-client-key-123
Content-Type: application/json

{"name": "My Policy", ...}
```
