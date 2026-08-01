---------------------------- MODULE AmbiguousOrder ----------------------------
(***************************************************************************)
(* Formal model (a), SLICE A: the ambiguous-order classifier, the          *)
(* possible-delta interval algebra, the per-venue finality contract, and   *)
(* the reserved IntervalFloorReduction.                                    *)
(*                                                                         *)
(* COMPANION ARTIFACT. docs/state-machines.md (SM) stays normative; this   *)
(* module model-checks SM SS2 (ambiguous-order classifier & the finality   *)
(* contract) against the invariants its prose promises. Design record:     *)
(* docs/superpowers/specs/2026-08-01-formal-model-a-ambiguous-order-       *)
(* design.md (SS46). Companion to FenceTakeover.tla (model (b)); same      *)
(* artifact discipline, next finding tag is FM-5.                          *)
(*                                                                         *)
(* SLICE A scope (SS46 SS5): the interval algebra, the classifier          *)
(* transitions, INV1-INV3 + INV7, and one deliberate broken-constant run   *)
(* per invariant. The reservation ledger is present but SINGLE-PRODUCER    *)
(* (this model's own reductions). Slice B adds PackageDelever's reserved   *)
(* spot leg and the determined branch's fenced generation as the second    *)
(* and third producers, with INV4-INV6. Slice C adds liveness P1-P4.       *)
(*                                                                         *)
(* Deliberately OUT of model (SS46 "Deliberately not modeled"):            *)
(* venue-specific REST semantics, money/prices/fees (quantities are small  *)
(* bounded integers), the liq-distance trigger, the full SS1 OMS order     *)
(* lifecycle (an order is modeled by the interval it contributes and       *)
(* whether it has been observed executing), and real time (a deadline is a *)
(* nondeterministic environment event, exactly as in model (b)).           *)
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
  RESERVATION_AWARE_SIZING

(***************************************************************************)
(* Value domains. Deliberately generous: TypeOK is the CHECK that the      *)
(* bounds hold, so a domain that is too tight would report as a type       *)
(* violation (a real signal) rather than silently truncating behavior.     *)
(***************************************************************************)
QDom    == 0..8        \* an order's remaining quantity
AmtDom  == -16..16     \* signed reduction magnitudes / realized delta
Dirs    == { -1, 1 }   \* -1 = the order can still SELL (reduce delta)
                       \* +1 = the order can still BUY  (raise delta)
Kinds   == { "exposure", "floor" }
Classes == { "Determined", "Ambiguous", "DeadlineExceeded" }

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
  \* pending[k] = [active, amt, kind]. `amt` is signed in the same units
  \* as the residual it cancels: +1 means "1 of long residual is already
  \* committed to be reduced". SLICE A is single-producer: every entry is
  \* one of this model's own dispatches. Slice B promotes this to the
  \* three-producer ledger of SS46 SS2.1.
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
  \* --- Bound bookkeeping ------------------------------------------------
  budget

vars == << orders, realized, pending, cls, deadline, floorIssued, floorCount,
           overshoot, longReduced, shortReduced,
           dispatchedWhileAmbiguous, nonFloorInDeadline, floorWhileStraddling,
           budget >>

histVars == << overshoot, longReduced, shortReduced,
               dispatchedWhileAmbiguous, nonFloorInDeadline,
               floorWhileStraddling >>
epiVars  == << cls, deadline, floorIssued, floorCount >>

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

SumLo    == SumSeq([i \in OrderIds |-> LoOf(orders[i])])
SumHi    == SumSeq([i \in OrderIds |-> HiOf(orders[i])])

\* outstanding_reserved_reductions, summed over live generations.
Reserved == SumSeq([k \in PendIds |->
                      IF pending[k].active THEN pending[k].amt ELSE 0])

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
ImplResidual == IF RESERVATION_AWARE_SIZING THEN realized - Reserved ELSE realized
ImplLo       == ImplResidual + SumLo
ImplHi       == ImplResidual + SumHi
ImplStraddles == ImplLo < 0 /\ ImplHi > 0

TrueResidual == realized - Reserved
TrueLo       == TrueResidual + SumLo
TrueHi       == TrueResidual + SumHi
Straddles    == TrueLo < 0 /\ TrueHi > 0

(***************************************************************************)
(* The classifier (SM SS2 States + Scope).                                 *)
(*                                                                         *)
(* TRUTH: the residual is determined when NO exposure-changing order on    *)
(* the hedge group has any remaining executable OR UNOBSERVED quantity.    *)
(* An order whose interval has collapsed to [0,0] has neither, whether it  *)
(* collapsed by executing in full or by a cancel acknowledged with zero    *)
(* executions.                                                             *)
(***************************************************************************)
TrulyDetermined(o)    == \A i \in OrderIds : o[i].q = 0

\* What the classifier REPORTS. Under CLASSIFY_ON_FILLS_ONLY it reads the
\* fill ledger alone, so an order with remaining quantity that has never
\* been observed executing is invisible to it.
ReportedDetermined(o) ==
  IF CLASSIFY_ON_FILLS_ONLY
  THEN \A i \in OrderIds : (o[i].q = 0 \/ ~o[i].seen)
  ELSE TrulyDetermined(o)

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
Reclassify(o, d) ==
  LET det == ReportedDetermined(o)
  IN  /\ cls'         = IF det THEN "Determined"
                        ELSE IF d THEN "DeadlineExceeded" ELSE "Ambiguous"
      /\ deadline'    = IF det THEN FALSE ELSE d
      /\ floorIssued' = IF det THEN FALSE ELSE floorIssued
      /\ floorCount'  = IF det THEN 0     ELSE floorCount

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

FreePend  == { k \in PendIds : ~pending[k].active }
FreeOrder == { i \in OrderIds : orders[i].q = 0 }

\* An inactive slot is canonicalized (amt = 0, kind = "exposure") and a
\* collapsed order is canonicalized (dir = -1, seen = FALSE). Sound
\* because nothing reads those fields once the slot/order is inactive:
\* LoOf/HiOf are 0 at q = 0, Reserved skips inactive slots, and the broken
\* classifier's `q = 0 \/ ~seen` is already true at q = 0. It exists only
\* to stop two behaviorally identical states counting twice.
DeadPend  == [active |-> FALSE, amt |-> 0, kind |-> "exposure"]
DeadOrder == [dir |-> -1, q |-> 0, seen |-> FALSE]

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
  /\ cls = (IF ReportedDetermined(orders) THEN "Determined" ELSE "Ambiguous")
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
\* INVARIANT under a fill. (That is the first half of why INV1 checked at
\* dispatch is not weaker than INV1 checked everywhere; see INV1.)
OrderFill(i, x) ==
  /\ orders[i].q > 0
  /\ x \in 1..orders[i].q
  /\ LET q2 == orders[i].q - x
         o2 == IF q2 = 0 THEN DeadOrder
                         ELSE [dir |-> orders[i].dir, q |-> q2, seen |-> TRUE]
         no == [orders EXCEPT ![i] = o2]
     IN /\ orders' = no
        /\ realized' = realized + orders[i].dir * x
        /\ Reclassify(no, deadline)
  /\ UNCHANGED << pending, budget >>
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
        /\ Reclassify(no, deadline)
  /\ UNCHANGED << realized, pending, budget >>
  /\ UNCHANGED histVars

\* SM SS2 row 3: the contract cannot complete within its deadline
\* (queries unavailable / rate-limited) -> DeadlineExceededAmbiguous.
DeadlineBlows ==
  /\ cls = "Ambiguous"
  /\ deadline' = TRUE
  /\ cls' = "DeadlineExceeded"
  /\ UNCHANGED << orders, realized, pending, floorIssued, floorCount, budget >>
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
     IN /\ pending' = [pending EXCEPT ![k] =
                         [active |-> TRUE, amt |-> amt, kind |-> "exposure"]]
        /\ RecordDispatch(amt, "exposure")
  /\ budget' = budget - 1
  /\ UNCHANGED << orders, realized >>
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
        /\ pending' = [pending EXCEPT ![k] =
                         [active |-> TRUE, amt |-> amt, kind |-> "floor"]]
        /\ RecordDispatch(amt, "floor")
  /\ floorIssued' = TRUE
  /\ floorCount' = floorCount + 1
  /\ budget' = budget - 1
  /\ UNCHANGED << orders, realized, cls, deadline >>

