# Record §87 — M4 task #224: §7.1's terminal rung is a FUNDING step, not a headroom step — and six adversarial rounds turned one false parenthetical into five prerequisites

**Date:** 2026-08-09 · **Task:** #224 · **Design record, no code** · Branch
`feat/milestone-1-foundation`.

Record §85 §3 predicted this: *"margin released: no `q*` exists; the rung must be
re-designed."* Task #221 measured the bad branch. This record is the redesign.

**The one-line finding:** a `PackageDelever` does not raise liquidation headroom,
but it does **release the exact margin the failed top-up was short of** — so the
rung's purpose inverts. It is not a bigger hammer than the top-up; it is what
makes the top-up's precondition true, and the ladder re-enters at the top-up
rather than escalating past it.

**How this record was produced, because it bears on how much to trust it.** Six
adversarial review rounds (a second model family, `codex-companion
artifact-review`), each returning NO-SHIP. The arithmetic converged at round 3
and has not moved since; rounds 4–6 found only **missing cases in a prose state
machine**, one instance per round — `ClassInternal` unrouted, a stranded
premise-lost supersession, `==` where `>=` belongs, a lock where a reservation
belongs. That is the class `docs/formal/`'s two models exist to find, and §5e
already makes model revision a blocking prerequisite, so review was stopped
there deliberately rather than run to a seventh round. **Treat §3's episode
machine as specified-but-unmodelled.**

§6 lists findings this record rejects and why; §5 lists the prerequisites it
**opens rather than assumes**.

## 1. The defect

§7.1's escalation reads: if the margin top-up "fails or misses its budget
(rate-limit, venue lag, insufficient staged margin), the ladder escalates to a
`PackageDelever` … toward reduced notional (**raising liq headroom**)".

The parenthetical is false at the sizes this system trades. Measured under task
#221 on both venues: a partial reduce-only close on an isolated position releases
posted margin **pro rata**, and the liquidation price did not move by a published
decimal (Binance `isolatedWallet` 6.43067264 → 3.21533632, liq 67753.52223108
unchanged; Bybit `positionIM` 13.0395549 → 6.51977745, liq 70931.6 unchanged).

### 1.1 Why — and the bracket qualifier the measurement could not see

Maintenance margin is **one continuous, convex, piecewise-linear function of
notional** with `MM(0) = 0`, not a per-tier lookup: inside bracket `i`,
`MM(N) = N·MMR_i − cum_i`, and `cum` exists to keep `MM` continuous across
boundaries. The isolated-short liquidation condition is

    m + (e − L)·q = MM(q·L)

Under a pro-rata reduction (`m' = αm`, `q' = αq`, α ∈ (0,1)) the left side scales
exactly by α while the right becomes `MM(αqL)`. Convexity through the origin
gives `MM(αN) ≤ α·MM(N)`, **strictly whenever `cum_i > 0`**. So the reduced
position is weakly over-margined at the old `L`, and `L` moves weakly up — weakly
MORE headroom for a short:

    L exactly unchanged  ⟺  cum_i = 0  ⟺  bracket 1

**#221 measured a 0.002 BTC position (≈ $130 notional) — bracket 1 on both
venues.** So the inertness is exact where it was measured, and §7.2's small-size
posture says bracket 1 is where this system trades.

**Normative statement:** the delever raises **no** liquidation headroom in
bracket 1 and **weakly** raises it above bracket 1. Any claim omitting the
bracket qualifier is falsified the first time this system trades at size, and
falsified in the safe direction — the kind nobody notices.

The reasoning that makes "reduced notional ⇒ more headroom" unconditionally true
is **cross** margin semantics. §4.1 pins **Isolated**.

### 1.2 The bracket ceilings differ by venue by ~40×, measured

`GET /v5/market/risk-limit?category=linear&symbol=BTCUSDT` (public, called
2026-08-09):

| tier | `riskLimitValue` | `maintenanceMargin` | `mmDeduction` |
|---|---|---|---|
| 1 | 2000000 | 0.005 | `""` |
| 2 | 2600000 | 0.0056 | 1200 |

Two facts follow. **Bybit's bracket 1 runs to 2,000,000 USDT notional**, so for
any size this system will hold, the delever is inert on Bybit and the cross-tier
solve below is **Binance-only in practice**. And **`mmDeduction` is the empty
string in tier 1, not `"0"`** — the same absence spelling as Bybit's `liqPrice`
outside the price band. A decoder that refuses `""` refuses the only tier this
system trades in; reading it as zero is correct, and correct for a reason that
must be written down rather than relied on.

Binance's bracket-1 ceiling is **unverified** (the authenticated
`leverageBracket` call, task #220) and is the one input this section's normative
claim depends on. It is in the measurement plan (§7).

## 2. Notation

    q_now     current |position quantity|      (venue.MarginStatus.SizingQuantity)
    q_final   REMAINING position after the reduction
    r         q_now − q_final   ← the PackageDelever generation's target
    α         q_final / q_now
    L*        target liquidation price (task #220's open policy question;
              every formula here is parametric in it)
    D_i       (1 + MMR_i)·L* − e     for bracket i
    A         venue-reported AVAILABLE balance (see R8 — a prerequisite, not a
              derived quantity)

