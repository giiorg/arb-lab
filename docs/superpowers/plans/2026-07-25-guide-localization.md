# Guide Localization (EN/KA) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `docs/guide/index.html` a bilingual English/Georgian guide with a language switcher, without changing what it teaches or how it computes.

**Architecture:** Prose duplicates as sibling `<article lang="en">` / `<article lang="ka">` blocks; controls (calculator, search, theme toggle) stay single instances and read their strings from a table in `#guide-pure`. Language rides in the hash (`#/ka/edge/costs`), defaulting to English so existing links keep working. Georgian renders serif via an inlined, subset Noto Serif Georgian. Translation runs through committed `agy` scripts that write reviewable fragments to `docs/guide/i18n/ka/`, so re-translating after the pending voice review is one command per section.

**Tech Stack:** Plain ES5-style JS in two `<script>` blocks (no build step), `bun test`, `agy` v1.1.4 with `Gemini 3.1 Pro (High)`, `fonttools`/`pyftsubset` for the font subset, Node for the splice tooling.

**Spec:** `docs/superpowers/specs/2026-07-25-guide-localization-design.md`

## Global Constraints

- **No document wrapper.** Never add `<!DOCTYPE>`, `<html>`, `<head>` or `<body>` to `index.html` — the Artifact host supplies them. A `<title>` is correct.
- **Self-contained.** No external scripts, stylesheets, fonts, images or network calls. The only permitted `url(...)` in CSS is `data:font/woff2;base64,`. A strict CSP blocks everything else.
- **Two script blocks only.** `#guide-pure` is DOM-free and exposes `GuidePure`; `#guide-ui` does all DOM wiring. Tests evaluate `#guide-pure` only, so anything that needs testing must live there.
- **ES5 syntax in both script blocks.** `var`, `function`, no arrow functions, no template literals, no `const`/`let` — matching the existing code exactly.
- **Citation chips are never translated.** `core §7.4`, `tutorial §3.9`, `Profiles §1` and their kin stay byte-identical in both languages, in both the `data-src` attribute and the visible text.
- **`computeEconomics` is not modified.** One code path serves both languages. The five pinned figures — $90 funding, $38 round trip, 8.4 days, 5.68% in-position, 3.41% vs the 4.60% hurdle — must keep passing. If one fails, the spec and the guide disagree; that is a finding, not a test to relax.
- **Default language is `en`.** `#/edge/costs` (no language segment) means English, forever.
- **Glossary is frozen before bulk translation.** `docs/guide/i18n/glossary.md` is passed verbatim in every prompt and is never edited mid-run.
- **No AI attribution** in commit messages (user rule).
- Run tests with `cd docs/guide && bun test`.

---

### Task 1: Hash routing primitives

Pure functions that turn a hash into `{lang, route}` and back. Nothing consumes them yet — this task exists on its own because the routing rules (legacy links, defaults, language-vs-route ambiguity) are where a subtle bug would be most expensive later.

**Files:**
- Modify: `docs/guide/index.html` (`#guide-pure`, after `ROUTES` at :899-911, and the return object at :1061-1071)
- Test: `docs/guide/tests/routing.test.js` (create)

**Interfaces:**
- Consumes: `ROUTES` (existing, `#guide-pure`)
- Produces: `GuidePure.LANGS` (`["en","ka"]`), `GuidePure.DEFAULT_LANG` (`"en"`), `GuidePure.parseHash(hash) -> {lang, route}`, `GuidePure.formatHash({lang, route}) -> string`

- [ ] **Step 1: Write the failing test**

Create `docs/guide/tests/routing.test.js`:

```js
import { test, expect } from "bun:test";
import { loadPure } from "./extract.js";

test("reads a language segment out of the hash", () => {
  const pure = loadPure();
  expect(pure.parseHash("#/ka/edge/costs")).toEqual({ lang: "ka", route: "edge/costs" });
});

test("treats a hash with no language segment as English (existing links keep working)", () => {
  const pure = loadPure();
  expect(pure.parseHash("#/edge/costs")).toEqual({ lang: "en", route: "edge/costs" });
});

test("falls back to the first route when the hash is empty", () => {
  const pure = loadPure();
  const first = pure.ROUTES[0].route;
  expect(pure.parseHash("")).toEqual({ lang: "en", route: first });
  expect(pure.parseHash("#/")).toEqual({ lang: "en", route: first });
});

test("a bare language segment still resolves to the first route", () => {
  const pure = loadPure();
  expect(pure.parseHash("#/ka")).toEqual({ lang: "ka", route: pure.ROUTES[0].route });
});

test("only exactly en or ka counts as a language segment", () => {
  const pure = loadPure();
  expect(pure.parseHash("#/world/perps").route).toBe("world/perps");
  expect(pure.parseHash("#/english/perps").route).toBe("english/perps");
});

test("English omits its segment, Georgian carries one", () => {
  const pure = loadPure();
  expect(pure.formatHash({ lang: "en", route: "edge/costs" })).toBe("#/edge/costs");
  expect(pure.formatHash({ lang: "ka", route: "edge/costs" })).toBe("#/ka/edge/costs");
});

test("an unknown language formats as the default", () => {
  const pure = loadPure();
  expect(pure.formatHash({ lang: "de", route: "edge/costs" })).toBe("#/edge/costs");
});

test("parse and format round-trip for every route in both languages", () => {
  const pure = loadPure();
  for (const lang of pure.LANGS) {
    for (const r of pure.ROUTES) {
      const state = { lang: lang, route: r.route };
      expect(pure.parseHash(pure.formatHash(state))).toEqual(state);
    }
  }
});
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd docs/guide && bun test tests/routing.test.js`
Expected: FAIL — `pure.parseHash is not a function`.

- [ ] **Step 3: Add the primitives to `#guide-pure`**

Insert immediately after the `ROUTES` array (which ends at `index.html:911`):

```js
  var LANGS = ["en", "ka"];
  var DEFAULT_LANG = "en";

  // The hash carries the language as an optional first segment: "#/ka/edge/costs".
  // English omits it, so every link written before localization still resolves.
  // A segment counts as a language only if it is exactly "en" or "ka" — no part id
  // collides with those, and treating a near-miss as a language would silently
  // swallow a real route segment.
  function parseHash(hash) {
    var raw = String(hash || "").replace(/^#\/?/, "");
    var segs = raw.split("/").filter(function (s) { return s.length > 0; });
    var lang = DEFAULT_LANG;
    if (segs.length && LANGS.indexOf(segs[0]) !== -1) lang = segs.shift();
    var route = segs.join("/");
    return { lang: lang, route: route || ROUTES[0].route };
  }

  function formatHash(state) {
    var lang = LANGS.indexOf(state.lang) !== -1 ? state.lang : DEFAULT_LANG;
    var prefix = lang === DEFAULT_LANG ? "" : lang + "/";
    return "#/" + prefix + state.route;
  }
```

Add to the return object at the end of the block:

```js
    LANGS: LANGS,
    DEFAULT_LANG: DEFAULT_LANG,
    parseHash: parseHash,
    formatHash: formatHash,
```

- [ ] **Step 4: Run the whole suite**

Run: `cd docs/guide && bun test`
Expected: PASS — the 8 new routing tests plus the existing 40, 48 total.

- [ ] **Step 5: Commit**

```bash
git add docs/guide/index.html docs/guide/tests/routing.test.js
git commit -m "guide: hash routing primitives for language selection

parseHash/formatHash put the language in an optional first hash segment.
English omits it, so every link written before localization still resolves
to the same section."
```

---

### Task 2: Language-aware titles and search

Turn the `title` fields into `{en, ka}` bundles and make the search index language-partitioned. Georgian values are omitted here on purpose — the `pick` fallback returns English for any missing key, which is what lets the remaining tasks land Georgian incrementally without the guide ever showing a blank nav.

**Files:**
- Modify: `docs/guide/index.html` (`PARTS` :890-897, `ROUTES` :899-911, `buildSearchIndex` :913-922, `searchIndex` :924-943, return object)
- Test: `docs/guide/tests/i18n.test.js` (create), `docs/guide/tests/search.test.js` (extend)

**Interfaces:**
- Consumes: `LANGS`, `DEFAULT_LANG` (Task 1)
- Produces: `GuidePure.pick(bundle, lang) -> string`; `PARTS[i].title` and `ROUTES[i].title` are now `{en: string, ka?: string}`; `buildSearchIndex(records)` where a record may carry `lang`; `searchIndex(index, query, lang)`

- [ ] **Step 1: Write the failing test**

Create `docs/guide/tests/i18n.test.js`:

