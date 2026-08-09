# Record §92 — tasks #231 + #232: a retirement that reached the machine and not the lists that enumerate it, and a premise re-derived onto a third section

**Status:** landed, 2026-08-09. Milestone 4, tasks **#231** and **#232**, taken
together because #231 edits the section #232 must re-derive against.
**Documents only, no engine code.** `build-spec.md` v0.9.69 → **v0.9.70**,
`state-machines.md` v0.9.59 → **v0.9.60**, `adr-history.md` v0.9.69 →
**v0.9.70**, `docs/formal/freshness.json` (four markers re-pinned, one
discharged, one row added), `docs/formal/fence-takeover-model.md` (§4 gains the
re-derivation). `make ci` green on both modules; the `SPEC EDIT OWED` grep is
zero.

## 0. Why the pair, and why in this order

#232 discharges by re-deriving a justification against `build-spec.md` §6 and
then **restamping** that section for `FenceTakeover`. #231 **edits** §6. Done in
the other order, #232's restamp would have been spent on a digest #231
immediately invalidated — a marker "pins the digest it is stale against, so it
covers exactly one delta" (§88), and the delta it covered would have been the
wrong one. The ordering is forced by the manifest's own semantics, not by
convenience.

## 1. #231 — the defect, and the SEVEN sites the task did not name

v0.9.67 (task #224, record §87 R6) retired `TerminalFlatten`: the state
*recorded* an ending and issued no order, so a machine entering it asserted a
flatten the venue had not performed. Record §87 §4 scoped the edit as
"state-machines §7: `TerminalFlatten` — the state and BOTH its incoming edges —
is removed or redefined". That is what landed. The state table and the
transition rows were corrected.

**The retirement did not reach the places that ENUMERATE the state or POINT AT
it.** #231 named one. The sweep found **eight, over four sections in two
documents** — and it found them in two passes, which is the method note in §6:

| Site | What it claimed | Found by |
|---|---|---|
| core §6 step 7 | `terminal-flattening` is a producer of `outstanding_reserved_reductions` | the task |
| SM §7, ledger-movements invariant | the entry moves on `Complete`, `TerminalFlatten`, **and either supersession row** | pass 1 (mirror) — **plus §2 below** |
| SM §7, supersession-retires invariant | an external row may fire against a generation "already at `Complete` or `TerminalFlatten`" | pass 1 (mirror) |
| core §5.3, generation-mechanics pointer | "terminal flatten" is among the mechanics `[mechanism: SM §7]` | pass 1 |
| core §9 chaos row (+ its SM §7 quotation) | the delever escalation is "bounded + terminal-flatten escalation" | pass 1 |
| **SM §2, its own copy of the producer list** | same dead producer, in the CLASSIFIER's own machine | pass 2 |
| SM §7, quiesce **termination** argument | the spot-side park's exits are "`Complete` or `TerminalFlatten`" | pass 2 |
| SM §7, FLAG-4 ruling | `Held/Alert`'s terminal exit "enters `TerminalFlatten`" | pass 2 |

The last three are the ones worth the method note — **the first pass matched all
three and never opened them** (§6). **SM §2 matters most of the three**: it is
the classifier's own machine and a `subject` section of model (a), so the stale
producer sat in the document the model **encodes**, not only in ones it cites. **The termination argument matters for a different reason:** it
bounds the quiesce obligation by enumerating the park's successors, and an exit
nothing can reach is the last kind of member to leave in a proof that the set of
exits is complete.

Why the three producer-list sites matter beyond tidiness: that list is the
**normative enumeration of what re-triggers §2 classification** `[→ FM-6]`, and
it is stated three times — core §6 step 7, SM §2 and SM §7 — which is itself the
condition for this defect. A
producer naming a state nothing can enter is the *unreachable sentinel* shape at
document scope — the list reads as total while one entry is dead, and the next
reader deriving the producer set from that sentence gets a set that cannot be
implemented.

`core §7.1`'s lowercase *"a terminal flatten closes the perp first"* was
inspected and **left**: it describes the act, not the retired state, and it is
correct of `EmergencyClosePackage`.

## 2. The finding that was not in the task, and it is the sharper one

SM §7's ledger-movements invariant also said the entry moves on
**"either supersession row"**. *Either* means two.

`git log -S` dates the phrase to **3a64cf6 (slice E)**, when there were two — the
`EmergencyClosePackage` row and the `FenceReadyLost` row. **310e854 (slice F)**
added a third: the premise-lost row `[→ FM-8, FM-9]`, which ends a generation and
retires its entry exactly as the external two do. SM §7's own quiesce invariant
already says "**all three** superseding paths", eight bullets below, so the
document contradicts itself and has since slice F.

So one sentence carried a **dead** member and was missing a **live** one, and the
two are not symmetric hazards:

- a dead member is **findable** — a reader looks `TerminalFlatten` up and lands
  on a retirement note two tables above;
- a missing member is **invisible** — nothing in the document states how many
  supersession rows there are, so there is no count to check the list against;
- and `FM-6`'s failure is **one-directional**: the classifier that does not
  re-run keeps the verdict the last *order* event left it. The trigger
  enumeration is exactly what `FM-6` exists to make total.

**Generalization, and it is the reusable half:** *a fix applied in one place gets
applied to its mirror* is this repo's standing rule, and it is what found the
other seven sites. It did not find the missing row — that came from reading the enumeration for
**totality** rather than for **correctness**. The two are different passes. A
list can be entirely correct about every member it names and still be short one,
and the tell here was an indefinite quantifier (*either*) doing the work a count
should do.

## 3. What changed, and the two things that are more than errata

1. **core §6 step 7 now states the COUNT.** "being superseded" became "being
   superseded — all THREE of SM §7's supersession rows, the terminal rung's
   `EmergencyClosePackage` (§5.3) among them". An indefinite plural cannot be
   checked for totality by the reader who has to implement it.
2. **SM §7's supersession-retires invariant loses its second already-ended
   state, which raises a question the two-member set concealed.** With
   `TerminalFlatten` gone, `Complete` is the **whole** of the set an external row
   may legitimately fire against — so *what about an already-`Superseded`
   generation?* becomes askable. **Ruled a refusal**, and recorded: the ending
   carries the path that decides whether the ladder resumes (`ResumesLadder`), a
   second row landing on it would overwrite that answer, and the row is durably
   journaled **before** the aggregate judges it — so the record would name an
   arbitration ending a generation another arbitration had already ended. (The
   implementation already refuses it, `ErrGenerationAlreadySuperseded`. The
   reason above stands on its own terms; the code is corroboration, not the
   argument.)

## 4. #232 — the re-derivation, and its result

**The claim under review.** `FenceTakeover`'s liveness layer puts **WF** on *"the
obligated ≤5 s naked-reduce (core §6.7 — not only in wind-down)"* (companion §4
item 4), and the FM-2 lasso analysis rests on the same obligation: the un-fenced
healthy bot's *"obligated ≤5 s naked-reduce keeps submitting closes"*. The
freshness gate reported `build-spec.md` §6 changed since 310e854. `FenceTakeover`'s
**subject** sections (SM §3/§4/§6) are byte-identical, so its 14 runs stand —
what was owed was a **re-derivation, not a re-run**.

