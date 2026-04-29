# Sprint 6 Dashboard Cleanup

## Context
Branch: `main`. Dashboard is functional but has rough edges found during Sprint 6 device testing and the S6-13 audit. This prompt collects UI/UX items that don't affect backend functionality.

Read `.kiro/steering/STEERING.md` and `.kiro/steering/SESSION_NOTES.md` for project conventions.
Dashboard uses Go templates + HTMX + Tailwind CSS. No React.

## Items

### 1. Pending Enrollments visibility
Devices with `status = 'pending'` are invisible in the main device list. Add a "Pending Enrollments (N)" indicator on the devices page — clicking it shows a filtered view with device_id, platform, serial, and created_at. This helps admins see devices that started enrollment but haven't completed it.

### 2. Empty state SVG inline styles → Tailwind
The empty state illustrations on devices, groups, and policies pages use inline `style=` attributes instead of Tailwind classes (workaround from failed centering attempts). Replace with Tailwind utilities (`text-center`, `mx-auto`, `mb-2`, `block`, `mt-2`, `mt-1`) and run `make css` to rebuild. Affected files:
- `internal/api/templates/pages/devices.html`
- `internal/api/templates/pages/groups.html`
- `internal/api/templates/pages/policies.html`

