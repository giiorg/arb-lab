# Interactive Guide — Stage 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build and publish stage 1 of the interactive guide — the navigable shell (routing, search, theme) plus Part 1 "The world" and Part 2 "The edge, honestly" with a working economics calculator whose arithmetic is verified against five figures printed in the spec.

**Architecture:** One self-contained HTML file, `docs/guide/index.html`, with two inline script blocks. `#guide-pure` holds DOM-free logic and exposes a single `GuidePure` object; `#guide-ui` does all DOM wiring and consumes it. That split is what makes the logic testable: the Bun test suite reads the HTML, extracts only the pure block, evaluates it, and asserts against it — no DOM, no jsdom, no build step. A second test suite asserts the structural constraints (self-containment, route integrity, citation coverage) directly against the file text.

**Tech Stack:** Vanilla HTML/CSS/JS (no framework, no bundler, no dependencies). Bun's built-in test runner (`bun test`) — already installed, nothing added to the repo. Mermaid via the Artifact platform's native rendering (stage 2; not used in stage 1).

## Global Constraints

Every task's requirements implicitly include these. Values are copied verbatim from `docs/superpowers/specs/2026-07-25-interactive-guide-design.md`.

- **Teach-and-point, never restate.** The guide explains why a control exists and how it composes; it never becomes a second place where a requirement is written down.
- **Every factual claim carries a citation chip.** Format: `<span class="cite" data-src="core §2.4">core §2.4</span>`. A chip is a rendered inline text label, **not a hyperlink** — a published artifact cannot reach local markdown. A claim with no chip is a defect.
- **No invented numbers.** Every quantity traces to the docs. Illustrative figures are labelled illustrative.
- **Honest about non-existence.** The header carries the marker `spec v0.9.4 · 0 lines of Go`. No section may read as though the system is running.
- **Self-contained, CSP-strict.** No CDN scripts, no external stylesheets, no web fonts, no remote images, no `fetch`/XHR/WebSocket. All CSS and JS inline; any asset a data URI.
- **No `<!DOCTYPE>`, `<html>`, `<head>`, or `<body>` tags.** The Artifact platform wraps the file in that skeleton at publish time. A `<title>` element in the content is correct and expected.
- **No build step.** The file is authored, committed, and published as-is.
- **Failure first, control second.** State what goes wrong, then the mechanism that prevents it — the order the system was actually designed in.
- **Name the thing that is hard.** Where the spec made a genuinely difficult call, say why the obvious alternative fails.
- **Plain language for the domain, precise language for the mechanism.** Part 1 assumes no trading background.
- **No cheerleading.** The predicted milestone-2 outcome is failure; say so wherever it is relevant.
- **Responsive.** Relative units; no horizontal body scroll at 375 px; wide content scrolls inside its own `overflow-x: auto` container.
- **Theme-aware.** `prefers-color-scheme` for the default; `:root[data-theme="dark"]` and `:root[data-theme="light"]` overrides must win in both directions.

---

## File Structure

| File | Responsibility |
|---|---|
| `docs/guide/index.html` | The entire guide: markup, inline `<style>`, inline `<script id="guide-pure">` (DOM-free logic), inline `<script id="guide-ui">` (DOM wiring) |
| `docs/guide/README.md` | How to run the tests and how the file is published |
| `docs/guide/tests/extract.js` | Test helper: reads `index.html`, returns its raw text and the evaluated `GuidePure` object |
| `docs/guide/tests/structure.test.js` | Constraint tests: self-containment, no wrapper tags, route integrity, citation coverage |
| `docs/guide/tests/economics.test.js` | Arithmetic tests for `GuidePure.computeEconomics` against the five spec figures |
| `docs/guide/tests/search.test.js` | Tests for `GuidePure.buildSearchIndex` and `GuidePure.searchIndex` |

The test directory sits beside the artifact deliberately: the Artifact tool publishes only `index.html`, so extra files in the directory are inert.

---

### Task 1: Test harness and the walking skeleton

Establishes the file, the extraction helper, and the constraint suite. Nothing renders yet beyond a title and one section — the point is that the constraints are enforced from the first commit rather than retrofitted.

**Files:**
- Create: `docs/guide/index.html`
- Create: `docs/guide/README.md`
- Create: `docs/guide/tests/extract.js`
- Test: `docs/guide/tests/structure.test.js`

**Interfaces:**
- Consumes: nothing.
- Produces: `readGuide(): string` and `loadPure(): object` from `tests/extract.js`; the markup conventions `<a class="nav-link" data-route="…">`, `<article data-route="…">`, `<span class="cite" data-src="…">` that every later task relies on.

- [ ] **Step 1: Write the extraction helper**

Create `docs/guide/tests/extract.js`:

```js
import { readFileSync } from "node:fs";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";

const GUIDE_PATH = join(dirname(fileURLToPath(import.meta.url)), "..", "index.html");

export function readGuide() {
  return readFileSync(GUIDE_PATH, "utf8");
}

// Extracts the DOM-free script block and evaluates it in isolation.
// The block must define `var GuidePure = (function () { ... })();`
export function loadPure() {
  const html = readGuide();
  const match = html.match(/<script id="guide-pure">([\s\S]*?)<\/script>/);
  if (!match) throw new Error("no <script id=\"guide-pure\"> block found in index.html");
  return new Function(`${match[1]}\n;return GuidePure;`)();
}
```

- [ ] **Step 2: Write the failing structure test**

Create `docs/guide/tests/structure.test.js`:

```js
import { test, expect } from "bun:test";
import { readGuide } from "./extract.js";

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
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `cd docs/guide && bun test`
Expected: FAIL — `ENOENT` on `index.html`, because the file does not exist yet.

- [ ] **Step 4: Write the minimal guide file**

Create `docs/guide/index.html`:

```html
<title>Funding-Rate Arbitrage — The Whole Thing</title>

<style>
  :root {
    --bg: #fbfaf8; --fg: #1a1a1a; --muted: #5a5a5a;
    --rule: #e2ded7; --accent: #7a4a1e; --chip-bg: #efeae1;
  }
  @media (prefers-color-scheme: dark) {
    :root {
      --bg: #17171a; --fg: #e8e6e3; --muted: #9a9791;
      --rule: #2e2e33; --accent: #d9a066; --chip-bg: #24242a;
    }
  }
  :root[data-theme="light"] {
    --bg: #fbfaf8; --fg: #1a1a1a; --muted: #5a5a5a;
    --rule: #e2ded7; --accent: #7a4a1e; --chip-bg: #efeae1;
  }
  :root[data-theme="dark"] {
    --bg: #17171a; --fg: #e8e6e3; --muted: #9a9791;
    --rule: #2e2e33; --accent: #d9a066; --chip-bg: #24242a;
  }

  * { box-sizing: border-box; }
  .guide {
    background: var(--bg); color: var(--fg);
    font: 16px/1.65 ui-sans-serif, system-ui, -apple-system, "Segoe UI", sans-serif;
    min-height: 100vh;
  }
  .status {
    font-size: 0.78rem; color: var(--muted); letter-spacing: 0.02em;
    padding: 0.6rem 1.25rem; border-bottom: 1px solid var(--rule);
  }
  .content { padding: 1.5rem 1.25rem; max-width: 46rem; }
  .cite {
    font-size: 0.75rem; color: var(--accent); background: var(--chip-bg);
    padding: 0.1rem 0.4rem; border-radius: 0.25rem; white-space: nowrap;
  }
</style>

