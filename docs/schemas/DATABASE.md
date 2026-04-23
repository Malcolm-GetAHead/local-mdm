# Database Schema

**Version**: 2.0  
**Last Updated**: 2026-04-22

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
    password_hash VARCHAR(255),
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
- `password_hash`: NULL (auth via Keycloak OIDC, not local passwords)
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
    is_template BOOLEAN NOT NULL DEFAULT false,
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
- `is_template`: Whether this policy is a template (for cloning)
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

### device_commands

Device management command queue (migration 000003).

```sql
CREATE TABLE device_commands (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    command_type VARCHAR(50) NOT NULL,
    command_data JSONB DEFAULT '{}',
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    sent_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    error_message TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    -- Added in migration 000006:
    enterprise_id UUID REFERENCES enterprises(id) ON DELETE CASCADE,
    expires_at TIMESTAMP WITH TIME ZONE,
    batch_id UUID
);

CREATE INDEX idx_device_commands_device_id ON device_commands(device_id);
CREATE INDEX idx_device_commands_status ON device_commands(status);
CREATE INDEX idx_device_commands_device_pending ON device_commands(device_id, status) WHERE status = 'pending';
CREATE INDEX idx_device_commands_enterprise_id ON device_commands(enterprise_id);
CREATE INDEX idx_device_commands_batch_id ON device_commands(batch_id) WHERE batch_id IS NOT NULL;
```

**Fields**:
- `id`: Unique identifier
- `device_id`: Target device
- `command_type`: Command type (lock, wipe, restart, device_info, install_profile, etc.)
- `command_data`: Command parameters (JSONB)
- `status`: Command status (pending, sent, completed, failed)
- `sent_at`: When command was sent to device
- `completed_at`: When command completed or failed
- `error_message`: Error details if failed
- `enterprise_id`: Enterprise scope for queries (added Sprint 4)
- `expires_at`: Command expiration time (added Sprint 4)
- `batch_id`: Groups bulk operations (added Sprint 4)

### dep_names

DEP (Device Enrollment Program) server configurations with encrypted OAuth tokens (migration 000004).

```sql
CREATE TABLE dep_names (
    name VARCHAR(255) NOT NULL PRIMARY KEY,
    consumer_key BYTEA NULL,
    consumer_secret BYTEA NULL,
    access_token BYTEA NULL,
    access_secret BYTEA NULL,
    access_token_expiry TIMESTAMPTZ NULL,
    config_base_url VARCHAR(255) NULL,
    tokenpki_cert_pem TEXT NULL,
    tokenpki_key_pem BYTEA NULL,
    tokenpki_staging_cert_pem TEXT NULL,
    tokenpki_staging_key_pem BYTEA NULL,
    syncer_cursor VARCHAR(1024) NULL,
    assigner_profile_uuid TEXT NULL,
    assigner_profile_uuid_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);
```

**Fields**:
- `name`: DEP server name (primary key)
- `consumer_key/secret`, `access_token/secret`: OAuth1 tokens encrypted with pgp_sym_encrypt
- `access_token_expiry`: Token expiration
- `config_base_url`: Apple DEP API base URL
- `tokenpki_*`: Token PKI certificates and keys for Apple portal exchange
- `syncer_cursor`: Cursor for incremental DEP device sync
- `assigner_profile_uuid`: Auto-assign MDM profile UUID

### dep_devices

DEP-synced device tracking with serial numbers and profile assignment status (migration 000004).

```sql
CREATE TABLE dep_devices (
    serial_number VARCHAR(255) NOT NULL,
    dep_name VARCHAR(255) NOT NULL REFERENCES dep_names(name) ON DELETE CASCADE,
    profile_uuid TEXT NULL,
    profile_status VARCHAR(50) DEFAULT 'empty',
    device_data JSONB DEFAULT '{}',
    synced_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    assigned_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (serial_number, dep_name)
);
```

**Fields**:
- `serial_number`: Device serial number (from Apple)
- `dep_name`: Associated DEP server
- `profile_uuid`: Assigned MDM profile UUID
- `profile_status`: Profile assignment status (empty, assigned, pushed, removed)
- `device_data`: Device metadata from Apple (JSONB)
- `synced_at`: Last sync from Apple DEP

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

