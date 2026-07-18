# Funding-Rate Arbitrage Platform — Engineering Build-Spec (MVP)

**Status:** Draft v0.5.1 (2026-07-18) — architecture resolved & validated across Codex adversarial rounds 1–4 (11 → 6 → 4 → 4 findings; round 4 was a coherence gate that caught v0.5 integration bugs, now fixed). Converged; ready for external human review → implementation.
**Relation to other docs:** `docs/legacy/` is the frozen research record (the "why"). `docs/review/2026-07-18-05-tutorial-critical-review.md` is the critical review this spec resolves. `docs/tutorial.md` is the teaching companion. This spec is the normative "what to build."
**Conventions:** MUST / SHOULD / MAY are normative. Requirements that resolve a review finding are tagged `[→ P#/C#/A#/S#/B#]`; architecture decisions `[→ D#]`; Codex-round-1 resolutions `[→ X#]`. A traceability table is in the Appendix.
**House style:** Go, **DDD + hexagonal (ports/adapters)**, mirroring the reference backend (`bitnet-backend-go`: bounded contexts, `internal/{boundary,engine,platform}`, ports in `application/ports/`, `infrastructure/ = driven/ + driving/`, compile-time port assertions, explicit `module.go` wiring — no DI container) **and** NautilusTrader's engine design (single-writer execution/portfolio state, central event dispatch, deterministic backtest-live parity). Open-source frameworks are **reference/inspiration only — never dependencies**; `internal/platform/*` is reimplemented fresh, not imported.

---

## 1. Product scope & non-goals

**Goal.** A multi-exchange funding-rate arbitrage platform. The platform is a **strategy-agnostic foundation** that runs pluggable, parametrized **strategies**. The MVP proves the foundation end-to-end with **cash-and-carry** on **Binance + Bybit**, built **N-venue-ready** from day one.

**MVP scope.**
- Venues: Binance (spot + USDⓈ-M linear perp) and Bybit (spot + linear perp). Adding a venue = implement one adapter (§4).
- Strategy #1 (proven first): **cash-and-carry** — long spot + short perp on the same venue, harvesting positive funding.
- Strategy #2 (next): **cross-exchange spread** — long perp on the low-funding venue, short perp on the high-funding venue. Scoped to alt pairs / dislocation windows, **not** majors `[→ P1]`.
- Real capital, live (eventually). Correctness, **durable order/account truth**, custody/key security, reconciliation, and money-losing edges are ship-blockers `[→ risk bar, X1]`.

**Explicit assumptions / scope boundaries** (previously scattered in the tutorial — now normative) `[→ pedagogy]`:
- **Deployment shape:** a **single-process engine** for MVP — all bounded contexts run in one `arblab-bot` binary over an in-process dispatcher (§3) `[→ D1]`. N-service split is a documented later path (§3.7), not built.
- **Authoritative state:** one context — `oms` — is the **single writer** of order/fill/trade/position truth, backed by a durable **write-ahead journal**. No other context mutates that state `[→ X1, X3]`.
- **Contracts:** linear USDⓈ-M / USDT-margined perps only in MVP. Inverse (COIN-M) perps are out of MVP scope; the adapter still carries a `ContractType` so neutrality math can branch later `[→ C6]`.
- **Position mode:** One-way mode on both venues (single net position per symbol). The venue nets all strategies' exposure per symbol, so the OMS enforces an exclusive lease per `(account, venue, symbol)` unless explicit allocation accounting is built `[→ X3]`.
- **Margin collateral:** plain **USDT or USDC only** (the Oct-2025 USDe local-mark failure is the reason) `[→ S1]`.
- **Money math:** the `platform/fixedpoint` scaled-integer decimal type **everywhere** — no `float64` in any price/size/notional/margin/rate/fee path, lint-enforced. Intermediates use 128-bit/`big.Int`; values are wrapped in **dimensional types** (`Money{Currency}`, `Quantity{Asset}`, `Price{Base,Quote}`, `Rate`) so unit mismatches fail to compile `[→ D3, X7]`.
- **Time:** exchange **server time is authoritative**; the injected `Clock` models per-venue server-clock offset + uncertainty. Domain/application code reads time only from the `Clock` (§3.5), never `time.Now()` `[→ D4, X8]`.

**Non-goals (MVP).** True HFT / co-location / kernel-bypass; options/vol overlays; DEX/perp-DEX legs; portfolio-margin optimization; automated tax filing (emit records only); multi-process/NATS deployment (seam documented §3.7, not built); cross-strategy netting on a shared `(venue,symbol)` (excluded by the exclusive lease, §3.2) `[→ X3]`.

---

## 2. Strategy model

### 2.1 The edge, stated honestly `[→ P1, P2, P3]`

Two different trades with different edge magnitudes; the foundation supports both, MVP leads with the first:

- **Funding *level* (cash-and-carry).** Long spot + short perp on one venue collects that venue's funding when it is positive. This is the ~5–10% APY the research documents (what Ethena industrializes). The **level** is the edge.
- **Funding *spread* (cross-exchange).** Long/short perps across two venues collects `F_high − F_low`. On majors this spread is sub-bp to low-single-digit-bp/8h and mean-reverts within hours, while round-trip cost is ~0.29% — so it rarely clears break-even for a realistic hold. Therefore cross-exchange is scoped to **alt pairs and dislocation windows** with a much higher edge bar.

**Hurdle rate.** Net APY MUST be evaluated against the on-chain risk-free floor (tokenized T-bills ~4.6%), not zero `[→ P2]`. Report strategy return on the **full allocated bankroll**, never on posted margin `[→ 4.2 of review]`.

**Tail reality — stated precisely `[→ P3, X2]`.** ADL is a *realized* hedge-failure mode. The subtle part: in the Oct-2025 crash the exchange force-closed **profitable short** legs (ADL), leaving desks **naked long into a still-falling market** — the hedge vanished exactly when it was needed. So cash-and-carry's advantage is narrower than "spot offsets the loss":
- The **spot leg cannot itself be ADL'd or liquidated** — you own it outright, so you never lose the whole position involuntarily.
- But a **force-closed perp short still leaves directional exposure** that MUST be detected from a private account/ADL event and **auto-rehedged or flattened immediately** (§7.4). Waiting for the next monitor tick is too slow.
- Sizing MUST budget the **naked-leg price gap + reacquisition slippage** during the unhedged window, not merely "budget for ADL" `[→ X2]`.

### 2.2 Strategy #1 — Cash-and-carry (MVP flagship)

- **Setup:** long `q` spot + short `q` perp, same venue, same asset. Net delta ≈ 0.
- **PnL:** `Σ funding_received − fees − slippage − borrow(if any) ± basis_drift`. The spot and perp price terms cancel modulo spot-vs-perp basis.
- **Capital:** `C_total = N + N/L_perp = N(1 + 1/L_perp)` where `N` = notional, `L_perp` = perp leverage. At 3× perp leverage, ~1.33·N. Lower capital efficiency than perp-perp, but no inter-venue basis risk and no second counterparty for the hedge.
- **Enters when:** venue funding is positive and expected forward funding over the hold clears (round-trip cost + hurdle) with margin `[→ C2, C3]`.
- **Primary risk:** **negative-funding regimes** (e.g., the 46-day negative streak in 2026) turn the carry negative → the strategy MUST detect and exit/stand-down, not hold through `[→ 04 negative-regime]`. Secondary: perp-short liquidation on a sharp rally → rebalance margin from pre-funded rescue collateral (§7.1); monitor liq distance from live account/margin events.

