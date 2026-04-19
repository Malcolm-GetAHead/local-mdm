# Sprint 4b: macOS Platform SSO with Keycloak

**Duration**: 1-2 weeks  
**Goal**: Enable passwordless macOS login via Keycloak using Apple's Platform SSO  
**Depends on**: Sprint 4 complete (policy system for pushing SSO profiles), S2-01 (macOS enrollment)

---

## Why a Separate Sprint

Platform SSO involves three distinct deliverables across three different technology stacks:

1. **Go backend** — MDM profile generation and delivery (builds on existing MDM infrastructure)
2. **Java** — Keycloak PSSO extension JAR (Keycloak SPI, separate build pipeline)
3. **Swift** — macOS SSO Extension app (Xcode project, Apple Developer signing)

Keeping this separate from Sprint 4's pure Go policy work avoids context-switching and lets each piece get proper attention.

---

## Tasks

| ID | Task | Stack | Effort |
|---|---|---|---|
| S4b-01 | Keycloak PSSO Extension | Java | 2-3 days |
| S4b-02 | macOS SSO Extension App | Swift | 2-3 days |
| S4b-03 | PSSO MDM Profile Generation & Delivery | Go | 1-2 days |
| S4b-04 | Integration Testing & Documentation | All | 1-2 days |

### S4b-01: Keycloak PSSO Extension
- Implement Keycloak SPI for device-based authentication
- PSSO token endpoint for macOS SSO Extension
- Device registration and validation
- Deploy as JAR to Keycloak providers directory

### S4b-02: macOS SSO Extension App
- Implement `ASAuthorizationSingleSignOnProvider` protocol
- Register with Keycloak PSSO endpoint
- Handle authentication challenges
- Requires Apple Developer account and enterprise signing

### S4b-03: PSSO MDM Profile Generation & Delivery
- Generate Extensible SSO configuration profile
- Push via MDM command (InstallProfile) using Sprint 4 policy system
- Apple App Site Association serving

### S4b-04: Integration Testing
- End-to-end: macOS device enrolls → receives PSSO profile → SSO Extension registers → user authenticates passwordless
- Unenrollment removes device from Keycloak PSSO

## Prerequisites
- Apple Developer account with enterprise team ID
- Keycloak 23+ (already in docker-compose)
- Real or virtual macOS device for testing

## Definition of Done
- [ ] macOS device receives Platform SSO profile via MDM
- [ ] SSO Extension registers device with Keycloak
- [ ] User authenticates passwordless on macOS
- [ ] Device unenrollment removes PSSO registration

---

*Moved from Sprint 4 (S4-04) on 2026-04-18 to reduce context-switching.*
