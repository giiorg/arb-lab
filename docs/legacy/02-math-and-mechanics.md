# Part 2 — Mechanics, Math, Strategies, Vocabulary

## 2.1 Perpetual swap fundamentals

A **perpetual swap** ("perp") is a derivative contract that tracks an underlying spot price _without an expiry date_. Unlike traditional futures (which converge to spot at expiry), perps converge continuously through a **funding mechanism**: a periodic peer-to-peer cash flow between long and short holders sized to push the contract price toward spot.

Formally, perp counterparties exchange:

- **Variable leg:** mark-to-market PnL from price movement of the underlying
- **Funding leg:** periodic payment, $\text{Funding Payment} = \text{Notional} \times F$, paid from one side to the other based on $\text{sign}(F)$

**Origin:** Launched on BitMEX May 2016 by Arthur Hayes / Ben Delo / Samuel Reed as XBTUSD perpetual swap. Inspired by Robert Shiller's 1992 academic proposal for perpetual claims on real estate. Perps now >75% of all crypto trading volume (BitMEX 2025Q3 report).

**Why funding exists:** A contract that never expires has no terminal cash flow tying it to spot. Funding imposes negative carry on whichever side is bidding the contract away from spot. If perp > spot, positive funding makes longs pay shorts, raising the cost of being long, incentivizing arbitrage until basis closes.

**Direction:**

- Positive $F$ → longs pay shorts (perp at premium)
- Negative $F$ → shorts pay longs (perp at discount)

**Why this is an arb opportunity:** Funding is a cash transfer _unrelated_ to price direction. A delta-neutral position (long 1 BTC spot + short 1 BTC perp) is fully hedged against price and collects funding as pure yield, modulo execution costs, basis risk, and counterparty risk.

## 2.2 Funding rate formulas — per exchange

### General canonical form

$$F = P + \mathrm{clamp}(I - P,\ -C,\ +C)$$

$$\text{Funding Payment} = N \cdot F$$

where $P$ = time-weighted Premium Index, $I$ = baseline interest rate, $C$ = clamp half-width (typically 0.05%), $N$ = position notional.

When premium and interest are aligned, $F \approx I$ (small baseline). When premium dominates ($|P-I| > C$), $F \approx P$, dragging perp back toward spot.

### Binance (USDⓈ-M and COIN-M)

$$F = \frac{P + \mathrm{clamp}(I-P,\ \pm 0.0005)}{8/N_\text{hours}}$$

- **Premium Index:** $P = \frac{\max(0, \text{Impact Bid} - \text{Index}) - \max(0, \text{Index} - \text{Impact Ask})}{\text{Index Price}}$
- **Interest Rate:** $I = 0.01\%$ per 8h (= 0.03% daily). Clamp ±0.05%
- **Funding interval:** 8h default (00:00, 08:00, 16:00 UTC); some pairs 4h or 1h
- **Sampling:** Premium Index every 5s → 5,760 samples in 8h, time-weighted
- **Impact Margin Notional (IMN):** $\frac{200\ \text{USDT}}{\text{Initial margin rate at max leverage}}$
- **Cap:** ±2% for most contracts; risk-tiered ones ±0.75 × maintenance margin ratio
- USDⓈ-M and COIN-M use identical formula. Linear: $\Delta V = Q \cdot \Delta P$. Inverse: $\Delta V_\text{coin} = Q \cdot (1/P_0 - 1/P_1)$

### Bybit

- Same general form with $I = 0.03\%$ daily (= 0.01% per 8h cycle)
- **TWAP** linear-weighted: $P = \frac{1\cdot p_1 + 2\cdot p_2 + \dots + 480\cdot p_{480}}{1+2+\dots+480}$ over 480 minute samples
- 8h default; many pairs migrated to 4h or 1h in 2025-2026

### OKX

$$F = \mathrm{clamp}\bigl[\,P_\text{avg} + \mathrm{clamp}(I-P_\text{avg}, \pm 0.0005),\ \text{Floor},\ \text{Ceiling}\,\bigr]$$

- $I = 0.03\% \div (24/N_\text{hours})$
- $P_\text{avg}$: linearly weighted TWAP
- **Outer clamp** to per-market floor/ceiling distinguishes OKX
- 8h default; selected pairs 1h/2h/4h
- Formula updated in 3 batches Apr 10/17/24, 2025