<div class="guide">
  <div class="status">spec v0.9.4 · 0 lines of Go</div>
  <main class="content">
    <article data-route="world/perps">
      <h1>The world</h1>
      <p>
        A <strong>perpetual</strong> is a futures contract with no expiry. To keep its
        price tethered to spot without a settlement date it uses a funding rate — the
        anchor this entire business is built on.
        <span class="cite" data-src="tutorial §1.1">tutorial §1.1</span>
      </p>
    </article>
  </main>
</div>

<script id="guide-pure">
var GuidePure = (function () {
  "use strict";
  return {};
})();
</script>

<script id="guide-ui">
(function () {
  "use strict";
})();
</script>
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `cd docs/guide && bun test`
Expected: PASS — 6 tests.

- [ ] **Step 6: Write the README**

Create `docs/guide/README.md`:

```markdown
# Interactive guide

`index.html` is the comprehensive interactive guide to the funding-rate arbitrage
platform. Design: `docs/superpowers/specs/2026-07-25-interactive-guide-design.md`.

## Constraints that the tests enforce

- **Self-contained.** No external scripts, stylesheets, fonts, images or network
  calls — a published Artifact runs under a strict CSP that blocks all of them.
- **No document wrapper.** No `<!DOCTYPE>`, `<html>`, `<head>` or `<body>` tags;
  the Artifact platform supplies that skeleton at publish time. A `<title>` is correct.
- **Two script blocks.** `#guide-pure` is DOM-free and exposes `GuidePure`;
  `#guide-ui` does all DOM wiring. The tests evaluate only the pure block.
- **Citation chips.** Every factual claim carries `<span class="cite" data-src="…">`.
  The guide teaches; `build-spec.md` and `state-machines.md` remain normative.

## Running the tests

    cd docs/guide && bun test

## Publishing

The file is published as an Artifact and redeploys to the same URL. There is no
build step — what is committed is what is published.
```

- [ ] **Step 7: Commit**

```bash
git add docs/guide/
git commit -m "guide: walking skeleton with self-containment and CSP constraint tests"
```

---

### Task 2: Left rail and hash router

**Files:**
- Modify: `docs/guide/index.html`
- Test: `docs/guide/tests/structure.test.js`

**Interfaces:**
- Consumes: `readGuide()` from `tests/extract.js`; the `data-route` convention from Task 1.
- Produces: `GuidePure.ROUTES` — an ordered array of `{part, id, route, title}`, the single source of truth for both the nav and the route-integrity test. Later tasks add entries to it rather than editing markup in two places.

- [ ] **Step 1: Write the failing route-integrity tests**

Extend the existing import at the top of `docs/guide/tests/structure.test.js` to `import { readGuide, loadPure } from "./extract.js";`, then append:

```js
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
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd docs/guide && bun test`
Expected: FAIL — `pure.ROUTES is not iterable`, because `GuidePure` is still `{}`.

- [ ] **Step 3: Declare the routes in the pure block**

In `index.html`, replace the body of the `#guide-pure` IIFE:

```js
var GuidePure = (function () {
  "use strict";

  var PARTS = [
    { id: "world",   n: 1, title: "The world" },
    { id: "edge",    n: 2, title: "The edge, honestly" },
    { id: "why",     n: 3, title: "Why the design looks like this" },
    { id: "build",   n: 4, title: "What we're building" },
    { id: "go",      n: 5, title: "The Go build" },
    { id: "runbook", n: 6, title: "Operator runbook" }
  ];

  var ROUTES = [
    { part: "world", id: "perps", route: "world/perps", title: "Spot, futures, perpetuals" }
  ];

  return { PARTS: PARTS, ROUTES: ROUTES };
})();
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd docs/guide && bun test`
Expected: PASS — 10 tests.

- [ ] **Step 5: Build the rail markup and router**

Replace the `<div class="guide">` block in `index.html` with a two-column layout, and fill in the `#guide-ui` block. Add to `<style>`:

```css
  .guide { display: grid; grid-template-columns: 16rem 1fr; min-height: 100vh; }
  .rail {
    border-right: 1px solid var(--rule); padding: 1rem 0.75rem;
    position: sticky; top: 0; height: 100vh; overflow-y: auto;
  }
  .rail h2 {
    font-size: 0.7rem; text-transform: uppercase; letter-spacing: 0.08em;
    color: var(--muted); margin: 1rem 0 0.35rem;
  }
  .nav-link {
    display: block; padding: 0.28rem 0.5rem; border-radius: 0.3rem;
    color: var(--fg); text-decoration: none; font-size: 0.9rem;
  }
  .nav-link:hover { background: var(--chip-bg); }
  .nav-link[aria-current="true"] { background: var(--chip-bg); color: var(--accent); font-weight: 600; }
  .pane { overflow-y: auto; }
  article[data-route] { display: none; }
  article[data-route].is-active { display: block; }
  table { border-collapse: collapse; width: 100%; }
  .scroll-x { overflow-x: auto; }
  @media (max-width: 860px) {
    .guide { grid-template-columns: 1fr; }
    .rail { position: static; height: auto; border-right: 0; border-bottom: 1px solid var(--rule); }
  }
```

Markup — the rail is empty in source and rendered from `ROUTES`:

```html
<div class="guide">
  <nav class="rail" id="rail" aria-label="Guide contents"></nav>
  <div class="pane">
    <div class="status">spec v0.9.4 · 0 lines of Go</div>
    <main class="content" id="content">
      <article data-route="world/perps">
        <h1>The world</h1>
        <p>
          A <strong>perpetual</strong> is a futures contract with no expiry. To keep its
          price tethered to spot without a settlement date it uses a funding rate — the
          anchor this entire business is built on.
          <span class="cite" data-src="tutorial §1.1">tutorial §1.1</span>
        </p>
      </article>
    </main>
  </div>
</div>
```

And the UI block:

```js
(function () {
  "use strict";
  var rail = document.getElementById("rail");

  GuidePure.PARTS.forEach(function (part) {
    var routes = GuidePure.ROUTES.filter(function (r) { return r.part === part.id; });
    if (!routes.length) return;
    var h = document.createElement("h2");
    h.textContent = part.n + " · " + part.title;
    rail.appendChild(h);
    routes.forEach(function (r) {
      var a = document.createElement("a");
      a.className = "nav-link";
      a.href = "#/" + r.route;
      a.dataset.navRoute = r.route;
      a.textContent = r.title;
      rail.appendChild(a);
    });
  });

  function show(route) {
    var articles = document.querySelectorAll("article[data-route]");
    var found = false;
    articles.forEach(function (el) {
      var on = el.dataset.route === route;
      el.classList.toggle("is-active", on);
      if (on) found = true;
    });
    if (!found && articles.length) {
      articles[0].classList.add("is-active");
      route = articles[0].dataset.route;
    }
    document.querySelectorAll(".nav-link").forEach(function (a) {
      a.setAttribute("aria-current", a.dataset.navRoute === route ? "true" : "false");
    });
    document.getElementById("content").scrollTop = 0;
  }

  function routeFromHash() {
    return (location.hash || "").replace(/^#\/?/, "") || GuidePure.ROUTES[0].route;
  }

  window.addEventListener("hashchange", function () { show(routeFromHash()); });
  show(routeFromHash());
})();
```

- [ ] **Step 6: Verify it renders**

Run: `open docs/guide/index.html`
Expected: the rail lists "1 · The world" with one entry; clicking it sets the hash to `#/world/perps` and the article stays visible. An unknown hash such as `#/nope` falls back to the first article without a blank screen.

- [ ] **Step 7: Run the tests**

Run: `cd docs/guide && bun test`
Expected: PASS — 10 tests.

