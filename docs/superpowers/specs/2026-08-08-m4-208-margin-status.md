# Record §79 — `AccountSource.MarginStatus`, and the word §4 used names a quantity neither venue publishes

**Session 123, 2026-08-08. Task #208 (milestone 4, slice B1 part 2).**
**Docs v0.9.60. 32 mutants, one real survivor (closed), one `NOBUILD!!` and two
`NO-OP!!` re-aimed. `make ci` green uncached on both modules, `SPEC EDIT OWED`
grep zero.**

Record §78 re-scoped this half before it was built: #207 had filed it as *"un-narrow
`binancePositionWire`"*, and §4 line 331 already declared `MarginStatus` as its own
port. That correction held — the position codec's narrowing stands, and this is a
second codec over the same venue rows. What the build then found was one layer in:
**the declared port's own wording names a quantity that does not exist at the
granularity the port is keyed on.**

---

## 1. "Available margin" — three readings, and the venue publishes the one nobody meant

§4 line 331, verbatim before this session: *"available margin, maintenance, live
liq price"*. Maintenance and the liquidation price are published per position by
both venues. The first is not, and the three things it could mean come apart
exactly where this trigger fires.

| Reading | Where it actually lives | Verdict |
|---|---|---|
| the account's FREE balance | Binance `availableBalance`, **top level** of `GET /fapi/v3/account` | not a function of a `VenueInstrument`; §7.2 already gives `treasury` "balances … and the rescue-margin float" |
| the position's MARGIN BALANCE (posted + uPnL) | published by **neither** venue | would be a subtraction this system performs where the venue's computation is the definition |
| the position's POSTED margin | Binance `isolatedWallet`, Bybit `positionIM` | **this** — both venues publish it, and it is named for what it is |

The signature settles the first: a free wallet balance is not keyed on an
instrument, and Binance's own connector puts `availableBalance` *beside* the
positions array rather than inside it — the layout is the argument.

The second is settled by what a venue states versus what this system would
invent. But the reason to be exact about the naming is sharper than either:
**the difference between "posted" and "margin balance" is unrealized PnL, which
is the quantity a rally moves against the short and is largest at the instant the
threshold breaches.** A caller reading `posted − maintenance` as headroom
overstates it by precisely the adverse PnL. So the field is `PostedMargin`, the
prohibition is written on the field, and §4 now carries it.

**The tell was in the venue's own deprecation notice.** Bybit's `positionBalance`
is the field whose NAME matches §4's retired wording, and it is documented
`Deprecated, can refer to positionIM`. A codec written from the spec's word would
have reached for the retired field first. That is
[[a-field-read-as-a-property-it-does-not-carry]] with the conflation living in the
**spec** rather than in a derivation — and the falsifier was, as usual, a comment
already published: the venue's own one-word annotation.

### Why the pairing was already decided elsewhere

`isolatedWallet` rather than `isolatedMargin` is not a coin flip. §4's margin-op
evidence rule already resolves a Bybit top-up by *"the `positionIM` delta vs a
before-state"*, so the read side and the evidence side must name **one** field or
a before/after comparison compares two quantities. On Binance the same duty picks
`isolatedWallet`, because a top-up is the only thing that moves it —
`isolatedMargin` moves with price as well, so evidence taken from it is confounded
by the market event that provoked the op. B3's `MarginOp` inherits this rather
than re-deciding it.

## 2. Both venues have a documented "no liquidation price", and both spellings compute

Bybit, verbatim: `liqPrice` *"keeps `""` when liqPrice <= minPrice or liqPrice >=
maxPrice"*. That is the **well-margined** position — a short whose liquidation
sits above the symbol's price band — so the absence arrives on the healthy path,
not on a failure. Binance documents no absence spelling at all, which makes any
degenerate value it sends unlabelled.

The failure direction is identical and it is the worst one available: §7.1's
distance is taken against a **short**, whose liquidation sits ABOVE the mark. A
liq price of zero sits far below it, and every formula over that pair reports
maximum headroom. The trigger does not fail — it retires.

So the rule has three arms and is stated **once**, shared by both codecs
(`marginLiqPrice`), because the question "may §7.1 divide by this" is the
consumer's and not a venue's:

1. the empty string is Bybit's documented absence;
2. a value that parses **non-positively** is absence too — conservative, on a
   spelling no venue documents. This is the trade the codebase already took for
   Bybit's assumed empty-list absence: *"the unverified branch fails closed"*;
