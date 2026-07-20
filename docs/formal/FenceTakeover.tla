------------------------------ MODULE FenceTakeover ------------------------------
(***************************************************************************)
(* Formal model (b): the FenceReadyLost -> actuator-takeover ->            *)
(* EmergencyPlan composition of the safety kernel.                         *)
(*                                                                         *)
(* COMPANION ARTIFACT. docs/state-machines.md (SM) stays normative; this   *)
(* module model-checks SM SS3 (FenceReadyLost arbitration), SM SS4         *)
(* (actuator takeover protocol), SM SS6 (EmergencyPlan lifecycle) against  *)
(* the invariants the prose promises. Correspondence map + abstraction     *)
(* register: docs/formal/fence-takeover-model.md. Comments cite SM rows.   *)
(*                                                                         *)
(* Deliberately OUT of model (proposed model (a), or implementation):      *)
(* delta arithmetic / the SS2 classifier internals (exposure is            *)
(* qualitative Flat/Neutral/Naked), interval-floor math, RTO clocks,       *)
(* venue REST/WS mechanics, prefix-complete discovery internals (R1d-5 --  *)
(* reconcile here reads venue truth directly), multi-trade concurrency     *)
(* (one trade + lease-gated successor admission).                          *)
(***************************************************************************)
EXTENDS Naturals

CONSTANTS
  \* Nominal TRUE: the revoke epoch survives signer restart until operator
  \* clearance (SM SS4 epoch semantics). FALSE = negative control NC1 --
  \* "a transiently-crashed signer could restart mid-takeover and restore
  \* two writers" (SM SS4 Invariants).
  DURABLE_LATCH,
  \* Nominal TRUE: takeover completion and the plan tombstone require
  \* proved-flat evidence -- SS2 finality of every close + zero reconciled
  \* per-leg allocations + final surfaces (Cx5-1, R1d-F7 "one gate, no
  \* gap"). FALSE = negative control NC2: issuance-gated completion.
  EVIDENCE_GATED,
  \* Nominal FALSE: the $1K+ fence-unavailable floor -- the actuator NEVER
  \* trades un-latched (SM SS3 tiered floor). TRUE = negative control NC3:
  \* the pre-$1K canary regime's unfenced de-minimis action, which
  \* "explicitly does not claim single-writer safety" (core SS7.4).
  CANARY_UNFENCED,
  \* Nominal TRUE: the FM-1 resolution -- a trade whose flatten reached
  \* completion evidence (its own close finality, or the takeover's durable
  \* ActuatedFlat record) is TERMINAL: it re-admits no exposure; fresh
  \* exposure is a successor trade via the SS6 lease, which only the
  \* tombstone releases. FALSE = negative control NC4: reproduces the FM-1
  \* counterexample this model discovered (operator clearance in the
  \* ActuatedFlat->Deactivate crash window + same-plan re-entry -> the
  \* stale re-driven tombstone kills coverage under live exposure).
  TRADE_TERMINALITY,
  \* The replicated close-attempt budget (max_close_attempts; Cx5-1).
  \* T_close is subsumed: budget exhaustion is the modeled exhaustion.
  MAX_CLOSE_ATTEMPTS,
  \* TRUE bounds crash events by CRASH_BUDGET so liveness is checkable
  \* ("behaviors with finitely many crashes terminate"); FALSE leaves
  \* crashes unbounded for the safety configs (counter frozen).
  BOUND_CRASHES,
  CRASH_BUDGET

VARIABLES
  \* --- Venue truth (abstract: one package, one venue-abstract account) ---
  pos,           \* "flat" | "neutral" | "naked" -- qualitative exposure
  entryLive,     \* a bot entry order the venue holds (can fill/terminal)
  protLive,      \* armed resting protection (separate cancel surface, SM SS4)
  botCloseLive,  \* a bot close/reduction order (state-dependent verb, R1d-1)
  actCloseLive,  \* an actuator close order
  \* --- Bot / engine ---
  botUp,
  botPhase,      \* "normal" | "winddown" | "reconcile" (fenced, awaiting clearance)
  \* --- Signer (its own trust boundary, core SS7.3) ---
  signerUp,
  latched,       \* the durable, monotonic revoke epoch (SM SS4)
  \* --- Environment / health-dimension truth (SM SS3 dimensions, R1d-4 cut) ---
  latchable,     \* primary-revoke readiness: a durable latch is achievable
  backupOK,      \* backup (exchange-side) fence readiness
  actReadyOK,    \* ActuatorReady: own credential/venue-reach/plan-store integrity
  \* --- FenceReadyLost arbitration (SM SS3) ---
  fenceCtl,      \* "ready"|"degraded"|"lost"|"winddown"|"manual"|"takeover"
  branchMax,     \* highest branch rank entered this incident (escalate-only, R1d-F6)
  incidentActive,
  \* --- Actuator takeover (SM SS4) ---
  actPhase,      \* "standby"|"latch"|"quiesce"|"reconcile"|"reduce"|
                 \* "closeInFlight"|"held"|"unavail"|"finalize"|"done"
  attemptsLeft,  \* remaining close-attempt budget (durable/replicated, Cx5-1)
  actuatedFlat,  \* durable ActuatedFlat evidence record (R1d-F7)
  \* --- EmergencyPlan store (SM SS6; store-visible state, ListActive view) ---
  plan,          \* "none" | "active" | "tombstone"
  leaseFree,     \* hedge-group lease released (admits successor trade)
  hadExposure,   \* this trade has held exposure (distinguishes a flattened
                 \* trade from one whose entry simply never filled)
  \* --- History flags for action-level invariants ---
  botLandedPostLatch,  \* a bot venue-write landed while the actuator was in a
                       \* post-latch active phase (two-writer evidence)
  actLandedUnfenced,   \* an actuator venue-write landed without the latch
  \* --- Liveness bookkeeping ---
  crashBudget

vars == << pos, entryLive, protLive, botCloseLive, actCloseLive,
           botUp, botPhase, signerUp, latched,
           latchable, backupOK, actReadyOK,
           fenceCtl, branchMax, incidentActive,
           actPhase, attemptsLeft, actuatedFlat,
           plan, leaseFree, hadExposure,
           botLandedPostLatch, actLandedUnfenced, crashBudget >>

venueVars  == << pos, entryLive, protLive, botCloseLive, actCloseLive >>
botVars    == << botUp, botPhase >>
signerVars == << signerUp, latched >>
envVars    == << latchable, backupOK, actReadyOK >>
ctlVars    == << fenceCtl, branchMax, incidentActive >>
actVars    == << actPhase, attemptsLeft, actuatedFlat >>
planVars   == << plan, leaseFree, hadExposure >>
flagVars   == << botLandedPostLatch, actLandedUnfenced >>

(***************************************************************************)
(* Derived predicates                                                     *)
(***************************************************************************)

\* The bot can land a venue write: signer up and un-latched. The latch is
\* what fences -- NOT the bot's own belief (a partitioned/confused bot still
\* tries; the attempt must fail at the signer/venue). Bot<->signer link loss
\* is folded into signerUp (SM SS3 signing dimension).
SigningWorks == signerUp /\ ~latched

