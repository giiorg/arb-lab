# Strategy #2 capture expansion — milestone 2b step 2 landed (2026-08-01, session 57)

**Status: COMPLETE (config landed 2026-08-01 11:13Z; §32m-D figure measured
2026-08-02, three hours inside its 48-hour deadline) — the capture runs 40
sources (PID 8495), all 32 new alt streams delivered first data, every
pre-existing source minted its honest `process_restart` gap record, and the
per-tier one-day figure is §44e below: **11.165 GiB/day, BTC tier 4.314 / alt
tier 6.852, 0 integrity over the measured day.** §42c's derived per-symbol
bound is refuted in both directions and its premise with it; **§38f's
destination decision is back with the user on a measured clock** (85 days to a
full disk, the gate at day 59).**

Mandate: scoping record §42's RESUME, step 2, as re-stated by screening
record §43's RESUME HERE — the §43d shortlist × §42c's four streams into
`configs/capture.json`, a restart with the honest `process_restart` gap
record, and the §32m-D one-day measurement within 48h. The surface itself
was ruled in §42c/§42f and is **not re-decided here**: no spot legs, no
per-alt diff evidence, no third venue, no broad funding tier, BTC's eight
streams unchanged. What this record adds is the pre-config verification
§42c demanded, the config shapes actually chosen, and the measured restart.

## §44a — the pre-config item: per-IP WebSocket ceilings, verified

§42c's operational note — one source = one connection = one topic, so the
expansion's connection count is the config's source count — made the venue
per-IP connection ceilings a profiles verification item **before** any
config lands. Verified 2026-08-01, full facts and URLs in the profiles'
dated block (the normative home; this is the reading):

- **Bybit** states the only numeric ceiling that binds a restart: ≤500 new
  connections per 5 minutes **per WebSocket domain**. Our restart burst is
  19 against it (18 linear + 1 spot).
- **Binance spot** states 300 connections per attempt per 5 minutes per IP;
  we make 1.
- **Binance futures states no numeric per-IP ceiling at all** — the bound is
  behavioral ("IPs that are repeatedly disconnected may be banned"), and the
  capture's per-source backoff ladder (1s→60s) is exactly the pacing it
  asks for. 19 concurrent connections with paced reconnects is well inside
  any reading of a policy whose stated concern is churn, not count.

The verification's real finding is therefore an asymmetry: the shortlist
tier clears every **documented** bound by an order of magnitude, but the
venue we open the most connections to is the one with no number — so "39
connections is fine" rests on the behavioral reading plus the observed fact
that 7 connections have run for two days with paced reconnects and no ban.
A broad funding tier re-runs this arithmetic at 2N sockets and stays gated
on §42c's own pricing.

## §44b — the config: 32 new sources, every shape a copy

`configs/capture.json` grows from 8 sources (7 WS + 1 REST) to 40
(39 WS + 1 REST): per shortlisted symbol, Binance `<sym>@markPrice@1s`
(prefix `/market/ws`) and `<sym>@depth20@100ms` (prefix `/public/ws`),
Bybit `tickers.<SYM>` and `orderbook.50.<SYM>` (both `/v5/public/linear`).

