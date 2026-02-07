# Weblogin SSO Extension (macOS)

> Source: [github.com/unioslo/weblogin-mac-sso-extension](https://github.com/unioslo/weblogin-mac-sso-extension)

A macOS Platform SSO Extension developed at the University of Oslo. This is the client-side companion to the [Keycloak PSSO Extension](keycloak-psso-extension.md).

## What It Does

Installed on managed Macs, this extension:

- Handles device and user registration with Keycloak via Secure Enclave keys
- Intercepts authentication requests to the Keycloak IdP from Safari, native apps, etc.
- Provides passwordless login to Keycloak after initial registration
- Supports Touch ID and password fallback for re-authentication

## Requirements

- macOS with Secure Enclave (T2 chip or Apple Silicon)
- Device must be MDM-enrolled (attestation requires MDM profile)
- Keycloak server with the [PSSO extension](keycloak-psso-extension.md) installed
- Companion MDM profile to configure the SSO Extension

## Known Limitations

- Secure Enclave authentication method only
- Works poorly with Keycloak required actions during re-authentication
- Limited SAML flow testing

## MDM Profile Requirement

This extension requires an MDM configuration profile to be deployed to managed Macs. This is where Local MDM comes in — we'll push this profile as part of macOS enrollment/policy.

The MDM profile configures:
- The SSO Extension identifier and team ID
- The Keycloak IdP URL
- Associated domains for the extension
- Platform SSO settings (authentication method, token behavior)

## Build & Install

Compile with Xcode and install on target Macs. See the [wiki](https://github.com/unioslo/weblogin-mac-sso-extension/wiki) for configuration details specific to your environment.
