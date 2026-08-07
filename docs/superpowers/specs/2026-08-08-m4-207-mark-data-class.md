# Record §78 — the MARK is §3.4's third data class, and the OMS-owned mark-risk read port

**Task #207 (part 1 of 2). 2026-08-08 (session 122). Docs v0.9.59.**

`Deleverer.Open` is built, journaled, reinstated at boot and filter-conformed,
and **no production code calls it**. Core §7.1's escalation chain is what should:
liq-distance breach → margin top-up → `PackageDelever`. A survey found the whole
chain unbuilt, and its FIRST INPUT absent from the system entirely.

| §7.1 link | State before this slice |
|---|---|
| the mark | **decoded nowhere.** `venue.BinanceMarkPrice` reads `e/E/s/r/T` and is documented as *"the funding-relevant half"* — `p` is dropped |
| liq distance / §5.2's policy | no code; `config/reload.go` names `risk.liq_distance.rebalance_pct` in a doc comment as an example path |
| live liq price / margin | `AccountSource.MarginStatus` is declared in §4 and implemented nowhere |
| the mark-risk read port (§4 line 553) | `engine/state` had `StrategyReader` only — `Funding` and `TopOfBook` |
| `MarginOp` / the top-up | nothing; `boot.GateMarginOpsResolved` is one of the four gates `reconcile/app.go` names as *"no producer anywhere"* |

This slice delivers the **mark** end to end and the **read port**. It is part 1
of #207; part 2 is the account half, and §5 below corrects what part 2 actually
is.

---

## 1. The mark is a DATA CLASS, not a field on funding — two independent reasons

On **both** venues the mark rides the same message as the funding rate: Binance's
`<sym>@markPrice` states `p` beside `r`; Bybit's `tickers.<sym>` states
`markPrice` beside `fundingRate`. One struct is therefore the obvious move, and
it is wrong. Either reason alone settles it.

**(a) One struct is one Budget.** §5.2 configures freshness budgets *"one per
(data class, venue) — book, mark and funding"*. Sharing an observation means
sharing a `domain.Budget`, so the mark inherits funding's tolerance — which is
the failure `ports.BookBudgets` already names from the book's side: a budget
written for metadata that moves every few hours, applied to a number a decision
is priced from. Here the decision is §7.1's, and the mark is what its ladder
escalates on.

**(b) Bybit's funding decoder DISCARDS exactly the frames the mark needs.**
`BybitTickerState.Apply` emits *"exactly when the message is a snapshot or a
delta carrying at least one funding field"*, because most deltas are bid/ask
churn and *"emitting ~10/s identical readings would round-trip the full two-hop
dispatch for nothing"*. That rule is right for funding and **inverted** for the
mark: those ~10/s deltas ARE the mark's news. A mark riding the funding emission
gate refreshes about once a minute — which is §7.1's own stated defect reached
one class over, *"a trigger frozen behind a slower event's cadence"*, and §7.1
says what it costs: *"a pure rally would otherwise go undetected until
liquidation."*

Reason (b) was not visible from the spec and would not have been guessed. It is
recorded because it is the argument that survives if §5.2's budget line is ever
re-read as advisory.

## 2. The Bybit mark decoder is STATELESS, and the first attempt was not

The first implementation folded the mark into `BybitTickerState`: a `hasMark`
field, per-class news flags, and `NewBybitTickerState(base, quote)` for the asset
tags a price needs. It compiled and the design was wrong, for a reason the
compiler surfaced as ripple: **nine call sites** in `replay`, `crossvenue` and
`sim` — every one of them a funding consumer that will never read a mark — had
to supply asset tags for a class they do not consume.

The accumulator exists because **funding is four fields arriving at different
times**, so *"a stateless decoder therefore cannot exist — the accumulated
last-known state IS the reading"*. A mark is **one field**, and one field is a
complete reading. There is nothing to carry across frames, no snapshot/delta
distinction that changes the answer, and no cold start to wait out.

`DecodeBybitMark` is therefore a pure function, which also makes the two venues
symmetric: both marks are now one function over one frame taking the assets the
price needs. **The asymmetry I had built was an artifact of which struct I
reached for first, not of the venues.**

Two consequences are stated in code because they look like omissions:

- **A delta needs no preceding snapshot.** `BybitTickerState` refuses to emit
  before a snapshot because a partial accumulation is a wrong reading. Nothing
  is accumulated here, so a mid-stream delta's `markPrice` is a complete mark,
  and waiting for a snapshot would blind §7.1's trigger for the whole window
  after a reconnect — on a rule protecting a property this class does not have.
- **Absence splits on the frame's own claim.** A mark-less DELTA is silence (the
  ordinary majority of the stream); a mark-less SNAPSHOT is `ErrWireIncomplete`,
  because a snapshot states full ticker state. Both halves are asserted in one
  test, because either alone passes against an implementation that treats them
  the same in whichever direction.

