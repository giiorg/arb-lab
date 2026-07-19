# Building a Funding-Rate Arbitrage Bot — A Teaching Companion

**Audience.** Senior engineers with strong systems instincts and little-to-no derivatives background. This teaches the *why*; the normative *what to build* lives in [`docs/build-spec.md`](./build-spec.md), which this document cross-references as **§spec**. Where the two ever disagree, the build-spec wins.

**What changed from the legacy tutorial.** The frozen original ([`docs/legacy/05-tutorial-building-a-bot.md`](./legacy/05-tutorial-building-a-bot.md)) taught a **cross-exchange perp–perp** trade as the flagship. Deep review showed that trade is structurally thin on majors (it can *lose* money after costs), so this reframe leads with **cash-and-carry** — the trade that actually harvests the funding *level* — and demotes cross-exchange to a scoped strategy #2. Load-bearing market/API facts were re-verified against official venue docs on **2026-07-18 (v0.6 round)** and **2026-07-19 (v0.7 round — isolated-margin top-up, position-mode, key-management/fence endpoints, SQLite durability)** (the normative endpoints + their sources live in §spec §4/§4.1/§4.2 and the review records; treat any fact older than a venue API cycle as due for re-checking). The teaching voice is preserved; the framing and the facts are fixed.

---

## Scope & Assumptions

Read this box first — it bounds everything below.

| Assumption | MVP choice | Why it matters |
|---|---|---|
| **Contract type** | **Linear USDⓈ-M / USDT-margined perps only** | Equal-base delta-neutrality math assumes linear payoff. Inverse (COIN-M) perps have a `1/P` payoff with nonzero gamma and would break the math (§spec C6). |
| **Position mode** | **One-way mode** (single net position per symbol) | The venue nets *all* strategies' exposure on a symbol. The engine enforces an exclusive lease per `(account, venue, symbol)` — no cross-strategy netting in MVP (§spec §5.1). |
| **Margin collateral** | **USDT or USDC only** | The Oct-2025 USDe local-mark failure is the reason. No exotic stablecoins, LSTs, or yield-bearing margin. |
| **Custody** | **On-exchange, small size, per-venue caps ≤~20–30%** | A conscious, documented acceptance of exchange risk to ship. Off-exchange settlement (OES) is the scale-up path, not an MVP requirement (§spec §7.2). |
| **Time** | **Exchange server-time is authoritative** | Your local clock is for latency/freshness only, never for funding-boundary decisions (§spec §3.5). |
| **Money** | **Scaled-integer `fixedpoint`, never `float64`** | Determinism, no rounding drift, reproducible backtests. All prices/sizes/notionals/rates are dimensional types (§spec §3.6). |
| **Teaching venues** | **Binance + Bybit**, majors (BTC/ETH) as examples | Built N-venue-ready behind one adapter seam (§spec §4). |

---

## Part 1 — A 10-minute market crash course

### 1.1 Spot, futures, and perpetuals
- **Spot:** buy the asset, own it. No leverage, no funding, no liquidation *of the spot itself*. If you hold 1 BTC spot outright nobody force-closes it — though whether it can be pulled in to cover a *derivatives* loss depends on your account posture (§spec §4.1), and we deliberately pin one where it can't.
- **Futures:** an agreement to trade at a future date/price. Expires.
- **Perpetual ("perp"):** a future with **no expiry**. To keep its price tethered to spot without a settlement date, it uses a **funding rate** — the anchor this whole business is built on.

### 1.2 The funding rate in plain English
A perp can drift from spot. To pull it back, the exchange makes the crowded side *pay* the other side, periodically (commonly every 8h, but see 1.7 and §spec §4):
- Perp trading **above** spot (longs crowded) → **funding positive** → **longs pay shorts**.
- Perp trading **below** spot → **funding negative** → **shorts pay longs**.

The exchange only orchestrates the transfer; it takes no side. Funding is the counterparties paying each other to hold the tether.

