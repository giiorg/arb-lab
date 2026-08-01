# Milestone 2 step 4 — the pre-registered calibration artifact: design (2026-07-31)

**Status:** design only — **nothing in this step is built.** Session 49. Implements §31a's step 4 row
(*"The pre-registered calibration artifact — §8: approved before the promotion sample is scored"*)
toward §12.2's milestone-2 deliverable: the `$1K` economic gate *"whose thresholds are the
pre-registered, independently-approved calibration artifact produced first in this milestone, before
the promotion sample is scored"*. It answers **§39l-1's step-4 half** (what data backs the numbers
the artifact freezes) and sharpens the step-5 half rather than pre-answering it (§41h). §40k-4's
rescue-margin registration is honored here (§41b family 3).

**Provenance note:** round 1 (Codex, session 49) — verdict NO-SHIP, two blockers + five majors,
**all seven valid**, all applied: the approval was prose the runner never reads (→ the committed
detached attestation, §41e — reversing this record's own first rejection of a committed approval
file, with the runner now its consumer); H1/H2 left both samples selectable after inspection
(→ H3's closed identities + outcome-independent promotion rule, §41c); the declared latency
contradicted §8's "measured" (→ the pre-registered `[SPEC EDIT OWED]` marker on the §40d
modeled-tier shape, §41b); the quantity-only p90 table dropped §8's book-state conditioning
(→ pre-registered strata with per-cell minimum evidence, §41b); the dispersion's imminent
residuals were underivable from settled history — §8's own "funding OHLC ≠ live estimate" one
step earlier (→ the two-part measurement over the archive's `markPrice@1s` stream + the
pointwise-more-adverse merge, §41b); `EntryDrift` was missing from a roster the loader must
satisfy, and the minimum-utilization threshold was named but never frozen (→ both added, §41b);
and the derivation's placement violated §3.6 rule 3's permitted-children letter (→ domain math +
driving adapter, §41f). One further defect was caught before the round: the approval block sat
inside its own hash preimage (§41d). Round 2 (Codex, fresh thread) — verdict NO-SHIP, two
blockers + five majors, **all seven valid**, all applied, digging one level deeper than round 1's
structural findings into observation definitions and execution binding: the promotion sample was
still selectable post-approval through the deferred fork/venue/end choices (→ the closed
promotion identity frozen inside the artifact + the fork decision moved to slice C's first act,
§41c/§41h); the negative-regime absence had an "accept a stated limitation" escape §8 never
grants (→ hard refusal; waiver only via a marked §8 edit + new version + rerun, §41c); the
passive p90 had no defined observation, making conforming implementations divergent (→ the
committed counterfactual — classify at *t*, walk at *t*+λ, reference best-at-*t* — with method
identity per cell, §41b); the utilization proxy measured eligibility, not §2.4's time-in-position
(→ the Enter/Continue occupancy replay over the closed sample, §41b); the p90 pooled both legs
under one key (→ the (venue, product, side, stratum, bucket) cell key, §41b); the attestation was
replaceable without changing the journaled identity (→ write-once attestation + its SHA-256 in
the run evidence, §41e); and the procedure commit named code without proving it executed (→
dirty-tree/revision-mismatch refusals + the executable digest in artifact and attestation,
§41e/§41f). Round 3 (Codex, fresh thread; a first attempt died at 6 minutes with only
evidence-gathering in its log — nothing formed, nothing recovered, retried clean) — verdict
NO-SHIP, one blocker + two majors, **all three valid**, all applied: sealing the fork/venue at
slice C still allowed inspecting the *public* holdout era across candidate identities first (→
H3's both-conditions rule — sealed before any holdout fetch AND before derivation — plus the
honesty declaration; dissolved at the strongest point by the user sealing **hybrid + Binance**
in this session, §41h); the occupancy replay demanded execution costs over a bookless
calibration era (→ the pre-registered constant-cost surrogate at a declared quantity and
stratum, conservative direction argued, the same frozen definition for the band's
entry-clearing level, §41b); and the p90 strata had no frozen classifier (→ classifier code
under the procedure commit, parameters in the hashed artifact, same classifier at calibration
and runtime, unplaceable book = refusal, §41b). **The review loop closed here, deliberately:**
three complete rounds descended from structure (r1) through observation definitions and
execution binding (r2) to contract pinning (r3); every §8 term now has a declared carrier — a
schema field, a refusal, or the dated seal — and the next verifier of these contracts is
slice A itself: the compiler, the six gates, and the battery are stronger checks of a frozen
interface than a fourth prose round. The findings are folded in below rather than appended,
because a record is what a later session resumes from, not a changelog.

**Doc set:** v0.9.17. The live statement of what to do next is at **`## RESUME HERE`** at the bottom.

---

## §41 — step 4's design

### Preamble — what step 4 is, and the one sentence that bounds it

Step 4 produces the **record of frozen numbers** that step 5's scored run is judged against: §8's
*"pre-registered, versioned calibration artifact"* — frozen dataset + holdout identifiers, schema +
content hash, per-number rationale, approval record — **produced and approved before the promotion
sample is scored**, so that step 5 cannot tune thresholds against the sample it is grading. It does
**not** run the scored gate (step 5), and it does **not** re-derive any structure step 3 built —
every seam it fills was built expecting it (`CalibrationRef` + hash on every class-2 value, the
`fixture:` marker discipline, §40e's one-artifact rule, `CostCalibration`'s field set). The boundary
discipline: **step 3 built structure, step 4 freezes numbers into it, step 5 drives it with replayed
data.** §8's own argument bounds the governance: two implementations agreeing on the same artifact
proves *determinism, not independence* — the governance (pre-registered procedure, frozen holdout,
approval before scoring) is what prevents tuning-until-it-passes.

### §41a — scope

**In:** the artifact's **schema** (context-owned, beside its loader — slice A's reference-schema
pattern) and **content-hash rule** (§41d); the **roster of frozen quantities in three provenance
families** (§41b) — the load-bearing decision; the **dataset freeze + holdout declaration
protocol**, declared before any derivation runs (§41c); the **governance**: mechanically-derived
thresholds under a pre-registered procedure, solo-honest approval (§41e); the **derivation tooling**
(`arblab calibrate`, §41f); the **runtime loader** that turns the committed artifact into the
class-2 values step 3 defined (§41g); and the **committed funding-history datasets** the
family-1 derivations read (§41c/§41f).

**Out, with the reason each stays out:** the scored run and its runner (step 5 — §40h/§40i/§40k-2);
the reference-**revision** repository (step 5, pre-registered §40k-2 — revisions are fee/reference
facts, not calibration numbers; the two mechanisms stay distinct because their change cadences and
authorities differ); the rescue margin's **real** derivation (§7.1's milestone-4 protocol — this
step freezes §40k-4's interim number with rationale, nothing more); the `$10K+` full
adverse-selection model (§8 tiers it out of milestone 2); and any change to the six §3.6 gates
(nothing here adds an importable surface the existing gates do not already police).

### §41b — the roster: every frozen quantity, in three provenance families

**Decision: the artifact freezes §8's threshold roster plus §40e's class-2 roster, and every
quantity is assigned to exactly one of three provenance families — a family names what must exist
before the number can be honest, which is what turns §39l-1's "what data backs the window" from one
blocking question into three answerable ones.** One quantity — the dispersion — is measured in
**two parts from two families** and frozen as one distribution by a committed merge rule; it is
the recorded exception, stated below rather than absorbed into the one-family claim.

**Family 1 — measured from refetchable venue REST history.** Both venues serve funding-rate history
and kline history over public REST (§38f's finding: funding is re-fetchable; the venue's published
funding/mark **is** the definition, per the venue-published-vs-locally-derived rule). ≥3 months —
years, in fact — is available today, negative regimes included. Quantities:

- **The dispersion distribution (C3) — the EWMA part.** The deployed estimator uses **two
  forecast sources** (the live per-settlement estimate for the imminent settlement; the trailing
  EWMA for later ones) and applies **one** dispersion shift to both, so an honest dispersion needs
  residuals from both sources. The EWMA part is family 1: the domain estimator's trailing forecast
  is run causally over the calibration era's settlement history and the residuals
  (realized − forecast) collected — causal trailing forecasts make every residual out-of-sample by
  construction. **Settled-rate history cannot back the imminent part** — §8's own line ("funding
  OHLC ≠ live estimate") is the same fact one step earlier — so the imminent residuals come from
  family 2 (below), and the frozen `Dispersion.Points` are the **pointwise more-adverse** of the
  two measured quantile sets — a committed merge rule, conservative at every quantile, requiring
  no change to the domain's one-distribution shape.
