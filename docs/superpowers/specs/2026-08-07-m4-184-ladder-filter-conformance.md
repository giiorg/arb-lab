# Record §77 — SM §7's ladder re-verifies §2.4's grid and §69's row date, per leg

**Task #184. 2026-08-07 (session 121). Docs v0.9.58.**

Task #184 was a deferral against a named trigger — *"the single-order `Dispatch`
path's first real caller"* — and that caller landed in #177, two slices back. By
this repo's own discipline (*"the gate landed the same session as its subject,
which is now the rule rather than the exception"*) it was debt, not backlog. It
was also the right thing to land **before** §7.1's escalation trigger: wiring the
trigger is what makes delever legs get dispatched in anger, and landing the
re-check afterwards would have shipped a window in which a safety leg goes to a
venue unconformed while a position nears liquidation.

---

## 1. The deferral's first option is refuted, and the refutation is the design

#184 named two candidate homes: *"whether the conformance+freshness pair moves
onto `Dispatch` itself or onto a shared preflight both `Executor` and `Deleverer`
call; `Executor.preflight` already holds the argument in full."*

The first is refuted by a comment in `execute.go`, written by §62d:

> ONE admission instant for the whole package: a two-leg package judged at two
> instants could straddle the filter-age bound and refuse asymmetrically, which
> is the package-atomicity harm §62d closed on the conformance side re-opened on
> the freshness side.

A freshness check inside `Dispatch` reads **its own instant, per leg, inside the
executor's fan-out, after siblings may already have been sent**. That is §62d's
harm verbatim, re-opened for a third time and this time from inside the very
method the fix would live in. The conformance half could survive there (the map
is immutable in-process); the freshness half cannot, and splitting them is what
`filters.go` exists to forbid.

**So the ruling is: the check belongs at the caller's ADMISSION UNIT — the set of
legs that must be refused together — and the unit is precisely what `Dispatch`
cannot know.**

| Enforcer | Admission unit | Why |
|---|---|---|
| `Executor.preflight` | the whole package | its legs go out in one fan-out; refusing leg 2 after leg 1 was sent is the asymmetric-rejection harm |
| `Deleverer.verifyLeg` | one leg | SM §7 sequences its two **deliberately** — the spot cap is the *confirmed* perp reduction, which does not exist until `PerpReduceFinal`, so there is no shared instant to stamp |

A row that was fresh for the perp leg may be stale for the spot leg. That is
correct rather than an inconsistency: the two legs are separated by a venue round
trip and a finality wait, and reusing the perp leg's verdict would be a freshness
claim about a row nobody re-read.

### 1a. The token alternative, and why it was rejected

A stronger shape exists and was considered: make `Dispatch` take an unexported
`admission` value that only the policy can mint, so the check becomes
*structurally* unskippable while the instant stays the caller's. It is this
repo's recorded move for a check nothing can make (*"a signature closes what a
check cannot"*), and it would also remove the preflight/dispatch duplication.

It was rejected on cost, not principle. `Dispatch` is exercised directly by an
external test package across ~29 construction sites; an unexported argument makes
the exported method uncallable from outside `application`, and unexporting the
method deletes that suite's whole approach. The claim it would buy — *no future
caller can forget* — is bought more cheaply and less completely by the policy
move below, and the residual risk (a future third dispatch-capable use case that
forgets to call `admits`) is named here rather than left silent.

---

## 2. The POLICY moves even though the CHECK does not

Splitting the two questions is what makes the answer work.

The map and its row-age bound are one policy (§69), and there is no honest reason
for the process's two admission-time enforcers to hold different ones. So
`filterPolicy` lands on the `Dispatcher` — **held and deliberately not read** —
and both use cases inherit it, exactly as they already inherit the
`DispatchAuthority`. `NewExecutor` **loses** two parameters; `NewDeleverer` gains
none.

That is §59b's argument on a second object. Its own wording: *"a composition seam
that could pass the dispatcher one authority and the executor another would
fragment the double-dispatch backstop … the fix is this repo's recorded move for
a check nothing can make: close it with the signature."* Here the fragmentation
is two grids in one process, and it has a specific realistic accident: a root
wiring `NewDeleverer(..., nil, 0)`. That compiles, validates (`len==0 && age==0`
is the legal reconcile-only shape) and refuses **every delever leg** with
`ErrFiltersUnknown` — a halt, at the moment a package is nearing liquidation. The
worse variant is silent: a map with entries but from an older revision, which
conforms legs against a grid the executor would refuse.

