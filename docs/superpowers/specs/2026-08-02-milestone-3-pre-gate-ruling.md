# §47 — Milestone 3 admitted before strategy #2's gate: the pre-gate foundation ruling (2026-08-02, session 73)

**Status: RULED by the user 2026-08-02 (this session). The §12 edit is LANDED
(doc set v0.9.28). This record reopens one rejected alternative from §42d,
states the ground on which it is reopened, and draws the boundary that keeps
the reopening from becoming the error §42d was right to fear.**

Mandate: the user's question — *"why can we not build the next milestones?"* —
asked against a roster that had milestone 2b's gate waiting on the calendar
until **~2026-09-30** with no code work in front of it.

The answer turned out to be in the record that rejected it. This is not a
change of mind about the kill-gate discipline: **the gate keeps its authority
over live capital, and milestones 5–6 stay hard-gated.** What changes is which
milestones the gate's verdict can actually invalidate.

## §47a — what reopened it: §42d's rejected alternative (i) contradicts §42d's rejected alternative (ii)

Strategy-#2 scoping (`2026-08-01-strategy-2-scoping.md` §42d) records two
rejections one sentence apart:

> **(i)** proceed to milestone 3 now and build the OMS against a killed v1 edge
> — rejected by the roster's own preamble; the OMS would be built **"on spec"
> for a strategy whose economics are unmeasured**.
>
> **(ii)** Rewrite the roster wholesale around strategy #2 — rejected: the
> roster's **milestones 3–6 are strategy-agnostic controls and survive
> unchanged**; a minimal dated edit is auditable, a rewrite is drift wearing a
> redesign's clothes.

**(ii) is the load-bearing claim of the whole re-sequencing** — it is *why* the
roster was annotated rather than rewritten after the v1 kill. And it says
milestones 3–6 survive a **complete strategy replacement** unchanged.

If that is true, then a gate verdict on strategy #2's economics **cannot
invalidate milestone 3**, and (i)'s stated ground — "built on spec for a
strategy whose economics are unmeasured" — has nothing to bite on. The two
rejections were written in the same paragraph and neither was read against the
other.

**This is the failure mode the repo already has a name for.** A rejected
alternative is an unchecked claim: the benefit a record credits a rejected
option with is asserted, never verified, because nobody re-derives a rejection.
§42d credited (i)'s rejection with avoiding rework, and the rework it names is
refuted by the sentence below it.

**The normative core sides with (ii), twice.**

- **§3.1:** *"The foundation runs every Intent through the **same** risk gate →
  OMS pipeline **regardless of strategy**."* Strategies are pure signal +
  sizing; they never manage orders.
- **§1.1:** *"The dimensional-`fixedpoint` money kernel, **the single-writer
  OMS**, the parity dispatcher, and the §3.6 CI gates are **foundational, not
  tiered***.*"

**Session 64 had already made this argument and applied it to one artifact
only.** Commissioning formal model (a), it reasoned — in as many words — that
the model *"is **not a bet on that gate**: core §6 is titled 'Trade lifecycle
(foundation, **strategy-agnostic**)', and the two-leg partial-failure problem is
identical for cash-and-carry spot+perp and strategy #2's perp+perp. Wasted only
if the **project** ends, not if strategy #2 ends. **Milestone 3 requires it
anyway.**"*

Every clause of that generalizes to milestone 3's Go, and none of it was
generalized. **The argument was not missing; it was scoped to the one artifact
somebody happened to want to build.** That is the more useful form of this
finding than the contradiction alone: a correct argument can sit in the record
for nine sessions, be acted on, and still leave the general case closed, because
nothing forces a session to ask what else its own reasoning covers.

**And the practice had already departed from the rule.** §12 milestone 3 names
its own safety kernel — *"the ambiguous-order-recovery state machine
(determined-vs-ambiguous residual, §5.3/§6.7) is **this milestone's** safety
kernel — model-test it **here***"* — and sessions 68–71 built exactly that,
pre-gate, as formal model (a), finding `FM-5`…`FM-9`. So milestone-3 work has
been in progress for four sessions. The operative rule was never *"no
milestone-3 work before the gate"*; it was *"no milestone-3 work in Go"* — a
distinction no record defends, and one that leaves nine findings as corrections
to prose that no implementation has ever been measured against.

## §47b — the partition: what the gate's verdict can and cannot invalidate

The test applied to each roster item is narrow and mechanical: **could a
strategy #2 gate verdict of FAIL change this artifact?** Not "is this useful
work" — the question is invalidation.

