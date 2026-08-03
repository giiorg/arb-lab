# M3 slice E — the §6.7 classifier: determined vs ambiguous residual

**Task:** #164 (§47f slice E)
**Date:** 2026-08-03 (session 83)
**Reads:** core §6 step 7 (*"§6.7"*, the normative owner), core §5.3 (the state
set, the command verbs, the §7.6 class table), core §5.1 (the aggregate and the
**hedge-group key**), SM §2 (the classifier + the four-step finality contract),
SM §7 (generations and the reserved-reduction ledger),
`docs/formal/ambiguous-order-model.md` **§3's correspondence map, read before
the `.tla`** per §47f, and `AmbiguousOrder.tla` itself for the four definitions
§54f turns on.

---

## §54a — what the classifier IS

§6 step 7 states it in one sentence and the sentence has three verbs:

> On **every** fill/position event **and every hedge-group order-lifecycle/
> finality event**, recompute net delta from the fill ledger and **classify the
> residual as *determined* or *ambiguous* before acting** — this classification,
> not the raw delta, gates the compensating trade.

So the object has three parts, and they are separable:

| | Part | Where it lands |
|---|---|---|
| 1 | **The residual** — `realized_delta − outstanding_reserved_reductions` over one hedge group | `domain` — arithmetic over `fixedpoint.Quantity` |
| 2 | **The verdict** — `Determined` / `Ambiguous` / `DeadlineExceededAmbiguous`, from the group's possible-delta intervals | `domain` — a §3.6 rule 8 enum-like plus the interval algebra |
| 3 | **The re-run rule** — the verdict is recomputed when *any* of three inputs moves | `application` — the fold over records, and the ledger whose mutators *are* the re-run |

The load-bearing asymmetry, which the model states and which decides most of the
design below: **the classifier can refuse to conclude, and must never conclude
wrongly.** `Ambiguous` is the safe verdict and `Determined` is the one that
unblocks a trade, so every uncertainty resolves toward `Ambiguous` — with one
exception that is not symmetric and is §54e.

## §54b — RULING: the hedge group gets a carrier, because it has three components and the orders carry two

§5.1 defines it exactly once, and not in §6.7:

> It holds an **exclusive lease per hedge-group `(account, venue, symbol)`** —
> deliberately grouping a trade's spot + perp legs (**`Account` = the venue
> subaccount** — the lease MUST span both legs, which on Binance are distinct
> spot/futures API domains under one subaccount and on Bybit are one UTA, §4.1),
> and **distinct from per-instrument identity (§5.3)**.

Three facts follow, and only the first is obvious.

**(1) The group is not the trade.** `ClientOrderID` embeds a `TradeID`, so
grouping by trade is the cheap implementation and it is the wrong one: §5.1's
key has no trade in it, and the lease exists precisely so a *successor* trade on
the same key is refused. Netting per trade would classify two trades on one
symbol independently while they share one venue position.

**(2) The group is not the instrument.** §5.1 says so in terms. Spot BTCUSDT and
perp BTCUSDT are different `VenueInstrument`s with independent order and
execution-ID namespaces (§5.3), and the whole point of the classifier is that
their exposures net.

**(3) `ports.OrderRecord` carries two of the three components, and the missing
one is `Account`.** `domain.Order` holds a `VenueInstrument` — venue, product,
symbol, contract type — and nothing else that identifies who holds it.
`domain.Fill` carries `Account string`, because §5.1's dedupe key needs it. So a
set of order records cannot be grouped by §5.1's key without a component none of
them holds.

**The failure this produces is a wrong number, not a halt**, which is why it is
worth stating rather than absorbing. A classifier keyed on `(venue, symbol)`
alone nets across subaccounts. Positions and margin are per subaccount, so a
naked long in subaccount A and a naked short in subaccount B net to a residual
of zero — the group reads flat, `ExposureReduction` is never sized, and both
subaccounts stay naked with every invariant green. It is the mirror of the
usual failure direction here: the classifier concludes *determined-and-flat*
when it should conclude *two groups, each determined and naked*.

