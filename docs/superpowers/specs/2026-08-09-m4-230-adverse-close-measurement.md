# Record §89 — task #230: the adverse-close measurement, and the four things it found that it was not looking for

**Date:** 2026-08-09 (session 133) · **Task:** #230 · **Milestone:** 4
**Predecessors:** record §85 (top-up relation), §87 (#224's terminal-rung redesign, R1a and §7's
measurement plan), record §86 / task #221 (the measurement this one exists to replace)
**Venues:** Binance demo (`demo-fapi.binance.com`), Bybit demo (`api-demo.bybit.com`). Demo settles
**mechanism**, never economics.

---

## 1. Why this measurement exists

Record §87 R1a says a reduce-only close of `r` at fill price `P` on a short does not return the
posted margin it releases. It returns

```
    ΔA = (1 − α)·m  +  r·(e − P)  −  fees          [+ funding, if a settlement lands in the window]
```

and under the production condition the rung exists for — an adverse rally, `P > e` — the realized
term is **negative**, so the re-post is **not** self-funded.

Task #221 could not have found this. It measured a partial isolated close **at a mark near entry**,
where `r·(e − P) ≈ 0`, so *margin released* and *cash received* were the same number. The fixture was
pinned to the value that collapses the distinction the design has to make. §87 §7 therefore requires
a **materially adverse** close, a **positive control** that reproduces #221's invariance, a
**tier-boundary** run, and **predicted-vs-observed `liqPrice`** on both venues.

All four ran. The identity holds **exactly**. Four further findings came out of the run, and three of
them were not what it was looking for.

---

## 2. Method — and the one part of it that is a finding

An adverse close needs `P` materially different from `e`, and a venue will not sell you a bad price
on request: a limit order fills at better-or-equal, so an adverse **entry** cannot be bought. Two
levers exist, and this run used both, deliberately, as independent routes to the same condition.

**(a) The spread, deterministically.** A short opened and closed with market orders sells at the bid
and buys back at the ask, so `e − P = −(spread)` with no waiting on the market. On BTCUSDT the spread
is 0.0106% and this is invisible; on a wide-spread instrument it is material by construction. This is
not a cheat on the physics — §87's identity is over *our own fills*, and a `P > e` produced by the
spread is as genuinely adverse as one produced by a rally.

**(b) Genuine market drift.** BTC rallied ~0.10% against both shorts during the session, which is
§87's literal production condition.

**The method fact worth recording: (a) works on Binance demo and does NOT work on Bybit demo.**
Binance filled a 15,000-unit `DRIFTUSDT` market sell as a run of fills **all at the bid (0.0130)**
and the reduce-only buy **at the ask (0.0132)** — it matches against the published book. Bybit filled
a 49,000-unit `0GUSDT` market sell at a **single price** of 0.1503 and the reduce-only buy of 25,000
at a **single price** of 0.1503 while the displayed ask was **0.1505** — better than the opposite
side, no walk, at any size. So **Bybit demo does not consume its published order book**, and an
adverse fill cannot be manufactured there at all; only real mark drift produces one. That bounds what
Bybit demo can be asked to measure, and it is why route (b) carries the Bybit adverse result.

A third lever was considered and is worth writing down because it is wrong in an interesting way:
**leverage cannot help.** The realized term as a fraction of released margin is `leverage × δ`, but
the realized term against the **fee** is `δ / f` — *leverage-independent*. Leverage scales both terms
against released margin together and buys no separation between the two quantities that must be told
apart. Only a wider `δ` does.

---

## 3. What was measured

Six closes across two venues. `m` = posted margin (`isolatedWallet` / `positionIM`), `α = q₁/q₀`,
`δ = (P − e)/e`, `f` = taker fee rate.

