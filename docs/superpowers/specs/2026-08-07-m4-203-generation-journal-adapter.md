# Design record §75 — M4 task #203: the `ports.GenerationJournal` adapter, one chain, and the replay rule that did not reach a second scope

**Session 119 · 2026-08-07 · docs v0.9.56**

Predecessor: `2026-08-07-m4-177-generation-ladder.md` (§74), whose work item 6 this
is. §74i named the fork and argued it in advance; this record is what building it
added, corrected and left open.

---

## §75a — What the task was

`ports.GenerationJournal` had six methods, a caller (`application.Deleverer`) and
**only a test fake**, so FLAG-3's *"a durable journal record … so recovery knows
the perp-finality precondition was met without re-derivation"* was a contract
nothing honoured. The store was the deliverable.

The design fork was already settled in §74h and core §5.1 — **one chain over a
widened subject column**, not two chains — so this slice's job was to build the
argued thing and find out what building it changes. Three things it changed, one
finding it produced, and one gap the mutation campaign found.

---

## §75b — The migration's scariest part is a non-event, and CHECKING that is the work

Task #203 carried an explicit warning: *"`chainLabel` keeps `/v1` only if no
existing preimage changes — widening the shared column changes the RECORD, so
re-read `encodePlanned` before assuming it does."*

It does not. `encodePlanned` hashes `string(o.ClientOrderID)` — a value off a
`domain.Order` — not "whatever is in the `client_id` column". Every encoder in
`journalcodec.go` is the same shape. So renaming the column changes the storage
and not one byte of any preimage, and `/v1` stands.

That is a claim rather than an observation, so it is a **fixture**:
`TestAJournalWrittenBeforeTheSubjectRenameStillVerifies` hand-builds a database in
the previous build's shape — `journal.client_id`, the three original tables, two
chained records written with the **oracle's** literal-spelled encoders — opens it
through `OpenSQLiteJournal`, and asserts the column moved, both links still
verify, a generation record extends the chain, and a reopen is a no-op rather
than a second migration.

**Two independent reasons the append-only store survives a DDL change**, worth
separating because only the first is obvious:

1. The triggers fire `BEFORE UPDATE` and `BEFORE DELETE` **on rows**. `ALTER
   TABLE … RENAME COLUMN` is neither — no row is written, none removed, every
   value survives under a new name. The append-only rule is about the HISTORY.
2. Even a migration that *did* rewrite rows would not move a hash, for the reason
   above. This is the transferable one and it is now core §5.1 ruling (c).

