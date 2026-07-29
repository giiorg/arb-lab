# Milestone 2 — the kill gate: opening decisions (2026-07-29)

**Status:** milestone 2 is **open and nothing is built in it.** Milestone 1's seven slices are closed,
the doc set is at **v0.9.12**, and `grep -rn "SPEC EDIT OWED" internal/ tools/` returns zero. Branch
`feat/milestone-1-foundation` — verify with `git status --short` and
`git rev-list --left-right --count @{u}...HEAD` rather than believing this sentence.
**Doc set:** implements `build-spec.md` §12.2 (with §8 as its fidelity contract). Milestone 1's record
is `2026-07-26-milestone-1-foundation-decisions.md` and stays live — §1's deferral table and §9b are
cited below.

This record exists so a fresh session can resume without re-deriving anything. **Session 28 wrote no
code.** It established the ordering of milestone 2, settled one question that would otherwise be
reopened every time somebody looks at the funding feed, and surfaced one roster item and one scope
question. The live statement is at **`## RESUME HERE`** at the bottom.

---

## §31 — session 28: milestone 2's ordering, and the derived-vs-authoritative question settled

### §31a — the ordering, and why capture goes first

**Decision: self-capture of the venue streams starts before the dispatcher and before `strategy`.**

The ordering is not dependency-driven, and that is the point. Every other item in milestone 2 costs the
same whether it starts today or next month. Capture does not: **today is the only day you can capture
today.** §8's requirement is *"self-capture both streams from day one for parity + recent days"*, and
both halves of that phrase are calendar-bound — parity needs a stretch of time where we hold our own
capture *and* the vendor's for the same window, and that overlap can only ever be created going forward.

The resulting order for milestone 2:

| # | Step | Why here |
|---|---|---|
| 1 | **Self-capture running** | The only item whose cost grows while other work happens |
| 2 | **Parity dispatcher + persisted `DispatchSeq`** | §31f — stops being deferrable at this milestone |
| 3 | **`strategy` context + `cashcarry` + the fee seam** | §2.4's money math; no fee fetch path exists today |
| 4 | **The pre-registered calibration artifact** | §8 — approved *before* the promotion sample is scored |
| 5 | **The research simulator, then the verdict** | The kill gate itself |

**Rejected: dispatcher first (the dependency-ordered choice).** It reads as the disciplined option —
build the spine, then feed it. It was rejected because the dispatcher is **not** a prerequisite for
capture (§31e), so ordering by dependency here buys nothing and spends the one budget that cannot be
refilled.

**Rejected: rely on Tardis for everything and capture later.** Tardis sells both streams, so history is
purchasable. What is not purchasable is the paired window: §3.5 defines parity as replaying **our own
persisted log**, and our freshness budgets and cross-leg skew are measured against **our** `RecvTime`
— a timestamp from our machine, on our network. Vendor data carries the vendor's. Deferring also defers
the moment we find out the capture code was wrong all along.

**Step 4's placement has a constraint worth restating**, because it looks like bureaucracy and is not:
§8's thresholds must be *derived from* measured data yet *frozen before* the promotion sample is scored.
The only way both hold is to split the captured data into a calibration set and a declared-frozen
holdout **up front, before looking at either**. §8's own argument: two implementations agreeing on the
same artifact proves determinism, not independence — the **governance** is what prevents
tuning-until-it-passes.

### §31b — THE SETTLED QUESTION: the funding rate is consumed, never derived

The user asked, before any capture code was written, whether we should take primitives (trades, book)
and compute the funding rate ourselves rather than consume the venue's published field — on the
reasonable instinct that deriving is more precise and less dependent on someone else's pipeline.

**Decision: consume the venue's published live estimate. Do not derive it.**

**The reason is not that deriving is hard.** That argument would be weak, and it would invite a future
session with more time to reopen it. The reason is categorical:

> **The funding rate is not a market observation. It is the venue's own policy computation, and the
> venue's number is the one that debits the account.**

There is no independent truth behind it that a better computation could approach. Suppose we replicated
the formula perfectly and got a different answer — **ours would be the wrong one**, by construction. It
is closer to a central bank's policy rate than to a market price. The job is to receive it, not to
estimate it.