- [ ] **Step 8: Commit**

```bash
git add docs/guide/
git commit -m "guide: left rail rendered from ROUTES, hash router with fallback"
```

---

### Task 3: Search over all parts

Native browser find only sees the visible article, so search is the compensating mechanism, not a nicety.

**Files:**
- Modify: `docs/guide/index.html`
- Test: `docs/guide/tests/search.test.js`

**Interfaces:**
- Consumes: `GuidePure.ROUTES`.
- Produces: `GuidePure.buildSearchIndex(records) → index` and `GuidePure.searchIndex(index, query) → [{route, title, snippet}]`. `records` is `[{route, title, text}]`; the UI supplies it by walking the DOM, the test supplies it as a literal.

- [ ] **Step 1: Write the failing search tests**

Create `docs/guide/tests/search.test.js`:

```js
import { test, expect } from "bun:test";
import { loadPure } from "./extract.js";

const RECORDS = [
  { route: "world/perps", title: "Spot, futures, perpetuals", text: "A perpetual has no expiry and uses a funding rate." },
  { route: "edge/kill",   title: "The kill math",             text: "Round trip at VIP0 taker is about 0.38% of notional." }
];

test("finds a term that appears only in a non-first record", () => {
  const pure = loadPure();
  const hits = pure.searchIndex(pure.buildSearchIndex(RECORDS), "notional");
  expect(hits.length).toBe(1);
  expect(hits[0].route).toBe("edge/kill");
});

test("matches titles as well as body text", () => {
  const pure = loadPure();
  const hits = pure.searchIndex(pure.buildSearchIndex(RECORDS), "kill");
  expect(hits[0].route).toBe("edge/kill");
});

test("is case-insensitive", () => {
  const pure = loadPure();
  const hits = pure.searchIndex(pure.buildSearchIndex(RECORDS), "PERPETUAL");
  expect(hits[0].route).toBe("world/perps");
});

test("returns nothing for a blank or one-character query", () => {
  const pure = loadPure();
  const index = pure.buildSearchIndex(RECORDS);
  expect(pure.searchIndex(index, "")).toEqual([]);
  expect(pure.searchIndex(index, "a")).toEqual([]);
});

test("returns a snippet containing the match", () => {
  const pure = loadPure();
  const hits = pure.searchIndex(pure.buildSearchIndex(RECORDS), "funding");
  expect(hits[0].snippet.toLowerCase()).toContain("funding");
});
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd docs/guide && bun test search`
Expected: FAIL — `pure.buildSearchIndex is not a function`.

- [ ] **Step 3: Implement search in the pure block**

Add inside the `GuidePure` IIFE, before the `return`:

```js
  function buildSearchIndex(records) {
    return records.map(function (r) {
      return {
        route: r.route,
        title: r.title,
        text: r.text,
        haystack: (r.title + " " + r.text).toLowerCase()
      };
    });
  }

  function searchIndex(index, query) {
    var q = String(query || "").trim().toLowerCase();
    if (q.length < 2) return [];
    var out = [];
    index.forEach(function (entry) {
      var at = entry.haystack.indexOf(q);
      if (at === -1) return;
      var source = entry.text;
      var pos = source.toLowerCase().indexOf(q);
      var snippet;
      if (pos === -1) {
        snippet = source.slice(0, 90);
      } else {
        var from = Math.max(0, pos - 35);
        snippet = (from > 0 ? "…" : "") + source.slice(from, pos + q.length + 55).trim();
      }
      out.push({ route: entry.route, title: entry.title, snippet: snippet });
    });
    return out;
  }
```

Extend the returned object to `{ PARTS: PARTS, ROUTES: ROUTES, buildSearchIndex: buildSearchIndex, searchIndex: searchIndex }`.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd docs/guide && bun test search`
Expected: PASS — 5 tests.

- [ ] **Step 5: Wire the search UI**

Add to `<style>`:

```css
  .search { width: 100%; padding: 0.4rem 0.55rem; font: inherit; font-size: 0.85rem;
    background: var(--bg); color: var(--fg); border: 1px solid var(--rule); border-radius: 0.3rem; }
  .results { list-style: none; margin: 0.5rem 0 0; padding: 0; }
  .results li { margin-bottom: 0.5rem; }
  .results a { color: var(--fg); text-decoration: none; display: block;
    padding: 0.3rem 0.5rem; border-radius: 0.3rem; }
  .results a:hover { background: var(--chip-bg); }
  .results .snippet { display: block; font-size: 0.75rem; color: var(--muted); }
  .rail.searching h2, .rail.searching .nav-link { display: none; }
```

Add as the first children of `<nav class="rail">` (rendered nav is appended after these):

```html
  <input class="search" id="search" type="search" placeholder="Search the guide" aria-label="Search the guide">
  <ul class="results" id="results"></ul>
```

Append to the `#guide-ui` IIFE:

```js
  var records = [];
  document.querySelectorAll("article[data-route]").forEach(function (el) {
    var meta = GuidePure.ROUTES.filter(function (r) { return r.route === el.dataset.route; })[0];
    records.push({
      route: el.dataset.route,
      title: meta ? meta.title : el.dataset.route,
      text: el.textContent.replace(/\s+/g, " ").trim()
    });
  });
  var index = GuidePure.buildSearchIndex(records);

  var box = document.getElementById("search");
  var results = document.getElementById("results");
  box.addEventListener("input", function () {
    var hits = GuidePure.searchIndex(index, box.value);
    rail.classList.toggle("searching", box.value.trim().length >= 2);
    results.innerHTML = "";
    hits.forEach(function (hit) {
      var li = document.createElement("li");
      var a = document.createElement("a");
      a.href = "#/" + hit.route;
      a.innerHTML = "";
      var strong = document.createElement("strong");
      strong.textContent = hit.title;
      var span = document.createElement("span");
      span.className = "snippet";
      span.textContent = hit.snippet;
      a.appendChild(strong);
      a.appendChild(span);
      li.appendChild(a);
      results.appendChild(li);
    });
  });
```

Note: the rail element is already in scope as `rail` from Task 2. Because the search input and results list are authored in the markup and the nav links are appended after them, no reordering is needed.

- [ ] **Step 6: Verify it renders**

Run: `open docs/guide/index.html`
Expected: typing `perpetual` shows one result with a snippet; clicking it navigates. Clearing the box restores the nav.

- [ ] **Step 7: Commit**

```bash
git add docs/guide/
git commit -m "guide: search across all parts, including unvisited sections"
```

---

### Task 4: The economics engine

The heart of the guide. Pure arithmetic, verified against five figures printed in `tutorial §3.9` and `core §2.4`.

**Files:**
- Modify: `docs/guide/index.html`
- Test: `docs/guide/tests/economics.test.js`

**Interfaces:**
- Consumes: nothing.
- Produces: `GuidePure.computeEconomics(input) → result` and `GuidePure.PRESETS` (an object keyed by preset id, each value a complete `input`). Task 5 renders both.

Input fields, all required: `notional`, `perpLeverage`, `rescueMarginUsd`, `feeReserveUsd`, `committedFloatUsd`, `fundingRatePerSettlement`, `fundingIntervalHours`, `holdDays`, `spotFeeRate`, `perpFeeRate`, `slippagePerLegRate`, `roundTripMultiplier`, `utilization`, `hurdleApy`.

Result fields: `cLegs`, `cAllocated`, `settlements`, `fundingUsd`, `roundTripRate`, `roundTripCostUsd`, `netUsd`, `periodReturn`, `inPositionApy`, `netApy`, `apyOnLegsFloor`, `breakEvenSettlements`, `breakEvenDays`, `clearsHurdle`, `shortfallPts`.

