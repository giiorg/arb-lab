# M3 slice D — recovery: the venue-specific stable-not-found rule

**Task:** #163 (§47f slice D)
**Date:** 2026-08-03 (session 81)
**Reads:** core §5.3 (the order lifecycle + the v0.9.33 retention bound), core §6
step 6/7/10, SM §1 (the edge set), SM §2 (the four-step finality contract),
`docs/formal/ambiguous-order-model.md` §3, Profiles §8 (the read surface),
`engine/venue`'s `orderquery.go` (slice C2b).

---

## §53a — what the rule IS, derived from the two normative statements

§5.3 names it twice and never states its steps: *"finality follows a
**venue-specific stable-not-found rule**"*, and *"an order at `DispatchAuthorized`
or beyond is resolved **only** by `OrderByClientID`/`Fills` … never resubmitted,
and 'not sent' is never inferred"*. SM §2 supplies the mechanism — the four-step
finality contract — and core §5.3's v0.9.33 paragraph supplies the bound: the
rule *"MUST take the answering surface's published reach as an input and refuse
to claim proof beyond it"*.

Composed, the executable rule has **four gates**, and `AbsentAfterFinality`
requires all four. Each is separately falsifiable, which is why they are four
gates and not one predicate:

| | Gate | Source | What it refuses |
|---|---|---|---|
| 1 | **Landing horizon passed** | SM §2 step 1 | An absence observed while our own send may still land. *"A point-in-time not-found cannot collapse an in-flight order — a `DispatchAuthorized` send may land AFTER the query."* |
| 2 | **Absent from every surface** | §5.3's Bybit clause | An absence from `GET /v5/order/realtime` alone, which holds ~500 closed orders and clears on a server restart. C2b's `FallbackSurface` loop already runs this inside `engine/venue`. |
| 3 | **A conclusive sweep past the watermark** | §6.7 step 3 | A *"synthetic stable-not-found alone"*. The sweep must PROVE coverage through an instant strictly later than gate 1's horizon, and must find no execution this system cannot rule out for this order. |
| 4 | **Within the answering surface's reach** | §5.3 (v0.9.33) | A not-found from a surface that has purged the row. Past the cliff the query cannot distinguish *"existed and expired unfilled"* from *"never existed"*. |

Gates 1 and 4 are both **time bounds measured from the same instant** — the
moment this system committed to the single `Place`. That instant is where this
slice's finding lives.

## §53b — THE FINDING: the rule's anchor instant has no home, and slice A rejected it on a reason that has since expired

**Gate 1 needs `authorized_at + max_landing`. Gate 4 needs
`authorized_at + retention`. This system records `authorized_at` nowhere.**

- `ports.OrderRecord` carries `Order`, `State`, `Cumulative`, `Fills`,
  `Version`. No instant.
- `domain.Order` is *"the IMMUTABLE half: everything about an order that was
  fixed when it was planned"*. No instant, correctly — the plan does not know
  when dispatch will happen.
- `ports.Journal` is `AppendPlanned(Order)` and `AppendState(id, state)`.
  Neither takes a time, and the SQLite schema has no column for one:
  `journal(seq, kind, client_id, prev_hash, hash)`,
  `journal_state(seq, state)`.

The journal's absence is **deliberate and recorded**, which is what makes this a
finding rather than an oversight. `journalcodec.go`:

> *"There is no timestamp in a record, and the absence is deliberate. §5.1
> orders the journal by its sequence, not by a clock, and §7.5 replays it in
> that order. A wall-clock field would need `platform/clock` injected into an
> adapter that otherwise has no time in it, and it would add a value the chain
> covers but **nothing reads** — an attractive nuisance for a later reader who
> mistakes it for the ordering authority. **When an event time is genuinely
> needed it is the venue's, and it arrives on a fill.**"*

Every clause of that was true when it was written in slice A. Two are false now,
and the second is false in the exact case the rule exists for:

1. *"nothing reads it"* — §5.3's retention bound reads it, as of v0.9.33, which
   landed **one slice later** than the rejection.