**Rejected: local premium-index replication.** Recorded with what it would actually require, so the
scope is on the record rather than imagined: the venue's **impact bid/ask** (a price computed by walking
an undisclosed notional into its own book) on the venue's own sampling cadence and averaging window; the
**index price** (a basket of *other exchanges'* spot prices, under the venue's constituent list, weights,
and outlier/staleness exclusion rules); the **interest** term and **damper** as venue policy constants;
and the **cap/floor**, which is venue policy that *changes* — `tutorial.md` §1.4 already separates damper
from cap, and `venue-profiles.md` §1 records that the cap must be read live (`fundingCap` on Bybit, the
`fundingInfo` overlay on Binance). Every one of these is a venue-internal parameter, and getting all of
them right still loses to the decisive argument above.

**Trades are not even the input, which is the sharper correction.** The premium index is computed from
the **order book**, not from time-and-sales. `tutorial.md` §1.6 states the neighbouring fact — *"Last:
the last trade on this venue. Noisy, manipulable"* — and the venue builds mark and funding off index +
book **precisely so that trades cannot drive them**. Trades are the one primitive funding is designed to
be insulated from. (They remain worth capturing, for realized slippage, §8's persistence model, and the
adverse-selection calibration — just not as a funding input.)

**The discriminator worth carrying, because it flips:**

| Situation | Rule |
|---|---|
| The venue publishes a lossy or lagged view of something with an **independent truth** | **Reconstruct locally** — the order book from snapshot + deltas (which `marketdata` already does), realized slippage, executable depth |
| The venue's computation **is** the definition | **Take the venue's number** — funding rate, mark price, liquidation price, fee tier |

**§8 has already made this call once, in the other direction, and the shape is identical.** Coinglass is
rejected not for being slow but because funding OHLC **is a different number** — the settled rate, when
the signal is the live estimate. Both rulings are "which representation of this quantity", never "which
source is faster".

**⚠ Verification flag.** The premium-index mechanics used above (sampling cadence, impact-notional
construction) are **not recorded in `venue-profiles.md`** and carry no dated source. They were used
*only* to justify not going down this road, which is the one use needing no verification. **Nothing we
build may depend on them** without a verification into Profiles under its stated discipline.

### §31c — latency is not the axis this strategy is scored on

Established so that capture design does not get optimized against the wrong property.

The signal path is **latency-insensitive by construction**. §2.4 puts minimum holds at **weeks** — the
same length-scale as the negative regimes; a ~20-day hold spans ~60 eight-hour settlements; and §2.4's
default estimator for later settlements is a **trailing EWMA with a ~3-day half-life**, with exit gated
on a dwell of `exit_persist` settlements. Nothing in that decision moves if a funding reading is two
seconds old.

**Systematic bias dominates, and the comparison is derivable from figures the spec already pins.** A
*systematic* error of 0.001% per settlement — a rounding direction, a mis-parsed Bybit delta, a wrong
interval — compounds over ~60 settlements to **0.06% of notional**, against a round trip of
**0.34–0.38%**. That is roughly a sixth of the entire transaction cost, silently. Latency error affects
at most the single settlement being decided around, and usually not even that.

**Where latency genuinely does matter** — both separate pipelines, neither the capture path:
1. **Execution** — the spot↔perp leg gap (§6.6's leg-risk mandate; §8's decision-to-arrival latency).
2. **Mark-driven liq monitoring** (§7.1 `[→ R2c-1]`) — a pure rally moves mark toward the short's liq
   price **with no account event**, which is why `StreamMark`, the REST failover and `T_blind` exist.
   A safety deadline, not a profit optimization.

**Consequence for the capture design:** spend the effort on **losslessness and provenance**, not on
latency. This is the same allocation the project already makes elsewhere — dimensional money types and
an oracle, rather than colocation.

### §31d — OPEN: the capture set is probably wider than §12.2's phrase (decide at design)

§12.2 says *"self-capture both funding streams from day one."* §8's fill model — the thing that makes the
simulator credible — requires *"time-synchronized spot & perp books"* and models each fill by
*"walking the actual order quantity through book depth"*. **Funding-only capture cannot support that**,
and `venue-profiles.md` §3 closes the other door: **spot has no mark and no funding at all**, so the spot
leg is priced from its own book.