NoLiveOrders == ~entryLive /\ ~protLive /\ ~botCloseLive /\ ~actCloseLive

\* Post-latch phases: the actuator acts on the premise the bot is fenced.
ActuatorFencedActive ==
  actPhase \in { "quiesce", "reconcile", "reduce", "closeInFlight", "held", "finalize" }

\* FM-1 resolution: a trade is TERMINAL once its flatten reached completion
\* evidence -- the takeover's durable ActuatedFlat record, or (having held
\* exposure) proved-flat with final surfaces (the bot-side close-finality /
\* stop-out evidence). A terminal trade re-admits no exposure; its plan may
\* only proceed to the tombstone; fresh exposure is a successor trade
\* through the lease.
TradeTerminal ==
  actuatedFlat \/ (hadExposure /\ pos = "flat" /\ NoLiveOrders)

\* SM SS3 observed dimensions (observed = truth in-model; a monitoring-lag
\* refinement is future work, noted in the correspondence map).
SigningOK == SigningWorks
RevokeOK  == latchable

\* Branch ranks: every permitted re-arbitration is strictly increasing --
\* winddown(1) -> manual(2) -> takeover(3); 1->2 encoded by SM SS3 as 1->3
\* (manual), 1->takeover and 3->takeover as escalations (R1d-F6, Cx5-2).
BranchRank == CASE fenceCtl = "winddown" -> 1
                [] fenceCtl = "manual"   -> 2
                [] fenceCtl = "takeover" -> 3
                [] OTHER                 -> 0

CrashAllowed == ~BOUND_CRASHES \/ crashBudget > 0
CrashSpend   == IF BOUND_CRASHES THEN crashBudget - 1 ELSE crashBudget

(***************************************************************************)
(* Init                                                                   *)
(***************************************************************************)
Init ==
  /\ pos = "flat"
  /\ entryLive = FALSE /\ protLive = FALSE
  /\ botCloseLive = FALSE /\ actCloseLive = FALSE
  /\ botUp = TRUE /\ botPhase = "normal"
  /\ signerUp = TRUE /\ latched = FALSE
  /\ latchable = TRUE /\ backupOK = TRUE /\ actReadyOK = TRUE
  /\ fenceCtl = "ready" /\ branchMax = 0 /\ incidentActive = FALSE
  /\ actPhase = "standby" /\ attemptsLeft = MAX_CLOSE_ATTEMPTS
  /\ actuatedFlat = FALSE
  /\ plan = "none" /\ leaseFree = TRUE /\ hadExposure = FALSE
  /\ botLandedPostLatch = FALSE /\ actLandedUnfenced = FALSE
  /\ crashBudget = CRASH_BUDGET

(***************************************************************************)
(* Bot actions (engine). Every venue write goes through SigningWorks and  *)
(* records two-writer evidence if it lands during a fenced-active takeover.*)
(***************************************************************************)

\* SM SS6 (none) -> Active: "replicate the EmergencyPlan off-host BEFORE
\* exposure opens"; ack required before the entry submits. Acquires the
\* hedge-group lease; a new plan resets the (per-plan) close budget and
\* clears any stale ActuatedFlat evidence (per-trade records).
BotReplicatePlan ==
  /\ botUp /\ botPhase = "normal" /\ plan \in { "none", "tombstone" } /\ leaseFree
  /\ plan' = "active" /\ leaseFree' = FALSE /\ hadExposure' = FALSE
  /\ attemptsLeft' = MAX_CLOSE_ATTEMPTS /\ actuatedFlat' = FALSE
  /\ UNCHANGED << actPhase >>
  /\ UNCHANGED << venueVars, botVars, signerVars, envVars, ctlVars, flagVars, crashBudget >>

\* Core SS6 step 5/6 + SM SS6 ack-before-entry + SM SS3 first-safe-action:
\* openings blocked unless ready/degraded (R1d-F5: degraded keeps openings).
\* Protection arms with the entry (attach abstraction; SM SS5 arm side).
BotEntrySubmit ==
  /\ botUp /\ botPhase = "normal" /\ fenceCtl \in { "ready", "degraded" }
  /\ plan = "active" /\ pos = "flat"
  /\ ~entryLive /\ ~botCloseLive /\ SigningWorks
  \* FM-1: no exposure re-admission on a terminal trade (NC4 disables).
  /\ ~(TRADE_TERMINALITY /\ TradeTerminal)
  /\ entryLive' = TRUE /\ protLive' = TRUE
  /\ botLandedPostLatch' = (botLandedPostLatch \/ ActuatorFencedActive)
  /\ UNCHANGED << pos, botCloseLive, actCloseLive, actLandedUnfenced >>
  /\ UNCHANGED << botVars, signerVars, envVars, ctlVars, actVars, planVars, crashBudget >>

\* Close/reduction submit -- the state-dependent verb of R1d-1 abstracted to
\* one order kind whose fill flattens (two-leg close for neutral, reduction
\* for naked). Enabled in normal ops and branch-1 wind-down (SM SS3 branch 1:
\* "bot-driven wind-down through the defined verbs"); closes are NOT gated
\* by stop-opening (first-safe-action blocks openings only).
BotCloseSubmit ==
  /\ botUp /\ botPhase \in { "normal", "winddown" }
  /\ pos # "flat" /\ ~botCloseLive /\ SigningWorks
  /\ botCloseLive' = TRUE
  /\ botLandedPostLatch' = (botLandedPostLatch \/ ActuatorFencedActive)
  /\ UNCHANGED << pos, entryLive, protLive, actCloseLive, actLandedUnfenced >>
  /\ UNCHANGED << botVars, signerVars, envVars, ctlVars, actVars, planVars, crashBudget >>

