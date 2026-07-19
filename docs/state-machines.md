# Execution & Safety-Kernel State Machines — Funding-Rate Arbitrage Platform

**Status:** v0.8 (2026-07-19) — structural release. Content frozen from build-spec v0.7.1; reorganization only, no normative change.
**Doc set:** normative core `build-spec.md` · venue facts `venue-profiles.md` · kernel machines `state-machines.md` · history/traceability `adr-history.md` · teaching companion `tutorial.md`.
**Role:** the safety kernel + execution state machines, re-expressed from converged core prose — state lists, transition tables, invariants, chaos cases; the core owns the requirements.

**No new behavior:** every row derives from core v0.7.1 prose; ambiguities are FLAGged, not resolved. Cross-refs: (core §N) = build-spec.md. Quoted prose keeps the core's internal reference style: "§6.7" = core §6 step 7 (the ambiguous-order classifier — §2 here); "§5.3" = core §5.3. The `[→ …]` tags on machine headings are **copies** of the source prose's finding tags for traceability; the tags' normative home stays in the core.

---

## §1. OMS order lifecycle `[→ X1, Y1, Y2, Z1, Z2, R2b-4, R2b-12, R2-#1]`

**Source (v0.7.1):** build-spec §5.3 (identities, order-state table, rules) + §6 steps 5–6 — re-expressed, no new behavior. **Normative owner:** the requirement + tag live in core §5.3 (and §6 steps 5–6); this section is the mechanism.

**Identities (core §5.3).** `ClientOrderID` — deterministic, OMS-minted, unique per submission; a **recovery correlation handle, not a venue idempotency key** (Binance permits reuse once an order is terminal); never reused within the recoverable window. `ExecutionID` — the venue's immutable fill id (Binance `tradeId`, Bybit `execId`), unique only **within** an instrument; `VenueSeq` is *not* an identity.

### States

Order state tracks **submission/finality**; the fill ledger (Invariants below) tracks **realized quantity**. Meanings are the core §5.3 state table's, verbatim:

| State | Meaning |
|---|---|
| `Planned` | intent + client ID journaled; **not yet committed to dispatch** (may be superseded/abandoned before send). Entry context (core §6 step 5): deterministic `ClientOrderID`s minted; `TradePlanned` + intents persisted **and fsynced** to the write-ahead journal **before any submission** |
| `DispatchAuthorized` | **the crash boundary** — a single `Place` for this client ID is **fsync-committed *before* the non-transactional network write**; the send is imminent/in-flight. Recovery resolves this order **by query only** — never resubmit, never infer "not sent" `[→ R2b-4]` |
| `Submitted` | the single `Place` returned a transport result; awaiting/parsing venue ack |
| `SubmissionUnknown` | ambiguous; resolve **only** by `OrderByClientID`/`Fills`, never resubmit |
| `Acknowledged` | live on venue |
| `PartiallyFilled` | some fills applied; more possible ("`PartiallyFilled` explicitly means *more fills possible*", core §6.7) |
| `CancelPending` | cancel requested |
| `CancelUnknown` | cancel ambiguous |
| `Filled` · `Canceled` · `Rejected` | terminal for *state* — the fill ledger stays open (Invariants) |

### Transitions

Every "→ next on" cell of the core §5.3 state table, one row per edge; crash-recovery rows from the core §5.3 rules paragraph.

