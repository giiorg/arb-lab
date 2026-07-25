# ADR & Review History — Funding-Rate Arbitrage Platform

**Status:** v0.9.3 (2026-07-25) — formal-model round-2 release: ADR §3 gains the **Cx7 block** (the round-2 re-review of the model) and **FM-3** (the branch-2 re-engagement silence — the first spec defect found by attacking a *previous round's fix*). v0.9.2 added the **Cx6 block** (the Codex adversarial round on the model itself) and **FM-2** (found by that round's liveness re-verification). v0.9.1 added the **FM block** (model-checker findings, `docs/formal/`); v0.9 was the round-4 content release: ADR §3 gains the R1d block; the ADR §4 FLAG list records the seven rulings.
**Doc set:** normative core `build-spec.md` · venue facts `venue-profiles.md` · kernel machines `state-machines.md` · history/traceability `adr-history.md` · teaching companion `tutorial.md`.
**Role:** the convergence record — how the spec got here (review rounds, Codex convergence), plus the finding→resolution traceability that used to be the build-spec Appendix. Nothing here is normative on its own; the requirements live in the core.

## 1. Convergence narrative (v0.1 → v0.7.1)

The v0.7.1 status paragraph, frozen verbatim:

Draft v0.7.1 (2026-07-19) — Codex adversarial rounds 1–4 (11 → 6 → 4 → 4), then **two external human reviews (R1, R2) + a Codex convergence pass → v0.6**, then a **second external round (R1, R2) + 4 Codex convergence rounds → v0.7** (both reviews applied in full — near-zero false positives — plus 17 Codex convergence findings, all valid and applied; the **safety kernel** took most of them; new venue/tooling facts re-verified against official docs 2026-07-19). v0.7 hardens the **safety kernel**: the one-leg-failure handler now classifies residuals **determined vs ambiguous by *remaining executable quantity*** and quiesces-to-finality before compensating (a compensating trade against a `PartiallyFilled`/pending counter-leg could otherwise *create* the exposure it removes); a durable **`DispatchAuthorized`** crash boundary makes recovery query-only; rescue margin under Isolated is an **active `add-margin` top-up** with a **`PackageDelever`** fallback; a per-venue **`Fence`** capability (with signer isolation pulled to `$1K` where exchange-side fencing is unavailable); **one-way position-mode + leverage** posture verification; an **operator control-plane security** contract; a hardened, **calibration-artifact-gated** economic bound; **raw-arrival** (not `SourceTime`) event ordering; and SQLite **WAL + `synchronous=FULL`** durability. **Deliberately not "converged":** implementation-ready *by capital tier* (§1.1) — **foundation + read-path + edge-proof (milestones 1–2) are fully build-ready**; the **off-host actuator *and* the ambiguous-order-recovery state machine are the safety kernel** (§6.7, §7.4): the ambiguous-order machine is **model-tested *within* milestone 3 before live capital**, and the takeover is **prototyped + chaos-tested in its later actuator workstream** and proven in a **sub-`$1K` canary** — an explicit acceptance of bounded loss (§12) — before it gates real size. Before real capital: an empirical testnet liquidation test of the §4.1 posture. **v0.7.1** applies a **third external round (R1c, R2c) + a 5-round Codex convergence pass (→ converged)** — refinements/corrections on the v0.7 architecture, not a new redefinition: liq-distance is **mark-price-driven** (recomputed on every fresh mark, not only account events) with a **stale-mark block/escalate** `[→ R2c-1]`; a continuously-monitored, **differentiated `FenceReady`** health gate whose loss drives a **bounded `FenceReadyLost` wind-down (or actuator takeover if the signer itself failed)** within an RTO `[→ R2c-2, Cx4-5]`; first-class safety-capability **ports** (`MarginGateway`/`FenceGateway`/`ProtectionGateway`/`EmergencyPlanStore`; `TransferCollateral` gains submit-unknown semantics) `[→ R2c-3]`; an explicit **three-failure-domain topology** (trading engine ∥ operator/control ∥ a minimal actuator daemon with no dashboard/HTTP surface) `[→ R2c-4]`; the **dispatch sequence decoupled** from raw-arrival order (no cross-source head-of-line blocking; held/coalesced inputs need no 1:1 mapping) `[→ R2c-6]`; **signer isolation elevated to the primary `Fence`** (exchange-side disable the backup) with the Bybit fence credential's unavoidable **transfer-class authority** stated honestly `[→ R1c-2]`; **interval-floor** compensation in the deadline-exceeded ambiguous branch `[→ R1c-4]`; a **quantified canary loss bound** `[→ R2c-8]`; real **backup/anchor/retention/restore-drill** durability requirements `[→ R2c-9]`; a **near-risk-free, dated/haircut/revision-controlled hurdle** `[→ R2c-10]`; and tutorial + provenance corrections `[→ R2c-5, R2c-11]`. R2's two canary blockers (mark-driven liq, quantified loss) and two `$1K` blockers (incident-available fence, explicit safety-plane topology) are **closed at the design layer here**; their residual gating is now implementation + chaos-test, as the spec brackets it. Open: a normative/ADR/tutorial **doc split** (both R2 rounds — now escalated from optional to recommended) and a "what changed" tutorial appendix.

## 2. Review records

