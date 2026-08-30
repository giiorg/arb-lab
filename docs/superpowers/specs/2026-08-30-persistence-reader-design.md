# A Go reader for the persistence artifacts — design (record §103)

**Date:** 2026-08-30
**Status:** design approved, unimplemented
**Closes:** the last open next action of the web dashboard review — *"a reader
for the persistence artifacts"*, F83's structural half.
**Opens:** F102, F103 (below), and review round 13.

## 1. Why this exists

Round 9 (`2026-08-29-web-dashboard-design.md` §15.14) established the check that
found F80, F81, F82 and F83: **resolve a row against the port's signature, not
against the file.**

> A file answers *what values exist*; a signature answers *which coordinates the
> answer depends on, and how it can refuse*.

Two of round 9's three rows had a port between the row and the file — `ports.Fees`
and `ports.LotRules` — and each failed on both halves of the signature. **Row 21
has no port at all, and §15.14 states that this is what makes its coordinate loss
invisible.**

The absence is structural and it is measured. `summary.json` and `episodes.json`
have **zero** Go readers in `internal/`, `cmd/` or `tools/`. Their three consumers
are all outside the type system:

| Consumer | What it is |
|---|---|
| `research/persistence/summarise.py` | the writer of `summary.json` |
| `research/persistence/verdict_levels.py` | F60's measurement |
| round 12's `gen-remaining-screens.py` | the mockup generator |

The contrast is one directory over. `internal/strategy/infrastructure/screening/report.go`
declares Go types for `ranking.json`, `shortlist.json` and `census.json` — and
**F55, F60, F79 and F83 are four coordinate losses in the ported file's unported
sibling, against none in the reference document.**

**A fourth un-typed consumer is the trigger.** Round 12's generator was written to
draw mockups and became the third Python reader of these files. Round 12's own
conclusion — *"changing the instrument is what produced the findings"* — names the
instrument this record builds.

## 2. What is lost, and where

`summarise.py` computes four blocks. One records its coordinates; three do not.

| Block | Computed at | Records it? | Finding |
|---|---|---|---|
| `pooled` | every declared level; `wins`/`mean_net` keyed by cost | **yes, both axes** | — |
| `run_vs_breakeven` | level `0.0005`, cost `0.0021` (`summarise.py:84`, `:87`) | no | F55, F84 |
| `per_symbol` | level `0.001` (`summarise.py:98`) | no | F60, F79 |
| `verdict` | inherits `per_symbol`'s `0.001` | no | F60 |

So the artifact is sound where F83 lands (`pooled` carries both axes) and the loss
is in the **row**; and the artifact is itself lossy in the other three blocks,
where the loss is in the **file**. A reader has to answer for both.

**A fifth coordinate is in no field and in no row.** `summarise.py:96` pins
`CAP = 2000` and `LEG = 1000`. The field name `annual_pct_on_2000` carries the
capital. The **per-leg notional scales the numerator** and is recorded nowhere —
F83's disease one layer down, and it is why `SizeAt` (below) carries two Money
values rather than one.

## 3. Decisions, and the alternatives rejected

### 3.1 Recompute from `episodes.json`; never decode `summary.json` in production

The reader parses **only** `episodes.json` — the per-episode expansion, which
carries `entry` on every element — and computes run length, break-even, net and
annualised return itself, at coordinates the **caller** names. Every returned
value therefore carries its coordinates by construction, and F55, F60, F83 and F84
become **unrepresentable** rather than annotated.

`summary.json` is used in tests only, as an oracle (§6).

**Rejected — decode `summary.json` and refuse the pinned blocks.** Cheapest, and
it makes the loss loud at the type level. Rejected because the refusal would be
**permanent**: nothing in a reader can supply what the file never wrote, so the
three blocks would be refused forever, and serving them would mean hard-coding
`summarise.py`'s pins as Go constants — *a constant asserting an external fact*,
about a script that can change without the constant changing.

**Rejected — read both, and treat `summary.json` as a checked cache.** Strongest
evidence and the most work. Rejected for production use because it makes a derived
file an input; kept **as the test strategy**, where its agreement is measured
rather than assumed and carries a negative control.

