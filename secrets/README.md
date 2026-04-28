# Secrets Directory

This directory contains sensitive credentials for local development.

**⚠️ NEVER commit secrets to version control!**

## Setup for Local Development

Create the following files:

```bash
# Database password
echo "postgres-dev-password-1234" > secrets/db_password

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
- `ppkg_signing.crt` - Windows .ppkg code signing certificate - auto-generated for dev
- `ppkg_signing.key` - Windows .ppkg code signing private key - auto-generated for dev

## Windows PPKG Signing Certificate

For development, a self-signed code signing certificate is auto-generated on first use.
Windows will show a trust warning when applying packages signed with this cert.

**To use a real code signing certificate in production:**

```bash
# 1. Obtain a code signing certificate from your CA (or purchase one)
# 2. Export as PEM files:
openssl pkcs12 -in your-cert.pfx -clcerts -nokeys -out secrets/ppkg_signing.crt
openssl pkcs12 -in your-cert.pfx -nocerts -nodes -out secrets/ppkg_signing.key

# 3. Configure in config.yaml:
#    windows:
#      ppkg_signing_cert: secrets/ppkg_signing.crt
#      ppkg_signing_key: secrets/ppkg_signing.key
```

For internal enterprise use, you can also use your organization's internal CA
to issue a code signing certificate and distribute the CA cert via Group Policy.

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
export DB_PASSWORD="postgres-dev-password-1234"
export JWT_SECRET="your-secret-key"
export KEYCLOAK_CLIENT_SECRET="your-client-secret"
```
