# Design — Georgian translation of `docs/tutorial.md`

**Date:** 2026-07-25
**Subject:** `docs/tutorial.md` → `docs/tutorial.ka.md`, plus the Markdown localization toolchain
**Predecessor:** `2026-07-25-guide-localization-design.md` (the guide's EN/KA localization), whose
frozen glossary this design reuses **unchanged**

## Purpose

`tutorial.md` is the teaching companion in the five-doc spec set — the document that
explains *why* the design looks the way it does, at an altitude a reader can absorb
before milestone 1 starts. Georgian is the reader's first language. The interactive
guide is already bilingual; the tutorial is the other document the reader actually
reads end to end.

**Why this is safe to run now, unlike the guide's prose.** The guide's English is
paused at a voice-review gate, so translating it risks translating twice.
`tutorial.md` has been through four review rounds and is stable — nothing here waits
on that gate.

**And it stays regenerable anyway.** A second external round-4 response (R2d) is still
pending and will edit `tutorial.md`. Translated fragments therefore live on disk as
reviewable, individually re-runnable artifacts: when R2d changes §3.9, the Georgian
costs one command against §3.9, not a re-translation project.

## Decisions taken (2026-07-25, by the user)

1. **The Georgian lives at `docs/tutorial.ka.md`** — a sibling file, not an interleaved
   block. The guide's sibling-`<article>` trick exists only because an Artifact
   publishes exactly one file; Markdown has no such constraint.
2. **Section *numbers* are frozen, section *titles* are translated.** `### 3.9 ` stays
   byte-identical, including any trailing `` `[→ …]` `` tag; the title prose after it
   becomes Georgian. Every `§3.9`-style pointer resolves on the number, not the title,
   and the guide's Georgian nav already translates its own titles.
3. **The Status block stays English, behind a Georgian banner.** Line 3 (`**Status:**`)
   and line 4 (`**Doc set:**`) are maintainer provenance citing every FM/Cx tag in the
   project and four doc filenames — the densest drift surface in the file and the least
   teaching value. They are reproduced verbatim. Above them, a short Georgian banner
   states that this is a translation, that it is generated from
   `docs/i18n/ka/tutorial/` and must not be hand-edited, and that `tutorial.md` wins on
   any disagreement.

## The organizing rule: `tutorial.ka.md` is a build output

The guide **splices** Georgian into `index.html` because both languages must ship in one
file. Here the Georgian is its own file, so it is instead **assembled**: a
byte-reproducible concatenation of per-section fragments, in the order the *English*
file declares.

This is strictly stronger than splicing. There is no insert-vs-replace branch, no regex
over translated prose, and therefore none of the `$&`-in-a-replacement-string hazard the
guide's `i18n.mjs` had to defend against with function replacements. `assemble` is total
and idempotent, and a test asserts the file on disk equals a fresh assemble — so a
hand-edit of `tutorial.ka.md` is a test failure, not a silent divergence.

**`tutorial.ka.md` is generated. Never hand-edit it. Edit the fragment and re-assemble.**

## Fragment granularity: one per `###`

A fragment is **one `###` subsection**, or the `##` section itself when it has no `###`
children. That yields **50 fragments** (44 `###`, 5 childless `##`, 1 header block),
evenly sized at roughly 800 bytes.

Two alternatives were rejected:

- **One fragment per `##`** (the handoff's estimate of ~12 calls at ~3.5 KB). Measured,
  the sections are not uniform: Part 3 is **11.2 KB** — 3× the size the guide's sections
  translated cleanly at — and it is the section that contains the §3.9 worked ledger,
  the highest-consequence content in the file. Part 6 is 6.8 KB; four sections are under
  700 B.
- **Greedy size-packing to ~4 KB.** Produces even chunks, but the boundaries are
  derived from content size, so an English edit shifts every downstream boundary and
  R2d's "one changed section" becomes "re-translate the tail". Fragment identity must
  survive English edits or the regenerable property is a fiction.

Fragment IDs are the **spec section number**, which is the granularity at which changes
actually arrive:

| Heading | Fragment ID |
|---|---|
| the preamble before the first `##` | `header` |
| `### 3.9 A complete worked trade …` | `s-3-9` |
| `## Part 8 — Common pitfalls (field notes)` | `part-8` |
| `## Scope & Assumptions` | `scope-assumptions` |
| `## Closing thoughts` | `closing-thoughts` |

Fragments are stored at `docs/i18n/ka/tutorial/<id>.md`. Directory order is irrelevant:
`assemble` derives document order from `tutorial.md` on every run, because the English
file is the source of truth for structure.

## The glossary is shared, and stays exactly where it is

`docs/guide/i18n/glossary.md` is read by both toolchains and **never copied**. It is
what makes the guide and the tutorial agree with each other in Georgian — the same
`leg`, the same `მარჟა`, the same `ლიკვიდაცია`.

A test fails if a second glossary file appears anywhere under `docs/`. The failure the
guide recorded — `leg` omitted from the glossary, then rendered as three different
Georgian words in three sections, each fluent in isolation — is exactly what two
drifting copies would reintroduce, silently. The rule from that round carries over
verbatim: **never edit the glossary mid-run**; if a rule must change, change it and
re-run *every* fragment.

## The refuse-to-assemble gate

`assemble.mjs` writes nothing unless **every** fragment matches its English original on
six raw-text invariants. This is the Markdown counterpart of the guide's
citation-chip check, and it exists for the same reason: these guarantees live in the
file's raw text, not in its rendered output, so they need raw-text tests.

| # | Invariant | What it catches |
|---|---|---|
| 1 | **Heading prefix** — `### 3.9 ` byte-identical, plus any trailing `` `[→ …]` `` tag | Renumbering or reordering. `build-spec.md`'s `[mechanism: …]` pointers, ADR §3 traceability and the guide's `tutorial §3.9` citation chips all address sections by number. |
| 2 | **Numeric multiset** — 452 tokens file-wide | A mangled ledger figure. §3.9 is the *source* of the five numbers the guide's calculator is pinned to, and the guide asserts them against hard-coded values, not against the tutorial — so without this check the two documents could disagree with nothing failing. |
| 3 | **Inline-code multiset** — 140 spans, 98 distinct | A paraphrase inside backticks. `AbsentAfterFinality`, `C_allocated`, `cid-perp` and `DispatchAuthorized` are identifiers, not prose. |
| 4 | **§-pointer multiset** — `§spec §2.2`, `SM §2`, `Profiles §1`, bare `§3.9` | A localized `§spec` label, or a pointer dropped mid-sentence. Three syntactic shapes, three different breakages. |
| 5 | **Table shape** — row count, per-row column count, `\|---\|` alignment rows intact | Broken Markdown tables. 3 tables, 28 rows, incl. the §3.9 ledger and the failure-timeline table. |
| 6 | **Link targets** — `](./build-spec.md)`, `](./legacy/05-tutorial-building-a-bot.md)` | A translated path. |

**There is no escape hatch, and the tests are not to be relaxed.** A fragment that
cannot satisfy these is a translation to fix. The guide's rule applies unchanged: if an
assertion over a spec figure fails, the documents disagree — that is a finding.

### One prompt rule that invariant 2 depends on

Numbers must not cross the digit/word boundary in either direction. English "two legs"
must not become Georgian "2 leg", and `20 days` must not become a spelled-out Georgian
numeral. The translation prompt states this explicitly; otherwise a perfectly fluent
translation fails the numeric multiset for a reason that is not a defect.

## Toolchain

Modelled on `docs/guide/tools/`, adapted from HTML to Markdown. Nothing under
`docs/guide/` changes except that its glossary gains a second reader.

```
docs/i18n/
  README.md                    operating instructions (mirrors docs/guide/README.md § Languages)
  ka/tutorial/<id>.md          50 fragments; `header.md` is hand-written
  tools/split.mjs              parses tutorial.md → ordered fragment list; prints one for prompting
  tools/translate.sh           ./tools/translate.sh s-3-9   → writes ka/tutorial/s-3-9.md via agy
  tools/assemble.mjs           fragments → tutorial.ka.md, refusing on any invariant
  tests/parity.test.js         bun
```

`agy` v1.1.7 with `Gemini 3.1 Pro (High)` — the setup proven on the guide. The prompt
reuses the guide's three hard rules, translated from HTML to Markdown terms, plus the
digit/word rule above, plus the glossary verbatim. The parent `## Part N` heading is
passed as *context, not for output*, so a small fragment still knows what it is about.

**Working loop, one fragment:**

```
./tools/translate.sh s-3-9        # writes docs/i18n/ka/tutorial/s-3-9.md, reviewable
node tools/assemble.mjs           # regenerates tutorial.ka.md, or refuses and says why
cd docs/i18n && bun test
```

The bulk run is the same loop over all 50, run in the background.

## Tests

`cd docs/i18n && bun test`. Invariants 1–6 run **file-wide** against `tutorial.ka.md`,
not only per-fragment at assemble time, so a hand-edit is caught even if assemble is
never re-run. Plus the guide's proven set, ported:

- `tutorial.ka.md` is byte-identical to a fresh assemble.
- Every fragment contains Mkhedruli.
- No fragment is a byte-copy of its English original.
- Pinned Latin terms (`leg`, `spot`, `perp`, `funding`, `mark price`, `basis`, `maker`,
  `taker`, `slippage`, `notional`, `hurdle`, `utilization`, `settlement`, `ADL`) survive
  into every Georgian fragment whose English original uses them — the `leg` regression
  test, carried over.
- Every pinned term is actually named in the glossary (the guide's guard against a test
  checking a term the glossary never froze).
- Exactly one file named `glossary.md` exists under `docs/`, at
  `docs/guide/i18n/glossary.md`.
- The frozen lines — `**Status:**` and `**Doc set:**` — appear verbatim in
  `tutorial.ka.md`.

The guide's own suite (`cd docs/guide && bun test`, 86 tests) must stay green; the only
shared object is the glossary, which is not being edited.

## Out of scope

- **No change to `docs/guide/`** beyond a second reader for its glossary. The guide's
  English is at a voice gate; its Georgian is not re-run here.
- **No change to `tutorial.md`.** This is a translation, not a revision. The open
  `core §12` / `core §2.4` finding recorded in HANDOFF (the −40% / −50% paraphrase
  collapse) is *not* triaged here — the Georgian faithfully reproduces whatever the
  English says.
- **No Artifact publication.** `tutorial.ka.md` is a repository document.

## Risks

- **A fluent translation that fails invariant 2 for a non-defect reason** — the
  digit/word boundary. Mitigated by an explicit prompt rule; if it still happens, the
  fix is the fragment, never the test.
- **Fragment-level context loss.** ~800 B fragments see less surrounding prose than the
  guide's ~3.5 KB sections did. Mitigated by passing the parent `## Part N` heading as
  context and by the frozen glossary, which is what actually carries terminology
  consistency across fragments.
- **`tutorial.ka.md` hand-edited by a future session** that does not know it is
  generated. Mitigated by the byte-identity test, by the Georgian banner saying so in
  the first three lines of the file, and by `docs/i18n/README.md`.
