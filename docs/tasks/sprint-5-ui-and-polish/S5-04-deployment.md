# S5-04: Deployment & Operations Guide

**Sprint**: 5 — UI & Polish
**Parallel**: ✅ Yes
**Effort**: 2-3 days

## Tasks

### 1. Deployment Guide
- Docker Compose (development)
- Docker Compose (production with TLS, Keycloak, SCEP)
- Bare metal / systemd deployment
- Reverse proxy configuration (nginx/caddy) with TLS termination
- Files: `docs/user/DEPLOYMENT.md`

### 2. Operations Guide
- Backup and restore (PostgreSQL, CA certificates, Keycloak realm)
- Certificate renewal procedures (APNs, CA, TLS, DEP tokens)
- Monitoring and health checks
- Log management
- Files: `docs/user/OPERATIONS.md`

### 3. Enrollment Guides (per platform)
- macOS enrollment guide (manual + DEP)
- Windows enrollment guide (Settings → Access work or school)
- Android enrollment guide (QR code)
- Files: `docs/user/enrollment/MACOS.md`, `WINDOWS.md`, `ANDROID.md`

### 4. Troubleshooting Guide
- Common enrollment failures
- Certificate issues
- Connectivity problems
- Files: `docs/user/TROUBLESHOOTING.md`

## Acceptance Criteria

- [ ] Fresh deployment from docs works end-to-end
- [ ] Backup/restore procedure tested
- [ ] Enrollment guides tested with real devices
