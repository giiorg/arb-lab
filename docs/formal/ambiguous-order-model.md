# Formal model (a) — AmbiguousOrder: correspondence map, results, FM-5

Companion artifact for `docs/formal/AmbiguousOrder.tla`. Its design record is
**§46** in `docs/superpowers/specs/2026-08-01-formal-model-a-ambiguous-order-design.md`
(built across slices A/B/C in sessions 65–67); this file is the reader-facing
half: what the model claims, which spec clause each claim came from, what it
deliberately does **not** claim, and how to re-run it.

The subject is `docs/state-machines.md` **§2** (ambiguous-order classifier +
finality contract) **composed with §7** (`PackageDelever` generations and
reserved reductions) on the one shared `outstanding_reserved_reductions`
ledger. Finding tags continue model (b)'s series: this model's first is
**`FM-5`**.

---

## 1. What was checked, and the headline results

**Eight** safety invariants and **five** temporal properties, over §2 composed
with §7. `./run.sh` is **60 runs** — 8 for model (b) `FenceTakeover`, **52 for
model (a)**: 2 safety nominals + `NC1`–`NC12` + `NC17` + `NC19`, the liveness
nominal + `NC13`–`NC16` + `NC18`, 26 positive controls, and 4 control-witness
probes. Every run is checked against a **recorded prediction**; a nominal that
fails and a control that passes are both failures of the suite. Slice E added a
fifth verdict, `deadlock`, because a control that **stranded** the model and one
that **caught** something are different claims — see `NC19`.

| | |
|---|---|
| Nominal (composed, safety) | **PASS** — 6,729,250 states generated / **832,179 distinct**, depth 30, no deadlock, ~8 s |
| Nominal (`_Live`, liveness) | **PASS** — same graph, `P1`–`P4` green, ~4 min |
| Nominal (`_SM2`, `MAX_GEN = 0`) | **PASS** — 25,514 / **5,169**, depth 15 — slice A's recorded graph **to the digit** |
| Negative controls | **16 of 16 VIOLATED**, each on its predicted invariant or temporal property |
| Positive controls | **24 of 24 VIOLATED** — each names a state the model must reach, phrased as the negative claim, so a *pass* is a coverage gap |
| Control-witness probes | **4 of 4 REACHED** |

Toolchain: Java 25, `tla2tools.jar` v1.7.4 pinned by SHA-256, TLC 2.19,
`-lncheck final`.

**Two results are worth more than the green runs.**

