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

## Publishing

The file is published as an Artifact and redeploys to the same URL. There is no
build step — what is committed is what is published.
