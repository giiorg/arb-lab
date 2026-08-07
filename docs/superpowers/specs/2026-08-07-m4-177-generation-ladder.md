# Design record §74 — M4 task #177: SM §7's `PackageDelever` generation ladder, and the four rulings that only building it could produce

**Date:** 2026-08-07 · **Sessions:** 117–118 · **Docs:** v0.9.55 (spec edits
LANDED, session 118 — §74g) · **Task:** #177 · **Opens:** #203, and fires #184's
stated trigger

**Reads:** core §5.3 (the `PackageDelever` command and its sequencing), core
§5.1 (the OMS aggregate and its journal), core §6 step 5 (the planning use case
owns order creation), core §6 step 6 (marketable IOCs), core §6 step 7 / §6.7
(the classifier, the reserved-reduction ledger and its three producers), core
§7.1 (the liq-distance ladder this escalates from), core §7.6 (shed classes),
`state-machines` §2 (the finality contract) and §7 (the machine this
implements), `docs/formal/AmbiguousOrder.tla` + `ambiguous-order-model.md`
(`FM-5`…`FM-9`, `INV9`/`INV10`, `NC21`/`NC22`/`NC23`, `P5`/`P6`).

**Builds on:** §54 (slice E — the classifier, the ledger, and the half of `FM-8`
that landed there), §46 (model (a)).

---

## §74a — What the task was, and what "FM-9's consumer" meant

Slice E made `FM-5`, `FM-6` and `FM-7` executable and left `FM-8` half-landed
and `FM-9` deferred, for one reason: both need a `PackageDelever` generation,
and SM §7's ladder did not exist. `FM-9` in particular is a rule about a
**quantity** — the entry test re-evaluated net of the generation's own legs —
and a quantity with no subject cannot be computed. So the deliverable was the
ladder, and `FM-9` becoming a claim the types make was the point of building it.

Three things §7 requires are not expressible in an aggregate, and they are what
the use-case layer exists for here:

1. **FLAG-3's durability.** `SpotReduceAuthorized` is "a durable journal record
   … so recovery knows the perp-finality precondition was met without
   re-derivation", and an in-memory phase field is not that.
2. **The ledger staying in step with the ladder.** §7's *exactly backed at every
   instant* invariant is a statement about **two objects** — the generation's
   commitment and the shared entry — so only something holding both can keep it.
3. **Discharging the premise-lost supersession.** Model (a)'s `P5`/`NC23`: *"a
   standing obligation nothing is obliged to discharge is not a standing
   obligation."* It is a **liveness** requirement, so no invariant finds it, and
   an aggregate that can only *refuse* a step cannot satisfy it — refusing is
   precisely the *merely permitted* shape the control was written against.

That third one decided the shape of the whole use case: every step that commits
new exposure routes through a reassessment that **performs** the supersession,
and there is no call through which the obligation is merely reported. A caller
cannot decline to discharge because no interface offers the choice.

---

## §74b — Ruling 1: the quiesce obligation needs NO cancel port

All three supersession rows require the generation's reserved second leg to
reach §6.7 finality **first**, and SM §7 worded that as *cancel/quiesce*, with
one row's parenthetical reading *"cancelling its reserved second leg"*.

Implementing it appeared to require a `Cancel` on §4's venue contract. There
isn't one, and there never has been across three slices of venue work (C1, C2a,
C2b). The natural reading of that absence is *an omission this task must close*.

**It is not.** Core §6 step 6 sends nothing but **marketable IOCs**, so every
order this system places is immediately terminal at the venue. "Quiesce" is
therefore **awaiting** finality — the venue's own resolution composed with SM
§2's four-step contract — and a `Cancel` verb would be a path with no caller
whose existence weakened the argument that every order is immediately terminal.

The wording also hid the question that actually matters, which is whether the
wait can hang. It cannot, and the reason is structural rather than a timeout:
the obligation binds on exactly two phases (`SpotReduceInFlight` and the
spot-side park), and **every successor of that pair is either the other member
of it or an ending** — the submitted leg reaches `Complete` or parks, the park
reaches `Complete` or `TerminalFlatten`. So no path leads out of the pair except
by finishing the generation, which discharges the obligation by finishing its
subject. There is no path on which a dissolved premise leaves a generation
running.

Landed in SM §7 as its own invariant, and in core §5.3 because it is a statement
about this command's own vocabulary.

**Rejected:** adding `Cancel` to `ports.OrderPlacer` "for completeness". It
would be a method with no caller and no test that could distinguish a correct
implementation from a stub, and — worse — its existence is an assertion that
some order in this system is not immediately terminal, which is a premise
several other rules rest on.

---

## §74c — Ruling 2: two thresholds on one counter, and the ordering that follows