## 3. `p` vs `P` — the finding, and it is this package's SECOND instance

`DecodeBinanceMark` read **64001.0** where the frame said **64000.5**. Binance's
markPrice frame states both `p` (mark) and `P` (estimated settlement price).
`encoding/json` prefers an exact tag match, but a key with **no declared field**
falls back to **case-insensitive** matching — so a struct declaring only `p` lets
`P` compete for the same field, and **which one wins depends on key order in the
document**.

The failure mode is the dangerous kind. An estimated settlement price is a
plausible mark, a dollar off the real one. `Mark.Validate` passes, the liq
distance computes, and the trigger is simply wrong. Nothing downstream can catch
it.

**This package had already paid for it once.** `binanceFuturesDepthWire.FirstID`
declares `U` for exactly this reason, *"found by a survived mutation, session
52"* — and that note is attached to **the struct that had the defect**, so it
said nothing to the next single-letter Binance wire written, two files away.
[[hazard-named-for-one-input]]: the note covered the input, not the seam. Both
comments now cross-reference, and the rule is stated for the SEAM — *any struct
here with single-letter tags must declare every key of the frame that differs
from a declared one only by case, whether or not anything reads it.*

The regression test asserts **both key orders**, because one order alone passes
against the broken struct whenever the real key happens to come last.

## 4. Where the read port lives — TWO gates moved it, and the second improved it

§4 line 553 requires *"an OMS-owned mark-risk read port over `engine/state` (not
by importing `marketdata`; the read port keeps the §3.6 hexagonal boundary)"*.
Three placements were tried; the first two were refused by gates, and each
refusal was correct.

**Attempt 1 — `engine/state`, beside `StrategyReader`.** This is the documented
pattern: strategy's own ports doc says adapters live in `infrastructure/driven`
*"or, for the read seam, in the engine subsystem that holds the data"*.
`singlewriter` refused it. Rule 6 closes `oms/application/ports` to every package
outside `oms`, **port as well as adapter**, *"because depending on the
abstraction is the recommended way to depend on anything"*.

**The pattern does not transfer from `strategy` to `oms`, and rule 6 is why.**
The gate is right even though `MarkRiskReader` carries no write authority,
because the rule is about the PACKAGE: an exemption reading *"this one is only a
read port"* would have to be re-judged for every member added afterwards, by
whoever added it. **Rejected: loosening `singlewriter`.**

**Attempt 2 — `oms/infrastructure/driven`, beside the journal and gateway
adapters.** `app/reconcile`'s import fence refused it: this adapter's backing
store is `engine/state`, whose graph reaches `marketdata` and `strategy`, and the
§7.5 boot root imports `driven/` for adapters it genuinely needs. **One file was
enough to pull two forbidden contexts into a boot binary transitively, with every
individual import defensible.** The fence's own comment names the shape —
*"a composition root is exactly where such an edge arrives most naturally: while
we are here, read the mark off marketdata so the report can price the delta"* —
and it caught it one hop out, through a package nobody was thinking about the
graph of.

**Landed — `oms/infrastructure/markstate`, its own package.** A PACKAGE is the
unit of dependency. The split is honest rather than a workaround, and the test is
what each adapter is over: `driven/`'s are over durable stores and venue
surfaces, which a boot must reach; this one is over the LIVE market-state
pipeline, which by §7.5's own argument does not exist yet at that point.

### 4a. The fence then improved the port itself

One edge survived the move: `oms/application/ports` imported
`marketdata/domain` for `Watermark`, and the §7.5 boot root imports that ports
package. **Rejected: adding `marketdata/domain` to `permittedByGate`** — that
exemption is *"keyed to its REASON"* (risk produces a declared boot gate), and
"it is only a value type" is a predicate that would admit `strategy/domain` next
([[an-exemption-argued-from-one-cause]]).

Being forced off the watermark produced a **better seam**. The port now returns
**ages** and takes the instants to measure them against:

```go
Mark(vi ids.VenueInstrument, now, serverNow time.Time) (MarkReading, bool, error)
```

Three things follow, and the third is the one worth having:

1. The age arithmetic — two clocks, and the clock-inversion rule that makes a
   negative age `FreshnessUnknown` rather than fresh — stays **single-sourced**
   in `marketdata/domain.Watermark`, computed by the adapter, which may import
   it. A watermark-carrying port would have invited every consumer to redo it,
   and the half a reader gets wrong is that the DATA age is measured against
   `ServerNow(venue)` and never `Now`.
2. There is no second definition of "when this arrived" to drift.
3. **"The age is derived at the READ against an instant the caller supplies" is
   now enforced by the SIGNATURE rather than requested by a comment.** There is
   no verdict that could be returned stale, and no reading whose age could be
   computed later against a different clock read than the one the call used.
   `engine/state`'s cache doc argues at length that a STORED verdict reads fresh
   for an entire outage; this is that argument made structural.