**Accepted consequence:** the Go fold becomes a **second independent
implementation** of `summarise.py`. This repo's standing rule is that real defects
here have been found by two implementations disagreeing, not by review
(`fixedpoint` has a `math/big` oracle; `ids` has one for its base32 codec). A
disagreement on committed data is a finding, not a test to loosen.

### 3.2 A concrete package, no entry in `application/ports`

`internal/strategy/infrastructure/persistence`, beside `screening/`, which is the
exact precedent: also a research harness in `strategy/infrastructure`, also
emitting committed artifacts, also keeping its `.txt` emitter beside it.

**Rejected — an eighth port in `strategy/application/ports`.** Most literally
§3.6 rule 2, and it would earn the `var _ ports.X = (*Impl)(nil)` assertion whose
`NOBUILD!!` this repo counts as a kill. Rejected on two grounds this repo has
already paid for:

1. `ports.go:7` reads *"The roster is §40b's, **seven in all**"*. An eighth entry
   is a normative claim, and *an enumeration's count word is a claim*.
2. **Ask which end of the policed edge exists.** The consumer is a phase-3
   dashboard context that does not exist, and no application service in `strategy`
   consumes this. A driven port with no caller is a check that cannot fail —
   and `ports.go:33` already names *"the consumer-owned-port trigger"* as the
   right direction. Phase 3 declares the interface it needs; this satisfies it.

**Rejected — a pure fold in `strategy/domain` plus a thin reader.** Best
separation, and the arithmetic would be unit-testable with no file. Rejected
because §2.3's live strategy does not use this research arithmetic, so it would be
domain code with no production consumer — *a transition with no driver*.

### 3.3 Coordinates are nesting types, and they ride on the value

```go
type EntryAt struct{ Entry fixedpoint.Rate }                        // run length
type CostAt  struct{ EntryAt; Cost fixedpoint.Rate }                // break-even, net
type SizeAt  struct{ CostAt; LegNotional, Capital fixedpoint.Money } // annualised, verdict
```

Each has `Validate()`; each embeds the previous. The nesting **is** F83's claim —
*run length is a function of entry level alone* (`summarise.py:58`; cost enters
only `wins`/`mean_net`) — expressed as a type rather than as a sentence in a
record.

Every result struct **embeds the coordinates it was computed at**:

```go
type RunLength struct {
        P50, P75, P90, Max int
        Mean               fixedpoint.Rate
        Episodes           int
        At                 EntryAt
}
```

**Rejected — required arguments with plain result structs.** Smaller and more
idiomatic. Rejected because F55, F60, F83, F84 and F100 are one family, and the
family is *not* "the function forgot an argument" — it is **the value travelled
without its base**. `HANDOFF.md` restating *"median episode is 2 settlements"*
with no level is what a correct call looks like three hops downstream, and at two
of the four levels it is **1**. A required argument cannot reach that; a
coordinate on the struct can, because a screen renders what it holds.

**Rejected — one `Query` in, one `Answer` out.** Guarantees every figure on a
panel shares one coordinate set, which is arguably what a panel wants. Rejected
because it collapses five questions into one: a caller needing run length would
pay for the annualisation, and the per-row refusals would merge into one.

**Rejected — one `Coordinates` struct with optional fields.** Rejected because an
unset `Cost` is a field nobody populated reading as a value, which is exactly what
`platform`'s zero-value rule exists to prevent.

### 3.4 The hurdle is an argument, never a constant

```go
type Hurdle struct {
        Annual        fixedpoint.Rate
        Class, Source string
}
```

An unset `Hurdle` is refused. F45 is the reason: `summarise.py`'s `HURDLE_PCT` is
`illustrative_constant`, is **not** reference data (§8c defines that as
`configs/reference.*.json` specifically), and is not a calibrated benchmark per
build-spec §55, whose input is dated and haircut-adjusted. A Go constant here
would re-commit F45's defect in a second language.

## 4. The surface

```go
func Load(path string) (Set, error)

func (Set) RunLength(at EntryAt) (RunLength, error)
func (Set) Breakeven(at CostAt) (BreakevenTable, error)
func (Set) NetPerEpisode(at CostAt) (NetPerEpisode, error)
func (Set) PerSymbol(at SizeAt) (SymbolReturns, error)
func (Set) Verdict(at SizeAt, h Hurdle) (Verdict, error)
```