- `docs/review/2026-07-20-10-v0.9-formal-model-b-FM-1-record.md` — **formal model (b)** round (TLA+ fence/takeover/plan composition): FM-1 finding + resolution → v0.9.1; **its Cx6 addendum (same file): the Codex adversarial round on the model + FM-2 → v0.9.2**; **its Cx7 addendum: the round-2 re-review + FM-3 + Cx7-1/-2/-3 → v0.9.3 (read that addendum's Provenance section first — the Codex dispatch died on an upstream outage and the round was completed by self-probing, so an independent Codex round-2 verdict is still owed)**; model + results: `docs/formal/`.
- `docs/review/2026-07-20-09-v0.8-round4-R1d-review-and-resolution-record.md` — **round-4 (R1d)** findings + triage + venue-verification annexes → v0.9 (R2d pending).
- `docs/review/2026-07-19-08-v0.7.1-review-and-verification-record.md` — round-3 (R1c/R2c) findings + resolutions + Codex convergence pass.
- `docs/review/2026-07-19-07-v0.7-review-and-verification-record.md` — round-2 (R1/R2 second round) findings + resolutions + venue/tooling verification citations (§2 there).
- `docs/review/2026-07-18-06-v0.6-review-and-verification-record.md` — v0.6 round.
- `docs/review/2026-07-18-05-tutorial-critical-review.md` — the earlier critical review the spec resolves.
- `docs/legacy/` — frozen research record (the "why"). Do not edit.

## 3. Finding → resolution traceability

| Finding / Decision | Resolved in |
|---|---|
| P1 spread-vs-level | core §1, core §2.1–2.3 |
| P2 hurdle rate | core §2.1 |
| P3 ADL (realized hedge-failure) | core §2.1, core §7.1 (naked-leg-gap sizing) |
| P4 build-vs-buy | core §11 |
| C1 cost single-source | core §2.4 |
| C2 discrete settlement | core §2.4, core §6 |
| C3 drift-based min-edge | core §2.4 |
| C4 tiered MMR | Profiles §1, core §7.1 |
| C5 buffer policy | core §5.2 |
| C6 inverse perps | core §1, core §4 |
| C7 basis in funding audit | core §6 |
| C8 Bybit dynamic settlement | core §2.4, Profiles §1, core §6 |
| A1 venue adapter | core §3.2, core §4 (segregated capabilities) |
| A2 multi-region / split-ready | core §3.7, core §11 |
| A3 durable state / reconcile-only boot | core §7.5, core §6.10 |
| A4 exactly-once funding | core §5.1 (UNIQUE + ledger txn ID) |
| A5 kill-switch modes | core §7.4 |
| S1 custody/OES | core §7.2 |
| S2 key security | core §7.3 |
| S3 audit durability | core §3.2, core §7.5 (append-only projection) |
| S4 escalation ladder | core §7.4 |
| B1 backtest fidelity | core §3.5, core §8 |
| B2 test taxonomy | core §9 |
| D1 topology (in-process engine) | core §3.0, core §3.1, core §3.7 |
| D2 durability (journal + outbox → embedded store) | core §3.0, core §5, core §6, core §7.5 |
| D3 money (`platform/fixedpoint`) | core §3.0, core §3.6, core §4, core §5.1, core §11 |
| D4 clock (parity dispatcher + server-time) | core §3.0, core §3.5, core §3.6 |
| **X1 durable order/account truth (single-writer OMS)** | core §3.0, core §3.2, core §4 (`OrderByClientID`/`Fills`), core §5.1, core §6 |
| **X2 ADL/margin controls + account contract** | core §2.1, core §4 (`AccountSource`), core §6.8, core §7.1 |
| **X3 Trade ownership + exclusive lease** | core §1, core §3.2, core §5.1 |
| **X4 bus back-pressure / delivery lanes / freshness** | core §3.4 |
| **X5 DDD boundary (strategy input, venue ACL)** | core §3.2, core §3.3, core §3.6 |
| **X6 pre-authorized (not human-gated) emergency** | core §7.4 |
| **X7 fixedpoint intermediates + dimensional types** | core §3.6, core §4, core §5.1 |
| **X8 clock total-order parity + server-time** | core §3.5, core §3.6, core §6.3 |
| **X9 DedupeKey → UNIQUE + ledger txn ID** | core §5.1 |
| **X10 key blast-radius = whole subaccount** | core §7.3 |
| **X11 paper acceptance-gate pass/fail** | core §9 |
| **Y1 fill dedup identity (`ExecutionID` + `UNIQUE`)** | core §4, core §5.1, core §5.3 |
| **Y2 `Place` non-idempotent / `SubmissionUnknown`** | core §4, core §5.3, core §6 |
| **Y3 `ExposureReduction` command (executable ADL path) + actuator failure domain** | core §5.3, core §6, core §7.4 |
| **Y4 replay/total order = `DispatchSeq`/`IngressSeq` (the consumed order); `AcceptOrdinal` = raw-arrival capture/audit (SourceTime → analysis only); deterministic coalescing** | core §3.5 |
| **Y5 funding fallback key uses symbol + ledger-entry type** | core §5.1 |
| **Y6 acceptance-gate numeric thresholds** | core §9 |
| **Z1 canonical `VenueInstrument` identity (spot ≠ perp; independent ID namespaces)** | core §3.3, core §3.4, core §3.5, core §4, core §5.1, core §5.3 |
| **Z2 orthogonal fill ledger + race-complete state machine + immediate-flatten** | SM §1, SM §2 |
| **Z3 mandatory emergency-actuator failover (venue-native baseline / replicated `EmergencyPlan`)** | core §7.4 |
| **Z4 single reconciliation truth (journal, not the audit projection)** | core §3.2, core §7.5 |
| **— External human review (R1, R2) → v0.6 —** | |
| **R1-F1 / R2-#2 account & margin architecture (boot-verified posture)** | core §4.1, core §2.1, core §7.1, core §7.5, core §9 |
| **R1-F2 / R2-#4 protection lifecycle: disarm + actuator MVP-mandatory on Binance** | Profiles §1 (attach row), SM §5, core §7.4, core §11, core §12 |
| **R1-F3 / R2-#3 forecasting model + total-carry + exit hysteresis** | core §2.2, core §2.4, core §3.3, core §5.2 |
| **R1-F4 cash-and-carry kill-math (break-even, utilization, maker lever)** | core §2.4, tutorial §3.9 |
| **R1-F5 spot-leg surface + per-(venue,API-family) rate limits** | Profiles §3, Profiles §1, core §7.6 |
| **R1-meta capital-gate tiering** | core §1.1, core §7.4, core §9, core §12 |
| **R1-meta solo-honest controls (time-delay, out-of-band, dead-man)** | core §7.4, core §5.2, core §9, core §3.2 |
| **R2-#1 Bybit order-history recovery fallback (realtime cache cleared on restart)** | Profiles §1, SM §1, core §12 |
| **R2-#5 two authorities named (venue = reality; journal = intent/allocation)** | core §3.0 |
| **R2-#6 `DispatchSeq`/`IngressSeq` = replay authority (dense, no "ties"); the earlier "arrival order = replay authority" formulation is superseded by R2c-6/Cx4-1** | core §3.5 |
| **R2-#7 coalescing only on reconstructed books; reconstruction ownership** | core §3.4, core §11 |
| **R2-#8 Intent idempotency (IntentID/Revision/ValidUntil)** | core §3.3 |
| **R2-#9 funding = venue-ledger amount (accounting truth); rate×mark = audit** | tutorial §3.8, core §5.1 |
| **R2-#10 money: compile-time category vs runtime unit check** | core §1, core §3.6 |
| **R2-#11 durability + local-journal-loss coverage** (fullfsync → WAL+`synchronous=FULL`, see R2b-9) | core §3.0, core §11 |
| **R2-#12 acceptance gate = operational; economic gate is separate** | core §9 |
| **R2-#13 milestone reorder — prove the edge before the OMS** | core §12 |
| **R1/R2 tax classification is downstream policy (emit raw events)** | core §10 |
| **R1 journal tamper-evidence (hash-chain the journal, not only audit)** | core §5.1 |
| **R1 `VenueInstrument` in normative signatures; `Account` = subaccount** | core §4, core §5.1 |
| **R1 rescue-margin float counts toward per-venue cap** | core §7.2 |
| **R1/R2 worked numerical trade ledger + crash/partial-fill timeline** | tutorial §3.9 |
| **— Codex convergence pass (v0.6) —** | |
| **Codex-C1 $1K host-death: off-host executor + two-leg plan, not deferred** | core §1.1, core §7.4, core §12 |
| **Codex-C2 ADL rehedge: `reduceOnly` by instrument position, not package delta** | SM §2, core §7.7, core §9 |
| **Codex-M3 disarm fenced state machine (`DisarmPending` + late-fill sweep)** | SM §5, core §7.7, core §9 |
| **Codex-M4 forecast/exit testable contract (quantile, horizon, dwell, basis default)** | core §2.4, core §5.2 |
| **Codex-M5 `C_allocated` = full-bankroll denominator (rescue + reserves)** | core §2.1, core §2.2, core §4.1 |
| **Codex-M6 posture re-verified pre-trade (`AccountPostureSource`); UTA-status fix** | core §4, core §4.1, core §6.3, core §7.5 |
| **Codex-M7 economic gate tiered (conservative $1K bound)** | core §8, core §9, core §12 |
| **Codex-m8 `AcceptOrdinal` = raw-arrival capture/audit stamp (the historical "final tie-breaker" framing is superseded by R2c-6/Cx4-1 — `DispatchSeq` is the dense replay key)** | core §3.5 |
| **— Codex round 2 (convergence) —** | |
| **Cx2-Blocker actuator takeover = fence-first, reduce the *reconciled residual* not allocated qty** | SM §4, core §7.7, core §9 |
| **Cx2 §6.7 reduction delegates to the §5.3 state matrix (reduce-only-by-position)** | SM §2 |
| **Cx2 forecast `HoldPlan` + joint-aggregation rule + one-unit normalization** | core §2.4, core §5.2 |
| **Cx2 $1K economic bound defined (dataset/slippage/threshold) + explicit m2 deliverable** | core §8, core §9, core §12 |
| **Cx2 `AcceptOrdinal` single stamping point; $1K/$10K actuator tier language split** | core §3.5, core §7.4 |
| **— Codex round 3 (verification) —** | |
| **Cx3-Blocker takeover must quiesce ALL resting orders (algo/TP-SL/OCO) to finality first** | SM §4, core §9 |
| **Cx3 `EmergencyPlan` replicates full recovery handles (client/algo IDs, cursors, `TradeID`)** | SM §6 |
| **Cx3 exit = forward continue-vs-close score (entry costs are sunk, not re-charged)** | core §2.4 |
| **Cx3-meta off-host actuator = bounded subproject; sub-$1K canary de-risking path** | core §7.4, core §12 |
| **— External human review round 2 (R1, R2) + 2 Codex convergence rounds → v0.7 —** | |
| **R2b-B1 ambiguous-vs-determined residual by *remaining executable quantity*; quiesce-to-finality before compensating (fixes partial-fill/pending-leg exposure creation)** | core §5.3, SM §2, core §6.4, core §7.4, core §9, tutorial §3.9 |
| **R2b-B2 canary = acceptance of bounded loss, not validation of an unbuilt path; Bybit create = async ack; spot-IOC bracket = integration-test item** | core §12, core §7.4, Profiles §1 |
| **R1b-1 rescue margin under Isolated = active `add-margin` top-up (not passive buffer); `PackageDelever` fallback; isolated-vs-cross trade-off** | core §7.1, core §4.1, SM §7, core §9 |
| **R1b-2 / R2b-3 per-venue `Fence` capability (verified endpoints/authority) + fence-unavailable branch + credential blast radius; signer isolation binds at `$1K` where exchange-side fencing unavailable** | SM §3, SM §4, core §7.3, core §1.1, core §12, core §9 |
| **R1b-3 §1.1 ↔ §7.4 attach-coverage alignment (actuator mandatory for any leg lacking *verified* attach, incl. Bybit spot IOC); §7.7 diagram fix** | core §1.1, core §7.4, core §7.7 |
| **R1b-4 exit score charges closing cost only where branches differ (no premature-exit bias)** | core §2.4 |
| **R1b-5 posture freshness = max-age TTL + pre-trade REST re-verify (position-mode changes emit no venue event)** | core §6.3, core §7.5 |
| **R1b-6a `holdFraction` defined; R1b-6b tutorial pre-ship checklist (posture assertion, tiered signer isolation); R1b-6c market-data capture-lane + losslessness discipline** | core §2.4, tutorial §9, core §3.5 |
| **R2b-4 crash boundary `DispatchAuthorized` (query-only recovery for every pre-ack state)** | SM §1, core §6 |
| **R2b-5 one-way position mode + configured leverage in posture verification (boot/pre-trade/on-change)** | core §4, Profiles §2, core §6.3, core §7.5 |
| **R2b-6 operator control-plane security contract (mTLS/authz/replay/CSRF/network/audit)** | core §7.3, core §12 |
| **R2b-7 economic gate hardened (depth-walk, latency, per-regime + rolling, drawdown, LCB) + versioned calibration artifact (not author-selectable)** | core §8, core §12 |
| **R2b-8 tutorial worked-trade denominator = `C_allocated` (not `C_legs`)** | tutorial §3.9 |
| **R2b-9 SQLite durability = WAL + `synchronous=FULL` on Linux (fullfsync is macOS-only); local fs only** | core §3.0, core §11, Profiles §4 |
| **R2b-10 shared IP-ban safety envelope default (separate product weight/order counters)** | core §7.6, Profiles §3 |
| **R2b-11 raw arrival (`AcceptOrdinal`) = capture/causal order (NOT the replay authority — see R2c-6); `SourceTime` → freshness/analysis only; capture-lane persistence** | core §3.5 |
| **R2b-12 fill-ledger admission unbounded for known orders (finality window bounds only ambiguous-acceptance resolution)** | SM §1 |
| **R2b-13 tutorial ADL re-hedge = ordinary non-reduce-only short** | tutorial §6.2 |
| **— External human review round 3 (R1c, R2c) + Codex convergence → v0.7.1 —** | |
| **R2c-1 liq distance is mark-price-driven (recompute on every mark + margin/position/tier change); stale mark blocks openings + escalates** | core §2.2, core §6, core §7.1, core §9, tutorial §6.2 |
| **R2c-2 `FenceReady` continuous health gate + proactive wind-down (bounded `FenceReadyLost` RTO — differentiated in Cx4-5); incident branch = last-resort residual only** | core §1.1, SM §3, core §4, core §9 |
| **R2c-3 first-class safety ports (`MarginGateway`/`FenceGateway`/`ProtectionGateway`/`EmergencyPlanStore`); `TransferCollateral` leaves `AccountSource`, gains submit-unknown semantics** | core §4 |
| **R2c-4 three-failure-domain topology (engine ∥ operator/control ∥ minimal actuator daemon, no dashboard/HTTP); "single process" = the engine only** | core §3.2, core §11 |
| **R2c-5 tutorial corrected to the safety kernel (top-up→`PackageDelever`; determined/ambiguous ladder; split <5 s / <30 s; `DispatchAuthorized` not IDs-alone)** | tutorial §3.6, §6.2, §6.5, §7.7 |
| **R2c-6 dispatch sequence decoupled: `IngressSeq`/`DispatchSeq` (replay key, consumed order) ≠ `AcceptOrdinal`/`RawIngressSeq` (raw-arrival causal); no cross-source head-of-line blocking; explicit mapping** | core §3.5 |
| **R2c-7 `EmergencyClosePackage` rationale fixed (CloseTrade is not `stop-opening`-subject; justification = foundation authority + delta≈0 no-op)** | core §5.3 |
| **R2c-8 canary loss = written risk-acceptance spec (max notional/collateral; worst-case = uncovered-leg notional; BTC-only; manual-recovery RTO; independent monitoring; signed acceptance)** | core §12, core §7.4 |
| **R2c-9 durability backup/anchor made real (SQLite online-backup / `VACUUM INTO`, not raw copy; RPO/RTO; encryption; external anchor; restore-drill gate)** | core §11 |
| **R2c-10 hurdle = near-risk-free, dated/sourced/haircut-adjusted, revision-controlled (not a hardcoded 4.6%, not "risk-free")** | core §2.1, core §8 |
| **R2c-11 provenance corrected (4 Codex rounds; verification dates)** | HANDOFF, review records, tutorial |
| **R1c-1 tutorial top-up fallback = `PackageDelever` (was `ExposureReduction`) — folded into R2c-5** | tutorial §6.2 |
| **R1c-2 fence credential holds unavoidable transfer-class authority on Bybit (grant narrowest — `Sub Member Transfer`, never `Withdrawal`); signer isolation elevated to primary `Fence`** | core §7.3, SM §4 |
| **R1c-3 sub-account availability is a verify-item (tier-gated on Binance); single-account fallback + signer-isolation fence** | core §7.2, SM §4 |
| **R1c-4 interval-floor compensation in the deadline-exceeded branch (compensate the residual invariant across the ambiguity; only when the interval doesn't straddle zero)** | core §6.7, SM §2, SM §3 |
| **R1c-5 strategic fork if the milestone-2 gate fails (maker entry / regime-selective / alt niche / tuition); mechanically-derived thresholds = solo default** | core §12, core §8 |
| **— Codex convergence (v0.7.1) —** | |
| **Cx4-1 replay/total-order language unified: `DispatchSeq`/`IngressSeq` = the sole replay authority; raw `AcceptOrdinal` = capture/audit only; parity = replay the persisted dispatch log (not regenerate from raw capture)** | core §3.5, ADR §3 |
| **Cx4-2 interval-floor = a reserved `IntervalFloorReduction` generation (counted in `outstanding_reserved_reductions`, no blind retry, own ambiguous result routes through finality) — never issued twice; "no compensation before finality" reworded to "uncertain part"** | SM §2, core §9 |
| **Cx4-3 mark-risk = OMS-owned read port over `engine/state`; adverse-preserving mark coalescing (worst-adverse never dropped); per-venue mark max-age + `Clock`-timer staleness-on-silence** | core §3.4, core §6, core §7.1 |
| **Cx4-4 signer isolation pinned to its own actuator-fenceable failure domain (authenticated protocol + health + revoke; not bot-co-located); "no venue credential" corrected to "no additional key-management/transfer-class credential"** | core §7.3, SM §4, core §11 |
| **Cx4-5 `FenceReadyLost` arbitration mapped to commands (block OpenTrade; supersede delever; `EmergencyClosePackage`/`ExposureReduction` via §6.7; overrides hard-halt); "never outlives its fence" → bounded wind-down + RTO** | SM §3 |
| **Cx4-6 parity seam `ExecutionVenue` gains `MarginGateway`+`ProtectionGateway` (same safety code in backtest/chaos); `EmergencyPlanStore` gains `ListActive` / generation-CAS / `Deactivate` + actuator reconcile-only boot** | core §3.5, core §4, SM §6, core §9 |
| **Cx4-7 deployment non-goals + tutorial qualified: "single process" = the trading engine only; operator + actuator are separate required domains** | core §1, core §11, tutorial §7.3 |
| **— External human review round 4 (R1d) → v0.9 —** | |
| **R1d-1 takeover terminal state = flat: state-dependent end action (fixed residual → `ExposureReduction`; neutral package → two-leg close within plan bounds — post-quiesce a package would be unprotected/unmanaged)** | core §1.1, core §7.4, core §7.7, core §9, SM §4 |
| **R1d-2 interval-floor pre-action capital bound = `max(\|a\|,\|b\|)` (the `\|b\|` phrasing was net-long-only; a net-short interval's worst exposure is `\|a\|`); §9 overshoot assertion generalized to "opposite sign"** | core §6.7, core §9, SM §2 |
| **R1d-3 classification re-runs on every input-changing event: fill/position (realized delta) + order-lifecycle/finality (possible-delta intervals — cancel ack, terminal status, sweep/watermark completion, finality timer, query result); never waits for the next fill/position event** | core §5.3, core §6.7, SM §1, SM §2 |
| **R1d-4 actuator execution authority made coherent: "no venue credential" shorthand fixed (fence *action* needs none; the signer holds the trade key); `ActuatorReady` (actuator's own independent trade credential + reachability + plan-store integrity) added as a monitored dimension + branch-2 precondition + blast-radius entry; durably-fenced reclassified as incident/takeover state** | core §1.1, core §7.3, core §7.4, core §9, SM §3, SM §4 |
| **R1d-5 plan generations: replication = freshness, never coverage — prefix-complete discovery from the acked entry generation (namespace-derived IDs + venue enumeration + cursor sweep); per-generation ack rejected for hot-path handles (would couple reductions to plan-store availability); policy-changing generations require ack before effect** | core §7.4, core §9, SM §6 |
| **R1d-6 margin ops are their own durable target-state machine: journaled `MarginOp` (local op ID, intended delta, before-state + watermark) fsynced before dispatch; resolution by per-venue evidence (Binance `positionMargin/history` symbol+type+amount+window match; Bybit `positionIM` delta vs before-state — its transaction-log documents NO margin type; Bybit transfers by client `transferId` UUID, Binance transfers read-back before retry), never OrderByClientID-like point lookup, never blind delta retry; boot resolves pending margin ops** | core §4, core §7.1, core §7.5, core §9, SM §7, Profiles §4 |
| **R1d-7 Bybit forced-close evidence corrected (no `closeType` on order/execution streams): normative source fields, dedup, and recovery query per verified schemas** | Profiles §1, Profiles §4, core §6.8 |
| **R1d-8 canary sequencing made implementable: actuator/`Fence`/takeover = canary-concurrent workstream; `$1K` promotion (not canary start) gates on the §9 pass; pre-`$1K` de-minimis actuator action conditioned on the actuator existing; "isn't built yet" corrected to "not yet built or not yet §9-proven"** | core §7.4, core §12, SM §3 |
| **R1d-9 stale-mark escalation defined: REST-mark/`MarginStatus` failover on the reserved rate-limit budget; `T_blind` + `mark_blind_action` (`delever`\|`close`) pre-authorized de-risk within a bounded response time — never an alert alone** | core §2.2, core §5.2, core §6.8, core §7.1, core §9, SM §7, tutorial §6.2 |
| **R1d-10 tutorial re-synced: t₅′ teaches `AbsentAfterFinality` (not "stable Rejected/not-found"); §3.7 monitoring leads mark-driven; Part 9 signer-isolation gate matches the core's `$1K` fence floor** | tutorial §3.7, §3.9, Part 9 |
| **R1d-11 governance: SM sections = normative mechanism incorporated by reference (core wins on conflict); tag conventions cover all lettered rounds + informal provenance tags; Profiles citation coverage stated honestly + ellipsis URLs expanded** | core header, SM header, Profiles header/§4 |
| **R1d-F1…F7 rulings on the seven v0.8 FLAGs — all adopted (details: ADR §4 list below)** | SM §1, SM §3, SM §6, SM §7, core §5.3, core §7.4 |
| **— Codex convergence (v0.9) —** | |
| **Cx5-1 takeover completion is evidence-gated, never issuance-gated: `CloseInFlight` awaits §6.7 finality + zero reconciled per-leg allocations + final surfaces (= the §6 tombstone evidence); re-attempts consume a replicated **close-attempt budget** (`max_close_attempts` + `T_close`, plan fields — round-2 refinement: the aggregate bound is a consumable budget, not a per-attempt adjective), exhaustion → `HeldIncident` unconditionally (fence latched, plan active, operator escalated) — never a false "flat"** | core §1.1, core §7.4, core §9, SM §4, SM §6 |
| **Cx5-2 core gains the 1→3 escalation (signing dies mid-wind-down with no latch or `ActuatorReady`) — branch 2 iff latch + `ActuatorReady`, else branch 3; core/SM/§9 now agree** | core §7.4, core §9, SM §3 |
| **Cx5-3 prefix-complete discovery made real: `HistoricalOrders` port (normal + algo history, server↔client ID mapping, end watermark); completeness is retention-bounded (Binance ≈90d orders / ≈6mo trades, no server-side prefix filter) → periodic acknowledged checkpoint generations (ID map + cursors) keep discovery inside retention** | core §4, core §7.4, SM §6, Profiles §4 |
| **Cx5-4 `MarginOpStatus` gains first-class `Ambiguous` (evidence ≠ causal identity); same-position margin ops serialized; on `Ambiguous`: no delta retry, openings blocked, live-state safety continues, explicit float reconciliation** | core §4, core §7.1, core §7.5, core §9, SM §7 |
| **Cx5-5 residual "no venue credential" shorthand purged (HANDOFF + SM tiering quote)** | HANDOFF, SM §4 |
| **— Formal model (b) counterexamples (v0.9.1; `docs/formal/`) —** | |
| **FM-1 takeover completion evidence terminalizes the trade: once `ActuatedFlat` exists (or any flatten reaches §6.7 finality with flat evidence) the trade re-admits no exposure-opening order — plan may only proceed to the tombstone; fresh exposure = successor trade via the lease. Closes the model-checked composition hazard (R1d-F7 re-drive × R1d-F6 clearance × §6 ack-before-entry): clearance in the `ActuatedFlat`→`Deactivate` crash window + same-plan re-entry → the stale re-driven tombstone revokes host-independent coverage under live exposure. Tombstone-as-clearance-precondition rejected (plan-store coupling, the R1d-5 argument)** | core §7.4, SM §3, SM §4, SM §6 |
| **— Codex convergence round on the formal model (Cx6) + its liveness-debug finding (v0.9.2; `docs/formal/`) —** | |
| **Cx6-1 (BLOCKER) the model's branch-3/branch-2 encodings were not write-gated: a signer restart re-enabled automated bot closes + protection cancels inside `ManualOnly`/`TakeoverTransfer` (contradicting "bounded pre-armed protection + operator manual kill only" and single-writer-by-transfer) — bot venue writes now gate on the branch; the atomic `OpManualKill` also erased the real cancel-vs-fill UI race — split into non-atomic flatten + cancel-all with **over-close fill semantics** (a close filling on an already-flat package creates reverse exposure)** | model only (`docs/formal/`; SM §3/§4 already normative on both points) |
| **Cx6-2 `ActuatorReady` constrains the whole takeover, not just the branch-2 hand-off: the watchdog-path latch requires it confirmed (else the attempt times out to `FenceUnavailable` — fencing with no execution capability transfers the writer to nobody); every actuator venue write (quiesce cancels included) requires execution readiness; **mid-takeover `ActuatorReady` loss → `HeldIncident`** (fence latched, plan `Active`, operator escalated) — new SM §4 edge + core §7.4 clause** | core §7.4, SM §4, `docs/formal/` |
| **Cx6-3 operator clearance is health-gated: a fresh propagation test cannot pass while signing / primary-revoke / `ActuatorReady` is failed (those dimensions are what it exercises), and clearing with only the backup still failed returns to `FenceDegraded`, never straight to `FenceReady`** | SM §3, `docs/formal/` |
| **Cx6-4 the model's `I1` single-writer instrumentation now observes every actuator venue write — the four quiesce-cancel actions record unfenced landings exactly like closes (an unfenced cancel is a two-writer event too)** | model only (`docs/formal/`) |
| **Cx6-5 liveness de-masked: a strongly-fair operator manual kill could rescue any stalled branch, so the single liveness property proved only "operator clearance eventually happens" — split into the operator-assisted claim (`IncidentResolves`) and an automated-convergence config (`OPERATOR_INTERVENES = FALSE` + SF fills) checking the §9 branch-1/2 composite-guarantee shape (`Branch12AutoResolves`: automated flat or an honest escalation terminal, no silent stall); both claims conditioned on an eventually-stable environment (`EnvStabilizes` — no fairness scheme can force progress through guards an anti-phase flapping environment keeps permanently unaligned)** | model only (`docs/formal/`) |
| **Cx6-6 (minor) recorded TLC state counts were stale against the reviewed revision — counts + revision identifier refreshed in the correspondence doc and the FM-1 record** | `docs/formal/`, review record |
| **FM-2 false-alarm dismissal: a watchdog fire that latched nothing and started no branch (bot verified alive/healthy, actuator stood down) is operator-DISMISSED as an alert — the R1d-F6 branch-exit preconditions (terminal outcome + clearance + fresh propagation test) are scoped to a *started* branch and do not apply (else acknowledging a false alarm demands flattening a healthy trade). Root cause of the post-Cx6 liveness lasso: with branch clearance as the only exit, a spurious-fire incident had no reachable resolution while the un-fenced healthy bot's obligated naked-reduce resubmissions raced the operator's non-atomic manual kill (every flatten over-closed by a live close's late fill). The model also gains the operator's kill-switch trip (`OpTripHalt` — core §7.4 modes (a)/(c), the §7.3 control plane's first power; halt-first is what ends the operator-vs-bot two-writer race), previously omitted entirely** | core §7.4, SM §4, `docs/formal/` |
| **— Round-2 re-review of the formal model (Cx7) + its spec finding (v0.9.3; `docs/formal/`) —** | |
| **FM-3 engagement is a durable obligation, not a delivered message: an actuator that boots (or stands down) into an incident whose §3 branch is `TakeoverTransfer` with an active plan and NO latched epoch MUST re-attempt the fence latch, keying off the durable branch record rather than a received hand-off signal (lost hand-off ≡ crashed actuator); the re-attempt is `T_fence`-bounded to the `FenceUnavailable` floor as usual. Closes the model-checked stall: branch 2 engages durably (one-way, R1d-F6), the actuator crashes pre-latch, and the Cx6-1 write gate — scoped to the *branch*, not to the failed dimension — permanently bars the healthy bot → unprotected exposure with both writers alive and idle. Stated corollary: a branch that transfers the writer MUST guarantee the transferee engages. Alternatives rejected: watchdog re-arm (moves stalled-takeover detection into the read-only watcher); bot-side re-drive (host dependency inside the takeover's own recovery path)** | core §7.4, SM §3, SM §4, `docs/formal/` |
| **Cx7-1 (MAJOR, model fidelity — SM §4 already correct) the takeover recorded completion without execution capability: `ActQuiesceDone`/`ActReconcileDone`/`ActMarkFlat`/`ActDeactivate` were ungated on `ActuatorReady` and `ActMarkFlat` accepted `held`, so `held → finalize → tombstone` escaped the Cx6-2 park and asserted a plan-store write while plan-store integrity was the failed dimension. Gated + `held` removed, **plus** the park's other spec-stated exit implemented (`ActHeldReentry` — reconcile-only re-entry once ready again), without which sealing the escape would create a liveness dead end. One clarifying (non-normative) sentence added to SM §4: the park is not exited by *recording completion*** | model only + SM §4 clarification (`docs/formal/`) |
| **Cx7-2 (MAJOR, model/property) `Branch12AutoResolves` certified FM-3's stall as "resolved": its pre-latch stand-down disjunct rested on "the healthy bot remains the writer" — a premise Cx6-1 falsified in the same round — and was additionally vacuous for branch 1 (it holds on entry to every wind-down). Disjunct now conditioned on `fenceCtl ∉ {manual, takeover}` **and** the property split into `Branch1AutoResolves` + `Branch2AutoResolves` (conditioning alone leaves branch 1 vacuous)** | model only (`docs/formal/`) |
| **Cx7-3 (MINOR, verification gap) nothing in `I1`…`I7` tied a completion transition to execution capability — why 7/7 green missed Cx7-1. `P1_NoCompletionWithoutCapability` (action property, scoped to the *edge* into `finalize`) added to the nominal safety config; `P2_NoBranch2Stall` added to the automated-convergence config as FM-3's regression check** | model only (`docs/formal/`) |
| *(Cx7-4 considered and **REJECTED**: `OpDismissFalseAlarm` is not health-gated, unlike Cx6-3 clearance — but a dismissal into an unhealthy state is re-raised by the WF-fair `CtlLoss`, and the asymmetry is justified: clearance exits a **started branch** and asserts `FenceReady`; dismissal exits an incident that started nothing and asserts nothing. Recorded so a later round does not re-raise it.)* | — |

## 4. v0.8 doc-split map (what moved where)

The trimmed core keeps its §-numbers; this maps each v0.7.1 block to its v0.8 home (design `docs/superpowers/specs/2026-07-19-doc-split-design.md`, plan `docs/superpowers/plans/2026-07-19-doc-split-execution.md`).

| v0.7.1 location (frozen blob, `6d211cd`) | v0.8 home |
|---|---|
| Convergence narrative (Status ¶ + Codex/review-round summary) | ADR §1 |
| Appendix — finding→resolution traceability table | ADR §3 (this table) |
| §4 quirk matrix (perp/futures venue facts) | Profiles §1 |
| §4.1 account & margin posture tables | Profiles §2 |
| §4.2 spot-leg surface table | Profiles §3 |
| Review-record §2 dated verification citations | copied to Profiles §4 |
| §5.3 order-state list + transition table | SM §1 |
| §5.3 `reduceOnly`-by-instrument-position naked-leg cases | SM §2 |
| §5.3 `PackageDelever` generation mechanics | SM §7 |
| §6.7 (core §6 step 7) ambiguous-order classifier mechanics | SM §2 |
| §7.4 actuator takeover protocol (fence latch → quiesce → reconcile → reduce) | SM §4 |
| §7.4 `Fence`/`FenceReadyLost` differentiated-state branches | SM §3 |
| §7.4 protective-order disarm (`Armed → DisarmPending → Disarmed`) | SM §5 |
| §7.4 `EmergencyPlan` lifecycle (replicate/generations/deactivate) | SM §6 |

The requirement text, its `[→ …]` tag, and a `[mechanism: …]` pointer stay in the core at every row above — only the state list / transition table / dated fact table moved.

**Flagged ambiguities surfaced by the split — all seven resolved in v0.9 by the round-4 (R1d) reviewer rulings** `[→ R1d-F1…F7]`. The v0.8 flag texts are preserved (struck through in spirit) with each ruling:

- **FLAG-1** — SM §1 — *was:* the entry edge into `CancelUnknown` (and any cancel-lost-race exit) never stated. **Resolved (R1d-F1):** ambiguous cancel transport result → `CancelUnknown` (symmetry with `SubmissionUnknown`); explicit terminal cancel + final executions sweep → `Canceled`; cancel-lost-to-fill → `Filled`.
- **FLAG-2** — SM §1 — *was:* the terminal state of a stably-not-found order never named. **Resolved (R1d-F2):** new terminal **`AbsentAfterFinality`**; `Rejected` is not overloaded (explicit venue rejection only); a discovered execution supersedes it.
- **FLAG-3** — SM §7 — *was:* a distinct state between perp-reduce finality and spot submission under-determined. **Resolved (R1d-F3):** durable **`SpotReduceAuthorized`** journal record (crash-boundary discipline at generation scope).
- **FLAG-4** — SM §7 — *was:* exit from `Held/Alert` on later resolution never stated. **Resolved (R1d-F4):** nonterminal — eventual finality resumes the remaining target, or `TerminalFlatten` under the liquidation policy.
- **FLAG-5** — SM §3 — *was:* no stated branch for a backup-fence-readiness-only loss. **Resolved (R1d-F5):** **`FenceDegraded`** (alert/repair, openings continue on the intact primary — not a wind-down); dimension space re-cut in the same round (`ActuatorReady` added, durably-fenced reclassified, R1d-4).
- **FLAG-6** — SM §3 — *was:* exits from the branches never stated. **Resolved (R1d-F6):** one-way per incident — restored health never aborts a started branch; escalate-only re-arbitration; recovery = terminal outcome + operator clearance + fresh propagation test.
- **FLAG-7** — SM §6 — *was:* deactivation for a trade flattened *by the actuator* never stated. **Resolved (R1d-F7):** actuator-side tombstone gated on two-leg close + zero reconciled per-leg allocations + final surfaces, with a durable `ActuatedFlat`/`DeactivationPending` record until the tombstone is durable.
