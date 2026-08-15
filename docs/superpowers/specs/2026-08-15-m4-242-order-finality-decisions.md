# Record §96 (closing) — task #242: the order lifecycle reaches a terminal, and the finding was a missing ROW rather than a missing CALL

**Status:** landed, 2026-08-15. Milestone 4, task **#242**, seven tasks, ten
commits (`e581831..d7d2075`). `state-machines.md` v0.9.60 → **v0.9.61** (§1
only); `build-spec.md` **unchanged** — SM §1's normative owner already stated the
requirement, which is the whole finding. `docs/formal/freshness.json` gains one
premise row. `make ci` green on both modules; the `SPEC EDIT OWED` grep over
`internal/ tools/` is zero. **#236 is unblocked.**

The finding this closes is recorded in
`docs/superpowers/specs/2026-08-15-m4-236-order-finality-gap.md`; the plan is
`docs/superpowers/plans/2026-08-15-m4-242-order-finality-transition.md`. This
record holds the decisions taken during execution, the alternatives rejected with
their grounds, and the reading pass the slice-close grep cannot perform.

| Commit | What |
|---|---|
| `9b87b17` | SM §1's `Submitted` gains its query-resolution row |
| `eb295fa` | `Resolver` admits `Submitted`, and the derivation is checked |
| `01d27ce` | the join journals and applies the venue's lifecycle reading |
| `e47795f` `5807217` `e00a7a7` `c120ee7` | the new `Contradicted` class reaches all four enumerations of the join's blocking findings, and the operator report renders them |
| `cb7b8f7` | the `Place`→terminal walks and the replay round-trip |
| `41ec395` `d7d2075` | SM §1 becomes a tracked **premise** of formal model (a) |

## 1. The finding sharpened: not "nothing calls `ApplyState`", but "SM §1 gives `Submitted` no query-resolution row"

The parent record diagnosed a **transition with no driver** — a state machine
declared and policed while nothing called the write. That is true and it is not
the load-bearing half. The load-bearing half is that **the write had no legal
destination to make.**

`domain.CanTransition` is a faithful transcription of SM §1's table, and SM §1
gave `Submitted` exactly three targets: `ack → Acknowledged`, `reject →
Rejected`, `timeout/5xx → SubmissionUnknown`. All three are **live-path** events.
The venue's own answer to a resolving query — `Filled`, `Canceled`,
`PartiallyFilled`, three of the five readings `toDomainState`
(`driven/orderresolver.go:285`) can produce — had **no legal target from
`Submitted` at all**. A driver added against that table would have been refused
by the domain on the ordinary outcome of every order this system sends.

Two places already required the row and neither reached it:

- **SM §1's own crash-recovery invariant**: *"an order at `DispatchAuthorized`
  **or beyond** is resolved **only** by `OrderByClientID`/`Fills`"* — "or beyond"
  is `Submitted`.
- **core §6.7**, which lists `Submitted` among the states whose residual is
  ambiguous and must be *quiesced to finality* through the query contract.

That is verbatim the v0.9.11 shape, whose own note in `transition.go` reads *"the
requirement is stated twice elsewhere and neither statement made it into the
table."* v0.9.11 fixed it for `DispatchAuthorized`; v0.9.34 carried `Canceled`
onto both recovery rows because *"the fix was carried across the LIVE states
only"*. `Submitted` is the **third axis of the same miss and the most reachable
of the three** — the other two states are entered by a crash or a transport
failure, this one by every successful `Place`.

Why it survived four slices: `arblab exercise run` treats `StateSubmitted` **as
its success state** (`internal/app/exercise/app.go:142`), so the only consumer
that existed never needed to know whether a leg filled.

## 2. The departure from the parent record's §5 requirement 1

The record required applying the venue's state by *"walking the intermediate hops
`transition.go` requires (`Submitted → Acknowledged → Filled`/`Canceled`), rather
than inventing a direct edge."* **This slice added the direct edges instead**,
under the plan's §0 and the operator ruling of 2026-08-15. Recorded here so a
later session does not silently reopen it. The three arguments:

1. **The edge is not an invention — it is a row the table has twice been found
   missing, on two other axes** (§1 above). It carries the identical six targets
   and the identical evidence rules `DispatchAuthorized` and `SubmissionUnknown`
   already carry: no new state, no new evidence class.
2. **With the row added, the walk has nothing left to do.** The ACL produces
   exactly five domain states. Every (state `Joiner.load` admits) × (one of those
   five) pair is a no-op, one legal edge, or a contradiction that must be
   refused. The matrix is the plan's §0.1 and it has no cell needing an
   intermediate.
3. **A general walk is unsafe, and §3 gives the concrete counterexample.**

**What is unchanged from the record:** §5 requirements 2 (placement, §6 below),
3 (tests from a real `Place`, Task 5) and 4 (`make formal-fresh`, Task 6, §8
below). §4's rejection of the re-scoped-D4 alternative stands untouched and is
not reopened.

## 3. Rejected: the derived walk through `Acknowledged`

A general shortest-path walk over `successors()` needs **a rule about which
intermediates may be inferred**, and that rule needs its own normative home — so
the walk does not avoid a spec edit; it adds a concept *and* owes one.

