# Record §85 — M4 slice B4's sizing inputs: the quantity comes from the margin row, and following that through puts B4 behind two prerequisites

**Date:** 2026-08-08 · **Task:** #218 · **Docs:** no version bump — the normative
edits land with the slices §4 schedules · **Campaign:** none, this is a ruling
record and no code landed. Branch `feat/milestone-1-foundation`.

*Numbering note:* the task store was reset by a crash on 2026-08-08 and the
#150–#217 roster was lost; the live store restarts at #1. This record continues
the **documentary** numbering every other record uses, and the store's own IDs
are offset from it. Where the two must agree, the documentary number is written
into the task's subject.

A design session, taken before any B4 code, because B4 could not be written. All
three of §7.1's rungs exist — trigger (#210), top-up (#215), ladder (#177) — and
both of the acts B4 must perform take a quantity nothing in this system computes:

| Rung | Signature | Wants |
|---|---|---|
| Top-up | `MarginOpDispatcher.Dispatch(instrument, amount fixedpoint.Money)` | how much margin to add now |
| Escalation | `Deleverer.Open(id, group, reservation, target fixedpoint.Quantity)` | an absolute, strictly positive target position |

`Dispatch`'s own doc says so in as many words — *"the amount is the caller's, and
this method does not size it"* — and names §6 step 4's
`margin-for-target-liq-distance` as the owner, adding *"nothing in this system
computes it yet"*.

---

## 0. The first thing to get right is that this is TWO questions

The block was originally recorded as one — "B4's sizing input" — and that framing
hides the harder half.

§6 step 4's `margin-for-target-liq-distance` is **entry** sizing: it bounds how
large a position may be **opened** so that its liquidation distance starts above
§5.2's `open` threshold. B4's top-up runs the other direction: given a position
already held, how much margin restores a distance that has **fallen** below
`rebalance`. Same physics, inverse solve, and the confusion matters because §12's
milestone-4 roster says sizing *"awaits B4's root"*, which reads as circular
against a B4 that needs sizing.

**It is not circular.** §12's clause is about entry sizing lacking a *caller* —
there is no trading loop to invoke it — not about it needing B4's logic. What B4
needs is the inverse solve, which §12 never assigned to anything. The apparent
circle was two different computations sharing one name.

The escalation cannot dodge its quantity either. `Generation` refuses a
non-positive target by name (`ErrGenerationTargetNotPositive`), so "flatten
everything" is not expressible through `Open`, and `EmergencyClosePackage` exists
only as a *supersession path* on a generation — a way one ends, never a use case
that opens one. A reduction toward a target position is arithmetic over the
current position, whatever the top-up above it does.

---

## 1. Ruling: the position quantity comes from the MARGIN read, widened