### 2.3 Strategy #2 — Cross-exchange spread (post-MVP)

- **Setup:** long perp on lower-`F` venue, short perp on higher-`F` venue, equal base quantity `[→ delta residual math]`.
- **Scope:** alt pairs and dislocation windows only; edge bar set from measured spread persistence, not the headline spread `[→ P1]`.
- Inherits the full two-venue execution/one-leg-failure machinery (§6) — which is exactly why it is valuable to build the foundation for it even though MVP leads with cash-and-carry.

### 2.4 Break-even & sizing math — single-sourced `[→ C1, C2, C3]`

- **One source of truth for cost.** Round-trip cost MUST be computed from programmatically-fetched, per-leg, per-venue maker/taker fees plus a measured slippage model. No hardcoded fee constants; no second copy of the number in another module `[→ C1]`. **Spot taker fees are materially higher than perp** (~0.075–0.10% vs ~0.05%) and MUST be fetched separately — do **not** reuse the perp round-trip figure for cash-and-carry.
- **Discrete settlement.** Expected funding capture MUST enumerate the actual settlement timestamps inside the planned hold on each leg (from live `nextFundingTime` + `fundingIntervalHour`) and sum expected per-settlement rates. It MUST NOT multiply a per-hour rate by hours as if funding accrued continuously `[→ C2]`.
- **Interval is live, not static.** Funding interval is read per-symbol on every market-data update, because Bybit can flip an alt's interval 8h↔1h mid-position without notice `[→ C8]`.
- **Min-edge from measured drift.** The entry threshold multiplier MUST be derived from the empirically-measured distribution of `(realized_settlement_rate − entry_estimate)` per symbol/venue (e.g., cover the p75–p90 adverse drift), not a flat 2× `[→ C3]`.
- **Leg quantization.** Target quantity MUST be quantized to the **intersection of both legs' venue lot sizes**, with an explicit residual-delta tolerance; leftover delta beyond tolerance is a sizing failure, not silently accepted `[→ X7]`.

---

## 3. Architecture

### 3.0 Resolved architecture decisions

| # | Decision | Choice (MVP) | Rationale |
|---|---|---|---|
| **D1** | Topology / hot-path transport | **Single-process engine.** All contexts in one `arblab-bot` binary; hot path via **in-process ports + a single deterministic dispatcher** (§3.4). No NATS on the trading loop. | Lowest latency, simplest ops for a single trading bot. bitnet's DDD discipline is kept; only its NATS *transport* is deferred (§3.7). |
| **D2** | Durable events + audit substrate | **Local write-ahead journal + transactional outbox → embedded store** (SQLite/Postgres). The OMS journal is the source of truth; the outbox is an **at-least-once export** to idempotent consumers (audit, obs, tax). | bitnet's outbox discipline without NATS. Crucially, the outbox is *not* the exactly-once execution-truth mechanism — the journal is `[→ X1]`. |
| **D3** | Money type | **`platform/fixedpoint`** — scaled-integer decimal, 128-bit/`big.Int` intermediates, canonical scale + operation-specific rounding, SQL codec, wrapped in dimensional types. No `float64`. | Deterministic, overflow-safe, reproducible; unit-safe by construction (§3.6). Supersedes `shopspring/decimal`. |
| **D4** | Time / clock / parity | **NautilusTrader-style `Clock`** + a **single event dispatcher with a documented total order** over all inputs (events + timers), per-venue server-clock model. | The backtester must drive time *and* reproduce input ordering; frozen timestamps alone don't guarantee parity (§3.5). |

**Authoritative-state principle (the highest-leverage rule).** Order, fill, trade, and position truth is owned by **exactly one** context, `oms`, as a durable single-writer state machine with a write-ahead journal. Execution is its *mechanism*; monitoring and reconciliation are its *projections/process-managers*; audit is a *projection*. Nothing else writes that state `[→ X1, X3]`.

### 3.1 Foundation vs Strategy

```
       ┌──────────────────────── Foundation (strategy-agnostic, one process) ─────────────────────────┐
Venues►│ engine/venue → marketdata → engine/state ─► [ strategy.Evaluate ] ─► risk ─►                   │
       │                                                                                                │
       │      ┌───────────────── oms (SINGLE-WRITER authority) ─────────────────┐                       │
       │      │ plan+journal(fsync client IDs) → execute(OrderGateway) →         │ ──► audit (projection)│
       │      │ reconcile-to-truth → monitor/net-delta-invariant  (durable WAL)  │ ──► treasury / obs    │
       │      └──────────────────────────────────────────────────────────────────┘                      │
       └────────────────────────────────────────────────────────────────────────────────────────────────┘
                                        ▲
                               Strategy (pluggable) ── pure signal + sizing; emits Intents
```

Strategies are **pure signal + sizing policy**. They read a normalized `MarketView` and emit `Intent`s; they never call venue APIs, touch risk plumbing, or manage orders. The foundation runs every Intent through the **same** risk gate → OMS pipeline regardless of strategy `[→ A1]`.

### 3.2 Bounded contexts, engine subsystems & shared kernel

Per Codex round 1, the write-side is consolidated: `execution`, open-position `monitor`, and the authoritative part of `reconcile` collapse into one single-writer **`oms`** context `[→ X1]`.

