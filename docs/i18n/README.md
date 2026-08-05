# Markdown localization — `tutorial.md` → `tutorial.ka.md`

Georgian translation of the teaching companion. Design:
[`../superpowers/specs/2026-07-25-tutorial-georgian-translation-design.md`](../superpowers/specs/2026-07-25-tutorial-georgian-translation-design.md).

**`docs/tutorial.ka.md` is generated. Never hand-edit it.** Edit the fragment, re-run
`node tools/assemble.mjs`. A test asserts the file equals a fresh assemble, so a
hand-edit fails the suite rather than diverging silently.

## Layout

```
ka/tutorial/<id>.md    55 Georgian fragments
tools/split.mjs        parses tutorial.md into fragments (English side)
tools/translate.sh     one fragment ← agy
tools/assemble.mjs     fragments → tutorial.ka.md, refusing on any invariant violation
tools/invariants.mjs   the six raw-text checks, shared by assemble and the tests
tests/parity.test.js   bun
```

The glossary is **the guide's**, read in place at `../guide/i18n/glossary.md` and never
copied. It is what makes the two Georgian documents agree with each other. A test fails
if a second `glossary.md` ever appears under `docs/`.

## Working loop

```sh
cd docs/i18n
./tools/translate.sh list         # every fragment id
./tools/translate.sh s-3-9        # re-translate one section
node tools/assemble.mjs           # regenerate, or refuse and say why
bun test
```

From the repo root, `make tutorial-ka` runs the regeneration and `make
tutorial-ka-check` fails if the generated file is stale against the fragments.
The check is part of `make ci` (node-only — CI has no bun), so a doc-set bump
that forgets the Georgian file fails the build rather than stranding it.

Bulk run (roughly 15 minutes, 6-way parallel):

```sh
./tools/translate.sh list | cut -f1 | xargs -P 6 -I{} ./tools/translate.sh {}
```

## Fragment ids

One fragment per `###` subsection; a `##` section with no `###` children is its own
fragment; the preamble is `header`.

| Heading | id |
|---|---|
| the preamble | `header` |
| `### 3.9 A complete worked trade …` | `s-3-9` |
| `## Part 8 — Common pitfalls (field notes)` | `part-8` |
| `## Scope & Assumptions` | `scope-assumptions` |

Ids are **spec section numbers, not ordinals**, so inserting or deleting a section never
renames its neighbours. That is what makes per-section regeneration work: when R2d edits
`tutorial.md`, the Georgian costs one `translate.sh` call per changed section.

## The six invariants

`assemble.mjs` writes nothing unless every fragment matches its English original on all
six. `tests/parity.test.js` then re-runs them file-wide against the generated file.

| # | Invariant | What it catches |
|---|---|---|
| 1 | heading number + trailing `` `[→ …]` `` tag | Renumbering. `build-spec.md`'s `[mechanism: …]` pointers, ADR §3 traceability and the guide's `tutorial §3.9` citation chips all address sections by number. |
| 2 | numeric multiset | A mangled ledger figure. §3.9 is the *source* of the five numbers the guide's calculator is pinned to — and the guide asserts them against hard-coded values, not against this file. Without this check the two documents could disagree with nothing failing. |
| 3 | inline-code multiset | A paraphrase inside backticks. `AbsentAfterFinality` is a state name, not prose. |
| 4 | §-pointer multiset | A localized `§spec` label, or a pointer dropped mid-sentence. |
| 5 | table shape | Broken Markdown tables. |
| 6 | link targets | A translated path. |

**Do not relax a failing check.** A §-number, a figure or an identifier that drifts here
is a finding, not a test to loosen — the same rule the guide's calculator assertions
carry.

Invariant 1 also refuses a heading identical to its English original, which fired once
in the first bulk run: `### 1.6 Mark vs index vs last` is made entirely of glossary-pinned
Latin terms, so the model correctly changed nothing and the check correctly complained.
The fix was the heading (`Mark, index და last — განსხვავება`), not the check. Re-running
`translate.sh s-1-6` will reintroduce it — which is the intended behaviour: the gate
fires loudly rather than letting a genuinely skipped heading through.

### The one rule invariant 2 depends on

Numbers must not cross the digit/word boundary in either direction: English "two legs"
must not become "2 leg", and `20 days` must not be spelled out in Georgian words. The
prompt states this explicitly, so the numeric check only ever fires on a real defect.

## What stays English

- **Section numbers** — `### 3.9 ` is byte-identical; only the title after it is Georgian.
- **`**Status:**` and `**Doc set:**`** — maintainer provenance citing every FM/Cx tag in
  the project. Reproduced verbatim, behind a Georgian banner explaining the file.
- **Everything in backticks**, every `§` pointer, every link target, every figure.
- **Borrowed trading vocabulary** — `leg`, `spot`, `perp`, `funding rate`, `mark price`
  and their kin, per the glossary.

## Rules carried over from the guide's localization

1. **The glossary is frozen.** Never edit it mid-run. The guide's first bulk run omitted
   `leg` and produced three different Georgian words across three sections, each fluent
   in isolation. The fix is to change the glossary and re-run **every** fragment — never
   to patch sections individually.
2. **A guarantee that lives in raw file text needs a raw-text test.** Section numbers,
   figures and identifiers are exactly that: they render fine while being wrong.
