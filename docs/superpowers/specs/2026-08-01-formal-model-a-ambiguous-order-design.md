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

## RESUME HERE

**Slices A, B and C are BUILT and VERIFIED** (§7, §8, §9 above). Next act is
**slice D**: the companion artifact `docs/formal/ambiguous-order-model.md` —
correspondence map (spec clause ↔ model construct), results table, honesty
notes, how-to-run. **Done when** every clause in §2's and §7's Invariants
sections is either mapped to a model construct or listed as deliberately
unmodeled *with a reason*. §9f is the shortest statement of what it
inherits.

**`FM-5`'s normative edit is still OWED, and it is not optional** (§8d). The
model found a spec defect in SM §7's FLAG-3 ruling composed with §2's
reservation-aware sizing; the model encodes the sound reading and NC7
encodes the defective one, but `docs/state-machines.md` still says the
unsound thing. §5's roster puts findings triage in **slice E**, so it is
scheduled there — but until it lands the two artifacts disagree and
CLAUDE.md's standing rule ("treat the formal model as authoritative about
the safety kernel's behaviour") is what resolves it. The wording to land is
in §8d.

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

**Two things slice D should say out loud, because they are the model's own
best evidence** (§9d): `NC15` produces a state graph *identical* to the
nominal one, so the defect it installs is invisible to every invariant in
the module by arithmetic rather than by choice — that is the correspondence
map's answer to "why is there a liveness section at all"; and `NC13`
doubles as the control on the fairness set's own omissions.

Carried forward for **slice E**, not to be resolved earlier: §7d's question
about whether "never issued twice" carries safety weight independent of the
reservation, and §8f's question about whether *physical* neutrality is the
stronger and intended precondition for opening a generation (it is FM-5's
mirror and should be attacked with it).

Standing context: task **#154**. The §45 gate waits on evidence until
~2026-09-30 (task #152); the §32m-D disk figure is due 2026-08-03T11:13Z
(task #150) and is measurable from 2026-08-02T11:13Z. Capture rituals
unchanged — pgrep PID 8495 first, never rebuild over `bin/arblab`. `run.sh`
now takes **`TLC_WORKERS`**: cap it while the capture runs.
