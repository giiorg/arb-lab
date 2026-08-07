# Design record §76 — M4 task #204: the generation reinstatement, a gate that had to be its own, and the number that decides where recovery can run

**Session 120 · 2026-08-07 · docs v0.9.57**

Predecessor: `2026-08-07-m4-203-generation-journal-adapter.md` (§75), whose work
item 10 this is. §75e named the hold and §75h named the composition problem; this
record is what building the answer added, corrected and left open.

---

## §76a — What the task was

`ports.GenerationJournal` had a store (#203) and the replay reported
`Recovery.LiveGenerations` as a list of bare IDs. Every payload — the hedge group,
the reservation, the target, the authorized size, the leg ids, the supersession
path — reached the visitor and was dropped, and `Reconciler` turned a non-empty
list into `boot.OutcomeIncomplete`: a refusal that held the line and did no work.

Two things had to exist for the work to happen, and the task said so:

- **(a)** a place to put a rebuilt `domain.Generation`;
- **(b)** a way to re-enter its reservation into §6.7's **shared** ledger, which
  lives on the `Classifier` — while §75h refuses to hand the replaying side a
  `Classifier`, because that is "the ability to change the state it is reasoning
  about while it reasons".

Both are answered below. The finding that reorganised the slice is neither of
them.

---

## §76b — THE FINDING: the number recovery needs is in no journal, and the
## justification for that said the opposite

A generation's ledger entry carries its **outstanding net intent**, which before
the authorization is `perpDone` — the CONFIRMED PERP REDUCTION. So a
reinstatement cannot re-`Reserve` correctly without that quantity.

`Deleverer.RecordPerpFill` is the one ladder step with no generation record, and
its comment explained why:

> the quantity IS the perp order's own cumulative execution, which `ports.Journal`
> already made durable at order scope and which §7.5's replay already rebuilds

**Both halves are false.** `ports.Journal` has no `AppendFill` — it is "narrowed
to what the dispatch path writes" — so no fill is journaled at order scope
either; and `Replayer`'s own doc says in as many words that every record it
rebuilds carries a **zero** cumulative. The two comments contradicted each other
across two files and neither was challenged, because nothing had ever needed the
number.

The conclusion survives on a **different premise**: §7.5's join rebuilds executed
quantity from **venue truth**, so the number is recoverable without a second
record of it. And that replacement premise is what fixes the slice's shape —
the reinstatement can only run **after the join and after §5.3's absence
resolution**, which is a different phase of the boot from the replay.

Rebuilding earlier is not merely premature. A generation whose log holds an
authorization of `s` would be rebuilt against a confirmed perp reduction of
**zero**, and `CheckSpotSize` would refuse the very record the journal proves was
written. The ordering is forced by arithmetic, not chosen for tidiness.

*Transferable:* a justification for NOT storing something is a claim about where
else it can be got, and nobody re-derives it until something needs the value. The
tell here was two comments in one package asserting opposite things about the
same field.

---

## §76c — The gate had to be its own, and §75e's refusal MOVED rather than lifted

`GateJournalReplayed` reported `OutcomeIncomplete` for a live generation. The
reinstatement cannot run there, so the refusal could not simply be lifted in
place — the alternatives were to leave the gate `Pending` across three more
phases, or to split.

**`boot.GateGenerationsReinstated` is the split**, and `GateJournalDurable`'s own
argument is the precedent, one axis over: *"a gate is reported about its OWN
evidence"*. That gate separated the **filesystem beneath** the records from the
records; this one separates a body of evidence the replay does not have **yet**.

The posture is held identically in the meantime, and that is arithmetic rather
than a promise: the fold is over the **required** roster, so an unreported gate
refuses exactly as a failed one does. What the split buys is that a boot which
successfully reinstates its generations can now reach `Passed`, which the merged
gate could never have expressed.

**Placement in the sequence:** after `resolveAbsences`, after the stale-projection
check, after `joinOutcome == Passed`, before the net-delta assertion. Each of
those guards is load-bearing:

| Guard | Why the reinstatement must not run past it |
|---|---|
| `len(out.Stale) > 0` | The projection is known wrong, and the perp cumulative is read FROM it. |
| `joinOutcome != Passed` | An unconcluded join is exactly the state where the executed quantity is knowably SHORT — so entries would understate what the generations committed, and understating the subtrahend reads as more room to trade. |

It reports its gate and the sequence **continues** to the assertion, on §75e's
rule: each gate reports on its own evidence, and stopping leaves an operator
holding a refusal with no picture of the account behind it.

---

## §76d — The seam: a different SUBJECT, not a narrower Classifier

§75h's constraint is real and was not worked around. The way through is that what
recovery needs is not *"amend the ledger"* — it is *"this generation was live;
make the ladder true again"*, and the ladder is the one object that already holds
both the generation and the ledger and is **required to keep them in step**.

So `application.GenerationLadder` is three whole acts, each atomic over both
objects, and `*Deleverer` implements it:

```go
Restore(g *domain.Generation) error
BarLadder(group domain.HedgeGroup, path domain.SupersessionPath) error
Reassess(group domain.HedgeGroup) (Reassessment, error)
```

`Reinstater` holds that interface and `ports.OrderLookup` — a new READ-only port,
cut off `OrderWriteRepository` on the same argument `Journal`/`JournalReplay` and
`OrderPlacer`/`OrderResolver` are cut. It can reach no ledger, no journal and no
placer. The `Replayer` still holds no `Classifier`; the walk that decides what
records MEAN produces **values**, and a separate use case performs the
reinstatement from them.

**The reserve lives in `Restore`, not in the caller**, and that is why the method
exists rather than an exported `live` map. §7's *"exactly backed at every
instant"* is a claim about two objects; a reinstatement that installed the
aggregate and left the caller to reserve would move that invariant into a second
place — the shape FM-6 came out of.

---

## §76e — Rehydration drops the PREMISE and nothing else

`domain.RestoreGeneration` is `OpenGeneration` without the premise argument. The
division is not "boot is trusted":

- every other check is about the **record** — an id, a group, a reservation, a
  positive target — and a stored value that fails one is a log that verifies and
  does not compose. Those are shared through one constructor, so a rehydrated
  generation cannot be laxer about itself than a fresh one;
- the premise is about the **world at the instant of a decision** that has
  already been taken and journaled. Re-asking it judges a generation that EXISTS
  against a world that has moved.

**The failure direction settles it.** A recovered generation refused for a
dissolved premise is a generation whose ledger entry is never re-`Reserve`d —
precisely the FM-5-shaped hole this gate exists to hold. Refusing to rebuild and
discharging the supersession look alike from one step away and are opposites: one
leaves the exposure unaccounted, the other accounts for it and then ends the
attempt.

The premise binds again immediately, with more force, as FM-8's **standing**
obligation: `Reinstate` calls `Reassess` on every restored generation, so a
premise that dissolved during the outage fires the premise-lost row at boot.
Not reassessing would be model (a)'s NC23 reached through recovery — an
obligation made possible on the next event and guaranteed by nothing.

This is the same division §7.5 already makes for ORDERS: `ApplyPlanned` at boot
does not re-run the planner's admission checks, because those judged a decision
that was taken; what replay re-checks is SM §1's edge set, which is a property of
the history.

**The steps are replayed in the RECORDED ORDER**, not jumped to the final phase,
for §7.5's own reason: driving `domain.Generation`'s own mutators re-checks SM
§7's edge set on every recorded transition, so a log recording an impossible
LADDER is refused exactly as one recording an impossible order history is. Only
two steps needed premise-free twins (`RestorePerpSubmission`,
`RestoreSpotAuthorization`); everything else reuses the live methods, and both
twins share their payload closure with the live step, so the transition table and
the never-net-short cap each keep one home.

---

## §76f — `ResumesLadder` is ACTED on, and a fixture in the suite asserted the
## opposite

§75i left "act on `ResumesLadder`" open. It is now `Deleverer.barred`, a
monotone per-hedge-group map written by `BarLadder`, read by `Open` **before**
the live check and before the reserve — so a barred package is refused whatever
else is true of it, and a refused `Open` leaves the shared ledger untouched.

**One rule, two situations.** The bar is applied on the LIVE path (inside
`supersede`) and at boot (from `Recovery.BarredLadders`). A version that held only
across a restart would be a system that refuses what it has forgotten and permits
what it remembers.

Building it broke `TestOpenAdmitsANextGenerationOnceTheFirstHasEnded`, and the
break was correct. That test quotes core §7.1's *"Complete → the escalation ladder
continues"* and reached its ending with an **`EmergencyClosePackage`
supersession** — the one ending that CLOSES the ladder. The ending KIND was a
collapsed dimension for as long as nothing acted on the path: any ending would
do, so the fixture reached for whichever was cheapest to write. The test now
completes the generation through the ladder (a `completeWithNoSpotSale` helper
exists so the next fixture does not reach for the cheap one again), and the
ladder-closing case is its own test over both external rows.

**No unbar, and that is the ruling.** `ResumesLadder` states the cost of being
wrong in that direction and accepts it — *"a breach needs a fresh escalation to
reach it — an alert"* — where the cost the other way is this machine
re-authorising itself against an arbitration that just fired. Nothing can clear
it by accident either, and the reason is arithmetic: once a group is barred no
generation opens on it, so no later ending can arrive to overwrite the entry.

---

## §76g — The group is TRACKED by the reinstatement, because nothing else tracks

`Classifier.Track` had **zero non-test callers** in the whole repo. At §7.5 boot
the ledger for a recovered generation's group therefore does not exist, and
`Reserve` would refuse.

`Restore` tracks it when untracked, with the asset taken from the generation's
own **target** — the same quantity `OpenGeneration` seeds every one of its zeros
from, which is what stops the ledger and the aggregate disagreeing about the
asset. (That disagreement surfaces as a quantity error from inside a fold rather
than as a refusal naming the group.)

An already-tracked group is **left alone** rather than re-tracked, because `Track`
installs a fresh ledger: re-tracking a group holding entries would discard them.
The question is asked through a new READ predicate, `Classifier.Tracked`, rather
than by calling `Track` and reading meaning into its error — reading an error as
a state is how a genuine "something else already registered this group" turns
into a silent no-op.

---

## §76h — What the evidence showed

- **`make ci` green uncached**, both modules. `SPEC EDIT OWED` grep over
  `internal/ tools/`: **zero**.
- **29 mutants.** 25 caught on the first aim or after a re-aim; **three real
  survivors**, each closed by a new test; **one equivalent mutant**, diagnosed.
  - **M10** — `BarLadder` overwrites a differing first path instead of refusing.
    Real gap: the first-writer-wins rule decides which arbitration an operator is
    SHOWN, and there is no other record of it once the generations are gone.
  - **M11** — the replay bars on the RESUMING path. Real gap, and the sharpest:
    `Recovery.BarredLadders` had **no test at all**. Closed by one test over all
    three paths together, because a version barring on every supersession passes
    any external-row fixture, a version barring on none passes any premise-lost
    fixture, and inverting the predicate passes both of those suites.
  - **M12** — the replay keeps the LAST bar rather than the first. Real gap, and
    the fixture had to be SCRIPTED: the log it needs is one this build can no
    longer write, since the first bar refuses every later `Open`. A rule about
    which of two facts survives is not testable from a world that can only
    produce one.
  - **M28** — `RestoreGeneration` delegates to `OpenGeneration` with a
    hand-built intact premise. **SURVIVED and the mutant was wrong**: it is
    behaviourally equivalent, because an intact premise is exactly what the
    check passes on. The claim it aimed at lives in the **signature** — and
    **M28b**, adding a `premise Premise` parameter, reports `NOBUILD!!` with
    *"not enough arguments in call to domain.RestoreGeneration"*, which is this
    repo's recorded kill-by-compiler verdict. The absence of the parameter is
    what makes consulting a premise unexpressible rather than discouraged.
- **Four `NO-OP!!` and three `NOBUILD!!` were bad seds**, all re-aimed. The
  `NOBUILD` shape was the same one three times: deleting an `if !x {` guard
  orphans the variable, so `if x && false {` is what works on that idiom.
- **The collapse checklist ran before the mutations** and pinned six dimensions
  the existing suites do not reach: target-vs-confirmed (1.0 target, 0.4
  executed — equal values make FM-5's own mistake invisible), authorized-vs-
  confirmed (0.25 against 0.4), generation cardinality (two GROUPS, since §7
  forbids two on one), the step SEQUENCE (a log that skips the perp submission is
  the only fixture a jump-to-final-phase rebuild fails), the `tracked` parameter,
  and the bar's PATH class. It also caught a fixture that would have passed for
  the wrong reason: two perp legs planted in the classifier's fold poison each
  other's premise, superseding the generation and releasing the entry the test is
  about.

---

## §76i — Rejected alternatives (do not silently reopen)

- **Handing the `Replayer` a `Classifier`.** §75h's refusal, unchanged. The way
  through is a different SUBJECT (§76d), not a narrower capability.
- **A generation write repository port.** Rejected: a repository is a place to
  PUT a generation, and the invariant that matters is that the aggregate and its
  ledger entry move together. A store would satisfy the first and be silent about
  the second, leaving the reserve as a caller's discipline — which is what
  `Restore` exists to make a call that cannot be forgotten.
- **Reinstating at `GateJournalReplayed` with `perpDone = 0`.** Rejected on
  arithmetic (§76b): an authorized generation cannot be rebuilt at all there, and
  an unauthorized one would need a second amend after the join, so the phase
  would exist twice.
- **Keeping one gate and reporting it after the whole sequence.** Rejected: it
  contradicts "a gate is reported about its own evidence", and it makes the
  replay's own verdict — the one that separates a broken chain from a log that
  does not compose — arrive after three phases that assume it passed.
- **Making the reinstatement a REQUIRED collaborator of `Reconciler`.** Rejected
  on `SecondLook`'s precedent: a deployment whose escalation ladder never runs has
  nothing to reinstate. What it must not do is PASS, so an unwired phase meeting a
  live generation is `OutcomeFailed` with `ErrGenerationRestoreMissing` — the
  failure that names the wiring, exactly as `ErrSecondLookUnwired` does one gate
  over.
- **Rebuilding every ENDED generation to derive the ladder bar.** Rejected: the
  bar needs `(group, path)` and nothing else, which is a fold over the record
  stream. Rebuilding aggregates for the whole history would need a projection read
  per generation to answer a question neither the aggregate nor the ledger is
  about.
- **Refusing, at replay, a generation OPENED on an already-barred group.** A real
  composition check, and rejected for this slice: `Open` is the single home of
  that rule, and a replay-side twin is the second-copy shape this repo has paid
  for. `Restore` refuses the combination when it meets it, which is the same
  contradiction caught where the ledger is in hand.
- **Carrying the confirmed perp reduction in `Recovery`.** Rejected: it did not
  exist when the `Recovery` was produced. Loading it inside the reinstatement is
  what makes the dependency on the join's having run a fact about the call graph
  rather than a comment.

---

## §76j — What this task deliberately does NOT do

- **Trigger the ladder.** Core §7.1's liq-distance escalation is milestone 4's
  other workstream; nothing calls `Open`. `oms.Module.Delever` exists and the
  field's doc says so.
- **Rebuild the generation's SPOT executed quantity.** `spotSold` moves only at
  `ConfirmSpotReduction`, which ENDS the generation — so no live generation has
  one, and `OutstandingIntent` at every spot-side phase is the authorized size.
  Stated rather than assumed, because the symmetric-looking case (the perp leg)
  does need the venue.
- **Give `Classifier.Track` a caller outside recovery.** The gap is older than
  this slice and is not closed by it: every hedge group that never had a
  generation is still untracked.
- **Filter-conform the delever legs (task #184).** Unchanged from §74i/§75i; its
  trigger has fired.

---

## §76k — Work items

| # | Item | State |
|---|---|---|
| 1 | `domain.RestoreGeneration` + the two premise-free restore steps, sharing their payload closures with the live ones | **done** |
| 2 | `Recovery.LiveGenerations` widened to `GenerationRecovery`; `Recovery.BarredLadders` | **done** |
| 3 | `Deleverer.Restore` (track → reserve → amend → install, releasing on failure) and `Deleverer.BarLadder` | **done** |
| 4 | The ladder bar consulted by `Open` and written by `supersede` — one rule, both situations | **done** |
| 5 | `application.GenerationLadder` + `Reinstater`; `ports.OrderLookup` | **done** |
| 6 | `boot.GateGenerationsReinstated`, its three outcomes, and the roster | **done** |
| 7 | `oms.Module.Delever` + `Reinstater` wired; the Deleverer's first composition root | **done** |
| 8 | Collapse checklist, 29 mutants, three gaps closed | **done** |
| 9 | Spec edits → v0.9.57 | **done** |
| 10 | Trigger the ladder from §7.1's escalation | **milestone 4, other workstream** |
| 11 | Filter-conform the single-order dispatch path | **task #184** |