`Load` takes a path. There is no ambient default, and no `time.Now`.

### 4.1 Refusals, named

| Error | Fires when |
|---|---|
| `ErrLevelUndeclared` | the file declares no such entry level — asking for `0.0003` **refuses**; it does not return an empty table |
| `ErrCoordinateUnset` | a coordinate's `Validate()` failed |
| `ErrSpanDegenerate` | a symbol whose span is under a day, where `summarise.py:33`'s `max(1, days)` silently floors the annualisation denominator |

`ErrSpanDegenerate` fires on **zero** rows of the committed file (measured: 0 of
61 symbols span under a day). It is declared anyway, because `max(1, ...)` is a
papering-over that inflates an annualised return without failing, and *a gate's
refusing path is the untested one* — it gets a hand-built fixture, not a
committed-data one.

### 4.2 The denominator moves, and the reader says so

`SymbolReturns` carries **`Excluded []string`** beside its rows.

`summarise.py:99` is `if not e: continue`. Measured over the committed file, the
symbols with an empty episode list are **1 at `0.0005`, 5 at `0.001`, 12 at
`0.002`** — which is why `verdict.total_symbols` reads **56** while the file's own
header declares `symbols_eligible: 61`, and why F60's counts run 61 / 60 / 56 / 49
across the four levels. **The denominator is a function of the entry level and no
field records that it is.** The reader returns the shrinkage rather than swallowing
it.

## 5. Arithmetic, and one thing `fixedpoint` cannot represent

Counts and percentiles are `int`, replicating `summarise.py:38`'s nearest-rank
index exactly. Everything else is `fixedpoint`, every rounding an explicit
argument.

**Annualised return is returned as a `Rate` ratio, not a percentage.**
`rate.go:8` is explicit: *"Rates are stored as plain ratios, never as percentages
… Nothing in this package multiplies or divides by 100."* So the artifact's
`annual_pct_on_2000` carries a unit the type system declines, and the conversion
lives in the test that compares against it.

**`summary.json` publishes numbers this project's money type cannot hold.**
`mean_gross` at level `0.0002` is `0.0006533427008602499594221717254` — 28 decimal
places, straight out of Python's `Decimal`. `fixedpoint.Scale` is **9**. The
oracle comparison is therefore *"agrees when the Python value is rounded to 9
places under the declared mode"*, and the test says so in words. A byte-equality
assertion is not available, and an assertion that passes must not imply one was.

## 6. Testing

### 6.1 The oracle, and its negative control

Decode the committed `episodes.json`; compute `pooled` at all four declared
levels; compare against `summary.json`. Exact on the integers; at 9 places on the
means.

A **negative control** is required: the same comparison at the **wrong** entry
level must **fail**. `research/persistence/README.md` already carries this lesson
as control C — *"an agreement that was never shown capable of failing is the
repo's standing trap"*.

### 6.2 The fixture-collapse checklist, run before the tests are written

Three results, all settled before implementation:

1. **The separating member is `episodes`, not `p75`.** Measured across the four
   declared levels: p75 is **3, 3, 3, 3**; p50 is 1, 2, 2, 1; p90 is 5, 6, 5, 4;
   `episodes` is **12 322 / 3 425 / 1 259 / 472** — the only member with four
   distinct values. A test asserting p75 passes at every level and detects
   nothing. **This is F83's own collapse, and it would reproduce inside the test
   written to fix it.**
2. **`>` and `>=` are the same predicate on this file.** `wins` counts
   `accrued > cost`. Measured over all 17 478 episodes: **exactly zero** accrue
   any of the three costs (`0.0021`, `0.0029`, `0.0090`), so both comparisons
   give 1 969 / 1 485 / 430. The committed data **cannot** separate them, so the
   boundary needs a hand-built fixture with an episode accruing exactly the cost.
   **The sixteenth collapse**, found by running the checklist first.
3. **`ErrSpanDegenerate` has no committed instance** (§4.1) and takes a
   hand-built fixture.

### 6.3 Mutation targets

