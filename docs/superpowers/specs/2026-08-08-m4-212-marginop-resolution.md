# Record §81 — M4 slice B3 part 1: the durable `MarginOp`, its resolution at boot, and the outcome that never latched

**Date:** 2026-08-08 · **Task:** #212 · **Docs:** v0.9.62 (owed) ·
`SPEC EDIT OWED` grep zero at close.

Read `docs/build-spec.md` §7.1's *"every margin op is a durable target-state
machine"* bullet and §7.5's two paragraphs on preconditions (the roster and the
self-clearing/latching split) for the normative form of everything below.

---

## 0. What this slice is, and what it deliberately is not

§7.1 gives the margin top-up the **§5.3 crash-boundary discipline**: journal +
fsync a `MarginOp` record *before* the call, and resolve an ambiguous result or a
crash mid-op by **evidence reconciliation**, never a blind delta retry. Two
halves, and this slice is the second one:

| half | what it is | where |
|---|---|---|
| **dispatch** | read the before-state, journal it, `AddIsolatedMargin`, serialize one op per position | **part 2** |
| **resolution** | find the in-flight op at boot, conclude by evidence, record the verdict, report the gate | **here** |

Resolution first, and the order is forced rather than preferred. The dispatch
half is a **non-idempotent external write with no venue-side operation
identity**; landing it before anything can resolve its ambiguous outcome would
create exactly the hazard §7.1 spends a paragraph eliminating. Landing
resolution first means that when the dispatcher arrives, its crash recovery
already exists and is already tested.

The consequence to state plainly: **no production path writes a `MarginOp` until
part 2**, so at boot this phase finds nothing and passes. Every fixture below
writes its records through `ports.MarginOpJournal` directly. A phase that passes
because there was nothing to do and a phase that passes because it works are the
same green, which is why the fixtures are the evidence and the clean run is not.

---

## 1. The finding: `OutcomeBlocksOpening` has no producer, and the placement it
## was given is the one its own doc argues against

`boot.OutcomeBlocksOpening` is referenced by tests and by nothing else. Its
entire doc comment names exactly one intended producer:

> `OutcomeBlocksOpening` is the gate refusing new exposure while leaving safety
> execution available — §7.5's Ambiguous `MarginOp`, which blocks openings
> *"until the staged-float ledger is explicitly reconciled"*.

This slice is that producer, and it arrives disagreeing with where the member
sits. `Outcome.latching()` places it in the **freely re-reportable** column, so
a later `Report(GateMarginOpsResolved, OutcomePassed)` lifts it with nobody
having looked — while the clause quoted in its own doc says the only thing that
lifts it is an explicit reconciliation.

The confusion is traceable, and it is §7.5 sorting on the wrong scope. §7.5
lists the preconditions that clear themselves and names this one among them:

> Some are **self-clearing**: a journaled `MarginOp` resolves through its
> evidence contract […] — a fresh report lifts it.

That sentence is about the **gate** and it is true: an op that was in flight at
the crash resolves itself, and the next attempt legitimately passes. The
Ambiguous **outcome** is the exception §7.1 states one section earlier, and the
property belongs to the outcome, not to the gate — which is the exact
distinction `Outcome`'s own type doc was written to make. So the member is
right, the doc is right, and the predicate is wrong.

`[[hazard-named-for-one-input]]`, one layer out: the placement was decided for
the gate's ordinary case and inherited by the outcome that is its exception.

### 1a. Three independent reasons it must latch, and only the third is about probability

1. **Both normative sentences say so.** §7.1: on `Ambiguous` "the staged-float
   ledger is reconciled by explicit accounting / operator action, **a §9 case**".
   §7.5: it blocks openings "**until** the staged-float ledger is explicitly
   reconciled". Neither is a description of a check that re-runs.

2. **One gate cell, many ops.** The checklist holds one outcome per gate, not
   one per op. Non-latching, the next reporter's `OutcomePassed` clears an
   earlier ambiguity — and part 2's live path resolves ops one at a time, so
   that reporter exists by construction. The boot phase here folds and reports
   once, so this is latent in part 1 and live in part 2; the member's semantics
   have to be right at the moment its first producer commits to them, because
   nothing later re-derives them.

3. **Re-reading the evidence is not more informative, it is more confounded.**
   The evidence is a delta against a journaled before-state (§3 below). Every
   later read includes every later mutation of the position, so a second
   resolution is strictly worse-positioned than the first. A freely
   re-reportable outcome invites precisely the re-run that cannot help.

### 1b. What it costs today, which is a wrong exit code rather than a wrong trade

`app/reconcile`'s `ExitClass` sorts `GateMarginOpsResolved` into the arm named
*"the four with no producer anywhere"*, which falls through to **`ExitClean`**.
So before this slice an Ambiguous margin op — a §9 accounting incident — would
have exited `arblab reconcile` clean. Giving the gate a producer is what that
arm's own comment instructs to act on (*"a gate added to §7.5's roster has to
choose a side here"*), and #198 item 7 is the precedent: `GateAccountPosture`
crossed the same way.

