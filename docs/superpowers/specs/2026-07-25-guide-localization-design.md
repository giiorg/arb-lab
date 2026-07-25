# Design — English/Georgian localization of the interactive guide

**Date:** 2026-07-25
**Subject:** `docs/guide/index.html` — bilingual EN/KA with a language switcher
**Predecessor:** `2026-07-25-interactive-guide-design.md` (the guide itself), stage-1 plan `../plans/2026-07-25-interactive-guide-stage-1.md`

## Purpose

The guide exists to make the 415 KB five-doc spec set comprehensible before milestone 1
starts. Georgian is the reader's first language; a guide that teaches in it teaches
better. This design covers the localization *machinery* plus the stage-1 prose
(Parts 1–2, 11 routes). Parts 3–6 do not exist yet and will translate on arrival.

Stage 1 is paused at a voice-review gate, so the translated prose is explicitly
treated as **regenerable**: re-translating a section after a voice change is one
command, not human work. The machinery is built on that assumption — translated
fragments live on disk as reviewable, re-runnable artifacts.

## Decisions taken (2026-07-25, by the user)

1. **Georgian renders serif.** Noto Serif Georgian is subset and inlined as a data URI.
   The alternative — letting Georgian fall back to `.SF Georgian` (sans) while English
   renders serif — would split the visual identity by accident rather than by choice.
2. **Borrowed terms stay in Latin script.** `perp`, `funding rate`, `spot`, `mark price`
   and their kin appear as English inside Georgian prose, which is how Georgian traders
   write and what exchange UIs show. Full glossary below.
3. **English is the default language, and language rides in the hash.** `#/ka/edge/costs`
   opens that section in Georgian; `#/edge/costs` keeps working and means English, so
   every existing link and the published Artifact URL are unaffected.

## The organizing rule: prose duplicates, controls localize in place

The calculator article holds 16 element IDs (`in-rate` … `calc-note`). Duplicating it
per language would produce duplicate IDs — invalid HTML, and every `getElementById`
call in `#guide-ui` would silently bind to the English copy. So:

- **Prose** — static, marked-up, no IDs — is duplicated as sibling `<article>` blocks.
- **Controls** — anything with an ID or an event listener (calculator, search box, theme
  toggle) — stays a single instance and reads its strings from a table.

Prose is kept as prose because it is *marked up*: tables, `<strong>`, `<em>`, headings
and citation chips. Moving it into a JS string table would mean HTML-in-strings assembled
via `innerHTML`, which is worse code and makes the citation chips unreachable to the
`grep -o 'data-src="[^"]*"'` drift check the guide's governing rule depends on.

## Architecture

### 1. Three string tiers

| Surface | Today | After |
|---|---|---|
| Article prose (~700 lines, 11 routes) | static HTML | duplicated `lang="en"` / `lang="ka"` sibling `<article>` blocks |
| Part titles, route titles, status line, search placeholder, theme-toggle label, aria-labels | `PARTS`/`ROUTES` + inline attributes | `{en, ka}` objects on each entry, in `#guide-pure` |
| Calculator field labels, 11 output row names, the two verdict sentences, 4 preset labels + notes | **English literals in `#guide-ui`** (`index.html:1207-1235`, `PRESETS` at `index.html:1016-1041`) | a new `UI` string table in `#guide-pure` |

The third row carries a bonus: those strings currently sit in the UI block, which
`bun test` cannot reach — the tests evaluate `#guide-pure` only. Moving them puts the
verdict sentences and output row names under test for the first time.

**`computeEconomics` is not touched.** One code path serves both languages, so the five
pinned spec figures cannot drift by language. Numbers, currency and percentage formatting
stay identical across languages (`$10,000`, `3.41%`) — Georgian financial writing uses
the same numerals and the `$` symbol, and diverging here would break the tie to the
figures printed in `tutorial §3.9` / `core §2.4`.

### 2. Markup shape

