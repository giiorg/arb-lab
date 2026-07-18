# Critical Review — `05-tutorial-building-a-bot.md` (and supporting docs)

**Reviewer:** Claude (Opus 4.8), max-effort pass
**Date:** 2026-07-18
**Artifact under review:** `docs/legacy/05-tutorial-building-a-bot.md` (866 lines), cross-checked against `README.md`, `01-market-landscape.md`, `02-math-and-mechanics.md`, `03-tech-stack.md`, `04-comparative-and-operational.md`, `sources.md`.

## Review frame (agreed with user)

- **Deliverable:** two artifacts — (a) keep/polish the tutorial *and* (b) extract a terser engineering **build-spec**. This review feeds both.
- **Scope bar:** MVP stays **Binance + Bybit** but must be **N-venue-ready**; anything hard-coding two venues is flagged.
- **Risk bar:** **real capital, live** — correctness, custody/key security, failure handling, reconciliation, and money-losing edge cases are ship-blockers.
- **Verification:** load-bearing, time-sensitive claims verified via web research (4 agents). Verification status per cluster in §7.

## Severity legend

- **BLOCKER** — would lose money, corrupt state, or mislead the MVP's core strategy decision. Fix before building.
- **MAJOR** — materially weakens correctness, safety, or the spec's usefulness. Fix before shipping.
- **MINOR** — real but bounded; fix opportunistically.
- **NIT** — polish / consistency.
- **STRENGTH** — called out so the critique stays calibrated and these aren't lost in a rewrite.

---

## 1. Executive summary

This is a genuinely strong document at the **execution layer**. It is unusually self-aware: it correctly separates the ±0.05% premium/interest *damper* from per-symbol *caps* (a distinction most write-ups botch), it handles Bybit snapshot/delta reconstruction, it has a real one-leg-failure/reconcile-to-position-truth protocol with reduce-only flattening, it resists the leverage-yield seduction, and it insists on a live-estimate-vs-settled-rate backtest discipline that avoids a subtle lookahead trap. Whoever wrote it understood the operational failure modes.

The problems are almost all **above** the execution layer:

1. **Premise mismatch (BLOCKER).** The tutorial builds a *cross-exchange funding-spread* bot, but the economics it inherits (5–10% APY) are for a *different* trade — single-venue *cash-and-carry level-harvesting*. On the majors it uses as its flagship example, the inter-venue spread structurally cannot clear its own round-trip cost over a realistic hold. The doc contains every fact needed to see this but never draws the conclusion.
2. **Custody is a footnote (BLOCKER for real capital).** The single largest money-loss vector in this business (FTX, Bybit hack) — off-exchange settlement / where the collateral actually lives — is one paragraph in `05`, while `03`/`04` correctly call it the central institutional risk mitigation.
3. **Build-vs-buy is never asked (MAJOR).** `03` documents that Hummingbot ships a Funding-Rate-Arbitrage V2 strategy for exactly Binance+Bybit and that NautilusTrader is best-in-class; the tutorial is a build-from-scratch blueprint that never weighs adopting the 90% and building the 10%.
4. **A cluster of internal inconsistencies (MAJOR/MINOR)** across the doc set: round-trip cost (0.20% vs 0.29%), buffer sizing (≥16% vs ≥25–30%), Ethena book size ($3–7B vs $5–15B vs actual ~$4B), and — most importantly — sizing/liquidation math that uses a **constant** maintenance margin while a pitfall elsewhere correctly warns it is **tiered**.
5. **N-venue readiness is asserted but not designed (MAJOR).** There is no venue-adapter contract; the architecture is single-region/single-binary; and the concrete quirks (Binance route prefixes, Bybit deltas, position modes) leak into the core instead of being isolated behind an interface.

None of this is fatal. The execution scaffolding is good enough that the right move is to **re-aim** it, not rebuild it. Recommendations in §9.

---

## 2. Premise-level findings — *should we build this, and which variant?*

### P1 — Spread-vs-level mismatch: the flagship trade is structurally the thinnest one, on the assets where it works worst. **[BLOCKER, confidence: high]**

**Claim.** Cross-exchange funding arb captures the *difference* between two venues' funding rates (`02 §2.3-C`: "Net = F_B − F_A > 0"). The 5–10% APY figures throughout `README`/`01`/`04` are for capturing the funding *level* — single-venue cash-and-carry (long spot + short perp), which is what Ethena industrializes. **These are two different businesses with different edge magnitudes**, and the tutorial silently imports the economics of one into the design of the other.

