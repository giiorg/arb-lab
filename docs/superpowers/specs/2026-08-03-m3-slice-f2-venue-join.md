# M3 slice F2 — §7.5's venue join: the position enumeration, the rebuilt fill ledger, and the invariant's real domain

**Task:** #178 (§47f slice F, second half)
**Date:** 2026-08-03 (session 85)
**Reads:** core §7.5 (the normative owner), §5.3 (the fill ledger's orthogonality,
the anchor, "resolved by query only"), §5.1 (the dedupe identity, the hedge-group
key, the account), §6 steps 6/7/10 (the crash boundary, the projection-soundness
clause, foreign orders), §6.7 (what a residual of zeroes means), §7.6 (the shed
class this call carries), Profiles §7/§8 (the submission and read surfaces) and
§55 (slice F1, which this takes `Recovery` from).

---

## §56a — what F1 handed over, and why the gap is the dangerous kind

F1 rebuilds STATE and explicitly cannot rebuild quantity: `ports.Journal` has no
`AppendFill`, so no fill has ever been journaled and every replayed record
carries a cumulative of zero.

Zero is not a missing number here. §6.7 folds those cumulatives into a residual,
a group of zeroes classifies **Determined** with a residual of zero, and
*Determined* is the verdict that PERMITS a compensating trade. A process that
booted, replayed and started classifying would size reductions against a position
it cannot see — and every invariant computed over the projection would agree with
it, which is §46 §8e's rule (an invariant built on a store cannot audit that
store) arriving at recovery.

So this slice's job is to make the venue's own record into the projection's, and
`boot.Checklist` is what holds the interval: `GateJournalReplayed` and
`GateNetDeltaAsserted` are separate gates and the posture is the weakest over the
required set.

## §56b — RULING: the position read is an ENUMERATION, not a per-instrument query

§4 names the surface `Positions` and this slice's client takes a **scope** —
venue, product family, settlement asset — and no symbol.

The argument is §7.5's own sentence. It alerts on *"foreign orders or unexplained
position deltas"*, and a read keyed on an instrument can only ask about
instruments this system already knows it traded, which is exactly the set that
cannot contain a foreign one. The per-symbol form is therefore structurally
unable to fail the test the section exists to run.

**The rejected alternative and its real cost.** A per-instrument read makes
"flat" a positive per-symbol answer, and the enumeration makes it an ABSENCE from
a list — a shape this repo has paid for (a negative assertion admitting the
degenerate value). Two things bound it and both are stated in the code: the query
names no symbol, so a missing symbol is missing because the venue says it holds
nothing there rather than because we spelled it wrong; and an unanswered
enumeration is an ERROR, never an empty report, which is §5.3's
never-infer-an-absence rule one surface over. The residual risk is a scope that
names the wrong settlement asset on Bybit, and that is why `Settle` is required
rather than optional — the field one venue ignores.

**A symbol we do not trade cannot be typed, and is reported by NAME.** The money
kernel refuses a `Quantity` without a base asset and nobody configured a
decomposition for a symbol this system does not trade. Carrying the venue's raw
decimal beside a sometimes-meaningless typed field was rejected (two fields, one
fact); failing the whole read was rejected (it conflates "cannot parse" with
"found something foreign"). §7.5 does not auto-correct a foreign position anyway
— it alerts — so the name is the whole requirement.

## §56c — RULING: Binance V3, because two versions' SILENCE means opposite things

