# Record §88 — the formal-model freshness gate: model staleness moves from memory into the build, and tracking PREMISES found two things nobody was looking for

**Date:** 2026-08-09 · **Task:** tooling (opens #231, #232) · Branch
`feat/milestone-1-foundation`.

**The one-line finding:** the two TLA+ models are companion artifacts to a
normative spec, so a spec edit can void a model's coverage while every run
recorded in its companion still reads green — the runs were real, they were just
run against a **different document** — and until this session nothing but human
memory stood between those two facts.

**What makes this a gate rather than a nicety:** the failure is silent in the
safe-looking direction. A stale model does not error, does not warn, and does
not lose its results. It keeps presenting 75 passing runs. The reader's only
signal is a version number in prose, in a companion, that nobody diffs.

---

## 1. The question this answers, and the one it cannot

The question is **"has the text this model was checked against moved?"** It is
answered by extracting each tracked section and comparing a SHA-256 against a
stamp taken at the model's last full verification.

The question it does **not** answer is "is the model still correct?" Nothing
short of re-running `./run.sh` answers that, and the gate says so in its failure
message rather than implying otherwise.

**TLC cannot answer either question.** This is worth stating because "just run
the model checker in CI" is the obvious objection to the whole design. TLC reads
`.tla`, not `.md`. A model that has drifted from the spec is still internally
consistent, so a TLC run over a stale model is **green** — that is precisely the
failure being defended against, and the model checker is structurally blind to
it. The gate is not a cheap substitute for TLC; it checks a thing TLC cannot
see.

## 2. What the measurement found before a line of code was written

Both models were last verified at **`310e854`** (slice F, 2026-08-02) — the
single unfiltered invocation that reported ALL 75 RUNS MATCH PREDICTIONS across
both. Extracting their subject sections at that commit and at HEAD:

| Model | Subjects | Premises |
|---|---|---|
| `FenceTakeover` | SM §3, §4, §6 — **all byte-identical** across 43 commits to `state-machines.md` and 63 doc revisions | SM §7 **changed**, core §6 **changed** |
| `AmbiguousOrder` | SM §2 (+3 lines), §7 (+23 lines) — **both changed** | core §5.3, §6, §7.1 **changed** |

Model (b) is **not stale on its subject**, which is the result that shaped the
design: a whole-document hash would have reported it stale 43 times over text
its machines never touched, and a gate that cries wolf 43 times teaches its
reader to restamp without looking. Section granularity is not a refinement, it
is what makes the signal usable.

## 3. The design

- **`tools/formalfresh`**, a Go package in the ROOT module whose tests *are* the
  gate. `make test` therefore runs it, and it is named in `make ci` ahead of the
  race suite so a stale model reports in a second rather than after the full run.
- **`docs/formal/freshness.json`** carries the stamps: per model, its modules,
  its companion, the verification run these digests belong to, and the tracked
  sections with a `role` and a `why`.
- **Two roles, and the distinction earned itself immediately (§5).**
  `subject` = the model ENCODES this machine; a change can invalidate the runs.
  `premise` = the correspondence map CITES this section to justify an encoding
  or a stated omission; a change can invalidate a recorded justification without
  touching a single run. **`FM-7` is this repo's evidence for what a
  never-re-derived justification costs**, which is why premises are tracked at
  all.
- **`knownStale` markers**, the `[SPEC EDIT OWED]` pattern: a changed section
  with the task that owns re-verifying it. It is **not** an exemption — it pins
  the digest it is stale AGAINST, so it covers exactly one known delta. A
  further edit to the same section fails again (`MARKER-STALE`), and a marker
  whose section has gone back to matching is itself an error
  (`MARKER-OBSOLETE`), so markers cannot rot into permanent silence.
- **Restamping takes a model NAME, not a flag.** `make formal-restamp
  MODEL=AmbiguousOrder`. Re-running `./run.sh` filtered to one model and then
  restamping both would relabel the other model's staleness as verified with
  nothing having verified it — the expensive-and-correct action and the
  cheap-and-wrong one must not share a spelling.

### The vacuity guards, which are the load-bearing part

A staleness gate's characteristic failure is passing because it found nothing to
compare. Each of these is refused rather than defaulted:

| Guard | Without it |
|---|---|
| heading not found → error | a renamed heading hashes to a stable empty string forever |
| heading found twice → error | the gate silently picks one |
| heading with no body → error | a section whose content moved still "matches" |
| manifest with no models → fatal | the whole gate passes on an empty file |
| no `.tla` found on disk → fatal | the coverage check passes over an empty directory |
| every `.tla` must be claimed | a model (c) added later is unpoliced and nothing says so |

This was not theoretical. The **first** section-diff run during investigation
reported `IDENTICAL` on **0 lines extracted on both sides** — the repo's own
collapse pattern, caught only because line counts were printed beside the
verdict.

## 4. Evidence that it bites

**8 mutants, 8 caught, 0 survivors** (`tools/mutate.sh`, target
`./tools/formalfresh`): prefix-instead-of-whole-id matching, section closing
only at a shallower heading, fenced headings counted as real, seven hashes
accepted as a level, the empty-body guard weakened, the trailing-separator
normalisation removed, and both marker-verdict arms.

**Three end-to-end controls**, each reverted:

1. an unmarked edit to SM §3 → **STALE** reported against *both* models that
   track it, naming the role, the stamp, the actual, and the two ways out;
2. `## §3.` renamed to `## §3b.` → **"the manifest names a section
   docs/state-machines.md no longer resolves"**, not a silent pass;
3. a stray `ProbeModel.tla` dropped into `docs/formal/` → **"no manifest entry
   claims them, so no document change can ever be reported against them."**

**And the negative control is real rather than constructed:** model (b)'s three
subject sections pass, and they pass across 43 commits to the very file being
hashed. A green there is the gate declining to fire, not the gate declining to
look.

**The fence branch has no natural fixture and that is recorded, not hidden.**
Neither spec document contains a heading-like line inside a code fence today, so
the real corpus cannot exercise it; a `# comment` in a future config example
would otherwise truncate a section's hash at the fence. It is covered by a
synthetic fixture in which the *same* bug produces two different wrong answers.

## 5. What tracking PREMISES found — the part that justifies the role split

Both findings came from `premise` rows. Neither would exist if the manifest had
tracked only each model's declared subject, which was the obvious cheaper design.

**#231 — core §6 step 7 still names the RETIRED `TerminalFlatten`.** SM §7
records it as **RETIRED `[v0.9.67, task #224, record §87 §3 R6]`**, while
build-spec §6 step 7's producer list — the normative enumeration of what
re-triggers §2 classification `[→ FM-6]` — still reads *"a `PackageDelever`
generation opening, amending as its perp leg confirms, completing,
**terminal-flattening**, or being superseded"*. Record §87 §4's normative-edit
list names build-spec §7.1, SM §7, venue-profiles, build-spec §5.1, build-spec
§7.5 and record §85; **core §6 step 7 is not in it**, so this is a missed edit
rather than a deliberate retention. It is the *declared enum member nothing
emits* shape at the spec layer: the producer list reads as total while one entry
names a state nothing can enter.

**#232 — model (b)'s core §6 premise is un-re-derived.** `FenceTakeover`'s
liveness fairness set encodes step 7's *"obligated ≤5 s naked-reduce (core
§6.7)"*, and the FM-2 lasso analysis rests on the same obligation. §6 went 18 →
29 lines and step 7 itself gained the FM-5-family projection clause. The model's
**runs stand** — its subjects are byte-identical — so what is owed is a
re-derivation, not a re-run. That is `FM-7` exactly, and it is cheap: two
citations, one section diff.

