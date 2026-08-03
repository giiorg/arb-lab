# §52 — M3 slice C2b: the order read path, and the cursor that cannot answer §6.7

**Date:** 2026-08-03 · **Session:** 79 · **Docs:** v0.9.33 (owed)
**Task:** #172 · **Status: IN PROGRESS — record written mid-slice, per §51's practice.**

> §51 was written mid-slice deliberately and its §51h became the closing report.
> Same shape here: **§52h is the remaining list**, and it closes as a report.

---

## §52a — The fork §51a left open is TAKEN: C2b keeps its own number

§51a recorded three cuts and adopted the argument of the one it rejected: the
query methods have **no consumer in `oms`** until slice D declares one — the
*schema-field-frozen-before-its-consumer* shape — so "whether C2b keeps its own
number or lands folded into D is a decision to take when the read path is next".

It is next. **C2b keeps its number**, on §51a's own ground rather than on a new
one: folding C2b into D produces a slice *larger* than "build C2 whole", which
§51a rejected because **what a long session cuts is the evidence, not the code**.
An argument that rejects the smaller merge cannot admit the bigger one.

**What the fold's objection buys instead, and it is not a consolation prize.**
The objection is answered by *where the slice stops* and by *what the DTOs are
derived from*:

1. **C2b adds NOTHING to `oms/application/ports`.** The read surface lands in
   `engine/venue` only. Slice D declares `ports.OrderResolver` **with** its
   consumer and writes the ACL adapter, exactly as C2a's `ports.OrderPlacer`
   landed beside `dispatch.go`. So no OMS port is frozen without a caller.
2. **The venue DTOs are derived from state-machines §2's four-step finality
   contract, not from the endpoints' shapes.** They are frozen without a Go
   caller — unavoidable, that is what the slice *is* — and the mitigation is that
   their shape is answerable to a clause that is already normative.

That second point is not a formality. **It found the slice's headline defect
before a line was written**, below.

---

## §52b — FINDING 1: §4's `Cursor` cannot be either venue's cursor, and the
## obvious choice satisfies §4 completely while making §6.7 inexpressible

§4 declares `Fills(VenueInstrument, since Cursor) ([]Fill, Cursor, error)` and
says nothing about what a `Cursor` is. There is **no `Cursor` type anywhere in
`internal/`** — C2b mints it. Three surfaces have to fit inside it, and they
offer three different things (all verified 2026-08-03, Profiles §8):

| Surface | Resume handle | Direction | Comparable to a time? |
|---|---|---|---|
| Binance USDⓈ-M `/fapi/v1/userTrades` | `fromId` (trade id) | ascending | **no** |
| Binance spot `/api/v3/myTrades` | `fromId`, *"trades >= that `fromId`"* | ascending | **no** |
| Bybit `/v5/execution/list` | `nextPageCursor` (opaque) | **descending by `execTime`** | **no** |

State-machines §2's finality contract, step 3, requires sweeping executions
*"through a **venue/server watermark strictly later than** the order's
cancellation-effectiveness / max landing horizon — never a synthetic
stable-not-found alone"*. That is a **comparison against a time horizon**. None
of the three handles above can be compared to one. A `Cursor` defined as "the
venue's own resume handle" — the obvious and idiomatic choice, and the one each
venue's documentation invites — implements §4's signature **completely and
correctly on all three surfaces** and makes §6.7 step 3 **inexpressible**.

**So `Cursor` carries a venue SERVER TIME plus a venue-local discriminator**, and
the venue's own handle is demoted to what it actually is: a paging detail *within*
one sweep, not the thing that persists between sweeps.

**The discriminator is not defensive padding — Bybit says it is needed.** The
execution list is *"sorted by `execTime` in descending order"* and, when
`execTime` matches, *"it is recommended to sort according to
`execId+OrderId+leavesQty`"* — i.e. **`execTime` is not unique**. A time-only
watermark therefore either drops the tied remainder or re-reads it.

