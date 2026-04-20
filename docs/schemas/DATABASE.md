# Database Schema

**Version**: 1.0  
**Last Updated**: 2026-02-05

## Overview

The Local MDM database uses PostgreSQL 15+ with JSONB for flexible policy storage. All tables use UUIDs for primary keys and include audit timestamps.

## Schema Diagram

```
┌─────────────────┐
│   enterprises   │
└────────┬────────┘
         │
         │ 1:N
         │
┌────────▼────────┐      ┌──────────────┐
│     users       │      │   policies   │
└─────────────────┘      └──────┬───────┘
                                │
         ┌──────────────────────┤
         │                      │
         │ N:1                  │ N:M
         │                      │
┌────────▼────────┐      ┌──────▼───────────┐
│    devices      │◄─────┤ device_policies  │
└────────┬────────┘      └──────────────────┘
         │
         │ 1:N
         │
┌────────▼────────┐
│  certificates   │
└─────────────────┘
```

## Tables

### enterprises

Represents organizations/tenants in the system.

```sql
CREATE TABLE enterprises (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(100) UNIQUE NOT NULL,
    settings JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_enterprises_slug ON enterprises(slug);
CREATE INDEX idx_enterprises_deleted_at ON enterprises(deleted_at);
```

**Fields**:
- `id`: Unique identifier
- `name`: Display name of the enterprise
- `slug`: URL-friendly identifier
- `settings`: Enterprise-specific settings (JSONB)
- `created_at`: Creation timestamp
- `updated_at`: Last modification timestamp
- `deleted_at`: Soft delete timestamp

### users

Admin users who manage the MDM system.

```sql
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    enterprise_id UUID NOT NULL REFERENCES enterprises(id) ON DELETE CASCADE,
    email VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    full_name VARCHAR(255),
    role VARCHAR(50) NOT NULL DEFAULT 'admin',
    is_active BOOLEAN DEFAULT true,
    last_login_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    UNIQUE(enterprise_id, email)
);

CREATE INDEX idx_users_enterprise_id ON users(enterprise_id);
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_deleted_at ON users(deleted_at);
```

**Fields**:
- `id`: Unique identifier
- `enterprise_id`: Associated enterprise
- `email`: User email (login)
- `password_hash`: Bcrypt hashed password
- `full_name`: User's full name
- `role`: User role (admin, viewer, etc.)
- `is_active`: Account status
- `last_login_at`: Last successful login
- `created_at`: Creation timestamp
- `updated_at`: Last modification timestamp
- `deleted_at`: Soft delete timestamp

**Roles**:
- `super_admin`: Full system access
- `admin`: Full enterprise access
- `operator`: Can manage devices and policies
- `viewer`: Read-only access

### devices

Enrolled devices across all platforms.

```sql
CREATE TABLE devices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    enterprise_id UUID NOT NULL REFERENCES enterprises(id) ON DELETE CASCADE,
    platform VARCHAR(20) NOT NULL,
    device_id VARCHAR(255) NOT NULL,
    serial_number VARCHAR(255),
    name VARCHAR(255),
    model VARCHAR(255),
    os_version VARCHAR(100),
    enrollment_date TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    last_seen TIMESTAMP WITH TIME ZONE,
    status VARCHAR(50) DEFAULT 'enrolled',
    platform_data JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    UNIQUE(enterprise_id, platform, device_id)
);

CREATE INDEX idx_devices_enterprise_id ON devices(enterprise_id);
CREATE INDEX idx_devices_platform ON devices(platform);
CREATE INDEX idx_devices_status ON devices(status);
CREATE INDEX idx_devices_last_seen ON devices(last_seen);
CREATE INDEX idx_devices_deleted_at ON devices(deleted_at);
```

**Fields**:
- `id`: Unique identifier
- `enterprise_id`: Associated enterprise
- `platform`: Device platform (windows, macos, android)
- `device_id`: Platform-specific device identifier
- `serial_number`: Hardware serial number
- `name`: Device name
- `model`: Device model
- `os_version`: Operating system version
- `enrollment_date`: When device was enrolled
- `last_seen`: Last check-in timestamp
- `status`: Device status (enrolled, pending, unenrolled, wiped)
- `platform_data`: Platform-specific data (JSONB)
- `created_at`: Creation timestamp
- `updated_at`: Last modification timestamp
- `deleted_at`: Soft delete timestamp

**Platform Values**:
- `windows`: Windows 10/11
- `macos`: macOS
- `android`: Android

**Status Values**:
- `pending`: Enrollment initiated but not complete
- `enrolled`: Active and managed
- `unenrolled`: Removed from management
- `wiped`: Remote wipe executed
- `lost`: Marked as lost/stolen

