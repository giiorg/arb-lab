# Design record §59 — M3 slice G3: parallel-IOC, and the backstop that fragments per path

**Date:** 2026-08-03 · **Session:** 88 · **Docs:** v0.9.40 · **Tasks:** #181

Slice G2 built the process; this slice makes it dispatch. §47f's slice-G row is
the charter: *"two-leg parallel marketable-IOC (never raw MARKET), identical
base quantity; process-kill-mid-IOC integration test"*, checked against §9's
rollout bar, *"the one that proves A–F composed"*. Nothing in the repo executed
a plan before this slice — `Planner.Plan` and `Dispatcher.Dispatch` had zero
production callers — so the execution use case, the composition root that runs
it, and the chaos harness that kills it are all first arrivals.

---

## §59a — the executor: parallelism is a property of how a write reaches the dispatcher

§6 step 6 requires *parallel* marketable-IOC legs, and `Dispatcher` is
documented not safe for concurrent use — deliberately, with a stated rejection
of the mutex: *"serialisation is a property of how a write reaches this type,
decided one layer out."* `application.Executor` is that layer, and its design
takes the dispatcher's doc at its word rather than amending it.

**One dispatcher per leg, each confined to one goroutine, constructed per
call.** The design went through a slot-table stage worth recording because it
looked safer and held nothing: dispatchers keyed by minted leg index, kept
across `Execute` calls, promising that "the same order meets the same
admission history". But once §59b extracted the admission history into the
shared authority, a `Dispatcher` is five immutable references — a remembered
dispatcher and a fresh one answer identically, and a table whose only content
is what a shared collaborator already holds is a second copy of a fact. The
executor now constructs one dispatcher per leg per call, serially and before
the first fsync, and confines each to one goroutine. Same-id concurrency
inside one call is refused in preflight (duplicate ids; one trade and one
attempt make distinct ids imply distinct legs), which is also what keeps the
authority's check-then-mark window closed structurally.

The executor itself is documented not safe for concurrent use, on the same
§5.3 single-writer ground as everything else on this path: one strategy loop
plans and executes, and two concurrent `Execute` calls of one package would
present the same ids concurrently — the same-id window the authority
deliberately does not close.

## §59b — the finding this design surfaced: an in-process backstop that fragments per path

`Dispatcher.authorized` is the in-process double-dispatch backstop, and its own
doc states exactly when it is load-bearing: when the projection broke —
`Outcome.Applied` false leaves the record at `StatePlanned`, so the repository
*"would happily admit the order a second time, for an order that has already
been sent. The map is the mechanism that holds precisely when the projection is
the thing that broke."*

A second dispatch path fragments that mechanism. `Module.Dispatch` and every
executor slot would each hold a private map, so an order sent through one path
and re-presented to another — the module's dispatcher retried by hand after an
`Execute`, or vice versa — is checked against a map that never saw it. Every
durable layer still holds **except** in the broken-projection case, which is
the one case the map exists for. The backstop's guarantee was quietly scoped
"per path" the moment a second path existed, and nothing would have reported
the narrowing.

**The fix makes the sent-set process-wide:** `application.DispatchAuthority`,
a synchronised, monotone set of every `ClientOrderID` this process has
committed to `Place`. `NewDispatcher` now takes it as a required collaborator;
`oms.New` constructs ONE and threads it through the module's dispatcher and
every per-leg dispatcher the executor confines. The dispatcher's own concurrency contract is unchanged —
it is still goroutine-confined for its sequencing — the shared piece is only
the set, whose operations are membership and insert, both safe to interleave
because the set only grows.

What this does NOT fix, stated so nobody reads more into it: two concurrent
dispatches of the SAME id can still both pass the check before either marks
(check and mark sit on opposite sides of the authorization fsync, deliberately —
marking before the fsync would burn the id on a refusal). That window is closed
by serialisation per id, which the executor guarantees structurally (distinct
ids per call, one call at a time) and which `Module.Dispatch`'s caller owes as
before. The authority closes the cross-path hole, not the same-id race.

