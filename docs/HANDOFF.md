# HANDOFF — Funding-Rate Arbitrage Platform

## Status (2026-08-01, session 66): **FORMAL MODEL (a) SLICE B IS BUILT AND VERIFIED — SM §2 IS NOW COMPOSED WITH SM §7 ON THE ONE SHARED RESERVATION LEDGER, AND IT FOUND A SPEC DEFECT: `FM-5`.** Docs stay at v0.9.23 (marker grep **ZERO**), **but a normative edit is now OWED** — see below. The build record is **§46 §8** in `docs/superpowers/specs/2026-08-01-formal-model-a-ambiguous-order-design.md` — read it, then its RESUME HERE; **the next act is slice C** (liveness `P1`–`P4`, each with its negative control in the same slice). **35 model-(a) runs, all matching predictions**: the composed nominal green (6,729,250 generated / **832,179 distinct**, depth 30, no deadlock, ~8 s), a second `_SM2` nominal (`MAX_GEN = 0`) that **reproduces slice A's state graph to the digit** (25,514 / **5,169**, depth 15), **twelve** negative controls each violating their predicted invariant, and **twenty-one** positive controls each confirming their state is reached. **`FM-5` — SM §7's FLAG-3 ("the reservation stays generation-scoped FROM CREATION … the §2 classifier subtracts it THROUGHOUT") composed with §2's unconditional reservation-aware sizing lets a generation book its TARGET into the ledger at `Planned`, before the perp leg has bought anything.** On a package that is neutral, `realized − reserved` then reports a phantom SHORT, §2's determined branch buys — and buys back exactly the exposure the generation's own perp leg is about to buy. **TLC exhibits the unbacked ledger at two steps and the resulting DISPATCH at three; the end position (net long by the target) follows arithmetically from the model's own transitions and is labelled a derivation in §46 §8d, not a checked property** — pinning "the package was neutral at the start" would need history the model does not carry. **Four rounds of review and slice A all missed it for one structural reason: `INV1` is phrased over ledger-exact quantities, which are COMPUTED FROM the ledger, so a corrupt ledger corrupts them too and the phantom buy computes as non-overshooting — `overshoot` is FALSE in the violating state. An invariant built on the ledger cannot audit the ledger.** The fix is narrow and FLAG-3 keeps its words: what was underdetermined is the reservation's **amount**, not its existence — the entry is created at `Planned` carrying the generation's **outstanding net intent**, which is ZERO until the perp leg confirms and is amended as it fills (§2's own "coalesce / amend / cancel-replace, never a fresh command per event", applied to the reservation). That reading is nominal; the literal one is control NC7. **THE NORMATIVE EDIT IS OWED AND SCHEDULED FOR SLICE E** by §46 §5's roster ("findings triaged; spec defects tagged FM-5…"); until it lands, `docs/formal/` and `docs/state-machines.md` disagree about §7's FLAG-3 and CLAUDE.md's standing rule resolves it ("treat the formal model as authoritative about the safety kernel's behaviour") — the exact wording to land is in §46 §8d. **The model also found TWO defects in its OWN slice-A reasoning** (§46 §8e): `INV1`'s persistent conjunct rested on slice A's stated bound "no new counter-leg order appears mid-episode", which §7's perp reduce legitimately breaks — it was NOT weakened but MOVED to `INV1b` in the `_SM2` config where its premise holds (and the at-dispatch conjunct alone was verified to still catch NC1/NC6 composed); and **two formulations of `INV4` were tried and both are wrong**, one false of correct composed behaviour and one blind to `FM-5`, which is itself the finding — there is no harm-level phrasing that is both sound and complete, because every "what will the position become" quantity is computed from the ledger. **The slice-B fixture collapse, named by the checklist BEFORE the runs and probed (`PC18`): if the perp reduction always confirmed in FULL, "sells no more spot than the CONFIRMED reduction" and "…than the TARGET" are the same expression and NC8 is undetectable.** `NPEND` went 2 → 3 so the one subtraction can have three nonzero terms at once (`PC12`), and `PC9`/`PC10`/`PC11` are slice B's done-when: each of the three producers separately shown to contribute a nonzero term. **`TypeOK` earned its keep** — it failed at depth 22 on `q = 9` against slice A's ceiling of 8, because §7's generations move delta without spending the dispatch budget; domains widened deliberately, not silently. **NC12's own witness was DEGENERATE** (a zero-amount orphan, which leaks a slot but subtracts nothing) — the harmful nonzero orphan was confirmed reachable separately, and NC12 also DEADLOCKS, a second independent signature of the same defect. **NEXT (in order): (1) §46 slice C** (task #154); **(2) the §32m-D per-tier figure, DUE BY 2026-08-03T11:13Z**, measurable from 2026-08-02T11:13Z (task #150); **(3) off-host archive sync** — no target yet, recommendation a ~$60 external USB drive; **(4) the gate itself** (task #152, BCHUSDT binds ~2026-09-30). Task #153 (the hourly `restart_boundary`) awaits a user ruling and blocks nothing. H1 unpolluted; capture runs on (**PID 8495**). **Start at `## START HERE` below.**

## Status (2026-08-01, session 65 — HISTORICAL): **FORMAL MODEL (a) SLICE A IS BUILT AND VERIFIED — `docs/formal/AmbiguousOrder.tla` model-checks SM §2's classifier, finality contract and interval-floor generation; nominal green, SIX negative controls each violate their predicted invariant, EIGHT positive controls each confirm the interesting state is reached.** Docs stay at v0.9.23 (no normative change; no spec edit taken, marker grep **ZERO**). The build record is **§46 §7** in `docs/superpowers/specs/2026-08-01-formal-model-a-ambiguous-order-design.md` — read it, then its RESUME HERE; **the next act is slice B**, the reservation ledger's other two producers + `PackageDelever`'s phases + `INV4`–`INV6`. Landed: `AmbiguousOrder.tla` (+ nominal cfg + `NC1`–`NC6`), `AmbiguousOrderCoverage.tla` (+ 8 probes, cfgs generated by `run.sh`), and `run.sh` extended to check **both** models — it now takes a **`TLC_WORKERS`** cap (used at 4 this session so a 30-minute all-core run could not starve the capture) and a config-tag filter whose summary prints `FILTERED … NOT the full suite`, so a partial green can never be quoted as the whole one. **All 23 runs across both models were re-verified through it**, and model (b) — untouched this session — **reproduces its recorded v0.9.4 state graphs to the digit** (`_Live` 3,945,352/491,160, `_Live2` 1,705,418/249,936), which says the pinned toolchain and configs still produce the same object a different session recorded. The whole model-(a) block runs in **under a second** (nominal 25,514 generated / **5,169 distinct**, depth 15, no deadlock — `Settled` is an explicit self-loop at the one predicted terminal, so any *other* dead end reports as a deadlock and is a finding). **Three build decisions worth not reopening (§46 §7a): (1) an order is `(dir, q)`, not `(lo, hi)`** — a single order executes in ONE direction so its interval always has zero as an endpoint and straddling is a property of the SUM, which is where §2 forbids compensation; the `(lo,hi)` encoding was tried first and is unfaithful *and* unbounded (endpoints drift without limit under repeated fills); **(2) `Impl*` vs `True*`** — every computed quantity exists twice, guards read only `Impl*`, invariants read only `True*`, so §46's "the property must not read the mechanism" stops being a rule to remember and becomes a property of the file's layout; **(3) `INV1` is checked at dispatch AND persistently**, the second conjunct existing to make TLC *check* the monotonicity argument that justifies the first rather than trusting a comment. **The model-altitude fixture collapse it caught: `NPEND = 2` is load-bearing** — with one generation slot the `realized − reserved` subtrahend is pinned to **zero in every reachable nominal state**, so "sized from the residual" and "sized net of reservations" would have been the same expression and NC6 would have been undetectable; probe `PC3` is what proves the nonzero case is reached. **TLC found a real defect on the first run:** in TLA+ `=` binds tighter than `\/`, so `flag' = flag \/ cond` parses as `(flag' = flag) \/ cond` and leaves the variable **unassigned** — all six history flags had it. **One question carried to slice E, deliberately not resolved now (§46 §7d): "never issued twice" may carry no safety weight independent of the reservation** — NC2 issues a second floor with `overshoot` FALSE throughout, and the spec's own counterexample for the rule assumes the reservation is *not* counted, which the adjacent sentence requires it to be. **NEXT (in order): (1) §46 slice B** (task #154); **(2) the §32m-D per-tier figure, DUE BY 2026-08-03T11:13Z**, measurable from 2026-08-02T11:13Z (task #150); **(3) off-host archive sync** — no target yet, recommendation a ~$60 external USB drive; **(4) the gate itself** (task #152, BCHUSDT binds ~2026-09-30). Task #153 (the hourly `restart_boundary`) awaits a user ruling and blocks nothing. H1 unpolluted; capture runs on (**PID 8495**). **Start at `## START HERE` below.**

## Status (2026-08-01, session 64 — HISTORICAL): **FORMAL MODEL (a) IS COMMISSIONED AND DESIGNED — §12 hard-gates milestones 3–7 on strategy #2's gate, so this is the substantial engineering that gate does NOT forbid.** Docs stay at v0.9.23 (no normative change this session; `docs/formal/` still at Cx8). The design record is **§46** in `docs/superpowers/specs/2026-08-01-formal-model-a-ambiguous-order-design.md` — read it, then its RESUME HERE; **the next act is slice A, `docs/formal/AmbiguousOrder.tla`.** Model (a) is `state-machines.md` **§2** (ambiguous-order classifier + finality contract) composed with **§7** (`PackageDelever` generations + reserved reductions); next finding tag is **FM-5**. **It is not a bet on the gate passing** — core §6 is titled "Trade lifecycle (foundation, STRATEGY-AGNOSTIC)", and the two-leg partial-failure problem is identical for cash-and-carry (spot+perp) and strategy #2 (perp+perp), so the work is wasted only if the PROJECT ends, not if strategy #2 does; milestone 3 independently requires it ("model-test it here, before any live capital"). **Two §46 decisions taken and argued, do not silently reopen: (1) §2 is modeled WITH §7** — the tempting smaller scope is rejected on model (b)'s own headline result, because FM-1 was a COMPOSITION defect and here the composition is three-way (`outstanding_reserved_reductions` has three producers: the determined branch's fenced generation, the `IntervalFloorReduction`, and `PackageDelever`'s reserved spot leg, all resting on one subtraction being exactly right); **(2) every liveness property ships with its negative control in the same slice**, because Cx8 proved a passing liveness property and a vacuous one look identical (NC5 arrived late; model (b)'s P2/P3/Branch1/Branch2 still have none). **Two traps named BEFORE any `.tla` was written, both the fixture-collapse checklist at formal-methods altitude:** `INV2` ("floor never issued twice") must count dispatched reductions, NOT read the `floorIssued` guard, else it proves the flag consistent with itself; and `INV1` must be phrased over RESOLUTIONS ("no resolution produces an opposite sign"), never over the endpoint formula — the spec already carried a bug there (v0.7.1's `|b|` was right only for the net-long case, corrected to `max(|a|,|b|)` by R1d-2), so a model re-implementing the corrected formula and checking itself against it proves nothing. Toolchain verified runnable: Java 25, `tla2tools.jar` pinned by SHA-256, `docs/formal/run.sh` fetches it. **Also this session:** HANDOFF's entry point had DRIFTED (sessions 62 and 63 updated the Status line but stacked no START HERE, so the file CLAUDE.md designates as "start here" routed readers to session 61's block, whose next item was §45 slice D — already landed); fixed, and **the convention is: a session that updates Status MUST also stack a START HERE and mark the prior one HISTORICAL.** A new capture observation is recorded and awaits a ruling (**task #153**): `capture verify` reports `restart_boundary` on binance `fundingInfo` EVERY HOUR because a REST-overlay source writes `data 0`, so the writer's `lastSeq` never leaves zero — not a correctness bug (the archive is honest), a SIGNAL-QUALITY question, since that marker also means "a restart happened". **NEXT (in order): (1) §46 slice A** (task #154); **(2) the §32m-D per-tier figure, DUE BY 2026-08-03T11:13Z**, measurable from 2026-08-02T11:13Z (task #150); **(3) off-host archive sync — the user has no target yet; recommendation is a ~$60 external USB drive** (slice F's machinery is built and proven against its second-root fixture; §38g excluded transport+credentials, so a cloud bucket is the larger change), because the gate's ~2-month evidence wait now commits ~180–200G of un-backfillable archive to a single laptop disk and §38f's own words are "the money loss is bounded, the schedule loss is not"; **(4) the gate itself** (task #152, BCHUSDT binds ~2026-09-30). H1 unpolluted; capture runs on (**PID 8495**); marker grep ZERO; `make ci` green. **Start at `## START HERE` below.**

## Status (2026-08-01, session 63 — HISTORICAL): **v0.9.23 — §45 IS CODE-COMPLETE: strategy #2's cross-venue gate PROCEDURE COMMIT landed (`7ccadec`), which lifts the §45f-2 post-cutoff seal. The gate now waits on EVIDENCE, not on code.** The build record is **§45p** in `docs/superpowers/specs/2026-08-01-strategy2-crossvenue-gate.md` — read it, then its RESUME HERE. Landed: the `Proc*` roster (`procedure_crossvenue.go`) with rationale beside every constant — per-symbol entry levels CHECKED on every run against the committed `shortlist.json` `best_entry` (and a three-distinct-levels assertion, so a collapse to one modal level cannot pass); per-symbol sizes by one stated rule (**250 USDT notional per leg, rounded UP, lot-aligned on BOTH venues** — steps differ on KAITO 0.1/1 and BCH 0.001/0.01, Binance's BCHUSDT wants 20 USDT notional against every other row's 5; eight symbols × two legs = 4,006 gross = **1,001 USDT of margin at 4×**, §8's $1K tier, so the size is the tier divided by the roster); equal weights (§45n-2), the 180-matched end rule (§45f-1), a **0.25 refusal ceiling** (§45n-3 — refusals cluster on stalls, which are the expensive instants, so the survivors are biased CALM), 4h periods, 6-period mean block, 2000 resamples, 0.95 confidence, date-derived seed. **Fees and the hurdle are READ, not pinned** — §2.4 forbids hardcoded fee constants and §2.1 a hardcoded hurdle, and the committed reference already carries both perp taker cells (venue+product keyed, no symbol column) plus the dated treasury.gov hurdle; `ProcCrossVenueSimParams` takes the fees as ARGUMENTS so the assembly cannot complete without the read (a zero taker fee is a free round trip). Two pure judgements in `domain/crossvenue/gate.go` (the ceiling cross-multiplies at the mantissa scale — dividing first would force a rounding call exactly at the boundary the rule is about). The runner **refuses the RUN** when any symbol is unfit rather than dropping it, because §45f-3's pass shape is a PRE-DECLARED portfolio — that interpretation is recorded in §45p so it is not silently reopened. **One spec deviation TAKEN and its clause landed the same commit:** the gate reports on the per-leg NOTIONAL, not `C_allocated`; §2.1's prohibition names only SMALLER denominators because the risk is a flattered return, and the notional is larger here, so the figure understates — §2.1 gained the declared-and-demonstrably-larger clause. Marker grep **ZERO**. Battery: the checklist BEFORE the battery caught a **ceiling of exactly 1.0 that could never fire**; **22 aimed mutations — 20 caught, two NOBUILD re-aims, one SURVIVED** diagnosed as a wrong mutant AND a real finding (an `episodes == 0` branch that could not fail, removed). **THE SMOKE REFUSED THREE TIMES, all three the machinery working:** dirty tree (§41e), then the unvouched OPEN hour (H2 — so a gate window can only include COMPLETE days), then, over a hardlinked scratch archive with the open hour excluded (live archive untouched, verified after), **the end rule by name, exit 2, no artifact written**. That run measured three things: matched counts **4 (4h grid) and 2 (BCH 8h)** exactly as the seal's window arithmetic predicts; **286 joined decision points against 287 available minutes (~99.7%)**, which is §45o(a)'s continuity carry on live data against the ~9% ruling §45n-1 rejected; and **MMTUSDT scoring 4 episodes end to end** — episode machine, both book walks, cost composition and phase ledger, on real post-cutoff bytes. **NEXT (in order): (1) the §32m-D per-tier figure, DUE BY 2026-08-03T11:13Z** (§44e + §38f's revisit; archive stood at 9.4G on 08-01); **(2) accumulate evidence and re-run the gate — `arblab crossvenue fetch` periodically (first fetch 2026-08-01T16:00Z, 16/16 ok), then `arblab crossvenue gate`. BCHUSDT is the BINDING symbol** (8h grid = half the others' rate), so the roster qualifies around **2026-09-30**, not the majority's ~08-31 — a forecast, not a schedule; the `matched` column decides. **(3) when it scores**, the write-once artifact authorizes nothing until a detached attestation binds it (24h cooling, sign-off, out-of-band) — a USER decision under milestone 2's kill discipline. Doc set v0.9.23. H1 unpolluted; capture runs on (**PID 8495**). **Start at `## START HERE` below.**

## Status (2026-08-01, session 57 — HISTORICAL): **v0.9.22 — MILESTONE 2b STEP 2 IS LANDED: the capture runs the shortlist — 40 sources (PID 8495), 32 of them new, restarted 11:13Z with the honest `process_restart` gap on every pre-existing source.** The capture-expansion design record is **`docs/superpowers/specs/2026-08-01-strategy2-capture-expansion.md` (§44)** — read it, then its RESUME HERE. The order §42c demanded was kept: **the per-IP WebSocket ceiling verification landed FIRST** (profiles' dated block, discharging the v0.9.20 scope item — Bybit ≤500 new connections/5min per domain, restart burst 19; Binance spot 300/5min, we open 1; **Binance futures states NO numeric per-IP ceiling** — behavioral bound, the per-source 1s→60s backoff ladder is the compliance), then the config (§44b: 8 → 40 sources, **nothing per-symbol chosen** — every new source is its BTC counterpart's shape with the symbol substituted, scripted with a uniqueness assertion; BTC's eight byte-identical; §42c's absences re-affirmed), then the restart (§44c: ritual verify FIRST — **COVERAGE, 0 integrity, 639 coverage**, the known thin windows — SIGTERM 11:13:02Z, clean stop, start 11:13:10Z, **~8s recorded gap**, all 40 sources non-thin within two minutes, all 8 pre-existing BTCUSDT sources minted `process_restart` gap records; config hash `1c362feffe4a`). **NEXT (in order): (1) the §32m-D per-tier one-day figure, DUE BY 2026-08-03T11:13Z** — lands as §44e in the §44 record; `du` split BTC-tier vs alt-tier against §42c's derived ~0.6–1.1G/day/symbol bound, full-day verify over the measured day (must still be 0 integrity; it will honestly show this restart's gaps) — **this is also §38f's named revisit point** (ruling §42f-3: the off-host decision gets re-argued on the measured disk clock); **(2) the `strategy/crossvenue` read-path, the research sim, and the gate's procedure commit** (§42e — gate parameters pinned THERE; §43b's selection-contamination statement is a named holdout input). Doc set v0.9.22. H1 unpolluted; capture runs on (**PID 8495** — the old 74192 is gone, stop pgrep-ing for it). **Start at `## START HERE` below.**

## Status (2026-08-01, session 56 — HISTORICAL): **v0.9.21 — MILESTONE 2b STEP 1 IS EXECUTED: the screening ran, and the shortlist is LABUSDT, BEATUSDT, KAITOUSDT, MMTUSDT, MUSDT, SOONUSDT, ESPORTSUSDT, BCHUSDT.** The screening design record is **`docs/superpowers/specs/2026-08-01-strategy2-screening.md` (§43)** — read it, then its RESUME HERE. The harness is **committed code** (`internal/strategy/infrastructure/screening` + `arblab screen run`; parameters pinned in `params.go`, §43b: cutoff 2026-08-01T00Z strictly before, BTCUSDT permanently sealed out — tested as "no BTC funding URL is ever requested" — 21bp two-venue round trip, entry grid {2,5,10,20}bp with exit E/2 hysteresis, ≥180-matched floor, 5M turnover floor on both venues, census 25bp, shortlist 8). The run (§43c): **465/465 symbols, zero failures**, 936 raw files (git-ignored, every SHA-256 pinned in the committed ledger), **1,572,157 matched instants, zero pairing collisions**; eligibility partitions the 466 exactly — 61 ranked / 367 below floor / 35 mismatched-grid / 2 thin / 1 sealed. The ranking (§43d): best-entry annualized net over EPISODES after costs — **71.6%/yr (LABUSDT) down to 1.5%/yr (BCHUSDT)**, cut at #9 LTCUSDT 0.93%; seven of eight are 2025 listings on the 4h grid, every win rate below half — persistence is what pays. The census (§43e): **2,972 events ≥ 25bp across 365 symbols, 2.4k in 2025–26** — current, not a relic. Selection contamination stated once (§43b); **nothing in step 1 is gate evidence** (§43g). **NEXT (in order): (1) the §42c capture expansion** — the shortlist × four streams into `configs/capture.json`, restart with the honest `process_restart` gap record, the **§32m-D one-day figure within 48h** (also §38f's named revisit point); the per-IP WebSocket ceiling verification is due BEFORE the config lands; **(2) the `strategy/crossvenue` read-path, the research sim, and the gate's procedure commit** (§42e — gate parameters pinned THERE). Doc set v0.9.21. H1 unpolluted; capture runs on (PID 74192). **Start at `## START HERE` below.**

## Status (2026-08-01, session 55 — HISTORICAL): **v0.9.20 — STRATEGY #2 IS SCOPED AND §12 IS RE-SEQUENCED: milestone 2b (strategy-#2 edge proof) precedes the OMS, milestones 3–6 are hard-gated on its pre-registered gate, and all four scoping forks are RULED.** The scoping design record is **`docs/superpowers/specs/2026-08-01-strategy-2-scoping.md` (§42)** — read it, then its RESUME HERE. The organizing fact (§42a): **backfillability asymmetry** — settled funding is free REST on both venues (screening needs no capture), while books + our own log are un-backfillable (the time-sensitive object is the *shortlist's* streams, and the shortlist comes from screening measured in days, not from capturing broadly). Measured universe (2026-08-01): **466 both-venue USDT perps, 431 same-grid (332 at 4h — twice BTC's settlement frequency), 35 mismatched (out of v1 scope by ruling 4)**; Binance `fundingInfo` covers all 466, so interval provenance already flows; same interval ≠ same phase — screening pairs settlements by venue-published instants. **The four §42f rulings (user, same session): ~8-symbol shortlist** for the four-stream tier (both venues' perp funding + books; no spot legs — §2.3 is perp-perp; no per-alt diff evidence; BTC's eight streams unchanged); **no broad funding tier** until the dislocation census (§42b-1, backfillable) prices it; **§38f re-deferred dated** — the expansion compresses the off-host clock ~3× (~3 months of local disk), revisit at the first post-expansion §32m-D measurement; **roster edit signed off** — core §12 gains milestone 2b, item 7 narrows to the execution half, ADR §3 gains the dated scoping block (tagless total stays fifty-four), doc set v0.9.20, tutorial.ka header synced FIRST then re-assembled (102 doc tests pass). **NEXT (§42's RESUME HERE, in order): (1) the screening step** — scratch harness over settled-funding REST history for the 431 pairs, parameters pinned at its own commit, output = dated screening record + dataset ledger + ranked shortlist + dislocation census; **(2) the capture expansion** for the shortlist + the §32m-D ritual within 48h; **(3) the crossvenue read-path, research sim, and the gate's procedure commit** (parameters pinned THERE). Strategy #2's gate inherits the §41 discipline whole: measured alt depth (no BTC cost figure transfers), §2.3's persistence-measured edge bar, holdout declared before scoring, kill authority real. H1 holdout unpolluted; capture runs on (PID 74192). **Start at `## START HERE` below.**

## Status (2026-08-01, session 54 — HISTORICAL): **v0.9.19 — THE §12 FORK IS RULED: (c), THE ALT/DISLOCATION NICHE — strategy #2 arrives early, and the other three forks are adjudicated on the measured record.** The user's dated ruling (calibration design record **§41r**) answered the question the kill left open, and §41p's decomposition (`clearing(h) ≈ A/h + R`: A ≈ 39bp round trip, R ≈ 3.2bp/settlement recurring drag) is what made three of the four forks decidable on paper: **(a) maker-entry** — numerically refuted standalone (a 40% cut on A leaves the short-hold corner at ≈10.8bp clearing vs the era's 8.8bp all-time maximum; long-hold effect ≈0.2bp), **retained as a standing tactic**; **(b) regime-selective** — declined: the conservative estimator IS regime-selectivity priced, and the deployment sweep already bounded the concession (11.5% utilization, 4 entries/3yr); **(d) tuition** — declined as mispriced (assumed 60% utilization, measured 11.5% — the sub-hurdle spread buys standby management, not live education; the platform value lands via (c)'s build path); **(c)** — the one fork whose hypothesis the era has NOT refuted: it changes the funding distribution itself instead of shaving costs around BTC's (median 0.74bp, max 8.8bp). **Two caveats normative for the scoping (§41r):** alt book depth must be MEASURED (§41p's "fees, not depth" is a BTC finding, does not transfer), and §2.3's persistence-measured edge bar stands — strategy #2 gets its own pre-registered economic gate under the same kill discipline. Landings: §41r + new RESUME HERE (the calibration record is COMPLETE), §12's dated ruling sentence, the ADR ruling block, all doc stamps at v0.9.19, tutorial.ka re-assembled. **NEXT: a NEW design record scoping strategy #2** — the §12 roster re-sequencing, the alt/cross-venue capture surface (time-sensitive: Tardis cannot backfill our own gap; the BTC-only capture universe is the live gap now), candidate-universe selection. H1 holdout unpolluted; capture runs on. **Start at `## START HERE` below.**

## Status (2026-08-01, session 53 — HISTORICAL): **v0.9.18 — MILESTONE 2 IS CLOSED: THE ECONOMIC GATE FIRED, by dated governance ruling, on the machinery's own evidence.** The governed derivation ran from a clean checkout (VCS-stamped `95c0819`, two invocations) and **REFUSED on the real era** — `ErrNoExcursions`, byte-identical twice: at the pre-registered v1 procedure the entry clearing level (0.0016/settlement) sits at **1.8× the 2023–2026 era's maximum funding print** (3,381 settlements: median 0.74bp, p99 4.7bp, max 8.8bp vs a 16bp clearing), so the deployed `Enter` never fires and the exit band has nothing to measure. The user ruled DIAGNOSE FIRST; the read-only sensitivity sweep (**§41p** in the calibration design record, scratch-clone harness mirroring `Run`'s own pipeline) found the clearing level **capital-scale-invariant to the ninth digit** (×1/×5/×25 tiers identical — every frame cost is proportional to notional; measured calm-stratum p90 slippage is ZERO at 0.02–0.5 BTC), decomposing as ≈39bp round-trip amortization + ≈3.2bp/settlement recurring drag; the deployment sweep (§41p addendum) then bounded the space — the best admissible corner (98% deployment, 30-day min hold) reaches **11.5% utilization, 4 entries in three years**, which §8's own LCB scoring could only measure as a kill. **The user's second ruling (§41q): KILL NOW, BY RULING** — no artifact, no attestation, step 5 unbuilt; the ceremony would have measured the same verdict. Landings with the close: §8's pre-registered latency clause (the slice's one marker → `[SPEC EDIT LANDED — v0.9.18]`, marker grep **ZERO**), §12's dated outcome sentence, the ADR verdict block, doc set v0.9.18, tutorial.ka re-assembled (102 doc tests pass). **THE OPEN QUESTION — the first thing to put to the user — is core §12's pre-registered fork choice: (a) maker-entry tactic, (b) regime-selective deployment, (c) alt/dislocation niche, (d) tuition. §41q maps each fork onto the measured decomposition; start there.** The H1 holdout stays unpolluted; capture runs on. **Start at `## START HERE` below.**

## Status (2026-07-31, session 48 — HISTORICAL): **v0.9.17 — STEP 3 IS CODE-COMPLETE: the cashcarry core is BUILT, the write-ahead `Decide` door is REAL, and §2.4's kill-math runs in Go.** Slice C (§40j, build record **§40n** — read it, then the record's `RESUME HERE`) lands in four commits: `strategy/domain` gains `DecisionTerms.EntryMargin` (§2.2's C3 "with margin" finally consumed — mirrored on the exit `Band`, entry-required/exit-refused, one wire member) plus two exit coherence sentinels (`ErrTerminalMoved`, `ErrCandidateNotHeld`); **`strategy/domain/cashcarry`** holds the pure core — `Enter` (§2.4's inequality, every term `Money` over the `HoldPlan`, hurdle on `C_allocated` never `C_legs`, quantile under the joint rule, the §40g quantization ladder with named refusals, the capital-funding gate), `Continue` (sunk costs excluded by construction, Δ terms only where branches differ, band+dwell on REALIZED settlements — at most one advance per evaluation that observes an uncounted settlement, reset on recovery, untouched by calls and regeneration — plan-complete closes), `ProposeHoldPlan` (the MINIMUM policy-admissible hold; longer holds emerge from repeated Continues), `RegeneratePlan` (§40f: retained ≤ at, future regenerated to the UNCHANGED terminal, flip-vs-shift classified from the schedules); the application `Decide(held, Continuation)` retires `ErrCoreUnbuilt` — assemble from the seven ports, judge §3.4 freshness/skew at the read (both skew directions), sink write-ahead (refusals included), derive Intents only downstream of the journaled Decision, `Deps` gains the injected `ids.Generator`, `Settings` refuses each zero by name. **The tutorial §3.9 pinned-ledger test passes — three independent implementations of the kill-math agree to the digit** ($90/$38/8.44d/5.68%/3.41% vs 4.6%). **The §40d marker is LANDED: §2.4 gains the two fee-revision clauses (as-of composition with §3.5; the modeled-tier milestone-2 authorization); marker grep ZERO.** Checklist BEFORE battery found **eight** collapses (pinned-equal candidates, the 8h-interval constant, no at-instant fixture, terminal==last-settlement, equal exec legs, one-sided skew, one-sided budgets, exit expected==adverse); **52/52 aimed mutations caught** (one NO-OP re-aim, one NOBUILD re-aim, one SURVIVED diagnosed into the second-rounding-absorbs-the-first fixture gap — its fix the lone killer). `make ci` green cold, all six gates clean, docs 102 pass, tutorial.ka re-assembled. **Start at `## START HERE` below.**

## Status (2026-07-31, session 47 — HISTORICAL): **v0.9.16 — STEP 3 SLICE B IS BUILT (the models), and SLICE D's DISK FIGURE IS TAKEN.** Slice B (§40j, build record **§40m** — read it, then the record's `RESUME HERE`) lands the model layer in four commits: `fixedpoint.Rate.Div` (Money.Div's mechanics, full battery + the unset-map mirror), `strategy/domain`'s models — `Dispersion` (class-2 quantile points, selection rounding **outward from the median** with out-of-span refusal), the **time-decay EWMA** (weight `2^(-Δt/halfLife)` in pure fixedpoint: scale-9 constant `0.999830789` re-derived from its definition by an internal test, pow-by-squaring, 31-half-life underflow skip whose tested claim is the *equivalence*), `EWMAEstimator` on the explicit-materials signature (imminent keyed on the INSTANT never the index; live cap/floor clamps on both passes; the dispersion draw applies to the imminent settlement too), the conservative `Aggregate` (one quantile + one artifact per sum, hash-only difference refused) — and `driven`'s adapters: the conservative `BasisForecast` (declared-zero favorable; measured adverse move × ask-priced notional, away-from-zero) and the **§8 conservative cost bound** (worse branch of depth-walk+adverse vs p90-conditional-on-size, on the WORSE SIDE of each book since the request names no direction; refusals by name for uncalibrated tactic, longer-than-calibrated latency, out-of-range size, thin depth). The `math/big` oracle: 256-bit big.Float EWMA (tolerance 2e-6, budget stated) + big.Rat interpolation judging outward-conservatism, both with inline broken-implementation controls, 500 rapid iterations. **Checklist BEFORE battery found three collapses** (11th–13th instances: `horizon[0]` pinned to `NextSettle` hiding an index-keyed imminent rule; the median constant pinned from one side; every cost product exact at scale 9 so no rounding mode was visible); **27/27 aimed mutations caught** (one NOBUILD re-aim). Slice D's row: **3.0G/day measured** (§32m-D in the capture design record; funding alone ~114M/day, books are the cost, the honest 23:00–04:15 absent window inside the figure — and the §38f off-host decision now has a disk clock on it). Marker grep: still **exactly one** (§40d's, until slice C's close). `make ci` green, all six gates clean. **Start at `## START HERE` below.**

## Status (2026-07-31, session 46 — HISTORICAL): **v0.9.16 — STEP 3 SLICE A IS BUILT: the strategy context skeleton stands, and the fee seam is real.** Step 3 was designed session 45 (design record 2026-07-31 §40, four adversarial review rounds) and slice A built session 46 the same day (§40l is the build record — read it, then that record's `## RESUME HERE`). Landed: `internal/strategy/` full anatomy — domain (the §40b roster as validated value types; `DecisionInput`/`Decision`, the Decision embedding its whole input so §40f's recompute property holds by construction; the versioned wire DTO, mantissa-as-string, byte-level round-trip battery), application (§3.3 `Strategy` VERBATIM + the seven ports + the `Decide` door refusing via `ErrCoreUnbuilt` until slice C + the compile-checked adapter), infrastructure/driven (reference document schema beside its `ReferenceStore` — provenance refusal, as-of refusal naming the input). Profiles gains TWO dated observation sections (the four BTCUSDT lot cells from the venues' public APIs; the four base-tier fee cells from the official fee pages rendered in a real browser — both venues' fee pages time out a plain fetcher, §40d's scraper rejection observed live), and `configs/reference.btcusdt.json` is the committed milestone-2 revision citing those rows, hurdle sourced from treasury.gov's 13-week close (3.69% on 2026-07-30 — lower than the spec's illustrative 4.6%, the never-hardcode rule earning its keep). `engine/state` gains `StrategyReader` — its doc's deferral trigger (the first consumer-owned read port) fired with the port and the translation landed the same session. Checklist-before-battery found four fixture collapses (three orientation pins + the hex-case fixture whose numeric-only hash had no letter to uppercase — found BY the battery's one survivor, tenth instance); **24/24 aimed mutations caught** (one SURVIVED diagnosed into the fixture fix, two NOBUILD re-aims, one deliberate NOBUILD-as-caught on the rule-2 assertion). Marker grep: **exactly one**, §40d's two-clause §2.4 edit taken at the `ReferenceData` port, landed at slice C's close. Capture note: Bybit spot book absent ~23:00–04:15 UTC (30→31 Jul), recorded honestly (COVERAGE verdict, 0 integrity findings), self-recovered; slice D's one-day figure (~09:53 UTC) is STILL DUE and will include the gap. `make ci` green, all six gates clean. **Start at `## START HERE` below, not at this line.**

## Status (2026-07-30): **v0.9.16 — MILESTONE-2 STEP 2 IS COMPLETE: slice C is CLOSED, and the parity probe passes on production data.** Slice C (designed §39p session 43, built same session, closed session 44) wires replayed archive → ingestion (`AcceptOrdinal` minted per consumed record, §3.5 verbatim) → `marketdata` → mailboxes → dispatcher+`LogWriter` → `engine/state` under `TestClock`, and §39j's done-when holds: two Generate runs over the committed production fixture produce **byte-identical dispatch logs** and equal terminal cache states, and the directed Rerun following that log **rewrites a byte-identical log** to the same terminal state. The close ran the full rhythm: a replay unit battery over a crafted equal-instant contention fixture (in-package deliberately — the tape's `CaptureSeq` tie-break arm is UNREACHABLE through `Load` over an honest archive, where file order is seq order), the fixture-collapse checklist BEFORE mutations finding **five pinned parameters** (§39q-2: no test read the clock at envelope dispatch; the staged sink error sat on the collapsing LAST slot; the Bybit snapshot reset was observable in no test — every refusal fixture ran cold; total-only probe counts one venue could carry; and the REST status-range clause was indistinguishable from the error clause because `RestMeta`'s conflict rule ties them at status 0), then **29/29 aimed mutations caught** with **two survivors diagnosed rather than absorbed** (§39q-3): the driver's timer-tie `≤` is an equivalent mutant because the dispatcher's rank fires due timers first anyway, and `ErrSuperseded` is **structurally unreachable** in the sim's exactly-once world (`advances` fails only STRICT regression; per-source dispatch is monotone) — `TestEqualInstantRedeliveryIsAbsorbed` pins the absorbed behavior and the CaptureSeq-later last-writer, `TestUnbudgetedVenueIsFatal` closes the fatal direction's real gap, and the predicate stays as milestone-3 transport policy. **One core edit:** §3.5's `Envelope.Validate` clause narrowed to `IngressSeq` — the §39m-5 absence door, opened by the wiring and now normative, the marker flipped to `[SPEC EDIT LANDED — v0.9.16]`. Profiles gains the dated Bybit `instruments-info` measurement (clamps ±0.005 exactly symmetric — the floor mirror's ground; `fundingInterval: 480` **minutes**, the units trap live). The reading pass found ONE stale unmarked claim the grep cannot see (`driving/funding.go` attributing the ordinal refusal to `Validate` — corrected to name the two refusers). Marker grep **ZERO**. `make ci` green, all six gates clean, docs 102 pass, `docs/formal/` untouched at Cx8. **Start at `## START HERE` below, not at this line.**