\* Protective-order cancel (SM SS5 disarm, abstracted: the DisarmPending
\* window is the cancel-vs-fire race below). A cancel is a venue write.
BotCancelProt ==
  /\ botUp /\ botPhase \in { "normal", "winddown" } /\ protLive /\ SigningWorks
  /\ protLive' = FALSE
  /\ botLandedPostLatch' = (botLandedPostLatch \/ ActuatorFencedActive)
  /\ UNCHANGED << pos, entryLive, botCloseLive, actCloseLive, actLandedUnfenced >>
  /\ UNCHANGED << botVars, signerVars, envVars, ctlVars, actVars, planVars, crashBudget >>

\* SM SS6 Active -> DeactivationPending -> Deactivated, collapsed to the
\* atomic idempotent tombstone (the pending record + bounded-backoff re-drive
\* is modeled by this action staying enabled across a crash -- evidence is
\* re-derivable in-model). EVIDENCE_GATED nominal: close reached SS2
\* finality with final surfaces (pos flat + no live orders). NC2: the
\* issuance-gated bug -- tombstone as soon as the close is merely issued.
BotDeactivate ==
  /\ botUp /\ botPhase \in { "normal", "winddown" } /\ plan = "active"
  /\ IF EVIDENCE_GATED
       THEN pos = "flat" /\ NoLiveOrders
       ELSE (pos = "flat" \/ botCloseLive)
  /\ plan' = "tombstone"
  /\ UNCHANGED << leaseFree, hadExposure >>
  /\ UNCHANGED << venueVars, botVars, signerVars, envVars, ctlVars, actVars, flagVars, crashBudget >>

\* Core SS6 step 9: only the tombstone's persisted acknowledgement releases
\* the hedge-group lease / admits a successor.
BotLeaseRelease ==
  /\ botUp /\ botPhase = "normal" /\ plan = "tombstone" /\ ~leaseFree
  /\ leaseFree' = TRUE
  /\ UNCHANGED << plan, hadExposure >>
  /\ UNCHANGED << venueVars, botVars, signerVars, envVars, ctlVars, actVars, flagVars, crashBudget >>

BotCrash ==
  /\ botUp /\ CrashAllowed
  /\ botUp' = FALSE /\ crashBudget' = CrashSpend
  /\ UNCHANGED << botPhase >>
  /\ UNCHANGED << venueVars, signerVars, envVars, ctlVars, actVars, planVars, flagVars >>

\* Reboot = reconcile-only boot (core SS7.5). "A bot that recovers
\* mid-takeover finds itself fenced and stays in reconcile-only until an
\* operator clears it" (SM SS4). Un-latched reboot resumes normal operation
\* (journaled IDs auto-recovered -- abstracted). fenceCtl is durable state:
\* a started branch survives reboot (one-way per incident, R1d-F6).
BotReboot ==
  /\ ~botUp
  /\ botUp' = TRUE
  \* A started branch survives reboot (one-way per incident, R1d-F6): the
  \* rebooted un-latched bot re-derives wind-down duty from durable state.
  /\ botPhase' = IF latched THEN "reconcile"
                 ELSE IF fenceCtl = "winddown" THEN "winddown" ELSE "normal"
  /\ UNCHANGED << venueVars, signerVars, envVars, ctlVars, actVars, planVars, flagVars, crashBudget >>

(***************************************************************************)
(* Signer actions. The epoch contract (SM SS4): the latch is durable and   *)
(* honored across restart until explicit operator clearance.               *)
(***************************************************************************)
SignerCrash ==
  /\ signerUp /\ CrashAllowed
  /\ signerUp' = FALSE /\ crashBudget' = CrashSpend
  /\ UNCHANGED << latched >>
  /\ UNCHANGED << venueVars, botVars, envVars, ctlVars, actVars, planVars, flagVars >>

\* NC1 lives here: with DURABLE_LATCH = FALSE a restart forgets the epoch.
SignerRestart ==
  /\ ~signerUp
  /\ signerUp' = TRUE
  /\ latched' = IF DURABLE_LATCH THEN latched ELSE FALSE
  /\ UNCHANGED << venueVars, botVars, envVars, ctlVars, actVars, planVars, flagVars, crashBudget >>

(***************************************************************************)
(* Environment: health dimensions degrade and repair nondeterministically. *)
(***************************************************************************)
LatchableFlip ==
  /\ latchable' = ~latchable
  /\ UNCHANGED << backupOK, actReadyOK >>
  /\ UNCHANGED << venueVars, botVars, signerVars, ctlVars, actVars, planVars, flagVars, crashBudget >>

BackupFlip ==
  /\ backupOK' = ~backupOK
  /\ UNCHANGED << latchable, actReadyOK >>
  /\ UNCHANGED << venueVars, botVars, signerVars, ctlVars, actVars, planVars, flagVars, crashBudget >>

ActReadyFlip ==
  /\ actReadyOK' = ~actReadyOK
  /\ UNCHANGED << latchable, backupOK >>
  /\ UNCHANGED << venueVars, botVars, signerVars, ctlVars, actVars, planVars, flagVars, crashBudget >>

(***************************************************************************)
(* FenceReadyLost arbitration (SM SS3). Bot-side logic (needs botUp).      *)
(* One-way per incident; escalate-only; clearance is the only exit.        *)
(***************************************************************************)

\* SM SS3: FenceReady -> FenceDegraded on backup-only loss (R1d-F5);
\* openings continue on the intact primary.
CtlDegrade ==
  /\ botUp /\ fenceCtl = "ready"
  /\ ~backupOK /\ SigningOK /\ RevokeOK /\ actReadyOK
  /\ fenceCtl' = "degraded"
  /\ UNCHANGED << branchMax, incidentActive >>
  /\ UNCHANGED << venueVars, botVars, signerVars, envVars, actVars, planVars, flagVars, crashBudget >>

