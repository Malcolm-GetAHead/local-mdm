# S4-04: macOS Platform SSO Profile & Keycloak PSSO

**Sprint**: 4 — Policy & Identity
**Parallel**: ✅ Yes (independent of policy tasks)
**Depends on**: S2-01 (macOS enrollment), S1-04 (Keycloak running)
**Effort**: 4-5 days

## Objective

Push an MDM configuration profile to macOS devices that configures the Weblogin SSO Extension to authenticate against our Keycloak instance with Platform SSO.

## Tasks

### 1. Keycloak PSSO Extension Deployment
- Install `keycloak-psso-extension` JAR into Keycloak providers
- Verify PSSO endpoints available: `/realms/{realm}/psso/nonce`, `/token`, `/enroll`, `/userenroll`
- Configure `psso` client (already created in S1-04)
- Add PSSO authenticator to browser auth flow
- Update Docker Compose / deployment to include the extension
- Files: `docker/keycloak/providers/keycloak-psso-extension.jar`, update `docker-compose.yml`

### 2. Platform SSO Configuration Profile
- Generate macOS configuration profile with:
  - Extensible SSO payload (SSO Extension identifier, team ID, URLs)
  - Associated Domains payload (for `/.well-known/apple-app-site-association`)
  - Platform SSO settings (authentication method: Secure Enclave, token behavior)
- Configurable per-enterprise (Keycloak realm URL, client settings)
- Files: `internal/platform/macos/profiles/psso.go`

### 3. Apple App Site Association
- Serve `/.well-known/apple-app-site-association` JSON at Keycloak domain
- Contains SSO Extension bundle identifier and team ID
- Files: `internal/api/handlers/apple_site_assoc.go` or Keycloak reverse proxy config

### 4. Profile Push via MDM
- Push PSSO profile to enrolled macOS devices via InstallProfile command
- Can be assigned as part of a policy or pushed on enrollment
- Files: update `internal/platform/macos/commands.go`

### 5. Weblogin SSO Extension Build (documentation)
- Document how to build the Weblogin SSO Extension from source
- Document how to sign with enterprise team ID
- Document distribution via MDM (or direct install for dev)
- Files: `docs/dependencies/keycloak/building-sso-extension.md`

## Key Reference Docs
- [Keycloak PSSO Extension](../../dependencies/keycloak/keycloak-psso-extension.md)
- [Weblogin SSO Extension](../../dependencies/keycloak/weblogin-sso-extension.md)
- [Integration Notes](../../dependencies/keycloak/integration-notes.md)

## Acceptance Criteria

- [ ] Keycloak PSSO endpoints respond correctly
- [ ] Platform SSO configuration profile generated with correct payloads
- [ ] Profile pushed to macOS device via MDM
- [ ] SSO Extension registers device with Keycloak (device attestation)
- [ ] SSO Extension registers user with Keycloak
- [ ] User can authenticate to Keycloak via SSO Extension (passwordless after registration)
