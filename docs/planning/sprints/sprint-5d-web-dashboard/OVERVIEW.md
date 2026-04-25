# Sprint 5d: Web Dashboard

**Status**: 🟡 In Progress (Phase 2 polish)  
**Branch**: `sprint-5d/web-dashboard`  
**Duration**: Started 2026-04-24  
**Goal**: Web-based admin dashboard for device management, policy management, and compliance monitoring  
**Depends on**: Sprint 5b (EventBus), Sprint 5f (API hardening) — both merged into branch  
**Stack**: Go HTML templates + HTMX v2.0.9 + Tailwind CSS v4.2.4 — no separate frontend build pipeline

---

## Completed

### Infrastructure
- [x] Go HTML template engine with `embed.FS`, `Clone()` for block overrides
- [x] HTMX v2.0.9 vendored at `web/static/js/htmx.min.js`
- [x] Tailwind CSS v4.2.4 standalone CLI — compiled in Dockerfile (arch-aware: arm64/x64 musl)
- [x] CSP nonces (URL-safe base64) for inline scripts/styles — no `unsafe-inline`
- [x] HMAC-SHA256 signed session cookies (prevents forgery)
- [x] CSRF protection on all POST forms (HMAC token, HTMX requests exempt)
- [x] Keycloak OIDC login/logout flow (code exchange, SSO session termination)
- [x] `KC_HTTP_PORT: 8180` — same port internal/external (requires `/etc/hosts` entry for `keycloak`)
- [x] Enterprise ID claim mapper in Keycloak JWT
- [x] Auto-run migrations on container startup (`docker/entrypoint.sh`)
- [x] Seed data: 55 devices, 8 policies, 3 groups, compliance results, audit logs
- [x] Favicon SVG shield with "MDM"

### Dashboard Home
- [x] Stat cards: Total Devices, Enrolled, Non-Compliant, Active Policies
- [x] SVG pie charts: Platforms, Device Status, Compliance (server-rendered, fixed-size, HTML legends)
- [x] Chart hover: cross-highlight between pie slices and legend items
- [x] "Needs Attention" panel: non-compliant devices + devices not seen in 7+ days
- [x] Recent Activity feed (last 5 audit entries)

### Devices
- [x] List with server-side sorting (all 6 columns), pagination (50/page), platform/status filters, debounced search
- [x] Name-as-link pattern (no separate "View" column)
- [x] Detail page: 3-column layout (Hardware, OS, Enrollment)
- [x] Tabbed section: Compliance (per-setting rows), Policies (all effective with "Assigned Via"), Commands
- [x] Actions: Lock, Unenroll, Wipe, Delete (with confirmation dialogs)
- [x] Auto-evaluate compliance via EventBus on policy assignment
- [x] Manual "Re-evaluate" button (circular arrow icon) in Enrollment card
- [x] "Last Evaluated" timestamp in Enrollment card

### Policies
- [x] List with sortable name column, platform filter
- [x] Settings catalog: 15 settings across Security/Restrictions/WiFi/VPN categories
- [x] Platform-aware filtering (Cross-Platform shows only `platforms: ["all"]` settings)
- [x] Debounced filter input with CSP-safe event binding
- [x] Create/Edit with checkbox/input/select per setting (no raw JSON)
- [x] Invalid setting keys rejected on submit
- [x] Policy type auto-detected from selected settings
- [x] Assignment count + "Manage Assignments" link on edit page
- [x] Assign page: dynamic text search for groups/devices, filter out already-assigned
- [x] Delete with assignment check (blocked if assigned, error message shown)
- [x] Assign/Delete as styled buttons

### Groups
- [x] List with sortable name column, name-as-link, delete button
- [x] Create group form (toggle visibility, CSP-safe)
- [x] Detail page: inline edit name/description (Edit/Save/Cancel)
- [x] Member toggle: all enterprise devices listed with "In Group" / "Add" buttons
- [x] Debounced device filter in member list

### Compliance
- [x] Summary cards: Compliant/Non-Compliant/Unknown (clickable toggles, toggle off on re-click)
- [x] Table with sortable columns (device, policy, status, evaluated_at)
- [x] Text search filter, "Clear Filter" button
- [x] Real violation details from DB (not just status)
- [x] Server-side pagination (50/page)

