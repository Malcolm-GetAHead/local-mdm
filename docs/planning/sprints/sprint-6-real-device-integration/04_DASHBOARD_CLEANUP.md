# Sprint 6 Dashboard Cleanup

## Context
Branch: `main`. Dashboard is functional but has rough edges found during Sprint 6 device testing and the S6-13 audit. This prompt collects UI/UX items that don't affect backend functionality.

Read `.kiro/steering/STEERING.md` and `.kiro/steering/SESSION_NOTES.md` for project conventions.
Dashboard uses Go templates + HTMX + Tailwind CSS. No React.

## Workflow
For each item, use a custom Playwright script to visit the relevant page, take a screenshot, verify the fix visually, and iterate. Example:

```js
const { chromium } = require('playwright');
(async () => {
  const browser = await chromium.launch();
  const page = await browser.newPage();
  // Login
  await page.goto('http://localhost:8080/dashboard/');
  await page.fill('input[name="username"]', 'admin');
  await page.fill('input[name="password"]', 'admin123');
  await page.click('text=Sign In');
  await page.waitForTimeout(2000);
  // Navigate and screenshot
  await page.goto('http://localhost:8080/dashboard/devices');
  await page.screenshot({ path: '/tmp/devices.png', fullPage: true });
  await browser.close();
})();
```

Run with `node script.js` from `tests/browser/`. Review screenshots before committing.

## Items

### 1. Pending Enrollments visibility
Devices with `status = 'pending'` are invisible in the main device list. Add a "Pending Enrollments (N)" indicator on the devices page — clicking it shows a filtered view with device_id, platform, serial, and created_at. This helps admins see devices that started enrollment but haven't completed it.

### 2. Empty state SVG inline styles → Tailwind
The empty state illustrations on devices, groups, and policies pages use inline `style=` attributes instead of Tailwind classes (workaround from failed centering attempts). Replace with Tailwind utilities (`text-center`, `mx-auto`, `mb-2`, `block`, `mt-2`, `mt-1`) and run `make css` to rebuild. Affected files:
- `internal/api/templates/pages/devices.html`
- `internal/api/templates/pages/groups.html`
- `internal/api/templates/pages/policies.html`

### 3. Verify `formatBytes` template function
The `formatBytes` helper was added but never verified in a browser. Find where it's used in templates, navigate to that page with Playwright, and screenshot to confirm it renders correctly (e.g., "4.2 GB" not "4200000000").

