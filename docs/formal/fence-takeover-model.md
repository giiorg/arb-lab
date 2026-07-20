# Formal model (b) — FenceTakeover: correspondence map, results, FM-1

**Date:** 2026-07-20 · **Model:** `FenceTakeover.tla` (TLA+, checked with TLC 2.19 / tla2tools v1.7.4) · **Doc set at modeling time:** v0.9 → **found FM-1, resolved in v0.9.1**
**Status:** companion artifact — `state-machines.md` (SM) stays normative; on any disagreement the SM/core wins and the model is wrong. Design: `docs/superpowers/specs/2026-07-20-formal-model-b-fence-takeover-design.md`.

## 1. What was checked, and the headline results

The model composes SM §3 (FenceReadyLost arbitration), SM §4 (actuator takeover), SM §6 (EmergencyPlan lifecycle) with an adversarial environment: bot/signer/actuator crashes and restarts (unbounded in the safety configs), health-dimension flapping, watchdog false positives (partition abstraction), venue fill/cancel races and late fills, and operator actions.

| Run | Config | Expectation | Result | States (generated / distinct) |
|---|---|---|---|---|
| Safety (nominal) | `FenceTakeover.cfg` | all invariants hold | **PASS** | 2,969,602 / 320,624 |
| Liveness (nominal) | `FenceTakeover_Live.cfg` | `IncidentResolves` under fairness | **PASS** | 2,057,168 / 263,008 |
| NC1 non-durable latch | `_NC1.cfg` | violate I1 | **VIOLATED I1** (7 steps) | — |
| NC2 issuance-gated completion | `_NC2.cfg` | violate I2/I3 | **VIOLATED I3** (4 steps) | — |
| NC3 unfenced action at `$1K`+ | `_NC3.cfg` | violate I1 | **VIOLATED I1** (8 steps) | — |
| NC4 no trade-terminality (**FM-1**) | `_NC4.cfg` | violate I2/I3 | **VIOLATED I3** (11 steps) | — |

Checked invariants (nominal): **I1** never-two-writers (no bot write lands post-latch pre-clearance; no actuator write lands un-latched), **I2** host-independent coverage (`pos ≠ flat ⇒ plan Active`), **I3** evidence-gated terminals (`done`/tombstone ⇒ proved flat + final surfaces), **I4** lease releases only on tombstone, **I6** `HeldIncident ⇒ fence latched`, **I7** escalate-only arbitration, plus `TypeOK`. Liveness: **`IncidentResolves`** — every incident eventually reaches operator clearance, under the fairness stated in §4 below.

Negative-control traces (the model catches the documented hazards — each reproduces a specific spec warning):

- **NC1** (`DURABLE_LATCH = FALSE`): Replicate → SignerCrash → WatchdogFire → **ActLatch** → **SignerRestart (epoch forgotten)** → BotEntrySubmit lands while the actuator is fenced-active → two writers. Verbatim SM §4: "a transiently-crashed signer could restart mid-takeover and restore two writers."
- **NC2** (`EVIDENCE_GATED = FALSE`): Replicate → EntrySubmit → **BotDeactivate on issuance evidence** → tombstone with the entry order still live (I3); the entry then fills into a coverage hole. The Cx5-1/R1d-F7 gate, shown load-bearing in 4 steps.
- **NC3** (`CANARY_UNFENCED = TRUE`): Replicate → Entry → BotCrash → WatchdogFire → **ActLatchFail** → VEntryFill (late fill) → **unfenced actuator close lands** → I1. The §7.4 tier rationale: the canary regime claims no single-writer safety — which is exactly why the `$1K`+ floor is read-only + manual.
- **NC4** (`TRADE_TERMINALITY = FALSE`): the **FM-1** counterexample — §2 below.

## 2. FM-1 — the finding this model surfaced against v0.9 (resolved v0.9.1)

**Trace (11 steps, minimal):** Replicate → WatchdogFire (host-silence; the entry never opened) → ActLatch → QuiesceDone → ReconcileDone → **ActMarkFlat** (durable `ActuatedFlat`) → **ActCrash** (the exact R1d-F7 window between `ActuatedFlat` and `Deactivate`) → **OpClearIncident** (spec-legal: the R1d-F6 clearance preconditions are the branch's terminal outcome *flat/reconciled* + propagation test — the plan tombstone is **not** among them) → **BotEntrySubmit under the same still-`Active` plan** (§6's "ack required before the entry submits" is satisfied by the *old* ack) → **ActDeactivate** — the stale re-driven tombstone lands **under live fresh exposure** → I3, then I2: open exposure with no `ListActive`-visible plan. The actuator would never protect that exposure. Host-independent coverage — the property the whole §6 lifecycle exists for — is broken by the composition of three individually-correct rules (R1d-F7 re-drive × R1d-F6 clearance × §6 ack-before-entry).

