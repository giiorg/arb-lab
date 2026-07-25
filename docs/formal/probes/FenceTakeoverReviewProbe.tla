----------------------- MODULE FenceTakeoverReviewProbe -----------------------
EXTENDS FenceTakeover

\* A detected loss with live exposure must not remain forever in the
\* pre-branch arbitration state after the environment stabilizes.
LostEventuallyLeaves ==
  EnvStabilizes =>
    [] ( (incidentActive /\ fenceCtl = "lost" /\ pos # "flat")
          => <> (fenceCtl # "lost" \/ pos = "flat" \/ ~incidentActive) )

\* Close-budget exhaustion is documented as an operator-escalated park, not
\* an internal held -> quiesce -> reduce -> held cycle.  Once the exhausted
\* state has no live orders and readiness is healthy, it must eventually
\* remain parked (or actually resolve).
ExhaustedHeldEventuallyParks ==
  EnvStabilizes =>
    [] ( (incidentActive /\ actPhase = "held" /\ attemptsLeft = 0
           /\ pos # "flat" /\ NoLiveOrders /\ actReadyOK)
          => <>[] (actPhase = "held" \/ pos = "flat" \/ ~incidentActive) )

===============================================================================
