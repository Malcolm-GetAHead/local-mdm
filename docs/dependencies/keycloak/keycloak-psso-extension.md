# Keycloak Platform Single Sign-On Extension

> Source: [github.com/unioslo/keycloak-psso-extension](https://github.com/unioslo/keycloak-psso-extension)

A Keycloak extension that makes it compliant with [Apple Platform Single Sign-on for macOS](https://support.apple.com/en-ca/guide/deployment/dep7bbb05313/web).

## Requirements

- Keycloak 26.4+
- PostgreSQL or MariaDB backend (adds a custom JPA entity / table for device data)

## Features

- Device attestation via Apple Secure Enclave (only enrolled macOS devices accepted)
- User registration credential management in Keycloak admin and account consoles
- Device management API for MDM integration

## Known Limitations

- Secure Enclave authentication method only
- Requires a client named `psso` (not yet configurable)
- Client must be public, include `urn:apple:platformsso` scope
- "Revoke Refresh Token" must be OFF (default)
- No UI/API for full device lifecycle management yet

## Keycloak Configuration

### 1. Create client scope

Create a scope called `urn:apple:platformsso`. It can be empty (no mappers needed).

### 2. Create the `psso` client

- Type: OIDC, non-confidential (public)
- Valid redirect URI: `weblogin-sso://idp-login-redirect`
- Assigned scopes: `urn:apple:platformsso`, `offline_access` (optional, as optional scope)
- If using `offline_access`: set `Access token lifespan` to a higher value (e.g. 8 hours) under Client > Advanced

### 3. Enable Required Action

Authentication menu → Required Actions → enable the PSSO required action so users see credentials in their account console.

### 4. Add Authenticator to flow

Add the PSSO authenticator to your browser authentication flow, right under the Cookies authenticator.

### 5. Apple App Site Association

Serve this at `/.well-known/apple-app-site-association` on your Keycloak domain:

```json
{
  "authsrv": {
    "apps": ["<TEAMID>.no.uio.WebloginSSO"]
  },
  "webcredentials": {
    "apps": ["<TEAMID>.no.uio.WebloginSSO"]
  }
}
```

Replace the bundle identifier if using a custom SSO Extension build.

## API Endpoints

All endpoints are under `/realms/<realm>/psso`:

| Endpoint | Purpose |
|---|---|
| `/nonce` | Nonce generation for login requests |
| `/token` | Login request processing / login response |
| `/enroll` | Device registration |
| `/userenroll` | User registration |

### Device Management API

Requires a Bearer token from a `psso-admin` client with `mac-admin` role.

| Endpoint | Method | Description |
|---|---|---|
| `/device` | GET | List all registered devices |
| `/device/{serial}` | GET | Query one device |
| `/device/{serial}` | DELETE | Remove a device |

## Authentication Flow

### Device Registration

1. PSSO Extension generates Secure Enclave keys (signing + encryption)
2. Extension requests nonce from `/nonce`
3. Extension prompts user to authenticate (gets access token via OIDC)
4. Extension sends keys, attestation, nonce, and access token to `/enroll`
5. Keycloak verifies attestation against Apple Root CA, extracts serial number and UDID
6. Device persisted as custom JPA entity in Keycloak DB

### User Registration

1. Extension sends attestation, nonce, access token, and user key to `/userenroll`
2. Keycloak verifies attestation and stores user key as a Keycloak credential (`CredentialModel`)
3. User can see/manage this credential in their Keycloak account console

### SSO Authentication (subsequent logins)

1. SSO Extension intercepts authentication request (SAML or OIDC)
2. Extension creates a signed JWT envelope containing a token, signed by device key
3. Envelope sent in `Platform-SSO-Authorization` header
4. Keycloak authenticator verifies envelope signature, introspects token, attaches user to session
5. If token expired or re-auth required (`ForceAuthn`, `prompt=login`): extension triggers local auth (Touch ID / password), sends new login request for fresh tokens

## Token Behavior

- With `offline_access` scope: no `refresh_token_expires_in` in login response; SSO Extension sends `id_token` for auth. Access token lifespan controls re-auth interval.
- Without `offline_access`: `refresh_token_expires_in` included; SSO Extension sends `refresh_token` for auth.