`FM-9`'s netting must remove *this generation's own entry* from the subtrahend.
The only handle the ledger offers is the **producer** (`ProducerPackageDeleverSpot`),
so the netting is correct exactly while **one** such entry exists; with two, it
removes a **sibling's** entry as well, and the premise a generation is judged
against then contains another generation's subject. That is `FM-9` itself,
arriving one layer out — through the decomposition rather than through the
trigger.

So the premise computation refuses at more than one entry. Meanwhile SM §7's
entry rule — "only after the prior generation reaches §6.7 finality" — is a
claim about the same counter at **zero**. Two rules, one counter, two
thresholds.

**The ordering is what reconciles them, and it is normative.** Reserving the
entry **before** asking for the premise makes a stale sibling the *second*
entry, so the existing refusal **is** the entry gate — one check serving both
rules. Asked in the natural order (premise, then reserve) a stale sibling reads
as a count of one and is silently netted out, which is the defect above.

Two consequences are recorded with it rather than left to be rediscovered:

- **Every refusal after the reserve must release the entry.** The ledger is
  mutated before admissibility is known, and an entry left behind subtracts from
  every later sizing for the life of the process, on behalf of a generation that
  does not exist. In the code this is why `Open` splits its body at the reserve:
  a `defer` reading a named error return is the shape where a later edit adds a
  success path that tidies away a **live** generation's entry.
- **The release error is joined, not substituted.** The caller's question is why
  the generation was refused; a ledger the use case could not tidy is a second,
  separate fault that must not hide the first.

**Rejected:** trusting the in-process registry of live generations as the gate.
A generation left in the shared ledger by a **previous process** is invisible to
it, and that is exactly the case the durable ladder exists for.

---

## §74d — Ruling 3: an ending RELEASES the entry, it does not amend it to zero

Both leave the entry subtracting nothing, so the ledger's **final contents**
cannot distinguish them. That is why the rule had to be stated about the **act**.

What the entry carries at the instant a generation ends is **real exposure the
generation put on the package and did not take back** — a superseded generation
that bought one perp and sold no spot leaves the group genuinely net **long** by
one. Releasing says so: the residual moves by that amount, §2 sees a naked
residual, and its determined branch compensates it. Amending to zero first
reports the same final ledger while asserting the exposure went somewhere, so
the group reads flat while holding a leg.

Its companion ruling is narrower and was found by the two external rows'
guards. **A supersession retires the entry only if that supersession is what
ENDED the generation.** `EmergencyClosePackage` and `FenceReadyLost` are not
guarded on the generation still being active — deliberately, because the way a
caller satisfies "quiesce first" is to drive the leg to finality, which takes
the generation to `Complete`, so the ordinary arrival of an external arbitration
mid-generation is **quiesce-then-supersede** and by then the completion already
retired the entry. Retiring unconditionally would release an entry the ledger no
longer holds, and the refusal surfaces **after** the row was durably journaled —
an arbitration recorded as having ended a generation the process still believes
is live. So activity is read **before** the ending is applied, and read rather
than inferred from the path, because either external row can arrive at either
kind of generation.

---

## §74e — Ruling 4: the ladder's resumption is a property of the PATH

This is the one the implementation carried as `[SPEC EDIT OWED]`, and it is the
only ruling of the four that is a genuine open question rather than a
composition defect.

SM §7's three supersession rows say a **generation** ends. None of them says
whether the **ladder** does — whether core §7.1 may open a later generation.

**Model (a) does not answer it and cannot be read as doing so.** `Supersede`
requires `superseded = "none"` and sets `superseded' = path`; `PdPlan` requires
`superseded = "none"` as well. So after any supersession, `PdPlan` is
**permanently disabled** in that behaviour. One supersession per behaviour is a
**state-space bound** of exactly the kind the companion's own clause map already
flags for `MAX_GEN` ("the model's own bound, not the spec's") — not a rule
anybody stated.

**And the companion asserted the opposite, in a parenthetical.** Its
bounded-generations row read *"`PdPlan`'s guard `pd = "None"`, reachable only via
`PdEnds` from `Complete`/`TerminalFlatten` (or via `Supersede`)"* — naming a
route that reaches the guard while permanently disabling the action. Corrected
in place. A parenthetical is exactly where such a claim survives review: it
reads as a completeness note about reachability, and it was wrong about the one
thing it asserted.

**The ruling, decided by the failure directions:**

- **The two external arbitrations do NOT resume.** `EmergencyClosePackage` is
  the kill-switch flatten; `FenceReadyLost` means writer identity or the fence is
  in doubt. In both, the foundation has taken the package **away from** the
  leverage-reduction ladder, and a generation opening afterwards is this machine
  re-authorising itself against an arbitration that just fired. Cost of being
  wrong: a breach needs a fresh escalation to reach it — an alert.
