# HANDOFF — Funding-Rate Arbitrage Platform

## THE IMMEDIATE NEXT ACTION: **THE TWENTY UNSAMPLED §8 PROVENANCE ROWS. THE CHECK IS `ls`, AND IT HAS NEVER BEEN RUN ON ANY OF THEM.**

**§7's REVIEW IS FINISHED.** Round 5 read **Venues**, **Evidence** and **System** — the three screens no round had reached — and found five: **F60**–**F64** (record §100 §15.10, ADR rows, §14 debts 10–13). **No code, no venue contact, no normative edit; `build-spec.md` is unchanged at v0.9.74.**

**§100 IS STILL NO-SHIP, and the open list is now twelve.** F52–F57 from round 4 and F60–F64 from round 5; F51 is the only one closed. **No implementation plan and no `serve` package exist**, and that remains a preference rather than a gate — `build-spec.md:821` clause (a) has held for the entrypoint since F51 closed.

**WHAT ROUND 5 FOUND, AND THE ONE SENTENCE WORTH CARRYING.** The cross-tabulation instrument that found F53 in four commands was run to exhaustion, and then one step further: *for every quantity that IS present in all three lists, open the source and check it holds the quantity.* **Four of five findings came from that second step.** A cross-tabulation proves a quantity is *claimed* consistently; it never proves it *exists*.

| # | Finding | Severity |
|---|---|---|
| **F60** | The **Evidence screen's headline** declares its hurdle three ways and its entry level zero ways, and the published column is the **most favourable** of four | MAJOR |
| **F61** | **Leverage brackets** are sourced to a directory holding an arithmetic **oracle** and no data | MAJOR |
| **F62** | **"Formal-model freshness"** is a five-verdict gate rendered as a boolean, green side = *"the runs are void"* | MAJOR |
| **F63** | **Posture** is stamped `1 / 2` and every reader it names is **signed** | MAJOR |
| **F64** | **Connectivity** has no source in the record or the tree | MINOR |

**⚠ F60 IS THE ONE THAT TOUCHES THE KILL GATE'S PUBLISHED NUMBER.** `summary.json`'s `verdict` block — the one F45 fixed, now carrying four hurdle-provenance fields — is computed at entry level **`0.001`** and records no level. Measured with a passing control (level `0.001` reproduces the committed block exactly) and a passing negative control: `beat_hurdle_symbols` at 21 bp is **2/61 · 3/60 · 4/56 · 3/49** across the four levels the file's own header declares, and the published `4/56` is the unique **maximum**. At **90 bp** the published `0/56` becomes **2/49** at level `0.002` — a qualitative claim flipping on an axis the artifact does not hold. **Halve step-1 numbers before a hurdle comparison (F30), and now also: state the entry level.** §99 §5a's table declares the capital denominator and not the level, so the figure travels into this document and the ADR without it.

**⚠ AND THE FORMAL MODELS ARE KNOWN-STALE RIGHT NOW — 6 OF 16 TRACKED SECTIONS, BOTH MODELS, OWNED BY #229.** `make formal-fresh` is green and prints nothing about them, because `MarkedStale` narrates through `t.Log` and the target runs `go test` without `-v` (measured: 1 occurrence with `-v`, 0 without). One of the six is `state-machines.md §2`, a **subject** section of `AmbiguousOrder`. Neither model has been re-verified since **2026-08-02**. This is not new drift and nothing is failing — it is marked, not absorbed, exactly as §88 intends — but it is invisible, and F62 is what made it visible.

**THE NEXT ACTIONS, IN ORDER.** (1) **The twenty unsampled §8 provenance rows**, against **both** schemas and the venue-semantics comments *and* an `ls` of every named directory. The error rate on sampled rows is now **four in twelve** (F46 ×2, F56, F61), and F61 is the first found by opening the named path rather than by reading either source. (2) **`ReconcileMarginOp`'s actor identity** and **F42's `notimenow` hole** — the only §15.7 items still untouched, two rounds on. (3) **§14's debt list re-read against the file rather than against its findings** — F60 is a debt written too narrowly, and debts 8 and 9 have not had that treatment. (4) **Mockups for the three screens this round read**; F36 and F52 both landed on Overview because it is the only screen drawn, and F52's lesson is that a mockup regenerated from real data is what refutes the prose.

**WHAT MUST NOT BE RE-LITIGATED WITHOUT A NEW ARGUMENT.** Every *Rejected alternative* in §100, §101 and §102, plus round 5's two: re-pointing §8's bracket row at `230-adverse-close/captures/` as the whole remedy (it fixes the citation and leaves a venue-wide panel claimed from three credentialed snapshots of one closed position), and fixing F60 by adding a `level` field to each `per_symbol` **row** (it leaves `verdict`, a separate top-level emission, unchanged while making the block above it read as audited).
## Status (2026-08-29, session 146 · first subject): **PERSISTENCE IS MEASURED. THE SPREAD WAS NEVER THE BINDING CONSTRAINT — THE COST IS, AND IT IS THE QUANTITY THIS PROJECT HAS MEASURED LEAST. NO CODE, NO VENUE CONTACT, NO NORMATIVE EDIT.**

**Read `docs/superpowers/specs/2026-08-29-persistence-measurement.md` (record
§99)** — the measurement, three controls, and five findings. ADR rows **F26**,
**F27**, **F28**, **F29**, **F30**. Artifacts in `research/persistence/`.

**WHAT WAS MEASURED.** 17 478 episodes over 61 eligible symbols × 4 entry
levels, 235 073 matched settlements, 2020-10-21 → 2026-07-31, from the data
`screen run` had already fetched. Nothing new was fetched. Three controls:
replay == `screenLevel` (244/244), `ScreenPair` == committed `ranking.json`
(244/244), and a **negative** control showing the comparison can fail
(248 ≠ 131).

**THE ANSWER TO §2.3's QUESTION.** Median episode is **2 settlements** — 8h on a
4-hour grid, 16h on an 8-hour grid. Break-even is ≈6 settlements, which sits at
about the **p93** of the run-length distribution. The typical episode does not
reach it.

**⚠ F26 — THE VERDICT IS A FUNCTION OF AN ASSUMPTION, NOT OF A MEASUREMENT.**
`RoundTripCost()` is `0.0021` and holds **four taker legs and nothing else** —
no spread crossing, no slippage. Every `net_annual` and the shortlist itself are
scored against it. Mean net per episode is positive at two entry levels at
**21bp**, at one at **29bp** (`build-spec.md:53`'s own model), and at **none**
at **90bp** (§98's live figure, which §98 says is not a measurement).

**⚠ F27 — 84% OF THE NET IS ONE SYMBOL.** At 10bp/21bp the net across 56 symbols
is `+0.6460`; `LABUSDT` alone is `+0.5439`. Dropping the best **10** of **1 259**
episodes turns the total negative. Median symbol: **−0.1%/yr**. Against an
illustrative 4.6% hurdle: **4/56** clear it at 21bp, **3/56** at 29bp,
**0/56** at 90bp.

**⚠ F30 — THE SHORTLIST'S HEADLINE RETURN HAS NO DECLARED DENOMINATOR, AND IT IS
PER *ONE* LEG.** `LABUSDT`'s `rank_score 0.71586586` is 71.6% of **one leg's
notional**; the same episodes on a two-leg deployment return **35.8%** on the
capital, and less on a true `C_allocated`. `ranking.json` declares this in no
field. §12 item 2b's own annotation and the operator's memory both quote
*"71.6%/yr … vs a 4.6% hurdle"* — the comparison §55 exists to protect, done at
2×. **Not a breach** (§2.1's clause binds the crossvenue *gate*, and §43g says
step 1 is not gate evidence) — but **halve any step-1 number before comparing it
to a hurdle**.

**F28** — run length and break-even are different questions with a five-row
overlap band; an edge bar phrased as *hold for N settlements* misclassifies both
ways. **F29** — every symbol above 4%/yr has under 530 days of history and every
symbol over 1 800 days returns ≈0; *regime* and *small sample* predict this
identically and **elapsed time is the only separating parameter**, so it is left
open by construction.

**⚠ `research/persistence/episodes.json` IS COMMITTED BECAUSE ITS INPUT IS NOT.**
`research/screening/raw/` is gitignored on §42b's sound argument, which makes the
derived file the only durable form of this measurement on a fresh clone. Do not
delete it as regenerable — regenerating needs a re-fetch of 468 venue files.

**WHAT IS AND IS NOT CLAIMED.** Claimed: the distributions and per-symbol nets
are exact readings of the committed screening data, three-way controlled.
Not claimed: anything about mainnet cost (21/29/90bp are an assumption, a model,
and one unattributed observation), anything about capital (the totals are a sum,
not a portfolio), and anything about whether F29's short-history edge is real.

---

## THE STANDING ECONOMIC NEXT ACTION: **MEASURE THE ROUND TRIP.** Not the spread again.

*(This is the milestone-level next action and it is **not** superseded by §100's
review. The dashboard work is a parallel subject; the kill gate still turns on
the number below. §100 §8b claims a book-derived per-symbol cost is part of this
measurement — that claim is itself one of the things the Codex rounds should
attack.)*

Three of §99's four findings turn on the cost. §98 already said what a clean
figure needs: **the send must follow the book read inside the book's
timescale** — a property of an automated path, not of a runbook a person
executes. Until that exists the kill gate's answer is set by an assumption whose
span covers the whole verdict (F26's table).

⚠ A second live run still needs **fresh journal paths** (`./run/pair/binance.2.db`,
`bybit.2.db`) — both journals remain latched on `GateNetDeltaAsserted` from §98,
and those files are the evidence that the gate fired. Do not delete them.

**THE OPERATOR'S STATED NEXT TOPIC WAS A WEB UI**, raised at the end of session
146. ***Scoped the same session: record §100, commit `9d7a5f3`.*** The paragraph
that stood here said it was "not yet scoped, designed or decided" and that §12's
roster has no UI item; the first clause is now false and **the second is still
true — the §12 roster entry is a debt §100 §14.1 opens.** §100's answer to "what
is a UI *for* while the gate is undecided" is that it is an instrument for the
open question rather than a readout of a settled one, and §100 §8b/§8c argue the
cost panel is a real fraction of the round-trip measurement below.

## Previously (2026-08-16, session 145): **THE PAIR RAN. FOUR ORDERS PLACED ON THE DEMOS, `VerdictIntact`, EXIT 0, BOTH ACCOUNTS BACK TO FLAT. THREE FINDINGS, ONE OF THEM STRUCTURAL. ⚠ BOTH JOURNALS ARE NOW LATCHED.**

**Read `docs/superpowers/specs/2026-08-16-m4-236-first-live-pair-run.md` (record
§98)** — what ran, the three findings, and the one measurement this run did *not*
take cleanly. ADR rows **F23**, **F24**, **F25**.

**THE RUN.** `arblab pair run -config configs/pair.demo.2026-08-16.json` drove one
cross-venue perp-perp package across two live demo deployments. Both legs filled.
`VerdictIntact`, exit 0. Then both legs were closed by hand, reduce-only. Both
venues confirm flat. `pair.id` `01a00a83-e068-7550-9dbf-13283b0cefa5` is spent.

| | Entry | Exit | Fee charged |
|---|---|---|---|
| Binance | sell 100 @ `0.08012` | buy 100 @ `0.08067` | `0` |
| Bybit | buy 100 @ `0.0806` | sell 100 @ `0.0806` | `0.008866` each side |

**⚠ F23 — A PAIR CAN BE OPENED BUT NOT CLOSED IN BAND, AND BOTH JOURNALS ARE
LATCHED BECAUSE OF IT.** `pair run`'s unwind fires only on a **half-open** pair;
a *successful* run leaves a live position the root has no command to take off.
The runbook's manual reduce-only close — written as exit-4 recovery — is the only
exit from a **green** run too, and taking it latches `GateNetDeltaAsserted` on
both sides. **No CLI lifts that latch** (`margin reconcile` is scoped to §9's
Ambiguous *margin ops*). So **every successful `pair run` ends with two latched
journals**. Nothing could have caught it: every #236 fixture had a half-open pair
or a clean refusal, and the verdict was always the end of the test — the
successful pair's *post-condition* was never a subject. The in-band close belongs
with **#241**.

**⚠ A SECOND RUN NEEDS FRESH JOURNAL PATHS** — `./run/pair/binance.2.db` and
`bybit.2.db` in a new document. Do **not** delete `binance.db` / `bybit.db`: they
are now the evidence that the gate fired.

**F24 — BOTH DEMOS MISREPORT COST, IN OPPOSITE DIRECTIONS, AND BINANCE'S DOES IT
WHILE PUBLISHING THE RIGHT NUMBER.** `commissionRate` returns
`takerCommissionRate 0.000500` — exactly the reference row — and `userTrades`
then shows `commission: 0`. Model and published field agree perfectly; only the
realized ledger disagrees, and no decision path reads it. Bybit charged `0.0011`
against a reference row of `0.00055` and refuses `/v5/account/fee-rate` with
`retCode 10001`. **§95 §8's not-claimed list gains demo FEES beside demo fills.**

**F25 — AND THE BYBIT HALF CANNOT BE SETTLED BY ANY FIXTURE.** `0.008866` on
`8.06` is equally consistent with "the rate is `0.0011`" and "the rate is
`0.00055` on twice the notional". Both readings are **linear in quantity**, so no
order size separates them. Left open deliberately; the instrument has to be the
venue's published rate.

**SETTLED BY A CONTROL, so nobody re-opens it.** The entry filled at exactly the
limit price on Bybit, which admits "the market moved" or "the demo fills at the
limit". A reduce-only sell with `orderPrice 0.078` against a bid of `0.08060`
filled at **`0.0806`** — the bid. **Bybit demo matches against its book.** The
entry fill was real drift inside the 77s between the book read and the send.

**⚠ THE ROUND TRIP COST 0.90% AND THAT IS NOT A MEASUREMENT.** `0.048` of the
`0.055` basis was paid at entry, where the four-tick buffer and 77 seconds of
drift are confounded and this run cannot attribute between them. §2.1's model is
~0.29%. **Do not carry 0.90% into a hurdle calculation.** A clean figure needs the
send to follow the read inside the book's timescale — a property of an automated
path, not of a runbook a person executes.

**WHAT IS AND IS NOT CLAIMED.** Claimed, on venue evidence: the code reaches two
venues, places two legs, sizes leg B from leg A's established fill, journals both
with **0 duplicates**, and reports a verdict two independent venue surfaces
corroborate; `GateNetDeltaAsserted` detects an out-of-band change. **Not claimed:**
anything about mainnet fills, mainnet fees, funding accrual, or the economics.

---

## Previously (2026-08-16, session 144): **THE DEMO RUN'S PREPARATION FOUND TWO §4.1 DEFECTS THAT NO TEST COULD, BOTH FIXED. BOTH VENUES NOW BOOT `ExitClean` AGAINST THE LIVE DEMOS. NO ORDER HAS STILL BEEN PLACED.**

**Read `docs/superpowers/specs/2026-08-16-m4-236-bybit-spot-margin-path.md` (record
§97)** — two defects, the reason no check in this repo could have found either,
and a hazard that was written down years ago under the wrong heading. ADR rows
**F21** and **F22**.

**WHY THIS SESSION EXISTS.** #236 landed green with nothing ever sent to a venue.
The operator asked to run the demo smoke test. **Step 1 did not finish** before
the first live read refused.

**DEFECT 1 — a doc page SLUG transcribed as an HTTP path.** The Bybit spot-margin
probe called `GET /v5/spot-margin-uta/status`, which **does not exist**: 404 with
an EMPTY body on demo *and* mainnet. Bybit envelopes every route it owns, so an
empty 404 means the request never reached the API layer — a JSON 404 and an empty
404 are opposite evidence. `spot-margin-uta/status` is the **documentation page
slug**; that page reads *"HTTP Request — `GET /v5/spot-margin-trade/state`"*. It
had reached **seven sites**, one of them `venue-profiles.md`'s verified-facts
table. **Nothing could have caught it**: the `httptest` double served the same
wrong path the code asked for, and its default arm already refused unknown paths.
**A strict fixture cannot object to a fact both sides state identically.**

**DEFECT 2 — a gate no flat account could pass.** With the path fixed, the boot
reached the posture check and refused again: three per-symbol settings
`ReasonUnobserved`. The position probe scoped by `settleCoin`, which answers
**only for symbols carrying an open position**. So §4.1's three
`SubjectDerivativesLeg` rows were unobservable on a flat account — and **flat is
the state a two-leg open requires**. The posture gate and the flat-start gate had
**no account state satisfying both**; the Bybit boot could never have passed,
independent of defect 1.

**⚠ THE HAZARD WAS ALREADY WRITTEN DOWN, UNDER THE WRONG HEADING.**
`posture_test.go`'s **Binance** test has said since record §73o: *"re-adding a
per-symbol probe against a POSITION enumeration would restore a gate no flat
account can pass."* True, general, and filed under the endpoint that motivated it
rather than under the mechanism — so nothing carried it one screen down to Bybit,
which was doing exactly that. **When a hazard note is written, check whether its
subject is the input or the mechanism; if it is the mechanism, read the siblings
at once.**

**THE FIX REVERSES A SETTLED RULING, AND A CONTROL IS WHY.** `ports.PostureScope`
ruled the symbol roster belongs to the requirement rather than to the read,
because *"a read scoped by the symbols this system already names cannot surface a
configured symbol the venue does not know."* Encoded as a control and answered by
the venue: `symbol=NOTAREALUSDT` → **`retCode 10001 "symbol not exist"`**. The
symbol scope NAMES an unknown symbol; the settleCoin scope returns the same empty
list for *"the venue does not know it"* and *"you are flat"* — **the collapse the
ruling meant to prevent was the one it shipped.** The ruling's goal was right and
its premise about venue behaviour was untested. The roster still comes from the
requirement, so the rules and the read range over one list. Cost: §73c-2's
quotable posture cost becomes **`3 + len(symbols)`**.

**WHAT IS VERIFIED NOW.** `make ci` green both modules; `SPEC EDIT OWED` at zero
across `internal/ tools/ cmd/`; `formal-fresh` untouched (`venue-profiles.md` is
not tracked, `knownStale` still **six**, all #229's). Three mutations killed on
the new code, including the one that proves the two-symbol test roster earns its
keep — with a single symbol, "one step per symbol" and "one step, always" render
an identical list. **Both venues boot `ExitClean` against the live demo hosts.**

**⚠ ONE CLAIM OF MINE WAS REFUSED BY ITS OWN BITE-CHECK,** and it is left refused
rather than quietly kept: the retired path in the endpoint-routing table is a
**findability marker, not a regression guard**. No single-line resurrection
reaches it, because `ratelimit`'s duplicate-name gate fires first. The comment
says so.

**⚠ NO ORDER HAS STILL BEEN PLACED.** Every probe this session was a signed READ.
§95 §8's not-claimed list stands unchanged: nothing here is evidence about demo
books, demo fills, demo funding, or the strategy's economics. What IS new evidence
is that the doubles proved the code consistent with itself and nothing else.

**THE OPERATOR'S ACCOUNT STATE, established this session.** Binance demo
`binance-demo-futures` — 4 908 USDT, flat, LABUSDT ISOLATED/5x, Portfolio-Margin
attestation written and pinned (`run/pair/portfolio-margin.binance.json`). Bybit
demo `bybit-demo-1` — ~50 000 USDT, flat, LABUSDT ISOLATED/5x, **key expires
2026-11-10T07:03:17Z**. Both keys re-confirmed to carry trade permission today.

**THE RUNBOOK'S DEMO-VS-MAINNET PARAGRAPH WAS BACKWARDS** and is corrected (§97
§8). It read *"For Binance that is the same environment. For Bybit it is not"*,
generalising §95 §2's demo-vs-**testnet** symbol counts to a different pair of
environments. Measured on LABUSDT: **Bybit demo and mainnet grids are
byte-identical; Binance's differ** (tick `0.0001` vs `0.00001`, step `1` vs
`0.1`). Safe in direction — mainnet's grid is coarser, so the error can only
over-refuse — but the harm was the reassuring half, which told an operator not to
check Binance.

**THE SLICE IS `aa6bf0b..107c220`, MERGED TO `main`, `make ci` GREEN BOTH
MODULES, branch deleted, tree clean.**

---

## ✅ DONE (session 146, record §99): the persistence measurement. Kept below for its runbook notes.

**The persistence question this section set is ANSWERED — see the status block
at the top of this file and record §99. The prose below is session 144/145's
material and is retained only for a second live run.**

**The run is DONE (session 145, record §98) — the whole runbook, §1 through §6.**
The section below is session 144's preparation notes, kept because a *second*
run would reuse most of it. Read the three ⚠ items at the top of this file first;
in particular a second run needs **fresh journal paths**, because both journals
are latched.

**WHY PERSISTENCE IS THE NEXT QUESTION AND NOT MORE EXECUTION.** Break-even at
today's spread is `0.29% ÷ 0.00047` ≈ **6.2 settlements ≈ 25 hours** of holding.
So the strategy's viability is a question about **how long an alt's cross-venue
funding spread survives**, not about whether the order path works — and §2.3 says
exactly that: *"edge bar set from measured spread persistence, not the headline
spread."* Today's `0.047%`/settlement annualizes to ~100%, which is precisely the
headline number that sentence exists to distrust.

It is measurable from **settled history, with nothing at risk**. `screen run`
(§42b) already fetches both venues' settled-funding history for the intersection
universe and writes a ledger, ranking, shortlist and dislocation census. **Start
by reading what it has already produced for the 8-alt shortlist** before fetching
anything new.

**THE SECOND-CHEAPEST BOOLEAN, if a live run is wanted anyway: does either demo
pay funding at all?** Untested. Both venues settle `LABUSDT` on a **4-hour**
interval at `00/04/08/12/16/20` UTC (`fundingIntervalHours: 4`;
`fundingInterval: 240`), and the instants **coincide across the two venues**, so
no window exists where one leg has settled and the other has not. Open ~15 min
before a settlement, hold across it, close after. ⚠ Given F24 — Binance demo
publishes a correct fee rate and charges **zero** — *"publishes funding, does not
pay it"* is a live possibility with precedent from this very session. Note this
buys a **boolean about the plumbing**, not an economics measurement.

---

### Session 144's preparation notes (all of it already done once)

**Steps 1, 2 and 5 of `docs/runbooks/pair-demo-run.md` were DONE in session 144**
and consumed by the run in 145:

- `./run/pair/` and both lock directories exist. `./run/` is **gitignored**.
- The Portfolio-Margin attestation is written and pinned:
  `run/pair/portfolio-margin.binance.json`, sha256
  `3eefaed93981d88f480395e693f3be40e3c7ae9048d0b8571d16f5a2dc71da33`. **It is not
  in git and must not be** — see `.gitignore`. If it is missing, the operator
  writes a new one and the digest in the config changes with it.
- The reference revision is `configs/reference.crossvenue.2026-08-16.json`, both
  venues' `LABUSDT` rows, `class: fetched`, **one** observation instant.
  ⚠ **`max_age` is 24h — refetch if the run is after 2026-08-17 09:43Z.**
- Both dry checks exit `ExitClean`:
  `arblab reconcile run -config configs/reconcile.{binance,bybit}.demo.json`.
  Re-run both first; they send nothing and they are the cheapest proof the
  account has not drifted.

**What remains is §3, §4 and §6, and §3 → §6 MUST BE ONE CONTINUOUS SEQUENCE.**

1. **§3** — read both books, set four prices. The two `limit_price` values
   marketable (sell at or below the bid, buy at or above the ask); the two
   `unwind_limit_price` values marketable in the **opposite** direction. Prices
   must be multiples of `0.0001` — the reference carries mainnet's tick, which is
   the coarser grid on Binance and identical on Bybit, so a price valid under it
   is valid on both demo hosts.
2. **§4** — mint a fresh UUIDv7 for `pair.id`. `attempt` stays 0. A re-run with
   the same id is REFUSED, which is the fail-safe direction.
3. Write `configs/pair.demo.<date>.json` from `configs/pair.demo.example.json`,
   with `account` **`binance-demo-futures`** / **`bybit-demo-1`**, the
   attestation block above, and `reference.path` pointing at the revision.
4. **§6** — `arblab pair run -config configs/pair.demo.<date>.json`.

**⚠ THE FAILURE MODE THAT LOOKS LIKE SUCCESS.** A limit that is not marketable
turns the IOC into a no-fill, which is a **clean** run under D6 — exit 0, verdict
`VerdictUnopened` — and proves nothing about fills. It is the most likely way to
get a green run that measured nothing, and it is why §3 and §6 cannot be
separated by a conversation turn spent on anything else.

**⚠ EXIT 4 MEANS THE PAIR IS EXPOSED** and nothing fixes it automatically: §6.7
is the single gate for every compensating trade and cannot see a cross-venue pair
(`domain.HedgeGroup` is `(Account, Venue, Symbol)`). Recovery is `reconcile run`
on each side, then closing the exposed leg **by hand**, reduce-only. Never re-run
`pair run` with the same trade id.

**STILL OPEN, carried forward.** `[OPEN, §98 §3]` **no in-band close exists for
an intact cross-venue pair**, so both journals are latched and a second run needs
fresh paths — it belongs with **#241**, which owns the classifier a compensating
close would need. `[OPEN, §98 §5]` **Bybit's demo taker rate is undetermined**
between `0.0011` and `0.00055`-on-double-notional, and no fixture can settle it —
only the venue's published rate, which refuses with `retCode 10001`.
`[OPEN, §98 §8]` **does either demo pay funding at all** — untested, and F24 makes
"publishes it, does not pay it" a live possibility. `[OPEN, §97 §9]` **no gate
reads any path constant against its venue** — session 144 falsified one by hand
and checked exactly one; an absence of further findings is an absence of
looking. Capture
still down (~9 GiB/day accepted, pending Hetzner) — #239/#240 unmoved. #225/#226
remain B4's only blockers. #229 unchanged, still owing SM §1's model-(a) revision
plus the `ambiguous-order-model.md` §1 row. #235 carries seven; #220, #227, #228,
#238, #241 unchanged. The `CLAUDE.md` slice-close grep widening to
`internal/ tools/ cmd/` is **still owed to the operator**.

---

## Previously (2026-08-16, session 143): **#236 IS BUILT — `internal/app/pair` DRIVES ONE CROSS-VENUE PACKAGE ACROSS TWO DEPLOYMENTS, `make ci` GREEN BOTH MODULES, `SPEC EDIT OWED` AT ZERO. NO LIVE RUN WAS PERFORMED.**

**Read `docs/superpowers/specs/2026-08-15-m4-236-crossvenue-root-scoping.md` §10**
— it holds what was built against each ruling, the three markers that keep
alternative (v) visible, the five owed items, and **five findings the plan did not
predict**. The plan is
`docs/superpowers/plans/2026-08-15-m4-236-crossvenue-pair-root.md`; every one of
its fourteen tasks is landed.

**DO NOT RE-DERIVE THESE.** Record §95's three rulings are **settled**; its three
unknowns are **measured** (§95 §2, dated 2026-08-15); and the pair concept lives
**in the root, not the kernel**, by ruling 3's split. `domain.HedgeGroup` is
untouched, §6.7 is unedited, and `Module.Classify` still has **no production
caller** — all three deliberately.

**WHAT LANDED.** `4a54686` the two `oms` projections a root behind the
`singlewriter` fence cannot otherwise take → `8dc4345` the pair concept
(`Verdict`, `Compose`, three-valued `Establish`) → `5851983` the two-sided
document and the two refusals its shared `TradeID` rests on → `0c63c03` per-side
filters from one reference revision → `1ddadb9` two boots and the
both-clean-and-flat gate → `4e90a1c` the sequential drive → `5ae7a88` the unwind
and the two readings it refuses → `1a62faa` `ExitClass` with `ExitExposed` →
`4b2f591` the import fence → `1201ccd` `arblab pair run` → `30ade99` the demo
document and runbook → `7d4c363` paired mode.

**⚠ WHAT #236 SHIPS IS §95 §7's ALTERNATIVE (v), WITH THE MARKERS ON.** The root
composes a verdict **above** the hedge group and leaves cross-venue
*classification* owed to **#241**. §95 §7 (v) warned the root must not silently
*become* (v) with no marker; the three markers are `pair.go`'s package comment,
`Verdict`'s own doc, and `ErrUnwindRefused`/`drive.compensate` — each naming §6.7
and §95 §4 by section. **The one compensating trade this root issues runs on
exactly one reading** (half-open, side A, fill established); skewed and
indeterminate end at `ExitExposed` for a human, because sizing that residual is
§6.7's job and §6.7 cannot see this pair.

**THE SPEC EDIT IS LANDED, NOT ROUTED.** `build-spec.md` **v0.9.70 → v0.9.71**:
§5.2's OMS block now says it describes ONE deployment on ONE venue and that a
cross-venue package is **two** blocks under one root — with the two rules that
make one shared `TradeID` safe (**different venues, different PHYSICAL journals**,
the journal one an *identity* check because a relative/absolute alias, a symlinked
parent and a hard link all resolve to one chain) — and §12 item 7 carries the
scope correction "live wiring" hid. **`grep -rn "SPEC EDIT OWED" internal/ tools/
cmd/` is at ZERO.** Neither edited section is tracked by `formal-fresh`, so no
model stamp moved and **no new `knownStale` marker exists: still six, all
#229's.**

**⚠ NO ORDER REACHED A VENUE.** Everything ran against `httptest` doubles. §95 §8's
not-claimed list stands **entirely unchanged** — nothing here is evidence about
demo books, demo fills, demo funding or the strategy's economics. The live run
follows `docs/runbooks/pair-demo-run.md`, whose §8 restates the same list; note
its two dated limits, that **the Bybit demo key expires 2026-11-10T07:03:17Z** and
that `reference fetch` reads the **mainnet** public hosts, which is the same
environment on Binance and a different one on Bybit.

**THREE FINDINGS WORTH CARRYING PAST THIS SLICE.** (1) **A guard duplicating a
downstream check reads as tested and is not** — mutating away "leg A filled
nothing → do not send leg B" SURVIVED, because the planner refuses a zero-quantity
leg anyway, so the Place count is identical and only the **refusal** differs.
(2) **An unestablished leg was always also flat** in every fixture the plan
specified, so the `Established` guard was equivalent to the flat guard; the
isolating fixture holds side A's order live **with 60 already executed against
it**. (3) **Bybit's metering shape is not Binance's** — no per-endpoint weight at
all, a closed eight-endpoint budget roster, and a §4.1 posture read that rides the
**spot** family even on a perp-only deployment. A cross-venue document is the
first one that has to carry both, and copying one venue's rows under the other's
name is a fixture asserting a false external fact.

**THE NEXT ACTION IS THE OPERATOR'S CALL, and there are three candidates.**
(a) **Run it on the demos** — the runbook is written, and it is the only thing
that turns any of this into evidence. (b) **#241**, the normative re-scope of
§5.1/§6.7 the three markers point at, which also owns the durable-pair-record
question. (c) **#237**, the post-cutoff funding fetch — still cheap, still
deadline-free, still the thing most likely to be forgotten.

**STILL OPEN, carried forward.** The capture is **still down** by the 2026-08-15
decision (~9 GiB/day accepted, pending the Hetzner host) — #239 and #240 unmoved.
#225 and #226 remain B4's only blockers and **neither blocked #236**; §94 §10's
deferred review round still reopens when #225's port is next picked up. #229 is
unchanged and still owes SM §1's model-(a) revision plus the
`ambiguous-order-model.md` §1 row. #235 carries seven; #220, #227, #228, #238 and
#241 unchanged. **Six `knownStale` markers, all #229's — five on
`AmbiguousOrder`, one on `FenceTakeover`.**

**ONE ITEM STILL OWED TO THE OPERATOR, unchanged from session 142.** Widen
`CLAUDE.md`'s slice-close grep to `internal/ tools/ cmd/` — this session ran it
that way by hand and it is how the `cmd/` marker would have been found. No marker
is missing today; only the pattern's reach is short, and it is still neither a
`make` target nor in CI.

---

## Previously (2026-08-15, session 142): **#242 IS CLOSED — A PLACED ORDER NOW REACHES A TERMINAL, AND THE FINDING WAS A MISSING SM §1 *ROW* RATHER THAN A MISSING *CALL*. THE SLICE IS `e581831..6449b43`, MERGED TO `main`, `make ci` GREEN BOTH MODULES. **#236 IS UNBLOCKED** AND IS THE NEXT ACTION.**

**Read `docs/superpowers/specs/2026-08-15-m4-242-order-finality-decisions.md`** —
it holds the sharpened finding, the one departure from the parent record and the
operator ruling behind it, three rejected alternatives with their grounds, the
reading pass the `SPEC EDIT OWED` grep cannot perform, and four items routed to it
by controller ruling. The finding itself stays in
`2026-08-15-m4-236-order-finality-gap.md`; the plan is
`docs/superpowers/plans/2026-08-15-m4-242-order-finality-transition.md`.

**⚠ THE FINDING SHARPENED, AND THIS IS THE HALF WORTH CARRYING.** Session 141
diagnosed *a transition with no driver* — the state machine declared and policed
while nothing called `ApplyState`. True, and not load-bearing. **SM §1 gave
`Submitted` no query-resolution row at all**, so a driver written against
`CanTransition` would have been **refused by the domain** on the ordinary outcome
of every order this system sends: `Filled`, `Canceled` and `PartiallyFilled` —
three of the five readings the ACL can produce — had no legal target from
`Submitted`. Two places already required the row and neither reached it (SM §1's
own *"`DispatchAuthorized` **or beyond** is resolved only by query"* invariant,
and core §6.7's ambiguous-residual list, which names `Submitted`). That is the
**third axis** of v0.9.11's finding and **the most reachable** of the three — the
other two states are entered by a crash or a transport failure, this one by every
successful `Place`. `state-machines.md` v0.9.60 → **v0.9.61**; `build-spec.md`
**unchanged**, because SM §1's normative owner already stated the requirement,
which *is* the finding.

**WHAT LANDED.** `9b87b17` the SM §1 row → `eb295fa` `Resolver` admits `Submitted`
(a **derived** bonus fix: `Submitted → AbsentAfterFinality` becomes legal, so a
placed order the venue holds no record of can now be resolved by §5.3's rule at
all) → `01d27ce` the join journals and applies the venue's reading, LAST, after
the cross-check, and **even when the cross-check disagreed** (§5.3 keeps state and
ledger orthogonal) → `e47795f`/`5807217`/`e00a7a7`/`c120ee7` the new
`Contradicted` class reaches **all four** enumerations of the join's blocking
findings and the operator report renders them → `cb7b8f7` five `Place`→terminal
walks plus the replay round-trip → `41ec395`/`d7d2075` SM §1 becomes a tracked
**premise** of formal model (a).

**⚠ THE NEXT ACTION: #236.** Its plan's `BLOCKED` block is replaced with the
dependency-landed note. Two of its tasks change and nothing else does: task 6's
`drive.establish` now reads a terminal state the join actually produces, and task
5's `venueDouble` gains the venue-status responses the advance consumes. **D4's
mechanism sentence is corrected in two places**: `Module.Join.Join([id])` now
advances the leg's state so `Module.LegState(id)` reads a terminal one, and D4's
blocking-list enumeration (`Absent`/`Incomplete`/`Unattributed`) was total when it
was written and is not now — **`Contradicted` joins it**. #236 is not re-planned
here.

**THE GATE FINDING, AND IT IS OWED TO THE OPERATOR.** This slice put
normative-weight prose in **`cmd/arblab/reconcile.go`** — claims about
`joinOutcome`'s ladder order, which findings latch, and what `Blocking()`
includes — and the slice-close grep visits `internal/` and `tools/` only. Nothing
in `cmd/` is wrong (all of it was read; the totality claim there is *enforced by a
reflection test* rather than asserted), so **no marker is missing**. What is
missing is reach: a `[SPEC EDIT OWED]` landing in `cmd/` today would let a slice
close over an undischarged deviation. **Verdict: a pattern change, one word —
`grep -rn "SPEC EDIT OWED" internal/ tools/ cmd/`** — on the exact precedent that
put `tools/` in the pattern. **Not made here**: the pattern's only home is
`CLAUDE.md`. And a third limit found while checking the second: the grep is **not
a `make` target and not in CI**, so beside "cannot see an unmarked claim" and
"cannot see a directory nobody added" sits **"does not run unless somebody
remembers"**. Proposed, not built: a `spec-owed` target inside `make ci`.

**A COUNT WORD THIS FILE HAS CARRIED WRONG FOR SIX SESSIONS.** *"Five
`knownStale` markers, all #229's"* is right for **model (a) only** and does not
say so — it omits `FenceTakeover`'s `state-machines.md` §7 marker, which is also
#229's. The manifest holds **six**, has held six since `2b2acae` (2026-08-09), and
**#242 added none**: the SM §1 premise row was stamped **fresh**, on the footing
`FenceTakeover`'s `build-spec.md` §6 and §7.5 rows already use — a premise
re-derived rather than re-run, because a `knownStale` marker asserts an
*un-discharged* delta and deferring one inside the mechanism built to prevent FM-7
would be FM-7. **Six markers, all six #229's — five on `AmbiguousOrder`, one on
`FenceTakeover`.** Task 7's brief independently said *"six: five #229's plus this
one"*, which is the right total reached by adding a marker that does not exist to
a count short by one. Two errors, opposite directions, cancelling: **re-derive the
decomposition, not the total.**

**THE `formal-fresh` FINDING, which is quieter than the failure that was
expected.** The gate stayed **green through Task 1's edit to SM §1** — not because
it healed, but because `state-machines.md` §1 was tracked by **neither model in
neither role**. The gate was green about a section it does not watch. #229 now
owes **two** things on SM §1: its scheduled model-(a) revision, and — new —
**a §1 row in `docs/formal/ambiguous-order-model.md`**, which never mentions the
section the manifest now tracks. The re-derivation's own result is the sharp
part: the fairness conjunct is **jointly sourced**, §2 states the promise and §1
is the only place that says the edge exists, so §1 is the **stricter** co-source —
and this slice's `Submitted` delta is the proof, since §2 alone would have
licensed a fairness the table granted no exit for. Nothing green goes red.

**STILL OPEN, unchanged from session 141 except where noted.** The capture is
**still down** by the 2026-08-15 decision (~9 GiB/day accepted, pending the
Hetzner host) — #239 and #240 unmoved. #237 (post-cutoff funding fetch) is still
cheap, still deadline-free, still the thing most likely to be forgotten. #225 and
#226 remain the only B4 blockers; **neither blocks #236, and #242 no longer does
either.** §94 §10's deferred review round still reopens when #225's port is next
picked up. #241 unchanged. #235 carries seven; #220, #227, #228 unchanged; **#229
is widened by one item** (the `ambiguous-order-model.md` SM §1 row). **Six
`knownStale` markers, all #229's**, per the correction above.

**THINGS NAMED RATHER THAN FIXED, so they are not re-derived as bugs — the list
is open, nothing closes it, and it gained a member during review, so add to it
rather than trusting its length** (§12c).
(1) A contradiction found at the **second** look is reported on
`GateNetDeltaAsserted` while `out.Join` still holds the first look's report
(`reconcile.go:307`), so the operator report's `contradicted` lines cover the
first-look case only — consistent with `Incomplete`'s try-again semantics, and the
escalation is not lost because the next boot's *first* look latches it.
(2) `Reconciliation`'s own finding lists (`Unproved`, `Stale`) are under no
obligation equivalent to the reflection guard now covering `Join`'s; all are
rendered today, so there is no defect — the *protection* stops at `Join`'s
boundary while the report spans both structs. (3) `internal/app/exercise/app.go:142`
still treats `StateSubmitted` as success, deliberately — the exercise root places
one order, calls no join, and exits — but it is the consumer whose success
condition hid this for four slices. (4) `internal/platform/boot/gate.go:369`
illustrates `OutcomeNeedsOperator` with three examples; the real roster is **five
causes across seven sites** (`reconcile.go:373`/`:431`/`:435` net-delta,
`:680` `Unattributed`, `:689` `Contradicted`, `:699` permanent `Unproved`,
`app/reconcile/app.go:665` account posture), four of them pre-slice. No count
word, so nothing is false — but `boot` declares no producer roster, so any number
written about it has to be greped rather than recalled.
(5) **The join's sweep gate pre-empts `advance` entirely, so `advance`'s two
error returns are withheld along with its writes** — a venue state failing
`Validate()` (`join.go:1001-1007`) and `ErrVenueStateNotAnAbsence` on a present
order (`:1010-1019`). Those reach `reconcile.go:308-311` as `boot.OutcomeFailed`
→ `PostureHalted`; under a short sweep the same order yields `OutcomeIncomplete`
→ `PostureReconcileOnly` instead, so the gate **downgrades a halt**. Not a safety
hole (nothing trades either way, and no shipped `toDomainState` arm emits
`AbsentAfterFinality`), but it means the port defence fires only on boots whose
sweep reaches the watermark. Record §10.2's PORT exception.
(6) **`arblab record` now MOVES ORDER STATES as a side effect of taking a spot
baseline.** `RecordSpotBaseline` calls `m.Join.Join(rec.Dispatched)` at
`internal/oms/record.go:192` — step 2 of its gauntlet — and since #242 that call
journals and applies the venue's reading instead of only reporting it. Not a new
*class* of write on that root (the join was already there, and the baseline
already refused on its findings), so no code change is owed; written down here
because "taking a baseline" does not sound like a write path, and the next
session to reason about which roots mutate order state should not have to
re-derive it. Reached from `internal/app/reconcile/record.go:84`.

**⚠ A DEFECT CLASS THIS SLICE'S OWN REVIEW FOUND, AND IT IS THE MOST TRANSFERABLE
THING HERE.** `internal/oms/application/resolve.go:397-401` said proved absence is
an accepted answer *"of `DispatchAuthorized` and `SubmissionUnknown` **and of
nothing else**"*. `Submitted` now reaches `AbsentAfterFinality`, so the derived set
has **three** members — and **this slice never opened that file**
(`git diff … -- resolve.go` is empty). Not a spec deviation: core §5.3 licenses it
verbatim with *"`DispatchAuthorized` **or beyond**"* (`build-spec.md:518`). The
code was right; the comment was wrong; it is fixed.

**The lesson is about the search, not the sentence.** The slice-close reading pass
was scoped to *comment blocks the slice added* — a **diff-scoped** search, which
cannot find this class by construction. The pass that finds it is keyed off the
diff's **effects**: *for each derived set or predicate whose value this slice
moved, grep the tree for prose asserting that value and re-read every hit.*

**No ordinal is given, deliberately** — an earlier draft said "sixth instance" and
listed five, and review showed the ordinal and the list disagree because the class
had **no written membership criterion**. It splits in two, with different lessons:
claims this slice's behaviour **falsified** (true when written, nothing edited
them — R1's and R6/R9's enumerations, and `resolve.go`), and claims **already
wrong** when the slice arrived (R10, HANDOFF's own `knownStale` five, Task 6's
brief's delta count — re-derivation finds those, not search scope). What makes
`resolve.go` distinctive is a checkable property rather than a position in a
count: **it is the first of the falsified group found in a file with an empty
diff.** **The branch review then sharpened that: the empty-diff property belongs
to the HUNK, not the file.** `transition.go:77-79`'s *"two states share it"* was
falsified by the `case StateSubmitted:` arm **109 lines below it in the same
function**, added by this slice's own first commit `9b87b17` — one hunk at `:184`,
nothing else in the file touched. A file-level `git diff --stat` marks that file
as *touched* and would have discharged it, so the effect-keyed pass must also
**re-read the whole surrounding declaration**: the falsifying edit and the
falsified sentence are routinely within scroll distance and in different hunks. Second-order point worth keeping: the *derivation* in `resolve.go` worked
perfectly, widening with no code edit, which is why a listed set was rejected
there. **A derived set cannot drift; a sentence describing it can.**

**AND THE RULE THAT CAME OUT OF THE REPAIRS THEMSELVES: prefer WEAKENING to
enumerating** (record §12c). Repeatedly in this slice a fix for a false
enumeration introduced the next one — in code, in a comment, in a stamp, in the
unblocking document, in the closing record and in the slice ledger — because a
closed list is falsified by anything that adds a member and **a fix is exactly
the event that adds one.** So where a claim ranges over a set you would otherwise
have to keep counting, write a **universal plus a named subset**: record §10.2's
bound now reads *"below `PostureTrading` in every case, whatever was withheld —
and never more permissive than the path it pre-empts except where that path would
have halted the boot"* in place of *"one bound covers both exceptions"*, and it
survives the next exception being found, which the earlier wording did not. The
universal is also the checkable half: `posture()` is a total function over
`Outcome`, so a reader verifies it by reading a switch rather than by trusting a
search somebody once ran. Where a closed enumeration really is load-bearing,
**say what closes it** — record §10.2's terminal set is *"`IsTerminal`'s members
less `AbsentAfterFinality`"* with the exclusion argued, so a new terminal moves
the referent instead of silently falsifying the sentence.

**ONE INSTRUMENT PROPOSED AND NOT BUILT, and it is cheap.** SM §1's transitions
table has now been found incomplete **three times on the same shape**, each fix
applied to the states the omission was *noticed* on. `transition_test.go`'s
`specEdges` oracle is a **flat transcription** — it cannot see a row the document
never had, in either direction. A **reverse pass** (for each state the joiner can
load, for each of the five ACL readings: name the target or name the refusal) is a
page of work, produces the plan's §0.1 matrix, and contains all three historical
omissions **plus** one nobody has reached — `CancelPending → PartiallyFilled` is
absent while `CancelPending → Filled` exists, so a cancel losing the race to a
*partial* fill has no target. Unreachable today because nothing sends a cancel,
which is exactly why it is written down.

---

## Status (2026-08-15, session 141 — HISTORICAL): **#236 IS PLANNED, REVIEWED TWICE AND NOW BLOCKED — THE ORDER LIFECYCLE STOPS AT `Submitted` AND NOTHING IN THE REPOSITORY ADVANCES IT, SO THE PLAN'S FILL GATE CAN NEVER OPEN. DOCS ONLY, NO CODE. THE NEXT ACTION IS `writing-plans` FOR THE FINALITY TASK, NOT FOR #236.**

**Read `docs/superpowers/specs/2026-08-15-m4-236-order-finality-gap.md` — it holds the finding, the
evidence chain link by link, the two rejected alternatives and what the remedy must deliver.** The
#236 plan (`docs/superpowers/plans/2026-08-15-m4-236-crossvenue-pair-root.md`, 14 tasks) is written
and opens with a `BLOCKED` block naming the dependency. No `make ci` run: nothing under `internal/`
or `tools/` was touched, and the `SPEC EDIT OWED` grep is correctly zero.

**⚠ THE FINDING: A LEG THIS SYSTEM PLACES REACHES `domain.StateSubmitted` AND STAYS THERE.** Not the
join — its only repository writes are `Load` (`join.go:777`) and `ApplyFill` (`join.go:881`), and
**neither `ApplyFill` nor `ApplyAck` touches `rec.state`** (`driven/orders.go:313, 466`). Not the
resolver — `resolve.go:397` derives its resolvable set from *"proved absence is one of the answers the
domain would accept"*, which is `DispatchAuthorized` and `SubmissionUnknown` **and nothing else**. The
only `ApplyState` callers are `resolve.go:741`, `dispatch.go:503` (which *sets* Submitted) and
`replay.go:521` (journal replay). `StateSubmitted` is non-terminal (`orderstate.go:146`) and may
advance only to `{Acknowledged, Rejected, SubmissionUnknown}` (`transition.go:186`). **The fill ledger
moves and the state does not.**

**WHAT IT WOULD HAVE COST, as the run rather than as a risk.** #236's D4 establishes exposure only on
a terminal state. Leg A fills 60; the join admits the 60; `Establish` returns **not established** on
every settle attempt; leg B is **never sent**; D8 permits an unwind only on an *established*
`VerdictHalfOpen`, so the compensation is **refused**; the run exits 4 with a real naked 60-unit
position. **The happy path produces the outcome the slice exists to prevent.** Second-order: task 7
re-uses `establish` to confirm the unwind, so **even a successful reduce-only close could not be
recognised as closed.**

**WHY IT WAS INVISIBLE, and this is the durable half.** `arblab exercise run` treats `StateSubmitted`
**as its success state** and exits on it (`exercise/app.go:142` — a leg *below* `StateSubmitted` is the
failure). The single-venue root is a one-shot proof of wire contract, transport and journal; **it never
needed to know whether a leg filled.** #236 is the first thing that does, because a hedge sized to a
partner's fill is the first computation whose input is the fill itself. So the shape is: **a declared
state nothing drives is invisible while every consumer's success condition stops short of it** —
compiler green, `exhaustiveswitch` green (every switch names the member), coverage green, because
fixtures that *seed* a terminal record exercise the reading code perfectly. Only a consumer that must
**arrive** at the state from a real `Place` can tell. This is `[[declared-enum-member-nothing-emits]]`
one layer out: not a member with no producer, a **transition** with no driver.

**THE REMEDY IS SMALLER THAN IT LOOKS, and this fact sizes the next plan.** `ports.VenueOrder` already
carries `State domain.OrderState` — its own comment says *"the venue's lifecycle reading, **already
translated into SM §1's**"* vocabulary — and the joiner **already fetches it**, at `join.go:801`
(`status, err := j.venue.Resolve(rec.Order)`), using it today for fill attribution and the
`Unattributed` comparison. The venue's terminal state is **in hand, in the domain's own words, and
simply never journaled.** The task is to journal it, walking the intermediate hops `transition.go`
requires rather than inventing a direct edge, exposed to roots through `oms.Module` only
(`singlewriter` keeps `ports` unnameable from every `app/` root).

**⚠ THE NEXT ACTION: `superpowers:writing-plans` FOR THE FINALITY TASK.** Not for #236 — it is blocked
and its plan already says so. Four requirements are in the record's §5: the OMS-owned transition
operation; a decision about whether it runs inside the existing join or as a sibling use case; **tests
that start from a real successful `Place`** and walk to each terminal (full fill, partial-then-canceled,
zero-fill canceled, rejection, unresolved status) — **a seeded terminal record proves nothing, because
the defect is that the path to it does not exist**; and a `make formal-fresh` check, since a transition
change is exactly the class that can void a model's coverage while its companion still reports passing
runs.

**WHY IT IS SPLIT OUT RATHER THAN FOLDED INTO #236 (operator, 2026-08-15).** The remedy edits SM §1,
one of the seven normative state machines. Record §95 ruling 3 already declined exactly this bundling
for §5.1/§6.7, and the argument transfers unchanged: **a slice that places real orders against real
venues should not also be the slice that edits the state machine those orders move through**, because
the two failure classes need different reviewers and different evidence. The re-scoped-D4 alternative
— establish from fills plus join completeness alone, arguing an IOC cannot rest — is rejected at the
record's §4: it replaces a guarantee the system can prove with an unverified claim about venue
behaviour, which is `docs/venue-profiles.md`'s class. The gap is **finality, not amount**: the join's
`Unattributed` check already compares the venue's cumulative against the attributed fills.

**THE #236 PLAN IS SOUND AND INHERITS A DEPENDENCY, NOT A REDESIGN.** D4, D5, D7 and D8 were written
against SM §1 *as specified*, and SM §1 is not wrong — it was unimplemented. Two tasks change when the
finality task lands: `drive.establish` (task 6) calls the new operation in place of its
`Join` + `LegState` polling loop, and `venueDouble` (task 5) gains the venue-status responses that walk
consumes. **Everything else stands.**

**WHAT THE TWO REVIEW ROUNDS COST AND BOUGHT.** Both returned **NO-SHIP**, on different shapes, so the
stopping rule never fired — the same curve as #225's three rounds. Round 1: 2 blockers, 5 majors. The
sharpest was `drive.open` checking only `Execute`'s **aggregate** error and never `ex.Legs[0].Err`,
which is *the exact reflex* `execute.go:134`'s split signature exists to make inexpressible (*"an
aggregate error here would invite `if err != nil { return }` over live orders"*) — and the single
half-open test used `rejectPlace()`, which is a **third** path (terminal `StateRejected`,
`dispatch.go:679`), so neither broken path was covered. Round 2: the finality blocker above. **Six
further findings were the primary agent's**, including one **in its own round-1 fix**:
`filepath.EvalSymlinks` lstats the *final* component, so on a not-yet-created journal it returns
`IsNotExist` and the canonicalization silently did not happen — **the symlink refusal could never fire
on a clean first boot**, which is the only boot where the journal is absent. Measured with an
eight-line probe, not reasoned about.

**STILL OPEN, unchanged from session 140 except where noted.** The capture is **still down** by the
2026-08-15 decision (~9 GiB/day accepted, pending the Hetzner host) — #239 and #240 unmoved. #237
(post-cutoff funding fetch) is still cheap, still deadline-free, still the thing most likely to be
forgotten. #225 and #226 remain the only B4 blockers; **neither blocks #236, but the new finality task
now does.** §94 §10's deferred review round still reopens when #225's port is next picked up. #241
(re-scope §5.1/§6.7, twin of #238) unchanged and still blocks nothing today. #235 carries seven; #220,
#227, #228, #229 unchanged; five `knownStale` markers, all #229's; **no tracked section edited, so
`make formal-fresh` is untouched.** **One task is new:**

| Task | What |
|---|---|
| **#242** | The order finality transition: journal the venue's already-fetched, already-translated `State` as legal SM §1 transitions from `Submitted` through to a terminal. **Blocks #236 entirely** — and B4 needs it too, for the same reason: any consumer that must know whether a leg filled. Record: `2026-08-15-m4-236-order-finality-gap.md`. |

**ONE METHOD NOTE, and it is about the review instrument rather than the subject.** Both rounds' worst
findings needed **the code, not the document**. "The plan contradicts D5" was readable from the plan
alone; "the unwind is unreachable because `establish` waits on a state nothing advances" needed
`dispatch.go`, `transition.go`, `join.go`, `resolve.go` and `driven/orders.go` read *together*. A plan
review that only reads the plan can confirm internal consistency and nothing else — and this plan was
internally consistent throughout, which is exactly why two rounds of it were worth the cost.

---

## Status (2026-08-15, session 140 — HISTORICAL): **#236's THREE UNKNOWNS ARE MEASURED AND TWO CAME BACK THE OTHER WAY ROUND — AND VERIFYING THEM FOUND THAT THE SAFETY KERNEL CANNOT SEE A CROSS-VENUE PAIR. DOCS ONLY, NO CODE. THE NEXT ACTION IS `writing-plans` FOR THE CROSS-VENUE COMPOSITION ROOT.**

**Read `docs/superpowers/specs/2026-08-15-m4-236-crossvenue-root-scoping.md` (§95) — it holds the
measurements, the two findings, three rulings and five rejected alternatives.** No `make ci` run:
nothing under `internal/` or `tools/` was touched. Read-only venue probes plus one
`POST /fapi/v1/order/test`; **nothing was placed on either venue.**

**⚠ #236 IS NOT A CONFIGURATION, AND THAT IS SETTLED BY THE ARCHITECTURE RATHER THAN BY OPINION.**
`internal/app/reconcile/config.go:58` — *"Venue is §5.2's per-venue block. **ONE, singular** … `oms.Deps`
takes one venue client … **A deployment trading two venues runs two of these, against two journals.**"*
`exercise/config.go:82` carries it downstream and `ErrVenueMismatch` enforces it. **No document can
express a two-venue package.** Session 139's inventory ("all of it built and mutation-tested") is
accurate **per venue** and silent about the join.

**⚠ THE FINDING THAT SHAPES EVERYTHING AFTER IT: §6.7's CLASSIFIER CANNOT SEE A CROSS-VENUE PAIR, AND
ITS FAILURE DIRECTION CONCLUDES.** `domain.HedgeGroup` is `(Account, Venue, Symbol)` —
`hedgegroup.go:62` — and §6.7 computes its residual **per hedge group**, so a cross-venue pair sits in
**two groups and no group holds both**. Inside each venue's own group a correctly-hedged pair reads as
a filled leg with no pending counter-order: §6 step 7's ***determined*** residual, the branch that
compensates **≤5 s** with an `ExposureReduction`. Driven naively the kernel flattens both halves of a
healthy position, once per venue, in opposite directions. **§2.3's claim that strategy #2 "inherits the
full two-venue execution/one-leg-failure machinery (§6)" is ASSERTED, NOT BUILT** — true of the dispatch
half, false of the classification half. **Nothing acts on the verdict today and this was VERIFIED, not
assumed:** `Module.Classify` is constructed at `module.go:975` and has **no production caller** (the
only `.Classify(` sites are `transport/stream.go`'s and `strategy`'s, unrelated). The driver is B4.

**WHY IT WAS ON NO LIST — the 2026-08-01 fork reached the roster's prose and not the objects it names.**
§5.1's hedge group was designed to group *"a trade's spot + perp legs"* — two legs, one venue, correct
for cash-and-carry. The gate killed cash-and-carry; §41r ruled strategy #2 (perp-perp, **two venues**)
early; §47 then admitted milestone 3 pre-gate *"regardless of strategy"* — true of the order path, false
of the grouping — and slice E built the classifier on the dead strategy's key. §47 **did** hard-gate
*"milestone 4's leg-shape-dependent half"* and named three members; **the hedge group is not among
them**, and §12 item 7's whole entry is one sentence reading *"live wiring"*. **This is task #238's
shape one layer deeper** — #238 found milestone 6's canary spec still fixing BTC-only scope and a spot
IOC leg. Two sites now.

**THE THREE RULINGS (operator, 2026-08-15).** **(1) #236 is the FULL cross-venue composition root** —
two `reconcile.Runner`s, two journals, two OMS modules, a package spanning venues with paired dispatch
and fan-in; the two smaller options are rejected at §95 §7 with their objections kept. **(2) The
cross-venue residual lives in a NEW concept ABOVE the hedge group; §5.1's key is NOT touched** —
because the venue key is right for margin/liquidation/ADL, because §5.1 keys **remedy episodes** by
hedge group so §7.1's whole ladder rides the same object, and because what is needed is not a merged
number but the fact that **a leg has a partner elsewhere**. **(3) The normative re-scope is task #241,
NOT #236** — #236 states which model it follows; §5.1/§6.7's edits land in #241 beside #238.

**⚠ THE NEXT ACTION: `superpowers:writing-plans` FOR THE CROSS-VENUE ROOT, under ruling 2.** The
brainstorm is done and its design decisions are §95 §6. **Do not re-verify the three unknowns — they
are measured and dated below.** Write the plan so that §95 §7's rejected alternative (ii) — venue A,
gate on verdict, venue B, reduce-only unwind of A on B's failure — is its **first phase**, because it
is the smallest runnable increment of the same root.

**THE THREE UNKNOWNS, CLOSED.** **(1) Symbols: seven of the eight shortlist alts are TRADING with
five levels of two-sided book on BOTH demo venues**, at prices tracking each other, min notional
**5 USDT** against BTCUSDT's 50 on Binance — so an alt run is *cheaper* than BTC. **MMTUSDT is
`PENDING_TRADING` on Binance with an empty book and is out of scope.** Session 139's named trap — "the
fixture will not contain the thing under test" — is **refuted**. **(2) Credentials: both keys
authenticate and can trade today.** Binance demo 4 908.60 USDT available and `order/test` for
**LABUSDT SELL MARKET 100** → HTTP 200; Bybit demo `readOnly=0`, `ContractTrade:[Order,Position]`,
164 776 USDT equity, **key expires 2026-11-10T07:03:17Z** (pinned here, held by no prior record).
**(3) `chaos_test.go` drives a FAKE venue** and says why at its line 51 (§59e) — **zero overlap** with
#236.

**SAY "DEMO", NOT "TESTNET".** Every credential in `.env` is `*_DEMO_*` and every probe targets
`demo-fapi.binance.com` / `api-demo.bybit.com`. On **Binance** the distinction is empty (demo and
testnet returned identical 733-symbol sets, zero status disagreements). On **Bybit they are different
environments** — 821 demo symbols against 809 testnet, 29 demo-only, 17 testnet-only.

**WHAT IS NOT CLAIMED, because a snapshot is not a guarantee:** that demo books persist at other
hours, that an IOC fills on a demo alt, that demo funding matches mainnet intervals, or that
`order/test`'s 200 predicts a real `Place` acceptance — it validates a request and deliberately never
reaches the engine that checks margin, position mode and leverage.

**STILL OPEN, unchanged from session 139 except where noted.** The capture is **still down** by the
2026-08-15 decision (~9 GiB/day accepted, pending the Hetzner host) — #239 and #240 unmoved. #237
(post-cutoff funding fetch) is still cheap, still deadline-free, still the thing most likely to be
forgotten. #225 and #226 remain the only B4 blockers and **neither blocks #236**. §94 §10's deferred
review round still reopens when #225's port is next picked up. #235 carries seven; #220, #227, #228,
#229 unchanged; five `knownStale` markers, all #229's; **no tracked section edited, so `make
formal-fresh` is untouched**; the `SPEC EDIT OWED` grep is correctly zero. **One task is new:**

| Task | What |
|---|---|
| **#241** | Re-scope §5.1 and §6.7 for strategy #2's leg shape: the pair concept above the hedge group, its residual composition rule, and whether it is §5.1's **fifth** journal record scope. Twin of #238. **Blocks nothing today** — the classifier has no caller — and blocks B4's monitor when that lands. |

**ONE METHOD NOTE, and it is about this session's own instrument.** The first depth probe reported an
empty book for **every** symbol including BTCUSDT. The uniformity was the tell: `urllib` failing TLS
with no CA bundle — **the exact trap `probe230.py`'s own header comment documents**, in a script
written without reading it. A probe that fails identically everywhere reads as a finding about the
venues. The positive control was free and was not run first: a symbol that certainly has a book.

---

## Status (2026-08-15, session 139 — HISTORICAL): **AN INVENTORY SESSION, AND IT FOUND THE CAPTURE DEAD FOR SEVEN DAYS. NO CODE, NO NORMATIVE DOC CHANGED — THIS BLOCK AND THE TASK LIST ARE THE SESSION'S OUTPUT. THE NEXT ACTION IS A TWO-LEG DEMO SMOKE TEST ON BOTH TESTNETS.**

**No `make ci` run this session — nothing was edited that it covers.** Tree was clean at `185a724`
on entry. Everything below is a measurement of the working tree and the archive, not a build result.

**⚠ THE CAPTURE STOPPED AT 2026-08-08T11Z AND IS STILL DOWN. THIS IS NOW A DECISION RATHER THAN A
FAULT, AND THE LOSS IS ACCEPTED IN THE OPEN.** `archive/v1/_manifest/` ends at `2026-08-08.json`;
the newest `.arbcap` is hour `2026-08-08T11`. `capture.log`'s last line is `capture run: 40 sources
… interrupt to stop` **with no `stopped cleanly` after it** — every prior run in that file has one,
so the process was killed or the host slept. **Nothing noticed for seven days**, because sessions
133–138 were docs-only and no gate, alarm or check sits above a process whose only product is
elapsed time. The operator ruled on 2026-08-15 to **leave it down until the Hetzner host is
verified** rather than restart on the laptop, on the ground that the laptop cannot hold a long-running
process and would fail the same silent way again. **The accepted cost is ~9 GiB/day of
unrecoverable book data for the length of the verification wait, on top of the seven days already
gone.** The alternative offered and declined was a laptop restart with a crude liveness check; a
third option — restart the alt tier only, dropping BTC's eight streams to halve the burn — was
also declined. **Recorded so that a later reader does not read the gap as an accident.**

**WHAT THE ARCHIVE HOLDS — measured 2026-08-15, and the shape of it is the argument for treating
the outage as expensive.** `archive/` is **71.34 GiB** over **7,118 `.arbcap` files**, spanning
2026-07-30 → 2026-08-08T11Z.

| Cut | Size | Recoverable? |
|---|---|---|
| **Books** | **67.07 GiB (94.0%)** | **Never.** Not backfillable at any price, by anyone. |
| Funding | 4.27 GiB (6.0%) | Yes — REST history, any time. |

BTCUSDT is **34.96 GiB — 49% of the archive on 20% of the sources** (eight streams to each alt's
four, and the only symbol with spot legs and the diff-evidence stream). The seven alts run 2.14 GiB
(SOON) to 8.85 GiB (BEAT). `research/` is a further 376 MiB (venue probes), `datasets/` 14 MiB (the
milestone-2 BTC corpus). **§44e's "books are 93.6% of the burn" reproduces at 94.0% on ten days
rather than one** — a rare case in this repo of a measured figure surviving re-measurement.

**THE BURN RATE MOVED, AND §44e WAS NOT WRONG — IT WAS MEASURED ON ONE DAY OF AN ACTIVITY-DRIVEN
STREAM.** Whole days now measure **8.54–8.82 GiB** (08-03, 08-05, 08-07) against §44e's `11.165`.
The partial 08-08 day is 4.84 GiB over 11 hours — **10.6 GiB/day-equivalent**, close to §44e. So the
spread is real day-to-day variance and **§44e's own conclusion is what explains it**: the venues'
documented update speed is a **ceiling, not a cadence**, books are activity-driven, and no
symbol-independent floor exists. **The consequence is for §38f's clock, which was derived from the
single-day figure:** 921 GiB free reads as **~82 days at 11.165, ~102 days at ~9**. Treat this as
*widen the estimate to a range*, not *move the deadline out* — a volatility spike returns it to the
fast figure, and the direction that hurts is believing the slow one.

**THE REST STORE HAS NO POST-CUTOFF ROWS AT ALL, AND THIS WAS NOT PREVIOUSLY RECORDED ANYWHERE.**
`research/crossvenue/rest/{binance,bybit}/` holds exactly 18 files, every one named
`funding.<SYM>.2026-08-01.json` — the cutoff-day fetch, run 2026-08-01 and never since. **The clean
window has been accumulating for two weeks and none of its settled funding has been pulled down.**
This is recoverable whenever `arblab crossvenue fetch` is run with a later cutoff (task **#237**),
but it means the honest answer to *"have we analysed the clean data"* is that **the clean data is
not on this machine.**

**WHAT ANALYSIS HAS ACTUALLY BEEN RUN — the answer is "two things, and neither decides anything",
and a fresh session should not mistake either for a verdict.**

*(1) The screening* (`research/screening/shortlist.json`, 2026-08-01) **is** an `edge − cost`
computation over 466 pairs, and it produced the shortlist:

| Symbol | Net/yr | Entry | Eps | Wins | Win rate | Interval changes |
|---|---:|---:|---:|---:|---:|---:|
| LABUSDT | 71.6% | 0.001 | 71 | 29 | 41% | 454 |
| BEATUSDT | 26.4% | 0.002 | 15 | 8 | 53% | 449 |
| KAITOUSDT | 17.0% | 0.0005 | 55 | 23 | 42% | 1 |
| MMTUSDT | 9.6% | 0.001 | 22 | 10 | 45% | 1 |
| MUSDT | 8.1% | 0.001 | 75 | 19 | 25% | 1240 |
| SOONUSDT | 5.6% | 0.002 | 33 | 12 | 36% | 1445 |
| ESPORTSUSDT | 2.6% | 0.002 | 19 | 9 | 47% | 420 |
| BCHUSDT | 1.5% | 0.001 | 39 | 16 | 41% | — |

Six of eight clear the 4.6% hurdle on these numbers. **Four reasons none of it is evidence**, three
of them already recorded and the first one not: **(a) the cost is the constant
`params.round_trip_cost: "0.0021"` — a flat 21bp for all eight symbols, so the 67 GiB of books has
never been consulted for a cost figure at all**, which is §41r's no-BTC-figure-transfers caveat
failing in a stronger form than the caveat describes; (b) §43b's selection contamination — these
same numbers picked the top 8 of 466, so the 8 necessarily look good *on this score*; (c) every row
ends 2026-07-31, one day before the clean window opens; (d) §43g states in terms that nothing in
step 1 is gate evidence.

*(2) The simulator has exactly one run*, `research/crossvenue/sims/s61-smoke-btc.json`: 9 episodes,
BTCUSDT, two pre-cutoff days, **`Net: "0"` and `Return: "-0.0021"` on every single episode**, all
four depth p90s `"0"`, `PortfolioLCB: "-0.0049"`. It is a **smoke test** — it proves the pipeline
decodes, pairs and scores end to end, and nothing else. Every episode captured zero funding because
none spanned a settlement, and BTC is permanently sealed out of this strategy by §42e's H1 seal.

**THE NUMBER MOST WORTH CARRYING FORWARD IS THE WIN RATE: 25–53%, median ~42%. Most episodes lose.**
LABUSDT's 71.6% is 29 winners carrying 42 losers over nine months, and **every loser costs exactly
the round trip** — which is the constant in (a). A strategy whose return is carried by a minority of
episodes against a uniform per-episode cost is one where a wrong cost figure flips the sign, not one
where it moves the margin. **An untested hypothesis sits beside it and is recorded AS a hypothesis:**
the two symbols with the most funding-interval churn (MUSDT 1240, SOONUSDT 1445) are two of the three
with the worst win rates, and §2.4 warns that Bybit flips an alt's interval without notice. That is
an eyeball correlation over eight points. **Nobody has tested it.**

**MILESTONE 6's CANARY SPEC WAS NEVER RE-SCOPED AFTER THE 2026-08-01 FORK RULING (task #238).**
§12 item 6 still fixes **"BTC-only scope"** and describes a **"spot IOC leg"** — cash-and-carry's
leg shape, for a strategy the gate killed. It carries no dated annotation, unlike items 2, 2b, 3 and
4. Nothing is broken: `2026-08-02-milestone-3-pre-gate-ruling.md:94` noticed the same thing and used
it as a *reason to gate* milestone 6. But the canary's six mandatory figures (max notional, max
collateral, scope, RTO, monitoring, signed acceptance) are written against a two-leg spot+perp trade
on BTC, and strategy #2's canary is a **four-leg perp-perp trade across two venues**. The re-scope is
owed and was on no list before this session.

**⚠ THE NEXT ACTION: A TWO-LEG DEMO SMOKE TEST ACROSS BOTH TESTNETS (task #236), RULED BY THE
OPERATOR 2026-08-15.** Open a perp position on Binance testnet and the opposite leg on Bybit
testnet, confirm both, close both. **Rejected: a single-leg one-venue probe** (smaller, but it never
exercises the two-leg failure case the entire safety kernel exists for) **and two-leg-plus-chaos**
(the right eventual bar, but `app/exercise/chaos_test.go` may already cover part of it and the file
was not opened this session — read it before sizing the slice).

**Why this is the right work now, and it is not "something to do while we wait":** it is
**gate-independent** — §3.1 runs every Intent through the same risk-gate → OMS pipeline regardless
of strategy, which is the ground §47 admitted milestone 3 on pre-gate — and it is **B4's
prerequisite**, since B4 is the loop that drives the order path and proving the path belongs before
writing its caller. It would be on the list even if the gate scored tomorrow.

**What it can use, all of it built and mutation-tested:** the signed REST transport
(`engine/venue/transport/`), `Place` on both venues (`engine/venue/order_{binance,bybit}.go`), the
read path (`orderquery*.go`), plan→journal→dispatch (`oms/application/`), parallel-IOC (slice G3),
reconcile-only boot and `arblab reconcile` (slice G2), and the boot posture gate (`risk/` slice A).
**What it does NOT need, despite each being unbuilt:** sizing-by-live-tier (`planning.go` takes
`Quantity` as an input *by design*), B4 itself, the milestone-7 strategy→OMS wiring, §6.3's
pre-trade posture caller, and **#225's available-balance port** — that port sizes a margin re-post,
not a fixed-size entry, **so #225 does not block this and a fresh session should not treat it as if
it did.**

**⚠ THE TRAP, NAMED BEFORE THE SLICE RATHER THAN AFTER — and it is this repo's own eleven-instance
shape in new clothing.** The testnets are unlikely to list LAB, BEAT, KAITO, MMT, M, SOON or
ESPORTS, so **the smoke test will realistically run on BTCUSDT** — a symbol permanently sealed out
of this strategy, with a liquidity profile no shortlist alt shares. **The fixture will not contain
the thing under test.** That does not make the test worthless: wire formats against a real server,
journal replay, two-leg recovery and both venues' response shapes are exactly what it *can* prove,
and `risk/` slice A's own note says every codec there is fixture-tested and a wrong field name would
be invisible to every test until first contact. It means **the design record must state the
uncovered half up front** — alt-specific behaviour stays owed until a shortlist symbol is run
against a live venue — rather than let a green run read as broader coverage than it is.

**THREE THINGS TO VERIFY BEFORE PROMISING THE SLICE WORKS**, none checked this session: which
symbols each testnet actually lists (the one most likely to bite); whether the demo credentials used
from session 129 carry **trade** permission and not only the read scopes the probes needed; and
whether `app/exercise/chaos_test.go` drives a live testnet or a local fake — the roster says testnet,
the file was not opened.

**INFRASTRUCTURE IS NOW ON THE BOARD (task #239).** The operator registered a Hetzner account
2026-08-15; verification is pending. On verification, two decisions follow that nothing in the docs
has ever considered: **cloud vs dedicated**, and **where 71 GiB and growing lives** — object storage,
an S3-compatible target, or local disk. §38f's destination decision has been open since 2026-08-02
on a measured clock and this is the session it acquires a real candidate. The capture moves there;
**a liveness gate above the capture process moves with it (task #240)**, because the failure this
session found was silence, not error, and the same host change does not fix it.

**DEFERRED, NOT RESOLVED — and the trigger is named because this repo's rule is that a deferral
names one.** §94 §10's open decision — a fourth `codex-copilot` round scoped to §94 §6 and §3, or
straight to `writing-plans` for #225's port — **was not taken this session and is not abandoned.**
It reopens **when #225's port is next picked up**, which is when B4 is next unblocked, which is
after the smoke test and the infrastructure move. The reasoning in §94 §10 stands unchanged and
nothing this session bears on it.

**STILL OPEN.** **#225 (port, designed and thrice reviewed) and #226 remain the only B4 blockers** —
unchanged, and **neither blocks task #236**. #235 carries seven. #220 carries #233's
table-validation requirement; #227, #228, #229 unchanged. Five `knownStale` markers remain, all
#229's; **no tracked section was edited this session**, so `make formal-fresh` is untouched. Five
`[SPEC EDIT OWED]` markers are listed in §94 §9 and **none is in the tree**, so the grep is
correctly zero. **Five tasks are new this session:**

| Task | What |
|---|---|
| **#236** | **The two-leg demo smoke test across both testnets — THE NEXT ACTION.** Ruled 2026-08-15; verify the three unknowns above before sizing. |
| **#237** | Fetch post-cutoff settled funding — the REST store holds nothing after 2026-08-01. Backfillable, so no deadline; but the gate cannot score without it. |
| **#238** | Re-scope §12 milestone 6's canary spec for strategy #2 — it still says BTC-only and names a spot IOC leg. |
| **#239** | Hetzner: cloud vs dedicated, and the storage target for the archive. Feeds §38f's open destination decision. |
| **#240** | A liveness gate above the capture process. The 2026-08-08 outage went seven days unnoticed because nothing watches a process whose only product is elapsed time. |

**ONE METHOD NOTE, and it is the session's most portable result.** Every finding above came from
**opening an artifact and measuring it**, not from reading what the records said about it — the
archive's own bytes, `capture.log`'s missing line, the REST store's filenames, the sim's actual
`Net` field, the screening's `params` block. **Four of the six contradicted or materially qualified
a documented figure, and none of the documents was lying**: §44e's burn rate was measured honestly
on one day, the screening's cost constant is visible in its own params file, milestone 6's stale
scope is plain text. The records were accurate about what they did and silent about what they did
not do. **The productive question for the next session is the same one §94 arrived at from the other
direction: not "is this record right?" but "what would I see if I opened the thing it describes?"**

---

## Status (2026-08-09, session 138 — HISTORICAL): **#225's PORT IS DESIGNED, REVIEWED THREE TIMES, AND NOT BUILT — ALL THREE ROUNDS RETURNED NO-SHIP ON DIFFERENT SHAPES, SO THE STOPPING RULE NEVER FIRED. DOCS ONLY, NO ENGINE CODE. `make ci` GREEN BOTH MODULES, OWED GREP ZERO.**

**Read `docs/superpowers/specs/2026-08-09-m4-225-available-balance-port.md` (§94), and read its §10
first — that is the review log and it decides what the next action is.**

**⚠ THE NEXT ACTION IS A DECISION, THEN EITHER A FOURTH ROUND OR `writing-plans`.** The plan was 2–3
`codex-copilot` rounds on record §87's stopping rule — *stop when successive rounds return findings
of the same shape*. Three ran. **All three returned NO-SHIP, and no two shared a shape**, so the rule
never became applicable and "three rounds ran" is **not** evidence the review converged.

| Round | Finding | Shape | Section |
|---|---|---|---|
| 1 | Bybit's derivation short `locked` + `bonus` | a **missing term** | §4 (Bybit) |
| 2 | source chosen by appeal to **order-admission**; the consumer is `positionMargin` | an **invalid justification** | §4 (Binance) |
| 3 | absent asset refused on both venues; on Bybit absence is a valid **zero** | an **inverted venue semantic** | §5 |

**The pattern is positional and that is the uncomfortable part:** each round found its blocker in a
section the previous round had not examined, and found exactly one there. That is a review bound by
**coverage**, not by depth. §6 (fixtures) and §3 (signature) have had one pass each.

**RECOMMENDATION, not a ruling — the budget is this session's to spend:** one more round scoped
**only to §6 and §3**, then `writing-plans`. §6 is where this project's eleven-instance
fixture-collapse history lives. If that round returns nothing, the curve has flattened *with evidence
behind the claim*; if it returns a blocker, the positional reading was right and the review continues
until a round re-examines an examined section and finds nothing. Skipping straight to
`writing-plans` is defensible — the three findings are fixed and verified against the tree — but it
should be a decision taken knowingly, not by treating the count as the criterion.

**WHAT §94 RULES, AS AMENDED.** The port lives in `oms/application/ports` on the rule that
**placement follows the CONSUMER** — `SpotBalances`' precedent, against core §4 line 435's literal
assignment of this quantity to a `treasury` that has no directory, and `singlewriter` makes that
placement **self-enforcing** because a future `treasury` consumer trips rule 6 rather than merely
looking wrong. The read is **keyed by settlement asset** while the venue call stays **unscoped**. The
answers are now:

| | Binance | Bybit |
|---|---|---|
| Answer | `min(availableBalance, maxWithdrawAmount)`, both off the `assets[]` row | `walletBalance − totalPositionIM − totalOrderIM − locked − bonus` |
| Absent asset | **refusal** | a valid asset-tagged **zero** |
| `Provenance` | `AvailabilityVenueStated` | `AvailabilityLocallyDerived` |

**One rule produced three of those cells and it is the session's most portable result:** when a
question is open and both readings can be satisfied at once, take the reading that is
**correct-or-understating under both**, and file the measurement. Bybit's `locked`, Bybit's `bonus`
and Binance's unrealized-profit exclusion were each unmeasured; deferring them would have blocked the
port on three separate credential-gated probes.

**WHAT IT AMENDS — and the attack the last handoff named was ANSWERED, in a direction nobody
predicted.** That handoff said to press "F3 rejected a published field and this ruling accepts a
different published field — the distinction rests on per-asset vs aggregate, and if that distinction
is wrong the ruling inverts." Checked: **F3 is about the other venue** (`totalAvailableBalance` is
Bybit's, on `collateralSwitch`/`usdValue`/`totalEquity`), so the symmetry never held. And the
Binance-side analogue turned out **stranger than either reading** — the captured account holds
**three funded assets** and the top-level `availableBalance` still equals the **USDT row exactly**,
because Binance's top-level figures are USDT-only outside multi-assets mode, not a converted
aggregate. §6(b)'s stated *cause* was false and §9 point 3's *hazard* was inverted while both
conclusions stood. The lesson is the one worth carrying: **the named attack was productive, but not
by confirming or refuting the thing it named.**

§93 §8 point 2 left Binance's source open
and points 3–4 read as though the derivation were the answer on both venues. §94 rules that a
published figure answers, on grounds **(ii)** the derivation **as measured spans TWO responses**
(`probe225_pass3_separate.py:332-341` takes `isolatedWallet` from a separate `positionRisk` call),
which is §85 §3(iii)'s two-instant objection, and **(iii)** its per-asset attribution of
`isolatedWallet` fails by **dropping a subtrahend**, which overstates available.

**Ground (i) was NARROWED by round 2 and this is the correction most likely to be re-broken.** It
said available balance is what Binance's **order-admission** engine evaluates. §7.1's re-post **is
not an order** — `marginwrite.go:14-17`: *"the one state-changing call this system makes that is not
an order"*, `POST /fapi/v1/positionMargin`. True sentence, wrong engine. (i) still says a published
figure beats our subtraction; it can no longer **select** among the published figures, and §93 §7(a)
records `availableBalance` and `maxWithdrawAmount` as unseparated because every phase had unrealized
profit ≤ 0. Hence `min(...)`.

**THE LIVE CROSS-CHECK IS DROPPED, and that is a ruling rather than an omission.** An earlier form
ran the derivation as a live oracle against the published field, refusing on disagreement. The only
**single-response** derivation reads `account.positions[]`, an array **captured only EMPTY** (pass-1
holds it as `[]` from a flat account; the pass-3 captures project it away), so a wrong assumption
about its shape produces a spurious refusal — which escalates §7.1's ladder into a **delever of a
healthy position**. A refusal gate on an unmeasured shape trades a measured hazard for an unmeasured
one. It also removes a `Corroborated` field nothing would have set.

**FOUR COLLAPSES IN THE CAPTURED EVIDENCE — the first set found in EVIDENCE rather than in a test,
and checklist entries 13–16.** (a) `posIM` and `orderIM` are **never both non-zero on either venue**,
so **sum and max are the same function**. (b) The per-asset and top-level `availableBalance` are
**byte-identical in all five Binance phases** — but **NOT** for the reason the first draft gave: the
account holds **three funded assets** and the top-level figure equals the **USDT row**, so Binance's
top-level is USDT-only, and the hazard is a **wrong-coin** read rather than an aggregation. (c) The
asked-for asset is always the only row, so "this asset's row" and "the first row" are one predicate.
**(d) `locked` and `bonus` are zero in every Bybit phase**, so a three-term and a five-term
subtraction are one function — which is how the missing subtrahends survived §93, §93 §8, §94's first
draft *and its own three-collapse audit*. **An audit for collapses looks at the terms that are
present; a missing term has no fixture to collapse.** Separating fixtures in §94 §6, and (b)'s was
rebuilt because the first one modelled a venue that does not exist.

**A SPEC EDIT FOUND BY VERIFYING A LINE NUMBER.** Core §4 line 435 points a reader at the
**top-level** `availableBalance` — a locator defect inside a correct argument, and precisely the
field collapse (b) conceals. A port built by following that sentence would have passed against every
capture and returned **USDT's** availability for a USDC-settled perp the first time one existed.

**STILL OPEN, AND THE COUNTS MOVED.** **#225 (port, now designed and thrice reviewed) and #226 remain
the only B4 blockers.** **#235 now carries SEVEN** — it held two, §94's first draft made four, round
1 added the `locked`/`bonus` inclusion question, round 2 added *which figure `positionMargin`
admission reads* and *Binance's unrecorded multi-assets mode*. **The count changed three times in one
session, which is the finding rather than an accident: a debt list written before review is a lower
bound, and any handoff quoting its number is stale the moment a round lands.** `[SPEC EDIT OWED]`
markers are now **five**, listed in §94 §9 — the two new ones are Venue Profiles §4 (state Bybit's
derivation with five terms, and cite §196's open `locked` question *at the formula*, not two entries
away) and `analyze225.py`'s printed CONCLUSION, which asserts a completeness its own experiment never
established. None is in the tree yet, so the grep is correctly zero. Five `knownStale` markers
remain, all #229's — **no tracked section was edited**, so `make formal-fresh` is untouched. #220,
#227, #228 unchanged.

**ONE METHOD NOTE FOR THE NEXT REVIEWER, worth more than any single finding above.** Eight defects
landed this session and **every one was a claim about EVIDENCE, not a logic error** — none would have
been caught by re-reading the argument. Five of the eight came from re-deriving *this record's own
citations* rather than from the review rounds. So the productive question was never "is this argument
sound?" but **"does the thing it cites say what it says it says?"** — and the reviewer must be told
explicitly not to trust the artifact's summary, because the summary's author and the record's author
are the same.

---

## Status (2026-08-09, session 137 — HISTORICAL): **#225's MEASUREMENT HALF IS CLOSED — THE ⚠ ANSWERS "POSTURE", AND THEN THE ANSWER STOPS MATTERING. THE PORT IS NOT BUILT. DOCS + MEASUREMENT, NO ENGINE CODE. `make ci` GREEN BOTH MODULES, OWED GREP ZERO.**

**Read `docs/superpowers/specs/2026-08-09-m4-225-available-balance.md` (§93).** Probes and verbatim
captures: `research/venue-probes/225-available-balance/`.

**THE ⚠ IS DISCHARGED.** Record §89 F5 observed Bybit serving empty availability strings under
§4.1's `UNIFIED` + `ISOLATED_MARGIN`, and #225 carried a ⚠ blocking design until the cause was known:
**demo limitation or posture?** The two imply different ports. One observation cannot separate them,
so the experiment is a `set-margin-mode` flip on the same flat demo account — and **the flip back is
the control, not cleanup**: seven account-level fields go `"" → populated → ""`, where a one-way
trace would have been equally consistent with a lazy first computation. **Posture.** So they are
absent in **production** too.

**AND THEN IT IS OVERTAKEN, WHICH IS THE REAL RESULT.** `totalAvailableBalance` is a **USD,
multi-coin, collateral-filtered aggregate** — exactly `Σ usdValue` over `collateralSwitch: true`
coins, **residual `0E-8`**, from a single response. Sizing a USDT re-post from it would overstate
available USDT by the whole USDC leg (`49992.10` on a `49995.60` wallet, **≈2×**), in the direction
that funds an order the account cannot pay for. **The derivation is required under ANY posture**, so
F1's answer changes nothing about the design. A field read as a property it does not carry.

**THE TWO THINGS THE TASK'S DESCRIPTION HAD MERGED.** `availableToWithdraw` stayed empty under
**both** modes — a different cause from the seven account-level fields, and undetermined. One
sentence naming two fields invites one cause.

**`walletBalance` IS GROSS, AND THE PROBE'S OWN FIRST VERDICT SAID THE OPPOSITE.** `add-margin +50`
moves `totalPositionIM` exactly `+50.00000000` and `walletBalance` `0E-7`; a resting order moves
`walletBalance` `0E-8`. So `A = walletBalance − totalPositionIM − totalOrderIM` and the subtraction
is **required**. The market-fill phase is **not** evidence — its `−0.07171582` is the taker fee to a
residual of `−3.30E-8`, so that fixture moves two quantities and attributes nothing to margin, while
producing a confident opposite answer. `add-margin` is the isolating operation *and* the one §7.1
issues — motivating case and isolating fixture are the same one again.

**⚠ THE NAME-MATCH IS WRONG BY THE WHOLE POSTED MARGIN.** Binance publishes the same construction
(`availableBalance = walletBalance − (isolatedWallet + openOrderInitialMargin)`, residual `0E-8`
every phase) — but Bybit's `totalPositionIM` **absorbs** added margin (it is **posted**) while
Binance's `positionInitialMargin` **does not** (it is the **requirement**; the posted figure is
`isolatedWallet`). Pairing them by name overstates available balance by **`49.98010570`**. Record
§89 F2's same-word-different-quantities trap, in a second place, and the name-match is the natural
implementation.

**THE THIRD METHOD FINDING, AND IT IS AGAINST THIS SESSION'S OWN GUARD (§93 §9).** The README's
redaction check — "re-run the consumer and confirm it reaches the same number" — **ran, passed, and
proved nothing**: the redactor changed only the pass-1 capture and the analyzer read only the pass-3
ones, so the clean diff was guaranteed by construction. The consumer now reads every capture the
redactor writes and asserts on the marker. **The rule now sits in the probes README: a reproduction
check must name the file the mutation touched.** Twelfth collapse-checklist entry, first one found in
a harness whose job is keeping another harness honest.

**WHAT IS LEFT OF #225 — THE PORT, and §93 §8 fixes its design and its rejected alternatives so the
next slice does not re-derive them.** Per settlement asset, never an aggregate; one construction with
**two provenances visible in the type**; Binance's inputs are `isolatedWallet` + `openOrderIM`, *not*
`positionInitialMargin`; tests assert on wallet and margin because **the identity has no independent
witness on either venue**; and Bybit's `""` must be a refusal naming the field, not a decoded zero.

**STILL OPEN.** **#225 (port) and #226 remain the only B4 blockers.** **#235 is new**: the two things
these runs could not separate — Binance's `availableBalance` vs `maxWithdrawAmount` never split
because every phase had unrealized profit ≤ 0, so the separating state is a position in **profit**;
and `availableToWithdraw`'s cause. #220 carries #233's table-validation requirement; #227, #228, #229
unchanged. Five `knownStale` markers remain, all #229's — **no tracked section was edited**, so
`make formal-fresh` is untouched. Both demo accounts left flat, no open orders, Bybit posture
restored to `ISOLATED_MARGIN` and verified.

---

## Status (2026-08-09, session 136 — HISTORICAL): **#231 AND #232 ARE CLOSED — THE `TerminalFlatten` RETIREMENT HAD LEFT EIGHT SITES BEHIND, ONE OF THEM SHORT A PRODUCER RATHER THAN CARRYING A DEAD ONE. DOCS ONLY, NO ENGINE CODE. `make ci` GREEN BOTH MODULES, OWED GREP ZERO.**

**Read `docs/superpowers/specs/2026-08-09-m4-231-232-terminal-flatten-errata.md` (§92).**

**THE PAIR, AND THE ORDER IS FORCED.** #231 edits `build-spec.md` §6; #232
re-derives a justification against it and restamps that section. Done the other
way round, #232's restamp lands on a digest #231 immediately invalidates — a
marker covers exactly one delta, and it would have covered the wrong one.

**#231 — EIGHT SITES, NOT ONE.** v0.9.67 retired `TerminalFlatten` from SM §7's
state table and transition rows and did not reach anything that **enumerates** it
or **points at** it: core §6 step 7, **SM §2's own copy of the same producer
list**, SM §7's two ledger invariants, SM §7's quiesce **termination** argument,
SM §7's FLAG-4 ruling, core §5.3's mechanics pointer and core §9's chaos row. All
eight are corrected and tagged `[v0.9.70, task #231]`. core §7.1's lowercase "a
terminal flatten closes the perp first" was inspected and **left** — it describes
the act, and it is correct of `EmergencyClosePackage`.

**THE FINDING THAT WAS NOT THE TASK.** SM §7's ledger invariant also said the
entry moves on *either* supersession row — **two**. `git log -S` dates the phrase
to slice E, when there were two; **slice F added a third**, the premise-lost row
`[→ FM-8, FM-9]`, which ends a generation and retires its entry exactly as the
external two do. So one sentence carried a **dead** member and was missing a
**live** one, and only the dead one is findable: nothing in the document states
how many supersession rows there are, and `FM-6`'s failure runs one way — the
classifier that does not re-run keeps a stale verdict. The producer lists now
state their **count**.

**#232 — THE PREMISE HOLDS, AND IT MOVED.** Step 7's whole delta since 310e854 is
three opcodes: two are #231's, the third is the FM-5-family clause, and the
sentence the fairness set encodes (*"immediately (≤5 s, §9) issues an
`ExposureReduction` … for a determined residual"*) is **byte-identical**. The new
clause adds a *"holds new trading"* state, which could have made the WF stronger
than the spec grants — it does not, because **core §7.5** rules that hold
**stop-opening** with "the safety verbs must stay available". So the
justification now consumes §7.5, and **`build-spec.md` §7.5 is tracked as a
`FenceTakeover` premise from today**. `build-spec.md` §6 is **restamped** for that
model; its marker is gone.

**A METHOD NOTE WORTH MORE THAN THE ERRATA.** The first sweep found five sites and
stopped — it grepped the identifier (15 lines) and read only the ones the task and
the mirror rule made salient. The second pass, a case-insensitive `flatten` sweep
over both documents (58 lines) read one hit at a time, found the two lowercase
prose sites the identifier grep **cannot** match **and three it DID match and
nobody had read**. Counting matches is not reading them.

**STILL OPEN.** **#225/#226 remain the only B4 blockers**, unchanged. #229 is
widened by one item, recorded in §92: the trigger set its revision must encode is
three supersession rows, and SM §2 — a **subject** section — changed, so that
re-pin sits inside the re-run rather than beside it. Five `knownStale` markers
remain, all #229's. #220 carries #233's table-validation requirement; #227
unchanged.

---

## Status (2026-08-09, session 135 — HISTORICAL): **#234 IS CLOSED — THE FRESHNESS GATE NOW PRINTS THE DIGEST IT DEMANDS, AND THE DURABLE HALF IS THAT ITS MESSAGES HAVE A NAME. TOOLING ONLY, NO NORMATIVE DOCUMENT CHANGED. `make ci` GREEN BOTH MODULES.**

**Read `docs/superpowers/specs/2026-08-09-m4-234-freshness-digest-transcription.md` (§91).**

**THE FIX.** `TestManifestSubjectsAreWellFormed` refuses a `knownStale` marker whose `sha256Actual`
is not 64 characters, and the `Stale` verdict that tells you to write one printed **12**. Same on
`MarkerStale`, which says "update the marker" and means "write the current digest". Both sites now
print the current digest in full. The rule, and it is deliberate rather than an inconsistency: **a
digest you must TRANSCRIBE is printed whole; a digest you only COMPARE stays short**, because the
comparison values are already in `freshness.json` — the file you are about to edit — and four
64-character lines bury the one that has to be copied.

**THE DURABLE HALF.** The messages were format strings inside `t.Errorf` calls, so nothing could
assert on them and `tools/mutate.sh` had no line to aim at — a claim that cannot be mutated wants a
name. They now live in `Explain(verdict, model, subject, actual) Report` in
`tools/formalfresh/advice.go`; the gate keeps only the decision to fail or narrate. **Fix the strings
in place and skip the extraction** was the rejected alternative, and it is rejected in §91 §5: it
lands the right bytes and leaves the next edit with exactly the protection this one had.

**HOW IT WAS CHECKED.** 8 mutants / 0 survivors, including **both argument swaps** — the first draft
of the test asserted only `Contains(text, fullDigest)`, which a swap satisfies by printing the full
digest under `stamped:`, so the assertions are positional by line label. Fixture digests are 64
characters and `init()` panics if that stops holding, because at ≤12 "full" and "truncated" are the
same string. And an **end-to-end control on the live manifest**: §7.4 perturbed, digest copied
verbatim from the failure into a marker on both models, gate green with both reporting `marked
stale`, then reverted. That loop is the actual claim — the value in the message is the value the
manifest wants — and no unit test reaches it.

**STILL OPEN.** **#225/#226 remain the only B4 blockers.** #220 carries #233's table-validation
requirement. #227 unchanged. #229 is the model revision that owns the two `knownStale` markers;
#231/#232 are §88's two findings. Nothing in this session touched what the gate *tracks*.

---

## Status (2026-08-09, session 134 — HISTORICAL): **#233 IS CLOSED — THE BRACKET CRITERION IS NORMATIVE, AND ITS UNIQUENESS TURNED OUT TO BELONG TO THE FETCHED TABLE RATHER THAN TO THE RULE. DOCS ONLY, NO ENGINE CODE. `make ci` GREEN BOTH MODULES.**

**Read `docs/superpowers/specs/2026-08-09-m4-233-bracket-fixed-point.md` (§90)** — the
derivation, the negative control that produced the normative half, four checks and their limits.

**THE HEADLINE.** core §7.1 now states the criterion §87 R1b got wrong: **the maintenance bracket is
the one containing the LIQUIDATION notional, and the fixed point is what that rule becomes when the
liquidation price is the unknown.** Closed form for a short, `N_i = q·L_i = (m + q·e + cum_i)/(1 +
MMR_i)`. R1b's **enumeration** survives untouched; only step 2's acceptance test changes, and §87 R1b
now carries a forward pointer so a reader of that record cannot implement the superseded rule.

**THE PART THAT WAS NOT IN THE TASK, and it is the normative half.** Existence and uniqueness follow
from `N_{i+1} − N_i = (MMR_{i+1} − MMR_i)·(1 + MMR_i)·(b_i − N_i)` — which **consumes `cum`'s
continuity relation**, a claim about a *fetched response*. Encoding the rejected reading as a control:
with `cum` zeroed the criterion admits **two** self-consistent brackets in some configurations and
**none** in others (494 / 2 904). So **"exactly one qualifies" is a property of the TABLE**, and
#220's read now MUST assert contiguity-from-zero and continuity, with a failing table
**refuse-to-size** — never a nearest-bracket fallback, whose error is silent and lands on the incident
path. Verified on the live table: continuity residual **exactly 0** at all eight boundaries, top cap
**finite**, so "no bracket qualifies" is reachable from size as well as from a malformed table.

**A SECOND R1b DEFECT, found while making the first precise.** Step 2 conflates **two** selections on
**two** bases — step 4's feasibility test classifies at the *target* notional `q·L*`, step 5's
reconstruction at the *achieved* `q·L` — and since the enumeration accepts the first on-grid candidate
with `L ≥ L*`, they straddle a boundary in **7 of 960** sweep candidates.

**AND A UNITS TRAP FOR WHOEVER SIZES.** A short's liquidation notional exceeds its current notional by
`(1 + 1/λ)/(1 + MMR)`, so bracket 1 ends at **61–76% of the published cap** across §7.1's leverage
range. A cap check in current-notional terms crosses the boundary while **reading as compliant**.

**METHOD NOTES.** The oracle (`research/venue-probes/233-bracket-fixed-point/`) reproduces §89 F3's
four rows to 5.59E-10 / 3.23E-9 in `Decimal`, off the raw capture's own bracket table — **#230's probe
scripts and captures are now preserved** under `research/venue-probes/230-adverse-close/` (redacted),
which is what made an exhaustive check possible instead of a two-row hand-check. Two harness lessons:
the step-identity check first failed on a **`Decimal.copy_sign` misuse** (it returns `|x|`), caught as
a harness fault only because the other checks passed; and the uniqueness sweep first passed
**vacuously** — its grid step was wider than bracket 1 itself, so it never entered the brackets §89
measured. It now prints per-bracket coverage.

**STILL OPEN.** **#225/#226 are now the only B4 blockers** — #233 is discharged. #220 gained a
requirement (table validation) rather than a new task. #227 unchanged, two candidate coefficients
still alive. #229 is the model revision; #231/#232 are the freshness gate's two findings. **#234 is
new and small**: `formalfresh` prints truncated digests in the very message telling you to record a
marker, found by being its first re-pinner. `docs/formal/freshness.json`'s build-spec §7.1 marker was
**re-pinned**, not widened.

---

## Status (2026-08-09, session 133 — HISTORICAL): **#230 IS CLOSED — §87 R1a's IDENTITY IS CONFIRMED TO THE LAST PUBLISHED DIGIT ON BOTH VENUES, AND THE RUN FOUND FOUR THINGS IT WAS NOT LOOKING FOR. ONE IS A NEW NORMATIVE BLOCKER (#233). DOCS + MEASUREMENT, NO ENGINE CODE.**

**Read `docs/superpowers/specs/2026-08-09-m4-230-adverse-close-measurement.md` (§89)** — the
method, the seven runs, six findings, and the limits.

**THE HEADLINE.** `ΔA = (1−α)·m + r·(e−P) − fees` holds **exactly** — R1a error `0E-9` on the
engineered 1.5% adverse close and on both 0.1% market-drift closes. #221's reading is wrong by
**14.8%** of released margin in the adverse run, in the direction that **overstates** the cash
available to re-post. The positive control reproduces #221 byte-for-byte (`liquidationPrice`
65919.93470120 unchanged across the close), so the harness distinguishes "the formula is right" from
"nothing moved".

**THE FOUR IT WAS NOT LOOKING FOR — and the first two are method, not venue:**

- **#233 (NEW, BLOCKS B4's sizing) — §87 R1b's bracket criterion is a FIXED POINT.** Binance selects
  the maintenance bracket for `liquidationPrice` at the **liquidation** notional, not the current
  one, and the chosen bracket is the self-consistent one: bracket `i` whose own `L*` puts `q·L*` back
  inside bracket `i`. Measured both legs — a short reaches **up** a bracket (5.6E-10 vs the
  current-notional bracket's 1.0E-5), a long reaches **down** (3.2E-9 vs 9.8E-6) — so it is not
  directional. `maintMargin` on the same row uses the **current**-notional bracket, so two published
  fields sit on two bases. R1b's enumeration is right; its **acceptance criterion** is not.
- **`α = 0.5` made every partial close this project ever took VACUOUS on the pro-rata claim**,
  #221's included: at a half close `(1−α)·m` and `α·m` are the same number, so "releases the closed
  fraction" and "releases the remaining fraction" predicted an identical observation for four
  sessions. `α = 0.6` separates them 1.5×; both venues release the **closed** fraction. This is the
  collapse checklist's **tenth** entry and the first found against a live venue rather than a test —
  CLAUDE.md now says the checklist applies to measurements, where nothing fails and the number is
  real.
- **`realizedPnl` and `closedPnl` are the same word for different quantities** — Binance's is
  per-fill and **gross**, Bybit's is per-closed-position and **net of BOTH fees**. Reading the
  name-matching field on each venue double-charges Bybit's opening fee (measured: **0.357** on one
  0.005 BTC close) and errs by *understating* cash returned, which is the direction nothing alarms on.
- **Bybit serves NO available balance under §4.1's posture** — `totalAvailableBalance` and
  `availableToWithdraw` are **empty strings**, the asset-transfer family answers demo `10032`, so `A`
  can only be **derived** there. #225 is updated: its Bybit half is harder than its description
  assumed, and any test asserting the identity against that derived `A` is **tautological**.

**WHAT THE FEE FINDING CHANGES.** At the mark where a rebalance actually fires — near entry — the
**fee** is the dominant error in #221's reading and the realized term is a correction on it
(0.1298 vs 0.013 in the control). The realized term only overtakes it past `δ = f`. So a fee-only
patch would look right in every quiet-market test and fail exactly when the rung matters.

**METHOD NOTE FOR THE NEXT CREDENTIALED SESSION.** An adverse fill can be **manufactured** on Binance
demo via the spread (it matches against the published book) and **cannot** be on Bybit demo (single-price
fills better than the displayed opposite side, at any size) — there, only real mark drift works.
Leverage does not help either venue: `δ/f` is leverage-independent.

**STILL OPEN, UNCHANGED.** #227 now has evidence rather than an answer (two candidate coefficients
survive, and every point shares one `e` and `q`). #225/#226 remain the hard B4 blockers; #229 is the
model revision; #231/#232 are the freshness gate's two findings. **#233 joins the B4 blockers.**
`make formal-fresh` is untouched by this session — no tracked section was edited.

---

## Status (2026-08-09, session 132 — HISTORICAL): **MODEL STALENESS IS NOW A BUILD GATE, AND TRACKING PREMISES FOUND A MISSED NORMATIVE EDIT. `make ci` GREEN BOTH MODULES, OWED GREP ZERO. TOOLING + DOCS, NO ENGINE CODE.**

**Read `docs/superpowers/specs/2026-08-09-formal-freshness-gate.md` (§88)** —
the design, the seven rejected alternatives, and the limits.

**WHY IT EXISTS.** The TLA+ models are companion artifacts and the spec is
normative, so a normative edit can void a model's coverage while its companion
still reports 75 passing runs — the runs were real, they were just run against a
**different document**. Nothing detected that but memory. `make formal-fresh`
now hashes each tracked section against `docs/formal/freshness.json` and fails
the build. **TLC cannot catch this class**: a drifted model is still internally
consistent and checks green, which is why "just run the model checker in CI" is
not the answer.

**THE MEASUREMENT THAT SHAPED IT.** Both models were last verified at `310e854`
(slice F, 2026-08-02, the one unfiltered 75-run invocation). Model (b)
`FenceTakeover`'s subject sections — SM §3/§4/§6 — are **byte-identical across
43 commits to `state-machines.md`**, so it is not stale; model (a)
`AmbiguousOrder`'s are not (SM §2 +3 lines, §7 +23), which is #229. A
whole-document hash would have cried wolf 43 times and taught the reader to
restamp without looking.

**TWO FINDINGS, BOTH FROM `premise` ROWS RATHER THAN `subject` ROWS** — the
role split earned itself in its first session:

- **#231 — core §6 step 7 still names the RETIRED `TerminalFlatten`.** SM §7
  retired it under #224 (`[v0.9.67, record §87 §3 R6]`), but build-spec §6 step
  7's producer list — the normative enumeration of what re-triggers §2
  classification `[→ FM-6]` — still lists `terminal-flattening`. **Record §87
  §4's normative-edit list does not name core §6 step 7**, so this is a missed
  edit, not a deliberate retention. Declared-member-nothing-emits, at the spec
  layer.
- **#232 — model (b)'s core §6 premise is un-re-derived.** Its fairness set
  encodes step 7's "obligated ≤5 s naked-reduce (core §6.7)"; §6 went 18 → 29
  lines. Its **runs stand** (subjects unchanged) — what is owed is a
  re-derivation, not a re-run. `FM-7` exactly, and cheap.

**HOW A STALE SECTION IS HANDLED.** Either re-run `cd docs/formal && ./run.sh`
and `make formal-restamp MODEL=<name>`, or record a `knownStale` marker naming
the owning task — **marked, not absorbed**. The marker pins the digest it is
stale against, so it covers **one** delta: a further edit fires again, and a
marker whose section matches again is itself an error. Seven markers are live
today (five owned by #229, one by #232, one by #231's neighbourhood).

**EVIDENCE IT BITES:** 8 mutants / 0 survivors, plus three reverted end-to-end
controls — an unmarked edit to SM §3 (reported against both models), a renamed
heading (reported as unresolvable, not as a silent pass), and an untracked
`.tla` dropped into `docs/formal/`.

**NEXT IS UNCHANGED — this session did not touch the #229/#230 track.** #230
(the adverse-close measurement) is still the cheapest thing with real
information in it; #225/#226 are the hard B4 blockers; #229 is the model
revision. #231 is a small normative edit that should ride along with whoever
next opens build-spec §6.

**ON THE BROADER QUESTION THIS CAME FROM.** The remaining gap after this gate is
**not** formal-methods coverage of the control flow — TLA+ is earning its keep
there. It is that the **economic algebra has no machine-checked layer**: §87's
`MM(αN) ≤ α·MM(N)`, strict iff `cum_i > 0`, is a theorem verified by six review
rounds and three boundary measurements. The models exclude money by construction
("sizing de-minimis needs a magnitude threshold, **which is money**"). The
cheapest close is rapid property tests over `fixedpoint` for the §87
derivations — and §87 already contains a worked counterexample (closed form
selects `q ≈ 0.472` needing margin 10 against 7.36 available; the inequality
gives `q ≤ 0.309`) that is a test case sitting in a document.

---

## Status (2026-08-09, session 131 — HISTORICAL): **#224 IS CLOSED AS A DESIGN RECORD — §7.1's TERMINAL RUNG IS A *FUNDING* STEP, NOT A HEADROOM STEP, AND THE LADDER RE-ENTERS THE TOP-UP. DOCS v0.9.67, NO CODE, `make ci` GREEN BOTH MODULES, OWED GREP ZERO. B4 IS NOW BEHIND FIVE PREREQUISITES, FOUR OF WHICH THIS RECORD DISCOVERED.**

**Read `docs/superpowers/specs/2026-08-09-m4-224-escalation-rung-redesign.md`
(§87) first** — it is the whole design, and §5 is the prerequisite list that
changes the plan.

**THE FINDING IN ONE LINE.** A `PackageDelever` raises no liquidation headroom,
but it releases **exactly** the margin the failed top-up was short of
(`Δ = (1−α)·m`), so rung 3 exists to make rung 2's precondition true. The ladder
**re-enters the top-up** instead of escalating past it. §7.1's parenthetical was
not merely dated; it was doing load-bearing work on the safety kernel's terminal
rung and it was false.

**THE REASON NOW REACHES FURTHER THAN THE MEASUREMENT.** Maintenance margin is
one **continuous convex piecewise-linear function of notional**
(`MM(N) = N·MMR_i − cum_i`, `MM(0) = 0`), so `MM(αN) ≤ α·MM(N)` and a pro-rata
reduction leaves `L` *exactly* unchanged **iff the bracket's deduction is zero** —
bracket 1, which §7.2's small-size posture makes the bracket we trade in. #221
measured 0.002 BTC, ≈$130, i.e. bracket 1: the exact-inertness case. Measured at
three boundaries across both venues, continuity holding to the cent.

**TWO NORMATIVE DOCUMENTS WERE FOUND WRONG, NOT DATED.** `venue-profiles.md`
named `instruments-info` for Bybit's tier table — an endpoint carrying **no**
risk-limit field; the answer is `GET /v5/market/risk-limit` (corrected from a
live call, and `mmDeduction` is `""` in tier 1, its value *established* by
continuity rather than assumed). And **record §85's `Deleverer.Open` row
described its argument backwards** — "target position" where the code bounds the
executed **reduction**. §85 carries an erratum; §87 renames the two quantities
(`q_final`, `r`) so they cannot be spelled the same way again.

**WHAT B4 IS NOW WAITING ON — five, and two are hard blockers:** **#225**
available-balance read both venues (released *posted margin* ≠ returned *cash*,
because an adverse close realises a loss and a fee under exactly the rally that
fired the trigger); **#226** `EmergencyClosePackage`'s both-leg close, **which
does not exist** — only its supersession half does; **#227** Bybit's isolated
liquidation formula, verified (§85 verified Binance only); **#228** the verified
insufficient-margin error code per venue, now a blocker because an
undistinguished venue rejection defaults to terminal; **#229** formal-model
revision, because removing SM §7's `Complete → Planned` invalidates
`AmbiguousOrder.tla`'s `MAX_GEN = 2` premise and `PC20`'s coverage directly.
**#230** is the adverse-close measurement §87 owes.

**HOW THE RECORD WAS PRODUCED IS PART OF WHAT IT ASSERTS.** Six adversarial
review rounds against a second model family, every one NO-SHIP. The arithmetic
converged at round 3 and never moved again; rounds 4–6 found only **missing cases
in a prose state machine** — one per round. That is the class the TLA+ models
exist to find, so review was stopped there **deliberately**: the remedy episode
is recorded as **specified-but-unmodelled** and #229 is a blocking prerequisite
rather than a follow-up. Do not re-review §3's episode in prose; model it.

---


## Status (2026-08-09, session 130 — HISTORICAL): **#219 IS CLOSED — THE MARGIN READ CARRIES §7.1's TWO SIZING INPUTS, AND THE FENCE THAT PERMITS THE SECOND DECODE BINDS ONE WAY. DOCS v0.9.66, 16 MUTANTS / 0 SURVIVORS, `make ci` GREEN BOTH MODULES, OWED GREP ZERO.**

**Read `docs/superpowers/specs/2026-08-09-m4-219-margin-read-sizing-fields.md`
(§86) first** — four rulings, the alternatives each rejected, and one finding
that changes what a later slice may safely assert.

**WHAT LANDED.** `SizingQuantity` (signed, base asset) and `EntryPrice` (quote
asset) on `venue.MarginStatus` and `ports.MarginReading`; both venue codecs
decoding them; `driven.VenueMarginStatus` carrying them; both `Validate`s
checking them; five new sentinels; and the shared-payload differential test
record §85 owed. Normative edits landed in the same session: §4's declared
`MarginStatus` line, a new §4 paragraph, and §7.1's bullet — which said the
surface reads the amount as a **presence test only** and was thereby made false
by this slice.

**THE NORMATIVE HALF IS THE PROHIBITION, NOT THE PERMISSION.** The widened
quantity **may never be fed to §7.5's net-delta invariant**, and no consumer may
substitute it for `AccountSource.Positions` on cost grounds. That invariant's
value is joining two authorities that CAN disagree; a second decode of one venue
field is not a second authority, so the substitution leaves it comparing the
venue with itself and passing forever. The field is **named for the use it is
permitted** — that name is the only carrier the rule has at this seam, since the
consumer that must not read it lives in another context.

**THE FINDING WORTH CARRYING FORWARD: THE OBVIOUS CROSS-CHECK IS THE
LIQUIDATION CONDITION ITSELF.** `EntryPrice` and `LiqPrice` are the same type in
the same pair, so a transposition validates, and the guard that suggests itself
is "a short liquidates above its entry". Solved out of §85's relation,
`L > e ⟺ m/q > e·MMR` — which is exactly *"this position is not currently
liquidatable"*. Enforcing it would refuse precisely the readings §7.1's ladder
exists to act on, at the instant it needs them, and would return an error where
ruling (iv) requires a ladder state. **Anything #220 or #222 writes that
compares these two prices needs the same solve run against it first.** The
transposition is covered by value assertions instead.

**Bybit's margin wire gained `side`**, reversing a v0.9.60 note that was right
when written — §85 changed what the consumer needs, and Bybit's `size` is
documented always-positive. The sign rule is **shared** with §7.5's codec, which
removes a failure mode and costs the differential test its ability to see a sign
disagreement *on that venue*; what it covers there is key selection, and on
Binance it stays genuinely differential (two independent parses of
`positionAmt`, no helper between them).

**NEXT, in this order:** **#224** (the escalation redesign — a decision record,
and it changes what B4 is); **#220** (the `leverageBracket` codec and the
now-unblocked inverse solve — #219 delivered its quantity half); **#223**
(correct the weight constants and audit every other `Weight` literal against a
measurement). **#222 (B4) is blocked on #220 and #224 only.** #216 stays open
and independent.

---

## Status (2026-08-09, session 129 — HISTORICAL): **THE FIRST CREDENTIALED SESSION — AND §7.1's TERMINAL RUNG TURNS OUT TO RAISE NO LIQUIDATION HEADROOM. B4's SIZING RULING TAKEN (RECORD §85), #221 ANSWERED, #223 CONFIRMED, #224 OPENED. NO CODE CHANGED; DOCS ONLY.**

**Read `docs/superpowers/specs/2026-08-08-m4-218-b4-sizing-inputs-design.md` (§85)
first**, then this block — §85 is the ruling, and this session's measurements
confirmed one of its derivations exactly and refuted the premise of another.

**VENUE CREDENTIALS NOW EXIST, AND THEY ARE DEMO ON BOTH VENUES.** `.env` is
populated and gitignored; `.env.example` is the tracked template and carries the
hosts, the caveats and the provenance of every fact in it. **Demo, not testnet,
and the reason reaches the spec:** Binance demo is ONE account with Spot and
Futures as tabs, so §4.1's spot≠futures wallet split is observable, where testnet
is two unrelated deployments in which the split is not a split. Binance's own
docs say demo's *"IP Limits, Unfilled Order Count, Exchange Filters are **exactly**
the same as the live exchange"*. **What demo is NOT for is economics** — Binance
states plainly that realistic market data is not real market data, so milestone
2's kill gate keeps running off the unauthenticated capture.
`BINANCE_DEMO_*` covers both legs on one key (Reading + Spot&Margin + Futures);
`BYBIT_DEMO_*` likewise. **Neither venue serves the TRANSFER path on demo** —
Bybit omits it by documentation, Binance by routing (`/sapi/v1/asset/transfer`
answers an **HTML** 404, i.e. the whole namespace is absent, not a rejected
request). `TransferCollateral` therefore needs Bybit testnet when its slice comes.

**§7.1's `PackageDelever` RAISES NO LIQUIDATION HEADROOM, AND THIS IS MEASURED
RATHER THAN ARGUED `[task #224, from #221]`.** Both venues release isolated
margin **pro rata** on a partial reduce-only close, so margin-per-unit `m/q` is
invariant and the liquidation price does not move:

| Venue | quantity | posted margin | liquidation price |
|---|---|---|---|
| Binance | −0.0020 → −0.0010 | `isolatedWallet` 6.43067264 → 3.21533632 | 67753.52223108 → **67753.52223108** |
| Bybit | 0.002 → 0.001 | `positionIM` 13.0395549 → 6.51977745 | 70931.6 → **70931.6** |

**The reasoning that made the rung look right is CROSS semantics, and §4.1 pins
Isolated** — under cross the wallet backs the position and `m` does not scale
down with `q`. §7.1 warns about exactly that confusion two paragraphs earlier,
about `auto-add-margin`. **And the defect is CONDITIONAL, which is the part worth
carrying:** of the three failure causes §7.1 names, *insufficient staged margin*
has a working escalation (delever, then re-post the released margin — halving `q`
while restoring `m` doubles `m/q`, and the released margin does return to
available balance), while *rate-limit* and *venue lag* have none, because the
re-post travels the path that just failed. One verb, three causes, one remedy
that fits one of them. §7.1's bullet now carries a ⚠ block; the redesign is #224
and it wants a decision record, not a patch.

**RECORD §85 TOOK B4's SIZING RULING, AND ONE OF ITS DERIVATIONS IS NOW EXACT.**
`L = (m/q + e + cum/q)/(1+MMR)` predicted Binance's published `liquidationPrice`
to **eight decimals** (m=6.43067264, q=0.002, e=64809.2, MMR=0.004, cum=0 →
67753.52223108). So #220's inverse solve `Δ = (L*−L)·q·(1+MMR)` rests on a
verified relation, and Binance's `cum` **is** the tier deduction the derivation
cancels. It is exact only with `isolatedWallet`; `isolatedMargin` is that plus
unrealized PnL and moves with the mark, which would put the mark back into a
figure §7.1 ruling (iv) requires to exclude — `marginstatus_binance.go:200`
already decodes the right one, now measured rather than assumed.

**#223: THE WEIGHT CONSTANTS ARE 4–20× TOO HIGH, CONFIRMED AUTHENTICATED.**
Measured deltas on `x-mbx-used-weight-1m`: `/fapi/v3/positionRisk` **5** (code
charges 20, `positions_binance.go:115`), `/fapi/v1/positionMargin/history` **1**
(#217 recorded 20), `/fapi/v1/leverageBracket` **1**. Authenticated and
unauthenticated deltas are identical, which closes the confound. Positive control
passed: `/fapi/v1/depth?limit=5` charged 2 and `limit=1000` charged 20, matching
documentation. **The error direction is the unsafe one** — over-charging cannot
cause a ban, so nothing fails loudly, but §7.6 sizes the **Safety reserve** from
these constants, so an inflated weight manufactures spurious refusals on the
incident path. Retires the browser-confirmation halves of **#180** and **#217**:
the weight is a response header, and it is readable on the very endpoints whose
docs answer a fetcher with HTTP 202 and zero bytes.

**NEXT, in this order:** **#219** (widen the margin read — signed quantity +
entry price; unblocked, pure fixture work, and both fields are already sitting in
`marginstatus_test.go`'s own fixtures decoded by nobody); **#224** (the
escalation redesign — a decision record, and it changes what B4 is); **#220**
(the `leverageBracket` codec + the now-verified inverse solve); **#223** (correct
the weight constants and audit every other `Weight` literal against a
measurement). **#222 (B4) is blocked on all of #219/#220/#224.** #216 stays open
and independent.

**Task-store numbering:** the store was reset by a crash on 2026-08-08 and the
#150–#217 roster was lost, so live IDs are offset from the documentary numbering
every record uses; the documentary number is written into each task's subject.

**Unchanged:** three standing operator decisions ($20/$55 live test, #171's
credential, §38f); capture **PID 8495**; never rebuild over `bin/arblab`;
`make ci` runs `-race` AND the ka staleness gate (needs node); cap `TLC_WORKERS`
at 4; docs/i18n parity is `bun test`.

---

## Status (2026-08-08, session 128 — HISTORICAL): **§7.1's MARGIN TOP-UP DISPATCHES — AND A RETURNED CALL MUST CONCLUDE, OR THE REMEDY'S SUCCESS IS INDISTINGUISHABLE FROM ITS FAILURE. DOCS v0.9.65, 19 MUTANTS / 0 SURVIVORS, `make ci` GREEN BOTH MODULES, OWED GREP ZERO.**

**Read `docs/superpowers/specs/2026-08-08-m4-215-marginop-dispatch.md` (§84)
first** — the six rulings, the alternatives each rejected, and what B4 inherits.

**#215 IS CLOSED, CODE AND DOCS.** The code landed in `2d4cfbe` (session 127b);
this session is its docs half — §4's signature edit, which **discharged the
slice's one `[SPEC EDIT OWED]` marker**, plus §7.1's dispatch bullet, §12's
part-2 annotation, Profiles §4's two new bullets and its second ⚠, `adr-history`
§84, and this file. The 19-mutant / 0-survivor figure belongs to the code commit;
the only code touched today is the marker line's replacement.

**THE DESIGN QUESTION WAS WHO IS ENTITLED TO CONCLUDE, NOT HOW TO CALL THE
ENDPOINT.** The tidy instinct is to route every op through `MarginOp.Resolve` so
§7.1's evidence rule keeps one home. Run right after a call that returned, that
path answers **`Pending`** on a venue with an act history — #214's own re-derived
premise is that the history lags *independently* of the margin figure, so the
propagation bound guards its silence. No verdict ⇒ nothing journaled ⇒ the
position stays latched ⇒ the ladder escalates to a `PackageDelever` **on a top-up
that worked**. The narrower reading is also the more accurate one:
`ResolveMarginOp` is a *reconstruction* of what the response would have said,
built for the case where no response was read.

**THE TWO VENUES ACKNOWLEDGE IN COMPLEMENTARY, NON-OVERLAPPING VOCABULARIES.**
Binance states the **act** (`{amount, code, msg, type}` — no symbol); Bybit
states the **subject** (a position snapshot — no amount). So there is no shared
echo helper, unlike `marginLiqPrice` on the read side: each codec checks its own
venue's half and records what it structurally cannot. Bybit's `positionIM` is
deliberately unread — a second authority for a figure `MarginStatus` owns, on one
venue, in the one case that needs no evidence at all.

**THE LATCH IS RELEASED BY DURABILITY, NOT BY KNOWLEDGE.** A verdict known but
not journaled must be rebuilt from evidence later, and a second op on that
position **destroys** that evidence — the posted margin then moved by the sum of
two deltas and matches neither. Different from `Outcome.Journaled` on the order
path, where a lost state record is re-derived by query.

**B4 IS NOW THE WHOLE GAP, AND IT IS THE CRITICAL PATH.** All three of §7.1's
rungs exist — trigger (#210), top-up (#215), ladder (#177) — and **not one is
driven**. Nothing re-resolves a *live* in-flight op either, so a failed top-up
latches its position until the next boot; that is §7.1's escalation rather than a
gap, but B4 is where it stops being the only answer.

**#217 NOW COVERS TWO UNVERIFIED BINANCE WEIGHTS, NOT ONE.** The write
(`POST /fapi/v1/positionMargin`) joins the read (`positionMargin/history`), both
charged at 20 behind #180's WAF wall. The write's argument was made **afresh
rather than inherited**, because unlike the boot-time read it is on the incident
path — what makes overcharging affordable there is §7.1's own
one-op-in-flight-per-position ceiling, not a traffic estimate.

**NEXT:** (1) **slice B4** — the loop that drives the monitor, calls the
dispatcher on the Rebalance band and `Deleverer.Open` on failure; it is what
turns three rungs into a ladder. **#216** (the liq monitor's unwired module
branch) is still open and independent of it.

**Unchanged:** three standing operator decisions ($20/$55 live test, #171's
credential, §38f); credentials deferred; capture **PID 8495**; never rebuild over
`bin/arblab`; `make ci` runs `-race` AND the ka staleness gate (needs node); cap
`TLC_WORKERS` at 4; docs/i18n parity is `bun test`. **#180 is untouched.**

---

## Status (2026-08-08, session 127 — HISTORICAL): **BINANCE'S ACT HISTORY LANDS — AND TWO OF #212's CORRECT RULES TURNED OUT TO BE SCOPED TO A ONE-SOURCE WORLD. DOCS v0.9.64, 22 MUTANTS / 2 REAL SURVIVORS (BOTH FIXTURE COLLAPSES IN THE NEW TESTS, BOTH CLOSED), `make ci` GREEN BOTH MODULES, OWED GREP ZERO.**

**Read `docs/superpowers/specs/2026-08-08-m4-214-marginop-act-history.md` (§83)
first** — the division of labour between the two sources, the distinction that
inverts a verdict, the two re-derived premises, and `deltaType`.

**#215 IS NOW UNBLOCKED.** #214 was the last thing in front of the dispatch path.

**THE "PLUS" IS NOT A CONJUNCTION, AND EACH SOURCE GETS THE JOB IT IS GOOD AT.**
A history row records an **act**; the posted delta is a **live quantity every
fill moves**. So: **the history is the only thing that can conclude `Applied`,
the delta is the only thing that can conclude `NotApplied`, and disagreement is
`Ambiguous`.** The case this wins is the ordinary one — a partial fill inside the
window moves the margin to a figure matching nothing and leaves a clean row.

**THE DISTINCTION THE WHOLE DESIGN PROTECTS: "no act history exists" and "the
act history returned no rows" are OPPOSITE evidence.** One observation — the
posted margin moved by exactly the delta — is `Applied` on Bybit and `Ambiguous`
on Binance, because there the venue was asked and said it did not perform the
act. A `[]MarginAct` whose nil means "no rows" makes the value a forgetful caller
produces mean the reading that **blocks openings until an operator clears it**.
Hence a three-state unexported enum whose zero is refused by name, two
constructors, `ports.VenueMarginActs` returning the **domain type whole** (the
one read port that does — the ACL mirrors exist for an import rule that does not
apply here), a **required** act port on the resolver, and `Deps.VenueMarginActs`
as its own field rather than a type assertion on `VenueMargin`.

**COMPOSING BROKE TWO PREMISES #212 HAD DISCHARGED CORRECTLY.** (i) A **flat**
position no longer outranks everything: that rule's argument was about the
DELTA's evidence, and a venue that recorded the act recorded it whether or not
the position closed afterwards. (ii) The propagation bound now guards the
**history's SILENCE** rather than the margin's stillness — "no row yet" is also
an observation time can change, and it lags independently, so holding the old
rule would have made the half built to REDUCE over-ambiguation introduce a new
source of it. A row that HAS arrived is never held for the bound.

**§7.1's STATED MATCH IS MISSING A COMPONENT, AND §4.1 ALREADY IMPLIED IT.** The
section says *symbol + type + amount + time-window*, silent about WHY the margin
moved — while §4.1 reads `isAutoAddMargin` per configured symbol **precisely
because the venue can add margin on its own**. The row shape is now verified
**verbatim** (from the spec-generated connector's own mocked response, since the
doc site answers a fetcher with a WAF challenge) and carries a seventh field,
`deltaType`. Only `USER_ADJUST` is verified, so the decode is two-valued and an
**unrecognised value is REPORTED, not dropped** — `platform/config`'s precedent
inverted, because dropping is both the silent reading and the unsafe one.

**BOTH SURVIVORS WERE THE SAME SHAPE, AND BOTH WERE IN TESTS WRITTEN THIS
SESSION:** a fixture whose `live` figure made the mutant's fallthrough branch
answer identically. The duplicate-row case used a **moved** margin, where
dropping the row count sends two rows to the silent branch which also says
`Ambiguous`; and the silent-and-flat case passed a **valid** zero where the
resolver actually passes the **unset** `Money`, so removing the flat guard
differenced a legal figure instead of failing.

**NEW: #217** — the `positionMargin/history` weight is unverified behind the same
WAF wall as #180 and charged at 20. Nearly free (one call per in-flight op at
boot, zero in steady state), so it rides along with #180's browser read.

**NEXT:** (1) **#215** — the dispatch path (before-state read, fsync,
`AddIsolatedMargin`, one op in flight per position). Then B4, the loop that
finally calls `Deleverer.Open`. **#216** (the liq monitor's unwired module
branch) is still open and independent.

**Unchanged:** three standing operator decisions ($20/$55 live test, #171's
credential, §38f); credentials deferred; capture **PID 8495**; never rebuild over
`bin/arblab`; `make ci` runs `-race` AND the ka staleness gate (needs node); cap
`TLC_WORKERS` at 4; docs/i18n parity is `bun test`. **#180 is untouched.**

---

## Status (2026-08-08, session 126 — HISTORICAL): **THE `Ambiguous` MARGIN OP HAS AN END — AND §7.3 HAD ALREADY SAID WHERE TO PUT IT. DOCS v0.9.63, 29 MUTANTS / 5 REAL SURVIVORS (ALL CLOSED, ONE BY A SIGNATURE), `make ci` GREEN UNCACHED BOTH MODULES, OWED GREP ZERO.**

**Read `docs/superpowers/specs/2026-08-08-m4-213-marginop-clearance.md` (§82)
first** — the home §7.3 names, the finding that is deliberately not a field, and
five survivors of which only one was a missing test of a rule.

**#212's TWO CORRECT DECISIONS COMPOSED INTO A PERMANENT HALT.** The verdict is
durable, and `Ambiguous` latches — both right, both required by §7.1 and §7.5 —
and together they meant the next boot read the verdict back, reported
`OutcomeBlocksOpening` again, and returned `ExitOperator` again, for ever.
Nothing in the system could end one. `arblab margin reconcile` is that act now,
and it had to land **before** the dispatcher: unreachable today, unrecoverable
the day #215 lands.

**THE HOME WAS NOT A CHOICE BETWEEN TWO OPTIONS.** §7.3 requires *"an immutable
per-actor audit of every control action (who / what / when, **appended to the
tamper-evident log, §5.1**)"* — so the clearance is a **fourth record kind on the
one chain**, and the document-shaped alternative (§4.1's attestation) becomes a
recorded rejection. It is refused by that document's **own** discriminator: it
exists because *"every field is the operator's own word"* and no machine work is
involved, where a chain append at a sequence is machine work by construction.

**THE RULING MOST LIKELY TO BE REOPENED IS THE FINDING THAT IS NOT A FIELD.**
§4.1's attestation argues for one — record what was OBSERVED, never the verdict —
and §7.1 refuses it in its own terms: the state is called `Ambiguous` because
*"evidence is not causal identity"*, so demanding a determination demands exactly
what the evidence cannot support, and **no reachable value would keep the block
on** (even the doubled top-up §7.1 names reconciles and lifts). What is
reconciled is the **float**, whose ledger is §7.2's unbuilt `treasury` — so the
structured half is owed **with its consumer**, and §82 §2a is where to start.

**§7.5 GAINS THE GENERAL OBLIGATION:** an outcome that latches across processes
is only completable if some act can produce the evidence that clears it, and the
act must land **with or before** the first thing that can produce the outcome.

**FOUR OF FIVE SURVIVORS WERE NOT ABOUT RULES.** Two were one CLI fixture: an
all-flags-empty invocation left four of five required-flag checks as dead code,
and a usage assertion could not tell a **routed** subcommand from the
fall-through, because the usage block prints the same flag names and both return
`64`. One was a message — two refusals share a sentinel, so only the wording
separates them. One was a guard duplicating a downstream check, kept for its
failure direction and re-aimed at what only it can see. **The fifth was closed by
the SIGNATURE:** two adjacent `string` parameters, and SQLite *creates* the file
it is asked to open — so a swap would mint an empty journal named after the
subaccount and refuse with the same message a mistyped op id produces.

**NEXT, IN THE ORDER IT MUST LAND:** (1) **#214** — Binance's
`positionMargin/history` evidence half, whose reading is already ruled in §7.1
(the "plus" is a fallback, not a conjunction); (2) **#215** — the dispatch path
(before-state read, fsync, `AddIsolatedMargin`, one op in flight per position),
which #214 is now the last thing blocking. Then B4, the loop that finally calls
`Deleverer.Open`. **#216** (the liq monitor's unwired module branch) is still
open and is independent of all three.

**Unchanged:** three standing operator decisions ($20/$55 live test, #171's
credential, §38f); credentials deferred; capture **PID 8495**; never rebuild over
`bin/arblab`; `make ci` runs `-race` AND the ka staleness gate (needs node); cap
`TLC_WORKERS` at 4; docs/i18n parity is `bun test`. **#180 is untouched.**

---

## Status (2026-08-08, session 125 — HISTORICAL): **§7.1's `MarginOp` RESOLVES AT BOOT — AND THE OUTCOME DECLARED FOR IT NEVER LATCHED. DOCS v0.9.62, 24 MUTANTS / 2 REAL SURVIVORS (BOTH FIXED, BOTH IN THE FIXTURE), `make ci` GREEN BOTH MODULES, OWED GREP ZERO.**

**Read `docs/superpowers/specs/2026-08-08-m4-212-marginop-resolution.md` (§81)
first** — the outcome with no producer, the scope §7.5 sorted on, the stored
verdict's two failure directions, and both survivors (a clock-skew fixture too
small to cross its own bound, and a call site nothing exercised).

**`boot.GateMarginOpsResolved` HAS A PRODUCER**, after five slices of being
declared with none. So it crosses `app/reconcile` `ExitClass` into the produced
arm — and before that crossing an **`Ambiguous` margin op would have exited
`arblab reconcile` CLEAN**, because the gate sat in the arm named *"the four with
no producer anywhere"* and fell through. §7.1 calls that state a §9 case needing
explicit accounting.

**THE HEADLINE: `boot.OutcomeBlocksOpening` HAD NO PRODUCER, AND ITS OWN DOC
QUOTED THE CLAUSE THAT MAKES IT LATCH.** The member was declared for exactly one
condition — §7.5's Ambiguous `MarginOp` — and placed in the freely re-reportable
column, so a later `OutcomePassed` on the same gate lifted a §9 incident with
nobody having looked. §7.5 sorts self-clearing from latching by **precondition**
where the property belongs to the **outcome**, and one precondition produces
both: a `Pending` op clears itself, an `Ambiguous` one does not. Three grounds,
and only the third is probabilistic — both normative sentences; the checklist
holds **one outcome per gate rather than one per op**; and re-reading the
evidence is more **confounded**, not more informative.

**`Report`'s POSTURE COMPARISON WAS LOOSER THAN ITS OWN DOC AND NOBODY COULD
SEE IT.** It read `<=` where the doc says STRICTLY more restrictive. That agreed
only because the three latching members happened to have three **distinct
postures**, so equality could hold only for a repeat. The accident expired the
moment a second member reached the stop-opening row.

**THE VERDICT IS JOURNALED, NOT RE-DERIVED, AND THE TWO WRONG ANSWERS GO
OPPOSITE WAYS.** An ambiguity held only in a checklist dies with the process, so
a **restart would satisfy** "until the staged-float ledger is explicitly
reconciled". An `Applied` re-derived after Binance's windowed history has rolled
past **inverts** to `NotApplied` — the reading under which nothing happened.

**BOTH SURVIVORS WERE THE FIXTURE, NOT THE CODE.** The venue-clock anchor test
placed the op half a bound old and asserted `Pending`: the clocks are 250ms apart
against a 30s bound, so both readings agreed and the right parameter was varied
by an amount the predicate cannot see. It straddles the bound now, sign computed
from the clock. And nothing exercised the module wiring at all — `fullDeps`
supplies no `VenueMargin`, so making `oms.New` skip the resolver survived every
test in the package. What the new fixture covers is the **shared margin adapter
and the resolver**; the liq monitor built one slice earlier sits behind a
different predicate (`len(deps.LiqPolicies) > 0`) that no module fixture sets, so
**its wiring is still unexercised** — filed as **#216** rather than claimed.

**ONE CLAIM IS COVERED BY THE LINTER RATHER THAN THE SUITE, VERIFIED BY RUNNING
IT.** Removing `OutcomeBlocksOpening` from `latching()`'s case list survives `go
test` — the trailing `return true` (there for a corrupted enum) absorbs a
declared member's removal. `exhaustiveswitch` refuses it by name.

**NEXT, IN THE ORDER IT MUST LAND:** (1) the ambiguity's **clearance** — no
operator act exists to lift it, which is unreachable today because nothing writes
a `MarginOp`, and a permanent halt the day one does; (2) Binance's
`positionMargin/history` evidence half, so no Binance op is over-`Ambiguous`ed in
production; (3) the **dispatch** path (before-state read, fsync,
`AddIsolatedMargin`, one-op-in-flight-per-position). Then B4 — the escalation
that finally calls `Deleverer.Open`.

**Unchanged:** three standing operator decisions ($20/$55 live test, #171's
credential, §38f); credentials deferred; capture **PID 8495**; never rebuild over
`bin/arblab`; `make ci` runs `-race` AND the ka staleness gate (needs node); cap
`TLC_WORKERS` at 4; docs/i18n parity is `bun test`. **#180 is untouched.**

---

## Status (2026-08-08, session 124 — HISTORICAL): **§7.1's LIQ-DISTANCE TRIGGER IS BUILT — AND §5.2's FOUR THRESHOLDS ANSWER TWO QUESTIONS, NOT ONE LADDER. DOCS v0.9.61, 36 MUTANTS / 2 REAL SURVIVORS (BOTH FIXED, THE SECOND FOUND BY THE MIRROR), `make ci` GREEN UNCACHED BOTH MODULES, OWED GREP ZERO.**

**Read `docs/superpowers/specs/2026-08-08-m4-210-liq-distance-monitor.md` (§80)
first** — the deleted band, #209's convergence argument, the side that cannot be
inferred, the fence that did not object, and the survivor that turned out to be
`Money.Div`'s.

**THE CHAIN OF DEFERRALS TERMINATES.** §12's milestone-3 closure pass wrote that
*"eight comments across seven packages defer liq-distance to a computation no
package owns"*. `oms/application.LiqMonitor` is that computation, and it is the
first consumer of BOTH read ports — so the doubled wiring deferral #207 and #208
each recorded is discharged and both adapters are wirable into `oms.Module`.

**THE HEADLINE: §5.2's FOUR NUMBERS PARTITION THE LINE INTO FIVE INTERVALS.**
Read as four rungs of one ladder they delete **[25%, 30%)** — below the opening
threshold, above the alert — where openings are withheld and nothing else
happens, which is where a position spends most of its drift. The repair is not a
fifth member: the members are points in a PRODUCT of two properties, so the
policy answers `Band(d)` and `PermitsOpening(d)` **separately** and neither is
derivable from the other. Without a fixture in that interval the two functions
return the same answer on every input and the second axis is untestable.

**#209 IS RULED, AND IT TURNS ON CONVERGENCE RATHER THAN CAUTION.** An unstated
liquidation price **withholds** openings (never "far from liquidation") and does
**not** escalate — because §7.1's remedy is a `PackageDelever` toward reduced
notional, which RAISES headroom, which pushes the liq price further outside the
band that made Bybit withhold it. **The trigger's own remedy strengthens the
trigger**, so it has no fixed point except flat: escalating would de-risk every
well-margined Bybit position to zero, one generation at a time, with the safety
system appearing to work throughout. What would convert absence into a LOWER
BOUND is the symbol's price band, which this system does not read.

**A FLAT INSTRUMENT WITHHOLDS NOTHING, AND THAT ONE IS FORCED.** A uniform "no
distance ⇒ veto" makes the permitted region unreachable from a flat book, because
the evidence that lifts the veto — a position's liquidation price — is produced
only BY opening. Its mark is still screened, so the staleness block does not
become conditional on already holding the exposure it prevents acquiring. This is
also why `OpeningWithheld` is a VETO and not a permission.

**THE SIDE IS A PARAMETER AND CANNOT BE INFERRED.** `liq > mark` looks
sufficient and misses the case that matters: a short at or past liquidation is
indistinguishable from an ordinary long. Where it SHOULD come from is the venue
row that states the liq price (Binance `positionAmt`'s sign — which #208's codec
parses and discards; Bybit a `side` field the wire struct does not declare) —
filed as **#211**, not absorbed, because #208's narrowing ruling is one day old
and Bybit's flat-position `side` spelling is unverified.

**THE CAMPAIGN'S SECOND SURVIVOR WAS SEVERAL MILESTONES OLD.** `Price.Div`'s
zero-divisor guard is redundant with `mulDivMantissa`'s, so `o.p == 0` → `o.p < 0`
survives — it only narrows the NEGATIVE divisor, which nothing in `fixedpoint`
ever exercised. Carrying the mutant to its mirror found the identical hole in
**`Money.Div`**. Both covered now, on both types.

**`app/reconcile`'s IMPORT FENCE GAINED A SECOND EXEMPTION BECAUSE THE FIRST
SILENTLY COVERED THIS EDGE.** `oms/application` now imports `risk/domain` (§5.2's
policy is single-sourced with a second consumer in `risk` — §6 step 4's sizing),
and `risk/domain` was already on the list, admitted for the account-posture gate.
Nothing went red. Split in two now, each half with its own argument, so the
gate-keyed lapse check names only the packages whose reason really is the gate.

**NEXT:** B3 (`MarginOp`, first producer for `boot.GateMarginOpsResolved`), then
B4 — the escalation that finally calls `Deleverer.Open`, drives this monitor, and
brings the gate that refuses a trading process with no liq trigger (§7.5's
checklist has no member for it, and adding one the reconcile root cannot produce
would refuse the only binary there is).

**Unchanged:** three standing operator decisions ($20/$55 live test, #171's
credential, §38f); credentials deferred; capture **PID 8495**; never rebuild over
`bin/arblab`; `make ci` runs `-race` AND the ka staleness gate (needs node); cap
`TLC_WORKERS` at 4; docs/i18n parity is `bun test`. **#180 is untouched.**

---

## Status (2026-08-08, session 123 — HISTORICAL): **§4's `AccountSource.MarginStatus` IS BUILT ON BOTH VENUES — AND THE WORD §4 USED NAMES A QUANTITY NEITHER VENUE PUBLISHES PER POSITION. DOCS v0.9.60, 32 MUTANTS / 1 REAL SURVIVOR (FIXED), `make ci` GREEN UNCACHED BOTH MODULES, OWED GREP ZERO.**

**Read `docs/superpowers/specs/2026-08-08-m4-208-margin-status.md` (§79) first**
— the three readings of "available margin", the two documented spellings of "no
liquidation price", the re-derived V3 choice, the survivor and the comment it
corrected.

**§7.1's TRIGGER NOW HAS BOTH ITS INPUTS.** #207 landed the mark; this lands the
account half. `MarginStatus` on both venues (Binance `GET /fapi/v3/positionRisk`
scoped to one symbol, weight 5; Bybit `GET /v5/position/list` with
`category`+`symbol`, request-metered), `ports.VenueMarginStatus` +
`MarginReading`, and `driven.VenueMarginStatus` — the ACL that pins `Safety`.

**THE HEADLINE: §4 LINE 331 SAID "AVAILABLE MARGIN", AND THERE IS NO SUCH
PER-POSITION QUANTITY.** Three readings, coming apart exactly where this trigger
fires. The account's FREE balance is real and is keyed on nothing — Binance
publishes `availableBalance` at the **top level** of `GET /fapi/v3/account`,
beside the positions array — and §7.2 already gives `treasury` the rescue-margin
float. The position's MARGIN BALANCE is published by neither venue, so returning
it would be a subtraction where the venue's computation is the definition. The
**POSTED** figure is what both publish (`isolatedWallet` / `positionIM`), and it
is now named for what it is: the difference is **unrealized PnL**, the quantity a
rally moves against the short and largest at the instant the threshold breaches,
so `posted − maintenance` is NOT headroom and §4 now says so. **The tell was the
venue's own deprecation notice** — Bybit's `positionBalance`, the field a codec
written from §4's word would have grabbed first, is documented `Deprecated`.

**BOTH VENUES SPELL "NO LIQUIDATION PRICE" DIFFERENTLY AND BOTH SPELLINGS
COMPUTE.** Bybit, verbatim: `liqPrice` keeps `""` when it falls outside the
symbol's price band — the **well-margined** case, so absence arrives on the
healthy path. Binance documents no absence spelling at all. Read as a zero,
either one puts a **short's** liquidation far below its mark, and every distance
formula reports maximum headroom: the trigger does not fail, it **retires**. So
absence is a flag beside the value, the rule is stated once for both codecs, and
an unreadable value stays an ERROR — §5.3's rule is what stops the conservative
arm swallowing a format change.

**THE V3-vs-V2 CHOICE WAS RE-DERIVED, AND LANDS ON V3 FOR A DIFFERENT REASON.**
§7.5 needs a missing row to mean flat; a symbol-scoped call never asks that. This
one needs a flat symbol not to answer with a **row of zeros**, which V2 gives.
Two consequences had to be acted on: V3 answers for "position **or open
orders**", so a resting order on a closed position yields a zero-amount row — a
row is not a position; and Bybit returns a row "regardless of having position or
not", so the two venues spell "no position" **oppositely** and §7.5's enumeration
trained the reading that is wrong here.

**TWO CODECS OVER ONE ANSWER, AND `markPrice` IS DECODED FROM NEITHER.** Record
§78 §5's ruling built: the position codec's narrowing stands, so this surface
reads the amount as a **presence test** only. The mark is on both venues' position
rows and stays with its own class and port — a second producer would give §7.1's
ratio a numerator and denominator from two different clocks.

**THE CAMPAIGN.** 32 mutants, all caught. One real **SURVIVED**: the adapter's own
`reading.Validate()` had no failing input, because every fixture came from the one
implementation that cannot fail it (the reader is an **interface**). Diagnosing it
also corrected the comment beside the call, which claimed the field-for-field copy
made a **transposition** visible there — it does not, since both money fields are
non-negative and in the same asset. One `NOBUILD!!` (bad sed, "declared and not
used") and two `NO-OP!!` re-aimed. **The collapse checklist paid for itself before
the campaign**: the zero-margin fixture it demanded is exactly what killed
`sign < 0` → `sign <= 0`.

**ALSO FIXED, AND IT WAS THIS DOC SET'S OWN CLAIM:** `adr-history.md`'s v0.9.59
status line said §3 gained a task #207 block. It did not. Both #207's and #208's
blocks are now in §3.

**NEITHER ADAPTER IS WIRED INTO `oms.Module`, and the deferral is now doubled** —
`markstate` and this one both wait on the same trigger: **B2's liq-distance
monitor**, the first consumer of either. A root wiring them today drags
`engine/state` into every OMS binary for nothing.

**NEXT:** B2 (liq distance + §5.2's policy, the first consumer of both ports),
then B3 (`MarginOp`, first producer for `GateMarginOpsResolved`), then B4 (the
escalation that finally calls `Deleverer.Open`). Task **#209** is filed: what the
ladder concludes from a liq price the venue declines to state — the decoder
reports absence faithfully and deliberately does not answer it.

**Unchanged:** three standing operator decisions ($20/$55 live test, #171's
credential, §38f); credentials deferred; capture **PID 8495**; never rebuild over
`bin/arblab`; `make ci` runs `-race` AND the ka staleness gate (needs node); cap
`TLC_WORKERS` at 4; docs/i18n parity is `bun test`. **#180 is untouched** — the
connector carries no weight annotations, so the charged 5 is still the
conservative maximum over three unread candidates, and it is now the second line
one browser read would loosen.

---

## Status (2026-08-08, session 122 — HISTORICAL): **THE MARK IS §3.4's THIRD DATA CLASS, END TO END — §7.1's LIQ TRIGGER HAD NO INPUT IN THE SYSTEM AT ALL. DOCS v0.9.59, 16 MUTANTS / 1 REAL SURVIVOR (FIXED), `make ci` GREEN UNCACHED BOTH MODULES, OWED GREP ZERO.**

**Read `docs/superpowers/specs/2026-08-08-m4-207-mark-data-class.md` (§78) first**
— the two independent reasons the mark is its own class, the stateless-decoder
correction, the `p`/`P` finding, the two gates that moved the read port, and the
correction to #207's own description.

**THE SURVEY IS THE HEADLINE.** `Deleverer.Open` is built, journaled, reinstated
at boot and filter-conformed, and **nothing calls it**. Core §7.1's chain is what
should — liq breach → top-up → `PackageDelever` — and every link is unbuilt:
no liq-distance code, no `MarginOp` (`boot.GateMarginOpsResolved` is one of the
four gates `reconcile/app.go` names as *"no producer anywhere"*), no
`AccountSource.MarginStatus`, and **no code anywhere decoded a mark price**.
`venue.BinanceMarkPrice` reads `e/E/s/r/T` and says so: *"the funding-relevant
half"*.

**WHAT LANDED.** `venue.Mark` + `DecodeBinanceMark` + `DecodeBybitMark` (both
stateless, both taking the assets a price needs); `marketdata/domain.MarkObservation`;
`ports.MarkSnapshot` / `MarkPublisher` / `MarkBudgets`; `application.MarkIngest`
with its **own** silence timer; `engine/state`'s third map (`MarkEntry`,
`PutMark`, `Mark`, and `Len` widened to three); `oms/application/ports.MarkRiskReader`
+ `MarkReading`; `oms/infrastructure/markstate.Reader`. Plus `fixedpoint.Price.Sign()`,
`Money.Sign`'s twin, which had no caller until a mark needed screening.

**THE MARK IS A CLASS, NOT A FIELD — AND THE SECOND REASON WAS NOT IN THE SPEC.**
On both venues the mark rides funding's frame. §5.2 settles it once (one struct
is one `Budget`, so the mark would inherit funding's tolerance). The reason that
would not have been guessed is Bybit's: `BybitTickerState` emits *only* on
funding news, deliberately discarding the ~10/s churn deltas — which ARE the
mark's news. A folded mark refreshes about once a minute, which is §7.1's own
*"MUST NOT be account-event-only"* defect reached through a different event.

**THE `p`/`P` FINDING, AND IT IS THIS PACKAGE'S SECOND.** `encoding/json` falls
back to CASE-INSENSITIVE matching for an undeclared key, so Binance's `P`
(estimated settle) landed in the `json:"p"` mark field and **key order decided
it**. The wrong value is a dollar off, validates, and computes. `binanceFuturesDepthWire`
already declares `U` for this exact reason (session 52) — and that note sat on
the struct that had the defect, so it said nothing to the next single-letter
wire written two files away. §4 now states the rule for the **seam**; both
comments cross-reference; the fixture asserts **both key orders**.

**TWO GATES MOVED THE READ PORT AND THE SECOND IMPROVED IT.** `singlewriter`
refused `engine/state` naming an `oms` port — rule 6 closes the package, so
§40b's read-seam pattern (*"or the engine subsystem that holds the data"*) **does
not transfer to `oms`**. `app/reconcile`'s import fence then refused
`oms/infrastructure/driven` (its graph reaches `marketdata` via `engine/state`),
and finally refused `marketdata.Watermark` on the port itself. Being forced off
the watermark is what produced the good shape: the port **returns AGES and takes
the instants**, so "the age is derived at the READ" is a property of the
signature, and the two-clock arithmetic stays single-sourced where it is tested.

**THE CAMPAIGN.** 16 mutants, all caught. One real **SURVIVED**: `Len` reporting
the funding count as the mark count — every fixture had them equal or the mark
discarded, and the test's own comment already said *"ASYMMETRIC counts on
purpose"*. Now `(1, 3, 2)`. Two `NOBUILD!!` were bad seds ("declared and not
used") re-aimed as transpositions.

**WHAT IS NOT DONE, AND #207's DESCRIPTION WAS WRONG ABOUT IT.** Part 2 is the
ACCOUNT half. #207 filed it as *"un-narrow `binancePositionWire` for
`liquidationPrice`/`isolatedMargin`/`maintMargin`"*; §4 line 331 already declares
`AccountSource.MarginStatus(VenueInstrument) (MarginStatus, error)` as its own
port, so part 2 is **implementing a declared port**, and the position codec's
narrowing **stands** — §7.5's net-delta invariant genuinely should not acquire a
second source for those numbers. On Binance it reads the same endpoint, which
makes it the same two-codecs-over-one-answer shape this slice built for the mark.
Then B2 (liq distance + §5.2's policy), B3 (`MarginOp`, first producer for
`GateMarginOpsResolved`), B4 (the escalation that calls `Open`).

**The adapter is deliberately NOT wired into `oms.Module`** — it has no consumer
until B2, and a root wiring it today drags `engine/state` into every OMS binary.
The deferral names its trigger: **the first `MarkRiskReader` consumer.**

**Unchanged:** three standing operator decisions ($20/$55 live test, #171's
credential, §38f); credentials deferred; capture **PID 8495**; never rebuild over
`bin/arblab`; `make ci` runs `-race` AND the ka staleness gate (needs node); cap
`TLC_WORKERS` at 4; docs/i18n parity is `bun test`.

---

## Status (2026-08-07, session 121 — HISTORICAL): **SM §7's LADDER RE-VERIFIES §2.4's GRID AND §69's ROW DATE, PER LEG — AND #184's OWN FIRST OPTION WAS REFUTED BY A RULE ONE FILE AWAY. DOCS v0.9.58, 14 MUTANTS ALL CAUGHT, `make ci` GREEN BOTH MODULES, OWED GREP ZERO.**

**Read `docs/superpowers/specs/2026-08-07-m4-184-ladder-filter-conformance.md`
(§77) first** — the refutation that decided the design, the policy/check split,
the stranding argument, the one finding, and the six rejected alternatives.

**WHAT LANDED.** `application/filters.go` — `filterPolicy` (the map and its
row-age bound as ONE value), `newFilterPolicy` (the three wiring refusals, moved
off `NewExecutor`), and `admits` (conformance **then** date, at a caller-chosen
instant). The `Dispatcher` now **holds** the policy and deliberately does not read
it; `NewExecutor` **loses two parameters**; `Deleverer.verifyLeg` gains a fifth
check, last. `oms.Module` wires one policy beside the one authority.

**THE HEADLINE: #184 ASKED WHETHER THE PAIR SHOULD MOVE ONTO `Dispatch`, AND IT
MAY NOT.** `Execute` already says why, in a comment: it stamps ONE admission
instant per package because *"a two-leg package judged at two instants could
straddle the filter-age bound and refuse asymmetrically"* (§62d). A freshness
check inside `Dispatch` reads its own instant **per leg, inside the fan-out,
after siblings may already have been sent** — that harm re-opened from the one
method that cannot see it. So the check goes to each caller's **admission unit**:
the package for the Executor, the **single leg** for the ladder, whose two legs
are sequenced across a finality wait because the spot cap is the *confirmed* perp
reduction. A row fresh for the perp leg may be stale for the spot leg, and that
is correct.

**THE POLICY MOVED EVEN THOUGH THE CHECK DID NOT.** §59b's argument on a second
object: a root that could hand the Executor one grid and the Deleverer another
would fragment §2.4's enforcement with no test able to see it, and the realistic
accident is `NewDeleverer(..., nil, 0)` — legal, meaning "plans nothing", and it
refuses every delever leg during an incident. Inherited, a second policy in one
process is not expressible from outside the package. **The Planner keeps its own
map on purpose**: plan time and admission time are different enforcement points,
and the whole reason admission re-verifies is that they may differ.

**A SPOT-RUNG REFUSAL DOES NOT STRAND THE LADDER, AND THAT IS THE SHARP PART.**
`SubmitSpotReduction` is un-premise-guarded because TLC returned a 16-state
stuttering behaviour for the guarded variant. This is not that shape, twice: an
off-grid order is one the **venue** refuses too (the premise guard would have
refused an order the venue would have *filled*), and the refusal is a **read**
taken before any journal write, so the same rung admits a leg replanned onto the
grid — where a venue rejection burns the id's one unrepeatable `Place`.

**ONE FINDING, FILED AS #206.** `verifyLeg` requires the leg's quantity to EQUAL
the generation's own figure, so §2.4's three QUANTITY members bite only on an
off-grid **target or authorization** — and then no conforming leg exists at all.
Pre-existing (the venue refused the same order before, later and with a burned
id); the slice made it the only route to those branches. Not fixed, because the
fix is a **derivation** and `filters.go` puts derivation with sizing — which is
§7.1's escalation trigger, and does not exist yet. The recoverable member is the
**price**, which the generation does not pin, and that is what the anti-stranding
fixture had to vary.

**THE CAMPAIGN.** 14 mutants, all caught. Three isolated to exactly one test: the
ordering swap, `conformsToFilters` substituted for `admits`, and journal-before-
verify. **One claim has no single-line mutant and it is stated rather than
implied** — "the instant is re-read per leg" can only be broken by CACHING it,
which is a design change, so the test defends a future refactor.

**WHAT IS NOT DONE.** Triggering the ladder from core §7.1's liq-distance
escalation — nothing calls `Open`, and it is now milestone 4's **only** open
ladder work. Task **#206** (above) and task **#205** (`Classifier.Track`'s second
caller) are untouched by this slice.

**Unchanged:** three standing operator decisions ($20/$55 live test, #171's
credential, §38f); credentials deferred; capture **PID 8495**; never rebuild over
`bin/arblab`; `make ci` runs `-race` AND the ka staleness gate (needs node); cap
`TLC_WORKERS` at 4; docs/i18n parity is `bun test`.

---

## Status (2026-08-07, session 120 — HISTORICAL): **THE LIVE GENERATION IS REINSTATED AT §7.5 BOOT — SM §7's LADDER HAS ITS ENTRY BACK IN §6.7's SHARED LEDGER, AND THE GATE IT NEEDED COULD NOT BE THE REPLAY'S. DOCS v0.9.57, 29 MUTANTS / 3 REAL SURVIVORS (ALL CLOSED), `make ci` GREEN BOTH MODULES, OWED GREP ZERO.**

**Read `docs/superpowers/specs/2026-08-07-m4-204-generation-reinstatement.md`
(§76) first** — the quantity that decided the slice's shape, the seam that
answers §75h without weakening it, the three findings, and the eight rejected
alternatives.

**WHAT LANDED.** `internal/oms/application/reinstate.go` — `GenerationLadder`
(three whole acts of the ladder, each atomic over the aggregate AND its ledger
entry) and `Reinstater`, which drives SM §7's own transitions through the
recorded phases. `domain.RestoreGeneration` + `RestorePerpSubmission` +
`RestoreSpotAuthorization` — `OpenGeneration` and its two premise-guarded steps
with the premise removed and **nothing else**, sharing their payload closures
with the live ones. `Deleverer.Restore` (track → reserve → amend → install,
releasing on any failure) and `Deleverer.BarLadder`, with the bar read by `Open`
**before** the live check and written by `supersede`.
`boot.GateGenerationsReinstated` + `Reconciler.reinstateGenerations`.
`ports.OrderLookup` — the read half of the write repository, cut off on
`Journal`/`JournalReplay`'s argument. `oms.Module.Delever` is the **Deleverer's
first composition root**.

**THE HEADLINE: THE NUMBER RECOVERY NEEDS IS IN NO JOURNAL, AND THAT DECIDED
EVERYTHING.** A generation's entry carries its CONFIRMED PERP REDUCTION.
`ports.Journal` has no `AppendFill`; the generation journal has no fill record;
the replay leaves every cumulative at **zero**. The number reaches this process
only from the VENUE, so the rebuild runs after the join and after §5.3's absence
resolution — a different phase from the replay, which is why #203's refusal
**moved** to a gate of its own rather than lifting in place. Rebuilding earlier
is not merely early: an authorized generation would be rebuilt against a
confirmed perp reduction of zero and refused by its own never-net-short cap.

**`RecordPerpFill`'s COMMENT SAID THE OPPOSITE, IN BOTH HALVES.** It justified
storing no fill record by saying the quantity was "durable at order scope" and
rebuilt by "§7.5's replay". Neither is true, and `replay.go` says so in as many
words one file away — two comments contradicting each other for two slices,
because nothing had needed the number. The conclusion survives on a different
premise (the join re-derives it) and that premise is what pins the phase's
position. Corrected in place.

**THE SEAM ANSWERS §75h WITHOUT WEAKENING IT.** The way through is a different
SUBJECT, not a narrower `Classifier`: what recovery needs is "make the ladder
true again", and the ladder is the one object required to keep the generation and
its entry in step. So `Reinstater` holds `GenerationLadder` + `ports.OrderLookup`
and can reach no ledger, no journal and no placer; the `Replayer` still holds no
`Classifier`. **The reserve is inside `Restore`**, because a reinstatement that
left the caller to reserve would move §7's "exactly backed at every instant" into
a second place — the shape FM-6 came out of.

**TWO MORE FINDINGS.** `Classifier.Track` had **zero non-test callers**, so no
hedge group was ever registered — the reinstatement is the first thing that
tracks, with the asset from the generation's own target. And
`TestOpenAdmitsANextGenerationOnceTheFirstHasEnded` had been asserting the
**opposite** of `ResumesLadder`: it quotes §7.1's *"Complete → the ladder
continues"* and ended its generation with an `EmergencyClosePackage`
supersession. The ending KIND was a collapsed dimension for as long as nothing
acted on the path.

**THE THREE SURVIVORS.** The bar's first-writer-wins rule; the replay's bar
derivation, which had **no test at all**; and last-writer-wins, whose fixture had
to be SCRIPTED because the log it needs is one this build can no longer write. A
fourth mutant SURVIVED and was **equivalent** — its claim lives in
`RestoreGeneration`'s SIGNATURE, and adding the parameter reports `NOBUILD!!`,
which is this repo's kill-by-compiler verdict.

**WHAT IS NOT DONE.** Triggering the ladder from core §7.1's liq-distance
escalation — milestone 4's other workstream; nothing calls `Open`. Task **#184**,
the delever legs' §2.4/§69 re-check, trigger already fired. And the `Track` gap
one layer out: every hedge group that never had a generation is still untracked,
so §6.7's ledger does not exist for it.

**Unchanged:** three standing operator decisions ($20/$55 live test, #171's
credential, §38f); credentials deferred; capture **PID 8495**; never rebuild over
`bin/arblab`; `make ci` runs `-race` AND the ka staleness gate (needs node); cap
`TLC_WORKERS` at 4; docs/i18n parity is `bun test`.

---

## Status (2026-08-07, session 119 — HISTORICAL): **SM §7's GENERATION RECORDS ARE IN §5.1's CHAIN — `ports.GenerationJournal` HAS A STORE, AND §7.5's REPLAY RULE TURNED OUT NOT TO REACH A SECOND SCOPE. DOCS v0.9.56, 23 MUTANTS / 1 REAL SURVIVOR (FIXED), `make ci` GREEN UNCACHED BOTH MODULES, OWED GREP ZERO.**

**Read `docs/superpowers/specs/2026-08-07-m4-203-generation-journal-adapter.md`
(§75) first** — the migration's two independent survival reasons, the outcome
table with its four rejected members, the survivor and what it taught, and the
seven rejected alternatives.

**WHAT LANDED.** `internal/oms/infrastructure/driven/generationjournal.go` — the
six `Append*` methods, `AppendGenerationPhase`'s payload refusal enumerated
POSITIVELY so a new phase lands in neither list and `exhaustiveswitch` bites.
`journalcodec.go` — six kind tags and six encoders; the tag is now load-bearing
four more times, and the perp/spot pair is the first collision between two
records of one machine. `journal.go` — `journal.client_id` → `journal.subject`,
five detail tables, ten triggers, and `migrateSubjectColumn` with its two
refusals. `journalreplay.go` — nine joins, six preimage arms, six dispatch arms.
`ports.JournalVisitor` gained six methods, which broke every visitor in the
system at build time; that is the property being bought.

**THE HEADLINE: ONE CHAIN BUYS A FACT, AND THE TEST IS WHAT COLLECTS IT.**
`TestTheAuthorizationRecordPrecedesTheSpotLegInOneChain` asserts FLAG-3's
authorization sits below both the generation's spot submission and the spot
order's own `planned` record. **Under two chains that test could not be written**
— each verifies internally, neither positions the other — so the clause would
have held exactly as long as the application layer kept it.

**THE MIGRATION'S SCARIEST PART IS A NON-EVENT, AND CHECKING IT WAS THE WORK.**
#203 warned that widening the shared column might change the RECORD. It does not:
every encoder hashes DOMAIN VALUES, not columns, so the rename moves no byte of
any preimage and `chainLabel` keeps `/v1`. That is now core §5.1 **ruling (c)**,
and it is a fixture — a hand-built old-shape database, written with the oracle's
own literal-spelled encoders, that still verifies after `Open` migrates it. The
append-only triggers are untouched for a SEPARATE reason: they police row writes,
and a column rename is neither.

**§7.5 GAINED THE ANSWER ITS OWN REPLAY RULE DOES NOT GIVE.** That rule rebuilds
state *through the write path the live system uses* — a statement about ORDERS. A
generation's recovered state is a live entry in §6.7's **shared** ledger, which is
process memory nothing yet reinstates. So a journal holding a generation with no
ending record is **`Incomplete`**, never `Passed`: a ledger missing an entry reads
as MORE room, and §6.7's determined branch would buy back exposure the
generation's own perp leg already committed to — `FM-5` through recovery,
invisible to every invariant computed from that ledger. **Liveness is decided by
`GenerationPhase.IsActive`, never by a kind→phase table**, which would be a second
home for §7's transition set.

**THE ONE SURVIVOR WAS THE MIRROR RULE COLLECTING.** M14b — `replayGenerationOpening`
stops validating the recovered hedge group — SURVIVED on a real gap: every check
on the replay's side has a twin on the write path, so **no journal this adapter
produces can reach any of them**, and every natural fixture is built by writing.
The bypass fixture existed (`tamperColumn`, since slice F1) and its trigger list
had not grown with the five new tables. Fixed; M14c caught.

**WHAT IS NOT DONE, BOTH FILED.** Task **#204** — rebuild the aggregate and
re-`Reserve` its ledger entry; it needs a place to put the generation AND a seam
that re-enters the reservation **without** handing the replayer a `Classifier`
(§75h refuses that composition). It also owes acting on `ResumesLadder`, which is
validated at replay and not yet obeyed. **#204 is what unblocks wiring the
`Deleverer`**, since the boot hold fires the day a root constructs one. Task
**#184** — the delever legs' §2.4/§69 re-check, trigger already fired.

**Unchanged:** §7.1's escalation *trigger* half is milestone 4's other workstream
and untouched; three standing operator decisions ($20/$55 live test, #171's
credential, §38f); credentials deferred; capture **PID 8495**; never rebuild over
`bin/arblab`; `make ci` runs `-race` AND the ka staleness gate (needs node); cap
`TLC_WORKERS` at 4; docs/i18n parity is `bun test`.

---

## Status (2026-08-07, sessions 117–118 — HISTORICAL): **SM §7's `PackageDelever` GENERATION LADDER EXISTS. `FM-9` HAS A CONSUMER, `FM-8`'s OBLIGATION IS DISCHARGED, AND BUILDING IT SETTLED FOUR THINGS THE SPEC HAD NOT SAID. DOCS v0.9.55, THIRTEEN MUTANTS ALL CAUGHT, `make ci` GREEN BOTH MODULES, OWED GREP ZERO.**

**Read `docs/superpowers/specs/2026-08-07-m4-177-generation-ladder.md` (§74)
first** — the four rulings with their failure directions, the seven rejected
alternatives, the two mutants that point opposite ways on one signal, and what
this deliberately does NOT do.

**WHAT LANDED.** `internal/oms/domain/{generation,generationphase}.go` — the
ten-phase aggregate, SM §7's transition table, the never-net-short spot cap, and
`CheckAdvance`/`CheckSpotSize`/`CheckSupersede` as *questions*, so a phase can be
journaled before the aggregate carries it. `internal/oms/application/ports/generationjournal.go`
— six methods, one per payload, so a phase carrying a target, a leg id, a size or
a path is **not reachable** through the phase recorder at all.
`internal/oms/application/delever.go` — the ladder, FLAG-3's durability, the
ledger kept in step by construction, and `Reassess`, which is model (a)'s
`P5`/`NC23` discharge. `Classifier.PremiseFor` computes `FM-9`'s netted quantity
with the **same** fold and subtraction as the ordinary verdict, so the premise and
the verdict cannot drift.

**THE HEADLINE: FOUR RULINGS CAME OUT OF BUILDING THE MACHINE, NOT READING IT.**
(1) **The quiesce obligation needs no cancel port.** Every order is a marketable
IOC, so "quiesce the reserved second leg to §6.7 finality first" is *awaiting*
finality — which is why no `Cancel` exists anywhere, and why the wait is bounded:
every successor of the two phases it binds on is either the other one of them or
an ending. (2) **The entry gate and the standing trigger want different
thresholds on one counter.** `PremiseFor` refuses at >1 `PackageDelever` entry
(the netting keys on producer); SM §7's entry rule is ==0. So `Open` **reserves
before it asks**, which makes a stale sibling the second entry and turns the
existing refusal into the gate — asked the other way round the sibling's entry is
silently netted out, `FM-9` through the decomposition. (3) **Ending a generation
RELEASES its entry; it does not amend it to zero.** Same final ledger, different
act: what the entry carried at the ending is real exposure the generation put on
and did not take back, and releasing is what lets §2 see it. Mutant M2 confirms
the suite tells them apart. (4) **Ladder resumption is a property of the PATH** —
premise-lost resumes, the two external arbitrations do not — which was the one
`SPEC EDIT OWED` marker and is now SM §7's own invariant.

**TWO FINDINGS BESIDE THEM.** `IssueFloor` booked the floor order's own exposure
**change** where the ledger wants the **exposure removed** — the direction `FM-5`'s
amount rule left open, and the wrong sign makes the determined branch sell 1.4
against 0.6 of naked long. And `ambiguous-order-model.md`'s clause map asserted a
reachability route (*"or via `Supersede`"*) that reaches `PdPlan`'s guard while
permanently disabling the action, so the model is **silent** on ladder resumption
rather than supporting either answer — corrected in place.

**TWO MUTANTS, ONE SIGNAL, OPPOSITE VERDICTS.** M3 (collapse
`premiseOrDischarge`'s three branches) **SURVIVED and the mutant was wrong** — the
branches report, `CheckAdvance` enforces one layer down, and M3b (report the
supersession, never perform it) is what negates the claim. M12 (delete
`verifyLeg`'s hedge-group check) survived on a **real gap**: every fixture planted
its legs on the group by construction. `exhaustiveswitch` bit on a **test fake**,
catching the fake journal's phase switch on its first run.

**WHAT IS NOT DONE, BOTH FILED.** Task **#203** — `ports.GenerationJournal` has
only a test fake, so FLAG-3's durability is a contract nothing honours yet; the
design fork is one chain over a widened subject column (migration needed) versus
two chains, and one chain wins because it is the only shape in which *the
authorization precedes the spot order* is a fact a replay can check. Task
**#184** — its stated trigger has **fired**: the `Deleverer` is the single-order
`Dispatch` path's first real caller, and its legs get no §2.4/§69 re-check.

**Unchanged:** §7.1's escalation *trigger* half (liq-distance on the mark stream,
`MarginGateway`, sizing-by-live-tier) is milestone 4's other workstream and
untouched; three standing operator decisions ($20/$55 live test, #171's
credential, §38f); credentials deferred; capture **PID 8495**; never rebuild over
`bin/arblab`; `make ci` runs `-race` AND the ka staleness gate (needs node); cap
`TLC_WORKERS` at 4; docs/i18n parity is `bun test`.

---

## Status (2026-08-07, session 116 — HISTORICAL): **#199 IS RULED AND SLICE A HAS NO OPEN QUESTIONS. `Scope` WAS DOING DUTY FOR A PROPERTY IT DOES NOT CARRY. DOCS v0.9.54, NINE MUTANTS ALL CAUGHT, `make ci` GREEN UNCACHED BOTH MODULES, OWED GREP ZERO.**

**Read `docs/superpowers/specs/2026-08-06-m4-198-account-posture.md` §73p
first** — the fifteen-row subject table, the four rejected alternatives, the
mutant table, and what it deliberately does NOT settle.

**THE HEADLINE: THE LAST OPEN QUESTION WAS NOT A TRADE-OFF.** #199 asked whether
a spot-only Binance deployment must route the futures family, framed as config
burden vs. coverage loss. A rule's **scope** is the granularity it is *answered*
at; the family requirement was derived from the union of all probes, which reads
that as the rule's **subject** — which part of the deployment it *governs*.
`observationKey`'s own comment had said those differ since the rules landed
(`positionMode` is `ScopeAccount` on Binance and `ScopeSymbol` on Bybit, one
subject at two granularities), stated as a fact about one setting and never as a
fact about the table. `posture.go`'s doc comment said the opposite outright.

**TABULATING ALL FIFTEEN ROWS SETTLED IT, AND THE TWO VENUES DISAGREE.** Every
**probed** Binance rule governs the derivatives leg; the one whose subject
reaches the spot wallet is Portfolio-Margin, which is attested and needs no
endpoint. So a spot-only Binance deployment issues **zero** probes and still
enforces the only check that could hurt it. A spot-only **Bybit** deployment
still requires linear-perp, because `/v5/account/info` rides it and answers three
**account**-subject settings. **Same question, opposite answers, one rule** —
which is why this is an axis on the rule and not "Binance is exempt". The
plausible wrong implementation (*skip the posture families when the roster has no
derivative*) passes the Binance test and fails
`TestASpotOnlyBybitDeploymentMustStillRouteTheLinearFamily`.

**THE LAPSE IS STATED, NEVER SILENT.** `Verdict.Lapsed` carries every rule that
did not run with its subject and the coverage that lapsed it; `Evaluated`
excludes them, so the vacuity guard still refuses a verdict that evaluated
nothing. An exemption can narrow what is asked and can never mint a clean answer.

**NARROWING THE CONFIG WITHOUT NARROWING THE READ WOULD HAVE BEEN WORSE THAN
DOING NOTHING** — it moves F20's failure from config-time to boot-time. So the
want-set travels: `ProbedSettings` → `PostureScope.Settings` → the ACL's inverse
translation → `PostureStepsFor`. `CheckWithoutVenueRead` handles the one shape
with nothing to read and **checks its own precondition**, so it cannot be reused
to judge a Bybit account without looking.

**THREE THINGS FELL OUT, NONE ABSORBED.** (1) `Check`'s per-symbol loop claimed
an empty roster was already refused — **false from the session it was written**,
since the guard is conditional on `RosterCoverageDerivatives` and
`RosterCoverageNoDerivatives` was added by the same slice. (2) §4.1's only
end-to-end refusing-path fixture rested on the same conflation *in a test*
(*"it is an ACCOUNT-WIDE rule, so it fires on a deployment with no derivative"*)
— it read as covered because its assertion is true of the rule in general. (3)
Mutant M7 returned `TIMEOUT!!`: `TestProcessKillMidIOC`'s 60s channel bound tied
`mutate.sh`'s own 60s default, so the guard could never be seen to fire. Now 10s,
matching every other bound in the repo, and the hang became a kill.

**WHAT IS NOW OPEN AND IS NEW — task #201.** Profiles §2's Binance cell checks
**nothing** about a spot leg where the Bybit cell checks three things. A
spot-only Binance deployment's whole §4.1 answer is one attestation and zero
venue reads. That is the rule table faithfully applied *and* the first time the
gap has been visible, because every Binance deployment previously dragged in the
futures probes regardless. Binance spot accounts do support margin borrowing —
the same hazard Bybit's borrow rows refuse. **Filed, not guessed**: blocked on
the same WAF and missing credential as #180/#171, and a rule on a guessed field
name would halt every Binance spot deployment. Try the spec-generated connector
first; it answered §73o's field question. Beside it, #171's unchanged loosening
trigger: whether Bybit's `position/list` answers for a never-traded symbol.

**Everything below the venue codecs is still fixture-tested and has never met a
real account** (§73f). Unchanged: three standing operator decisions, credentials
deferred, capture **PID 8495**, never rebuild over `bin/arblab`, `make ci` runs
`-race` AND the ka staleness gate (needs node), cap `TLC_WORKERS` at 4,
docs/i18n parity is `bun test`.

---

## Status (2026-08-06, session 115 — HISTORICAL): **#199 ITEM 1 — THE OWED OPERATOR RULING WAS A DEFECT REPORT. §4.1's BINANCE GATE REFUSED ON EVERY ACCOUNT, NOT ONLY FLAT ONES. DOCS v0.9.53, NINE MUTANTS ALL CAUGHT, `make ci` GREEN BOTH MODULES, OWED GREP ZERO.**

**Read `docs/superpowers/specs/2026-08-06-m4-198-account-posture.md` §73o
first** — the verified field lists, the fold, the collapse checklist, the mutant
table, and what it deliberately does NOT settle.

**THE HEADLINE: A QUESTION FILED FOR AN OPERATOR TURNED OUT NOT TO BE A CHOICE.**
Item 11 was to put four candidate rulings for §73j F9 / §73k F14 / §73m F18 to
the user. Settling F18's underlying **fact** first removed most of the question.
`GET /fapi/v3/positionRisk` **does not publish `isAutoAddMargin` at all** — so
the probe was aimed at an endpoint without the field, and the gate refused on
**every** Binance account: a flat symbol got no row (`ReasonUnobserved`), a
positioned symbol got a row with the field missing (`ReasonUnstated`). F9's
candidate 1 (*bootstrap a position per symbol*) would not have worked.

**THE FOLD COSTS NOTHING.** `GET /fapi/v1/symbolConfig` publishes **all three**
per-symbol settings for every configured symbol, position or not — and it was
already being probed. So: the read moved, the fourth probe and its enum member
were **deleted** (four probes → three, 80 weight → 60, six checks on three
calls), and **#199's candidate 4 was superseded by something cheaper than
itself.** A flat Binance account now boots clean, which no fixture in this repo
could previously produce.

**THE SOURCE IS THE OTHER HALF OF THE FINDING.** `developers.binance.com`
answers every non-browser client with an **AWS WAF challenge** (#180's blocker,
still total; the old `binance-docs.github.io` mirror now 302s into the same SPA).
WebFetch was deliberately not used — §9 already holds two contradicting
summarizer readings of this endpoint. The way in was
**`github.com/binance/binance-connector-go`**, Binance's own connector whose
models are generated from their OpenAPI spec: raw, version-controlled,
venue-published text. Its V3 field list matches Profiles §9's dated 2026-08-03
transcription **exactly**, so §9's twenty were never an abbreviation. **It
carries no weight annotations at all, so #180 is untouched** — a source answers
the questions it answers.

**THE READING PASS FOUND WHERE THE DEFECT WAS DISMISSED.**
`2026-08-03-m3-slice-f2-venue-join.md` — the record that *established* the V2/V3
distinction — said of the other consumer: *"where the parenthetical is
harmless."* Struck in place. **Second instance of one pattern in this project**,
after §73k F14's *"Bybit is not symmetric — answerable regardless"*: a
subordinate clause ruling a neighbouring consumer out, inside a record that is
otherwise right. In both cases the falsifier was a grep of a file the slice
already had.

**ONE COVERAGE CONSEQUENCE WAS NOTICED RATHER THAN ABSORBED.** Deleting the
Binance positionRisk posture read removed `ReasonAmbiguous`'s only **end-to-end**
witness (symbolConfig holds one row per symbol whatever the position mode is;
Binance hedge mode is still named account-wide by `dualSidePosition`).
`TestBybitHedgeRowsReachTheDomainAsAnAmbiguity` replaces it — real codec, real
ACL, real rules — because F19's lesson is that a between-layers defect is
invisible to either layer's suite.

**WHAT IS STILL OPEN, and #199 is now genuinely one question.** Whether a
spot-only Binance deployment should have to route the futures family (F14/F20 —
all three remaining probes ride it). Beside it, one **loosening trigger** for
#171: whether Bybit's `position/list` answers for a configured-but-never-traded
symbol — because all three of Bybit's per-symbol settings ride it, so if it
omits such rows, **Bybit now has exactly the shape Binance just lost.**

**Everything below the venue codecs is still fixture-tested and has never met a
real account** (§73f). Unchanged: three standing operator decisions, credentials
deferred, capture **PID 8495**, never rebuild over `bin/arblab`, `make ci` runs
`-race` AND the ka staleness gate (needs node), cap `TLC_WORKERS` at 4,
docs/i18n parity is `bun test`.

---

## Status (2026-08-06, session 114 — HISTORICAL): **#198 ITEM 10 LANDS — THE FINAL `code_diff` REVIEW RETURNED NO-SHIP WITH TWO MAJORS, BOTH FOLDED. DOCS v0.9.52, SIX MUTANTS ALL CAUGHT, `make ci` GREEN UNCACHED BOTH MODULES, OWED GREP ZERO. ONLY ITEM 11 (#199's OPERATOR RULING) REMAINS.**

**Read `docs/superpowers/specs/2026-08-06-m4-198-account-posture.md` §73n
first** — it carries both majors, the three minors, the rejected alternatives,
the review's one wrong line of attack, and the mutant table.

**BOTH MAJORS ARE THE SAME SHAPE, AND IT IS THE ONE A PACKAGE SUITE CANNOT
SEE: a defect that exists only BETWEEN two layers whose own tests are green.**
That is the argument for this ritual, arriving in its cleanest form yet — in
F19's case the two layers' tests encode *contradictory premises about the same
wire shape* and both pass.

**F19 — a hedge-mode account was reported as "could not be read".** Both venue
codecs refused a symbol answered twice, which is hedge mode's documented wire
shape (Profiles §9, verbatim, for Binance V3) and therefore a §4.1 **violation**.
So a posture read and found WRONG travelled through the arm meaning *could not be
read* — `OutcomeFailed` → `ExitIncomplete`, *"nothing is known to be wrong, a
retry may lift it"* — for an account no retry will change, **discarding every
probe already decoded**, including Binance's account-wide `dualSidePosition`
answer that names the same violation directly. Meanwhile `risk/domain` had
carried `ReasonAmbiguous` for exactly this since the rules landed, with a fixture
whose comment calls it *"the posture violation this rule exists to catch"* — and
**no production path could reach it**. Folded: the codecs report both rows for
the two POSITION enumerations, the rules judge them, and folding two rows into
one answer — all the original argument actually ruled out — is still forbidden.

**F20 — §4.1's probes ride product FAMILIES, and §73k F14's reassurance about
Bybit was false of this code.** A family with no configured endpoint makes its
probe unissuable and the gate unanswerable for ever. F14 recorded the Binance
half and added *"Bybit is not symmetric — answerable regardless"*: true of the
exchange, false of the routing, because two of Bybit's four probes ride spot and
two ride linear-perp, so **either** one-sided Bybit deployment is short a family.
**A claim that reads as reassurance is the one nobody re-derives** — the
falsifier was a grep of `Family:` in the file the same slice wrote. Folded as a
startup refusal naming the family, derived from the codec's own encoded steps.
`§73k F14`'s sentence is struck and corrected **in place**.

**THE FIXTURE WAS THE DEFECT'S OWN WITNESS.** This repo's `spotOnly` helper
trimmed the endpoint list to `[:1]`, so every test built on it exercised a
deployment whose posture gate could never conclude, while three tests appended
the futures endpoint back by hand. The new rule refused it on the first run.

**Three minors folded too:** `arblab --help` said the reconcile binary *"produces
four"* of §7.5's gates while its own output printed 5 (and never mentioned the
posture phase); a nil `*venue.OrderClient` would have reached the ACL as a
**non-nil interface** and panicked, the entire defence being one `r.halted`
boolean set two phases earlier (now `ErrVenueClientMissing`, with a test);
`Failure.Subject`'s documented invariant was broken by `Check` itself.

**WHAT REMAINS ON #198 — item 11 only**, which is task #199's operator ruling
(F9 + F14 + F18) and nothing downstream is blocked by it. F20 narrowed F14: a
spot-only Binance deployment is no longer silently unpassable but a **named
startup refusal**, so what is left open there is whether requiring the futures
family is the right answer, not whether the failure is diagnosable.

**Everything below the venue codecs is still fixture-tested and has never met a
real account** (§73f). Unchanged: three standing operator decisions, credentials
deferred, capture **PID 8495**, never rebuild over `bin/arblab`, `make ci` runs
`-race` AND the ka staleness gate (needs node), cap `TLC_WORKERS` at 4,
docs/i18n parity is `bun test`.

---

## Status (2026-08-06, session 113 — HISTORICAL): **#198 ITEM 9 LANDS — DOCS GO TO v0.9.51. §4.1 GAINS EIGHT NORMATIVE PARAGRAPHS, §6.3/§5.2/§7.5/§12 ITEM 4 THEIR ANNOTATIONS, PROFILES §2 A ⚠ BLOCK OF THREE MARKED UNKNOWNS, ADR §3 A SEVENTEEN-ROW DATED BLOCK. THE READING PASS FOUND ONE THING (F18). OWED GREP ZERO, `make ci` GREEN BOTH MODULES.**

**Read `docs/superpowers/specs/2026-08-06-m4-198-account-posture.md` §73m
first** — it lists every edit taken, carries F18, and records the one §73f claim
that was re-checked before landing.

**THE SESSION'S FINDING — F18, and only a reading pass could produce it.**
`posture_binance.go` reads `isAutoAddMargin` from **`GET /fapi/v3/positionRisk`**.
Profiles §9's own dated, verbatim V3 transcription lists **twenty** row fields and
**neither `isAutoAddMargin` nor `autoAddMargin` is among them**; Profiles §2 cites
the **V2** page for the setting, with an unverified *"(or v3)"*. Neither document
is wrong alone — §9 was written for §7.5's net-delta join, §2's row for §4.1's
posture, and nobody had read one against the other. Underneath is a mechanism
worth carrying: §9 argues the version choice as *"a safety decision, not a
preference"* — **for the question §7.5 asks**, where a missing row must mean flat.
§4.1 asks the opposite question and **inherited** the choice, where V3's silence
is precisely the ambiguity. *A narrowing carries an argument whose scope the next
consumer does not re-derive.*

**IT IS MARKED, NOT GUESSED, AND NOT FIXED.** Both readings leave the gate
**refusing** rather than passing, which is exactly why no fixture could see it and
why nothing is urgent. Settling it needs one read of the two endpoint pages
(#180's standing SPA+WAF blocker) or one credentialed call (#171). Profiles §2 and
§9 carry the ⚠; **#199 gains it as a FOURTH candidate** — read the per-symbol
settings from the version that enumerates every symbol — which is the only
candidate needing neither an operator bootstrap per symbol nor reliance on a
recorded default, and is not free (V2's verified weight 10 against V3's unsettled
charged 20, plus two versions of one endpoint to maintain).

**§5.2 WAS NOT ON ITEM 9'S OWN LIST AND NEEDED AN EDIT.** The `leverage` pin and
`portfolio_margin_attestation` landed in code in session 111 with no schema entry
in the spec; item 9's list was drafted in session 109, before either existed. Both
are now in §5.2, each **refused where nothing would read them**.

**THE SHARPENING IN §7.5 IS THE ONE TO READ.** That section said a posture check
that failed *because the venue was unreachable* may pass on a retry — correct
about the half it names, silent about the other. A posture **read and found
wrong** now **LATCHES** in the text as well as in the code: §4.1 forbids this
engine repairing a posture, so no retry can lift it, and a re-reportable refusal
would be lifted by re-reading an account nobody changed.

**STILL NOT DONE — items 10 and 11.** The final fresh `code_diff` review; and
F9 + F14 + F18's operator ruling (task #199), which nothing downstream is blocked
by.

**Everything below the venue codecs is still fixture-tested and has never met a
real account** (§73f). Unchanged: three standing operator decisions, credentials
deferred, capture **PID 8495**, never rebuild over `bin/arblab`, `make ci` runs
`-race` AND the ka staleness gate (needs node), cap `TLC_WORKERS` at 4,
docs/i18n parity is `bun test`.

---

## Status (2026-08-06, session 112 — HISTORICAL): **#198 ITEM 8 LANDS — the collapse checklist run at the layers it was never applied to. THREE FINDINGS (F15–F17), FOUR NEW FIXTURES, SIX MUTANTS ALL CAUGHT, `make ci` GREEN UNCACHED BOTH MODULES, OWED GREP ZERO. Docs still v0.9.50 — item 9's spec edits are next.**

**Read `docs/superpowers/specs/2026-08-06-m4-198-account-posture.md` §73l
first**, and note §73g grew two items (8 and 9) that are general rather than
about this slice.

**THE SESSION'S FINDING — a checklist runs at ONE LAYER unless it is re-run.**
§73g's seven items were drafted against `risk/domain` and sessions 110–111
honoured all seven, there. Item 8's remaining work was therefore not to write
them but to run the same checklist over what they never saw — and **every
end-to-end posture boot fixture in the repo carried an EMPTY roster** (F15).
`RosterCoverageNoDerivatives` had every integration witness and
`RosterCoverageDerivatives`, the ordinary production shape, had none, so §4.1's
whole per-symbol half was reachable at boot and asserted nowhere. The reason is
worth keeping: `spotOnly` was built for §70g, F11 then made its empty roster
**legal**, and a legal degenerate shape is the one nobody notices is degenerate.

**IT IS COVERAGE OF A COMPOSITION, NOT AN UNGUARDED DEFECT — do not re-read it
as the second thing.** All six probe mutations are caught by their own layer's
unit suite. What had no witness was the chain: an operator's `leverage` pin →
`PostureRequirement` → the guard → the ACL → the venue codec → a rule comparison
→ the gate outcome. `Evaluated == 9` is the first assertion anywhere measuring
the composed fold's arity end to end.

**F9 NOW HAS A FIXTURE INSTEAD OF A PARAGRAPH.**
`TestBootOnAFlatAccountRefusesEveryConfiguredSymbol` executes §73j F9: two
configured derivatives, `symbolConfig` answering for both, `positionRisk` empty —
exactly two failures, one per symbol, `SettingAutoAddMargin`/`ReasonUnobserved`.
It asserts the **strict** reading with a comment saying so, because a fixture
must not silently answer a question left to an operator. **When #199 rules, that
test changes with it, and its failure is the reminder that it must.**

**F16 IS THE ONE REAL GAP.** The attestation loader's four rungs each carry a
stated argument for where they sit, and **nothing checked the order**: swap rungs
1↔2 or 3↔4 and all eighteen ladder cases still pass, because each breaks only one
member of the pair. Demonstrated by hand-applying both reorderings — each fails
exactly one test, the new two-violation one. The general lesson is now §73g item
9: wherever one-violation-per-fixture is the discipline, the ladder's ORDER
becomes a separate claim needing its own two-violation fixture.

**STILL NOT DONE — items 9, 10, 11.** Spec edits to v0.9.51; the final fresh
`code_diff` review; and F9 + F14's operator ruling (task #199), which nothing
downstream is blocked by.

**Everything below the venue codecs is still fixture-tested and has never met a
real account** (§73f). Unchanged: three standing operator decisions, credentials
deferred, capture **PID 8495**, never rebuild over `bin/arblab`, `make ci` runs
`-race` AND the ka staleness gate (needs node), cap `TLC_WORKERS` at 4,
docs/i18n parity is `bun test`.

---

## Status (2026-08-06, session 111 — HISTORICAL): **`GateAccountPosture` HAS A PRODUCER. #198 items 5, 6, 7 and 7a LAND — the attestation document, the leverage pin, the boot wiring and the endpoint roster. 15 MUTANTS, TWO REAL GAPS FOUND AND FIXED, `make ci` GREEN UNCACHED BOTH MODULES, OWED GREP ZERO.**

**Read `docs/superpowers/specs/2026-08-06-m4-198-account-posture.md` §73k
first** — it carries this session's five findings (F10–F14) and the mutant table.

**WHAT LANDED.** `internal/risk/infrastructure/attestation` (the Portfolio-Margin
document and its verifying loader, no writer); the `leverage` pin on
`InstrumentConfig` and the `portfolio_margin_attestation` two-act pin in §5.2's
schema; `Config.PostureRequirement` and the REST-dialect → account-model mapping;
a `posture` phase in `app/reconcile` between oms and reconcile; and
`GateAccountPosture` **moved across `ExitClass`** into the produced arm, which is
what that arm's comment has instructed since slice G2. The renderer prints every
failed check by name.

**THE SESSION'S FINDING — a gate's REFUSING paths were the untested ones.** Two
mutants survived: a posture read and found WRONG reporting as passed, and a
posture that could not be read at all reporting as passed. Every boot fixture in
the repo served a clean posture, so both refusing paths were reachable and
unasserted. Both are now pinned, and so is the DISTINCTION between them:
read-and-wrong is `OutcomeNeedsOperator` and latches (§73c-3 forbids this engine
repairing a posture, so no retry lifts it); could-not-be-read is `OutcomeFailed`,
which a retry may answer. A third survivor (`M33`) is a documented unreachable
interlock, left in place with its reason.

**TWO OPERATOR QUESTIONS ARE NOW OWED, and they are one conversation** (task
#199). §73j **F9**: per-symbol coverage is answered from the configured roster, so
a flat Binance account refuses on `isAutoAddMargin` for every symbol — the gate
that must pass before the first position opens cannot pass until one exists.
§73k **F14**: all four Binance probes are on the FUTURES host and the
`symbolConfig` codec refuses an empty array, so a spot-only Binance deployment
cannot answer §4.1 at all. Nothing downstream is blocked by either.

**THREE THINGS NOT TO RE-DERIVE.** The attestation copies #179's baseline shape
correctly and is **not** equally strong — the baseline's chain mark falsifies it,
and nothing here plays that role (F10; #171's credential is the loosening
trigger). A deployment with **no derivative instrument is valid** — §5.2's venue
block is singular, so that is the spot half of a cross-venue carry — and the
empty roster reaches `risk/domain` as a stated fact (`RosterCoverage`) rather
than as either unsafe default (F11). And this root's import fence forbade
`internal/risk` while `declaredGates()` had listed the gate for five slices; the
exemption is exact-path and **keyed to the gate it was granted for**, so it
lapses when its reason does (F12).

**STILL NOT DONE — items 8, 9, 10.** The §73g checklist as fixtures plus the
remainder of the campaign; spec edits to v0.9.51; the final fresh `code_diff`
review. Plus F9 and F14's ruling (item 11).

**Everything below the venue codecs is still fixture-tested and has never met a
real account** (§73f). Unchanged: three standing operator decisions, credentials
deferred, capture **PID 8495**, never rebuild over `bin/arblab`, `make ci` runs
`-race` AND the ka staleness gate (needs node), cap `TLC_WORKERS` at 4,
docs/i18n parity is `bun test`. Docs stay at **v0.9.50** — no spec edits yet.

---

## Status (2026-08-06, session 110 — HISTORICAL): **`internal/risk` EXISTS — §4.1's RULES, THE GUARD USE CASE AND THE ACL ALL LAND (#198 items 2–4 of 11). 21 MUTANTS, ONE SURVIVOR FOUND AND FIXED, `make ci` GREEN UNCACHED BOTH MODULES, OWED GREP ZERO. ONE OPERATOR RULING IS NOW OWED (§73j F9) AND NOTHING IS BLOCKED BY IT.**

**Read `docs/superpowers/specs/2026-08-06-m4-198-account-posture.md` §73j
first** — it carries this session's findings, the 21-mutant table, and the F9
ruling that is owed.

**WHAT LANDED.** `internal/risk/domain` (the posture vocabulary in risk's own
words, `Requirement` with its roster-and-pins, the two per-dialect rule tables
transcribed from Profiles §2, `Check`, and the `Verdict`),
`internal/risk/application` + `application/ports` (`PostureGuard.Assert` and
`AccountPostureSource`), and `internal/risk/infrastructure/driven` (the ACL over
`engine/venue`'s posture surface). **The `risk` bounded context now exists** —
one of the three `CLAUDE.md`'s layout diagram has claimed all along.

**THE ONE THING THAT NEEDS YOU — §73j F9, and it is not a bug.** F4 ruled that
per-symbol coverage is answered from the configured roster, so a configured
symbol the venue said nothing about REFUSES. The consequence F4 did not state:
Binance's `isAutoAddMargin` comes from `positionRisk`, which enumerates
**positions**, so a **flat account refuses on every configured symbol** — and
§73c-2 puts the same read before every trade, so the gate that must pass before
the first position opens cannot pass until a position exists. Bybit is the same
for all three of its per-symbol settings **if** `position/list` omits
never-traded symbols, which nobody here has established. A gate that cannot pass
is not conservative, it is broken — §5.2 already refuses a config value *"that
makes the permitted region unreachable"*, and this is that shape arriving through
a rule. **Three candidates are written out in §73j F9** (keep strict + a manual
per-symbol bootstrap; per-setting reliance on Profiles §2's recorded "off by
default" for Binance auto-add-margin only; or keep the refusal and classify it as
`OutcomeIncomplete`/`NeedsOperator` at the gate). **The strict reading is what is
implemented** — the safe direction, left in place rather than silently loosened.
Nothing downstream is blocked: (3) would land in item 7.

**Three claims worth not re-deriving.** The rule tables are asserted against a
roster written out **by hand** in `check_internal_test.go`, because a test that
reads the table agrees with whatever the table says. `Verdict.Evaluated` exists
because "passed" and "nothing happened" are the same zero value, and
`Satisfied()` **refuses** a verdict that evaluated nothing — which is what stops
a failed read decaying into a clean posture, and is tested through the guard.
And the ACL's presence mapping carries all three of the venue's silences across
distinctly; collapsing any two is how a field the venue never mentioned becomes a
value the rules conclude from.

**M26 was the campaign's one survivor and its lesson is inverted.**
`Assert`'s scope check is unreachable through `NewPostureGuard` (the constructor
validated the venue), so the mutant that made it return a clean verdict survived.
The branch is not dead — `var g PostureGuard` reaches it, and without it the next
line panics inside §7.5's boot. The fix was a **test for the zero-value guard**,
not a deleted branch.

**STILL NOT DONE — items 5–11.** The Portfolio-Margin attestation document and
its two-act pin; the leverage-pin config field; `app/reconcile` wiring with
`GateAccountPosture` moving out of `ExitClass`'s producerless arm; item 7a (the
two Bybit endpoints in every rate-limit profile); the §73g checklist as fixtures
plus the full campaign; spec edits to v0.9.51; the final review; and F9's ruling.
**Nothing yet calls the guard** — it has no caller, exactly as `GateAccountPosture`
has no producer.

**Everything below the venue codecs is still fixture-tested and has never met a
real account** (§73f). Unchanged: three standing operator decisions, credentials
deferred, capture **PID 8495**, never rebuild over `bin/arblab`, `make ci` runs
`-race` AND the ka staleness gate (needs node), cap `TLC_WORKERS` at 4,
docs/i18n parity is `bun test`. Docs stay at **v0.9.50** — no spec edits yet.

---

## Status (2026-08-06, session 109 — HISTORICAL): **MILESTONE 4 IS OPEN. SLICE A (#198, §4.1's ACCOUNT POSTURE) IS MID-SLICE — THE VENUE CODECS AND THEIR 13 TESTS LAND, `make ci` GREEN UNCACHED BOTH MODULES, OWED GREP ZERO. THREE OPERATOR RULINGS ARE RECORDED AND ARE THE SLICE'S SPINE. NO SPEC EDITS YET (docs stay v0.9.50).**

**Read `docs/superpowers/specs/2026-08-06-m4-198-account-posture.md` (record
§73) before touching any of this.** It carries the six pre-code findings, the
three rulings with their rejected alternatives, and §73i's two
implementation-time findings.

**Why milestone 4 opens on the posture and not on sizing.** §12 item 4 states
the ordering itself — *"the account/margin-mode verification above is what makes
a tier reading mean anything"* — so a live leverage tier read under the wrong
margin or position mode is a number computed against a different contract than
the one that would be traded. #197 reassigned `sizing-by-live-tier` into this
milestone; this slice builds its premise first.

**The seam already existed and that is the point.** `platform/boot` has declared
`GateAccountPosture` since slice G2, `app/reconcile`'s `declaredGates()` lists it
in §7.5's eight-gate roster, and `Report.ExitClass` names it in the arm for
gates *"with no producer anywhere"* — whose comment instructs exactly this
slice: *"a gate added to §7.5's roster has to choose a side here, and choosing
'not ours' by falling through is how a real refusal becomes invisible."* The
slice's structural deliverable is to give that gate its first producer and move
it across that switch. **Not done yet** — the move is work item 7.

**THE THREE OPERATOR RULINGS (session 109), do not silently reopen:**

1. **Portfolio Margin → operator attestation, default refuse.** Profiles §2's
   Binance assert cell lists four checks each naming a `GET` and **"Portfolio-
   Margin OFF" with no read path anywhere in either document** (§73b F1). It is
   the one setting that folds spot into futures collateral — i.e. the single
   account state that falsifies §4.1's whole premise — so it cannot be the check
   that silently passes. No attestation → the gate refuses by name. The
   attestation follows #179's baseline shape (inputs-not-conclusions, two-act
   `{path,sha256}` pin, invalidated by events never time, absent → refuse) and
   **does not become a venue fact**: nothing enters Profiles §2's verified table
   on its strength. *Rejected: arguing the sibling checks imply it — PM is a
   different account TYPE, not a setting inside the classic account.*
2. **Pre-trade → a REST read before EVERY trade.** §6.3's *"fresh within a
   max-age TTL **and** re-verified by the pre-trade REST read"* is read
   conservatively: the TTL is an upper bound on any cached snapshot, never a way
   to skip the read. §6.3's own sentence is the argument — position-mode changes
   emit **no** venue event on either venue, so a flip inside a TTL window is
   exactly what the re-verify exists to catch. Affordable structurally: carry
   opens are hours apart. **Measured cost: Binance 4 calls / 80 weight, Bybit 4
   unweighted requests, per read.**
3. **Reads only — posture is asserted, never repaired.** No code path calls
   Profiles §2's `POST` setters. Beyond the obvious (an account-wide
   irreversible mutation inside an automated boot), the stronger ground: **a
   posture the bot can repair is a posture whose violation stops being
   evidence** — its value is that a wrong margin mode means something happened
   we do not understand. *Rejected for now: an `arblab posture set` command — a
   new credentialed WRITE path that cannot be tested against a venue while
   credentials are deferred.*

**WHAT LANDED (work item 1 of 10).** `internal/engine/venue/posture.go` (the
vocabulary: `PostureProbe`, `PostureSetting`, `PostureScope`, `PosturePresence`,
`PostureObservation`, `AccountPostureReport`, `AccountPostureCodec`, and
`OrderClient.AccountPosture`), `posture_binance.go` (4 probes),
`posture_bybit.go` (4 probes), two rows in `transport.bybitEndpointRegistry`
(`account_info`, `spot_margin_status`), and `posture_test.go` — **13 tests, 4
mutants all caught.**

**The surface REPORTS, it does not JUDGE.** Nothing in `engine/venue` knows that
ISOLATED is required or that `dualSidePosition` must be false. Those rules are
`risk/domain`'s (work items 2–4, the context does not exist yet), stated once
against an operator-declared pin — because a codec that encodes the requirement
supplies both halves of its own check, which is §73b F3's vacuity.

**THE TRAP THIS SLICE WAS BUILT AROUND (§73b F2), and it has a warning label
already in the tree.** `balances_bybit.go:193` reads an **empty**
`borrowAmount`/`spotBorrow` as *"the venue stated no borrow figure"* and lets
the row PIN — correct there, and its own comment nominates this slice as the
owner of the strict reading. Here the same absence must mean **cannot conclude →
refuse**, because the claim being asserted *is* "borrow is off". So
`bybitCoinBorrowed` is **deliberately not called** and the wallet is read a
second time; reusing it would import the permissive arm into the one gate whose
purpose is refusing borrow. Two folds over one record, opposite safe directions.

**Every boolean wire field is a POINTER, and that is load-bearing rather than
fastidious.** `false` is the PASSING value for `multiAssetsMargin`,
`dualSidePosition`, `isAutoAddMargin` and spot hedging, so a plain `bool`
decodes a renamed field, a wrong endpoint or a truncated body as *the posture is
correct*. Mutant M2 (a missing bool reads as `false`) is killed by two tests.

**Three more pre-code findings, all encoded:** the leverage pin is
operator-declared config and an **absent** pin refuses — deriving it from the
venue at first boot makes the check compare the venue against itself (§73b F3);
Bybit's auto-add-margin check is **roster-scoped**, since a position-driven one
checks zero symbols on a flat account, which is every cold boot (§73b F4); and
`unifiedMarginStatus` is a **whitelist** {3,4,5,6}, so the unlisted `2` and any
value the venue adds later refuse (§73b F5).

**§73i's two implementation-time findings.** `CheckFold` validates profile ⊆
registry and **never the reverse**, so the two new endpoints passed every check
while no profile declares them — not silent at runtime (`Governor.Observe`
returns `ErrLimitUnknown`, and its comment names this exact case), but it makes
**work item 7a** real: declare `account_info` and `spot_margin_status` in every
profile before the first posture read. And `go test -run Posture` selects only
**4 of the 13** new tests, because the rest are named for the venue — the
campaign at slice close runs against the package, never a name pattern.

**WHAT IS NOT DONE — items 2–10 of §73h.** The `internal/risk` context
(domain/application/ports/driven) does not exist; the Portfolio-Margin
attestation document and its config pin do not exist; the leverage pin config
field does not exist; `GateAccountPosture` still has no producer and still sits
in `ExitClass`'s producerless arm; item 7a; the ≥15-mutant campaign; the spec
edits to v0.9.51; the final `code_diff` review.

**What this slice can never establish, stated rather than implied (§73f).**
Credentials are deferred by operator ruling, so every codec here is
fixture-tested and has **never met a real account**. A wire-shape error — a
field name, a nesting level, an enum spelling — is invisible to every test in
this slice and would surface as a refusal on the first credentialed boot. That
direction is a **halt**, which is the acceptable one, but it is not "verified".
Profiles §2 supplies the paths and the names `marginType`, `isAutoAddMargin`,
`dualSidePosition`, `marginMode`, `unifiedMarginStatus`, `spotHedgingStatus`,
`borrowAmount`, `spotBorrow`, `positionIdx`; **everything else on the wire
structs is this codec's best reading and is marked as such at both call sites.**

**Open work, unchanged otherwise:** #177, #184, #159, #152 and #158
(clock-bound), #180 (three unread weight candidates, charged 20 = the max, safe
to leave open). The three standing operator decisions are unchanged and
credentials stay deferred. Capture **PID 8495**, never rebuild over
`bin/arblab`, `make ci` runs `-race` AND the ka staleness gate (needs node), cap
`TLC_WORKERS` at 4, docs/i18n parity is `bun test`.

---

## Status (2026-08-05, session 108 — HISTORICAL): **MILESTONE 3 IS RECORDED COMPLETE — §12's ROSTER IS HONEST AGAIN, AND THE CLOSURE PASS FOUND ONE CLAUSE LISTED WITH NO CODE ANYWHERE: `sizing-by-live-tier` IS REASSIGNED TO MILESTONE 4. DOCS AT v0.9.50, `make ci` GREEN UNCACHED BOTH MODULES, OWED GREP ZERO. #180 GAINED A THIRD UNREAD CANDIDATE AND STAYS OPEN.**

**Read `docs/superpowers/specs/2026-08-05-m3-197-milestone-closure-pass.md`
(record §72) before reopening any of this.** ADR §3 has the dated block; §12's
item 3 carries the consolidated annotation and item 4 the reassigned clause.

**Why the pass ran.** §12's item 3 carried a landing annotation for every slice
through **E (v0.9.35)** and then stopped, with the doc set at **v0.9.49** — five
slices and a twenty-eight-task tail unrecorded on the roster, and no sentence
anywhere stating the milestone done. That is **§47e finding 1's shape**
(milestone 1 recorded complete with half of `engine/venue` undelivered), and the
`SPEC EDIT OWED` grep is structurally blind to it.

**The first question was whether the roster had been deliberately DEMOTED**, in
which case the gap is correct and the fix is one sentence. **It had not:** the
annotation is an explicit close item in the slice-**C2a** and slice-**C2b**
records, appears in **no** record from slice F1 onward, and was retired by no
ruling. It lapsed between two same-day sessions. `adr-history` had already named
that mechanism on the capture release — a state that *"expired silently"*, where
*"the close's reading pass is what re-derived it; the grep, by construction,
could not."*

**THE FINDING — eight of item 3's nine clauses check out; the ninth has no code
at all.** `MMR`, `maintenance margin`, `leverageBracket`, `risk limit`,
`marginTier`: **zero** non-test hits across `internal/`. No `internal/risk/`. No
sizing function of any kind. **And this document's own traceability table had
already filed it elsewhere** — `| C4 tiered MMR | Profiles §1, core §7.1 |`,
where §7.1 is *"Sizing for survival & the margin top-up protocol"*, a section
§12 gives to **milestone 4** in terms. Three further grounds: `planning.go`'s
`Quantity` is a **designed** input that `Plan` verifies rather than trusts (the
single-writer OMS is the wrong home for a producer); the live-tier MMR needs a
signed `GET /fapi/v1/leverageBracket` read that only milestone 4's
account/margin-mode verification makes meaningful; and `tutorial.md`'s
acceptance checklist had **already paired** *"live-tier MMR sizing + margin
top-up protocol"* as one clause. **User ruled: misfiled — move it.** Item 3's
text is annotated, not rewritten (milestone 1's scope-correction precedent), and
item 4 gains it beside the §7.1 top-up protocol.

**What the finding is really about.** The venue fact is **recorded and read by
nobody** (profiles: *"fetch `GET /fapi/v1/leverageBracket` at runtime — table
not otherwise readable `[→ C4]`"*, signed `USER_DATA`, IP weight 1), and **eight
comments across seven packages** name liq-distance as a quantity computed
*elsewhere* — each locally true, collectively a chain of deferrals with **no
terminal**. No gate can see that: every comment is correct, and the grep polices
only *marked* claims. **The failure direction is a halt, not a wrong number** —
with no producer, `Plan` cannot be handed a size at all, so nothing mis-sizes,
which is exactly how it survived five slices, the tail and a formal-model
campaign. **Nothing in this system sizes a trade today.** That is now stated in
§12 item 4 rather than implied nowhere.

**#180 gained a THIRD unread candidate and did not close.** An operator-supplied
second AI summarizer reported *"1 for a single symbol; 10 when the symbol
parameter is omitted"*, against the first summary's *"5 with `symbol`, 20
without"* and §9's recorded flat **5**. **Two summarizers, a factor of two
apart** — and the second carries all three recorded tells: every answer from
**one aggregated catalog page**, V2 and V3 consequently reported as carrying the
**byte-identical** weight line (which *weakens* the earlier "V2 now says 10"
report rather than corroborating it), and changelog citations that are
`google.com/search?q=…` **redirects** rather than the changelog. **What the
disagreement DOES establish is the SHAPE:** both independently report a
*conditional* weight and neither reports the flat row, and the shape is the
load-bearing half because §7.5's enumeration always sends no `symbol`. **The
charged 20 is the MAXIMUM over all three candidates** — correct-or-pessimistic
under each, and **no further candidate below 20 can move the code**, which is
the property that makes the task safe to leave open. Profiles §9 and
`positions_binance.go`'s file comment both record all three readings.

**Only a direct read closes #180:** the V3 endpoint page's own Weight line in a
real browser (**not** the aggregated `/api/rest-api/trade` catalog, **not** a
summarizer), or the venue's `X-MBX-USED-WEIGHT-1M` response header on testnet —
the venue's actual accounting rather than its prose — which needs #171's
credential. `developers.binance.com` still answers every headless fetcher with an
AWS WAF JS challenge, and `list_connected_browsers` returned empty again this
session.

**Open work, unchanged otherwise:** #177 (FM-9's consumer — now unblocked in
principle by nothing, still milestone 4's generation ladder); #184 (awaits its
first real caller); #159 (at the next capture restart); #152 and #158
(clock-bound, ~2026-09-30); the three standing operator decisions (the $20/$55
live test, #171's Bybit testnet credential, §38f's off-host archive target).
**Milestone 4 is ADMITTED but NOT SCHEDULED** — §47c's boundary is untouched by
this session, and it is the part a later session silently widens. Capture **PID
8495**, never rebuild over `bin/arblab`, `make ci` runs `-race` AND the ka
staleness gate (needs node), cap `TLC_WORKERS` at 4, docs/i18n parity is
`bun test`.

---

## Status (2026-08-05, session 107 — HISTORICAL): **#153 IS RULED AND CLOSED — `restart_boundary` SPLITS BY EVIDENCE, THE NEW `unsequenced_boundary` IS THE ONE VERDICT-NEUTRAL KIND, 22 MUTANTS WITH TWO SURVIVORS FOUND AND FIXED, `make ci` GREEN UNCACHED BOTH MODULES. #180 IS IN PROGRESS AND WAITING ON ONE PASTE FROM THE OPERATOR.**

**#153, the ruling (record §71).** `restart_boundary` was one name over a
PRODUCT of two properties — the `prev_last_seq == 0` claim, and whether a
sequence existed to lose. On a REST-overlay source the second is permanently
false (slice E: a Rest record is not a Data record, so `lastSeq` never leaves
zero), so it fired once per hour per source: **24 of 50 findings on a clean
day**. The user chose "split the kind by evidence" from three options, and the
table is now: predecessor ended `> 0` → `restart_boundary`; predecessor ended
`0` **and** both Open records name the same `(host, pid)` → the new
`unsequenced_boundary`; identity differs or the walk was truncated →
`restart_boundary`. **Nothing is silenced** — every seam is still a coverage
finding on the report; one class got a name that says what is known.

**What the task's framing did NOT say, and it is the bigger half.**
`assignVerdicts` flags a window on ANY finding but `window_absent`, so every
REST-overlay hour has been reporting **`VerdictGapped`** — §32k's answer to
*what would a replay of this hour get* — for an hour in which a living writer
rotated a file and lost nothing. `FindingKind.AffectsVerdict()` is now an
exhaustive switch with no default (every existing kind returns today's answer;
only the new kind is neutral), so a kind added later makes the verdict decision
where the compiler and `exhaustiveswitch` both check it.

**Side finding, fixed here (§71e):** `finding_test.go`'s roster — the map whose
own doc says a kind with no producer is unreachable — had drifted by **five**
members (every manifest kind). A test that ranges over a map cannot see the
member that is not in it. It is now a `producerFor` SWITCH, gate-enforced;
deleting one case and running `make lint` fails by name, demonstrated.

**The campaign found two fixture defects, both worth remembering.** M9: the
adopted-claim fixture SEALED its silent hour, and a sealed file states its
sequence twice (Open claim + Close payload), so deleting one source changed
nothing — the fixture absorbed its own variation. M13: the stale-identity
fixture asked verify for one window, and verify walks exactly ONE file earlier
than the range, so the three-file setup never had its first file read. Both were
aimed correctly and unobservable; neither was findable by reading the test.

**#180 turned up a live undercharge, and the conservative arm is LANDED while
the figure stays unverified.** A browser-side AI summary (operator-run,
2026-08-05) reported `/fapi/v3/positionRisk` at **"5 with `symbol`, 20
without"**. §7.5's enumeration deliberately sends **no** symbol, so under that
reading every position read costs 20 and the code was charging **5** — a
four-times undercount of the IP budget, in the direction whose end is the ban
§7.6 exists to avoid. **The reading is NOT verified and was not recorded as a
venue fact** (a summarizer is the exact channel that misreported
`/api/v3/account` as 10 against a raw-docs 20 two sessions ago; profiles §9 now
records the unsettled STATE, both candidate readings and their provenance).
What landed is the both-readings response: `binancePositionsRoute` charges
**20**, correct under the conditional reading and pessimistic by 15 once per
reconcile pass under the flat one, with the argument at the decision site, a
test pinning the literal, and a killed mutant (M23). **A direct read of the
published line is a LOOSENING trigger, not a prerequisite.**

Three further tells about that summary, all recorded: it answered the
"does the endpoint page still carry its own sentence" question with the
**changelog's** 2024-07-24 sentence and the changelog URL; it cited ONE
aggregated catalog page for both endpoints, where "which endpoint owns this
Weight line" is a judgement rather than a reading, and V2/V3 are adjacent
there; and it reported V2's page as now saying **10**, i.e. that the two-year
page-vs-changelog discrepancy §9 recorded may since have been fixed at source —
also unread.

**#180 is still OPEN, and what remains is one look at one line.** `developers.binance.com` now
serves an **AWS WAF JavaScript challenge** (HTTP 202, empty body) to curl and to
any headless fetcher, and the recorded §9 URLs 302 to a restructured catalog path
(`/en/docs/catalog/core-trading-derivatives-trading-usd-s-m-futures/api/rest-api/trade#position-information-v3`).
The Claude Chrome extension was not connected this session. The operator offered
to open the pages; **what is needed, verbatim, not summarized** (a summarized
fetch is what misread `/api/v3/account` as weight 10 two sessions ago): (1) the
V3 page's **Weight** line and whether it is CONDITIONAL on `symbol` — this
deployment deliberately sends none, and `positions_binance.go:85` charges
`ratelimit.Cost{Weight: 5}` for the no-symbol enumeration; (2) the V2 page's
Weight line as printed today (we recorded 5; the 2024-08-07 changelog entry says
`5->10`, so the question is whether the page is still stale); (3) any changelog
entry **after 2024-08-07** naming `positionRisk`. Undercharging a weight is the
direction that ends in the IP ban §7.6 exists to avoid.

**Open work otherwise unchanged:** the standing operator decisions ($20/$55 live
test + #171's Bybit testnet credential + §38f's off-host archive target); #177
(FM-9's consumer, milestone 4); #184 (Dispatch filter conformance, awaits its
first real caller); #159 (raise the nine Binance depth20 `first_data_timeout`
10s → 30s at the next capture restart); #152 (`arblab crossvenue gate`, BCHUSDT
binds ~2026-09-30). Capture **PID 8495**, never rebuild over `bin/arblab`,
`make ci` runs `-race` AND the ka staleness gate (needs node), cap
`TLC_WORKERS` at 4, docs/i18n parity is `bun test`.

---

## Status (2026-08-05, session 106 — HISTORICAL): **#179 IS CLOSED — DOCS AT v0.9.49, THE ONE `[SPEC EDIT OWED]` MARKER DISCHARGED, AND THE FINAL `code_diff` REVIEW RAN TWO ROUNDS: NO-SHIP WITH FOUR MAJORS (ALL VALIDATED AND FOLDED, RECORD §70i, EIGHT-MUTANT ADDENDUM — CAMPAIGN TOTAL 28 ZERO SURVIVED), THEN SHIP. `make ci` GREEN UNCACHED ON BOTH MODULES. THE SPOT LEG'S EXPOSURE BASELINE IS DONE END TO END.**

**(e) The spec edits, v0.9.48 → v0.9.49.** §7.5's spot paragraph is rewritten
into two: the baseline construct DEFINED (`baseline := B − ours` per
`(venue, account, base asset)`, operator-run `arblab baseline record`, two-act
`{path, sha256}` pin, inputs-not-conclusions document chain-marked to the
journal, invalidated by events never time, stable-across-a-second-look
latching), and the must-not-hold exemption RETIRED with its expired premise
stated in place — `UnassertedBaselineUnrecorded` HOLDS the posture, coverage
is roster-driven, and `spot_baseline`'s optionality is load-bearing. §5.2's
OMS block gains both fields (settle required-when-spot and refused-when-
spotless; the pin refused-when-spotless too — a reading-pass catch, the code
asserted it and the first spec draft missed it). adr-history gains the #179
block with an eleven-row findings table; venue-profiles gains the v0.9.49
set-version entry over its already-landed balance-surfaces block; the design
record's WIP banner is a closure record. `grep -rn "SPEC EDIT OWED"
internal/ tools/` returns zero.

**(f) The final review, and it earned its keep: four majors, every one
real.** (1) **Binance absence read as zero** — §70d's own sentence said a
Binance absence "carries no conclusion" while the port declared
absent-means-zero universally; now the enumeration kind is a CARRIER
(`SpotAbsenceSemantics`/`ports.AbsenceSemantics`, stamped by each codec,
zero value refused at the client, the ACL, the join and the record command):
Bybit silence stays the venue's zero, a Binance absence is `SpotUnconcluded`
at assert, `ErrRecordBalanceAbsent` at record (both bracket reads share the
guarded path), `ErrStoredAbsenceUninterpretable` at load. (2) **The second
look re-asserted over a mid-bracket disappearance** — `newAbsence` now
downgrades to `OutcomeIncomplete` for NEWLY absent orders only; the
both-looks control (a resolved `AbsentAfterFinality` order beside a stable
delta that must still latch) pins the other direction, and the paired
mutants (check dropped / widened to any absence) are each killed by their
own fixture. (3) **The loader trusted stored side/base/fees against §70b's
claim that only the cumulative was attested** — the prefix visitor now
collects sides and a contradicting row refuses (`ErrOrderRowSideDisagrees`);
folding it instantly caught baseline_test's sell row journaled as a BUY.
§70b corrected in place; the bijection demand rejected with the recorded
argument. (4) **`oms/infrastructure/baseline` violated §3.6 rule 3
unmarked** — the position was deliberate in §70b but the permitted-children
rule was never edited; rule 3 now carries the documented exception with the
rule-6 interaction argument, scoped to ONE named package. Round 2 verified
all four folds end-to-end: **SHIP**.

**Open work, none of it #179's:** the standing operator decisions ($20/$55
live test + #171's Bybit testnet credential + §38f's off-host archive
target); #177 (FM-9's consumer in SM §7's generation ladder); #180 (browser-
confirm positionRisk weights); #184 (Dispatch filter conformance, awaits its
first real caller); #152/#153/#159 (capture/gate housekeeping). Capture
**PID 8495**, never rebuild over `bin/arblab`, `make ci` runs `-race` AND
the ka staleness gate (needs node), cap `TLC_WORKERS` at 4, docs/i18n parity
is `bun test`.

---

## Status (2026-08-05, session 105 — HISTORICAL): **#179 MID-SLICE, THIRD SAFE POINT — ITEMS (a) THROUGH (d) OF SESSION 104'S LIST ARE DONE: THE TEN-CASE `record_test.go` TRANSCRIBED AND GREEN, THE CONFIG ROWS + RENDERER GOLDENS LANDED, ALL SIX `awaits #179` FIXTURES UNSKIPPED, THE §70h ITEM 1–24 AUDIT COMPLETE (IT FOUND AND FILLED TWO REAL GAPS), AND THE MUTATION CAMPAIGN RUN — 20 MUTANTS, ZERO SURVIVED (results table at §70h's tail). `make ci` GREEN UNCACHED. WHAT REMAINS IS ITEMS (e) SPEC EDITS → v0.9.49 (DISCHARGING THE ONE `[SPEC EDIT OWED]` AT join.go:~502) AND (f) THE FINAL FRESH `code_diff` REVIEW, THEN THE SLICE CLOSES.**

**What session 105 did, in order.**

**(a) `internal/oms/record_test.go`** — the ten-case design transcribed
verbatim: the happy path (Planned-only beside terminal filled RECORDS —
§70h.12; the wait exactly once with SpotSettle; order reads a known exact
count of 3, every urgency Routine on BOTH surfaces — §70h.13; baseline 0.5005
via the returned loader recomputation; document rows exactly the filled order,
words verbatim, ChainSeq exactly 7; SHA256 re-derived from the on-disk bytes),
the four gauntlet refusals each on its own fixture (§70h.8), the two
settle-window refusals as SEPARATE fixtures (§70h.18 — the balance-moved one
moves the SPLIT while keeping the total, pinning verbatim-words equality), the
unproved absence (recent anchor → horizon gate), the unwired bracket (both
halves), and the absorbed-delta print {0.3 → 0.5005, delta 0.2005}. Plus
`TestBootBalanceReadRidesSafety` (the boot rung beside the record rung — one
fake, two call sites, two logged priorities) and
`driven/balances_test.go` (constructor refusals incl. the zero priority,
which no composed fixture could reach).

**(b) Config + renderer.** Seven `TestConfigRefusals` rows (settle
required-when-spot / refused-when-spotless, pin shape ×4, pin-on-spotless with
a WELL-FORMED pin so the row cannot pass on the shallower shape check) + the
two acceptance positive controls; `TestReconcileReportGoldenSpotSections`
pins the complete report with every assertion section at once — spot
unexplained BESIDE the derivative delta (§70h.11), both unpinned reasons'
lines, and the combined latching-plus-unpinned output with verdict
ExitOperator (§70h.21/14).

**(c) The six unskips.** The recipe everywhere: a seq-0 baseline document
(`pinnedCleanBaseline` in reconcile's and exercise's tests, `chaosBaseline` in
chaos_test) whose stored words the fake venue's `GET /api/v3/account` answer
repeats, pinned by {path, sha256} in the FIXTURE CONFIG (never the Deps data
seam), fingerprint computed through `transport.NewCredential(...).Fingerprint()`
— the production derivation, not a literal. The reconcile clean boot now
serves exactly ONE wallet read and asserts the count; chaos config gained
`spot_settle` (it would no longer validate without it) and both chaos runs
boot through the pin (seq-0 verifies vacuously against both journals).
`TestBootWithoutABaselineHoldsAtUnrecorded` is the virgin path stated
app-level (§70h.15): no pin → ExitOperator, GateNetDeltaAsserted at
NeedsOperator naming UnassertedBaselineUnrecorded, zero venue calls.

**(d) The audit found the safe-point summaries had OVERSTATED coverage** —
"§70h 5–6 partly in join_test" was false: `Joiner.assertSpot` and the
Reconciler's second look had NO tests at all (every joiner fixture passed
`SpotLeg{}`). Two new files close it: `join_spot_test.go` (the cell
arithmetic with baseline≠0 AND ours≠0, all three fee shapes incl. a perp
order whose BTC fee must NOT count, both delta signs, absence-as-zero against
a two-row report, the shared-base cell with one read and ONE delta,
unpinned-is-unconcluded-not-a-pass, the duplicate-row refusal, and the
validateSpotLeg table incl. §70h.24's quote-overlap deny with its
same-roster-no-baseline positive control) and `reconcile_spot_test.go` (the
second look: stable delta → NeedsOperator latched with the wait recorded;
delta that MOVES mid-bracket → Incomplete, no latch — the fixtures mutate the
venue INSIDE the injected Wait; delta fully explained → Passed; unwired
bracket meeting a delta → Failed naming ErrSecondLookUnwired; unpinned alone →
Incomplete with the bracket never running, and unpinned-beside-stable-delta →
still latches — §70h.14/23). Then the campaign: **20 mutants, zero
SURVIVED**, results table + the four re-aimed NOBUILDs recorded at §70h's
tail in the design record.

**NOT YET DONE (the next session's list, in order):**

**(e) Item 7 — spec edits → v0.9.49:** §7.5's spot paragraph rewrite
discharges the ONE `[SPEC EDIT OWED]` marker at
`internal/oms/application/join.go` (~line 502, on `UnassertedBaselineUnrecorded`
— grep `SPEC EDIT OWED`); §5.2 gains `spot_baseline` {path, sha256} optional +
`spot_settle` required-when-spot + the CLI (`arblab baseline record -config
-out`); venue-profiles already carries the dated 2026-08-05 balance-surfaces
block (verify, don't re-add); adr-history dated block; §70's WIP banner →
closure record. Docs to v0.9.48 → v0.9.49. Remember the reading pass beside
the OWED grep: the audit's own finding this session was a summary claiming
coverage that did not exist.

**(f) Item 8 — the final fresh `code_diff` review** over the whole #179 diff
(three safe points: 6c3f719's predecessor..HEAD). Round-4 design review stays
optional. Then commit as the slice close and #179 is done.

**Standing user decisions, unchanged:** the $20/$55 live-test decision; #171;
§38f. Capture **PID 8495**, never rebuild over `bin/arblab`, `make ci` runs
`-race` AND the ka staleness gate (needs node), cap `TLC_WORKERS` at 4,
docs/i18n parity is `bun test`.

---

## Status (2026-08-05, session 104 — HISTORICAL): **#179 MID-SLICE, SECOND SAFE POINT — ITEMS 1–4 OF SESSION 103'S EIGHT ARE IMPLEMENTED AND `make ci` IS GREEN UNCACHED ON BOTH MODULES: THE ENGINE BALANCE CODECS + CLIENT READ, THE BASELINE DOCUMENT PACKAGE WITH ITS VERIFYING LOADER (TESTED AGAINST A REAL JOURNAL), THE SIX-STEP RECORD PROTOCOL + `arblab baseline record`, AND THE CONFIG/ROOT/RENDERER THREADING. THE RECORD PROTOCOL IS IMPLEMENTED BUT NOT YET TESTED — ITS FIXTURE DESIGN IS WRITTEN OUT BELOW SO NOTHING RE-DERIVES. THE SIX SKIPS AND THE ONE `[SPEC EDIT OWED]` MARKER (join.go:~502) STILL STAND BY DESIGN.**

**Read record §70 first; then this block's "NOT YET DONE" is the whole work
list.** What landed this session, by §70 section:

**(1) Engine codecs + client (§70d/§70f).** `SpotBalanceCodec` embedded in
`OrderCodec` (both venues or no venue); `balances_binance.go` — weight **20**
asserted in-test, `omitZeroBalances` deliberately UNSET (params length 0 is
asserted), `BinanceSpotTotal` exported (free+locked, the ONE shared meaning),
empty-balances body REFUSED (with omitZeroBalances false every asset comes
back, so a rowless body is wrong-shaped and §5.3 forbids reading it as a flat
wallet); `balances_bybit.go` — `accountType=UNIFIED`, `coin` never passed
(asserted at the WIRE in a client test, not only at encode), `BybitSpotTotal`
exported (both-readings: pinned only at locked==0), borrow arms
`borrowAmount`/`spotBorrow` each unpinning on its OWN fixture, and an
**empty-string borrow field is "no evidence", not a refusal** — the unpin is
evidence-bound (a STATED nonzero inflates walletBalance); a non-empty
unparseable figure is a decode error; #171 measures reality (decision recorded
in `bybitCoinBorrowed`'s comment). `OrderClient.SpotBalances(ctx, urgency)`
routes via the SPOT transport (two-host test: fapi sees nothing), stamps
`ObservedAt` from the new `transport.SignedClient.ServerNow()` (same clock the
request was signed with — one authority). Roster row
`/v5/account/wallet-balance` → `wallet_balance` landed; the transport fixtures
are ROSTER-DRIVEN (they iterate `Endpoints()`), so the §66 complete-roster
ripple auto-propagated and no hand-built Bybit profile exists outside them —
the hand-enumerated path→name test table got its row explicitly.

**(2) The document package (§70b) — `internal/oms/infrastructure/baseline`**
(under oms BY POSITION so it may hold the journal adapter; roots may import
it, singlewriter still bites on ports/driven). `Document` stores INPUTS:
per-asset verbatim words OR an explicit `absent` marker (the venue's silence
stated out loud — absent coin = zero), per-order rows {id, side, base,
cumulative, fees by asset}. `Write` is O_EXCL with partial-write cleanup;
`Encode` is exported (the digest the operator pins is of the exact bytes) and
pinned by a BYTE GOLDEN — the encoding is deployment contract, a marshalling
drift would refuse every pinned document in the field. `Load(pin, journal,
bindings)` runs the ladder in dependency order: digest → shape → chain mark
(`HashAt`; seq-0 vacuous BY THE SEQ; `driven.ErrJournalMarkBeyondHead` passes
through NAMED — truncation is a different operator conversation than
`ErrChainMarkMismatch`) → credential fingerprint → venue/account →
rows-vs-prefix (a `prefixOrders` visitor rides the chain-verifying `Replay`;
mark 0 collects nothing, so order rows on an empty-journal document refuse) →
RECOMPUTE (B via the shared per-venue derivation selected by
`baseline.Dialect` — ids.Venue is an open key, the dialect is the
discriminator; a stored Bybit row with locked != 0 is
`ErrStoredRowUnpinnable`, which is what makes the walletBalance/locked
VALUE-swap loud; ours by §70c's formula; baseline = B − ours last).
`baseline_test.go` runs against a REAL SQLiteJournal: §70h items 3 (B and ours
both nonzero: B=1.0, ours=0.5−0.2−0.0005−0.0001=0.2994, baseline=0.7006, plus
an absent-ETH row giving baseline −1), 1–2 (fee shapes incl. the BNB-shape
cross-pair fee), 7 (three chain-mark cases), 16 (round-trip + word-edit moves
the answer by exactly the edit + byte golden), 19 (fingerprint the ONLY
discriminator), 20 (bytes replaced at path → digest; row past the mark →
prefix — journaled AFTER the mark, so "in the journal" ≠ "in the prefix").

**(3) The record protocol (§70b) — `oms.RecordSpotBaseline` in
`internal/oms/record.go`,** a Module method (Module gained unexported
`orders/reader/balances/venue/account/roster/baselines` fields; the venue
derivation moved above the spot-leg block in New). Steps: replay (Dispatched
IS the MayExistAtVenue set — Planned-only excluded by the same predicate the
join uses); join (Unattributed → its own sentinel, THE non-negotiable;
Incomplete → its own); every absence through §5.3 (`OutcomeUnproved` refuses;
`Journaled && !Applied` refuses as stale); terminal check AFTER resolution
(spot only — a live perp is fine, mid-carry re-record); balance read #1
through the protocol's OWN Routine adapter; `SettleWait(SpotSettle)`; re-read
BOTH surfaces (per-order: present-at-join must still be present with an
unchanged Executed, absent-at-join must still be absent — either flip is
`ErrRecordCumulativeMoved`; balances: verbatim-words equality per coverage
asset, else `ErrRecordBalanceMoved`); chain mark = `journal.Head()` taken at
document build (resolution may have appended state records); write O_EXCL;
**verify-back through the real loader** — the Baselines the operator sees are
the loader's recomputation, not the protocol's arithmetic. Absorbed deltas
against the prior: `RecordBaselineDeps.PriorPin` is loaded BEST-EFFORT
(failure → `RecordedBaseline.PriorError`, printed) because re-recording is the
REMEDY for a prior that no longer verifies — the BOOT stays strict
(`oms.New` errors on an unloadable `Deps.SpotBaselineDocument`).
`app/reconcile.RecordBaseline` wires the same config at
`RecoveryReadPriority: Routine`; `cmd/arblab/baseline.go` is
`arblab baseline record -config -out` (prints path, sha256, baselines,
absorbed, and the "pin it" next step; exit 1 on any gauntlet refusal).

**(4) Config/root/renderer (§70g).** `Config.SpotBaseline *BaselinePinConfig`
({path, sha256} — hex-validated 32 bytes) OPTIONAL; `Config.SpotSettle`
required-iff-spot AND REFUSED on a spotless deployment (a bound nothing waits
is a claim nobody checks); `oms.Deps.SpotBaselineDocument` (pin + fingerprint
+ dialect) mutually exclusive with the `SpotBaselines` data seam
(`ErrSpotBaselineDoubleSourced`), loaded inside `New` against the journal it
just opened; `venueWiring.fingerprint` exposed; the boot threads
`SpotBalances: wiring.client` always, the settle pair when configured, and
the pin when present. Renderer: `spot unexplained` lines sit BESIDE the
derivative deltas (same finding, one leg over), `spot unconcluded` after
`unasserted`. The two `validConfig` fixtures (reconcile, exercise) gained
`SpotSettle: 2s`. The stale `UnassertedNoPositionSurface` comment in
Validate's spot arm was rewritten.

**NOT YET DONE (the next session's list, in order):**

**(a) `internal/oms/record_test.go` — the protocol has NO tests; the design
is settled, transcribe it:** fakes at the VENUE level (the driven adapters
are real): a per-id scriptable `driven.VenueOrderReader` — `statuses
map[string][]venue.OrderStatus` walked per call (last repeats), ONE shared
`venue.ExecutionPage` per symbol (both spot orders share BTCUSDT; `attribute`
filters by id, which is the realistic answer), `Watermark` past `until` for a
complete sweep, urgency log — and a `driven.SpotBalanceReader` fake with
`reports []venue.SpotBalanceReport` walked per call + urgency log
(`ObservedAt` via `clock.NewServerTime`; the ENGINE normally stamps it, the
fake must). Journal pre-populated via `driven.OpenSQLiteJournal` BEFORE
`oms.New` (planned/authorized/acked/state rows — module_test.go's
`TestASecondModuleRebuilds` shape), reuse `fullDeps` with overrides and
`RecoveryReadPriority: Routine`. Ten cases: HAPPY (a Planned-only order
beside a terminal filled one RECORDS — §70h.12; the recording wait called
exactly once with SpotSettle; every urgency on BOTH surfaces is Routine —
§70h.13's seam evidence; ours=0.5−0.0005 fee=0.4995, B=1.0 (words
free/locked), baseline=0.5005 via the returned loader recomputation; document
rows exactly [the filled order]); StateSubmitted spot order →
`ErrRecordSpotOrderOpen`; short watermark → `ErrRecordSweepIncomplete`;
venue Executed ≠ swept rows → `ErrRecordUnattributed`; unpinned BTC row →
`ErrRecordBalanceUnpinned`; statuses [0.5 then 0.6] →
`ErrRecordCumulativeMoved`; balance reports differing across the wait →
`ErrRecordBalanceMoved` (§70h.18 = these two are SEPARATE fixtures); absent +
recently-authorized → `ErrRecordAbsenceUnproved`; nil SettleWait →
`ErrRecordSettleUnwired`; prior via `Deps.SpotBaselines` {BTC 0.3} →
Absorbed {0.3 → 0.5005, delta 0.2005}. Useful shapes: `venueReach()` from
driven/orderresolver_test.go; `venue.OrderPresent`, `venue.OrderStateFilled`.
Also add the driven-adapter zero-priority construction refusal if no fixture
reaches it.

**(b) Config refusal tests + renderer goldens:** spot_settle
required-when-spot AND refused-when-spotless; pin hex-shape refusals; the
`TestConfigRefusals` table rows; renderer goldens for SpotUnexplained order
against existing sections and SpotUnconcluded lines (§70h.11/21) incl. the
combined latching-plus-unpinned boot output.

**(c) Item 5 — UNSKIP the six `awaits #179` fixtures** (grep it: reconcile
app_test 1, exercise app/filters 2, chaos 3). They exercise ROOTS, so the
path is: build the fixture journal, `baseline.Write` a document against it,
sha256 the bytes, put {path, digest} in the fixture config — NOT the Deps
data seam.

**(d) Item 6 — §70h checklist audit 1–24** (many landed at the two safe
points: 1–3, 7, 16, 19, 20 in baseline_test; 5–6 partly in engine
balances_test + join_test — AUDIT, don't assume; 22's encode assertion is in
balances_test; 4, 8–15, 17–18, 21, 23–24 are spread over join_test/
reconcile_test from session 103 plus (a)/(b) above — verify each by name)
**then the ≥15-mutant campaign** (battery list at §70h's tail; remember
MUTATE_TIMEOUT spelled `180s`, and the §70h.16 Binance free/locked swap is
BEHAVIOR-EQUIVALENT (sum commutes) — the Bybit value-swap is the loud one,
already a fixture).

**(e) Item 7 — spec edits → v0.9.49:** §7.5 spot paragraph rewrite discharges
the ONE `[SPEC EDIT OWED]` at join.go:~502; §5.2 gains `spot_baseline`
{path,sha256} optional + `spot_settle` required-when-spot (+ the CLI:
`arblab baseline record`); profiles already carry the dated block;
adr-history dated block; §70's WIP banner → closure record.

**(f) Item 8 — the final fresh `code_diff` review** (round-4 design review
stays optional). Then commit as the slice close.

**Standing user decisions, unchanged:** the $20/$55 live-test decision; #171;
§38f. Capture **PID 8495**, never rebuild over `bin/arblab`, `make ci` runs
`-race` AND the ka staleness gate (needs node), cap `TLC_WORKERS` at 4,
docs/i18n parity is `bun test`.

---

## Status (2026-08-05, session 103): **#179 IS MID-SLICE AT A COMMITTED SAFE POINT — THE DESIGN (RECORD §70) IS THROUGH THREE ADVERSARIAL ROUNDS WITH ALL FOURTEEN MAJORS FOLDED IN, THE APPLICATION CORE IS IMPLEMENTED AND `make ci` IS GREEN, SIX FIXTURES ARE LOUDLY SKIPPED AWAITING THE BASELINE LOADING PATH, AND ONE `[SPEC EDIT OWED]` MARKER STANDS BY DESIGN.**

**Read record §70 (`docs/superpowers/specs/2026-08-05-m3-179-spot-baseline.md`)
before touching anything — it is the plan of record, and its review header
lists every round's findings.** The design: a spot baseline is `B − ours` per
(venue, account, base asset), recorded by `arblab baseline record` under a
six-step protocol (Planned-only orders excluded via `MayExistAtVenue`;
terminal + sweep-concluded + not-`Unattributed`; balance read; `spot_settle`
wait; BOTH surfaces re-read unchanged; O_EXCL dated document), stored as
INPUTS (venue words verbatim, per-order rows) with FOUR bindings (journal
chain mark via new `SQLiteJournal.Head/HashAt`, credential fingerprint,
account label, config `{path, sha256}` digest pin); asserted inside
`GateNetDeltaAsserted` at ASSET granularity with base-asset fees subtracted;
Bybit's unverified walletBalance/locked split gets the both-readings rule
(pinned only at `locked == 0`, else `SpotUnconcluded` → `OutcomeIncomplete`);
a spot delta LATCHES only if STABLE across a second look (Reconciler waits
`spot_settle`, re-runs the idempotent join, re-asserts); a baselined asset
must not be any spot pair's QUOTE (constructor deny); read priorities are
constructor-pinned per call site (boot Safety, record Routine — both the new
balances adapter AND `driven.OrderResolver`, whose const pin expired);
Binance `GET /api/v3/account` is weight **20** (a summarized fetch misread 10
— grep raw docs for load-bearing numbers); Bybit wallet-balance joins the
closed endpoint roster at 50/s (complete-roster ripple = every Bybit profile
must declare it).

**IMPLEMENTED at this safe point (all green under `make ci`):**
`domain.Ledger` per-asset fee retention + `OrderRecord.Fees` (+ tests);
`SQLiteJournal.Head/HashAt` + `ErrJournalMarkBeyondHead` (+ tests);
`ports.SpotBalances` (Words verbatim, ObservedAt, Unpinned reasons);
`application.SpotLeg`/`SpotInstrument`(Base+Quote)/`SpotBaseline` +
`validateSpotLeg` (incl. quote-overlap deny, unconsumed-baseline refusal,
balances-required-iff-baselines); `Joiner.Assert`'s spot half (roster-driven
coverage, fee subtraction at the one `feeQuantity` crossing, asset-cell
grouping, absence-is-zero, per-venue single read + duplicate-row refusal);
`Assertion.SpotUnexplained`/`SpotUnconcluded` + `Holds`; the taxonomy swap
(`UnassertedNoPositionSurface` DELETED, `UnassertedBaselineUnrecorded` holds
— the §7.5 exemption's membership is now empty, marker owed); `Reconciler`
second look (`SecondLook{Wait,Settle}`, both-or-neither, unwired-meeting-
delta = named `OutcomeFailed`, precedence NeedsOperator > Incomplete >
Passed); `driven.SpotBalances` adapter (venue+priority constructor-pinned,
engine→port reason translation); `engine/venue` balance vocabulary types;
`driven.OrderResolver` priority now a constructor argument;
`oms.Deps` gains `SpotBalances`, `SpotBaselines`, `RecoveryReadPriority`
(required), `SettleWait`+`SpotSettle`; module derives the spot roster from
`InstrumentAssets`. Profiles §4 gained the dated 2026-08-05 balance-surfaces
block (weight 20 corrected).

**NOT YET BUILT (the next session's work, §70 has every ruling):**
(1) engine/venue balance CODECS + client method `SpotBalances(ctx, urgency)`
(Binance free+locked from unset-`omitZeroBalances` read; Bybit UNIFIED
coin-less call, both-readings pinning, borrow arm) + transport roster row
`/v5/account/wallet-balance` → `wallet_balance` (requests, 1s) + every Bybit
profile/fixture gains the declaration; (2) the baseline DOCUMENT package
(write O_EXCL; load: digest → chain mark → fingerprint → account/venue →
rows-vs-prefix → recompute `ours` and `B` via the ONE shared per-venue
derivation) ; (3) `arblab baseline record` (six-step protocol, Routine
wiring, prints absorbed delta when a prior baseline is configured);
(4) reconcile config: `spot_baseline {path,sha256}` OPTIONAL + `spot_settle`
+ root threading (SettleWait=time.Sleep, baselines into oms.Deps) +
renderer lines for SpotUnexplained/SpotUnconcluded + goldens; (5) UNSKIP the
six fixtures marked `awaits #179` (grep it) once fixtures can pin a baseline;
(6) the §70h checklist items 1–24 as tests + the ≥15-mutant campaign;
(7) spec edits: §7.5 spot paragraph rewrite (discharges the one OWED marker
in join.go), §5.2 fields, adr-history block, docs → v0.9.49; (8) a fresh
round-4 design review is OPTIONAL (three rounds converged the shape; the
final code_diff review is the required one).

**Standing user decisions, unchanged:** the $20/$55 live-test decision; #171;
§38f. Capture **PID 8495**, never rebuild over `bin/arblab`, `make ci` runs
`-race` AND the ka staleness gate (needs node), cap `TLC_WORKERS` at 4,
docs/i18n parity is `bun test`.

---

## Status (2026-08-05, session 102): **#183 AND #167 ARE CLOSED — TRADING FILTERS HAVE A FETCHED-PROVENANCE WRITER AND AN ADMISSION-TIME AGE GATE, THE KA GENERATOR HAS A ROOT MAKE TARGET AND A CI STALENESS GATE, DOCS ARE v0.9.48, TWENTY MUTANTS ZERO SURVIVED, FINAL code_diff REVIEW SHIP WITH ZERO FINDINGS. MARKER GREP IS ZERO.**

**What session 102 did, in order.** (1) **#167**: `make tutorial-ka` /
`make tutorial-ka-check` land, the check wired into `make ci` as
`assemble.mjs --verify` — the node-only spelling of the bun parity
assertion, because CI has node and no bun. Bite-checked: a hand-edit of
the generated file and a real fragment change both kill; a
trailing-newline probe was a NO-OP (unfence trims it — the mutation must
be in the equivalence class the code distinguishes); regen shown
byte-stable. Committed 2009e96. (2) **#183 design** (record **§69**): one
adversarial design-review round returned five majors, ALL validated
against code and folded in — non-2xx/redirect bodies earning a `fetched`
stamp, cell binding missing quote/status/contractType (a SUSPENDED
instrument keeps valid numerics — the asymmetric-rejection harm arriving
through the freshness mechanism itself), backward-clock negative age
reading as fresh, path-textual overwrite guard (symlinks defeat it;
O_EXCL doesn't), and the one-instant claim having no discriminating
fixture on a fixed clock. (3) **#183 implementation**, split: the
orchestrator built `transport.PublicClient` (params-carrying Poller
sibling, shared `doGet`, redirects surfaced as 3xx results),
`TradingFilters.ObservedAt` (required — zero-value-unusable in the safe
direction), the Executor's admission freshness (ONE `clk.Now()` per
package; inclusive bound; `ErrFiltersStaleAtAdmission` +
`ErrFiltersFromTheFuture` as separate sentinels because the operator's
fix differs), and the `FilterMaxAge` threading (oms.Deps →
`WithInstrumentFilters(filters, maxAge)` — one option because it is one
policy; the exercise root reuses `Reference.MaxAge` so ONE declared
tolerance governs boot and admission); a codex session
(`impl-183-fetcher`) built the fetcher + `arblab reference fetch` from
§69b verbatim — four pure decoders over trimmed 2026-08-05 live-probe
fixtures (fapi's roster fixture holds THREE symbols with BTCUSDT in the
middle, asserted in-test; the spot fixture keeps both LOT_SIZE and
MARKET_LOT_SIZE with their live different steps), all-or-error document
assembly, O_EXCL + partial-write cleanup, and a 429-with-poisoned-body
fixture that proves status-precedes-decode DISCRIMINATELY (decode-first
returns the wrong sentinel). Every changed file read by the
orchestrator; one one-liner applied directly (trailing newline on the
written document). (4) **Evidence**: §69d checklist run before the
campaign — the one-instant claim needed a STEPPING clock (on a fixed
fake, one read and per-leg reads are observationally identical); twenty
mutants, zero SURVIVED (19 harness kills; one NOBUILD re-aimed — the sed
orphaned the file's only `http` use). (5) **Docs v0.9.48**: §2.4 gains
the twice-judged max row age, §40d the unsigned-public-fetch writer
shape, `ProvenanceFetched`'s comment carries both writer shapes,
venue-profiles §5 gains the dated 2026-08-05 re-probe note
(`lotSizeFilter`/`priceFilter` parent keys pinned; padding-verbatim
fact), adr-history has the dated block with the five-finding table, §69's
WIP banner is the closure record. (6) Final fresh `code_diff` review:
**SHIP, zero findings** — the reviewer also ran a LIVE
`arblab reference fetch` against the committed document: all four cells
fetched, bound and validated, values matching. `make ci` green both
modules, verified uncached by the orchestrator.

**Deferred WITH arguments (do not silently reopen):** the Planner's
freshness mirror and hot-reload land with the long-running engine's own
loop (§69b — the planner has no clock, its one caller is one-shot, and
the executor covers the replay path); #184's single-order Dispatch
surface is untouched and still owed to its own task.

**What the next session picks from:** open tasks #171 (Bybit signer vs a
real testnet credential), #179 (spot-leg exposure baseline), #184
(Dispatch filter conformance — waits on §6.7's compensating-trade
caller), #153 (restart_boundary ruling), #159 (capture restart timeout
raise), #177 (FM-9's consumer — milestone 4), #180 (browser-confirm
positionRisk weights). Clock-bound: #158 (§38f off-host archive,
~2026-09-30) and #152 (`arblab crossvenue gate`, BCHUSDT binds
~2026-09-30).

**Standing user decisions, unchanged:** the $20/$55 live-test decision;
#171; §38f. Capture **PID 8495**, never rebuild over `bin/arblab`,
`make ci` runs `-race` AND the ka staleness gate (needs node), cap
`TLC_WORKERS` at 4, docs/i18n parity is `bun test`.

---

## Status (2026-08-05, session 101): **#188 IS CLOSED — THE POST-WRITE REVIEW COMPLETED WITH FIVE FINDINGS ALL FIXED AND RE-VERIFIED, THE COLLAPSE CHECKLIST CAUGHT TWO OF §68e's OWN NAMED FIXTURES ABSENT, THE FOURTEEN-MUTANT CAMPAIGN RAN WITH ZERO SURVIVED, AND DOCS ARE v0.9.47. MARKER GREP IS ZERO.**

**What session 101 did, in order.** (1) The two queued findings fixed
through codex session `impl-188-reservation` and the diffs read by the
orchestrator: `Hold.Consume` gained `admits`' `units == 0` guard with the
§68e discrimination fixture demonstrated red-then-green (the guard was
absent from **Appendix A-2 itself** — corrected in the record), and
signed.go's dry-run names its unreachable error. (2) The post-write
review COMPLETED over every remaining file (wire/app/config + the four
root lifecycle fixtures — faithful to A-15…A-17 including round 8/9's
shapes — renderer + golden, module/acceptance/exercise, and all new test
files). It produced finding 3: **A-18's "after the fold-miss lines"
order had NO observer** — no fixture combined `FoldMisses` and
`Reserved`, so swapping the render loops passed everything; the golden is
extended and the swap was demonstrated red both ways. (3) The collapse
checklist ran BEFORE the campaign and caught findings 4/5 — §68e's
reserved-heavy admits boundary fixtures (`used+reserved == ceiling ±1`
plus the cross-priority `reserved > avail` underflow corner) and the
round-7 alias-pair lock fixture were specified and ABSENT; both added,
green against unchanged production code. (4) The campaign per §68e/A-19:
fourteen mutants, zero SURVIVED — results block in §68e, including one
PANIC!! re-verified as an assertion kill on the single fixture, one
genuine NOBUILD!! re-aimed (orphaned `math` import), and the
**`MUTATE_TIMEOUT=180` spelling trap**: A-19's own unitless spelling
fails `go test -timeout` at flag parse and reads as NOBUILD!! — §68e now
says `180s`. `holds[i]`→`holds[0]` was KILLED by the port-contract
fixture's distinct holds and recorded as behavior-equivalent in the
shipped composition, per §68e. (5) Docs to v0.9.47: §7.6 gains the
reservation + one-spender-lock clause and the §67-premise scoping
sentence (the per-host slice is now machine-checked; the premise stays
operational only across hosts and against lock-unaware processes); §6
step 6 gains reserve-before-fan-out; §4's Place carries the hold
(discharging the one [SPEC EDIT OWED] marker in ports/gateway.go); §5.2
gains `spend_lock_dir` (required, with example) and the
signing-readiness construction sentence beside the recvWindow rule;
adr-history has the dated block; §68's WIP banner is replaced with the
closure. Tutorial parity checked: its rate-limit mentions are venue
facts, not §7.6 clause teaching — no edit, ka untouched (#167 still
pending). `make ci` green on both modules, verified by the orchestrator
after every write batch and once uncached at the end.

**What the next session picks from (no ordered debt left on #188):**
open tasks #171 (Bybit signer vs a real testnet credential — also the
fold-miss baseline #187 assigned it), #179 (spot-leg exposure baseline),
#183/#184 (trading filters: fetched provenance + single-order Dispatch
conformance), #153 (restart_boundary ruling), #159 (capture restart
timeout raise), #167 (ka assemble make target), #177 (FM-9's consumer —
milestone 4). Clock-bound: #158 (§38f off-host archive, ~2026-09-30) and
#152 (`arblab crossvenue gate`, BCHUSDT binds ~2026-09-30).

**Standing user decisions, unchanged:** the $20/$55 live-test decision —
its single-spender confirmation now names exactly the remainder the
spend lock cannot check (across hosts, lock-unaware processes); #171;
§38f. Capture **PID 8495**, never rebuild over `bin/arblab`, `make ci`
runs `-race`, cap `TLC_WORKERS` at 4, docs/i18n parity is `bun test`.

---

## Status (2026-08-05, session 100 — HISTORICAL): **#188'S DESIGN CONVERGED (ROUND 11 SHIP) AND THE IMPLEMENTATION LANDED THE SAME SESSION — A-1…A-18 COMPLETE, `make ci` GREEN ON BOTH MODULES VERIFIED BY THE ORCHESTRATOR, MARKER GREP = EXACTLY THE ONE OWED. THE WORK IS UNCOMMITTED-THEN-COMMITTED AS THE SESSION-END SAFE POINT. TWO REVIEW FINDINGS ARE QUEUED AND NOT YET FIXED; THE MUTATION CAMPAIGN HAS NOT RUN; DOCS ARE STILL v0.9.46.**

**Where the implementation came from.** A second-model codex session
(`impl-188-reservation`) built Appendix A verbatim in two turns — the
first crashed client-side at ~27min with ~950 insertions preserved in
the tree; the resumed turn surveyed and finished (A-15…A-18, root
lifecycle, renderers, acceptance/composed/boot tests). Its report:
zero deviations from Appendix A; the §68b.5a wall.go
calibration-constancy caveat re-verified (`Wall.cals` built once, no
mutation path — the completeness claim stands). ~2,000 insertions
across ~40 files.

**THE TWO QUEUED FINDINGS (from the orchestrator's partial post-write
review — fix FIRST, through the same codex session, before anything
else):**
1. **DEFECT — `Hold.Consume` refuses zero-unit windows**
   (`internal/platform/ratelimit/hold.go`, capacity loop): the check
   `used <= Capacity && units <= Capacity-used` fails its first
   conjunct when a fold has ratcheted `used` past capacity on a window
   this entry charges ZERO units on — refusing a leg whose charge
   cannot violate that window, manufacturing the exact post-fsync
   episode the slice removes. `admits` (window.go) guards with
   `units == 0 → true`; Consume must mirror
   (`if units == 0 { continue }`), WITH the fixture that catches it
   (fold past capacity on window W, consume an entry charging 0 on W
   and >0 on its sibling — must ADMIT; §68e's discrimination family).
2. **NIT — signed.go's WriteProxy dry-run** discards the
   `http.NewRequest` error (`req, _ :=`) before `req.WriteProxy` —
   unreachable after the preceding parse succeeded, but the repo's
   idiom names unreachability rather than discarding it.

**What the next session does, in order:** (1) loop the two findings
through codex session `impl-188-reservation` (context intact), re-run
`make ci` YOURSELF; (2) finish the post-write review — read against
Appendix A the files not yet reviewed: app/reconcile
config.go/wire.go/app.go + app_test.go (the four root lifecycle
fixtures), cmd/arblab/reconcile.go + test (golden intact?),
module_test.go's composed two-leg test, the acceptance test,
reservation_test.go, order_test.go, signed_internal_test.go,
spendlock_internal_test.go, exercise test edits (already reviewed:
window/governor/hold/errors in platform, credential/signing/spendlock*/
signed in transport, order.go, ports/, execute.go, dispatch.go threading
— all faithful); (3) run the COLLAPSE CHECKLIST on the new fixtures
BEFORE mutating (§68e's named collapse dimensions per fixture); (4) the
mutation campaign per §68e/A-19 — six named mutants: per-call-site
`holds[i]`→nil, `holds[i]`→`holds[0]` EXPECTED-EQUIVALENT (record, not
kill), skip-last-leg in the driven adapter (composed test kills),
Consume→Acquire in Do's hold branch (exact-Used kills),
defer-Close-inside-newOrderClient (root fixture 1 kills), delete the
oms.New error-arm release (root fixture 4 kills) — plus §68e's
boundary/wrap mutants; `MUTATE_MODULE` unset, `MUTATE_TIMEOUT=180`
where the exercise suite is in scope; (5) docs to v0.9.47 per §68e's
inventory (build-spec §7.6 package-reservation clause +
spend-exclusivity scoping of §67's premise, §6 step 6 + §4 Place
signature [discharges the one SPEC EDIT OWED marker in
ports/gateway.go], `spend_lock_dir` config schema + example,
adr-history row, tutorial-parity check; ka is generated — never
hand-edit); (6) close #188, replace §68's WIP banner, update this
handoff. Rejections that must not be reopened: **§68d items 1–12**.

---

## Status (2026-08-04, session 100 mid-session — HISTORICAL): **#188'S DESIGN IS CONVERGED — ROUND 11 RETURNED SHIP AFTER ELEVEN ADVERSARIAL ROUNDS. RECORD §68 HOLDS THE DESIGN, THE FULL ARC (§68f, ROUNDS 1–11), TWELVE RECORDED REJECTIONS (§68d) AND APPENDIX A, THE FILE-BY-FILE IMPLEMENTATION SPEC. NOTHING IN `internal/` HAS BEEN TOUCHED.**

**What sessions 99–100 settled beyond the session-99 block below.** Rounds
5–10 each NO-SHIP, every finding adjudicated against the shipped code:
round 5 rekeyed the spend lock off the credential and narrowed the
guarantee to lock-aware processes (two rebuttals recorded as §68d items
10/11 — the in-flight charge ladder and persisted governor handoff, both
pre-existing properties this slice neither narrows nor widens); round 6
made a malformed endpoint host the third member of round 3's
boot-detectable post-fsync genus (canonical base-URL parse at
NewSignedClient; the `price` seam widened to `prepare`, shared by Do and
ReservePlace); round 7 rekeyed the lock AGAIN — the open `ids.Venue`
alias to the closed signer-answered `transport.SpendDomain` — added
`u.Hostname() != ""` (`:443` round-trips with an empty hostname), and
refused control bytes in credentials at the carrier (item 12 rejecting
the representative-wire variant); round 8 made the token's lifetime
structural through the root's two-stage lifecycle; round 9 CORRECTED
round 8's fold (the predicate is an owned REACHABLE spending capability —
the `oms.New` error arm releases, §68e fixture 4 flipped); round 10 fired
item 12's own reopening trigger (the Host header is config-derived) and
closed it with a `WriteProxy(io.Discard)` dry-run at construction —
stdlib-only, with the direct-path silent-zeroing mechanism verified
against the pinned request.go and recorded. Round 11: SHIP, zero material
findings, triggers 10–12 checked un-fired.

**What the next session does, in order:** (1) dispatch a codex
implementation session from **Appendix A verbatim** (sections A-1…A-20;
the evidence plan is §68e), post-write review every changed file
yourself; (2) collapse checklist BEFORE the mutation campaign, then the
campaign per §68e/A-19 — six named mutants including the two lifecycle
ones (defer-Close-inside-newOrderClient; delete the oms.New error-arm
release) and the recorded expected-equivalent (`holds[i]`→`holds[0]`,
single venue); (3) docs to v0.9.47 per §68e's inventory (§7.6
package-reservation clause + spend-exclusivity scoping, §6 step 6 + §4
Place signature edits, `spend_lock_dir` config schema + example,
adr-history row, tutorial-parity check); (4) close #188, replace §68's
WIP banner, update this handoff. The rejections that must not be
reopened without new evidence: **§68d items 1–12** (10–12 carry explicit
reopening triggers).

---

## Status (2026-08-04, session 99 — HISTORICAL): **#188 WAS MID-FLIGHT — THE DESIGN WAS FOUR ADVERSARIAL ROUNDS IN AND NOT YET CONVERGED. RECORD §68 (`docs/superpowers/specs/2026-08-04-m3-188-package-reservation.md`) HOLDS THE FULL DESIGN AND THE ROUND-BY-ROUND ARC (§68f). NOTHING IN `internal/` HAS BEEN TOUCHED.**

**What the design is, as it stands after round 4.** A package's rate-limit
charges are validated and PROVISIONALLY OCCUPIED as one governor operation
before any leg's fsync (`Governor.Reserve`, all-or-nothing on the summed,
overflow-checked batch; `window.reserved` — untimed occupancy, never
evicted, invisible to the observe fold); each leg's transport CONSUMES its
occupancy in `Do` instead of acquiring, converting it to an ordinary charge
stamped at consume time; Consume re-checks ONLY the venue's word — the
pause, and capacity on the folded count (non-wrapping arithmetic) — so the
one surviving post-fsync governor refusal is venue-initiated by
construction. Hold entries are a three-state machine (reserved → consumed |
canceled) under the governor lock; the executor DEFERS a blanket Cancel of
every distinct hold (occupancy does not self-heal — release of never-sent
occupancy is mandatory, release of consumed charges inexpressible). The
hold threads Executor → Dispatch → ports.Place → venue.OrderRequest →
transport.Call — Urgency's exact path, nil = the single-order
acquire-at-send mode. Wiring: ONE widened `Deps.VenueOrders` provider
(Place + ReservePlace), one driven gateway narrowed into the two ports
(round 4's blocker: an independent second carrier let reservation and
placement land on different governors), hold-governor identity refused
pre-send at `Do`, `OrderClientConfig.Governor` required and checked against
every family transport (the shared-governor prose becomes a construction
refusal). Two absorbed adjacencies: `NewSignedClient` refuses a
configuration whose clock-uncertainty band cannot clear
`min(RecvWindow, aheadTolerance)` (round 3's blocker: a config-static,
deterministic post-fsync refusal on every order — checkServerTime reused,
complete because wall.go's calibration is per-process constant), and the
one-spender guard: `transport.SpendLock`, flock on a credential-derived
fingerprint, `Signer.Fingerprint()` compared at `NewSignedClient`,
`spend_lock_dir` a required config field, root-owned lifetime — §67's
premise enforced at host scope, cross-host residual recorded.

**The arc so far (all findings accepted, §68f has the full log):** round 1
killed the charge-at-reserve-time model twice (expiry-before-consume;
Observe→Cancel erasing venue readings) and unbound the spend lock; an
orchestrator interim fix made Cancel mandatory (occupancy does not
self-heal); round 2 added the fold-past-capacity consume refusal (taken as
capacity-check, NOT the proposed epochs — §68d item 7), fixed the
zero-NotBefore overload in `freeAt` (occupancy ignored — NOT a Decision
vocabulary change, §68d item 8), and named the executor wiring; round 3
absorbed the signing-readiness refusal, forced non-wrapping arithmetic
(§66 round 6's lesson recurring), and demanded the composed
Do-obeys-ReasonCeiling fixture; round 4 made the reserve/place split
inexpressible and demanded the composed real-adapter test (skip-last-leg
mutant). Rejections that must NOT be reopened without new evidence: §68d
items 1–9.

**What the next session does, in order:** (1) round 5 — a FRESH codex
artifact-review (design_doc) of record §68, context: the shipped files
§68 cites (ratelimit governor/window/limit/decision, oms
execute/dispatch/ports/module, driven ordergateway, engine/venue order +
transport signed/signing/credential, reconcile config/wire, cmd/arblab
renderers, build-spec §7.6, records §66/§67), instruction "convergence
check; do not re-litigate §68d items 1–9 without a concrete failure their
recorded reason does not cover; if nothing rises above MINOR, say SHIP"; (2) on SHIP, write
Appendix A (file-by-file implementation spec — the §66/§67 Appendix
pattern; the file inventory is derivable from §68b's six numbered
sections); (3) dispatch a codex implementation session from Appendix A
verbatim, post-write review every changed file yourself; (4) run the
collapse checklist BEFORE the mutation campaign, then the campaign per
§68e (the named mutants: per-call-site hold arguments, holds[i]→holds[0]
recorded expected-equivalent single-venue, skip-last-leg in the driven
adapter, Consume→Acquire double-charge with exact-Used kills); (5) docs
owed at close per §68e (v0.9.47: §7.6 package-reservation clause +
spend-exclusivity scoping of §67's premise paragraph, §6 step 6 + §4
signature edits, `spend_lock_dir` config schema, adr-history row,
tutorial-parity check), close #188, replace §68's WIP banner.

**Standing, unchanged:** capture **PID 8495**, never rebuild over
`bin/arblab`, `make ci` runs `-race`, cap `TLC_WORKERS` at 4, docs/i18n
parity is `bun test`, `MUTATE_TIMEOUT` 180s with the unit when mutating
exercise. The **$20/$55 live-test decision** (now also carrying the
single-spender confirmation) and **#171** still need the USER; **§38f
(task #158)** still the only clock-bound user decision (~2026-09-30).

---

## Status (2026-08-04, session 98 — HISTORICAL): **#187 IS CLOSED — A MAPPED BYBIT RESPONSE THE API DEMONSTRABLY HANDLED THAT YIELDS NO USABLE RATE-LIMIT OBSERVATION IS A COUNTED PER-ENDPOINT FOLD MISS: MONOTONE IN `Snapshot` BESIDE `Bans`, DECIDED AFTER CLASSIFICATION (ANY 2xx NOT CLASSIFIED AS THE IP BAN, OR ANY THROTTLE-CLASSIFIED REFUSAL), RENDERED HISTORICALLY BY BOTH ROOTS WITH EXERCISE RENDER-ONCE, ADMISSION UNTOUCHED, DERIVE-NOTHING INTACT. DOCS ARE v0.9.46; MARKER GREP IS ZERO.**

**The slice is done end to end.** `make ci` green on both modules under
`-race` (run by the orchestrator), marker grep **0**, **26 targeted
mutations / 26 kills** (25 harness + 1 demonstrated-SURVIVED-then-killed),
zero survivors, zero no-ops. Design record **§67**
(`docs/superpowers/specs/2026-08-04-m3-187-fold-health.md`): four
adversarial design rounds to SHIP, Appendix A the implementation spec a
second-model session built verbatim, §67d the evidence plan the campaign
executed. Finding table in adr-history §3. Task **#187** is discharged.

**THE DECISION AND ITS BOUNDARY.** Count-and-alert; stop-opening and hold
REJECTED with recorded arguments (§67c). The proportionality argument rests
on a RECORDED OPERATIONAL premise — exactly one consumer spends the Bybit
UID at a time — which no code enforces (every root builds a process-local
governor): its enforcement home is **#188**, its reopening trigger is the
first deployment shape with a second concurrent spender, and the
**$20/$55 live-execution decision must confirm single-spender deployment**
before any live Bybit spend. #171's role for this seam is baseline
measurement of the miss rate (rate needs its external denominator; the
rendered line is deliberately historical).

**THE REVIEW ARC'S KEEPERS.** Round 1: a pure 2xx guard missed 403+10006 —
the endpoint meter ITSELF refusing headerless, the exact filed scenario;
the predicate now reads the classified penalty. Round 2: the header-time
decision counted 200+10018 (the ban wearing a success status, provable only
after the body) — the decision moved after ClassifyResponse with a
PenaltyBan suppression, surviving at header time only on the two
body-failure exits where it is the response's ONLY signal (Penalize never
ran either). Round 3: call-site ARGUMENTS are claims helper tests cannot
see — per-site hardcoded-argument mutations, all caught. code_diff round:
"byte-identical reconcile output" was a claim with no test, demonstrated
with a surviving format mutant — now pinned by a golden full-report test.
One rebuttal survived: retCode-provenance for the non-2xx arm (blinding a
diagnostic on an unverified venue claim; #171 arbitrates).

**WHAT IS NEXT.** The **$20/$55 live-test decision** (Binance perp's 50
USDT floor — grow the package or move the leg to Bybit, blocked on **#171**,
needs the USER — and it now also carries the single-spender confirmation
above), **#188** (package-level budget reservation — now ALSO the named
enforcement home for the one-spender premise), **#179** (spot leg's
exposure baseline), **#183**/**#184** (filters follow-ons), **#167** (ka
make target), **#177**, **#180**, or the **§9 acceptance campaign**. Note
for Bybit work: #171's testnet run now exercises the read-path seam, the
fold-check boot path, the endpoint-budget roster, AND supplies the
fold-miss baseline.

**Standing, unchanged:** capture **PID 8495**, never rebuild over
`bin/arblab`, `make ci` runs `-race`, cap `TLC_WORKERS` at 4, docs/i18n
parity is `bun test`, `MUTATE_TIMEOUT` 180s with the unit when mutating
exercise. **§38f (task #158) still the only clock-bound user decision**
(~2026-09-30).

---

## Status (2026-08-04, session 97 — HISTORICAL): **#185 IS CLOSED — BYBIT'S PER-ENDPOINT BUDGETS ARE EXPRESSIBLE, CHARGED, FOLDED AND REQUIRED: `ScopeEndpoint` IS A VENUE-LEVEL WINDOW PER ENDPOINT (FAMILY REFUSED), THE NAME IS PATH-DERIVED THROUGH THE DIALECT'S CLOSED REGISTRY WITH `Call.Limit` DELETED, THREE PRE-SEND REFUSALS GUARD THE REQUEST PATH, CONSTRUCTION REQUIRES THE COMPLETE DIMENSION-CHECKED ROSTER, THE HEADER FOLD NOW SURVIVES A LOST BODY, AND THE PRE-EXISTING `freeAt` UNDERFLOW IS FIXED. DOCS ARE v0.9.45; MARKER GREP IS ZERO.**

**The slice is done end to end.** `make ci` green on both modules under
`-race` (run by the orchestrator), marker grep **0**, **19 targeted
mutations / 19 kills** (17 harness + 2 demonstrated-and-reverted in review),
zero survivors, zero no-ops. Design record **§66** — §66a the six-round
design arc, §66b the converged design, §66d the implementation findings and
mutation table, Appendix B the spec the build followed verbatim. Finding
table in adr-history §3. Task **#185** is discharged.

**HOW IT LANDED.** Implementation was a second-model (codex) session working
from Appendix B verbatim; the orchestrator reviewed every changed file
against the spec, then a fresh adversarial code_diff round ran. That round
found two TEST-EVIDENCE defects, both fixed and both bite-checked by
demonstrating the killing mutation and reverting it: the composed seam test's
`Used == 40` could not see the pre-send charge (the fold alone produces 40 —
`Acquire`→`Check` survived the whole suite; fixed with a headerless case
asserting `Used == 1`, siblings zero), and the latest-refusal fixture made
the envelope both first-evaluated AND latest-releasing (the collapse
checklist's TENTH instance and a new dimension of it — evaluation ORDER;
fixed with one subtest per ordering). Two argued build deviations accepted:
the path→name map derived from the registry array (two views that cannot
drift), and the composed R1 test swapping the reader post-construction
(honest wiring cannot reach R1 — the boot check working).

**WHAT IS NEXT.** Unchanged: the **$20/$55 live-test decision** (Binance
perp's 50 USDT floor — grow the package or move the leg to Bybit, blocked on
**#171**, needs the USER), **#179** (spot leg's exposure baseline),
**#183**/**#184** (filters follow-ons), **#187** (fold health on headerless
mapped responses — the derivation this slice added is what makes it
detectable), **#188** (package-level budget reservation; the losing leg is
post-fsync `StateSubmissionUnknown`), **#167** (ka make target), **#177**,
**#180**, or the **§9 acceptance campaign**. Note for Bybit work: #171's
testnet validation now exercises the repaired read-path seam, the fold-check
boot path, AND the endpoint-budget roster — and #171's measurement is
recorded in venue-profiles §4 as the LOOSENING trigger for splitting endpoint
windows per family, never a correctness prerequisite.

**Standing, unchanged:** capture **PID 8495**, never rebuild over
`bin/arblab`, `make ci` runs `-race`, cap `TLC_WORKERS` at 4, docs/i18n
parity is `bun test`, `MUTATE_TIMEOUT` 180s with the unit when mutating
exercise. **§38f (task #158) still the only clock-bound user decision**
(~2026-09-30).

---

## Status (2026-08-04, session 96 — HISTORICAL): **#185 WAS MID-FLIGHT — THE DESIGN CONVERGED AND RECORDED, THE IMPLEMENTATION NOT STARTED. RECORD §66 (`docs/superpowers/specs/2026-08-04-m3-185-endpoint-budgets.md`): §66b THE CONVERGED DESIGN, APPENDIX B THE FILE-BY-FILE IMPLEMENTATION SPEC. NOTHING IN `internal/` HAD BEEN TOUCHED.**

**What the design is.** `ratelimit.Scope` gains `ScopeEndpoint` — a VENUE-level per-endpoint budget (family refused, keyed `("", name)`, one shared window across product categories — conservative under either reading of Bybit's unverified per-category counting question, and it keeps a two-family package structurally routable). The endpoint name is DERIVED from the call path by a closed `{path, name, required dimension}` registry in `BybitLimitReader` (`Call.Limit` is DELETED — the #186 one-carrier lesson; wiring mistakes become inexpressible). Three request-time refusals in `Governor.decide` (R1 endpoint-unbudgeted, R2 endpoint-unnamed under a per-endpoint profile, R3 endpoint-window-uncharged-by-this-cost), the cannot-count check extended so a matched endpoint window counts its own dimension (Bybit's order meter IS the create endpoint's budget — without this every Bybit create is refused by #173's own rule), boot checks in Bybit's `CheckFold` (roster name + registry dimension + exactly-1s interval; family scope refuses roster names) plus a completeness check at `NewSignedClient` over `LimitReader.Endpoints()` (all five, dimension-checked). The header fold MOVES to immediately after `client.Do` returns (a body-read failure was discarding the 429's own reading). One PRE-EXISTING bug found by review round 6 gets fixed in-slice: `window.freeAt` wraps when `used > ceiling` (a state the observe ratchet explicitly produces) and reports a false retry-immediately.

**The review arc is §66a**: six adversarial rounds, three rebuttals (one upheld a normative derive-nothing ruling, one classified the concurrent-leg race as pre-existing §6 territory, one rejected a hallucinated contract), two tasks filed with full context — **#187** (fold-health on headerless mapped responses) and **#188** (package-level budget reservation; the losing leg of an Acquire race is post-fsync `StateSubmissionUnknown`, not a clean unsent — verified in dispatch.go).

**What the next session does, in order:** (1) dispatch the implementation from Appendix B (repo norm: a codex impl session, then post-write review, then `make ci` verified YOURSELF); (2) the mutation campaign per Appendix B §9-10 plus the collapse checklist BEFORE mutating; (3) the docs owed at close, listed at the top of §66 (build-spec §7.6 clause + venue-profiles dated block + adr-history row + tutorial/ka parity if touched — docs are currently at v0.9.44, this slice makes v0.9.45); (4) close #185, replace §66's WIP banner with the close findings, update this handoff. The verified venue facts (create 10/s linear + 20/s spot upgradable; four reads 50/s; verbatim quotes; the WS-vs-API caveat on "counted separately") are already IN §66's docs-owed paragraph — do not re-fetch, do not cite the "counted separately" sentence for API budgets.

**Standing, unchanged:** capture **PID 8495**, never rebuild over `bin/arblab`, `make ci` runs `-race`, cap `TLC_WORKERS` at 4, docs/i18n parity is `bun test`, `MUTATE_TIMEOUT` 180s with the unit when mutating exercise. The **$20/$55 live-test decision** and **#171** still need the USER; §38f (#158) still clock-bound (~2026-09-30).

---

## Status (2026-08-04, session 95 — HISTORICAL): **#186 IS CLOSED — A PROFILE LIMIT OF A HEADERED DIMENSION MUST BEAR EXACTLY THE FOLD-DERIVED NAME, CHECKED AT SIGNED-CLIENT CONSTRUCTION (WHERE THE READER/GOVERNOR PAIR IS FORMED, SO NO ROOT CAN FORGET IT), AND THE RUNTIME READER NOW CANONICALIZES HEADER WINDOWS THROUGH THE SAME RENDERER THE CHECK USES — AND THE DESIGN REVIEW FOUND THE DEFECT LIVE THREE MORE TIMES: THE EXERCISE CHAOS FIXTURE AND BOTH TYPED LimitConfig FIXTURES STILL CARRIED spot_weight-STYLE NAMES, SO EVERY EXERCISE RUN AND EVERY SPOT-ONLY RECONCILE RUN WAS SILENTLY DROPPING EVERY VENUE READING. DOCS ARE v0.9.44; MARKER GREP IS ZERO.**

**The slice is done end to end.** `make ci` green on both modules, marker grep
**0**, **13 targeted mutations: 12 kills + 1 diagnosed expected-equivalent**
(the parser's `n == 0` guard — both consumers re-render through the shared
renderer, which refuses zero one step later; the `Reserve()` precedent).
Design record **§65**
(`docs/superpowers/specs/2026-08-04-m3-186-fold-name-agreement.md`), finding
table in adr-history §3, ka-tutorial parity **16/16**. Task **#186** is
discharged.

**WHAT LANDED.** `transport.LimitReader.CheckFold(name, dimension, interval)`
beside `Observations` so both share one window parser/renderer and cannot
drift. Binance: family weight/orders budgets must be named
`weight_<window>`/`orders_<window>` rendered from their own interval
(`10s`/`1m`/`2h`/`1d`); a requests budget is refused only under an EXACTLY
derivable name (round-trip through the shared parser — window-independent,
because `Observe` folds by `(family, name)` with neither dimension nor
interval checked), so `weight_guardrail` and `weight_60s` stay legal local
names. `Observations` canonicalizes header suffixes through the same renderer
(`60S` ≡ `1M` → one fold target; checked unit multiplication, because 2^55+1
seconds wraps to exactly 1s and would mint `weight_1s`). Bybit's `CheckFold`
is deliberately vacuous with the argument recorded — #185 owns that axis.
`NewSignedClient` walks `Governor.Snapshot().Limits` and refuses construction
on the first family-scoped mismatch (`ErrLimitFoldUnaddressable`, naming both
the declared and the derived name); venue-scoped entries are skipped (keyed
under the empty family, unreachable by any fold, requests-only since §64d).
Three fixture families migrated (pure renames): exercise `chaosConfig` JSON,
exercise `validConfig`, reconcile `spotOnly`. `Limit.Name` and
`LimitConfig.Name` doc contracts corrected ("ours to choose" was false for
headered dimensions the moment this landed). §7.6 gains the
fold-addressability clause.

**THE REVIEW ARC (six rounds, and one rebuttal that survived adjudication).**
Round 1: the check and the runtime derived names from different inputs — a
`60S`-spelled header would evade a check that certified `weight_1m` (the
"recomputed check must share the anchor" memory, caught before any code).
Round 2: prefix-based collision falsely refuses non-derivable guardrail
names; the scope-filter mutation was unkillable until the fixture became a
venue envelope legally SHARING `weight_1m` with a family budget. Rounds 3–4:
the fixture inventory was falsified twice — a profile takes three shapes
(typed literals, config builders, serialized documents) and the sweep covered
one. Round 5: the reviewer's spelling-tuple identity was REBUTTED (it
reintroduces the observed silent loss to guard an unobserved venue shape
whose failure direction is conservative anyway — the upward-only fold merges
to the max) and round 6 upheld the rebuttal; the rejection is recorded in
§65c with a dual-header test banking the merge.

**WHAT IS NEXT.** Unchanged: the **$20/$55 live-test decision** (Binance
perp's 50 USDT floor — grow the package or move the leg to Bybit, blocked on
**#171**, needs the USER), **#179** (spot leg's exposure baseline),
**#183**/**#184** (filters follow-ons), **#185** (Bybit per-endpoint budgets
— its CheckFold body now has a named home waiting), **#167** (ka make
target), or the **§9 acceptance campaign**. Note for any Bybit work: #171's
testnet validation exercises the repaired read-path seam AND now the
fold-check boot path.

**Standing, unchanged:** capture **PID 8495**, never rebuild over
`bin/arblab`, `make ci` runs `-race`, cap `TLC_WORKERS` at 4, docs/i18n
parity is `bun test`. `MUTATE_TIMEOUT` must exceed the chaos test's 60s bound
when mutating the exercise package (**180s — WITH the unit**). **§38f (task
#158) is still the only clock-bound user decision** (≥2 TB off-host, gate
~2026-09-30).

---

## Status (2026-08-04, session 94 — HISTORICAL): **#173 IS CLOSED — §7.6's CANNOT-COUNT REFUSAL IS TWO ENFORCED LAYERS (THE GOVERNOR ERRORS ON A COST CHARGING A DIMENSION ITS APPLICABLE BUDGETS DO NOT COUNT; A PROFILE IS REJECTED AT CONSTRUCTION UNLESS EVERY FAMILY BUDGETS EVERY CHARGEABLE DIMENSION OR WRITES DOWN WHY NOT, WITH THE VENUE REQUEST ENVELOPE MANDATORY AND NEVER WAIVABLE) — AND THE REVIEW SURFACED A LIVE DEFECT: EVERY BYBIT READ THROUGH THE SIGNED CLIENT FAILED ITS OWN PRECONDITION, BECAUSE THE TRUTHFUL REQUEST-ONLY COST WAS SPELLED AS THE ZERO Cost THE VALIDATOR REFUSES. DOCS ARE v0.9.43; MARKER GREP IS ZERO; SESSIONS 91–93 ARE COMMITTED AND PUSHED (274f944).**

**The slice is done end to end.** `make ci` green on both modules, marker grep
**0**, **19 targeted mutations / 19 kills** (+1 documented expected-equivalent,
the `Reserve()` precedent). Design record **§64**
(`docs/superpowers/specs/2026-08-04-m3-173-uncountable-cost-refusal.md`),
finding table in adr-history §3. Task **#173** is discharged; **#185** (Bybit
per-endpoint budgets are inexpressible — declared ones cross-charge, undeclared
ones lose folds) and **#186** (boot-time profile-name/fold-name agreement
check) are filed with full context.

**WHAT LANDED.** `ratelimit.Profile.Uncounted` (family + dimension + written
reason, no wildcard — the `//nofloat:allow` shape) plus the coverage rule:
every family budgets weight and orders with FAMILY-scoped limits or writes the
waiver; the venue-scoped REQUESTS envelope is mandatory and not waivable
(family request counters are additive, never the envelope — each can sit
inside its own capacity while their sum trips the shared gateway ban).
`Limit.Validate` reserves venue scope for requests (the header fold addresses
`(family, name)`, so a venue-scoped product budget can never receive the
venue's own reading). `Governor.decide` refuses — as an ERROR, no waiting
repairs it — any cost charging a caller-filled dimension its applicable
windows do not count; a waiver NEVER suppresses this (the waiver is a claim
about traffic, and the arriving cost is its falsification, fail-closed before
send). `Cost` gains `RequestOnly()` (unexported mark, constructor-only, zero
value still refused, contradiction refused); the three Bybit read call sites
converted, a composed test drives one through `SignedClient.Do` against a real
governor. Reconcile config gains the `uncounted` passthrough; its fixture's
limit names are renamed to the fold-derived convention (`weight_1m`,
`orders_10s`/`orders_1m`) it could never previously receive a fold under.

**THE TWO TRANSFERABLE FINDINGS.** (1) #173's own trigger ("the first
production ratelimit.Profile") had FIRED two slices earlier when G2's
reconcile root landed, and nobody re-checked the debt's premise — a filed
task's trigger clause is re-READ, not re-CHECKED, unless the check (one grep)
is run at open. (2) Two slices' individually-correct claims contradicted at a
seam neither suite composed: C2b's "nothing to charge" (`Cost{}`) versus
C2a's "the zero cost is the field nobody filled in" — and `Do` validates
before `Acquire`, so the read path was dead on Bybit while every builder and
governor test passed. The fix is a carrier (`RequestOnly()`), and the
composed test is the evidence the seam now has.

**WHAT IS NEXT.** Unchanged: the **$20/$55 live-test decision** (Binance
perp's 50 USDT floor — grow the package or move the leg to Bybit, blocked on
**#171**, needs the USER), **#179** (spot leg's exposure baseline), **#183**/
**#184** (filters follow-ons), **#185**/**#186** (this slice's follow-ons —
#185 interacts with #171), or the **§9 acceptance campaign**. Note for any
Bybit work: #171's testnet validation now ALSO exercises the repaired
read-path seam.

**Standing, unchanged:** capture **PID 8495**, never rebuild over
`bin/arblab`, `make ci` runs `-race`, cap `TLC_WORKERS` at 4, docs/i18n parity
is `bun test`. `MUTATE_TIMEOUT` must exceed the chaos test's 60s bound when
mutating the exercise package (**180s — WITH the unit**). **§38f (task #158)
is still the only clock-bound user decision** (≥2 TB off-host, gate
~2026-09-30).

---

## Status (2026-08-04, session 93 — HISTORICAL): **#174 IS CLOSED — THE PLACE ACK'S VENUE ORDER ID IS PERSISTED AT DISPATCH (A FOURTH JOURNAL RECORD KIND, ACK-BEFORE-STATE) AND §5.3's GATE 3 RULES A SWEPT ROW OUT AGAINST THE INSTRUMENT'S PERSISTED-ACK SET, SO A BINANCE LOST-ACK ABSENCE WHOSE WINDOW HOLDS ONLY SIBLINGS' FILLS NOW CONCLUDES TO AbsentAfterFinality INSTEAD OF REFUSING FOR EVER. THE RULE-OUT'S PREMISE — PER-SYMBOL orderId NON-REUSE — WAS CONSUMED UNBANKED BY THE FIRST DRAFT AND IS NOW VERIFIED VERBATIM FROM THE VENUE'S OWN PAGES. DOCS ARE v0.9.42; MARKER GREP IS ZERO.**

**The slice is done end to end.** `make ci` green on both modules, marker grep
**0** (the spec edits landed directly; no marker was ever owed), **19
mutations / 19 kills**. Design record **§63**
(`docs/superpowers/specs/2026-08-04-m3-174-persist-venue-order-id.md`),
finding table in adr-history §3. Task **#174** is discharged.

**WHAT LANDED.** `ports.Journal.AppendAcked` + `JournalVisitor.Acked` + the
`acked` record kind (tag 4, own table, append-only triggers; `chainLabel`
stays `/v1` — a new kind changes no existing preimage, and an older build
refuses at the kind column). `OrderRecord.VenueOrderID` with `ApplyAck` its
one producer: write-once, same-value no-op (version untouched), contradiction
refused with `ports.ErrVenueOrderIDConflict` — this order under a different
id, or the id under another order on the SAME instrument (the scope is the
banked fact's own). `ports.VenueOrderIndex` (per-instrument venue-id → owner)
is the Resolver's new read port, same store handed over as a second
capability. `attribute` gains the map and two arms — own persisted id claims
the row (`executions_found`), a sibling's rules it out, an unrecorded id still
refuses (`sweep_inconclusive`, now honestly rare). Dispatch journals the ack
BEFORE the state record (the crash between the two writes then leaves
`authorized`+`acked`, which recovery can join; the reverse leaves the
pre-#174 world) and reports `AckJournaled`/`AckApplied`/`AckConflict`;
`HoldsNewTrading` is `!Applied || AckConflict`, so the executor's fan-in
latches on a conflict, and a conflicted journal REFUSES TO REPLAY (verifies,
does not compose — `OutcomeFailed`, not `EvidenceUnusable`), so no boot ever
builds an index a conflict poisoned; the module-level composed test drives
both halves. The Joiner deliberately does NOT read the index: it refuses a
non-empty store-vs-live id mismatch (`ErrVenueOrderIDMismatch`) and past that
check attributes by whichever source is non-empty. Ordinary dropped ack
writes hold NOTHING (degrade = the old refusal, safe and late); they are
reported per leg (CLI `ack_journaled`/`ack_applied`, printed only when an id
existed) and fold into `ExitExecution`.

**THE VENUE FACTS, banked 2026-08-04 (Profiles §8):** USDⓈ-M Query Order —
*"orderId is self-increment for each specific symbol"*; SPOT glossary —
*"uniquely identifies the order on a symbol"*. Per SYMBOL, hence every
consumer scopes to the full instrument. The second-model review (Codex)
blocked the slice on this premise being unbanked and caught two more real
holes (index-retains-first-owner after a refused conflict; the Joiner as a
second authority) — all three landed, dispositions in §63.

**WHAT IS NEXT.** Unchanged from sessions 91–92: the **$20/$55 live-test
decision** (the Binance perp's 50 USDT floor — grow the package or move the
leg to Bybit, blocked on **#171**, needs the USER), **#179** (spot leg's
exposure baseline), **#173** (the other §7.6 remnant), **#183**/**#184**
(filters follow-ons), or the **§9 acceptance campaign** — for which #174 just
removed a standing operator-hold: a live cycle's recovery can now prove its
absences on Binance with a sibling's fill in the window.

**Standing, unchanged:** capture **PID 8495**, never rebuild over
`bin/arblab`, `make ci` runs `-race`, cap `TLC_WORKERS` at 4, docs/i18n
parity is `bun test`. `MUTATE_TIMEOUT` must exceed the chaos test's 60s bound
when mutating the exercise package (**180s — WITH the unit**: a unitless
`180` parses as nanoseconds, `go test` refuses the flag with zero `--- FAIL`
lines, and mutate.sh prints `NOBUILD!!` with an EMPTY compiler-error block —
an empty diagnostic under `NOBUILD!!` means the runner never reached the
compiler; reproduce by hand before re-aiming). **§38f (task #158) is still
the only clock-bound user decision** (≥2 TB off-host, gate ~2026-09-30).

---

## Status (2026-08-03, sessions 91–92 — HISTORICAL): **#182 IS CLOSED — §2.4's TRADING FILTERS (LOT STEP, MIN QTY, TICK, MIN NOTIONAL) ARE A VERIFIED PROPERTY OF THE ORDER PATH, ENFORCED BY ONE SHARED CHECK AT PLAN AND AT EXECUTE PREFLIGHT, FAIL-CLOSED ON A NIL MAP — AND THE OPERATOR DECISION IT SURFACED IS THAT THE $20 LIVE TEST CANNOT RUN ON THE BINANCE PERP: THE RE-CONFIRMED MIN_NOTIONAL FLOOR IS 50 USDT. DOCS ARE v0.9.41; MARKER GREP IS ZERO.**

**The slice is done end to end.** `make ci` green on both modules, marker grep
**0** (the slice's own `[SPEC EDIT OWED]` landed and came out the same
session), **18 mutations / 18 kills**. Design record **§62**, battery **§62h**.
Task **#182** is discharged; **#183** (fetched-provenance filters + per-package
freshness for the long-running engine) and **#184** (filter-conform the
single-order Dispatch path with its first real caller) are filed with full
context. The Georgian tutorial was regenerated (status line only; parity 16/16).

**WHAT LANDED.** `fixedpoint` gains `Quantity.OnStep`/`Price.OnTick`;
`oms/domain.TradingFilters` (placed in domain because `singlewriter` leaves it
nameable by roots — the `InstrumentAssets` precedent); one shared
`conformsToFilters` in `oms/application` enforced by the Planner per leg
BEFORE journaling and by the Executor's preflight per package BEFORE any fsync
(second pass, so a mixed-base package keeps its sharper diagnosis; catches
replayed `Planned` records conformed under an older revision — §5.3's
replan-or-discard made structural). Nil map = plans nothing, by name.
`strategy.LotRules.Tick` is OPTIONAL (pre-tick journaled revisions keep
decoding; the order-path resolver REFUSES a tick-less row — each consumer's
own safe direction). `arblab exercise` requires `reference.{path,max_age}` and
resolves the map pre-boot, refusing missing/tick-less/stale rows naming the
file; the map reaches the boot via `reconcile.WithInstrumentFilters` (an
Option, not a Config field — the reconcile fence caught the first placement,
which is that fence working). §2.4 gains the conformance bullet, §6 steps 5/6
the verification clauses — tick and min-notional were two UNMARKED spec holes
(the OWED grep cannot see an absent clause; the reading pass found them).
Profiles §5 gains the dated 2026-08-03 block: all four ticks (spot 0.01, the
other three 0.1), lot/notional rows re-confirmed, fapi `?symbol=` IGNORED,
Binance spot `MARKET_LOT_SIZE` publishes a literal ZERO step, perp
min-notional spelled `notional`. `configs/reference.btcusdt.json` keeps
`observed_at` at 2026-07-31 deliberately (the frozen milestone-2 calibration
must stay answerable; tick provenance lives in the source strings — do NOT
move it forward).

**THE OPERATOR DECISION (yours, surfaced not absorbed):** the $20 live test's
perp leg needs ≥50 USDT notional on Binance. Either the package grows to
~$55, or the perp leg moves to Bybit — blocked on **#171** (Bybit testnet
credential, needs you). Note also: the committed filter rows are
MAINNET-observed; a testnet run may meet different filters (failure mode is a
refused cycle, not exposure) — the mainnet run is the correct target.

**WHAT IS NEXT.** With the quantization blocker gone, the candidates nearest
the money: the **$20/$55 live-test decision above**, **#174** (persist
`Ack.VenueOrderID`), **#179** (spot leg's exposure baseline), **#171** (needs
the USER), **#173** (the other §7.6 remnant). The §9 acceptance campaign
remains runnable.

**Standing, unchanged:** capture **PID 8495**, never rebuild over `bin/arblab`,
`make ci` runs `-race`, cap `TLC_WORKERS` at 4, docs/i18n parity is `bun test`.
`MUTATE_TIMEOUT` must exceed the chaos test's own 60s bound when mutating the
exercise package (180s+), or a caught mutant reads `TIMEOUT!!`. **§38f (task
#158) is still the only clock-bound user decision** (≥2 TB off-host, gate
~2026-09-30).

---

## Status (2026-08-03, session 90 — HISTORICAL): **#170 IS CLOSED — §7.6's 418/IP-BAN MUST-ALERT IS DELIVERED END TO END: THE ROOT HOLDS THE GOVERNOR IT USED TO DROP, `Report.RateLimit` CARRIES THE SNAPSHOT, BANS > 0 IS ExitOperator, AND THE FINDING IS THAT THE OLD ANSWER WAS NOT SILENT BUT WRONG — A BAN READ AS ExitIncomplete, WHOSE "RETRY MAY LIFT IT" IS THE RESPONSE THAT ESCALATES A BINANCE BAN TOWARD THREE DAYS. DOCS REMAIN v0.9.40; NO SPEC EDIT OWED; MARKER GREP IS ZERO.**

**The slice is done end to end.** `make ci` green on both modules, marker grep
**0**, **13 mutations / 13 kills** (including two aimed at the PRODUCER through
the root's e2e test, so the HTTP-status→exit-code chain is checked at the
consumer). Design record **§61**, battery **§61e**. Task **#170** is
discharged. No spec document changed — the slice implements §7.6 as written —
so the set stays **v0.9.40** and the Georgian needed no regeneration.

**WHAT LANDED.** `reconcile.newOrderClient` returns the governor beside the
client (the handle G2 dropped); `Runner.RateLimit()` reads the live snapshot;
`reconcile.Report.RateLimit` carries it and `ExitClass` consults `Bans > 0 →
ExitOperator` — NOT the Storage/Cause dead-branch trap, because no gate
carries the ban fact (a banned join is `OutcomeFailed` → ExitIncomplete alone,
and a fallback can absorb the banned call entirely). `exercise.Report` gains a
POST-execute `RateLimit` beside the deliberately pre-execute `Boot` snapshot;
its ExitClass deliberately does NOT read it — a fan-out ban IS a leg at
SubmissionUnknown (RateLimited = EffectNotApplied without FLAG-2's rejection
evidence), so the leg loop already answers ExitExecution, and the claim is
pinned by `TestABanDuringDispatchIsSurfaced` with forced leg ordering. A
boot-banned run now REFUSES to dispatch at the execute gate, pre-fsync. One
shared `writeRateLimitStatus` draws §7.6's line in output: ban → OPERATOR
ALERT line, throttle → information, zero snapshot → nothing; the exercise
renderer's post-execute call is gated on Executed so the alert prints exactly
once (counted, not just "contained"). CLI usage: exit 1 = latched gate OR IP
ban.

**WHAT IS NEXT.** The candidates nearest the money, unchanged: **#174**
(persist `Ack.VenueOrderID`), **#179** (the spot leg's exposure baseline),
**#171** (Bybit testnet credential — needs the USER), and the **unowned
quantization slice** (lot/tick/min-notional; still the one genuine build
blocker for the $20 live test — nothing in the order path reads
`exchangeInfo`/`instruments-info`). The §9 acceptance campaign remains
runnable. #173 (refuse a profile that cannot count a charged dimension) is the
other §7.6 remnant.

**Standing, unchanged:** capture **PID 8495**, never rebuild over `bin/arblab`,
`make ci` runs `-race`, cap `TLC_WORKERS` at 4, docs/i18n parity is `bun test`.
**§38f (task #158) is still the only clock-bound user decision** (≥2 TB
off-host, gate ~2026-09-30). The live-test inventory and its governance gate
are in session 89's block below.

---

## Status (2026-08-03, session 89 — HISTORICAL): **#169 IS CLOSED — §6 STEP 7's HOLD HAS ITS LIVE PRODUCER AND ITS READER, BOTH INSIDE THE WRITE PATH: THE EXECUTOR FILES THE PROJECTION-STALE LATCH AT ITS FAN-IN AND REFUSES THE NEXT OPENING AT ITS OWN ADMISSION — AND BOTH OBVIOUS ADMISSION READS WERE WRONG, EACH IN ITS OWN DIRECTION. DOCS REMAIN v0.9.40; NO SPEC EDIT OWED; MARKER GREP IS ZERO.**

**The slice is done end to end.** `make ci` green on both modules, marker grep
**0**, **16 mutations / 16 kills** (one `TIMEOUT!!` re-aimed with
`MUTATE_TIMEOUT=240s` — the chaos test's bounded waits legitimately exceed the
60s default when the mutant sends nothing — then caught). Design record
**§60**, battery **§60e**. Task **#169** is discharged. No spec document
changed, so the set stays **v0.9.40** and the Georgian needed no regeneration
(§60f records the reading pass).

**WHAT LANDED.** `Executor.Execute` takes the root's `boot.Checklist` per call
(the Reconciler's settlement applied to the live path): its fan-in files
`GateNetDeltaAsserted → OutcomeProjectionStale` when a SENT leg's apply failed
— the producer `Outcome.Applied`'s doc has promised since v0.9.30 — and its
admission (`admitPackage`, before any fsync) refuses a package whose class the
concluded checks do not permit, `ErrExecutionHeld`. `boot.Checklist` gains
**`PermitsNow(Act)`**: a fold over VERDICT-BEARING outcomes only (`verdict()`
is the outcome table's third axis as a predicate), weakest ceiling vs the act,
**fail-closed on an empty verdict set**. `reconcile.Runner` gains
`Checklist()`; the exercise root hands it through; urgency maps to act by
definition (`actFor`). So in one process: stale execute → latch → next opening
refused with nothing sent → safety package still executes — asserted end to end
on one checklist with fresh legs, so the refusal can only be the latch's.

**THE FINDING — both plain admission reads are wrong, in opposite
directions.** `Posture().Permits(ActOpen)` is §59i(iv)'s dead driver (five
producer-less gates pin the full fold at reconcile-only forever). But the
latch-read that G2's ExitClass made canonical is wrong the OTHER way at a
per-act gate: a non-latching `OutcomeFailed` is a concluded HALTED verdict with
`Latched()` empty — an admission built on the latch admits an opening over it —
and the latch axis cannot say that `EvidenceUnusable` refuses safety execution
while `ProjectionStale` admits it; the posture axis does. Hence the verdict
fold. And the fold's identity is the third rule: only-the-answers folds to
PostureTrading on a FRESH checklist — Weakest's "nothing was checked and
everything passed" collapse reappearing one method over — so zero verdicts
refuses every act: a checklist no check has concluded in belongs to a process
§7.5's boot has not run in.

**WHAT IS NEXT.** The candidates nearest the money, unchanged: **#170** (the
§7.6 418/IP-ban alert no code raises), **#174** (persist `Ack.VenueOrderID`),
**#179** (the spot leg's exposure baseline), **#171** (Bybit testnet
credential). The §9 acceptance campaign remains runnable. A future `risk`
context's pre-trade gate now has its seam (`admitPackage`) and its read
(`PermitsNow`) — §60g.

**THE LIVE-TEST PATH, from session 89's full-repo inventory.** The order path
is REAL end to end — signed transports (submit-once by construction), real
Binance/Bybit payloads for spot+perp, `env:`/`file:` key refs, hosts free-text
per product family with **no mainnet guard anywhere** — so a config file
(nearest template: `internal/app/exercise/app_test.go`'s fixture) plus
withdrawal-disabled keys would mechanically send a real order today. What
separates "sent" from "trustworthy": (1) **quantization is the one genuine
build blocker** — nothing in the order path reads lot/tick/min-notional
(`planning.go` passes the literal quantity verbatim; Binance's per-symbol
NOTIONAL floor is $5–$20, exactly the $20-test's range) — about one slice,
data via `exchangeInfo`/`instruments-info`; (2) **Binance before Bybit** —
Binance's signer has verified evidence, Bybit's is #171's open question; (3)
fills are visible only via `arblab reconcile` polling or the venue UI (no
user-data stream, no `ApplyFill` producer); (4) knowingly-accepted gaps:
hand-typed limit price, perp-only net-delta invariant (#179), no IP-ban alert
(#170), no fence, no close path (flatten by hand). **The binding gate is
governance, not code:** the M2 kill gate's authority stands ("no live capital,
no mainnet keys, no promotion", BCHUSDT evidence ~2026-09-30), and the
sub-$1K canary is approved only once the USER writes its risk-acceptance
numbers — max notional, accepted worst-case loss (the uncovered leg's full
notional), manual-recovery RTO, who signs.

**Standing, unchanged:** capture **PID 8495**, never rebuild over `bin/arblab`,
`make ci` runs `-race`, cap `TLC_WORKERS` at 4, docs/i18n parity is `bun test`.
**§38f (task #158) is still the only clock-bound user decision** (≥2 TB
off-host, gate ~2026-09-30).

---

## Status (2026-08-03, session 88 — HISTORICAL): **MILESTONE 3 SLICE G3 IS CLOSED — §47f's ROSTER IS COMPLETE. THE PARALLEL TWO-LEG MARKETABLE-IOC EXECUTOR EXISTS, `arblab exercise run` IS THE FIRST BINARY THAT DISPATCHES, THE PROCESS-KILL-MID-IOC CHAOS TEST PROVES A–F COMPOSED — AND THE IN-PROCESS DOUBLE-DISPATCH BACKSTOP HAD SILENTLY SCOPED ITSELF "PER PATH" THE MOMENT A SECOND DISPATCH PATH EXISTED. DOCS ARE v0.9.40; MARKER GREP IS ZERO.**

**The slice is done end to end.** `make ci` green on both modules, marker grep
**0**, **24 mutations / 23 caught + 1 made inexpressible** (two `NOBUILD!!` and
two `NO-OP!!` re-aimed; ONE genuine survivor, fixed and re-killed — see below),
docs at **v0.9.40** across all five, the Georgian regenerated through its own
checker. Design record **§59**, battery **§59g**. Task **#181** is discharged,
which closes §47f's seven-slice roster.

**WHAT LANDED.** `oms/application` gains `Executor` (§6 step 6's package half:
preflight above every fsync, one goroutine per leg on per-leg dispatchers,
per-leg results after the fan-out), `DispatchAuthority` (the process-wide
sent-set), `DispatchRef`, `Execution`, and the re-exported urgency constants;
`oms.Module` gains `Execute`; `app/reconcile` exports `Runner` so a composing
caller can hold the boot open; `internal/app/exercise` is new — §5.2's OMS
block mounted under `"oms"` plus a `trade` block, the reconcile boot run to
conclusion FIRST and the execute step gated on `ExitClean`, after the sequence
(never as a phase — `boot.Sequence` enforces reconcile-last and that is right);
`cmd/arblab` gains `exercise run`, exit **3** = dispatched-and-needs-a-human
(recovery is `arblab reconcile run`, never a re-run). Core **§6 step 6** gains
the package rule; **§11** names the verb.

**THE FINDING — the backstop that holds when the projection broke was
process-wide only by ACCIDENT of there being one path.** `Dispatcher.authorized`
exists for one case: a failed apply leaves a SENT order at `Planned`, the
repository re-admits it, and only the in-process sent-set refuses the second
`Place`. The executor is a second dispatch path, so a private map per
dispatcher would have narrowed the guarantee to "this PATH never
double-Places" — with nothing failing, because the abandoned case is itself an
error path. Fixed twice: `DispatchAuthority` (one synchronised monotone set per
process), and then — after the battery proved the WIRING untestable (a
two-authorities mutant SURVIVED; the fragmentation is observable only through
a broken projection no real repository produces on command) — structurally:
`NewExecutor(d *Dispatcher)` inherits the dispatcher's authority, so a second
authority stopped being expressible. A check nothing could make became a
signature — slice B's own move, arriving as a fix.

**THE SECOND FINDING — the collapse checklist's TENTH instance, and the first
caught by the battery rather than before it.** The identical-base-quantity
fixture varied only upward (0.6 vs 0.5), where `cmp != 0` and the one-sided
`cmp > 0` are the same predicate; the mutant survived until the 0.4 case
existed. Checklist-first is necessary and NOT sufficient.

**THE CHAOS TEST** kills the real binary between the `DispatchAuthorized`
fsync and the transport result — deterministically, the fake venue holds both
IOCs open, so the window is a signalled state rather than a race — then
reconciles the survivor journal: both absences resolve through §5.3's four
gates to `AbsentAfterFinality`, the net-delta assertion explains every
position, exit clean, and **exactly one `Place` per `ClientOrderID` across
both runs**. A real testnet can do none of that; the venue-facts half of
testnet validation stays #171's (§59e).

**WHAT IS NEXT.** §47f is complete, so the next slice is chosen rather than
inherited. The §9 acceptance campaign (≥100 cycles, 14 days, per-fault RTOs)
is now RUNNABLE — an operational undertaking, not a code slice. The open tasks
nearest the money: **#169** (act on `Outcome.Applied=false` — the executor now
REPORTS the §6 step 7 hold and nothing yet acts on it), **#170** (the §7.6
418/IP-ban alert no code raises), **#174** (persist `Ack.VenueOrderID`),
**#179** (the spot leg's exposure baseline), **#171** (Bybit testnet
credential). Record **§59h** lists what G3 deliberately left out.

**Standing, unchanged:** capture **PID 8495**, never rebuild over `bin/arblab`
(the chaos test builds into `t.TempDir()`), `make ci` runs `-race` and `go
test` does not, cap `TLC_WORKERS` at 4, read TLC figures only from a COMPLETED
invocation, docs/i18n parity is `bun test` (not node). **§38f (task #158) is
still the only clock-bound user decision**: ≥2 TB off-host, gate ~2026-09-30.

---

## Status (2026-08-03, session 87 — HISTORICAL): **MILESTONE 3 SLICE G2 IS CLOSED — THE COMPOSITION ROOT EXISTS, §5.2 HAS AN OMS SCHEMA, `arblab reconcile run` IS REAL, AND §7.5's EXEMPTION FOR AN UNCOVERABLE INSTRUMENT WAS ARGUED FROM ONE CAUSE OVER A PREDICATE COLLECTING TWO. DOCS ARE v0.9.39; MARKER GREP IS ZERO.**

**The slice is done end to end.** `make ci` green on both modules, marker grep
**0**, **26 mutations / 26 kills** (six re-aimed from bad seds; six SURVIVED
first — three equivalent mutants that deleted DEAD code, three real gaps now
closed), docs at **v0.9.39** across all five, the Georgian regenerated through
its own checker (16/16 parity). Design record **§58**, battery **§58g**. Tasks
**#166** and **#168** are discharged; **#181** (G3) is what remains of §47f's
slice G.

**WHAT LANDED.** `internal/app/reconcile` is new — `Config` + `Validate` (§5.2's
OMS block), §5.2's key refs resolved into `transport.Credential`, a
`venue.OrderClient` built from a file, `checkJournalStorage` (#168, per-GOOS),
a `boot.Sequence` over §3.2's five phase kinds and a `boot.Checklist` over all
**eight** of §7.5's gates. `platform/boot` gains `GateJournalDurable` and
`Checklist.Latched`; `oms/application` gains `UnassertedReason`,
`UnassertedInstrument`, `HoldsPosture`; `driven.NewVenuePositions` refuses a
two-venue instrument table; `ports.PositionScope` refuses a spot scope.
`cmd/arblab` gains `reconcile run`. Core **§5.2**, **§7.5** and **§11** edited.

**THE FINDING — an exemption argued from ONE cause over a predicate that
collects TWO.** F2 ruled that an instrument whose exposure cannot be compared is
*reported* and does **not** hold the posture, reasoning entirely from the
permanent, venue-imposed gap (no venue publishes a position surface for spot).
The predicate implementing it is *no configured enumeration covered this
instrument*, which a **derivative** reaches: a scope nobody declared, one
declared for another venue, or an instrument dropped from the config while the
journal still holds an open order on it. So `GateNetDeltaAsserted` reports
**Passed on a boot that issued no position query at all** — and the degenerate
case is the one `oms.Deps.PositionScopes` documented as legal, an empty list.
Fixed by carrying the REASON and putting the exemption on it. *The general rule:
when a rule EXEMPTS something, check what else lands in the bucket it is written
over.* The tell was in the TEST — F2's fixture planted one order, on the spot
leg, so the two causes were the same answer.

**#168, AND THE CONDITION IS DURABILITY.** `GateJournalDurable` runs before the
journal is opened and refuses halted-and-latching. §13's words are about
LOCALITY, and `tmpfs` satisfies every one of them while losing the whole log at
the reboot §7.5 exists for — so a volatile filesystem is refused under its own
name, and an unrecognised filesystem or an uninspectable host is a refusal too.

**THE POSTURE IS NOT THIS BINARY'S VERDICT.** It declares all eight gates and
produces four, so reconcile-only is the honest answer on a clean account. What
separates a clean boot from an alerting one is the LATCH — and until
`Checklist.Latched` the only way to discover a latched gate was to call `Clear`
and see whether it worked. `Report.ExitClass` is the verdict: 0 clean, 1
operator, 2 incomplete.

**WHAT IS NEXT: G3** — two-leg parallel marketable-IOC plus the
process-kill-mid-IOC chaos test (task **#181**), which needed a process to kill
and now has one. Record **§58h** lists everything else deliberately left out
(the five gates with no producer, two venues, a §10 series for the gates, a
measured clock calibration).

**Standing, unchanged:** capture **PID 8495**, never rebuild over `bin/arblab`,
`make ci` runs `-race` and `go test` does not, cap `TLC_WORKERS` at 4, read TLC
figures only from a COMPLETED invocation, docs/i18n parity is `bun test` (not
node). **§38f (task #158) is still the only clock-bound user decision**: ≥2 TB
off-host, gate ~2026-09-30.

---

## Status (2026-08-03, session 86 — HISTORICAL): **MILESTONE 3 SLICE G1 IS CLOSED — §7.5's RECONCILE-ONLY BOOT IS ONE USE CASE IN THE ONE ORDER ITS PHASES ARE CORRECT IN, AND THE FUSED READING ASSERTED THE NET-DELTA INVARIANT OVER A QUANTITY THE JOIN HAD DECLINED TO REBUILD. DOCS ARE v0.9.38; MARKER GREP IS ZERO.**

**The slice is done end to end.** `make ci` green on both modules, marker grep
**0**, **23 mutations / 23 kills** (six re-aimed from bad seds; two SURVIVED
first and are recorded), docs at **v0.9.38** across all five. Design record
**§57**, battery **§57h**. Tasks **#175** and the recovery half of **#169** are
discharged; **#166** is re-scoped into **G2** and **G3** (see below).

**WHAT LANDED.** `oms/application` gains `Reconciler`, `Reconciliation` and
`UnprovedOrder`; `Joiner.Join` is split from a new `Joiner.Assert` with its own
`Assertion` report; `UnprovedReason` gains `RetryMayLift`; `platform/boot` gains
`OutcomeIncomplete`; `oms.Module` gains `Reconcile`. Core **§7.5** edited.

**THE FINDING — a contradiction between two clauses of ONE use case, visible only
from the composition.** `Join` records an order in `Absent` *precisely because*
§5.3 forbids inferring "no executions" from an absence — so it does not sweep it
and it keeps the replay's zero cumulative — and `assert`, one function later,
folded that zero in as a fact. The venue's standing position is the other side of
the subtraction, so any fill such an order took reports as an **unexplained
position delta**: §7.5's *alert for operator clearance*, which **latches**. The
boot it fires on is the one §7.5 exists for — a dispatch that reached the venue,
a crash seconds later, a propagation bound that has merely not elapsed — so a
condition that discharges itself in seconds became an incident needing a human.
Neither half is wrong alone, which is why four review rounds passed over it:
**nothing owned the ORDER**, because §7.5's three phases were three unwired use
cases. Fix: the sequence is a use case (replay → join → §5.3's rule on every
absence → assert) **and** the assertion is gated on the rebuild having concluded
— ordering alone is insufficient, because §5.3's rule does not admit executions.
The rejected order is kept as a **control that runs**.

**THE SECOND HALF — the outcome vocabulary's THIRD empty cell.** F2's own table
showed the reconcile-only/re-reportable cell as occupied by
`OutcomeUnreported`/`OutcomePending`, and **both are PRE-VERDICT** while every
other row has a member meaning "it ran and it refuses". *A product is only as
complete as the properties you tabulate it on — a cell filled by a member that
answers a DIFFERENT question from its row-mates is the tell that an axis is
missing.* `OutcomeIncomplete` is that member; reported as `Pending` instead, a
concluded-and-refusing boot is indistinguishable from a hung one, which is the
alive-but-wedged **uncovered** failure §7.4 names.

**THE BATTERY'S TWO SURVIVORS WERE THE RECORDED SWITCH-DEFAULT TRAP.** Dropping
`OutcomeIncomplete` from `posture()` and `latching()` falls through to the
trailing `return`, and `platform/boot`'s suite had no fixture for the new member
at all. Both became kills once the posture table and
`TestIncompleteRefusesWITHOUTLatching` covered it. The collapse checklist, run
first, found a **fake with a second opinion**: `fakeOrders.ApplyPlanned` returned
an UNTAGGED zero cumulative where the real repository's ledger tags it at
`Register` — invisible for four slices because nothing did arithmetic on a
replayed-but-unfilled order's cumulative, which is exactly what the assertion
does.

**WHAT IS NEXT: G2, and it is still where the BOOT SEQUENCE lands.** Nothing
constructs a `boot.Checklist` in production. The deferral now **names the
construct** rather than the feature (§3.6's rule): it waits on a
**`venue.OrderClient` built from configuration** — §5.2's OMS config schema does
not exist and credential loading has no home outside `engine/venue`. G2 delivers
the composition root, the `arblab reconcile` subcommand §11 names, and §13's
local-filesystem precondition (**#168**). **G3** is parallel-IOC +
process-kill-mid-IOC. Record **§57i** lists everything deliberately left out.

**Standing, unchanged:** capture **PID 8495**, never rebuild over `bin/arblab`,
`make ci` runs `-race` and `go test` does not, cap `TLC_WORKERS` at 4, read TLC
figures only from a COMPLETED invocation. **§38f (task #158) is still the only
clock-bound user decision**: ≥2 TB off-host, gate ~2026-09-30.

---

## Status (2026-08-03, session 85 — HISTORICAL): **MILESTONE 3 SLICE F2 IS CLOSED — §7.5's VENUE JOIN IS EXECUTABLE, THE FILL LEDGER IS REBUILT FROM VENUE TRUTH, AND THE NET-DELTA INVARIANT TURNS OUT TO COVER ONE LEG OF A TWO-LEG PACKAGE. DOCS ARE v0.9.37; MARKER GREP IS ZERO.**

**The slice is done end to end.** `make ci` green on both modules, marker grep
**0**, **31 mutations / 30 killed by the suite + 1 by the gate**, docs at
**v0.9.37** across all five, the Georgian regenerated through its own invariant
checker (16/16 parity tests). Design record **§56**, build record **§56i**.
Tasks **#176** and **#178** are complete; **#179** and **#180** were raised at
the close, and **#169**'s remainder is re-scoped onto the boot sequence.

**WHAT LANDED.** `engine/venue` gains the position surface — `PositionScope`,
`Position`, `PositionReport`, `PositionCodec` on both venues' codecs, embedded
into `OrderCodec`; `ports` gains `VenuePositions` and widens `VenueExecution`
with price and fee; `driven` gains the position ACL; `application` gains
`Joiner`/`Join` and `attribute`'s second key; `domain` gains
`OrderState.MayExistAtVenue`; `Recovery` gains `Dispatched`; `boot` gains
`OutcomeProjectionStale`. Core **§7.5** edited, Profiles gains **§9**.

**THE JOIN IS AN ENUMERATION, and that is a requirement rather than a shape.**
§7.5 alerts on foreign orders *or* unexplained position deltas, and a
per-instrument position read can only ask about instruments this system already
knows it traded — the one set that cannot contain a foreign one. The endpoint
VERSION is part of it: Binance's V2 and V3 `positionRisk` differ in what their
SILENCE means (V3, verbatim: *"only symbol that has position or open orders will
be returned"*), so only V3's absent row means *flat* — and on the same page it is
also how a foreign resting ORDER becomes visible.

**THE CUMULATIVE COMES FROM THE VENUE, WITH TWO KEYS AND A CROSS-CHECK.** F1 left
every replayed order at zero, and zero is not a missing number there — §6.7 reads
a group of zeroes as *Determined*, which is the verdict that PERMITS a
compensating trade. The join resolves every order the venue may hold, attributes
executions by client order id **or by the venue's own order id from that same
query** (the only key Binance offers, since its trade rows carry no client id),
and compares the order surface's stated cumulative against the rows it could
attribute. It needs no transaction because it advances no cursor: **idempotence
is what the live path buys with a transaction and this one gets from the venue.**

**FINDING 1 — the invariant is stated over a hedge GROUP and venue truth exists
for one leg.** No venue has a position surface for spot; the balance standing in
its place holds coins this system never bought, so it needs a baseline nothing
records. Using it anyway fails in the rare CONCLUDING direction — a pre-existing
holding reads as our long and the group reads over-hedged. So the uncoverable
instrument is **reported on every boot** (`Join.Unasserted`) and deliberately
does **not** hold the posture, because a leg whose invariant can never be
asserted would keep a correct system below trading for ever. Task **#179**.

**FINDING 2 — the outcome vocabulary's SECOND empty cell, one slice after the
first was filled without tabulating.** §6 step 7's hold is **stop-opening AND
latching**: not halted, because the order may be executing and the safety verbs
must stay available; not re-reportable, because the check that would lift it
recomputes from the projection that is known stale — and the stale reading is the
permissive one (`Planned` contributes no possible-delta interval). *An outcome
vocabulary is a PRODUCT of the properties it decides, and a missing member is
found by tabulating it rather than by waiting for a producer.*

**THE BATTERY'S SURVIVOR WAS A COIN FLIP IN THE CODE.** Deleting the product
comparison from the position lookup was caught only when Go's randomized map
iteration happened to visit the spot instrument first. The answer is not more
trials: the lookup now refuses a SECOND match, which is also a real configuration
check (an unvalidated `ContractType` splits one instrument across two keys with
different base assets — `domain.FillKey`'s recorded hazard). Three trials confirm
determinism. The one gate-covered mutation is the trailing-return trap on
`Outcome.latching`, verified by running `make lint` against the mutant with a
clean positive control.

**WHAT IS NEXT: slice G — parallel-IOC + testnet chaos (task #166), and it is
where the BOOT SEQUENCE lands.** Nothing in this repo constructs a
`boot.Checklist`, so both of recovery's report predicates —
`application.ChainBroken` (F1) and `Outcome.HoldsNewTrading` (F2) — are consumed
by nobody. Slice G builds the phase that reports them, which is also what
discharges **#169**'s remainder and **#168**/#175's boot-posture items.

**Standing, unchanged:** capture **PID 8495**, never rebuild over `bin/arblab`,
`make ci` runs `-race` and `go test` does not, cap `TLC_WORKERS` at 4, read TLC
figures only from a COMPLETED invocation. **§38f (task #158) is still the only
clock-bound user decision**: ≥2 TB off-host, gate ~2026-09-30.

---

## Status (2026-08-03, session 84 — HISTORICAL): **MILESTONE 3 SLICE F1 IS CLOSED — §7.5's REPLAY HALF IS EXECUTABLE, §5.3's ANCHOR HAS A JOURNAL RECORD KIND, AND THE BOOT VOCABULARY WAS MISSING THE OUTCOME THAT HALTS *AND* LATCHES. DOCS ARE v0.9.36; MARKER GREP IS ZERO.**

**The slice is done end to end.** `make ci` green on both modules, marker grep
**0**, **31 mutations / 29 killed by the suite + 2 by the gate**, docs at
**v0.9.36** across all five, the Georgian regenerated through its own invariant
checker. Design record **§55**, build record **§55g**. **§47f's slice F is
split**: this is F1, and the venue join + net-delta invariant are task **#178**.

**WHY THE SPLIT, on the seam the code already draws.** `boot.Gate` holds §7.5's
preconditions as a closed vocabulary and the first three are separate gates —
`GateJournalReplayed` (before any venue call), `GateVenueJoined`,
`GateNetDeltaAsserted` — and the posture is the **weakest over the required
set**, so a build that satisfies the first and not the others cannot trade. F2
also needs venue surfaces that do not exist: §7.5 names `Positions` and
`FundingLedger`, and `engine/venue` has neither.

**WHAT LANDED.** `ports` gains `JournalReplay` + `JournalVisitor` (one method
per record kind, so a fourth kind breaks every visitor at build time) and
`ErrJournalChainBroken`; `application` gains `Replayer`/`Recovery`; `driven`
gains the walk, the per-record verification and the `journal_authorized` table;
`ports.Journal` gains `AppendAuthorized` and `AppendState` **refuses**
`StateDispatchAuthorized`; `Module` gains `Recover`.

**THE REPLAY REBUILDS THROUGH THE WRITE PATH.** Every record is applied through
`ApplyPlanned`/`ApplyAuthorized`/`ApplyState`, so the recovered projection can
only reach states a live process could have produced and an impossible history
is **refused rather than installed**. Verification is **per record and before
the apply** — three claims, each with an isolating probe. **No fill is
journaled**, so after this phase an order's *state* is authoritative and its
executed *quantity* is not yet a claim at all; that is F2's. Stated limit:
**tail truncation is undetectable** without an independently durable head.

**FINDING 1 — the anchor is the mirror rule on a NEW axis.** Slice D made the
anchorless authorization inexpressible in the **projection**; the **journal**
kept the anchorless spelling as its only one, so a crash-recovered order reached
the crash boundary with no anchor *however carefully the live path had stamped
it* — and `AbsentAfterFinality` stayed unreachable for exactly the order §5.3
says recovery resolves that way. The mirrors are **journal and projection**, not
two venues and not two directions of travel. *When a rule is enforced by making
a shape inexpressible, ask which OTHER stores can express it.*

**FINDING 2 — §7.5's preconditions vary on TWO axes and the vocabulary had one.**
Restrictiveness and latching are independent; *halted + latching* had no value,
and it is what a journal that will not verify needs. As an ordinary failure it
is freely re-reportable, so a deterministic retry re-admits trading. As an
ordinary operator matter it latches at **reconcile-only**, which permits
`ActReconcile` — and the `DeactivationPending` re-drive fires a **terminal, non-reactivatable** tombstone for whichever plan the corrupt log appears to name — after its ack no `Replicate` for that `TradeID` is accepted and `ListActive` never returns it, so a trade that is in fact still open is left **permanently without a takeover path**, with its hedge-group lease released for a successor. **An act is exposure-neutral by construction only where its INPUTS are
trustworthy.** `boot.OutcomeEvidenceUnusable` is that combination, and the latch
rule now admits an escalation between latching outcomes while refusing every
non-latching one — because `OutcomeFailed` is *more* restrictive and does not
latch, so admitting it would launder the hold.

**THE READING PASS CORRECTED THIS SLICE'S OWN FINDING.** The harm was first
written as *"a tombstone that disarms venue-native protection"* — wrong about
the mechanism, since `EmergencyPlanStore` is an **internal** port and not a
venue call. What `Deactivate` actually does is worse: it is terminal with **no
reactivation**, `ListActive` is the actuator's only enumeration after host
death, and §6 step 9 makes the ack what releases the hedge-group lease. It had
reached four places, including a Go doc comment, before the pass opened the
port's declaration. *A wrong harm attached to a correct finding is the version
nobody challenges.*

**THE BATTERY: one real survivor, and it is the checklist's own lesson.**
Deleting the sequence-density check survived, because the fixture that looked
like its probe (a deleted middle record) is caught **first** by the link check —
a probe two checks can see shows that *something* objected. The isolating probe
is a **dropped prefix re-anchored to the genesis seed**, where every other rule
is satisfied exactly. Two further survivors are **gate-covered rather than
suite-covered**, and that was verified by running `make lint` against each
mutant rather than asserted: both switches' trailing return is the safe answer,
so the member's own group is untestable — the recorded trailing-return trap.

**WHAT IS NEXT: slice F2 — the venue join + the continuous net-delta invariant
(task #178).** It takes `Recovery.Unresolved` (every order whose replayed state
contributes a possible-delta interval, sorted, and deliberately **without**
`StatePlanned`), joins it to venue truth, and treats ids the journal does not
contain as §7.5's foreign orders — the canonical `OutcomeNeedsOperator`. It is
where **#169** (`Outcome.Applied` holding new trading) and **#176**
(`Fill.Account`'s producer) land, and it needs new `Positions`/`FundingLedger`
venue clients.

**Standing, unchanged:** capture **PID 8495**, never rebuild over `bin/arblab`,
`make ci` runs `-race` and `go test` does not, cap `TLC_WORKERS` at 4, read TLC
figures only from a COMPLETED invocation. **§38f (task #158) is still the only
clock-bound user decision**: ≥2 TB off-host, gate ~2026-09-30.

---

## Status (2026-08-03, session 83 — HISTORICAL): **MILESTONE 3 SLICE E IS CLOSED — §6.7's CLASSIFIER IS EXECUTABLE, `FM-5`/`FM-6`/`FM-7` ARE NOW CLAIMS THE TYPES MAKE, AND §5.1's HEDGE-GROUP KEY HAD A COMPONENT NOTHING CARRIED. DOCS ARE v0.9.35; MARKER GREP IS ZERO.**

**The slice is done end to end.** `make ci` green on both modules, marker grep **0**,
**34 mutations / 34 kills**, docs at **v0.9.35** across all five. Design record **§54**,
build record **§54h**. Tasks **#176** and **#177** were raised at the close and **#164**
is complete.

**WHAT LANDED.** `oms/domain` gains `HedgeGroup`, `PossibleDelta`,
`ReservedReductions` and `Classification`; `oms/application` gains `Classifier` and
`Verdict`; `ports` gains `HedgeGroupOrders` — **one read method**. The write repository
gains `OrdersIn` and an **account**, `Deps` gains `Account`, `Module` gains `Classify`.

**THE CLASSIFIER DECIDES AND CANNOT ACT.** §6.7 routes six clients through this gate and
every one of them wants to trade, so `Classifier` holds a one-method read port and no
writer, placer or journal: there is no method through which a compensating trade could be
issued. `Resolver`'s no-placer argument, applied to the other end of the same section.

**`FM-5` IS INEXPRESSIBLE, NOT FORBIDDEN.** `Reserve` takes **no amount** — the nominal
reading in `AmbiguousOrder.tla` is literally `Pend(0, "pkgspot", FALSE)`, and
`RESERVE_AT_CREATION = TRUE`, the control that *is* `FM-5`, has no Go spelling. An entry
becomes nonzero only through `Amend`, a producer stating what it has **committed and not
yet realized**. **`FM-6` gets the guard the spec's stated inputs never needed:** the
subtrahend's mutators live on the classifier and each **returns a fresh verdict**, so a
ledger change that does not re-classify is a call that does not exist. The asymmetry is
the lesson — *a re-run rule that is written down has a reader; the missing one has none.*
**`FM-8` lands by half** (the release is here; the generation that must be told is not)
and **`FM-9` is deferred with its construct named** — SM §7's ladder, milestone 4, task
**#177**, and the seam it reads (`OutstandingFor`) is built and exercised.

**FINDING 1 — §5.1's hedge-group key has THREE components and an order record carries
TWO.** `(account, venue, symbol)` is defined once, in §5.1, whose own `Order` field list
has no account in it; the only `Account` in the system was `domain.Fill`'s, a field with
a real consumer and **no producer at all** (task **#176**). Its failure direction is the
rare one in this kernel that **concludes rather than halts**: two subaccounts' opposite
naked legs net to a residual of **zero**, the group reads flat, no reduction is sized,
and every invariant over the group agrees. §5.1 is edited; the store holds one account
and refuses another's group.

**FINDING 2 — THE READING PASS CAUGHT A FIGURE ONE STEP BEFORE IT REACHED THE CODE.**
SM §2 cited `NC20` as **1,086,199** distinct states — the *model*'s slice E measurement
on the pre-`FM-8` graph. The current suite reproduces **1,093,953**. §46 froze the old
figure deliberately, as a record of what that release checked, which is right for a
record and wrong for a normative sentence a reader tries to reproduce; by the time this
slice read it the stale number had reached **six documents** and was on its way into a Go
doc comment. **A figure no run reproduces reads exactly like one that does.**

**THE BATTERY: 34 kills, two survivors on the first pass, and both were ONE NEW SHAPE.**
Deleting either episode-reset flag left every test green — the deadline flag is invisible
at the moment of resolution (a collapsed range returns `Determined` whatever the flag
says) and the floor flag is invisible when the next call sets the deadline itself. **The
observable difference exists only on the FOLLOWING incident, and no test had one.** For
the checklist: *when state is per-episode, the fixture that separates "cleared" from
"latched" is the NEXT episode, not the current one.*

**WHAT IS NEXT: slice F — reconcile-only boot (§7.5) + the continuous net-delta
invariant (task #165).** It owes slice D's journal column for `AuthorizedAt` **with its
replayer**, and it is where §6 step 7's `Outcome.Applied` refuse-to-trade condition
(task **#169**) and `Fill.Account` (**#176**) both land.

**Standing, unchanged:** capture **PID 8495**, never rebuild over `bin/arblab`,
`make ci` runs `-race` and `go test` does not, cap `TLC_WORKERS` at 4, read TLC figures
only from a COMPLETED invocation. **§38f (task #158) is still the only clock-bound user
decision**: ≥2 TB off-host, gate ~2026-09-30.

---

## Status (2026-08-03, session 82 — HISTORICAL): **MILESTONE 3 SLICE D IS CLOSED — THE STABLE-NOT-FOUND RULE IS EXECUTABLE, ITS ANCHOR EXISTS, AND THE READING PASS FOUND A VERBATIM QUOTE ATTRIBUTED TO A SECTION THAT DOES NOT CONTAIN IT. DOCS ARE v0.9.34; MARKER GREP IS ZERO.**

**The slice is done end to end.** `make ci` green on both modules, marker grep **0**,
28 mutations / 28 kills, docs at **v0.9.34** across all five, the Georgian regenerated
through its own invariant checker. Design record **§53**, build record **§53h**. Tasks
**#174** and **#175** were raised at the close and **#163** is complete.

**THE RULE.** §5.3 names the venue-specific stable-not-found rule twice and never states
its steps, so it is **composed**: State-Machines §2's four-step finality contract
supplies three gates and §5.3's retention paragraph the fourth — the **landing horizon**
has passed; the order is **absent from every surface**; an **executions sweep proved
coverage** past that horizon with no unattributable row; the answering surface's
**published reach** still covers the order. Any gate failing yields a named
`UnprovedReason` and **no state change**. `Resolver` holds no `OrderPlacer`, so §5.3's
"never resubmitted" is **inexpressible** rather than forbidden.

**FINDING 1 — the anchor had no home, and slice A's rejection of it had EXPIRED.** The
journal codec deliberately refused a timestamp because it would add *"a value the chain
covers but nothing reads"* and because *"when an event time is genuinely needed it is
the venue's, and it arrives on a fill"*. Both true then, both false now — and the case
that needs it is an order with **no fill and no venue record at all**. **A rejection's
REASON is a claim about the future, and nobody re-derives a rejection when the future
arrives**; no grep can see it, because the clause is *absent*. Consequence: with the
projection in memory, a crash-recovered order has no anchor and `AbsentAfterFinality` is
unreachable for exactly the order §5.3 says recovery resolves that way — a quiet halt
with every light green. Taken: `OrderRecord.AuthorizedAt` as a venue-frame
`clock.ServerTime` above the fsync; **the journal column is slice F's, with its
replayer**.

**FINDING 2 — SM §1's query-resolution set had no `Canceled`**, so the commonest thing a
resolving query can find had no legal target and the nearest available one was
`AbsentAfterFinality` — FLAG-2's own evidence-class conflation. v0.9.11's fix added this
edge for the LIVE states and stopped there: **the mirror rule's second axis,
live-vs-recovery**.

**THE READING PASS FOUND TWO MORE, and this is why it sits beside the grep.** (1) Ten
sites cited *"§6.7 step 2/3"* — **core §6.7 has no numbered steps**; they are SM §2's,
and §6.7 says so. The sharpest instance is a **verbatim quotation attributed to a
section that does not contain the phrase at all** (`"authoritative final cumulative
executed quantity"` appears nowhere in the core). C1's tell, and it was visible *inside
the same doc comment*, which cited "SM §2 step 1" correctly one gate earlier.
(2) **SM §1 stated two different evidence rules for the same target state** — the
FLAG-1 rows gate `Canceled` on the executions sweep, v0.9.11's venue-initiated row named
none, and §2 admits no per-source exemption. The code was already stricter than the row
it implements, which is exactly why nothing failed.

**THE BATTERY: 28 kills, but eight survived the first pass and every one was real.**
Three families, all recorded shapes. **(a)** The mirror rule's second axis *inside one
rule*: gate 1 had a clock placed inside the uncertainty band, gate 4 — the same band,
the deliberately opposite edge — had every fixture hours from its boundary, and the test
file's own fixture header named the pair while citing one test. **(b)**
Assertion-shaped, not fixture-shaped: `ProvenThrough != horizon` is satisfied by
**zero**, `calls < hardStop` by every wrong bound below it — both now positive
assertions against known exact values. **(c)** `AuthorizedAt`, the field the slice
exists to add, was **written by every repository test and read back by none**; and the
dispatcher's own anchor check was masked by the repository refusing the same instant one
layer down, so only the **ordering** claim (nothing journaled, nothing sent) could see
it.

**WHAT IS NEXT: slice E — the §6.7 classifier (task #164)**, where `FM-5`…`FM-9` become
executable claims. It is the first slice whose subject is a formal model's findings, so
read `docs/formal/ambiguous-order-model.md` **before** the `.tla`. Slice D hands it a
`Resolution` and classifies nothing.

**Standing, unchanged:** capture **PID 8495**, never rebuild over `bin/arblab`,
`make ci` runs `-race` and `go test` does not, cap `TLC_WORKERS` at 4, read TLC figures
only from a COMPLETED invocation. **§38f (task #158) is still the only clock-bound user
decision**: ≥2 TB off-host, gate ~2026-09-30.

---

## Status (2026-08-03, session 81 — HISTORICAL): **MILESTONE 3 SLICE D IS BUILT AND *NOT CLOSED* — CODE IS IN `6508fb1`, DOCS ARE OWED, MUTATIONS HAVE NOT RUN. DOCS STAY AT v0.9.33; MARKER GREP IS **2**, DELIBERATELY.**

**This is a mid-slice handoff at a green seam, not a close.** `make ci` is green
on both modules, every test passes, the tree is clean at `6508fb1`, and the
capture is untouched at **PID 8495**. What is *not* done is listed under WHAT
REMAINS below — read that before writing anything.

**What landed (code only).** New: `internal/oms/application/ports/resolver.go`
(`OrderResolver` + its result types), `internal/oms/application/resolve.go`
(`Resolver`, the four gates), `internal/oms/infrastructure/driven/orderresolver.go`
(the read-half ACL), plus `resolve_test.go` and `orderresolver_test.go`. Changed:
`ports.OrderRecord` gains `AuthorizedAt`, `OrderWriteRepository` gains
`ApplyAuthorized` and closes `ApplyState` against `StateDispatchAuthorized`,
`Dispatcher` takes a `clock.Clock`, `domain.successors` gains the `Canceled`
edge, `oms.Deps`/`Module` gain the read wiring. Design record is **§53**
(`docs/superpowers/specs/2026-08-03-m3-slice-d-stable-not-found.md`) — **read it
first; it holds the rulings and the rejected alternatives.**

**THE RULE, MADE EXECUTABLE — four gates, and `AbsentAfterFinality` needs all
four.** §5.3 names the venue-specific stable-not-found rule twice and never
states its steps; SM §2 supplies the four-step contract and §5.3's v0.9.33
paragraph supplies the bound. Composed: (1) the **landing horizon** has passed,
so an authorized send can no longer land after the query; (2) the order is
**absent from every surface** the venue offers (engine/venue's walk); (3) an
**executions sweep proved coverage** past that horizon and contains no row this
system cannot rule out; (4) the answering surface's **published reach** still
covers the order. Any gate failing yields an `UnprovedReason` and **no state
change** — the type can refuse to conclude and cannot conclude wrongly.

**FINDING 1 — the rule's anchor has no home, and slice A's rejection of it
expired.** Both time bounds count from the instant this system committed to the
single `Place`. Nothing recorded it: not `domain.Order` (the immutable plan-time
half), not `OrderRecord`, and **not the journal** — whose codec *deliberately*
refused a timestamp, on the ground that it would add *"a value the chain covers
but **nothing reads**"* and that *"when an event time is genuinely needed it is
the venue's, and **it arrives on a fill**"*. Both clauses were true in slice A.
Both are false now: §5.3's retention bound reads it as of **v0.9.33, one slice
later**, and the case that needs it is an order with **no fill and no venue
record at all** — there is no fill for it to arrive on, and a not-found answer
carries no venue `created time` either. **This is the recorded
rejected-alternative-is-an-unchecked-claim shape with a second axis: the
rejection's REASON was a claim about the future, and nobody re-derives a
rejection when the future arrives.** No grep can see it — the clause is *absent*,
not departed from. **Consequence:** the projection is in process memory, so a
crash-recovered order has no anchor, and `AbsentAfterFinality` is **unreachable
for exactly the order §5.3 says recovery resolves that way** — the order stays in
a state `ContributesPossibleDelta` answers true for, its group is `Ambiguous` for
ever, new trading is held, every light green. **Taken:** `OrderRecord.AuthorizedAt`
as a **venue-frame `clock.ServerTime`** stamped above the fsync; the journal
column is **slice F's, with its replayer** (schema-field-frozen-before-its-consumer).

**FINDING 2 — SM §1's query-resolution set had no `Canceled`.** §6 step 6 sends
nothing but marketable IOCs, so the commonest thing a resolving query can find is
an order the venue terminated unfilled — and that answer had **no legal target**
from `DispatchAuthorized` or `SubmissionUnknown`. Worse than stuck: the nearest
available terminal is `AbsentAfterFinality`, which is precisely FLAG-2's
evidence-class conflation, so a missing edge left a **wrong** edge as the only
legal one. The v0.9.11 venue-initiated-termination fix added exactly this edge
for the two LIVE states and stopped there — **the mirror rule's second axis
again, live-vs-recovery this time.**

**Two rulings taken, both in §53, do not silently reopen.** Gate 4 compares
against **`min(unfilled, filled)`**, because an absence must be inexplicable
under *every* hypothesis and we do not know which kind of order it was — that is
what the absence means. And an **unpublished retention licenses only what an
operator configures** (`application.SurfaceKey`): "no cliff" licenses proved
absence for ever, "never provable" makes every Binance-spot lost-ack IOC
unresolvable — half of every cash-and-carry — so both defaults are unsafe and the
carrier is forced.

**The checklist ran BEFORE the battery and found dead code in the new code
itself:** `attribute()` took a venue order id, and the only path that calls it is
the absent one, where the venue by definition supplies none — two unreachable
arms reading as handled cases. Removed, and the consequence priced honestly: **on
Binance ANY execution row inside the window makes the absence inconclusive**,
because the client order id is absent from both trade lists and the venue id is
the only other key. Closing that means persisting `Ack.VenueOrderID` and
reconciling against the set of our own orders — §7.5's reconciler, slice F.

**WHAT REMAINS for slice D to close, in order:**

1. **The mutation battery has NOT run.** `tools/mutate.sh` over `resolve.go`,
   `orderresolver.go`, `orders.go`'s `ApplyAuthorized`/`advance`, and
   `dispatch.go`'s anchor check. Aim at each gate separately — the four are
   independently falsifiable and that is the point of the shape.
2. **Two `[SPEC EDIT OWED]` markers are live** (`ports/repository.go`,
   `domain/transition.go`) and the grep is **2**, which is correct for a WIP
   commit and must be **zero** before the slice closes.
3. **Docs owed:** core **§5.3** (the journal must carry the anchor; the
   crash-recovered consequence), **SM §1's transitions table** (the two
   `→ Canceled` query-resolution rows), core **§4** (the resolver's shape if
   anything is owed there), **§12**'s milestone-3 slice-D annotation, **ADR §3**'s
   dated block, this HANDOFF, and the Georgian regen. Version goes to **v0.9.34**.
4. **The reading pass**, beside the grep and not into it — it is what caught the
   finding in each of the last three slices.
5. **§53h**, the design record's build record, is a stub awaiting the close.

**Tasks to raise at the close** (not yet created): persist `Ack.VenueOrderID` so
a Binance sweep can rule out our *other* orders; a **§7.5 boot posture check**
refusing to trade an instrument whose absences could never be proved (the check
that would be *in time*, where gate 4's `ReasonReachUnknown` is safe but late).

**Standing, unchanged:** capture **PID 8495**, never rebuild over `bin/arblab`,
`make ci` runs `-race` and `go test` does not, cap `TLC_WORKERS` at 4, read TLC
figures only from a COMPLETED invocation. **§38f (task #158) is still the only
clock-bound user decision**: ≥2 TB off-host, gate ~2026-09-30.

---

## Status (2026-08-03, session 80 — HISTORICAL): **MILESTONE 3 SLICE C2b IS LANDED AND CLOSED — THE ORDER READ PATH EXISTS, §47e FINDING 1 IS FULLY DISCHARGED, AND §4's `Cursor` CANNOT BE EITHER VENUE'S CURSOR. DOCS ARE v0.9.33.**

**Code landed in `b0f7f33` (session 79, "WIP, docs owed"); the docs half is this
session.** New: `internal/engine/venue/orderquery{,_binance,_bybit}.go` +
`orderquery_test.go`. Changed: `ports.Ack.VenueOrderID`'s comment (its claim is
**withdrawn**, see below) and `orderquery.go`'s header (both `[SPEC EDIT OWED]`
markers discharged). Design record is **§52**
(`docs/superpowers/specs/2026-08-03-m3-slice-c2b-read-path.md`).
**`venue-profiles.md` gains a whole new §8**; core **§4 and §5.3** are edited;
§12's milestone 3 gains its slice-C2b annotation; ADR §3 has the dated block.
**`make ci` green both modules; marker grep ZERO; 26 mutations, 26 kills; Georgian
regenerated (`bun test` 16/16).**

**`engine/venue`'s account/order half is now WHOLE.** §47e finding 1 — milestone
1's undelivered `engine/venue` line — is closed across both directions of travel:
C1 the signed transport, C2a `Place`, C2b `OrderByClientID` + the executions
sweep + Bybit's normative history fallback. **Nothing was added to
`oms/application/ports`**, deliberately: slice D declares `ports.OrderResolver`
**with** its consumer, exactly as C2a's `ports.OrderPlacer` landed beside
`dispatch.go`.

**THE FINDING: the obvious `Cursor` satisfies §4 completely and makes §6.7
inexpressible.** §4 declares `Fills(VenueInstrument, since Cursor)` and never says
what a `Cursor` is. The three execution surfaces offer an **ascending trade id**
(both Binance families) and an **opaque descending page token** (Bybit), and
**none can be compared to a time** — while §6.7 step 3 sweeps *"through a
venue/server watermark strictly later than"* a horizon. So "the venue's own
handle", the idiomatic and documented choice, implements the signature
**correctly on all three surfaces** and leaves the one clause that consumes it
unwritable. The cursor is therefore a **server instant**; the venue's handle is
demoted to within-sweep paging and carried in its own field. Corroborated
independently from the venues: on **both** Binance families `fromId` and the time
window are **mutually exclusive**. **Carry the tell: a cursor's DIRECTION is not
in its type** — Bybit's `nextPageCursor` walks into **older** data, so storing it
as the next `since` re-reads one window forever while every request succeeds,
every page decodes, and dedup absorbs the duplicates. Sub-finding: the executions
window cap is **24 hours on Binance spot against 7 days on USDⓈ-M** — same venue,
same parameter name, factor of seven.

**What found it: deriving the DTOs from State-Machines §2's four-step finality
contract instead of from the endpoints.** That was the stated mitigation for
freezing a schema whose Go consumer arrives in slice D, and it earned its keep —
two of four findings came from asking the contract, not the venues, and this one
before a line was written.

**FINDING 2: not-found is a RESULT, not an error.** Binance states it in an error
envelope (`-2013`), Bybit in a success with an empty list — C2a's finding with the
venues swapped. As an error it is indistinguishable at the call site from a
rate-limited query, the inference §5.3 forbids in terms. An absence is a value;
only an unanswered query is an error.

**FINDING 3: the stable-not-found rule has an EXPIRY DATE — and THE READING PASS
CORRECTED THIS FINDING'S OWN PREMISE, which makes it worse.** USDⓈ-M drops an
unfilled `CANCELED`/`EXPIRED` order after **3 days**; Bybit's history drops
`Cancelled`/`Rejected`/`Deactivated` after **24 hours**; §6 step 6 sends only
marketable IOCs, so **this system's commonest terminal outcome is the row that
becomes unqueryable soonest on both venues**, while `AbsentAfterFinality` means
**proved** absence. §5.3 now bounds it; #163 builds it. **The record claimed
nobody had written the cliff down. False for Binance:** Profiles §4's
history-retention row has carried *"some unfilled-cancelled orders purged in ~3
days"* since **2026-07-20**, self-flagged *"browser-confirm exact windows"* — the
exact verification §8 has now done. So the fact was recorded, was marked as
needing checking, and **its consequence went underived for a fortnight**, because
§4's row is about the §7.4 actuator's history enumeration and the reader carrying
§5.3's question never had cause to open it. **This is C1's
citation-to-a-section-that-does-not-exist tell INVERTED** — the section exists,
says the thing, and nobody re-read it, which no grep can catch. **Cheap habit:
before writing "X is recorded nowhere", grep the doc set for the venue's own noun.
An absence claim is a claim, and it is the one nobody verifies.**

**FINDING 4: on Binance a swept execution carries NO client order id at all.**
Neither trade list returns a `clientOrderId` or an `origClientOrderId`, so the
venue `orderId` is the only join key — and `ports.Ack.VenueOrderID` said in
shipped code that *"nothing in this system REQUIRES the venue's id"*, justified by
a parenthetical naming Bybit, the one venue where that holds. **Withdrawn in §4
and in the code.** The composition closes only by a second venue fact (a filled
order survives 90 days), which makes `Fills` and `OrderByClientID` a
**composition** where §4's signatures suggest independence — and useless alone in
exactly the scenario it exists for, a lost ack.

**Evidence: 26 mutations, 26 kills; five survivors, four real gaps.** The one to
carry: `bybitExecutionSide("Buy") → Sell` survived, which is **C2a's own §51g
survivor one slice later on the mirror path** — C2a checked the mirror across
**venues**; the one it missed is the other **direction of travel**. Every
execution fixture SOLD, because the flagship's perp leg sells, and the spot leg of
every cash-and-carry trade BUYS, so the unreached arm was half of all real fills.
**The mirror rule has a second axis.** The fifth was an **equivalent mutant** and
diagnosing it was the work: `AbsenceFromError`'s effect check is unreachable
because every `apperror` constructor that sets a `Code` pins `Effect` to
`EffectNotApplied`. Kept, and now reached via `apperror.New`, because its
necessity rests on another package's unstated invariant.

**START HERE: slice D is task #163 — the venue-specific stable-not-found rule.**
It is the consumer C2b deliberately stopped short of: it declares
`ports.OrderResolver` **with** its caller, writes the ACL adapter, and it is where
**§52d's retention bound becomes executable** — `AbsentAfterFinality` may not be
claimed past the answering surface's published reach, and `QueryReach` is the
input already sitting there for it. Task **#171** still gates any real Bybit
order, and it also closes §8's **one assumed venue fact** (that Bybit reports an
unknown `orderLinkId` as `retCode` 0 with an empty list — the codec fails closed
either way, stated in both the code and the record). **§38f (task #158) is still
the only clock-bound item**: ≥2 TB recommended, gate at ~2026-09-30.

---

## Status (2026-08-03, session 78 — HISTORICAL): **MILESTONE 3 SLICE C2a IS LANDED AND CLOSED — THE ORDER WRITE PATH EXISTS, AND ONE VENUE STATES A REFUSAL INSIDE A SUCCESS STATUS. DOCS ARE v0.9.32.**

**Code changed** (landed in the WIP commit `e152141`, docs completed this session).
New: `internal/engine/venue/order{,_binance,_bybit}.go` + `order_test.go`,
`transport/envelope.go` + `envelope_internal_test.go`,
`internal/oms/infrastructure/driven/ordergateway.go`. Changed:
`ports.OrderPlacer.Place` and `Dispatcher.Dispatch` take §7.6's shed class,
`Deps.Placer` → `Deps.VenueOrders`, `ratelimit.Cost.Validate` corrected,
`transport.ClassifyStatus` unexported. Design record is **§51**
(`docs/superpowers/specs/2026-08-02-m3-slice-c2a-write-path.md`).
**`venue-profiles.md` gains a whole new §7**; core **§4, §5.3, §6 step 6 and §7.6**
are all edited; §12's milestone 3 gains its slice-C2a annotation; ADR §3 has the
dated block. **`make ci` green both modules; marker grep ZERO; 20 mutations, 20
kills.**

**THE FINDING: a hazard written down for one INPUT is not covered for the others.**
Bybit answers a **refused** request with **HTTP 200** and a `retCode` (verified
live, no credential needed), so C1's status-only classifier read a rejection as an
acceptance. The worse outcome is one step on: a refusal body carries no `orderId`,
so the ack decode fails, and `ClassifyAckDecodeFailure` calls a failed decode after
a 2xx **`EffectApplied`** — an **explicit venue rejection** recorded as an order
that **definitely exists**, collapsing §5.3's two evidence classes into their
opposite. **Profiles §6 had already described this exact seam** for Bybit's
zero-length 401 body, and the second instance landed anyway. The fix composes a
per-venue envelope reader with the status classifier **with a direction** — a body
may contradict a success status, never soften a failure one — and its general arm
(an error body beside a 2xx with no venue rule for the code → `EffectUnknown`) is
reachable and tested on **Binance**, not on the venue that motivated it.

**FINDING 2: §7.6's shed classes had NO CARRIER, and it is now three spec sections.**
The class is a property of the §5.3 **command**; neither the `Order` aggregate nor
its projection records which command that was. Both defaults are unsafe and
unequally so — `Opening` sheds a **flatten** during exactly the incident the
reserve exists for. §5.3's new command→class table is **total over the command
set**, and `Opening` holds **exactly one command**. **One row was NOT settled by
the design record and is decided here: `CloseTrade` → `Safety`**, on §7.6's own
"monitoring outranks new exposure" argument — shedding an exit at the ~70% ceiling
makes the governor the reason a now-negative carry cannot be stood down. Reopen it
deliberately if that reads wrong. **The better long-term shape is recorded and not
taken**: a field on the `Order` aggregate naming its originating command would also
decide §5.3's `stop-opening` exemption — one carrier, two rules — and is a §5.1
aggregate change reaching the journal codec, the planner and recovery.

**FINDING 3: a documented venue ZERO met the zero-is-unusable default.** Binance
documents `POST /fapi/v1/order` as *"0 on IP rate limit"* while spot's is
*"Weight: 1"*, so the truthful cost for the most important endpoint in the system
was the one `Cost.Validate` refused. The argument survived, its subject did not:
the check is now *"charges no dimension a caller fills in"*. **Where the venue
publishes a zero, this repo's rule inverts**, and the collision is invisible
because both readings are "a zero in a required field".

**Closed: `ids.ClientOrderID`'s standing pre-milestone-3 assumption.** Both venues'
id rules verified, the minted format inside both with **one character of
headroom**, and the rules **differ** (`.` legal on Binance, illegal on Bybit) —
which is what gave the tests a discriminating fixture.

**THE READING PASS FOUND ONE, AND IT IS FINDING 3's OWN CONSEQUENCE — task #173.**
`Governor.decide` walks the **profile's** declared budgets and takes each one's
dimension from the cost, so **a dimension the profile declares no `Limit` for is
silently uncounted** — no error, no refusal, no metric. Harmless while every
metered endpoint charged weight; **finding 3 is what made it reachable**, because
accepting the documented zero made futures new-order **the one call whose entire
cost rides on a single dimension**. A Binance profile with weight budgets and no
order-count budget charges *nothing* per futures order, forever, against a venue
that bans on order counts. **Nothing is broken today** — no production
`ratelimit.Profile` exists anywhere (the app roots are `capture` and `sim`) — so
the trigger is named as the **construct**, per the deferral rule: the first
production profile. §7.6 states the requirement from this release; #173 builds it.
**Carry the shape:** the fix was right and it **relocated the premise** onto a
different object that nothing checks — `FM-5`'s family at the config layer, and
invisible to the marker grep because the clause is *absent*, not departed from.

**START HERE: slice C2b is task #172 — the order READ path** (`OrderByClientID`,
`Fills`, the Bybit history fallback). It carries one open question recorded at the
split: **those query methods have no consumer in `oms` until slice D declares one**
(the schema-field-frozen-before-its-consumer shape), so whether C2b keeps its own
number or folds into slice D (#163) is a decision to take when the read path is
next — **the ports are declared with their consumer either way**. Task **#171**
still gates any real Bybit order: `BybitSigner` has no offline oracle.

---

## Status (2026-08-02, session 76): **MILESTONE 3 SLICE C1 IS LANDED — `engine/venue` HAS A SIGNED REST TRANSPORT, AND THE SLICE'S LESSON IS THAT THE TWO ADAPTERS' *EVIDENCE* IS NOT SYMMETRIC EVEN THOUGH THEIR CODE IS. DOCS ARE v0.9.31.**

**Code changed.** New: `internal/engine/venue/transport/{credential,signing,signed,venueerror}.go`
+ `{signing,signed}_internal_test.go`. Changed: `transport/errors.go`. Design record is **§50**
(`docs/superpowers/specs/2026-08-02-m3-slice-c1-signed-transport.md`); **`venue-profiles.md` gains a
whole new §6**, core **§7.6 is corrected**, §12 milestone 3 gains its slice-C1 annotation, ADR §3 has
the dated block. **`make ci` green both modules; marker grep zero; 18 mutations, 18 kills.**
**Landed in `e1b3c78`, tree clean.**

**SLICE C IS SPLIT BY LAYER, NOT BY VENUE — and the alternative is refuted in advance.** §47f's full
slice C is 2–3× slices A and B. Splitting Binance-then-Bybit puts the two venues' clients in
**different sessions**, which is exactly what §47f's own standing hazard forbids (*"slice C builds two
venues' clients; whatever repetition, bound or isolation trick one earns, the other gets the same
session"*). Splitting by layer keeps both venues inside each sub-slice. **User accepted 2026-08-02.**
C1 = credentials, both signature compositions, the clock-injected timestamp, the §7.6 governor
attachment, effect classification. **C2 = `Place`, `OrderByClientID`, `Fills`, the Bybit history
fallback, and the `ports.OrderPlacer` adapter** — still task #162.

**FINDING 1: the scope note's premise was FALSE, and the tell is cheap.** §47f's slice-C row cites
*"profiles §1/§4/§7"* for facts it says are already recorded. **There is no §7** — the document has
ended at §5 throughout — and **how either venue composes the string it signs was recorded nowhere in
the doc set.** That is §47e's own shape one layer out: §47e caught a normative claim the code silently
failed; this is a **verification claimed as banked when it was not**. Both are invisible to
`grep -rn "SPEC EDIT OWED"` because the clause is *absent* rather than *departed from*. **Carry the
tell: a citation to a section that does not exist. Check the section exists before relying on what it
says.**

**FINDING 2 — THE ONE TO CARRY: the evidence does not mirror, even where the code does.** Binance
publishes **three** reproducible HMAC vectors (two documents, two secrets — all three re-computed
here and matching). **Bybit's masks its secret**, and the rescue hypothesis (that the secret is the
literal placeholder) was **tested and refuted**. The credential-free differential probe — sign with an
invented key twice, correctly and incorrectly, and read the venue's complaint — is **refuted on both
venues**, because each checks the credential *before* the signature: Binance answers `-2015` to both,
Bybit answers **HTTP 401 with `content-length: 0`** to both. **That refutation is only trustworthy
because Binance ran first as its positive control** — on Bybit alone the result is two identical 401s,
readable equally as *"no complaint, so it is right"*, with nothing in the observation to arbitrate.
**So `BybitSigner` has NO offline oracle and `BinanceSigner` has three.** The two look symmetric in
the code and their passing test counts are **not comparable evidence**. §47f's mirror rule covers
*fixes* propagating; **its inverse does not follow from it.** Task **#171** is the only thing that
closes it: a real testnet credential, before any Bybit order is placed.

**FINDING 3: §7.6 was WRONG about Bybit's header — corrected in the core.** It folded *"the reported
consumption (`X-MBX-USED-WEIGHT-*`, `X-Bapi-Limit-*`)"* upward-only, but `X-Bapi-Limit-Status` is
verbatim *"your **remaining** requests"* — the complement. Folding it directly **inflates** the count
when the venue reports room (90 remaining > 30 used → adopted) and **discards** it when the venue
reports almost none (2 remaining < 30 used → ignored). **The discarded direction is silent and is the
one that ends in the ban.** Fixed to `used = X-Bapi-Limit − X-Bapi-Limit-Status`, deriving nothing
when the cap is absent. Invisible because both headers are integers about the same budget in
similarly-named fields — the **format-collapse family one layer up**, colliding on *meaning*.

**THE DESIGN DECISION WORTH KNOWING: the acceptance window is asymmetric and only one side is
configurable.** Both venues test `timestamp < serverTime + 1000 && serverTime - timestamp <=
recvWindow`. `recvWindow` widens **only the behind side**; the ahead side is a **fixed 1000 ms** no
parameter can raise. So the documented knob for timing rejections repairs one direction and **cannot
touch the other** — a fast host keeps failing intermittently with the fix apparently applied. It is a
named constant composed with `platform/clock`'s uncertainty band: signing refuses when the band does
not fit inside `min(aheadTolerance, recvWindow)`. And **`recvWindow` unset is REFUSED**, because the
venues' 5000 ms default silently adopts *their* policy, and `recvWindow` bounds how long a delayed,
non-idempotent `Place` can still execute after we stopped waiting for it.

**METHOD NOTE — the collapse checklist gains a SIBLING.** Three mutants survived. Two were ordinary
fixture collapses (**the body was empty in every Binance fixture**, so signing it or not was the same
expression; **every fixture used recvWindow = 5000**, which is also both venues' *default* — the value
a fixture drifts toward). The third is a **new shape**: the credential-redaction fixture was **varied
correctly** — key and secret given different lengths so a swap would show — and the **assertion** was
what collapsed, because `strings.Contains(rendered, len(secret))` is still true when the two lengths
trade places. **Beside "the fixture does not contain the thing under test" now belongs "the assertion
does not distinguish the thing under test."**

**Also worth knowing: a data race in a TEST failed the whole package under `make ci` and passed under
`go test`.** Two parallel subtests shared one `SignedRequest`. A race abort's blast radius is the
entire test binary, so *"everything broke"* is a plausible symptom of **one** shared fixture — do not
read the first red `make ci` as a catastrophe before reading the trace.

## START HERE (session 76, 2026-08-02 — C1 is done; C2 is next)

**Branch `feat/milestone-1-foundation`. Docs are v0.9.31.**

**First act, in order:**

1. **Slice C2 — the order surface (task #162's remainder).** `Place`, `OrderByClientID`, `Fills`,
   Bybit's `GET /v5/order/history` fallback, and the first real `ports.OrderPlacer` adapter
   (`oms/module_test.go` still fakes it). **Testnet only.** Build on
   `transport.SignedClient`/`Signer`/`ClassifyStatus` — the effect classification is done, so C2
   supplies venue codes to it rather than re-deriving effects.
2. **§4's constraint C1 could not satisfy and C2 must:** every per-instrument method keys on
   **`VenueInstrument`**, not C1's `ProductCategory` — spot and perp have independent order-ID
   namespaces. `Call.Family` is right for a transport call and insufficient for `Place`.
3. **Task #171 gates the Bybit half.** There is no offline oracle for its signature. A real testnet
   credential must sign one accepted request before any Bybit order is placed. Do **not** infer
   correctness from Binance's vectors passing.
4. **§38f is STILL a live user decision (task #158)** — untouched by A, B and C1, still the only
   clock-bound item. ≥2 TB recommended; gate at ~2026-09-30.
5. **New this session: #170** (§7.6's 418/IP-ban operator alert is unmet end-to-end and unmarked) and
   **#171** (above). #152, #153, #159, #167, #168, #169 unchanged.

**Standing rituals, unchanged.** `pgrep -f "arblab capture run"` **FIRST** (PID 8495, 951 GiB free);
**never rebuild over `bin/arblab` while capture runs** — `make lint` builds `bin/arblab-**lint**`; cap
`TLC_WORKERS` (4); marker grep `grep -rn "SPEC EDIT OWED" internal/ tools/` must be **zero**; never
edit a `.tla` or `run.sh` mid-run; read figures only from a **completed** invocation. **`make ci`
runs `-race` and `go test` does not — do not call a slice done on the plain run.**

**Doc-set landing is COMPLETE, including the Georgian.** All seven docs stamped v0.9.31.
`docs/tutorial.ka.md` was regenerated with **`node docs/i18n/tools/assemble.mjs`** after editing
`docs/i18n/ka/tutorial/header.md` — **source first, then assemble**; the expected one-line diff in
each. It lives under `docs/i18n/tools/`, **not** `tools/`, and **no `make` target invokes it**; task
**#167** is still that gap.

## Status (2026-08-02, session 75 — HISTORICAL): **MILESTONE 3 SLICE B IS LANDED — §6 STEP 5 EXISTS, `module.go`'s STATED HOLE IS CLOSED, AND WHAT CLOSED IT WAS A SIGNATURE RATHER THAN A CHECK. DOCS ARE v0.9.30.**

**Code changed.** New: `internal/oms/application/planning.go` + `planning_test.go`.
Changed: `application/dispatch.go`, `dispatch_test.go`, `application/errors.go`,
`application/ports/repository.go`, `internal/oms/module.go`, `module_test.go`.
Design record is **§49**
(`docs/superpowers/specs/2026-08-02-m3-slice-b-planning.md`); core §6 step 5 is
**rewritten**, §6 step 7 gains a clause, §6's sequence diagram is corrected, §12
milestone 3 gains its slice-B annotation, ADR §3 has the dated block.
**`make ci` green, both modules; marker grep zero; 17 mutations, 17 kills.**
**Landed in `7b20f93`, tree clean.**

**What landed.** Deterministic `ClientOrderID` minting (a pure function of
trade/leg/attempt), every leg's intent fsynced to the write-ahead journal before
any submission, `ApplyPlanned`'s first caller, the OCC version threaded into
`Dispatch`, and `Deps.Journal` converted from a port field to a **path** — the
conversion `module.go` predicted and `singlewriter` forced, since with the
adapter inside a policed package the only way a root could still fill a
`ports.Journal` field was by writing a second write path.

**THE DECISION THE SLICE TURNS ON IS `Dispatch`'s SIGNATURE.** It is now
`Dispatch(ids.ClientOrderID, int64)` and it **reads the order from the write
repository**. Two holes close that no check could have closed: an order that was
never planned cannot be dispatched, and an order mutated between planning and
dispatch cannot be submitted in place of the one that was journaled. §6 step 5's
ordering becomes a property of the type system. **One consequence points the
other way and is the transferable half:** `admit`'s `domain.Order.Validate` had
to be **removed**, because with the order arriving from a repository whose
`ApplyPlanned` refuses a malformed one it became a branch no test could reach —
an unreachable sentinel reading as a handled case. Its removal is sound only
because the refusal is now stated as a **port contract** rather than assumed from
one adapter.

**`TradePlanned` is deliberately NOT written, and the argument is the part worth
carrying.** §6 step 5 named *"`TradePlanned` + intents"*. The easy reason is
insufficient (§5.1's `Trade` aggregate has no domain type). What makes the
omission *safe* is **a different clause of the same sentence** — *before any
submission*: every leg is journaled before `Plan` returns anything dispatchable,
so the only state a partially-journaled plan can reach is one where nothing was
sent, which §5.3 prices as *"safe to replan or discard"*. The premise is code, so
it is a **run** rather than a paragraph — the test breaks the *last* leg's
journal write and requires the repository to be entirely empty. The change that
would break it is named in the spec: a planner that dispatches leg 0 before
journaling leg 1.

**THE FINDING: `FM-5`'s shape arrives in Go, at the projection layer.** A
journal-then-apply failure *after* the send leaves a sent order at `Planned` in
the current-state store — and `Planned` is the **one** non-terminal state
contributing no possible-delta interval. So a sent order reads to the §6.7
classifier as contributing nothing, turning an **ambiguous** residual into a
**determined** one and unblocking precisely the ≤5 s compensation the ambiguity
existed to hold. **The projection does not merely lag; it lags in the direction
that unblocks the held action.** It inherits the family's consequence unchanged —
**no invariant computed from the projection can detect it** (§46 §8e, one layer
out of TLA⁺). Landed as a refuse-to-trade clause in §6 step 7; **task #169**
carries the response to slice E/F. **It carried no marker and the grep could
never have found it** — nothing departs from a clause, the clause was absent.
The reading pass beside the grep is what found it.

**Two method notes, both cheap and both new.** (1) The collapse checklist found a
**format** collapse rather than a cardinality one: `ClientOrderID` renders leg and
attempt as **two decimal digits each**, so at attempt 0 a field-order bug mints
the identical id — the tell was the format, not the data. Every fixture now uses
attempt 7. (2) **All four `NOBUILD!!` this slice were bad seds**, and the
discriminator is now stated generally: *"does not implement"* is §3.6 rule 2's
assertion doing its job (a kill); ***"declared and not used"* is Go reporting
that the mutation deleted a value's only reader** (re-aim, keeping the variable
read).

**§47f's gate re-verification is DONE by running, not reading.** `make lint` is
clean with `module.go`'s new `driven` import, and the gate was shown to still
bite on CLAUDE.md's **isolating** probe — `internal/marketdata` (a context root,
where `hexboundary` is silent) importing `oms/application/ports`. Probe applied
and removed. **No new §3.6 gate is due**, checked rather than inherited.

## START HERE (session 75 — HISTORICAL, superseded by session 76's above; its step 1 is DONE for slice C1)

**Branch `feat/milestone-1-foundation`. Docs are v0.9.30.**

**First act, in order:**

1. **Slice C — `engine/venue`'s never-delivered account/order half (task #162).**
   §47e finding 1: the package holds books, funding, sources and both venues'
   wire decoders, and there is **no signed REST client, no `Place`, no
   `OrderByClientID`, no `Fills`**. `ports.OrderPlacer` has had no adapter for
   the whole of milestone 3 so far — slice B's `module_test.go` still fakes it.
   **Testnet only**; venue facts are normative and already recorded (profiles
   §1/§4/§7), so nothing unverified enters. Attach to §7.6's priority ladder via
   `platform/ratelimit`, which is built.
2. **§47f's standing hazard for slice C applies literally:** any test reading a
   channel a mutation could stop feeding needs **its own bound**, or a mutant
   that blocks reports `TIMEOUT!!` and names no defect. And **a fix applied in
   one place gets applied to its mirror** — slice C builds *two* venues' clients,
   so whatever repetition, bound or isolation trick one earns, the other gets the
   same session.
3. **§38f is STILL a live user decision (task #158)** — untouched by slices A and
   B, and still the only clock-bound item. ≥2 TB recommended; 85 days to a full
   disk, gate at day 59 (~2026-09-30).
4. **Tasks #152, #153, #159, #167, #168 are unchanged by this slice.** New this
   session: **#169** (act on `Outcome.Applied`, slice E/F).

**Standing rituals, unchanged.** `pgrep -f "arblab capture run"` **FIRST**
(PID 8495, config `1c362feffe4a`, 951 GiB free); **never rebuild over
`bin/arblab` while capture runs** — `make lint` builds `bin/arblab-**lint**`, a
different binary, which is what let slices A and B run the gates safely; cap
`TLC_WORKERS` (4); marker grep `grep -rn "SPEC EDIT OWED" internal/ tools/` must
be **zero**; never edit a `.tla` or `run.sh` mid-run; read figures only from a
**completed** invocation.

**Doc-set landing is COMPLETE, including the Georgian.** All seven docs are
stamped v0.9.30. `docs/tutorial.ka.md` was regenerated with **`node
docs/i18n/tools/assemble.mjs`** after editing `docs/i18n/ka/tutorial/header.md`
**and `s-3-9.md`** — source first, then assemble, which produced exactly the
expected two-line diff. The §3.9 crash-walkthrough's t₀ row named `TradePlanned`
and the §6 step 5 edit invalidated it, so both language sources moved together.
The tool lives under `docs/i18n/tools/`, **not** `tools/`, and **no `make` target
invokes it**; task **#167** is still that gap.

## Status (2026-08-02, session 74 — HISTORICAL): **MILESTONE 3 SLICE A IS LANDED — `ports.Journal` HAS AN ADAPTER, AND THE STORAGE ENGINE WAS DECIDED BY A REQUIREMENT THAT BINDS NONE OF ITS OWN METHODS. DOCS ARE v0.9.29.**

**Code changed.** `internal/oms/infrastructure/driven/`: `journal.go`,
`journalcodec.go`, `journalerrors.go`, `journal_internal_test.go`. Design record
is **§48**
(`docs/superpowers/specs/2026-08-02-m3-slice-a-durable-journal.md`); core §12
milestone 3 and §13 gain dated annotations; ADR §3 has the dated block.
**`make ci` green, both modules; marker grep zero; 12 mutations, 12 kills.**

**§47e finding 2 is CLOSED.** §5.3's fsync-before-network crash boundary is no
longer proven only against a test fake. The adapter is append-only,
hash-chained, one fsync per record, on SQLite in WAL mode at
`synchronous=FULL` — **verified at open rather than assumed**, because
`synchronous` is a per-CONNECTION setting and `database/sql` owns a pool, so a
pragma issued as a statement configures whichever connection served it and the
next one gets the default.

**THE FINDING: the store was chosen by §5.3, not by §13 — and it is invisible
from inside slice A.** Neither `AppendPlanned` nor `AppendState` needs a
transaction, so a bespoke hash-chained log file is the smaller implementation
and the one a reader reaches for. What forbids it is §5.3's rule that the fill
apply, the REST cursor advance and the journal/outbox append share **one
transaction**, plus `driven/doc.go`'s already-recorded consequence that *a
transaction spanning two stores cannot be added to one of them alone*. The log
file gets rewritten at slice D/F or the rule goes unmet. **A component's
storage engine can be decided by a constraint that binds none of its own
methods.**

**TWO MORE FINDINGS, both about evidence rather than code.** (1) **Session 73's
predicted design fork was mis-premised** — it named reuse of `engine/capture`'s
codec, *"already fuzzed, already has a hash chain"*, and `engine/capture` has
**no hash chain**: per-record CRC32C, a dense *sequence* chain, whole-file
`.sha256` sidecars. (2) **The pragma check hid a self-consistent mutation** —
`journalDSN` and `verifyPragmas` deliberately share `wantSynchronous` so they
cannot drift, which means mutating the constant makes the request and the check
**agree**, and every *Open succeeds* test passes against a journal that no
longer fsyncs. Capture's wrong-CRC-polynomial note arriving as a configuration
constant; the fix is the same — the required values are stated as **literals**
in the test. **When a value is deliberately shared between the code that SETS a
thing and the code that CHECKS it, the check cannot also be the test.**

**§47f's collapse warning stopped ONE SHORT and this is the reusable part.** It
named a chain of length 1. At length **2**, "link to the previous record" and
"link to the first record" are *still* the same predicate — record 1 is both.
**Three** is the shortest chain that separates them, and the inequality is
asserted rather than implied.

**THREE THINGS SLICE A DELIBERATELY DID NOT DO — read these before assuming.**
(1) **`Deps.Journal` is still a port field.** `module.go` predicts its
conversion to configuration and the prediction is now *live* (a root can fill it
only by writing its own journal outside `oms`), but §47f assigns that to slice B
and no binary wires `oms` at all today. (2) **Chain VERIFICATION is not
implemented** — only construction; the sole replayer is the test oracle, and a
production verifier belongs with §7.5. (3) **§13's third durability
precondition is not checked** — local filesystem only, never NFS. It carries
**no `SPEC EDIT OWED`**: nothing departs from the spec, the requirement is
scheduled to §7.5's refuse-to-trade posture list (**task #168**), which is the
§47e shape caught on the way in rather than two milestones later.

**The engine module gained its second third-party runtime dependency and the
cost is worse than the first.** `modernc.org/sqlite`, cgo-free (a cgo driver
makes the OMS the one component of a Linux-target deployment that cannot be
cross-built) — **+9 modules**, against `coder/websocket`'s one node with no
transitive edges. No document named a driver; §13 now does.

## START HERE (session 74 — HISTORICAL, superseded by session 75's above; its steps 1–3 are DONE — §48g item 1 read, slice B landed, the gate re-verified by running the isolating probe)

**Branch `feat/milestone-1-foundation`. Docs are v0.9.29.**

**First act, in order:**

1. **Read §48g before slice B**, specifically item 1. Slice B owns the
   `Deps.Journal ports.Journal` → configuration conversion, and the reason it is
   slice B's rather than slice A's is recorded there — including the cost, which
   is that `module_test.go`'s one distinctive claim currently rests on injecting
   a fake journal at that seam. Decide what replaces it before editing `Deps`.
2. **Slice B — the planning use case, §6 step 5 (task #161).** Deterministic
   `ClientOrderID` minting, `TradePlanned`, `ApplyPlanned`'s first caller, OCC
   version threaded into `Dispatch`. It is what gives the wiring something to be
   for, and it closes `module.go`'s stated hole. `ids` already has its base32
   oracle for the determinism claim.
3. **Re-verify the gate check at slice B rather than inheriting it.** §47f says
   no new §3.6 gate is due because `singlewriter` polices by position — slice A
   confirmed that by RUNNING the gate, not by reading the analyzer. Do the same:
   rule 10's deferral named the wrong construct and cost 13 silent fallthroughs.
4. **§38f is STILL a live user decision (task #158)** — untouched by slice A and
   still the only clock-bound item. ≥2 TB recommended; 85 days to a full disk,
   gate at day 59.
5. **Tasks #152, #153, #159, #167 and the new #168 are unchanged by this slice.**

**Standing rituals, unchanged — and one now bites.** `pgrep -f "arblab capture
run"` **FIRST** (PID 8495, config `1c362feffe4a`); **never rebuild over
`bin/arblab` while capture runs** — slice A did not, because `make lint` builds
`bin/arblab-**lint**`, a different binary, which is worth knowing before the
first slice that builds `bin/arblab` itself; cap `TLC_WORKERS` (4); marker grep
`grep -rn "SPEC EDIT OWED" internal/ tools/` must be **zero**; never edit a
`.tla` or `run.sh` mid-run; read figures only from a **completed** invocation.

**Doc-set landing is COMPLETE, including the Georgian.** All seven docs are
stamped v0.9.29; `docs/tutorial.ka.md` was regenerated with **`node
docs/i18n/tools/assemble.mjs`** after syncing `docs/i18n/ka/tutorial/header.md`
— source first, then assemble, which produced the expected one-line diff. The
tool lives under `docs/i18n/tools/`, **not** `tools/`, and **no `make` target
invokes it**; task **#167** is still that gap.

## Status (2026-08-02, session 73 — HISTORICAL): **MILESTONE 3 IS ADMITTED BEFORE STRATEGY #2's GATE — THE ROSTER IS UNBLOCKED FOR ~2 MONTHS OF FOUNDATION WORK, AND THE GROUND FOR IT WAS ALREADY IN THE RECORD THAT REJECTED IT. DOCS ARE v0.9.28.**

**No code changed. This was a ruling session.** The ruling and the whole doc set
landed in **`f92fbff`**; a follow-up commit adds this SHA record itself, which
is why the count is two and not one. Record is **§47**
(`docs/superpowers/specs/2026-08-02-milestone-3-pre-gate-ruling.md`); core §12
gains **three** dated annotations; ADR §3 has the dated block. `make ci` was
**not** run and does not need to be — the change is docs-only, and the gate
that applies to it is the marker grep, which is **zero**.

**THE FINDING: §42d's rejected alternative (i) is refuted by §42d's rejected
alternative (ii), one sentence below it.** (i) rejected *"proceed to milestone 3
now"* because the OMS would be **"built 'on spec' for a strategy whose economics
are unmeasured"**. (ii) rejected a roster rewrite because **"milestones 3–6 are
strategy-agnostic controls and survive unchanged"** — and (ii) is the
load-bearing claim of the whole post-kill re-sequencing, the reason the roster
was annotated rather than rewritten. If they survive a *complete strategy
replacement*, a gate verdict cannot invalidate milestone 3. **Neither rejection
had ever been read against the other.** The core sides with (ii) twice — §3.1
(*same pipeline **regardless of strategy***) and §1.1 (the single-writer OMS is
***"foundational, not tiered"***) — and **the practice had already departed from
the rule**: formal model (a) IS milestone 3's named safety kernel, and sessions
68–71 built it pre-gate. The operative rule was never "no milestone-3 work"; it
was "no milestone-3 work **in Go**", which no record defends.

**The partition is by INVALIDATION, not by milestone number — so milestone 4
splits.** Admitted: **milestone 3 in full**, plus milestone 4's
strategy-agnostic half (`Fence`, `FenceReady`, kill-switch ladder, `audit` —
admitted, *not scheduled*; §12's canary-concurrent language still governs it).
Still hard-gated: **milestone 4's leg-shape-dependent half** (§1.1's $1K tier
names *"the Binance perp and both spot IOC legs"* — **cash-and-carry's** shape,
not perp-perp), and **milestones 5, 6, 7**. **The gate's kill authority over
live capital is UNTOUCHED — construction order moves, promotion order does
not.** No live capital, no mainnet keys, no promotion.

**The residual risk is project mortality, not rework, and it is priced (§47d),
not argued away.** If strategy #2's gate also fires the project stops — base
rate **1 fired of 1 run**. Against it: nothing built is strategy-specific, so a
*third* strategy inherits all of it; the work converts `FM-1`…`FM-9` from prose
into checked claims; and the wait is not free at 11.165 GiB/day with the gate at
day 59 of an 85-day disk, on the roster's **longest-lead item**.

**TWO SCOPE FINDINGS (§47e), both of which change milestone 3's real size and
neither visible from §12's text.** (1) **Milestone 1's `engine/venue`
account/order half was NEVER DELIVERED** — the package is market data only; no
signed REST client, no `Place`, no `OrderByClientID`, no `Fills`. §12 records
milestone 1 complete. This is a normative claim the code silently does not
satisfy, and the `SPEC EDIT OWED` grep is **structurally blind** to an unmarked
one. (2) **`ports.Journal` HAS NO ADAPTER** — so §5.3's fsync-before-network
crash boundary, the most safety-critical ordering in the OMS, **is today proven
only against a test fake**. `module.go` is honest about the write repository
being unwired; the journal's absence is the same shape one layer down and was
stated nowhere.

## START HERE (session 73 — HISTORICAL, superseded by session 74's above; its step 2 is DONE)

**Branch `feat/milestone-1-foundation`. Docs are v0.9.28.**

**First act, in order:**

1. **Read §47 before writing code** — specifically §47f (the seven-slice plan)
   and §47c (the boundary). The boundary is the part a later session can
   silently widen: *admitted* is not *scheduled*, and milestone 4's
   strategy-agnostic half is admitted but out of this workstream.
2. **Slice A — the durable hash-chained journal (task #160).** It is first by
   dependency, not preference: every recovery claim in milestone 3 reads what
   it wrote, so building recovery on the test fake would prove the recovery
   logic and nothing about the durability under it. Oracle discipline applies —
   an **independent replayer**, per `fixedpoint`/`ids`. **Collapse checklist
   BEFORE the mutations:** a chain of length 1 makes "verify against the
   previous record" and "verify against the first" the same predicate.
3. **Slice E is the payoff and it lands fifth, not first.** Its three inputs
   are produced by A–D; §47g (v) records why building it against fakes was
   rejected. **Read `docs/formal/ambiguous-order-model.md` §3's correspondence
   map before the `.tla` and before the Go.** §46/§8e's rule comes with it: an
   invariant built on the ledger cannot audit the ledger — the hazard transfers
   to the Go unchanged.
4. **§38f is STILL a live user decision (task #158)** — untouched by this
   ruling and now the only clock-bound item. ≥2 TB recommended; 85 days to a
   full disk, gate at day 59.
5. **Tasks #152 (gate evidence, ~2026-09-30), #153 (the hourly
   `restart_boundary`, 24 of 50 findings) and #159 (the `depth20` timeout,
   trigger-bound to the next restart) are unchanged by this ruling.**

**Standing rituals, unchanged:** `pgrep -f "arblab capture run"` **FIRST**
(PID 8495, config `1c362feffe4a`); **never rebuild over `bin/arblab` while
capture runs** — this now bites harder, because milestone 3 is the first
workstream in months that builds binaries routinely; cap `TLC_WORKERS` (4);
marker grep `grep -rn "SPEC EDIT OWED" internal/ tools/` must be **zero**;
never edit a `.tla` or `run.sh` mid-run; a full-day verify costs ~27 minutes at
99% of one core — not beside TLC, and read figures only from a **completed**
invocation.

**Doc-set landing is COMPLETE, including the Georgian.** All seven docs are
stamped v0.9.28. The generated `docs/tutorial.ka.md` was assembled with
**`node docs/i18n/tools/assemble.mjs`** after syncing its source fragment
`docs/i18n/ka/tutorial/header.md` — source first, then assemble, which is
session 63's recorded order and produced the expected one-line diff.
**Carry the tool's location, because it is not where you will look for it:**
it lives under `docs/i18n/tools/`, **not** in `tools/`, and **no `make` target
invokes it** — searching `Makefile` and `tools/` finds nothing and reads as
"the generator does not exist", which is how a forbidden hand-edit gets
rationalized. Task **#167** is the remaining half: give it a `make` target so
the next bump cannot strand it.

## Status (2026-08-02, session 72 — HISTORICAL): **THE §32m-D PER-TIER DISK FIGURE IS MEASURED — TASK #150 IS CLOSED THREE HOURS INSIDE ITS DEADLINE, AND §38f's OFF-HOST DECISION IS BACK WITH THE USER ON A MEASURED CLOCK. DOCS ARE v0.9.27.**

**The figure: 11.165 GiB for the measured day — BTC tier 4.314, alt tier 6.852,
per shortlisted symbol 0.387 (MUSDT) to 1.743 (BEATUSDT).** Record is
**capture-expansion §44e**; the profiles gained a 2026-08-02 dated block; core
§12's milestone 2b step 2 carries the landed-figure annotation; ADR §3 has the
dated block. No code changed — this was a measurement session.

**The measured day is 2026-08-01T12:00Z → 2026-08-02T12:00Z, 24 whole hours,
deliberately NOT the 11:13Z restart anniversary.** That hour holds ~11 minutes
of the *previous* process for the eight BTC sources and only ~47 minutes of life
for the 32 new alt ones, so a tier split over it is biased by construction in
the direction that flatters the tier under test. Sized rather than waved at:
~1%, so it would not have changed a conclusion — the window is still chosen so
both tiers are measured over identical, complete, single-process hours.

**Both verifies are clean in the sense that matters.** The full-day run is
**0 integrity / 50 coverage, verdict COVERAGE**, and **all 960 per-hour source
lines (40 × 24) read `alive 60/60`** — a fully covered day, unlike the
2026-07-31 figure. The restart-hour run reports the restart exactly as §44d
promised: **8 `process_restart` gap records** (the pre-existing BTC sources) and
**32 `alive_short`** (the new alt sources at 47/60). Of the 50 findings, **24
are the `fundingInfo` hourly `restart_boundary`** — task #153's marker is
**48% of a clean day's findings**, which is a measured input to that ruling.

**THE FINDING: §42c's aggregate was right and its premise was false.** The
derived ~0.6–1.1 G/day/symbol rested on "the two fixed-rate Binance streams
contribute ~450M/day regardless of symbol activity". The eight-symbol total
landed inside the derived band and the mean inside the per-symbol one — **and no
symbol-level claim survived**: MUSDT is 35% below the floor, BEAT is 1.58× the
ceiling, and BEAT's Bybit book (959.5 MiB) *exceeds BTC's* (678.9). Cause
measured, not inferred, by splitting bytes/day into **records/day ×
bytes/record**: bytes/record is constant across the nine perp streams
(966–1132 B for a 20-level book) and the **rate** over those same nine runs
2.68/s to 8.99/s against a 10/s ceiling (9.98/s on BTC spot, the closest
anything comes to saturating it). **A venue's
documented "update speed" is a MAXIMUM, not a cadence** — both venues' book
streams push only on change. Ruled out as local loss, which is the claim that
matters since both produce the same byte count: **`pu`-chain breaks zero**,
minimum inter-push **102 ms** (the cap, exactly), maximum silence **22.5 s** in the hour checked, **94.8 s** over the day.

**The refuted premise also underwrites an operational setting, and that is the
part with a to-do.** §44b copied BTC's 10 s `first_data_timeout` to all 32 new
sources *because* depth20 was believed fixed-rate. Binance partial depth has no
subscribe-time snapshot (Bybit does, on both topics), and MUSDT's Binance book
is silent >10 s for **3.42% of the day, max 94.8 s** → **~1 reconnect in 100
would trip its own first-data deadline** on a perfectly healthy stream. It
self-recovers via the backoff ladder and has not bitten (the day's one
`no_first_data` is on BTC during the blip cluster). **Recorded, not fixed:**
the setting is restart-required, and a restart is an operational act — raise
the nine Binance `depth20` sources to 30 s **at the next restart that happens
for another reason**.

**`markPrice@1s` is the one genuinely fixed-cadence stream (exactly 86,400
records/day on 7 of 9 symbols), and that makes it an INSTRUMENT.** It prices
the two reconnect hours at **103 s and 48 s of lost data inside hours scoring
`alive 60/60`** (not a verify defect — `alive` counts minutes touched — but a
limit §45's decision-grid joins must know); read backwards it says the
2026-07-31 day was **~61% covered**, where the scale-up that produced
3.6–3.8 G/day assumed ~1.2×; and it caught a defect in **this session's own
reading tooling**, which silently dropped `gapped` hour lines and would have
published a false 91.67% coverage figure. **A one-number scale-up cannot
separate coverage from activity.**

**§38f, re-argued on the measured clock (ruling §42f-3's named revisit):**
954 GiB free, 11.165 GiB/day → **85 days to zero free, 76 to a 100 GiB floor**,
and the gate the schedule waits on (BCHUSDT, ~2026-09-30) is **at day 59**, by
which point **~678 GiB of un-backfillable archive sits single-copy**. **Books
are 93.6% of the daily bytes.** §38f's premium estimate ("~6 GB/day") was half
the real rate — the rsync line is unchanged, the sizing is not: **≥2 TB**, not
512 GB. **Recommendation on the table for the user: take the target now rather
than at a third deferral.** Compression and retention stay refused on §38g's
original grounds.

## START HERE (session 72, 2026-08-02 — HISTORICAL, superseded by session 73's above; task #150 CLOSED)

**Branch `feat/milestone-1-foundation`. Docs are v0.9.27.**

**First act, in order:**

1. **§38f's destination is a live user decision (task #158)** — §44e states the
   measured case and recommends a **≥2 TB** target now. It has been deferred
   twice, both times against derived numbers; this is the first time it is
   being put with a measured one. Nothing else is blocked on it, and everything
   is at risk from it. If it is taken, the remaining work is one rsync line and
   one verify run, both already proven against §38's second-root fixture.
2. **Task #153 — the hourly `restart_boundary` on `fundingInfo`** is now cheap
   to rule on and has a measured weight (24 of 50 findings on a clean day,
   multiplying with every REST-overlay source added). The three options and the
   two-directions argument are recorded in session 64's block below; nothing
   needs re-deriving.
3. **Task #152 waits on the clock, not on work** — gate evidence accumulates
   until BCHUSDT's 180 matched post-cutoff settlements (~2026-09-30), then
   `arblab crossvenue gate`. Periodic `crossvenue fetch` keeps it fed.
4. **Task #159 is a CARRIED TODO with a trigger, not a due date** — raise the
   nine Binance `depth20` sources' `first_data_timeout` 10 s → 30 s **at the
   next restart that happens for another reason**, never for this alone.
   Reasoning and the measured exposure are in §44e; the config change is two
   minutes, the restart is the expensive part.
5. **Formal model (a) is at a clean stopping point** (slice F closed, suite
   75/75 from one unfiltered invocation). `FM-8`'s candidate 2 remains
   unrefuted and unadopted — see session 71's block.

**Standing rituals, unchanged:** `pgrep -f "arblab capture run"` **FIRST**
(PID 8495, config `1c362feffe4a`); **never rebuild over `bin/arblab` while
capture runs**; cap `TLC_WORKERS` (4) beside it; marker grep
`grep -rn "SPEC EDIT OWED" internal/ tools/` must be **zero**; never edit a
`.tla` or `run.sh` mid-run. **New from this session:** a full-day verify over 40
sources costs ~27 minutes at 99% of one core — do not run it beside TLC, and
read figures only from a completed invocation.

## Status (2026-08-02, session 71 — HISTORICAL): **SLICE F IS BUILT — `FM-8`'s RULING IS ENCODED AND CHECKED, THE HOLE IS SHOWN TO CLOSE, AND ENCODING IT FOUND `FM-9`. DOCS ARE v0.9.26.**

**The one thing slice E left open is closed, and there is now a run that says
so.** `NC21` — the **pre-ruling world**, §7's precondition as an entry guard
alone, which is exactly how slices A–E modeled it — **violates `INV9`**, the
new standing obligation. The constant *is* the ruling and nothing else differs
from nominal, so the control isolates it. `PC29` shows the new third
supersession row **actually fires** in the nominal model, without which a green
`INV9` would be equally consistent with a rule nothing exercises. `W5` shows
the pre-ruling harm reaches a **nonzero spot sale** on the dissolved premise —
`NC21`'s own witness only reached `PdSubmitPerp`, the cheapest advance.

**`FM-9` — and it is a defect in the clause that landed LAST SESSION.** SM §7's
new row stated its trigger as *"the §2 classifier leaves `Determined` for this
hedge group"*. **The generation's own perp reduction is an exposure-changing
order on that group**, so submitting it drives the classifier out of
`Determined` by itself — and **§7 depends on that**, because it is how §2
refuses every compensating trade while the leg is live. Read literally, the row
fires on the generation's own machinery. Control **`NC22`** encodes **the
spec's own sentence** — the first negative control in either model aimed at
one — and reaches the abort **4 states from `Init` with no external event at
all**: `realized = 0`, nothing outstanding, ledger empty, a generation opened
on a **physically flat** package and cancelled one step later by itself. A
spurious abort is not bookkeeping: §7's generations exist because the perp
short is nearing liquidation, so cancelling one cancels the mechanism that was
reducing the risk, with a reserved leg still out.

**The fix, landed v0.9.26: the trigger is the entry test re-evaluated NET OF
THE GENERATION'S OWN LEGS** — its unsold transient out of the realized delta,
its own ledger entry out of `outstanding_reserved_reductions`, both sides of
the same subtraction. **The RULING IS UNTOUCHED**: standing-vs-entry was not
reopened, and what the model supplied is the quantity that makes it coherent.

**`FM-9` is the third finding in one family, and naming the family is the
point.** `FM-5`: an entry read **before its backing exists**. `FM-8`: an
entry's **premise outliving it**. `FM-9`: a rule evaluated on a quantity that
**includes its own subject**. All three are §2 and §7 disagreeing about what
the shared ledger means; none was visible to any invariant phrased over the
ledger, for §8e's reason — *an invariant built on the ledger cannot audit the
ledger*.

**`P6` is the transferable move: the fix's own justification is now a run.**
`FM-9`'s resolution rests on one claim — the netted premise is a quantity **the
generation cannot move**. That was about to live as prose in three documents,
and `FM-7` is this repo's evidence for what that costs (a justification nobody
re-derives ages into a fact, and a reader who checks it and finds it false
retires a rule doing real work). It is now the action property
`[][PdOwnStep ⇒ PremiseResidual' = PremiseResidual]_vars`, **PASSING over the
complete graph** (7,923,475 generated / 976,541 distinct / **0 left on
queue**). It turns out to be a conservation law: every ledger change the
generation makes is to **its own** entry and every realized change is **its
own** transient, so both subtractions cancel — `FM-5`'s fix showing up as
arithmetic.

**Three process facts this slice paid for.**
**(1) The witness-degeneracy question needed asking TWICE.** `W5`'s first
version was violated by a spot authorization of size **ZERO** (the perp leg had
confirmed nothing, so `SpotCap` was 0) — correctly flagged by `INV9`, moving no
money. That is **`W1`'s own finding** — an orphaned entry of amount zero leaks
a slot and subtracts nothing — landing on the probe written to avoid that exact
class, one slice later. The quantity had to go **into the flag**.
**(2) `UNCHANGED` over a bare variable name failed three times in one
session** — `RecordDispatch`, then `Settled` (via `vars`), then `Supersede`.
The module's own comment already said the two-group split exists for this
reason; the groups did not extend themselves to new members. Fixed by making
the new flags a **group** (`advHist`).
**(3) The regression evidence is `SM2`, not the nominal run.** Slice F adds an
action and three variables, so the composed graph necessarily grows;
`AmbiguousOrder_SM2` (`MAX_GEN = 0`) has no generation, so its graph must be
**unchanged** — and it reproduces slice A's **25,514 / 5,169** to the digit.

**THE SUITE IS 75 RUNS AND `ALL 75 RUNS MATCH PREDICTIONS` FROM A SINGLE
UNFILTERED INVOCATION** (2026-08-02, `TLC_WORKERS=4`). That also discharges the
debt session 70 left: its 69/69 was assembled from two invocations and was
recorded as such. Nominal graph **8,066,455 generated / 976,541 distinct**,
depth 29, **0 left on queue**.

**The check that mattered most, and it is not the nominal:** all **40** probes
are still REACHED (29 positive controls + 5 model-(a) witness probes + model
(b)'s 6). The standing guard blocks the generation's decisions while the premise
is dissolved, so it *could* have made previously-reachable states unreachable —
buying safety by quietly deleting coverage, which a green nominal cannot
distinguish from a working fix. It did not.

**`NC15`'s identical-graph claim survives slice F**, re-measured: 7,891,109
generated / **976,541 distinct** against the nominal's 8,066,455 / **976,541** —
the generated counts differ, the distinct counts match to the digit, and the
distinct count is what the argument is about. (A stale `.out` from the previous
suite made this look broken mid-session; the liveness controls run *after*
`Live` in the runner, so their files still held session-70 figures. **Read
figures only from a completed invocation.**)

**`NC20`'s figures moved** (8,948,671 / 1,093,953 against v0.9.25's 8,770,801 /
1,086,199) because slice F changed the state graph. `FM-7`'s conclusion is
re-established, not weakened; v0.9.25's numbers are left as measured then rather
than rewritten, because they record what that release checked.

Capture **PID 8495** healthy throughout; `TLC_WORKERS=4` beside it; marker grep
**ZERO**; `bun test` **16/16** after the tutorial header bump was re-assembled
(the v0.9.24 i18n trap, checked this time rather than discovered).

### The two boundaries of the standing obligation, both drawn by FAILED runs

**Neither was designed; the nominal liveness config found both, and the second
refuted a considered design note written earlier the same session.**

**(1) It does not cover sending an already-authorized leg.** The first encoding
guarded all three of the generation's decision steps. `AmbiguousOrder_Live`
stuttered at 16 states: a generation buys 1 perp, authorizes **zero** spot
(releasing its ledger entry, so the transient becomes a genuine naked
residual), §2 correctly compensates it, and the package ends **physically flat
and `Determined`** while the generation sits at `SpotReduceAuthorized` unable
to finish. Blocking a step that **commits nothing** buys no safety and costs
termination — worse, it *strands* the reserved leg, which is `INV6`'s harm one
step earlier.

**(2) The obligation must be DISCHARGED, not merely admissible.** The module
carried a reasoned comment saying the premise-lost supersession was
*"deliberately NOT in the fairness set"*. Re-running `Live` returned a 20-state
behaviour stuttering at `PerpReduceFinal`: a reduction's own ambiguous result
had released its entry (**`FM-8`'s exact scenario**), the premise was genuinely
dissolved, `PdAuthorizeSpot` was correctly blocked, the **dispatch budget was
spent** so §2 could not compensate either — and the one action that could end
it was available and never taken. **`P5` violated.** *A standing obligation
nothing is obliged to discharge is not a standing obligation.*

**Both are kept as RUNS, not as this paragraph.** `NC23`
(`PREMISE_SUPERSESSION_FAIR = FALSE`) removes exactly the one fairness
conjunct — same constants, **same 976,541-state graph**, only the behaviours
differ — and violates `P5`. Without it the refutation is a session anecdote and
a later reader tidying the fairness set finds a reasoned comment and no
objection: `FM-7`'s failure mode aimed at a mistake **this slice made itself**.
Same move as `P6`, one layer out.

**The analogy that produced the wrong note, named so it is not repeated:**
`EmergencyClosePackage` and `FenceReadyLost` are **external arbitration
events** — nothing here decides when they fire, so leaving them unfair is
honest. The premise-lost row is a **machine obligation triggered by state**,
which is exactly why it needed its own guard in `Next` while the other two are
enabled unconditionally. It belongs beside the machine's other protocol steps.
**Both boundaries are NORMATIVE and landed in SM §7**, because an implementer
would hit each one: refusing the authorized leg strands it, and "we check it on
the next event" is a real stuck state.

**What the fairness-set rule costs, stated honestly.** §9a's rule — *the
omissions are the theorem* — makes leaving an action out the safe default, and
it is right. But a **wrong** omission does not show up as a weaker property; it
shows up as an **outright failure**, and only if some property's subject can
reach the stalled state. Both of these were caught by the one run in the suite
whose job is to notice that the machine can still finish. No invariant, no
probe and none of the 29 positive controls would have found either.

**Still available, still not refuted: `FM-8`'s candidate 2** (tighten `PdPlan`
to physical `realized = 0`). It was not adopted because it does not close the
class on its own, which is a different claim from being wrong — and `FM-7`'s
shape (two independent mechanisms, either surviving a defect in the other)
argues for eventually having both. Slice F gives no new evidence either way.

## Status (2026-08-02, session 70 — HISTORICAL): **§46 SLICE E IS COMPLETE — ALL FIVE DOCKET ITEMS CLOSED, AND ITEMS 2/3/4 PRODUCED `FM-7` AND `FM-8` — BOTH NORMATIVE EDITS LANDED (`FM-8` BY USER RULING IN-SESSION; ITS MODEL ENCODING IS OWED, TASK #156). DOCS ARE v0.9.25.**

**`FM-8` was RULED by the user in-session and its normative edit LANDED; what
remains is the MODEL ENCODING (task #156, slice F, sketch in §46 §11f).**
The ruling is the **standing precondition**: "no naked residual" is a standing
condition and not only an entry test, so the §2 classifier leaving `Determined`
**supersedes** an active generation, quiescing its reserved second leg to §6.7
finality first — a **third** row beside `EmergencyClosePackage` and
`FenceReadyLost`, policed by the **same** quiesce invariant rather than a new
one. The rejected alternative (tighten `PdPlan` to physical `realized = 0`)
**moves the witness rather than removing the class**: a floor issued a step
after the generation opens dissolves the premise identically. It is on record
as an available hardening, not as a refuted idea. **The clause is normative
from v0.9.25 and NO RUN YET SHOWS THE HOLE CLOSES** — that split is marked
everywhere it appears, because a fix argued is not a fix checked.

**Item 2 — model (b)'s five controls, asked the witness question for the first
time.** Six probes (`FenceTakeover_W1`–`W6`), **all REACHED**: every control's
stated harm is real, and TLC had simply been reporting a shallower witness.
**The degeneracy was systematic rather than four accidents** — every invariant
in model (b) can be tripped by a cheap *surrogate* for exposure (an order
sitting live, unfilled) or by exposure itself, and the surrogate is always
exactly one `VEntryFill` closer to `Init`, so breadth-first search returns it
every time. `FM-1`'s full cost is `NC4`'s committed 11-step trace **plus one
fill**. `NC3` is the exception **and not by luck**: its harmful precondition
(`pos ≠ "flat"`) is a *guard of the action its constant unlocks*, which is the
general form — a control built that way is degenerate-proof in that dimension.

**Two suite defects came out of it, neither visible from a green run.**
**(1) `I2_HostIndependentCoverage` had never been violated by any run in the
suite.** It is listed in exactly two configs and both halt on a shallower `I3`
first, so the search ends before the `I2` state exists to be found; the
configs' own "I2 or I3" wording is what hid it. **A config listing two
invariants is checking whichever is shallower** — the one-claim-per-config rule
now has a second, independent reason beside attribution. **(2)
`fence-takeover-model.md`'s prose carried harm its runs never showed** (§1's
NC2 line, §2's FM-1 trace said "→ I3, then I2"; NC4 never violates I2). The
claims were **true about the system**, which is exactly why four review rounds
passed over them — a false claim gets challenged, a true one resting on a run
that did not show it reads as verified. Corrected in place, not quietly.

**Item 3 — `FM-7`, LANDED.** §7d's candidate finding was itself read off a
**truncated search**: `NC2` lists `TypeOK` and `INV2`, and `INV2` *is*
`floorCount <= 1`, so TLC halted at the first second-dispatch state and
`overshoot` was never a checked invariant in that run. `NC20` is the honest
experiment (NC2's constants, `INV2` removed, `INV1` alone) and it **PASSES over
the complete graph — 8,770,801 generated / 1,086,199 distinct / 0 left on
queue**. Non-vacuity cost no run, since `NC2`'s `INV2` violation already proves
double issuance is reached. **The rule is right and stays; its recorded
justification was the defect** — the `[0.4, 1.0]` counterexample assumes the
reservation is not counted while the adjacent sentence requires it to be. It is
now stated as deliberate defence in depth, with the parenthetical retained as
*what the rule protects against* (the `FM-5` world). Fixed rather than left
because **nobody re-derives a recorded justification**.

**Item 4 — `FM-8`, RULED IN-SESSION; encoding owed.** §7's "the package has no naked residual" is
enforced **only as an entry guard**, on a reservation-netted quantity. `PC27`
(3 steps) opens a generation on a package **physically short 2**, exactly
offset by somebody else's reserved reduction. `PC28` (4 steps) goes one further:
the offsetting reduction's own result comes back **ambiguous**, its ledger entry
is released **exactly as SM §2 row 4 mandates**, and a live `PackageDelever`
generation is left over a package that is physically short and **no longer even
`Determined`, with nothing reserved against it**. Not a model artifact — the
release is correct; what is missing is anything that tells the *generation*.
It is **`FM-5`'s mirror** (*counted before its backing exists* / *premise
outliving the entry*) and invisible to every invariant, since `INV5` polices the
generation's own legs and never its premise. §8f asked *which* neutrality; the
probes say the sharper question is **when it is checked**.

**Also repaired: v0.9.24 bumped `tutorial.md` and never re-ran the i18n
pipeline** — the ka header fragment was still at v0.9.23. That is the v0.9.6
§15c finding recurring, and `assemble.mjs` **refused outright** when run, which
is the mechanism working and nobody having run it. Header synced, re-assembled,
`bun test` **16 pass / 0 fail**.

**Suite is 69 runs, and every one of them has its predicted result in this
tree state — but the claim is ASSEMBLED FROM TWO INVOCATIONS and must be
quoted that way.** The unfiltered `./run.sh` reported **68 of 69**; the one
off-prediction, `FenceTakeover_Live2`, was then re-run alone and **PASSES**
(1,705,418 generated / 249,936 distinct, depth 34, 24m02s — the artifact's
recorded figures). **A single-invocation 69/69 has NOT been produced.** That
distinction is exactly what `run.sh`'s `FILTERED … NOT the full suite` line
exists to preserve, and it is cheap to close now: one unfiltered run on
cleared metadirs.

**Why `Live2` failed the first time, and why it was NOT a model regression:**
the JVM died with **SIGBUS in an mmap'd `arraycopy`** as the temporal phase
started, *after* the safety phase had reproduced the recorded state graph
**exactly**. `.tools/` had accumulated **12 GB over 114 `states-*`
directories** — TLC never reclaims a `-metadir`. Cleared (12 GB → 3.2 MB) and
the config passed. `run.sh`'s header now carries this as a standing
pre-flight step; it matters most beside the capture, where disk is the shared
resource that cannot be re-fetched.

**A process rule was broken this session and the breach is recorded rather
than absorbed:** `run.sh` was edited at 08:02Z *while the `Live2` re-run was
executing*, ~40 minutes after this same handoff gained the rule forbidding
it. It was benign — the run completed coherently and its trailing section
header printed the PRE-edit text, so bash had already buffered that region,
and `bash -n` plus the runner's own 1+68 count confirm the file afterwards.
Benign is not the same as correct: the edits were comments, and an edit to an
executable region can resume bash at a wrong byte offset.

Marker grep **ZERO**; capture **PID 8495** healthy; `make ci` green;
`bun test` 16/16.

**Two process facts worth carrying.** A running `run.sh` **re-reads the `.tla`
and the script itself on every config**, so neither may be edited mid-run — a
baseline run had to be stopped for this. And **a probe being violated says the
state is reachable; only the trace says it is the harm the control names** —
`W2`'s route was not the one predicted and `W6` exists only because `W4`'s
trace was read.

## Status (2026-08-02, session 69 — HISTORICAL): **SLICE E IS PART-DONE — `FM-5`'s NORMATIVE EDIT LANDED, AND BUILDING THE MISSING TRANSITION FOUND A SIXTH SPEC DEFECT, `FM-6`. DOCS ARE v0.9.24.**

**Docket items 1 and 5 are CLOSED; items 2, 3 and 4 remain OPEN** (§46 §11, and
its RESUME HERE). Marker grep **ZERO**; `make ci` green; capture **PID 8495**
healthy and the 16-hour ritual verify came back **COVERAGE / 0 integrity**.

**Item 1 — `FM-5`'s edit landed (§46 §11a).** The footprint is **wider than
§8d prescribed**, deliberately: §8d named two sites, but SM §7's `Planned` row
and its reserved-leg invariant both said "atomically reserved from creation"
with **no amount attached**, which is the exact underdetermination `FM-5`
exploits — landing only the two would have left the unsound reading literally
available in the doc. **Then the reading pass caught the fix over-generalizing,
which is the finding's own lesson recurring inside it:** the first wording said
an entry is "zero before its **counterpart leg** has moved anything", true of a
`PackageDelever` generation and **wrong of the other two producers** — an
in-flight `IntervalFloorReduction` *is* its own reduction, so its entry is its
full dispatched size, and zeroing it restores the very re-read the reservation
prevents. The general statement, which all three satisfy: **an entry is the
exposure change its reserving action has already COMMITTED and not yet
REALIZED**; the producers differ only in *when* commitment happens.

**Item 5 — the perp-side park, and it found `FM-6`.** Two things, in order.
**(a) §10a's stated REASON for the narrowing was wrong**, and the model says so
before any run: `PdPerpFill` fires while `pd = "PerpReduceInFlight"` and amends
the ledger entry upward, so §2's own `PartiallyFilled` case puts a park *after*
a partial fill — holding a **real net-long transient and a nonzero
reservation**. `pdDone = 0` describes the park taken before any fill and no
other. Probe **`PC26`** is violated, so the harmful shape is reachable. The
narrowing's conclusion may have been tolerable; **its recorded argument was
not, and a recorded justification is what a later session relies on instead of
re-deriving.** **(b) Building the park DEADLOCKED the nominal model**, and the
work was telling which of three things that was. HEAD's nominal is green at
**6,729,250 / 832,179** (the recorded figure, to the digit), and the model with
`PdPerpHolds` alone removed reproduces **the same figure** — so every other
slice-E edit is behaviour-neutral and the park is the cause. Making
`TerminalFlatten` actually flatten **did not** fix it, so it is not that stated
abstraction either.

**`FM-6` — a verdict goes stale because the LEDGER changed and no order did,
and the spec's own wording names it.** §2 Scope said *"classification is a
function of **two inputs** and re-runs when either changes"*. But the residual
it gates on is `realized_delta − outstanding_reserved_reductions` — **THREE**
inputs, and the third was absent from the re-run rule. The **subtrahend**
decides the verdict exactly as the minuend does, and every producer moves it on
events that are **not order events**. TLC's signature is a **DEAD END**, not a
violated invariant: every counter-leg interval collapsed, the verdict still
`DeadlineExceeded`, its one permitted action (the interval floor) spent by
`FLOOR_ONCE`, a **determined naked residual** and dispatch budget in hand — and
**nothing enabled**. **Normative edit LANDED in v0.9.24** (core §6 step 7, SM §2
Scope, a new SM §7 invariant): three inputs, re-run on any; the machine may not
wait for an order event when the **ledger alone** changed the residual — which
is the argument §2 **already makes** for the cancel-with-zero-executions case,
owed to the third input. Control **`NC19`**.

**`FM-6` is `FM-5`'s sibling** — *the ledger must be backed when read* versus
*a change to the ledger must be observed*; both are §2 trusting
`outstanding_reserved_reductions` as an input it never re-derives.
**And neither was findable before slice E, for a structural reason worth
keeping: every other exit from a generation runs through `PerpReduceFinal`,
which reclassifies — the perp-side park is the UNIQUE route that bypasses it.**
SM §7 has carried that transition row since v0.9 and the model did not. The
clause-by-clause map found the omission (§10a); **building it found the
defect** (§11b). The map's justification, paid twice.

**Also in item 5: model (b)'s `Cx8-2` was asked of model (a) for the first
time.** A second park raises "is this two states wearing one name?", which
model (b) already paid for — so `HeldPerp` is a **distinct state**, and `NC17`
(`HELD_EXIT_CAUSE_BLIND`) is the cause-blind exit made explicit, caught by
**`INV8`**. `INV8`'s phrasing was chosen for **isolation**: the tempting
phrasing over the *authorization* would also be violated by `NC9`, showing that
*something* objected rather than that *this rule* did. `P5` ships with `NC18`,
which is `NC15`'s constant with `P5` listed alone — a property whose *sibling*
fails is an argument, not evidence. **`P5`'s consequent carries a third
disjunct `P3`'s does not**, and TLC is what said so: supersession is
**admissible** from a perp-side park because §7's quiesce clause is scoped to
"its **reserved second leg**", which does not exist yet there. Recorded as an
observation, not a verified claim.

**`run.sh` gained a `deadlock` verdict**, because a control that *stranded* the
model and one that *caught* something are different claims — here the dead end
**is** the finding, so it is named. **`./run.sh AmbiguousOrder` — all 52
model-(a) runs MATCH PREDICTIONS**; nominal **967,576 distinct, depth 30**. The
suite is 60 with model (b), whose 8 runs were **not re-verified in this
session's final state** — no `FenceTakeover*` file changed, and `run.sh`'s own
edits are exercised by model (a)'s 52 (every verdict path including the new
`deadlock` one). **Re-run `./run.sh` unfiltered to restore the full-suite
claim** — a filtered run is not the suite result, and this note exists so the
next session does not quote one as the other.

**NEXT: slice E items 2, 3, 4** (§46 §11's RESUME HERE — model (b)'s five
controls have never been asked the witness-degeneracy question; §7d's "never
issued twice"; §8f's physical-neutrality precondition). **Then the §32m-D
per-tier figure, task #150, DUE 2026-08-03T11:13Z and measurable from
2026-08-02T11:13Z** — the tightest clock on the board. **Start at
`## START HERE` below.**

## Status (2026-08-02, session 68 — HISTORICAL): **FORMAL MODEL (a) SLICE D IS BUILT — `docs/formal/ambiguous-order-model.md` EXISTS AND ITS DONE-WHEN IS MET: ALL NINE CLAUSES OF SM §2's INVARIANTS AND ALL NINE OF SM §7's ARE EITHER MAPPED TO A MODEL CONSTRUCT OR LISTED UNMODELED WITH A REASON AND A NAMED OWNER.** Docs stay at v0.9.23 (marker grep **ZERO**); no model change, no re-run — slice D is documentation over slice C's committed results, so `run.sh` stays at 55 runs and every figure in the artifact is quoted from §46 §7b/§8b/§9b rather than re-measured. **`FM-5`'s normative edit is STILL OWED and is now item 1 of an assembled slice-E docket** (§46 §10's RESUME HERE). The build record is **§46 §10**. **THE MAP FOUND SOMETHING THE BUILD RECORD DID NOT, AND IT IS THE ARTIFACT'S OWN JUSTIFICATION: SM §7's transition row enters `Held/Alert` from `PerpReduceInFlight` OR `SpotReduceInFlight`, and the model has only `PdSpotHolds` — the perp leg cannot hold.** Slices B and C could not have noticed: no invariant and no property is *about* the set of ways a state is entered, so **nothing inside a model can report that a spec clause never entered it** — which is exactly why slice D's done-when is a totality condition over the SPEC's clause list rather than over the model's constructs. The narrowing is defensible (at the perp leg neither the unsold transient nor a nonzero reservation exists yet, so the perp-side park is the harmless instance) and is **recorded as a narrowing, carried to slice E**, not absorbed. **Two claims the artifact deliberately REFUSES to make**, both places a green model would otherwise read as evidence: "a pre-authorized panic-MARKET does not bypass the finality barrier" is listed UNMODELED *even though the model contains no bypass edge*, because the absence of a modeled bypass is not evidence that the implementation has none — a rule of the form "there is no other path" cannot be checked by a model containing only the paths its author wrote; and "never a weaker cancel-ack-only variant" is an abstraction boundary, since the four-step contract collapses to one barrier and the model can tell whether a barrier was *waited for* but not a correct barrier from a weak one. **Also: `CLAUDE.md` was stale** — it said `docs/formal/` holds *a* model of the fence/takeover composition which found **four** spec defects; it is two models and five, and the companion artifact is now named as the entry point rather than the `.tla`. **NEXT IS SLICE E**, the adversarial round, whose docket is assembled in §46 §10's RESUME HERE: (1) FM-5's normative edit; (2) model (b)'s five controls, never asked the witness-degeneracy question; (3) §7d's "never issued twice"; (4) §8f's physical-neutrality precondition (FM-5's mirror); (5) §10a's perp-side park. **Then the §32m-D per-tier figure, DUE 2026-08-03T11:13Z** (task #150) — the tightest clock on the board. Capture **PID 8495** healthy. **Start at `## START HERE` below.**

## Status (2026-08-02, session 67 — HISTORICAL): **FORMAL MODEL (a) SLICE C IS BUILT AND VERIFIED — `P1`–`P4` ARE MODEL-CHECKED UNDER AN ARGUED FAIRNESS SET, AND ALL FOUR NEGATIVE CONTROLS BITE.** Docs stay at v0.9.23 (marker grep **ZERO**); no new spec defect, and **`FM-5`'s normative edit is still OWED at slice E** (wording in §46 §8d). The build record is **§46 §9** in `docs/superpowers/specs/2026-08-01-formal-model-a-ambiguous-order-design.md` — read it, then its RESUME HERE; **the next act is slice D**, the companion artifact `docs/formal/ambiguous-order-model.md`. **`run.sh` is now 55 runs (8 model (b) + 47 model (a))**: the nominal liveness config green on the same 6,729,250 / **832,179 distinct** graph at depth 30, **four** new negative controls each violating their predicted TEMPORAL property, **three** new positive controls for the liveness antecedents, and **four CONTROL-WITNESS probes** (`W1`–`W4`). **THE HEADLINE IS `NC15`: `HELD_RESUMES = FALSE` parks a `PackageDelever` generation in `Held` for ever — holding an unsold net-long transient and a reservation that subtracts from every later sizing — and its state graph is IDENTICAL to the nominal one, 832,179 distinct states to the digit.** Every invariant in the module is a state predicate, so every one of them stays green on it: no safety property could have caught it, and that is arithmetic about the state set rather than a claim about which invariants were chosen. **`NC13` is the second lesson: it doubles as the control on the FAIRNESS SET's own omissions** — `DeadlineBlows` is enabled in every `Ambiguous` state and its target satisfies `P1`'s consequent, so a fair `DeadlineBlows` would have discharged `P1` by assuming the degradation `P1` exists to survive. **§46 §8g's budget trap was answered by the properties' SUBJECT, not by an antecedent**: none of `P1`–`P4` asserts that a dispatch ever happens (both compensating-trade actions are deliberately unfair), so budget exhaustion falsifies nothing — the property that *would* have needed conditioning is "the residual is eventually flat", and it is not stated because it would be a claim about the model's bound rather than the spec. **`NC16` needed a STATE where slices A/B had an EDGE**: a blind retry that changes nothing observable is a STUTTERING step and weak fairness steps over stutter, so `pending[k].amb` is what makes the retry loop a fair lasso instead of a fill the fairness would have forced. **And the `W*` probes exist because two of the four witnesses were degenerate** — `NC13`'s and `NC16`'s both land at `budget = 0`, where the machine could not have compensated anything anyway, so the trace showed the stall and not its cost; `W2` reaches the harmful stall **two steps from `Init`** with the FULL budget intact (`realized = −1`, every interval collapsed, `cls = "Ambiguous"` for ever). `W1` is the same question §46 §8b asked of NC12 **by hand** in session 66, now a committed run — the session that adds the mirrors owes the original.

## Status (2026-08-01, session 66 — HISTORICAL): **FORMAL MODEL (a) SLICE B IS BUILT AND VERIFIED — SM §2 IS NOW COMPOSED WITH SM §7 ON THE ONE SHARED RESERVATION LEDGER, AND IT FOUND A SPEC DEFECT: `FM-5`.** Docs stay at v0.9.23 (marker grep **ZERO**), **but a normative edit is now OWED** — see below. The build record is **§46 §8** in `docs/superpowers/specs/2026-08-01-formal-model-a-ambiguous-order-design.md` — read it, then its RESUME HERE; **the next act is slice C** (liveness `P1`–`P4`, each with its negative control in the same slice). **35 model-(a) runs, all matching predictions**: the composed nominal green (6,729,250 generated / **832,179 distinct**, depth 30, no deadlock, ~8 s), a second `_SM2` nominal (`MAX_GEN = 0`) that **reproduces slice A's state graph to the digit** (25,514 / **5,169**, depth 15), **twelve** negative controls each violating their predicted invariant, and **twenty-one** positive controls each confirming their state is reached. **`FM-5` — SM §7's FLAG-3 ("the reservation stays generation-scoped FROM CREATION … the §2 classifier subtracts it THROUGHOUT") composed with §2's unconditional reservation-aware sizing lets a generation book its TARGET into the ledger at `Planned`, before the perp leg has bought anything.** On a package that is neutral, `realized − reserved` then reports a phantom SHORT, §2's determined branch buys — and buys back exactly the exposure the generation's own perp leg is about to buy. **TLC exhibits the unbacked ledger at two steps and the resulting DISPATCH at three; the end position (net long by the target) follows arithmetically from the model's own transitions and is labelled a derivation in §46 §8d, not a checked property** — pinning "the package was neutral at the start" would need history the model does not carry. **Four rounds of review and slice A all missed it for one structural reason: `INV1` is phrased over ledger-exact quantities, which are COMPUTED FROM the ledger, so a corrupt ledger corrupts them too and the phantom buy computes as non-overshooting — `overshoot` is FALSE in the violating state. An invariant built on the ledger cannot audit the ledger.** The fix is narrow and FLAG-3 keeps its words: what was underdetermined is the reservation's **amount**, not its existence — the entry is created at `Planned` carrying the generation's **outstanding net intent**, which is ZERO until the perp leg confirms and is amended as it fills (§2's own "coalesce / amend / cancel-replace, never a fresh command per event", applied to the reservation). That reading is nominal; the literal one is control NC7. **THE NORMATIVE EDIT IS OWED AND SCHEDULED FOR SLICE E** by §46 §5's roster ("findings triaged; spec defects tagged FM-5…"); until it lands, `docs/formal/` and `docs/state-machines.md` disagree about §7's FLAG-3 and CLAUDE.md's standing rule resolves it ("treat the formal model as authoritative about the safety kernel's behaviour") — the exact wording to land is in §46 §8d. **The model also found TWO defects in its OWN slice-A reasoning** (§46 §8e): `INV1`'s persistent conjunct rested on slice A's stated bound "no new counter-leg order appears mid-episode", which §7's perp reduce legitimately breaks — it was NOT weakened but MOVED to `INV1b` in the `_SM2` config where its premise holds (and the at-dispatch conjunct alone was verified to still catch NC1/NC6 composed); and **two formulations of `INV4` were tried and both are wrong**, one false of correct composed behaviour and one blind to `FM-5`, which is itself the finding — there is no harm-level phrasing that is both sound and complete, because every "what will the position become" quantity is computed from the ledger. **The slice-B fixture collapse, named by the checklist BEFORE the runs and probed (`PC18`): if the perp reduction always confirmed in FULL, "sells no more spot than the CONFIRMED reduction" and "…than the TARGET" are the same expression and NC8 is undetectable.** `NPEND` went 2 → 3 so the one subtraction can have three nonzero terms at once (`PC12`), and `PC9`/`PC10`/`PC11` are slice B's done-when: each of the three producers separately shown to contribute a nonzero term. **`TypeOK` earned its keep** — it failed at depth 22 on `q = 9` against slice A's ceiling of 8, because §7's generations move delta without spending the dispatch budget; domains widened deliberately, not silently. **NC12's own witness was DEGENERATE** (a zero-amount orphan, which leaks a slot but subtracts nothing) — the harmful nonzero orphan was confirmed reachable separately, and NC12 also DEADLOCKS, a second independent signature of the same defect. **NEXT (in order): (1) §46 slice C** (task #154); **(2) the §32m-D per-tier figure, DUE BY 2026-08-03T11:13Z**, measurable from 2026-08-02T11:13Z (task #150); **(3) off-host archive sync** — no target yet, recommendation a ~$60 external USB drive; **(4) the gate itself** (task #152, BCHUSDT binds ~2026-09-30). Task #153 (the hourly `restart_boundary`) awaits a user ruling and blocks nothing. H1 unpolluted; capture runs on (**PID 8495**). **Start at `## START HERE` below.**

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

## START HERE (updated session 70, 2026-08-02 — HISTORICAL — §46 SLICE E COMPLETE; `FM-7` and `FM-8` both landed, `FM-8`'s model encoding owed as slice F, done in session 71)

**Branch `feat/milestone-1-foundation`. Docs are v0.9.25.**

**First act, in order:**

1. **Task #150 — the §32m-D per-tier one-day figure.** It became measurable at
   **2026-08-02T11:13Z** and is **due 2026-08-03T11:13Z**; it was *not yet
   measurable* during session 70, which is the only reason it is not done.
   Tightest clock on the board. The procedure is fixed
   (`docs/superpowers/specs/2026-08-01-strategy2-capture-expansion.md`, its
   RESUME HERE item 1): **`du` split BTC-tier vs alt-tier** against §42c's
   bound (~0.6–1.1 G/day/symbol — the two Bybit streams are the unmeasured
   scale-down), **plus a full-day verify over the measured day**, which will
   show this restart's `process_restart` gaps and **must still report
   0 integrity**. Lands as **§44e in that record**, and it is **§38f's named
   revisit point** (ruling §42f-3) — the off-host decision gets re-argued on a
   measured disk clock.
2. **Task #156 — slice F: encode `FM-8`'s ruling and prove the hole closes.**
   The ruling landed; the run has not. §46 §11f holds the sketch — a
   `PREMISE_IS_STANDING` constant gating `Supersede("premise")` on
   `PdActive /\ cls # "Determined"`, `PC28` **flipping polarity** from
   positive control to invariant, and `NC21` as the control that must violate
   it. **Read the encoding subtlety before writing it:** the harm is not the
   state existing *transiently* (a legal quiesce window contains it), so the
   invariant must be read from a flag written at the event — the shape
   `INV6_SupersessionQuiescesFirst` already uses.
3. **Confirm the full-suite verdict** if session 70's background `./run.sh`
   did not finish. A filtered run is not the suite result.

**Slice E is closed; do not reopen its docket.** §46 §11a–§11f record all five
items, and §11d/§11e/§11f are this session's. The design record's RESUME HERE
is current.

**Standing rituals, unchanged:** `pgrep -f "arblab capture run"` **FIRST**
(PID 8495); **never rebuild over `bin/arblab` while capture runs**; cap
`TLC_WORKERS` beside it (4); marker grep `grep -rn "SPEC EDIT OWED" internal/ tools/`
must be **zero**. **New:** never edit a `.tla` or `run.sh` while `run.sh` is
running — it re-reads both on every config.

## START HERE (updated session 69, 2026-08-02 — HISTORICAL — §46 SLICE E PART-DONE, items 1 and 5 closed, `FM-6` found and landed; next are items 2, 3, 4)

**Branch `feat/milestone-1-foundation`. Docs are v0.9.24.** Read
**`docs/superpowers/specs/2026-08-01-formal-model-a-ambiguous-order-design.md`
§11 (the slice-E record so far), then its RESUME HERE**, before anything else.
The remaining docket is assembled there and is **not to be re-derived**.
Task **#154**.

**What closed this session:**

1. ~~`FM-5`'s normative edit~~ — **LANDED, v0.9.24** (§11a). Wider footprint
   than §8d prescribed, and the reading pass caught the first wording
   over-generalizing from one producer to three. Read §11a before touching
   the ledger clauses again.
5. ~~§10a's perp-side park~~ — **BUILT, and it found `FM-6`** (§11b, §11c).
   §10a's stated reason for the narrowing was wrong (`PC26` proves it), and
   the park's construction produced a **deadlock** that is a real spec gap.

**Still OPEN, in the order they were argued:**

2. **Model (b)'s five controls have never been asked the witness-degeneracy
   question** (§9f). Model (b) is in scope for the adversarial round anyway.
   `W1`–`W4` are model (a)'s precedent for how to ask it.
3. **§7d:** does "never issued twice" carry safety weight independent of the
   reservation? `NC2` issues a second floor with `overshoot` FALSE throughout,
   and §2's own counterexample for the rule assumes the reservation is *not*
   counted — which the adjacent sentence requires it to be. **Do not resolve
   it by weakening the spec.**
4. **§8f:** is *physical* neutrality the stronger and intended precondition
   for opening a generation? `PdPlan` guards on `TrueResidual = 0`, which is
   **reservation-netted**, so a generation may open on a package that is
   neutral *pending an in-flight reduction* rather than physically flat. It is
   `FM-5`'s mirror and should be attacked with it.

**Two things this session establishes that the remaining items should use.**
First, **building a missing transition is a finding technique, not bookkeeping**
— item 5 was carried as a correspondence tidy-up and produced `FM-6`. Items 3
and 4 are questions about *guards*, and the same move applies: encode the other
reading as a control and let TLC answer. Second, **a recorded justification is
load-bearing**: §10a's reason was wrong while its conclusion was defensible,
and nobody re-derives a recorded rejection ([[a-rejected-alternative-is-an-unchecked-claim]]).

**Standing rituals, unchanged:** `pgrep -f "arblab capture run"` **FIRST**
(PID 8495); **never rebuild over `bin/arblab` while capture runs**; cap
`TLC_WORKERS` beside it (4 was used this session); marker grep
`grep -rn "SPEC EDIT OWED" internal/ tools/` must be **zero**.

**The clock:** task **#150**, the §32m-D per-tier one-day figure, is **due
2026-08-03T11:13Z** and became **measurable at 2026-08-02T11:13Z**. It is the
tightest deadline on the board and does not depend on slice E.

## START HERE (updated session 68, 2026-08-02 — HISTORICAL — §46 SLICE D BUILT; superseded by session 69's slice-E items 1 and 5)

**Branch `feat/milestone-1-foundation`. THE NEXT ACT IS §46 SLICE E — the
adversarial round on the model itself.** Read
**`docs/superpowers/specs/2026-08-01-formal-model-a-ambiguous-order-design.md`
§10 (the slice-D record), then its RESUME HERE**, before anything else. The
docket is already assembled there and is not to be re-derived. Task **#154**.

**Slice E's five items, in the order they were argued:**

1. **`FM-5`'s normative edit to `docs/state-machines.md` — not optional**,
   exact wording in §46 §8d. §7's FLAG-3 composed with §2's sizing lets a
   generation book its TARGET before its perp leg buys anything; the fix is
   that the entry carries the **outstanding net intent**, zero until the perp
   confirms. Until it lands, `docs/formal/` and `docs/state-machines.md`
   disagree and CLAUDE.md's rule decides for the model.
2. **Model (b)'s five controls have never been asked the witness-degeneracy
   question** (§46 §9f). Model (b) is in scope for this round anyway.
3. **§46 §7d:** does "never issued twice" carry safety weight *independent* of
   the reservation? `NC2` issues a second floor with `overshoot` FALSE
   throughout.
4. **§46 §8f:** is *physical* neutrality the stronger and intended
   precondition for opening a generation? It is FM-5's mirror — attack them
   together.
5. **§46 §10a:** the perp-side `Held/Alert` park the model narrows away.

**What slice D landed.** `docs/formal/ambiguous-order-model.md` — correspondence
map (SM §2 Scope/States/Transitions, §2 Invariants **exhaustively**, §7
States/Transitions, §7 Invariants **exhaustively**), the full 47-run results
with the positive controls and the `W1`–`W4` witness probes, the honesty notes,
and a how-to-run. **No model change and no re-run**: every figure is quoted from
the committed slice A/B/C results, `run.sh` is still 55 runs.

**The shape worth carrying out of this slice.** *Nothing inside a model can
report that a spec clause never entered it.* Invariants catch bad behaviours
inside the model's world; an omitted clause leaves no trace there. That is why
the done-when is a totality condition over the **spec's** clause list, and it
is what found the `Held/Alert` gap on the first pass. The mirror at code
altitude is already in CLAUDE.md — the OWED grep cannot see an **unmarked**
claim, so a reading pass sits beside it.

**Also standing, in order after slice E:** (2) the **§32m-D per-tier figure,
due 2026-08-03T11:13Z**, measurable since 2026-08-02T11:13Z → §44e + §38f's
revisit (task #150) — **the tightest clock on the board**; (3) **off-host
archive sync** — no target exists yet, recommendation a ~$60 external USB
drive; (4) the **gate** itself (task #152). Task #153 (the hourly
`restart_boundary` on REST-overlay sources) awaits a user ruling and blocks
nothing.

Capture rituals first, always: **pgrep PID 8495**, background verify, marker
grep (**ZERO**). QA binaries to the scratchpad — **never rebuild over
`bin/arblab` while capture runs**. `run.sh` takes `TLC_WORKERS`; cap it at 4
whenever the capture is running.

## START HERE (updated session 67, 2026-08-02 — HISTORICAL — §46 SLICE C BUILT + VERIFIED; superseded by session 68's slice-D artifact)

**Branch `feat/milestone-1-foundation`. THE NEXT ACT IS §46 SLICE D — the
companion artifact `docs/formal/ambiguous-order-model.md`: the
correspondence map (spec clause ↔ model construct), the results table, the
honesty notes and a how-to-run.** Done when every clause in SM §2's and
§7's Invariants sections is either mapped to a model construct or listed as
deliberately unmodeled **with a reason**. Read
**`docs/superpowers/specs/2026-08-01-formal-model-a-ambiguous-order-design.md`
§9 (the slice-C build record), then its RESUME HERE**, before anything
else. Task **#154**.

**What stands.** `AmbiguousOrder.tla` model-checks SM §2 composed with SM
§7 for **safety** (`INV1`–`INV7`) and now for **liveness** (`P1`–`P4`)
under `LiveSpec = Spec ∧ Fairness`. `./run.sh` is **55 runs** — 8 for model
(b), 47 for model (a): 2 safety nominals + NC1–NC12, the liveness nominal +
NC13–NC16, 24 positive controls, and 4 control-witness probes. Model (a)'s
whole suite takes roughly 20 minutes at `TLC_WORKERS=4`.

**`FM-5`'s NORMATIVE EDIT IS STILL OWED**, scheduled slice E, exact wording
in §46 §8d. Until it lands the formal model and `docs/state-machines.md`
disagree, and CLAUDE.md's rule decides: the model is authoritative.

**Three things slice D must not re-derive (§46 §9a, §9d):**

- **The fairness set's OMISSIONS are the theorem.** `DeadlineBlows`,
  `PdSpotHolds`, `PdTerminalFlatten`, `PdPlan`, `Supersede` and **both
  compensating-trade dispatches** are unfair on purpose, each with its
  reason written at the definition. Making any of them fair would discharge
  a property by assuming what it exists to prove — `DeadlineBlows` would do
  it to `P1`, `PdTerminalFlatten` to `P3`.
- **`P4`'s consequent stops at "the ledger entry clears" deliberately.**
  Conjoining "and the episode ends `Determined`" would make `NC13` fail
  `P4` too, and a property two controls can fail names neither. The rest is
  recovered by composition with `P1`.
- **`INV1b` still lives in `AmbiguousOrder_SM2.cfg` (`MAX_GEN = 0`)**, and
  the generation's own legs are still not `RecordDispatch`ed (§46 §8a).

**The two shapes worth carrying out of this slice.** (1) **A control whose
state graph is identical to the nominal one is proof that no invariant
could have caught its defect** — `NC15`, 832,179 distinct states to the
digit. (2) **A control's WITNESS deserves the same fixture-collapse
question a fixture does**: TLC reports *a* counterexample, not the most
harmful one, and two of four landed at `budget = 0` where the harm is
unattributable. `W1`–`W4` are that question as committed runs.

**`run.sh` takes `TLC_WORKERS`.** Cap it (4 is what sessions 65–67 used)
whenever the capture is running: model (b)'s liveness configs hold every
core for ~30 minutes and the archive is un-backfillable. Every model-(a)
config tag is inside the `AmbiguousOrder_` namespace, so
`./run.sh AmbiguousOrder` selects all 47 — slice C added nine configs and
re-armed the session-66 trap without tripping it.

**Also standing, in order after slice D:** (2) the **§32m-D per-tier
figure, due 2026-08-03T11:13Z**, measurable from 2026-08-02T11:13Z → §44e +
§38f's revisit (task #150); (3) **off-host archive sync** — no target
exists yet, recommendation is a ~$60 external USB drive; (4) the **gate**
itself (task #152). Task #153 (the hourly `restart_boundary` on
REST-overlay sources) awaits a user ruling and blocks nothing.

Capture rituals first, always: **pgrep PID 8495**, background verify,
marker grep (**ZERO**). QA binaries to the scratchpad — **never rebuild
over `bin/arblab` while capture runs**; `make ci` writes no `bin/`.

## START HERE (updated session 66, 2026-08-01 — HISTORICAL — §46 SLICE B BUILT + VERIFIED, `FM-5` FOUND; superseded by session 67's slice-C build)

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

**`./run.sh AmbiguousOrder` now really does select model (a)'s 35 runs** —
it selected **14** until session 66, because `$FILTER` is matched against
the config TAG and the generated probe configs were named `PC_*`, outside
the namespace their own model owns. They are `AmbiguousOrder_PC_*` now.
Worth carrying as a shape, since slice C adds `P1`–`P4` and their controls:
a probe a plausible invocation cannot select is a probe that is not run,
and the FILTERED summary line counts the skips without naming them.

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
