# Design record §62 — M3 quantization: §2.4's trading filters enter the order path

**Date:** 2026-08-03 · **Sessions:** 91–92 · **Docs:** v0.9.41 · **Tasks:** #182 (closes), #183/#184 (filed)

The slice was chartered as "quantization" and turned out to be **verification**:
the deriving quantizer (`cashcarry.TargetQty`, §40g) had existed for weeks — on
the scoring path only — while `LegIntent.Quantity`'s doc asserted *"§6 step 4
has already quantized it"* with no code making it true, and §2.4 mandated only
the LOT intersection. What landed is the verification half: the venue's
published filter set (lot step, min quantity, price tick, min notional) as
`oms/domain.TradingFilters`, one shared conformance check
(`oms/application/filters.go`), and two enforcement points — the Planner per
leg before anything is journaled, the Executor's preflight per package before
any authorization fsync.

---

## §62a — verify, never derive: the split and its reason

The deriving quantizer STAYS in strategy: rounding to the intersection grid and
judging the residual against its tolerance is §6 step 4's sizing duty, owns a
tolerance policy, and produces the number the Decision journals. The order path
VERIFIES and refuses. Rounding there instead would silently repair a
disagreement between sizing's idea of the grid and the venue's — precisely the
disagreement worth surfacing — and would dispatch a quantity the caller's own
records do not describe.

The harm class that makes verification load-bearing rather than pedantic is
**asymmetric leg rejection**. Each leg is judged by its own venue against its
own grid, and the grids differ inside one symbol on one venue: BTCUSDT lot
steps differ 100× (Binance spot 0.00001 vs perp 0.001), ticks 10× (0.01 vs
0.1). An off-grid package sent anyway is not an error message — it is one leg
filled and one refused, i.e. naked exposure. Both enforcement points are
therefore **package-atomic**: refused whole with nothing journaled (plan) or
nothing fsynced or sent (preflight).

Two spec holes came out of stating this: the tick check and the min-notional
refusal had **no normative clause anywhere** — §2.4 mandated only the lot
intersection. Both were invisible to the OWED grep because the clauses were
absent rather than departed from; the reading pass found them. §2.4's
conformance bullet and §6 step 5/6's clauses landed this slice (v0.9.41), and
the `[SPEC EDIT OWED]` marker came out the same session.

## §62b — placement: `oms/domain`, forced by `singlewriter`

`TradingFilters` lives in `oms/domain`, not `oms/application/ports`, and the
placement is load-bearing: §3.6 rule 6 forbids every package outside the
context — composition roots included — from importing `oms/application/ports`
and `oms/infrastructure/driven`, while a root must STATE the filters (it is the
layer that resolves them from the reference revision). The domain package is
the one the gate deliberately leaves nameable — the `InstrumentAssets`
precedent, §5.1's pattern.

**Rejected:** a `ports` type (roots cannot name it); a `platform` type (the
kernel must not know venue trading vocabulary, and §11 forbids the dependency
direction that would tempt).

## §62c — one shared check, nil map fail-closed

`validateFilters` (construction: every entry usable, or a refusal naming the
wiring) and `conformsToFilters` (per leg: entry present, quantity on step and
above min, price on tick, notional at the leg's own limit price above the
floor) are ONE implementation consumed by both use cases — two statements of
"on the venue's grid" is how a plan-time pass and an execute-time refusal come
to disagree by one lot step.

Decisions inside the check, each with its rejected reading:

- **Absent entry refuses** (`ErrFiltersUnknown`) — absence-as-freedom is the
  flattering direction, and the accident it admits (an instrument added to the
  config but not the reference file) is the realistic one.
- **A nil filters map is legal and plans nothing.** The zero value had two
  candidate meanings — "verify nothing" and "plan nothing" — and the first
  reads as a working deployment with the one safety check silently absent.
  Rejected: requiring a non-nil map at construction, because the reconcile
  binary legitimately has no filters (it plans nothing by design) and a forced
  dummy map is a lie the next reader believes.
