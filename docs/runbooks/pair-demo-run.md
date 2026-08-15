# Runbook — one cross-venue pair run on the demos

**Command:** `arblab pair run -config <file>`
**Document:** copy `configs/pair.demo.example.json` and edit it. Never run the
example itself — its trade id, its prices and its account names are all
placeholders, and two of the three are unsafe to reuse.

This drives **one** cross-venue perp-perp package across **two** deployments:
two §5.2 blocks, two journals, two reconcile-only boots, and — only if both
verdicts are clean and both legs' venues are flat — leg A, then leg B sized to
leg A's **established fill**.

---

## 1. Preconditions

- `.env` holds `BINANCE_DEMO_API_KEY`, `BINANCE_DEMO_API_SECRET`,
  `BYBIT_DEMO_API_KEY`, `BYBIT_DEMO_API_SECRET`. The document holds **refs
  only** — `platform/config`'s `SecretRef` has no field for a value and an
  inline credential fails to parse.
- ⚠ **The Bybit demo key expires `2026-11-10T07:03:17Z`** (record §95 §2).
- Both keys were confirmed to carry trade permission on **2026-08-15**. That is
  a fact with a date, not a standing guarantee: a key whose permissions were
  narrowed since then fails at the first `Place`, after the boots pass.
- `mkdir -p ./run/pair/locks/binance ./run/pair/locks/bybit`. The journal
  directories must **exist**: the document's two-journal identity check
  resolves each parent, and a path that cannot be resolved cannot be told apart
  from the other side's, so an absent directory is refused rather than assumed
  distinct.
- Write the Portfolio-Margin attestation the Binance side pins, and put its real
  SHA-256 in the document. §4.1 has no read path for it — it is the one check
  that is an operator's word — and the boot refuses by name without it.

## 2. Fetch a fresh reference revision

```
arblab reference fetch -reference <in.json> -out configs/reference.crossvenue.<date>.json
```

It must cover **both** venues' `LABUSDT` linear-perp rows: one revision serves
both sides, deliberately, because every time this root sizes a fill on one venue
as an order on the other it compares two grids, and two revisions of different
ages would put that comparison across two observation instants.

⚠ **Named limitation.** `referencePublicConfig` in `cmd/arblab/reference.go`
targets the **mainnet** public hosts (`fapi.binance.com`, `api.bybit.com`), not
the demo hosts. For Binance that is the same environment. **For Bybit it is
not** — record §95 §2 measured 821 demo symbols against 809 testnet, and demo is
a distinct environment. So a Bybit demo grid may differ from the mainnet grid
this revision records. The mismatch surfaces as a `Plan` refusal or a venue
rejection, never as a wrong fill; check the refusal against the demo
`instruments-info` before widening anything.

## 3. Read both books and set four prices

Set the two `limit_price` values **marketable** (a sell at or below the bid, a
buy at or above the ask) and the two `unwind_limit_price` values marketable in
the **opposite** direction — the unwind is a reduce-only inverse.

The prices in the example are placeholders. Record §95 §2's LAB quote was
`0.0845 / 0.08450` at one instant on 2026-08-15; a marketable limit has to come
from a fresh book read, not from a document written days earlier.

⚠ **A limit that is not marketable turns the IOC into a no-fill**, which is a
**clean** run under D6 — exit 0, verdict `VerdictUnopened` — and proves nothing
about fills. It is the most likely way to get a green run that measured nothing.

## 4. Mint a fresh `pair.id`

A UUIDv7, new for every run. §5.3 forbids `ClientOrderID` reuse inside the
recoverable window and the planner refuses a repeat — which is the fail-safe
direction: a re-run of the same document **fails** rather than double-sending.

`attempt` stays at 0 unless you are deliberately replanning; the unwind mints
`attempt + 1`, which is why the document refuses an attempt of 99.

## 5. Dry checks before the live run

```
arblab reconcile run -config <side A's oms block, as its own document>
arblab reconcile run -config <side B's oms block, as its own document>
```

Both must exit 0. If either does not, `pair run` will refuse anyway — but the
single-side command names the problem more directly, and it sends nothing.

## 6. The run

```
arblab pair run -config configs/pair.demo.<date>.json
```

## 7. Reading the exit code

| Code | Meaning |
|---|---|
| 0 | Nothing needs a human. Includes the IOC that filled nothing. |
| 1 | An operator alert: a latched gate on either side, or a venue IP ban. |
| 2 | A boot could not finish, or a leg was refused with **nothing sent**. Retry may lift it. |
| 3 | A leg was **dispatched** and its record needs a human. Recovery is `reconcile run`, never a re-run of this command with the same id. |
| 4 | **THE PAIR IS EXPOSED.** |

**Exit 4 is the one this whole root exists to be able to report.** The two
venues hold unbalanced positions, and **nothing will fix it automatically**:
§6.7 is the single gate for every compensating trade and it cannot see a
cross-venue pair at all (record §95 §4), because `domain.HedgeGroup` is
`(Account, Venue, Symbol)` and the pair sits in two groups with no group holding
both.

Recovery for a 4:

1. `arblab reconcile run` on **each** side, to establish what each venue holds.
2. Close the exposed leg **by hand**, reduce-only, on that venue.
3. Do not re-run `pair run` with the same trade id — it is refused, which is
   correct.

## 8. What a green run proves, and what it does not

**It proves:** both wire contracts against real matching engines; both signed
transports, including Bybit's per-endpoint budgets and Binance's weight folding;
both journal paths; the plan → journal → dispatch path on two venues at once;
the both-clean-and-flat gate; the establish-then-hedge sequencing; and the
paired report.

**It does not prove:** that demo books persist at other hours; that an IOC fills
on a demo alt at all; that demo funding matches mainnet intervals; that mainnet
grids match demo grids; or **anything whatsoever about the strategy's
economics**. Record §95 §8 states the same list; this section is its operational
form.