```js
import { test, expect } from "bun:test";
import { loadPure } from "./extract.js";

test("pick returns the requested language", () => {
  const pure = loadPure();
  expect(pure.pick({ en: "Costs", ka: "ხარჯები" }, "ka")).toBe("ხარჯები");
});

test("pick falls back to English when a language is missing", () => {
  const pure = loadPure();
  expect(pure.pick({ en: "Costs" }, "ka")).toBe("Costs");
});

test("pick passes a plain string through unchanged", () => {
  const pure = loadPure();
  expect(pure.pick("Costs", "ka")).toBe("Costs");
});

test("every part and route title is a bundle with English", () => {
  const pure = loadPure();
  for (const p of pure.PARTS) expect(typeof p.title.en).toBe("string");
  for (const r of pure.ROUTES) expect(typeof r.title.en).toBe("string");
});
```

Append to `docs/guide/tests/search.test.js`:

```js
const BILINGUAL = [
  { route: "world/funding", lang: "en", title: "The funding rate", text: "The crowded side pays the other side." },
  { route: "world/funding", lang: "ka", title: "funding rate", text: "ბირჟა ხალხმრავალ მხარეს აიძულებს გადახდას." }
];

test("search returns only results in the requested language", () => {
  const pure = loadPure();
  const index = pure.buildSearchIndex(BILINGUAL);
  const en = pure.searchIndex(index, "crowded", "en");
  expect(en.length).toBe(1);
  expect(en[0].lang).toBe("en");
  expect(pure.searchIndex(index, "crowded", "ka")).toEqual([]);
});

test("a Latin glossary term finds the Georgian section too", () => {
  const pure = loadPure();
  const index = pure.buildSearchIndex(BILINGUAL);
  const hits = pure.searchIndex(index, "funding", "ka");
  expect(hits.length).toBe(1);
  expect(hits[0].lang).toBe("ka");
});

test("records without a language default to English", () => {
  const pure = loadPure();
  const index = pure.buildSearchIndex([{ route: "a/b", title: "T", text: "body text here" }]);
  expect(pure.searchIndex(index, "body", "en").length).toBe(1);
});
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd docs/guide && bun test tests/i18n.test.js tests/search.test.js`
Expected: FAIL — `pure.pick is not a function`, and the language-filtered search returns both records.

- [ ] **Step 3: Implement**

Replace the `PARTS` array (`index.html:890-897`) with:

```js
  var PARTS = [
    { id: "world",   n: 1, title: { en: "The world" } },
    { id: "edge",    n: 2, title: { en: "The edge, honestly" } },
    { id: "why",     n: 3, title: { en: "Why the design looks like this" } },
    { id: "build",   n: 4, title: { en: "What we're building" } },
    { id: "go",      n: 5, title: { en: "The Go build" } },
    { id: "runbook", n: 6, title: { en: "Operator runbook" } }
  ];
```

Replace the `ROUTES` array (`index.html:899-911`) with the same entries, each `title:` wrapped as `{ en: "…" }`:

```js
  var ROUTES = [
    { part: "world", id: "perps",          route: "world/perps",          title: { en: "Spot, futures, perpetuals" } },
    { part: "world", id: "funding",        route: "world/funding",        title: { en: "The funding rate" } },
    { part: "world", id: "prices",         route: "world/prices",         title: { en: "Mark, index, last" } },
    { part: "world", id: "liquidation",    route: "world/liquidation",    title: { en: "Liquidation, margin, ADL" } },
    { part: "world", id: "venues",         route: "world/venues",         title: { en: "Binance and Bybit" } },
    { part: "world", id: "counterparties", route: "world/counterparties", title: { en: "Who is on the other side" } },
    { part: "edge",  id: "two-trades",     route: "edge/two-trades",      title: { en: "Two trades that look alike" } },
    { part: "edge",  id: "costs",          route: "edge/costs",           title: { en: "What it actually costs" } },
    { part: "edge",  id: "calculator",     route: "edge/calculator",      title: { en: "Will this make money?" } },
    { part: "edge",  id: "gate",           route: "edge/gate",            title: { en: "The economic gate" } },
    { part: "edge",  id: "forks",          route: "edge/forks",           title: { en: "When the gate fails" } }
  ];
```

Add `pick` just below `formatHash`:

```js
  // Georgian keys are added section by section. Until one lands, pick returns the
  // English string rather than nothing — a half-translated guide stays usable.
  function pick(bundle, lang) {
    if (bundle == null) return "";
    if (typeof bundle === "string") return bundle;
    return bundle[lang] != null ? bundle[lang] : bundle[DEFAULT_LANG];
  }
```

Replace `buildSearchIndex` and the head of `searchIndex`:

```js
  function buildSearchIndex(records) {
    return records.map(function (r) {
      return {
        route: r.route,
        lang: r.lang || DEFAULT_LANG,
        title: r.title,
        text: r.text,
        haystack: (r.title + " " + r.text).toLowerCase()
      };
    });
  }

  function searchIndex(index, query, lang) {
    var q = String(query || "").trim().toLowerCase();
    if (q.length < 2) return [];
    var want = lang || DEFAULT_LANG;
    var out = [];
    index.forEach(function (entry) {
      if (entry.lang !== want) return;
      var at = entry.haystack.indexOf(q);
      if (at === -1) return;
```

Keep the body of `searchIndex` from `var source = entry.text;` onward exactly as it is, changing only the pushed object to carry the language:

```js
      out.push({ route: entry.route, lang: entry.lang, title: entry.title, snippet: snippet });
```

Add `pick: pick,` to the return object.

- [ ] **Step 4: Fix the one UI reference that breaks**

`index.html:1094` and `:1101` read `part.title` and `r.title` directly and would now render `[object Object]`. Change them to use the parsed language:

```js
  var currentLang = GuidePure.parseHash(location.hash).lang;
```

placed just after `var rail = document.getElementById("rail");`, then:

```js
    h.textContent = part.n + " · " + GuidePure.pick(part.title, currentLang);
```
```js
      a.textContent = GuidePure.pick(r.title, currentLang);
```

Also update the search-record builder at `index.html:1139` from `meta ? meta.title : el.dataset.route` to `meta ? GuidePure.pick(meta.title, currentLang) : el.dataset.route`.

- [ ] **Step 5: Run the whole suite**

Run: `cd docs/guide && bun test`
Expected: PASS. The five original search tests pass unchanged — their fixtures carry no `lang` and their calls pass no language, and both default to English.

- [ ] **Step 6: Verify in the browser**

Run the preview server from `docs/guide/README.md`, open `http://localhost:8731`, and confirm the nav still reads "1 · The world" with all eleven links, not `[object Object]`.

- [ ] **Step 7: Commit**

```bash
git add docs/guide/index.html docs/guide/tests/i18n.test.js docs/guide/tests/search.test.js
git commit -m "guide: language-aware titles and search index

Titles become {en, ka} bundles read through pick(), which falls back to
English for any key not yet translated so the guide stays usable while
Georgian lands section by section. Search records carry a language and
searchIndex filters on it; records and calls that omit it default to
English, so the existing search tests hold unchanged."
```

---

### Task 3: Move the calculator's strings into the pure block

The calculator's labels, output row names and verdict sentences are English literals inside `#guide-ui`, where `bun test` cannot reach them. They have to move to be translatable, and moving them puts them under test for the first time. This task is English-only and changes no rendered text.

**Files:**
- Modify: `docs/guide/index.html` (`PRESETS` :1016-1041, the calculator article :713-754, `render()` :1204-1236, preset wiring :1238-1253)
- Test: `docs/guide/tests/i18n.test.js` (extend)

**Interfaces:**
- Consumes: `pick` (Task 2)
- Produces: `GuidePure.UI` (nested `{en, ka?}` bundles), `GuidePure.fmt(template, vars) -> string`; `PRESETS[id].label` and `.note` become bundles

- [ ] **Step 1: Write the failing test**

Append to `docs/guide/tests/i18n.test.js`:

```js
function bundles(node, path = "", out = []) {
  if (node && typeof node === "object") {
    if (typeof node.en === "string") out.push([path, node]);
    else for (const k of Object.keys(node)) bundles(node[k], path ? path + "." + k : k, out);
  }
  return out;
}

test("every UI string is a bundle with an English value", () => {
  const pure = loadPure();
  const found = bundles(pure.UI);
  expect(found.length).toBeGreaterThan(20);
  for (const [path, b] of found) {
    expect(typeof b.en, `UI.${path} has no English string`).toBe("string");
    expect(b.en.length, `UI.${path} is empty`).toBeGreaterThan(0);
  }
});

test("the verdict templates carry a points placeholder", () => {
  const pure = loadPure();
  expect(pure.UI.calc.verdictPass.en).toContain("{pts}");
  expect(pure.UI.calc.verdictFail.en).toContain("{pts}");
});

test("fmt substitutes named placeholders", () => {
  const pure = loadPure();
  expect(pure.fmt("Clears by {pts} points.", { pts: "1.19" })).toBe("Clears by 1.19 points.");
});

test("fmt leaves an unknown placeholder alone rather than printing undefined", () => {
  const pure = loadPure();
  expect(pure.fmt("a {missing} b", { pts: 1 })).toBe("a {missing} b");
});

test("preset labels and notes are bundles", () => {
  const pure = loadPure();
  for (const id of Object.keys(pure.PRESETS)) {
    expect(typeof pure.PRESETS[id].label.en, `${id} label`).toBe("string");
    expect(typeof pure.PRESETS[id].note.en, `${id} note`).toBe("string");
  }
});

test("the calculator markup binds its text through i18n keys", () => {
  const html = readGuide();
  const calc = html.match(/<article data-route="edge\/calculator"[\s\S]*?<\/article>/)[0];
  for (const key of [
    "calc.title", "calc.field.rate", "calc.field.notional", "calc.field.hurdle"
  ]) {
    expect(calc, `missing data-i18n="${key}"`).toContain(`"${key}"`);
  }
});
```