| # | Venue · symbol | route | α | δ | realized term | as % of released `m` | realized : fee | R1a error |
|---|---|---|---|---|---|---|---|---|
| 1 | Binance · BTCUSDT | **control**, near entry | 0.5 | +0.004% | −0.013 | −0.20% | **1 : 10** | −1.0E-8 |
| 2 | Bybit · BTCUSDT | **control**, near entry | 0.5 | +0.0002% | +0.0005 | +0.007% | 1 : 357 | 5E-9 |
| 3 | Binance · DRIFTUSDT | spread (a) | 0.5 | **+1.538%** | **−1.500** | **−14.35%** | **30 : 1** | **0E-9** |
| 4 | Bybit · 0GUSDT | spread (a) — did not bite | 0.5 | −0.001% | +0.050 | +0.026% | 1 : 41 | 5E-9 |
| 5 | Binance · BTCUSDT | drift (b) | 0.5 | **+0.100%** | −0.16275 | **−3.41%** | 2.5 : 1 | **0E-9** |
| 6 | Bybit · BTCUSDT | drift (b) | **0.6** | **+0.118%** | −0.1536 | **−3.52%** | 2.15 : 1 | −1.4E-8 |
| 7 | Binance · BTCUSDT | drift (b), mirror of 6 | **0.6** | +0.101% | −0.0658 | −3.45% | 2.5 : 1 | **0E-9** |

Runs 1 and 2 are the positive control: they reproduce #221 exactly — `isolatedWallet`
12.74714440 → 6.37357220, `positionIM` 13.34616567 → 6.67308284, and **`liquidationPrice` /
`liqPrice` byte-identical across the close** (65919.93470120 and 65878.9). So the harness
distinguishes "the formula is right" from "nothing moved".

**R1a's identity is confirmed on both venues, to the last published digit**, across a 1.5% adverse
close, two 0.1% adverse closes and two near-entry controls.

---

## 4. Finding F1 — the FEE alone already breaks #221's reading, and by more than the realized term does at near entry

In the control, the realized term is **−0.013** and the commission is **0.1298** — the fee is **ten
times larger**. #221's reading `ΔA = (1−α)m` is wrong by 0.1428 there, and *almost all of that is
fee*.

§87 R1a frames the gap as the realized term and names fees second. At the mark where a rebalance
actually fires — near entry, most of the time — **the ordering is reversed**: the fee is the dominant
error and the realized term is a rounding correction on it. The realized term overtakes it only past
`δ = f` (0.04% on Binance, 0.055% on Bybit), which run 5 crosses and run 1 does not.

Both terms are always present and always adverse to the re-post. The consequence for §7.1 is that
**a fee-only correction to #221's reading would look right in every quiet-market test and fail
exactly when the rung matters**, which is the shape of defect this project keeps finding. The
identity must carry both terms, and R1a already does; what changes is that the fee is not the
second-order one.

---

## 5. Finding F2 — every run before #6 was VACUOUS on the pro-rata claim, including #221's

Bybit's `qtyStep` on BTCUSDT is 0.001, so run 6 closed 0.002 of 0.005 and `α = 0.6` rather than 0.5.
That accident caught a fixture collapse in **every partial close this project has ever measured**.

At `α = 0.5`, `(1 − α)·m` and `α·m` are **the same number**. So "the venue releases the margin backing
the CLOSED fraction" and "the venue releases the margin backing the REMAINING fraction" predict an
identical observation, and #221's half-close — and runs 1–5 here — could not tell them apart. The
claim looked measured and was not.

At `α = 0.6` they separate by a factor of 1.5:

| venue | `m₀` | `(1−α)·m₀` | `α·m₀` | observed release | error |
|---|---|---|---|---|---|
| Bybit · BTCUSDT | 10.90308284 | **4.361233136** | 6.541849704 | **4.36123315** | −1.4E-8 |
| Binance · BTCUSDT | 4.77178610 | **1.908714440** | 2.863071660 | **1.90871444** | 0 |

Both venues release the **closed** fraction. The claim is now pinned; it was not before, on either
venue. Run 7 exists only because this repo's rule is that a fix earning a repetition gets applied to
its mirror immediately.

**This is a checklist item, not a war story:** `α = 0.5` belongs on the list of fixture values that
collapse a distinction, beside a mailbox depth of 1 and a three-element reversal.

---

## 6. Finding F3 — Binance picks the maintenance bracket at the LIQUIDATION notional, and it is a fixed point

§87 §5g left the tier **basis** open — whether bracket membership is evaluated at the mark notional,
the entry notional, or `q·L*` — and answered it with a *ruling* (cap supported Bybit notional inside
tier 1) because Bybit's 2,000,000 boundary is unreachable on demo. Binance's is reachable. It is now
measured, and the answer is **not** the natural assumption.

