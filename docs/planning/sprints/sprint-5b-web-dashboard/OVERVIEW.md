# Sprint 5b: Web Dashboard

**Duration**: 1-2 weeks  
**Goal**: Web-based admin dashboard for device management, policy management, and compliance monitoring  
**Depends on**: Sprint 5 complete (API docs, CLI tools, and backend polish provide a stable API surface)  
**Stack**: React (or similar) frontend, separate build pipeline

---

## Why a Separate Sprint

The web dashboard is a greenfield frontend project with a different technology stack (JavaScript/TypeScript, React, CSS), different build tooling (Node.js, Vite/webpack), and different testing patterns (component tests, browser tests) from the Go backend. Keeping it separate:

- Avoids context-switching between Go and React development
- Lets the API stabilize in Sprint 5 before building a UI on top of it
- Allows the dashboard to be developed as a separate package or even a separate repo
- Can be parallelized with Sprint 5 if a frontend developer is available

## Tasks

| ID | Task | Effort |
|---|---|---|
| S5b-01 | Project setup (React, routing, auth, API client) | 1-2 days |
| S5b-02 | Login & navigation (Keycloak OIDC, sidebar, header) | 1 day |
| S5b-03 | Device management (list, detail, lock/wipe actions) | 2-3 days |
| S5b-04 | Policy management (list, create/edit, assign to devices/groups) | 2-3 days |
| S5b-05 | Compliance & reporting (compliance dashboard, audit log viewer) | 1-2 days |

### S5b-01: Project Setup
- Initialize React project in `web/` directory
- Configure build tooling (Vite recommended)
- Set up API client using OpenAPI spec from S5-03
- Configure Keycloak OIDC authentication (use existing Keycloak realm)
- Proxy API requests to Go backend in development

### S5b-02: Login & Navigation
- Keycloak login redirect flow
- Role-based navigation (admin sees everything, viewer sees read-only)
- Sidebar: Dashboard, Devices, Policies, Groups, Certificates, Audit Logs
- Header: user info, enterprise selector, logout

### S5b-03: Device Management
- Device list with pagination, filtering (platform, status), search
- Device detail: info, enrolled policies, certificates, command history
- Actions: lock, wipe (with confirmation dialog)
- Platform-specific info display (Windows OMA-DM data, macOS DEP status, Android compliance)

### S5b-04: Policy Management
- Policy list with filtering by platform and type
- Create/edit policy form with platform-specific config
- Assign policy to device or group
- Policy status per device (pending, applied, failed)

### S5b-05: Compliance & Reporting
- Compliance dashboard: compliant/non-compliant counts, trend chart
- Device inventory report with CSV export
- Audit log viewer with search by actor, action, date range

## Definition of Done

- [ ] Admin logs in via Keycloak
- [ ] Dashboard shows device counts and compliance summary
- [ ] Device list with pagination, filtering, and search
- [ ] Remote lock/wipe from device detail page
- [ ] Policy create, edit, and assign from UI
- [ ] Compliance view shows non-compliant devices with reasons
- [ ] Audit log searchable by actor and date

## Technical Decisions (to be made at sprint start)

- **Framework**: React (recommended) vs Vue vs Svelte
- **State management**: React Query (recommended for API-heavy app) vs Redux
- **UI library**: Tailwind + headless components vs Material UI vs Ant Design
- **Hosting**: Served by Go backend (embed in binary) vs separate static hosting

---

*Created: 2026-04-18 — Split from Sprint 5 (S5-01)*
