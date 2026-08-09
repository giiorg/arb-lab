# Record §94 — task #225: the available-balance PORT, and the venue whose own engine is the definition

**Date:** 2026-08-09 (session 138) · **Task:** #225 (port half) · **Milestone:** 4
**Predecessors:** record §93 (the measurement half, same task — §8 states what the port must be),
record §70d / task #179 (`SpotBalances`, the sibling read port), record §79 / task #208
(`MarginStatus`, which rules this quantity OUT of the position surface), record §85 §1 / task #219
(the one-directional fence over a second decode)
**Status: DESIGN, AMENDED AFTER REVIEW ROUNDS 1, 2 AND 3. No code in this record.** The
implementation plan follows it.

**⚠ THE STOPPING RULE DID NOT FIRE, AND THAT IS THE HEADLINE.** Record §87's rule says stop when
successive rounds return findings of the *same shape* — that reads as the yield curve naming a
different instrument. **All three rounds returned NO-SHIP on three DIFFERENT shapes, each in a
section the previous round had not examined.** The honest reading is that this record was
under-reviewed rather than that the review converged, and the next session must decide whether to
run a fourth round rather than treat three as done. §10 states the case both ways.

- **Round 1 — a missing term.** Bybit's derivation was short two subtrahends (`locked`, `bonus`),
  and every capture hid it because both are zero in every phase. §6(d). Confirmed not against
  Bybit's public docs but against this repo's own `venue-profiles.md:196`, which records the deciding
  question as **open and deliberately unanswered**.
- **Round 2 — a justification about the wrong thing.** §4(i) selected Binance's published figure by
  appeal to the venue's **order-admission** engine. §7.1's re-post is not an order
  (`marginwrite.go:14-17`); it is `POST /fapi/v1/positionMargin`. The sentence was true, well-cited,
  and about an engine this rung never invokes.
- **Round 3 — an inverted venue semantic, contradicting a shipped decoder.** §5 refused on an absent
  asset for *both* venues. On Bybit the unscoped call §3 mandates returns **non-zero coins only**, so
  an absent coin is a valid **zero** — `venue-profiles.md:193` on this exact endpoint, and
  `balances_bybit.go:129-131` already decoding the same response that way. The record contradicted
  itself two sections apart: §5's justification distinguished this port from "a holdings
  enumeration", while §3 had ruled the venue call unscoped, which makes it one.

**Five further defects were found by re-deriving this record's own cited premises rather than by any
round**, which is the second lesson: every one of the eight is a claim about *evidence*, and not one
would have been caught by re-reading the argument. §4(i)'s appeal to F5 (evidence §6(b) itself
declares collapsed); §4's "an array no capture contains" (false — captured **empty**); §6(b)'s stated
**cause** (false — the account holds three funded assets, and Binance's top-level figure is
**USDT-only**, not an aggregate); the unrecorded **multi-assets-mode** dimension; and §4's claim that
provenance "becomes a statement about what the number *is* rather than which formula ran", which R2's
`min` made too strong. All corrected in place, marked **[R1]**, **[R2]** or **[R3]** where they land.

