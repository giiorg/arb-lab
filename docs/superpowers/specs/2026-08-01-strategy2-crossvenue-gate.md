# Strategy #2 crossvenue read-path, research sim & gate procedure — design (§45, 2026-08-01, session 58)

**Status: RULED + SLICES A AND B BUILT — all four §45f forks were ruled by
the user 2026-08-01 (session 58, after slice A landed); slice B (the domain
math, §45k) landed session 59; slices C–D build in order, nothing waits on
a decision.** Mandate: §42's RESUME item 3 and §44's RESUME item 2 — the
`strategy/crossvenue` read-path, the research simulator, and the gate's
procedure commit. Taken up while §44e's per-tier disk figure accumulates its
measured day (the §37-preamble ordering principle: the figure is a one-command
calendar checkpoint, and idling against it is the rejected alternative).

This record pins **structure, placement and governance shape**. It deliberately
pins **no gate parameters** — the §41e doctrine, inherited whole: every
threshold, quantile, entry level and evidence floor is chosen at the procedure
commit (slice D), with rationale beside each constant, never here.

## §45a — scope: what the three deliverables are, stated once

1. **The read-path**: committed code that turns the §42c capture (four streams
   × eight symbols) plus post-cutoff settled-funding REST history into the
   gate's evidence — matched cross-venue funding series (decision-time
   estimates AND settled truth) and per-symbol book-depth measurements.
2. **The research simulator**: the engine that walks that evidence and produces
   episode-level economics under §42e's frame — persistence-conditioned entry,
   hysteresis exit, four taker legs, both venues' `C_allocated`, funding-grid
   phase as a first-class term, measured depth costs. It is core §8's "$1K
   economic gate … cleared by milestone 2's research sim", transplanted to
   strategy #2.
3. **The gate procedure commit**: the sim plus pinned parameters plus the §41e
   governance — VCS-bound execution, dataset ledger, write-once artifact,
   detached attestation, holdout declared before scoring, kill authority real
   (milestones 3–6 are hard-gated on the verdict, core §12).

What the gate is NOT: it does not score this week. Selection-clean evidence
(§43b) begins 2026-08-01T00Z for settled funding and 2026-08-01T11:13Z for
books; the scoring instant is fixed by a deterministic end rule (§45f fork 1),
and the machinery lands now so the evidence window is spent accumulating, not
building.

## §45b — the read-path rides the measurement walker, not the replay tape

**Decision: the crossvenue read-path extends the `family2` style — a
manifest-gated statistical walk over `capture.WalkRecords`-decoded streams —
and does NOT extend `engine/replay`'s tape.** Two facts decide it, both
measured against the code this session:

- The gate's unit of evidence is the matched settlement and the sampled book
  state, not the dispatched envelope. Nothing in §42e's frame needs the
  dispatcher's total order, mailbox freshness re-judgement, or `TestClock`
  timers — that machinery is the *backtester's* parity claim (core §8), a
  milestone-5 deliverable for strategy #2's execution half.
- `replay.Ingest` burns an `AcceptOrdinal` on every consumed record, emitting
  or not — so adding book streams to the tape roster renumbers every existing
  funding envelope's ordinal and invalidates every committed parity fixture
  **with no behaviour change involved**. A roster change that silently
  invalidates golden logs is exactly the kind of drift this repo marks, and
  the research sim gets nothing for that price.

**Rejected: extend `internal/app/sim` and the replay tape.** Costs a book
decoder kind, book routing in the sim root, the ordinal renumbering above, and
couples the gate's calendar to parity machinery it does not consume. The
tape/dispatcher path remains the right home for the milestone-5 backtester,
where fill-model parity IS the claim.

**Rejected: reuse `family2.go` itself by widening it.** `family2` is the v1
calibration's collector; its H1/H2 guards and its Binance-only stream roster
are v1's frozen procedure identity. The gate gets its own walker with its own
guards (post-cutoff seal instead of H1), sharing the `capture` package's
decode API and the `BookStreamSpec`-closure pattern rather than the file.