| From | Event / condition | To | Action / guard |
|---|---|---|---|
| `Planned` | authorize dispatch | `DispatchAuthorized` | **fsync `DispatchAuthorized`** immediately before the single `Place` — "the commit to call `Place` exactly once — the crash boundary" (core §6 step 6) |
| `Planned` | crash recovery while still `Planned`-only | *(replan or discard)* | "a `Planned`-only order was never dispatched (no venue order can exist) → safe to replan or discard" |
| `DispatchAuthorized` | send returns | `Submitted` | the single `Place`; submit **once** per order (no transport retry) |
| `DispatchAuthorized` | no-return / crash | `DispatchAuthorized` (stays) | resolved **by query only** — `OrderByClientID`/`Fills` under the venue-specific stable-not-found rule; **never resubmit, never infer "not sent"** (the single `Place` may have executed with its response lost) `[→ R2b-4]` |
| `Submitted` | ack | `Acknowledged` | — |
| `Submitted` | reject | `Rejected` | — |
| `Submitted` | timeout/5xx | `SubmissionUnknown` | "A timeout/5xx moves the order to `SubmissionUnknown`, resolved by `OrderByClientID`/`Fills`, never resubmitted" (core §6 step 6) |
| `SubmissionUnknown` | query (`OrderByClientID`/`Fills`) | `Acknowledged` / `PartiallyFilled` / `Filled` / `Rejected` | resolve **only** by query, **never resubmit**; acceptance-resolution finality follows the **venue-specific stable-not-found rule** (Invariants; FLAG-2); on Bybit, realtime → history fallback `[→ R2-#1]` |
| `Acknowledged` | fill | `PartiallyFilled` / `Filled` | fill posts to the ledger (dedup → ledger → `PositionAllocation`, one transaction — Invariants) |
| `Acknowledged` | cancel-req | `CancelPending` | — |
| `PartiallyFilled` | more fills | `Filled` | — |
| `PartiallyFilled` | cancel-req | `CancelPending` | — |
| `CancelPending` / `CancelUnknown` | confirmed | `Canceled` | **"a late fill still posts to the ledger"** (entry edge into `CancelUnknown`: FLAG-1) |
| `Filled` / `Canceled` / `Rejected` | — | — | terminal for *state*; "the **fill ledger never closes for a *known* order**" (Invariants) |

### Invariants

- **Orthogonal cumulative-execution fill ledger `[→ Z2]`** (core §5.3): "Fills are *not* gated by order lifecycle: a **cumulative-execution ledger** accepts any deduped fill for a *known* order regardless of its current state — so a post-cancel or post-`SubmissionUnknown` late fill still applies. Order state tracks submission/finality; the fill ledger tracks realized quantity; **net delta is recomputed from the ledger on every fill/position event**" (feeding the §2 classifier).
- **The ledger never closes `[→ R2b-12]`** (core §5.3): "the **fill ledger never closes for a *known* order** — any deduped fill (`UNIQUE(account,venue,product,symbol,execution_id)`) is admissible **indefinitely**, since a legitimate fill can surface after prolonged downtime. The bounded 'finality window' governs only **ambiguous-order *acceptance* resolution** (the stable-not-found rule), **not** fill admission."
- **Dedupe key `[→ Y1, Z1]`** (core §5.3): fills dedupe on `UNIQUE(account, venue, product_category, symbol, execution_id)` — **the single normative key** (spot and perp ID namespaces are independent).
- **Single `Place` `[→ Y2]`:** `Place` is called **once** per order; **transport retries forbidden**.
- **Crash recovery is defined for every pre-ack state `[→ R2b-4]`:** an order at **`DispatchAuthorized` or beyond** is resolved **only** by `OrderByClientID`/`Fills` under the venue-specific stable-not-found rule — **never resubmitted, and "not sent" is never inferred** (the single `Place` may have executed with its response lost). "`ClientOrderID` durability alone does not reveal whether the `Place` ran — the state machine does."
- **Resolution is concurrent, not blocking:** "`SubmissionUnknown`/cancel resolution runs **concurrently** with exposure management — the OMS never blocks flattening of a **determined** residual on it; an **ambiguous** residual (any counter-leg order with remaining executable quantity) is quiesced to finality **first**" (§2 here; core §6.7).
- **Resolving-query outage:** "If the resolving query is unavailable/rate-limited, new trading stays disabled and rate-limit budget is **reserved for recovery**; 'not submitted' is never inferred from a transient not-found — finality follows a **venue-specific stable-not-found rule**."
- **Bybit stable-not-found fallback `[→ R2-#1]`:** "On **Bybit**, `GET /v5/order/realtime` holds only the most-recent ~500 closed orders and is **cleared on a server restart**, so resolution MUST fall back to `GET /v5/order/history` by `orderLinkId` (a filled order is always recoverable there); never infer 'not submitted' from realtime alone."
- **One transaction `[→ Y1, Z1]`:** "Applying a fill (dedup → ledger → `PositionAllocation`), advancing the **per-instrument** REST recovery cursor, and appending journal/outbox events happen in **one transaction**"; the outbox exports at-least-once (core §6 step 6).

### Chaos & acceptance cases (§9 of the core)

From the core §9 chaos list, the clauses exercising this machine:

