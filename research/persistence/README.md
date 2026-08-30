# research/persistence/ — the cross-venue spread persistence measurement (record §99)

`§2.3` sets the edge bar from **measured spread persistence, not the headline
spread**. `research/screening/ranking.json` measures 466 venue pairs but
publishes **totals** per entry level — `episodes`, `wins`, `accrued_settlements`,
`max_run`. It does not publish the **length of each episode**, so neither
"how long does a spread survive" nor "what happens at a round trip other than
`RoundTripCost()`" is answerable from it.

This directory holds the per-episode expansion and its summary.

| File | Holds |
|---|---|
| `harness.go.txt` | The emitter. **Not part of the build** — copied into the `screening` package to run. |
| `episodes.json` | Every episode: settlements collected, spread accrued (exact decimal), start and end instant. 17 478 episodes, 61 symbols × 4 entry levels. |
| `summarise.py` | Derives `summary.json` and prints the report. Exact `Decimal` throughout. |
| `summary.json` | Pooled run-length percentiles, the run-vs-break-even table, and per-symbol net at three round-trip costs. |
| `verdict_levels.py` | **F60's measurement.** What `verdict` would say at each of the four entry levels the file declares, with a control and a negative control. `verdict` is pinned to `0.001` and records no level. Re-run it to verify §100 §14 debt 10's fix. |

## Reproducing

```
cp research/persistence/harness.go.txt \
   internal/strategy/infrastructure/screening/zz_persistence_test.go
go test ./internal/strategy/infrastructure/screening/ -run TestPersistence -v
rm internal/strategy/infrastructure/screening/zz_persistence_test.go
python3 research/persistence/summarise.py
```

Expect `screened=61 controlA=244 controlB=244 episodes=17478`.

**`episodes.json` is committed because its INPUT is not.**
`research/screening/raw/` is gitignored (468 verbatim venue files, §42b's
argument: re-derivable over REST, sha256-pinned in the committed
`ledger.json`). That argument is sound for the raw store and it makes this
derived file the only durable form of the measurement on a fresh clone — so
the derived artifact is committed precisely because the thing it derives from
is not. Regenerating it needs the raw store, i.e. a re-fetch.

## The three controls

The harness is a second reading of `screenLevel`'s state machine. Nothing
about the extra per-episode detail is trustworthy unless the reading agrees
with the original, so:

| | Claim | Result |
|---|---|---|
| **A** | `replayEpisodes`' derived totals == the real `screenLevel`'s, in process | 244/244 |
| **B** | the real `ScreenPair` output == the committed `ranking.json` | 244/244 |
| **C** | *negative* — replay at the WRONG entry level must **disagree** | 248 ≠ 131 |

C is not decoration. A and B are two agreements, and an agreement that was
never shown capable of failing is the repo's standing trap: a universally
agreeing comparison reads as a perfect score.