(***************************************************************************)
(* How a reserved generation resolves.                                     *)
(***************************************************************************)

\* The generation reaches finality fully executed: the committed delta
\* becomes realized delta and the reservation is released. The TRUE net
\* range is unchanged across this step (realized and Reserved move
\* together) -- the second half of INV1's at-dispatch sufficiency.
ReductionFills(k) ==
  /\ pending[k].active
  /\ realized' = realized - pending[k].amt
  /\ pending' = [pending EXCEPT ![k] = DeadPend]
  /\ UNCHANGED << orders, budget >>
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
  /\ FreeOrder # {}
  /\ LET i == CHOOSE i \in FreeOrder : TRUE
         a == pending[k].amt
         no == [orders EXCEPT ![i] =
                  [dir |-> IF a > 0 THEN -1 ELSE 1,
                   q   |-> Abs(a), seen |-> FALSE]]
     IN /\ orders' = no
        /\ pending' = [pending EXCEPT ![k] = DeadPend]
        /\ Reclassify(no, deadline)
  /\ UNCHANGED << realized, budget >>
  /\ UNCHANGED histVars

(***************************************************************************)
(* Settled terminal. Deadlock checking stays ON deliberately: this is the  *)
(* ONLY terminal state the model predicts, so any other dead end is        *)
(* reported by TLC as a deadlock and is a finding, not noise.              *)
(***************************************************************************)
AllCollapsed == \A i \in OrderIds : orders[i].q = 0
NoPending    == \A k \in PendIds : ~pending[k].active

Settled ==
  /\ AllCollapsed /\ NoPending
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
  \/ Settled

Spec == Init /\ [][Next]_vars

(***************************************************************************)
(* Invariants.                                                             *)
(***************************************************************************)

TypeOK ==
  /\ orders \in [ OrderIds -> [dir: Dirs, q: QDom, seen: BOOLEAN] ]
  /\ realized \in AmtDom
  /\ pending \in [ PendIds -> [active: BOOLEAN, amt: AmtDom, kind: Kinds] ]
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
INV1_NeverOvershootSign ==
  /\ ~overshoot
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
  cls = "Determined" => \A i \in OrderIds : orders[i].q = 0

=============================================================================
