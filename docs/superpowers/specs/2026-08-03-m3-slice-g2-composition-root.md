# Design record §58 — M3 slice G2: the composition root, and an exemption argued from one cause

**Date:** 2026-08-03 · **Session:** 87 · **Docs:** v0.9.39 · **Tasks:** #166, #168

Three slices closed recording that `platform/boot` had no importer outside
itself. G1's deferral finally named what it waited on rather than what it wanted
— *"a `venue.OrderClient` built from configuration"* — and this slice builds it:
§5.2's OMS configuration schema, §5.2's key refs resolved into the one type that
may hold a credential, the first `boot.Checklist` any production code
constructs, #168's storage gate, and `arblab reconcile run`.

Writing the configuration is what found the defect. It is the fourth slice in a
row whose finding came from a composition, and the first whose composition is a
**file**.

---

## §58a — FINDING 1: the exemption was argued from ONE cause and the predicate collects TWO

Slice F2 established §7.5's honest limit and ruled on it:

> An instrument whose exposure cannot be compared MUST be **reported** on every
> boot rather than silently omitted, because a clean report is otherwise read as
> a complete one. And it MUST NOT hold the posture: a leg whose invariant can
> never be asserted would keep a correct system below trading for ever.

Every word of that is about a **permanent, venue-imposed** gap — no venue
publishes a position surface for spot, a spot balance is not a position, and the
baseline that would make one comparable is undefined (task #179). The ruling is
right about that case.

The predicate the code collects on is different. `Joiner.assert` walks the
configured scopes, marks every instrument some scope covered, and puts the rest
in `Unasserted` — so membership is *no configured enumeration covered this
instrument*, and a **derivative** reaches it three ways: a scope the deployment
never declared, a scope declared for another venue or family, and an instrument
dropped from the configuration while the journal still holds an open order on
it. None of those is an instrument whose invariant can never be asserted. Each
is an instrument whose invariant **did not run**, over an account that may hold
exposure on it, where a position surface exists and was simply never queried.

**The harm, stated as narrowly as it is true.** `Assertion.Holds` excluded
`Unasserted` by design, so `Reconciler` reports `GateNetDeltaAsserted` =
`OutcomePassed`. The degenerate case is the one the wiring documented as legal —
`oms.Deps.PositionScopes` says in as many words that *"an empty list is legal and
asserts nothing"* — and with it the reconcile-only boot's central invariant
passes **having issued no position query at all**. The control that runs it is
`TestAPerpNoScopeCoversHoldsThePosture`, and it asserts the harm directly: the
fake records every scope it is asked for, and the list is empty.

The empty-list sentence has a correct reason attached to it — a scope must not be
derived from the instruments we trade, because the enumeration exists to find
exposure on instruments we have never heard of and a derived scope could not
widen past them. That reasoning forbids **narrowing** the scopes to what we know.
It does not license **nothing**, and the two were never separated.

**The fix is a reason, and the exemption belongs to it.**
`Unasserted` becomes `[]UnassertedInstrument`, each carrying an
`UnassertedReason` with a `HoldsPosture` method:

| Reason | Holds the posture | Because |
|---|---|---|
| `UnassertedNoPositionSurface` | no | no enumeration could ever cover it — F2's case, unchanged |
| `UnassertedScopeNotEnumerated` | **yes** | a surface exists and was not asked; the venue may hold exposure this system cannot account for |
| anything unrecognised | yes | the permissive default is an instrument silently outside the invariant |

A method on the reason rather than a set the caller recognises, mirroring
`UnprovedReason.RetryMayLift` down to the signature — the same shape of decision,
and a reader who has understood one has understood this one. `exhaustiveswitch`
forces a member added later to choose.

The discriminator is decidable here and it is a **venue fact** rather than a
configuration one: spot has no surface anywhere, everything else has one. That is
what makes the classification safe to make inside the use case.

**Why the review rounds passed over it.** Both halves are correct in isolation.
The tell was in the **test**, and the collapse checklist found it:
`TestJoinNamesTheLegItCannotAssert` planted exactly one order, on the spot leg,
so every instrument that could land in `Unasserted` was a spot one and the two
causes were literally the same answer. That is the checklist's own tell — *the
fixture does not contain the thing under test* — run on a REASON instead of on a
value.

> **The general rule, beside the recorded *hazard named for one input* and
> *reassuring claim generalized from one input*:** when a rule EXEMPTS something,
> the thing to check is not whether the exemption is sound but **what else lands
> in the bucket it is written over**. A justification argued from one cause
> governs every cause that reaches the same predicate.

## §58b — RULING: the config check and the use-case check are NOT the same check twice

`Config.Validate` also refuses a derivative instrument no scope covers
(`ErrScopeNotCovered`). That looks like the copy that drifts, and this repo's own
rule — `oms.New` adds no refusals of its own, because *"a second statement of the
same rule here is the copy that drifts"* — is what the objection would cite.

It does not apply, and the reason is which universe each one quantifies over.
The config check is about the **configured** instruments and can name the missing
field before the process opens a journal. The use-case check is about the
**journal's** instruments, and the journal outlives any one configuration file:
an operator who removes a symbol from `instruments` while an order on it is still
open produces exactly the case the config check cannot see. The second is a
strict superset in coverage and a strictly worse diagnosis, so both earn a place.

A **constructor** refusal in `NewJoiner` was considered and rejected: it would
have needed the instrument universe threaded into a use case that does not hold
one, and it catches a strict SUBSET of what the assert-time reason catches, at
the cost of a signature change and a third statement of one rule.

## §58c — FINDING 2: #168, and the condition is DURABILITY rather than locality

§11/§13 require the journal on a **local filesystem — ext4/xfs with write
barriers, never NFS/network FS** — and schedule the check to §7.5's boot posture
list. Three decisions came out of building it.

**It is a gate of its own.** §57g's rule — *a gate is only ever reported about
its OWN evidence* — predicted this: the replay's evidence is the records, and
this one's is the filesystem beneath them. Merging them is not merely untidy, and
the cost is mechanical: the storage check runs FIRST and its refusal LATCHES, and
`Checklist.Report` refuses a latched gate any non-latching outcome — including
the replay's own opening `OutcomePending`. One gate would turn a correct
sequencing into an error returned from the phase §57c established must never
return one.

**Its refusal is `OutcomeEvidenceUnusable`** — halted and latching, the same cell
a chain that will not verify occupies, and for the same argument: reconcile-only
permits `ActReconcile`, whose exposure-neutrality is a property of the act *given
trustworthy inputs*, and the `DeactivationPending` re-drive it performs is
terminal with no reactivation.

**It runs before the journal is opened**, because a database created on a network
filesystem is already the thing being refused.
`TestAStorageRefusalLatchesAndTheJournalIsNeverOPENED` asserts the ordering by
the only evidence that exists for it: no file afterwards.

**And it widened.** §13's words are about LOCALITY, and locality is a proxy for
**durability**: `tmpfs` satisfies every one of them, honours every fsync, and
loses the whole journal at the next reboot — which is the boot §7.5 exists for.
The gate is named `GateJournalDurable` for that reason and refuses a volatile
filesystem under its own sentinel. An **unrecognised** filesystem and a host this
build cannot inspect are refusals too: an unverifiable claim about durability is
not a verified one, and admitting the unknown is exactly how a network filesystem
arrives under a name nobody enumerated (a new client, a FUSE driver, a vendor
module). `overlay` is deliberately in neither table, so a container's default
mount refuses and an operator mounts a volume.

What this protects is the limit §7.5 already states about its own
tamper-evidence: *a chain proves nothing about records that were never seen*, so
truncation at the tail is undetectable — and a network filesystem is precisely
the condition under which records go missing with every remaining link still
verifying.

## §58d — FINDING 3: the posture cannot be the verdict, and the question that can be had no accessor

`Checklist.Posture` reduces over the **required** gates, which is §7.5's
default-deny mechanism and is not negotiable. This root declares all eight gates
(§57b's ruling, now with a producer) and five of them have no producer anywhere,
so the posture is `PostureReconcileOnly` on a boot that found nothing and on a
boot that found a foreign order. `Blocking()` has the same problem one layer
down: it names those five on every run, so a real finding arrives as one row
among six.

What distinguishes them is whether anything **latched** — §7.5's own distinction
between preconditions that clear themselves and ones that *alert for operator
clearance*. And `Outcome.latching` was unexported with no read-only accessor, so
the only way to discover whether a gate was latched was to call `Clear` and see
whether it succeeded: **to answer the question by performing the act that answers
it.** The tool §7.5 exists to feed could not ask what the section is about.

`Checklist.Latched` is the accessor and `Report.ExitClass` is the verdict:
`ExitOperator` when anything latched, `ExitIncomplete` when a gate this binary
PRODUCES did not conclude, `ExitClean` otherwise. The five structural gates are
named explicitly in that switch rather than left to fall through, so a gate added
to §7.5's roster has to choose a side.

## §58e — the schema, and what is deliberately not in it

`Config` is the OMS's slice of §5.2 and not the whole schema. Risk caps,
market-data freshness budgets, per-strategy params, the strategy registry and the
emergency policy have no consumer in a reconcile-only boot, and a field declared
for them would be a schema frozen against no reader — this repo's recorded shape
for a claim wearing a type annotation. The test for what belongs is mechanical:
every field is an argument something refuses to be built without.

Four of its rules are about two fields AGREEING, which is what no constructor
beneath it can check: every instrument and scope names the configured venue;
every instrument's family has an endpoint; every derivative instrument is covered
by a scope; and §5.3's propagation bound sits above the request timeout and
`recvWindow`.

**`Dialect` is one field selecting four collaborators** — codec, signer,
limit-header reader, envelope reader. Configuring them independently makes the
mixture expressible, and the costly mixture is not hypothetical: on Bybit a
refusal is stated in the body of an HTTP 200, so a client holding Binance's
envelope reader classifies every rejection as an **acceptance**.

## §58f — the requiredKinds claim was falsifiable, and this binary did not falsify it

`boot.requiredKinds` carries a pre-registered claim: every binary loads
configuration, reports on itself and installs a signal disposition — *"and if a
binary lands that legitimately has none of one of them — and a genuinely one-shot
tool would have no signal disposition worth installing — relax the rule rather
than declaring a phase that does nothing"*.

`arblab reconcile` is the first one-shot tool and it does not falsify it. The
disposition is load-bearing for the engine's own reason: this process opens a
SQLite journal and makes signed venue calls, and an interrupt during a slow join
under the default disposition kills it outright, with the WAL unlanded and the
handle released by the kernel rather than by `Close`. The window where a boot is
slowest is the window where an operator interrupts it, and this tool's entire
runtime IS that window.

The config phase is genuine work rather than a stub, and finding out *which* work
was the interesting part: **the sequence's own bounds cannot come from inside the
sequence**. `Sequence.Validate` refuses a teardown budget at or above §7.4's
watchdog threshold and checks it before the first phase starts, so a boot that
loaded its own configuration inside its `KindConfig` phase could not bound its
own teardown until after that phase had run unbounded. The caller performs the
bootstrap read; the config phase resolves §5.2's key refs into the one credential
this process holds, which is work that genuinely cannot happen earlier and whose
failure is an inability to start rather than a verdict.

## §58g — the battery, and the two dead guards it found

**26 kills.** Six first attempts returned `NO-OP!!` or `NOBUILD!!` and every one
was a bad sed against the recorded discriminators — three `declared and not
used`/`imported and not used` (an orphaned reader), one duplicate `case`, two
wrong line numbers.

**Three SURVIVED and each was a real result, not a gap in two of the three.**

1. `Assertion.Holds` tested `err != nil || holds`. `HoldsPosture` answers `true`
   for every value it also errors on, so no input reaches that branch with `err`
   non-nil and `holds` false — an **unreachable guard**, which is worse than no
   guard: it reads as handled and stops the next reader asking who guarantees the
   invariant. Removed, with the property it rested on stated at the call site and
   asserted by `HoldsPosture`'s own table.
2. `Report.ExitClass` consulted `Storage != nil || Cause != nil`. Every path that
   sets either also reports a produced gate below `Passed`, so the loop below
   already answers. Same shape, same removal — §7.5's authority is the checklist,
   and those two fields are the diagnosis an operator reads afterwards.
3. Swapping the two venues in `ErrInstrumentVenuesMixed`'s message is neutralised
   by the sort beside it — an **equivalent mutant**, re-aimed at the sort itself,
   which is where the claim lives.

**Three more SURVIVED and were genuine gaps, all of one family: a fixture whose
two claims were satisfied by one check.**

- `ExitClass`'s produced-gate case list was exercised only through
  `GateNetDeltaAsserted`, so dropping `GateJournalReplayed` from it survived. One
  row per member now.
- `Checklist.Latched`'s sort survived because the assertion ran once over a map
  walk — a coin flip about whether an unsorted implementation happened to answer
  correctly. 50 trials, on Send-and-Receive's lesson arriving on a map instead of
  a `select`.
- The `max_landing` bound: the fixture shipped `Timeout` and `RecvWindow` at the
  same value, so a `MaxLanding` equal to both satisfied either check and
  weakening one of them survived. Each case now moves the other bound out of the
  way, and both test EQUALITY, because "not above" is the requirement.

The same family bit during the checklist run too: the `ErrFamilyUnrouted` fixture
dropped an endpoint and left its rate-limit budgets, which `ratelimit.Profile`
refuses FIRST — so the case passed while never reaching the rule it is named
after.

## §58h — what is NOT in this slice

- **G3: parallel two-leg marketable-IOC and the process-kill-mid-IOC chaos test**
  (task #181). It needs a process to kill, which is what this slice built.
- **The remaining five gates.** `GateAccountPosture`,
  `GateDeactivationsResolved`, `GateMarginOpsResolved` and `GateWriterAuthority`
  still have no producer. The root declares them, which yields a correct and
  honest reconcile-only posture and is exactly why `ExitClass` exists.
- **Two venues.** `oms.Deps` holds one venue client and one account, so a
  cross-venue deployment runs two of these against two journals. The config now
  REFUSES the document that would have expressed otherwise, rather than leaving
  the shape available and wrong.
- **A metric for the boot's outcome.** The observability phase builds the
  registry and reports the configuration provenance; no §10 series is declared
  for the gates yet, because the label set is the gate roster and the value set
  is the outcome vocabulary, and both are still moving.
- **Clock calibration is stated rather than measured.** A boot that calibrated
  from the venue's server-time endpoint would be a venue call before the journal
  is replayed, which §3.2's ordering forbids. Both of §5.3's bounds take the band
  edge that refuses sooner, so a band that is too wide costs refusals and never a
  false proof.

## §58i — rejected alternatives

**(i) Keep `Unasserted` a bare instrument list and refuse the empty scope list at
the wiring.** Rejected. It fixes the degenerate case and none of the others — a
scope for the wrong family, a scope for another venue, an instrument the journal
holds and the config dropped all still reach the same bucket and are still
exempt. It also puts the whole rule in a composition root, where a second binary
re-derives it.

**(ii) Make every unasserted instrument hold the posture.** Rejected, and it is
the tempting two-line change. It re-opens exactly what F2 closed: the spot leg is
unassertable at every venue for ever, so a correct system with a cash-and-carry
position would sit below trading permanently — the
halts-itself-with-every-light-green failure, arriving through the fix for its
mirror.

**(iii) Report the storage refusal against `GateJournalReplayed`.** Rejected on
§57g and on a mechanical consequence: the latch would make the replay phase's own
`OutcomePending` an `ErrGateLatched`, and `Reconcile`'s error result means the
verdict could not be FILED — which `boot.Run` turns into an unwind and a dead
process. The correct sequencing would have produced the one failure §57c exists
to prevent.

**(iv) Derive the exit code from the posture.** Rejected — see §58d. It is the
obvious shape and it answers the same on every run of this binary.

**(v) Let the root declare only the gates it can report.** Rejected in §57b and
re-affirmed with a producer in hand: it is the shape that reads as tidy and
produces a boot that checked four of eight preconditions and reported
`PostureTrading`.