### 1.3 Terminology you need
**Long/short** (bet up/down) · **notional** (position size in quote currency, e.g. USDT) · **margin** (collateral posted) · **leverage** (notional ÷ margin) · **liquidation** (forced close when margin can't cover losses) · **funding rate `F`** (per-interval %) · **funding payment** (`F × notional`, paid at the settlement timestamp on the **mark price at settlement**, not your entry) · **funding interval** (hours between settlements — *live state*, see 1.7).

### 1.4 The funding formula, and the two things people conflate
Simplified: `F ≈ premium_index + clamp(interest − premium, ±damper)`. Two distinct clamps that people constantly mix up:
- **The damper** (Binance ~±0.05%): a small band *inside the formula* that keeps `F` near the interest term when premium is small.
- **The cap/floor** (per-symbol, e.g. BTC ±0.3%, alts ±2%): the *hard limit* on the final rate. **Read it live** — Bybit exposes `fundingCap` on its ticker; Binance overlays via `fundingInfo` (§spec §4).

Damper ≠ cap. The damper shapes normal rates; the cap bounds extreme ones.

### 1.5 What funding means in dollars — and the seduction to resist
Say BTC perp funding is **+0.01% per 8h**. Short **$100,000** of perp and you *receive*:
`0.0001 × $100,000 = $10 every 8h` → `$30/day` → **~10.95% APR** on the notional.

That's a **single-venue *level*** number — and it's exactly the cash-and-carry edge (Part 3). Now the trap: if you posted only $10,000 margin (10× leverage), that $30/day reads as **~110% APR on margin**. **Report yield on the full capital you put at risk, never on posted margin** (§spec §2.1). The leverage number is a mirage that ignores the liquidation risk you took on to get it.

### 1.6 Mark vs index vs last
Three prices, do not confuse them:
- **Last:** the last trade on *this* venue. Noisy, manipulable.
- **Index:** a basket of spot prices across venues. The "true" reference.
- **Mark:** a smoothed price derived from index + basis. **Drives liquidation and funding.** Your live funding *estimate* is anchored to mark, not last.

### 1.7 Funding is discrete, not continuous *(new — a correction)*
Funding is a **payment at a timestamp**, not a stream that accrues by the second. You capture a settlement **only if you hold across it**. This has two consequences the legacy tutorial got wrong (§spec C2):
- To estimate capture over a hold, **enumerate the actual settlement timestamps** inside the hold on each leg and sum expected per-settlement rates. Do **not** multiply a per-hour rate by hours.
- The **interval itself is live state**. Since 2025-10-30 **Bybit auto-switches an alt between 8h and 1h** when funding pins the cap, and reverts **without notice** (BTC/ETH excluded; alts not). Read `fundingIntervalHour` + `nextFundingTime` on every ticker update; the monitor must detect a mid-position flip. Binance has no equivalent auto-switch (§spec C8).

---

## Part 2 — The edge, stated honestly

This is the part the legacy tutorial soft-pedaled. Read it before writing a line of code.

### 2.1 Two different trades, two different edges
- **Funding *level* — cash-and-carry.** Long spot + short perp on **one** venue. You collect that venue's funding whenever it's positive. This is the ~5–10% APY the research documents and what Ethena industrializes. **The level is the edge.**
- **Funding *spread* — cross-exchange.** Long perp on the low-funding venue, short perp on the high-funding venue. You collect `F_high − F_low`.

They look similar. They are not. One harvests a *level*; the other harvests a *difference of levels*.

### 2.2 Why cash-and-carry is the flagship
- The **level** on majors is a real, persistent, positive number most of the time.
- The **spot leg is not liquidated or ADL'd to cover the perp** — *under the segregated/isolated account posture we pin and verify at boot* (§spec §4.1); you own it outright. Your only forced-close risk is the perp short, and its loss on a rally is offset by the spot's gain — the margin engine only sees the leg it's pinned to, which is exactly why the posture matters.
- No inter-venue basis risk, no second exchange counterparty for the hedge.

That combination — real edge, structurally safer hedge — is why we prove cash-and-carry first.

### 2.3 Why cross-exchange spread is thin on majors
Here is the legacy tutorial's own example, now with its conclusion stated:

> Round-trip cost is **~0.29%** (perp maker/taker both legs). Suppose the cross-venue funding spread on a major is **1 bp/8h**. Over a **week** (21 settlements) you collect `21 × 1 bp = 21 bp`. Cost `29 bp`. Net: **−8 bp — a loss.**

On majors the spread is sub-bp to low-single-digit-bp/8h and mean-reverts within hours, so it rarely clears the round-trip. Therefore cross-exchange is **scoped to alt pairs and dislocation windows** with a much higher edge bar, and demoted to strategy #2 (Part 5, §spec §2.3). This is the single most important honesty correction in the reframe (§spec P1).

### 2.4 The hurdle rate and the competition
Your net APY isn't measured against zero — it's measured against the **on-chain *near*-risk-free floor: tokenized T-bills at ~4.6%** ("near" — it still carries smart-contract and depeg risk), accessible with far less operational risk (§spec P2). On majors, the net edge *over that floor* is near-zero or negative, and you're **front-run** by Ethena and prime-brokered desks running the same trade at lower cost. The **more defensible** ground is likely **alts, dislocation windows, and collateral-stacking** — a hypothesis to validate in the backtester, not a law — rather than BTC/ETH cross-exchange.

### 2.5 Why neither is "free money"
The risks, ordered for cash-and-carry: **negative-funding regimes** (the level can flip negative for weeks — the 2026 46-day streak; you must exit, not hold through) · **perp-short liquidation** on a sharp rally · **ADL** force-closing the short (Part 6.2) · **fees & slippage** eating a thin edge · **capacity** (funding compresses as you scale) · **basis stress** on the spot-vs-perp leg · **custody/counterparty** risk from on-exchange collateral.

---

## Part 3 — Cash-and-carry, end to end (the flagship)

This mirrors the strategy-agnostic lifecycle in **§spec §6**; here we instantiate it concretely for spot+perp on one venue.

### 3.1 The setup
Long `q` spot + short `q` perp, **same venue, same asset**, equal base quantity. Net delta ≈ 0. Capital ≈ `N(1 + 1/L_perp)` — at 3× perp leverage, ~1.33·N (the spot leg is fully funded; only the perp posts margin).

### 3.2 Delta-neutrality, the easy case
Because both legs are on the same venue in the same asset, and both are linear, equal base quantity cancels delta directly — there's **no inter-venue basis** to model (that complication belongs to cross-exchange, Part 5). The only residual is spot-vs-perp basis drift, which mean-reverts as the perp tethers to spot.

### 3.3 Break-even — and why spot fees ≠ perp fees
Compute round-trip cost from **programmatically-fetched, per-leg fees**, never hardcoded constants (§spec C1). The correction that bites here: **spot taker fees are materially higher than perp** (~0.075–0.10% vs perp's ~0.05%). Do **not** reuse the ~0.29% perp round-trip figure for cash-and-carry — the spot leg costs more, and you must fetch it separately from the **product-specific** endpoint: Binance **spot** `/api/v3/account/commission`, Binance **perp** `/fapi/v1/commissionRate` (futures-only — don't call it for the spot leg), Bybit `/v5/account/fee-rate` with `category=spot|linear`. (Ignore any hardcoded "raised in May 2026" fee lore — that timing was never real, §spec C1.)

### 3.4 The entry signal
Enter when expected forward funding over the planned hold clears **cost + hurdle**. Two corrections make this honest:
- **Discrete-settlement capture** (1.7): enumerate the real settlement timestamps in the hold, sum expected per-settlement rates.
- **Drift-based min-edge:** don't use a flat "2× break-even." Derive the entry multiplier from the **empirical distribution of `(realized_settlement_rate − entry_estimate)`** per symbol/venue (cover, say, the p75–p90 adverse drift). The estimate at entry is not what settles; size the margin of safety to the measured slippage between them (§spec C3).

### 3.5 Sizing for survival
- **Use the live tier MMR.** Maintenance margin is **tiered**, not a constant 0.5%. Fetch Binance's `leverageBracket` at runtime (the table isn't otherwise readable) and drive liq-distance from the live tier for your current notional — a tier crossing silently raises your margin requirement (§spec C4).
- **Budget for ADL.** Size so that a forced-close of the short, and the *reacquisition slippage* to re-hedge the resulting naked spot leg, are survivable (Part 6.2, §spec P3/X2).
- **Report on bankroll,** effective leverage 2–3× on BTC, 1.5–2× on alts.

