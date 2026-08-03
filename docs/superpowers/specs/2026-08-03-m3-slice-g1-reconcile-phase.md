# Design record §57 — M3 slice G1: §7.5's reconcile-only boot becomes one use case

**Date:** 2026-08-03 · **Session:** 86 · **Docs:** v0.9.38 · **Task:** #166 (split)

Slice G was one row in §47f's plan — *"parallel-IOC + testnet chaos"* — carrying
a second thing HANDOFF had been naming for two slices: the boot sequence,
because `platform/boot` had **zero importers** outside itself and neither
`application.ChainBroken` (F1) nor `Outcome.HoldsNewTrading` (F2) had a consumer.
This slice is the first half of that second thing, and it turned out to contain a
defect that neither F1 nor F2 could have seen.

---

## §57a — the split, and the seam it is drawn on

`#166` becomes three:

| | Delivers | Why it is separable |
|---|---|---|
| **G1** (this) | §7.5's three phases composed into ONE use case, in order, reporting into a `boot.Checklist` a root owns | The ORDER is the normative content, and it needs no binary to be wrong |
| **G2** | The composition root: a `boot.Sequence` with §3.2's phases, the `arblab reconcile` subcommand §11 names, and §13's local-filesystem precondition (#168) | Needs a construct that does not exist — see §57i |
| **G3** | Two-leg parallel marketable-IOC + the process-kill-mid-IOC chaos test | Needs G2's process to kill |

The seam is the one `oms.Module` already draws. G1 is a **use case**, checked
against fakes; G2 is **wiring**, checked against a process; G3 is **behaviour
under a signal**, checked against a venue. Splitting elsewhere would have put the
ordering rule in a composition root's comment, where a second binary re-derives
it and the two differ.

## §57b — RULING: the checklist belongs to the ROOT, and this use case takes a pointer

§7.5's gate roster is eight and this context can establish three
(`GateJournalReplayed`, `GateVenueJoined`, `GateNetDeltaAsserted`). The other
five are risk's, `engine/venue`'s and §7.4's. Two shapes were available and both
were rejected before the pointer was taken:

- **A checklist built here over all eight.** The five this context cannot report
  would hold the posture at reconcile-only for ever, with no way for their real
  owners to report them — a correct system pinned below trading by its own
  wiring.
- **A checklist built here over the three it can report.** This is the dangerous
  one, and it is the shape that reads as tidy: a boot that checked three of eight
  preconditions and reported `PostureTrading`. §7.5 argues exactly that failure
  from the other side, about the fold's seed — *"nothing was checked" and
  "everything passed" must not be the same answer* — and a short roster produces
  it through the roster instead of through the arithmetic.

So the root declares the roster and this use case reports into it. The refusal
that follows is deliberate: `Reconcile` returns `boot.ErrGateUndeclared` if the
root's checklist is missing one of the three, rather than skipping it.

## §57c — RULING: a refusal is a REPORT, and `error` means "could not report"

`Reconcile` returns `(Reconciliation, error)` and the error result means only
that the verdict could not be filed — a nil checklist, an undeclared gate. Every
refusal a boot can legitimately reach is a value.

Two reasons, and the second is the one that decides it. First, `Dispatch`'s: a
refusal returned as an error is one `if err != nil { return }` away from a caller
that discards it and boots anyway. Second, `boot.Run` turns a phase error into an
**unwind and a dead process**, and `oms/module.go` had already recorded what that
costs — a journal that will not verify must leave a process that is alive,
halted, latched and audible to §7.4's watchdog, because exiting presents to that
watchdog exactly as a dead host and escalates to an actuator takeover. The
diagnosis rides on `Reconciliation.Cause`.

## §57d — FINDING 1: the invariant was asserted over a quantity the join DECLINED to rebuild

`Joiner.Join` reads each dispatched order. When the venue's *order* surface
answers "no such order" it does **not** sweep the order's executions, and the
reason is in its own doc:

> Handed to §5.3's rule rather than concluded here. An absence is the one answer
> this use case must not act on: it is the input to the four-gate
> stable-not-found contract, and reading it as "no executions" would be the
> inference §5.3 forbids in as many words.

The order therefore keeps the **zero cumulative** the replay gave it. And then,
in the same call, `assert` folded that zero into `expected` and compared it
against the venue's position enumeration — which is reading the absence as "no
executions", one function later, in the use case that had just refused to.

**The harm, stated as narrowly as it is true.** The venue's standing position is
the other side of the subtraction, so any fill such an order took reports as an
unexplained delta of its whole size. That is §7.5's *alert for operator
clearance*, which **latches** — `boot.OutcomeNeedsOperator`, liftable only by
`Clear`. The boot it fires on is the one this section exists for: a dispatch that
reached the venue and a crash seconds later, where §5.3's gate 1 refuses only
because the propagation bound has not elapsed and will pass on its own inside
`MaxLanding + SweepMargin`. **A self-clearing condition frozen into an operator
incident** is §7.4's *"never health-flap re-entry"* run backwards, and it costs
the same thing: a human in the loop of something no human can decide.

It is **not** a wrong-number defect and it is **not** unsafe in the exposure
direction — the error runs toward alerting, and the posture it produces withholds
trading correctly. Overstating it would be the mistake F1's own reading pass
caught in itself.

**Why four rounds of review passed over it.** Both halves are correct alone. The
join's refusal to infer is right; asserting the invariant before trading resumes
is right. What was wrong is that **nothing owned the order**, because §7.5's
three phases were three unwired use cases and the sequence existed in no code —
so there was no artifact for a reviewer to be wrong about. This is the repo's own
pattern arriving in Go: each formal model found its first defect on a
*composition*, never on a machine reviewed alone.

**The fix is two changes, and one alone is insufficient.**

1. **Order.** `Joiner.Assert` is split out of `Join`, and `Reconciler` runs
   replay → join → §5.3's rule on every absence → assert. `Assert` re-derives
   `expected` from the projection rather than taking `Join`'s running total,
   because a total computed during the join is a total taken before the last
   writer finished.
2. **A gate.** The assertion runs **only where the rebuild concluded**. Ordering
   alone does not close it: §5.3's rule does **not** admit executions — an
   absence it refutes returns `OutcomeUnproved`/`ReasonExecutionsFound` and hands
   the rows back, because admitting them is the fill path's single transaction,
   which no port here declares. So after resolution the ledger can still be
   knowably short, and an invariant computed over a knowably short store is not a
   weaker check but a wrong one.

Where the assertion does not run, its gate stays **unreported** rather than
failed. Unreported is the honest value — the check never ran — and it is not the
permissive reading, because the posture folds over the **required** gates.

**The rejected order is a control that RUNS.** `TestAssertingOverAnUnrebuiltAbsenceLatches`
performs the fused sequence explicitly — join, then assert, with §5.3 left out —
and asserts that it reports `−0.5` and that the outcome it implies latches. If it
ever stops alerting, the fix is measuring nothing. That is the standing habit:
encode the rejected reading and let the runner answer, rather than arguing it.

## §57e — FINDING 2: the outcome vocabulary's THIRD empty cell

The fixed reading needs an outcome the enum did not have: *the check ran,
concluded, and left work a further attempt may finish*. Its producers are §7.5's
ordinary answers — a sweep that did not prove coverage through the instant it was
asked about, an absence whose finality timer has not elapsed.

- **Reconcile-only, not halted.** Halting forbids the reconciliation writes that
  finish the work — §7.5's own `DeactivationPending` re-drive among them — and
  the venue is healthy.
- **Not latching.** Demanding operator clearance for a timer is the
  halts-itself-with-every-light-green failure.

F2's own tabulation showed that cell as occupied:

```
                  freely re-reportable        latching
reconcile-only    OutcomeUnreported/Pending   OutcomeNeedsOperator
```

It was not. **Both of those members are PRE-VERDICT** — one says the check never
ran, the other that it is running — while every other row has a member meaning
*"it ran and it refuses"* (`OutcomeFailed` on the halted row,
`OutcomeBlocksOpening` on the stop-opening row). So the property the table was
missing is the one every other cell has without saying so: **whether the outcome
reports a result at all.**

> **The general rule, one turn past the one F2 earned:** a product is only as
> complete as the properties you tabulate it on, so when a cell is filled by a
> member that answers a **different question** from its row-mates, that is the
> tell that an axis is missing rather than that the cell is occupied.

**The harm of the collapse, stated precisely.** `OutcomePending` and the missing
member impose the same posture and the same clearance rule, so nothing about
*permission* changes — this is a diagnostic collapse, and the diagnosis is what
an operator acts on. `Pending` means WAIT; this means ACT. A boot that concluded
and refuses, reported as one still running, leaves a process that is alive,
audible and below stop-opening: it may not reduce exposure, nobody escalates
because the boot "has not finished", and §7.4's watchdog reads liveness so it
does not escalate either. That is the **alive-but-wedged** state
`SignalImmediateHalt`'s doc calls an *uncovered* failure — reached this time
through a vocabulary rather than through a shutdown.

`boot.OutcomeIncomplete` is the member. `OutcomePending` keeps its meaning and
keeps a producer, so the distinction is a distinction and not a rename.

## §57f — task #175 is the same finding's third face, and it belongs to the REASON

*"Refuse to trade an instrument whose absences could never be proved"* was an
open task with no obvious home. It has one: §5.3's `UnprovedReason` constants had
each **already committed in prose** to whether waiting could ever help —
*"this one resolves itself"*, *"it never resolves itself, and it gets strictly
worse with time"*, *"permanent, and fixed by configuration rather than by
waiting"* — and nothing read those sentences. `RetryMayLift` makes them a value:

| Reason | Retry may lift | Because |
|---|---|---|
| `ReasonHorizonNotPassed` | yes | a fixed instant the clock reaches |
| `ReasonSweepIncomplete` | yes | the next attempt gets further |
| `ReasonNoAnchor` | no | the anchor is what the journal holds, and the replay already supplied it |
| `ReasonReachExceeded` | no | retention only grows worse |
| `ReasonReachUnknown` | no | an unconfigured bound is configuration |
| `ReasonSweepInconclusive` | no | the same unattributable row next time |
| `ReasonExecutionsFound` | no | exposure waiting on the fill path's transaction, not on another query |

A method on the reason rather than a set the boot recognises, and it names every
member so `exhaustiveswitch` forces a later one to choose — because the
permissive default is the one that costs: a gate added next year and defaulting
to *"a retry may lift it"* would be reported as ordinary boot progress for ever.

## §57g — a gate is only ever reported about its OWN evidence

Two findings can hold at once with no ordering between them: a truncated sweep is
reconcile-only and lifts on a retry; a failed projection write is stop-opening
and **latches**. One gate carries one outcome, so reporting both against the same
gate must drop one — and whichever is dropped is wrong. Reporting the more
restrictive loses the latch (a later pass lifts a hold nobody looked at);
reporting the latching one understates the posture.

They are reported on **different gates** because they are about different
evidence: the venue rebuild against `GateVenueJoined`, the projection against
`GateNetDeltaAsserted`. The checklist's fold then combines them, which is what it
is for. This also discharges the recovery half of **#169**: a resolution whose
state was journaled and whose apply failed is §6 step 7's condition reached
through recovery rather than through dispatch, and it is reported
`OutcomeProjectionStale`.

A test found a real bug in the first version of this: reporting the stale gate
and then continuing to the assertion made the assertion's own `OutcomePending`
report hit a latched gate. The correction is not the report order — it is that a
stale projection **stops the assertion running at all**, because the assertion is
the check that computes *from* the projection, and §6 step 7's rule is that the
stale reading is the permissive one. An invariant agreeing with its own corrupted
input is §46 §8e arriving at the boot checklist.

## §57h — the battery, and the two survivors that were the recorded trap

**23 mutations, 23 kills.** Six first attempts returned `NO-OP!!` or
`NOBUILD!!` and every one was a bad sed rather than a claim — a duplicate `case`
in a switch, an orphaned `declared and not used`, a line-number prefix inside a
command the harness already scopes to a line, and two wrong line numbers. Each
matched the recorded discriminator, so each was re-aimed rather than written down
as a non-result.

**Two SURVIVED first and both were the same recorded trap.** Dropping
`OutcomeIncomplete` from `posture()`'s and `latching()`'s case lists falls through
to the trailing `return PostureHalted` / `return true`, so the new member's group
membership was untestable — *switch default hides its own group*, arriving on a
member added this session. The real cause was upstream: `platform/boot`'s own
suite had **no fixture for the new member at all**, because the posture table was
written before it existed. Both became kills once the table gained the member and
`TestIncompleteRefusesWITHOUTLatching` landed — which asserts **both** halves
deliberately, since the posture alone is already provided by `OutcomePending` and
the non-latching alone by `OutcomeFailed`, so a fixture checking either half
would pass against the wrong member.

**The collapse checklist ran first and paid immediately.** It found a **fake with
a second opinion**: `fakeOrders.ApplyPlanned` handed back an UNTAGGED zero
cumulative, where the real repository registers the order in the fill ledger and
the ledger's zero is tagged at `Register`. Invisible for four slices, because
nothing did arithmetic on a replayed-but-unfilled order's cumulative — which is
exactly what the net-delta assertion does to every one of them. The join tests
had papered over it by setting the field explicitly in their own planter, so the
replay→join path's real value had never been exercised.

The file's checklist is recorded in `reconcile_test.go`, and item 3 is the one
worth carrying: **the finding is about a SEQUENCE, and no value fixture can reach
it.** The absent order and the resolved order are the same order at two instants,
so a test inspecting only the end state passes against either ordering. What
separates them is the assertion's own answer, which is why the control runs the
rejected order explicitly.

## §57i — what is NOT in this slice, stated so the absence stays visible

- **The composition root and the `arblab reconcile` binary.** Nothing constructs
  a `boot.Checklist` in production yet, and this is the third slice to say so —
  which is why the deferral **names the construct**, per §3.6's rule, rather than
  the feature. What it waits on is a **`venue.OrderClient` built from
  configuration**: §5.2's config schema for the OMS does not exist, credential
  loading has no home outside `engine/venue`'s own package, and `arblab reconcile`
  against a real account is the first thing in this repo that needs both. That is
  a different subject from an ordering rule, and building it here would have put
  a credential path in the same session as a safety-kernel finding.
- **§13's local-filesystem-only precondition (#168).** §11 schedules it to §7.5's
  boot posture list rather than to the journal adapter's constructor. There is no
  boot posture phase until G2, and attaching it to the reconcile use case would
  put a host-storage check in the one call that must not know about hosts.
- **The remaining five gates.** `GateAccountPosture`, `GateDeactivationsResolved`,
  `GateMarginOpsResolved` and `GateWriterAuthority` have no producer anywhere.
  They are the root's to declare and other contexts' to report; a root declaring
  the full roster today gets a correct, honest reconcile-only posture.
- **Fill durability.** Unchanged from F2: admitted fills live in the in-memory
  projection and are re-derived on the next boot.

## §57j — rejected alternatives

**(i) Fix the ordering by making `Join` resolve absences itself.** Rejected: it
would put §5.3's four-gate contract inside the use case whose whole discipline is
that it must not conclude about an absence, and `Joiner` holds no journal, so a
resolution's state write would have nowhere to go. The ordering is a
composition's business, which is the finding's own lesson.

**(ii) Leave the assertion fused and simply exclude absent orders from
`expected`.** Rejected, and it is the tempting one because it is a two-line
change. It makes the invariant assert over a *silently narrowed* set: the venue's
position still contains the absent order's fills, so the same unexplained delta
appears with no order in scope to explain it, and the report would name an
instrument with no account of why. It also loses the reason — a reader a year
from now finds a filter with no clause behind it.

**(iii) Report a truncated rebuild as `OutcomeFailed` and skip the new member.**
Rejected on §7.5's own text: `OutcomeFailed` halts, and halting forbids the
reconciliation writes — the `DeactivationPending` re-drive above all — that are
how the work gets finished. It is also wrong about what happened: the venue
answered.

**(iv) Report it as `OutcomePending` and skip the new member.** Rejected, and
this is the one the record exists to keep settled, because it is a defensible
reading: the posture and the clearance rule are identical, so nothing about
permission changes. What changes is the operator's answer to *"is this boot still
working?"* — see §57e. A vocabulary that cannot distinguish WAIT from ACT hands
the wrong one to the person holding the exposure.