- **Process kill mid-IOC** — the `DispatchAuthorized` crash boundary: recovery queries, never resubmits ("never infer 'not sent'").
- **Post-cancel late fills** — the `CancelPending`/`CancelUnknown` row + the never-closing ledger: the late fill posts to the ledger.
- **Bybit realtime-cache-cleared → order-history fallback** — the stable-not-found invariant's `GET /v5/order/history` fallback by `orderLinkId`.
- **Identical spot/perp execution IDs** — the dedupe key `UNIQUE(account, venue, product_category, symbol, execution_id)`: spot and perp ID namespaces are independent, so identical IDs are distinct fills, applied once each.
- **Venue rejects · partial fills** — the `Submitted → Rejected` and `Acknowledged/PartiallyFilled` fill edges.

### Flagged ambiguities

- **[FLAG-1]** The entry edge into `CancelUnknown` — and any cancel-lost-race exit — is never stated. The core §5.3 table models `CancelPending` / `CancelUnknown` as one composite row ("cancel requested / cancel ambiguous") whose only stated exit is "confirmed → `Canceled` · a late fill still posts to the ledger". Readings: (i) `CancelPending` → `CancelUnknown` on an ambiguous cancel transport result (timeout/5xx), by symmetry with `Submitted` → `SubmissionUnknown`, with a cancel that loses the race to a full fill exiting to `Filled`; (ii) the composite row is deliberate — no distinct entry edge or race exit is modeled, and the ledger rule alone carries the race. Not resolved here; the transitions table carries only the exits the source states.
- **[FLAG-2]** The terminal state of a **stably-not-found** order is never named. The query-outcome enumeration for `SubmissionUnknown` (and for `DispatchAuthorized`-recovery, whose outcomes the source leaves at "resolved by query") is "`Acknowledged` / `PartiallyFilled` / `Filled` / `Rejected`"; the stable-not-found rule defines *when* acceptance-resolution is final, but the prose never maps the stably-not-found case onto an enumerated state. Readings: (i) `Rejected` — the only never-accepted terminal in the enumeration; (ii) an unnamed abandon/discard disposition analogous to a never-dispatched `Planned`. Not resolved here.

---

## §2. Ambiguous-order classifier & the finality contract `[→ Z2, R2b-B1, R1c-4, S4, X6, Y3]`

**Source (v0.7.1):** build-spec §6 step 7 ("§6.7") + §5.3 rules & command verbs + §6 step 8 + §9 — re-expressed, no new behavior. **Normative owner:** the requirement + tag live in core §6.7 (with §5.3, §6 step 8, §9); this section is the mechanism.

**Scope.** "On **every** fill/position event, recompute net delta from the fill ledger and **classify the residual as *determined* or *ambiguous* before acting** — this classification, not the raw delta, gates the compensating trade." The classifier runs per hedge-group residual, over the §1 fill ledger.

### States

| State | Meaning (core §6.7, verbatim in the load-bearing parts) |
|---|---|
| `Determined` | "the exposure is **fixed because no exposure-changing order on the hedge group has any remaining executable *or unobserved* quantity**: e.g. a `ForcedCloseReason` (ADL/liquidation) event definitively closed a leg with no pending counter-order, or every leg has reached a **venue-terminal state with its cumulative execution reconciled**" |
| `Ambiguous` | "**any counter-leg exposure-changing order still has remaining executable quantity or an unresolved state** (`DispatchAuthorized` / `Submitted` / `SubmissionUnknown` / `Acknowledged` / **`PartiallyFilled`** / `CancelPending` / `CancelUnknown`), so the **sign *or size*** of the true net exposure is not yet fixed — each such order contributes a **possible-delta *interval*, not a point**" `[→ R2b-B1]` |
| `DeadlineExceededAmbiguous` | the finality contract "**cannot complete within its deadline** (queries unavailable/rate-limited)" — "a **deadline-exceeded *ambiguous* state that does NOT blind-compensate** — it holds new trading, alerts the operator, and permits **only provably-safe action**". Safe-action latitude exists "because the bot is the **sole writer**: this is an ambiguous *order*, not an ambiguous *writer* — contrast the §7.4 fence-unavailable branch, where writer identity is in doubt and `$1K`+ stays read-only" (core §7.4) |

