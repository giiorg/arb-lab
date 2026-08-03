# M3 slice F1 — §7.5's replay half: the anchor's record kind, the read port, and the rebuilt projection

**Task:** #165 (§47f slice F, first half)
**Date:** 2026-08-03 (session 84)
**Reads:** core §7.5 (the normative owner), core §5.3 (the anchor, the state set,
"resolved by query only"), core §5.1 (the aggregate, the OCC version, the
hash-chained journal), core §6 steps 5/6/10 (journal-then-apply, the crash
boundary), SM §1 (the edge set the replay re-checks), §48 (slice A's journal
adapter, whose rejected timestamp is this slice's subject) and §53 (slice D,
which added the anchor to the projection and scheduled the column here).

---

## §55a — the split: why slice F is two slices, and where the seam is

§47f's slice F is *"reconcile-only boot (§7.5) + continuous net-delta
invariant"*, and it is split here into **F1 (this record) and F2 (task #178)**.
The seam is not effort; it is the one §7.5 and `platform/boot` already draw.

`boot.Gate` holds §7.5's preconditions as a closed vocabulary, and the first
three are **separate gates**: `GateJournalReplayed` (state rebuilt from the
journal, *before any venue call*), `GateVenueJoined` (the journal joined to
venue truth), `GateNetDeltaAsserted` (the invariant). The posture is the weakest
over the required set, so a build that satisfies the first and not the others is
in reconcile-only and cannot trade — the split cannot produce a system that
believes it has recovered when it has not.

The second half also needs venue surfaces this milestone has not built:
§7.5 names `OrderByClientID`, `Fills`, **`Positions`** and **`FundingLedger`**,
and slice C built the first two. `Positions` and `FundingLedger` have no client
in `engine/venue` at all, so F2 is a venue slice with a use case on top, while
F1 touches nothing outside the local store.

## §55b — RULING: the authorization gets its own journal RECORD KIND

`ports.Journal` gains `AppendAuthorized(id, at)`, and `AppendState` **refuses
`StateDispatchAuthorized` on the target alone**.

This is slice D's split arriving at the second store, and the reason it did not
arrive with it is the finding worth keeping. Slice D made the anchorless
authorization inexpressible in the **projection**: `ApplyAuthorized` takes the
instant, `ApplyState` refuses that target, so no caller can advance an order to
the crash boundary without producing the value §5.3 measures from. The
**journal** kept the anchorless spelling as its only one — and the journal is
the store §7.5 replays. So the projection could not hold an unanchored
authorization and the log could not hold an anchored one, which means a
crash-recovered order arrived at `DispatchAuthorized` with no anchor **however
carefully the live path had stamped it**, and `AbsentAfterFinality` stayed
unreachable for exactly the order §5.3 says recovery resolves that way.

That is the recorded **mirror-rule second axis**, on an axis this repo had not
used it on: not two venues and not two directions of travel, but
**journal-and-projection**. The general form is worth stating because the next
instance will not look like this one: *when a rule is enforced by making a shape
inexpressible, ask which OTHER stores can express it.*

**The stored form.** A third detail table (`journal_authorized`) with the
instant as Unix nanoseconds and the band as nanoseconds, both `NOT NULL`; a
third kind tag inside the hashed preimage; two more append-only triggers. Two
consequences were checked rather than assumed:

- **The kind tag stopped being defensive.** With two kinds it guarded a future
  edit (a planned record has strictly more fields than a state one, so nothing
  could collide). An authorization is a client id plus sixteen fixed bytes and a
  state record is a client id plus a length-prefixed string, so without the tag
  a state whose text encoded as those bytes would share a preimage with an
  authorization of the same order.
- **An instant outside 1678..2262 is REFUSED at append.** `time.Time.UnixNano`
  wraps rather than reporting, and the wrapped value is a plausible instant in
  the wrong century — which §5.3's retention gate then reads as *"the venue has
  purged this by now"*, an absence that proves nothing, reported as proof.
  `engine/capture`'s frame codec pays for the same lesson with the same
  round-trip check.

**Slice A's rejection, re-examined rather than inherited.** The codec refused a
timestamp on two grounds: the journal is ordered by sequence, so a wall-clock
field would be *"a value the chain covers but nothing reads"*; and *"when an
event time is genuinely needed it is the venue's, and it arrives on a fill"*.
The first still holds and is why there is still **no per-record timestamp** —
nothing here records when a record was written. The second expired, and the
order that needs the anchor is one with **no fill and no venue record at all**.
The codec's doc now says which half held.

## §55c — RULING: the replay is a rebuild THROUGH the write path

