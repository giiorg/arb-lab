# Interactive guide

`index.html` is the comprehensive interactive guide to the funding-rate arbitrage
platform. Design: `docs/superpowers/specs/2026-07-25-interactive-guide-design.md`.
Plan: `docs/superpowers/plans/2026-07-25-interactive-guide-stage-1.md`.

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

## Previewing it locally

Open `index.html` directly and the browser renders it in **quirks mode**, because the
file has no doctype (the host supplies one). Quirks mode changes real behaviour —
tables stop inheriting `color` from their ancestors, for one — so a raw preview is
not representative. Preview through a wrapper that mimics the host instead:

```js
// preview.js — bun preview.js, then open http://localhost:8731
Bun.serve({ port: 8731, async fetch() {
  const body = await Bun.file("index.html").text();
  return new Response(
    `<!doctype html><html><head><meta charset="utf-8">`
    + `<meta name="viewport" content="width=device-width,initial-scale=1">`
    + `<style>*{margin:0;padding:0}</style></head><body>${body}</body></html>`,
    { headers: { "content-type": "text/html; charset=utf-8", "cache-control": "no-store" } });
}});
```

Note that changing only the URL hash does not refetch — add a query string to
force a reload after editing.

## Design

Cool instrument neutrals rather than paper; the accent is steel blue and deliberately
avoids green and red, which are reserved as semantic pass/fail on the calculator
verdict. Three type roles: sans for headings, serif for body prose, mono for data,
code and chrome. All system faces except one — Noto Serif Georgian is inlined as a
data URI, because no system serif covers Mkhedruli (see Languages, below). Nothing
is ever fetched; the CSP forbids it.

## Languages

The guide is bilingual, English and Georgian. Prose is duplicated as sibling
`<article lang="en">` / `<article lang="ka">` blocks; controls — the calculator,
search box and theme toggle — exist once and read their strings from `GuidePure.UI`.
The calculator alone holds 16 element ids, so duplicating it per language would
duplicate those ids and every `getElementById` call would silently bind to the
English copy. Hence the rule: **prose duplicates, controls localize in place.**

Language rides in the hash: `#/ka/edge/costs`. English omits its segment, so every
link written before localization still resolves. `parseHash`/`formatHash` in
`#guide-pure` own that, and `pick(bundle, lang)` falls back to English for any key
not yet translated, so a partly-translated guide stays usable.

Georgian renders serif via an inlined subset of Noto Serif Georgian (OFL 1.1) —
none of the body serif stack covers Mkhedruli and the artifact CSP blocks font CDNs,
so without it Georgian would silently fall back to system sans. Rebuild with
`tools/build-font.sh` and paste the base64 into the two `@font-face` rules.

To re-translate a section after an English edit:

    ./tools/translate.sh edge/costs        # writes i18n/ka/edge__costs.html
    # read the fragment, then:
    node tools/i18n.mjs splice edge/costs  # idempotent; refuses mismatched citations
    bun test

The interface strings are a separate batch — `tools/translate-strings.sh` translates
them in one call so the labels stay consistent with each other.

The glossary in `i18n/glossary.md` is passed in every prompt and is **frozen**;
editing it mid-run is how terminology drifts between sections. If a translation
breaks a rule, fix the prompt in `translate.sh` and re-run — never hand-patch a
fragment, or the fix is lost on the next translation.

Citation chips are never translated. `tests/parity.test.js` enforces that, along
with identical citations across languages, no section left in English, and no
Georgian text inside a chip.

## Published

<https://claude.ai/code/artifact/710d7ac1-98b2-45e7-9464-309829ebaa4d>

Stages 2–4 redeploy to that same URL by publishing this same file path — keep the
favicon (📉) stable so the browser tab stays recognisable. There is no build step:
what is committed is what is published.

**Stage 1 covers Parts 1–2 only, in both languages.** Parts 3–6 (why the design looks
like this, what we're building, the Go build, the operator runbook) are pending the
voice review, and translate as they are written.
