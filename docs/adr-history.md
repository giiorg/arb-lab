# ADR & Review History — Funding-Rate Arbitrage Platform

**Status:** v0.8 (2026-07-19) — structural release. Content frozen from build-spec v0.7.1; reorganization only, no normative change.
**Doc set:** normative core `build-spec.md` · venue facts `venue-profiles.md` · kernel machines `state-machines.md` · history/traceability `adr-history.md` · teaching companion `tutorial.md`.
**Role:** the convergence record — how the spec got here (review rounds, Codex convergence), plus the finding→resolution traceability that used to be the build-spec Appendix. Nothing here is normative on its own; the requirements live in the core.

## 1. Convergence narrative (v0.1 → v0.7.1)

The v0.7.1 status paragraph, frozen verbatim:

Draft v0.7.1 (2026-07-19) — Codex adversarial rounds 1–4 (11 → 6 → 4 → 4), then **two external human reviews (R1, R2) + a Codex convergence pass → v0.6**, then a **second external round (R1, R2) + 4 Codex convergence rounds → v0.7** (both reviews applied in full — near-zero false positives — plus 17 Codex convergence findings, all valid and applied; the **safety kernel** took most of them; new venue/tooling facts re-verified against official docs 2026-07-19). v0.7 hardens the **safety kernel**: the one-leg-failure handler now classifies residuals **determined vs ambiguous by *remaining executable quantity*** and quiesces-to-finality before compensating (a compensating trade against a `PartiallyFilled`/pending counter-leg could otherwise *create* the exposure it removes); a durable **`DispatchAuthorized`** crash boundary makes recovery query-only; rescue margin under Isolated is an **active `add-margin` top-up** with a **`PackageDelever`** fallback; a per-venue **`Fence`** capability (with signer isolation pulled to `$1K` where exchange-side fencing is unavailable); **one-way position-mode + leverage** posture verification; an **operator control-plane security** contract; a hardened, **calibration-artifact-gated** economic bound; **raw-arrival** (not `SourceTime`) event ordering; and SQLite **WAL + `synchronous=FULL`** durability. **Deliberately not "converged":** implementation-ready *by capital tier* (§1.1) — **foundation + read-path + edge-proof (milestones 1–2) are fully build-ready**; the **off-host actuator *and* the ambiguous-order-recovery state machine are the safety kernel** (§6.7, §7.4): the ambiguous-order machine is **model-tested *within* milestone 3 before live capital**, and the takeover is **prototyped + chaos-tested in its later actuator workstream** and proven in a **sub-`$1K` canary** — an explicit acceptance of bounded loss (§12) — before it gates real size. Before real capital: an empirical testnet liquidation test of the §4.1 posture. **v0.7.1** applies a **third external round (R1c, R2c) + a 5-round Codex convergence pass (→ converged)** — refinements/corrections on the v0.7 architecture, not a new redefinition: liq-distance is **mark-price-driven** (recomputed on every fresh mark, not only account events) with a **stale-mark block/escalate** `[→ R2c-1]`; a continuously-monitored, **differentiated `FenceReady`** health gate whose loss drives a **bounded `FenceReadyLost` wind-down (or actuator takeover if the signer itself failed)** within an RTO `[→ R2c-2, Cx4-5]`; first-class safety-capability **ports** (`MarginGateway`/`FenceGateway`/`ProtectionGateway`/`EmergencyPlanStore`; `TransferCollateral` gains submit-unknown semantics) `[→ R2c-3]`; an explicit **three-failure-domain topology** (trading engine ∥ operator/control ∥ a minimal actuator daemon with no dashboard/HTTP surface) `[→ R2c-4]`; the **dispatch sequence decoupled** from raw-arrival order (no cross-source head-of-line blocking; held/coalesced inputs need no 1:1 mapping) `[→ R2c-6]`; **signer isolation elevated to the primary `Fence`** (exchange-side disable the backup) with the Bybit fence credential's unavoidable **transfer-class authority** stated honestly `[→ R1c-2]`; **interval-floor** compensation in the deadline-exceeded ambiguous branch `[→ R1c-4]`; a **quantified canary loss bound** `[→ R2c-8]`; real **backup/anchor/retention/restore-drill** durability requirements `[→ R2c-9]`; a **near-risk-free, dated/haircut/revision-controlled hurdle** `[→ R2c-10]`; and tutorial + provenance corrections `[→ R2c-5, R2c-11]`. R2's two canary blockers (mark-driven liq, quantified loss) and two `$1K` blockers (incident-available fence, explicit safety-plane topology) are **closed at the design layer here**; their residual gating is now implementation + chaos-test, as the spec brackets it. Open: a normative/ADR/tutorial **doc split** (both R2 rounds — now escalated from optional to recommended) and a "what changed" tutorial appendix.