`application.Replayer` walks the journal and applies every record through
`ApplyPlanned` / `ApplyAuthorized` / `ApplyState` — the same methods the live
path calls.

The alternative is a bulk load that installs records directly, and it is faster
and gives up the property that makes the recovered state worth having: replayed
through the write path, the projection can only reach states a live process
could have produced. SM §1's edge set is checked on every replayed transition,
§5.1's OCC version advances exactly as it did the first time, and
`domain.Ledger.Register` refuses a duplicate `ClientOrderID` at boot for the
reason it refuses one at planning. **A log recording an impossible history is
refused rather than installed** — the difference between a projection that is a
replay of the log and one that merely came from it.

Two consequences that are not obvious from that sentence:

- **A duplicate delivery is not a corrupt history.** `journalcodec.go` puts the
  sequence in the preimage precisely because §5.3's duplicate deliveries mean
  `AppendState` may legitimately record the same order reaching the same state
  twice, and `CanTransition` answers `ErrTransitionNoOp` for a self-edge. A
  replay that treated one as a broken log would refuse to boot over a journal
  the spec calls well-formed. It is a counted no-op, and counted rather than
  ignored because a journal that is mostly duplicates means something upstream
  is re-delivering and nothing else would report it.
- **Nothing rebuilds cumulative quantity.** There is no `AppendFill` — the port
  is *"narrowed to what the dispatch path writes"* — so every replayed record
  carries zero executions. After this phase an order's **state** is
  authoritative and its executed **quantity** is not yet a claim at all. The
  right reading is the restrictive one, and the checklist enforces it: the
  net-delta gate is separate, so the posture stays below trading until F2 runs.

## §55d — RULING: verification is per record, before the record is applied

`ports.JournalReplay` is a **second port**, not three methods on `ports.Journal`.
Nothing in the system holds both: the planner and the dispatcher can append and
cannot replay; the replayer can replay and cannot append. That is
`ports.OrderResolver`'s move (the read half of the venue surface, so that "never
resubmitted" is unexpressible) applied to the local store, and it is worth more
here than tidiness — the two capabilities compose into the one thing recovery
must not do, which is write to the log it is reasoning about while it reasons.

The visitor is **one method per record kind**, not a record struct with a kind
tag. A single struct would carry an order on one kind, an instant on another and
a state on the rest — optional fields whose zero values are legal and
meaningless — and a replayer that forgot a kind would produce a silently short
state with nothing to report. With a method per kind the omission is a compile
error and a fourth kind breaks every visitor at build time: `exhaustiveswitch`'s
argument expressed in the type system, available here because the set of kinds
is the set of methods.

**Verification runs per record and before the visitor sees it.** A verify pass
of its own reads the log twice and leaves a window between the passes;
verify-after-replay hands a use case records it is about to be told to distrust.
Per record, a caller that stops on the first error stops with state built
entirely from verified records — which is what makes the partial rebuild worth
leaving in place rather than rolling back.

**Three claims, three distinct failures**, and the first is the one a reader
assumes the database already makes:

| Check | What only it catches |
|---|---|
| the sequence is **dense from 1** | a dropped **prefix**, re-anchored to the genesis seed: every remaining record verifies against its own predecessor and the first verifies against genesis, so the only surviving evidence is that the first sequence is 2 |
| `prev_hash` names its predecessor | a link edited to claim a different predecessor — the digest is recomputed from the WALKED prev, so the stored column is checked by nothing else |
| the digest matches the **columns** | a record edited in place. Rebuilt from typed columns rather than a stored preimage, which would verify its own output perfectly |

**A stated limit: truncation at the tail is undetectable.** A chain proves
nothing about records that were never seen, and nothing here stores an
independently durable head. Deleting the last *k* records leaves a journal that
replays as a shorter, perfectly valid one. Recorded in §7.5 rather than left for
a later reader to discover as a gap, and the fix — an independently durable head
— is out of this milestone.

## §55e — THE FINDING: §7.5's preconditions vary on TWO axes and the vocabulary had one

`boot.Outcome` gains `OutcomeEvidenceUnusable`, and the reason is the finding.

§7.5's own paragraph separates its preconditions on **one** axis: self-clearing
versus latching. `boot.Outcome` was built on that reading, and the postures its
members impose vary on a **second** — how restrictive the refusal is. The two
are independent, so the members are points in a product rather than rungs of a
ladder, and three of the four combinations had a value:

| | freely re-reportable | latching |
|---|---|---|
| **reconcile-only** | `OutcomePending` | `OutcomeNeedsOperator` |
| **halted** | `OutcomeFailed` | *(missing)* |

The missing one is what a journal that will not verify needs, and each available
reading is wrong in its own direction:

- **As `OutcomeFailed`** it halts and is freely re-reportable, so a re-run of
  the replay — over bytes that cannot change — could re-admit trading with
  nobody having looked. That is §7.4's *"never health-flap re-entry"* arriving
  through a check whose retry is deterministic.
- **As `OutcomeNeedsOperator`** it latches, and the posture it imposes is
  `PostureReconcileOnly`, which **permits `ActReconcile`**. This is the unsafe
  one, and the reason is a claim the package makes one type over: an
  `ActReconcile` is *"exposure-neutral by construction"*. The construction is
  that it re-drives what already happened — and what it re-drives is read from
  the journal. §7.5's `DeactivationPending` re-drive fires
  `EmergencyPlanStore.Deactivate` for whichever plan a corrupt log appears to
  name, and **that call is terminal with no reactivation**: after its ack no
  `Replicate` for that `TradeID` is accepted and `ListActive` never returns it.
  A plan tombstoned for a trade that is in fact still open leaves that position
  **permanently without a takeover path** — an actuator enumerating plans after
  host death finds none, and what the plan carried (client order ids, allocated
  quantities, recovery cursors) is exactly what it cannot rebuild from a dead
  journal — and it releases the **hedge-group lease**, so a successor trade may
  be admitted over live exposure.

**The general form: exposure-neutrality is a property of an ACT given
trustworthy INPUTS, and this is the one gate whose failure means they are not.**
The nearest recorded relative is *an invariant cannot audit its own input* —
same asymmetry, one layer out: the classification of an action as safe was
derived from the very store whose trustworthiness is in question.

**The latch rule generalises with it.** A latched gate now admits exactly two
reports: the same outcome again (honest — the same walk over the same bytes
fails identically, and a producer must not special-case its own second call),
and an **escalation to a strictly more restrictive latching outcome** (the
alternative pins a gate at the weaker of two refusals by the accident of which
was discovered first). Everything else is refused, and the case that shows why
the check cannot be a posture comparison alone is `OutcomeFailed`: it is *more*
restrictive than `OutcomeNeedsOperator` and it does not latch, so admitting it
would launder the hold into one a later `OutcomePassed` could lift — clearance
by two reports with no operator in either.

## §55f — rejected alternatives

**(i) Report a broken chain by failing the boot PHASE.** `boot.Run` turns a
phase's error into an unwind and a dead process, and that is the wrong verdict
twice: `PostureHalted` is where the package says a failed boot lands, so the
process is meant to stay alive; and a process that exits presents to §7.4's
watchdog **exactly as a dead host**, escalating to an actuator takeover over a
file an operator could have looked at. The verdict is a posture, and a
constructor or a phase error cannot report one. This is also why `oms.New`
constructs the replayer and does not run it.

**(ii) Materialise the journal in memory and walk it.** Rejected on a property a
write-ahead log must not give up: the walk is O(1) in memory whatever the
journal's length, and a recovery that needs the whole journal resident stops
working on the day the journal is big — which is the day it matters most. The
implementation is one query with three `LEFT JOIN`s, and the shape is forced for
a second reason: `SetMaxOpenConns(1)` means an open `sql.Rows` holds the pool's
only connection, so the obvious per-record detail lookup **deadlocks**. A test
would report `TIMEOUT!!`, which names no defect.

**(iii) Give the `Replayer` a `ports.Journal` as well, so it could compact or
mark.** Rejected in §55d: the two capabilities compose into writing to the log
being reasoned about.

**(iv) Keep the replay's own id→state map and report from it.** Written first,
then removed. It is a second copy of a fact the repository owns, able to
disagree with it in exactly the case that matters — a record the repository
refused — and it bought nothing, because the walk already does a `Load` per
state record for the duplicate check. The report now reads each order's final
state back out of the projection, which also makes the `Load` failure path
reachable by a test instead of unreachable-and-assumed.

**(v) Treat a `NULL` detail row as a zero.** Rejected: it produces an order with
no side, at no price, in no state — every field legal, nothing true. It is
unreachable through the adapter (one transaction, foreign keys verified on at
open) and reported anyway, because the store is a file an operator can open with
`sqlite3`.

## §55g — the build record

**Landed.** `internal/oms/application/ports/journalreplay.go` (the read port +
the visitor), `internal/oms/application/replay.go` (`Replayer`, `Recovery`),
`internal/oms/infrastructure/driven/journalreplay.go` (the walk, the
verification, the column→domain rebuild); `ports.Journal.AppendAuthorized` +
`AppendState`'s refusal; the `journal_authorized` table, its triggers, its kind
tag and `encodeAuthorized`; `ports.ErrJournalChainBroken`;
`boot.OutcomeEvidenceUnusable` and the generalised latch rule; `Module.Recover`.
Core §5.3, §7.5 and SM §1 edited. Marker grep **0**; `make ci` green on both
modules; docs at **v0.9.36**.

