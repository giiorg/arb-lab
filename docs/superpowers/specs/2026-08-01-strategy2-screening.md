# Strategy #2 screening — milestone 2b step 1 executed (2026-08-01, session 56)

**Status: RUN COMPLETE (2026-08-01 10:39Z) — 465/465 symbols fetched with
zero failures, 61 pairs ranked, and the shortlist is
`LABUSDT BEATUSDT KAITOUSDT MMTUSDT MUSDT SOONUSDT ESPORTSUSDT BCHUSDT`
(§43d). §43c/§43d/§43e below are filled from
`research/screening/{ledger,ranking,shortlist,census}.json`; doc set
closed at v0.9.21 the same session.**

Mandate: scoping record §42's RESUME HERE, step 1. The §42b screening step —
both venues' settled-funding REST history for the intersection universe,
paired by venue-published instants, ranked by persistence-adjusted capturable
spread after two-venue costs, plus the §42b-1 dislocation census. Parameters
were deliberately NOT chosen in the scoping record; they are chosen HERE
(§41e's pattern), and §43b records each with its rationale.

## §43a — what was built, and the two departures §42 licensed

The harness is **committed code** — `internal/strategy/infrastructure/screening`
plus the `arblab screen run` subcommand — not a scratch-clone throwaway like
§41p's sweep. The difference is principled: §41p's harness mutated the
governed Proc roster in-process, which the committed tree must never contain;
the screening mutates nothing governed, and §42b requires its parameters
pinned "at its own commit", which only a commit can do. What it keeps from
the §41p pattern: read-only over the world, mirrors nothing it does not need,
one dated record.

It deliberately does NOT reuse the calibration fetcher (`driving.Fetcher`),
because two of that fetcher's guarantees are WRONG here rather than merely
unneeded:

- **H1 refuses rows on or after 2026-02-01** — the screening window runs to
  its own cutoff (§43b). The seal the screening DOES enforce is narrower and
  permanent: **BTCUSDT is excluded from the fetch universe entirely** (§42e:
  post-2026-02-01 BTC stays untouched, and a screening read of BTC funding
  through the cutoff would cross it). The exclusion is a tested behaviour —
  the harness test asserts no BTC funding URL is ever requested — not a
  filtering step after the bytes arrive.
- **H3 demands a closed grid declared up front** — an alt's history starts at
  its listing date and its interval has changed over time, so the grid is
  **measured from the assembled rows** (§42a's honesty note), never declared.
  Grid facts in the outputs (modal interval, interval changes, per-symbol
  first/last) are measurements.

Raw venue responses land verbatim under `research/screening/raw/`
(**git-ignored**: hundreds of megabytes, re-derivable — settled funding is
the backfillable half, §42a), and every file's SHA-256 is pinned in the
**committed** `research/screening/ledger.json`, so a divergent refetch is
visible without committing the bytes. This is the §41n ledger discipline with
the storage inverted, and the inversion is the backfillability asymmetry
applied to our own outputs.

## §43b — the pinned parameters (normative source: `screening/params.go`)

| Parameter | Value | Why |
|---|---|---|
| Cutoff | 2026-08-01T00:00:00Z, strictly before | Complete days; everything the ranking saw precedes the capture this step triggers, so post-cutoff data is selection-clean for the gate |
| History start | 2019-01-01Z | Predates both venues' USDT-perp funding; a bound, not a claim |
| Excluded | BTCUSDT | §42e's permanent H1 seal (see §43a) |
| Pairing | venue-published instants rounded to NEAREST minute | Absorbs Binance's ms jitter (profiles §7); far below the smallest interval; a venue printing twice in one bucket is dropped and counted, never guessed |
| Adjacency | ≤ 2× the pair's measured modal interval | An episode may not accrue across settlements we could not match; the census splits on the same bound |
| Round trip | 21bp (2×0.0500% + 2×0.0550%) | Four perp taker legs at the base-tier published cells (profiles fee table, 2026-07-31); maker entry stays a gate-time tactic (§42e), never a screening assumption |
| Entry grid | {2, 5, 10, 20}bp | An episode opens at \|spread\| ≥ E; the ranking takes each pair's best level AND REPORTS IT — best-of-four is mild selection, carried openly |
| Exit | E/2, hysteresis | Exit-at-entry churns round trips on noise; exit-at-zero holds through prints that pay neither venue's fee |
| Episode accounting | trigger print observed never accrued; exit print accrued; episodes losing money stay in the sum; open episode at data end closed and charged | No look-ahead; no winner-picking; conservative close |
| Ranking floor | ≥ 180 matched settlements | ≈30 days at the 4h majority grid; a thinner pair annualizes noise (§42a listing-date honesty). Thin pairs stay in the census |
| Tradability floor | 24h turnover ≥ 5M USDT on BOTH venues | Deliberately permissive — drops dead markets, does not pre-empt the §42c depth measurement; volume-only (Bybit OI value recorded as evidence, ungated; Binance OI would cost a per-symbol call and decides nothing here) |
| Rank score | max over the entry grid of annualized net (Σ episode nets − episodes × round trip, over the matched span in 365.25d years) | §2.3's bar is persistence: the episode, not the print, is the unit of evidence — a spread that never persists cannot out-rank costs no matter its headline quantile |
| Census level | \|spread\| ≥ 25bp at one matched settlement | Larger than the whole round trip; 2.8× BTC's era maximum (8.8bp, §41p) — nothing steady-state qualifies |
| Shortlist size | 8 | Ruling §42f-1 |

**The selection-contamination statement, made once and carried forward:** the
shortlist is selected on pre-cutoff history. Any future gate scoring over
pre-cutoff data for shortlisted symbols is scoring the winners of this
selection and must say so; the clean evidence for the gate is what
accumulates after the cutoff — the capture this shortlist now triggers, and
post-cutoff settled funding. Strategy #2's holdout is declared at its
procedure commit (§42e), and this statement is an input to that declaration.

## §43c — the run (measured 2026-08-01)

Launched 09:54:45Z as a serial walk; relaunched 09:57:56Z with the venue
walks made concurrent (the resume skipped the 28 files already on disk —
the resume discipline exercised for real on its first day), fetch + compute
done 10:39:05Z. **465/465 symbols fetched, zero failures**
(`raw/fetch_failures.json` empty); BTCUSDT refused by the §42e seal with the
refusal logged, never fetched. **936 raw files, 11,684 pages; 935 ledgered**
(930 funding = 465 × 2 venues, plus `exchange_info`, `funding_info`,
`instruments`, two `tickers`; `fetch_failures.json` is the harness's own
record, not a venue response, so it is deliberately outside the ledger).
`fetched_at` is the injected launch instant of the process that fetched the
file — 28 files carry the serial stamp, 907 the concurrent one — provenance,
not a per-file clock read.

**1,788,250 Binance rows + 1,888,177 Bybit rows → 1,572,157 matched
instants, zero pairing collisions on either venue** — nearest-minute
rounding absorbed every publication jitter without ever seeing two prints
in one bucket. Eligibility partitions the 466 exactly: **61 ranked, 367
below the 5M turnover floor, 35 mismatched-grid (out of v1 scope by ruling),
2 insufficient history (GRAMUSDT at 178 matched — two short of the floor;
GRVTUSDT at 3), 1 excluded (BTCUSDT)**. The 61 ranked split 32 at 4h modal /
29 at 8h. The grid honesty note earned its keep: several pairs' matched
counts exceed pure-modal-grid arithmetic over their spans and their
`interval_changes` counts run into the hundreds (denser early grids, venue
interval moves) — measured from the assembled rows, exactly what a declared
grid would have silently mis-modeled.

## §43d — the ranking and the shortlist

The top eight ranked pairs are the shortlist (ruling §42f-1: size 8):

| # | Symbol | Best E | Net/yr | Episodes (won) | Cont./10k | Matched | First | Grid |
|---|---|---|---|---|---|---|---|---|
| 1 | LABUSDT | 10bp | **71.6%** | 71 (29) | 6294 | 2004 | 2025-10-27 | 4h, 454 moves |
| 2 | BEATUSDT | 20bp | **26.4%** | 15 (8) | 6363 | 1851 | 2025-11-21 | 4h, 449 moves |
| 3 | KAITOUSDT | 5bp | **17.0%** | 55 (23) | 6976 | 3157 | 2025-02-20 | 4h, 1 move |
| 4 | MMTUSDT | 10bp | **9.6%** | 22 (10) | 5178 | 1616 | 2025-11-04 | 4h, 1 move |
| 5 | MUSDT | 10bp | **8.1%** | 75 (19) | 4267 | 3260 | 2025-07-08 | 4h, 1240 moves |
| 6 | SOONUSDT | 20bp | **5.6%** | 33 (12) | 4545 | 3690 | 2025-05-23 | 4h, 1445 moves |
| 7 | ESPORTSUSDT | 20bp | **2.6%** | 19 (9) | 4062 | 2519 | 2025-07-29 | 4h, 420 moves |
| 8 | BCHUSDT | 10bp | **1.5%** | 39 (16) | 6134 | 6167 | 2020-12-14 | 8h, 0 moves |

The cut fell between BCHUSDT (1.5%) and #9 LTCUSDT (0.93%/yr); the veterans
(LTC, LINK, XLM, ZEC…) cluster under 1% — the dislocation spread is a
young-listing phenomenon, and seven of the eight are 2025 listings on the 4h
grid. Best-of-four selection is carried openly per §43b: the winning entry
level varies 5–20bp across the eight. **Every row's win rate is below half**
(LABUSDT nets 71.6%/yr winning 29 of 71 episodes) — the score is net of
every losing episode and a 21bp round trip per episode, so what pays is
persistence, and the continuation column (P(stay above E | above E) ≈
0.41–0.70) is that fact measured. The floor did most of the excluding (367
of 405 ineligible), which is the permissive-floor design working: dead
markets out, depth judgment deferred to §42c's books. The §43b
selection-contamination statement covers everything in this table.

## §43e — the dislocation census (§42b-1)

**2,972 events of |spread| ≥ 25bp at a matched settlement, across 365 of the
465 pairs covered.** 867 (29%) span more than one settlement under the
adjacency bound; direction splits 1,690 positive / 1,282 negative. By year:
2 (2020), 87 (2021), 169 (2022), 151 (2023), 104 (2024), **1,064 (2025),
1,395 (2026 in seven months)** — raw counts over a growing universe, not
per-symbol-normalized, so the rise conflates regime with listing count; what
it does establish is that the phenomenon is current, not a 2021 relic.
Extremes: peak single-settlement spread **3.01%** (CELOUSDT, 2022-03-17);
longest episode **23 settlements** (TRBUSDT, 2023-09-14, summing 14.4%);
largest cumulative **39.1% over 20 settlements** (HUSDT, 2026-06-09 — twenty
consecutive 4h settlements averaging nearly 2%). All eight shortlist symbols
are census-active (LABUSDT 42 events down to BEATUSDT 11), so the ranking's
winners are dislocation names, not merely steady-drift names. This is the
table that prices any future broad funding tier (ruling §42f-2): the events
exist venue-wide as settled prints — 2.4k of the 2.97k in 2025–26 — and
whether a book exists to execute against is exactly what it cannot say
(§43g).

## §43f — verification

- The checklist ran BEFORE the mutations and found three fixture collapses
  in the first draft — no fixture sat at the exact entry level, the exact
  exit level, or the exact gap bound, the three prints where `<` and `<=`
  give different answers (the ratelimit lesson, again) — plus an
  uncovered sign guard in the continuation counter. All four fixtures were
  added first.
- **Nine aimed mutations, nine caught** (episode net sign, gap bound
  off-by-one, entry boundary, exit boundary, trigger-print accrual,
  pairing round-vs-truncate, census boundary, 365d-vs-365.25d
  annualization; one NO-OP re-aimed into the trigger `continue` and then
  caught). Every mutant was aimed at a comparison or constant the ranking's
  value flows through.
- The end-to-end battery drives a canned two-venue world through both
  stages and asserts on REQUESTS as well as outputs: the BTC seal is
  checked as "no BTC funding URL was ever asked", and resume is checked as
  "a second fetch stage issues no new requests except the recorded
  failure's retry".
- `make ci` green (fmt, vet, all six gates, race suite, both modules); the
  fetch stage's venue concurrency is race-tested through the same battery.

## §43g — what this record does NOT establish

The §42b-1 statement carries: the census shows dislocations EXISTED as
settled prints; it cannot show a book to execute against — that evidence
begins only when §42c's capture starts, and only for the shortlist. The
ranking's costs are the published base-tier taker cells, not measured alt
depth (§41r caveat 1 — the gate re-prices from §42c books). Nothing here is
gate evidence; it is the ordering that decides whose books get captured.

## RESUME HERE

Step 1 of milestone 2b is DONE: harness committed, run executed, shortlist
and census committed, this record dated. Next, per §42's RESUME (unchanged):

1. **The capture expansion (§42c)** — the §43d shortlist × four streams into
   `configs/capture.json`, restart with its honest `process_restart` gap
   record, then the **§32m-D one-day measurement within 48h** — which is
   also §38f's named revisit point (ruling §42f-3).
2. **The `strategy/crossvenue` read-path, the research simulator, and the
   gate's procedure commit** (§42e) — after data flows; gate parameters are
   pinned there, never here.

H1 stays sealed. BTC's eight streams run unchanged. Capture alive, verify
COVERAGE/0-integrity, marker grep zero.