**The second half of the fix arrived from the battery (§59g): the WIRING of
the sharing was itself a claim no test could check.** A mutant handing the
executor a fresh authority in `oms.New` SURVIVED — the fragmentation is
observable only through a broken projection, which the real repository cannot
be made to produce — and the diagnosis was a test gap with no reachable test.
So the claim became a signature: `NewExecutor(d *Dispatcher)` inherits the
dispatcher's authority (and its journal, placer, repository and clock), a
second authority in one process stopped being expressible from outside the
package, and the package-internal fork mutant is caught by the cross-path
test. Slice B closed `Dispatch`'s unplanned-order hole the same way.

## §59c — pre-flight above every fsync: a package is refused whole or sent whole

§6 step 6 puts every caller-controllable refusal ABOVE the authorization fsync,
and the executor extends that rule from the order to the **package**: every
check that could refuse any leg runs before ANY leg's fsync. A version
conflict, a non-IOC order, a quantity mismatch, an unparseable id — discovered
at leg 2 after leg 1's fsync, each would strand a durable authorization (or a
sent order) on a package the caller thinks was refused. So `Execute` pre-loads
every record and refuses the whole package while nothing has been sent; its
`err != nil` contract is "no leg was attempted", mirroring `Dispatch`'s
"nothing was sent" one level up. After fan-out there is no composite error:
each leg's result speaks for itself, because a sent IOC cannot be un-sent and
an aggregate error would invite `if err != nil { return }` over live orders —
the exact trap `Dispatch`'s split signature exists to forbid.