3. anything that will not parse is an **ERROR**, because an unreadable answer is
   not an absence (§5.3) — and that arm is what stops arm 2 swallowing a venue
   that changed its format.

`[[a-documented-zero-meets-zero-is-unusable]]` inverted: there an external
authority published a legitimate zero; here two authorities publish two different
spellings of *nothing*, and one of them is a zero that would validate.

**What is deliberately NOT encoded** is Bybit's reason for its absence ("outside
the price band" — which really does mean far from liquidation). The port reports
absence and the contract says a false is **never** "far from liquidation": no
distance is computable, which is §7.1's stale-input rung. Acting on the venue's
reason is a B2 ruling about what the ladder does with an unstated liq price, and
it is filed rather than absorbed (task #209).

## 3. The V3-vs-V2 choice is re-derived, and lands on V3 for a DIFFERENT reason

Profiles §9 records the rule the expensive way: *"the decision is PER QUESTION"*,
learned when §4.1 inherited §7.5's V3 choice and V3 turned out not to publish the
field it wanted. This is the third consumer to reach that endpoint, so the
argument is made again from scratch.

§7.5 enumerates with **no symbol** and needs a missing row to mean flat — only V3's
*"only symbol that has position or open orders"* gives that. This call passes a
symbol, where **both** versions answer, so that argument does not apply at all.
What decides it is the degenerate case: V2 answers a flat symbol with a **row of
zeros** — liquidation price 0, maintenance 0, isolated wallet 0 — and every one of
those is a number §7.1 computes with. V3 answers with an empty array, which is an
absence.

Same endpoint, same version, a different reason, and the reason is the one that
would have been lost by inheriting. `[[an-endpoint-version-is-a-per-question-choice]]`.

**A consequence fell out that the V3 choice's own documentation names and this
consumer had to act on:** V3 returns a row for a symbol with *"position **or open
orders**"*, so a resting order on a closed position produces a row whose
`positionAmt` is 0. A row is therefore **not** a position, and presence is decided
from the amount rather than from the row count — on Binance. On Bybit it is the
opposite: *"If `symbol` passed, it returns data regardless of having position or
not"*, so flat is a row with `size: "0"` and an empty list is not the documented
answer at all. **The two venues spell "no position" oppositely, and §7.5's
enumeration trained the reading that an absent row means flat** — true there, and
saying nothing about this call.

## 4. Two codecs over one venue answer, and the mark is the corollary

Record §78 §5's ruling built: the position codec is **not** widened. §7.5's
net-delta invariant must keep a single source for the position quantity, so this
surface reads the amount as a **presence test** and carries no quantity out.

The corollary is the one that would be got wrong, and it is the same shape §78
settled through a shared *frame* rather than a shared *endpoint*: **`markPrice`
is on both venues' position rows and is decoded from neither.** The mark is
§3.4's own class with its own budget, its own silence timer and its own port. A
second producer inside the account surface would give §7.1's ratio a numerator
and a denominator from two different clocks, with nothing anywhere comparing them.

## 5. The one finding from the campaign

**`SURVIVED` → closed:** deleting the adapter's `reading.Validate()` broke no
test. Every fixture handed the adapter a status the engine itself would have
accepted, so the port's own checks were never exercised at the seam that runs
them.

Diagnosing it produced the more useful half. The reader is an **interface** — the
real client validates before returning, and nothing about the seam requires that
of whatever else satisfies it. And the comment beside the call **overstated** what
it catches: it claimed the field-for-field copy made a transposition visible here,
and it does not — `Maintenance` and `PostedMargin` are both non-negative and both
in the margin asset, so each is legal in the other's slot and only a value
assertion can see the swap. What Validate genuinely earns its place on is
`LiqPriceStated`: losing the flag while keeping the price leaves a reading the
**engine's** own status still validates, and the flag is what every consumer
branches on. Both the comment and the test were fixed.

`[[a-gates-refusing-path-is-the-untested-one]]` one layer out: not a refusal
nothing reaches, but a check whose every input came from the one implementation
that cannot fail it.

## 6. Rejected alternatives

| Rejected | Why |
|---|---|
| widening `binancePositionWire` for the liq price | §4 (and record §78 §5) — the port is declared separately, and §7.5 must not get a second source for the position |
| `AvailableMargin` as the field name, filled from `isolatedWallet`/`positionIM` | §1 — the two readings differ by unrealized PnL, which is exactly what moves during the trigger's own event; a name that invites `posted − maintenance` as headroom |
| Bybit `positionBalance` for the posted figure | the venue documents it `Deprecated`, and it is the field whose NAME matched §4's retired wording — the trap, not the answer |
| `isolatedMargin` for the Binance posted figure | §1 — it moves with price, so the margin-op evidence rule taken from it is confounded by the move that provoked the op |
| deriving the margin BALANCE as posted + `unRealizedProfit` / `unrealisedPnl` | both venues publish the components and neither publishes the sum; the standing rule is that the venue's computation is the definition for anything its liquidation engine acts on |
| a `*fixedpoint.Price` for the liq price instead of a value + `LiqPriceStated` | the codebase's optional shape is (value, bool), and Validate can then refuse a populated price beside a cleared flag — a pointer makes the contradiction unrepresentable but also makes every consumer nil-check a field that is absent on the HEALTHY path |
| reading Bybit's `""` as "far from liquidation" | the venue's reason is real, but acting on it is a §7.1 ladder ruling about an unstated liq price, not a decoder's; filed as #209 |
| `(MarginStatus, error)` per §4's declared line | a flat instrument would be the zero struct, whose every field computes — a maintenance of 0 reads as a position requiring nothing. §4 gains the presence bool instead |
| reusing `bybitPositionListWire` for the envelope | Go's decoder is structural, and these are two narrowings of the SAME rows — the case where a mistaken reuse looks most correct |
| pinning the urgency inside `OrderClient.MarginStatus` | a periodic risk sweep is a legitimate `Routine` caller; the **adapter** is the Safety one, and a second caller gets a second adapter rather than a parameter every site copies |
| wiring the adapter into `oms.Module` now | record §78's deferral, unchanged and now doubled: neither this nor `markstate` has a consumer until B2. The trigger names itself — **B2's liq-distance monitor**, which needs both |

## 7. The campaign

**32 mutants, all caught** after one real survivor and three re-aims.

- **`SURVIVED` → fixed:** §5 above.
- **One `NOBUILD!!`, the bad-sed case** ([[nobuild-verdict-read-the-compiler-error]]):
  `if !held {` → `if false {` orphaned `held` ("declared and not used"). Re-aimed
  as `if !held && false {`, which keeps every variable used and expresses the real
  claim — that the flat-instrument return comes **before** `Validate`, because the
  zero status is precisely what "the zero value is unusable" refuses. Two rules
  each right on their own, composing into a refusal unless the order is this one.
- **Two `NO-OP!!`**, both stale line numbers after an edit. Bookkeeping.

**The collapse checklist ran first and paid for itself twice.** The zero-margin
fixture was added *before* the campaign, from the checklist alone — and the
mutant `sign < 0` → `sign <= 0` on `requireNonNegative` would have SURVIVED
without it, because every other fixture sat strictly inside the bound where the
two predicates agree (`ratelimit`'s `freeAt` shape, one package over). The
Portfolio-Margin refusal was likewise fixtured one field at a time, because
emptying both `positionMM` and `positionIM` makes `||` and `&&` the same
predicate.

Both venues also got a **full-shape** fixture carrying every field the venue
really publishes — twenty on Binance, all of Bybit's including the three
deprecated ones — with the undeclared neighbours holding values that would be
WRONG if they landed in a declared field. A narrowed wire struct is a claim about
which keys it does *not* pick up, and a fixture containing only the declared keys
cannot test that claim (§4's wire-tag seam rule, record §78 §3).

## 8. What is not delivered, and it is named

- **No consumer.** B2 — the liq-distance monitor that reads this port beside
  `MarkRiskReader`, computes the §5.2 distance and escalates — is the next slice,
  and it is what makes both adapters wirable into `oms.Module`.
- **B3** (`MarginOp`, first producer for `boot.GateMarginOpsResolved`) and **B4**
  (the escalation that finally calls `Deleverer.Open`) follow it.
- **Task #209**, filed rather than absorbed: what §7.1's ladder does with a liq
  price the venue declines to state. The decoder reports absence faithfully; what
  the ladder should conclude from Bybit's documented "outside the price band" —
  escalate, or treat as headroom — is a policy question this slice deliberately
  did not answer, and answering it either way silently would be the defect.
- **#180 is untouched.** The connector carries no weight annotations, so the
  charged 5 is still the conservative maximum over three unread candidates. It is
  now the second line one browser read would loosen.
