# Record §93 — task #225: the available-balance read, and the published field that answers a different question

**Date:** 2026-08-09 (session 137) · **Task:** #225 · **Milestone:** 4
**Predecessors:** record §87 §5a (why B4's sizing needs `A` at all), record §89 F5 (the observation
this task's ⚠ was raised against), record §79 / task #208 (the position surface carries no available
margin)
**Venues:** Binance demo (`demo-fapi.binance.com`), Bybit demo (`api-demo.bybit.com`).
**Probes preserved:** `research/venue-probes/225-available-balance/`

**Status: the MEASUREMENT half is closed. The port is not built** — §8 states what it must be and
why that is a separate slice.

---

## 1. What was owed

§7.1's redesigned escalation sizes the re-post from `A₀`, the available balance. Record §89 F5
observed that Bybit demo, under §4.1's pinned `UNIFIED` + `ISOLATED_MARGIN` posture, serves
`totalAvailableBalance` and `availableToWithdraw` as **empty strings**, and that the asset-transfer
family answers `retCode 10032`. The task carried a ⚠ that **blocked design rather than riding along
with it**:

> Confirm whether the empty strings are a DEMO limitation or an ISOLATED_MARGIN one before designing
> around them — this was measured on demo only, and the two have different consequences for
> production.

The ⚠ is correct to block. The two readings imply different ports: under *demo-only*, production
publishes `A` and both venues have one shape; under *posture*, `A` is derived on Bybit forever and
the provenance split is permanent and belongs in the type.

**The ⚠ is discharged, and then overtaken.** The answer is *posture* (§3) — and §4 shows the answer
does not matter, because the published field is not the quantity anyway.

---

## 2. Method — one observation cannot separate two causes

§89 F5 is a single observation of a single state. Both candidate causes predict it exactly, so no
amount of re-reading that state distinguishes them. They differ on a state nobody had visited: the
**same demo account under a different margin mode**.

| | under `ISOLATED_MARGIN` | under `REGULAR_MARGIN` |
|---|---|---|
| H-demo | empty | **empty** |
| H-posture | empty | **populated** |

So the experiment is a flip, a read, and a flip back — and **the flip back is not cleanup, it is the
positive control**. If the fields populate under `REGULAR_MARGIN` and go empty again on restore, the
mode moved them. Had they stayed populated after restore, a lazy first computation would fit the
trace equally well and the reading would have been wrong *in the direction that looks like a clean
result*. The script prints that third outcome as `AMBIGUOUS` rather than folding it into the answer.

This is the repo's "encode the rejected reading as a control and let the checker answer" habit
(`FM-6`…`FM-9`) applied to a venue instead of TLC.

**Preconditions were re-read, not assumed.** Bybit rejects a margin-mode change with any position or
order live. Pass 1 read the account flat; pass 2 re-read it immediately before the flip, because the
state that mattered had been read minutes earlier. Restoration runs in `finally` and the script
verifies it, because §4.1's posture is a premise of every other probe in this tree.

---

## 3. F1 — the ⚠'s answer is POSTURE

`POST /v5/account/set-margin-mode`, flat account, `unifiedMarginStatus 3` throughout:

| field | A. `ISOLATED_MARGIN` | B. `REGULAR_MARGIN` | C. restored `ISOLATED_MARGIN` |
|---|---|---|---|
| `totalMarginBalance` | `""` | `99949.39907401` | `""` |
| `totalAvailableBalance` | `""` | `99949.39907401` | `""` |
| `totalInitialMargin` | `""` | `0` | `""` |
| `totalMaintenanceMargin` | `""` | `0` | `""` |
| `accountIMRate` / `accountMMRate` / `accountLTV` | `""` | `0` | `""` |
| **`availableToWithdraw` (per-coin)** | `""` | **`""`** | `""` |
| `walletBalance`, `totalPositionIM`, `totalOrderIM` | populated | populated | populated |

Seven account-level fields appeared under `REGULAR_MARGIN` and vanished again on restore. **The
account-level aggregates are a function of margin mode**, so under §4.1's pinned posture they are
absent in **production** as well as on demo. H-demo is refuted for those fields.

### F2 — the task's description merged two absences that have different causes

`availableToWithdraw` **stayed empty under both modes**. Only the account-level aggregates moved. So
"Bybit serves no available balance" is two absences, not one, and only one of them is the posture's.
The per-coin field's cause is **not established** by this run — demo, UTA generally, or something
else — and it is recorded as open in §7 rather than absorbed into F1.