The deliberate choice is that **nothing per-symbol was chosen**: every
field of every new source is its BTC counterpart's value with only the
symbol substituted — hosts, prefixes, protocols, and each stream's
`first_data_timeout` (10s, except Bybit tickers' 30s, kept because the
reason it was 30s for BTC — push-on-change has no guaranteed first-message
deadline — is symbol-independent). Rejected alternative: re-deriving
timeouts per symbol from activity, which would encode screening-time
activity into an operational config that outlives it, for streams whose
subscribe-time behavior (Bybit sends an initial snapshot on both topics;
Binance's markPrice is fixed-rate 1/s, depth20 fixed-rate 10/s) does not
vary with activity in the first place. **[REFUTED IN PART by §44e
(2026-08-02): `depth20@100ms` is NOT fixed-rate — 100 ms is a floor between
pushes and the stream is change-driven, so its 10 s first-data deadline is a
bet on activity. Measured consequence and the recommendation are in §44e; the
other two clauses hold.]**

BCHUSDT is the one 8h-grid symbol on the shortlist (the other seven are
4h); the capture does not know or care — grids matter to pairing at read
time, not to byte capture.

Generation was scripted from the shortlist, not hand-typed, with a
uniqueness assertion over (venue, product, symbol, stream, topic); the
config parses under the same loader `capture run` uses (exercised via
`capture verify -config` on the new file before the restart).

## §44c — the restart, measured

The sequence, each instant from the shell that ran it (2026-08-01):

1. **Pre-restart ritual verify** over the full accumulated range
   (`-from 2026-07-30T00:00:00Z -to 2026-08-01T10:55:56Z`): **verdict
   COVERAGE — 0 integrity, 639 coverage findings**, all of them the known
   honest thin windows (the pre-start hours of Jul 30, the ~23:00–04:15
   lid-closed window, reconnect churn). The gate that authorizes touching
   the writer.
2. **SIGTERM at 11:13:02Z** to PID 74192 (up 1d 19h); `stopped cleanly` in
   the log within a second.
3. **Start at 11:13:10Z**, PID **8495**: `capture run: 40 sources under
   ./archive (config 1c362feffe4a, build 4ae7d1725aab…)`. The stop-to-start
   gap is **~8 seconds of wall clock**, recorded rather than hidden.
4. **First data: all 40 sources** (39 WS + the hourly `fundingInfo` REST
   overlay) had a non-thin current-hour segment within two minutes; every
   shortlisted symbol shows 4/4 stream directories.
5. **The honest gap is on disk:** all 8 pre-existing BTCUSDT sources'
   current-hour files carry a `process_restart` gap record (the writer
   mints it at reopen — §32j's restart-required trade working as designed).
   The alt sources are new streams, so their history simply begins at
   11:13Z; there is nothing before it to bridge, and §42a already named
   everything before the shortlist existed as un-backfillable by anyone.

One build fact worth its sentence: the new process runs the **current**
`bin/arblab` (embedded revision `4ae7d17`, on-disk mtime 09:50 same day —
replaced during session 56 while PID 74192 kept running, which a Go
rename-style build permits; the old process was on `59d5371`, the v0.9.15
build). The capture subsystem is unchanged between those revisions; the
standing never-rebuild-over-the-running-binary rule stays exactly as
written, because rename-survival is luck of the tooling, not a guarantee.

## §44d — what is now due, with dates

- **§32m-D per-tier one-day figure by 2026-08-03** (48h from the config
  landing): `du` roster split BTC-tier vs alt-tier, plus the full-day
  verify over the measured day. This is also **§38f's named revisit point**
  (ruling §42f-3): at §42c's derived bound the expansion compresses the
  off-host disk clock ~3×, and the measured figure decides whether the
  deferral survives. **[DISCHARGED 2026-08-02 — §44e. The deferral does not
  survive intact: the measured clock puts the gate inside the disk's own
  window.]**
- The `strategy/crossvenue` read-path, the research simulator, and the
  gate's procedure commit (§42e) — after data flows; gate parameters are
  pinned there, never here. §43b's selection-contamination statement is a
  named holdout input there.

## §44e — the per-tier one-day figure, measured (2026-08-02, session 72)

**The headline: 11.165 GiB/day for all 40 sources over a fully covered day.
BTC tier 4.314, alt tier 6.852 — 0.387 to 1.743 GiB/day per shortlisted
symbol, mean 0.856. The full-day verify over exactly that day reports
0 integrity / 50 coverage, verdict COVERAGE.** Units are GiB throughout, to
match §32m-D's `du` figures; in decimal GB the day is 11.99.

### The window, and why it is not the restart anniversary

The measured day is **24 whole hours, 2026-08-01T12:00Z → 2026-08-02T12:00Z**,
not the 11:13Z→11:13Z anniversary the deadline is written against. The reason
is the comparison the figure exists to make. The 11:00Z hour holds ~11 minutes
of the *previous* process for the eight BTC sources and only ~47 minutes of
life for the 32 new alt sources — the restart-hour verify (below) reports
exactly that: 8 `process_restart` gap records and 32 `alive_short` findings at
47/60. An anniversary window would therefore have added ~34 MiB of a dead
process to the BTC tier and dropped ~63 MiB from the alt tier, both in the
direction that flatters the tier being tested. The bias is ~1% and would not
have changed a conclusion; the window is still chosen so the two tiers are
measured over **identical, complete, single-process hours**, and the day is
the same 24 hours long, one hour later.

**The day is fully covered, and that is verified rather than assumed:** the
verify emits 960 per-hour source lines (40 × 24) and **every one reads
`alive 60/60`**. This is what makes the figure comparable to a healthy day —
unlike the 2026-07-31 measurement, which contained the lid-closed
~23:00–04:15Z window and was labeled an under-measure at the time.

### The roster

BTC tier — the eight streams that ran before the expansion, unchanged:

| Stream | MiB/day | data records/day | B/record | msg/s |
|---|---|---|---|---|
| binance perp `btcusdt@depth@100ms` (diff evidence) | 1265.4 | 842,786 | 1574 | 9.75 |
| binance spot `btcusdt@depth20@100ms` | 1120.0 | 862,141 | 1362 | 9.98 |
| binance perp `btcusdt@depth20@100ms` | 768.5 | 776,420 | 1038 | 8.99 |
| bybit perp `orderbook.50.BTCUSDT` | 678.9 | 2,428,085 | 293 | 28.10 |
| bybit spot `orderbook.50.BTCUSDT` | 448.1 | 1,820,173 | 258 | 21.07 |
| bybit perp `tickers.BTCUSDT` | 115.4 | 529,157 | 229 | 6.12 |
| binance `btcusdt@markPrice@1s` | 18.0 | 86,400 | 219 | 1.00 |
| binance `fundingInfo` REST overlay | 3.0 | 0 (1,512 records) | — | hourly |
| **BTC tier** | **4417.3 (4.314 GiB)** | 7,345,162 | | |

Alt tier — the shortlist, four streams each, largest first:

| Symbol | bybit `orderbook.50` | binance `depth20@100ms` | bybit `tickers` | binance `markPrice@1s` | Total | GiB/day |
|---|---|---|---|---|---|---|
| BEATUSDT | 959.5 | 660.1 | 148.5 | 16.8 | 1784.9 | **1.743** |
| KAITOUSDT | 747.9 | 635.4 | 71.0 | 16.9 | 1471.2 | 1.437 |
| MMTUSDT | 368.9 | 443.2 | 53.5 | 16.8 | 882.4 | 0.862 |
| ESPORTSUSDT | 345.3 | 388.0 | 53.4 | 17.0 | 803.7 | 0.785 |
| LABUSDT | 206.8 | 357.0 | 54.1 | 16.7 | 634.6 | 0.620 |
| BCHUSDT | 120.2 | 399.6 | 38.3 | 17.4 | 575.5 | 0.562 |
| SOONUSDT | 192.6 | 230.2 | 27.3 | 16.8 | 466.9 | 0.456 |
| MUSDT | 66.6 | 299.0 | 14.5 | 16.5 | 396.6 | **0.387** |
| **alt tier** | | | | | **7015.8** | **6.852** |

Exact, so the rounding is checkable: **BTC 4,631,822,128 B + alt 7,356,773,820
B = 11,988,595,948 B**. Each tier is rounded to three decimals independently,
so the two GiB figures sum to 11.166 while the day is 11.165.

**Books are 10.450 GiB of the 11.165 (93.6%); every funding stream on both
venues together is 0.715 GiB (6.4%).** That ratio is the §38f argument in one
line: the un-backfillable half is essentially the whole burn.

### The verify, both runs

1. **The measured day** (`-from 2026-08-01T12:00:00Z -to 2026-08-02T12:00:00Z`):
   **0 integrity, 50 coverage, verdict COVERAGE (exit 2)**. The 50 split
   **26 `gap_record` reconnects** (clustered in two hours — 2026-08-01T17Z and
   2026-08-02T09Z — across many sources at once, i.e. local network blips, not
   per-source faults; one is a `no_first_data` on BTC's perp `depth20` at 09Z)
   and **24 `restart_boundary` on `fundingInfo`, one per hour** — task #153's
   chronic marker, which on a clean day is **48% of all findings**. That number
   is a new input to #153's ruling and is recorded here rather than argued.
2. **The restart hour** (`-from 2026-08-01T11:00:00Z -to 2026-08-01T12:00:00Z`),
   run because §44d promised this figure would show the restart honestly:
   **0 integrity, 42 coverage** — **8 `gap_record: process_restart`** (exactly
   the eight pre-existing BTC sources, §32j's restart-required trade) plus
   **32 `alive_short`** (exactly the 32 new alt sources, alive 47/60 because
   they began at 11:13Z) and 2 `restart_boundary`. Every one of the 40 sources
   reports the true thing about itself, and the 8/32 split is the tier split.

### What §42c derived, and what the measurement says about it

§42c priced a symbol at **~0.6–1.1 G/day, derived not measured**, on a stated
two-part premise: *"the two fixed-rate Binance streams contribute ~450M/day
regardless of symbol activity; the two Bybit streams scale down with activity
by an unmeasured factor."*

**The aggregate lands inside the derived band and the premise under it is
false** — which is the finding, because an aggregate agreeing is not evidence
that the model producing it was right. §42f-1 priced ~8 symbols at +5–9 G/day;
measured **6.852**. Mean per symbol 0.856, inside 0.6–1.1. But:

- **No symbol-independent floor exists.** Binance `depth20@100ms` measured
  **230.2 MiB (SOON) to 768.5 MiB (BTC)** against the derived "~441M
  regardless", and `markPrice@1s` is 16.5–18.0 MiB, not 11M. MUSDT lands at
  **0.387 GiB/day — 35% below the derived lower bound**.
- **There is no BTC-anchored ceiling either.** "Alts likely lower" fails for
  the top two: BEAT's `orderbook.50` is **959.5 MiB against BTC's 678.9**, and
  BEAT's `tickers` beats BTC's too (148.5 vs 115.4). BEAT lands at **1.743
  GiB/day, 1.58× the derived upper bound** — 40% of what all eight BTC streams
  cost together.
- So the measured spread is **4.5×** (0.387 → 1.743) where the derived band was
  1.8× wide, and it is wrong at **both** ends rather than shifted.

### The mechanism, measured rather than inferred: "update speed" is a ceiling

Bytes/day collapses two factors, so the roster above carries **records/day and
bytes/record** beside it — and the split is decisive:

- **Binance `@depth20@100ms` is change-driven, not fixed-cadence.**
  Bytes/record is essentially constant across the nine perp streams
  (**966–1132 B** for 20 levels each side; the spot book is a fatter 1362 B and
  is not in that comparison), while the rate over those same nine runs
  **2.68/s (SOON) to 8.99/s (BTC)** against the venue's 10/s ceiling — 9.98/s
  on BTC spot, the closest anything comes. Nobody saturates it. Counted over all 24
  hours rather than one sampled hour: MUSDT 324,748 records (mean 3.76/s, range
  1.8–6.1 hour to hour), BEAT 695,640 (8.05/s).
- **Bybit `orderbook.50` is the same shape one step up** — profiles record a
  20 ms push frequency (50/s); measured 30.53/s (BEAT), 28.10/s (BTC), 3.33/s
  (MUSDT).
- **`markPrice@1s` is the one genuinely fixed-cadence stream**: exactly
  **86,400 data records/day for seven of nine symbols**, ~204–219 B each. It is
  the positive control for the whole reading — it shows the class of stream
  §42c assumed *exists*, and that the depth stream is not in it.

**"The venue did not send" and "we dropped it" produce the same byte count, so
the reading was not left on a rate alone.** On one hour of the quietest symbol
(MUSDT `@depth20@100ms`, 6,387 records), every message's `pu` equals its
predecessor's `u` — **zero chain breaks**, so nothing was lost between the
venue and the disk — while the **minimum observed inter-message interval is
102 ms**, the cap honoured exactly, the median is 156 ms, p90 is 1,254 ms and
the maximum silence in that hour is 22.5 s (94.8 s over the full day). The
per-message update-ID span runs from **1 to 1,297,023**: each push carries
everything accumulated since the last one, which is what coalescing under a
rate cap looks like from the receiving end. A lossy capture would have shown
the opposite signature — broken `pu` chain, intervals *below* 102 ms absent by
accident rather than by cap.

So §42c's error was not arithmetic. It read a venue's documented **update
speed** as a delivery cadence when it is a **maximum**: both book streams push
only when the book changed, so per-symbol disk cost is activity all the way
down, on all four legs. This is recorded as a measured wire fact in the
profiles' dated block, because the next consumer to assume "fixed rate" would
be the read path.

### The same refuted premise underwrites an operational setting — §44b's `first_data_timeout`

§44b justified copying BTC's `first_data_timeout` to all 32 new sources with
the sentence the measurement now refutes: *"subscribe-time behavior (Bybit
sends an initial snapshot on both topics; Binance's markPrice is fixed-rate
1/s, depth20 fixed-rate 10/s) does not vary with activity in the first place."*
It is right about two of the three and wrong about the one that matters.
**Binance `@depth20@100ms` has no subscribe-time snapshot and pushes only on
change, so its 10 s first-data deadline is a bet on activity** — exactly the
bet Bybit `tickers` was given 30 s to avoid, for a reason (*"push-on-change has
no guaranteed first-message deadline"*) that applies here word for word.

Measured over the same day, per-message spacing on the two quietest Binance
book streams:

| Stream | max silence | p99 | intervals > 10 s | P(no message in the next 10 s)¹ |
|---|---|---|---|---|
| MUSDT `@depth20@100ms` | **94.8 s** | 2.93 s | 206, covering 3.42% of the day | **1.03%** |
| SOONUSDT `@depth20@100ms` | 57.4 s | 2.77 s | 10, covering 0.18% of the day | 0.06% |
| MUSDT `orderbook.50` (Bybit, for contrast) | 45.9 s | 4.98 s | 492, covering 7.88% | 2.18% — **not exposed**, Bybit snapshots at subscribe |

¹ Σ(gap − 10 s)⁺ ÷ day, i.e. the fraction of instants from which a fresh
subscription would see nothing for 10 s.

So **~1 reconnect in 100 on the quietest alt's Binance book would trip its own
first-data deadline and reconnect again**, on a stream that is behaving
perfectly. It self-recovers through the per-source backoff ladder and it has
not bitten: the measured day carries exactly **one** `no_first_data` record,
and it is on **BTCUSDT** during the 09Z blip cluster, not on a quiet alt.
**Recorded, not fixed:** `first_data_timeout` is restart-required by
construction (§platform/config's default-deny hot reload), and a capture
restart is an operational act with its own gap record — so the recommendation
is to raise the nine Binance `depth20` sources to 30 s **at the next restart
that happens for another reason**, and not to restart for this alone. The
funding side needs nothing: `markPrice@1s` really is 1/s.

### An instrument that fell out of it

**A fixed-cadence stream converts bytes into a coverage measurement**, and it
is the sharpest one this archive has:

- On the two hours that carried reconnects, `markPrice` delivered 86,297
  (BEAT) and 86,352 (MMT) against 86,400 — i.e. the reconnects cost exactly
  **103 and 48 seconds** of funding data. **Both hours read `alive 60/60`.**
  That is not a verify defect — `alive` counts minutes touched, by definition —
  but it is a limit worth writing down where §45's decision-grid joins can see
  it: a source can lose a hundred seconds of a 1/s stream in an hour that
  scores full coverage.
- Applied backwards, it re-reads the 2026-07-31 figure: `markPrice` measured
  **11M there against 18.0 MiB here**, so that day was **~61% covered**, and
  the "naive scale-up to full wall coverage" that produced 3.6–3.8 G/day used
  a ~1.2× multiplier where the instrument says ~1.64×. The BTC tier measures
  **4.314 GiB/day**, ~14% above that projection — and the decomposition is
  honest in both directions: 3.0 GiB at 61% coverage projects to ~4.9, so this
  day's BTC *activity* was actually lower than that day's, and the two errors
  partly cancelled. **A one-number scale-up cannot separate coverage from
  activity, which is exactly why the ritual measures instead of deriving.**
- It also caught a defect in **this session's own reading tooling**: the first
  join reported BEAT's `markPrice` at 79,200/day (91.67%), a suspiciously exact
  22/24. The cause was the parser matching only `replayable` hour lines and
  silently dropping `gapped` ones — the interesting hours. Had the stream not
  had an exact expected count, the false figure would have shipped.

### §38f revisited on the measured clock (ruling §42f-3's named point)

The deferral was re-taken on 2026-08-01 with a derived number (~3 months) and
**this measurement as its named revisit**. The measured arithmetic, all on
today's `df`: **954 GiB free, 11.165 GiB/day, archive 18.9 GiB today.**

- **85 days to zero free** (≈2026-10-26); **76 days to a 100 GiB floor**, which
  is the operative limit — the archive shares the disk with the machine's own
  needs, and the failure at zero is not "capture stops", it is macOS
  misbehaving while capture keeps writing.
- **The gate is inside that window, not after it.** BCHUSDT binds ~2026-09-30,
  59 days out: **+659 GiB, leaving ~295 GiB free, with ~678 GiB of
  un-backfillable archive on a single laptop disk** at the moment the project's
  next decision is taken. §42c's "~3 months of local disk" survives as an
  order of magnitude and dies as a plan: the gate wait alone consumes 69% of
  today's free space.
- **§38f's premium estimate was half the real rate** — it priced the insurance
  at "~6 GB/day and a one-line rsync". Measured is 11.165 GiB/day ≈ 12.0 GB/day.
  The rsync line is unchanged; the *sizing* is not. A target must hold ~678 GiB
  by the gate, so the session-64 recommendation stands in kind but not in
  capacity: **≥2 TB**, not 512 GB. The copy itself remains cheap — ~1–2 hours
  for the first full sync over USB 3, ~2 minutes/day incremental.
- **The risk being re-accepted, restated with the measured number:** a disk
  loss now costs the book history since 2026-07-30 and restarts an
  un-backfillable clock that the gate's own date depends on. Funding *settled*
  rates survive it (public REST); 93.6% of the daily bytes do not.

**Recommendation, for the user's ruling:** take the off-host target now rather
than at a third deferral — the measured clock says the deferral's own margin is
spent by the gate, and every day of waiting adds 11.2 GiB that no vendor
replays. What is *not* recommended, and stays refused for the reasons already
recorded: compression (§38g — a codec between the digest and the bytes) and
retention/pruning (§38g — a deletion policy over data whose value is unknown
until the gate rules).

### What this figure does not measure, stated so it is not over-read

- **It is one day, and activity is the whole variable.** MUSDT's own hourly
  rate moves 3.4× within this day. The tier totals are a sample from a
  distribution whose driver is market activity, not a constant.
- **A fully covered day is the high end of accumulation.** Days with the lid
  closed accumulate less, so 85 days to a full disk is a *conservative*
  (short) estimate of the time available — the right direction for a decision
  about protecting the archive.
- **It says nothing about read-side cost.** Verify over this day is
  single-threaded and took ~27 minutes at 99% of one core for 11 GiB / 23.8M
  records;
  that is the ritual's own price and it grows with the archive, not with the
  day.
- **It re-prices the broad funding-only tier (§42f-2, ruled none) without
  reopening it:** derived was 20–110M/day/symbol; measured funding-only is
  **31.0–165.3 MiB** (BTC 136.4). A 30-symbol broad tier is ~0.9–4.8 GiB/day
  measured, against ~0.6–3.2 derived. The ruling stands; the number is now
  measured for whoever revisits it.

## RESUME HERE

Step 2 of milestone 2b is **COMPLETE**: the shortlist's 32 streams flow, the
restart is recorded honestly, the capture runs 40 sources as PID 8495, and
**§44e's per-tier figure is measured, three hours inside its 48-hour
deadline**. Next, in order:

1. **The §38f destination is a live user decision again** — §44e re-argues it
   on the measured clock (85 days to a full disk; the gate at day 59 with
   ~678 GiB of un-backfillable archive on one disk) and recommends taking a
   **≥2 TB** target now. This is the third time it has been put; the two prior
   deferrals were both taken against derived numbers.
2. **The `strategy/crossvenue` read-path, the research simulator, and the
   gate's procedure commit** (§42e) — **all landed** (v0.9.23, §45); what
   remains is task #152, accumulating gate evidence until BCHUSDT's 180
   matched post-cutoff settlements (~2026-09-30), then `arblab crossvenue gate`.
3. **Task #153** (the hourly `restart_boundary` on `fundingInfo`) now has a
   measured weight: **24 of the 50 findings** on a clean day, and it scales
   with every REST-overlay source added.

Capture ritual unchanged, with the new facts: `pgrep -fl "arblab capture
run"` expects **PID 8495**, the config hash is `1c362feffe4a`, and a verify
over any range crossing 2026-08-01T11:13Z will honestly show the restart
gap. H1 stays sealed. Marker grep zero.
