# Formal model (b) — FenceReadyLost → takeover → EmergencyPlan composition — design

**Date:** 2026-07-20 · **Status:** design for the first formal companion artifact (HANDOFF "Open decisions → Formal specs"; model (b) of the two proposed) · **Doc set:** v0.9

## 1. Purpose & standing

Model-check the **writer-arbitration composition** of the safety kernel — SM §3 (FenceReadyLost arbitration), SM §4 (actuator takeover protocol), SM §6 (EmergencyPlan lifecycle) — against the invariant family the prose promises: **never two writers, host-independent coverage, evidence-gated completion/tombstone**, under crash, partition, signer restart, message reorder/retry, and watchdog false positives.

Standing per HANDOFF (process pinned 2026-07-20):

- `state-machines.md` **stays normative**. The TLA+ spec is a **companion artifact** in `docs/formal/` with a correspondence map back to the SM rows it models.
- **Any counterexample against the *spec's* rules is an FM-tagged finding** (fresh prefix `FM-1…`, recorded in ADR §3), never a silent fix. Counterexamples against my own model-encoding errors are ordinary dev iteration, no tag.
- This artifact **informs, not forecloses,** the user's open TLA+ adoption decision. It is deliberately standalone: the R1d rulings (R1d-1 terminal-flat takeover, R1d-4 `ActuatorReady`, R1d-5 prefix-complete discovery, R1d-F5/F6/F7) plus Cx5-1 (evidence-gated completion, close-attempt budget) fixed model (b)'s scope; model (a) (classifier/finality/reservations) is not started here.

## 2. What is modeled (and what deliberately is not)

**In scope — the question the model answers:** can any interleaving of {bot crash/recover, signer crash/restart, health-dimension loss/repair, watchdog false fire under partition, actuator crash/reboot, venue fills/cancel races/late fills, plan-store message reorder/retry, operator clearance} reach a state where

1. two principals' exposure-changing orders land in the same un-cleared incident (two writers),
2. nonzero exposure exists with the bot host dead and **no** `Active` plan (coverage hole),
3. the takeover reports complete / the plan tombstones **without** proved-flat evidence (false flat / premature tombstone),
4. the hedge-group lease releases (successor admitted) with the plan still actionable, or a tombstoned plan is re-actioned,
5. a started arbitration branch de-escalates or health-flap re-enters normal operation without operator clearance.

**Out of scope (owned elsewhere, abstracted here):**

- **Delta arithmetic & the §2 classifier** — model (a)'s job. Exposure is qualitative: `Flat / Neutral / Naked` (one package, one venue-abstract account). "Quiesce to finality" is an abstract barrier: a live order either fills (venue race) or reaches terminal; the four-step finality contract's internals are not re-modeled.
- **Interval-floor arithmetic** (the optional Lean lemma), RTO clocks/latency numbers (safety is untimed; `T_fence`/`T_close` appear only as budget-exhaustion nondeterminism), venue REST/WS mechanics, rate limits, and the prefix-complete *discovery* internals of R1d-5 (modeled as: reconcile reads venue truth; the retention/checkpoint bound is a data-completeness argument, not a concurrency one).
- Multi-trade concurrency: **one trade + an abstract successor-admission bit** (enough to state the lease/tombstone hazard); N-trade generalization adds states, not new interleavings of this composition.

## 3. Model architecture (processes → variables)

Single TLA+ module, explicit-state (TLC). Actors as action groups over shared state:

