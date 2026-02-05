# API Documentation

**Version**: 1.0  
**Base URL**: `https://your-mdm-server.com/api/v1`  
**Last Updated**: 2026-02-05

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

### Unenroll Device

```http
DELETE /api/v1/devices/:id
```

**Response**:
```json
{
  "data": {
    "message": "Device unenrolled successfully"
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
POST /api/v1/policies/:id/unassign
Content-Type: application/json

{
  "device_ids": [
    "550e8400-e29b-41d4-a716-446655440000"
  ]
}
```

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
      "name": "Microsoft Office",
      "platform": "windows",
      "version": "2024",
      "package_id": "com.microsoft.office",
      "install_count": 30,
      "created_at": "2026-02-01T09:00:00Z"
    }
  ]
}
```

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
GET /windows/discovery
```

Used by Windows devices during enrollment.

#### Enrollment Service

```http
POST /windows/enrollment
```

#### Management Service

```http
POST /windows/management
```

OMA-DM SyncML endpoint.

### macOS

#### Enrollment Profile

```http
GET /macos/enroll/:token
```

Returns enrollment profile for download.

#### Check-in

```http
PUT /macos/checkin
```

Device check-in endpoint.

### Android

#### Enrollment QR Code

```http
GET /android/enroll/:token/qr
```

Returns QR code for device enrollment.

#### Webhook

```http
POST /android/webhook
```

Receives events from Google Android Management API.

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
