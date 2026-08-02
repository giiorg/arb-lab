---------------------------- MODULE AmbiguousOrder ----------------------------
(***************************************************************************)
(* Formal model (a), SLICES A+B: the ambiguous-order classifier, the       *)
(* possible-delta interval algebra, the per-venue finality contract, the   *)
(* reserved IntervalFloorReduction, and PackageDelever's generations       *)
(* composed onto the one shared reservation ledger.                        *)
(*                                                                         *)
(* COMPANION ARTIFACT. docs/state-machines.md (SM) stays normative; this   *)
(* module model-checks SM SS2 (ambiguous-order classifier & the finality   *)
(* contract) composed with SM SS7 (PackageDelever generations & reserved   *)
(* reductions) against the invariants their prose promises. Design record: *)
(* docs/superpowers/specs/2026-08-01-formal-model-a-ambiguous-order-       *)
(* design.md (SS46). Companion to FenceTakeover.tla (model (b)); same      *)
(* artifact discipline, next finding tag is FM-5.                          *)
(*                                                                         *)
(* SLICE A (SS46 SS5): the interval algebra, the classifier transitions,   *)
(* INV1-INV3 + INV7, one deliberate broken-constant run per invariant.     *)
(* SLICE B: the reservation ledger's remaining producer (PackageDelever's  *)
(* reserved spot leg), the SS7 generation phases, and INV4-INV6. This is   *)
(* where SS46 SS2.1's ruling is cashed: `outstanding_reserved_reductions`  *)
(* has THREE producers -- the determined branch's fenced generation, the   *)
(* IntervalFloorReduction, and PackageDelever's reserved spot leg -- and   *)
(* they all rest on ONE subtraction being exactly right. Model (b)'s       *)
(* headline defect (FM-1) was a COMPOSITION defect; this is the            *)
(* composition. Slice C adds liveness P1-P4.                               *)
(*                                                                         *)
(* Deliberately OUT of model (SS46 "Deliberately not modeled"):            *)
(* venue-specific REST semantics, money/prices/fees (quantities are small  *)
(* bounded integers), the liq-distance trigger and the margin top-up       *)
(* ladder that precedes a generation, the full SS1 OMS order lifecycle (an *)
(* order is modeled by the interval it contributes and whether it has been *)
(* observed executing), and real time (a deadline is a nondeterministic    *)
(* environment event, exactly as in model (b)).                            *)
(***************************************************************************)
EXTENDS Integers, Sequences

CONSTANTS
  \* --- Model bounds (SS46 SS6: stated limits, never silent choices) ------
  \* Order slots. The first (NORD - 1) are the hedge group's unresolved
  \* exposure-changing counter-legs at Init; the remainder are free slots
  \* that a reduction's own ambiguous result recycles into (SM SS2
  \* transition row 4: "its own ambiguous result ... routes through this
  \* same SS6.7 finality contract"). A slot with q = 0 is collapsed and
  \* reusable.
  NORD,
  \* Per-order remaining quantity ceiling at Init.
  MAXQ,
  \* Concurrent reserved generations. TWO is not padding: the floor
  \* generation can still be in flight when the intervals collapse and the
  \* determined branch issues its own reduction -- which is the only
  \* nominal state where `realized - reserved` has a NONZERO subtrahend,
  \* i.e. the only state where SS2.1's "one subtraction being exactly
  \* right" is exercised at all. With NPEND = 1 that subtraction would be
  \* pinned to zero and RESERVATION_AWARE_SIZING could not be shown to
  \* bite -- the fixture-collapse checklist at model altitude.
  NPEND,
  \* Total dispatch budget across a behavior. Bounds the floor's recursion
  \* (its ambiguous result re-enters the same contract) so the state graph
  \* is finite. A stated bound, not a claim.
  MAX_DISPATCH,
  \* PackageDelever generations per behavior (SM SS7: "each generation is a
  \* bounded attempt made ONLY AFTER the prior generation reaches SS6.7
  \* finality"). TWO, not one: with a single generation the ladder's
  \* Complete -> next-generation edge is unreachable and the "only after
  \* finality" gate blocks nothing observable.
  MAX_GEN,

  \* --- Nominal-vs-control constants (each FALSE/TRUE-flip is one NC) ----
  \* Nominal TRUE: the interval-floor magnitude is the endpoint NEAREST
  \* ZERO -- `a` when the summed interval is guaranteed net-long (a >= 0),
  \* `b` when guaranteed net-short (b <= 0) (SM SS2 Interval-floor rule).
  \* FALSE = negative control NC1: the WORST-magnitude endpoint
  \* max(|a|,|b|). That is the sign-asymmetry class of error the spec
  \* itself carried -- v0.7.1's "|b|" phrasing was correct only for the
  \* net-LONG case, corrected to max(|a|,|b|) by R1d-2 -- and it is why
  \* INV1 is phrased over RESOLUTIONS and never over this formula: an
  \* invariant that re-implemented the endpoint rule would flip with the
  \* rule and NC1 would pass (SS46 SS6).
  FLOOR_NEAREST_ZERO,
  \* Nominal TRUE: "never issued twice" (SM SS2 transition row 5 + the
  \* Interval-floor rule). FALSE = negative control NC2. NOTE the flag
  \* this guards, `floorIssued`, is deliberately NOT what INV2 reads --
  \* see `floorCount` below.
  FLOOR_ONCE,
  \* Nominal TRUE: the floor fires only when the summed interval does NOT
  \* straddle zero (SM SS2 rows 5/6: a < 0 < b means "no nonzero
  \* compensation is safe in all states -> de-minimis only"). FALSE =
  \* negative control NC5.
  STRADDLE_GUARD,
  \* Nominal FALSE: no compensating trade is dispatched on the OBSERVED
  \* delta while the residual is ambiguous -- SM SS2's headline rule
  \* ("acting on the observed delta is wrong in at least one reachable
  \* venue state ... -> naked short / doubled short"). TRUE = negative
  \* control NC3: the classifier's verdict stops gating the reduction, so
  \* the raw delta does, which is the wrong implementation the spec names.
  BLIND_COMPENSATION,
  \* Nominal FALSE: the classifier counts EVERY order with remaining
  \* executable OR UNOBSERVED quantity (SM SS2 `Determined`: "no remaining
  \* executable *or unobserved* quantity"). TRUE = negative control NC4:
  \* the classifier reads the fill ledger alone, so an order that has
  \* never been seen executing (SubmissionUnknown, Acknowledged with zero
  \* fills) is treated as contributing nothing.
  CLASSIFY_ON_FILLS_ONLY,
  \* Nominal TRUE: the determined branch is sized from
  \* `realized_delta - outstanding_reserved_reductions` (SM SS2
  \* Reservation-aware sizing). FALSE = negative control NC6: sized from
  \* the realized delta alone, so a reduction already reserved is re-read
  \* as fresh naked residual and compensated a second time. This is the
  \* single subtraction SS46 SS2.1 calls the composition surface.
  RESERVATION_AWARE_SIZING,

  \* --- SLICE B: PackageDelever (SM SS7) ---------------------------------
  \* The reservation's LIFETIME. SM SS7 FLAG-3's ruling reads: "the
  \* reservation stays generation-scoped FROM CREATION, not first made here
  \* [SpotReduceAuthorized]; the SS2 classifier subtracts it THROUGHOUT."
  \* TRUE models that literally -- the pkg-spot entry enters the ledger at
  \* `Planned`, carrying the generation's target, before the perp reduce has
  \* confirmed anything. FALSE models the alternative reading: the entry is
  \* the generation's OUTSTANDING NET intent, which is zero while the
  \* not-yet-executed perp buy still offsets the intended spot sell, so the
  \* counted amount only becomes nonzero at SpotReduceAuthorized, sized from
  \* the CONFIRMED perp reduction.
  \* The two readings differ observably, which is why this is a constant and
  \* not a modeling choice taken silently.
  RESERVE_AT_CREATION,
  \* Nominal TRUE: the reserved spot leg "sells no more spot than the
  \* CONFIRMED perp reduction" (SM SS7 Invariants, perp-short-first
  \* sequencing). FALSE = negative control NC8: sized from the generation's
  \* TARGET instead, so a partially-confirmed perp reduction is followed by
  \* a full-size spot sale and the transient goes net-SHORT -- exactly the
  \* direction SS7 says it must never take, in the rally that triggered the
  \* generation. Detectable ONLY where the perp confirmation can fall short
  \* of the target; probe PC18 is what proves that state is reached.
  SPOT_CAPPED_BY_PERP,
  \* Nominal TRUE: the perp short is reduced FIRST and confirmed to finality
  \* before the spot leg is authorized (SM SS7 transitions: "confirms that
  \* reduction to finality (SS6.7)" before any spot sell). FALSE = negative
  \* control NC9: the spot leg may be authorized from `Planned`.
  \* NOT an independent knob on the same number: an authorization taken
  \* before finality has NO confirmed quantity to size from, so the target
  \* is all that exists. NC9 removes the CONFIRMATION; sizing from the
  \* target is what its absence leaves behind (see SpotCap).
  PERP_FIRST,
  \* Nominal TRUE: both superseding paths (EmergencyClosePackage, core
  \* SS5.3; FenceReadyLost, core SS7.4) "cancel/quiesce the generation's
  \* reserved second leg to SS6.7 finality FIRST" (SM SS7 Invariants).
  \* FALSE = negative control NC10: the generation is ended while its
  \* reserved leg is still live.
  QUIESCE_BEFORE_SUPERSEDE,
  \* Nominal TRUE: a reservation is released when its reduction reaches
  \* finality. FALSE = negative control NC11: the entry survives its own
  \* counterpart, so `outstanding_reserved_reductions` keeps subtracting a
  \* reduction that already happened -- SS46 SS4's "no naked residual is
  \* masked by a reservation that has already COMPLETED". This is the
  \* control that forces the Impl/True split down into the LEDGER itself:
  \* see ImplReserved vs TrueReserved.
  RELEASE_ON_FINALITY,
  \* Nominal TRUE: a generation that ends by supersession or terminal
  \* flatten takes its reservation with it. FALSE = negative control NC12:
  \* the entry is ORPHANED -- no generation owns it and nothing will ever
  \* resolve it, so it subtracts from every later sizing forever. Separated
  \* from QUIESCE_BEFORE_SUPERSEDE (NC10) rather than folded into it so that
  \* one control trips one invariant: NC10 is about the LEG's finality
  \* (INV6), this is about the LEDGER ENTRY (INV4).
  RELEASE_ON_SUPERSEDE,

  \* --- SLICE C: the LIVENESS controls (one per property) -----------------
  \* Each of these four is read by exactly one action, and each is the
  \* constant a single P fails on. They are all NOMINAL-VALUED here in the
  \* safety configs too, where they change nothing: none of them is read by
  \* a guard that a safety invariant can see, and the nominal state graph is
  \* IDENTICAL to slice B's (the recorded 832,179 / 5,169 distinct-state
  \* counts are the check on that claim, not an assumption behind it).
  \*
  \* Nominal TRUE: SM SS2 Scope -- classification re-runs on every
  \* fill/position event AND "every hedge-group order-lifecycle/finality
  \* event", the case where "a cancel confirmed with ZERO executions emits
  \* no fill and no position event, yet fully collapses that order's
  \* interval". FALSE = negative control NC13: the machine re-classifies on
  \* FILLS ONLY, so an interval that collapses without executing leaves the
  \* verdict STALE-AMBIGUOUS -- the residual is determined and the machine
  \* never says so. SS46 SS7e parked this half of the clause here on purpose:
  \* its SAFETY half is INV7/NC4 (a classifier that reads fills alone
  \* OVERSHOOTS); this half STALLS instead, which no invariant can see.
  RECLASSIFY_ON_FINALITY,
  \* Nominal TRUE: SM SS2's late-resolution transition -- a
  \* DeadlineExceededAmbiguous episode whose intervals later collapse goes
  \* back to `Determined`. FALSE = negative control NC14: the machine treats
  \* the deadline-exceeded verdict as latched until an operator clears it,
  \* so classification stops re-running for the rest of the episode. The
  \* narrower control of the two: NC13 removes the finality edge for EVERY
  \* verdict (so it fails P1 and P2 both), this one removes only the
  \* transition BACK OUT of DeadlineExceeded, which is P2 alone.
  RECLASSIFY_AFTER_DEADLINE,
  \* Nominal TRUE: SM SS7 FLAG-4 rules `Held/Alert` NONTERMINAL -- eventual
  \* finality of the held leg RESUMES the generation. FALSE = negative
  \* control NC15: `Held` is a park, and the leg reaching finality no longer
  \* completes the generation. The park's only remaining exit is
  \* TerminalFlatten, which is a POLICY branch and therefore unfair, so a
  \* generation may sit held forever. Note the exit is REMOVED, not the
  \* state's successors: PdTerminalFlatten stays enabled, so the control
  \* reports as a temporal violation rather than as a deadlock.
  HELD_RESUMES,
  \* Nominal FALSE: SM SS2 row 4 -- "it NEVER BLIND-RETRIES, and its own
  \* ambiguous result ... routes through this same SS6.7 finality contract".
  \* TRUE = negative control NC16: on an ambiguous result the machine
  \* RE-SENDS the reduction instead of routing it, so the generation can
  \* cycle result-ambiguous -> re-send -> result-ambiguous forever and its
  \* reservation never leaves the ledger.
  \*
  \* This is the one control that needed a STATE rather than a constant
  \* flip, and the reason is worth keeping: in slices A/B a reduction's
  \* ambiguity is a CHOICE OF EDGE (ReductionFills vs ReductionAmbiguous),
  \* never a state the machine sits in -- so a blind retry was literally
  \* unrepresentable, and a control that leaves `vars` unchanged is a
  \* STUTTER, which weak fairness steps straight over. `pending[k].amb` is
  \* what makes the retry loop a fair lasso instead of a stutter.
  BLIND_RETRY,
  \* SLICE E. Nominal FALSE. SM SS7's Held/Alert is entered from EITHER leg,
  \* and FLAG-4's exit is "eventual finality of the HELD LEG resumes the
  \* remaining target" -- the held leg, not any leg. TRUE = negative control
  \* NC17: the exit from a park is CAUSE-BLIND, so the reserved spot leg's
  \* finality barrier also discharges a park whose PERP leg is the one that
  \* is unresolved. This is model (b)'s Cx8-2 recreated deliberately in
  \* model (a) -- "a park is two states wearing one name" -- and it is the
  \* first time model (a) has been asked that question. It is checked by
  \* INV8, and what it produces is a generation that reports SS6.7 FINALITY
  \* with its perp reduction still live, which is the state SS7's own
  \* "only after the prior generation reaches SS6.7 finality" gate reads.
  HELD_EXIT_CAUSE_BLIND,
  \* SLICE E -- `FM-6`. Nominal TRUE: a change to
  \* `outstanding_reserved_reductions` re-triggers SS2's classification,
  \* exactly as an order event does, because SS2's residual is
  \* `realized_delta - outstanding_reserved_reductions` and the SUBTRAHEND
  \* changes the verdict just as the minuend does. FALSE = negative control
  \* NC19: classification rides on ORDER events only, which is what slices
  \* A-D modeled and what SS2 says if read literally.
  \*
  \* Its signature is a DEADLOCK rather than an invariant or a temporal
  \* violation, and that is the honest report: the state has NO successors.
  \* A generation ends, the ledger changes, no order event accompanies it,
  \* so the episode keeps a `DeadlineExceeded` verdict after its intervals
  \* have collapsed -- and that verdict permits exactly one action, the
  \* interval floor, which is already spent. Determined naked residual,
  \* budget remaining, and nothing enabled.
  RECLASSIFY_ON_LEDGER_CHANGE,

  \* --- SLICE F: `FM-8`'s ruling (SM SS7, v0.9.25) -----------------------
  \* Nominal TRUE: "the package has no naked residual" is a STANDING
  \* condition and not only an entry test, so while a generation is active
  \* the premise it opened under is re-evaluated, and losing it SUPERSEDES
  \* the generation (quiescing the reserved second leg to SS6.7 finality
  \* first, exactly as SS7's other two superseding paths do).
  \*
  \* FALSE = negative control NC21, which is the PRE-`FM-8` world: the
  \* precondition is an ENTRY GUARD alone, checked once by PdPlan and never
  \* again. That is the model as it stood through slices A-E, and `PC28` is
  \* its witness -- a live generation over a package physically short, no
  \* longer `Determined`, with nothing reserved against it.
  PREMISE_IS_STANDING,
  \* Nominal TRUE: the premise is evaluated NET OF THE GENERATION'S OWN
  \* LEGS -- what the package's naked residual and classification would be
  \* if this generation were not running. FALSE = negative control NC22:
  \* the landed transition row read LITERALLY, "the SS2 classifier leaves
  \* `Determined` for this hedge group", on the classifier's actual verdict.
  \*
  \* The two readings are not a modeling preference. The generation's own
  \* perp reduction IS an exposure-changing order on the hedge group -- it
  \* enters SS2's interval algebra and makes the residual Ambiguous while it
  \* is live, which is the composition SS7 DEPENDS on (it is how SS2 refuses
  \* every compensating trade until the leg reaches finality). So under the
  \* literal reading the trigger fires on the generation's OWN first step,
  \* and what it reports is not a premise that dissolved.
  PREMISE_NETS_OWN_LEGS,
  \* Nominal TRUE: the standing obligation must be DISCHARGED, not merely
  \* admissible -- SM SS7 says the generation IS superseded, not MAY BE, so
  \* the row sits in the fairness set beside the machine's other protocol
  \* steps. FALSE = negative control NC23.
  \*
  \* This constant exists because the FIRST DRAFT LEFT THE ROW UNFAIR, with
  \* a recorded reason, and the nominal liveness config refuted it. Without
  \* the run the refutation is a session anecdote, and a later reader
  \* tidying the fairness set would find no objection -- which is `FM-7`'s
  \* lesson (nobody re-derives a recorded justification) applied to this
  \* slice's own mistake. NC23 keeps it re-derivable.
  PREMISE_SUPERSESSION_FAIR

(***************************************************************************)
(* Value domains. Deliberately generous: TypeOK is the CHECK that the      *)
(* bounds hold, so a domain that is too tight would report as a type       *)
(* violation (a real signal) rather than silently truncating behavior.     *)
(***************************************************************************)
\* Widened for slice B, and the widening was FORCED rather than chosen: with
\* SS7 composed in, TypeOK failed at depth 22 on an order of q = 9 against
\* the slice-A ceiling of 8. That is the domain doing its declared job -- a
\* too-tight domain reports as a type violation, which is a real signal,
\* instead of silently truncating behavior. The composition ratchets the
\* residual higher than SS2 alone can: SS7's generations move delta without
\* spending the dispatch budget, so the reachable |residual| gains
\* MAX_GEN * MAXQ on top of what SS2's own budget allows, and a reduction
\* sized from that residual re-enters as an order of that size.
QDom    == 0..14       \* an order's remaining quantity
AmtDom  == -28..28     \* signed reduction magnitudes / realized delta
Dirs    == { -1, 1 }   \* -1 = the order can still SELL (reduce delta)
                       \* +1 = the order can still BUY  (raise delta)
\* The THREE producers of `outstanding_reserved_reductions` (SS46 SS2.1):
\* the determined branch's fenced generation, the IntervalFloorReduction,
\* and PackageDelever's reserved spot leg. The kind is carried on the entry
\* so a per-producer claim can be made about the ONE subtraction they share
\* -- without it, "the ledger is nonzero" could not be attributed.
Kinds   == { "exposure", "floor", "pkgspot" }
Classes == { "Determined", "Ambiguous", "DeadlineExceeded" }

\* PackageDelever generation phases, verbatim from SM SS7 States, plus
\* "None" for "no active generation" (SS7's own transition table names that
\* as a from-state).
\* SLICE E: `HeldPerp` is a SECOND park, and it is a distinct state rather
\* than a second way into `Held` -- deliberately, and the reason is model
\* (b)'s Cx8-2 finding: "HeldIncident is two states wearing one name", where
\* a cause-blind exit guard discharged BOTH parks and the defect was
\* invisible to a green suite. SM SS7's transition row enters Held/Alert from
\* `PerpReduceInFlight` OR `SpotReduceInFlight`; slices A-D modeled only the
\* second (SS46 SS10a), so the mirror was never checked here. It is now:
\* NC17 is the cause-blind exit, and it is a violation.
PdPhases == { "None", "Planned", "PerpReduceInFlight", "PerpReduceFinal",
              "SpotReduceAuthorized", "SpotReduceInFlight", "Complete",
              "Held", "HeldPerp", "TerminalFlatten" }

\* The two superseding paths, kept DISTINCT rather than collapsed to a
\* boolean: SM SS7 names them separately (core SS5.3's EmergencyClosePackage
\* and core SS7.4's FenceReadyLost arbitration), and a single flag would let
\* one path be reachable while the other is dead code with the coverage
\* probe still green.
\* SLICE F adds the THIRD, `FM-8`'s: the generation's entry premise has
\* dissolved. Kept distinct for the same reason the first two are -- it is a
\* separate row in SM SS7's table with a separate trigger, and collapsing it
\* into either would make the probe that says it fires say nothing about
\* which one did.
SupPaths == { "none", "EmergencyClosePackage", "FenceReadyLost",
              "PremiseLost" }

VARIABLES
  \* --- The hedge group's unresolved counter-legs (SM SS2 `Ambiguous`) ---
  \* orders[i] = [dir, q, seen]. Its POSSIBLE-DELTA INTERVAL is [-q, 0]
  \* when dir = -1 and [0, q] when dir = +1: a single order executes in
  \* ONE direction, so its interval always has zero as an endpoint and
  \* never straddles. Straddling is a property of the SUM -- which is
  \* exactly where SM SS2 forbids compensation.
  \* `seen` = at least one execution of this order has been observed.
  \* Read ONLY by the broken classifier (CLASSIFY_ON_FILLS_ONLY); it is
  \* the difference between "remaining executable" and "unobserved"
  \* quantity, which is the distinction INV7 exists to police.
  orders,
  \* Realized net delta from the SS1 fill ledger. Positive = net long
  \* residual, negative = net short residual.
  realized,
  \* outstanding_reserved_reductions, as reserved GENERATIONS.
  \* pending[k] = [active, amt, kind, stale]. `amt` is signed in the same
  \* units as the residual it cancels: +1 means "1 of long residual is
  \* already committed to be reduced". `kind` names the PRODUCER, which is
  \* what makes a per-producer claim about the shared subtraction possible.
  \* `stale` = the entry's counterpart has already reached finality and the
  \* entry outlived it; reachable only under RELEASE_ON_FINALITY = FALSE
  \* (NC11), and read by the SIZING but never by the invariants -- the
  \* Impl/True split pushed down into the ledger.
  \* `amb` (SLICE C) = the generation's own result came back ambiguous and
  \* the machine has NOT yet routed it through the finality contract.
  \* Reachable only under BLIND_RETRY = TRUE (NC16): nominally a reduction's
  \* ambiguity is an EDGE (ReductionAmbiguous) and never a state, so `amb`
  \* is FALSE in every reachable nominal state and the nominal state graph
  \* is unchanged by its existence.
  pending,
  \* --- The classifier's REPORTED verdict (SM SS2 States) ----------------
  \* A variable, not a definition: the whole content of INV7 is that the
  \* REPORTED verdict matches the truth, and a `cls` defined as the truth
  \* would make INV7 a tautology under every constant setting.
  cls,
  \* The finality contract's deadline has been exceeded for this episode
  \* (SM SS2 row 3: "queries unavailable/rate-limited"). Cleared when the
  \* residual reclassifies Determined -- that is what ends an episode.
  deadline,
  \* --- The floor's guard and, separately, its DISPATCH COUNT ------------
  \* `floorIssued` is the guard the IMPLEMENTATION has.
  floorIssued,
  \* `floorCount` counts IntervalFloorReduction generations actually
  \* DISPATCHED in the current deadline-exceeded episode. INV2 reads THIS
  \* and never `floorIssued`. Written as `floorIssued => ~issue`, INV2
  \* would prove the flag consistent with itself: green, vacuous,
  \* worthless (SS46 SS3, the named trap). The two variables are kept
  \* deliberately separate so the property cannot read the mechanism it
  \* is checking.
  floorCount,
  \* --- History flags for action-level invariants (model (b) precedent:
  \* botLandedPostLatch / actLandedUnfenced). Every one is written
  \* UNCONDITIONALLY from the state at dispatch; the GUARDS are what
  \* prevent the bad dispatch. Relaxing a guard therefore SETS a flag
  \* rather than hiding it. ------------------------------------------------
  overshoot,                 \* INV1: some dispatch made the opposite sign
                             \* reachable under some resolution
  longReduced,               \* a reduction against a long residual fired
  shortReduced,              \* a reduction against a short residual fired
  dispatchedWhileAmbiguous,  \* INV3: compensation before the barrier
  nonFloorInDeadline,        \* INV3: a non-floor reduction in the
                             \* deadline-exceeded branch
  floorWhileStraddling,      \* INV3: a floor over a straddling interval
  \* --- SLICE B: the PackageDelever generation (SM SS7) -------------------
  \* The generation's phase. "None" = no active generation.
  pd,
  \* The generation's explicit per-leg target (SM SS7 `Planned`: "explicit
  \* per-leg target positions"). One number here: both legs share it,
  \* because the package is neutral at entry so the per-leg targets are the
  \* same reduction on each side.
  pdTarget,
  \* Remaining executable quantity on the IN-FLIGHT perp-short reduction
  \* (a buy-to-reduce, so it can only RAISE delta: its possible-delta
  \* interval is [0, pdPerp]). It is an exposure-changing order on the
  \* hedge group, so it enters SS2's interval algebra and makes the residual
  \* Ambiguous while it is live -- that is the composition, not a bolt-on.
  pdPerp,
  \* CONFIRMED perp reduction this generation (executed and reconciled).
  \* The spot leg is capped by THIS, never by pdTarget.
  pdDone,
  \* Spot actually SOLD this generation. INV5 reads this (the outcome), not
  \* the cap (the guard).
  pdSold,
  \* Generations started, bounded by MAX_GEN.
  gens,
  \* Which superseding path fired, if any (SM SS7's two supersession rows).
  superseded,
  \* INV6: a supersession ended a generation whose reserved second leg had
  \* NOT reached finality. Written unconditionally from the state at the
  \* supersession, exactly like the SS2 history flags above.
  supersededWithLiveLeg,
  \* --- SLICE F: `FM-8`'s two flags --------------------------------------
  \* INV9: a generation was ADVANCED BY MACHINE DECISION while the premise
  \* it opened under had dissolved. Written unconditionally at each such
  \* decision from the state before it, exactly like the flags above -- so
  \* NC21, which relaxes the guard, SETS the flag rather than hiding the
  \* step.
  \*
  \* It is a flag and not a state predicate, and that is the whole shape of
  \* the claim: the harm is NOT the dissolved state EXISTING. A premise can
  \* dissolve under a generation whose reserved leg is live, and SS7 then
  \* requires that leg to reach SS6.7 finality FIRST -- so a legal quiesce
  \* window contains exactly the state a naive invariant would flag. What
  \* is forbidden is the machine CHOOSING to go further in it.
  premiseAdvanced,
  \* The same event restricted to the SECOND LEG's decisions. Not read by
  \* any invariant: it exists to answer the witness-degeneracy question of
  \* NC21 (probe W5). TLC reports A counterexample, not the most harmful
  \* one, and NC21's is the CHEAPEST advance -- submitting the perp leg one
  \* step after the premise dissolved. This asks whether the machine can get
  \* all the way to AUTHORIZING AND SENDING REAL SPOT on the dissolved
  \* premise, which is where the money is.
  premiseAdvancedToSpot,
  \* INV10: the premise-lost supersession fired on a generation whose
  \* premise was INTACT. Written from the net-of-own-legs quantities, never
  \* from the constant under test -- the Impl/True split applied to a
  \* control: an invariant that read PREMISE_NETS_OWN_LEGS would move with
  \* the reading it is checking and NC22 would pass.
  supersededOnIntactPremise,
  \* --- Bound bookkeeping ------------------------------------------------
  budget

vars == << orders, realized, pending, cls, deadline, floorIssued, floorCount,
           overshoot, longReduced, shortReduced,
           dispatchedWhileAmbiguous, nonFloorInDeadline, floorWhileStraddling,
           pd, pdTarget, pdPerp, pdDone, pdSold, gens, superseded,
           supersededWithLiveLeg, premiseAdvanced, premiseAdvancedToSpot,
           supersededOnIntactPremise,
           budget >>

\* The SS2 flags and the SS7 flag are kept as two groups so a SS7 action can
\* write its own flag while leaving the SS2 six alone, without listing six
\* names at every site (where one omission is a silently unassigned
\* variable, the trap SS46 SS7d records).
sm2Hist  == << overshoot, longReduced, shortReduced,
               dispatchedWhileAmbiguous, nonFloorInDeadline,
               floorWhileStraddling >>
\* SLICE F. The advance flags are their own group for the reason the whole
\* two-group split exists: a site that names them individually can leave one
\* unassigned, and this file collected that error THREE TIMES in one session
\* -- once per site that writes a sibling flag by hand. TLC refuses the
\* successor state, so it is loud rather than silent; it is still a failure
\* at the wrong moment.
advHist  == << premiseAdvanced, premiseAdvancedToSpot >>
pdHist   == << supersededWithLiveLeg, advHist, supersededOnIntactPremise >>
histVars == << sm2Hist, pdHist >>
epiVars  == << cls, deadline, floorIssued, floorCount >>
pdVars   == << pd, pdTarget, pdPerp, pdDone, pdSold, gens, superseded >>

OrderIds == 1..NORD
PendIds  == 1..NPEND

(***************************************************************************)
(* Interval algebra.                                                       *)
(***************************************************************************)
RECURSIVE SumSeq(_)
SumSeq(s) == IF Len(s) = 0 THEN 0 ELSE Head(s) + SumSeq(Tail(s))

Abs(x)   == IF x < 0 THEN -x ELSE x

LoOf(o)  == IF o.dir = -1 THEN -o.q ELSE 0
HiOf(o)  == IF o.dir =  1 THEN  o.q ELSE 0

\* The summed possible-delta interval. The in-flight PackageDelever perp
\* reduction is part of it: it is an exposure-changing order on the hedge
\* group with remaining executable quantity, so SM SS2 `Ambiguous` covers it
\* verbatim. It is a BUY-to-reduce, so it can only raise delta -- its
\* interval is [0, pdPerp] and it contributes to the high end alone.
SumLo    == SumSeq([i \in OrderIds |-> LoOf(orders[i])])
SumHi    == SumSeq([i \in OrderIds |-> HiOf(orders[i])]) + pdPerp

\* outstanding_reserved_reductions, summed over live generations -- TWICE.
\*
\*   ImplReserved -- what the SIZING subtracts. Includes an entry whose
\*                   counterpart already reached finality (RELEASE_ON_
\*                   FINALITY = FALSE, NC11).
\*   TrueReserved -- what is genuinely still outstanding. The INVARIANTS
\*                   read this and only this.
\*
\* Without the split, NC11 would corrupt the invariants' own inputs and the
\* control would be checking a quantity it had just broken.
ImplReserved == SumSeq([k \in PendIds |->
                          IF pending[k].active THEN pending[k].amt ELSE 0])
TrueReserved == SumSeq([k \in PendIds |->
                          IF pending[k].active /\ ~pending[k].stale
                            THEN pending[k].amt ELSE 0])

\* The PackageDelever producer's own contribution to that one sum.
PdPendIds  == { k \in PendIds : pending[k].active /\ pending[k].kind = "pkgspot" }
PdReserved == SumSeq([k \in PendIds |->
                        IF pending[k].active /\ pending[k].kind = "pkgspot"
                          THEN pending[k].amt ELSE 0])

(***************************************************************************)
(* TWO views of the same quantities, kept apart on purpose.                *)
(*                                                                         *)
(*   Impl*  -- what the IMPLEMENTATION computes. A broken constant         *)
(*             corrupts these. Every ACTION GUARD reads these.             *)
(*   True*  -- the ledger-exact truth. Every INVARIANT reads these, and    *)
(*             only these.                                                 *)
(*                                                                         *)
(* This split is the structural form of SS46's "the property must not read *)
(* the mechanism it is checking": an invariant physically cannot see a     *)
(* constant's effect on the implementation's arithmetic.                   *)
(***************************************************************************)
ImplResidual == IF RESERVATION_AWARE_SIZING THEN realized - ImplReserved ELSE realized
ImplLo       == ImplResidual + SumLo
ImplHi       == ImplResidual + SumHi
ImplStraddles == ImplLo < 0 /\ ImplHi > 0

TrueResidual == realized - TrueReserved
TrueLo       == TrueResidual + SumLo
TrueHi       == TrueResidual + SumHi
Straddles    == TrueLo < 0 /\ TrueHi > 0

(***************************************************************************)
(* NOT LIVE. PhysLo/PhysHi and CommittedLo/Hi below exist only to define    *)
(* UnwarrantedREJECTED, which no configuration checks -- they are kept so   *)
(* the rejected reasoning can be read rather than re-proposed. INV4's live  *)
(* clause is IntentsBacked.                                                 *)
(*                                                                         *)
(* The PHYSICAL exposure range: fills that happened, plus the intervals of *)
(* orders that are actually live. The LEDGER PLAYS NO PART IN IT.          *)
(*                                                                         *)
(* This exists because True-side quantities are not a third opinion -- they *)
(* are computed FROM the ledger, so a corrupt ledger corrupts them too, and *)
(* INV1, phrased over them, computes a dispatch as non-overshooting         *)
(* precisely when the ledger it trusted is the thing that is wrong. An      *)
(* invariant built on the ledger cannot audit the ledger. PhysLo/PhysHi are *)
(* the independent quantity INV4 needs to say what the harm actually is.    *)
(***************************************************************************)
PhysLo == realized + SumLo
PhysHi == realized + SumHi

\* ...plus the reductions that are ACTUALLY OUT. A reservation whose order
\* has been submitted WILL move delta, so netting against it is correct
\* anticipation; a reservation that is still an INTENTION will not, and
\* netting against it is the FM-5 defect. The `sent` bit is what separates
\* them, and the separation is the whole content of INV4's harm clause:
\*
\*   YOU MAY NET AGAINST AN ORDER THAT IS OUT.
\*   YOU MAY NOT NET AGAINST AN INTENTION.
\*
\* Note the SIZING must still subtract intentions -- that is precisely what
\* stops SS7's deliberate net-long transient being re-hedged (SS7 Invariants,
\* "the SS6.7 classifier subtracts the reservation"). The two questions are
\* different, so they get two quantities: ImplResidual for the sizing,
\* CommittedLo/Hi for the harm.
SentReserved == SumSeq([k \in PendIds |->
                          IF pending[k].active /\ ~pending[k].stale
                             /\ pending[k].sent
                            THEN pending[k].amt ELSE 0])
CommittedLo == realized - SentReserved + SumLo
CommittedHi == realized - SentReserved + SumHi

\* THE SECOND FORMULATION THAT WAS TRIED AND IS ALSO WRONG (SS46 SS8e,
\* FM-model-2). A dispatch whose direction no reachable state warrants once
\* every order actually in flight has settled.
\*
\* It fails in the opposite direction to ReservationsBackedREJECTED: judged
\* against SENT reservations only, it flags CORRECT anticipation of a booked
\* intent (TLC, 8 steps: a group flat at `realized = 0` with the generation
\* committed to sell 2, where buying 2 is exactly right and lands flat);
\* judged against ALL reservations, it reads the very ledger that is wrong
\* and FM-5 becomes invisible to it again.
\*
\* THE CONCLUSION IS THE FINDING: there is no harm-level phrasing here that
\* is both sound and complete, because every "what will the position be"
\* quantity is computed from the ledger. A ledger defect has to be caught at
\* the LEDGER level -- which is what IntentsBacked does.
UnwarrantedREJECTED(amt) == \/ (amt > 0 /\ CommittedHi <= 0)
                            \/ (amt < 0 /\ CommittedLo >= 0)

(***************************************************************************)
(* The classifier (SM SS2 States + Scope).                                 *)
(*                                                                         *)
(* TRUTH: the residual is determined when NO exposure-changing order on    *)
(* the hedge group has any remaining executable OR UNOBSERVED quantity.    *)
(* An order whose interval has collapsed to [0,0] has neither, whether it  *)
(* collapsed by executing in full or by a cancel acknowledged with zero    *)
(* executions.                                                             *)
(***************************************************************************)
\* `p` is the in-flight PackageDelever perp reduction's remaining quantity.
\* It is an exposure-changing order on the group like any other, so a
\* classifier that ignored it would report Determined over a live leg -- the
\* composition's own version of INV7's hazard.
TrulyDetermined(o, p)    == (\A i \in OrderIds : o[i].q = 0) /\ p = 0

\* What the classifier REPORTS. Under CLASSIFY_ON_FILLS_ONLY it reads the
\* fill ledger alone, so an order with remaining quantity that has never
\* been observed executing is invisible to it -- including a PackageDelever
\* perp reduction that has not yet printed a fill (pdDone = 0), which is
\* the same SubmissionUnknown/Acknowledged case one level out.
ReportedDetermined(o, p) ==
  IF CLASSIFY_ON_FILLS_ONLY
  THEN /\ \A i \in OrderIds : (o[i].q = 0 \/ ~o[i].seen)
       /\ (p = 0 \/ pdDone = 0)
  ELSE TrulyDetermined(o, p)

\* Re-classification. SM SS2 Scope [-> R1d-3]: classification re-runs on
\* EVERY fill/position event AND every hedge-group order-lifecycle/finality
\* event -- so it is folded into each such action atomically rather than
\* being a separate step. (The "machine MUST NOT wait for the next fill"
\* hazard is therefore a LIVENESS question here, not a safety one: a
\* classifier that re-ran on fills only would go STALE-AMBIGUOUS, which
\* stalls rather than overshoots. Its control belongs with P1/P2 in slice
\* C. The SAFETY half of the same clause is INV7, controlled by NC4.)
\*
\* An episode ENDS when the classifier reports Determined: the deadline
\* clears and the floor's guard and dispatch count reset with it. INV2 is
\* therefore per-episode, exactly as SS46 SS4 states it.
\* SLICE C. `frozen` is NC14: the machine latches the deadline-exceeded
\* verdict and stops re-classifying for the rest of the episode, so SS2's
\* late-resolution transition (row 6 -> row 2) never fires. Written as a
\* whole-step no-op rather than as a guard on `det`, because the defect
\* being modeled is "classification stopped running", not "classification
\* ran and answered wrongly" -- the latter is INV7's territory and already
\* has NC4.
Reclassify(o, p, d) ==
  LET det    == ReportedDetermined(o, p)
      frozen == ~RECLASSIFY_AFTER_DEADLINE /\ d
  IN  /\ cls'         = IF frozen THEN cls
                        ELSE IF det THEN "Determined"
                        ELSE IF d THEN "DeadlineExceeded" ELSE "Ambiguous"
      /\ deadline'    = IF frozen THEN deadline
                        ELSE IF det THEN FALSE ELSE d
      /\ floorIssued' = IF frozen \/ ~det THEN floorIssued ELSE FALSE
      /\ floorCount'  = IF frozen \/ ~det THEN floorCount  ELSE 0

\* SLICE C. The ORDER-LIFECYCLE/FINALITY half of SM SS2 Scope's
\* re-classification clause, split out so NC13 can remove it alone. The
\* fill/position half stays in `Reclassify` at its own call sites: a machine
\* that re-classifies on fills only is exactly the stale-ambiguous stall the
\* clause exists to forbid, and the collapse-with-no-execution case is the
\* one where the two halves differ (PC8 is the probe that says that case is
\* reached).
ReclassifyOnFinality(o, p, d) ==
  IF RECLASSIFY_ON_FINALITY THEN Reclassify(o, p, d) ELSE UNCHANGED epiVars

\* SLICE E -- `FM-6`. The LEDGER half of the same re-classification clause.
\*
\* SS2's residual is `realized_delta - outstanding_reserved_reductions`, and
\* re-classification in slices A-D rides on ORDER events alone. But the
\* SUBTRAHEND is as capable of changing the verdict as the minuend is, and a
\* generation ending changes it WITHOUT producing an order event. Until
\* slice E that could not be observed, because every path out of a
\* generation ran through PdPerpFinal, which reclassifies -- the perp-side
\* park (SM SS7's own transition row, unmodeled until now) is the unique
\* route that bypasses it.
\*
\* Nominal TRUE = the sound reading. FALSE = negative control NC19, the
\* literal one, whose signature is a DEADLOCK: the episode's intervals have
\* collapsed, its verdict is still `DeadlineExceeded`, the one action that
\* verdict permits is spent, and no event will ever re-open one.
ReclassifyOnLedgerChange(o, p, d) ==
  IF RECLASSIFY_ON_LEDGER_CHANGE THEN Reclassify(o, p, d) ELSE UNCHANGED epiVars

(***************************************************************************)
(* Dispatch bookkeeping.                                                   *)
(*                                                                         *)
(* Overshoots(amt) is INV1's content, phrased OVER RESOLUTIONS: after      *)
(* committing `amt`, is a value of the OPPOSITE sign still reachable under *)
(* SOME resolution of the still-unresolved intervals?                      *)
(*                                                                         *)
(* Why the post-dispatch range is [TrueLo - amt, TrueHi - amt] and not     *)
(* something wider: the dispatched reduction may itself fail to fill in    *)
(* full, which moves the outcome BACK toward the pre-dispatch range. For a *)
(* long reduction (amt > 0) that only raises the low end, so the extreme   *)
(* for a sign crossing is the FULL fill -- the two formulations coincide   *)
(* on the endpoint that matters, and no worst-case hand-wave is needed.    *)
(***************************************************************************)
Overshoots(amt) == \/ (amt > 0 /\ TrueLo - amt < 0)
                   \/ (amt < 0 /\ TrueHi - amt > 0)

\* Every right-hand side is fully parenthesized ON PURPOSE: in TLA+ `=`
\* binds tighter than `\/`, so `f' = f \/ cond` parses as
\* `(f' = f) \/ cond` -- a disjunction satisfied by `cond` alone, which
\* leaves f' UNASSIGNED and the flag silently unrecorded. TLC rejects that
\* outright ("successor state is not completely specified"), and it did on
\* this file's first run.
RecordDispatch(amt, kind) ==
  /\ overshoot'                = (overshoot \/ Overshoots(amt))
  /\ longReduced'              = (longReduced  \/ (amt > 0))
  /\ shortReduced'             = (shortReduced \/ (amt < 0))
  /\ dispatchedWhileAmbiguous' = (dispatchedWhileAmbiguous \/ (cls = "Ambiguous"))
  /\ nonFloorInDeadline'       = (nonFloorInDeadline
                                  \/ (cls = "DeadlineExceeded" /\ kind # "floor"))
  /\ floorWhileStraddling'     = (floorWhileStraddling
                                  \/ (kind = "floor" /\ Straddles))
  \* The SS7 flags are written by SS7's own actions alone. Named here rather
  \* than at the two call sites so that adding a third dispatching action
  \* cannot leave one unassigned -- which is not a silent failure (TLC
  \* refuses the successor state outright) but is a failure at the wrong
  \* moment. SLICE F: `pdHist` is a GROUP for exactly that reason -- adding
  \* FM-8's two flags as bare names left them unassigned here, and TLC said
  \* so on the first run.
  /\ UNCHANGED pdHist

FreePend  == { k \in PendIds : ~pending[k].active }
FreeOrder == { i \in OrderIds : orders[i].q = 0 }

\* An inactive slot is canonicalized (amt = 0, kind = "exposure") and a
\* collapsed order is canonicalized (dir = -1, seen = FALSE). Sound
\* because nothing reads those fields once the slot/order is inactive:
\* LoOf/HiOf are 0 at q = 0, the ledger sums skip inactive slots, and the
\* classifier's `q = 0 \/ ~seen` is already true at q = 0. It exists only
\* to stop two behaviorally identical states counting twice.
DeadPend  == [active |-> FALSE, amt |-> 0, kind |-> "exposure",
               stale |-> FALSE, sent |-> FALSE, amb |-> FALSE]
DeadOrder == [dir |-> -1, q |-> 0, seen |-> FALSE]

\* A live ledger entry, by producer. `snt` = its counterpart order has been
\* SUBMITTED. A reservation booked at INTENT and one booked at DISPATCH are
\* different objects and INV4 has to tell them apart -- see CommittedLo.
Pend(a, kd, snt) == [active |-> TRUE, amt |-> a, kind |-> kd,
                     stale |-> FALSE, sent |-> snt, amb |-> FALSE]

(***************************************************************************)
(* Init. The environment picks the hedge group's opening position: the     *)
(* realized residual, and for each counter-leg its direction, its          *)
(* remaining quantity, and whether it has been observed executing (the     *)
(* PartiallyFilled case) or not (SubmissionUnknown / Acknowledged).        *)
(* Slot NORD starts collapsed -- it is the recycling slot for a            *)
(* reduction's own ambiguous result.                                       *)
(***************************************************************************)
InitOrders ==
  { [dir |-> d, q |-> q, seen |-> s] :
      d \in Dirs, q \in 1..MAXQ, s \in BOOLEAN } \union { DeadOrder }

Init ==
  /\ orders \in [ OrderIds -> InitOrders ]
  /\ orders[NORD] = DeadOrder
  /\ realized \in -MAXQ..MAXQ
  /\ pending = [ k \in PendIds |-> DeadPend ]
  /\ deadline = FALSE
  /\ pd = "None" /\ pdTarget = 0 /\ pdPerp = 0
  /\ pdDone = 0 /\ pdSold = 0 /\ gens = 0
  /\ superseded = "none" /\ supersededWithLiveLeg = FALSE
  /\ premiseAdvanced = FALSE /\ premiseAdvancedToSpot = FALSE
  /\ supersededOnIntactPremise = FALSE
  /\ cls = (IF ReportedDetermined(orders, 0) THEN "Determined" ELSE "Ambiguous")
  /\ floorIssued = FALSE /\ floorCount = 0
  /\ overshoot = FALSE
  /\ longReduced = FALSE /\ shortReduced = FALSE
  /\ dispatchedWhileAmbiguous = FALSE
  /\ nonFloorInDeadline = FALSE
  /\ floorWhileStraddling = FALSE
  /\ budget = MAX_DISPATCH

(***************************************************************************)
(* Environment: how an order's possible-delta interval collapses.          *)
(***************************************************************************)

\* A fill/position event. The order executes x of its remaining quantity:
\* the realized delta moves by dir*x and the remaining interval shrinks by
\* exactly the same amount -- so the TRUE net range [TrueLo, TrueHi] is
\* INVARIANT under a fill. (That is the first half of the small theorem
\* behind INV1b -- which slice B moved to the SS2-ONLY configuration, because
\* SS7's perp reduce is exactly the kind of event the theorem assumes away.
\* See INV1b.)
OrderFill(i, x) ==
  /\ orders[i].q > 0
  /\ x \in 1..orders[i].q
  /\ LET q2 == orders[i].q - x
         o2 == IF q2 = 0 THEN DeadOrder
                         ELSE [dir |-> orders[i].dir, q |-> q2, seen |-> TRUE]
         no == [orders EXCEPT ![i] = o2]
     IN /\ orders' = no
        /\ realized' = realized + orders[i].dir * x
        /\ Reclassify(no, pdPerp, deadline)
  /\ UNCHANGED << pending, budget >>
  /\ UNCHANGED pdVars
  /\ UNCHANGED histVars

\* An order-lifecycle/finality event that collapses the interval with NO
\* execution: a cancel acknowledged with zero executions, or a terminal
\* status whose cumulative executed quantity has been reconciled through
\* the four-step contract's watermark. SM SS2 Scope names this case
\* explicitly -- it "emits no fill and no position event, yet fully
\* collapses that order's interval" -- which is why it is a first-class
\* action here and not a variant of OrderFill.
\*
\* The four-step per-venue finality contract (fence the dispatch path ->
\* terminal status -> sweep past a watermark -> reconcile to that
\* watermark) abstracts to THIS one barrier: it either collapses the
\* interval, or its deadline blows (DeadlineBlows). SS46 SS2 item 4.
OrderFinal(i) ==
  /\ orders[i].q > 0
  /\ LET no == [orders EXCEPT ![i] = DeadOrder]
     IN /\ orders' = no
        /\ ReclassifyOnFinality(no, pdPerp, deadline)
  /\ UNCHANGED << realized, pending, budget >>
  /\ UNCHANGED pdVars
  /\ UNCHANGED histVars

\* SM SS2 row 3: the contract cannot complete within its deadline
\* (queries unavailable / rate-limited) -> DeadlineExceededAmbiguous.
DeadlineBlows ==
  /\ cls = "Ambiguous"
  /\ deadline' = TRUE
  /\ cls' = "DeadlineExceeded"
  /\ UNCHANGED << orders, realized, pending, floorIssued, floorCount, budget >>
  /\ UNCHANGED pdVars
  /\ UNCHANGED histVars

(***************************************************************************)
(* The two compensating-trade branches.                                    *)
(***************************************************************************)

\* SM SS2 row 2, the DETERMINED branch: issue an ExposureReduction sized
\* from `realized_delta - outstanding_reserved_reductions`, tracked as a
\* fenced reduction generation.
\*
\* Note what stops a SECOND command for the same residual: not a guard
\* about generations, but the sizing itself -- once the first generation
\* is reserved, `realized - reserved` is zero and the action is disabled.
\* That is the whole mechanism of "coalesce / amend / cancel-replace it,
\* never a fresh command per event", and NC6 removes it.
IssueExposureReduction ==
  /\ budget > 0
  /\ FreePend # {}
  \* The CLASSIFICATION gates the compensating trade, not the raw delta
  \* (SM SS2 Scope; NC3 removes exactly this conjunct).
  /\ (BLIND_COMPENSATION \/ cls = "Determined")
  /\ ImplResidual # 0
  /\ LET k == CHOOSE k \in FreePend : TRUE
         amt == ImplResidual
     IN /\ pending' = [pending EXCEPT ![k] = Pend(amt, "exposure", TRUE)]
        /\ RecordDispatch(amt, "exposure")
  /\ budget' = budget - 1
  /\ UNCHANGED << orders, realized >>
  /\ UNCHANGED pdVars
  /\ UNCHANGED epiVars

\* SM SS2 row 5 + the Interval-floor rule: the deadline-exceeded branch's
\* SOLE exception to "no compensation before finality". The magnitude is
\* the endpoint of the summed interval NEAREST ZERO, which is naked in
\* EVERY reachable state; it is a distinct RESERVED generation, so a
\* still-in-flight floor is not re-read as fresh naked exposure; and it is
\* never issued twice.
IssueFloor ==
  /\ budget > 0
  /\ FreePend # {}
  /\ cls = "DeadlineExceeded"
  /\ (FLOOR_ONCE => ~floorIssued)
  /\ (STRADDLE_GUARD => ~ImplStraddles)
  /\ LET amt ==
           IF ImplLo >= 0
             THEN (IF FLOOR_NEAREST_ZERO THEN ImplLo ELSE ImplHi)
           ELSE IF ImplHi <= 0
             THEN (IF FLOOR_NEAREST_ZERO THEN ImplHi ELSE ImplLo)
           \* Straddling: reachable only with STRADDLE_GUARD = FALSE
           \* (NC5). The nearest-zero endpoint BY MAGNITUDE is the
           \* plausible wrong implementation -- it is what an author who
           \* dropped the straddle test would write next.
           ELSE (IF Abs(ImplLo) <= Abs(ImplHi) THEN ImplLo ELSE ImplHi)
         k == CHOOSE k \in FreePend : TRUE
     IN /\ amt # 0
        /\ pending' = [pending EXCEPT ![k] = Pend(amt, "floor", TRUE)]
        /\ RecordDispatch(amt, "floor")
  /\ floorIssued' = TRUE
  /\ floorCount' = floorCount + 1
  /\ budget' = budget - 1
  /\ UNCHANGED << orders, realized, cls, deadline >>
  /\ UNCHANGED pdVars

(***************************************************************************)
(* How a reserved generation resolves.                                     *)
(***************************************************************************)

\* The generation reaches finality fully executed: the committed delta
\* becomes realized delta and the reservation is released. The TRUE net
\* range is unchanged across this step (realized and the ledger move
\* together) -- the second half of INV1b's small theorem.
\*
\* The pkg-spot producer is excluded here and resolves through PdSpotFinal
\* instead: SM SS7 gates its dispatch on the perp reduction's finality, so a
\* reserved-but-not-yet-authorized spot leg cannot fill, and "only the
\* RESERVED spot reduction (never a policy-selected re-hedge) completes the
\* generation" makes its resolution a generation event rather than a plain
\* reduction's.
ReductionFills(k) ==
  /\ pending[k].active
  /\ pending[k].kind # "pkgspot"
  /\ ~pending[k].stale
  \* SLICE C: a generation whose result has already come back ambiguous
  \* cannot now report a clean fill. Nominally vacuous (`amb` is FALSE in
  \* every reachable state); under NC16 it is what leaves the re-send as
  \* the only edge out, which is the defect.
  /\ ~pending[k].amb
  /\ realized' = realized - pending[k].amt
  \* RELEASE_ON_FINALITY = FALSE (NC11) keeps the entry in the ledger after
  \* its counterpart has realized. It is MARKED rather than silently left
  \* alone, so the invariant can name the condition instead of inferring it
  \* from an arithmetic discrepancy it is not allowed to see.
  /\ pending' = [pending EXCEPT ![k] =
                   IF RELEASE_ON_FINALITY THEN DeadPend
                   ELSE [pending[k] EXCEPT !.stale = TRUE]]
  /\ UNCHANGED << orders, budget >>
  /\ UNCHANGED pdVars
  /\ UNCHANGED epiVars
  /\ UNCHANGED histVars

\* SM SS2 row 4: the reduction's OWN result is ambiguous (timeout /
\* partial fill), and "it never blind-retries -- its own ambiguous result
\* routes through this same SS6.7 finality contract". So the reservation
\* is released and the generation becomes an unresolved order in its own
\* right, contributing the interval [-amt, 0] (or [0, -amt] for a short
\* reduction): somewhere between nothing and all of it executed.
\* `seen = FALSE` -- nothing has been observed executing on it, which is
\* precisely the "unobserved quantity" INV7 is about.
\*
\* This is the recursion SS46 SS6 names, and why P4 exists in slice C:
\* non-termination here is a naked position held open indefinitely.
ReductionAmbiguous(k) ==
  /\ pending[k].active
  /\ pending[k].kind # "pkgspot"
  /\ ~pending[k].stale
  /\ ~pending[k].amb
  /\ FreeOrder # {}
  /\ LET i == CHOOSE i \in FreeOrder : TRUE
         a == pending[k].amt
         no == [orders EXCEPT ![i] =
                  [dir |-> IF a > 0 THEN -1 ELSE 1,
                   q   |-> Abs(a), seen |-> FALSE]]
     IN /\ orders' = no
        /\ pending' = [pending EXCEPT ![k] = DeadPend]
        /\ Reclassify(no, pdPerp, deadline)
  /\ UNCHANGED << realized, budget >>
  /\ UNCHANGED pdVars
  /\ UNCHANGED histVars

(***************************************************************************)
(* SLICE C: the BLIND RETRY (NC16 only).                                   *)
(*                                                                         *)
(* SM SS2 row 4 forbids exactly this: "it NEVER blind-retries, and its own *)
(* ambiguous result ... routes through this same SS6.7 finality contract". *)
(* Under BLIND_RETRY the machine takes the other path -- it re-sends the   *)
(* reduction on its own authority -- and the two actions below are that    *)
(* path, so the forbidden behaviour is REACHABLE and P4 has something to   *)
(* fail on. Both are disabled nominally.                                   *)
(*                                                                         *)
(* Why the ambiguity needed a STATE here when slices A/B modeled it as an  *)
(* EDGE: a re-send that changes nothing observable is a STUTTERING step,   *)
(* and weak fairness steps over stutter -- WF_vars(ReductionFills \/       *)
(* ReductionAmbiguous) would simply force the fill and P4 would pass with  *)
(* the defect installed. The `amb` bit makes the retry loop a genuine fair *)
(* LASSO: in the `amb` state both nominal exits are disabled, so the       *)
(* fairness premise (continuous enabledness) is not met and nothing forces *)
(* the generation out. That is also the honest shape of the defect -- a    *)
(* machine that keeps re-sending is not one whose fills are being starved. *)
(***************************************************************************)
ReductionResultAmbiguous(k) ==
  /\ BLIND_RETRY
  /\ pending[k].active
  /\ pending[k].kind # "pkgspot"
  /\ ~pending[k].stale
  /\ ~pending[k].amb
  /\ pending' = [pending EXCEPT ![k].amb = TRUE]
  /\ UNCHANGED << orders, realized, budget >>
  /\ UNCHANGED pdVars
  /\ UNCHANGED epiVars
  /\ UNCHANGED histVars

\* The re-send. It spends NO dispatch budget on purpose: a machine that
\* blind-retries is not asking the finality contract for permission, and
\* charging the retry to MAX_DISPATCH would let the model's own bound
\* terminate the loop and hide the defect -- the budget trap of SS46 SS8g,
\* pointed the other way.
ReductionBlindRetry(k) ==
  /\ BLIND_RETRY
  /\ pending[k].active
  /\ pending[k].amb
  /\ pending' = [pending EXCEPT ![k].amb = FALSE]
  /\ UNCHANGED << orders, realized, budget >>
  /\ UNCHANGED pdVars
  /\ UNCHANGED epiVars
  /\ UNCHANGED histVars

(***************************************************************************)
(* SLICE B: PackageDelever generations (SM SS7), composed onto the SAME    *)
(* reservation ledger.                                                     *)
(*                                                                         *)
(* The composition is deliberately made of PARTS THAT ALREADY EXIST rather *)
(* than a parallel bookkeeping track:                                      *)
(*                                                                         *)
(*  - the perp-short reduction is an exposure-changing order on the hedge  *)
(*    group, so while it is in flight it contributes a possible-delta      *)
(*    interval [0, pdPerp] to the SAME sum SS2's classifier reads, and the *)
(*    residual is Ambiguous for the SAME reason any counter-leg makes it   *)
(*    ambiguous;                                                           *)
(*  - the reserved spot leg is a `pending` entry, one of the THREE         *)
(*    producers of the one subtraction, distinguished only by its kind.    *)
(*                                                                         *)
(* WHAT IS NOT RECORDED THROUGH RecordDispatch, and why: the generation's  *)
(* own legs are not SS2 compensating trades. SS2's single-gate invariant    *)
(* says PackageDelever ROUTES THROUGH the classifier -- and it does here,  *)
(* because its perp leg lands in the interval algebra and its spot leg     *)
(* lands in the ledger -- but neither leg compensates an uncertain         *)
(* residual, so INV1/INV3 are not the properties that own them. INV5 is.   *)
(* Recording them as dispatches would make INV3 fire on the generation     *)
(* completing normally, which is a property the spec does not state.       *)
(***************************************************************************)

\* A generation exists and has not finished.
PdActive  == pd \notin { "None", "Complete", "TerminalFlatten" }

\* The reserved SECOND leg has been submitted and has not reached finality.
\* This is precisely what SM SS7's two supersession rows require to be
\* quiesced to SS6.7 finality FIRST.
PdLegLive == pd \in { "SpotReduceInFlight", "Held" }

(***************************************************************************)
(* SLICE F -- `FM-8`'s PREMISE, and which quantity it is.                  *)
(*                                                                         *)
(* PdPlan's two entry conjuncts ARE the premise: `cls = "Determined"` and   *)
(* `TrueResidual = 0`. SM SS7 now says they are a STANDING condition, so    *)
(* the model needs them re-evaluated while a generation runs -- and the     *)
(* moment you try, you have to answer a question the clause does not:       *)
(* EVALUATED ON WHAT?                                                      *)
(*                                                                         *)
(* Both conjuncts are moved by the generation's OWN legs, by design:        *)
(*                                                                         *)
(*   - `cls` leaves "Determined" at PdSubmitPerp, because the perp reduce   *)
(*     is an exposure-changing order on the hedge group and enters SS2's    *)
(*     interval algebra. SS7 REQUIRES this -- it is how SS2 refuses every   *)
(*     compensating trade while the leg is live.                           *)
(*   - `TrueResidual` moves whenever the generation deliberately sells back *)
(*     less than it bought (PdAuthorizeSpot with s < the transient), which  *)
(*     is SS7's own "no more spot than the CONFIRMED perp reduction" and is *)
(*     the SAFE direction (net-long, never net-short -- INV5).             *)
(*                                                                         *)
(* So the premise is the entry guard evaluated NET OF THE GENERATION'S OWN  *)
(* LEGS: what the package's residual and classification would be if this    *)
(* generation were not running. That quantity is INVARIANT under every one  *)
(* of the generation's own actions -- PdPerpFill raises `realized` and      *)
(* amends the entry by the same x, PdAuthorizeSpot and PdSpotFinal move     *)
(* `pending` and the transient together -- which is the `FM-5` fix working, *)
(* stated as a quantity. It therefore moves ONLY when something external    *)
(* moves it, which is exactly what "the premise dissolved" means.          *)
(***************************************************************************)

\* The generation's own net contribution to the realized delta: perp bought,
\* spot not yet sold back.
PdOwnRealized   == pdDone - pdSold

\* The package's naked residual with this generation's own legs removed from
\* BOTH sides of SS2's subtraction. At entry (pdDone = pdSold = PdReserved =
\* 0) it is exactly `TrueResidual`, which is what makes this the entry guard
\* re-evaluated rather than a new rule.
PremiseResidual == (realized - PdOwnRealized) - (TrueReserved - PdReserved)

\* The classifier's verdict with the generation's own perp leg removed --
\* `ReportedDetermined` is asked what it would report at p = 0. It is the
\* REPORTED verdict rather than the true one because that is what the
\* machine acts on, and because NC4's defect must stay visible here too.
PremiseDetermined == ReportedDetermined(orders, 0)

\* The premise, intact. Read by INV10 and by the supersession's flag write,
\* NEVER through PREMISE_NETS_OWN_LEGS -- an invariant that moved with the
\* reading it is checking would let NC22 pass.
PremiseIntact == PremiseResidual = 0 /\ PremiseDetermined

\* What the machine treats as premise loss. The constant is read HERE, in
\* the guard, and nowhere an invariant can see.
PremiseDissolved ==
  IF PREMISE_NETS_OWN_LEGS THEN ~PremiseIntact ELSE cls # "Determined"

\* SS7's standing obligation as a GUARD on the generation's own protocol
\* steps. Attached to the steps that COMMIT NEW EXPOSURE -- submitting the
\* perp leg, authorizing a spot sale -- and NOT to:
\*
\*   - venue events (fills, finality) or the parks, because those are what a
\*     quiesce window is MADE of, and forbidding them would strand the
\*     reserved leg, which is the harm INV6 exists for;
\*   - `PdSubmitSpot`, which COMMITS NOTHING NEW: the amount was fixed at
\*     authorization and the entry is already in the shared ledger, so
\*     sending it is the completion of a decision already taken. Refusing to
\*     send it is exactly the strand INV6 forbids, one step earlier.
\*
\* THE LAST EXCLUSION WAS FOUND BY A RUN, NOT BY DESIGN, and it is the
\* nominal LIVENESS config that found it. With `PdSubmitSpot` guarded, TLC
\* returned a 16-state behaviour ending in STUTTERING: a generation buys 1
\* perp, authorizes ZERO spot (which RELEASES its ledger entry, so its
\* transient becomes a genuine naked residual), SS2's determined branch
\* correctly compensates that residual, and the package ends PHYSICALLY FLAT
\* and `Determined` -- while the generation sits at `SpotReduceAuthorized`
\* unable to finish, because the premise reads dissolved and the premise-lost
\* supersession is deliberately unfair. Blocking a step that commits nothing
\* buys no safety and costs termination.
PremiseOK == PREMISE_IS_STANDING => ~PremiseDissolved

\* The flag write those same steps share. It reads `PremiseIntact` -- the
\* net quantity -- and NOT `PremiseDissolved`, so neither constant can move
\* what the invariant means. Same discipline as ImplReserved/TrueReserved:
\* the guard computes on the implementation's reading, the flag records the
\* truth, and a control cannot corrupt its own judge.
\* `toSpot` marks the SECOND LEG's decisions FOR A NONZERO QUANTITY, for
\* W5's question alone. The nonzero part is not fussiness -- it is this
\* file's own W1 finding applied to its newest probe. W5's first witness
\* authorized a spot leg of size ZERO (the perp leg had confirmed nothing,
\* so SpotCap was 0): a decision taken on a dissolved premise, correctly
\* flagged by INV9, that moves no money. Asking the money question means
\* the quantity has to be in the flag, because the shortest counterexample
\* will always be the empty one.
AdvancesGeneration(toSpot) ==
  /\ premiseAdvanced' = (premiseAdvanced \/ ~PremiseIntact)
  /\ premiseAdvancedToSpot' =
       (premiseAdvancedToSpot \/ (toSpot /\ ~PremiseIntact))
  /\ UNCHANGED << sm2Hist, supersededWithLiveLeg, supersededOnIntactPremise >>

\* The spot leg's size cap. SM SS7: "sells no more spot than the CONFIRMED
\* perp reduction". An authorization taken before perp finality (NC9) has
\* no confirmed quantity in existence, so the generation's target is all
\* there is to size from -- which is why PERP_FIRST and SPOT_CAPPED_BY_PERP
\* are not two knobs on one number.
SpotCap == IF SPOT_CAPPED_BY_PERP /\ pd = "PerpReduceFinal"
             THEN pdDone
             ELSE pdTarget

\* SM SS7 Planned: "a generation is created" carrying "explicit per-leg
\* target positions", entered from the liq-distance/top-up ladder.
\*
\* The two entry conjuncts are SS7's own precondition, not convenience:
\* core SS5.3 defines PackageDelever as reducing leverage "on a STILL-NEUTRAL
\* package", and SS7 Invariants say "the package has NO NAKED RESIDUAL".
\* A generation opened over an unresolved or nonzero residual is a different
\* scenario that SS7 does not specify, and letting the model open one there
\* would manufacture violations of properties nobody claimed.
PdPlan(t) ==
  /\ pd = "None"
  /\ superseded = "none"
  /\ gens < MAX_GEN
  /\ cls = "Determined"
  /\ TrueResidual = 0
  /\ t \in 1..MAXQ
  /\ pd' = "Planned"
  /\ pdTarget' = t
  /\ pdDone' = 0
  /\ pdSold' = 0
  /\ gens' = gens + 1
  \* SM SS7 FLAG-3: "the reservation stays generation-scoped FROM CREATION,
  \* not first made here [SpotReduceAuthorized]". The ENTRY is created here
  \* under both readings; what they disagree about is its AMOUNT.
  \*   nominal  -- the generation's OUTSTANDING NET INTENT: it will buy the
  \*               perp and sell the same back, so at creation, with nothing
  \*               bought yet, the outstanding commitment is ZERO. It grows
  \*               with the perp leg (PdPerpFill amends it).
  \*   NC7      -- the generation's TARGET, counted immediately.
  /\ FreePend # {}
  /\ pending' = [pending EXCEPT ![CHOOSE k \in FreePend : TRUE] =
                   Pend(IF RESERVE_AT_CREATION THEN t ELSE 0, "pkgspot", FALSE)]
  /\ UNCHANGED << orders, realized, pdPerp, superseded, budget >>
  /\ UNCHANGED epiVars
  /\ UNCHANGED histVars

\* SM SS7: "the trigger is the perp short nearing liquidation on a rally, so
\* it reduces the perp short FIRST (buy-to-reduce -- which itself relieves
\* the liq risk)". The submission makes the residual Ambiguous, which is the
\* composition doing its work: SS2 now refuses every compensating trade until
\* this leg reaches finality.
PdSubmitPerp ==
  /\ pd = "Planned"
  /\ PremiseOK
  /\ pd' = "PerpReduceInFlight"
  /\ pdPerp' = pdTarget
  /\ Reclassify(orders, pdTarget, deadline)
  /\ UNCHANGED << orders, realized, pending, pdTarget, pdDone, pdSold,
                  gens, superseded, budget >>
  /\ AdvancesGeneration(FALSE)

\* The perp reduction executes x. Buying to reduce a short RAISES delta, so
\* this is where the intended net-LONG transient is created.
\* The transient it creates is committed to be sold back, so the ledger entry
\* is AMENDED to match it in the same step -- "coalesce / amend /
\* cancel-replace it, never a fresh command per event" (SM SS2), applied to
\* the reservation rather than to the order. Keeping the entry equal to the
\* outstanding net intent at every instant is what keeps
\* `realized - reserved` equal to the genuinely naked residual WHILE the
\* generation runs, which is the composition's whole claim: SS7's
\* "the SS6.7 classifier subtracts the reservation and does NOT misread it as
\* a naked residual to re-hedge".
\* Under RESERVE_AT_CREATION (NC7) the entry already carries the target and
\* is left alone, which is what that reading means.
PdPerpFill(x) ==
  /\ pd = "PerpReduceInFlight"
  /\ pdPerp > 0
  /\ x \in 1..pdPerp
  /\ realized' = realized + x
  /\ pdPerp' = pdPerp - x
  /\ pdDone' = pdDone + x
  /\ pending' = (IF RESERVE_AT_CREATION \/ PdPendIds = {} THEN pending
                 ELSE [pending EXCEPT ![CHOOSE k \in PdPendIds : TRUE] =
                         Pend(pdDone + x - pdSold, "pkgspot", FALSE)])
  /\ Reclassify(orders, pdPerp - x, deadline)
  /\ UNCHANGED << orders, pd, pdTarget, pdSold, gens, superseded, budget >>
  /\ UNCHANGED histVars

\* SM SS7: "confirms that reduction to finality (SS6.7) before any spot sell"
\* -- the same four-step contract abstracted to the same one barrier as
\* OrderFinal. It collapses the leg's remaining interval WITHOUT executing
\* it, so pdDone can be strictly less than pdTarget: that is the partial
\* confirmation the whole "no more spot than the CONFIRMED perp reduction"
\* rule is about, and the state SPOT_CAPPED_BY_PERP needs in order to bite
\* at all (checklist item 1, probe PC18).
\* SLICE C: this is the SAME barrier as OrderFinal, so it reads the same
\* constant (NC13). Supersede's Reclassify is deliberately NOT covered by
\* it -- that is an arbitration event, not a venue finality event -- and
\* ReductionAmbiguous's is not either, which is what keeps NC13's stall
\* attributable to the collapse-with-no-execution case PC8 names.
\* SLICE E: `HeldPerp` is in the from-set for the same reason `Held` is in
\* PdSpotFinal's -- FLAG-4 rules the park NONTERMINAL and "eventual finality
\* of the held leg resumes the remaining target". Here the held leg IS the
\* perp leg, so ITS barrier is the one that resumes. HELD_RESUMES = FALSE
\* (NC15) takes this edge out too, which is the same reading applied to the
\* same ruling: the control is about FLAG-4, not about which leg parked.
PdPerpFinal ==
  /\ pd \in (IF HELD_RESUMES THEN { "PerpReduceInFlight", "HeldPerp" }
                             ELSE { "PerpReduceInFlight" })
  /\ pdPerp' = 0
  /\ pd' = "PerpReduceFinal"
  /\ ReclassifyOnFinality(orders, 0, deadline)
  /\ UNCHANGED << orders, realized, pending, pdTarget, pdDone, pdSold,
                  gens, superseded, budget >>
  /\ UNCHANGED histVars

\* SM SS7: PerpReduceFinal -> "fsync the generation-scoped authorization
\* record" -> SpotReduceAuthorized (FLAG-3's durable record). The spot leg's
\* size is fixed HERE, from the confirmed perp reduction.
\*
\* s ranges over 0..cap rather than being pinned at the cap on purpose: with
\* s always equal to the confirmed reduction, "no more than" and "exactly"
\* are the same predicate and the inequality in SS7's own sentence would be
\* unchecked (checklist item 3). s = 0 is the honest extreme -- a perp
\* reduction that confirmed nothing authorizes no spot sale at all.
\*
\* The entry is AMENDED where one already exists rather than a second one
\* added: SS2's fenced-generation rule is "coalesce / amend / cancel-replace
\* it, never a fresh command per event", and a second entry for one
\* commitment is the double-count INV4 forbids.
PdAuthorizeSpot(s) ==
  /\ pd \in (IF PERP_FIRST THEN {"PerpReduceFinal"} ELSE {"Planned", "PerpReduceFinal"})
  /\ PremiseOK
  /\ s \in 0..SpotCap
  /\ pd' = "SpotReduceAuthorized"
  /\ \/ /\ PdPendIds # {}
        /\ pending' = [pending EXCEPT ![CHOOSE k \in PdPendIds : TRUE] =
                         IF s = 0 THEN DeadPend ELSE Pend(s, "pkgspot", FALSE)]
     \/ /\ PdPendIds = {}
        /\ \/ /\ s = 0
              /\ pending' = pending
           \/ /\ s > 0
              /\ FreePend # {}
              /\ pending' = [pending EXCEPT
                               ![CHOOSE k \in FreePend : TRUE] = Pend(s, "pkgspot", FALSE)]
  /\ UNCHANGED << orders, realized, pdTarget, pdPerp, pdDone, pdSold,
                  gens, superseded, budget >>
  /\ UNCHANGED epiVars
  /\ AdvancesGeneration(s > 0)

\* SM SS7: "submit the reserved spot reduction". A zero-size authorization
\* has nothing to submit and the generation is done.
\* SLICE F: deliberately NOT guarded by PremiseOK, and the reason is in
\* PremiseOK's own comment -- this step commits nothing new (the amount was
\* fixed at authorization, the entry is already in the shared ledger), so
\* refusing it strands the reserved leg instead of preventing anything. The
\* nominal LIVENESS config is what established that: guarded, it stutters.
PdSubmitSpot ==
  /\ pd = "SpotReduceAuthorized"
  /\ pd' = (IF PdPendIds = {} THEN "Complete" ELSE "SpotReduceInFlight")
  \* The reservation stops being an intention here and becomes an order that
  \* is actually out. Nothing else about the entry changes -- the AMOUNT was
  \* fixed at authorization; what changes is whether other machinery may net
  \* against it as a movement rather than a plan.
  /\ pending' = (IF PdPendIds = {} THEN pending
                 ELSE [pending EXCEPT
                         ![CHOOSE k \in PdPendIds : TRUE].sent = TRUE])
  /\ UNCHANGED << orders, realized, pdTarget, pdPerp, pdDone, pdSold,
                  gens, superseded, budget >>
  /\ UNCHANGED epiVars
  /\ UNCHANGED histVars

\* SM SS7: "a leg that is deadline-exceeded/ambiguous HOLDS AND ALERTS,
\* never blind-retries or blind-flattens" -- and FLAG-4 rules Held/Alert
\* NONTERMINAL, so the exit is finality (PdSpotFinal) or TerminalFlatten.
PdSpotHolds ==
  /\ pd = "SpotReduceInFlight"
  /\ pd' = "Held"
  /\ UNCHANGED << orders, realized, pending, pdTarget, pdPerp, pdDone, pdSold,
                  gens, superseded, budget >>
  /\ UNCHANGED epiVars
  /\ UNCHANGED histVars

\* SLICE E: the OTHER entry into Held/Alert, which SM SS7's transition row
\* has had all along and slices A-D did not model (SS46 SS10a's correspondence
\* gap). The PERP leg is the deadline-exceeded/ambiguous one.
\*
\* SS10a justified the narrowing on the ground that a perp-side park is the
\* HARMLESS instance -- "at the perp leg neither the unsold transient nor a
\* nonzero reservation exists yet (pdDone = 0)". BUILDING IT SHOWS THAT
\* REASONING IS WRONG, and the model itself is what says so: PdPerpFill fires
\* while pd = "PerpReduceInFlight" and AMENDS the ledger entry upward, so a
\* perp leg may partially fill and THEN go ambiguous -- which is SS2's own
\* `PartiallyFilled` case ("explicitly means more fills possible"). Such a
\* park holds a REAL net-long transient (realized raised by pdDone) AND a
\* NONZERO reservation, and it blocks the next generation on SS7's
\* "only after the prior generation reaches SS6.7 finality" gate. pdDone = 0
\* is only the park taken BEFORE any fill; it is one instance, not the case.
\*
\* Unfair, exactly like PdSpotHolds: it is the degradation that creates P5's
\* antecedent, and a fair one would assume the very degradation P5 survives.
PdPerpHolds ==
  /\ pd = "PerpReduceInFlight"
  /\ pd' = "HeldPerp"
  /\ UNCHANGED << orders, realized, pending, pdTarget, pdPerp, pdDone, pdSold,
                  gens, superseded, budget >>
  /\ UNCHANGED epiVars
  /\ UNCHANGED histVars

\* The reserved spot reduction reaches SS6.7 finality with x actually
\* executed (x <= the reserved size; a partial fill's remainder is not
\* re-sent -- SS7's "never blind-retries"). Reachable from SpotReduceInFlight
\* directly and from Held, which is FLAG-4's ruling: eventual finality of the
\* held leg resumes the generation.
\* SLICE C: `Held` is in the from-set because FLAG-4 rules the park
\* NONTERMINAL -- "eventual finality of the held leg resumes the
\* generation". HELD_RESUMES = FALSE (NC15) takes it back out, which is the
\* park read as terminal-pending-policy, and P3 is what notices.
\* SLICE E: HELD_EXIT_CAUSE_BLIND (NC17) adds `HeldPerp` to the from-set --
\* the reserved SPOT leg's barrier discharging a park whose PERP leg is the
\* unresolved one. It is reachable rather than absurd because the "pkgspot"
\* entry EXISTS from creation (FLAG-3), so PdPendIds is nonempty at HeldPerp
\* and the guard below passes; under the nominal reading its amount is the
\* outstanding net intent, so x ranges over 0..that. What it produces is a
\* generation in `Complete` -- SS7's "the generation has reached SS6.7
\* finality; only now is a next generation admissible" -- with its perp
\* reduction still live. INV8 is what says so.
PdSpotFinal(x) ==
  /\ pd \in ( (IF HELD_RESUMES THEN { "SpotReduceInFlight", "Held" }
                               ELSE { "SpotReduceInFlight" })
              \cup (IF HELD_EXIT_CAUSE_BLIND THEN { "HeldPerp" } ELSE {}) )
  /\ PdPendIds # {}
  /\ LET k == CHOOSE k \in PdPendIds : TRUE
     IN /\ x \in 0..pending[k].amt
        /\ realized' = realized - x
        /\ pdSold' = pdSold + x
        /\ pending' = [pending EXCEPT ![k] =
                         IF RELEASE_ON_FINALITY THEN DeadPend
                         ELSE [pending[k] EXCEPT !.stale = TRUE]]
  /\ pd' = "Complete"
  /\ UNCHANGED << orders, pdTarget, pdPerp, pdDone, gens, superseded, budget >>
  /\ UNCHANGED epiVars
  /\ UNCHANGED histVars

\* SM SS7 / FLAG-4: "if completion is impossible or the liquidation policy's
\* imminence branch fires -> TerminalFlatten", whose full-flatten "routes
\* through the same SS6.7 finality contract, not a blind market dump".
\* The flatten's OWN orders are not modeled (they are a different scenario's
\* legs); what is modeled is that the generation ends and its reservation
\* stops being counted. A stated abstraction, not a claim.
\* SLICE E: FLAG-4's policy exit belongs to BOTH parks -- its ruling is about
\* Held/Alert, and SM SS7 enters that from either leg. Leaving `HeldPerp` out
\* would have stranded it whenever HELD_RESUMES is FALSE, and TLC would have
\* reported NC15 as a DEADLOCK rather than as the temporal violation it is --
\* the model objecting to the shape of the control instead of to the
\* behaviour (the distinction NC15's own config note draws).
PdTerminalFlatten ==
  /\ pd \in { "Held", "HeldPerp" }
  /\ pd' = "TerminalFlatten"
  /\ pending' = (IF PdPendIds = {} \/ ~RELEASE_ON_SUPERSEDE THEN pending
                 ELSE [pending EXCEPT ![CHOOSE k \in PdPendIds : TRUE] = DeadPend])
  /\ UNCHANGED << orders, realized, pdTarget, pdPerp, pdDone, pdSold,
                  gens, superseded, budget >>
  /\ UNCHANGED epiVars
  /\ UNCHANGED histVars

\* The generation ends; SM SS7 admits a next one only from here ("each
\* generation is a bounded attempt made ONLY AFTER the prior generation
\* reaches SS6.7 finality"). The per-generation counters reset, which is also
\* what makes INV5 a per-generation claim.
\* SLICE E -- `FM-6`: this is where the generation, and therefore its ledger
\* entry, is definitively gone, so this is where the residual SS2 reads
\* changes with no order event to announce it. `p = 0` matches this action's
\* own reset of pdPerp.
PdEnds ==
  /\ pd \in { "Complete", "TerminalFlatten" }
  /\ pd' = "None"
  /\ pdTarget' = 0 /\ pdPerp' = 0 /\ pdDone' = 0 /\ pdSold' = 0
  /\ ReclassifyOnLedgerChange(orders, 0, deadline)
  /\ UNCHANGED << orders, realized, pending, gens, superseded, budget >>
  /\ UNCHANGED histVars

\* SM SS7's two supersession rows: EmergencyClosePackage (core SS5.3) and the
\* FenceReadyLost arbitration (core SS7.4; SS3 here) each "supersede and
\* quiesce any active PackageDelever generation (cancelling its reserved
\* second leg to SS6.7 finality FIRST)".
\*
\* The flag is written UNCONDITIONALLY from the state at the supersession and
\* the GUARD is what prevents the bad one -- so relaxing the guard (NC10)
\* SETS the flag rather than hiding the event. Same shape as RecordDispatch.
Supersede(path) ==
  /\ pd # "None"
  /\ superseded = "none"
  /\ (QUIESCE_BEFORE_SUPERSEDE => ~PdLegLive)
  /\ supersededWithLiveLeg' = (supersededWithLiveLeg \/ PdLegLive)
  /\ superseded' = path
  /\ pd' = "None"
  /\ pdTarget' = 0 /\ pdPerp' = 0 /\ pdDone' = 0 /\ pdSold' = 0
  /\ pending' = (IF PdPendIds = {} \/ ~RELEASE_ON_SUPERSEDE THEN pending
                 ELSE [pending EXCEPT ![CHOOSE k \in PdPendIds : TRUE] = DeadPend])
  \* Dropping the in-flight perp leg's remaining interval is quiescence, not
  \* amnesia: it is OrderFinal's semantics (the leg reaches finality with no
  \* further executions), and everything it DID execute is already in
  \* `realized`.
  /\ Reclassify(orders, 0, deadline)
  \* SLICE F. The same shape one row down: written UNCONDITIONALLY from the
  \* state at the supersession, so a reading that fires the row spuriously
  \* SETS the flag rather than being absorbed. `PremiseIntact` is the
  \* net-of-own-legs quantity, never the constant -- see AdvancesGeneration.
  /\ supersededOnIntactPremise' =
       (supersededOnIntactPremise \/ (path = "PremiseLost" /\ PremiseIntact))
  /\ UNCHANGED advHist
  /\ UNCHANGED << orders, realized, gens, budget >>
  /\ UNCHANGED sm2Hist

(***************************************************************************)
(* Settled terminal. Deadlock checking stays ON deliberately: this is the  *)
(* ONLY terminal state the model predicts, so any other dead end is        *)
(* reported by TLC as a deadlock and is a finding, not noise. Slice B adds *)
(* `pd = "None"` to the conjunction rather than widening it: every active  *)
(* generation phase has an enabled successor, so a generation that could   *)
(* get STUCK must report as a deadlock instead of being absorbed into the  *)
(* terminal self-loop.                                                     *)
(***************************************************************************)
AllCollapsed == \A i \in OrderIds : orders[i].q = 0
NoPending    == \A k \in PendIds : ~pending[k].active

Settled ==
  /\ AllCollapsed /\ NoPending
  /\ pd = "None"
  /\ pdPerp = 0
  /\ (TrueResidual = 0 \/ budget = 0)
  /\ UNCHANGED vars

Next ==
  \* Fill sizes range over the whole quantity domain, not over MAXQ: a
  \* reduction's ambiguous residue can carry more remaining quantity than
  \* any Init counter-leg, and capping the fill size at MAXQ would make
  \* that residue only partially executable -- a bound with no counterpart
  \* in the spec.
  \/ \E i \in OrderIds : \E x \in QDom \ {0} : OrderFill(i, x)
  \/ \E i \in OrderIds : OrderFinal(i)
  \/ DeadlineBlows
  \/ IssueExposureReduction
  \/ IssueFloor
  \/ \E k \in PendIds : ReductionFills(k)
  \/ \E k \in PendIds : ReductionAmbiguous(k)
  \* --- SLICE C: disabled unless BLIND_RETRY (NC16) ------------------------
  \/ \E k \in PendIds : ReductionResultAmbiguous(k)
  \/ \E k \in PendIds : ReductionBlindRetry(k)
  \* --- SLICE B: the SS7 generation ---------------------------------------
  \/ \E t \in 1..MAXQ : PdPlan(t)
  \/ PdSubmitPerp
  \/ \E x \in QDom \ {0} : PdPerpFill(x)
  \/ PdPerpFinal
  \/ \E s \in QDom : PdAuthorizeSpot(s)
  \/ PdSubmitSpot
  \/ PdSpotHolds
  \/ PdPerpHolds
  \/ \E x \in QDom : PdSpotFinal(x)
  \/ PdTerminalFlatten
  \/ PdEnds
  \* The first two rows are EXTERNAL arbitration events: enabled against any
  \* active generation, because nothing in this model decides when a
  \* liquidation policy or a fence arbitration fires.
  \/ \E p \in SupPaths \ {"none", "PremiseLost"} : Supersede(p)
  \* --- SLICE F: the THIRD row, `FM-8`'s -----------------------------------
  \* Unlike the other two this one is TRIGGERED BY STATE, so it carries its
  \* own guard. `PdActive` and not `pd # "None"`: a generation already at
  \* Complete/TerminalFlatten has reached SS6.7 finality and there is nothing
  \* left to supersede.
  \*
  \* It IS in the fairness set, and the other two are not -- see `Fairness`.
  \* The first draft said the opposite, with a reason, and the nominal
  \* liveness config refuted it: control NC23 is that refutation, kept as a
  \* run.
  \/ (PREMISE_IS_STANDING /\ PdActive /\ PremiseDissolved
      /\ Supersede("PremiseLost"))
  \/ Settled

Spec == Init /\ [][Next]_vars

(***************************************************************************)
(* SLICE C: FAIRNESS, and the argument for every line of it.               *)
(*                                                                         *)
(* The split is the same one model (b) made and for the same reason: what  *)
(* the VENUE and the MACHINE promise is fair; what an adversary, a policy  *)
(* branch or a discretionary decision does is NOT. A fairness assumption   *)
(* is a premise of every P below, so a generous one is a weaker theorem --  *)
(* every action left out of this list is an action the properties are      *)
(* proved WITHOUT.                                                         *)
(*                                                                         *)
(* DELIBERATELY UNFAIR, each with its reason:                              *)
(*                                                                         *)
(*  - DeadlineBlows -- an environment degradation (queries unavailable /   *)
(*    rate-limited). Model (b) keeps crashes and degradations unfair. It   *)
(*    matters here more than it looks: DeadlineBlows is ENABLED in every   *)
(*    Ambiguous state and its target satisfies P1's consequent, so a fair  *)
(*    DeadlineBlows would discharge P1 by ASSUMING the very degradation    *)
(*    P1 is meant to survive. P1 is instead driven by the finality         *)
(*    barrier, which is the promise SS2 actually rests on.                 *)
(*  - IssueExposureReduction / IssueFloor -- the compensating trades. Left *)
(*    unfair for TWO reasons. (i) It makes P1-P4 stronger: no property     *)
(*    below asserts that a dispatch ever happens. (ii) It is what defuses  *)
(*    SS46 SS8g's budget trap AT THE SOURCE rather than with an antecedent: *)
(*    a behaviour that exhausts MAX_DISPATCH and then stutters in `Settled`*)
(*    with a nonzero residual falsifies NO property here, because none of  *)
(*    them says a residual is eventually compensated. A "the residual is   *)
(*    eventually flat" property WOULD have needed the budget conditioning; *)
(*    it is not one of SS46 SS4's four, and adding it would have been a    *)
(*    claim about the model's bound rather than about the spec.            *)
(*  - PdPlan -- a generation is opened by the liq-distance ladder, which   *)
(*    is a decision and out of model.                                      *)
(*  - PdSpotHolds -- the degradation that CREATES P3's antecedent.         *)
(*  - PdPerpHolds -- the same, for P5. SLICE E.                           *)
(*  - PdTerminalFlatten -- a policy branch (FLAG-4's "if completion is     *)
(*    impossible"). If it were fair it would discharge P3 by itself, and   *)
(*    P3 would be green with NC15's defect installed.                      *)
(*  - Supersede -- an external arbitration event (core SS5.3 / SS7.4).     *)
(*  - Settled -- the terminal stutter.                                     *)
(***************************************************************************)
Fairness ==
  \* The per-venue finality contract's own promise, per order slot: an
  \* order eventually executes or reaches a terminal status. The DISJUNCTION
  \* is the honest form -- fairness on OrderFinal alone would assume the
  \* no-execution path, and on OrderFill alone would assume away the cancel
  \* case SS2 Scope names. Model (b)'s WF_vars(VEntryFill \/ VEntryTerminal).
  /\ \A i \in OrderIds :
       WF_vars((\E x \in QDom \ {0} : OrderFill(i, x)) \/ OrderFinal(i))
  \* A reduction generation reaches finality one way or the other: it fills,
  \* or its own result is ambiguous and routes back through the contract.
  \* Which of the two is the VENUE's choice, never the machine's, so the
  \* fairness is on the disjunction and P4 is not handed its conclusion.
  /\ \A k \in PendIds : WF_vars(ReductionFills(k) \/ ReductionAmbiguous(k))
  \* The SS7 generation's own protocol steps -- the machine's obligations
  \* once a generation exists. Model (b)'s WF over the engine's arbitration
  \* and the actuator's protocol.
  /\ WF_vars(PdSubmitPerp)
  /\ WF_vars((\E x \in QDom \ {0} : PdPerpFill(x)) \/ PdPerpFinal)
  /\ WF_vars(\E s \in QDom : PdAuthorizeSpot(s))
  /\ WF_vars(PdSubmitSpot)
  \* The reserved spot leg's finality -- the same barrier as OrderFinal,
  \* and the ONLY fair exit from `Held`. P3's whole content is that this
  \* barrier is what resumes a held generation (FLAG-4), so its fairness is
  \* the finality promise and not a policy assumption.
  /\ WF_vars(\E x \in QDom : PdSpotFinal(x))
  /\ WF_vars(PdEnds)
  \* SLICE F. SM SS7's THIRD supersession row, and it is in this set while the
  \* other two are deliberately OUT -- which is not an inconsistency but the
  \* same distinction `Next` already draws. `EmergencyClosePackage` and
  \* `FenceReadyLost` are EXTERNAL ARBITRATION EVENTS: nothing in this model
  \* decides when a liquidation policy or a fence arbitration fires, so
  \* assuming they eventually do would be assuming away the environment. The
  \* premise-lost row is a MACHINE OBLIGATION triggered by state -- the spec
  \* says the generation IS superseded, not MAY BE -- so it belongs beside
  \* the machine's other protocol steps above.
  \*
  \* IT WAS LEFT OUT ON THE OPPOSITE ARGUMENT AND THE NOMINAL LIVENESS RUN
  \* REFUTED IT. The reasoning was that fairness here would hand P3/P5 an exit
  \* their own premises do not contain. What TLC returned instead was a
  \* 20-state behaviour STUTTERING at `PerpReduceFinal`: a reduction's own
  \* ambiguous result had released its ledger entry (`FM-8`'s exact
  \* scenario), so the premise was genuinely dissolved, `PdAuthorizeSpot` was
  \* correctly blocked, the dispatch budget was spent so SS2 could not
  \* compensate either -- and the one action that could end it was admissible
  \* and never taken. P5 violated. A standing obligation nothing is obliged
  \* to discharge is not a standing obligation.
  \*
  \* P3 is unaffected by construction rather than by luck: in `Held` the
  \* reserved leg is live, so QUIESCE_BEFORE_SUPERSEDE blocks this action
  \* outright and P3's consequent (`Complete`/`TerminalFlatten`) admits no
  \* supersession anyway.
  \* NC23 removes exactly this conjunct and nothing else, so what it
  \* demonstrates is attributable to the fairness and not to a state change:
  \* the two models have the SAME state graph, and only the behaviours
  \* differ.
  /\ (PREMISE_SUPERSESSION_FAIR =>
        WF_vars(PREMISE_IS_STANDING /\ PdActive /\ PremiseDissolved
                /\ Supersede("PremiseLost")))

LiveSpec == Spec /\ Fairness

(***************************************************************************)
(* Invariants.                                                             *)
(***************************************************************************)

TypeOK ==
  /\ orders \in [ OrderIds -> [dir: Dirs, q: QDom, seen: BOOLEAN] ]
  /\ realized \in AmtDom
  /\ pending \in [ PendIds -> [active: BOOLEAN, amt: AmtDom, kind: Kinds,
                               stale: BOOLEAN, sent: BOOLEAN, amb: BOOLEAN] ]
  /\ pd \in PdPhases
  /\ pdTarget \in QDom /\ pdPerp \in QDom
  /\ pdDone \in QDom /\ pdSold \in QDom
  /\ gens \in 0..MAX_GEN
  /\ superseded \in SupPaths
  /\ supersededWithLiveLeg \in BOOLEAN
  /\ premiseAdvanced \in BOOLEAN
  /\ premiseAdvancedToSpot \in BOOLEAN
  /\ supersededOnIntactPremise \in BOOLEAN
  /\ cls \in Classes
  /\ deadline \in BOOLEAN
  /\ floorIssued \in BOOLEAN
  /\ floorCount \in 0..MAX_DISPATCH
  /\ overshoot \in BOOLEAN
  /\ longReduced \in BOOLEAN /\ shortReduced \in BOOLEAN
  /\ dispatchedWhileAmbiguous \in BOOLEAN
  /\ nonFloorInDeadline \in BOOLEAN
  /\ floorWhileStraddling \in BOOLEAN
  /\ budget \in 0..MAX_DISPATCH

\* INV1 -- NeverOvershootSign. SM SS2 acceptance case: "no naked-short
\* created and no doubled/over-reduced short across EVERY reachable venue
\* state"; Interval-floor rule: "reducing exposure by exactly that
\* magnitude can never overshoot into the opposite sign under any
\* resolution".
\*
\* Phrased over RESOLUTIONS, never over the endpoint formula (SS46 SS6):
\* the endpoint arithmetic lives in IssueFloor and this property does not
\* mention it. Two conjuncts, deliberately:
\*
\*  (a) AT DISPATCH -- `overshoot` is set from the state at every dispatch
\*      whether or not the guards allowed it, so relaxing a guard makes
\*      the flag fire rather than hiding the dispatch.
\*
\*  (b) PERSISTENTLY -- once a reduction against a long residual has been
\*      dispatched, no later state may make a short residual reachable
\*      (and mirror). This exists to CHECK the argument that (a) is
\*      sufficient rather than to trust it: fills leave the true net range
\*      invariant, finality only shrinks it, and an ambiguous reduction
\*      result widens it only back toward the pre-dispatch side -- so no
\*      later event should be able to introduce an opposite sign that a
\*      dispatch did not. If (b) fires where (a) did not, that reasoning
\*      was wrong, which is a finding either way.
\* SLICE B SPLIT THE TWO CONJUNCTS APART, and the reason is a MODEL finding
\* rather than a weakening. See FM-model-1 in SS46 SS8e.
\*
\* Conjunct (a), `~overshoot`, is the faithful mechanization and stays in
\* every configuration.
\*
\* Conjunct (b) rested on a small theorem about the SLICE A event set --
\* fills leave the true net range invariant, finality only shrinks it, an
\* ambiguous reduction result widens it only back toward the pre-dispatch
\* side -- whose premise is SS46 SS7e's stated bound, IN THESE WORDS: "no new
\* counter-leg order appears mid-episode ... so the only new intervals are
\* the model's own reductions' residues". SS7's perp-short reduction IS a new
\* exposure-changing order appearing mid-episode. It is authorized
\* independently of any dispatch, its transient is net-long BY DESIGN (SS7's
\* whole sequencing rule), and its own safety property is INV5.
\*
\* So in the COMPOSED model, conjunct (b) asserts "once a short has been
\* reduced, no other authorized command may ever make long exposure
\* reachable" -- which the spec nowhere says and which would forbid
\* PackageDelever outright. It is not weakened; it is checked where its
\* premise holds, in the MAX_GEN = 0 configuration, which is slice A's
\* model exactly. Deleting it instead would have cost real bite: SS46 SS7a
\* measured that (b) catches NC1 and NC6 on its own.
INV1_NeverOvershootSign == ~overshoot

INV1b_NoLaterOppositeSign ==
  /\ (longReduced  => TrueLo >= 0)
  /\ (shortReduced => TrueHi <= 0)

\* INV2 -- FloorIssuedAtMostOnce. SM SS2 row 5 / Interval-floor rule:
\* "never issued twice ... (else, for [0.4, 1.0], two uncertain 0.4
\* reductions could reach -0.4 and defeat the proof)". COUNTED FROM
\* DISPATCHES, never from `floorIssued` (SS46 SS3's named trap).
INV2_FloorIssuedAtMostOnce == floorCount <= 1

\* INV3 -- NoUncertainCompensation. SM SS2 Invariants (Single gate): "no
\* path compensates the UNCERTAIN PART of an ambiguous residual before the
\* finality barrier (only the deadline-exceeded interval-floor generation
\* may compensate the reachable-state-invariant floor)"; row 6: a
\* straddling interval permits "de-minimis only".
INV3_NoUncertainCompensation ==
  /\ ~dispatchedWhileAmbiguous
  /\ ~nonFloorInDeadline
  /\ ~floorWhileStraddling

\* INV7 -- DeterminedIsReallyDetermined. SM SS2 `Determined`: the exposure
\* is fixed "because no exposure-changing order on the hedge group has any
\* remaining executable OR UNOBSERVED quantity". Guards against a
\* classifier that reads the fill ledger alone.
\*
\* Honesty note: under the nominal constants this holds BY CONSTRUCTION --
\* ReportedDetermined is TrulyDetermined. Its evidence is therefore NC4,
\* not the nominal run; what NC4 shows is that the classifier's
\* correctness is load-bearing, since the same flip also makes INV1
\* reachable through a determined dispatch over an unobserved interval.
INV7_DeterminedIsReallyDetermined ==
  cls = "Determined" => (\A i \in OrderIds : orders[i].q = 0) /\ pdPerp = 0

(***************************************************************************)
(* SLICE B invariants -- the composition (SS46 SS2.1).                      *)
(***************************************************************************)

\* THE INVARIANT THAT WAS TRIED FIRST AND IS WRONG -- kept, unused, because
\* a rejected formulation nobody wrote down gets re-proposed (SS46 SS8e,
\* FM-model-2).
\*
\* The idea was: a reservation must never commit to removing delta that does
\* not exist, so the summed ledger is dominated by the realized delta. It is
\* true throughout slice A, and it catches the FM-5 defect. It is
\* nevertheless FALSE of correct composed behaviour, and TLC found the
\* counterexample in six steps: a group short 2, a generation whose perp leg
\* buys 1 (moving delta TOWARD zero), and a reserved spot sale of 1 that
\* undoes exactly that -- `realized = -1` with `TrueReserved = +1`.
\*
\* The error is the premise that a reservation is always backed by the NET
\* delta. PackageDelever's reserved leg is backed by the generation's own
\* transient, and legitimately moves the group's delta AWAY from zero,
\* because it is restoring a position the perp leg temporarily reduced. No
\* inequality between the ledger's sum and `realized` survives that, which
\* is why INV4 below is phrased over the HARM and the ENTRY instead.
ReservationsBackedREJECTED ==
  /\ (TrueReserved > 0 => realized >= TrueReserved)
  /\ (TrueReserved < 0 => realized <= TrueReserved)

\* An UNSENT reservation is an INTENTION: no order is out, so nothing will
\* move delta on its account until one is. It may therefore only be booked
\* against delta the generation HAS ALREADY MOVED -- the perp leg's
\* confirmed reduction net of what has been sold back.
\*
\* This is FM-5 stated generally. Booking the generation's TARGET at
\* creation (NC7) books an intention against a movement that has not
\* happened, so `realized - reserved` reports a residual of the opposite
\* sign on a package that is neutral, and SS2's determined branch buys back
\* exactly the exposure the generation is about to buy itself.
\*
\* It is scoped to the pre-authorization phases deliberately. Once SS7 has
\* SIZED the spot leg, the governing rule is its own cap ("no more spot than
\* the confirmed perp reduction") and that is INV5's claim -- so NC8 trips
\* INV5 alone rather than both, and each control still names one invariant.
\*
\* Not a restatement of the mechanism (SS46 SS3's trap): three different
\* expressions write that amount -- 0 at creation, a running
\* `pdDone + x - pdSold` per fill, and `s` at authorization -- and the
\* invariant is an INEQUALITY with genuine slack, not the formula any of
\* them uses.
IntentsBacked ==
  \A k \in PendIds :
    ( /\ pending[k].active
      /\ ~pending[k].stale
      /\ ~pending[k].sent
      /\ pd \in { "Planned", "PerpReduceInFlight", "PerpReduceFinal" } )
    => pending[k].amt <= pdDone - pdSold

\* INV4 -- ReservationLedgerSound. SS46 SS4: "determined sizing
\* (realized - reserved) neither double-counts a reservation nor misses one;
\* no reserved transient is read as naked residual, and no naked residual is
\* masked by a reservation that has already completed."
\*
\* Three clauses, each with its own control:
\*
\*  (a) NO ORPHANED RESERVATION (NC12) -- a reserved spot leg with no
\*      generation to own it. An orphan subtracts from EVERY later sizing
\*      forever, which is SS46 SS4's "misses one" direction: a genuine naked
\*      residual reads as already-committed and is never compensated.
\*
\*  (b) INTENTS BACKED (NC7) -- an intention booked against delta the
\*      generation has not moved. This is SS46 SS4's "no reserved transient is
\*      read as naked residual" and "neither double-counts a reservation",
\*      and it is FM-5. It is stated at the LEDGER level and not as the
\*      trade it causes, because both harm-level phrasings were tried and
\*      both are wrong -- see UnwarrantedREJECTED. Note INV1 cannot see this
\*      defect at all: INV1 is phrased over True-side quantities, which are
\*      computed from the ledger, so they are wrong in exactly the states
\*      this is about.
\*
\*  (c) NO STALE ENTRY (NC11) -- SS46 SS4's "no naked residual is masked by a
\*      reservation that has already COMPLETED". A condition on the ENTRY,
\*      not a discrepancy between the two ledger sums: an invariant that
\*      compared ImplReserved to TrueReserved would be reading the
\*      implementation's arithmetic, the one thing the Impl/True split
\*      exists to make impossible.
\*
\* What is deliberately NOT here: an inequality bounding the ledger's sum by
\* the realized delta. It was tried, it is false of correct composed
\* behaviour, and ReservationsBackedREJECTED above records why.
INV4_ReservationLedgerSound ==
  /\ (pd = "None" => PdPendIds = {})
  /\ IntentsBacked
  /\ \A k \in PendIds : pending[k].active => ~pending[k].stale

\* INV5 -- PackageDeleverNeverNetShort. SM SS7 Invariants: the generation is
\* "sequenced so the transient never worsens the at-risk direction": perp
\* short reduced first, confirmed to finality, then "SELLS NO MORE SPOT THAN
\* THE CONFIRMED PERP REDUCTION -- the momentary exposure is thus net-LONG
\* (safe in the triggering rally), NEVER NET-SHORT."
\*
\* Two conjuncts, mirroring INV1's at-dispatch/persistent pair:
\*  (a) the OUTCOME -- what was actually sold never exceeds what was
\*      actually confirmed. Reads pdSold, which the guard never touches.
\*  (b) EVERY REACHABLE INTERLEAVING -- the still-outstanding reserved sale
\*      could execute in full at any moment, so the commitment must already
\*      satisfy the same bound. Without (b) the invariant would only catch a
\*      generation that had already gone net-short, i.e. after the harm.
INV5_PackageDeleverNeverNetShort ==
  /\ pdSold <= pdDone
  /\ (pdSold + PdReserved) <= pdDone

\* INV6 -- SupersessionQuiescesFirst. SM SS7 Invariants: "both superseding
\* paths (EmergencyClosePackage, core SS5.3; FenceReadyLost, core SS7.4)
\* cancel/quiesce the generation's reserved second leg TO SS6.7 FINALITY
\* FIRST." Read from the flag written at the supersession, never from the
\* guard that prevents it.
INV6_SupersessionQuiescesFirst == ~supersededWithLiveLeg

(***************************************************************************)
(* SLICE E invariant -- the second park (SS46 SS10a, SS11b).                 *)
(***************************************************************************)

\* SM SS7 `Complete`: "the generation has reached SS6.7 finality; ONLY NOW is
\* a next generation admissible", and the transition table gates the next
\* generation on exactly that. So a generation in `Complete` may not still
\* have an executable perp reduction outstanding -- `Complete` is a claim
\* about finality, and SS7's ladder READS it to decide whether to open
\* another attempt.
\*
\* It is stated over the PERP leg's remaining quantity rather than over the
\* phase history on purpose: the harm is executable quantity that a later
\* generation's sizing cannot see, not the bookkeeping of which states were
\* visited.
\*
\* ISOLATION, which is the part worth checking rather than assuming (the
\* "a probe two gates can see" rule): NC9 (PERP_FIRST = FALSE) also produces
\* a spot sale with no confirmed perp reduction, so an invariant phrased over
\* the AUTHORIZATION would be violated by both controls and would show that
\* something objected rather than that this rule did. This one stays GREEN
\* under NC9 -- there the perp leg was never submitted, so pdPerp is 0 -- and
\* is violated only by a park exited through the wrong leg's barrier.
INV8_CompleteMeansLegsFinal == pd = "Complete" => pdPerp = 0

(***************************************************************************)
(* SLICE F invariants -- `FM-8`'s ruling, and the reading it is ruled in.   *)
(***************************************************************************)

\* INV9 -- SM SS7 (v0.9.25): "'no naked residual' is a STANDING condition,
\* not only an entry test." The generation MUST NOT continue on a premise
\* that has stopped holding.
\*
\* THE FORM IS THE CLAIM. It is not `PdActive => PremiseIntact`, and that is
\* not a weakening: the dissolved state is REACHABLE and LEGAL for as long as
\* SS7's own quiesce obligation takes -- the reserved second leg must reach
\* SS6.7 finality FIRST, so a window in which the premise is gone and the
\* generation still exists is exactly what the spec MANDATES. A state
\* predicate would flag the quiesce window and the harm alike and could only
\* be satisfied by a model that violated INV6. What is forbidden is the
\* machine taking the generation FURTHER -- submitting a leg, authorizing a
\* sale -- and that is an event, so the invariant reads a flag written at it.
\*
\* Same reasoning as INV6, one row down, which is why FM-8's ruling gave the
\* third supersession row no discipline of its own.
INV9_GenerationNeverAdvancesOnLostPremise == ~premiseAdvanced

\* INV10 -- the premise-lost supersession never fires on a generation whose
\* premise is INTACT.
\*
\* This is not a restatement of INV9 and it is not defence in depth: it
\* polices the OTHER direction, and it exists because the landed transition
\* row names a quantity the generation itself moves. Superseding a live
\* generation is not free -- SS7's generations exist because the perp short
\* is nearing liquidation, so a spurious abort cancels the very mechanism
\* that was reducing the risk, and does it while a reserved leg is out.
\*
\* NC22 is the control, and note what it is a control ON: not a defect
\* somebody might implement, but the LITERAL READING OF THE SPEC'S OWN
\* SENTENCE. If it is violated, the sentence names the wrong quantity.
INV10_PremiseSupersessionOnlyOnRealLoss == ~supersededOnIntactPremise

(***************************************************************************)
(* SLICE F -- the FIX'S OWN JUSTIFICATION, checked rather than recorded.    *)
(*                                                                         *)
(* `FM-9`'s resolution rests on one claim: that the premise, evaluated net  *)
(* of the generation's own legs, is a quantity THE GENERATION ITSELF CANNOT *)
(* MOVE. If that is false the fix is wrong, because the trigger would then  *)
(* fire on the generation's own behaviour for a second time and the whole   *)
(* finding would have been mis-diagnosed.                                   *)
(*                                                                         *)
(* It is stated as an argument in three places (SM SS7, the companion, the  *)
(* design record) and an argument is exactly what `FM-7` says not to leave  *)
(* lying about: NOBODY RE-DERIVES A RECORDED JUSTIFICATION. So it is an     *)
(* ACTION property here, and TLC re-derives it on every run.               *)
(*                                                                         *)
(* Non-vacuity costs no run: every action in `PdOwnStep` is already shown   *)
(* REACHED by an existing probe -- PC13 (Complete), PC14/PC25 (both parks), *)
(* PC15 (in flight), PC18 (partial perp confirmation), PC21 (a nonzero      *)
(* transient), PC20 (a second generation).                                 *)
(***************************************************************************)

\* Every step the GENERATION itself takes. PdEnds and Supersede are out on
\* purpose: both END the generation, and the quantity is only defined
\* against a live one -- their resets are the scope closing, not the
\* generation moving its own premise.
PdOwnStep ==
  \/ \E t \in 1..MAXQ : PdPlan(t)
  \/ PdSubmitPerp
  \/ \E x \in QDom \ {0} : PdPerpFill(x)
  \/ PdPerpFinal
  \/ \E s \in QDom : PdAuthorizeSpot(s)
  \/ PdSubmitSpot
  \/ PdSpotHolds
  \/ PdPerpHolds
  \/ \E x \in QDom : PdSpotFinal(x)

\* P6 -- the generation cannot move its own premise. Every ledger change it
\* makes is to ITS OWN entry (counted in PdReserved) and every realized
\* change is ITS OWN transient (counted in PdOwnRealized), so both
\* subtractions cancel. That is `FM-5`'s fix -- "the reservation is exactly
\* backed at every instant" -- showing up as a conservation law.
P6_GenerationCannotMoveItsOwnPremise ==
  [][ PdOwnStep => PremiseResidual' = PremiseResidual ]_vars

\* Not in any nominal configuration. It exists to interrogate NC12's own
\* witness: TLC reports the SHORTEST counterexample, and for the orphan
\* clause that is an entry of amount ZERO -- which leaks a ledger slot but
\* subtracts nothing, so the control would demonstrate the leak while saying
\* nothing about the harm. Checked against NC12's constants, this asks
\* whether the HARMFUL orphan is reachable too. It is a question about a
\* control's evidence, which is the same question the fixture-collapse
\* checklist asks about a fixture.
NoNonzeroOrphan ==
  pd = "None" =>
    \A k \in PendIds :
      ~(pending[k].active /\ pending[k].kind = "pkgspot" /\ pending[k].amt # 0)

\* SLICE C. The same question asked of NC13's and NC14's witnesses, and it
\* had to be asked for the same reason: TLC reports A counterexample, not the
\* most harmful one, and NC13's ends in a state where `budget = 0`. There the
\* machine could not have compensated the residual even with a correct
\* classifier -- so that trace demonstrates the STALL (which is all P1
\* claims) while saying nothing about the HARM, exactly as NC12's shortest
\* orphan was an entry of amount zero.
\*
\* This asks whether the machine can be stranded on a residual it was still
\* able to act on: every interval collapsed, no generation running, a NONZERO
\* residual, dispatch budget LEFT, and a verdict that is not `Determined` and
\* now never will be. Checked against NC13's and NC14's constants, where a
\* violation is the good outcome; it is not in any nominal configuration,
\* where it is TRUE and says nothing.
NoStrandedNakedResidual ==
  ~ /\ cls # "Determined"
    /\ AllCollapsed
    /\ pdPerp = 0
    /\ pd = "None"
    /\ TrueResidual # 0
    /\ budget > 0

\* SLICE C. The same question asked of NC16's witness, which lassos at
\* `budget = 0` for the same reason NC13's stutters there: TLC's
\* counterexample search finds the deep states. A stuck reservation costs
\* nothing if nothing will ever size against it again, so the harm claim is
\* that the machine can be ABOUT TO SIZE -- determined, budget left, a free
\* generation slot, a residual to act on -- while a floor generation caught
\* in the blind-retry loop contributes a nonzero term to the subtraction it
\* will size from. That reduction is short by exactly the amount of a
\* reduction that is never going to land.
\*
\* Deliberately the same shape as the PC9-PC11 `SubtrahendFrom` probes, one
\* conjunct longer: the conjunct is `amb`, and it is the whole difference
\* between a reservation that is working and one that is stuck.
NoStuckFloorSubtrahend ==
  ~ \E k \in PendIds :
      /\ pending[k].active /\ pending[k].kind = "floor" /\ pending[k].amb
      /\ cls = "Determined"
      /\ budget > 0
      /\ FreePend # {}
      /\ realized - TrueReserved # 0

(***************************************************************************)
(* SLICE C: LIVENESS (SS46 SS4), checked under LiveSpec only.              *)
(*                                                                         *)
(* Each ships with its negative control in this same slice, because Cx8    *)
(* proved a vacuous liveness property and a passing one are indistinguish- *)
(* able. The controls are NC13-NC16 and the mapping is one-to-one:         *)
(* P1 -> NC13, P2 -> NC14, P3 -> NC15, P4 -> NC16.                         *)
(*                                                                         *)
(* ISOLATION IS PART OF THE DESIGN, not a happy accident. CLAUDE.md's rule *)
(* for the lint gates -- a probe two gates can see shows that SOMETHING    *)
(* objected, not that THIS rule did -- applies to a control the same way.  *)
(* So the consequents below are kept as narrow as the claim allows:        *)
(* P4 says the floor generation leaves the ledger and stops there, because *)
(* conjoining "and the episode ends at Determined" would make NC13 fail P4 *)
(* as well and the control would no longer name its own defect. What the   *)
(* narrow P4 gives up is recovered by COMPOSITION: what the floor routes   *)
(* into is an order, and P1 is the claim about orders.                     *)
(*                                                                         *)
(* The one deliberate overlap is NC13, which fails P1 AND P2: it removes   *)
(* re-classification on the finality edge for every verdict. NC14 is the   *)
(* narrower control that isolates P2, which is why both exist.             *)
(***************************************************************************)

\* A reserved IntervalFloorReduction generation is outstanding.
FloorOutstanding ==
  \E k \in PendIds : pending[k].active /\ pending[k].kind = "floor"

\* P1 -- an `Ambiguous` residual never sits unclassified: it reaches
\* `Determined` (its intervals collapsed and the machine SAID SO) or
\* `DeadlineExceeded` (the contract's deadline blew and the machine said
\* THAT). SM SS2 Scope's "the machine MUST NOT wait for the next fill",
\* stated as the liveness half SS46 SS7e split off from INV7.
\*
\* Its evidence is NC13. Note what the property does NOT assume: the
\* deadline never has to blow (DeadlineBlows is unfair), so P1 is
\* discharged by the finality barrier alone.
P1_AmbiguousEventuallyClassified ==
  [] ( cls = "Ambiguous" => <> (cls \in { "Determined", "DeadlineExceeded" }) )

\* P2 -- SS2's LATE-RESOLUTION transition: a deadline-exceeded episode whose
\* intervals later collapse goes back to `Determined`. The episode is not a
\* terminal state, and the difference between "not terminal" and "terminal
\* pending an operator" is invisible to every invariant in this module --
\* which is the whole reason a liveness property is the right instrument.
P2_DeadlineExceededResolves ==
  [] ( cls = "DeadlineExceeded" => <> (cls = "Determined") )

\* P3 -- FLAG-4's nonterminal ruling, mechanized: a `Held/Alert` generation
\* eventually resumes to `Complete` or enters `TerminalFlatten`. It holds
\* and alerts; it does not hold forever.
\*
\* SS46 SS8g's MAX_GEN trap is answered by the SUBJECT, not by an
\* antecedent: this is a claim about THE HELD GENERATION's own resolution,
\* and nothing in it mentions a LATER generation. A spent `gens` count
\* disables PdPlan, which would stall a "another generation eventually
\* starts" property -- so that property is not stated, because the ladder
\* stopping at MAX_GEN is a bound of the model and not a behaviour of the
\* spec.
P3_HeldGenerationResumes ==
  [] ( pd = "Held" => <> (pd \in { "Complete", "TerminalFlatten" }) )

\* P4 -- the reserved floor generation does not stay outstanding: it fills,
\* or its own ambiguous result ROUTES back through the same SS6.7 finality
\* contract as an order (SM SS2 row 4, "it never blind-retries"). Either way
\* the ledger entry clears, and that is what makes the recursion a
\* recursion rather than a hang.
\*
\* Honesty note, and it is the sharpest one in this slice: "the recursion
\* TERMINATES" in the sense of "there are finitely many floors" is
\* guaranteed here by MAX_DISPATCH, i.e. by the model's own bound -- so a
\* control against THAT reading could not exist, and a property stating it
\* would be evidence about the bound. What is checked instead is the claim
\* the bound does NOT supply: each floor generation individually reaches
\* finality. NC16 is what shows that claim has content, and it has content
\* precisely because a blind retry spends no budget.
P4_FloorGenerationResolves ==
  [] ( FloorOutstanding => <> ~FloorOutstanding )

\* P5 -- SLICE E. FLAG-4's nonterminal ruling applied to the park SM SS7 has
\* had all along and slices A-D did not model: Held/Alert is entered from
\* `PerpReduceInFlight` OR `SpotReduceInFlight`, and the ruling is about the
\* STATE, not about which leg parked. So the perp-side park must resolve on
\* the same terms P3 demands of the spot-side one.
\*
\* Its consequent is P3's, deliberately: the claim is that THE GENERATION
\* resolves, not merely that it leaves the park. A consequent of
\* "PerpReduceFinal" would be discharged by the resume edge alone and would
\* say nothing about whether the resumed generation then completes -- and
\* SS7's ladder gates the next attempt on `Complete`, not on "unparked".
\*
\* Its evidence is NC15, which removes FLAG-4's resume edge from BOTH parks
\* (the control is about the ruling, not about a leg) and NC17, which exits
\* this park through the WRONG leg's barrier. Note that NC17 is a SAFETY
\* violation (INV8) rather than a liveness one -- a cause-blind exit does not
\* stall the generation, it resolves it WRONGLY, which is exactly why the
\* park needed an invariant as well as a property.
\* THE CONSEQUENT CARRIES A THIRD DISJUNCT AND P3's DOES NOT, and the
\* asymmetry is a fact about the spec rather than a convenience. SM SS7's
\* transition table ends an active generation THREE ways: resume-to-Complete,
\* TerminalFlatten, and "*(any active state)* -> *(superseded -- generation
\* ends)*" for EmergencyClosePackage / FenceReadyLost. P3 needs only two
\* because supersession is BARRED from `Held`: SS7's rule is "cancelling its
\* RESERVED SECOND LEG to SS6.7 finality first", `Held` is a state in which
\* that leg is live, and QUIESCE_BEFORE_SUPERSEDE therefore blocks the edge.
\*
\* At a PERP-side park the reserved second leg does not exist yet, so the
\* quiesce precondition has nothing to bind on and supersession IS
\* admissible -- TLC found exactly that behaviour (PdPerpHolds -> Supersede
\* -> stuttering) against the two-disjunct phrasing. Recorded rather than
\* absorbed: SS7's quiesce clause is scoped to the reserved second leg, so a
\* perp-side park is superseded WITHOUT it. That looks safe here -- Supersede
\* drops the perp leg's remaining interval with OrderFinal's semantics and
\* reclassifies, so no residual is stranded, and both superseding paths have
\* their own quiesce barrier over all resting orders (core SS7.4) -- but the
\* scoping is an observation this slice makes, not a claim it verifies.
\*
\* No vacuity in the added disjunct: Supersede requires `superseded = "none"`
\* and sets `pd' = "None"`, and PdPlan requires `superseded = "none"`, so a
\* generation can never REACH `HeldPerp` with the disjunct already true.
P5_HeldPerpGenerationResumes ==
  [] ( pd = "HeldPerp"
         => <> (pd \in { "Complete", "TerminalFlatten" } \/ superseded # "none") )

=============================================================================