**(1) `FM-5` — a spec defect, found by the composition.** §7's FLAG-3 ruling
("the reservation stays generation-scoped **from creation**; the §2 classifier
subtracts it throughout") composed with §2's unconditional reservation-aware
sizing lets a generation book its **target** into the shared ledger at
`Planned`, before its perp leg has bought anything. A package that is neutral
then reads as *short* by that amount, and §2's determined branch buys back
exactly the exposure the generation's own perp leg is about to buy. Section 2
below. **Its normative edit LANDED in v0.9.24** (§46 slice E, 2026-08-02):
core §6 step 7 and SM §2/§7 now require every ledger entry to carry its
generation's *outstanding net intent* rather than its target.

**(1b) `FM-6` — a second spec defect, and the spec's own words name it.**
§2 Scope said classification "is a function of **two inputs** and re-runs when
either changes". The residual it gates on is
`realized_delta − outstanding_reserved_reductions` — **three** inputs, and the
third was missing from the re-run rule. So the **subtrahend**, which decides the
verdict exactly as the minuend does, moves on events that are **not order
events**: a generation opening, amending as its perp leg confirms, completing,
terminal-flattening, being superseded. TLC's signature is a **DEAD END**, not a
violated invariant — every interval collapsed, the verdict still
`DeadlineExceeded`, its one permitted action (the interval floor) spent, a
**determined naked residual** with dispatch budget in hand, and **nothing
enabled**. Control **`NC19`**. Normative edit landed v0.9.24.

**`FM-6` is `FM-5`'s sibling**: *the ledger must be backed when it is read*
versus *a change to the ledger must be observed* — the same asymmetry, once in
the value and once in its freshness, both because §2 treats
`outstanding_reserved_reductions` as an input it never re-derives.

**And neither was findable before slice E built the perp-side park**, for a
reason worth keeping: **every other exit from a generation runs through
`PerpReduceFinal`, which reclassifies.** The perp-side `Held/Alert` park is the
unique route that bypasses it. SM §7 has carried that transition row since v0.9;
the model did not. §4's clause-by-clause map found the omission, and *building*
it found the defect — the map's own justification, paid twice.

**(2) `NC15`'s state graph is identical to the nominal one — 832,179 distinct
states, to the digit** (a pre-slice-E figure; the nominal graph is **967,576**
distinct at depth 30 since the second park was added, and `NC15`'s equality
argument is unchanged — the control only *removes* a transition)**.** `HELD_RESUMES = FALSE` parks a `PackageDelever`
generation in `Held` for ever, holding an unsold net-long transient and a
reservation that subtracts from every later sizing. The control only *removes*
a transition, so its reachable set is a **subset** of the nominal one — and an
equal-cardinality subset of a finite set is that set. Every invariant in this
module, all seven plus `TypeOK`, is a **state predicate**, so every one of them
stays green on it. *No safety property could have caught this defect*, and that
is arithmetic about the state set rather than a claim about which invariants
were chosen. It is the sharpest available answer to "why is there a liveness
section at all".

---

## 2. `FM-5` — the reservation may be counted before its counterpart exists

**Status: found by this model (slice B, session 66); NC7 encodes the defective
reading; the model encodes the sound one. THE NORMATIVE EDIT LANDED IN v0.9.24
(§46 slice E, 2026-08-02)** — core §6 step 7, SM §2's reservation-aware sizing,
and SM §7's `Planned` / `SpotReduceAuthorized` rows, its `PerpReduceFinal`
action, a new "exactly backed at every instant" invariant and the **amended
FLAG-3 ruling**. `docs/formal/` and `docs/state-machines.md` **now agree**; the
window in which CLAUDE.md's standing rule ("treat the formal model as
authoritative about the safety kernel's behaviour") had to resolve a live
disagreement between them is closed. The wording that landed is §46 §8d's.

**The composition.** SM §7 FLAG-3 (resolved v0.9): *"The reservation stays
generation-scoped **from creation**, not first made here; the §2 classifier
subtracts it throughout."* SM §2 Invariants (Reservation-aware sizing): the
determined branch is sized from `realized_delta − outstanding_reserved_reductions`,
*"which includes reserved **generations**"*. Neither sentence is wrong alone.
Together they underdetermine **the reservation's amount at creation**, and the
literal reading books the generation's *target*.

**What TLC exhibits.** With `RESERVE_AT_CREATION = TRUE` (NC7), on a package
with `realized = 0` and `pdDone = 0`, `PdPlan` books `amt = 1`. Two steps in,
the ledger holds a reservation backed by nothing; three steps in,
`ImplResidual = 0 − 1 = −1` reads as a naked **short**, and
`IssueExposureReduction` dispatches a **buy of 1** — against a package that was
neutral, and for exposure the generation's own perp leg is about to acquire. The
end position (net long by the target) follows arithmetically from the model's
own transitions; it is recorded as a **derivation**, not as a checked property,
because pinning "the package was neutral at the start" would need history the
model does not carry.

**Why four rounds of human and machine review missed it, and slice A too.**
`INV1` is phrased over ledger-exact quantities — which are **computed from the
ledger**. A corrupt ledger corrupts them too, so the phantom buy computes as
non-overshooting and `overshoot` is **FALSE** in the violating state. *An
invariant built on the ledger cannot audit the ledger.* That is why `INV4`'s
`IntentsBacked` clause is stated at the **entry** level rather than as the trade
it causes: both harm-level phrasings were tried and both are wrong, one false of
correct composed behaviour and one blind to `FM-5` — recorded in the module as
`ReservationsBackedREJECTED` and `UnwarrantedREJECTED`, kept unused so a
rejected formulation nobody wrote down cannot be re-proposed.

**The fix, and FLAG-3 keeps its words.** What was underdetermined is the
reservation's *amount*, not its *existence*. The entry is created at `Planned`
carrying the generation's **outstanding net intent** — **zero** until the perp
leg confirms, amended as it fills. That is §2's own *"coalesce / amend /
cancel-replace it, never a fresh command per event"* applied to the reservation
rather than to the order. It is what the module implements
(`RESERVE_AT_CREATION = FALSE`), and `NC7` is the literal reading kept as the
control.

---

## 3. Correspondence map

Notation: SM clause → model construct. **"UNMODELED"** = deliberately out of
scope, with the reason and the discharging owner named. The abstraction register
in §5 applies throughout: quantities are small bounded integers, there is no
real time, no instruments and no prices, and one hedge group.

Every clause of SM §2's and §7's **Invariants** sections appears below — that
totality is this slice's done-when, and it is why the two Invariants tables are
exhaustive rather than illustrative.

### SM §2 — Scope, States, Transitions

| SM row / rule | Model |
|---|---|
| **Scope**: classification re-runs on **every fill/position event and every order-lifecycle/finality event** `[→ R1d-3]` | `Reclassify(o, p, d)`, folded **atomically into each such action** rather than being a separate step. The lifecycle/finality half is split out as `ReclassifyOnFinality` so **`NC13`** (`RECLASSIFY_ON_FINALITY = FALSE`) can remove it alone |
| **Scope**: "MUST NOT wait for the next fill when order state alone became conclusive" — a cancel confirmed with **zero** executions collapses the interval, emitting no fill | **Split in two, deliberately.** Safety half = `INV7` (control **`NC4`**, `CLASSIFY_ON_FILLS_ONLY`). Liveness half = a classifier re-running on fills only goes **stale-ambiguous**, which *stalls* rather than overshoots = `P1` (control **`NC13`**). `PC8` is the probe that the zero-execution collapse case is reached at all |
| `Determined` — "no exposure-changing order has any remaining executable **or unobserved** quantity" | `TrulyDetermined(o, p) == (∀ i : o[i].q = 0) ∧ p = 0`. `p` is the in-flight `PackageDelever` perp reduction: an exposure-changing order on the group like any other, so a classifier ignoring it would report `Determined` over a live leg — `INV7` reads `pdPerp` for exactly that reason |
| `Ambiguous` — a live counter-leg contributes a **possible-delta interval, not a point** | `orders[i] = [dir, q, seen]`, interval `[−q, 0]` for `dir = −1` and `[0, q]` for `dir = +1`. **A single order executes in ONE direction**, so its interval always has zero as an endpoint and never straddles; **straddling is a property of the SUM** (`SumLo`/`SumHi`), which is exactly where §2 forbids compensation. The `(lo,hi)`-per-order encoding was tried first and is both unfaithful and unbounded (§46 §7a decision 1) |
| `Ambiguous` covers `SubmissionUnknown` / `Acknowledged` (**unobserved**) as well as `PartiallyFilled` (**observed**) | `orders[i].seen`. Read **only** by the broken classifier — it *is* the difference between "remaining executable" and "unobserved" quantity, which is the distinction `INV7` polices |
| `DeadlineExceededAmbiguous` — the contract cannot complete within its deadline | `cls = "DeadlineExceeded"`, entered by `DeadlineBlows`. **Not a terminal state**: the late-resolution row is `P2` |
| **The 4-step per-venue finality contract** (fence dispatch path → terminal status → sweep past a watermark → reconcile to that watermark) | Abstracted to **one barrier**: `OrderFinal(i)` (and `PdPerpFinal` / `PdSpotFinal` for §7's legs), which either collapses an order's interval or does not. **The watermark itself is UNMODELED** — see §5; what the model checks is that the barrier is *waited for*, not that its four steps are implemented |
| Transition row 2 — determined branch: issue an `ExposureReduction` sized from `realized_delta − outstanding_reserved_reductions`, "tracked as a **fenced reduction generation** — coalesce / amend / cancel-replace, never a fresh command per event" | `IssueExposureReduction`, guarded on `cls = "Determined"` and `ImplResidual ≠ 0`. **What stops a second command for the same residual is the sizing itself**, not a generation guard: once the first generation is reserved, `realized − reserved` is zero and the action is *disabled*. That is the whole mechanism of "never a fresh command per event", and `NC6` removes it |
| Transition row 3 — quiesce every unresolved counter-leg to finality, only then issue | The dispatch guard `cls = "Determined"` composed with `TrulyDetermined`; violation flag `dispatchedWhileAmbiguous` → `INV3` |
| Transition row 5 — **`IntervalFloorReduction`**: endpoint **nearest zero**, only when the summed interval does not straddle, reserved as its own generation, **never issued twice** | `IssueFloor`, with three separable guards: `FLOOR_NEAREST_ZERO` (magnitude), `STRADDLE_GUARD` (`¬ImplStraddles`), `FLOOR_ONCE` (`¬floorIssued`). Controls `NC1`/`NC5`/`NC2` respectively. Reserved as `Pend(amt, "floor", TRUE)` in the same ledger |
| Transition row 6 — straddling interval ⇒ "**de-minimis only**" | Modeled as **nothing dispatched**. The de-minimis latitude is UNMODELED (§5): sizing it needs a magnitude threshold, which is money |
| Transition row 4 — the floor's **own** ambiguous result "**never blind-retries** … routes through this same §6.7 finality contract" | `ReductionAmbiguous(k)`: the reservation is released and the generation **becomes an unresolved order in its own right**, contributing `[−amt, 0]` with `seen = FALSE`. The forbidden path exists too, gated on a constant: `ReductionResultAmbiguous` + `ReductionBlindRetry` (**`NC16`**), so `P4` has something to fail on |
| Transition row 7 — intervals later collapse ⇒ back to `Determined` | `Reclassify` on every lifecycle event; `RECLASSIFY_AFTER_DEADLINE = FALSE` (**`NC14`**) latches the deadline verdict and is `P2`'s control |
| **Interval-floor rule** — floor magnitude is the endpoint nearest zero; the un-compensated ceiling is `max(\|a\|,\|b\|)` `[→ R1d-2]` | `IssueFloor`'s `amt` selection over `ImplLo`/`ImplHi`. **The `max(\|a\|,\|b\|)` gap-budget ceiling is UNMODELED** — it is a §6.4 *sizing* claim (money). What is checked is the floor's safety, not the budget that covers the remainder |

### SM §2 — Invariants (every clause)

| SM Invariants clause | Model |
|---|---|
| **Single gate** — "the classification, not the raw delta, gates the compensating trade" | `IssueExposureReduction`'s conjunct `(BLIND_COMPENSATION ∨ cls = "Determined")`. **`NC3`** removes exactly this conjunct → `INV3` VIOLATED |
| **Single gate** — "**no path compensates the uncertain part** of an ambiguous residual before the finality barrier (only the deadline-exceeded interval-floor generation may compensate the reachable-state-invariant floor)" | `INV3_NoUncertainCompensation` = `¬dispatchedWhileAmbiguous ∧ ¬nonFloorInDeadline ∧ ¬floorWhileStraddling`. **Checked in a STRONGER form than the sentence**: the model forbids *any* reduction while `Ambiguous`, not merely the uncertain part — because §2 itself narrows the floor to the deadline-exceeded branch ("normal operation waits for finality and compensates the exact residual"). The guaranteed floor of a *merely*-ambiguous residual is therefore not compensable, and the model matches the narrowing rather than the general sentence |
| **Single gate** — the roster of six clients (one-leg failure, ADL/liquidation, kill-switch/panic-MARKET, protection-disarm, `PackageDelever`/`EmergencyClosePackage`, actuator takeover) all route through this classifier | **Partially modeled.** `PackageDelever` and `EmergencyClosePackage` are first-class (§7 map below). The other four are **UNMODELED**: a client that routes through the classifier contributes *no transition the classifier can distinguish* — it is the same dispatch under a different name, so what a model would check is a **call-graph fact, not a behaviour**. Kill-switch/panic, protection-disarm and takeover are model (b)'s subject (`FenceTakeover`, SM §3/§4/§6) |
| **Single gate** — a pre-authorized panic-MARKET / venue-native stop "does **not** bypass the §6.7 finality barrier"; "any ambiguous close order routes through the §6.7 classifier" | **UNMODELED, and stated as such rather than claimed.** The model contains no bypass edge, and *the absence of a modeled bypass is not evidence that the implementation has none* — a rule of the form "there is no other path" cannot be checked by a model that only contains the paths its author wrote. Owner: §9 chaos cases + the §3.6 `singlewriter` gate's write-path discipline |
| **One canonical boundary** — "the ADL/panic *immediate* path is the **determined** branch, and a forced close *with* a still-live/partial counter-leg is ambiguous and quiesces first" | The dispatch guard `cls = "Determined"` + `TrulyDetermined` including `pdPerp = 0`; `INV7` is what makes the reported verdict trustworthy. `PC18` shows a partial confirmation is reached, so the "still-live/partial" case is not vacuous |
| **One canonical boundary** — "protection-disarm's sweep, the takeover's reconcile, and `PackageDelever`'s generations all reconcile through **this** contract **and its watermark**, never a weaker cancel-ack-only variant" | **Modeled for `PackageDelever` only**, and that is the load-bearing third: `PdPerpFinal` and `PdSpotFinal` are the **same barrier** as `OrderFinal`, not a private one. The sweep and the reconcile are model (b)'s clients. **The watermark is UNMODELED** (one barrier, no partial-order of observations), so "never a weaker cancel-ack-only variant" is an **abstraction boundary, not a check** — the model cannot tell a correct barrier from a weak one, it can only tell whether a barrier was waited for |
| **Forced-close events** — a `ForcedCloseReason` fires an immediate `ExposureReduction` **when the residual is determined**; with a live counter-leg it routes through the finality barrier first | The determined/ambiguous fork is `IssueExposureReduction`'s guard + `INV3`. **The `ForcedCloseReason` event itself is UNMODELED**: it is one of the ways `realized` comes to be nonzero, and the model's environment already chooses the opening residual at `Init` and moves it with `OrderFill` — the *cause* of a residual has no distinct transition, and adding one would add states no invariant reads |
| **Forced-close events** — "**never a next-tick reaction** either way" | **UNMODELED — a timing claim.** There is no real time in this model (§5). Owner: the §9 chaos gate's `<5 s` determined-reduce RTO |
| **Reservation-aware sizing** — sized from `realized_delta − outstanding_reserved_reductions`, "which **includes reserved generations**" | `ImplResidual == IF RESERVATION_AWARE_SIZING THEN realized − ImplReserved ELSE realized`. Control **`NC6`** (`INV1` VIOLATED — the reserved generation is re-read as fresh naked residual and a second reduction dispatched). **`NPEND = 3`** is load-bearing: with one slot the subtrahend is pinned to zero in every reachable state and "sized from the residual" and "sized net of reservations" become the same expression. `PC3` proves a nonzero subtrahend is reached |
| **Reservation-aware sizing** — the ledger's named producers: "a `PackageDelever`'s in-progress reserved second leg, **a protective order's leg reservation**" | Three producers modeled — `kind ∈ {"exposure", "floor", "pkgspot"}` — with `PC9`/`PC10`/`PC11` showing **each separately** contributes a nonzero term to the one subtraction, and `PC12` showing all three live at once. **The protective-order leg reservation is UNMODELED**: it is a fourth producer of the same shape, adding a term to the same sum and no new mechanism, and protective orders are SM §5's subject. So the clause is checked for three of the four named producers, and the composition argument (§46 §2.1) is exercised by those three |
| **Safety across reachable states** — "**no naked-short created and no doubled/over-reduced short across *every* reachable venue state**" | `INV1_NeverOvershootSign` = `¬overshoot`, where `Overshoots(amt)` is phrased **over resolutions**: after committing `amt`, is a value of the *opposite sign* still reachable under **some** resolution of the still-unresolved intervals? Never over the endpoint formula — the spec itself carried a bug there (v0.7.1's `\|b\|`, corrected to `max(\|a\|,\|b\|)` by R1d-2), so a model re-implementing the corrected formula and checking itself against it would prove nothing. `NC1` reproduces that asymmetry exactly. `INV1b_NoLaterOppositeSign` adds the persistent conjunct, checked in `AmbiguousOrder_SM2.cfg` where its premise holds |
| **Order type of the reduction** — the state-dependent §5.3 function: `reduceOnly` set **only** when the order reduces an existing position *on that instrument*; naked-long-spot vs naked-short-perp cases; "the choice follows policy + the net-delta invariant, the flag follows the instrument's current position" | **UNMODELED, deliberately.** The model has no instruments and no per-instrument positions — a reduction is a *signed magnitude* against one hedge-group residual. The rule's content is venue rejection semantics (a reduce-only order cannot *open*), which is a **Profiles** fact and a §5.3 command-verb fact; encoding it here would check a transcription. Nothing in `INV1`–`INV7` moves either way. Owner: the §9 acceptance case "ADL from a zeroed perp re-hedges via a *non*-reduce-only short" |
| **`EmergencyClosePackage` as a classifier client** — "quiesces existing orders through the §6.7 finality contract, then closes **both** legs (paired-IOC), routing any resulting one-leg residual back through the §6.7 classifier" | **Its supersession half is modeled**: `Supersede("EmergencyClosePackage")`, guarded by `QUIESCE_BEFORE_SUPERSEDE ⇒ ¬PdLegLive` → `INV6`, control `NC10`, reachability `PC16`. **The paired-IOC close is UNMODELED**: its two legs are orders like any other, and the resulting one-leg residual is exactly the state the environment already chooses at `Init` — modeling it would add two orders whose only effect is to produce a residual the model already ranges over |
| **During the ambiguous window** — directional risk bounded by pre-placed venue-native protection (§7.4) and the **naked-leg-gap sizing budget** (§7.1), never by a blind compensating trade | **The negative half is `INV3`** (never by a blind compensating trade), which is the half with behaviour in it. **The positive half is UNMODELED**: the gap budget is a *sizing* quantity (money, out of scope per §46 §2) and pre-placed protection is SM §5 / model (b) |
| **During the ambiguous window** — "**stop new strategy orders**" | Encoded as the **stated bound** "no new counter-leg order appears mid-episode" (§46 §7e). This is what makes `INV1` a claim about compensation *creating* an opposite sign rather than about pre-existing gap exposure — so it is a modeling premise **taken from this clause**, not an omission |
| **During the ambiguous window** — hold new trading, **reserve rate-limit budget** for recovery; escalate via the pre-authorized ladder; **operator alerted in parallel** | **UNMODELED.** Rate-limit budget and operator alerting have no state any property here reads; the escalation ladder's own shape is model (b)'s liveness layer (`IncidentResolves`). What is modeled of "hold" is the dispatch guard itself |
| **Determined-only shortcut** — "'reconciliation is never a prerequisite for flattening' holds for a *determined* residual, **not** an ambiguous one" | Both halves are structural. **Not for ambiguous**: `INV3`'s first conjunct, `¬dispatchedWhileAmbiguous`. **For determined**: `IssueExposureReduction` is enabled the moment `cls = "Determined"` — there is no further reconcile step in its guard, so the shortcut exists in the model rather than being asserted about it |

### SM §7 — States and Transitions

| SM row / rule | Model |
|---|---|
| `Planned` — "a generation is created" carrying "**explicit per-leg target positions**"; the intended net-long transient is "an **atomically reserved part of the fenced generation**" from creation | `PdPlan(t)`: `pd' = "Planned"`, `pdTarget' = t`, and a `"pkgspot"` ledger entry created **at this step under both readings** — what they disagree about is its **amount** (`RESERVE_AT_CREATION`: nominal = outstanding net intent, i.e. **zero** until the perp confirms; `NC7` = the target, which is `FM-5`). One target number: both legs share it, because the package is neutral at entry |
| `Planned` entry precondition — reduce leverage "on a **still-neutral** package"; "the package has **no naked residual**" | `PdPlan`'s guards `cls = "Determined" ∧ TrueResidual = 0`. **This is a reading and is marked as one** (§5): "no naked residual" is evaluated on §2's *reservation-netted* residual, which admits a package neutral **pending an in-flight reduction** rather than physically flat. Whether *physical* neutrality is the stronger intended precondition is a **slice-E question** — it is FM-5's mirror |
| `PerpReduceInFlight` — the perp-short reduction is submitted **first**, "which itself relieves the liq risk" `[→ R1b-1]` | `PdSubmitPerp` (guarded by `PERP_FIRST`), setting `pdPerp`. The submission makes the residual `Ambiguous` — **that is the composition doing its work**: §2 now refuses every compensating trade until this leg reaches finality. Its interval is `[0, pdPerp]` (a buy-to-reduce raises delta only), and it enters `SumHi` |
| `PerpReduceFinal` — the perp reduction confirmed **to finality** (§2) | `PdPerpFinal`, the **same barrier** as `OrderFinal`. `PdPerpFill(x)` moves `pdDone`; a partial confirmation is first-class and `PC18` shows it is reached |
| `SpotReduceAuthorized` — a **durable journal record** on perp finality `[→ R1d-F3]` (FLAG-3) | `PdAuthorizeSpot(s)`, `s ∈ 0..SpotCap`. The **durability** of the record is UNMODELED (no crash/recovery in this model); what is modeled is the **ordering** it exists to guarantee — no spot dispatch before perp finality — which is `PERP_FIRST` + `SpotCap`, controls `NC9` and `NC8` |
| `SpotReduceInFlight` — "**sells no more spot than the confirmed perp reduction**" | `PdSubmitSpot`; `SpotCap == IF SPOT_CAPPED_BY_PERP ∧ pd = "PerpReduceFinal" THEN pdDone ELSE pdTarget`. **The spot leg's size ranges over `0..cap` rather than sitting at the cap** — pinned at the cap, "no more than" and "exactly" would be the same predicate and §7's own inequality would be unchecked |
| `Complete` — "only now is a next generation admissible" | `PdSpotFinal(x)` → `Complete`; `PdEnds` returns `pd` to `"None"` and resets the per-generation counters, which is what makes `INV5` a **per-generation** claim. `PC13` (reached), `PC20` (a second generation starts) |
| `Held/Alert` — "a leg that is deadline-exceeded/ambiguous **holds and alerts, never blind-retries or blind-flattens**" | **BOTH entries modeled as of slice E.** Spot leg: `PdSpotHolds` → `pd = "Held"` (`PC14`). Perp leg: `PdPerpHolds` → `pd = "HeldPerp"` (`PC25`). There is **no retry edge out of either**. `HeldPerp` is a **distinct state, not a second way into `Held`** — model (b)'s `Cx8-2` ("a park is two states wearing one name") applied to its mirror here; the cause-blind exit is control **`NC17`**, caught by **`INV8`**. Exits: `Held` → `PdSpotFinal` / `PdTerminalFlatten`, with `Supersede` blocked by `QUIESCE_BEFORE_SUPERSEDE`; `HeldPerp` → `PdPerpFinal` / `PdTerminalFlatten` / **`Supersede`, which is NOT blocked** — §7's quiesce clause is scoped to "its **reserved second leg**", which does not exist yet at a perp-side park. That asymmetry is recorded, not absorbed (§46 §11b). **Slices A–D modeled the spot side only, and building the other one found `FM-6`** — §2 below |
| `Held/Alert` is **nonterminal** — eventual finality resumes the remaining target `[→ R1d-F4]` (FLAG-4) | `PdSpotFinal` from `Held`, gated by `HELD_RESUMES`. This is **`P3`**, and **`NC15`** is the control — the run whose state graph is identical to the nominal one |
| `TerminalFlatten` — "its terminal full-flatten routes through the same §6.7 finality contract, not a blind market dump" | `PdTerminalFlatten` (`Held → TerminalFlatten`), releasing the generation's reservation. **The flatten's own orders are UNMODELED**: what is modeled is that the generation ends and its reservation stops being counted; the flatten's legs are a different scenario's. `PC19` shows the state is entered |
| Supersession rows — `EmergencyClosePackage` (core §5.3) and `FenceReadyLost` (core §7.4) each "supersede and quiesce any active generation (cancelling its reserved second leg to §6.7 finality **first**)" | `Supersede(path)` for both paths, `PC16`/`PC17` showing **each separately** fires — the sign-mirror lesson applied to paths. Dropping the in-flight perp leg's remaining interval is *quiescence, not amnesia*: it is `OrderFinal`'s semantics, and everything the leg executed is already in `realized` |
| The escalation ladder that reaches `PackageDelever` (liq-distance breach → top-up → failure) | **UNMODELED** — `PdPlan`'s enabling is a nondeterministic event. See §7 Invariants below and §5 |

### SM §7 — Invariants (every clause)

| SM Invariants clause | Model |
|---|---|
| **Perp-short-first sequencing — never net-short** `[→ R1b-1]`: perp short reduced first, confirmed to finality, then "sells no more spot than the confirmed perp reduction — the momentary exposure is thus net-**long**, **never net-short**" | `INV5_PackageDeleverNeverNetShort`, two conjuncts mirroring `INV1`'s at-dispatch/persistent pair: **(a)** `pdSold ≤ pdDone` — the **outcome**, reading `pdSold`, which no guard touches; **(b)** `pdSold + PdReserved ≤ pdDone` — **every reachable interleaving**, because the still-outstanding reserved sale could execute in full at any moment. Without (b) the invariant would only catch a generation that had *already* gone net-short, i.e. after the harm. Controls: **`NC8`** (`SPOT_CAPPED_BY_PERP = FALSE`) and **`NC9`** (`PERP_FIRST = FALSE`) — two knobs, because an authorization taken before perp finality has no confirmed quantity to size from. Non-vacuity: `PC18` (partial confirmation) and `PC21` (the transient is actually nonzero) |
| **The reserved second leg vs the §2 classifier** — "an **atomically reserved part of the fenced generation**, so the §6.7 classifier **subtracts the reservation** and does not misread it as a naked residual" | The `"pkgspot"` producer in the shared ledger, subtracted by `ImplReserved` in the *same* expression as the other two producers — the composition surface itself. `IntentsBacked` is the clause that says the entry may not run ahead of the delta the generation has actually moved; **`NC7` breaks it and that is `FM-5`** |
| **The reserved second leg** — "only the **reserved spot reduction** (never a policy-selected re-hedge) completes the generation" | `ReductionFills`/`ReductionAmbiguous` **exclude `kind = "pkgspot"` by guard**; the pkg-spot entry resolves only through `PdSpotFinal`. So a generation cannot be completed by a plain reduction resolving — the exclusion is structural rather than asserted |
| **Bounded generations, gated on finality** — "each generation is a bounded attempt made **only after** the prior generation reaches §6.7 finality" | `PdPlan`'s guard `pd = "None"`, reachable only via `PdEnds` from `Complete`/`TerminalFlatten` (or via `Supersede`). `PC20` shows a second generation is genuinely started, so the gate is not passing because nothing ever gets there |
| **Bounded generations** — "explicit per-leg target positions" | `pdTarget`, one number shared by both legs (stated abstraction: the package is neutral at entry, so the per-leg targets are the same reduction on each side) |
| **Bounded generations** — the ladder is "**bounded and terminal-escalating, dropping notional/margin requirement**" (core §7.1) | **UNMODELED.** `MAX_GEN = 2` bounds generations, but that is **the model's own bound, not the spec's ladder** — and notional-dropping is a sizing policy, i.e. money. Stating a property about it would be evidence about `MAX_GEN`. `MAX_GEN = 2` is nevertheless not padding: with one generation the `Complete → next-generation` edge is unreachable and the "only after finality" gate blocks nothing observable |
| **Never blind** — a deadline-exceeded/ambiguous leg "**holds and alerts, never blind-retries or blind-flattens**" | For the **generation's** leg: `PdSpotHolds` → `Held`, with no retry edge, and `P3` for the exit. For a **reduction generation**: `ReductionAmbiguous` (the compliant path — route back through the contract) versus `ReductionResultAmbiguous`/`ReductionBlindRetry` (the forbidden path, `BLIND_RETRY`, **`NC16`**), with `P4` as the property. **The alert itself is UNMODELED** — no operator in this model |
| **Never blind** — the terminal full-flatten "routes through the §6.7 finality contract, **not a blind market dump**" | `PdTerminalFlatten` models the generation ending and its reservation being released; **its orders are UNMODELED** (see the states table). So the clause is checked as "the generation does not dispatch outside the contract from here", not as "the flatten's own legs are contract-routed" |
| **Completes the target on a determined leg failure** — "never buying the spot back to restore neutrality *and* the original notional (that would re-arm the same liq breach and loop)" | **Partially modeled, and this is where `FM-5` bites.** `PdSpotFinal(x)` admits `x ∈ 0..reserved`, so a shortfall becomes a genuine naked residual that §2's determined branch then compensates — the composition working. **Re-driving the remainder as a fresh leg is UNMODELED** (§46 §8f). The forbidden buy-back is exactly what `NC7` produces from the *ledger* side: a phantom short read off an unbacked reservation makes §2 buy back what the generation is about to buy |
| **Not an `ExposureReduction`** — "the package has **no naked residual**, so an `ExposureReduction` would either no-op (delta ≈ 0) or, by reducing a single leg, *create* a naked leg during the rally" | `PdPlan`'s entry guard (`cls = "Determined" ∧ TrueResidual = 0`) is §7's own precondition, encoded rather than assumed: a generation opened over an unresolved or nonzero residual is a different scenario §7 does not specify, and letting the model open one would manufacture violations of properties nobody claimed. The **reading** it embeds is marked in §5 |
| **Ambiguous top-up result ⇒ re-query first** — the durable journaled `MarginOp` (local op ID, intended delta, before-state + watermark, fsynced before dispatch), resolved by **per-venue evidence** (Binance `positionMargin/history` match; Bybit `positionIM` delta vs before-state), never a point lookup, never a blind delta retry; same-position ops serialized; `MarginOpStatus` carries a first-class `Ambiguous` `[→ R1d-6, Cx5-4]` | **UNMODELED in full, per §46 §2, and the reason is worth stating precisely:** the venue-specific halves are **Profiles facts**, and a model encoding them would be checking a **transcription, not a behaviour**. The part of the clause that *has* state — an ambiguous op resolved by evidence rather than by a blind retry — is present one level down and in the same shape: a generation **leg** whose result is ambiguous holds (`PdSpotHolds`) and a reduction generation's ambiguous result routes back through the contract (`ReductionAmbiguous`) instead of being re-sent (`NC16`). So the *hazard class* is checked; the margin-op instance of it is not. Owners: `docs/venue-profiles.md` §4 and the §9 integration cases |
| **Supersession quiesces the reserved leg first** — both paths "cancel/quiesce the generation's reserved second leg **to §6.7 finality first**" | `INV6_SupersessionQuiescesFirst == ¬supersededWithLiveLeg`, where the flag is written **unconditionally** from the state at the supersession and `QUIESCE_BEFORE_SUPERSEDE ⇒ ¬PdLegLive` is the guard that prevents the bad one. Relaxing the guard (**`NC10`**) therefore **sets the flag** rather than hiding the event — the same shape as `RecordDispatch`. `PdLegLive == pd ∈ {"SpotReduceInFlight", "Held"}` is precisely what the two rows require to be quiesced |
| **Escalate, don't wait** — "a slow top-up under stress … gets a pre-authorized fallback" (core §7.1) | **UNMODELED** — a latency-budget policy, and there is no clock here. `PdPlan`'s enabling is nondeterministic, which is the model's way of saying the trigger is the environment's |

**One correspondence gap this map surfaced, recorded here rather than absorbed.**
SM §7's transition row enters `Held/Alert` from **either** in-flight leg
(`PerpReduceInFlight` **or** `SpotReduceInFlight`); the model has only
`PdSpotHolds`, so **the perp leg cannot hold**. The reason it is defensible:
`Held` matters to this model because of what a held generation *holds* — an
unsold net-long transient and a reservation that subtracts from every later
sizing — and at the perp leg neither exists yet (`pdDone = 0`, and the
`"pkgspot"` entry carries the outstanding net intent, which is zero until the
perp confirms). So the perp-side park is the *harmless* instance of the state,
and `P3`/`NC15` are aimed at the one with content. But it is a **narrowing of
the spec row, not a coincidence of it**, and a slice-E round should ask whether
a perp-side park has a hazard this reasoning misses — a generation stuck before
its reservation grows still blocks the next generation on §7's own
"only after finality" gate, which is a liveness question no property here
states.

### Where the model is stricter than the spec sentence

Three places, each deliberate and each recorded above: `INV3` forbids *any*
reduction while `Ambiguous` (§2 narrows the floor to the deadline branch);
`INV5`'s conjunct (b) constrains the **commitment** and not only the outcome;
and `P2` is checked without §46 §4's "whose intervals later collapse"
qualifier, which the fairness set makes automatic. A model that is stricter than
its spec is only sound when the strictness is argued — these three are, and a
fourth (`P4`, checked in a **narrower** form) is argued in §5.

---

## 4. Results in full

### Nominals and negative controls (19 runs)

| Run | Constant flipped | Predicted | Result |
|---|---|---|---|
| nominal (composed, safety) | — | pass, all seven invariants | **PASS** — 6,729,250 / **832,179 distinct**, depth 30, no deadlock, ~8 s |
| `_SM2` (`MAX_GEN = 0`) | — | pass, incl. `INV1b` | **PASS** — 25,514 / **5,169**, depth 15 |
| `_Live` (nominal liveness) | — | pass, `P1`–`P4` + all seven invariants | **PASS** — same graph, ~4 min |
| `NC1` | `FLOOR_NEAREST_ZERO = FALSE` | `INV1` | **VIOLATED** |
| `NC2` | `FLOOR_ONCE = FALSE` | `INV2` | **VIOLATED** |
| `NC3` | `BLIND_COMPENSATION = TRUE` | `INV3` | **VIOLATED** |
| `NC4` | `CLASSIFY_ON_FILLS_ONLY = TRUE` | `INV7` | **VIOLATED** |
| `NC5` | `STRADDLE_GUARD = FALSE` | `INV3` | **VIOLATED** |
| `NC6` | `RESERVATION_AWARE_SIZING = FALSE` | `INV1` | **VIOLATED** |
| `NC7` | `RESERVE_AT_CREATION = TRUE` | `INV4` | **VIOLATED — this is `FM-5`** |
| `NC8` | `SPOT_CAPPED_BY_PERP = FALSE` | `INV5` | **VIOLATED** |
| `NC9` | `PERP_FIRST = FALSE` | `INV5` | **VIOLATED** |
| `NC10` | `QUIESCE_BEFORE_SUPERSEDE = FALSE` | `INV6` | **VIOLATED** |
| `NC11` | `RELEASE_ON_FINALITY = FALSE` | `INV4` | **VIOLATED** |
| `NC12` | `RELEASE_ON_SUPERSEDE = FALSE` | `INV4` | **VIOLATED** (+ deadlock — a second, independent signature) |
| `NC13` | `RECLASSIFY_ON_FINALITY = FALSE` | `P1` | **VIOLATED** — 7,101,314 / 930,385 |
| `NC14` | `RECLASSIFY_AFTER_DEADLINE = FALSE` | `P2` | **VIOLATED** — 3,597,110 / 460,997 |
| `NC15` | `HELD_RESUMES = FALSE` | `P3` | **VIOLATED** — 6,600,882 / **832,179** |
| `NC16` | `BLIND_RETRY = TRUE` | `P4` | **VIOLATED** — 10,787,270 / 1,352,785 |

**Every violation was read, not merely counted** — a violation by the wrong
mechanism is the formal-methods twin of `mutate.sh`'s `PANIC!!`: a kill, but not
the one aimed at. The ones worth recording:

- **`NC1`** reproduces R1d-2's asymmetry exactly. Residual `+1` with a pending
  sell of 1 gives summed interval `[0, 1]`: the near endpoint is **zero**,
  because the pending hedge may already cover the whole position. Nearest-zero
  issues nothing; worst-endpoint sells 1 and reaches `−1`.
- **`NC6`** dispatches `−1` against a determined `−1` residual and then, with
  the reservation not subtracted, **dispatches `−1` again**.
- **`NC12`** deadlocks as well as violating: with the orphan un-releasable,
  `Settled` (which requires an empty ledger) is permanently unreachable, so TLC
  independently reports "this ledger can never return to empty".
- **`NC13`** ends with every interval collapsed, `pdPerp = 0`, no generation
  running, a naked long of 1 — and `cls = "Ambiguous"` **for ever**, so the
  determined branch is barred from compensating a residual that is now exactly
  knowable.
- **`NC15`** parks at `pd = "Held"` with `gens = 2`, an **active** `pkgspot`
  reservation of 1, `pdDone = 1`, `pdSold = 0`, budget left: the perp leg
  bought, the spot leg never sold it back, and the reservation goes on
  subtracting from every later sizing.
- **`NC16`** lassos on a `floor` entry of `−1` against `realized = −2`, cycling
  `amb` for ever.

**`_SM2` reproduces slice A's state graph to the digit** — 25,514 / 5,169 at
depth 15. That is the evidence that slice B's refactor (a doubled ledger, three
new entry fields, a new `SumHi` term, a changed `Reclassify` arity) did not
quietly change the object slice A verified. The same discipline model (b)
supplies by reproducing its v0.9.4 counts across sessions, applied *within* one
model.

### Positive controls — 24 probes, all VIOLATED (i.e. all reached)

`AmbiguousOrderCoverage.tla` holds them, each phrased as the **negative** claim,
so **a TLC violation is the good outcome and a pass is a coverage gap**.

| Probe | The state it proves is reached |
|---|---|
| `PC1` | an `IntervalFloorReduction` is issued at all |
| `PC2` | two reserved generations live at once |
| `PC3` | a determined sizing with a **nonzero subtrahend** |
| `PC4` | a straddling summed interval inside the deadline branch |
| `PC5` | the floor's own ambiguous result re-entering the contract |
| `PC6` / `PC7` | a floor on the **net-short** / **net-long** side — the sign mirror |
| `PC8` | a lifecycle event collapsing an **unobserved** order's interval, delta unmoved |
| `PC9` / `PC10` / `PC11` | a determined sizing with a nonzero term from the **exposure** / **floor** / **pkgspot** producer, **separately** |
| `PC12` | all **three** producers live in the ledger at once |
| `PC13` / `PC14` / `PC15` / `PC19` | `Complete`, `Held`, `SpotReduceInFlight`, `TerminalFlatten` each entered |
| `PC16` / `PC17` | **both** superseding paths fire, separately |
| `PC18` | a perp reduction confirmed to finality having executed **some but not all** of its target |
| `PC20` | a **second** generation is started |
| `PC21` | the intended net-long transient is actually **nonzero** |
| `PC22` / `PC23` / `PC24` | `Ambiguous`, `DeadlineExceeded`, and a floor generation **outstanding** — the liveness antecedents (`P3`'s was already `PC14`) |

Three of these were named by the **fixture-collapse checklist before the model
ran**, and each pins a parameter that would otherwise collapse a distinction the
code makes: **`PC3`** (`NPEND`, or "sized from the residual" and "sized net of
reservations" are the same expression), **`PC18`** (a perp leg that always
confirms in full makes "no more than the **confirmed** reduction" and "…than the
**target**" the same expression, and `NC8` undetectable), and **`PC21`** (a
transient always zero makes both of `INV5`'s conjuncts trivially true).
`PC9`–`PC11` exist because `PC3` shows only that the *sum* is nonzero somewhere
— a three-producer ledger in which just one producer is ever nonzero would pass
`PC3` while §46 §2.1's entire argument for modeling §2 with §7 went unexercised.

### Control-witness probes — 4 runs

**A control's witness deserves the same fixture-collapse question a fixture
does.** TLC reports *a* counterexample, not the most harmful one, and the
difference is invisible unless the trace is read. `NC13`'s and `NC16`'s
witnesses both land at `budget = 0` — where the machine could not have
compensated anything even with a perfect classifier, so the trace demonstrates
the **stall** (which is all `P1` and `P4` claim) while saying nothing about what
the stall costs. So the question was asked of all four controls, and the answers
are committed runs rather than hand-checks.

| Probe | Asks | Answer |
|---|---|---|
| `W1` (NC12 constants) | is the orphaned ledger entry ever **nonzero**? | **reached** — the perp leg buys 1, the generation is superseded, and a `+1` entry masks a genuine naked long for ever |
| `W2` (NC13 constants) | can the machine be stranded with a **nonzero residual and budget left**? | **reached — 2 steps from `Init`**: `realized = −1`, every interval collapsed, `cls = "Ambiguous"`, the **full** budget intact |
| `W3` (NC14 constants) | the same, of the latched `DeadlineExceeded` verdict | **reached** |
| `W4` (NC16 constants) | can the determined branch be **about to size** while a floor stuck in the retry loop contributes to the subtraction? | **reached** — `realized = 2`, a stuck `floor` entry of 1: the branch sizes `2 − 1 = 1` and compensates **half** of a naked long |

`W3` was run rather than inferred from `W2` on purpose: the two controls prune
the state graph very differently (930,385 vs 460,997 distinct states), and *"the
harmful state is reachable over there"* is not evidence that it is reachable
here. `NC15` needed no probe — its park is harmful on its face.

**The mirror this pattern owes: model (b)'s five controls have never been asked
this question.** That is recorded as a slice-E debt (§46 §9f), not done here —
but the session that generalizes a check is the session that can see where else
it applies.

### Liveness: the fairness set, and what is left out of it

`P1`–`P4` are claimed only of behaviours satisfying `Fairness`, so **a generous
fairness set is a weaker result** and the omissions are the theorem.

**Fair:** the per-venue finality contract per order slot, as the **disjunction**
`OrderFill(i,·) ∨ OrderFinal(i)` — fairness on `OrderFinal` alone would assume
the no-execution path and on `OrderFill` alone would assume away the cancel case
§2 Scope names; a reduction generation's resolution, again a disjunction
(`ReductionFills ∨ ReductionAmbiguous`), so `P4` is not handed its conclusion;
and §7's own protocol steps, which are the machine's obligations once a
generation exists.

**Unfair, each for a stated reason:** `DeadlineBlows` (an environment
degradation), `PdSpotHolds` (the degradation that *creates* `P3`'s antecedent),
`PdTerminalFlatten` (a policy branch), `PdPlan` and `Supersede` (decisions out
of model), and **both compensating-trade dispatches**.

**`NC13` doubles as the control on the fairness set's own omissions.**
`DeadlineBlows` is enabled in *every* `Ambiguous` state and its target satisfies
`P1`'s consequent — so a **fair** `DeadlineBlows` would discharge `P1` by
assuming the very degradation `P1` exists to survive, and `NC13`'s stall would
be "rescued" by the deadline blowing, leaving `P1` green with the defect
installed. `NC15` is the same check on `PdTerminalFlatten`: made fair, it would
discharge `P3` by itself.

**None of the four properties is satisfiable in its own antecedent state**,
checked by inspection before the runs — that is the model-altitude form of the
fixture-collapse tell, since a property whose consequent already holds where its
antecedent does is green, vacuous and indistinguishable from a real one.
`cls = "Ambiguous"` is not in `P1`'s consequent set; `DeadlineExceeded ≠
Determined`; `Held ∉ {Complete, TerminalFlatten}`; `FloorOutstanding`
contradicts `¬FloorOutstanding`.

**The budget trap was answered by the properties' SUBJECT, not by an
antecedent.** Slice B predicted `P1`–`P4` would need conditioning on the
dispatch budget the way model (b) conditions liveness on `BOUND_CRASHES`,
because budget exhaustion produces a `Settled` stutter with a possibly-nonzero
residual. They are **not** conditioned — because **none of the four asserts that
a dispatch ever happens**, which is exactly what leaving both dispatch actions
out of the fairness set buys. A behaviour that spends `MAX_DISPATCH` and
stutters for ever with a naked residual falsifies nothing here. The property
that *would* have needed the conditioning is "the residual is eventually flat",
and it is deliberately **not stated**: it would be a claim about the model's own
bound rather than about the spec. `MAX_GEN` gets the same treatment — `P3` is
about the held generation's own resolution and never mentions a later one.

---

## 5. Honesty notes / limits

- **Small-scope claim.** Passing runs are evidence over *this* abstraction —
  one hedge group, `NORD = 3`, `MAXQ = 2`, `NPEND = 3`, `MAX_DISPATCH = 3`,
  `MAX_GEN = 2`, integers for quantities — not proof at implementation scale.
  §9's chaos cases remain the implementation gate.
- **This model cannot see implementation drift.** It checks the spec's
  mechanism. The eventual defence is TLC trace-checking against the real OMS,
  which is milestone 3's direction and out of scope here.
- **Budget exhaustion is a modeling terminal, not a claim.** `Settled` is an
  explicit self-loop at the one predicted terminal, so any *other* dead end
  reports as a deadlock and is a finding. That a behaviour can stop
  compensating when `MAX_DISPATCH` is spent says nothing about the system.
- **`P4` is checked in a NARROWER form than §46 §4 states, and `P2` in a
  STRONGER one.** `P4` claims only that the floor generation leaves the ledger,
  not that "the recursion terminates" — because the finitely-many-floors reading
  is guaranteed by **`MAX_DISPATCH`**, i.e. by the model's own bound, so a
  control against *that* reading could not exist and a property stating it would
  be evidence about the bound. `NC16`'s retry spends **no** budget precisely so
  the bound cannot terminate the loop and hide the defect. The narrow
  consequent is also what keeps each control naming its own defect: conjoining
  "and the episode ends `Determined`" would have made `NC13` fail `P4` too, and
  a property two controls can fail shows that *something* objected, not that
  *this* rule did. What `P4` gives up is recovered by composition — what the
  floor routes into is an order, and `P1` is the claim about orders. `P2` drops
  §46 §4's "whose intervals later collapse" qualifier because the fairness set
  makes it automatic.
- **Fairness is a premise, not a fact**, and the liveness config's state graph is
  the safety config's — fairness constrains behaviours, never reachability. That
  is what lets the 24 positive controls, all run against the safety constants,
  stand as the non-vacuity evidence for the liveness antecedents.
- **`INV7` holds by construction under the nominal constants** (`ReportedDetermined`
  *is* `TrulyDetermined`). Its evidence is `NC4`, not the nominal run.
- **"Never issued twice" is read as PER EPISODE**, and the model implements it
  that way: `floorIssued` and `floorCount` both reset when the classifier reports
  `Determined`, which is what ends a deadline-exceeded episode. §2 states the rule
  flatly but states it *inside* the `DeadlineExceededAmbiguous` row, and a later,
  distinct episode must plainly be allowed its own floor — a global reading would
  leave a second incident with no admissible safe action for ever. **A reading,
  stated, because an unstated reading is what later gets called drift.**
- **A generation may open on a package that is neutral *pending an in-flight
  reduction*, not necessarily physically flat.** `PdPlan` evaluates §7's "no
  naked residual" on §2's reservation-netted residual. Also a reading; whether
  *physical* neutrality is the stronger and intended precondition is a **slice-E
  question**, and it is `FM-5`'s mirror — they should be attacked together.
- **The straddling branch's "de-minimis only" latitude is not modeled.** The
  model dispatches nothing there, so `INV3`'s straddle clause is checked against
  "no floor", not against "nothing larger than de-minimis". That is unmodeled
  *permitted* behaviour — an abstraction, not a contradiction. Sizing de-minimis
  needs a magnitude threshold, which is money.
- **No real time.** Deadlines are nondeterministic environment events; every RTO
  in §2 and §7 (`<5 s` determined reduce, `<30 s` quiesce-to-finality) is a §9
  chaos-gate number, not a model property.
- **No crash/recovery.** `SpotReduceAuthorized`'s **durability** is therefore
  unchecked; what is checked is the ordering that record exists to guarantee.
  Crash-recovery is model (b)'s territory (`ActCrash`/`ActResume`).
- **`Held/Alert`'s resolution is one barrier and the generation completes at
  it.** FLAG-4's "resumes the remaining per-leg target" is abstracted to a single
  resolution with `x ∈ 0..reserved` executed; a shortfall becomes a genuine naked
  residual that §2's determined branch compensates. Re-driving the remainder as a
  fresh leg is not modeled, and `P3` says the park is *left*, not that the target
  is re-driven.
- **Domains were widened once, and the widening was FORCED.** With §7 composed
  in, `TypeOK` failed at depth 22 on an order of `q = 9` against slice A's
  ceiling of 8 — §7's generations move delta **without spending the dispatch
  budget**, so the reachable magnitude gains `MAX_GEN × MAXQ`. `QDom` went
  `0..8 → 0..14` and `AmtDom` `±16 → ±28`. A too-tight domain reporting as a
  type violation is the domain doing its declared job; the alternative is silent
  truncation of behaviour.
- **`CHECK_DEADLOCK FALSE` in exactly five configs — `NC13`, `NC14`, `W1`, `W2`,
  `W3` — and the asymmetry is itself evidence.** Those controls *strand* the machine (a
  frozen verdict with every interval collapsed and budget left has no enabled
  action at all), so TLC would report `Deadlock reached` during state generation
  and never run the liveness pass. It stays **on** in `NC15`/`NC16`, where
  nothing is stranded — those two were designed to remove an **edge** rather
  than a state's successors, precisely so the defect reports as the temporal
  violation it is instead of as TLC objecting to the shape of the control.
- **Named refinement candidates**, if this model is ever extended: the
  watermark's partial order (so "never a weaker cancel-ack-only variant" becomes
  checkable), the protective-order leg reservation as a fourth ledger producer,
  crash/recovery over the durable `SpotReduceAuthorized` record, and **a
  perp-side `Held/Alert` park** (§3's correspondence-gap note — the model
  narrows SM §7's row to the spot leg).

---

## 6. How to run

```
cd docs/formal && ./run.sh                 # the whole suite: 60 runs, both models
cd docs/formal && ./run.sh AmbiguousOrder  # model (a) only: all 52
TLC_WORKERS=4 ./run.sh AmbiguousOrder      # capped — see below
```

`run.sh` fetches `tla2tools.jar` v1.7.4 (SHA-256 pinned) into gitignored
`.tools/`, then runs each config and checks it against its **recorded
prediction**: nominals must pass, negative controls must fail on the predicted
invariant or temporal property, positive controls must be violated. Requires
Java 17+. Full outputs land in `.tools/*.out`.

Model (a)'s whole block is roughly **20 minutes at `TLC_WORKERS=4`**; the
safety nominal alone is ~8 s and the liveness nominal ~4 min. Model (b)'s
`_Live2` dominates the full suite at ~23 min.

**Two things about the runner that are load-bearing.**

**`TLC_WORKERS` caps the worker count**, and it is not a performance knob: the
market-data capture's archive is un-backfillable, and model (b)'s liveness
configs otherwise hold every core for ~30 minutes. Cap it (4 is what sessions
65–67 used) whenever the capture is running.

**The filter argument is an extended regex on the config *tag*, and the
namespace is what makes it honest.** `./run.sh AmbiguousOrder` selects all 47
model-(a) runs because **every** model-(a) config tag lives inside the
`AmbiguousOrder_` namespace — **including the coverage probes, whose configs the
script generates**. They did not always: they were written as `PC_*`, outside
the namespace their own model owns, so the script's own documented invocation
("model (a) + its positive controls") selected the 14 checked configs and
silently skipped all 21 probes — 14 of 35, with the summary line still reading
green. It was fixed by moving the generated configs into the namespace rather
than by correcting the comment, so the plausible invocation does the right thing
instead of being documented not to. The lesson generalizes past this script: **a
check that bites is worth nothing if the runner's own documented invocation
cannot select it.**
The summary prints `FILTERED: n run(s) skipped -- NOT the full suite` whenever
an argument is given, so a partial green can never be quoted as the whole one —
and the **recorded results in §4 mean the unfiltered suite**.
