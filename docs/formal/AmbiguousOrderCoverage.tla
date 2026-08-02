------------------------ MODULE AmbiguousOrderCoverage ------------------------
(***************************************************************************)
(* POSITIVE CONTROLS for AmbiguousOrder.tla.                               *)
(*                                                                         *)
(* A clean run and a vacuous run look identical. The nominal safety run     *)
(* being green says nothing until the interesting states are shown to be    *)
(* REACHED -- this repo has nine recorded instances of a fixture pinned to  *)
(* the value that collapses the distinction the code makes, and a formal    *)
(* model is not exempt: a bound, an Init range or a CHOOSE can pin a        *)
(* parameter just as effectively as a Go test fixture can.                  *)
(*                                                                         *)
(* EVERY property here is phrased as the NEGATIVE claim ("never reached"),  *)
(* so a TLC VIOLATION is the GOOD outcome and a PASS is a coverage gap.     *)
(* The polarity is in the names on purpose: `PC1_FloorNeverDispatched`      *)
(* passing means no interval floor was ever issued, i.e. INV2's subject     *)
(* does not occur and NC1/NC2 prove nothing about the nominal model.        *)
(*                                                                         *)
(* Run via ../run.sh, which expects each to be VIOLATED.                    *)
(***************************************************************************)
EXTENDS AmbiguousOrder

\* PC1 -- an IntervalFloorReduction is dispatched at all. Without this the
\* whole deadline-exceeded branch is dead code in the nominal model.
PC1_FloorNeverDispatched == floorCount = 0

\* PC2 -- two reserved generations are live at once. This is the state in
\* which `outstanding_reserved_reductions` is a SUM rather than a single
\* entry, and the only nominal shape in which slice B's three-producer
\* ledger has a precedent.
PC2_NeverTwoGenerations ==
  ~ \E k1, k2 \in PendIds :
      k1 # k2 /\ pending[k1].active /\ pending[k2].active

\* PC3 -- the determined branch is about to size itself with a NONZERO
\* subtrahend: `realized - reserved` where reserved # 0. If this is never
\* reached, the subtraction SS46 SS2.1 calls the composition surface is
\* pinned to `realized - 0` and NC6 is the only thing that ever exercises
\* it -- which would make the nominal run's INV1 evidence about a
\* different, simpler system.
PC3_SubtrahendAlwaysZero ==
  ~ /\ cls = "Determined"
    /\ TrueReserved # 0
    /\ realized - TrueReserved # 0
    /\ FreePend # {}
    /\ budget > 0

\* PC4 -- a summed interval that STRADDLES zero is reached in the
\* deadline-exceeded branch. This is the state STRADDLE_GUARD refuses. If
\* it is unreachable the guard blocks nothing and NC5 is testing a
\* transition that never fires nominally.
PC4_StraddleNeverReached == ~(cls = "DeadlineExceeded" /\ Straddles)

\* PC5 -- the floor's OWN ambiguous result re-entered the contract while
\* counter-legs were still open. All NORD slots carrying quantity is
\* reachable ONLY through ReductionAmbiguous: Init leaves the last slot
\* collapsed, and it is the recycling slot. This is the recursion SM SS2
\* row 4 describes and P4 will police in slice C.
PC5_FloorNeverRecursed == ~ \A i \in OrderIds : orders[i].q > 0

\* PC6/PC7 -- BOTH SIGN BRANCHES of the interval-floor rule are exercised.
\* The rule is asymmetric (`a` when guaranteed net-long, `b` when
\* guaranteed net-short) and the spec's own historical defect lived in
\* exactly that asymmetry (R1d-2). A model that only ever reaches one side
\* checks half the rule while looking complete -- the sign mirror of the
\* fixture-collapse tell.
PC6_NoShortSideFloor ==
  ~ \E k \in PendIds :
      pending[k].active /\ pending[k].kind = "floor" /\ pending[k].amt < 0
PC7_NoLongSideFloor ==
  ~ \E k \in PendIds :
      pending[k].active /\ pending[k].kind = "floor" /\ pending[k].amt > 0

\* PC8 -- an ACTION probe: an order-lifecycle event collapses the interval
\* of an order that has NEVER been observed executing, with the realized
\* delta unmoved. That is SM SS2 Scope's named case verbatim -- "a cancel
\* confirmed with ZERO executions emits no fill and no position event, yet
\* fully collapses that order's interval" -- and the reason the machine
\* must not wait for the next fill. It is the one coverage claim that
\* cannot be made about a state, only about a step: the successor is
\* indistinguishable from an Init state where the order was never there.
\*
\* `realized' = realized` is what discriminates it from a full fill, which
\* also collapses the interval but always moves the delta (x >= 1).
PC8_NoCancelWithZeroExecutions ==
  [][ \A i \in OrderIds :
        ~ /\ orders[i].q > 0
          /\ ~orders[i].seen
          /\ orders'[i] = DeadOrder
          /\ realized' = realized ]_vars

(***************************************************************************)
(* SLICE B probes.                                                          *)
(*                                                                          *)
(* PC9-PC11 are SLICE B'S DONE-WHEN (SS46 SS7f): "whatever the ledger        *)
(* becomes, there must be a reachable state in which EACH producer's         *)
(* contribution to the one subtraction is nonzero". PC3 shows the sum is     *)
(* nonzero somewhere; it cannot say WHICH producer put it there, and a       *)
(* three-producer ledger where only one producer is ever nonzero would pass  *)
(* PC3 while SS46 SS2.1's entire argument for modeling SS2 with SS7 went      *)
(* unexercised.                                                              *)
(***************************************************************************)

\* The determined branch is about to size itself while producer `kd` is
\* contributing a nonzero term to `realized - reserved`.
SubtrahendFrom(kd) ==
  /\ cls = "Determined"
  /\ \E k \in PendIds :
        pending[k].active /\ pending[k].kind = kd /\ pending[k].amt # 0
  /\ realized - TrueReserved # 0
  /\ FreePend # {}
  /\ budget > 0

PC9_NoExposureSubtrahend  == ~SubtrahendFrom("exposure")
PC10_NoFloorSubtrahend    == ~SubtrahendFrom("floor")
PC11_NoPkgSpotSubtrahend  == ~SubtrahendFrom("pkgspot")

\* PC12 -- all THREE producers live in the ledger simultaneously. This is
\* the sharpest composition state there is, and it is the reason NPEND went
\* from 2 to 3: with two slots the sum can never have three terms and
\* "the three producers rest on one subtraction" would be a claim the model
\* could not reach. Same lesson as NPEND=1 in slice A, one producer out.
PC12_NeverThreeProducers ==
  ~ \E k1, k2, k3 \in PendIds :
       /\ k1 # k2 /\ k2 # k3 /\ k1 # k3
       /\ pending[k1].active /\ pending[k1].kind = "exposure"
       /\ pending[k2].active /\ pending[k2].kind = "floor"
       /\ pending[k3].active /\ pending[k3].kind = "pkgspot"

\* PC13-PC15, PC19 -- the SS7 phases are reached at all. A phase no behavior
\* enters is dead code that INV5/INV6 would police vacuously.
PC13_PdNeverCompletes      == pd # "Complete"
PC14_PdNeverHeld           == pd # "Held"
PC15_PdNeverInFlight       == pd # "SpotReduceInFlight"
PC19_PdNeverTerminalFlatten == pd # "TerminalFlatten"

\* PC16/PC17 -- BOTH superseding paths fire, separately. SM SS7 names them as
\* two rows (EmergencyClosePackage, core SS5.3; FenceReadyLost, core SS7.4),
\* and one probe over a merged flag would let either be dead code while
\* reading green. The sign-mirror lesson of PC6/PC7, applied to paths.
\* Supersede requires an ACTIVE generation, so reaching either at all is
\* also what makes INV6 non-vacuous.
PC16_NeverSupersededByEmergency == superseded # "EmergencyClosePackage"
PC17_NeverSupersededByFence     == superseded # "FenceReadyLost"

\* PC18 -- THE SLICE B FIXTURE-COLLAPSE PROBE. A perp reduction that
\* confirmed to finality having executed SOME BUT NOT ALL of its target.
\* If the perp leg always confirms in full then pdDone = pdTarget always,
\* "sells no more spot than the CONFIRMED reduction" and "...than the
\* TARGET" are the same expression, and NC8 is undetectable -- the exact
\* shape of the nine recorded fixture collapses, at model altitude. This is
\* the slice-B counterpart of slice A's PC3/NPEND finding.
PC18_PerpAlwaysFullyConfirmed ==
  ~ /\ pd \in { "PerpReduceFinal", "SpotReduceAuthorized" }
    /\ pdDone > 0
    /\ pdDone < pdTarget

\* PC20 -- a SECOND generation is started. SM SS7's ladder admits one "only
\* after the prior generation reaches SS6.7 finality"; with MAX_GEN = 1 that
\* edge is unreachable and the gate blocks nothing observable.
PC20_NeverTwoGenerations == gens < 2

\* PC21 -- the intended net-LONG transient is actually nonzero: the perp leg
\* has bought and the spot leg has not yet sold it back. INV5 says this
\* transient is never net-short; if it is never nonzero either, INV5 is a
\* claim about a quantity that is always 0 and both of its conjuncts hold
\* trivially.
PC21_TransientAlwaysZero == ~(PdActive /\ pdDone - pdSold > 0)

(***************************************************************************)
(* SLICE C probes -- THE LIVENESS ANTECEDENTS.                              *)
(*                                                                          *)
(* A liveness property whose antecedent is unreachable is not merely weak,   *)
(* it is TRUE: `[](A => <>B)` with A never reached is the vacuous pass Cx8   *)
(* named, and a negative control does not close it -- a control shows the    *)
(* property CAN fail somewhere, not that its subject occurs in the NOMINAL   *)
(* model. So each of P1-P4 gets its antecedent named here, in the same       *)
(* slice, on the same terms as every other probe: a PASS is the gap.         *)
(*                                                                          *)
(* Two of the four were already covered and are NOT duplicated: P3's         *)
(* antecedent is PC14 (`pd = "Held"` is reached) and PC1 is adjacent to      *)
(* P4's. PC24 exists anyway because PC1 counts DISPATCHES (`floorCount`)     *)
(* while P4 quantifies over OUTSTANDING LEDGER ENTRIES, and the states       *)
(* those two name come apart exactly when a floor dispatches and resolves    *)
(* inside one step of the other -- the "does the fixture contain the thing   *)
(* under test" question, asked of a probe instead of a test.                 *)
(***************************************************************************)

\* PC22/PC23 -- P1's and P2's antecedents. Both look obvious (Init itself can
\* be Ambiguous), and that is precisely why they are cheap to state and
\* worth stating: the alternative is inferring them from a probe that was
\* written for something else.
PC22_ClsNeverAmbiguous        == cls # "Ambiguous"
PC23_ClsNeverDeadlineExceeded == cls # "DeadlineExceeded"

\* PC24 -- P4's antecedent: a reserved IntervalFloorReduction generation is
\* OUTSTANDING in the ledger, not merely counted as dispatched.
PC24_FloorNeverOutstanding == ~FloorOutstanding

(***************************************************************************)
(* SLICE E -- the second park (SS46 SS10a, SS11b).                          *)
(***************************************************************************)

\* PC25 -- P5's antecedent, and NC17's: the PERP-side park is reached at all.
\* SM SS7's transition row has entered Held/Alert from `PerpReduceInFlight`
\* since v0.9 and slices A-D modeled only the spot side, so before this probe
\* the state did not exist -- which is why no run could report it missing.
PC25_PdNeverHeldPerp == pd # "HeldPerp"

\* PC26 -- THE PROBE THAT DECIDES SS46 SS10a's ARGUMENT, and it is a fixture-
\* collapse question asked of a narrowing rather than of a test.
\*
\* SS10a justified modeling only the spot-side park on the ground that a
\* perp-side one is the HARMLESS instance: "at the perp leg neither the
\* unsold transient nor a nonzero reservation exists yet (pdDone = 0, and the
\* pkgspot entry carries the outstanding net intent, which is zero until the
\* perp confirms)". That is true of the park taken BEFORE any fill and of no
\* other. PdPerpFill fires while pd = "PerpReduceInFlight" and AMENDS the
\* entry upward, so SS2's own `PartiallyFilled` case -- "explicitly means more
\* fills possible" -- puts a park after it.
\*
\* This probe claims the harmful shape is unreachable: a perp-side park
\* holding a REAL net-long transient AND a nonzero reservation. If it is
\* VIOLATED, SS10a's stated reason for the narrowing is wrong -- not the
\* narrowing's conclusion necessarily, but the argument offered for it, which
\* is what a later session would rely on. A reason that does not hold is
\* worse than none, because nobody re-derives a recorded justification.
PC26_PerpParkAlwaysEmpty ==
  ~ /\ pd = "HeldPerp"
    /\ pdDone > 0
    /\ PdReserved > 0

(***************************************************************************)
(* SLICE E item 4 -- SS8f's question about WHICH NEUTRALITY opens a         *)
(* generation. PC27/PC28 below, and they must be read in that order.        *)
(*                                                                         *)
(* PdPlan guards on `TrueResidual = 0`, and TrueResidual is `realized -     *)
(* TrueReserved` -- RESERVATION-NETTED. So the guard admits a package that  *)
(* is neutral PENDING AN IN-FLIGHT REDUCTION rather than physically flat,   *)
(* while core SS5.3 says PackageDelever reduces leverage "on a STILL-       *)
(* NEUTRAL package" and SS7's invariants say "the package has NO NAKED      *)
(* RESIDUAL". SS8f recorded that as a READING and deferred the question of  *)
(* whether physical neutrality is the stronger and intended precondition.   *)
(* It is FM-5's mirror: FM-5 was a reservation COUNTED BEFORE ITS BACKING   *)
(* EXISTED; this is a reservation counted AS IF ALREADY REALIZED, when it   *)
(* may never realize.                                                      *)
(*                                                                         *)
(* Neither probe can settle whether the spec MEANS the stronger reading --  *)
(* that is a spec question. What they settle is whether the two readings    *)
(* are DISTINGUISHABLE on this model, and whether the state that            *)
(* distinguishes them can lose its premise. Asked in this order because a   *)
(* negative on PC27 closes item 4 by itself: if the guard never admits a    *)
(* physically-exposed package, then "reservation-netted" and "physical" are *)
(* the same predicate here and the model has been silently checking the     *)
(* strong reading all along. That is the fixture-collapse tell at model     *)
(* altitude -- a parameter pinned to the value that collapses the           *)
(* distinction the code makes -- and it would mean four slices of runs said *)
(* nothing about SS8f either way.                                          *)
(***************************************************************************)

\* PC27 -- is a generation EVER opened over a package that is
\* reservation-netted neutral but PHYSICALLY exposed? `pd = "Planned"` with
\* both legs still at zero is the instant after PdPlan fired, so `realized #
\* 0` there means the guard passed on `realized = TrueReserved # 0`: real
\* exposure, exactly offset by somebody else's in-flight reduction.
\*
\* If this is NOT violated, the weak guard has never once admitted anything
\* the strong guard would have refused, and SS8f's question is moot ON THIS
\* MODEL -- which is itself the answer, and a different one from "the weak
\* guard is fine".
PC27_GenNeverOpenedOverReservedExposure ==
  ~ /\ pd = "Planned"
    /\ pdDone = 0
    /\ pdSold = 0
    /\ realized # 0

\* PC28 -- the harm shape, and the reason PC27 alone would not settle it. A
\* generation opened on a netted-neutral package has borrowed its premise
\* from a reservation it does not own. Reservations are RELEASED as well as
\* realized -- on finality, and on supersession (RELEASE_ON_SUPERSEDE) --
\* and a release is not an execution. So this asks for the state where the
\* premise has evaporated underneath a live generation: real exposure
\* (`realized # 0`), nothing reserved against it any more (TrueReserved =
\* 0, which includes the generation's own entry still being zero because its
\* perp leg has bought nothing), and the generation still running.
\*
\* No existing invariant covers this. INV5 polices the generation's OWN legs
\* (`pdSold <= pdDone`), not the package premise it was opened under, so a
\* green nominal run says nothing about it -- which is why this is a probe
\* and not an invariant flip. If reached, SS8f's deferred question has a
\* concrete witness and the answer is owed by the SPEC, not by the model.
PC28_GenPremiseNeverEvaporates ==
  ~ /\ PdActive
    /\ realized # 0
    /\ TrueReserved = 0

===============================================================================