2. *"when an event time is genuinely needed it is the venue's, and it arrives on
   a fill"* — the anchor is needed for an order with **no fill and no venue
   record at all**. There is no fill for it to arrive on; that is the definition
   of the case. And the venue's own instant is unavailable for the same reason:
   a not-found answer carries no `created time` to read.

**This is the recorded shape "a rejected alternative is an unchecked claim",
with a second axis: the rejection's REASON was a claim about the future
("nothing reads it"), and nobody re-derives a rejection when the future
arrives.** The marker grep cannot see it — no clause was departed from, a clause
is *absent* — which is the same limit §52d hit from the other direction.

### The consequence is a stall, and it lands on the recovery path specifically

`MemoryOrderWriteRepository` is *"backed by process memory"*. So after a crash:

- The projection — and any anchor we put only in it — is **gone**.
- The journal survives and says the order reached `DispatchAuthorized`, which is
  exactly what §7.5 replays and exactly the state §5.3 says is *"resolved only by
  `OrderByClientID`/`Fills` under the venue-specific stable-not-found rule"*.
- With no anchor, gates 1 and 4 cannot be evaluated, so `AbsentAfterFinality` is
  **unreachable** for a crash-recovered order.

An order stuck short of that terminal sits in a state that
`ContributesPossibleDelta` answers `true` for, so its hedge group is `Ambiguous`
for ever, §6.7 forbids compensating the residual, and new trading is held. The
familiar direction: **not a wrong number, a system that halts itself and reports
every component healthy** — the same sentence `ContributesPossibleDelta`'s own
doc uses about `!IsTerminal()`.

### The ruling

**The anchor is a `clock.ServerTime` in the VENUE's frame, captured by the
dispatcher immediately before the authorization fsync, and it is normative that
the journal must carry it.** Three parts:

1. **The frame is the venue's, not this host's.** Gate 4's comparison is against
   the venue's own arithmetic — Binance's cliff is literally *"created time + 3
   days < current time"*, evaluated by Binance on Binance's clock. A local
   instant compared against a venue retention is this repo's recorded
   *recomputed-check-must-share-the-anchor* defect: off by a small stable offset
   and reading as correct. `platform/clock`'s `ServerTime` carries the
   uncertainty band and `Passed()` returns a three-valued `Certainty`, which is
   the instrument this comparison has been waiting for.
2. **Where it errs is stated and checked, not assumed.** Our authorization
   precedes the venue's creation, so `anchor ≤ venue_created`, so
   `anchor + retention ≤ venue_created + retention`: our bound expires EARLIER
   than the venue's. Gate 4 therefore refuses slightly early, never late. This
   is a claim about a quantity, so per `FM-7`'s lesson it is a test with a name
   rather than this paragraph.
3. **This slice puts it in the projection; the journal is a SPEC EDIT and slice
   F's construct.** `ports.OrderRecord` gains `AuthorizedAt`, and
   `ApplyState` is closed against `StateDispatchAuthorized` so the anchorless
   path stops being expressible (a signature closing what a check cannot). The
   journal column is NOT built here, deliberately, on the recorded
   *schema-field-frozen-before-its-consumer* rule: its consumer is §7.5's
   replayer, which is slice F. What lands here instead is §5.3 stating the
   requirement and the resolver **refusing to prove absence without an anchor**,
   which is the safe reading of a crash-recovered order and is what makes the
   gap loud rather than silent.

## §53c — the ruling C2b deferred: what an UNPUBLISHED retention licenses

C2b left this explicitly: *"Unpublished says the unknown out loud, and slice D
decides what an unpublished retention licenses."* Binance SPOT's Query-order
page states no retention at all, so `StatusReach` answers
`QueryReach{Unpublished: true}` for every spot order.

**Both defaults are unsafe, and unequally:**

- **Treat unpublished as "no cliff"** → a spot absence is proved absence for
  ever. That writes an evidence class we do not have into a terminal state, and
  §7.5 consumes it as truth after arbitrary downtime. This is the direction
  C2b's `StatusReach` comment already refuses to encode.