- **The premise-lost path DOES resume**, and refusing is the unsafe direction.
  The premise dissolving means the package acquired a naked residual, which §2's
  determined branch compensates; once it has, the package is neutral again and
  the perp short may still be nearing liquidation. Refusing would disable the one
  mechanism that reduces leverage on a neutral package for the rest of the
  process's life, at exactly the moment it is needed — the stuck-state shape
  `FM-8`'s own ruling was taken to prevent. Nothing is loosened by resuming: the
  entry precondition still binds, so a later generation opens only on a package
  that is genuinely `Determined` and flat again.

One rule gives the two classes opposite answers, which is what makes it a
property of the **path** rather than an exemption for one row. And because the
two failure directions are asymmetric and neither is derivable from an order
journal, **the path is carried on the durable supersession record** rather than
re-derived at boot.

---

## §74f — The two findings that are not rulings

**The ledger's SIGN.** `FM-5` fixed what an entry's **amount** is and was silent
on its **direction**, and the two readings differ by a sign on every entry.
`IssueFloor` was booking the floor order's own exposure **change**; the ledger's
subtrahend wants the **exposure removed**, which is its negation. Under the
wrong reading, a group realized net long 1.0 with an in-flight sale of 0.4
computes a residual of **1.4**, and the determined branch sells 1.4 against 0.6
of naked long — **a naked leg minted by the rule that exists to prevent one**.

The tell that settles it without a model run is in the sentence itself: §6.7
describes the `PackageDelever` entry as *"the confirmed perp reduction not yet
matched by that sale"*, and a buy-to-reduce of `x` is `+x` of realized exposure
that only an entry of `+x` nets to zero. **Under the order-change reading the
two producers would need opposite signs from one sentence.** Model (a) encodes
the exposure reading for both — `Pend(ImplLo, "floor", TRUE)` and
`Pend(pdDone − pdSold, "pkgspot", FALSE)` — and is the authority, because §46
§8e bites here as everywhere in this family: no invariant phrased over this
ledger can see a sign error in it.

**`Held/Alert` is one row and must be two phases.** SM §7's table has a single
`Held/Alert` entered from either leg, and its resume row then says *"the HELD
LEG's §6.7 contract later reaches finality"* — so the machine must know which
leg parked, and a single state plus a remembered leg identity is two states with
extra steps. The sharper reason is supersession: the reserved **second** leg is
live at a spot-side park and does not exist at a perp-side one, so the
quiesce-first obligation binds on one and has nothing to bind on at the other.
§46 §10a had narrowed the perp-side park away as *"the harmless instance …
`pdDone = 0`"* and building it refuted that — a perp leg can partially fill and
**then** go ambiguous, so such a park holds a real net-long transient and a
nonzero reservation. Recorded as a correspondence note in the table rather than
left to each implementer to rediscover.

---

## §74g — What the evidence showed

Thirteen mutants, all caught. Two of them point **opposite ways on the same
signal**, and both are recorded because telling which is which is the work.

- **M3 SURVIVED and the mutant was WRONG.** Collapsing `premiseOrDischarge`'s
  three branches to "always return the premise" left the suite green — because
  the branches **report**, they do not enforce: `Generation.CheckAdvance`
  re-reads `Premise.Intact` one layer down and refuses a dissolved premise with
  the same sentinel. What the branches buy is the **message** (ended, or awaiting
  the reserved leg), which the phase no longer distinguishes. The discharge lives
  in the `reassess` call **above** the branches, so `M3b` — report the
  supersession, never perform it — is the mutation that actually negates the
  claim, and it is caught. That reading is now a comment, because *this looks
  like the guard and the guard is one layer down* is the reverse of the natural
  reading.
- **M12 SURVIVED and it was a REAL GAP.** `verifyLeg`'s hedge-group check
  deleted changed nothing, because every fixture in the file planted its legs on
  the group by construction — the collapse checklist's own tell, *the fixture
  does not contain the thing under test*. It needed a new test.

**The `exhaustiveswitch` gate bit on a test fake**, catching the fake journal's
`AppendGenerationPhase` switch on its first run. An unnamed `default` there
would let a later phase fall through to *recorded, no payload* while this suite
kept passing and the port's totality argument had quietly acquired a hole. The
gate policing the **evidence** rather than the subject is worth more than it
looks.

`make ci` green uncached on both modules; all six §3.6 gates pass. The
`SPEC EDIT OWED` grep over `internal/ tools/` is back to **zero**.

---

## §74h — Rejected alternatives (do not silently reopen)