### Hyperliquid

$$F_{8h} = P + \mathrm{clamp}(I-P, \pm 0.0005); \quad F_{1h} = F_{8h}/8$$

- $I = 0.01\%$ per 8h
- **Premium (standard):** $P = \frac{\max(0, \text{Impact Bid}-\text{Oracle}) - \max(0, \text{Oracle}-\text{Impact Ask})}{\text{Oracle Price}}$
- **HIP-3 perps:** $P = \frac{0.5(\text{Impact Bid}+\text{Impact Ask})}{\text{Oracle}} - 1$ (more responsive)
- **Cap: 4% per hour** (much higher than CEX)
- **Payment uses oracle price (not mark)** — eliminates mark-price gaming

### dYdX v4

$$F_{1h} = P_{8h}/8 + I_\text{hourly}$$

- Premium (8h): standard impact-bid/ask form, averaged from 60 one-minute median FundingPremiumVotes
- **Interest Rate: 0% for cross markets by default.** Isolated markets: $I = 0.125$ bps/hr ≈ 1 bp/8h
- Funding tick 1h; sampling 1m
- 8h cap: $600\% \times (\text{IM} - \text{MM})$. BTC-USD large-cap: 600% × (5% - 3%) = 12% per 8h
- Built on Cosmos SDK / CometBFT

### Drift Protocol (Solana)

$$F_{1h} = \frac{1}{24} \cdot \frac{\text{Mark TWAP} - \text{Oracle TWAP}}{\text{Oracle TWAP}}$$

- Mark TWAP: $(bid+ask)/2$ EWMA 1h span
- **Lazy hourly** — funding triggered by user activity; if no activity, next trader pays/receives accumulated rate
- Caps by tier: B+ 0.125%/hr; C-tier 0.208%; lower 0.4167%

### GMX v2 — different model

Splits into two distinct fees:

**Funding fee** (imbalance side pays smaller side):

$$f_\text{funding/sec} = \text{FundingFactor/sec} \cdot \frac{|\text{OI Imbalance}|^{e_f}}{\text{Total OI}}$$

**Borrowing fee** (paid by ALL leveraged positions to LPs):

$$f_\text{borrow/sec, longs} = \text{BorrowingFactor} \cdot \frac{(\text{OI}_\text{long, USD} + \text{Pending PnL})^{e_b}}{\text{Pool USD}}$$

GMX v2 does **not** have a traditional premium-index funding. Funding only balances OI between sides; borrowing is true financing cost paid to GLP/GM LPs (not zero-sum vs. the other side).

### Other venues

| Exchange      | Interval                   | Formula notes                                                                                                            |
| ------------- | -------------------------- | ------------------------------------------------------------------------------------------------------------------------ |
| Aevo          | 1h                         | $F_{1h} = F_{8h}/8$, Binance-style with $I = 0.01\%$                                                                     |
| Backpack      | 1h                         | Mean-premium clamp variant; payment = $F \times \text{Mark} \times \text{Quantity}$                                      |
| Coinbase Intl | 1h                         | $P = (\text{Mark}-\text{Index})/\text{Index} \div 24$, every 3min × 20 samples; EMA: $F_t = 0.75P + 0.25 F_{t-1}$        |
| Vertex        | 1h                         | Funding Index = TWAP(Perp Mark) − TWAP(Spot Index); cap 10%/day                                                          |
| Deribit       | continuous (every few sec) | $P = (\text{Mark}-\text{Index})/\text{Index}$; damper zero in [-0.025%, +0.025%]; cap ±0.5% BTC, ±1% ETH (8h-equivalent) |

### Worked annualization examples

$$\text{APR} = F \times \frac{8760}{T}, \qquad \text{APY} = (1+F)^{8760/T} - 1$$

**Example 1 — Binance 8h, $F = 0.01\%$:**

- APR = $0.0001 \times 3 \times 365 = 10.95\%$
- APY = $(1.0001)^{1095} - 1 = 11.58\%$

**Example 2 — Hyperliquid 1h, $F = 0.00125\%$:** APR = same 10.95%. Not a coincidence; Hyperliquid simply pays the 8h rate spread over 8 hours.

**Example 3 — Stress case, $F = 0.08\%$/hr (Drift cap):**