`GET /fapi/v3/positionRisk`, not V2. Verbatim from the venue's changelog
(2024-07-24): *"`GET /fapi/v3/positionRisk`: Compared to `GET
/fapi/v2/positionRisk`, this endpoint only returns symbols that the user has
positions or open orders in."*

V2 returns a row for every symbol on the family, so an absent row means the
symbol does not exist; V3's absent row means the account is flat there. This
surface is read to decide whether the venue holds exposure nothing here can
account for, which makes *flat* a conclusion — and V3 is the version whose
silence carries it. Profiles §4 has cited *"`GET /fapi/v2/positionRisk` (or v3)"*
since 2026-07-20 for a different consumer (`isAutoAddMargin`), where the
parenthetical is harmless; for this consumer the two are not interchangeable.

A second fact came out of the verification and is recorded as evidence about the
source rather than as a venue fact we depend on: the **V2 endpoint page still
publishes IP weight 5 while the changelog raised it to 10** (2024-08-07,
verbatim: *"`GET /fapi/v2/positionRisk`: 5->10"*). An endpoint page can lag its
own changelog, and under-charging a weight is the direction that ends in the ban
§7.6 exists to avoid. Nothing here depends on V2's number; Profiles §9 carries
the ⚠ and the instruction to browser-confirm both figures before live trading.

## §56d — the two venues encode DIRECTION in different layers

	Binance USDⓈ-M   positionAmt is SIGNED; positionSide is "BOTH" in one-way mode
	Bybit V5         size is "Position size, always positive"; side is "Buy"/"Sell",
	                 and "" for an empty position

This is §7.6's header asymmetry one surface over: two venues, one quantity, and
only the venues' prose says which way it points. A decoder written once against
either reads the other's short as a long, with the magnitudes agreeing and
nothing failing — and the number it corrupts is the one §7.5 asserts before
trading resumes.

Two consequences are rules rather than defensive checks. **A row that is not
one-way is refused** on both venues (`positionSide != BOTH`, `positionIdx != 0`),
because a hedge-mode account answers with two rows whose net is a subtraction no
field states — §4.1 already makes that posture refuse-to-trade, and this query
runs before the posture check has anything to compare against. And **Bybit's
empty side is a VALUE**: `""` beside `size: "0"` is the documented flat row and
must be admitted, or a settled account becomes a failed boot; `""` beside a
NONZERO size is a direction the venue did not state, and both available defaults
report a live position as flat or as pointing the wrong way.

## §56e — RULING: the join admits fills, and it needs no transaction

The projection's cumulative moves only through `ApplyFill`, so the join admits
each attributed execution as a `domain.Fill`. Two premises that used to forbid
that have expired, and `ports.VenueExecution`'s own doc had recorded both:

- *"an Account, which nothing on this path can populate"* — the write repository
  has held one since slice E, where §5.1's hedge-group key turned out to have
  three components and an order record to carry two. §7.5's join is the first
  caller that both knows the account and holds venue executions, so **task #176
  is discharged here**.
- *"admitting a fill is a transaction §5.3 requires to include the recovery
  cursor and the outbox"* — that is a requirement on the LIVE path, where losing
  the write loses the fill because the cursor advances. This join advances no
  cursor: it re-sweeps from each order's authorization anchor on every boot, so
  its admission is **re-derivable**, and the dedupe identity makes the repeat a
  no-op. *Idempotence is what the live path buys with a transaction and this one
  gets from the venue.*

The port therefore widens to carry price and fee — the fields `domain.Fill`
refuses to be built without — and NOT side, because direction lives on the order
and a second statement of it is a second place for a sign error to live.

**This is the recorded "a narrowing carries an argument" shape, on a type.** The
reason a thing was narrowed away is load-bearing and unverified; here it was
written down, correct when written, and false one slice later.

## §56f — RULING: `attribute` gains a second key, and slice D's rejection of it expired

Slice D wrote: *"this function is reached from ONE place, the absent path, where
a venue that reports no such order has supplied no id for it either. A parameter
for it would be an argument every caller passes empty."* True of its only caller,
false of the function — and the difference arrived with this slice, whose orders
the venue DOES hold and whose resolving query returns the venue's id for them.

On Binance it is the only key that exists: neither `/fapi/v1/userTrades` nor
`/api/v3/myTrades` returns a client order id, so without it no cumulative could
be rebuilt on that venue at all. Task **#174** (persisting `Ack.VenueOrderID`) is
a different lever on the same fact and stays open: it would let the ABSENT path
rule out our other orders' rows, which this parameter cannot, because an order
the venue denies holding has no id to compare against.

## §56g — FINDING 1: the invariant is stated over a GROUP, and venue truth exists for one leg

§5.1's hedge group is *(account, venue, symbol)* and deliberately spans the spot
and perp legs. §7.5 asserts a net-delta invariant and names `Positions` among the
surfaces its join uses. **Neither venue has a position surface for spot** —
Binance's is `/fapi`, Bybit's `category` has no spot member — because a spot
balance is not a position.

The balance that stands in its place is not a substitute. It contains coins this
system never bought, so it is comparable to our exposure only against a
**recorded baseline**, which nothing in this spec defines. The failure of using
it anyway is the rare concluding one: a pre-existing 1 BTC reads as our long,
nets against the perp short, and the group reads over-hedged — so a reduction is
sized in the wrong direction, from a number every invariant over the group
agrees with.

**The ruling has two halves and the second is the one that is easy to get
backwards.** The uncoverable instrument is REPORTED on every boot
(`Join.Unasserted`), because a clean report is otherwise read as a complete one.
And it does **not** hold the posture: a leg whose invariant can never be asserted
would keep a correct system below trading for ever, which is the
halts-itself-with-every-light-green failure `ContributesPossibleDelta` exists to
avoid, arriving from the other end. What IS asserted is stated exactly: *every
unit of the venue's position is attributable to an order this system journaled*,
per derivative instrument in the enumerated scopes.

**The cross-check is what keeps the assertion from being a tautology.** A
recomputation from our own orders alone would agree with itself; this one joins
the ORDER surface's stated cumulative to the EXECUTIONS sweep's rows and to the
POSITION enumeration — three venue surfaces, two of which can disagree with our
ledger and with each other. An order whose totals do not match is reported as
`Unattributed`: an execution exists that cannot be named, and the exposure is
real whether or not we can name its row.

## §56h — FINDING 2: the outcome vocabulary's second empty cell

F1's finding was that `boot.Outcome`'s members are points in a PRODUCT — how
restrictive, and whether a re-run lifts it — rather than rungs of a ladder, and
it filled the halted row's empty cell (`OutcomeEvidenceUnusable`). It did not
tabulate the rest, and the stop-opening row was in exactly the state the halted
row had just been rescued from:

	                  freely re-reportable        latching
	halted            OutcomeFailed               OutcomeEvidenceUnusable  (F1)
	reconcile-only    OutcomeUnreported/Pending   OutcomeNeedsOperator
	stop-opening      OutcomeBlocksOpening        OutcomeProjectionStale   (F2)

§6 step 7's rule is what falls in the empty cell — a dispatch that reached the
venue and whose projection write failed *"HOLDS NEW TRADING until that order's
state is re-derived from the journal"* — and both halves are load-bearing in
opposite directions. **Not halted**: the order may be executing right now, so the
safety verbs must stay available; halting a process whose venue is filling an
order it cannot see is worse than the stale reading. **Latching**: the check that
would lift it is the net-delta assertion, which recomputes from the projection
that is known stale — and the stale reading is the PERMISSIVE one, since the
order sits at `Planned`, the single non-terminal state contributing no
possible-delta interval. A freely re-reportable outcome would be lifted by an
invariant agreeing with its own corrupted input, with nobody having looked.

Its discharge is the re-derivation §6 step 7 names — a journal replay, which in
this architecture is a fresh process with a fresh checklist — so inside a running
process only `Clear` lifts it, and the latch does not have to model the
mechanical discharge at all.

**General form, and it is the transferable half:** an outcome vocabulary is a
PRODUCT of the properties it decides, and a missing member is found by tabulating
it rather than by waiting for a producer. Waiting found these two one slice
apart; the table finds them together.

## §56i — the battery: 31 mutations, 30 suite kills, 1 gate kill, one survivor that changed the code

The collapse checklist ran before the tests were written and shaped every
fixture: positions are a SHORT and a LONG rather than two of a kind; two declared
symbols, so "this row's instrument" and "the first row's" differ; one order
attributed by client id and one by venue id **in the same sweep**, because
Binance's rows carry no client id; a row belonging to a third venue order, so
"attributes what is ours" and "attributes everything in the window" are different
answers; the join run TWICE, because a dedupe claim is unobservable inside the
boot that admits the fills.

**The real survivor is the coin-flip shape.** Deleting the product comparison
from the position lookup — so a perp row could match the spot instrument with the
same symbol, which every cash-and-carry deployment declares — was caught only
when Go's randomized map iteration happened to visit spot first. A test that
fails half the time is not a flaky test; it is a **coin flip in the code**, and
the recorded answer (200 trials on a `select`) is the wrong one here. The lookup
now refuses a SECOND match, which is also a real configuration check: an
unvalidated `ContractType` splits one real instrument across two map keys
carrying different base assets, which `domain.FillKey` records as a hazard on the
dedupe identity. Three trials confirm the mutation is now deterministic.

**The one gate-covered mutation** is the recorded trailing-return trap: deleting
`OutcomeProjectionStale` from `latching()`'s true case leaves it falling through
to the unrecognised-outcome arm, which returns true — behaviourally equivalent,
suite-invisible, and refused by `exhaustiveswitch`. Verified by running `make
lint` against the mutant with a clean positive control rather than asserted.

Two `NOBUILD!!` verdicts were bad seds (*"declared and not used"* — the mutation
orphaned a value's only reader), re-aimed, and both then killed.

## §56j — what is NOT in this slice, stated so the absence stays visible

- **`FundingLedger`.** §7.5 lists it among the join's surfaces. It settles in the
  quote asset on both venues' linear perps, so it moves no position and can
  explain no delta, and no consumer for it exists — a port with no use case is a
  shape frozen against nothing. It lands with §8's funding accrual.
- **The boot sequence that reports these outcomes.** Nothing in this repo
  constructs a `boot.Checklist` yet, so `Join`'s report and `Outcome.HoldsNewTrading`
  are consumed by no composition root — the same standing `application.ChainBroken`
  has had since F1, and the same slice's work.
- **The spot leg's baseline.** Task raised at the close.
- **Durability of the admitted fills.** They live in the in-memory projection and
  are re-derived on the next boot. That is the property that let the transaction
  requirement go, and it is also the limit: a process that admits fills and then
  crashes re-reads them from the venue rather than from the log.
