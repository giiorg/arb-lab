# Record §82 — M4 slice B3 part 1b: the `Ambiguous` margin op's clearance, and the act §7.3 already told us where to put

**Date:** 2026-08-08 · **Task:** #213 · **Docs:** v0.9.63 · **Campaign:** 29
mutants, 5 real survivors (all closed; one by a SIGNATURE rather than a test) ·
`make ci` green uncached on both modules · `SPEC EDIT OWED` grep zero ·
**Blocks:** #215 (with #214).

Read record §81 first — this closes the first of the three items its §8 lists as
owed, in the order it names. Read `docs/build-spec.md` §7.1's *"every margin op
is a durable target-state machine"* bullet for the incident, §7.5's
outcome-scope paragraph for the latch, and **§7.3's operator control-plane
bullet** for the sentence that decides this slice's shape.

---

## 0. The hole, stated as what a real deployment would do

Task #212 made the verdict durable and made `Ambiguous` **latch**, which is
right and is what §7.1 and §7.5 both say. The consequence nobody could act on:

> `boot.Checklist.Clear` lifts the outcome within one process. Nothing lifts the
> **journaled** verdict. So the next boot reads the `ambiguous` record back,
> `Recovery.AmbiguousMarginOps` names the op again, the gate reports
> `OutcomeBlocksOpening` again, and `ExitClass` returns `ExitOperator` again —
> for ever, with no act in the system that ends it.

