# §49 — Milestone 3 slice B: the planning use case, and the signature that closed the hole

**Date:** 2026-08-02 (session 75)
**Task:** #161 (§47f slice B)
**Subject:** `internal/oms/application/planning.go`, `dispatch.go`, `errors.go`,
`application/ports/repository.go`, `internal/oms/module.go`, and their tests.
**Predecessor:** §48 (slice A — the durable journal adapter).

Slice B delivers §6 step 5: deterministic `ClientOrderID` minting, intents
fsynced to the write-ahead journal before any submission, `ApplyPlanned`'s first
caller, and the OCC version threaded into `Dispatch`. It also carries the
`Deps.Journal` → configuration conversion §48g item 1 assigned here.

---

## §49a — the fork §47f named, and the clause that settled it

§6 step 5 says: *"persist **and fsync** `TradePlanned` + intents to the
write-ahead journal **before any submission**"*. Two things are named. This
slice writes **one**.

The reason it writes one is easy and not sufficient: §5.1's aggregate is
`Trade → Orders[] → Fills[] → PositionAllocation`, and two of those four have no
domain type. A `TradePlanned` record would mean inventing `domain.Trade` in the
slice that owns neither it nor `PositionAllocation`.

**What makes the omission SAFE rather than merely cheap is a different clause of
the same sentence, and finding that is the slice's first result.** The question a
trade-level record answers is *was this trade's leg set complete?* — and the case
where that matters is a crash that leaves one leg filled and the other never
recorded. That case is unreachable under *"before any submission"*, because every
leg is journaled before `Plan` returns anything dispatchable. So the only state a
partially-journaled plan can be in is one where **nothing was sent**, and §5.3
prices that exactly: *"a `Planned`-only order was never dispatched (no venue
order can exist) → safe to replan or discard."* The question has no consequence
attached to it.

