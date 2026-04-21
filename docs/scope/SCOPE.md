# Project Scope - Local MDM

**Version**: 1.0  
**Last Updated**: 2026-02-05  
**Status**: Planning Phase

## Executive Summary

Local MDM is a unified, multi-platform Mobile Device Management solution designed to manage Windows, macOS, and Android devices without requiring custom agent installation. The system leverages native OS capabilities and open protocols to provide enterprise-grade device management.

## Goals

### Primary Goals

1. **Multi-Platform Support**: Single platform to manage Windows 10/11, macOS, and Android devices
2. **Agent-Less Architecture**: Utilize native OS MDM capabilities where possible
3. **Open Source Foundation**: Build on open protocols and integrate with existing open-source solutions
4. **Self-Hosted**: No dependency on cloud services (Azure, Google Workspace, etc.) for core functionality
5. **Enterprise Ready**: Support multi-tenancy, RBAC, audit logging, and compliance reporting

### Secondary Goals

1. **Developer Friendly**: Well-documented APIs, clear architecture, extensible design
2. **Operational Simplicity**: Easy deployment, minimal dependencies, clear monitoring
3. **Cost Effective**: Reduce licensing costs compared to commercial MDM solutions

## In Scope

### Platform Support

#### Windows 10/11
- ✅ Agent-less enrollment via native MDM client
- ✅ OMA-DM protocol implementation
- ✅ MS-MDE2 enrollment protocol
- ✅ Core CSP support:
  - DeviceInfo (inventory)
  - Policy/Config (settings)
  - WiFi configuration
  - VPN configuration
  - AppManagement (app deployment)
  - DeviceLock (security policies)
- ✅ Certificate-based authentication
- ✅ Manual enrollment (Settings → Access work or school)
- ✅ Provisioning packages (.ppkg) support (Sprint 3)

#### macOS
- ✅ Integration with nanoMDM (as library)
- ✅ Manual enrollment via configuration profiles
- ✅ DEP/ADE support via nanoDEP (Sprint 2)
- ✅ Configuration profile management
- ✅ Core MDM commands:
  - DeviceInformation
  - InstallApplication
  - RemoveApplication
  - DeviceLock
  - EraseDevice
  - InstallProfile
  - RemoveProfile
- ✅ APNs certificate management
- ✅ Platform SSO integration with Keycloak (Sprint 4)

#### Android
- ✅ Google Android Management API integration
- ✅ QR code enrollment
- ✅ Token-based enrollment
- ✅ Work Profile management
- ✅ Fully Managed Device mode
- ✅ Dedicated Device mode
- ✅ App distribution via managed Google Play
- ✅ Policy enforcement (restrictions, security settings)
- ✅ Webhook integration for device events

### Core Features

#### Device Management
- Device enrollment and provisioning
- Device inventory and status tracking
- Remote lock and wipe capabilities
- Device grouping and organization
- Compliance monitoring
- Last seen/heartbeat tracking

#### Policy Management
- Policy creation and templates
- Policy assignment to devices/groups
- Platform-specific policy translation
- Policy versioning and rollback
- Compliance policy enforcement

#### Application Management
- App deployment and removal
- App inventory tracking
- Managed app configuration
- App update management
- Platform-specific app stores integration

#### User Management
- Admin user authentication
- Role-based access control (RBAC)
- Multi-tenant support
- Audit logging
- API token management

#### Certificate Management
- Internal PKI for device certificates
- Certificate issuance and renewal
- Certificate revocation
- APNs certificate management (macOS)
- TLS certificate management

#### API & Integration
- RESTful API for all operations
- Webhook support for events
- API documentation (OpenAPI/Swagger)
- CLI tools for administration (Sprint 5)
- Prometheus metrics endpoint (Sprint 5)

### Technical Requirements

#### Backend
- Language: Go 1.21+
- Database: PostgreSQL 15+
- Migration tool: golang-migrate or similar
- API framework: gorilla/mux or chi
- Authentication: JWT-based

#### Security
- TLS 1.3 for all endpoints
- Certificate-based device authentication
- Encrypted database credentials
- Audit logging for all operations
- Rate limiting and DDoS protection

#### Deployment
- Docker support
- Docker Compose for development
- Production deployment tooling — ECS Fargate (future, see F-02)
- Health check endpoints (liveness and readiness)
- Prometheus metrics (Sprint 5)
- Structured JSON logging (Sprint 5)

## Out of Scope (v1.0)