**The Planner keeps its own map, and that is NOT the same fragmentation.** Plan
time and admission time are different enforcement points, and the entire reason
admission re-verifies is that the planning revision may differ (`filters.go`).
Merging them would delete the check's own premise.

### Rejected alternatives

| Rejected | Why |
|---|---|
| Conformance in `Dispatch`, freshness at the caller | Splits one policy across two layers; `filters.go`'s header says two statements of "on the venue's grid" is how they come to disagree. It would also make the preflight check conformance twice per leg |
| `Dispatch` takes the admission instant as a parameter | Puts a decision about time in a signature whose doc explicitly refuses one, and the executor would pass an instant it had already used — dead weight on the path with the least room for it |
| `NewDeleverer(d, c, j, filters, maxAge)` | ~15 test sites instead of ~51, and it leaves the divergence expressible and silent. Deriving is what makes it inexpressible |
| An exported `FilterPolicy` the root builds once and passes twice | Makes the wrong wiring *visible* rather than *inexpressible*, which is a weaker standard than §59b's, and a future third enforcer would still need to remember the parameter |
| Fold the grid check into `Dispatch` as a backstop *in addition* | Every caller must still call `admits`; only the policy is shared either way. The backstop would only duplicate the conformance half — see row 1 |

---

## 3. The stranding question, which is the sharp part

`SubmitSpotReduction` is deliberately **un-premise-guarded**, and the reason is a
model run: guarded, TLC returned a 16-state behaviour ending in stuttering — a
generation authorizes zero spot, §2's determined branch compensates the residual,
and the generation sits at `SpotReduceAuthorized` unable to finish. Refusing to
send an already-reserved reduction does not *prevent* a commitment; it **strands**
one, which is what §7's quiesce rule forbids.

A filter refusal at that same rung looks like the same shape. It is not, and both
halves of the distinction matter:

1. **An off-grid order is one the VENUE refuses too.** Refusing locally forfeits
   nothing and gains a diagnosis. The premise guard, by contrast, would have
   refused an order the venue would have filled — it converted a *live* option
   into a stuck state, where this converts a *doomed* one into a named one.
2. **The refusal is RECOVERABLE where a venue rejection is not.** Everything
   `verifyLeg` does is a read, taken before any generation record is journaled
   and before the aggregate advances, so the generation stays exactly where it
   was and the same rung admits a leg replanned onto the grid. A venue rejection
   burns the ClientOrderID's single unrepeatable `Place`.

Point 2 is a property of *placement*, not of the check, which is why it has its
own fixture and its own mutant (journal-before-verify fails only that test).

---

## 4. Finding: the quantity members can only bite on a quantity the generation fixed

`verifyLeg` requires the leg's quantity to **equal** the generation's own figure
— `g.Target()` for the perp leg, `g.Authorized()` for the spot leg. So §2.4's
three quantity members (lot step, min qty, min notional) are reachable from this
ladder only when the generation itself fixed an off-grid quantity — and in that
world **no conforming leg can ever be planned**. The rung is stuck for the
generation's life; the exits are a supersession or a terminal flatten.

This is **not** a defect the slice introduced. Before it, the same order reached
the venue and was refused by the same filter — later, with a burned id and no
diagnosis. What the slice did was make the shape the *only* route to those three
branches, which is what made it visible.

Filed as **task #206** rather than fixed, and the reason is a boundary
`filters.go` already draws: the fix for an off-grid size is to **round it onto
the grid**, which is a derivation, and *"the DERIVING quantizer … is §6 step 4's
and lives with sizing."* The caller that would do the rounding is §7.1's
escalation trigger, which does not exist yet. Deciding now would move the stuck
state one rung earlier without removing it.

**The recoverable member is the PRICE**, which the generation does not pin — and
that is what the anti-stranding fixture varies. The fixture that would obviously
have been written (an off-step quantity) cannot demonstrate recoverability at
all, and would have read as demonstrating it.

---

## 5. What the fixtures had to vary, and why the suite could not see the gap

Every existing delever fixture conforms: legs sized in whole multiples of both
steps at a limit price on both ticks. So *"the check runs"* and *"the check does
not run"* gave the same answer everywhere — the collapse tell, ninth-plus
instance: **the fixture did not contain the thing under test.** Adding the check
alone would have left it unwitnessed.

