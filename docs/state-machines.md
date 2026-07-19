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
| `DeadlineExceededAmbiguous` | the finality contract "**cannot complete within its deadline** (queries unavailable/rate-limited)" — "a **deadline-exceeded *ambiguous* state that does NOT blind-compensate** — it holds new trading, alerts the operator, and permits **only provably-safe action**". Safe-action latitude exists "because the bot is the **sole writer**: this is an ambiguous *order*, not an ambiguous *writer* — contrast the §7.4 fence-unavailable branch, where writer identity is in doubt and `$1K`+ stays read-only" (core §6.7; contrast §7.4) |

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

## §3. FenceReadyLost arbitration `[→ R2c-2, Cx4-5, Codex]`

**Source (v0.7.1):** build-spec §7.4 (the `Fence` bullet: the `FenceReady` health gate, the `FenceReadyLost` branches, the fence-unavailable failure branch) + §9 — re-expressed, no new behavior. **Normative owner:** the requirement + tag live in core §7.4 (tier bindings: core §1.1); this section is the mechanism. Ownership split with §4: this section owns the **arbitration** (dimension tracking, first-safe-action, the three branches, the RTO structure, the tiered fence-unavailable floor); §4 owns takeover/epoch mechanics and is referenced, never restated.

**Scope.** "`FenceReady` — a continuously-monitored health gate, not a one-time preflight" `[→ R2c-2]`: "a successful provisioning-time propagation test is **not** an availability guarantee during a later incident", so the primary `Fence`'s health is "**polled continuously**" (`FenceGateway.FenceStatus`, core §4 ports: "CONTINUOUSLY polled health — blocks openings + winds down open exposure on loss"). The arbitration branches instead of taking one wind-down path "because the naïve "bot winds down" path is **impossible exactly when the signer itself failed** (the bot is alive but cannot sign `EmergencyClosePackage`/`ExposureReduction`, since signing readiness feeds `FenceReady`)".

### States

"**`FenceReady` is a differentiated *state*, not one Boolean `[→ Cx4-5, Codex]`**" — it tracks four dimensions (core §7.4, verbatim):

- **signing readiness** — "can the bot sign at all?"
- **primary-revoke readiness** — "can the actuator **durably latch** a revoke/starve the signer honors across restart?"
- **backup-fence readiness** — exchange-side (mechanism: §4 here)
- **durably-fenced** — "a **latched, monotonic revoke *epoch*** — `VerifyFence` validates the epoch, **not** a point-in-time snapshot" (epoch semantics owned by §4 here)