**And the tie is resolved TOWARD re-reading, on §5.1's own ground.** Fills dedupe
on `UNIQUE(account, venue, product_category, symbol, execution_id)`, so a
re-delivered fill costs one wasted row-check and a gap costs a missing execution
in the ledger the net-delta invariant is computed from. Duplicates are free here
and gaps are not, so the watermark **rewinds to the start of the tied instant**
rather than advancing past it. This is the two-directions rule applied to a
cursor: it is right when somebody forgets. Binance spot's `fromId` semantics —
*"If `fromId` is set, it will get trades >= that `fromId`"*, **inclusive** — are
the same choice made by the venue, which is corroboration rather than coincidence.

### §52b-1 — FINDING 1b: Bybit's "next page" walks AWAY from `since`

`since Cursor` is forward in time. Bybit's list is **descending** and
`nextPageCursor` advances **into older data**. The two are spelled the same way
(a string handed back to the next call) and they point in opposite directions.
An implementation that stores the last `nextPageCursor` as the next `since`
resumes at **the oldest record of the previous sweep**, re-reads the same window
forever, and never advances — while every request succeeds, every page decodes,
and the ledger fills with duplicates that dedup silently absorbs. **Nothing
fails.** The tell is that a cursor's *direction* is not in its type.

### §52b-2 — FINDING 1c: the window cap is 24 HOURS on exactly one of the three

| Surface | Max `startTime`→`endTime` span | If omitted |
|---|---|---|
| Binance USDⓈ-M `userTrades` | *"cannot be longer than 7 days"* | *"the last 7 days' data"* |
| **Binance spot `myTrades`** | ***"can't be longer than 24 hours"*** | *"the most recent trades"* |
| Bybit `execution/list` | *"endTime - startTime <= 7 days"* | *"return 7 days by default"* |

A sweeper written once against "the 7-day window" is wrong on Binance **spot** by
a factor of seven — the *same venue*, the other family, the parameter spelled
identically. This is §51f's `newOrderRespType` finding repeating on a different
parameter: **the two Binance families differ, and the differing value is the one a
reader would never re-check.**

**`fromId` and the time window are MUTUALLY EXCLUSIVE on both Binance families,
and the two documents say so differently** — futures: *"The parameter `fromId`
cannot be sent with `startTime` or `endTime`"*; spot: an explicit whitelist of
supported combinations (`symbol`, `symbol`+`orderId`, `symbol`+`startTime`,
`symbol`+`endTime`, `symbol`+`fromId`, `symbol`+`startTime`+`endTime`,
`symbol`+`orderId`+`fromId`) in which no `fromId`+time combination appears. So
**a sweep bounded by a §6.7 watermark cannot also resume by id**, on either
Binance family. The watermark wins; the id is not the cursor. That is the same
conclusion as §52b, reached a second way from the venues rather than from §6.7.

---

## §52c — FINDING 2: not-found is a RESULT of the read path, not an error of it —
## and the two venues put it in opposite layers, inverted from C2a's finding

| | Binance (both families) | Bybit |
|---|---|---|
| Order absent | error **`-2013 NO_SUCH_ORDER`**, *"Order does not exist."* | (assumed) `retCode` 0 with an **empty `list`** |
| Layer | the **envelope** — C2a's `ClassifyResponse` already turns it into an `*apperror.Error` | the **payload** — the envelope reader sees success and passes it through |

C2a found Bybit stating a *refusal* inside a success status. This is the same
seam **inverted**: here **Binance** states a *semantic answer* inside a transport
*error*, and Bybit states it in a body the classifier is right to call successful.

**Why that matters more than the symmetry.** `-2013` on a 4xx classifies
`EffectNotApplied` — a true statement about the *query* (it was refused, nothing
happened) and a category error about the *order*. The fact the caller needs is
"the venue answered, and its answer is: no such order", which is the **input** to
§5.3's stable-not-found rule. Propagated as an error it is indistinguishable, at
the call site, from a query that was rate-limited or timed out — and §5.3 forbids
exactly that conflation in so many words: *"'not submitted' is never inferred
from a transient not-found"*, and *"if the resolving query is
unavailable/rate-limited, new trading stays disabled"*.