| v0.1 component | arb-lab home | Kind | Responsibility |
|---|---|---|---|
| VenueAdapter + Binance/Bybit | `engine/venue` (raw clients + normalized DTOs) **+** each context's `infrastructure/driven` ACL | engine plumbing + context infra | Raw REST/WS transport & normalized DTOs live in `engine/venue`; each context wraps the capability it needs into its own port (ACL translation) — resolves the "adapter in engine" boundary smell `[→ X5]`. |
| marketdata | `boundary/marketdata` | context | Ingest + normalize funding/book/mark; reconstruct books; stamp venue-seq + source/receive time + freshness; publish an immutable `MarketView`. |
| state (cache) | `engine/state` | engine (cache) | In-memory latest-value cache; single-writer; serves consumer-owned **read ports**; translates to the strategy-owned `MarketView` (strategies never import `engine/*`) `[→ X5]`. |
| strategy | `boundary/strategy` | context | `Strategy` port + `cashcarry`/`crossvenue`; pure signal + sizing → `Intent`s. |
| risk | `boundary/risk` | context | Pre-trade `RiskGate`; stress/naked-leg sizing; **kill-switch policy armed via `foureyes` before trading** (§7.4); consumes account/margin/liquidation events. |
| **execution + monitor + authoritative reconcile** | **`boundary/oms`** | **context (single-writer authority)** | **The durable OMS/portfolio state machine: owns `Trade → Orders → Fills → PositionAllocation`; write-ahead order intent (fsync deterministic client IDs before submit); deterministic command handling; one-leg-failure via query-by-client-ID; continuous net-delta invariant; monitoring + reconcile as its process-managers** `[→ X1, X3]`. |
| treasury | `boundary/treasury` | context | Balances, available margin, collateral transfer, inventory drift, custody policy, **pre-funded rescue margin** (§7.1). |
| audit | `boundary/audit` (projection) + `platform/{auditevent,outbox}` | projection + kernel | Append-only, tamper-evident event log built as a **projection of the OMS journal** via the outbox (at-least-once, idempotent). The **journal** (§7.5) — not this lagged projection — is authoritative for reconciliation; audit serves reporting + tax `[→ X1, Z4]`. |
| operator | `app/operator` + `platform/foureyes` | composition root + kernel | Control API; **arms/edits emergency policy (two-person) before exposure exists**; config hot-reload; hosts the out-of-process watchdog (§7.4). |
| obs | `platform/observability` | kernel | Metrics/logs/tracing on an isolated **lossy** lane (§3.4). |
| backtest | `engine/backtest` + `cmd/arblab-backtest` | engine + composition root | The **same** contexts over the parity seam: replay `DataSource`, fill-model `ExecutionVenue`, `TestClock`, replayed config/reference revisions + seeds (§3.5, §8). |
| money math | `platform/fixedpoint` | kernel | Scaled-int decimal, big.Int intermediates, rounding modes, SQL codec, dimensional wrappers `[→ X7]`. |
| time / parity | `platform/clock` | kernel | `Clock` + per-venue server-time model + the total-order event dispatcher (§3.5). |
| event bus | `platform/events` (typed lanes) + `platform/outbox` (export) | kernel | In-process dispatch with typed delivery lanes (§3.4); outbox = at-least-once export. |
| ids | `platform/ids` | kernel | UUIDv7 + typed IDs; **deterministic `ClientOrderID`** derivation. |
| config | `platform/config` | kernel | Typed load + `Validate()` + safe hot-reload; revisions recorded for replay. |
| errors | `platform/apperror` | kernel | Error taxonomy (`errclass`). |
| rate limit | `platform/ratelimit` | kernel | Per-venue header-aware governors + the Bybit unheadered IP-ban guard (§7.6). |
| boot / reconcile-only-boot | `platform/boot` | kernel | Bootstrap phases: signal, config, obs, reconcile-only-boot, graceful shutdown (§7.5). |

**Context anatomy (every `boundary/<ctx>`):** `domain/` (aggregates, value objects, domain events — no infra/engine imports), `application/` (`ports/` = driven-port interfaces + use-case `Core` structs + a `<Ctx>Service` interface), `infrastructure/` (`driven/` outbound adapters + `driving/` inbound transport). Enforced rules in §3.6.

### 3.3 Strategy interface (context inbound port)

```go
type Strategy interface {
    Name() string
    Universe() []VenueInstrument                    // canonical (venue, product, symbol, contract) — spot & perp are DISTINCT [→ Z1]
    Evaluate(v MarketView) []Intent                 // MarketView is strategy-owned & immutable — NOT engine/state's type
    OnPosition(p PositionView, v MarketView) []Intent
    Params() StrategyParams                         // typed, validated, hot-reloadable
}

type Intent struct {
    Kind      IntentKind        // OpenTrade | CloseTrade
    Legs      []LegSpec         // each leg names a VenueInstrument (spot vs perp on one symbol are distinct) [→ Z1]
    TargetQty Quantity          // dimensional; identical across legs for exact neutrality, lot-quantized (§2.4)
    Reason    string            // for the audit log
}
```

`MarketView`/`PositionView` are immutable value types owned by `boundary/strategy` (or a domain-safe shared kernel); `engine/state` implements a read port and **translates** its cache into them, so `domain/application` never imports `engine/*` `[→ X5]`. The foundation owns `TradeID`/`ClientOrderID` generation, sizing caps, execution, and monitoring. A strategy that emits a two-leg Intent gets the same one-leg-failure handling as any other.

### 3.4 Concurrency, dispatch & delivery lanes `[→ X4]`

A single trading process needs explicit delivery semantics or a slow consumer silently corrupts either latency or the audit log. Lanes are typed, not one undifferentiated bus:

- **Control lane (lossless, ordered, write-ahead).** Private order/execution/account/liquidation/ADL events and OMS commands. Never dropped; feeds the single-writer OMS in a documented total order (§3.5).
- **Market-data lane (bounded, latest-value).** Book/mark/funding updates in per-**`VenueInstrument`** mailboxes (spot and perp on the same symbol are distinct sources, §5.1) `[→ Z1]`; superseded updates MAY be **coalesced** (latest wins). Bounded depth with defined overflow (drop-oldest + freshness gap counter), never unbounded growth.
- **Observability lane (isolated, lossy).** Metrics/logs/traces; back-pressure here never blocks trading.
- **Command sequencer.** Risk→OMS commands pass through one deterministic sequencer so live and replay apply them in the same order.

`engine/state` is single-writer (the marketdata reducer) handing immutable `MarketView`s to readers. Every published snapshot carries a **watermark**: source sequence, exchange timestamp, receive timestamp, and max cross-leg skew. **Freshness is per-data-class**, not one 30 s threshold: order books used for execution need sub-second budgets + a cross-leg skew limit; funding metadata tolerates longer. Shutdown drains the control lane before exit. Execution uses per-venue worker goroutines with rate-limit governors (§7.6).

### 3.5 Backtest-live parity (the from-scratch discipline) `[→ B1, D4, X8]`

Parity requires reproducing **input ordering**, not just timestamps. The engine depends only on these seams and one dispatcher:

```go
// platform/clock — live wall-clock vs. a deterministic TestClock the backtester advances.
type Clock interface {
    Now() time.Time                       // engine time; per-venue server offset/uncertainty modeled separately
    ServerNow(Venue) ServerTime           // authoritative for funding-boundary decisions, with uncertainty band
    SetTimer(id TimerID, at time.Time, prio uint8)  // cancel/replace by id; ties broken by prio then id
    CancelTimer(id TimerID)
}

// The dispatcher assigns every ACCEPTED input a monotonic IngressSeq — that sequence, not the
// timestamps, is the replay key. VenueSeq is local to a SourceID and can collide across symbols.
type SourceID struct { Venue Venue; Product ProductCategory; Symbol string; Stream StreamKind } // = a VenueInstrument stream; NOT globally ordered [→ Z1]
type Envelope struct {
    IngressSeq uint64        // monotonic, assigned at accept time; the total order + replay key [→ Y4]
    Source     SourceID
    SourceTime, RecvTime time.Time
    VenueSeq   uint64        // per-SourceID only
    Prio       uint8
    Payload    any
}

// engine/venue — the data & execution seams (live adapter vs. replay / fill-model adapter).
type DataSource     interface { Subscribe(VenueSymbol) <-chan Envelope }
type ExecutionVenue interface { OrderGateway; AccountSource }   // see §4
```

