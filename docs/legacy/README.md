# Crypto Funding Rate Arbitrage — Deep Research Report (May 2026)

> Synthesized from four parallel Opus 4.7 max-effort research agents, each running independent web research. All non-trivial claims are cited inline within the section files. Uncertainty is flagged explicitly. Treat any specific number that's not from a primary venue/dashboard as approximate.

## Files in this report

| File                                                                     | Topic                                                                                                                                             |
| ------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------- |
| [01-market-landscape.md](./01-market-landscape.md)                       | 2026 opportunities, capital sizing, risks, realistic returns, notable events                                                                      |
| [02-math-and-mechanics.md](./02-math-and-mechanics.md)                   | Perp fundamentals, per-exchange formulas, strategies, execution math, vocabulary                                                                  |
| [03-tech-stack.md](./03-tech-stack.md)                                   | Languages, frameworks, networking, hardware, infra, DEX tech, the 2026 reference stack                                                            |
| [04-comparative-and-operational.md](./04-comparative-and-operational.md) | Funding arb vs other yield strategies, Ethena, predictive alpha, tax & jurisdictions                                                              |
| [05-tutorial-building-a-bot.md](./05-tutorial-building-a-bot.md)         | **Build tutorial:** zero-to-bot walkthrough for cross-exchange funding arb on Binance + Bybit (engineer audience, no finance background required) |
| [sources.md](./sources.md)                                               | Consolidated source URLs and uncertainty flags                                                                                                    |

## Executive summary

Three findings dominate everything in this report:

1. **The trade is alive but fundamentally compressed.** 2024 funding-rate yields of 25-50% APY on majors are gone. 2026 reality: ~5-10% APY all-in on BTC/ETH at scale, often _below_ T-bills net of taxes for high-bracket US persons. Three forces drove the compression:
   - Ethena's $5-15B systematic short book absorbing the structural retail-long imbalance
   - ETF basis-trade infrastructure scaling post-Jan 2024
   - Tokenized Treasuries (BUIDL, USDY) raising the risk-free floor

2. **Tail risk has not compressed.** The Oct 10-11, 2025 cascade ($19B liquidations, 1.62M accounts wiped, USDe locally depegged to $0.62 on Binance, ADL closing winners' hedge legs) demonstrated that "delta-neutral" assumes both legs survive intact through stress — increasingly contingent on which venues you use and your custody arrangement.

3. **The technical bar to compete is no longer "language choice."** Open source covers 90% of what you need (NautilusTrader, CCXT Pro, Hummingbot, barter-rs). Real differentiation is now in:
   - Capital allocation across 5-15 venues
   - Off-exchange settlement (Copper / Ceffu / Fireblocks)
   - Tokenized-Treasury collateral on perp DEXes
   - Tax-residency optimization

## Bottom line by capital tier

| Capital   | Annualized net edge over T-bills | Comment                                 |
| --------- | -------------------------------- | --------------------------------------- |
| <$10K     | likely **negative or net-zero**  | Withdrawal/transfer fees consume edge   |
| $10-100K  | 0-3% over T-bills                | Marginal; one mistake erases a year     |
| $100K-1M  | 2-6% over T-bills                | Worthwhile for serious individuals      |
| $1-10M    | 3-8% over T-bills                | Sweet spot for ops-light funds          |
| $10-100M  | 4-7% over T-bills                | Capacity friction begins                |
| $100M-1B+ | 3-6% over T-bills                | Capacity binding; alt rotation required |

For a sophisticated operator in 2026: **5.5-7.5% all-in pre-tax** is realistic. Top-quartile institutional with prime-broker plumbing, BUIDL/USDY collateral, and alt rotation: **6-12% all-in, Sharpe 1.5-2.5.**

20%+ "delta-neutral" claims in 2026 are either rotating heavily into illiquid alts or accepting hidden tail risks (USDe depeg, ADL, oracle manipulation, smart-contract).

## How to read this

- Start with **01-market-landscape.md** for context on the current environment, what's happening, and what's at stake.
- Then **02-math-and-mechanics.md** for the formulas, strategies, and vocabulary you need to actually understand and execute.
- Then **03-tech-stack.md** for what to build with.
- Then **04-comparative-and-operational.md** for whether you should bother (vs alternatives, after taxes).
- **sources.md** has all the citation URLs and uncertainty caveats.

## Methodology note

Four research agents ran in parallel using Opus 4.7 max-effort, each tasked with one of the four parts. They independently used WebSearch and WebFetch to verify formulas against current exchange docs, pull recent market data, check GitHub stats and last-commit dates, and search academic literature. Citations are preserved from agent output. Where agents could not verify a specific number live (e.g., May 9 2026 intraday funding rate), the regime is described qualitatively and the gap is flagged.