So the honest capture set looks like *both funding streams **plus** the spot and perp books for BTC*.
**Not yet decided:** whether §8 intends Tardis for books with self-capture scoped to the funding signal.

**Decide this before capture starts.** It is the expensive-to-revisit half — the funding-only choice is
discovered to be wrong at step 5, by which point the missing months cannot be recovered. If the wider
set is taken, it is plausibly a **core §12.2 edit** and gets marked `[SPEC EDIT OWED]` under the usual
discipline.

Practical note: do **not** estimate the disk cost. Funding is small (Binance publishes `markPrice@1s`
once per second); books at 100–250ms with 50 levels are orders of magnitude larger. Run it for one day
and measure.

### §31e — why capture needs no dispatcher, and the objection that was tested rather than assumed

Milestone 1's record refused to write venue clients on a stated ground: *"a WS client with no dispatcher
to feed would have to invent its own arbitration, which is the second-authority defect §3.5 spends a
paragraph forbidding."*

**That objection was re-tested against a capture tool and does not apply.** Arbitration is the act of
deciding the order in which inputs from *different* sources are consumed — the decision §3.5 requires be
made in exactly one place and persisted as `IngressSeq`. A capture tool makes no such decision: one file
per source, in that source's arrival order, with a local `RecvTime`. Nothing is merged, nothing is
prioritized, no `IngressSeq` is minted.

**The standing risk this creates, stated now rather than found later: the capture tool must not grow into
the engine's ingestion path.** The temptation arrives the moment the dispatcher lands, because both touch
the same bytes. Merging them is precisely how a capture tool becomes a second authority over ordering.

### §31f — §12.1's roster is empty of packages and NOT of its own sentence: the parity dispatcher

`HANDOFF.md` v0.9.12 states milestone 1's roster is empty. **That is true of §12.1's package list and not
of its full sentence**, which also reads: *"Parity dispatcher and all §3.6 CI gates from day one."* The
six gates landed. The dispatcher did not — `IngressSeq` and `TimerSource` exist in the tree **only as
comments promising a component that will stamp them**.

**This is a recorded deferral, not drift**, and the reason is real: milestone 1's §1 deferral table gives
*"Needs `events` + `clock` + persisted `DispatchSeq`"*, and §9b scopes the `events` slice to the envelope
and the three lanes explicitly excluding the dispatcher. `events` is done; the **persisted `DispatchSeq`**
is not, and `migrations/` currently holds a README and nothing else.

**It stops being deferrable at this milestone**, because §8 defines the research simulator as *"the §3.5
engine … over the `Clock`/`MarketDataSource`/`ExecutionVenue` seam with a single total-order dispatcher,
**not** a parallel simulator."* The kill-gate number is produced by that engine or it is produced by
something whose agreement with the live path is unproven.

**It is already leaking into components that shipped, which is the part worth watching.** `engine/state`
accepts an *equal* `RecvTime` on write specifically because under `TestClock` every observation dispatched
between advances shares a timestamp — and §29's own comment says the invariant that would otherwise
prevent the problem *"lives inside a component that does not exist"*. Each such accommodation is cheap in
isolation and collectively becomes the parallel simulator §8 forbids.

Pulling this in also pulls in **`platform/outbox`'s store half** (milestone 1 record item 1b): the
`Append` verb, the SQL, the migration, the driver. Its own warning applies on arrival — check first that
the append and the journal write are genuinely **one transaction**, because everything in
`platform/outbox` assumes it and nothing there can test it.

### §31g — the capture tool's shape, as far as it is settled

Not a design — the design is the next session's work. These are the constraints that are already decided,
each with the failure it prevents.

1. **Raw bytes verbatim, plus `RecvTime`, plus a per-source counter. Append-only.** Parsing is
   redoable against a raw log forever; **capture is not redoable at all.** A normalizer bug, or a
   corrected understanding of a venue's format — which this project has had repeatedly — destroys the
   original fact if it was parsed on the way in.