\* SM SS3: FenceDegraded -> FenceReady on repair -- the sole health-driven
\* return (no branch was started).
CtlDegradedRepair ==
  /\ botUp /\ fenceCtl = "degraded" /\ backupOK
  /\ fenceCtl' = "ready"
  /\ UNCHANGED << branchMax, incidentActive >>
  /\ UNCHANGED << venueVars, botVars, signerVars, envVars, actVars, planVars, flagVars, crashBudget >>

\* SM SS3: loss of any non-backup-only dimension -> FenceReadyLost.
\* First-safe-action (block openings) is the fenceCtl guard on entry submit.
CtlLoss ==
  /\ botUp /\ fenceCtl \in { "ready", "degraded" }
  /\ (~SigningOK \/ ~RevokeOK \/ ~actReadyOK)
  /\ fenceCtl' = "lost" /\ incidentActive' = TRUE
  /\ UNCHANGED << branchMax >>
  /\ UNCHANGED << venueVars, botVars, signerVars, envVars, actVars, planVars, flagVars, crashBudget >>

\* Branch 1: revoke-control (or ActuatorReady, R1d-4) lost, signing works ->
\* bot-driven wind-down (overrides a configured hard-halt, SM SS3).
CtlBranch1 ==
  /\ botUp /\ fenceCtl = "lost" /\ SigningOK /\ (~RevokeOK \/ ~actReadyOK)
  /\ fenceCtl' = "winddown" /\ branchMax' = 1
  /\ botPhase' = "winddown"
  /\ UNCHANGED << incidentActive, botUp >>
  /\ UNCHANGED << venueVars, signerVars, envVars, actVars, planVars, flagVars, crashBudget >>

\* Branch 2: signing unavailable, a durable revoke latchable, AND
\* ActuatorReady confirmed (R1d-4) -> hand-off IS the takeover's fence-first
\* step (SM SS3/SS4): the actuator enters FenceLatch.
CtlBranch2 ==
  /\ botUp /\ fenceCtl = "lost" /\ ~SigningOK /\ RevokeOK /\ actReadyOK
  /\ fenceCtl' = "takeover" /\ branchMax' = 3 /\ incidentActive' = TRUE
  \* The takeover machine is plan-scoped (SM SS4 standby/ListActive): the
  \* hand-off engages it only when a plan exists to take over; with no
  \* active plan the incident resolves administratively via clearance.
  /\ actPhase' = IF actPhase = "standby" /\ plan = "active" THEN "latch" ELSE actPhase
  /\ UNCHANGED << attemptsLeft, actuatedFlat >>
  /\ UNCHANGED << venueVars, botVars, signerVars, envVars, planVars, flagVars, crashBudget >>

\* Branch 3: fence state uncertain -- no durable latch achievable, or
\* signing down with ActuatorReady unconfirmable -> bounded protection +
\* operator manual kill only; never an un-latched takeover (SM SS3).
CtlBranch3 ==
  /\ botUp /\ fenceCtl = "lost" /\ ~SigningOK /\ (~RevokeOK \/ ~actReadyOK)
  /\ fenceCtl' = "manual" /\ branchMax' = 2
  /\ UNCHANGED << incidentActive >>
  /\ UNCHANGED << venueVars, botVars, signerVars, envVars, actVars, planVars, flagVars, crashBudget >>

\* Escalate-only re-arbitration (R1d-F6; the 1->3 edge is Cx5-2's restored
\* escalation): signing fails mid-wind-down.
CtlEscalate1to2 ==
  /\ botUp /\ fenceCtl = "winddown" /\ ~SigningOK /\ RevokeOK /\ actReadyOK
  /\ fenceCtl' = "takeover" /\ branchMax' = 3
  \* The takeover machine is plan-scoped (SM SS4 standby/ListActive): the
  \* hand-off engages it only when a plan exists to take over; with no
  \* active plan the incident resolves administratively via clearance.
  /\ actPhase' = IF actPhase = "standby" /\ plan = "active" THEN "latch" ELSE actPhase
  /\ UNCHANGED << incidentActive, attemptsLeft, actuatedFlat >>
  /\ UNCHANGED << venueVars, botVars, signerVars, envVars, planVars, flagVars, crashBudget >>

CtlEscalate1to3 ==
  /\ botUp /\ fenceCtl = "winddown" /\ ~SigningOK /\ (~RevokeOK \/ ~actReadyOK)
  /\ fenceCtl' = "manual" /\ branchMax' = 2
  /\ UNCHANGED << incidentActive >>
  /\ UNCHANGED << venueVars, botVars, signerVars, envVars, actVars, planVars, flagVars, crashBudget >>

\* 3 -> 2: a durable latch becomes achievable with ActuatorReady confirmed --
\* "manual-only -> fenced takeover is strictly safer" (R1d-F6).
CtlEscalate3to2 ==
  /\ botUp /\ fenceCtl = "manual" /\ RevokeOK /\ actReadyOK
  /\ fenceCtl' = "takeover" /\ branchMax' = 3
  \* The takeover machine is plan-scoped (SM SS4 standby/ListActive): the
  \* hand-off engages it only when a plan exists to take over; with no
  \* active plan the incident resolves administratively via clearance.
  /\ actPhase' = IF actPhase = "standby" /\ plan = "active" THEN "latch" ELSE actPhase
  /\ UNCHANGED << incidentActive, attemptsLeft, actuatedFlat >>
  /\ UNCHANGED << venueVars, botVars, signerVars, envVars, planVars, flagVars, crashBudget >>

(***************************************************************************)
(* Watchdog + actuator takeover (SM SS4).                                  *)
(***************************************************************************)

\* "The watchdog escalates (i)->(iii) on host silence" (core SS7.4). Host
\* silence includes a PARTITION false positive: the fire is deliberately
\* unguarded on botUp -- a live-but-unobservable bot is exactly the
\* two-writer hazard the fence-first protocol must absorb.
WatchdogFire ==
  /\ actPhase = "standby" /\ plan = "active"
  /\ actPhase' = "latch" /\ incidentActive' = TRUE
  /\ UNCHANGED << fenceCtl, branchMax, attemptsLeft, actuatedFlat >>
  /\ UNCHANGED << venueVars, botVars, signerVars, envVars, planVars, flagVars, crashBudget >>

