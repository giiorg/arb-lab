# HANDOFF — Funding-Rate Arbitrage Platform

## Status (2026-07-18): **v0.6** — external reviews (R1, R2) + Codex convergence applied; implementation-ready *by capital tier* (§1.1)

The normative build-spec is **v0.6**. Since v0.5.1 it absorbed **two external human reviews (R1, R2)**, **three venue-fact re-verifications** (account-mode, protection capability, spot surface — official docs, 2026-07-18), and a **Codex adversarial convergence pass** (three rounds, 8 + 5 + 3 findings, all applied). The spec is **deliberately not re-labelled "converged"** — it is honest about being institutional-grade and is now **tiered by capital gate**: the **foundation + read-path + edge-proof (milestones 1–2) are fully build-ready**, heavier controls (signer isolation, hardened off-host actuator, full backtest fidelity, four-eyes) bind at **$10K+**. The convergence rounds surfaced a **meta-finding**: the **off-host emergency actuator is a bounded distributed-systems subproject** — each round found a deeper takeover requirement (fence-first → reduce the reconciled residual → quiesce all resting orders to finality → replicate full recovery handles) — so go-live is **de-risked via a sub-$1K canary** (§12 / §7.4) that proves the actuator protocol in production-lite before real size. Next gate: **implementation per §12** (milestones reordered so the edge is proven before the heavy OMS build).