2. **Capture the REST overlay too.** Binance's `GET /fapi/v1/fundingInfo` has the property that
   **absent means 8h** (`venue-profiles.md` §1). Without recording the fetch, *"we asked and it was
   empty, so 8h is correct"* and *"we never asked, so 8h is a guess"* are indistinguishable in our own
   archive — and §4's v0.9.9 rule already refuses a defaulted zero for exactly this reason.
3. **Gaps are recorded, never absorbed.** §3.5's capture lane is *"append-only with per-`Source`
   sequence numbers … any drop is a recorded capture gap — never a silent reorder"*, marking the
   affected window non-replayable and alerting. On reconnect, write a gap marker.
4. **It needs its own liveness check, and the check needs a positive control.** A dead capture process
   and a quiet market are **identical from the outside** — nothing errors, the file just stops growing,
   and it is discovered three months later at scoring time. Same shape as §3.4's silence timer: per-source
   message counts, gap markers, and an assertion that the newest record is recent. Then kill a stream
   deliberately and confirm a **recorded gap and an alert**, not a silent hole. A monitor that has never
   reported a problem has not been shown able to.

**Two venue traps already visible, both of which fail silently** (`venue-profiles.md` §1):
- Binance's legacy `/ws/` path was decommissioned 2026-04-23 and **returns nothing rather than an
  error** — an old code sample yields a process that connects, waits politely, and captures an empty file.
- Bybit sends **snapshot then delta, and an absent field means unchanged, never null-coerce** — a naive
  per-message read of `fundingIntervalHour` gets a valid number on the snapshot and a zero on every delta
  after it, which is exactly what §2.4's `[→ C8]` live-interval rule exists to catch.

---

## RESUME HERE — **SUPERSEDED by session 29.** Historical from here down

**The live statement is `docs/superpowers/specs/2026-07-29-capture-tool-design.md` `## RESUME HERE`,
and `HANDOFF.md` `## START HERE (session 29)`.** Session 29 took both decisions this block leaves open
— self-capture covers **funding + books** (§31d resolved), and capture runs on the dev machine — and
designed the tool against them. The next work is **building slice A**, not designing.

§31a–g above stay live as *reasoning* and are cited throughout §32; only this block is stale.

---

## RESUME HERE — as written at the end of session 28 (milestone 2 opened, no code)

**Branch `feat/milestone-1-foundation`, tree clean apart from this session's docs, `make ci` green,
23+ commits unpushed** *(both figures now stale — session 29 committed and pushed; verify with
`git status --short` and `git rev-list --left-right --count @{u}...HEAD`)*. Milestone 1 closed;
**milestone 2 is open and empty**.

### The next work: DESIGN the capture tool *(DONE in session 29 — see the supersession note above)*

The user's instruction at the end of session 28 was explicit — **design the capture tool in a fresh
conversation.** Everything needed is in §31 above; nothing has to be re-derived.

**Decide first, because it changes what gets built (§31d):** does self-capture cover **books as well as
funding**, or funding only with Tardis for books? The wider set is the safer default and is plausibly a
core §12.2 spec edit.

**Then design, against §31g's four constraints:** the record format, the file/directory layout, the
gap-marker representation, the REST-overlay capture, and the liveness check with its positive control.

**Do not re-open (§31b): the funding rate is consumed from the venue, never derived.** The reason is
categorical, not effort-based — the venue's computation *is* what charges the account. Trades are not
even the premium index's input; the book is.

**Do not optimize for latency (§31c).** The signal path is latency-insensitive by construction; the
capture design's properties are losslessness and provenance.

**Keep the capture tool out of the engine's ingestion path (§31e)**, permanently.

### Standing debts, unchanged

1. **`venue-profiles.md`'s verification of the two venues' depth update-ID shapes**, which
   `venue.BookDelta`'s span design cites. Owed since session 20. A Profiles verification rather than a
   core edit, so the marker grep has never answered for it and never will. **§31d may promote this from
   background debt to blocking** — if we self-capture books, their update-ID shapes stop being theoretical.
2. **The parity dispatcher + persisted `DispatchSeq`** (§31f), which pulls in `platform/outbox`'s store
   half and the first migration. Step 2 of the milestone.
3. **No fee-fetch path exists** — §2.4 forbids hardcoded fee constants and `Fee` appears nowhere in
   `internal/` outside a doc comment. Step 3.