- [ ] **Step 1: Write the failing economics tests**

Create `docs/guide/tests/economics.test.js`. Every assertion below traces to a printed figure in the docs — the comment names its source.

```js
import { test, expect } from "bun:test";
import { loadPure } from "./extract.js";

// tutorial §3.9 worked trade: N=$10k, BTC flat at $100k, +0.015%/8h, 20-day hold, 3x perp,
// VIP0 taker, rescue margin ~$3,333, fee/slippage reserve ~$40.
const WORKED = {
  notional: 10000,
  perpLeverage: 3,
  rescueMarginUsd: 3333,
  feeReserveUsd: 40,
  committedFloatUsd: 0,
  fundingRatePerSettlement: 0.00015,
  fundingIntervalHours: 8,
  holdDays: 20,
  spotFeeRate: 0.0010,
  perpFeeRate: 0.0005,
  slippagePerLegRate: 0.0002,
  roundTripMultiplier: 1,
  utilization: 0.6,
  hurdleApy: 0.046
};

test("leg-capital floor is 1.33·N — tutorial §3.9, core §2.2", () => {
  const r = loadPure().computeEconomics(WORKED);
  expect(r.cLegs).toBeCloseTo(13333.33, 1);
});

test("C_allocated is about $16.7K — tutorial §3.9", () => {
  const r = loadPure().computeEconomics(WORKED);
  expect(r.cAllocated).toBeCloseTo(16706.33, 1);
});

test("60 settlements over a 20-day hold at 8h — tutorial §3.9", () => {
  expect(loadPure().computeEconomics(WORKED).settlements).toBe(60);
});

test("funding collected is $90 — tutorial §3.9", () => {
  expect(loadPure().computeEconomics(WORKED).fundingUsd).toBeCloseTo(90, 6);
});

test("round-trip cost is $38, i.e. 0.38% of notional — tutorial §3.9, core §2.4", () => {
  const r = loadPure().computeEconomics(WORKED);
  expect(r.roundTripCostUsd).toBeCloseTo(38, 6);
  expect(r.roundTripRate).toBeCloseTo(0.0038, 6);
});

test("net is +$52 over the hold — tutorial §3.9", () => {
  expect(loadPure().computeEconomics(WORKED).netUsd).toBeCloseTo(52, 6);
});

test("break-even takes about 25 settlements / 8.4 days — tutorial §3.9", () => {
  const r = loadPure().computeEconomics(WORKED);
  expect(r.breakEvenSettlements).toBeCloseTo(25.33, 1);
  expect(r.breakEvenDays).toBeCloseTo(8.44, 1);
});

test("in-position return is ~5.7% annualized — tutorial §3.9", () => {
  const r = loadPure().computeEconomics(WORKED);
  expect(r.inPositionApy * 100).toBeCloseTo(5.68, 1);
});

test("the C_legs denominator overstates it as ~7.1% — tutorial §3.9", () => {
  const r = loadPure().computeEconomics(WORKED);
  expect(r.apyOnLegsFloor * 100).toBeCloseTo(7.12, 1);
});

// THE NAMED TEST from the design doc, and the one core §12 predicts milestone 2 will hit.
test("at 60% utilization it nets ~3.4% and FAILS the 4.6% hurdle — core §12, tutorial §3.9", () => {
  const r = loadPure().computeEconomics(WORKED);
  expect(r.netApy * 100).toBeCloseTo(3.41, 1);
  expect(r.clearsHurdle).toBe(false);
  expect(r.shortfallPts).toBeCloseTo(-1.19, 1);
});

// core §2.4 break-even worked example: baseline +0.01%/8h against a 0.38% round trip.
test("baseline +0.01%/8h takes ~12-13 days to break even — core §2.4", () => {
  const r = loadPure().computeEconomics({ ...WORKED, fundingRatePerSettlement: 0.0001 });
  expect(r.breakEvenDays).toBeGreaterThan(12);
  expect(r.breakEvenDays).toBeLessThan(13);
});

test("a rich +0.02%/8h regime still takes ~6 days — core §2.4", () => {
  const r = loadPure().computeEconomics({ ...WORKED, fundingRatePerSettlement: 0.0002 });
  expect(r.breakEvenDays).toBeCloseTo(6.33, 1);
});

test("a 1h funding interval yields eight times the settlements — core C8", () => {
  const r = loadPure().computeEconomics({ ...WORKED, fundingIntervalHours: 1 });
  expect(r.settlements).toBe(480);
});

test("every preset is a complete, computable input", () => {
  const pure = loadPure();
  const keys = Object.keys(WORKED);
  for (const [id, preset] of Object.entries(pure.PRESETS)) {
    for (const k of keys) {
      expect(preset.input[k], `preset ${id} missing ${k}`).toBeDefined();
    }
    expect(Number.isFinite(pure.computeEconomics(preset.input).netApy)).toBe(true);
  }
});

test("the maker-entry fork halves round-trip cost and just clears the hurdle — core §2.4", () => {
  const pure = loadPure();
  const r = pure.computeEconomics(pure.PRESETS.maker.input);
  expect(r.roundTripCostUsd).toBeCloseTo(19, 6);
  expect(r.clearsHurdle).toBe(true);
  expect(r.netApy * 100).toBeCloseTo(4.65, 1);
});
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd docs/guide && bun test economics`
Expected: FAIL — `pure.computeEconomics is not a function`.

- [ ] **Step 3: Implement the engine in the pure block**

Add inside the `GuidePure` IIFE, before the `return`:

```js
  // All terms in one unit — USD over the hold, then a return on C_allocated (core §2.4).
  // The reporting denominator is C_allocated, never the C_legs floor and never posted
  // margin (core §2.1/§2.2).
  function computeEconomics(input) {
    var N = input.notional;
    var perpInitialMargin = N / input.perpLeverage;
    var cLegs = N + perpInitialMargin;
    var cAllocated = cLegs + input.rescueMarginUsd + input.feeReserveUsd + input.committedFloatUsd;

    var perDay = 24 / input.fundingIntervalHours;
    var settlements = Math.floor(input.holdDays * perDay);
    var fundingUsd = input.fundingRatePerSettlement * N * settlements;

    var roundTripRate =
      (2 * input.spotFeeRate + 2 * input.perpFeeRate + 4 * input.slippagePerLegRate) *
      input.roundTripMultiplier;
    var roundTripCostUsd = N * roundTripRate;

    var netUsd = fundingUsd - roundTripCostUsd;
    var annualize = 365 / input.holdDays;
    var periodReturn = netUsd / cAllocated;
    var inPositionApy = periodReturn * annualize;
    var netApy = inPositionApy * input.utilization;
    var apyOnLegsFloor = (netUsd / cLegs) * annualize;

    var perSettlementUsd = input.fundingRatePerSettlement * N;
    var breakEvenSettlements = perSettlementUsd > 0 ? roundTripCostUsd / perSettlementUsd : Infinity;

    return {
      cLegs: cLegs,
      cAllocated: cAllocated,
      settlements: settlements,
      fundingUsd: fundingUsd,
      roundTripRate: roundTripRate,
      roundTripCostUsd: roundTripCostUsd,
      netUsd: netUsd,
      periodReturn: periodReturn,
      inPositionApy: inPositionApy,
      netApy: netApy,
      apyOnLegsFloor: apyOnLegsFloor,
      breakEvenSettlements: breakEvenSettlements,
      breakEvenDays: breakEvenSettlements / perDay,
      clearsHurdle: netApy >= input.hurdleApy,
      shortfallPts: (netApy - input.hurdleApy) * 100
    };
  }

  var WORKED_TRADE = {
    notional: 10000,
    perpLeverage: 3,
    rescueMarginUsd: 3333,
    feeReserveUsd: 40,
    committedFloatUsd: 0,
    fundingRatePerSettlement: 0.00015,
    fundingIntervalHours: 8,
    holdDays: 20,
    spotFeeRate: 0.0010,
    perpFeeRate: 0.0005,
    slippagePerLegRate: 0.0002,
    roundTripMultiplier: 1,
    utilization: 0.6,
    hurdleApy: 0.046
  };

  function withInput(overrides) {
    var out = {}, k;
    for (k in WORKED_TRADE) out[k] = WORKED_TRADE[k];
    for (k in overrides) out[k] = overrides[k];
    return out;
  }

  var PRESETS = {
    worked: {
      label: "The worked trade (tutorial §3.9)",
      note: "A good major regime at VIP0 taker fees. Nets ~3.4% at 60% time-in-position — below the hurdle.",
      cite: "tutorial §3.9",
      input: withInput({})
    },
    baseline: {
      label: "Baseline BTC, VIP0 — what milestone 2 expects to find",
      note: "Funding at +0.01%/8h. Roughly 12-13 days just to cover the round trip.",
      cite: "core §2.4",
      input: withInput({ fundingRatePerSettlement: 0.0001 })
    },
    maker: {
      label: "Fork (a) — maker spot entry",
      note: "core §2.4: routing the spot entry maker roughly halves round-trip cost. It just clears the hurdle.",
      cite: "core §2.4, core §12",
      input: withInput({ roundTripMultiplier: 0.5 })
    },
    regime: {
      label: "Fork (b) — regime-selective deployment",
      note: "Trade only measured-positive regimes: a richer rate bought with lower utilization.",
      cite: "core §12",
      input: withInput({ fundingRatePerSettlement: 0.0002, utilization: 0.35 })
    }
  };
```

Extend the returned object with `computeEconomics: computeEconomics` and `PRESETS: PRESETS`.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd docs/guide && bun test economics`
Expected: PASS — 15 tests. If the worked-trade assertions disagree, **stop and reconcile against `tutorial §3.9` before continuing** — a mismatch means either the model or the reading of the spec is wrong, and which one must be established before the number is shown to anybody.

- [ ] **Step 5: Commit**

```bash
git add docs/guide/
git commit -m "guide: economics engine verified against five figures from tutorial 3.9 and core 2.4"
```

---

### Task 5: The calculator UI

**Files:**
- Modify: `docs/guide/index.html`
- Test: `docs/guide/tests/structure.test.js`

**Interfaces:**
- Consumes: `GuidePure.computeEconomics`, `GuidePure.PRESETS`.
- Produces: `GuidePure.formatUsd(n)`, `GuidePure.formatPct(n)` — used by the calculator and reused by later parts.

- [ ] **Step 1: Write the failing formatter tests**

Append to `docs/guide/tests/economics.test.js`:

```js
test("formats USD without false precision", () => {
  const pure = loadPure();
  expect(pure.formatUsd(16706.33)).toBe("$16,706");
  expect(pure.formatUsd(52)).toBe("$52");
  expect(pure.formatUsd(-38)).toBe("−$38");
});

test("formats percentages to two decimals", () => {
  const pure = loadPure();
  expect(pure.formatPct(0.034084)).toBe("3.41%");
  expect(pure.formatPct(0.046)).toBe("4.60%");
});
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd docs/guide && bun test economics`
Expected: FAIL — `pure.formatUsd is not a function`.

- [ ] **Step 3: Implement the formatters**

Add inside the `GuidePure` IIFE and to its returned object:

```js
  function formatUsd(n) {
    var sign = n < 0 ? "−" : "";
    var whole = Math.round(Math.abs(n));
    return sign + "$" + whole.toString().replace(/\B(?=(\d{3})+(?!\d))/g, ",");
  }

  function formatPct(x) {
    return (x * 100).toFixed(2) + "%";
  }
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd docs/guide && bun test economics`
Expected: PASS — 17 tests.

- [ ] **Step 5: Build the calculator markup**

Add a new article to the content (its route is registered in Task 7):

```html
      <article data-route="edge/calculator">
        <h1>Will this actually make money?</h1>
        <p>
          Move the inputs. The verdict is the same test milestone 2 runs, on the same
          denominator the spec insists on — the full allocated bankroll, never posted
          margin. <span class="cite" data-src="core §2.1">core §2.1</span>
          <span class="cite" data-src="core §2.2">core §2.2</span>
        </p>

        <div class="calc">
          <div class="calc-presets" id="calc-presets"></div>
          <div class="calc-grid">
            <label>Funding rate (% per settlement)
              <input id="in-rate" type="number" step="0.001" value="0.015"></label>
            <label>Funding interval (hours)
              <select id="in-interval"><option value="8">8</option><option value="1">1</option></select></label>
            <label>Hold length (days)
              <input id="in-hold" type="number" step="1" value="20"></label>
            <label>Notional N (USD)
              <input id="in-notional" type="number" step="1000" value="10000"></label>
            <label>Perp leverage
              <input id="in-leverage" type="number" step="1" value="3"></label>
            <label>Rescue margin (USD)
              <input id="in-rescue" type="number" step="100" value="3333"></label>
            <label>Spot fee (%)
              <input id="in-spotfee" type="number" step="0.005" value="0.10"></label>
            <label>Perp fee (%)
              <input id="in-perpfee" type="number" step="0.005" value="0.05"></label>
            <label>Slippage per leg (%)
              <input id="in-slip" type="number" step="0.005" value="0.02"></label>
            <label>Round-trip multiplier
              <input id="in-mult" type="number" step="0.1" value="1"></label>
            <label>Utilization (%)
              <input id="in-util" type="number" step="5" value="60"></label>
            <label>Hurdle (%)
              <input id="in-hurdle" type="number" step="0.1" value="4.6"></label>
          </div>
          <div class="scroll-x"><table class="calc-out" id="calc-out"></table></div>
          <p class="verdict" id="calc-verdict"></p>
          <p class="calc-note" id="calc-note"></p>
        </div>
      </article>