## Sprint 5 Schema (Migration 000008)

### scep_challenges

SCEP challenge passwords for device certificate enrollment. Challenges are single-use and time-limited.

```sql
CREATE TABLE scep_challenges (
    password VARCHAR(64) PRIMARY KEY,
    device_id VARCHAR(255) NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    used BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
```

**Fields**:
- `password`: SCEP challenge password (primary key, unique per challenge)
- `device_id`: Device identifier the challenge was issued for
- `expires_at`: Challenge expiration time
- `used`: Whether the challenge has been consumed
- `created_at`: Creation timestamp

## Sprint 5 Performance Indexes (Migration 000009)

Composite indexes for common query patterns identified during Sprint 5 performance work.

```sql
-- Device queries by enterprise + status/platform
CREATE INDEX idx_devices_enterprise_status ON devices(enterprise_id, status);
CREATE INDEX idx_devices_enterprise_platform ON devices(enterprise_id, platform);

-- Active policies per enterprise
CREATE INDEX idx_policies_enterprise_active ON policies(enterprise_id, is_active);

-- Compliance lookups by device + policy
CREATE INDEX idx_compliance_device_policy ON compliance_results(device_id, policy_id);

-- Audit log queries by enterprise + time range
CREATE INDEX idx_audit_logs_enterprise_created ON audit_logs(enterprise_id, created_at);

-- Command queue by device + status
CREATE INDEX idx_commands_device_status ON device_commands(device_id, status);

-- Group membership lookups by device
CREATE INDEX idx_group_members_device ON group_memberships(device_id);
```

These composite indexes optimize the most frequent query patterns: enterprise-scoped device listing, compliance evaluation, audit log search with date filtering, and command queue polling.

## Sprint 5c: Nullable password_hash (Migration 000010)

Auth is via Keycloak OIDC — local passwords are not used. The `password_hash` column was `NOT NULL` with a misleading `"oidc-managed"` placeholder.

```sql
ALTER TABLE users ALTER COLUMN password_hash DROP NOT NULL;
UPDATE users SET password_hash = NULL WHERE password_hash = 'oidc-managed';
```

Users created after this migration have `NULL` password_hash. Repository queries use `COALESCE(password_hash, '')` for scan safety.

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
- **Writer/Reader pool split** (Sprint 4b): all writes go to Writer pool, reads go to Reader pool. In dev both point to the same PostgreSQL; in production Reader points to Aurora read replica
- Connection pooling with configurable limits per pool (MaxOpenConns, MaxIdleConns, ConnMaxLifetime)

---

**Next Steps**:
1. Create initial migration file
2. Add database connection code
3. Implement repository layer
4. Add seed data for development

## Sprint 4 Schema Prep (Migration 000006)

### device_groups

Static device groups for policy targeting.

| Column | Type | Description |
|--------|------|-------------|
| id | UUID PK | Group identifier |
| enterprise_id | UUID FK → enterprises | Owning enterprise |
| name | VARCHAR(255) | Group name (unique per enterprise) |
| description | TEXT | Group description |
| created_at | TIMESTAMPTZ | Creation time |
| updated_at | TIMESTAMPTZ | Last update |
| deleted_at | TIMESTAMPTZ | Soft delete |

### group_memberships

Device-to-group mapping (many-to-many).

| Column | Type | Description |
|--------|------|-------------|
| group_id | UUID FK → device_groups | Group |
| device_id | UUID FK → devices | Device |
| added_at | TIMESTAMPTZ | When device was added |

PK: (group_id, device_id)

### policy_assignments

Flexible policy targeting — assign to device, group, or enterprise-wide.

| Column | Type | Description |
|--------|------|-------------|
| id | UUID PK | Assignment identifier |
| policy_id | UUID FK → policies | Policy being assigned |
| target_type | VARCHAR(20) | `device`, `group`, or `enterprise` |
| target_id | UUID | ID of the target (device, group, or enterprise) |
| priority | INT | Higher = takes precedence on conflict |
| created_at | TIMESTAMPTZ | Assignment time |

