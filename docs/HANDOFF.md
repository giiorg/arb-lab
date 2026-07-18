# HANDOFF — Funding-Rate Arbitrage Platform

## Status (2026-07-18): spec + tutorial **converged**; ready for external human review

Phase-1 deliverables are complete. The normative build-spec and the teaching tutorial are drafted and hardened through **four Codex adversarial rounds on the spec + one on the tutorial**. The architecture is validated and internally coherent; each round narrowed (11 → 6 → 4 → 4 findings) from structural to integration/precision. **Next gate: the user's external reviewers → then implementation** per spec §12.

## Artifacts (read in this order)
1. **`docs/build-spec.md`** — the **normative** MVP build-spec, **v0.5.1**. The "what to build," and the source of truth. Its Appendix traces every resolved finding (P/C/A/S/B = original review; D = architecture decisions; X/Y/Z = Codex rounds 1/2/3+4) to a section.
2. **`docs/tutorial.md`** — teaching companion (the "why"). Cash-and-carry-first; defers all normative detail to the spec via `§spec` references.
3. **`docs/review/2026-07-18-05-tutorial-critical-review.md`** — the critical review (26 findings; verified facts in §7).
4. **`docs/legacy/`** — frozen research record (01–05 + sources). **Do not edit** — it's the historical "why."

## Locked decisions (do NOT re-litigate)
- **Stack:** from-scratch **Go** backend; **React/Next on Bun** operator dashboard. OSS (NautilusTrader, Hummingbot, CCXT, barter-rs) = reference only, never dependencies.
- **Strategy-pluggable foundation** running swappable strategies. **MVP proves cash-and-carry FIRST** (long spot + short perp, one venue — harvests the funding *level*, the real edge). Cross-exchange *spread* is strategy #2, scoped to alts/dislocation only (structurally too thin on majors — finding P1).
- **Venues:** Binance + Bybit, N-venue-ready behind one adapter seam.
- **Architecture** (resolved & validated): **D1** single-process in-process engine (in-proc dispatch on the hot path; NATS deferred, seam documented); **D2** local write-ahead journal + transactional outbox → embedded store; **D3** `platform/fixedpoint` dimensional scaled-int money (no float64); **D4** Nautilus-style parity `Clock` + single total-order dispatcher. DDD/hexagonal à la `bitnet-backend-go` (`internal/{boundary,engine,platform}`, ports in `application/ports`, `driven/`+`driving/`, compile-time port assertions, `module.go` wiring, no DI container).
- **Custody = on-exchange for MVP** (small size, isolated per-venue subaccounts, caps ≤~20–30%/venue, USDT/USDC margin). OES = documented scale-up path.
- **Risk bar = real capital, live** — durable order/account truth, custody/key security, reconciliation, and money-losing edges are ship-blockers.

## What the Codex rounds hardened (full map in spec Appendix)
- **Round 1 (11):** single-writer **`oms`** context + durable write-ahead journal; ADL/account venue contract; delivery lanes; strategy/venue DDD boundary; pre-authorized (not human-gated) emergency; fixedpoint intermediates + dimensional types; clock total-order; dedupe UNIQUE; key blast-radius; acceptance gate.
- **Round 2 (6):** fill dedup identity; `Place` non-idempotent + `SubmissionUnknown`; `ExposureReduction` (spot has no `reduceOnly`); total-order `IngressSeq`; funding key uses symbol; acceptance-gate numbers.
- **Round 3 (4):** canonical **`VenueInstrument`** (spot ≠ perp, independent ID namespaces); orthogonal fill ledger + race-complete state machine; **mandatory** emergency-actuator failover; journal (not audit projection) is reconciliation truth.
- **Round 4 coherence gate (4):** fixed a UNIQUE-key contradiction, made the escalation ladder concurrent, closed the first-fill→protection host-death window, and fenced the reduction generation against duplicate-stacking.
- **Tutorial review (4):** product-specific fee endpoints; cash-and-carry funding is perp-leg-only; acceptance gate is per funding-bearing perp leg; taught host-independent flatten coverage.

## Residual follow-ups for the implementing team (from Codex — TDD, not the spec)
These are specified as invariants + acceptance tests (spec §9 chaos), to be *proven in code* against real venue behaviors:
- Fill-ledger + protective-order races (identical spot/perp exec IDs; duplicate position events during an outstanding reduction; protective-order ↔ OMS inventory handoff).
- Venue-specific **stable-not-found / finality** windows for `SubmissionUnknown` resolution.
- The host-death **first-fill→protection-ack** window is *bounded and tested*, not eliminated — validate the per-venue attached-protection (or pre-armed `EmergencyPlan`) mechanism.

## Next steps
1. **External human review** of `docs/build-spec.md` (with `docs/tutorial.md` as companion).
   - **When reviewer feedback arrives (in a fresh session):** triage each item like the Codex findings — validate it against the spec + the verified facts (review §7), apply the valid ones (silently discard false positives), re-run a Codex convergence pass (`codex:codex-copilot` → `artifact-review`) if the changes are non-trivial, then bump the spec version and update this file. Keep the Appendix traceability current.
2. **Implement** per spec **§12 milestones**: foundation skeleton (`platform/*` + `engine/venue` + `engine/state` + CI gates) → cash-and-carry read-path → OMS write-path → account/control + risk + reconcile-only-boot → backtester + paper gate → $1K live → strategy #2.

## Status notes
- All docs are **uncommitted**. Per the user's rule, **no AI attribution** in commit messages / authored content.
- Verified facts (APIs/market/tooling) were checked 2026-07-18; the 4 verification subagents from the prior session are complete and terminated.
