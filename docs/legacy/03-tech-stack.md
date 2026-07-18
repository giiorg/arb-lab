# Part 3 — Technical State of the Art

## 3.1 Programming languages

| Language              | Crypto-arb role                                                  | Strengths                                                            | Weaknesses                                      |
| --------------------- | ---------------------------------------------------------------- | -------------------------------------------------------------------- | ----------------------------------------------- |
| **Rust**              | Hot path, full systems (Hyperliquid core, Drift, NautilusTrader) | Memory + thread safety, zero-cost abstractions, mature `tokio` async | Steep learning curve, long compile times        |
| **C++**               | Matching engines, tier-1 HFT                                     | Lowest controllable latency, mature ecosystem                        | Manual memory mgmt; talent scarcity for new C++ |
| **Go**                | dYdX v4 chain, GoCryptoTrader                                    | Fast iteration, GC-friendly                                          | GC pauses unsuitable for true HFT               |
| **Python**            | Research, backtesting, control plane                             | Largest ecosystem (NumPy, pandas, asyncio)                           | GIL, slow hot path without Cython/PyO3          |
| **Julia**             | Quant research niche                                             | Math-first syntax, JIT speed near C                                  | Small crypto library ecosystem                  |
| **Java/Kotlin/Scala** | Cumberland, GSR                                                  | JVM tooling (Aeron, Chronicle)                                       | GC tuning is its own discipline                 |
| **OCaml**             | Jane Street                                                      | Strong static guarantees                                             | Niche talent pool                               |

### Where each is actually used

- Hyperliquid HyperCore is Rust
- Drift is Rust on Solana
- dYdX v4 validator is Go on Cosmos SDK
- NautilusTrader has migrated nearly all hot-path components to Rust core with Python control plane (22.6K stars)
- Wintermute mixes Python+Rust
- Jane Street's OCaml extends to its crypto desk
- Cumberland (DRW) and GSR remain C++/Java-heavy from TradFi heritage

### Why Rust keeps winning

Async runtimes (`tokio`, `tonic`, `axum`) made multi-venue WebSocket consumers ergonomic without sacrificing throughput; memory safety eliminates whole classes of crashes that required exhaustive testing in C++. Reth (5.6K stars) and Erigon migration illustrate the broader pattern.

### Why Python dominates research

vectorbt (7.5K stars) lets you grid-search thousands of parameter combos at C speed via Numba, then drop to PyO3-bound Rust for live trading. CCXT (42.4K stars) and CCXT Pro give immediate access to ~100 exchange REST + WebSocket APIs in unified form. **Standard 2026 sequence: research in Python → port hot path to Rust.**

## 3.2 Open-source frameworks (verified GitHub stats, May 2026)

| Project                              | Lang    |  Stars | License     | Last push  | Funding-arb fit                                      |
| ------------------------------------ | ------- | -----: | ----------- | ---------- | ---------------------------------------------------- |
| **ccxt/ccxt**                        | Python  | 42,353 | MIT         | 2026-05-09 | Excellent connectivity; default research choice      |
| **freqtrade/freqtrade**              | Python  | 50,013 | GPL-3.0     | 2026-05-09 | Spot-focused; not ideal for cross-venue funding arb  |
| **hummingbot/hummingbot**            | Python  | 18,499 | Apache-2.0  | 2026-05-08 | **Has dedicated Funding Rate Arbitrage V2 strategy** |
| QuantConnect/Lean                    | C#      | 18,879 | Apache-2.0  | 2026-05-08 | Strong backtest-live parity                          |
| **nautechsystems/nautilus_trader**   | Rust+Py | 22,604 | LGPL-3.0    | 2026-05-09 | **Best-in-class for institutional-grade crypto arb** |
| jesse-ai/jesse                       | JS+Py   |  7,853 | MIT         | 2026-05-08 | Crypto-native; spot focus                            |
| polakowo/vectorbt                    | Python  |  7,452 | Custom      | 2026-04-25 | Premier vectorized backtesting                       |
| bmoscon/cryptofeed                   | Python  |  2,810 | NOASSERTION | 2026-02-01 | WS aggregator with funding-rate channels             |
| barter-rs/barter-rs                  | Rust    |  2,111 | MIT         | 2026-04-10 | Event-driven Rust; lower-level than Nautilus         |
| thrasher-corp/gocryptotrader         | Go      |  3,430 | MIT         | 2026-05-05 | Mature Go option                                     |
| hyperliquid-dex/hyperliquid-rust-sdk | Rust    |    445 | MIT         | 2025-10-21 | Official; signing/nonce handled                      |
| drift-labs/protocol-v2               | TS+Rust |    392 | Apache-2.0  | 2026-05-07 | On-chain program + TS SDK                            |
| dydxprotocol/v4-chain                | Go      |    326 | NOASSERTION | 2026-05-01 | Validator + clients                                  |