**Torn-tail policy follows `family2`, not `WalkRecords`' recovery default:**
the walker consumes manifested days only, refuses an unmanifested day, and
treats a torn manifested file as `ErrArchiveCorrupt` — gate evidence is
vouched-complete files, and measuring around damage silently thins the sample
(the recorded design statement behind the two readers' opposite policies).

## §45c — placement: the name in the records vs. the tree the gates police

The records say "`strategy/crossvenue`". Measured against `hexboundary`'s
classifier, a top-level `internal/strategy/crossvenue` is `layerOther` — the
gate would say NOTHING about it, and by this repo's own rule a silent gate is
not evidence. **Decision: the capability lands split along the context's
existing shape, every piece policed:**

- **`internal/strategy/domain/crossvenue`** — pure math, mirrors
  `domain/calibrate`: pairing (venue-published instants, the screening's rules
  restated as domain code with an oracle), the episode model, the
  persistence-conditioned scoring statistic, the funding-phase ledger, the
  depth-cost bound. Imports `platform/` and sibling domain only.
- **`internal/strategy/infrastructure/crossvenue`** — the walker, peer of
  `screening`: manifest-gated archive reads, REST-history reads (screening's
  `store.go` raw-file + digest-ledger pattern), venue decoders bound as
  closures, book accumulation.
- **`internal/strategy/infrastructure/driving`** — the CLI verbs (`arblab
  crossvenue …`), beside `calibrate.go`/`screen.go`, reusing
  `ReadBuildRevision`/`CheckExecutionBinding` for slice D verbatim.
- **`internal/engine/venue`** — `DecodeBybitOrderbook`, beside every other
  wire decoder (§45d).

## §45d — the Bybit book decoder is the one new engine piece, and it lands first

Measured gap: `engine/venue` decodes Binance depth20 (both products) and
NOTHING parses Bybit `orderbook.50` — the capture has recorded those bytes
since day one and the shortlist doubled the surface yesterday. Bybit's book
wire is snapshot+delta (unlike Binance's per-frame snapshots), so this is a
decoder AND an accumulator:

- **Decoder in `engine/venue`** (`DecodeBybitOrderbook`): wire bytes →
  `venue.BookSnapshot` | `venue.BookDelta`, refusing unknown type words and
  malformed levels by name — profiles §3's verified facts (u/seq semantics,
  the snapshot-on-reconnect rule) are the contract, and any fact the decoder
  needs that the profiles do not hold is verified INTO the profiles first,
  never assumed.
- **Accumulator in `infrastructure/crossvenue`**: applies deltas to the
  running book, refuses a delta that does not follow (sequence break →
  re-anchor at next snapshot, gap counted), samples book state at the walker's
  instants. It is walker machinery, not venue wire knowledge, so it lives with
  the walker; `marketdata`'s domain accumulator serves the live pipeline and
  stays untouched.

**Slice A ships with a shortlist smoke-decode** (`arblab crossvenue check`):
walk one captured day of all 40 sources, decode every record, report counts
and refusals per source. Nothing has ever parsed the alt bytes we started
recording at 11:13Z — a wire-shape surprise (an absent field on an alt, a
different type word) should surface while the loss is hours of capture, not
weeks. This is the §32 verify's honesty extended one level: verify proves the
bytes are intact; the smoke-decode proves they are *the thing we think they
are*.

## §45e — the simulator: no lookahead, settled truth, phase as a term

The sim's frame, inherited and made structural (parameters all slice-D):

- **Decision-time signal is the live estimate** (core §8's fidelity rule):
  entry/exit evaluate on what the capture's `markPrice`/`tickers` streams
  showed at that instant. Settled REST rates are the ACCRUAL truth episodes
  earn, never the entry signal — the screening scored on settled rates and
  said so; the gate does not get that licence.
- **Funding-grid phase is a ledger, not a footnote**: each leg accrues at its
  own venue-published instants; a window where one leg has settled and the
  other has not carries the unhedged-accrual term explicitly.
- **Costs are measured, per symbol, from our books**: the §8 conservative
  bound structure (worse of depth-walked-fill vs p90-conditional-on-size)
  re-measured from §42c capture; no BTC figure transfers (§41r caveat 1).
  Maker-entry is priced as the standing tactic where the fee schedule rewards
  it, never assumed.
- **Utilization honesty**: results quoted at expected time-in-position on
  full both-venue `C_allocated`.
- **Refusals, never defaults**: thin evidence, an episode spanning a capture
  gap, an unmeasured book at a priced instant, an unmanifested day — each a
  named sentinel that propagates, in the `ErrNoExcursions` mould.

**Exploration posture (until fork 2 is ruled):** pre-cutoff data is already
selection-contaminated (§43b) and is the legitimate development corpus;
post-cutoff data is the gate's evidence and the sim REFUSES to read it before
the procedure commit exists — the seal is a guard in code from slice C, not a
reminder.

## §45f — the forks and the rulings (user, 2026-08-01, session 58)

All four were put to the user after slice A landed and RULED the same
session, each as recommended; the alternatives stand recorded with their
prices:

1. **The end rule for the gate's evidence window** — the deterministic
   scoring trigger (§41's `PromotionIdentityDoc` lesson: never an
   operator-declared date). **RULED: ≥180 post-cutoff matched settlements
   per shortlisted symbol** (~30 days at the 4h grid — the same evidence
   floor the screening ranked on). A symbol still short when the others
   qualify is refused by name at scoring, never guessed. Rejected: a fixed
   calendar span (30 or 45 days) — a capture outage could leave a symbol
   thin while the calendar said go.
2. **Exploration posture** — **RULED: in-repo on pre-cutoff data only,
   labeled and logged** (the §41e informed-not-tuned audit trail);
   post-cutoff reads are SEALED IN CODE until the procedure commit exists.
   Rejected: §41p's scratch-clone-only discipline — cleaner separation, but
   it loses the logged trail of what informed the parameters.
3. **The pass shape** — **RULED: a pre-declared portfolio policy.** The
   procedure commit declares an allocation policy over the shortlist; the
   gate passes iff that portfolio's LCB net return clears the hurdle at
   measured utilization, with per-symbol verdicts reported as evidence.
   Rejected: any-symbol-clears (the deployable strategy becomes one alt —
   concentration the gate never priced) and all-must-clear (one dead listing
   kills a viable portfolio).
4. **Holdout shape inside the clean window** — **RULED: the clean window IS
   the promotion sample**; the holdout discipline is the end-rule seal plus
   §43b carried as a named input (v1's H1-by-era shape). Parameters must be
   pinned before ANY post-cutoff read, and ruling 2 makes that a refusal in
   code, not a rule the operator remembers. Rejected: splitting the clean
   window again — purer, but halves an already-scarce sample and delays the
   earliest honest verdict.

## §45g — verification plan: suspects named before fixtures exist

The checklist-before-battery suspects, from this design's own distinctions:

- Pairing: a fixture whose two venues share a grid ORIGIN collapses
  "published-instant pairing" into "grid arithmetic" (the anchor lesson);
  at least one fixture needs venue instants that match only via rounding,
  and one that must NOT match.
- Accumulator: a delta fixture whose sequence numbers are consecutive from
  the snapshot collapses "follows" into "arrived next"; the re-anchor path
  needs a gap that a wrong `<=` would silently absorb (the ratelimit lesson).
- Phase ledger: legs settling at the SAME instant collapse the phase term to
  zero — the term must be visible in at least one fixture, and its sign
  checked both ways.
- Smoke-decode: a fixture where every record decodes collapses "decoded and
  counted" into "counted"; one deliberately foreign frame must be REFUSED and
  reported, or the check is a line counter.
- Depth walk: an order smaller than level 1 never walks; the fixture must
  cross levels, and the two-venue worse-side choice needs a fixture where the
  venues DISAGREE about which side is worse.

Oracles: the episode/scoring math gets a `math/big` oracle (slice-B pattern);
the accumulator gets a model-based oracle (replay a shuffled-then-sorted
delta stream against a map-based reference book).

## §45h — deliberate absences

- **No gate parameters here** (§41e doctrine; slice D's commit).
- **No Bybit book decoder in the live pipeline**: `marketdata` wiring for
  Bybit books is milestone-3+ work; the decoder lands in `engine/venue` where
  the live path will find it, but no module wiring changes now.
- **No third venue, no spot legs, no broad tier** — §42c's absences stand.
- **No backtester claim**: the sim is a research instrument under §8's $1K
  tier; fill-model parity via the replay tape stays milestone-5.

## §45i — build order

- **Slice A — decode the shortlist's bytes**: `DecodeBybitOrderbook` (+
  profiles verification of any wire fact not yet held), the accumulator, the
  smoke-decode command over the live archive. Positive control: BTC book
  bytes through the same door. Full rhythm (checklist, mutations, oracle).
- **Slice B — the domain math**: pairing, episodes, phase ledger, scoring
  statistic, depth-cost bound; oracles beside each.
- **Slice C — the walker + sim CLI**: manifest-gated walk, REST-history
  store, the post-cutoff seal-in-code, `arblab crossvenue sim` over
  pre-cutoff data.
- **Slice D — the procedure commit**: params roster with rationale, VCS
  binding, dataset ledger, write-once artifact + attestation, holdout
  declaration naming §43b. **Blocked on §45f.**

## §45j — slice A build record (session 58, 2026-08-01)

**Slice A is BUILT and the smoke check ran against the live archive the same
session: all 40 streams decode clean — 0 refusals across ~3.7M data records.**

- **`venue.DecodeBybitOrderbook`** (`wire_bybit_book.go`): stateless — the
  book wire has no absent-field semantics, so the ticker's accumulator
  argument runs the other way and the file says so. One decoder for both
  products, on a measured fact: the envelope and every value type observed
  identical across a linear alt and the BTCUSDT spot stream. The wire facts it
  rests on were verified INTO the profiles first from our own capture bytes
  (`u`/`seq` JSON numbers, `ts`/`cts` epoch-ms numbers, string level pairs, a
  live delete observed) — the §45d rule executed, not just stated. `cts` is
  deliberately unread until its semantics are a verified profiles fact.
  Refusal boundaries follow errors.go's vocabulary: wrong topic FOREIGN (the
  reader goes to the wiring), missing `u`/`seq`/`ts`/symbol INCOMPLETE (the
  reader goes to the profiles); `u=1` — the venue's restart floor — DECODES,
  because monotonicity across the reset is the consumer's judgement.
- **`strategy/infrastructure/crossvenue`** (§45c's policed placement) with
  `Check`: walks one UTC day, routes every stream default-deny (a skip is
  NAMED, never silent), aggregates per stream across hour files, reports
  refusals with the first one's capture seq, reports torn tails, and REFUSES
  an empty walk (`ErrNoStreams`) — a wrong `-root` must not read as "0
  refusals". Stateful decoders start cold per file (§39p-2's documented cold
  start, counted decoded-not-emitted).
- **`arblab crossvenue check -day` CLI**: the day is a required argument,
  never a clock default (§41f's freeze-instant rule).

**The measured smoke result (2026-08-01, hours 00–11):** 40 streams, 0
refusals. The new decoder consumed 1.79M BTC + ~289k alt Bybit book frames;
every alt's four streams decode. Two named skips, both by design: the BTC
diff evidence stream (§42c's absence roster) and `fundingInfo` (REST-only;
its framing decodes). One observation for slice C: alt Bybit tickers emit
funding news RARELY (1–37 emissions per thousands of frames — the rest is
bid/ask churn), so the alt-side decision-time funding signal leans on
Binance's 1/s markPrice stream; Bybit's contribution is snapshot-anchored.

**Verification:** decoder 10/10 aimed mutations caught (one NOBUILD re-aim —
the topic-prefix mutation removed the only `strings` use); check 9/9 caught
(two NOBUILD re-aims on unused variables, and **one SURVIVED diagnosed into a
real fixture gap**: the wrong-day fixture differed in MONTH, so the
day-of-month comparison was unexercised — the fixture now differs in exactly
one component per file, and all three component mutants die). The
checklist-before-battery pass had already forced two fixture changes:
an op-ack frame so Decoded ≠ Emitted (equal counts collapse the distinction),
and the Binance dialect-routing fixture where a product swap surfaces as
refusals. Profiles gained the dated own-archive observation block. `make ci`
green, marker grep zero.

## §45k — slice B build record (session 59, 2026-08-01)

**Slice B is BUILT: `internal/strategy/domain/crossvenue` — pairing,
episode machine, phase ledger, cost selections, portfolio scoring at the
LCB — with an oracle beside each, calibrate's mirror (§45c).** Parameters
stay arguments throughout; slice D pins the values.

The semantics this slice makes normative, each stated in the code:

- **Pairing restates the screening's committed rules** (drop-and-count
  collision buckets, nearest-rounding of published instants) and ADDS what
  the screening threw away: each leg's exact instant survives into the
  `Match`, because the phase ledger accrues at venue instants, not
  buckets.
- **The episode machine emits windows `(OpenAt, CloseAt]`** instead of
  accruing inline — the no-lookahead rule made structural (trigger
  observed never collected; exit print collected because the position is
  open when it settles). Exit level is entry/divisor at RoundTowardZero,
  bit-identical to the screening's halving. The signal is the
  decision-time estimate; the ledger owns settled truth.
- **The phase ledger needs no cross-leg pairing to define the unhedged
  term**: at any instant where both legs have settled equally often the
  position is hedged, so the term is the running signed net's deviation
  from its value at the LAST count-balanced instant, extremes reported
  both ways. Same-instant settlements are processed atomically, which
  makes the aligned-grid collapse to zero exact. A count imbalance at
  close is REPORTED (`EndImbalance`), not refused — whether it voids the
  episode is slice D's rule.
- **A duplicated same-leg settlement inside a window is refused**
  (`ErrDuplicateSettlement`) where pairing would drop-and-count it: the
  screening's tolerance is its committed rule; accrual truth with a
  doubled print has no honest reading. The asymmetry is deliberate and
  documented at the sentinel.
- **Conservatism enters once, at selection** (calibrate's convention):
  `WorseOf` (walked vs p90) and `WorseSide` (per venue, independently —
  §45g's disagree case) are pure selections where negative measurements
  survive; `RoundTrip` is the CHARGING act and refuses a negative cell
  (`ErrCostNegative`) — an unpopulated cost must not read as free, and a
  market that measured negative is a governance question, not a discount.
- **The portfolio statistic is ruling §45f-3 executed**: weights positive,
  unique, summing EXACTLY to one; the series set must match the declared
  holdings exactly both ways. The LCB is a stationary-bootstrap percentile
  bound whose resampling stream is pinned to an in-package SplitMix64 (a
  rerun IS a replay), selected at the FLOOR order statistic — never above
  the exact rank, UpperQuantile's rule mirrored for a lower bound.

**Verification.** Four oracles: pairing (independent integer-ms rounding +
occupancy counting + big.Rat spreads), ledger (big.Rat brute-force prefix
re-summation, no running state, byte equality — the arithmetic is exact so
no tolerance exists to go vacuous), episodes (a second implementation in a
different shape: index-scanning over plain int64 mantissas, no
fixedpoint), LCB (independent SplitMix64 transcription + exact-rational
rank arithmetic + an inline floor-vs-ceil control at non-integer ranks).
The checklist-before-battery pass forced the collapse-avoiding fixtures up
front (jitter straddling a bucket edge where Round and Truncate disagree;
a non-zero balanced anchor so "deviation from anchor" ≠ "running net";
0.75/0.25 weights so "weighted" ≠ "average"; odd-mantissa exit level so
toward-zero ≠ half-even; boundary prints exactly ON OpenAt/CloseAt and a
spacing exactly AT maxGap) and added one fixture mid-battery-planning: a
flip landing EXACTLY at the entry level, without which the retrigger
comparison's `>=` was unpinned. **Mutations: 29 aimed, 29 caught**, plus
one SURVIVED diagnosed as an EQUIVALENT mutant — the ledger's same-instant
batch loop could never iterate twice because the duplicate rule forbids
it, so production now states the invariant as a guarded `if` with the
comment carrying the why — and one TIMEOUT!! (an `if false` mutant froze
the merge loop; the suite hung, naming no defect) re-aimed at the accrual
arithmetic and caught. `make ci` green, marker grep zero.

## RESUME HERE

Slices A and B are BUILT (§45j, §45k): the shortlist's bytes decode and
the gate's domain math exists with oracles. **All four §45f forks are
RULED** — nothing waits on a decision. Build in order:

1. **Slice C — the walker's measurement stages + sim CLI** (§45i):
   manifest-gated walk over the §42c capture, REST-history store
   (screening's raw-file + digest-ledger pattern), the Bybit book
   ACCUMULATOR (deltas applied with sequence-break re-anchor — §45d named
   it walker machinery and slice A did not need it; §45g's accumulator and
   depth-walk suspects are still open), decision-point sampling, and
   `arblab crossvenue sim` over pre-cutoff data with the post-cutoff seal
   IN CODE (ruling §45f-2) — the sim refuses post-cutoff reads until the
   procedure commit exists; exploration runs labeled and logged.
2. **Slice D — the procedure commit**: params roster with rationale (every
   argument slice B left open: pairing round, entry grid, exit divisor,
   max gap, allocation policy, mean block, resamples, confidence, seed,
   fees, depth cells), VCS binding, dataset ledger, write-once artifact +
   attestation, the holdout declaration naming §43b (ruling §45f-4), and
   the end rule (ruling §45f-1: ≥180 post-cutoff matched settlements per
   symbol, short symbols refused by name).

The capture rituals stand (PID 8495, verify COVERAGE/0-integrity — 2570
coverage findings at session 59's check, the 2002 pre-restart alt
window_absent findings by construction). §44e's per-tier figure remains
due by 2026-08-03T11:13Z; its clock runs independently of this record.