That argument has a premise, and the premise is a property of code rather than of
prose — which is this repo's standing rule about justifications (`FM-7`, and the
`P6` habit: *when a fix's correctness rests on a claim about a quantity, make it
a run*). So `Plan` is **phase-wise** — mint-and-check every leg, then journal
every leg, then apply every leg — and the property is asserted:
`TestPlanJournalsEveryLegBeforeApplyingAny` breaks the **last** leg's journal
write and requires the repository to be **entirely empty**. Under a per-leg
interleaving it holds two records, and the mutation confirms it (`caught`, and
the ordering test fails by name with *"the repository holds 3 records after a
failed journal write, want 0"*).

**The change that breaks the argument is named in the spec rather than left to be
rediscovered:** a planner that dispatches leg 0 before journaling leg 1. Then a
crash between them leaves a filled leg whose package reads as single-legged, and
the trade-level record becomes required. That sentence is the deliverable — a
narrowing carries an argument, and the argument is what a later slice needs.

### The rejected alternative, and why it was not merely "more work"

`AppendPlan([]domain.Order)` — one transaction, one fsync, an atomic plan — was
considered and refused. It is genuinely better on two axes (N fsyncs become one;
the truncated-plan state stops existing rather than being argued away). It was
refused because **nothing in the spec requires plan atomicity**, and inventing a
requirement is the same class of error as absorbing a deviation: it adds a port
method whose justification is a property nobody asked for, in a slice whose job
is to implement §6 step 5 as written. It is recorded here as available, not
refuted — the argument that would make it necessary is the leg-0-dispatch change
above, and that argument would carry it in.

---

## §49b — `Dispatch` takes an id, and this is the decision the slice is about

The signature was `Dispatch(domain.Order)`. It is now
`Dispatch(ids.ClientOrderID, int64)`, with the order **read from the write
repository**.

Until this slice there was no alternative: no planning path existed, so the
caller was the only source of an order. Now there is one, and keeping the old
signature would have left two holes open **with the mechanism to close them
sitting one field away**:

1. **An order that was never planned could be dispatched.** No journal record, no
   `ClientOrderID` uniqueness, nothing for §7.5's recovery to correlate — §6 step
   5's "before any submission" reduced to a convention about call order.
2. **An order could be mutated between planning and dispatch**, so the journal
   describes one order and the venue receives another. The journal is what §7.5
   replays; a divergence there is a recovery that reconstructs the wrong intent.

Neither is expressible now. An id the repository does not hold is
`ports.ErrOrderUnknown`, and there is no parameter through which a different
order could arrive. **The property is enforced by the type system rather than by
a check**, which is worth stating because it means no mutation can test it — the
claim is the signature. (This is the *a claim that cannot be mutated wants a
name* shape, taking the other branch: there the answer was to extract the claim
so it had a line; here the claim is structural and gets a paragraph instead.)

### One check disappeared, and it disappeared for the right reason

`admit` no longer calls `domain.Order.Validate`. It was correct when the caller
supplied the order; with the order coming from the repository — whose
`ApplyPlanned` refuses a malformed one — a `Validate` here is a branch **no test
can reach**. That is this repo's unreachable-sentinel shape, and leaving it in
would have been a false branch reading as a handled case.

The removal is only sound if the refusal is a **contract** rather than one
adapter's behaviour, so `ports.OrderWriteRepository.ApplyPlanned` now states it
as a requirement on implementations — the same kind of clause `Journal` uses for
durability and `ErrVersionConflict` uses for its sentinel, and the same kind the
§3.6 rule 2 assertion cannot check. The test double implements it too, with the
reason in a comment, because a fake that skipped it would have removed the check
from the system rather than from the double.

**The minted-id check stayed, and moving the source strengthened its reason.**
§6 step 10 routes foreign orders to operator clearance and the reconciler holds
them in this same repository — so *"we loaded it from our own store"* is
precisely **not** evidence that we minted the id. Same for the marketable-IOC
comparison: `Planner` mints only that pair, so the reachable case is now a
foreign order rather than a planning bug.

---

## §49c — the OCC ordering problem: a refusal that arrives after the commitment

§5.1 gives the record an OCC version; §6 step 6 wants **nothing** between the
`DispatchAuthorized` fsync and the single `Place`. Composing them is not free,
and the naive reading gets it backwards.

The naive reading: journal-then-apply is §6's rule, so `ApplyState` follows the
fsync — and `ApplyState` is where the OCC version is checked, so that is where a
stale caller is refused. **The refusal then arrives after the system has already
committed to submitting.** It can report the conflict; it cannot prevent the
order.

The resolution is that the version is **read and compared in `admit`**, before
the fsync, and **written after the `Place`**. Read-check-then-write is what OCC
is; putting the read on the pre-commitment side is what makes the refusal useful.
What remains — a write racing the window between the read and the write — is
caught by the `ApplyState`, which is to say caught *after* the send, and that
limit is stated in `admit`'s doc rather than left implicit. Serialising
`Dispatch` is the caller's, on this type's stated non-concurrency; the version is
what makes the loss **visible** rather than silent.

### The apply must not abort the send, and that is forced rather than chosen

If `ApplyState(DispatchAuthorized)` fails, the fsync has already happened. That
fsync **is** the commitment to call `Place` exactly once. Refusing there would
leave a durable `DispatchAuthorized` record for an order nothing ever sent — and
§5.3 then resolves it *by query only*, against a venue that never heard of it.
Being careful at that point manufactures a permanently unresolvable order.

So the send proceeds and the failure is reported through `Outcome.Applied`. The
**result** apply is then skipped outright rather than attempted against a guessed
version: the record is still at `Planned`, so every target the classification can
produce is a forbidden edge from it, and the attempt could only fail with a
transition error describing the gap rather than its cause.

### The in-process guard was NOT deleted, and the reason changed

The repository now also refuses a repeat — the record sits at
`DispatchAuthorized` and `CanTransition` answers `ErrTransitionNoOp` — which
looks like grounds to delete `d.authorized`. It is not, and the new reason is
sharper than the old one: **if the apply failed, the record is still at
`Planned`, so the repository would happily admit the order a second time, for an
order that has already been sent.** The map is the mechanism precisely when the
other mechanism is the thing that broke. `TestDispatchGuardHoldsWhenTheProjectionDidNot`
is that case, and `TestASecondDispatcherOverTheSameRepositoryIsAlsoRefused` is
the isolating probe for the other one — a fresh `Dispatcher` starts with an empty
guard set (§5.3 requires it: a restarted process queries rather than
re-dispatches), so what refuses there can only be SM §1's edge set. Two
mechanisms, two probes, neither reading as evidence for the other.

---

## §49d — THE FINDING: `FM-5`'s shape at the projection layer

`Outcome.Applied` began as a symmetric twin of `Outcome.Journaled` — a bool
reporting a store that did not take the write. Writing its doc comment is what
surfaced that the two "false"s are not the same kind of thing.

A false `Journaled` loses a **record**, and recovery is unharmed: the journal
still holds the authorization, which is the record §5.3 resolves an in-flight
order from. A false `Applied` loses a **reading** — and the reading is one §6.7
gates a compensating trade on.

**The specific hazard.** A failed apply leaves the order at `StatePlanned` in the
projection, and `StatePlanned` is the one non-terminal state that contributes
**no** possible-delta interval (`domain.ContributesPossibleDelta` says so, and
`OrderState`'s own doc calls the asymmetry "the whole point"). So an order that
was actually **sent** reads to the classifier as contributing nothing. That turns
an **ambiguous** residual into a **determined** one — and §6.7's determined
branch compensates *immediately, ≤5 s*, where the ambiguous branch would quiesce
to finality first. **The projection does not merely lag; it lags in the direction
that unblocks the action the ambiguity existed to hold.**

This is the `FM-5` family arriving in Go rather than in TLA⁺: *a rule evaluated
on a quantity read from a store that a different store is the authority for*. It
inherits the family's consequence unchanged — **no invariant computed from the
projection can detect it**, which is §46 §8e's rule (*an invariant built on the
ledger cannot audit the ledger*) one layer out. The report therefore has to be
this field, at the moment of the write, from the one caller that knows the send
happened.

**Landed normatively in §6 step 7** rather than left in a doc comment, because
the consequence belongs to a slice that does not exist yet: an apply failure on
an order whose dispatch reached the venue is a **hold-new-trading** condition,
not an alerting one, until that order's state is re-derived from the journal.
Recoverable rather than lost — the journal is written first and is what §7.5
replays. Task **#169** carries the implementation to slice E/F.

This is the reading pass earning its place beside the marker grep. The grep found
one marker (§6 step 5's `TradePlanned`); the reading pass found this, which
carried **no** marker, because nothing here departs from a spec clause — the
clause was simply absent.

---

## §49e — the collapse checklist, run before the mutations

Ten instances now. Five fixture parameters sat at values that would have made a
distinction unobservable, and the first is the one worth carrying forward.

1. **THE ATTEMPT, and it is a format collapse rather than a cardinality one.**
   `ClientOrderID` renders leg and attempt as **two decimal digits each**, so a
   plan at attempt 0 mints `…-00-00` for leg 0 — where *"leg, attempt"* and
   *"attempt, leg"* are **the same string**. A field-order bug is invisible at
   the value a fixture naturally takes. Every plan in the suite uses attempt 7,
   chosen not to equal any leg index in the fixtures. **The tell was the format,
   not the data**: two fields rendered identically, both plausibly zero.
2. **THE LEG COUNT.** At one leg, "mints per leg" and "mints once" agree, and the
   phase-ordering claim has no interleaving to be wrong about. Two separates
   them; the ordering test uses **three**, so a failure on the last leg still
   leaves two applies that must not have happened.
3. **THE SIDE.** Both legs buying makes "carries this leg's side" and "carries
   the first leg's side" the same answer. A cash-and-carry package buys spot and
   sells the perp — the realistic fixture is also the discriminating one.
4. **THE INSTRUMENT**, same argument, and it carries a second distinction free:
   the spot leg has no `ContractType` and the perp leg has one, so a `Plan` that
   copied one leg's instrument onto the other produces an order
   `VenueInstrument.Validate` refuses.
5. **THE VERSION**, in `dispatch_test.go`. Every record `ApplyPlanned` creates is
   at version 1, so a suite that only ever dispatches version-1 records cannot
   tell *"threads the caller's expected version"* from *"assumes the first
   version"* — they write the same two numbers. `plantAt` puts a record at
   version 5, and the applied-call log turns the separation into an assertion:
   the two writes must claim 5 and 6.

A sixth was found and then deleted rather than fixed: a module-level test
asserting only that a failed `New`'s error string was non-empty. It passes
against every implementation. The limit it was reaching for — that the journal
handle is released on a refusal — is not portably observable (`sql.DB.Close` is
idempotent and answers nil, so no `Close` failure can be produced), and that is
now a named gap in `module_test.go` instead of a check measuring nothing.

---

## §49f — bite-check results

**17 mutations, 17 kills.** Five needed re-aiming first: four `NOBUILD!!` and one
`NO-OP!!`.

| Claim | Verdict |
|---|---|
| leg index is the position in `Legs` | `caught` (9) |
| the plan's attempt reaches the id | `caught` (4) |
| §6 step 6 fixes the order type | `caught` (13) |
| §6 step 6 fixes the time in force | `caught` (2) |
| an empty plan is refused | `caught` (1) |
| only `ErrOrderUnknown` means the id is free | `caught` (1) |
| a known id is refused **before** it is journaled | `caught` (2) |
| **no leg is applied until every leg is journaled** | `caught` (9) |
| the OCC version is compared before anything is sent | `caught` (4) |
| SM §1 refuses a re-authorization | `caught` (3) |
| the result apply uses the version the authorization produced | `caught` (14) |
| `Applied` reports the result write, not the attempt | `caught` (2) |
| the result apply is skipped when the authorization never applied | `caught` (2) |
| the in-process double-dispatch guard | `caught` (3) |
| `Plan` and `Dispatch` share ONE repository | `caught` (1) |
| the journal opens at the CONFIGURED path | `caught` (5) |
| `Close` releases the journal `New` opened | `caught` (1) |

**All four `NOBUILD!!` were bad seds, not compiler-enforced claims**, and they
were the same sed each time: the mutation orphaned a variable
(`"declared and not used: authorized"`, `resultErr`, `applyErr`; and `"errors"
imported and not used`). §48/CLAUDE.md's exception — a `NOBUILD!!` that IS a kill
— applies to `var _ ports.X = (*Impl)(nil)`, where the compiler error names the
missing method. **The discriminator is the error text, and it is worth stating in
the general form: `does not implement` is the assertion doing its job;
`declared and not used` is Go's unused-variable rule reporting that the mutation
deleted the only reader of a value, which says nothing about the claim.** Each
was re-aimed to keep the variable read (`authorized.Version-1`,
`resultErr == nil || true`, `applyErr != nil || true`,
`errors.Is(...) || err != nil`) and each then killed.

The `NO-OP!!` was an off-by-one line number — the harness reporting exactly what
it exists to report.

**The phase-ordering mutation was verified by hand as well as by count**, because
9 failing tests is consistent with collateral damage rather than with the claim
being checked. Applied alone, `TestPlanJournalsEveryLegBeforeApplyingAny` fails
by name: *"the repository holds 3 records after a failed journal write, want 0"*.

---

## §49g — the gate re-verification, done by running rather than reading

§47f: *"slice B is the first code to compose the write repository, and
`singlewriter` already polices both the port and the adapter by position, so no
new gate is due. **This is a claim to re-verify at slice B, not to inherit.**"*

Re-verified, and in both directions:

- **The new import is permitted.** `internal/oms/module.go` now imports
  `oms/infrastructure/driven` — the first time anything does. `make lint` is
  clean, which is what says the gate reads `oms` as the permitted importer by
  position rather than by an enumerated list somebody would have had to extend.
- **The gate still bites**, on CLAUDE.md's **isolating** probe rather than on the
  one two gates can see: `internal/marketdata` (a context **root**, where
  `hexboundary` is silent because the importer is not an inward layer) importing
  `oms/application/ports`. It fires with the full diagnostic. Probe applied and
  removed; a clean run alone would have shown nothing.

**No new §3.6 gate is due.** The claim is now checked rather than inherited.

---

## §49h — what this slice did NOT do

1. **`ApplyFill` still has no caller.** §6's private order and execution stream
   handlers are slices C and D, and §5.3's one-transaction requirement for the
   fill path needs a transaction seam no port here declares — which is the
   requirement that chose SQLite in §48a.
2. **The hold-new-trading response to a false `Applied` is not implemented.** The
   condition is normative from this release (§6 step 7); the code reports it and
   nothing acts on it. Task #169, slice E/F.
3. **`Plan` does not roll back a partial apply**, and cannot: the journal is
   append-only, so undoing the applied legs would leave the two stores
   disagreeing about a trade the log says exists. What makes that acceptable is
   §5.3 — every applied record is at `StatePlanned`, hence discardable — and the
   caller's correct move is a fresh attempt, which the duplicate pre-check then
   **forces** rather than suggests.
   `TestPlanReportsAFailedApplyAndLeavesThePartialPlanDiscardable` pins it.
4. **Chain verification is still not implemented** (§48g item 3, unchanged), and
   §13's local-filesystem precondition is still task #168.
5. **No binary wires `oms`.** `app/capture`'s `imports_test.go` still forbids it,
   and there is no OMS binary yet. `Module.Close` exists and nothing calls it
   outside tests.

---

## §49i — what lands in the normative core

- **§6 step 5** — rewritten. Deterministic minting stated as the pure function it
  is, with the attempt advancing per **plan** (the direction that is right when
  somebody forgets: §5.3's danger is reuse, and a per-leg field lets a replan
  advance a subset). The `TradePlanned` record is recorded as awaiting §5.1's
  `Trade` aggregate, **with the ordering clause that makes the omission safe and
  the change that would break it**. Marketable-IOC fixed at minting. Dispatch
  reads what it sends from the write repository.
- **§6 step 7** — gains the projection-soundness clause (§49d): a
  journal-then-apply failure after the send is a **refuse-to-trade** condition,
  the failure direction is the unsafe one, and no invariant computed from the
  projection can detect it.
- **§6's sequence diagram** — the `TradePlanned` line becomes *"fsync intents +
  client IDs — ALL legs, before any send"*.
- **§12 milestone 3** — the slice-B line gains its dated annotation.
- **§5.1, §5.3, §7.5** — **no change.** The slice implements them as written.
- **ADR §3** — dated block.
- **Doc set → v0.9.30.**
