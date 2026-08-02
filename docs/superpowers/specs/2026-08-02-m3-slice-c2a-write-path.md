# §51 — M3 slice C2a: the order write path, and the venue that refuses inside a success

**Date:** 2026-08-02 (code) / 2026-08-03 (docs) · **Sessions:** 77–78 · **Docs:** v0.9.32
**Task:** #162 (re-scoped again) · **Opens:** #172 · **Status: COMPLETE — code and docs.**

> **This record was written mid-slice, deliberately**, with §51h as the exact
> remaining list. §51h is now a **closing report** rather than a to-do: it records
> what landed where, and the one item that turned out not to be transcription.

---

## §51a — Why C2 was split again, and what the alternatives were

§47f's slice C was already split by layer into C1 (transport) and C2 (the order
surface). Measured at the start of this session, C2 as scoped is **larger than
C1 was**: `Place` + `OrderByClientID` + `Fills` + the Bybit history fallback,
each across **two venues × two product families**, on top of a venue-facts
verification pass the size of C1's Profiles §6 — because Profiles §1/§3 name the
*endpoints* and record nothing about their parameters, response shapes or cursor
semantics.

Three cuts were put to the user, who asked for a recommendation:

1. **(taken) Split by layer again.** C2a = the write path — `Place` end-to-end,
   the normalized venue DTOs, the real `ports.OrderPlacer` adapter, Profiles §7's
   submission facts. C2b = the read path (#172).
2. **(rejected) Build C2 whole.** The objection is specific rather than general:
   with ~6 codecs to bite-check, what gets cut when a session runs long is the
   *evidence*, not the code, and that is the one trade this repo has repeatedly
   decided against.
3. **(rejected, but its argument was adopted) Fold the queries into slice D.**
   The objection it raises is real — the query methods have **no port and no
   consumer** in `oms` until slice D declares one, which is the
   *schema-field-frozen-before-its-consumer* shape. It was rejected only because
   D is the safety-kernel-adjacent slice (the stable-not-found *rule*), and
   merging it with the venue-side query *client* puts two different correctness
   questions in one slice. **The argument is carried forward on #172**: whether
   C2b keeps its own number or lands folded into D is a decision to take when the
   read path is next, and the ports are declared with their consumer either way.

---

## §51b — FINDING 1: Bybit states a REFUSAL inside a success status, and C1's
## classifier read it as an acceptance

Verified live, 2026-08-02, on public testnet endpoints (no credential involved):

```
GET api-testnet.bybit.com/v5/market/tickers      -> HTTP 200
  {"retCode":10001,"retMsg":"Illegal category","result":{},...}
GET demo-fapi.binance.com/fapi/v1/depth?symbol=  -> HTTP 400
  {"code":-1102,"msg":"Mandatory parameter 'symbol' was not sent, ..."}
```

`ClassifyStatus` — C1's whole classifier — returns `(nil, PenaltyUnspecified)`
for any 2xx. So **on Bybit a venue rejection classified as success.** C1 could
not have found it: it sent signed requests and read *headers*, and never decoded
a response body.

**The worse outcome is one step further on, and it is the one to carry.** A Bybit
refusal body carries no `orderId`, so the ack decode fails — and
`ClassifyAckDecodeFailure` exists to call a failed decode after a success status
**`EffectApplied`**, meaning "the order is live and our record of it is broken".
Left alone, an **explicit venue rejection** would have been recorded as an order
that **definitely exists**. §5.3's two evidence classes — proved absence
(`AbsentAfterFinality`) and explicit rejection (`Rejected`) — would have
collapsed into their opposite.

Profiles §6 named this exact mechanism one layer over, on Bybit's zero-length
401: *"the parse failure is then what classifies the request's effect — which is
the §5.3 seam, reached by a path nobody designed."* **The seam was already
described and the second instance still landed**, which is the transferable
lesson: a hazard written down for one input is not thereby covered for the
others.

**The seam existed and was unwired.** `ClassifyStatus` already took a `code`
parameter, and every caller passed `""`. C1 built the parameter with nothing to
fill it. `EnvelopeReader` is what fills it.

### The shape of the fix

- `EnvelopeReader` per venue (`BinanceEnvelope`, `BybitEnvelope`), the
  `LimitReader` pattern applied to a second asymmetry — *where the venue's answer
  lives* — so no shared switch has to hold both venues' codes without knowing
  which venue it holds.
- `ClassifyStatus` is **unexported** (`classifyStatus`). It is still correct as
  far as it goes; what it does not go far enough for is a caller with a body, and
  on the signed path that is every caller. Leaving it exported would leave the
  wrong answer reachable by the more obvious name.
- `ClassifyResponse` composes the two, **with a direction**: a body may
  CONTRADICT a success status, but it may never SOFTEN what the status
  established. A 5xx that classified `EffectUnknown` keeps that reading even if
  the body's code documents a clean refusal — a venue that could not answer is a
  venue whose account of what it did is the thing in doubt.
- **The general arm matters as much as the Bybit one**: a body reporting an
  error beside a success status, with no venue rule for the code, is
  `EffectUnknown` (`ErrEnvelopeContradictsStatus`). Without it the function would
  have had a hole of exactly the shape it was written to close — one venue's
  asymmetry fixed, the general case still reading a self-contradicting response
  as an acceptance. It is reachable and tested on **Binance**.
- Bybit's code table is **default-deny**: 10001/10002 (`NotApplied`, the venue
  parsed and refused), 10006/10018 (rate limit, `Throttle`/`Ban`), 10016
  (`Unknown`, Bybit's analogue of the 5xx Binance documents), **everything else
  `EffectUnknown`**. Bybit documents nothing anywhere about whether a failed
  request may have executed, so an ordinary "insufficient balance" costs a
  resolving query and a trading hold. That is the right way round and the cost is
  the point: the other default is silent when it is wrong.
- `BinanceEnvelope` reads only a **negative** `code`, because non-negative `code`
  fields appear on Binance responses that mean success.

---

## §51c — FINDING 2: §7.6's shed classes have NO CARRIER, anywhere

The venue call needs `ratelimit.Priority` for `Governor.Acquire`. The class of an
order is a property of the §5.3 **command** that produced it — `OpenTrade`
creates exposure, `ExposureReduction`/`EmergencyClosePackage` remove it — and
**neither `domain.Order` nor `ports.OrderRecord` holds which command that was.**
The adapter is the layer that must state it and the only layer with no way to
know it.

**Defaulting is not available, and the two defaults are not equally wrong.**
`Opening` puts the order under §7.6's ~70% back-off ceiling, so a **flatten**
defaulted to `Opening` is shed during exactly the incident the reserve exists
for. `Safety` puts it at capacity, so ordinary trading spends the reserve and
needs a second coincidence before it hurts. A silent default would have been the
first kind.

**Taken:** `ports.OrderPlacer.Place(domain.Order, ports.Urgency)`, and
`Dispatcher.Dispatch(id, version, urgency)` — checked **above** the
`AppendState` fsync, because that fsync is the commitment to call `Place` exactly
once and refusing after it would leave a durable authorization for an order
nothing ever sent. Marked `[SPEC EDIT OWED]` in both files; **§51h owes the
landing.** It was cheap *now* because `Dispatch` has no production caller yet —
§6's orchestrator is a later slice — and would not be later.

**`ports.Urgency` has TWO members, not three.** §7.6's `Routine` is read traffic,
which no order is under any command. Declaring it would put a value in the type
that no caller can correctly choose and that maps to **neither ceiling** —
`Routine` sits between them, so an adapter translating it must round, and both
roundings are the silent version of the decision the type exists to make
explicit. `venue.OrderRequest` refuses `PriorityRoutine` for the same reason.

**Rejected, and it is the better long-term shape:** a field on `domain.Order`
naming its originating command. The same fact also decides §5.3's `stop-opening`
exemption, which likewise has no carrier — so one missing carrier serves two
rules. It is a §5.1 aggregate change reaching the journal codec, the planner and
recovery, and dragging it into C2a would have been the scope creep the split
existed to avoid. **Recorded so a later slice re-opens it deliberately.**

---

## §51d — FINDING 3: a documented venue ZERO met a zero-is-unusable default

`ratelimit.Cost.Validate` refused `Weight == 0`, on the reasoning that *"every
metered endpoint on both venues weighs at least one, so a zero is a field nobody
filled in"*. **That premise is false, and it is false about the most important
endpoint in the system.** Binance documents `POST /fapi/v1/order` verbatim as
*"1 on 10s order rate limit(X-MBX-ORDER-COUNT-10S); 1 on 1min order rate
limit(X-MBX-ORDER-COUNT-1M); **0 on IP rate limit(x-mbx-used-weight-1m)**"* —
while spot's is *"Weight: 1"*. So the truthful `Cost` for placing a futures order
was the one `Validate` refused.

The argument survived; only its subject was wrong. The check is now **"charges no
dimension a caller fills in"** (`Weight == 0 && Orders == 0`), and
`DimensionRequests` is deliberately not counted as charged: every `Acquire`
charges one request by construction, so counting it would make the check vacuous
on the one budget with no header to correct it afterwards.

**The transferable shape:** this repo's "the zero value is unusable" rule assumes
zero is never a legitimate value. Where the *venue* publishes a zero, the rule
inverts — and the collision is invisible because both readings are "a zero in a
required field". The tell was reading the weight line verbatim instead of
assuming it.

---

## §51e — What the layering decisions were

- **`engine/venue` does not import `oms/domain`.** §4 gives the seam "normalized
  DTOs" and gives each context a "thin ACL adapter". **No gate polices this
  edge** — `hexboundary` watches `domain/`/`application/` importing `engine/`,
  not the reverse — so it is recorded at the place it would be broken
  (`order.go`'s header) rather than left to a linter that has no opinion.
- **`venue.OrderRequest` has no `Type` and no `TimeInForce`.** §6 step 6's
  "marketable-IOC limit, never raw MARKET" is enforced **by the type**: a market
  order is not expressible past the ACL. The argument for the narrowing is
  recorded because a narrowing carries one — it is **not** "we only need IOC
  today", it is that an `OrderType` whose only legal value is `Limit`, or a
  `TimeInForce` whose only emitted value is `IOC`, **declares enum members
  nothing emits**, which satisfies the compiler, `exhaustiveswitch` and switch
  coverage while leaving the member unreachable. The narrowed-away case is a
  RESTING order, and it arrives with its consumer or not at all.
- **`Deps.Placer` became `Deps.VenueOrders`.** The old field asked a root for
  something implementing `Place(domain.Order)`, satisfiable only by (a)
  `engine/venue` importing `oms/domain` — inverting §4's layering — or (b) the
  root writing this context's ACL adapter, which puts the context's translation
  in a composition root where a second binary rewrites it. `module.go`'s own doc
  had called this field *"permanent"*; **the premise was right and the conclusion
  did not follow** — a venue *client* does live outside, but the field did not
  ask for a venue client. Same forcing function as slice B's `JournalPath`.
- **The context passed to `Place` is deliberately NOT cancellable.** Cancelling a
  `Place` in flight does not stop the order — it stops us waiting for the answer,
  converting an outcome the venue was about to give into `EffectUnknown`. An
  operator's Ctrl-C would manufacture exactly the ambiguity the crash boundary
  exists to minimise. `SignedClientConfig.Timeout` is the bound, and its own doc
  already says it is "a safety parameter, not a responsiveness one".
- **Routing keys on `VenueInstrument`, and on Binance that selects a HOST.**
  `api.binance.com` vs `fapi.binance.com` — so §4's rule has one layer beneath
  the usual "independent order-ID namespaces" reason. An unconfigured family is
  **refused, never defaulted**: "BTCUSDT" is a real, executable instrument on
  both hosts, so a mis-route is an accepted order on the wrong product — a
  leveraged short where a cash purchase was intended. There is no status code for
  that.
- **Binance's two clients MUST share one `*ratelimit.Governor`** — §7.6 keeps
  per-family budgets under a shared IP-ban envelope per venue, and two governors
  would be two envelopes for one IP, each seeing half the traffic.

---

## §51f — Two venue facts that would each have shipped silently

- **`newOrderRespType` differs in enum AND default between the families.**
  Futures is `{ACK, RESULT}` defaulting to `ACK`; **spot is `{ACK, RESULT, FULL}`
  where "MARKET and LIMIT order types default to FULL"**. So the family that
  defaults to the largest response is the one whose response we least want: a
  spot LIMIT's default response carries a **`fills` array**, handing this system
  executions on the submission path at the moment §4 says the private stream is
  the single authority and §5.3 has no transaction open. They would have been
  dropped — correct, and the fragile kind of correct, one edit from a reader who
  thinks applying them early is an optimization. **`ACK` is now stated
  explicitly**, which converts a discipline into an absence.
- **`isLeverage=0` and `omitempty`.** Profiles §2 makes `isLeverage=0` normative
  on every Bybit spot order — it is what keeps the spot leg an unborrowed cash
  purchase, the premise of the whole "spot cannot be seized to cover the perp
  loss" property. **The value that must be sent is zero, and `omitempty` drops
  exactly zero.** The obvious struct tag silently produces a margin-enabled spot
  buy. Encoded through a pointer; the test asserts the field is **present**, not
  that it decodes to 0 — a dropped field also decodes to 0.

**And one assumption CLOSED.** `ids.ClientOrderID` has carried since it was
written: *"the 36-character budget and the `[A-Za-z0-9_-]` character set this
format fits inside are NOT recorded in venue-profiles.md … Confirm before
milestone 3 places a real order."* Both rules are now verified — Binance's
`^[\.A-Z\:/a-z0-9_-]{1,36}$` and Bybit's *"max of 36 characters … numbers,
letters (upper and lower cases), dashes, and underscores"* — and the minted
format is 35 characters of `[a-zA-Z0-9-]`, inside both, **with one character of
headroom**. The test asserts the length so the margin is on the record.

The two rules are **not the same**, and that is what made the discriminating
fixture possible: `.` is inside Binance's class and outside Bybit's. A test using
only ids both venues accept could not tell a per-venue check from one shared
check written to whichever rule the author remembered.

---

## §51g — Evidence

**20 mutations, 20 kills.** `make ci` green both modules (all six analyzers).

**One SURVIVOR, and it was a real gap of the checklist's sibling shape.**
`binanceSide(OrderSideBuy)` was changed to return `"SELL"` and every test still
passed. The fixtures *were* varied — the perp request sells, the spot request
buys — and **no Binance assertion ever read the buy fixture's side**. The Bybit
mirror was already covered, because its body assertion is a byte-for-byte
comparison that necessarily includes the side. **Two codecs, the same code shape,
asymmetric evidence** — slice C1's finding arriving one layer down, in the tests
instead of in the venues. `TestBinanceRendersBothSides` closes it.

**Two NO-OPs, both re-aimed and caught.** Both were `if <cond> {` patterns whose
line number pointed at the comment block above the `return`; aiming at the
`return` itself applied.

Collapse-checklist items applied while writing rather than after: `recvWindow` is
3s (not the venues' 5000 ms default); quantity `0.5` against price `50100.25`, so
a transposition shows; the minted id is leg 1 / attempt 3, neither field zero;
the perp fixture SELLS; `reduceOnly` has both a true and a false fixture because
false makes the parameter *absent*; the id-length fixtures are 36 and **37**, not
36 and 100, because one-past is where `<` and `<=` differ; and the host-routing
test uses **two** httptest servers, since one server could only check the path
and would pass against a client that sent everything to whichever host it had.

---

## §51h — CLOSING REPORT (was: what remains)

**Completed 2026-08-03, session 78.** Every item below landed. `make ci` green
both modules; **marker grep `internal/ tools/` returns ZERO**; docs are v0.9.32
across all seven.

1. **Both `[SPEC EDIT OWED]` markers converted to `[SPEC EDIT LANDED — v0.9.32]`.**
   `ports/gateway.go` → core **§4** (the `Place(Order, Urgency)` signature and
   the paragraph on why nothing it could read carries the class) + **§5.3** (the
   command→class table) + **§7.6** (the carrier paragraph).
   `application/dispatch.go` → **§6 step 6**, which now fixes *where* an invalid
   class is refused: above the fsync, because refusing after it leaves a durable
   authorization for an order nothing ever sent.
2. **`venue-profiles.md` §7 written** — the shape-asymmetry table, both
   parameter tables, `newOrderRespType`, both id rules with Binance SPOT's
   stated assumption, the per-endpoint costs incl. the futures zero, the
   HTTP-200-with-`retCode` transcripts, Bybit's default-deny code table, and the
   testnet hosts. **The three hosts were re-probed 2026-08-03** rather than
   transcribed; all three still answer 200, and the table carries both dates.
3. **Core spec:** §4 (signature + carrier + host-per-family), §5.3 (the table),
   §6 step 6, §7.6 (the carrier), §12's slice-C2a annotation. **Plus one section
   §51h did not anticipate — see item 7.**
4. **`adr-history.md` §3** dated block landed, newest-first above §50's.
5. **`HANDOFF.md`** — the ⚠ IN FLIGHT block is replaced by the closing status.
6. **v0.9.32 across all seven docs**, Georgian regenerated from
   `docs/i18n/ka/tutorial/header.md` via `assemble.mjs` (127,674 bytes written).
7. **THE READING PASS WAS NOT TRANSCRIPTION, AND IT FOUND ONE — task #173.**

### §51h-1 — What the reading pass found, and why the marker grep could not

**A `Cost` charging a dimension the profile declares no `Limit` for is silently
uncounted.** `Governor.decide` walks the **profile's** declared windows and takes
each one's dimension from the cost, so a dimension with no window is never
visited: no error, no refusal, no metric. A governor that counts nothing admits
everything.

That was harmless while every metered endpoint charged weight. **§51d is what
made it reachable.** Correcting `Cost.Validate` to accept Binance's documented
`POST /fapi/v1/order` weight of **zero** was right — and it made that endpoint
**the one call in this system whose entire cost rides on a single dimension**. A
Binance profile declaring weight budgets but no order-count budget therefore
charges *literally nothing* per futures order, admitted forever, against a venue
that meters `X-MBX-ORDER-COUNT-10S`/`-1M` and bans on them.

**Nothing is broken today, and the reason matters:** no production
`ratelimit.Profile` is constructed **anywhere** — the only app roots are
`capture` and `sim`, neither of which trades. So this is the *consumer is absent*
category, the same as C1's §7.6 alert finding (#170), and it is recorded with its
**trigger named as the construct rather than the feature**, per §3.6's deferral
rule: **the first production `ratelimit.Profile`**. §7.6 states the requirement
normatively from this release; #173 builds the refusal.

**The shape is why this is worth carrying.** The §51d fix was correct and it
**moved the safety of the zero onto a claim about a different object** — that the
profile counts orders — which nothing anywhere checks. It is the `FM-5` family at
the configuration layer: *a validator cannot audit the thing it is validated
against*, and the tell is that fixing a validator did not change what the
validator's output would be checked against. **A correction that relocates a
premise is exactly as much of a finding as a defect**, and the marker grep is
blind to it for the standing reason — the clause is *absent* rather than
departed from.

**Standing rituals unchanged.** Capture is PID **8495**, alive throughout both
sessions; never rebuild over `bin/arblab` while it runs. `make ci` runs `-race`
and `go test` does not.
