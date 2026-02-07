# Data Migration & Versioning

**Version**: 1.0  
**Last Updated**: 2026-02-06

## Overview

This document defines strategies for data migration, policy schema versioning, data retention, and GDPR compliance.

---

## Device Import from Other MDMs

### Import API

**Endpoint**: `POST /api/v1/devices/import`

**Request**:
```json
{
  "source": "jamf|intune|airwatch|csv",
  "devices": [
    {
      "platform": "macos",
      "serial_number": "C02XYZ123456",
      "name": "John's MacBook",
      "model": "MacBookPro18,1",
      "os_version": "14.2.1",
      "user_email": "john@example.com",
      "metadata": {
        "department": "Engineering",
        "location": "San Francisco"
      }
    }
  ]
}
```

**Response**:
```json
{
  "imported": 45,
  "failed": 5,
  "errors": [
    {
      "serial_number": "C02ABC999999",
      "error": "Device already exists"
    }
  ]
}
```

### CSV Import Format

```csv
platform,serial_number,name,model,os_version,user_email,department,location
macos,C02XYZ123456,John's MacBook,MacBookPro18,14.2.1,john@example.com,Engineering,SF
windows,VMW-123456,Jane's Laptop,Surface Pro 9,11.0.22621,jane@example.com,Sales,NYC
android,ABC123DEF456,Bob's Phone,Pixel 7,13,bob@example.com,Support,Remote
```

### Import Process

1. **Pre-validation**: Check for duplicates, validate required fields
2. **Device creation**: Create device records with `status=pending`
3. **Enrollment trigger**: Generate enrollment URLs/profiles for each device
4. **Notification**: Email users with enrollment instructions
5. **Status tracking**: Monitor enrollment completion

### Supported Sources

- **Jamf Pro**: Export device inventory, map to Local MDM schema
- **Microsoft Intune**: Export via Graph API, map to Local MDM schema
- **VMware Workspace ONE**: Export device list, map to Local MDM schema
- **CSV**: Manual export from any system

---

## Policy Schema Versioning

### Version Strategy

Each policy has a `schema_version` field that tracks the policy definition format.

```json
{
  "id": "policy-123",
  "name": "Corporate WiFi",
  "schema_version": "1.0",
  "policy_type": "wifi",
  "policy_config": {
    "ssid": "CorpNet",
    "security": "wpa2-enterprise"
  }
}
```

### Version Migration

When policy schema changes (e.g., adding new fields), implement migration:

```go
// internal/policy/migration.go
func MigratePolicy(policy *Policy) error {
    switch policy.SchemaVersion {
    case "1.0":
        return migrateV1ToV2(policy)
    case "2.0":
        return nil // Current version
    default:
        return fmt.Errorf("unknown schema version: %s", policy.SchemaVersion)
    }
}
```

### Backward Compatibility

- **Additive changes**: New optional fields are backward compatible
- **Breaking changes**: Require version bump and migration
- **Deprecation**: Mark fields as deprecated, remove after 2 major versions

### Policy Versioning

Each policy update creates a new version record:

```sql
CREATE TABLE policy_versions (
    id UUID PRIMARY KEY,
    policy_id UUID REFERENCES policies(id),
    version INT NOT NULL,
    policy_config JSONB NOT NULL,
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
```

**Rollback**: Restore previous version by copying `policy_config` from `policy_versions`.

---

## Data Retention Policies

### Retention Periods

| Data Type | Retention Period | Rationale |
|-----------|------------------|-----------|
| Active devices | Indefinite | Required for management |
| Unenrolled devices | 90 days | Compliance/audit trail |
| Audit logs | 1 year | Compliance requirement |
| Command history | 90 days | Troubleshooting |
| Policy versions | All versions | Rollback capability |
| Certificates | Until expiration + 30 days | Revocation checking |

### Automated Cleanup

**Cron Job** (runs daily):
```sql
-- Delete unenrolled devices older than 90 days
DELETE FROM devices 
WHERE status = 'unenrolled' 
  AND updated_at < NOW() - INTERVAL '90 days';

-- Archive old audit logs
INSERT INTO audit_logs_archive 
SELECT * FROM audit_logs 
WHERE created_at < NOW() - INTERVAL '1 year';

DELETE FROM audit_logs 
WHERE created_at < NOW() - INTERVAL '1 year';

-- Delete old command history
DELETE FROM device_commands 
WHERE created_at < NOW() - INTERVAL '90 days';
```

### Configuration