Unique: (policy_id, target_type, target_id)

### compliance_results

Per-device, per-policy compliance evaluation state.

| Column | Type | Description |
|--------|------|-------------|
| id | UUID PK | Result identifier |
| device_id | UUID FK → devices | Device evaluated |
| policy_id | UUID FK → policies | Policy evaluated against |
| status | VARCHAR(20) | `compliant`, `non_compliant`, `unknown`, `error` |
| details | JSONB | Evaluation details |
| evaluated_at | TIMESTAMPTZ | When evaluation ran |

Unique: (device_id, policy_id)

### device_apps

Tracks app installation state per device.

| Column | Type | Description |
|--------|------|-------------|
| device_id | UUID FK → devices | Device |
| app_id | UUID FK → apps | App from catalog |
| installed_version | VARCHAR(100) | Version installed on device |
| status | VARCHAR(20) | `pending`, `installed`, `failed`, `removed` |
| installed_at | TIMESTAMPTZ | When app was installed |
| updated_at | TIMESTAMPTZ | Last status change |

PK: (device_id, app_id)

### Column additions

- `device_commands.enterprise_id` — UUID FK, for enterprise-scoped queries
- `device_commands.expires_at` — TIMESTAMPTZ, command expiration
- `device_commands.batch_id` — UUID, tracks bulk operations
- `policies.is_template` — BOOLEAN, marks policy templates

## Sprint 4 Features (Migration 000007)

### token_cache

PostgreSQL-backed token cache (replaces Redis).

| Column | Type | Description |
|--------|------|-------------|
| token_hash | VARCHAR(64) PK | SHA-256 hex of the bearer token |
| user_data | JSONB | Cached user info (id, email, roles, enterprise_id) |
| expires_at | TIMESTAMPTZ | When the cached entry expires |

### idempotency_keys

Stores responses for Idempotency-Key header support.

| Column | Type | Description |
|--------|------|-------------|
| key | VARCHAR(255) PK | Client-provided idempotency key |
| method | VARCHAR(10) | HTTP method (POST, PUT, PATCH) |
| path | VARCHAR(500) | Request path |
| status_code | INT | Cached response status |
| response_headers | JSONB | Cached response headers |
| response_body | BYTEA | Cached response body |
| created_at | TIMESTAMPTZ | When the key was first used |
| expires_at | TIMESTAMPTZ | When the cached response expires (24h) |

### policy_versions

Full snapshots of policy state for versioning and rollback.

| Column | Type | Description |
|--------|------|-------------|
| id | UUID PK | Version identifier |
| policy_id | UUID FK → policies | Parent policy |
| version | INT | Version number (1, 2, 3...) |
| policy_config | JSONB | Full policy config snapshot |
| name | VARCHAR(255) | Policy name at this version |
| description | TEXT | Policy description at this version |
| platform | VARCHAR(20) | Platform at this version |
| policy_type | VARCHAR(50) | Policy type at this version |
| is_active | BOOLEAN | Active state at this version |
| created_by | VARCHAR(255) | User who created this version |
| created_at | TIMESTAMPTZ | When version was created |

Unique: (policy_id, version)

### Event Triggers

PostgreSQL `LISTEN`/`NOTIFY` triggers for event-driven architecture (Go listener in Sprint 5b).

| Trigger | Table | Event Type | Fires On |
|---------|-------|------------|----------|
| device_enrolled_event | devices | device.enrolled | INSERT |
| device_updated_event | devices | device.status_changed | UPDATE OF status (when changed) |
| policy_updated_event | policies | policy.updated | UPDATE |
| command_created_event | device_commands | command.created | INSERT |
| policy_assigned_event | policy_assignments | policy.assigned | INSERT |
| compliance_evaluated_event | compliance_results | compliance.evaluated | INSERT OR UPDATE |

All triggers call `notify_mdm_event()` which sends JSON payload to `mdm_events` channel:
```json
{"type": "event.type", "id": "entity-uuid", "device_id": "device-uuid", "table": "table_name", "op": "INSERT"}
```