Live wiring injects real WS/REST clients and a wall-clock `Clock`; backtest injects a replay `DataSource` (Tardis + self-captured streams), a `TestClock`, a fill-model `ExecutionVenue`, and **replayed config/reference-data revisions + deterministic seeds**. Strategy, risk, sizing, OMS, and monitor code are identical across both. **The total order is `IngressSeq`**, assigned after a **bounded watermark reorder** (buffer by `SourceTime` up to a max per-venue skew window; a later-than-watermark arrival records a gap, it does not silently reorder). Within that, ties break by `(SourceTime, Source, VenueSeq, Prio)`. **Market-data coalescing is a deterministic rule** keyed on `(Source, field)` — never a function of goroutine scheduling — and both the pre- and post-coalescing dispatched sequences are persisted for replay. **REST calls run outside the state-transition dispatcher** and re-enter as fresh envelopes with their own `IngressSeq`. The guarantee is **shared code + invariant parity under this single ingress order** — not "byte-identical," which is false the moment two inputs share a timestamp `[→ Y4]`.

### 3.6 Structural rules (enforced, not aspirational)

Each is checked by a boundary/lint test in CI:
1. **Hexagonal boundary.** `domain/` and `application/` MUST NOT import `infrastructure/` or `engine/`. Strategy inputs are domain-safe value types, not `engine/state` types (rule enforces §3.3) `[→ X5]`.
2. **Ports in `application/ports/`.** Every driven dependency is an interface there; adapters live in `infrastructure/driven/` with a compile-time `var _ ports.X = (*Impl)(nil)`.
3. **Infrastructure shape.** `infrastructure/` contains exactly `driven/` + `driving/`.
4. **Money is dimensional `fixedpoint`.** No `float64` in any money path (`nofloat` AST lint). Values are `Money{Currency}`/`Quantity{Asset}`/`Price{Base,Quote}`/`Rate`; arithmetic uses 128-bit/`big.Int` intermediates, canonical scale, and an explicit rounding mode per operation; a SQL codec (not raw floats) persists them `[→ X7]`.
5. **Time is injected.** No `time.Now()` in `domain/`/`application/`; funding-boundary logic uses `Clock.ServerNow(venue)` with its uncertainty band `[→ X8]`.
6. **Single-writer authority.** Only `oms` mutates order/fill/trade/position state, through the journal-then-apply path (§6). A lint forbids other contexts from importing the OMS write repositories `[→ X1]`.
7. **Explicit wiring.** Each context has a `module.go`; the binary wires modules by hand. **No DI container.**
8. **Typed value objects.** Enum-likes are `type X string` + constants + `Validate()`.

### 3.7 Service-split seam (documented, not built) `[→ D1, A2]`

If a context ever needs its own process: introduce an `internal/platform/<ctx>sdk` courier (interface + wire DTOs + NATS request-reply), swap the in-process port for the sdk client at the composition root, and route that context's domain events through `platform/outbox` → NATS JetStream. Nothing in `domain/`/`application/` changes. The MVP builds none of this; today's ports stay split-ready (narrow, one concern, DTOs free of `engine/*` types).

---

## 4. Venue contract (the N-venue seam) `[→ A1, X1, X2]`

`engine/venue` owns raw Binance/Bybit clients and normalized DTOs; contexts consume **segregated capability interfaces** declared in their own `application/ports/` and satisfied by thin ACL adapters. Every per-instrument method keys on the canonical **`VenueInstrument{Venue, ProductCategory, Symbol, ContractType}`** — spot `BTCUSDT` and perp `BTCUSDT` are *different* instruments with independent order/execution-ID namespaces and separate REST endpoints; `VenueSymbol` in the signatures below is shorthand for it, and recovery (`Fills`) is always product+symbol-scoped `[→ Z1]`.

```go
// Consumed by boundary/marketdata:
type MarketDataSource interface {
    StreamFunding(VenueSymbol) <-chan Envelope   // FundingUpdate: rate, interval, nextSettle, cap/floor, source, venueSeq
    StreamBook(VenueSymbol)    <-chan Envelope   // BookUpdate: reconstructed top-N, venueSeq
    StreamMark(VenueSymbol)    <-chan Envelope
}

// Consumed by boundary/oms (order lifecycle mechanism):
type OrderGateway interface {
    Place(Order) (Ack, error)                     // NON-idempotent: submit once, NO transport retry; ClientOrderID is a
                                                  //   recovery handle, NOT a venue idempotency key (§5.3) [→ Y2]
    Cancel(OrderRef) error                        // OrderRef{Instrument VenueInstrument, ClientOrderID} — spot/perp differ [→ Z1]
    OrderByClientID(OrderRef) (OrderStatus, error) // product-routed; resolve SubmissionUnknown — NEVER blind-resubmit [→ X1, Z1]
    OpenOrders() ([]Order, error)
    Fills(VenueInstrument, since Cursor) ([]Fill, Cursor, error) // product+symbol-scoped (venues require it); per-instrument cursor [→ Z1]
    StreamOrders()     <-chan Envelope            // private, sequenced: accept/partial/fill/cancel/reject
    StreamExecutions() <-chan Envelope            // fills also arrive here — same ExecutionID, applied exactly once [→ Y1]
}

// Consumed by boundary/oms (position truth), boundary/treasury, boundary/risk:
type AccountSource interface {
    Positions() ([]Position, error)
    Balances()  ([]Balance, error)                // per asset; wallet/account mode
    MarginStatus(VenueSymbol) (MarginStatus, error) // available margin, maintenance, live liq price
    TransferCollateral(CollateralTransfer) (Ack, error)
    StreamAccount() <-chan Envelope               // private, sequenced: position / margin-call / LIQUIDATION / ADL,
                                                  //   carrying ForcedCloseReason [→ X2]
    FundingLedger(since time.Time) ([]FundingEvent, error) // prefer venue immutable ledger txn IDs [→ X9]
}

// Consumed by boundary/risk + oms sizing (fetched, never hardcoded):
type ReferenceSource interface {
    Fees(VenueSymbol) (FeeTier, error)            // maker/taker per symbol, spot≠perp, live
    ContractSpec(VenueSymbol) (SymbolSpec, error) // linear|inverse, tick/lot
    MarginTier(VenueSymbol, notional Money) (mmr Rate, err error) // Binance leverageBracket at runtime [→ C4]
}

type OpsSource interface { UsedWeight() RateLimitState; ServerTime() (time.Time, error) }

// engine/venue's Binance & Bybit adapters satisfy all five; each context depends only on the narrow one it needs.
type VenueAdapter interface { MarketDataSource; OrderGateway; AccountSource; ReferenceSource; OpsSource }
```

**Per-venue quirk matrix the adapter MUST encode (verified 2026-07-18):**