- APR (annualized) = $0.0008 \times 8760 = 700.8\%$
- Realized over 1 week = $(1.0008)^{168} - 1 = 14.4\%$

This is why high-funding altcoin trades produce outsized short-term returns but cannot be assumed to persist.

## 2.3 Strategy types

### A. Cash-and-carry (core strategy)

**Setup:** Long $X$ units spot + short $X$ units perp. Net delta ≈ 0.

$$\text{PnL} = \underbrace{(P_T-P_0)X}_\text{spot} + \underbrace{(P_0-P_T)X}_\text{perp} + \underbrace{\sum_t F_t N_t}_\text{funding} - \text{fees} - \text{slippage}$$

First two terms cancel modulo basis drift. Capital allocation: deploy $C_\text{spot} = L \cdot C_\text{margin}$ for delta neutrality, total $C_\text{total} = C_\text{spot}(1 + 1/L)$. Higher leverage on short = more capital efficient but reduces liquidation distance.

**Worked example (BTC, 5×):** $100K spot + $20K perp margin → fully hedged. If $F = 0.01\%$/8h: daily yield = $0.01\% \times 3 \times \$100K = \$30$/day on $\$120K$ → 9.13% APR. Round-trip taker fees (4 × 0.04% = 0.16%) → ~5 days to break even.

### B. Reverse cash-and-carry

**Setup:** Short spot + long perp when funding negative. Retail rarely shorts spot directly; practical implementations:

1. Margin-borrow on Binance/Bybit/Coinbase, sell, long perp
2. Perp-only cross-exchange (see C)
3. Borrow stETH from Aave, sell to ETH, long ETH perp on Hyperliquid

Profitable iff $|F| > r_\text{borrow} + \text{fees}$.

### C. Cross-exchange funding arbitrage

Long perp on negative-funding venue, short perp on positive-funding venue. Net = $F_B - F_A > 0$.

- **Capital efficiency:** 100% margin-only
- **Risks:** inter-venue basis drift, transfer latency (Binance withdrawal ~30 min during stress), liquidation correlation

### D. Same-asset cross-pair

USDT-perp vs USDC-perp on the same exchange (Bybit, OKX). Coin-margined (inverse) vs USDT-margined (linear) — e.g., BTCUSD COIN-M vs BTCUSDT USDⓈ-M on Binance. Different funding regimes (different premium index, Asian retail bias toward inverse). Bonus: inverse funding paid in BTC = implicit BTC long.

### E. Cross-asset relative value

Long ETH perp / short BTC perp. Net delta ≠ 0; it's $\beta_\text{ETH/BTC} N_\text{ETH} - N_\text{BTC}$. Hedge ratio:

$$h = \rho_\text{ETH,BTC} \cdot \sigma_\text{ETH}/\sigma_\text{BTC}$$

Convexity from beta drift is hidden risk.

### F. Basis arbitrage with dated futures

Cash-and-carry vs quarterly futures (CME, Deribit, Binance dated):

$$\text{Basis APR} = \frac{F_T - S}{S} \cdot \frac{365}{T_\text{days}}$$

- **Advantage:** known terminal cash flow, no funding-rate-vol risk
- **Disadvantage:** 90-day roll cost (8-15bp round-trip on Binance/Deribit)

### G. Funding rate prediction (alpha-seeking)

- OI imbalance, taker buy/sell volume
- Premium drift (mark-oracle widening over last 30 min predicts next funding)
- Cross-venue lead-lag (Binance leads Bybit ~5-15 min)
- Bayesian: $P(F_{t+1} > F^* | \text{premium}_t, \text{OI}_t)$ trained on per-minute history
- Regression: $F_{t+1} = \alpha + \beta_1 P_t + \beta_2 \Delta\text{OI}_t + \beta_3 \text{LSR}_t + \epsilon$

### H. LST/LRT variants

**Stack 1:** long stETH + short ETH perp. PnL = $r_\text{stETH staking}$ + $F_\text{ETH-perp}$. ~3.2% staking + ~10% funding = ~13% APR target.

**Risks:**

- stETH/ETH peg breaks (May 2022 Celsius unwind: stETH down to 0.94 ETH)
- LRT smart-contract + AVS slashing
- Hedge tracking error during depeg

**Hedge correction:** $N_\text{ETH} = q_\text{stETH} \cdot \text{stETH/ETH ratio}$, recompute daily.