**Why acting on the observed delta is forbidden while `Ambiguous`:** "Acting on the *observed* delta is then **wrong in at least one reachable venue state**: sell the filled spot against a perp only *partially* filled (0.4 of 1.0) and the remaining 0.6 fills → **naked short**; re-hedge the perceived gap and the pending quantity fills → **doubled short** (`PartiallyFilled` explicitly means *more fills possible*; a `CancelPending` order can still fill; a `SubmissionUnknown` may be filled). `outstanding_reserved_reductions` dedups *reductions* but does **not** represent this pending exposure."

**The per-venue finality contract** (quiesce-to-finality; "integration-proven, §8/§9" of the core). Needed "because a point-in-time `not-found` cannot collapse an in-flight order — a `DispatchAuthorized` send may land *after* the query, and §5.3 admits late fills":

1. "**tear down / fence that order's dispatch path** (an IOC is immediately terminal at the venue; a still-in-flight send is waited past its max propagation/timeout so it cannot land after the query)";
2. "read **terminal order status with the authoritative final cumulative executed quantity**";
3. "sweep executions through a **venue/server watermark strictly later than the order's cancellation-effectiveness / max landing horizon** — never a synthetic stable-not-found alone";
4. "reconcile position/balance to that **same watermark**."

### Transitions

| From | Event / condition | To | Action / guard |
|---|---|---|---|
| *(any)* | every fill/position event | `Determined` / `Ambiguous` | recompute net delta from the §1 fill ledger; **classify before acting** — the classification, not the raw delta, gates the compensating trade `[→ Z2, R2b-B1]` |
| `Determined` | residual fixed (sign *and* size) | *(reduction issued; re-classified on next event)* | **immediately** (≤5 s, core §9) issue an `ExposureReduction` (core §5.3) sized from **`realized_delta − outstanding_reserved_reductions`**, "tracked as a **fenced reduction generation** — coalesce / amend / cancel-replace it, never a fresh command per event"; "reconciliation is never a precondition for flattening, so the ≤5 s budget is not held hostage to the resolution budget" |
| `Ambiguous` | quiesce **every** unresolved counter-leg order to finality via the 4-step contract above | `Determined` | "Only when every possible-delta interval has collapsed to a **fixed** residual does it issue the state-dependent `ExposureReduction`"; quiesce-to-finality RTO < 30 s (core §9) |
| `Ambiguous` | the contract **cannot complete within its deadline** (queries unavailable/rate-limited) | `DeadlineExceededAmbiguous` | hold new trading; alert the operator; permit **only provably-safe action** |
| `DeadlineExceededAmbiguous` | summed possible-delta interval `[a, b]` does **not** straddle zero (`a ≥ 0` or `b ≤ 0`) | `DeadlineExceededAmbiguous` (floor issued **once**) | issue the reserved **`IntervalFloorReduction`** generation for exactly the endpoint-nearest-zero magnitude (Interval-floor rule below); **never issued twice** |
| `DeadlineExceededAmbiguous` | interval **straddles zero** (`a < 0 < b`) | `DeadlineExceededAmbiguous` | "no nonzero compensation is safe in all states → **de-minimis only**" |
| `DeadlineExceededAmbiguous` | the `IntervalFloorReduction`'s **own** result is ambiguous (timeout / partial fill) | *(re-enters this same finality contract)* | "it **never blind-retries**, and **its own ambiguous result … routes through this same §6.7 finality contract**" |
| `DeadlineExceededAmbiguous` | intervals later collapse to a fixed residual (late resolution / fills) | `Determined` | re-classified on every fill/position event; the determined branch then applies |

