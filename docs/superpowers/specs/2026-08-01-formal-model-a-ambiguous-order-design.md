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

## RESUME HERE

Design only — **no `.tla` written yet**. Next act is **slice A**:
`docs/formal/AmbiguousOrder.tla`, the interval algebra and the classifier,
with `INV1`–`INV3` + `INV7` and a broken-constant control for each.

Two decisions already taken here, recorded so they are not silently reopened:
**(1)** §2 is modeled **with** §7, because the shared
`outstanding_reserved_reductions` ledger is the composition surface and
composition is where model (b) found its headline defect (§2.1); **(2)**
every liveness property ships with its negative control in the same slice,
because Cx8 proved a vacuous liveness property is indistinguishable from a
passing one.

Standing context: task **#154**. The §45 gate waits on evidence until
~2026-09-30 (task #152); the §32m-D disk figure is due 2026-08-03T11:13Z
(task #150) and is measurable from 2026-08-02T11:13Z. Capture rituals
unchanged — pgrep PID 8495 first, never rebuild over `bin/arblab`.
