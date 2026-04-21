# Backup Guide

## What to Back Up

### 1. PostgreSQL Database
The database contains all device, policy, user, compliance, and audit data.

```bash
# Full backup
pg_dump -h localhost -U postgres -d localmdm -F c -f localmdm_$(date +%Y%m%d).dump

# Restore
pg_restore -h localhost -U postgres -d localmdm -c localmdm_20260421.dump
```

Schedule daily backups. For AWS RDS, automated backups are enabled by default.

### 2. CA Certificates and Keys
Located in the path configured by `certificates.ca_cert_path` and `certificates.ca_key_path`.

Default dev location: `internal/api/certs/`
- `ca.crt` — CA certificate (public)
- `ca.key` — CA private key (**critical secret**)

**Loss of the CA key means all device certificates become unverifiable.** Back up to a secure location (AWS Secrets Manager in production).

### 3. Secrets Directory
`secrets/` contains service credentials:
- `dep_encryption.key` — DEP token encryption key
- `google-service-account.json` — Android Management API credentials

In production, these are stored in AWS SSM Parameter Store.

### 4. Configuration
- `configs/config.yaml` — Server configuration
- `docker-compose.yml` — Service definitions
- `docker/keycloak/realm-export.json` — Keycloak realm configuration

### 5. APNs Certificate
The Apple Push Notification certificate (uploaded separately) is required for macOS MDM push notifications. Store a copy securely — Apple limits reissuance.

## Backup Schedule

| Item | Frequency | Retention |
|------|-----------|-----------|
| PostgreSQL | Daily | 30 days |
| CA certs/keys | On change | Permanent |
| Secrets | On change | Permanent |
| Config files | On change | 5 versions |
| Keycloak realm | Weekly | 4 weeks |

## Disaster Recovery

1. Provision new infrastructure (ECS + RDS)
2. Restore PostgreSQL from backup
3. Deploy CA certs and secrets to SSM
4. Deploy application containers
5. Verify `/health/ready` returns healthy
6. Test enrollment with a single device