**Why it matters (the doc's own numbers).** Round-trip cost is ~0.29% of notional (`05 §4.1`). The doc's own rule is "demand ≥ 2× break-even" at a "realistic 1–3 cycle hold" (`§3.3`, `§4.1`). Working that through:

| Hold | Break-even spread (per 8h) | 2× bar (per 8h) |
|------|---------------------------|-----------------|
| 1 cycle (8h) | 0.29% | **0.58% = 58 bp** |
| 3 cycles (24h) | 0.097% | **0.19% = 19 bp** |
| 8 cycles (~2.7d) | 0.036% | 0.073% = 7.3 bp |

On BTC/ETH, Binance and Bybit funding are tightly correlated (shared global spot index, shared arb pressure), so the **inter-venue spread** is typically sub-bp to low-single-digit bp per 8h and mean-reverts within hours. A 19–58 bp inter-venue funding spread on a major is a crisis-grade dislocation, not a Tuesday. The tutorial's *own* §2.5 example — a 1 bp spread held 7 days — **loses money** (21 cycles × 1 bp = 21 bp < 29 bp cost).

**The structural bind, stated precisely:** to amortize a *thin* spread you must hold *long*; but thin spreads don't persist long (they mean-revert faster than the hold needed to cover costs). On majors, the intersection of "spread wide enough" × "persists long enough" is nearly empty. The doc has all three pieces — the 0.29% cost, the "spreads compress fast" observation, and the §2.5 loss example — but never connects them into the strategic conclusion.

**This does not kill the project.** It re-aims the flagship. Options (to decide with the user, §9):
- **(a) Flagship = single-venue cash-and-carry** (long spot + short perp), capturing the *level*; use cross-venue only to *relocate the short* when a venue degrades (this is literally how Ethena's hedge engine works, per `03 §3.10`). Cleanest path to the 5–10% the research actually documents.
- **(b) Keep cross-exchange, but scope it to where the spread is real:** alt pairs (wider, more idiosyncratic inter-venue spreads) and genuine dislocation windows — with the much higher edge bar those require, and explicit acknowledgment of the capacity/tail cost.
- **(c) Treat the cross-exchange bot as infrastructure/learning scaffolding** whose real value is the venue-adapter + reconciliation + risk engine, which a level-harvesting or alt strategy then reuses.

**Also:** cross-exchange perp-perp *adds* inter-venue basis risk, capital inventory drift (`§4.7`), and a *second* counterparty to capture a *thinner, more mean-reverting* signal than single-venue cash-and-carry. The only genuine advantages are 100%-margin capital efficiency and not needing spot custody — both real, but `04 §4.6` notes locked-margin opportunity cost is the most-ignored cost anyway. The tutorial never lays this trade-off out.

### P2 — The hurdle rate has risen; net edge may not clear it. **[MAJOR, confidence: high]**

Per verified market data (§7): tokenized T-bills (BUIDL/USDY) now yield **~4.6%** and are on-chain accessible as *productive collateral*. So the strategy must net **>4.6%** just to beat parking the same capital in tokenized T-bills — before accounting for its operational risk, tail risk, and (for US persons) ordinary-income tax that `04 §4.4` shows can halve the net. `01 §1.5` already concedes "true alpha ... probably 1–3% over T-bills." Combined with P1, the honest MVP framing is: **on majors, expected net edge over the risk-free-on-chain floor is near zero or negative**; the project's justification must rest on (i) alt/dislocation capture, (ii) collateral-yield stacking, or (iii) building durable infrastructure/learning — not on majors spread capture.

### P3 — ADL is a *realized* first-order hedge-failure mode; sizing must model it, not footnote it. **[MAJOR, confidence: high]**

The docs cover ADL (`05 §4.4`, pitfall #5; `01 §1.4`) — good. But verification underscores it was *the* marquee lesson of Oct 10–11 2025: exchange auto-deleveraging force-closed **profitable short hedge legs**, leaving "delta-neutral" desks naked-long into a −14% BTC / −40% alt move; Hyperliquid alone force-closed $10B+ in its first ADL in 2+ years. Implication for the build-spec: **hedge-leg ADL is a when-not-if, not a tail curiosity.** Sizing (`§4.2`) currently reasons about *your own* liquidation distance; it must *also* budget for the hedge leg being ADL'd out from under you — i.e., size so that a suddenly-naked leg at a stress price is survivable, and prefer venues by ADL-queue mechanics (your profitability rank on the contra side determines ADL priority). This is absent from the sizing math.

### P4 — Build-vs-buy is never asked. **[MAJOR, confidence: high — pending tooling agent for specifics]**

`03 §3.2` states Hummingbot ships a **Funding-Rate-Arbitrage V2** strategy supporting Binance perp + Bybit (+ Hyperliquid, dYdX, OKX, others) with configurable size/min-profitability/take-profit, and that NautilusTrader is "best-in-class for institutional-grade crypto arb" with backtest-live parity. It even notes production teams "wrap Hummingbot with their own reconciliation watchdog." The tutorial is a from-scratch blueprint that never weighs: *fork Hummingbot's funding-arb (the 90%) and build the reconciliation/risk/backtester differentiators (the 10%)* vs *build on Nautilus's event core* vs *greenfield*. For an MVP targeting real capital, this is arguably the single most important engineering decision and it's not on the page.

**Verified answer (§7.4):** the defensible MVP posture is **BUY the plumbing, BUILD the edge.** Buy: NautilusTrader (engine + backtester + existing Binance/Bybit adapters), Tardis.dev (historical live-estimate data), CCXT for connectivity (but *not* its unified funding). Build: the live-estimate signal from raw streams, the two-venue delta-neutral risk/liquidation engine, custody-rail integration, and funding-aware backtest accounting. Hummingbot's funding-arb script is a *reference*, not a foundation to run real capital on. So "build from scratch" (the tutorial's implicit stance) is the wrong default — but so is "just use Hummingbot."

### P5 — Competitive honesty: on majors you are front-run. **[MINOR/context, confidence: high]**

`04 §4.2` is blunt ("You have largely been front-run" by Ethena's book + institutional OES/rebate/prime-broker flow). The build doc should inherit this framing so the MVP's *niche* is explicit from line one: alt pairs, nimble entry/exit, dislocation capture, and collateral stacking — not competing with prime-brokered desks on BTCUSDT.

---

## 3. Correctness & internal-consistency findings — *the math/mechanics layer*

### C1 — Round-trip cost is inconsistent across docs, and `02` miscounts slippage. **[MAJOR, confidence: high]**

- `05 §4.1`: 0.05%/0.055% taker + 0.02% slippage × 4 fills → **0.29%**. Internally consistent.
- `02 §2.4`: "Total cost = 4 × 0.04% + 2 × 0.02% = 0.20%." Two problems: (i) **old 0.04% taker fee**; (ii) **only 2 slippage events counted** — there are 4 fills (open+close × 2 legs), so slippage should be 4 × 0.02% = 0.08%, giving 0.24% even at the old fee. The "20 cycles → 6.7 days" break-even downstream inherits the error.
- The **single source of truth** must be the programmatic fee fetch (`§4.1` `commissionRate`/`fee-rate`) plus a measured slippage model — and every worked example must derive from it. Right now three different round-trip numbers float around the doc set.

### C2 — Per-hour "continuous" framing obscures that funding is a *discrete* payment at settlement. **[MAJOR, confidence: high]**

`§3.3` normalizes to `rate_per_hour = live_rate / interval_hours` and offers `break_even_spread_per_hour = round_trip / expected_hold_hours`. Per-hour is fine for *ranking* mismatched-interval pairs, but it quietly implies you accrue funding continuously and can capture it pro-rata. **You do not.** You capture a settlement's funding only if you *hold across that settlement timestamp*. Hold 3 hours between settlements → capture **zero**. For real sizing, expected capture must **enumerate the actual settlement timestamps inside the planned hold on each venue** (which differ when intervals differ — Binance 8h vs a 4h/1h alt on Bybit) and sum expected rates, not multiply a per-hour rate by hours. The current framing can materially over- or under-state expected capture for mismatched-interval pairs — exactly the alt pairs P1 pushes you toward.

### C3 — The signal trades on the live estimate but never quantifies (realized − estimate) drift. **[MAJOR, confidence: high]**

`§3.3`/`§3.7` correctly distinguish audit (vs realized rate) from attribution (vs signal-time estimate) and note the live estimate "moves continuously between entry and settlement." But the *entry decision* is `spread ≥ 2× break-even` on the **instantaneous** estimate, and the 2× multiplier is asserted, not derived. The buffer you actually need is a function of the **variance of (realized_settlement_rate − entry_estimate)** over your hold. The spec should require measuring that drift distribution empirically per symbol/venue and sizing the min-edge multiplier from it (e.g., cover the p75–p90 adverse drift), rather than a flat 2×. Without this, the bot systematically enters trades whose edge evaporates by settlement — the dominant quiet-bleed failure for this strategy.

### C4 — Sizing & liquidation math use a *constant* maintenance margin; MMR is *tiered*. **[MAJOR, confidence: high]**

`§3.5` and `§4.2` compute "3x @ 0.5% MM → ~33% liq distance" and set monitor thresholds (open ≥30%, rebalance <20%, emergency <15%) against that. But `02 §2.4` and pitfall #9 correctly state Binance MMR is **tiered** (~0.4% at ≤$50K rising to ~5% at >$50M). At the §3.5 example size ($50K/leg) the constant 0.5% is fine; **at scale it is dangerously optimistic** — at a 5% tier, 3x gives ~28% liq distance, and crossing a tier mid-position *silently* raises required margin. The liq-distance and sizing functions must read the **current tier's MMR for the current notional** (and re-check on size changes), not a constant. The pieces are in the doc; they aren't wired into the formulas. This is a scaling landmine that surfaces exactly when capital grows — the worst time. (§7.2 confirms the fix: the exact bracket table isn't even publicly readable, so sizing MUST fetch `/fapi/v1/leverageBracket` at runtime.)

### C5 — Liquidation-buffer policy differs between docs. **[MINOR, confidence: high]**

`02 §2.5`: "keep ≥ 16% liq distance" (2× an 8% 99.9%/24h move). `05 §4.2`: "≥ 25–30% for BTC." Different numbers, different rationales (pre- vs post-Oct-10). The spec needs **one** sizing policy; `05`'s post-Oct-10 stress-inclusive number is the defensible one, and `02` should be reconciled to it (or explicitly marked as the superseded rolling-window view).

### C6 — Equal-base-quantity neutrality breaks for inverse (COIN-M) perps. **[MAJOR for design-for-N, confidence: high]**

The tutorial assumes **linear USDⓈ-M** everywhere (delta = notional/price). But `02 §2.1/§2.5` note inverse (COIN-M) perps have a `1/P` payoff and **nonzero gamma** (`Γ = −2/P³` per coin notional). The moment the platform generalizes (design-for-N) to a venue/instrument where a leg is inverse — e.g., Binance COIN-M vs Bybit linear, which `02 §2.3-D` explicitly lists as a strategy — equal-base-quantity is **not** delta-neutral and needs dynamic gamma rebalancing. The tutorial should state the linear-only assumption as an explicit scope boundary, and the spec's venue-adapter must carry a `contract_type` (linear/inverse) that the neutrality math branches on.

### C7 — "spread × notional" is an approximation because funding base = mark-at-settlement, and marks differ across venues. **[MINOR, confidence: high]**

`§1.3` correctly notes funding payment = `F × |qty| × mark_at_settlement`, and flags that dollar examples assume mark ≈ entry. But net funding for the pair is `q × (rate_short × mark_short − rate_long × mark_long)`, and `mark_short ≠ mark_long` (that's the basis). So even at equal base quantity, the two legs' funding bases differ slightly and "spread × notional" is an approximation off by the basis fraction. Bounded and usually tiny, but the PnL-attribution/audit code (`§3.7`) should compute funding per leg from each venue's settled mark, not from a single notional — otherwise the audit alert (C-grade) will false-positive on the basis.

### C8 — Bybit's dynamic settlement can change a contract's funding interval mid-position, without notice. **[MAJOR for alts / design-for-N, confidence: high — verified live]**

Verified: since **2025-10-30**, Bybit auto-switches a contract to **hourly** settlement when its funding pins the cap at settlement, then reverts **without prior announcement** (BTC/ETH currently on an exclusion list, but "reserves the right"). The mechanic was already live when the doc was written (May 2026), yet the doc treats a symbol's funding interval as a slowly-changing property you can "refresh periodically" (`§3.3`). It is not: on an alt, the interval can flip **8h→1h mid-trade**, which silently changes (i) funding-accrual timing, (ii) the per-hour normalization, (iii) the ±60–120s funding-boundary guard, and (iv) your expected-capture plan — all while you hold the position. **Requirement:** treat `fundingIntervalHour` + `nextFundingTime` as live per-symbol state read on *every* ticker update (not a periodic overlay), and have the monitor detect an interval change on an open position and re-plan/close. This is exactly why Bybit added the ticker funding fields (see C2, §7.3). Binance has **no** equivalent auto-switch (confirmed §7.2), so the venue-adapter must model this asymmetry per-venue.

### Strengths in this layer (do not lose in rewrite)
- **[STRENGTH]** Damper (±0.05% premium/interest clamp) vs per-symbol cap/floor correctly separated and repeatedly reinforced (`§1.4`, `§3.3`, pitfall #4). Rare and correct.
- **[STRENGTH]** Delta residual math for equal-USD vs equal-base sizing is correct: `q_long − q_short ≈ −N·basis/P²`, USD exposure `≈ −N·basis/P` (verified by hand).
- **[STRENGTH]** Basis-change PnL sign convention (`§3.8`) is correct and the "trips up many readers" callout is warranted.
- **[STRENGTH]** APR/APY arithmetic checks (0.01%/8h → 10.95% APR, 11.58% APY) — though prefer APR as the headline; APY's reinvestment assumption doesn't hold for funding receipts (NIT).

---

## 4. Architecture & N-venue readiness

### A1 — There is no venue-adapter contract; venue quirks leak into the core. **[MAJOR, confidence: high]**

This is *the* design-for-N refactor. The doc gestures at a normalized `FundingRateUpdate` schema (`§3.2`) but the concrete per-venue quirks — Binance `/market`|`/public`|`/private` route prefixes, `!markPrice@arr@1s` full-snapshot semantics, `fundingInfo` overlay, Hedge-Mode reduceOnly rejection; Bybit snapshot/delta ticker, `orderbook.50` reconstruction, `positionIdx` — are described *inline in the narrative* rather than isolated behind an interface. The spec needs an explicit **`VenueAdapter`** contract enumerating the capability/quirk surface the core must never assume: normalized funding (rate, interval, next-settlement, cap/floor, source), orderbook (reconstructed local book), positions, orders (with position-mode semantics), fee fetch, symbol/contract spec (linear/inverse, tick/lot), time source, and rate-limit accounting. The core (signal/risk/execution/monitor) must depend only on that contract. Without it, "generalizing to a third venue is straightforward" (`§intro`) is untrue — every core module has embedded venue assumptions.

### A2 — Single-region/single-binary won't hold for N venues. **[MAJOR, confidence: high]**

`§5.1` recommends "a single binary process per region"; `§5.3` says "Tokyo is typically fine for both." But `03 §3.4` places Bybit in Singapore, OKX in HK/SG, DEX legs elsewhere. For design-for-N you get a latency-asymmetric, multi-region topology and need an explicit story for: which components run where, how in-memory state is shared/replicated across regions, and how the signal engine reasons over venues it is far from. The current design is a single-region SPOF with an unstated cross-region state model.

### A3 — Durable state / crash-recovery is "optional"; for real capital it is mandatory. **[MAJOR, confidence: high]**

`§5.1` says in-memory "or Redis if you want crash-recovery." But the (excellent) reconcile-only boot protocol (`§5.1`) rebuilds trade state by *joining venue ground truth against the audit log written by Module G* — which only works if that audit/trade store is **durable and append-only**. For real capital this is not optional. Make it a hard requirement: durable, append-only trade+audit store; the in-memory store is a cache rebuilt from it + venue truth on boot.

### A4 — Funding-ledger / income ingestion needs exactly-once semantics. **[MINOR, confidence: high]**

The audit path (`§3.7`, C-grade) compares recorded funding events to venue-settled amounts. WebSocket gaps + periodic REST reconciliation (`§3.2`) can double-count or miss income events. The spec needs an idempotency/dedup key per funding event (venue + symbol + settlement timestamp) so REST backfill and WS delivery converge instead of double-booking.

### A5 — The kill switch is too simplistic for real capital. **[MAJOR, confidence: high]**

`Part 8`: "Kill switch flattens all positions with one command." Two problems: (i) flattening is most expensive/impossible *exactly when* you'd hit the switch (thin books, rate limits, a venue down) — a single "flatten" verb hides that; (ii) if the bot itself is wedged, an in-process kill switch is unreachable. Spec needs **modes** — (a) stop-opening / (b) flatten-if-feasible-within-slippage-cap / (c) hard-halt-and-alert — plus an **out-of-band** trip (operator can disable trade keys / pull IP allowlist / hit a venue-side kill independent of the bot process). Tie mode (b) to the same reduce-only reconcile path as `§3.6`.

### Strengths
- **[STRENGTH]** Reconcile-only boot protocol (`§5.1`) — fetch positions/open-orders/funding-ledger, join to audit log, alert-don't-auto-correct on mismatch. Exactly right; keep verbatim.
- **[STRENGTH]** Module decomposition (A–H) is clean and maps well onto the adapter refactor.

---

## 5. Security / custody / operations — *the real-capital bar*

### S1 — Custody / off-exchange settlement is a footnote; it should be a first-order section. **[BLOCKER for real capital, confidence: high]**

`05` treats counterparty risk as "you might lose 100% of your margin there" (`§2.4`) and mentions OES only in passing (`§4.7`, closing). But `03 §3.10` and `04 §4.4` make the case that OES (Copper ClearLoop / Ceffu MirrorX / Fireblocks Off-Exchange) is *the* institutional mitigation — Ethena survived the Feb-2025 Bybit hack precisely because collateral never sat on the exchange. For a real-capital multi-venue platform, custody is a first-order architecture decision: where do assets live, how is margin mirrored to each venue, key/withdrawal controls, and the T+1 settlement penalty vs counterparty-exposure trade-off. The build-spec needs a **Custody & Settlement** section; today the biggest historical money-loss vector in this business is absent from the build doc.

**Verified, and worse than the doc implies (§7.4):** for the *specific* Binance+Bybit pair there is **no single OES rail** — Binance is Ceffu-MirrorX-only; Bybit is Copper/Fireblocks-only — so you'd run **two custodians** and cannot do a pure off-chain collateral hop between the two venues' pools. That partially defeats the "shift capital between venues without on-chain hops" story the docs lean on (and compounds the inventory-drift problem in `§4.7`). And OES is **necessary-but-not-sufficient**: it removes custody/counterparty risk but leaves you fully exposed to each venue's margin/mark-price engine (Oct-10 USDe local mark liquidated OES users too). The Custody & Settlement section must state both limits plainly rather than presenting OES as a clean fix.

### S2 — API-key security is thin for the risk bar. **[MAJOR, confidence: high]**

`Part 8` has "sub-account-scoped, IP-whitelisted, separate read-only and trade-enabled keys" — good but incomplete. Missing, and non-negotiable for real capital: **trade keys MUST NOT carry withdrawal permission** (the single highest-blast-radius control); keys stored in HSM/Vault/KMS (`03 §3.8` says Vault — `05` doesn't); rotation policy; and an explicit blast-radius analysis of a leaked trade key (bounded to position manipulation within IP allowlist, no exfiltration). Promote these into the spec as MUSTs.

### S3 — Audit log needs durability + tamper-evidence. **[MINOR, confidence: high]**

The audit log is both the reconciliation source of truth (`§5.1`) and the forensic/tax record. It should be append-only and tamper-evident (e.g., hash-chained), not just "persisted to durable storage" (`Part 8`).

### S4 — "Escalate to operator" is not an automated stop-loss. **[MAJOR, confidence: high]**

`§3.6`'s stuck-residual outcome and `§3.7`'s "consider closing the leg" both terminate in "alert operator." For an *automated* real-capital system, a stuck one-leg residual at 3am is unbounded directional exposure until a human wakes. The spec needs an **automated escalation ladder** with bounded-loss stops (widen slippage → time-boxed retries → panic-MARKET behind the documented flag → hard directional stop-loss on the naked leg), where operator alerting runs in parallel, not as the terminal action.

---

## 6. Backtesting & validation

### B1 — Backtester fidelity is a strength, with three gaps. **[MINOR→MAJOR, confidence: high]**

`§5.5` is one of the best parts of the doc (live-estimate-vs-settled lookahead trap, need for captured WS streams, Tardis/Coinglass caveat, replay all exit rules, the "20% Sharpe 3 → 8-12% Sharpe 1.5" friction warning). Gaps:
- **Spread-persistence model.** The backtest must model how the *spread* decays over the hold (ties to C2/C3), or it will overstate edge by assuming entry spread persists.
- **Adverse-selection fill bias.** Your IOC fills preferentially when the book is moving *against* you; naive "fill at mid ± budget if depth available" understates realized slippage. Model fills conditional on concurrent price motion.
- **Survivorship bias** for the alt pairs P1 pushes toward — delisted/forced-settled pairs (pitfall #12) must be in the historical set or the backtest is rosy.

### B2 — No testnet step and no explicit test taxonomy. **[MAJOR, confidence: high]**

`Part 8` lists "fault injector" + "backtest," but for real capital the spec should name the full ladder: unit (funding normalization, book reconstruction, break-even, liq-distance-by-tier), **property-based** (book invariants — `03 §3.9` cites Hypothesis/proptest), **exchange-testnet integration** (Binance & Bybit both offer testnets — unmentioned), deterministic replay / chaos fault injection, and a **paper-trading acceptance gate** with pass/fail criteria before real capital. Right now the jump from "backtest" to "$1K real" (`§5.6`) skips integration testing against the real API surface in a safe environment.

---

## 7. Technical-claim verification (agent results)

### 7.1 Market & event facts — **COMPLETE**

Verified 2026-07-18. Doc's market narrative is largely current. Corrections that matter:
- **Ethena short book: doc says $5–15B (README) / $3–7B (`04 §4.2`) — actually ~$4B now** (peaked ~$14.5B early 2025; shed $8B+ in/after Oct crash; ~$5.9B Mar 2026 → ~$4.0–4.7B now). Update both figures; note the doc is internally inconsistent.
- **USDe Oct-10 low: doc says $0.62 (`§2.4`,`§4.6`) — authoritative low ~$0.65** on Binance (CoinDesk); some feeds ticked ~$0.62. And it was a *local venue* dislocation, not a protocol depeg (Ethena processed >$2B redemptions with zero downtime). Tighten the wording so it's not read as "USDe depegged."
- **ADL** confirmed as the marquee realized risk (Hyperliquid force-closed $10B+; first ADL in 2+ yrs) → drives P3.
- **Hurdle rate:** tokenized T-bills ~4.6% and on-chain accessible → drives P2.
- **Funding intervals fragmenting** (many alts moved to 4h/1h; 8h still BTC/ETH default) — confirmed; reinforces C2 and the normalization logic.
- $19B liquidations (CONFIRMED, likely understated), 1.62M accounts (CONFIRMED), BTC ~14.5% (CONFIRMED), alts −30%+ (CONFIRMED), perps >75% volume (CONFIRMED), BitMEX 2016 (CONFIRMED).

### 7.2 Binance API surface — **COMPLETE** (verified against live docs 2026-07-18)

**10 of 13 claims fully CONFIRMED; the doc's Binance surface is accurate and current — a genuine strength given how fast this changes.** Details:
- **[CRITICAL, now LIVE] WS `/market` routing is mandatory.** CONFIRMED and *more urgent than the doc frames it*: the legacy `wss://fstream.binance.com/ws/…` and `/stream/…` paths were **decommissioned 2026-04-23**; unmigrated connections now receive **only `/public`**, so `!markPrice@arr`/`@markPrice` and User Data return **nothing**. Funding feed MUST use `…/market`, depth/bookTicker `…/public`, User Data `…/private`. A bot still on the old path has had **no funding data since April 2026**. The doc's "verify before deploying" should become "the old path is dead — route to `/market` now."
- `markPrice@arr@1s` fields (`p` mark, `i` index, `r` funding, `T` next-funding, **no interval**) — CONFIRMED. Interval must come from `fundingInfo` (absent ⇒ 8h default) + the `T` countdown; the formula carries a `/(8/N)` scaling for non-8h symbols (relevant to C2's normalization).
- Funding formula + ±0.05% damper; BTCUSDT ±0.3% cap (= 0.75×MMR) / alts ±2% default; `fundingInfo` overlay; `commissionRate` (now also returns `rpiCommissionRate`); 15s funding-time deviation; `X-MBX-USED-WEIGHT-*`/`ORDER-COUNT-*` headers + 429→418 ban (2 min→3 days); Hedge-Mode `reduceOnly` rejection (error **-2022**) + `positionSide` close; MARKET-has-no-price-cap — **all CONFIRMED as written.**
- **Two minor factual corrections:** (a) the doc's "taker raised 0.04%→0.05% in **May 2026**" *timing* is **unsupported** — 0.05% predates May 2026 and no such announcement exists; the *value* (maker 0.02%/taker 0.05%) is right and the "fetch via `commissionRate`" advice makes it moot. (b) Futures partial-depth default cadence is **250ms** (not spot's 1000ms), with 100/500ms options — small precision fix for §3.2.
- **C4 confirmed + concrete fix:** BTCUSDT MMR *shape* verified (tier-1 0–50k @ 125×, rising toward ~5% in the multi-$M range; 0.5% ≈ tier-2), but the exact bracket table isn't publicly readable (JS-rendered; `leverageBracket` is auth-only). **Fix: fetch `/fapi/v1/leverageBracket` at runtime** and drive sizing/liq-distance from it. Binance has **no** Bybit-style dynamic-settlement auto-switch (confirms the C8 asymmetry).

### 7.3 Bybit API surface — **COMPLETE** (verified against live API 2026-07-18)

All 11 claims **CONFIRMED** against official V5 docs + live `GET /v5/market/tickers` / `instruments-info`. Highlights:
- **Critical open question resolved: YES** — the linear `tickers.<symbol>` WS exposes `fundingIntervalHour`, `fundingCap`, `fundingRate`, `nextFundingTime`, `markPrice`, `lastPrice`, `indexPrice`. So `05 §3.3`'s "get interval from Bybit ticker" path is correct. **Caveat:** these funding fields are *new* (WS 2025-10-16 / REST 2025-10-23), so any SDK/client older than that won't parse them — the spec must pin post-Oct-2025 clients. They arrive in the **snapshot** and are omitted from unchanged deltas → must not be coerced to 0/null (the doc's snapshot/delta rule covers this).
- Snapshot/delta ticker, `orderbook.50` reconstruction, `fee-rate` endpoint, fees **0.020%/0.055%**, ±5s funding window, `positionIdx` 0/1/2, Market→IOC-with-band, and `fundingCap` (live BTCUSDT = 0.5%/8h, read from ticker or `instruments-info.upperFundingRate/lowerFundingRate`) — all CONFIRMED as written. Note the doc's *static example* cap (0.00375) already differs from the live value (0.005) — proof that hardcoded constants drift; the doc's "read live" rule is right.
- **NEW, load-bearing, doc misses it → see C8:** Bybit **Dynamic Settlement Frequency System** (live 2025-10-30) auto-switches a contract to hourly settlement when funding pins its cap, then reverts **without notice**. BTC/ETH currently excluded (stable 8h), but any alt can flip 8h↔1h mid-position.
- Rate-limit nuance: two independent limits — per-UID-per-endpoint (`X-Bapi-Limit*` headers, retCode 10006/403) **and** an *unheadered* IP-level gateway ban (~600 req/5s, retCode 10018) that is the true analog to Binance's 418. The doc's §3.6 references only the headers.
- Minor: Bybit now supports an **optional user `slippageTolerance`** on market orders (TickSize/Percent), so `05 §3.6`'s "that band is set for the venue's risk, not your strategy's" is slightly outdated — though the recommendation to use explicit marketable IOC limits still stands.

### 7.4 Tooling / backtest data / custody — **COMPLETE** (verified 2026-07-18)

- **Frameworks:** NautilusTrader v1.230.0 (LGPL-3.0) is the best fit — Rust core + Python, unified backtest+live event loop, deterministic sim, **has Binance USDⓈ-M + Bybit adapters**. Hummingbot v2.15.0 ships a `v2_funding_rate_arb.py` cross-exchange *reference script* — a starting point, **not** a production two-venue risk book, and no backtester of the needed fidelity. barter-rs (MIT) is thinner on Binance/Bybit funding specifics. CCXT / CCXT Pro is great for connectivity but its **unified funding struct is inconsistent across venues** (Binance `fundingRate` = *previous* rate; Bybit differs; timestamp semantics differ) — **do NOT use the CCXT abstraction for the live-estimate signal; read raw exchange fields** (validates the doc's raw-`r` insistence). → drives **P4**.
- **"Open source covers ~90%" (README) — verified verdict: true for *plumbing*, misleading for *this strategy*.** The differentiated ~10–30% you must build: (1) live funding-*estimate* signal from raw streams, (2) two-venue delta-neutral margin/liquidation risk (frameworks are largely single-venue-position oriented), (3) OES/collateral-rail integration (no framework does this), (4) funding-aware backtest accounting. → sharpens **P4**.
- **Backtest data (drives B1):** **Tardis.dev = the buy** — cleanly records *both* live-estimate streams (Binance `markPrice@1s` carrying `r`/`p`/`i` since 2020-02-13; Bybit `tickers` since 2023-04-05), raw + normalized (`derivative_ticker.predictedFundingRate`), ~4yr history, $350–6000/mo, min order $300, no free tier. **Coinglass = NOT usable for the estimate signal** — funding is OHLC candles only (sampled ~realized), *not* the per-second predicted-funding stream; even 1m funding OHLC ≠ the live `r`. This **resolves the doc's own open caveat**: use Tardis (and/or self-capture), never Coinglass, for the live-estimate backtest. Still self-capture both streams from day one for exact live/backtest parity.
- **Custody / OES (drives S1) — MVP-BLOCKER:** **no single rail spans both venues.** Binance → **Ceffu MirrorX only**; Bybit → **Copper ClearLoop or Fireblocks** (neither supports Binance). So the Binance+Bybit pair forces **two custodians**; capital shifts freely *within* each rail's mirrored pool but moving collateral *between* the Ceffu(Binance) and Copper/Fireblocks(Bybit) pools crosses custodians — **not** the pure off-chain hop the docs imply. Also verified: **OES protects custody/counterparty risk, not exchange-side liquidation/mark-price risk** — Binance's local USDe→$0.65 mark still force-liquidated OES users on Oct 10. OES is necessary-but-not-sufficient.

---

## 8. Pedagogy & the two-artifact split

The user chose to keep the tutorial *and* extract a build-spec. Guidance for each:

**Tutorial (keep the voice):**
- Add an explicit **Scope & Assumptions box** up front — currently scattered: linear USDⓈ-M only, One-way position mode, USDT/USDC margin, majors as example, 8h funding assumed, Binance+Bybit. (`C6`, position-mode note in `§3.6`, USDe-collateral warning in `§4.6` all belong here.)
- Consolidate the three separate treatments of funding-timing (`§1`, `§3.3`, pitfall #3) and caps (`§1.4`, `§3.3`, pitfall #4) — currently repeated with slight variations.
- Add the P1 strategic framing so a learner doesn't walk away thinking majors cross-exchange arb is a live business.

**Build-spec (new, terse, normative):**
- Requirements as **MUST/SHOULD**, not prose.
- **Data model** (Trade, Order, Fill, FundingEvent, Position, VenueConfig, SymbolSpec) and **config schema** (caps/thresholds/per-venue params) — currently only inline fragments.
- **`VenueAdapter` contract** (A1).
- **Custody & Settlement** section (S1).
- **Security** MUSTs (S2/S3).
- **Test plan** taxonomy + paper-gate criteria (B2).
- **Tax-record hooks** — `04 §4.4`: funding receipts are ordinary income, each settlement a realized event; the bot MUST emit per-event USD-valued records for tax. Zero coverage in `05` today.
- **Treasury ops** components — funding venues, rebalance rails, inventory-drift automation (`§4.7` has the logic; make it a component).

---

## 9. Recommendations / proposed next steps

**Decide first (needs user):**
1. **Flagship re-aim (P1).** Pick (a) single-venue cash-and-carry as flagship with cross-venue as hedge-relocation, (b) cross-exchange scoped to alts/dislocation, or (c) cross-exchange as reusable infra. Everything downstream depends on this.
2. **Build-vs-buy (P4).** Greenfield vs fork-Hummingbot-funding-arb vs build-on-Nautilus. (Await §7.4.)

**Then correct (no decision needed):**
3. Single-source the round-trip cost and fix `02`'s slippage miscount (C1); reconcile buffer policy (C5); update Ethena book + USDe-low figures (7.1).
4. Wire tiered-MMR into sizing/liq-distance (C4); add discrete-settlement expected-capture (C2); require empirical drift-based min-edge (C3).
5. Promote to first-order sections: Custody/OES (S1), key security (S2), automated escalation ladder (S4), kill-switch modes (A5).
6. Author the `VenueAdapter` contract (A1); make durable state mandatory (A3); add linear/inverse branch (C6).
7. Backtester gaps (B1) + test taxonomy/paper gate (B2).

**Then iterate:** self-review → Codex copilot rounds to convergence → external reviewers.

---

## Appendix — finding index by severity

- **BLOCKER:** P1 (spread-vs-level), S1 (custody footnote). Plus P2/P3 as blocker-adjacent (viability).
- **MAJOR:** P3, P4, C1, C2, C3, C4, C6, C8, A1, A2, A3, A5, S2, S4, B2.
- **MINOR:** P5, C5, C7, A4, S3, B1(partial).
- **NIT:** APY-as-headline, redundancy consolidation.
- **STRENGTHS (preserve):** damper/cap separation, delta-residual math, basis-PnL sign, reconcile-only boot, one-leg reduce-only reconciliation, backtester lookahead discipline, leverage-yield skepticism.