## 3. The redesign

### R1. The delever is a FUNDING rung, not a headroom rung

Solving the bracket-`i` relation for the remaining quantity that puts `L` at
`L*` when the position ends up holding margin `m` again:

    m_required,i = q_final·D_i − cum_i
    q_final,i    = (m + cum_i) / D_i          (setting m_required = m)

and the top-up the reduced position then needs is

    Δ = m_required − α·m = (m + cum_i) − cum_i − α·m = (1 − α)·m

**exactly the released posted margin, in every bracket.** (Round 2 blocker 1:
the previous round omitted `−cum_i` from `m_required` and therefore
over-requested by `cum_i` above bracket 1 — which fails for insufficient funds,
which R5 then reads as a non-funding second failure and terminal-flattens a
position whose correctly sized re-post was affordable.)

So the delever converts *"insufficient staged margin"* into *"sufficient staged
margin"*. **Normative consequence:** after a completed delever generation the
ladder **re-enters at the top-up rung**. Rung 3 exists to make rung 2's
precondition true. This inverts §7.1's current one-way escalation ordering and is
the change that most affects slice B4 (#222).

### R1a. Released MARGIN is not returned CASH, and the difference is adverse here

`PostedMargin` excludes unrealized PnL by construction
(`marginstatus.go:190-208`). A reduce-only close of `r` at fill price `P` on a
short **realizes** `r·(e − P)` and charges a closing fee, so the cash reaching
available balance is

    ΔA = (1 − α)·m + r·(e − P) − fees

and under the production condition this rung exists for — an adverse rally,
`P > e` — the realized term is **negative**. The re-post of `Δ = (1−α)m` is
therefore **not** self-funded; it is short by `r·(P − e) + fees`.

Three consequences, and the third is the one that changes the slice:

1. **The rung still works, directionally.** At the rebalance band the position is
   not liquidated, so equity `m − U > 0` where `U` is the unrealized loss, hence
   `m > U` and re-posting the actual cash still raises `m/q`. It lands short of
   `L*` by the realized term, and R7's re-derivation absorbs the shortfall.
2. **"Exactly" leaves the design.** The self-funding property is *approximate and
   adverse-biased*, which is the honest statement.
3. **The available-balance read is a PREREQUISITE, not an optimization** — R8's
   deferral is refuted, because the amount that came back is not the amount that
   left.

**The re-post amount is defined from LIVE post-close state, and `Δ = (1−α)m` is
not it.** `Δ` is the released *posted margin*, which the realized term makes
different from both the cash on hand and the margin still owed. The normative
form is

    add = clamp( q_live·D_i − cum_i − m_live ,  0 ,  A_live )

read after the generation reaches finality, followed by verification against the
venue's own published liquidation price. Nothing sized before the close survives
it.

**The solve is therefore a FEASIBILITY INEQUALITY over candidate reductions, not
an equation.** With `m₀, q₀, A₀` the pre-close state, `r = q₀ − q_final`, `P` the
fill price and `f` the closing fee, the maximum margin the position can hold
after the re-post is `m₀ + A₀ + r·(e − P) − f`, so a candidate `q_final` is
feasible in bracket `i` iff

    q_final·D_i − cum_i  ≤  m₀ + A₀ + (q₀ − q_final)·(e − P) − f(r, P)

**`A` is observed BEFORE the close (`A₀`), and `P` and `f` enter as conservative
bounds** — a slippage-adverse fill price and an upper fee bound. Reading `A`
after the close instead would make the solve circular: the close has by then
already fixed `q_final`. The post-close live read of `m, q, A` is used only to
**size the re-post and verify the target**, never to retroactively define the
generation's target.

**#221's measurement cannot distinguish these two quantities.** It recorded
posted margin halving and that released margin "returns to available balance" —
but at a mark near entry the realized term is ≈ 0, so *margin released* and
*cash received* are the same number. The fixture was pinned to the value that
collapses the distinction. §7's measurement plan requires a **materially
adverse** close for this reason.

### R1b. The sizing is a PIECEWISE solve, per venue, verified against the venue

> **STEP 2's ACCEPTANCE CRITERION IS SUPERSEDED — read record §90 before
> implementing this `[v0.9.69, task #233, measured under #230, record §89 F3]`.**
> The enumeration below stands. "Classify `q_final` into its **actual** bracket"
> reads as *the bracket containing `q_final·e`*, and the venue does not do that:
> the bracket is the one containing the **liquidation** notional, which makes
> step 5 a **fixed point** (bracket `i` whose own `L` puts `q·L` back inside `i`).
> Step 2 also conflates two selections on two bases — step 4's feasibility test
> classifies at the *target* notional `q·L*`, step 5 at the *achieved* `q·L`.
> The open ambiguity this ruling names below is therefore **closed for Binance**
> and still ruled-around for Bybit.

`MMR` and `cum` are functions of the notional being solved for. `MM` is
continuous and strictly increasing, so the solution is unique and is found by
enumeration over the brackets task #220's read supplies:

**This is the single authoritative algorithm. The closed form `q_final,i =
(m + cum_i)/D_i` is a DERIVATION, not a step** — it is what the inequality
collapses to when `A₀ = 0`, `P = e` and `f = 0`, and using it as the selection
rule picks candidates the cash cannot fund (worked: `m₀=10, q₀=1, e=100, L*=120,
MMR=0.01, cum=0, P=105, f=0` → the closed form selects `q_final ≈ 0.472`
requiring margin 10 against 7.36 available; the inequality gives `q ≤ 0.309`).

**The objective is the GREATEST retained position that reaches `L*`** — the
position is the carry trade, so giving up more of it than the safety target
requires is a cost, not a margin of safety. The candidate domain is therefore
`0 < r < q₀`, enumerated **`r` ascending** (equivalently `q_final` descending),
and the first candidate that passes every test wins.

1. Enumerate on-grid reductions `r` (R4) with `0 < r < q₀`, **ascending**;
   `q_final = q₀ − r`, so `q_final > 0` always.
2. Classify `q_final` into its **actual** bracket `i` at the venue's risk basis
   (§5g), boundary inclusion from the venue's published bounds.
3. Reject the candidate unless `D_i > 0`.
4. Reject the candidate unless it satisfies R1a's cash-feasibility inequality,
   with the conservative `P` and `f` bounds.
5. Reconstruct `L` with the **venue's own formula** for that bracket; accept the
   first candidate reaching `L ≥ L*`.
6. **No candidate with a positive remainder passes ⇒ terminal (R6).** `r = q₀` is
   deliberately outside the domain: at `q_final = 0` bracket classification and
   liquidation-price reconstruction are undefined, and a full close is
   `EmergencyClosePackage`'s act, not a delever target.

**Two venue-specific inputs are NOT assumed to be shared, and both are open:**

- **Binance** `leverageBracket` supplies bracket floors/caps, MMR and `cum`. A
  user-specific `notionalCoef` is **not present** on the symbol-scoped response
  under §4.1's `multiAssetsMargin == false` posture (measured, §7), so it is
  recorded as absent-under-this-posture and **not** retained in the solve. If a
  deployment ever leaves that posture, the field returns and the solve must be
  re-derived — which is a reason the posture assertion is a refuse-to-trade gate.
- **Bybit's isolated liquidation formula is NOT assumed equal to Binance's.**
  Published material describes leverage, fee-to-close and extra-margin terms that
  the common relation above does not carry. §85 verified the relation to eight
  decimals on **Binance only**; #221's Bybit agreement establishes pro-rata
  invariance, which follows from the *shape* of any such formula and does not
  validate the coefficients. Task (§5c).
- **The tier basis is not yet stated precisely enough to implement**: whether
  bracket membership is evaluated at the current mark, at `L*`, or at another
  venue-specific basis. Named as an ambiguity, resolved by measurement (§7), not
  guessed.

### R1c. The generation's target is the REDUCTION, not the remainder

`Generation.target` bounds cumulative executed reduction (`generation.go:487-503`
refuses `perpDone > target`), so `Deleverer.Open` takes `r = q_now − q_final`.

**Record §85's line describing that argument as "an absolute, strictly positive
target position" is WRONG and is corrected by this slice.** Written as stated, a
`q_final` of 0.43 would be passed where 0.57 was meant — an under-delever on the
safety kernel's escalation.

### R2. A crash between the two steps is weakly-not-worse (safety), derived

`L` unchanged (bracket 1) or weakly higher (above it, §1.1), with `q` smaller:
the same or better distance to liquidation, with a smaller loss cap. The re-post
needs no durable **safety** obligation. What a crash destroys is the attempt
accounting, which is R3.

### R3. The remedy EPISODE is durable, with a write-before-act protocol

Round 1 and round 2 blockers. Without durable accounting a driver can delever
repeatedly, each generation releasing margin nothing re-posts, grinding the
position toward zero. **§7.1 already names this exact failure shape** in its
unstated-`liqPrice` bullet: *"it would de-risk every well-margined Bybit position
to zero, one generation at a time, with the safety system appearing to work
throughout."*

A durable **remedy episode** is journaled per hedge group on §5.1's one chain.
This field list is authoritative; nothing later in this document adds to it:

| Field | Holds |
|---|---|
| `EpisodeID` | identity |
| `OpenedBy` | the failure class that opened it (R5) |
| `ActsUsed` | every counted act — top-ups, delever generations, re-posts, zero-fill completions |
| `MaxEpisodeActs` | **snapshotted into the episode at open**, not read from config at the test |
| `GenerationID` | the delever generation, when one is open |
| `AuthorizedOpID` | the `MarginOpID` of the currently authorized margin call, whichever call it is |
| `Phase` | one of the members below |

`Phase ∈ {TopUpAuthorized, TopUpAttempted, DeleverOpen,
AwaitingPremiseRestoration, AwaitingRePost, RePostAuthorized, RePostAttempted,
TargetVerified, Resolved, Terminal}`.

**The bound is snapshotted because §7.4 permits operators to edit escalation
parameters.** An episode recovered with `ActsUsed = 4` whose configured maximum
moved 5 → 3 would test `4 == 3`, false, and every later increment moves *further*
from equality — a budget that can never exhaust, in a record whose whole purpose
is to exhaust. So the test is **`ActsUsed >= MaxEpisodeActs`** against the
snapshot, and a durable record's termination condition lives inside it.

**Crash boundary.** The dispatcher mints the `MarginOpID` internally, fsyncs its
own `MarginOp` and only then returns (`margindispatch.go:326`), so an episode
that records an attempt *after* the call can lose a landed one and re-issue it.
Therefore **every** margin call in the episode — including the **first top-up,
whose failure opens the episode** — is preceded by a journaled authorization
bound to exactly one `MarginOpID`, and recovery must resolve or live-verify that
op before any further margin call.

Binding only the *re-post* leaves the boundary uncovered at the one place the
episode begins: a crash after a durable `NotApplied` verdict but before the
caller creates the episode leaves no episode to reinstate, and repeating that
boundary buys unlimited "first" attempts. So the episode is **opened and its
budget incremented before the initial top-up is authorized**, not on its failure.

This requires the dispatcher to accept a **caller-supplied** op id, or to publish
the minted id durably before the call. #215 ruling (v) deliberately made the
dispatcher the minter — for a reason that still holds (a re-used id files two
dispatch records under one subject) — so the change is *who supplies a unique
id*, not *whether uniqueness is enforced. Named here as a §222 design point.

**Every way a generation can END needs an episode transition, not only
`Complete`.** SM §7 ends a generation four ways, and they do not agree on whether
the ladder resumes, so an episode keyed only on `Complete` strands itself on
three of them:

| Generation ending | Ladder | Episode |
|---|---|---|
| reaches `Complete` | continues | → `AwaitingRePost` (R7) |
| **premise-lost supersession** — the group's residual and classification, net of this generation's own legs, leave the state it opened under | **resumes** | → `AwaitingPremiseRestoration`. When §2 restores a neutral package, re-read live risk and re-enter the budgeted decision. Without this phase the episode never re-posts, so the "post-re-post failure" condition for a next generation can never arise, the episode never ends, and recovery faithfully reinstates the stranded phase — blocking every later remedy on that hedge group |
| `EmergencyClosePackage` issued | **does NOT resume** | → `Terminal`, on perp-flat evidence |
| `FenceReadyLost` arbitration | **does NOT resume** | → `Terminal`; the arbitration owns the outcome |

The premise-lost row is the one worth stating in full, because it is the only
generation ending that is neither a success nor an escalation — the attempt
stopped being *valid*, not being *possible*, and the position is exactly as
exposed as before.

**Endings are as normative as failures.** `TargetVerified` is an observation, not
an ending: the episode ends at **`Resolved`**, journaled, which is what permits a
fresh episode on that hedge group. Without it, recovery either retains the
episode forever — blocking every later remedy on that group — or ends it
silently, which is the un-auditable direction §7.3 forbids for control actions.
The reinstatement rule is therefore explicit: `Resolved` and `Terminal` are
**ended** (the latter still requiring §7.5's clearance act below); every other
phase is **reinstated** and blocks a fresh episode.

**One authorization record per counted act**, each naming its own
`MarginOpID` or `GenerationID`. `ActsUsed` is incremented durably *after* the
exhaustion test and *before* the act, so the sequence is
`test → increment → authorize → dispatch` and no crash instant buys a free
attempt.

**Posture (round 2 blocker 5, corrected).** `minPostureFor(ActReduce) =
PostureStopOpening` (`posture.go:172`), so a *reconcile-only* process performs no
safety writes at all and §7.1 task #215 ruling (iv)'s premise **stands**. §7.5's
"not safety actions, which read live state" governs the `Ambiguous` **latch**,
which sits at stop-opening and does permit `ActReduce`. The episode's boot rule
is therefore an **episode-reinstatement gate at stop-opening**, not a
reconcile-only one.

**The `Terminal` phase needs a clearance act, by §7.5's own rule:** *"an outcome
that latches across processes is only completable if some act in the system can
produce the evidence that clears it, and the act must land with or before the
first thing that can produce the outcome."* The clearance is the operator
reconciliation record §7.1 already has (task #213), extended to name an episode;
it must land in the same slice as the episode.

**The budget is a durable NUMBER, and it counts successes too.** R1a predicts a
re-post that **applies and still lands short of `L*`**, so a bound phrased over
consecutive *failures* does not bound the loop: an episode could delever, re-post
successfully, miss the target, delever again, and grind the package toward zero
through entirely successful external calls, each step durable and correctly
recovered. `AppliedButTargetMissed` is therefore a **counted outcome**, the
budget spans **delever generations and every re-post dispatch**, it is
**incremented before each external act** (so a crash cannot buy a free attempt),
and **exhaustion routes unconditionally to `EmergencyClosePackage`**.

**The bound is a named, configured number — `max_episode_acts` — and the episode
carries `acts_used` covering every counted category**, not a top-up attempt
count. The check is `if acts_used == max → EmergencyClosePackage`, evaluated
*before* the durable increment and authorization, so the last act is spent on the
terminal close rather than on discovering the budget is gone. It is configured
alongside §5.2's other liq-distance policy, because it is a statement about how
many attempts a breach is worth and that is the same kind of judgement.

**A zero-fill completion is a counted act.** SM §7 admits a generation reaching
`Complete` with **nothing executed** — `SpotReduceAuthorized → Complete` is "the
zero-size authorization: a perp reduction that confirmed nothing authorizes no
spot sale" (`generationphase.go:321`). Uncounted, that is a free iteration that
changes no state and re-presents the identical situation: the loop that looks
most like progress and makes none. Removing
SM §7's `Complete → Planned` (R7) without this merely relocates the loop behind
an episode transition.

*Rejected alternative:* re-derive consecutiveness from the existing MarginOp and
generation journals. Derivable, but §7.1 task #212 ruling (iv) settled this shape
in the other direction ("the verdict is JOURNALED, never re-derived").

### R4. On-grid enumeration is over the REDUCTION `r`

Flooring `q_final` does not put `r` on the grid: with `q_now = 0.0025`, step
`0.001`, flooring `q_final` to `0.001` leaves `r = 0.0015`, off-grid — and SM §7
records that an off-grid target admits no conforming leg (task #206). So the
enumeration is over on-grid `r`, with `q_final = q_now − r` derived and **not
required to be a grid multiple**. Larger `r` overshoots `L*` in the safe
direction, so ties resolve upward in `r`.

(Bybit BTCUSDT `qtyStep` 0.001 / `minOrderQty` 0.001, measured §1.2.)

### R5. The branch key is a TOTAL (Class, Effect) matrix

`MarginOpNotApplied` is the projection of the `apperror` classes that fix
`EffectNotApplied`. That projection is correct for the resolution question ("did
the margin move?") and destroys the axis this rung needs ("can I call again?").
The class is retained on the verdict and routed on. Every cell is named; no
default.

**`apperror` declares SEVEN classes** — `ClassTransport`, `ClassRateLimit`,
`ClassAuth`, `ClassVenueReject`, `ClassNotFound`, `ClassPrecondition`,
`ClassInternal` (`class.go:26-69`). An earlier draft of this design said six and
built a matrix claimed to be *total* on that count, which is worse than an
ordinary omission: claiming exhaustiveness suppresses the default branch that
would otherwise have absorbed the missing member, so the gap surfaces as an
unroutable incident rather than a degraded one. The table below is checked
against the const block, and B4 owes a product-coverage test over every declared
(class, effect) pair rather than over the rows a fixture happened to reach.

| Class | Effect | Rung |
|---|---|---|
| `ClassVenueReject`, verified insufficient-margin code | NotApplied | delever to `q_final`, re-enter at top-up |
| `ClassVenueReject`, any other / undistinguished | NotApplied | **terminal.** The earlier "a delever is a prefix of the flatten" argument is WITHDRAWN: a `PackageDelever` sequences partial-perp → finality → **spot sale** → finality (SM §7), while a terminal flatten closes the perp to zero *before* touching spot. The spot leg is time spent not reducing liquidation risk, so in a liq-distance incident the extra latency **is** a safety loss |
| `ClassRateLimit` | NotApplied | hold + retry **inside §7.1's own top-up latency budget**; terminal on that budget's expiry. **NOT `T_blind`**, which begins only when both WS and REST *mark* sources are stale (§7.1) and has no bearing on a margin write with a fresh mark |
| `ClassAuth`, any | NotApplied | **the independent actuator, and the split is NOT taken.** The two sub-cases would route differently — an endpoint-permission failure leaves the order path intact, a dead or fenced signer does not — but **no discriminator exists**: `apperror` deliberately collapses bad signature, expired key, bad IP and endpoint-not-permitted into one class (`class.go:35`) and the carrier keeps only class, verbatim code and free text (`apperror.go:57`); and a successful authenticated *read* does not prove write permission, while testing write permission with an order is itself a state-changing safety act. So every auth result routes to §7.4's external actuator, which writes on **its own credential with an independent key/IP** (SM §4's takeover protocol), plus a §9 incident. A venue-verified `AuthReason` and a non-mutating readiness surface would permit the split — §5f |
| `ClassNotFound` | NotApplied | **read the position live first — the two readings are opposite evidence.** `NotFound` is not terminal evidence on its own (`class.go:50`), and the dispatcher has a reachable pre-dispatch *position-flat* refusal (`margindispatch.go:289`): proven flat ⇒ the risk is gone ⇒ `Resolved`; held or unknown ⇒ the position is still exposed ⇒ terminal, recorded only on perp-flat evidence. Grouping this with a wiring defect loses the only distinction that matters |
| `ClassPrecondition` | NotApplied | a wiring defect, per §7.1 ruling (iv)'s "an error means a WIRING defect". **The incident is an additional alert, never the safety outcome** — the position is still liquidatable while the defect is diagnosed, so the episode routes terminal on the same evidence rule as the row above |
| `ClassInternal` | NotApplied | **terminal, plus an internal-defect incident.** A bug in this system is the one cause that says nothing about the venue, so no evidence licenses the funding branch, and the position is still liquidatable while the defect is diagnosed |
| `ClassTransport` | NotApplied | the venue formed no opinion but the request demonstrably did not execute — retry inside the latency budget, then as `ClassRateLimit` |
| `ClassTransport` | Unknown / Applied | op in flight; position latched; nothing re-resolves a live op before the next boot (#215) ⇒ terminal |
| any | Applied / Unknown | as above — terminal |
| second consecutive top-up failure after a delever released what it asked for | — | terminal |

### R6. "Terminal" is `EmergencyClosePackage`, WHICH DOES NOT EXIST YET

`GenerationTerminalFlatten` is reachable only from a parked leg —
`GenerationComplete` has **no successors** (`generationphase.go:333`) — and
`Deleverer.TerminalFlatten` *records* an ending and retires the ledger entry;
"issuing the flatten itself is the caller's" (`delever.go:624-632`). So the
terminal rung is §5.3's `EmergencyClosePackage`.

**And repository-wide, only its supersession half is built** (record §77 says so
explicitly). Calling `Deleverer.Supersede` alone would bar the ladder while
leaving the position open. **The evidence-gated both-leg close is therefore a
blocking deliverable of its own** (§5b), with recovery states, attempt and
slippage bounds, and a named invocation from the episode. The terminal outcome is
recorded **only after venue evidence shows the perp flat**.

It is the only rung that raises headroom without the margin path, because α
cancels for every α > 0: the single `q` that moves `L` is **zero**. Perp-first
sequencing reaches it correctly — closing the perp leg ends liquidation risk
outright, since the residual naked **long** carries no liquidation price.

### R7. `Complete` enters `AwaitingRePost` unconditionally — the direct repetition goes

Round 2 blocker 3: retaining SM §7's `Complete → Planned` alongside R1's
re-entry lets a completed generation open **another delever before the
re-post** — and in bracket 1 the breach *always* persists after a delever, so
the guard "a fresh read shows the breach persists" is true every time. That
reproduces the grind-to-zero R3 exists to prevent.

So: `Complete` → the episode's `AwaitingRePost`, unconditionally. A later
generation requires a **post-re-post** failure condition and consumes the same
durable episode budget (R3). `Complete → Planned` as an unguarded ladder
repetition is **removed** from SM §7.

**This edit invalidates the formal models' coverage, and revising them is a
BLOCKING prerequisite (§5e).** `AmbiguousOrder.tla` sets `MAX_GEN = 2`
*specifically* because "with a single generation the ladder's
`Complete -> next-generation` edge is unreachable" (line 61), and
`PC20_NeverTwoGenerations` is the coverage probe that a second generation starts.
Meanwhile `PdPlan` — "a generation is opened by the liq-distance ladder" — is
declared **out of model**, so the model cannot see the new re-post gate or the
episode budget at all. Left alone, the models keep certifying an edge the spec no
longer has, while the replacement behaviour lands in the region they exclude —
and no invariant goes red in either direction. The revision must model the
episode, the re-post authorization and result, the target-miss route and the
durable budget; preserve second-generation coverage through the new guarded path;
and re-run nominal, negative-control and coverage configurations before the
normative edit lands.

No `Δ` is ever carried across a generation: the re-post is sized from the
post-generation read of live `m, q, e, MMR_i, cum_i, A`. §7.1 already legislates
this — *"re-queries margin + position first (target-state, not delta)"*. An
under-filled generation leaves `x > q_final` and a shortfall of `D_i·(x −
q_final)`, which is simply the next iteration's input.

### R8. REFUTED — the available-balance read is a prerequisite

Previously deferred on the grounds that `S` is short of what was needed anyway.
That argument survives and is beside the point: by R1a the re-post cannot be
*sized* without it. `venue.MarginStatus` carries no free balance, and
venue-profiles §9 records that neither venue publishes available margin per
position — Binance's is `availableBalance` at the top level of
`GET /fapi/v3/account`; Bybit's is on the wallet-balance surface.

**There is no closed-form "general solve".** An earlier draft wrote
`q_final,i = (m + A + cum_i)/D_i`; it omits the realized-loss and fee terms and
is superseded by R1b's algorithm, which is the sole selection rule. Available
balance enters there, as `A₀` inside the feasibility inequality, and again after
the close as `A_live` in R1a's `add` clamp — never as a term in a closed form.
The reason it once looked like one is real and survives: staged balance lets the
remedy give up strictly less position than a purely self-funded reduction would.

### R9. `A₀` is account-scoped, so the SIZING is serialized per margin asset

The feasibility inequality spends available balance, which is reported per
**account and margin asset**. But generations run independently per hedge group
(`delever.go:37`) and margin ops serialize only per **position**
(`margindispatch.go:57`), so two episodes on one account can each read `A₀ = 100`,
each deem an 80-unit re-post feasible, and each choose too small a reduction.
The first to spend it leaves the second infeasible — and the second discovers
that as a *failed re-post*, which R5 routes to terminal. A shared quantity read
without reservation converts one venue's balance into two positions' plans.

**Ruling: the journaled intent is a durable RESERVATION, and sizing reads the
balance net of it.**

    A_effective = A₀(venue) − Σ outstanding reservations on (venue, account, asset)

A reservation is taken **atomically before `q_final` is selected**, is rebuilt at
§7.5's boot, and is released or adjusted only when its spend or its episode
reaches durable finality. Everything else in an episode — the generation, the
re-post dispatch, verification — runs concurrently across hedge groups exactly
as today.

**A LOCK over the sizing sequence was considered and is REFUSED, because it does
not close the hole.** Releasing the lock after *read `A₀` → select `q_final` →
journal intent* leaves the venue still reporting the full balance: a journal
record does not debit a venue. Episode A can journal an intent to spend 80 and
unlock, and episode B then sizes against the same 100 and also plans 80. The
hazard spans from sizing to **expenditure**, which is far past any section a lock
may be held across — so the primitive has to be one that persists, not one that
excludes. A lock protects a critical section; a reservation protects a quantity
over time.

An earlier draft claimed the lock gave "the same safety" as a reservation and
made §5h optional. **That claim is withdrawn: the reservation ledger is
required.**

**Serializing whole episodes was considered and is REFUSED, because it trades a
double-spend for a liquidation.** SM §7's `Held/Alert` is explicitly nonterminal:
a parked leg waits for its §6.7 finality, which can take arbitrarily long. An
episode-scoped lock therefore lets a first, parked episode block a second
position from receiving **any** remedy — no top-up and no delever — and the
correlated rally that breaches two positions at once is the exact condition where
both need one. Generations run independently per hedge group by design
(`delever.go:37`); removing that concurrency without replacing its liveness
guarantee is a regression on the safety kernel, not a conservative choice.

**The terminal branch NEVER queues**, and the reason is structural rather than a
priority rule: `EmergencyClosePackage` consumes no margin, so it contends for
nothing the critical section protects. A queued breach whose wait exceeds the
top-up latency budget therefore escalates straight to terminal instead of waiting
longer — the remedy that needs no balance is always admissible.

**Its eventual owner is §7.2's `treasury`**, which owns the staged rescue float
and is **unbuilt**. The reservation described above is the minimum that ledger
must provide, keyed by `(venue, account, margin asset)`; `treasury` subsumes it
when it exists rather than replacing it. Prerequisite §5h, **required**.

## 4. What this design changes in the normative documents

- **build-spec §7.1**: the false parenthetical, replaced by §1.1's
  bracket-qualified statement; the ladder re-entry (R1); the class matrix (R5);
  the terminal rung's identity and its unbuilt status (R6).
- **state-machines §7**: `Complete → Planned` removed as an unguarded repetition
  (R7). **And `TerminalFlatten` — the state and BOTH its incoming edges (the
  parked-leg terminal transition and the "cannot complete" transition) — is
  removed or redefined**, because leaving them normative leaves a reachable
  terminal path that records an ending and issues no close (R6). Every
  cannot-complete and imminence case invokes the episode-owned
  `EmergencyClosePackage`, preserving the quiesce-first ordering the two
  supersession rows already require, and records terminal completion **only on
  perp-flat evidence**. These edges are in scope for §5e's model revision. No new
  generation states are added — the terminal rung leaves §7 for §5.3.
- **venue-profiles**: the Bybit tier-table row corrected to
  `GET /v5/market/risk-limit` with the measured fields (§1.2), and the
  `mmDeduction: ""` absence spelling recorded.
- **build-spec §5.1**: the **remedy-episode record kind** and the **reservation
  record kind** (R3, R9) join the journal's declared records — a fifth and sixth
  alongside the `MarginOp`, its verdict, and the operator reconciliation.
- **build-spec §7.5**: the **episode-reinstatement gate** at stop-opening, and
  the rebuild of outstanding reservations at boot (R3, R9). Both are boot
  preconditions and neither is in §7.5's current gate set.
- **record §85**: errata on the `Deleverer.Open` argument (R1c).

## 5. Prerequisites this design OPENS (none are assumed)

a. **Available-balance read** on both venues (R1a, R8) — blocks the re-post
   sizing, therefore blocks B4.
b. **`EmergencyClosePackage`'s evidence-gated both-leg close** (R6) — blocks the
   terminal rung, therefore blocks B4.
c. **Bybit's isolated liquidation formula**, verified against a live position
   (R1b). Published material carries leverage, taker-fee and extra-margin terms
   the common relation does not; §85 verified the relation on **Binance only**.
d. **The verified insufficient-margin error code** on each venue (R5) — now a
   **BLOCKER, not an optimization.** With the default routed to terminal, the
   funding rung is unreachable until a rejection can be positively identified as
   insufficient funds. Without it, this redesign flattens where it should rescue.
e. **Formal-model revision** (R7) — the episode, the re-post authorization and
   result, the target-miss route, the durable budget, and preserved
   second-generation coverage; nominal + negative-control + coverage re-run.
f. **A venue-verified `AuthReason` and a non-mutating execution-readiness
   surface** (R5) — not required; it would permit the auth split rather than
   routing every auth failure to the actuator.
g. **The tier basis** for bracket membership (R1b). On **Binance** the 50,000
   boundary is reachable on demo, so this closes by measurement (§7). On
   **Bybit** the 2,000,000 boundary is not reachable on demo, so measurement
   cannot close it — and an unverified production assumption is not an option
   here. **Ruling: supported Bybit notional is capped inside tier 1 until the
   basis is verified**, which makes every permitted basis agree and the question
   moot on that venue. §7.2's small-size posture already implies the cap; this
   makes it load-bearing rather than incidental, and names what lifts it.
   Bybit's primary documentation appears to state that linear position value is
   `quantity × mark price` and that the tier adjusts with the mark — recorded as
   a **claim to verify**, not a fact, per venue-profiles' sourcing rule.
h. **A reservation ledger** per `(venue, account, margin asset)` (R9) —
   **required.** A lock over the sizing sequence does not substitute: the venue
   reports the full balance until the spend actually lands, so the intent must
   persist and be subtracted by every later sizing. §7.2's `treasury` is its
   eventual owner and subsumes it.

## 6. Rejected findings

- **"A bracket crossing invalidates the not-worse crash claim."** Convexity
  through the origin gives the crossing a weakly **favourable** sign (§1.1).
- **"Flooring `q_final` is the rounding rule."** Superseded by R4 — accepted, and
  the rule now enumerates `r`.

Round 1's other two rejections are **withdrawn**: `ClassAuth` is broader than a
dead credential, and §7.4's actuator does hold an independent credential, so the
auth branch has remedies (R5).

## 7. Measurement plan

Demo, both venues; credentials exist. Demo settles mechanism, never economics.

1. **Binance bracket-1 ceiling and `notionalCoef`** via authenticated
   `leverageBracket` — the one input §1.1's normative claim depends on.
2. **A materially ADVERSE partial close**, not a near-entry one: record `m, q, e,
   MMR_i, cum_i, L, A` before; delever by an on-grid `r`; record `ΔA` and confirm
   it equals `(1−α)m + r·(e−P) − fees` rather than `(1−α)m`. This is the run that
   #221 could not have produced, and its absence is what made R1a invisible.
3. **Positive control**: the bracket-1, near-entry case reproduces #221's exact
   invariance, so the run distinguishes "the formula is right" from "nothing
   moved".
4. **Predicted-vs-observed `liqPrice`** after the re-post, on both venues — the
   check that would surface a wrong Bybit coefficient (§5c).
5. **A tier-boundary run, which the plan above cannot substitute for.** Fetching
   the bracket table establishes the **bounds**; it does not establish **which
   notional the liquidation engine evaluates them at**. So: hold a position whose
   notional sits just above a boundary and whose `q·L*` sits just below it, and
   read the venue's own `liqPrice`. Mark-based, entry-based and `L*`-based
   membership predict different values there and the same value everywhere else,
   which is why no other run in this plan can settle it (§5g). Binance's boundary
   at 50,000 USDT notional is reachable on demo; Bybit's at 2,000,000 is not,
   so on Bybit this stays an open assumption rather than a measurement.
6. **Bybit's formula needs two ORTHOGONAL cases, not one predicted/observed
   point**, capturing leverage, taker fee rate, initial margin, separately
   identified added margin and the deduction — a single point is satisfiable by
   more than one candidate formula (§5c).

Both Binance measurements above already exist in part: `leverageBracket` was
called on demo 2026-08-09 at weight 1, returning BTCUSDT brackets
`1: [0, 50000) MMR 0.004 cum 0`, `2: [50000, 250000) MMR 0.005 cum 50`,
`3: [250000, 3M) MMR 0.010 cum 1300`. Maintenance continuity holds exactly at
both boundaries (200 and 1200 from either side), which measures the
`MM(N) = N·MMR_i − cum_i` model this design's convexity argument rests on. The
same check on Bybit's published tiers gives 10,000 from either side of the
2,000,000 boundary **only if tier 1's `mmDeduction` is exactly zero** — so the
empty string's value is established by the neighbouring tier's arithmetic rather
than assumed. **No `notionalCoef` field is present** on the symbol-scoped
response under §4.1's `multiAssetsMargin == false` posture; it is recorded as
absent-under-this-posture rather than retained in the solve.

No code in this slice.