- **Treat unpublished as "never provable"** → a lost-ack spot IOC that did not
  fill can never reach `AbsentAfterFinality`. The spot leg of a cash-and-carry
  is a BUY on spot (§6.7's naked-long case), so this is not a corner: it is half
  of every trade. The hedge group stays ambiguous for ever and new trading is
  held — §53b's stall, arrived at by being careful.

**So neither default is available and the carrier is forced** — this repo's
recorded *a-policy-with-no-carrier* pattern. `Resolver` takes a configured
`AssumedReach` per `(venue, product)` and **refuses at construction** when a
surface it may be asked about has none. The value is a **local assumption with a
named owner, not a venue fact**, so it does NOT go in `venue-profiles.md`; §8's
"one assumed fact" convention is the precedent for saying so out loud.

Refused at construction rather than at the moment of proof, for the reason §6
step 6 gives about the fsync: the moment of proof is downstream of an order
already at a venue, and a refusal there reports a gap it cannot prevent.

## §53d — WHICH retention gate 4 compares against, and it is not the obvious field

`QueryReach` carries two durations: `UnfilledRetention` and `FilledRetention`
(Binance USDⓈ-M: 3 days and 90 days). The tempting read is *"our orders are
IOCs that mostly do not fill, so use `UnfilledRetention`"*.

That is backwards. Gate 4 asks whether an ABSENCE is explicable by purging, and
at that moment **we do not know which kind of order it was** — that is what the
absence means. The absence must be inexplicable under EVERY hypothesis, so the
bound is the retention that **expires first**:

```
reach = min(UnfilledRetention, FilledRetention)
```

`min` rather than `UnfilledRetention` even though the two are ordered that way on
both venues today, because the ordering is a venue fact and not a type
invariant, and reading a field by its name instead of by its role is how a
venue that publishes them the other way round silently widens the bound.

**Collapse-checklist note, before the tests are written:** a fixture whose two
retentions are EQUAL makes `min(a,b)`, `a` and `b` the same answer, so it cannot
tell this rule from either wrong one. Every gate-4 fixture varies them.

## §53e — C2b finding 4 becomes executable: the sweep cannot always attribute

C2b established that **on Binance a swept execution carries no client order id at
all** — neither `/fapi/v1/userTrades` nor `/api/v3/myTrades` returns one — so the
venue `orderId` is the only join key. Gate 3 asks "does this sweep contain an
execution belonging to our order", and on Binance that question is:

- answerable when we hold the order's `VenueOrderID` (from an ack, or from a
  present `OrderByClientID`);
- **unanswerable when we do not** — which is precisely the absent case, since a
  not-found answer supplies no id either.

So on Binance, for an order that is absent from the order surface, an execution
row in the window can be neither attributed to us nor ruled out. Gate 3
therefore has three outcomes, not two, and the third is reported rather than
absorbed:

- **no executions in the window** → gate 3 satisfied;
- **an execution attributable to this order** (client id match, or venue id
  match where known) → the order EXISTS and has fills; absence is refuted;
- **an execution attributable to neither** → `SweepInconclusive`. No proof, no
  terminal, new trading held.

The third arm is what makes the rule honest on Binance. Its frequency is low —
an unattributable fill on the same instrument inside a window of seconds — and
its cost when it fires is a hold, which is the direction §6.7 requires.

## §53f — rejected alternatives

1. **"Stable" means polling: query N times over T seconds and call it absent if
   every answer agrees.** Rejected, and the spec rejects it in terms: §6.7 step
   3 forbids *"a synthetic stable-not-found alone"*. Repetition adds no evidence
   about a send that has not landed yet — N agreeing not-founds inside the
   propagation window are N observations of the same unfinished race. Stability
   comes from the FENCE (gate 1), which is a claim about time, not from
   agreement between samples. The polling rule is also the one that looks
   diligent, which is why it is written down here as rejected rather than left
   unmentioned.
2. **Put the anchor in `domain.Order`.** Rejected: `Order` is the immutable
   plan-time half, and planning does not know when — or whether — dispatch will
   happen. A nullable instant there would have a zero value that reads as an
   order authorized at the epoch, which gate 4 would then judge purged.
3. **Give `ApplyState` an optional instant.** Rejected on the port's own
   argument for having two Append methods: an optional field whose zero value is
   legal-but-meaningless is the shape slice A refused for the journal record.
   `ApplyAuthorized` is total, and closing `ApplyState` against
   `StateDispatchAuthorized` makes the anchorless authorization inexpressible
   rather than merely discouraged.
