# Record §99 — the spread was measured exhaustively and its own cost input never was

**Date:** 2026-08-29 (session 146) · **Task:** the persistence measurement §98 named as the next action · **Milestone:** 4 (pre-gate)
**Predecessors:** record §98 (the first live pair run, and the 0.90% round trip it could not attribute), §42b (the screening harness), core §2.3 (the edge bar), §2.1 (the cost model)
**ADR rows:** F26, F27, F28, F29
**Nothing was sent to a venue. No normative document changed. The output is a measurement and four findings.**

---

## 1. Why this record exists

`docs/HANDOFF.md` closed §98 with an instruction: *not another live run — measure
spread persistence*, because break-even at the then-current spread is ≈6.2
settlements and the strategy's viability is therefore a question about **how long
an alt's cross-venue funding spread survives**. §2.3 says the same thing
normatively: *"edge bar set from measured spread persistence, not the headline
spread."*

The handoff also said where to start: **read what `screen run` has already
produced** before fetching anything. That instruction was correct and it is why
this session fetched nothing. `research/screening/ranking.json` already measures
466 venue pairs at four entry levels, and `research/screening/census.json`
already holds a dislocation census. What neither publishes is the **length of an
individual episode** — `LevelResult` folds episodes into `Episodes`, `Wins`,
`Accrued` and `MaxRun` — and what nobody had varied is the **cost the whole
ranking is scored against**.

## 2. What was done

A harness (`research/persistence/harness.go.txt`) re-reads `screenLevel`'s state
machine and emits each episode instead of folding it: settlements collected,
spread accrued in the episode's own direction as an exact decimal, and the start
and end instants. 61 eligible symbols × 4 entry levels → **17 478 episodes**,
over **235 073 matched settlements**, 2020-10-21 → 2026-07-31.

Of `ranking.json`'s 466 pairs only **61** are eligible: 367 sit below the
turnover floor, 35 have mismatched grids, 2 are thin, 1 is the H1 seal's BTC.
Among the 61, 32 settle on a 4-hour grid and 29 on an 8-hour grid.

**Three controls, and the third is the one that makes the first two mean
anything.** A: the replay's derived totals equal the real `screenLevel`'s, in
process, 244/244. B: the real `ScreenPair` output equals the committed
`ranking.json`, 244/244. C, negative: a replay at the *wrong* entry level must
**disagree** — 248 ≠ 131. Without C, A and B are two agreements nobody showed
could fail, which is this repo's standing trap in its most ordinary dress.