**The rounds also converged on one rule without being told it**, applied three times here: when a
question is open and both readings can be satisfied at once, take the reading that is
**correct-or-understating under both**, and file the measurement. §4's `min(availableBalance,
maxWithdrawAmount)` and its five-term Bybit derivation are that rule, not three separate judgements.
Deferring each instead would have blocked the port on three credential-gated probes.

---

## 1. What is owed, and what §93 already settled

Record §93 closed #225's measurement half and fixed the port's semantics in its §8: per settlement
asset and never an aggregate; one construction with the provenance visible in the type; Binance's
derivation inputs; tests asserting on wallet and margin; and Bybit's `""` as a refusal rather than a
decoded zero. Four alternatives were rejected there and are not reopened here.

What §93 §8 did **not** fix, because a measurement cannot: which bounded context owns the port, what
the Go surface is, and — the one that turned out to matter — whether the answer on Binance is our
subtraction or the venue's own published figure.

This record settles those three and **amends §93 §8 in two places, both inside its point 2**, marked
in §4 below rather than absorbed. **[R1] It was one until review round 1**, and the second is the
more consequential: point 2 fixes the construction as `A = wallet − (posted margin + committed order
margin)` **on both venues**, and on Bybit that is short two subtrahends (§4's `[R1]` sub-section,
§6(d)). The first amendment concerns which *source* answers on Binance; the second concerns whether
the Bybit formula is *complete*. A reader tracking only the headline amendment would carry the
incomplete formula forward, which is what happened between §93 and this record's first draft.

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

**[R1] And the trigger fires as a GATE REFUSAL, not as a design smell, which is why it cannot be
missed.** `singlewriter` polices the whole of `internal/oms/application/ports` by position, not just
the write repository (`tools/lint/singlewriter/singlewriter.go:158`). So a `treasury` package cannot
import this port at all: the second consumer does not arrive as an ugly dependency somebody might
wave through, it arrives as a failing `make ci` naming rule 6. The placement ruling is therefore
*self-enforcing* — its own expiry is checked by a gate rather than by a reader remembering this
paragraph. Two resolutions will be open at that point and both are legitimate: re-home the port to a
shared location, or have `oms` drive it and hand `treasury` the answer through a port `treasury`
declares itself, which is the escape `singlewriter`'s own diagnostic already names.

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

**(i) The venue's own engine is the definition for a quantity its engine acts on — [R2] and an
earlier draft of this paragraph NAMED THE WRONG ENGINE.** It said available balance is what Binance's
**order-admission** check evaluates when the re-post arrives. §7.1's re-post is **not an order**:
`marginwrite.go:14-17` states it in as many words — *"core §7.1's margin TOP-UP, the one
state-changing call this system makes that is not an order"* — `POST /fapi/v1/positionMargin
{type:1}`. §93's own probe labels the phase *"after add-margin (7.1's actual operation)"*. So the
argument proved a true thing about an engine **this system never invokes on that rung**.

This is the sharper failure of the two the review found, because the sentence is *correct* — order
admission does read available balance — and it is attached to the right conclusion. Nothing about it
reads as wrong; it simply does not bear on `positionMargin` admission, whose input this project has
never measured. **A justification can be true, well-cited, and about something else.**

What survives is the *principle*, which is venue-agnostic: for a quantity a venue's own engine acts
on, the venue's figure is the definition and ours is an opinion. It still argues against our local
subtraction. What it can no longer do is **select** among the venue's published figures, and §93 §7(a)
records that two candidates were never separated. That selection is resolved conservatively below
rather than by this ground.
`MarginStatus.LiqPrice` already carries this exact ruling — *"It is the VENUE's number and never one
this system reconstructs … §7.1 computes the distance the liquidation engine will act on, so the
engine's own figure is the definition."* If our subtraction and Binance's published figure ever
disagree, the venue's is operative and ours is an opinion.

This does **not** reopen F3, and **[R1] the reason is narrower than an earlier draft of this
paragraph claimed.** Two corrections, because the loose version of this argument is the one a
reviewer would wave through.

*First, F3 is about the other venue.* `totalAvailableBalance` is a **Bybit** UTA field — §93 §4
rejects it on `collateralSwitch`, `usdValue` and `totalEquity`, which are Bybit's vocabulary. So the
symmetry is not "F3 rejected a published field and this accepts one"; it is that F3 established a
*general* rule — **a published aggregate answers a different question** — and this ruling accepts a
published **per-asset** figure, which is not an instance of what F3 rejected. Binance's own analogue
of F3's field is its **top-level** `availableBalance`, and that one is rejected here too: §9 point 3
and §6(b) below.

*Second, F5 cannot be cited for the per-asset reading, and citing it was this record's own collapse.*
F5 measured a **decomposition** — that `availableBalance` equals `walletBalance −
(isolatedWallet + openOrderInitialMargin)` with residual `0E-8` in every phase. It did not, and could
not, show that the `assets[]` row is the right *source*, because §6(b) below states that the
per-asset and top-level figures are byte-identical in exactly those five phases. An earlier draft
wrote "F5 measured it doing so across all five phases", which leans on evidence this record's own §6
declares collapsed — the failure mode being catalogued two sections later, committed in the section
doing the ruling. **The per-asset reading rests on (ii) and (iii) below and on §9 point 3's reading of
the response shape, not on F5.**

**(ii) The derivation, exactly as measured, spans TWO responses.**
`probe225_pass3_separate.py:332-341` takes `walletBalance` and `openOrderInitialMargin` from
`GET /fapi/v3/account` and `isolatedWallet` from a separate `GET /fapi/v3/positionRisk`. So §93 §8
point 3's identity was established across two calls at two instants — which is the objection this
codebase has already made twice, in `marginstatus.go` (*"a ratio whose numerator and denominator came
from two different producers is one nothing compares"*) and in record §85 §3(iii) (*"margin and
quantity must describe ONE instant"*). Bybit has no such problem: `walletBalance`, `totalPositionIM`,
`totalOrderIM`, **[R1] `locked` and `bonus` are five fields on one coin row of one response** — which
is the whole of the argument, and note that the one-instant property held just as well while two of
the five were being ignored. **A single-response guarantee says the terms agree in time; it says
nothing about whether you took all of them.**

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
| Answer | **[R2]** `min(availableBalance, maxWithdrawAmount)`, both off the `assets[]` row | `walletBalance − totalPositionIM − totalOrderIM − locked − bonus` |
| `Provenance` | `AvailabilityVenueStated` | `AvailabilityLocallyDerived` |

### [R2] Binance takes the MINIMUM of the two published candidates, because (i) can no longer choose between them

With (i) narrowed to "a published figure beats our subtraction" and silent on *which*, the selection
falls to §93 §7(a), which records both candidates as **unseparated**: `availableBalance` and
`maxWithdrawAmount` held one distinct value in every phase, at all three levels, because **every
phase had unrealized profit ≤ 0** and `maxWithdrawAmount` is the field that excludes unrealized
*profit*. The two are pinned equal by the sign of a quantity that run never controlled.

They come apart on a position **in profit**, and the open question is whether `positionMargin`
admission can spend the unrealized-profit portion. If it cannot, `availableBalance` **overstates** —
the feasibility solver retains too much position and submits a top-up the account cannot fund, which
is the failed-re-post path into terminal escalation.

**So the port answers `min(availableBalance, maxWithdrawAmount)`**, and this is the same ruling shape
as `locked`/`bonus` one sub-section down, reached independently on the other venue:

- Under "the top-up **can** spend unrealized profit", the min equals `maxWithdrawAmount` and
  **understates** by the unrealized profit — refuses a top-up that could have been funded.
- Under "it **cannot**", the min is exactly right.
- **Never overstates**, under either.
- In every phase §93 captured the two are equal, so the min is byte-identical to the measured value
  and this ruling **costs nothing today**. It becomes load-bearing the first time a hedge leg sits in
  profit, which on a short is precisely a falling market.

**Three unmeasured questions were resolved in one session by the same rule** — Bybit's `locked`,
Bybit's `bonus`, and Binance's profit exclusion — and that is worth stating as a rule rather than
three times as a judgement: **when a question is open and both readings are cheap to satisfy at once,
take the reading that is correct-or-understating under both, and file the measurement.** Deferring
instead would have blocked the port on three separate credential-gated probes.

Rejected here, for the record: **pick `availableBalance` and file the question.** That is what the
first draft did implicitly, and it ships the overstating direction while §7 claims conservatism —
the same defect §7 item 3 already documents on Bybit, which is how it was recognised.

### [R1] Bybit subtracts FIVE terms, and the two added ones close a question this repo had already refused to bet on

An earlier draft of this table wrote three subtrahends. Review round 1 found the omission, and
checking it against the captures and against `docs/venue-profiles.md` makes it blocking rather than
cosmetic.

`locked` and `bonus` sit on the same coin row as the other three. They are **present in every phase
of the Bybit capture and zero in every one** (`probe225-pass3-bybit.json`, all five rows) — so across
the whole of §93's evidence the three-term and the five-term subtraction are **the same function**.
That is §6(d) below.

What makes the omission unsafe rather than merely unverified is that `venue-profiles.md:196` already
records the deciding question as **open, and refuses to answer it**: *"no sentence states whether
`walletBalance` includes `locked`"*, with the equity formula's silence *implying* inclusion. Record
§70d took that seriously — it computed its total under **both** readings, pinned a row only where the
two coincide (`locked == 0`), and reported a `locked != 0` row **unpinned** rather than asserted, with
#171's testnet session as the loosening trigger. A three-term derivation silently resolves that
question, and resolves it in the direction that **overstates** available by `locked` if the profiles
doc's own implied reading is right — F3's direction, and the one that funds an order the account
cannot pay for.

**So the port subtracts all five, and the argument is the direction rather than the formula.** Under
*inclusion* the five-term result is correct. Under *exclusion* it double-subtracts and **understates**
— which refuses a re-post that could have been funded, and is the safe half of §7.1's ladder rather
than the unfunded-order half. **Correct-or-understated under both readings, never overstated**, which
is this codebase's own rule for a default with two directions applied to a premise nobody has
measured. `bonus` takes the same treatment for a weaker reason and the same direction: its
spendability as margin is documented nowhere this project has verified, and subtracting an
unspendable credit understates.

The two rejected alternatives, recorded so the question is not silently reopened. **Refuse when
`locked != 0` or `bonus != 0`** — §70d's own move with this field — is rejected here on §4's dropped-
cross-check reasoning: a resting spot order in the settlement coin would refuse **every** re-post, on
the incident path, which is the spurious-refusal-delevers-a-healthy-position hazard that killed the
live cross-check three paragraphs up; taking it here after rejecting it there would be inconsistent
in the direction that costs a position. **Keep three terms and file the inclusion question as owed**
is rejected because it ships the overstating direction knowingly while §7 claims conservatism.

The inclusion question stays owed regardless — it is §7 item 4 — because until it is measured the
five-term answer is *safe* rather than *right*, and those are different claims.

§93 §8 point 2's requirement — **the provenance difference is visible in the type** — is honoured,
**[R2/R3] with one qualification the first draft of this sentence did not survive.** It claimed the
provenance became "a statement about what the number *is* rather than about which formula happened to
run". After R2 that is too strong: Binance's answer is `min` of two published figures, and a `min` is
a **selection rule of ours**. The number returned is still one the venue published verbatim — so
`AvailabilityVenueStated` is not false — but "no decision of ours is in this number" now is. §5 must
say what the member means: *the value is a figure the venue published; which of several candidates
was selected is this system's, and `Words` carries every candidate so the selection is auditable.*
Without that sentence a consumer reads `VenueStated` as "the venue would admit exactly this", which
is the claim the deferred measurement (§7 item 5) exists to settle. Point 3 stands unaltered as the specification of the **corroboration**
owed in §7 below; its warning against the `positionInitialMargin` name-match is why that corroboration
must be built from `isolatedWallet` when it is built. Point 4 stands for Bybit, which is where its
premise — the identity has no independent witness — is true.

### The cross-check is NOT in this slice, and dropping it is a ruling

An earlier form of this design ran the derivation on Binance as a live oracle against the published
field, refusing on disagreement. It is dropped, for a reason that outranks the evidence it would add:
**the check's failure direction is worse than the failure it guards.** A single-response derivation
would have to read `account.positions[]`, **[R1] an array captured only EMPTY** — and the precise
wording is the argument. `probe225-pass1-readonly.json` *does* hold `binance.account.positions`, as
`[]`, because the account was flat at that pass; the pass-3 captures, the only ones taken with a
position open, project it away entirely (`probe225_pass3_separate.py:332-341` keeps selected
top-level keys plus the USDT `assets[]` row). An earlier draft wrote "an array no capture contains",
which is false as stated and **weaker than the truth**: a populated row's shape is unmeasured *while
the field name appears in the evidence*, which is how a reader satisfies themselves it was covered. A
wrong assumption about that shape produces a spurious disagreement, which refuses the re-post, which
escalates §7.1's ladder into a **delever of a healthy position**. Building a refusal gate on an
unmeasured shape trades a measured hazard for an unmeasured one.

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
- **The asked-for asset absent from the response — [R3] and this is VENUE-DEPENDENT, where an
  earlier draft made it one rule and got Bybit exactly backwards.** The draft said *"deliberately not
  `AbsenceIsZero`: that rule was earned for a holdings enumeration … a venue that did not mention it
  has not said the account holds nothing."* On Bybit that is false, and the record defeated itself
  two sections apart: **§3 rules that the Bybit venue call stays UNSCOPED**, which makes the response
  precisely the enumeration the draft was distinguishing itself from. `venue-profiles.md:193` covers
  this exact endpoint and quotes the venue — *"If not passed, it returns non-zero asset info"* —
  concluding **absent coin = zero balance**. `balances_bybit.go:129-131` is a shipped decoder of the
  same response saying the same thing and setting `SpotAbsenceZero`.

  So:

  - **Bybit, coin absent from a well-formed answer → a valid asset-tagged ZERO**, not a refusal.
    `fixedpoint.ZeroMoney(asset)`. An account that has spent its settlement coin is a state §7.1 must
    be able to express — §224's algorithm takes `A₀ = 0` as an ordinary input — and turning a
    successful venue answer into an error escalates the ladder over a fact the venue reported
    correctly.
  - **Bybit, empty ACCOUNT list → refusal**, which is a different absence with opposite meaning: the
    call named one `accountType` and the answer echoed none. `balances_bybit.go` already separates
    these two and this port inherits the separation rather than re-deriving it. **Two absences, one
    response, opposite conclusions.**
  - **Binance, asset absent from `assets[]` → refusal.** The asymmetry is earned, not stylistic:
    Binance's account response enumerates zero rows too — `probe225-pass1-readonly.json` carries
    **eight** `assets[]` rows of which three are funded — so a missing row is not the venue saying
    "zero", it is an answer that does not contain the asset it was asked about.

  **The evidence for a Bybit zero cannot be five verbatim fields, because none was sent.** `Words`
  promises the venue's own fields; synthesizing `"0"` five times would manufacture evidence the venue
  never produced, which is the stored-form-contains-the-answer defect. The zero-by-enumeration case
  carries the **enumeration itself** as its evidence — the coins that were returned, plus the venue's
  own sentence as the reason silence is readable — and the implementation plan owes the exact shape.
- **A negative input** — **[R1] on all five Bybit terms** (`walletBalance`, `totalPositionIM`,
  `totalOrderIM`, `locked`, `bonus`) **and [R2] on both Binance candidates** (`availableBalance`,
  `maxWithdrawAmount`, off the `assets[]` row). Neither venue publishes one, so a negative is a parse
  that read a sign off the wrong field — `marginstatus.go:375`'s `requireNonNegative`, one surface
  over.

  Two of the additions need this refusal **more** than the terms that were already there. `locked`
  and `bonus` are zero in every capture, so a decode that reads the wrong field into either produces
  the only value the evidence has ever shown, and the refusal is the sole thing that makes a sign
  error visible. And Binance's pair sits under a `min`, which **silently prefers a negative**: a sign
  error in either candidate does not get outvoted by the other, it *becomes the answer*, and it is
  the one direction — below zero — that no downstream check treats as suspicious, because refusing a
  re-post is the safe-looking outcome.
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

## 6. FOUR COLLAPSES IN THE CAPTURED EVIDENCE, and the fixtures that separate them

The captures are real and the numbers are right. What they cannot do is tell several candidate
implementations apart, and all four cases are invisible unless looked for — the thirteenth through
**sixteenth** entries on the collapse checklist, and the first set found in *evidence* rather than in
a test.

**[R1] The count moved from three to four, and how the fourth arrived is the lesson.** (a)–(c) were
found by auditing the evidence for collapses. (d) was found by a review round asking a different
question — *is the formula complete?* — and the collapse is what had **kept it invisible** through
the audit that produced (a)–(c). An audit for collapses looks at the terms that are present; a
missing term has no fixture to collapse. So the checklist habit and a completeness check are
different instruments, and the first does not subsume the second.

**(a) `posIM` and `orderIM` are never both non-zero — on EITHER venue.** No phase of either probe
held an open position and a resting order at the same time. With one term always zero, **sum and max
are the same function**, so the `− totalPositionIM − totalOrderIM` half of Bybit's derivation is
unverified as a *combining rule*. On Binance the same gap makes §93 §8's formula and a per-asset-only
alternative (`crossWalletBalance − openOrderIM`) agree in all five phases, because
`walletBalance − crossWalletBalance` happened to equal `isolatedWallet` exactly in the only two
phases with a position.

**(b) The per-asset and top-level `availableBalance` are IDENTICAL in all five Binance phases** — so
reading the top-level figure instead of the `assets[]` row is invisible in the evidence §93
produced.

**[R2] The CAUSE this record first gave for (b) was false, and the correction changes the fixture.**
The first draft wrote *"because USDT is the only funded asset on the demo account."* It is not. The
same Binance account — `4908.71304740` identifies it across both passes —
holds **three** funded assets in `probe225-pass1-readonly.json`:

    FUNDED                             BTC 0.01000000 | USDT 4908.71304740 | USDC 5000.00000000
    Σ assets[].availableBalance        9908.72304740
    top-level availableBalance         4908.71304740      == the USDT row, exactly
    top-level totalWalletBalance       4908.71304740      != the Σ either

So the account **is** funded in more than one settlement asset and the two fields are identical
anyway. Binance's top-level figures are **USDT-only**, not a converted aggregate — which matches
Binance's own documented note that `totalWalletBalance` is *"only for USDT asset"* outside
multi-assets mode. The collapse is real; its mechanism is the opposite of the one stated.

**This inverts the hazard in §9 point 3 while leaving its conclusion standing.** The defect in
reading the top-level field is not that it sums USDC into USDT. It is that it **answers USDT's
question whichever settlement asset was asked for** — a wrong-coin error, not an aggregation error.
Still overstating whenever the USDT balance exceeds the asked-for coin's, so `assets[]`-row is still
the answer; but a reader who took the record's word would have built the fixture for the wrong
venue's behaviour. `wrong-harm-on-a-correct-finding`, and the harm clause is the part nobody
challenges precisely because the conclusion is agreed.

**And it exposes a dimension the record never named: Binance's MULTI-ASSETS MODE.** Whether the
top-level figure is USDT-only or a converted aggregate is a function of that account setting. No
capture records it — `probe225-pass1-readonly.json` contains no `multiAssetsMargin` field at all —
and every phase ran under one unrecorded value of it. This is exactly the shape §93 spent a whole
session on for the **other** venue: Bybit's *posture* deciding which fields are populated, settled by
flipping it and flipping back. The mirror was never carried across. Filed as §7 item 6.

**(c) The asked-for asset is the only row.** "This asset's row" and "the first row" are one
predicate throughout.

**[R1] (d) `locked` and `bonus` are zero in every Bybit phase, so a THREE-term and a FIVE-term
subtraction are the same function.** This is the zero-operand collapse, and it differs from (a)–(c)
in what it hides: (a)–(c) hide *which rule* the code applies, (d) hides *whether a term exists at
all*. It is why the omission survived §93's measurement, §93 §8's design, this record's first draft
and its own three-collapse audit — every one of which was looking at the terms that were there. The
fields are not obscure and not absent: they are printed on the same row as the three that were used.

Fixtures, therefore:

| Collapse | Fixture that separates it |
|---|---|
| (a) sum ≡ max | both terms non-zero **and unequal**, so sum, max and either alone are four distinct answers |
| **[R2] (b) per-asset ≡ top-level** | the asked-for asset is **not USDT**, USDT is funded to a **different** availability, and the top-level figure equals **USDT's** — which is what the venue actually does. The first draft's fixture ("a top-level total equal to neither") models a converted aggregate and would not reproduce Binance's behaviour at all |
| **[R2] (b′) `availableBalance` ≡ `maxWithdrawAmount`** | unrealized profit **strictly positive**, so the two published candidates differ and `min` is distinguishable from either. §93 §7(a) never reached this state |
| (c) row ≡ first row | the asked-for asset is **not** first in the array |
| **[R1] (d) 3 terms ≡ 5 terms** | `locked` and `bonus` each non-zero, **unequal to each other and to both IM terms**, so dropping any one subtrahend gives a distinct answer |

(d)'s fixture inherits (a)'s discipline deliberately: five subtrahends that are non-zero but *equal*
would let a decode that reads `locked` into `bonus` pass, which is the wrong-field parse the negative
refusal in §5 exists to catch and cannot catch alone. **Five distinct non-zero values, and the
expected result computed by hand rather than by the implementation's own formula.**

Plus a `""` fixture **per field — now five on Bybit**, a negative-input fixture **per field**, a
Bybit fixture whose derived result goes negative, and **[R3] four absence fixtures rather than one**,
because §5's absence rule is venue-dependent and the single "absent-asset fixture" an earlier draft
listed would have tested one venue's rule against both:

| Absence fixture | Expected |
|---|---|
| Bybit, non-empty coin list **lacking** the asked-for coin | asset-tagged **zero** |
| Bybit, `coin: []` on a well-formed account row | asset-tagged **zero** |
| Bybit, **empty account list** | **refusal** — opposite conclusion, same response shape |
| Binance, `assets[]` lacking the asked-for asset | **refusal** |

The middle two are the pair that matters: they differ only in *which* list is empty, and they must
give opposite answers. A suite holding one of them proves nothing about the other.

Every claim is then bite-checked with `tools/mutate.sh` on the decode lines. **[R3] The mirror-rule
check applies to the rules that ARE shared, and no longer to absence.** An earlier draft said the
refusal rules are *"consumer-owned and stated once"*, so breaking one must fail both venues' tests.
That still holds for the empty-string, negative-input and zero-value refusals. It is now **false for
absence**, where the venues' answers are deliberately opposite — and a mutation harness run on the
old assumption would have read a break failing only Bybit as evidence the rule got copied, when it is
the rule working. **A mirror check needs its own list of what is actually mirrored**; a blanket claim
that "the rules are shared" turns a correct asymmetry into a reported defect. A break that fails only
one venue means the rule got copied instead of
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

   **[R1] and the earlier wording of this item was wrong in a way worth keeping visible.** It said
   the rule ships "on the conservative reading" full stop. That is true only *between the two
   candidates it was comparing* — sum versus max. Conservatism is a property of a choice against an
   alternative, not of a formula on its own, and stating it unqualified is what let a **missing
   subtrahend** sit underneath a sentence claiming safety. A formula short a term is not conservative
   at all; it is wrong in the overstating direction, whichever of sum and max it uses. **A
   conservatism claim must name what it is conservative *against*.**

4. **[R1] Does Bybit's `walletBalance` include `locked` and `bonus`?** `venue-profiles.md:196`
   records the question as open and refuses to answer it; the equity formula's silence implies
   inclusion but states nothing. The five-term derivation in §4 is **safe under both readings**, which
   is not the same as right: under exclusion it understates by `locked + bonus`. #171's testnet
   session is the loosening trigger already named in the profiles doc — a resting spot order in the
   settlement coin, measured against `walletBalance` — and it settles §70d's question and this one
   with one observation. Cheap, needs credentials.

5. **[R2] Which published figure does `positionMargin` admission actually read?** §93 §7(a) records
   `availableBalance` and `maxWithdrawAmount` as **unseparated** — one distinct value in every phase,
   at all three levels, because every phase had unrealized profit ≤ 0. §94's first draft promoted
   `availableBalance` to the answer without carrying that debt forward, which is how a *known* open
   question left the record. The probe is: reach a position **in profit** so the two split, then
   attempt a top-up strictly between them and record which bound the endpoint accepts. §4's `min`
   is safe until then and is exactly right afterwards if the answer is `maxWithdrawAmount`.

6. **[R2] Binance's MULTI-ASSETS MODE was never recorded and never varied.** It decides whether the
   top-level figures are USDT-only or a converted aggregate (§6(b)). Every capture ran under one
   unrecorded value. §93 settled the analogous question on Bybit — the posture — by flipping it and
   **flipping back, with the flip back as the control rather than as cleanup**; the same experiment
   is owed here. It does not block: the port reads the `assets[]` row, which is per-asset under
   either mode. It is owed because the *argument* for reading that row cites a response shape that
   one unrecorded setting governs.

None of 1, 2, 4, 5 or 6 blocks the port. All are filed against #235, which held **two** items before
this record; §94's first draft added 1 and 2 to make four, this amendment adds 4 after round 1 and 5
and 6 after round 2, making **seven**. The count is stated at each step because an enumeration's
count word is a claim — and it has now changed **three times in one record**, which is itself the
finding: a debt list written before review is a lower bound, and the number in the handoff was stale
within a day.

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

**Derive on both venues and ignore the published Binance figures.** Rejected on §4(i) **as [R2]
narrowed it**, plus (ii) and (iii). (i) no longer says *which* published figure the top-up engine
reads — that is §7 item 5 — but it still says a venue-stated number beats our subtraction, which is
the direction this codebase refuses for the liq price and the mark. (ii) and (iii) are untouched by
round 2 and independently rule out the local derivation on Binance: it spans two responses, and its
per-asset attribution fails by dropping a subtrahend. **Three grounds, one of which was narrowed and
none of which was reversed** — worth stating, because a reader who sees (i) weakened might assume the
rejection weakened with it.

**Derive on both venues and refuse on disagreement with the published field.** Rejected on §4's
sub-section: it is more evidence, and it was the recommendation until the probe was read closely. The
derivation available inside one response reads an array **[R1] captured only empty** (§4), so the
gate rests on an unmeasured shape whose failure **delevers a healthy position**. Deferred to owed
measurement 1, at which point it becomes cheap and correct.

**Smooth the provenance away behind one port type** (§93's own rejected alternative, restated because
this design makes it *more* tempting, not less): under this ruling the two venues now differ in what
the number **is** — venue-stated versus locally derived — and not merely in which formula ran. The
type keeps the distinction.

---

## 9. Spec edits owed

**[R1] Five, not the three this record's first draft listed.** All land with the code, marked
`[SPEC EDIT OWED]` until they do — except item 5, which is a probe rather than a spec and is noted
below for why the grep will not see it:

1. **Core §4, the `AccountSource` block and the line-435 paragraph.** §4 declares
   `Balances() ([]Balance, error)` — a holdings enumeration — and nothing that answers "what may this
   account spend in asset X". The new surface must be declared, and line 435's assertion that the
   quantity "is already owned elsewhere" must be re-worded to separate the quantity's owner from the
   read port's home (§2).
2. **Core §7.2**, so the placement rule is stated once and cited, rather than contradicted twice.
3. **Core §4 line 435's LOCATOR names the wrong field, and it is the aggregate.** The sentence reads
   "Binance publishes `availableBalance` **at the top level** of `GET /fapi/v3/account`, beside the
   positions array rather than inside it". True of the response, and wrong as a pointer for this
   port. The port reads the **`assets[]` row for the settlement asset**. The sentence exists to argue
   that the quantity is not a function of a `VenueInstrument`, which it establishes either way — so
   this is a locator defect inside a correct argument, the kind only found by going to the field.

   **[R2] The re-wording must state the RIGHT reason, because the first draft of this item stated the
   wrong one.** It said the top-level figure "is an account total, and on an account funded in more
   than one settlement asset it answers F3's question rather than this one." §6(b)'s correction shows
   that is not what Binance does: outside multi-assets mode the top-level figure is **USDT-only**, and
   the captured account *is* funded in three assets while the two fields stay identical. So the
   sentence must warn that the top-level figure answers **USDT's** question whichever asset was
   asked for — a wrong-coin read, overstating whenever USDT is the larger balance.

   It is also where §6(b)'s collapse could have done real harm: on this account the two fields are
   byte-identical in every captured phase, so a port built by following the sentence would have
   passed against all the evidence §93 produced, and returned USDT's availability for a USDC-settled
   perp the first time one existed.

4. **[R1] Venue Profiles §4 must state Bybit's availability derivation with FIVE terms**, and must
   cite §196's open `locked`-inclusion question at the point of the formula rather than only in the
   holdings paragraph two entries up. The omission that round 1 caught was available to anyone who
   read §196 and §4 together; nothing in §4 sends the reader to §196, and the port's author read §4.
   **A caveat recorded away from the formula it qualifies is a caveat the formula's next reader does
   not see** — same shape as the single-letter-key rule's own lesson in core §4, where a note living
   on the struct that had the defect did not reach the next struct two files away.

5. **[R1] `analyze225.py` prints a CONCLUSION stronger than its evidence, and §93 §8 inherited it.**
   Lines 100–102 state *"A = walletBalance − totalPositionIM − totalOrderIM is the correct derivation
   and the subtraction is REQUIRED, not defensive."* The experiment behind it — `add-margin` moving
   `totalPositionIM` by `+50` while `walletBalance` moves `0E-7` — establishes that `walletBalance`
   is **gross of the two IM terms**. It establishes nothing about whether those are the **only**
   subtrahends, which is a different quantifier and the one that was wrong. The string must be
   narrowed to the claim the phase actually supports, because a printed conclusion is what the next
   session reads instead of re-deriving. This is a probe under `research/`, so the slice-close grep
   does not see it; it is listed here so it is not lost.

---

## 10. [R3] THE REVIEW'S OWN RECORD — and why the stopping rule did not fire

Three rounds were planned, on record §87's precedent. Three ran. **All three returned NO-SHIP.** The
plan's stopping rule was *"if each round returns one finding of the same shape, stop"* — and it never
became applicable, because no two rounds returned the same shape:

| Round | Finding | Shape | Section |
|---|---|---|---|
| 1 | Bybit's derivation short `locked` + `bonus` | a **missing term** — completeness of inputs | §4 (Bybit) |
| 2 | source selected by appeal to **order-admission**; the consumer is `positionMargin` | an **invalid justification** — true, well-cited, about the wrong engine | §4 (Binance) |
| 3 | absent asset refused on **both** venues; on Bybit absence is a valid zero | an **inverted venue semantic**, contradicting a shipped decoder | §5 |

**The pattern is positional, and it is the uncomfortable part.** Each round found its blocker in a
section the previous round had not examined, and each found exactly one. That is not the signature of
a converging review — it is the signature of a review whose coverage, not whose depth, is the binding
constraint. §6's fixtures and §3's signature have now had **one** pass each; §2 has had two. On the
observed rate, the expected number of remaining blockers is not obviously zero.

**The case for stopping at three.** The three findings are independent, each is fixed, and the fixes
were verified against the captures and the tree rather than accepted from the reviewer. Rounds 2 and
3 explicitly confirmed the prior round's fixes and the sections they had been pointed at. The record
is materially stronger than the artifact that entered round 1, and an implementation plan written
from it now would be built on measured premises. Reviewing prose has a floor: at some point the
remaining defects are ones only *code* will surface, and this project's own history says the
expensive discoveries come from probes and mutants, not from re-reading.

**The case for a fourth round.** The rule that was supposed to say "stop" never fired. Three rounds
produced three unrelated blockers at a steady rate of one per round, and the sections that have had
the least attention are the ones that decide whether the tests can *bite* — §6's fixtures, where this
project has a documented eleven-instance history of fixtures that could not see the thing under test.
A fourth round pointed **only** at §6 and §3 would cost one call and test the coverage hypothesis
directly: if it returns nothing, the curve has flattened and the evidence for it is real; if it
returns a blocker in §6, the positional reading was right and the review should continue until a
round examines a section already examined and finds nothing.

**The recommendation is the fourth round, scoped to §6 and §3, and it is a recommendation rather than
a ruling** — the budget is the next session's to spend. What must not happen is treating "three
rounds ran" as equivalent to "the review converged". **The plan bounded the count; the evidence did
not meet the criterion. Those are different facts and only one of them is about this record.**

### What the round-by-round evidence says about the instrument

Eight defects, **every one a claim about evidence rather than a logic error**. Not one would have been
found by re-reading the argument: they needed the captures opened, the cited line numbers visited,
and the shipped decoder read. Three came from the reviewer, five from re-deriving this record's own
citations — which means the productive question was never *"is this argument sound?"* but *"does the
thing it cites say what it says it says?"* A review round is cheap; a second reader who takes the
summary's word for the evidence is worth very little, and this record's summary was written by the
same author as the record.

**The debt list changed three times in one session** — two items, then four, then seven. A debt list
written before review is a **lower bound**, and any handoff quoting its count is stale the moment a
round lands.
