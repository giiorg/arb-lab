# §46 — Formal model (a): ambiguous-order classifier, finality contract, reservation ledger — design (2026-08-01)

Models `state-machines.md` **§2** (ambiguous-order classifier & the finality
contract) composed with **§7** (`PackageDelever` generations & reserved
reductions). Companion to `2026-07-20-formal-model-b-fence-takeover-design.md`;
same process, same artifact discipline, next finding tag is **FM-5**.

---

## 1. Purpose & standing

**Commissioned by the user 2026-08-01 (session 64)** — HANDOFF's open
formal-methods decision (ii), taken. The occasion is that §12 hard-gates
milestones 3–7 on strategy #2's economic gate, which now waits ~2 months on
evidence rather than on code; this is the substantial engineering that gate
does *not* forbid.

**Why this is not a bet on the gate passing.** Core §6 is titled *"Trade
lifecycle (**foundation, strategy-agnostic**)"*. The classifier solves the
two-leg partial-failure problem — a compensating trade against an unresolved
counter-leg can *create* the exposure it removes — which is equally true of
cash-and-carry (spot + perp) and of strategy #2's cross-venue frame (perp +
perp). It is wasted only if the **project** ends, not if **strategy #2** ends.
Milestone 3's roster independently requires it: *"The ambiguous-order-recovery
state machine … is this milestone's safety kernel — **model-test it here,
before any live capital**."*

**Standing.** Same as model (b): the model is authoritative about the
behaviour it covers, the spec remains normative, and any disagreement is a
finding resolved in the doc set under an `FM-` tag — never a silent fix on
either side.

**Adoption evidence this extends.** Model (b) produced FM-1 (composition),
FM-2 (spurious-fire silence), FM-3 (found by attacking a previous round's
*fix*), FM-4 (pre-branch arbitration silence) — four defects that four rounds
of human and machine review missed. The honest counter-evidence it also
produced: model and verification bugs appeared at a comparable rate, and
Cx8's own fix introduced a liveness dead end that only the suite caught.

---

## 2. What is modeled (and what deliberately is not)

### In scope

1. **The per-hedge-group residual classifier** — `Determined`, `Ambiguous`,
   `DeadlineExceededAmbiguous`, and the rule that classification (not the raw
   delta) gates every compensating trade.
2. **The possible-delta interval algebra.** Each unresolved exposure-changing
   counter-leg order contributes an interval `[a,b]`, not a point; the group's
   net lies in the summed interval. This is the heart of the model.
3. **Re-classification on lifecycle events, not only fills** (§2 Scope,
   `[→ R1d-3]`) — a cancel confirmed with **zero** executions emits no fill and
   no position event yet fully collapses that order's interval. Modeled
   explicitly, because "waits for the next fill" is a reachable wrong
   implementation the spec calls out by name.
4. **The finality contract as a barrier with a deadline** — its four steps
   (fence dispatch path → terminal status → sweep past a watermark →
   reconcile to that watermark) abstract to *one* barrier that either
   collapses an order's interval or exceeds its deadline.
5. **`IntervalFloorReduction`** — the deadline-exceeded branch's sole
   exception: endpoint-nearest-zero magnitude, only when the summed interval
   does not straddle zero, reserved as its own generation, **never issued
   twice**, and its own ambiguous result re-entering the same contract.
6. **`outstanding_reserved_reductions` as a shared ledger with three
   producers** — the determined branch's fenced `ExposureReduction`
   generation, the `IntervalFloorReduction`, and `PackageDelever`'s reserved
   spot leg. **This is the composition surface and the main reason §7 is in
   scope** (see §2.1 below).
7. **`PackageDelever` generation phases** (§7) — enough to produce a
   generation-scoped reservation, its perp-first sequencing, the durable
   `SpotReduceAuthorized` record (FLAG-3), the nonterminal `Held/Alert`
   (FLAG-4), and `TerminalFlatten`.
8. **Both supersession paths** — `EmergencyClosePackage` (core §5.3) and
   `FenceReadyLost` arbitration (core §7.4) — each of which must quiesce the
   generation's reserved second leg to §6.7 finality *first*.

### 2.1 The scope ruling: §2 **with** §7, not §2 alone

**Decision: model both.** The alternative — model §2's classifier alone, which
is smaller and self-contained — is **rejected**, and the reason is model (b)'s
own headline result: FM-1 was a **composition** defect, invisible to review of
either machine separately.

Here the composition is concrete and three-way. The determined branch sizes
from `realized_delta − outstanding_reserved_reductions`; the
`IntervalFloorReduction` *adds itself* to that ledger so the floor is never
issued twice; and `PackageDelever` reserves its second leg in the same ledger
so an intentional net-long transient is not misread as naked. Three producers,
one subtraction, and the safety proofs of all three rest on it being exactly
right. A model of §2 alone would encode the ledger as a single-producer
counter and prove a theorem about a system we are not building.

**Cost accepted:** a larger state space than model (b). Mitigation in §5.

### Deliberately not modeled

- **Venue-specific REST semantics** — Binance `positionMargin/history` match
  vs Bybit `positionIM` delta-vs-before-state (§7 Invariants). These are
  *Profiles* facts; a model encoding them would be checking a transcription,
  not a behaviour. The `MarginOp` resolution is abstracted to
  ok / failed / **ambiguous**, which is the part that has state.
- **Money, prices, fees.** Quantities are small bounded integers. No
  `fixedpoint` semantics; rounding is not what these invariants are about.
- **The liq-distance trigger and the top-up latency budget** (§7's entry
  context, core §7.1). `PackageDelever` entry is a nondeterministic event.
  Modeling the mark-driven trigger adds states without touching any invariant
  under test — and it is model (b)'s territory-adjacent, not this one's.
- **The §1 OMS order lifecycle in full.** An order is modeled by the interval
  it contributes and whether it is resolved — the eight-state lifecycle is
  §1's, and the classifier reads only "has remaining executable or unobserved
  quantity".
- **Real time.** Deadlines are nondeterministic events, exactly as in model
  (b). "The deadline was exceeded" is a choice the environment makes, not a
  clock the model advances.
- **The Lean 4 interval-floor arithmetic lemma** — a separate optional one-off
  (HANDOFF open decision (iii)), not folded in here.

---

## 3. Model architecture (processes → variables)

One hedge group. The environment is adversarial: it chooses when orders
resolve, where inside their intervals they resolve, and when deadlines blow.

| Variable | Meaning |
|---|---|
| `orders` | id ↦ `[lo, hi, resolved]`. An unresolved order's true contribution is *any* value in `[lo,hi]`; resolution picks one and fixes it. |
| `realized` | realized net delta from the §1 fill ledger (integer). |
| `reserved` | `outstanding_reserved_reductions` — the shared ledger, with the *producer* recorded per entry so double-counting is detectable rather than assumed away. |
| `cls` | `Determined` / `Ambiguous` / `DeadlineExceeded`. |
| `floorIssued` | whether an `IntervalFloorReduction` generation exists. Recorded, but **not** what INV2 reads — see below. |
| `issued` | the sequence of reductions actually dispatched, with magnitude, sign and kind. The invariants are checked against **this**, not against the guard flags. |
| `pd` | `PackageDelever` phase: none / `Planned` / `PerpReduceInFlight` / `PerpReduceFinal` / `SpotReduceAuthorized` / `SpotReduceInFlight` / `Complete` / `Held` / `TerminalFlatten`. |
| `superseded` | which superseding path fired, if any. |

**The one architectural commitment that makes the safety property meaningful:**
invariants are evaluated over **every** resolution of the still-unresolved
intervals, not over one chosen future. §2's acceptance case says *"no
naked-short created and no doubled/over-reduced short across **every reachable
venue state**"* — so "reachable" must quantify over resolutions, or the model
proves a weaker theorem than the spec states and looks green doing it.

**The one modeling trap to avoid, named now.** `floorIssued` is a guard the
*implementation* will have. If INV2 ("never issued twice") is written as
`floorIssued ⇒ ¬issue`, the model proves the flag consistent with itself —
vacuously true and worthless. INV2 must count `IntervalFloorReduction` entries
in `issued`. This is the same defect class as a fixture pinned to the value
that collapses the distinction: **the property must not read the mechanism it
is checking.**

---

## 4. Checked properties

### Safety

| Tag | Property |
|---|---|
| `INV1` | **NeverOvershootSign** — after any dispatched reduction, the resulting exposure does not cross into the opposite sign under **any** resolution of the still-unresolved intervals. This is the interval-floor proof, mechanized. |
| `INV2` | **FloorIssuedAtMostOnce** — at most one `IntervalFloorReduction` in `issued` per deadline-exceeded episode, counted from `issued`, never from the guard flag. |
| `INV3` | **NoUncertainCompensation** — while `Ambiguous`, no reduction is dispatched at all; while `DeadlineExceeded`, only the interval floor, and only when the summed interval does not straddle zero. |
| `INV4` | **ReservationLedgerSound** — determined sizing (`realized − reserved`) neither double-counts a reservation nor misses one; no reserved transient is read as naked residual, and no naked residual is masked by a reservation that has already completed. |
| `INV5` | **PackageDeleverNeverNetShort** — the generation's transient is net-long in every reachable interleaving; spot sold never exceeds the confirmed perp reduction. |
| `INV6` | **SupersessionQuiescesFirst** — neither `EmergencyClosePackage` nor `FenceReadyLost` ends a generation without its reserved second leg reaching finality first. |
| `INV7` | **DeterminedIsReallyDetermined** — the classifier reports `Determined` only when no order retains remaining executable *or unobserved* quantity. Guards against the classifier reading fills alone. |

### Liveness