**Taken:** a `domain.HedgeGroup{Account, Venue, Symbol}` value object with the
zero value unusable, derived from an order record plus its account — never
inferred. The classifier takes the group as an argument and **refuses any record
that does not belong to it**, so a caller that assembled the wrong set gets a
refusal rather than an answer computed over it.

**Rejected: derive the group from `VenueInstrument` alone and add `Account`
later.** That is the §47e shape — a normative claim the code silently does not
satisfy — and it is invisible to the marker grep because the clause would be
*absent*, not departed from. It is also the shape a later `Account` field cannot
fix on its own: by then every call site has a group key that compiles.

## §54c — RULING: a reservation entry is CREATED AT ZERO, so `FM-5` becomes inexpressible rather than forbidden

`FM-5` is the model's first finding and it is a **spec** defect: §7's FLAG-3
(*"the reservation stays generation-scoped from creation"*) composed with §2's
unconditional reservation-aware sizing let a generation book its **target** into
the shared ledger at `Planned`, before its perp leg had bought anything. The
package then reads short by that amount and §2's determined branch buys back
exactly the exposure the generation's own perp leg is about to buy.

The normative fix (v0.9.24) is that an entry carries its producer's
**outstanding net intent** — *"the exposure change its reserving action has
already COMMITTED and not yet REALIZED, never a target it merely intends"*.

**The executable form is the model's own nominal constant.** In
`AmbiguousOrder.tla`, `PdPlan` books

```
Pend(IF RESERVE_AT_CREATION THEN t ELSE 0, "pkgspot", FALSE)
```

— the nominal reading is **literally an entry of zero at creation**, and `NC7`
(the control that *is* `FM-5`) is the branch that books `t`. So the Go
constructor takes **no amount**: an entry is created empty and becomes nonzero
only through `Amend`, which is the producer stating what it currently has
outstanding. `RESERVE_AT_CREATION = TRUE` has no Go spelling.

This is a genuine narrowing rather than a comment, and it costs the one producer
that is nonzero from its first instant — the `IntervalFloorReduction`, which
carries *"its full dispatched size from dispatch"*. That producer must reserve
its slot and then amend it, and **the two-step is correct rather than a tax**:
§6.7 requires the floor to reserve *before* submission, so the ordering
reserve → amend → submit is the one the spec asks for, made visible.

**What no type can check, stated so it is not mistaken for checked:** whether
the amount a producer amends to is *truly* backed by what it has committed. The
producer is outside this ledger. What the type removes is the misuse §46 §8e
says no invariant could catch — and the reason it could not is that *an
invariant built on the ledger cannot audit the ledger*, which is §47f's first
standing hazard for this slice, inherited unchanged.

**Rejected: `Reserve(amount)` with a doc comment forbidding a target.** That is
the shape the spec had, and it is what four review rounds and slice A of the
model read past. A rule whose only enforcement is a sentence in the paragraph
that also states the sizing is the exact configuration `FM-5` came out of.

**Rejected: `Reserve(committed, realized)` computing `amount = committed −
realized`.** Tempting, and it is what the perp-side ladder does. It is wrong at
`PdAuthorizeSpot`, which the module checks: that action sets the entry to **`s`,
the authorized sale size**, not to `pdDone − pdSold`. The two coincide only
while `s` is the whole transient, so a `committed − realized` type would
misdescribe the ladder's second half — and it would do so by making the
authorized-short case *unrepresentable*, which is a refusal in the wrong place.

## §54d — the three inputs (`FM-6`), and why the third one's mutators return the verdict

`FM-6` is `FM-5`'s sibling: *the ledger must be backed when it is read* versus
*a change to the ledger must be observed*. §2 Scope used to say classification
was a function of **two** inputs; the residual it gates on is
`realized_delta − outstanding_reserved_reductions`, which is **three**, and the
subtrahend moves on events that are not order events at all — a generation
opening, amending, completing, terminal-flattening, being superseded; an
`IntervalFloorReduction` retiring; a protective order's leg reservation
released. TLC's signature was a **dead end**, not a violated invariant.

So the three inputs are:

| | Input | Moved by |
|---|---|---|
| 1 | realized delta (the minuend) | fill / position events |
| 2 | the possible-delta intervals | order-lifecycle / finality events |
| 3 | `outstanding_reserved_reductions` (the **subtrahend**) | the ledger's producers, on events that are neither |

