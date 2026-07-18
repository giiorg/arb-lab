# HANDOFF — Funding-Rate Arbitrage Platform

## Status (2026-07-19): **v0.7** — second external review round (R1, R2) + 2 Codex convergence rounds applied; implementation-ready *by capital tier* (§1.1)

The normative build-spec is **v0.7**. Since v0.6 it absorbed a **second round of two external human reviews (R1, R2)**, a **re-verification of the new load-bearing venue/tooling facts** (Binance/Bybit isolated-margin top-up + position-mode + key-management endpoints, SQLite durability — official docs, 2026-07-19), and **four Codex adversarial convergence rounds** (17 findings, all valid, all applied). This round applied **both external reviews in full plus the Codex findings** — most of them in the **safety kernel** (ambiguous-order recovery and the off-host actuator). The spec remains **deliberately not "converged"** and **tiered by capital gate**: the **foundation + read-path + edge-proof (milestones 1–2) are fully build-ready**.

**The meta-finding sharpened:** the **safety kernel is *two* state machines, not one** — the off-host **actuator/takeover** protocol *and* the **ambiguous-order-recovery** handler (§6.7). The ambiguous-order machine is **model-tested *within* milestone 3** (before live capital); the takeover is **prototyped + chaos-tested in its later actuator workstream** and proven in a **sub-`$1K` canary** (an explicit acceptance of bounded loss) before it gates real size.