The run was reproduced from scratch later in the session after the working
directory was lost, and returned byte-identical counts (61 / 244 / 244 / 17 478,
and C's 248 ≠ 131). That is not a claim about correctness; it is a claim about
determinism, and it is worth one line because the first run's artifacts were
gone by the time the record was written.

## 3. What persistence actually is

Pooled over all 61 symbols, at the 10bp entry level: mean run **2.57**
settlements, median **2**, p90 **5**, max **20**. Split by grid, in hours:

| Grid | p50 | p75 | p90 | max |
|---|---|---|---|---|
| 4-hour (32 symbols) | 8h | 12h | 24h | 80h |
| 8-hour (29 symbols) | 16h | 24h | 40h | 136h |

So the handoff's ≈6.2-settlement break-even sits at roughly the **p93** of the
run-length distribution. The typical episode does not reach it. That is the
answer to the question §2.3 asks, and on its own it is not yet a verdict — the
next three sections are why.

## 4. F26 — the round trip is a fee-only constant, and it decides the verdict

`RoundTripCost()` is `2×0.0005 + 2×0.00055 = 0.0021`: **four taker legs and
nothing else**. No spread crossing, no slippage. Every `net`, `net_annual`,
`wins` and `RankScore` in `ranking.json` — and therefore the shortlist itself —
is scored against it.

`build-spec.md:53` states the cross-exchange round trip as **~0.29%**. §98
observed **0.90%** on the one live run and says explicitly that this is *not a
measurement*. Mean net per episode, per unit of one leg's notional:

| Entry | Episodes | Mean gross | @21bp | @29bp | @90bp |
|---|---|---|---|---|---|
| 2bp | 12 322 | 0.000653 | −0.001447 | −0.002247 | −0.008347 |
| 5bp | 3 425 | 0.001519 | −0.000581 | −0.001381 | −0.007481 |
| 10bp | 1 259 | 0.002613 | **+0.000513** | −0.000287 | −0.006387 |
| 20bp | 472 | 0.003678 | **+0.001578** | **+0.000778** | −0.005322 |

Two entry levels are profitable at 21bp, one at 29bp, **none** at 90bp. The
screening measured the spread across 466 pairs and 235 073 settlements and never
varied the one input on the other side of the subtraction.

## 5. F27 — 84% of the net is one symbol

At the 10bp level and 21bp cost, summed across 56 symbols that produced an
episode there:

```
total net                 +0.6460
  LABUSDT alone           +0.5439   (84%)
  the other 55 symbols    +0.1021
drop the best 10 of 1259 episodes  ->  −0.0388
```

Per symbol, annualised on $2 000 of capital ($1 000 per leg):

| Cost | Median symbol | Best | Worst | Positive | Beat a 4.6% hurdle |
|---|---|---|---|---|---|
| 21bp | **−0.1%/yr** | +35.8% | −4.0% | 16/56 | **4/56** |
| 29bp | −0.2%/yr | +32.1% | −5.8% | 12/56 | 3/56 |
| 90bp | −1.5%/yr | +3.6% | −24.6% | 1/56 | **0/56** |

The hurdle column is the gate's actual question and the 4.6% is illustrative,
per §55's rule that the benchmark is a dated, sourced, haircut-adjusted input
and never a hardcoded constant.

⚠ **The sign test above runs on the raw net, not on the rounded percentage.**
Three symbols (`NEARUSDT`, `WIFUSDT`, `HBARUSDT`) earn $1.63–$2.53 across 3–5
years, which is positive and renders as `+0.0%/yr`; counting on the rendered
figure gave 15/56 and 10/56. Caught by comparing two passes of the same
analysis. A rounding applied for display became a predicate.

## 6. F28 — run length and break-even are two different questions

At 5bp entry and 21bp cost, by run length:

| Run | Episodes | Cleared the round trip |
|---|---|---|
| 1 | 1648 | 0.0% |
| 2 | 746 | 2.5% |
| 3 | 341 | 15.0% |
| 4 | 207 | 38.6% |
| 5 | 130 | 73.8% |
| 6 | 92 | 87.0% |
| 7 | 71 | 98.6% |
| 8+ | 190 | 100.0% |

Run length is strongly predictive and is **not** the criterion: runs 2 through 7
are a band where both outcomes occur, because the spread varies *within* an
episode. A short intense dislocation can pay where a long weak one does not.
An edge bar written as "hold for N settlements" would misclassify in both
directions across five of the eight rows.

## 7. F29 — the edge lives entirely in short histories, and no fixture separates the two readings

| Symbol | Days | @21bp |
|---|---|---|
| LABUSDT | 277 | +35.8% |
| BEATUSDT | 252 | +13.0% |
| KAITOUSDT | 526 | +7.1% |
| MMTUSDT | 269 | +4.8% |
| BCHUSDT | 2055 | +0.8% |
| LTCUSDT | 2109 | +0.5% |
| LINKUSDT | 2109 | +0.5% |
| SOLUSDT | 1858 | −0.1% |

Every high performer has under 530 days of paired history. Every symbol with
over 1 800 days returns approximately zero. Two readings fit this exactly:
**(a)** a real regime in new, thin, high-funding alts, and **(b)** small-sample
dispersion, with the long series showing the true value.

This is **F25's family**: both readings are the same function of every quantity
the experimenter controls. The separating parameter is *elapsed time*, and
unlike F25's notional it cannot be varied at all — only waited out. The
conclusion is therefore not "design a better fixture" but "this measurement
cannot answer it, and a claim that it does is the finding."

## 8. What is and is not claimed

**Claimed.** The run-length distribution, the run-vs-break-even table and the
per-symbol nets are exact readings of the committed screening data, reproduced
from the artifact and checked by three controls. The round-trip constant is
fee-only; that is a fact about `params.go:104`.

**Not claimed.** Nothing about mainnet costs — 21bp, 29bp and 90bp are an
assumption, a model, and one unattributed live observation. Nothing about
capital: the totals assume every episode is taken simultaneously at unit
notional, which is a sum and not a portfolio. Nothing about whether the
short-history edge is real (§7). Selecting the best entry level is mild
selection on the answer and the screening already carries that caveat
(`params.go:126`); this record inherits it.

## 9. The operational finding

`research/screening/raw/` is gitignored on a sound argument (§42b: hundreds of
megabytes of venue words, re-derivable over REST, sha256-pinned in the committed
`ledger.json`). The consequence, which nobody had needed until now, is that a
**derived** artifact over that store is the only durable form of the measurement
on a fresh clone. `episodes.json` is therefore committed *because* its input is
not, and `README.md` states the inversion so the next reader does not delete it
as regenerable.

## 10. What this leaves open

The next question is **not** more spread measurement. Three of the four findings
turn on the cost, and the cost is the quantity this project has measured least:
one live figure, confounded between a four-tick buffer and 77 seconds of drift,
which §98 already refused to carry into a hurdle calculation. **A clean
round-trip figure requires the send to follow the book read inside the book's
timescale** — a property of an automated path, not of a runbook a person
executes.

Until that exists, the gate's answer is a function of an assumption rather than
of a measurement, and §4's table shows the assumption spans the whole verdict.