This matters because it is exactly the shape the ⚠ warned about, one level down: a single sentence
naming two fields invites one cause, and the run that checks the cause checks it for whichever field
the reader had in mind.

---

## 4. F3 — the published field answers a DIFFERENT QUESTION, which makes F1 moot

Under `REGULAR_MARGIN` the account-level `totalAvailableBalance` reads `99949.39907401`. The USDT
wallet is `49995.59570032`. It is not the same quantity, and the response says what it is —
from a **single response**, so no price drift sits between the terms:

```
totalAvailableBalance                        99949.39907401
Σ usdValue where collateralSwitch == true    99949.39907401     residual 0E-8
     USDC   49992.10000000   collateralSwitch true
     USDT   49957.29907401   collateralSwitch true
     BTC    65171.92218100   collateralSwitch false  — excluded
     ETH     1922.87617700   collateralSwitch false  — excluded
Σ usdValue over ALL coins                   167044.19743201  == totalEquity, residual 0
```

So `totalAvailableBalance` is a **USD, multi-coin, collateral-filtered aggregate**. Three separate
reasons it is not the input §7.1 wants:

1. **Denomination.** It is USD, not USDT. USDT's own `usdValue` is `49957.29907401` against a
   `walletBalance` of `49995.59570032` — the peg is not 1, and the conversion rate is not published
   on the response.
2. **Coin scope.** It has already summed USDC into USDT. A re-post of isolated margin to a
   USDT-settled perp must be funded in **USDT**.
3. **Collateral filter.** It excludes `collateralSwitch: false` coins, which is why it is neither
   `totalEquity` nor a wallet total. That filter is account configuration, not a constant.

**The counterfactual therefore collapses.** Had the posture published this field, sizing from it
would have overstated available USDT by the entire USDC leg — `49992.10` on a `49995.60` USDT wallet,
very nearly **2×** — in the direction that sizes a re-post the account cannot fund, arriving on the
incident path. The derivation is not a workaround for a posture gap; **it is the only correct reading
for a multi-coin UTA account under any posture**, and F1's answer changes nothing about the design.

This is `a-field-read-as-a-property-it-does-not-carry` — the shape right, the quantifier wrong — and
`two-published-fields-on-different-bases`, on a live venue.

---

## 5. F4 — `walletBalance` is GROSS, and the probe's own first verdict said the opposite

The derivation `A = walletBalance − totalPositionIM − totalOrderIM` was **unfalsifiable** on a flat
account: both margin terms are zero, so it equals `walletBalance` and `A = W − IM` and `A = W` are
the same number. The substantive question is whether `walletBalance` **already** nets out posted
margin — if it does the subtraction double-counts and sizes the re-post **short**; if it does not,
omitting it sizes **long**. Both are wrong and a flat account reports neither.

Four phases, Bybit demo, BTCUSDT 0.002:

| phase | `walletBalance` | `totalPositionIM` | `totalOrderIM` |
|---|---|---|---|
| A. flat | `49995.59570032` | `0` | `0` |
| B. resting limit 4% below mark | `49995.59570032` | `0` | `12.64846955` |
| C. market fill | `49995.5239845` | `13.10378424` | `0` |
| D. **add-margin +50** | `49995.5239845` | `63.10378424` | `0` |

**Phase D is the isolating fixture.** `totalPositionIM` moved `+50.00000000` exactly and
`walletBalance` moved `0E-7`. Phase B is the same result on the order leg: `+12.65` of order margin,
`walletBalance` delta `0E-8`. **`walletBalance` is gross of committed margin on both legs, so the
subtraction is required, not defensive.**

### The method finding, and it is the more valuable half

**The probe's own inline verdict read phase C and concluded the opposite.** Phase C's `walletBalance`
moved by `−0.07171582`, and a test asking "did `walletBalance` move when margin appeared?" reads that
as *yes, it nets margin*. It is the **taker fee**:

```
0.002 × 65196.17 × 0.00055  =  0.0717157870
observed                     =  0.07171582
residual                     = −3.30E-8
```

The fee accounts for the entire move, so phase C attributes **nothing** to margin. A market fill
moves two quantities at once and the test cannot assign the change to either — while producing a
confident, wrong, opposite answer.

`add-margin` is the isolating operation: no fill, no fee, no PnL, and it is **exactly** the operation
§7.1 issues. This is CLAUDE.md's `singlewriter` lesson — "a probe two gates can see shows that
*something* objected, not that *this rule* did" — arriving on a venue measurement, where the
motivating case and the isolating fixture again turn out to be the same one.