4. **Let the resolver read the anchor from the journal.** Rejected for this
   slice: `ports.Journal` has no read method, and adding one is §7.5's
   reconcile-only boot — slice F, whose whole subject it is. Named here so slice
   F inherits it as an obligation rather than rediscovering it.
5. **Have the resolver apply discovered fills.** Rejected: `ApplyFill` takes a
   `domain.Fill`, which carries an `Account` this layer cannot populate — the
   same reason `engine/venue` mints its own `Execution` rather than reaching for
   `domain.Fill` — and §5.3 requires the fill path to be ONE transaction with the
   cursor advance and the outbox append, which no port here declares. The
   resolver REPORTS executions and refuses the absence proof; applying them is
   the fill path's.
6. **Resolve every state, not just the two that admit an absence.** Rejected in
   favour of deriving the admissible set from `domain.CanTransition` — the
   resolvable states are exactly those from which `StateAbsentAfterFinality` is a
   legal successor (`DispatchAuthorized`, `SubmissionUnknown`). Restating that
   set here would be a second home for SM §1's edge set, and the two would drift.
   It also refuses `StatePlanned` for free, which matters: querying a venue about
   an order we never sent runs the contract to its deadline and alerts an
   operator about our own unsent order.

## §53g — scope, and what is deliberately NOT here

**In:** `ports.OrderResolver` declared **with** its consumer; the
`application.Resolver` use case (the four gates, journal-then-apply of the
resolved state); the `driven.OrderResolverGateway` ACL over
`*venue.OrderClient`; the anchor (`OrderRecord.AuthorizedAt`, `ApplyAuthorized`,
`ApplyState`'s new refusal, the dispatcher's `clock.Clock`); module wiring.

**Out, each named with the construct that will carry it:**

- **The journal's durable anchor column** → slice F, with §7.5's replayer (§53b).
- **Position/balance reconciliation to the same watermark** — SM §2 step 4 →
  slice E/F. This slice runs steps 1–3, which are the whole of the
  stable-not-found rule; step 4 is the classifier's composition.
- **Applying discovered executions** → the fill path (§53f item 5).
- **Deciding what a resolution means for the hedge group's residual** → §6.7,
  slice E. The resolver returns a `Resolution`; it classifies nothing.

## §53h — the build record

**Landed** in two commits: the code half (`6508fb1`, marked WIP with two
`[SPEC EDIT OWED]` markers live) and this close. `make ci` green on both modules,
marker grep back to **zero**, docs at **v0.9.34**.

**Files.** New: `internal/oms/application/ports/resolver.go`,
`internal/oms/application/resolve.go`,
`internal/oms/infrastructure/driven/orderresolver.go`, plus `resolve_test.go` and
`orderresolver_test.go`. Changed: `ports.OrderRecord` (+`AuthorizedAt`),
`ports.OrderWriteRepository` (+`ApplyAuthorized`, `ApplyState` closed against
`StateDispatchAuthorized`), `application.Dispatcher` (+`clock.Clock`),
`domain.successors` (+the `Canceled` edge), `oms.Deps`/`Module`.

**Spec edits landed.** Core **§4** (the consumer port narrows to the two read
methods), core **§5.3** (the anchor, its durability requirement, the `min()`
retention rule, the forced carrier for an unpublished reach), core **§12**
(milestone-3 slice-D annotation), State-Machines **§1** (`Canceled` on both
query-resolution rows, plus the anchor as an invariant). Both markers discharged
as `[SPEC EDIT LANDED — v0.9.34]`.

### The mutation battery — 28 aimed, 28 killed, and eight survived the first pass

Every survivor was a **real gap**, none was an equivalent mutant. They fall into
three families, and the families are more useful than the count.

**(a) The mirror rule's second axis, INSIDE one rule.** §5.3's two time bounds
read *opposite* edges of the same uncertainty band. Gate 1 had a fixture placing
the clock inside that band — the only window one band wide where "latest edge"
and "instant" give different answers. Gate 4 had none: every fixture sat hours
from its cliff, so both `Earliest()`→`Latest()` and
`!= CertainlyNotPassed`→`== CertainlyPassed` survived. The test file's own
fixture header stated the pair and cited a single test, which is how the second
came to be missing. A nonzero band makes the distinction **expressible**; only a
clock placed inside it makes the distinction **observed**.