Add `readGuide` to that file's import line: `import { readGuide, loadPure } from "./extract.js";`

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd docs/guide && bun test tests/i18n.test.js`
Expected: FAIL — `Cannot read properties of undefined (reading 'calc')`.

- [ ] **Step 3: Add the `UI` table and `fmt` to `#guide-pure`**

Insert before `var PRESETS`:

```js
  // Chrome and calculator strings. Prose lives in the markup as sibling <article>
  // blocks; these are the strings attached to controls, which exist once and cannot
  // be duplicated (the calculator alone holds 16 element ids).
  var UI = {
    status:       { en: "spec v0.9.4 · 0 lines of Go" },
    searchLabel:  { en: "Search the guide" },
    searchPlace:  { en: "Search the guide" },
    railLabel:    { en: "Guide contents" },
    themeLabel:   { en: "Switch colour theme" },
    themeText:    { en: "◐ theme" },
    calc: {
      title: { en: "Will this actually make money?" },
      intro: {
        en: "Move the inputs. The verdict is the same test milestone 2 runs, on the same "
          + "denominator the spec insists on — the full allocated bankroll, never posted "
          + "margin. <span class=\"cite\" data-src=\"core §2.1\">core §2.1</span> "
          + "<span class=\"cite\" data-src=\"core §2.2\">core §2.2</span>"
      },
      field: {
        rate:     { en: "Funding rate (% per settlement)" },
        interval: { en: "Funding interval (hours)" },
        hold:     { en: "Hold length (days)" },
        notional: { en: "Notional N (USD)" },
        leverage: { en: "Perp leverage" },
        rescue:   { en: "Rescue margin (USD)" },
        spotfee:  { en: "Spot fee (%)" },
        perpfee:  { en: "Perp fee (%)" },
        slip:     { en: "Slippage per leg (%)" },
        mult:     { en: "Round-trip multiplier" },
        util:     { en: "Utilization (%)" },
        hurdle:   { en: "Hurdle (%)" }
      },
      row: {
        cLegs:        { en: "Leg-capital floor C_legs" },
        cAllocated:   { en: "Allocated bankroll C_allocated" },
        settlements:  { en: "Settlements in the hold" },
        funding:      { en: "Funding collected" },
        roundTrip:    { en: "Round-trip cost" },
        breakEven:    { en: "Days to break even" },
        net:          { en: "Net over the hold" },
        inPosition:   { en: "Annualized while in position" },
        utilization:  { en: "× utilization" },
        netApy:       { en: "Net APY on the bankroll" },
        hurdle:       { en: "Hurdle" }
      },
      verdictPass: { en: "Clears the hurdle by {pts} points." },
      verdictFail: { en: "Fails the hurdle by {pts} points." }
    }
  };

  function fmt(template, vars) {
    return String(template).replace(/\{(\w+)\}/g, function (whole, key) {
      return Object.prototype.hasOwnProperty.call(vars, key) ? String(vars[key]) : whole;
    });
  }
```

Wrap each `PRESETS` `label` and `note` value in `{ en: … }`, leaving `cite` and `input` untouched. For example:

```js
    worked: {
      label: { en: "The worked trade (tutorial §3.9)" },
      note: { en: "A good major regime at VIP0 taker fees. Nets ~3.4% at 60% time-in-position — below the hurdle." },
      cite: "tutorial §3.9",
      input: withInput({})
    },
```

Do the same for `baseline`, `maker` and `regime`, copying their existing English text verbatim.

Add `UI: UI,` and `fmt: fmt,` to the return object.

- [ ] **Step 4: Bind the calculator markup to the table**

In the calculator article (`index.html:713-754`), replace the heading, the intro paragraph and each `<label>`'s text with keyed elements. The inputs, their ids, types, steps and values do not change:

```html
      <article data-route="edge/calculator">
        <h1 data-i18n="calc.title"></h1>
        <p data-i18n-html="calc.intro"></p>

        <div class="calc">
          <div class="calc-presets" id="calc-presets"></div>
          <div class="calc-grid">
            <label><span data-i18n="calc.field.rate"></span>
              <input id="in-rate" type="number" step="0.001" value="0.015"></label>
            <label><span data-i18n="calc.field.interval"></span>
              <select id="in-interval"><option value="8">8</option><option value="1">1</option></select></label>
            <label><span data-i18n="calc.field.hold"></span>
              <input id="in-hold" type="number" step="1" value="20"></label>
            <label><span data-i18n="calc.field.notional"></span>
              <input id="in-notional" type="number" step="1000" value="10000"></label>
            <label><span data-i18n="calc.field.leverage"></span>
              <input id="in-leverage" type="number" step="1" value="3"></label>
            <label><span data-i18n="calc.field.rescue"></span>
              <input id="in-rescue" type="number" step="100" value="3333"></label>
            <label><span data-i18n="calc.field.spotfee"></span>
              <input id="in-spotfee" type="number" step="0.005" value="0.10"></label>
            <label><span data-i18n="calc.field.perpfee"></span>
              <input id="in-perpfee" type="number" step="0.005" value="0.05"></label>
            <label><span data-i18n="calc.field.slip"></span>
              <input id="in-slip" type="number" step="0.005" value="0.02"></label>
            <label><span data-i18n="calc.field.mult"></span>
              <input id="in-mult" type="number" step="0.1" value="1"></label>
            <label><span data-i18n="calc.field.util"></span>
              <input id="in-util" type="number" step="5" value="60"></label>
            <label><span data-i18n="calc.field.hurdle"></span>
              <input id="in-hurdle" type="number" step="0.1" value="4.6"></label>
          </div>
          <div class="scroll-x"><table class="calc-out" id="calc-out"></table></div>
          <p class="verdict" id="calc-verdict"></p>
          <p class="calc-note" id="calc-note"></p>
        </div>
      </article>
```

- [ ] **Step 5: Apply the table in `#guide-ui`**

Add near the top of the UI block, after `currentLang` is defined:

```js
  function applyStrings(lang) {
    document.querySelectorAll("[data-i18n]").forEach(function (el) {
      el.textContent = GuidePure.pick(lookup(el.dataset.i18n), lang);
    });
    document.querySelectorAll("[data-i18n-html]").forEach(function (el) {
      el.innerHTML = GuidePure.pick(lookup(el.dataset.i18nHtml), lang);
    });
  }

  // "calc.field.rate" -> GuidePure.UI.calc.field.rate
  function lookup(path) {
    return path.split(".").reduce(function (node, key) {
      return node == null ? null : node[key];
    }, GuidePure.UI);
  }
```

`data-i18n-html` is used only for `calc.intro`, whose value is authored in the table above and carries the two citation chips. It is never fed anything from outside the file.

In `render()` (`index.html:1204-1236`), replace the hard-coded row labels and verdict text:

```js
    var U = GuidePure.UI.calc;
    var rows = [
      [GuidePure.pick(U.row.cLegs, currentLang),       GuidePure.formatUsd(r.cLegs)],
      [GuidePure.pick(U.row.cAllocated, currentLang),  GuidePure.formatUsd(r.cAllocated)],
      [GuidePure.pick(U.row.settlements, currentLang), String(r.settlements)],
      [GuidePure.pick(U.row.funding, currentLang),     GuidePure.formatUsd(r.fundingUsd)],
      [GuidePure.pick(U.row.roundTrip, currentLang),   GuidePure.formatUsd(-r.roundTripCostUsd)],
      [GuidePure.pick(U.row.breakEven, currentLang),   r.breakEvenDays.toFixed(1)],
      [GuidePure.pick(U.row.net, currentLang),         GuidePure.formatUsd(r.netUsd)],
      [GuidePure.pick(U.row.inPosition, currentLang),  GuidePure.formatPct(r.inPositionApy)],
      [GuidePure.pick(U.row.utilization, currentLang), GuidePure.formatPct(input.utilization)]
    ];
```

```js
      totalRow.insertCell().textContent = GuidePure.pick(U.row.netApy, currentLang);
```
```js
      hurdleRow.insertCell().textContent = GuidePure.pick(U.row.hurdle, currentLang);
```
```js
      verdictEl.className = "verdict " + (r.clearsHurdle ? "pass" : "fail");
      verdictEl.textContent = GuidePure.fmt(
        GuidePure.pick(r.clearsHurdle ? U.verdictPass : U.verdictFail, currentLang),
        { pts: Math.abs(r.shortfallPts).toFixed(2) }
      );
```