### 3.6 Execution — two legs, and the failure that matters
Place both legs as **parallel marketable-IOC limit orders** (never raw `MARKET` on Binance — it has no price cap; use LIMIT+IOC). The hard part is **one-leg failure**: one leg fills, the other doesn't. The lesson that the legacy tutorial had right and the first spec draft *lost*:

- Submit each order **exactly once** with a deterministic client ID; **never blind-resubmit** on a timeout. A timed-out IOC may have *filled* — a filled order is no longer an "open order," so you must resolve it by **querying its client ID** (`OrderByClientID`) and the fills history, not by listing open orders (§spec §5.3, X1).
- Order/fill/position truth lives in **one single-writer component (the OMS) with a durable write-ahead journal**. Fsyncing the client ID before submission is **necessary but not sufficient**: what makes a crash mid-flight recoverable is the **`DispatchAuthorized` state machine** — a durable "a single `Place` is imminent" boundary that forces recovery to *query* venue truth by client ID rather than assume, resubmit, or infer "not sent" (§spec §5.3/§3.2). (This is the correction §3.9's crash variant already teaches — ID durability *alone* is not the mechanism.)
- When a leg is stuck, **classify the residual first**: if it is **determined** (sign *and* size fixed — no counter-leg order still has executable quantity) restore the **net-delta invariant** with an explicit `ExposureReduction`; if it is **ambiguous** (a counter-leg order is still live / partial / `SubmissionUnknown`) **quiesce it to finality *before* any compensating trade** — acting on the observed delta would be wrong in some reachable state (§spec §6.7). Then escalate (Part 6.5).

### 3.7 Monitoring — including the exit nobody wants to code
Per open trade, on **private account events** (not just polling): liq distance, basis divergence, and two *distinct* funding numbers — **audit** (did the settled rate match the venue's realized rate?) and **attribution** (did it match my signal?). Keep them separate or you'll false-alarm on basis. Detect **interval flips** (1.7). And code the **negative-funding-regime exit** as a **hysteresis band, not a sign flip**: majors dip briefly negative constantly and each false exit is a full round trip, so stand down only when the forward estimate stays adverse past a band materially worse than entry — don't hold a persistently-negative carry, but don't whipsaw on noise either (§spec §2.4).

### 3.8 Closing
Close with the **same parallel-IOC pattern** — never an optimistic post-only that may not fill. Decompose realized PnL **per leg** from actual fills. Compute realized funding for the funding-bearing perp leg from the **venue's immutable funding-ledger amount** — that is the accounting truth; the `rate × settlement-mark × position` reconstruction is the *audit cross-check*, not the booked number (§spec §5.1). In cash-and-carry the spot leg earns *no* funding, so value it from its own entry/exit fills. (The compute-funding-from-each-leg's-own-mark subtlety applies only when there are *two* funding legs — a cross-exchange concern, §spec C7.) Persist everything to the durable journal.

### 3.9 A complete worked trade — numbers, then a crash `[→ R1-F4, R2-tutorial]`

Concrete beats abstract. One full cash-and-carry trade at VIP0 taker fees, BTC flat at $100,000 for a clean ledger, `+0.015%/8h` funding, a 20-day hold, 3× perp leverage.

**Capital.** `N = $10,000` notional. Spot leg fully funded = $10,000; perp initial margin = `N/3 ≈ $3,333` in the **futures wallet** → **leg-capital *floor* `C_legs ≈ $13,333` (= 1.33·N)**. But the **reporting denominator is `C_allocated`** (§spec §2.2), which adds **pre-funded rescue margin** (§spec §7.1 — say ≈ $3,333, a conservative de-lever-under-stress buffer for the isolated perp) + a **fee/slippage reserve** (~$40) + **committed float** → **`C_allocated ≈ $16.7K`**. Report on `C_allocated`, never on the `C_legs` floor.

| Event | Cash flow | Running note |
|---|---|---|
| Buy 0.1 BTC spot @ $100k | −$10,000, −$10.00 fee (0.10% taker) | spot leg on, in the spot wallet |
| Short 0.1 BTC perp @ $100k | −$3,333 margin, −$5.00 fee (0.05% taker) | net delta ≈ 0; isolated margin |
| Entry slippage (both legs) | −$4.00 (≈0.02%/leg) | **cost so far ≈ $19** |
| 60 funding settlements (20d × 3/day) @ 0.015% | **+$90.00** (0.00015 × $10,000 × 60) | the edge; the short *receives* positive funding |
| Sell 0.1 BTC spot @ $100k | +$10,000, −$10.00 fee | spot leg off |
| Buy-to-close 0.1 BTC perp @ $100k | +$3,333 margin back, −$5.00 fee | hedge off |
| Exit slippage (both legs) | −$4.00 | **round-trip cost ≈ $38 (0.38% of notional)** |
| **Net** | **+$90 − $38 ≈ +$52** | over 20 days on `C_allocated ≈ $16.7K` |

**Read it honestly.** +$52 / **$16.7K `C_allocated`** = **0.31% in 20 days ≈ ~5.7% annualized *while in position***. On the `C_legs` floor it would *look* like ~7.1% — but that overstates the return by ignoring rescue margin + reserves, the exact denominator error §spec §2.2 forbids. Cost alone ($38) took **~25 settlements (~8.4 days)** to cover before a cent of profit. At **60% time-in-position** the bankroll nets **~3.4%** — **below** the 4.6% T-bill hurdle (§spec §2.1). This is the flagship on a *good* major regime; on baseline `+0.01%/8h` it barely clears costs. The edge is real and thin — which is why the forecast, the hysteresis exit, and utilization all matter (§spec §2.4), and why the durable value is *not bleeding it back through bugs*.

**Now the failure that matters — one leg fills, the other times out:**

| t | What happens | The invariant that saves you |
|---|---|---|
| t₀ | OMS mints `cid-spot`, `cid-perp`; **fsyncs `TradePlanned` to the journal** | client IDs are durable *before* any network write |
| t₁ | Submits both IOC orders — **once each**, each `Place` preceded by an fsynced `DispatchAuthorized` commit | no transport retry, ever; recovery will *query*, not assume, whether a `Place` ran |
| t₂ | Spot **fills** (0.1 BTC); ExecutionID + fill-ledger + journal in one txn | **exposure is now OPEN and unhedged** |
| t₃ | Perp submit **times out (5xx)** → `cid-perp` = `SubmissionUnknown` | a timed-out IOC *may have filled* — do **not** resubmit |
| t₄ | Net-delta breach → handler classifies the residual **ambiguous** (`cid-perp`=`SubmissionUnknown`, so the exposure *sign* is unknown) → it **resolves `cid-perp` to finality first**, issuing **no** compensating trade yet | a spot sell *now* leaves a naked short if the perp actually filled; the pre-armed protection bounds the directional window during resolution (§spec §6.7) |
| t₅ | `OrderByClientID(cid-perp)` returns **Filled** — the response was just lost; hedge is actually complete, residual ≈ 0, nothing to do | a blind resubmit *or* a blind spot-sell here would have **doubled the short / gone naked short** |
| t₅′ | *(alt)* query returns stable **Rejected/not-found** → residual now **determined** (naked long spot) → `ExposureReduction`: inventory-capped **spot sell** of 0.1 BTC to flatten | resolve-then-act: the compensating trade waits for the determined sign; `reduceOnly` doesn't exist for spot — a plain sell, capped by OMS inventory |

**Crash variant:** kill the process between t₂ and t₃. On reboot, **reconcile-only boot** reads the journal and, for every order at **`DispatchAuthorized` or beyond**, *queries* venue truth by client ID (`spot=Filled`; resolve `cid-perp`) — **never resubmitting, never inferring "not sent"** — rebuilds state, asserts net-delta **and the §4.1 account posture**, then resumes. What makes this safe is the **state machine, not ID durability alone**: fsyncing the client ID before the send is necessary but not sufficient — the `DispatchAuthorized` boundary is what forces recovery to *query rather than assume* whether the single `Place` ran (§spec §5.3, §7.5).

---

## Part 4 — The strategy-agnostic foundation

This is the layer the legacy tutorial didn't name explicitly. It's what turns "a cash-and-carry bot" into "a platform that runs strategies." Full architecture in **§spec §3**.

### 4.1 Foundation vs strategy
A **strategy** is *pure signal + sizing policy*: it reads a normalized `MarketView` and emits `Intent`s (open/close, with legs and a target quantity). It **never** calls a venue API, touches risk plumbing, or manages orders. The **foundation** runs every Intent through the *same* risk → OMS → monitor pipeline, no matter which strategy emitted it (§spec §3.1, §3.3). Cash-and-carry and cross-exchange are two implementations of one `Strategy` interface; you switch or compose them by config.

### 4.2 The VenueAdapter seam — where quirks go to be contained
The legacy tutorial's claim that "generalizing to a third venue is straightforward" was untrue *as written* — venue quirks (WS routing, delta semantics, position modes) leaked everywhere. The fix is one **adapter contract** that every venue implements and the core depends on (§spec §4). A taste of the quirks it must hide (verified 2026-07-18, with the safety-kernel endpoints re-verified 2026-07-19):

- **Binance WS routing changed:** the legacy `/ws/` path was **decommissioned 2026-04-23**. Funding → `/market`, book → `/public`, user-data → `/private`. An unmigrated connection silently receives **nothing** for funding — this is a "your feed is dead and you won't know" bug (§spec §4).
- **Bybit dynamic funding interval** (1.7) and the **new** `fundingIntervalHour`/`fundingCap` ticker fields (WS 2025-10-16, REST 2025-10-23) — pin clients ≥ Oct-2025 or they won't parse.
- **Position-mode close semantics:** Binance Hedge Mode *rejects* `reduceOnly` (err -2022); one-way uses it. Bybit uses `positionIdx`.
- **Rate limits differ:** Binance `X-MBX-USED-WEIGHT` → 429 → 418 IP ban; Bybit has headers **and** an **unheadered IP gateway ban (~600 req/5s, retCode 10018)** that header-watching alone won't catch.

### 4.3 The trade lifecycle, strategy-agnostic
Ingest → normalize → signal → **pre-trade risk re-check** → sizing → **plan+journal** → execute-once → one-leg-failure handling → monitor → close (§spec §6). The two hard-won invariants: **submit-once + query-by-client-ID** (never resubmit into an ambiguous timeout), and **one durable single-writer of order/position truth** so a crash is always recoverable.

### 4.4 State, clock, money, and backtest parity
- **State** is an in-memory, single-writer cache handing *immutable* snapshots to readers — no shared-mutable races (§spec §3.4).
- **Clock** is injected, never `time.Now()`. Live wiring uses a wall-clock; the backtester uses a `TestClock` that *drives* time. Parity comes from a **single dispatcher applying every input in one total order** (a monotonic ingress sequence), so live and replay behave identically — not from "byte-identical timestamps," which is a mirage the moment two events share a millisecond (§spec §3.5).
- **Money** is scaled-integer `fixedpoint`, dimensional (`Money`, `Quantity`, `Price`, `Rate`), with big-int intermediates — a `float64` in a money path fails the build (§spec §3.6).
- **Durability** is mandatory, not "in-memory, or Redis if you want crash-recovery." The append-only, hash-chained journal *is* the source of truth; the in-memory state is a cache rebuilt from it + venue truth at boot (§spec §7.5).

---

## Part 5 — Cross-exchange spread as strategy #2 (scoped)

Everything from the legacy Part 2, demoted and re-pitched with the P1 bar. Build it on the *same* foundation (§spec §2.3).

### 5.1 When the spread is actually real
Not on majors (Part 2.3). The niche: **alt pairs** where one venue's funding dislocates, and **dislocation windows** (a venue-local spike). Set the edge bar from **measured spread persistence** — how long the spread survives after you observe it — not the headline number, which mean-reverts before you've earned the round-trip.

### 5.2 The extra risks you take on
- **Inter-venue basis:** now you *do* have two different marks; the hedge is only as good as their correlation under stress.
- **A second counterparty** for the hedge, doubling custody/exchange risk.
- **Inventory drift:** PnL accrues on different venues; you must rebalance. Note the **OES correction**: there is **no single off-exchange rail spanning Binance + Bybit** (Binance → Ceffu MirrorX only; Bybit → Copper/Fireblocks only). "Shift capital in seconds without an on-chain hop" is false for this pair — crossing pools needs two custodians. And OES protects *custody*, not *margin*: Oct-2025 liquidated OES users too (§spec S1).

### 5.3 Equal-base neutrality, and why inverse breaks it
Equal base quantity is delta-neutral **for linear perps only**. For **inverse (COIN-M)** perps the payoff is `1/P` with nonzero gamma, so equal-base is *not* neutral. That's why linear-only is an explicit scope boundary and the adapter carries `ContractType` (§spec C6). When you compute residual delta for equal-*USD* (vs equal-*base*) sizing, it's `≈ −N·basis/P²` — a cross-exchange concern, kept out of the cash-and-carry path (§spec §2.3).

### 5.4 Execution deltas across venues
The one-leg-failure machinery is identical (that's the payoff of the foundation), but position-mode and close semantics differ per venue (4.2) — the adapter encodes them so the strategy doesn't have to.

---

## Part 6 — Risk, custody & security (the real-capital bar)

Promoted from a footnote to a first-order part. This is where real money is lost. Normative detail in **§spec §7**.

### 6.1 Sizing for survival — one policy
A single liq-distance policy, reconciled across the codebase: open ≥30% / alert <25% / rebalance <20% / emergency <15% for BTC, tighter for alts (§spec §5.2, C5). Use the **post-Oct-2025** numbers (25–30% BTC / 40–50% alts buffers) — never the older, looser 16%. Report on bankroll.

### 6.2 Liquidation and ADL — the first-order tail
- **Liquidation:** the perp short can be liquidated on a sharp rally. **Liq distance moves with the *mark price*, not only account events** — so recompute it on **every fresh mark update** *and* every margin/position change, and treat a **stale mark feed** as itself a reason to block new openings and escalate (§spec §6/§7.1). Keep **pre-funded rescue margin already at the venue**. Under the isolated posture the rescue is an **active `add-margin` API top-up** from margin staged *outside* the position (§spec §7.1), **not** a passive wallet buffer (the isolated liq engine can't see free wallet balance) — and reactive spot→futures transfers are unavailable exactly during the stress that needs them, which is why the float is pre-staged and the top-up carries a reserved rate-limit budget with a **`PackageDelever` fallback if it's slow** — a **perp-short-first, both-leg** reduction that never transits a naked short. Note it is deliberately **not** an `ExposureReduction`: the package is still neutral, so reducing a single leg would *create* a naked leg during the rally (§spec §7.1/§5.3).
- **ADL (auto-deleveraging) is not a footnote — it's a realized hedge-failure mode.** In Oct-2025 exchanges force-closed *profitable short* legs, leaving desks **naked long into a still-falling market** (§spec P3/X2). Cash-and-carry's advantage is narrow-but-real: the **spot leg isn't ADL'd or seized** (under the §spec §4.1 posture), so you never lose the whole position — but a force-closed short still leaves directional exposure that must be **detected from a private ADL event and reduced immediately**. And here's the subtlety the first spec draft missed: to flatten a naked **spot** leg you place an ordinary **sell** — `reduceOnly` does not exist for spot on either venue — so the emergency path uses a distinct `ExposureReduction` command whose type follows the *instrument's own position*: to flatten a naked **spot** long it's an inventory-capped **spot sell**, and to re-hedge a perp that was ADL'd **to zero** it's an **ordinary *non*-reduce-only short** — a reduce-only order would be *rejected*, since reduce-only can't *open* a position; `reduceOnly` is used only to flatten an *existing* perp position. All exempt from "stop-opening" (§spec §5.3).

### 6.3 When a venue goes down, or basis blows up
- **Venue outage:** cap exposure per venue (≤20–30% bankroll) so one venue freezing can't sink you.
- **Basis blowup:** watch/alert/exit on basis divergence. Note the **USDe correction**: the Oct-10 low was **~$0.65** and a **local-venue dislocation, not a protocol depeg** — Ethena processed >$2B redemptions with zero downtime (§spec §1, S1). Don't over-fit to a "depeg" that wasn't one; do keep USDT/USDC-only margin so a collateral wobble can't cascade.

### 6.4 Custody & settlement — on-exchange for MVP
On-exchange, deliberately, at small size, in **isolated per-venue subaccounts** with hard collateral caps (§spec §7.2). OES is the documented scale-up path (5.2), gated on size — not skipped, not implied away. The spot leg is held on-exchange under withdrawal-disabled keys (6.5).

### 6.5 API keys, kill-switch modes, and the escalation ladder
- **Keys:** trade-enabled keys carry **no withdrawal permission**; separate read-only monitoring keys; stored in Vault/KMS, per-subaccount, IP-allowlisted, rotated. And know the **real blast radius**: a compromised trade key from an allowed host can *bleed the whole subaccount* via deliberately adverse trades even with withdrawals off — so isolate the signer, cap collateral per subaccount, and run an independent read-only watcher (§spec §7.3, X10). And the **operator control plane is itself privileged**: the API that trips the kill switch, edits emergency policy, or fences a venue key needs authentication + authorization, replay/CSRF protection, a private-network boundary (never public), and per-actor audit — a stolen dashboard session must not be able to do what a stolen key can't (§spec §7.3).
- **Kill switch is not one command.** It has **modes**: (a) stop-opening, (b) flatten-if-feasible, (c) hard-halt-and-alert — plus an **out-of-band trip** (disable trade keys / venue-side controls) for when the process is wedged (§spec A5).
- **The escalation ladder is automated and pre-authorized.** A stuck naked residual at 03:00 cannot wait for a human. The approval kernel (solo-honest by default — time-delayed self-approval + an out-of-band confirm; two-person four-eyes only if you're actually staffed) **arms the emergency policy beforehand**; once exposure exists, pre-authorized loss bounds **execute automatically**: stop openings → reconcile ∥ bounded `ExposureReduction` (concurrently — but **only for a *determined* residual; an *ambiguous* one, where a counter-leg order still has executable quantity, is quiesced to finality *first***, §spec §6.7) → pre-authorized panic/venue-native stop (**which also does not bypass that finality barrier for an ambiguous residual**) → revoke key. Alerting runs in parallel, never as the terminal action. **And the flatten authority must live *outside* the bot's failure domain:** if the host and its local journal die, the OMS can't act and a read-only watcher can only watch — so protection must be **pre-placed venue-native orders (or a replicated `EmergencyPlan` on an independent actuator), established before exposure opens** (concretely: Bybit can attach TP/SL atomically to the entry, but **Binance USDⓈ-M cannot** — its conditional orders are separate placements — so on Binance the independent actuator is *mandatory*, not optional; §spec §4/§7.4); revoking the key is the *last* step, never the flatten itself. Beware too: pulling the IP allowlist can cut the very connectivity you need to flatten — prefer venue-native stops, sequence any connectivity kill last (§spec §7.4, S4).

### 6.6 Inventory drift & treasury ops
Track per-venue balance skew; rebalance within a rail's pool where possible (5.2). The treasury context owns balances, the concentration caps, and the rescue-margin float.

---

## Part 7 — The architecture you build (Go, DDD/hexagonal)

Maps the legacy Part 5 onto the house style. Full layout in **§spec §3, §11**.

### 7.1 Bounded-context layout
`internal/{boundary,engine,platform}`: **contexts** (`marketdata`, `strategy`, `risk`, `oms`, `treasury`, `audit`) each split domain/application/infrastructure; **engine** subsystems (`venue` adapters, `state` cache, `backtest`); a **platform** shared kernel (`fixedpoint`, `clock`, `events`, `outbox`, `foureyes`, …). The write-side — order/fill/position truth — is **one single-writer `oms` context** with a durable journal; audit and reconcile are *projections* of it, never competing owners (§spec §3.2). Durable state is mandatory (§spec A3).

### 7.2 Reconcile-only boot
On every cold start / kill-switch toggle / extended feed gap: **disable trading and rebuild state from venue truth** (positions, open orders by client ID, fills, funding ledger) joined to the journal. Known journaled client IDs auto-recover; only genuinely foreign orders or unexplained deltas alert for a human — and the system **alerts, never auto-corrects** (§spec §7.5).

### 7.3 Latency & deployment
This is **not HFT** — millisecond budgets, not microseconds. Single region to start (Tokyo `ap-northeast-1` is fine for both venues); **"single process" means the trading engine only — the operator/control process and a minimal actuator daemon are *separate required* failure domains (§spec §11)**. The service-split seam (splitting the engine's own contexts / NATS) is documented for later, not built (§spec §3.7).

### 7.4 Backtesting: Tardis, not Coinglass
The signal is the **live funding *estimate*** (per-second mark-derived), not the settled rate — replaying settled rates is lookahead and will lie to you. **Coinglass cannot back this** (it serves funding OHLC ≠ the live per-second `r`). **Tardis.dev can** (it records Binance `markPrice@1s` `r` and Bybit `tickers`), so it's the buy — and **self-capture both streams from day one** for parity + recent data (§spec §8, B1). Add a spread/level **persistence model**, an **adverse-selection fill model** (IOC fills when the book moves against you), and **survivorship** (include delisted/force-settled alts) or your backtest will flatter you.

### 7.5 Build vs buy — decide it, don't skip it
The honest posture, stated straight: **the edge is thin and commoditized, so the durable value is operational excellence in the plumbing you build.** That is the *opposite* of the usual "buy the plumbing, build the edge" slogan — here you deliberately *build* the plumbing (from scratch in Go, for control and determinism) and the "edge" is largely *not losing it back through bugs*. Reference (never depend on) NautilusTrader (engine + backtest-parity patterns), Hummingbot's `v2_funding_rate_arb.py` (and its bug #7295 to avoid), CCXT (venue-quirk cheat-sheet — but its unified funding is inconsistent, so read raw per-venue fields), Tardis (data). Build from scratch in Go, OSS as reference only (§spec P4, §11).

### 7.6 The testing ladder
Unit → **property-based** (order-book invariants, delta idempotence, exactly-once funding *and* fills) → **exchange-testnet integration** (both venues have testnets — exercise `OrderByClientID` recovery) → **chaos/fault-injection** (partial fills, post-cancel late fills, WS gaps, 429/418, **process-kill mid-IOC**, ADL events) → **paper-trading acceptance gate with explicit pass/fail** (§spec §9, B2). Don't jump from backtest to real capital.

### 7.7 Iterative rollout
Paper → $1K → $10K → scale, canarying each build on a small-notional subaccount for 1–3 funding cycles. The paper gate has hard numbers now (≥14 days, **≥30 funding settlements on each funding-bearing perp leg** — the spot leg has none, so it's gated on synchronized spot↔perp cycles instead — 100% reconciliation, bounded per-fault recovery times, **<5 s max *determined*-unhedged exposure — the *ambiguous* window (a counter-leg still has executable quantity) is bounded separately by a <30 s quiesce-to-finality RTO, its directional risk covered by the sized naked-leg-gap budget + pre-armed protection, not scored as a <5 s violation**) — meet them before promotion (§spec §9).

---

## Part 8 — Common pitfalls (field notes)
Stale data used for a decision (per-data-class freshness: sub-second for books, longer for funding metadata) · one-leg orphan bugs (submit-once + query-by-client-ID) · funding-boundary timing (guard ±60–120s on **server** time) · cap-bound spreads that look real but can't widen · **ADL** (size for it) · withdrawal/transfer queues during stress (pre-fund rescue) · rate-limit bans incl. Bybit's unheadered IP ban · clock skew · **tiered-margin transitions** raising MMR silently (drive off `leverageBracket`) · weekend/holiday liquidity · fee schedule changes (fetch, don't hardcode) · alt delistings/force-settlements.

## Part 9 — Pre-ship checklist
**Account & margin-mode posture asserted at boot *and* before each trade** (isolated margin, single-asset/segregated, spot-borrow/hedge off, **one-way position mode + leverage pinned**, auto-add-margin off — the safety premise, §spec §4.1) · durable append-only + hash-chained journal live (**WAL + `synchronous=FULL`**, local fs) · reconcile-only boot verified against a real crash · exactly-once for **fills and funding** (DB uniqueness, not just a hash) · submit-once execution with **`DispatchAuthorized`→query-only** client-ID recovery · **kill-switch modes** + pre-authorized escalation ladder + external watchdog + **host-independent flatten coverage** (survives simultaneous host+journal loss; a **working per-venue `Fence`**; key revocation is the last step, not the flatten) · trade keys withdrawal-disabled, **read-only watcher running, order-signer isolated** ($10K+, *or* $1K where it is the venue's fence) · **operator control plane authenticated/authorized, private-network-bound, per-actor audited** · per-venue concentration caps enforced (rescue-margin float included) · live-tier MMR sizing + **margin top-up protocol** · negative-funding-regime exit coded · backtester on Tardis + self-capture with the fidelity models · paper acceptance gate passed with evidence · per-event USD tax records emitted.

## Part 10 — Beyond the MVP
Cross-pair funding baskets · LST-collateral + short stacking (mind the USDe lesson) · quarterly-vs-perp basis · funding-rate prediction models · additional venues behind the same adapter · eventually, a service split (§spec §3.7). Cash-and-carry is the *flagship* (Part 3), cross-exchange is *strategy #2* (Part 5) — "beyond" is what comes after those.

## Closing thoughts
This business is **mechanical, not easy**. On majors the net edge over on-chain T-bills is single digits and you're front-run by better-capitalized desks; the durable money is in **operational excellence** (never losing a leg, never mis-reconciling, never getting liquidated by a tier transition you didn't model) and in the **niches** (alts, dislocations, collateral-stacking). Build the boring infrastructure — the durable journal, the reconcile-only boot, the pre-authorized kill switch — *first*. The edge is real but thin; the way you keep it is by not bleeding it back through bugs.