- **Notional rounds `RoundTowardZero`** at the leg's own limit price (the
  quantity the venue's filter reads for a limit order): the only window
  rounding can open is refuse-what-the-venue-accepts, never the reverse. The
  collapse checklist caught the fixture dimension here — every natural fixture
  is scale-exact, where all rounding modes agree — so a dedicated test
  (`TestNotionalRoundsTowardZeroAtTheFloor`) uses a scale-10 product where
  TowardZero and HalfUp give opposite verdicts.

## §62d — the preflight re-verifies, and why (Codex review, accepted adapted)

Plan is the only way an order enters the system today, but the repository is
also rebuilt by §7.5's replay, and a replayed `StatePlanned` record was
conformed by whichever filters revision (or pre-filters build) planned it.
§5.3's doctrine for such records is replan-or-discard, never dispatch; the
preflight check is that doctrine made structural for the package path. It runs
as a SECOND pass after the package-shape checks, so a mixed-base package keeps
its sharper `ErrPackageQuantityMismatch` diagnosis rather than surfacing as a
filters/asset disagreement on whichever leg the first pass reached.
`TestExecuteRefusesAReplayedOffGridLegWhole` plants replayed-shape records
directly and kills the loop-dropped mutant.

The Codex review (session 91, Codex session `019fc8f3-746b-7313-83b7-5cd7f0a9530a`)
contributed two accepted hardenings and two rejections worth recording:

