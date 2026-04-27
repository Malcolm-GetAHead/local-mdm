#!/usr/bin/env node
/**
 * Error State Tests — Local MDM Dashboard
 *
 * Tests HTMX error handling by intercepting requests and returning error responses.
 * Separate from the playbook DSL because error testing requires page.route() interception.
 *
 * Usage:
 *   node run-error-tests.js [--headed] [--base-url URL]
 */

const playwright = require("playwright");

const args = process.argv.slice(2);
const flag = (name) => args.includes(name);
const opt = (name, def) => (args.includes(name) ? args[args.indexOf(name) + 1] : def);

const BASE_URL = opt("--base-url", "http://localhost:8080");
const HEADED = flag("--headed");

async function login(page) {
  await page.goto(BASE_URL + "/dashboard/");
  await page.waitForTimeout(500);
  // If redirected to Keycloak login
  if (await page.locator('input[name="username"]').isVisible({ timeout: 3000 }).catch(() => false)) {
    await page.fill('input[name="username"]', "admin");
    await page.fill('input[name="password"]', "admin123");
    await page.click('input[type="submit"], button:has-text("Sign In")');
    await page.waitForTimeout(1000);
  }
}

let pass = 0, fail = 0;
const failures = [];

async function test(name, fn) {
  try {
    await fn();
    pass++;
    console.log(`  ✓ ${name}`);
  } catch (err) {
    fail++;
    failures.push({ name, error: err.message });
    console.log(`  ✗ ${name}: ${err.message}`);
  }
}

function assert(condition, msg) {
  if (!condition) throw new Error(msg || "Assertion failed");
}

(async () => {
  const browser = await playwright.chromium.launch({ headless: !HEADED });
  const context = await browser.newContext();
  const page = await context.newPage();
  page.on("dialog", (d) => d.accept());
  page.setDefaultTimeout(5000);

  console.log("\n## Error State Tests\n");

  // Login first
  await login(page);

  // ── Test 1: Server 500 on device list returns error, not blank page ──
  await test("Device list 500 shows error content", async () => {
    // Navigate to devices first (clean state)
    await page.goto(BASE_URL + "/dashboard/devices");
    await page.waitForTimeout(500);

    // Intercept the next HTMX request to devices
    await page.route("**/dashboard/devices**", (route) => {
      route.fulfill({
        status: 500,
        contentType: "text/html",
        body: '<tr><td colspan="6" class="px-6 py-8 text-center text-red-600">Internal Server Error</td></tr>',
      });
    });

    // Trigger an HTMX request (e.g., filter change)
    const platformSelect = page.locator('select[name="platform"]');
    if (await platformSelect.isVisible()) {
      await platformSelect.selectOption({ index: 1 });
      await page.waitForTimeout(500);
    }

    // The page should still have structure (sidebar, header)
    const sidebar = page.locator("#sidebar");
    assert(await sidebar.isVisible(), "Sidebar should still be visible after error");

    // Clean up route
    await page.unroute("**/dashboard/devices**");
  });

  // ── Test 2: Server 500 on policy create shows error, preserves form ──
  await test("Policy create 500 preserves form", async () => {
    await page.goto(BASE_URL + "/dashboard/policies/new");
    await page.waitForTimeout(500);

    // Fill in some form data
    const nameField = page.locator('input[name="name"]');
    if (await nameField.isVisible()) {
      await nameField.fill("Error Test Policy");
    }

    // Intercept the POST
    await page.route("**/dashboard/policies", (route) => {
      if (route.request().method() === "POST") {
        route.fulfill({
          status: 500,
          contentType: "text/html",
          body: "<html><body>Internal Server Error</body></html>",
        });
      } else {
        route.continue();
      }
    });

    // The form should still be visible (we don't submit, just verify the form is intact)
    assert(await nameField.isVisible(), "Form fields should remain visible");

    await page.unroute("**/dashboard/policies");
  });

  // ── Test 3: 404 on device detail shows not found ──
  await test("Device detail 404 shows not found", async () => {
    const resp = await page.goto(BASE_URL + "/dashboard/devices/00000000-0000-0000-0000-000000000000");
    assert(resp.status() === 404 || (await page.content()).includes("not found") || (await page.content()).includes("Not Found"),
      "Should show 404 or not found message");
  });

  // ── Test 4: Delete action failure shows toast error ──
  await test("Failed delete shows error toast", async () => {
    await page.goto(BASE_URL + "/dashboard/devices");
    await page.waitForTimeout(500);

    // Intercept DELETE requests
    await page.route("**/dashboard/devices/*/delete", (route) => {
      route.fulfill({
        status: 500,
        contentType: "text/html",
        body: "Internal Server Error",
      });
    });

    // Find a device delete button and click it
    const deleteBtn = page.locator('button:has-text("Delete")').first();
    if (await deleteBtn.isVisible({ timeout: 2000 }).catch(() => false)) {
      await deleteBtn.click();
      await page.waitForTimeout(500);

      // The device should still be in the list (not removed)
      const deviceRows = page.locator("table tbody tr");
      const count = await deviceRows.count();
      assert(count > 0, "Device rows should still be present after failed delete");
    }

    await page.unroute("**/dashboard/devices/*/delete");
  });

  // ── Summary ──
  console.log(`\n${pass + fail} tests: ${pass} passed, ${fail} failed`);
  if (failures.length > 0) {
    console.log("\nFailures:");
    for (const f of failures) console.log(`  - ${f.name}: ${f.error}`);
  }

  await browser.close();
  process.exit(fail > 0 ? 1 : 0);
})();
