# Record §86 — M4 slice #219: the margin read carries §7.1's sizing inputs, and the fence that permits the second decode binds in one direction

**Date:** 2026-08-09 · **Task:** #219 · **Docs:** v0.9.66 · **Campaign:** 16
mutants / 0 survivors · Branch `feat/milestone-1-foundation`.

Record §85 §1 ruled that B4's position quantity comes from the margin row. This
record is that ruling **built**, plus the four decisions the build forced which
§85 did not settle. Read §85 first — the three arguments and the rejected
alternatives are there and are not repeated.

What landed: `venue.MarginStatus` and `ports.MarginReading` each gain
`SizingQuantity` (signed, base asset) and `EntryPrice` (quote asset); both venue
codecs decode them; the OMS adapter carries them; both `Validate`s check them;
and the differential test §85 owed exists on both venues.

---

## 1. Ruling: the field is named for the use it is PERMITTED, not for the number

`SizingQuantity`, not `Quantity` and not `Net`.

§85's load-bearing half is a prohibition — the margin row's quantity may never be
fed to §7.5's net-delta invariant — and a prohibition with no carrier is one
nobody sees at the call site. `Position.Net` already exists on the surface this
field must not replace, so naming this one `Net` would make the substitution look
like a rename. `Quantity` is neutral, which is the same problem more quietly.

`SizingQuantity` states the permitted consumer in the identifier, so a §7.5
consumer writing `reading.SizingQuantity` into a net-delta assertion is visibly
doing something the name does not sanction. Both of the field's legitimate
readers — the top-up's `Δ = (L* − L)·q·(1 + MMR)` and the delever's
`q* = m/((1 + MMR)·L* − e)` — are sizings, so the name is not narrower than the
truth.

**Rejected:** `Net` (invites the substitution the ruling forbids); `Quantity`
(neutral, carries nothing); a separate wrapper type enforcing the prohibition in
the type system (the invariant lives in a different bounded context and the
wrapper would have to cross it, which is the import edge §3.6 rule 1 exists to
prevent — the prohibition is a rule about consumers, and a name plus a normative
paragraph is the only carrier available at this seam).

## 2. Ruling: the entry price is REFUSED where the liq price is FLAGGED

`marginLiqPrice` has three arms and returns a `stated` bool. `marginEntryPrice`
has two arms and returns an error. The asymmetry looked like an inconsistency and
is the opposite.

The liq price's absence is a **documented value on a healthy position** — Bybit
keeps `liqPrice` at `""` when it falls outside the symbol's band — and §7.1 has a
defined behaviour for it (the stale-input rung). The entry price has no
documented absence on either venue, arrives on every live position, and has **no
consumer that could proceed without one**. An `EntryPriceStated` flag would
therefore be a boolean whose false branch nothing reads: an unreachable sentinel
that reads as handled.

The direction decides it. Absorbed as zero, the delever target's denominator
`(1 + MMR)·L* − e` grows, so `q*` shrinks and the rung reduces **harder** than
asked, off a number the venue never stated. Refused, the position lands on the
stale-input rung, which is where an unreadable margin answer already belongs.

Two sentinels rather than one, because the diagnoses differ: an **empty** field
is a shape change, a **zero** is the value a flat row carries and therefore says
the presence test above it let a flat row through. A value that will not parse is
neither — §5.3's rule, and the test asserts it is neither.

## 3. Finding: the obvious cross-check is the liquidation condition itself

`EntryPrice` and `LiqPrice` are the same Go type in the same pair, so a
transposed assignment produces a status that validates. The guard that suggests
itself is that a short liquidates **above** its entry, and it is wrong to write.

Solve §85 §2's relation for `L` and compare:

```
L = (m/q + e) / (1 + MMR)        L > e   ⟺   m/q > e·MMR
```

`m/q > e·MMR` is margin-per-unit above maintenance-per-unit — that is, **"this
position is not currently liquidatable"**. A validator enforcing `L > e` would
therefore refuse precisely the readings §7.1's ladder exists to act on, at the
instant it needs them, and it would do so by returning an error on the path where
§7.1's own ruling (iv) says every venue-side failure must be a ladder state.

This is the family CLAUDE.md already names: a rule evaluated on a quantity that
includes its own subject. The invariant is not false — it is **equivalent to the
condition being monitored**, which makes it useless as a check and harmful as a
gate.