In the preset wiring (`index.html:1238-1253`):

```js
      b.textContent = GuidePure.pick(preset.label, currentLang);
```
```js
        noteEl.textContent = GuidePure.pick(preset.note, currentLang) + "  [" + preset.cite + "]";
```

Call `applyStrings(currentLang);` once, before the first `render()`.

- [ ] **Step 6: Run the whole suite**

Run: `cd docs/guide && bun test`
Expected: PASS, including the five pinned economics figures — this task must not move a single number.

- [ ] **Step 7: Verify in the browser**

Preview, open `#/edge/calculator`, and confirm: all twelve labels render, the output table has eleven rows, clicking each of the four presets updates the note, and the default state shows the fail verdict with `1.19` points against the 4.60% hurdle.

- [ ] **Step 8: Commit**

```bash
git add docs/guide/index.html docs/guide/tests/i18n.test.js
git commit -m "guide: move calculator strings into the pure block

Labels, output row names, verdict sentences and preset copy were English
literals in #guide-ui, which the tests cannot reach. They are now bundles in
GuidePure.UI, read through pick(), and covered by tests for the first time.
No rendered text changes."
```

---

### Task 4: Inline the Georgian serif face

**Files:**
- Create: `docs/guide/tools/build-font.sh`
- Modify: `docs/guide/index.html` (`<style>` block :3-160)
- Test: `docs/guide/tests/structure.test.js` (extend)

**Interfaces:**
- Consumes: nothing
- Produces: two `@font-face` rules for `"Noto Serif Georgian"` (weights 400 and 600) and a `:lang(ka)` font stack

- [ ] **Step 1: Write the failing test**

Append to `docs/guide/tests/structure.test.js`:

```js
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
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd docs/guide && bun test tests/structure.test.js`
Expected: FAIL — no `@font-face` rules exist.

- [ ] **Step 3: Write the font build script**

Create `docs/guide/tools/build-font.sh`:

```bash
#!/usr/bin/env bash
# Builds the inlined Georgian serif face for index.html.
#
# The guide is published as a self-contained Artifact under a CSP that blocks font
# CDNs, and none of the body stack (Iowan Old Style, Charter, Palatino, Georgia) has
# Mkhedruli coverage — so without this, Georgian would silently fall back to a system
# sans while English rendered serif. Inlining a subset keeps one visual identity.
#
# Source: Noto Serif Georgian, SIL Open Font License 1.1, from google/fonts.
# Output: base64 for two @font-face rules, ~21 KB and ~22 KB raw (~58 KB base64).
#
# Requires: curl, fonttools (pip install 'fonttools[woff]' brotli)
set -euo pipefail

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

url="https://github.com/google/fonts/raw/main/ofl/notoserifgeorgian/NotoSerifGeorgian%5Bwdth,wght%5D.ttf"
curl -sL --max-time 60 -o "$work/src.ttf" "$url"

# Georgian: Asomtavruli + Mkhedruli (U+10A0-10FF) and Mtavruli (U+1C90-1CBF).
# Latin and punctuation are kept so a Latin glossary term inside Georgian prose
# renders in the same face rather than jumping to the fallback mid-sentence.
ranges="U+10A0-10FF,U+1C90-1CBF,U+0020-007E,U+00A0,U+2013,U+2014,U+2018,U+2019,U+201C,U+201D,U+2026,U+00B7"

for weight in 400 600; do
  fonttools varLib.instancer "$work/src.ttf" wdth=100 "wght=$weight" -o "$work/i.ttf" >/dev/null
  pyftsubset "$work/i.ttf" \
    --output-file="$work/g$weight.woff2" --flavor=woff2 \
    --unicodes="$ranges" --layout-features="kern,liga,locl" \
    --desubroutinize --no-hinting
  printf '=== weight %s (%s bytes raw) ===\n' "$weight" "$(wc -c < "$work/g$weight.woff2" | tr -d ' ')"
  base64 < "$work/g$weight.woff2" | tr -d '\n'
  printf '\n\n'
done
```

Make it executable: `chmod +x docs/guide/tools/build-font.sh`

- [ ] **Step 4: Run it and paste the output into the stylesheet**

Run: `cd docs/guide && ./tools/build-font.sh > /tmp/georgian-font.txt` and confirm both raw sizes land near 21 KB and 22 KB. If `curl` returns a 404, the upstream path changed — check `https://github.com/google/fonts/tree/main/ofl/notoserifgeorgian` for the current filename before editing anything else.

Insert at the very top of the `<style>` block (`index.html:3`), before the `:root` rule, substituting the two base64 strings:

```css
  /* Noto Serif Georgian (OFL 1.1), instanced and subset by tools/build-font.sh.
     Inlined because the artifact CSP blocks font CDNs and no face in the serif
     stack covers Mkhedruli. unicode-range keeps English pages off it entirely. */
  @font-face {
    font-family: "Noto Serif Georgian";
    src: url(data:font/woff2;base64,PASTE_WEIGHT_400_HERE) format("woff2");
    font-weight: 400; font-style: normal; font-display: swap;
    unicode-range: U+10A0-10FF, U+1C90-1CBF;
  }
  @font-face {
    font-family: "Noto Serif Georgian";
    src: url(data:font/woff2;base64,PASTE_WEIGHT_600_HERE) format("woff2");
    font-weight: 600; font-style: normal; font-display: swap;
    unicode-range: U+10A0-10FF, U+1C90-1CBF;
  }
```

Then add, immediately after the `:root[data-theme="dark"]` block:

```css
  /* Georgian body prose keeps the serif identity; headings stay on the sans stack,
     which .SF Georgian covers. Georgian x-height runs larger than Latin at the same
     size, so it is nudged down a hair to keep the two languages optically equal. */
  :lang(ka) { font-family: "Noto Serif Georgian", var(--serif); font-size: 0.98em; }
  .guide h1:lang(ka), .guide h2:lang(ka), .guide h3:lang(ka) { font-family: var(--sans); }
```

- [ ] **Step 5: Run the whole suite**

Run: `cd docs/guide && bun test`
Expected: PASS, including the pre-existing "references no external resources" test.

- [ ] **Step 6: Verify the face actually renders**

Preview, and in the browser console run:

```js
document.fonts.check('16px "Noto Serif Georgian"')
```

Expected: `true`. Then temporarily set a paragraph's text to `ქართული ტექსტი` with `lang="ka"` and confirm it renders serif, not sans. Undo the temporary edit.

- [ ] **Step 7: Commit**

```bash
git add docs/guide/index.html docs/guide/tools/build-font.sh
git commit -m "guide: inline a subset Noto Serif Georgian

None of the body serif stack covers Mkhedruli and the artifact CSP blocks font
CDNs, so Georgian would have fallen back to system sans while English rendered
serif. build-font.sh instances the variable font to two weights and subsets it
to the Georgian blocks plus Latin — ~58 KB base64, reproducible rather than a
mystery string."
```

---

### Task 5: Language switching in the DOM

Wire the machinery: mark prose articles `lang="en"`, add the switcher, and make routing, nav, search and the calculator all follow the active language. No Georgian content yet — switching to Georgian shows the English articles with Georgian chrome falling back to English, which is the correct intermediate state.

**Files:**
- Modify: `docs/guide/index.html` (`.guide` container :162, rail :163-167, status :169, all eleven `<article>` tags, `#guide-ui` :1076-1165)
- Test: `docs/guide/tests/structure.test.js` (extend)

**Interfaces:**
- Consumes: `parseHash`, `formatHash`, `pick`, `UI`, `searchIndex` (Tasks 1-3)
- Produces: `.guide[data-lang]` as the active-language signal; `article[data-route][lang]` as the prose unit; `#lang-switch` as the control

- [ ] **Step 1: Write the failing test**

Append to `docs/guide/tests/structure.test.js`:

```js
test("the guide container declares an active language", () => {
  expect(readGuide()).toMatch(/<div class="guide" data-lang="en">/);
});

test("a language switch offers both languages with pressed state", () => {
  const html = readGuide();
  const group = html.match(/<div class="lang-switch"[\s\S]*?<\/div>/);
  expect(group, "no .lang-switch group found").not.toBeNull();
  expect(group[0]).toMatch(/data-lang-choice="en"/);
  expect(group[0]).toMatch(/data-lang-choice="ka"/);
  expect(group[0]).toMatch(/aria-pressed=/);
  expect(html).toMatch(/<div class="lang-switch"[^>]*role="group"[^>]*aria-label=/);
});

test("every prose article declares its language", () => {
  const html = readGuide();
  const opens = [...html.matchAll(/<article data-route="([^"]+)"([^>]*)>/g)];
  for (const [, route, rest] of opens) {
    if (route === "edge/calculator") continue; // a control, localized in place
    expect(rest, `article ${route} has no lang attribute`).toMatch(/lang="(en|ka)"/);
  }
});

test("navigation links are built through formatHash, not string concatenation", () => {
  expect(readGuide()).not.toMatch(/a\.href\s*=\s*"#\/"\s*\+/);
});
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd docs/guide && bun test tests/structure.test.js`
Expected: FAIL — no `data-lang`, no `.lang-switch`.