`ports.MarginReading` today carries `Instrument`, `LiqPrice`, `LiqPriceStated`,
`Maintenance` and `PostedMargin`, and reads the position amount as a **presence
test only** (v0.9.60, task #208). It is widened to decode the **signed position
quantity** and the **entry price**, both of which are on the row it already
fetches.

Three arguments, independent, and the third is the one that decides it.

### (i) §7.1 forbids the alternative in its own words

The escalation clause reads: *"because a lost timeout may mean the top-up
actually applied, it **re-queries margin + position first** (target-state, not
delta — a lost ack can never fake a restored liq distance)."* A quantity carried
forward from the last boot **is** the delta reasoning that clause rejects. This
is not an inference from the spec; it is the spec naming the act.

### (ii) The local projection's error is correlated with the trigger, and errs high

§7.5's `Joiner.Assert` builds a per-instrument signed quantity from the order
repository — `signedQuantity(rec.Order.Side, rec.Cumulative)` accumulated over
dispatched orders (`join.go:683`) — and asserts it against venue truth. That
number is genuinely ours, genuinely validated, and behind no fence. It is still
wrong for this rung.

The Joiner runs at **boot only**; afterwards the projection is maintained by our
own fills. So it diverges from venue truth exactly when the venue moved the
position without us — **partial liquidation, ADL, a manual close**. That is
precisely the event that co-occurs with a liq-distance breach. The number would
be least trustworthy at the only moment it is read, and it would say we hold
**more** than we do, sizing a reduction against phantom size with the two legs
clipping asymmetrically at reduce-only and the never-net-short spot cap.

This is the shape §7.1 already paid for one field over: `PostedMargin −
Maintenance` is not headroom because it is the adverse PnL, **largest exactly
when the trigger fires**. Same defect, different quantity — a value whose error
is maximised by the condition that reads it.

### (iii) Margin and quantity must describe ONE instant, and they are on one row

`marginstatus.go:21` states the shared endpoint outright: *"On Binance both this
and §7.5's enumeration read `/fapi/v3/positionRisk`"*; on Bybit both read
`/v5/position/list`. Taking the margin from `MarginStatus` and the quantity from
`VenuePositions` is **two calls at two instants**, and the sizing arithmetic then
mixes producers — the exact defect the v0.9.60 ruling exists to prevent, in that
ruling's own sentence: *"a ratio whose numerator and denominator came from two
different producers is one nothing compares."*

Applied to **this** question that principle points the opposite way from where it
pointed on the mark, and the asymmetry is principled rather than convenient: the
mark has a dedicated producer with its own §5.2 budget and its own silence timer,
so decoding a second one off the margin row would create a rival. The position
quantity has **no** producer on this path at all, and the row that carries the
margin carries it.

### What the fence actually says, and what is owed alongside

v0.9.60's rule is *"the two consumers get two narrowed codecs over one venue
answer, each projecting its own fields"*. Widening the **margin consumer's own**
codec is that rule working, not that rule breached — §7.5's net-delta invariant
keeps `ports.VenuePositions` as its single source, unchanged and untouched by
this slice.

What must be recorded explicitly, because it is now the load-bearing half of the
rule, is the prohibition in the **other direction**: the margin row's quantity
**may never be fed to the net-delta invariant**, and no future consumer may
substitute it for `VenuePositions` on cost or convenience grounds. The invariant's
value is that it joins two authorities that can disagree; a second decode of the
same field is not a second authority.

And because two decodes of one field is a real hazard, the slice owes a
**differential test**: one captured venue payload fed to both codecs, asserting
the quantities agree. It is free — the endpoint is shared — and it is this repo's
`math/big`-oracle habit arriving at no cost.

### Alternatives rejected

| Rejected | Why |
|---|---|
| **Local projection from the order repository** | Ruled out by §7.1's *"re-queries … target-state, not delta"*, and by (ii): stale exactly when read, wrong in the permissive direction. |
| **Read `ports.VenuePositions` from the safety path** | Fresh, but a whole-account enumeration per breach under stress, at a second instant from the margin figures. Worse, it puts a second consumer on the surface deliberately kept un-narrowed so foreign positions stay visible — the first cost pressure on the safety path becomes pressure to symbol-scope it, which destroys that property. |
| **Configured absolute delever target per instrument** | Buildable today and meaningless: a target above the current size runs the entire ten-phase ladder and reduces nothing, and no configured constant can know where the position is. |
| **Configured pre-funded rescue tranche for the top-up** | Tempting, since §7.1 does describe rescue margin as capital *"staged at the venue but outside the position"*. But that clause is about **where the capital sits**, not about how much of it one op posts. A fixed tranche that fails to clear the band re-fires for ever, so it needs a per-episode attempt bound — a bound that exists only to compensate for not having computed the number. Rejected as the tractable answer rather than the correct one. |

---

## 2. Consequence: the top-up amount is COMPUTED, and precision needs the live-tier MMR

For an isolated short at liquidation price `L`, with posted margin `m`, quantity
`q`, entry `e`, maintenance-margin rate `MMR` and the tier's deduction `cd`:

```
m + (e − L)·q  =  MMR·q·L − cd
```

Adding `Δ` to reach a target liquidation price `L*` and subtracting the two
states, **`cd` and `e` both cancel and `MMR` does not**:

```
Δ = (L* − L) · q · (1 + MMR)
```

That the entry price and the tier deduction difference away is what makes the
top-up expressible in observables at all. `MMR` surviving is what makes it
depend on a read no code performs.

**Every shortcut under-sizes, and under-sizing is the unsafe direction.** The
tempting move is to recover `MMR` from the published `Maintenance` figure, but
`Maintenance = MMR·q·mark − cd` with `cd ≥ 0`, so `Maintenance/(q·mark)` is a
**lower** bound on `MMR`. A lower `MMR` yields a smaller `Δ`, so the top-up posts
too little and the position stays inside the band that triggered it — with the
ladder then escalating on a remedy that was merely mis-sized. At the deep tiers
the shortfall is on the order of 12%.

So §7.1's *"MMR is always the live tier value (`leverageBracket`)"* and §6 step
4's *"liq distance from the live tier MMR `[→ C4]`"* are load-bearing rather than
aspirational. **The live-tier read is a prerequisite to B4, not a follow-on** —
Binance `GET /fapi/v1/leverageBracket` (signed), Bybit's risk-limit equivalent,
each with the per-venue formula instantiated rather than assumed common.

**The model is checkable in production, and that is worth building in.** After a
top-up applies, the venue publishes a new liquidation price on the very row the
dispatcher already re-reads. The predicted `L*` and the observed one can be
compared, which turns the sizing formula from an unverifiable assumption into a
measured claim — the alternative to a paragraph asserting the arithmetic is
right, in the shape `P6` took in the formal models.

*Assumption stated rather than absorbed:* the relation above is the isolated
linear-perp liquidation condition ignoring fees and pending funding. Both are
small next to a rebalance-band move, both push `L` in the adverse direction, and
the production check above is what would surface it if the omission mattered.

---

## 3. Finding: the escalation rung's premise is an UNVERIFIED VENUE FACT

Solving the same relation for the liquidation price alone:

```
L = (m/q + e) / (1 + MMR)
```

`L` depends on `m/q` — margin per unit — **and on nothing else that a reduction
changes**. So if a partial reduce-only close on an isolated position **releases
posted margin pro rata** (`m' = αm` alongside `q' = αq`), then `m'/q' = m/q` and
the liquidation price is **exactly unchanged**. A `PackageDelever` would cap the
size of the eventual loss and raise **no liquidation headroom whatsoever**.

That would make §7.1's parenthetical — *"`PackageDelever` toward reduced notional
(raising liq headroom)"* — false on the safety kernel's terminal rung, and it
would make the whole escalation the wrong remedy for the trigger that fires it.
If instead the margin stays posted, headroom improves by `1/α` and the rung works
as the spec describes.

Which behaviour holds is a **venue fact on each venue**, `docs/venue-profiles.md`
takes nothing unverified, and the delever target cannot be sized until it is
settled — the target formula itself is contingent on the answer:

```
margin retained:      q* = m / ((1 + MMR)·L* − e)      ← needs the ENTRY PRICE
margin released:      no q* exists; the rung must be re-designed
```

Note the second input the retained case needs: **`e`, the entry price**, which
does *not* cancel here as it did in §2's difference. It is on the same row —
Binance `entryPrice`, Bybit `avgPrice`, both already present in
`marginstatus_test.go`'s own fixtures and decoded by neither codec — which is a
third, independent reason the widened margin read is the right surface: **every
input either sizing needs is on the row the dispatcher already fetches.**

---

## 4. What B4 now waits on

B4 is later than §12's roster assumes, and the ordering is forced rather than
chosen:

1. **#219 — widen the margin read** (signed quantity + entry price on both
   codecs), the §7.5 single-source restatement in the other direction, and the
   shared-payload differential test.