The discriminating configuration has to be arranged: `cum` exists precisely to make maintenance
margin continuous at a boundary, so the brackets agree *at* the line and diverge only away from it. A
position sitting near the boundary discriminates weakly. The run used `DRIFTUSDT` (bracket 1
`[0, 5000)` MMR 0.01 `cum` 0; bracket 2 `[5000, 10000)` MMR 0.015 `cum` 25) and pushed `q·L*` well
past the line.

**Short leg** — `q` 370 000, `e` 0.0131, `m` 966.9765, current notional **4 847 (bracket 1)**:

| bracket used | predicted `L*` | error vs observed 0.01554780 | `q·L*` | self-consistent? |
|---|---|---|---|---|
| 1 — the CURRENT notional's | 0.01555787 | +1.0E-5 | 5 756 | **no** — outside `[0, 5000)` |
| 2 — the LIQUIDATION notional's | **0.01554780** | **+5.6E-10** | 5 753 | **yes** — inside `[5000, 10000)` |

**Long mirror** — `q` 400 000, `e` 0.0132, `m` 1093.36, current notional **5 240 (bracket 2)**:

| bracket used | predicted `L*` | error vs observed 0.01057232 | `q·L*` | self-consistent? |
|---|---|---|---|---|
| 1 — the LIQUIDATION notional's | **0.01057232** | **+3.2E-9** | 4 229 | **yes** — inside `[0, 5000)` |
| 2 — the CURRENT notional's | 0.01056254 | −9.8E-6 | 4 225 | **no** — outside `[5000, 10000)` |

A short reaches **up** a bracket from a bracket-1 notional; a long reaches **down** from a bracket-2
notional. So the rule is not directional and is not "the current notional's tier": **the bracket is
the one that is self-consistent at `q·L*`** — a fixed point.

**And the venue publishes two fields on two different bases.** `maintMargin` is 48.47 on the short
(= current notional 4 847 × 0.01, bracket 1) and 53.60 on the long (= 5 240 × 0.015 − 25, bracket 2)
— the **current**-notional bracket in both cases, while `liquidationPrice` uses the
**liquidation**-notional bracket. Deriving one from the other is a trap, and both are published on
the same row.

**What this changes.** §87 R1b's piecewise solve enumerates brackets for the candidate `q_final`. The
enumeration is still right, but **the acceptance criterion is not "the bracket containing
`q_final·e`"** — it is "the bracket `i` whose own `L*` puts `q_final·L*` back inside bracket `i`".
This is filed as its own follow-up rather than patched here (§9).

---

## 7. Finding F4 — Bybit's liquidation coefficient carries a fee term, measured, and the common relation is wrong by 0.055%

§87 §5c predicted this: Bybit's published material carries leverage, taker-fee and extra-margin terms
the common relation does not, and record §85 verified the relation on **Binance only**. Plan item 4
existed to surface a wrong coefficient. It surfaced one.

The common relation `L = (m/q + e + ded/q)/(1 + MMR)` **overstates** Bybit's published `liqPrice`
consistently:

| point | symbol | MMR | `m` | predicted | observed | error |
|---|---|---|---|---|---|---|
| A | BTCUSDT | 0.005 | 6.67308284 | 65915.14086 | 65878.9 | **+36.24** |
| B | BTCUSDT | 0.005 | 10.90308284 | 66756.93191 | 66720.2 | **+36.73** |
| C | 0GUSDT | **0.015** | 190.04748513 | 0.15557034 | 0.1554 | **+0.00017** |

Points A and B are **orthogonal**: the re-post moved `m` alone (4.23 USDT) while `q`, `e` and the
tier held. That matters because plan item 6 is right — **a single predicted/observed point is
satisfiable by more than one candidate formula**, and the two survivors here differ by 3×10⁻⁶.

Writing `L = (e + m/q)/(1 + k)` and solving `k` from each observation:

- `k_A = 0.005552864`, `k_B = 0.005553289` — one coefficient fits both to **4.3×10⁻⁷**, which is
  rounding scale against the 0.1 tick.
- `k ≠ MMR + f = 0.00555`. That candidate misses by **+0.19 and +0.22, same sign on both points**,
  against a 0.1 tick — two ticks out, twice, in the same direction.