- **One `AppendGeneration(record)` method on the journal port.** Rejected: a
  single record type needs a target on the opening, a client order id on each leg
  submission, a size on the authorization and a path on one ending — five
  optional fields whose zero values are legal, and **two are actively
  dangerous**. The zero **authorization** is a real, legal, terminal decision, so
  a dropped size and a genuine zero are the same bytes; an absent **leg id** reads
  as a generation with no own legs, which is `FM-9`'s netting evaluated over the
  wrong set, arriving through recovery. Six methods, and a phase carrying a
  payload is not reachable through the phase recorder at all.
- **Widening `ports.Journal`'s key rather than declaring a second port.**
  Rejected: its `client_id` column is `NOT NULL` and every chained preimage begins
  with it, while a generation's subject is a `GenerationID` that outlives either
  leg. A key that is "a string that is sometimes an order and sometimes a
  generation" is the optional-field shape moved into the **primary key**, where a
  lookup for one can match the other. **Separate ports is not separate stores** —
  one chain is the recommended adapter, and #203 carries it.
- **Journaling the perp leg's fills at generation scope.** Rejected: the
  quantity **is** the perp order's own cumulative execution, already durable at
  order scope and already rebuilt by §7.5's replay. A second record is two stores
  holding one number — the number §7's spot cap is sized from — with no rule for
  a replay that finds them disagreeing.
- **Guarding `SubmitSpotReduction` with the premise.** Rejected by a model run,
  not by argument: the amount was fixed at authorization and the entry is already
  in the shared ledger, so refusing to send **strands** a commitment rather than
  preventing one. Guarded, TLC returned a 16-state behaviour ending in
  stuttering — a generation authorizes zero spot, §2's determined branch
  correctly compensates, and the package ends physically flat and `Determined`
  while the generation sits at `SpotReduceAuthorized` unable to finish.
- **Admitting the premise-lost row on `Deleverer.Supersede`.** Rejected: it is
  this machine's own standing obligation, decided by the netted quantity, so an
  outside caller asserting it is a generation cancelled on nobody's evidence. It
  is reachable only through `Reassess`, which computes the quantity.
- **Letting `Deleverer` mint its own leg orders.** Rejected: §6 step 5 makes the
  planning use case *"the only way an order enters the system"*, so a leg minted
  here is an order with no journaled intent. It takes a reference and **verifies**
  it against the durable record instead — version, hedge group, product and exact
  quantity, each with its own distinct harm.

---

## §74i — What this task deliberately does NOT do

- **The journal adapter (task #203).** `ports.GenerationJournal` has only a test
  fake, so FLAG-3's durability is a contract nothing yet honours. The design fork
  is real and is written down: one chain over a widened subject column (migration
  needed) beats two chains, because it is the only shape in which *the
  authorization record precedes the spot order's* is a fact a replay can check
  rather than a discipline this package keeps.
- **§7.1's escalation TRIGGER.** Liq-distance on the mark stream, the
  `MarginGateway` top-up, sizing-by-live-tier — milestone 4's other workstream.
  What exists here is the machinery the ladder escalates **into**.
- **Filter-conforming the delever legs (task #184).** The `Deleverer` is the
  single-order `Dispatch` path's **first real caller**, which is #184's own stated
  trigger; its legs get no §2.4/§69 re-check today. Filed, not folded in — the
  fix belongs with the dispatch path rather than with this caller.
- **Anything about `EmergencyClosePackage`'s own both-leg close.** Only its
  supersession half touches this machine.

---

## §74j — Work items

| # | Item | State |
|---|---|---|
| 1 | `domain.Generation`, `GenerationPhase`, `SupersessionPath`, `Premise` — the aggregate, the ten-phase table, the never-net-short cap, the `Check*` seams | **done** |
| 2 | `ports.GenerationJournal` (six methods) + `application.Deleverer` — the ladder, FLAG-3's durability, the ledger kept in step, `Reassess` as `P5`/`NC23`'s discharge | **done** |
| 3 | `Classifier.PremiseFor` + `ReservedReductions.CountFor` — `FM-9`'s netted quantity, computed by the same fold and subtraction as the ordinary verdict | **done** |
| 4 | The `IssueFloor` sign fix (§74f) | **done** |
| 5 | Spec edits → v0.9.55: SM §7's five new invariants + the `Held/Alert` correspondence note, SM §2's sign invariant, core §6.7's sign clause, core §5.1's journal scope, core §5.3's two clauses, core §12 item 4's annotation, `ambiguous-order-model.md`'s correction | **done** |
| 6 | The `ports.GenerationJournal` adapter | **task #203** |
| 7 | Filter-conform the single-order dispatch path | **task #184** |