With the latch, an ambiguity reaches `Checklist.Latched()` → `ExitOperator`.
Without it, the best available is `ExitIncomplete`, whose meaning is *"nothing is
known to be wrong; a retry may lift it"* — the wrong advice for an incident
whose remedy is an accounting reconciliation.

### 1c. The cell the member vacates, and why nothing is put in it

Moving `OutcomeBlocksOpening` into the latching column empties the
**stop-opening / freely re-reportable / verdict-bearing** cell of the product
`Outcome`'s doc tabulates. Nothing is invented to fill it. The cell was never
occupied: the member that appeared to occupy it had no producer, and the one
producer it was declared for belongs in the neighbouring cell. An empty cell
with no condition asking for it is the table doing its job; the table exists to
be re-checked when a condition arrives, which is what happened here.

The two members now sharing the latching column (`OutcomeBlocksOpening`,
`OutcomeProjectionStale`) are checked for the missing-axis tell the type doc
names, and there is none: inside a running process the only lift for either is
`Clear`, and across processes both are re-derived by a fresh boot. They differ
in **cause**, which is what the GATE already names. Filed as an observation
rather than a change: the two other latching members are likewise named for
their first producer's cause, and the next second-producer to reach either will
have this same argument to make.

---

## 2. The record, and the one field that is a verdict rather than an input

§7.1 names the record's contents: *"local op ID, intended delta, **before-state
snapshot + venue watermark**"*. Four fields, and this slice adds a fifth that the
section does not name.

```
MarginOp {
    ID           MarginOpID      // local, minted by the dispatcher (part 2)
    Instrument   VenueInstrument // what to re-read
    Delta        Money           // SIGNED: positive adds, negative reduces
    BeforePosted Money           // the position's posted isolated margin before the call
    Watermark    time.Time       // VENUE server time at which BeforePosted was read
}
```

**The delta is signed rather than an enum beside a magnitude.** Bybit's API is
already this shape (`margin` positive adds, negative reduces) and Binance's
`type` 1/2 is a rendering of the sign, so a `MarginOpKind` would be a second
home for a fact the amount already carries — and, with only the add direction
having a caller, a one-member enum whose fixtures all collapse
(`[[fixture-count-of-one-collapses]]`). A **zero** delta is refused: an op that
moves nothing is not an op, and it is the one delta under which *Applied* and
*NotApplied* are indistinguishable by any evidence either venue publishes.

**The watermark is the VENUE's clock, not ours.** It anchors the evidence window,
and the rows the window will eventually be matched against (part 2's Binance
half) are timestamped by the venue. A window opened on the local clock and
closed against venue-stamped rows is off by the calibration offset and reads as
correct — `[[recomputed-check-must-share-the-anchor]]`. `clock.ServerNow(venue)`
is the source, and an uncalibrated venue is refused at construction for the
reason `LiqMonitor` refuses one.

### 2a. The verdict is journaled, and re-deriving it at the next boot is the defect

`MarginOpStatus` is written to the log as its own record the moment it is first
concluded. Not because a second boot would be slow, but because **re-deriving it
is not idempotent and drifts in the unsafe direction**:

- **A restart would launder the latch.** An ambiguity that blocks openings
  "until explicitly reconciled" is a fact about an op. Held only in a checklist,
  it dies with the process — and the next boot re-derives from a delta that has
  moved further, so it may well conclude *Applied* and pass. Explicit
  reconciliation would be satisfied by rebooting.
- **Binance's evidence expires.** `positionMargin/history` is a windowed query
  over rows the venue retains for a bounded time (Profiles §4). An op resolved
  today by a matching row resolves tomorrow by its absence — *NotApplied*, which
  is the reading under which nothing happened and a top-up may be re-attempted.
  Age turns a correct *Applied* into its opposite.

This is `AppendGenerationSuperseded`'s argument at a second subject: a value
whose two wrong answers are wrong in different directions, which nothing
downstream re-derives, is **stored**. It is not `[[stored-form-that-contains-the-answer]]`
— that hazard is a checker fed its own output, and here the checker (the next
boot) is deliberately forbidden from re-running.

---

## 3. The evidence contract as this part implements it, and the half that is deferred