- **Accepted (major #1): the staleness bound.** Filters move server-side
  without notice, and every check this slice adds passes against a stale row
  while the venue judges the current one — the asymmetric-rejection window
  re-opened behind a green run. Landed as `Reference.MaxAge` on the exercise
  config: a declared operator tolerance, checked per resolved row at startup,
  the fix for tripping it being a fresh probe rather than a wider bound.
- **Accepted (major #2, adapted): preflight re-verification** (above). Codex
  proposed it for replayed records specifically; it landed package-atomic and
  unconditional, which subsumes the replayed case.
- **Rejected: persisting the authorizing filter revision per order.** Overkill
  for a one-shot binary: the preflight re-verifies against the CURRENT
  process's filters, which is the stronger check for this deployment — the
  stored revision answers "what did the planner believe", the preflight
  answers "is it safe now", and only the second gates a send. Revisit when the
  long-running engine (#183) makes plan-to-dispatch gaps real.
- **Rejected: a conformance check in `Dispatcher.admit`.** The single-order
  dispatch of a replayed record is doctrine-forbidden (§5.3 replan-or-discard)
  and has no current caller; the admission read sits below the fsync boundary
  such a check belongs above; and threading filters into the dispatcher
  touches ~23 construction sites for a path nothing exercises. Documented on
  `Module.Dispatch`; lands with the compensating-trade slice as **#184**.

## §62e — the resolver lives in the binary that plans, and the fence proved it

The exercise root resolves the map at startup: read the committed reference
file, build the strategy `ReferenceStore`, resolve every configured
instrument's `LotRules` as-of now, refuse a missing row, a tick-less row
(`ErrFilterTickUnrecorded`) or a stale row (`ErrFilterRowStale`) — each naming
the file and the instrument, before any venue call or fsync. Everything-or-
error, because skip-the-bad-row reports the wiring defect on a real order's
refusal, attributed to the trade instead of the file.

The first placement attempt put the resolver in `app/reconcile` and the
root's `imports_test` refused it — §7.5's boot must not acquire a collaborator
from a context whose posture it has not decided, and the fence caught exactly
the import it exists to forbid. That is evidence the fence works, recorded as
such. The finished map reaches the boot through a new
`reconcile.WithInstrumentFilters` **Option** — not a Config field, so a
reconcile-only deployment cannot even express the dependency, and its nil
default is the plans-nothing meaning from §62c.

## §62f — strategy's `Tick` is optional where the OMS's is not, on purpose

`strategy/domain.LotRules.Tick` is OPTIONAL (zero = unrecorded): revisions
journaled before the field existed must keep decoding, and the scoring path
never prices with a tick. The order-path resolver REFUSES a tick-less row —
the guard restated in terms of its consumer rather than widened in the domain.
This is the two-folds-opposite-defaults rule applied at design time: the two
consumers of "no tick recorded" have opposite safe directions, so the
discrimination lives at the consumer, not in the shared type. A PRESENT tick
is held to full coherence (positive, asset-checked against step and notional);
a declared `"0"` is refused at load — absence and zero are different claims.

## §62g — the reference file: ticks land, `observed_at` does not move

`configs/reference.btcusdt.json`'s four lot rows gain `"tick"` (0.01 / 0.1 /
0.1 / 0.1) with the 2026-08-03 probe cited in each row's `source` string —
while `observed_at` stays **2026-07-31**, deliberately: the committed
calibration's frozen as-of (the run_test freeze at 2026-07-31T12:00) must stay
answerable, a single-revision store carries ONE date per row, and moving it
forward would orphan the milestone-2 evidence chain. §40k-2's archive is the
real fix; do not move the date as a tidy-up.

Live-probe facts banked in Profiles §5's dated block: all four ticks; every
2026-07-31 lot/notional row re-confirmed; fapi's `exchangeInfo?symbol=` is
IGNORED (full ~1 MB roster answers — a fetcher must filter client-side);
Binance spot's `MARKET_LOT_SIZE` publishes a literal ZERO `stepSize` (the
zero-reads-as-no-constraint case arriving from real data, which is why
`TradingFilters.Validate` and `LotRules.Validate` both refuse it); the perp's
min-notional field is a fourth spelling (`notional`).

**The operational finding, surfaced loudly rather than absorbed:** the
re-confirmed **50 USDT** USDⓈ-M `MIN_NOTIONAL` floor means §9's $20 live test
CANNOT run on the Binance perp as imagined. Either the package grows to ~$55,
or the perp leg moves to Bybit — blocked on #171's signer validation. That
choice is the operator's. Related: the committed rows are MAINNET-observed, so
a testnet run may meet different filters — the failure mode is a refused
cycle, not exposure, and the mainnet run is the correct target.

## §62h — evidence

**Collapse-checklist pass (before the battery):** grids everywhere are
non-powers-of-ten (0.005/0.25 in fixedpoint, 0.025/0.05/25/20 in the planner
fixtures) because at a power of ten "whole multiple of step" and "few enough
decimals" are the same predicate; the notional rounding dimension was
collapsed on every natural fixture (scale-exact products) and got the scale-10
edge case; the `MaxAge` boundary got fixtures on BOTH sides of `age == bound`
(exactly-at passes — the check is `>` — one second over refuses), without
which `>` and `>=` are the same predicate; the two instruments' filters differ
in EVERY member so cross-wiring is distinguishable from correctness.

**Mutation battery: 18/18 caught.** OnStep/OnTick: `<=0` vs `<0` on the step
guard, `%` vs `/` on the grid test (both twins). conformsToFilters: each of
the five refusals dropped, the min-qty boundary flipped (`<` vs `<=` — killed
by the at-the-minimum fixture), `RoundTowardZero` → `RoundHalfUp` (killed by
the scale-10 test). Preflight second-pass loop dropped; both constructors'
`validateFilters` dropped — which exposed a test gap first (no construction-
refusal test existed; `TestConstructionRefusesAnUnusableFiltersEntry` landed
before the mutant ran). Exercise resolver: tick-required dropped, staleness
`>` → `>=` (killed only by the boundary fixture) and inverted, resolved row
dropped from the map. Five NOBUILDs during the run were all bad seds
("declared and not used" — the mutation orphaned the variable it should
consume) and were re-aimed with variable-consuming always-false forms. Two
mutants first reported `TIMEOUT!!`: the chaos test's own 60s arrival bound
loses the race against `mutate.sh`'s default 60s suite bound — re-run with
`MUTATE_TIMEOUT=180s` so the in-test bound fires first and names the failure;
the harness knob exists for exactly this and the finding is recorded here
rather than by shortening a bound sized for a subprocess compile-and-boot.

**End-to-end:** `TestRunRefusesAnOffGridTradeBeforeAnythingIsSent` boots the
real sequence against a fake venue with a spot-grid-legal, perp-grid-illegal
quantity and asserts the whole thread-through (exercise resolver →
`WithInstrumentFilters` → `oms.Deps` → planner refusal → `ExitIncomplete`,
zero Places, refusal naming quantity, step and grid) — which is also why the
reconcile-side option test only needs to pin the Option wiring itself.