**Taken:** the `Classifier` owns the reserved-reduction ledger, and every
mutator on it — reserve, amend, release — **returns a fresh verdict**. A ledger
change that does not re-classify is then not a discipline anybody has to keep;
it is a call that does not exist. Inputs 1 and 2 keep the ordinary shape (the
handler re-runs on the event), because they already had a stated re-run rule and
the model's control for them is `NC13`/`NC4`, not `NC19`.

The asymmetry is deliberate and worth naming: the input with the executable
guard is the one the spec forgot, not the one it stated. **A re-run rule that is
written down has a reader; a re-run rule that is missing has none.**

## §54e — the interval algebra, and the one place clamping is right

Each order that `ContributesPossibleDelta` contributes an interval over its
**remaining executable quantity**, `ordered − cumulative`, signed by side: a buy
contributes `[0, +r]`, a sell `[−r, 0]`. A single order executes in one
direction, so its own interval always has zero as an endpoint and **never
straddles** — straddling is a property of the sum, which is exactly where §6.7
forbids compensation (model (a) §3, and §46 §7a decision 1 rejected the
`(lo,hi)`-per-order encoding as unfaithful *and* unbounded).

`domain.Ledger.Remaining` is signed and may be **negative** — a venue
over-execution — and its own doc says clamping it at zero "would report a
package as flat while it holds reverse exposure". **The interval is the one
place the opposite is true**, and the two must not be confused:

- The **residual** must not be clamped: the overshoot is realized exposure and
  belongs in the minuend, where it moves the answer.
- The **interval** must be clamped at zero: an over-executed order has no
  remaining executable quantity — an execution cannot be un-executed — so its
  future contribution is the point zero. An unclamped negative remainder would
  invert the interval and hand a buy order a *short* endpoint it can never
  reach, which widens the sum toward straddling and turns a compensable residual
  into "de-minimis only".

Same quantity, two consumers, opposite correct treatments. It is recorded here
because the tell is invisible at the call site: both readings compile and both
produce a plausible number.

The floor magnitude is the endpoint **nearest zero** (`lo` when `lo ≥ 0`, `|hi|`
when `hi ≤ 0`), and `NC1` is why it is not the worst endpoint: residual `+1`
with a pending sell of `1` gives `[0, 1]`, whose near endpoint is **zero** —
because the pending hedge may already cover the whole position — so
nearest-zero issues nothing and worst-endpoint sells 1 and reaches `−1`.

## §54f — scope: which of `FM-5`…`FM-9` become executable HERE, and who owns the rest

§47f's slice-E row says *"this is where `FM-5`…`FM-9` become executable
claims"*. Three of the five land here in full. The other two have a consumer
that this milestone does not build, and the honest split is by **consumer**, on
slice D's own precedent (`AuthorizedAt` landed on the record; its journal column
went to slice F *with its replayer*).

| Finding | Slice E | Why |
|---|---|---|
| **`FM-5`** — an entry read before its backing exists | **Executable** — §54c's create-at-zero constructor | The ledger is this slice's |
| **`FM-6`** — a change to the ledger must be observed | **Executable** — §54d's mutators return the verdict | Same |
| **`FM-7`** — the floor is issued once, as *defence in depth* | **Executable** — a per-episode flag, reset when the classifier reports `Determined` | §6.7's own row |
| **`FM-8`** — an entry's premise outliving it | **Half.** The *release* is here: §2 row 4's "the floor's own ambiguous result never blind-retries; it routes back through this contract", which releases its entry and re-enters the exposure as an unresolved order. What must be *told* is a `PackageDelever` generation, which does not exist | SM §7's ladder is milestone 4 (§7.1's liq-distance escalation) |
| **`FM-9`** — a rule evaluated on a quantity including its own subject | **Deferred, with the seam named.** `PremiseResidual` nets a generation's own transient and its own ledger entry out of both sides of the same subtraction. There is no generation to net | Same owner |

**`FM-9`'s deferral names the construct, per §3.6's deferral rule**, and the
construct is **SM §7's generation ladder**, not "the classifier". The seam it
needs from this slice is that the ledger can report **one producer's own
contribution separately from the sum** — which the three-producer `kind` already
gives it — so nothing has to be re-derived when the ladder lands.