**v0.9.15 (2026-07-30) — capture slice F is CLOSED: the archive now testifies against its own copies.** Slice F (§38, opened session 38, paused at a recorded mid-slice seam, closed session 39) lands the full-verify layer: every clean close writes a `_manifest/<day>.json` entry (sidecar-first, day keyed to the WINDOW's UTC day — the 23:00 file belongs to its own day), verify cross-checks entries against recomputed files with five new findings (`manifest_missing_file` — the deletion evidence the layer exists for — `manifest_mismatch`, `manifest_unreadable`, `unmanifested`, `manifest_absent`) plus two paid-for guards (a changed file earns `digest_mismatch` ONCE, not a manifest echo; a header-abort's empty digest is never compared), `capture manifest -day` backfills pre-manifest days from sidecars **labeled** so a repair can never impersonate close-time truth, and `capture cat` renders NDJSON — core §3.6 rule 8's "where a kind becomes a name" sentence made TRUE, not edited. The done-when is a test: a faithful copy verifies clean, a copy missing one closed hour is caught by the entry that outlives the file — provable without asking the source (§38e; destination stays the user's open §38f decision, machinery no longer waits on it). Verification, full rhythm: checklist pass BEFORE mutations found **three pinned fixture parameters** (sorted==insertion order, no out-of-day backfill candidate, a single-day verify universe — the day loop had never been seen to advance) plus a torn-tail backfill fixture; then **30/30 aimed mutations caught**, after the battery exposed two gaps the checklist could not — nothing DROVE a failing sidecar write (the closeFile gate had never refused) and nothing followed a crash-recovered file to its close's manifest entry (both tests added, §38h). One spec edit, in-record: §32e's manifest field list widened by `{window, last_seq, closed_at, source}`, dated. Marker grep **ZERO**. `make ci` green, all six gates clean, docs 102 pass, `docs/formal/` untouched at Cx8. **Start at `## START HERE` below, not at this line.**

**v0.9.14 (2026-07-30) — capture slice E is CLOSED and the `fundingInfo` REST OVERLAY IS LIVE: the capture runs on EIGHT streams.** Slice E was taken up ahead of slice D's one-day disk figure deliberately — the figure is a one-command calendar checkpoint (measurable ~09:53 UTC 2026-07-31), the overlay's own data is calendar-bound, and §32m's stated ordering principle cuts for starting it, not idling (record §37's preamble). One session, full rhythm: design record with rejected alternatives (§37a–§37g), implementation over the seams slices A–C left ready (`RecordKindRest`, `Writer.Rest`, the framing contract), the fixture-collapse checklist BEFORE the mutations (three fixture improvements — §37h), **14/14 aimed mutations caught** (one honest `NOBUILD!!` re-aim), and the close's reading pass finding the one owed spec edit the marker grep could not: the config test now asserts an EIGHTH capture stream while core §8/§12.2 stopped at seven — the v0.9.13 rows-1/4 lesson repeating one session later, landed as v0.9.14's single edit (§37i). A failed poll is a Rest record with the transport's words, not a Gap; verify's `no_data` now requires zero Data AND zero Rest records (§37g); the poller lives in `transport` with its own http/https scheme rule. Marker grep **ZERO**. `make ci` green on a cold cache, all six gates clean, docs 102 pass, `docs/formal/` untouched at Cx8. **Start at `## START HERE` below, not at this line.**

