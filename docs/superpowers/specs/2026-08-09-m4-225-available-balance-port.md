# Record §94 — task #225: the available-balance PORT, and the venue whose own engine is the definition

**Date:** 2026-08-09 (session 138) · **Task:** #225 (port half) · **Milestone:** 4
**Predecessors:** record §93 (the measurement half, same task — §8 states what the port must be),
record §70d / task #179 (`SpotBalances`, the sibling read port), record §79 / task #208
(`MarginStatus`, which rules this quantity OUT of the position surface), record §85 §1 / task #219
(the one-directional fence over a second decode)
**Status: DESIGN. No code in this record.** The implementation plan follows it.

---

## 1. What is owed, and what §93 already settled

Record §93 closed #225's measurement half and fixed the port's semantics in its §8: per settlement
asset and never an aggregate; one construction with the provenance visible in the type; Binance's
derivation inputs; tests asserting on wallet and margin; and Bybit's `""` as a refusal rather than a
decoded zero. Four alternatives were rejected there and are not reopened here.

What §93 §8 did **not** fix, because a measurement cannot: which bounded context owns the port, what
the Go surface is, and — the one that turned out to matter — whether the answer on Binance is our
subtraction or the venue's own published figure.

This record settles those three and **amends §93 §8 in one place**, marked in §4 below rather than
absorbed.

---

## 2. RULING — the port lives in `oms/application/ports`, and the rule is that placement follows the
## CONSUMER

Core §4's context table gives `treasury` "Balances, available margin, collateral transfer, inventory
drift, custody policy, **pre-funded rescue margin**", and core §4's `MarginStatus` paragraph
(line 435) says of this exact quantity that it "is already owned elsewhere: §7.2 gives `treasury`
'balances … and the rescue-margin float'". Read literally, that assigns this port to a context with
**no directory**.

It is not read literally, and the precedent is already in the tree: `SpotBalances` — which §7.2's
same sentence would claim just as well — lives in `oms/application/ports/balances.go`. It is there
because its consumer is §7.5's reconcile assertion, an OMS boot use case. The rule that explains
both is **placement follows the consumer**, and this port's consumer is §7.1's re-post decision,
which is OMS.