\* SM SS4 FenceLatch: latch the durable, monotonic revoke epoch; VerifyFence
\* validates THE EPOCH (never a point-in-time snapshot).
ActLatch ==
  /\ actPhase = "latch" /\ latchable
  /\ latched' = TRUE /\ actPhase' = "quiesce"
  /\ UNCHANGED << signerUp, attemptsLeft, actuatedFlat >>
  /\ UNCHANGED << venueVars, botVars, envVars, ctlVars, planVars, flagVars, crashBudget >>

\* SM SS4: "no durable fence latch can be established" -> FenceUnavailable;
\* the SS3 tiered floor governs. Deliberately NOT guarded on ~latchable:
\* the latch attempt is a bounded action (the T_fence window) whose timeout
\* can expire even while the mechanism is transiently available -- and an
\* anti-phase flapping environment must not be able to pin the protocol in
\* the latch state forever (timeout-to-unavail is always available).
ActLatchFail ==
  /\ actPhase = "latch"
  /\ actPhase' = "unavail"
  /\ UNCHANGED << attemptsLeft, actuatedFlat >>
  /\ UNCHANGED << venueVars, botVars, signerVars, envVars, ctlVars, planVars, flagVars, crashBudget >>

\* SM SS4: a durable latch established within T_fence -> fence-first resumes.
ActUnavailRetry ==
  /\ actPhase = "unavail" /\ latchable
  /\ actPhase' = "latch"
  /\ UNCHANGED << attemptsLeft, actuatedFlat >>
  /\ UNCHANGED << venueVars, botVars, signerVars, envVars, ctlVars, planVars, flagVars, crashBudget >>

\* NC3 ONLY (CANARY_UNFENCED): the pre-$1K canary's unfenced de-minimis
\* action after T_fence. At $1K+ this action does not exist: the floor is
\* read-only + operator manual kill (SM SS3).
ActCanaryUnfencedClose ==
  /\ CANARY_UNFENCED /\ actPhase = "unavail"
  /\ pos # "flat" /\ ~actCloseLive /\ attemptsLeft > 0
  /\ actCloseLive' = TRUE /\ attemptsLeft' = attemptsLeft - 1
  /\ actPhase' = "closeInFlight"
  /\ actLandedUnfenced' = (actLandedUnfenced \/ ~latched)
  /\ UNCHANGED << pos, entryLive, protLive, botCloseLive, botLandedPostLatch, actuatedFlat >>
  /\ UNCHANGED << botVars, signerVars, envVars, ctlVars, planVars, crashBudget >>

\* SM SS4 QuiesceSurfaces: cancel every bot-owned order across all cancel
\* surfaces -- entries, reductions, and ALL resting protection -- plus any
\* of the actuator's own prior orders after a crash (re-attempts stay
\* bounded; the fence + finality contract, not command IDs, prevent
\* double-execution). Cancels race venue fills (the venue actions below).
ActQuiesceCancelEntry ==
  /\ actPhase = "quiesce" /\ entryLive
  /\ entryLive' = FALSE
  /\ UNCHANGED << pos, protLive, botCloseLive, actCloseLive >>
  /\ UNCHANGED << botVars, signerVars, envVars, ctlVars, actVars, planVars, flagVars, crashBudget >>

ActQuiesceCancelProt ==
  /\ actPhase = "quiesce" /\ protLive
  /\ protLive' = FALSE
  /\ UNCHANGED << pos, entryLive, botCloseLive, actCloseLive >>
  /\ UNCHANGED << botVars, signerVars, envVars, ctlVars, actVars, planVars, flagVars, crashBudget >>

ActQuiesceCancelBotClose ==
  /\ actPhase = "quiesce" /\ botCloseLive
  /\ botCloseLive' = FALSE
  /\ UNCHANGED << pos, entryLive, protLive, actCloseLive >>
  /\ UNCHANGED << botVars, signerVars, envVars, ctlVars, actVars, planVars, flagVars, crashBudget >>

ActQuiesceCancelOwnStale ==
  /\ actPhase = "quiesce" /\ actCloseLive
  /\ actCloseLive' = FALSE
  /\ UNCHANGED << pos, entryLive, protLive, botCloseLive >>
  /\ UNCHANGED << botVars, signerVars, envVars, ctlVars, actVars, planVars, flagVars, crashBudget >>

\* The finality barrier holds only when EVERY order surface is final.
ActQuiesceDone ==
  /\ actPhase = "quiesce" /\ NoLiveOrders
  /\ actPhase' = "reconcile"
  /\ UNCHANGED << attemptsLeft, actuatedFlat >>
  /\ UNCHANGED << venueVars, botVars, signerVars, envVars, ctlVars, planVars, flagVars, crashBudget >>

\* SM SS4 Reconcile at the watermark. Sound here because post-quiesce no
\* order is live and the bot is fenced (nominal), so pos is stable; the
\* takeover then acts on reconciled LIVE state (never pre-allocated qty).
ActReconcileDone ==
  /\ actPhase = "reconcile"
  /\ actPhase' = "reduce"
  /\ UNCHANGED << attemptsLeft, actuatedFlat >>
  /\ UNCHANGED << venueVars, botVars, signerVars, envVars, ctlVars, planVars, flagVars, crashBudget >>

\* SM SS4 ReduceResidual -> CloseInFlight: the state-dependent terminal
\* action (R1d-1) -- fixed residual -> reduction; neutral package -> two-leg
\* close (post-quiesce it would sit unprotected/unmanaged, host dead).
\* Each attempt consumes the replicated budget (Cx5-1).
ActReduceIssue ==
  /\ actPhase = "reduce" /\ pos # "flat" /\ attemptsLeft > 0
  /\ actCloseLive' = TRUE /\ attemptsLeft' = attemptsLeft - 1
  /\ actPhase' = "closeInFlight"
  /\ actLandedUnfenced' = (actLandedUnfenced \/ ~latched)
  /\ UNCHANGED << pos, entryLive, protLive, botCloseLive, botLandedPostLatch, actuatedFlat >>
  /\ UNCHANGED << botVars, signerVars, envVars, ctlVars, planVars, crashBudget >>