## 6. Alternatives considered and rejected

- **Run TLC in CI.** Rejected on two grounds, and the second is the real one:
  32 min on 18 cores plus a JVM is a gate people disable — but more importantly
  **TLC cannot see this class of defect at all** (§1). It would have been an
  expensive way to not answer the question.
- **Hash whole documents.** Rejected on measurement: model (b) would have
  reported stale 43 times over text its machines never touched (§2).
- **Track only declared subjects.** Rejected because a correspondence-map
  citation is a load-bearing justification, and `FM-7` is the standing evidence.
  The rejection paid the same session: both findings in §5 are premise rows.
- **Let the gate fail CI on the current #229 staleness.** Rejected: it would
  block unrelated work for several sessions, and the repo's doctrine is
  *marked, not absorbed*. A marker pinned to a specific digest keeps that honest
  in a way a blanket exemption would not.
- **A boolean `-restamp`.** Rejected: it makes the wrong action as cheap to type
  as the right one (§3).
- **A shell script beside `tools/mutate.sh`.** Rejected: *a linter you cannot
  unit-test cannot be shown to bite*, and the extractor carries genuine collapse
  hazards — prefix matching and fence handling — that need fixtures.
- **Put it in `tools/lint`.** Rejected: that module is nested to keep
  `golang.org/x/tools` out of the engine's dependency graph (§11). This package
  has zero dependencies beyond the standard library and belongs where
  `make test` already reaches it.

## 7. Limits, stated

- **It cannot see an unlisted premise.** If a model rests on a section nobody
  put in the manifest, the gate is silent about it. The subject list is a
  reviewed claim, seeded from the correspondence maps' own citations — the same
  limit the `SPEC EDIT OWED` grep has, and it wants the same mitigation: a
  reading pass beside it, not inside it.
- **It cannot tell whether a change matters.** A typo fix in a tracked section
  fires it. That is the safe direction; the cost is a restamp.
- **It cannot detect a restamp that did not re-run the suite.** Nothing textual
  can. The defence is that restamping is a deliberate act with a git-visible
  diff and a `verifiedAt`/`verifiedCommit` that must move with it.
- **`venue-profiles.md` and `adr-history.md` are not tracked**, because no model
  cites them normatively. Revisit if one does — venue-profiles was found *wrong*
  rather than dated only one session ago (§87 §1.2).

## 8. How to run

```
make formal-fresh                              # the gate (also inside make ci)
make formal-restamp MODEL=AmbiguousOrder       # after a real ./run.sh
cd docs/formal && ./run.sh                     # the actual model checking, ~32 min
```
