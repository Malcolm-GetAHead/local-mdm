# Sprint 5b: Web Dashboard

**Duration**: 1-2 weeks  
**Goal**: Web-based admin dashboard for device management, policy management, and compliance monitoring  
**Depends on**: Sprint 5 complete (API docs, CLI tools, and backend polish provide a stable API surface)  
**Stack**: Go HTML templates + HTMX + Tailwind CSS — no separate frontend build pipeline

---

## Why HTMX over React

**Decision (2026-04-20):** The dashboard is read-heavy, form-based CRUD — tables, status indicators, forms. HTMX handles this with zero JavaScript build toolchain:

- **Same repo, same language** — Go templates live alongside the backend code. No Node.js, no npm, no webpack.
- **No separate build pipeline** — templates are compiled into the Go binary. One deploy artifact.
- **Simpler for the team** — HTML + Go templates is closer to Python/Jinja than React/JSX.
- **14KB JS** — HTMX is a single vendored JS file vs 100KB+ React bundle.
- **Server-rendered** — handlers return HTML fragments for partial page updates. Same auth middleware, same server, same port.

React would be better for highly interactive real-time UIs (drag-and-drop, live collaboration, offline-capable). An MDM dashboard doesn't need any of that.

---

## Why a Separate Sprint

The dashboard adds HTML templates, CSS, and HTMX patterns that are distinct from the Go backend API work. Keeping it separate:

- Lets the API stabilize in Sprint 5 before building a UI on top of it
- Keeps Sprint 5 focused on backend polish (reporting, testing, CLI)
- Dashboard can be parallelized with Sprint 5 if desired

## Architecture

```
Browser
  ↕ HTML (full pages + HTMX partial fragments)
Go Server
  ├── internal/api/templates/     ← Go HTML templates
  ├── internal/api/web_handlers.go ← dashboard handlers (return HTML)
  ├── web/static/htmx.min.js     ← vendored HTMX (~14KB)
  └── web/static/styles.css       ← Tailwind CSS (CDN or vendored)
```

Dashboard handlers are separate from API handlers — API returns JSON, dashboard returns HTML. Both use the same services and auth middleware.

## Tasks

| ID | Task | Effort |
|---|---|---|
| S5b-01 | Project setup (templates, HTMX, Tailwind, auth) | 1 day |
| S5b-02 | Login & navigation (Keycloak redirect, sidebar, layout) | 1 day |
| S5b-03 | Device management (list, detail, lock/wipe actions) | 2-3 days |
| S5b-04 | Policy management (list, create/edit, assign to groups) | 2-3 days |
| S5b-05 | Compliance & reporting (dashboard, audit log viewer) | 1-2 days |

### S5b-01: Project Setup
- Go HTML template layout (base template, partials, components)
- Vendor HTMX JS into `web/static/`
- Tailwind CSS via CDN (or vendored for production)
- Keycloak OIDC login redirect (reuse existing auth middleware)
- Template helper functions (format dates, status badges, pagination)
- Embed static files in Go binary via `embed.FS`

### S5b-02: Login & Navigation
- Keycloak login redirect flow
- Role-based navigation (admin sees everything, viewer sees read-only)
- Sidebar: Dashboard, Devices, Policies, Groups, Apps, Audit Logs
- Header: user info, enterprise name, logout
- Base layout template with HTMX boost for SPA-like navigation

### S5b-03: Device Management
- Device list table with HTMX pagination, filtering (platform, status), search
- Device detail page: info, enrolled policies, command history
- Lock/wipe actions with HTMX confirmation dialog
- Platform-specific info display (Windows OMA-DM data, macOS DEP status, Android compliance)
- HTMX partial updates: action buttons swap to status indicators after click

### S5b-04: Policy Management
- Policy list with filtering by platform and type
- Create/edit policy form — settings catalog rendered as checkboxes + value inputs
- Assign policy to device or group (HTMX-powered select + submit)
- Policy status per device (pending, applied, failed)

### S5b-05: Compliance & Reporting
- Compliance dashboard: compliant/non-compliant counts per enterprise
- Device inventory table with CSV export link
- Audit log viewer with search by actor, action, date range
- HTMX infinite scroll or pagination for large result sets

## HTMX Patterns

```html
<!-- Device list with pagination -->
<table id="device-list">
  <tr hx-get="/dashboard/devices?page=2" hx-target="#device-list" hx-swap="innerHTML">
    <!-- rows -->
  </tr>
</table>

<!-- Lock device with confirmation -->
<button hx-post="/dashboard/devices/{{.ID}}/lock"
        hx-confirm="Lock this device?"
        hx-target="#device-status"
        hx-swap="outerHTML">
  Lock Device
</button>

<!-- Search with debounce -->
<input type="search" name="q"
       hx-get="/dashboard/devices"
       hx-trigger="keyup changed delay:300ms"
       hx-target="#device-list">
```

## Definition of Done

- [ ] Admin logs in via Keycloak
- [ ] Dashboard shows device counts and compliance summary
- [ ] Device list with pagination, filtering, and search
- [ ] Remote lock/wipe from device detail page
- [ ] Policy create, edit, and assign from UI
- [ ] Compliance view shows non-compliant devices with reasons
- [ ] Audit log searchable by actor and date
- [ ] All pages work without JavaScript disabled (graceful degradation)
- [ ] Embedded in Go binary — single deploy artifact

---

*Created: 2026-04-18 — Split from Sprint 5 (S5-01)*  
*Updated: 2026-04-20 — Changed from React to HTMX + Go templates*