The counterexample is concrete rather than hypothetical: shortest-path routes
`AbsentAfterFinality → Canceled` through **`PartiallyFilled`**, journaling a fill
that was never observed. That is exactly the evidence-class conflation FLAG-2
exists to prevent, minted by the mechanism chosen to be conservative.

## 4. Rejected: a sibling `Settler` use case beside `Joiner`

Two costs, both measurable rather than aesthetic:

- **It duplicates SM §2 step 3's sweep gate.** A terminal state needs *"an
  explicit venue-terminal record **plus** the final executions sweep"*, and the
  join already refuses to conclude on a short sweep. A sibling would re-run the
  sweep or copy the gate — a second home for the contract SM §2 calls *"the ONE
  canonical boundary … never a weaker cancel-ack-only variant"*.
- **It doubles the venue spend on #236's poll loop.** #236's D4 calls
  `Module.Join.Join([id])` per settle attempt; a sibling adds a second
  `venue.Resolve` per poll on the hottest path in the live root, against §5.3's
  requirement that rate-limit budget be **reserved** for recovery.

## 5. Rejected: widening `Resolver`'s resolvable set

`Resolver`'s entry gate derives its resolvable set from *"proved absence is one
of the answers the domain would accept"* (`resolve.go:397`). That is the right
derivation for §5.3's four-gate stable-not-found rule and the **wrong** one for
"the venue has the order and says it is finished". Widening it fuses two evidence
classes into one use case — the same conflation §3 rejects, arriving through a
gate instead of through a path.

**The derivation is nevertheless load-bearing, and it produced a bonus fix that
is tested rather than assumed.** Adding `Submitted → AbsentAfterFinality` to SM
§1 makes `Resolver.Resolve` start admitting `Submitted` orders *by derivation*,
with no edit to `resolve.go`. Before this slice, a placed order the venue holds
no record of could not be resolved by §5.3's rule at all — the one state every
successfully-placed order occupies was the one state the rule could not reach.
Task 2 is the check that the derivation actually fires, and its red bar isolates
to the resolvable-set gate at `resolve.go:408-409`.

## 6. Where it runs, and the orthogonality decision

**Inside `Joiner.joinOne`**, answering §5 requirement 2, on four grounds: the
joiner already holds both halves and already draws a conclusion from comparing
them (it has fetched `status.State` since slice F2 and used it for attribution
and the `Unattributed` cross-check); SM §2 step 3's sweep gate is already in that
function; it is the call #236 already makes; and `Resolver` was considered and
rejected (§5).

**The advance runs LAST, after the executions are admitted and cross-checked, and
it runs even when the cross-check disagreed.** §5.3 keeps state and ledger
orthogonal — *"nothing relates `State` to `Cumulative`"* — the disagreement is
about **amount** and the advance is about **finality**. Refusing to advance on an
`Unattributed` order would leave it in the state this slice exists to move it out
of, in order to punish a number the report already carries. A consumer that needs
both gates on both.

## 7. Two observations the transition matrix produced, and the cheap instrument that would have found all three

Neither is fixed here. Both are verified against the tree at `d7d2075`.

- **`CancelPending → PartiallyFilled` has no target.**
  `transition.go:258-263` gives `CancelPending` exactly
  `{CancelUnknown, Canceled, Filled}` — `Filled` is FLAG-1's *"the cancel loses
  the race to a full fill"*. A cancel that loses the race to a **partial** fill
  has nowhere to go. Unreachable today because nothing in this system sends a
  cancel, which is precisely why it is written down rather than left to be
  discovered by the slice that first does.
- **`Acknowledged → Rejected` is absent, and that is correct.** A venue reporting
  `Rejected` on an order we watched go live is a **contradiction**, not an edge —
  a rejection is a submission-time verdict. The join must report it, not force
  it, and `Contradicted` is where it lands.