## Artifacts (read in this order)
1. **`docs/build-spec.md`** — the **normative** MVP build-spec, **v0.6**. New since v0.5.1: **§1.1** capital-gate tiering; **§4.1** boot-verified account/margin architecture matrix; **§4.2** spot-leg surface; **§7.7** sequence diagrams (entry+disarm / ADL / host-death takeover). The Appendix traces every finding (P/C/A/S/B, D, X/Y/Z, **R1-F#/R2-#/R1-meta**, **Codex-C#/M#/Cx2**) to a section.
2. **`docs/tutorial.md`** — teaching companion. New: **§3.9** a complete worked numerical trade ledger + a crash/partial-fill timeline; "spot can't be liquidated" now qualified by the §4.1 posture; softened absolutes.
3. **`docs/review/2026-07-18-06-v0.6-review-and-verification-record.md`** — the R1/R2 findings + resolution + the **venue-verification citations** (the spec carries only endpoint names; the URLs live here).
4. **`docs/review/2026-07-18-05-tutorial-critical-review.md`** — the earlier critical review (26 findings).
5. **`docs/legacy/`** — frozen research record. **Do not edit.**

## Locked decisions (do NOT re-litigate)
- **Stack:** from-scratch **Go** backend; **React/Next on Bun** operator dashboard. OSS = reference only.
- **Strategy-pluggable foundation**; **MVP proves cash-and-carry FIRST**; cross-exchange spread is strategy #2 (alts/dislocation only).
- **Venues:** Binance + Bybit, N-venue-ready behind one adapter seam. (N-venue-*ready* ≠ wiring both in the first slice — the walking skeleton is one venue/one symbol.)
- **Architecture D1–D4** (single-process engine; journal+outbox→embedded store; `platform/fixedpoint`; parity `Clock` + total-order dispatcher). DDD/hexagonal à la `bitnet-backend-go`.
- **Custody = on-exchange for MVP** (isolated per-venue subaccounts, caps ≤~20–30%/venue, USDT/USDC margin). OES = documented scale-up.
- **Risk bar = real capital, live.**
- **v0.6 author decisions (via the user):** (a) **tier the MUSTs by capital gate + reorder milestones** (edge before OMS); (b) **solo-honest controls** (time-delayed self-approval, out-of-band confirm, dead-man alerting — no assumed second person); (c) **per-venue account matrix defaulting to segregated/isolated** (Binance classic + single-asset + isolated + Portfolio-Margin-off; Bybit UTA `ISOLATED_MARGIN` + isLeverage=0 + spot-margin/hedge off).

## What the reviews + Codex hardened (full map in the Appendix + the review record)
- **R1/R2 (external):** account & margin-mode architecture (the flagship's whole safety premise, boot-verified); the **forecasting model** (was undefined — now a `FundingForecast` port + testable contract); **exit hysteresis** (was a boolean); **protection disarm** (was specified only at arming); the **spot-leg surface** (was perp-centric); the cash-and-carry **kill-math**; durability + local-journal-loss; two-authority naming; capital tiering; solo-honest controls; funding=ledger-truth; money compile-vs-runtime; tax classification downstream.
- **Codex round 1 (8):** $1K host-death needs an **off-host executor** (not deferred); ADL rehedge must not use reduce-only into a zeroed perp; disarm needs a fenced `DisarmPending`; forecast/exit need a testable contract; `C_total`→`C_allocated`; posture re-verified pre-trade; economic gate tiered; `AcceptOrdinal` tie-breaker.
- **Codex round 2 (Blocker + 4):** the off-host actuator is a **second writer** → **takeover must fence-first** (revoke the bot credential, reconcile, reduce only the *residual*), not sell the allocated qty; §6.7 had to delegate to the §5.3 reduce-only matrix; forecast needs a durable `HoldPlan` + joint-aggregation rule; the $1K economic bound had to be **defined** (dataset/slippage/threshold); `AcceptOrdinal` single stamping point.
- **Codex round 3 (Blocker + 2 Major):** takeover must **quiesce every resting bot order (algo/TP-SL/OCO) to finality** before reducing (fencing stops *new* submissions, not resting orders — an armed stop fires after the reduction otherwise); the `EmergencyPlan` must **replicate full recovery handles** (client/algo IDs, cursors, `TradeID`) since the local journal is gone; **exit is a forward continue-vs-close score** (entry costs are sunk, not re-charged). Meta: the actuator is a **subproject** → the sub-$1K canary path.

## Residual follow-ups (for the implementing team, TDD — not the spec)
- **Before real capital:** an **empirical testnet liquidation test** of the isolated-perp / segregated-spot posture (§4.1) — the "spot can't be seized" property rests on per-position/segregation semantics, not a single guarantee.
- **Prove in code** (spec §9 chaos, now including): actuator **takeover fence** under network partition / late-fill-after-takeover / bot-recovery-during-takeover (no double-reduce); ADL-from-a-zeroed-perp re-hedge; disarm cancel-vs-trigger race; Bybit realtime-cache-cleared→history fallback; wrong/out-of-band posture refusal.
- **Calibrate:** the `FundingForecast` estimator + the joint-aggregation rule + the `BasisForecast` default, against the §8 persistence model; tune the defined $1K economic bound.

## Open polish (non-blocking)
- Tutorial "what changed" appendix — fold the recurring "the legacy tutorial got this wrong" asides into one appendix so the teaching text stands alone (R1).

## Open decision (for the user — surfaced, not decided)
- **Go-live sequencing.** The off-host actuator being a subproject creates a fork the spec resolves with a *recommended default* (sub-$1K canary on Bybit — best venue-native attach — to build + chaos-test the takeover protocol before $1K), but it's your call: (a) take the recommended sub-$1K canary path; (b) build the full distributed takeover protocol up front before any live trade; (c) constrain the first slice further (e.g. Bybit-only, or a size where a naked-window loss is de-minimis). Milestones 1–2 (foundation + edge-proof) are independent of this choice — it only gates *how you first go live*.

## Next steps
1. **Implement** per spec **§12 milestones** (reordered): foundation skeleton → **read-path + data-capture + minimal research sim that clears the $1K economic gate (BTC, one venue) BEFORE the OMS** → OMS write-path → account/control + risk + reconcile-only-boot + host-independent protection → full backtester + economic gate → $1K live → strategy #2.
2. **When more reviewer feedback arrives (fresh session):** triage per this file — validate each item against the spec + the verified facts (review record §2), apply the valid ones, re-run a Codex convergence round if non-trivial, bump the version, keep the Appendix current.

## Status notes
- All docs are **uncommitted** (branch `docs/funding-arb-spec`). Per the user's rule, **no AI attribution** in commits / authored content.
- Verified facts (APIs/market/tooling) checked 2026-07-18; the verification subagents are complete and terminated.