**(b) Assertion-shaped, not fixture-shaped.** Both survivors here had correct
fixtures and blind assertions. `ProvenThrough != horizon` on a truncated sweep is
satisfied by **zero**, so a loop recording the watermark only on the final page
(the page a truncated sweep never reaches) passed. `calls < hardStop` is
satisfied by every wrong page bound between 1 and 199. Both were negative
assertions; both are now positive ones against a known exact value — the page
budget stated as a **literal**, since a constant shared with the code under test
moves with it and checks nothing.

**(c) The field the slice exists to add, never read back.** `AuthorizedAt` was
passed by every repository test and asserted by none, so a repository that
dropped it on the floor passed the whole suite. The unset-anchor refusal — the
case `ErrAuthorizationNeedsAnchor` is *named* after — was asserted only for the
wrong-method path. And the dispatcher's own anchor check was **masked one layer
down**: `ApplyAuthorized` refuses the same instant, so deleting the dispatcher's
check still produced an error and any test inspecting only the error passed. What
changes is the **ordering** — with the check gone the authorization is journaled
and the order is *sent* — so only "nothing journaled, nothing placed" could see
it.

One `NOBUILD!!` (`declared and not used: ok`) was a bad sed orphaning a
neighbour, not a compiler-enforced claim; re-aimed to `ok || true` and killed.

### What the checklist caught before the battery

Dead code in the code just written: `attribute()` took a venue order id, and its
only caller is the absent path, where the venue by definition supplies none —
two unreachable arms reading as handled cases (§53e). Deleting them converted a
vague hazard into a priced one, and the price is a raised task rather than a
comment.

### The reading pass — two findings, both citation-shaped

The pass sits **beside** the marker grep because the grep gates *marked* claims and
neither of these carried a marker.

**1. Ten sites cited "§6.7 step 2" / "§6.7 step 3", and core §6.7 has no numbered
steps.** The four steps are State-Machines §2's, and §6.7 says so itself — *"the
contract's four steps: [mechanism: State-Machines §2]"*. The sharpest instance is a
**verbatim quotation attributed to a section that does not contain it**: `Executed is
§6.7 step 2's "authoritative final cumulative executed quantity"` — that phrase appears
**nowhere** in the core. This is C1's citation-to-a-nonexistent-section tell, and the
tell was visible *inside the same doc comment*, which cites "SM §2 step 1" correctly for
gate 1 and "§6.7 step 3" for gate 3. Retargeted to `SM §2 step N`; the quotations that
genuinely ARE §6.7's — *"a point-in-time not-found cannot collapse an in-flight
order"* — were left as they were, and keeping both makes the difference legible.

**2. State-Machines §1 stated two different evidence rules for the same target state.**
`CancelPending`/`CancelUnknown` reach `Canceled` by *"explicit venue-terminal cancel +
the final executions sweep"*. The **venue-initiated-termination** row — v0.9.11's, and
the one that fires on this system's *ordinary* outcome — named no sweep. §2 admits no
per-source exemption (*"the ONE canonical boundary … never a weaker cancel-ack-only
variant"*), and this slice's own code already refuses one: `present()` runs the sweep
before recording **every** terminal, explicitly rejecting a per-state exemption on the
ground that it would be a second rule about evidence held in one function. The
partially-filled IOC is where the gap bites — the status is terminal before the last
fills have been read, so collapsing on status alone collapses early. The row now carries
the rule. **No code changed**, because the code was stricter than the row it implements
— which is why nothing failed and why only a reading pass could find it.

### Tasks raised at the close

- **Persist `Ack.VenueOrderID`** so a Binance sweep can rule out our *other*
  orders — without it, any execution row in the window makes the absence
  inconclusive (§53e).
- **A §7.5 boot posture check** refusing to trade an instrument whose absences
  could never be proved — the check that would be *in time*, where gate 4's
  `ReasonReachUnknown` is safe but late (§53c).
