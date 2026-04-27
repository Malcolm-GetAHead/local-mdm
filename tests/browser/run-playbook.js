#!/usr/bin/env node
/**
 * Browser Test Playbook Runner — Local MDM Dashboard
 *
 * Reads tests/browser/browser-playbook.md and executes each step using Playwright.
 * Adapted from dev-deployer-htmx pattern.
 *
 * Usage:
 *   node run-playbook.js [--section "Name"] [--headed] [--browser firefox] [--base-url URL]
 */

const fs = require("fs");
const path = require("path");
const playwright = require("playwright");

const args = process.argv.slice(2);
const flag = (name) => args.includes(name);
const opt = (name, def) => (args.includes(name) ? args[args.indexOf(name) + 1] : def);

const BASE_URL = opt("--base-url", "http://localhost:8080");
const HEADED = flag("--headed");
const BROWSER = opt("--browser", "chromium");
const SECTION_FILTER = opt("--section", null);
const FIXTURES = path.join(__dirname, "fixtures");

// ── Field name → locator mapping ─────────────────────────────────────────────
// The playbook uses logical names; map to actual HTML inputs.

const FIELD_MAP = {
  search: 'input[type="search"], input[name="q"], input[placeholder*="Search" i]',
  name: 'input[name="name"], textbox[name="Name" i]',
  description: 'input[name="description"], textarea[name="description"]',
  platform: 'select[name="platform"]',
  policy_type: 'select[name="policy_type"]',
  username: 'input[name="username"]',
  password: 'input[name="password"]',
};

async function fillField(page, key, value) {
  const k = key.toLowerCase();

  // Try select elements first (dropdowns)
  if (k === "platform" || k === "policy_type" || k === "status") {
    const sel = page.locator(`select[name="${k}"]`);
    if ((await sel.count()) > 0) { await sel.selectOption({ label: value }); return; }
  }

  // Try FIELD_MAP selectors
  if (FIELD_MAP[k]) {
    for (const sel of FIELD_MAP[k].split(", ")) {
      const match = sel.match(/textbox\[name="([^"]+)"( i)?\]/);
      if (match) {
        const loc = page.getByRole("textbox", { name: new RegExp(match[1].replace(/[.*+?^${}()|[\]\\]/g, '\\$&'), match[2] ? "i" : undefined) });
        if ((await loc.count()) > 0) { await loc.first().fill(value); return; }
      } else {
        const loc = page.locator(sel);
        if ((await loc.count()) > 0) { await loc.first().fill(value); return; }
      }
    }
  }

  // Fallback: try role textbox with name
  const byRole = page.getByRole("textbox", { name: new RegExp(key, "i") });
  if ((await byRole.count()) > 0) { await byRole.first().fill(value); return; }

  // Fallback: try placeholder
  const byPlaceholder = page.locator(`[placeholder*="${key}" i]`);
  if ((await byPlaceholder.count()) > 0) { await byPlaceholder.first().fill(value); return; }

  // Fallback: input by name attribute
  const byName = page.locator(`input[name="${key.toLowerCase().replace(/ /g, '_')}"]`);
  if ((await byName.count()) > 0) { await byName.first().fill(value); return; }

  throw new Error(`Cannot find field "${key}"`);
}

// ── Step interpreter ─────────────────────────────────────────────────────────

