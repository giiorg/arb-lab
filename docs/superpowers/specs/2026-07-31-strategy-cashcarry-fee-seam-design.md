# Milestone 2 step 3 — the `strategy` context, `cashcarry`, and the fee seam: design (2026-07-31)

**Status:** design only — **nothing in this step is built.** Session 45. Implements §31a's step 3 row
(*"`strategy` context + `cashcarry` + the fee seam — §2.4's money math; no fee fetch path exists
today"*) toward §12.2's milestone-2 deliverable: *"funding/level signal, break-even (real spot+perp
fees), discrete-settlement capture, the `FundingForecast` port (§2.4)"* plus *"Paper decisions
journaled."* It inherits §39l's three open questions and §39q-5's note verbatim; **§40h answers
§39l-2 and §40i answers §39l-3**; §40k passes §39l-1 on unchanged, sharpened rather than pre-answered.

**Provenance note:** this record survived four adversarial review rounds (Codex, session 45) —
three complete, each initially NO-SHIP, each accepting the prior round's repairs and digging one
level deeper, and a fourth killed mid-run whose formed findings were recovered and applied.
Round 1: two blockers (the `Decision` had no declared path across the §3.3 boundary; the gate's
cost terms had no declared inputs) and four majors — five of six applied, one partially refuted on
§2.4's own text. Round 2, all six applied: the `HoldPlan` interval-flip staleness (§40f), the
undeclared production inbound door (§40f), the missing lot-rules port/input/identity (§40b/§40g),
the container's periodic-fsync reality against the write-ahead claim (§40h), the missing lossless
wire encoding for dimensional values (§40f), and two measured quantities absent from the class-2
roster plus the unassigned `BasisForecast` default adapter (§40e/§40j). Round 3, all four applied:
the exit policy and §2.4's two Δ terms absent from the "complete" `DecisionInput` (§40b), the
errorless adapter wrongly claimed as milestone 3's production path (§40f — now compile-check only,
with the foundation-loop door a named milestone-3 debt), the slippage charge with no seam able to
produce it (§40e's `ExecutionCostModel`), and the settlement-history accumulator's parity hole
(§40c — one deterministic accumulator over the dispatched stream, never a runner fixture). Round 4
was killed mid-run (~10 min, before a verdict), and its two already-formed findings were recovered
from its log and applied rather than discarded: the revision **schema** belongs to the context, not
the kernel — `platform/config` supplies the machinery, `strategy/infrastructure/driven` declares
the types (§40j) — and the hurdle benchmark, required by `DecisionInput`, had **no supplying
port**, which folded `FeeSchedule` + `InstrumentRules` + the hurdle into the one `ReferenceData`
port they always were (three as-of faces of one dated-revision mechanism, §40b). A killed review
is not a verdict; what it had found is still evidence. **The review loop closed here,
deliberately:** four rounds moved from structural blockers to contract omissions to
placement-and-wording, every §2.4 term now has a declared carrier and every carrier a declared
owner, and the next verifier of these contracts is slice A itself — the compiler, the six gates,
and the battery are stronger checks of a frozen interface than a fifth prose round. The findings
are folded in below rather than appended, because a record is what a later session resumes from,
not a changelog.

**Doc set:** v0.9.16. The live statement of what to do next is at **`## RESUME HERE`** at the bottom.

---

## §40 — step 3's design

### Preamble — what step 3 is, and the one sentence that bounds it

Step 3 builds the **decision layer** of the kill gate: the context that turns normalized market
facts into a scored, decomposed, journal-ready entry/exit decision. It does **not** build the thing
that runs decisions over months of data (step 5), and it does **not** choose the measured numbers
the decision is scored against (step 4's pre-registered artifact, whose governance §8 puts *before*
scoring precisely so this step cannot lean on it). The boundary discipline throughout: **step 3
builds structure that step 4 fills with frozen numbers and step 5 drives with replayed data.**
Anywhere this record wants a measured number, it declares a validated input instead (§40e's
two-class rule says exactly which numbers those are).

### §40a — scope

**In:** the `internal/strategy/` context (full §3.2 anatomy); the strategy-owned value types §3.3
names (`MarketView`, `PositionView`, `Intent` + `IntentKind`/`LegSpec`, and §2.4's durable
`HoldPlan` and `SettlementForecast`); the `Strategy` inbound port **verbatim from §3.3 — unchanged**;
the **`DecisionInput`** value (§40b) and the **`Decision`** value (§40f) that bracket the core; the
`cashcarry` evaluation core — §2.4's entry inequality, the exit continue-vs-close score, the
hysteresis band + `exit_persist` dwell; the **`FundingForecast` port with its default estimator**
(imminent = live estimate; later = trailing realized-settlement EWMA clamped to live cap/floor);
the **`BasisForecast` port with §2.4's conservative default** (zero favorable convergence, charge
the measured adverse move — the default is an adapter, so "absent a configured estimator" is a
wiring state, not a nil check); the **fee seam** (§40d) — one face of the **`ReferenceData`
port** (fees, lot rules, the hurdle benchmark: three as-of faces of one dated-revision seam,
§40b); the **`ExecutionCostModel` port** (§40e — the slippage charge's seam); the **split read
seam** (`MarketStateReader` + `SettlementHistory`, §40c); the **`DecisionSink` port** (§40f/§40h —
how a Decision reaches the journal, durably); the typed `StrategyParams` of §5.2's per-strategy
roster; and
the `Decision` decomposition that makes §12.2's "paper decisions journaled" a property of the value
the core returns rather than of a runner that remembers to log.

**Out, with the reason each stays out:** the calibration artifact and every measured number in
it — adverse-drift dispersion (C3), the slippage model's numbers, utilization, the rescue-margin
figure (step 4; §8's governance forbids this step choosing them — their *input types* are in,
§40e); the research simulator, the replay-driven paper runner, and the **fee/reference revision
repository + replay applicator** (step 5 — pre-registered in §40d and §40k, because §8 requires
replayed reference revisions and §40d shows `platform/config` alone cannot provide them); the risk
gate, OMS, and any consumer of `Intent` (milestone 3 — in this milestone Intents terminate in the
paper journal); `crossvenue` (§12.7); live venue credentials (§40d rejects them for this
milestone); and the realized-settlement **history accumulator** wiring (§40c names the seam; step 5
wires it, because the accumulation window is a property of the replay run, not of the context).

### §40b — anatomy: the pure core takes ONE validated input; application assembles it from ports

**Decision: `cashcarry` is a domain package — `internal/strategy/domain/cashcarry` — whose
evaluation core is one pure function per direction (`Enter`, `Continue`) over a single validated
`DecisionInput`, returning a `Decision`. The application layer owns the ports, assembles the
input, and adapts the result to §3.3's `Strategy` interface.**

**`DecisionInput` is the complete roster of what the verdict may depend on** — the review round's
second blocker was precisely that the first draft let the core need terms (slippage, reserve,
hurdle, `C_allocated`, prior dwell state) that no declared input carried, which is how hidden
state, ambient fetches, or silently-zero costs enter a "pure" core. The roster, each with its
ground: the `MarketView` (§40c); the **policy projection** — the validated, revision-identified
snapshot of every `StrategyParams` value the core reads (`exit_band` with its artifact identity,
`exit_persist`, the decision quantile, the aggregation rule, the EWMA half-life…), because a core
that reads ambient params is not a function of its inputs, and the round-3 review caught that the
roster carried the dwell *state* while the band and dwell *policy* had no declared carrier; the
`FundingForecast` output over the `HoldPlan`'s settlements; the `BasisForecast` output; the
`LegFees` per leg with revision identity (§40d); the **per-leg lot rules** — lot increments + the
residual-delta tolerance, with their revision identity (§40g's quantization cannot run on ambient
data); the **candidate quantity** the sizing is being scored at (explicit, never inferred back out
of `C_allocated`); the **execution-cost/slippage charge** from the `ExecutionCostModel` port with
its model + calibration identity (§40e); the **hurdle benchmark revision** (§2.1's
dated/sourced/haircut-adjusted input, same revision pattern as fees); the **`C_allocated`
decomposition** (§2.2's five terms as a validated struct); the reserve/opportunity term; **the
closing-cost delta and the risk delta as separate dimensional members** — §2.4's exit score names
`Δ(closing cost: now vs later)` and `Δrisk` as distinct terms, and folding them into the generic
cost basket is how one gets silently charged to the wrong branch; for the exit direction, the
**position facts** (the `PositionView`: held quantity, entry evidence, the recorded entry
`HoldPlan` identity); the `HoldPlan` (entry: proposed; exit: the **current journaled revision**,
§40f's interval-flip rule); and the **prior exit-dwell state** (dwell count + last counted
*realized* settlement) — `exit_persist` counts *settlements*, not calls, so the dwell is an input
fed back from the prior journaled `Decision`, never a counter the core keeps (a core that keeps
state across calls is not the §2.4 "two conforming implementations decide alike" shape).
`DecisionInput.Validate()` refuses any **absent** member — every one of these terms reads as zero
when forgotten, and every zero flatters the gate. One deliberate distinction: the two Δ terms may
legitimately BE zero (§2.4's own "in the common case closing cost is branch-symmetric"), so for
them the refusal is of *absence*, not of zero — an explicit zero carried by the policy revision is
a declared judgement with provenance; a Go zero value with no declaration is the forgotten case
and is refused. That is the config-tag rule's shape: deny silence, permit stated exceptions.

The placement rejections, each with the failure it avoids:

- **Rejected: `internal/strategy/cashcarry` as a peer of `domain/`.** §3.2's context anatomy is
  three segments, and `hexboundary` keys on the `domain/`/`application/` path positions — a peer
  package would sit outside the gate that exists to police exactly this code. "Pure signal +
  sizing" (§3.1) is the property rule 1 enforces; putting the package where the rule cannot see it
  turns the property into a promise.
- **Rejected: cashcarry consuming `application/ports` interfaces directly from domain.** The gate
  does not forbid a `domain → application` import, which is precisely why the discipline has to be
  the design's: a domain core that resolves its own ports cannot be tested without fakes for every
  port, and a decision must be a function of its inputs, not of what a port answered mid-call.
  Accept dependencies as values; return results.
- **Rejected: the `Strategy` interface in `domain/`.** §3.3 calls it the *context inbound port*,
  and §3.2's anatomy places the `<Ctx>Service` interface in `application/`. Its
  `Params()`/hot-reload face is a configuration concern (§5.2), which domain has no business
  holding.

Concretely: `strategy/domain` holds the value types and `strategy/domain/cashcarry` the core;
`strategy/application` holds the `Strategy` interface and the `cashcarry` service that satisfies
it; `strategy/application/ports` holds the driven ports — `FundingForecast`, `BasisForecast`,
`ExecutionCostModel` (§40e), **`ReferenceData`** — three as-of faces, `Fees(inst, asOf)`,
`LotRules(inst, asOf)`, `Hurdle(asOf)`, **one port because they are one mechanism**: dated
reference revisions with provenance refusal, one store, one step-5 replay applicator; three
one-method ports would be three shallow doors over the same seam, and the round-4 review caught
the cost of naming them separately — the hurdle, required by `DecisionInput`, had no source at
all — `DecisionSink`, and the two consumer-owned read ports of §40c (`MarketStateReader`,
`SettlementHistory`) — **seven in all**;
`strategy/infrastructure/driven` holds the revision adapters (§40d). `Intent.ValidUntil` is stamped by the **application**
layer from the injected `Clock` — §3.6 rule 5 bars the domain from any ambient reading, and the
domain core has no reason to know the wall time at all: every age it judges arrives pre-derived
from the `MarketView` watermark against an instant the caller supplies, which is §3.4's read-time
re-judgement rule observed from the consumer side.

### §40c — `MarketView` and the read seam

**Decision: `MarketView` v1 carries exactly the raw materials §2.4 names, per instrument, plus the
§3.4 watermark — and nothing that exists to serve a consumer this milestone does not have.**
Fields, each with its ground: the perp's live funding estimate, live interval, `nextFundingTime`,
live cap/floor (§2.4's "raw materials" sentence; all already carried by `marketdata`'s
`FundingObservation`); the **recent realized settlements** window (the EWMA's input — §2.4 names it
a `MarketView` material explicitly); spot and perp reference prices with their watermarks (the
break-even needs notional, and the §3.4 skew/freshness verdicts need both legs' ages); and the
watermark itself (source seq, source time, recv time, skew) so the core can *refuse* a stale or
skew-breaching view — §2.4's entry gate is meaningless computed over data the freshness rule would
have rejected, and a refusal is a journaled `Decision` (§40f), not a silent skip.

**The read seam is TWO consumer-owned ports, because its two halves have different shapes and
different owners.** Both live in `strategy/application/ports`, both return `strategy/domain`
types, and the **application assembles `MarketView` from them** — the assembly is the service's
job, not a port's, so neither implementor needs to know the view's shape:

- **`MarketStateReader`** — the latest-value half: funding observation, reference prices,
  watermarks. `engine/state` implements it by translation, exactly where its doc comment left it
  (§3.3, [→ X5]). `engine/state` importing strategy-owned types is the allowed direction of every
  gate (rule 1 polices inward layers importing `engine/`, never the reverse).
- **`SettlementHistory`** — the realized-settlement window the EWMA needs. The latest-value cache
  is structurally the wrong home for a *window* (§3.4's cache holds one observation per source,
  and widening it into a ring buffer for one consumer is how a cache grows a second contract).
  Its adapter is **one deterministic accumulator over the DISPATCHED funding stream — the same
  component, fed by the same envelopes, in live and replay.** The round-3 review caught the first
  draft's parity hole here: an accumulator owned by the step-5 *runner* is replay-specific, so the
  later live service either lacks the window or grows a second, unrecorded accumulator — and then
  the forecast's inputs differ between the paths §3.5 requires to be the same engine. Sitting
  behind the dispatcher makes its state a pure function of the dispatch sequence, which is
  precisely the determinism §3.5 already guarantees. Step 5's root is its **first instantiation**;
  slice A declares the port shape and the accumulator's ownership (a strategy-context adapter
  wired by the composition root, not a runner fixture).

**Rejected: `marketdata` accumulating the window.** It would put a strategy-parameterized
retention policy (how many settlements matter is a `forecast_model` param) inside a context that
§3.2 scopes to normalize-and-publish, coupling the two across the boundary the read ports exist to
keep apart. **Rejected: one fat read port returning the assembled `MarketView`.** It forces every
implementor to know the view's full shape, so adding a view field touches `engine/state` — the
exact coupling §3.3's "strategy-owned type" exists to prevent; two narrow ports keep each
implementor answerable for only what it holds.

### §40d — the fee seam: fetched facts become revisions; the decision path reads revisions as-of

This is the step's headline decision, because §2.4's sentence — *"programmatically-fetched,
per-leg, per-venue maker/taker fees … No hardcoded fee constants"* — collides with three facts on
the record: **`GET /v5/account/fee-rate` requires Bybit authentication** (Profiles, dated),
**`GET /fapi/v1/commissionRate`'s security type is unverified** (Profiles records the gap
explicitly), and this project has **never held a venue credential** — the capture path is
public-only by design, and §7.3's key-handling discipline is milestone-4 machinery. A literal
reading of "programmatically-fetched" would make milestone 2's first credential a *fee lookup*,
standing up the blast-radius machinery of §7.3 out of order, for a number that an unfunded account
would report at its default tier anyway.

**Decision: the fee is a dated, sourced, revision-controlled input consumed through one port —
exactly the shape §2.1 already mandates for the hurdle benchmark — and "programmatically fetched"
is satisfied by a fetch that WRITES a revision, never by a fetch on the decision path.**

The end-state composition, stated now so the milestone-2 adapter is visibly a stage of it rather
than a detour: a fee fetcher (milestone 3/4, when credentials legitimately exist for the OMS) polls
the venue fee endpoints and **lands each response as a new fee-schedule revision** with provenance;
the decision path — live and replay alike — reads a revision through the port. This is forced, not
preferred: §8 scores the gate over *"replayed config/reference-data revisions"* and §3.5's parity
means the backtest cannot call a venue endpoint at scoring time — so **even with live credentials,
the decision path consumes a recorded revision**. The live fetch is one more *writer* of revisions.
Milestone 2 has a human as the writer, and the revision's `source` labels which — the same
labeled-provenance rule the manifest backfill established (§38): **a hand-recorded revision can
never impersonate a fetched one.**

**And the milestone-2 revision is named for what it is: a modeled-tier proxy, not the account's
fees.** The review round's sharpest valid objection: a future authenticated fetcher does not
retroactively make a hand-entered VIP0 table "programmatically fetched", and §12.2 promises "real
spot+perp fees". Provenance labels make the proxy *honest*; they do not make it *conforming* — so
the spec edit below has to authorize it explicitly rather than let the composition rule absorb it.
The authorization is defensible on the gate's own logic: VIP0 is the **worst published tier**, so
the proxy errs in the adverse direction — a kill gate scored on VIP0 fees is a *harder* gate, which
is the direction that is right when somebody forgets to update the revision. The requirement that
survives to live capital: **account-real, authenticated fee revisions are a precondition of any
live order, and the milestone-2 gate's verdict is labeled as scored on the modeled tier.**

Mechanics: the fee face of the `ReferenceData` port (§40b; consumer-owned, in
`strategy/application/ports`), answering per-`VenueInstrument` **as-of an instant** —
`Fees(inst, asOf) → LegFees` — with maker and taker,
**spot and perp fetched separately** (§2.4's explicit trap: spot taker is materially higher; four
`(venue, product)` cells for the BTC universe, each with both sides). `LegFees.Validate()` refuses
an unpopulated rate and refuses **missing provenance** (source + as-of date + revision hash): the
shared-kernel rule — *"an unpopulated rate reads as zero fee — computes the economic gate in our
favour with nothing failing"* — applies here with the kill gate itself as the blast radius, and a
schedule without provenance is a hardcoded constant wearing a config file's clothes.

**What `platform/config` does and does not provide — the first draft overclaimed this, and the
review round read the code.** `config.Store` holds one atomic *current* snapshot; `Reload` swaps
it without retaining the predecessor; `Revision.LoadedAt` dates a document, it does not archive
one. That machinery gives milestone 2's single-revision world exactly what it needs — a typed,
validated, dated schedule the sim loads at tape start — and gives a **historical** run nothing:
a scored sample spanning months needs *period-correct* fees, and an as-of lookup needs an archive
to look into. So the seam is built for the archive from day one (the `asOf` parameter; the
revision hash in every `Decision`, §40f), and the **append-only revision repository + the replay
applicator are pre-registered as step 5 work** (§40k) — they land with their first consumer, the
runner, under the standing "build it with the code it serves" preference; what step 3 refuses to do
is freeze a port shape that would have to change when they arrive.

Rejected alternatives, each with the failure it avoids:

- **Rejected: live authenticated fetch in milestone 2.** Pulls §7.3's credential surface forward by
  two milestones for a read; couples the kill gate's input to an account that does not exist yet
  (fee-rate endpoints answer for *the authenticated account's* tier); and still cannot serve the
  replayed gate, so the revision path gets built anyway — the fetch would add a credential, not
  remove a component.
- **Rejected: a programmatic importer of the public fee-schedule pages.** It answers the
  "programmatically fetched" words while inverting their intent: the venue fee *pages* are
  unversioned marketing HTML with no schema contract, so the importer is a scraper whose silent
  breakage produces exactly the stale constant §2.4 forbids — with a `fetched` provenance label
  attesting otherwise. A dated human verification into Profiles is weaker automation and stronger
  evidence.
- **Rejected: hardcoded VIP0 constants with an escape comment.** §2.4 forbids it in as many words,
  and the failure is the recorded one: a fee constant is a second copy the venue's next schedule
  change silently orphans, in the direction that flatters the gate.
- **Rejected: fees inside the calibration artifact.** The artifact freezes *measured thresholds*
  (§8's roster); fees are venue reference facts that change on the venue's calendar, not ours.
  Fusing them means a venue fee change forces a full artifact version + rerun (§8's rule) when
  nothing measured changed — and conversely tempts a "small" fee update to ride without the rerun,
  which is the artifact's governance dissolving.
- **Rejected: a `treasury`-owned fee service.** §3.2 scopes `treasury` to balances/collateral/
  custody, and it does not exist yet; inventing a context to hold one table is the shallow-module
  shape, and the consumer-owned port already gives `strategy` the fact without owning the fetch.

**The values themselves are venue facts and go through Profiles' discipline first.** The VIP0
maker/taker rates for Binance spot, Binance USD-M perp, Bybit spot and Bybit linear perp are
**verified into `venue-profiles.md` with dated sources during slice A** (public fee-schedule pages;
no credential needed for the *published* tier table), and the committed milestone-2 revision cites
those rows. Nothing goes in unverified — including these.

**One spec edit is owed and taken deliberately, in two clauses:** §2.4's cost sentence gains
(i) the composition rule — *fetched fees land as dated revisions; the decision path (live and
replay) reads a revision as-of its instant; a revision without provenance is refused* — and
(ii) the milestone-2 authorization — *the pre-capital gate MAY be scored on a sourced, dated,
published-tier revision, labeled modeled-tier; account-real authenticated revisions are a
precondition of live orders*. The code lands marked `[SPEC EDIT OWED]` at the port, and the edit
lands in the core before this step closes, under the standing discipline. Without clause (i), §2.4
reads as demanding the exact thing §3.5 forbids at scoring time; without clause (ii), the
milestone-2 gate is non-conforming the day it is scored and the record absorbs the violation
silently — the R2d distinction between deliberate change and drift is exactly this marker.

### §40e — `FundingForecast`, and the two classes of number

**Decision: the port implements §2.4's contract — per-settlement forecasts
`{ts, expRate, dispersion}` over the `HoldPlan`'s enumerated settlements — with the Go signature
taking the forecast's raw materials explicitly: `Estimate(materials, horizon) → []SettlementForecast`,
where `materials` is the `MarketView`'s funding block (live estimate, realized-settlement window,
live cap/floor, interval).** §2.4 writes the contract as `Estimate(inst, horizon)` *and in the same
paragraph* states that `MarketView` carries the estimator's raw materials — an implementation
honoring the literal two-argument signature must therefore hold market state internally or fetch it
mid-call, which contradicts the pure-core rule and makes the estimator untestable as a function.
The explicit-materials signature preserves the contract's semantics (same output shape, same
horizon rule, still keyed by instrument through the materials) and is recorded here as a **stated
reading, to be confirmed by the close's reading pass** — if the pass judges it a deviation rather
than a reading, it becomes a marked §2.4 edit at that moment, not a silent one.

The default estimator: imminent settlement = the live per-settlement estimate; later settlements =
trailing realized-settlement EWMA clamped to the **live** cap/floor from the materials, never a
stored copy. Pure math, so it lives in `strategy/domain` and is *wired* behind the application
port — the port earns its seam not from I/O but from §2.4's own requirement that the model be
swappable (`forecast_model`), hot-reloadable, and out-of-sample validated: a second adapter is
already scheduled by the spec.

**The two classes of number, and the rule for each — this distinction is what the review round's
last finding forced into the open, and half of that finding was wrong in an instructive way.**

1. **Spec-defaulted strategy params** — the EWMA half-life (*"a strategy param, default ~3 days"*,
   §2.4's words), the decision quantile (*"default p25"*, ditto), the aggregation rule (*"default
   conservative"*, ditto). These have defaults **because the core states them**; they ride §5.2's
   typed params, config-defaultable and hot-reloadable. Refusing them at runtime would be inventing
   a stricter rule than the spec's and calling it safety.
2. **Measured quantities** — the dispersion distribution (C3), the slippage model's numbers, the
   rescue-margin figure inside `C_allocated`, utilization, **the exit band's width** (§2.4: *"the
   band's width from measured regime-persistence (the C3 discipline)"* — `exit_band` is a §5.2
   param slot, but its *value* is measured, so the slot carries the artifact identity too), and
   **the default `BasisForecast`'s measured adverse move** (§2.4: *"charge the measured adverse
   basis move"* — conservative in direction, still a measurement in magnitude). These have **no
   usable default anywhere**: each one's zero value flatters the gate (zero adverse drift, zero
   slippage, free rescue margin, a zero-width band that exits on the first dip's first settlement,
   a zero basis charge), so each is a validated input that **refuses to construct without the
   step 4 artifact's identity** (ID + content hash) attached, and the values committed to the repo
   live **only in named test helpers**. A missing artifact is a refusal that names the field —
   never a forecast that runs on a plausible fallback. **A scored run additionally requires every
   class-2 input to resolve from ONE artifact identity** — mixing two artifacts' numbers in one
   verdict is a version the governance never approved, and it is refused, not warned.

The first draft blurred these ("committed defaults ship only in test fixtures" beside "default ~3
days"), and the review round read the blur as a production fallback for calibrated values. The
class rule dissolves it: the half-life default is the spec's own; the dispersion has none.

**Quantile aggregation follows §2.4's stated joint rule as the default:** one conservative
scenario — every settlement at its per-settlement adverse quantile, funding and basis co-moving
against us — never independent draws; the rule is a named param (`aggregation`) so the artifact can
later justify a different one *in writing*.

**The slippage charge has a port of its own: `ExecutionCostModel`.** §2.4's cost is fees *plus a
measured slippage model*, and §8 conditions that model on order size and book state — so a charge
that arrives as a constant is wrong by construction (a cost independent of size and liquidity),
and the round-3 review caught that the first draft declared the charge as an input with no seam
able to produce it. The port's contract: expected per-leg execution cost for a **candidate
quantity**, given per-leg book state, the execution tactic (§2.4's taker default; the maker-entry
fork is a *documented opt-in*, §12.2a), a latency assumption, and the **calibration identity** its
numbers came from. The adapter sources its own book state (its wiring, not the strategy
application's — book plumbing stays out of the context); milestone 2's adapter is the
**conservative bound** whose structure §8 fixes and whose numbers are the step 4 artifact's
(class 2 — refuses without the artifact identity). Two adapters are already scheduled by the spec
(the `$1K` conservative bound; the `$10K+` full adverse-selection model), so the seam is real.
**The expected charge in a `Decision` is never the simulator's realized fill** — one is the
decision's input, the other is the run's output, and conflating them lets the sim grade its own
homework.

### §40f — the money math: one `Decision` value, complete enough to recompute, with an oracle and a pinned ledger

**Decision: the core's output is a `Decision` value that carries the verdict, every term of the
inequality that produced it, AND the identity of every input it was produced from.** The terms:
the funding quantile over the planned/remaining `HoldPlan`, the basis term, each leg's fees by
side, the slippage charge, the reserve/opportunity term, **the closing-cost delta and the risk
delta as their own lines** (§2.4's exit score subtracts each separately; §40b carries them in
separately, and the decomposition must show where each landed), the scaled hurdle
(`hurdle · C_allocated · holdFraction`), and the band/dwell state for exits. The identities — the
review round's "decidable from the record alone" objection, valid in full: the `MarketView`
watermarks per leg, the `HoldPlan` (its current revision, and which revision), the fee revision
hash, the **lot-rules revision hash**, the hurdle revision hash, the forecast model name + params +
the materials it consumed, the calibration artifact ID + hash (or the explicit fixture-helper
marker in tests), the `C_allocated` decomposition, the strategy-params revision, and the prior
dwell state it was fed. **The completeness rule is stated as a property, not a roster:** the
journaled `Decision` must contain enough to *recompute the verdict* and to answer, from the record
alone, which revision of every input produced it — a Decision that names its conclusion but not
its evidence is the audit-projection mistake (§3.2's outbox argument) one layer up. A **refusal is
a `Decision` too** — stale view, skew breach, unpopulated input, quantization failure — with the
refusing rule named and no Intents; a gate that only journals its approvals cannot show it ever
refused anything.

**The `Decision` has a versioned wire DTO, because the domain value cannot serialize itself.** The
round-2 review read `platform/fixedpoint`: `Money`/`Rate`/`Price`/`Quantity` hold unexported
fields, the codec is expressly a database-column codec, and nothing implements `MarshalJSON` — so
`encoding/json` over the domain `Decision` writes **empty objects for every economic term**, and
the recompute property dies silently at the first journal write. The wire DTO carries each
dimensional value as mantissa + scale + its asset/base/quote tags explicitly, is versioned (a `v`
field, the dispatch log's token-is-the-version-surface pattern), decodes **strictly** (unknown
fields and missing members are refusals, not zeros — a zero here is a flattering term again), and
slice A proves it with **byte-level round-trip tests over every dimensional field, every revision
identity, a refusal, and the dwell state**. The DTO lives beside the domain type in
`strategy/domain`; it is a representation, not a second authority.

**The `HoldPlan` is durable, not frozen — the interval flip is §2.4's own C8 rule arriving here.**
The first draft had exit evaluating "the recorded" plan; the round-2 review put that against §2.4's
*"Bybit can flip an alt's interval 8h↔1h mid-position without notice"* and `marketdata`'s own doc
comment (*a stored-at-entry interval makes the flip invisible; the settlement count silently
octuples*). An 8h→1h flip makes the entry-time schedule omit seven of every eight real settlements
(biasing exit premature); 1h→8h forecasts carry that will never settle (retaining a loser). So:
**the entry `HoldPlan` is immutable evidence, and the *working* plan is a journaled
`HoldPlanRevision`** — on every funding-metadata change that moves the schedule, completed
settlements are retained, future timestamps are regenerated from the live `Interval`/`NextSettle`
**to the unchanged terminal time**, and the revision is journaled with its trigger. `Continue`
consumes the current revision (that is what `DecisionInput` carries); **the dwell counts realized
settlements observed, never entries in the mutable schedule** — a schedule regeneration must not
reset or inflate a dwell. This is recorded as a **stated reading of §2.4** (the terminal time and
the durable-record property are preserved; "recorded at entry" binds the *hold*, not the venue's
schedule inside it), on the close's reading-pass docket beside §40e's — if the pass judges it a
deviation, it becomes a marked edit then.

**How a `Decision` crosses the boundary — the first draft's blocker, and round 2 forced the door
to be NAMED, not gestured at.** The production inbound door is **one application-service method:
`Decide(...) → (Decision, []Intent, error)`** — the step-5 runner consumes *this*, and so does
every future production caller. §3.3's `Strategy` interface stays **verbatim** (no core edit in
this step): it is satisfied by a thin adapter over `Decide` whose sole job is the compile-checked
conformance assertion (`var _ Strategy = …`) — **the errorless adapter is NOT a production path,
in this milestone or any other.** The round-3 review is right that it cannot be one: `Evaluate`
has no error channel, so a fatal journal failure inside it could only be swallowed, panicked, or
side-channeled — and for an open position, silently returning zero Intents can suppress a
required close while the engine keeps running without journal evidence. **How the milestone-3
foundation loop consumes the error-bearing door — a §3.3 edit widening the interface, or a
declared synchronous fatal channel the loop must observe before the next event — is milestone 3's
design decision, named here as a debt whose construct is the foundation loop itself** (the
deferral rule: name the construct the decision reads, and the loop is what makes it decidable).
What this step guarantees is that the error-bearing door *exists and is the only consumed one*. The semantics are write-ahead: **`Decide`
writes the `Decision` to the `DecisionSink` port before returning any Intent derived from it; a
failed sink write returns zero Intents AND a non-nil error, and the runner treats that error as
run-aborting** — a paper run that silently stopped journaling is a scored sample with missing
evidence, which is worse than a stopped run. **Pre-core refusals are the service's to construct:**
an assembly or validation failure (a port that errored, an unpopulated input, a stale view) never
reaches the core — the service builds the refusal `Decision` naming the refusing rule, sinks it,
and returns no Intents. No Intent without its journaled Decision — the record of *why* must not
depend on the action's success. Test adapters are in-memory; milestone 2's real adapter is §40h's
container writer, owned by the step 5 runner.
**Rejected: journaling as a side effect inside the core** (defeats purity and makes the journal a
thing that happens rather than a thing that is checked); **rejected: the runner reconstructing the
Decision from the Intent** (an Intent is a projection of a Decision; reconstructing the cause from
its effect is exactly backwards and silently loses the refusals); **rejected: widening §3.3 to
return the Decision** (couples every future strategy to the journal, and the foundation loop has
no use for the audit value — the sink is the audit path, the interface is the trading path).

Three rules of §2.4 restated here because each is a silent-inversion hazard the battery must aim
at:

1. **Entry:** `Q_p[Σ funding + basis − round-trip − slippage] − reserve ≥ hurdle · C_allocated ·
   holdFraction`, every term **`Money` over the `HoldPlan`** — one unit, dimensional, no rate ever
   multiplied by hours (`FundingObservation.Enumerate` is the only settlement source).
2. **Exit is incremental, never a re-run of entry:** sunk entry costs excluded; closing cost
   charged only as `Δ(now vs later)`; the same quantile + joint rule over the **remaining**
   `HoldPlan`; stand-down only after the score stays below the exit band for `exit_persist`
   settlements, the dwell fed back through `DecisionInput`. The two wrong shapes — re-subtracting
   the round trip, charging the close to continue alone — both bias toward premature exit, both
   compute cleanly, and both get an aimed mutation.
3. **The denominator is `C_allocated`, never `C_legs`** (§2.2). The guide already pins the wrong
   denominator's flattering 7.12% as a *negative* fixture; the Go core inherits that exact trap as
   a test.

**The oracle discipline applies, and the oracle already exists — in another language.** The guide
calculator (`docs/guide`) computes the same ledger in JS, pinned to tutorial §3.9's five figures
($90 funding, $38 round trip, 8.4 days to break even, 5.68% in-position, 3.41% at 60% utilization
vs the 4.60% hurdle). **The cashcarry core must reproduce tutorial §3.9's ledger exactly, as a
test.** That makes three independent implementations of the flagship's kill-math (tutorial prose,
guide JS, Go core) that must agree to the digit — and per the guide's own rule, if the assertion
ever fails, the spec and the code disagree: **that is a finding, never a test to relax.** For the
quantile/EWMA arithmetic the §3.9 ledger does not reach, the slice adds a `math/big` oracle in the
`fixedpoint` tradition — real defects in this repo have been found by two implementations
disagreeing, not by review.

### §40g — sizing and quantization: the intersection rule, structure-only

`Intent.TargetQty` MUST be lot-quantized to the **intersection of both legs' venue lot sizes** with
an explicit residual-delta tolerance, leftover delta beyond tolerance a sizing *failure* (§2.4,
[→ X7]) — and a journaled refusal, per §40f. The quantization is a pure domain function whose lot
rules arrive **in `DecisionInput`, resolved by the application from `ReferenceData`'s lot face**
(§40b — as-of, revision-hashed, both legs resolved before evaluation); the lot filters are venue
reference facts and ride **the same dated-revision pattern as fees** (§40d) — Profiles already
holds the dated Bybit instruments-info measurement from v0.9.16, and slice A's verification pass
extends the table to the four `(venue, product)` cells with sources.
`C_allocated`'s composition (spot principal + perp IM + rescue margin + reserves + float, §2.2) is
a validated input struct; its rescue-margin term is a **class-2 measured quantity** (§40e) —
artifact-identified, never defaulted — because §7.1's protocol that *computes* it live is
milestone 4, and the gate needs the number as a declared, provenance-carrying input, not the
machinery.

### §40h — §39l-2 answered: paper decisions are journaled in the capture container, as Decisions

**Decision: the paper-decision journal is capture-lane framing over `engine/capture`'s codec —
`Data` records with `k`-discriminated JSON payloads (`k:"decision"`), one stream per run, written
by the milestone-2 `DecisionSink` adapter the step 5 runner owns; the payload is the §40f wire
DTO.** Step 3's obligation is the `Decision` being journal-complete (§40f's recompute property)
and the sink port existing with its write-ahead semantics; step 5 wires the container adapter.

**The sink's success is a durability claim, so the adapter must earn it — the container's default
write path does not.** The round-2 review read `engine/capture`'s writer: the `Data` path appends
and returns, with fsync arriving on a periodic tick or at close — right for eight market streams
at market rate, and **wrong for a write-ahead journal**, because torn-tail recovery cannot recover
bytes never made durable, so "write-ahead" would quietly mean "write-eventually". The rule:
**`DecisionSink.Write` returns success only after the decision frame is flushed and fsynced.** A
decision stream runs at decision rate — one frame per evaluation — so per-frame fsync costs
nothing that matters; the periodic tick stays what it is, the market streams' amortization. The
step 5 adapter uses a synchronous-sync door on the writer (or a decision-specific writer), never
the tick.

Rejected: **waiting for the OMS journal** (milestone 3 — but §12.2 puts "paper decisions journaled"
in *this* milestone, and the kill-gate run must be auditable the day it is scored); **plain
NDJSON** (discards the torn-tail recovery, integrity split, and verify machinery the container
already paid for — and slice B's dispatch log already proved the container carries a non-capture
payload cleanly, §39n-1); **a new `RecordKind`** (rejected for the dispatch log already: a
`FormatMinor` bump orphans the live v1.0 production archive — the same ruling, same reason, not
reopened).

### §40i — §39l-3 answered: the sim pump only; paper trading is replay-driven

**Decision: milestone 2 exercises the sim pump only. The live pump stays uncalled. "Paper
decisions" are produced by replaying each accumulated capture day through the sim engine —
replay-driven paper — not by a live read-path.**

The ground: the kill-gate verdict is scored over ≥3 months of history (§8) through the replayed
engine, so the live path contributes **nothing to the verdict** — a live paper runner would be new
transport surface (reconnect policy, live freshness under wall clock, §39q-5's live-transport
predicate) exercised for output the gate cannot use. Replay-driven paper gives §12.2 its journaled
decisions from the first accumulated day, through the *same* engine the verdict runs on — which is
§3.5's parity doing the work it exists for. The live pump's real trigger is milestone 3 (the OMS
needs a live path to trade against), and §39q-5's note — the sim's ordinary predicate is defensive
until the live transport exists — **stays open, correctly, until then**. Rejected: a milestone-2
live paper path "to shake out the transport early" — it shakes out transport the milestone does not
ship, on the machine whose only job (§38f) is to keep the capture honest.

### §40j — build order: three slices, the standing rhythm

Each slice closes under the full discipline: fixture-collapse checklist **before** mutations, aimed
battery with all six verdicts read, reading pass beside the marker grep, spec edits landed before
close.

- **Slice A — the context skeleton and the value types.** `strategy/domain` (+ `Validate()` on
  every type — zero value unusable throughout), `DecisionInput` (policy projection, Δ terms and
  the absent-vs-declared-zero rule included)/`Decision` **+ the versioned wire DTO with its
  byte-level round-trip battery** (§40f), the `Strategy` interface + the compile-checked adapter
  over `Decide`, `application/ports` (all seven ports — §40b's roster, the two read ports split
  per §40c, the `SettlementHistory` accumulator's ownership declared), the Profiles verification
  pass (fee tables + lot filters, dated sources), and the fee/lot/hurdle revision schema — **types
  declared in `strategy/infrastructure/driven` beside their adapter, riding `platform/config`'s
  load/validate/revision machinery**: the kernel owns the mechanism and never a context's schema,
  which is §11's dependency direction and the round-4 review's recovered catch — with the
  provenance-refusal rule and the `asOf` port shape. The
  `[SPEC EDIT OWED]` marker for §40d's two-clause §2.4 edit is taken here.
- **Slice B — the models.** The default funding estimator (EWMA + clamp + imminent-settlement
  rule) on the explicit-materials signature, **the default conservative `BasisForecast` adapter**
  (zero favorable convergence; the measured adverse move as a class-2 artifact-identified input —
  round 2 caught that no slice owned it), **the milestone-2 `ExecutionCostModel` conservative-
  bound adapter's structure** (round 3 caught that the charge had no seam; numbers are the
  artifact's, and the adapter refuses without its identity), `Dispersion`/quantile machinery with
  the class-2 artifact-identity refusal, the joint aggregation rule, the `math/big` oracle for the
  quantile/EWMA arithmetic. The checklist's standing suspects, named now: an EWMA
  window pinned to one settlement (where "EWMA" and "last value" collapse); a cap/floor fixture
  whose clamp never binds; a quantile fixture symmetric enough that p25 and p75 agree; a
  dispersion fixture with zero width (where "quantile" and "mean" collapse).
- **Slice C — the cashcarry core.** Entry inequality, exit incremental score with the fed-back
  dwell, band + dwell **on realized settlements**, the `HoldPlanRevision` regeneration rule with
  an interval-flip fixture in BOTH directions (8h→1h omits settlements; 1h→8h invents them — the
  two failures bias opposite ways, so one fixture cannot witness both), quantization over
  port-resolved lot rules, the write-ahead `Decide` semantics in the application service (a staged
  sink failure must be shown to suppress the Intents AND surface the error — and not on the
  collapsing last-slot fixture, §39q-2's own lesson), the tutorial §3.9 pinned-ledger test, the
  §12.2 worked-example forks' figures as fixtures, refusal Decisions for stale/skew/unpopulated/
  unquantizable inputs. Close lands the §2.4 edit, runs the reading pass over every normative
  claim the slices asserted — §40e's signature reading and §40f's `HoldPlan` reading explicitly on
  its docket — bumps the doc set.

### §40k — open questions passed on, sharpened not pre-answered

1. **§39l-1 unchanged** — the ≥3-month historical window's data source is step 4/5's decision.
   Sharpened by this record only in that the revision mechanism now exists for whatever that data
   turns out to be: a purchased-history run needs *period-correct* fee revisions, and the
   repository below is where they go.
2. **The append-only reference-revision repository + replay applicator — step 5, pre-registered
   here** (§40d): exact bytes + content hash + source + observed-at + effective interval,
   applied in deterministic event order (or as-of lookup against replay time) during a scored run.
   The trigger construct is named per the deferral rule: **the step 5 runner is its first
   consumer**; the port's `asOf` parameter and the `Decision`'s revision hashes are the seam it
   arrives behind. A scored run over a window with no period-correct revision for some input is a
   **refusal naming the input**, never a silent apply-current.
3. **Which venue hosts the scored gate** ("one venue / one symbol", §12.2) — step 5's call, with
   both venues' capture and both venues' fee rows ready either way.
4. **`C_allocated`'s rescue-margin figure** — a declared class-2 input this step; its derivation is
   §7.1's milestone-4 protocol; the step 4 artifact freezes the interim number with rationale.
5. **The foundation loop's error-bearing door — milestone 3, pre-registered here** (§40f): how the
   live loop consumes `Decide`'s error (a §3.3 edit widening the interface, or a declared
   synchronous fatal channel observed before the next event). The construct that makes it
   decidable is the foundation loop itself; until it exists, the errorless §3.3 adapter is a
   compile-check and nothing more.

---

## RESUME HERE

**Step 3 is DESIGNED (session 45, 2026-07-31) — nothing is built.** §40 above is the design; §40j
is the build order. It survived four adversarial review rounds (three complete, each initially
NO-SHIP; round 4 killed mid-run with its two formed findings recovered and applied; all valid
findings applied and the loop deliberately closed — provenance note at top). §39l-2 and §39l-3 are ANSWERED here (§40h:
capture-container Decision journal via the `DecisionSink` port, fsync-on-write; §40i: sim pump
only, replay-driven paper) — do not reopen them; §39l-1 passes to step 4/5, and two new debts are
pre-registered with their constructs (§40k-2 the revision repository — step 5; §40k-5 the
foundation loop's error door — milestone 3).

**Next, in order:**
1. **Slice D's one-day disk figure, ~09:53 UTC 2026-07-31** — `du -sh archive/` + a full-day
   verify, recorded in the capture design record (§32m's row). This is a calendar item and lands
   whenever its boundary passes, independent of step 3's slices.
2. **The off-host destination** — still the user's open §38f decision; the laptop still holds the
   ONLY copy.
3. **Step 3 slice A** (§40j): context skeleton, `DecisionInput`/`Decision` + the wire DTO with
   its round-trip battery, all seven ports, the Profiles fee/lot verification pass, the
   context-owned revision schema with `asOf` + provenance refusal, the §40d `[SPEC EDIT OWED]`
   marker (two clauses).

Standing rituals first, every session: `pgrep -fl "arblab capture run"` (PID 74192 as of session
45), `capture verify` over the accumulated range (0 integrity findings, always), `git status
--short` (clean), marker grep (**ZERO** until slice A deliberately takes §40d's marker).

Do not reopen from this record: §40b's placement and the one-`DecisionInput` shape (policy
projection and Δ terms included), §40c's split read seam and the dispatched-stream accumulator,
§40d's fee composition (fetch-writes-a-revision; decision path reads as-of; provenance refusal;
the modeled-tier authorization with its account-real-before-capital condition), §40e's two-class
number rule and the `ExecutionCostModel` seam, §40f's `Decide` door, `DecisionSink` write-ahead,
the recompute property and the `HoldPlanRevision` rule, §40h's fsync-on-write, §40i — each carried
with its reasoning above.