What `treasury` retains is what its table entry is actually about and what no OMS use case wants:
the float ledger (§7.1's staged-float reconciliation, still owed *with* its consumer per record §82),
the per-venue concentration cap, the inventory-drift monitor, and custody policy.

**The deviation is marked, not absorbed.** Core §4's line 435 asserts an ownership this slice
departs from, and the departure is a `[SPEC EDIT OWED]` the implementation carries: the sentence
must be re-worded to assign the *quantity* to treasury's concerns while placing the *read port* with
its consumer, or the placement rule must be stated once and cited from both sites.

**The expiry trigger, named rather than left as "nothing needs it yet":** the day `treasury` exists
with a float ledger that reads availability for a purpose that is not a re-post — a concentration
check, a drift sweep — this port has two consumers in two contexts and its home is re-derived. That
is the same trigger shape §70d used for `SpotBalances`' priority carrier.

---

## 3. RULING — the read is KEYED by settlement asset, and the venue call stays unscoped

    SettlementAvailability interface {
        Available(AvailabilityScope) (Availability, error)
    }
    AvailabilityScope{ Venue ids.Venue; Asset fixedpoint.Asset }

`SpotBalances` is deliberately **unscoped** by asset — `balances.go` states it in as many words,
because that port's whole premise is surfacing a coin nobody expected, and a per-asset query is
exactly the read that cannot. **That argument does not transfer.** This port's caller is asking
whether the account can fund a re-post in a settlement asset it has already chosen; there is no
unexpected coin to surface, and handing the caller a set it must then index is how a caller ends up
holding the aggregate §93 F3 exists to prevent.

The **venue call** is still unscoped by asset — Bybit's coin-less call and Binance's account read
both answer for every coin, and Bybit's dialect makes the parameter load-bearing (Profiles §4). Only
the port's **answer** is narrowed. This is `marginLiqPrice`'s split one surface over: the wire shape
is the venue's, the narrowing is the consumer's.

---

## 4. RULING — on Binance the VENUE'S published per-asset figure is the answer, and this AMENDS
## §93 §8

§93 §8 point 2 wrote "Binance **may** return its published `availableBalance`, which F5 measures as
exactly that expression", leaving the choice open, and points 3–4 were written as though the
derivation were the answer on both venues. It is not, and three things decide it.

**(i) The venue's own engine is the definition for a quantity its engine acts on.** Available
balance is what Binance's **order-admission** check will itself evaluate when the re-post arrives.
`MarginStatus.LiqPrice` already carries this exact ruling — *"It is the VENUE's number and never one
this system reconstructs … §7.1 computes the distance the liquidation engine will act on, so the
engine's own figure is the definition."* If our subtraction and Binance's published figure ever
disagree, the venue's is operative and ours is an opinion.

This does **not** reopen F3. `totalAvailableBalance` was rejected because it answers a *different*
question — a USD, multi-coin, collateral-filtered aggregate. The **per-asset** `availableBalance` on
the `assets[]` row answers exactly this question, per settlement asset, and F5 measured it doing so
across all five phases including the add-margin.

**(ii) The derivation, exactly as measured, spans TWO responses.**
`probe225_pass3_separate.py:332-341` takes `walletBalance` and `openOrderInitialMargin` from
`GET /fapi/v3/account` and `isolatedWallet` from a separate `GET /fapi/v3/positionRisk`. So §93 §8
point 3's identity was established across two calls at two instants — which is the objection this
codebase has already made twice, in `marginstatus.go` (*"a ratio whose numerator and denominator came
from two different producers is one nothing compares"*) and in record §85 §3(iii) (*"margin and
quantity must describe ONE instant"*). Bybit has no such problem: `walletBalance`,
`totalPositionIM` and `totalOrderIM` are three fields on one coin row of one response.

**(iii) The attribution the derivation needs fails in the UNSAFE direction.** A per-asset derivation
on Binance must attribute each position's `isolatedWallet` to a settlement asset, because
`isolatedWallet` is on the position row while the answer is per asset. An unattributed position
**drops a subtrahend**, which overstates available — the direction that funds an order the account
cannot pay for, and the same direction F3's error ran. Letting the venue attribute removes that
hazard from the answer path entirely.

**So:**

| | Binance | Bybit |
|---|---|---|
| Call | `GET /fapi/v3/account`, the `assets[]` row for the asset | `GET /v5/account/wallet-balance?accountType=UNIFIED`, the coin row |
| Answer | published `availableBalance` | `walletBalance − totalPositionIM − totalOrderIM` |
| `Provenance` | `AvailabilityVenueStated` | `AvailabilityLocallyDerived` |

§93 §8 point 2's requirement — **the provenance difference is visible in the type** — is honoured
exactly, and by this ruling it becomes a statement about what the number *is* rather than about
which formula happened to run. Point 3 stands unaltered as the specification of the **corroboration**
owed in §7 below; its warning against the `positionInitialMargin` name-match is why that corroboration
must be built from `isolatedWallet` when it is built. Point 4 stands for Bybit, which is where its
premise — the identity has no independent witness — is true.

### The cross-check is NOT in this slice, and dropping it is a ruling

An earlier form of this design ran the derivation on Binance as a live oracle against the published
field, refusing on disagreement. It is dropped, for a reason that outranks the evidence it would add:
**the check's failure direction is worse than the failure it guards.** A single-response derivation
would have to read `account.positions[]`, an array no capture contains; a wrong assumption about its
shape produces a spurious disagreement, which refuses the re-post, which escalates §7.1's ladder into
a **delever of a healthy position**. Building a refusal gate on an unmeasured shape trades a measured
hazard for an unmeasured one.

It also removes a field that would have had no producer. A `Corroborated bool` set by nothing is the
declared-member-nothing-emits shape: compiler green, `exhaustiveswitch` green, coverage green, and
the state never reported. The corroboration is filed as an owed measurement in §7, and the field
arrives **with** it.

---

## 5. The type

    type Availability struct {
        ObservedAt clock.ServerTime
        Asset      fixedpoint.Asset
        Available  fixedpoint.Money        // tagged with Asset; there is no account total in this type
        Provenance AvailabilityProvenance  // venue-stated vs locally derived; zero value refused
        Words      []VerbatimField         // the venue's own fields, verbatim
    }

`Provenance` is an enum-like in the shape `AbsenceSemantics` established (`balances.go:154-204`): a
sentinel zero refused by `Validate`, a `String`, and `exhaustiveswitch` forcing every consumer to
name both members. It is carried as a field rather than derived from the venue for `AbsenceSemantics`'
own reason — the discriminating fact must have a carrier, and a fact that is per-venue-constant
*today* is not one by definition.

`Words` carries each venue's inputs verbatim beside the typed answer, on `AssetBalance.Words`'
precedent (`balances.go:86-88`): it is what lets the corroboration slice work from stored evidence
instead of standing up a second decoder.

### Refusals — each names the field or the asset, and none returns a value

- **An empty string on any input field.** §93 §8 point 5. Bybit serves `""` on this surface today
  under the pinned posture (§93 F1, F2), so this is live rather than defensive. Decoded as zero it
  says "no cash", which refuses the re-post — the safe direction — but silently, attributing a
  venue's silence to the account.
- **The asked-for asset absent from the response.** Deliberately **not** `AbsenceIsZero`: that rule
  was earned for a holdings *enumeration*, where Bybit's non-zero-only answer makes silence
  meaningful. Here the caller named an asset it intends to spend, and a venue that did not mention
  it has not said the account holds nothing.
- **A negative input** (`walletBalance`, `totalPositionIM`, `totalOrderIM`). Neither venue publishes
  one, so a negative is a parse that read a sign off the wrong field — `marginstatus.go:375`'s
  `requireNonNegative`, one surface over.
- **A negative derived result on Bybit.** Unmeasured, and "committed margin exceeds the wallet" is a
  §9-class observation rather than a routine "no". Refusing costs nothing real: an account in that
  state cannot re-post anyway.
- **An unanswered read is never a zero availability** (§5.3's absence rule).

### `Validate`

Refuses the zero value, as every constructed `platform/` type does — an unpopulated `Available`
reads as "no cash" and an unpopulated `Provenance` is a wiring that never chose. It also checks
`Available.Asset() == Asset`: the substitution available at every call site is the venue's own coin
spelling standing in for the canonical asset, and a mistagged `Money` validates perfectly until it
meets a differently-tagged one inside the consumer. Same trade `MarginStatus.Validate` takes.

### Rate-limit class is PINNED to `Safety`

There is exactly one caller and it sits inside §7.1's remedy episode. `SpotBalances` takes its
priority at construction *because* §70d found it had two call sites on different rungs; this port
does not, and a knob with one setting is an unchecked claim — `MarginWriteCodec`'s situation, and
its ruling. **The expiry trigger goes in the field's own doc:** the day a second caller reads this
at a different rung, the class becomes a constructor argument that refuses the zero value.

---

## 6. THREE COLLAPSES IN THE CAPTURED EVIDENCE, and the fixtures that separate them

The captures are real and the numbers are right. What they cannot do is tell several candidate
implementations apart, and all three cases are invisible unless looked for — the thirteenth,
fourteenth and fifteenth entries on the collapse checklist, and the first set found in *evidence*
rather than in a test.

**(a) `posIM` and `orderIM` are never both non-zero — on EITHER venue.** No phase of either probe
held an open position and a resting order at the same time. With one term always zero, **sum and max
are the same function**, so Bybit's `walletBalance − totalPositionIM − totalOrderIM` is unverified
as a *combining rule*. On Binance the same gap makes §93 §8's formula and a per-asset-only
alternative (`crossWalletBalance − openOrderIM`) agree in all five phases, because
`walletBalance − crossWalletBalance` happened to equal `isolatedWallet` exactly in the only two
phases with a position.

**(b) The per-asset and top-level `availableBalance` are IDENTICAL in all five Binance phases**,
because USDT is the only funded asset on the demo account. So reading F3's account aggregate instead
of the `assets[]` row — the precise error §93 exists to prevent — is invisible in the evidence §93
produced.

**(c) The asked-for asset is the only row.** "This asset's row" and "the first row" are one
predicate throughout.

Fixtures, therefore:

| Collapse | Fixture that separates it |
|---|---|
| (a) sum ≡ max | both terms non-zero **and unequal**, so sum, max and either alone are four distinct answers |
| (b) per-asset ≡ aggregate | ≥2 funded coins with **different** availability, and a top-level total equal to neither |
| (c) row ≡ first row | the asked-for asset is **not** first in the array |

Plus a `""` fixture per field, an absent-asset fixture, a negative-input fixture, and a Bybit fixture
whose derived result goes negative.

Every claim is then bite-checked with `tools/mutate.sh` on the decode lines. Because the refusal
rules are **consumer-owned and stated once**, breaking one must fail *both* venues' tests — that is
the mirror-rule check, and a break that fails only one venue means the rule got copied instead of
shared.

---

## 7. What this slice does NOT establish — recorded, not absorbed

1. **Binance single-response corroboration.** Capture `account.positions[]` and confirm
   `walletBalance − (Σ isolatedWallet + openOrderInitialMargin)` reproduces the published per-asset
   `availableBalance` from **one** response. Until then the derivation on Binance is measured only
   across two calls, and `Corroborated` does not exist as a field.
2. **The separating phase.** A position and a resting order open **simultaneously**, on both venues.
   This is the fixture collapse (a) above, taken against the venues rather than against a test: it
   separates sum from max on Bybit and separates the two candidate Binance derivations. It is cheap
   — one extra phase in the existing probe — and needs credentials.
3. **Bybit's combining rule ships UNVERIFIED**, on the conservative reading. Summing yields the
   smaller available balance, which refuses re-posts rather than funding one the account cannot pay
   for. Chosen for its direction, not because the evidence picked it.

Neither 1 nor 2 blocks the port. Both are filed against #235, which already holds "the two things
#225's runs could not separate"; this record makes it four, and the count is stated because an
enumeration's count word is a claim.

---

## 8. Rejected alternatives

**Stand up the `treasury` context now and put the port there.** Rejected: core §4's table is about
the *quantity* and treasury's real members are the float ledger, concentration cap, drift monitor
and custody policy — none of which this slice builds. A context whose only member is one read port
models an ownership no use case exercises, and record §82 already ruled that treasury's float ledger
is owed **with its consumer** rather than before it. The `SpotBalances` precedent decides the rest.

**Put it in `risk/application/ports`, beside `AccountPostureSource`.** Rejected: §7.2 does not assign
it there and the consumer is not in `risk`. Availability is not a policy input; it is an account fact
the re-post use case reads.

**Enumerate every settlement asset, mirroring `SpotBalances` exactly.** Rejected: that port's
unscoped enumeration exists to surface a coin nobody expected, which is a *baseline audit's* concern.
A spend decision names its asset by construction, and returning a set the caller must index is a step
toward holding the aggregate.

**Enumerate, plus a keyed convenience lookup.** Rejected harder: two ways to ask one question is
exactly how a caller reaches the aggregate by accident.

**Derive on both venues and ignore the published Binance field.** Rejected on §4(i): it discards a
venue-stated number — the one the venue's own admission engine will evaluate — in favour of our
subtraction, which is the direction this codebase refuses for the liq price and the mark.

**Derive on both venues and refuse on disagreement with the published field.** Rejected on §4's
sub-section: it is more evidence, and it was the recommendation until the probe was read closely. The
derivation available inside one response reads an array no capture contains, so the gate rests on an
unmeasured shape whose failure **delevers a healthy position**. Deferred to owed measurement 1, at
which point it becomes cheap and correct.

**Smooth the provenance away behind one port type** (§93's own rejected alternative, restated because
this design makes it *more* tempting, not less): under this ruling the two venues now differ in what
the number **is** — venue-stated versus locally derived — and not merely in which formula ran. The
type keeps the distinction.

---

## 9. Spec edits owed

All three land with the code, marked `[SPEC EDIT OWED]` until they do:

1. **Core §4, the `AccountSource` block and the line-435 paragraph.** §4 declares
   `Balances() ([]Balance, error)` — a holdings enumeration — and nothing that answers "what may this
   account spend in asset X". The new surface must be declared, and line 435's assertion that the
   quantity "is already owned elsewhere" must be re-worded to separate the quantity's owner from the
   read port's home (§2).
2. **Core §7.2**, so the placement rule is stated once and cited, rather than contradicted twice.
3. **Core §4 line 435's LOCATOR names the wrong field, and it is the aggregate.** The sentence reads
   "Binance publishes `availableBalance` **at the top level** of `GET /fapi/v3/account`, beside the
   positions array rather than inside it". True of the response, and wrong as a pointer for this
   port: the top-level figure is an account total, and on an account funded in more than one
   settlement asset it answers F3's question rather than this one. The port reads the **`assets[]`
   row for the settlement asset**. The sentence exists to argue that the quantity is not a function
   of a `VenueInstrument`, which it establishes either way — so this is a locator defect inside a
   correct argument, the kind only found by going to the field.

   It is also where §6(b)'s collapse could have done real harm: on the demo account the two fields
   are byte-identical in every captured phase, so a port built by following this sentence would have
   passed against all the evidence §93 produced, and been wrong by the whole second leg the first
   time the account held one.