```

Add to `<style>`:

```css
  .calc { border: 1px solid var(--rule); border-radius: 0.5rem; padding: 1rem; margin: 1.25rem 0; }
  .calc-presets { display: flex; flex-wrap: wrap; gap: 0.4rem; margin-bottom: 0.9rem; }
  .calc-presets button {
    font: inherit; font-size: 0.78rem; padding: 0.3rem 0.6rem; cursor: pointer;
    background: var(--chip-bg); color: var(--fg); border: 1px solid var(--rule); border-radius: 0.3rem;
  }
  .calc-presets button[aria-pressed="true"] { border-color: var(--accent); color: var(--accent); }
  .calc-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(11rem, 1fr)); gap: 0.6rem; }
  .calc-grid label { display: flex; flex-direction: column; font-size: 0.75rem; color: var(--muted); gap: 0.2rem; }
  .calc-grid input, .calc-grid select {
    font: inherit; font-size: 0.9rem; padding: 0.3rem 0.4rem;
    background: var(--bg); color: var(--fg); border: 1px solid var(--rule); border-radius: 0.25rem;
  }
  .calc-out { margin-top: 1rem; font-size: 0.9rem; }
  .calc-out td { padding: 0.22rem 0.6rem 0.22rem 0; border-bottom: 1px solid var(--rule); }
  .calc-out td:last-child { text-align: right; font-variant-numeric: tabular-nums; }
  .calc-out tr.total td { font-weight: 700; border-bottom: 2px solid var(--rule); }
  .verdict { font-weight: 700; margin-top: 0.8rem; }
  .verdict.pass { color: #1f7a3d; }
  .verdict.fail { color: #a12d2d; }
  @media (prefers-color-scheme: dark) {
    .verdict.pass { color: #58c07d; } .verdict.fail { color: #e07070; }
  }
  :root[data-theme="dark"] .verdict.pass { color: #58c07d; }
  :root[data-theme="dark"] .verdict.fail { color: #e07070; }
  .calc-note { font-size: 0.8rem; color: var(--muted); }
```

- [ ] **Step 6: Wire the calculator**

Append to the `#guide-ui` IIFE:

```js
  var FIELDS = {
    "in-rate":     { key: "fundingRatePerSettlement", scale: 0.01 },
    "in-interval": { key: "fundingIntervalHours",     scale: 1 },
    "in-hold":     { key: "holdDays",                 scale: 1 },
    "in-notional": { key: "notional",                 scale: 1 },
    "in-leverage": { key: "perpLeverage",             scale: 1 },
    "in-rescue":   { key: "rescueMarginUsd",          scale: 1 },
    "in-spotfee":  { key: "spotFeeRate",              scale: 0.01 },
    "in-perpfee":  { key: "perpFeeRate",              scale: 0.01 },
    "in-slip":     { key: "slippagePerLegRate",       scale: 0.01 },
    "in-mult":     { key: "roundTripMultiplier",      scale: 1 },
    "in-util":     { key: "utilization",              scale: 0.01 },
    "in-hurdle":   { key: "hurdleApy",                scale: 0.01 }
  };

  var calcOut = document.getElementById("calc-out");
  if (calcOut) {
    var verdictEl = document.getElementById("calc-verdict");
    var noteEl = document.getElementById("calc-note");
    var presetBar = document.getElementById("calc-presets");

    function readInputs() {
      var input = { feeReserveUsd: 40, committedFloatUsd: 0 };
      Object.keys(FIELDS).forEach(function (id) {
        var f = FIELDS[id];
        input[f.key] = parseFloat(document.getElementById(id).value) * f.scale;
      });
      return input;
    }

    function writeInputs(input) {
      Object.keys(FIELDS).forEach(function (id) {
        var f = FIELDS[id];
        document.getElementById(id).value = String(input[f.key] / f.scale);
      });
    }

    function render() {
      var input = readInputs();
      var r = GuidePure.computeEconomics(input);
      var rows = [
        ["Leg-capital floor C_legs", GuidePure.formatUsd(r.cLegs)],
        ["Allocated bankroll C_allocated", GuidePure.formatUsd(r.cAllocated)],
        ["Settlements in the hold", String(r.settlements)],
        ["Funding collected", GuidePure.formatUsd(r.fundingUsd)],
        ["Round-trip cost", GuidePure.formatUsd(-r.roundTripCostUsd)],
        ["Days to break even", r.breakEvenDays.toFixed(1)],
        ["Net over the hold", GuidePure.formatUsd(r.netUsd)],
        ["Annualized while in position", GuidePure.formatPct(r.inPositionApy)],
        ["× utilization", GuidePure.formatPct(input.utilization)]
      ];
      calcOut.innerHTML = "";
      rows.forEach(function (row) {
        var tr = calcOut.insertRow();
        tr.insertCell().textContent = row[0];
        tr.insertCell().textContent = row[1];
      });
      var totalRow = calcOut.insertRow();
      totalRow.className = "total";
      totalRow.insertCell().textContent = "Net APY on the bankroll";
      totalRow.insertCell().textContent = GuidePure.formatPct(r.netApy);
      var hurdleRow = calcOut.insertRow();
      hurdleRow.insertCell().textContent = "Hurdle";
      hurdleRow.insertCell().textContent = GuidePure.formatPct(input.hurdleApy);

      verdictEl.className = "verdict " + (r.clearsHurdle ? "pass" : "fail");
      verdictEl.textContent = r.clearsHurdle
        ? "Clears the hurdle by " + r.shortfallPts.toFixed(2) + " points."
        : "Fails the hurdle by " + Math.abs(r.shortfallPts).toFixed(2) + " points.";
    }

    Object.keys(GuidePure.PRESETS).forEach(function (id) {
      var preset = GuidePure.PRESETS[id];
      var b = document.createElement("button");
      b.type = "button";
      b.textContent = preset.label;
      b.setAttribute("aria-pressed", "false");
      b.addEventListener("click", function () {
        writeInputs(preset.input);
        render();
        noteEl.textContent = preset.note + "  [" + preset.cite + "]";
        presetBar.querySelectorAll("button").forEach(function (o) {
          o.setAttribute("aria-pressed", String(o === b));
        });
      });
      presetBar.appendChild(b);
    });

    Object.keys(FIELDS).forEach(function (id) {
      document.getElementById(id).addEventListener("input", function () {
        render();
        presetBar.querySelectorAll("button").forEach(function (o) {
          o.setAttribute("aria-pressed", "false");
        });
        noteEl.textContent = "";
      });
    });

    render();
  }
```

- [ ] **Step 7: Verify it renders and the default reproduces the spec**

Run: `open docs/guide/index.html`, navigate to the calculator.
Expected: with the defaults as authored, "Net APY on the bankroll" reads **3.41%**, hurdle 4.60%, and the verdict reads "Fails the hurdle by 1.19 points." Clicking *Fork (a) — maker spot entry* flips it to a pass at 4.65%.

- [ ] **Step 8: Run the full suite**

Run: `cd docs/guide && bun test`
Expected: PASS — all tests.

- [ ] **Step 9: Commit**

```bash
git add docs/guide/
git commit -m "guide: economics calculator UI with the four milestone-2 fork presets"
```

---

### Task 6: Part 1 — The world

**Files:**
- Modify: `docs/guide/index.html`
- Test: `docs/guide/tests/structure.test.js`

**Interfaces:**
- Consumes: `GuidePure.ROUTES`, the `.cite` chip convention.
- Produces: six `world/*` routes that Task 7's citation-coverage test will scan.

- [ ] **Step 1: Write the failing citation-coverage test**

Append to `docs/guide/tests/structure.test.js`:

```js
test("every article carries at least one citation chip", () => {
  const html = readGuide();
  const articles = [...html.matchAll(/<article data-route="([^"]+)"[\s\S]*?<\/article>/g)];
  expect(articles.length).toBeGreaterThan(0);
  for (const [block, route] of articles) {
    expect(block, `article ${route} has no citation chip`).toMatch(/class="cite" data-src="/);
  }
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
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd docs/guide && bun test structure`
Expected: FAIL — the Part 1 route assertions fail; only `world/perps` exists.

- [ ] **Step 3: Register the Part 1 routes**

Replace the `ROUTES` array in the pure block:

```js
  var ROUTES = [
    { part: "world", id: "perps",          route: "world/perps",          title: "Spot, futures, perpetuals" },
    { part: "world", id: "funding",        route: "world/funding",        title: "The funding rate" },
    { part: "world", id: "prices",         route: "world/prices",         title: "Mark, index, last" },
    { part: "world", id: "liquidation",    route: "world/liquidation",    title: "Liquidation, margin, ADL" },
    { part: "world", id: "venues",         route: "world/venues",         title: "Binance and Bybit" },
    { part: "world", id: "counterparties", route: "world/counterparties", title: "Who is on the other side" }
  ];
```

- [ ] **Step 4: Write the Part 1 content**

Write six `<article data-route="world/…">` blocks into `#content`, replacing the placeholder `world/perps` article.

**Write this one verbatim first** — it establishes the markup pattern, the chip placement, and the voice every other article must match:

```html
      <article data-route="world/funding">
        <h1>The funding rate</h1>

        <p>
          A perpetual can drift away from spot. Nothing forces them back together,
          because there is no settlement date to converge on. So the exchange makes
          the crowded side pay the other side, at fixed intervals.
          <span class="cite" data-src="tutorial §1.2">tutorial §1.2</span>
        </p>

        <ul>
          <li>Perp trading <strong>above</strong> spot — longs are crowded — funding
            is <strong>positive</strong>, and longs pay shorts.</li>
          <li>Perp trading <strong>below</strong> spot — funding is
            <strong>negative</strong>, and shorts pay longs.</li>
        </ul>

        <p>
          The exchange orchestrates the transfer and takes no side. Funding is
          counterparties paying each other to hold the tether. Being short the perp
          while long the spot is how you get paid for it.
          <span class="cite" data-src="tutorial §1.2">tutorial §1.2</span>
        </p>

        <h2>The damper is not the cap</h2>

        <p>
          Simplified, the rate is
          <code>F ≈ premium_index + clamp(interest − premium, ±damper)</code>. Two
          different clamps live in that sentence and they get conflated constantly.
          <span class="cite" data-src="tutorial §1.4">tutorial §1.4</span>
        </p>

        <div class="scroll-x">
          <table>
            <tr><td><strong>The damper</strong> (Binance ~±0.05%)</td>
                <td>A small band <em>inside</em> the formula. Keeps the rate near the
                    interest term when the premium is small. Shapes normal rates.</td></tr>
            <tr><td><strong>The cap / floor</strong> (per-symbol; BTC ±0.3%, alts ±2%)</td>
                <td>The hard limit on the <em>final</em> rate. Bounds extreme ones.</td></tr>
          </table>
        </div>

        <p>
          Both are read live, never hardcoded — Bybit exposes <code>fundingCap</code> on
          its ticker and Binance overlays it via <code>fundingInfo</code>.
          <span class="cite" data-src="core §4">core §4</span>
          <span class="cite" data-src="Profiles §1">Profiles §1</span>
        </p>

        <h2>Funding is a payment, not a stream</h2>

        <p>
          This is where a plausible-looking estimate goes wrong. Funding is paid
          <em>at a timestamp</em>. It does not accrue by the second. You capture a
          settlement only if you are holding across it — so expected capture has to
          enumerate the actual settlement timestamps inside the planned hold and sum
          them. Multiplying a per-hour rate by hours held is not an approximation of
          that; it is a different number.
          <span class="cite" data-src="tutorial §1.7">tutorial §1.7</span>
          <span class="cite" data-src="core C2">core C2</span>
        </p>

        <p>
          Worse, the interval itself is live state. Since 2025-10-30 Bybit
          auto-switches an alt between 8h and 1h when funding pins the cap, and
          reverts <em>without notice</em>. A position opened under one interval can be
          settling on another an hour later, so the monitor has to detect the flip
          rather than trust the interval it saw at entry.
          <span class="cite" data-src="tutorial §1.7">tutorial §1.7</span>
          <span class="cite" data-src="core C8">core C8</span>
        </p>
      </article>
```

Then write the remaining five to the same pattern. Content requirements — **every claim gets a chip, no invented numbers**:

- **`world/perps`** — spot vs futures vs perpetual; a perp has no expiry, so funding is the tether. Note that spot held outright is not force-closed, but whether it can be pulled in to cover a derivatives loss depends on account posture, and this project deliberately pins one where it cannot. Chips: `tutorial §1.1`, `core §4.1`.
- **`world/funding`** — written verbatim above; nothing further needed.
- **`world/prices`** — last vs index vs mark; mark drives liquidation and funding, which is why liq distance moves on every mark update and a stale mark is itself a reason to block openings. Chips: `tutorial §1.6`, `tutorial §6.2`, `core §7.1`.
- **`world/liquidation`** — margin, leverage, isolated posture, liquidation; why the rescue is an active `add-margin` top-up rather than a passive wallet buffer (the isolated liq engine cannot see free wallet balance), and why the fallback is `PackageDelever`, a both-leg reduction, rather than reducing one leg and creating a naked leg during a rally. ADL as a realized hedge-failure mode: in Oct-2025 exchanges force-closed profitable short legs and left desks naked long into a falling market. Chips: `tutorial §6.2`, `core §7.1`, `core §5.3`, `core P3`.
- **`world/venues`** — Binance and Bybit as this project's two venues, and the shape of what an adapter must hide: Binance's decommissioned `/ws/` path (a dead-feed-you-cannot-see bug), Bybit's `fundingIntervalHour`/`fundingCap` fields, position-mode close semantics differing per venue, and rate-limit regimes differing including Bybit's unheadered IP gateway ban. Chips: `tutorial §4.2`, `Profiles §1`, `core §4`.
- **`world/counterparties`** — the exchange orchestrates the transfer and takes no side; funding is counterparties paying each other to hold the tether. Who is paying: crowded longs. What that implies about the edge — it is real, thin, and commoditized, and better-capitalized desks are in the same trade. Chips: `tutorial §1.2`, `tutorial Closing thoughts`, `core §2.1`.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `cd docs/guide && bun test`
Expected: PASS — all tests, including citation coverage on all seven articles.

- [ ] **Step 6: Verify it renders**

Run: `open docs/guide/index.html`
Expected: the rail shows six entries under "1 · The world"; each navigates; search finds a term from an unvisited section (try `damper`).

- [ ] **Step 7: Commit**

```bash
git add docs/guide/
git commit -m "guide: Part 1 — the world, six sections, every claim cited"
```

---

### Task 7: Part 2 — The edge, honestly

**Files:**
- Modify: `docs/guide/index.html`
- Test: `docs/guide/tests/structure.test.js`

**Interfaces:**
- Consumes: `GuidePure.ROUTES`, the calculator article from Task 5.
- Produces: five `edge/*` routes including `edge/calculator`, completing stage 1's content.

- [ ] **Step 1: Write the failing Part 2 route test**

Append to `docs/guide/tests/structure.test.js`:

```js
test("Part 2 covers the five topics the design names", () => {
  const routes = new Set(loadPure().ROUTES.map((r) => r.route));
  for (const r of [
    "edge/two-trades", "edge/costs", "edge/calculator", "edge/gate", "edge/forks"
  ]) {
    expect(routes.has(r)).toBe(true);
  }
});
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd docs/guide && bun test structure`
Expected: FAIL — no `edge/*` routes are registered.

- [ ] **Step 3: Register the Part 2 routes**

Append to the `ROUTES` array:

```js
    { part: "edge", id: "two-trades", route: "edge/two-trades", title: "Two trades that look alike" },
    { part: "edge", id: "costs",      route: "edge/costs",      title: "What it actually costs" },
    { part: "edge", id: "calculator", route: "edge/calculator", title: "Will this make money?" },
    { part: "edge", id: "gate",       route: "edge/gate",       title: "The economic gate" },
    { part: "edge", id: "forks",      route: "edge/forks",      title: "When the gate fails" }
```

- [ ] **Step 4: Write the Part 2 content**

Four new articles around the existing `edge/calculator`. Requirements:

- **`edge/two-trades`** — funding *level* (cash-and-carry: long spot + short perp on one venue) versus funding *spread* (cross-exchange). They look similar and are not: one harvests a level, the other a difference of levels. Why cash-and-carry is the flagship, and why cross-exchange is strategy #2 scoped to alts and dislocations. The extra risks the spread takes on: inter-venue basis, a second counterparty, inventory drift — and the correction that no single off-exchange rail spans Binance and Bybit. Chips: `tutorial §2.1`, `tutorial §2.2`, `tutorial §5.2`, `core §2.3`.
- **`edge/costs`** — the real cost stack: spot taker fees are materially higher than perp and must be fetched separately, never reused from the perp figure. Round trip at VIP0 taker ≈ 0.34–0.38%. Minimum holds are weeks — the same length-scale as negative funding regimes, which makes regime persistence the dominant PnL driver rather than a tail risk. Report on the full allocated bankroll, never on posted margin: at 10× leverage the same $30/day reads as ~110% APR on margin and it is a mirage. Chips: `core §2.4`, `core §2.1`, `core §2.2`, `tutorial §1.5`, `tutorial §3.9`.
- **`edge/gate`** — the hurdle is the near-risk-free floor (tokenized T-bills ~4.6%), haircut-adjusted and never called risk-free, and it is a dated, sourced, revision-controlled input rather than a hardcoded constant. The economic gate is a pre-registered, versioned calibration artifact produced *before* the promotion sample is scored, carrying a frozen dataset, holdout identifiers, a content hash, rationale and an approval record; any change forces a new version and a complete rerun. Why that governance exists: two implementations agreeing proves determinism, not independence — pre-registration is what prevents tuning until it passes. Chips: `core §2.1`, `core §8`, `core §12`.
- **`edge/forks`** — milestone 2 is a kill gate and the spec predicts it fails. The four documented forks: maker spot entry; regime-selective deployment; the alt/dislocation niche arriving earlier than planned; or consciously accepting sub-hurdle returns as tuition for the platform. Which fork to take is explicitly a user decision, not an automatic one. Link back to the calculator presets. Chips: `core §12`, `core §2.4`, `core §2.3`.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `cd docs/guide && bun test`
Expected: PASS — all tests.

- [ ] **Step 6: Commit**

```bash
git add docs/guide/
git commit -m "guide: Part 2 — the edge stated honestly, wrapped around the calculator"
```

---

### Task 8: Theme toggle, accessibility, responsive pass

**Files:**
- Modify: `docs/guide/index.html`
- Test: `docs/guide/tests/structure.test.js`

**Interfaces:**
- Consumes: everything prior.
- Produces: nothing new for later tasks.

- [ ] **Step 1: Write the failing theme test**

Append to `docs/guide/tests/structure.test.js`:

```js
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
});
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd docs/guide && bun test structure`
Expected: FAIL on the scroll-container assertion if any table in Parts 1–2 was authored without a `.scroll-x` wrapper.

- [ ] **Step 3: Fix the failures and add the theme toggle**

Wrap every `<table>` in `<div class="scroll-x">`. Add the toggle to the rail, above the search input:

```html
  <button class="theme-toggle" id="theme-toggle" type="button" aria-label="Switch colour theme">◐ theme</button>
```

```css
  .theme-toggle {
    font: inherit; font-size: 0.75rem; padding: 0.25rem 0.5rem; margin-bottom: 0.5rem;
    cursor: pointer; background: transparent; color: var(--muted);
    border: 1px solid var(--rule); border-radius: 0.3rem;
  }
```

```js
  document.getElementById("theme-toggle").addEventListener("click", function () {
    var root = document.documentElement;
    var current = root.getAttribute("data-theme");
    if (!current) {
      var prefersDark = window.matchMedia("(prefers-color-scheme: dark)").matches;
      current = prefersDark ? "dark" : "light";
    }
    root.setAttribute("data-theme", current === "dark" ? "light" : "dark");
  });
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd docs/guide && bun test`
Expected: PASS — all tests.

- [ ] **Step 5: Verify responsive and theme behaviour by hand**

Run: `open docs/guide/index.html`
Check, and fix anything that fails:
1. At a 375 px-wide window there is no horizontal page scroll; the rail sits above the content.
2. The toggle flips light↔dark and overrides the OS setting in both directions.
3. Tab order reaches the theme toggle, the search box, every nav link, and every calculator input.
4. Text remains legible in both themes, including the pass/fail verdict colours.

- [ ] **Step 6: Commit**

```bash
git add docs/guide/
git commit -m "guide: theme toggle, accessible names, responsive and scroll-container pass"
```

---

### Task 9: Publish stage 1 and open the voice-review gate

**Files:**
- Modify: `docs/guide/README.md`
- Modify: `docs/HANDOFF.md`

**Interfaces:**
- Consumes: the finished stage-1 `index.html`.
- Produces: a published Artifact URL recorded in the README, and a HANDOFF entry so a fresh session finds the guide.

- [ ] **Step 1: Run the full suite one final time**

Run: `cd docs/guide && bun test`
Expected: PASS. Record the actual test count in the commit message; do not claim a pass without having seen this output.

- [ ] **Step 2: Audit the citations by hand**

The automated test proves every article *has* chips; it cannot prove they point anywhere real. For each distinct `data-src` value in the file, open the cited document and confirm the section exists and says what the guide claims it says.

Run this to list every distinct citation and how often it is used:

```bash
grep -o 'data-src="[^"]*"' docs/guide/index.html | sort | uniq -c | sort -rn
```

Fix any chip that points at a section that does not exist or does not support the claim. A wrong chip is worse than no chip — it makes an unsupported claim look sourced.

- [ ] **Step 3: Publish the Artifact**

Use the Artifact tool with `file_path: docs/guide/index.html`, a `favicon`, and a one-sentence `description`. Keep the favicon stable across every future redeploy — stages 2–4 republish to this same URL.

- [ ] **Step 4: Record the URL in the README**

Add to `docs/guide/README.md` under a `## Published` heading: the artifact URL, and the note that stages 2–4 redeploy to it by publishing the same file path.

- [ ] **Step 5: Add the HANDOFF entry**

Add a short paragraph to `docs/HANDOFF.md` under Artifacts: the guide's path, its purpose (orientation before implementation), its design and plan documents, the published URL, and that stage 1 covers Parts 1–2 with stages 2–4 pending the voice review.

- [ ] **Step 6: Commit**

```bash
git add docs/guide/README.md docs/HANDOFF.md
git commit -m "guide: publish stage 1 and record it in the handoff"
```

- [ ] **Step 7: Open the voice-review gate**

Present to the user: the published URL, what stage 1 covers, and the specific question the gate exists to answer — **is the voice, density and altitude right before Parts 3–6 are written?** Do not begin stage 2 until that is answered.

---

## Follow-up recorded during planning

**A spec inconsistency to triage, not to fix here.** `core §12` cites §2.4 for "spot maker entry ≈ −40% round-trip cost". `core §2.4` itself says maker spot entry "roughly **halves** round-trip cost and shortens **break-even** by ~40%" — that is a −50% cost effect and a separate −40% break-even effect, so §12's paraphrase appears to have collapsed the two. This plan models the maker preset on §2.4's own wording (a 0.5 multiplier on round-trip cost) rather than the paraphrase, which is why the preset's expected result is 4.65%.

Per the design doc's out-of-scope rule, a spec defect surfaced while writing the guide is **a finding to be recorded and triaged like any other, never a silent fix and never fixed in the guide alone**. Raise it with the user at the stage-1 gate for a tag and an ADR §3 entry; it is small, but it is exactly the kind of drift the citation discipline exists to catch.