**Interval-floor rule `[→ R1c-4]`** (deadline-exceeded branch **only**): "each unresolved counter-leg contributes a possible-delta *interval* `[a, b]`, so the group's net exposure lies in a summed interval; if that interval **does not straddle zero** (`a ≥ 0`, guaranteed net-long — or `b ≤ 0`, guaranteed net-short), the endpoint **nearest zero** is naked in *every* reachable state, so reducing exposure by **exactly that magnitude** can never overshoot into the opposite sign under any resolution" — it "shrinks the worst-case naked window from the full `|b|` to the interval width `(b − a)` while remaining safe". Mechanically: "**The interval-floor action is a distinct, reserved fenced generation (`IntervalFloorReduction`), not a plain `ExposureReduction`** (whose precondition requires a *fixed* residual): before submission it **reserves its own generation**, which subsequent net-delta recomputation **counts in `outstanding_reserved_reductions`** — so a still-in-flight interval-floor order is **not** re-read as fresh naked exposure and the floor is **never issued twice** (else, for `[0.4, 1.0]`, two uncertain `0.4` reductions could reach `−0.4` and defeat the proof)". "It is the **sole** exception to 'no compensation before finality,' and only for the reachable-state-**invariant** floor, never the uncertain part. This applies **only in the deadline-exceeded branch** (normal operation waits for finality and compensates the exact residual); the §6.4 gap budget still MUST cover the un-compensated `|b|` ceiling (core §6 step 4) — interval-floor compensation shrinks the *realized* window, it does not loosen the conservative budget, and does not assume the ≤30 s target always succeeds."

### Invariants

- **Single gate `[→ R2b-B1]`:** "**This classifier is the *single* gate for *every* compensating trade** — one-leg-failure (core §6.7), ADL/liquidation (core §6 step 8), kill-switch / panic-MARKET (core §7.4), protection-disarm (core §7.4; §5 here), `PackageDelever`/`EmergencyClosePackage` (core §5.3; §7 here), and actuator takeover (core §7.4; §4 here) all route through it; **no path compensates the *uncertain part* of an ambiguous residual before the finality barrier** (only the deadline-exceeded **interval-floor generation** may compensate the reachable-state-invariant floor)." A pre-authorized panic-MARKET / venue-native stop "does *not* bypass the §6.7 finality barrier for an ambiguous residual" (core §7.4); "any ambiguous close order routes through the §6.7 classifier" (`CloseTrade`, core §5.3).
- **One canonical boundary:** "the ADL/panic 'immediate' path is the *determined* branch (no pending exposure-changing order), and a forced close *with* a still-live/partial counter-leg order is ambiguous and quiesces first. The §6.7 **finality contract is the *one canonical* boundary**: protection-disarm's sweep, the takeover's reconcile, and `PackageDelever`'s generations all reconcile through *this* contract **and its watermark**, never a weaker cancel-ack-only variant."
- **Forced-close events (core §6 step 8):** "A `ForcedCloseReason` (ADL/liquidation) event fires an immediate `ExposureReduction` **when the residual is determined — no pending exposure-changing order on the group**; if a counter-leg order is still live/partial it routes through the §6.7 **finality barrier first** — never a next-tick reaction either way."
- **Reservation-aware sizing:** the determined branch is sized from `realized_delta − outstanding_reserved_reductions`, "which **includes reserved *generations*** — a `PackageDelever`'s in-progress reserved second leg, a protective order's leg reservation — so an intentional in-progress transient is **not** misread as a naked residual" (§7 here).
- **Safety across reachable states:** assert "**no naked-short created and no doubled/over-reduced short across *every* reachable venue state**" (core §9).
- **Order type of the reduction — the state-dependent function (core §5.3):** "The reduction's order type is the **single state-dependent function of §5.3** — `reduceOnly` set **only** when the order reduces an existing position on that instrument …, all exempt from `stop-opening`" (core §6.7). The core §5.3 cases, in full: "The action is **state-dependent, and `reduceOnly` is set only when the order reduces an *existing position on that instrument*** — never merely because it reduces package delta `[→ R2-#4, Codex]`:
  - **Naked long spot** (e.g. the perp short was ADL'd to zero): either an **inventory-capped spot sell** to flatten (a plain sell — `reduceOnly` doesn't exist for spot), or an **ordinary *non*-reduce-only perp short** to re-hedge — a reduce-only order here would be **rejected**, because the perp position is zero and reduce-only cannot *open*.
  - **Naked short perp** (spot leg undersized/sold): a **reduce-only perp buy** to flatten the existing short, or an **ordinary spot buy** to re-hedge.

  "The choice (flatten vs re-hedge) follows policy + the net-delta invariant; the `reduceOnly` flag follows the *instrument's current position*, not the package. A §9 acceptance case proves ADL-from-a-zeroed-perp re-hedges successfully."