The evidence for both venues in part 1 is the **posted-margin delta**: the live
`PostedMargin` from the already-built `ports.VenueMarginStatus` (#208), against
the journaled `BeforePosted`.

| observation | verdict |
|---|---|
| `live − before == delta` | `Applied` |
| `live − before == 0` | `NotApplied` |
| anything else | `Ambiguous` |
| watermark newer than the propagation bound | `Pending` |
| the margin read failed | *no verdict* — the gate reports `OutcomeFailed` |
| the venue reports the position **flat** | `Ambiguous` |

**Why the posted figure and not the margin balance.** #208 settled this from the
other direction and the pairing is the same: `isolatedWallet` / `positionIM`
exclude unrealized PnL, so they do not move with price. That is what makes them
evidence at all — a figure that moved with the mark would be confounded by
exactly the rally that provoked the op.

**What still confounds it, stated rather than assumed away:** a **fill** on the
position moves the posted margin too. So a partial close inside the crash window
produces `Ambiguous`, correctly and by design — §7.1's own list of ambiguity
causes names "position mutation" second.

**A flat position is `Ambiguous`, not `NotApplied`.** The position the op was
posted against no longer exists, so there is no before/after to difference: the
margin was returned to the wallet whether or not the top-up landed first. This
is the one row where the conservative reading and the informative reading agree,
and it is worth naming because the tempting answer — *the position is gone, so
nothing is owed* — is a float-accounting claim, and the float is exactly what
§7.1 says must be reconciled explicitly.

### 3a. `Pending` is §5.3's propagation bound, reused rather than re-invented

`Pending` is the status for an op whose dispatch is too recent for silence to be
evidence — the venue may still apply it. That is SM §2 step 1's rule, already
implemented as `ResolutionPolicy.MaxLanding` for orders, derived from the same
transport (the signed client's timeout, and `recvWindow` beneath it, Profiles
§6). A second bound with its own name would be a second assumption about one
transport, free to drift.

It maps to `boot.OutcomeIncomplete`, whose doc describes this exact producer
shape one machine over: the check ran, concluded, and the only thing wrong is
that not enough time has passed — *"reporting it as `OutcomeFailed` halts a
process whose venue is healthy; reporting it as `OutcomeNeedsOperator` demands a
human for a timer."* It is freely re-reportable, which is right and is §7.5's
self-clearing sentence getting its outcome.

**A `Pending` verdict is NOT journaled.** It is not a verdict — it is the absence
of one, and writing it would put a value in the log that the next boot must
ignore.

### 3b. Binance's second evidence half is deferred, with the cost named

§7.1 gives Binance two evidence sources: the posted delta **and**
`GET /fapi/v1/positionMargin/history` matched on symbol + type + amount +
time-window. Only the first is built here. Two consequences, both stated:

- **Binance concludes `Ambiguous` where it could conclude `Applied`.** Any
  coincident position mutation in the crash window becomes an operator incident
  instead of a resolved op. That is conservative in the safety direction and
  expensive in the operator direction, and it is a narrowing, not a reading.
- **It is unreachable during its own lifetime.** No code writes a `MarginOp`
  until part 2, so no Binance op can exist to be over-Ambiguated. Part 2 lands
  the history half as its first item, before the dispatcher that could produce
  one.

`[[a-narrowing-carries-an-argument]]`: the argument here is slice size, and it is
recorded as that rather than dressed as a design ruling.

**When the history half lands, "plus the position-risk margin delta" must not be
read as a conjunction.** The two sources have different confounds: a history row
records an **act**, and the posted delta is a **live quantity every fill moves**.
Requiring them to agree makes an ordinary partial fill into an operator incident
— the veto arriving from the more-confounded source. The reading that gives both
a job: the history row decides when it speaks (one row ⇒ `Applied`, two or more
⇒ `Ambiguous`, §7.1's named duplicate case), and the delta decides only when the
history is **silent** (no row and the margin moved ⇒ `Ambiguous`, not
`NotApplied`).

---

## 4. Where things live

| where | what |
|---|---|
| `oms/domain` | `MarginOpID`, `MarginOp`, `MarginOpStatus`, and the verdict rule as a pure function over (delta, before, live, held) |
| `oms/application/ports` | `MarginOpJournal` — a **third** port on the one §5.1 chain, subject = `MarginOpID` |
| `oms/infrastructure/driven` | the journal half (two tables, two record kinds) |
| `oms/application` | `MarginOpResolver` (the use case) + `Recovery.InFlightMarginOps` + the `Reconciler` phase |
| `platform/boot` | `OutcomeBlocksOpening` latches |
| `app/reconcile` | `GateMarginOpsResolved` crosses into the produced arm of `ExitClass` |

**A third PORT, one STORE, one chain** — `ports.GenerationJournal`'s argument,
unchanged and load-bearing for the same reason. A `MarginOpID` is not a
`ClientOrderID` and not a `GenerationID`; the three id spaces share no namespace,
so widening either existing key would put "a string that is sometimes an order,
sometimes a generation and sometimes a margin op" into a primary key. The single
chain buys the ordering fact: a resolution record provably follows its dispatch
record, checked by the same walk that checks everything else.

---

## 5. Rejected alternatives

1. **Dispatch first, resolution second.** Refused in §0: it ships the
   non-idempotent write before anything can resolve its ambiguous outcome.
2. **Report `Ambiguous` as `OutcomeNeedsOperator`.** It latches, which is right,
   but its posture is *reconcile-only* — more restrictive than §7.1 asks for.
   §7.1 is explicit that safety actions continue ("not safety actions, which read
   live state"), and reconcile-only would take the ladder away during the
   incident that needs it.
3. **Add a new member for the margin ambiguity beside `OutcomeProjectionStale`.**
   The two would share every property the type decides on, differing only in
   `String()` — and the cause is what the GATE names. `Outcome`'s own doc:
   members are points in a product, so a second member in an occupied cell is a
   name, not a state.
4. **Keep the verdict out of the log and re-derive it each boot.** §2a: it
   launders the latch and inverts with age.
5. **Resolve a flat position as `NotApplied`.** §3: it is a float-accounting
   claim wearing an evidence answer.