**v0.9.13 (2026-07-30) — the FIRST MILESTONE-2 release: capture slice C is CLOSED and CAPTURE IS LIVE.** The calendar moment happened this session: `bin/arblab capture run -config configs/capture.json` is running detached on this machine, all SEVEN of §32c's sources connected, and the first real `capture verify` over the open hour reported **0 integrity findings** — the 14 coverage findings are the honest description of an hour not yet closed (no Close record, 1/60 Alive ticks at the time of asking). Before the close, the fixture-collapse checklist over the session-35 code confirmed **all three named suspects and found three more** (a scheduling-rate collapse, a one-source label-set collapse, two unexercised shutdown doors — six new fixtures), and **11/11 aimed mutations were caught**. The close landed **six spec edits** (five pre-registered on the capture design's §12 roster + the module rename): core §3.5 ×2 (per-`(Source, Topic)` capture counters; `Wall`'s monotonic clamp stated with its `Backsteps()` observation seam), §3.6 rule 8 (second integer-enum exception: `capture.RecordKind`), §11 (`nhooyr.io/websocket` → `github.com/coder/websocket`), and §12.2 + §8 (self-capture is funding AND books + one evidence stream). `grep -rn "SPEC EDIT OWED" internal/ tools/` returns **ZERO**; four markers read `[SPEC EDIT LANDED — v0.9.13]`. The reading pass found a wrong rule citation (code fix) and caught roster rows 1/4's "no code asserts them yet" having silently expired. `make ci` green on a cold cache, all six gates clean, docs 102 pass, `docs/formal/` untouched at Cx8. **Start at `## START HERE` below, not at this line.**

**v0.9.9 (2026-07-27) — the fourth-slice release, and the one that CLOSES `engine/venue`.** The slice built the normalized funding DTO that retired `marketdata`'s `driving.FundingUpdate` placeholder, and produced **three** spec edits — **all silences in the core rather than disagreements with it**, the v0.9.6 shape rather than v0.9.8's. **This is milestone 1's first close containing no code change**, which is what "the code already agreed with the spec, the spec had just never said it" looks like from outside. **Two of the three are a sharper kind of silence and the distinction is the finding worth carrying: the core does not omit them, it states them twice, incompatibly.** §3.5: the **market-data seam was declared twice** — a flat `DataSource{ Subscribe(VenueInstrument) <-chan Envelope }` here and three per-stream methods in §4, for the same pipeline position, with no type satisfying both. **§4's per-stream shape wins and the seam is `MarketDataSource` throughout**, on three grounds — one channel per instrument merges three `SourceID`s and forces the subscriber to re-derive a key the seam could have kept; the per-stream product constraints are **inexpressible** in `Subscribe(VenueInstrument)`; and §7.6's capacity ladder gets spent on a book feed a funding-only consumer never wanted. The tell that §4 was current is structural rather than a preference: **every other line of §3.5's seam block is written as a reference to §4**, while `DataSource` was the one written as an independent definition — and `venue-profiles.md` §3 was **already** writing `MarketDataSource.StreamMark`, which is cheaper evidence than any of the three arguments. §3.5 also: **a `SourceID` MUST refuse an `(instrument, stream)` pair no venue can produce** — §4 marks funding and mark perp-only per stream and §3.5 never carried it across, so a spot funding source validated cleanly, and §3.4 makes `SourceID` the mailbox key, so an impossible source is **a mailbox that exists, can be subscribed to, and never delivers**, indistinguishable downstream from a venue gone quiet. The rule states the **full table including the negative rows** (the private streams and control are *not* product-constrained), because the constraint reads like a market-data property and is not one — §4.2 makes spot a full API family with its own user-data stream, so inferring it would silently unsubscribe half a cash-and-carry trade. §4: **the units discipline that covered money now covers time.** `fixedpoint` puts a money field's dimension in its type; a duration has none, and a venue duration field's unit lives only in its **name** (Bybit's `fundingIntervalHour`) — so an adapter can multiply by `time.Minute`, satisfy the never-`float64` rule completely, and hand §2.4 a 480-minute interval that enumerates a plausible settlement count while **§12's kill-gate carry figure is wrong by a factor of sixty with nothing failing**. That is §10's own v0.9.7 argument applied one layer earlier, where the duration is *created*. A zero is **refused, never defaulted**: it arrives from three situations a fallback answers identically, two are a wiring failure wearing a correct BTC reading's clothes, and the third **absorbs §2.4's C8 flip**. Three code markers now read `[SPEC EDIT LANDED — v0.9.9]`, `grep -rn "SPEC EDIT OWED" internal/ tools/` returns **zero**, ADR §3 gains the **milestone-1 fourth-slice block** (tagless total now **thirty-three**) and the `C8` traceability row gains core §4, `docs` 102 pass, `make ci` green. **No formal-model change; `docs/formal/` is untouched and still at the Cx8 verdict.** Full reasoning with rejected alternatives: decision record **§20** (the slice) and **§21** (the close).

**v0.9.8 (2026-07-27) — the third-slice release, and the one that CLOSES `marketdata`.** The slice built the first bounded context on the funding path — `domain/` + `application/` + `infrastructure/{driven,driving}` + `module.go` — and produced **eight** spec edits. **The lead one is a different kind from every milestone-1 edit before it: a violation of the core, not a silence in it.** §3.5 already required a dispatched envelope to carry the `AcceptOrdinal` of the input that triggered it; `driven.LanePublisher` published zero, so a published funding snapshot could not be traced back to its cause — and its doc comment confidently argued the rule did not apply. **The `[SPEC EDIT OWED]` grep could not have found it**, because a violation is by definition unmarked; the reading pass beside the grep is what caught it, which is why that pass is not replaceable by the grep. The mechanism generalizes: **a true rule stated without its scope becomes a false rule** — "this layer must not invent a sequence" is true of `IngressSeq` (the replay key, assigned in consumption order) and false of `AcceptOrdinal` (stamped upstream, merely *inherited*), and one sentence covering both is what cost the trace. §3.5 takes four edits: the causal-traceability rule is **scoped, not extended** (it binds every producer, not only book reconstruction — the reconstructed snapshot was an *example* being read as the extent); the envelope derived from the **absence** of an input — §3.4's silence timer — carries **no ordinal**, and reusing the last one is **forbidden** (it makes the capture log claim one raw input produced two dispatched envelopes, so a replay reports a phantom duplicate and concludes the feed is healthy); **`Envelope.Validate()` is the dispatcher's post-stamp check**, so a §3.4 mailbox holds pre-dispatch envelopes and each field now says *when* it is required rather than only who stamps it; and **`Prio` is a per-`SourceID` constant that may not depend on content** (fixing only the direction leaves the `FreshnessBlind` escalation available, which reorders one source inside its own mailbox so **a degrading feed appears to recover**). §3.4: **one budget per (data class, venue), measured against two clocks** — transport age from `RecvTime` against `Clock.Now()` and data age from `SourceTime` against `Clock.ServerNow(venue)`, reported separately and reduced by minimum, because a venue-side stall keeps updates arriving on schedule with a frozen `SourceTime` and the transport verdict reads perfectly fresh for the whole outage. §4: the **`Envelope` is authoritative for provenance** and a normalized payload carries none. §3.2: `marketdata` stops claiming to publish `MarketView`, resolving its contradiction with §3.3 in §3.3's favour. §3.6: rules **1 (`hexboundary`) and 10 (`exhaustiveswitch`) move to enforced**, leaving `singlewriter` the only deferral — and the paragraph keeps two findings worth more than the status change: rule 10's deferral **named the wrong trigger** (it waited on a state machine while the kernel's eleven enum-likes were what made it buildable; it found **13 silent fallthroughs** on its first real run), and **rule 1's obvious positive control is an import cycle**, so the type-checker refuses before the analyzer runs and a clean gate run proves nothing. Six code markers now read `[SPEC EDIT LANDED — v0.9.8]`, `grep -rn "SPEC EDIT OWED" internal/ tools/` returns **zero**, ADR §3 gains the **milestone-1 third-slice block** (tagless total now **thirty**), 7/7 mutations caught, `docs` 102 pass, `make ci` green. **No formal-model change; `docs/formal/` is untouched and still at the Cx8 verdict.** Full reasoning with rejected alternatives: decision record **§18** (the finding) and **§19** (the close).

**v0.9.7 (2026-07-27) — the re-judgement release, doc-only.** Four normative statements the shared kernel's *code* makes that the core had never stated, held out of v0.9.6 because they carried no `[SPEC EDIT OWED]` marker, re-tested and landed: core **§3.2** (+ a D2 pointer), **§7.5**, **§7.6**, **§10**. Details and the process finding behind them are under **NEXT REQUEST** below; full reasoning in decision record **§16**. No formal-model change.


**v0.9.6 (2026-07-27) — the second implementation-feedback release, and the one that CLOSES milestone 1's second slice.** The slice built the **ten-package shared kernel** (`fixedpoint`, `clock`, `apperror`, `ids`, `events`, `config`, `outbox`, `observability`, `ratelimit`, `boot`) and accumulated **eight** deviations-and-additions, each marked `[SPEC EDIT OWED]` in the *code* as it was taken; all eight are landed here, the markers now read `[SPEC EDIT LANDED — v0.9.6]`, and `grep -rn "SPEC EDIT OWED" internal/` returns **zero**. **They are all one kind of finding, and that is the result worth reading:** not one is a disagreement with the core — every one is a **requirement the core relied on without ever stating**, found by having to write the code that would otherwise have relied on it silently. §3.2: the outbox's at-least-once export is **also per-`Key` FIFO** (audit is *a projection of the OMS journal*, and a projection that applies a fill before its order does not reproduce its source — so a delivery-*count* guarantee was leaning on an unstated delivery-*order* one; scoped to the `Key`, because global FIFO lets one poison record halt every consumer), and the **bootstrap phase roster is a set of constraints, not a list** (signal first, because a Go process's default SIGINT/SIGTERM disposition is to *die*; observability before anything that can fail, because every later phase reports through it — both failures silent, and the roster stays **open** since §11's three binaries differ). §3.5: **`SourceID` composes `VenueInstrument`** rather than restating four flat fields that could not express `ContractType`. §5.2: hot-reload is **per-field default-DENY** (`config:"hot"`), with an *unrecognised* tag value an error rather than a silent deny — the one place where the safe direction would also have been the silent one. §7.4: the **total graceful-shutdown budget MUST be strictly less than the watchdog's host-silence threshold**, refused at startup, or an ordinary restart is indistinguishable from a dead host and a routine deployment becomes an operator-cleared incident. §7.5: its preconditions **latch or self-clear**, and clearance returns a latched one to *unreported*, never to *passed* — **it restores the ability to pass, not the pass** — because a uniform checklist would let a check that failed and then passed on retry re-admit trading, which is §7.4's forbidden "never health-flap re-entry" arriving through §7.5's door. §7.6: the ~70% threshold, §7.1's reserved top-up allocation and §6's reserved recovery budget are **one priority ladder** (`Opening` → `Routine` → `Safety`) in which **monitoring outranks new exposure** (shedding reads to protect trading makes the rate limiter itself the cause of the stale mark that fires §7.1's de-risk) and **capacity is a ceiling for every class — the safety reserve is an exclusive band, not a bypass**. §10: metric **label cardinality is default-deny**, because §3.4's lossy-lane guarantee is about a stalled *consumer* and does not cover unbounded time-series growth, where every scrape succeeds and zero drops are reported for the whole duration of the outage. ADR §3 gains the **milestone-1 second-slice block** plus the note on why these carry no `[→ …]` tag (a `M1-#` family would have covered eight of eighteen, and a greppable scheme that silently omits the rest reads as complete — rejected on this project's own standing lesson). **No formal-model change; `docs/formal/` is untouched and still at the Cx8 verdict.** Two things this release deliberately did *not* do, both recorded as scope calls for the user: **four unmarked normative statements were left out** (decision record §15b — `outbox`'s `Seq`-as-dedup-key and committed-bytes payload, `observability`'s integer-typed `Sample` with unit+asset tags, `ratelimit`'s upward-only header ratchet, `boot`'s four-rung posture), because the marker count is what makes the release gate mechanical and an unmarked ninth would break that correspondence permanently. **And one process finding fixed in passing (§15c): `tutorial.ka.md` was stamped v0.9.4 against an English v0.9.5** — the v0.9.5 release bumped the status line and never re-ran the i18n pipeline. The drift was entirely in the frozen provenance line, and **the invariant checker had already caught it** — `assemble.mjs` refuses outright, `bun test` reported 5 failing. The mechanism worked; nobody ran it. Re-synced and re-assembled (**102 pass / 0 fail**), and the release checklist in the decision record now names it, because *"the doc set is stamped"* and *"the generated doc set is stamped"* were two different claims with only one being checked.

**v0.9.5 (2026-07-26) — the first implementation-feedback release, and the first spec change driven by writing code rather than by review.** Milestone 1's first slice accumulated ten deviations from and additions to the normative core, each marked `[SPEC EDIT OWED]` as it was taken rather than applied silently; all ten are now landed, so **R2d reads deliberate change rather than drift**. Nothing here comes from a review round. §3.2/§11: the **`boundary/` grouping segment is dropped** — contexts sit at `internal/<ctx>/`, settled by the observation that *none of §3.6's gates depend on the prefix*, so it carried zero enforcement value. §3.5: **`TimerSource`** (timer consumption is pull-based on an interface separate from `Clock` — a Clock pushing due timers would put the arbitration that decides the replay key in two places) and the **timer priority direction made normative** (lower `Prio` is more urgent, so the zero value is the most urgent; the spec gave the tie-break order but not its direction, and the opposite convention silently sends an unset safety timer to the back of the queue). §3.6: rule 4 gains `nofloat`'s type-based character and the **error-precedence contract** (invalid mode → divide-by-zero → **overflow** → precision loss — *found by an oracle disagreeing with the implementation, not by review*); **rule 5 is restated as the whole ambient-time family and widened to `engine/`** with a transport-only escape; rule 8 documents `RoundingMode` as an integer-enum exception; **rules 9 (`kernelpurity`) and 10 (`exhaustiveswitch`, planned)** are added, with a gate-status paragraph recording which of the six are enforced today. ADR §3 gains the milestone-1 implementation-decision block, including **Go evaluated-and-confirmed against Rust** — recorded with Rust's genuine wins stated honestly, the GC-pause argument explicitly debunked, and the decisive structural counter, so it reads as a decided question rather than an unexamined assumption. **No formal-model change; `docs/formal/` is untouched and still at the Cx8 verdict.**

**v0.9.4 (2026-07-25):** **the genuine independent Codex round-2 on the formal model is done — verdict NO-SHIP, two MAJOR findings, both valid, both applied and re-verified: `./run.sh` → ALL 8 RUNS MATCH PREDICTIONS** (nominal safety 3,983,804/445,216 with the new `I8`; operator-assisted liveness 3,945,352/491,160; automated convergence 1,705,418/249,936 with the new `P3`+`P4`; NC1–4 violate as predicted; **NC5** — the first negative control over a *temporal property* — violates `P4` as predicted; ≈32 min, of which `_Live2` alone is 23 min). **This discharges the re-review obligation Cx6 raised and Cx7 could not deliver:** an independent reviewer ran the suite, wrote its own TLA+ probe module, and produced both findings as counterexample lassos — so FM-3 and Cx7-1/-2/-3, previously self-certified, have now survived a review explicitly pointed at them. One normative change — **FM-4: the arbitration branches on the *recorded* failure, not on live health.** Which dimension failed is snapshotted when the loss is detected, so a dimension that repairs inside the detect→select window does not abort a *pending* branch — **the one-way rule binds from detection, not from selection**; the 2-vs-3 choice still reads *live* capability, because "latchable + `ActuatorReady` confirmed" is a claim about acting *now*. Without it all three branch guards re-read live health and **go false simultaneously** on a self-healed loss (branch 1 needs a live failed dimension, branches 2/3 need live signing loss), stranding the incident in un-arbitrated `FenceReadyLost` — openings blocked, exposure open, no branch, no automated exit. It also **disproved the model's own SF→WF soundness argument** (branch guards were claimed *persistent* under `EnvStabilizes`; the trace is a guard enabled at one instant and never again) — now true by construction. Rejected alternatives: a health-driven return from `FenceReadyLost` with no branch started; atomic detect-and-select (both recorded with reasons). Plus **Cx8-2/-3**, model/verification only: `HeldIncident` is **two states wearing one name** (SM §4 gives the readiness park and the exhaustion park different exits; the cause-blind re-entry guard exited both and spun forever), and an escalation terminal that is never *actually* terminal still discharged `Branch2AutoResolves` — one transient visit satisfies a `<>`. **Honest note: applying the Cx8-2 fix introduced a liveness dead end of its own** (a manually-flattened exhausted park became unclearable), caught by `_Live` and fixed from the spec's own "any later fill re-classifies". Record: the **Cx8 addendum** in `docs/review/2026-07-20-10-…`.

**v0.9.3 (2026-07-25):** **the Cx7 round-2 review of the formal model is applied and fully re-verified — `./run.sh` → ALL 7 RUNS MATCH PREDICTIONS** (nominal safety 2,725,268/301,952 with the new `P1`; operator-assisted liveness 2,927,252/357,136; automated convergence 1,366,854/197,600 with the split branch properties + the new `P2`; NC1–4 violate exactly as predicted; ≈20 min). One normative change — **FM-3: takeover engagement is a *durable obligation*, not a delivered message.** An actuator that boots (or stands down) into an incident whose §3 branch is `TakeoverTransfer` with an active plan and **no latched epoch** MUST re-attempt the fence latch, keying off the durable branch record rather than a received hand-off signal (so a lost hand-off and a crashed actuator are the same case); the re-attempt is `T_fence`-bounded to the fence-unavailable floor as usual. Without it the branch transferred the writer to **nobody**: the bar on the bot is scoped to the *branch*, not to the dimension that failed, so a pre-latch actuator crash left **unprotected exposure with both writers alive and idle, indefinitely**. Stated corollary (SM §3): a branch that transfers the writer MUST guarantee the transferee engages. Rejected alternatives: watchdog re-arm; bot-side re-drive (both recorded with reasons). Plus three **model/verification** fixes with no normative content: **Cx7-1** (the completion path escaped the Cx6-2 `HeldIncident` park — now capability-gated, with the park's *other* spec-stated exit implemented, without which the safety fix would have been a liveness dead end), **Cx7-2** (the automated-convergence property was certifying FM-3's stall as "resolved" and was vacuous for branch 1 — now split + conditioned), **Cx7-3** (nothing tied a completion transition to execution capability — `P1`/`P2` are now permanent suite members). **Provenance caveat: this was not a clean Codex round** — three dispatches died on an OpenAI outage, and everything beyond two streamed seed observations was self-probed. **A genuine Codex round-2 against the fixed artifact is still owed** (RESUME block below). Record: the **Cx7 addendum** in `docs/review/2026-07-20-10-…` (read its Provenance section first).

**v0.9.2 (2026-07-21):** **the Cx6 Codex adversarial round — run against the model itself — is applied and fully re-verified: 7/7 TLC runs match predictions** (nominal safety 2.77M states, unbounded crashes; `IncidentResolves` + the new `Branch12AutoResolves` automated-convergence config both green; NC1–4 violate exactly as predicted; ≈13 min, `docs/formal/`). Cx6-1…6, all valid: **branch-3/2 bot-write gating + non-atomic manual kill with over-close fill semantics** (Cx6-1, model-only — SM was already normative); **`ActuatorReady` gates the takeover end-to-end** (watchdog-path latch, every actuator venue write incl. quiesce cancels, **mid-takeover loss → `HeldIncident`**) — **spec edit** core §7.4 + SM §4 (Cx6-2); **clearance is health-gated with a degraded-return** — **spec edit** SM §3 (Cx6-3); I1 instruments actuator cancels (Cx6-4); **liveness de-masked** — `OPERATOR_INTERVENES` + `LiveSpec2`/`Branch12AutoResolves` + `EnvStabilizes` conditioning (Cx6-5); state counts refreshed to the reviewed revision (Cx6-6). **The liveness re-verification then found FM-2:** a spurious watchdog fire that latched nothing and started no branch had **no stated exit** — with R1d-F6 clearance the only mechanism, acknowledging a false alarm would demand flattening a healthy trade (a fair 9-state lasso; the model was also missing the operator's kill-switch trip entirely). Resolved: **false-alarm dismissal** (a fire that engaged nothing is operator-dismissed as an alert; the branch-exit preconditions are scoped to a *started* branch) — core §7.4 + SM §4 `[→ FM-2]`; model gains `OpTripHalt` + `OpDismissFalseAlarm`. Checking-methodology close-out (documented in `run.sh`/cfgs/correspondence doc): `-lncheck final`; arbitration fairness **WF** (the weaker assumption — a stronger theorem — with SF retained only where load-bearing: `ActLatch` phase-alternation + the five operator actions). **Still owed: the Codex round-2 re-review (RESUME block below).**

**v0.9.1 (2026-07-20):** **formal model (b) is built, checked, and it found a real defect.** The FenceReadyLost → takeover → EmergencyPlan composition (SM §3/§4/§6) is model-checked in TLA+/TLC (`docs/formal/`: `FenceTakeover.tla` + 6 configs + pinned runner + `fence-takeover-model.md` correspondence/results). Nominal safety passes over ~3M states (unbounded crash/restart, watchdog false positives, venue races, reorder/retry); liveness (every incident eventually clears) passes under obligation-scoped fairness; four negative controls each violate exactly the predicted invariant — non-durable latch (the SM §4 signer-restart warning), issuance-gated completion (Cx5-1), unfenced canary action at `$1K`+ (the §7.4 tier rationale), and **FM-1**. **FM-1 (found on the first nominal run, resolved → v0.9.1):** v0.9 never stated the disposition of a trade the takeover ran to completion evidence on — clearance (R1d-F6) + the `ActuatedFlat`→`Deactivate` crash-window re-drive (R1d-F7) + §6 ack-before-entry compose so a cleared bot re-opens exposure under the still-`Active` plan and the stale re-driven tombstone revokes host-independent coverage under live exposure. Fix: **completion evidence terminalizes the trade** (no exposure re-admission; tombstone-only lifecycle; fresh exposure = successor trade via the lease; the tombstone-as-clearance-precondition alternative rejected on the R1d-5 coupling argument). Record: `docs/review/2026-07-20-10-…`; traceability: ADR §3 **FM block**; `FM-#` registered in the core Conventions. **This delivers model (b) as evidence for the still-open formal-methods adoption decision — it does not preempt it; model (a) is unstarted.**

**v0.9 (2026-07-20):** the **first round-4 external reviewer response (tagged R1d; R2d still pending)** is triaged and applied over the v0.8 five-doc structure. All 11 findings were valid in substance (0 discarded; 3 framings corrected / 1 proposed fix replaced — record: `docs/review/2026-07-20-09-…`), and **all 7 FLAG rulings are adopted — FLAG-1…7 are resolved** (ADR §4). Two blockers closed: the takeover now **terminates flat** (state-dependent end action incl. a two-leg close of a still-neutral package — R1d-1) and the interval-floor pre-action capital bound is **`max(|a|,|b|)`** (R1d-2). Plus: classifier re-runs on every lifecycle/finality event (R1d-3); **`ActuatorReady`** monitored dimension + branch-2 precondition, durably-fenced reclassified (R1d-4); plan-generation **prefix-complete discovery** (per-generation ack rejected for hot-path handles, required for policy edits — R1d-5); **margin ops = durable journaled target-state machine** with per-venue evidence resolution (R1d-6, venue-verified 2026-07-20); **Bybit forced-close evidence corrected — no `closeType` exists; `execType` is authoritative** (R1d-7, venue-verified); canary sequencing made implementable (actuator = canary-concurrent workstream — R1d-8); **stale-mark escalation defined** (REST failover + `T_blind` de-risk — R1d-9); tutorial re-synced (R1d-10); governance tightened (SM = normative-by-reference; tag header complete; Profiles citations honest — R1d-11). A **Codex convergence pass (Cx5-1…5)** then hardened the application itself: takeover completion **evidence-gated** (`CloseInFlight`/`HeldIncident` — never a false "flat" on a zero-fill close); the core's missing 1→3 escalation restored; a **`HistoricalOrders`** discovery port + venue-retention bound + acked checkpoint generations behind the prefix-completeness claim; `MarginOpStatus` gains first-class `Ambiguous` + same-position serialization; shorthand cleanup.

The normative content is now **v0.9** (v0.7.1 + the R1d round-4 resolutions above) in the five-doc structure the v0.8 doc-split created (`build-spec.md` normative core + `state-machines.md` + `venue-profiles.md` + `tutorial.md` + `adr-history.md`). History: v0.7 absorbed a **second round of two external human reviews (R1, R2)**, a **re-verification of the new load-bearing venue/tooling facts** (Binance/Bybit isolated-margin top-up + position-mode + key-management endpoints, SQLite durability — official docs, 2026-07-19), and **four Codex adversarial convergence rounds** (17 findings, all valid, all applied) — most in the **safety kernel** (ambiguous-order recovery and the off-host actuator). **v0.7.1** then absorbed a **third external round (R1c, R2c) + a Codex convergence pass** — refinements/corrections on v0.7's architecture (not a new redefinition): mark-price-driven liq monitoring, a `FenceReady` health gate + proactive wind-down, first-class safety-capability ports, an explicit three-failure-domain topology, a decoupled dispatch sequence, signer-isolation-as-primary-`Fence`, interval-floor compensation, a quantified canary loss bound, and real durability/backup requirements. **R2's two canary blockers (mark-driven liq, quantified loss) + two `$1K` blockers (incident-available fence, safety-plane topology) are closed at the design layer; residual gating is implementation + chaos-test.** The spec remains **deliberately not "converged"** and **tiered by capital gate**: the **foundation + read-path + edge-proof (milestones 1–2) are fully build-ready**.

## Doc-split executed (2026-07-20): v0.7.1 → v0.8 — **DONE**
The **doc-split** (both round-3 reviewers' recommendation) is complete. Design: `docs/superpowers/specs/2026-07-19-doc-split-design.md` (78bfd87). Plan: `docs/superpowers/plans/2026-07-19-doc-split-execution.md` (540f18b). Execution: Tasks 1–5 (`ba1e4ad`…`2bf55de` — adr-history extraction, venue-profiles extraction, state-machines ×2, core trim, a restore-fix) plus Task 6 (`5548f19` — cross-refs + HANDOFF + tutorial; stamp v0.8). Content frozen from v0.7.1 verbatim; **reorganization only, no normative change** — checked by the mechanical cross-ref/stamp sweep (Task 6) and **confirmed by the two-round Codex losslessness pass (Task 7)**: round 1 found 2 BLOCKERs (stub wording in core §4/§4.2), fixed in `72a3ef2`; round 2 confirmed **EQUIVALENT**. The **safety kernel is now seven explicit state machines** in `state-machines.md` (OMS order lifecycle, ambiguous-order classifier, FenceReadyLost arbitration, actuator takeover, protective-order disarm, EmergencyPlan lifecycle, PackageDelever generations) — what moved where: **ADR §4**. Structuring the prose into explicit transitions surfaced **seven flagged ambiguities** (FLAG-1…7, ADR §4) in the source material — deliberately left unresolved in v0.8; **all seven are now resolved in v0.9 by the round-4 R1d rulings** (ADR §4 records each ruling).

## Interactive guide (orientation before implementation) — **stage 1 published 2026-07-25**

`docs/guide/index.html` — a single self-contained interactive HTML guide, published at <https://claude.ai/code/artifact/710d7ac1-98b2-45e7-9464-309829ebaa4d> (redeploy by publishing the same file path; keep the 📉 favicon). Its purpose is orientation: making the 415 KB five-doc set comprehensible **before** milestone 1 starts, with the safety kernel included at a teachable altitude — the gap `tutorial.md` deliberately leaves ("below this tutorial's altitude").

Design: `docs/superpowers/specs/2026-07-25-interactive-guide-design.md`. Plan (stage 1): `docs/superpowers/plans/2026-07-25-interactive-guide-stage-1.md`.

**Governing rule — teach-and-point.** The guide owns *understanding*; `build-spec.md` and `state-machines.md` keep owning *requirements*. It never restates a requirement, and **every factual claim carries a citation chip** (`<span class="cite" data-src="core §7.4">`), which is what makes drift against a future review round (R2d) mechanically detectable: `grep -o 'data-src="[^"]*"' docs/guide/index.html | sort | uniq -c`.

**Stage 1 = Parts 1–2 + the economics calculator.** Parts 3–6 (why the design looks like this · what we're building + the state-machine explorer · the Go build · the operator runbook) are **pending the voice-review gate** — do not write them until the user has reviewed stage 1's voice and density. Tests: `cd docs/guide && bun test` (40 tests). The calculator is pinned to five figures printed in `tutorial §3.9`/`core §2.4` — $90 funding, $38 round trip, 8.4 days to break even, 5.68% in-position, **3.41% at 60% utilization vs the 4.60% hurdle**, plus the 7.12% the `C_legs` denominator would wrongly show. **If any of those assertions ever fails, the spec and the guide disagree — that is a finding, not a test to relax.**

### Localization to English + Georgian — **DONE (2026-07-25)**

The guide is bilingual. Design: `docs/superpowers/specs/2026-07-25-guide-localization-design.md`. Plan: `docs/superpowers/plans/2026-07-25-guide-localization.md`. Operating instructions live in `docs/guide/README.md` under "Languages".

**Three decisions the user took:**
1. **Georgian renders serif** — Noto Serif Georgian (OFL 1.1) instanced to two weights, subset to Mkhedruli + Latin, inlined as a data URI by `tools/build-font.sh` (21.5 KB + 22.8 KB raw; the file went 58 KB → ~200 KB). Verified in-browser: Georgian text measures 145.8 px in the inlined face vs 135.2 px in the system fallback. Without it, asking for `serif` silently got a *sans* face — `serif` and `sans-serif` measured within 0.5 px of each other for Georgian.
2. **Borrowed trading terms stay in Latin script** inside Georgian prose (`funding rate`, `spot`, `perp`, `mark price`, `leg`), per `docs/guide/i18n/glossary.md`.
3. **English is the default and language rides in the hash** — `#/ka/edge/costs`; `#/edge/costs` means English, so every pre-existing link and the published Artifact URL still work.

**The organizing rule — prose duplicates, controls localize in place.** The calculator article holds 16 element ids; duplicating it per language would duplicate those ids and every `getElementById` in `#guide-ui` would silently bind to the English copy. So prose is sibling `<article lang="en">` / `<article lang="ka">` blocks, while the calculator, search box and theme toggle stay single instances reading from `GuidePure.UI`. That refactor also moved ~40 strings out of `#guide-ui` (which `bun test` cannot reach) and into `#guide-pure`, putting the verdict sentences and output row names under test for the first time.

**`leg` is why the glossary is frozen — keep it that way.** The first bulk run omitted `leg` (41 occurrences, the most frequent domain noun in the guide) and produced **three different words across three sections**: `ნაწილი` (part), `ეტაპი` (stage), `მხარე` (side). All sections were re-translated against the corrected glossary. Never edit `i18n/glossary.md` mid-run; if a rule needs to change, change it and re-run **every** section.

**Two file-text traps found during the work, both invisible to the browser:**
- Citation chips written into a JS string with escaped quotes (`data-src=\"core §2.1\"`) render perfectly but vanish from the `data-src` drift grep the guide's governing rule depends on. The string is single-quoted so the chips survive as literal file text; `tests/structure.test.js` enforces it.
- An attribute selector built as a string literal (`'article[data-route="' + route + '"]'`) registers as a **phantom route** in the raw-text scans. The article fallback is found by iteration instead.

**To re-translate after the voice review** (the English prose is still pending it): `./tools/translate.sh <route>` writes a reviewable fragment to `i18n/ka/`, then `node tools/i18n.mjs splice <route>` — idempotent, and it refuses any fragment whose citation chips differ from the English original. Parts 3–6 translate the same way as they are written.

## Georgian translation of `tutorial.md` — **DONE (2026-07-25)**

`docs/tutorial.ka.md` (112 KB) is the Georgian teaching companion. **It is generated — never hand-edit it.** Design: `docs/superpowers/specs/2026-07-25-tutorial-georgian-translation-design.md`. Operating instructions: `docs/i18n/README.md`. Tests: `cd docs/i18n && bun test` (16 tests). The guide's suite is unaffected (86 tests, still green) — the only shared object is its glossary, read in place and **not edited**.

**Three decisions the user took:** the Georgian is a sibling file `docs/tutorial.ka.md`; **section numbers are frozen and section titles are translated** (`### 3.9 ` byte-identical, title Georgian — every `§3.9` pointer addresses the number); and the `**Status:**` / `**Doc set:**` provenance lines stay **verbatim English** behind a Georgian banner that names the file as generated.

**55 fragments at `###` granularity**, IDed by spec section number (`s-3-9`, `part-8`) — *not* by ordinal, so an English edit that inserts or deletes a section never renames its neighbours. This is what makes R2d cheap: when it edits `tutorial.md`, the Georgian is one `./tools/translate.sh <id>` per changed section. The handoff's "~12 calls by `##`" estimate was wrong on measurement — **Part 3 is 11.2 KB**, 3× the size the guide's sections translated cleanly at, and it holds the §3.9 ledger.

**Assembled, not spliced.** `tutorial.ka.md` is a pure function of the fragments, so there is no insert-vs-replace branch and no regex ever runs over translated prose (the guide needed function replacements to survive `$&`). A test asserts the file equals a fresh assemble — a hand-edit fails the suite instead of diverging silently.

**Six raw-text invariants gate assembly** (`tools/invariants.mjs`, re-run file-wide by the tests): heading number + trailing `` `[→ …]` `` tag · numeric multiset (452 tokens) · inline-code multiset (140 spans) · §-pointer multiset (three syntactic shapes — `§spec §2.2`, `SM §2`, bare `§3.9`) · table shape · link targets. **Invariant 2 closes the gap the handoff flagged as the third fork:** §3.9 is the *source* of the five figures the guide's calculator is pinned to, and the guide asserts them against hard-coded values rather than against this file — so without it the two documents could disagree with nothing failing. Verified independently of the JS checker by a shell `diff` of the two numeric multisets.

**One prompt rule the numeric check depends on:** numbers never cross the digit/word boundary in either direction ("two legs" stays a word; `20 days` stays `20`), or a *correct* translation would fail invariant 2.

**Results:** 54 of 55 fragments passed all six invariants untouched, including `s-3-9` (96 numeric tokens, 34 code spans, 11 pointers, 2 tables) on the first attempt. The single violation was `### 1.6 Mark vs index vs last` — a heading made entirely of glossary-pinned Latin terms, so identity was the model's correct output and the check's complaint was also correct. Fixed in the heading, not the check (recorded in `docs/i18n/README.md`).

**The glossary is shared and stays at `docs/guide/i18n/glossary.md`,** read by both toolchains, never copied. A test fails if a second `glossary.md` appears anywhere under `docs/` — two copies would drift, which is exactly the `leg` failure this project already paid for once.

**Deliberately not done here:** no change to `tutorial.md` (this is a translation, not a revision — the open `core §12` / `core §2.4` finding below is untouched), no change to `docs/guide/`, no Artifact publication.

## START HERE (updated session 66, 2026-08-01 — §46 SLICE B BUILT + VERIFIED, `FM-5` FOUND; next act is SLICE C)

**Branch `feat/milestone-1-foundation`. THE NEXT ACT IS §46 SLICE C —
liveness `P1`–`P4` in `docs/formal/AmbiguousOrder.tla`, each shipping with
its negative control in the same slice.** Read
**`docs/superpowers/specs/2026-08-01-formal-model-a-ambiguous-order-design.md`
§8 (the slice-B build record), then its RESUME HERE**, before anything
else. Task **#154**.

**What stands.** `AmbiguousOrder.tla` model-checks SM §2 **composed with**
SM §7: the classifier, the interval algebra, the finality barrier, the
reserved `IntervalFloorReduction`, and `PackageDelever`'s generation phases
on the one shared reservation ledger, with `INV1`–`INV7`. `./run.sh` checks
both models — 8 runs for model (b), then **35** for model (a) (two nominals
+ `NC1`–`NC12` + **21** positive controls). The composed nominal is
832,179 distinct states at depth 30 and runs in about eight seconds.

**`FM-5` IS A FOUND SPEC DEFECT AND ITS NORMATIVE EDIT IS OWED.** SM §7's
FLAG-3 lets a generation book its target into the ledger before the perp
leg has bought anything; on a neutral package that reports a phantom short
and §2 buys back exactly what the generation is about to buy. The model
encodes the **sound** reading (the entry exists from creation, carrying the
generation's *outstanding net intent*, which is zero until the perp
confirms) and NC7 encodes the defective one. §46 §5's roster schedules
findings triage for slice E, so the edit is due there — **the exact wording
to land is in §46 §8d**. Until then the two artifacts disagree, and
CLAUDE.md's rule decides: the formal model is authoritative.

**Two decisions slice C must preserve, not rediscover (§46 §8a, §8e):**

- **`INV1b` lives in `AmbiguousOrder_SM2.cfg` (`MAX_GEN = 0`) on purpose.**
  Its premise is slice A's "no new counter-leg order appears mid-episode",
  which §7's perp reduce breaks legitimately. It was not weakened — it is
  checked where it holds, and that config also reproduces slice A's graph
  to the digit. Do not move it back into the composed config.
- **The generation's own legs are not `RecordDispatch`ed.** They are not §2
  compensating trades; `INV5` owns them. Recording them would make `INV3`
  fire on a generation completing normally.

**Three traps set for slice C (§46 §8g):** budget exhaustion produces a
`Settled` stutter with a possibly-nonzero residual, so `P1`–`P4` must be
conditioned on the budget (model (b)'s `BOUND_CRASHES` is the precedent);
**`MAX_GEN` is now a second such bound** and `P3` must not read a spent
generation count as a stall; and `superseded` gates `PdPlan`, so a liveness
property about *later* generations is vacuous after a supersession and must
say so.

**`run.sh` takes `TLC_WORKERS`.** Cap it (4 is what sessions 65 and 66 used)
whenever the capture is running: model (b)'s liveness configs hold every
core for ~30 minutes and the archive is un-backfillable.

**Also standing, in order after slice C:** (2) the **§32m-D per-tier
figure, due 2026-08-03T11:13Z**, measurable from 2026-08-02T11:13Z → §44e +
§38f's revisit (task #150); (3) **off-host archive sync** — no target exists
yet, recommendation is a ~$60 external USB drive; (4) the **gate** itself
(task #152). Task #153 (the hourly `restart_boundary` on REST-overlay
sources) awaits a user ruling and blocks nothing.

Capture rituals first, always: **pgrep PID 8495**, background verify,
marker grep (**ZERO**). QA binaries to the scratchpad — **never rebuild
over `bin/arblab` while capture runs**; `make ci` writes no `bin/`.

---

## START HERE (updated session 65, 2026-08-01 — HISTORICAL — §46 SLICE A BUILT + VERIFIED; superseded by session 66's slice-B build)

**Branch `feat/milestone-1-foundation`. THE NEXT ACT IS §46 SLICE B —
the reservation ledger's other two producers, `PackageDelever`'s phases,
and `INV4`–`INV6`, in `docs/formal/AmbiguousOrder.tla`.** Read
**`docs/superpowers/specs/2026-08-01-formal-model-a-ambiguous-order-design.md`
§7 (the slice-A build record), then its RESUME HERE**, before anything
else. Task **#154**.

**What stands.** `AmbiguousOrder.tla` model-checks SM §2 — the classifier,
the interval algebra, the finality barrier, the reserved
`IntervalFloorReduction` and its recursion — with `INV1`–`INV3` + `INV7`.
`./run.sh` checks **both** models: 8 runs for model (b), then 7 for model
(a) (nominal + `NC1`–`NC6`), then 8 positive controls from
`AmbiguousOrderCoverage.tla`. **The positive controls are phrased as the
NEGATIVE claim, so a VIOLATION is the good outcome and a PASS is a
coverage gap** — that polarity is in the names, do not "fix" one by making
it pass. Model (a)'s whole block runs in under a second.

**`run.sh` now takes `TLC_WORKERS`.** Cap it (4 is what session 65 used)
whenever the capture is running: model (b)'s liveness configs hold every
core for ~30 minutes and the archive is un-backfillable.

**Slice B's done-when** (§46 §5 + §7f): green, **plus** the positive
control extended so **each of the three ledger producers is shown to
contribute a nonzero term to the one subtraction**. That is §2.1's entire
argument for modeling §2 with §7, and `PC3` is the precedent — with one
producer the subtrahend was pinned to zero and the distinction the code
makes was invisible. Report state counts; do not assume them.

**Three slice-A decisions slice B must preserve, not rediscover (§46 §7a):**

- **`Impl*` vs `True*`.** Guards read the implementation's arithmetic;
  invariants read the ledger-exact truth, and nothing else. New producers
  go into `Reserved`, which is a `True*` input — a producer that only the
  guards can see would defeat the whole split.
- **`floorIssued` vs `floorCount`.** The guard and the dispatch count are
  separate variables on purpose; `INV2` reads only the count. Do not
  collapse them while adding generations.
- **An order is `(dir, q)`.** Intervals keep zero as an endpoint and
  shrink monotonically — that is what makes the state space finite.
  `PackageDelever`'s reserved spot leg must join the ledger, not become a
  straddling order.

**Carried to slice E, deliberately unresolved (§46 §7d):** whether "never
issued twice" carries safety weight independent of the reservation. NC2's
counterexample keeps `overshoot` FALSE throughout. **Do not resolve it by
weakening the spec** — it is an `FM-5` candidate if it survives an attack.

**Also standing, in order after slice B:** (2) the **§32m-D per-tier
figure, due 2026-08-03T11:13Z**, measurable from 2026-08-02T11:13Z → §44e +
§38f's revisit (task #150); (3) **off-host archive sync** — no target
exists yet, recommendation is a ~$60 external USB drive, because slice F's
machinery is already proven against its second-root fixture while §38g
excluded transport+credentials, and the gate's two-month wait now commits
~180–200G of un-backfillable archive to one laptop disk; (4) the **gate**
itself (task #152). Task #153 (the hourly `restart_boundary` on
REST-overlay sources) awaits a user ruling and blocks nothing.

Capture rituals first, always: **pgrep PID 8495**, background verify,
marker grep (**ZERO**). QA binaries to the scratchpad — **never rebuild
over `bin/arblab` while capture runs**; `make ci` writes no `bin/`.

---

## START HERE (updated session 64, 2026-08-01 — HISTORICAL — FORMAL MODEL (a) COMMISSIONED + DESIGNED; superseded by session 65's slice-A build)

**Branch `feat/milestone-1-foundation`. THE NEXT ACT IS §46 SLICE A —
write `docs/formal/AmbiguousOrder.tla`.** Read
**`docs/superpowers/specs/2026-08-01-formal-model-a-ambiguous-order-design.md`
(§46), then its RESUME HERE**, before anything else; it holds the scope
ruling, the variable table, the seven safety invariants, the four liveness
properties and the slice plan. Task **#154**.

**Why this work, now.** §12 hard-gates milestones 3–7 on strategy #2's
economic gate, which waits on evidence until ~2026-09-30 — so the heavy
build is closed, and model (a) is the substantial engineering that is not.
It is **not** a bet on that gate: core §6 is "Trade lifecycle (foundation,
**strategy-agnostic**)", the two-leg partial-failure problem is identical
for spot+perp and perp+perp, and milestone 3 requires the model-test before
any live capital regardless.

**Slice A's done-when** (from §46 §5): the interval algebra + classifier
transitions + `INV1`–`INV3` + `INV7`, nominal TLC run green, **and one
deliberate broken-constant run per invariant showing each one CAN fail.**
Toolchain is verified runnable — Java 25, `tla2tools.jar` pinned by
SHA-256; `docs/formal/run.sh` is the pattern to extend (it already asserts
every negative control FAILS on its predicted property).

**The two traps §46 names before any `.tla` exists — both are the
fixture-collapse checklist at formal-methods altitude, and both are easy to
walk into while feeling productive:**

- `INV2` ("the interval floor is never issued twice") must **count
  dispatched `IntervalFloorReduction` entries**, never read the
  `floorIssued` guard the implementation will have. Written as
  `floorIssued ⇒ ¬issue` it proves the flag consistent with itself —
  green, vacuous, worthless.
- `INV1` must be phrased **over resolutions** ("no resolution of the
  unresolved intervals produces an opposite sign"), never over the endpoint
  formula. The spec already carried a bug in exactly this algebra —
  v0.7.1's `|b|` was correct only for the net-long case and R1d-2 corrected
  it to `max(|a|,|b|)` — so a model that re-implements the corrected
  formula and checks itself against it proves nothing.

**Also standing, in order after slice A:** (2) the **§32m-D per-tier
figure, due 2026-08-03T11:13Z**, measurable from 2026-08-02T11:13Z → §44e +
§38f's revisit (task #150); (3) **off-host archive sync** — no target
exists yet, recommendation is a ~$60 external USB drive, because slice F's
machinery is already proven against its second-root fixture while §38g
excluded transport+credentials (so a cloud bucket is the bigger change),
and the gate's two-month wait now commits ~180–200G of un-backfillable
archive to one laptop disk; (4) the **gate** itself (task #152). Task #153
(the hourly `restart_boundary` on REST-overlay sources) awaits a user
ruling and blocks nothing.

Capture rituals first, always: **pgrep PID 8495**, background verify,
marker grep (**ZERO**). QA binaries to the scratchpad — **never rebuild
over `bin/arblab` while capture runs**; `make ci` writes no `bin/`.

---

## START HERE (updated session 64, 2026-08-01 — HISTORICAL — §45 code-complete + the capture observation; superseded by the §46 block above)

**Branch `feat/milestone-1-foundation`.** Sessions 62–63 landed §45 slice D
end to end, and **there is no crossvenue code left to write for the gate**:
the continuity carry (`efa4bbe`), (b)'s first half (`b6c99fb`), and (b)'s
SECOND HALF — the procedure commit (`7ccadec`) that lifts the §45f-2
post-cutoff seal — recorded as §45p (`d66b553`). **Read
`2026-08-01-strategy2-crossvenue-gate.md` §45p, then its RESUME HERE**; that
record's RESUME HERE is the live statement of what remains.

**What is next, in order — both items are clock-bound, not work-bound:**

1. **The §32m-D per-tier one-day figure, DUE BY 2026-08-03T11:13Z**
   (measurable from **~2026-08-02T11:13Z**, one full day after the 11:13:10Z
   restart), landing as §44e — `du` split BTC-tier vs alt-tier against
   §42c's derived ~0.6–1.1G/day/symbol, plus a full-day verify (0 integrity;
   the restart's gaps show honestly). **This is §38f's named revisit point**
   (ruling §42f-3): re-argue off-host on the measured disk clock. Archive
   stood at 9.4G on 2026-08-01 — context for that argument, not the figure.
2. **Accumulate the gate's evidence, then run the gate.** `arblab crossvenue
   fetch` periodically (the store is keyed by cutoff DAY and refuses to
   overwrite, so a new day accumulates beside the old file); `arblab
   crossvenue gate` reads the evidence table any time. **BCHUSDT is the
   BINDING symbol** — 8h grid against the other seven's 4h, so it accrues
   matched settlements at half their rate and the roster qualifies on BCH's
   clock, around **2026-09-30** rather than the majority's ~08-31. That is a
   forecast, not a schedule: the `matched` column decides. Two operational
   facts, both guards working: the tree must be CLEAN (§41e), and `-before`
   must fall on a day AFTER the last complete archive day (H2 — the open
   hour is not yet vouched by its day manifest). A not-yet-scoreable run
   exits **2**, names every short symbol, and writes NOTHING.
3. **When it scores**, the write-once artifact authorizes nothing until a
   detached attestation binds it (24h cooling, sign-off, out-of-band). That
   is a **USER decision** under milestone 2's kill discipline.

**One interpretation §45p had to take, recorded so it is not silently
reopened:** a symbol failing either admission rule refuses the whole RUN
rather than being dropped, because ruling §45f-3's pass shape is a
PRE-DECLARED portfolio and scoring the survivors would score a portfolio
nobody declared, chosen by whichever symbol happened to be thin. If that
reading is wrong it is a ruling to revisit with the user, not a bug to patch.

**NEW (session 64) — an observation measured, not absorbed, in the spirit of
§45m's Bybit-coverage fork.** The standing verify ritual came back **0
integrity / COVERAGE** as expected, but its coverage list carries a
`restart_boundary` finding on
`binance/…/BTCUSDT/funding/fundingInfo` **every hour** — hours 01…10 before
the restart and 12…15 after it, so it is chronic, not a restart artifact.
The cause is a designed property meeting an undesigned consequence: slice E
settled that **"a Rest record is not a Data record"**, so this REST-overlay
source mints `records 63, data 0` per hour; the writer's `lastSeq` therefore
never leaves zero, and `w.prevLastSeq = w.lastSeq` at rotation makes every
hourly Open declare "no predecessor". Verify's classification is defensible
and the archive is **not** lying — a stream with no sequenced data genuinely
has no predecessor *sequence*, and zero is the documented "no predecessor".
**What is at stake is signal quality, not correctness:** `restart_boundary`
is the evidence that a *restart* happened (HANDOFF §32a's price;
`pipeline_test.go:668` asserts exactly it), and a marker that also fires
once per hour by construction stops distinguishing "a restart happened" from
"an hour passed on a REST source" — and it scales with every REST source
added. **It is ruled by nobody yet, and it blocks nothing:** BTCUSDT is
sealed out of the crossvenue gate (§43b), and the walker's continuity carry
runs on the WS book/funding sources, which sequence correctly (§45p's smoke
measured ~99.7% joined coverage). The tension to rule on is this project's
own two-directions rule pointing both ways — suppressing the finding would
hide a real restart on a source that was quiet the previous hour, while
keeping it spends attention every hour on a non-event.

Capture rituals first, always: **pgrep PID 8495** (alive, restarted
2026-08-01T11:13:10Z), background verify, marker grep (**ZERO**). QA
binaries go to the scratchpad — **never rebuild over `bin/arblab` while
capture runs**; `make ci` writes no `bin/`, so it is safe.

---

## START HERE (updated session 61, 2026-08-01 — HISTORICAL — §45 SLICE C IS CLOSED; superseded by session 64's block above)

**Branch `feat/milestone-1-foundation`.** Session 61 closed §45 slice C
(**read `2026-08-01-strategy2-crossvenue-gate.md` §45m, then its RESUME
HERE**): the aimed battery ran — **31/31 claims caught** (3 predicted
`NOBUILD!!` unused-variable re-aims, all re-aimed and caught; zero
SURVIVED/PANIC/TIMEOUT), after a mirror-pass checklist round found TWO
more fixture collapses BEFORE any mutation ran (the Bybit staleness
at-bound — §45l's fix had landed on Binance only, the ratelimit shape;
and the Binance sampler's file-boundary reset had NO fixture — the Bybit
mirror existed, its counterpart didn't; both new fixtures then killed
their aimed mutants). The write-once stub was patched to fail gracefully
past its script, pre-empting the roster's predicted `PANIC!!`
non-verdict. **The labeled smoke ran end to end** (`s61-smoke-btc`,
params + report committed under `research/crossvenue/sims/`, raw REST
store now properly git-ignored): fetch stored both venues with digests;
the sim walked ONLY the two pre-cutoff days (the seal structural in a
live run), paired 6,958 settlements with zero collisions, scored 9/9
episodes at cost exactly 0.0021 — **all four p90s ZERO at 0.05 BTC,
§41p's calm-stratum zero-slippage reproduced from our own books**. One
measured observation recorded for slice D (§45m): **Bybit decision-grid
coverage is ~9%** — the designed per-file cold start meeting a wire
whose tickers snapshot only at subscribe; the safe recovery seam (framing
records) is written up as a slice-D fork, not absorbed. `make ci` green,
marker grep ZERO.

**What is next, in order: (1) the §32m-D per-tier figure** (due by
2026-08-03T11:13Z, measurable from ~2026-08-02T11:13Z) → §44e + the §38f
off-host re-argument on the measured disk clock; **(2) §45 slice D** —
the procedure commit (params roster incl. the Bybit-coverage ruling, VCS
binding, dataset ledger, write-once artifact + attestation, holdout
declaration naming §43b, the ≥180-matched end rule; the seal lifts only
through it). Capture rituals first, always: pgrep PID 8495, background
verify (session 61: COVERAGE, 0 integrity), marker grep (ZERO).

---

## START HERE (updated session 60, 2026-08-01 — HISTORICAL — §45 SLICE C CODE + BATTERY BUILT; superseded by session 61's close)

**Branch `feat/milestone-1-foundation`.** Session 60 built everything §45
slice C names, checkpointing deliberately before the mutation battery:
the wire fact FIRST (Profiles §5 dated block: Bybit book `u` advances by
EXACTLY 1 per delta — 8.46M deltas, zero exceptions — measured from our
own archive before the accumulator read it), then
`strategy/infrastructure/crossvenue`'s walker stack — manifest gating
with the gate's own guards (`walk.go`), the §43b post-cutoff seal IN CODE
(`seal.go`), the settled REST store (`store.go`, screening imported not
copied), decision-point sampling with the churn-refreshes-anchor rule
(`sample.go`), the Bybit book accumulator on the measured contiguity fact
(`accumulator.go`), the book-cost walkers (`books.go`), the sim assembly
(`sim.go`) — plus `domain/crossvenue.WalkedFill` (the standing depth
walk; CounterfactualSlippage's pair guard rightly refuses a same-instant
pair, so the walk got its own function and oracle) and the `arblab
crossvenue fetch`/`sim` verbs. TWO new oracles green (big.Rat depth walk;
sorted-slice reference book). The checklist-before-battery pass found
THREE fixture collapses before any mutation ran (staleness-at-the-bound,
churn-vs-state reset, p90≡max masking the walked arm). `make ci` GREEN,
marker grep ZERO. **Read
`2026-08-01-strategy2-crossvenue-gate.md` §45l, then its RESUME HERE —
the aimed mutation roster and the BTC exploration smoke are written out
there and are the next two work items, then slice D.** The §32m-D
per-tier figure (due by 2026-08-03T11:13Z, measurable from
~2026-08-02T11:13Z) outranks them if the calendar is tight. Capture
rituals first, always: pgrep PID 8495, background verify, marker grep.

---

## START HERE (updated session 59, 2026-08-01 — HISTORICAL — §45 SLICE B BUILT; superseded by session 60's slice C build)

**Branch `feat/milestone-1-foundation`.** Session 59 built §45's slice B:
**`internal/strategy/domain/crossvenue`** — pairing (the screening's rules
restated domain-grade, exact per-leg instants kept for the phase ledger),
the window-emitting episode machine (`(OpenAt, CloseAt]` — no-lookahead
made structural), the funding-phase ledger (unhedged term = running net
minus its value at the last count-balanced instant, atomically batched, so
aligned grids collapse it to exactly zero), the §8 cost selections
(`WorseOf`/`WorseSide` pure, `RoundTrip` refuses negative cells), and
ruling §45f-3's portfolio statistic (weights sum exactly 1, stationary-
bootstrap LCB on a pinned in-package SplitMix64 stream, FLOOR order
statistic). Four oracles beside them. 29/29 aimed mutations caught; one
SURVIVED diagnosed EQUIVALENT (the same-instant batch loop could never
iterate twice under the duplicate rule — production now states the
invariant as a guarded `if`); one TIMEOUT!! (a mutant froze the merge
loop — a hang names no defect) re-aimed and caught. Read
**`2026-08-01-strategy2-crossvenue-gate.md` §45k**, then its RESUME HERE.

**What is next, in order: (1) the §32m-D per-tier figure when its day has
run (due by 2026-08-03T11:13Z, measurable from ~2026-08-02T11:13Z) → §44e
+ the §38f off-host re-argument; (2) §45 slice C** — the walker's
measurement stages + sim CLI, including the Bybit book ACCUMULATOR (§45d
named it walker machinery; §45g's accumulator/depth-walk suspects still
open) and the post-cutoff seal IN CODE; **(3) §45 slice D** — the
procedure commit pinning every argument slice B left open. Capture rituals
first, always: pgrep PID 8495, background verify (session 59: COVERAGE, 0
integrity, 2570 coverage), marker grep (ZERO).

---

## START HERE (updated session 58, 2026-08-01 — HISTORICAL — §45 SLICE A BUILT + ALL FOUR §45f FORKS RULED; superseded by session 59's slice B build)

**Branch `feat/milestone-1-foundation`.** Session 57 executed the §42c capture expansion: WS
ceilings verified into the profiles first, the shortlist × four streams into
`configs/capture.json` (8 → 40 sources, every shape a BTC-counterpart copy), and the capture
restarted with the honest recorded gap. The capture-expansion design record
(**`2026-08-01-strategy2-capture-expansion.md`, §44** — read it, then its RESUME HERE) holds the
verified ceilings reading (§44a), the config decisions (§44b), and the measured restart (§44c:
COVERAGE/0-integrity pre-verify, ~8s gap, `process_restart` records on all 8 pre-existing
sources, first data on all 40 within two minutes).

Verify rather than trust: `git status --short` (clean), `make ci` (green COLD, all six gates —
it writes no `bin/`, safe beside the capture), `grep -rn "SPEC EDIT OWED" internal/ tools/` —
expect **ZERO**.

**The capture rituals stay first, every session — with the new facts:**
`pgrep -fl "arblab capture run"` (**PID 8495** since 11:13Z; config `1c362feffe4a`, 40 sources),
then a verify over the accumulated range **in the background** (`capture verify` needs `-config`,
`-from` AND `-to`, RFC3339; **exit code 2 IS the COVERAGE verdict**) — 0 integrity findings,
always. A range crossing 2026-08-01T11:13Z honestly shows the restart's gaps; alt-stream history
begins there, so pre-restart windows for alt sources report absent by construction. **Never
rebuild over `bin/arblab` while the capture runs** (the rule survives §44c's rename-survival
observation — luck of the tooling, not a guarantee).

**What is next — §44's RESUME, in order:**
1. **The §32m-D per-tier one-day figure, DUE BY 2026-08-03T11:13Z** (48h from the config
   landing), landing as **§44e**: `du` split BTC-tier vs alt-tier (check against §42c's derived
   ~0.6–1.1G/day/symbol — the two Bybit streams are the unmeasured scale-down), plus the
   full-day verify over the measured day. **This is §38f's named revisit point** (ruling
   §42f-3): re-argue the off-host decision on the measured clock, not the derived one.
2. **The §42e work is UNDER WAY — design record §45**
   (`2026-08-01-strategy2-crossvenue-gate.md`, session 58 — read it, then its RESUME HERE).
   Slice A is BUILT: `venue.DecodeBybitOrderbook` (the wire facts verified into the profiles
   from our own capture bytes first), `strategy/infrastructure/crossvenue` (§45c's policed
   placement — a top-level `strategy/crossvenue` would be invisible to `hexboundary`), and
   `arblab crossvenue check` — **the live smoke ran same-session: all 40 streams decode, 0
   refusals over ~3.7M frames.** 19/19 aimed mutations caught (one SURVIVED diagnosed into
   the wrong-day-fixture gap and fixed). **All four §45f forks are RULED (user, same
   session): end rule = ≥180 post-cutoff matched settlements per symbol; exploration =
   in-repo pre-cutoff only, post-cutoff SEALED IN CODE; pass = pre-declared portfolio
   policy at the LCB; holdout = the clean window IS the sample, §43b a named input.**
   Next, in order: slice B (domain math with oracles), slice C (walker measurement stages
   + sim CLI with the seal), slice D (the procedure commit — gate parameters pinned THERE).
   Read §45, then its RESUME HERE.

Constraints that carry over: H1 (post-2026-02-01 BTC) unpolluted; the §41h seal stands;
maker-entry a standing tactic priced at the gate; no v2 of the v1 cash-and-carry procedure.

---

## START HERE (updated session 56, 2026-08-01 — HISTORICAL — MILESTONE 2b STEP 1 EXECUTED; superseded by session 57's capture expansion)

**Branch `feat/milestone-1-foundation`.** Session 56 built the screening harness, ran it end to end,
and closed the doc set at v0.9.21. The screening design record
(**`2026-08-01-strategy2-screening.md`, §43** — read it, then its RESUME HERE) holds the pinned
parameters (§43b), the measured run and its outputs (§43c–§43e), and the verification story (§43f:
checklist BEFORE mutations found three `<`-vs-`<=` collapses; 9/9 aimed mutations caught; the
battery asserts on REQUESTS — the BTC seal and resume-idempotence — as well as outputs).
`research/screening/{ledger,ranking,shortlist,census}.json` committed;
`research/screening/raw/` git-ignored, re-derivable, SHA-256-pinned in the ledger.

Verify rather than trust: `git status --short` (clean), `make ci` (green COLD, all six gates),
`grep -rn "SPEC EDIT OWED" internal/ tools/` — expect **ZERO**.

**The capture rituals stay first, every session:** `pgrep -fl "arblab capture run"` (PID 74192),
then a verify over the accumulated range **in the background** (`capture verify` needs `-config`,
`-from` AND `-to`, RFC3339; **exit code 2 IS the COVERAGE verdict**) — 0 integrity findings, always.
**Operational trap, paid for once: never rebuild over `bin/arblab` while the capture runs** (macOS
kills a running process whose binary is replaced); the screening binary is `bin/arblab-screen` for
exactly this reason.

**What is next — §42's RESUME, step 2 (the time-sensitive, un-backfillable half):**
1. **The §42c capture expansion:** the §43d shortlist × four streams per symbol (both venues' perp
   funding + perp books; no spot legs — §2.3 is perp-perp; no per-alt diff evidence; BTC's eight
   streams unchanged) into `configs/capture.json`; restart the capture with its honest
   `process_restart` gap record; then the **§32m-D one-day measured disk figure within 48h** —
   which is also §38f's named revisit point (ruling §42f-3, the ~3×-compressed disk clock).
2. **Before the config lands:** the venue per-IP WebSocket connection ceilings are a profiles
   verification item (the v0.9.20 scope note, now due).
3. **Then** the `strategy/crossvenue` read-path, the research simulator, and the gate's procedure
   commit (§42e) — gate parameters pinned THERE, never in screening; the holdout declared at that
   commit, with §43b's selection-contamination statement as a named input.

Constraints that carry over: H1 (post-2026-02-01 BTC) unpolluted; the §41h seal stands; maker-entry
a standing tactic priced at the gate, never a screening assumption; no v2 of the v1 cash-and-carry
procedure.

---

## START HERE (updated session 55, 2026-08-01 — HISTORICAL — STRATEGY #2 SCOPED, §12 RE-SEQUENCED; superseded by session 56's screening execution)

**Branch `feat/milestone-1-foundation`.** Session 55 turned §41r's mandate into the scoping
design record (**`2026-08-01-strategy-2-scoping.md`, §42** — read it, then its RESUME HERE) and
the user ruled all four §42f forks the same session: ~8-symbol shortlist, no broad tier until
the census prices it, §38f re-deferred dated at the ~3×-compressed clock, and the §12 roster
edit signed off — **milestone 2b (strategy-#2 edge proof: screening → capture expansion →
crossvenue read-path → its own pre-registered gate) now precedes the OMS, and milestones 3–6
are hard-gated on that gate.** The next act is §42's RESUME HERE item 1: the screening step,
buildable immediately on backfillable settled-funding history (no capture dependency).
Doc set v0.9.20.

Verify rather than trust: `git status --short` (clean), `make ci` (green COLD, all six gates),
`grep -rn "SPEC EDIT OWED" internal/ tools/` — expect **ZERO**.

**The capture rituals stay first, every session:** `pgrep -fl "arblab capture run"` (PID 74192),
then a verify over the accumulated range **in the background** (`capture verify` needs `-config`,
`-from` AND `-to`, RFC3339; **exit code 2 IS the COVERAGE verdict**) — 0 integrity findings,
always. The capture keeps running; the archive serves strategy #2's BTC-side evidence and the
disk-budget clock keeps ticking.

**What is next — the strategy-#2 scoping design record (a design session, docs-first):**
1. **Open a NEW design record** (`2026-08-xx-strategy-2-scoping.md` or similar) — nothing
   further lands in the calibration record. Its scope, per §41r: (i) core §12's roster
   re-sequencing — strategy #2 was item 7, "on the same foundation", and what that means before
   the OMS exists is the scoping question; (ii) the **alt/cross-venue capture surface** —
   time-sensitive, the BTC-only capture universe is the live gap (Tardis cannot backfill our own
   gap, §8's day-one argument applies to strategy #2's instruments too); (iii) candidate-universe
   selection (which alt pairs, which venues — §2.3 scopes to alts/dislocation windows);
   (iv) **strategy #2's own pre-registered economic gate** under the same kill discipline —
   edge bar from measured spread persistence (§2.3), alt book depth measured not inherited
   (§41r caveat 1), holdout rules of its own.
2. **Constraints that carry over:** H1 (post-2026-02-01 BTC) stays unpolluted; the §41h seal
   stands; maker-entry is a standing tactic to price wherever the fee schedule rewards it; no
   v2 of the v1 cash-and-carry procedure — that story is closed.

---

## START HERE (updated session 53, 2026-08-01 — HISTORICAL — MILESTONE 2 CLOSED BY KILL; superseded by session 54's fork ruling)

**Branch `feat/milestone-1-foundation`.** Session 53 ran the governed act and closed milestone 2
by kill. The sequence, each step dated and recorded: (1) governed derivation from a clean
checkout of `95c0819` — **refused, `ErrNoExcursions`, byte-identical across two stamped
invocations** (the anti-invention rule at full stakes); (2) the user's first ruling: diagnose
before deciding — the **§41p sensitivity sweep** (clearing capital-scale-invariant, hold-length
dominated) and its **deployment addendum** (best admissible corner: 98% deployment, 30-day min
hold → 11.5% utilization, 4 entries/3yr); (3) the user's second ruling, **§41q: KILL NOW, BY
RULING** — milestone 2 closes with no artifact, no attestation, step 5 unbuilt; (4) the close —
§8's latency clause landed (marker → `[SPEC EDIT LANDED — v0.9.18]`, grep ZERO), §12's dated
outcome sentence, the ADR verdict block, all five doc stamps at v0.9.18, tutorial.ka
re-assembled. **§41p/§41q in the calibration design record are the session's records; that
record's `RESUME HERE` is the live statement — read it first.**

Verify rather than trust: `git status --short` (clean), `make ci` (green COLD, all six gates),
`grep -rn "SPEC EDIT OWED" internal/ tools/` — expect **ZERO**.

**The capture rituals stay first, every session:** `pgrep -fl "arblab capture run"` (PID 74192),
then a verify over the accumulated range **in the background** (`capture verify` needs `-config`,
`-from` AND `-to`, RFC3339; **exit code 2 IS the COVERAGE verdict**) — 0 integrity findings,
always; session 53's check: COVERAGE-only, the known Bybit-spot absent-window ledger. The
capture keeps running through the fork decision — the archive serves any fork.

**What is next — ONE thing, and it is the user's, not a build task:**
1. **The §12 fork choice.** Core §12 milestone 2 pre-registered the failure forks: (a) the
   maker-entry tactic (≈−40% round-trip cost — attacks the ≈39bp A-term), (b) regime-selective
   deployment (concedes utilization, prices the concession), (c) the alt/dislocation niche
   (strategy #2 early — changes the funding distribution itself), (d) sub-hurdle returns as
   tuition (re-prices the hurdle). §41q maps each onto §41p's measured decomposition — put the
   choice to the user FIRST, starting from that table. Do not author a v2 procedure, do not
   touch the H1 holdout era, and do not build step 5 unless the chosen fork calls for it.

---

## START HERE (updated end of session 52, 2026-08-01 — HISTORICAL — STEP 4 SLICE C HALF-BUILT; superseded by session 53's kill close)

**Branch `feat/milestone-1-foundation`.** Session 52 built slice C's machinery in four commits
(`c4bcd82`…the battery commit): **the §41h seal re-verified standing** (slice C's mandated first
act — seal commit predates every dataset fetch, no holdout-era file exists), the **Binance
partial-depth wire decoders** (spot and futures dialects, each refusing the other as foreign —
plus the session's headline finding: encoding/json's case-INSENSITIVE tag fallback meant the
futures u-vs-U position claim was held by the venue's key order, not the type; found by the
battery's one survivor, fixed with an exact-match `U` field), the **family-2 domain math**
(counterfactual slippage per §41b-ii-i, adverse fill move, upper-quantile selection, the
two-family merge under ShiftAt's outward convention — big.Rat oracles with inline controls), the
**family-2 collectors** (H2 universe = day manifests strictly before the freeze day, per-file
vouching, the committed pairing rule with gap/slack/depth-short all counted), **`procedure.go` —
the §41e parameter roster, every derivation constant chosen with in-file rationale** (latency
bound 250ms = healthy p99 ×2.5; classifier cut points; MinEvidence 500; frame clamps ±0.003
measured from OUR fundingInfo overlay), and the real **`Calibrator.Run`** (§41b's order, §41e
binding checked before and recorded after, write-once, emitted bytes re-loaded through the
production door, two invariants CHECKED: drift-tail family pinning and clearing-level band
independence). **§41o in the calibration design record is the mid-slice build record; that
record's `RESUME HERE` is the live statement — read both before anything.**

Verify rather than trust: `git status --short` (clean), `make ci` (green COLD, all six gates),
`grep -rn "SPEC EDIT OWED" internal/ tools/` — expect **EXACTLY ONE** (§8's latency marker at
`strategy/infrastructure/driven/cost.go`; it stays until the slice C close lands the §8 clause).

**The capture rituals stay first, every session:** `pgrep -fl "arblab capture run"` (PID 74192),
then a verify over the accumulated range **in the background** (`capture verify` needs `-config`,
`-from` AND `-to`, RFC3339; **exit code 2 IS the COVERAGE verdict**) — 0 integrity findings,
always; session 52's check: COVERAGE-only, the known Bybit-spot absent-window ledger.

**Verification shape this session:** checklist BEFORE battery found **five collapses** in the
slice's own new fixtures (17th–21st instances) plus the rounding-tie gap — the tie fixture became
the SOLE killer of its mutant. **23 aimed mutations: 22 first-aim kills, 1 SURVIVED diagnosed
into the real u-vs-U routing defect** (fix + key-order fixture then killed both tag mutants),
2 NO-OP re-aims caught. `exhaustiveswitch` and `nofloat` both fired live on first drafts.

**What is next, in order (the RESUME lists it):**
1. **The governed derivation run** (§41e): clean checkout, VCS-stamped build, `arblab calibrate
   run -freeze <RFC3339>` twice, byte-identical, artifact committed write-once. Refusals
   (`ErrNoExcursions`, zero occupancy, `ErrFrameUnpriced`) are governance questions for the
   user — never numbers to invent.
2. **The 24-hour delay → attestation → close** (§8 latency clause landed, marker to ZERO,
   doc-set bump to v0.9.18, §41o part 2).
3. Then **step 5**: the scored replay runner under the sealed identity.

---

## START HERE (updated end of session 51, 2026-08-01 — HISTORICAL — STEP 4 SLICE B IS BUILT; superseded by session 52's slice C machinery build)

**Branch `feat/milestone-1-foundation`.** Session 51 built §41j's slice B in five commits
(`d9c41ef`…`51f6adf` + the close commit): **`strategy/domain/calibrate`** (the family-1
derivation math — causal EWMA residuals with saturation warmup, outward-from-median quantile
points, the EntryDrift tail rule via the exported `cashcarry.Annualized`, the excursion band,
the relative adverse basis move, the `Surrogate` constant-cost frame, `ClearingLevel` by exact
bisection against the DEPLOYED `Enter`, and the `Occupancy` replay driving the real
`Enter`/`Continue` over history), the **family-1 fetchers** (verbatim venue pages, grid
normalization with an observed-jitter tolerance, H1 refused before one byte moves, write-once
digested files), **`arblab calibrate fetch`** wired, and **four committed datasets** under
`datasets/` with digests/counts/eras PINNED in `committed_test.go`. One slice-A schema defect
fixed at its first consumer: `adverse_basis_move` is now a RATE (the runtime adapter charges
rate × notional; a relative move is what transfers across an era where BTC tripled). **§41n in
the calibration design record is the build record; that record's `RESUME HERE` is the live
statement — read both before anything.**

Verify rather than trust: `git status --short` (clean), `make ci` (green COLD, all six gates),
`grep -rn "SPEC EDIT OWED" internal/ tools/` — expect **EXACTLY ONE** (§8's latency marker at
`strategy/infrastructure/driven/cost.go`; it stays until slice C's close lands the §8 clause).

**The capture rituals stay first, every session:** `pgrep -fl "arblab capture run"` (PID 74192),
then a verify over the accumulated range **in the background** (`capture verify` needs
`-config`, `-from` AND `-to`, RFC3339) — 0 integrity findings, always; session 51's check:
COVERAGE-only, the known Bybit-spot absent-window ledger.

**Verification shape this slice:** 25 aimed mutations — 21 first-aim kills, 3 NOBUILD re-aims
and 1 NO-OP re-aim (all caught on re-aim), **ZERO survived**. Checklist BEFORE battery found
**three collapses in this slice's own new fixtures** (14th–16th instances: every q=0.5 rank
exact; every jitter fixture drifting late; the dwell-reset arm only ever run at count zero) —
and each checklist fixture was the decisive killer for its aimed mutant. The `exhaustiveswitch`
gate fired live on the first draft's two verdict switches. H3's completeness refusal fired live
on real data (Binance's 2023-03-24 spot-halt kline hole), which is why the kline era starts
2023-04-01.

**What is next, in order (the RESUME lists it):**
1. **Step 4 slice C** (§41j): seal verification first, then family-2 measurements over the
   archive, family-3 declarations, the derivation run from a CLEAN checkout (§41e binding,
   §41b's derivation order, the two-invocation determinism claim), the 24h delay, attestation,
   close (lands §8's latency clause, marker to ZERO, doc-set bump). The frame's committed
   constants and every derivation quantile are procedure parameters chosen AT the §41e commit.
2. **Step 5**: the scored replay runner under the sealed identity.

---

## START HERE (updated end of session 50, 2026-08-01 — HISTORICAL — STEP 4 SLICE A IS BUILT; superseded by session 51's slice B build)

**Branch `feat/milestone-1-foundation`.** Session 50 built §41j's slice A in four commits
(`5281dfa`…`4642879`): the frozen stratum classifier (`strategy/domain/stratum.go`), the
stratified per-leg `CostCell` widening with `MinEvidence` and the classifier-keyed lookup, the
artifact schema + loader (`driven/calibration.go` — hash over exact bytes, §41b's family
pinning ENFORCED), the detached attestation + binding check (`driven/attestation.go`, 24h delay
inclusive at the boundary), the `strategy/infrastructure/driving` refusal frame (H1/H2/H3, the
§41h seal guard, the §41e execution binding, `Calibrator.Run` refusing `ErrDerivationUnbuilt`),
and the `datasets/` layout. **§41m in the calibration design record is the build record; that
record's `RESUME HERE` is the live statement — read both before anything.**

Verify rather than trust: `git status --short` (clean), `make ci` (green cold, all six gates),
`grep -rn "SPEC EDIT OWED" internal/ tools/` — expect **EXACTLY ONE** (§8's latency marker at
`strategy/infrastructure/driven/cost.go`, taken deliberately this slice; it stays until slice
C's close lands the §8 clause).

**The capture rituals stay first, every session:** `pgrep -fl "arblab capture run"` (PID 74192),
then a verify over the accumulated range **in the background** (`capture verify` needs
`-config`, `-from` AND `-to`, RFC3339) — 0 integrity findings, always; session 50's check:
COVERAGE-only, the known Bybit-spot absent-window ledger.

**Verification shape this slice:** 19 aimed mutations — 18 first-aim kills, one NOBUILD
re-aimed, one SURVIVED **diagnosed into a real gap** (CheckComplete's positional extra-row
claim was masked by the count backstop; the count-preserving substitution fixture and its
late-shifted MIRROR both landed). One live fixture collapse caught mid-build: an
uppercase-digest fixture seeded `0x14` hexes to digits only, so `ToUpper` was the identity —
the hex-case lesson's eleventh instance, now in the §41m ledger.

**What is next, in order (the RESUME lists it):**
1. **Step 4 slice B** (§41j): family-1 fetchers (calibration era only, H1-guarded,
   H3-complete) + derivations (EWMA residuals, band, basis, occupancy-with-surrogate,
   `EntryDrift`) + `math/big` oracles; wire `calibrate` into `cmd/arblab` when Run stops
   refusing. §41i's slice-B suspects are pre-named in the RESUME.
2. **Slice C**: seal verification, family-2/3 freeze, clean-checkout derivation, 24h delay,
   attestation, close (lands §8's latency clause, marker back to ZERO, doc-set bump).
3. **Step 5**: the scored replay runner under the sealed identity.

---

## START HERE (updated end of session 49, 2026-07-31 — HISTORICAL — STEP 4 DESIGNED, docs-only; superseded by session 50's slice A build)

**Branch `feat/milestone-1-foundation`.** Session 49 wrote **no code** — it opened milestone 2's
**step 4** (§31a's row: the pre-registered calibration artifact) and produced its design record.
**The single source of truth is
`docs/superpowers/specs/2026-07-31-calibration-artifact-design.md`** — §41 is the design, its
`RESUME HERE` is the live statement. It survived **three complete adversarial review rounds**
(Codex; each initially NO-SHIP, each digging one level deeper — structure → observation
definitions/execution binding → contract pinning; 17 findings total, all valid, all applied; the
loop then deliberately closed — the provenance note at its top records the full ledger). Headlines:
the frozen quantities split into **three provenance families** (free REST history / our captured
books / declared-with-rationale — the dispersion measured in two parts, merged pointwise-adverse);
the holdout protocol is **three mechanical rules** (H1 boundary 2026-02-01, H2 freeze-instant, H3
closed identities + everything-eligible); approval is a **committed detached attestation** the
step-5 runner consumes (write-once, its own hash in run evidence); the derivation procedure is
**execution-bound** (dirty-tree refusal, embedded revision, executable digest); placement is
domain math + a `driving` adapter (rule 3's shape); **ONE pre-registered core marker** (§8's
latency clause, the §40d modeled-tier shape) lands at slice C's close. **Two user decisions were
taken and SEALED in-session (§41h): the step-5 fork is HYBRID and the gate venue is BINANCE —
dated 2026-07-31, before any holdout fetch; do not reopen.** §38f stays deferred by the user's
same-session choice (the disk clock still runs).

Verify rather than trust: `git status --short` (clean), `make ci` (green, all six gates),
`grep -rn "SPEC EDIT OWED" internal/ tools/` — expect **ZERO** (the design pre-registers ONE
marker that slice A will deliberately take; until slice A lands, zero stays the expectation).

**The capture rituals stay first, every session:** `pgrep -fl "arblab capture run"` (PID 74192),
then a verify over the accumulated range **in the background** — 0 integrity findings, always
(session 49's check: COVERAGE-only, the known Bybit-spot ledger).

**What is next, in order (the RESUME lists it):**
1. **Step 4 slice A** (§41j): the artifact schema (stratified per-leg p90 + frozen classifier
   params + `EntryDrift`) + loader + attestation contract + H1/H2/H3 refusal frame +
   `datasets/` layout + the §8 latency marker taken.
2. **Slices B and C** (§41j), then **step 5** with the sealed identity.

---

## START HERE (updated end of session 48, 2026-07-31 — HISTORICAL — STEP 3 CLOSED at v0.9.17; superseded by session 49's step-4 design)

**Branch `feat/milestone-1-foundation`.** Session 48 built §40j's slice C and closed step 3: the
`strategy/domain/cashcarry` core (entry inequality, incremental exit, quantization, plan
regeneration), the write-ahead `Decide` door, the §3.9 pinned-ledger test, and the close that
landed §40d's §2.4 edit and bumped the doc set to **v0.9.17**. **§40n in the strategy design
record is the build record; that record's `RESUME HERE` is the live statement — read both before
anything.**

Verify rather than trust: `git status --short` (clean), `make ci` (green, all six gates),
`grep -rn "SPEC EDIT OWED" internal/ tools/` — expect **ZERO**; the newest marker reads
`[SPEC EDIT LANDED — v0.9.17]` at `strategy/application/ports/ports.go`.

**The capture rituals stay first, every session:** `pgrep -fl "arblab capture run"` (PID 74192),
then a verify over the accumulated range — 0 integrity findings, always; coverage gaps are
information, not failures. The verify now takes minutes over the multi-gigabyte archive: run it
in the background while reading the records.

**What is next, in order (the RESUME lists it):**
1. **The off-host destination** — the user's open §38f decision; the laptop still holds the ONLY
   copy, and §32m-D's 3.0G/day figure keeps a weeks-not-months clock on it.
2. **§31a step 4: the calibration artifact** — the §8 pre-registered thresholds record every
   class-2 measured quantity refuses to construct without (dispersion, slippage bound, exit band,
   adverse basis move, rescue margin). Design session first, in the record's style.
3. **§31a step 5: the scored replay runner** — the capture-container `DecisionSink` adapter
   (§40h), the `SettlementHistory` accumulator's first instantiation (§40c), the
   reference-revision repository (§40k-2), replay-driven paper decisions (§40i), and §39l-1's
   window decision.

---

## START HERE (updated end of session 47, 2026-07-31 — HISTORICAL — SLICES A+B BUILT, SLICE D FIGURE TAKEN; superseded by session 48's slice C close)

**Branch `feat/milestone-1-foundation`.** Session 47 took slice D's one-day disk figure (3.0G/day,
0 integrity / 353 coverage — §32m-D in the capture design record) and built §40j's slice B in
four commits: `Rate.Div`, the `strategy/domain` model layer (dispersion + time-decay EWMA +
estimator + joint rule + oracle), and the two driven model adapters (conservative basis, §8
conservative cost bound). **§40m in the design record is the build record; that record's
`RESUME HERE` is the live statement — read both before anything.**

Verify rather than trust: `git status --short` (clean), `make ci` (green, all six gates),
`grep -rn "SPEC EDIT OWED" internal/ tools/` — expect **EXACTLY ONE** (§40d's, at
`strategy/application/ports/ports.go`, until slice C's close lands the §2.4 edit). The `Decide`
door still refuses with `ErrCoreUnbuilt` — slice C is what unbuilds that refusal.

**The capture rituals stay first, every session:** `pgrep -fl "arblab capture run"` (PID 74192),
then a verify over the accumulated range — 0 integrity findings, always. Coverage gaps are
information, not failures; the morning of 31 Jul carries reconnect churn (laptop lid), recorded
honestly.

**What is next, in order (the RESUME lists it):**
1. **The off-host destination** — the user's open §38f decision; the laptop still holds the ONLY
   copy, and §32m-D's arithmetic (weeks of disk, not months) now puts a clock on it.
2. **Step 3 slice C** (§40j): the cashcarry core — entry inequality, exit incremental score with
   the fed-back dwell, `HoldPlanRevision` regeneration (interval-flip fixtures BOTH directions),
   quantization, write-ahead `Decide`, the tutorial §3.9 pinned-ledger test, refusal Decisions.
   Its close lands the §2.4 edit, runs the reading pass (§40m lists slice B's stated readings for
   the docket), and bumps the doc set.

---

## START HERE (updated end of session 46, 2026-07-31 — HISTORICAL — STEP 3 SLICE A IS BUILT; superseded by session 47's slice B build)

**Branch `feat/milestone-1-foundation`.** Session 46 built §40j's slice A in six commits — the
`internal/strategy/` context skeleton (domain roster + wire DTO + application with the seven ports
and the `Decide` door + the reference revision store), the Profiles verification pass with the
committed milestone-2 revision (`configs/reference.btcusdt.json`), and the `engine/state`
`StrategyReader` (the deferral trigger fired with the port). **§40l in the design record is the
build record; that record's `RESUME HERE` is the live statement — read both before anything.**

Verify rather than trust: `git status --short` (clean), `make ci` (green, all six gates),
`grep -rn "SPEC EDIT OWED" internal/ tools/` — expect **EXACTLY ONE** (§40d's two-clause §2.4
marker at `strategy/application/ports/ports.go`, taken deliberately; it stays until slice C's
close lands the edit). The `Decide` door refuses with `ErrCoreUnbuilt` until slice C — that is the
skeleton being honest, not a bug.

**The capture rituals stay first, every session:** `pgrep -fl "arblab capture run"` (PID 74192),
then a verify over the accumulated range — 0 integrity findings, always. Session 46's check found
the **Bybit spot book absent ~23:00–04:15 UTC (30→31 Jul)**, recorded honestly as coverage
findings (reconnect gaps + absent windows), self-recovered before session start. Coverage gaps are
information, not failures — but the slice D disk-figure row should name this one.

**What is next, in order (the RESUME lists it):**
1. ~~Slice D's one-day disk figure~~ — **TAKEN, session 47 (2026-07-31): 3.0G/day, 0 integrity /
   353 coverage over the full day, recorded as §32m-D in the capture design record.** Funding
   alone ~114M/day; books are the cost; the figure under-measures a healthy day (the honest
   23:00–04:15 absent window is inside it).
2. **The off-host destination** — still the user's open §38f decision; the laptop still holds the
   only copy, and §32m-D's arithmetic (weeks of disk, not months) now puts a clock on it.
3. **Step 3 slice B** (§40j): the default estimators + Dispersion/quantile machinery + the
   `math/big` oracle. §40j names slice B's checklist suspects in advance — an EWMA window pinned
   to one settlement, a clamp that never binds, a symmetric quantile fixture, a zero-width
   dispersion — read them before writing fixtures.

---

## START HERE (updated end of session 45, 2026-07-31 — HISTORICAL — §31a STEP 3 DESIGNED, docs-only; superseded by session 46's slice A build)

**Branch `feat/milestone-1-foundation`.** Session 45 wrote **no code** — it opened milestone 2's
**step 3** (§31a's roster: the `strategy` context + `cashcarry` + the fee seam) and produced its
design record. Verify rather than trust: `git status --short` (clean),
`grep -rn "SPEC EDIT OWED" internal/ tools/` — expect **ZERO** (the design pre-registers ONE marker
that slice A will deliberately take; until slice A lands, zero stays the expectation). `make ci`
green, docs 102 pass.

**The single source of truth is
`docs/superpowers/specs/2026-07-31-strategy-cashcarry-fee-seam-design.md`** — §40 is the design,
its `RESUME HERE` is the live statement. It survived **four** adversarial review rounds (three
complete, each initially NO-SHIP, each accepting the prior round's repairs and digging one level
deeper; round 4 killed mid-run with its two formed findings recovered from its log and applied;
the loop then deliberately closed — the provenance note at its top records the full ledger and
the closing reason). Headlines: `cashcarry`
is a **pure domain core over one validated `DecisionInput`** (policy projection, §2.4's two Δ
terms, lot rules, candidate quantity and position facts included), **seven ports** at application
incl. the split read seam (`MarketStateReader` + `SettlementHistory` — one deterministic
accumulator over the DISPATCHED funding stream, same component live and replay, §40c), the
`ExecutionCostModel` slippage seam (§40e), and the consolidated `ReferenceData` port (fees, lot
rules, hurdle — three as-of faces of one dated-revision mechanism, schema types context-owned,
kernel machinery only, §40b/§40j); the **fee seam** is dated provenance-carrying
**revisions** read as-of through that port — fetch-writes-a-revision,
decision-path-reads-revisions, with a two-clause §2.4 edit owed (composition rule + the
modeled-tier VIP0 authorization for milestone 2, account-real fees before capital) (§40d); numbers
split into **spec-defaulted params vs artifact-identified measured quantities** (exit-band width
and the basis adverse move included) — the latter refuse to construct without step 4's artifact
hash, one artifact identity per scored run (§40e); the production door is **`Decide → (Decision,
[]Intent, error)`** — the errorless §3.3 adapter is a compile-check ONLY, never a path — with the
`Decision` crossing via a **`DecisionSink` port, write-ahead AND fsync-on-write**, journal-complete
to the recompute-from-the-record-alone property with a versioned wire DTO (`fixedpoint` has no
JSON face — §40f/§40h); the **`HoldPlan` is durable evidence with journaled revisions**
regenerated on interval flips, dwell counted on realized settlements (§40f); **§39l-2 and §39l-3
are ANSWERED** (§40h: capture-container Decision journal; §40i: sim pump only, replay-driven
paper) — do not reopen; two debts pre-registered with their constructs: the reference-revision
repository (**step 5**, §40k-2) and the foundation loop's error-bearing door (**milestone 3**,
§40k-5).

**The capture rituals stay first, every session:** `pgrep -fl "arblab capture run"` (PID 74192 as
of session 45), then a verify over the accumulated range — 0 integrity findings, always (session
45's check: COVERAGE-only over 09:00–21:00, the known reconnect/restart ledger plus the open hour).

**What is next, in order (the RESUME lists it):**
1. **Slice D's one-day disk figure, ~09:53 UTC 2026-07-31** — `du -sh archive/` + a full-day
   verify, recorded in the capture design record (§32m's row).
2. **The off-host destination** — still the user's open §38f decision; the laptop still holds the
   only copy.
3. **Step 3 slice A** (§40j): context skeleton, `DecisionInput`/`Decision` + the wire DTO with
   its round-trip battery, all seven ports, the Profiles fee/lot verification pass, the
   context-owned revision schema with `asOf` + provenance refusal, the §40d `[SPEC EDIT OWED]`
   marker.

---

## START HERE (updated end of session 44, 2026-07-30 — HISTORICAL — v0.9.16 RELEASED, SLICE C CLOSED, §31a STEP 2 COMPLETE; superseded by session 45's step 3 design)

**Branch `feat/milestone-1-foundation`.** Verify rather than trust: `git status --short` (clean),
`grep -rn "SPEC EDIT OWED" internal/ tools/` — expect **ZERO**; the newest marker reads
`[SPEC EDIT LANDED — v0.9.16]` in `internal/platform/events/envelope.go`. `make ci` green, docs 102
pass.

**The single source of truth is
`docs/superpowers/specs/2026-07-30-parity-dispatcher-design.md` → §39q (the close) and its
`RESUME HERE`.** Step 2's three slices (A core, B log, C wiring) are all closed; the §39m-5 absence
door is LANDED in core §3.5; the two §39q-3 equivalence rulings (the rank owns timer-ties; the
supersede is milestone-3's) are settled — do not reopen them.

**The capture rituals stay first, every session:** `pgrep -fl "arblab capture run"` (PID 74192 as of
session 44), then a verify over the accumulated range — 0 integrity findings, always (session 44's
check: COVERAGE-only over 09:00–20:00, the known reconnect/restart ledger plus the open hour).

**What is next, in order (the RESUME lists it):**
1. **Slice D's one-day disk figure, ~09:53 UTC 2026-07-31** — `du -sh archive/` + a full-day verify,
   recorded in the capture design record (§32m's row).
2. **The off-host destination** — still the user's open §38f decision; the laptop still holds the
   only copy.
3. **§31a step 3: the `strategy` context + `cashcarry` + the fee seam** — design session first, in
   the record's style; it inherits §39l's three open questions and §39q-5's note (the sim's ordinary
   predicate is defensive until the live transport exists; the live pump is still uncalled).

---

## START HERE (updated MID-SESSION 43, 2026-07-30 — HISTORICAL — SLICE C BUILT and probe passing; superseded by session 44's close)

**Branch `feat/milestone-1-foundation`.** Session 43 built §39j's **slice C** — the wiring — and
**the parity probe passes on real production data**: two sim runs over the committed fixture
produce byte-identical dispatch logs and equal terminal cache states, and the directed rerun
following that log rewrites a byte-identical log to the same terminal state. Everything is
committed (`7338342`…`4dd2258` + the docs commit), `make ci` green, all six gates clean.

**The single source of truth is
`docs/superpowers/specs/2026-07-30-parity-dispatcher-design.md` → §39p (the slice design, with
rejected alternatives) and its `RESUME HERE` (the mid-slice seam)** — read both before touching
anything. Headlines: the funding path only, both venues (§39p-1); wire decoders in `engine/venue`
(Bybit delta accumulator, Binance markPrice+fundingInfo overlay join); the ingestion seam and both
pumps in `engine/replay`; the root, router and probe in `app/sim` over a committed 532 KB fixture
cut at the 15:30:40Z restart seam. **The §39m-5 absence door is OPEN**: `Envelope.Validate` no
longer judges `AcceptOrdinal` (zero is §3.5's absence-derived reading; producer-side guards are
load-bearing), marked `[SPEC EDIT OWED]` — **the marker grep returns ONE by design until the close
lands the §3.5 edit.** A live instruments-info measurement (Bybit floor symmetry, 2026-07-30) is
owed to `venue-profiles.md` at close.

**What remains before the slice closes** (the RESUME lists it in order): the replay-package unit
battery + the crafted contention fixture, the fixture-collapse checklist BEFORE mutations, the
aimed battery (§39p-10's roster), then the close — land both doc edits, marker grep to ZERO,
reading pass, §39q, HANDOFF, version bump to v0.9.16.

**The capture rituals stay first, every session:** `pgrep -fl "arblab capture run"` (PID 74192 as
of session 43), then a verify over the accumulated range — 0 integrity findings, always (session
43's check: COVERAGE-only over 09:00–19:00, the known reconnect/restart ledger plus the open
hour). Calendar items unchanged: **slice D's disk figure ~09:53 UTC 2026-07-31** (`du -sh
archive/` + full-day verify); the off-host destination is still the user's open §38f decision and
the laptop is still the only copy.

---

## START HERE (updated end of session 42, 2026-07-30 — HISTORICAL — DISPATCHER SLICE B BUILT and CLOSED; superseded by session 43's slice C build)

**Branch `feat/milestone-1-foundation`.** Session 42 built §39j's **slice B**: the persisted
dispatch log — `capture.WalkRecords` (the torn-tail scanner's one public door, `scanArchive`
rebuilt on it), and in `engine/dispatch`: the sequence-record schema (JSON payloads,
`k`-discriminated, no version integer — the token IS the version surface), `LogWriter` (a `Sink`
write-ahead of the sink it wraps, density asserted, fresh files only, a Close seal whose ABSENCE is
the crash signal), and `ReplayLog` (torn tail recovered + reported, everything else refused with
capture's truncated-versus-wrong split). Container is §32e's frame VERBATIM — `Data` entries,
`Close` seal, `CaptureSeq` IS the `IngressSeq`, `Mono` the constant 0 — new `RecordKind`s rejected
because a `FormatMinor` bump would orphan the live v1.0 production archive (§39n-1). Verify rather
than trust: `git status --short` (clean), marker grep **ZERO**, `make ci` green on a cold cache.

**Read `docs/superpowers/specs/2026-07-30-parity-dispatcher-design.md` → §39n** (the design, with
rejected alternatives) **and §39o** (the close) **and its `RESUME HERE`**. Headlines: the
§39m-6/§39k-1 watch item closed **without a core edit** — §3.5 already entails timer positions in
the log by composition, and the derivation plus the refuted counter-reading are §39o-1; do not
re-litigate it into an edit. The checklist changed fixtures twice BEFORE the battery (a timer at a
mid-sequence position; the seq-0 boundary), `exhaustiveswitch` caught the reader's kind switch
naming 2 of 8 members on the first `make ci` (a live rule-10 positive control), then **37/37 aimed
mutations caught** (one honest `NOBUILD!!` re-aim). The Mono gap was found while WRITING the close
— nothing decoded Mono, so byte-identity alone could not see a deterministic-nonzero mutant; the
frame-by-frame `Mono == 0` walk now exists (§39o-3).

**Next: slice C — the wiring and the parity probe** (§39j): replayed archive → ingestion
(`AcceptOrdinal` at the seam) → `marketdata` → mailboxes → dispatcher+`LogWriter` →
`engine/state`, under `TestClock`; done when two sim runs over one archive produce byte-identical
dispatch logs and the replayed log re-drives `engine/state` to the same terminal state. Fixture is
a small COMMITTED archive; the production tree is a manual soak.

**The capture rituals stay first, every session:** `pgrep -fl "arblab capture run"`, then a verify
over the accumulated range — 0 integrity findings, always (session 42's check: COVERAGE-only over
09:00–18:00, the known reconnect/restart ledger plus the open hour). Calendar items unchanged:
**slice D's disk figure ~09:53 UTC 2026-07-31** (`du -sh archive/` + full-day verify); the
off-host destination is still the user's open §38f decision and the laptop is still the only copy.

---

## START HERE (updated end of session 41, 2026-07-30 — HISTORICAL — DISPATCHER SLICE A BUILT and CLOSED; superseded by session 42's slice B close)

**Branch `feat/milestone-1-foundation`.** Session 41 built §39j's **slice A**: `internal/engine/dispatch`
exists — the §3.5 total-order dispatcher core (Step, IngressSeq stamped only at dispatch, timer pump,
Prio-constancy assertion) plus `MarketDataLane.Peek`, the kernel read-half the arbitration needed.
Verify rather than trust: `git status --short` (clean), marker grep **ZERO**, `make ci` green.

**Read `docs/superpowers/specs/2026-07-30-parity-dispatcher-design.md` → §39m** (the slice-A close)
**and its `RESUME HERE`**. Headlines: the arbitration is a RANK — due timers, then control FIFO, then
market by most-urgent head Prio with ties rotating over a field-wise-sorted roster (never map order;
the sim pump must be deterministic) — and the rank shape was FORCED by the pop-only primitives, not
preferred (§39m-1). Refusals burn no sequence; sink errors spend it (§39m-4). The absence-derived
door (`AcceptOrdinal == 0`) is deferred to its construct: §3.4's silence re-judger (§39m-5). The
checklist found two fixture gaps BEFORE the battery (comparator-beyond-symbol — the realistic
accident is the actual capture set — and the control-FIFO mirror); then **24/24 aimed mutations
caught**, with the honest non-aims named in §39m-8 (two equivalent mutants in `cmpSource`, one
unreachable defensive branch, invariant 6 having no mutable line because it is compile-shaped).

**Next: slice B — the dispatch log** (capture-lane framing over `engine/capture`'s codec, sequences
not payloads, §39e). Its first design question is §39m-6's watch item: whether the sequence-record
field set (timer positions included) needs a normative sentence in core §3.5.

**The capture rituals stay first, every session:** `pgrep -fl "arblab capture run"`, then a verify
over the accumulated range — 0 integrity findings, always (session 41's check: COVERAGE-only over
09:00–18:00). Calendar items unchanged: **slice D's disk figure ~09:53 UTC 2026-07-31**; the
off-host destination is still the user's open §38f decision and the laptop is still the only copy.

---

## START HERE (updated end of session 40, 2026-07-30 — HISTORICAL — the PARITY DISPATCHER is DESIGNED, docs-only; superseded by session 41's slice A build)

**Branch `feat/milestone-1-foundation`.** Session 40 wrote **no code** — it opened milestone 2's
**step 2** (§31a's roster: the parity dispatcher + persisted `DispatchSeq`, the deferral §31f says
"stops being deferrable at this milestone") and produced its design record. Verify rather than
trust: `git status --short` (clean), `grep -rn "SPEC EDIT OWED" internal/ tools/` — expect **ZERO**.

**Read `docs/superpowers/specs/2026-07-30-parity-dispatcher-design.md`** — §39 is the design, its
`RESUME HERE` is the live statement. Headlines: the dispatcher is **`engine/dispatch`** (§39c); the
persisted log is **capture-lane framing over `engine/capture`'s codec, sequences not payloads**
(§39e); **the outbox store half does NOT come with it** — §31f's sentence saying so contradicted
milestone 1's own item 1b and core §3.5, corrected on the record (§39d); `engine/state`'s
equal-`RecvTime` accommodation is ratified and its check stays (§39h); build order is three slices
(§39j), core → log → wiring, closing on the parity probe: **two sim runs over one archive produce
byte-identical dispatch logs**. §39a records why session 39's "calibration artifact first" sketch
was a compression of steps 4–5, not a reordering — §31a stands.

**The capture rituals stay first, every session:** `pgrep -fl "arblab capture run"`, then a verify
over the accumulated range — 0 integrity findings, always (session 40's check: COVERAGE-only, with
hour 13–15's reconnect/restart gaps the honest ledger of a laptop afternoon). Calendar items
unchanged: **slice D's disk figure ~09:53 UTC 2026-07-31**; the off-host destination is still the
user's open §38f decision and the laptop is still the only copy.

---

## START HERE (updated end of session 39, 2026-07-30 — HISTORICAL — v0.9.15 RELEASED, slice F CLOSED, capture restarted onto the MANIFEST-WRITING build)

**Branch `feat/milestone-1-foundation`.** Verify rather than trust: `git status --short` (clean),
`grep -rn "SPEC EDIT OWED" internal/ tools/` — expect **ZERO** (slice F wrote no markers; its one
edit landed in-record, §38h). `make ci` green, docs 102 pass.

**A capture process is (or should be) RUNNING on the v0.9.15 build** — restarted end of session 39
at a recorded seam (same as v0.9.13→14), eight streams, and from this build on **every clean close
lands a manifest entry**. Day 2026-07-30's pre-manifest hours were backfilled
(`capture manifest -config configs/capture.json -day 2026-07-30`, labeled entries). It still does
NOT survive a reboot. Check it before anything else:

```
pgrep -fl "arblab capture run"      # alive?
bin/arblab capture verify -config configs/capture.json -from 2026-07-30T09:00:00Z -to <next hour boundary, RFC3339>
```

Expect **0 integrity findings, always** — and from now on that sentence covers the manifest layer
too: `manifest_missing_file` on a copy or on this root is a file that EXISTED and is GONE. Coverage
findings for the open hour are the honest description of an hour not yet closed. The restart seam
itself is recorded evidence (`process_restart` gap; the restart hour's file is closed by the NEW
build with recovered counters — the exact path `TestReopenedFileClosesWithFullFileCounts` pins).
**"Always" includes the hour a restart reopened, and a verify that races the rotation (§38i +
postscript):** production's first hour delivered both siblings — a continued hour read as
`digest_mismatch`, then eleven phantom `manifest_mismatch` findings from scanning mid-append while
the rotation's fresh entries landed (rotation fires on venue `RecvTime`, ~35s ahead of this
laptop). Two rules now own them: the sidecar side suppresses only when it equals the digest AT the
file's close over a decoding tail (`digestAtClose` — vandalized/truncated files still report), and
the manifest side compares entries only against a file whose scan ENDED at a Close (`endsClosed` —
a moving file convicts nothing; the next close's upsert replaces the entry anyway). 10/10 aimed
mutations caught across both; the closed-day verify after the boundary: **0 integrity, 54 manifest
entries (46 backfill + 8 close), zero unmanifested**.

**Read `docs/superpowers/specs/2026-07-30-capture-slice-f-full-verify.md`** — §38 is the whole
slice; §38h is the close (checklist findings, 30/30 mutations, the two driver-shaped test gaps).
What remains, in order:

1. **Slice D's one-day disk figure, ~09:53 UTC 2026-07-31** — `du -sh archive/` + a full-day
   verify (day 1 pace: 324M by 11:27 UTC, ≈6 GB/day, Bybit books dominating).
2. **The off-host destination** — the user's open §38f decision (option 4 taken 2026-07-30:
   destination-agnostic). The machinery is proven (`verify -root` over a copy; the done-when
   test); the laptop is STILL the only copy until this is decided. Loss exposure is calendar
   time on the kill-gate measurement, not irreplaceable data (§38f's honest calculus).
3. **§32m's next roster row** after D's figure lands.

---

## START HERE (updated MID-SESSION 38, 2026-07-30 — HISTORICAL — slice F was OPEN and PAUSED at a deliberate seam; session 39 resumed and closed it)

**Session 38 opened slice F** (full verify: manifests + cross-check, `capture
cat`, the sync claim — destination-agnostic by the user's recorded choice,
§38f) and paused mid-slice at a context limit with **everything built so far
committed and `make ci` green**. The single source of truth for what exists
and what is owed is **`docs/superpowers/specs/2026-07-30-capture-slice-f-full-verify.md`
→ `RESUME HERE`** — read it before touching anything. Headlines: the writer
now lands a `_manifest/<day>.json` entry on every clean close; verify has five
new manifest findings with two paid-for guards; backfill is
`capture manifest -day`; cat is built but **its tests are not written**; the
checklist pass, mutations, reading pass and close have **not** run; and the
live capture still runs the PRE-manifest v0.9.14 build — restart + backfill is
step 5 of the resume list, `pgrep -fl "arblab capture run"` first, always.
Slice D's disk figure is due ~09:53 UTC 2026-07-31.

---

## START HERE (updated end of session 37, 2026-07-30 — HISTORICAL — v0.9.14 is RELEASED, slice E is CLOSED, and **THE CAPTURE RUNS ON EIGHT STREAMS**)

**Branch `feat/milestone-1-foundation`.** Verify rather than trust: `git status --short` (clean),
`git rev-list --left-right --count @{u}...HEAD` (the exact number is whatever the command says),
`grep -rn "SPEC EDIT OWED" internal/ tools/` — expect **ZERO**; the newest marker reads
`[SPEC EDIT LANDED — v0.9.14]` at `TestRealCaptureConfigLoads`' overlay assertion. `make ci` green,
docs 102 pass.

**A capture process is (or should be) RUNNING on the v0.9.14 build**: restarted end of session 37
onto the eight-stream config — §32c's seven WS sources plus §32h's hourly `fundingInfo` REST
overlay. It still does NOT survive a reboot. Check it before anything else:

```
pgrep -fl "arblab capture run"      # alive?
bin/arblab capture verify -config configs/capture.json -from 2026-07-30T09:00:00Z -to <next hour boundary, RFC3339>
```

Expect **0 integrity findings, always**. The session-37 restart seam is recorded evidence
(`process_restart` gap + `restart_boundary` on the next full verify) — that is §32a's price, paid
as designed. The overlay stream (`.../funding/fundingInfo/`) fills at one Rest record per hour plus
Alive ticks; verify shows it as `rest N` with `data 0`, and that is NOT a dead feed (§37g).

**Read `docs/superpowers/specs/2026-07-30-capture-slice-e-rest-overlay.md`** — §37 is the whole
slice (decisions §37a–§37g, verification §37h, the spec edit §37i, the deliberate absences §37j) —
**and its `RESUME HERE`** for what remains, in order: **slice D's one-day disk figure at
~09:53 UTC 2026-07-31** (`du -sh archive/` + a full-day verify, recorded in the design record;
pace so far ≈6 GB/day, Bybit books dominating), then **slice F** (full verify, `capture cat`,
**off-host sync verified by digest** — the laptop disk still holds the only copy of an
unrecoverable archive, and F is where that stops being true).

---

## START HERE (updated end of session 36, 2026-07-30 — HISTORICAL — v0.9.13 RELEASED, slice C CLOSED, capture live on the SEVEN-stream config; superseded by session 37's restart onto eight)

**Branch `feat/milestone-1-foundation`.** Verify rather than trust: `git status --short` (clean),
`git rev-list --left-right --count @{u}...HEAD` (the exact number is whatever the command says),
`grep -rn "SPEC EDIT OWED" internal/ tools/` — expect **ZERO**, for the first time since slice A;
four markers read `[SPEC EDIT LANDED — v0.9.13]`. `make ci` green, all six gates clean, `-race`
clean on a cold cache, docs 102 pass.

**A capture process is (or should be) RUNNING**: `bin/arblab capture run -config configs/capture.json`,
started 2026-07-30 **~09:53 UTC** (measured, not the session's "~09:30" estimate: hour 09's files
hold 6/60 Alive ticks with seq continuous from 1), detached via nohup from the repo root, logging
to `capture.log` (gitignored, as is `archive/`). Hour 09 closed and sealed on schedule while
detached — sidecar present, and the first verify over a **complete** hour reported 0 integrity
findings, 7 `alive_short` coverage findings (the honest 6-minute hour). Note `verify` only judges
hours **fully inside** `-from/-to`; to see the open hour, pass `-to` at the next hour boundary.
Check it before anything else:

```
pgrep -fl "arblab capture run"     # is it alive? (it does NOT survive a reboot)
bin/arblab capture verify -config configs/capture.json -from 2026-07-30T09:00:00Z -to <now, RFC3339>
```

Expect **0 integrity findings, always**. Coverage findings are the laptop's honest ledger — sleeps,
reboots and network changes are recorded gaps, and §32a decision 2 priced that in. If the process
died, restart it (same command, from the repo root); the writer recovers its own files, seeds from
the archive, and the seam shows up as a `restart_boundary` coverage finding, which is it working.

**Read `docs/superpowers/specs/2026-07-29-capture-slice-c-transport.md` §36t–§36u** for what the
verification session found (six fixture collapses, 11/11 kills, the two reading-pass findings) and
**its `RESUME HERE`** for what a later session owes the running capture: a verify over the
accumulated range, slice D's one-day disk figure recorded in the design record, then §32m's slices
E (REST overlay) and F (full verify, `capture cat`, off-host sync). **The off-host copy matters
more than usual** — a laptop disk holds the only copy of an unrecoverable archive (§32a decision 2).

### The one tension left visible (§36u)

§32m slice D says "measure one day of funding on disk, THEN add sources 3–6" — written before §32c
pinned one config as the single spelling of the capture set. The RESUME's instruction ran all seven
from day one; the disk figure is still measured rather than estimated, which is what §31d actually
requires. Second observation (10:05 UTC): 50 MB in the first ~12 minutes across all seven, ≈6 GB/day
pace, Bybit books dominating. The real one-day figure becomes measurable **~09:53 UTC 2026-07-31**
(`du -sh archive/` plus a verify over the full day) and is slice D's to record.

---

## START HERE (updated end of session 35, 2026-07-30 — HISTORICAL — slice C was FULLY BUILT and NOT CLOSED: the config schema, `internal/app/capture` and `cmd/arblab` all landed; **the checklist + mutations + close have NOT run, and the calendar moment has NOT happened**)

**Branch `feat/milestone-1-foundation`.** Verify rather than trust: `git status --short` (clean),
`git rev-list --left-right --count @{u}...HEAD` (the exact number is whatever the command says —
recording it here cannot be right, because the commit that records it changes it),
`grep -rn "SPEC EDIT OWED" internal/ tools/` — expect **exactly four**, unchanged through three
sessions: the three §34 left on the design's §12 roster, plus §36a's `nhooyr.io/websocket` rename.
`make ci` green, all six gates clean, `-race` clean on a **cold** cache (`go clean -testcache`
first). Code is at the `feat(capture): config schema, composition root and CLI` commit.

**Read `docs/superpowers/specs/2026-07-29-capture-slice-c-transport.md` (§36), not this summary.**
Its `## RESUME HERE` now holds **verification work only**, four items: the fixture-collapse
checklist over the session-35 code (with three named suspects), the mutations (with six named
targets), the slice close (marker grep + the reading pass), and **the calendar moment** —
`bin/arblab capture run -config configs/capture.json` against the live venues. §36p–§36s are this
session's record.

### What landed (session 35)

`internal/app/capture` — the `platform/config` schema (no field hot-reloadable, asserted as a
`Reload` refusal; `dial_timeout`/`subscribe_timeout` had to be ADDED to the RESUME list's roster
because `StreamConfig.Validate` refuses zeros for all four timeouts), the composition root (writer
opens on the run's **first `Subscribed`** so the ack reaches the Open record; the stream goroutine
opens it and seeds the sink's counter from `Stats().LastSeq` — §36m's obligation — before handing
the writer to the consumer through a one-shot channel; transport gaps ordered by `afterSeq`), and
§32j's metrics under **closed label sets** with `Registry.Rejected() == 0` asserted end-to-end.
`cmd/arblab` — `capture run` / `capture verify`; the exit map is an exhaustive switch that spells
`Report.ExitClass` without re-deciding its precedence; the report prints the two finding axes
separately, never summed. `configs/capture.json` — §32c's seven verified sources, loaded by the
schema's positive-control test (one spelling of the capture set in the repo).

### The trap for the next session

**None of the session-35 code has been mutation-tested.** The suites pass and CI is green, and this
repo's whole history says that is not yet evidence. Run the checklist BEFORE the mutations — in
`boot` it changed five fixtures for free.

---

## START HERE (updated end of session 34, 2026-07-30 — HISTORICAL — slice C was PARTLY BUILT: transport, live `Mono` and verify had landed; three items remained)

**Branch `feat/milestone-1-foundation`.** Verify rather than trust: `git status --short` (clean),
`git rev-list --left-right --count @{u}...HEAD` (**a dozen or so ahead, nothing pushed** — the exact
number is whatever the command says, and writing it here cannot be right, because the commit that
records it changes it; session 20's handoff said 3 and was wrong for exactly this reason. The code
this block describes is `87bb26b`), and
`grep -rn "SPEC EDIT OWED" internal/ tools/` — expect **exactly four**, unchanged from session 33:
the three §34 left on the design's §12 roster, plus §36a's `nhooyr.io/websocket` rename in
`internal/engine/venue/transport/doc.go`. `make ci` green, all six gates clean, `-race` clean on a
**cold** cache (`go clean -testcache` first), docs 102 pass.

**Read `docs/superpowers/specs/2026-07-29-capture-slice-c-transport.md` (§36), not this summary.**
Its `## RESUME HERE` now lists **three** items, each with the obligations it carries, so nothing has
to be re-derived. §36a–§36h are session 33's transport; **§36i–§36o are this session's verify.**

### What landed

`internal/engine/capture/finding.go` and `verify.go` — §32k's out-of-process liveness check, plus
three test files. **20 mutations / 20 kills**, after two real survivors and three re-aims. No new
spec markers.

Two axes that are never summed (§36i): a **verdict** per `(stream, window)` —
`replayable`/`gapped`/`absent`, about replay — and a **finding class**, `integrity` (exit 1, the
archive is lying) or `coverage` (exit 2, honest and thin), about trust. `Report.ExitClass()` puts
integrity first. **The class is a METHOD on the finding kind, not a field**, so adding a twentieth
kind is a compile-time failure until somebody decides its exit code — probed, not assumed.

### NOT landed — this is the rest of slice C

The `platform/config` schema, `internal/app/capture` (the composition root), and `cmd/arblab`.
**The calendar moment has not happened**: nothing is capturing yet.

### Four findings worth carrying

1. **`time.Time.Truncate` anchors at the ZERO TIME, not the Unix epoch** (§36j). `Writer.nextTick`
   uses `Truncate`; verify's expected-tick count used `UnixNano`. The epoch sits 62135596800 s after
   year 1 and **62135596800 mod 420 = 60**, so the two 7-minute grids sat one minute apart and one
   boundary crossed each hour edge — one window over, the next under, for a healthy archive. **The
   grids agree exactly when the interval divides 60 s**, so a one-minute Alive interval hides it
   completely. §32l's coprime requirement is what exposed it. Reusable form: when a check recomputes
   what another component computes, the two must share the **anchor**, not merely the formula.
2. **A declared enum member nothing emits is the unreachable-sentinel defect one layer out** (§36k).
   `FindingAliveShort` was declared, classified, documented and never produced — the shortfall was
   computed inline to set the verdict. Verdict right, finding list silent, exit code **clean**.
   Compiler, `exhaustiveswitch` and `Class()` coverage were all satisfied. Fix is structural: every
   reason to be gapped is now a finding, and the verdict derives from the list alone. **Its mirror
   was eleven lines up** — the bad-magic early return did not mark the walk aborted, so a file verify
   declined to read was faulted for coverage it never measured.
3. **A live slice-B defect, found by writing a fixture** (§36m). A restart into an hour the writer
   had already opened refuses **every** `Data` record for the rest of that hour: `OpenWriter` takes
   the recovered sequence as its floor and `Data` requires strict ascent. The composition root must
   seed the reader's counter from `writer.Stats().LastSeq` — **item 2's first obligation**, no new
   API needed. Review had read that line three times; writing a two-writer fixture found it.
4. **§35's item 2 was addressed to the wrong slice** (§36l). Source 4's missing wire-level gap check
   does not reach verify, because verify reads **no payload a venue wrote** — its continuity is over
   our own `CaptureSeq`, stamped identically for all seven sources. Source 4's shape is in the
   healthy fixture anyway, so "verify treats every source alike" is checked rather than assumed.
   **A debt written against a document can outlive the reason it applied.**

Also: diagnosing the two surviving mutants was the work (§36n). One was a **wrong mutant** —
`start = i * 0` makes verify parse *more* files, not fewer, so it is not a mutant of a claim about
doing less. The other was a **real test gap** whose diagnosis produced a sharper claim than the one
under test: a lying Close record is **one** fault, not two.

---

## START HERE (updated end of session 33, 2026-07-30 — HISTORICAL — slice C's transport and the live `Mono` landed; four items remained)

**Branch `feat/milestone-1-foundation`.** Verify rather than trust: `git status --short` (clean),
`git rev-list --left-right --count @{u}...HEAD` (**8 ahead**), and
`grep -rn "SPEC EDIT OWED" internal/ tools/` — expect **exactly four**: the three §34 left on the
design's §12 roster, plus a new one in `internal/engine/venue/transport/doc.go`. `make ci` green, all
six gates clean, `-race` clean, docs 102 pass.

**Read `docs/superpowers/specs/2026-07-29-capture-slice-c-transport.md` (§36), not this summary.** Its
`## RESUME HERE` lists the four remaining items in order, each with the obligations it must carry, so
nothing has to be re-derived.

### What landed, and what did not

Landed: `internal/engine/venue/transport/` (the raw WS socket — three-part endpoint, both venue
dialects, subscribe + ack, **§32k's first-data assertion**, reconnect-with-gap, stats) and
`platform/clock.Wall.Mono()`, which resolves §34c's open decision. **7 mutations / 7 kills.**
New dependency: `github.com/coder/websocket v1.8.15`, the engine module's first runtime third-party
dependency, zero transitive deps, the user's call (§36a).

**NOT landed — this is the rest of slice C:** `capture verify` (minimal), the `platform/config` schema,
`internal/app/capture` (the composition root), and `cmd/arblab`. **The calendar moment has not
happened**: nothing is capturing yet.

### Four findings worth carrying

1. **A pong renewing the first-data window** (§36d). Refusing to count a pong as data is NOT enough
   for §32k: with a per-read deadline every non-data frame RESTARTS the window, so a 20 s heartbeat
   under a 30 s timeout renews it forever and the assertion never fires. The deadline is now absolute
   from subscription. Found by writing the test, in code written by someone who had just spent a page
   arguing pongs must not satisfy the assertion.
2. **Fixture collapse number TEN, and it is a new variant** (§36e). The mutation for finding 1
   SURVIVED because the fake venue called `Conn.Ping`, which **blocks**, so exactly one pong was ever
   sent — the test was the silent-socket test wearing a different name. The collapsing parameter was a
   **rate**, pinned by a side effect of a library call rather than by a literal, so reading the
   fixture's arguments could not find it. The guard is an assertion on the FIXTURE
   (`Stats().PongFrames >= 3`). **When a test's discriminating power depends on a rate, assert the rate.**
3. **A claim written inline can be unmutatable** (§36f). That mutation first reported `NOBUILD!!`
   because breaking it made a neighbouring variable unused. The re-aim was not a different `sed` — it
   was extracting `readDeadline()` so the claim had a name. **If a claim cannot be mutated without
   breaking the build for an unrelated reason, it is entangled with its neighbours and wants a name.**
4. **§32i's `notimenow` prediction survives, but not for the predicted reason** (§36g). Two real
   escapes (backoff timer, venue heartbeat ticker). The predicted use — *socket read deadlines* —
   needed **none**, because a ctx-first library routes every deadline through `context.WithTimeout`,
   which reads no clock in our code. Taking `gorilla/websocket` instead would have needed an escape on
   every read: the library choice changed which lines are policed. Also new: **`notimenow` polices
   `engine/` TEST files** — four escapes went into the transport's test harness.

Also resolved: **§34d's `..` traversal finding was applied to its mirror before it bit** — a topic is a
path segment in a URL too, so the guard went in the session the second path-segment site appeared
(§36c).

---

## START HERE (updated end of session 32, 2026-07-29 — HISTORICAL — §13's debts were CLEARED; slice C was UNBLOCKED and unstarted)

**Branch `feat/milestone-1-foundation`. No code changed this session** — `make ci`, the six gates and
`-race` are as session 31 left them (green). Verify rather than trust: `git status --short`,
`git rev-list --left-right --count @{u}...HEAD`, and `grep -rn "SPEC EDIT OWED" internal/ tools/` —
expect **exactly three**, unchanged, all on the design's §12 roster.

**Session 32 did the research task that had blocked slice C for three sessions.** All five of §13's
verification debts and the standing depth-update-ID debt are **cleared against primary venue
documentation**. The facts are in **`docs/venue-profiles.md` §5** (new, dated 2026-07-29, fully cited);
the record is **`docs/superpowers/specs/2026-07-29-profiles-verification.md` §35** — read that, not this
summary. `2026-07-29-capture-tool-design.md` §13 and §32c are updated in place.

### The next work: slice C (§32m), and it is now unblocked

Raw WS transport, subscribe + ack + first-data assertion, reconnect-with-gap, `arblab capture run`,
config, observability, and the **minimal `capture verify`** with its positive control. **Funding capture
for BTC on both venues starts running here — this is the calendar moment.** §35's `RESUME HERE` lists the
four things it must carry out of the verification; the two that change its design rather than its
constants are:

- **Config must express host, path prefix and topic separately.** The seven sources sit on **three**
  Binance hosts with three path disciplines (`/market` for funding, `/public` for futures book, and
  `stream.binance.com:9443` with **no prefix and different speeds** for spot book), and Bybit sources 5
  and 6 have **byte-identical topic strings** distinguished only by URL. §32c's table now carries a
  `Connect to` column; a config keyed on topic alone cannot express it.
- **Source 4 (Binance *spot* partial depth) has no wire-level gap check at all** — its payload is
  `{lastUpdateId, bids, asks}`: no span, no back-link, no event time. `capture verify`'s arm for it cannot
  assert what the other six can, and the positive control must not be written so the asymmetry disappears.

**Still open and unchanged: the live `Mono` source** (§34c). A `Mono` method on `platform/clock.Wall` or
an adapter in `internal/app/capture`. **It must not be `Now() - start`** — that satisfies the interface
and defeats the field.

### Three findings worth carrying

1. **The Binance SPA has a server-rendered twin** (§35a). `developers.binance.com/docs/…` returns an
   empty app shell; **`/en/docs/catalog/…` returns the same docs as real text.** Every Binance fact in
   Profiles §5 was fetched verbatim from it, no browser involved. **§4's three `⚠ browser-confirm`
   blocks are therefore removable by re-fetch** — available work, deliberately not done here. The reason
   this stood since 2026-07-20: §4 *already cited one catalog URL*, filed beside four shell fetches and
   caveated with them. **A source that worked is invisible when it is listed beside four that did not.**
2. **A debt that names a count has already answered part of the question** (§35c). Standing debt #1 asked
   for *"the **two** venues' depth update-ID shapes"* and there are **three**, split by product and stream
   type rather than by venue. It survived four review rounds and five sessions because re-reading it
   re-reads the schema. Same shape as §33c's rejected alternative, one layer earlier: there an **answer**
   arrived pre-worded and went unchecked, here a **question** did — and a debt list is the more dangerous
   of the two, because re-reading it is what you are supposed to do with it.
3. **A live probe contradicted the design's stated premise three times** (§35e). §32h rests on
   *"absent ⇒ 8 h"*; **BTCUSDT is present**, not absent. **4 h is the majority interval** (442 of 742)
   and Profiles recorded no 4 h case at all. And **`updateTime` is `null` on BTCUSDT specifically** while
   other rows carry an integer — a type that holds for 740 rows and fails on the one symbol we trade.

---

## START HERE (updated end of session 31, 2026-07-29 — HISTORICAL — slice B was BUILT; slice C was BLOCKED)

**Branch `feat/milestone-1-foundation`, `make ci` green, all six gates clean, `-race` clean.**
Verify rather than trust: `git status --short`, `git rev-list --left-right --count @{u}...HEAD`, and
`grep -rn "SPEC EDIT OWED" internal/ tools/` — expect **exactly three**, all intended and all on the
design's §12 roster (rows 2, 3, 5).

**Session 31 built slice B of §32m: the writer.** Archive layout and its inverse, the gap taxonomy, the
exclusive per-stream `flock`, torn-tail recovery, the bounded drop-newest queue, and the writer itself —
rotation on UTC `RecvTime`, unconditional `Alive` ticks, two digests at close. The record is
**`docs/superpowers/specs/2026-07-29-capture-slice-b-writer.md` §34** — read that, not this summary.
The design it implements is §32; slice A's codec underneath it is §33.

Verification: **23 mutations, 23 kills**, after diagnosing **three survivors** that were all real test
gaps (§34e). `exhaustiveswitch`'s binding on `Cause` was probed by hand rather than assumed.

### §33c is CLOSED, and not the way it was parked

The two previous handoffs carried it as an open five-line format change that got cheaper the sooner it
was taken. **It was never available.** A decoder must bound `frameLen` and check that the claimed bytes
are AVAILABLE before it can checksum them, and no field order covers an absent span — so moving the
checksum ahead of the length leaves the one mis-diagnosed case exactly as it was. **The user decided:
keep v1.0.** §33c now carries the correction in place. What would actually close the hole is a second
header-only checksum, costing 4 bytes per record; it is recorded there and not taken.

### The next work: slice C (§32m) — **the calendar moment, and the first BLOCKED slice**

Raw WS transport, subscribe + ack + first-data assertion, reconnect-with-gap, `arblab capture run`,
config, observability, and the **minimal `capture verify`** with its positive control. Funding capture
for BTC on both venues starts running here.

**It cannot start until §13's five Profiles verifications are done, and they have not moved for three
sessions.** Three are missing venue endpoint facts — 1 (Binance per-symbol funding topic), 3/4/7
(Binance futures book host, path, cadences), 5/6 (Bybit linear public WS URL) — plus standing debt #1
and the public-vs-authenticated question for the REST overlays. **Nothing goes into Profiles unverified,
and nothing built may depend on an unverified fact.** This is a research task against primary venue
documentation and it wants its own scope. **It is now the critical path for the whole milestone.**

Also open, and smaller: **the live `Mono` source** (§34c). Either a `Mono` method on
`platform/clock.Wall` or an adapter in `internal/app/capture`. **It must not be `Now() - start`** — that
satisfies the interface and defeats the field, because `Wall.Now` is clamped monotonic and a derived
`Mono` stands still during exactly the window it exists to measure.

### Three findings worth carrying

1. **A topic of `..` climbed out of the archive tree** (§34d), found by the `rapid` round-trip property
   on its fourth case. `.` is in §32e's unencoded set and `filepath.Join` CLEANS `.` and `..` away, so
   two streams shared a path and one escaped the root — with encoder, decoder and every hand-written
   fixture agreeing nothing had happened. Fixed by escaping a LEADING dot only.
2. **All three surviving mutants were fixtures on the cheap side of a SIZE threshold** (§34e): a torn
   tail smaller than the recovery's own writes, a record smaller than the scan buffer, a gap refused by
   a second check as well as the one under test. That is a new checklist entry beside "pinned to the
   value that collapses the distinction" — *find the fixture too small for the mechanism to matter.*
3. **A rejected alternative recorded with a stated benefit is an unchecked claim.** §33c's survived four
   reviews and two handoffs by being written down confidently. Nobody re-derives a rejection; they
   re-read it.

---

## START HERE (updated end of session 30, 2026-07-29 — HISTORICAL — slice A was BUILT; slice B was the open work)

**Branch `feat/milestone-1-foundation`, `make ci` green, all six gates clean, `-race` clean.**
Verify rather than trust: `git status --short`, `git rev-list --left-right --count @{u}...HEAD`, and
`grep -rn "SPEC EDIT OWED" internal/ tools/` — expect **exactly two**, both intended and both on the
design's §12 roster.

**Session 30 wrote the first code of milestone 2.** Slice A of §32m: the frame codec, its oracle, the
`rapid` properties and the fuzz target, in `internal/engine/capture/`. Plus §32i's
`platform/clock.Wall.Backsteps()`, which the session-29 handoff named as a companion item. The record
is **`docs/superpowers/specs/2026-07-29-capture-slice-a-frame-codec.md` §33** — read that, not this
summary. The design it implements is §32; neither is repeated in the other.

Verification: **21 mutations, 21 kills**; **25.5 M fuzz executions, no finds**; every negative control
bites and every positive control passes.

### The next work: BUILD slice B (§32m)

The writer — files, hourly rotation on UTC `RecvTime`, the exclusive `flock`, torn-tail recovery, gap
records and `Alive` ticks, driven by a **fake** source under `TestClock`. Done when §32l's
fixture-collapse table is satisfied and `-race` is clean. Like slice A it needs **no venue facts**.

### Two things open, and both should be settled BEFORE slice C starts the calendar

1. **A format question the build surfaced (§33c).** `frameLen` sits **outside its own integrity cover**
   — §32e puts it ahead of the checksum, and the checksum covers only what follows itself. The
   practical consequence is narrow: corruption of the **last** record's length field reports as
   `ErrTruncated` rather than `ErrChecksum`, which matters because `ErrTruncated` is the one verdict
   the recovery acts on **by truncating the file**. Moving the checksum ahead of the length closes it
   in five lines and is a **format-major** change — cheap today, an archive migration once capture
   runs. §32e's field order was kept and all three outcomes are tested; **the change is the user's
   call.**
2. **§13's five Profiles verifications — NOT STARTED.** They block slice C's sources and slice C is the
   calendar moment. Three are missing venue endpoint facts. Nothing goes into Profiles unverified.

### Three findings worth carrying

1. **An invariant set that consulted the oracle only on the SUCCESS path let a negative control
   through** (§33e). A decoder answering `ErrTruncated` to every refusal satisfied all four invariants,
   because "exactly one sentinel" is satisfied by the wrong sentinel. Agreeing that an input is bad is
   worth little; agreeing **why** is the contract.
2. **A surviving mutant had a third meaning beside "test gap" and "wrong mutant": a guard nothing can
   trigger** (§33e). `!lastSys.IsZero() &&` was unreachable because the zero time is year 1. Deleted
   rather than tested — this is session 27's unreachable-sentinel finding in different clothes, and it
   argues for running mutations over **defensive** code specifically.
3. **`RecordKind` forced a second §3.6 rule 8 exception** (§33a), and the argument for it is §32e's own
   NDJSON rejection one level down: *the archive is the fact; readability is a tool over it.* What the
   integer shape buys is that a **zero-filled page cannot decode as a valid record**.

---

## START HERE (updated end of session 29, 2026-07-29 — HISTORICAL — the capture tool was DESIGNED; slice A was the open work)

**Branch `feat/milestone-1-foundation`, `make ci` green, doc set v0.9.12, marker grep zero.**
Verify rather than trust: `git status --short`, `git rev-list --left-right --count @{u}...HEAD`, and
`grep -rn "SPEC EDIT OWED" internal/ tools/` (expect **zero**).

**Session 29 wrote no code.** It took the two decisions session 28 left open and designed the capture
tool against them. The design is
**`docs/superpowers/specs/2026-07-29-capture-tool-design.md` §32** — read that, not this summary,
before writing any of it. Session 28's reasoning (`…-milestone-2-capture-decisions.md` §31) stays live
and is cited throughout.

### The two decisions, taken (§32a)

1. **Self-capture covers funding AND books** — §31d resolved. The sharpened question is *"what does the
   parity claim cover?"*, because §8's ≥3-month scoring sample comes from Tardis either way; capture
   buys parity + recent days. Book capture is **Binance partial-book snapshots** (each message stands
   alone) plus **one Binance diff stream as evidence**. **Correction the venue facts forced: Bybit has
   no self-contained snapshot feed** — `orderbook.{1|50|200}` is snapshot+delta — so the Bybit half is
   reconstructed at read time regardless, and **standing debt #1 (the depth update-ID shapes) is now
   BLOCKING for the Bybit book sources**, not background.
2. **Capture runs on the dev machine for now.** So `capture verify` ships with the *first* capture, not
   after it: on this host gaps are expected, and an archive whose gaps are expected is worthless unless
   they are measured. The line is *a recorded gap is not an error; an unexplained silence is.*

### The next work: BUILD slice A (§32m)

The frame codec with its `math/big`-style oracle, a `rapid` round-trip property and a fuzz target. No
I/O, no network, **no venue facts** — the one slice none of §13's verification debts block. Slices
B (writer) → C (**funding capture STARTS RUNNING — the calendar moment**) → D (measure a day, then
books) → E (REST overlay) → F (full verify + off-host copy).

**Do §13's five Profiles verifications in parallel with A and B** — they block slice C, and slice C is
where the calendar starts running. Three of them are missing venue endpoint facts we would otherwise
have guessed.

### Three findings from the design worth carrying

1. **`Wall.Now()` absorbs a backwards clock step by design, and capture is where that is wrong** (§32i).
   The clamp is right for the engine — "a timer fires late rather than a settlement evaluated twice" —
   but on a laptop an NTP step yields a run of **identical `RecvTime`s**, §3.4's cache **accepts an
   equal `RecvTime`** (added deliberately in session 27), and the replay proceeds over a squashed time
   axis with every freshness and skew number wrong and nothing failing. **No caller can currently learn
   the clamp engaged.** Chosen fix: `Wall` gains a pure `Backsteps()` accessor; capture records a
   `ClockStep`. A `[SPEC EDIT OWED]`.
2. **The capture key is `(SourceID, Topic)`, not `SourceID`** (§32c) — one `SourceID` has more than one
   wire representation. The realistic accident is not the evidence stream; it is changing
   `@depth20@100ms` to `@250ms` six weeks in, which keyed on `SourceID` alone puts two cadences in one
   file with nothing recording the change. Departs from §3.5's literal "per-`Source`", so it is marked.
3. **§3.5's "parity replays the persisted dispatch log, not raw capture" reads forward as a format
   property** (§32d): the capture log has **no cross-source order and needs none**. That is what makes
   minting no `AcceptOrdinal`/`IngressSeq` safe rather than merely disciplined.

---

## START HERE (updated end of session 28, 2026-07-29 — HISTORICAL — milestone 2 opened; the capture tool's DESIGN was the open work)

**Branch `feat/milestone-1-foundation`, `make ci` green, doc set v0.9.12, marker grep zero.**
Verify rather than trust: `git status --short`, `git rev-list --left-right --count @{u}...HEAD`, and
`grep -rn "SPEC EDIT OWED" internal/ tools/` (expect **zero**).

**Session 28 wrote no code.** It opened milestone 2, fixed its ordering, and settled one question that
would otherwise be reopened every time somebody looks at the funding feed. Everything below is in
decision record **`docs/superpowers/specs/2026-07-29-milestone-2-capture-decisions.md` §31**, with the
rejected alternatives — read that, not this summary, before designing anything.

### The next work: DESIGN the capture tool (a fresh conversation, by the user's instruction)

**Milestone 2 is the KILL GATE** — if measured net carry does not clear the cost hurdle, the project
stops. The deliverable is a number with a verdict attached. Its ordering:

| # | Step | Status |
|---|---|---|
| 1 | **Self-capture of the venue streams** | **NEXT — design it** |
| 2 | Parity dispatcher + persisted `DispatchSeq` (+ `outbox`'s store half, the first migration) | unbuilt |
| 3 | `strategy` context + `cashcarry` + §2.4's money math + **a fee-fetch seam that does not exist** | unbuilt |
| 4 | The pre-registered calibration artifact (§8) — approved **before** the sample is scored | unbuilt |
| 5 | The research simulator → the verdict → the four §12.2 forks (a **user** decision) | unbuilt |

**Capture is first because it is the only item whose cost grows while other work happens** (§31a).
Today is the only day you can capture today; §8's *"parity + recent days"* is calendar-bound on both
halves, and the paired our-capture-plus-vendor window can only be created going forward.

### Decide this before writing the design (§31d)

**Does self-capture cover BOOKS as well as funding?** §12.2 says "both funding streams"; §8's fill model
requires *"time-synchronized spot & perp books"* and depth-walking, which funding alone cannot support —
and Profiles §3 says spot has **no mark and no funding at all**, so the spot leg prices from its own book.
The wider set is the safer default and is plausibly a **core §12.2 spec edit**. Getting this wrong is
discovered at step 5, when the missing months are unrecoverable.

### Do not re-open — settled in §31b, categorically

**The funding rate is CONSUMED from the venue, never derived locally.** Not because deriving is hard —
because the funding rate is **the venue's own policy computation and it is what debits the account**.
There is no independent truth behind it to approximate better; a perfect local replication that disagreed
would be the wrong number by construction. **Trades are not even its input** — the premium index comes
from the *book*, and `tutorial.md` §1.6 records that the venue builds mark and funding off index + book
precisely so trades cannot drive them. The discriminator: **reconstruct** what the venue reports lossily
about an independent truth (the order book); **take** what the venue's computation defines (funding, mark,
liq price, fee tier). §8's Coinglass rejection is the same call already made once.

**And do not optimize the capture path for latency (§31c).** The signal path is latency-insensitive by
construction — holds of weeks, a ~3-day EWMA half-life, ~60 settlements per hold. A *systematic* 0.001%
per-settlement error compounds to ~0.06% of notional against a 0.34–0.38% round trip; two seconds of
staleness moves the verdict by nothing. Latency matters in execution (§6.6) and mark-driven liq
monitoring (§7.1) — different pipelines. **Capture's properties are losslessness and provenance.**

### Four constraints the design already has (§31g)

1. **Raw bytes verbatim + `RecvTime` + a per-source counter, append-only.** Parsing is redoable forever;
   capture is not redoable at all.
2. **Capture the REST overlay too** — Binance's `fundingInfo`, where **absent means 8h**, so without it
   "asked and empty" and "never asked" are indistinguishable in our own archive.
3. **Gaps recorded, never absorbed** — §3.5's capture lane marks the window non-replayable and alerts.
4. **Its own liveness check, with a positive control.** A dead capture and a quiet market are identical
   from outside. Kill a stream deliberately and confirm a recorded gap, not a silent hole.

Two venue traps that fail silently: Binance's legacy `/ws/` path was decommissioned 2026-04-23 and
**returns nothing rather than erroring**; Bybit's **absent field means unchanged, never null-coerce**.

**The capture tool needs no dispatcher** (§31e — it performs no arbitration, so milestone 1's
"no venue clients" objection was tested and does not apply), and **must never grow into the engine's
ingestion path**, which is how it would become a second authority over ordering.

### One correction to the line above (§31f)

**§12.1's roster is empty of packages but not of its own sentence** — it also says *"Parity dispatcher …
from day one"*, and the dispatcher is unbuilt: `IngressSeq` and `TimerSource` exist in the tree only as
comments promising a component that will stamp them. **A recorded deferral, not drift** (it needs a
persisted `DispatchSeq`; `migrations/` holds only a README). But it **stops being deferrable here**,
because §8 defines the simulator as the §3.5 engine with a single total-order dispatcher, *not* a
parallel simulator — and it is already leaking, in `engine/state`'s equal-`RecvTime` acceptance whose own
comment says the invariant lives in a component that does not exist.

---

## START HERE (updated end of session 27, 2026-07-27 — HISTORICAL — the CLOSE, v0.9.12, and milestone 1's roster is empty)

**Branch `feat/milestone-1-foundation`, tree clean, `make ci` green, all six gates clean, `docs` 102 pass.**
Verify rather than trust: `git status --short`, `git rev-list --left-right --count @{u}...HEAD`, and
`grep -rn "SPEC EDIT OWED" internal/ tools/` (expect **zero**).

### Where the work is

**Milestone 1 has NO unbuilt and NO open slice.** Seven slices, all closed. The doc set is at
**v0.9.12** across all six headers.

| Path | Holds |
|---|---|
| `internal/platform/` | the ten-package shared kernel |
| `internal/engine/venue/` | the normalized venue-seam DTOs |
| `internal/engine/state/` | §3.2's latest-value cache + §3.4's cross-leg skew — **CLOSED this session** |
| `internal/marketdata/` | funding + book paths, `application/ports/`, `module.go` |
| `internal/oms/` | §5.3's lifecycle + ledger, ports, dispatch, the write repository, `module.go` |
| `tools/lint/` | all six §3.6 gates, enforced |

### The next work: §12 MILESTONE 2, which is a KILL GATE

This is the decision the project exists to reach: **if measured net carry does not clear the cost
hurdle, the project stops.** Read §12's milestone-2 roster before designing anything — the shape of
the work is a measurement, not a feature, and the deliverable is a number with a verdict attached.

**The one thing the close leaves pointed at it:** §5.2 now carries §3.4's freshness budget family
and the cross-leg skew limit as *configuration*, and milestone 2's verdict is the output of a money
computation. Both halves of the CLAUDE.md warning apply at once — an unpopulated cost reads as free,
and a wrong rounding direction moves the number across the threshold without anything failing.

### Three things worth carrying, all from the close

1. **The reading pass out-produced the marker grep, three releases running.** Two of this slice's
   four edits carried markers; the other two — the latest-value write rule and §5.2's config family
   — existed as claims nobody had marked, because at writing time each read as *implementing* an
   existing requirement rather than departing from one. The grep's three named limits are in ADR §3
   and decision record §28d; this adds no fourth limit, it just makes the pattern a majority.

2. **The fixture-collapse tell moved out of the tests and into the error taxonomy** (§30c).
   `ErrInstrumentMismatch` was declared, documented as an assertion, returned by nothing, and
   unreachable by construction — so `errors.Is` against it compiled, always answered false, and
   **read as a handled case**. No coverage tool objects: an unused package-level `var` is legal Go.
   No mutation harness objects either: killing it changes no test. **The check that finds this class
   is "does anything `return` this sentinel", and it is now worth running at every close.**

3. **A silence can be a question the core RAISES and declines** (§30b). Earlier blocks record
   omissions (v0.9.6) and statements made twice incompatibly (v0.9.9). The skew-timestamp edit is
   neither: §3.5 names `SourceTime` *and* `RecvTime` for cross-leg skew, so the reader is told the
   question exists and left to answer it — and the answer a reader reaches for first is the one that
   silently measures the venues' clock offset instead of the market.

### The one standing debt, unchanged and invisible to the close gate by construction

**The `venue-profiles.md` verification of the two venues' depth update-ID shapes**, which
`venue.BookDelta`'s span design cites. Owed since session 20. A **Profiles verification** rather than
a core edit, so the marker grep has never answered for it and never will.

---

## START HERE (updated end of session 26, 2026-07-27, HISTORICAL — `engine/state` BUILT, not closed)

**Branch `feat/milestone-1-foundation`, tree clean, `make ci` green, all six gates clean.** Verify
rather than trust: `git status --short` and `git rev-list --left-right --count @{u}...HEAD`.

### Where the work is

**Milestone 1's SEVENTH slice — `engine/state` — is BUILT and OPEN.** `grep -rn "SPEC EDIT OWED"
internal/ tools/` returns **TWO**, which is correct mid-slice: markers land at the close. The doc set
is **still v0.9.11** and correctly so — the version marks a slice close.

| Path | Holds |
|---|---|
| `internal/platform/` | the ten-package shared kernel |
| `internal/engine/venue/` | the normalized venue-seam DTOs |
| `internal/engine/state/` | **NEW** — §3.2's latest-value cache + §3.4's cross-leg skew |
| `internal/marketdata/` | funding + book paths, `application/ports/`, `module.go` |
| `internal/oms/` | §5.3's lifecycle + ledger, ports, dispatch, the write repository, `module.go` |
| `tools/lint/` | all six §3.6 gates, enforced |

With this, **§12 milestone 1's roster has no unbuilt item left.** Reasoning with the rejected
alternatives: decision record **§29**. §28 stays live for §28d's third named limit on the close gate.

### The next work: THE CLOSE, then milestone 2

Two spec edits are owed, both stated in full in §29c and §29d, so neither has to be re-derived:

1. **Core §3.4 — a latest-value cache re-judges freshness at READ time.** §3.4 names the silence
   timer and does not name the cache, so the defect it forbids is available one hop downstream.
2. **Core §3.4/§3.5 — cross-leg skew is measured over `RecvTime`.** The spec names both timestamps
   for that metric and chooses neither; over `SourceTime` it is the unsynchronized-clock subtraction
   §3.5 forbids.

Then the doc-set bump, the ADR block, the reading pass, and §12 milestone 2's kill gate.

### Two things worth carrying, both from the build

1. **A latest-value cache is §3.4's silence-timer defect one hop downstream** (§29c). A cache that
   stores the verdict `marketdata` computed at ACCEPT time reports a dead feed **fresh for the whole
   outage, with every light green** — the observation ages while nothing arrives to re-judge it. The
   response is structural, not careful: `FundingEntry` and `BookEntry` have **no `Freshness` field**,
   so the stale verdict is unrepresentable rather than merely unused, and re-judging is forced to the
   instant of the read. A **reflection test over the struct fields** is what bite-checks it, because
   the field somebody adds back will look like a helpful convenience.

2. **"Single-writer" is a claim about AUTHORITY, not about synchronization** (§29f). §3.4's sentence
   names who may *write*; `strategy`, `risk` and `oms` read concurrently, so the maps are
   `RWMutex`-guarded. **This is the claim `mutate.sh` cannot report on** — the harness runs `go test`
   without `-race`, so deleting the `RLock` is a survivor in its output. Hand-checked instead. A
   harness's silence on a claim is not the same as a harness clearing it.

### The one standing debt, unchanged and invisible to the close gate by construction

**The `venue-profiles.md` verification of the two venues' depth update-ID shapes**, which
`venue.BookDelta`'s span design cites. Owed since session 20. A Profiles verification rather than a
core edit, so the marker grep never answered for it.

---

## START HERE (updated end of session 25, 2026-07-27, HISTORICAL — the close)

**Branch `feat/milestone-1-foundation`, tree clean, `make ci` green, `docs` 102 pass.** Verify
rather than trust this sentence: `git status --short` and
`git rev-list --left-right --count @{u}...HEAD`.

### Where the work is

**Milestone 1's first SIX slices are CLOSED** — `platform` foundations, the shared kernel,
`marketdata` (funding), `engine/venue`, `marketdata` (book), and `oms`. **Nothing is owed to close
what exists**, and the marker grep returns zero.

| Path | Holds |
|---|---|
| `internal/platform/` | the ten-package shared kernel |
| `internal/engine/venue/` | the normalized venue-seam DTOs |
| `internal/marketdata/` | funding + book paths, `application/ports/`, `module.go` — **now rule-2 compliant** |
| `internal/oms/` | §5.3's lifecycle + ledger, ports, dispatch, the write repository, `module.go` |
| `tools/lint/` | all six §3.6 gates, enforced |

Reasoning with the rejected alternatives: decision record **§27** (the slice) and **§28** (the
close). §26 stays live for §26e's gate scope; §25 for §25b's classifier seam; §24 for §24e's table.

### The next work

**Milestone 1's remaining slices, then milestone 2's kill gate.** There is no in-flight slice and
no owed marker, so the next session starts from §12's roster rather than from a partial context.

### Four things worth carrying, all from the close

1. **SM §1's three new rows are ONE silence appearing three times** (§28b). The transitions table
   is complete about the edges somebody *designs* and silent about the ones a **venue** takes or a
   **recovery path** re-enters through. All three failed identically when absent — **a stall with
   every light green**, never a wrong order — because each source state contributes a §6.7
   possible-delta interval, so the hedge group sits `Ambiguous` forever with nothing reporting an
   error. The sharpest: without the venue-initiated `Canceled` row, **the only order type this
   system sends could never terminate on its ordinary outcome**.

2. **The seventh edit came from the READING PASS, not the marker grep** (§28c). §5.1 said "a
   `Version int64` (OCC)" and the code *relied on* two things it never stated — the version starts
   at 1, and only a write that changes the record increments it. A claim the code merely relies on
   is never marked. Each half fails in a **different** direction, which is why stating one alone
   would be worse than stating neither.

3. **The slice-close gate now has a THIRD named limit** (§28d), and it is the one to remember.
   v0.9.8: the grep cannot see an *unmarked* claim. v0.9.10: nor a rule the code *silently breaks*.
   v0.9.11: **nor can the reading pass** — because a rule the code silently does not meet looks
   exactly like a rule it meets, especially when the non-compliant shape is the one a reviewer
   would call correct. What found §3.6 rule 2's drift was `oms` **implementing the same sentence**,
   so one repo held two contexts disagreeing. **A second implementation sees what neither the grep
   nor the reading pass can.**

4. **`mutate.sh` reports `NOBUILD!!` for a claim the COMPILER enforces** (§28e) — a kill wearing a
   non-result's verdict. Breaking an adapter guarded by a `var _ ports.X = (*Impl)(nil)` fails the
   build, which the harness calls "the tests never ran". Read the error: *"does not implement
   ports.X"* is the assertion working, and it names its own line. `CLAUDE.md`'s verdict table now
   says so. Every other `NOBUILD!!` still means re-aim.

### The one standing debt, and it is invisible to the close gate by construction

**The `venue-profiles.md` verification of the two venues' depth update-ID shapes**, which
`venue.BookDelta`'s span design cites. Owed since session 20 and untouched. It is a **Profiles
verification rather than a core edit**, which is exactly the item v0.9.10's ADR block warned gets
filed under the wrong process — the marker grep answers only for spec edits, so this did not and
will not block a close.

---

## START HERE (updated end of session 25, 2026-07-27, HISTORICAL — mid-slice)

**Branch `feat/milestone-1-foundation`, tree clean, `make ci` green.** Verify rather than trust
this sentence: `git status --short` and `git rev-list --left-right --count @{u}...HEAD`.

### Where the work is

**Milestone 1's sixth slice — `oms` — has its CODE COMPLETE. What is left is the close.**

| Path | Holds |
|---|---|
| `internal/oms/domain/` | §5.3's twelve-state lifecycle + `CanTransition` (30 edges), the fill ledger, §5.1's order intent |
| `internal/oms/application/ports/` | `Journal`, `OrderPlacer`, `Ack`, `OrderWriteRepository` + `OrderRecord`, `errors.go` |
| `internal/oms/application/dispatch.go` | §6 step 6's crash boundary |
| `internal/oms/infrastructure/driven/` | `MemoryOrderWriteRepository`, §3.6 rule 6's subject |
| `internal/oms/module.go` | **NEW — the composition seam. `Deps{Journal, Placer}`, `Module{Dispatch}`** |
| `tools/lint/singlewriter/` | **FIXED this session — it was reporting `oms`'s own test suite** |

Reasoning with the rejected alternatives: decision record **§27**. §26 stays live for §26e's gate
scope and §26j's owed list; §25 for §25b's classifier seam; §24 for §24e's table and §24f's finding.

### The next work

**The slice close.** It owes **SIX markers AND the doc-set version bump** — see below. There is no
code left in the slice.

### Five things to know before you touch anything

1. **`Module` must never expose a type from `ports/` or `driven/`** (§27b). A field
   `Module.Orders` typed `ports.OrderWriteRepository` is the shape that makes the wiring look
   finished and is the one §3.6 rule 6 cannot survive: the gate reads **declared import edges**,
   Go's interfaces are structural, so a root handed that value passes write authority to a context
   that needs **no import at all** to accept it — and every diagnostic disappears. When the
   repository is wired, it is wired **into a use case** here, and the use case is what `Module`
   exposes.

2. **The write repository is deliberately NOT wired** (§27c). Nothing consumes it: `ApplyPlanned`
   belongs to §6 step 5's planning use case, `ApplyState`/`ApplyFill` to the stream handlers, and
   `Dispatch` carries no OCC version. Constructing it into an unread field was refused on
   `driven/doc.go`'s own argument — a shape that looks compliant while asserting nothing.

3. **`Deps.Journal` is temporary and `Deps.Placer` is not** (§27d). The placer's adapter is a venue
   client outside this context (§20a); the journal's belongs in `oms/infrastructure/driven/` and
   just is not written. When it lands, **`singlewriter` forces the correction** — the adapter sits
   in a policed package, so no root can name its constructor, and the field becomes the journal's
   *configuration*.

4. **`grep -rn "SPEC EDIT OWED" internal/ tools/` now returns SIX.** Three in `transition.go`, one
   in `fill.go`, one in `oms/infrastructure/driven/doc.go`, and a new one in
   `tools/lint/singlewriter/singlewriter.go`: **§3.6 is silent about external test packages**
   (§27h). An external test package is the one Go construct where a package's import path and its
   directory disagree, and two gates have now answered it independently — `hexboundary` polices
   them, `singlewriter` exempts the owner's — both correct, neither derived from a stated rule.

5. **The gate fix is worth reading before writing any analyzer** (§27e/§27f). `singlewriter`
   reported `oms/module_test.go` because Go suffixes the **last segment**, so `internal/oms_test`
   matches neither of `within()`'s arms — and every *other* external test package in the repo sits
   below the exempt path and was covered by accident. **The fix already existed in `hexboundary`
   and had not been carried across.** Then the shared helper's `LastIndex` split turned out to be
   an **equivalent mutant** (it is exactly `strings.TrimSuffix`, verified against an oracle over 21
   paths) whose comment credited it with a protection `TrimSuffix` gives for free.

### Still the user's call, carried forward from session 22

**§3.6 rule 2 is silently unmet in `marketdata`, on both halves** (§24f) — ports flat in
`application/ports.go`, and no `var _` assertions in `infrastructure/driven/`, in a **closed**
slice against decision record §7. `oms` follows the rule on both halves, so the inconsistency
between the two contexts is visible rather than theoretical.

**Still owed from session 20 and untouched since:** the `venue-profiles.md` verification of the two
venues' **depth update-ID shapes**, which `venue.BookDelta`'s span design cites.

---

## START HERE (updated end of session 24, 2026-07-27, HISTORICAL)

**Branch `feat/milestone-1-foundation`, tree clean, `make ci` green.** Verify rather than trust
this sentence: `git status --short` and `git rev-list --left-right --count @{u}...HEAD`.

### Where the work is

**Milestone 1's sixth slice — `oms` — is still OPEN**, and what is left in it is `module.go` and
the close. What exists now:

| Path | Holds |
|---|---|
| `internal/oms/domain/` | §5.3's twelve-state lifecycle + `CanTransition` (30 edges), the fill ledger, §5.1's order intent |
| `internal/oms/application/ports/` | `Journal`, `OrderPlacer`, `Ack`, **`OrderWriteRepository` + `OrderRecord`**, **`errors.go`** |
| `internal/oms/application/dispatch.go` | §6 step 6's crash boundary |
| `internal/oms/infrastructure/driven/` | **NEW — `MemoryOrderWriteRepository`**, §3.6 rule 6's subject, with rule 2's `var _` assertion (the first in the repo) |
| `tools/lint/singlewriter/` | **NEW — the last deferred gate, now enforced** |

Reasoning with the rejected alternatives: decision record **§26**. §25 stays live for §25b's
classifier seam and §25c's diagnosed survivor; §24 for §24e's table and §24f's finding.

### The next work

**`internal/oms/module.go`, then the slice closes.** The composition seam has a real decision in
it this time and `marketdata/module.go` is the precedent: the context constructs its own adapters,
and a root supplies only what varies between binaries. Note that `singlewriter` now polices `app/`
roots, so anything a root would have to *name* from `ports` or `driven` is a signal to move the
type rather than to widen the gate (§26e).

**The slice close owes five markers AND a doc-set version bump** — see below.

### Four things to know before you touch anything

1. **`singlewriter` is live and it polices `application/ports` as well as `infrastructure/driven`.**
   By position, not by naming a type, so a `TradeWriteRepository` added to `ports/` is covered the
   day it is declared. `oms/domain` is deliberately outside it (a projection naming the state enum
   is §5.1's pattern working) and so is `oms/application` (rule 6 forbids *bypassing* the write
   path, not using it). §26e has all three decisions with their failure directions.

2. **The doc set is v0.9.10 while §3.6's text has moved, deliberately** (§26j). Rule 6 and the
   gate-status paragraph were rewritten when the gate landed, because leaving the core saying
   "remains deferred" while it runs in CI is a live contradiction. The version marks a **slice
   close**; the bump is owed at that close, along with the markers. `build-spec.md`'s status line
   says this in its first sentence.

3. **`grep -rn "SPEC EDIT OWED" internal/ tools/` now returns FIVE.** Three in `transition.go`, one
   in `fill.go`, and a new one in `oms/infrastructure/driven/doc.go`: §3.6 rule 3's "contains
   **exactly** driven/ + driving/" should read as a permitted-children rule, or a context with
   adapters in one direction has to create an empty package in the other.

4. **The write repository's mutex claim is hand-checked, not in the mutation output** (§26i).
   `mutate.sh` is line-scoped and removing `Load`'s `Lock`/`defer Unlock` takes two lines. The
   check: delete both, `TestConcurrentWritesAndReadsAreFreeOfDataRaces` reports three `DATA RACE`s
   and fails; restore, and it is green.

### Still the user's call, carried forward from session 22

**§3.6 rule 2 is silently unmet in `marketdata`, on both halves** (§24f) — ports flat in
`application/ports.go`, and no `var _` assertions in `infrastructure/driven/`, in a **closed**
slice against decision record §7. It now has an in-repo counterexample: `oms` follows the rule on
both halves, so the inconsistency between the two contexts is visible rather than theoretical.

**Still owed from session 20 and untouched since:** the `venue-profiles.md` verification of the two
venues' **depth update-ID shapes**, which `venue.BookDelta`'s span design cites.

---

## START HERE (updated end of session 23, 2026-07-27, HISTORICAL)

**Branch `feat/milestone-1-foundation`, tree clean, `make ci` green.** Verify rather than trust
this sentence: `git status --short` and `git rev-list --left-right --count @{u}...HEAD`.

### Where the work is

**Milestone 1's sixth slice — `oms` — is still OPEN.** What exists now:

| Path | Holds |
|---|---|
| `internal/oms/domain/` | §5.3's twelve-state lifecycle + `CanTransition` (30 edges), the fill ledger, §5.1's order intent |
| `internal/oms/application/ports/` | `Journal`, `OrderPlacer`, `Ack` |
| `internal/oms/application/dispatch.go` | **NEW — §6 step 6's crash boundary.** `Dispatcher`, `Outcome`, `Dispatch`, `admit`, `classifyPlace`, `classifyEffect` |
| `internal/oms/application/errors.go` | the refusals the dispatch path makes |

Reasoning with the rejected alternatives: decision record **§25**. §24 stays live for §24e's
classification table and §24f's `marketdata` finding; §23 is history.

### The next work

**`internal/oms/infrastructure/driven/` with the write repository — and `singlewriter` lands in
that SAME session**, not the one after. Rule 6's subject is a port plus an adapter; the moment the
repository exists the deferral expires, and both other deferred gates landed the session their
subject appeared rather than after the code they police. Then `module.go`, then the slice closes.

The `var _ ports.X = (*Impl)(nil)` assertions §3.6 rule 2 requires go in with the adapters. Note
that `marketdata` has none (§24f), so there is no in-repo example to copy — write them from the
rule's own wording.

### Three things to know before you touch anything

1. **`classifyPlace` is split into `classifyEffect(Effect, Class)`, and §25b is why.** It reads like
   a gratuitous helper and is not: two of `Effect`'s four members cannot reach the switch through
   `apperror`'s public API, because `apperror.New` degrades any invalid `Spec` to `EffectUnknown`.
   Through an `error` argument alone, deleting `EffectUnspecified` from its case group changes
   nothing any test can observe. **Do not inline it back.**

2. **There is one SURVIVING mutant on `dispatch.go`, and it is diagnosed** (§25c) — dropping
   `ErrEffectUnhandled` from `Outcome.Cause`'s join. Equivalent by construction: no
   `ports.OrderPlacer` error can carry an undeclared `Effect`. Its compensating control is
   `exhaustiveswitch`, verified to bite. The diagnosis is in the `Outcome.Cause` doc comment too, so
   a future mutation run finds it at the survivor. **It is not an unexamined gap.**

3. **FOUR markers are owed, unchanged from session 22, and that is correct mid-slice.**
   `grep -rn "SPEC EDIT OWED" internal/ tools/` returns 3 × `transition.go` + 1 × `fill.go`. The doc
   set is **still v0.9.10** and correctly so: nothing has landed in the spec yet. The four are
   listed under session 22's START HERE below, stated so they can be landed without re-deriving.

### Still the user's call, carried forward from session 22

**§3.6 rule 2 is silently unmet in `marketdata`, on both halves** (§24f) — ports flat in
`application/ports.go` rather than `application/ports/`, and no `var _` assertions in
`infrastructure/driven/`, in a **closed** slice against decision record §7. `oms` follows the rule
as written, so the inconsistency between the two contexts is itself the flag. Third demonstration of
the class the OWED grep cannot see.

**Still owed from session 20 and untouched since:** the `venue-profiles.md` verification of the two
venues' **depth update-ID shapes**, which `venue.BookDelta`'s span design cites.

---

## START HERE (updated end of session 22, 2026-07-27, HISTORICAL)

**Branch `feat/milestone-1-foundation`, tree clean, `make ci` green.** Verify rather than trust
this sentence: `git status --short` and `git rev-list --left-right --count @{u}...HEAD`.

### Where the work is

**Milestone 1's sixth slice — `oms` — is still OPEN.** What exists now:

| Path | Holds |
|---|---|
| `internal/oms/domain/` | §5.3's twelve-state lifecycle + `CanTransition` (**30 edges**), the fill ledger, and now §5.1's order intent — `Side`, `OrderType`, `TimeInForce`, `Order` |
| `internal/oms/application/ports/` | `Journal`, `OrderPlacer`, `Ack` — the driven ports, in `application/ports/` as §3.6 rule 2 wrote it |
| `internal/oms/application/errors.go` | the refusals the dispatch path makes |

Reasoning with the rejected alternatives: decision record **§24**. §23 is now history.

### The next work, and it is already designed

**Build the dispatch use case — §6 step 6's crash-boundary sequence — from decision record §24e.**
That section is not a summary; it is the design taken this session, written out with the rejected
alternatives, so the next session **implements rather than re-derives**. It gives the exact
signature and why, the seven-step sequence, `admit`'s three checks, the full `Effect` × `Class` →
`OrderState` classification table with the argument for its one contested row, the double-dispatch
guard, and the fixture-collapse checklist to run **before** the mutations.

The two things in §24e most likely to be got wrong if skimmed:

1. **`Dispatch` returns `(Outcome, error)`, not `(OrderState, error)`.** The rejected signature is
   the point: with it, the returned state is authoritative *even when the error is non-nil*, and the
   universal Go reflex `if err != nil { return }` then silently drops an order that may be live at a
   venue. Splitting them makes the mistake inexpressible — `err != nil` means **nothing was sent**.
2. **`EffectNotApplied` does not mean `Rejected`** unless the class is `ClassVenueReject`. `Effect`
   answers "did the side effect happen", not "which evidence class proves it didn't", and FLAG-2
   makes SM §1's two absence terminals differ by exactly that. Everything else goes to
   `SubmissionUnknown` and costs one query.

Then `infrastructure/driven` with the write repository, **and `singlewriter` lands in that same
session** rather than after the code it polices. Then `module.go`, then the slice closes.

### Three things to know before you touch anything

1. **`singlewriter`'s deferral has STILL not expired.** Its subject is `oms`'s **write repository** —
   a port plus an adapter. `ports.Journal` is a write-ahead **log**, not rule 6's "OMS write
   repositories", and `infrastructure/driven/` does not exist. Landing the gate now is a lint over a
   tree with nothing to police, which is what the deferral was written to prevent.

2. **FOUR markers are owed, and that is correct mid-slice.**
   `grep -rn "SPEC EDIT OWED" internal/ tools/` returns 3 × `transition.go` + 1 × `fill.go`. The doc
   set is **still v0.9.10** and correctly so: nothing has landed in the spec yet.

3. **Do not "simplify" `ContributesPossibleDelta` to `!IsTerminal()`** (§23c), and do not read
   `return false, s.Validate()` as the fix for the trailing-return trap (§23d).

### The four owed edits, stated so they can be landed without re-deriving

All four are **silences or stale field lists — none is a disagreement with the core.**

- **SM §1, `DispatchAuthorized` row — the five query-resolution edges** (`Acknowledged` /
  `PartiallyFilled` / `Filled` / `Rejected` / `AbsentAfterFinality`), same set as
  `SubmissionUnknown`'s. Cost of the silence: the hedge group is permanently `Ambiguous`, exposure
  open, new trading held, nothing erroring. (§23b)
- **SM §1, `DispatchAuthorized` row — `→ SubmissionUnknown`** on an ambiguous transport result,
  distinct from the "no-return / **crash**" row. **This is the one the core states aloud** (§6 step 6)
  while SM §1 tabulates no path to it, and the two paths the table does offer each record something
  untrue — `Submitted` claims a transport result a timeout never produced, into the log §7.5 replays;
  "stays" claims a send still in flight after the call returned. FLAG-1 already created the
  symmetric edge one step later. (§24b)
- **SM §1 — a NEW venue-initiated termination row.** `Acknowledged` / `PartiallyFilled` → `Canceled`
  on IOC expiry, GTD expiry, venue cancel-all or self-trade prevention, distinct from the FLAG-1
  rows. Without it the only order type this system sends can never terminate on its ordinary
  outcome. (§23b)
- **Core §5.1, the `Fill` field list.** Drop `FeeAsset`; `Fee Money` carries the asset tag. (§23i)

### A finding this session raised that is the USER'S call, not the next session's

**§3.6 rule 2 is silently unmet in `marketdata`, on both halves** (§24f). The rule reads "Ports in
`application/ports/` … adapters live in `infrastructure/driven/` with a compile-time
`var _ ports.X = (*Impl)(nil)`". `marketdata` declares its ports flat in `application/ports.go`, and
`grep -rn "var _ " internal/marketdata/infrastructure/` returns **nothing** — in a **closed** slice,
against a decision record (§7) that had already ruled *for* the subdirectory.

**Third demonstration of the class the OWED grep cannot see**, after v0.9.8's `AcceptOrdinal` and
v0.9.10's missing `StreamOrders`. `oms` follows rule 2 as written, so the inconsistency between the
two contexts is itself the flag. Fixing `marketdata` is a real refactor of a closed slice and was
deliberately not absorbed into this one. The `var _` assertions are separately cheap.

**Still owed from session 20 and untouched since:** the `venue-profiles.md` verification of the two
venues' **depth update-ID shapes**, which `venue.BookDelta`'s span design cites.

---

## START HERE (updated end of session 21, 2026-07-27, HISTORICAL)

**Branch `feat/milestone-1-foundation`, tree clean, `make ci` green.** Verify rather than trust
this sentence: `git status --short` and `git rev-list --left-right --count @{u}...HEAD`. The head
commit is not named here on purpose — a paragraph citing its own hash is stale the moment the
commit containing it is written.

### Where the work is

**Milestone 1's sixth slice — `oms` — is OPEN, and this is the first session on it.** What exists
is `internal/oms/domain/` and nothing else: §5.3's twelve-state order lifecycle (`OrderState`,
`CanTransition` over SM §1's edge set) and the cumulative-execution fill ledger (`Fill`, `FillKey`,
`Ledger`). Two commits. **22 mutations, 22 caught** after one diagnosed survivor was fixed.

Reasoning with the rejected alternatives: decision record **§23**. §22 is now history.

### Three things to know before you touch anything

1. **`singlewriter`'s deferral has NOT expired, and do not land the gate yet.** The deferral is
   scoped to *"until `oms`'s **write repository** exists"*, and a repository is a port with an
   adapter — `oms/application/ports/` and `oms/infrastructure/driven/`, neither of which exists.
   A `domain/` package is not its subject. Landing the analyzer now means a lint over a tree with
   nothing to police, which is the exact thing the deferral was written to prevent, and a clean run
   would be indistinguishable from a broken one. It expires **the session the repository appears**,
   which on current shape is the next one.

2. **THREE markers are owed, and that is correct mid-slice** — they land at slice close, not when
   taken. `grep -rn "SPEC EDIT OWED" internal/ tools/` returns 2 × `transition.go` + 1 × `fill.go`.
   The doc set is **still v0.9.10** and correctly so: nothing has been landed into the spec yet.

3. **Do not "simplify" `ContributesPossibleDelta` to `!IsTerminal()`.** It is the one predicate in
   the package whose correctness is invisible to type-checking, its doc comment argues the case at
   length, and a test asserts the two predicates disagree on **exactly one** state. See §23c.

### The three owed edits, stated so the next session can land them without re-deriving

All three are **silences or stale field lists — none is a disagreement with the core.**

- **SM §1 transitions table, `DispatchAuthorized` row.** Add the five query-resolution edges
  (`Acknowledged` / `PartiallyFilled` / `Filled` / `Rejected` / `AbsentAfterFinality`), the same set
  `SubmissionUnknown` has. Core §5.3's rules paragraph, SM §1's own crash-recovery invariant and
  FLAG-2's ruling **all three** already require it; the table has the resolution in its *action*
  column and its target column pointing back at itself, so read literally a crash-recovered order
  can never leave the state. Cost of the silence: the hedge group is permanently `Ambiguous`,
  exposure open, new trading held, nothing erroring.
- **SM §1 transitions table, a NEW venue-initiated termination row.** `Acknowledged` /
  `PartiallyFilled` → `Canceled` on IOC expiry, GTD expiry, venue cancel-all or self-trade
  prevention — **distinct from the FLAG-1 rows**, which are about a cancel *we* requested. §6 step 6
  makes marketable-IOC the only order type sent, and SM §2 step 1 already says "an IOC is
  immediately terminal at the venue". Without it the only order type this system sends can never
  terminate on its ordinary outcome.
- **Core §5.1, the `Fill` field list.** Drop `FeeAsset`; `Fee Money` carries the asset tag already.
  The list predates the money kernel.

### What comes next, in the order that keeps the gate honest

`oms/application/` — the ports and the use case that drives the lifecycle — then
`infrastructure/driven` with the write repository, **and `singlewriter` lands in that same session**
rather than after the code it polices. Then `module.go`, then the slice closes: the three markers
above land in the spec, all five stamps and the Georgian header re-sync, `tutorial.ka.md`
re-assembles, ADR §3 gains the seventh milestone-1 block.

**Still owed from session 20 and untouched here:** the `venue-profiles.md` verification of the two
venues' **depth update-ID shapes**, which `venue.BookDelta`'s span design cites. Not a correctness
risk — see the session-20 section below for why, and why it decides whether `SeqRelationOverlap` is
reachable in production at all.

---

## START HERE (updated end of session 20, 2026-07-27, HISTORICAL)

**Branch `feat/milestone-1-foundation`, tree clean, `make ci` green on Go 1.26.5.** Verify rather
than trust this sentence: `git rev-list --left-right --count @{u}...HEAD`. The head commit is not
named here on purpose: a paragraph citing its own hash is stale the moment the commit containing it
is written.

### Where the work is

**Milestone 1's fifth slice — `marketdata`'s ORDER-BOOK path — is CLOSED.** `Module` now exposes
`Funding` **and** `Book`; the venue seam gained `BookSnapshot`/`BookDelta`; `platform/events` gained
the `StreamOrders` constant it had been missing. Doc set at **v0.9.10** across all five stamps plus
the Georgian header, `tutorial.ka.md` re-assembled, ADR §3 carries the **sixth** milestone-1 block,
`grep -rn "SPEC EDIT OWED" internal/ tools/` — **both paths** — returns **zero**, `docs` 102 pass
(`cd docs && bun test`, NOT `node --test`). Reasoning: decision record **§22**; **§21** is history.

**ONE thing is owed, and it is a Profiles verification rather than a spec edit** — which is exactly
why the slice-close grep says clean and this paragraph exists. `venue.BookDelta`'s design cites an
asymmetry between the two depth feeds: *Binance's depth events carry a first and a final update ID
and may cover several venue sequences per message; Bybit's carry one.* `venue-profiles.md` §1
records the book **topics** (`@depth…` + init snapshot from `GET /api/v3/depth`; `orderbook.50` with
delete-on-size-0, reset-on-snapshot) and records **neither feed's update-ID shape**. Verify both
against primary docs, date them, and add them to §1.

**It is not a correctness risk, and knowing why matters before you prioritise it.** The span is
`FirstSeq..LastSeq` *inclusive*, and a single-sequence venue sets First equal to the envelope's
`VenueSeq` — the span of length one. So the code is right whichever venue turns out to do which; the
fact only decides whether the multi-sequence case is ever exercised in production. What is actually
at stake is that `SeqRelationOverlap` is **unreachable for a single-sequence venue**, so if BOTH
venues turn out to be single-sequence, one of the four relations is dead code in live trading and
its tests are the only thing keeping it honest.

**The next slice is `oms`**, and it is no longer a choice between two: the book path was the other
candidate and is now done. It **expires `singlewriter`'s deferral** — the last unbuilt gate, roster
6 of 6 — because that deferral was scoped to "until `oms`'s write repository exists", and this
repo's rule is that a deferral expires when its subject appears. `StreamOrders` now exists for it.

### Three things from this slice that will bite if you do not know them

1. **`SURVIVED` is a question, and two survivors in one session needed OPPOSITE answers.** One was a
   **third copy of one check** — `toDomainLevels` validated each level as it copied it, so bypassing
   the venue-seam check entirely still produced the venue sentinel and no test could tell the two
   apart. "Write a better test" was the wrong response; "the helper is doing two jobs" was the right
   one, and it now cannot fail. The other was a genuine **test gap**. Diagnosing which is the work,
   and a session that assumes both are the same kind fixes one of them wrongly.
2. **Naming the marker grep's pattern inside `internal/` BREAKS the gate.** A comment explaining
   that the grep cannot find unmarked defects made the grep match forever. A gate whose pattern is a
   string is a gate the prose can trip — reword rather than accept a permanently dirty gate.
3. **`Watermark.VenueSeq` is optional by design, and a BOOK is the case that exemption does not
   reach.** A REST-polled funding read genuinely has no sequence, so `Watermark.Validate` admits
   zero. A book admitted at position zero validates, is marked **synced**, and then makes every
   later delta unplaceable forever — a resync loop reporting a connected feed, a valid book and a
   healthy transport. `domain.ErrBookSeqUnset` is the refusal. **The general shape: an exemption
   written for one data class silently covers every other one, and the place to look is wherever a
   field's doc comment says "not validated, because <one specific caller>".**

### Still true, still deferred, still deliberate

- **Mark normalization is unwired** — `Module` has `Funding` and `Book` and no `Mark`. There is no
  `venue.Mark` and no use case, so there is nothing to compose; the deferral is visible in the type,
  and a caller cannot reach a path that is not assembled.
- **`Deps.Resnapshotter` is the one PORT a root supplies**, and that asymmetry is load-bearing:
  every other driven port has exactly one adapter inside this context, while a resnapshot is a venue
  REST call and §20a defers those clients. Whatever a root wires there **must be idempotent** — the
  use case requests on every held delta, deliberately, because a single dropped request leaves the
  book held forever with nothing retrying.
- **`Deps.BookHoldDepth` is restart-required, not `config:"hot"`**, and that is a claim rather than
  an omission: shrinking the depth mid-resync discards held deltas the in-flight snapshot was going
  to chain to, converting a recovering book into the loop the buffer exists to prevent — at exactly
  the moment an operator is reaching for a knob.
- **`observability`'s Prometheus adapter and tracing** — unchanged; see the session-19 section.
- **No raw Binance/Bybit clients and none of §4's nine capability interfaces** — unchanged (§20a).

---

## START HERE (updated end of session 19, 2026-07-27, HISTORICAL)

**Branch `feat/milestone-1-foundation`, tree clean, `make ci` green on Go 1.26.5.** Verify rather
than trust this sentence: `git rev-list --left-right --count @{u}...HEAD` should print `0 0`. No PR —
deliberate. The head commit is not named here on purpose: a paragraph citing its own hash is stale
the moment the commit containing it is written.

### Where the work is

**Milestone 1's fourth slice — `engine/venue` — is CLOSED, and NOTHING IS OWED.** The doc set is at
**v0.9.9** across all five stamps plus the Georgian header; `tutorial.ka.md` is re-assembled from its
fragments; ADR §3 carries the **fifth** milestone-1 block; and
`grep -rn "SPEC EDIT OWED" internal/ tools/` — **both paths** — returns **zero**. `docs` 102 pass.
The reasoning is decision record **§21**; **§20** is the slice it closes and is now history.

**The next work is a new slice, and the choice is yours.** Two candidates, and they are not
equivalent in what they unlock:

| Candidate | What it buys | What it costs |
|---|---|---|
| **`marketdata`'s book path** | The §3.4 machinery the funding path never needed — reconstruction, gap detection on `VenueSeq`, resnapshot, and the coalescing rule that is *deterministic and keyed on `(Source, field)`*. It is the first code that exercises `StreamBook`, so it is the first that could disagree with the seam §3.5 just resolved | Stays inside a context that is already built; adds no new gate |
| **`oms`** | **Expires `singlewriter`'s deferral** — the last unbuilt gate, roster 6 of 6 — because the deferral was explicitly scoped to "until `oms`'s write repository exists", and this repo's rule is that a deferral expires when its subject appears, not after the code it polices | A bigger slice: §5.3's identity, states and commands, plus the crash-boundary journal discipline |

### Three things from this close that will bite if you do not know them

1. **A marker names where a defect was SEEN, never where its fix reaches.** The `DataSource`
   resolution was marked against the *declaration* in §3.5, because that is where the contradiction
   was visible — but the rename's real surface was **five sites**: §3.2's backtest row, §3.5's
   live-vs-backtest wiring sentence, §8's parity bullet and §11's repo tree all named the retired
   type. Renaming the declaration alone turns a resolution into a *second* ambiguity, since a reader
   greps the old name, finds four hits and no definition, and cannot tell rename from deletion. **Any
   future close: re-derive the surface, do not trust the marker's location.**
2. **Look for the citation before writing the third paragraph of argument.** `engine/venue/doc.go`
   argued §4's shape wins on three grounds and all three hold — but the thing that actually settled
   it was found during the close: **`venue-profiles.md` §3 was already writing
   `MarketDataSource.StreamMark`.** The doc set had adopted §4's name already. An argument says which
   shape *should* win; a citation says which one the set was *using*, and only the second is
   checkable by someone who disagrees with you.
3. **A contradiction and a silence are not the same finding, and only one is findable alone.** A gap
   is visible to a reader of one section; **a contradiction is visible only to a reader holding two
   sections at once**, and neither is visible to a grep. Session 17 found a rule the *code* silently
   broke; session 18 found rules the *spec* silently disagreed with itself about. Both came from the
   reading pass, which now has two independent cases behind it — so the pass is not a formality that
   sits beside the gate, it is the only thing covering two whole classes of defect.

### Still true, still deferred, still deliberate

- **`observability`'s Prometheus adapter and tracing** — the adapter lands with `app/` where the
  per-binary `/metrics` decision lives (§11 gives `arblab-actuator` no HTTP surface at all); tracing
  lands once §3.5's `DispatchSeq` exists to hang a span on. Both on argument (§12a, §12f).
- **No raw Binance/Bybit clients and none of §4's nine capability interfaces in `engine/venue`** —
  §20a. The dispatcher is not built, so a WS client would invent its own arbitration, which is the
  second-authority defect §3.5 forbids; and eight of the nine interfaces are consumed by contexts
  that do not exist. **The `MarketDataSource` resolution does not change this** — declaring the
  interface now is still the empty-tree case.
- **No `DefaultFundingInterval` constant, and its absence is now normative** — §4 states the
  refuse-rather-than-default rule as of v0.9.9, so the 8h default belongs to the Binance adapter,
  applied from an overlay that was fetched and came back empty.

---

## START HERE (updated end of session 18, 2026-07-27, HISTORICAL)

**Branch `feat/milestone-1-foundation`, tree clean, `make ci` green on Go 1.26.5.** Verify rather
than trust this sentence: `git rev-list --left-right --count @{u}...HEAD` should print `0 0`. No PR —
deliberate. The head commit is not named here on purpose: a paragraph citing its own hash is stale
the moment the commit containing it is written.

### Where the work is

**Milestone 1's fourth slice — `engine/venue` — is BUILT and NOT CLOSED.** The user chose it from
session 17's three candidates on the decay argument: `driving.FundingUpdate` was explicitly the
contract *until* `engine/venue` landed, and exactly one consumer held that placeholder. It is now
`venue.Funding`, and `driving.FundingUpdate` no longer exists. `domain/` and `application/` were
untouched by the move — §3.6 rule 1's promised property, now checked rather than predicted.

| Piece | State |
|---|---|
| `internal/engine/venue/{doc,funding,source,errors}.go` | **Done** — normalized funding DTO + the mailbox-key constructor |
| `marketdata` migrated onto it (`driving/`, `driven/`) | **Done** |
| §4's perp-only rule, pushed down to `platform/events` | **Done** — `StreamKind.RequiresPerp` + `SourceID.Validate` |
| **The slice CLOSE** | **OWED — this is the next work.** Full list: decision record **§20h** |

`grep -rn "SPEC EDIT OWED" internal/ tools/` returns **three**, and all three are *silences in the
core rather than disagreements with it* — the v0.9.6 shape:

1. **`engine/venue/doc.go`** — §3.5 and §4 declare the market-data seam **twice, with incompatible
   shapes**. §3.5: `DataSource interface { Subscribe(VenueInstrument) <-chan Envelope }`. §4:
   `StreamBook`/`StreamFunding`/`StreamMark`. Same position in the pipeline; a type satisfying one
   does not satisfy the other. §4 wins. **It is deliberately not resolved by declaring the
   interface** — that is the empty-tree case.
2. **`platform/events/envelope.go`** — §3.5's `SourceID` can name a source **no venue can produce**.
   §4 marks funding and mark perp-only; §3.5 never carries it across, so a spot funding source
   validated cleanly and was covered only by accident, three layers downstream, by `marketdata`'s
   own domain.
3. **`engine/venue/funding.go`** — §4 states a units discipline **for money and none for time**.
   Bybit's `fundingIntervalHour` multiplied by `time.Minute` satisfies the spec completely and hands
   §2.4 a settlement count wrong by 60×. This is §10's v0.9.7 argument, one layer earlier.

### Three things from this slice that will bite if you do not know them

1. **`venue.Funding` and `domain.FundingObservation` overlap ON PURPOSE, and neither is deletable.**
   §3.6 rule 1 forbids `domain/` from importing `engine/`, so the domain *cannot* be expressed in
   terms of the venue type. The consequence: the venue-side `Validate` is a strict subset of the
   domain's over the shared fields, so **no input exists that one refuses and the other accepts** —
   the only observable difference is which boundary the error names. That is what
   `TestDeliverJudgesTheVenueReadingBeforeTranslatingIt` pins, and without it the venue check is
   silently deletable.
2. **A trailing `return X` in a switch makes the `X` group's membership invisible to tests.** Any
   member dropped from it falls through to the same answer; only `exhaustiveswitch` catches it
   (verified by hand — `make lint` exits 3). Probed against **committed code from an earlier slice**
   to confirm it generalizes: dropping `StreamControl` from `Lane`'s control case **survives**,
   dropping `StreamFunding` from its market-data case is **caught**. The two gates are not redundant.
   Do not "fix" this by flipping a default — that trades a checkable claim for a dangerous one.
3. **There is no `DefaultFundingInterval` constant, and its absence is load-bearing.** Profiles §1
   gives Binance's "absent ⇒ 8h default", but a zero-value fallback answers three different
   situations identically and two are a wiring failure wearing a correct BTC reading's clothes. Zero
   is refused; the default belongs to the Binance adapter, applied from an overlay that was fetched
   and came back empty.

### The finding this slice contributes to method

**§19e's lesson recurred inside a single session, and the mutation harness is what caught it.** A
comment at `LanePublisher`'s new `venue.FundingSource` call claimed the constructor produced the
outer, better-named diagnosis for a spot instrument. Replacing the call with the literal it replaced
**survived** — because the rule lives on `events.SourceID.Validate`, both paths produce the identical
wrapped error. The comment was arguing for a property the code did not have. That is the good
outcome (the rule is enforced by *construction*, not by remembering the constructor) reported by a
false claim, and the fix was to say what the call actually buys.

---

## START HERE (updated end of session 17, 2026-07-27, HISTORICAL)

**Branch `feat/milestone-1-foundation`, tree clean, `make ci` green on Go 1.26.5, pushed and in
sync.** Verify rather than trust this sentence: `git rev-list --left-right --count @{u}...HEAD`
should print `0 0`. No PR — deliberate. The head commit is not named here on purpose: a paragraph
citing its own hash is stale the moment the commit containing it is written.

### Where the work is

**Milestone 1's third slice is CLOSED.** `marketdata` — the first bounded context, scoped to the
funding path — is built, its spec edits are landed as `build-spec.md` **v0.9.8**, and **nothing is
owed**. Verify with the slice-close gate itself: `grep -rn "SPEC EDIT OWED" internal/ tools/`
returns nothing. **Both paths** — the second was added the session normative claims first appeared
outside `internal/`.

| Piece | State |
|---|---|
| `internal/marketdata/{domain,application,infrastructure,module.go}` | **Done** — funding path complete end to end |
| `tools/lint/{hexboundary,exhaustiveswitch}` | **Done** — analyzer roster is **5 of 6** |
| The slice CLOSE | **Done** (session 17) — code fix, then v0.9.8, then the release checklist |

### What is next, and it is a decision rather than a queue

The funding path is complete; the **book and mark paths of `marketdata` are not built**, and
`marketdata.Module` has one field so that absence is visible in the type rather than as a nil to
check. Three candidates, and the choice has not been made:

- **`engine/venue`** — the normalized venue DTOs §4 assigns to it. `driving.FundingUpdate` is
  explicitly the contract *until then*, and when `engine/venue` lands the translation is one
  function in that package with `domain/` and `application/` untouched. That property is the
  reason to do it soon rather than late.
- **`marketdata`'s book path** — book reconstruction, sequence-gap detection and resnapshot
  (§3.4). It is what §3.5's causal-traceability rule was originally *written* about, so it
  exercises v0.9.8's edits at their intended target.
- **`oms`** — which is what **expires `singlewriter`'s deferral**: the last unbuilt gate waits on
  its write repository, and this project's standing rule is that such a deferral expires the moment
  its subject appears, not after the code it polices is written.

### The two findings from this slice worth carrying, because both are about method

**The `[SPEC EDIT OWED]` grep cannot find a rule the code silently broke.** It gates claims the code
*marked*; a violation is unmarked by definition, and this one shipped with a doc comment arguing the
rule did not apply. The reading pass beside the grep is what caught it. The generalizable form: **a
true rule stated without its scope becomes a false rule** — worth looking for wherever two fields of
one type sit behind one prohibition.

**A clean gate run proves nothing, and `hexboundary`'s obvious positive control does not work.**
Importing `infrastructure/` from `application/` is an **import cycle**, so the type-checker refuses
before the analyzer runs. Every infrastructure package in a context imports `application` or
`domain`, so *within one context Go's own no-cycles rule already covers most of §3.6 rule 1*. The
probe that works is a package under `infrastructure/` importing **only `domain`** — no cycle, and
the gate objects with exit 3. Where rule 1 earns its place is the edge Go cannot catch: `engine/`,
and another context's infrastructure.

---

## START HERE (updated end of session 16, 2026-07-27, HISTORICAL)

**Branch `feat/milestone-1-foundation`, tree clean, `make ci` green on Go 1.26.5, pushed and in
sync.** Verify rather than trust this sentence: `git rev-list --left-right --count @{u}...HEAD`
should print `0 0`. No PR — deliberate. The head commit is not named here on purpose: a paragraph
citing its own hash is stale the moment the commit containing it is written.

### Where the work is

Milestone 1's **third slice** is `marketdata`, the first bounded context, scoped to the
**funding-rate path only**. **All of its code is now built:**

| Piece | State |
|---|---|
| `internal/marketdata/domain/` | **Done** (session 14) |
| `tools/lint/{hexboundary,exhaustiveswitch}` | **Done** (session 15) — analyzer roster is **5 of 6** |
| `internal/marketdata/application/` | **Done** (session 15) |
| `internal/marketdata/infrastructure/{driven,driving}` | **Done** (session 16) |
| `internal/marketdata/module.go` | **Done** (session 16) |
| **The slice CLOSE** | **OWED — this is the next work, and the user approved running it in full** |

### Do this next — the order matters, and it does NOT start with a doc edit

The full owed list, with every decision and its rejected alternatives, is **decision record §18h**.
Summarised:

1. **First, a CODE fix (§18f).** The reading pass found a real **spec violation**, not an ambiguity.
   §3.5 (line 242) requires a dispatched envelope to carry the `AcceptOrdinal` of the raw input that
   triggered it — *causal traceability* — and `driven.LanePublisher` publishes with zero, so a
   published snapshot cannot be traced back to the input that produced it. The fix is designed in
   full (a named `application.AcceptOrdinal`, a `FundingSnapshot.Trigger`, `Ingest` refusing a zero,
   and **zero meaning timer-triggered** — because §3.4's silence timer produces the one envelope
   whose entire news is that *nothing arrived*, a case §3.5 has no rule for). It was started and
   **deliberately reverted** at a clean commit rather than left half-applied.
2. **Then the spec edits**, so the spec describes what the code does: five `[SPEC EDIT OWED]`
   markers plus the two answered questions below, as `build-spec.md` **v0.9.8**.
3. **Then the release checklist** (below): five stamps, ADR §3 block, marker flip, i18n re-assemble,
   `cd docs && bun test`, `make ci`.

**The exact edit sites are already located** and recorded in §18h, so they need not be re-found.

### The two spec questions are ANSWERED — do not re-ask them

The user decided both in session 16 (reasoning and rejected alternatives: decision record §18g).

- **`Age` vs `DataAge` → one budget, two measurements.** §3.4's per-venue budget is per *data
  class*, applied to both the transport age and the venue-side data age, reported separately and
  reduced by minimum. This is what the code already does.
- **§3.2 vs §3.3 on `MarketView` → §3.3 wins, fix §3.2.** `marketdata` publishes normalized,
  freshness-stamped observations; `engine/state` translates into the strategy-owned `MarketView`.

### The session-16 finding worth carrying, because it is about method

**A clean gate run proves nothing, and the obvious positive control for `hexboundary` does not
work.** Importing `infrastructure/` from `application/` is an **import cycle**, so the type-checker
refuses before the analyzer runs. Every infrastructure package in a context imports `application` or
`domain`, so *within one context Go's own no-cycles rule already covers most of rule 1*. The probe
that works is a package under `infrastructure/` importing **only `domain`** — no cycle, and the gate
objects with exit 3. Where rule 1 earns its place is the edge Go does not catch: `engine/`, and
another context's infrastructure.

---

## START HERE (updated end of session 15, 2026-07-27, HISTORICAL)

**Branch `feat/milestone-1-foundation`, PUSHED and in sync (`0 0`), tree clean, `make ci` green on
Go 1.26.5. No PR — deliberate; open one only if asked.** Verify rather than trust this sentence:
`git rev-list --left-right --count @{u}...HEAD`.

### Where the work is

Milestone 1's **third slice** is `marketdata`, the first bounded context, scoped to the
**funding-rate path only** (order-book reconstruction deferred — the user's call). Built so far:

| Piece | State |
|---|---|
| `internal/marketdata/domain/` | **Done** (session 14) — `Watermark`, `Freshness`+`Budget`, `FundingObservation` |
| `tools/lint/hexboundary` (§3.6 rule 1) | **Done** (session 15) |
| `tools/lint/exhaustiveswitch` (§3.6 rule 10) | **Done** (session 15) |
| `internal/marketdata/application/` | **Done** (session 15) — ports + the funding use case |
| `internal/marketdata/infrastructure/{driven,driving}` | **OWED — this is the next work** |
| `internal/marketdata/module.go` | **OWED** |

**The analyzer roster is 5 of 6.** Only `singlewriter` is deferred, and it is the one gate whose
subject genuinely does not exist yet (`oms`'s write repository).

### Do this next

1. **`marketdata/infrastructure/{driven,driving}` + `module.go`.** `driven/` implements
   `application.Budgets` (over `platform/config`) and `application.FundingPublisher`; `driving/` is
   the surface a dispatcher calls. **`hexboundary` is live**, so an import running inward-to-outward
   is a build failure now, not a review comment. `internal/engine/` still does not exist and the
   funding path does not need it — `engine/venue`'s `DataSource` is §3.5's seam, not this context's.
2. **Then close the slice** with the release checklist below.

### Four things that will bite if you do not know them

1. **The slice-close grep is `grep -rn "SPEC EDIT OWED" internal/ tools/` — BOTH paths.** Scoped to
   `internal/` alone it returns **0 while two markers exist** in `tools/lint/`. The analyzers make
   normative claims about §3.6, so code that owes spec edits now lives outside `internal/`. This is
   the third instance of a gate reporting clean because it cannot see its subject.
2. **`tools/mutate.sh` needs `MUTATE_MODULE=tools/lint` to reach the analyzers.** Without it,
   `go test ./tools/lint/...` from the root fails with *"main module does not contain package"* —
   non-zero, which the harness reads as a **kill**. Every analyzer bite-check would be a phantom.
   The harness also has a **sixth verdict, `PANIC!!`** (a mutant that compiled and crashed; a kill,
   but a weaker one — the suite saw a crash, not a wrong answer).
3. **Do NOT re-run the `golangci-lint exhaustive` evaluation.** It is done, with numbers, in
   `exhaustiveswitch`'s doc comment and decision record §17c. The verdict was **scope, not
   capability**, and one flag does not mean what its name suggests: `-package-scope-only` means
   "members declared at package scope", **not** "enums from this package", and had no effect.
4. **Two `[SPEC EDIT OWED]` markers are open**, both about §3.6's *own status text* —
   `hexboundary.go` (the "Gate status" paragraph still lists rule 1 as deferred and names three
   enforced) and `exhaustiveswitch.go` (rule 10 still says "planned, not yet enforced" and still
   carries the "evaluate `exhaustive` first" instruction, both discharged).

### Two spec questions this slice raised and deliberately left open

Decide them at slice close, in the reading pass — not before.

- **`Age` vs `DataAge`.** §3.4 says "per-venue max-age budget", **singular**, but a feed dies two
  ways and the venue-side stall (transport healthy, `SourceTime` frozen) is invisible to the check
  written first. The application layer confirmed both verdicts are needed. Does the spec owe a
  **second budget**, or is one budget applied to two measurements the right reading? The code does
  the latter.
- **§3.2 vs §3.3 on `MarketView`.** §3.2's marketdata row says this context publishes one; §3.3 says
  `MarketView` is strategy-owned and `engine/state` translates into it. Built to the consistent
  reading — this context publishes a `FundingSnapshot`.

### The release checklist, for when `infrastructure/` lands

`grep -rn "SPEC EDIT OWED" internal/ tools/` → land what it finds → **then a reading pass** over
what the slice asserts *normatively* that the spec does not (the grep cannot see an **unmarked**
claim, which is how four of them sat through a whole release with the gate green; ask of each: *could
a reimplementation get this wrong in the UNSAFE direction while still satisfying the core as
written?*) → bump **all five** doc-set stamps → ADR §3 block → flip code markers to
`[SPEC EDIT LANDED — v<version>]` → re-sync `i18n/ka/tutorial/header.md` and run
`node i18n/tools/assemble.mjs` → **`cd docs && bun test`** → `make ci`.

Worth asking in that reading pass: **rule 10's deferral named the wrong trigger.** It waited on a
*state machine*, while what actually made it buildable was the kernel's eleven enum-likes, which had
existed for several slices. `singlewriter` may be mis-triggered the same way.

### The finding from session 15 worth carrying, because it is about method

`domain.Budget.NextTransition` returned the boundary instant while `Budget.At` is **inclusive-fresh
by design** — so the silence timer fired at an instant where the verdict had *not* changed, and the
caller re-armed for that same instant. A **spin**: a timer firing forever while a dead feed stays
marked fresh, with the ladder frozen short of the rung that fires §5.2's `mark_blind_action`.

The domain suite had a test written to catch exactly this class of defect. It checks the verdict one
tick **before** the named instant and one tick **after** — and never **at** it, which is the only
instant that matters, because it is when the timer fires. **A bracket around a boundary is not a
test of the boundary.** It was invisible from inside `domain/`: `At` and `NextTransition` are each
self-consistent and only their *composition* is wrong. The second implementation that disagreed was
not an oracle — it was the layer above, which is the strongest argument yet for the layering itself.

**Full detail, every decision and its rejected alternatives: decision record §17, and the
`RESUME HERE` block below it (it carries a session-15 header at its top).**

---

## NEXT REQUEST — session 14 (2026-07-27, HISTORICAL): **the doc set is at v0.9.7; next is `marketdata`, the first bounded context**

**Both scope calls the previous brief left open were put to the user and answered (session 14).**

**(1) §15b's four unmarked normative statements are LANDED, as a doc-only `build-spec.md` v0.9.7** — the outbox's sequence-as-dedup-key and committed-bytes payload (§3.2 + a D2 pointer), reconcile-only as a four-rung **posture** rather than a boot stage (§7.5), the rate limiter's **upward-only** header correction and retry-hint-as-**floor** (§7.6), and the **integer-typed** recording seam with unit and asset tags (§10). All five doc-set stamps are at v0.9.7, ADR §3 carries a **third** milestone-1 block (tagless total now **twenty-two**), and the four code sites carry `[SPEC EDIT LANDED — v0.9.7]` markers. **No formal-model change; `docs/formal/` is untouched and still at the Cx8 verdict.** Full reasoning: decision record **§16**. **Nothing is owed.**

**What reversed the original verdict is worth carrying forward, because it is a gap in the release gate itself.** Each of the four had been judged an *implementation of* an existing requirement, so none carried a marker — and v0.9.6's gate was the marker count. Re-testing needed a sharper question than "is this in the spec?", whose honest answer for all four was "sort of, by implication": **could a reimplementation get this wrong in the *unsafe* direction while still satisfying the core as written?** All four fail it, and not narrowly — D2 says *idempotent consumers* and never says idempotent **on what**, so content-hash dedup is a natural reading that silently drops the second of two identical funding accruals, surfacing as a missing receipt in the §10 tax records. **So `grep OWED` returning zero proves nothing was *flagged*; it does not prove code and spec *agree*** — this project's vacuous-coverage lesson, arriving in the release process. The checklist now carries a slice-close **reading pass** alongside the grep, kept separate from it because they answer different questions (§16b). And **§15c's item worked this time**: the `ka` header fragment was re-synced and re-assembled *during* the release, `cd docs && bun test` reports **102 pass / 0 fail**.

**(2) The first bounded context is `marketdata` — the user's call.** It makes **`hexboundary` and `exhaustiveswitch`** buildable for the first time (both deferred on *a lint over an empty tree passes trivially, which is indistinguishable from a broken lint*). **`singlewriter` stays deferred** — it is the one gate whose subject genuinely does not exist yet, so two of the three wake here rather than three. **Why not `oms`, recorded so it is not silently reopened:** §3.2 makes `oms` the single-writer authority and it is the context all three gates are written *about*, but it needs the journal — and `platform/outbox`'s store half is deferred to milestone 3, so `oms` first means either pulling milestone-3 work forward or building against a **stubbed write repository**, which is exactly the hazard §14h rejected for the `Reconciler` port: *a nil-returning implementation reports every gate passed with nothing joined, and the boot is green*. The decisive argument is the kill gate — **milestone 2's verdict is measured net carry, and you cannot measure carry without funding-rate data while you can measure it without ever placing an order.** `marketdata` is on that critical path; `oms` is not.

---

### The slice as built (history below is unchanged)

**Branch `feat/milestone-1-foundation`, `make ci` green on Go 1.26.5.** **Milestone 1's first slice (§1) is COMPLETE** — the scaffold, `platform/fixedpoint`, `platform/clock`, the `kernelpurity` / `nofloat` / `notimenow` gates, the ten `[SPEC EDIT OWED]` items landed as build-spec v0.9.5, and the `gremlins` audit run and closed out.

**The second slice is now eight packages in: `platform/apperror` and `platform/ids` (session 6), then `ids`'s instrument identity and `platform/events` (session 7), then `platform/config` (session 8), then `platform/outbox` (session 9), then `platform/observability` (session 10), then `platform/ratelimit` (session 11), then `platform/boot` (session 12).** The shared kernel is **ten packages** deep, **`boot` was the last entry in §1's deferred table**, and **session 13 closed the slice** by landing its eight spec edits as v0.9.6.

- **`platform/boot`** — §3.2's bootstrap phases and §7.5's reconcile-only boot. It imports `platform/clock` and nothing else. Both of its shaping decisions were about a **type signature** rather than an algorithm.
- **Reconcile-only is a POSTURE, not a stage, and the obvious shape loses it in the signature.** §7.5 reads as a sequence, so the obvious implementation is a list of `func() error` steps — which has two outcomes where §7.5 has **three**. The third is *alive and refusing*, stated three separate times and never named: foreign orders and unexplained position deltas "alert for operator clearance" and are "not auto-corrected"; an `Ambiguous` MarginOp blocks openings "until the staged-float ledger is explicitly reconciled"; and §7.4 keeps a bot that recovered mid-takeover in reconcile-only "until an operator clears it". Collapsing it costs money **both ways** — folded into failure the process exits, so §7.4's actuator has to cover a host that died *deliberately*; folded into success, trading resumes over an unexplained position delta. So `Run` returns a four-rung `Posture` whose **zero value is the most restrictive**, and the default-deny is structural: `Checklist.Posture` reduces over the gates that were **required**, not the ones that were reported.
- **§7.5's gates latch or they don't, and the spec never says which** `[SPEC EDIT OWED]`. One paragraph gives every precondition the same verb while stating two that clear differently: a MarginOp resolves through its evidence contract, a foreign order needs an operator. §7.4 then spells the second kind out for the fence — clearance **and a fresh propagation test**, "never health-flap re-entry". A uniform checklist would let a check that failed and then passed on retry re-admit trading, which is **§7.4's forbidden re-entry arriving through §7.5's door**. `Clear` therefore returns a gate to *unreported*, never to passed: clearance restores the ability to pass, not the pass.
- **§3.2's phase list is a set of constraints, and both load-bearing ones fail silently** `[SPEC EDIT OWED]`. Signal is first because a Go process's default disposition for SIGINT/SIGTERM is to **die** — installed after a forty-second journal join, an operator's interrupt during that join kills the process with no flush, and the window where a boot is slowest is the window where somebody interrupts it. Observability precedes anything that can fail because every later phase reports through it — started late, a boot that dies in the journal join dies **silently**. Enforced as constraints with an open `KindService` slot (the user's call; a closed five-value order was rejected because §11's binaries have different phase sets).
- **The shutdown budget is a safety number** `[SPEC EDIT OWED]`. §7.4 gives the watchdog a host-silence trigger, §7.5 gives a graceful shutdown, and nothing joins them — but a total stop budget at or above the threshold makes an **ordinary restart indistinguishable from a dead host**: the actuator fences the signer mid-flush, latches an epoch, and the process comes back up fenced and stuck in reconcile-only until an operator clears it. A deployment becomes an operator-cleared incident, caused by a sum of durations nothing was adding up. `Validate` adds them up and refuses at `>=`, because at exactly the threshold the watchdog fires. The bound is **allotted and accounted for, not enforced** — no goroutines, so no closer can be interrupted — and the enforcement it lacks is the operator's second interrupt. Which is safe for a reason worth stating: **host death is a covered state** (that is what the actuator is for) and a process wedged in shutdown is **not**, because the watchdog reads liveness.
- **62 mutations, 62 caught.** Four findings from the exercise. Two `TIMEOUT!!`s were a **real test defect** — breaking the signal subscription made the test *hang* rather than fail, so both claims about `Install` actually subscribing were unchecked while looking covered (§5e's lesson, second time; the receive is now bounded). One `SURVIVED` was an unreachable-but-undeletable compiler-required return, pinned by an internal test. Five `NOBUILD!!`s were all mine, every one an `if false` that orphaned the variable the condition read. And the **fixture-dimension checklist was applied in advance for the first time**, changing five fixtures before a mutation ran — cheapest of them, a reverse-teardown test with three closers, where reversing and swapping the ends are the same order.

- **`platform/ratelimit`** — §7.6's header-aware governors, plus the reserved allocation §6.7, §7.1 and §7.5 spend while recovering rather than trading. **Machinery, not a venue table**, and here that argument is *stronger* than the `config`/`apperror` precedent it follows: `venue-profiles.md` records the rate-limit **header names** as verified facts and does **not** record the numeric budgets, so a table would need numbers nobody verified. Non-blocking and goroutine-free — §6.3 makes rate-limit budget a `risk.Gate` *predicate*, which a call that blocked until budget appeared would answer by hanging; a refusal instead carries `NotBefore`, and the caller arms a `clock` timer.
- **The ladder is one mechanism the spec states three times** `[SPEC EDIT OWED]`. §7.6 gives a threshold ("back off at ~70% weight") without saying *what* backs off; §7.1 and §6.7 give a reservation without saying how much, or what happens when it runs out. Traffic now has a **priority**, and each priority has its own ceiling on the same budget: `Opening` (new exposure — the same act §5.3 calls `stop-opening`) → `Routine` (the reads that keep the risk view live) → `Safety` (§6.7 resolution, §7.1's top-up and REST mark failover, §7.5's reconcile, a flatten). **Monitoring outranks new exposure deliberately**: shedding market-data reads to protect trading would make the rate limiter itself the cause of a stale mark, which is §7.1's own escalation trigger — the governor would fire the de-risk it exists to help avoid. Ceilings are **absolute counts, not percentages**, because a fraction of a count is a division this package would have to round.
- **Capacity is a ceiling for every class — the user's call.** `PrioritySafety` gets an *exclusive band*, not a bypass. Past capacity the venue is expected to **refuse**, so an issued flatten is not a flatten; on Binance repeat violations escalate a 2-minute ban toward **three days**, so pushing lengthens the outage being pushed against; and §7.4's pre-placed venue-native protection and off-host actuator are the spec's *designed* answer to "cannot act at the venue". The rejected alternative — refuse only on an *observed* ban, since the local count is an estimate — is recorded with its real merit stated.
- **Header-aware means the venue's count wins in ONE direction.** A reading is always stale, so one *below* the local count is what staleness looks like and adopting it would discard every request sent after the header; one *above* it is information there is no other way to get. Ignoring a legitimate decrease costs throughput; adopting an illegitimate one costs a ban — so the correction **ratchets**, applied as a synthetic charge at the observation instant so it ages out on the window's own terms. `Penalize` carries the same ratchet twice: a venue `RetryAfter` is a **floor, not a ceiling**, and a milder penalty neither shortens a running pause nor downgrades its recorded kind.
- **Two mechanisms, because there are two failure modes.** A family's weight budget is headered and gets corrected; the venue-scoped IP envelope is not — Bybit's ~600/5s ban (retCode 10018) has no header at all, and §7.6 says IP-ban-track separation is "*inferred, not documented*". So `Cost` has **no `Requests` field**: every `Acquire` charges one to every request-dimension limit, making the one budget with no correction also the one nobody can forget to charge. The **sliding window is deliberately stricter than the venue's fixed one**: guessing the server's boundary alignment wrong lets a burst land at the end of our window and again at the start of the next.

**46 mutations, 46 caught**, two documented equivalents. **The one real defect was found before a single mutation ran**, by working out what each mutation *should* catch: `Penalize` read "was a pause already running?" *after* applying the new pause, where it is a constant — so a ban that expired last week kept outranking the throttle running now, and §7.6 alerts on exactly that. **The fixture-dimension lesson fired for the fifth and sixth time**, twice in this session, and is now a six-entry checklist item in `CLAUDE.md`.

- **`platform/observability`** — §10's metrics and structured logs on §3.4's isolated lossy lane. **The engine module still has zero production dependencies**: the Prometheus adapter is deferred to the composition roots, a scope call put to the user (decision record §12a). §11 names `prometheus/client_golang` and §3.6 rule 9 carves `kernelpurity` to in-module imports *specifically* so this package may take it — but nothing under `internal/` emits a metric yet, so a registered exporter is wiring no test can exercise end to end, and **which** binary exposes `/metrics` on **which** interface is a composition-root decision (§11 gives `arblab-actuator` no HTTP surface at all). The float64 conversion is **not** deferred: every wire metric format carries a double, so it belongs to metrics rather than to any one client library.
- **The port is integer-typed, and that is `outbox.Append` in a new place.** `prometheus.Counter` has `Add(float64)`; a port mirroring it would compile, read as the metrics contract, and turn §3.6 rule 4's *single sanctioned metric seam* into a `//nofloat:allow` at every instrumented line in the system. So the three handles take a `Sample` — an int64 mantissa at `fixedpoint`'s canonical scale, **tagged with the unit it was converted into**, and with the **asset** too for money. The unit tag catches `d.Milliseconds()` observed into a seconds histogram; the asset tag is `fixedpoint`'s runtime check extended one layer out, because BTC-`Money` and USD-`Money` are the same Go type and the one that lands on the USD PnL panel is wrong by a factor of the BTC price with nothing failing. `float64` lives in one file, and a positive control confirms the gate proves it.
- **Cardinality is the fourth `[SPEC EDIT OWED]`.** §10 requires per-venue/per-symbol metrics and says nothing about label cardinality; §3.4's lane guarantee is about a stalled *consumer*. Neither covers the real killer: a metric labelled by `ids.ClientOrderID` allocates one time series per order forever in the trading process's heap, while **every scrape succeeds and the lane reports zero drops**. A `Label` now declares its values or writes an `Unbounded` reason — the `//nofloat:allow` and `config:"hot"` shape again.
- **The logger exists because `slog` reads the wall clock where no seam reaches.** `slog.Logger.Info` calls `time.Now()` internally when it builds the `Record`. `notimenow` polices `domain/`, `application/` and `engine/` — **not `platform/`**, since `platform/clock` is where the ambient read has to live — so the kernel is exactly where such a read is invisible to the gate. `Logger` builds `slog.NewRecord` from `clock.Now`. `BufferedHandler` is the deliberate mirror of `events.ObservabilityLane`: never blocks, drops the **newest**, exact never-reset drop count, and **no internal goroutine** — `Drain` is a sweep the caller schedules.
- **Tracing is deferred with an argument.** A span is a claim about a causal chain, and this system's is §3.5's `DispatchSeq`, which does not exist until the dispatcher does. Built now it would model goroutine call trees — the wrong spine, and load-bearing by the time the right one arrived.

- **`platform/outbox`** — D2's at-least-once export, built as **ports plus a relay and deliberately no store**. The outbox's defining guarantee is that the event append commits **in one transaction with the OMS journal write** (§5.3, §6 step 6) — a property of the store, and the journal is milestone 3. So `Append`, the SQL, the schema and the driver are all absent *as a decision*: an `Append(ctx, Message)` on the port would compile, read as the outbox contract, and silently be a **second** transaction, which is the split-brain the pattern exists to prevent. Same argument that defers `hexboundary` — a check over an empty tree passes trivially. Both alternatives were put to the user (decision record §11a).
- **Per-key FIFO, scoped** `[SPEC EDIT OWED]`. §3.2 and D2 state a delivery-**count** guarantee and rely, unstated, on a delivery-**order** one — audit is "a projection of the OMS journal", and a projection that applies a fill before its order does not reproduce its source. FIFO holds **within a `Key` and not across two**, on §3.5's own rule that an unready source never stalls a ready one. The cost is stated rather than hidden: a record that fails forever blocks **its own key** forever, which is the loud direction where skipping past it silently reorders a hash-chained projection.
- **Three things whose absence would be quiet.** `Seq` is **named as the dedup key**, because at-least-once without one invites content-hash dedup, which drops the second of two legitimately identical funding accruals. `Payload` is **bytes, not `any`** — unlike `events.Envelope`, deliberately: these are the bytes that committed, and re-marshalling a Go object at delivery time lets the exported bytes differ from the committed ones, breaking a §7.5/Z4 hash chain in a way that reads as **tampering**. And the backoff **guards on the right-hand side** (`Base > Max>>attempts`) rather than shifting first: `1s << 34` overflows `int64` into a **negative** duration, scheduling the retry in the past and turning the backoff into a hot loop against a consumer that is already failing.

**38 mutation runs, 30 claims, all caught.** Four survived. Three were real gaps — the relay sorted the store's own slice in place with nothing looking, and (the **fixture-dimension lesson for the third time**) every malformed-identifier fixture put its bad character mid-string, where `i >= 0` and `i > 0` are the same predicate, so `" audit"` validated; the mirror on `Key` was fixed in the same pass. The fourth is semantically equivalent and is now **flagged in the code as an expected survivor**, because a mutation harness cannot tell a fast path from a defect.

- **`platform/config`** — §5.2's typed load + `Validate()` + safe hot-reload + the revision identity §3.5 replays against. It owns the **machinery, not the schema**: risk caps and venue endpoints belong to the contexts that own their meaning, because a kernel holding them would make every context's parameters a kernel edit (both shapes were put to the user; decision record §10a). Four things a config file does silently are now default-deny, and **two of them are checks the standard library does not make** — `encoding/json` resolves a **duplicate key** to the last occurrence with no error (and `DisallowUnknownFields` cannot see it, because both occurrences are known fields), and `json.Decode` reads one value and **ignores everything after it**, so a file holding two objects loads the first. Both carry *measurement* tests that run the same bytes through `encoding/json` and assert it accepts them, so a guard that stops being load-bearing says so.
- **Hot-reload is default-deny by struct tag** `[SPEC EDIT OWED]`. §5.2 says "hot-reloadable where safe" and stops; "where safe" is per field, and the question that settles it is which way it fails when somebody forgets. Default-allow is noticed when a running process picks up a journal path mid-trade; default-deny is noticed when a reload is refused with the field named. Same argument §3.6 rule 4 already settled for `float64`. Composites recurse rather than compare wholesale, so a hot rate-limit budget inside a non-hot venue map stays tunable while **adding** a venue needs the map itself to be hot — editing a budget and wiring a new client are different acts.
- **Two shared value types, each because its absence is quiet.** `SecretRef` **cannot hold a secret**: it holds a scheme and a name, so an inline credential has no scheme, fails to parse, and the process does not boot — §5.2's "never inline secrets" enforced rather than requested, and printing one is safe by construction rather than by remembering to redact. `Duration` crosses as a **string** because a JSON number into a `time.Duration` is *nanoseconds*: a `T_blind` of `300` meaning five minutes becomes a stale-mark budget that expires before it is first checked, firing the §7.1 de-risk on the first tick with nothing wrong.

**43 mutation runs, 37 claims, all caught.** Six survived the first pass and every one was worth the diagnosis: four were fixtures failing the check *after* the one under test (§8f's lesson, four times in one table — fixed by asserting the message, not the sentinel), one was a real gap (removing a map entry is caught by the per-key walk; **adding** one is invisible to it), and one was redundant code, deleted with the surviving path bite-checked in its place.

- **`platform/apperror`** — the error taxonomy §5.1 calls `errclass`, built on **two orthogonal axes**. `Class` routes (backoff §7.6, alerting, the §10 metric label); `Effect` answers *did the side effect happen*, which is what drives an order into `SubmissionUnknown`. Neither determines the other — a 429 and a read timeout are both HTTP failures at a venue and differ absolutely on the safety axis — and collapsing them would make the safety fact something callers **derive from the cause**, which is the inference §5.3 forbids. Every default is chosen so that forgetting it fails safe: an unclassified error reads `ClassInternal`/`EffectUnknown`, and **wrapping can never manufacture certainty** (agreement is preserved; disagreement collapses to `EffectUnknown`), so an outer layer cannot launder "the order might be live" into "there is no order".
- **`platform/ids`** — UUIDv7 (written fresh per §11 and checked against **RFC 9562's own worked example**), `TradeID`/`IntentID`/`ExecutionID`, `Venue` **moved here from `platform/clock`**, and the **deterministic, reversible `ClientOrderID`**: `ab-<26 Crockford base32 of the TradeID>-<leg>-<attempt>`, fixed at 35 characters. Reversible rather than hashed because §5.3 makes it a *recovery correlation handle* — an operator reading a venue's order page must recover the trade **without database access**, and §11 says the journal is host-local and lost on host death, so the incident where that matters is the one where the lookup table is gone.

Load-bearing-tested as before: an independent `math/big` oracle for the base32 codec (which caught a wrong hand-encoded golden value on its first run), RFC 9562's vector for the layout, and **39 hand-applied mutations across the two packages — 38 caught, 1 diagnosed as semantically equivalent**. Three survived a first pass and every one was a real gap; the diagnoses are recorded as standing lessons.

**Session 7 added the instrument identity and the delivery lanes.**

- **`platform/ids` gains `VenueInstrument`/`ProductCategory`/`ContractType`** — §5.1's canonical identity, in `ids` rather than `events` so that an `oms.Order` does not import the event bus to name an instrument. Three claims carry it: spot and perp on one symbol are **distinct** `[→ Z1]` (collapsing them merges two execution-ID namespaces and silently drops a fill); there is **exactly one spelling** of each instrument, because it keys the §3.4 mailboxes and two spellings are two mailboxes; and `Product` and `ContractType` **must agree**, since §5.1 encodes linear-vs-inverse twice and a redundant encoding is safe only while something forbids the copies from disagreeing.
- **`platform/events`** — §3.4's three lanes, which are three answers to one question. **Control** blocks when full and never drops (back-pressure reaching the socket is *designed*: a stalled reader is visible, a dropped execution event is not). **Market data** never blocks, coalesces, and drops its **oldest** with a counted freshness gap. **Observability** never blocks and drops its **newest** — the opposite direction, deliberately, and pinned by a test in each file. The mark is the exception the lane exists for `[→ R2c-1]`: **adverse-preserving** coalescing keeps whichever payload is worse, so a spike that breached a threshold survives the recovery that follows it, and the surviving envelope is always a **real observation** rather than the worst price wearing the newest timestamp. Deliberately absent: the parity dispatcher (it needs a persisted `DispatchSeq` log) and §3.4's command sequencer (no Risk and no OMS exist to order between).

**43 mutations, 41 caught, 2 diagnosed — and both survivors were real gaps, now closed.** `tools/mutate.sh` gained two verdicts, each found by a real mutation rather than designed in: **`NOBUILD!!`** (a mutant that does not compile printed as `caught` and proved nothing — the tell was a failing-test count of 0) and **`TIMEOUT!!`** (a mutation that made a non-blocking send block *hung* the suite instead of failing it — §5e's own lesson, arriving through the harness).

**A `[SPEC EDIT OWED]`, tracked by a marker in the code:** §3.5 declares `SourceID` as four flat fields, and the implementation composes `ids.VenueInstrument` instead, because the flat form cannot express the `ContractType` §5.1 gives an instrument. Following v0.9.5's precedent it lands with the rest of the slice's edits as one release — **`grep -rn "SPEC EDIT OWED" internal/` before closing the slice.**

**Also discharged: the `CLAUDE.md` owed since the layout flatten.** It now exists at the repo root with the `internal/` taxonomy, the doc-set map, the two-module build split, the three enforced gates and the standing working rules.

**One item carried forward, not blocking:** the ClientOrderID's 36-character venue budget is an **unverified assumption** — not in `venue-profiles.md`, not browser-confirmed. Named as a constant, asserted by a test, and to be confirmed before milestone 3 places a real order.

**That fork is settled: (a) was taken and the slice is closed** — the eight edits are `build-spec.md` v0.9.6 (top of this file); §15b's four followed as v0.9.7, and the next slice is **`marketdata`**, the first bounded context.

**Still owed and deferred on argument, not forgotten:** `platform/outbox`'s store half (`Append`, the SQL, the migration, the driver) lands with the OMS journal in milestone 3 per §11a; `boot`'s reconcile join (`OrderByClientID`, `Fills`, `Positions`, `FundingLedger`) lands there too, on the same argument, sharpened — a `Reconciler` port now would make the checklist look **satisfiable**, since a nil-returning implementation reports every gate passed with nothing joined; and `observability`'s Prometheus adapter and tracing land with the composition roots (§12a, §12f).

> **Read `docs/superpowers/specs/2026-07-26-milestone-1-foundation-decisions.md` first — its RESUME HERE block.**
> That record holds every decision taken with its rationale *and rejected alternatives*, the verified
> environment, the deferred scope each with a reason, **§5b/§5c on `fixedpoint` as actually built and
> the test strategy as actually executed**, and the exact next work items in order.
> Everything below is a summary of it; the record is authoritative.

**All four forks the previous brief named are settled**, plus thirteen more. The one-line versions:

1. **Scope.** First slice = scaffolding + `platform/fixedpoint` + `platform/clock` + the `nofloat` and no-`time.Now()` gates. The eleven kernel packages form a strict dependency spine and anything above the bottom would be rewritten when the bottom lands. `fixedpoint` first because **milestone 2 is a kill gate whose verdict is a money computation** — a wrong rounding direction moves the number across the threshold and kills or green-lights the project on a bug.
2. **Layout.** `go.mod` at root, module `github.com/giiorg/arb-lab`, `go 1.26.0` + `toolchain go1.26.5`. `main` fast-forwarded to v0.9.4 and **pushed**; the spec is now v0.9.5 on the feature branch, also pushed, code on `feat/milestone-1-foundation`. `tools/lint` is a **nested module** and there is deliberately **no `go.work`** (a workspace would let `x/tools` become importable from the engine). `migrations/` partitioned by dialect from the first file.
3. **Gates = custom `go/analysis` analyzers**, each with `analysistest` fixtures that MUST produce diagnostics — because a `depguard` stanza cannot be unit-tested and `nofloat` cannot be config at all (it needs type info). **6 planned, 3 built** (`kernelpurity`, `nofloat`, `notimenow`); the other three are deferred because nothing exists to police yet.
4. **`fixedpoint`** = `int64` mantissa @ `Scale = 9` + `math/bits` 128-bit intermediates; `(T, error)` + a sticky-error `Calc`; **zero value invalid**; `Price` carries `base`+`quote`.

**Also decided, and not in the previous brief:**

- **The language question was asked and properly assessed: Go is confirmed over Rust.** Recorded with measured switching costs and Rust's genuine wins stated honestly (compile-time `enum` exhaustiveness across the seven safety machines; native `i128` + `Result`/`?`, which would have removed the `Calc` type entirely; `cargo-mutants`). The decisive counter is structural: **this design is ports-and-adapters maximalist (~20 I/O ports needing dynamic dispatch, which is §3.5's entire parity mechanism) and `dyn`-compatible async traits are Rust's worst-supported combination.** The compensating measure is a sixth gate, `exhaustiveswitch` — now §3.6 rule 10. *"No GC pauses"* is explicitly debunked for a strategy on 8-hour settlement cycles. **The correctness ceiling is set by neither language — it is TLC trace-checking against the TLA+ model, which the spec already plans.**
- **`boundary/` is dropped — contexts sit at `internal/<ctx>/`.** `contexts/` was rejected by the user (collides with `context.Context`); no third word survives. Settled by the observation that **none of §3.6's gates depend on the prefix** — they key on the inner layer names — so it carried zero enforcement value.
- **`kernelpurity` is a fifth gate, now §3.6 rule 9** (built, green, bite-checked): `platform/` may import only stdlib + `platform/`, catching a domain leak and a spine inversion with one rule.
- **Test-the-tests = negative controls in CI** (the NC1–NC5 discipline from `docs/formal/`), with `gremlins` as a timeboxed audit only — **RUN and closed out in session 5 (decision record §6b); do not treat it as owed.** It *does* build on Go 1.26, contrary to the guess in §0, and it earned its keep: **four real gaps found, all four fixed.** The one that matters is that `mulDivMantissa`'s guard in front of `bits.Div64` was untested at its boundary — loosening `hi >= uden` to `hi > uden` survived the whole suite, and the input that separates them **panics inside `math/bits`**, on exactly the `DispatchAuthorized` crash boundary §5.3 exists to keep clear. Also found: `Quantity.Mul` at 0.0% coverage, `Certainty.String` at 0.0%, and both analyzers' diagnostic *ordering* unasserted (because `analysistest` matches by line and is order-insensitive by construction). **It stays out of CI for reasons that are now measured, not reputational:** a warm Go test cache makes it report `0.00% efficacy` on a healthy suite, `./...` silently finds nothing, non-viable mutants are scored as kills, bare-`switch` case expressions are structurally invisible to it (which is *most of the money kernel's three-way comparison logic*), and **one run in four produced a false LIVED** on a mutant the suite kills 20/20 by hand. Every `LIVED` it reports must be reproduced manually before it is believed.

**All ten [SPEC EDIT OWED] items are LANDED (session 4) as `build-spec.md` v0.9.5** — the first implementation-feedback release — plus a milestone-1 implementation-decision block in ADR §3. **Nothing is owed and nothing is pending.** What changed: the `boundary/` flatten (26 path occurrences; `boundary/lint` in §3.6 left alone, it is not a path); `kernelpurity` and `exhaustiveswitch` added as §3.6 rules 9 and 10, with a **gate-status paragraph** naming which of the six are enforced and what each deferred one waits on; the Go-vs-Rust ADR entry; `RoundingMode` documented as an integer-enum exception to rule 8; the `fixedpoint` error-precedence contract; rule 5 restated as the whole ambient-time family **and widened to `engine/`**; `TimerSource` and the timer priority direction added to §3.5.

**The one user question of this milestone was asked and answered: YES, rule 5 reaches `engine/`** — with a transport-only escape. The deciding argument is **not** parity: it is that `platform/clock`'s monotonic clamp is only a guarantee if everyone reads the same clock. `Wall` absorbs backwards time and `time.Now` does not, so a `RecvTime` stamped from the raw clock and an age computed from the injected one disagree **exactly across the NTP step backwards the clamp exists for** — yielding a negative age, which reads as *perfectly fresh*, in code where freshness gates execution. Scope was kept **structural** rather than an allow-list of the parity-shared engine packages, since an allow-list is the failure mode the gate exists to prevent. `engine/` alone may write `//notimenow:allow <reason>`; `domain/`/`application/` admit none, and the directive written there suppresses nothing and is reported. **This carries a falsifiable claim: if `engine/venue` ships and the escape is never used, delete the mechanism.** Full reasoning + the three rejected alternatives: decision record §4d.

The `RoundingMode` set was confirmed **as proposed, unchanged** — eight modes, `RoundInvalid = 0`, `RoundExact` kept, `HalfDown` not added.

**Nothing outward-facing is owed.** Both branches were pushed 2026-07-26 on the user's instruction: `main` as a verified fast-forward (`origin/main` was a strict ancestor, 58 behind / 0 diverged), and `feat/milestone-1-foundation` now tracks its remote. **No PR has been opened** — pushed is not proposed; open one only if asked.

**Read first, in this order:** the decision record above (its RESUME HERE block) → `build-spec.md` **§3.2** (the package map), **§3.5** (backtest-live parity — the `Clock`/`Envelope` contract the conformance suite tests), **§3.6** (ten structural rules as of v0.9.5, plus the gate-status paragraph), **§4** (`VenueInstrument`), **§12.1**. Then this file's *Locked decisions*.

**Verified environment, do not re-derive:** **Go 1.26.5** (upgraded from 1.26.0 in session 5; `make ci` green on it and the `gremlins` audit reproduced identically across both). **Both `go.mod`s declare `go 1.26.0` as the minimum and `toolchain go1.26.5` as what to build with** — the split is deliberate, because `.github/workflows/ci.yml` installs via `go-version-file: go.mod`, so the `go` line doubles as the CI pin; do not collapse the two directives. Remote `github.com/giiorg/arb-lab`. Branch `feat/milestone-1-foundation`, tree clean, `make ci` green (fmt-check → vet both modules → custom gates → race tests both modules). Two Go modules: root (the engine) and `tools/lint` (nested, so `golang.org/x/tools` stays out of the engine's dependency graph). `bitnet-backend-go` is **not on this machine** — a described style, not a readable source. The `gopls` "not in your workspace" warnings are **expected and correct** — see the `go.work` decision.

**Built and green:** the scaffold (`go.mod`, `Makefile`, GitHub Actions, `.gitignore`, `migrations/README.md`), the `kernelpurity` analyzer + its multichecker binary, and **the whole of `platform/fixedpoint`**:

- **Four categories** — `Money`, `Quantity`, `Price{base,quote}`, `Rate` — every zero value invalid, cross-category arithmetic a *compile* error, cross-asset arithmetic a runtime error. `Rate` carries an explicit `set bool` because it is dimensionless and an unpopulated **fee** rate would otherwise read as zero fees, which clears any hurdle.
- **`mulDivMantissa`** — one 128-bit `math/bits` rounding kernel every operation routes through, so rounding policy exists in exactly one place. `Notional`/`AveragePrice` as inverses, `Money.Mul(Rate)`, `Money.Div(Money) → Rate`, the `Calc` chain, and the persistence codec (`int64` mantissa = SQLite `INTEGER` = Postgres `BIGINT`, which is D2's cheap migration path made concrete).
- **An error-precedence contract**: invalid mode → divide-by-zero → **overflow** → precision loss. This was *found by the oracle*, not by review: the kernel had been classifying the same condition two ways depending on which side of an internal 64-bit boundary it fell.
- **`rapid` (test-only, v1.3.0) against a `math/big` oracle written as a different algorithm** — it rounds by comparing distances to the two integer neighbours, where the fast path derives an increment from a remainder. 19 negative controls, each rejected with its counterexample logged; 4 positive controls, because a predicate that always objects looks identical to a perfect result. The `float64` control is caught at `123456789.123456789` returning `...784` — rule 4 demonstrated, not asserted.
- **`-rapid.seed` semantics verified from module source** and found to be the *opposite* of a safe gate default: 0 means "choose a random one", so the properties are non-deterministic out of the box. `make test`/`make ci` now pin `RAPID_SEED`; `make test-properties` deliberately does not (the nightly explorer).
- **21 mutations applied across the session, every construct bite-checked.** Two did not bite and both were diagnosed rather than papered over — one semantically equivalent (pinned by an internal test instead), one simply a bad mutation.

**Also built and green (session 3):**

- **`FuzzParseMantissa`** — the parser is the only untrusted-input boundary, so its fuzz body asserts four *total* invariants rather than checking for panics: `ErrSyntax` exactly when the input is outside an independently-stated grammar; every error matching exactly one sentinel class; the canonical round trip as a fixed point; and agreement with the `math/big` oracle where the oracle is defined. **The oracle gate was justified from `math/big` source** — `Rat.SetString` refuses `|exp| > 1e6` and reports it as a *parse failure*, which is not what this package means by a syntax error. Corpus reach was **measured, not assumed**: 162 of 348 generated entries reach the oracle cross-check.
- **`nofloat`** — type-based and default-deny under `internal/`, because the rule is not "the token `float64` does not appear" but "no value here is a float". It catches `x := 1.5`, `a / b` with float operands, and `d.Seconds()` — the last being how it actually happens, with no float token at the call site. The `//nofloat:allow <reason>` escape is *enforced*: a bare directive exempts nothing, and an escape on a line with no float is reported as unused. It fired on real code immediately and both hits were resolved rather than suppressed.
- **`notimenow`** — resolved through `go/types`, so `import wall "time"` and a dot-import are both caught, and it reports the *reference* so `var f = time.Now` counts. It bans the whole ambient-time family, because a gate stopping at `time.Now` is bypassed by `time.Since` — which is idiom, not evasion.
- **`platform/clock`** — the four §3.5 methods, `Wall` + `TestClock`, and `ServerTime` with a **three-valued** `Passed()` so a funding boundary inside the uncertainty band cannot be silently resolved. An uncalibrated venue yields an *unset* `ServerTime`, because a zero offset with a zero band is the most confident answer the package can give and would be given on no data. Timer delivery is **pull-based** on a separate `TimerSource`: §3.5 gives the dispatcher the total order over events *and* timers, so a pushing Clock would split the arbitration that decides the replay key. The package therefore contains no goroutines at all.

**Next items, in order** (detail in the decision record's RESUME HERE):
1. **Milestone 1's second slice**, per §1's deferred table: `platform/apperror` and `platform/ids` (both import nothing), then `platform/events` — which needed `clock`, and now has it.
2. A **`CLAUDE.md` line for the flattened layout** — `ls internal/` now shows contexts and machinery groups as peers, so the taxonomy needs one sentence.

*(The ten `[SPEC EDIT OWED]` items and the `gremlins` audit were the previous two entries here. Both are done — see above.)*

**Do not over-build.** Milestone 1 exists to make **milestone 2 — the economic gate — measurable**, and §12.2 is an explicit kill gate the spec predicts BTC/VIP0 may fail (≈3.4% at 60% utilization vs the ~4.6% hurdle). Build the foundation the gate needs; the OMS is milestone 3.

**Nothing blocks this.** It does not wait on the guide's voice gate, on R2d, or on the Binance ⚠ items (those gate milestone 3). If R2d later changes the spec, §3.2/§3.5/§3.6 are the parts stable across four review rounds — **but note this session has now deliberately edited §3.2/§3.6's surface** (the flatten + two added rules), all four edits recorded as owed and traceable.

**Build it test-first** — the user's stack note is Go + DDD/hexagonal, NautilusTrader-style, and "don't assume, clarify"; the `tdd` skill governs the loop, red shown before green. Per the user's standing rule, **no AI attribution** in commits or any authored content.

### Open finding, still untriaged (needs a tag + ADR §3 entry)

`core §12` cites §2.4 for "spot maker entry ≈ **−40% round-trip cost**", but `core §2.4` says maker spot entry "roughly **halves** round-trip cost and shortens **break-even** by ~40%" — a −50% cost effect and a separate −40% break-even effect, which §12's paraphrase appears to have collapsed. The guide models §2.4's own wording. Found by trying to *compute* from the spec rather than read it.

## Artifacts (read in this order)
0. **`docs/review/2026-07-20-10-v0.9-formal-model-b-FM-1-record.md`** — the **formal-model (b) rounds**: FM-1 finding + resolution → v0.9.1, the **Cx6 addendum** (the Codex adversarial round on the model itself + FM-2 → v0.9.2), the **Cx7 addendum** (the round-2 *attempt*: **FM-3 + Cx7-1/-2/-3, all applied → v0.9.3**; read its Provenance section — the round is deliberately *not* recorded as a clean Codex round), and the **Cx8 addendum** (the **genuine independent round-2: NO-SHIP, FM-4 + Cx8-2/-3, both applied → v0.9.4**; newest — the re-review obligation is now **discharged**). The model + correspondence map + TLC results: **`docs/formal/`** (run `./run.sh` there to reproduce all eight checks, ≈32 min).
0b. **`docs/review/2026-07-20-09-v0.8-round4-R1d-review-and-resolution-record.md`** — the **round-4 R1d** findings + triage + venue-verification annexes.
1. **`docs/build-spec.md`** — the **normative core**, v0.9. Requirements + `[→ …]` tags live here; mechanism detail lives behind `[mechanism: …]` pointers into the other four.
2. **`docs/state-machines.md`** — the **seven safety-kernel/execution state machines** (OMS order lifecycle, ambiguous-order classifier, FenceReadyLost arbitration, actuator takeover, protective-order disarm, EmergencyPlan lifecycle, PackageDelever generations): state lists, transition tables, invariants, chaos cases; **normative mechanism incorporated by reference from the core's `[mechanism:]` pointers (R1d-11); the 7 FLAGs are resolved in place (R1d rulings)**.
3. **`docs/venue-profiles.md`** — dated venue capability facts for Binance + Bybit (quirk matrix, account/margin posture, spot-leg surface, verification citations).
4. **`docs/tutorial.md`** — teaching companion (Georgian: `docs/tutorial.ka.md`, **generated** — see `docs/i18n/README.md`) (worked ledger on `C_allocated`; ADL re-hedge, crash-boundary, denominator, mark-driven liq + `PackageDelever` fallback corrected); now cross-references `state-machines.md` (SM) and `venue-profiles.md` (Profiles) where the taught mechanism moved.
5. **`docs/adr-history.md`** — the convergence narrative (ADR §1), review-records index (ADR §2), the finding→resolution traceability table (ADR §3), and the v0.8 doc-split map + flagged ambiguities (ADR §4).
6. **`docs/review/2026-07-19-08-v0.7.1-review-and-verification-record.md`** — the **round-3 R1c/R2c** findings + resolution + the Codex convergence pass.
7. **`docs/review/2026-07-19-07-v0.7-review-and-verification-record.md`** — the round-2 R1/R2 findings + resolution + the Codex convergence findings + the venue/tooling verification citations (URLs).
8. **`docs/review/2026-07-18-06-v0.6-review-and-verification-record.md`** — the v0.6 round.
9. **`docs/review/2026-07-18-05-tutorial-critical-review.md`** — the earlier critical review.
10. **`docs/legacy/`** — frozen research record. **Do not edit.**

## Locked decisions (do NOT re-litigate)
- **Stack:** from-scratch **Go** backend; **React/Next on Bun** operator dashboard. OSS = reference only. **Reaffirmed 2026-07-26 after an explicit Rust-vs-Go assessment** at the moment the switching cost was smallest (~700 lines of code; the real cost was 1–2 sessions of spec revision plus a review round). Rust's genuine wins are recorded, not strawmanned — compile-time `enum` exhaustiveness over the seven safety machines, native `i128`, `Result`/`?` (which would remove the `Calc` type entirely), and `cargo-mutants`. The decisive counter is structural: **this design is ports-and-adapters maximalist — ~20 I/O-performing ports needing dynamic dispatch, which *is* §3.5's parity mechanism — and `dyn`-compatible async traits are Rust's worst-supported combination.** Compensating measure: the `exhaustiveswitch` gate. Full assessment: `docs/superpowers/specs/2026-07-26-milestone-1-foundation-decisions.md` §0. **Do not reopen without new information.**
- **Strategy-pluggable foundation**; **MVP proves cash-and-carry FIRST**; cross-exchange spread is strategy #2 (alts/dislocation only).
- **Venues:** Binance + Bybit, N-venue-ready behind one adapter seam.
- **Architecture D1–D4** (single-process engine; journal+outbox→embedded store **SQLite WAL + `synchronous=FULL`**; `platform/fixedpoint`; parity `Clock` + total-order dispatcher). DDD/hexagonal à la `bitnet-backend-go`.
- **Custody = on-exchange for MVP** (isolated per-venue subaccounts, caps ≤~20–30%/venue, USDT/USDC margin). OES = documented scale-up.
- **Account posture = segregated / isolated on both venues**, boot- and pre-trade-verified (incl. **one-way position mode + leverage pin**).
- **Risk bar = real capital, live.**

## What v0.7 hardened (full map in `adr-history.md` ADR §3 + the v0.7 review record)
- **Safety kernel — ambiguous-order recovery (R2 Blocker + Codex Blocker):** the one-leg-failure handler classifies a residual **determined vs ambiguous by *remaining executable quantity*** — a compensating trade against a `PartiallyFilled`/`Acknowledged`/`CancelPending`/`SubmissionUnknown` counter-leg can *create* the exposure it removes (naked-short / doubled-short), so it must **quiesce every such order to finality** (terminal state + executions/position reconciliation; for a cancel: cancel-ack **then** an executions sweep) **before** compensating.
- **Safety kernel — off-host `Fence` (R1 + R2 major):** a per-venue **`Fence` capability** with verified mechanisms — Bybit `update-sub-api` (transfer/withdrawal-class authority); Binance has **no REST key-disable for a *regular* account** (only via sub-account + a held master key) — a **fence-unavailable failure branch**, the fence-credential's blast radius, and the coupling that **signer isolation binds at `$1K` (not `$10K`) on any venue lacking exchange-side fencing**.
- **Crash boundary (R2 major):** a durable **`DispatchAuthorized`** state makes recovery **query-only** for every pre-ack state (never resubmit / never infer "not sent").
- **Rescue margin (R1 major):** under Isolated it is an **active `add-margin` top-up** (Binance `positionMargin{type:1}` / Bybit `position/add-margin`) with latency budget + reserved rate-limit, and a **`PackageDelever`** fallback (equal-base both-leg reduction) — *not* `ExposureReduction` — if the top-up fails.
- **Posture (R2 major):** **one-way position mode + configured leverage** added to boot/pre-trade/on-change verification; freshness anchored to a **max-age TTL + REST re-verify** (position-mode changes emit no venue event).
- **Operator control plane (R2 major):** a security contract (mTLS/authz/replay/CSRF/private-network/per-actor-audit).
- **Economic gate (R2 major):** hardened (depth-walk, decision-to-arrival latency, per-regime + rolling, max-drawdown, lower confidence bound) and its numeric thresholds are a **versioned, approved calibration artifact** (not per-developer-selectable).
- **Plus:** raw-arrival (not `SourceTime`) event ordering; SQLite `synchronous=FULL` on Linux; shared IP-ban envelope; `$1K`↔§7.4 attach-coverage alignment; exit-score closing-cost symmetry; canary reframed as bounded-loss acceptance; several tutorial corrections.

## What v0.7.1 added (round 3 — R1c/R2c + Codex; full map in `adr-history.md` ADR §3 + the v0.7.1 review record)
- **Liq monitoring is mark-price-driven (R2 blocker):** liq distance is recomputed on **every fresh mark update** (not only account events — it moves with mark); a **stale mark blocks openings + escalates** (§2.2/§6/§7.1/§9).
- **`FenceReady` continuous health + proactive wind-down (R2 major):** the fence's health is polled continuously; its loss **blocks openings and winds exposure down while the bot is alive**, so a live `$1K` position is wound down (or handed to the actuator if the signer itself failed — Cx4-5) before reaching an incident un-fenced, within a bounded RTO (the incident-time de-minimis branch is a last resort for the residual only) (§7.4/§1.1).
- **First-class safety ports (R2 major):** `MarginGateway` / `FenceGateway` / `ProtectionGateway` / `EmergencyPlanStore`; `TransferCollateral` leaves the read-oriented `AccountSource` and gains submit-unknown semantics (§4).
- **Three-failure-domain topology (R2 major):** trading engine ∥ operator/control ∥ a **minimal actuator daemon (no dashboard/HTTP surface, holds the fence credential)**; "single process" = the engine only (§3.2/§11).
- **Dispatch sequence decoupled (R2 major):** `IngressSeq`/`DispatchSeq` (replay key = consumed order) is no longer forced to equal `AcceptOrdinal`/`RawIngressSeq` (raw-arrival causal order) — no cross-source head-of-line blocking; explicit mapping; replay still keys on consumed order with no `SourceTime` reorder (§3.5).
- **Signer isolation is the *primary* `Fence` (R1):** the exchange-side Bybit fence credential unavoidably holds a **transfer-class** authority (grant the narrowest — `Sub Member Transfer`, never `Withdrawal`); signer isolation (the fence action needs no venue call or additional key-management/transfer-class credential — the signer holds the trade key; tier-independent) is preferred, exchange-side disable the backup (§7.3/§7.4).
- **Interval-floor compensation (R1):** in the deadline-exceeded ambiguous branch, compensate the residual **invariant across the ambiguity** (the interval floor, when the possible-delta interval doesn't straddle zero) instead of de-minimis-only — provably safe, shrinks the naked window (§6.7).
- **Quantified canary loss bound (R2 medium):** the canary now requires a **written risk-acceptance spec** (max notional/collateral, worst-case = uncovered-leg notional, BTC-only, manual-recovery RTO, independent monitoring, signed acceptance) — "sub-`$1K`" is not a spec (§12).
- **Plus:** `EmergencyClosePackage` rationale fixed (R2); durability backup/anchor/restore-drill requirements made real — SQLite online-backup, not raw copy (R2); near-risk-free, revision-controlled hurdle (R2); sub-account-availability verify-item + single-account fallback (R1); the milestone-2 strategic fork named + mechanically-derived thresholds as solo default (R1); tutorial corrected to the safety kernel (R1/R2); provenance fixes — 4 Codex rounds, verification dates (R2).

## Residual follow-ups (for the implementing team, TDD — not the spec)
- **Model-test before milestone 3:** the **ambiguous-order-recovery state machine** (§6.7 — determined/ambiguous by remaining executable quantity, quiesce-to-finality) and the **takeover state machine** (§7.4).
- **Before real capital:** an **empirical testnet liquidation test** of the isolated-perp / segregated-spot posture (§4.1).
- **Integration-test items (verify, don't assume):** whether a Bybit/Binance **spot IOC can carry an attached bracket** (doc-silent — assume unprotected until proven); the per-venue **`Fence` propagation test**; **confirm Binance sub-account availability for the actual account** (tier-gated — R1c-3; if absent, single-account fallback + signer-isolation fence).
- **Prove in code** (spec §9 chaos): ambiguous one-leg failure across *every* reachable venue state (partial-fill before/after cancel-ack; stable-not-found→discovered fill); **mark-price-driven liq breach + stale-mark block/escalate**; **`FenceReady`-loss proactive wind-down**; actuator takeover fence + **fence-unavailable** branch; **actuator crash *before* the latch during a branch-2 hand-off → re-attempt from durable state, never an idle incident with the bot barred `[→ FM-3]`** (and the same test with the hand-off signal dropped); **every failed dimension repairs *before* a branch is selected → a branch is still selected, from the recorded failure; never a stranded un-arbitrated `FenceReadyLost` `[→ FM-4]`**; **a budget-exhausted `HeldIncident` stays parked (no retry loop with the budget spent) yet still completes once the position is proved flat — the two parks have different exits `[→ Cx8-2]`**; **margin top-up under stress → `PackageDelever`**; ADL-from-a-zeroed-perp; disarm race; Bybit history fallback; wrong/out-of-band posture (incl. flipped position mode).
- **Calibrate:** the `FundingForecast` estimator + joint-aggregation + `BasisForecast`; and produce the **versioned economic-gate calibration artifact** (the milestone-2 gate input).

## Doc split (R2/R2c-recommended) — **DONE**
Executed 2026-07-20 as the v0.8 restructuring (see the status line and the doc-split record above). The prior single doc mixing normative spec + review history + incident catalogue is now five focused docs (Artifacts, above); what moved where: **ADR §4**.

## Open decisions (for the user)
- **Go-live sequencing** (largely converged). Round-2 *and* round-3 reviewers converge on **option (a): the sub-`$1K` canary** — an **explicit acceptance of a bounded, de-minimis naked-window loss** (Bybit's venue-native attach covers only the *perp*; the spot IOC leg shares the uncovered window), during which the `Fence` + takeover protocol is **built and chaos-tested**; promotion to `$1K` is **gated on that protocol passing §9**. The spec's recommended default (§12); still your call to confirm. **New in v0.7.1:** the canary is **not approved until you write its risk-acceptance numbers** (max notional/collateral; accepted worst-case loss = the uncovered leg's full notional; manual-recovery RTO; who signs) — R2c-8 made "sub-`$1K`" insufficient. Those numbers encode *your* risk tolerance → a genuine user input.
- **Strategic fork if the milestone-2 economic gate fails** (R1c-5) — the likely BTC/VIP0 outcome the kill-math predicts. The forks (maker-entry tactic / regime-selective / alt niche earlier than planned / accept sub-hurdle as tuition, §12) are surfaced now; which one is a user call, not automatic.
- **Doc split** (above) — **done**; see the v0.8 doc-split map (ADR §4).
- **Formal specs for the safety kernel (raised 2026-07-20; adoption decision still pending — but model (b) is now DONE through its own adversarial round as the evidence):** **model (b) — the FenceReadyLost → takeover → EmergencyPlan composition (SM §3/§4/§6) — is built, checked, adversarially reviewed (Cx6), and productive** (v0.9.1: found + resolved **FM-1**; v0.9.2: the Cx6 round hardened the model itself — 6 valid findings — and its liveness re-verification found + resolved **FM-2**; `docs/formal/`, **eight** TLC runs all matching predictions; process followed exactly — companion artifact, correspondence map, counterexample → FM-tagged finding, never a silent fix; **the round-2 re-review is complete (Cx8) and the obligation is discharged**). **Remaining for the user:** (i) whether to adopt formal modeling as a standing practice (the concrete evidence: FM-1 — a real composition defect three review rounds missed, found on the first nominal run; FM-2 — a smaller spec silence found by the round's liveness re-check; and the instructive counterpoint that the model itself needed a 6-finding adversarial round — the loop catches errors in both directions — all at the cost of ~two sessions); (ii) whether to commission **model (a)** — the ambiguous-order classifier + finality contract + reservation ledger (`IntervalFloorReduction`/`PackageDelever`, SM §2/§7 — invariants: never overshoot sign, floor never issued twice, no naked/doubled short in any reachable state) — same process; (iii) the optional one-off **Lean 4** proof of the interval-floor arithmetic lemma. **Gobra stays deferred to implementation** (then prefer `rapid` property tests + the deterministic-sim harness the parity `Clock`/dispatcher enables + TLC trace-checking against the real OMS). Two named model-(b) refinement candidates if extended: monitoring lag (observed health ≠ truth), multi-trade concurrency. **Updated 2026-07-25 by Cx7 (applied, v0.9.3):** the adoption evidence is now **three** model-found spec defects — FM-1 (composition), FM-2 (spurious-fire silence), **FM-3 (branch-2 re-engagement silence)** — and FM-3 is the first found by attacking a *previous round's fix* rather than the original spec, which is the argument that the practice keeps paying out rather than converging to zero. The counter-evidence also grew: two of Cx7's four items are model bugs, not spec bugs, and a third (Cx7-3) is a gap in the *verification suite* — the round's sharpest lesson being that **7/7 green proved only what the properties said**: the completion path had no capability property over it, and the automated-convergence property's escape hatch was certifying the very stall it should have caught. Cost so far ≈ four sessions. **Updated 2026-07-25 by Cx8 (applied, v0.9.4) — and this is the round that makes the evidence *independent*:** the adoption case is now **four** model-found spec defects (FM-1 composition, FM-2 spurious-fire silence, FM-3 branch-2 re-engagement silence, **FM-4 pre-branch arbitration silence**), and the self-certification caveat that weakened the FM-3 evidence is **gone** — an independent reviewer audited the fixed artifact, wrote its own properties, ran the checker and returned a verdict, and FM-3 + Cx7-1/-2/-3 all survived it. The counter-evidence is equally sharpened: Cx8's own two findings are one spec bug and one model bug, applying the model fix **introduced a liveness dead end** that only the suite caught, and the round's headline methodological lesson is that **a passing liveness property and a vacuous one look identical** — which is why NC5 (a negative control over a *temporal property*) now exists, and why the other four liveness properties still lack one. Cost so far ≈ five sessions.

## RESUME HERE (2026-07-25): no formal-model debt is outstanding

**v0.9.4 is applied and re-verified (8/8 — status line above); the Cx8 round is closed and the re-review obligation Cx6 raised is discharged** (an independent reviewer audited the fixed artifact, wrote its own properties, ran TLC, and returned a verdict — Cx8 addendum in `docs/review/2026-07-20-10-…`). Nothing from Cx7 or Cx8 is outstanding.

**A round-3 review of the model is now a judgment call, not a debt.** If commissioned: refresh `docs/formal/.tools/round2-review-context-v0.9.3.md` for the v0.9.4 surface (`signingAtLoss` + the branch-guard partition and whether it is *really* exhaustive; `heldCause` and whether the two `HeldIncident` exits are now both correct **and** complete; `P3`/`P4` scoping — `P4` deliberately admits actuator engagement as a hand-off, so attack whether that is an escape hatch; **NC5** and whether the other liveness properties deserve controls of their own — `P2`, `P3`, `Branch1AutoResolves` and `Branch2AutoResolves` currently have none, which is exactly the "green vs vacuous" gap NC5 exists to close for `P4`). Sessions for reference: round 1 `codex resume 019f7dff-94da-79e1-88aa-d7d4499199ee`, round 2 `codex resume 019f990c-5d2e-7760-9d22-2022b0e220da`. **Round 2's reviewer-authored probes are tracked at `docs/formal/probes/` — read its README before running them:** `ExhaustedHeldEventuallyParks` PASSES on the fixed model (independent confirmation of Cx8-2), while `LostEventuallyLeaves` still fails **by design** on a benign watchdog-park trace and must not be "fixed" into the model. Standing pushback to be ready for is unchanged (the SF→WF audit — note FM-4 has now *partly vindicated* the concern, since the persistence argument it rested on was false as written; `-lncheck final`; the recorded design rejections for FM-3 and FM-4).

**The one cheap, high-value item if the model is touched again:** negative controls for the remaining liveness properties. NC5 proved `P4` bites; `P2`, `P3` and the two branch properties are still in the state every property was in before this round — passing, with no demonstration that they *can* fail. Each control costs one config + one constant and a few seconds of runtime; each liveness property costs ≈4-5 min of `_Live2` temporal phase, so the budget is real.

**Priorities from here (revised 2026-07-26 — the immediate task is now `milestone 1`, see the NEXT REQUEST block above; these are the *spec-side* priorities, none of which block it):** R2d triage when it arrives (step 0 below) → the ⚠-flagged Binance verify-items (Profiles §4) → the user's open formal-methods adoption decision (evidence now FM-1 + FM-2 + FM-3 + **FM-4**, against the counter-evidence that model/verification bugs keep appearing at a similar rate and that this round's own fix introduced a liveness dead end).

## Next steps
0. **Round 4 on the v0.8 set — ½ DONE (2026-07-20): the first reviewer response (R1d) is triaged and applied → v0.9, and the formal-model round then took the set to v0.9.1 (FM-1); the second reviewer response (R2d) is still pending.** When R2d arrives: triage per step 2 **against v0.9.1** — the feedback was written against v0.8, so first check each item against the R1d resolutions (ADR §3 R1d block) **and the FM block** and mark already-fixed items as such rather than re-applying; genuinely new findings get R2d tags; pay attention to any *disagreement* with an R1d ruling **or the FM-1 resolution** (that is a real finding, not noise). The original **reviewer packet** (kept for the R2d reviewer): primary review targets, in read order — `build-spec.md` (normative core; every requirement + tag, with the `[→ …]` conventions in its header), `state-machines.md` (the seven kernel machines — the round-3 ask; includes FLAG-1…7), `venue-profiles.md` (dated venue facts), `tutorial.md` (spec↔tutorial consistency has caught real findings every round). Context, read-to-orient-not-critique — `HANDOFF.md` (this file: locked decisions + what is deliberately bracketed) and `adr-history.md` (ADR §3 traceability so resolved findings aren't re-raised; ADR §4 what-moved-where + the FLAG list). **Explicit asks:** (1) rule on each of FLAG-1…7; (2) check the split introduced no drift (any place core/SM/Profiles disagree is a finding); (3) new findings under fresh tags R1d/R2d. The review records in `docs/review/` are reference only.
1. **Then implement** per spec **§12 milestones** (reordered): foundation skeleton → read-path + data-capture + minimal research sim clearing the `$1K` economic gate (BTC, one venue) BEFORE the OMS → OMS write-path (+ **model-test the ambiguous-order state machine**) → account/control + risk + reconcile-only-boot + host-independent protection + `Fence` → full backtester + economic gate → sub-`$1K` canary → `$1K` → strategy #2.
2. **When more reviewer feedback arrives (fresh session):** triage per this file — validate each item against the spec + the verified facts (v0.7 review record §2), apply the valid ones, **silently discard false positives, correct overstated framings**, re-run a Codex convergence round if non-trivial, **disambiguate new tags** (round N → a fresh prefix so the traceability table stays collision-free — **R1/R2, R1b/R2b, R1c/R2c, R1d are taken; the pending second round-4 response → R2d; a future round 5 → R1e/R2e**), bump the version, keep the traceability table (`adr-history.md` ADR §3) current.

## Status notes
- Branch `docs/funding-arb-spec` history: v0.7 (16e995e) → v0.7.1 (6d211cd — the frozen content blob) → doc-split design (78bfd87) → doc-split plan (540f18b) → **doc-split execution, Tasks 1–5** (`ba1e4ad`…`2bf55de`) → **Task 6** (`5548f19`) → **Task 7 fix** (`72a3ef2`) → **polish** (`d67708e`) → HANDOFF touch-up (648db00) → **v0.9: round-4 R1d triage + Cx5 Codex convergence (e3a9266; three Codex rounds: round-1 NO-SHIP findings applied → round-2 caught the undefined retry bound + a premature-convergence claim, both applied → round-3 scoped re-verify — verdict recorded in the review record's Cx5 section)** → **v0.9.1: formal model (b)** (design 3e8daf7 → model + TLC results 56e4b0b → FM-1 resolution across the doc set, d587b6e) → **Cx6 mid-round wip** (36695d9) → **v0.9.2: Cx6 convergence completed on the model + FM-2 + spec edits + 7/7 re-verification** (9773224) → Cx7 findings recorded-but-unapplied (f5b7411) → **v0.9.3: Cx7 applied — FM-3 spec edit + Cx7-1/-2/-3 model/suite fixes + 7/7 re-verification** (246e22b) → **v0.9.4: the genuine independent Codex round-2 (Cx8) — NO-SHIP triaged, FM-4 spec edit + Cx8-2/-3 model/suite fixes + NC5 (the first negative control over a temporal property) + 8/8 re-verification (this commit; the round-2 re-review obligation is discharged — no formal-model debt outstanding)**. Per the user's rule, **no AI attribution** in commits / authored content.
- Verified facts (APIs/market/tooling) checked 2026-07-18 (v0.6), 2026-07-19 (v0.7), and **2026-07-20 (v0.9 — Bybit forced-close schemas + margin-op/transfer evidence, verbatim; Binance items corroborated-not-verbatim, ⚠-flagged in Profiles §4 as browser-confirm verify-items)**; verification subagents complete and terminated.