```html
<article data-route="world/perps" lang="en">…</article>
<article data-route="world/perps" lang="ka">…</article>

<!-- controls: single instance, no lang attribute -->
<article data-route="edge/calculator">
  <h1 data-i18n="calc.title"></h1>
  <p data-i18n-html="calc.intro"></p>
  …
  <label><span data-i18n="calc.field.rate"></span>
    <input id="in-rate" type="number" …></label>
</article>
```

`data-i18n` sets `textContent`; `data-i18n-html` is used only where a string carries
citation chips, and its values are authored in the string table, never user input.

The calculator's own explanatory paragraph carries two citation chips, so it uses
`data-i18n-html`. Everything else in the calculator is plain text.

### 3. Language selection

The active language is an attribute on the guide container, not on `<html>` — the
Artifact host supplies `<html>` and the guide must not depend on owning it:

```html
<div class="guide" data-lang="en">
```

`show(route, lang)` activates exactly the one article matching both route and language;
the calculator article, which has no `lang` attribute, matches any language and is
re-rendered through the string table on switch. Setting `lang="ka"` on the Georgian
containers drives correct font fallback via `:lang(ka)` **and** screen-reader
pronunciation — one attribute, both jobs.

The switcher is a two-state control in the rail beside the theme toggle, labelled
`EN / ქა`, with `aria-pressed` reflecting the active language. Switching rewrites the
hash, preserving the current route, so you keep your place.

### 4. Routing

Two pure functions in `#guide-pure`, both directly testable with no DOM:

```
parseHash("#/ka/edge/costs")  → { lang: "ka", route: "edge/costs" }
parseHash("#/edge/costs")     → { lang: "en", route: "edge/costs" }   // legacy form
parseHash("")                 → { lang: "en", route: "world/perps" }  // default
formatHash({lang, route})     → "#/edge/costs" | "#/ka/edge/costs"
```

A leading segment is treated as a language only if it is exactly `en` or `ka`; anything
else is part of the route. English omits its segment so existing links stay valid.

### 5. Search

Records gain a `lang` field, gathered from each article's `lang` attribute.
`searchIndex(index, query, lang)` filters to the active language, and result links are
built with `formatHash` so they open in the language you searched from. The calculator
article, having no `lang`, is indexed under both languages using its active strings.

Because glossary terms stay in Latin script, searching `funding` finds the Georgian
section too — a direct benefit of decision 2.

### 6. Typography

`docs/guide/tools/build-font.sh` — committed, reproducible:

1. Fetch `NotoSerifGeorgian[wdth,wght].ttf` from `google/fonts` (OFL).
2. `fonttools varLib.instancer` to `wdth=100`, `wght=400` and `wght=600`.
3. `pyftsubset --flavor=woff2` over Mkhedruli (`U+10A0-10FF`), Mtavruli (`U+1C90-1CBF`),
   basic Latin and the punctuation the guide uses; `--no-hinting --desubroutinize`.
4. Emit base64 for pasting into the `@font-face` blocks.

Measured: 21.1 KB and 22.3 KB raw → **~58 KB base64 inlined**. `unicode-range` scoped to
the Georgian blocks means English pages never fetch or use it.

```css
:lang(ka) { font-family: "Noto Serif Georgian", var(--serif); }
```

**File size, stated honestly.** Georgian in UTF-8 costs 3 bytes per character, so
translated prose runs roughly 3× the bytes of the English it mirrors. From 58 KB today:
font ~58 KB + Georgian prose ~100 KB → **~200-215 KB published**. Acceptable for a
self-contained Artifact, but a number to watch rather than discover.

### 7. Translation pipeline

`docs/guide/tools/translate.sh <route>`:

- extracts the `lang="en"` article for `<route>` from `index.html`
- calls `agy --model "Gemini 3.1 Pro (High)" -p …` with a fixed preamble carrying the
  glossary and three hard rules:
  1. preserve every HTML tag and attribute **exactly**, including `class` and `data-src`
  2. never translate the text inside `<span class="cite">` and never alter `data-src`
  3. output only the fragment — no preamble, no code fence