2. **#220 — the live-tier MMR read** (`leverageBracket` / Bybit risk limit) and
   the inverse solve `Δ = (L* − L)·q·(1 + MMR)`, with the post-op predicted-vs-
   observed liquidation-price check.
3. **#221 — verify the isolated partial-close margin-release behaviour** on both
   venues, into `venue-profiles.md`. Blocks the delever target, and may reopen
   §7.1's escalation design.
4. **#222 — B4 itself**, which by then has both quantities and can be the loop it
   was always meant to be.

(1) and (3) are independent and can run in either order; (2) depends on (1) only
for the quantity, and (4) on all three.

---

## 5. What this record does NOT settle

- **The target distance `L*`.** §5.2 has four thresholds, and a top-up restoring
  the position to exactly `rebalance` sits on the boundary that triggered it, so
  the next tick re-fires. The target is presumably `alert` or `open`, and picking
  it is a policy question for the sizing slice — flagged here so it is not
  discovered as a loop.
- **What happens when the staged float cannot cover `Δ`.** §7.1 names
  *"insufficient staged margin"* as one of the escalation's triggers, so the
  path exists; whether a partial top-up is dispatched or the rung is skipped
  outright is undecided, and the two differ on whether a partial remedy consumes
  the one-op-per-position latch.
- **Bybit's liquidation formula.** The algebra above is Binance's isolated linear
  form. Bybit's risk-limit mechanism is the same shape and not the same
  expression, and this record asserts nothing about it beyond the shape.
- **#216**, the liq monitor's unwired module branch, which is independent of all
  of the above.
