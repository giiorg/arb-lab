# datasets/ — committed calibration inputs (step 4, §41c)

This directory holds the **family-1 calibration datasets**: venue REST
history, fetched once, committed verbatim. It is a sibling of `configs/`
because the files are *inputs the derivation reads*, not configuration the
process loads.

## The rules (from the 2026-07-31 calibration-artifact design record, §41c)

- **One file per venue per series**, named
  `datasets/<venue>/<series>.<symbol>.<start>.<end>.json` — e.g.
  `datasets/binance/funding.BTCUSDT.2023-01-01.2026-01-31.json`.
- **The venue's words, verbatim.** A file holds what the venue's REST API
  returned, dated, with no local transformation — the
  venue-published-vs-locally-derived rule: for funding and settlement
  facts, the venue's computation *is* the definition.
- **Closed identities (rule H3).** Every file covers a declared
  `[start, end]` range completely: every settlement/kline the venue serves
  inside the range, pagination exhausted. The calibrate tool refuses a file
  with a missing **or extra** row (`driving.CheckComplete`).
- **Calibration era only (rule H1).** No row dated on or after
  **2026-02-01T00:00:00Z** may appear here; the fetchers refuse them
  (`driving.GuardCalibrationInput`). Holdout-era history is fetched only by
  step 5's scoring machinery, under the §41h seal.
- **Digest-enumerated.** Each file's SHA-256 (lowercase hex) is recorded in
  the calibration artifact's dataset block; the derivation refuses to emit
  if a re-hash disagrees (`driving.CheckDatasetDigest`).
- **Write-once.** A corrected or extended dataset is a NEW file feeding a
  NEW artifact version — never an edit to one an artifact already cites.

Book data is deliberately **not** here: the capture archive owns book
bytes, and the artifact cites archive manifest digests instead (rule H2).

## The committed roster (slice B, fetched 2026-08-01)

| File | Rows | Grid |
|---|---|---|
| `binance/funding.BTCUSDT.2023-01-01.2026-01-31.json` | 3381 | 8h, 00:00→16:00 UTC |
| `bybit/funding.BTCUSDT.2023-01-01.2026-01-31.json` | 3381 | 8h, 00:00→16:00 UTC |
| `binance/spot_kline_1h.BTCUSDT.2023-04-01.2026-01-31.json` | 24888 | 1h open times |
| `binance/mark_kline_1h.BTCUSDT.2023-04-01.2026-01-31.json` | 24888 | 1h open times |

Digests, row counts and era bounds are PINNED in
`internal/strategy/infrastructure/driving/committed_test.go`, so an edit
to any of these files is a build break, not a review hope.

**The kline era starts 2023-04-01, not the funding era's 2023-01-01.**
Binance served no spot kline for 2023-03-24T13:00Z (that day's spot
halt), and a closed identity cannot claim an instant the venue never
served — the completeness refusal fired live on the full range, which is
rule H3 doing its job. Both kline series share one era because the basis
derivation subtracts them pairwise. Bybit klines are deliberately absent:
no derivation consumes them this milestone (the §41h seal fixes the
scored venue to Binance), and the fetcher refuses them by name.