- `k = (MMR + f)/(1 − f) = 0.005553054` fits both to **≤0.016**, well inside the tick. (`(MMR+f)(1+f)`
  is indistinguishable from it here — they agree to 1.7×10⁻⁹.)

The taker fee is not assumed: it is **measured at exactly 0.00055** from the execution ledger
(`execFee 0.1785025 / (0.005 × 64910)`). Point C, at a different symbol, `e`, `q` **and MMR (0.015)**,
rules out the common relation independently — but its 0.0001 tick is too coarse to separate the two
survivors, so the BTCUSDT pair is what does that.

This is #227's evidence, not its conclusion — see the limits in §8.

---

## 8. Finding F5 — Bybit publishes NO available balance under this posture, so `A` must be DERIVED, and that makes the obvious check tautological

§87's re-post sizing reads `A_live`: `add = clamp(q_live·D_i − cum_i − m_live, 0, A_live)`. On Bybit
demo, under `UNIFIED` + `ISOLATED_MARGIN` (`unifiedMarginStatus: 3`), **there is no `A` to read**:

- `GET /v5/account/wallet-balance` returns `totalAvailableBalance`, `totalMarginBalance`,
  `totalInitialMargin`, `totalMaintenanceMargin`, `accountIMRate` and the per-coin
  `availableToWithdraw` as **empty strings**. Only `totalEquity`, `totalWalletBalance`,
  `walletBalance`, `equity`, `totalPositionIM` and `totalOrderIM` are populated.
- `GET /v5/asset/transfer/query-account-coins-balance` answers **`retCode 10032 — "Demo trading are
  not supported."`**, so the asset-transfer balance family is unavailable on demo entirely.

venue-profiles §9 already records *"available margin … not published; `positionBalance` is
`Deprecated`"* for the per-position question. This extends it to the **account** level and to the
posture §4.1 actually pins: the account-level figure Binance answers with (`availableBalance`) has no
Bybit counterpart that is served here.

**The consequence has to be stated rather than buried.** With `A := walletBalance − totalPositionIM −
totalOrderIM`, the identity `ΔA = released + realized − fee` is **true by construction**
(`ΔA = ΔW − ΔIM`), so "confirming" it re-derives our own definition and measures nothing. That is this
repo's *stored-form-that-contains-the-answer* trap one layer out. The Bybit rows in §3 are therefore
reported against the **two independently published facts** underneath:

1. `ΔW = r·(e − P) − closeFee` — **error 0E-8 on both Bybit runs**;
2. `Δm = −(1 − α)·m₀` — **error 5E-9 / −1.4E-8**;

and the derived `ΔA` is labelled derived. On Binance `availableBalance` is published and `ΔA` is a
real observation, which is why the Binance rows carry the identity directly.

---

## 9. Finding F6 — `closedPnl` and `realizedPnl` are the same word for different quantities

Feeding Bybit's `closedPnl` into §87's identity as "the realized term" **double-charges the opening
fee**. Measured: on the 0.005 control close, `ΔW` was −0.178 while `closedPnl − execFee` was −0.535 —
off by **0.357**, which is exactly the open fee (`0.01 × 64910.1 × 0.00055`).

The venue's own decomposition confirms it. From `/v5/position/closed-pnl` on the 0GUSDT run:
`closedPnl = -4.0832775`, `openFee = 2.0666525`, `closeFee = 2.066625`, and gross
`25000 × (0.150302 − 0.1503) = 0.05` — so `closedPnl = gross − openFee − closeFee`, exactly.

| venue | field | scope | fees |
|---|---|---|---|
| Binance | `realizedPnl` on `GET /fapi/v1/userTrades` | **per fill** | **GROSS** — `commission` is a separate field on the same row |
| Bybit | `closedPnl` on `GET /v5/position/closed-pnl` | **per closed position record** | **NET of BOTH the open and the close fee** |

The identity's realized term is the **gross** `r·(e − P)`; the fee that belongs in it is the
**closing** `execFee` alone. An adapter that reads the field whose name matches on each venue gets
one venue right and silently mis-states the other — and it mis-states it in the direction that
**understates** the cash returned, which is the safe direction for a re-post and therefore the one
nothing would alarm on.

---

## 10. Limits — stated, not absorbed