- [ ] **Step 3: Update the markup**

Change the container (`index.html:162`):

```html
<div class="guide" data-lang="en">
```

Add the switcher to the rail, after the theme toggle (`index.html:164`):

```html
    <div class="lang-switch" id="lang-switch" role="group" aria-label="Language · ენა">
      <button type="button" data-lang-choice="en" aria-pressed="true">EN</button>
      <button type="button" data-lang-choice="ka" aria-pressed="false" lang="ka">ქარ</button>
    </div>
```

The group's `aria-label` is deliberately bilingual and static, and is **not** in the
`UI` table. A language switcher has to be legible to someone who cannot read the
language currently active — localizing it would hide the way out for exactly the
reader who needs it.

Give the status line a key (`index.html:169`):

```html
    <div class="status" data-i18n="status"></div>
```

Add `lang="en"` to all ten prose articles (every `<article data-route="…">` **except** `edge/calculator`):

```html
      <article data-route="world/perps" lang="en">
```

Add styling for the switcher next to the existing `.theme-toggle` rules:

```css
  .lang-switch { display: flex; gap: 0.25rem; margin: 0 0 0.75rem; }
  .lang-switch button {
    flex: 1; font: 0.78rem/1 var(--mono); letter-spacing: 0.02em;
    padding: 0.38rem 0.3rem; cursor: pointer;
    color: var(--muted); background: transparent;
    border: 1px solid var(--rule); border-radius: 4px;
  }
  .lang-switch button[aria-pressed="true"] {
    color: var(--surface); background: var(--accent); border-color: var(--accent);
  }
```

- [ ] **Step 4: Rewrite the UI block's routing and rendering**

Replace the nav builder, `show`, `routeFromHash` and the search wiring (`index.html:1090-1165`) with:

`currentLang` already exists from Task 2 (declared just after `var rail = …`) — do not
declare it a second time. Add `guideEl` beside it:

```js
  var guideEl = document.querySelector(".guide");
```

Then the replacement block:

```js
  function renderNav(lang) {
    rail.querySelectorAll("h2, .nav-link").forEach(function (el) { el.remove(); });
    GuidePure.PARTS.forEach(function (part) {
      var routes = GuidePure.ROUTES.filter(function (r) { return r.part === part.id; });
      if (!routes.length) return;
      var h = document.createElement("h2");
      h.textContent = part.n + " · " + GuidePure.pick(part.title, lang);
      h.setAttribute("lang", lang);
      rail.appendChild(h);
      routes.forEach(function (r) {
        var a = document.createElement("a");
        a.className = "nav-link";
        a.href = GuidePure.formatHash({ lang: lang, route: r.route });
        a.dataset.navRoute = r.route;
        a.setAttribute("lang", lang);
        a.textContent = GuidePure.pick(r.title, lang);
        rail.appendChild(a);
      });
    });
  }

  function show(route, lang) {
    var articles = document.querySelectorAll("article[data-route]");
    var found = false;
    articles.forEach(function (el) {
      var articleLang = el.getAttribute("lang");
      // An article with no lang (the calculator) belongs to every language.
      var on = el.dataset.route === route && (!articleLang || articleLang === lang);
      el.classList.toggle("is-active", on);
      if (on) found = true;
    });
    if (!found) {
      // Unknown route, or that route has no article in this language yet: fall back
      // to the same route in the default language, then to the first article.
      var fallback = document.querySelector(
        'article[data-route="' + route + '"][lang="' + GuidePure.DEFAULT_LANG + '"]'
      ) || articles[0];
      if (fallback) {
        fallback.classList.add("is-active");
        route = fallback.dataset.route;
      }
    }
    rail.querySelectorAll(".nav-link").forEach(function (a) {
      a.setAttribute("aria-current", a.dataset.navRoute === route ? "true" : "false");
    });
    var pane = document.querySelector(".pane");
    if (pane) pane.scrollTop = 0;
    window.scrollTo(0, 0);
    return route;
  }

  function setLanguage(lang) {
    currentLang = lang;
    guideEl.setAttribute("data-lang", lang);
    document.getElementById("lang-switch").querySelectorAll("button").forEach(function (b) {
      b.setAttribute("aria-pressed", String(b.dataset.langChoice === lang));
    });
    applyStrings(lang);
    renderNav(lang);
    var box = document.getElementById("search");
    box.placeholder = GuidePure.pick(GuidePure.UI.searchPlace, lang);
    box.setAttribute("aria-label", GuidePure.pick(GuidePure.UI.searchLabel, lang));
    rail.setAttribute("aria-label", GuidePure.pick(GuidePure.UI.railLabel, lang));
    var theme = document.getElementById("theme-toggle");
    theme.setAttribute("aria-label", GuidePure.pick(GuidePure.UI.themeLabel, lang));
    theme.textContent = GuidePure.pick(GuidePure.UI.themeText, lang);
    if (typeof rerenderCalculator === "function") rerenderCalculator();
  }

  function applyState() {
    var state = GuidePure.parseHash(location.hash);
    if (state.lang !== currentLang) setLanguage(state.lang);
    show(state.route, state.lang);
  }

  document.getElementById("lang-switch").addEventListener("click", function (e) {
    var choice = e.target && e.target.dataset ? e.target.dataset.langChoice : null;
    if (!choice) return;
    location.hash = GuidePure.formatHash({
      lang: choice,
      route: GuidePure.parseHash(location.hash).route
    });
  });

  window.addEventListener("hashchange", applyState);
  setLanguage(currentLang);
  applyState();

  var records = [];
  document.querySelectorAll("article[data-route]").forEach(function (el) {
    var meta = GuidePure.ROUTES.filter(function (r) { return r.route === el.dataset.route; })[0];
    var langs = el.getAttribute("lang") ? [el.getAttribute("lang")] : GuidePure.LANGS;
    langs.forEach(function (lang) {
      records.push({
        route: el.dataset.route,
        lang: lang,
        title: meta ? GuidePure.pick(meta.title, lang) : el.dataset.route,
        text: el.textContent.replace(/\s+/g, " ").trim()
      });
    });
  });
  var index = GuidePure.buildSearchIndex(records);

  var box = document.getElementById("search");
  var results = document.getElementById("results");
  box.addEventListener("input", function () {
    var hits = GuidePure.searchIndex(index, box.value, currentLang);
    rail.classList.toggle("searching", box.value.trim().length >= 2);
    results.innerHTML = "";
    hits.forEach(function (hit) {
      var li = document.createElement("li");
      var a = document.createElement("a");
      a.href = GuidePure.formatHash({ lang: hit.lang, route: hit.route });
      a.setAttribute("lang", hit.lang);
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

Note the ordering constraint: `setLanguage` runs before the search index is built, and the calculator block sits after all of this. To let `setLanguage` re-render the calculator on a language switch, declare a hoisted binding above it and assign it inside the calculator block:

```js
  var rerenderCalculator = null;
```

placed beside `var guideEl = …`, and at the end of the `if (calcOut) { … }` block, immediately before the existing final `render();`:

```js
    rerenderCalculator = function () { applyStrings(currentLang); render(); };
```

Since `rerenderCalculator` starts as `null`, the `typeof … === "function"` guard in `setLanguage` correctly skips it on the first call, when the calculator has not been wired yet.

- [ ] **Step 5: Run the whole suite**

Run: `cd docs/guide && bun test`
Expected: PASS.

- [ ] **Step 6: Verify in the browser**

Preview and check each of these:
- The rail shows `EN` pressed and `ქარ` unpressed.
- Clicking `ქარ` changes the hash to `#/ka/world/perps`, keeps you on the same section, and leaves `EN` unpressed.
- Nav links now point at `#/ka/…`; content is still English (expected — nothing is translated yet).
- Loading `#/edge/costs` directly still opens Costs in English.
- The calculator renders and computes identically in both language states.
- Search still returns results in the active language.

- [ ] **Step 7: Commit**

```bash
git add docs/guide/index.html docs/guide/tests/structure.test.js
git commit -m "guide: language switcher, language-aware routing and search

Prose articles now declare lang=en; the calculator stays a single instance and
localizes in place, since duplicating it would duplicate 16 element ids. The
switcher rewrites the hash and keeps your place. Switching to Georgian falls
back to English content, which is the correct state until the prose lands."
```

---

### Task 6: Translation toolchain, proved on one route

Build the `agy` pipeline and use it on `world/perps` only. One route end to end is what proves the prompt rules hold — in particular that citation chips survive — before ten more calls are spent.