The migration refuses two shapes no build produces: a `journal` table with
**neither** column (realistically a path pointing at somebody else's database —
and §6 step 6's send path is the wrong place to discover that), and one with
**both**, which is refused for the sharper reason that there is no rule for which
column the existing records were keyed by, so choosing either silently re-keys a
chain that verifies perfectly afterwards.

---

## §75c — One chain buys a FACT; the test is what collects it

`TestTheAuthorizationRecordPrecedesTheSpotLegInOneChain` is the whole argument for
paying the migration. It asserts that FLAG-3's authorization record sits at a
lower sequence than **both** the generation's spot submission and the spot
order's own `planned` record — the second being the one that matters
operationally, since §6 step 5 writes it before the order can be sent, so it is
the boundary a crash can land inside.

**Under two chains that test could not be written at all.** Each chain verifies
internally; neither positions the other. The clause would hold exactly as long as
the application layer kept it, and nothing would report the day it stopped.

---

## §75d — Liveness without a kind→phase table

`application.rebuild` has to answer one question — which generations did the log
open and never end — and the obvious implementation is a map from record kind to
the phase that kind records. `ports.GenerationJournal` states that mapping method
by method, so a copy in the replayer is a second home for SM §7's table, drifting
the moment a kind's phase changes.

The rule that avoids it: **a generation is live unless the log records an
ENDING**, where an ending is a phase `domain.GenerationPhase.IsActive` calls
inactive. The three mid-ladder kinds do not participate at all, so there is
nothing to drift. What it rests on is one reachability claim — every inactive
phase can actually arrive — and that is a test rather than a paragraph:
`TestEveryEndedGenerationPhaseIsReachableThroughAnEndingRecord`, which iterates
every declared phase, asks `IsActive`, and for each inactive one records it
through the method that owns it. It counts the inactive ones and fails at
anything but three, because a suite whose loop body never ran would otherwise
pass having asserted nothing.

---

## §75e — The refusal had to land NOW, not with the rebuild

`NewDeleverer` has no caller outside tests. No composition root constructs one, so
**no journal in the field contains a generation record**, and the full rebuild —
reinstating the aggregate and re-`Reserve`-ing its ledger entry — is genuinely a
later slice (task #204). It needs a generation repository that does not exist and
the `Classifier` the replayer deliberately does not hold.

The refusal still lands here, and the reason is the standing rule about defaults:
*pick the one that is right when somebody forgets*. On the day the Deleverer is
wired, a boot over a journal holding a live generation must already refuse rather
than trade. `Reconciler` reports `boot.OutcomeIncomplete` on `GateJournalReplayed`
when `Recovery.LiveGenerations` is non-empty. Today that branch is unreachable, so
it costs zero operational noise and is correct the moment it can fire.

**Why `Incomplete` and not one of the other four:**

| Candidate | Rejected because |
|---|---|
| `Passed` | A ledger missing an entry reads as **more** room. §6.7's determined branch sizes a compensating trade from a residual too large by exactly the generation's outstanding net intent — buying back exposure that generation's own perp leg already committed to. `FM-5`'s shape through recovery, invisible to every invariant because they are computed FROM the ledger. |
| `Failed` | The log is intact and it composed. Nothing failed. |
| `EvidenceUnusable` | Halts and latches. The chain verified; the journal is trustworthy. Halting also forbids the reconciliation writes §7.5's own `DeactivationPending` re-drive needs. |
| `NeedsOperator` | Latches, and there is nothing for an operator to decide. What lifts this is a build that rebuilds generations — a fresh process with a fresh checklist, which a latch would not admit. |

It is reported **instead of** `Passed` and the sequence **continues**, because a
gate carries one outcome and each gate reports on its own evidence: stopping would
leave two gates unreported and an operator holding a refusal with no picture of
the account behind it.

---

## §75f — The one SURVIVOR, and it was the mirror rule collecting

23 mutations, 22 caught. The survivor:

> **M14b** — `replayGenerationOpening` stops validating the recovered hedge group
> (`group.Validate()` → `error(nil)`). **SURVIVED.**

Diagnosed as a **real test gap**, not a wrong mutant, and the gap is structural
rather than an oversight. Every check on the replay's side of the opening record
has a twin on the write path, so **no journal this adapter produces can reach any
of them** — a fixture built only by appending leaves every read-side refusal dead
while the suite reads as complete. The write-side refusals were tested; the
read-side ones were not.

The checks exist because the store is a **file an operator can open with
`sqlite3`**, and each column has its own consequence: an empty account merges
every subaccount into one hedge group, so the generation is judged against another
account's exposure; an empty reservation is a generation that cannot retire its
own ledger entry, which then subtracts from every later sizing for the life of the
process; an empty target asset is a quantity §7's never-net-short cap cannot
compare.

**The mirror was already there.** `TestReplayRefusesAPlannedRecordThatIsNoLongerAValidOrder`
has existed since M3 slice F1 and uses `tamperColumn`, which drops the update
triggers and edits a detail row — the exact technique the generation opening
needed. `tamperColumn`'s trigger list had not grown with the five new tables, so
the technique was one line away from being unavailable. Fixed, fixture written,
M14c caught.

*The lesson, stated for reuse: when a validation exists on BOTH sides of a store,
the write side's fixtures cannot reach the read side's copy, and every natural
fixture is built by writing. The read-side check needs a fixture that bypasses
the writer — and if the store is append-only, that fixture needs the tamper
helper's permission list to have kept up.*

---

## §75g — What the evidence showed

- **`make ci` green uncached**, both modules. `SPEC EDIT OWED` grep over
  `internal/ tools/`: **zero**.
- **23 mutants**: 22 caught, 1 real survivor (§75f, now caught as M14c). Kills
  covering: the two leg kinds sharing a tag; the migration renaming nothing; the
  migration failing to recognise an already-migrated journal; the both-columns
  refusal falling through to a rename; the phase recorder admitting `Superseded`;
  liveness inverted; a supersession not ending its generation; every generation
  reported live; the unknown-generation refusal; the duplicate-opening refusal;
  the gate always/never `Incomplete`; the opening preimage dropping the
  reservation; the account losing its length prefix; a perp record dispatched as
  a spot one; the perp dispatch passing the generation id as the leg; an
  asset-less authorized size accepted; the replay accepting an opening with no
  reservation; the spot leg preimage dropping its order id; the supersession path
  unvalidated; the generation count reporting the ended ones.
- **Two `NOBUILD!!` verdicts were bad seds** (`duplicate case` from replacing one
  enum member with another already present) and one more was the `if err := X;
  err != nil` idiom orphaning `err`. Re-aimed at the CALL (`X` → `error(nil)`)
  rather than at the condition, which is the shape that works on that idiom.
- **The collapse checklist ran before the mutations** and added six dimensions the
  four existing ones do not reach — the leg pair (with a **same-order-id** variant
  as the isolating fixture for the kind tag), generation cardinality, the ending
  kind, the authorized zero, the opening's adjacent free strings, and the stored
  column name (which every fresh-journal fixture pins, hence the hand-built
  old-shape database).

---

## §75h — Rejected alternatives (do not silently reopen)

- **Two chains / a second store.** Rejected in §74h and re-confirmed by §75c:
  it makes FLAG-3's ordering clause unwritable as a test.
- **A `leg` discriminator column on a shared `journal_generation_leg` table.**
  Rejected: it is a second spelling of a fact the `kind` column already states,
  with no rule for a replay that finds the two disagree — `encodeAuthorized`'s own
  argument for leaving the state out of its preimage. The SHARED table is kept,
  because both kinds' column means the same thing (a client order id); what was
  refused is the redundant discriminator beside it. Contrast `state` and
  `venue_order_id`, which are both one `TEXT` and got separate tables precisely
  because they mean different things.
- **A `schema_version` row to drive the migration.** Rejected: it states the
  schema's shape a second time, and the copy that drifts is the one that is not
  the definition — the same argument the `CHECK` constraint lost in slice A.
  `PRAGMA table_info` asks the database what it **is** rather than what it was
  told.
- **A shared `appendLeg` helper that picks its encoder from the kind.** Rejected:
  the whole claim under test is that the tag distinguishes the two legs, and a
  switch inside the helper would be a third place the distinction is made, able to
  disagree with both call sites. The helper takes the **already-encoded preimage**,
  so it carries no opinion about which leg it is writing. The oracle refuses the
  same fold for the same reason and spells all six tags as literals.
- **A `subject` lookup method on the adapter.** Rejected: the two id spaces share
  no namespace, so a `WHERE subject = ?` with no kind predicate silently matches
  the other space. There is no such method, so the unsafe query is **unwritable**
  rather than discouraged — the replay walks in sequence order and branches on the
  kind it already has.
- **Rebuilding the generation aggregate in `rebuild` now.** Rejected as
  out-of-scope AND as unsafe in the shape available: it needs the `Classifier` to
  re-`Reserve`, and handing the replayer one gives recovery the exact capability
  `ports.JournalReplay` exists to withhold — the ability to change the state it is
  reasoning about while it reasons. Task #204.
- **Carrying `SupersessionPath.ResumesLadder`'s answer into `Recovery`.**
  Rejected for this slice: the ladder is per hedge GROUP, so reporting it needs the
  opening record's group threaded through, and the field would have no reader
  until #204. An unread field is a worse defect than a named gap. The path is
  **validated** at replay (an undeclared one has no answer to the question that
  follows a supersession) and then dropped, which is stated at the method.

---

## §75i — What this task deliberately does NOT do

- **Rebuild the generation aggregate and re-`Reserve` its ledger entry (task
  #204).** The boot refusal above is what holds the line until it lands.
- **Act on `ResumesLadder` at boot.** The external rows do not resume, so a
  recovered process reopening a generation after an `EmergencyClosePackage`
  supersession would be this machine re-authorising itself against an arbitration
  that just fired. Unreachable today (no root builds a `Deleverer`); part of #204.
- **Wire the `Deleverer` into a composition root.** Unchanged from §74.
- **Filter-conform the delever legs (task #184).** Unchanged from §74i; its
  trigger has fired.

---

## §75j — Work items

| # | Item | State |
|---|---|---|
| 1 | Six kind tags + six encoders; the tag now load-bearing four more times | **done** |
| 2 | The subject-column migration, its two refusals, and the old-shape fixture | **done** |
| 3 | Five detail tables + ten append-only triggers | **done** |
| 4 | Six `Append*` methods on `SQLiteJournal`; `AppendGenerationPhase`'s payload refusal | **done** |
| 5 | The replay: nine joins, six preimage arms, six dispatch arms, two rebuild helpers | **done** |
| 6 | `ports.JournalVisitor` + 6; every visitor in the system answered at build time | **done** |
| 7 | `Recovery.Generations`/`LiveGenerations` + `Reconciler`'s `OutcomeIncomplete` | **done** |
| 8 | Oracle extension (six kinds, literal tags), collapse checklist, 23 mutants | **done** |
| 9 | Spec edits → v0.9.56: §5.1 ruling (c), §7.5's recovered-generation paragraph | **done** |
| 10 | Rebuild the aggregate + re-`Reserve` the ledger entry | **task #204** |
| 11 | Filter-conform the single-order dispatch path | **task #184** |