### Hummingbot Funding Rate Arbitrage strategy

Introduced v1.27.0 (April 2024). Scans configured perpetual connectors, evaluates pairwise funding-rate differentials, opens offsetting long/short. Supports Hyperliquid, dYdX v4, Binance perp, Bybit, Gate.io, Kucoin, OKX, Bitget. Configurable `position_size_quote`, `min_funding_rate_profitability`, `take_profit_per_funding_collected`, `tokens` whitelist. Slippage approximated via top-of-book + `slippage_buffer`. Known issues include partial double-fill bugs against Hyperliquid (issue #7295) — production deployments wrap with their own reconciliation watchdog.

### NautilusTrader

Strongest single open-source choice: deterministic event-driven Rust core with backtest-live semantic parity. The same strategy code runs against historical Parquet/in-memory data and live exchange feeds with identical event semantics. Nanosecond resolution, optional Redis-backed state persistence, ports-and-adapters pattern. Its backtester is the only open-source one simulating multiple venues, instruments, and strategies simultaneously with order-book reconstruction.

### barter-rs

Lower-level: composable crates (`barter-data`, `barter-execution`, `barter-instrument`, `barter-integration`). Choose this if you want full Rust control without Nautilus's opinions.

### CCXT Pro

Adds WebSocket to CCXT with auto rate-limiting and reconnection. Supports `watchFundingRate` on most venues. Bundled with `ccxt` as of late 2024.

### cryptofeed

Most popular pure-Python WS aggregator with synthetic NBBO across configured venues; normalized callbacks for trades, L2/L3, candles, OI, funding rates.

### Notable gap

None of the open-source frameworks ship a _production_ funding-arb backtester combining sub-second order-book replay + historical funding ticks + realistic taker/maker accounting + cross-venue capital allocation. Most teams build this in-house on top of vectorbt, NautilusTrader, or kdb+/Python.

## 3.3 Exchange connectivity

### WebSocket vs REST

WS mandatory for live arb (REST adds 100-300ms jitter, rate limits at any meaningful symbol count). REST remains essential for snapshots, account/wallet calls, transfers.

### FIX protocol

- **Binance FIX API** supports SBE encoding in order entry as of late-2025; separate FIX Drop Copy (60 msg/min) and Market Data (2000 msg/min) sessions; requires Ed25519 keys; rolled out Order-Amend-Keep-Priority in April 2025
- **Coinbase International Exchange** offers FIX for both order entry and market data
- Still the exception, not the norm

### Native SDKs

- Binance `binance-spot-connector-{python,java,...}`
- Bybit `pybit`
- OKX `python-okx`
- Hyperliquid official Python and Rust SDKs handle msgpack signing of L1 actions and EIP-712 admin actions
- Coinbase Pro Node SDK deprecated since 2020

### Order book maintenance pattern

1. Subscribe to delta channel before fetching snapshot
2. Buffer deltas
3. Fetch REST snapshot
4. Replay buffered deltas where `update_id > snapshot.lastUpdateId`

**Pitfalls:**

- Binance `firstUpdateId/finalUpdateId` semantics differ across spot vs USDⓈ-M perp
- Bybit snapshot delivered through WS itself
- Hyperliquid uses different L4 book

### Time sync

NTP (chrony) minimum. PTP via hardware-timestamping NIC + GPS grandmaster gives sub-microsecond, but overkill for funding-arb. What matters is that you trust _exchange's_ event timestamps over yours, and reconcile within ~50ms.

### Rate limits

- Binance weighted (1200 req-weight/min for most endpoints)
- Bybit per-endpoint token buckets
- Hyperliquid per-IP and per-address

Implement client-side governors and reactive backoff on 429/418.

## 3.4 Networking and latency

### Co-location reality

True co-lo doesn't exist for most crypto venues — co-locate by being in the same AWS region.

| Venue           | Optimal location                                                    | Notes                                                  |
| --------------- | ------------------------------------------------------------------- | ------------------------------------------------------ |
| Binance USDⓈ-M  | AWS ap-northeast-1 (Tokyo)                                          | RTT 5-12ms typical                                     |
| Bybit           | AWS ap-southeast-1 (Singapore), some Tokyo                          |                                                        |
| OKX             | AWS ap-east-1 (HK) / Singapore                                      |                                                        |
| **Hyperliquid** | **AWS ap-northeast-1 (Tokyo)** — all 24 validators clustered        | **Tokyo traders get ~200ms edge** (CoinDesk, Mar 2026) |
| dYdX v4         | Cosmos network — distributed; co-locate near a high-stake validator |                                                        |
| Drift           | Solana RPC — Frankfurt/Amsterdam hot for Pyth                       |                                                        |

For Hyperliquid: end-to-end median order placement ~200ms with 99p ~900ms — server-side processing dominates over network at sub-second timescales.

### Kernel bypass

DPDK ~850ns P50 vs Linux sockets ~18μs. AF_XDP and io_uring fall in between. For funding arb, the _exchange's_ matching round-trip dominates — kernel bypass rarely the bottleneck unless you're competing in passive market-making at the same venue. **io_uring is the easy win** — most modern Rust async runtimes (and Python's asyncio via liburing) can use it without code changes.

### TCP tuning

- `TCP_NODELAY` always on for order entry
- BBR over CUBIC for data feeds
- Jumbo MTU only inside your VPC

### Hardware timestamping NICs

Intel E810, Mellanox CX-5/CX-6, Solarflare X2/X3 with Onload. Standard in TradFi HFT; for crypto-arb, only the most aggressive market-makers on Binance/OKX deploy.

### Network monitoring

eBPF tools (`bpftrace`, Cilium Hubble) for live latency histograms; `tcpdump` + Wireshark for FIX forensics; `chronyc sources` for clock disciplining.

## 3.5 Hardware

Typical institutional crypto-arb node (2026):

- **CPU:** AMD EPYC 9005-series or Intel Xeon 6700/Sapphire Rapids; 32-64 cores; `isolcpus`, CPU pinning, NUMA awareness, `SCHED_FIFO`
- **RAM:** 128-512 GB DDR5; full order book + L2 deltas in RAM
- **NIC:** 25/100 GbE; hardware timestamping mandatory if FPGA, optional otherwise
- **Storage:** NVMe Gen4/Gen5; ZFS/XFS

### FPGA in crypto

Hudson River Trading is publicly known to deploy FPGA broadly across markets; one of the largest crypto liquidity providers since 2017. Jump Crypto's FPGA in crypto specifically: rumored, not publicly confirmed. **For funding arb (multi-second-to-multi-hour timescale), FPGA is overkill** — value is in true HFT.

### GPUs

Not relevant in execution path. Useful for batch backtests (CUDA-accelerated vectorbt PRO) and ML signal training.

### Time-sync hardware

Endrun Tempus LX or Oscilloquartz GPS-disciplined PTP grandmasters give sub-μs accuracy. An Intel i210 NIC at ~$30 supports hardware PTP at the edge.

## 3.6 Infrastructure patterns

### Architecture

Modular monolith inside one colo'd box; microservices at WAN edge. Hot-path order pipeline = one process (or tightly cooperating processes via shared memory or Aeron IPC); risk, position reconciliation, P&L, observability run as separate services.

### OMS

Pre-trade risk in-process; post-trade reconciliation as separate service hitting a position-tracking store (Redis live + Postgres/ClickHouse history). Every order has idempotency key (clientOrderId).

### Backtest-live parity

NautilusTrader's deterministic event core. For Python-only stacks, vectorbt + custom event-replay shim is the common pattern.

### Observability

- Grafana + Prometheus metrics
- Loki structured logs
- OpenTelemetry traces
- PagerDuty + Slack webhooks

### Time-series storage

| DB                   |       Stars | Strength                                                 |
| -------------------- | ----------: | -------------------------------------------------------- |
| ClickHouse           |      47,305 | Best query speed for ad-hoc OLAP; default for new builds |
| QuestDB              |      16,929 | High-throughput streaming ingest                         |
| TimescaleDB          |      22,600 | Postgres-native, easy ops                                |
| kdb+/KDB-X           | proprietary | Still fastest at extreme-scale tick storage              |
| ArcticDB (Man Group) |       3,090 | Pandas-native; great for Python research                 |
| Parquet + DuckDB     |         n/a | Cheap immutable archives; default modern research format |

### Message queues

- **Aeron** (8.6K stars; 29μs RTT on AWS c6in.16xlarge) — default for cross-host trading messaging in finance
- **Chronicle Queue** (3.7K) — sub-μs intra-host persistence
- **NATS** (19.8K) — non-trading event distribution
- **ZeroMQ** (10.9K) — when "good enough" is good enough
- Redis Streams at venture-stage shops

## 3.7 DEX-specific tech

### Hyperliquid (HyperEVM + HyperCore)

Two interleaved layers under HyperBFT consensus. HyperCore is Rust, runs the order book on-chain _without smart contracts_, sub-second blocks. Validators must hit <200ms two-way latency to one-third stake or be jailed. L1 action signing uses msgpack-encoded EIP-712-like domain ("Exchange") with strict ms-resolution nonces.

### dYdX v4

Cosmos SDK chain, Go binary, custom `x/clob` module with in-memory order book on every validator, separate Indexer service (Postgres + Redis). Orders gossiped via CometBFT mempool; matching in-block.

### Drift

Solana program in Rust with "Liquidity Trifecta" of CLOB + vAMM + just-in-time auction. v3 (Dec 2025) shipped 10× faster fills with gasless trading default. Funding from mark-vs-oracle premium, paid hourly.

### Aevo / Vertex

- **Aevo** = custom L2 derivatives venue with off-chain CLOB + on-chain settlement, hourly funding
- **Vertex** unifies spot/perp on Arbitrum and Base, hourly funding capped 10%/day

### SynFutures V3 / Entropy

Oyster AMM hybrid (concentrated liquidity + on-chain orderbook); Entropy 2026 made fully on-chain order books viable at scale.

### RPC infrastructure

- **Solana:** Yellowstone gRPC (Triton, Helius, Quicknode, Chainstack, or self-hosted) — modern standard for low-latency state and shred-level streams
- **EVM:** Erigon (3.6K stars) and Reth (5.6K stars) self-hosted; Reth 2.0 (April 2026) production-grade

### MEV

- **Solana:** Jito bundles de facto required (>22% of validator rewards from Jito tips Q1 2025)
- **Ethereum:** Flashbots, private mempool, MEV-Share

### Bridging

- **Across** (intent-based, ERC-7683, 2-30s clears)
- **LayerZero OFT** (canonical movement across 150+ chains)
- **Wormhole** (general messaging)
- **Squid / Li.Fi** (user aggregators)

For arb desk shifting capital between Hyperliquid (USDC native), dYdX v4 (USDC), Drift (USDC), CEXes, the "Across-then-CCTP-then-deposit" flow is the standard 2026 pattern.

### EIP-7702 (Pectra May 2025)

~14M EOAs have signed at least one 7702 authorization. For arb, the relevant unlock is _batched calls_ — single signed delegation can submit `permit + deposit + open_position` atomically.

## 3.8 Deployment and ops

### AWS vs bare metal vs hybrid

- AWS for venues clustering own infra in AWS regions (Binance, Bybit, OKX, Hyperliquid)
- Equinix bare metal (TY3, NY4, SG1, LD4) for cross-region aggregation, FIX co-lo with TradFi venues, self-hosted Solana/EVM nodes needing full bandwidth
- **Hybrid is the rule**

### Containerization

Kubernetes for non-hot services (analytics, indexer, API). Bare process or systemd-managed binaries for hot path — k8s networking adds 50-200μs.

### Secrets

HashiCorp Vault, AWS Secrets Manager/KMS, or `age`-encrypted files for smaller setups. Exchange API keys must be IP-whitelisted (every major venue supports).

### Sub-account architecture

One sub-account per strategy per venue, separate API keys per sub-account, separate IP whitelist per key. Cut a misbehaving strategy without taking down the desk.

### Wallet hygiene

Hot wallets on dedicated HSMs (Fireblocks, Copper, Ledger Enterprise); multisig (Safe) for treasury; separate signing key per chain.

## 3.9 Software development practices

### Backtesting requirements specific to funding arb

1. Per-venue tick-level funding rate history (Tardis.dev, Coinglass, Coinalyze)
2. Per-venue maker/taker fee schedules including VIP discounts
3. Order book at funding tick boundaries to model entry slippage
4. Cross-venue latency model
5. Capital cost model — your USDC parked on venue X earns 0% while it could earn 4% on Aave

### Risk limits

Max gross notional, max per-symbol, max venue concentration, daily P&L kill switch (-2σ vs backtest), max drawdown stop, automatic flatten on staleness > N seconds.

### Property-based testing

Hypothesis (Python) or proptest (Rust) for orderbook invariants — catches 90% of book-corruption bugs.

### Deterministic simulation

NautilusTrader and barter both support replay against captured event streams — standard for race-condition forensics.

### Deployment

Blue/green rare in trading. Common pattern: canary on small notional. Deploy new build to sub-account at 1% normal size, monitor 1-3 funding cycles, then promote.

## 3.10 Practitioner stack examples

| Firm                 | Stack                                                                                                                                        | Source                    |
| -------------------- | -------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------- |
| Wintermute           | Python + Rust core; Wintermute Node OTC platform                                                                                             | publicly known            |
| Jane Street          | OCaml top-to-bottom; some Rust; in-house                                                                                                     | janestreet.com/technology |
| Cumberland (DRW)     | Java + C++ from TradFi                                                                                                                       | publicly known            |
| GSR                  | Python research + Java/C++ execution                                                                                                         | publicly known            |
| B2C2                 | Java/Kotlin OTC infra                                                                                                                        | publicly known            |
| Hudson River Trading | C++ / Rust + FPGA; major BTC/ETH ETF LP                                                                                                      | hudsonrivertrading.com    |
| Hummingbot Inc       | Python (their open product) + Rust components                                                                                                | their repos               |
| **Ethena Labs**      | **Off-chain delta-neutral hedging engine; runs short perps on Binance/Bybit/OKX/Hyperliquid; collateral in OES via Copper/Ceffu/Fireblocks** | docs.ethena.fi            |
| Retail prop shops    | Python + CCXT, NautilusTrader, or Rust + tokio                                                                                               | community                 |

**Ethena specifically:** collateral never sits on the exchange (eliminates Bybit-hack-style loss vectors); hedge engine reroutes to other venues automatically when one is degraded. **They survived the Feb 2025 Bybit hack with no fund loss for exactly this reason.**

## 3.11 Backtesting data sources

| Source          | Coverage                                                                                                                | Pricing     | Best for                                   |
| --------------- | ----------------------------------------------------------------------------------------------------------------------- | ----------- | ------------------------------------------ |
| **Tardis.dev**  | Tick L2/L3, trades, funding, liquidations across BitMEX, Deribit, Binance, OKX, Bybit, Bitfinex, Kraken, Coinbase, Gate | Paid        | Highest-fidelity historical reconstruction |
| Coinglass       | L2/L3, funding, OI, liquidation heatmaps, 30+ exchanges                                                                 | Tiered API  | Funding-rate cross-venue history           |
| Coinalyze       | Funding rate aggregation                                                                                                | Free + paid | Fast lookup                                |
| Kaiko           | Institutional-grade L1/L2 normalized                                                                                    | Enterprise  | Compliance-friendly                        |
| CoinAPI         | Real-time + historical L1/L2                                                                                            | Tiered      | Drop-in feed normalization                 |
| Exchange-native | Binance Vision (free tick), Bybit historical CSVs                                                                       | Free        | Cost-conscious teams                       |

## 3.12 Recent innovations (2024-2026)

- **LLM-driven research:** Claude Opus and GPT-5 increasingly used in Jupyter to generate strategy hypotheses, parameter grids, risk-limit suggestions; production gated by traditional backtest/canary discipline. Fully autonomous parameter-tuning agents experimental (Hummingbot Synthia for SynFutures).
- **HRT's AI cluster:** Hudson River Trading partnered with Google Cloud 2025 to build AI compute for trading research — shift from pure speed to "smartest model."
- **New L2 perp DEXes:** SynFutures Entropy, Aori, Drift v3 pushing fills toward 50-100ms.
- **EIP-7702 atomic execution:** smart-EOA delegations enable batched `permit + deposit + open` flows in one signed tx.
- **Tokenized money-market collateral:** USDe, USDS, sDAI, stable yield tokens accepted as derivatives collateral on Hyperliquid, Drift, major CEXes — your idle margin earns ~5% instead of 0%. **Single most important capital-efficiency development of 2026.**
- **Off-exchange settlement (OES) custodians:** Copper ClearLoop, Fireblocks Off-Exchange, Ceffu MirrorX — collateral never leaves custody. "Exchange default" risk transformed from existential to operational.
- **Hyperliquid as strategic venue:** ~5ms internal network and hourly funding made it default leg in cross-venue funding portfolios; Tokyo geographic edge driving institutional teams to relocate AWS workloads to ap-northeast-1.

## 3.13 The 2026 reference stack

State-of-the-art crypto funding-rate arb stack, May 2026:

```
Hot path (Rust)
  └─ NautilusTrader or in-house tokio engine
  └─ CCXT Pro or per-venue native SDK (Hyperliquid Rust, Drift TS/Rust, dYdX Go)
  └─ io_uring async
  └─ Aeron / Chronicle Queue messaging

Research environment (Python)
  └─ vectorbt PRO + ArcticDB
  └─ Tardis.dev historical data
  └─ ClickHouse or kdb+ for tick

Deployment
  └─ AWS Tokyo (ap-northeast-1) primary for Binance / Hyperliquid
  └─ Equinix bare-metal complement for cross-region
  └─ Yellowstone gRPC + Reth/Erigon multi-region for DEX legs
  └─ OES custody (Copper / Fireblocks / Ceffu) for CEX collateral

Above engine
  └─ LLM-assisted research (Claude Opus, GPT-5)
  └─ EIP-7702 atomic execution
  └─ Tokenized yield-bearing collateral (BUIDL, USDY)
```

The hard problems are no longer language choice or basic connectivity. They are:

1. **Capital allocation across 5-15 venues** with real cross-venue latency
2. **Exchange default risk** (Bybit hack, FTX echoes) → OES
3. **Hedge slippage during volatility events** (Oct 2025 USDe washout demonstrated thin liquidity matters)
4. **Operational reliability** — funding arb is a yield product, and yield products that crash lose investor trust faster than they earn it