### The collapse checklist, run BEFORE the battery

| Fixture | Would have collapsed |
|---|---|
| The chain's record KINDS | The length-varying chain test interleaved **two** kinds. With three, the third tag's presence in the preimage is unobservable — the same defect one member along, and the tell is that the fixture does not contain the thing under test. It now cycles all three |
| The anchor's two numbers | An instant and a band are both int64 nanosecond columns, so a codec writing the instant into both, or reading them back swapped, produces a chain that verifies. Both helpers derive the band from the offset, so the two never coincide |
| The tamper's target | Every tamper edits record **1 of 3**, so the claim is about a record that is neither the first-and-only nor the last |
| The report's order COUNT | Every per-order claim holds **two** orders: with one, "this order's final state" and "the first order's" are one value |
| The unresolved fixture's two orders | They end in **different** states, and the resolved one passes THROUGH an unresolved state on its way — so "derive from the final state" and "accumulate as you walk" disagree |
| The unresolved ids' order | Appended descending, so insertion order and sorted order differ |

### The mutation battery — 31 aimed, 29 killed by the suite, 2 by the gate

**One survivor was a real gap and it is the checklist's own lesson one layer
out.** Deleting the sequence-density check survived, because the fixture that
looked like its probe — a deleted middle record — is caught **first** by the
link check: record 3's `prev_hash` still names record 2's hash. A probe two
checks can see shows that something objected, not that this rule did. The
isolating probe is a **dropped prefix re-anchored to genesis**, where every
other rule is satisfied exactly; with it, the mutant dies.

**Two survivors are gate-covered, and the claim was verified rather than
asserted.** Dropping `OutcomeEvidenceUnusable` from either switch in `boot`
leaves it falling through to a trailing return that gives the same answer —
`PostureHalted` in `posture`, `true` in `latching` — because in both functions
the fallback IS the safe reading and the member's own group is therefore
untestable. That is this repo's recorded trailing-return trap, and what covers
it is `exhaustiveswitch`, which was run against each mutant and reports *"switch
over Outcome does not name OutcomeEvidenceUnusable"* with a clean positive
control on the unmutated file. The mutations that matter — the **plausible wrong
readings**, moving the member into the reconcile-only group or into the
non-latching group — are caught loudly by the suite.

Two `NOBUILD!!` verdicts were bad seds that orphaned a variable, re-aimed and
killed; two `NO-OP!!`s were stale line numbers.

### The reading pass corrected THIS SLICE's own finding, one step before it shipped

The finding's harm was first written as *"the `DeactivationPending` re-drive
fires a terminal tombstone that **disarms venue-native protection**"*. That is
wrong about the mechanism. `EmergencyPlanStore` is an **internal** port — "not a
venue call" — so `Deactivate` disarms nothing at any exchange; it tombstones the
off-host **emergency plan**.

The real harm is larger and needed the port's own contract to state: the
tombstone is *"TERMINAL, IDEMPOTENT … after ack any `Replicate` for this
`TradeID` (same OR later gen) is **REJECTED** (no reactivation); `ListActive`
never returns a tombstoned trade"*. So a plan tombstoned for a trade that is in
fact still open cannot be re-replicated **ever**, and the actuator — whose whole
reconcile path after host death is `ListActive`, because it "has NO `TradeID`
list after host death" — enumerates nothing for it. §7.4's plan content is
exactly what it cannot rebuild without the dead journal: client/algo order ids,
allocated quantities, recovery cursors. And §6 step 9 makes the ack what
releases the hedge-group lease, so a successor trade may be admitted over live
exposure.

It had reached four places — the core, the history, the handoff and a Go doc
comment — before the pass opened `EmergencyPlanStore`'s declaration. The shape
is the recorded one: **a plausible mechanism, asserted from a name rather than
read from the contract**, in a sentence whose *conclusion* was right. A wrong
harm attached to a correct finding is the version nobody challenges, because the
finding survives every check the harm would have failed.

### What F1 hands to F2 (task #178)

`Recovery.Unresolved` is the set §5.3 must resolve by query — every order whose
replayed state contributes a possible-delta interval, sorted, and deliberately
**without** `StatePlanned`, since a Planned-only order was never dispatched and
a resolving query would run to its deadline over an order this system merely has
not sent. The venue join takes that set, `Fills` supplies the cumulative
quantity this slice cannot rebuild, and the ids the journal does **not** contain
are §7.5's foreign orders — the canonical `OutcomeNeedsOperator`.