- **Venue (truth):** `pos ∈ {Flat, Neutral, Naked}`; live-order flags `botEntryLive`, `botProtLive`, `botCloseLive`, `actCloseLive`. Fills are nondeterministic: entry fill `Flat→Neutral` (both-leg landing abstracted — one-leg residuals are model (a)); protection fire `Neutral→Naked` or `Naked→Flat`; close fill `→Flat` (two-leg close, R1d-1) or zero-fill (attempt consumed); cancels race fills.
- **Bot:** `botUp`, `botPhase ∈ {normal, winddown, reconcile, halted}`. Writes go **through the signer**: a bot submit lands only if the signer is up and un-latched — a partitioned-but-alive bot keeps trying (it sees no incident), which is exactly the hazard the fence must absorb.
- **Signer (own trust boundary):** `signerUp`, `epochLatched`. **`DURABLE_LATCH` constant:** TRUE → restart preserves the latch (the SM §4 epoch contract); FALSE → restart clears it (negative control).
- **Health dimensions (SM §3, R1d-4 cut):** `signingOK` (derived: signer up ∧ un-latched ∧ bot↔signer link), `revokeOK` (latch achievable), `backupOK` (exchange-side), `actReadyOK`. Environment degrades/repairs them; `fenceCtl ∈ {ready, degraded, lost, winddown, takeover, manual}` per the §3 table — `degraded` on backup-only loss (openings continue, R1d-F5); branch selection on the R1d-4 guards (branch 2 requires latchable ∧ `ActuatorReady` confirmed); **escalate-only** re-arbitration and clearance-only exit (R1d-F6).
- **Watchdog/partition:** `partitioned` — watchdog may fire on host-silence that is a partition, not death (false positive). Takeover entry = watchdog fire ∨ §3 branch-2 hand-off.
- **Actuator (SM §4):** `actPhase ∈ {standby, latch, quiesce, reconcile, reduce, closeInFlight, held, unavail, done}`, `attemptsLeft` (the Cx5-1 replicated budget, `MAX_CLOSE_ATTEMPTS = 2` in-model), `actuatedFlat` (durable evidence record, R1d-F7). Actuator crash/reboot re-enters via `ListActive` + re-drives from durable records. `FenceUnavailable` floor: at `$1K`+ the actuator never trades un-latched; **`CANARY_UNFENCED` constant** TRUE relaxes it (negative control encoding the §7.4 tier rationale).
- **Plan store (SM §6):** `plan ∈ {none, active, deactPending, tombstone}`, `staleRepInFlight` (a delayed `Replicate` that can arrive post-tombstone), `leaseFree`, `successorOpen`. Ack-before-entry; tombstone idempotent + rejects re-replication; bot-side and actuator-side (`ActuatedFlat`) deactivation paths; crash-between-close-and-ack re-drive. **`EVIDENCE_GATED` constant:** TRUE → completion/tombstone require `pos = Flat ∧ all orders terminal` (the Cx5-1/R1d-F7 unified gate); FALSE → issuance-gated (negative control).
- **History flags (for action-level invariants):** `botLandedPostLatch`, `actLandedUnfenced`, `branchDeescalated`, `planReactivated`.
- **Operator:** clearance (requires branch-terminal outcome flat/reconciled + fresh propagation test — un-latches, resets incident), manual kill (venue-UI close in `manual`/`held`/`unavail` — no writer flag; it is neither credential).

## 4. Checked properties

Invariants (nominal run — all must hold):

