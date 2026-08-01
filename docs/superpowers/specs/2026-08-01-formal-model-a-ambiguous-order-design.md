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

## RESUME HERE

**Slice A is BUILT and VERIFIED** (§7 above). Next act is **slice B**: the
reservation ledger's other two producers + the `PackageDelever` phases of
SM §7, with `INV4`–`INV6`, in `docs/formal/AmbiguousOrder.tla`.

Slice B's done-when (§5): green, **plus** the positive control that the
unbroken model reaches the interesting states — extend
`AmbiguousOrderCoverage.tla` so that **each of the three producers is shown to
contribute a nonzero term to the one subtraction**, which is §2.1's whole
argument for modeling §2 with §7. Report state counts; do not assume them.

Decisions already taken, recorded so they are not silently reopened:
**(1)** §2 is modeled **with** §7, because the shared
`outstanding_reserved_reductions` ledger is the composition surface and
composition is where model (b) found its headline defect (§2.1); **(2)**
every liveness property ships with its negative control in the same slice,
because Cx8 proved a vacuous liveness property is indistinguishable from a
passing one; **(3)** slice A's seven build decisions in §7a — in particular
the `Impl*`/`True*` split and the `floorIssued`/`floorCount` separation, both
of which slice B must preserve as it adds producers.

Carried forward for **slice E**, not to be resolved earlier: §7d's question
about whether "never issued twice" carries safety weight independent of the
reservation.

Standing context: task **#154**. The §45 gate waits on evidence until
~2026-09-30 (task #152); the §32m-D disk figure is due 2026-08-03T11:13Z
(task #150) and is measurable from 2026-08-02T11:13Z. Capture rituals
unchanged — pgrep PID 8495 first, never rebuild over `bin/arblab`. `run.sh`
now takes **`TLC_WORKERS`**: cap it while the capture runs.