Machine states (the branch labels name the source prose's numbered branches 1–3):

| State | Meaning |
|---|---|
| `FenceReady` | the differentiated health state above, continuously monitored; normal trading posture |
| `FenceReadyLost` | the explicit arbitration entered on loss of any dimension: the first-safe-action is applied, then a branch is selected on *which* dimension failed |
| `WindDown` (branch 1) | "Revoke-control lost, signing still works" — bot-driven wind-down through the defined verbs |
| `TakeoverTransfer` (branch 2) | "Signing unavailable *and* a durable revoke is latchable" — execution transfers to the actuator takeover (§4 here) |
| `ManualOnly` (branch 3) | "Fence state uncertain" — bounded pre-armed protection + operator manual kill only |

### Transitions

| From | Event / condition | To | Action / guard |
|---|---|---|---|
| `FenceReady` | loss of any dimension | `FenceReadyLost` | "Loss triggers an explicit **`FenceReadyLost` arbitration** that first **blocks `OpenTrade`** and **supersedes any active `PackageDelever` generation** (quiescing its reserved leg to §6.7 finality first)" (§7 here) ", then **branches on *which* dimension failed**"; the automated **first-safe-action** (block openings + confirm pre-armed protection) carries its own always-holds RTO (Invariants) |
| `FenceReadyLost` | "Revoke-control lost, signing still works" | `WindDown` (branch 1) | "**bot-driven wind-down** through the defined verbs — `EmergencyClosePackage` for a *neutral* package (an `ExposureReduction` would no-op at delta≈0), `ExposureReduction` only for a *fixed* residual — each routed through the §6.7 classifier" (§2 here). "This **overrides a configured `hard-halt-and-alert`**" (kill-switch mode (c), core §7.4): "halting while the fence is degrading is worse than a controlled exit — an explicit priority choice, not an arbitration conflict" |
| `FenceReadyLost` | "Signing unavailable *and* a durable revoke is latchable" | `TakeoverTransfer` (branch 2) | "the actuator **latches a durable, monotonic takeover epoch**" — "the same **fence-first** revoke/starve primitive as the takeover protocol" (§4 here, which owns the epoch semantics: honored across restart until explicit operator clearance; `VerifyFence` validates **that epoch**, never a point-in-time snapshot) — "only then does execution **transfer to the actuator takeover**, reducing the reconciled residual" (§4 here) |
| `FenceReadyLost` | "Fence state uncertain" — "no durable revoke latch achievable, and signing cannot be confirmed" | `ManualOnly` (branch 3) | "**bounded pre-armed protection + operator manual kill only**, never a compensating trade *or an un-latched takeover* that could create **two writers**". Branch membership rule (core §7.4): "a signer merely unreachable *without* a durable latch is **branch 3**", not branch 2 — because a point-in-time `VerifyFence` alone is insufficient (§4 here) |

**The tiered fence-unavailable floor** (core §7.4 "Failure branch — fence unavailable at incident time"; stated here, consumed by §4's failed-latch edge): "fence-first is otherwise absolute (never two writers), so a *down* fence while the bot host is *also* dead would **deadlock the actuator while naked** — the precise failure the actuator exists to prevent. The resolution differs by tier, because **interval-floor safety proves only that a reduction cannot *overshoot the sign*, not that the actuator is the *sole writer*** — an unfenced actuator trade can still collide with a recovering bot:"

- **At `$1K`+ (single-writer guaranteed):** "if **no durable fence latch** can be established (neither a latched revoke epoch nor a validated `VerifyFence`), the actuator stays **read-only** — **wait `T_fence` → on miss, alert + operator manual kill (venue UI/2FA) *only*; it does NOT automatically trade**, because a transient double-write is *not* acceptable where single-writer is promised. The prior `FenceReady` wind-down (run while the bot was alive) is what keeps a position from reaching this point un-fenced; read-only + manual covers the residual."
- **In the pre-`$1K` canary only (core §12):** "the actuator MAY act on an **interval-floor / de-minimis residual** after `T_fence` — but this is the **separately-signed accepted bounded loss of the canary**, which **explicitly does *not* claim single-writer safety** (the fence/takeover isn't built yet); it is never presented as a `$1K`+ guarantee."

(The writer-identity contrast with `DeadlineExceededAmbiguous` — ambiguous *order* vs ambiguous *writer* — is stated in §2's state table.)

### Invariants

- **RTO structure (core §7.4, verbatim):** "Each branch carries its own §9 target — an **automated first-safe-action RTO** (block openings + confirm pre-armed protection) that always holds, a **composite wind-down / takeover RTO** for branches 1–2 (which may include a §6.7 finality step), and a **manual-kill escalation *target*** for branch 3 (not an automated RTO)." Numbers (core §9): first-safe-action **< 5 s**; branch-1/2 composite **< 30 s**; branch 3 is a manual target, never an automated RTO (Chaos below).
- **The honest guarantee (core §7.4, verbatim):** "The honest guarantee is a **bounded first-safe-action, then wind-down or takeover within the composite RTO** — *not* "never outlives its fence": an *ambiguous* residual still quiesces to finality first (which can delay closure), so what is guaranteed is "**exposure is reduced toward flat within the RTO, never knowingly grown while `FenceReady` is lost, and never left with two writers**.""
- **Why the wind-down exists (tier gap, core §7.4):** "This closes the gap where "`$1K` requires a working fence" (§1.1) otherwise met only an incident-time **read-only + manual** fallback: exposure is wound down (or handed to the actuator) *before* an incident, rather than discovered fence-dead at host-death time."
- **Openings stay blocked while lost:** `OpenTrade` is blocked by the first-safe-action and exposure is "never knowingly grown while `FenceReady` is lost" (the guarantee above) — the block is scoped to the loss, not to a one-shot check.
- **Tier bindings stay core-owned:** the `$1K` working-fence requirement and the actuator tiering ("a *minimal single-node* actuator with the fence-first protocol above + a plan **integrity check**" at `$1K`; `$10K+` hardens it; "The watchdog escalates (i)→(iii) on host silence") are core §1.1 / core §7.4 requirements — referenced here, owned there.

### Chaos & acceptance cases (§9 of the core)

- **`FenceReady` loss** (core §9, mapped in full): "differentiated state: revoke-lost-but-signing-works → bot wind-down; **signing-unavailable + `VerifyFence`-proves-fenced → actuator takeover**; uncertain → bounded-protection + manual, **never two writers**) → new openings blocked (first-safe-action) + **branch-appropriate RTOs met** (branch-1/2 wind-down/takeover composite; branch-3 → manual escalation)" `[→ R2c-2, Cx4-5]`.
- **RTO gate (core §9):** "**`FenceReadyLost` first-safe-action (block openings + confirm pre-armed protection) < 5 s** with a **branch-1/2 wind-down or durable-fence takeover composite < 30 s** (may include a §6.7 finality step) and **branch-3 = a manual-kill escalation *target*, not an automated RTO**".
- **Fence-unavailable** — the tiered floor above, exercised inside §9's actuator-takeover-fence clause (§4 here): "**fence unavailable** (no durable latch → **actuator read-only + operator manual kill at `$1K`+**, never an unfenced trade; de-minimis unfenced action only in the signed canary regime, §12, which claims no single-writer safety)".
- **Fence-race** — the core promises it: "Fence-unavailable and fence-race are §9 cases" (core §7.4). No §9 sub-case is *named* fence-race; the closest named exercise of the race (a second writer racing the fence) is the takeover clause's **bot-recovery-during-takeover** sub-case (§4 here).

### Flagged ambiguities

- **[FLAG-5]** Branch coverage of the dimension space is under-determined. The arbitration "branches on *which* dimension failed" and `FenceReady` tracks **four** dimensions, but the three branch guards name only revoke-control, signing, and durable-latch conditions — the branch for a **backup-fence-readiness-only** loss (signing confirmed working, primary revoke still latchable) is never stated. Readings: (i) with signing and primary-revoke intact no branch condition fires — the loss degrades redundancy only, and the stated response reduces to the first-safe-action posture + alerting, without a wind-down; (ii) every dimension loss must select a branch, and the conservative default for a degraded fence with working signing is branch 1's wind-down. Not resolved here; the transitions table carries only the three stated branch guards.
- **[FLAG-6]** Exits from the branches are never stated. No edge is given for `FenceReady` being **restored** mid-arbitration (does a started wind-down abort or run to completion?), nor for **re-arbitration** when a branch condition changes (branch 3 → branch 2 once a durable latch becomes achievable; branch 3 → branch 1 if signing is re-confirmed). The only stated endings are: branch 2 transfers execution to the takeover (whose fence persists "until explicit operator clearance", §4 here), branch 3 ends in operator manual kill, and openings stay blocked "while `FenceReady` is lost". Readings: (i) the arbitration is one-way per incident — a branch runs to its terminal outcome and normal operation resumes only via operator/boot paths (consistent with core §7.5's reconcile-only boot and branch 2's operator clearance); (ii) the continuously-polled `FenceStatus` re-arbitrates as dimensions change. Not resolved here.

---

## §4. Actuator takeover protocol `[→ Codex, R1b-2, R2b-3, R1c-2, R1c-3, R2c-2, R2c-3, R1-meta]`

**Source (v0.7.1):** build-spec §7.4 (takeover bullet + `Fence` capability bullet + tiering/subproject bullets) + the §4 ports (`FenceGateway`, `ProtectionGateway`) + §9 — re-expressed, no new behavior. **Normative owner:** the requirement + tag live in core §7.4; this section is the mechanism. Ownership split: this section owns **takeover/epoch mechanics**; the `FenceReadyLost` arbitration that can hand over to it, and the tiered fence-unavailable floor, are owned by §3 here; the `EmergencyPlan` lifecycle it consumes is §6 here.

**Scope.** "**Takeover is a fence-first protocol, not a fire-and-forget sell `[→ Codex]`.** The actuator is a *second writer*, so before it trades it MUST **fence the bot's trading credential/signer** (via the venue-specific **`Fence` capability** defined below — exchange-side credential disable *or* signer isolation) — single-writer is preserved by *transferring* the writer, never by running two." The protocol is "a bounded distributed-systems subproject" (core §7.4): "Three convergence rounds each surfaced a deeper takeover requirement (fence-first → reduce the *reconciled residual* → quiesce *all* resting orders to finality → replicate *full* recovery handles)"; the spec fixes the invariants below, and the protocol itself "MUST be **designed, prototyped, and chaos-tested as its own workstream** (host-death mid-`DisarmPending`, mid-reduction, partition, late-fill), not assumed correct from prose" — the recommended sub-`$1K` de-risking path stays core §12.

### States

Protocol phases, derived from the core §7.4 takeover bullet's mandatory sequence (fence → quiesce-to-finality → reconcile → reduce; the prose sequences the steps, it does not name states):

| State | Meaning |
|---|---|
| *(standby)* | the actuator holds replicated plans (§6 here) with watcher→actuator auth and independent key/IP (core §7.4); "The watchdog escalates (i)→(iii) on host silence" (core §7.4) |
| `FenceLatch` | latching the fence: a durable, monotonic revoke/takeover **epoch** via the `Fence` capability (mechanisms below) |
| `QuiesceSurfaces` | the finality barrier: every bot-owned exposure-changing order quiesced to finality across both product families |
| `Reconcile` | executions/positions reconciled via the §6.7 finality contract (§2 here) at the **same watermark** |
| `ReduceResidual` | the **reconciled live residual** reduced — a §2-classifier client |
| `FenceUnavailable` | "**no durable fence latch** can be established" — the tiered fence-unavailable floor governs (§3 here) |

### Transitions

| From | Event / condition | To | Action / guard |
|---|---|---|---|
| *(standby)* | the bot host *and its local journal* die (watchdog: host silence) | `FenceLatch` | the emergency actuator "MUST act after the bot host *and its local journal* die" (core §7.4); if the actuator itself restarted, its own reconcile-only boot first enumerates active plans via `EmergencyPlanStore.ListActive()` and applies the latest generation per trade (§6 here) |
| *(standby)* | §3 branch-2 hand-off ("Signing unavailable *and* a durable revoke is latchable") | `FenceLatch` | the hand-off *is* this machine's fence-first step: the epoch is latched first; execution transfers only once latched (§3 here) |
| `FenceLatch` | durable epoch latched; `VerifyFence` validates **the epoch** | `QuiesceSurfaces` | fence-first is absolute — "before it trades it MUST **fence the bot's trading credential/signer**"; epoch semantics in Invariants |
| `FenceLatch` | "**no durable fence latch** can be established" | `FenceUnavailable` | the tiered floor (§3 here): at `$1K`+ read-only — wait `T_fence` → on miss, alert + operator manual kill only; pre-`$1K` canary = the separately-signed bounded-loss regime |
| `FenceUnavailable` | a durable latch is established within `T_fence` | `QuiesceSurfaces` | the floor's condition ("**no durable fence latch** can be established") no longer holds, so fence-first proceeds; the floor's stated consequences are for the `T_fence` **miss** (§3 here) |
| `QuiesceSurfaces` | every bot-owned exposure-changing order reaches finality | `Reconcile` | "Fencing stops *new* submissions but does **not** cancel orders the venue already holds, so the takeover then **quiesces every bot-owned exposure-changing order to finality across both product families** — entries, reductions, and **all resting protection** (conditional / algo / TP-SL / OCO / trailing / attached), which are **separate cancel surfaces** (Binance cancel-all-*algo* is distinct from cancel-all-orders; Bybit stop/OCO need distinct filters and ack asynchronously) — a **finality barrier `[→ Codex]`**" (port: `ProtectionGateway.EnumerateResting`/`CancelToFinality`, core §4) |
| `Reconcile` | reconcile completes at the watermark | `ReduceResidual` | "Only after that barrier does it **reconcile executions/positions via the §6.7 finality contract (same watermark)**" (§2 here — "never a weaker cancel-ack-only variant", §2's one-canonical-boundary invariant) |
| `ReduceResidual` | the reduction is issued (routed through the §2 single gate) | *(complete — the fence stays latched until explicit operator clearance)* | "**reduce the reconciled live residual**, *not* the pre-allocated qty (which may be stale: an entry leg can still have filled, or a partial reduction already run)" |
| *(any takeover state)* | the bot recovers mid-takeover | *(bot fenced, reconcile-only)* | "A bot that recovers mid-takeover finds itself fenced and stays in reconcile-only until an operator clears it" |

**The `Fence` capability — "per-venue, verify-and-pin, with a failure branch" `[→ R1b-2, R2b-3, R1c-2, R1c-3, R2c-2]`** (core §7.4; the dated per-venue endpoint facts live in Profiles §4):

- **Definition + availability:** "Fencing = guaranteeing single-writer by making the bot unable to trade; it has **two mechanisms**, and **at least one MUST be available and propagation-tested per venue before that venue reaches `$1K`**. **Signer isolation is the *primary* mechanism on both venues** (no venue credential, no money-moving blast radius, account-tier-independent, §7.3); exchange-side disable is the **backup**".
- **Signer isolation (primary, core §7.3):** "the order-signer runs in **its own trust boundary the actuator can fence independently of the bot host** — a minimal signer daemon (a **fourth failure domain**, or a credential the actuator solely owns and can rotate), reached by the bot over an **authenticated signing protocol**, with a **health test** (feeding `FenceReady`" — §3 here — ") and a defined **revoke/starve** mechanism. A signer that merely lived on — and died with — the bot host would **not** be re-fenceable when the bot *restarts* mid-takeover, so **co-location with the bot is not signer isolation**. It holds the **trade** credential (within the §7.3 subaccount blast radius) but **no additional key-management/transfer-class credential** and has **no account-tier dependency** — that, plus the actuator's independent control, is why it is preferred over the money-moving exchange-side path. The trade-off it *does* add (a bot-facing signing RPC + the trade credential in that domain) is part of the §7.3 blast-radius analysis."
- **Exchange-side credential disable (backup):** on **Bybit** the caller "**must hold a transfer/withdrawal-class authority** (`Account Transfer` / `Sub Member Transfer` / `Withdrawal`): a money-moving credential, so grant the **narrowest — `Sub Member Transfer`, never `Withdrawal`** (§7.3)" — the blast-radius policy for that hotter-than-the-trade-key credential stays core §7.4. "**Binance** has **no REST way to disable a *regular* account's own key** (UI + 2FA only); it is API-automatable **only if the bot runs on a *sub-account*** and the actuator holds a **master key** with sub-account management … — **which presupposes the account actually *has* sub-accounts, a tier-gated Binance feature that MUST be confirmed for the real account (§7.2 verify-item); if sub-accounts are unavailable the Binance exchange-side path does not exist and signer isolation is the sole fence** `[→ R1c-3]`". (The elided endpoint list + dated sources: Profiles §4.)
- **Tiering consequence:** "because the primary mechanism (signer isolation) needs no venue credential, a working fence **binds at `$1K` on every venue**; the exchange-side path is a per-venue *backup* used only where it is available *and* verified (a Binance regular/sub-less account has none). The `Fence` MUST define **ack semantics + a propagation test** (prove the bot can no longer write) before it is trusted." (`FenceGateway.VerifyFence`, core §4: "propagation test: prove the bot can no longer write".)

### Invariants

- **Single-writer by transfer `[→ Codex]`:** "single-writer is preserved by *transferring* the writer, never by running two" — the actuator is a second writer and MUST fence before it trades.
- **Epoch semantics (owned here; consumed by §3's branch 2 and its durably-fenced dimension):** the latch is "a **latched, monotonic revoke *epoch***" / "a durable, monotonic takeover epoch" — the same fence-first revoke/starve primitive in both uses — "that the signer **MUST honor across restart until explicit operator clearance**, and `VerifyFence` validates **that epoch** (not a point-in-time snapshot)"; "A **point-in-time `VerifyFence` alone is insufficient** — a transiently-crashed signer could restart mid-takeover and restore **two writers**".
- **Fencing is not cancellation:** "Fencing stops *new* submissions but does **not** cancel orders the venue already holds" — hence the quiesce barrier is mandatory: "Skipping it re-creates the exact hazard: an armed stop fires *after* the reduction and leaves a naked leg."
- **Reduce only the reconciled residual:** never the pre-allocated qty ("which may be stale: an entry leg can still have filled, or a partial reduction already run"); the takeover is a §2-classifier client ("actuator takeover", §2's single-gate enumeration) and reconciles through the §2 contract at its watermark.
- **The fence, not command IDs, prevents double-reduce:** "`Place` isn't a venue idempotency key (§5.3), so the fence — not "idempotent command IDs" — is what prevents a double-reduce."
- **Plan handles, never coarse balances:** the takeover associates orders/fills via the replicated plan's handle set (§6 here); coarse-balance fallback is forbidden (core §7.4: "never on selling from **coarse venue balances** (forbidden — it can hit unrelated inventory)").

### Chaos & acceptance cases (§9 of the core)

- **Actuator takeover fence** (core §9, mapped in full; bracketed pointers mark the sub-cases owned by sibling machines): "network partition / late-fill-after-takeover / bot-recovery-during-takeover / **fence unavailable** (no durable latch → **actuator read-only + operator manual kill at `$1K`+**, never an unfenced trade; de-minimis unfenced action only in the signed canary regime, §12, which claims no single-writer safety)" [floor: §3 here] " / **host-death with an armed stop or `DisarmPending`**" [§5 here] " / **host-death with an outstanding reduction** → single writer, **all resting orders quiesced to finality *before* the reduction**, no double-reduce, reduces only the reconciled residual; **actuator reconcile-only boot enumerates plans via `ListActive` (no `TradeID` list post-host-death) and never re-actions a `Deactivate` tombstone; `Replicate`↔`Deactivate` reorder/retry, crash between close and deactivation, actuator restart → no stale plan re-actioned**" [plan lifecycle: §6 here] ") — assert journal recovery + net-delta invariant + no duplicate/over-reduction + host-independent coverage every time."
- **From the takeover bullet (core §7.4):** "Network-partition, late-fill-after-takeover, partial-fill, and bot-recovery-during-takeover are §9 cases."

### Flagged ambiguities

None.

---

## §5. Protective-order disarm `[→ R1-F2, Z3, R2-#4, R2b-B2, Codex]`

**Source (v0.7.1):** build-spec §7.4 (host-independent-coverage bullet [the arm side] + protective-order-disarm bullet) + §9 — re-expressed, no new behavior. **Normative owner:** the requirement + tag live in core §7.4 (per-venue attach facts: Profiles §1/§3); this section is the mechanism.

**Scope.** Per live leg-level protective order. "Entry-time leg-level protection exists only to bound the first-fill→hedge-complete window." The window exists because "Exposure begins the instant an entry leg fills — a *separate* venue op from any follow-on protective order, so it cannot share the fill's journal transaction" (core §7.4). Protection is armed venue-natively where a leg *verifiably* supports atomic attach (core §7.4/§1.1; per-venue attach facts + the spot-IOC caveat — "Assume the spot IOC leg is unprotected until an integration test proves otherwise" — Profiles §1/§3); the host-death backstop for every leg lacking verified attach is the pre-armed `EmergencyPlan` actuator (§6 here) — "armed **before** entry submission and covering **whichever leg fills first**" — not this machine. "Disarm is a **fenced state machine `Armed → DisarmPending → Disarmed`**, not a fire-and-forget cancel". A protective execution while still `Armed` pre-neutral is the protection doing its job — a fill event classified by §2 like any other; this machine models the disarm half of the lifecycle.

### States

| State | Meaning |
|---|---|
| `Armed` | a live leg-level protective order covers the first-fill→hedge-complete window; it **reserves** the leg allocation (core §7.4) — the §2 sizing counts it ("a protective order's leg reservation", §2's reservation-aware sizing) |
| `DisarmPending` | the disarm is in flight: the cancel is issued but "a stop can trigger *during* cancellation, and §5.3's post-cancel-late-fill rule applies" (§1 here) |
| `Disarmed` | entered only through the §6.7 finality contract (§2 here) — never on a cancel-ack alone; remaining-executable quantity has collapsed; "Ongoing protection is then package-level (the §6 monitor + this ladder), not leg-level" |

### Transitions

| From | Event / condition | To | Action / guard |
|---|---|---|---|
| *(first-fill→ack window)* | the armed protection is confirmed | `Armed` | arm side (core §7.4): the attach is "submitted atomically with the entry, but **acceptance is async — the fill *and* the armed protection are confirmed on the private WS, not the HTTP response**" `[→ R2b-B2]`; "The residual first-fill→ack window is minimized, bounded, and a §9 acceptance case" |
| `Armed` | hot-path `ExposureReduction` on the protected leg | `Armed` (same order, amended) | "A live (non-host-death) protective order **reserves** the leg allocation, so a hot-path `ExposureReduction` **amends / cancel-replaces that same fenced protective order** rather than duplicating it" (core §7.4) |
| `Armed` | neutral-hedge confirmation (both legs filled, package net-delta-neutral) | `DisarmPending` | the package is now neutral, so the armed stop has become "a naked-leg generator" (the full rationale sentence: Invariants); "on neutral-hedge confirmation the OMS issues the cancel" |
| `DisarmPending` | the protective order **partially** executes (incl. a wick-triggered late fill) | `DisarmPending` (stays) | "a protective order that **partially** executes stays in **`DisarmPending`** and is **reclassified after every fill**" (§2 here); "a partial fill with quantity still live is *ambiguous*, never compensated mid-flight" |
| `DisarmPending` | the §6.7 finality contract completes (§2 here) | `Disarmed` | "runs the **§6.7 finality contract** (not a cancel-ack alone) before `Disarmed`": the contract collapses the order's remaining-executable quantity to a fixed residual; "a protective execution is package-compensated via the fenced reduction generation **only once the finality contract collapses its remaining-executable quantity to a fixed residual**", "and the protection's inventory reservation is held until then" |

### Invariants

- **The naked-leg-generator rationale (core §7.4, verbatim):** "**Once both legs are filled and the package is net-delta-neutral, an armed leg-level stop is no longer protection — it is a naked-leg generator** (a benign wick through the spot stop-sell leaves you naked short)."
- **Never a cancel-ack alone:** disarm completes only through the §6.7 finality contract (§2 here) — "protection-disarm's sweep" reconciles "through *this* contract **and its watermark**, never a weaker cancel-ack-only variant" (§2's one-canonical-boundary invariant).
- **Ambiguous mid-flight is never compensated:** a protective execution is package-compensated via the fenced reduction generation only once finality collapses the remaining-executable quantity to a fixed residual — "a partial fill with quantity still live is *ambiguous*, never compensated mid-flight" — and the inventory reservation is held until then.
- **Reservation/amend rule (arm side, core §7.4):** a live protective order reserves the leg allocation; the hot path amends / cancel-replaces that same fenced protective order, never duplicates it (so the §2 classifier does not misread the reservation as naked exposure).
- **Package-level thereafter:** "Ongoing protection is then package-level (the §6 monitor + this ladder), not leg-level."
- **The honest acceptance claim (core §7.4, verbatim):** "Absent an atomic package-level conversion, the acceptance claim is a **measurable bound on detect→re-neutralize**, not "a wick can never create a naked leg".

### Chaos & acceptance cases (§9 of the core)

- **Protective-order disarm race** (core §9): "wick-triggered late fill during `DisarmPending` → re-neutralized within bound"; the source's own case line: "a §9 case (armed → neutral → DisarmPending → wick-triggered late fill → re-neutralized within bound)."
- **Host death between first fill and protective-order ack** (core §9) — the arm-side window: covered by the pre-armed `EmergencyPlan` backstop (§6 here) + the minimized, bounded first-fill→ack window (core §7.4).
- **Protective-order ↔ OMS inventory handoff** (core §9) — exercises the reservation lifecycle above: reserve at arm → amend on the hot path → release only at finality.
- **Host-death with an armed stop or `DisarmPending`** (core §9, inside the actuator-takeover-fence clause — §4 here): the takeover quiesces the armed/disarming protective order to finality **before** any reduction — "an armed stop fires *after* the reduction and leaves a naked leg" is the exact hazard (§4's quiesce invariant).

### Flagged ambiguities

None.

---

## §6. EmergencyPlan lifecycle `[→ Y3, Z3, R1-F2, R2c-3, Codex]`

**Source (v0.7.1):** build-spec §7.4 (actuator-authority bullet: the replicated `EmergencyPlan`) + the §4 `EmergencyPlanStore` port + §6 step 9 (Close) + §7.5 (reconcile-only boot) + §9 — re-expressed, no new behavior. **Normative owner:** the requirement + tag live in core §7.4 / §6 step 9 / §7.5 (the three-failure-domain and watcher→actuator auth requirements stay core §7.4); this section is the mechanism.

**Scope.** Per-`TradeID` plan, across replicate → act → deactivate. The port (core §4): "INTERNAL driven port (NOT a venue call) — replicate the EmergencyPlan off-host BEFORE exposure opens" `[→ R2c-3, Codex]`; "EmergencyPlanStore is internal-only (off-host replication, not a venue call)". Its methods (port comments, verbatim): `Replicate` — "monotonic per-(TradeID,generation) upsert/CAS — a later generation supersedes; ack REQUIRED before the entry submits"; `ListActive` — "actuator reconcile-only boot enumerates ALL active plans — it has NO TradeID list after host death"; `Load` — "latest generation for one trade"; `Deactivate` — "TERMINAL, IDEMPOTENT TOMBSTONE — ack succeeds even if already tombstoned; after ack any Replicate for this TradeID (same OR later gen) is REJECTED (no reactivation); ListActive never returns a tombstoned trade". Why the plan exists (core §7.4): the emergency actuator "MUST act after the bot host *and its local journal* die. Because the allocation cap lives in that journal, the actuator relies on **host-independent coverage** …, never on selling from **coarse venue balances** (forbidden — it can hit unrelated inventory)."

**Plan content (core §7.4, verbatim):** "carrying, for **both legs**, everything the takeover needs *without* the host-local journal: subaccount + `TradeID`, all intended **client/algo order IDs** + the bot-owned order namespace/prefix, sides, allocated qtys, the creation window, **recovery cursors**, and attached-protection parent/child IDs, plus loss/slippage bounds + fencing generation". "Without these handles the actuator can't associate fills once the journal dies" (consumed by §4's reconcile/reduce).

### States

| State | Meaning |
|---|---|
| *(none)* | no plan replicated — exposure MUST NOT open: the plan "is pushed to the actuator **before exposure opens** (actuator **ack required before the entry submits**)" |
| `Active` | replicated and acknowledged; carries the latest **generation** ("monotonic per-(TradeID,generation) upsert/CAS — a later generation supersedes", port); enumerable by `ListActive()` |
| `DeactivationPending` | close-side: a durable journal record that the close reached finality but the `Deactivate` ack is not yet persisted; the hedge-group lease is retained |
| `Deactivated` (tombstone) | terminal: the idempotent `Deactivate` tombstone — rejects any same-or-later-generation replication; "ListActive never returns a tombstoned trade" (port) |

### Transitions

| From | Event / condition | To | Action / guard |
|---|---|---|---|
| *(none)* | entry planned; `Replicate(plan)` acknowledged (`ReplicaAck`) | `Active` | "ack REQUIRED before the entry submits" (port); "pushed to the actuator **before exposure opens**", "with watcher→actuator auth and independent key/IP" (core §7.4) |
| `Active` | a later protection/reduction generation is created | `Active` (generation++) | "later protection/reduction generations are replicated **as they are created** (monotonic generation upsert/CAS)"; a later generation supersedes (port) |
| `Active` | the close reaches finality (core §6 step 9) | `DeactivationPending` | the OMS "**journals a durable `DeactivationPending` before/atomically-with close finality, retains the hedge-group lease while pending, and invokes the idempotent `EmergencyPlanStore.Deactivate` (a terminal tombstone)**" |
| `DeactivationPending` | the `Deactivate`'s **persisted acknowledgement** | `Deactivated` (tombstone) | "only its *persisted acknowledgement* releases the lease or admits a successor trade"; idempotent: "ack succeeds even if already tombstoned" (port) |
| `DeactivationPending` | crash between close and ack, or the store is unreachable | `DeactivationPending` (held; re-driven) | "A crash between close and ack is recovered by **reconcile-only boot re-driving the pending `Deactivate` (idempotent, bounded backoff, alert)**" (core §6 step 9, §7.5); "Holding the lease while the store is unreachable is the correct fail-safe (a symbol waits rather than risk a stale-plan takeover against a successor)" |
| `Deactivated` (tombstone) | a same-or-later-generation `Replicate` arrives (reorder / retry) | `Deactivated` (replication rejected) | "after ack any Replicate for this TradeID (same OR later gen) is REJECTED (no reactivation)" (port); "a flattened trade leaves **no stale, still-actionable plan** even under `Replicate`↔`Deactivate` reordering" (core §7.4) |

### Invariants

- **Replicate-before-exposure, ack-before-entry `[→ R2c-3, Codex]`:** the plan is replicated off-host **before** exposure opens and the actuator ack is **required before the entry submits** — the *(none)* → `Active` guard above; entry-time arming order is a core §6/§7.4 requirement.
- **Actuator boot reads `ListActive`, never a `TradeID` list (core §7.4, verbatim):** "The actuator's **own reconcile-only boot enumerates active plans via `EmergencyPlanStore.ListActive()`** — it has no `TradeID` list once the host is gone — applies the **latest generation** per trade, and treats a **`Deactivate` tombstone** as terminal (rejecting any same-or-later-generation replication)".
- **Close awaits the tombstone (core §7.4):** "the normal **Close awaits that tombstone before releasing the lease** (§6 step 9), so a flattened trade leaves **no stale, still-actionable plan** even under `Replicate`↔`Deactivate` reordering."
- **The lease is never released with a plan still active (core §6 step 9, verbatim):** "the lease is **never** released with a plan still active — so a closed trade leaves no plan an actuator restart could re-action, even under `Replicate`↔`Deactivate` reordering or a mid-close crash". The §7.5 restatement: "A journaled **`DeactivationPending`** (a close that had not confirmed its `EmergencyPlanStore.Deactivate`) is **re-driven to acknowledgement (idempotent, bounded backoff, alert) before its hedge-group lease is released** — a closed trade never leaves an active plan behind".
- **Generations are monotonic:** per-(`TradeID`, generation) upsert/CAS; a later generation supersedes; the actuator always applies the **latest** generation (port + core §7.4).

### Chaos & acceptance cases (§9 of the core)

- **Plan sub-cases of the actuator-takeover-fence clause** (core §9, mapped in full; the clause itself is quoted in §4): "**actuator reconcile-only boot enumerates plans via `ListActive` (no `TradeID` list post-host-death) and never re-actions a `Deactivate` tombstone; `Replicate`↔`Deactivate` reorder/retry, crash between close and deactivation, actuator restart → no stale plan re-actioned**".
- Mapping: reorder/retry → the tombstone-rejection row; crash between close and deactivation → the re-drive row (core §7.5); actuator restart → the boot invariant (latest generation; tombstone terminal, never returned by `ListActive`).

### Flagged ambiguities

- **[FLAG-7]** The deactivation path for a trade flattened **by the actuator** (host dead — the core §6-step-9 close path never runs) is never stated: the "closed trade leaves no plan an actuator restart could re-action" guarantee is defined via the OMS close → `DeactivationPending` → tombstone chain, and §9's "actuator restart → no stale plan re-actioned" sub-cases are that chain's scenarios. Readings: (i) such a plan simply stays `Active` until the recovered, operator-cleared OMS (or the operator) drives close/deactivation — harmless to re-action in the interim because the takeover "reduce[s] the reconciled live residual" (zero once flattened; §4 here); (ii) an unstated actuator-side `Deactivate` after its own flatten. Not resolved here; the tables carry only the OMS-side deactivation path.

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
