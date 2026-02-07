# S5-01: Web Dashboard

**Sprint**: 5 — UI & Polish
**Parallel**: ✅ Yes
**Depends on**: All API endpoints from Sprints 1-4
**Effort**: 6-8 days

## Tasks

### 1. Project Setup
- Framework choice (React, Vue, or Svelte — recommend React for ecosystem)
- Keycloak JS adapter for OIDC login (redirect flow with PKCE)
- API client with auth token injection
- Files: `web/` directory

### 2. Core Pages
- Login (redirect to Keycloak)
- Dashboard overview (device counts by platform/status, compliance summary)
- Device list (filterable, sortable, paginated)
- Device detail (info, compliance, command history, installed apps)
- Policy list and editor
- Group management
- Audit log viewer

### 3. Actions
- Remote lock / wipe from device detail
- Push profile from device detail
- Deploy app from device detail
- Create/edit policy with form builder

### 4. Responsive Design
- Mobile-friendly layout
- Accessibility compliant (WCAG 2.1 AA)

## Acceptance Criteria

- [ ] Admin can log in via Keycloak
- [ ] Dashboard shows device counts and compliance summary
- [ ] Device list loads with pagination and filtering
- [ ] Remote lock can be triggered from UI
- [ ] Policy can be created and assigned from UI