| Concern | Binance USDⓈ-M | Bybit V5 linear |
|---|---|---|
| WS routing | `wss://fstream.binance.com/market` (mark/funding), `/public` (book/bookTicker), `/private` (user data). **Legacy `/ws/` decommissioned 2026-04-23 — old path returns nothing** `[→ 7.2]` | single WS host; topics `tickers.<sym>`, `orderbook.50.<sym>`, private |
| Funding estimate | `!markPrice@arr@1s` → `r` (rate), `p` (mark), `i` (index), `T` (next). **No interval field.** | `tickers.<sym>` → `fundingRate`, `nextFundingTime`, **`fundingIntervalHour`, `fundingCap`** (new Oct-2025; pin client ≥ Oct-2025) |
| Interval source | REST `GET /fapi/v1/fundingInfo` overlay (absent ⇒ 8h default); formula scales `/(8/N)` | from ticker `fundingIntervalHour`; **dynamic 8h↔1h auto-switch on alts, no notice** `[→ C8]` |
| Cap/floor | BTCUSDT ±0.3% (=0.75×MMR); alts ±2% default; overlay via `fundingInfo` | ticker `fundingCap` / `instruments-info.upper/lowerFundingRate`; read live |
| Book | `@depth5/10/20@100/250/500ms` (futures default 250ms) | `orderbook.50` snapshot+delta; delete on size "0"; reset on snapshot |
| Ticker deltas | full snapshot each message | snapshot then delta; **absent field = unchanged, never null-coerce** |
| Private streams | `/private` user-data WS (listenKey lifecycle): order, execution, `ACCOUNT_UPDATE`, margin-call, ADL | private topics `order`, `execution`, `position`, `wallet`; auth handshake |
| Forced-close reason | user-data event fields distinguish liquidation vs ADL | order/execution/position events carry `closeType` (ADL / liquidation) |
| Order query | `GET /fapi/v1/order` by `origClientOrderId`; `userTrades` for fills | `GET /v5/order/realtime` + `/v5/execution/list` by `orderLinkId` |
| Fees | `GET /fapi/v1/commissionRate` (+`rpiCommissionRate`) | `GET /v5/account/fee-rate` |
| Position mode | Hedge Mode **rejects `reduceOnly` (err -2022)**; close via opposite side + matching `positionSide`. One-way uses `reduceOnly` | `positionIdx` 0/1/2 + `reduceOnly` |
| Market order | `MARKET` has **no price cap** — use LIMIT+IOC | converts to IOC limit w/ venue band; optional user `slippageTolerance` |
| Tiered MMR | **fetch `GET /fapi/v1/leverageBracket` at runtime** — table not otherwise readable `[→ C4]` | `instruments-info` risk limits |
| Rate limit | `X-MBX-USED-WEIGHT-*`/`ORDER-COUNT-*`; 429→418 IP ban (2 min→3 days) | `X-Bapi-Limit*` headers **and** unheadered IP gateway ban ~600/5s (retCode 10018) |

Contract-type (`linear`/`inverse`) is first-class so neutrality math can branch when inverse is added `[→ C6]`. All numeric fields cross the boundary as dimensional `fixedpoint` (parsed from venue decimal strings), never `float64` `[→ D3, X7]`.

---

## 5. Data model & configuration

### 5.1 Aggregates, ownership & value objects `[→ X1, X3, X7, X9]`

**The OMS aggregate (single owner).** `boundary/oms` owns one aggregate root `Trade` → `Orders[]` → `Fills[]` → `PositionAllocation`, plus a `Version int64` (OCC) and a durable write-ahead journal. It holds an **exclusive lease per hedge-group `(account, venue, symbol)`** — deliberately grouping a trade's spot + perp legs, and distinct from per-**instrument** identity (§5.3) — for the life of a trade; a second Intent on a leased key is rejected (no cross-strategy netting in MVP) `[→ X3, Z1]`. `audit` and `reconcile` are projections/process-managers over the journal, never co-owners.

Types (exported fields for repo hydration; all money is dimensional):
`Trade{ID, Strategy, Legs[], State, OpenedAt, ClosedAt, RealizedPnL Money}` · `Order{ClientOrderID, Instrument VenueInstrument, Side, Type, TIF, Qty Quantity, LimitPrice Price, ReduceOnly, PositionSide/Idx, State}` · `Fill{ExecutionID, Account, Instrument VenueInstrument, OrderID, Qty Quantity, Price Price, Fee Money, FeeAsset, Ts}` (immutable venue `ExecutionID` = Binance `tradeId` / Bybit `execId`, unique only **within** an instrument; deduped by `UNIQUE(account, venue, product_category, symbol, execution_id)` — spot & perp ID namespaces are independent — so a fill seen on both `StreamExecutions` and REST `Fills` recovery applies once) `[→ Y1, Z1]` · `Position{Instrument VenueInstrument, Qty Quantity, EntryPrice Price, Margin Money, LiqPrice Price, MMR Rate}` · `FundingEvent{Account, Venue, Symbol, SettleAsset, SettleTs, Rate, MarkAtSettle Price, Amount Money, LedgerTxnID, Raw json.RawMessage}`.

Value objects (`Validate()`-guarded): **`VenueInstrument{Venue, ProductCategory, Symbol, ContractType}`** (the canonical identity for routing, market-data keys, orders, fills, reference calls, and recovery cursors `[→ Z1]`), `VenueSymbol` (underlying/display only), `ProductCategory{Spot|LinearPerp|…}`, `ContractType{Linear|Inverse}`, `Side`, `OrderState`, `KillSwitchMode`, `ForcedCloseReason{ADL|Liquidation|…}`, `SymbolSpec{ContractType, Tick, Lot, IntervalSource, CapSource}`, `FeeTier`, `StrategyParams`.

**Exactly-once funding ingestion `[→ A4, X9]`.** Prefer the venue's immutable **ledger transaction ID** as the identity. Absent that, key on `(account, venue, symbol, settleAsset, settleTs, ledgerEntryType)` — the specific contract **symbol**, not the `Spot|Perp` enum (else BTCUSDT and ETHUSDT perps settling USDT at the same instant collide and one receipt is silently dropped) — with a **DB `UNIQUE` constraint** and `INSERT … ON CONFLICT DO NOTHING` **in the same transaction** as ledger progress — a hash alone guarantees nothing `[→ Y5]`. Store the raw payload; model corrections/reversals as explicit events, never in-place edits.

### 5.2 Configuration schema
- **Global risk caps:** `per_trade_notional`, `per_venue_concentration_pct`, `per_symbol_concentration_pct`, `max_total_open`, `liq_distance` policy (open ≥30% / alert <25% / rebalance <20% / emergency <15% for BTC; tighter for alts) — one policy, reconciled `[→ C5]`.
- **Per-venue:** endpoints, key refs (never inline secrets), IP-allowlist expectations, region, subaccount id.
- **Per-strategy:** typed params (cash-and-carry: `min_funding_apr`, `min_hold_settlements`, `exit_on_negative`, `perp_leverage`; cross-venue: `min_spread_bar`, symbol whitelist, `max_hold`).
- **Strategy registry:** `{enabled, weight}` per strategy `[→ user goal #1]`.
- **Emergency policy:** pre-authorized loss bounds + escalation ladder params (§7.4), editable only via two-person approval **before** exposure exists.
- Config is a typed struct with `Validate()`, hot-reloadable where safe; every change is audit-logged and **revisions are recorded for deterministic replay** (§3.5).

### 5.3 OMS order & fill identity, states, and commands `[→ X1, Y1, Y2, Y3]`

**Identities.**
- **`ClientOrderID`** — deterministic, OMS-minted, unique per submission; a **recovery correlation handle, not a venue idempotency key** (Binance permits reuse once an order is terminal). Never reused within the recoverable window.
- **`ExecutionID`** — the venue's immutable fill id (Binance `tradeId`, Bybit `execId`), unique only **within** an instrument. Fills dedupe on `UNIQUE(account, venue, product_category, symbol, execution_id)` — the single normative key (spot and perp ID namespaces are independent); `VenueSeq` is *not* an identity `[→ Y1, Z1]`.