**So `OrderByClientID` returns a presence-bearing VALUE, and reserves `error` for
"the venue did not answer".** An absence and a failure are different returns, not
one return with a sentinel inside it. The rule that a caller cannot get wrong is
the one where the wrong thing is not expressible — §51e's argument for
`OrderRequest` having no `Type`, applied to a result instead of a request.

---

## §52d — FINDING 3: the stable-not-found rule has an EXPIRY DATE, and no
## document records it — and each venue's cliff lands on this system's own
## most common terminal state

The order surfaces do not retain orders forever, and neither venue's cliff is
mentioned anywhere in the doc set (verified 2026-08-03, Profiles §8):

| Surface | Drops an order when |
|---|---|
| Binance USDⓈ-M `GET /fapi/v1/order` | *"order status is `CANCELED` or `EXPIRED` **AND** order has NO filled trade **AND** created time + 3 days < current time"*; and unconditionally at *"order create time + 90 days"* |
| Binance spot `GET /api/v3/order` | **no retention published** — recorded as an unverified difference, not as "no cliff" |
| Bybit `GET /v5/order/realtime` | *"the most recent 500 orders"*; *"After a server release or restart, filled, cancelled, and rejected orders of Unified account should only be queried through order history"* |
| Bybit `GET /v5/order/history` | *"last 7 days"* for closed statuses; ***"last 24 hours"* for `Cancelled`, `Rejected`, `Deactivated`**; beyond 7 days *"supports querying orders which have fills only"* |

**Now compose that with §6 step 6.** This system sends *"parallel marketable-IOC
limit orders (never raw MARKET)"* and nothing else. A marketable IOC that does not
fill is terminal immediately, with **no filled trade** — which is precisely the
row Binance USDⓈ-M drops after **three days**, and precisely the row Bybit's
history drops after **twenty-four hours**. **The single most common outcome of
this system's own orders is the one that becomes unqueryable soonest, on both
venues.**

Past that cliff `OrderByClientID` cannot distinguish *"the order existed and
expired unfilled"* from *"the order never existed"* — and §5.3 maps a stable
not-found to **`AbsentAfterFinality`**, whose whole meaning is **proved** stable
absence. Past the cliff the query proves nothing, so the terminal state would be
asserting evidence it does not have.

**The exposure reading is unchanged and that is exactly why this is easy to
dismiss** — an unfilled IOC and a never-existent order both contribute zero
delta, so nothing is mis-hedged and no invariant fires. What is wrong is the
**evidence class**, which is the thing §5.3 spends a paragraph keeping distinct
from `Rejected`, and which §7.5's reconcile-only boot consumes as truth after
downtime that can trivially exceed three days.

**The spec's own rule is already right, and what is new is that it is not always
SATISFIABLE.** §6.7 step 3 says absence is never proved by *"a synthetic
stable-not-found alone"* — the executions sweep past a watermark is the other
half, and a filled order survives 90 days on Binance orders and 6 months on
Binance trades. So the composition holds *inside* the retention envelope and has
no meaning outside it. **Slice D must bound `AbsentAfterFinality` by the venue's
published retention and refuse to claim proof beyond it** — recorded as the
construct that triggers it, per §3.6's deferral rule: **the stable-not-found rule
itself** (#163). C2b's job is to make the bound *representable* — the read path
reports which surface answered and how far back that surface can see — not to
apply it.

**The transferable shape** is `[[documented-zero-meets-zero-is-unusable]]`'s
family: a **venue-published bound** colliding with a repo rule written as though
the quantity were unbounded. There the bound was a zero the venue legitimately
published; here it is a horizon the venue legitimately publishes. Both times the
repo's rule is right about the world it was written for and silent about the one
the venue actually offers.

---

## §52d-1 — FINDING 4: on Binance a swept fill carries NO client order id, and a
## shipped comment says that cannot matter

Response fields, verbatim from each document:

| Surface | Client-order-id field |
|---|---|
| Binance USDⓈ-M `/fapi/v1/userTrades` | **none** — `{buyer, commission, commissionAsset, id, maker, orderId, price, qty, quoteQty, realizedPnl, side, positionSide, symbol, time}` |
| Binance spot `/api/v3/myTrades` | **none** — `{symbol, id, orderId, orderListId, price, qty, quoteQty, commission, commissionAsset, time, isBuyer, isMaker, isBestMatch}` |
| Bybit `/v5/execution/list` | **`orderLinkId`**, on every row |

So on Binance the venue's own `orderId` is the **only** join key between a swept
execution and one of our orders. C2a's `ports.Ack.VenueOrderID` says the opposite
in shipped code: *"§5.3's recovery path keys on ClientOrderID (Bybit resolves by
orderLinkId, and its order-history fallback is what makes a filled order always
recoverable), so **nothing in this system REQUIRES the venue's id**."*

