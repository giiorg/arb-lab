# Design record §60 — #169: the live §6 step 7 hold, and the read that had two wrong defaults

**Date:** 2026-08-03 · **Session:** 89 · **Docs:** v0.9.40 (no spec edit owed — see §60f) · **Tasks:** #169

Task #169's remaining half was the LIVE path: G1 discharged recovery (a
resolution whose apply failed reports `OutcomeProjectionStale` against
`GateNetDeltaAsserted`), and `Outcome.Applied` reported the live fact with
nothing acting on it — a running process that sent an order and lost the
projection write would happily open the next package. §6 step 7's rule has been
normative since v0.9.30: *a false Applied on a dispatch that reached the venue
holds new trading until that order's state is re-derived from the journal*.
This slice gives the rule its producer and its reader, both inside the write
path.

---

## §60a — where the mapping lives: the Reconciler's settlement, not dispatch.go's deferral

`dispatch.go` deferred the mapping to "the composition root that owns a
boot.Checklist", written when nothing constructed one. Two slices later the
Reconciler settled what that division actually means: **the root OWNS the
checklist; the use case that holds the evidence REPORTS into it, handed the
checklist per call** — and reports each gate about its OWN evidence. The live
path takes the same settlement: `Executor.Execute(c *boot.Checklist, refs,
urgency)` files `GateNetDeltaAsserted → OutcomeProjectionStale` at the fan-in,
because the fan-in is the one place that knows a send happened and its apply
did not.

The signature is the safety half. A root that had to read
`Execution.HoldsNewTrading()` and file the report is a root that can forget
to — and a forgotten filing is exactly a stale projection nothing refuses an
opening over, discovered the day the projection lies. This repo's recorded move
for a check a caller can forget is to make the signature carry it (§59b's
lesson, G3's constructor); here the parameter does.

The filing happens after `wg.Wait`, in the caller's goroutine, and that
placement is forced rather than stylistic: the checklist is not safe for
concurrent use, so a per-leg report from inside the fan-out would race the
other legs'. That is also why `Dispatcher.Dispatch` does NOT take a checklist:
the executor's per-leg dispatchers run concurrently, and the one safe moment is
the fan-in. A direct-Dispatch caller (none exists in production) inherits the
duty `HoldsNewTrading`'s doc states.

## §60b — the admission read: both plain defaults are wrong, and the fold that is right needed a third rule

The task's own wording — "needs the risk gate to read a posture" — predates
G2/G3, and both obvious readings fail:

- **`Posture().Permits(ActOpen)`** is §59i(iv)'s rejected alternative: the
  full fold is default-deny over a roster that declares five producer-less
  gates, so it answers reconcile-only on every healthy boot forever. A gate
  that can never open is not a gate.
- **"refuse openings when anything is latched"** fails the OTHER way: a
  non-latching `OutcomeFailed` is a concluded HALTED verdict — a margin-mode
  check that ran and refused — and `Latched()` is empty. An admission built on
  the latch alone admits an opening over it. It also cannot answer for Safety
  packages at all: a latched `OutcomeEvidenceUnusable` must refuse safety
  execution (halted — the journal itself is untrustworthy) while a latched
  `OutcomeProjectionStale` must admit it, and the latch axis does not carry
  that difference. The posture axis does.

So the read is `boot.Checklist.PermitsNow(act)`: **fold over VERDICTS only** —
the type doc's third axis, now a predicate (`Outcome.verdict()`) — take the
weakest ceiling, compare against the act. `Unreported` and `Pending` say
nothing and constrain nothing, which is `ExitClass`'s produced-versus-
structural split available per act. Urgency maps to act by definition
(`UrgencyOpening → ActOpen`, `UrgencySafety → ActReduce`, `actFor`), so a
latched stale projection refuses the next opening and admits a safety package
with no case analysis anywhere — the posture algebra §7.5 already fixed
(v0.9.37: stop-opening AND latching) produces the asymmetry.

The third rule is the fold's identity. Only-the-answers folds to
`PostureTrading` on a FRESH checklist — the precise "nothing was checked and
everything passed" collapse `Weakest`'s halted identity exists to refuse,
reappearing one method over the moment the default-deny fold is set aside. So
`PermitsNow` is **fail-closed on an empty verdict set**: zero concluded checks
refuses every act, including safety, because a process whose checklist holds no
verdict is a process §7.5's boot has not run in. The two folds read "no
information" in opposite directions, and each direction is the safe one for
its own question.

## §60c — the loop closes inside the executor, and the root's gate stays

Sequence, one process: opening package → a sent leg's apply fails → fan-in
files the latch → next opening package → `admitPackage` refuses at
`ErrExecutionHeld`, before any fsync, with nothing sent → a safety package
still executes. The end-to-end test
(`TestASecondOpeningInTheSameProcessIsRefusedOverTheHold`) runs exactly that
sequence on one checklist, with FRESH legs in the second package so the refusal
cannot be the authority's or any per-order state's.

The exercise root's own gate (`ExitClass == ExitClean`, §59d) is unchanged and
is not redundant: it runs once per run and covers the whole roster's produced
gates; the executor's read runs per package. `Report.Boot` deliberately stays
the pre-execute snapshot — re-deriving it after a stale execute would rank the
latch as `ExitOperator` and bury the more specific fact that THIS run
dispatched (`ExitExecution`, exit 3, whose doc now records the latch). Inside a
one-shot binary the latch dies with the process, and that is correct:
`OutcomeProjectionStale`'s doc already states that the lift is the
re-derivation, which is a fresh process replaying the journal — precisely what
`arblab reconcile run` and the next exercise boot do.

`reconcile.Runner` gains `Checklist()` — the live value, not a copy — so the
composing root can hand it to the use cases that report into it. Clear rides
along; it remains the operator's on the same trust that keeps the phases
unexported.

## §60d — what the fixtures had to separate

- **One stale leg of two** (`failApplyID`, a per-ID discriminator the fakes
  could not previously express): the only shape separating "ANY sent leg's
  failed apply holds" from "ALL". The both-stale shape lives in the
  second-opening test, so the suite carries both cardinalities.
- **Two gates on every checklist fixture**: "the gate the executor files" and
  "whatever gate the checklist has" must be different answers; the filing test
  asserts the neighbour is untouched.
- **The undeclared-gate fixture carries a Passed verdict** on the gate it does
  declare, so `PermitsNow` would admit and the refusal isolates the
  declared-check — the two-gates-can-see-it trap, avoided by construction.
- **The non-latching-refusal fixture asserts `Latched()` is empty in-fixture**,
  so it provably separates "reads the verdicts" from "reads the latches".
- **The verdictless fixture runs BOTH urgencies**, because fail-closed must
  refuse safety too, and an opening-only fixture would let a
  safety-passes-by-default mutant through.
- **The authority probes take a FRESH checklist deliberately** (commented at
  both sites): the first execution latches its own, and the admission would
  otherwise refuse before the layer those tests isolate.
- The `PermitsNow` weakest-fold test runs **both declaration orders**, killing
  first-verdict-wins; the ceiling walk seats a Passed verdict beside every
  member under test, killing reads-the-wrong-gate.

## §60e — the battery

**16 mutations run to a verdict: 16 kills** (15 `caught`; 1 `TIMEOUT!!`
re-aimed — the exercise-wiring mutant `r.Checklist() → nil` hung the 60s
default budget because the chaos test's own bounded waits legitimately take
longer than that when no Place ever arrives; at `MUTATE_TIMEOUT=240s` those
bounds fired and the mutant died with 3 failing tests). Targets: `verdict()`
(Pending flipped in, Passed flipped out — the latter a two-line mutant that
worked because mutate.sh line-scopes only the FIRST sed expression, verified
from the printed diff before being believed), `PermitsNow` (fail-closed
deleted, fold direction reversed, ceiling comparison deleted), the admission
(nil misnamed, declared-check inverted, PermitsNow read inverted), `actFor`
(both mappings crossed), the fan-in (filing deleted, polarity reversed, wrong
outcome, wrong gate), and the root wiring.

**The one un-mutated claim is named**: `Execution.Hold` is the home for a
filing refusal the post-fan-out contract cannot return as an error, and it is
unreachable BY the admission's postcondition (gate declared + class permitted
⇒ the gate is unlatched or latched at exactly the outcome being repeated, and
`Report` admits both). A mutation of the assignment is an equivalent mutant by
that same argument; the field is checked-rather-than-assumed on the dispatcher
loop's precedent, and its doc carries the derivation so a future change to
`Report`'s refusal set re-opens it consciously.

