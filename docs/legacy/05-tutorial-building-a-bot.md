# Building a Cross-Exchange Funding Rate Arbitrage Bot — A Tutorial for Engineers

## Who this is for

You are an experienced software engineer. You can build distributed systems, you understand WebSockets, you can reason about race conditions and latency budgets. But you have never traded a perpetual swap, you don't know what "funding rate" means, and you'd like to actually understand — not just copy code — why this strategy works, what your bot needs to do, and what can go wrong.

This tutorial assumes zero finance background. By the end, you will:

- Understand the market structure (perpetual swaps, funding, mark prices, liquidations) well enough to reason about edge cases
- Understand the specific trade we're building: **cross-exchange funding rate arbitrage** — long one venue's perp, short another venue's perp on the same asset, net-collecting funding payments
- Know exactly what your bot does at each stage: what it listens to, what it computes, when it opens positions, what it monitors, and when it closes
- Know the risk controls that separate "boring monthly carry" from "ruinous tail"
- Have an architectural blueprint for what to build

We'll use Binance and Bybit as our running example because they're both major centralized exchanges (CEXes), they both have well-documented WebSocket APIs, and their funding-rate formulas are nearly identical. Once you understand this pair, generalizing to a third or fourth venue is straightforward.

This is a long read. It's structured so each part builds on the previous. Resist the urge to skip ahead.

---

# Part 1: A 10-Minute Crash Course in the Market

You can't build a bot for a market you don't understand. Read this even if you think you can.

## 1.1 Spot, futures, and perpetuals

In any financial market, three flavors of "buying X" exist:

- **Spot.** You buy the asset itself. If you "buy 1 BTC spot," you now own 1 BTC. Settlement is immediate (or near-instant on-chain). You can hold it forever, and your profit/loss equals the change in price.

- **Futures (dated).** A contract to exchange the asset at a fixed future date. "I agree to buy 1 BTC at $70,000 on December 31, 2026." If BTC ends up at $80,000 on that date, you profit $10,000 (you bought at $70K what's now worth $80K). Futures _expire_ and _settle_ on a schedule (monthly, quarterly).

- **Perpetual swap (perp).** A futures contract with **no expiry date**. You can hold it forever. This is the dominant instrument in crypto: BitMEX invented it in 2016, and today perps account for over 75% of all crypto trading volume.

The perp is a clever financial-engineering trick. A traditional future converges to spot at expiry — there's a fixed terminal date when the contract becomes the spot asset, so its price has nowhere to drift. A perp has no expiry, so without some other mechanism, the perp's price could drift arbitrarily far from spot. The perp price is set by traders bidding and offering on the perp itself; the spot price is set by traders bidding and offering on the actual asset. There's no automatic anchor between them.

That mechanism — the anchor — is the **funding rate**.

## 1.2 The funding rate, in plain English

A perp tracks an underlying asset (say, BTC). The exchange wants the perp's price to stay close to the BTC spot price. To accomplish this, every few hours, the exchange forces a cash transfer between the people who are **long** the perp and the people who are **short** the perp:

- **If the perp is trading above spot** (perp at $70,500 vs spot at $70,000): too many people want to be long. To discourage being long, the exchange makes longs pay shorts on a schedule. This is "positive funding."

- **If the perp is trading below spot** (perp at $69,500 vs spot at $70,000): too many people want to be short. To discourage being short, the exchange makes shorts pay longs. This is "negative funding."

The size of the payment is set by a formula based on how far perp has drifted from spot (the "premium"). The bigger the premium, the bigger the funding payment.

This is genuinely all the funding rate is. It is a peer-to-peer cash transfer between long holders and short holders, paid every funding cycle (typically every 8 hours on Binance and Bybit), sized to nudge the perp's price toward spot. The exchange does not pay or receive funding — it just orchestrates the transfer.

## 1.3 The terminology you need to know

A few terms get used a lot. Lock these in now:

- **Long** = holding a position that profits if price goes up. (You "bought.")
- **Short** = holding a position that profits if price goes down. (You "sold first, will buy back later.")
- **Notional** = the dollar size of a position. If you're long 0.5 BTC at $70,000, your notional is $35,000.
- **Margin** = the collateral you've posted with the exchange to back your position. With leverage, your margin is smaller than your notional. If you post $7,000 of margin to back a $35,000 position, you're at 5x leverage.
- **Leverage** = notional ÷ margin. 5x means $1 of margin supports $5 of notional.
- **Liquidation** = if the price moves against your position far enough that your margin is exhausted, the exchange forcibly closes your position. You lose your margin.
- **Funding rate (F)** = the per-cycle interest-like rate. Quoted as a percentage. F = +0.01% per 8h means longs pay shorts 0.01% of position value, every 8 hours.
- **Funding payment** = F × position value at the funding timestamp = F × |base_quantity| × mark_price_at_settlement. (NOT the entry notional — if mark moves, the funding base moves with it. The §1.5 / §2.2 / §2.5 dollar examples assume mark stays near entry for simplicity.)
- **Funding interval** = how often payments happen. 8h on Binance/Bybit/OKX defaults; 1h on Hyperliquid; 4h or 1h on selected pairs.

## 1.4 The funding formula, simplified

The exact formulas live in [02-math-and-mechanics.md](./02-math-and-mechanics.md). For this tutorial, here's what you need:

The funding rate F is approximately:

> F ≈ (Perp price − Spot price) / Spot price + a small interest-rate term

Both Binance and Bybit use an internal **damper** of ±0.05% so that when the premium and interest-rate terms are close, F equals the small interest term (0.01% per 8h ≈ 10.95% simple APR — or 11.58% APY if every payment is reinvested at the same rate). When the premium is _outside_ the damper band, F tracks premium with a 0.05 percentage-point offset (formally: `F = P + clamp(I − P, −0.05%, +0.05%)`, so for `P = 0.10%` and `I = 0.01%`, F = 0.10% − 0.05% = 0.05%, not 0.10%). The damper prevents tiny noise from causing wild funding oscillations.

Separately — and this is a distinct parameter, often confused with the damper — each contract has a **per-cycle cap and floor** that bounds the final F. **Caps are per-symbol, not universal.** Binance documents BTCUSDT at ±0.3% per 8h. Lower-tier and altcoin contracts can be ±2% or use risk-tier-derived caps such as ±0.75 × maintenance margin ratio. Bybit's `tickers.<symbol>` WebSocket exposes a live `fundingCap` for each pair (currently 0.5% on BTCUSDT/ETHUSDT per the live API; Bybit's published cap formula uses a coefficient adjustable in `[0.5, 1.0]` and the venue reserves the right to adjust without announcement). **Always read the live cap from the venue's API or contract spec before sizing a trade — do not assume ±2%, and don't hardcode the per-symbol number either.**

## 1.5 What a funding rate means in dollars

This is critical. People who don't think in dollars get burned.

Suppose you hold a $100,000 short perp position on Binance BTCUSDT, and the funding rate is +0.01% per 8h. Three times a day, you receive:

> 0.01% × $100,000 = $10

Per day: $30. Per year: $30 × 365 = $10,950 → that's 10.95% APR on $100K notional.

But wait — you didn't post $100K. With 5x leverage, you posted $20K of margin. So the return on your margin is:

> $10,950 ÷ $20,000 = 54.75% APR on margin

That's the seductive part. Annualizing apparent yield by leverage is also how people end up liquidated. Don't fall in love with the leverage-adjusted number; the strategy's actual return is on the _capital at risk_ (your full margin, plus any safety buffers).

Convert any funding rate to APR with this formula:

> APR = F × (8760 ÷ funding_interval_in_hours)

So 0.01% per 8h = 0.0001 × 1095 = 0.1095 = 10.95% APR.
0.005% per hour (Hyperliquid) = 0.00005 × 8760 = 0.438 = 43.8% APR.

## 1.6 Mark price vs index price vs last price

Three prices coexist for a perp, and confusing them will hurt:

- **Last price** = the price of the most recent trade on the perp. This is what TradingView shows. Volatile, can be wicked by manipulation.
- **Index price** = a weighted average of the spot price across reference exchanges (Coinbase, Kraken, Binance spot, etc.). This is the "true" external benchmark.
- **Mark price** = a smoothed price that combines the index price with the perp's premium. **The mark price is what triggers liquidations and is used for unrealized PnL.** It is deliberately resistant to short-term manipulation of the last price.

Why this matters: in a flash crash, the perp's last price might wick down 10% momentarily, but the mark price (anchored to spot) might only move 2%. If liquidations were triggered by last price, every flash wick would liquidate longs. By using mark price, the exchange protects users from manipulation. As an arbitrageur, you should care about mark price for liquidation distance and for the live funding-rate estimate (§3.2 explains why both venues stream a single continuously-updated estimate, not a separate predicted/current pair).

That's it for finance. We have everything we need.

---

# Part 2: The Trade — Cross-Exchange Funding Rate Arbitrage

## 2.1 The core observation

The same instrument — say, the BTCUSDT perpetual — exists on Binance, on Bybit, on OKX, on Hyperliquid, on dozens of venues. Each venue independently computes its own funding rate from its own orderbook and its own index price. **At any given moment, the funding rates for the same asset on different venues are different.**

This shouldn't be obvious or surprising; markets are not perfectly arbitraged. Asian retail traders concentrate on certain venues; American institutions on others; algorithmic market makers aren't connected to every exchange equally. The result is that at 12:00 UTC today, Binance's BTCUSDT perp might have a funding rate of +0.015% per 8h while Bybit's BTCUSDT perp has +0.005% per 8h.

When this happens, there is an arbitrage available.

## 2.2 The setup

The trade is symmetric and surprisingly clean:

1. **Go long the perp on the venue with the LOWER funding rate.** (You'll pay the smaller positive funding, or — even better — receive negative funding.)
2. **Go short the perp on the venue with the HIGHER funding rate.** (You'll receive the larger positive funding.)
3. **Size both legs to the same base quantity** (e.g., 1.5 BTC on both venues). Equal USD notional is a close approximation when the two venues' prices are within a few basis points — for BTC/ETH this is usually fine — but it leaves a small residual delta from the inter-venue basis. As you scale capital up or trade less liquid markets, prefer equal-base-quantity sizing so the hedge is exact. (We'll work the math in §3.8.)
4. Hold both positions until the spread between the two funding rates compresses.

Example with our numbers above:

- Binance BTCUSDT perp: F = +0.015% (longs pay 0.015%, shorts receive 0.015%)
- Bybit BTCUSDT perp: F = +0.005% (longs pay 0.005%, shorts receive 0.005%)
- Trade: long Bybit BTCUSDT $100K notional, short Binance BTCUSDT $100K notional.
- Per 8h cycle:
  - Bybit long leg: pays 0.005% × $100K = $5 (you pay this)
  - Binance short leg: receives 0.015% × $100K = $15 (you receive this)
  - Net: +$10 per 8h cycle, or $30/day on $100K notional.
- Annualized: 10.95% × 100K notional = ~$10,950/year. (On the margin you posted, even more — but again, don't fall in love with that number.)

## 2.3 Why this is "delta-neutral"

"Delta" in derivatives jargon means "sensitivity to the underlying price." For a linear perp like BTCUSDT, delta in BTC terms = notional ÷ price. If BTC = $70,000 and you're long $100K notional on Bybit, your delta is `100,000 / 70,000 ≈ 1.43 BTC` — you make money when BTC goes up. If you simultaneously short an _equal-base-quantity_ 1.43 BTC on Binance, your delta there is −1.43 BTC, and the legs cancel exactly.

Equal-USD-notional sizing (the convenient default) gets you _approximately_ neutral when both venues' marks are similar, but it leaves a small residual base delta: `q_long − q_short ≈ −N × basis / P²` (units: BTC). The equivalent USD exposure is `≈ −N × basis / P`. Equal-base-quantity sizing is exactly neutral. We'll come back to this in §2.2 step 3 and §3.8; for the rest of the §2.3 walkthrough, assume equal base quantity.

If BTC rallies 10%, your Bybit long gains 10% × $100K = $10K (in unrealized PnL) and your Binance short loses $10K. Cancels.

If BTC crashes 10%, your Bybit long loses $10K and your Binance short gains $10K. Cancels.

You are completely indifferent to BTC's price. Your only PnL comes from:

1. The funding rate spread (the alpha)
2. Trading fees (the cost)
3. Tiny basis differences between Bybit and Binance prices (usually noise, but can blow up in stress)
4. Liquidation events (the existential risk)

That's the elegance. You're collecting an interest-rate-like yield, with no directional exposure to BTC.

## 2.4 Why this isn't free money

The trade isn't risk-free, despite being delta-neutral. Concretely:

1. **Trading fees.** Every time you open or close a position, you pay maker or taker fees. Round-trip on both legs adds up. On Binance and Bybit, taker fees are ~0.04-0.05% per side. Round trip = 4 fee events. If your funding spread is only 0.005% per 8h, you'll need many cycles before fees are paid back.

2. **Position limits and capacity.** You can't put $1B on this trade — venue per-account caps and orderbook depth limit you. At sub-$10M, capacity is not a problem; at $100M+, it is.

3. **Liquidation risk.** Each leg can be liquidated independently. If BTC moves sharply against your short leg (price spikes up), and you don't have enough margin, the exchange forcibly closes your short. Your other leg (the long) is now naked: you have +1 BTC of price exposure with no hedge. If BTC then drops, you eat the loss.

4. **Counterparty risk.** Every dollar you have on Binance is a dollar Binance owes you. If Binance becomes insolvent (FTX-style), you might lose 100% of your margin there. Your Bybit leg is now naked.

5. **Basis risk during stress.** In normal markets, Binance's BTC mark ≈ Bybit's BTC mark within a few basis points. In a flash crash, the two can diverge by tens to low hundreds of basis points (the [Oct 10-11, 2025 cascade](https://www.coingecko.com/learn/october-10-crypto-crash-explained) is the canonical recent example for inter-CEX basis dislocation on majors; see also [`sources.md`](./sources.md) for the full event citation set including FTI, insights4vc, and CoinDesk's ADL coverage). A _separate_ category of basis risk is collateral / oracle failure on a single venue — for instance, Binance's local USDe orderbook traded at $0.62 during Oct 10-11 while USDe held near $1.00 elsewhere, slashing effective margin for anyone using USDe as collateral. We unpack both in §4.6.

6. **Spread compression / flip.** The whole reason you have an opportunity is the spread between Binance and Bybit funding rates. If the spread compresses to zero (or flips sign) faster than expected, you can close at a loss after fees.

7. **Capacity-induced compression.** If everyone runs the same trade, the spread compresses, the trade becomes unprofitable, and the marginal participant takes a loss.

These risks are why the bot needs to do more than just open positions. Most of the work is monitoring and risk management.

## 2.5 What this trade looks like over a real holding period

Let's walk through a one-week trade at our example sizes.

Day 0 (open):

- Binance F = +0.015%, Bybit F = +0.005%, spread = 0.010% per 8h
- Open: long Bybit $100K notional, short Binance $100K notional
- Round-trip cost (we'll derive this in §4.1): ≈ 0.29% of notional, i.e. ~$290 on $100K — split roughly $145 at open and $145 at close, covering both legs' taker fees and expected slippage

Day 0-7 (hold for 21 funding cycles, 7 days):

- Average net funding capture per cycle: $10 (assuming spread stays roughly constant)
- Cumulative funding income: 21 × $10 = $210

Net result on $100K notional in 7 days: $210 funding − $290 cost = **−$80 (loss)**. The 0.010% spread is below break-even at this hold length; you would have been better off not opening. This is exactly the point of break-even analysis: the screen-printed funding rate isn't profit, the funding rate _minus round-trip cost over your actual hold_ is.

So is this even worth it? The answer is: **it depends on the spread**. At a 0.025% spread held the same 7 days you'd make $525 − $290 = **+$235 net**. At 0.050%, $1,050 − $290 = **+$760 net**. That's the spread bar your bot needs to clear, not the headline yield.

The bot's job is to **find the trades where the spread is wide enough**, and **avoid the trades where it isn't**.

---

# Part 3: The Bot, End to End

Now we get to the core of the tutorial: what does the bot actually do, step by step? We'll walk through every phase.

## 3.1 Bird's-eye architecture

Your bot has roughly these components:

1. **Market data ingestion.** WebSocket clients connected to Binance and Bybit, listening for funding rates, orderbook tops, and your own account state.
2. **In-memory state store.** A live picture of "what funding rate is each venue offering on each symbol right now?" plus orderbook depth and account balances.
3. **Signal engine.** A scanner that periodically (or event-driven) looks at the state and asks: "Is there a profitable spread we should trade?"
4. **Risk gate.** A pre-trade check: "Are we allowed to take this trade right now? Is there capital? Is venue health OK?"
5. **Execution engine.** The thing that actually sends orders to both venues, ideally in parallel.
6. **Position monitor.** A continuous loop that watches open positions and decides when to act (rebalance margin, exit, alert).
7. **Closer.** Logic for when to exit a position.
8. **Audit log.** Every decision, every order, every fill, every funding payment received.
9. **Alerting.** PagerDuty / Slack on errors, threshold breaches, liquidation warnings.
10. **Backtester.** Offline replay against historical data. NEVER deploy without this.

Each component is small. The discipline is in the protocols between them — what events one fires, what state another reads, what failure modes propagate where.

## 3.2 Phase 1: Connecting and listening

**Goal:** the bot has a real-time, accurate view of funding rates and orderbook tops on both venues.

What websockets to subscribe to:

- **Binance USDⓈ-M futures.** Binance's current docs split WebSocket endpoints by stream category — funding/mark streams must be routed to `/market`, orderbook/depth streams to `/public`, and User Data Stream to `/private`. **Verify the exact URL prefixes against [Binance's current "Connect" page](https://developers.binance.com/docs/derivatives/usds-margined-futures/websocket-market-streams/Connect) before deploying** (the historical unrouted `/ws/<stream>` path silently returns nothing for `@markPrice` and private streams). Streams to subscribe to:
  - `!markPrice@arr@1s` (route: `/market`) — every second, an array per symbol containing the mark price, the live funding rate (`r`), and the next-settlement timestamp (`T`). The `r` value is the rate that _would_ apply if settlement happened now; it updates continuously between settlements as the orderbook moves.
  - `<symbol>@bookTicker` (route: `/public`) — best bid and ask only (level 1). Useful for mid-price and tight quote display, but NOT enough to estimate fillable notional or top-5 slippage.
  - `<symbol>@depth5@100ms` (or `@depth10@100ms` / `@depth20@100ms`) (route: `/public`) — top-N levels of the order book at 100ms cadence. Required for the depth-based slippage check in §3.3.
  - User Data Stream (route: `/private`, after creating a listenKey) — account balance changes, order updates, position updates.
- **Bybit linear perp**:
  - [`tickers.<symbol>`](https://bybit-exchange.github.io/docs/v5/websocket/public/ticker) — ticker exposing `fundingRate` (live rate for the upcoming settlement), `nextFundingTime`, `fundingIntervalHour`, and `fundingCap` (per-symbol cap; always read the live value rather than hardcoding — Bybit can adjust caps without announcement, and the published cap formula uses an adjustable coefficient). Also includes mark price and last price.
  - `orderbook.50.<symbol>` — top 50 levels of the orderbook
  - Private channels for your own positions and orders

> **Note on terminology.** Both venues stream a **single live funding-rate estimate** for the upcoming settlement; that estimate updates as the orderbook moves. There is no separate "predicted rate" vs "current rate" pair on either WebSocket — it's one continuously-recomputed number that converges to the realized rate as the settlement timestamp approaches. The live estimate is what you compare against the other venue's live estimate when looking for a spread. The historical/realized rates are queryable via REST (Binance `/fapi/v1/fundingRate`, Bybit funding history endpoints) — useful for backtesting, not for real-time signal detection.

> **Bybit ticker semantics — important.** Bybit's V5 ticker stream sends two message types: an initial `snapshot` containing the full set of fields, and subsequent `delta` messages containing only the fields that changed. **A field absent from a delta means "unchanged" — you must NOT overwrite it with null/zero.** Maintain a per-symbol ticker state object: apply the snapshot as full state, then patch each delta over it; emit funding-rate updates from the merged state. A bot that treats every Bybit message as a complete snapshot will silently corrupt `fundingRate`, `fundingCap`, or `nextFundingTime` and trade on garbage. (Binance's `markPrice@arr@1s` does not have this gotcha — every message is a complete update per symbol.)

> **Bybit orderbook semantics — same gotcha, different stream.** `orderbook.50.<symbol>` is also delta-based: an initial `snapshot` carries the full top-50 book, then subsequent `delta` messages carry only changed price levels. **Reconstruction rule:** for each delta, _insert_ missing levels, _update_ existing levels, and _delete_ levels where `size = 0`. Reset the local book on any new `snapshot` or service-restart marker. The §3.3 slippage / fillable-notional check must read from your reconstructed local book — never from the raw latest delta message, which would give you a near-empty book and either skip good trades or wildly mis-size them.

What the listener does on every message:

1. Parse the message into a normalized internal schema (e.g., `FundingRateUpdate { venue, symbol, live_rate, next_settlement_time, funding_interval_hours, cap, floor, observed_at }`)
2. Update an in-memory hashmap keyed by `(venue, symbol)` with the latest values
3. Emit an event to the signal engine if a relevant value changed

In plain pseudocode:

> When a funding rate message arrives from Binance:
>
> 1. Extract symbol, live funding rate (`r`), next-settlement timestamp (`T`), and arrival time.
> 2. Compare against last-known value for `(binance, symbol)`.
> 3. If changed by more than a threshold (e.g., 0.001%), update the state store and signal "funding-changed" for that symbol.
> 4. Bump a heartbeat timer for "binance funding feed alive."

What the listener also does:

- **Reconnects automatically** on dropped connections (exponential backoff, then alert if too long)
- **Reconciles via REST snapshots** every few minutes — the websocket can deliver wrong or out-of-order messages, so periodically pull the canonical truth from REST to verify
- **Tracks freshness** — if no message has arrived in the last 30 seconds for a feed, mark it stale and stop trusting it

This phase is unsexy but determines everything. Garbage in, garbage out.

## 3.3 Phase 2: Detecting an opportunity

**Goal:** based on the current state, decide whether there's a trade worth taking.

The signal engine runs on every funding-update event (or every 5-30 seconds — whichever comes first). For each symbol that exists on both venues:

1. **Fetch the latest live funding-rate estimates.** Both Binance and Bybit stream a continuously-updated estimate of what the funding rate _would be_ if settlement happened right now. (Binance's `markPrice` stream calls this `r`; Bybit's `tickers` calls it `fundingRate`.) This live estimate is what your trade will earn at the next settlement, modulo continued price action between now and then. Don't trade off the _historical_ last-settled rate from REST — that already happened and was paid out to whoever held positions through that cycle; what matters is the rate that will apply to your upcoming cycles.

2. **Normalize, then compute the spread.** Most BTC/ETH pairs have 8h funding intervals on both Binance and Bybit, in which case raw rates compare directly. **For pairs where intervals differ** (some altcoin pairs use 4h or 1h on selected contracts), you MUST normalize before ranking — the simplest convention is per-hour: `rate_per_hour = live_rate / funding_interval_hours`. Spread (in per-hour units) = rate_per_hour_higher_venue − rate_per_hour_lower_venue. The side with the higher _normalized_ rate is where you'll go short. **Comparing raw per-cycle rates across mismatched intervals will invert the trade direction** — e.g., Binance F = +0.030% per 1h vs Bybit F = +0.100% per 8h looks like Bybit is higher, but normalized Binance is 0.030%/h vs Bybit 0.0125%/h, so Binance is actually higher and that's where you'd short.

   **Where do you get the interval?** Bybit's `tickers.<symbol>` WebSocket exposes `fundingIntervalHour` and `fundingCap` directly per symbol. **Binance's `markPrice@arr@1s` WebSocket does NOT** — it only carries the funding rate (`r`) and next-settlement timestamp (`T`). For Binance, treat REST `GET /fapi/v1/fundingInfo` as an **adjustment overlay**, not a full symbol catalog: it returns only symbols with non-default settings (`adjustedFundingRateCap`, `adjustedFundingRateFloor`, `fundingIntervalHours`). Start every Binance symbol from documented defaults (8h interval, ±2% cap, or whatever the contract spec lists), then overlay anything `fundingInfo` returns for that symbol. Refresh the overlay periodically. If the _overlay request itself_ fails or times out, fail closed; but a symbol's _absence_ from a successful response means it's on defaults, not stale.

   In pseudocode, the Binance side is deterministic:

   > def get_binance_funding_params(symbol):
   >
   > 1. Look up `symbol` in the symbol-info cache built from `GET /fapi/v1/exchangeInfo` (refreshed daily). If absent, fail closed — the bot has no contract spec for this pair and must not size a trade against guessed bounds.
   > 2. Initialize `(interval_hours, cap, floor)` from a per-symbol rule table you maintain from Binance's funding FAQ and contract specs. The FAQ enumerates which symbols sit on explicit caps (BTCUSDT/ETHUSDT at ±0.3% per 8h, etc.) and which sit on a risk-tier-derived formula (e.g., ±0.75 × MMR). Refresh this table on a known cadence (weekly is plenty). **Do not silently default to ±2% for a major** — Binance has steadily moved symbols onto the capped-formula list, and ±2% is appropriate only as the last-resort fallback in step 4.
   > 3. Apply the latest `GET /fapi/v1/fundingInfo` overlay (refreshed every few minutes). For each field present for this symbol (`adjustedFundingRateCap`, `adjustedFundingRateFloor`, `fundingIntervalHours`), overwrite the step-2 value. **Fail closed only if the overlay request itself failed or timed out**; a successful response that omits the symbol means it's on the step-2 rule.
   > 4. Return `(interval_hours, cap, floor)`. If any field is still unknown after step 3 — i.e., the symbol exists on Binance but is absent from both your FAQ-derived rule table and `fundingInfo` — fall back to (8h, ±2%, ∓2%), log a loud warning, and prefer to skip new trades on the symbol until the rule table is refreshed.

3. **Compute the break-even spread for your expected hold.** Round-trip cost (derived in §4.1) is ~0.27-0.29% of notional. The break-even formula must be expressed in the same unit as your spread:
   - Per-hour spread (the recommended unit when intervals can differ): `break_even_spread_per_hour = round_trip_cost / expected_hold_hours`. With 0.29% round-trip and 24h expected hold: `0.29% / 24 = 0.0121%/h`. With 8h hold: `0.29% / 8 = 0.0363%/h`.
   - Per-cycle spread (only valid when both venues share the same cycle length): `break_even_spread_per_cycle = round_trip_cost / cycles_held`. At 0.29% and 8h cycles: 8 cycles → 0.0363%/cycle; 3 cycles → 0.0967%/cycle; 1 cycle → 0.29%/cycle.

   The two views are equivalent for 8h/8h pairs (just divide the per-cycle threshold by 8 to get the per-hour). **Spreads compress fast** — realistic plans are 1-3 cycles (8-24 hours), not 8 cycles, so use the tighter per-cycle bar.

4. **Apply a minimum-edge multiplier.** Don't trade at break-even — you'd be working for nothing. Aim for at least 2× break-even. With a 3-cycle (24h) hold plan that's > 0.193%/8h cycle, equivalently > 0.0242%/h. With a 1-cycle plan, > 0.58%/8h, equivalently > 0.0725%/h. The bar is much higher than naive intuition suggests.

5. **Filter by liquidity.** Pull the top-of-book depth on both venues. If you can't fill your target notional within, say, 5 basis points of mid, the trade is too costly. Either size down or skip.

6. **Filter by funding payment timing.** The next funding cycle on Binance might be 4 minutes away, but on Bybit it might be 4 hours away. If you open right before a funding payment on one venue, you'll receive (or pay) that funding within minutes — sometimes a feature, sometimes a bug.

   **Hard rule: enforce a no-trade guard window of ±60-120 seconds around each venue's funding timestamp.** Binance's funding-rate FAQ documents a **15-second** deviation in the actual funding-fee transaction time (so opening at 08:00:05 UTC may _still_ be charged for the 08:00:00 cycle); Bybit documents that opening or closing within **±5 seconds** of funding time does not guarantee inclusion or exclusion. The ±60-120s window in this rule is conservative _bot policy_ on top of those documented venue windows — pad your operational margin beyond what the venue strictly guarantees, because clock skew, network jitter, and your own poll-and-confirm latency add up. Use the exchange's server time (not your local wallclock), wait for `nextFundingTime` to roll AND for the venue's funding ledger event to confirm settlement, and only then trade near a boundary. Beyond the guard window, prefer trades where the two venues' next-settlement times are roughly synchronized.

7. **Filter by current position state.** Are you already in this trade? Did you just close it 2 hours ago (cooldown)? Skip if so.

8. **Filter by venue health.** Is the websocket fresh? Is the API responding? If either is degraded, skip.

If all filters pass, the signal engine emits "open trade for symbol X, long Bybit, short Binance, target notional $N."

In plain pseudocode:

> Every 10 seconds (or on funding-rate-changed event):
>
> 1. For each common symbol on Binance and Bybit:
> 2. Read live_F and funding_interval_hours for each venue from state store.
> 3. If either is stale (>30s old), skip this symbol.
> 4. Normalize: rate_per_h_binance = live_F_binance / interval_h_binance; rate_per_h_bybit = live_F_bybit / interval_h_bybit.
> 5. Determine direction: short on the higher rate_per_h venue, long on the lower.
> 6. Compute spread (per hour) = |rate_per_h_binance − rate_per_h_bybit|.
> 7. If spread < 2 × break_even_spread (also expressed per hour), skip.
> 8. **Funding-boundary guard:** check each venue's server time against `nextFundingTime`. If now is within ±60-120s of either funding timestamp, OR if `nextFundingTime` has rolled but the funding-ledger event has not confirmed settlement, skip.
> 9. Read orderbook depth top-5 on both venues for this symbol.
> 10. Estimate slippage to fill target_notional. If > slippage_budget, size down or skip.
> 11. Check we have margin available on both venues for target_notional.
> 12. Check we are not already in this trade and not within cooldown.
> 13. Check both venues' health (websocket, recent API success).
> 14. If all OK, emit "open_trade" signal with symbol, direction per venue, sized notional.

## 3.4 Phase 3: Pre-trade risk checks

**Goal:** between "we want to open a trade" and "we send the orders," confirm one more time that this is safe.

Why a separate phase? Because between the signal and execution, time passes. State can shift. We re-check:

1. **Re-fetch the latest live funding-rate estimates.** Has the spread compressed below threshold while we were preparing? If so, abort.
2. **Re-fetch account balances and free margin.** Has another trade just consumed margin? Has a withdrawal partially completed?
3. **Re-fetch position state.** Are we somehow already in this trade (race condition)?
4. **Check the kill switch.** Is the bot operator-disabled? Is there a global "no trade" flag set due to upstream alerting?
5. **Check the rate-limit budget.** Are we close to the venue's rate-limit ceiling?
6. **Check the time-of-day window.** Some teams disable opening new trades during low-liquidity hours (weekends, off-hours) or just before a funding settlement.
7. **Check concentration limits.** Will adding this trade put us above per-venue or per-symbol concentration limits?

If any check fails, we abort cleanly: log the reason, increment a counter for monitoring.

## 3.5 Phase 4: Sizing the position

**Goal:** decide exactly how big to make this trade.

The naive answer is "as big as possible." The right answer is constrained by:

1. **Per-trade notional cap.** A static config value, e.g., $50K per trade. (You don't want one bad trade to wipe you.)
2. **Per-venue concentration cap.** No more than X% of total capital exposed at any one venue. E.g., 30% — this caps your loss if the venue goes insolvent.
3. **Per-symbol concentration cap.** Don't put 80% of your capital into one symbol's spread, even across venues.
4. **Liquidity cap.** Notional you can fill within your slippage budget. This is computed from the orderbook depth at top-of-book.
5. **Liquidation distance cap.** How much margin do you want to post per leg? The more leverage, the closer to liquidation. We typically want 2x-3x effective leverage for arb (modest), which means more margin, less yield-on-margin, but vastly safer.
6. **Rebalance budget cap.** You want enough margin headroom that small price moves don't trigger margin calls.

Worked example:

- Total capital across all venues: $1,000,000
- Per-venue concentration limit: 30% → max $300,000 deployable per venue
- Per-trade cap: 5% of capital → max $50,000 notional per trade
- Liquidity check: at top-5 of orderbook, you can absorb $80,000 within 3bp of mid → OK
- Effective leverage target: 3x → margin posted = $50,000 / 3 = ~$17,000 per venue
- Liquidation distance check: at 3x with 0.5% maintenance margin, liquidation is about 33% adverse move away. That's safe.

Final size: $50,000 notional per leg, $17,000 margin per venue.

In plain pseudocode:

> Given target_symbol, target_direction, capital, current_state:
>
> 1. Compute candidate_notional = min(per_trade_cap, capital × per_venue_pct − existing_exposure_at_venue)
> 2. For both venues:
> 3. Read top-of-book depth.
> 4. Compute fillable_notional = sum of size at levels within slippage_budget of mid.
> 5. candidate_notional = min(candidate_notional, fillable_notional × safety_factor)
> 6. Compute required_margin = candidate_notional / target_leverage
> 7. Verify free_margin ≥ required_margin × buffer_factor on both venues
> 8. Compute liquidation_distance at target_leverage; verify ≥ liquidation_distance_min
> 9. Return (notional, margin_per_venue) or "skip if any check fails"

## 3.6 Phase 5: Execution

**Goal:** open both legs — long on the lower-F venue, short on the higher-F venue — with as little time between them as possible, and ideally without leaving you exposed if one fails.

This is the most operationally fragile phase. Get this wrong and you have **one-leg exposure** — a naked perp position that can lose far more than the funding spread you were trying to capture.

> **Position-mode assumption.** This tutorial assumes **One-way Position Mode** on both venues — a single net position per symbol; longs close by selling, shorts close by buying. Pick this at account setup, document it in the runbook, and align the bot's order construction with it. The reason this matters: on **Binance USD-M Hedge Mode**, the venue keeps separate LONG and SHORT positions per symbol and **rejects orders carrying `reduceOnly`** — you close by sending the opposite side tagged with the matching `positionSide` (LONG or SHORT). A literal implementation of the cleanup pseudocode below in Hedge Mode will fail exactly when you most need it (during one-leg cleanup or emergency close) because every reduce-only flag will be rejected. Bybit has an analogous concept driven by `positionIdx` (0 = one-way, 1 = hedge-long, 2 = hedge-short) plus `reduceOnly`; the field names differ, but the same discipline applies — pick a mode and write the close-side logic for that mode. If you do choose Hedge Mode, mirror every reduce-only step below by closing each `positionSide` (or `positionIdx`) independently against its own reconciled position size, and drop the `reduceOnly` flag on Binance.

### Execution strategies, from worst to best

**Naive: serial market orders.** Submit the long order, wait for fill, then submit the short order. Total time can be 1-3 seconds. During that gap, the underlying might move, and you could end up with a worse-than-expected entry. In stressed markets, the gap risk is real.

**Better: parallel marketable IOC limit orders with a hard slippage cap.** Submit both orders at the same instant via parallel API calls, each as a **limit** order with `time-in-force = IOC` priced at `worst_acceptable_price = mid ± slippage_budget` (BUY uses mid + budget, SELL uses mid − budget). The IOC takes whatever liquidity sits inside that price band and cancels the rest. **This is not the same as a raw market order.** Binance's USD-M `MARKET` order type takes quantity but accepts no user price ceiling — if the book is thin or moving, your fill can blow through your slippage budget and you only learn after the fact. Bybit converts an incoming `Market` order into an internal IOC limit using a _venue-set_ protective band, but that band is set for the venue's risk, not your strategy's break-even. Setting your own marketable limit makes the worst case explicit and refusable. Wait for both fills. Total wall-clock time ≈ max(latency to either venue), typically 100-500ms. Risks: an IOC may partial-fill (or not fill at all) if the book moves through your cap before you arrive, and one venue's order may reject outright (insufficient margin, rate limit, exchange outage) — both feed the one-leg-failure handler below.

**Best (when fees dominate): parallel post-only limit orders, with cancel-and-revert on partial.** Submit limit orders at the top of book (post-only = guaranteed maker). Cheaper fees but slower and may not fill at all if the book ticks through your quote. If only one fills, cancel it and revert any leftover position immediately.

For most operators, **parallel marketable IOC limit orders with a strict timeout and a one-leg-failure handler** is the pragmatic default — taker fees, but a price ceiling you actually control.

### The one-leg-failure handler

This is the most important piece of execution code you'll write. Pseudocode:

> 1. Submit BOTH orders concurrently, each tagged with a unique client order ID.
> 2. Wait for both to reach **terminal state** (filled / rejected / canceled) with a timeout T (e.g., 5 seconds).
> 3. Clean case: both filled within slippage tolerance → continue to monitoring phase.
> 4. Anything else (one timed out, one rejected, partial fill, slippage exceeded) → enter degraded-state handler:
>    a. **Stop submitting new strategy orders for this symbol.** This trade is now in cleanup, not opening.
>    b. **Issue cancels** on every non-terminal order across all venues. Best-effort: a cancel can race with a late fill, so don't trust the cancel response alone.
>    c. **Poll each order by client order ID until truly terminal.** Filled, rejected, canceled — wait for one of those. **Do not assume a timed-out order is dead.** Late fills are a real failure mode, including after a "successful" cancel.
>    d. **Reconcile actual positions per venue** from the venue's position endpoint. The truth lives in the position store, not in your local order-state assumptions.
>    e. **Flatten residual exposure with reduce-only marketable IOC limit orders, priced at an emergency slippage cap.** Use `price = mid ± emergency_slippage_budget`, where `emergency_slippage_budget` is _wider_ than the opening `slippage_budget` (cleanup happens during stress, and a wider tolerance is the price of getting flat) but still bounded. The reduce-only flag prevents accidentally opening a _new_ position if a late fill arrives during the flatten; the explicit limit price prevents the order from blowing through your worst-case loss — Binance's raw `MARKET` reduce-only has no user price ceiling and can fill arbitrarily badly in a stressed book. Allow raw `MARKET` reduce-only **only as a documented panic-mode escalation** for cases where even the wider IOC fails to fill and stuck residual exposure has become the bigger risk; gate it behind operator authorization or an explicit panic flag, never as the default.
>    f. Log every step with full context. Alert operator.
> 5. Final outcomes after cleanup:
>    a. Net flat → trade aborted; log realized cost.
>    b. Position matches the original target on both venues → trade open; continue to monitoring.
>    c. Stuck residual on a venue (cancel rejected, reduce-only rejected, API down) → escalate to operator immediately and treat as open directional risk.

The key invariant: **never assume an outstanding order won't fill late.** Always reconcile to actual position state before submitting flattening orders. The naive flow ("reverse the filled leg, then cancel the other") creates a textbook race: the timed-out order fills _after_ you've reversed, leaving you with a position in the opposite direction. Reduce-only and explicit reconciliation prevent that.

The cleanup case has a real cost (extra fees, slippage on the flatten). But that cost is bounded; one-leg exposure that you've stopped tracking is unbounded.

**Rate-limit discipline during cleanup is non-negotiable.** Cleanup is when the bot polls hardest under stress — exactly the moment when REST budget is most contested, both because _every_ arb bot is hammering the same endpoints and because your own cleanup loop (poll-by-client-order-id, reconcile positions, flatten, re-poll) burns weight on every iteration. Both venues track and enforce: Binance returns `X-MBX-USED-WEIGHT-*` and `X-MBX-ORDER-COUNT-*` headers on every REST response and progresses from `429 Too Many Requests` (back off) to `418 I'm a teapot` (IP ban) with bans escalating from 2 minutes to multiple days for repeat offenders; Bybit exposes `X-Bapi-Limit*` headers analogously. Make the cleanup polling loop **header-aware**: read the used-weight value on every response, exponentially back off when usage crosses ~70% of the per-IP limit, treat 429 as a hard pause (respect the `Retry-After` header) rather than an immediate retry, and surface 418 as an operator alert immediately — a multi-hour IP ban during a one-leg event is far more expensive than the cleanup itself, and once banned you have no remediation path that doesn't involve waiting. Allocate independent IPs per venue if you can; a shared IP across venues only works with deliberate cross-venue weight budgeting.

### Idempotency

Every order must have a **client order ID** that you control. If a network glitch makes you uncertain whether an order succeeded, you can re-query by client order ID and find out — vs. submitting a duplicate.

In plain pseudocode:

> def open_trade(symbol, long_venue, short_venue, target_quantity):
>
> 1. Generate trade_id, client_order_id_long, client_order_id_short.
> 2. Convert from a target notional if that's how sizing fed in: `target_quantity = target_notional / reference_mid_price`. Use the SAME `target_quantity` on both venues so the hedge is exactly delta-neutral; venue notionals will differ slightly with the venue marks.
> 3. Read mid_long, mid_short from the reconstructed local books. Compute worst_acceptable_buy = mid_long × (1 + slippage_budget) and worst_acceptable_sell = mid_short × (1 − slippage_budget).
> 4. Construct order_long = (long_venue, symbol, BUY, target_quantity, type=LIMIT, tif=IOC, price=worst_acceptable_buy, client_order_id_long).
> 5. Construct order_short = (short_venue, symbol, SELL, target_quantity, type=LIMIT, tif=IOC, price=worst_acceptable_sell, client_order_id_short).
> 6. Fire both orders in parallel.
> 7. Await both within timeout T.
> 8. Inspect results: if both filled, persist trade with both fills. Else handle one-leg failure.
> 9. Emit "trade_opened" event with full state.

## 3.7 Phase 6: Monitoring

**Goal:** while a position is open, watch every relevant signal and react when something matters.

This is where the bot spends most of its time. Open positions can sit for hours to days. The monitor is a continuous loop that re-evaluates state every few seconds.

### What to track

For each open trade:

1. **Live funding-rate estimates on both venues.** Is the spread still favorable? Has it compressed? Has it flipped (now we'd be paying funding net)?

2. **Mark price on both venues.** Are they tracking each other? If Binance's mark for BTC is $70,000 and Bybit's is $69,800, that's a 30bp basis dislocation. Normal. If it's $69,000, that's 1.4% and you should investigate.

3. **Liquidation distance on each leg.** Define `liq_distance_pct = |mark − liquidation_price| / mark` — the _absolute_ adverse-move percentage required to hit liquidation. Don't conflate this with "% of buffer consumed" (which is the inverse view); pick one convention and stick to it. Recommended thresholds for BTC (consistent with §4.2 sizing): only open if `liq_distance_pct ≥ 30%`; alert at `< 25%`; rebalance margin at `< 20%`; emergency-close at `< 15%`. Tighten these for alts.

4. **Unrealized PnL per leg.** In a delta-neutral position, the PnLs should approximately cancel (one leg's gain ≈ other leg's loss, modulo basis). If they're way off, something is wrong (mark prices diverging, you got partially filled, etc.).

5. **Funding payments received.** Every funding cycle, record what you actually received and what you actually paid. Run **two distinct comparisons** against that ledger entry — they answer different questions and must not be conflated:
   - **Audit (did the venue pay correctly?):** compare the ledger entry to (realized rate at settlement × position size at settlement × settled mark/notional), using the venue's published realized rate, not your signal-time estimate. A mismatch indicates a venue accounting bug, a missed/duplicated event, or local position-state drift — investigate immediately.
   - **Attribution (was our signal good?):** compare the ledger entry to what you projected at signal time from the live estimate. Drift between signal-time estimate and the realized rate is _normal_ — the live estimate moves continuously between entry and settlement. Persistent, structured drift is a signal-model problem, not a venue problem; refit the model rather than alerting.
     Conflating these will false-positive your audit alerts every cycle the live estimate moves (i.e., almost every cycle).

6. **Venue health.** Is the websocket fresh? Has the REST API responded recently? Are we seeing rejected orders or rate-limit errors?

7. **Position size vs current limits.** A liquidation cascade elsewhere can change Binance's tier-margin requirements. Re-verify current MMR for our notional.

8. **Time held.** Are we approaching a hold-time limit (e.g., "close after 7 days regardless")?

9. **Account-level events.** Has Binance pushed a margin call notification? A withdrawal limit? An ADL warning?

### What to do when something matters

1. **Spread compressed below close threshold:** trigger close.
2. **Spread flipped sign:** trigger close (priority).
3. **`liq_distance_pct < 20%` on either leg (BTC; tighter for alts):** transfer margin (rebalance). If we can't, trigger emergency close at `< 15%`.
4. **Mark price divergence > 1%:** investigate. If basis blows up further, consider hedging or closing.
5. **One venue's websocket dead > 60 seconds:** alert. After 5 minutes, consider closing the leg on the working venue (you've lost ability to react).
6. **Margin call notification received:** transfer margin or close immediately.

### How often to monitor

- High-priority signals (websocket disconnect, margin call): event-driven, react in <1 second
- Liquidation distance: every 5 seconds
- Spread re-evaluation: every 10-30 seconds
- Funding payment audit: every funding cycle (8h on Binance/Bybit)
- Heavy reconciliation (full account snapshot vs internal state): every 5 minutes

In plain pseudocode:

> def monitor_loop():
>
> 1.  While bot is running:
> 2.  For each open trade:
> 3.      Read latest live funding-rate estimates from state.
> 4.      Read latest mark prices.
> 5.      Compute current spread, mark divergence, liquidation distances.
> 6.      If spread compressed below close_threshold: emit "close_signal".
> 7.      If spread flipped: emit "close_signal" with high priority.
> 8.      If liq_distance_pct < 20% on any leg (BTC; tighter for alts): emit "rebalance_margin". If < 15%: emit "emergency_close".
> 9.      If mark divergence > 1%: emit "investigate" alert.
> 10. Else: continue monitoring.
> 11. Sleep(5 seconds).

## 3.8 Phase 7: Closing

**Goal:** exit both legs cleanly when the trade should end, capturing whatever PnL was made.

When to close:

- **Spread compressed**: the original opportunity is gone. We've earned what we earned. Close to free up margin for the next opportunity.
- **Spread flipped**: continuing to hold means paying funding. Close as soon as possible.
- **Hold-time limit reached**: rotate capital.
- **Liquidation imminent on either leg**: emergency close to avoid forced liquidation (which is more expensive).
- **Venue degraded**: stop trusting the venue, exit while you can.
- **Operator command**: kill switch / scheduled maintenance.

Closing procedure mirrors opening. Submit both orders concurrently, ensure both fill, log realized PnL.

**Use the same parallel-marketable-IOC pattern as opening (§3.6) — not post-only maker orders — even when the close is unhurried.** The maker rebate looks attractive on paper (save the 5–6 bp taker fee, maybe earn a small rebate), but a maker close introduces **asymmetric queue risk**: if one venue fills as a maker and the other doesn't (or only partially does), you've re-created one-leg exposure on the side that filled, and now you must either (a) chase with a taker fill anyway — paying the fee you tried to avoid plus realized slippage from however far the book has moved — or (b) let the unfilled leg sit in queue while the spread keeps moving against you. Across many trades, the expected cost of (a) and (b) typically outweighs the saved taker fees, and the variance is much worse. Reserve maker exits for advanced patterns where you're explicitly trading queue exposure for fee savings (e.g., very large notionals where market impact dominates fees, or hold patterns long enough that re-entry on a partial fill is fine). For the standard 1–3-cycle arb close, a deterministic IOC flatten beats an optimistic post-only every time.

The realized PnL has these components:

- **Net funding:** funding_payments_received − funding_payments_paid (across all cycles held)
- **Long leg trading PnL:** (long_close_price − long_open_price) × quantity
- **Short leg trading PnL:** (short_open_price − short_close_price) × quantity
- Minus all fees (open + close × both legs)
- Minus realized slippage on the fills

Each leg's trading PnL uses _that leg's own_ base quantity:

> trading_pnl_total = q_long × (long_close − long_open) + q_short × (short_open − short_close)

When you sized by equal base quantity (q_long = q_short = q), this collapses to the change in basis × q:

> trading_pnl ≈ q × ((long_close − short_close) − (long_open − short_open))

This is the _change_ in basis (defined here as `long_venue_price − short_venue_price`), not the basis itself. If basis was +$1 at open and is +$1 at close, the basis-driven part of trading PnL is ~0 — even if BTC moved 20% in between. **The sign matters and trips up many readers**: under this convention, an _increase_ in basis (long-venue price runs ahead of short-venue price) is a _gain_; a _decrease_ is a _loss_. Concretely, if basis moved from +$1 to +$5, trading PnL is **+$4 × q (gain)**. If basis moved from +$1 to −$3 (long-venue price fell behind), trading PnL is **−$4 × q (loss)**. Open in a calm market and close in a stressed one and you'll see large basis swings; open and close at similar basis and it's noise.

When you sized by equal USD notional `N` instead of equal base quantity, `q_long = N / P_long` and `q_short = N / P_short` differ. With basis defined as `P_long − P_short`:

> q_long − q_short = N/P_long − N/P_short ≈ −N × basis / P² (units: BTC)
> equivalent USD exposure ≈ −N × basis / P (units: USD)

For typical CEX-vs-CEX basis on majors (a few to tens of basis points), this residual is on the order of single-digit to a few hundred dollars of USD-equivalent exposure per $100K notional, scaling linearly with both notional and basis. (At 5 bps and $100K: `0.0005 × $100K = $50`. At 20 bps: `$200`.) Small but not zero, and not captured by the equal-q basis-change shortcut. **For execution, prefer to pick a target quantity `q` and submit `q` on both venues** rather than equal USD notional; that way the formula above collapses to zero exactly.

In plain pseudocode:

> def close_trade(trade_id):
>
> 1. Fetch open trade record.
> 2. Construct opposite-direction orders for both legs (sized to current position).
> 3. Submit both concurrently with timeout T.
> 4. On both filled cleanly: compute realized_pnl, persist trade record as closed.
> 5. On any failure (timeout, partial, rejected, slippage exceeded): invoke the same degraded-state cleanup as §3.6 — stop new orders, cancel non-terminal close orders, poll each by client order ID until terminal, reconcile actual position per venue, then flatten residual exposure with reduce-only orders. Do NOT "reverse the filled side first" — that recreates the late-fill race.
> 6. Persist trade record with realized PnL based on actual fills.
> 7. Emit "trade_closed" event with full context.

---

# Part 4: Risk Management Deep-Dive

The previous section described what the bot does. This section explains _why_ the safety controls exist and how to set them.

## 4.1 The break-even funding rate

Every trade has a fixed cost: opening fees, closing fees, expected slippage. To make money, the cumulative funding payments received over the life of the trade must exceed the round-trip cost.

**Calculating round-trip cost:**

As of mid-2026, Binance USDⓈ-M VIP-0 taker fee is around 0.05% (Binance updated this number from 0.04% to 0.05% in May 2026). Bybit linear VIP-0 derivatives taker fee is around 0.055% (was 0.06% historically). Both vary by VIP tier, BNB-discount / Bybit-kickback programs, and region; both venues revise fee schedules regularly — **the canonical source is each venue's own published fee schedule** (Binance fee page, Bybit fee page, both linked from your account dashboard); verify against the current schedule monthly rather than trusting the literal numbers above. Add expected slippage of ~0.02% per leg.

**Don't hardcode fees — fetch them programmatically on bot startup.** Both venues expose your account's _current_ taker/maker fee tier directly: Binance returns it via `GET /fapi/v1/commissionRate?symbol=BTCUSDT` (response includes `makerCommissionRate` and `takerCommissionRate`, reflecting your current VIP/BNB-discount/promotion state for that symbol); Bybit returns it via `GET /v5/account/fee-rate?category=linear&symbol=BTCUSDT` (response includes `makerFeeRate` and `takerFeeRate`). Call these on every boot (and after any tier-change notification), log the values, and feed them into the break-even module rather than relying on a static config. A hardcoded fee that's wrong by even 1 bp will silently flip individual trades from positive-EV to negative-EV — exactly the failure mode the round-trip-cost discipline is meant to prevent, and exactly what happens when the venue updates the schedule, your VIP tier rolls over, or a promotion ends without your config being updated.

- Binance leg open: 0.05% + 0.02% = 0.07%
- Bybit leg open: 0.055% + 0.02% = 0.075%
- Binance leg close: 0.05% + 0.02% = 0.07%
- Bybit leg close: 0.055% + 0.02% = 0.075%
- **Round-trip total: ≈ 0.29% of notional**

To break even on a trade with funding spread `s` per cycle held for `n` cycles:

> n × s ≥ 0.29%
> n ≥ 0.29% / s

If your spread is 0.01% per 8h cycle, you need n ≥ 29 cycles ≈ 9.7 days.
If your spread is 0.025% per 8h, n ≥ 11.6 cycles ≈ 3.9 days.
If your spread is 0.050% per 8h, n ≥ 5.8 cycles ≈ 1.9 days.

**Don't open trades expecting to hold longer than the spread tends to last.** The empirical observation: spreads compress fast. A 0.025% spread might be gone in 24 hours. Plan for short holds (1-3 cycles is realistic) and demand spreads big enough that 1-3 cycles cover round-trip costs with margin to spare.

A practical rule: **only open if expected_realized_funding ≥ 2 × round_trip_cost.** That gives you a 100% safety margin on cost recovery.

## 4.2 Position sizing for survival, not yield

The biggest mistake new arbitrageurs make is sizing for yield. They see "I can earn 30% APR if I lever to 10x" and go to 10x. Then a 10% adverse move liquidates their short leg, the long leg becomes naked, the price keeps moving, and they lose more than 12 months of expected profit in one event.

The right framing: **size such that you survive the worst event you can plausibly imagine over your hold period.** For 2025-26 specifically, that means _including_ the Oct 10-11, 2025 cascade, when BTC moved ~14.5% intraday and many altcoins moved 30%+. Quoting an empirical "99.9th percentile" without an explicit dataset window and exclusion methodology will mislead you, because the relevant tail for an 8h-to-24h arb hold has been demonstrated to be larger than rolling-window stats suggest.

Practical buffer requirements:

- **BTC: liquidation distance ≥ 25-30%** to absorb a stress move plus basis dislocation, latency, and partial-cancel race during cleanup.
- **Major alts: liquidation distance ≥ 40-50%** — the Oct 10-11 event saw 30%+ moves on many alts within minutes.

What that implies for leverage (at 0.5% maintenance margin):

- 3x leverage → ~33% liq distance. Workable for BTC, marginal for alts.
- 5x leverage → ~20% liq distance. **Insufficient for BTC under the post-Oct-10 stress regime; do not assume "5x is fine".**
- 10x leverage → ~10% liq distance. Suicidal in any environment.

For arb, **stay between 2x and 3x effective leverage per leg on BTC, and 1.5x-2x on alts.** The yield-on-margin number you'll see at conservative leverage looks unattractive; that's the point — the strategy's real return is on capital at risk, and the buffer is what makes the strategy survivable across stress events.

**Use the bankroll, not deployed margin, as the yield denominator.** When reporting strategy APR/APY — to yourself, to a stakeholder, or against a T-bill benchmark — denominate against the **full allocated bankroll**, not just the initial margin posted on open trades. Idle exchange-side buffer (capital sitting on a venue but not posted as margin), pre-funded rescue capital (§4.5), and any off-exchange custody float are all **committed to running this strategy** — they earn 0% (or whatever the custodian pays), they constrain how much capital you can re-deploy elsewhere, and they're the reason your strategy survives a stress event. A bot that reports "15% APR on margin" while actually deploying 30% of its bankroll as margin and holding 70% as buffer has a true bankroll yield of **4.5%**, and that — not the on-margin number — is the figure that competes with T-bills, ETF basis trades, or tokenized Treasuries. The leverage-adjusted yield is a sizing diagnostic, not a return metric.

## 4.3 Cross-margin vs isolated margin

Every venue offers two margin modes:

- **Cross-margin:** all your positions on the venue share one margin pool. A loss on one position eats into the margin backing other positions. Capital efficient: a winning position can subsidize a losing one. Dangerous: a single bad position can liquidate everything.

- **Isolated margin:** each position has its own dedicated margin. A loss on one position is contained — the position liquidates, but other positions are untouched. Less capital efficient (you have to post separate margin for each position). Safer.

**For a cross-exchange arb bot, use isolated margin per leg.** If the Binance short leg gets liquidated, you don't want it to eat the margin backing your other unrelated trades on Binance. Isolation contains the damage.

The exception: if you're running a sophisticated portfolio-margin setup where the bot is the only thing trading on the venue, cross-margin can give you better capital efficiency. But until you know exactly how the venue computes portfolio-margin requirements, stick with isolated.

## 4.4 What if a leg liquidates?

Despite all your defenses, sometimes a leg liquidates. Here's what happens:

1. The leg's position is forcibly closed at the bankruptcy price.
2. Your margin on that leg is gone.
3. The other leg becomes a naked directional position.
4. Until you close the other leg manually, you have full directional exposure.

The bot should handle this:

- **Detect liquidation** within seconds (via private user-data websocket or position-changed event).
- **Immediately close the other leg.** Yes, the close will likely be at a worse price than entry. But the alternative is unbounded loss.
- **Log the incident with full context.** Cause analysis later.
- **Alert the operator.** Big loss events need eyes.

What is the actual loss? The naive answer "you lose the margin posted on the liquidated leg" is _wrong_ for a delta-neutral pair — it ignores the offsetting unrealized gain on the hedge leg.

Walk through it. Setup: short $50K Binance perp at $70K BTC (margin $17K, 3x leverage), long $50K Bybit perp at $70K (margin $17K). BTC rallies ~33% to ~$93K, your Binance short hits liquidation. At that moment:

- **Binance short leg:** liquidated. Margin recovery depends on the bankruptcy-price gap and the venue's insurance fund — typically you lose most of the $17K, possibly all.
- **Bybit long leg:** unrealized gain ≈ 33% × $50K ≈ +$16.5K.

If you close the Bybit leg cleanly, your portfolio-level loss is roughly: (margin lost on Binance) + (slippage to close Bybit) + (any fees and adverse funding accrued during the move) − (Bybit's realized hedge gain). For typical 3x setups in a clean event, that nets out to **a low single-digit thousand dollars of loss**, not the full $17K.

The catastrophic scenarios — when a single-leg liquidation actually approaches full margin loss or worse — are when the hedge leg is _also impaired_:

- The other venue is down or unresponsive, so you can't close the hedge promptly while it bleeds funding or drifts on basis.
- An ADL event force-closes the hedge leg at an unfavorable price (this happened broadly during the Oct 10-11, 2025 cascade — winning legs were auto-deleveraged before holders could realize gains).
- Basis between the two venues blew up during the move, so the hedge leg's "unrealized gain" is partially fictional once you actually realize it.
- Bankruptcy-price gap on the liquidated leg is large enough that the insurance fund is tapped, or in the worst case, ADL hits the _other_ side of the book.

**Position sizing is what determines how bad the worst case is.** A bot running many small positions can absorb several liquidation events a year — including the catastrophic-correlation ones — and still net positive, _if_ each individual position is sized small enough that the worst plausible loss on it (full margin on both legs, in the catastrophic case) doesn't compound across positions to wipe the book.

## 4.5 What if a venue goes down?

API errors. Websocket disconnects. Maintenance windows. Worse, full insolvency.

- **Short outage (< 5 minutes):** orders may queue, websocket re-establishes, no action needed beyond alerting.
- **Medium outage (5 minutes - several hours):** you've lost the ability to react on that venue. The other leg is drifting. Mitigation: smaller positions, multiple venues, the ability to manually flatten the working leg if needed.
- **Long outage / suspected insolvency:** treat as worst case. The leg on the bad venue is potentially worth zero. Close the other leg. Acknowledge potential 100% loss on the bad venue.

The systemic mitigation: **don't run more than X% of capital on any one venue.** Common practitioner rule: max 20-30% per venue. If Binance dies, you lose 20-30%, not 100%.

## 4.6 What if the basis blows up?

In normal conditions, Binance's BTC mark price ≈ Bybit's BTC mark price within a few basis points (both pull from similar spot-index baskets). In stress, modest divergence happens. During the Oct 10-11, 2025 cascade, basis between major CEXes widened materially for several minutes before convergence, and ADL on the contra side of bankruptcies forced winning legs to close at unfavorable prices.

When basis blows up, your "delta-neutral" book is no longer neutral. The mark prices used to compute liquidations have diverged. You can be liquidated on one leg even though the "true" external price hasn't moved that much. Mitigation:

- **Watch basis divergence in real time.** Alert at 0.5%, freak out at 1%+, exit at 2%.
- **Have pre-funded rescue capital ready** to send to whichever venue is blowing up, to widen liquidation distance on the affected leg.
- **Accept that some events will hurt.** No infrastructure can fully prevent black-swan microstructure failures. Size for survival.

A _related but distinct_ risk is **collateral / oracle failure**, where a venue's _internal_ pricing of a margin asset diverges from external markets. The canonical recent example is Binance's local USDe depeg on Oct 10-11, 2025: USDe traded at $0.62 on Binance internal orderbooks while remaining near $1.00 elsewhere, because a Binance-specific pricing mechanism fed margin and liquidation calculations from local order flow rather than external references. Anyone using USDe as collateral on Binance saw their effective margin slashed even though their assets were sound externally. The lesson for cross-exchange arb: **use plain USDT or USDC as margin** — not exotic stablecoins, LSTs, or yield-bearing tokens whose venue-internal pricing can fail independently of the asset itself. The USDe story is _not_ direct evidence that BTCUSDT perp marks themselves diverge by ~38% between Binance and Bybit; it's a separate, collateral-side category of failure that can hurt you even when the BTC inter-venue basis is well-behaved.

## 4.7 Capital inventory drift across venues

PnL accrues _per venue_. Funding payments hit the venue where the position lives; trading PnL settles into that venue's wallet; cleanup costs (extra fees, slippage on the flatten) lock losses on whichever venue the cleanup happened on. Even a strategy that's roughly delta-neutral at the trade level develops a steady **capital skew at the wallet level** because the realized cost asymmetry compounds across many trades. A bot running for three months on a single symbol pair can easily drift from a 50/50 venue allocation to 70/30 or worse — _without any losing trades_ — at which point the lighter venue is increasingly margin-constrained for new opens, and your effective deployable capacity is set by whichever wallet ran dry first, not the bankroll.

Once drift is meaningful, you have three operational responses, each with different cost and operational footprint:

- **Withdraw → bridge → deposit (the on-chain rebalance).** Withdraw USDT (or USDC) from the heavy venue, bridge or transfer to the light venue's deposit address, deposit. Cost: gas (Tron's TRC-20 USDT or a low-fee bridge is the usual path), the venue's withdrawal fee, the venue's withdrawal queue latency. Time: minutes in calm markets, **hours during stress** — withdrawal queues lengthen exactly when you most want capital to move (see §4.5). The bot is paused on the affected pair until the deposit confirms. Workable for retail/small operators; brittle as the only mechanism at scale.
- **Off-exchange settlement (Copper / Ceffu / Fireblocks).** Hold assets in third-party custody with mirrored sub-accounts; the venue runs margin calculations against the custodian-held balance rather than on-venue funds, and capital can shift between venue allocations in seconds without an on-chain hop. Cost: custodian fees + integration complexity + counterparty risk on the custodian. Typically only economical above ~$10M deployed. The Oct 10-11 USDe-depeg episode (§4.6) shifted significantly more institutional capital onto this rail because on-venue collateral was visibly fragile.
- **Live with the skew, scale positions to the constrained side.** Accept that the smaller venue's wallet sets your trade size cap; trim notionals proportionally; rebalance manually (i.e., trigger an on-chain transfer) on a weekly or monthly cadence rather than reactively. Pragmatic at small scale and avoids paying for a custody integration you don't need. A hidden capacity ceiling at large scale, since position sizes shrink as drift compounds.

The bot itself should at minimum **monitor and alert on inventory drift**: track per-venue capital share against the target allocation, and alert when the deviation exceeds a threshold (e.g., 15 percentage points off target, or whenever the constrained venue's free margin is less than 1.5× the next planned trade's required margin). Most retail operators handle the rebalance manually on alert; production-scale operators automate the trigger via off-exchange settlement.

---

# Part 5: The Architecture You Actually Need to Build

This is what you'd put on a whiteboard if you were designing the system from scratch.

## 5.1 Process and component layout

The simplest production-viable design is a single binary process per region, with clear internal modules:

- **Module A: Market data collectors** (one per venue). Each maintains its websocket connections, normalizes events into a common schema, and pushes them into a shared in-memory state store.
- **Module B: State store.** A simple in-memory data structure (or Redis if you want crash-recovery). Keys like `(venue, symbol)`, values are the live funding-rate estimate, next-settlement timestamp, funding-cap and floor, mark price, top-of-book bid/ask, freshness timestamp.
- **Module C: Signal engine.** Periodic scanner. Reads from State Store. Emits "open_trade" or "close_trade" events.
- **Module D: Risk gate.** Subscribes to signals. Re-validates against current state. Forwards to Execution if all checks pass.
- **Module E: Execution engine.** Per-venue API clients. Submits orders. Handles one-leg-failure.
- **Module F: Position monitor.** Background loop. Reads State Store for currently-open trades. Triggers re-evaluation, rebalance, or close.
- **Module G: Audit log + metrics.** Every event is persisted. Metrics flow to Prometheus / Grafana.
- **Module H: Operator interface.** A small HTTP API or CLI for kill switch, manual close, and inspection.

**Boot and recovery protocol (reconcile-only mode).** A bot restart with open positions or in-flight orders is dangerous if signal processing resumes before state is reconciled — the bot can emit "open" signals into a venue where it already has a position, or assume positions are flat when they aren't. On every cold start, after a kill-switch toggle, and after any extended websocket gap that may have masked private events, the bot must boot into a **reconcile-only** state: signal engine and execution engine remain disabled while Module B is rebuilt from venue ground truth. Specifically: pull current positions, all open orders (by client order ID), and the recent funding-ledger / income-history records from each venue's REST endpoints; rebuild the trade state by joining those records against the audit log written by Module G; resolve every discrepancy explicitly — open orders the bot doesn't recognize, positions whose recorded direction or size doesn't match, missing funding payments — by **alerting the operator rather than auto-correcting**, since silent self-repair turns the audit log into fiction. Only after reconciliation completes successfully (or the operator clears each flagged item) does the bot enable Modules C, D, E. The cost is a few seconds of unavailability at startup; the alternative is the bot trading against stale assumptions.

A separate **backtester** binary uses the same Modules C, D (with mock execution), feeds them historical data instead of live websocket streams.

## 5.2 Latency budget

Funding arbitrage is not microsecond-sensitive. Your competition is other funding-arb bots, not market-makers. Reasonable targets:

- Funding rate update message arrives → reflected in State Store: < 50ms
- State changed → Signal engine evaluates → emits trade signal: < 100ms
- Trade signal → Risk gate validates → Execution submits orders: < 500ms
- Orders submitted → fills confirmed: < 2-5 seconds (depending on venue)

If you're consistently above these, the live funding-rate estimate may have moved before you act. Either improve latency or back off (re-validate before acting).

## 5.3 Deployment

Run the bot in the same cloud region as the venue's matching engine. **Venue matching-engine regions are not officially published**, so the mapping below is empirical — community/practitioner consensus plus your own latency probes from candidate regions. Treat it as a starting hypothesis to verify with venue ops, account-management contacts, or a few days of measured RTT before committing capacity:

- For Binance USDⓈ-M perp: AWS Tokyo (`ap-northeast-1`)
- For Bybit linear: AWS Singapore (`ap-southeast-1`) historically, with some workloads moving to Tokyo
- For both: a Tokyo deployment is typically fine; ~10ms RTT to either is acceptable for 8h-funding strategies

Use AWS, GCP, or self-hosted bare metal in Equinix TY3 if you want full control. For most teams starting out, a single c6i.xlarge or c7i.large in Tokyo is more than enough for hundreds of symbols.

## 5.4 Observability

You need to know what's happening at all times:

- **Metrics:** funding rate per (venue, symbol), spread, websocket freshness, open positions, unrealized PnL, realized PnL, trades per day, fill rate, error rate.
- **Logs:** every signal evaluation, every risk check (pass or fail with reason), every order submitted and its outcome, every funding payment received.
- **Dashboards:** real-time view of open positions, current spreads, account balances per venue, alert state.
- **Alerts:** websocket dead > 30s, position drawdown > X, `liq_distance_pct < 25%` (alert) / `< 20%` (rebalance) / `< 15%` (emergency close), basis divergence > 0.5%, kill switch activation.

## 5.5 Backtesting before deployment

**Never deploy without a working backtester.** Here's what backtesting must capture:

- **Tick-level historical _live funding-rate estimates_** on both venues for at least 6 months. **Important distinction:** the signal in §3.3 trades on the live, continuously-updated estimate (Binance `markPrice@arr@1s` `r` field, Bybit `tickers.<symbol>` `fundingRate`). Per-exchange funding-history REST endpoints (Binance `/fapi/v1/fundingRate`, Bybit funding-history) return only the _realized settled rate at each funding timestamp_. Using a settled rate **after** its timestamp as a lagged feature is fine (just a different, lower-frequency strategy). What's not fine is letting your backtest see the final settled rate _before_ settlement — that's the actual lookahead-bias trap. To backtest THIS tutorial's live-estimate signal, you need captured exchange-native WebSocket streams. **Tardis.dev** records WebSocket-sourced derivative tick data including funding rate, mark price, and index price across major venues. **Coinglass** advertises funding OHLC/history and order-book tick data — verify with their current plan whether they actually replay Binance `!markPrice@arr@1s` and Bybit `tickers.<symbol>` at full tick granularity, since funding _OHLC_ aggregated at coarser intervals is not the same as the live-estimate stream. If neither vendor fits, capture the live streams yourself before you can backtest against them.
- **Top-of-book and depth history** for slippage modeling (Binance `depth5@100ms` and Bybit `orderbook.50` recordings)
- **Realistic fee model** matching your actual VIP tier
- **Realistic latency model** — orders submitted at time T fill at time T + Δ, where Δ is sampled from your live latency distribution
- **Realistic one-leg-failure rate** — based on past observations
- **Funding payment timing** — settlements happen at scheduled UTC times, but with documented venue ambiguity (Binance: 15-second deviation per current FAQ; Bybit: ±5 second no-guarantee window). Model both venue windows explicitly and replay the same ±60-120s conservative bot-policy guard from §3.3 in the simulator, so the backtest accounts for trades that get caught (or excluded) by the boundary.
- **Full close-rule policy replayed against forward realized funding** — the §3.3 entry signal trades on the _live estimate_ over an _expected hold window_, and the §3.7/§3.8 monitor closes on spread compression, sign-flip, hold-time limit, and liquidation-distance triggers. The backtester must simulate all of those exit rules tick-by-tick against the forward stream of live estimates _and_ realized cycle settlements, never assume "open at signal, hold for N cycles, collect entry-time spread × N." A simulator that holds for a fixed N regardless of how the spread evolves will overstate edge by leaving compressed/flipped trades open and by under-counting close-side fees, slippage, and one-leg failures on early exits.

A backtest that says "20% APR Sharpe 3" usually delivers 8-12% Sharpe 1.5 live. The gap is real-world friction (fees, slippage, race conditions, occasional one-leg failures, basis events). If your backtest can't survive realistic friction modeling, your live deployment won't either.

## 5.6 Iterative deployment

Start tiny. Don't deploy with $1M of capital on day one.

- Week 1: paper trade. Run the bot but don't actually submit orders — just log "would have submitted X." Verify decisions look right.
- Week 2-4: $1,000 of real capital. You will likely lose some on fees, but you'll find every operational bug.
- Month 2-3: $10,000 of capital if things look good. You'll catch one-leg-failure handling, websocket disconnect recovery, etc.
- Month 4+: scale up gradually. Each 10x scale-up exposes new failure modes (rate limits, order book impact, tier-margin transitions). Take it slow.

---

# Part 6: Common Pitfalls (Curated)

Things that bite operators in practice:

1. **Stale funding data.** The live funding-rate estimate updates continuously. If your websocket is lagging by 15 seconds, you might trade on stale data — and the actual realized rate could be significantly different. Always check timestamp freshness before acting.

2. **One-leg fill handling bugs.** This is the most common production incident. Test the cleanup handler with a fault injector — simulate venue rejections, partial fills, and post-cancel late fills, and verify the bot reconciles to actual position state and flattens residual with reduce-only orders. Many bots accumulate small one-leg losses that, over a year, cost more than the strategy earns.

3. **Funding payment timing.** Funding settles at fixed UTC times, but with documented venue-specific ambiguity — **Binance: 15-second deviation per current FAQ**; **Bybit: ±5 second no-guarantee window**. If you open a position 5 minutes before settlement on the receiving venue, you'll receive that funding within minutes. If you open at 08:00:05 thinking you've skipped it, Binance may _still_ include you in the 08:00:00 cycle. **Implement a ±60-120s guard window** around each venue's funding timestamp as conservative bot policy (broader than the documented venue windows, padding for your own clock skew, jitter, and confirmation latency); never open or close inside that window unless your strategy explicitly models settlement inclusion. At the BTCUSDT cap of ±0.3% per 8h, one surprise funding charge on a $100K leg is $300 — bigger than the entire $290 round-trip cost from §4.1.

4. **Cap-bound spreads.** Each contract has a per-symbol funding-rate cap and floor; "±2%" is the loose upper bound, but specific pairs are tighter. Binance documents BTCUSDT at ±0.3% per 8h, and Bybit's `tickers.<symbol>` exposes a live `fundingCap` (currently 0.5% on BTCUSDT/ETHUSDT, venue-adjustable without announcement). If you see what looks like a wide spread because one venue's live rate is deeply negative, but the floor on that venue caps how negative the _realized_ rate can be, your captured spread will be smaller than the screen-printed live spread. **Always read the live cap and floor from the API per symbol — don't assume ±2% and don't confuse the cap with the ±0.05% premium-vs-interest damper inside the formula. They are different parameters.**

5. **Liquidation cascade / ADL.** During Oct 10-11, 2025, exchanges used Auto-Deleveraging (ADL) to forcibly close winning positions on the contra side of bankruptcies. Your delta-neutral book can be torn apart not by your own liquidation but by being on the "wrong side" of a venue-wide ADL event. There's no full mitigation — only smaller sizes and venue diversity.

6. **Withdrawal queues.** When markets are stressed, withdrawals from CEXes can take hours instead of minutes. If your bot's emergency response is "transfer margin to the affected venue," that response is unavailable during the crisis when you need it. Pre-fund rescue capital on each venue or use off-exchange settlement (Copper / Ceffu / Fireblocks).

7. **API rate limits during stress.** During flash crashes, every bot is hammering the API. You'll hit rate limits faster than usual, and your "emergency close" might queue. Pre-budget your rate-limit usage; don't run normal-state operations at >50% of the limit so you have headroom for stress.

8. **Time sync issues.** A 1-second clock skew between your bot and the exchange can cause subtle bugs around funding-cycle boundaries. Use NTP at minimum (chrony). Some funding-edge-related "mysterious" PnL discrepancies trace back to clock drift.

9. **Tier-margin transitions.** Binance has tiered maintenance margin: at $50K notional, MMR is 0.4%; at $5M notional, MMR is 5%. As you grow position size, you might cross a tier and suddenly need much more margin. The tier change happens silently — model it, and don't size into a tier without intentional planning.

10. **Weekend liquidity.** Spreads on most pairs widen by 50-100% over weekends as market makers reduce activity. Slippage budget should account for this. Some teams disable opening new positions outside major-session hours.

11. **Fee schedule changes.** Both Binance and Bybit periodically update their fee schedules. Subscribe to their announcement channels; verify your bot's fee model matches the current published schedule monthly.

12. **Symbol delistings.** Occasionally venues delist a perp pair (low volume, regulatory issue). If you have an open position when the delisting is announced, plan ahead — typically you have 24-72h to close, after which forced settlement happens at venue-determined prices.

---

# Part 7: Beyond Cross-Exchange (Briefly)

You're now equipped to build cross-exchange funding arb. Here's a quick map of related strategies for context:

- **Cash-and-carry (long spot + short perp).** Capture funding on a single venue. Higher capital required (you actually own the spot asset), but cleanest accounting, no second-venue dependency. Good for individuals with custody arrangements.

- **Cross-pair within one venue.** Long USDT-perp, short USDC-perp on the same exchange. Tiny edge from stablecoin-funding differentials. Niche.

- **LST + perp short.** Long stETH (earns ~3% staking), short ETH perp (earns funding). Stack both yields. Requires LST custody and tracks stETH/ETH peg risk.

- **Basis arbitrage with quarterly futures.** Long spot, short a Q2/Q3/Q4 future. Lock in a known terminal basis at trade entry. No funding-rate volatility risk. Roll cost every 90 days.

- **Funding-rate prediction.** Don't always be in; only enter when your model predicts the spread will widen. Edge often eaten by extra round-trip costs. Better at improving Sharpe than CAGR.

If your cross-exchange bot works, the natural next step is adding more venues (Hyperliquid, OKX, Drift) — the same logic generalizes. After that, cross-pair and LST stacks are the natural extensions.

---

# Part 8: Closing Checklist Before You Ship

A practical pre-deployment checklist:

- [ ] WebSocket clients reconnect automatically with exponential backoff
- [ ] State store includes freshness timestamps; stale data is filtered out at signal time
- [ ] Risk gate has hard caps for: per-trade notional, per-venue concentration, per-symbol concentration, total open positions
- [ ] Execution engine submits orders concurrently, has a 5-second timeout, and has a tested one-leg-failure handler
- [ ] Cleanup paths exercised end-to-end with a fault injector — simulated venue rejections, partial fills, post-cancel late fills, network timeouts, and 429/418 rate-limit responses — and the bot reconciles to actual position state and flattens with reduce-only IOC limits in every case
- [ ] Position monitor checks `liq_distance_pct` every 5 seconds; rebalances at the configured threshold (e.g., 20% on BTC) and emergency-closes below the floor (e.g., 15%)
- [ ] Closer fires on: spread compression, spread flip, liquidation imminent, hold-time limit, kill switch
- [ ] All orders use client-controlled order IDs (idempotency)
- [ ] All decisions, orders, and fills are logged with full context to durable storage
- [ ] Metrics are exposed to Prometheus; dashboards show open positions, spreads, balances, error rates
- [ ] Alerts fire on: websocket dead > 30s, `liq_distance_pct < 25%` / 20% / 15% (alert / rebalance / emergency-close), basis divergence > 0.5%, drawdown threshold breach
- [ ] Kill switch flattens all positions with one command
- [ ] Bot boots into reconcile-only mode on every cold start and after extended websocket outages: fetches positions, open orders, and recent funding-ledger entries from every venue, rebuilds trade state, alerts on every unmatched item, and only enables signal/execution after reconciliation completes
- [ ] Backtest covers ≥ 6 months, with realistic fee + slippage + latency modeling, and shows acceptable risk-adjusted returns
- [ ] Deployed in venue-friendly cloud region (Tokyo for Binance/Bybit)
- [ ] Time synced via NTP; clock drift monitored
- [ ] API keys are sub-account-scoped, IP-whitelisted, with separate read-only and trade-enabled keys
- [ ] Started with $1K of real capital and grew with proof
- [ ] Have a written runbook for: exchange outage, websocket flapping, one-leg failure spike, basis blowup, full venue insolvency

---

# Closing Thoughts

You now know what a funding rate is, why cross-exchange funding arbitrage works, what your bot needs to do at each stage, the risks that can ruin you, and the architecture to build.

A few things worth internalizing:

**This trade is mechanical, but not easy.** The math is simple, the infrastructure is well-trodden, and the strategy has been documented for years. But the operational discipline required — handling one-leg failures, monitoring liquidation distance, surviving stressed-market basis blowups, sizing for tail events — is where the bot earns its keep.

**The realistic 2026 edge on majors is single digits over T-bills net of taxes** (the by-capital-tier breakdown and the 5.5–7.5% / Sharpe 1.5–2.5 ranges underlying this claim are derived in [`README.md`](./README.md), [`01-market-landscape.md`](./01-market-landscape.md), and [`04-comparative-and-operational.md`](./04-comparative-and-operational.md), with primary sources catalogued in [`sources.md`](./sources.md)). If you build for that — solid, durable, defensive infrastructure — you can run this for years. If you build for "moonshot" returns by leveraging up, you're trading the strategy of "boring monthly carry, ruinous tail" without realizing it.

**Build the boring infrastructure. Run it small first. Grow with proof.** The operators who are still in business in 2026 are the ones who treated risk controls as the primary product, not an afterthought.

Good luck. Build it well.