**The parenthetical justifying the claim names the one venue where it is true.**
That is C2a's own finding 1 — *a hazard written down for one input is not covered
for the others* — arriving in the **reassuring** direction, which is worse,
because a hazard note at least announces that something needs checking.

**The composition still closes, and only by a second venue fact.** Binance
retains an order **with fills** for 90 days, so `OrderByClientID` can always
recover the `orderId` for any fill worth attributing. But that makes `Fills` and
`OrderByClientID` a **composition** where §4's signatures suggest independence —
and it makes the sweep useless alone in exactly the scenario it exists for: a
lost ack, which is precisely when §6.7 runs and precisely when we have no
`orderId`. Marked `[SPEC EDIT OWED]` on `ports/gateway.go`; §4 and §5.3 owe the
sentence.

---

## §52e — What the layering decisions are

- **No `oms/application/ports` change in this slice** — §52a item 1. The whole
  point of keeping the number was to stop *there*.
- **`engine/venue` still does not import `oms/domain`** (§51e). The read path
  needs an execution DTO, so `venue.Execution` is minted here rather than
  reaching for `domain.Fill`, for §4's stated reason and for the practical one:
  `domain.Fill` carries `Account`, which is an OMS concept the venue layer has no
  way to populate.
- **The Bybit realtime→history fallback is in the CODEC, not in the client.**
  §5.3 makes it normative (*"resolution MUST fall back to `GET
  /v5/order/history`"*), it is a fact about one venue's surfaces, and the client
  is the layer that knows about transports and routing. A fallback in the client
  would be a Bybit rule living where Binance can see it.
- **The two-surface fallback reports WHICH surface answered.** Not for
  observability — §52d needs it: the two surfaces have different retention, so
  "absent" from realtime and "absent" from history are different claims, and only
  the composed one is admissible input to the stable-not-found rule.

---

## §52f — Collapse-checklist items identified BEFORE writing tests

Per the standing rule (run it before the mutations, not after). The parameters
that would collapse a distinction this slice makes:

- **The two Binance families' window caps (7 d vs 24 h)** — any fixture using a
  span under 24 h makes both correct. Vary to a span *between* them.
- **`execTime` ties on Bybit** — a fixture whose executions all have distinct
  `execTime` makes "rewind to the tied instant" and "advance past the newest"
  the same cursor. At least two executions must share one `execTime`.
- **Bybit's descending order** — a single-page fixture makes direction
  unobservable; a two-page fixture with the newest first is what distinguishes
  forward-resume from backward-paging. `[[fixture-count-of-one-collapses]]`.
- **Not-found vs empty sweep** — a fixture where the order is absent *and* has no
  executions cannot tell "absent" from "found with zero fills".
- **The retention cliff** — a fixture at 2 days and one at 4 days, not one at
  1 hour and one at a year; the boundary is where `<` and `<=` differ.
- **Presence vs error** — an assertion that only checks `err != nil` cannot see
  §52c's whole point. Assert the presence value.

---

## §52g — Evidence (code half)

**26 mutations, 26 kills** after fixes. `make ci` green both modules (all six
analyzers, `-race`). Marker grep returns **two**, both deliberate and both owed
to the docs half (§52h).

**FIVE survivors, and four were real gaps.** The interesting part is what kind:

| Survivor | Diagnosis |
|---|---|
| `bybitExecutionSide("Buy")` → Sell | **C2a's §51g survivor, one slice later on the mirror path.** Every execution fixture SOLD, because the flagship's perp leg sells. C2a fixed the identical shape on the WRITE path and the mirror it checked was the other *venue*; the mirror it missed is the other *direction of travel* — and the spot leg of every cash-and-carry trade is a BUY, so the unreached arm is half of all real fills. |
| `Untriggered`/`Triggered` → terminal | The codec's comment **argues** for mapping them non-terminal (safe if the impossible happens); nothing checked the direction. Prose stating a claim is not a check of it. |
| an execution **before** `Since` | The two-sided-bound trap: the fixture went past the horizon and never before the resume point, so one of the two predicates was unexercised. |
| a row stating **both** direction spellings | A defensive arm with no fixture. |
| `AbsenceFromError`'s effect check | **An equivalent mutant, and diagnosing it was the work** — see below. |

### §52g-1 — The equivalent mutant, and why the guard stayed

`AbsenceFromError` requires `Code() == "-2013" && Effect() == EffectNotApplied`.
The effect half survived, and it survived because **every `apperror` constructor
that sets a `Code` also pins `Effect` to `EffectNotApplied`** — `RateLimited`,
`Unauthorized`, `VenueRejected` — while the one constructor taking an arbitrary
effect, `Transport`, sets no code at all. So the combination is unreachable
through the classifier, and my "right code, wrong effect" fixture **never had the
code on it**: I varied the effect and the assertion could never see it, because
the first conjunct already failed.

The guard is **kept**, and the test now reaches it through `apperror.New`
directly. The reason is that its necessity rests on an invariant of a **different
package that nothing states**: a constructor for "a 5xx that carried a venue
code" is an entirely natural thing to add, and it would make this branch live the
day it lands. Reaching it in a test converts *"unreachable, so harmless"* into a
claim that fails out loud if that stops being true — which is
`[[a-claim-that-cannot-be-mutated-wants-a-name]]` applied to a guard rather than
to a `NOBUILD`.

**Two `NO-OP!!`s** were mis-aimed line numbers, re-aimed and caught. No
`TIMEOUT!!`, no `PANIC!!`.

---

## §52h — What remains (the DOCS half, owed)

The code half is committed on its own, deliberately, exactly as C2a's session 77
did (`e152141`, *"WIP, docs owed"*): the marker grep is the gate that keeps that
honest, and it currently returns the two markers below rather than zero.

1. **`venue-profiles.md` §8** — the read surface: the three window caps
   (24 h / 7 d / 7 d), the retention cliffs, the two not-found layers, the
   per-family weights, the execution-row field lists, Bybit's descending sort and
   non-unique `execTime`, and the ONE unverified assumption (Bybit's empty-list
   absence, which needs #171's credential).
2. **Core §4** — the two read signatures' widenings (`[SPEC EDIT OWED]` in
   `orderquery.go`), the `Cursor`-is-a-server-instant paragraph, and finding 4's
   correction to the `Ack.VenueOrderID` claim (`[SPEC EDIT OWED]` in
   `ports/gateway.go`).
3. **Core §5.3** — the retention bound on `AbsentAfterFinality` (§52d), stated
   normatively; #163 builds it.
4. §12's slice annotation; ADR §3 dated block; HANDOFF; **v0.9.33 across all
   seven**; Georgian regenerated via `assemble.mjs`.
5. A reading pass — **not transcription**, per §51h-1.

**Standing rituals.** Capture is PID **8495**; never rebuild over `bin/arblab`
while it runs. `MUTATE_MODULE=tools/lint` for the analyzers.

**Standing rituals.** Capture is PID **8495**; never rebuild over `bin/arblab`
while it runs. `MUTATE_MODULE=tools/lint` for the analyzers.
