# Strategy #2 capture expansion — milestone 2b step 2 landed (2026-08-01, session 57)

**Status: LANDED (2026-08-01 11:13Z) — the capture runs 40 sources
(PID 8495), all 32 new alt streams delivered first data, and every
pre-existing source minted its honest `process_restart` gap record. The
§32m-D per-tier one-day figure is due by 2026-08-03T11:13Z and lands as
§44e below.**

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
vary with activity in the first place.

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
  deferral survives.
- The `strategy/crossvenue` read-path, the research simulator, and the
  gate's procedure commit (§42e) — after data flows; gate parameters are
  pinned there, never here. §43b's selection-contamination statement is a
  named holdout input there.

## RESUME HERE

Step 2 of milestone 2b is LANDED: the shortlist's 32 streams flow, the
restart is recorded honestly, and the capture runs 40 sources as PID 8495.
Next, in order:

1. **The §32m-D per-tier one-day figure, due by 2026-08-03T11:13Z** — lands
   as §44e in THIS record: `du` split BTC-tier vs alt-tier (the §42c bound
   to check: ~0.6–1.1G/day/symbol, the two Bybit streams the unmeasured
   scale-down), plus the full-day verify over the measured day (which will
   show this restart's `process_restart` gaps and must still report
   0 integrity). That figure is **§38f's named revisit point** (ruling
   §42f-3): with the measured number in hand, the off-host decision gets
   re-argued on a measured disk clock, not the derived one.
2. **The `strategy/crossvenue` read-path, the research simulator, and the
   gate's procedure commit** (§42e) — gate parameters pinned THERE, never
   here; holdout declared at that commit with §43b's
   selection-contamination statement as a named input.

Capture ritual unchanged, with the new facts: `pgrep -fl "arblab capture
run"` expects **PID 8495**, the config hash is `1c362feffe4a`, and a verify
over any range crossing 2026-08-01T11:13Z will honestly show the restart
gap. H1 stays sealed. Marker grep zero.
