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

## §45l — slice C build state (session 60, 2026-08-01): CODE + BATTERY BUILT, mutations and the live smoke PENDING

**Everything slice C names exists in code with its battery green and
`make ci` green; what has NOT run yet is the mutation battery and the
real-archive exploration smoke.** The session checkpointed here
deliberately (context budget), with the tree committed clean.

**The wire fact first (§45d's rule executed).** The accumulator's follows
rule needed a contiguity fact the profiles did not hold. Measured from our
own archive (all 10 Bybit book streams then recorded, 8,463,452 deltas):
every delta with an in-connection predecessor advanced `u` by EXACTLY 1 —
zero jumps, zero backward moves; snapshots appear ONLY as a
subscribe/reconnect's first frame; `seq` strictly increasing throughout.
Recorded into Profiles §5 as a dated own-archive observation BEFORE the
accumulator was written. Consequence: follows ≡ `u == lastU+1`; anything
else is a break, re-anchored at the next snapshot.

**What was built** (all in `strategy/infrastructure/crossvenue` unless
said otherwise):

- `walk.go` — the gate's own manifest walker (family2's SHAPE, not that
  file): strictly-before-day bound, file-granular vouching
  (`ErrDayUnmanifested`), torn = `ErrArchiveCorrupt`, and an `onFile`
  cold-start seam for stateful consumers (§39p-2).
- `seal.go` — §43b's two cutoffs as cited constants; enumeration bound =
  the seal made structural; settled rows AT or past the cutoff are
  dropped-and-counted (`SealedExcluded`), a row AT the instant sealed
  because evidence begins AT it. `ErrPostCutoffSealed` reserved for an
  explicit bound-past-the-seal request.
- `store.go` — settled REST store: screening imported (not copied — its
  fetchers are generic over symbol/cutoff), one file per (venue, symbol,
  cutoff-day), `ErrStoreExists` checked BEFORE the venue walk (a resume
  costs no requests), merge by exact instant with disagreement refused
  (`ErrStoreConflict`) and identity dedup.
- `sample.go` — decision-point sampling: epoch grid, RecvTime anchor both
  venues, family2's exactly-on-grid convention (the boundary instant
  reports the frame AT it, emitted on the next frame); Bybit ticker
  consumed through the venue accumulator with **churn refreshing the
  freshness anchor** — absent-means-unchanged is the venue's semantics,
  and without it §45j's rare-news alts would never sample; cold per file
  and per gap. `MergeDecisionPoints` joins equal instants, one-sided
  counted, signal = Binance − Bybit.
- `accumulator.go` — the Bybit book accumulator on the measured fact:
  snapshot re-anchors unconditionally, `u≠lastU+1` breaks (book dropped
  until next snapshot), delete-of-absent is a counted no-op (window
  slide, NOT desync — the reasoning is in the file), `Sample()` renders
  best-first truncated to the venue's 50 (bounds the unmaintained tail),
  `Drop()` for gap/file seams (not a break).
- `domain/crossvenue/depth.go` — `WalkedFill`, the standing depth walk.
  Deliberately NOT `CounterfactualSlippage(book, book, …)`: its pair
  validation requires the later instant strictly after, and satisfying a
  guard with a forged watermark is not this repo's move. Own sentinels
  (`ErrDepthShort`, `ErrCandidateInvalid`, `ErrSideInvalid`).
- `books.go` — per-venue cost walkers on the shared grid: walked-slip
  distributions per side + boundary samples at wanted instants
  (episode open/close); depth-short excluded-and-counted; a wanted
  instant with no measurable book carries its reason out.
- `sim.go` — the assembly. Cost composition stated once in its header:
  per venue per side, worse-of-boundaries → worse-of p90 → worse-side;
  `RoundTrip` doubles each cell. Episode refusals NAMED and excluded,
  EndImbalance's mould (voiding is slice D's rule). Period series sparse
  in the report (`NonZeroReturns`, period-start keyed). Label required
  (`ErrLabelUnset`) — §45f-2's labeled-and-logged trail; the report is
  write-once at the CLI.
- `cmd/arblab` — `crossvenue fetch` (politeGet, per-identity skip via
  `ErrStoreExists`) and `crossvenue sim` (-params JSON with
  DisallowUnknownFields, -out write-once).

**exhaustiveswitch bit during the build** — four `default:` switches over
`RecordKind` in the new walkers, refused by the gate and rewritten as
named groups. The gate doing exactly what §3.6 rule 10 promises.