| # | Invariant | Encodes |
|---|---|---|
| I1 | `¬botLandedPostLatch ∧ ¬actLandedUnfenced` | **Never two writers** — no bot write lands after the epoch latch (pre-clearance); no actuator write lands un-latched at `$1K`+ (core §7.4; SM §4 single-writer-by-transfer + the fence-unavailable floor) |
| I2 | `pos ≠ Flat ⇒ plan = active` | **Host-independent coverage** — replicate-before-exposure/ack-before-entry + no-premature-tombstone (SM §6; R1d-F7) |
| I3 | `actPhase = done ⇒ pos = Flat ∧ no live orders` ∧ `plan = tombstone ⇒ pos = Flat ∧ no live orders` | **Evidence-gated completion & tombstone** — terminal-flat, never issuance-gated (R1d-1, Cx5-1, R1d-F7 "one gate, no gap") |
| I4 | `successorOpen ⇒ plan = tombstone` (∧ `leaseFree ⇒ plan = tombstone`) | Lease never released with an actionable plan (core §6 step 9) |
| I5 | `¬planReactivated` | Tombstone rejects same-or-later-generation `Replicate` under reorder/retry (SM §6 port semantics) |
| I6 | `actPhase = held ⇒ epochLatched ∧ plan = active` | `HeldIncident` honesty — fence latched, plan active, never a false flat (Cx5-1) |
| I7 | `¬branchDeescalated` | One-way-per-incident, escalate-only arbitration (R1d-F6; Cx5-2's 1→3 edge included) |

Liveness (nominal, under weak fairness on resolution/actuator/venue-terminal actions): **takeover started ⇒ eventually `done` ∨ `held` ∨ operator-resolved** — the honest §7.4 guarantee ("flat when the close can execute within plan bounds; held with single-writer preserved otherwise"), with the close-attempt budget forcing `held` instead of livelock on repeated zero-fills.

**Negative controls** (the model is load-bearing only if it catches the documented hazards):

| Control | Constant flip | Predicted violation | Prose it reproduces |
|---|---|---|---|
| NC1 | `DURABLE_LATCH = FALSE` | I1 via signer crash+restart mid-takeover; partitioned live bot writes again | "a transiently-crashed signer could restart mid-takeover and restore **two writers**" (SM §4) |
| NC2 | `EVIDENCE_GATED = FALSE` | I2/I3 via zero-fill close + issuance-gated complete/tombstone | "never a false 'flat' on a zero-fill close" (Cx5-1); premature tombstone (R1d-F7) |
| NC3 | `CANARY_UNFENCED = TRUE` | I1 via actuator acting after `T_fence` with no latch while the bot recovers | the §7.4 tier rationale — canary regime "explicitly does *not* claim single-writer safety" |

## 5. Deliverables & execution plan

1. `docs/formal/FenceTakeover.tla` + `FenceTakeover.cfg` (nominal) + `FenceTakeover_NC{1,2,3}.cfg` — constants-only variations.
2. `docs/formal/run.sh` — fetches a **pinned** `tla2tools.jar` (version + SHA-256 checked) to a gitignored `.tools/` dir, runs all four configs, prints pass/fail vs prediction. Java 25 present locally.
3. `docs/formal/fence-takeover-model.md` — scope + abstraction register (§2 above), **correspondence map** (every SM §3/§4/§6 transition row → TLA action name, or an explicit "abstracted/out-of-scope" entry — no silent omissions), TLC results (state counts, violation traces for NC1–3), how-to-run.
4. TLC runs recorded; **any spec-level counterexample → FM-tagged finding** in ADR §3 + HANDOFF, per process.
5. Codex convergence round on model + correspondence claims (repo convention), then HANDOFF/memory update. No version bump — companion artifact, no normative change. No AI attribution in commits.

**Process note (autonomous session):** brainstorming's interactive approval gate is collapsed — the task, tooling, and process rules were fixed with the user across rounds 3–4 and offered as this session's continuation; the design doc is committed separately so the design remains independently reviewable, and writing-plans is folded into §5 (single-artifact task; the doc-split precedent's multi-task plan machinery isn't warranted).

## 6. Risks / honesty notes

- **Abstraction risk:** a too-coarse venue (qualitative `pos`) could mask real interleavings; mitigated by the correspondence map's per-row accounting + the Codex round explicitly prompted to attack the abstractions.
- A passing nominal run over a small state space is **evidence, not proof**, of the composition's safety at implementation scale; the §9 chaos tests remain the implementation gate (HANDOFF Residual follow-ups). The model's primary value is exhausting *interleavings* humans don't enumerate — crash × restart × partition × reorder.
- If TLC finds a genuine hole in the R1d-resolved rules, that is a **success condition** of this exercise (FM finding), not a failure of the round-4 work.