**Files:**
- Create: `docs/guide/i18n/glossary.md`, `docs/guide/tools/i18n.mjs`, `docs/guide/tools/translate.sh`
- Modify: `docs/guide/index.html` (adds the first `lang="ka"` article)
- Test: `docs/guide/tests/parity.test.js` (create)

**Interfaces:**
- Consumes: the `article[data-route][lang]` markup shape (Task 5)
- Produces: `node tools/i18n.mjs extract <route> <lang>` (prints a fragment), `node tools/i18n.mjs splice <route>` (writes the ka fragment into `index.html`), `./tools/translate.sh <route>` (writes `i18n/ka/<route with / as __>.html`)

- [ ] **Step 1: Write the glossary**

Create `docs/guide/i18n/glossary.md`, copying the term lists from the design spec verbatim:

```markdown
# Translation glossary — English → Georgian

Frozen before bulk translation. Passed verbatim in every prompt. Never edited
mid-run: terminology drift between sections is the failure mode this prevents.

## Keep in Latin script, untranslated

Spec and venue vocabulary — what exchange UIs show and what the normative docs
call these things:

perpetual, perp, spot, futures, funding rate, funding interval, settlement,
mark price, index price, last price, basis, maker, taker, slippage, notional,
APY, hurdle, round trip, utilization, ADL, auto-deleveraging, fence, actuator,
order book, VIP0, Binance, Bybit, BTC, USDT, USDC, bracket, IOC

## Translate into Georgian

| English | Georgian |
|---|---|
| exchange (the venue) | ბირჟა |
| fee | საკომისიო |
| profit / loss | მოგება / ზარალი |
| price | ფასი |
| position | პოზიცია |
| account | ანგარიში |
| capital / bankroll | კაპიტალი |
| risk | რისკი |
| cost | ხარჯი |
| trade (noun) | გარიგება |
| margin | მარჟა |
| liquidation | ლიკვიდაცია |
| leverage | ბერკეტი |

## Never touched in either language

Citation chip contents — `core §7.4`, `tutorial §3.9`, `Profiles §1`, `SM §4` —
are pointers into English normative documents. Neither translated nor
transliterated. A test enforces this.

Numbers, currency and percentages keep their English formatting (`$10,000`,
`3.41%`), because they must stay tied to the figures printed in the spec.
```

- [ ] **Step 2: Write the extract/splice tool**

Create `docs/guide/tools/i18n.mjs`:

```js
#!/usr/bin/env node
// Extracts and splices per-language <article> blocks in index.html.
//
//   node tools/i18n.mjs extract <route> <lang>   -> prints the fragment
//   node tools/i18n.mjs splice  <route>          -> writes i18n/ka/<route>.html in
//
// Splice is idempotent: it inserts the ka article after its en sibling on the first
// run and replaces it in place afterwards, so re-translating one section is safe to
// repeat. It refuses to write a fragment whose citation chips do not match the
// English original — catching a mangled translation at build time, not review time.
import { readFileSync, writeFileSync } from "node:fs";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";

const HERE = dirname(fileURLToPath(import.meta.url));
const GUIDE = join(HERE, "..", "index.html");

const esc = (s) => s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
const articleRe = (route, lang) =>
  new RegExp(`<article data-route="${esc(route)}" lang="${lang}">[\\s\\S]*?<\\/article>`);

export const fragmentPath = (route) =>
  join(HERE, "..", "i18n", "ka", route.replace(/\//g, "__") + ".html");

export function extract(html, route, lang) {
  const m = html.match(articleRe(route, lang));
  if (!m) throw new Error(`no <article data-route="${route}" lang="${lang}"> in index.html`);
  return m[0];
}

export const cites = (fragment) =>
  [...fragment.matchAll(/data-src="([^"]+)"/g)].map((m) => m[1]).sort();

function splice(route) {
  const html = readFileSync(GUIDE, "utf8");
  const en = extract(html, route, "en");
  let ka = readFileSync(fragmentPath(route), "utf8").trim();

  // Models sometimes wrap output in a code fence despite being told not to.
  ka = ka.replace(/^```[a-z]*\n?/i, "").replace(/\n?```$/, "").trim();

  const open = `<article data-route="${route}" lang="ka">`;
  if (!ka.startsWith(open)) {
    throw new Error(`fragment for ${route} must start with ${open}\n  got: ${ka.slice(0, 90)}`);
  }
  if (!ka.endsWith("</article>")) throw new Error(`fragment for ${route} is truncated`);

  const a = cites(en).join("|");
  const b = cites(ka).join("|");
  if (a !== b) throw new Error(`citation chips differ for ${route}\n  en: ${a}\n  ka: ${b}`);

  const existing = html.match(articleRe(route, "ka"));
  const next = existing
    ? html.replace(articleRe(route, "ka"), () => ka)
    : html.replace(en, () => en + "\n\n      " + ka);
  writeFileSync(GUIDE, next);
  console.log(`${existing ? "replaced" : "inserted"} ka article for ${route}`);
}

const [cmd, route, lang] = process.argv.slice(2);
if (cmd === "extract") process.stdout.write(extract(readFileSync(GUIDE, "utf8"), route, lang || "en"));
else if (cmd === "splice") splice(route);
else if (cmd) {
  console.error("usage: i18n.mjs extract <route> [lang] | i18n.mjs splice <route>");
  process.exit(2);
}
```

Note both `html.replace` calls pass a **function** as the replacement. A plain string would let `$&` or `$1` sequences inside translated prose be interpreted as replacement patterns and corrupt the output.

- [ ] **Step 3: Write the translation script**

Create `docs/guide/tools/translate.sh`:

```bash
#!/usr/bin/env bash
# Translates one guide section into Georgian.
#
#   ./tools/translate.sh world/perps
#
# Writes i18n/ka/<route>.html for review. Splice it in with:
#   node tools/i18n.mjs splice world/perps
#
# Fragments live on disk on purpose: the English prose is still pending a voice
# review, so re-translating a section has to cost one command, not human work.
set -euo pipefail

route="${1:?usage: translate.sh <route>   e.g. world/perps}"
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
out="$here/../i18n/ka/${route//\//__}.html"
mkdir -p "$(dirname "$out")"

fragment="$(node "$here/i18n.mjs" extract "$route" en)"
glossary="$(cat "$here/../i18n/glossary.md")"

read -r -d '' prompt <<EOF || true
You are translating one section of a technical guide about crypto funding-rate
arbitrage from English into Georgian, for a Georgian-speaking software engineer
who trades. Write natural, fluent Georgian prose — not a word-for-word rendering.
Keep the original's plain, direct, unhyped register.

THREE HARD RULES:

1. Preserve every HTML tag and attribute EXACTLY as given, including class and
   data-src values and the order of attributes. Translate only visible text.
   The opening tag must become exactly:
       <article data-route="$route" lang="ka">
   Every other tag stays byte-identical.

2. NEVER translate or transliterate the text inside <span class="cite"> elements,
   and never alter a data-src attribute. "core §7.4" and "tutorial §3.9" are
   pointers into English normative documents and stay verbatim.

3. Output ONLY the translated HTML fragment. No preamble, no explanation, no
   markdown code fence.

Numbers, currency and percentages keep their English form: \$10,000 and 3.41%.

GLOSSARY — follow it exactly, every occurrence:

$glossary

SECTION TO TRANSLATE:

$fragment
EOF

echo "translating $route …" >&2
agy --model "Gemini 3.1 Pro (High)" -p "$prompt" > "$out"
echo "wrote $out ($(wc -c < "$out" | tr -d ' ') bytes)" >&2
```

Make both executable: `chmod +x docs/guide/tools/translate.sh`

- [ ] **Step 4: Write the failing parity test**

Create `docs/guide/tests/parity.test.js`:

```js
import { test, expect } from "bun:test";
import { readGuide, loadPure } from "./extract.js";

const MKHEDRULI = /[Ⴀ-ჿᲐ-Ჿ]/;

export function articles(html) {
  const out = [];
  const re = /<article data-route="([^"]+)"(?: lang="(en|ka)")?>([\s\S]*?)<\/article>/g;
  for (const m of html.matchAll(re)) out.push({ route: m[1], lang: m[2] || null, body: m[3] });
  return out;
}

const cites = (body) => [...body.matchAll(/data-src="([^"]+)"/g)].map((m) => m[1]).sort();
const translated = () => articles(readGuide()).filter((a) => a.lang === "ka");

test("at least one section is translated", () => {
  expect(translated().length).toBeGreaterThan(0);
});

test("every translated section has an English sibling", () => {
  const en = new Set(articles(readGuide()).filter((a) => a.lang === "en").map((a) => a.route));
  for (const ka of translated()) expect(en.has(ka.route), `${ka.route} has no en sibling`).toBe(true);
});

test("citation chips are identical between languages", () => {
  const all = articles(readGuide());
  for (const ka of translated()) {
    const en = all.find((a) => a.route === ka.route && a.lang === "en");
    expect(cites(ka.body), `citations differ for ${ka.route}`).toEqual(cites(en.body));
  }
});

test("no citation chip text is Georgian", () => {
  for (const ka of translated()) {
    for (const m of ka.body.matchAll(/<span class="cite" data-src="[^"]*">([^<]*)<\/span>/g)) {
      expect(MKHEDRULI.test(m[1]), `translated citation in ${ka.route}: ${m[1]}`).toBe(false);
    }
  }
});

test("every translated section actually contains Georgian", () => {
  for (const ka of translated()) {
    expect(MKHEDRULI.test(ka.body), `${ka.route} contains no Georgian`).toBe(true);
  }
});

test("no translated section is a copy of its English sibling", () => {
  const all = articles(readGuide());
  for (const ka of translated()) {
    const en = all.find((a) => a.route === ka.route && a.lang === "en");
    expect(ka.body.trim(), `${ka.route} is untranslated`).not.toBe(en.body.trim());
  }
});

test("glossary terms stay in Latin script inside Georgian prose", () => {
  const html = readGuide();
  const all = articles(html);
  const checks = [
    { route: "world/funding", term: "funding" },
    { route: "world/perps", term: "spot" }
  ];
  for (const { route, term } of checks) {
    const ka = all.find((a) => a.route === route && a.lang === "ka");
    if (!ka) continue; // not translated yet
    expect(ka.body.toLowerCase(), `${route} lost the Latin term "${term}"`).toContain(term);
  }
});

test("each route has at most one article per language", () => {
  const seen = new Set();
  for (const a of articles(readGuide())) {
    const key = a.route + "/" + a.lang;
    expect(seen.has(key), `duplicate article ${key}`).toBe(false);
    seen.add(key);
  }
});

test("every declared route still has an English article", () => {
  const en = new Set(articles(readGuide()).filter((a) => a.lang === "en").map((a) => a.route));
  for (const r of loadPure().ROUTES) {
    if (r.route === "edge/calculator") continue; // a control, localized in place
    expect(en.has(r.route), `${r.route} has no English article`).toBe(true);
  }
});
```

- [ ] **Step 5: Run the test to verify it fails**

Run: `cd docs/guide && bun test tests/parity.test.js`
Expected: FAIL on "at least one section is translated" — nothing is translated yet.

- [ ] **Step 6: Translate and splice one route**

```bash
cd docs/guide
./tools/translate.sh world/perps
```

Read `i18n/ka/world__perps.html` before splicing. Check by eye: the opening tag is `<article data-route="world/perps" lang="ka">`, the `<table>` and `<strong>` markup survived, the three `<span class="cite">` chips read `tutorial §1.1`, `core §4.1`, `tutorial §9` in Latin, and `spot`/`perpetual` are Latin inside Georgian sentences. If any rule was broken, fix the prompt in `translate.sh` and re-run — do not hand-patch the fragment, or the next re-translation loses the fix.

```bash
node tools/i18n.mjs splice world/perps
```

- [ ] **Step 7: Run the whole suite**

Run: `cd docs/guide && bun test`
Expected: PASS.

- [ ] **Step 8: Verify in the browser**

Preview and open `#/ka/world/perps`. Confirm the prose is Georgian, renders **serif** (not the system sans), the table renders, and the citation chips still read `tutorial §1.1` and friends. Switch to `EN` and back and confirm you stay on the same section.

- [ ] **Step 9: Commit**

```bash
git add docs/guide/i18n docs/guide/tools docs/guide/index.html docs/guide/tests/parity.test.js
git commit -m "guide: translation toolchain, proved on world/perps

translate.sh calls agy with the frozen glossary and three hard rules; i18n.mjs
splices the reviewed fragment into index.html idempotently and refuses any
fragment whose citation chips do not match the English original. Fragments live
on disk so re-translating after the voice review is one command per section.

Parity tests assert what review cannot: citations identical across languages,
no Georgian inside a citation chip, no section left in English."
```

---

### Task 7: Georgian for the chrome and the calculator

**Files:**
- Modify: `docs/guide/index.html` (`UI` and `PRESETS` in `#guide-pure`, `PARTS`/`ROUTES` titles)
- Test: `docs/guide/tests/i18n.test.js` (extend)

**Interfaces:**
- Consumes: `UI`, `PRESETS`, `pick` (Tasks 2-3)
- Produces: `ka` values on every `UI` bundle, every `PRESETS` label/note, and every `PARTS`/`ROUTES` title

- [ ] **Step 1: Write the failing test**

Append to `docs/guide/tests/i18n.test.js`:

```js
const MKHEDRULI = /[Ⴀ-ჿᲐ-Ჿ]/;

test("every UI string has a Georgian value", () => {
  const pure = loadPure();
  for (const [path, b] of bundles(pure.UI)) {
    expect(typeof b.ka, `UI.${path} has no Georgian`).toBe("string");
  }
});

test("every route and part title has a Georgian value", () => {
  const pure = loadPure();
  for (const p of pure.PARTS) expect(typeof p.title.ka, `part ${p.id}`).toBe("string");
  for (const r of pure.ROUTES) expect(typeof r.title.ka, `route ${r.route}`).toBe("string");
});

test("the Georgian verdict templates keep the placeholder", () => {
  const pure = loadPure();
  expect(pure.UI.calc.verdictPass.ka).toContain("{pts}");
  expect(pure.UI.calc.verdictFail.ka).toContain("{pts}");
});

test("Georgian UI strings are actually Georgian where they are prose", () => {
  const pure = loadPure();
  expect(MKHEDRULI.test(pure.UI.calc.title.ka)).toBe(true);
  expect(MKHEDRULI.test(pure.UI.calc.verdictFail.ka)).toBe(true);
  expect(MKHEDRULI.test(pure.UI.searchPlace.ka)).toBe(true);
});

test("the Georgian calculator intro keeps both citation chips", () => {
  const pure = loadPure();
  expect(pure.UI.calc.intro.ka).toContain('data-src="core §2.1"');
  expect(pure.UI.calc.intro.ka).toContain('data-src="core §2.2"');
});

test("the Georgian preset notes keep their spec citations verbatim", () => {
  const pure = loadPure();
  for (const id of Object.keys(pure.PRESETS)) {
    expect(MKHEDRULI.test(pure.PRESETS[id].cite), `${id} cite was translated`).toBe(false);
  }
});
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd docs/guide && bun test tests/i18n.test.js`
Expected: FAIL — `UI.status has no Georgian`.

- [ ] **Step 3: Translate the string table**

The strings are short and interdependent, so translate them in one call rather than one per string. Extract them to a scratch file, then:

```bash
cd docs/guide
node -e '
const fs = require("fs");
const html = fs.readFileSync("index.html", "utf8");
const src = html.match(/<script id="guide-pure">([\s\S]*?)<\/script>/)[1];
const pure = new Function(src + "\n;return GuidePure;")();
const out = [];
(function walk(node, path) {
  if (node && typeof node === "object") {
    if (typeof node.en === "string") out.push(path + "\t" + node.en);
    else Object.keys(node).forEach((k) => walk(node[k], path ? path + "." + k : k));
  }
})(pure.UI, "");
pure.PARTS.forEach((p) => out.push("part." + p.id + "\t" + p.title.en));
pure.ROUTES.forEach((r) => out.push("route." + r.route + "\t" + r.title.en));
Object.keys(pure.PRESETS).forEach((id) => {
  out.push("preset." + id + ".label\t" + pure.PRESETS[id].label.en);
  out.push("preset." + id + ".note\t"  + pure.PRESETS[id].note.en);
});
fs.writeFileSync("i18n/ka/_strings.en.tsv", out.join("\n") + "\n");
console.log(out.length + " strings");
'
```

Then translate that file:

```bash
./tools/translate-strings.sh
```

Create `docs/guide/tools/translate-strings.sh`:

```bash
#!/usr/bin/env bash
# Translates the UI string table (chrome, nav titles, calculator) in one call.
# Input:  i18n/ka/_strings.en.tsv   (key <TAB> English)
# Output: i18n/ka/_strings.ka.tsv   (key <TAB> Georgian)
set -euo pipefail
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
src="$here/../i18n/ka/_strings.en.tsv"
out="$here/../i18n/ka/_strings.ka.tsv"
glossary="$(cat "$here/../i18n/glossary.md")"

read -r -d '' prompt <<EOF || true
Translate the interface strings of a crypto funding-rate arbitrage guide from
English into Georgian. The audience is a Georgian-speaking engineer who trades.

Input is one string per line: a key, a TAB, then the English text.
Output the SAME keys in the SAME order, one per line, key TAB Georgian text.
No preamble, no code fence, no extra lines.

HARD RULES:
- Keep {pts} placeholders exactly as written.
- Keep any HTML tags and their attributes byte-identical; translate only visible text.
- Never translate spec citations: "core §2.1", "tutorial §3.9", "core §12" stay verbatim.
- "spec v0.9.4 · 0 lines of Go" keeps the version and the numeral; translate only the words.
- Numbers, currency and percentages keep their English form.
- These are UI labels: keep them short. A Georgian label much longer than its English
  original will not fit the rail or the calculator grid.

GLOSSARY — follow it exactly:

$glossary

STRINGS:

$(cat "$src")
EOF

agy --model "Gemini 3.1 Pro (High)" -p "$prompt" > "$out"
echo "wrote $out ($(wc -l < "$out" | tr -d ' ') lines)" >&2
```

`chmod +x docs/guide/tools/translate-strings.sh`

Verify the line count matches the input before using it:

```bash
diff <(cut -f1 i18n/ka/_strings.en.tsv) <(cut -f1 i18n/ka/_strings.ka.tsv) && echo "keys match"
```

If the keys drifted, re-run rather than hand-repairing — a hand-repaired table diverges from what the script would produce next time.

- [ ] **Step 4: Paste the Georgian values into `#guide-pure`**

Add a `ka:` key beside every `en:` key in `UI`, in `PARTS`/`ROUTES` titles, and in each `PRESETS` `label`/`note`, taking values from `_strings.ka.tsv`. Leave `cite` fields untouched. Keep the existing formatting — two-space indent, aligned keys.

- [ ] **Step 5: Run the whole suite**

Run: `cd docs/guide && bun test`
Expected: PASS — including the five pinned economics figures, which must not move.

- [ ] **Step 6: Verify in the browser**

Preview, switch to `ქარ`, and open `#/ka/edge/calculator`. Confirm: the nav headings and all eleven link titles are Georgian; the twelve calculator labels are Georgian and still fit their grid cells without overflowing; the output table's row names are Georgian while the numbers are unchanged (`$13,333`, `3.41%`); the verdict sentence is Georgian and still reads `1.19`; each preset button's label fits and its note ends with the Latin citation in brackets. Switch back to `EN` and confirm every string reverts.

- [ ] **Step 7: Commit**

```bash
git add docs/guide/index.html docs/guide/i18n docs/guide/tools/translate-strings.sh docs/guide/tests/i18n.test.js
git commit -m "guide: Georgian chrome, nav titles and calculator strings

Translated as one batch so the labels stay consistent with each other, with
tests asserting that {pts} placeholders, spec citations and the computed
numbers survive the round trip untouched."
```

---

### Task 8: Translate the remaining nine sections

**Files:**
- Modify: `docs/guide/index.html` (nine new `lang="ka"` articles)
- Create: nine fragments under `docs/guide/i18n/ka/`
- Test: `docs/guide/tests/parity.test.js` (tighten)

**Interfaces:**
- Consumes: `translate.sh`, `i18n.mjs splice` (Task 6)
- Produces: a `lang="ka"` article for all ten prose routes

- [ ] **Step 1: Tighten the parity test to demand full coverage**

In `docs/guide/tests/parity.test.js`, replace the "at least one section is translated" test with:

```js
test("every prose route has a Georgian article", () => {
  const ka = new Set(translated().map((a) => a.route));
  for (const r of loadPure().ROUTES) {
    if (r.route === "edge/calculator") continue; // a control, localized in place
    expect(ka.has(r.route), `${r.route} has no Georgian article`).toBe(true);
  }
});
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd docs/guide && bun test tests/parity.test.js`
Expected: FAIL — `world/funding has no Georgian article`.

- [ ] **Step 3: Translate the remaining nine routes**

Each `agy` call takes a while. Run them one at a time and read each fragment before splicing:

```bash
cd docs/guide
for route in world/funding world/prices world/liquidation world/venues \
             world/counterparties edge/two-trades edge/costs edge/gate edge/forks; do
  ./tools/translate.sh "$route"
done
```

Then, for each, read `i18n/ka/<route>.html` and splice it:

```bash
for route in world/funding world/prices world/liquidation world/venues \
             world/counterparties edge/two-trades edge/costs edge/gate edge/forks; do
  node tools/i18n.mjs splice "$route"
done
```

`splice` throws if a fragment's citation chips do not match the English original, which is the failure this catches at build time rather than in review. If one throws, re-run `translate.sh` for that route only.

- [ ] **Step 4: Run the whole suite**

Run: `cd docs/guide && bun test`
Expected: PASS.

- [ ] **Step 5: Check the citation inventory is unchanged**

The guide's governing rule makes citation drift mechanically detectable. Confirm localization did not disturb it — every chip should now appear exactly twice, once per language:

```bash
cd docs/guide && grep -o 'data-src="[^"]*"' index.html | sort | uniq -c | sort -rn | head -20
```

Any chip with an odd count means a language is missing it. Investigate rather than adjusting the test.

- [ ] **Step 6: Commit**

```bash
git add docs/guide/index.html docs/guide/i18n docs/guide/tests/parity.test.js
git commit -m "guide: Georgian for the remaining nine sections

Parts 1-2 are now complete in both languages. Parity tests demand a Georgian
article for every prose route, identical citation chips per route, and no
section left in English."
```

---

### Task 9: Browser verification and publish

**Files:**
- Modify: `docs/guide/README.md`, `docs/HANDOFF.md`

- [ ] **Step 1: Verify against a real browser**

Serve `index.html` through the wrapper in `docs/guide/README.md` and check, in both light and dark themes:

- **Both languages at both viewport widths** (wide, and narrow enough to collapse the rail): no horizontal page scroll, tables scroll inside their own containers.
- **Georgian renders serif.** In the console: `getComputedStyle(document.querySelector('article[lang="ka"] p')).fontFamily` should start with `"Noto Serif Georgian"`.
- **The switcher is keyboard reachable** — Tab to it, Enter activates, `aria-pressed` follows.
- **Deep links.** `#/ka/edge/gate` opens Georgian directly; `#/edge/gate` opens English; `#/edge/gate` after using Georgian still opens English.
- **Search in Georgian** returns Georgian results only, and clicking one lands on the Georgian section.
- **No console errors** in either language.

- [ ] **Step 2: Confirm the published size**

```bash
wc -c docs/guide/index.html
```

Expected: roughly 200-215 KB, per the spec's estimate. A number far above that means the font was pasted unsubset — re-run `tools/build-font.sh`.

- [ ] **Step 3: Update the guide README**

Add to `docs/guide/README.md`, after the "Design" section:

```markdown
## Languages

The guide is bilingual. Prose is duplicated as sibling `<article lang="en">` /
`<article lang="ka">` blocks; controls (calculator, search, theme toggle) exist once
and read their strings from `GuidePure.UI` — the calculator alone holds 16 element
ids and cannot be duplicated. Language rides in the hash: `#/ka/edge/costs`. English
omits its segment, so links written before localization still resolve.

Georgian renders serif via an inlined subset of Noto Serif Georgian (OFL 1.1),
rebuilt with `tools/build-font.sh`.

To re-translate a section after an English edit:

    ./tools/translate.sh edge/costs      # writes i18n/ka/edge__costs.html
    # read the fragment, then:
    node tools/i18n.mjs splice edge/costs
    bun test

The glossary in `i18n/glossary.md` is passed in every prompt and is frozen —
editing it mid-run is how terminology drifts between sections. Citation chips are
never translated; `tests/parity.test.js` enforces that, along with identical
citations across languages and no section left in English.
```

- [ ] **Step 4: Publish**

Publish `docs/guide/index.html` as an Artifact to the **same URL** (`710d7ac1-98b2-45e7-9464-309829ebaa4d`), keeping the 📉 favicon and the existing title.

- [ ] **Step 5: Update the handoff**

In `docs/HANDOFF.md`, replace the "NEXT REQUEST … localize the guide" section with a completed record: the three decisions taken, where the machinery lives, how to re-translate after the voice review, and the note that Parts 3–6 now need translating as they are written. Keep the voice-review gate open — it is still the blocker on Parts 3–6.

- [ ] **Step 6: Commit**

```bash
git add docs/guide/README.md docs/HANDOFF.md
git commit -m "guide: publish the bilingual guide and record the localization

Parts 1-2 ship in English and Georgian behind a hash-routed switcher. The
handoff now carries the re-translation procedure, since the English prose is
still pending its voice review and Parts 3-6 translate as they are written."
```

---

## Notes for the implementer

**The one thing not to do:** if a test asserting a pinned economics figure fails, do not adjust the test. Those five numbers are printed in `tutorial §3.9` and `core §2.4`. A failure means the guide and the spec disagree, which is a finding to report, not a number to update.

**Why fragments live on disk.** The English prose is paused at a voice-review gate. When that review lands and the prose changes, re-translating is `./tools/translate.sh <route>` plus a splice — minutes, not a rewrite. That property is the reason for the whole `i18n/` directory, and it is why hand-patching a fragment is a mistake: the fix is lost on the next run. Fix the prompt instead.

**Why the calculator is different from everything else.** It holds 16 element ids. Duplicating that article per language would produce duplicate ids, and every `getElementById` call in `#guide-ui` would silently bind to the English copy — no error, just a dead Georgian calculator. That single fact is what splits the design into "prose duplicates, controls localize in place."