Pre-flight checks, in order: non-empty; every id parses as OMS-minted; all legs
share one `TradeID` **and one attempt** (a package is one planning of one
trade, §6 step 5's "the attempt advances for the whole plan"); distinct leg
indices; every record loads at its expected version; every state admits
`StateDispatchAuthorized`; marketable-IOC pair; **identical base quantity**
across all legs, compared with `Quantity.Cmp` — which is asset-checked, so
"identical" includes "in the same base asset" and a BTC-spot/ETH-perp package
is a refusal, not a comparison.

The pre-flight is a read pass and the races it leaves are the ones `admit`
already documents: a write landing between pre-flight and a leg's own admit is
caught by that admit (nothing sent), or by OCC at the apply (after the send,
visible, never silent).

[SPEC EDIT LANDED — v0.9.40] §6 step 6 fixes the package-level rule: every
refusal a package can earn lands before any leg's authorization; after the
first authorization the package is not refusable, only reportable per leg.

## §59d — the exercise root: §9's cycle driver, and why it may act under PostureReconcileOnly

The chaos test needs a process that dispatches, and §9's acceptance gate
independently needs *"≥ 100 synchronized spot↔perp entry/exit cycles"* — a
synthetic exerciser of the spot↔perp path that is explicitly **not** a strategy
("this gate validates execution correctness + fault coverage, NOT strategy
economics"). Those are the same binary: `internal/app/exercise`, run as
`arblab exercise run -config <file>`.

Its config embeds `reconcile.Config` (the §5.2 OMS block, unchanged — one
schema, one owner) and adds a `trade` block: trade id, attempt, and two-or-more
legs of instrument/side/quantity/limit-price. Its boot is the reconcile root's
six phases **plus one**: signal → config → observability → storage → oms →
**reconcile** → execute. The ordering claim is the slice's core: an exercising
process runs §7.5's reconcile-only boot to conclusion FIRST, on the same
checklist, and the execute phase runs only on a clean verdict. That is A–F
composed, in the order the spec composes them, with G on top.

**The gate the execute phase reads is `ExitClass`, not the posture — and
executing under `PostureReconcileOnly` is deliberate.** G2 established that
this deployment's posture is reconcile-only on every boot, however healthy,
because five of §7.5's eight gates have no producer anywhere. A driver gated on
`PostureTrading` would be a driver that can never run, waiting on contexts
(risk, treasury, the emergency path) that §12's roster sequences after the
evidence this driver exists to produce. So the execute phase requires
`ExitClean` — every produced gate concluded, nothing latched — and the record
states plainly what that means: the exercise binary trades on testnet-class
targets under a posture that names five unverified preconditions. That is the
sub-`$1K` regime §7.4 and §12 already carve out, and the binary is not the
strategy engine; when the five gates gain producers, the same `ExitClass`
tightens by itself.

A latched checklist, an incomplete reconciliation, or a held posture refuses
the execute phase with nothing sent — the phase reports rather than unwinds,
on §7.5's alive-halted-audible rule.

## §59e — the chaos test: a deterministic kill window, and why testnet is not the harness

§9 names **process kill mid-IOC**, and the acceptance criteria are §9's:
journal recovery, the net-delta invariant, no duplicate or over-reduction. The
test (`internal/app/exercise/chaos_test.go`):

1. builds `arblab` from source into `t.TempDir()` — never over `bin/arblab`,
   which belongs to the running capture (standing rule);
2. starts a fake venue (httptest, mirroring `engine/venue`'s own codec
   fixtures) whose Place handler **signals receipt and then blocks**;
3. runs `arblab exercise run` against it and waits — bounded — until BOTH
   legs' Place requests have arrived, so the journal provably holds two
   authorizations and the process provably has two IOCs in flight;
4. SIGKILLs the process inside that window — the §5.3 crash boundary, hit
   exactly between the fsync and the transport result;
5. reconciles the survivor journal in-process (`reconcile.Run` against the
   same fake, now answering queries), and asserts: the authorized orders
   resolve by query rather than resubmission — the fake counts Place calls
   per client id and the count is 1, ever, across both runs; fills the fake
   discloses are applied; the net-delta assertion runs; the exit class matches
   the scenario.

The kill lands mid-flight **deterministically** because the venue's clock is
the test's: the handler blocks until killed. No sleep-and-hope, no race
against a real venue's latency. That is also the honest answer to the task
title's "testnet": a real testnet cannot hold an IOC open while a harness
kills the client, cannot count our Place calls, and needs credentials #171 has
not validated. The chaos evidence §9 wants is about OUR crash boundary, not
about a venue's matching engine — the venue-facts half of the testnet story is
#171's and stays there. Running the exercise binary against a real testnet is
an operational act this slice makes possible, not a CI assertion it makes.

## §59f — standing hazards, inherited and applied

- **The two-leg fixture collapses "reversed" and "swapped"** (§47f, boot's
  three-closer finding). Applied twice: the executor's routing tests use a
  THREE-leg plan wherever an ordering/routing claim is made, and the two-leg
  chaos fixture makes every dimension asymmetric — different products,
  different sides, different prices — so a leg-swap defect cannot cancel out.
  Assertions key on `ClientOrderID`, never on slice position.
- **Any test reading a channel a mutation could stop feeding needs its own
  bound.** Every wait on the fake venue's signals, on process exit, and on the
  executor's fan-in is a `select` with a deadline arm.
- **A fix applied in one place gets applied to its mirror.** The executor's
  fan-in is the `Send`-vs-`Receive` lesson's territory; whatever bound or
  repetition a leg-result path earns, the other legs' paths get in the same
  session.
- **The collapse checklist runs BEFORE the mutations.** The fixture parameters
  pinned to distinction-collapsing values here: leg count (2 where the
  mechanism needs 3), equal quantities (the identical-quantity check needs an
  UNEQUAL fixture to be shown to bite), and same-slot presentation order.

## §59g — the battery

**24 mutations run to a verdict: 23 caught, 1 resolved by making it
inexpressible.** Targets: the executor's preflight (empty, duplicate, mixed
trade, mixed attempt, stale version, terminal state, non-IOC, both quantity
directions, the asset-mismatch arm), the fan-out (serialised `go` removed —
caught by the gated-placer parallelism test, bounded in both directions),
fan-in (`wg.Wait` removed, result indexing collapsed to slot 0), the
aggregates (`HoldsNewTrading` guard flipped and widened, `Dispatched`
inverted), the authority (never marked, never consulted), the exercise root
(verdict gate deleted — caught by the message-asserting unclean-boot test,
because the halted-guard behind it wraps the same sentinel and would mask a
bare `errors.Is`; exit-class arms; leg-verdict arms), the config translation
(sides swapped, quantity minted in the quote asset), and the cmd mapping
(`ExitExecution` → 0, caught end-to-end by the rejected-leg binary test).

**The two survivors were the slice's last two findings.** (1) The
two-authorities wiring mutant — a gap with no reachable test, closed by
`NewExecutor`'s signature (§59b); its package-internal sibling (the executor
forking its own authority inside the constructor) is caught by the cross-path
test. (2) The one-sided comparator `cmp > 0` — the collapse checklist's tenth
instance, the first CAUGHT BY the battery rather than before it: the mismatch
fixture varied only upward, so `cmp != 0` and `cmp > 0` were the same
predicate on every fixture the suite held. Fixed with the 0.4 case; both
one-sided mutants now die. The lesson for the standing rule: running the
checklist before the mutations is necessary and not sufficient — neither
substitutes for the other.