| Roster item | Gate can invalidate? | Ground |
|---|---|---|
| M3 — hash-chained WAL, deterministic `ClientOrderID`s, `DispatchAuthorized`→query-only crash boundary, `OrderByClientID`/`Fills` recovery, stable-not-found + Bybit history fallback, §6.7 classifier, net-delta invariant, reconcile-only boot | **No** | §3.1 (strategy-agnostic pipeline), §1.1 ("foundational, not tiered"), §42d (ii) |
| M3 — parallel-IOC two-leg execution | **No** | §6 step 6's pattern is *"identical base quantity, unique client IDs"* over two legs; **which** instruments is the Intent's content, not the OMS's |
| M4 — per-venue `Fence` (signer isolation + exchange-side disable), `FenceReady` monitoring, kill-switch modes + pre-authorized ladder, `audit` projection, operator control-plane security | **No** | Per-venue and per-authority, not per-strategy. Model (b) `FenceTakeover` already covers this composition pre-gate |
| M4 — `EmergencyPlan` **contents**, §4.1 account/margin posture verification | **Partly — yes** | §1.1's $1K tier names the covered legs as *"the Binance perp **and both spot IOC legs**"* — **cash-and-carry's leg shape**. Strategy #2 is perp-perp across two venues with dual-venue margin in `C_allocated` (§42e). The *executor* is generic; its **plan targets and posture checks are leg-shaped** |
| M5 — full backtester fidelity (adverse-selection, survivorship) | **Yes** | §41r caveat 1 is normative: *"no BTC cost figure transfers"*; fidelity models are measured per strategy |
| M6 — canary | **Yes** | Live capital, and §12 milestone 6 fixes **BTC-only scope** — cash-and-carry's |
| M7 — strategy #2 execution half | **Yes** | Definitionally |

**The ruling follows the partition, not the milestone numbering.** Milestone 4
splits; that is the only place the boundary is not a milestone edge, and it is
why this record draws it explicitly rather than saying "milestones 3 and 4".

## §47c — the ruling

**ADMITTED before strategy #2's gate:**

1. **Milestone 3 in full**, as §12 states it, with one scope note in §47e.
2. **Milestone 4's strategy-agnostic half only** — and *not in this
   workstream*. It is admitted, not scheduled; §12's existing
   canary-concurrent language (`[→ R1d-8]`) already governs its sequencing, and
   this record adds no schedule for it. Named here so the boundary is one
   partition, not two.

**STILL HARD-GATED on strategy #2's economic gate:**

3. **Milestone 4's leg-shape-dependent half** — `EmergencyPlan` contents, §4.1
   posture verification's leg roster, dual-venue margin.
4. **Milestones 5, 6 and 7** — unchanged from §42d.

**UNCHANGED in every respect: the gate's kill authority.** No live capital, no
mainnet keys, no promotion of any kind moves on this ruling. §12 milestone 6's
sequencing, §1.1's tiers, and §45p's pre-registered procedure are untouched.
This ruling moves **construction order only**.

## §47d — the residual risk, priced and accepted rather than argued away

(i)'s rejection was wrong about **rework**. It gestured at something real, and
naming it correctly is the difference between reopening a question and
forgetting why it was closed.

**The real risk is project mortality, not rework.** If strategy #2's gate also
fires, the project stops (CLAUDE.md: milestone 2 is a deliberate kill gate — if
measured net carry does not clear the hurdle, *the project stops*). Milestone 3
would then be correct, checked, strategy-agnostic code that nothing ever runs.
**The base rate is 1 gate fired out of 1 gate run.** That is not a small
number and it is not being waved at.

Priced against it:

- **Nothing built under this ruling is strategy-specific**, so a *third*
  strategy inherits all of it. The loss case is project death, not strategy
  death — a strictly narrower event than the one (i) worried about.
- **It converts `FM-1`…`FM-9` from prose into checked claims.** Nine defects
  currently exist as corrections to spec sentences that no implementation has
  been measured against. A model is worth more with an implementation to check
  against; the models' own value decays while that gap stays open.
- **The counterfactual is not free.** Capture burns **11.165 GiB/day** (§44e);
  the gate lands at **day 59** of an **85-day** local disk. The wait is not a
  quiet period, and the OMS is the **longest-lead-time item on the roster** —
  paying for it during a calendar-bound wait is the cheapest window that will
  exist.

**Accepted by the user 2026-08-02.** The acceptance is recorded here so a later
session finds a priced decision rather than an unexplained departure from
§42d.

## §47e — two scope findings surfaced while mapping the boundary