**Why it is a spec gap, not a model artifact:** nothing in core §7.4 / SM §3/§4/§6 (v0.9) states what becomes of a trade the takeover ran to completion evidence on. The bot-side close is terminal by construction (§6 step 9 sequences close finality → `Deactivate` → lease), but the actuator-side flatten leaves the trade's disposition unstated: the plan is still `Active`, the ack is still valid, and no rule forbids the cleared bot from re-admitting exposure under it while the tombstone re-drive is still owed.

**Resolution (v0.9.1, tagged `[→ FM-1]`):** **takeover completion evidence terminalizes the trade.** Once the durable `ActuatedFlat` record exists (equally: once any flatten of the trade reaches §2 finality with flat evidence), the trade re-admits no exposure-opening order; its plan lifecycle may only proceed to the §6 tombstone (re-driven across actuator restarts); fresh exposure is admitted solely as a **successor trade through the §6 lease**, which only that tombstone releases. Operator clearance restores trading for successors, never for the terminated TradeID. — The alternative fix (make the tombstone ack a clearance precondition) was **rejected**: it would couple operator clearance to plan-store availability, the same coupling R1d-5 rejected for hot-path generation acks; the terminality rule closes the hazard without it, because the lease already sequences successors on the tombstone.

**Normative edits:** core §7.4 (takeover bullet — terminality sentence), SM §4 (completion invariant), SM §6 (scope + actuator-side row note). Model: the `TRADE_TERMINALITY` constant guards `BotEntrySubmit` (nominal TRUE); **NC4 keeps the pre-fix world permanently reproducible.**

## 3. Correspondence map

Notation: SM row → model action(s). "OOS" = deliberately out of scope with the discharging owner named. The abstraction register (design doc §2) applies throughout: exposure is qualitative (`flat/neutral/naked`), one trade + a lease-gated successor bit, §2-classifier internals belong to proposed model (a).

### SM §3 — FenceReadyLost arbitration

| SM row / rule | Model |
|---|---|
| `FenceReady → FenceDegraded` (backup-only loss; openings continue, R1d-F5) | `CtlDegrade`; openings: `BotEntrySubmit` accepts `fenceCtl ∈ {ready, degraded}` |
| `FenceDegraded → FenceReadyLost` (further loss) | `CtlLoss` (guard spans ready+degraded) |
| `FenceDegraded → FenceReady` (sole health-driven return) | `CtlDegradedRepair` |
| `FenceReady → FenceReadyLost` + first-safe-action | `CtlLoss` sets `incidentActive`; the OpenTrade block is the entry guard. PackageDelever-supersession: OOS (SM §7 / model (a)) |
| Branch 1 (revoke or `ActuatorReady` lost, signing works) | `CtlBranch1` (`botPhase := winddown`) |
| Branch 2 (signing down ∧ latchable ∧ `ActuatorReady`) — hand-off *is* FenceLatch | `CtlBranch2` → `actPhase := latch` (plan-scoped: engages only with an `Active` plan; an incident with no plan resolves administratively via clearance) |
| Branch 3 (uncertain / `ActuatorReady` unconfirmable) | `CtlBranch3` |
| Escalate-only: 1→2, 1→3 (Cx5-2), 3→2; one-way per incident (R1d-F6) | `CtlEscalate1to2/1to3/3to2`; **I7** + `branchMax`; `BotReboot` preserves a started branch; exit only via `OpClearIncident` (flat + propagation test abstracted atomic) |
| Tiered fence-unavailable floor (`$1K`+ read-only; canary signed-loss regime) | `ActLatchFail → unavail`; `$1K`+ = no unfenced action exists (nominal); `CANARY_UNFENCED` (NC3) is the canary regime; `OpManualKill` is the manual-kill escalation |
| RTO structure (<5 s / <30 s / manual target) | OOS — untimed model; RTOs are §9 chaos-gate numbers. The *shape* (automated first-safe-action, composite completion, manual target) is the liveness layer |
| Health dimensions (R1d-4 cut); durably-fenced = incident state | `SigningOK/RevokeOK/backupOK/actReadyOK`; observed = truth (**monitoring lag not modeled** — a refinement candidate); durably-fenced = `latched` (§4-owned) |