**Method.** Extract §6 at 310e854 and at HEAD with an extractor that reproduces
the gate's own digest (`322aba32…`, verified against the gate's output before
trusting a single diff — the extractor is an oracle only once it agrees with the
instrument), then word-diff **step 7 alone**, which is the only part of §6 this
model cites (`why`: *"cited as core §6.7 … §6 carries no subsections"* — checked
by grep over the companion: two citations, both §6.7).

**Result: three opcodes.** Two are #231's producer-list correction, which this
model does not encode. The third is the FM-5-family refuse-to-trade clause. And
**the sentence the fairness set encodes is byte-identical**:

> "Here the handler **immediately** (≤5 s, §9) issues an `ExposureReduction`
> (§5.3) sized from `realized_delta − outstanding_reserved_reductions`"

**Why the third edit could have mattered, and why it does not.** It adds a state
in which the handler **holds new trading**. Fairness assumed over a guard the
spec has since narrowed is fairness *stronger than the spec grants* — the
direction that proves liveness the system does not have, silently, because no run
changes. It does not bite for a reason that lives **outside core §6**: core §7.5
rules that hold **stop-opening** rather than halted, *"the order may be
executing, so the safety verbs must stay available"*, and the ≤5 s determined
flatten is a safety verb. FM-2's lasso is clear twice over — its bot is
**healthy by construction** (a spurious watchdog fire on a live, healthy bot), so
the new clause's antecedent is false in that trace.

**Verdict: the fairness encoding holds.** `build-spec.md` §6 is restamped for
`FenceTakeover` and the marker is removed. #232 is discharged.

## 5. The re-derivation moved the premise, so the manifest gained a row

The justification above **consumes core §7.5**. Before today it consumed only
core §6. An edit to §7.5 widening "holds new trading" to a full halt would void
this conclusion with **no run changing, no invariant going red, and §6's digest
untouched** — the gate would stay green over an expired justification, which is
`FM-7` exactly, one layer out from where §88 first met it.

So `build-spec.md §7.5` is now a tracked **premise** of `FenceTakeover`, with a
`why` naming this derivation. Two things follow, and both are accepted rather
than glossed:

- §7.5 is an actively-moving section (#224 owes it two boot preconditions), so
  this row will produce a marker soon. That is the gate working, not friction to
  design away — the alternative is the justification quietly expiring.
- **`make formal-restamp MODEL=FenceTakeover` was NOT used**, and could not be:
  it restamps *every* subject of the model, including SM §7, which is #229's and
  must stay marked. Discharging one premise row is a hand edit. That the blunt
  tool would have silently absorbed somebody else's marker is worth knowing
  before the next person reaches for it.

## 6. How it was checked

- **The extractor is an oracle only after it agrees with the instrument.** Its
  digest for §6 (`322aba32…`) matches what `make formal-fresh` printed for the
  same section before any diff was believed.
- **Positive control on both new manifest rows.** A green gate proves nothing
  until it can go red on the rows just written. `build-spec.md` §7.5 and §6 were
  each perturbed and the gate reported §7.5 as *"CHANGED … and nothing owns it"*
  for `FenceTakeover` and §6 as changed for both models; the perturbation was
  then reverted and the gate re-run green. Without this the newly-added §7.5 row
  and the newly-**discharged** §6 row are indistinguishable from rows the gate
  ignores.
- **The site sweep is itself a claim** ("these are all the places"), this one was
  WRONG the first time, and **the pattern was never the problem** — which is the
  most useful thing in this record. Pass 1 grepped the identifier
  (`TerminalFlatten|terminal-flatten|terminal flatten`) and had **full recall**:
  15 lines, every one of the eight sites among them. I read **all five** of
  `build-spec.md`'s hits and **five of `state-machines.md`'s ten** — because I
  navigated §7 by its state table and its invariant list, which is how a person
  reads a state machine, rather than by the hit list. The three I never opened
  are exactly the three that were missing: SM §2 (line 98, nowhere near §7), SM
  §7's termination argument and SM §7's FLAG-4 ruling. Pass 2 was a differently
  *shaped* sweep — case-insensitive `flatten` over both documents, **58 lines**,
  printed with ±85 characters of context and walked one hit at a time — and it
  found all three. Of the 58, eight were the defect, one (`core §7.1`'s *"a
  terminal flatten closes the perp first"*) was correct-as-written and is left
  alone, and the rest are a different flatten entirely: the actuator's, the
  blind-flatten prohibition, the `Safety`-class shed rows. **A hit count is an
  execution count** — the coverage lesson from CLAUDE.md arriving at a grep
  instead of a fixture, and the fix is the same one: print what you reached.
- `make ci` green on both modules; `grep -rn "SPEC EDIT OWED" internal/ tools/`
  is zero.

## 7. Rejected alternatives

- **Delete `terminal-flattening` and stop.** Rejected: a bare deletion answers
  "is this list wrong?" and not "where did the terminal ending go?", which is the
  question the deletion provokes. Every site now names the replacement.
- **Fix only core §6 step 7, the site the task named.** Rejected on the standing
  mirror rule, and it would have left SM §7's twin invariant asserting the same
  dead producer while the core's was corrected — external review could no longer
  tell which document was right.
- **Rewrite the historical records to match.** Rejected: ADR §3's FM-6 row and
  ADR §4's FLAG-4 resolution both quote the pre-v0.9.67 producer set, and a
  finding record states what was concluded *then*. Both gain a **forward
  pointer** instead — the treatment §90 gave §87 R1b for the same reason.
- **Remove `GenerationTerminalFlatten` from `internal/oms` in this session.**
  Rejected as out of scope and probably wrong as stated: the phase value must
  survive as a **decode** target regardless, or a journal written before v0.9.67
  stops replaying. Task **#226** owns the removal-or-redefinition, and
  `delever.go:640` already records why it carries no `[SPEC EDIT OWED]` marker
  (the spec moved first, by decision; the code owes the change).
- **Restamp `build-spec.md` §6 for `AmbiguousOrder` too.** Rejected: #231
  discharges the *producer-list* half of that marker, but the FM-5-family clause
  in the same section is model (a)'s own `FM-5` one layer out and is squarely
  #229's. The marker is **re-pinned with a narrowed note**, not removed.
- **Leave `build-spec.md` §7.5 untracked and note the dependency in prose.**
  Rejected — that is the shape `FM-7` names. A justification recorded in a
  paragraph nobody re-derives is what this gate exists to replace.

## 8. What this does not do

- It does not touch what any model **encodes**. Model (a) still has
  `PdTerminalFlatten` and its `PC19` probe; the companion still quotes the old
  producer list. That is **#229**, whose scope this record widens by one item:
  the trigger set the revision must encode is now three supersession rows, and
  the enumeration it re-reads has changed since the marker was written.
- It does not run TLC. `FenceTakeover`'s 14 runs are unchanged and unrepeated;
  the only claim made about them is that their **subject** sections are
  byte-identical, which the gate asserts on every build.
- It does not settle whether "holds new trading" is well-scoped anywhere other
  than §7.5's ruling. The phrase appears at eight sites; only §7.5 defines it.
  That is enough for this derivation and is not a general audit.