### Audit Log
- [x] Parsed details: `key: value; key: value` format, truncated at 100 chars
- [x] Expandable detail rows (▶ arrow, styled key/value table card)
- [x] User email resolution (users table → details fallback → truncated UUID)
- [x] Action filter with debounce, date range filters
- [x] Server-side pagination (50/page)
- [x] `audit_logs.user_id` FK dropped (migration 000012) — allows OIDC users not in users table

### Dark Mode
- [x] Toggle in header (sun/moon icon), persists via localStorage
- [x] `dark:` variants on all components, cards, tables, badges, sidebar, header
- [x] Tailwind v4 `@variant dark` configured

### Mobile
- [x] Responsive sidebar: hidden on mobile, hamburger menu toggle with backdrop overlay
- [x] Playwright mobile viewport test (375px)

### Playwright Browser Tests
- [x] 78/78 passing
- [x] Real Keycloak login/logout (no cookie bypass)
- [x] Console error tracking (JS errors, page errors, HTTP 4xx/5xx)
- [x] Viewport auto-resize for mobile sections
- [x] Covers: login, dashboard, devices (list/sort/filter/search/detail), policies (list/create/edit/assign), groups (list/create/detail), compliance (filters/toggles), audit (filter), mobile hamburger, logout

---

## Remaining

### Low Priority (deferred to next session)
- [ ] Playwright full CRUD workflow rewrite + cleanup (create→edit→delete cycles with verification)
- [ ] Visual polish — page transitions, loading indicators on HTMX requests, toast notifications
- [ ] HTMX content replacement navigation (sidebar links swap main content without full reload)
- [ ] Playwright multi-select/checkbox testing

### Known Issues (tracked in SESSION_NOTES.md)
- **EventBus compliance evaluation is fire-and-forget** — no retry on failure. Fix: `event_queue` table + background retry worker (5 attempts, then audit log failure).
- **Session cookie HMAC key uses Keycloak client secret** — if secret rotates, sessions invalidate. Fix: dedicated `session_secret` config key, source from AWS SSM in prod.

---

## Architecture

```
Browser
  ↕ HTML (full pages + HTMX partial fragments)
Go Server (:8080)
  ├── internal/api/templates/     ← Go HTML templates (embed.FS)
  ├── internal/api/web_handlers.go ← dashboard handlers (return HTML)
  ├── internal/api/web_handlers_pages.go ← policy/group/compliance/audit handlers
  ├── internal/api/web_session.go  ← OIDC session, CSRF, HMAC cookies
  ├── internal/api/web_templates.go ← template engine, helper functions
  ├── internal/api/web_charts.go   ← SVG pie chart generator
  ├── internal/api/web_policy_catalog.go ← settings catalog
  ├── web/static/js/htmx.min.js  ← vendored HTMX v2.0.9
  ├── web/static/css/input.css    ← Tailwind v4 source
  ├── web/static/css/output.css   ← compiled CSS (built in Dockerfile)
  └── web/static/favicon.svg      ← shield icon
```

Dashboard handlers are separate from API handlers — API returns JSON, dashboard returns HTML. Both use the same services, repos, and auth middleware.

## Commits (sprint-5d/web-dashboard)

Key commits (not exhaustive):
- S5d-06: Seed data (25 devices, policies, groups, compliance, audit)
- S5d-01: Project setup (templates, HTMX, Tailwind, routes)
- S5d-07: Playwright browser playbook
- S5d: Keycloak port alignment (8180 internal+external)
- S5d: HMAC-SHA256 signed session cookies
- S5d: Policy settings catalog with filter/checkbox UI
- S5d: Group management (create/delete/edit/member toggle)
- S5d: Sortable columns, pagination, compliance filters
- S5d: Dashboard SVG pie charts
- S5d: Device detail 3-column layout with tabs
- S5d: Dark mode, favicon, mobile hamburger menu
- S5d: CSRF protection, device delete, needs-attention dashboard
- S5d: Auto-run migrations on container startup
- S5d: Compliance per-setting rows, EventBus trigger fix

*Created: 2026-04-18*  
*Updated: 2026-04-24 — Sprint in progress, Phase 2 polish*