- **`EmergencyClosePackage` as a classifier client (core §5.3):** it "**quiesces existing orders through the §6.7 finality contract**, then **closes *both* legs** (paired-IOC), routing any resulting one-leg residual back through the §6.7 classifier" `[→ R2b-B1]` (its supersession of an active `PackageDelever` generation: §7 here).
- **During the ambiguous window:** "directional risk is bounded by any **pre-placed venue-native protection** (core §7.4) and by the **naked-leg-gap sizing budget over the resolution window** (core §7.1), **never** by a blind compensating trade. Stop new *strategy* orders; if the resolving query is unavailable/rate-limited, hold new trading and reserve rate-limit budget for recovery. Escalate via the pre-authorized ladder (core §7.4), operator alerted in parallel" `[→ S4, X6, Y3]`.
- **Determined-only shortcut:** "'Reconciliation is never a prerequisite for flattening' holds for a *determined* residual, **not** an ambiguous one."

### Chaos & acceptance cases (§9 of the core)

- **Ambiguous one-leg failure** (core §9, mapped in full): "a counter-leg exposure-changing order with remaining executable quantity — `SubmissionUnknown`, **`PartiallyFilled`**, `Acknowledged`, or `CancelPending` → **quiesce-to-finality *before* compensating the *uncertain part***; sub-cases: partial fill before/after cancel-ack, **stable-not-found *followed by* a discovered execution**, and a **deadline-exceeded interval-floor generation** (a reserved `IntervalFloorReduction` that itself times out / partially fills → assert **never issued twice, never overshoots to net-short**); assert **no naked-short created and no doubled/over-reduced short across *every* reachable venue state**."
- **Forced-close/ADL events** — the determined-vs-ambiguous fork of core §6 step 8 (Invariants).
- **Duplicate position events during an outstanding reduction** — the fenced reduction generation: coalesce / amend / cancel-replace, never a fresh command per event.
- **ADL from a zeroed perp** — "re-hedge via a *non*-reduce-only short, never a rejected reduce-only" (the state-dependent order-type function).
- **RTOs (core §9 gate):** **determined** naked-leg detected→reduced **< 5 s** (alert at 2 s); **ambiguous-order quiesce-to-finality < 30 s**; "**max observed *determined*-unhedged-exposure duration < 5 s** — the **ambiguous** window … is bounded separately by the ≤30 s quiesce RTO, and its directional risk is covered by the **sized naked-leg-gap budget (§6.4) + pre-armed protection (§7.4)**, not scored as a `<5 s` violation (§6.7 forbids a blind compensating trade inside it)" `[→ R2b-B1]`.

### Flagged ambiguities

None.

---

## §3. FenceReadyLost arbitration *(Task 4)*

## §4. Actuator takeover protocol *(Task 4)*

## §5. Protective-order disarm *(Task 4)*

## §6. EmergencyPlan lifecycle *(Task 4)*

---

## §7. PackageDelever generations & reserved reductions `[→ R1b-1, C4, C5, P3, X2, R2c-1]`

**Source (v0.7.1):** build-spec §5.3 (`PackageDelever` command) + §7.1 (trigger / top-up / escalation) + §9 — re-expressed, no new behavior. **Normative owner:** the requirement + tag live in core §5.3 + §7.1; this section is the mechanism.

**Definition & entry context (core §5.3, §7.1).** `PackageDelever` is "foundation-originated, **exempt from `stop-opening`**, used to **reduce leverage on a *still-neutral* package** when isolated-margin liq distance breaches and the top-up path fails". The escalation chain that reaches it: the §5.2 `liq_distance` **rebalance** threshold (<20% BTC; tighter for alts) breaches — "recomputed via the OMS-owned mark-risk read port … on every mark update *and* every margin/position/tier-change event" (never account-event-only; "a **stale mark feed blocks openings and escalates** … never silently freezing the trigger" `[→ R2c-1]`) — → the **active top-up** (Binance `POST /fapi/v1/positionMargin {type:1}`; Bybit `POST /v5/position/add-margin {margin:+}`) with its **bounded latency budget** (alert-on-miss) and **reserved rate-limit allocation** → "if the top-up fails or misses its budget (rate-limit, venue lag, insufficient staged margin), the ladder **escalates to a `PackageDelever`** … rather than waiting". It is "deliberately **not** an `ExposureReduction`: the package has **no naked residual**, so an `ExposureReduction` would either no-op (delta≈0) or, by reducing a single leg, *create* a naked leg during the rally."

### States