1. **The Bybit liquidation coefficient is identified, not closed.** Points A and B share the same `e`
   and `q`; only `m` moved. A partial close can never supply a new `m/q` (pro-rata release keeps it
   invariant), so only adding margin varies it, and the third point C is too coarse to separate the
   two surviving candidates. `(MMR + f)/(1 − f)` and `(MMR + f)(1 + f)` remain indistinguishable, and
   nothing here derives *why* a `(1 − f)` appears. **#227 stays open**; this record is its evidence.
2. **The Bybit tier basis is still unmeasured**, exactly as §87 §5g said: the 2,000,000 boundary is
   unreachable on demo. §5g's ruling — cap supported Bybit notional inside tier 1 — **stands
   unchanged**, and F3 does not transfer across venues.
3. **Bybit's published `positionMM` implies a third rate** (`MM/posValue = 0.005561`, constant across
   both `m` values) that matches neither `MMR + f` nor the fitted `k`. It is not needed by anything
   here and is not explained; it is recorded because it is a second published quantity disagreeing
   with the first, the same shape as F3's `maintMargin`/`liquidationPrice` split.
4. **No funding settled inside any window.** Next funding was 196 minutes out at the start and the
   income ledger was checked for non-`REALIZED_PNL`/`COMMISSION` entries on every Binance run. The
   `[+ funding]` term of the identity is therefore **untested**, not confirmed.
5. **Demo settles mechanism, never economics** (unchanged standing caveat), and §2's Bybit fill
   behaviour is now a known bound on what this venue's demo can be asked.
6. **The adverse runs on Bybit reach `δ/f ≈ 2.1`, against Binance's 30.** Both separate the realized
   term from the fee; Binance's is emphatic and Bybit's is not, and F5's route (a) is why.

---

## 11. Rejected alternatives

- **"Wait for a big enough BTC move and skip the wide-spread instrument."** Rejected on evidence: BTC
  drifted 0.008% → 0.148% → 0.101% over ~35 minutes and never reached the 0.25% threshold the watch
  was set to. A measurement whose completion depends on market volatility is not a measurement plan.
  The spread route made run 3 deterministic and immediate, and the drift runs 5–7 arrived anyway as
  the faithful version.
- **"Raise leverage until the realized term is material."** Rejected on the algebra (§2): `δ/f` is
  leverage-independent, so leverage cannot separate the realized term from the fee. It was also
  actively unsafe here — 0GUSDT at 50× would have opened *already liquidated* (`L* = 0.150856` against
  a mark of 0.1508).
- **"Use a dated futures contract for a wider spread on Bybit."** Available (`DOGEUSDT-25SEP26` at
  0.582%) and rejected: §4.1 pins perpetuals, and buying a wider `δ` at the cost of a caveat on the
  instrument class is the wrong trade when route (b) supplies the perp answer.
- **"Report the derived Bybit `ΔA` as a third confirmation."** Rejected as tautological — see §8.
- **"Use BTCUSDT's 50,000 boundary for the tier-basis run, as §87 §7 item 5 names."** Substituted with
  `DRIFTUSDT`'s 5,000 boundary: same question, one-tenth the margin, and no collision with the BTC
  shorts still being measured. The 50,000 run remains available and would add nothing F3 does not
  already settle.
- **"Patch §87 R1b's bracket criterion here."** Rejected — it is a normative change to the sizing
  solve on the incident path, and it wants its own task rather than a paragraph in a measurement
  record (§12).

---

## 12. What this opens

| # | Item | Where it goes |
|---|---|---|
| **#233** | §87 R1b's piecewise solve: the bracket acceptance criterion is the **fixed point at `q·L*`**, not the bracket containing `q_final·e` (F3) | normative, blocks B4's sizing |
| #227 | Bybit's liquidation formula — evidence supplied (F4), two candidates surviving, `e`/`q` still unvaried | unchanged, now evidenced |
| #225 | available-balance read — Binance answers, **Bybit has no published `A` under §4.1's posture** (F5) | unchanged, now measured |
| — | `α = 0.5` joins the fixture-collapse checklist (F2) | CLAUDE.md |
| — | `closedPnl` vs `realizedPnl` (F6), Binance's two-basis rows (F3) | venue-profiles §9 |

**#230 is closed.** The identity §87 R1a asserts is confirmed to the last published digit on both
venues, under both an engineered 1.5% adverse close and a genuine market rally, against a positive
control that reproduces #221 exactly.
