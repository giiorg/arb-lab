# Strategy #2 scoping — the §41r mandate turned into a plan (2026-08-01, session 55)

**Status: RULED — all four §42f forks were ruled by the user 2026-08-01 (same
session); the §12 edit is LANDED (milestone 2b; doc set v0.9.20). §42f below
records each ruling in place.**

Mandate: calibration design record **§41r** (v0.9.19). The user ruled core §12's
post-kill fork (c): strategy #2 — the cross-exchange spread on alt pairs and
dislocation windows (core §2.3) — arrives early. Two caveats are normative for
this scoping: **alt book depth must be MEASURED** (§41p's "the cost is fees, not
depth" is a BTC finding and does not transfer), and **§2.3's edge bar is measured
spread persistence** — strategy #2 gets its own pre-registered economic gate
under the same kill discipline that produced §41p/§41q.

This record scopes four things: the candidate universe (§42b), the capture
surface (§42c), core §12's roster re-sequencing (§42d), and the shape of
strategy #2's gate (§42e). It deliberately does **not** pin gate parameters,
screening thresholds, or quantile choices — the v1 lesson stands: procedure
parameters are chosen and recorded at the procedure's own commit (§41e's
pattern), never in the scoping record, or the scoping record becomes a place
where numbers are tuned before the procedure exists to govern them.

## §42a — the backfillability asymmetry is what orders the work

§41r calls the capture surface "time-sensitive". Measured against what each
data class can be recovered from, the pressure decomposes — and it lands on a
narrower target than "expand capture now":

| Data class | Backfillable? | From | Consequence |
|---|---|---|---|
| Settled funding history | **Yes, free** | Both venues' public REST (profiles §7: Binance `fundingRate` 1000/page ascending; Bybit `funding/history` 200/page newest-first; complete 8h BTC grid 2023-01→2026-01 verified) | Screening needs **no capture** and can run immediately |
| Live-estimate funding streams | Paid (Tardis), their format | `markPrice@1s` / `tickers` | Substitutable at cost; not our log |
| L2 books | Paid (Tardis), their format | — | **Parity is not substitutable**: §3.5 defines parity as replaying *our own* log, and §8 calls a book path never replayed from our log an unexercised pipeline |
| Our own archive | **Self-capture only** | — | The §8 day-one argument, unchanged |

So the time-sensitive object is **the shortlist's books and live-estimate
streams in our own format** — and the shortlist does not exist yet. The
screening that produces it runs on backfillable data, which means the correct
order is: screen fast (days), then expand capture for the survivors — not
"capture everything now" (disk, §42c) and not "design the gate first" (the gate
consumes months of the very capture this step starts).

One per-symbol honesty note the screening must carry: settled-funding history
begins at each symbol's **listing date**, and much of the candidate space listed
after 2023. Per-symbol history depth is recorded with the screening result;
persistence is measured over what exists, and a symbol with three months of
history supports a much weaker persistence claim than one with three years.
"Complete grid 2023→2026" is a **BTC verification** (profiles §7) — each
candidate's own history depth and grid integrity are verified during screening,
not inherited.

## §42b — the candidate universe: measured facts and the screening step

Measured this session (2026-08-01, both venues' public instrument endpoints;
the fetch is re-run at screening time — this roster moves):

- **466 symbols** are USDT linear perps in trading status on **both** Binance
  and Bybit (530 on Binance, 676 on Bybit).
- Funding grids over the intersection: **431 same-grid pairs** (332 at 4h, 97 at
  8h, 2 at 1h) and **35 mismatched-grid pairs** (31 of them Binance-4h vs
  Bybit-8h). The majority of the space settles **every 4 hours** — twice BTC's
  settlement frequency, which doubles both the revenue events and the
  per-settlement recurring drag `R` in any §41p-style decomposition.
- Binance's `fundingInfo` endpoint returned interval overrides for **all 466**
  intersection symbols — so the existing hourly REST overlay (§32h/§37) is
  **already recording venue-wide interval provenance**; no Binance-side
  expansion is needed for interval facts. Bybit's per-symbol interval arrives
  inside `tickers.<SYM>` once that stream is captured.
- **Same interval is not same phase.** Two venues on a 4h interval may still
  settle at different instants. Screening pairs settlements by
  **venue-published settlement timestamps**, never by assuming a shared grid
  origin — the recomputed-check-must-share-the-anchor lesson, applied before it
  bites rather than after.

**The screening step** (its own scratch harness + dated record, the §41p
pattern: read-only, mirrors nothing it doesn't need, its parameters pinned at
its own commit):