- **The `EntryDrift` margin** (§5.2's `entry_drift_pct` — the C3 adverse-drift margin the entry
  must clear, a required `MeasuredRate` the core already charges) — frozen from the dispersion
  distribution's adverse tail by a committed rule, with its own field, derivation identity and
  rationale in the artifact. It was absent from the first draft's roster while `StrategyParams`
  refuses to validate without it — the review round's catch; a roster the loader cannot satisfy is
  a loader that invents a number.
- **The exit band's width** — from measured regime persistence (§2.4's own words): the committed
  derivation rule reads calibration-era settlement excursions (dips below the entry-clearing level
  and whether they recovered within the dwell) and emits the width.
- **The adverse basis move** (the `BasisForecast` default's charge) — from venue-published mark and
  spot kline history: basis-proxy series at kline granularity, adverse-direction move over
  hold-length horizons, high quantile frozen. This is the **hold-scale** basis quantity — distinct
  from family 2's execution-scale adverse move, despite the colliding names (§41b-note below).
- **The expected-utilization figure** — **occupancy, not eligibility**: §2.4's utilization is
  expected time *in position*, and the first draft's proxy (the fraction of entry-clearing
  windows) measures a different thing — a few clearing windows can start multi-week holds, and
  many isolated ones can produce almost no occupancy, so the proxy is not even conservative
  (round 2's catch). The derivation **replays the actual `Enter`/`Continue` state machine** —
  step 3 built it as a pure function precisely so history can be driven through it — over the
  closed calibration sample, and freezes occupied duration ÷ deployable duration, rounded
  **down**. Round 3's catch: the calibration era has **no books**, and `Enter` demands an
  execution cost, so a naive "actual replay" either refuses every entry or invents book states.
  The replay therefore runs on a **pre-registered constant-cost surrogate**: execution cost held
  at the artifact's own frozen conservative numbers, evaluated at a **declared candidate
  quantity and declared stratum** — with the conservative direction argued in the rationale
  (holding historical costs at the frozen bound overstates them, which suppresses entries and
  understates occupancy — the direction that hardens the gate). The **exit band's
  entry-clearing level uses the same frozen definition** (same candidate quantity, same
  surrogate cost), or its derivation would move with an undeclared choice. Derivation order
  within the run: costs, dispersion and band freeze first; the occupancy replay consumes them.
  The
  scored run's *measured* utilization is step 5's output; this number is the pre-registered
  expectation §8 names, and §8's minimum-utilization threshold is what catches a scored run that
  idles far below it.

**Family 2 — measured from our captured books.** The archive (capture design §32) holds
time-synchronized spot and perp books from 2026-07-30 on — days, not months, and §41c's freeze rule
bounds which days derivation may read. Quantities:

- **The execution-scale adverse move** (`CostCalibration.AdverseMove`) — price move against the
  order side over the latency window, measured across archived book instants, adverse-side high
  quantile.
- **The p90 slippage table — a defined counterfactual observation, keyed to the leg it prices,
  stratified by book state as well as size.** §8's letter is *"p90 slippage conditional on order
  size **and book state**"*, and the step-3 seam's `CostBucket` carries only `MaxQty` — a
  quantity-only table pooled across book states understates the p90 exactly in the stressed state
  being scored, and the depth-walk branch does not repair it (the walk sees displayed depth;
  adverse selection is the depth that vanishes on contact, which is the p90 branch's whole job).
  Three commitments, each a round-2 catch:
  (i) **The observation is defined, because with no order path "measured slippage" is otherwise
  whatever the implementer meant**: classify the book at instant *t*, walk the candidate quantity
  through the book at *t + assumed latency* on the same side, and measure against the best price
  at *t* — the passive counterfactual for depth that vanished over the latency window. Sampling
  cadence, gap handling, insufficient-depth handling and archive-discontinuity treatment are all
  part of the committed procedure, and the artifact records the method per cell; real-fill
  recalibration arrives with milestone 3's order path, the same account-real-before-capital shape
  as the latency clause.
  (ii) **The cell key names the population**: (venue, product, side, book-state stratum, quantity
  bucket) — the current adapter applies one quantity-keyed p90 to **both legs**, and a pooled
  table is dominated by the liquid population precisely where the illiquid leg needs its own
  answer.
  (ii-b) **The stratum classifier is itself frozen in the artifact, not named and left to the
  implementer** (round 3's catch: "classify the book at *t*" without frozen features, cut
  points, boundary inclusivity, precedence and an out-of-range refusal lets two conforming
  implementations put the same book in different strata and draw different charges from the same
  approved artifact). The classifier's *code* is domain code under the procedure commit; its
  *parameters* live in the hashed artifact; calibration and runtime use the same classifier, and
  the runtime lookup keys on (instrument, side, **classified** stratum, quantity bucket) with a
  book the classifier cannot place being a refusal, never a nearest-cell guess.
  (iii) **A cell below a declared minimum evidence count is absent, and absent is a refusal** —
  never a thin-sample number with a rationale attached.
  The `CostCalibration`/`CostBucket` schema widens accordingly in step 4's slice A: a deliberate,
  recorded extension of a seam that was built expecting this artifact, driven by §8's letter —
  not a re-derivation of step 3's structure.
- **The dispersion's imminent part** — residuals of the **live per-settlement estimate** (the
  `markPrice@1s` stream's funding estimate, which the archive carries from day one) against the
  realized settlement, collected over the capture window; merged with family 1's EWMA residuals
  by the §41b merge rule above.
- **Transport-latency statistics** — not frozen directly, but measured (RecvTime vs SourceTime
  across the archive) to *inform* family 3's latency declaration.

A short measurement window is the honest current state, and the family answer is versioning, not
waiting: the v1 artifact freezes what the archive can support **with the window's shortness stated
in the rationale**, and a longer-windowed v2 (more capture, or a purchased-history backfill) is a
new version that §8's own rule prices correctly — a complete rerun.

**Family 3 — declared with rationale, not measured.** Each is a scoring-protocol choice or a
quantity milestone 2 structurally cannot measure; the artifact carries the value, the rationale,
and what informed it:

- **The decision-to-arrival latency** — no OMS exists until milestone 3, so no order was ever
  sent, and §8's letter says **measured** decision-to-arrival latency. Declaring it is therefore a
  **deviation, not a reading**, and it takes the §40d modeled-tier shape: a **pre-registered
  `[SPEC EDIT OWED]` marker**, taken at the schema in slice A and landed at slice C's close as a
  §8 clause authorizing a **declared upper-bound assumption for milestone 2's `$1K` gate** —
  informed by family 2's transport statistics with a stated multiplier for the unmeasured order
  path — **with a measured order-path latency required before capital** (milestone 3's OMS is
  what makes it measurable; the testnet integration §9 already schedules is the probe). The
  schema names the field an *assumed bound*, never "measured at" — and `CostCalibration.Latency`'s
  comment currently says "measured at", so slice A owes that comment the same correction. The
  driven adapter already refuses a longer-than-calibrated latency at decision time, which is what
  keeps the declaration honest later: a live system slower than the assumption refuses rather
  than flatters.
- **The rescue-margin figure** — §40k-4's interim number, rationale owed, real derivation
  milestone 4.
- **The rolling-window length + the regime segmentation rule** — what classifies a sub-window as
  negative; a written rule, applied mechanically by step 5.
- **The standby benchmark** — §8's "idle/deployable capital, not the hurdle"; the declared figure
  the negative-regime criterion compares against.
- **The max-drawdown ceiling** and **the confidence level + method** for the LCB — declared,
  with rationale.
- **The minimum-utilization threshold** — §8 makes it part of pass eligibility (a rolling window
  below it is not held to the hurdle), and it is a **distinct number from family 1's
  expected-utilization figure**: expected utilization is the pre-registered forecast the
  utilization-adjusted LCB uses; the minimum is the eligibility cutoff. Both are frozen; the
  artifact states the mechanical relation it chooses between them (the cutoff as a declared
  fraction of the expectation, or an independent figure — either way written, never implied), so
  neither the loader nor the runner ever synthesizes a verdict-affecting number outside the
  one-artifact governance.

**This record deliberately names no values.** A number written here would be a threshold chosen
outside the governance — the values exist only in the approved artifact (§41e), and committed
numbers outside it live only in `fixture:`-marked test helpers (§40e, unchanged).

**§41b-note — two "adverse moves", named apart.** The execution-scale move (family 2, seconds,
prices a fill) and the hold-scale basis move (family 1, days, prices a position) share an adjective
and nothing else. The schema names them `adverse_fill_move` and `adverse_basis_move`; a reviewer
finding one where the other belongs is finding a real defect.

**Rejected: one family (buy Tardis, measure everything from months of books).** It is the
maximal-fidelity answer and it makes the artifact hostage to a purchase decision plus an ingest
project, while family 1's quantities never needed books at all. The purchase stays live as the
step-5 fork (§41h) and as the v2 upgrade path — rejected as a *precondition*, not as an option.

**Rejected: declare everything (family 3 for all).** Fastest to freeze and it abandons §8's
"mechanically-derived thresholds are the default" for the quantities where measurement is free
(family 1) or already captured (family 2). Declaration is reserved for quantities where measurement
is structurally impossible this milestone, and each such declaration must say so in its rationale.

### §41c — the dataset freeze and the holdout split, declared before looking

**Decision: two mechanical disjointness rules, one per data direction, both enforced by refusals in
the derivation tool — declared here, before any dataset is fetched or read.** §31a's constraint is
that the calibration/holdout split precede looking at either; a split chosen after inspection is
the tuning door §8's governance exists to close.

**Rule H1 — fetched history splits at a declared boundary.** The derivation tool **refuses any
historical input dated on or after 2026-02-01T00:00:00Z**. Everything before is calibration era;
everything on/after is holdout era, reserved for scoring. The boundary is chosen by calendar
arithmetic, not by inspection: it leaves the most recent ~6 months as the promotion sample —
recent-regime scoring is the honest promotion test — and years of history behind it for
calibration. Whether the holdout era contains the negative regime §8 requires is **discovered, not
chosen**; if it turns out not to, the response is a **hard refusal — the gate cannot report a
verdict at all**, because §8's negative-regime requirement is unconditional and a "stated
limitation" would be a nonconforming green gate wearing a footnote (round 2's catch: the
full-sample LCB could pass while the strategy's principal failure regime — stand-down correctness
and negative-regime drawdown — was never scored, the exact aggregate-metric masking §8 was
written to prevent). The permitted resolutions are **wait** (a negative regime occurs in the
holdout era), **extend the sample by purchase** (still entirely post-boundary), or an **explicit
marked §8 edit** followed by a new artifact version and a complete rerun — never prose.
**Extending scoring backward into the calibration era is forbidden.**

**Rule H2 — forward capture splits at the freeze instant.** Derivation reads only archive days
whose manifest entries exist when the derivation runs; the artifact **enumerates the manifest
digests it read** in its dataset block; step 5's runner **refuses to score any window whose day
appears in that enumeration**. Freeze-then-score gives disjointness by construction — every scored
forward byte postdates the artifact.

**Rule H3 — both samples are closed identities, not eligibility floors.** H1 and H2 alone leave a
selection door open on each side, and the review round's second blocker named both. Calibration
side: an input set defined only by "nothing after the boundary" lets derivation *omit* adverse
calibration rows and still verify — so the artifact's dataset block declares the calibration
identity **closed**: venue, symbol, series, start **and end** instants, and a completeness rule
(every settlement/kline the venue serves inside the range, pagination exhausted; the tool refuses
a dataset with a missing **or extra** row, naming the gap). Promotion side: an operator free to
choose *which* eligible holdout window, venue, or fork gets scored after seeing the artifact can
score until one passes — so the promotion identity is **frozen inside the hashed artifact
itself, closed on every axis**: venue, data source (§41h's fork), instruments, sample start, a
**deterministic end rule** (for forward data: the first complete three-month interval after the
freeze — a closed rule, never an operator-declared run date, which permits outcome-dependent
stopping), and the missing-data policy. Round 2's catch on the first H3: recording the fork
"before holdout bytes are read" still let three verdict-affecting choices (venue, fork, sample
end) happen *after the thresholds were known* — a selection made after approval is a tuning door
whatever it is keyed on. Round 3 then caught the repair's own gap: "before the artifact freezes"
alone permits **inspecting the public holdout-era history across candidate identities and then
choosing the favorable one** — leakage at the population-selection layer, which no deterministic
end rule repairs afterward. So the ordering constraint is **both conditions, not either**:
§41h's fork and §40k-3's venue are sealed — dated, in this record — **before any holdout-era
byte is fetched or read by our tooling AND before any threshold derivation runs**; the fetchers
consume the sealed identity (a holdout-era fetch before the seal is a refusal); and the seal
carries an **honesty declaration**, because holdout-era funding is public data a solo operator
cannot un-see: if holdout-era history was examined across candidate identities before sealing,
the seal must say so and a fresh prospective holdout must be declared in its place. The scored
sample is **everything eligible** under the sealed identity, never a chosen subset; any
different fork, venue, or boundary afterward is a v2 plus a complete rerun, §8's own price.

**The dataset block is the identifier set §8 names.** For family 1: fetched history is written once
to committed dataset files — the venue's words, verbatim, dated, one file per venue per series —
under `datasets/` (top-level, sibling of `configs/`; funding and kline history at settlement/kline
granularity is small enough to commit, and committing it is what makes the derivation reproducible
byte-for-byte). Each file's SHA-256 is enumerated in the artifact. For family 2: book data is never
committed (that is the archive's job); the manifest digests are the identifiers, per rule H2.

**Rejected: split by the freeze instant alone (no H1).** Clean for forward capture, meaningless for
fetched history — all history predates any freeze, so derivation could read the very windows
scoring will replay, and the negative-regime sub-windows §8 scores would be windows the band width
and dispersion were fitted on.

**Rejected: choose the holdout after fetching, placing it around a verified negative regime.**
Guarantees §8's regime requirement is satisfiable — by inspecting the data before declaring the
split, which is the exact move pre-registration forbids. The refusal-and-user-decision path costs
calendar honesty instead of governance.

### §41d — the artifact's form: one committed file, append-only versions, hash over exact bytes

**Decision: the artifact is one committed JSON document, `configs/calibration.btcusdt.v1.json`,
with `CalibrationRef.ID` = the file's basename (`calibration.btcusdt.v1`) and `CalibrationRef.Hash`
= lowercase-hex SHA-256 over the file's exact bytes. Versions are append-only: any change is a new
file `v2`, never an edit — §8: any change forces a new version and a complete rerun.**

The schema is **context-owned, declared beside its loader** in `strategy/infrastructure/driven`
(slice A's rule: the kernel owns mechanisms, never a context's schema). Numbers ride
**mantissa-as-string at declared scale** — `fixedpoint` has no JSON face (§40f), and the wire DTO
battery already proved the encoding. The document carries, per §8's list: the schema version; the
quantity values with **per-quantity provenance** (family, dataset citation by digest, derivation
procedure identity, rationale — family 3's rationale states why the quantity is declared rather
than measured); the **dataset block** (§41c's enumerations); and the **procedure identity** (§41e). The **approval
record is deliberately NOT inside the file**: the approval names the content hash (§41e), so an
in-file approval block would either sit inside its own preimage or force a re-hash after the
24-hour delay — the frozen bytes and the approval of those bytes are two facts with two homes.
§8's "carrying … an approval record" is satisfied by composition: the artifact identity (ID +
hash) is what the attestation (§41e) approves, and the pair is complete only with both.

Hashing the exact bytes rather than a canonicalized parse is deliberate: the file is
**write-once**, so canonicalization buys nothing and adds a codec between the digest and the bytes
— the same argument that kept compression out of the archive (§38g). The hash lives **outside the
file** (in the attestation §41e defines, in every journaled `Decision`'s `CalibrationRef`), so
there is no self-reference to canonicalize around.

**Rejected: store the artifact in the reference-revision repository (§40k-2).** Both are dated,
hashed, append-only facts, and merging them is one mechanism instead of two. Rejected because their
authorities differ — a fee revision records what a venue said and updates at the venue's cadence; a
calibration version records what the governance approved and updates only through a rerun — and
§40e's one-artifact rule needs the artifact to be **one** identity, not a bundle of per-quantity
revisions whose mixture the rule exists to refuse.

**Rejected: per-quantity files (one hash each).** It would let a scored run cite five hashes, which
is exactly the mixed-provenance verdict §40e forbids; one file makes the one-artifact rule the
natural reading instead of a cross-file constraint.

### §41e — governance: pre-registered procedure, mechanical derivation, solo-honest approval

**Decision: the derivation *procedure* is pre-registered the same way the numbers are — the
`calibrate` tool and its committed parameters land in a commit whose hash is recorded inside the
artifact, before the tool runs against the calibration set — and the recorded commit is bound to
the code that actually executed, not merely named.** Number-tuning has an obvious door (edit the
number) and the governance closes it; **rule-tuning** (edit the derivation until the number comes
out right) is the same door one level up. A commit *string* alone does not close it — a
modified-working-tree build can emit thresholds while naming the last clean commit (round 2's
catch) — so the binding is mechanical: the tool reads its own build info (the stdlib's embedded
VCS revision and dirty flag — standard library only, `kernelpurity`-compatible), **refuses to run
from a dirty tree or when the embedded revision disagrees with the commit it is about to
record**, and writes both the revision and its own **executable digest** into the artifact; the
attestation binds the same executable digest. A changed procedure is then a visibly different
artifact, which is a new version, which is a complete rerun.

**Approval, solo-honest (§8's own default), machine-verifiable:** mechanically-derived thresholds
are the governance — a named independent authority only if the operation is staffed. The approval
is a **committed detached attestation file**, `configs/calibration.btcusdt.v1.approval.json`,
carrying the artifact ID + content hash, the procedure commit, the dataset ledger (files read,
digests, refusals hit), the approval instant, and the sign-off — created only after §9's
solo-honest posture: a **time-delayed self-sign-off** (declared here as 24 hours between
derivation and approval — long enough to reread cold, short enough not to idle the milestone)
plus an **out-of-band confirmation**, both named in the file. **Step 5's runner consumes the
attestation, not a naked expected hash**: it verifies the attestation binds to the artifact it
loaded (ID + recomputed hash match) and refuses to score without one — because a runner keyed on
a caller-supplied hash makes an unapproved artifact mechanically indistinguishable from an
approved one (the operator hashes anything and pins that hash; every check passes; every
journaled `Decision` cites the pinned hash consistently — internally tamper-evident evidence that
proves only that *an* artifact was used). The first draft kept approval as prose in this record
and was rightly NO-SHIPped for it: prose the runner never reads is governance the run cannot
enforce. A dated entry in this record still accompanies the attestation — the record is where
reasons live — but the attestation is what the machinery consumes. **The honest solo limit,
stated:** a solo operator can forge any file; what the attestation buys is that approval is a
dated, committed, git-historied fact the runner refuses without — tamper-*evident* under the same
version control as everything else, which is the strongest property a solo posture offers and
exactly the one §8's "mechanically-derived by default" acknowledges. Two hardenings from round 2:
the attestation is **write-once like the artifact** — replacing its approval instant, sign-off or
confirmation claim while the artifact hash stands still would leave every journaled
`CalibrationRef` pointing at the same two fields, so an approval-record replacement is a **new
artifact version**, never an edit; and the **runner records the attestation's own SHA-256 in the
scored run's evidence**, so the run identity names *which* approval authorized it, not merely
which numbers it used.

### §41f — the derivation tooling: `arblab calibrate`, placed where fetch-writes-a-revision lives

**Decision: the derivation splits along the context's existing shape — pure derivation math in
`strategy/domain` (a `calibrate` subpackage: residual collection, excursion statistics, the
depth-walk measurement, the merge rule — beside the estimator whose residuals it collects), and
the `calibrate` CLI command as a driving adapter in `strategy/infrastructure/driving`, which
reads the datasets and the archive, runs the domain math, and writes the artifact.** The §40d
pattern (`fetch-writes-a-revision`) one level up: *derive-writes-an-artifact*. The first draft
placed the whole thing at `internal/strategy/infrastructure/calibrate`, which **violates §3.6
rule 3's own letter** — `infrastructure/` contains only `driven/` and `driving/`, a
permitted-children rule precisely so the adapter *direction* stays visible in the path — the
review round's catch, and the fix is the shape the rule wanted anyway: a CLI that drives the
context is a driving adapter. It is context work, not engine machinery: it consumes
`strategy/domain`'s own estimator (the dispersion residuals **are** the deployed model's
residuals — measuring a different model's residuals would calibrate a stranger), and it emits the
context's schema. Under `internal/`, so `nofloat` polices every computation; all quantile,
excursion and depth-walk arithmetic is `fixedpoint`, and each new derivation computation gets an
**independent oracle** per the standing rule (slice B's `math/big` oracle pattern; the depth-walk
measurement can additionally be cross-checked against the driven adapter's walk on shared
fixtures — two implementations, one claim).

The tool's honesty features are refusals, same as everywhere: refuse a historical input on/after
H1's boundary **naming the record's date**; refuse an archive day absent from the manifest; refuse
to overwrite an existing artifact version; refuse to emit if any family-1 dataset file's digest
disagrees with a re-hash; refuse to run from a **dirty tree** or when the **embedded build
revision disagrees** with the commit about to be recorded (§41e's execution binding). Determinism
is a tested claim: two runs over the same inputs emit byte-identical artifacts (no wall-clock in
the output path — the freeze date is an argument, per the injected-time rule).

**Rejected: place derivation in `engine/` beside capture.** Capture is context-free machinery;
derivation consumes a context's domain model and produces a context's schema. Putting it in
`engine/` would also put a `strategy/domain` import into `engine/`, inverting the direction every
other edge in the repo respects.

**Rejected: derive in an external notebook/script and commit only the output.** The procedure
would be unregisterable (no commit hash inside the repo's gates), `nofloat` could not police it,
and reproducibility would rest on an environment no test pins. The whole point of §41e dies there.

### §41g — the runtime loader: bytes → class-2 values, refusing everything the schema promises

**Decision: a driven-side loader (`strategy/infrastructure/driven`) reads the committed artifact
file and constructs the class-2 values step 3 defined — `Dispersion`, `CostCalibration`, the
`MeasuredRate`/`MeasuredMoney` fields — each carrying `CalibrationRef{ID, Hash}` with the hash
recomputed from the file's bytes at load.** Refusals by name: hash mismatch against the caller's
pinned expectation; a `fixture:`-prefixed ID arriving through the production loader (the marker is
for test helpers; a committed artifact wearing it is an impersonation); a missing quantity (the
zero value is unusable, as everywhere); a mantissa that does not parse at its declared scale.
Validation is the types' own `Validate()` — the loader adds no second authority over rules the
domain already states.

### §41h — §39l-1: the step-4 half answered, the step-5 half sharpened

**Answered here (step 4's half): the artifact's own data needs require no purchase.** Family 1 is
free REST history, family 2 is our archive, family 3 is declared. The v1 artifact can freeze on
data that exists or is fetchable today, under §41c's split.

**Sharpened for step 5 (not pre-answered): what the scored ≥3-month window replays.** The honest
fork, stated with its costs:

1. **Wait** — score forward capture only; ≥3 months lands late October, negative regime not
   guaranteed by any calendar. Cost: calendar; the §38f clock keeps running.
2. **Buy** — Tardis books for the holdout era; full §8 letter (time-synchronized books across the
   sample), historical negative regimes included. Cost: money + an ingest path for vendor format.
3. **Hybrid** — historical funding (free, holdout-era) backs the negative-regime sub-windows,
   where the correct behavior is stand-down and the criterion is bounded loss vs standby (§8's own
   criterion — a standing-down strategy fills nothing, so book fidelity is doing no work there);
   forward captured books back the fill-bearing positive windows. Cost: **a §8 reading** — the
   letter says books across the sample; the hybrid reads books-where-fills-occur. If step 5 takes
   it, it is a stated reading confirmed by that close's reading pass or a marked §8 edit — never
   silent.

Which fork is the user's decision, money and calendar both being theirs to spend.

**SEALED — 2026-07-31, session 49, by the user, in this design session:** the fork is **hybrid**
(option 3) and the scored gate's venue is **Binance** (§40k-3 closed — chosen on archive
cleanliness: the known absent windows and reconnect churn sit on the Bybit spot book, and
Binance's `markPrice@1s` stream is what the imminent-residual derivation reads). The seal
predates any holdout-era fetch, any derivation run, and every slice of this step — the earliest
instant H3's both-conditions rule admits. **Honesty declaration, per H3:** holdout-era history
has not been systematically examined across candidate identities; both choices were taken on
structural grounds (cost/calendar for the fork, archive cleanliness for the venue) stated before
any holdout byte was fetched by any tooling of ours. The hybrid's pre-registered duty stands:
its books-where-fills-occur reading of §8 is confirmed by a close's reading pass or lands as a
marked §8 edit at step 5 — never silent.

### §41i — verification plan: the checklist suspects, named before the fixtures exist

The standing rhythm (checklist **before** battery, aimed mutations, oracles, positive controls)
applies to every slice. Named in advance, per the nine-instance lesson — the fixture parameter
pinned to the value that collapses the distinction:

- A residual fixture **symmetric about zero** — adverse-side selection and sign conventions
  collapse; make the distribution skewed.
- A **single-bucket** p90 table — bucket selection, ordering, and the boundary rule are all
  invisible; and no fixture quantity exactly **at** a bucket's `MaxQty` (inclusive-vs-exclusive
  collapses).
- **No input dated exactly at H1's boundary** — `<` and `<=` are the same predicate on strictly
  inside/outside fixtures; the boundary instant must appear and be refused.
- An excursion fixture whose dips **all** exceed the dwell — the recovered-within-dwell branch is
  never taken; both branches need instances.
- A utilization fixture where the clearing fraction divides **exactly** — round-down and round-up
  agree; make it inexact.
- An artifact fixture in which **every quantity cites the same dataset digest** — per-quantity
  citation is untestable; use distinct digests.
- A hash fixture already **lowercase** end-to-end — the case rule is invisible; one uppercase
  input must appear.
- The determinism test run **twice in the same process** — hidden ordering from map iteration can
  agree within one process and differ across runs; the claim needs two invocations of the binary,
  or an explicit ordering assertion on every emitted collection.
- An attestation fixture that binds **correctly in every case** — the runner's refusal needs a
  fixture whose attestation names a *different* hash than the artifact it sits beside, and one
  with no attestation at all; a suite of only-valid attestations tests the parse, not the binding.
- A p90 fixture with a **single stratum** — stratum selection collapses to the quantity-only
  table the strata exist to replace; and no observation count exactly **at** the minimum-evidence
  threshold (at-vs-above collapses).
- A completeness fixture whose gap is **only a missing row** — the "or extra" half of H3's
  refusal needs a dataset with an inserted row the venue never served.
- An occupancy fixture where **eligibility and occupancy agree** (every clearing window's hold
  ends before the next window) — the replay-vs-proxy distinction collapses; the fixture needs one
  short clearing window that starts a hold spanning many non-clearing ones.
- A p90 fixture where **both legs' books are identical** — the per-(venue, product, side) keying
  is invisible; the legs must differ enough that pooling would move the answer.
- A counterfactual fixture whose book is **unchanged between *t* and *t + latency*** — the
  walk-at-*t*+λ observation collapses into the plain depth walk; the fixture needs depth that
  vanishes inside the window.
- An execution-binding fixture that only exercises the **clean** tree — the dirty-tree and
  revision-mismatch refusals need their own failing instances, and the positive control beside
  them.

### §41j — build order: three slices, the standing rhythm

- **Slice A — the doors.** Schema + loader (§41d/§41g) including the **`CostBucket` book-state
  strata widening and the `EntryDrift` field** (§41b), hash discipline, the attestation schema +
  the runner-side binding check's contract (§41e), every refusal named; the H1/H2/H3 refusal
  frame in the driving adapter's skeleton; `datasets/` layout; **the §8 latency marker taken**
  (`[SPEC EDIT OWED]` at the schema field, §41b family 3) and `CostCalibration.Latency`'s
  "measured at" comment corrected. The gates already police everything here; no new deferral is
  created (the deferral rule's construct check: every analyzer input this slice creates —
  imports, enums, floats — is a kind the six gates already read).
- **Slice B — family 1.** The history fetchers (venue's words → committed dataset files, dated,
  digested, H3-complete), EWMA-residual/band/basis/utilization/`EntryDrift` derivations + their
  oracles.
- **Slice C — family 2 + freeze + close.** **First act: verify the §41h seal stands** (the fork
  and venue were sealed in this design session — slice C re-checks that no holdout-era fetch
  predates the seal and that the sealed identity is what the artifact enumerates). Then the
  book-derived measurements over the archive (stratified per-leg p90 cells with observation
  counts and method identities, adverse fill move, imminent residuals + the merge rule), the
  transport-latency statistics, the family-3 declarations with rationale, the derivation run from
  a clean checkout (§41e's execution binding), the 24-hour approval delay, the attestation + the
  dated approval entry in this record, and the close: reading pass, **the §8 latency clause
  landed** (this record's one expected core edit — everything else §8 already states; the reading
  pass holds the veto per the standing lesson), marker grep back to zero, doc-set bump.

### §41k — open questions passed on, sharpened not pre-answered

1. ~~The step-5 window fork~~ — **SEALED 2026-07-31 (§41h): hybrid.** What passes to step 5 is
   the fork's pre-registered §8-reading duty, not the choice.
2. ~~Which venue hosts the scored gate~~ — **SEALED 2026-07-31 (§41h): Binance.** §40k-3 is
   closed; both venues' datasets and fee rows remain committed either way (the calibration side
   still reads both venues' history — the seal fixes the *scored* population, not the measured
   one).
3. **The reference-revision repository + replay applicator** — §40k-2 unchanged, step 5.
4. **The rescue margin's real derivation** — §7.1's milestone-4 protocol; v1 freezes the interim
   figure with rationale (§41b family 3).

### §41l — deliberate absences

- **No values in this record** (§41b) — a number here would be a threshold chosen outside the
  governance.
- **No recalibration schedule.** Recalibration is a new version + rerun by §8's rule; a schedule
  is a decision for after the kill gate, over data whose value is then known (§38g's shape).
- **No artifact hot-reload.** A calibration change mid-run is a different run by definition;
  restart-required is §5.2's default direction and it is correct here.
- **No committed book data.** The archive owns book bytes; the artifact cites manifest digests
  (§41c H2).
- **No independent approval authority.** §8's solo-honest default, revisited only if the
  operation is staffed.
- **No `$10K+` model numbers.** The full adverse-selection model is milestone 5's; freezing
  placeholder numbers for an unbuilt model would be family 3 without a consumer.

### §41m — slice A build record (session 50, 2026-08-01)

**Slice A is BUILT: the doors exist and refuse.** Four commits (`5281dfa`, `70a9529`, `9333f7a`,
`4642879`), landing:

- **`strategy/domain/stratum.go`** — the frozen classifier (§41b-ii-b): explicit three-member
  `BoundKind` (a forgotten interval end REFUSES rather than silently meaning unbounded — the
  default-deny direction applied to the classifier's own parameters), ordered rules with declared
  first-match precedence, `ErrStratumUnclassifiable` for a book no stratum admits. Features are
  `(relative spread, side depth)`; the one shared extractor is `driven.BookFeaturesFor` (runtime
  pricing and the slice-C derivation both call it — two extractors would put one book in two
  strata).
- **`driven/cost.go` widened** — `CostCell` keyed `(instrument, side, stratum, MaxQty)` with
  `Observations` + `Method`; `MinEvidence` (a thin cell is a construction refusal, §41b-ii-iii);
  per-group strictly-ascending buckets (cross-group interleave legal — the positive control a
  globally-comparing mutant refuses); the lookup classifies each side's book and refuses an
  uncalibrated population (`ErrStratumUncalibrated`) — never nearest-cell. **The §8 latency
  marker is TAKEN** at `CostCalibration.Latency` (comment corrected from "measured at" to
  assumed-bound); marker grep now expects **EXACTLY ONE** until slice C's close.
- **`driven/calibration.go`** — the artifact schema + loader (§41d/§41g): hash = lowercase-hex
  SHA-256 over exact bytes, recomputed at load; §41b's family assignment ENFORCED per quantity
  (measured-claiming-declared and declared-wearing-a-derivation both refuse — the roster as
  machinery); `EntryDrift` present (the round-1 catch honored); closed dataset identities; the
  promotion identity typed and fork-checked; `fixture:` IDs refused through the production door.
- **`driven/attestation.go`** — the detached attestation (§41e): binds ID + content hash +
  procedure commit + executable digest; `ApprovalDelay` 24h **inclusive at the boundary** (the
  golden fixture sits exactly AT it); own SHA-256 exposed for run evidence.
- **`driving/calibrate.go`** — the refusal frame: H1 (`GuardCalibrationInput`, boundary instant
  itself refused), H2 (`GuardArchiveDay`), H3 (`CheckComplete` — missing AND extra, including
  count-preserving substitutions in BOTH directions), the §41h seal guard (`GuardHoldoutFetch`:
  fetch-before-seal + unsealed-identity), execution binding (`CheckExecutionBinding` pure +
  `ReadBuildRevision` thin), `GuardNewVersion`, `CheckDatasetDigest`. `Calibrator.Run` refuses
  `ErrDerivationUnbuilt` — the step-3 door pattern.
- **`datasets/README.md`** — the family-1 layout, rules restated with their §41c anchors.

**Verification ledger.** Checklist BEFORE battery, per the standing rule — the §41i suspects
applicable to slice A are all embodied in fixtures (at-bound candidates, two strata, per-leg
differing cells, at-MinEvidence observations, uppercase digests, wrong-hash + absent
attestations, H1 boundary instant, dirty-tree + clean control). One live catch during the build:
the first uppercase-digest fixture used seed `0x14`, which hexes to DIGITS ONLY — `ToUpper` was
the identity and the case rule untestable (the tenth-instance hex-case collapse, caught by the
failing test, fixed to `0xab`). **19 aimed mutations: 18 caught on first aim, one NOBUILD
re-aimed (caught), one SURVIVED diagnosed into a real fixture gap** — `CheckComplete`'s
positional extra-row check was masked by the count backstop (an inserted row also changes the
count), so a count-preserving SUBSTITUTION validated clean under the mutant; the fixture was
added and its MIRROR (late-shifted rows masking the gap arm) immediately after, both killers
now. The spread ratio's away-from-zero rounding is pinned by a boundary-straddling book whose
stratum — and printed charge — flips with the direction. `make ci` green on a COLD cache, all
six gates clean.

**Deliberately not done:** no committed artifact or attestation file (values exist only after
slice C's governed derivation run); no CLI wiring of `calibrate` into `cmd/arblab` (lands with a
runnable derivation, slice B/C); no fetchers (slice B).

### §41n — slice B build record (session 51, 2026-08-01)

**Slice B is BUILT: the family-1 derivations exist with oracles, the fetchers exist with real
committed datasets, and the CLI doors are wired.** Five commits (`d9c41ef`, `821d947`, `c0970d7`,
`4ae7d17`, `51f6adf`), landing:

- **`strategy/domain/calibrate`** (new package, §41f's pure half): `Residuals` (causal trailing-EWMA
  residuals; warmup = the EWMA's own saturation horizon via the new `domain.EWMAHorizon`, so the
  rule froze no extra number), `Points` (outward-from-median order statistics matching `ShiftAt`'s
  read-time rule — one convention on both sides of the artifact), `EntryDriftFrom` (annualized
  adverse-tail magnitude via the now-exported `cashcarry.Annualized` — one annualization base, both
  consumers), `ExitBandWidth` (dips strictly below clearing, at-level ends a dip, recovered =
  shorter than the dwell, truncated runs are evidence of nothing, upper-statistic depth,
  no-recovered-dip refusal), `AdverseBasisMove` (relative rise of mark−spot over spot per §41b-note,
  upper statistic, negative tail freezes as a DECLARED zero mirroring `NewConservativeBasis`),
  `Surrogate` (§41b's constant-cost frame, ONE type feeding both), `ClearingLevel` (exact mantissa
  bisection against the DEPLOYED `Enter` in a flat-rate world; cap-never-clears refuses,
  floor-clears answers the floor), `Occupancy` (the actual `Enter`/`Continue` replay: same-instant
  re-entry after a close, era truncation stated, funding-floor refusals are idle time, every OTHER
  refusal is `ErrReplayRefused` — a broken frame must not measure as an idle era; occupied over
  deployable, `RoundFloor`).
- **The `adverse_basis_move` schema correction**: `MoneyQuantityDoc` → `RateQuantityDoc`. Slice A
  froze a Money the runtime could not consume — `NewConservativeBasis` charges rate × notional, so
  the Money had no consumer without an unstated price to divide by — and a relative move is the only
  measurement that transfers across an era in which the underlying tripled. Caught at the seam's
  first consumer; no committed artifact existed, so no version was orphaned.
- **`driving` datasets + fetchers**: the verbatim-pages `DatasetFile` (the digest the artifact cites
  is a digest over the venue's words), venue-dialect parsers, grid normalization with
  `GridJitterTolerance` (1 min, inclusive — Binance fundingTime jitter observed live:
  `1672560000008`), `Fetcher` behind an injected transport seam (cursor dialect and window dialect,
  stall refusal, H1 refused before one byte moves), write-once files, `calibrate fetch`/`calibrate
  run` in `cmd/arblab`. Bybit klines refuse by name (`ErrSeriesUnsupported`) — no derivation
  consumes them this milestone.
- **Four committed datasets** (fetched 2026-08-01): both venues' funding 2023-01-01→2026-01-31
  (3381 rows, complete 8h grids) and Binance spot+mark 1h klines 2023-04-01→2026-01-31 (24 888 rows
  each). **The kline era starts 2023-04-01 because H3 fired live**: Binance served no spot kline for
  2023-03-24T13:00Z (that day's spot halt), and a closed identity cannot claim an instant the venue
  never served. Digests/counts/eras are PINNED in `committed_test.go`, so write-once is a build
  break. Venue-profiles gains the dated endpoint-observation section.

**Stated readings for slice C's reading-pass docket** (each also recorded at its code site):
(i) residuals grade the UNCLAMPED trailing forecast — the refetchable history carries no cap/floor
series, and unclamped residuals are never smaller in magnitude than deployed ones (conservative);
(ii) the surrogate frame's prices are FIXED — price level enters only through notional-vs-capital
scaling, and a kline-driven frame would make the clearing level a curve no single number honestly
summarizes; stated limitation, not absorbed; (iii) a close falls through to a same-instant re-entry,
which is how `ProposeHoldPlan`'s minimum-hold reading composes into long occupancies.

**Verification ledger.** Checklist BEFORE battery found **three collapses in this slice's own new
fixtures** (14th–16th instances): every `q=0.5` fixture sat at an EXACT rank, collapsing the
at-median rule (`>=` vs `>`) — fixed with a four-element set putting the median at rank 1.5; every
jitter fixture drifted LATE, collapsing round-to-nearest into truncate-toward-floor — fixed with a
row 8 ms BEFORE its grid point; and the dwell RESET arm only ever ran at count zero, where resetting
and not resetting are the same state — fixed with a dip→recovery→dip series under a six-settlement
plan. **25 aimed mutations: 21 caught on first aim, 3 NOBUILD re-aims and 1 NO-OP re-aim (all
caught on re-aim), ZERO survived** — and each of the three checklist fixtures was the sole or
decisive killer for its mutant, so the checklist paid before the battery ran, again. The
`exhaustiveswitch` gate fired live on this slice's first draft (two verdict switches with
`default:` arms) — the §3.6 rule-10 machinery catching new code the session it was written. The
occupancy battery's every count, duration and digit string was hand-derived before the first run
and matched unmodified. `make ci` green on a COLD cache, all six gates clean; marker grep still
EXACTLY ONE (§8's latency marker, lands at slice C's close).

**Deliberately not done:** no derivation RUN (slice C's governed act, from a clean checkout, with
the §41e binding); no family-2 measurements, no family-3 declarations, no merge rule (slice C); no
artifact or attestation file; `Calibrator.Run` still refuses (`calibrate fetch` is real, `calibrate
run` is the honest door).

### §41o — slice C build record, part 1 of 2 (session 52, 2026-08-01 — MID-SLICE, machinery built, governed act pending)

**The slice C machinery is BUILT and verified; the governed derivation run, the 24h delay, the
attestation and the close are deliberately NOT done** — the derivation must run from a clean
checkout of a commit that contains all of this, and the 24-hour delay forces the session split
anyway. Four commits (`c4bcd82`, `1454b36`, `c906f4e`, + the battery commit), landing:

- **First act, per §41j: the seal was verified standing.** Seal commit `a5b5d33`
  (2026-07-31T19:37Z) predates every dataset fetch (`fetched_at` 2026-08-01T05:50Z); all four
  committed datasets end 2026-01-31 (strictly inside H1's calibration era); no holdout-era file
  exists anywhere; `PromotionIdentityDoc` + the driving seal constants are what the emitter
  enumerates.
- **`engine/venue` book decoders** (`wire_binance_book.go`): the spot `depth20` dialect (bare
  `lastUpdateId/bids/asks`, NO venue timestamp — the consumer's anchor is RecvTime, stated) and
  the futures dialect (a `depthUpdate` event around the same twenty levels, event time `E`,
  position `u`), each refusing the other's bytes as FOREIGN, ordering copied never repaired.
  **Finding worth carrying (the battery's one survivor, diagnosed):** encoding/json matches tags
  case-INSENSITIVELY as a fallback, and the futures frame carries both `"U"` and `"u"` — so with
  only `u` declared, the u-vs-U claim was enforced by the venue's KEY ORDER (last case-insensitive
  match wins), not by the type. Fixed by declaring `U` its own exact-match field (read by nobody,
  documented as routing); a key-order-independence fixture now kills both tag mutants.
- **`strategy/domain/calibrate` family-2 math** (`slippage.go`): `CounterfactualSlippage`
  (§41b-ii-i verbatim — walk the candidate through the book at t+latency, measure against best at
  t, adverse-positive both sides, negative samples KEPT, thin later book = `ErrDepthShort` never a
  zero), `AdverseFillMove` (execution-scale, signed adverse-positive), `UpperQuantile` (ceil
  selection, `ceilIndex` shared), `MergeAdverse` (§41b's committed merge: pointwise more-adverse
  under ShiftAt's own outward convention, shared grid demanded). Measurement arithmetic is
  half-even with conservatism entering ONCE at selection (basis.go's stated convention). big.Rat
  oracles with inline vacuity controls; the merge oracle is exact with the inward anti-rule as its
  control.
- **`driving` family-2 collectors** (`family2.go`): `LoadManifestDays` (the H2 universe = day
  manifests STRICTLY before the freeze day — the freeze day's manifest is still growing, so the
  strict bound is what makes two invocations agree; digests over manifest bytes are the artifact's
  H2 identifiers), `scanStream` (per-FILE vouching against the day's manifest, a vouched file that
  fails to decode is `ErrArchiveCorrupt` — integrity, never measured around), `MeasureBooks` (the
  committed pairing rule: epoch-grid samples, last book at-or-before t within slack, FIRST book
  at-or-after t+latency within slack, gap-spanning pairs discarded AND counted, depth-short
  excluded AND counted), `MeasureImminent` (settlement-flip realized rule within slack, stale
  instants take no sample, unrealized dropped and counted), `LatencyStats` (negatives KEPT — venue
  clock skew observed live, reported not clamped).
- **`procedure.go` — the §41e commit's parameter roster, every value chosen with rationale
  in-file:** 7-point residual grid {.05,.1,.25,.5,.75,.9,.95}; drift tail 0.05; EWMA half-life 72h
  (§2.4's stated default); replay policy (APR floor 1%, min hold 3, p25, dwell 2, 4× leverage);
  band q=0.9; basis horizon 24h q=0.99; family-2 cadences (10s grid, 1s slack, candidates
  0.02/0.1/0.5 BTC); classifier (calm/normal/stressed, leg-agnostic cut points: spread ≤5e-6 &
  depth ≥2 → calm; ≤2e-5 & ≥0.5 → normal; catch-all stressed — LAST RULE UNBOUNDED so every
  runtime book classifies, pinned by test); MinEvidence 500; **latency assumed bound 250ms**
  (healthy-hour transport p99 ≈100ms × stated 2.5 multiplier — the §8 marker's subject); frame
  clamps ±0.003 (measured from OUR fundingInfo overlay, 2026-07-31); capital 1100/300/100/50;
  scoring block (90d window, written regime rule, standby 3.69%, ceiling 5%, 0.95
  block-bootstrap LCB, min-util = 0.5×expected floor).
- **`Calibrator.Run`** (`run.go`/`emit.go`): §41b's order as the function's shape; §41e binding
  checked before one byte derives (dirty/unstamped/mismatch refusals + injectable seams) and
  recorded after (commit + executable self-digest); emit-time dataset re-hash; write-once;
  emitted bytes re-loaded through the production loader. Two invariants CHECKED not assumed:
  **the drift's family-1 pinning cannot understate the merged tail** (`ErrDriftTailDominated` —
  compared as RESULTING DRIFTS, because two non-negative tails both freeze zero; the first draft
  compared raw shifts and the synthetic world honestly refused it), and **the clearing level must
  not move when the frozen band substitutes into the frame** (`ErrClearingUnstable`). The CLI
  `calibrate run` takes `-freeze` (RFC3339, REQUIRED — a derivation that stamps "now" cannot be
  byte-identical twice). The dataset block enumerates the THREE files derivation reads (binance
  funding + spot/mark klines); Bybit funding stays committed but un-enumerated — read by no v1
  derivation, recorded here rather than silently included.

**Verification ledger (checklist BEFORE battery, again decisive):** the checklist found FIVE
collapses in this slice's own new fixtures (17th–21st instances): the pair-expiry arm was never
exercised (and its `<=` boundary had no at-boundary fixture); the stale-book skip never fired;
the imminent slack arms (stale estimate, stale realized testimony) never fired; `runWorld`'s two
legs and two sides were IDENTICAL books (§41i's named suspect verbatim — fixed with distinct
ladders, a perp completion that vanishes its best ask, and value assertions that pin leg AND side
keying); and every cell sat at 505 observations with MinEvidence 500 — no at-boundary cell (fixed
with a 500-exact spot tail KEPT and a 20-observation perp tail DROPPED). Plus the rounding-tie
fixture: every slippage ratio terminated before the ninth digit, where half-even and truncate are
the same function — the tie fixture (ratio exactly 1.5e-9 → half-even 2e-9) became the SOLE
killer of the rounding mutant. **23 aimed mutations: 22 first-aim kills, 1 SURVIVED diagnosed
into the real u-vs-U routing defect above (fix + fixture then killed both tag mutants), 2 NO-OP
re-aims caught.** Two gates fired live on first drafts: `exhaustiveswitch` (both RecordKind
switches) and `nofloat` (a test helper's float64) — the §3.6 machinery catching this slice's own
code. `make ci` green COLD, all six gates. Marker grep still EXACTLY ONE (§8's latency marker —
lands at part 2's close).

**Deliberately not done (part 2's docket, in order):** the governed derivation run from a CLEAN
checkout (two binary invocations, byte-identical; the artifact committed write-once), the
24-hour delay, the attestation + dated approval entry HERE, and the close (reading pass over
§41n's docket + slice A's + this record's stated readings, the §8 latency clause landed, marker
to ZERO, doc-set bump). **Watch on the real run:** the frame's clearing level will sit near
0.0011/settlement (the 24h min hold amortizes the full round trip) — if the real era yields no
recovered dips below it (`ErrNoExcursions`) or zero occupancy, that refusal is the governance
working, and what an evidence-free era means is the USER'S question, not a number to invent.

### §41p — the governed run REFUSED on the real era; the sensitivity diagnostic (session 53, 2026-08-01)

**The governed derivation was attempted and the era refused it — the watch note's named case,
live.** From a pristine clone at `95c0819` (VCS-stamped, `vcs.modified=false`, §41e binding
passed), `arblab calibrate run -freeze 2026-08-01T07:30:00Z` ran TWICE and refused
IDENTICALLY in ~22s each: `exit band: clearing 0.001602836 over 3381 rows: no recovered
excursion to measure a band from` (`ErrNoExcursions`). The determinism claim held on the
refusal path; no artifact bytes were written; write-once was never reached. Against the
committed funding era (3,381 settlements, 2023-01→2026-01): median 0.000074, p99 0.000468,
**maximum 0.000881** — zero settlements ever reached the clearing level, which sits at
**1.8× the era's all-time maximum**. There are no dips below clearing because the series
never got above it.

**The user's dated ruling (2026-08-01): diagnose first, then decide** — a read-only
sensitivity sweep over calibration-era data only, recorded here, before the revise-vs-kill
question is answered. The sweep ran as a throwaway harness in the scratch clone (deliberately
NOT committed: it mutates the Proc roster in-process, which is exactly what the committed tree
must never contain; method recorded here instead). It mirrors `Run`'s own pipeline — same
collectors, same `buildFrame`, same `ClearingLevel`/`ExitBandWidth`/`Occupancy` — varying
only `ProcMinHoldSettlements` and the capital/candidate tier, on the committed datasets and
the two closed archive days. No holdout row was touched; H1/H2/H3 stand.

The measured table (clearing per settlement; era count at-or-above; utilization where the
era yields evidence):

| min hold | clearing | ≥clearing | band (ann.) | utilization | entries/3yr |
|---|---|---|---|---|---|
| 3 (24h, committed) | 0.001603 | 0/3381 | refused | — | — |
| 6 (48h) | 0.000959 | 0/3381 | refused | — | — |
| 9 (72h) | 0.000745 | 4/3381 | refused | — | — |
| 21 (7d) | 0.000500 | 26/3381 | 0.162 | 0.0068 | 1 |
| 45 (15d) | 0.000402 | 49/3381 | 0.108 | 0.0145 | 1 |
| 90 (30d) | 0.000359 | 62/3381 | 0.126 | 0.0580 | 2 |

**Three structural findings:**

1. **The clearing level is capital-scale-invariant to the ninth digit.** Three tiers —
   $1.5K/0.02 BTC (committed), $7.75K/0.1, $38.75K/0.5, capital and candidate scaled
   together — produce byte-identical columns at every hold. Every cost the frame charges is
   proportional to notional (fee bps, per-unit p90, the adverse-fill and basis RATES), so
   the evidence-free era is a property of the strategy's economics on this venue/instrument/
   era, NOT of the $1K probe tier. The "it's the small-capital tier" hypothesis is refuted.
2. **The curve decomposes as round-trip-amortization plus recurring drag.** Fitting
   clearing(h) = A/h + R over the six holds: A ≈ 0.0039 (the full round trip ≈ 39bp of
   candidate notional, spread over the hold) and R ≈ 0.00032/settlement (hurdle excess +
   frozen drift + adverse basis, all under the notional-vs-C_allocated scaling). Even with
   the round trip fully amortized away (h→∞), the recurring floor alone (~3.2bp/settlement
   ≈ 35% APR-equivalent at the frame's scaling) exceeds the era's p90 funding print.
3. **Measured calm-stratum p90 slippage is ZERO to the ninth digit at every candidate**
   (0.02/0.1/0.5 BTC; 6.5–8.0k observations per cell). Binance top-20 depth absorbs these
   sizes without a walk — the frame's execution cost is fees, not book depth, which is why
   finding 1 holds so exactly.

**What the diagnostic does not say:** whether revising is justified. The conservative
pre-registered choices (p25 decision quantile, conservative aggregation, outward dispersion
merge) suppress entries BY DESIGN and their direction is stated in the §41 rationale; the
sweep held all of them frozen. The open question — revise the procedure openly (a v2 §41e
commit, post-hoc, dated) versus record the evidence-free era as kill-gate-shaped evidence —
is the user's, pending below.

---

## RESUME HERE

**THE GOVERNED RUN REFUSED ON THE REAL ERA (session 53, 2026-08-01, §41p): `ErrNoExcursions`
at clearing 0.001603/settlement against an era whose maximum settled funding is 0.000881 —
zero of 3,381 settlements ever cleared entry.** The refusal is deterministic (two stamped
clean-checkout invocations, identical), no artifact exists, write-once was never reached. The
user's dated ruling was DIAGNOSE FIRST; §41p records the sensitivity sweep: clearing is
capital-scale-invariant, hold-length-dominated, and even a 30-day minimum hold yields 5.8%
utilization with 2 entries in three years. **The open governance question is REVISE-VS-KILL:**
a v2 §41e procedure commit (post-hoc, dated, openly recorded) versus recording the
evidence-free era as kill-gate-shaped evidence. **Do not re-run the governed derivation, do
not write the attestation, and do not land the §8 clause until the user rules.** The §8
latency marker stays TAKEN (marker grep expects EXACTLY ONE). The seal (§41h: hybrid +
Binance) stands — the diagnostic touched no holdout row. The capture rituals stand
(session 53's verify: COVERAGE-only, 0 integrity findings). After the ruling, the docket is
§41o part 2's unchanged: governed run (under whatever procedure the ruling names) → 24h
delay → attestation → close (§8 clause, marker to ZERO, v0.9.18).

---

## RESUME HERE (superseded mid-session 53 — HISTORICAL)

**SLICE C IS HALF-BUILT (session 52, 2026-08-01, §41o): the family-2 machinery, the procedure
roster, and the real `Calibrator.Run` are BUILT and verified (23 mutations, one diagnosed into
the u-vs-U decoder fix); the GOVERNED ACT IS NOT DONE.** The seal was re-verified standing
(§41o first bullet). `calibrate run -freeze` is real and demands a clean stamped build. The §8
latency marker stays TAKEN (marker grep expects EXACTLY ONE until the close). The promotion
identity stays SEALED (§41h): hybrid + Binance — do not reopen. Next, in order:

1. **The governed derivation run** (§41e): from a CLEAN checkout of the committed tree, build
   `bin/arblab` (VCS-stamped), run `arblab calibrate run -freeze <RFC3339>` TWICE into two
   paths, verify byte-identical, commit the artifact write-once at
   `configs/calibration.btcusdt.v1.json`. Choose the freeze instant at the run (its UTC day
   bounds H2 to days strictly before). Expect and honor refusals: `ErrNoExcursions` /
   zero-occupancy / `ErrFrameUnpriced` are governance questions for the user, never numbers to
   invent (§41o's watch note).
2. **The 24-hour delay**, then the attestation (`configs/calibration.btcusdt.v1.approval.json`,
   §41e's shape: artifact ID + content hash, procedure commit, dataset ledger, approval instant,
   sign-off, out-of-band confirmation) + the dated approval entry in THIS record.
3. **The close**: reading pass over the three docket lists (§41n's three items, slice A's, and
   §41o's stated readings), land the §8 latency clause (the §40d modeled-tier shape: declared
   upper bound authorized for the $1K gate, measured order-path latency required before capital),
   marker grep to ZERO, doc-set bump to v0.9.18, §41o part 2, HANDOFF update.
4. Then **step 5** (§31a): the scored replay runner under the sealed identity.

Standing rituals first, every session: `pgrep -fl "arblab capture run"` (PID 74192), backgrounded
`capture verify` over the accumulated range (0 integrity findings, always — COVERAGE verdicts
over the known Bybit-spot absent windows are the honest ledger; **exit code 2 IS the COVERAGE
verdict**), `git status --short`, marker grep (EXACTLY ONE, §8's, at `driven/cost.go`).

---

## RESUME HERE (superseded end of session 52 — HISTORICAL)

**SLICE B IS BUILT (session 51, 2026-08-01, §41n): the family-1 derivations exist with oracles
(residuals/quantiles/drift, band, basis, the surrogate frame, clearing level, the
Enter/Continue occupancy replay), the fetchers are real, and FOUR datasets are COMMITTED with
pinned digests (both venues' funding 2023-01-01→2026-01-31; Binance spot+mark 1h klines
2023-04-01→2026-01-31 — the kline era starts late because H3 fired live on the 2023-03-24 spot
halt). `calibrate fetch` works; `calibrate run` still refuses. The §8 latency marker stays
TAKEN (marker grep expects EXACTLY ONE until slice C's close).** The promotion identity stays
SEALED (§41h): hybrid + Binance, 2026-07-31 — do not reopen. Next, in order:

1. **Slice C** (§41j): **first act — verify the §41h seal stands** (no holdout-era fetch
   predates it; the sealed identity is what the artifact will enumerate). Then the family-2
   measurements over the archive (stratified per-leg p90 cells via the frozen classifier and
   the committed counterfactual observation, adverse fill move, imminent residuals from the
   `markPrice@1s` stream + the §41b pointwise-more-adverse merge), transport-latency
   statistics, the family-3 declarations with rationale, the derivation run from a CLEAN
   checkout (§41e's execution binding; §41i's two-invocation determinism claim; §41b's order —
   costs, dispersion, band freeze before the occupancy replay consumes them), the 24-hour
   delay, the attestation + dated approval entry, and the close: the reading pass over §41n's
   three-item docket + slice A's, **the §8 latency clause landed** (marker grep back to ZERO),
   doc-set bump. Slice C decisions still open: the surrogate frame's committed constants (frame
   prices from the era's first klines, candidate quantity, capital decomposition), the residual
   quantile grid, the band/basis quantiles and the basis horizon — all procedure parameters the
   §41e commit pins, chosen and recorded THERE, never here.
2. Then **step 5** (§31a): the scored replay runner — the hybrid's pre-registered §8 reading
   duty, §40k-2's revision repository, the sealed identity's everything-eligible rule.

Standing rituals first, every session: `pgrep -fl "arblab capture run"`, backgrounded
`capture verify` over the accumulated range (0 integrity findings, always — COVERAGE verdicts
over the known Bybit-spot absent windows are the honest ledger), `git status --short`,
marker grep (**EXACTLY ONE**, §8's, at `driven/cost.go`).