The correction is recorded rather than re-run. A probe capture is **not re-derivable**
(`research/venue-probes/README.md`): re-running pass 3 opens a different position at a different
price. So the reading was re-derived from the captured bytes in `analyze225.py`, and the probe's
inline verdict was fixed to disqualify non-isolating phases by name.

---

## 6. F5 — Binance publishes the same construction, and a name-match on it is wrong by the whole added margin

Binance's `availableBalance`, measured against its own inputs in every phase:

```
availableBalance  =  walletBalance − (isolatedWallet + openOrderInitialMargin)     residual 0E-8
```

| phase | `W − A` | `isolatedWallet` | `openOrderIM` | `positionInitialMargin` | residual |
|---|---|---|---|---|---|
| A. flat | `0E-8` | 0 | 0 | 0 | `0E-8` |
| B. resting order | `12.52232000` | 0 | `12.52232000` | 0 | `0E-8` |
| C. position open | `13.02459251` | `13.02459251` | 0 | `13.04441781` | `0E-8` |
| D. add-margin +50 | `63.02459251` | `63.02459251` | 0 | `13.04448681` | `0E-8` |

So **Binance publishes the number Bybit makes us derive, by the same construction.** That is a
stronger design position than a provenance split alone: the two venues' `A` is one quantity computed
two ways, not two quantities hopefully alike.

**But the field that carries it is not the name-matching one.** Look at column 4 against column 5 in
phase D:

- Bybit's `totalPositionIM` **absorbed** the added 50 (`13.10 → 63.10`): it is **posted** margin.
- Binance's `positionInitialMargin` **did not** (`13.04441781 → 13.04448681`): it is the
  **requirement**. The posted figure is `isolatedWallet`.

An adapter that pairs them by name — Bybit `totalPositionIM`, Binance `positionInitialMargin` —
computes on Binance:

```
4908.66085796 − 13.04448681  =  4895.61637115
venue's own answer            =  4845.63626545
OVERSTATES by                    49.98010570
```

the whole added margin, again in the direction that sizes a re-post the account cannot fund. This is
record §89 F2's `realizedPnl`/`closedPnl` finding in a second place: **the same word for different
quantities**, and the name-match is the natural implementation.

---

## 7. What this run did NOT establish — recorded, not absorbed

**(a) `availableBalance` vs `maxWithdrawAmount` on Binance is UNSEPARATED.** All six candidate
fields — `availableBalance` and `maxWithdrawAmount`, at three levels (`/fapi/v3/account` top,
`assets[USDT]`, `/fapi/v3/balance`) — held **one distinct value in every phase**, including with a
position open and after add-margin. But **every phase had unrealized profit ≤ 0**
(`0`, `0`, `−0.02942195`, `−0.02873195`, `0`). `maxWithdrawAmount` is the field that should exclude
unrealized **profit**; with a loss there is nothing to exclude, so the two are pinned equal by the
sign of a quantity this run never controlled. **The separating state is a position in profit**, and
it was not reached. "They are the same field" is *not* measured, and the run says so rather than
reporting a clean sweep.

**(b) F2's per-coin cause.** `availableToWithdraw` is empty under both margin modes. Demo, UTA
generally, or another account property — undetermined.

**(c) Production.** Nothing here is a production measurement. F1 establishes the account-level fields
are a *function of margin mode*; that the same function holds on the production deployment is an
inference from the mode-dependence, not an observation.

**(d) Multi-coin funding.** §7.1's re-post is assumed USDT-funded. Whether a UTA account can post
isolated margin drawn from a non-settlement collateral coin is untouched, and F3 is the reason to
ask: the venue clearly *does* aggregate collateral across coins for some purposes.

**(e) The identity test has no independent witness on EITHER venue.** The task warns that
`ΔA = released + realized − fee` is tautological against the **derived** Bybit `A`. F5 extends the
warning: it is also uninformative against the **published** Binance `A`, because that number is
measured here to be a function of the same inputs. What each venue publishes independently is the
**wallet** and the **margin**; those are what a test may assert on.

---

## 8. What the port must be, and why it is a separate slice

The measurement fixes the design; it does not build it. Stated so the next slice does not re-derive
it:

1. **Per settlement asset, never an account aggregate.** F3. The port's answer is `Money` tagged with
   the settlement asset, and there is no account-level total in the type at all.
2. **One construction, two provenances.** `A = wallet − (posted margin + committed order margin)` on
   both venues. Binance may return its published `availableBalance`, which F5 measures as exactly
   that expression; Bybit must compute it. The **provenance difference is visible in the type**, per
   `venue-published-vs-locally-derived-data` — the consumer that decides whether to re-post is
   entitled to know which it holds.
