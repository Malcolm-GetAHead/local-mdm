# Keycloak

> Source: [keycloak.org](https://www.keycloak.org/)

Keycloak is an open-source Identity and Access Management (IAM) solution providing OIDC, OAuth 2.0, and SAML 2.0.

## Role in Local MDM

Keycloak serves two purposes in our architecture:

1. **Admin/API authentication** — OIDC provider for the Local MDM control plane (dashboard, REST API). Replaces our custom JWT auth with a standards-based IdP.
2. **macOS Platform SSO** — Combined with the UiO extensions (see below), enables macOS devices to authenticate at the login window using Keycloak credentials, with Secure Enclave-backed passwordless login.

## Components

| Component | Source | Purpose |
|---|---|---|
| Keycloak server | [keycloak.org](https://www.keycloak.org/) | OIDC/OAuth2 identity provider |
| Keycloak PSSO Extension | [unioslo/keycloak-psso-extension](https://github.com/unioslo/keycloak-psso-extension) | Server-side extension making Keycloak Apple Platform SSO compliant |
| Weblogin SSO Extension | [unioslo/weblogin-mac-sso-extension](https://github.com/unioslo/weblogin-mac-sso-extension) | macOS SSO Extension app installed on managed Macs |

## Documentation

- [Keycloak PSSO Extension](keycloak-psso-extension.md) — Server-side Keycloak extension for Apple Platform SSO
- [Weblogin SSO Extension](weblogin-sso-extension.md) — macOS companion SSO Extension
- [Integration Notes](integration-notes.md) — How these components fit into Local MDM