| Tag | Property |
|---|---|
| `P1` | An `Ambiguous` residual eventually reaches `Determined` or `DeadlineExceeded` — it never sits unclassified. |
| `P2` | A `DeadlineExceeded` episode whose intervals later collapse eventually reaches `Determined` (§2's late-resolution transition). |
| `P3` | A `Held/Alert` generation eventually resumes to `Complete` or enters `TerminalFlatten` — FLAG-4's nonterminal ruling, mechanized. |
| `P4` | An `IntervalFloorReduction` whose own result is ambiguous eventually resolves through the same contract — i.e. the recursion terminates. |

**As built (slice C), `P2` is checked in a stronger form than this table
states and `P4` in a narrower one. Both are recorded in §9e with the
reason** — this table is normative for the model, so a divergence between it
and `AmbiguousOrder.tla` is marked here rather than left to be found.

### Negative controls — **one per liveness property, from the start**

Model (b)'s sharpest methodological lesson (Cx8) was that **a passing liveness
property and a vacuous one look identical**. `NC5` was added late and proved
`P4` bites; `P2`, `P3`, `Branch1AutoResolves` and `Branch2AutoResolves` in
that model still have none. **This model does not repeat that**: each of
`P1`–`P4` ships with a control config that breaks exactly one constant or
transition and is expected to *fail*, recorded with its output. A liveness
property with no control is not considered checked.

---

## 5. Deliverables & execution plan

Sliced so each step ends somewhere defensible.

| Slice | Content | Done when |
|---|---|---|
| **A** | `AmbiguousOrder.tla` — variables, the interval algebra, classifier transitions, `INV1`–`INV3`, `INV7`. §2 only; `reserved` present but single-producer. | Nominal TLC run green; one deliberate broken-constant run shows each invariant *can* fail. |
| **B** | The reservation ledger's other two producers + `PackageDelever` phases; `INV4`–`INV6`. This is where the §2.1 composition actually gets checked. | Green, plus the positive control that the *unbroken* model reaches the interesting states (state-count reported, not assumed). |
| **C** | Liveness `P1`–`P4` and their four negative controls. | Each `P` green **and** its `NC` red, both outputs committed. |
| **D** | Companion artifact `docs/formal/ambiguous-order-model.md` — correspondence map (spec clause ↔ model construct), results table, honesty notes, how-to-run; `run.sh` extended. | Every clause in §2/§7's Invariants sections either mapped or listed as deliberately unmodeled with a reason. |
| **E** | Adversarial round on the model itself. | Findings triaged; spec defects tagged `FM-5…`, model defects fixed and re-verified. |

**Expect the model to be wrong at least once.** Cx6 found six findings *in
model (b)*, and two of Cx7's four items were model bugs rather than spec bugs.
Slice E is not a formality; it is where half the value historically came from.

---

## 6. Risks / honesty notes

- **State-space growth is the real risk of the §2.1 ruling.** Three
  reservation producers × generation phases × per-order resolution
  nondeterminism multiplies fast. Mitigation: two orders and a
  three-point quantity domain to start, widened only if a property turns out
  to be insensitive to the bound. **If a bound has to stay small, that is a
  stated limit in the artifact, not a silent choice** — model (b)'s honesty
  notes set the precedent.
- **The interval algebra is where a modeling error would hide.** The spec
  itself already carried one here: the v0.7.1 `|b|` phrasing was correct only
  for the net-long case, corrected to `max(|a|,|b|)` by R1d-2. A model that
  re-implements the *corrected* formula and checks itself against it proves
  nothing. `INV1` must be phrased over resolutions (*"no resolution produces
  an opposite sign"*), never over the endpoint formula.
- **`DeadlineExceededAmbiguous` is not a terminal state and the recursion is
  real** — the floor's own ambiguous result re-enters the same contract. `P4`
  exists because that is a plausible non-termination, and non-termination here
  means a naked position held open indefinitely.
- **This model cannot see implementation drift.** It checks the spec's
  mechanism. The eventual defence is TLC trace-checking against the real OMS
  (HANDOFF's stated milestone-3 direction), which is out of scope here.
- **Cost.** Model (b) ran ≈5 sessions including its adversarial rounds. If
  this one finds nothing by end of slice C, that is itself a result worth
  recording — the practice's value is measured over rounds, and one silent
  round is evidence, not failure.

---

## 7. Slice A — BUILT and VERIFIED (2026-08-01, session 65)

`docs/formal/AmbiguousOrder.tla` + 7 configs + `AmbiguousOrderCoverage.tla`,
with `run.sh` extended to check both models. **Nominal green; six negative
controls each violate their predicted invariant; eight positive controls each
confirm an interesting state is reached.**

### 7a. The design decisions taken during the build

Each of these was a fork, and the rejected side is recorded so it is not
silently reopened.

**1. An order is `(dir, q)`, not `(lo, hi)` — so an interval always has zero
as an endpoint.** A single order executes in ONE direction, so its remaining
possible contribution is `[-q, 0]` or `[0, q]`; it can never straddle.
*Straddling is a property of the SUM* — which is exactly where §2 forbids
compensation. The rejected `(lo, hi)` encoding was tried first and is not just
uglier, it is **unfaithful and unbounded**: modeling a partial fill as a shift
of both endpoints lets an order fill to one extreme and then back by the full
width, so the endpoints drift without limit and the state space never closes.
The direction-and-remainder encoding makes fills monotone-shrinking, which is
what makes the model finite at all.

**2. `Impl*` versus `True*`, as a structural defence rather than a rule to
remember.** Every computed quantity exists twice: `ImplResidual`/`ImplLo`/
`ImplHi`, which a broken constant corrupts and which **only action guards
read**, and `TrueResidual`/`TrueLo`/`TrueHi`, the ledger-exact values which
**only invariants read**. §46 §3's trap ("the property must not read the
mechanism it is checking") stops being a discipline applied invariant by
invariant and becomes a property of the file's layout: an invariant physically
cannot see a constant's effect on the implementation's arithmetic.

**3. `floorIssued` and `floorCount` are two variables, deliberately.**
`floorIssued` is the guard the implementation has; `floorCount` counts
generations actually **dispatched** in the current episode, and `INV2` reads
only the count. This is §46 §3's named trap implemented rather than merely
avoided — the two are kept apart in the variable list so a later editor has to
work to reintroduce the vacuous form.

**4. `INV1` is checked at dispatch AND persistently, and the second conjunct
exists to CHECK the argument for the first rather than to trust it.** The
at-dispatch flag is the faithful mechanization ("this dispatch made the
opposite sign reachable"). It is sufficient only because of a small theorem
about the model: a **fill leaves the true net range exactly invariant**
(`realized` and the interval move by equal and opposite amounts), **finality
only shrinks it**, and **an ambiguous reduction result widens it only back
toward the pre-dispatch side**. Rather than assert that in a comment, the
persistent conjunct (`longReduced ⇒ TrueLo ≥ 0`, and its mirror) makes TLC
check it: if it ever fires where the at-dispatch flag did not, the reasoning
was wrong, which is a finding either way. It has not fired.

  **Measured, not assumed:** with the at-dispatch conjunct stripped out in a
  scratch copy, the persistent half **still catches NC1 and NC6 on its own**.
  So the two are independently-biting formulations of the same claim rather
  than one real check and one decoration — which is the answer that makes
  keeping both worth the line.

**5. Six negative controls, not four.** §46 §5 asked for one per invariant.
`INV3` got two because its clauses are **separately load-bearing** — NC3
breaks the ambiguous-window clause, NC5 breaks the straddle clause, and a
control on one says nothing about the other. `INV1` got two because its two
failure mechanisms are unrelated — NC1 breaks the endpoint rule (the
arithmetic), NC6 breaks the reservation subtraction (the ledger). NC6 is the
one that exercises §2.1's composition surface, and it is the reason `NPEND`
is 2 (see 7c).

**6. The finality contract is one barrier, and `OrderFinal` is a first-class
action.** §2 Scope names the case that motivates it — "a cancel confirmed with
**zero** executions emits no fill and no position event, yet fully collapses
that order's interval" — so collapsing-without-a-fill is a separate action
from filling, not a parameter of it. Probe PC8 confirms it fires.

**7. Deadlock checking stays ON.** `Settled` is an explicit self-loop at the
one terminal the model predicts (everything collapsed, nothing reserved, and
either the residual is zero or the dispatch budget is spent). Any *other* dead
end is therefore reported by TLC as a deadlock and is a finding rather than
noise. The alternative — `-deadlock` off, or a blanket `UNCHANGED vars`
disjunct — would have silently absorbed exactly the states worth seeing.

  **And that check was itself positively controlled**, because "TLC reported
  no deadlock" is another clean run that could mean nothing: `Settled` was
  disabled in a scratch copy, and TLC reported `Deadlock reached` immediately.
  So the nominal run's silence about terminal states is evidence, not vacuum.

### 7b. Results

Toolchain: Java 25, `tla2tools.jar` pinned by SHA-256, TLC 2.19. Whole model-(a)
block runs in **under a second** — nominal is 25,514 states generated / **5,169
distinct**, depth 15.

| Run | Constant flipped | Predicted | Result |
|---|---|---|---|
| nominal | — | pass, all four invariants | **PASS** (5,169 distinct, depth 15, no deadlock) |
| NC1 | `FLOOR_NEAREST_ZERO = FALSE` | `INV1` | **VIOLATED** |
| NC2 | `FLOOR_ONCE = FALSE` | `INV2` | **VIOLATED** |
| NC3 | `BLIND_COMPENSATION = TRUE` | `INV3` | **VIOLATED** |
| NC4 | `CLASSIFY_ON_FILLS_ONLY = TRUE` | `INV7` | **VIOLATED** (initial state) |
| NC5 | `STRADDLE_GUARD = FALSE` | `INV3` | **VIOLATED** |
| NC6 | `RESERVATION_AWARE_SIZING = FALSE` | `INV1` | **VIOLATED** |

**Suite plumbing.** `run.sh` now takes a module argument and checks both
models — 8 runs for (b), 7 for (a), 8 positive controls — plus two additions
paid for this session: **`TLC_WORKERS`** (capped at 4 throughout, so a
30-minute all-core run could not starve the capture) and an optional
**config-tag filter**, because re-running 23 configs to re-check one is how a
suite stops being run. The filter prints `FILTERED: n run(s) skipped -- NOT
the full suite` in its summary line, so **a partial green can never be quoted
as the whole one**. **All 23 runs across both models were re-verified through
the refactored script this session** (in filtered batches at
`TLC_WORKERS=4`), and model (b) — untouched this session — **reproduces its
recorded v0.9.4 state graphs exactly**: `_Live` 3,945,352 / 491,160 (8 min
22 s) and `_Live2` 1,705,418 / 249,936, both to the digit. `NC5` still
violates `P4_ArbitrationCompletes` as predicted. That reproduction is worth
more than the pass itself: it says the pinned toolchain, the configs and the
module still produce the same object a different session recorded.

**Each was read, not merely counted** — a violation by the wrong mechanism is
the formal-methods twin of `mutate.sh`'s `PANIC!!`: a kill, but not the one
aimed at. Two worth recording:

- **NC1** reproduces R1d-2's asymmetry exactly. Residual `+1` with a pending
  sell of 1 gives the summed interval `[0, 1]`: the near endpoint is **zero**,
  because the pending hedge may already cover the whole position. The
  nearest-zero rule therefore issues nothing; the worst-endpoint rule sells 1
  and reaches `-1`. That is the v0.7.1 `|b|` defect as a lasso.
- **NC6** dispatches `-1` against a determined `-1` residual and then, with the
  reservation not subtracted, **dispatches `-1` again** — the reserved
  generation re-read as fresh naked residual, which is the failure the
  `realized_delta − outstanding_reserved_reductions` sizing exists to prevent.

### 7c. The positive controls — and the model-altitude fixture collapse they caught

*A clean run and a vacuous run look identical.* `AmbiguousOrderCoverage.tla`
holds eight probes, each phrased as the NEGATIVE claim, so **a TLC violation is
the good outcome and a pass is a coverage gap**. All eight are violated:

| Probe | The state it proves is reached |
|---|---|
| `PC1_FloorNeverDispatched` | an `IntervalFloorReduction` is issued at all |
| `PC2_NeverTwoGenerations` | two reserved generations live at once |
| `PC3_SubtrahendAlwaysZero` | a determined sizing with a **nonzero subtrahend** |
| `PC4_StraddleNeverReached` | a straddling summed interval inside the deadline branch |
| `PC5_FloorNeverRecursed` | the floor's own ambiguous result re-entering the contract |
| `PC6_NoShortSideFloor` | a floor on the **net-short** side |
| `PC7_NoLongSideFloor` | a floor on the **net-long** side |
| `PC8_NoCancelWithZeroExecutions` | a lifecycle event collapsing an **unobserved** order's interval, delta unmoved |

**`NPEND = 2` is not padding, and PC3 is why.** With a single generation slot,
`realized − reserved` would have had a subtrahend **pinned to zero in every
reachable nominal state** — "sized from the residual" and "sized from the
residual net of reservations" would have been the same expression, NC6 would
have been undetectable, and the nominal `INV1` evidence would have been about a
simpler system than the one being built. That is the fixture-collapse checklist
at formal-methods altitude, and the fix is the same one it always is: find the
parameter pinned to the value that collapses the distinction, and vary it. The
witness PC3 produces is the intended one — a floor of `+1` still in flight when
the intervals collapse, so the determined branch sizes `2 − 1`.

**PC6/PC7 are the sign mirror.** The interval-floor rule is asymmetric and the
spec's own defect lived in that asymmetry, so a model reaching only one side
would check half the rule while looking complete.

### 7d. Findings from the build

**A TLA+ precedence trap, caught by TLC on the first run.** `=` binds tighter
than `\/`, so `flag' = flag \/ cond` parses as `(flag' = flag) \/ cond` — a
disjunction satisfied by `cond` alone, leaving the variable **completely
unassigned**. All six history flags had it. TLC refused outright ("successor
state is not completely specified by action …"), which is the good failure
mode; every right-hand side is now fully parenthesized with the reason stated
in the file.

**A candidate for slice E, not yet a finding: is "never issued twice" doing any
safety work?** NC2's counterexample issues a second floor and `overshoot` stays
**FALSE** throughout — the second dispatch was safe, because reservation-aware
sizing had already netted the first one out. The spec's own counterexample for
the rule ("for `[0.4, 1.0]`, two uncertain `0.4` reductions could reach
`-0.4`") assumes the reservation is **not** counted, which the adjacent
sentence requires it to be. So on this model the flat rule is belt-and-braces
over the reservation, not an independent safety property. Either that is
deliberate defence-in-depth and should say so, or there is a case this model
does not reach. **Do not resolve it by weakening the spec.** It is stated here
so slice E attacks it deliberately; if it survives, it is an `FM-5` candidate
of the "a rule stronger than its stated justification" kind.

### 7e. Stated bounds and abstractions (§46 §6's discipline: limits, never silent choices)

- **`NORD = 3`** — two hedge-group counter-legs plus one recycling slot for a
  reduction's ambiguous residue; **`MAXQ = 2`**, **`NPEND = 2`**,
  **`MAX_DISPATCH = 3`**. Budget exhaustion produces a `Settled` stutter with a
  possibly-nonzero residual: that is a modeling terminal, **not** a claim that
  the system may stop compensating. **A trap this sets for slice C:** an
  infinite stutter with an uncompensated residual will make a liveness
  property fail for a reason that is purely the bound. `P1`–`P4` must be
  conditioned on the budget (model (b)'s `BOUND_CRASHES` is the precedent —
  liveness claimed only for behaviors with finitely many of the bounded
  event), or the failure will be read as a defect that is not there.
- **No new counter-leg order appears mid-episode.** Justified from §2 itself —
  the ambiguous window holds new strategy orders — so the only new intervals
  are the model's own reductions' residues, which is what makes `INV1` a claim
  about compensation *creating* an opposite sign rather than about pre-existing
  gap-budget exposure.
- **The fenced generation's identity is abstracted to its reserved amount.**
  Coalesce / amend / cancel-replace is modeled as the sizing guard refusing a
  redundant dispatch, which is the behaviour "never a fresh command per event"
  protects; the amend mechanics themselves are not modeled.
- **`INV7` holds by construction under the nominal constants** — `Reported`
  *is* `Truly`. Its evidence is NC4, not the nominal run.
- **"Never issued twice" is read as PER EPISODE**, and the model implements it
  that way: `floorIssued` and `floorCount` both reset when the classifier
  reports `Determined`, which is what ends a deadline-exceeded episode. §2
  states the rule flatly, but it states it *inside* the
  `DeadlineExceededAmbiguous` row, and a later, distinct episode must plainly
  be allowed its own floor — a global reading would leave a second incident
  with no admissible safe action forever. §46 §4 already phrased `INV2` this
  way; it is restated here because it is a **reading**, and an unstated
  reading is what later gets called drift.
- **The straddling branch's "de-minimis only" latitude is NOT modeled.** §2
  row 6 *permits* a de-minimis action where the interval straddles zero; the
  model dispatches nothing there. That is unmodeled **permitted** behaviour —
  an abstraction, not a contradiction — but it means `INV3`'s straddle clause
  is checked against "no floor", not against "nothing larger than
  de-minimis". Sizing de-minimis would require a magnitude threshold, which
  is money, which §46 §2 puts out of scope.
- **`INV3` forbids *any* reduction while `Ambiguous`, which is stronger than
  "no compensation of the uncertain part" — and deliberately so.** §2 narrows
  it explicitly: the interval floor "applies **only** in the deadline-exceeded
  branch (normal operation waits for finality and compensates the exact
  residual)". So the guaranteed floor of a merely-ambiguous residual is *not*
  compensable, and the model matches the narrowing rather than the general
  sentence.
- **The "MUST NOT wait for the next fill" clause is split.** Its safety half is
  `INV7` (NC4). Its other half — a classifier that re-runs on fills only goes
  *stale-ambiguous*, which **stalls rather than overshoots** — is a liveness
  question and belongs with `P1`/`P2` in slice C, with its own control.

### 7f. What slice B inherits

`reserved` is currently an operator over one producer's generations. Slice B
promotes it to the three-producer ledger of §2.1 (the determined branch's
fenced generation, the `IntervalFloorReduction`, and `PackageDelever`'s
reserved spot leg), adds the §7 generation phases, and adds `INV4`–`INV6`.
`PC3` and NC6 are the precedent to extend: whatever the ledger becomes, there
must be a reachable state in which each producer's contribution to the one
subtraction is nonzero, and a control that breaks it.

---

## 8. Slice B — BUILT and VERIFIED (2026-08-01, session 66)

SM §2 composed with SM §7 on the one shared reservation ledger, with
`INV4`–`INV6`, six more negative controls and thirteen more positive
controls. **Composed nominal green; all twelve negative controls violate
their predicted invariant; all twenty-one positive controls confirm their
state is reached. It found one SPEC DEFECT — `FM-5` — and two defects in
this model's own slice-A reasoning.**

### 8a. How §7 was composed — decisions taken during the build

**1. §7's legs go through machinery that already exists, not beside it.**
The perp-short reduction is an exposure-changing order on the hedge group,
so while it is in flight it contributes a possible-delta interval `[0, q]`
to the *same* sum §2's classifier reads and makes the residual `Ambiguous`
for the *same* reason any counter-leg does. The reserved spot leg is a
`pending` entry, one of the three producers of the one subtraction,
distinguished only by its `kind`. Nothing about §7 is a parallel bookkeeping
track — which is what makes the composition testable rather than asserted.

**2. The generation's own legs are NOT recorded through `RecordDispatch`.**
§2's single-gate invariant says `PackageDelever` *routes through* the
classifier, and it does here — its perp leg lands in the interval algebra,
its spot leg in the ledger. But neither leg *compensates an uncertain
residual*, so `INV1`/`INV3` are not the properties that own them; `INV5` is.
Recording them as dispatches would make `INV3` fire on a generation
completing normally, which is a property the spec does not state.

**3. `MAX_GEN = 2`, not 1.** With one generation the ladder's
`Complete → next generation` edge is unreachable and §7's "only after the
prior generation reaches finality" gate blocks nothing observable. Probe
`PC20` is what proves the second generation is reached.

**4. `NPEND` went 2 → 3.** Slice A needed two slots so the subtrahend could
be nonzero at all; slice B needs three so the sum can have three terms —
the state where §2.1's "three producers, one subtraction" is literally true.
`PC12` proves it is reached. This is slice A's own `NPEND` lesson one
producer out, and it was found by the checklist before the runs, not after.

**5. The `sent` bit.** A ledger entry now records whether its counterpart
order has actually been submitted. It exists because the two questions the
ledger has to answer are different: the SIZING must subtract *intentions*
(that is exactly what stops §7's deliberate net-long transient being
re-hedged), while a claim about what the position can become may only count
what is actually out. See §8e for the two formulations that were tried
before this one and are wrong.

### 8b. Results

Toolchain unchanged: Java 25, `tla2tools.jar` pinned by SHA-256, TLC 2.19,
`TLC_WORKERS=4` throughout (the capture was running).

| Run | Constant flipped | Predicted | Result |
|---|---|---|---|
| nominal (composed) | — | pass, all seven invariants | **PASS** — 6,729,250 generated / **832,179 distinct**, depth 30, no deadlock, ~8 s |
| `_SM2` (`MAX_GEN = 0`) | — | pass, incl. `INV1b` | **PASS** — 25,514 / **5,169**, depth 15 |
| NC1 | `FLOOR_NEAREST_ZERO = FALSE` | `INV1` | **VIOLATED** |
| NC2 | `FLOOR_ONCE = FALSE` | `INV2` | **VIOLATED** |
| NC3 | `BLIND_COMPENSATION = TRUE` | `INV3` | **VIOLATED** |
| NC4 | `CLASSIFY_ON_FILLS_ONLY = TRUE` | `INV7` | **VIOLATED** |
| NC5 | `STRADDLE_GUARD = FALSE` | `INV3` | **VIOLATED** |
| NC6 | `RESERVATION_AWARE_SIZING = FALSE` | `INV1` | **VIOLATED** |
| NC7 | `RESERVE_AT_CREATION = TRUE` | `INV4` | **VIOLATED** — this is `FM-5` |
| NC8 | `SPOT_CAPPED_BY_PERP = FALSE` | `INV5` | **VIOLATED** |
| NC9 | `PERP_FIRST = FALSE` | `INV5` | **VIOLATED** |
| NC10 | `QUIESCE_BEFORE_SUPERSEDE = FALSE` | `INV6` | **VIOLATED** |
| NC11 | `RELEASE_ON_FINALITY = FALSE` | `INV4` | **VIOLATED** |
| NC12 | `RELEASE_ON_SUPERSEDE = FALSE` | `INV4` | **VIOLATED** |

**`_SM2` reproduces slice A's state graph to the digit** — 25,514 / 5,169 at
depth 15, the numbers §7b recorded. That reproduction is the evidence that
slice B's refactor (a doubled ledger, a `stale` field, a `sent` field, a new
`SumHi` term, a changed `Reclassify` arity) did not quietly change the
object slice A verified. It is the same evidence model (b) supplies by
reproducing its v0.9.4 counts, applied within one model instead of across
sessions.

**Every violation was read, not merely counted.** Four are exact:
NC7 books `amt = 1` on a package with `realized = 0` and `pdDone = 0`;
NC8 authorizes a spot sale of 1 against a perp reduction that confirmed
**zero**; NC9 authorizes the spot leg directly from `Planned`, before the
perp order was even submitted; NC10 supersedes at `SpotReduceInFlight` with
`supersededWithLiveLeg` set.

**NC12's witness was DEGENERATE and that had to be chased.** TLC reports the
shortest counterexample, and for the orphan clause that is an entry of
amount **zero** — which leaks a ledger slot but subtracts nothing, so the
control would have demonstrated the leak while saying nothing about the
harm. That is the fixture-collapse tell aimed at a *control's evidence*
rather than at a fixture. Checked separately (`NoNonzeroOrphan`, kept in the
module for the purpose): the harmful orphan **is** reachable — the perp leg
buys 1, the generation is superseded, and a `+1` entry is left behind
masking a genuine naked long forever.

**And NC12 deadlocks, which is a second signature of the same defect.**
With the orphan un-releasable, `Settled` (which requires an empty ledger) is
permanently unreachable and TLC reports `Deadlock reached`. §7a decision 7
kept deadlock checking on precisely so a dead end would be a finding; here
it independently reports "this ledger can never return to empty".

### 8c. Positive controls — slice B's done-when

`PC9`/`PC10`/`PC11` **are** the done-when (§7f): each of the three producers
is shown to contribute a nonzero term to the one subtraction, separately.
`PC3` shows only that the *sum* is nonzero somewhere; a three-producer
ledger in which just one producer is ever nonzero would pass `PC3` while
§2.1's entire argument for modeling §2 with §7 went unexercised. All three
are violated, so all three are reached.

| Probe | The state it proves is reached |
|---|---|
| `PC9`/`PC10`/`PC11` | a determined sizing with a nonzero term from the **exposure**, **floor**, **pkgspot** producer respectively |
| `PC12` | all **three** producers live in the ledger at once |
| `PC13`/`PC14`/`PC15`/`PC19` | `Complete`, `Held`, `SpotReduceInFlight`, `TerminalFlatten` are each entered |
| `PC16`/`PC17` | **both** superseding paths fire, separately — the PC6/PC7 sign-mirror lesson applied to paths |
| `PC18` | a perp reduction confirmed to finality having executed **some but not all** of its target |
| `PC20` | a **second** generation is started |
| `PC21` | the intended net-long transient is actually **nonzero** |

**`PC18` is slice B's fixture-collapse probe, and the checklist named it
before the model ran.** If the perp leg always confirmed in full, then
`pdDone = pdTarget` always, "sells no more spot than the **confirmed** perp
reduction" and "…than the **target**" would be the same expression, and NC8
would be undetectable. `PC21` is its partner: if the transient were always
zero, both of `INV5`'s conjuncts would hold trivially.

### 8d. `FM-5` — the reservation may be counted before its counterpart exists

**The defect.** SM §7's FLAG-3 ruling says the reservation "stays
generation-scoped **from creation**, not first made here
[`SpotReduceAuthorized`]; the §2 classifier **subtracts it throughout**",
and §2's reservation-aware sizing subtracts
`outstanding_reserved_reductions` unconditionally. Read literally and
together, the two sentences let a generation book its **target** into the
ledger at `Planned` — before the perp leg has bought anything.

**The harm, and what is TLC-verified versus derived.** TLC exhibits two
states: the unbacked ledger at **2 steps** (`PdPlan` books `amt = 1` while
`realized = 0` and `pdDone = 0`), and the resulting **dispatch** at
**3 steps** — §2's determined branch reads `realized − reserved` as a
**short** of the target on a package that is neutral, and buys. That buy is
the harm: it buys back exactly the exposure the generation's own perp leg is
about to buy.

The end position **follows arithmetically** from the model's own
transitions and is stated as a derivation, not as a state TLC was asked to
produce: the classifier's buy fills (`realized = +1`), the perp leg buys its
target (`+2`), the spot leg sells the confirmed amount (`+1`) — the package
ends **net long by the target**. Each step is an enabled action of the
model, but the composite was not made a checked property, because pinning
"the package was neutral at the start" would need history the model does not
carry. The checked claim is the dispatch.

**Why four rounds of review and slice A both missed it.** `INV1` is phrased
over the ledger-exact `True*` quantities — but those are *computed from the
ledger*, so a corrupt ledger corrupts them too, and the phantom buy computes
as non-overshooting. `overshoot` is **FALSE** in the violating state. An
invariant built on the ledger cannot audit the ledger, which is why `INV4`
had to exist as a separate property and why it had to be phrased at the
ledger level rather than as the trade (see §8e).

**The resolution, and it is narrow.** What FLAG-3 left underdetermined is
the reservation's **amount**, not its existence. Book the entry at creation
as FLAG-3 requires, but carry the generation's **outstanding net intent** —
it will buy the perp and sell the same back, so at creation, with nothing
bought, the outstanding commitment is **zero**, and it grows with the perp
leg as it fills (the entry is *amended*, which is §2's own "coalesce /
amend / cancel-replace it, never a fresh command per event" applied to the
reservation rather than to the order). Then the ledger is exactly backed at
every instant, FLAG-3's words hold literally, and the phantom short cannot
arise. That reading is the nominal configuration; the literal one is NC7.

**Status: the normative edit is OWED and is slice E's first item, by the
§5 roster** ("findings triaged; spec defects tagged FM-5…"), not by
convenience. Until it lands, `docs/formal/` and `docs/state-machines.md`
disagree about §7's FLAG-3, and CLAUDE.md's standing rule resolves which
wins: *"treat [the formal model] as authoritative about the safety kernel's
behaviour."* The wording to land, in §7's FLAG-3 ruling and beside §2's
reservation-aware sizing, is the paragraph above.

### 8e. Two defects in slice A's own reasoning, found by composing

**`FM-model-1` — `INV1`'s persistent conjunct rests on a premise §7
breaks.** §7a decision 4 justified it with a small theorem about which
events can widen the true net range, and §7e's stated bounds gave the
premise **in these words**: *"no new counter-leg order appears
mid-episode … so the only new intervals are the model's own reductions'
residues."* §7's perp-short reduction **is** a new exposure-changing order
appearing mid-episode. In the composed model the conjunct therefore asserts
"once a short has been reduced, no other authorized command may ever make
long exposure reachable" — which the spec nowhere says and which would
forbid `PackageDelever` outright. TLC produced the counterexample in four
steps.

Decision 4 had pre-committed to the reading — *"if (b) fires where (a) did
not, that reasoning was wrong, which is a finding either way"* — and
`overshoot` was FALSE, so (a) did not fire. **It was not weakened and not
deleted**: it moved to `INV1b` in the `_SM2` configuration, where its
premise holds. Deleting it would have cost real bite, because §7a measured
that it catches NC1 and NC6 on its own. Verified in the other direction this
session: the at-dispatch conjunct alone still catches NC1 and NC6 in the
composed configuration, so nothing is unpoliced there either.

**`FM-model-2` — two formulations of `INV4` were tried and both are
wrong.** Recorded because a rejected alternative nobody wrote down gets
re-proposed, and both are the kind a reviewer would suggest.

- *"The ledger's sum is dominated by the realized delta"*
  (`ReservationsBackedREJECTED`). True throughout slice A, catches FM-5 —
  and **false of correct composed behaviour**: a group short 2, a generation
  whose perp leg buys 1 (moving delta *toward* zero), and a reserved spot
  sale of 1 that undoes exactly that gives `realized = −1` with
  `reserved = +1`. The error is the premise that a reservation is backed by
  the *net* delta; §7's reserved leg is backed by the generation's own
  transient and legitimately moves the group's delta *away* from zero.
- *"No dispatch in a direction no reachable state warrants"*
  (`UnwarrantedREJECTED`). Judged against **sent** reservations only, it
  flags correct anticipation of a booked intent (8 steps: a group flat with
  the generation committed to sell 2, where buying 2 is right and lands
  flat). Judged against **all** reservations, it reads the very ledger that
  is wrong and FM-5 becomes invisible to it again.

**The conclusion is itself the finding: there is no harm-level phrasing here
that is both sound and complete**, because every "what will the position
become" quantity is computed from the ledger. A ledger defect has to be
caught at the ledger level — which is what `IntentsBacked` does, by
forbidding an intention booked against delta the generation has not moved.

### 8f. Stated bounds and abstractions

- **`QDom` widened 0..8 → 0..14 and `AmtDom` ±16 → ±28, and the widening was
  FORCED.** With §7 composed in, `TypeOK` failed at depth 22 on an order of
  `q = 9`. That is the domain doing its declared job — a too-tight domain
  reports as a type violation, a real signal, instead of silently truncating
  behavior. The composition ratchets the reachable residual because §7's
  generations move delta **without spending the dispatch budget**, so the
  reachable magnitude gains `MAX_GEN × MAXQ` on top of what §2's own budget
  allows, and a reduction sized from that residual re-enters as an order of
  that size.
- **`MAX_GEN = 2`, and generations are bounded by that count rather than by
  `budget`.** Each generation's phase chain is finite and its perp leg
  strictly shrinks, so the sub-machine terminates; charging leg submissions
  to the shared dispatch budget would instead have starved §2's own branches
  and cost the floor/exposure coverage. A stated bound, not a claim.
- **A generation may only open on a package the classifier calls
  `Determined` with a zero residual.** That is §7's own entry context (core
  §5.3's "still-neutral package"; §7's "the package has no naked residual"),
  and it is a *reading*: "no naked residual" is evaluated on §2's
  reservation-netted residual, which admits a package that is neutral
  **pending an in-flight reduction** rather than physically flat. Stated
  here because an unstated reading is what later gets called drift. Whether
  physical neutrality is the stronger and intended precondition is a slice-E
  question — it is the mirror of FM-5 and should be attacked with it.
- **The spot leg's size ranges over `0..cap` rather than sitting at the
  cap.** Pinned at the cap, "no more than" and "exactly" would be the same
  predicate and the inequality in §7's own sentence would be unchecked
  (checklist item 3). `s = 0` is the honest extreme: a perp reduction that
  confirmed nothing authorizes no spot sale at all.
- **`Held/Alert`'s resolution is one barrier, and the generation completes
  at it.** FLAG-4's "resumes the remaining per-leg target" is abstracted to
  a single resolution with `x ∈ 0..reserved` executed; a shortfall becomes a
  genuine naked residual that §2's determined branch then compensates, which
  is the composition working. Re-driving the remainder as a fresh leg is not
  modeled.
- **`TerminalFlatten`'s own orders are not modeled.** What is modeled is
  that the generation ends and its reservation stops being counted; the
  flatten's routing through the §6.7 contract is a different scenario's
  legs.
- **The perp leg's identity is abstracted to its remaining quantity.** Its
  four-step finality contract is the same one barrier as `OrderFinal`, which
  is what makes a partial confirmation (`PdPerpFinal` after a partial fill)
  first-class — and `PC18` shows that is reached.

### 8g. What slice C inherits

Liveness `P1`–`P4` and their four negative controls, per §5. Two traps are
already named for it:

- **§7e's budget trap still stands**: budget exhaustion produces a `Settled`
  stutter with a possibly-nonzero residual, so `P1`–`P4` must be conditioned
  on the budget (model (b)'s `BOUND_CRASHES` is the precedent).
- **`MAX_GEN` is a second such bound now.** A generation that cannot start
  because the count is spent is a modeling terminal, not a claim that the
  ladder stops; `P3` (a `Held/Alert` generation eventually resumes or enters
  `TerminalFlatten`) must not read it as a stall.
- **`gens` and `superseded` both gate `PdPlan`**, so a superseded behavior
  never opens another generation. That is faithful (supersession quiesces
  the ladder) but it means a liveness property about *later* generations is
  vacuous after a supersession, and must say so.

---

## 9. Slice C — BUILT and VERIFIED (2026-08-01/02, session 67)

Liveness `P1`–`P4` under a fairness set argued action-by-action, each
shipping with its negative control in the same slice per §5 and decision
(2). **Nominal liveness green on the same 832,179-state graph; all four
controls violate their predicted temporal property; four new
CONTROL-WITNESS probes show each control's harm is real and not an artifact
of where TLC stopped looking.** No spec defect this slice — but it produced
the sharpest available answer to *why* slice C had to exist, and that is
`NC15`: see §9d.

### 9a. Decisions taken during the build

**1. The fairness set is the theorem, so what is left OUT of it is argued
line by line.** A fairness assumption is a premise of every `P`, so a
generous one is a weaker result. Fair: the per-venue finality contract (per
order slot, as the *disjunction* `OrderFill ∨ OrderFinal` — fairness on
`OrderFinal` alone would assume the no-execution path and on `OrderFill`
alone would assume away the cancel case §2 Scope names), the reduction
generation's resolution (again a disjunction, `ReductionFills ∨
ReductionAmbiguous`, so `P4` is not handed its conclusion), and §7's own
protocol steps. **Unfair, each for a stated reason:** `DeadlineBlows` (an
environment degradation), `PdSpotHolds` (the degradation that *creates*
`P3`'s antecedent), `PdTerminalFlatten` (a policy branch), `PdPlan` and
`Supersede` (decisions out of model), and **both compensating-trade
dispatches**.

**2. §8g's budget trap is answered by the properties' SUBJECT, not by an
antecedent — and that is the stronger answer.** §8g predicted `P1`–`P4`
would have to be conditioned on the budget the way model (b) conditions
liveness on `BOUND_CRASHES`, because budget exhaustion produces a `Settled`
stutter with a possibly-nonzero residual. They are not conditioned, because
**none of the four asserts that a dispatch ever happens** — leaving
`IssueExposureReduction` and `IssueFloor` out of the fairness set is what
makes that true. A behaviour that spends `MAX_DISPATCH` and stutters
forever with a naked residual falsifies nothing here. The property that
*would* have needed the conditioning is "the residual is eventually flat",
and it is deliberately not stated: it is not one of §4's four, and it would
have been a claim about the model's own bound rather than about the spec.
**`MAX_GEN` gets the same treatment**: `P3` is about the held generation's
own resolution and never mentions a later one, so a spent generation count
cannot read as a stall.

**3. `NC16` needed a STATE where slices A and B had an EDGE.** A reduction's
ambiguity was a choice between two transitions, never something the machine
sits in — so "it never blind-retries" named a behaviour the model could not
represent, and a re-send that changes nothing observable is a **stuttering
step**, which weak fairness steps straight over: `WF_vars(ReductionFills ∨
ReductionAmbiguous)` would simply have forced the fill and `P4` would have
passed with the defect installed. `pending[k].amb` is what turns the retry
into a fair lasso. It is gated on the constant, so the nominal graph is
untouched — **verified, not assumed: 6,729,250 / 832,179, slice B's figures
to the digit.**

**4. Each consequent is kept as narrow as its claim allows, so a control
names its own defect.** `P4` stops at "the floor generation leaves the
ledger" rather than conjoining "and the episode ends `Determined`" — the
conjunction would have made `NC13` fail `P4` as well, and a property two
controls can fail shows that *something* objected, not that *this* rule did.
That is CLAUDE.md's isolating-probe rule at model altitude. What the narrow
`P4` gives up is recovered by **composition**: what the floor routes into is
an order, and `P1` is the claim about orders. The one deliberate overlap is
`NC13`, which fails `P1` *and* `P2` because it removes re-classification on
the finality edge wholesale; `NC14` is the narrower control that isolates
`P2`, which is why both exist.

**5. Deadlock checking comes off in exactly two configs, and the asymmetry
is itself evidence.** `NC13`/`NC14` strand the machine — a frozen verdict
with every interval collapsed and budget left has no enabled action at all —
so TLC reports `Deadlock reached` during state generation and never runs the
liveness pass. Stranding is the point of those two, so the check comes off
there. It stays **on** in `NC15`/`NC16`, where nothing is stranded
(`PdTerminalFlatten` and `ReductionBlindRetry` stay enabled throughout): the
controls were designed to remove an *edge* rather than a state's successors,
precisely so the defect reports as the temporal violation it is instead of
as a deadlock, which would be TLC objecting to the shape of the control.

### 9b. Results

Toolchain unchanged: Java 25, `tla2tools.jar` pinned by SHA-256, TLC 2.19,
`-lncheck final`, `TLC_WORKERS=4` throughout (the capture was running).

| Run | Constant flipped | Predicted | Result |
|---|---|---|---|
| `_Live` (nominal) | — | pass, `P1`–`P4` + all seven invariants | **PASS** — 6,729,250 / **832,179 distinct**, depth 30, ~4 min |
| NC13 | `RECLASSIFY_ON_FINALITY = FALSE` | `P1` | **VIOLATED** — 7,101,314 / 930,385 |
| NC14 | `RECLASSIFY_AFTER_DEADLINE = FALSE` | `P2` | **VIOLATED** — 3,597,110 / 460,997 |
| NC15 | `HELD_RESUMES = FALSE` | `P3` | **VIOLATED** — 6,600,882 / **832,179** |
| NC16 | `BLIND_RETRY = TRUE` | `P4` | **VIOLATED** — 10,787,270 / 1,352,785 |
| W1 (NC12 constants) | — | `NoNonzeroOrphan` reached | **REACHED** |
| W2 (NC13 constants) | — | `NoStrandedNakedResidual` reached | **REACHED** |
| W3 (NC14 constants) | — | `NoStrandedNakedResidual` reached | **REACHED** |
| W4 (NC16 constants) | — | `NoStuckFloorSubtrahend` reached | **REACHED** |

Plus three positive controls for the liveness antecedents — `PC22`
(`Ambiguous`), `PC23` (`DeadlineExceeded`), `PC24` (a floor generation
**outstanding**, which is not the same state `PC1`'s dispatch *count*
names). `P3`'s antecedent was already `PC14`. All violated, i.e. reached.

**Every violation was read.** `NC13` ends with every interval collapsed,
`pdPerp = 0`, no generation running, a naked long of 1 — and
`cls = "Ambiguous"` for ever, so §2's determined branch is barred from
compensating a residual that is now exactly knowable. `NC15` parks at
`pd = "Held"` with `gens = 2`, an **active** `pkgspot` reservation of 1,
`pdDone = 1`, `pdSold = 0` and `budget = 1` left: the perp leg bought, the
spot leg never sold it back, and the reservation goes on subtracting from
every later sizing. `NC16` lassos on a **`floor`** entry of −1 against
`realized = −2`, cycling `amb` for ever.

### 9c. The control-witness probes — asking a control what its evidence shows

**`NC13`'s and `NC16`'s witnesses both land at `budget = 0`.** There the
machine could not have compensated anything even with a perfect classifier,
so those traces demonstrate the **stall** — which is all `P1` and `P4`
claim — while saying nothing about what the stall costs. That is exactly
the shape of §8b's `NC12` finding, where TLC's shortest orphan was an entry
of amount **zero**: a control can demonstrate a flag flipping while its
witness is degenerate for the harm. TLC reports *a* counterexample, not the
most harmful one, and the difference is not visible unless the trace is
read.

So the question was asked of all four controls. Three needed a probe; one
answered it in its own trace (`NC15`'s park is harmful on its face). The
probes are committed runs rather than a hand-check, and **`W1` is a
committed run of the check §8b performed by hand in session 66** — the
session that adds the mirrors is the session that owes the original, which
is CLAUDE.md's rule about fixes and their symmetric cases.

| Probe | Asks | Answer |
|---|---|---|
| `W1` | is `NC12`'s orphaned entry ever **nonzero**? | reached |
| `W2` | can `NC13` strand the machine with a **nonzero residual and budget left**? | reached — **2 steps from `Init`**: `realized = −1`, every interval collapsed, `cls = "Ambiguous"`, `budget = 3` (the *full* budget) |
| `W3` | the same of `NC14`'s latched `DeadlineExceeded` | reached |
| `W4` | can the determined branch be **about to size** while a floor generation stuck in the retry loop contributes to the subtraction? | reached — `realized = 2`, a stuck `floor` entry of 1, `budget = 2`: the branch sizes `2 − 1 = 1` and compensates half of a naked long |

`W3` was run rather than inferred from `W2` on purpose: the two controls
prune the state graph very differently (930,385 vs 460,997 distinct states),
and "the harmful state is reachable over there" is not evidence that it is
reachable here.

### 9d. Findings from the build

**`NC15` produces a state graph IDENTICAL to the nominal one — 832,179
distinct states, to the digit.** The argument, not just the coincidence:
the control only *removes* a transition (`PdSpotFinal` from `Held`), and
removing transitions can only shrink the reachable set, so NC15's states are
a **subset** of the nominal ones — and an equal-cardinality subset of a
finite set is the set. (The generated counts do differ, 6,600,882 vs
6,729,250, which is the removed edge showing up exactly where it should: in
the transitions explored, not in the states reached.) The consequence is the strongest available
statement of why this slice exists: **a machine that parks a generation for
ever, holding an unsold net-long transient and a reservation that subtracts
from every later sizing, is a machine whose reachable states are exactly the
correct machine's.** Every invariant in this module — all seven, plus
`TypeOK` — is a state predicate, so every one of them is still green on it.
No safety property could have caught it, and that is not a claim about which
invariants were chosen; it is arithmetic about the state set.

**`NC13` doubles as the control on the fairness set's own omissions, and
this was the reason for leaving `DeadlineBlows` unfair.** `DeadlineBlows` is
enabled in *every* `Ambiguous` state and its target satisfies `P1`'s
consequent, so a fair `DeadlineBlows` would discharge `P1` by assuming the
very degradation `P1` exists to survive — and `NC13`'s stall would be
"rescued" by the deadline blowing, leaving `P1` green with the defect
installed. `NC13` failing is therefore evidence about the fairness set and
not only about the classifier. `NC15` is the same check on
`PdTerminalFlatten`: fair, it would discharge `P3` by itself.

**None of the four properties is satisfiable in its own antecedent state** —
checked by inspection before the runs, because it is the model-altitude form
of the fixture-collapse tell. `cls = "Ambiguous"` is not in `P1`'s
consequent set, `DeadlineExceeded ≠ Determined`, `Held ∉ {Complete,
TerminalFlatten}`, and `FloorOutstanding` contradicts `¬FloorOutstanding`.
A property whose consequent already holds where its antecedent does is
green, vacuous and indistinguishable from a real one.

### 9e. Stated bounds and abstractions

- **Two of the four properties differ from §4's wording, and both
  differences are marked rather than absorbed** — §4 is this document's own
  normative text for the model, so a silent divergence between the table in
  §4 and the module is the design-record equivalent of spec drift.
  - **`P2` is checked in a STRONGER form than §4 states.** §4 says "a
    `DeadlineExceeded` episode **whose intervals later collapse**
    eventually reaches `Determined`"; the module drops the qualifier and
    claims it of *every* deadline-exceeded episode. That is sound here
    because the fairness set makes the qualifier automatic — the finality
    barrier is fair, so the intervals always do collapse — and the stronger
    claim is the one worth checking. It passes.
  - **`P4` is checked in a NARROWER form than §4 states.** §4 says "…
    eventually resolves through the same contract — i.e. **the recursion
    terminates**"; the module claims only that the floor generation leaves
    the ledger. Two reasons, both above: the finitely-many-floors reading is
    guaranteed by `MAX_DISPATCH` rather than by the spec, and the wider
    consequent would have been failed by `NC13` as well as `NC16`, leaving
    neither control naming its own defect. The give-up is recovered by
    composition with `P1`.
- **Fairness is a premise, not a fact.** `P1`–`P4` are claimed only for
  behaviours satisfying `Fairness`; the module argues each conjunct and,
  more importantly, each omission. Model (b)'s `EnvStabilizes` antecedent is
  the same discipline expressed as an implication instead.
- **"The recursion terminates" in the finitely-many-floors sense is
  guaranteed here by `MAX_DISPATCH` — by the model's own bound, not by the
  spec.** So a control against *that* reading could not exist, and a
  property stating it would be evidence about the bound. `P4` checks the
  claim the bound does not supply: each floor generation individually
  reaches finality. `NC16`'s retry spends no budget precisely so the bound
  cannot terminate the loop and hide the defect.
- **The liveness config's state graph is the safety config's.** Fairness
  constrains behaviours, never reachability. That is what lets the 24
  positive controls, all run against the safety constants, stand as the
  non-vacuity evidence for the liveness antecedents.
- **`CHECK_DEADLOCK FALSE` in `NC13`/`NC14` and in `W1`–`W3`**, for the
  reason in §9a decision 5. It stays on everywhere else, including every
  nominal config.
- **`Held/Alert`'s exit is still the single barrier §8f describes.** `P3`
  says the park is left; it does not say the remaining per-leg target is
  re-driven, which §8f already records as unmodeled.

### 9f. What slice D inherits

Slice D is the companion artifact `docs/formal/ambiguous-order-model.md`:
the correspondence map (spec clause ↔ model construct), the results table,
the honesty notes and a how-to-run, with every clause in §2's and §7's
Invariants sections either mapped or listed as deliberately unmodeled with a
reason. Three things to carry into it:

- **`FM-5`'s normative edit is still OWED** and still scheduled for slice E
  (§5's roster). The wording is in §8d. Until it lands the two artifacts
  disagree and CLAUDE.md's rule decides.
- **The `W*` probes are a pattern, not four files.** Any control whose
  witness TLC chose deserves the question "does this trace show the harm or
  the flag?", and the answer belongs in a committed run. **Its own mirror is
  therefore owed and is named here rather than assumed done: model (b)'s
  five controls have never been asked it.** That is not slice D's work —
  it belongs with slice E's adversarial round, where model (b) is in scope
  anyway — but the debt is recorded now, because the session that
  generalizes a check is the session that can see where else it applies.
- **`run.sh` is now 55 runs** (8 for model (b), 47 for model (a)), and every
  model-(a) config tag is inside the `AmbiguousOrder_` namespace so
  `./run.sh AmbiguousOrder` selects all 47 — the session-66 lesson, which
  slice C re-armed by adding nine configs.

---

## 10. Slice D — BUILT (2026-08-02, session 68)

`docs/formal/ambiguous-order-model.md`: the correspondence map, the full
results, the honesty notes and the how-to-run. **The done-when is met** — all
nine clauses of SM §2's Invariants section and all nine of SM §7's appear in
the two Invariants tables, each either mapped to a construct or listed
UNMODELED with a reason and a named owner. §2's Scope/States/Transitions and
§7's States/Transitions have their own tables above them.

### 10a. What the map surfaced that the build record did not

**A correspondence gap in `Held/Alert`, and it is the map's own justification.**
SM §7's transition row enters `Held/Alert` from **`PerpReduceInFlight` or
`SpotReduceInFlight`**; the model has only `PdSpotHolds`, so **the perp leg
cannot hold**. Slices B and C never noticed, and could not have: no invariant
and no property is *about* the set of ways a state is entered, so nothing in
the module reports a missing entry edge. Only a clause-by-clause reading does.

It is defensible and it is recorded as a narrowing rather than absorbed: `Held`
matters here because of what a held generation *holds*, and at the perp leg
neither the unsold transient nor a nonzero reservation exists yet (`pdDone = 0`
and the `"pkgspot"` entry carries the outstanding net intent, which is zero
until the perp confirms). So the perp-side park is the harmless instance and
`P3`/`NC15` are aimed at the one with content. **Carried to slice E**: whether
a perp-side park has a hazard that reasoning misses — a generation stuck before
its reservation grows still blocks the next one on §7's own "only after
finality" gate, which is a liveness question no property here states.

**The general form is worth keeping.** Invariants catch bad behaviours *inside*
the model's world; **nothing inside a model can report that a spec clause never
entered it**. That is why slice D's done-when is a totality condition over the
*spec's* clause list rather than over the model's constructs — the direction of
the map is what makes it capable of finding an omission, and it found one on
its first pass.

### 10b. Two claims the artifact refuses to make

Both are places where a green model would otherwise read as evidence:

- **"A pre-authorized panic-MARKET does not bypass the finality barrier"** is
  listed UNMODELED *even though the model contains no bypass edge*. The absence
  of a modeled bypass is not evidence that the implementation has none — a rule
  of the form "there is no other path" cannot be checked by a model that
  contains only the paths its author wrote. Writing "modeled: no such edge
  exists" would have been the most comfortable line in the document and the
  least honest.
- **"Never a weaker cancel-ack-only variant"** is an abstraction boundary, not
  a check: the four-step contract collapses to one barrier, so the model can
  tell whether a barrier was *waited for* and cannot tell a correct barrier
  from a weak one.

### 10c. Also this session

`CLAUDE.md` said `docs/formal/` holds *a* model of the fence/takeover
composition and that it found **four** spec defects. Both were stale — two
models, five defects. Corrected, with the pointer that the companion artifact
is the entry point rather than the `.tla`.

---

## 11. Slice E — the adversarial round (2026-08-02, session 69)

Slice E's docket was assembled at the close of slice D and is worked in order.
Item status is recorded as each closes, so an interrupted session resumes at the
first open item rather than re-deriving the list.

### 11a. Item 1 — `FM-5`'s normative edit: **LANDED (v0.9.24)**

The edit is FLAG-3's *silence*, not its words. What the ruling left
under-determined is the reservation's **amount**; the entry's existence at
creation is kept exactly as ruled. Every site now says the entry carries the
generation's **outstanding net intent** — zero before the perp leg has bought
anything, amended as it confirms, back toward zero as the spot leg sells.

**The footprint is wider than §8d prescribed, and deliberately so.** §8d named
two sites ("§7's FLAG-3 ruling, and beside §2's reservation-aware sizing").
Landing only those two would have left the unsound reading **still literally
available** in the doc: SM §7's `Planned` row and its "reserved second leg vs
the §2 classifier" invariant both say the transient is "atomically reserved from
creation" with **no amount attached**, which is precisely the underdetermination
`FM-5` exploits. A reader reaching either one first would re-derive the defect
from text that had just been declared fixed. Sites edited:

| Doc | Site | What it now says |
|---|---|---|
| `build-spec.md` | §6 step 7, `Determined residual` | the subtraction is unconditional, **therefore** every entry MUST be backed at the instant it is counted; entry carries outstanding net intent, never target |
| `state-machines.md` | §7 `Planned` row | amount is **zero** at creation; amended as the perp leg confirms |
| `state-machines.md` | §7 `SpotReduceAuthorized` row | what is first *fully amounted* here is the amount, the perp reduction now being confirmed |
| `state-machines.md` | §7 `PerpReduceFinal →` action | the amount subtracted tracks confirmed fills and is zero before any |
| `state-machines.md` | §7 Invariants | **new invariant**: "the reservation is exactly backed at every instant" |
| `state-machines.md` | §7 FLAG-3 ruling | amended — words kept, silence closed, `NC7` named |
| `state-machines.md` | §2 Reservation-aware sizing | the subtraction is unconditional, which is *why* entries must be backed; the asymmetry stated |
| `adr-history.md` | §2 review records, §3 dated block + FM-5 row | model (a)'s companion artifact is its review record |
| `docs/formal/ambiguous-order-model.md` | §1(1), §2 status | OWED → LANDED; the two artifacts no longer disagree |

**The reading pass caught the edit over-generalizing — a second time the same
trap, one level in.** The first wording said an entry carries "the reserving
generation's outstanding net intent — **zero before its counterpart leg has
moved anything**". That is exactly right for a `PackageDelever` generation and
**wrong as a general rule**, because the ledger has **three** producers and only
one of them has a counterpart leg. An in-flight `IntervalFloorReduction` *is*
its own reduction: its order is dispatched and genuinely working, so its entry
is its **full dispatched size** — zeroing it restores precisely the re-read of a
still-in-flight floor as fresh naked exposure that the reservation exists to
prevent. An `Armed` protective order carries its reserved leg allocation.

The general statement, which all three satisfy: **an entry is the exposure
change its reserving action has already COMMITTED and not yet REALIZED.** The
producers differ only in *when* commitment happens — at perp confirmation for a
generation, at dispatch for a floor, at arming for a protective order. Both
normative sites now say this, and both name all three producers rather than
generalizing from the one the finding came from.

Worth stating plainly, because it is the finding's own lesson recurring inside
its fix: **`FM-5` is a defect of underdetermined wording, and the first fix
introduced a different underdetermination.** A rule derived from one instance
reads as universal unless the other instances are named. The marker grep could
never have seen this — nothing was mismarked — which is what the reading pass
beside it is for.

**Set-version alignment to v0.9.24** across all five documents.
`state-machines.md` is **normatively changed for the first time since v0.9.4** —
every version between was set-version alignment on that doc. The ADR §3 tagless
table is untouched, so the tagless total **stays fifty-four** (`FM-5` is tagged,
so it lands in a Finding → resolution table).

**A drift found while bumping the versions, and fixed:** `adr-history.md`'s own
Status line was **two versions behind** (v0.9.21) — v0.9.22 and v0.9.23 each
added a dated §3 block and neither bumped it. Same class as session 64's HANDOFF
entry-point drift: the *body* of the file was updated and the line readers key
off was not, so the file misreported its own currency while being correct
everywhere else. The convention is now stated in the file: a change that adds a
§3 block bumps the Status line **in the same edit**.

### 11b. Item 5 — the perp-side park: built, and it found `FM-6`

Docket item 5 was the narrowing §10a recorded: SM §7's transition row enters
`Held/Alert` from **`PerpReduceInFlight` or `SpotReduceInFlight`**, and slices
A–D modeled only the second. It was carried as a question about whether the
perp-side park hides a hazard. It does, and not the one the docket expected.

**First: §10a's stated reason for the narrowing is wrong, and the model says so
before any run.** §10a justified it thus — "at the perp leg neither the unsold
transient nor a nonzero reservation exists yet (`pdDone = 0`, and the `pkgspot`
entry carries the outstanding net intent, which is zero until the perp
confirms)". But `PdPerpFill` fires *while* `pd = "PerpReduceInFlight"` and
**amends the ledger entry upward**, so §2's own `PartiallyFilled` case —
"explicitly means more fills possible" — puts a park *after* a partial fill.
Such a park holds a **real net-long transient and a nonzero reservation**.
`pdDone = 0` describes the park taken before any fill and no other.

Probe **`PC26_PerpParkAlwaysEmpty`** asks exactly this — is a perp-side park
with `pdDone > 0` and `PdReserved > 0` reachable? — and it is **VIOLATED**, so
the harmful shape is reachable. The narrowing's *conclusion* may still have been
tolerable; its *argument* was not, and a recorded justification is what a later
session relies on instead of re-deriving. **`PC25_PdNeverHeldPerp`** is the
plain antecedent probe (the park is reached at all) and is also violated.

**Then the park was built, and the nominal model DEADLOCKED.** That is the
finding, and the work was telling which of three things it was: an unfaithful
new transition, an artifact of a stated abstraction, or a real gap. Two
experiments settled it.

1. **Is it new?** HEAD's nominal is green at **6,729,250 / 832,179** — the
   recorded figure, to the digit. So the dead end is slice E's.
2. **Which edit?** With `PdPerpHolds` removed from `Next` and *every other*
   slice-E edit in place, the model reproduces **6,729,250 / 832,179** again.
   So the widened `PdPerpFinal` / `PdTerminalFlatten` from-sets, `INV8` and the
   new constant are behaviour-neutral, and the park alone is the cause.
3. **Is it the `TerminalFlatten` abstraction?** §8f states that the flatten's
   own orders are not modeled, which leaves the transient orphaned — the
   obvious suspect. So the flatten was made to actually flatten
   (`realized' = realized - (pdDone - pdSold)`). **It still deadlocks.** Not
   the abstraction.

#### `FM-6` — a verdict goes stale because the ledger changed and no order did

The dead end's state, and every disjunct of `Next` is disabled in it:

| Fact | Consequence |
|---|---|
| every counter-leg interval has **collapsed** (`q = 0` throughout) | the residual is determined in §2's own sense |
| the verdict is still **`DeadlineExceeded`** | the last thing that could re-run the classifier was an *order* event |
| `floorIssued` is spent (`FLOOR_ONCE`) | the one action that verdict permits is gone |
| the residual is **nonzero** and `budget` is **not** spent | `Settled` needs one or the other; `IssueExposureReduction` and `PdPlan` need `Determined` |

So: a **determined naked residual, dispatch budget in hand, and nothing
enabled** — the package holds with an alert and no admissible action, for ever.

**The defect.** §2's residual is
`realized_delta − outstanding_reserved_reductions`, and re-classification is
triggered by **order** events. But the **subtrahend** changes the verdict just
as the minuend does, and **a `PackageDelever` generation ending changes the
subtrahend without producing an order event.** Every other path out of a
generation happens to run through `PerpReduceFinal`, which reclassifies — so
the gap was invisible. The perp-side park is the *unique* route that leaves a
generation without it. **The park does not cause the defect; it reveals one
that was always reachable in the spec and was not reachable in the model.**

**It is `FM-5`'s sibling, and the pair is worth naming together:** FM-5 is *the
ledger must be backed at the instant it is read*; FM-6 is *a change to the
ledger must be observed*. Both are the same underlying asymmetry — §2 treats
`outstanding_reserved_reductions` as an input it can trust and never re-derives
it, so the ledger's producers can put it wrong (FM-5) or move it silently
(FM-6), and in neither case does the reader notice.

**The resolution, and it is as narrow as FM-5's:** a change to
`outstanding_reserved_reductions` is **itself a re-classification trigger**,
exactly as an order event is. The model takes the sound reading nominally
(`RECLASSIFY_ON_LEDGER_CHANGE`, applied at `PdEnds`, where the generation and
its entry are definitively gone); the literal reading is **`NC19`**.

**`NC19`'s signature is a DEADLOCK, and the harness gained a verdict for it.**
`run.sh` had `pass` / `fail` / `failprop` / `probe`; a deadlock would have been
scored by `fail` as an unexplained non-zero exit. It is now its own `deadlock`
verdict, for the reason NC15's config already draws out — a control that
*stranded* the model and a control that *caught* something are different
claims, and a harness that accepts either lets the first read as the second.
Here the dead end **is** the finding, so it is named rather than tolerated.

**Status: `FM-6`'s normative edit is OWED**, and it is not covered by v0.9.24's
FM-5 edit. Wording and sites are drafted in §11c below.

#### Also in item 5: model (b)'s `Cx8-2` asked of model (a) for the first time

Adding a second park raised a question this repo has already paid for once.
Model (b)'s `Cx8-2` was "`HeldIncident` is two states wearing one name — SM §4
gives the readiness park and the budget-exhaustion park *different* exits, and
`ActHeldReentry`'s cause-blind guard exited both." SM §7 has the identical
shape, and **model (a) had never been asked**, because until slice E it had
only one park. This is the repo's "a fix applied in one place gets applied to
its mirror" rule at model altitude.

So `HeldPerp` is a **distinct state**, not a second way into `Held`, and
`NC17` (`HELD_EXIT_CAUSE_BLIND`) is the cause-blind exit made explicit: the
reserved **spot** leg's finality barrier discharging a park whose **perp** leg
is the unresolved one. It is reachable rather than absurd precisely because of
FLAG-3 — the `pkgspot` entry exists from creation, so the guard passes.

Its invariant is **`INV8_CompleteMeansLegsFinal`** (`pd = "Complete" =>
pdPerp = 0`), and the phrasing was chosen for **isolation**, which is the part
worth checking rather than assuming. The tempting phrasing is over the
*authorization* — but `NC9` (`PERP_FIRST = FALSE`) also produces a spot sale
with no confirmed perp reduction, so that invariant would be violated by both
controls and would show that *something* objected rather than that *this rule*
did. `INV8` stays green under `NC9` (there the perp leg was never submitted, so
`pdPerp` is 0) and is violated only by a park exited through the wrong leg's
barrier.

`P5_HeldPerpGenerationResumes` is FLAG-4 applied to the new park, and it ships
with its own control in the same slice per §4's rule — **`NC18`**, which is
`NC15`'s constant flip with `P5` listed alone. That it is the *same* constant is
the point: FLAG-4's ruling is about the **state**, which §7 enters from either
leg, so one flip removes the resume edge from both parks. `NC15` shows `P3`
bites; `NC18` shows `P5` does. A property whose *sibling* fails is an argument,
not evidence.

### 11c. `FM-6`'s normative edit — the spec's own wording names the defect

The clause is SM §2's **Scope** (core §6 step 7 is the normative owner):

> **Classification is a function of two inputs and re-runs when either
> changes `[→ R1d-3]`:** fills/position events move the *realized* delta;
> order-lifecycle/finality events … collapse an order's *possible-delta
> interval* …

**Two inputs.** But the quantity the classifier gates on is
`realized_delta − outstanding_reserved_reductions`, which has **three** —
and the third is the one the re-run rule does not name. That is `FM-6` stated
in the spec's own vocabulary: the ledger is an input to the classification and
is absent from the list of things that re-trigger it.

The producers move it on events that are **not order events** — a generation
opening, its perp leg confirming (an amendment, per `FM-5`), a generation
ending, a supersession, a floor retiring. Each changes the residual. The one
the model reached is the generation ending, because that is the only one the
perp-side park can arrive at without passing `PerpReduceFinal`.

**Wording to land** (§6 step 7, mirrored in SM §2's Scope):

> Classification is a function of **three** inputs and re-runs when **any**
> changes: fills/position events move the *realized* delta; order-lifecycle /
> finality events collapse an order's *possible-delta interval*; and **a
> change to `outstanding_reserved_reductions` moves the subtrahend** — an
> entry created, amended or retired by any of its producers (a
> `PackageDelever` generation opening, amending as its perp leg confirms,
> completing, terminal-flattening or being superseded; an
> `IntervalFloorReduction` retiring; a protective order's leg reservation
> released). The machine MUST NOT wait for an order event when the ledger
> alone changed the residual, for the same reason it must not wait for the
> next fill when order state alone became conclusive: **a generation ending
> emits no fill and no order event, yet moves the residual by the whole
> reserved amount.**

The parallel in the last sentence is deliberate — §2 already carries exactly
this argument for the cancel-with-zero-executions case ("a cancel confirmed
with **zero** executions emits no fill and no position event, yet fully
collapses that order's interval"). `FM-6` is that same sentence owed to the
third input.

**Status: OWED.** It is landed in the same session it was found, below
§11d — the slice-close rule ("land it in the spec before the slice closes"),
and the situation it avoids is the one `FM-5` sat in for three slices, where
`docs/formal/` and `docs/state-machines.md` disagreed and CLAUDE.md's standing
rule had to arbitrate.

### 11d. Item 2 — model (b)'s five controls, asked the witness question

Model (a) has asked its controls since slice C whether their counterexamples
demonstrate the **harm** or only the **flag** (`W1`–`W4`, §9c). Model (b)'s five
had never been asked, and `NC1`–`NC4` predate the question entirely. Six probes
were built (`FenceTakeover_W1`–`W6`), and **all six were REACHED**: every
control's stated harm is real, and TLC had simply been reporting a shallower
witness. Model (b) is now 14 runs.

**The numbering restarts per model on purpose.** `W1`–`W6` here are model (b)'s
and are selected by the `FenceTakeover_` prefix, exactly as both models already
carry an `NC1`–`NC5`. The prefix is the namespace, and it is the thing
`./run.sh FenceTakeover` filters on — the lesson the coverage probes paid for
when `PC_*` sat outside the namespace its own model owned.

**The degeneracy was systematic, not four accidents, and the mechanism is
general.** Every invariant in model (b) can be tripped two ways: by a cheap
**surrogate** for exposure — an order sitting live, unfilled — or by exposure
itself. The surrogate is always exactly one `VEntryFill` closer to `Init`, so
breadth-first search returns it every time. Three of the four safety witnesses
end `pos = "flat"` with `hadExposure = FALSE`.

| Control | Its witness showed | Its probe reached |
|---|---|---|
| `NC1` | bot write post-latch, actuator in `quiesce`, **no** actuator order live, package flat and never exposed | `W1` — the same write with live exposure |
| `NC2` | tombstone over a **flat** package, via `BotDeactivate`'s `pos = "flat"` disjunct — **not** the `botCloseLive` one that *is* the issuance-gated bug | `W2` at 5 states: NC2's own trace **+1 fill** |
| `NC3` | real exposure, actuator close in flight, one writer | `W3` at 8 states: **both** writers landed, **both** close orders live |
| `NC4` | FM-1's route exactly, then the re-admitted entry **never fills** | `W4` (exposure back, coverage intact) and `W6` at 12 states — NC4's trace **+1 fill** |
| `NC5` | **already harmful**: lassos un-arbitrated with `neutral` + `hadExposure` | `W5` at 6 states: the same strand on a **naked** package |

**`NC3` is the exception and not by luck**, which is the transferable part:
`ActCanaryUnfencedClose` carries `pos ≠ "flat"` in its own guard, so no witness
of it can be flat. **A control whose harmful precondition is a *guard of the
action it enables* cannot have a degenerate witness in that dimension.** That is
the structural property the other three lack, and it is a design lever — a
control built on an action whose guard *is* the harm needs no witness probe.

**Two suite defects this surfaced, neither visible from a green run.**

**(1) `I2_HostIndependentCoverage` had never been violated by any run in the
suite.** It is listed in exactly two configs, `NC2` and `NC4`, and **both report
`I3`** — because TLC halts at the first violation, so a shallower `I3` violation
*ends the search before the `I2` state exists to be found*. The configs' own
"I2 or I3" wording is what made it invisible: an expectation phrased as a
disjunction cannot notice that one disjunct never fires. `W2` and `W6` now
demonstrate it under both controls. **The rule this yields:** a config listing
more than one invariant is not checking both — it is checking whichever is
shallower, and the other is decoration until something reaches it. Every other
config in both models lists exactly one for *attribution*; this is the same
discipline owed for a second reason, **search depth**, and the probes therefore
list one invariant and none of `I1`–`I8`.

**(2) This artifact's prose carried harm its runs never showed.**
`fence-takeover-model.md` §1 said NC2's "entry *then fills* into a coverage
hole"; §2's FM-1 trace said "→ I3, **then I2**". `NC4` never violates `I2`, and
both traces stop before any fill. **The claims were true about the system** —
`W6` proves it at depth 12 — **and that is precisely why four review rounds
passed over them.** A false claim gets challenged; a true claim resting on a run
that did not show it reads as verified. Both sentences are now scoped to what
their run reports, with the probe supplying the rest named beside them, and the
correction is marked in place rather than made quietly.

**What separates every degenerate witness from its harm is a fill** — the one
event in this system no component gets to decline. A control that stops at "the
flag flipped" is reporting a state the venue can convert into the real damage
unilaterally. That is the argument for why *shortest counterexample* and
*adequate evidence* are different things.

**And one prediction was wrong, which is the reason these are runs.** `W2` was
written claiming the tombstone-over-exposure state was reachable *only* through
`BotDeactivate`'s `botCloseLive` disjunct. It is not: the witness tombstones a
**flat** package whose entry is still live, then lets the entry fill; the
`botCloseLive` disjunct is not exercised even by the probe. What that yields is
better than the prediction was: **`I3`'s `NoLiveOrders` conjunct is not tidiness
beside `I2`, it forbids the *precursor* of an `I2` violation.** A terminal
declared while any order is live is one uncontrolled venue event away from
exposure under a tombstoned plan. `I2` is not decoration — `I3` is what keeps it
green. **`W6` likewise exists only because `W4`'s trace was read**: `W4` was
REACHED and showed itself to be half of NC4's claim (exposure re-admitted,
`plan` still `"active"`). A probe being violated says the state is reachable;
only the trace says whether it is the harm the control names.

### 11e. Item 3 — §7d's "never issued twice", and `FM-7`

**§7d's reading was taken off a truncated search, and item 2 is what exposed
it.** §7d reported that `NC2`'s counterexample leaves `overshoot` FALSE
throughout, concluding the flat rule looked like belt-and-braces over the
reservation. But `NC2` lists `TypeOK` and `INV2` only, and **`INV2` is exactly
`floorCount <= 1`** — so TLC halts at the first second-dispatch state and
explores nothing beyond it. `overshoot` was not merely FALSE there; **it was
never a checked invariant in that run at all.** "No overshoot was reported" and
"no overshoot exists" are different claims, and only the second answers §7d.
This is item 2's defect arriving in model (a), which is why the mirror rule is a
standing one.

**`NC20` is the honest experiment**: `NC2`'s constants exactly, `INV2` removed,
`INV1_NeverOvershootSign` listed alone. **Non-vacuity cost no run** — since
`INV2` *is* `floorCount <= 1`, `NC2` violating it already proves the
double-issuance state is reached under these same constants, so a pass here is a
real answer and not an unreached branch.

**Result: PASS, over the complete graph — 8,770,801 generated, 1,086,199
distinct, 0 left on queue.** Double issuance cannot overshoot the sign while
reservation-aware sizing is working. §7d's reading survives a full search.

**`FM-7` — a rule whose stated justification assumes what the adjacent sentence
forbids.** SM §2's interval-floor rule says the floor is "never issued twice
(else, for `[0.4, 1.0]`, two uncertain `0.4` reductions could reach `-0.4` and
defeat the proof)". That counterexample **requires the reservation not to be
counted**, and the neighbouring sentence requires it to be. So the offered
justification is unsound *even though the rule is right*: `NC20` says the harm
it describes cannot occur under the sizing the spec mandates.

**The rule stays. The justification is what is defective**, and the docket said
so before the run: *do not resolve it by weakening the spec*. The danger is
specific and this project has already been bitten by it once this slice —
**nobody re-derives a recorded justification** (§10a, and
[[a-rejected-alternative-is-an-unchecked-claim]]). A later reader who checks the
`[0.4, 1.0]` story against reservation-aware sizing will find it does not hold,
and the natural next move is to drop a rule whose reason has evaporated. The
rule must therefore carry a reason that is *true*.

**Wording to land** (SM §2's interval-floor rule, replacing the parenthetical):

> …and is **never issued twice**. This is **defence in depth, deliberately
> retained**: under reservation-aware sizing a second floor cannot reach the
> opposite sign, because the first is already netted out of the subtrahend —
> formally checked, `NC20`, full graph, no violation. The rule is kept because
> the proof it protects rests on *two* independent mechanisms and the
> single-issue rule is the one that survives a defect in the other: if the
> reservation is ever not counted — the `FM-5` failure mode, an entry that is
> not backed at the instant it is read — a second uncertain reduction *does*
> reach the opposite sign. The historical parenthetical (`[0.4, 1.0]`, two
> uncertain `0.4` reductions reaching `-0.4`) describes exactly that world and
> is retained **as the statement of what this rule protects against**, not as a
> hazard reachable under the sizing this document mandates.

That wording does the thing the finding is about: it keeps the rule, makes its
reason true, and names the *other* mechanism whose failure the rule covers — so
the two rules are visibly a pair rather than one looking redundant beside the
other.

**Re-measured in slice F (v0.9.26).** The model changed, so `NC20` was re-run
with it: **still PASSES over the complete graph — 8,948,671 generated /
1,093,953 distinct / 0 left on queue.** The figures above are left as slice E
measured them, because they record what that release checked; the point is that
**the finding's evidence is re-established by every suite run**, which is what a
kept control buys over a recorded conclusion.

### 11f. Item 4 — §8f's neutrality question, and `FM-8`

§8f recorded a **reading**: `PdPlan` guards on `TrueResidual = 0`, and
`TrueResidual` is `realized − TrueReserved` — **reservation-netted** — so the
guard admits a package neutral *pending an in-flight reduction* rather than
physically flat, while core §5.3 says `PackageDelever` reduces leverage "on a
**still-neutral** package" and §7's invariants say "the package has **no naked
residual**". Two probes, asked in that order because a negative on the first
would have closed the item by itself.

**`PC27` — REACHED at depth 3.** `IssueExposureReduction` → `PdPlan`, ending
`realized = −2`, `pd = "Planned"`, both legs zero. The generation opened on a
package that is **physically short 2**, exactly offset by somebody else's
reserved reduction. So the two readings are **distinguishable on this model**,
and four slices of green runs were not silently checking the strong one. (Had
`PC27` passed, that would have been the model-altitude form of the fixture
collapse — a guard whose two readings are the same predicate because nothing
ever varies the parameter that separates them.)

**`PC28` — REACHED at depth 4, and this is `FM-8`.** One step further:
`ReductionAmbiguous`. The offsetting reduction's **own result comes back
ambiguous**, its ledger entry is released, and the state is `realized = −1`,
`TrueReserved = 0`, `cls = "Ambiguous"`, `pd = "Planned"` — **a live generation
over a package that is physically short and no longer even determined, with
nothing reserved against it.**

**One correction to the landed wording, caught by re-reading the witness after
the edit — and it is `FM-5`'s fix-mistake a third time.** The first draft of
the new §7 invariant said the reservation can be released "when an
`IntervalFloorReduction`'s own result comes back ambiguous". `PC28`'s witness
uses an ordinary **`ExposureReduction`** (`Pend(amt, "exposure", TRUE)`), and
`ReductionAmbiguous` guards on `kind # "pkgspot"` — so **every** reduction
owning a ledger entry can release un-executed, and only §7's own spot leg is
excluded. Naming one producer would have repeated exactly what §11a records
about `FM-5`'s own fix: a rule derived from one of three producers reads as
universal. The clause now states the general case and names the witness's
actual producer.

**It is not a model artifact**, which was checked before the finding was
written: `ReductionAmbiguous` is faithful to SM §2 row 4 — "it never
blind-retries — its own ambiguous result routes through this same §6.7 finality
contract" — and the spec **itself** releases the reservation there, the exposure
re-entering as an unresolved order contributing an interval. The release is
correct. What is missing is anything that tells the *generation*.

**`FM-8` is `FM-5`'s mirror, as §8f predicted it would be.** `FM-5`: an entry
counted **before its backing exists**. `FM-8`: an entry's **premise outliving
the entry**. Both are §7 and §2 disagreeing about what the shared ledger means,
and both are invisible to every invariant in the module — **`INV5` polices the
generation's own legs (`pdSold <= pdDone`), never the package premise it was
opened under**, which is why a green nominal run says nothing here and why this
is a probe rather than an invariant flip.

**The deeper half, which §8f did not ask.** §8f asked *which neutrality*. `PC28`
says the more consequential question is *when it is checked*: §7's "no naked
residual" is enforced **once, as an entry guard**, and nothing re-evaluates it.
Even under the strong (physical) reading, a precondition checked only at entry
can be falsified the instant afterwards. Tightening `PdPlan` to `realized = 0`
would refuse `PC27`'s witness — but it is a fix to the *guard*, and the defect
is that the guard is the only enforcement.

**Status: RULED by the user, 2026-08-02 — candidate 1, the standing
precondition. The normative edit is LANDED in v0.9.25; the MODEL ENCODING AND
ITS CONTROL ARE OWED (slice F).** **→ DONE in §12, and note before reading on:
the trigger as worded below ("the §2 classifier leaving `Determined`") is
`FM-9` — encoding it showed it names a quantity the generation itself moves,
and v0.9.26 restates it net of the generation's own legs. The RULING stands
exactly as taken; only its trigger quantity changed.** That split is deliberate and is stated so a
later session does not mistake one for the other: the *ruling* is a design
decision and belongs in the spec the moment it is made, but **the run that
shows the hole closes has not been performed**, and this slice's own repeated
lesson is that a fix argued is not a fix checked. What landed in SM §7:

- **a third supersession row** — the §2 classifier leaving `Determined` for the
  hedge group supersedes an active generation, quiescing the reserved second
  leg to §6.7 finality first, exactly as the other two do;
- **a new invariant** stating that "no naked residual" is a **standing**
  condition and not only an entry test, with the reason the *stronger entry
  guard would not have sufficed* (candidate 2 below) written into it;
- **the existing quiesce invariant widened to all three paths** rather than
  given a fourth of its own — the obligation is identical, so a separate rule
  would be a second place for it to drift.

**What slice F owes**, sketched here so it is not re-derived: a
`PREMISE_IS_STANDING` constant (nominal TRUE) gating a `Supersede("premise")`
enabled on `PdActive /\ cls # "Determined"`; `PC28` **flips polarity** — it
becomes an invariant of the fixed model rather than a positive control, and
`NC21` (`PREMISE_IS_STANDING = FALSE`) is the control that must violate it.
Note the encoding subtlety before writing it: the harm is not the state
existing *transiently* — a legal quiesce window contains it — so the invariant
must be read from a flag written at the event, which is exactly the shape
`INV6_SupersessionQuiescesFirst` already uses (`supersededWithLiveLeg`, read
from the flag, never from the guard). Reusing `Supersede(path)` unmodified is
what makes the ruling cheap to encode; that it *is* cheap is evidence for the
ruling, not proof of it.

**The two candidates as they stood, kept because a rejected alternative is an
unchecked claim** ([[a-rejected-alternative-is-an-unchecked-claim]]):

1. **Standing precondition + existing machinery.** The classifier leaving
   `Determined` while a generation is live becomes a **supersession trigger**,
   quiesced to §6.7 finality exactly as §7's other two are (`EmergencyClosePackage`,
   `FenceReadyLost`) and policed by the `INV6` that already exists. Reuses §7's
   own mechanism; costs a generation that would often have been fine.
2. **Physical neutrality at entry.** `PdPlan` requires `realized = 0`, not
   `TrueResidual = 0`. Smaller edit, closes `PC27`'s route, and does **not**
   close the general case — a floor issued after the generation opens can still
   move the package underneath it.

Candidate 1 is the stronger reading and the one that answers what `PC28` found;
candidate 2 answers only what §8f asked. **Candidate 1 was ruled.** The
argument that decided it is worth keeping, because it is the reason candidate 2
looked adequate and is not: **candidate 2 refuses `PC27`'s witness and leaves
the defect** — a generation may open on a genuinely physically-flat package and
have a floor issued underneath it a step later, at which point the same
dissolution occurs with the same consequence. Tightening the entry guard moves
the witness; it does not remove the class. The distinguishing question was never
*which* neutrality but *when* it is checked, and only a standing obligation
answers that.

Candidate 2 is nonetheless **not refuted as a hardening** — it was not adopted
because it does not close the hole *on its own*, which is a different claim from
being wrong. If slice F's runs show the standing obligation is expensive in some
way the model can see, "both" is still available and is the shape `FM-7` argued
for one finding earlier: two independent mechanisms, either surviving a defect
in the other.

---

## 12. Slice F — `FM-8`'s ruling encoded, and it found `FM-9` (2026-08-02, session 71)

Slice E left exactly one thing open and said so in these words: *the clause is
normative from v0.9.25 and **no run yet shows the hole closes**.* This slice is
that run. It is also, again, a slice whose **first finding came from the
encoding rather than from the checking** — the same shape as item 5 of slice E,
where building a missing transition found `FM-6` before any invariant was
written.

### 12a. The question the ruling's wording did not answer: evaluated on WHAT?

§11f's sketch was: a `PREMISE_IS_STANDING` constant gating a
`Supersede("premise")` enabled on `PdActive /\ cls # "Determined"`. Writing it
exposes the problem in the first line.

`PdPlan`'s two entry conjuncts **are** the premise: `cls = "Determined"` and
`TrueResidual = 0`. Making them standing means re-evaluating them while the
generation runs — and **both are moved by the generation's own legs, by
design**:

- `PdSubmitPerp` calls `Reclassify(orders, pdTarget, deadline)`. The perp
  reduction is an exposure-changing order on the hedge group, so it enters §2's
  interval algebra and the residual is `Ambiguous` **from the generation's first
  step**. This is not incidental — §7 *depends* on it, because it is how §2
  refuses every compensating trade until the leg reaches finality. The model's
  own slice-B comment says so in as many words.
- `TrueResidual` moves whenever the generation deliberately sells back less
  than it bought (`PdAuthorizeSpot` with `s` below the transient), which is
  §7's own "no more spot than the **confirmed** perp reduction" and is the
  **safe** direction (net-long, never net-short — `INV5`).

So the literal sketch supersedes every generation on its own first step. The
quantity that actually **is** the premise is the entry test with this
generation's own legs removed from both sides of §2's subtraction:

```
PdOwnRealized   == pdDone - pdSold
PremiseResidual == (realized - PdOwnRealized) - (TrueReserved - PdReserved)
PremiseIntact   == PremiseResidual = 0 /\ ReportedDetermined(orders, 0)
```

**`PremiseResidual` is invariant under every action of the generation's own
ladder** — `PdPerpFill` raises `realized` by `x` and amends the entry by `x`;
`PdAuthorizeSpot` and `PdSpotFinal` move `pending` and the transient together —
which is **`FM-5`'s fix restated as a quantity**. It therefore moves only when
something external moves it, which is exactly what "the premise dissolved"
means. At entry (`pdDone = pdSold = PdReserved = 0`) it is literally
`TrueResidual`, so this is the entry guard **re-evaluated**, not a new rule.

**The invariant is over EVENTS, not states, and that was §11f's own warning
paying off.** `PdActive => PremiseIntact` is the obvious phrasing and it is
wrong: §7 requires the reserved second leg to reach §6.7 finality **first**, so
a window in which the premise is gone and the generation still exists is
**mandated by the spec**. A state predicate would flag the quiesce window and
the harm identically, and could be satisfied only by a model that violated
`INV6`. So `INV9` reads `premiseAdvanced`, a flag written at each of the
generation's **decision** steps (`PdSubmitPerp`, `PdAuthorizeSpot`,
`PdSubmitSpot`) — venue events and the parks are untouched, because they are
what a quiesce window is made of.

### 12b. `FM-9` — the landed trigger names a quantity that includes its own subject

`NC22` is a negative control on **the spec's own sentence**, which is a first
for either model: `PREMISE_NETS_OWN_LEGS = FALSE` evaluates the v0.9.25
transition row literally, `cls # "Determined"`.

**VIOLATED, and the trace is four states with no external event at all:**

| | |
|---|---|
| 1 | `Init` — `realized = 0`, `cls = "Determined"`, no orders outstanding, ledger empty. **The package is physically flat.** |
| 2 | `PdPlan` — a generation opens. Premise perfect under either reading. |
| 3 | `PdSubmitPerp` — the generation submits **its own** perp leg; `cls` → `"Ambiguous"` |
| 4 | `Supersede("PremiseLost")` — the row fires. `supersededOnIntactPremise = TRUE`, `realized` still `0` |

Nothing happened to the package. The row reported the generation's own
machinery back to it as a dissolved premise, and aborted it. **A spurious abort
is not bookkeeping**: §7's generations exist because the perp short is nearing
liquidation, so cancelling one cancels the mechanism that was reducing that
risk, and does it with a reserved leg out.

**`FM-9` is the third finding in one family**, and naming the family is the
point: `FM-5` — an entry read **before its backing exists**; `FM-8` — an
entry's **premise outliving it**; `FM-9` — a rule evaluated on a quantity that
**includes its own subject**. All three are §2 and §7 disagreeing about what
the shared ledger means. All three are invisible to every invariant phrased
over the ledger, for the reason §8e records: an invariant built on the ledger
cannot audit the ledger.

**It is also the second time in two sessions that a defect was found in the
clause landed one session earlier** — `FM-7` was a wrong *justification* under
a sound rule; `FM-9` is a wrong *quantity* under a sound ruling. The ruling
(standing precondition, not entry guard) is untouched and was not reopened;
what the model supplied is the quantity that makes it coherent. Normative edit
landed in v0.9.26: the §7 transition row and the standing-condition invariant.

### 12c. Results — and what each run is evidence for

| Run | Result | What it establishes |
|---|---|---|
| `AmbiguousOrder` (nominal) | **PASS** | `INV9`/`INV10` hold with all ten invariants listed |
| `AmbiguousOrder_SM2` | **PASS** — `25,514 / 5,169`, **identical to slice A's recorded graph** | slice F did not change the §2-only object |
| `NC21` (`PREMISE_IS_STANDING = FALSE`) | **VIOLATED `INV9`** | the pre-ruling world — the precondition as an entry guard alone — lets the machine advance a generation on a dissolved premise. **This is the run that says the hole closes**, because the constant *is* the ruling and nothing else differs from nominal |
| `NC22` (`PREMISE_NETS_OWN_LEGS = FALSE`) | **VIOLATED `INV10`** | `FM-9` — see §12b |
| `PC29` | **REACHED** | the new third supersession row **fires** in the nominal model |
| `W5` (NC21's constants) | **REACHED** | NC21's harm reaches a **nonzero spot sale** on the dissolved premise |
| `P6` (nominal) | **PASS** — `7,923,475 / 976,541`, **0 left on queue** | **the fix's own justification, checked**: the generation cannot move its own premise (§12e) |
| `NC23` (`PREMISE_SUPERSESSION_FAIR = FALSE`) | **VIOLATED `P5`** — same `976,541`-state graph as nominal | **a control on this slice's own first answer** (§12f): the standing obligation left merely admissible strands a generation forever |

**`PC29` is not optional and is the first thing to run after the fix.** `INV9`
is green under nominal constants **by construction** — the guard blocks
precisely the step the flag would record — so a green `INV9` is equally
consistent with the fix working and with a rule nothing ever exercises. That is
the model-altitude form of this repo's nine fixture collapses, and `PC29` is
what separates the two.

**`PC28` stays a positive control rather than flipping into an invariant**,
which is a correction to §11f's sketch. §11f predicted the flip and then, two
sentences later, gave the reason it cannot happen: the dissolved state is legal
during quiesce. `PC28` is still **REACHED** under the fixed nominal model, and
that is the correct result — the state exists, and what changed is that nothing
advances on it.

### 12e. `P6` — the fix arguing for itself, converted into a run

`FM-9`'s resolution rests on **one** claim: that the premise, evaluated net of
the generation's own legs, is a quantity **the generation itself cannot move**.
If that is false the fix is wrong — the trigger would fire on the generation's
own behaviour a second time and the finding would have been mis-diagnosed.

The claim was about to be recorded as an argument in **three** places (SM §7,
the companion artifact, this record). **§11e is this repo's evidence for what
that costs**: `FM-7` is a rule whose *recorded justification* was false while
the rule was sound, and the failure mode is specific — a later reader who
checks the reason and finds it does not hold will retire a rule that is doing
real work. **Nobody re-derives a recorded justification.** So it is checked:

```
PdOwnStep == PdPlan(t) ∨ PdSubmitPerp ∨ PdPerpFill(x) ∨ PdPerpFinal
           ∨ PdAuthorizeSpot(s) ∨ PdSubmitSpot ∨ PdSpotHolds
           ∨ PdPerpHolds ∨ PdSpotFinal(x)

P6_GenerationCannotMoveItsOwnPremise ==
  [][ PdOwnStep => PremiseResidual' = PremiseResidual ]_vars
```

**PASS over the complete graph** — 7,923,475 generated / 976,541 distinct / **0
left on queue**. `PdEnds` and `Supersede` are excluded on purpose: both *end*
the generation, and their counter resets are the scope closing rather than the
generation moving its premise.

**It is a conservation law, which is why it holds.** Every ledger change the
generation makes is to **its own** entry (counted in `PdReserved`) and every
realized change is **its own** transient (counted in `PdOwnRealized`), so both
subtractions cancel identically. That is `FM-5`'s fix — *"the reservation is
exactly backed at every instant"* — reappearing as arithmetic rather than as a
rule about behaviour. **Non-vacuity cost no run**: every action in `PdOwnStep`
is already shown REACHED by an existing positive control (`PC13`, `PC14`,
`PC15`, `PC18`, `PC20`, `PC21`, `PC25`).

**The general move, which is the part worth carrying:** when a fix's
correctness rests on a claim about a *quantity* rather than about a behaviour,
that claim is usually an action property and usually cheap. Prefer it to a
paragraph.

### 12f. The nominal liveness config refuted this record's own argument, twice

Both boundaries of the standing obligation were drawn by **runs that failed**,
not by design — and the second failure refuted an argument written in §12a of
this record earlier the same session.

**Failure 1 — the guard's SCOPE.** The first encoding guarded all three of the
generation's decision steps, including `PdSubmitSpot`. `AmbiguousOrder_Live`
returned a 16-state behaviour ending in **stuttering**: a generation buys 1
perp, authorizes **zero** spot — which *releases* its ledger entry, so its
transient becomes a genuine naked residual — §2's determined branch correctly
compensates that residual, and the package ends **physically flat and
`Determined`** while the generation sits at `SpotReduceAuthorized` unable to
finish. **Blocking a step that commits nothing buys no safety and costs
termination**: the amount was fixed at authorization and the entry is already
in the shared ledger, so refusing to send it *strands* the reserved leg, which
is `INV6`'s harm one step earlier. `PdSubmitSpot` is now unguarded, with that
argument recorded at the guard.

**Failure 2 — and it is this slice's own recorded reasoning, refuted.** The
first encoding carried this comment at `Next`: *"It is deliberately NOT in the
fairness set. Adding it would hand P3/P4 an exit their own premises do not
contain."* It was written as a considered design note, with a reason, in the
module itself — which is precisely the kind of claim §11e says nobody
re-derives. Re-running
`AmbiguousOrder_Live` after failure 1 returned a **20-state** behaviour
stuttering at `PerpReduceFinal`: a reduction's own ambiguous result had
released its ledger entry (**`FM-8`'s exact scenario**), so the premise was
genuinely dissolved, `PdAuthorizeSpot` was correctly blocked, the dispatch
budget was **spent** so §2 could not compensate either — and the one action
that could end it was *admissible and never taken*. **`P5` violated.**

**The analogy that produced the wrong argument is worth naming, because `Next`
already encoded the right distinction.** `EmergencyClosePackage` and
`FenceReadyLost` are **external arbitration events**: nothing in this model
decides when a liquidation policy or a fence arbitration fires, so leaving them
unfair is honest — assuming they eventually fire would assume away the
environment. The premise-lost row is a **machine obligation triggered by
state**, which is exactly why it needed its own guard in `Next` while the other
two are enabled unconditionally. Every other machine obligation here
(`PdSubmitPerp`, `PdAuthorizeSpot`, `PdSubmitSpot`, `PdEnds`) is in the
fairness set. It belongs there too, and SM §7's own verb says so: the
generation **is** superseded, not *may be*.

**`P3` is unaffected by construction rather than by luck**, which is worth
stating because it is the concern §12a raised: in `Held` the reserved leg is
live, so `QUIESCE_BEFORE_SUPERSEDE` blocks this action outright, and `P3`'s
consequent admits no supersession anyway.

**What this cost, and what it says about the fairness set.** §9a records that
*the fairness set's OMISSIONS are the theorem* — every action left out is one
the properties are proved without. That rule makes omission the safe default,
and it is right. **But it makes an omission that is WRONG invisible in exactly
one way: the property does not get weaker, it fails outright** — and only if
some property's subject can reach the stalled state. Both of these were caught
by the *nominal* liveness config, the one run in the whole suite whose job is
to notice that the machine can still finish. Neither would have been caught by
any invariant, any probe, or any of the 29 positive controls.

**A standing obligation nothing is obliged to discharge is not a standing
obligation** — and this is a *normative* point, not a modeling one, so it
landed in SM §7 beside the clause: an implementation that makes the
supersession possible on the next event and never guarantees it has a real
stuck state.

**And the refutation is kept as a RUN, not as this paragraph.** `NC23`
(`PREMISE_SUPERSESSION_FAIR = FALSE`) removes exactly the one fairness
conjunct and nothing else — **same constants, same 976,541-state graph, only
the behaviours differ** — so what it demonstrates is attributable to the
fairness rather than to a state change. It violates `P5`. Without it, the
refutation above is a session anecdote: a later reader tidying the fairness set
would find a reasoned comment and no objection, which is exactly `FM-7`'s
failure mode aimed at a mistake **this slice made itself**. Same move as `P6`,
one layer out — [[a-justification-belongs-in-a-run]] applies to the arguments
you lose as well as the ones you win.

### 12d. Three things this slice paid for

**(1) The witness-degeneracy question, asked of `NC21` in its own session —
and it needed asking twice.** NC21's counterexample advances via
`PdSubmitPerp`, the **cheapest** advance in the ladder, exactly as slice E
found to be systematic. `W5` asks for the money instead. Its **first** version
was violated by a spot authorization of size **ZERO** — the perp leg had
confirmed nothing, so `SpotCap` was `0` — a decision correctly flagged by
`INV9` that moves no money. **That is `W1`'s own finding** (an orphaned ledger
entry of amount zero leaks a slot and subtracts nothing) reappearing on the
probe written to avoid exactly this class, one slice later. The quantity had to
go **into the flag**; with it there, W5's witness is 8 states and ends in a spot
sale of 1 against a package short 1 with nothing reserved — the generation
**deepening** the naked short it was opened to be neutral toward.

**(2) `UNCHANGED` over a bare variable name failed three times in one
session.** Adding two history flags left them unassigned at `RecordDispatch`,
then at `Settled` (via `vars`), then at `Supersede` — three separate sites, each
found only by running. TLC refuses the successor state outright, so it is loud
rather than silent, but it is a failure at the wrong moment: the file's own
comment already said the two-group split (`sm2Hist` / the §7 flags) exists for
this reason, and the fix was to make the new flags a **group** (`advHist`,
inside `pdHist`) rather than three more names. **A discipline that exists
because of a past failure does not extend itself to new members.**

**(3) The regression evidence is the `SM2` figure, not the nominal one.** Slice
F adds an action and three variables, so the composed graph necessarily grows.
`AmbiguousOrder_SM2` (`MAX_GEN = 0`) has no generation, so its graph must be
**unchanged** — and it reproduces slice A's `25,514 / 5,169` to the digit. That
is what says the new variables and the new guard did not quietly alter the §2
object underneath the composition.

## RESUME HERE

**Slices A, B, C, D, E and F are BUILT** (§7–§12). **Slice F closed the one
thing slice E left open**: `FM-8`'s ruling is **encoded and checked** — `NC21`
violates `INV9`, so the hole the entry-guard-only reading left is shown to
close, and `PC29` shows the new supersession row fires rather than sitting
inert. **And encoding it found `FM-9`** (§12b): the transition row landed in
v0.9.25 named **the classifier's verdict**, which the generation's own perp leg
moves by design — control `NC22` aborts a generation on a **physically flat**
package 4 states from `Init` with no external event. Normative edit landed
v0.9.26; the *ruling* is untouched, only the quantity its trigger reads.

**There is no open finding on model (a).** `FM-5`, `FM-6`, `FM-7`, `FM-8` and
`FM-9` all have their normative edits landed **and** a run that demonstrates
each.

**Two things established this session that the open items should use.** First,
**building a missing transition is a finding technique, not bookkeeping** —
item 5 was carried as a correspondence tidy-up and produced a spec defect that
four rounds of review, four model slices, 55 runs and a clause-by-clause map had
all passed over. Items 3 and 4 are questions about **guards**, and the same move
applies: encode the other reading as a control and let TLC answer, rather than
arguing it. Second, **a recorded justification is load-bearing and is not
re-derived**: §10a's *reason* was wrong while its *conclusion* was defensible,
and the probe that showed it (`PC26`) cost one line.

**Slice E's docket, assembled rather than to be re-derived:**

1. ~~**`FM-5`'s normative edit — not optional, wording in §8d.**~~ **LANDED
   v0.9.24, 2026-08-02 — see §11a** (footprint wider than §8d prescribed, and
   why).
2. ~~**Model (b)'s five controls have never been asked the witness-degeneracy
   question**~~ **DONE, 2026-08-02 — see §11d.** Six probes (`FenceTakeover_W1`–`W6`),
   **all REACHED**; the degeneracy was systematic (one `VEntryFill`), and it
   found two suite defects: `I2` had never been violated by any run, and this
   artifact's prose carried harm its runs never showed.
3. ~~**§7d:** does "never issued twice" carry safety weight independent of the
   reservation?~~ **ANSWERED, 2026-08-02 — see §11e. `NC20` PASSES on the full
   graph** (1,086,199 distinct). The rule is defence in depth; **`FM-7`** is
   that its *stated justification* assumes what the adjacent sentence forbids.
   Wording ready.
4. ~~**§8f:** is *physical* neutrality the stronger and intended precondition
   for opening a generation?~~ **ANSWERED and it is worse than asked — see
   §11f. `PC27`/`PC28` both REACHED**; **`FM-8`** is that §7's "no naked
   residual" is enforced *only as an entry guard*, so a live generation can
   hold a premise that has dissolved. **Ruling owed.**
5. ~~**§10a:** the perp-side `Held/Alert` park the model narrows away.~~
   **BUILT, 2026-08-02 — see §11b/§11c. It found `FM-6`,** and it disproved
   §10a's own stated reason for the narrowing (`PC26`).

**`FM-5`'s normative edit LANDED in v0.9.24** (§11a) — the paragraph that stood
here saying otherwise was written for slice D and was not updated when item 1
closed. It is removed rather than corrected in place, because a stale "still
OWED" beside a closed item is the same drift `adr-history.md`'s Status line had
(§11a): the body moves and the line a reader keys off does not.

Decisions already taken, recorded so they are not silently reopened:
**(1)** §2 is modeled **with** §7, because the shared
`outstanding_reserved_reductions` ledger is the composition surface and
composition is where model (b) found its headline defect (§2.1); **(2)**
every liveness property ships with its negative control in the same slice,
because Cx8 proved a vacuous liveness property is indistinguishable from a
passing one; **(3)** slice A's seven build decisions in §7a — in particular
the `Impl*`/`True*` split and the `floorIssued`/`floorCount` separation;
**(4)** slice B's two in §8a/§8e — `INV1b` lives in the `MAX_GEN = 0`
config because §7's perp reduce legitimately breaks its premise, and the
generation's own legs are not `RecordDispatch`ed; **(5)** slice C's five in
§9a — in particular that the fairness set's OMISSIONS are the theorem, that
§8g's budget trap is answered by the properties' subject rather than by an
antecedent, and that a consequent stays narrow so each control names its own
defect.

**The two things slice D had to say out loud, and did** (§9d): `NC15`
produces a state graph *identical* to the nominal one, so the defect it
installs is invisible to every invariant in the module by arithmetic rather
than by choice — it is §1's headline in the artifact, and the answer to "why
is there a liveness section at all"; and the witness-degeneracy question,
which is the artifact's `W1`–`W4` subsection and carries model (b)'s unasked
mirror with it.

Slice E's docket is the numbered list above; nothing on it is to be resolved
earlier.

Standing context: task **#154**. The §45 gate waits on evidence until
~2026-09-30 (task #152); the §32m-D disk figure is due 2026-08-03T11:13Z
(task #150) and is measurable from 2026-08-02T11:13Z. Capture rituals
unchanged — pgrep PID 8495 first, never rebuild over `bin/arblab`. `run.sh`
now takes **`TLC_WORKERS`**: cap it while the capture runs.
