# Task #242 — The Order Finality Transition Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make an order this system places reach a terminal state, by giving SM §1's
transitions table the query-resolution row `Submitted` never had and journaling the
venue's already-fetched lifecycle reading from inside §7.5's join.

**Architecture:** Two changes, one in each of the two layers that were silent.
The **domain** gains one edge group: `Submitted` gets the same six query-resolution
targets `DispatchAuthorized` and `SubmissionUnknown` already carry, which is a
`[SPEC EDIT LANDED]` to `docs/state-machines.md` §1 and to `successors()`. The
**application** gains one step at the end of `Joiner.joinOne`: a single
journal-then-apply of `status.State`, the venue reading the joiner has fetched
since slice F2 and never written. No new use case, no new port, no walk — with the
missing row in place, every (stored state × venue reading) pair the ACL can produce
is a no-op, one legal hop, or a genuine contradiction that must be refused.

**Tech Stack:** Go 1.x, two modules (root + `tools/lint`). `make ci`, `tools/mutate.sh`,
`go test -race`. No new dependencies.

**Spec:** `docs/superpowers/specs/2026-08-15-m4-236-order-finality-gap.md` (the finding,
the evidence chain, the two rejected alternatives, §5's four requirements). Read it first.
This plan **departs from that record's §5 requirement 1** and §0 below states why.

---

## §0 The one departure from the record, stated before anything else

The record's §5 requirement 1 says to apply the venue's state by *"walking the
intermediate hops `transition.go` requires (`Submitted → Acknowledged → Filled` /
`Canceled`), rather than inventing a direct edge."*

**This plan adds the direct edges instead**, on evidence the record did not have.
The operator ruled for this on 2026-08-15 after the evidence below was put; it is
recorded here because a later session must not silently reopen it.

1. **The edge is not an invention — it is a row the table has twice been found
   missing, on two other axes.** `docs/state-machines.md` §1 gives `Submitted`
   exactly three targets (`ack → Acknowledged`, `reject → Rejected`,
   `timeout/5xx → SubmissionUnknown`) and **no query-resolution row at all**,
   while:
   - SM §1's own invariants say *"an order at **`DispatchAuthorized` or beyond** is
     resolved **only** by `OrderByClientID`/`Fills`"* — "or beyond" is `Submitted`;
   - core §6.7 lists `Submitted` among the states whose residual is ambiguous and
     which MUST be *quiesced to finality* through the query contract.

   That is verbatim the v0.9.11 shape, whose note in `transition.go:129` reads
   *"The requirement is stated twice elsewhere and neither statement made it into
   the table."* And v0.9.34's note one paragraph up says of the v0.9.11 fix:
   *"the fix was carried across the LIVE states only."* This is the third axis of
   the same miss, and CLAUDE.md's **"a fix applied in one place gets applied to
   its mirror"** names it.

2. **With the row added, the walk has nothing left to do.** The ACL produces
   exactly five domain states — `driven/orderresolver.go:285`'s `toDomainState`
   maps `Open → Acknowledged`, `PartiallyFilled`, `Filled`, `Terminated → Canceled`,
   `Rejected`, and nothing else. Every pair of (a state the joiner can load) × (one
   of those five) is then a no-op, one legal edge, or a contradiction that must be
   refused. The matrix is in §0.1.

3. **A general walk is unsafe, and the counterexample is concrete.** Shortest-path
   over `successors()` would route `AbsentAfterFinality → Canceled` through
   `PartiallyFilled` — journaling a fill that was never observed, which is exactly
   the evidence-class conflation FLAG-2 exists to prevent. A walk therefore needs
   its own rule about which intermediates may be inferred, and that rule needs its
   own normative home. So the walk does **not** avoid a spec edit; it adds a
   concept **and** owes one.

**What is unchanged from the record:** §5 requirements 2 (the placement decision —
answered "inside the join", §0.2), 3 (tests that start from a real `Place`, Task 5)
and 4 (`make formal-fresh`, Task 6). §4's rejection of the re-scoped-D4 alternative
stands untouched and is not reopened here.

### §0.1 The transition matrix this plan relies on

Rows are the states `Joiner.load` admits (`MayExistAtVenue` is true). Columns are the
five readings `toDomainState` can produce. `·` = no-op (already there), `→` = one
legal edge, `✗` = no edge, so the join reports it as a contradiction.

| stored ↓ / venue → | Acknowledged | PartiallyFilled | Filled | Canceled | Rejected |
|---|---|---|---|---|---|
| `DispatchAuthorized` | → | → | → | → | → |
| **`Submitted`** (after this task) | → | **→** | **→** | **→** | → |
| `SubmissionUnknown` | → | → | → | → | → |
| `Acknowledged` | · | → | → | → | ✗ |
| `PartiallyFilled` | ✗ | · | → | → | ✗ |
| `CancelPending` | ✗ | ✗ | → | → | ✗ |
| `CancelUnknown` | ✗ | ✗ | → | → | ✗ |
| `Filled` / `Canceled` / `Rejected` | ✗ | ✗ | ·/✗ | ✗/· | ✗/· |
| `AbsentAfterFinality` | ✗ | → | → | ✗ | ✗ |

The bolded cells are the whole defect: today a placed order sits at `Submitted` and
three of its five possible venue answers have no legal target.

**Two observations this matrix produced, neither of which this task fixes.** Record
them in the decision record (Task 7), do not act on them here:

- `CancelPending → PartiallyFilled` is absent while `CancelPending → Filled` exists
  ("the cancel loses the race to a full fill"). A cancel that loses the race to a
  *partial* fill has no target. Nothing in this system sends a cancel yet, so it is
  unreachable today — which is precisely why it should be written down rather than
  discovered later.
- SM §1's `Acknowledged → Rejected` absence means a venue reporting `Rejected` on an
  order we saw live is a **contradiction**, not an edge. That is correct — a
  rejection is a submission-time verdict — and the join must report it rather than
  force it.

### §0.2 Where it runs: inside the join, not as a sibling use case

The record's §5 requirement 2 left this open and named the `Unattributed` check's
placement as evidence. Answered: **inside `Joiner.joinOne`**, on four grounds.

1. **The joiner already holds both halves and already draws a conclusion from
   comparing them.** `join.go:801` fetches `status` (which carries `State`) and
   `join.go:896` compares `rec.Cumulative` against `status.Executed`. The venue's
   terminal state has been in hand there for four slices.
2. **SM §2 step 3's sweep gate is already in this function.** A terminal state needs
   *"an explicit venue-terminal record **plus** the final executions sweep"*, and
   `join.go:853` already refuses to conclude on a short sweep. A sibling use case
   would have to re-run the sweep or duplicate the gate — a second home for the
   contract SM §2 calls *"the ONE canonical boundary"*.
3. **It is the call #236 already makes.** The blocked plan's D4 polls
   `Module.Join.Join([id])` per settle attempt. A sibling use case would add a
   second `venue.Resolve` per poll on the hottest path in the live root, against
   §5.3's requirement that rate-limit budget be **reserved** for recovery.
4. **`Resolver` was considered and rejected.** Its entry gate derives the resolvable
   set from *"proved absence is one of the answers the domain would accept"*
   (`resolve.go:397`), which is the right derivation for the four-gate
   stable-not-found rule and the wrong one for "the venue has the order and says it
   is finished". Widening it would fuse two evidence classes in one use case.

**Consequence of the derivation, which is a bonus fix and must be tested rather than
assumed (Task 2):** `Submitted → AbsentAfterFinality` becomes a legal edge, so
`Resolver.Resolve` starts admitting `Submitted` orders. Today a placed order the
venue holds no record of can never be resolved by §5.3's rule at all.

---

## Global Constraints

Copied from `CLAUDE.md` and the spec set. Every task's requirements implicitly
include this section.

- **No `float64` under `internal/`.** Type-based `nofloat` gate, `//nofloat:allow <reason>` escape.
- **No `time.Now()`** in `domain/`, `application/` or `engine/`. Clock is injected (`platform/clock`).
- **`domain/` and `application/` may not import `infrastructure/` or `engine/`** (`hexboundary`).
- **Every switch over a module-declared enum-like names every member** (`exhaustiveswitch`); a `default:` is not an excuse.
- **Only `oms` may import `oms/application/ports` and `oms/infrastructure/driven`** (`singlewriter`), `app/` roots included.
- **Journal-then-apply, in that order**, and legality is established **before** the journal (`ports.Journal` is a recorder and holds no copy of the edge set).
- **Deviations from the spec are marked, not absorbed.** `[SPEC EDIT OWED]`; slice-close gate is `grep -rn "SPEC EDIT OWED" internal/ tools/` over **both** paths.
- **No AI/model attribution trailers** in commit messages or any authored content.
- **Doc versions at the start of this task:** `build-spec.md` v0.9.70, `state-machines.md` v0.9.60. This task bumps **state-machines.md to v0.9.61** and does **not** edit `build-spec.md` (SM §1's normative owner, core §5.3, already states the requirement — see §0).
- Verify with `make ci` (fmt-check, tutorial-ka-check, formal-fresh, vet, lint, race tests, both modules).

## The fixture-collapse checklist, run BEFORE the mutations

CLAUDE.md: *"before believing a suite, find the fixture parameter pinned to the value
that collapses the distinction the code makes, and vary it."* Eleven prior instances.
These are the eight parameters that collapse **this** slice. Each task's tests are
written against them; the reviewer checks them off.

| # | Pinned value that collapses | What becomes untestable | The variation |
|---|---|---|---|
| 1 | One order per join | "advances every order" = "advances the first" | ≥2 orders with **different** venue readings in one `Join` |
| 2 | Venue always reports a terminal | "journals the venue's reading" = "journals a terminal" | one fixture where the venue reports `Acknowledged` (non-terminal) and the state still advances |
| 3 | Zero fills before the state write | "threads the post-fill version" = "threads the loaded version" (`ApplyFill` bumps it) | at least one fixture that admits a fill **and** advances |
| 4 | Stored state ≠ venue state in every fixture | the `rec.State == status.State` short-circuit is never taken; a re-read would surface as `ErrTransitionNoOp` filed under `Contradicted` | the second-boot fixture: record already `Filled`, venue says `Filled` |
| 5 | Contradiction probe that another gate also refuses | "the edge check objected" = "something objected" | record `Acknowledged` + venue `Rejected`: both states valid, presence `has_order`, sweep complete — **only** `CanTransition` can refuse |
| 6 | Asserting only the projection | a mutation deleting `AppendState` leaves the projection right | assert the journal record **and** the projection state, separately |
| 7 | Absent/Incomplete fixtures asserting only the report | "did not advance" = "the report field is empty" | assert the journal recorded **nothing** on those paths |
| 8 | Incomplete-sweep fixture whose venue reading would not advance anyway | "gated by the sweep" = "nothing to advance" | the short-sweep fixture's venue reading is `Filled` — it *would* advance if ungated |

**Positive control, always.** Every table that asserts a refusal also asserts the
healthy walk succeeds in the same run, or a universally-objecting predicate reads as
a perfect score.

## File Structure

| File | Change | Responsible for |
|---|---|---|
| `internal/oms/domain/transition.go` | modify (`successors`, `StateSubmitted` case) | SM §1's edge set — gains `Submitted`'s query-resolution targets |
| `internal/oms/domain/transition_test.go` | modify (`specEdges`, two counts) | the flat-list oracle, transcribed from the table |
| `docs/state-machines.md` | modify (§1 transitions table + note; version header) | the normative mechanism |
| `docs/adr-history.md` | modify (new §3 block + traceability rows) | decision → resolution traceability |
| `internal/oms/application/resolve_test.go` | modify (two lists) | the resolvable-set derivation's consequence |
| `internal/oms/application/join.go` | modify (`Joiner`, `NewJoiner`, `Join` report, `joinOne`, new `advance`) | journaling the venue's lifecycle reading |
| `internal/oms/application/errors.go` | modify (one new sentinel, one message reword) | the use-case layer's refusals |
| `internal/oms/application/reconcile.go` | modify (`joinOutcome`) | the boot posture ladder |
| `internal/oms/module.go` | modify (one call site) | composition |
| `internal/oms/application/join_test.go` | modify (helper + tests) | the join's unit claims |
| `internal/oms/application/join_spot_test.go`, `reconcile_spot_test.go` | modify (call sites) | constructor arity |
| `internal/oms/application/finality_test.go` | **create** | the Place → terminal integration walks |
| `docs/formal/freshness.json` | modify (one premise row) | model (a)'s tracked premises |
| `docs/HANDOFF.md`, `docs/superpowers/specs/2026-08-15-m4-242-*.md` | modify / create | state and decision record |

`join.go` is 1218 lines and gains ~60. It is not split: the new step's whole
justification is that it sits beside the venue reading and the sweep gate that
already live in `joinOne`, and moving it to its own file would separate the code
from the two facts that place it.

---

## Task 1: SM §1's `Submitted` gains its query-resolution row

**Files:**
- Modify: `internal/oms/domain/transition.go:186-187`
- Test: `internal/oms/domain/transition_test.go:24-92` (`specEdges`), `:113`, `:143`
- Modify: `docs/state-machines.md` (§1 transitions table; the `**Status:**` line)
- Modify: `docs/adr-history.md` (a new block in §3, above the v0.9.70 block at line ~257)

**Interfaces:**
- Consumes: `domain.OrderState`, `domain.CanTransition`, `successors` (all existing).
- Produces: no new symbols. What later tasks rely on is the **edge set**:
  `CanTransition(domain.StateSubmitted, X) == nil` for
  `X ∈ {Acknowledged, PartiallyFilled, Filled, Canceled, Rejected, AbsentAfterFinality, SubmissionUnknown}`.

- [ ] **Step 1: Write the failing test — extend the transcribed oracle**

In `internal/oms/domain/transition_test.go`, replace the `Submitted` group in
`specEdges` (currently three entries at lines ~52-56) with:

```go
	// | Submitted | ack / reject / timeout-5xx | Acknowledged / Rejected / SubmissionUnknown |
	"submitted->acknowledged",
	"submitted->rejected",
	"submitted->submission_unknown",

	// OWED — the query-resolution row on the THIRD axis. SM §1's invariants say
	// an order at "DispatchAuthorized OR BEYOND" is resolved only by
	// OrderByClientID/Fills, and core §6.7 lists Submitted among the states that
	// must be quiesced to finality by that same contract. Neither statement made
	// it into the table, which is v0.9.11's sentence about DispatchAuthorized
	// arriving one state later. §6 step 6 sends nothing but marketable IOCs and
	// SM §2 step 1 makes an IOC immediately terminal at the venue, so the single
	// most likely thing a query finds about a Submitted order is a TERMINAL one —
	// and three of the five readings the ACL can produce had no legal target.
	"submitted->partially_filled",
	"submitted->filled",
	"submitted->canceled",
	"submitted->absent_after_finality",
```

Then update both counts, which are the transcription's own checks:

```go
	if got := len(legal); got != 36 {
		t.Fatalf("transcribed %d edges, want 36 — the count is a check on the transcription", got)
	}
```

and, in the positive control at the bottom of `TestCanTransitionMatrix`:

```go
	if permitted != 36 || refused != 12*12-12-36 {
		t.Errorf("walked %d permitted and %d refused pairs, want 36 and %d",
			permitted, refused, 12*12-12-36)
	}
```

- [ ] **Step 2: Run it to make sure it fails**

Run: `go test ./internal/oms/domain/ -run TestCanTransitionMatrix -v`
Expected: FAIL, four lines of the form
`CanTransition(submitted, filled) = ... transition forbidden, want nil (SM §1 has this edge)`.

- [ ] **Step 3: Add the edges to `successors`**

In `internal/oms/domain/transition.go`, replace the `StateSubmitted` case:

```go
	case StateSubmitted:
		// [SPEC EDIT LANDED — v0.9.61] This row carried the three LIVE-path
		// answers — an ack, a rejection, a transport timeout — and no
		// query-resolution row at all, while the two states either side of it
		// carry one. It is v0.9.11's finding on a third axis and v0.9.34's
		// sentence applies unchanged: the fix was carried across the states it
		// was noticed on.
		//
		// The requirement is stated twice elsewhere and neither statement made it
		// into the table. SM §1's own invariants: "an order at DispatchAuthorized
		// OR BEYOND is resolved only by OrderByClientID/Fills under the
		// venue-specific stable-not-found rule" — "or beyond" is this state.
		// Core §6.7 lists Submitted among the states whose residual is ambiguous
		// and which must be QUIESCED TO FINALITY through that same contract.
		//
		// What its absence cost is not a wrong number, it is a lifecycle with no
		// exit. §6 step 6 sends nothing but marketable-IOC limits and SM §2 step 1
		// makes an IOC "immediately terminal at the venue", so a query about a
		// successfully-placed order ordinarily finds it FILLED or CANCELED — and
		// three of the five readings §4's ACL can produce had no legal target
		// here. The order stayed Submitted for the life of the process,
		// ContributesPossibleDelta answered true for it for ever, and every
		// consumer whose success condition required finality was unsatisfiable
		// while nothing reported an error. The one consumer that existed —
		// `arblab exercise run` — treats Submitted AS its success state, which is
		// why four slices passed over this.
		//
		// The evidence rules are the other two rows' and this row does not weaken
		// them: an explicit venue rejection → Rejected; an explicit venue-terminal
		// record PLUS the final executions sweep → Canceled (SM §2 step 3, never a
		// status read alone); the stable-not-found rule COMPLETING →
		// AbsentAfterFinality, never mapped onto Rejected.
		return append([]OrderState{StateAcknowledged, StateSubmissionUnknown}, resolved...), nil
```

Note `StateRejected` is dropped from the literal because `resolved` already
contains it — appending it twice would put a duplicate in the returned slice.

- [ ] **Step 4: Run the domain suite**

Run: `go test -race ./internal/oms/domain/ -v`
Expected: PASS. `TestCanTransitionMatrix` walks 36 permitted and 96 refused pairs.

- [ ] **Step 5: Bite-check the edge group with a mutation**

The claim is *"`Submitted` reaches the query-resolution targets"*. Remove the
append and confirm the suite fails:

```bash
tools/mutate.sh internal/oms/domain/transition.go 186 \
  's#return append(\[\]OrderState{StateAcknowledged, StateSubmissionUnknown}, resolved...), nil#return []OrderState{StateAcknowledged, StateSubmissionUnknown, StateRejected}, nil#' \
  'Submitted keeps only its live-path targets' ./internal/oms/domain/
```

Expected: `caught`. Re-read the line number against the file first — `transition.go`
gains comment lines in step 3, so 186 is the pre-edit number and the `case` moves.

- [ ] **Step 6: Land the spec edit in `docs/state-machines.md`**

Two edits, both in §1.

**(a)** In the transitions table, immediately after the three `Submitted` rows, add:

```markdown
| `Submitted` | query (`OrderByClientID`/`Fills`) resolves the order | `Acknowledged` / `PartiallyFilled` / `Filled` / `Canceled` / `Rejected` / `AbsentAfterFinality` | **the third axis of the row `DispatchAuthorized` and `SubmissionUnknown` both carry, and the one this table never grew.** Same six outcomes and same evidence rules: an explicit venue rejection → `Rejected`; an explicit venue-**terminal** record **plus the final executions sweep** → `Canceled` (§2 step 3, never a status read alone); the stable-not-found rule **completing** → `AbsentAfterFinality`, never mapped onto `Rejected` `[→ R1d-F2]`. The requirement is stated twice outside this table and reached neither of its rows: this section's own crash-recovery invariant resolves "an order at **`DispatchAuthorized` or beyond**" by query only, and core §6.7 lists `Submitted` among the states whose residual must be **quiesced to finality** by that contract. Without the row a *successfully placed* order has no exit at all: §6 step 6 sends nothing but marketable-IOC limits and §2 step 1 makes an IOC "immediately terminal at the venue", so the ordinary answer to a query about a `Submitted` order is `Filled` or `Canceled` — and neither was a legal target `[→ M4 #242]` |
```

**(b)** After the existing `**The two query-resolution rows gained `Canceled` in
v0.9.34…**` paragraph, add:

```markdown
**`Submitted` gained the query-resolution row itself in v0.9.61, and it is the same
omission a third time `[→ M4 #242]`.** v0.9.11 gave the row to `DispatchAuthorized`
because "read as the table alone, an order crash-recovered at the dispatch boundary
has no way out"; v0.9.34 gave `Canceled` to both recovery rows because "the fix was
carried across the live states only". Both notes describe *this* row's absence
exactly, and it is the **most** reachable of the three: `DispatchAuthorized` and
`SubmissionUnknown` are reached by a crash or a transport failure, while `Submitted`
is where **every** successful `Place` lands. The failure mode is the one both earlier
notes name — the order contributes a possible-delta interval for ever, §6.7 holds its
hedge group `Ambiguous`, and nothing anywhere reports an error — with one addition
that made it survive four slices: the only consumer that existed treated `Submitted`
**as its success state**, so no test and no gate had a reason to look past it. **A
declared transition with no driver is invisible while every consumer's success
condition stops short of it**, which is the enum-member-with-no-producer shape one
layer out — the member has a producer here; what was missing was a row and a caller.
```

Then update the `**Status:**` line at the top of `docs/state-machines.md` to
`v0.9.61 (2026-08-15)` with a one-sentence headline, keeping the previous status
sentence after it in the existing "Previously:" style.

- [ ] **Step 7: Record it in `docs/adr-history.md`**

Add a new block in §3 **above** the v0.9.70 block (the block set is descending after
v0.9.8 — see the note at line 2425), following the existing shape: a bolded header
naming the version and task, then a prose paragraph, then a `| Decision → resolution | Where |`
table with one row:

```markdown
| **SM §1's `Submitted` gains the query-resolution row**, with the same six targets and the same evidence rules `DispatchAuthorized` and `SubmissionUnknown` carry. The requirement is stated twice outside the table — SM §1's own "`DispatchAuthorized` **or beyond** is resolved only by query" invariant, and core §6.7's ambiguous-residual list, which names `Submitted` — and neither statement reached a row. This is v0.9.11's finding on its third axis, and the **most reachable** one: the other two states are entered by a crash or a transport failure, this one by every successful `Place`. §6 step 6 sends only marketable IOCs and §2 step 1 makes an IOC immediately terminal at the venue, so the ordinary answer to a resolving query about a `Submitted` order is `Filled` or `Canceled` — three of the five readings §4's ACL produces had no legal target, and the order stayed `Submitted` for the life of the process with every gate green. Survived four slices because the only consumer, `arblab exercise run`, treats `Submitted` **as its success state**. | SM §1 transitions |
```

- [ ] **Step 8: Confirm no spec debt is left unmarked, then commit**

```bash
grep -rn "SPEC EDIT OWED" internal/ tools/
go test -race ./internal/oms/... && go vet ./...
git add internal/oms/domain/transition.go internal/oms/domain/transition_test.go \
        docs/state-machines.md docs/adr-history.md
git commit -m "oms: SM §1's Submitted gains its query-resolution row

The table gave Submitted the three live-path answers and no query row,
while its own invariant resolves 'DispatchAuthorized or beyond' by query
and §6.7 lists Submitted among the states quiesced to finality. Third
axis of v0.9.11's finding, and the one every successful Place lands on."
```

---

## Task 2: the `Resolver`'s resolvable set widens, and the consequence is verified

The `Resolver` gets no production change. Its entry gate is
`domain.CanTransition(rec.State, domain.StateAbsentAfterFinality)`, and Task 1 made
that succeed for `StateSubmitted`. **This task exists because a derived consequence
is an unchecked claim until something checks it** — CLAUDE.md's rejected-alternative
shape, arriving on a derivation rather than on a rejected option.

**Files:**
- Test: `internal/oms/application/resolve_test.go:305-341`

**Interfaces:**
- Consumes: Task 1's edge set. `application.Resolver`, `Resolution`, `newResolver`,
  `resolvableAt`, `dispatchableOrder`, `testPolicy` (all existing test helpers).
- Produces: nothing new.

- [ ] **Step 1: Write the failing test — move `Submitted` across the line**

In `TestResolveRefusesAnOrderTheDomainWouldNotProveAbsent`, delete
`domain.StateSubmitted,` from the `refused` slice, and add it to the positive
control below:

```go
	// The positive control, and it is what makes the list above a claim rather
	// than a universally-refusing predicate: these three ARE resolvable.
	//
	// StateSubmitted joined them in #242, and it joined by DERIVATION rather than
	// by edit — the gate asks the domain whether proved absence is an answer it
	// would accept, and SM §1's new query-resolution row says it is. That is the
	// derivation working as designed, and it closes a second gap nobody had
	// named: a successfully placed order the venue holds NO record of could not
	// be resolved by §5.3's rule at all, because the one state every such order
	// is in was refused at the door.
	for _, state := range []domain.OrderState{
		domain.StateDispatchAuthorized,
		domain.StateSubmitted,
		domain.StateSubmissionUnknown,
	} {
```

- [ ] **Step 2: Run it to confirm the old assertion fails and the new one is real**

Run: `go test ./internal/oms/application/ -run TestResolveRefusesAnOrderTheDomainWouldNotProveAbsent -v`
Expected before Task 1: FAIL. After Task 1: PASS — which is why this step is a
*confirmation* rather than a red bar. To get a genuine red bar, run it at Task 1's
parent commit first:

```bash
git stash && go test ./internal/oms/application/ -run TestResolveRefuses -v ; git stash pop
```
Expected on the stashed tree: FAIL with `Resolve at submitted = <nil>, want ErrOrderNotResolvable`
being absent and the positive control failing instead — evidence the test discriminates.

- [ ] **Step 3: Add the walk that the widening makes reachable**

New test in `resolve_test.go`, beside the existing absence tests:

```go
// TestResolveProvesAbsenceOfAPlacedOrder is the case the derivation opened.
//
// Before #242 a Submitted order was refused at the resolvable-set gate, so the
// one state every successfully placed order occupies was the one state §5.3's
// stable-not-found rule could not be run for. A Place that reached the venue's
// edge and left no record there — the lost-ack-plus-nothing-landed case — had no
// terminal available to it.
func TestResolveProvesAbsenceOfAPlacedOrder(t *testing.T) {
	t.Parallel()

	res, j, r, v := newResolver(t, testNow, testPolicy())
	o := dispatchableOrder(t)
	resolvableAt(t, r, o, domain.StateSubmitted, testNow.Add(-24*time.Hour))
	v.status[o.ClientOrderID] = ports.VenueOrder{
		Presence: ports.VenueHasNoOrder,
		Reach:    publishedReach(t),
	}

	got, err := res.Resolve(o.ClientOrderID)
	if err != nil {
		t.Fatalf("Resolve = %v, want nil", err)
	}
	if got.Outcome != application.OutcomeResolved {
		t.Fatalf("Outcome = %v (%v), want resolved", got.Outcome, got.Reason)
	}
	if got.State != domain.StateAbsentAfterFinality {
		t.Errorf("State = %s, want absent_after_finality", got.State)
	}
	if !got.Journaled || !got.Applied {
		t.Errorf("Journaled=%v Applied=%v, want both true", got.Journaled, got.Applied)
	}
	if states := j.states(); len(states) != 1 || states[0] != domain.StateAbsentAfterFinality {
		t.Errorf("journal holds %v, want one absent_after_finality record", states)
	}
}
```

Match `publishedReach` / `resolvableAt` to the helpers already in `resolve_test.go`
— read their signatures before writing this; if the reach helper has another name,
use the one the neighbouring absence tests use, and give the anchor an age past the
landing horizon and inside the surface's reach so all four gates pass.

- [ ] **Step 4: Run the resolver suite**

Run: `go test -race ./internal/oms/application/ -run TestResolve -v`
Expected: PASS, including the new walk.

- [ ] **Step 5: Commit**

```bash
git add internal/oms/application/resolve_test.go
git commit -m "oms: the resolver admits Submitted, and the derivation is checked

The resolvable set is derived from 'proved absence is an answer the domain
would accept', so SM §1's new row widened it without an edit here. A
derived consequence is an unchecked claim until something checks it: a
placed order the venue holds no record of now reaches AbsentAfterFinality."
```

---

## Task 3: the join journals the venue's lifecycle reading

This is the defect's remedy.

**Files:**
- Modify: `internal/oms/application/join.go` (`Joiner` struct :51, `NewJoiner` :143, `Join` report :304, `joinOne` :798)
- Modify: `internal/oms/application/errors.go` (one new sentinel; reword `ErrJournalMissing`'s message)
- Modify: `internal/oms/module.go:823` (call site)
- Modify: `internal/oms/application/join_test.go` (helper), `join_spot_test.go:131,217,598,607`, `reconcile_spot_test.go:59` (call sites)
- Test: `internal/oms/application/join_test.go`

**Interfaces:**
- Consumes: `ports.Journal.AppendState`, `ports.OrderWriteRepository.ApplyState`,
  `ports.VenueOrder.State`, `domain.CanTransition`, `domain.OrderState.Validate`.
- Produces, for Tasks 4 and 5:
  - `func application.NewJoiner(v ports.OrderResolver, p ports.VenuePositions, orders ports.OrderWriteRepository, jr ports.Journal, clk clock.Clock, account string, spot SpotLeg, scopes ...ports.PositionScope) (*Joiner, error)`
  - `application.Join` gains `Advanced int` and `Contradicted []ids.ClientOrderID`
  - `application.ErrVenueStateNotAnAbsence error`

- [ ] **Step 1: Write the failing tests**

Add to `internal/oms/application/join_test.go`. First a shared helper, because eight
tests need a joiner whose journal they can inspect:

```go
// newJoinerWithJournal is newJoiner plus the journal the state write goes to.
//
// The journal is a separate return rather than a field on the joiner's fakes
// because every claim in this file's finality half is a claim about TWO stores
// agreeing — a mutation that deletes the AppendState leaves the projection
// exactly right, so a test reading only the record cannot see it.
func newJoinerWithJournal(t *testing.T, scopes ...ports.PositionScope) (
	*application.Joiner, *fakeOrders, *fakeJoinVenue, *fakePositions, *fakeJournal) {
	t.Helper()
	orders := newFakeOrders()
	v := newFakeJoinVenue()
	p := newFakePositions()
	j := newFakeJournal()
	clk, err := clock.NewTestClock(joinNow, []clock.Calibration{
		{Venue: perp().Venue, Offset: testVenueOffset, Uncertainty: testUncertainty},
	})
	if err != nil {
		t.Fatalf("NewTestClock: %v", err)
	}
	joiner, err := application.NewJoiner(v, p, orders, j, clk, joinAccount, application.SpotLeg{}, scopes...)
	if err != nil {
		t.Fatalf("NewJoiner: %v", err)
	}
	return joiner, orders, v, p, j
}

// venueSays is the ordinary present reading: the venue holds the order, states a
// lifecycle and a cumulative.
func venueSays(t *testing.T, state domain.OrderState, venueOrderID, executed string) ports.VenueOrder {
	t.Helper()
	return ports.VenueOrder{
		Presence:     ports.VenueHasOrder,
		State:        state,
		Executed:     qty(t, executed),
		VenueOrderID: venueOrderID,
	}
}
```

Then the tests. Checklist item numbers from §"The fixture-collapse checklist" are
cited on each, so a reviewer can see which collapse each one is varying.

```go
// TestJoinAdvancesEachOrderToItsOwnVenueReading is the core claim, and the
// fixture carries checklist items 1, 2, 3 and 6 in one run.
//
//   - TWO orders with DIFFERENT readings (1): one the venue reports Filled, one
//     still Acknowledged. "advances every order" and "advances the first" are
//     different answers here, and so are "writes the venue's reading" and
//     "writes a terminal" (2).
//   - The filled one carries a swept EXECUTION (3), so ApplyFill bumps its
//     version before the state write. A fixture with no fills makes "threads the
//     post-fill version" and "threads the loaded version" the same number.
//   - Both stores are asserted (6): a mutation deleting the AppendState leaves
//     the projection correct and only the journal assertion falls.
func TestJoinAdvancesEachOrderToItsOwnVenueReading(t *testing.T) {
	t.Parallel()

	j, orders, v, _, jr := newJoinerWithJournal(t)
	filled := joinOrder(t, 1, perp(), domain.SideSell, "0.5")
	live := joinOrder(t, 2, perp(), domain.SideBuy, "0.2")
	plantJoinable(t, orders, filled, domain.StateSubmitted)
	plantJoinable(t, orders, live, domain.StateSubmitted)

	v.status[filled.ClientOrderID] = venueSays(t, domain.StateFilled, "v-1", "0.5")
	v.status[live.ClientOrderID] = venueSays(t, domain.StateAcknowledged, "v-2", "0")
	v.sweeps[perp()] = ports.ExecutionSweep{
		Executions: []ports.VenueExecution{
			execution(t, "e-1", filled.ClientOrderID, "v-1", "0.5"),
		},
	}

	report, err := j.Join([]ids.ClientOrderID{filled.ClientOrderID, live.ClientOrderID})
	if err != nil {
		t.Fatalf("Join = %v, want nil", err)
	}
	if report.Advanced != 2 {
		t.Errorf("Advanced = %d, want 2", report.Advanced)
	}
	if len(report.Contradicted) != 0 {
		t.Errorf("Contradicted = %v, want none", report.Contradicted)
	}

	if got := stateOf(t, orders, filled.ClientOrderID); got != domain.StateFilled {
		t.Errorf("projection holds %s for the filled order, want filled", got)
	}
	if got := stateOf(t, orders, live.ClientOrderID); got != domain.StateAcknowledged {
		t.Errorf("projection holds %s for the live order, want acknowledged", got)
	}

	// The journal, separately. Order is not asserted — Join walks the ids in the
	// order it was given, but the claim here is membership, and pinning the
	// sequence would make this test fail for a reason it is not about.
	want := map[domain.OrderState]int{domain.StateFilled: 1, domain.StateAcknowledged: 1}
	got := map[domain.OrderState]int{}
	for _, s := range jr.states() {
		got[s]++
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("journal holds %v, want %v", got, want)
	}
}

// TestJoinAdvancesAgainstThePostFillVersion is checklist item 3 stated as its own
// claim, because the test above would still pass if the version were read from
// the record as loaded: ApplyState would then be handed a stale version and the
// repository would refuse with ErrVersionConflict — which Join reports as an
// ERROR, so the failure is loud but its cause is one line away from the fixture.
func TestJoinAdvancesAgainstThePostFillVersion(t *testing.T) {
	t.Parallel()

	j, orders, v, _, _ := newJoinerWithJournal(t)
	o := joinOrder(t, 1, perp(), domain.SideSell, "0.5")
	plantJoinable(t, orders, o, domain.StateSubmitted) // planted at version 7
	v.status[o.ClientOrderID] = venueSays(t, domain.StateFilled, "v-1", "0.5")
	v.sweeps[perp()] = ports.ExecutionSweep{
		Executions: []ports.VenueExecution{execution(t, "e-1", o.ClientOrderID, "v-1", "0.5")},
	}

	if _, err := j.Join([]ids.ClientOrderID{o.ClientOrderID}); err != nil {
		t.Fatalf("Join = %v, want nil", err)
	}

	// The fill bumped 7 to 8, so the state write must claim 8. The applied-call
	// log is what makes this an assertion rather than an inference from the end
	// state.
	last := orders.lastApply(t, o.ClientOrderID)
	if last.expected != 8 {
		t.Errorf("ApplyState claimed version %d, want 8 (the fill bumped 7)", last.expected)
	}
	if last.to != domain.StateFilled {
		t.Errorf("ApplyState target = %s, want filled", last.to)
	}
}

// TestJoinRereadsATerminalOrderWithoutWriting is checklist item 4: the ORDINARY
// case on every boot after the first, and the one that makes the no-op
// short-circuit observable. Without it the record's state equals the venue's,
// CanTransition answers ErrTransitionNoOp, and a joiner that inspected that error
// instead of comparing first would file a healthy re-read under Contradicted and
// hold the boot posture at NeedsOperator for ever.
func TestJoinRereadsATerminalOrderWithoutWriting(t *testing.T) {
	t.Parallel()

	j, orders, v, _, jr := newJoinerWithJournal(t)
	o := joinOrder(t, 1, perp(), domain.SideSell, "0.5")
	plantJoinable(t, orders, o, domain.StateFilled)
	v.status[o.ClientOrderID] = venueSays(t, domain.StateFilled, "v-1", "0")

	report, err := j.Join([]ids.ClientOrderID{o.ClientOrderID})
	if err != nil {
		t.Fatalf("Join = %v, want nil", err)
	}
	if report.Advanced != 0 {
		t.Errorf("Advanced = %d, want 0 — a re-read is not an advance", report.Advanced)
	}
	if len(report.Contradicted) != 0 {
		t.Errorf("Contradicted = %v, want none — the states AGREE", report.Contradicted)
	}
	if states := jr.states(); len(states) != 0 {
		t.Errorf("journal holds %v, want nothing", states)
	}
}

// TestJoinReportsAContradictoryVenueReading is checklist item 5, and the fixture
// is the ISOLATING one: both states are declared and valid, the venue HAS the
// order, and the sweep is complete — so the only thing in the function that can
// object is the edge check. A malformed state would trip Validate instead, and a
// probe two gates can see shows that something objected rather than that this
// rule did.
//
// Acknowledged -> Rejected is also the realistic accident: a rejection is a
// submission-time verdict, so a venue reporting one about an order we watched go
// live means the two readings are about different histories.
func TestJoinReportsAContradictoryVenueReading(t *testing.T) {
	t.Parallel()

	j, orders, v, _, jr := newJoinerWithJournal(t)
	bad := joinOrder(t, 1, perp(), domain.SideSell, "0.5")
	good := joinOrder(t, 2, perp(), domain.SideBuy, "0.2")
	plantJoinable(t, orders, bad, domain.StateAcknowledged)
	plantJoinable(t, orders, good, domain.StateSubmitted)
	v.status[bad.ClientOrderID] = venueSays(t, domain.StateRejected, "v-1", "0")
	v.status[good.ClientOrderID] = venueSays(t, domain.StateFilled, "v-2", "0")

	report, err := j.Join([]ids.ClientOrderID{bad.ClientOrderID, good.ClientOrderID})
	if err != nil {
		t.Fatalf("Join = %v, want nil — a contradiction on ONE order is a report entry", err)
	}
	if want := []ids.ClientOrderID{bad.ClientOrderID}; !reflect.DeepEqual(report.Contradicted, want) {
		t.Errorf("Contradicted = %v, want %v", report.Contradicted, want)
	}
	// The positive control in the same run: the healthy order advanced, so this
	// is not a joiner that refused everything.
	if report.Advanced != 1 {
		t.Errorf("Advanced = %d, want 1 — the second order is the positive control", report.Advanced)
	}
	if got := stateOf(t, orders, bad.ClientOrderID); got != domain.StateAcknowledged {
		t.Errorf("the contradicted order moved to %s, want acknowledged (unchanged)", got)
	}
	if states := jr.states(); len(states) != 1 || states[0] != domain.StateFilled {
		t.Errorf("journal holds %v, want only the healthy order's filled record", states)
	}
}

// TestJoinRefusesProvedAbsenceFromAPresentOrder guards an edge this very slice
// made legal. Submitted -> AbsentAfterFinality is now a domain edge, so nothing
// in CanTransition would stop the join writing "proved stable absence" from a
// venue that is holding the order in its hand. That is FLAG-2's evidence-class
// conflation, and §5.3 gives that terminal four gates the Resolver runs.
func TestJoinRefusesProvedAbsenceFromAPresentOrder(t *testing.T) {
	t.Parallel()

	j, orders, v, _, jr := newJoinerWithJournal(t)
	o := joinOrder(t, 1, perp(), domain.SideSell, "0.5")
	plantJoinable(t, orders, o, domain.StateSubmitted)
	v.status[o.ClientOrderID] = venueSays(t, domain.StateAbsentAfterFinality, "v-1", "0")

	_, err := j.Join([]ids.ClientOrderID{o.ClientOrderID})
	if !errors.Is(err, application.ErrVenueStateNotAnAbsence) {
		t.Fatalf("Join = %v, want ErrVenueStateNotAnAbsence", err)
	}
	if got := stateOf(t, orders, o.ClientOrderID); got != domain.StateSubmitted {
		t.Errorf("projection holds %s, want submitted (unchanged)", got)
	}
	if states := jr.states(); len(states) != 0 {
		t.Errorf("journal holds %v, want nothing", states)
	}
}

// TestJoinDoesNotAdvanceAnAbsentOrder — checklist item 7. An absence is the one
// answer this use case must not act on: it is the INPUT to §5.3's four-gate rule,
// and the Resolver is what runs them. Asserting the journal is empty rather than
// only that Absent is populated is the point: the report field would be right
// even if a state had been written.
func TestJoinDoesNotAdvanceAnAbsentOrder(t *testing.T) {
	t.Parallel()

	j, orders, v, _, jr := newJoinerWithJournal(t)
	o := joinOrder(t, 1, perp(), domain.SideSell, "0.5")
	plantJoinable(t, orders, o, domain.StateSubmitted)
	v.status[o.ClientOrderID] = ports.VenueOrder{Presence: ports.VenueHasNoOrder}

	report, err := j.Join([]ids.ClientOrderID{o.ClientOrderID})
	if err != nil {
		t.Fatalf("Join = %v, want nil", err)
	}
	if len(report.Absent) != 1 || report.Advanced != 0 {
		t.Errorf("Absent=%v Advanced=%d, want one absence and no advance", report.Absent, report.Advanced)
	}
	if states := jr.states(); len(states) != 0 {
		t.Errorf("journal holds %v, want nothing — an absence is the resolver's input", states)
	}
}

// TestJoinDoesNotAdvanceOnAnIncompleteSweep — checklist item 8, and the fixture
// is built so the two readings differ: the venue reports FILLED, which WOULD
// advance. A short-sweep fixture whose venue reading was already the record's
// would make "gated by the sweep" and "nothing to advance" the same answer.
//
// SM §2 calls the four-step contract "the ONE canonical boundary … never a
// weaker cancel-ack-only variant", and step 3 is the sweep. A terminal recorded
// from a status read alone collapses the possible-delta interval EARLY, which is
// exactly what SM §1's venue-initiated-termination row warns about.
func TestJoinDoesNotAdvanceOnAnIncompleteSweep(t *testing.T) {
	t.Parallel()

	j, orders, v, _, jr := newJoinerWithJournal(t)
	o := joinOrder(t, 1, perp(), domain.SideSell, "0.5")
	plantJoinable(t, orders, o, domain.StateSubmitted)
	v.status[o.ClientOrderID] = venueSays(t, domain.StateFilled, "v-1", "0")
	v.sweeps[perp()] = ports.ExecutionSweep{ProvenThrough: joinNow.Add(-time.Hour)}

	report, err := j.Join([]ids.ClientOrderID{o.ClientOrderID})
	if err != nil {
		t.Fatalf("Join = %v, want nil", err)
	}
	if len(report.Incomplete) != 1 || report.Advanced != 0 {
		t.Errorf("Incomplete=%v Advanced=%d, want one incomplete and no advance",
			report.Incomplete, report.Advanced)
	}
	if got := stateOf(t, orders, o.ClientOrderID); got != domain.StateSubmitted {
		t.Errorf("projection holds %s, want submitted — the sweep proved nothing", got)
	}
	if states := jr.states(); len(states) != 0 {
		t.Errorf("journal holds %v, want nothing", states)
	}
}

// TestJoinAdvancesDespiteAnUnattributedCumulative pins the ORTHOGONALITY §5.3
// requires, which is a decision rather than an accident. The cross-check
// disagreed about the AMOUNT; the venue's reading is about FINALITY, and §5.3
// keeps the two orthogonal ("nothing relates State to Cumulative"). Refusing to
// advance would leave the order in the state this slice exists to move it out
// of, to punish a number the report already carries — and the consumer that
// gates on both (#236's D4) already refuses on the report.
func TestJoinAdvancesDespiteAnUnattributedCumulative(t *testing.T) {
	t.Parallel()

	j, orders, v, _, _ := newJoinerWithJournal(t)
	o := joinOrder(t, 1, perp(), domain.SideSell, "0.5")
	plantJoinable(t, orders, o, domain.StateSubmitted)
	// The venue states 0.5 executed and the sweep attributes nothing.
	v.status[o.ClientOrderID] = venueSays(t, domain.StateFilled, "v-1", "0.5")

	report, err := j.Join([]ids.ClientOrderID{o.ClientOrderID})
	if err != nil {
		t.Fatalf("Join = %v, want nil", err)
	}
	if len(report.Unattributed) != 1 {
		t.Fatalf("Unattributed = %v, want one", report.Unattributed)
	}
	if report.Advanced != 1 {
		t.Errorf("Advanced = %d, want 1 — state and ledger are orthogonal", report.Advanced)
	}
}

// TestJoinErrorsWhenAStoreRefusesTheState pins Join's contract, which is the
// OPPOSITE of Resolver's and deliberately so. Resolver promises "err != nil means
// NOTHING was written", so it must report a partial write as a value. Join
// promises "err != nil means the projection is PART-REBUILT and must not be
// traded on", and the posture is what stops trading. A journal that did not take
// the record, or a projection that refused the version, IS that condition.
func TestJoinErrorsWhenAStoreRefusesTheState(t *testing.T) {
	t.Parallel()

	boom := errors.New("store refused")

	t.Run("journal", func(t *testing.T) {
		t.Parallel()
		j, orders, v, _, jr := newJoinerWithJournal(t)
		o := joinOrder(t, 1, perp(), domain.SideSell, "0.5")
		plantJoinable(t, orders, o, domain.StateSubmitted)
		v.status[o.ClientOrderID] = venueSays(t, domain.StateFilled, "v-1", "0")
		jr.failOn[domain.StateFilled] = boom

		if _, err := j.Join([]ids.ClientOrderID{o.ClientOrderID}); !errors.Is(err, boom) {
			t.Fatalf("Join = %v, want the journal's error", err)
		}
		// The apply is NOT attempted: legality is established before the journal
		// and the journal is what §7.5 replays, so a projection ahead of the log
		// is the one ordering this must never produce.
		if got := stateOf(t, orders, o.ClientOrderID); got != domain.StateSubmitted {
			t.Errorf("projection holds %s, want submitted — the log did not take it", got)
		}
	})

	t.Run("projection", func(t *testing.T) {
		t.Parallel()
		j, orders, v, _, _ := newJoinerWithJournal(t)
		o := joinOrder(t, 1, perp(), domain.SideSell, "0.5")
		plantJoinable(t, orders, o, domain.StateSubmitted)
		v.status[o.ClientOrderID] = venueSays(t, domain.StateFilled, "v-1", "0")
		orders.failApplyState[domain.StateFilled] = boom

		if _, err := j.Join([]ids.ClientOrderID{o.ClientOrderID}); !errors.Is(err, boom) {
			t.Fatalf("Join = %v, want the repository's error", err)
		}
	})
}

// TestNewJoinerRefusesAMissingJournal keeps the constructor's refusal set total.
// Refused at construction rather than at the first call, for the reason
// ErrJournalMissing already states: the call that would discover it is the one
// whose whole purpose is a durable record.
func TestNewJoinerRefusesAMissingJournal(t *testing.T) {
	t.Parallel()

	orders, v, p := newFakeOrders(), newFakeJoinVenue(), newFakePositions()
	clk, err := clock.NewTestClock(joinNow, []clock.Calibration{
		{Venue: perp().Venue, Offset: testVenueOffset, Uncertainty: testUncertainty},
	})
	if err != nil {
		t.Fatalf("NewTestClock: %v", err)
	}
	if _, err := application.NewJoiner(v, p, orders, nil, clk, joinAccount, application.SpotLeg{}); !errors.Is(err, application.ErrJournalMissing) {
		t.Errorf("NewJoiner(no journal) = %v, want ErrJournalMissing", err)
	}
}
```

Two small test helpers these need, added beside them:

```go
// stateOf reads one order's state out of the projection.
func stateOf(t *testing.T, r *fakeOrders, id ids.ClientOrderID) domain.OrderState {
	t.Helper()
	rec, err := r.Load(id)
	if err != nil {
		t.Fatalf("Load(%q): %v", id, err)
	}
	return rec.State
}

// lastApply returns the final recorded write for one order, which is where the
// version arithmetic is observable.
func (r *fakeOrders) lastApply(t *testing.T, id ids.ClientOrderID) applyCall {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := len(r.applied) - 1; i >= 0; i-- {
		if r.applied[i].id == id {
			return r.applied[i]
		}
	}
	t.Fatalf("no apply recorded for %q", id)
	return applyCall{}
}
```

- [ ] **Step 2: Run them to make sure they fail**

Run: `go test ./internal/oms/application/ -run 'TestJoin|TestNewJoiner' -v`
Expected: FAIL to **compile** — `too many arguments in call to application.NewJoiner`,
`report.Advanced undefined`, `application.ErrVenueStateNotAnAbsence undefined`.
That is the intended red bar; a compile failure here is the assertion that the
symbols do not exist yet.

- [ ] **Step 3: Add the sentinel and reword `ErrJournalMissing`**

In `internal/oms/application/errors.go`, change line 14's message — it says
"dispatcher" and is already raised by `Dispatcher`, `Planner` and `Resolver`, and
gains a fourth caller here. Every consumer compares with `errors.Is`, so the string
is free to be accurate:

```go
	ErrJournalMissing    = errors.New("oms: use case has no journal")
```

and add, beside `ErrVenueOrderIDMismatch` (~line 287):

```go
	// ErrVenueStateNotAnAbsence reports a venue that HANDED US THE ORDER and
	// described it as proved absence.
	//
	// StateAbsentAfterFinality is §5.3's four-gate conclusion and FLAG-2's whole
	// point: proved absence and a venue-terminal record are different evidence
	// classes, and recording one as the other tells §7.5's reconcile-only boot
	// that we never found an order we in fact found. The join runs no gates —
	// Resolver does — so it may never reach that terminal on any reading.
	//
	// It is a REFUSAL rather than a Contradicted entry because it is not two
	// histories disagreeing: the transition may well be legal (Submitted ->
	// AbsentAfterFinality became an edge in this same slice), and it is the
	// EVIDENCE that is unavailable. A guard that let it through would be
	// checking the domain's edge set for a rule the domain does not hold.
	ErrVenueStateNotAnAbsence = errors.New("oms: the venue holds the order and reports proved absence")
```

- [ ] **Step 4: Add the journal to the `Joiner`**

In `internal/oms/application/join.go`, the struct (line ~51) gains a field:

```go
type Joiner struct {
	venue     ports.OrderResolver
	positions ports.VenuePositions
	orders    ports.OrderWriteRepository

	// journal is the durable half of §6's journal-then-apply. It arrived with
	// #242: until then this use case only ADMITTED FILLS, and ApplyFill has no
	// journal counterpart at all (ports.Journal is "narrowed to what the dispatch
	// path writes"). Writing a STATE is the first thing the join does that §7.5
	// replays, so it is the first thing that needs the log.
	journal ports.Journal

	clk     clock.Clock
	account string
	scopes  []ports.PositionScope
	spot    SpotLeg
}
```

`NewJoiner` gains the parameter — after `orders`, so the pairing §6 requires reads
in the signature — and the refusal:

```go
func NewJoiner(v ports.OrderResolver, p ports.VenuePositions, orders ports.OrderWriteRepository,
	jr ports.Journal, clk clock.Clock, account string, spot SpotLeg, scopes ...ports.PositionScope) (*Joiner, error) {
	if v == nil {
		return nil, ErrResolverMissing
	}
	if p == nil {
		return nil, ErrVenuePositionsMissing
	}
	if orders == nil {
		return nil, ErrOrdersMissing
	}
	if jr == nil {
		return nil, ErrJournalMissing
	}
	// ... unchanged from here
```

and the struct literal at the end gains `journal: jr,`.

- [ ] **Step 5: Add the two report fields**

In the `Join` struct (line ~304), after `Duplicates`:

```go
	// Advanced is how many orders' lifecycle STATE this join moved to the
	// venue's own reading.
	//
	// It counts writes, not orders: a re-read whose stored state already equals
	// the venue's is not an advance, which matters because that is the ordinary
	// case on every boot after the first. A run reporting advances equal to
	// Orders on a warm boot means the projection lost its states.
	Advanced int

	// Contradicted are the orders whose venue reading SM §1 has no edge to from
	// the state this system holds — a venue reporting Rejected about an order we
	// watched reach Acknowledged, say.
	//
	// It is a REPORT entry rather than an error because it is a fact about ONE
	// order's two histories, and aborting the join over it would leave every
	// later order unqueried. It is not retryable and must not be: the same
	// account answers the same way, so the reconciliation seam maps it to
	// NeedsOperator beside Unattributed — see Reconciler.joinOutcome.
	Contradicted []ids.ClientOrderID
```

- [ ] **Step 6: Write `advance`, and call it last in `joinOne`**

Add after `joinOne` in `join.go`:

```go
// advance journals and applies the venue's own lifecycle reading. It is the step
// that moves an order this system placed off StateSubmitted.
//
// # The reading was in hand for four slices and was never written
//
// ports.VenueOrder.State is "the venue's lifecycle reading, already translated
// into SM §1's vocabulary by the ACL", and joinOne has fetched it since slice F2
// to attribute fills and run the Unattributed cross-check. Nothing wrote it, and
// nothing noticed, because every consumer's success condition stopped short of
// finality: `arblab exercise run` treats StateSubmitted AS its success state
// (exercise/app.go), so the single-venue root never needed to know whether a leg
// filled. #236 is the first thing that does.
//
// # Legality is established BEFORE the journal, and the ordering is forced
//
// Resolver.record's argument, unchanged: ports.Journal is append-only and
// "refuses to hold a second copy of the edge set", while the write repository
// checks domain.CanTransition and would refuse. Journaling first and discovering
// the refusal second would leave the durable log holding a transition the
// projection rejected — and §7.5 replays the log.
//
// # A store's refusal is an ERROR here, and a VALUE in Resolver. Both follow
// # from their own contract
//
// Resolver promises "err != nil means NOTHING was written", so a half-written
// resolution has to leave as a Resolution with Journaled/Applied. Join promises
// the reverse — "err != nil means the projection is PART-REBUILT and must not be
// traded on" — and the boot posture is what stops trading. A log that did not
// take the record is exactly that condition, so it leaves by the error return.
func (j *Joiner) advance(rec ports.OrderRecord, status ports.VenueOrder, report *Join) (ports.OrderRecord, error) {
	id := rec.Order.ClientOrderID

	if err := status.State.Validate(); err != nil {
		// A malformed reading is a broken translation, not a disagreement about
		// this order's history, so it leaves by the error return rather than
		// joining Contradicted — whose members are two WELL-FORMED readings SM §1
		// has no edge between. Filing them together would put an adapter defect
		// and a venue disagreement in one bucket an operator has one response for.
		return rec, fmt.Errorf("join order %q: venue state: %w", id, err)
	}

	if status.State == domain.StateAbsentAfterFinality {
		// The venue HAS this order, so proved absence is not among the answers
		// available here whatever the reading says. §5.3 gives that terminal four
		// gates and Resolver is what runs them; recording it from a present order
		// is the evidence-class conflation FLAG-2 exists to prevent. The guard is
		// not hypothetical bookkeeping: Submitted -> AbsentAfterFinality became a
		// legal edge in this same slice, so the domain would now permit precisely
		// the write this refuses.
		return rec, fmt.Errorf("join order %q: venue reports %s: %w",
			id, status.State, ErrVenueStateNotAnAbsence)
	}

	if rec.State == status.State {
		// The ordinary case on every boot after the first. Checked here rather
		// than by inspecting CanTransition's ErrTransitionNoOp, for replay.State's
		// reason one layer over: the two are not the same claim — that sentinel is
		// about a self-edge, this is about whether the READING advances the
		// history — and they part company the moment anything else can write
		// between the load and this line.
		return rec, nil
	}

	if err := domain.CanTransition(rec.State, status.State); err != nil {
		report.Contradicted = append(report.Contradicted, id)
		return rec, nil
	}

	if err := j.journal.AppendState(id, status.State); err != nil {
		return rec, fmt.Errorf("join order %q: journal %s: %w", id, status.State, err)
	}
	applied, err := j.orders.ApplyState(id, rec.Version, status.State)
	if err != nil {
		return rec, fmt.Errorf("join order %q: apply %s: %w", id, status.State, err)
	}
	report.Advanced++
	return applied, nil
}
```

In `joinOne`, replace the final `return rec, nil` (line ~903) with:

```go
	// LAST, after the executions are admitted and cross-checked. The order is not
	// arithmetic — nothing in advance reads a quantity — it is what the record
	// MEANS: the reading is complete before its finality is journaled.
	//
	// It runs even when the cross-check disagreed. §5.3 keeps state and ledger
	// orthogonal ("nothing relates State to Cumulative"), the disagreement is
	// about AMOUNT and this is about FINALITY, and refusing to advance would
	// leave the order in the state this slice exists to move it out of in order
	// to punish a number the report already carries. The consumer that needs both
	// gates on both.
	return j.advance(rec, status, report)
```

Two earlier returns in `joinOne` are deliberately left short of this call and each
is a normative refusal rather than an omission: the `VenueHasNoOrder` return (an
absence is §5.3's input, not this use case's conclusion) and the
`sweep.ProvenThrough.Before(until)` return (SM §2 step 3's sweep is what closes a
terminal, and it is not "the ONE canonical boundary" if a status read can skip it).
Add a sentence to each pointing at `advance`, so a reader of either return knows the
state write is downstream of it.

**One asymmetry with `Resolver` to state in the sweep return's comment rather than
leave to be found:** `Resolver.present` skips the sweep for a NON-terminal reading
("the order is live, its interval has not collapsed, and there is no finality to
prove"), while here the sweep gate precedes every target including
`Acknowledged`. That is stricter than SM §2 requires, in the safe direction — a
state not advanced holds the possible-delta interval — and a short sweep is
retryable on the next poll.

- [ ] **Step 7: Update the five call sites**

```bash
grep -rn "NewJoiner(" internal/ | grep -v "func NewJoiner"
```

- `internal/oms/module.go:823` — `journal` is already in scope (it is passed to
  `NewResolver` at :735 and closed on every error path):

```go
	joiner, err := application.NewJoiner(reader, positions, orders, journal, deps.Clock, deps.Account, spot, deps.PositionScopes...)
```

- `internal/oms/application/join_test.go:144` (`newJoiner`) — build a `fakeJournal`
  and pass it; keep `newJoiner`'s existing four-value return so the tests that do
  not care are untouched.
- `internal/oms/application/join_spot_test.go:131,217,598,607` — pass `newFakeJournal()`.
- `internal/oms/application/reconcile_spot_test.go:59` — pass the journal the
  fixture already holds if there is one, otherwise `newFakeJournal()`.

- [ ] **Step 8: Run the tests to verify they pass**

```bash
go build ./... && go test -race ./internal/oms/... -v -run 'TestJoin|TestNewJoiner|TestReconcile'
```
Expected: PASS.

- [ ] **Step 9: Bite-check the four claims that carry the slice**

Run each from the repo root; re-read the target line numbers first, since step 6
added lines.

```bash
# 1. "the state is journaled, not only applied" — the mutation the projection
#    assertion alone cannot see (checklist item 6).
tools/mutate.sh internal/oms/application/join.go <AppendState line> \
  's#if err := j.journal.AppendState(id, status.State); err != nil {#if err := error(nil); err != nil {#' \
  'the state write skips the journal' ./internal/oms/application/

# 2. "the version threaded is the POST-FILL one" (checklist item 3).
tools/mutate.sh internal/oms/application/join.go <ApplyState line> \
  's#j.orders.ApplyState(id, rec.Version, status.State)#j.orders.ApplyState(id, rec.Version-1, status.State)#' \
  'the state write claims a stale version' ./internal/oms/application/

# 3. "a re-read is not an advance" (checklist item 4) — invert the short-circuit.
tools/mutate.sh internal/oms/application/join.go <no-op line> \
  's#if rec.State == status.State {#if rec.State != status.State {#' \
  'the no-op short-circuit is inverted' ./internal/oms/application/

# 4. "proved absence is refused from a present order".
tools/mutate.sh internal/oms/application/join.go <AAF guard line> \
  's#if status.State == domain.StateAbsentAfterFinality {#if false {#' \
  'the present-order absence guard is removed' ./internal/oms/application/
```

Expected: `caught` on all four. A `SURVIVED` is either a test gap or a wrong
mutant — tell which before moving on. A `NOBUILD!!` here means re-aim (none of
these four is a compiler-enforced claim).

- [ ] **Step 10: Commit**

```bash
git add internal/oms/application/join.go internal/oms/application/errors.go \
        internal/oms/application/join_test.go internal/oms/application/join_spot_test.go \
        internal/oms/application/reconcile_spot_test.go internal/oms/module.go
git commit -m "oms: the join journals the venue's lifecycle reading

joinOne has fetched ports.VenueOrder.State since slice F2 and used it for
fill attribution and the Unattributed cross-check. It was never written,
so a placed order stayed at Submitted for the life of the process. One
journal-then-apply at the end of the join, after the sweep gate SM §2
step 3 requires and the executions it admits."
```

---

## Task 4: the boot posture consumes `Contradicted`

A report field nothing reads is a finding nobody acts on. `Reconciler.joinOutcome`
is the ladder that turns join findings into `boot.Outcome`.

**Files:**
- Modify: `internal/oms/application/reconcile.go:663-690` (`joinOutcome`)
- Test: `internal/oms/application/reconcile_test.go`

**Interfaces:**
- Consumes: `application.Join.Contradicted` (Task 3), `boot.OutcomeNeedsOperator`.
- Produces: nothing new.

- [ ] **Step 1: Write the failing test**

In `internal/oms/application/reconcile_test.go`, beside the existing
`joinOutcome`/posture tests:

```go
// TestReconcileHoldsForOperatorOnAContradictedOrder.
//
// A contradiction is not retryable and the direction matters: the same account
// answers the same way on the next boot, so an outcome that permitted a retry
// would let a deployment boot forever over two readings that can never agree.
// It sits with Unattributed rather than with Incomplete for that reason —
// Incomplete's cell means "ran, concluded, work remains", and no further work
// changes this one.
func TestReconcileHoldsForOperatorOnAContradictedOrder(t *testing.T) {
	t.Parallel()

	// Build the fixture through the reconciler the way the neighbouring posture
	// tests do: plant an order at StateAcknowledged, have the venue report
	// StateRejected, and let the join produce the entry. A hand-built
	// Reconciliation would assert the ladder over a value no join can emit.
	f := newReconcileFixture(t)
	o := f.plant(t, domain.StateAcknowledged)
	f.joinVenue.status[o.ClientOrderID] = venueSays(t, domain.StateRejected, "v-1", "0")

	out, err := f.run(t)
	if err != nil {
		t.Fatalf("Reconcile = %v, want nil", err)
	}
	if len(out.Join.Contradicted) != 1 {
		t.Fatalf("Contradicted = %v, want one", out.Join.Contradicted)
	}
	if got := f.outcomeOf(t, out, boot.GateVenueJoined); got != boot.OutcomeNeedsOperator {
		t.Errorf("GateVenueJoined = %v, want NeedsOperator", got)
	}
}
```

Match `newReconcileFixture` / `plant` / `run` / `outcomeOf` to the helpers already in
`reconcile_test.go` — read the file's existing posture tests and reuse their shape
verbatim rather than introducing a second harness.

- [ ] **Step 2: Run it to make sure it fails**

Run: `go test ./internal/oms/application/ -run TestReconcileHoldsForOperatorOnAContradictedOrder -v`
Expected: FAIL — `GateVenueJoined = passed, want NeedsOperator`.

- [ ] **Step 3: Add the rung**

In `joinOutcome`, immediately after the `Unattributed` case:

```go
	// Two well-formed readings SM §1 has no edge between. Not lifted by running
	// the join again — the same account answers the same way — and it sits above
	// Incomplete for exactly that reason: Incomplete's cell is "ran, concluded,
	// work remains", and no further work reconciles a venue that reports Rejected
	// about an order this system watched go live. What an operator is being asked
	// to decide is which of the two records is about the order they think it is.
	if len(out.Join.Contradicted) > 0 {
		return boot.OutcomeNeedsOperator
	}
```

- [ ] **Step 4: Run the suite**

Run: `go test -race ./internal/oms/application/ -run TestReconcile -v`
Expected: PASS.

- [ ] **Step 5: Bite-check the rung**

```bash
tools/mutate.sh internal/oms/application/reconcile.go <the new line> \
  's#if len(out.Join.Contradicted) > 0 {#if false {#' \
  'a contradicted order no longer holds the posture' ./internal/oms/application/
```
Expected: `caught`.

- [ ] **Step 6: Commit**

```bash
git add internal/oms/application/reconcile.go internal/oms/application/reconcile_test.go
git commit -m "oms: a contradicted venue reading holds the boot for an operator

Not retryable — the same account answers the same way — so it sits with
Unattributed rather than with Incomplete, whose cell means work remains."
```

---

## Task 5: the Place → terminal walks, and the replay round-trip

The record's §5 requirement 3, stated in its own words: *"Tests that seed a
terminal record prove nothing here — the whole defect is that the path from `Place`
to terminal does not exist, and a seeded fixture starts on the far side of it."*

Every test in this task starts at `Dispatcher.Dispatch` over a `StatePlanned`
record and ends by reading the projection.

**Files:**
- Create: `internal/oms/application/finality_test.go`

**Interfaces:**
- Consumes: `application.NewDispatcher`, `application.Dispatcher.Dispatch`,
  `application.NewJoiner`, `application.Joiner.Join`,
  `application.NewReplayer(ports.JournalReplay, ports.OrderWriteRepository)` and
  `(*Replayer).Rebuild() (Recovery, error)`, and the package's existing fakes
  (`fakeJournal`, `fakePlacer`, `fakeOrders`, `fakeJoinVenue`, `fakePositions`,
  `fakeReplay`, `journalEntry`) plus `plant`, `dispatchableOrder`, `testClock`,
  `testFilters`, `testFilterMaxAge`, `venueSays`, `stateOf`, `execution`.

**Note on the replay half.** `NewReplayer` takes `ports.JournalReplay`, which
`fakeJournal` does not implement — `replay_test.go` drives it with `fakeReplay`
over a scripted `[]journalEntry`. So the round-trip goes through a **bridge** that
turns what this run wrote into that script, which is the better test anyway: it
walks the *records*, not the object that produced them.
- Produces: nothing importable.

- [ ] **Step 1: Write the harness and the five walks**

```go
package application_test

// This file is the composition the defect was invisible to.
//
// Every other test in this package exercises ONE use case over doubles, and both
// halves were individually right: the dispatcher classified a successful Place as
// StateSubmitted, the join fetched the venue's reading, and neither was wrong.
// What nothing exercised was an order travelling from a real Place to a terminal,
// because no consumer required it — `arblab exercise run` treats StateSubmitted as
// its success state and exits there.
//
// So the fixtures here MUST start at Dispatch. A test that plants a terminal
// record starts on the far side of the bug and would have passed against the tree
// as it stood before #242, which is the only property that makes this file
// evidence rather than coverage.

// finalityHarness holds one deployment's two use cases over SHARED stores.
//
// Shared is the whole point: the dispatcher's journal and projection are the
// joiner's, so the record the join advances is the record the Place created, with
// the version the dispatch left on it. Two sets of fakes would make this an
// assertion about two independent systems that happen to agree.
type finalityHarness struct {
	dispatch *application.Dispatcher
	join     *application.Joiner
	journal  *fakeJournal
	placer   *fakePlacer
	orders   *fakeOrders
	venue    *fakeJoinVenue
}

func newFinalityHarness(t *testing.T) *finalityHarness {
	t.Helper()
	j := newFakeJournal()
	p := &fakePlacer{watch: j, ack: ports.Ack{VenueOrderID: "v-1"}}
	r := newFakeOrders()
	v := newFakeJoinVenue()
	clk := testClock(t)

	d, err := application.NewDispatcher(j, p, r, clk, application.NewDispatchAuthority(),
		testFilters(t), testFilterMaxAge)
	if err != nil {
		t.Fatalf("NewDispatcher: %v", err)
	}
	joiner, err := application.NewJoiner(v, newFakePositions(), r, j, clk, joinAccount, application.SpotLeg{})
	if err != nil {
		t.Fatalf("NewJoiner: %v", err)
	}
	return &finalityHarness{dispatch: d, join: joiner, journal: j, placer: p, orders: r, venue: v}
}

// place runs the real dispatch path and asserts it landed where the system
// actually leaves a successful Place. That assertion is not decoration: it is the
// premise every walk below rests on, and if it ever changes these tests are
// measuring something else.
//
// The AppendPlanned is here because `plant` stands in for the PROJECTION half of
// the planning path only, and §6 writes an order twice. Without the log's half the
// replay round-trip below would meet a state record for an order the rebuilt
// projection has never heard of, and fail with ErrOrderUnknown — a red bar about
// the fixture rather than about the walk.
func (h *finalityHarness) place(t *testing.T) domain.Order {
	t.Helper()
	o := dispatchableOrder(t)
	plant(t, h.orders, o)
	if err := h.journal.AppendPlanned(o); err != nil {
		t.Fatalf("AppendPlanned: %v", err)
	}

	out, err := h.dispatch.Dispatch(o.ClientOrderID, 1, ports.UrgencyOpening, nil)
	if err != nil {
		t.Fatalf("Dispatch = %v, want nil", err)
	}
	if out.State != domain.StateSubmitted {
		t.Fatalf("a successful Place classified as %s, want submitted", out.State)
	}
	if !out.Applied {
		t.Fatalf("Dispatch reported Applied=false; the walk needs the projection")
	}
	if got := stateOf(t, h.orders, o.ClientOrderID); got != domain.StateSubmitted {
		t.Fatalf("projection holds %s after Place, want submitted", got)
	}
	return o
}

// TestAPlacedOrderReachesEveryTerminalTheVenueCanReport is the requirement, as a
// table over the FIVE readings driven.toDomainState can produce.
//
// The five are not a sample: engine/venue normalizes both venues' vocabularies to
// Open / PartiallyFilled / Filled / Terminated / Rejected, and the ACL maps them
// onto exactly these SM §1 members. A sixth row would be a claim about an adapter
// that does not exist.
//
// Fill quantities differ per row so that "advances to the venue's state" and
// "advances to a state derived from the fills" are different answers, and the
// partial row's remainder is what makes the canceled-with-fills case real rather
// than a relabelled zero-fill one.
func TestAPlacedOrderReachesEveryTerminalTheVenueCanReport(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		venueState domain.OrderState
		executed   string
		fill       string // "" = the sweep returns nothing
		wantFills  int
	}{
		{
			// The full-fill IOC: the ordinary success this whole slice exists for.
			name: "full fill", venueState: domain.StateFilled,
			executed: "0.5", fill: "0.5", wantFills: 1,
		},
		{
			// A marketable IOC that took part of the book; the venue cancels the
			// remainder with no request of ours (SM §1's venue-initiated
			// termination row). This is the SHAPE #236's hedge is sized against.
			name: "partial fill then canceled", venueState: domain.StateCanceled,
			executed: "0.3", fill: "0.3", wantFills: 1,
		},
		{
			// An IOC that crossed nothing. A real outcome on a thin book, and the
			// one where a zero cumulative is the TRUTH rather than a reading taken
			// too early — which is only distinguishable because the state is
			// terminal.
			name: "zero fill canceled", venueState: domain.StateCanceled,
			executed: "0", fill: "", wantFills: 0,
		},
		{
			// An explicit venue rejection. Distinct evidence class from proved
			// absence (FLAG-2), and the one target Submitted could already reach.
			name: "venue rejection", venueState: domain.StateRejected,
			executed: "0", fill: "", wantFills: 0,
		},
		{
			// Still live. The NON-terminal row, and it is what stops this table
			// asserting "the join writes a terminal" when the claim is "the join
			// writes the venue's reading".
			name: "still acknowledged", venueState: domain.StateAcknowledged,
			executed: "0", fill: "", wantFills: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			h := newFinalityHarness(t)
			o := h.place(t)

			h.venue.status[o.ClientOrderID] = venueSays(t, tc.venueState, "v-1", tc.executed)
			if tc.fill != "" {
				h.venue.sweeps[o.Instrument] = ports.ExecutionSweep{
					Executions: []ports.VenueExecution{
						execution(t, "e-1", o.ClientOrderID, "v-1", tc.fill),
					},
				}
			}

			report, err := h.join.Join([]ids.ClientOrderID{o.ClientOrderID})
			if err != nil {
				t.Fatalf("Join = %v, want nil", err)
			}
			if report.Advanced != 1 {
				t.Errorf("Advanced = %d, want 1", report.Advanced)
			}
			if report.Fills != tc.wantFills {
				t.Errorf("Fills = %d, want %d", report.Fills, tc.wantFills)
			}
			if len(report.Contradicted) != 0 || len(report.Unattributed) != 0 {
				t.Errorf("Contradicted=%v Unattributed=%v, want neither",
					report.Contradicted, report.Unattributed)
			}
			if got := stateOf(t, h.orders, o.ClientOrderID); got != tc.venueState {
				t.Errorf("projection holds %s, want %s", got, tc.venueState)
			}

			// The journal, separately from the projection: the log is what §7.5
			// replays, and a state applied but not journaled is lost on the next
			// boot while looking perfect on this one.
			if !h.journal.recorded(o.ClientOrderID, tc.venueState) {
				t.Errorf("journal holds %v, want a %s record for %q",
					h.journal.states(), tc.venueState, o.ClientOrderID)
			}
		})
	}
}

// TestAPlacedOrderTheVenueWillNotResolveStaysSubmitted is the fifth case the
// record names, and it is the one whose SAFE answer is "nothing happened".
//
// §5.3 forbids reading an unanswered query as an absence, and the direction here
// is the same: a joiner that advanced on a failed Resolve would be concluding
// finality from a query that did not run. The order stays where it was, the error
// carries the venue's, and the boot posture is what stops trading.
func TestAPlacedOrderTheVenueWillNotResolveStaysSubmitted(t *testing.T) {
	t.Parallel()

	h := newFinalityHarness(t)
	o := h.place(t)
	rateLimited := errors.New("venue: rate limited")
	h.venue.err[o.ClientOrderID] = rateLimited

	if _, err := h.join.Join([]ids.ClientOrderID{o.ClientOrderID}); !errors.Is(err, rateLimited) {
		t.Fatalf("Join = %v, want the venue's error", err)
	}
	if got := stateOf(t, h.orders, o.ClientOrderID); got != domain.StateSubmitted {
		t.Errorf("projection holds %s, want submitted — the query was not answered", got)
	}
	// The dispatch path's three records and nothing more.
	if got := h.journal.recordCountFor(o.ClientOrderID, domain.StateFilled); got != 0 {
		t.Errorf("journal holds %d terminal records, want 0", got)
	}
}

// TestTheJournaledWalkReplaysToTheSameTerminal is the claim only a round-trip can
// make, and it is the reason each hop has to be a legal edge rather than a
// convenient one.
//
// §7.5's reconcile-only boot rebuilds STATE by replaying the log, and
// replay.State applies every record through ApplyState — which checks
// domain.CanTransition. A journal holding a transition the edge set refuses does
// not degrade: the next boot REFUSES, over a log written by this system on the
// boot before. So "the walk is legal" and "the deployment can boot again" are the
// same property, and this is the test that makes them one.
func TestTheJournaledWalkReplaysToTheSameTerminal(t *testing.T) {
	t.Parallel()

	h := newFinalityHarness(t)
	o := h.place(t)
	h.venue.status[o.ClientOrderID] = venueSays(t, domain.StateFilled, "v-1", "0.5")
	h.venue.sweeps[o.Instrument] = ports.ExecutionSweep{
		Executions: []ports.VenueExecution{execution(t, "e-1", o.ClientOrderID, "v-1", "0.5")},
	}
	if _, err := h.join.Join([]ids.ClientOrderID{o.ClientOrderID}); err != nil {
		t.Fatalf("Join = %v, want nil", err)
	}

	// A fresh projection, rebuilt from the SCRIPT this run's records make.
	rebuilt := newFakeOrders()
	replayer, err := application.NewReplayer(&fakeReplay{entries: h.journal.replayScript()}, rebuilt)
	if err != nil {
		t.Fatalf("NewReplayer: %v", err)
	}
	if _, err := replayer.Rebuild(); err != nil {
		t.Fatalf("Rebuild = %v, want nil — the log holds a transition the edge set refuses", err)
	}
	if got := stateOf(t, rebuilt, o.ClientOrderID); got != domain.StateFilled {
		t.Errorf("replayed projection holds %s, want filled", got)
	}
}
```

Three additions to `fakeJournal` in `dispatch_test.go`, beside `states()`:

```go
// replayScript turns what this run durably recorded into the entry list
// ports.JournalReplay hands a visitor.
//
// It exists because NewReplayer takes ports.JournalReplay and fakeJournal is a
// ports.Journal — the two halves of the same log, split so a writer cannot replay
// and a replayer cannot append. Bridging through the RECORDS rather than handing
// the same object over is what makes the round-trip a claim about what was
// written instead of about which object wrote it.
//
// The planned intents come first because they are held in their own slice and a
// state record for an unknown order is refused; within calls, the recorded order
// is preserved, which is the property the replay is being asked about.
func (j *fakeJournal) replayScript() []journalEntry {
	j.mu.Lock()
	defer j.mu.Unlock()

	script := make([]journalEntry, 0, len(j.planned)+len(j.calls))
	for _, o := range j.planned {
		script = append(script, journalEntry{kind: "planned", order: o})
	}
	for _, c := range j.calls {
		switch {
		case c.venueOrderID != "":
			script = append(script, journalEntry{kind: "acked", id: c.id, venueOrderID: c.venueOrderID})
		case c.state == domain.StateDispatchAuthorized:
			script = append(script, journalEntry{kind: "authorized", id: c.id, at: c.at})
		default:
			script = append(script, journalEntry{kind: "state", id: c.id, state: c.state})
		}
	}
	for i := range script {
		script[i].seq = uint64(i + 1)
	}
	return script
}
```

```go
// recorded reports whether one order reached a state in the log — the membership
// question the finality walks ask, which states() cannot answer because it
// flattens every order's records into one slice.
func (j *fakeJournal) recorded(id ids.ClientOrderID, s domain.OrderState) bool {
	return j.recordCountFor(id, s) > 0
}

func (j *fakeJournal) recordCountFor(id ids.ClientOrderID, s domain.OrderState) int {
	j.mu.Lock()
	defer j.mu.Unlock()
	n := 0
	for _, c := range j.calls {
		if c.id == id && c.state == s {
			n++
		}
	}
	return n
}
```

`journalCall` already carries `at` (set by `AppendAuthorized`), so the bridge needs
no change to the fake's recording. Confirm that before writing `replayScript`.

- [ ] **Step 2: Run them against a tree with the advance removed**

This is the step that makes this file evidence rather than coverage, and it has to
produce a **behavioural** red bar — a compile failure would only show that a
signature changed. Temporarily restore `joinOne`'s old last line:

```bash
# in internal/oms/application/join.go, replace
#     return j.advance(rec, status, report)
# with
#     return rec, nil
go test ./internal/oms/application/ -run 'TestAPlacedOrder|TestTheJournaledWalk' -v
```

Expected: FAIL on every row, with `Advanced = 0, want 1` and
`projection holds submitted, want filled` / `want canceled` / `want rejected` /
`want acknowledged`. `TestTheJournaledWalk…` fails on
`replayed projection holds submitted, want filled`.

**This is exactly the defect as it shipped**, reproduced from a real `Place`, which
is what the parent record's §5.3 asks for. Record the output in the decision record
(Task 7). Then restore the line and continue.

- [ ] **Step 3: Run them against the slice**

Run: `go test -race ./internal/oms/application/ -run 'TestAPlacedOrder|TestTheJournaledWalk' -v`
Expected: PASS, six subtests plus the two standalone walks.

- [ ] **Step 4: Check the table is not vacuous**

The five rows must produce five *different* projections. Add a temporary
`t.Logf("%s -> %s", tc.name, stateOf(...))`, run with `-v`, confirm the printed set
is `{filled, canceled, canceled, rejected, acknowledged}`, then remove the log. A
table whose rows all land on the same state is the count-executions-not-coverage
failure this repo has recorded eleven times.

- [ ] **Step 5: Commit**

```bash
git add internal/oms/application/finality_test.go internal/oms/application/dispatch_test.go
git commit -m "oms: the Place-to-terminal walks, and the replay round-trip

Every fixture starts at Dispatch over a Planned record. A seeded terminal
starts on the far side of the defect and would have passed against the
tree before this slice, which is the only property that makes these
evidence. The round-trip is the claim that each journaled hop is a legal
edge: replay applies through ApplyState, so an illegal one refuses the
NEXT boot over a log this system wrote."
```

---

## Task 6: `make formal-fresh` — SM §1 becomes a tracked premise of model (a)

The record's §5 requirement 4. Running the gate and seeing green is the **weaker**
reading of it, and here green is a false all-clear: `docs/formal/freshness.json`
tracks `state-machines.md` sections 2, 3, 5 and 7 and `build-spec.md` 5.3, 6, 7.1,
7.4 — **§1 is tracked by neither model**. So this slice can rewrite the order
lifecycle and the gate reports nothing, which is the absence-is-a-claim shape.

Model (a) `AmbiguousOrder`'s subject is SM §2, the finality contract, and it already
tracks core §5.3 as a premise because *"the model's state vocabulary comes from
here"*. SM §1 is the **mechanism** for that same vocabulary — core §5.3's own
pointer says `[mechanism: State-Machines §1]` — so it is a premise by exactly the
argument that put §5.3 in the list, and FM-7 is this repo's evidence for tracking
what a justification *consumes*.

**Files:**
- Modify: `docs/formal/freshness.json` (one entry under `AmbiguousOrder.subjects`)

**Interfaces:**
- Consumes: `tools/formalfresh`'s manifest schema (`doc`, `section`, `role`, `why`,
  `sha256`, optional `knownStale{task, note, sha256Actual}`).
- Produces: nothing importable.

- [ ] **Step 1: Confirm the gate is currently blind to §1**

```bash
make formal-fresh
grep -n '"section": "1"' docs/formal/freshness.json
```
Expected: the gate PASSES over a tree whose SM §1 this slice just rewrote, and the
grep finds nothing. That pass is the finding.

- [ ] **Step 2: Get the digest SM §1 had at the verified commit**

The stamp must be the digest as of `310e854` — the commit `verifiedNote` says the
75 runs were made at. Stamping today's text would claim a verification that did not
happen. Use the gate's own extractor rather than an ad-hoc hash, because the
section boundaries are its definition:

```bash
SCRATCH=/private/tmp/claude-501/-Users-sofo-Projects-misc-arb-lab/510f3433-14f6-43b5-b359-702de8764c90/scratchpad
cp docs/state-machines.md "$SCRATCH/sm-current.md"
git show 310e854:docs/state-machines.md > docs/state-machines.md
```

Add the entry to `AmbiguousOrder.subjects` with a deliberately wrong digest so the
gate prints the real one:

```json
        {
          "doc": "state-machines.md",
          "section": "1",
          "role": "premise",
          "why": "PLACEHOLDER",
          "sha256": "0000000000000000000000000000000000000000000000000000000000000000"
        }
```

```bash
make formal-fresh   # FAILS, and names the actual digest of §1 at 310e854
```
Record that value as `<VERIFIED_DIGEST>`. Then restore and read today's:

```bash
cp "$SCRATCH/sm-current.md" docs/state-machines.md
make formal-fresh   # FAILS again, naming the CURRENT digest
```
Record that as `<CURRENT_DIGEST>`.

- [ ] **Step 3: Write the real entry**

Replace the placeholder with:

```json
        {
          "doc": "state-machines.md",
          "section": "1",
          "role": "premise",
          "why": "added 2026-08-15 by #242. Core §5.3 is already tracked because 'the model's state vocabulary comes from here', and §5.3's own pointer sends a reader to [mechanism: State-Machines §1] for the state meanings and the full transition table — so the section the correspondence map's §2 rows actually resolve against was untracked while the section that points AT it was tracked. §2's finality contract is a rule about WHICH transitions an evidence class licenses; an edit to the edge set can therefore invalidate a recorded justification without touching a run. FM-7 is the evidence for tracking what a justification consumes.",
          "sha256": "<VERIFIED_DIGEST>",
          "knownStale": {
            "task": "#229",
            "note": "Stamped at 310e854 and stale on TWO deltas, neither of which any run of this model has seen. (1) v0.9.34 added Canceled to the two query-resolution rows. (2) #242 (v0.9.61) gave Submitted the query-resolution row it never had — the third axis of v0.9.11's finding, and the most reachable, since every successful Place lands there. #229 owns this model's re-run and restamps this row with the rest. Delta (2) WIDENS the edge set the §2 correspondence rows quantify over, so it is inside that re-run rather than beside it: any 'the only legal target is X' reasoning in the map has to be re-derived against a Submitted row that now has six.",
            "sha256Actual": "<CURRENT_DIGEST>"
          }
        }
```

- [ ] **Step 4: Run the gate**

Run: `make formal-fresh`
Expected: PASS — the marker pins the delta rather than absorbing it.

- [ ] **Step 5: Prove the marker is not an exemption**

The gate's own contract: *"a further edit to the same section fails again"*. Verify
it, because a marker that swallowed later edits would be worse than no tracking:

```bash
printf '\n<!-- probe -->\n' >> docs/state-machines.md   # NOT inside §1 — see below
make formal-fresh                                        # still PASSES: the probe is outside the section
git checkout docs/state-machines.md
```
Then the real probe — insert the comment **inside** §1, between the States and
Transitions headings:

```bash
make formal-fresh   # must FAIL, naming §1 and a digest that is neither stamp
git checkout docs/state-machines.md && make formal-fresh
```
Expected: the out-of-section probe passes and the in-section probe fails. The pair
is what shows the extractor's boundaries are real; the second alone would not
distinguish "tracks §1" from "tracks the whole file".

- [ ] **Step 6: Commit**

```bash
git add docs/formal/freshness.json
git commit -m "formal: SM §1 is a tracked premise of model (a)

The gate covered §5.3 because 'the model's state vocabulary comes from
here' and not the section §5.3 points at for the transition table, so a
slice could rewrite the order lifecycle and formal-fresh stayed green.
Stamped at the verified commit with a knownStale marker naming #229 and
pinning both deltas."
```

---

## Task 7: slice close

**Files:**
- Create: `docs/superpowers/specs/2026-08-15-m4-242-order-finality-decisions.md`
- Modify: `docs/HANDOFF.md`
- Modify: `docs/superpowers/plans/2026-08-15-m4-236-crossvenue-pair-root.md` (the `BLOCKED` block)

- [ ] **Step 1: Run the full gate**

```bash
make ci
```
Expected: PASS, both modules. If `tutorial-ka-check` fails, the Georgian tutorial is
**generated** — regenerate it, never hand-edit.

- [ ] **Step 2: Run the marker grep over both paths**

```bash
grep -rn "SPEC EDIT OWED" internal/ tools/
```
Expected: no output. The SM §1 edit landed in Task 1 as `[SPEC EDIT LANDED — v0.9.61]`.

- [ ] **Step 3: The reading pass the grep cannot perform**

CLAUDE.md: the grep *"cannot see an UNMARKED claim"*. Read what this slice asserts
normatively and confirm each is either in the spec or marked:

- `advance`'s claim that a present order may never reach `AbsentAfterFinality` — is
  it in SM §1 or §5.3, or is it a new rule this code invented? (It is FLAG-2's
  evidence-class rule applied to a new site; state which sentence it rests on, and
  if none exists, mark it.)
- The join's stricter-than-`Resolver` sweep gate for non-terminal targets (§0.2 /
  Task 3 step 6) — a deviation from SM §2's "no sweep needed for a live reading",
  in the safe direction. Decide: mark it, or record it as a deliberate local
  strengthening with its reason.
- `Contradicted`'s mapping to `NeedsOperator` — is the boot outcome vocabulary's
  cell definition in `boot`'s own docs wide enough to hold it?

- [ ] **Step 4: Write the decision record**

`docs/superpowers/specs/2026-08-15-m4-242-order-finality-decisions.md`, following the
house shape — **why**, and the alternatives **rejected with their grounds**:

1. The finding sharpened: not "nothing calls `ApplyState`" but "SM §1 gives
   `Submitted` no query-resolution row", with the two places that require one.
2. The departure from the parent record's §5 requirement 1, with §0's three
   arguments and the operator ruling of 2026-08-15.
3. Rejected: the derived walk through `Acknowledged` — needs a rule about inferable
   intermediates, and shortest-path is unsafe (`AbsentAfterFinality → Canceled` via
   `PartiallyFilled` invents a fill).
4. Rejected: a sibling `Settler` use case — duplicates SM §2 step 3's sweep gate and
   doubles the venue spend on #236's poll loop.
5. Rejected: widening `Resolver`'s resolvable set — fuses two evidence classes.
6. The placement argument (§0.2) and the orthogonality decision (advance despite
   `Unattributed`).
7. §0.1's two observations: `CancelPending → PartiallyFilled` has no target, and
   SM §1's transitions table has now been found incomplete **three times on the same
   shape** — a fix applied to the states it was noticed on. Propose the cheap
   instrument: the `specEdges` oracle is a flat transcription, so a *reverse* pass —
   for each state, which of the five ACL readings has no target — is a page of work
   and would have found all three.
8. The `formal-fresh` finding: SM §1 was tracked by neither model.
9. `internal/app/exercise/app.go:142` still treats `StateSubmitted` as success. Left
   alone deliberately — the exercise root places one order and exits, and it calls
   no join — but it is the consumer whose success condition hid this for four
   slices, so it is named here rather than left to be re-derived.

- [ ] **Step 5: Update `docs/HANDOFF.md`**

New session block at the top, previous marked HISTORICAL. It must carry: the
sharpened finding; that #236 is **unblocked**; the two tasks in #236's plan that
change (task 6 `drive.establish` now reads a terminal state the join produces, task
5 `venueDouble` gains the venue-status responses); the new `knownStale` marker count
(six, five #229's plus this one — verify by counting) and that #229 now owes SM §1's
re-read too.

- [ ] **Step 6: Unblock the #236 plan**

In `docs/superpowers/plans/2026-08-15-m4-236-crossvenue-pair-root.md`, replace the
`BLOCKED` block with a note that the dependency landed, naming the commit, and
correct D4's mechanism sentence: `Module.Join.Join([id])` now advances the leg's
state, so `Module.LegState(id)` reads a terminal one. Do not re-plan #236 here.

- [ ] **Step 7: Commit**

```bash
git add docs/ && git commit -m "m4 #242 closes: the order lifecycle reaches a terminal

SM §1 gave Submitted no query-resolution row while its own invariant
resolves 'DispatchAuthorized or beyond' by query and §6.7 lists Submitted
among the states quiesced to finality — the third axis of v0.9.11's
finding, and the one every successful Place lands on. The join now
journals the venue's reading it has held since slice F2. #236 unblocked."
```

---

## Self-Review

**1. Spec coverage.** The parent record's §5:

| Requirement | Task |
|---|---|
| §5.1 — an OMS-owned operation applying the venue's read state as journaled, legal transitions, exposed through `oms.Module` only | Task 3. Exposure is already correct: `Module.Join` is an `*application.Joiner`, and `singlewriter` forbids roots importing `ports`, not `application`. **The walk is replaced by the missing row (§0), which is this plan's one departure.** |
| §5.2 — a decision about where it runs | §0.2, answered "inside the join" with four grounds and the `Resolver` alternative rejected. |
| §5.3 — integration tests starting from a real successful `Place`, walking to each terminal: full fill, partial-then-canceled, zero-fill canceled, rejection, unresolved status | Task 5, all five, plus the replay round-trip. Task 5 step 2 is the check that they fail on the pre-slice tree. |
| §5.4 — a `make formal-fresh` check | Task 6, and it goes past running the gate: SM §1 was tracked by neither model, so green was vacuous. |
| §6 — what #236 inherits | Task 7 step 6. |

**2. Placeholder scan.** Two steps deliberately name a value the executor must read
rather than one I can state: `resolve_test.go`'s reach helper (Task 2 step 3) and
`reconcile_test.go`'s fixture helpers (Task 4 step 1). Each says so explicitly and
names the file to read. The
mutation line numbers in Task 3 step 9 are `<placeholders>` **on purpose** — the
edits in step 6 move them, and a stale literal would produce a `NO-OP!!` that reads
as a passing gate. `<VERIFIED_DIGEST>` / `<CURRENT_DIGEST>` in Task 6 are produced
by steps 2's own commands.

**3. Type consistency.** `NewJoiner`'s new parameter is `jr ports.Journal` in
position 4 (after `orders`) in Task 3 steps 4, 7 and in Tasks 5's harness.
`Join.Advanced int` and `Join.Contradicted []ids.ClientOrderID` are spelled
identically in Tasks 3, 4 and 5. `ErrVenueStateNotAnAbsence` is the same name in the
sentinel block, `advance`, and `TestJoinRefusesProvedAbsenceFromAPresentOrder`.
`venueSays` and `stateOf` are defined once in `join_test.go` (Task 3 step 1) and
reused by Tasks 4 and 5, which share the `application_test` package.

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-08-15-m4-242-order-finality-transition.md`.