3. **The Binance inputs are `isolatedWallet` + `openOrderInitialMargin`, NOT
   `positionInitialMargin`.** F5. This is the one place a reviewer will "correct" toward the
   name-match, so it gets a comment naming the measurement.
4. **Tests assert on wallet and margin, not on the identity.** §7(e).
5. **Zero is unusable.** An unpopulated `A` reads as "no cash", which refuses the re-post — the safe
   direction — but an *empty string decoded as zero* is a different failure and must be a refusal
   that names the field, per §4.1's direction rule. Bybit's `""` spelling is live on this surface
   today (F1, F2), so this is not hypothetical.

### Rejected alternatives

**Read `totalAvailableBalance` and require `REGULAR_MARGIN`.** Rejected on F3: the field is a USD
multi-coin aggregate and would be wrong by the USDC leg even if the posture published it. It also
trades a correct local computation for a change to §4.1's pinned posture, which is load-bearing
elsewhere.

**Derive on Bybit, read the published field on Binance, and smooth the provenance away behind one
port type.** Rejected: F5 makes the two numerically identical *today*, which is exactly what makes
the smoothing tempting and what makes it unsafe — a venue that changes what `availableBalance` nets
would move one side silently. The type keeps the distinction the measurement found.

**Take `maxWithdrawAmount` as `A` on Binance, as the more conservative field.** Rejected as
*unmeasured*, not as wrong — §7(a). Choosing it because it "sounds safer" would be picking between
two fields this run could not tell apart, and recording the pick as a finding.

**Defer the whole task until a production credential exists.** Rejected: F1's mode-dependence and
F3's structural argument are both established from demo, and F3 does not depend on the deployment at
all. Waiting would block B4 on evidence that would not change the design.

---

## 9. The redaction check was VACUOUS on its first run, and that is the third method finding

`research/venue-probes/README.md` requires that a redaction pass be treated as a mutation of the
evidence: **re-run the consumer and confirm it still reaches the same number**, because a scrub that
silently removed an input would otherwise look identical to one that removed metadata.

It was run. It passed. It proved nothing.

`redact225.py` changed exactly one file — `probe225-pass1-readonly.json`, 8 `accountAlias` fields —
and `analyze225.py` at that moment read **only the pass-3 captures**. The consumer never opened the
file that was mutated, so the diff was clean *by construction*. A scrub that had deleted every
monetary field from pass 1 would have produced the identical clean diff.

This is the collapse checklist landing on the guard rather than on the measurement: **the fixture did
not contain the thing under test**, and the tell was the same one CLAUDE.md names — the check and its
subject were never connected, only adjacent. It is the twelfth entry, and the first found in a
harness whose whole job is keeping another harness honest.

The fix is to connect them, not to re-word the claim: `analyze225.py` now reads **every capture the
redactor writes**, re-derives F1 and F3 from pass 1 and pass 2, and asserts on the redaction marker
itself. Two facts fell out of widening it, which is the usual dividend:

- `Σ usdValue` over **all** coins equals `totalEquity` to residual `0` in **both** responses — the
  `ISOLATED_MARGIN` one and the `REGULAR_MARGIN` one. F3's decomposition holds across the flip, so it
  is a property of the wallet surface rather than of the mode.
- USDT's `walletBalance` exceeds its own `usdValue` by `38.29662631`. The de-peg is a *published*
  quantity on the same row, which is what makes F3 checkable rather than merely arguable.

**The generalisable rule, and it belongs beside the README's existing one:** a reproduction check must
name the file the mutation touched. "Re-run the consumer" is only a check when the consumer's inputs
and the mutation's targets are the same set — and nothing in the passing output says whether they
were.

---

## 10. Probes preserved

```
research/venue-probes/225-available-balance/
  probe225.py                        pass 1, READ-ONLY survey, both venues
  probe225_pass2_marginmode.py       pass 2, the flip / read / flip-back experiment (F1, F2)
  probe225_pass3_separate.py         pass 3, order + position + add-margin, both venues (F4, F5)
  analyze225.py                      the reading, re-derived from the captures
  captures/*.json                    verbatim responses
```

Redaction per `research/venue-probes/README.md`. Balances are kept — they *are* the measurement, and
F3's residual-`0E-8` decomposition cannot be reproduced without them.

**Account state on exit:** both demo accounts flat, no open orders, Bybit posture restored to
`ISOLATED_MARGIN` and verified. Pass 3 set BTCUSDT leverage to 10 and margin type to `ISOLATED` on
Binance; the prior leverage was not recorded before the change, which is a gap in the probe worth
not repeating.