**Order state machine + an orthogonal fill ledger `[→ Z2]`.** Fills are *not* gated by order lifecycle: a **cumulative-execution ledger** accepts any deduped fill for a *known* order regardless of its current state — so a post-cancel or post-`SubmissionUnknown` late fill still applies. Order state tracks submission/finality; the fill ledger tracks realized quantity; **net delta is recomputed from the ledger on every fill/position event** (§6.7).

| State | Meaning | → next on |
|---|---|---|
| `Planned` | intent + client ID fsynced to journal; not yet sent | begin submit → `Submitted` |
| `Submitted` | exactly one request sent; awaiting ack (**the crash boundary** — journal precedes the non-transactional network write, so recovery always queries by client ID) | ack → `Acknowledged` · reject → `Rejected` · timeout/5xx → `SubmissionUnknown` |
| `SubmissionUnknown` | ambiguous; resolve **only** by `OrderByClientID`/`Fills`, never resubmit | query → `Acknowledged` / `PartiallyFilled` / `Filled` / `Rejected` |
| `Acknowledged` | live on venue | fill → `PartiallyFilled` / `Filled` · cancel-req → `CancelPending` |
| `PartiallyFilled` | some fills applied; more possible | more fills → `Filled` · cancel-req → `CancelPending` |
| `CancelPending` / `CancelUnknown` | cancel requested / cancel ambiguous | confirmed → `Canceled` · **a late fill still posts to the ledger** |
| `Filled` · `Canceled` · `Rejected` | terminal for *state* (the fill ledger stays open to dedup late arrivals until a venue-specific **finality window** closes) | — |

**Rules.** `Place` is called **once** per order; transport retries forbidden `[→ Y2]`. `SubmissionUnknown`/cancel resolution runs **concurrently** with exposure management — the OMS never blocks flattening on it (§6.7). If the resolving query is unavailable/rate-limited, new trading stays disabled and rate-limit budget is **reserved for recovery**; "not submitted" is never inferred from a transient not-found — finality follows a **venue-specific stable-not-found rule**. Applying a fill (dedup → ledger → `PositionAllocation`), advancing the **per-instrument** REST recovery cursor, and appending journal/outbox events happen in **one transaction** `[→ Y1, Z1]`.

**Commands (distinct verbs, so emergency ≠ strategy) `[→ Y3]`:**
- **`OpenTrade` / `CloseTrade`** — strategy-originated; subject to `stop-opening`.
- **`ExposureReduction`** — foundation-originated, **exempt from `stop-opening`**, used to remove a naked leg after ADL / liquidation / one-leg-failure. It reduces exposure by either **(a)** a **spot sell capped by OMS-owned inventory allocation** — a plain sell, since `reduceOnly` does **not** exist for spot on either venue — or **(b)** a **hedge-restoring perp order** (reduce-only where the venue supports it). The per-leg choice is driven by which leg is naked and the net-delta invariant.

---

## 6. Trade lifecycle (foundation, strategy-agnostic) `[→ X1, X2]`

1. **Ingest → state.** `marketdata` normalizes with venue-seq + source/receive timestamps; `engine/state` latest-value cache; **per-data-class freshness** (sub-second books for execution) `[→ X4]`.
2. **Signal.** `Strategy.Evaluate(MarketView)` emits Intents.
3. **Pre-trade risk.** `risk.Gate` re-validates edge, free margin, position state, kill-switch, rate-limit budget, concentration, and the funding-boundary guard using `Clock.ServerNow(venue)` `[→ X8]`.
4. **Sizing.** `min(per-trade cap, per-venue headroom, fillable-within-slippage, margin-for-target-liq-distance)`; liq distance from the **live tier MMR** `[→ C4]`; quantize both legs to the lot-size intersection + residual tolerance `[→ X7]`; budget the naked-leg gap + reacquisition slippage `[→ P3, X2]`.
5. **Plan & journal (OMS).** Mint **deterministic `ClientOrderID`s**; persist **and fsync** `TradePlanned` + intents to the write-ahead journal **before any submission** `[→ X1]`.
6. **Execute (OMS via `OrderGateway`).** Submit **once** per order (no transport retry); parallel marketable-IOC limit orders (never raw MARKET), identical base quantity, unique client IDs. A timeout/5xx moves the order to `SubmissionUnknown`, resolved by `OrderByClientID`/`Fills`, never resubmitted (§5.3). Applying each fill, advancing the REST recovery cursor, updating `PositionAllocation`, and appending events happen in **one transaction**; the outbox exports at-least-once `[→ X1, Y1, Y2]`.
7. **One-leg-failure handler.** On **every** fill/position event, recompute net delta from the fill ledger; if it breaches tolerance, **immediately** issue an `ExposureReduction` (§5.3) sized from **`realized_delta − outstanding_reserved_reductions`** and tracked as a **fenced reduction generation** — coalesce / amend / cancel-replace it, never fire a fresh command per event, so repeated events cannot stack duplicate reductions `[→ Z2]`. The reduction is an inventory-capped **spot sell** for a naked spot leg (no `reduceOnly`) or a reduce-only **perp** order, both exempt from `stop-opening`. Resolving ambiguous order status (`OrderByClientID`/`Fills`) runs **concurrently**, never a precondition for flattening — so the ≤5 s unhedged budget (§9) is not held hostage to the ≤30 s resolution budget. Stop new *strategy* orders; if the resolving query is unavailable/rate-limited, hold new trading and reserve rate-limit budget for recovery. Escalate via the pre-authorized ladder (§7.4), operator alerted in parallel `[→ S4, X6, Y3, Z2]`.
8. **Monitor (OMS process-manager).** Driven by **private account/ADL events** (not just polling): liq distance, basis divergence, funding **audit** (vs realized) and **attribution** (vs signal) kept distinct, interval-change detection. A `ForcedCloseReason` (ADL/liquidation) event fires an immediate `ExposureReduction` (§5.3), not a next-tick reaction `[→ C7, C8, X2, Y3]`.
9. **Close.** Same parallel-IOC pattern; realized PnL decomposed per leg from actual fills + settled marks; journaled + exported.
10. **Crash recovery (boot).** Reconcile-only boot rebuilds from journal ⋈ venue truth (`OrderByClientID`/`Fills`/`Positions`/`FundingLedger`); **auto-recovers known journaled client IDs**; only genuinely foreign orders or unexplained position deltas require operator clearance `[→ A3, X1]`.

---

## 7. Risk, custody & security (real-capital bar)

### 7.1 Sizing for survival `[→ C4, C5, P3, X2]`
Liq-distance policy is single-sourced (§5.2) and post-Oct-2025 stress-inclusive. MMR is always the live tier value (`leverageBracket`). Effective leverage 2–3× on BTC, 1.5–2× on alts. **Stress sizing budgets the naked-leg price gap + reacquisition slippage** for the unhedged window after a forced close, and requires **pre-funded rescue margin** already at the venue (reactive transfers are unavailable during stress). Report yield on full bankroll.