## Artifacts (read in this order)
1. **`docs/build-spec.md`** — the **normative** MVP build-spec, **v0.7**. The Appendix traces every finding (P/C/A/S/B, D, X/Y/Z, R1-F#/R2-#, Codex-C#/M#/Cx2/Cx3, **and the round-2 R1b-#/R2b-#**) to a section.
2. **`docs/tutorial.md`** — teaching companion (worked ledger now on `C_allocated`; ADL re-hedge, crash-boundary, and denominator corrected).
3. **`docs/review/2026-07-19-07-v0.7-review-and-verification-record.md`** — the round-2 R1/R2 findings + resolution + the Codex convergence findings + the venue/tooling verification citations (URLs).
4. **`docs/review/2026-07-18-06-v0.6-review-and-verification-record.md`** — the v0.6 round.
5. **`docs/review/2026-07-18-05-tutorial-critical-review.md`** — the earlier critical review.
6. **`docs/legacy/`** — frozen research record. **Do not edit.**

## Locked decisions (do NOT re-litigate)
- **Stack:** from-scratch **Go** backend; **React/Next on Bun** operator dashboard. OSS = reference only.
- **Strategy-pluggable foundation**; **MVP proves cash-and-carry FIRST**; cross-exchange spread is strategy #2 (alts/dislocation only).
- **Venues:** Binance + Bybit, N-venue-ready behind one adapter seam.
- **Architecture D1–D4** (single-process engine; journal+outbox→embedded store **SQLite WAL + `synchronous=FULL`**; `platform/fixedpoint`; parity `Clock` + total-order dispatcher). DDD/hexagonal à la `bitnet-backend-go`.
- **Custody = on-exchange for MVP** (isolated per-venue subaccounts, caps ≤~20–30%/venue, USDT/USDC margin). OES = documented scale-up.
- **Account posture = segregated / isolated on both venues**, boot- and pre-trade-verified (incl. **one-way position mode + leverage pin**).
- **Risk bar = real capital, live.**

## What v0.7 hardened (full map in the Appendix + the v0.7 review record)
- **Safety kernel — ambiguous-order recovery (R2 Blocker + Codex Blocker):** the one-leg-failure handler classifies a residual **determined vs ambiguous by *remaining executable quantity*** — a compensating trade against a `PartiallyFilled`/`Acknowledged`/`CancelPending`/`SubmissionUnknown` counter-leg can *create* the exposure it removes (naked-short / doubled-short), so it must **quiesce every such order to finality** (terminal state + executions/position reconciliation; for a cancel: cancel-ack **then** an executions sweep) **before** compensating.
- **Safety kernel — off-host `Fence` (R1 + R2 major):** a per-venue **`Fence` capability** with verified mechanisms — Bybit `update-sub-api` (transfer/withdrawal-class authority); Binance has **no REST key-disable for a *regular* account** (only via sub-account + a held master key) — a **fence-unavailable failure branch**, the fence-credential's blast radius, and the coupling that **signer isolation binds at `$1K` (not `$10K`) on any venue lacking exchange-side fencing**.
- **Crash boundary (R2 major):** a durable **`DispatchAuthorized`** state makes recovery **query-only** for every pre-ack state (never resubmit / never infer "not sent").
- **Rescue margin (R1 major):** under Isolated it is an **active `add-margin` top-up** (Binance `positionMargin{type:1}` / Bybit `position/add-margin`) with latency budget + reserved rate-limit, and a **`PackageDelever`** fallback (equal-base both-leg reduction) — *not* `ExposureReduction` — if the top-up fails.
- **Posture (R2 major):** **one-way position mode + configured leverage** added to boot/pre-trade/on-change verification; freshness anchored to a **max-age TTL + REST re-verify** (position-mode changes emit no venue event).
- **Operator control plane (R2 major):** a security contract (mTLS/authz/replay/CSRF/private-network/per-actor-audit).
- **Economic gate (R2 major):** hardened (depth-walk, decision-to-arrival latency, per-regime + rolling, max-drawdown, lower confidence bound) and its numeric thresholds are a **versioned, approved calibration artifact** (not per-developer-selectable).
- **Plus:** raw-arrival (not `SourceTime`) event ordering; SQLite `synchronous=FULL` on Linux; shared IP-ban envelope; `$1K`↔§7.4 attach-coverage alignment; exit-score closing-cost symmetry; canary reframed as bounded-loss acceptance; several tutorial corrections.

## Residual follow-ups (for the implementing team, TDD — not the spec)
- **Model-test before milestone 3:** the **ambiguous-order-recovery state machine** (§6.7 — determined/ambiguous by remaining executable quantity, quiesce-to-finality) and the **takeover state machine** (§7.4).
- **Before real capital:** an **empirical testnet liquidation test** of the isolated-perp / segregated-spot posture (§4.1).
- **Integration-test items (verify, don't assume):** whether a Bybit/Binance **spot IOC can carry an attached bracket** (doc-silent — assume unprotected until proven); the per-venue **`Fence` propagation test**.
- **Prove in code** (spec §9 chaos): ambiguous one-leg failure across *every* reachable venue state (partial-fill before/after cancel-ack; stable-not-found→discovered fill); actuator takeover fence + **fence-unavailable** branch; **margin top-up under stress → `PackageDelever`**; ADL-from-a-zeroed-perp; disarm race; Bybit history fallback; wrong/out-of-band posture (incl. flipped position mode).
- **Calibrate:** the `FundingForecast` estimator + joint-aggregation + `BasisForecast`; and produce the **versioned economic-gate calibration artifact** (the milestone-2 gate input).

## Open recommendation (surfaced, not decided) — **doc split** (R2)
Both reviewers note the doc now mixes normative spec + review history + incident catalogue. R2 proposes splitting into: (a) a short **normative core**, (b) **venue capability profiles** with dated evidence, (c) **execution + takeover state-machine specs**, (d) an **ADR/review-history** doc, (e) the **tutorial**. Recommended as a *separate* pass (not folded into a content round, to avoid churn). Not yet done.

## Open decision (for the user — largely converged)
- **Go-live sequencing.** Both round-2 reviewers converge on **option (a): the sub-`$1K` canary** — now reframed as an **explicit acceptance of a bounded, de-minimis naked-window loss** (Bybit's venue-native attach covers only the *perp*; the spot IOC leg shares the uncovered window), during which the `Fence` + takeover protocol is **built and chaos-tested**; promotion to `$1K` is **gated on that protocol passing §9**. This is the spec's recommended default (§12); still your call to confirm.

## Next steps
1. **Implement** per spec **§12 milestones** (reordered): foundation skeleton → read-path + data-capture + minimal research sim clearing the `$1K` economic gate (BTC, one venue) BEFORE the OMS → OMS write-path (+ **model-test the ambiguous-order state machine**) → account/control + risk + reconcile-only-boot + host-independent protection + `Fence` → full backtester + economic gate → sub-`$1K` canary → `$1K` → strategy #2.
2. **When more reviewer feedback arrives (fresh session):** triage per this file — validate each item against the spec + the verified facts (v0.7 review record §2), apply the valid ones, **silently discard false positives, correct overstated framings**, re-run a Codex convergence round if non-trivial, **disambiguate new tags** (round N → a fresh prefix so the Appendix stays collision-free), bump the version, keep the Appendix current.

## Status notes
- v0.7 docs are **committed** on branch `docs/funding-arb-spec` (**local-only — not pushed**; say the word to push). Per the user's rule, **no AI attribution** in commits / authored content.
- Verified facts (APIs/market/tooling) checked 2026-07-18 (v0.6) and 2026-07-19 (v0.7); verification subagents complete and terminated.