### policies

Management policies that can be applied to devices.

```sql
CREATE TABLE policies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    enterprise_id UUID NOT NULL REFERENCES enterprises(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    platform VARCHAR(20) NOT NULL,
    policy_type VARCHAR(50) NOT NULL,
    policy_config JSONB NOT NULL,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_policies_enterprise_id ON policies(enterprise_id);
CREATE INDEX idx_policies_platform ON policies(platform);
CREATE INDEX idx_policies_policy_type ON policies(policy_type);
CREATE INDEX idx_policies_is_active ON policies(is_active);
CREATE INDEX idx_policies_deleted_at ON policies(deleted_at);
```

**Fields**:
- `id`: Unique identifier
- `enterprise_id`: Associated enterprise
- `name`: Policy name
- `description`: Policy description
- `platform`: Target platform (windows, macos, android, all)
- `policy_type`: Type of policy (wifi, vpn, security, app, restriction)
- `policy_config`: Policy configuration (JSONB)
- `is_active`: Whether policy is active
- `created_at`: Creation timestamp
- `updated_at`: Last modification timestamp
- `deleted_at`: Soft delete timestamp

**Policy Types**:
- `wifi`: WiFi configuration
- `vpn`: VPN configuration
- `security`: Security settings
- `app`: Application management
- `restriction`: Device restrictions
- `compliance`: Compliance rules

### device_policies

Junction table linking devices to policies.

```sql
CREATE TABLE device_policies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    policy_id UUID NOT NULL REFERENCES policies(id) ON DELETE CASCADE,
    applied_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    status VARCHAR(50) DEFAULT 'pending',
    error_message TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(device_id, policy_id)
);

CREATE INDEX idx_device_policies_device_id ON device_policies(device_id);
CREATE INDEX idx_device_policies_policy_id ON device_policies(policy_id);
CREATE INDEX idx_device_policies_status ON device_policies(status);
```

**Fields**:
- `id`: Unique identifier
- `device_id`: Associated device
- `policy_id`: Associated policy
- `applied_at`: When policy was applied
- `status`: Application status (pending, applied, failed)
- `error_message`: Error details if failed
- `created_at`: Creation timestamp
- `updated_at`: Last modification timestamp

**Status Values**:
- `pending`: Queued for application
- `applied`: Successfully applied
- `failed`: Application failed
- `removed`: Policy removed from device

### certificates

Device certificates for authentication.

```sql
CREATE TABLE certificates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    device_id UUID REFERENCES devices(id) ON DELETE CASCADE,
    cert_type VARCHAR(50) NOT NULL,
    subject VARCHAR(255) NOT NULL,
    serial_number VARCHAR(255) UNIQUE NOT NULL,
    cert_data TEXT NOT NULL,
    private_key_data TEXT,
    issued_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    revoked_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_certificates_device_id ON certificates(device_id);
CREATE INDEX idx_certificates_cert_type ON certificates(cert_type);
CREATE INDEX idx_certificates_serial_number ON certificates(serial_number);
CREATE INDEX idx_certificates_expires_at ON certificates(expires_at);
CREATE INDEX idx_certificates_revoked_at ON certificates(revoked_at);
```

**Fields**:
- `id`: Unique identifier
- `device_id`: Associated device (null for CA certs)
- `cert_type`: Certificate type (ca, device, apns)
- `subject`: Certificate subject
- `serial_number`: Certificate serial number
- `cert_data`: PEM-encoded certificate
- `private_key_data`: PEM-encoded private key (encrypted)
- `issued_at`: Issuance timestamp
- `expires_at`: Expiration timestamp
- `revoked_at`: Revocation timestamp
- `created_at`: Creation timestamp
- `updated_at`: Last modification timestamp

**Certificate Types**:
- `ca`: Root CA certificate
- `device`: Device client certificate
- `apns`: Apple Push Notification certificate

### api_tokens

API tokens for programmatic access.

```sql
CREATE TABLE api_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    token_hash VARCHAR(255) NOT NULL UNIQUE,
    scopes JSONB DEFAULT '[]',
    last_used_at TIMESTAMP WITH TIME ZONE,
    expires_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    revoked_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_api_tokens_user_id ON api_tokens(user_id);
CREATE INDEX idx_api_tokens_token_hash ON api_tokens(token_hash);
CREATE INDEX idx_api_tokens_revoked_at ON api_tokens(revoked_at);
```

**Fields**:
- `id`: Unique identifier
- `user_id`: Associated user
- `name`: Token name/description
- `token_hash`: Hashed token value
- `scopes`: Allowed scopes (JSONB array)
- `last_used_at`: Last usage timestamp
- `expires_at`: Expiration timestamp
- `created_at`: Creation timestamp
- `updated_at`: Last modification timestamp
- `revoked_at`: Revocation timestamp