Both are recorded because they change milestone 3's real size, and neither was
visible from §12's text.

**1. `engine/venue`'s account/order half was never delivered.** §12 milestone 1
claims *"`engine/venue` (market-data **+ account/order capabilities**)"*.
`internal/engine/venue/` contains books, funding, sources and both venues' wire
decoders — **market data only**. There is no signed REST client, no `Place`, no
`OrderByClientID`, no `Fills`. Milestone 1 is recorded complete; half of that
line is not.

This is a **normative claim the code silently does not satisfy**, which is
precisely the limit the marker grep cannot cover — `grep -rn "SPEC EDIT OWED"`
gates *marked* claims and is blind to an unmarked one. It is annotated in §12
rather than left as a discovery for whoever first needs to place an order.
**It is milestone 3 work by dependency, and slice C below is where it lands.**

**2. The `Journal` port has no adapter.** `ports.Journal` is referenced only by
its own definition, `application/dispatch.go`, `ports/repository.go`,
`module.go` and tests. So `Dispatcher`'s crash-boundary claim — fsync
`DispatchAuthorized` **before** the non-transactional network write, §5.3's
single most safety-critical ordering — **is today proven only against a test
fake.** No durable hash-chained journal exists.

`module.go` is honest about the write repository being unwired and argues the
absence well. The journal's absence is the same shape one layer down and is
**not** stated anywhere: the port's doc comment says durability *"is the
contract of the first call"*, and no implementation has ever had to honour it.

**Consequence for sequencing:** slice A is the journal, and it is not
optional-first. Every recovery claim in milestone 3 reads what the journal
wrote; building recovery against a fake journal would prove the recovery logic
and nothing about the durability it rests on.

## §47f — the slice plan, sequenced against the formal models

Seven slices. The ordering constraint is that **each slice's claims must be
checkable against something that is not itself** — the repo's oracle
discipline — and that the model-backed slice (E) lands **after** its three
inputs exist, because model (a)'s findings are all about those inputs
disagreeing.

| | Slice | Delivers | Checked against |
|---|---|---|---|
| **A** | **Durable hash-chained journal** | The `ports.Journal` adapter: append-only, hash-chained, fsync-committed. §5.1's WAL. | An **independent replayer** as oracle (the `fixedpoint`/`ids` pattern). Bite-check the fsync and the chain link separately — a mutant that drops fsync must fail, and one that verifies a record against *itself* must fail |
| **B** | **Planning use case (§6 step 5)** | Deterministic `ClientOrderID` minting, `TradePlanned` record, `ApplyPlanned`'s first caller, OCC version threaded into `Dispatch`. **Closes `module.go`'s stated hole** — the write repository becomes composable because a planning path now produces records for it | Determinism: same plan → same IDs, across processes. `ids` already has its base32 oracle |
| **C** | **`engine/venue`'s account/order half** (§47e finding 1) | Signed REST client both venues; `Place`, `OrderByClientID`, `Fills`, Bybit `GET /v5/order/history` fallback. Attaches to §7.6's priority ladder via `platform/ratelimit` (built) | **Testnet only.** Venue-profile facts are normative and already recorded (profiles §1/§4/§7); nothing unverified enters |
| **D** | **Recovery: the stable-not-found rule** | Per-venue finality determination; `SubmissionUnknown` resolved **by query only**; Bybit realtime→history fallback (realtime holds ~500 closed orders and clears on server restart, §5.3); `AbsentAfterFinality` as proved absence, distinct from `Rejected` | SM §2's four-step finality contract. **Model (a) is the authority** — the contract's steps are already checked there |
| **E** | **§6.7 classifier — the safety kernel** | Determined vs ambiguous residual; the **three** classifier inputs; `outstanding_reserved_reductions`; the interval-floor generation. **This is where `FM-5`…`FM-9` become executable claims** | **Model (a) directly.** Read `docs/formal/ambiguous-order-model.md` §3's correspondence map before the code — it maps every SM §2/§7 clause to a construct or a stated omission |
| **F** | **Reconcile-only boot (§7.5) + continuous net-delta invariant** | Journal ⋈ venue truth; auto-recover known journaled client IDs; operator clearance only for genuinely foreign orders | Slice A's journal + slice D's queries. The invariant is §12's named $1K control |
| **G** | **Parallel-IOC + testnet chaos** | Two-leg parallel marketable-IOC (never raw MARKET), identical base quantity; **process-kill-mid-IOC** integration test | §9's rollout bar. The chaos test is the one that proves A–F composed, and it is the slice that can fail the others retroactively |