- writes `docs/guide/i18n/ka/<route>.html`

`docs/guide/tools/splice.mjs` then writes each fragment into `index.html` as the
`lang="ka"` sibling of its English article — inserting it on the first run, replacing it
in place on every later run, so the script is idempotent and re-runnable per route. Fragments on disk, rather than translation in place, are what
make "the prose is regenerable" true in practice: after the voice review, re-translating
one section is one command and the diff is reviewable in isolation.

Both scripts are committed. `index.html` remains the single published file — there is
still no build step in the publish path.

## Glossary

Passed verbatim in every translation prompt. **Keep in Latin** — spec and venue
vocabulary, what exchange UIs show and what the normative docs call these things:

`perpetual` · `perp` · `spot` · `futures` · `funding rate` · `funding interval` ·
`settlement` · `mark price` · `index price` · `last price` · `basis` · `maker` ·
`taker` · `slippage` · `notional` · `APY` · `hurdle` · `round trip` · `utilization` ·
`ADL` · `auto-deleveraging` · `fence` · `actuator` · `order book` · `VIP0` ·
`Binance` · `Bybit` · `BTC` · `USDT` · `USDC` · `bracket` · `IOC`

**Translate into Georgian** — general finance and economics with settled Georgian usage:

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

The last three are the borderline calls — Georgian has established forms
(`მარჟა`, `ლიკვიდაცია` are transliterations already naturalized; `ბერკეტი` is a true
translation), and they are marked here specifically so the native speaker can overrule
them at spec review. Terminology drift between sections is the failure mode this table
exists to prevent, so it is fixed **before** bulk translation and never edited mid-run.

Citation chip contents (`core §7.4`, `tutorial §3.9`, `Profiles §1`) are neither
translated nor transliterated in either language — they are pointers into English
normative documents, and a test enforces it.

## Testing

The 40 existing tests assert English strings and route lists; they become
language-aware. The tests that carry the real weight are parity tests:

| Test | Catches |
|---|---|
| every route has exactly one `lang="en"` and one `lang="ka"` article | a missed or doubled section |
| per route, the multiset of `data-src` values is identical between languages | a citation dropped, added, or drifted in translation |
| every `ka` article contains Mkhedruli (`/[Ⴀ-ჿᲐ-Ჿ]/`) | a section left in English |
| no `ka` article is byte-identical to its `en` sibling | a silently skipped translation |
| glossary Latin terms still appear in Georgian prose | the model "helpfully" translating pinned terms |
| citation chip text is never Georgian | rule 2 of the translation prompt failing |
| `parseHash`/`formatHash` round-trip, incl. the legacy no-language form | a broken existing link |
| search index has both languages; `searchIndex` filters by language | cross-language result leakage |
| the five pinned economics figures | spec/guide disagreement — a finding, never a test to relax |
| no external network references (existing) | the CSP constraint, now including the font |

The font is inlined, so the existing self-containment test must be extended to accept
`data:font/woff2` while still rejecting every off-host URL.

## Scope

**In:** the machinery in full, plus Georgian for Parts 1–2 (11 routes), the calculator,
and all chrome.

**Out:** Parts 3–6 (unwritten — they translate on arrival); any language beyond EN/KA;
persisting the language choice to storage (the hash carries it, and storage was never in
the guide's scope); translating the normative spec documents.

## Risks

- **Voice review may change the English prose.** Mitigated by design: fragments on disk,
  one command per section to regenerate.
- **The model mangles markup.** Mitigated by the parity tests, which fail loudly on a
  dropped citation or a missing section rather than shipping quietly.
- **Terminology drift across sections.** Mitigated by the fixed glossary, passed in every
  prompt and frozen for the run.
- **`agy` latency.** Calls are slow; sections are translated one per call and the results
  are cached on disk, so a re-run costs only what changed.
