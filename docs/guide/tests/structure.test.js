import { test, expect } from "bun:test";
import { readGuide, loadPure } from "./extract.js";

test("has a title element", () => {
  expect(readGuide()).toMatch(/<title>[^<]+<\/title>/);
});

test("carries no document wrapper tags (the artifact platform supplies them)", () => {
  const html = readGuide();
  expect(html).not.toMatch(/<!DOCTYPE/i);
  expect(html).not.toMatch(/<html[\s>]/i);
  expect(html).not.toMatch(/<head[\s>]/i);
  expect(html).not.toMatch(/<body[\s>]/i);
});

test("references no external resources (strict CSP)", () => {
  const html = readGuide();
  expect(html).not.toMatch(/(?:src|href)\s*=\s*["']https?:/i);
  expect(html).not.toMatch(/(?:src|href)\s*=\s*["']\/\//);
  expect(html).not.toMatch(/@import/i);
});

test("makes no network calls", () => {
  const html = readGuide();
  expect(html).not.toMatch(/\bfetch\s*\(/);
  expect(html).not.toMatch(/XMLHttpRequest/);
  expect(html).not.toMatch(/new\s+WebSocket/);
});

test("states the honest build status", () => {
  expect(readGuide()).toMatch(/spec v0\.9\.4 · 0 lines of Go/);
});

test("defines a pure script block", () => {
  expect(readGuide()).toMatch(/<script id="guide-pure">/);
});

function routesInMarkup(html, attr) {
  return new Set([...html.matchAll(new RegExp(`${attr}="([^"]+)"`, "g"))].map((m) => m[1]));
}

test("every declared route has exactly one content article", () => {
  const pure = loadPure();
  const articles = routesInMarkup(readGuide(), "data-route");
  for (const r of pure.ROUTES) expect(articles.has(r.route)).toBe(true);
});

test("every content article is a declared route", () => {
  const declared = new Set(loadPure().ROUTES.map((r) => r.route));
  for (const route of routesInMarkup(readGuide(), "data-route")) {
    expect(declared.has(route)).toBe(true);
  }
});

test("route ids are unique", () => {
  const routes = loadPure().ROUTES.map((r) => r.route);
  expect(new Set(routes).size).toBe(routes.length);
});

test("every route belongs to one of the six parts", () => {
  const parts = new Set(["world", "edge", "why", "build", "go", "runbook"]);
  for (const r of loadPure().ROUTES) expect(parts.has(r.part)).toBe(true);
});

test("every article carries at least one citation chip", () => {
  const html = readGuide();
  const articles = [...html.matchAll(/<article data-route="([^"]+)"[\s\S]*?<\/article>/g)];
  expect(articles.length).toBeGreaterThan(0);
  for (const [block, route] of articles) {
    // The calculator is a control, not prose: its text is bound from GuidePure.UI so
    // it can be localized without duplicating its 16 element ids. Its chips live in
    // UI.calc.intro instead of the markup — asserted separately, below.
    if (route === "edge/calculator") continue;
    expect(block, `article ${route} has no citation chip`).toMatch(/class="cite" data-src="/);
  }
});

test("the calculator's citations live in the string table", () => {
  const intro = loadPure().UI.calc.intro.en;
  expect(intro).toContain('class="cite" data-src="core §2.1"');
  expect(intro).toContain('class="cite" data-src="core §2.2"');
});

// Drift against a future review round is detected by grepping the raw file:
//   grep -o 'data-src="[^"]*"' index.html | sort | uniq -c
// A chip written into a JS string with escaped quotes (data-src=\"…\") renders fine
// but is invisible to that grep, so every chip must survive as literal file text.
test("every citation chip is visible to the raw-text drift check", () => {
  const html = readGuide();
  const inFile = new Set([...html.matchAll(/data-src="([^"]*)"/g)].map((m) => m[1]));
  const bundle = loadPure().UI.calc.intro;
  for (const lang of Object.keys(bundle)) {
    for (const m of bundle[lang].matchAll(/data-src="([^"]*)"/g)) {
      expect(inFile.has(m[1]), `UI.calc.intro.${lang} chip "${m[1]}" is escaped in the file`).toBe(true);
    }
  }
  expect(html).not.toMatch(/data-src=\\"/);
});

test("Part 1 covers the six topics the design names", () => {
  const routes = new Set(loadPure().ROUTES.map((r) => r.route));
  for (const r of [
    "world/perps", "world/funding", "world/prices",
    "world/liquidation", "world/venues", "world/counterparties"
  ]) {
    expect(routes.has(r)).toBe(true);
  }
});

test("styles both themes, with data-theme overriding the OS preference", () => {
  const html = readGuide();
  expect(html).toMatch(/@media \(prefers-color-scheme: dark\)/);
  expect(html).toMatch(/:root\[data-theme="dark"\]/);
  expect(html).toMatch(/:root\[data-theme="light"\]/);
});

test("wide content is wrapped in a horizontal scroll container", () => {
  const html = readGuide();
  const tables = [...html.matchAll(/<table[\s>]/g)].length;
  const wrappers = [...html.matchAll(/class="scroll-x"/g)].length;
  expect(wrappers).toBeGreaterThanOrEqual(tables);
});

test("interactive controls carry accessible names", () => {
  const html = readGuide();
  expect(html).toMatch(/<input class="search"[^>]*aria-label=/);
  expect(html).toMatch(/<nav class="rail"[^>]*aria-label=/);
  expect(html).toMatch(/<button class="theme-toggle"[^>]*aria-label=/);
});

test("the only embedded resource is the inlined Georgian font", () => {
  const html = readGuide();
  const urls = [...html.matchAll(/url\(\s*(['"]?)([^)'"]*)\1\s*\)/g)].map((m) => m[2]);
  expect(urls.length).toBeGreaterThan(0);
  for (const u of urls) {
    expect(u.startsWith("data:font/woff2;base64,"), `non-inlined resource: ${u.slice(0, 40)}`).toBe(true);
  }
});

test("declares a Georgian face scoped to the Georgian unicode blocks", () => {
  const html = readGuide();
  expect(html).toMatch(/font-family:\s*"Noto Serif Georgian"/);
  expect(html).toMatch(/unicode-range:\s*U\+10A0-10FF/i);
  expect(html).toMatch(/:lang\(ka\)/);
});

test("both Georgian weights are present", () => {
  const faces = [...readGuide().matchAll(/@font-face\s*\{[\s\S]*?\}/g)].map((m) => m[0]);
  expect(faces.length).toBe(2);
  expect(faces.some((f) => /font-weight:\s*400/.test(f))).toBe(true);
  expect(faces.some((f) => /font-weight:\s*600/.test(f))).toBe(true);
});

test("Part 2 covers the five topics the design names", () => {
  const routes = new Set(loadPure().ROUTES.map((r) => r.route));
  for (const r of [
    "edge/two-trades", "edge/costs", "edge/calculator", "edge/gate", "edge/forks"
  ]) {
    expect(routes.has(r)).toBe(true);
  }
});