Generation phases, derived from the core §5.3 `PackageDelever` sequencing prose (phase-boundary caveats: FLAG-3, FLAG-4):

| State | Meaning |
|---|---|
| `Planned` | a **generation** is created: "each **generation is a bounded attempt**" carrying "**explicit per-leg target positions**"; its intended net-long transient is "an **atomically reserved part of the fenced generation**" from creation |
| `PerpReduceInFlight` | the perp-short reduction (buy-to-reduce) is submitted **first** — "which itself relieves the liq risk" |
| `PerpReduceFinal` | the perp reduction is confirmed **to finality** (§2; core §6.7) |
| `SpotReduceReserved` | the interval where only the **reserved spot reduction** remains outstanding (see FLAG-3 — the reservation itself is generation-scoped from creation, not first made here) |
| `SpotReduceInFlight` | the reserved spot reduction is submitted — "**sells no more spot than the confirmed perp reduction**" |
| `Complete` | the generation has reached §6.7 finality; only now is a next generation admissible |
| `Held/Alert` | "a leg that is deadline-exceeded/ambiguous **holds and alerts, never blind-retries or blind-flattens**" |
| `TerminalFlatten` | "if it cannot complete, its **terminal full-flatten routes through the same §6.7 finality contract** (the liquidation-imminent branch), not a blind market dump" |

### Transitions

| From | Event / condition | To | Action / guard |
|---|---|---|---|
| *(no active generation)* | liq-distance rebalance breach **and** the top-up fails or misses its budget | `Planned` | "never wait and never a single-leg `ExposureReduction`" (core §9); "because a lost timeout may mean the top-up actually applied, it **re-queries margin + position first**" — "On an *ambiguous* top-up result (timeout, success unknown) it **re-queries margin + position first** (the add-margin may have applied)" |
| `Planned` | submit the perp-short reduction (buy-to-reduce) | `PerpReduceInFlight` | "the trigger is the **perp short nearing liquidation on a rally**, so it **reduces the perp short *first*** (buy-to-reduce — which itself relieves the liq risk)" `[→ R1b-1]` |
| `PerpReduceInFlight` | perp reduction reaches §6.7 finality (§2) | `PerpReduceFinal` | "**confirms that reduction to finality** (§6.7)" before any spot sell |
| `PerpReduceFinal` | reserved spot reduction pending submission | `SpotReduceReserved` | phase boundary per FLAG-3; the §2 classifier **subtracts the reservation** throughout |
| `SpotReduceReserved` | submit the reserved spot reduction | `SpotReduceInFlight` | "**sells no more spot than the confirmed perp reduction**"; "only the **reserved spot reduction** (never a policy-selected re-hedge) completes the generation" |
| `SpotReduceInFlight` | spot reduction reaches §6.7 finality (§2) | `Complete` | generation done; "each **generation is a bounded attempt made *only after* the prior generation reaches §6.7 finality**" |
| `Complete` | the escalation ladder continues | `Planned` (next generation) | bounded attempts, "**bounded and terminal-escalating, dropping notional/margin requirement**" (core §7.1); a new generation only after the prior generation reaches §6.7 finality |
| `PerpReduceInFlight` / `SpotReduceInFlight` | the leg's resolution is **deadline-exceeded/ambiguous** | `Held/Alert` | "holds and alerts, **never blind-retries or blind-flattens**" (exit on later resolution: FLAG-4) |
| `PerpReduceInFlight` / `SpotReduceInFlight` | a **determined** leg failure | *(continue toward the per-leg targets)* | "On a *determined* leg failure it **completes the target** (never buying the spot back to restore neutrality *and* the original notional — that would re-arm the same liq breach and loop)" |
| *(any active state)* | the generation **cannot complete** | `TerminalFlatten` | full-flatten routes through the §6.7 finality contract (the liquidation-imminent branch), "not a blind market dump" |
| *(any active state)* | `EmergencyClosePackage` issued | *(superseded — generation ends)* | "It **supersedes and quiesces any active `PackageDelever` generation** (cancelling its reserved second leg to §6.7 finality first)" (core §5.3) |
| *(any active state)* | `FenceReadyLost` arbitration fires | *(superseded — generation ends)* | the arbitration "**supersedes any active `PackageDelever` generation** (quiescing its reserved leg to §6.7 finality first)" (core §7.4; §3 here) |

### Invariants