## 5. What part 2 is — a correction to #207's own description

#207 was filed saying the account half means *"un-narrow the position codec for
`liquidationPrice` / `isolatedMargin` / `maintMargin`"*, on the finding that
`binancePositionWire`'s narrowing comment (*"every other field is a number this
system would then have two sources for"*) is scoped to §7.5's net-delta consumer
and expires against §7.1's.

**That framing is wrong, and the spec already says so.** §4 line 331 declares
`AccountSource.MarginStatus(VenueInstrument) (MarginStatus, error)` — *"available
margin, maintenance, live liq price"* — as its own method. So the account half is
**implementing a declared port**, not widening the position codec. On Binance it
happens to read the same endpoint, which makes it the same shape this slice just
built for the mark: **two codecs over one venue answer, each projecting its own
consumer's fields**, rather than one struct widened until two consumers share it.

`binancePositionWire`'s narrowing therefore **stands**, and its reason does not
expire: §7.5's net-delta invariant genuinely should not acquire a second source
for those numbers. What expires is the assumption that *no* consumer needs them,
and the spec had already answered that with a separate port.

This is [[unimplemented-clause-check-its-traceability-tag]] arriving through a
task description rather than a spec clause: a gap whose owner is already named
elsewhere, where opening the register first would have saved the wrong framing.

## 6. Rejected alternatives

| Rejected | Why |
|---|---|
| mark as a field on `FundingObservation` / `venue.Funding` | §1 — one struct is one budget, and Bybit's emission rule discards the mark's news |
| mark folded into `BybitTickerState` with per-class news flags | §2 — nine funding-only call sites forced to supply asset tags; the mark has no state to accumulate |
| `Apply` keeping one `emitted` bool with a second flag beside it | with two classes the bool must mean "either", and every existing site reads it as "funding" — the reading that silently became wrong is the one nothing flags |
| `MarkSnapshot` carrying `GapDetected` / `SeqAvailable` | they could only ever be `(false, false)`: Binance sends no sequence, and Bybit's `cs` counts TICKER MESSAGES, so between two mark readings it skips by however many churn deltas fell between — a permanent gap in a healthy feed. A field pinned to a constant reads as an assurance |
| loosening `singlewriter` for a read-only port | §4 — the rule is about the package; a per-member exemption stops the gate biting |
| `marketdata/domain` added to `app/reconcile`'s `permittedByGate` | §4a — the exemption is keyed to producing a boot gate; "only a value type" admits `strategy/domain` next |
| `MarkRiskReader` returning `mdomain.MarkObservation` verbatim | the fence, plus: strategy PRICES against an observation and wants everything it says; §7.1's monitor wants one number and its age, and every extra field is one a liq computation could reach for |
| widening `binancePositionWire` for the liq price | §5 — §4 already declares `AccountSource.MarginStatus` as a separate port |
| wiring the adapter into `oms.Module` now | it has no consumer until B2's liq-distance monitor; a root wiring it today would drag `engine/state` into every OMS binary for nothing. The deferral names its trigger: **the first `MarkRiskReader` consumer** |

## 7. The campaign

**16 mutants, all caught** — after one real survivor and two re-aimed
`NOBUILD!!`s.

- **`SURVIVED` → fixed:** `Len` reporting the funding count as the mark count.
  Every fixture had the two counts equal or the mark discarded, so
  `len(c.funding)` substituted for `len(c.marks)` passed the whole suite. The
  test's own comment already said *"ASYMMETRIC counts on purpose"* — widening it
  to three classes is what made that a three-way requirement, and the third arm
  was the one nobody varied. Now `(1, 3, 2)`, all distinct and non-zero.
- **Two `NOBUILD!!` re-aimed, both "declared and not used"** — the bad-sed case,
  not the compiler-enforced-claim case ([[nobuild-verdict-read-the-compiler-error]]).
  Both became TRANSPOSITIONS (`Transport`↔`Data`, `Age`↔`DataAge`), which keep
  every variable used and are the realistic mistake anyway.
- Isolated to exactly one test each: the mark/funding **timer-ID collision**, the
  mark-less **snapshot** refusal, the **two-age transposition** in the read port,
  and **absence reported as a reading**.

Two fixture-collapse catches, both from the checklist rather than from a failure:

- The `spot instrument` case mutated only `Product`, and
  `ids.VenueInstrument.Validate` refuses a spot carrying a contract type one line
  earlier — so it asserted a rule this file does not own while reading as though
  it covered `ErrMarkOnSpot`. The fixture did not contain the thing under test.
- `markstate`'s three instants have pairwise-distinct GAPS (3 s and 11 s), not
  just distinct values: with equal gaps, an implementation measuring both ages
  against one clock is indistinguishable from a correct one.