### I. Stablecoin yield stacking

USDe/sUSDe = productized cash-and-carry packaged as stablecoin. Stack: hold sUSDe (~9-12% APY funding+staking) + deploy capital separately into your own funding harvest. Or stack sDAI/sUSDS for 6-8% base on idle margin.

## 2.4 Execution math

### Position sizing

$$N_\text{notional} = C_\text{margin} \times L; \quad N_\text{spot} = N_\text{perp}; \quad C_\text{total} = N(1 + 1/L)$$

At 1× leverage, $C_\text{total} = 2N$ (inefficient, ~50% liq distance). At 10×, $C_\text{total} = 1.1N$ but ~10% adverse move liquidates.

### Hedge ratio for imperfect correlation

$$h^* = \frac{\text{Cov}(R_A, R_B)}{\text{Var}(R_B)} = \rho_{AB} \cdot \frac{\sigma_A}{\sigma_B}$$

### Delta-neutrality maintenance

As prices move, leverage drifts. If price rises by $\Delta P$:

$$L_\text{new} = \frac{N(1 + \Delta P/P_0)}{C_\text{margin} - N\Delta P/P_0}$$

Rebalance trigger: typically $L_\text{new} > 1.5 L_\text{target}$. Automated rebalancers fire at 25%/50%/75% to liquidation.

### Cross-margin vs isolated

- **Isolated:** contained risk, locked capital. Good for high-leverage alts
- **Cross:** account-level pool. Spot leg supports short perp leg → drastically less capital lockup
- **Portfolio margin** (Binance, OKX Unified, Bybit Unified): algorithmic spot+perp risk netting → 5-10× higher capital efficiency

### Liquidation distance

$$P_\text{liq, short} = E(1 + 1/L - m); \quad P_\text{liq, long} = E(1 - 1/L + m)$$

**Worked example:** short BTC perp at $E = \$70{,}000$, $L = 5\times$, $m = 0.5\%$:

$$P_\text{liq} = 70{,}000 \times (1 + 0.20 - 0.005) = \$83{,}650 \quad (+19.5\%)$$

Tiered margin: $m$ rises stepwise with size (Binance tier 1: $m = 0.4\%$ for ≤$50K; tier 6: $m = 5\%$ for >$50M).

### PnL decomposition

$$\Pi = \sum_t F_t N_t - \sum (\text{fee}_\text{m/t} \cdot \text{notional}) - \text{slippage} - r_\text{borrow} N_\text{borrowed} \Delta t$$

### Break-even funding

With Binance taker $f_t = 0.04\%$ + 2bp slippage/leg:

- Total cost = $4 \times 0.04\% + 2 \times 0.02\% = 0.20\%$
- At $F = 0.01\%$/8h, need 20 cycles → 6.7 days minimum

Strategies must hold ≫ break-even period.

### Annualization conventions

| Interval   | APR formula                  | F = 0.01% → APR        |
| ---------- | ---------------------------- | ---------------------- |
| 8h         | $F \times 3 \times 365$      | 10.95%                 |
| 4h         | $F \times 6 \times 365$      | 21.9%                  |
| 1h         | $F \times 24 \times 365$     | 87.6%                  |
| Continuous | $F_{8h} \times 3 \times 365$ | 10.95% (8h equivalent) |

## 2.5 Risk metrics

### Greeks adapted to perps

- **Delta:** $\Delta = \partial\Pi/\partial S \approx 0$ when properly hedged
- **Gamma:** linear long+short = 0. Inverse perps have nonzero gamma due to $1/P$ payoff: $\Gamma = -2/P^3$ per coin notional
- **Theta:** _the alpha_ — $\theta = F \cdot N$ per funding interval. Always positive on receiving side
- **Vega:** options-only directly. Indirect: realized vol drives funding-vol → high IV correlates with funding-spike risk

### Liquidation distance = primary risk metric

Maintain ≥ 2× the 99.9-percentile 24h move. BTC 99.9% 24h ≈ 8% in 2025-2026 → keep ≥16% liq distance.

### VaR

$$\text{VaR}_{99\%} = z \cdot \sigma_\text{basis} \cdot N$$

- BTC $\sigma_\text{basis} \approx 5$bp daily; alts 30-60bp
- With $N = \$1M$: BTC VaR ~$1,160/day; alt ~$14K/day