```yaml
data_retention:
  unenrolled_devices_days: 90
  audit_logs_days: 365
  command_history_days: 90
  enable_auto_cleanup: true
  cleanup_schedule: "0 2 * * *"  # 2 AM daily
```

---

## GDPR Compliance

### Right to Access

**Endpoint**: `GET /api/v1/gdpr/export/{user_email}`

Exports all data associated with a user:
- Device records
- Audit logs (where user is actor)
- Policy assignments
- Command history

**Response**: JSON file with all user data

### Right to Deletion

**Endpoint**: `DELETE /api/v1/gdpr/delete/{user_email}`

Deletes or anonymizes user data:
1. **Soft delete** device records (set `deleted_at`)
2. **Anonymize** audit logs (replace user_id with `<deleted>`)
3. **Remove** personal identifiers (email, name)
4. **Retain** aggregated data for compliance

**Note**: Some data must be retained for legal/compliance reasons (audit logs).

### Data Minimization

- Only collect necessary device information
- Don't store user passwords (use Keycloak)
- Encrypt sensitive fields (serial numbers, device IDs)
- Anonymize logs after retention period

### Consent Management

- User consent tracked in `device_metadata.consent_date`
- Enrollment requires explicit consent
- Consent can be withdrawn (triggers unenrollment)

### Data Processing Agreement

Document in `docs/legal/DPA.md`:
- What data is collected
- How data is processed
- Where data is stored
- Who has access
- Retention periods
- User rights

---

## Database Schema Migrations

### Migration Versioning

Migrations use sequential numbering:
```
migrations/
├── 000001_initial_schema.up.sql
├── 000001_initial_schema.down.sql
├── 000002_add_policy_versions.up.sql
├── 000002_add_policy_versions.down.sql
```

### Migration Testing

Before applying to production:
1. **Test on copy**: Apply to production database copy
2. **Rollback test**: Verify down migration works
3. **Performance test**: Check migration duration
4. **Backup**: Take database backup before migration

### Zero-Downtime Migrations

For large tables, use these strategies:

**Add column** (safe):
```sql
ALTER TABLE devices ADD COLUMN new_field VARCHAR(255);
```

**Remove column** (requires multi-step):
```sql
-- Step 1: Stop writing to column (deploy code)
-- Step 2: Remove column (after verification)
ALTER TABLE devices DROP COLUMN old_field;
```

**Rename column** (requires multi-step):
```sql
-- Step 1: Add new column
ALTER TABLE devices ADD COLUMN new_name VARCHAR(255);

-- Step 2: Backfill data
UPDATE devices SET new_name = old_name;

-- Step 3: Deploy code using new column
-- Step 4: Drop old column
ALTER TABLE devices DROP COLUMN old_name;
```

---

## API Versioning Strategy

### Current Approach

- All endpoints under `/api/v1/`
- Breaking changes require new version (`/api/v2/`)
- Non-breaking changes added to existing version

### Breaking vs Non-Breaking Changes

**Non-Breaking** (safe to add):
- New optional fields in request
- New fields in response
- New endpoints
- New query parameters

**Breaking** (requires new version):
- Removing fields from response
- Changing field types
- Removing endpoints
- Changing authentication method
- Changing error response format

### Deprecation Process

1. **Announce**: Document deprecated endpoint/field
2. **Warning**: Add `Deprecated` header to responses
3. **Sunset**: Set sunset date (minimum 6 months)
4. **Remove**: Remove in next major version

### Version Support

- **Current version** (v1): Fully supported
- **Previous version** (v0): Supported for 12 months after v1 release
- **Older versions**: Not supported

---

## Backup Recommendations

### What to Backup

**Critical** (must backup):
- PostgreSQL database (all tables)
- Secrets directory (`secrets/`)
- Root CA private key
- APNs certificates

**Optional** (can regenerate):
- Configuration files (in git)
- Application binaries (can rebuild)
- Logs (if archived elsewhere)

### Backup Frequency

- **Database**: Continuous (WAL archiving) + daily snapshots
- **Secrets**: Daily encrypted backup to S3
- **Certificates**: On creation/renewal

### Restore Testing

- Test restore monthly
- Document restore procedure
- Measure RTO (Recovery Time Objective): < 1 hour
- Measure RPO (Recovery Point Objective): < 15 minutes

**Note**: Database backup/restore is handled by PostgreSQL infrastructure (RDS, managed PostgreSQL, etc.) and is outside the scope of Local MDM application.

---

## Future Enhancements

- Multi-region data replication
- Point-in-time recovery for policies
- Automated compliance reporting
- Data anonymization for test environments
- Cross-enterprise data sharing (with consent)
