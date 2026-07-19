# Venue Capability Profiles — Binance USDⓈ-M / Bybit V5

**Status:** v0.8 (2026-07-19) — structural release. Content frozen from build-spec v0.7.1; reorganization only, no normative change.
**Doc set:** normative core `build-spec.md` · venue facts `venue-profiles.md` · kernel machines `state-machines.md` · history/traceability `adr-history.md` · teaching companion `tutorial.md`.
**Role:** per-venue facts + quirk matrices behind the core's venue-contract requirements (`build-spec.md` §4); every fact dated with its verification source (§4 here).

## 1. Per-venue quirk matrix (perp/futures)

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
| Order query | `GET /fapi/v1/order` by `origClientOrderId`; `userTrades` for fills | `GET /v5/order/realtime` (open + last ~500 closed, **cleared on server restart**) → fall back to **`GET /v5/order/history`** by `orderLinkId`; `/v5/execution/list` for fills `[→ R2-#1]` |
| Attached protection | **No atomic attach-on-entry**: New Order carries no TP/SL param; conditional types are **separate Algo-Service orders** (`POST /fapi/v1/algoOrder`, since 2025-12-09; sending via `/order` returns **-4120**); **no futures OCO/bracket** → a pre-armed **external actuator is MVP-mandatory** on the perp leg for the first-fill→ack window (§7.4). SPOT brackets via `orderList/otoco` (LIMIT working order only) | **Venue-native TP/SL attaches atomically** on `POST /v5/order/create` — but the create response is an **async accept**; the fill *and* the armed protection are confirmed on the **private WS** (`order`/`execution`), not the HTTP ack. Baseline host-independent cover on the **perp** `[→ R1-F2, R2-#4, R2b-B2]` |
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
| Rescue margin lives in | **staged in the futures wallet, *outside* the isolated position** — applied by an explicit **`positionMargin {type:1}` top-up** when liq distance breaches rebalance (Isolated ⇒ free wallet is *not* seen by the position's liq engine; spot-wallet USDT is invisible to the futures engine, and spot→futures transfers degrade under stress); §7.1 | **staged in the UTA, *outside* the isolated position** — applied by an explicit **`position/add-margin` top-up** on the same trigger (Isolated ⇒ per-position margin only; auto-add-margin OFF); §7.1 |
| Posture assert-or-refuse (boot + pre-trade) | `GET /fapi/v1/multiAssetsMargin`==false; `GET /fapi/v1/symbolConfig`.marginType==ISOLATED (or `positionRisk`); Portfolio-Margin OFF; `positionRisk.isAutoAddMargin`==false (**read-only** — no REST setter, off by default); **one-way** `positionSide/dual`.dualSidePosition==false; configured leverage matches pin | `GET /v5/account/info`: **marginMode==ISOLATED_MARGIN** (the load-bearing check), unifiedMarginStatus ∈ {3,4,5,6} = any UTA (classic **1** refused; 5/6 expected for new accounts), spotHedgingStatus==OFF; `GET /v5/spot-margin-uta/status`==off; `GET /v5/account/wallet-balance` every coin borrowAmount==0 ∧ spotBorrow==0; **auto-add-margin OFF for every configured linear symbol** (not only open positions); **one-way** `positionIdx==0` (`position/list`); configured leverage matches pin |

## 3. Spot-leg surface

**Spot-leg facts — separate hosts/streams/filters; no mark/funding exists on spot (core §4.2):**

| Spot concern | Binance SPOT (`api.binance.com`) | Bybit SPOT (`category=spot`) |
|---|---|---|
| Order / query / fills | `POST /api/v3/order`, `GET /api/v3/order`, `/api/v3/openOrders`, **`/api/v3/myTrades`** (fills) — *not* the futures `userTrades` | V5 `/v5/order/*` with `category=spot`; `/v5/execution/list` |
| User-data stream | **separate listenKey** — `POST /api/v3/userDataStream` on the spot host (+PUT keepalive ~30 m, DELETE), distinct from `/fapi/v1/listenKey`. **Deprecated →** new path is WS-API `userDataStream.subscribe` needing **Ed25519 `session.logon`** (pin the auth path) | private `order`/`execution`/`wallet` on the spot private WS |
| Trading filters | `GET /api/v3/exchangeInfo` → `LOT_SIZE` (stepSize), `PRICE_FILTER` (tickSize), `NOTIONAL`/`MIN_NOTIONAL` | `instruments-info category=spot` (lot/tick/min-notional) |
| Book stream | `<sym>@depth`/`@depth5\|10\|20` on `wss://stream.binance.com` (init snapshot `GET /api/v3/depth`) — *not* fstream | `orderbook.{1\|50\|200}.<sym>` on `wss://stream.bybit.com/v5/public/spot` (category set by URL path) |
| Native bracket | `orderList/otoco` (OTOCO) / `oco` — atomic spot bracket, **LIMIT working order only** | TP/SL on the spot order where supported |
| Mark / index / funding | **none — spot has no mark or funding.** Price & freshness-check the spot leg from its **own book (best bid/ask / last)**; `StreamMark` is **perp-only**. Bybit spot ticker's `usdIndexPrice` is a USD-conversion ref, **not** a mark | same — no mark/funding on spot |

So `MarketDataSource.StreamMark` is defined **only for perp instruments** (below), and the spot leg's valuation/freshness reads its book. Rate-limit governors are **per `(venue, API family)`** — `(binance, spot)` and `(binance, futures)` report and enforce *separate* used-weight/order budgets via their own `exchangeInfo`, so they are distinct **weight/order** buckets, not one venue bucket — though the **IP-ban envelope is treated as shared per venue** (inferred, not documented; core §7.6).

## 4. Verification record (dated sources)

Verified against official docs 2026-07-18 / 2026-07-19 (from the v0.7 review record §2, frozen there):

### Binance USDⓈ-M — isolated margin, posture, keys
- **Isolated margin:** only the position's allocated isolated margin is at stake; **free futures-wallet USDT does not passively back an isolated position** — rescue is an active add-margin call. → https://academy.binance.com/en/glossary/isolated-margin · https://www.binance.com/en/support/faq/how-to-adjust-margin-balance-in-cross-isolated-margin-mode-360038447311
- **Add isolated margin:** `POST /fapi/v1/positionMargin` (`type` 1=add, 2=reduce). → https://developers.binance.com/docs/derivatives/usds-margined-futures/trade/rest-api/Modify-Isolated-Position-Margin
- **Auto-add-margin:** `isAutoAddMargin` is **READ-only** via `GET /fapi/v2/positionRisk` (or v3) — **no public REST setter** on USDⓈ-M; off by default → read-verify `false`. → https://developers.binance.com/docs/derivatives/usds-margined-futures/trade/rest-api/Position-Information-V2
- **Position mode read:** `GET /fapi/v1/positionSide/dual` → `dualSidePosition` (true=Hedge, false=One-way). Hedge mode: `reduceOnly` **cannot be sent** (param-level restriction; -2022 "ReduceOnly Order is rejected" is the *generic* error, not the Hedge mechanism). → https://developers.binance.com/docs/derivatives/usds-margined-futures/account/rest-api/Get-Current-Position-Mode · .../trade/rest-api/New-Order
- **Margin-mode events:** `ACCOUNT_CONFIG_UPDATE` fires on leverage (`ac`) and multi-assets (`ai`); **isolated/cross margin-type** change surfaces via `ACCOUNT_UPDATE` reason `MARGIN_TYPE_CHANGE`. **Position-mode (one-way/hedge) change emits NO event → REST poll.** → .../user-data-streams/Event-Account-Configuration-Update · .../Event-Balance-and-Position-Update
- **Key management / fence:** a **regular account has NO public REST endpoint to disable/delete/IP-edit its own key** (UI + 2FA only; read-only `GET /sapi/v1/account/apiRestrictions`). API-automatable **only via the sub-account program with a MASTER key**: `DELETE /sapi/v1/sub-account/subAccountApi`, permission `POST /sapi/v1/sub-account/subAccountApiPermission`, IP `POST /sapi/v2/sub-account/subAccountApi/ipRestriction`. → https://developers.binance.com/docs/wallet/account/api-key-permission · https://developers.binance.com/docs/sub_account/api-management

### Bybit V5 — isolated margin, posture, keys
- **Isolated margin (UTA):** loss capped at the position's margin; **no auto-transfer from available balance** unless Auto-Margin Replenishment (auto-add-margin) is on. → https://www.bybit.com/en/help-center/article/UTA-Trading-Rules · https://medium.com/bybit/what-is-the-isolated-margin-mode-how-to-add-auto-margin-replenishment-to-a-position-3d9bd61ccf2
- **Add/reduce margin:** `POST /v5/position/add-margin` (doc slug `manual-add-margin`; **path is `add-margin`** — a known slug/path mismatch). `margin` positive adds, negative reduces. → https://bybit-exchange.github.io/docs/v5/position/manual-add-margin
- **Auto-add-margin:** `POST /v5/position/set-auto-add-margin` (`autoAddMargin` 0=off). → https://bybit-exchange.github.io/docs/v5/position/auto-add-margin
- **Position mode:** read `GET /v5/position/list` `positionIdx` (0=one-way); set `POST /v5/position/switch-mode` (0=one-way, 3=hedge — enum differs from `positionIdx`). Isolated/cross must be read from **`GET /v5/account/info` `marginMode`**, NOT the deprecated position `tradeMode` (always 0). → https://bybit-exchange.github.io/docs/v5/position/position-mode
- **create-order ack:** the response is an **async accept** (returns `orderId`/`orderLinkId`); status/fills must be confirmed via the private WS — *"This request is asynchronous so please use the websocket to confirm the order status."* → https://bybit-exchange.github.io/docs/v5/order/create-order
- **Spot TP/SL + IOC:** spot **Limit** orders support attached TP/SL; spot supports IOC; **combining attached-TP/SL with an IOC spot order is doc-silent → integration-test item** (neither confirmed nor denied). → https://bybit-exchange.github.io/docs/v5/order/create-order
- **Sub-key fence:** modify `POST /v5/user/update-sub-api` (slug `modify-sub-apikey`), delete `POST /v5/user/delete-sub-api`. **Requires an authority holding ONE OF `Account Transfer` / `Sub Member Transfer` / `Withdrawal`** (a sub key may modify itself; a master key may manage its sub) — a **high-privilege, transfer/withdrawal-class** credential. → https://bybit-exchange.github.io/docs/v5/user/modify-sub-apikey · https://bybit-exchange.github.io/docs/v5/user/rm-sub-apikey

### SQLite durability (Linux target)
- **`PRAGMA fullfsync` is macOS-only** (`F_FULLFSYNC`); a **no-op on Linux**. → https://sqlite.org/pragma.html#pragma_fullfsync
- **WAL durability:** `synchronous=NORMAL` in WAL is durable across an *app* crash but **may lose the last transaction(s) on power loss**; **`synchronous=FULL` fsyncs the WAL on every commit** → power-loss durable. → https://sqlite.org/pragma.html#pragma_synchronous · https://sqlite.org/wal.html
- **Durability ≠ checkpoint** (a committed txn is durable in the fsync'd WAL before checkpoint; `wal_autocheckpoint` default 1000 pages). SQLite assumes the OS/hardware honor fsync; **network filesystems (NFS) are documented as corruption-prone** → local ext4/xfs with barriers only. → https://sqlite.org/atomiccommit.html · https://sqlite.org/howtocorrupt.html
- **Production (Linux):** `journal_mode=WAL; synchronous=FULL;` local fs; boot-verify pragmas.