### 7.2 Custody & settlement — **on-exchange for MVP** `[→ S1]`
- **Decision:** on-exchange collateral and on-exchange spot, deliberately, at **small size**, in **isolated per-venue subaccounts** with **hard collateral caps** and per-venue concentration caps (≤~20–30% of bankroll/venue); USDT/USDC-only margin. A conscious, documented acceptance of exchange-custody risk to ship — not an oversight.
- **Why not OES yet:** no single OES rail spans Binance + Bybit (Binance → Ceffu MirrorX; Bybit → Copper ClearLoop / Fireblocks). Crossing rails needs two custodians — the documented scale-up path, gated on size.
- **OES is necessary-but-not-sufficient anyway:** it removes custody/counterparty risk but not exchange-margin/mark-price risk (Oct-2025 liquidated OES users too). Keep liq-distance buffers + mark-price sanity checks regardless.
- **Spot leg (cash-and-carry):** held on-exchange in MVP under withdrawal-disabled keys (§7.3); cold/hot split + qualified custodian documented as the same scale-up step. `boundary/treasury` owns balances, the concentration cap, inventory-drift monitor, and the rescue-margin float.

### 7.3 API-key security & blast radius `[→ S2, X10]`
- Trade-enabled keys have **withdrawal permission disabled**; separate read-only monitoring keys; keys in Vault/KMS, per-subaccount, IP-allowlisted, rotation policy.
- **Realistic blast radius = the entire funded subaccount.** A compromised trade key from an allowed host can bleed the subaccount via deliberately adverse trades even with withdrawals disabled; an IP allowlist does not protect a compromised allowed host, and software risk caps are void if the bot/signer is compromised `[→ X10]`. Therefore: hard-cap collateral per isolated subaccount; disable internal transfers + every nonessential permission; **isolate the order-signer from the trading process**, restrict its egress and the order schemas it will sign; run an **independent read-only watcher** enforcing venue/account-level exposure limits out of band.

### 7.4 Kill-switch modes & pre-authorized escalation `[→ A5, S4, X6, Y3]`
- Modes: **(a) stop-opening**, **(b) flatten-if-feasible** (the `ExposureReduction` path, respects slippage cap), **(c) hard-halt-and-alert**. `KillSwitchMode` is a typed VO owned by `boundary/risk`.
- **Two-person approval arms/edits emergency *policy* before trading; it does NOT gate emergency *execution*.** Once exposure exists, pre-authorized loss bounds execute **automatically** — a 03:00 naked residual cannot wait for a human `[→ X6]`. Ladder (each step pre-authorized): stop openings → **concurrently { reconcile-to-truth ∥ bounded `ExposureReduction` }** (reconciliation may correct the residual but is **never** a prerequisite for flattening, §6.7) → **pre-authorized panic-MARKET / venue-native stop** → revoke key or allowlist. Operator alerting runs in parallel, never as the terminal action.
- **Actuator authority & failure domain — mandatory `[→ Y3, Z3]`.** Three components across separate failure domains: (i) the in-process OMS runs `ExposureReduction` on the hot path; (ii) a **read-only watcher** (§7.3) — observes, alerts, trips policy; no trading key; (iii) an **emergency actuator** that MUST act after the bot host *and its local journal* die. Because the allocation cap lives in that journal, the actuator relies on **host-independent coverage** (next bullet), never on selling from **coarse venue balances** (forbidden — it can hit unrelated inventory). The documented upgrade replicates a **signed, versioned `EmergencyPlan`** (instrument, allocated qty, loss/slippage bounds, fencing generation) to an out-of-process actuator **before exposure opens**, with watcher→actuator auth, idempotent command IDs, independent key/IP + reserved capacity, and OMS reconciliation on takeover. The watchdog escalates (i)→(iii) on host silence.
- **Host-independent coverage across the exposure-opening transition `[→ Z3]`.** Exposure begins the instant an entry leg fills — a *separate* venue op from any follow-on protective order, so it cannot share the fill's journal transaction. To close that window: entry legs MUST carry **venue-native attached protection submitted atomically with the entry** (e.g. Bybit TP/SL on the order) so protection acks *with* the fill; where a venue can't attach protection atomically, the replicated `EmergencyPlan` actuator MUST be **armed before** the entry submission. A protective order **reserves** the spot allocation, so a hot-path `ExposureReduction` **amends / cancel-replaces that same fenced protective order** rather than duplicating it. The residual first-fill→ack window is minimized, bounded, and a §9 acceptance case.
- **Out-of-band trip caveat:** pulling the IP allowlist can also remove the connectivity you need to flatten — prefer venue-native stops and permission reduction; sequence any connectivity kill last.

### 7.5 Reconcile-only boot `[→ A3, D2, X1]`
On every cold start / kill-switch toggle / extended control-lane gap, `boundary/oms` runs a reconcile-only boot phase (`platform/boot`): signal + new execution disabled while state is rebuilt from the **write-ahead journal joined to venue truth** (`OrderByClientID`, `Fills`, `Positions`, `FundingLedger`). Known journaled client IDs are **auto-recovered**; only foreign orders or unexplained position deltas **alert for operator clearance** (they are not auto-corrected). The net-delta invariant is asserted before trading resumes.

### 7.6 Rate-limit discipline
Header-aware governors per venue (`platform/ratelimit`); back off at ~70% weight; treat 429 (Binance) / 403+10006 and the **unheadered IP ban ~600/5s retCode 10018** (Bybit) as hard pauses; surface 418 / IP ban as immediate operator alerts. Independent IPs per venue where possible.

---

## 8. Backtester `[→ B1, B2, X8]`
- **Parity:** the §3.5 engine — the same contexts wired by `cmd/arblab-backtest` over the `Clock`/`DataSource`/`ExecutionVenue` seam with a single total-order dispatcher, **not** a parallel simulator.
- **Data:** Tardis.dev for both live-estimate streams (Binance `markPrice@1s` `r`; Bybit `tickers`) — **Coinglass cannot back this signal** (funding OHLC ≠ live estimate). Self-capture both streams from day one for parity + recent days.
- **Fidelity requirements:** live-estimate (not settled-rate) signal, no lookahead; discrete funding accrual at real settlement timestamps (fired by `TestClock` timers); spread/level **persistence model**; **adverse-selection fill model** (IOC fills preferentially when the book moves against you); **survivorship** (delisted/force-settled alt pairs); real fee tiers (spot ≠ perp) + slippage + latency distributions; **replayed config/reference-data revisions + deterministic seeds**; replay all exit rules tick-by-tick, never fixed-N-cycle holds.

---