**A derivation that looked like it would collapse the deferral, and does not.**
Under §54c's encoding it appeared that `PremiseResidual` reduces to the ordinary
residual, since the generation's entry would always equal `pdDone − pdSold` and
cancel out of both subtractions. **The module refutes it:** `PdAuthorizeSpot(s)`
sets the entry to `s`, so entry and own-transient part company for the whole
second half of the ladder. The netting is not a no-op, and had the derivation
been believed the deferral would have been recorded as "already structural" —
a `FM-7` shape, a justification nobody re-derives. It was checked against
`AmbiguousOrder.tla` line 1289 rather than reasoned from the companion.

**Also deferred, with owners:** the **fourth ledger producer** — a protective
order's leg reservation, which §2's Invariants name and model (a) leaves
unmodeled as SM §5's — is *not* declared as an enum member here, because a
declared member nothing emits satisfies the compiler, `exhaustiveswitch` and
every switch-coverage count while never being reported. And the **issuance** of
the compensating trade: §6.7's reduction carries §5.3's state-dependent
`reduceOnly`, which follows *"the instrument's current position, not the
package"*, and `PositionAllocation` has no domain type. The classifier therefore
delivers a **decision** and the order that executes it is milestone 4's.

## §54g — rejected alternatives

**(i) Give the classifier `ports.OrderWriteRepository` — the same adapter it
already reads through.** Rejected: §6.7 is a *decision*, and every state change
it implies is made by the caller that acts on the verdict, through the
journal-then-apply path. With the write repository in hand, a classification
could advance an order's state directly — no journal record, nothing for §7.5 to
replay, and the projection ahead of the log it is a projection **of**. The
one-method read port makes that unwritable rather than reviewed, which is the
move `Dispatch` made when it took a `ClientOrderID` instead of an order.

**(ii) Have the caller hand the classifier the group's records.** Rejected on
`FM-6`: the ledger's mutators must be able to recompute, and a classifier that
needed its records supplied could not re-classify on a subtrahend change without
the caller passing them again — which puts the re-run back in the caller's
discipline, exactly where `FM-6` found it missing.

**(iii) Derive the hedge group from `VenueInstrument` alone.** Rejected in
§54b. It is a normative claim the code silently would not satisfy, and the marker
grep is structurally blind to it because the clause would be *absent*. It is also
not fixable later by adding the field: by then every call site has a group key
that compiles.

**(iv) Track a group lazily, on first classification.** Rejected: an unknown
group would then answer `Determined`, residual zero, no reservations — the one
verdict that unblocks a compensating trade — for a group whose orders and
reservations the classifier has never seen. A typo in a group key would read as a
flat package. It also cannot answer an empty group at all without a base asset,
which is why `Track` carries one.

**(v) Give the classifier a clock and time the deadline itself.** Rejected: the
deadline belongs to the *quiesce attempt*, which `Resolver` runs, and §9's ≤30 s
quiesce RTO is a chaos-gate number rather than a property of this type. Formal
model (a) makes the same abstraction — `DeadlineBlows` is an environment event
and there is no real time in the model — and a clock here would put a second
authority on when the contract has failed.

**(vi) Declare the fourth ledger producer (a protective order's leg
reservation) now.** Rejected: a declared enum member nothing emits satisfies the
compiler, `exhaustiveswitch` and every switch-coverage count while the state it
names is never reported. SM §5 owns protective orders and no code in this
milestone arms one. It lands with its emitter.

**(vii) Build `PremiseResidual` now, so `FM-9` is "executable" in this slice.**
Rejected on slice D's own precedent and on the
schema-field-frozen-before-its-consumer hazard: the quantity's consumer is SM
§7's generation ladder, which is milestone 4. What this slice owes it is the
**seam** — `OutstandingFor`, the sum decomposable by producer — and that is
built and exercised. §54f records the derivation that would have made this look
unnecessary, and the module line that refutes it.

## §54h — the build record