async function runStep(page, step, ctx) {
  const s = step.trim();

  // ── Visit URL ──
  const visitMatch = s.match(/^Visit `([^`]+)`/);
  if (visitMatch) {
    const url = visitMatch[1].startsWith("http") ? visitMatch[1] : BASE_URL + visitMatch[1];
    await page.goto(url);
    await page.waitForTimeout(800);
    const titleMatch = s.match(/title should be "([^"]+)"/);
    if (titleMatch) assert((await page.title()) === titleMatch[1], `title "${await page.title()}" !== "${titleMatch[1]}"`);
    const containsMatch = s.match(/page contains "([^"]+)"/);
    if (containsMatch) assert(await page.isVisible(`text=${containsMatch[1]}`), `page missing "${containsMatch[1]}"`);
    return;
  }

  // ── Navigate via nav link ──
  if (s.match(/^Navigate to/)) {
    const nameMatch = s.match(/"([^"]+)"/);
    if (nameMatch) {
      await page.getByRole("link", { name: nameMatch[1] }).first().click();
      await page.waitForTimeout(500);
    }
    return;
  }

  // ── Wait ──
  const waitMatch = s.match(/^Wait ([\d.]+)s/);
  if (waitMatch) { await page.waitForTimeout(parseFloat(waitMatch[1]) * 1000); return; }

  // ── Accept dialog ──
  if (s.match(/^Accept/i)) return;

  // ── Select option from dropdown ──
  const selectMatch = s.match(/^Select "([^"]+)" from (?:the )?"([^"]+)" dropdown/);
  if (selectMatch) {
    const byLabel = page.getByLabel(selectMatch[2]);
    if ((await byLabel.count()) > 0) {
      await byLabel.selectOption({ label: selectMatch[1] });
    } else {
      const sel = page.locator(`select`).filter({ has: page.locator(`option:has-text("${selectMatch[1]}")`) });
      await sel.first().selectOption({ label: selectMatch[1] });
    }
    await page.waitForTimeout(500);
    return;
  }

  // ── Fill fields ──
  const fillMatch = s.match(/^Fill:\s*(.+)/);
  if (fillMatch) {
    const pairs = fillMatch[1].split(/,\s*/);
    for (const pair of pairs) {
      const eqIdx = pair.indexOf("=");
      const key = pair.slice(0, eqIdx).trim().replace(/^`|`$/g, "");
      const val = pair.slice(eqIdx + 1).trim().replace(/^`|`$/g, "");
      await fillField(page, key, val);
    }
    return;
  }

  // ── Click ──
  const clickMatch = s.match(/^[Cc]lick "([^"]+)"(.*)/);
  if (clickMatch && !s.startsWith("Verify")) {
    const name = clickMatch[1];
    const rest = clickMatch[2] || "";
    let clicked = false;

    // Scoped: 'Click "Delete" on "foo"'
    const scopeMatch = rest.match(/(?:on|for) "([^"]+)"/);
    if (scopeMatch) {
      const container = page.locator("tr, [class*='card']").filter({ hasText: scopeMatch[1] });
      for (const role of ["link", "button"]) {
        const loc = container.getByRole(role, { name });
        if ((await loc.count()) > 0) { await loc.first().click(); clicked = true; break; }
      }
    }

    if (!clicked) {
      for (const role of ["button", "link"]) {
        const loc = page.getByRole(role, { name });
        if ((await loc.count()) > 0) { await loc.first().click(); clicked = true; break; }
      }
    }

    if (!clicked) {
      const textLoc = page.locator(`text="${name}"`).first();
      if ((await textLoc.count()) > 0) { await textLoc.click(); clicked = true; }
    }

    if (!clicked) throw new Error(`Cannot find clickable "${name}"`);
    await page.waitForTimeout(300);

    const verifyInline = rest.match(/verify (.+)$/i);
    if (verifyInline) await runVerify(page, verifyInline[1], ctx);
    return;
  }

  // ── Verify ──
  if (s.match(/^Verify/i)) {
    await runVerify(page, s.replace(/^Verify\s+/i, ""), ctx);
    return;
  }

  console.log(`    ⚠️  Unhandled: ${s}`);
}

// ── Verify interpreter ───────────────────────────────────────────────────────

async function runVerify(page, c, ctx) {
  c = c.trim();

  // page title
  if (c.match(/page title is/)) {
    const expected = c.match(/"([^"]+)"/)[1];
    assert((await page.title()) === expected, `title "${await page.title()}" !== "${expected}"`);
    return;
  }

  // redirected + contains
  const redirMatch = c.match(/redirected to `([^`]+)`.*contains "([^"]+)"/);
  if (redirMatch) {
    assert(page.url().includes(redirMatch[1]), `URL ${page.url()} missing ${redirMatch[1]}`);
    assert(await page.isVisible(`text=${redirMatch[2]}`), `page missing "${redirMatch[2]}"`);
    return;
  }

  // form/modal appears with heading
  if (c.match(/(?:form|modal) (?:appears|opens) with heading/)) {
    const heading = c.match(/"([^"]+)"/)[1];
    await page.waitForSelector(`h2:has-text("${heading}"), h3:has-text("${heading}")`, { timeout: 3000 });
    return;
  }

  // noscript message
  if (c.match(/noscript/i)) {
    const html = await page.content();
    assert(html.includes("<noscript>"), "missing <noscript> tag");
    return;
  }

  // toast appears
  if (c.match(/toast appears/)) {
    await page.waitForTimeout(500);
    const toastText = c.match(/with "([^"]+)"/);
    if (toastText) {
      assert(await page.isVisible(`text=${toastText[1]}`), `toast missing "${toastText[1]}"`);
    }
    return;
  }

  // table header visible
  if (c.match(/table header.*visible/i)) {
    assert(await page.isVisible("thead th, th"), "table headers not visible");
    return;
  }

  // table row with text
  if (c.match(/table row appears with text "([^"]+)"/)) {
    const text = c.match(/"([^"]+)"/)[1];
    assert(await page.isVisible(`text=${text}`), `"${text}" not visible`);
    return;
  }

  // field contains value
  const fieldMatch = c.match(/(\w+) field contains "([^"]+)"/);
  if (fieldMatch) {
    const loc = page.getByRole("textbox", { name: new RegExp(fieldMatch[1], "i") }).first();
    assert((await loc.inputValue()) === fieldMatch[2], `${fieldMatch[1]} value mismatch`);
    return;
  }

  // text visible
  const textVisibleMatch = c.match(/"([^"]+)" is visible/);
  if (textVisibleMatch) {
    assert(await page.isVisible(`text=${textVisibleMatch[1]}`), `"${textVisibleMatch[1]}" not visible`);
    return;
  }

  // text not visible
  const notVisibleMatch = c.match(/"([^"]+)" is (?:no longer|not) visible/);
  if (notVisibleMatch) {
    assert(!(await page.isVisible(`text=${notVisibleMatch[1]}`)), `"${notVisibleMatch[1]}" still visible`);
    return;
  }

  // count check
  const countMatch = c.match(/(\d+) (?:rows?|items?|devices?|policies?) (?:are |is )?(?:visible|shown|displayed)/);
  if (countMatch) {
    const expected = parseInt(countMatch[1]);
    const rows = await page.locator("tbody tr").count();
    assert(rows === expected, `expected ${expected} rows, got ${rows}`);
    return;
  }

  console.log(`    ⚠️  Unhandled verify: ${c}`);
}