## 2. Review records

- `docs/review/2026-07-19-08-v0.7.1-review-and-verification-record.md` — round-3 (R1c/R2c) findings + resolutions + Codex convergence pass.
- `docs/review/2026-07-19-07-v0.7-review-and-verification-record.md` — round-2 (R1/R2 second round) findings + resolutions + venue/tooling verification citations (§2 there).
- `docs/review/2026-07-18-06-v0.6-review-and-verification-record.md` — v0.6 round.
- `docs/review/2026-07-18-05-tutorial-critical-review.md` — the earlier critical review the spec resolves.
- `docs/legacy/` — frozen research record (the "why"). Do not edit.

## 3. Finding → resolution traceability

| Finding / Decision | Resolved in |
|---|---|
| P1 spread-vs-level | §1, §2.1–2.3 |
| P2 hurdle rate | §2.1 |
| P3 ADL (realized hedge-failure) | §2.1, §7.1 (naked-leg-gap sizing) |
| P4 build-vs-buy | §11 |
| C1 cost single-source | §2.4 |
| C2 discrete settlement | §2.4, §6 |
| C3 drift-based min-edge | §2.4 |
| C4 tiered MMR | §4, §7.1 |
| C5 buffer policy | §5.2 |
| C6 inverse perps | §1, §4 |
| C7 basis in funding audit | §6 |
| C8 Bybit dynamic settlement | §2.4, §4, §6 |
| A1 venue adapter | §3.2, §4 (segregated capabilities) |
| A2 multi-region / split-ready | §3.7, §11 |
| A3 durable state / reconcile-only boot | §7.5, §6.10 |
| A4 exactly-once funding | §5.1 (UNIQUE + ledger txn ID) |
| A5 kill-switch modes | §7.4 |
| S1 custody/OES | §7.2 |
| S2 key security | §7.3 |
| S3 audit durability | §3.2, §7.5 (append-only projection) |
| S4 escalation ladder | §7.4 |
| B1 backtest fidelity | §3.5, §8 |
| B2 test taxonomy | §9 |
| D1 topology (in-process engine) | §3.0, §3.1, §3.7 |
| D2 durability (journal + outbox → embedded store) | §3.0, §5, §6, §7.5 |
| D3 money (`platform/fixedpoint`) | §3.0, §3.6, §4, §5.1, §11 |
| D4 clock (parity dispatcher + server-time) | §3.0, §3.5, §3.6 |
| **X1 durable order/account truth (single-writer OMS)** | §3.0, §3.2, §4 (`OrderByClientID`/`Fills`), §5.1, §6 |
| **X2 ADL/margin controls + account contract** | §2.1, §4 (`AccountSource`), §6.8, §7.1 |
| **X3 Trade ownership + exclusive lease** | §1, §3.2, §5.1 |
| **X4 bus back-pressure / delivery lanes / freshness** | §3.4 |
| **X5 DDD boundary (strategy input, venue ACL)** | §3.2, §3.3, §3.6 |
| **X6 pre-authorized (not human-gated) emergency** | §7.4 |
| **X7 fixedpoint intermediates + dimensional types** | §3.6, §4, §5.1 |
| **X8 clock total-order parity + server-time** | §3.5, §3.6, §6.3 |
| **X9 DedupeKey → UNIQUE + ledger txn ID** | §5.1 |
| **X10 key blast-radius = whole subaccount** | §7.3 |
| **X11 paper acceptance-gate pass/fail** | §9 |
| **Y1 fill dedup identity (`ExecutionID` + `UNIQUE`)** | §4, §5.1, §5.3 |
| **Y2 `Place` non-idempotent / `SubmissionUnknown`** | §4, §5.3, §6 |
| **Y3 `ExposureReduction` command (executable ADL path) + actuator failure domain** | §5.3, §6, §7.4 |
| **Y4 replay/total order = `DispatchSeq`/`IngressSeq` (the consumed order); `AcceptOrdinal` = raw-arrival capture/audit (SourceTime → analysis only); deterministic coalescing** | §3.5 |
| **Y5 funding fallback key uses symbol + ledger-entry type** | §5.1 |
| **Y6 acceptance-gate numeric thresholds** | §9 |
| **Z1 canonical `VenueInstrument` identity (spot ≠ perp; independent ID namespaces)** | §3.3, §3.4, §3.5, §4, §5.1, §5.3 |
| **Z2 orthogonal fill ledger + race-complete state machine + immediate-flatten** | §5.3, §6.7 |
| **Z3 mandatory emergency-actuator failover (venue-native baseline / replicated `EmergencyPlan`)** | §7.4 |
| **Z4 single reconciliation truth (journal, not the audit projection)** | §3.2, §7.5 |
| **— External human review (R1, R2) → v0.6 —** | |
| **R1-F1 / R2-#2 account & margin architecture (boot-verified posture)** | §4.1, §2.1, §7.1, §7.5, §9 |
| **R1-F2 / R2-#4 protection lifecycle: disarm + actuator MVP-mandatory on Binance** | §4 (attach row), §7.4, §11, §12 |
| **R1-F3 / R2-#3 forecasting model + total-carry + exit hysteresis** | §2.2, §2.4, §3.3, §5.2 |
| **R1-F4 cash-and-carry kill-math (break-even, utilization, maker lever)** | §2.4, tutorial §3.9 |
| **R1-F5 spot-leg surface + per-(venue,API-family) rate limits** | §4.2, §4, §7.6 |
| **R1-meta capital-gate tiering** | §1.1, §7.4, §9, §12 |
| **R1-meta solo-honest controls (time-delay, out-of-band, dead-man)** | §7.4, §5.2, §9, §3.2 |
| **R2-#1 Bybit order-history recovery fallback (realtime cache cleared on restart)** | §4, §5.3, §12 |
| **R2-#5 two authorities named (venue = reality; journal = intent/allocation)** | §3.0 |
| **R2-#6 `DispatchSeq`/`IngressSeq` = replay authority (dense, no "ties"); the earlier "arrival order = replay authority" formulation is superseded by R2c-6/Cx4-1** | §3.5 |
| **R2-#7 coalescing only on reconstructed books; reconstruction ownership** | §3.4, §11 |
| **R2-#8 Intent idempotency (IntentID/Revision/ValidUntil)** | §3.3 |
| **R2-#9 funding = venue-ledger amount (accounting truth); rate×mark = audit** | tutorial §3.8, §5.1 |
| **R2-#10 money: compile-time category vs runtime unit check** | §1, §3.6 |
| **R2-#11 durability + local-journal-loss coverage** (fullfsync → WAL+`synchronous=FULL`, see R2b-9) | §3.0, §11 |
| **R2-#12 acceptance gate = operational; economic gate is separate** | §9 |
| **R2-#13 milestone reorder — prove the edge before the OMS** | §12 |
| **R1/R2 tax classification is downstream policy (emit raw events)** | §10 |
| **R1 journal tamper-evidence (hash-chain the journal, not only audit)** | §5.1 |
| **R1 `VenueInstrument` in normative signatures; `Account` = subaccount** | §4, §5.1 |
| **R1 rescue-margin float counts toward per-venue cap** | §7.2 |
| **R1/R2 worked numerical trade ledger + crash/partial-fill timeline** | tutorial §3.9 |
| **— Codex convergence pass (v0.6) —** | |
| **Codex-C1 $1K host-death: off-host executor + two-leg plan, not deferred** | §1.1, §7.4, §12 |
| **Codex-C2 ADL rehedge: `reduceOnly` by instrument position, not package delta** | §5.3, §7.7, §9 |
| **Codex-M3 disarm fenced state machine (`DisarmPending` + late-fill sweep)** | §7.4, §7.7, §9 |
| **Codex-M4 forecast/exit testable contract (quantile, horizon, dwell, basis default)** | §2.4, §5.2 |
| **Codex-M5 `C_allocated` = full-bankroll denominator (rescue + reserves)** | §2.1, §2.2, §4.1 |
| **Codex-M6 posture re-verified pre-trade (`AccountPostureSource`); UTA-status fix** | §4, §4.1, §6.3, §7.5 |
| **Codex-M7 economic gate tiered (conservative $1K bound)** | §8, §9, §12 |
| **Codex-m8 `AcceptOrdinal` = raw-arrival capture/audit stamp (the historical "final tie-breaker" framing is superseded by R2c-6/Cx4-1 — `DispatchSeq` is the dense replay key)** | §3.5 |
| **— Codex round 2 (convergence) —** | |
| **Cx2-Blocker actuator takeover = fence-first, reduce the *reconciled residual* not allocated qty** | §7.4, §7.7, §9 |
| **Cx2 §6.7 reduction delegates to the §5.3 state matrix (reduce-only-by-position)** | §6.7 |
| **Cx2 forecast `HoldPlan` + joint-aggregation rule + one-unit normalization** | §2.4, §5.2 |
| **Cx2 $1K economic bound defined (dataset/slippage/threshold) + explicit m2 deliverable** | §8, §9, §12 |
| **Cx2 `AcceptOrdinal` single stamping point; $1K/$10K actuator tier language split** | §3.5, §7.4 |
| **— Codex round 3 (verification) —** | |
| **Cx3-Blocker takeover must quiesce ALL resting orders (algo/TP-SL/OCO) to finality first** | §7.4, §9 |
| **Cx3 `EmergencyPlan` replicates full recovery handles (client/algo IDs, cursors, `TradeID`)** | §7.4 |
| **Cx3 exit = forward continue-vs-close score (entry costs are sunk, not re-charged)** | §2.4 |
| **Cx3-meta off-host actuator = bounded subproject; sub-$1K canary de-risking path** | §7.4, §12 |
| **— External human review round 2 (R1, R2) + 2 Codex convergence rounds → v0.7 —** | |
| **R2b-B1 ambiguous-vs-determined residual by *remaining executable quantity*; quiesce-to-finality before compensating (fixes partial-fill/pending-leg exposure creation)** | §5.3, §6.4, §6.7, §7.4, §9, tutorial §3.9 |
| **R2b-B2 canary = acceptance of bounded loss, not validation of an unbuilt path; Bybit create = async ack; spot-IOC bracket = integration-test item** | §12, §7.4, §4 |
| **R1b-1 rescue margin under Isolated = active `add-margin` top-up (not passive buffer); `PackageDelever` fallback; isolated-vs-cross trade-off** | §7.1, §4.1, §5.3, §9 |
| **R1b-2 / R2b-3 per-venue `Fence` capability (verified endpoints/authority) + fence-unavailable branch + credential blast radius; signer isolation binds at `$1K` where exchange-side fencing unavailable** | §7.4, §7.3, §1.1, §12, §9 |
| **R1b-3 §1.1 ↔ §7.4 attach-coverage alignment (actuator mandatory for any leg lacking *verified* attach, incl. Bybit spot IOC); §7.7 diagram fix** | §1.1, §7.4, §7.7 |
| **R1b-4 exit score charges closing cost only where branches differ (no premature-exit bias)** | §2.4 |
| **R1b-5 posture freshness = max-age TTL + pre-trade REST re-verify (position-mode changes emit no venue event)** | §6.3, §7.5 |
| **R1b-6a `holdFraction` defined; R1b-6b tutorial pre-ship checklist (posture assertion, tiered signer isolation); R1b-6c market-data capture-lane + losslessness discipline** | §2.4, tutorial §9, §3.5 |
| **R2b-4 crash boundary `DispatchAuthorized` (query-only recovery for every pre-ack state)** | §5.3, §6 |
| **R2b-5 one-way position mode + configured leverage in posture verification (boot/pre-trade/on-change)** | §4, §4.1, §6.3, §7.5 |
| **R2b-6 operator control-plane security contract (mTLS/authz/replay/CSRF/network/audit)** | §7.3, §12 |
| **R2b-7 economic gate hardened (depth-walk, latency, per-regime + rolling, drawdown, LCB) + versioned calibration artifact (not author-selectable)** | §8, §12 |
| **R2b-8 tutorial worked-trade denominator = `C_allocated` (not `C_legs`)** | tutorial §3.9 |
| **R2b-9 SQLite durability = WAL + `synchronous=FULL` on Linux (fullfsync is macOS-only); local fs only** | §3.0, §11 |
| **R2b-10 shared IP-ban safety envelope default (separate product weight/order counters)** | §7.6, §4.2 |
| **R2b-11 raw arrival (`AcceptOrdinal`) = capture/causal order (NOT the replay authority — see R2c-6); `SourceTime` → freshness/analysis only; capture-lane persistence** | §3.5 |
| **R2b-12 fill-ledger admission unbounded for known orders (finality window bounds only ambiguous-acceptance resolution)** | §5.3 |
| **R2b-13 tutorial ADL re-hedge = ordinary non-reduce-only short** | tutorial §6.2 |
| **— External human review round 3 (R1c, R2c) + Codex convergence → v0.7.1 —** | |
| **R2c-1 liq distance is mark-price-driven (recompute on every mark + margin/position/tier change); stale mark blocks openings + escalates** | §2.2, §6, §7.1, §9, tutorial §6.2 |
| **R2c-2 `FenceReady` continuous health gate + proactive wind-down (bounded `FenceReadyLost` RTO — differentiated in Cx4-5); incident branch = last-resort residual only** | §1.1, §7.4, §4, §9 |
| **R2c-3 first-class safety ports (`MarginGateway`/`FenceGateway`/`ProtectionGateway`/`EmergencyPlanStore`); `TransferCollateral` leaves `AccountSource`, gains submit-unknown semantics** | §4 |
| **R2c-4 three-failure-domain topology (engine ∥ operator/control ∥ minimal actuator daemon, no dashboard/HTTP); "single process" = the engine only** | §3.2, §11 |
| **R2c-5 tutorial corrected to the safety kernel (top-up→`PackageDelever`; determined/ambiguous ladder; split <5 s / <30 s; `DispatchAuthorized` not IDs-alone)** | tutorial §3.6, §6.2, §6.5, §7.7 |
| **R2c-6 dispatch sequence decoupled: `IngressSeq`/`DispatchSeq` (replay key, consumed order) ≠ `AcceptOrdinal`/`RawIngressSeq` (raw-arrival causal); no cross-source head-of-line blocking; explicit mapping** | §3.5 |
| **R2c-7 `EmergencyClosePackage` rationale fixed (CloseTrade is not `stop-opening`-subject; justification = foundation authority + delta≈0 no-op)** | §5.3 |
| **R2c-8 canary loss = written risk-acceptance spec (max notional/collateral; worst-case = uncovered-leg notional; BTC-only; manual-recovery RTO; independent monitoring; signed acceptance)** | §12, §7.4 |
| **R2c-9 durability backup/anchor made real (SQLite online-backup / `VACUUM INTO`, not raw copy; RPO/RTO; encryption; external anchor; restore-drill gate)** | §11 |
| **R2c-10 hurdle = near-risk-free, dated/sourced/haircut-adjusted, revision-controlled (not a hardcoded 4.6%, not "risk-free")** | §2.1, §8 |
| **R2c-11 provenance corrected (4 Codex rounds; verification dates)** | HANDOFF, review records, tutorial |
| **R1c-1 tutorial top-up fallback = `PackageDelever` (was `ExposureReduction`) — folded into R2c-5** | tutorial §6.2 |
| **R1c-2 fence credential holds unavoidable transfer-class authority on Bybit (grant narrowest — `Sub Member Transfer`, never `Withdrawal`); signer isolation elevated to primary `Fence`** | §7.3, §7.4 |
| **R1c-3 sub-account availability is a verify-item (tier-gated on Binance); single-account fallback + signer-isolation fence** | §7.2, §7.4 |
| **R1c-4 interval-floor compensation in the deadline-exceeded branch (compensate the residual invariant across the ambiguity; only when the interval doesn't straddle zero)** | §6.7, §7.4 |
| **R1c-5 strategic fork if the milestone-2 gate fails (maker entry / regime-selective / alt niche / tuition); mechanically-derived thresholds = solo default** | §12, §8 |
| **— Codex convergence (v0.7.1) —** | |
| **Cx4-1 replay/total-order language unified: `DispatchSeq`/`IngressSeq` = the sole replay authority; raw `AcceptOrdinal` = capture/audit only; parity = replay the persisted dispatch log (not regenerate from raw capture)** | §3.5, Appendix |
| **Cx4-2 interval-floor = a reserved `IntervalFloorReduction` generation (counted in `outstanding_reserved_reductions`, no blind retry, own ambiguous result routes through finality) — never issued twice; "no compensation before finality" reworded to "uncertain part"** | §6.7, §9 |
| **Cx4-3 mark-risk = OMS-owned read port over `engine/state`; adverse-preserving mark coalescing (worst-adverse never dropped); per-venue mark max-age + `Clock`-timer staleness-on-silence** | §3.4, §6, §7.1 |
| **Cx4-4 signer isolation pinned to its own actuator-fenceable failure domain (authenticated protocol + health + revoke; not bot-co-located); "no venue credential" corrected to "no additional key-management/transfer-class credential"** | §7.3, §7.4, §11 |
| **Cx4-5 `FenceReadyLost` arbitration mapped to commands (block OpenTrade; supersede delever; `EmergencyClosePackage`/`ExposureReduction` via §6.7; overrides hard-halt); "never outlives its fence" → bounded wind-down + RTO** | §7.4 |
| **Cx4-6 parity seam `ExecutionVenue` gains `MarginGateway`+`ProtectionGateway` (same safety code in backtest/chaos); `EmergencyPlanStore` gains `ListActive` / generation-CAS / `Deactivate` + actuator reconcile-only boot** | §3.5, §4, §7.4, §9 |
| **Cx4-7 deployment non-goals + tutorial qualified: "single process" = the trading engine only; operator + actuator are separate required domains** | §1, §11, tutorial §7.3 |