\* Budget already exhausted at reduce time -> HeldIncident unconditionally.
ActReduceExhausted ==
  /\ actPhase = "reduce" /\ pos # "flat" /\ attemptsLeft = 0
  /\ actPhase' = "held"
  /\ UNCHANGED << attemptsLeft, actuatedFlat >>
  /\ UNCHANGED << venueVars, botVars, signerVars, envVars, ctlVars, planVars, flagVars, crashBudget >>

\* SM SS4 CloseInFlight resolution without flat: re-attempt within budget,
\* else HeldIncident (fence latched, plan Active, operator escalated --
\* never a false "complete", never an unbounded dump; Cx5-1).
ActCloseResolve ==
  /\ actPhase = "closeInFlight" /\ ~actCloseLive /\ pos # "flat"
  /\ actPhase' = IF attemptsLeft > 0 THEN "reduce" ELSE "held"
  /\ UNCHANGED << attemptsLeft, actuatedFlat >>
  /\ UNCHANGED << venueVars, botVars, signerVars, envVars, ctlVars, planVars, flagVars, crashBudget >>

\* Completion evidence (R1d-F7 + Cx5-1, "one gate, no gap"): proved flat --
\* SS2 finality of every order + zero reconciled allocations -- then the
\* durable ActuatedFlat record. NC2 (EVIDENCE_GATED=FALSE): issuance-gated
\* -- "complete" as soon as a close was issued, pos unchecked.
ActMarkFlat ==
  /\ actPhase \in { "reduce", "closeInFlight", "held" }
  /\ IF EVIDENCE_GATED
       THEN pos = "flat" /\ NoLiveOrders
       ELSE actPhase = "closeInFlight"
  /\ actuatedFlat' = TRUE /\ actPhase' = "finalize"
  /\ UNCHANGED << attemptsLeft >>
  /\ UNCHANGED << venueVars, botVars, signerVars, envVars, ctlVars, planVars, flagVars, crashBudget >>

\* SM SS6 actuator-side deactivation: drive the idempotent tombstone from
\* the durable ActuatedFlat record; enabled from finalize AND from standby
\* after a crash (the re-drive; R1d-F7). Tombstoning without ActuatedFlat
\* is impossible -- "a premature tombstone is forbidden".
ActDeactivate ==
  /\ actuatedFlat /\ plan = "active"
  /\ plan' = "tombstone"
  /\ UNCHANGED << leaseFree, hadExposure >>
  /\ UNCHANGED << venueVars, botVars, signerVars, envVars, ctlVars, actVars, flagVars, crashBudget >>

\* Completion also covers supersession: plan = "active" with actuatedFlat
\* reset means a SUCCESSOR trade occupies the one-slot plan abstraction --
\* reachable only via old-tombstone -> lease -> replicate, so the old
\* trade's Deactivate already acked (per-TradeID stores ack idempotently;
\* single-slot artifact, documented in the correspondence map).
ActDone ==
  /\ actPhase = "finalize"
  /\ (plan = "tombstone" \/ (plan = "active" /\ ~actuatedFlat))
  /\ actPhase' = "done"
  /\ UNCHANGED << attemptsLeft, actuatedFlat >>
  /\ UNCHANGED << venueVars, botVars, signerVars, envVars, ctlVars, planVars, flagVars, crashBudget >>

\* Actuator crash: volatile phase lost; durable state (the latched epoch,
\* ActuatedFlat, the replicated budget, the plan) survives.
ActCrash ==
  /\ actPhase # "standby" /\ CrashAllowed
  /\ actPhase' = "standby" /\ crashBudget' = CrashSpend
  /\ UNCHANGED << attemptsLeft, actuatedFlat >>
  /\ UNCHANGED << venueVars, botVars, signerVars, envVars, ctlVars, planVars, flagVars >>

\* Actuator reconcile-only boot (SM SS6): enumerate via ListActive (a
\* tombstoned plan is never returned/re-actioned), resume from durable
\* state: with the epoch latched, re-enter at the quiesce barrier (safe,
\* idempotent; own stale orders are re-quiesced). ActuatedFlat re-drive is
\* ActDeactivate above. An un-latched standby with an active plan re-enters
\* via WatchdogFire.
ActResume ==
  /\ actPhase = "standby" /\ incidentActive /\ latched
  /\ plan = "active" /\ ~actuatedFlat
  /\ actPhase' = "quiesce"
  /\ UNCHANGED << attemptsLeft, actuatedFlat >>
  /\ UNCHANGED << venueVars, botVars, signerVars, envVars, ctlVars, planVars, flagVars, crashBudget >>

(***************************************************************************)
(* Venue actions: fills, zero-fills, races. Global -- a venue order can    *)
(* resolve at any time regardless of process phases (late fills).          *)
(***************************************************************************)
VEntryFill ==
  /\ entryLive /\ pos = "flat"
  /\ pos' = "neutral" /\ entryLive' = FALSE
  /\ hadExposure' = TRUE
  /\ UNCHANGED << protLive, botCloseLive, actCloseLive, plan, leaseFree >>
  /\ UNCHANGED << botVars, signerVars, envVars, ctlVars, actVars, flagVars, crashBudget >>

VEntryTerminal ==
  /\ entryLive
  /\ entryLive' = FALSE
  /\ UNCHANGED << pos, protLive, botCloseLive, actCloseLive >>
  /\ UNCHANGED << botVars, signerVars, envVars, ctlVars, actVars, planVars, flagVars, crashBudget >>

\* An armed stop firing: on a neutral package it CREATES a naked leg (the
\* SM SS5 naked-leg-generator rationale); on a naked package it may flatten;
\* firing with no position opens exposure -- exactly the "armed stop fires
\* after the reduction" hazard the SS4 quiesce barrier exists for.
VProtFire ==
  /\ protLive
  /\ protLive' = FALSE
  /\ pos' = CASE pos = "neutral" -> "naked"
              [] pos = "naked"   -> "flat"
              [] pos = "flat"    -> "naked"
  /\ UNCHANGED << entryLive, botCloseLive, actCloseLive >>
  /\ UNCHANGED << botVars, signerVars, envVars, ctlVars, actVars, planVars, flagVars, crashBudget >>