- **Perp-short-first sequencing — never net-short `[→ R1b-1]`:** the reduction is "**sequenced so the transient never worsens the at-risk direction**": perp short reduced *first*, confirmed to finality, then "**sells no more spot than the confirmed perp reduction** — the momentary exposure is thus net-*long* (safe in the triggering rally), **never net-short**."
- **The reserved second leg vs the §2 classifier:** "That intended net-long transient is an **atomically reserved part of the fenced generation**, so the §6.7 classifier **subtracts the reservation** and does **not** misread it as a naked residual to re-hedge — only the **reserved spot reduction** (never a policy-selected re-hedge) completes the generation."
- **Bounded generations, gated on finality:** explicit per-leg target positions; "each **generation is a bounded attempt made *only after* the prior generation reaches §6.7 finality**"; the escalation ladder is "bounded and terminal-escalating, dropping notional/margin requirement" (core §7.1).
- **Never blind:** a deadline-exceeded/ambiguous leg "**holds and alerts, never blind-retries or blind-flattens**"; the terminal full-flatten routes through the §6.7 finality contract, never a blind market dump.
- **Completes the target on a determined leg failure** — "never buying the spot back to restore neutrality *and* the original notional (that would re-arm the same liq breach and loop)."
- **Not an `ExposureReduction`:** "the package has **no naked residual**, so an `ExposureReduction` would either no-op (delta≈0) or, by reducing a single leg, *create* a naked leg during the rally" (also core §7.1: "the package is still neutral, so reducing a single leg would *create* a naked leg").
- **Ambiguous top-up result ⇒ re-query first:** "(timeout, success unknown) it **re-queries margin + position first** (the add-margin may have applied)" — a lost timeout may mean the top-up actually applied.
- **Supersession quiesces the reserved leg first:** both superseding paths (`EmergencyClosePackage`, core §5.3; `FenceReadyLost`, core §7.4/§3 here) cancel/quiesce the generation's reserved second leg **to §6.7 finality first**.
- **Escalate, don't wait:** "A slow top-up under stress is the same failure class §7.1 warns about for spot→futures transfers, recreated one level down, so it gets a pre-authorized fallback" (core §7.1).

### Chaos & acceptance cases (§9 of the core)

- **Margin top-up under stress** (core §9, mapped in full): "liq-distance breach → `positionMargin`/`add-margin` top-up within budget; on failure/slow → **re-query margin/position, then `PackageDelever`** (**perp-short-first-sequenced**, transient **never net-short**, bounded + terminal-flatten escalation), never wait and never a single-leg `ExposureReduction`."
- **Mark-price-driven liq breach** (trigger side, core §9): "a pure rally moves mark toward the short's liq price with **no** account event → liq distance recomputed on the **mark stream** → rebalance/top-up fires; a **stale mark feed** blocks openings + escalates, never a frozen trigger" `[→ R2c-1]`.

### Flagged ambiguities

- **[FLAG-3]** Whether the interval between perp-reduce finality and spot submission is a **distinct modeled state** (`SpotReduceReserved`) is under-determined. The prose sequences "**confirms that reduction to finality** (§6.7), **then sells no more spot** than the confirmed perp reduction", and separately makes the reservation **atomic with the fenced generation** (not an event at this boundary). Readings: (i) a distinct await-state between `PerpReduceFinal` and `SpotReduceInFlight` (as tabled here); (ii) a direct `PerpReduceFinal` → `SpotReduceInFlight` hand-off, the reservation being a generation-scoped property throughout. Either way the reservation exists from generation creation and the §2 classifier subtracts it; no behavior differs, only the state count.
- **[FLAG-4]** The exit from `Held/Alert` on **later** resolution of the held leg's ambiguity is never stated. The prose enumerates: deadline-exceeded/ambiguous → "holds and alerts, never blind-retries or blind-flattens"; a *determined* leg failure → "completes the target"; cannot complete → `TerminalFlatten`; superseded → quiesce reserved leg first. Readings: (i) when the held leg's §6.7 contract eventually reaches finality, the leg outcome becomes determined and the generation resumes/completes the target (composing the determined-leg-failure sentence); (ii) `Held/Alert` is terminal-until-operator — hold + alert is the final automated word, exits only via `TerminalFlatten` or supersession. Not resolved here; the transitions table carries only the stated exits.