1. Fetch both venues' settled funding history for the intersection, full
   available depth per symbol, with a dataset ledger (ranges, row counts,
   digests) in the §41n style. Rate limits make this a background job measured
   in tens of minutes, not days (Binance: 500 req/5min).
2. Pair settlements by published instants; compute the spread series per pair.
3. Rank by **persistence-adjusted capturable spread, after two-venue costs** —
   the exact metrics are the screening commit's to pin, but the §2.3 bar means
   run-lengths and conditional persistence outrank headline spread quantiles,
   and the cost side prices four perp taker legs (§42e) so a spread that never
   clears them cannot rank.
4. Apply a tradability floor (open interest / volume on both venues) — a
   candidate we could never deploy real size on is screening noise; this floor
   is also what makes the later depth measurement meaningful.
5. Output: the shortlist for capture expansion (§42f decision 1 sets its size),
   each with history depth, grid facts, and rank evidence; plus the dislocation
   census (§42b-1).

### §42b-1 — dislocation windows are screened from the same data

§2.3 scopes strategy #2 to "alt pairs **and dislocation windows**". The
steady-state alt spread and the dislocation are different hypotheses with
different capture demands. The settled-funding history already contains every
dislocation of the past three years (a funding print at a dislocated level *is*
the settled record of one), so the screening also produces a **dislocation
census**: frequency, size, duration, and venue-asymmetry of extreme spread
events across the whole 466-symbol intersection — including symbols whose
steady-state spread ranks nowhere. What the census cannot show is
**executability** (the book during a dislocation), which is precisely the
un-backfillable half — and the honest consequence is stated now: **book
evidence for dislocations exists only for symbols whose books we are already
capturing when the dislocation happens.** Broad book capture is the only way to
catch dislocation executability broadly, and it is priced in §42c. The census
is what turns that from a guess into a priced decision (§42f decision 2).

## §42c — the capture surface expansion

**Per shortlisted symbol, four streams** — strategy #2 is perp-perp (§2.3);
there is no spot leg, so no spot books:

| Stream | BTC measured (§32m-D) | Transfer note |
|---|---|---|
| Binance `<sym>@markPrice@1s` | 11M/day | Fixed 1/s rate — transfers as-is |
| Binance `<sym>@depth20@100ms` | 441M/day | Fixed snapshot rate — transfers as-is |
| Bybit `tickers.<SYM>` | 101M/day | Push-on-change — alts likely lower |
| Bybit `orderbook.50.<SYM>` | 531M/day | Delta-driven — alts likely lower |

Per-symbol cost is therefore **bounded ~0.6–1.1G/day, derived not measured**
(the two fixed-rate Binance streams contribute ~450M/day regardless of symbol
activity; the two Bybit streams scale down with activity by an unmeasured
factor). The expansion lands with its own **§32m-D ritual: a one-day measured
figure per tier within 48h of the config landing**, and any further growth
decision waits for that number.

**Deliberate absences, each with its rejected alternative:**

- **No per-alt diff evidence stream.** BTC's `@depth@100ms` (890M/day, the
  largest single line item) exists to prove the *reconstruction pipeline*, and
  a pipeline claim does not need re-proving per symbol. Rejected alternative —
  per-alt evidence streams — fails §31e's growth-refusal roster at 890M/day
  each for a claim already held.
- **No spot books for alts.** §2.3's setup has no spot leg. If a future
  strategy revision adds one, that is a new capture decision with its own date,
  not a default. (Rejected: "capture spot books for optionality" — optionality
  at ~1G/day/symbol is not free, and the §38f clock is the scarce resource.)
- **BTC's existing eight streams run unchanged.** The v1 record is closed but
  the parity machinery, the H1 discipline, and any BTC revival a strategy-#2
  frame implies all read from it; stopping a running capture to save 3G/day
  re-opens a gap nothing can backfill — the exact §8 argument that started it.
- **No third venue.** The capture speaks `binance-path` and `bybit-v5`; the
  profiles hold verified facts for exactly these two; §42e's gate needs a
  two-venue spread, which the two-venue surface provides. A third venue is a
  new transport protocol + a new profiles verification campaign (§29's
  standard) + new fee/margin facts — real build weeks that postpone the
  time-sensitive capture for a wider universe the gate does not need. Rejected
  alternative recorded: OKX as the highest-liquidity third venue candidate —
  revisit **only if** screening shows the two-venue spread space is thin, which
  is a measured trigger, not a preference.