VBotCloseFill ==
  /\ botCloseLive
  /\ botCloseLive' = FALSE /\ pos' = "flat"
  /\ UNCHANGED << entryLive, protLive, actCloseLive >>
  /\ UNCHANGED << botVars, signerVars, envVars, ctlVars, actVars, planVars, flagVars, crashBudget >>

VBotCloseZeroFill ==
  /\ botCloseLive
  /\ botCloseLive' = FALSE
  /\ UNCHANGED << pos, entryLive, protLive, actCloseLive >>
  /\ UNCHANGED << botVars, signerVars, envVars, ctlVars, actVars, planVars, flagVars, crashBudget >>

VActCloseFill ==
  /\ actCloseLive
  /\ actCloseLive' = FALSE /\ pos' = "flat"
  /\ UNCHANGED << entryLive, protLive, botCloseLive >>
  /\ UNCHANGED << botVars, signerVars, envVars, ctlVars, actVars, planVars, flagVars, crashBudget >>

VActCloseZeroFill ==
  /\ actCloseLive
  /\ actCloseLive' = FALSE
  /\ UNCHANGED << pos, entryLive, protLive, botCloseLive >>
  /\ UNCHANGED << botVars, signerVars, envVars, ctlVars, actVars, planVars, flagVars, crashBudget >>

(***************************************************************************)
(* Operator actions.                                                      *)
(***************************************************************************)

\* Manual kill (venue UI / 2FA): branch 3, HeldIncident, and the $1K+
\* fence-unavailable floor all escalate here -- and during ANY incident the
\* operator's venue-UI kill is available (alerting runs in parallel from
\* the first-safe-action; core SS7.4). Uses neither modeled credential (no
\* writer flags). Atomically flattens + clears resting orders (a UI
\* cancel-all + close; abstraction noted in the map).
OpManualKill ==
  /\ incidentActive
  /\ (pos # "flat" \/ ~NoLiveOrders)
  /\ pos' = "flat"
  /\ entryLive' = FALSE /\ protLive' = FALSE
  /\ botCloseLive' = FALSE /\ actCloseLive' = FALSE
  /\ UNCHANGED << botVars, signerVars, envVars, ctlVars, actVars, planVars, flagVars, crashBudget >>

\* Return to normal operation (R1d-F6): only after the branch's terminal
\* outcome (flat/reconciled), with explicit operator clearance + a fresh
\* propagation test (abstracted into this atomic action). Un-latches the
\* epoch ("honored across restart until explicit operator clearance").
OpClearIncident ==
  /\ incidentActive /\ pos = "flat" /\ NoLiveOrders
  /\ actPhase \in { "standby", "done", "unavail" }
  /\ latched' = FALSE /\ incidentActive' = FALSE
  /\ fenceCtl' = "ready" /\ branchMax' = 0
  /\ actPhase' = "standby"
  /\ botPhase' = IF botUp THEN "normal" ELSE botPhase
  /\ UNCHANGED << signerUp, botUp, attemptsLeft, actuatedFlat >>
  /\ UNCHANGED << venueVars, envVars, planVars, flagVars, crashBudget >>

(***************************************************************************)
(* Next / Spec                                                            *)
(***************************************************************************)
Next ==
  \/ BotReplicatePlan \/ BotEntrySubmit \/ BotCloseSubmit \/ BotCancelProt
  \/ BotDeactivate \/ BotLeaseRelease \/ BotCrash \/ BotReboot
  \/ SignerCrash \/ SignerRestart
  \/ LatchableFlip \/ BackupFlip \/ ActReadyFlip
  \/ CtlDegrade \/ CtlDegradedRepair \/ CtlLoss
  \/ CtlBranch1 \/ CtlBranch2 \/ CtlBranch3
  \/ CtlEscalate1to2 \/ CtlEscalate1to3 \/ CtlEscalate3to2
  \/ WatchdogFire \/ ActLatch \/ ActLatchFail \/ ActUnavailRetry
  \/ ActCanaryUnfencedClose
  \/ ActQuiesceCancelEntry \/ ActQuiesceCancelProt
  \/ ActQuiesceCancelBotClose \/ ActQuiesceCancelOwnStale
  \/ ActQuiesceDone \/ ActReconcileDone
  \/ ActReduceIssue \/ ActReduceExhausted \/ ActCloseResolve
  \/ ActMarkFlat \/ ActDeactivate \/ ActDone \/ ActCrash \/ ActResume
  \/ VEntryFill \/ VEntryTerminal \/ VProtFire
  \/ VBotCloseFill \/ VBotCloseZeroFill
  \/ VActCloseFill \/ VActCloseZeroFill
  \/ OpManualKill \/ OpClearIncident

Spec == Init /\ [][Next]_vars

\* Obligated action variants (fairness scopes). The unconstrained actions
\* stay in Next as adversarial possibilities; fairness attaches only to
\* the spec-obligated cases.
\* Core SS7.4: the watchdog MUST act after the bot host dies -- the
\* obligated fire is real host silence; spurious fires stay unfair.
WatchdogFireObligated == WatchdogFire /\ ~botUp
\* SM SS3 branch 1: the wind-down is a bot OBLIGATION (composite RTO),
\* not an option -- close submission and protection disarm must progress.
ObligatedWindDown == botPhase = "winddown" /\ (BotCloseSubmit \/ BotCancelProt)

\* Fairness for the liveness config only: venue orders eventually resolve;
\* the engine's arbitration logic, the actuator protocol, the obligated
\* wind-down, and the obligated watchdog make progress; the operator
\* eventually acts on an escalated incident (STRONG fairness -- the
\* operator's enabling window can be made intermittent by a flapping
\* environment, and branch 3 / held / unavail are explicitly
\* operator-resolution targets in the spec). Crashes, env degradations,
\* spurious watchdog fires, and discretionary bot behavior stay unfair
\* (adversarial). Liveness is claimed only for behaviors with finitely
\* many crashes (BOUND_CRASHES).
Fairness ==
  /\ WF_vars(VEntryFill \/ VEntryTerminal)
  /\ WF_vars(VProtFire)
  /\ WF_vars(VBotCloseFill \/ VBotCloseZeroFill)
  /\ WF_vars(VActCloseFill \/ VActCloseZeroFill)
  /\ WF_vars(CtlLoss)
  /\ WF_vars(CtlBranch1) /\ WF_vars(CtlBranch2) /\ WF_vars(CtlBranch3)
  /\ WF_vars(CtlEscalate1to2) /\ WF_vars(CtlEscalate1to3) /\ WF_vars(CtlEscalate3to2)
  /\ WF_vars(WatchdogFireObligated) /\ WF_vars(ObligatedWindDown)
  \* STRONG fairness on the latch: an anti-phase flapping environment can
  \* make ActLatch's enabling window intermittent forever; SF encodes "a
  \* latch mechanism available infinitely often eventually latches" (the
  \* actuator keeps retrying across T_fence windows), after which the
  \* takeover runs to completion against the fenced bot (SM SS4).
  /\ SF_vars(ActLatch) /\ WF_vars(ActLatchFail)
  /\ WF_vars(ActQuiesceCancelEntry) /\ WF_vars(ActQuiesceCancelProt)
  /\ WF_vars(ActQuiesceCancelBotClose) /\ WF_vars(ActQuiesceCancelOwnStale)
  /\ WF_vars(ActQuiesceDone) /\ WF_vars(ActReconcileDone)
  /\ WF_vars(ActReduceIssue) /\ WF_vars(ActReduceExhausted)
  /\ WF_vars(ActCloseResolve) /\ WF_vars(ActMarkFlat)
  /\ WF_vars(ActDeactivate) /\ WF_vars(ActDone) /\ WF_vars(ActResume)
  \* STRONG fairness for the operator: branch 3 / HeldIncident / the $1K+
  \* fence-unavailable floor are explicitly operator-resolution states (a
  \* manual-kill escalation TARGET, not an automated RTO), and an
  \* adversarially flapping environment (latch <-> unavail) makes the
  \* operator's enabling window intermittent -- WF would be starved by a
  \* scheduler that only offers the operator disabled moments. SF says: an
  \* escalated incident that is actionable infinitely often is eventually
  \* acted on.
  /\ SF_vars(OpManualKill) /\ SF_vars(OpClearIncident)