**Four standing hazards this plan inherits, each already paid for elsewhere:**

- **Slice E must not audit the ledger with the ledger.** §46/§8e's rule bit
  three times in model (a): an invariant built on the ledger cannot audit the
  ledger, which is why `FM-5`, `FM-8` and `FM-9` were invisible to every
  invariant. The Go implementation inherits the hazard unchanged — the
  classifier's checks must be derived from something other than the quantity
  they police.
- **The collapse checklist runs before the mutations, not after.** Nine
  instances now. Slice A's obvious trap is a **chain of length 1**, where
  "verify against the previous record" and "verify against the first record"
  are the same predicate. Slice G's is a **two-leg** fixture, where "reversed"
  and "swapped" coincide — `boot`'s three-closer finding, one leg down.
- **Any test reading a channel a mutation could stop feeding needs its own
  bound.** Slices C, D and G all wait on venue responses; a mutant that blocks
  reports `TIMEOUT!!`, which names no defect.
- **A fix applied in one place gets applied to its mirror.** Slice C builds two
  venues' clients; slice D builds two venues' finality rules. Whatever
  repetition, bound or isolation trick one earns, the other gets the same
  session.

**Gate check per §3.6's deferral rule:** a deferred analyzer must name **the
construct it reads**, and lands the session its subject appears. All six §3.6
gates are built; slice B is the first code to compose the write repository, and
`singlewriter` already polices both the port and the adapter by position, so no
new gate is due. **This is a claim to re-verify at slice B, not to inherit** —
rule 10's deferral named the wrong construct and cost 13 silent fallthroughs
that were reachable the whole time.

## §47g — rejected alternatives

**(i) Keep waiting until ~2026-09-30.** Rejected: its only recorded ground
(§42d (i)) is refuted by §3.1, §1.1 and by §42d (ii) itself. The genuine
concern underneath it — project mortality — is real, is priced in §47d, and
argues about *how much* to build, not about building nothing. Waiting also has
a cost the original rejection never weighed: the roster's longest-lead item
sits idle through the one calendar-bound window that exists for it.

**(ii) Admit all of milestones 3–6.** Rejected: milestones 5 and 6 are
genuinely strategy-shaped — §41r caveat 1 makes fidelity models per-strategy by
normative text, and milestone 6 fixes BTC-only canary scope. Admitting them
would commit exactly the "on spec" error (i) feared, in the one region where
that error is real. **(i) was not wrong to exist; it was aimed at the wrong
milestones.**

**(iii) Build milestone 3 informally, without the §12 edit.** Rejected: §12's
*"hard-gated on this gate"* is normative text in the core. An unrecorded
departure is drift, external review could not tell it from a mistake, and the
marker grep cannot see an unmarked claim (§47e finding 1 is what that costs).
A dated annotation naming this record is the auditable form.

**(iv) Re-sequence §12's roster to put the OMS before 2b.** Rejected on §42d
(ii)'s own reasoning, which stands: *a minimal dated edit is auditable, a
rewrite is drift wearing a redesign's clothes.* The roster order is still
correct **for live capital** — the gate must precede promotion. Only
construction order moves.

**(v) Build slice E alone — the model-backed safety kernel — and skip the
plumbing.** Tempting, because E is the part the models already cover and the
part with the most findings banked. Rejected: **E's three inputs are produced
by A, B, C and D.** The classifier reads realized delta from the fill ledger,
possible-delta intervals from order state and recovery, and
`outstanding_reserved_reductions` from producers that commit at different
moments (`FM-5`). Building it against fakes would reproduce, in Go, exactly the
gap this ruling exists to close: a checked kernel with nothing real underneath
it. The composition is also where both models found their first defect —
**never on a machine reviewed alone.**

## §47h — what lands in the normative core

- **§12 milestone 2b** — the hard-gate sentence gains a dated annotation
  narrowing its scope to milestones 5–6, milestone 7, and milestone 4's
  leg-shape-dependent half, naming this record.
- **§12 milestone 3** — annotated as admitted pre-gate, with §47c's boundary
  and the §47e scope findings.
- **§12 milestone 1** — annotated with §47e finding 1: `engine/venue`'s
  account/order half is **not** delivered and lands in milestone 3 slice C.
- **§1.1, §3.1, §5.3, §6, §6.7, SM §1/§2/§7** — **no change.** This ruling
  relies on them as written; that they needed no edit is the evidence that the
  partition follows the spec rather than bending it.
- **ADR §3** — dated block.
- **Doc set → v0.9.28.**