Six new tests, each varying one predicate:

| Test | Varies | Kills |
|---|---|---|
| off-grid perp leg | lot step | the check missing entirely; a check placed after the send or after the record |
| off-tick spot leg + conforming re-presentation | price tick, then recovery | a refusal that consumes the rung |
| stale row | the row's **date** only | `conformsToFilters` substituted for `admits` |
| per-leg instant (23h30m vs 25h against a 24h bound) | **when** the row is judged | a cached per-generation instant |
| missized **and** off-step | two rules at once | the two checks swapped |
| no row for the spot instrument (perp row present) | which instrument is looked up | an empty policy; a hardcoded instrument |

The last one uses a **partial** map rather than an empty one deliberately: an
empty map refuses both legs and could not tell *"consults the dispatcher's map"*
from *"refuses everything"*.

---

## 6. The campaign

14 mutants, **all caught**. Three are isolated to exactly the test whose claim
they break, which is the result worth having:

| Mutant | Effect | Verdict |
|---|---|---|
| M1 `verifyLeg` never enforces the policy | `err != nil && false` | caught (5) |
| M2 conformance only — the row date is never judged | early `return nil` | caught (6) |
| M3 future-row sentinel fires on PAST rows | `After` → `Before` | caught (46) |
| M4 the age bound is exclusive | `>` → `>=` | caught (2) |
| M5 the ladder holds its own empty policy | `d.filters` → `filterPolicy{}` | caught (19) |
| M6 the executor holds its own empty policy | same | caught (28) |
| M7 a zero bound over a non-empty map is admitted | `<=` → `<` | caught (1) |
| M8 a positive bound over an empty map is admitted | `!=` → `<` | caught (1) |
| M9 `newDispatcher` drops the validated policy | `filters` → `filterPolicy{}` | caught (47) |
| M10 the grid check runs BEFORE the quantity check | block swap (by hand) | caught — **only the ordering test** |
| M11 the instant comes from the record, not the clock | `d.clk.Now()` → `rec.AuthorizedAt.Instant()` | caught (18) |
| M13 `conformsToFilters` substituted for `admits` | two-line edit (by hand) | caught — **only the two ladder-freshness tests** |
| M14 journal before verify in `SubmitSpotReduction` | block swap (by hand) | caught — **only the recoverability test** |

Three re-aims are worth recording because the same idiom keeps producing them:
deleting an `if err := f(...); err != nil {` guard by sed orphans `err`
(`NOBUILD!!`), and `err != nil && false` is what works on it; a mutation needing
a `time.Time` in a file that does not import `time` needs one from the arguments
(`rec.AuthorizedAt.Instant()`); and a mutation spanning two lines is outside
`mutate.sh`'s line scope and has to be done by hand.

**One claim has no single-line mutant, and the limit is stated rather than
implied.** "The instant is re-read per leg" can only be broken by *caching* it,
which is a design change and not an edit. What the per-instant test defends is a
future refactor; today the claim holds because there is nowhere to cache it.

---

## 7. What changed

| Where | What |
|---|---|
| `application/filters.go` | `filterPolicy` (map + bound as one value), `newFilterPolicy` (the three wiring refusals), `admits` (conformance **then** date, at a caller-chosen instant) |
| `application/dispatch.go` | the `Dispatcher` holds the policy and does not read it; `NewDispatcher` takes it; unexported `newDispatcher` over an already-validated one |
| `application/execute.go` | `NewExecutor(d, reserver)` — two parameters gone; preflight calls `admits`; the fan-out's dispatchers inherit the policy |
| `application/delever.go` | `Deleverer` inherits the policy and the clock; `verifyLeg` gains a **fifth** check, last |
| `application/errors.go` | `ErrFilterMaxAgeInvalid` is no longer the executor's; the two freshness sentinels no longer say "package" |
| `oms/module.go` | one policy beside the one authority, with the reason |
| `docs/` | core §2.4 (three enforcement points; the instant belongs to the unit), §6 steps 5–6, SM §7 (a new invariant), record §77 |

## 8. What is not done

- **§7.1's liq-distance escalation still does not call `Open`.** Milestone 4's
  other workstream, and now the next thing: the ladder is verified end to end and
  has no trigger.
- **Task #206** — the off-grid target/authorization, deliberately deferred to
  the workstream that will own the sizing.
- **Task #205** — `Classifier.Track`'s second caller, unchanged by this slice.