### audit_logs

Audit trail for all system actions.

```sql
CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    enterprise_id UUID REFERENCES enterprises(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    action VARCHAR(100) NOT NULL,
    resource_type VARCHAR(50) NOT NULL,
    resource_id UUID,
    details JSONB DEFAULT '{}',
    ip_address INET,
    user_agent TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_audit_logs_enterprise_id ON audit_logs(enterprise_id);
CREATE INDEX idx_audit_logs_user_id ON audit_logs(user_id);
CREATE INDEX idx_audit_logs_action ON audit_logs(action);
CREATE INDEX idx_audit_logs_resource_type ON audit_logs(resource_type);
CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at);
```

**Fields**:
- `id`: Unique identifier
- `enterprise_id`: Associated enterprise
- `user_id`: User who performed action
- `action`: Action performed (create, update, delete, enroll, etc.)
- `resource_type`: Type of resource affected
- `resource_id`: ID of affected resource
- `details`: Additional details (JSONB)
- `ip_address`: Client IP address
- `user_agent`: Client user agent
- `created_at`: Action timestamp

### apps

Application catalog for managed app deployment.

```sql
CREATE TABLE apps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    enterprise_id UUID NOT NULL REFERENCES enterprises(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    platform VARCHAR(20) NOT NULL,
    identifier VARCHAR(500) NOT NULL,
    version VARCHAR(100) DEFAULT '',
    install_type VARCHAR(20) NOT NULL DEFAULT 'required',
    app_config JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    UNIQUE(enterprise_id, platform, identifier)
);

CREATE INDEX idx_apps_enterprise_id ON apps(enterprise_id);
CREATE INDEX idx_apps_platform ON apps(platform);
```

**Fields**:
- `id`: Unique identifier
- `enterprise_id`: Associated enterprise
- `name`: Application display name
- `platform`: Target platform (windows, macos, android)
- `identifier`: Platform-specific app identifier (bundle ID, package name, etc.)
- `version`: Application version
- `install_type`: Default install type (required, available, uninstall)
- `app_config`: Platform-specific configuration (JSONB)
- `created_at`: Creation timestamp
- `updated_at`: Last modification timestamp
- `deleted_at`: Soft delete timestamp

**Install Types**:
- `required`: Force install on assigned devices
- `available`: Available in company portal
- `uninstall`: Remove from assigned devices

## Migrations

Migrations are managed using golang-migrate and stored in `/migrations` directory.

### Migration Naming Convention

```
{version}_{description}.up.sql
{version}_{description}.down.sql
```

Example:
```
000001_initial_schema.up.sql
000001_initial_schema.down.sql
```

### Running Migrations

```bash
# Up
make migrate-up

# Down
make migrate-down

# Create new migration
make migrate-create NAME=add_device_groups
```

## Data Types

### JSONB Examples

#### Enterprise Settings
```json
{
  "enrollment_domains": ["company.com"],
  "require_2fa": true,
  "session_timeout_minutes": 60
}
```

#### Device Platform Data (Windows)
```json
{
  "windows_edition": "Pro",
  "tpm_version": "2.0",
  "bitlocker_enabled": true,
  "last_sync_time": "2026-02-05T10:30:00Z"
}
```

#### Device Platform Data (macOS)
```json
{
  "model_identifier": "MacBookPro18,1",
  "filevault_enabled": true,
  "dep_enrolled": false,
  "push_magic": "..."
}
```

#### Device Platform Data (Android)
```json
{
  "management_mode": "work_profile",
  "play_store_mode": "whitelist",
  "security_patch_level": "2026-02-01"
}
```

#### Policy Config (WiFi)
```json
{
  "ssid": "CorpWiFi",
  "security_type": "WPA2-Enterprise",
  "eap_type": "PEAP",
  "auto_join": true,
  "hidden_network": false
}
```

## Indexes

All foreign keys have indexes for query performance. Additional indexes are created for:
- Frequently queried fields (status, platform, email)
- Soft delete columns (deleted_at)
- Timestamp fields used in sorting (last_seen, created_at)

## Constraints

- All foreign keys use `ON DELETE CASCADE` or `ON DELETE SET NULL` appropriately
- Unique constraints on natural keys (email per enterprise, device_id per platform)
- Check constraints on enum-like fields (future enhancement)

## Performance Considerations

- JSONB fields indexed with GIN indexes where needed
- Partitioning audit_logs by date (future enhancement)
- Read replicas for reporting queries (future enhancement)

---

**Next Steps**:
1. Create initial migration file
2. Add database connection code
3. Implement repository layer
4. Add seed data for development