The transposition is covered by value assertions in tests on both venues and at
the adapter seam instead, which is the trade v0.9.60 already took for the two
money fields. The adapter's comment naming that gap is extended rather than
replaced, because this is the same gap one field-pair out.

## 4. Ruling: Bybit's margin wire declares `side`, reversing a v0.9.60 note

`bybitMarginStatusWire` deliberately omitted `side`, with a note saying "a
direction is exactly what this consumer does not need and declaring it would put
a second reading of the position one field away from the first". That note was
right when written and is now false in its first clause: §85 changed what the
consumer needs, Bybit's `size` is documented always-positive, and the sign lives
in `side` and nowhere else on the row.

The note's underlying worry survives and is answered elsewhere: what must not
happen is §7.5's invariant acquiring a second source, which is a rule about who
may **read** `SizingQuantity`, not about which keys a struct declares.

The sign rule is **shared** with §7.5's codec (`bybitSignedSize`, taking strings
rather than a wire struct so neither narrowing picks up the other's fields).
That is deliberate and it costs something the record states rather than hides:

| | Binance | Bybit |
|---|---|---|
| Sign rule | none — the field is signed | shared helper |
| Differential test covers | two independent parses of `positionAmt` | which JSON **keys** each narrowing declares |

So the differential test is genuinely differential on Binance and is a
key-selection check on Bybit. Sharing the rule **removes** a failure mode rather
than hiding one — a rule stated once cannot drift into two readings of "Sell" —
but it does mean no sign disagreement can be observed there, and the alternative
(two independent sign rules kept in step by a test) is the shared-constant defect
with extra steps.

## 5. What the collapse checklist changed, including one collapse already present

Run before the mutations, per the standing rule.

- **The entry price was the SAME number on both venue fixtures** (`64000.10`),
  which was free while neither codec read it and is a wrong-codec collapse the
  moment they both do. Now `64180.41` and `63105.61`.
- **The entry price had to differ from its neighbours on the row.** Binance
  publishes `breakEvenPrice` — entry adjusted for fees — one key from
  `entryPrice`, same magnitude and character class; Bybit puts `avgPrice` beside
  `markPrice` and `positionValue`.
- **Bybit's `side` was pinned to `Sell`**, where "apply the side" and "negate
  unconditionally" are the same function. Fixtured at Sell, Buy and the
  documented empty string; Binance's amount at both signs, as the mirror.
- **Both `Validate`s needed a LONG positive control.** Every fixture in the tree
  is a short, so a rule written "must be negative" would have agreed with all of
  them and refused every long the system holds.
- **The fixture builders became structs with mutators.** Six same-typed string
  parameters is the file's own thesis — a transposed *argument* mints a
  well-formed row exactly as a transposed *assignment* mints a well-formed
  struct.
- **No fixture constant may end in a trailing zero**, because `Decimal()`
  normalizes one away and the assertion could then not be written against the
  constant the fixture used.

## 6. The bite-check

16 mutants, 0 survivors. Two re-aims, both instructive rather than incidental:
one `NO-OP!!` from a stale line number after the file grew, and one `NOBUILD!!`
from a mutant naming `fixedpoint` in a file that does not import it — a **bad
mutant**, not a claim, re-aimed to a zero value the file could express.

The mutant worth recording is the wire-tag one: pointing the margin codec's
`positionAmt` at `notional` makes the two codecs read `-32750.15` and `-0.503`
from one payload, and the differential test names the disagreement in those
terms. That is the test doing the job §85 asked for, observed rather than
asserted.

## 7. What this record does NOT settle

- **`L*`, the target liquidation distance.** §85 §5 flagged it and it is still
  open — a top-up restoring the position to exactly `rebalance` sits on the
  boundary that triggered it.
- **The live-tier MMR** (#220). The formula's remaining unknown; the quantity
  half is what this slice delivered.
- **The delever rung's design** (#224). §221 refuted the premise that a partial
  close raises headroom, so `q*` has a formula and not yet a purpose.
- **Whether `SizingQuantity` should also serve §7.5 on cost grounds.** Answered
  normatively — it must not — and recorded here because "it is already in hand
  and looks free" is the argument a later session will make.