function assert(cond, msg) { if (!cond) throw new Error(msg); }

// ── Playbook parser ──────────────────────────────────────────────────────────

function parsePlaybook(mdPath) {
  const lines = fs.readFileSync(mdPath, "utf-8").split("\n");
  const sections = [];
  let cur = null;
  for (const line of lines) {
    const h2 = line.match(/^## (.+)/);
    if (h2 && !line.startsWith("###")) { cur = { name: h2[1].trim(), subs: [{ name: null, steps: [] }] }; sections.push(cur); continue; }
    const h3 = line.match(/^### (.+)/);
    if (h3 && cur) { cur.subs.push({ name: h3[1].trim(), steps: [] }); continue; }
    const step = line.match(/^- \[ \] (.+)/);
    if (step && cur) cur.subs[cur.subs.length - 1].steps.push(step[1].trim());
  }
  return sections;
}

// ── Main ─────────────────────────────────────────────────────────────────────

(async () => {
  const playbookPath = path.join(__dirname, "browser-playbook.md");
  if (!fs.existsSync(playbookPath)) {
    console.log("No browser-playbook.md found. Create it in tests/browser/ to define test steps.");
    console.log("See Sprint 5d plan for playbook DSL reference.");
    process.exit(0);
  }

  const sections = parsePlaybook(playbookPath);
  if (sections.length === 0) {
    console.log("Playbook is empty — no test sections found.");
    process.exit(0);
  }

  const browser = await playwright[BROWSER].launch({ headless: !HEADED });
  const context = await browser.newContext();
  const page = await context.newPage();
  page.on("dialog", (d) => d.accept());
  page.setDefaultTimeout(5000);

  // Track browser console errors and failed resource loads
  const consoleErrors = [];
  page.on("console", (msg) => {
    const text = msg.text();
    if (text.includes("Content Security Policy") || text.includes("CSP")) {
      consoleErrors.push(`[CSP] ${text}`);
      return;
    }
    if (msg.type() === "error") {
      if (text.includes("favicon") || text.includes("manifest")) return;
      // Skip generic "Failed to load resource" — we track those via response events
      if (text.includes("Failed to load resource")) return;
      consoleErrors.push(`[${page.url().split("/").slice(3).join("/")}] ${text}`);
    }
  });
  page.on("pageerror", (err) => {
    consoleErrors.push(`[${page.url().split("/").slice(3).join("/")}] JS Error: ${err.message}`);
  });
  page.on("response", (resp) => {
    if (resp.status() >= 400) {
      const url = resp.url();
      if (url.includes("favicon")) return;
      consoleErrors.push(`HTTP ${resp.status()}: ${url}`);
    }
  });

  let pass = 0, fail = 0;
  const failures = [];

  for (const section of sections) {
    if (SECTION_FILTER && !section.name.toLowerCase().includes(SECTION_FILTER.toLowerCase())) continue;

    // Resize viewport if section name contains a pixel width like "Mobile (375px)"
    const vpMatch = section.name.match(/\((\d+)px\)/);
    if (vpMatch) {
      await page.setViewportSize({ width: parseInt(vpMatch[1]), height: 812 });
    } else {
      await page.setViewportSize({ width: 1280, height: 800 });
    }

    console.log(`\n## ${section.name}`);
    for (const sub of section.subs) {
      if (sub.name) console.log(`\n### ${sub.name}`);
      const ctx = {};
      for (const step of sub.steps) {
        try {
          await runStep(page, step, ctx);
          pass++;
          console.log(`  ✅ ${step}`);
        } catch (e) {
          fail++;
          console.log(`  ❌ ${step}`);
          console.log(`     → ${e.message.split("\n")[0]}`);
          failures.push(`${section.name}${sub.name ? " > " + sub.name : ""}: ${step}\n    → ${e.message.split("\n")[0]}`);
        }
      }
    }
  }

  await browser.close();
  console.log(`\n${"─".repeat(60)}`);
  console.log(`${pass} passed, ${fail} failed`);
  if (failures.length) { console.log("\nFailures:"); failures.forEach((f) => console.log(`  ${f}`)); }
  if (consoleErrors.length) {
    console.log(`\nBrowser console errors (${consoleErrors.length}):`);
    consoleErrors.forEach((e) => console.log(`  ⚠️  ${e}`));
    fail += consoleErrors.length;
  }
  process.exit(fail > 0 ? 1 : 0);
})();