### SM §4 — Actuator takeover

| SM row / rule | Model |
|---|---|
| standby → `FenceLatch` (watchdog: host *and journal* death) | `WatchdogFire` — deliberately fireable with `botUp` (partition false positive; replaces a `partitioned` variable). Journal death is not distinguished: reboot always reconciles from durable state |
| standby → `FenceLatch` (branch-2 hand-off) | `CtlBranch2`/escalations (plan-scoped, above) |
| `FenceLatch → QuiesceSurfaces` (durable monotonic epoch; `VerifyFence` validates the epoch) | `ActLatch` (`latched := TRUE`); durability = `DURABLE_LATCH` in `SignerRestart` (NC1 = the point-in-time-snapshot bug). Signer-isolation vs exchange-side mechanisms are folded into `latched`/`latchable`/`backupOK` |
| `FenceLatch → FenceUnavailable` | `ActLatchFail` — **not** guarded on `~latchable`: the latch attempt is a bounded action (`T_fence`) whose timeout can expire during transient availability; also prevents an anti-phase flapping environment pinning the protocol in `latch` |
| `FenceUnavailable → QuiesceSurfaces` (latch within `T_fence`) | `ActUnavailRetry` → `ActLatch` |
| `QuiesceSurfaces → Reconcile` (all surfaces final; separate cancel surfaces; own stale orders post-crash) | `ActQuiesceCancel{Entry,Prot,BotClose,OwnStale}` racing the venue fill actions; `ActQuiesceDone` requires `NoLiveOrders`. The per-venue cancel-surface mechanics are OOS (Profiles §1/§3/§4) |
| `Reconcile → ReduceResidual` (same-watermark, reconciled live state) | `ActReconcileDone`; sound because post-quiesce, post-latch nothing else mutates `pos` (nominal) — the watermark collapses to reading `pos`. R1d-5 prefix-complete discovery + retention/checkpoint bound: OOS (data-completeness argument, not a concurrency one) |
| `ReduceResidual → CloseInFlight` (state-dependent verb, R1d-1; budget consumed) | `ActReduceIssue` — one abstract close kind whose fill flattens (two-leg close ≈ reduction at this altitude); `attemptsLeft` is the replicated budget, durable across `ActCrash` |
| `CloseInFlight` → complete (evidence-gated, Cx5-1/R1d-F7 "one gate") | venue resolution → `ActMarkFlat` (evidence: `pos = flat ∧ NoLiveOrders`) → durable `actuatedFlat` → `ActDeactivate` → `ActDone`. `EVIDENCE_GATED = FALSE` is NC2 |
| `CloseInFlight → ReduceResidual` (retry) / → `HeldIncident` (exhaustion, unconditional) | `ActCloseResolve`; `ActReduceExhausted`; **I6** (`held ⇒ latched`) |
| crash mid-close → resume from durable state; no double-close via fence+finality | `ActCrash` (volatile phase lost; `latched`/`actuatedFlat`/`attemptsLeft`/plan survive) → `ActResume` (re-enter at quiesce, re-quiescing own stale orders) / `ActDeactivate` re-drive |
| bot recovers mid-takeover → fenced, reconcile-only until clearance | `BotReboot` → `botPhase = reconcile` when `latched` |
| single-writer by transfer; execution authority = actuator's own credential | **I1** (two landing flags); own-credential validity = `actReadyOK` dimension + branch-2 guard; credential mechanics OOS |

### SM §6 — EmergencyPlan lifecycle