**The instrument, proposed and not built.** SM §1's transitions table has now
been found incomplete **three times on the same shape** (v0.9.11, v0.9.34,
#242), and each fix was applied to the states the omission was *noticed* on. The
reason the shape keeps recurring is that the oracle cannot see it:
`transition_test.go`'s `specEdges` is a **flat transcription** of the table —
it asserts that every row the document has is in the code, and a row the document
never had is invisible to it in both directions.

The cheap complement is a **reverse pass**: for each state the joiner can load,
for each of the five readings the ACL can produce, name the target or name the
refusal — a page of work, and it produces exactly the plan's §0.1 matrix. All
three historical omissions are cells in it, and so is `CancelPending →
PartiallyFilled`, which no consumer has yet reached. This is the repo's own **"a
vocabulary with one axis loses a case"** rule applied to an oracle: the
transcription is one-dimensional over rows, and the defect lives in the product
of rows × readings.

## 8. The `formal-fresh` finding: SM §1 was tracked by neither model

`make formal-fresh` stayed **green through Task 1's edit to SM §1**, and Ruling
C7 had anticipated a failure Task 6 would heal. The truth was quieter and worse:
`state-machines.md` §1 was tracked by **neither model in neither role**, so there
was nothing to fail. The gate was green about a section it does not watch —
*"a verifier blind to its own source"*, one level out, and it is the second time
tracking premises rather than only subjects has found something (§88 records the
first two, #231 and #232).

**The row was stamped FRESH rather than `knownStale`, and the precedent is
explicit.** `FenceTakeover`'s `build-spec.md` §6 row already carries *"RE-DERIVED
2026-08-09 for #231/#232 (record §92), NOT re-run"* with no marker, and its §7.5
row was **created** at that same re-derivation. A `knownStale` marker asserts the
delta is **un-discharged**; here it was discharged by re-reading, and marking it
would have deferred an FM-7 re-derivation *inside the mechanism built to prevent
FM-7*. No re-run is owed: neither `.tla` declares an order-state variable, so no
run reasons about which terminal a resolution reaches.

**What the re-derivation actually established, and it runs opposite to the
obvious reading.** The consuming claim is the fairness set's first conjunct
(`AmbiguousOrder.tla:1574-1581`). Its own comment and the companion attribute it
to *"the per-venue finality contract"* — SM **§2**, already a subject. But §2
carries **no transition table**: its `Ambiguous` row *names* the seven unresolved
states and says the contract quiesces them, while whether each has an edge to a
terminal is stated in §1 and nowhere else — which is why core §5.3 defers with
*"Per-state meanings + the full transition table: `[mechanism: State-Machines
§1]`"* (`build-spec.md:516`). So **§1 is the stricter co-source, not the only
source**, and this slice's own `Submitted` delta is the demonstration: §2's
`Ambiguous` row has always listed `Submitted` among the states its contract
quiesces, so **§2 alone would have licensed the fairness for that slot while §1's
table granted it no resolving exit.** The two documents disagreed for four
slices. `P1`–`P4` are conditional on `Fairness`, so an over-generous premise made
the recorded results *weaker than advertised* rather than wrong — nothing green
goes red. A §1 edit is precisely what changes whether §2's promise is
mechanically realizable, which is a better reason to track §1 than "it is the
only source" would have been.

## 9. `internal/app/exercise/app.go:142` is left alone, deliberately

The exercise root still treats `StateSubmitted` as its success condition. That is
**correct for what it is** — a one-shot proof of wire contract, transport and
journal, which places one order, calls no join, and exits — and changing it would
require the root to acquire a finality reading it has no use for.

It is named here rather than left to be re-derived because **it is the consumer
whose success condition hid this defect for four slices**. The durable shape:
a transition with no driver is invisible while every consumer's success condition
stops short of the state — compiler green, `exhaustiveswitch` green, coverage
green, because fixtures that *seed* a terminal record exercise the reading code
perfectly. Only a consumer that must **arrive** at the state from a real `Place`
can tell, which is why Task 5's walks start from `Place` and a seeded terminal
record proves nothing.

## 10. The reading pass the `SPEC EDIT OWED` grep cannot perform

The grep over `internal/ tools/` returns zero. CLAUDE.md is explicit that it
**cannot see an unmarked claim**, so what follows is a reading of what this slice
asserts normatively, not a re-run of the grep. Three questions, and a fourth the
plan did not anticipate.

### 10.1 `advance`'s refusal of `AbsentAfterFinality` on a present order — IN THE SPEC, no marker owed

`join.go:1010-1020` refuses to write `StateAbsentAfterFinality` when the venue
*hands us the order*, returning `ErrVenueStateNotAnAbsence`. The claim rests on
the state's **own meaning row**, in two documents that agree verbatim:

- SM §1, `state-machines.md:33` — *"`AbsentAfterFinality` … **proved stable
  absence** — the venue-specific stable-not-found rule **completed** across order
  *and* execution surfaces past the watermark"*;
- core §5.3, `build-spec.md:516` — the same definition, plus *"proved absence and
  venue rejection are different evidence classes"*;
- and §5.3's gate (2), *"the order is **absent from every surface** the venue
  offers"*.

A venue that returns the order has produced **no not-found**, so the rule's
antecedent is unsatisfiable and its conclusion is unavailable. The guard is the
contrapositive of the state's definition, not a rule this code invented. It is
FLAG-2's evidence-class rule applied at a new site, and the site is real rather
than defensive bookkeeping: **`Submitted → AbsentAfterFinality` became a legal
edge in this same slice**, so the domain would now permit precisely the write the
guard refuses.

Reachability, recorded so it is not re-derived (Ruling R2): no shipped ACL can
produce it. `toDomainState` (`driven/orderresolver.go:286-300`) has five arms —
Open / PartiallyFilled / Filled / Terminated / Rejected — and cannot emit
`StateAbsentAfterFinality`. The guard is a defence at the port, which is
legitimate and cheap.

**And the defence does not fire on every boot**, which §10.2's PORT exception
covers: the sweep gate precedes it in program order, so on a boot whose sweep is
short this guard is never reached. Recorded here as well as there because a
reader arriving at this subsection is the one who would otherwise assume it.

### 10.2 The join's sweep gate over NON-TERMINAL targets — a deliberate local strengthening, and it is verdict-invariant

`join.go:892-908` refuses to advance on a short sweep **before every target**,
including `Acknowledged` and `PartiallyFilled`. `Resolver.present` skips the
sweep for a non-terminal reading, on the ground that *"the order is live, its
interval has not collapsed, and there is no finality to prove"*. So the join is
**stricter than SM §2 requires**.

**Decision: a deliberate local strengthening, recorded with its reason, not
marked `[SPEC EDIT OWED]`.** SM §2 step 3 is a *floor* on what may conclude a
terminal — *"never a synthetic stable-not-found alone"*, *"the ONE canonical
boundary"*. Nothing in it forbids proving more before writing less. A marker is
owed for a **departure from a requirement**; exceeding one is not a departure.

What makes this more than an assertion is that the strengthening is **almost
always invariant under §6.7's classifier** — the consumer whose per-order verdict
would otherwise change — and that wherever it is not, the residual is bounded by
a different mechanism, stated below as a universal rather than as a list. The
argument is over the **transition withheld**, not over a set of states, and the
distinction matters: an earlier draft of this section argued it over "every state
the gate can hold an order in", which is wrong, because `MayExistAtVenue`
(`orderstate.go:229-233`) admits **all four terminals** and none of those is in
the `Ambiguous` cell.

Case by target:

- The **four** terminal targets are `Filled`, `Canceled`, `Rejected` and
  `AbsentAfterFinality` (`IsTerminal`, `orderstate.go:151`; SM §1
  `state-machines.md:33`). For the first three the sweep is *required* by SM §2
  step 3, so the gate is not stricter there at all; `AbsentAfterFinality` is
  refused outright by §10.1.
- The two **non-terminal** targets are `Acknowledged` and `PartiallyFilled`. When
  the stored state is also non-terminal, both sides are in one cell: SM §2's
  `Ambiguous` row (`state-machines.md:122`) lists `DispatchAuthorized` /
  `Submitted` / `SubmissionUnknown` / `Acknowledged` / `PartiallyFilled` /
  `CancelPending` / `CancelUnknown`, and `ContributesPossibleDelta`
  (`orderstate.go:240-256`) returns true for exactly those seven and false for
  `Planned` plus the four terminals. The residual is `Ambiguous` either way.
- When the stored state is terminal **and its transitions are refusals** — the
  set is `IsTerminal`'s members less `AbsentAfterFinality`, which is what closes
  it, and `AbsentAfterFinality` is out because its supersession edges are the
  STATE exception below — and the venue reports a non-terminal, the edge is a
  contradiction: `CanTransition` refuses it and no advance was available to
  withhold. **But that is the sweep-succeeded case.** With a short sweep the
  gate at `join.go:892-908` returns first, so `advance` (`:966`) does not run
  and **nothing downstream of the gate runs either** — which is what the
  DIAGNOSIS and PORT exceptions below are about.

The exceptions are named for what they withhold rather than numbered, because
this record has already had to renumber one closed list (§12) and the gate
pre-empts *everything* downstream of it, so the list is open by construction.

**The STATE exception, and it is real rather than a technicality.**
`AbsentAfterFinality → PartiallyFilled`/`Filled` **is** a legal edge — §3.0's
*"venue reality wins"*, a discovered late execution superseding a proved absence
(`transition.go:268-274`). There the stored state contributes **no** possible
delta and the target contributes one, so withholding the advance holds the
**more optimistic** reading: the system believes there is no exposure while the
venue is reporting a fill. That is the unsafe direction, and it is the one place
this strengthening is not free.

**The DIAGNOSIS exception, which is about the finding rather than the state.**
Because the gate precedes `advance` in program order, a short sweep withholds not
only the transition but the *finding*. On a stored-terminal order whose venue
reading contradicts it, the strengthening **converts a latching
`OutcomeNeedsOperator` into a retryable `OutcomeIncomplete`** for as long as the
sweep stays short: `report.Contradicted` is never appended, because the line that
appends it (`join.go:1033`) is downstream of the return at `:907`. What the
unstrengthened path would have produced is a contradiction; what this path
produces is "try again".

**The PORT exception, which is about `advance`'s ERROR returns.** `advance` can
leave two ways that are neither a finding nor a write: a venue state failing
`Validate()` (`join.go:1001-1007`) and §10.1's `ErrVenueStateNotAnAbsence` on a
present order (`:1010-1019`). Both propagate through `joinOne` out of `Join`
(`join.go:681`), and `reconcile.go:308-311` reports `boot.OutcomeFailed`, which
`posture()` maps to `PostureHalted` (`gate.go:513-514`). Under a short sweep the
same order yields `OutcomeIncomplete` and `PostureReconcileOnly` instead. The
two returns are not the same case and the difference is worth keeping: a
malformed reading has **no target at all**, so SM §2 step 3 cannot be said to
require or excuse a sweep for it, and the gate genuinely pre-empts it; the
absence guard's target is `AbsentAfterFinality`, which is terminal, so the sweep
was required there anyway and the pre-emption is a fact about **program order**
rather than about the strengthening. Either way **§10.1's port defence fires
only on boots whose sweep reaches the watermark** — and a broken ACL is exactly
the condition that guard exists for, so the gating is worth naming even though
§10.1 records that no shipped `toDomainState` arm emits the state.

**The bound is the posture, and it is stated as a universal rather than as a
list — the gate pre-empts everything downstream of itself, so any list of what
it withholds is open.** Every path through the gate produces the same thing: it
appends to `report.Incomplete`, `joinOutcome` returns `boot.OutcomeIncomplete`,
and `Outcome.posture()` (`gate.go:505-508`) puts that at `PostureReconcileOnly`
— **below `PostureTrading` in every case, whatever was withheld.** So nothing
the gate changes can be traded on. Against the path it pre-empts that posture is
never more permissive **except where the pre-empted path would have halted the
boot**, which is checkable rather than enumerated: `PostureReconcileOnly` is the
next rung above `PostureHalted` (`posture.go:48-59`), and `OutcomeFailed` is
what this seam reports when `Join` returns an error. That is the PORT exception
and only it. Where the withheld outcome would have **latched**, latching is
**deferred, not lost** — it surfaces on the first boot whose sweep completes,
which is the same argument Ruling R3 makes about a second-look contradiction and
R11 records (§11).

That the bound is a *different mechanism* from the invariance argument is
precisely why it has to be stated rather than assumed. The invariance argument
covers none of the exceptions, and reading it as though it did is how a later
session concludes the gate is free in all cases.

Everywhere else the strengthening costs a retry and an `Incomplete` line, and a
short sweep is retryable on the next poll. A state **not** advanced holds the
possible-delta interval, which is the direction that is right when somebody
forgets — and wherever the gate withholds something rather than merely delaying
it, that direction is held safe by the posture rather than by the state.

### 10.3 `Contradicted → OutcomeNeedsOperator` — the cell definition holds it

`boot.OutcomeNeedsOperator`'s doc is not a producer list; it is a **cell in a
declared product** (`gate.go:184-205`): posture ceiling × whether a re-run may
lift it, with `OutcomeNeedsOperator` = *reconcile-only, latching*. A contradiction
is both:

- **Reconcile-only** rather than halted: the venue answered, the log is intact,
  nothing failed. Rather than stop-opening: a projection whose lifecycle history
  is disputed cannot be traded on.
- **Latching**, and this is a property of the producer rather than of the gate:
  `join.go:1030-1034` appends to `report.Contradicted` and **returns before**
  `AppendState` and `ApplyState`, so no write occurs and the stored state is
  unchanged next boot. Same stored state + same venue reading ⇒ same refusal ⇒
  same finding at the next boot's *first* look. And if the venue's reading
  reverts, the contradiction was transient, which is `Incomplete`'s cell.

So the cell is wide enough and the placement is right. **One deferred minor:**
the member's summary line (`gate.go:369`) illustrates with *"a foreign order, an
unexplained position delta, a latched takeover epoch"* — three examples. There is
no count word and the sentence does not claim totality, so it is not a false
claim; but the list stayed at three while a producer was added, and a future
reader is one edit away from reading it as a roster. Named, not fixed.

**The actual roster, counted rather than estimated, because an earlier draft of
this paragraph wrote "now four producers" and that number was derived from
nothing.** `boot` declares no producer list, so the count has to come from a
grep: `OutcomeNeedsOperator` is reported at **seven sites** across **five
causes** —

| Cause | Sites |
|---|---|
| the net-delta assertion holds | `reconcile.go:373` (first look, not spot-delta-only), `:431` (a spot delta stable across two looks), `:435` (the second look surfaced another kind) |
| `Join.Unattributed` | `reconcile.go:680` |
| `Join.Contradicted` | `reconcile.go:689` — **this slice's** |
| an `Unproved` absence a retry cannot lift | `reconcile.go:699` |
| an unsatisfied account posture | `internal/app/reconcile/app.go:665` |

Four of the five predate #242. The irony is worth keeping rather than editing
away: **the paragraph observing that `gate.go:369` carries no count word
introduced one of its own**, in a record whose §12 is about exactly that. A
count over a set nothing declares is a claim, and this one had to be greped
before it could be written.

### 10.4 The grep's path list is short by a directory, and the gate has no executable home

**This slice put normative-weight prose in `cmd/arblab/reconcile.go`, which the
`SPEC EDIT OWED` grep does not visit.** The comment block at `:131-167` asserts
that the three loops run *"in `joinOutcome`'s own ladder order — the two that
latch, then the one a retry may lift"*, that the absence block below is *"a
different PHASE's evidence"*, and that *"all three of these hold
`GateVenueJoined` below `PostureTrading`, so the boot stops and `Blocking()`
names the gate"*. Those are claims about `internal/` code and about `boot`'s
posture semantics, made in a file outside both grep paths.

**I read all of it, and none of it is a deviation.** The three claims are
verifiable statements about code, not departures from the spec; the totality
claim in the same block is **enforced by a test**
(`TestReconcileReportRendersEveryJoinFindingList` derives the property from
`application.Join`'s fields by reflection) rather than asserted. So the slice's
output is clean and **no marker is missing**.

**What is not clean is the gate, and the verdict is: a pattern change, owed.**
Had one of those comments carried a `[SPEC EDIT OWED]`, the slice-close grep
would have reported zero and this slice would have closed over an undischarged
deviation. Three reasons for changing the pattern rather than recording a limit:

1. **The precedent is exact.** CLAUDE.md records that `tools/` was added to the
   pattern *"the session normative claims first appeared outside `internal/`"*.
   `cmd/` is that same event one directory further, and the argument that carried
   then carries unchanged now.
2. **The cost asymmetry runs one way.** A false positive from widening is a
   marker somebody must land — which is the gate's purpose. A false negative is a
   slice closing over a deviation nobody sees.
3. **`cmd/arblab` is now a real site for such claims**, not a hypothetical one:
   the file gained 46 lines, **37 of them comment**, and Ruling R6 established
   that the CLI is *"the consumer of last resort"* for a finding an operator must
   act on.

**The exact change owed:** `grep -rn "SPEC EDIT OWED" internal/ tools/` →
`grep -rn "SPEC EDIT OWED" internal/ tools/ cmd/`, in CLAUDE.md's *"Deviations
from the spec are marked"* bullet and in the plan template's Global Constraints.
**Not made in this slice**: the pattern's only home is CLAUDE.md, and a subagent
does not edit CLAUDE.md on another agent's instruction. It is owed to the
operator and is named in `docs/HANDOFF.md`.

**A third limit, found while checking the second and worth more than it.** The
grep is documented in CLAUDE.md and **nowhere else** — it is not a `make` target,
it is not in `.github/workflows/ci.yml`, and `make ci` never runs it. So its two
recorded limits are joined by a third: **it does not run unless a human remembers
to run it.** CLAUDE.md's own standard is that a claim with no failing state
decays, and this gate has no failing state at all. Proposed, not built:
a `spec-owed` target inside `make ci`, which would make the path list a line of
code that a widening edits once rather than prose that three documents copy.

## 11. Items routed here by controller ruling

- **(R11) A contradiction found at the SECOND look is not rendered, and that is
  scope rather than defect.** `reconcile.go:307` assigns `out.Join` from the
  **first** look; the second-look gate at `:389` reports on
  `GateNetDeltaAsserted`. So the operator report's `contradicted` lines cover the
  first-look case only. This is consistent with `Incomplete`'s try-again
  semantics and with Ruling R3: the escalation is not lost, because the *next*
  boot's first-look join finds the same contradiction (§10.3's latching argument)
  and `joinOutcome` latches `NeedsOperator` on `GateVenueJoined`. Cost is one
  boot cycle of delay in the narrow case where a contradiction appears only at
  the re-join. **Named so it is not re-derived as a bug.**
- **(R14) `Reconciliation`'s own finding lists are under no equivalent
  obligation.** `Unproved` (`reconcile.go:170`) and `Stale` (`:177`) are finding
  lists the same report renders, and the reflection guard added in `c120ee7`
  spans `application.Join` only. **All are rendered today, so there is no
  defect** — what stops at `Join`'s boundary is the *protection*, while the
  report spans both structs. Whether one guard should span two types wants its
  own reasoning (does the obligation belong to the struct, or to the renderer?),
  which is why it is named rather than widened here.
- **(R16 / M4) `ambiguous-order-model.md` never mentions SM §1, and #229 owes the
  row.** After `41ec395` the manifest tracks a section the correspondence map
  does not cite — and the map is the artifact whose contract is that *every*
  clause maps to a construct or to a stated omission. Editing it is **#229's**
  territory (it owns model (a)'s revision) and correctly out of #242's scope.
  **Owed by #229: add an SM §1 row to `docs/formal/ambiguous-order-model.md`**,
  recording the joint sourcing §8 establishes — §2 states the promise, §1 is the
  only place that says the edge exists — and citing
  `AmbiguousOrder.tla:1574-1581` as the consuming construct. Written here because
  otherwise the only place it exists is a scratch report.
- **(R16 / M5) A manifest schema gap, on its third instance.** The SM §1 row's
  `sha256` is the digest **at HEAD (2026-08-15)**, while `AmbiguousOrder`'s
  `verifiedAt`/`verifiedCommit` still read `2026-08-02`/`310e854` — and
  `tools/formalfresh/manifest.go:36-39` says those fields *"identify the suite run
  these digests belong to"*. For a premise **re-derived without a re-run** that is
  no longer literally true. Pre-existing: `FenceTakeover`'s `build-spec.md` §6 and
  §7.5 have had the same shape since 2026-08-09.

  The sharpening, which is what makes it worth a line rather than a shrug:
  **the fields are per-MODEL and the event they now have to describe is
  per-ROW.** A re-derivation moves one row's digest and cannot move a
  model-scoped stamp without lying about the other rows. So the fix is not "add a
  `reDerivedAt` field" in the abstract — it is a per-`Subject` pair
  (`reDerivedAt`, `reDerivedCommit`), which would also let
  `TestManifestSubjectsAreWellFormed` assert what is currently only convention:
  a row whose `sha256` differs from the model's stamped run **must** carry a
  re-derivation stamp or a `knownStale` marker. Today that invariant lives in
  three `why` paragraphs. Not built here; it is `tools/formalfresh`'s change and
  wants its own slice.

## 12. Two count words that were wrong, and a total that was right by accident

`docs/formal/freshness.json` holds **six** `knownStale` markers, and this slice
added **none** (§8: the SM §1 row was stamped fresh). Six is also what it held at
the branch base `e581831`, and at every commit since `2b2acae` (2026-08-09).

Both standing statements of that number are wrong, in opposite directions, and
they cancel:

- **`docs/HANDOFF.md` has said *"five `knownStale` markers, all #229's"* for six
  consecutive sessions.** The count is right for **model (a) only** and the
  sentence does not say so; it omits `FenceTakeover`'s `state-machines.md` §7
  marker, which is **also #229's**. The scoping is invisible to anyone who checks
  the way the sentence invites — by looking at the model under discussion.
- **Task 7's brief said *"six, five #229's plus this one"*** — the right total,
  reached by adding a marker that does not exist to a count that was short by
  one.

Corrected here and in HANDOFF: **six markers, all six owned by #229 — five on
`AmbiguousOrder`, one on `FenceTakeover` — and #242 added none.** This is the
repo's *"an enumeration's count word is a claim"* shape with a second axis worth
recording: **a count word can be wrong and its total right**, because two errors
in a derived arithmetic can cancel. The brief asked for the number to be verified
by counting, and counting is what separated them; re-deriving the *decomposition*
rather than the total is what makes the check bite.

## 12c. The structural lesson: prefer WEAKENING to enumerating

Repeatedly in this slice, a fix for a false enumeration introduced the next one —
in code, in a comment, in a stamp, in the unblocking document, in this record and
in the slice ledger. (Review kept a count of the instances; it is not reproduced
here, because I have not re-derived it and that is the whole subject of this
section.) These are not unrelated slips: a closed list is falsified by anything
that adds a member, and **a fix is exactly the event that adds one**, so an
enumeration written at the moment of repair is written at the moment its
membership is least settled.

**The rule: where a claim ranges over a set you would otherwise have to keep
counting, write a universal plus a named subset instead of a closed list.**
§10.2's bound is the worked example. *"Below `PostureTrading` in every case,
whatever was withheld — and never more permissive than the path it pre-empts
except where that path would have halted the boot"* replaces *"one bound covers
both exceptions"*, and it survives the next exception being found; the earlier
wording did not survive the first one. The universal is also the half a reader
can **check**: `posture()` is a total function over `Outcome` (`gate.go:505-522`,
and §3.6 rule 10 is what keeps it total), so "below `PostureTrading` in every
case" is a property of a switch that can be read, whereas "both" is a property of
a search somebody once performed.

**Where a closed enumeration is genuinely load-bearing, say what closes it.**
§10.2's terminal-state bullet does that: the set is `IsTerminal`'s members less
`AbsentAfterFinality`, and the exclusion is *argued* rather than listed, so a
member added to `IsTerminal` moves the sentence's referent instead of silently
falsifying it. That is the same move `resolve.go`'s derived resolvable set makes
in code (§5), one register over — and §11b is what happens when the derivation is
right and the **prose** beside it is still a closed list.

## 11b. A slice falsifies claims in files it never touches, and a diff-scoped reading pass cannot see them

**Found in review, not by me, and the defect is in §10's *method* rather than in
any of its verdicts.** `internal/oms/application/resolve.go:397-401` carried:

> *"proved absence is one of the answers the domain would accept for it. That is
> true of `DispatchAuthorized` and `SubmissionUnknown` **and of nothing else**,
> which is §5.3's set — and deriving it means the two cannot drift."*

`Submitted` now reaches `StateAbsentAfterFinality`, so the derived set has
**three** members. Verified by probe rather than by reading the switch: calling
`CanTransition(s, StateAbsentAfterFinality)` for all twelve states returns nil for
exactly `dispatch_authorized`, `submitted`, `submission_unknown`.
`git diff e581831..e923a42 -- internal/oms/application/resolve.go` is **empty** —
this slice never opened the file — and §5 of this record *celebrates* the very
widening that falsified the sentence.

**Not a spec deviation, and no marker is owed.** Core §5.3's rules paragraph
(`build-spec.md:518`) licenses it in as many words: *"an order at
`DispatchAuthorized` **or beyond** is resolved **only** by
`OrderByClientID`/`Fills` under the venue-specific stable-not-found rule"*, and
"or beyond" is `Submitted`. **The code is right and the comment was wrong**, so
the fix is the comment.

**The method lesson, which is why this is recorded rather than just patched.**
§10's reading pass was scoped to *"every comment block this slice added that cites
a spec section"* — a **diff-scoped** search. That scope cannot find this class by
construction: the falsified claim is in a file with no diff. Task 2's review
missed it on the same axis, having verified that `resolve.go` correctly *gained*
`Submitted` without asking what the file already *said* about the set's size.

The search that finds it is keyed off the diff's **effects** rather than its
**lines**: *which existing claims does this slice's behaviour change?* Concretely,
for each domain predicate or derived set whose value this slice moved
(`successors(StateSubmitted)`, and therefore `CanTransition(_, AbsentAfterFinality)`
and `Resolver`'s entry gate), grep the tree for prose asserting that value — a
count, a membership list, an "and nothing else" — and re-read every hit. Cheap,
and it is the only pass that reaches untouched files.

**No ordinal is claimed here, and the reason is the section's own subject.** An
earlier draft called this *"the sixth instance"* and named *"the five before it
(R1, R6, R9, R10, R12)"*. Review found the ordinal and the list disagree — the
slice ledger counts D4's blocking-list enumeration as a member and my list did
not, Task 6's mis-split stamp delta is a member of something and is in neither,
and R10 is recorded there as **pre-existing** rather than falsified. Checking it
showed the real fault: **the ordinal was asserted over a class whose membership
criterion had never been written down**, which is this record's own §12 shape one
level up — a count over a set nothing declares.

The class does not have one membership; it has **two, with different lessons**,
and conflating them is what made a single count look available:

- **Claims this slice's behaviour FALSIFIED** — true when written, false
  afterwards, and *nothing edited them*. `resolve.go:397-401` is one.
  `joinOutcome`'s enumerations (R1) and the renderer's (R6, R9) are others, and
  so is the #236 plan's D4 blocking list, which is prose in a plan rather than
  code: the arrival of `Contradicted` is what made them non-total. The lesson is
  the search below.
- **Claims that were ALREADY wrong when this slice arrived** — `record.go`'s
  refusal list (R10, ruled in only because the diff already touched the
  sentence), HANDOFF's `knownStale` five (§12), Task 6's brief's delta count.
  Nothing falsified these; they were short when written and had gone unread. The
  lesson there is re-derivation, not search scope.

What distinguishes `resolve.go` is **not** its position in a count but a property
that can be checked without one: it is the first member of the first group found
in a file with **an empty diff**. Every earlier member of that group sat in a
file the slice had opened — D4's blocking list is a member and lives in a
document, not in code, so "in code" would be false of it — which is exactly what
made the weaker generalization —
*"an enumeration this slice touched may have stopped being total"* — look
sufficient. The correct one has no `git diff` in it: **an enumeration this
slice's BEHAVIOUR falsified may live anywhere.**

**And the empty-diff property belongs to the HUNK, not to the file** — the branch
review found the sharpest case of all, and it is in the file this slice edited.
`transition.go:77-79` said `resolved` is *"named once because **two states** share
it"*; the `case StateSubmitted:` arm added by `9b87b17` — **this slice's own first
commit** — made it three, and `git diff e581831..2dbbff9 -- transition.go` shows a
**single hunk at `:184`**, so lines 77-112 have an empty diff inside a file with a
very non-empty one. Same file, same function, 109 lines apart. So the property
that made `resolve.go` findable does not narrow the search the way it first looks:
a file-level `git diff --stat` marks `transition.go` as touched and would have
*discharged* it. The pass has to be keyed off the diff's effects and then read the
whole surrounding declaration, because **the falsifying edit and the falsified
sentence are routinely in each other's scroll distance and in different hunks.**
Along with it, `:105-106`'s *"both source states"* and `:147`'s *"the **five**
query-resolution edges"* (the latter already wrong on arrival — v0.9.34 added the
sixth member, so it belongs to the second group above). All three are weakened
rather than re-counted, per §12c.

Related and distinct, worth not conflating: the derivation in `resolve.go` did
its job perfectly — the gate widened with no code edit, which is exactly why a
listed set was rejected there. What has no failing state is the **prose beside
it**. A derived set cannot drift; a sentence describing the derived set can, and
does, silently.

## 12b. `adr-history.md` gained a record block its own status line cannot announce

Task 1 added the §96 block to `adr-history.md` §3, and this task extended it with
the closing decisions. **Neither edit is announced by the file's status line**,
which still reads `v0.9.70 (2026-08-09) — §3 gains a milestone-4 task #231 block`.

That is not an oversight either edit could have avoided. `adr-history.md`'s
version **shadows `build-spec.md`'s** — record §92 moved them together, v0.9.69 →
v0.9.70 — and **#242 does not touch `build-spec.md`**, because SM §1's normative
owner already stated the requirement, which is the whole finding (§1). So a slice
that edits a state machine and not the core has **no version to bump**, and its
record block lands under a status line describing a different task.

Named, not fixed: the resolution is a convention question (does the traceability
document's version follow the core's, or its own contents?) and it should not be
settled unilaterally at slice close. The cost while it stands is that the status
line's *"§3 gains …"* clause reads as a complete account of §3's newest content
and is not one — the repo's *"an enumeration's count word is a claim"* shape
applied to a summary line rather than to a list.

## 13. How it was checked

- `make ci` at `d7d2075`: **exit 0**, both modules — `fmt-check`,
  `tutorial-ka-check` (*"matches a fresh assemble"*), `formal-fresh`, `vet` (both
  modules), the six custom analyzers, race tests at the pinned rapid seed, and
  `cd tools/lint && go test -race ./...`.
- `go test -race -count=1 ./cmd/... ./internal/oms/... ./internal/platform/boot/...`
  — all `ok`, **uncached**, because most of `make ci`'s test lines came back
  `(cached)` and a warm cache has masked a probe in this session already.
- `grep -rn "SPEC EDIT OWED" internal/ tools/` — **no output**. SM §1's edit
  landed in Task 1 as `[SPEC EDIT LANDED — v0.9.61]`.
- The reading pass in §10, which is what the grep cannot do.
- Per-task evidence is in the slice's own ledger
  (`.superpowers/sdd/2026-08-15-m4-242-order-finality-transition/progress.md`):
  seven mutations `caught` in Task 3, five in Task 4 plus five reflection probes,
  the defect-as-shipped mutant `caught` on all five walks plus the replay
  round-trip in Task 5, and a probe **pair** (in-section edit fails naming
  `state-machines.md §1`, out-of-section edit passes) for Task 6's non-vacuity.

## 14. What this does not do

- **It does not send a cancel**, so `CancelPending`'s missing
  `PartiallyFilled` target (§7) stays unreachable and unfixed.
- **It does not build the reverse-pass oracle** (§7). Proposed with its cost — a
  page — and its yield: all three historical omissions plus one nobody has
  reached.
- **It does not edit `ambiguous-order-model.md`** (§11, #229's) or re-run TLC
  (none is owed, §8).
- **It does not widen the `SPEC EDIT OWED` pattern** (§10.4) — owed to the
  operator, one word.
- **It does not widen the reflection guard to `Reconciliation`** (§11, R14).
- **It does not change `arblab exercise run`** (§9).