### Sharpe

Clean BTC cash-and-carry typically delivers Sharpe 2-4 in steady positive-funding regime; alts 1-2.

### Stress tests

- March 12, 2020: funding flipped +0.03% → -0.375% in 24h on BitMEX
- May 19, 2021: BTC perp basis -5%
- November 2022 (FTX): 100% loss
- August 5, 2024: -1%/8h on multiple alts
- October 10-11, 2025: the new benchmark

## 2.6 Comprehensive glossary

- **Perpetual contract / perp / perpetual swap:** Futures contract with no expiry, anchored to spot via funding. BitMEX 2016
- **Spot:** Direct asset ownership, T+0 settlement
- **Futures (dated):** Forward contract with fixed expiry; settles to spot at expiry
- **Funding rate ($F$):** Periodic interest-like rate transferred between longs/shorts
- **Funding payment:** $F \times N$
- **Funding interval:** 1h, 4h, or 8h
- **Mark price:** Fair value used for PnL/liquidation; index + smoothing
- **Index price:** Weighted average of underlying spot across reference exchanges
- **Last price:** Most recent traded price on the perp itself; differs from mark to prevent wick liquidations
- **Premium:** $(\text{Mark}-\text{Index})/\text{Index}$
- **Premium index:** Smoothed/depth-aware version using Impact Bid/Ask
- **TWAP:** Time-weighted average premium
- **Basis:** $F_\text{contract} - S_\text{spot}$. Sub-types: spot-perp, calendar
- **Contango:** Forward/perp > spot (positive basis)
- **Backwardation:** Forward/perp < spot (negative basis)
- **Cash-and-carry:** Long spot + short perp
- **Reverse cash-and-carry:** Short spot + long perp
- **Open interest (OI):** Total notional of open positions
- **Initial margin (IM):** Capital required to open
- **Maintenance margin (MM):** Threshold below which liquidation triggers
- **Cross-margin:** Account-level margin pool
- **Isolated margin:** Per-position allocation
- **Leverage ($L$):** Notional/posted margin. **Effective leverage** drifts with PnL
- **Liquidation:** Forced closure when equity < MM
- **ADL (Auto-Deleveraging):** When insurance fund can't absorb bankruptcy, most-profitable opposite-side traders auto-closed
- **Insurance fund:** Pool that absorbs bankruptcies before ADL
- **Long / Short:** Buyer/seller of contract
- **Maker / Taker:** Liquidity-provider limit order vs liquidity-taker market order. Maker fees often rebated
- **Slippage:** Price impact between expected and realized fill
- **Market impact:** Cost of executing notional, scales as $\sqrt{N}$
- **Latency:** Round-trip from submit to ack
- **Order book depth:** Cumulative liquidity at price levels
- **Skew, OI imbalance, long/short ratio:** Sentiment/positioning metrics
- **vAMM:** Synthetic constant-product price curve, no underlying reserves (Drift v1, Perp Protocol). Math is $xy=k$
- **Oracle:** External price feed (Pyth, Chainlink) for mark/funding
- **Mark-price manipulation:** Wicking last price to trigger liquidations; mitigated by index-anchored mark
- **L1/L2/sequencer/rollup:** Base chain / Layer 2 / centralized orderer / off-chain execution
- **USDⓈ-M:** USDT/USDC-margined linear perp
- **COIN-M:** Coin-margined inverse perp
- **Inverse:** $\Delta V_\text{coin} = Q(1/P_0 - 1/P_1)$
- **Linear:** $\Delta V = Q(P_1-P_0)$
- **Settlement:** Booking realized PnL
- **MTM (mark-to-market):** Periodic revaluation of unrealized PnL
- **Tier system:** Notional-based scaling of margin requirements
- **Sub-account / master account:** Hierarchical accounts for strategy/risk segregation
- **Portfolio margin:** Cross-asset risk netting in one account
- **LST:** Liquid staking token (stETH, rETH, mETH)
- **LRT:** Liquid restaking token (weETH, ezETH, pufETH)
- **Stablecoin / depeg:** Token pegged to fiat / trading materially off peg
- **Unified margin:** One account, multiple collateral types
- **Predicted vs realized funding:** Rolling estimate displayed pre-settlement vs value locked at funding timestamp