**Checklist-before-battery found three fixture collapses BEFORE any
mutation ran** (the nine-times rule, now three-for-three on new code):
(1) no fixture held staleness EXACTLY equal to Slack — `>` vs `>=`
unpinned; a :16 frame against a :20 instant at 4s slack now pins
at-the-bound-is-fresh. (2) the file-boundary reset fixture used a churn
frame, which cannot distinguish the ticker-STATE reset from the anchor
reset — a funding DELTA in file 2 (emits against retained state, silent
against cold) now kills both. (3) the sim's four-sample distributions
made p90 ≡ max, so the walked arm of WorseOf could never win and
open-vs-close attribution shared a period — rebuilt with 38-sample
tails, venues disagreeing on the worse side (binance bid-worse via the
BOUNDARY WALK, bybit ask-worse via the P90 ARM), and a 10s period grid
whose `NonZeroReturns` key moves if attribution swaps to the open.

**Verification state:** all batteries green (`go test` whole repo via
`make ci`, race included), TWO new oracles green — `oracle_depth_test.go`
(independent big.Rat walk, rounding parity BY CONSTRUCTION: ≤4-decimal
inputs make every notional exact so only the final half-even division
rounds, reusing the score oracle's SplitMix64 + `roundHalfEven9`) and
`oracle_accumulator_test.go` (sorted-slice reference book over a
generated stream mixing follows/dup/jump/drop/absent-deletes, with
every arm asserted non-vacuous). Worked sim episode hand-computed end to
end: net 0.0008, cost 0.2219, return −0.2211, all four p90s pinned.

## §45m — slice C CLOSE (session 61, 2026-08-01): 31/31 aimed mutations caught, the smoke ran clean

**Checklist round two BEFORE the battery — the mirror pass found two more
fixture collapses** (instances 4 and 5 for this slice; the mirror rule is
"a fix applied in one place gets applied to its symmetric case", and both
were found by re-walking the pre-written roster against the fixtures
before running anything):

- **(4) The Bybit staleness bound was unpinned — §45l's finding 1 had
  been fixed on Binance only.** Every Bybit sampled instant sat strictly
  inside the 4s slack (1s, 1s, 3s) or strictly outside (6s, 6s, 8s), so
  `>` vs `>=` on the Bybit arm was indistinguishable — the ratelimit
  `freeAt` shape exactly. Moving the news frame :22 → :21 puts the :25
  instant EXACTLY at the bound with every expected count unchanged; only
  the comment moved. The mirror mutation then failed precisely one test.
- **(5) The Binance sampler's file-boundary reset had NO fixture** — the
  Bybit boundary test existed (§45l finding 2's home) and its Binance
  mirror did not; a held Binance estimate carried across a file boundary
  within a wide slack would have sampled as fresh with nothing failing.
  New `TestSampleBinanceFileBoundaryResets` (a 10:58:58 frame, an
  11:00:07 frame, one-minute slack — only the cold start explains the
  skip at 11:00:05). Its aimed mutation was then caught by exactly this
  test, so the gap was real, not a formality.

**The predicted `PANIC!!` was pre-empted, per the roster:** the
write-once test's transport stub indexed `pages[calls-1]` unguarded, so
disabling the pre-fetch exists check would have crashed the suite (a
kill by crash — the resume-costs-no-requests claim itself unchecked).
The stub now errors gracefully past its script, and the exists-check
mutant dies on the assertions it was aimed at.

**The battery: 31 distinct claims, 31 caught. Zero SURVIVED, zero
PANIC!!, zero TIMEOUT!!.** Three honest `NOBUILD!!` re-aims, all the
predicted unused-variable class, each re-aimed inside the same claim and
caught: the store conflict check (`if false` orphaned `c` → `c != c`),
and both WorseSide substitutions (dropping one argument orphaned the
other cell → closures returning one cell and explicitly discarding the
other). By file:

- `accumulator.go` 7/7 — duplicate-u absorbed by `>` (2 failing tests),
  jumped-u absorbed by `<` (4), cold delta applied (3), absent-delete
  uncounted (2), held position not advanced (3), window 49-not-50 (1),
  render sorted ascending (6 — the oracle among them).
- `walk.go` 5/5 — bound day admitted (1), mislabeled manifest accepted
  (1), empty universe not refused (1), unvouched file consumed (1), file
  boundary not signaled (3).
- `seal.go` 1/1 — at-cutoff row kept as pre-cutoff (1).
- `store.go` 4/4 — exists check disabled / resume costs requests (1),
  empty store reads as never-settled (1), conflicting refetch merged
  (re-aim, 1), merged series descending (3).
- `sample.go` 8/8 — boundary instant sampled on its own frame (2),
  Binance at-bound skipped as stale (1), **Bybit at-bound skipped as
  stale — the mirror (1)**, Binance gap not invalidating (1), **Binance
  file boundary carrying the estimate — the mirror (1)**, churn not
  refreshing the anchor (1), file boundary carrying ticker state (1),
  merge orientation mirrored (2).
- `sim.go` 7/7 — seal not wired (1), cost added not charged (1), return
  attributed to the open period (1), bid boundary worse-of dropped (1),
  p90 ask arm dropped (1), worse-side pinned to bid (re-aim, 1),
  worse-side pinned to ask (re-aim, 1) — the §45l rebuilt fixture doing
  exactly what it was rebuilt for: each of the four composition arms
  changes the cost, and the two venues disagree about the worse side so
  BOTH pin directions die.
- `domain/crossvenue/depth.go` 2/2 — short fill measured as a fill (2),
  ask orientation mirrored (2 — the big.Rat oracle among them).

**The live exploration smoke (slice A's precedent, the slice's positive
control) ran end to end, labeled `s61-smoke-btc`:**

- `crossvenue fetch -symbols BTCUSDT -cutoff 2026-08-01T00:00:00Z` —
  both venues stored verbatim with digests (binance `a78500bda48f…`,
  bybit `562bdbd47536…`), under the new `/research/crossvenue/rest/`
  gitignore rule (the CLI's "(git-ignored)" claim was ahead of the
  actual ignore file — rule added this session, in the screening-raw
  pattern; reports and params beside it in `sims/` are COMMITTED, the
  labeled-and-logged trail).
- `crossvenue sim` over the pre-cutoff corpus: **days 2026-07-30 +
  2026-07-31 only** (hashes `fdf6141c884b…`, `4eb3ebefaa95…`) — the
  2026-08-01 manifest exists and the enumeration bound left it unread,
  the seal structural in a live run. Binance 90,654 frames / 912
  skipped; Bybit 684,764 frames / 2,077 skipped; merge 202 joined,
  1,171 Binance-only, 7 Bybit-only. Pairing over the full REST history:
  7,550 Binance × 6,958 Bybit rows → **6,958 matched, zero collisions**.
  Nine episodes, all scored, zero refusals, `SealedExcluded` 0 (the
  fetch cutoff IS the seal instant, so the store holds nothing to
  exclude — the sim-battery fixture is where the exclusion arm is
  exercised). **Every episode's cost is exactly 0.0021 = 2×(0.0005) +
  2×(0.00055): all four p90s are ZERO at 0.05 BTC — §41p's
  calm-stratum zero-slippage finding, reproduced from our own measured
  books.** Sparse period attribution lands each return in its close's
  8h period; portfolio LCB −0.0049 over 6 periods (negative as a
  tiny-entry costs-only smoke must be).

**One measured observation for slice D (recorded, not absorbed): Bybit
decision-grid coverage is ~9%** (209 sampled of ~2,286 grid instants;
Binance covered ~60%). The mechanism is the designed cold-start rule
meeting Bybit's wire shape: tickers snapshot only at subscribe, the
hourly file rotation restarts ticker state cold (§39p-2's rule), and a
delta-only file leaves the state incomplete until the full field set
happens to recur — so coverage clusters after reconnects. This is the
conservative trade the walker documents, made visible by the honest
counters. If the procedure commit wants that coverage back, the safe
seam is the capture's own framing records (a file boundary with no
intervening Open/Gap record is the same connection, so carrying state
is not crossing a hole) — a fork for slice D to rule on explicitly,
not something to absorb into the walker silently.

## RESUME HERE

**Slice C is CLOSED (§45m): code, batteries, oracles, 31/31 mutations,
the labeled smoke. What remains, in order:**

1. **The §32m-D per-tier one-day figure, DUE BY 2026-08-03T11:13Z**
   (measurable from ~2026-08-02T11:13Z), landing as §44e in the §44
   record — `du` split BTC-tier vs alt-tier against §42c's derived
   ~0.6–1.1G/day/symbol, full-day verify over the measured day (0
   integrity; the restart's gaps will show honestly). **This is §38f's
   named revisit point** (ruling §42f-3): re-argue off-host on the
   measured disk clock.
2. **Slice D — the procedure commit**: params roster with rationale
   (pairing round, entry grid, exit divisor, max gap, allocation
   policy, mean block, resamples, confidence, seed, fees, depth cells —
   every argument slices B/C left open, now including a ruling on the
   §45m Bybit-coverage fork: accept ~9% Bybit grids, or carry ticker
   state across same-connection file boundaries keyed on framing
   records), VCS binding via `ReadBuildRevision`/`CheckExecutionBinding`,
   dataset ledger over the store digests + manifest-day hashes the
   report already carries, write-once artifact + detached attestation,
   the holdout declaration naming §43b (ruling §45f-4), and the end
   rule in code (ruling §45f-1: ≥180 post-cutoff matched settlements
   per shortlisted symbol, short symbols refused by name). The seal
   lifts ONLY through that commit.

The capture rituals stand (PID 8495, pgrep FIRST; verify COVERAGE/0-
integrity expected — session 61's verify: COVERAGE, 0 integrity, the
restart gap and pre-restart alt absences honest).

## RESUME HERE (session 60 — HISTORICAL, superseded by §45m's close)

**Slice C: code + battery + oracles + ci are DONE (§45l). Two work items
close the slice, then slice D:**

1. **The mutation battery** (`tools/mutate.sh`, aimed, one per claim).
   The planned roster, worked out but NOT yet run — aim at: the follows
   check both ways (`>` absorbs the dup, `<`/`>=`-shaped absorbs the
   jump — the two fixtures exist separately); the cold-delta return; the
   delete-absent counter (`if false` on the presence check — killed by
   the stats assertion); the lastU advance (killed by `UpdateID()==11`);
   `bybitBookDepth` 50→49 (killed by the 51-level fixture); the sort
   direction; walk.go's `>=` bound (bound-day fixture), Vouches check,
   day-mismatch check, empty-days check, onFile call (trace fixture);
   seal's `Before` (at-cutoff fixture); store's conflict `c != 0`,
   empty-glob check, ascending sort, pre-fetch exists check (patch the
   stub to overflow gracefully first — today it would panic, a PANIC!!
   non-verdict); sample.go's `t.Equal(recv)` break (boundary rate),
   staleness `>` (at-bound fixture), gap `cur=nil`, churn-refresh else-arm,
   per-file state reset (funding-delta fixture), merge orientation;
   sim.go's seal wiring (SealedExcluded==1 dies), Net.Sub→Add,
   close-period attribution (NonZeroReturns key dies), the four cost-
   composition substitutions (boundary-worse bid arm, p90 ask arm, both
   WorseSide directions — the §45l fixture was rebuilt so each changes
   the cost); depth.go's short check and orientation swap (plus oracle).
   Expect NOBUILD re-aims on unused-variable mutants as usual. Record
   verdicts in a §45m block.
2. **The live exploration smoke** (the slice's positive control, slice
   A's precedent): `arblab crossvenue fetch -symbols BTCUSDT -cutoff
   2026-08-01T00:00:00Z -rest research/crossvenue/rest` (public REST,
   polite pacing; Binance ~6 pages, Bybit ~25), then `arblab crossvenue
   sim -params <exploration json> -out research/crossvenue/sims/<label>.json`
   over the pre-cutoff BTC days (entry small, e.g. 0.00001 — machinery
   smoke, not economics; label it `s60-smoke-btc` or successor). Runtime
   minutes (≈15M frames). **Build the binary to the scratchpad if the
   capture is still writing bin/arblab's inode — never rebuild over a
   running capture's binary.** Note the sim walks BTC only: alts have no
   pre-cutoff books by construction (§43b — the cutoff IS their first
   byte).
3. **Then slice D — the procedure commit**: params roster with rationale
   (pairing round, entry grid, exit divisor, max gap, allocation policy,
   mean block, resamples, confidence, seed, fees, depth cells — every
   argument slices B/C left open), VCS binding via
   `ReadBuildRevision`/`CheckExecutionBinding`, dataset ledger over the
   store digests + manifest-day hashes the report already carries,
   write-once artifact + detached attestation, the holdout declaration
   naming §43b (ruling §45f-4), and the end rule in code (ruling §45f-1:
   ≥180 post-cutoff matched settlements per shortlisted symbol, short
   symbols refused by name). The seal lifts ONLY through that commit.

The capture rituals stand (PID 8495, pgrep FIRST; verify COVERAGE/0-
integrity expected). **§44e's per-tier figure is due by
2026-08-03T11:13Z** (measurable from ~2026-08-02T11:13Z) and outranks
slice C's remainder if the calendar is tight; its clock runs
independently of this record.