LiveSpec == Spec /\ Fairness

(***************************************************************************)
(* Invariants (design-doc numbering; I5 discharged by the port contract   *)
(* -- see the correspondence map).                                        *)
(***************************************************************************)
TypeOK ==
  /\ pos \in { "flat", "neutral", "naked" }
  /\ entryLive \in BOOLEAN /\ protLive \in BOOLEAN
  /\ botCloseLive \in BOOLEAN /\ actCloseLive \in BOOLEAN
  /\ botUp \in BOOLEAN
  /\ botPhase \in { "normal", "winddown", "reconcile" }
  /\ signerUp \in BOOLEAN /\ latched \in BOOLEAN
  /\ latchable \in BOOLEAN /\ backupOK \in BOOLEAN /\ actReadyOK \in BOOLEAN
  /\ fenceCtl \in { "ready", "degraded", "lost", "winddown", "manual", "takeover" }
  /\ branchMax \in 0..3
  /\ incidentActive \in BOOLEAN
  /\ actPhase \in { "standby", "latch", "quiesce", "reconcile", "reduce",
                    "closeInFlight", "held", "unavail", "finalize", "done" }
  /\ attemptsLeft \in 0..MAX_CLOSE_ATTEMPTS
  /\ actuatedFlat \in BOOLEAN
  /\ plan \in { "none", "active", "tombstone" }
  /\ leaseFree \in BOOLEAN /\ hadExposure \in BOOLEAN
  /\ botLandedPostLatch \in BOOLEAN /\ actLandedUnfenced \in BOOLEAN
  /\ crashBudget \in 0..CRASH_BUDGET

\* I1 -- never two writers (core SS7.4; SM SS4 single-writer-by-transfer +
\* the SS3 tiered fence-unavailable floor).
I1_NeverTwoWriters == ~botLandedPostLatch /\ ~actLandedUnfenced

\* I2 -- host-independent coverage: open exposure implies a ListActive-
\* visible plan (replicate-before-exposure + no premature tombstone).
I2_HostIndependentCoverage == pos # "flat" => plan = "active"

\* I3 -- evidence-gated terminal states: "done" and the tombstone both mean
\* proved flat with final surfaces (R1d-1, R1d-F7, Cx5-1).
I3_EvidenceGatedTerminal ==
  /\ actPhase = "done"   => (pos = "flat" /\ NoLiveOrders)
  /\ plan = "tombstone"  => (pos = "flat" /\ NoLiveOrders)

\* I4 -- the hedge-group lease never releases with an actionable plan
\* (core SS6 step 9).
I4_LeaseTombstone == leaseFree => plan # "active"

\* I6 -- HeldIncident honesty (Cx5-1): the fence stays latched in held.
\* The "plan stays Active" half of the prose claim is about the ACTUATOR's
\* own held-path never tombstoning; as a STATE property it is discharged by
\* I2+I3 (any tombstone is evidence-gated, so a held incident with open
\* exposure implies an Active plan). A healthy bot may legitimately
\* tombstone a manually-killed flat trade while the actuator's phase
\* variable still reads "held" -- unsynchronized processes, not a defect.
I6_HeldHonest == actPhase = "held" => latched

\* I7 -- escalate-only, one-way-per-incident arbitration (R1d-F6).
I7_EscalateOnly ==
  /\ fenceCtl = "winddown" => branchMax = 1
  /\ fenceCtl = "manual"   => branchMax = 2
  /\ fenceCtl = "takeover" => branchMax = 3

\* Liveness (LiveSpec only): every incident eventually resolves to operator
\* clearance -- through wind-down, takeover-to-flat, HeldIncident + manual
\* kill, or the fence-unavailable floor + manual kill -- given the stated
\* fairness (engine/actuator/venue progress, operator eventually acts,
\* finitely many crashes). This is the SS7.4 honest guarantee plus the
\* operator escalation targets, checked as one reachability claim.
IncidentResolves ==
  [] ( incidentActive => <> ~incidentActive )

===================================================================================