## §60f — the reading pass: no spec edit owed

The slice asserts normatively only what the spec already says: §6 step 7's
hold sentence (landed v0.9.30), stop-opening's safety-stays-available and the
latch's no-self-lift (landed v0.9.37 with `OutcomeProjectionStale`), and
reconcile-first (§7.5). `PermitsNow` is checklist MACHINERY on `Latched`'s
precedent (G2, no edit), not new vocabulary; the outcomes and their two axes
are unchanged. Marker grep is 0 over both policed paths.

## §60g — what is NOT in this slice

- **A `risk` bounded context.** §3.1's "risk gate → OMS pipeline" full
  resident stays future work; `admitPackage` is the seam it will occupy, and
  the read it will make (`PermitsNow`) now exists.
- **CLI surfacing of a mid-run latch.** Exit 3 plus the per-leg report is the
  operator's signal; the latch itself is process-local and its lift is the
  next boot's re-derivation.
- **Direct-Dispatch filing.** `Dispatch` keeps its signature; no production
  caller dispatches outside the package path, and the concurrency argument in
  §60a is why the checklist cannot ride on it.
- **#170, #174, #179, #171** — unchanged.

## §60h — rejected alternatives

**(i) Constructor-injected checklist (`NewExecutor(d, c)`).** Wireable (the
checklist exists before the module) and rejected: the Reconciler's per-call
precedent keeps `oms.New`'s Deps free of boot coupling, and a per-call
parameter lets a future root run two checklists' lifetimes without a rebuild.

**(ii) Filing from inside `Dispatch`.** The checklist is single-goroutine; the
executor's legs are not. A mutex on the checklist to permit it would contradict
the type's own concurrency doc for the benefit of a caller that does not exist.

**(iii) Latch-only admission.** §60b: admits an opening over a concluded
`OutcomeFailed`, and cannot distinguish the two latches that answer opposite
ways for safety traffic.

**(iv) Re-deriving `Report.Boot` after the execute step.** §60c: it would
reclassify a dispatched-and-stale run from exit 3 to exit 1, burying
"dispatched" — the fact that makes a retry unsafe.

**(v) An `Execution.Held` bool instead of the filing.** A report of the fact
beside the fact's consequence is the root-remembers shape §60a rejects; the
filing IS the acting, and `HoldsNewTrading()` already reports.
