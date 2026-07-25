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
verdict. Three type roles, all system faces so nothing is fetched: sans for headings,
serif for body prose, mono for data, code and chrome.

## Published

<https://claude.ai/code/artifact/710d7ac1-98b2-45e7-9464-309829ebaa4d>

Stages 2–4 redeploy to that same URL by publishing this same file path — keep the
favicon (📉) stable so the browser tab stays recognisable. There is no build step:
what is committed is what is published.

**Stage 1 covers Parts 1–2 only.** Parts 3–6 (why the design looks like this, what
we're building, the Go build, the operator runbook) are pending the voice review.