## 9. Testing & rollout `[→ B2, X11]`
- **Structural (CI gates):** hexagonal boundary test, single-writer-authority lint, compile-time port assertions, `nofloat` money lint, no-`time.Now()` lint (§3.6).
- **Unit:** funding normalization, book reconstruction, break-even, liq-distance-by-tier, PnL decomposition, `fixedpoint` arithmetic edges (overflow into big.Int intermediates, rounding, dimensional mismatch).
- **Property-based (gopter):** order-book invariants, delta-application idempotence, funding exactly-once under concurrent WS+REST, journal-replay determinism.
- **Testnet integration:** Binance + Bybit testnets for the full order lifecycle incl. `OrderByClientID` recovery, before any real key.
- **Chaos / fault injection:** venue rejects, partial fills, post-cancel late fills, WS gaps, 429/418, **process kill mid-IOC**, forced-close/ADL events, **identical spot/perp execution IDs**, **host death between first fill and protective-order ack**, **duplicate position events during an outstanding reduction**, **protective-order ↔ OMS inventory handoff** — assert journal recovery + net-delta invariant + no duplicate/over-reduction + host-independent coverage every time.
- **Paper-trading acceptance gate — explicit pass/fail `[→ X11, Y6]`** (initial bar; tune with external review): shadow-runs **≥ 14 continuous days** AND **≥ 30 funding settlements on each funding-bearing perp leg** (spot legs earn none) AND **≥ 100 synchronized spot↔perp entry/exit cycles** for spot-path coverage; **100% of position deltas explained** by journaled fills + funding (zero unexplained units); **order + funding reconciliation completeness = 100%** over the window; per-fault **RTOs met** — reconcile-only boot **< 60 s**, naked-leg detected→reduced **< 5 s** (alert at 2 s), WS reconnect+resync **< 10 s**, `SubmissionUnknown` resolved **< 30 s**; **max observed unhedged-exposure duration < 5 s**. Evidence = the paper-run report (drift ledger, fault-injection log with per-fault RTO, reconciliation report). Promotion authority: a named **trading lead** signs each tier ($1K→$10K→scale); **any on-call operator** may roll back unilaterally.
- **Rollout:** paper → $1K → $10K → scale; canary each new build on a small-notional subaccount for 1–3 funding cycles before promotion.

---

## 10. Observability & tax records
- **Metrics:** funding (per venue/symbol), spread/level, WS freshness + cross-leg skew, open positions, unrealized/realized PnL, fill rate, one-leg-failure rate, **unhedged-exposure duration**, error/rate-limit rate, inventory drift (isolated lossy lane).
- **Alerts:** control-lane gap, per-class staleness, liq-distance thresholds, basis divergence >0.5%, drawdown, kill-switch, rate-limit ban, inventory-drift threshold, watchdog miss.
- **Tax hooks `[→ 8 review]`:** each funding receipt is ordinary income and each settlement a realized event; the platform emits per-event USD-valued records to the durable audit projection for downstream tax accounting.

---

## 11. Tech stack & repo layout
- **Backend:** Go, single module. Native venue REST/WS (`nhooyr.io/websocket` or `gorilla/websocket`, `net/http`); **dimensional `platform/fixedpoint`** for all money (no `shopspring/decimal`, no `float64`) `[→ D3, X7]`; embedded durable store (SQLite for MVP, Postgres-ready) holding the **OMS write-ahead journal + outbox + audit projection**; `prometheus/client_golang`; gRPC/HTTP operator control API. **No DI container; no third-party HTTP router** (stdlib `net/http` ServeMux).
- **Frontend:** React + Next.js on Bun — operator dashboard (positions, spreads/levels, balances, risk, kill switch, watchdog) over the control API.
- **Reference-only OSS (inspect, never depend):** Hummingbot `v2_funding_rate_arb.py` (+ bug #7295 to avoid), NautilusTrader (single-writer execution/portfolio + backtest-parity patterns), CCXT (venue-quirk cheat-sheet — its unified funding is inconsistent, so read raw per-venue), barter-rs (event-driven patterns).
- **Deployment:** single region, single process to start (Tokyo `ap-northeast-1`); room for per-region collectors + a service split (§3.7) later.

**Repo layout:**

```
arb-lab/
├── cmd/
│   ├── arblab-bot/            # live engine — composition root binary
│   ├── arblab-backtest/       # backtester over the parity seam (§3.5, §8)
│   ├── arblab-operator/       # operator control API + dashboard backend + watchdog
│   └── arblab/                # ops CLI: capture, replay, reconcile, key checks
├── internal/
│   ├── boundary/              # bounded contexts (domain / application / infrastructure)
│   │   ├── marketdata/  strategy/  risk/
│   │   ├── oms/               #   SINGLE-WRITER authority: order/fill/trade/position + monitor + reconcile
│   │   ├── treasury/  audit/  #   audit = projection of the OMS journal
│   ├── engine/                # technical subsystems (no DDD layering)
│   │   ├── venue/             #   raw Binance/Bybit clients + normalized DTOs (§4)
│   │   ├── state/             #   latest-value cache: book reconstruction + MarketView translation
│   │   └── backtest/          #   replay DataSource + fill-model ExecutionVenue + TestClock
│   ├── platform/              # shared kernel (reimplemented fresh; never a dependency)
│   │   ├── fixedpoint/  clock/  events/  outbox/  ids/  config/
│   │   ├── apperror/    auditevent/  foureyes/  boot/  observability/  ratelimit/
│   └── app/                   # composition roots (wiring per binary): bot/ backtest/ operator/
├── migrations/                # embedded-store schema (journal, outbox, audit, funding-dedupe UNIQUE)
├── configs/                   # per-env config, risk caps, strategy registry, emergency policy
├── web/                       # Next.js on Bun — operator dashboard
└── docs/
```

---

## 12. MVP milestones (cash-and-carry first)
1. **Foundation skeleton:** `platform/{fixedpoint(dimensional),clock,events(lanes),outbox,ids,config,apperror,boot}` + `engine/venue` (market-data + account/order capabilities) + `engine/state` + `platform/observability`. Parity dispatcher and all §3.6 CI gates (boundary, single-writer, `nofloat`, no-`time.Now()`) from day one.
2. **Cash-and-carry read-path:** `boundary/marketdata` + `boundary/strategy/cashcarry` — funding/level signal, break-even (real spot+perp fees), discrete-settlement capture; paper decisions journaled.
3. **OMS write-path:** `boundary/oms` — write-ahead journal, deterministic client IDs, parallel-IOC, `OrderByClientID` recovery, one-leg-failure + net-delta invariant, sizing-by-live-tier; testnet integration incl. process-kill-mid-IOC.
4. **Account/control + risk + reconcile-only boot:** private account/ADL streams, `boundary/risk` kill-switch modes + pre-authorized ladder + external watchdog, `boundary/audit` projection, custody policy + withdrawal-disabled keys + signer isolation.
5. **Backtester:** `cmd/arblab-backtest` on Tardis + self-capture; fidelity gates; the §9 paper acceptance gate with explicit pass/fail.
6. **$1K live → scale;** `arblab-operator` + `web/` dashboard.
7. **Strategy #2 (cross-exchange spread):** `boundary/strategy/crossvenue` on the same foundation, scoped to alts/dislocation.

---

## Appendix — review-finding & decision → resolution traceability

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
| **Y4 total order = `IngressSeq` (watermark reorder + deterministic coalescing)** | §3.5 |
| **Y5 funding fallback key uses symbol + ledger-entry type** | §5.1 |
| **Y6 acceptance-gate numeric thresholds** | §9 |
| **Z1 canonical `VenueInstrument` identity (spot ≠ perp; independent ID namespaces)** | §3.3, §3.4, §3.5, §4, §5.1, §5.3 |
| **Z2 orthogonal fill ledger + race-complete state machine + immediate-flatten** | §5.3, §6.7 |
| **Z3 mandatory emergency-actuator failover (venue-native baseline / replicated `EmergencyPlan`)** | §7.4 |
| **Z4 single reconciliation truth (journal, not the audit projection)** | §3.2, §7.5 |