The percentile index (`min(len-1, (len*q)//100)` — off-by-one in either
direction), the `wins` comparison (needs 6.2's fixture to be killable at all),
the `max(1, days)` floor, and the `Excluded` accumulation.

## 7. Two findings the decoding produced before a line was written

### F102 — two timestamp layouts in one emitter, and 34 956 instants no consumer has ever parsed

`research/persistence/harness.go.txt` formats episode instants at line 120–121 as
`"2006-01-02T15:04Z"` and symbol-span instants at line 253–254 as
`"2006-01-02T15:04:05Z"` — two layouts, 133 lines apart, in one emitter. Neither
is `time.RFC3339`, and Go's `time.RFC3339` **rejects the first**:

> `parsing time "2025-10-27T08:00Z" as "2006-01-02T15:04:05Z07:00": cannot parse "Z" as ":"`

Measured over the committed file: **122** symbol-level timestamps at 20
characters, **34 956** episode-level timestamps at 17.

**Why it stayed invisible is the finding.** `summarise.py` parses only the
symbol-level pair, in `days()`, and never an episode instant; Python's
`fromisoformat` accepts both anyway. So the 34 956 episode instants are strings
that look like times to every consumer that has ever existed. Every other
committed research artifact stamps RFC3339 (`ranking.json`, `census.json` and
`shortlist.json` all carry `2026-08-01T09:57:56Z`), and `screening/report.go:178`
formats with `time.RFC3339` by name.

**General:** *a field nothing parses is not a typed field.* A format defect
survives indefinitely behind a permissive decoder, and the first strict reader is
what dates it.

**Not fixed here.** Correcting the emitter would require regenerating
`episodes.json`, whose input `research/screening/raw/` is gitignored — a re-fetch.
The reader accepts the file's actual layout and names it; the defect is recorded.

### F103 — five records with `run: 0`, which is not an episode

Five records in `episodes.json` carry `"run": 0`. All five are at entry level
`0.0002`; all five have `start == end`; all five accrue `"0"`; all five sit at the
tape's final decision point (`2026-07-31T16:00Z` or `20:00Z`, against a span
ending `2026-07-31T23:00Z`). They are entries triggered with no settlement after
them — **right-censored observations, not runs of length zero.**

They are counted in `episodes` — the published **12 322** includes 5 non-episodes
— and they move the published `run_mean` from **2.434 to 2.433**.

**Two things hide them, and the second is F55's pin.** First, 5 in 12 322 cannot
move a percentile, so `run_p50`/`p75`/`p90` are identical with and without them.
Second, `run_vs_breakeven` — the one published block whose shape would show a `0`
bucket, and whose table starts at run 1 — is computed at level **`0.0005`, where
none occur.** The pinned coordinate F55 names is exactly the one that hides the
zero bucket.

**Scoped honestly.** Total right-censoring is **14 / 4 / 2 / 0** episodes across
the four levels and moves **no** percentile at any level (measured: p50/p75/p90
identical with censored episodes included and excluded). So this is a defect in
the counts and in what an episode *means*, not in the run-length answer, and the
record should not claim more.

**What the reader does:** `crossvenue.Window.Validate` already *"rejects the
unusable zero value and every malformed direction"*. The reader classifies a
zero-settlement record as censored and reports the count rather than admitting it
to a run-length population, and `RunLength` carries the censored count beside the
percentiles.

## 8. Scope

**In:** the package, its types, its refusals, the oracle test with its negative
control, the three hand-built fixtures §6.2 names, and the mutation run.

**Out:** any dashboard or HTTP code; any change to `summarise.py`, `harness.go.txt`
or the committed artifacts; any `ports.go` edit; any normative edit. `build-spec.md`
stays at v0.9.74 and `make formal-fresh` is unaffected.

## 9. What this record claims and does not claim

**Claims.** That row 21's coordinate loss is structural and a signature is its
remedy; that recomputation from `episodes.json` makes four findings
unrepresentable rather than annotated; that F102 and F103 are measured facts about
the committed file, with the magnitudes stated in §7.

**Does not claim.** That the Go fold agrees with `summarise.py` — that is what
§6.1 measures, and a disagreement is a finding either way. That F103 changes any
published percentile; §7 measures that it does not. That the reader is used by
anything: it has no production consumer until phase 3, and §3.2 records why that
is the right shape rather than a gap.
