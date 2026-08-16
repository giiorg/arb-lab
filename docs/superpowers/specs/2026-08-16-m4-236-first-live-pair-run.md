# Record §98 — the first order this project ever placed, and the three things the run found that the doubles could not

**Date:** 2026-08-16 (session 145) · **Task:** #236 demo run · **Milestone:** 4 (pre-gate)
**Predecessors:** record §95 (#236's scoping and its not-claimed list), record §97 (the two §4.1
defects the run's *preparation* found), the runbook `docs/runbooks/pair-demo-run.md`
**Venues:** Binance demo (`demo-fapi.binance.com`), Bybit demo (`api-demo.bybit.com`).
**FOUR ORDERS WERE PLACED. Two opened a cross-venue pair; two closed it. Both accounts end flat.**

---

## 1. Why this record exists

Records §95 and §97 both close with the same sentence: *no order has reached a venue*. This one does
not. `arblab pair run` drove a cross-venue perp-perp package across two live demo deployments,
both legs filled, and the verdict was `VerdictIntact` at exit 0.

The run **passed**. That is the least interesting thing in this record. What matters is that a
passing run surfaced one structural gap and two measurement traps, none of which any test in this
repo could have reached — for the same reason §97's defects were unreachable, arriving one layer
out: **the doubles proved the code consistent with itself, and the demo proves the code consistent
with a venue that is not the one we will trade.**

## 2. What was run, and what it did

Steps 1, 2 and 5 of the runbook were done in session 144. This session did §3, §4 and §6 as one
continuous sequence, per the runbook's own warning that a conversational turn between the book read
and the send is how a non-marketable limit produces a clean run that measured nothing.

| Step | Value |
|---|---|
| Books read | 12:19:35Z — Binance `0.0801 / 0.08019`, Bybit `0.08010 / 0.08020` |
| Prices set | binance sell `0.0797` / unwind `0.0806`; bybit buy `0.0806` / unwind `0.0797` |
| Buffer | four ticks through the touch each way, all four on the `0.0001` grid |
| `pair.id` | `01a00a83-e068-7550-9dbf-13283b0cefa5`, `attempt` 0 |
| Document | `configs/pair.demo.2026-08-16.json` |
| Result | `VerdictIntact`, exit 0, both legs filled |

The attestation digest was **re-derived** rather than copied from the handoff, and matched
(`3eefaed9…`). §97's own lesson about premises nobody re-derives; it cost one `shasum`.

Fills, from the venues rather than from the run's own report:

| | Entry | Exit | Fee charged |
|---|---|---|---|
| Binance | sell 100 @ `0.08012` | buy 100 @ `0.08067` | `0` |
| Bybit | buy 100 @ `0.0806` | sell 100 @ `0.0806` | `0.008866` each side |

Post-run `reconcile run` on each side: `ExitClean`, 5 records replayed, 1 dispatched order, **1 fill
admitted, 0 duplicates**. The join had a record to join against for the first time.

## 3. F23 — a pair can be OPENED but not CLOSED in band, and the happy path is the one with no exit

`pair run` establishes an intact pair. Its unwind — `ErrUnwindRefused` / `drive.compensate` — fires
only on a **half-open** one, because sizing the residual of a skewed or indeterminate pair is §6.7's
job and §6.7 cannot see a cross-venue pair (§95 §4). So a *successful* run leaves a live position
that the root has no command to take off.

The runbook is not silent about this — its exit-4 recovery says *"close the exposed leg by hand,
reduce-only"*. What nobody had derived is that **the same instruction is the only exit from a
SUCCESSFUL run**, and that taking it has a consequence:

```
verdict:  ExitOperator
latched:  [GateNetDeltaAsserted]  <- operator clearance required; a retry will not lift these
  unexplained binance:linear_perp:LABUSDT:linear: venue 0, ours -100, delta 100
  unexplained bybit:linear_perp:LABUSDT:linear:   venue 0, ours 100, delta -100
```

The net-delta invariant caught an out-of-band position change on both sides. That is the gate
working, and it is the first time it has fired against a real venue rather than a fixture — the
single best piece of evidence this session produced, because it came from a source that shares no
code path with the writer.

But it composes into a standing state: **no CLI lifts `GateNetDeltaAsserted`.** `arblab margin
reconcile` is scoped to §9's Ambiguous *margin ops* — "the only thing that lifts one" is its own
help text, and it is a different incident. The gate is specified as an alert for operator clearance
that is "not auto-corrected" (§7.1, `platform/boot/gate.go`). Therefore:

> **Every successful `pair run` terminates in two latched journals, and the only way back to a
> bootable deployment is a fresh journal.**

**Why no test found it.** Every fixture in #236's fourteen tasks had either a half-open pair to
unwind or a clean refusal to assert. None had a *successful* pair sitting there afterwards asking to
be taken off, because the assertion was always the verdict and the verdict was the end of the test.
The unhappy paths were exhaustively covered; **the happy path was the untested one** — its
post-condition was never a subject.

*General:* when a system's failure branches all have named recoveries, check whether the SUCCESS
branch has one. A terminal state reached by everything working is easy to leave out of a test suite,
because nothing about it looks like a case.

## 4. F24 — the demos misreport cost in OPPOSITE directions, and one does it while publishing the right number

| | Reference row (`hand_recorded`) | Venue *states* | Venue *charged* |
|---|---|---|---|
| Binance demo | taker `0.0005` | `takerCommissionRate 0.000500` | **`0`** |
| Bybit demo | taker `0.00055` | `/v5/account/fee-rate` → `retCode 10001` | **`0.0011`** — exactly 2× |

Binance demo is the dangerous half. `GET /fapi/v1/commissionRate` returns exactly the number the
reference revision carries, so the model and the published field **agree perfectly** — and
`userTrades` shows `commission: 0` on both fills. A consistency check between the fee revision and
the venue's stated rate passes. A `Validate()` on the reference passes. The only artifact that
disagrees is the realized ledger, which nothing in the decision path reads.

*General:* **a source that publishes the correct number and acts on a different one passes every
consistency check you can write against it.** This is the venue-side twin of §46 §8e's rule — a
check built on the published field cannot audit the published field — and the only falsifier is a
realized outcome from a *different* surface.

*Consequence, and it is not small:* **no cost figure measured on either demo is evidence about
mainnet economics.** §95 §8's not-claimed list keeps its entry for demo fills, and gains one for
demo fees. The demos measure the machine. They do not measure the money.

## 5. F25 — an observation both readings are the SAME function of, and why no fixture parameter separates them

Bybit charged `0.008866` on `8.06` notional. That is equally consistent with:

- **(a)** the taker rate is `0.0011`, or
- **(b)** the rate is `0.00055` applied to **twice** the notional.

The collapse checklist's standing move is *find the fixture parameter pinned to the value that
collapses the distinction, and vary it*. **It cannot work here.** Both readings are **linear in
quantity**, so every order size produces the same ratio and the two predictions coincide at every
point of the only parameter we control. There is no `α = 0.6` to move to.

The separating instrument therefore has to be an external authority — the venue's own published
rate — and that endpoint refused with `retCode 10001`. **The question is left open rather than
resolved by the more plausible reading**, because "0.11% is a normal-looking taker rate" is exactly
the kind of plausibility that §97's slug transcription also had.

*General, and this is a new member of the fixture-collapse family:* the checklist assumes a
separating parameter **exists**. When both readings are the same function of every quantity the
experimenter controls, no fixture can be designed to tell them apart, and the finding is not "we
need a better fixture" but "the instrument must come from outside the system."

## 6. The fill-price question, settled by a control rather than an argument

The entry filled at `0.0806` on Bybit against an ask read as `0.0802` — *exactly the limit price*,
which admits two readings: the market moved, or the demo matches marketable IOCs at the limit.

Encoded as a control on the way out, per this repo's habit of letting the venue answer: a
reduce-only sell with `orderPrice 0.078` against a bid of `0.08060`.

```
12:37:23 Sell qty 100 price 0.0806 orderPrice 0.078
```

It filled at the **bid**, not at the limit. **Bybit demo matches against its book.** The entry fill
was real market movement inside the 77 seconds between the book read and the send. The reading that
would have poisoned every future fill measurement is refuted, and it cost one order that had to be
sent anyway.

## 7. A measurement this run did NOT take cleanly, stated so it is not mistaken for one

The apparent round trip was `-0.0727` USDT on ~8.06 notional — **0.90%**, against §2.1's ~0.29%
model. It decomposes as `0.055` of basis plus `0.0177` of Bybit fees, and `0.048` of that `0.055`
was paid at **entry**.

That entry basis is **confounded**: the four-tick buffer and the 77-second market drift both widen
it, and this run cannot attribute between them. The buffer was chosen to guarantee marketability —
and it earned its keep, since a tighter Bybit limit would have no-filled leg B against an already
short leg A — but a cost figure taken under it measures the operator's latency as much as the
market's. **0.90% is not this strategy's round-trip cost and must not be carried into a hurdle
calculation.** The clean version needs the send to follow the read inside the book's own timescale,
which is a property of an automated path and not of a runbook a person executes.

## 8. What is claimed and what is not

**Claimed, on evidence from the venues:** the code reaches two venues, places two legs, sizes leg B
from leg A's established fill, journals both without a duplicate, and reports a verdict two
independent venue surfaces corroborate. `GateNetDeltaAsserted` detects an out-of-band position
change. Bybit demo fills honestly.

**NOT claimed:** anything about mainnet fills, mainnet fees, funding accrual, or the strategy's
economics. §95 §8's list stands, and F24 adds demo *fees* to it.

**Open, and it is the next boolean worth buying:** **does either demo pay funding at all?** Nothing
this session tested it. Both venues settle LABUSDT on a 4-hour interval at `00/04/08/12/16/20` UTC
(`fundingIntervalHours: 4`; `fundingInterval: 240`), so the instants are known and they **coincide**
across the two venues — which matters, because it means no window exists where one leg has settled
and the other has not. Given that Binance demo publishes a correct fee rate and charges nothing,
"publishes funding, does not pay it" is a live possibility with precedent from this very session.

## 9. The next honest step is not another live run

Break-even at today's spread is ~6.2 settlements ≈ **25 hours** of holding (0.29% round trip against
a `0.00047` differential). Whether the strategy is viable at all is therefore a question about
**spread persistence**, not about execution — and §2.3 already says so: *"edge bar set from measured
spread persistence, not the headline spread."*

That number is measurable from settled history with no position at risk, and `screen run`'s §42b
harness already fetches both venues' settled-funding history for the intersection universe. Reading
what it produced for the 8-alt shortlist costs nothing and answers the question that decides whether
any of the rest matters.
