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
    /\ Reserved # 0
    /\ realized - Reserved # 0
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

===============================================================================