**Re-aims:** two `NOBUILD!!` (an orphaned `cmp`, an orphaned `domain` import)
and two `NO-OP!!` (stale line numbers), each re-run to a real verdict.

## §59h — what is NOT in this slice

- **#169** — acting on `Outcome.Applied=false` (the projection-stale hold) in
  the running engine. The executor REPORTS per-leg outcomes including it;
  `boot.OutcomeProjectionStale`'s producer for a live engine is #169's.
- **Sizing and translation** (§6 step 4, `strategy.Intent` →
  `application.TradePlan`). The exercise config states legs explicitly; the
  translation that derives them from a `strategy/domain.Intent` — per-leg
  limit price and lot-quantized quantity — has no owner yet and is the
  milestone-3 translation `strategy/domain/intent.go` already names.
- **The close half of §6 step 9's lifecycle** beyond the same-pattern reuse:
  `EmergencyPlanStore.Deactivate`, hedge-group leases, tombstones (SM §6) have
  no producers.
- **≥100 cycles / 14 days** — §9's acceptance QUANTITIES are an operational
  campaign the exercise binary now makes runnable, not a CI test.
- **Testnet credentials** — #171 unchanged.

## §59i — rejected alternatives

**(i) A mutex inside `Dispatcher`.** Rejected by the type's own standing doc,
and re-affirmed here: it would make two concurrent same-id dispatches pass the
race detector while the check-then-mark window still double-Places. The
executor achieves parallelism with confinement instead, and the one shared
piece (§59b's authority) has no read-modify-write semantics to get wrong.

**(ii) Per-slot dispatchers with private maps — the no-refactor shape.**
Rejected for the reason §59b records: it silently scopes the double-dispatch
backstop "per path", and the case it abandons (a broken projection) is the one
the backstop exists for. The authority extraction touches the crash-boundary
file, which is a cost, and it buys a process-wide invariant that was previously
an accident of there being one path.

**(iii) Serialize the authorization section, parallelize only the Place.**
Rejected: §6 step 6 puts NOTHING between an order's authorization fsync and its
single Place — a batch phase between them would widen exactly the window §5.3's
recovery pays for, for every leg at once.

**(iv) Gate the exercise's execute phase on `PostureTrading`.** Rejected in
§59d: with five gates producer-less by design, it is a driver that can never
drive. The honest gate is the verdict this deployment already defined
(`ExitClean`), and the record states the regime it operates in.

**(v) Drive the chaos test against a real venue testnet.** Rejected in §59e:
no deterministic kill window, no Place-count oracle, and a credential
dependency (#171) that would couple this slice's evidence to an external
account. The venue-facts half of testnet validation is #171's own task.

**(vi) A test-only main built inside the test, instead of a real subcommand.**
Rejected: §9's acceptance gate independently needs the cycle driver as an
operational tool, and a test-only main would prove a composition no production
binary has — the checked kernel with nothing real underneath it, §47g (v)'s
shape one level up.

**(vii) Embed `reconcile.Config` anonymously in the exercise config.**
Rejected by the config kernel itself: `auditSchema` refuses embedded fields,
deliberately — promoted-field path naming and hot-tag semantics are undefined
there, "refused rather than half-supported", and defining them means owning
encoding/json's silent-drop collision rules, which this repo has already paid
for once. The block mounts under `"oms"` instead: §5.2's schema keeps its one
owner in `app/reconcile`, and where a binary mounts it is that binary's
document shape, not a second schema. The kernel's deferral now has a named
prospective consumer — a future slice that implements embedding implements it
for this case.

**(viii) Keep the executor's per-leg-slot dispatcher table.** Rejected in
§59a's final form: with the authority extracted, the table's entire promised
content ("the same order meets the same admission history") lives in the
shared authority, so the table was a second copy of a fact — and the routing
rule it required ("slot by minted leg index") was contract-honoring machinery
with no observable behaviour left to test.
