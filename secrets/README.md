# Secrets Directory

This directory contains sensitive credentials for local development.

**⚠️ NEVER commit secrets to version control!**

## Setup for Local Development

Create the following files:

```bash
# Database password
echo "postgres" > secrets/db_password

# JWT secret (generate random 256-bit key)
openssl rand -base64 32 > secrets/jwt_secret

# Keycloak client secret
echo "localmdm-api-secret" > secrets/keycloak_client_secret
```

## Files

- `db_password` - PostgreSQL password
- `jwt_secret` - JWT signing key (256-bit)
- `keycloak_client_secret` - Keycloak OIDC client secret
- `apns_cert.pem` - APNs certificate (macOS push) - optional
- `apns_key.pem` - APNs private key - optional
- `ca_key.pem` - Root CA private key - auto-generated

## Production

In production, use AWS Secrets Manager or SSM Parameter Store:

```bash
# Example: AWS Secrets Manager
aws secretsmanager get-secret-value --secret-id localmdm/db_password
```

See `docs/deployment/SECRETS.md` for production setup.

## Environment Variables

Secrets can also be provided via environment variables (takes precedence):

```bash
export DB_PASSWORD="postgres"
export JWT_SECRET="your-secret-key"
export KEYCLOAK_CLIENT_SECRET="your-client-secret"
```