It is unreachable today, because nothing writes a `MarginOp` until the
dispatcher (#215). It becomes a **permanent halt on the day the dispatcher
lands**, which is why §81 §8 put it first and why it must land before #215
rather than beside it.

The remedy §7.1 names is not a retry and not a timeout: *"the staged-float
ledger is reconciled by explicit accounting / operator action, **a §9 case**"*.
So the missing thing is an **operator act**, and the whole design question is
what shape an operator act takes in a system whose operator control plane
(§7.3) does not exist yet.

---

## 1. The shape is not a choice between two homes — §7.3 names one

Task #213 was filed with two candidates: a third journal record kind, or a fifth
`MarginOpStatus`. §81 §5 had already refused the enum member (the four statuses
answer *what did the evidence conclude*; a clearance answers *what did the
operator do*, which is a second question in one vocabulary). That leaves the
record kind against a third candidate this slice had to consider and refuse —
an operator-authored **document**, which is the shape §4.1's Portfolio-Margin
attestation takes one context over.

§7.3's requirement settles it in its own words:

> the operator API … MUST enforce … an **immutable per-actor audit** of every
> control action (**who / what / when, appended to the tamper-evident log,
> §5.1**).

§5.1's tamper-evident log is `ports.Journal`'s hash chain — the store this
adapter already writes three record kinds into. So the clearance is a **fourth
record kind on the one chain**, not because a journal is convenient but because
the section that governs operator acts names that log as the destination.

**Why not the attestation's document shape.** `risk/infrastructure/attestation`
states its own discriminator, and applying it here answers cleanly:

> *"#179 needed `arblab baseline record` because minting a baseline meant
> reading the venue's wallet and the journal's head — machine work a human
> cannot do by hand. This document has no inputs a machine holds: every field is
> the operator's own word."*

A clearance record's inputs **are** partly the operator's own word — but its
destination is a hash-chained append-only log at a sequence whose `prev_hash` is
the current head. That is machine work by construction, so the act needs a
command, and once it needs a command the document adds a second durable home for
one fact. Three further asymmetries, each independently sufficient:

- **A document is a STATE; a clearance is an EVENT.** The attestation says
  *"Portfolio Margin is off"*, a standing fact re-read at every boot. A
  clearance says *"on this date this person reconciled op `M-7`"*. Storing a
  standing document of *cleared op ids* means a file that grows without bound
  and a config digest re-pinned on every incident.
- **The config pin would become the lift.** The attestation's two-act rule puts
  the second act in the deployment config. Applied here, the thing that lifts a
  §9 accounting incident would be a config edit — the surface `platform/config`
  deliberately makes restart-required and boring, now load-bearing for an
  incident.
- **The chain is what makes it evidence.** §81 §2a's argument for putting the
  *verdict* inside the chain — *"a verdict nothing covers is a verdict an edit
  can change"* — is stronger for the clearance, whose entire content is a claim
  about a human act. A clearance outside the chain is a line anyone can add
  afterwards.

---

## 2. What the record carries, and the field that is deliberately NOT on it

```
MarginOpReconciliation {
    Op    MarginOpID  // WHAT — the one incident this act ends
    Actor string      // WHO
    At    time.Time   // WHEN — the operator's own reconciliation instant
    Note  string      // the accounting trail; blank is REFUSED
}
```

§7.3's *who / what / when*, plus the note. Every field is required and none has
a usable zero — `platform/`'s rule reaching a record that is not in `platform/`,
for the reason it exists: an unattributed clearance is exactly the artifact an
audit requirement is written to prevent.

**`Op` is one id, and there is no bulk form.** A §9 incident is cleared by an act
that names it. A `--all` would let one keystroke end every open incident
including the one the operator has not looked at, and it is the shape that makes
the per-actor audit worthless while satisfying it.

### 2a. The FINDING is not a field, and this is the ruling most likely to be reopened

The tempting fifth field is what the accounting concluded — did the top-up land?
`risk`'s attestation is the precedent that argues for it: *"the document records
what was OBSERVED, never the verdict"*, and a clearance carrying only *who /
when / why* is, plainly, a **checkbox**.

It is refused, and on §7.1's own terms rather than on economy:

1. **§7.1 says the attribution may be impossible.** The section's word for this
   state is `Ambiguous` because *"evidence is not causal identity"* — a
   coincident manual adjustment, a position mutation, or a duplicate
   same-amount row inside the window. A required `Finding ∈ {applied,
   not_applied}` demands from the operator exactly the determination the
   section says the evidence cannot support, and the operator's answer would be
   a guess wearing a field name. Making it three-valued with an `undetermined`
   member does not fix this: all three values would lift the block, so the field
   decides nothing.

2. **What §7.1 asks to be reconciled is the FLOAT, not the op.** *"the
   staged-float ledger is reconciled"* — a claim about capital allocation
   (§7.2: `treasury` owns the rescue-margin float, and it counts toward the
   per-venue concentration cap). Even a doubled top-up, which §7.1 addresses by
   name, reconciles and lifts: *"capital-allocation drift, not naked exposure —
   still an incident (alert + reconcile the float), never silently absorbed."*
   So no finding a real accounting can reach is one that should keep the block
   on, which is the test for whether a field belongs to a decision.

3. **There is no staged-float ledger to record it into.** `treasury` is not
   built. A structured finding today is a schema field frozen before its
   consumer, and its second wrong direction is the one that bites: the
   reconciliation the ledger will actually want is *machine* work — re-anchor
   the belief to the venue's posted figure — not a word typed at a prompt.

**So a checkbox is the correct shape, and the design work is elsewhere:** making
the act attributable, durable, scoped to one incident, and impossible to take by
accident. §7.1 makes the operator's act the precondition; a system that demanded
evidence for it would be inventing a rule the section does not state.

**The revisit trigger is named rather than left as "nothing reads it yet"** — a
rejection's reason can expire. When `treasury`'s float ledger lands, the
reconciliation gains a **derived** half (the re-anchor, computed from a venue
read at the moment of the act), and that half arrives with its consumer. This
paragraph is where to start.

---

## 3. The refusals, and which end each one lives at

The split is `AppendMarginOpResolved`'s, unchanged: the **adapter** refuses what
would make the log uninterpretable, the **use case** owns legality, and the
**replay** refuses a history that cannot have happened.

| refusal | where | why there |
|---|---|---|
| blank `Op` / `Actor` / `Note`, zero `At` | domain `Validate`, called by the adapter | well-formedness; a record admitted here is one a replay must interpret |
| the op is unknown to the log | use case | a typo'd id, and the operator must be told which |
| the op has **no verdict** (still in flight) | use case | there is no incident yet — a clearance ahead of the ambiguity is a pre-authorisation |
| the verdict is `applied` / `not_applied` | use case | no block exists to lift; clearing it records an act with no subject |
| the op is **already reconciled** | use case | one incident, one clearance |
| a reconciliation for an op the walk never saw dispatched | replay | `MarginOpResolved`'s rule, same failure direction |
| a reconciliation whose op has no `ambiguous` verdict **at that sequence** | replay | the ordering claim the single chain buys |
| a **second** reconciliation of one op | replay | see below |

**The second reconciliation is refused rather than tolerated, and it parts
company with `MarginOpResolved`'s duplicate rule.** A restated *verdict* is
harmless because it says what the log already says; two reconciliations
necessarily differ in actor, instant and note, so there is no "identical repeat"
to be tolerant of. Both readings of which one governs are wrong: taking the
first discards the later act's attribution, taking the last lets a second actor
overwrite the record of who actually looked.

**The read-side check is reachable only by bypassing the write side**, which is
this repo's recurring shape — the write-side check is what makes its read-side
twin unreachable. The fixtures therefore write records through a **separate
journal handle** directly, which is also precisely the shape a crashed
predecessor leaves and is how #212 §7b's replacement fixture was built.

---

## 4. What the clearance lifts, and the one thing it does not

At the next `arblab reconcile` boot the replay folds the reconciliation and the
op **drops out of `Recovery.AmbiguousMarginOps`**. With no other ambiguity,
`MarginResolution.Outcome()` reports `OutcomePassed`,
`GateMarginOpsResolved` passes, `ExitClass` returns `ExitClean`.

**It does not lift the latch inside a process that is already holding it**, and
that is stated as a limitation rather than hidden:

- The engine's `boot.Checklist` latched `OutcomeBlocksOpening` when it read the
  ambiguity. Nothing in this slice reaches into a running process, because the
  thing that would — §7.3's control API — is what does not exist. `Clear`
  remains the in-process lift and remains unreachable from outside.
- The journal is a single-writer SQLite store (`SetMaxOpenConns(1)`, `_txlock=
  immediate`, *"a second process opening the same file fails fast instead of
  mid-append"*). So the command **cannot** run against a live engine's journal
  anyway: it refuses at open.

Both point the same way and it is the honest MVP: **stop the engine, take the
act, boot again.** A §9 incident whose remedy is an accounting reconciliation is
not one an operator handles without stopping to look, and the stop-opening
posture means the safety ladder was never taken away while they did.

---

## 5. Where things live

| where | what |
|---|---|
| `oms/domain` | `MarginOpReconciliation` + `Validate` |
| `oms/application/ports` | `MarginOpJournal.AppendMarginOpReconciled`; `JournalVisitor.MarginOpReconciled` |
| `oms/infrastructure/driven` | kind + tag + encoder, the detail table and its two triggers, the replay row |
| `oms/application` | `rebuild.MarginOpReconciled`, the `report()` fold, `MarginOpReconciler` (the use case) |
| `oms` | `ReconcileMarginOp` + `ClearanceDeps` — a package function, **not** a `Module` method: it builds its own reconciler from the two paths it is given, so a `Module` field for it was added and then removed as unread (and the act needs no venue credentials, which is what a `Module` carries) |
| `app/reconcile` | `ReconcileMarginOp` composition |
| `cmd/arblab` | `arblab margin reconcile` |

---

## 6. Rejected alternatives

1. **A fifth `MarginOpStatus`.** §81 §5, unchanged: two questions in one
   vocabulary, and it would make the clearance re-derivable by the same fold
   that reaches the verdict.
2. **An operator-authored document + config digest pin** (the attestation's
   shape). §1: a clearance is an event, not a state; the lift would become a
   config edit; and a record whose whole content is a claim about a human act is
   exactly what belongs inside the chain.
3. **A structured operator FINDING on the record.** §2a: §7.1 says the
   attribution may be impossible, no reachable value of it would keep the block
   on, and its real consumer (`treasury`'s float ledger) is unbuilt.
4. **A bulk clearance (`--all`, or a date range).** §2: it satisfies the audit
   requirement while making it worthless.
5. **Expiring the ambiguity on a timer.** §7.1 and §7.5 both say *until
   explicitly reconciled*, and §81 §1a's third ground applies with more force to
   a timer than to a re-read: time makes the evidence more confounded, never
   less.
6. **Letting the clearance run against a live engine.** §4: the store refuses
   it, and the control plane that would do it properly is §7.3's, unbuilt.

---

## 7. The campaign — 29 mutants, and only ONE survivor was a missing test of a rule

| Claim | Verdict |
|---|---|
| `Validate` refuses a missing op / actor / instant / note (four) | caught |
| the adapter validates the act before writing | caught |
| the chain covers the ACTOR | caught |
| the chain covers the INSTANT | caught |
| the chain covers the NOTE | caught |
| the kind tag separates a clearance from a verdict | caught |
| the replay dispatches a clearance to the visitor | caught |
| the replay refuses a clearance for an op no dispatch created | caught |
| the replay refuses a SECOND clearance | caught |
| the replay refuses a clearance for a SETTLED op | caught |
| the fold SUBTRACTS a cleared ambiguity | caught |
| the use case validates the act FIRST | caught |
| the use case refuses a second / settled / in-flight / unknown op (four) | caught |
| `Remaining` is read back AFTER the act | caught |
| the composition validates the WHOLE config | caught |
| a malformed `-at` is `EX_USAGE`, not a refusal | caught |
| **the replay refuses a clearance for an op with NO verdict** | **SURVIVED → fixed → caught** |
| **the dispatcher ROUTES `margin reconcile`** | **SURVIVED → fixed → caught** |
| **the command requires the NOTE (and each other flag)** | **SURVIVED → fixed → caught** |
| **the composition validates what it does NOT read** | **SURVIVED → fixed → caught** |
| **the journal path and the account cannot be swapped** | **SURVIVED → fixed by the SIGNATURE** |

### 7a. Survivor 1 — two refusals sharing one sentinel

Deleting the no-verdict branch survived: the op falls through to the settled
check, whose status is the empty `MarginOpUnresolved`, so the **same sentinel**
comes back and only the message differs — `resolved : …`, with a blank where the
verdict belongs. The sentinel was the whole assertion. The repair adds a
`wantMessage` to the two cases the sentinel cannot separate, which is the general
shape: **when two conditions share an error, the wording is the claim, so the
wording is what the test must pin.**

### 7b. Survivors 2 and 3 — one CLI fixture, two collapses

`dispatch(["margin","reconcile"])` with no flags asserted `stderr` names
`-config`. Both survivors live in that one line.

- **Routing.** Breaking the route sends the call to the usage fall-through, which
  prints the usage block — advertising every flag this command takes — and also
  returns `64`. So the assertion could not see the variation at all. Fixed by
  asserting text **only this command emits**.
- **Per-flag requirement.** With every flag missing, the first check trips and
  the other four are dead code; deleting the `-note` requirement changed nothing.
  Fixed with a table that supplies a complete invocation and removes **one** flag
  per case — the one-violation-per-fixture discipline, applied to flags.

### 7c. Survivor 4 — a guard duplicating a downstream check

`cfg.Validate()` deleted still refused every case the tests exercised, because
the two fields the act actually reads have their own refusals downstream (an
empty path fails at `Open`, an empty account at the repository). The guard is
**kept**, on the direction it fails in: the act binds to *the journal this
deployment boots against*, and reading two fields out of a config nobody checked
is how an incident gets cleared in a leftover path no engine reads. The new
fixture is a config **complete in both fields and inconsistent elsewhere** — aim
at the boundary the mutant moves, not at the case the guard is named for.

### 7d. Survivor 5 — closed by the signature, not by a sixth test

`ReconcileMarginOp(journalPath, account string, …)` took two adjacent strings.
Swapping them compiles, and the failure is silent in the worst available way:
**SQLite creates the file it is asked to open**, so the act mints an empty
journal named after the subaccount, finds no dispatch record, and refuses with
*"this op was never dispatched"* — the identical message a mistyped op id
produces. An operator would be told their id was wrong about an incident that is
still open.

A test was written for it and then replaced by `ClearanceDeps`, a struct with
named fields, because the misuse should be **inexpressible** rather than merely
detectable. The one test kept is the positive control, rebuilt so that only the
configured path can satisfy it: the journal path is a **directory**, which cannot
be opened as a database, while the account is an ordinary name a stray file could
be created under. The first version of that control — "a valid config, expect a
refusal that is not the config's" — passed the mutant, because both wirings
produced the same refusal.

---

## 8. What is still owed after this

1. **Binance's `positionMargin/history` evidence half** (#214) — the reading is
   already ruled in §7.1 (v0.9.62): the "plus" is a fallback, not a conjunction.
2. **The dispatch path** (#215) — before-state read, fsync, `AddIsolatedMargin`,
   one op in flight per position. Both of the above block it.