| SM row / rule | Model |
|---|---|
| *(none)* → `Active` (replicate-before-exposure; ack before entry) | `BotReplicatePlan` (acquires lease, resets per-trade `attemptsLeft`/`actuatedFlat`/`hadExposure`); `BotEntrySubmit` requires `plan = active` |
| `Active → Active` (generation++; async handle-bearing vs acked policy-changing, R1d-5) | OOS — generations collapsed to one slot; the R1d-5 asymmetry is a replication-freshness argument, not a writer-arbitration one |
| `Active → DeactivationPending → Deactivated` (bot close finality; §7.5 re-drive; idempotent tombstone) | `BotDeactivate` — atomic idempotent tombstone, evidence-gated; the pending record + bounded-backoff re-drive is modeled by the action staying enabled across crash/reboot (evidence is re-derivable in-model) |
| Actuator-side path (R1d-F7): flatten evidence → durable `ActuatedFlat` → same tombstone; premature tombstone forbidden; interrupted `Deactivate` re-driven | `ActMarkFlat` → `ActDeactivate` (guarded on `actuatedFlat`; enabled from `finalize` **and** from post-crash `standby` = the re-drive); **I3** forbids the premature tombstone |
| Tombstone rejects same-or-later `Replicate` (reorder/retry); `ListActive` never returns tombstoned; no stale plan re-actioned | **Discharged by the port contract** (design-doc I5): the single-slot model has no reactivation action to check against; the multi-trade stale-plan-vs-successor interleaving is OOS → the port's CAS/tombstone semantics + the §9 integration cases own it. `ActResume`/`ActDeactivate` guards encode "tombstone is terminal" |
| Lease: only the persisted tombstone ack releases / admits a successor | `BotLeaseRelease` (guard `plan = tombstone`); **I4**; successor = the next `BotReplicatePlan` cycle |
| Plan content (handles, cursors, close budget) | `attemptsLeft` models the Cx5-1 budget; the handle set/cursors are OOS (R1d-5) |

### Deliberate model refinements found during checking (model-side, no spec change)

1. **Plan-scoped hand-off:** branch-2 engagement with no `Active` plan left the takeover unfinishable (nothing to tombstone). The takeover machine is plan-scoped (SM §4 standby holds *plans*); an incident with no plan resolves via clearance. Encoded in the three hand-off actions.
2. **Supersession completion:** the one-slot plan abstraction conflated trade identities when a legitimately-tombstoned trade was succeeded mid-`finalize`; per-TradeID stores ack `Deactivate(old)` idempotently. `ActDone` accepts `plan = active ∧ ¬actuatedFlat` (reachable only via old-tombstone → lease → replicate).
3. **I6 narrowed:** "held ⇒ plan Active" is the *actuator's-own-path* claim (it never tombstones from held without evidence), already carried by I2+I3 as state properties; a healthy bot may legitimately tombstone a manually-killed flat trade while the actuator's phase variable still reads `held`. I6 checks the fence half: `held ⇒ latched`.
4. **Liveness layer** (relative to the design doc): the property generalized from "takeover terminates" to **`IncidentResolves`** (`incidentActive ~> cleared`) — the takeover-scoped antecedent mis-fired on a pre-latch actuator crash whose trigger evaporated (standby *is* that takeover's terminal, but the incident lives on bot-side). Fairness encodes the spec's obligations: WF on the engine's arbitration steps, the obligated wind-down (`botPhase = winddown`), the obligated watchdog (`¬botUp`), the actuator protocol, venue resolutions; **SF** on `ActLatch` (an anti-phase flapping environment must not starve a latch that is achievable infinitely often — the actuator retries across `T_fence` windows) and on the two operator actions (branch 3/held/unavail are operator-resolution *targets*; their enabling windows can be made intermittent). Spurious watchdog fires, crashes (bounded by `CRASH_BUDGET = 2` in the liveness config only), env flapping, and discretionary bot behavior stay adversarial. `OpManualKill` is available during any incident (the operator's venue-UI kill; alerting runs in parallel from the first-safe-action).

## 4. Honesty notes / limits

- **Small-scope claim:** passing nominal runs are evidence over *this* abstraction (one trade, qualitative exposure, observed=truth health), not proof at implementation scale. §9 chaos tests remain the implementation gate. The model's value: it exhausts crash × restart × partition × reorder interleavings prose review does not — and it found FM-1 in the first nominal run.
- **Monitoring lag** (observed health ≠ truth) and **multi-trade concurrency** are the two named refinement candidates if this model is extended.
- Liveness is a conditional claim: finitely many crashes + the stated fairness (operator eventually acts). Safety runs have **unbounded** crashes.
- I5 (tombstone permanence) is discharged by the port contract, not checked — see the SM §6 map row.

## 5. How to run

```
cd docs/formal && ./run.sh
```

Fetches `tla2tools.jar` v1.7.4 (SHA-256 pinned) into gitignored `.tools/`, then runs all six configs and checks each against its prediction (nominal safety + liveness must pass; NC1–4 must fail on the predicted invariant). Requires Java 17+. Full outputs land in `.tools/*.out`.