**Landed.** `internal/oms/domain/hedgegroup.go`, `possibledelta.go`,
`reservedreductions.go`, `classification.go`;
`internal/oms/application/classify.go`;
`internal/oms/application/ports/hedgegroup.go`; `OrdersIn` +
the account field on `driven/orders.go`; `Deps.Account` and `Module.Classify` in
`module.go`. Core §5.1, §6 step 7, §12 and State-Machines §2 edited. Marker grep
**0**; `make ci` green on both modules.

### The collapse checklist, run BEFORE the battery

It changed six fixtures, and every one of them is a distinction the code makes
that the natural fixture would have hidden:

| Fixture | Would have collapsed |
|---|---|
| The two legs' quantities | A cash-and-carry's legs carry the **same** base quantity, so buy 1 / sell 1 nets to zero under *every* sign convention — including "add both magnitudes". The legs are unequal in every sign test |
| The straddle table | `<` versus `<=`: every row sits exactly **on** a boundary or one step off it, since a range strictly inside or strictly outside makes the two predicates identical |
| The floor magnitude | A range whose near endpoint is zero cannot tell "nearest zero" from "worst endpoint" — `NC1`'s own case. The issuing fixture is `[0.4, 1]`, where they differ |
| The reserved ledger's producers | Three producers at three **different** amounts, none equal to the total: with one, "this producer's contribution" and "the whole sum" are one number |
| The `Reserve`-books-nothing fixture | On an **empty** ledger, "created at zero" and "there is nothing here" are the same sum. It is asserted against a pre-existing nonzero entry |
| The state table | Both rows hold the identical order and differ **only** in `OrderState`, so the state's effect is attributable |

### The mutation battery — 34 aimed, 34 killed, two survived the first pass

Both survivors were **one shape, and it is a new one for this list**: the
episode reset. Deleting either flag's clear left every existing test green, for
two different reasons that amount to the same thing — the deadline flag is
invisible at the moment of resolution, because `Classify`'s precedence returns
`Determined` for a collapsed range whether or not the flag is set; and the floor
flag is invisible because the next call in those tests was `DeadlineExceeded`,
which sets the deadline itself.

**The observable difference exists only on the FOLLOWING incident.** A fresh
ambiguity inheriting a stale deadline verdict holds new trading and admits an
interval floor with no deadline ever having blown; a spent floor flag leaves a
second, distinct incident with **no admissible safe action for ever**, which is
the reading §6.7's own placement of the rule (inside the
`DeadlineExceededAmbiguous` row) forbids. The fix is one test that walks **two
incidents**, and it kills both.

This is the collapse checklist's own shape with the varied dimension being
**sequence** rather than value — which is why the checklist, run properly and on
every fixture parameter, did not reach it. Worth adding to the list: *when state
is per-episode, the fixture that separates "cleared" from "latched" is the next
episode, not the current one.*

Four `NOBUILD!!` verdicts, all bad seds and all of the recorded kind — three
"declared and not used" from replacing a condition with `false` and orphaning
its only reader, one duplicate `case` — each re-aimed to a wrong *value* rather
than a deleted call. One `NO-OP!!` on a line number that had moved.

### The reading pass — one finding, and it is a number

State-Machines §2 cites `NC20` — the run that establishes "never issued twice"
as defence in depth rather than a live hazard — as **1,086,199 distinct
states**. The current suite reproduces **1,093,953**: the model changed in slice
F (`FM-8`'s standing precondition), and `docs/formal/ambiguous-order-model.md`
§4 records the re-measurement. §46 froze the old figure *deliberately*, as a
record of what that release checked, which is correct for a record and wrong for
a normative sentence.

By the time this slice read it, the stale figure had reached **six documents**,
and it was on its way into a Go doc comment — this file's author copied it there
before checking. **A figure no run reproduces reads exactly like one that does.**
Fixed in SM §2 with its provenance kept, and the companion named as the
authority.

### Tasks raised at the close

- **`Fill.Account` has no producer**, and it is the first component of two
  normative keys (§5.1's dedupe key and its hedge-group key). The account has to
  reach the fill path from the venue credential that identifies the subaccount.
- **`FM-9`'s consumer** — SM §7's generation ladder — with the netted premise
  quantity, in milestone 4. The seam it reads is `OutstandingFor`.