**The optional broad funding tier** (§42f decision 2): beyond the shortlist's
four-stream tier, a wide tier capturing **only the two funding streams** for
many more intersection symbols would cost roughly 20–110M/day per symbol
(derived; the Binance leg is fixed 11M, the Bybit leg activity-scaled). What it
buys: live-estimate dislocation coverage and drift monitoring across the space,
so future re-screening runs partly on our own archive; what it does not buy:
executability evidence (no books). Two operational facts bound its breadth: the
capture model is one source = one connection = one topic, so N broad symbols
open 2N sockets (venue per-IP connection ceilings are a profiles verification
item **before** any broad tier lands); and a multi-topic-per-connection capture
mode is a code slice with its own design record if the broad tier is ruled wide
enough to need it. Binance's all-symbol `!markPrice@arr@1s` (profiles §1) is
the rejected alternative for the Binance leg: one socket, but ~6G/day derived
for all 530 symbols' payloads — strictly worse than per-symbol streams below
~500 symbols.

**Disk and the §38f clock, stated honestly:** 967G free today; the current
capture burns ~3.6–3.8G/day healthy-derived. Shortlist tiers land at roughly
+0.6–1.1G/day per symbol. An eight-symbol shortlist roughly triples the burn
(~9–12G/day) → the disk alone carries ~3 months of everything captured so far
plus the expansion. §38f's own record already says the off-host decision
"stops being deferrable on capacity grounds alone" in weeks-to-months at the
*old* rate; the expansion **compresses that clock by ~3×**, and §42f puts the
destination question back to the user with the compressed number rather than
letting the deferral silently inherit a stale rate.

**Mechanics of landing it:** the expansion is a `configs/capture.json` edit
plus a capture restart. The restart writes an honest `process_restart` gap
record (§32's design working as built); the alternative — a second capture
process on the same archive root — is rejected: the writer's single-owner
model and the manifest's close-time truth were designed for one process, and
two writers is §5.1's forbidden shape transplanted to the archive.

## §42d — core §12's roster re-sequencing (spec edit proposal)

§41r's consequence: "what 'the same foundation' means when the OMS does not
exist yet is exactly the scoping question." Answered by the roster's own
organizing principle — **edge proven before the heavy build** — which survived
v1 by firing: milestone 2 killed the flagship's economics before the OMS was
built to trade them. Strategy #2 gets the same treatment.

**Proposed edit** (lands in core §12 as a dated edit with the ruling, when
§42f decision 5 is taken):

- **New milestone 2b — strategy-#2 edge proof, before the OMS:** the §42b
  screening (dated record + dataset ledger), the §42c capture expansion, the
  `strategy/crossvenue` **read-path** on the existing foundation (marketdata +
  strategy contexts, the §3.3 seam, the capture archive), a research simulator
  over captured + backfilled data, and **strategy #2's own pre-registered
  economic gate** (§42e) with kill authority. Deliverable shape mirrors
  milestone 2: an explicit pass/fail, cheap to kill.
- **Milestones 3–6 (OMS, risk/control, backtester, go-live) are unchanged in
  content and order, and are now hard-gated on milestone 2b's gate** — the
  heavy build starts only for an edge that measured alive. Their internal
  scope gains the two-venue notes strategy #2 implies (the §6 machinery was
  always two-venue; that is why §2.3 calls the foundation valuable), but that
  is their existing text already.
- **Roster item 7 narrows to strategy #2's execution half** (two-venue OMS
  wiring, live deployment) — its research half moved into 2b.

**Rejected alternatives:** (i) proceed to milestone 3 now and build the OMS
against a killed v1 edge — rejected by the roster's own preamble; the OMS
would be built "on spec" for a strategy whose economics are unmeasured. (ii)
Rewrite the roster wholesale around strategy #2 — rejected: the roster's
milestones 3–6 are strategy-agnostic controls and survive unchanged; a minimal
dated edit is auditable, a rewrite is drift wearing a redesign's clothes.
(iii) Run strategy #2 informally without a milestone row — rejected: the gate
needs a pre-registered home in the normative core, or its kill authority is a
promise in a side record.

## §42e — strategy #2's economic gate: shape now, parameters at its own commit

The §41p/§41q discipline transfers whole: **pre-registered procedure at a
named commit; dataset ledger with pinned digests; holdout declared before any
scoring; a governed derivation from a clean checkout that can refuse; refusals
are governance questions, never numbers to invent; kill authority is real.**
What this record pins is only what makes strategy #2's gate *different* from
v1's, so the eventual procedure commit inherits scope rather than re-deriving
it:

- **The edge bar is measured spread persistence** (§2.3, normative). The
  headline spread is not the edge; the procedure's core model is conditional
  persistence of the cross-venue spread, and the entry test prices the spread
  *given* it must persist across the planned hold's settlements.
- **The cost side must measure alt depth** (§41r caveat 1). The §8
  conservative-bound *structure* (worse of depth-walked-fill-after-latency
  vs. p90-conditional-on-size) transfers; every number in it is re-measured
  per symbol from the §42c books. No BTC cost figure crosses over.
- **The frame is two-venue perp-perp:** four perp taker legs round trip
  (maker-entry priced as the standing tactic §41r retained, never assumed),
  margin posted on both venues (both sides' `C_allocated` decomposition),
  inter-venue basis risk (the §2.2 "no second counterparty" advantage is
  surrendered — the gate prices what replaces it), and funding-grid phase
  (mismatched instants create windows where one leg has settled and the other
  has not — the 4h-majority grid makes this a first-class term, not a
  footnote).
- **Holdout:** H1 (post-2026-02-01 BTC) stays untouched, permanently. Strategy
  #2's own holdout is declared at its procedure commit over eras that
  actually exist per symbol (§42a's listing-date honesty); the standing rule
  inherited now is only that **the holdout is declared before any scoring
  touches it** — the seal ritual, not its dates.
- **Utilization honesty transfers:** returns quoted at expected
  time-in-position on full `C_allocated`, never in-position rates — the exact
  trap fork (d) died in.

## §42f — the forks this record surfaced, and the rulings (user, 2026-08-01)

1. **Shortlist breadth for the four-stream tier** — the cost knob. Screening
   ranks; this decision sets how many ranked survivors get books. Disk prices
   (derived): ~4 symbols ≈ +2.5–4.5G/day; ~8 ≈ +5–9G/day; ~12 ≈ +7–13G/day.
   **RULED: ~8 symbols** — wide enough that the gate isn't hostage to one or
   two candidates going quiet; the alternatives' prices stand recorded above.
2. **The broad funding-only tier** — none / ~30 symbols / wider. Buys the
   dislocation census a live-estimate continuation and future re-screening
   from our own archive; costs sockets and ~20–110M/day per symbol, and a wide
   setting motivates a multi-topic capture slice first.
   **RULED: none for now** — the census (backfillable, free) prices what
   breadth earns its disk before any socket opens; the tier is a future
   decision with a measured input, not a standing default.
3. **§38f destination** — the expansion compresses the off-host clock ~3×.
   Rule the destination now, or re-defer with the compressed number recorded.
   **RULED: re-defer, dated** — the deferral now carries the compressed
   number (~3 months of local disk at the expanded rate) and a named revisit
   point: **the first post-expansion §32m-D measurement**, where the derived
   per-symbol figure becomes a measured one. The risk it re-accepts is
   calendar-denominated (§38f's own words): a disk loss restarts the
   un-backfillable book clock.
4. **Mismatched-grid pairs (35)** — in or out of v1 screening scope.
   **RULED (by standing recommendation, unobjected): out** — the
   asymmetric-accrual ledger is real modeling work, and 431 same-grid pairs is
   not a thin space; the census still covers them read-only.
5. **The §42d roster edit** — sign-off to land it in core §12 (doc-set bump).
   **RULED: signed off as proposed** — landed same session as core §12
   milestone 2b, item 7 narrowed, doc set v0.9.20.

## RESUME HERE

**RULED AND LANDED (2026-08-01, session 55, v0.9.20).** All four §42f forks are
ruled; core §12 carries milestone 2b; the ADR block is dated. **Next, in
dependency order (milestone 2b's own sequence):**

1. **The screening step** (§42b) — buildable now: the scratch harness over both
   venues' settled-funding REST history for the 431 same-grid pairs, its
   parameters pinned at its own commit, output = the dated screening record +
   dataset ledger + the ranked shortlist + the dislocation census (§42b-1).
2. **The capture expansion** (§42c) — follows the shortlist within days: ~8
   symbols × four streams into `configs/capture.json`, restart with its honest
   `process_restart` gap record, then the **§32m-D one-day measurement within
   48h** — which is also §38f's named revisit point (ruling 3).
3. **The `strategy/crossvenue` read-path, the research simulator, and the
   gate's procedure commit** (§42e) — after data flows; the gate's parameters
   are pinned there, never here.

H1 stays sealed. BTC's eight streams run unchanged. The standing session
rituals continue (capture alive, verify COVERAGE/0-integrity, marker grep).
