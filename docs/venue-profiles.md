# Venue Capability Profiles — Binance USDⓈ-M / Bybit V5

**Status:** v0.9.19 (2026-08-01) — set-version alignment, no new venue fact in this bump (v0.9.19 is the §12 post-kill fork ruling: fork (c), strategy #2 early — calibration design record §41r; the ruling's caveat 1 concerns this document's future: §41p's "fees, not depth" finding — Binance calm-stratum p90 book slippage ZERO at 0.02–0.5 BTC — is a BTC fact and does NOT transfer to alt books, so strategy #2's scoping will add measured alt-pair depth facts here rather than inherit that row). Previous: v0.9.18 (2026-08-01), milestone 2's economic-gate kill. Previous: §1's Bybit cap/floor row and the dated §5 block record the **2026-07-30 `instruments-info` live measurement**: upper/lower funding clamps `±0.005`, exactly symmetric and equal to the ticker's `fundingCap` — the ground the replay decoder's floor mirror rests on, with the live clients obligated to replace the mirror with the polled pair — and `fundingInterval: 480`, MINUTES, the units-in-the-name trap live beside the same venue's `fundingIntervalHour`.
**Doc set:** normative core `build-spec.md` · venue facts `venue-profiles.md` · kernel machines `state-machines.md` · history/traceability `adr-history.md` · teaching companion `tutorial.md`.
**Role:** per-venue facts + quirk matrices behind the core's venue-contract requirements (`build-spec.md` §4). **Citation coverage, stated honestly `[→ R1d-11]`:** every fact carries its verification *date* (2026-07-18 / 2026-07-19 / 2026-07-20); the **safety-kernel rows carry full source citations in §4 here**; the remaining §1/§3 rows' per-row citations live in the v0.6/v0.7 review records (`docs/review/`), not inline — treat any fact older than a venue API cycle as due for re-checking.

## 1. Per-venue quirk matrix (perp/futures)

**Per-venue quirk matrix the adapter MUST encode (verified 2026-07-18):**

| Concern | Binance USDⓈ-M | Bybit V5 linear |
|---|---|---|
| WS routing | Base `wss://fstream.binance.com`. **Three prefixes, split by data rate** — `/market` (regular market data: **mark price**, aggTrade, kline, tickers, liquidations, mini-tickers, composite index), `/public` (high-frequency: **depth**, bookTicker, `!bookTicker`), `/private` (user data). Raw stream `…/<prefix>/ws/<stream>`; combined `…/<prefix>/stream?streams=<a>/<b>`, or connect to `…/<prefix>/stream` and send `SUBSCRIBE`. **Legacy `wss://fstream.binance.com/ws` and `/stream` decommissioned 2026-04-23** `[→ 7.2]` *(re-verified verbatim 2026-07-29, §5)* | **`wss://stream.bybit.com/v5/public/linear`** (USDT/USDC perp + futures); also `/v5/public/spot`, `/v5/public/inverse`, `/v5/private`, `/v5/trade`; testnet is `stream-testnet.bybit.com`, same paths. Topics `tickers.<sym>`, `orderbook.50.<sym>`, private. **Client sends ping ≈ every 20 s**; ≤ 500 connections per 5 min per domain; public `args` ≤ 21 000 chars *(verified 2026-07-29, §5)* |
| Funding estimate | **Per-symbol `<sym>@markPrice@1s`** (or `<sym>@markPrice` = 3 s) on the **`/market`** prefix; all-symbol `!markPrice@arr@1s`. Payload → `r` (funding rate), `p` (mark), `i` (index), `P` (estimated settle price, *"only useful in the last hour before the settlement starts"*), `ap` (mark price moving average), `T` (next funding time), `st` (symbol type, post-CM-migration: 1=UM, 2=CM), plus `e`/`E`/`s`. **No interval field.** *(per-symbol form + `ap`/`st` verified 2026-07-29, §5)* | `tickers.<sym>` → `fundingRate`, `nextFundingTime`, **`fundingIntervalHour`, `fundingCap`** (new Oct-2025; pin client ≥ Oct-2025) |
| Interval source | REST `GET /fapi/v1/fundingInfo` overlay (absent ⇒ 8h default); formula scales `/(8/N)` | from ticker `fundingIntervalHour`; **dynamic 8h↔1h auto-switch on alts, no notice** `[→ C8]` |
| Cap/floor | BTCUSDT ±0.3% (=0.75×MMR); alts ±2% default; overlay via `fundingInfo` | ticker `fundingCap` / `instruments-info.upper/lowerFundingRate`; read live. **BTCUSDT measured 2026-07-30: upper `0.005` / lower `-0.005`, exactly symmetric and equal to the ticker's `fundingCap`** (§5) |
| Book | On the **`/public`** prefix. Partial: `<sym>@depth<5\|10\|20>@<100\|250\|500>ms`; diff: `<sym>@depth@<100\|250\|500>ms`. Omitted speed = **250 ms** default *(levels + all three speeds verified 2026-07-29, §5)* | `orderbook.<1\|50\|200\|1000>.<sym>` on the **linear** URL; **level 50 pushes at 20 ms** (1 → 10 ms, 200 → 100 ms, 1000 → 200 ms). Snapshot+delta; delete on size "0"; reset on snapshot *(level-50 cadence verified 2026-07-29, §5)* |
| **Depth update-ID shape** `[→ standing debt #1, CLEARED 2026-07-29, §5]` | **Futures partial and diff depth carry the SAME shape** — `U` *"First update ID in event"*, `u` *"Final update ID in event"*, `pu` *"Final update Id in last stream (ie `u` in last stream)"*, plus `E` event time and `T` transaction time. A **span with a chain link**: a gap is detectable from the wire alone, because `pu` must equal the previous event's `u`. **Binance SPOT does not share this — see §3.** | **A single `u` per message** (*"Update ID … is used to identify the data version"*), plus `seq` (cross sequence) and `type` ∈ {`snapshot`, `delta`}. **No span, no previous-event link**, so continuity is `u` monotonicity alone and a `snapshot` is the reset signal. **`u` is reset to 1 on a service restart** — a decoder treating `u` as globally monotonic breaks across one |
| Ticker deltas | full snapshot each message | snapshot then delta; **absent field = unchanged, never null-coerce** |
| Private streams | `/private` user-data WS (listenKey lifecycle): order, execution, `ACCOUNT_UPDATE`, margin-call, ADL | private topics `order`, `execution`, `position`, `wallet`; auth handshake |
| Forced-close reason `[→ R1d-7]` | liquidation/ADL closes surface on `ORDER_TRADE_UPDATE` via **system `clientOrderId` prefixes** — `autoclose-` (liquidation), `adl_autoclose` (ADL); REST recovery **`GET /fapi/v1/forceOrders`** (`autoCloseType` `LIQUIDATION`\|`ADL`) *(corroborated 2026-07-20 — not verbatim-fetched, §4; browser-confirm before implementation)* | **no `closeType` field exists on any V5 private stream** (the v0.7.1 row was wrong). Verified 2026-07-20: **execution `execType` ∈ {`BustTrade` (liquidation), `AdlTrade` (ADL)} is the authoritative signal** (carries qty + `execId` — dedups through the standard fill ledger); corroboration: order `createType` ∈ {`CreateByLiq`, `CreateByTakeOver_PassThrough`, `CreateByAdl_PassThrough`}, position `positionStatus` ∈ {`Liq`, `Adl`} + `adlRankIndicator`. Recovery: **`GET /v5/execution/list`** (accepts + returns `execType`); closed-pnl distinguishes only `BustTrade`, **not** ADL. **No cross-topic arrival order is documented** — join fills↔position via `seq`+symbol (§4) |
| Order query | `GET /fapi/v1/order` by `origClientOrderId`; `userTrades` for fills | `GET /v5/order/realtime` (open + last ~500 closed, **cleared on server restart**) → fall back to **`GET /v5/order/history`** by `orderLinkId`; `/v5/execution/list` for fills `[→ R2-#1]` |
| Attached protection | **No atomic attach-on-entry**: New Order carries no TP/SL param; conditional types are **separate Algo-Service orders** (`POST /fapi/v1/algoOrder`, since 2025-12-09; sending via `/order` returns **-4120**); **no futures OCO/bracket** → a pre-armed **external actuator is MVP-mandatory** on the perp leg for the first-fill→ack window (core §7.4). SPOT brackets via `orderList/otoco` (LIMIT working order only) | **Venue-native TP/SL attaches atomically** on `POST /v5/order/create` — but the create response is an **async accept**; the fill *and* the armed protection are confirmed on the **private WS** (`order`/`execution`), not the HTTP ack. Baseline host-independent cover on the **perp** `[→ R1-F2, R2-#4, R2b-B2]` |
| Fees | `GET /fapi/v1/commissionRate` (+`rpiCommissionRate`) | `GET /v5/account/fee-rate` |
| Position mode | Hedge Mode: **`reduceOnly` cannot be sent** (param-level restriction; -2022 "ReduceOnly Order is rejected" is the *generic* error, not the Hedge mechanism); close via opposite side + matching `positionSide`. One-way uses `reduceOnly`; **read one-way via `GET /fapi/v1/positionSide/dual`.dualSidePosition==false** | `positionIdx` 0/1/2 + `reduceOnly`; one-way = `positionIdx 0` (read `position/list`; set `position/switch-mode 0`) |
| Market order | `MARKET` has **no price cap** — use LIMIT+IOC | converts to IOC limit w/ venue band; optional user `slippageTolerance` |
| Tiered MMR | **fetch `GET /fapi/v1/leverageBracket` at runtime** — table not otherwise readable `[→ C4]` | `instruments-info` risk limits |
| Rate limit | `X-MBX-USED-WEIGHT-*`/`ORDER-COUNT-*`; 429→418 IP ban (2 min→3 days) | `X-Bapi-Limit*` headers **and** unheadered IP gateway ban ~600/5s (retCode 10018) |

## 2. Account & margin architecture (posture facts)

**Per-venue posture facts + assert-or-refuse read paths (core §4.1):**

| Concern | Binance USDⓈ-M (classic account) | Bybit UTA (linear) |
|---|---|---|
| Account model | Segregated **spot wallet ≠ futures wallet** (separate pools; funds transferred in). **Portfolio Margin / Unified MUST be OFF** — it would fold spot into futures collateral | **UTA** unified account (UTA2.0 for new accounts; classic unavailable since the Dec-2024 forced migration) — the load-bearing property is **Isolated Margin mode**, not the UTA *version*; segregation is per-position, not separate wallets |
| Perp margin mode | **ISOLATED** — `POST /fapi/v1/marginType {marginType:ISOLATED}` | **ISOLATED_MARGIN** (account-level under UTA2.0) — `POST /v5/account/set-margin-mode {setMarginMode:ISOLATED_MARGIN}` |
| Collateral policy | **Single-Asset Mode** — `POST /fapi/v1/multiAssetsMargin {multiAssetsMargin:false}` (USDT-only margins BTCUSDT). Multi-Assets forces **Cross** and is mutually exclusive with Isolated → MUST be off | Per-coin collateral switch exists **only in Cross/Portfolio, not Isolated** — do **not** rely on "switch BTC off"; Isolated per-position liquidation *is* the segregation. `spotHedgingStatus` MUST be **OFF** (spot-hedging lets spot offset derivatives, re-entangling the legs) |
| Spot borrow/leverage | plain spot buy (no margin) | **`isLeverage=0`** hard-coded on every spot `POST /v5/order/create`; spot-margin mode **OFF** (`/v5/spot-margin-trade/switch-mode 0`) |
| Position mode & leverage `[→ R2b-5]` | **One-way** — `GET /fapi/v1/positionSide/dual`.dualSidePosition==false; configured leverage per symbol pinned (drives liq distance). Hedge mode forbids `reduceOnly` (param-level) | **One-way** — `positionIdx==0` (read `GET /v5/position/list`; set `POST /v5/position/switch-mode 0`); configured leverage per symbol pinned |
| Spot seizable for a perp loss? | **No** — futures liquidation is computed on the futures wallet only; the margin engine never sees spot BTC | **No, under Isolated** — per-position loss is capped at that position's margin; there is **no account-level MMR liquidation** in Isolated mode. Requires **auto-add-margin OFF** so the position can't silently pull funds |
| Rescue margin lives in | **staged in the futures wallet, *outside* the isolated position** — applied by an explicit **`positionMargin {type:1}` top-up** when liq distance breaches rebalance (Isolated ⇒ free wallet is *not* seen by the position's liq engine; spot-wallet USDT is invisible to the futures engine, and spot→futures transfers degrade under stress); core §7.1 | **staged in the UTA, *outside* the isolated position** — applied by an explicit **`position/add-margin` top-up** on the same trigger (Isolated ⇒ per-position margin only; auto-add-margin OFF); core §7.1 |
| Posture assert-or-refuse (boot + pre-trade) | `GET /fapi/v1/multiAssetsMargin`==false; `GET /fapi/v1/symbolConfig`.marginType==ISOLATED (or `positionRisk`); Portfolio-Margin OFF; `positionRisk.isAutoAddMargin`==false (**read-only** — no REST setter, off by default); **one-way** `positionSide/dual`.dualSidePosition==false; configured leverage matches pin | `GET /v5/account/info`: **marginMode==ISOLATED_MARGIN** (the load-bearing check), unifiedMarginStatus ∈ {3,4,5,6} = any UTA (classic **1** refused; 5/6 expected for new accounts), spotHedgingStatus==OFF; `GET /v5/spot-margin-uta/status`==off; `GET /v5/account/wallet-balance` every coin borrowAmount==0 ∧ spotBorrow==0; **auto-add-margin OFF for every configured linear symbol** (not only open positions); **one-way** `positionIdx==0` (`position/list`); configured leverage matches pin |

## 3. Spot-leg surface

**Spot-leg facts — separate hosts/streams/filters; no mark/funding exists on spot (core §4.2):**

| Spot concern | Binance SPOT (`api.binance.com`) | Bybit SPOT (`category=spot`) |
|---|---|---|
| Order / query / fills | `POST /api/v3/order`, `GET /api/v3/order`, `/api/v3/openOrders`, **`/api/v3/myTrades`** (fills) — *not* the futures `userTrades` | V5 `/v5/order/*` with `category=spot`; `/v5/execution/list` |
| User-data stream | **separate listenKey** — `POST /api/v3/userDataStream` on the spot host (+PUT keepalive ~30 m, DELETE), distinct from `/fapi/v1/listenKey`. **Deprecated →** new path is WS-API `userDataStream.subscribe` needing **Ed25519 `session.logon`** (pin the auth path) | private `order`/`execution`/`wallet` on the spot private WS |
| Trading filters | `GET /api/v3/exchangeInfo` → `LOT_SIZE` (stepSize), `PRICE_FILTER` (tickSize), `NOTIONAL`/`MIN_NOTIONAL` | `instruments-info category=spot` (lot/tick/min-notional) |
| Book stream | `<sym>@depth`/`@depth<5\|10\|20>` on `wss://stream.binance.com:9443` (or `:443`) — *not* fstream, and **no `/public` prefix**: the spot host kept its own base. Partial-depth speeds are **1000 ms or 100 ms** (*not* futures' 100/250/500). Init snapshot `GET /api/v3/depth` *(verified 2026-07-29, §5)* | `orderbook.<1\|50\|200>.<sym>` on `wss://stream.bybit.com/v5/public/spot` (category set by URL path); level 50 at 20 ms as on linear |
| **Depth update-ID shape** `[→ standing debt #1, §5]` | **Two shapes, and neither is the futures one.** Spot **diff** depth carries `U` + `u` but **no `pu`** — a span with no chain link. Spot **partial** depth carries **`lastUpdateId` and nothing else** — no span, no `pu`, no event time `E`. So a Binance spot partial-depth feed is **not gap-detectable from its own wire content**; ordering evidence has to come from outside the payload | Same as linear — single `u` (reset to 1 on service restart), `seq`, `type` ∈ {`snapshot`, `delta`} |
| Native bracket | `orderList/otoco` (OTOCO) / `oco` — atomic spot bracket, **LIMIT working order only** | TP/SL on the spot order where supported |
| Mark / index / funding | **none — spot has no mark or funding.** Price & freshness-check the spot leg from its **own book (best bid/ask / last)**; `StreamMark` is **perp-only**. Bybit spot ticker's `usdIndexPrice` is a USD-conversion ref, **not** a mark | same — no mark/funding on spot |

So `MarketDataSource.StreamMark` is defined **only for perp instruments** (below), and the spot leg's valuation/freshness reads its book. Rate-limit governors are **per `(venue, API family)`** — `(binance, spot)` and `(binance, futures)` report and enforce *separate* used-weight/order budgets via their own `exchangeInfo`, so they are distinct **weight/order** buckets, not one venue bucket — though the **IP-ban envelope is treated as shared per venue** (inferred, not documented; core §7.6).

## 4. Verification record (dated sources)

Verified against official docs 2026-07-18 / 2026-07-19 (from the v0.7 review record §2, frozen there):

### Binance USDⓈ-M — isolated margin, posture, keys
- **Isolated margin:** only the position's allocated isolated margin is at stake; **free futures-wallet USDT does not passively back an isolated position** — rescue is an active add-margin call. → https://academy.binance.com/en/glossary/isolated-margin · https://www.binance.com/en/support/faq/how-to-adjust-margin-balance-in-cross-isolated-margin-mode-360038447311
- **Add isolated margin:** `POST /fapi/v1/positionMargin` (`type` 1=add, 2=reduce). → https://developers.binance.com/docs/derivatives/usds-margined-futures/trade/rest-api/Modify-Isolated-Position-Margin
- **Auto-add-margin:** `isAutoAddMargin` is **READ-only** via `GET /fapi/v2/positionRisk` (or v3) — **no public REST setter** on USDⓈ-M; off by default → read-verify `false`. → https://developers.binance.com/docs/derivatives/usds-margined-futures/trade/rest-api/Position-Information-V2
- **Position mode read:** `GET /fapi/v1/positionSide/dual` → `dualSidePosition` (true=Hedge, false=One-way). Hedge mode: `reduceOnly` **cannot be sent** (param-level restriction; -2022 "ReduceOnly Order is rejected" is the *generic* error, not the Hedge mechanism). → https://developers.binance.com/docs/derivatives/usds-margined-futures/account/rest-api/Get-Current-Position-Mode · https://developers.binance.com/docs/derivatives/usds-margined-futures/trade/rest-api/New-Order
- **Margin-mode events:** `ACCOUNT_CONFIG_UPDATE` fires on leverage (`ac`) and multi-assets (`ai`); **isolated/cross margin-type** change surfaces via `ACCOUNT_UPDATE` reason `MARGIN_TYPE_CHANGE`. **Position-mode (one-way/hedge) change emits NO event → REST poll.** → https://developers.binance.com/docs/derivatives/usds-margined-futures/user-data-streams/Event-Account-Configuration-Update · https://developers.binance.com/docs/derivatives/usds-margined-futures/user-data-streams/Event-Balance-and-Position-Update
- **Key management / fence:** a **regular account has NO public REST endpoint to disable/delete/IP-edit its own key** (UI + 2FA only; read-only `GET /sapi/v1/account/apiRestrictions`). API-automatable **only via the sub-account program with a MASTER key**: `DELETE /sapi/v1/sub-account/subAccountApi`, permission `POST /sapi/v1/sub-account/subAccountApiPermission`, IP `POST /sapi/v2/sub-account/subAccountApi/ipRestriction`. → https://developers.binance.com/docs/wallet/account/api-key-permission · https://developers.binance.com/docs/sub_account/api-management

### Bybit V5 — isolated margin, posture, keys
- **Isolated margin (UTA):** loss capped at the position's margin; **no auto-transfer from available balance** unless Auto-Margin Replenishment (auto-add-margin) is on. → https://www.bybit.com/en/help-center/article/UTA-Trading-Rules · https://medium.com/bybit/what-is-the-isolated-margin-mode-how-to-add-auto-margin-replenishment-to-a-position-3d9bd61ccf2
- **Add/reduce margin:** `POST /v5/position/add-margin` (doc slug `manual-add-margin`; **path is `add-margin`** — a known slug/path mismatch). `margin` positive adds, negative reduces. → https://bybit-exchange.github.io/docs/v5/position/manual-add-margin
- **Auto-add-margin:** `POST /v5/position/set-auto-add-margin` (`autoAddMargin` 0=off). → https://bybit-exchange.github.io/docs/v5/position/auto-add-margin
- **Position mode:** read `GET /v5/position/list` `positionIdx` (0=one-way); set `POST /v5/position/switch-mode` (0=one-way, 3=hedge — enum differs from `positionIdx`). Isolated/cross must be read from **`GET /v5/account/info` `marginMode`**, NOT the deprecated position `tradeMode` (always 0). → https://bybit-exchange.github.io/docs/v5/position/position-mode
- **create-order ack:** the response is an **async accept** (returns `orderId`/`orderLinkId`); status/fills must be confirmed via the private WS — *"This request is asynchronous so please use the websocket to confirm the order status."* → https://bybit-exchange.github.io/docs/v5/order/create-order
- **Spot TP/SL + IOC:** spot **Limit** orders support attached TP/SL; spot supports IOC; **combining attached-TP/SL with an IOC spot order is doc-silent → integration-test item** (neither confirmed nor denied). → https://bybit-exchange.github.io/docs/v5/order/create-order
- **Sub-key fence:** modify `POST /v5/user/update-sub-api` (slug `modify-sub-apikey`), delete `POST /v5/user/delete-sub-api`. **Requires an authority holding ONE OF `Account Transfer` / `Sub Member Transfer` / `Withdrawal`** (a sub key may modify itself; a master key may manage its sub) — a **high-privilege, transfer/withdrawal-class** credential. → https://bybit-exchange.github.io/docs/v5/user/modify-sub-apikey · https://bybit-exchange.github.io/docs/v5/user/rm-sub-apikey

### Bybit V5 — forced-close evidence (verified 2026-07-20, official docs fetched verbatim) `[→ R1d-7]`
- **No `closeType` field exists** on the private order / execution / position streams — the v0.7.1 quirk-matrix row claiming it was wrong. → https://bybit-exchange.github.io/docs/v5/websocket/private/order · https://bybit-exchange.github.io/docs/v5/websocket/private/execution · https://bybit-exchange.github.io/docs/v5/websocket/private/position
- **Execution stream (authoritative):** `execType` enum — `Trade`, `AdlTrade` ("Auto-Deleveraging"), `Funding`, `BustTrade` ("Takeover liquidation"), `Delivery`, `Settle`, `BlockTrade`, `MovePosition`, … ; `execId` (the fill dedup key); `seq` — "Cross sequence, used to associate each fill and each position update … use seq + symbol to check unique". → https://bybit-exchange.github.io/docs/v5/enum · https://bybit-exchange.github.io/docs/v5/websocket/private/execution
- **Order stream (corroboration):** `createType` incl. `CreateByLiq` ("laddered liquidation"), `CreateByTakeOver_PassThrough` ("closed at bankruptcy price"), `CreateByAdl_PassThrough` ("Auto-Deleveraging(ADL)"). **Position stream (corroboration):** `positionStatus` = `Normal` / `Liq` / `Adl`; `adlRankIndicator`. → (order/position pages + enum, above)
- **Recovery:** `GET /v5/execution/list` **accepts an `execType` request param** and returns `execType` + `execId`; `GET /v5/position/close-pnl` carries a **reduced** `execType` set (`Trade`, `BustTrade`, `SessionSettlePnL`, `Settle`, `MovePosition`) — it can flag a liquidation but **not ADL**; use `execution/list` for ADL. → https://bybit-exchange.github.io/docs/v5/order/execution · https://bybit-exchange.github.io/docs/v5/position/close-pnl
- **No cross-topic arrival-ordering guarantee is documented** for order/execution/position; `seq`+symbol is the documented fill↔position correlation key, never an assumed arrival order.

### Binance USDⓈ-M — forced-close evidence (corroborated 2026-07-20 — ⚠ not verbatim: `developers.binance.com` is a client-rendered SPA; **browser-confirm before implementation**) `[→ R1d-7]`
- Liquidation/ADL closes surface on `ORDER_TRADE_UPDATE` via **system `clientOrderId` prefixes**: `autoclose-` (liquidation), `adl_autoclose` (ADL) (+ `settlement_autoclose-` for delisting/settlement). → https://developers.binance.com/docs/derivatives/usds-margined-futures/user-data-streams/Event-Order-Update
- REST recovery: **`GET /fapi/v1/forceOrders`** ("Query User's Force Orders") with **`autoCloseType` ∈ {`LIQUIDATION`, `ADL`}** — the explicit liq-vs-ADL distinguisher. → https://developers.binance.com/docs/derivatives/usds-margined-futures/trade/rest-api/Users-Force-Orders
- `ACCOUNT_UPDATE` event-reason (`m`) reportedly includes `ADL`; `MARGIN_CALL` is a *warning*, not a close confirmation — **both unverified here; confirm in-browser**.
- **History retention (recovery-relevant, `[→ Cx5-3]`):** order + algo-order history ≈**90 days** (some unfilled-cancelled orders purged in ~3 days); trade (execution) history ≈**6 months**; **no server-side client-ID-prefix filter** — prefix matching is client-side over enumerated pages. → https://developers.binance.com/en/docs/catalog/core-trading-derivatives-trading-usd-s-m-futures/api/rest-api/trade *(corroborated 2026-07-20 — browser-confirm exact windows)*.

### Margin-op & transfer resolution evidence (verified 2026-07-20; Binance response bodies corroborated, not verbatim — SPA) `[→ R1d-6]`
- **Bybit add/reduce margin returns only a position snapshot — no operation/transaction id**: `POST /v5/position/add-margin` response = `category, symbol, positionIdx, riskId, …, positionIM, positionMM, …, updatedTime` (no op id; the only resolution signal is the **`positionIM` delta** vs a before-state). → https://bybit-exchange.github.io/docs/v5/position/manual-add-margin
- **Bybit transaction-log has NO margin-adjustment type**: `GET /v5/account/transaction-log` records carry a unique `id`, but the ~100-value `type` enum (`TRANSFER_IN/OUT`, `TRADE`, `SETTLEMENT`, `LIQUIDATION`, `ADL`, …) contains **no value for a manual position-margin add/reduce** — whether a top-up emits a record at all is **undocumented; MUST NOT be relied on**. → https://bybit-exchange.github.io/docs/v5/account/transaction-log · https://bybit-exchange.github.io/docs/v5/enum
- **Binance margin-change history exists but records have no unique id**: `GET /fapi/v1/positionMargin/history` (`symbol`; `type` 1=add / 2=reduce; `startTime`/`endTime`/`limit` default 500) → rows `{amount, asset, symbol, time, type, positionSide}` — resolution matches on **symbol + type + amount + time window**, never an id. `POST /fapi/v1/positionMargin` responds `{amount, code, msg, type}` — **no op id**. → https://developers.binance.com/docs/derivatives/usds-margined-futures/trade/rest-api/Get-Position-Margin-Change-History · https://developers.binance.com/docs/derivatives/usds-margined-futures/trade/rest-api/Modify-Isolated-Position-Margin
- **Bybit transfers have a CLIENT-SUPPLIED idempotency key**: `POST /v5/asset/transfer/inter-transfer` **requires a client-generated `transferId` UUID** ("Please manually generate a UUID"); `GET /v5/asset/transfer/query-inter-transfer-list` queries **by that UUID** (`status`: `STATUS_UNKNOWN`/`SUCCESS`/`PENDING`/`FAILED`) — a timed-out create is idempotently resolvable, and a same-UUID retry is safe. → https://bybit-exchange.github.io/docs/v5/asset/transfer/create-inter-transfer · https://bybit-exchange.github.io/docs/v5/asset/transfer/inter-transfer-list
- **Binance transfers have NO client-supplied id**: `POST /sapi/v1/asset/transfer` returns a **server-generated** `tranId` only; resolution of a timed-out create = `GET /sapi/v1/asset/transfer` (keyed by required `type` + time window; `status` `CONFIRMED`/`PENDING`/`FAILED`) **before any retry** — a naive retry is not idempotent and can **double-transfer**. → https://developers.binance.com/docs/wallet/asset/user-universal-transfer

### SQLite durability (Linux target)
- **`PRAGMA fullfsync` is macOS-only** (`F_FULLFSYNC`); a **no-op on Linux**. → https://sqlite.org/pragma.html#pragma_fullfsync
- **WAL durability:** `synchronous=NORMAL` in WAL is durable across an *app* crash but **may lose the last transaction(s) on power loss**; **`synchronous=FULL` fsyncs the WAL on every commit** → power-loss durable. → https://sqlite.org/pragma.html#pragma_synchronous · https://sqlite.org/wal.html
- **Durability ≠ checkpoint** (a committed txn is durable in the fsync'd WAL before checkpoint; `wal_autocheckpoint` default 1000 pages). SQLite assumes the OS/hardware honor fsync; **network filesystems (NFS) are documented as corruption-prone** → local ext4/xfs with barriers only. → https://sqlite.org/atomiccommit.html · https://sqlite.org/howtocorrupt.html
- **Production (Linux):** `journal_mode=WAL; synchronous=FULL;` local fs; boot-verify pragmas.

## 5. Capture-source stream facts (verified 2026-07-29, official docs fetched verbatim)

**Why this section exists and why it is separate from §4.** §4 is frozen at the v0.7 review record. This
block clears the five verification debts the capture tool's design raised (design record §13) plus the
standing depth-update-ID debt open since v0.9.10, and it is dated separately so the two rounds are never
conflated.

**The method matters, because it is what unblocked three rows §4 could only mark "browser-confirm".**
`developers.binance.com/docs/…` is a client-rendered SPA and returns an empty app shell to a fetcher —
which is why §4 carries `⚠ not verbatim` on the forced-close and margin-op blocks. **There is a
server-rendered twin at `developers.binance.com/en/docs/catalog/…`** that returns the same documentation
as real text. §4 already cited one such URL without noticing it generalised. Everything below is
verbatim from that path or from Bybit's static docs; the one live-probe fact is labelled as such.

**The catalog is itself split by URL prefix**, which is corroboration rather than an inconvenience: the
`ws-streams/public` page contains **no** mark-price section, because mark price is a `/market` stream.
The page's silence is evidence for the routing split, not a failed fetch.

### Binance USDⓈ-M — websocket routing and the two decommissioned paths `[→ design §13 debt 2]`
- **Three prefixes on `wss://fstream.binance.com`:** `/public` (high-frequency public market data —
  Individual Symbol Book Ticker, Partial Book Depth, Differential Book Depth, All Book Tickers),
  `/market` (regular market data — Aggregate Trades, **Mark Price**, Klines, Tickers, Liquidations, Mini
  Tickers, Composite Index), `/private` (user data). **Legacy `wss://fstream.binance.com/ws` and
  `wss://fstream.binance.com/stream` were decommissioned 2026-04-23.** The prefixes were added to every
  stream page as a `URL PATH` section on **2026-03-05**. → https://developers.binance.com/en/docs/products/derivatives-trading-usds-futures/websocket-market-streams/Important-WebSocket-Change-Notice · https://developers.binance.com/docs/derivatives/change-log
- **Path forms:** raw `wss://fstream.binance.com/<prefix>/ws/<stream>`; combined
  `wss://fstream.binance.com/<prefix>/stream?streams=<a>/<b>`, or connect to `…/<prefix>/stream` and send
  a `SUBSCRIBE` request. → https://developers.binance.com/en/docs/catalog/core-trading-derivatives-trading-usd-s-m-futures/api/ws-streams/public
- **`wss://stream.binancefuture.com` is documented as a second base endpoint** alongside `fstream`. Not
  used, recorded so nobody re-derives it.

### Binance USDⓈ-M — the per-symbol mark-price (funding) stream `[→ design §13 debt 1]`
- **`URL PATH: wss://fstream.binance.com/market/ws/<symbol>@markPrice@<updateSpeed>`.** Stream name
  `<symbol>@markPrice@<updateSpeed>` — e.g. **`btcusdt@markPrice@1s`**; the bare `<symbol>@markPrice`
  form is the **3-second** cadence. **Update Speed: 3 s or 1 s** — there is no other cadence.
- **Payload:** `e` event type, `E` event time, `s` symbol, `p` mark price, `i` index price, `P`
  *"Estimated Settle Price, only useful in the last hour before the settlement starts"*, `r` funding
  rate, **`ap` mark price moving average**, `T` next funding time, **`st` *"(After CM migration) Symbol
  type: 1 = UM, 2 = CM"***. **`ap` and `st` were not previously recorded here.**
- **No funding-interval field on the stream** — confirmed against the field list, which is what §1's
  "Interval source" row and core §4's refuse-a-defaulted-zero rule both rest on.
- All-symbol form `!markPrice@arr@<updateSpeed>`, same 3 s / 1 s. → https://developers.binance.com/en/docs/catalog/core-trading-derivatives-trading-usd-s-m-futures/api/ws-streams/market

### Binance USDⓈ-M — book depth streams and their update-ID span `[→ design §13 debts 2 and 4]`
- **Partial Book Depth:** `<symbol>@depth<levels>@<updateSpeed>`; **levels 5, 10, 20**; speeds
  *"250ms or 500ms or 100ms"*. **Diff. Book Depth:** `<symbol>@depth@<updateSpeed>`, same three speeds.
  Both on the **`/public`** prefix.
- **Both carry the identical update-ID shape** — `U` *"First update ID in event"*, `u` *"Final update ID
  in event"*, `pu` *"Final update Id in last stream (ie `u` in last stream)"*, plus `E` event time, `T`
  transaction time, `ps` and `st` post-CM-migration. **The partial-book stream is therefore
  self-sequencing on futures**, which is *not* true of the same-named spot stream (below). →
  https://developers.binance.com/en/docs/catalog/core-trading-derivatives-trading-usd-s-m-futures/api/ws-streams/public

### Binance SPOT — book depth, and the shape that has no span at all `[→ design §13 debt 4]`
- Base endpoint **`wss://stream.binance.com:9443`** or **`wss://stream.binance.com:443`** — the spot host
  kept its own base and takes **no `/public` prefix**.
- **Partial Book Depth:** `<symbol>@depth<levels>` or `<symbol>@depth<levels>@100ms`; levels **5, 10, 20**;
  update speed **1000 ms or 100 ms** — *not* futures' 100/250/500.
- **Payload is `lastUpdateId` + `bids` + `asks`, and nothing else** — no `U`, no `u`, no `pu`, **no event
  time**. **Diff. Depth** carries `e`, `E`, `s`, `U`, `u`, `b`, `a` — a span, but **no `pu`**.
- **Consequence, stated because it is load-bearing:** a Binance spot partial-depth feed **cannot be
  gap-checked from its own wire bytes**. Ordering and loss evidence for that source must come from
  outside the payload. → https://github.com/binance/binance-spot-api-docs/blob/master/web-socket-streams.md

### Bybit V5 — public websocket URLs and the orderbook topic `[→ design §13 debts 3 and 4]`
- **Mainnet public:** `wss://stream.bybit.com/v5/public/linear` (USDT/USDC perpetual **and** futures),
  `…/v5/public/spot`, `…/v5/public/inverse`, `…/v5/public/option`, `…/v5/public/spread`,
  `…/v5/public/rfq`. **Private:** `wss://stream.bybit.com/v5/private`. **Order entry:**
  `wss://stream.bybit.com/v5/trade`. Testnet mirrors every path on `stream-testnet.bybit.com`.
- **Connection discipline:** *"Avoid establishing over 500 connections within a 5-minute window"* per
  domain; public `args` array ≤ 21 000 characters (spot: 10 args per request); client ping ≈ every
  **20 s**. → https://bybit-exchange.github.io/docs/v5/ws/connect
- **Orderbook topic `orderbook.<depth>.<symbol>`.** Linear depth/push-frequency: **1 → 10 ms, 50 → 20 ms,
  200 → 100 ms, 1000 → 200 ms**. Fields: `type` ∈ {`snapshot`, `delta`}, **`u` update id — *"resets to 1
  during service restarts"***, **`seq` cross sequence**, `b`/`a` as `[price, size]`. **Size 0 means
  delete the entry; a new snapshot means reset the local book.** Level-1 repeats its snapshot every 3 s
  with an unchanged `u`. → https://bybit-exchange.github.io/docs/v5/websocket/public/orderbook

### The two venues' depth update-ID shapes — the standing debt, and it is THREE shapes, not two `[→ standing debt #1, CLEARED]`
The debt was recorded as *"the two venues' depth update-ID shapes"*. That framing is wrong, and the
correction is the finding:

| Feed | Shape | Gap-detectable from its own bytes? |
|---|---|---|
| Binance **futures** partial **and** diff depth | `U` … `u` span **+ `pu`** chain link | **Yes** — `pu` ≠ previous `u` is a gap |
| Binance **spot** diff depth | `U` … `u` span, **no `pu`** | Only by comparing consecutive events |
| Binance **spot** partial depth | **`lastUpdateId` only** | **No** |
| Bybit linear **and** spot orderbook | single `u` (**resets to 1 on service restart**) + `seq` | Only by `u` monotonicity, and a restart breaks it |

**Binance is not one venue for this purpose** — it is a futures shape and two different spot shapes, and
the split is by *product and stream type*, not by venue. This is what core §4's *"a book delta payload
carries `FirstSeq` and never `LastSeq`"* has to absorb: a single-sequence feed sets `FirstSeq` from its
one number, and the spot-partial feed has no second number to lose.

### REST overlay endpoints — public vs authenticated `[→ design §13 debt 5]`
- **Binance security types, verbatim:** `NONE` — *"Endpoint can be accessed freely."*; `TRADE` and
  `USER_DATA` — *"requires sending a valid API-Key and signature"* (both are `SIGNED`); `USER_STREAM` and
  `MARKET_DATA` — *"requires sending a valid API-Key"* (key, no signature). → https://developers.binance.com/docs/derivatives/usds-margined-futures/general-info
- **`GET /fapi/v1/fundingInfo` is PUBLIC.** The catalog page marks it with no security type, and a
  **live unauthenticated probe on 2026-07-29 returned HTTP 200 with a full body** *(live probe, not a doc
  quote — labelled as such deliberately)*. Description: *"Query funding rate info for symbols that had
  FundingRateCap/FundingRateFloor / fundingIntervalHours adjustment"*. **Weight 0**, sharing a
  **500 / 5 min / IP** limit with `GET /fapi/v1/fundingRate`. → https://developers.binance.com/en/docs/catalog/core-trading-derivatives-trading-usd-s-m-futures/api/rest-api/market-data
- **`GET /fapi/v1/leverageBracket` is `USER_DATA`** (signed) — IP weight 1. **`GET /v5/account/fee-rate`
  requires Bybit authentication** (`X-BAPI-SIGN` / `X-BAPI-API-KEY` / `X-BAPI-TIMESTAMP` /
  `X-BAPI-RECV-WINDOW`); required param `category`, optional `symbol`/`baseCoin`. Both are on the
  design's **deferred** overlay list, and their being credential-bearing is exactly the deferral's first
  stated ground. → https://developers.binance.com/en/docs/catalog/core-trading-derivatives-trading-usd-s-m-futures/api/rest-api/account · https://bybit-exchange.github.io/docs/v5/account/fee-rate
- **`GET /fapi/v1/commissionRate`'s security type is NOT verified** — the catalog page truncated before
  its row. **Not asserted.** It is deferred anyway, so nothing built depends on it; recorded here so the
  gap is visible rather than assumed closed with its neighbours.

### The `fundingInfo` overlay, observed live — three facts that were not what the design assumed
A single unauthenticated `GET https://fapi.binance.com/fapi/v1/fundingInfo` on **2026-07-29** returned
**742 rows** of `{symbol, adjustedFundingRateCap, adjustedFundingRateFloor, fundingIntervalHours,
disclaimer, updateTime}`. *(Live observation. It is a measurement, not a documented guarantee, and
nothing may be built on it that a re-measurement could invalidate.)*

1. **BTCUSDT is PRESENT, not absent.** `{"symbol":"BTCUSDT", "adjustedFundingRateCap":"0.00300",
   "adjustedFundingRateFloor":"-0.00300", "fundingIntervalHours":8, "disclaimer":true,
   "updateTime":null}` — identical for ETHUSDT. The cap **confirms §1's ±0.3% row from a second
   direction**, and the interval is 8 h. But the *"absent ⇒ 8 h default"* branch **does not fire for our
   one traded symbol**; the overlay answers positively.
2. **8 h is not the common case.** `fundingIntervalHours` across the 742: **4 h × 442, 8 h × 296,
   1 h × 4.** §1 records no 4 h interval at all. "Absent ⇒ 8 h" is a statement about the *unlisted*
   symbols, and the majority of *listed* ones are 4 h — so the default is safe only because BTCUSDT is
   listed and says 8.
3. **`updateTime` is `null` on BTCUSDT** while other rows carry an epoch-ms integer. A decoder typing it
   as a non-nullable integer parses 740 rows and **fails on the one symbol we trade**.

### Bybit `instruments-info` funding clamps, observed live (2026-07-30) — the floor the ticker never states

A single unauthenticated `GET https://api.bybit.com/v5/market/instruments-info?category=linear&symbol=BTCUSDT`
on **2026-07-30** returned `upperFundingRate: "0.005"` and `lowerFundingRate: "-0.005"` — **exactly
symmetric, and equal to the live ticker's `fundingCap`**. *(Live observation, dated; a re-measurement
that finds asymmetry invalidates everything resting on the mirror below.)*

1. **This is the fact the replay decoder's floor mirror rests on.** The `tickers.<sym>` stream carries
   `fundingCap` and **no floor**; the floor exists only here, and the capture set does not poll this
   endpoint. The replay decoder therefore emits `Floor = -Cap`, justified by this measurement alone —
   and the live venue clients (milestone 3) MUST replace the mirror with the polled pair rather than
   inherit it.
2. **The same response carries `fundingInterval: 480` — MINUTES.** The ticker spells its interval field
   `fundingIntervalHour` ("8", hours); this endpoint spells the same 8-hour fact as 480 minutes with the
   unit only in the docs. Core §4's units-in-the-name rule, observed in the wild on one venue's two
   spellings of one number: an adapter that reads either field into a bare integer computes a carry
   figure wrong by a factor of sixty with nothing failing.

### BTCUSDT lot/notional filters, observed live (2026-07-31) — the four `(venue, product)` cells `[→ design §40g, step 3 slice A]`

Four unauthenticated probes on **2026-07-31 ~05:09 UTC** (Bybit response `time` 1785474562995), one per
cell of the milestone-2 universe. *(Live observations, dated; these are the rows the committed
milestone-2 reference revision cites, and step 5's period-correct archive replaces them the day a
re-measurement disagrees.)*

| Cell | Endpoint | Step / min qty | Max qty | Min notional |
|---|---|---|---|---|
| Binance SPOT | `GET /api/v3/exchangeInfo?symbol=BTCUSDT` | `LOT_SIZE` stepSize **0.00001**, minQty **0.00001** | 9000 | `NOTIONAL` minNotional **5** (USDT, `applyMinToMarket: true`) |
| Binance USDⓈ-M perp | `GET /fapi/v1/exchangeInfo` → BTCUSDT | `LOT_SIZE` stepSize **0.001**, minQty **0.001** | 1000 (market: 120) | `MIN_NOTIONAL` **50** |
| Bybit SPOT | `GET /v5/market/instruments-info?category=spot&symbol=BTCUSDT` | basePrecision **0.000001**, minOrderQty **0.000001** | 230 (market: 120) | minOrderAmt **5** (USDT) |
| Bybit linear perp | `GET /v5/market/instruments-info?category=linear&symbol=BTCUSDT` | qtyStep **0.001**, minOrderQty **0.001** | 1500 (market: 150) | minNotionalValue **5** |

1. **The two perps agree on 0.001 BTC steps; the two spots do not agree with each other** (10⁻⁵ vs
   10⁻⁶). §2.4's intersection rule therefore binds at the PERP step for either venue's package — a
   quantization that used the spot step would emit perp-unroutable quantities a thousandth of a BTC at
   a time.
2. **Min-notional is the binding floor on Binance's perp** (50 USDT ≈ 0.0008 BTC at ~65k, above the
   0.001 lot floor's ~65 USDT — close enough to flip as price moves; the sizing must check both).
3. **Bybit spot's `quotePrecision` (0.0000001) is finer than fixedpoint's scale-9 tick only in
   appearance** — it is a PRECISION, not a step; the order-quantity constraint is `basePrecision`, and
   nothing here requires sub-scale-9 arithmetic.

### Base-tier fee tables, observed live (2026-07-31, rendered pages) — the four fee cells `[→ design §40d, step 3 slice A]`

Read from the two venues' official fee pages **rendered in a real browser on 2026-07-31** (both are
client-rendered SPAs; a plain fetcher gets an empty shell or times out — the §40d rejection of a
programmatic fee-page importer, observed while verifying). Undiscounted base columns quoted; each page
also shows a token-discount column (BNB −25%/−10%, MNT −25%/−10%) which is NOT the base rate and is
recorded here only so a later reader knows which column the numbers came from.

| Cell | Tier row | Maker | Taker | Source |
|---|---|---|---|---|
| Binance SPOT (USDT pairs) | Regular User | **0.100%** | **0.100%** | https://www.binance.com/en/fee/trading — "Spot & Margin" tab |
| Binance USDⓈ-M perp (USDT pairs) | Regular User | **0.0200%** | **0.0500%** | https://www.binance.com/en/fee/futureFee — "USDⓈ-M Futures" tab |
| Bybit SPOT (crypto-crypto) | Non-VIP | **0.1000%** | **0.1000%** | https://www.bybit.com/en/announcement-info/fee-rate/ — "Spot" tab |
| Bybit linear perp | Non-VIP | **0.0200%** | **0.0550%** | same page — "Perpetual & Futures" tab |

1. **§2.4's spot-vs-perp premise holds at both venues**: spot taker is 2× (Bybit ~1.8×) the perp
   taker, so reusing the perp round-trip figure for cash-and-carry understates cost by roughly half —
   the exact trap the sentence names.
2. **These are the modeled-tier (VIP0/non-VIP) PUBLISHED rows, not any account's fees.** §40d's
   two-clause §2.4 edit authorizes scoring the pre-capital gate on them, labeled; account-real
   authenticated revisions (`/v5/account/fee-rate`, auth — §4 above) remain a precondition of any
   live order. VIP0 is the worst published tier, so the proxy errs against the gate.
3. **Bybit's page states a caveat worth carrying**: "Actual fees charged depend on the displayed fee
   rates applicable to your account after login" — one more reason the provenance class of these rows
   is `hand_recorded`, never `fetched`.

### Family-1 REST history endpoints, observed live (2026-08-01) — the slice B fetch facts `[→ design §41c/§41f, step 4 slice B]`

Verified by the `arblab calibrate fetch` runs that committed the four `datasets/` files (unauthenticated
GETs from this host, pages stored verbatim in the files — the observations below are re-derivable from
the committed bytes):

| Series | Endpoint | Page limit | Shape facts |
|---|---|---|---|
| Binance funding history | `GET https://fapi.binance.com/fapi/v1/fundingRate?symbol=&startTime=&endTime=&limit=` | 1000 | rows ASCENDING; `fundingTime` ms epoch **with millisecond jitter** (observed `1672560000008`, 8 ms past the 08:00 grid instant); `fundingRate` decimal string; `markPrice` empty on old rows; serves 2023-01-01 onward with a complete 8h grid through 2026-01-31 |
| Bybit funding history | `GET https://api.bybit.com/v5/market/funding/history?category=linear&symbol=&startTime=&endTime=&limit=` | 200 | V5 envelope (`retCode`/`result.list`); rows **NEWEST-FIRST**; `fundingRateTimestamp` is a STRING of ms; grid exact (no jitter observed); complete 8h grid 2023-01-01 → 2026-01-31 |
| Binance spot klines | `GET https://api.binance.com/api/v3/klines?symbol=&interval=1h&startTime=&endTime=&limit=` | 1000 | array-of-arrays, index 0 open-time ms, index 1 open-price string; **2023-03-24T13:00Z DOES NOT EXIST** (that day's spot halt — the completeness refusal fired live on the full range, so the committed kline era starts 2023-04-01); complete thereafter |
| Binance mark-price klines | `GET https://fapi.binance.com/fapi/v1/markPriceKlines?symbol=&interval=1h&startTime=&endTime=&limit=` | 1500 (1000 used) | same array shape, volume fields zero; **no 2023-03-24 hole** (perp mark kept printing through the spot halt) — the shared 2023-04-01 era start is the SPOT series' constraint, adopted by both because the basis derivation subtracts them pairwise |

1. **The jitter tolerance is an observed necessity, not caution**: exact-grid matching would refuse
   real Binance funding history on the 8 ms row above. `driving.GridJitterTolerance` (1 minute,
   inclusive) is 1/480th of the funding interval — no row can be ambiguous between grid points.
2. **§38f's "funding is re-fetchable" now has a measured floor**: both venues served the full
   three-year 8h grid with zero gaps. Klines are re-fetchable too, with the one spot-halt hole.