### Features
- ❌ iOS/iPadOS support (future consideration)
- ❌ Linux desktop management
- ❌ Chrome OS management
- ❌ Advanced reporting and analytics dashboard
- ❌ Email/SMS notifications
- ❌ Geofencing and location tracking
- ❌ Content management (documents, media)
- ❌ Mobile threat defense integration
- ❌ Zero-touch enrollment for Windows (Autopilot alternative)
- ❌ SCEP server (use external SCEP if needed)

### Integrations
- ❌ Active Directory integration (future phase)
- ❌ LDAP/SAML SSO (future phase)
- ❌ Third-party app stores
- ❌ Ticketing system integration
- ❌ SIEM integration

### Platforms
- ❌ Windows 7/8 (no native MDM support)
- ❌ macOS versions older than 10.13
- ❌ Android versions older than 8.0

## Success Criteria

### Phase 1: Foundation (Weeks 1-2)
- [ ] Project structure established
- [ ] Database schema designed and migrated
- [ ] Basic API server running
- [ ] Authentication system functional
- [ ] Certificate infrastructure operational

### Phase 2: Windows Support (Weeks 3-5)
- [ ] Windows device can discover MDM server
- [ ] Windows device can enroll successfully
- [ ] Device information collected and stored
- [ ] At least one policy (WiFi) can be deployed
- [ ] Device can be remotely locked

### Phase 3: macOS Support (Weeks 6-7)
- [ ] nanoMDM integrated
- [ ] macOS device can enroll via profile
- [ ] Configuration profile can be deployed
- [ ] App installation command works
- [ ] Device inventory visible in API

### Phase 4: Android Support (Weeks 8-9)
- [ ] Google Management API configured
- [ ] QR code enrollment functional
- [ ] Work Profile can be created
- [ ] App can be deployed from managed Play
- [ ] Device policy enforced

### Phase 5: Unified Features (Weeks 10-12)
- [ ] Single API manages all three platforms
- [ ] Policy abstraction layer functional
- [ ] Basic web dashboard operational
- [ ] API documentation complete
- [ ] Deployment guide written

## Constraints

### Technical Constraints
- Must support PostgreSQL (no proprietary databases)
- Must be deployable on-premises
- Must not require internet access for core functionality (except Android Management API)
- Must support standard TLS certificates

### Business Constraints
- Open source license (MIT)
- No paid dependencies for core functionality
- Must be maintainable by small team (1-3 developers)

### Platform Constraints
- Windows: Requires Windows 10 Pro or higher
- macOS: Requires APNs certificate from Apple
- Android: Requires Google Play Services on devices

## Assumptions

1. Target users have basic understanding of MDM concepts
2. Network infrastructure supports HTTPS traffic
3. Devices have internet connectivity during enrollment
4. Admin users can obtain necessary certificates (APNs for macOS)
5. For Android, organization can register with Google's Android Management API

## Risks & Mitigations

| Risk | Impact | Probability | Mitigation |
|------|--------|-------------|------------|
| Protocol documentation incomplete | High | Medium | Start with well-documented protocols, iterate with real devices |
| nanoMDM API changes | Medium | Low | Pin to specific version, monitor releases |
| Google API deprecation | High | Low | Follow Google's migration guides, maintain flexibility |
| Certificate management complexity | High | Medium | Use established crypto libraries, comprehensive testing |
| Multi-platform testing overhead | Medium | High | Automate where possible, prioritize one platform at a time |

## Dependencies

### External Services
- Apple Push Notification Service (APNs) - Required for macOS
- Google Android Management API - Required for Android
- Public Certificate Authority - Optional, can use self-signed for testing

### Open Source Projects
- nanoMDM - macOS MDM server
- golang-migrate - Database migrations
- PostgreSQL - Database

### Development Tools
- Go 1.21+
- Docker & Docker Compose
- Make
- Git

## Timeline

**Total Duration**: 12 weeks (3 months)

- **Weeks 1-2**: Foundation and infrastructure
- **Weeks 3-5**: Windows MDM implementation
- **Weeks 6-7**: macOS integration
- **Weeks 8-9**: Android integration
- **Weeks 10-12**: Unification and polish

## Stakeholders

- **Development Team**: Implementation and maintenance
- **System Administrators**: Deployment and operations
- **End Users**: Device enrollment and compliance
- **Security Team**: Security review and compliance

## Approval

This scope document should be reviewed and approved before beginning implementation.

---

**Document Control**
- Created: 2026-02-05
- Author: Kiro AI Assistant
- Review Status: Draft
- Next Review: After Phase 1 completion
