# Record §80 — M4 slice B2: §7.1's liq-distance monitor, §5.2's policy, and #209's ruling

**Date:** 2026-08-08 · **Tasks:** #210 (closed), #209 (ruled and closed) ·
**Docs:** v0.9.61 · **Campaign:** 36 mutants, 2 real survivors (both fixed), 1
`NO-OP!!` re-aimed · `make ci` green uncached on both modules · `SPEC EDIT OWED`
grep zero.

Read `docs/build-spec.md` §5.2's risk-caps bullet and §7.1's first three bullets
for the normative form of everything below.

---

## 0. What this slice is, and the survey it closes

§12's milestone-3 closure pass wrote that *"eight comments across seven packages
defer liq-distance to a computation no package owns"*. This is that computation.

Tasks #207 and #208 landed the two read ports §7.1 needs — the mark and the
position's margin state — and each landed with the same deferral: **no consumer**,
so a root wiring the adapter would drag `engine/state` into every OMS binary for
nothing. Record §79 named the trigger by name: *"B2's liq-distance monitor, which
needs both."* That deferral is discharged here.

What the slice delivers is an **assessment**, not an action. The margin top-up
(B3's `MarginOp`) and the `PackageDelever` escalation (B4, the first caller of
`Deleverer.Open`) are the rungs above it. The split is not staging convenience:
an assessment that acted would have to hold the write path, and this type would
then be a second writer alongside the `Deleverer` that already owns the ladder.

---

## 1. §5.2's four numbers answer TWO questions, not one ladder

§5.2 states `liq_distance` as *"open ≥30% / alert <25% / rebalance <20% /
emergency <15% for BTC; tighter for alts"*. Four numbers, and the natural model
is a four-member enum.

That model deletes a region. The thresholds partition the line into **five**
intervals, and `[25%, 30%)` — below the opening threshold, above the alert — has
no member in the four-rung reading. It is not an edge case: it is where a
position spends most of its drift, and its meaning is exactly §7.4's mode (a),
*stop-opening*.

The repair is not a fifth member. The members are points in a **product of two
properties** — *may new exposure open here?* × *what does the ladder do?* — so
the fix is the second axis:

| distance | `Band` | `PermitsOpening` |
|---|---|---|
| ≥ 30% | `Clear` | true |
| [25%, 30%) | `Clear` | **false** |
| [20%, 25%) | `Alert` | false |
| [15%, 20%) | `Rebalance` | false |
| < 15% | `Emergency` | false |

`[[a-vocabulary-with-one-axis-loses-a-case]]`, and the memory's own prescription
— tabulate the members against every property they decide, look for the empty
cell — is what produced the table. The cell was not empty so much as
unrepresentable.

The collapse-checklist consequence is direct and it is why this is the first
thing the tests assert: with no fixture in `[25%, 30%)`, `PermitsOpening(d)` and
`band == Clear` return the same answer on every input, and the second axis is
unobservable. That fixture exists.

## 1b. Two more properties of the policy, both refusals

**Per instrument, no default row.** *"Tighter for alts"* is a requirement, and
the one-policy-plus-overrides shape fails in the wrong direction: an alt nobody
wrote a row for inherits BTC's 30/25/20/15, which §5.2 says in as many words is
too loose for it. §3.6 rule 4 — deny until a reason is written down — applied to
a threshold table.

**Strictly descending, refused at startup by name.** `alert == rebalance` leaves
the alert band **empty** — a rung no distance can occupy — so the operator
notification that exists to precede an automated top-up can never fire, and the
first thing anyone hears about the position is the top-up itself. That is §5.2's
own unreachable-permitted-region rule (stated there for the cross-leg skew
limit), arriving through **equality** rather than through a zero. Each adjacent
pair is refused separately, and each has its own fixture: a violation that broke
the whole ordering at once would pass against a validator checking any single
pair (`[[a-one-violation-discipline-hides-the-order]]`).

There is deliberately **no upper bound**. A threshold above 1 looks like the same
unreachable-region case and is not: a short under 1× effective leverage has its
liquidation price at more than twice the mark, so distances above 100% are
ordinary, and capping would refuse a conservatively-sized deployment at startup.

---

## 2. Where the policy lives, and the fence that did not object

§7.1's opening sentence is a claim about location before it is a claim about
numbers: *"Liq-distance policy is single-sourced (§5.2)."* It has **two**
consumers in **two** contexts — this monitor, and §6 step 4's
`margin-for-target-liq-distance` sizing, which §4's context table assigns to
`risk`. So it lives in `risk/domain` and `oms/application` imports it. In
`oms/domain` it would force the safety context to import the OMS aggregate's
package the day sizing lands.

**And that edge was admitted by an exemption argued for something else.**
`app/reconcile`'s import fence forbids every other bounded context and carries
one exemption list, granted when §4.1's posture guard landed, for the packages
this root needs *because it produces `GateAccountPosture`*. `risk/domain` was on
that list. The new `oms → risk/domain` edge therefore arrived in a §7.5 boot
binary's graph with **nothing going red** — the entry was already present, so the
permission generalized over its predicate rather than over its argument
(`[[an-exemption-argued-from-one-cause]]`, which that file's own comment cites,
about a case it did not catch).

The edge is nonetheless correct: the fence's stated reason is that importing
another context is how a boot phase acquires a collaborator that **assumes the
posture this phase has not yet decided**, and a threshold table with two pure
comparisons — no port, no clock, no venue — assumes nothing. So the list is split
in two, each half carrying its own argument, and
`TestRiskExemptionIsKeyedToTheGateItProduces` now keys only on the packages whose
reason really is the gate. Left as one list, dropping `GateAccountPosture` from
this root's roster would have reported a permission outliving its argument for a
package whose argument had not expired.

---

## 3. The arithmetic, and three ways to get it wrong quietly

`Price` had `Cmp`, `Sign` and `OnTick` and no arithmetic at all, so
`(liq − mark) / mark` could not be written without leaving the type.
`Price.Sub` and `Price.Div` are `Money`'s twins, added here on the precedent
`Price.Sign()` set one slice earlier (*"which had no caller until a mark needed
screening"*).

**The denominator is the MARK.** Dividing by the liquidation price answers a
different question and gives a systematically smaller number for a short, because
the liq price is the larger of the two. §5.2's thresholds are read as price moves
— "a 20% rally away from liquidation" — so the denominator is where the price is
now. A policy calibrated on one denominator and evaluated against the other is
wrong by that ratio **in the permissive direction**.

**The sign is preserved.** A short whose mark has crossed above its liquidation
price has a negative distance. `Abs()` is the natural-looking defensive move and
is catastrophic: past liquidation, `|liq − mark|` **grows** as the rally
continues, so the single most urgent position on the book reports an improving
distance and walks back down the ladder. It lands on `Emergency` instead, which is
also why `Emergency` is documented as covering the already-breached state rather
than only the last threshold.

**`RoundFloor`, not `RoundTowardZero`.** Every consumer treats smaller as more
conservative (`Band` compares `<`, `PermitsOpening` compares `≥`), so the safe
direction is toward negative infinity, uniformly. `RoundTowardZero` is the mode
that looks right and is conservative only on the positive side — applied to a
negative distance it rounds *toward* zero, reporting an already-breached position
as marginally less breached. The two modes agree on every positive quotient, so
this is invisible to any suite built from healthy positions; the fixture that
splits them is `-2 / 3`.

## 3b. The side is a parameter, and it cannot be inferred

`LiqDistance` takes a `PositionSide`. This system's perp leg is a short — §7.1's
trigger is *"the perp short nearing liquidation on a rally"* — so the arithmetic
could have been hard-coded. What that costs is not generality: the premise "this
position is short" would then be stated nowhere, held by no type, and checked by
nothing, while a long put through the short's formula yields a negative distance
and de-risks a healthy position.

**Inferring it from the prices is the trap.** `liq > mark` looks like a
sufficient discriminator, and the case it misses is the one that matters most: a
short at or past liquidation is indistinguishable by that test from an ordinary
long. The inference would report the top of the ladder as a healthy position
facing the other way.

`PositionSideLong` exists rather than being deferred, and the reason is
structural: without it the arithmetic has no reason to name a direction at all,
and the premise the parameter exists to record goes back to being unstated.

**Where the side SHOULD come from is filed rather than absorbed (task #211).**
The venue publishes it on the very row that states the liquidation price —
Binance as `positionAmt`'s sign, which #208's codec already parses and discards
(*"the amount decides PRESENCE and is never carried out of this function"*);
Bybit as a separate `side` field the wire struct does not declare, because
Bybit's `size` is unsigned. Pairing them at the source is strictly better than
taking the side from the OMS's own book: the two must describe the same position,
and the venue is the one that will liquidate it. It is not done here for two
reasons, both of which would have been bad to absorb. #208's narrowing ruling is
one day old and its argument was about the **quantity** — §7.5's net-delta
invariant keeping a single source — which a direction is not, but re-opening a
just-landed ruling belongs in its own slice with its own argument. And Bybit's
spelling for a flat one-way position's `side` is **unverified**, so the enum
mapping would put an unverified fact in `venue-profiles.md`, which that document
forbids.

---

## 4. Every venue-side failure is a ladder state; only wiring defects are errors

The two ports can each fail to answer, and returning the error is the shape that
suggests itself. It is wrong here, because §7.1 attaches a **consequence** to not
knowing — a stale mark *"blocks openings and escalates open positions"* — and an
error a caller logs and moves past has silently lifted that block.

So `LiqUnknownReason` is a returned field, and the error channel is reserved for
conditions no venue can cause and no retry can clear: an unconfigured instrument,
an unstated side, a venue with no mark budget. Collapsing the two is what makes
an outage look like a bug report, and a bug report look like an outage.

| reason | withholds openings | band |
|---|---|---|
| `LiqInputsComplete` | iff `!PermitsOpening` | a real band |
| `LiqPositionFlat` | **no** | `Unknown` |
| `LiqMarginUnreadable` | yes | `Unknown` |
| `LiqMarkAbsent` / `LiqMarkStale` / `LiqMarkUnreadable` | yes | `Unknown` |
| `LiqLiquidationPriceUnstated` | yes | `Unknown` |

**`OpeningWithheld` is a VETO, not a permission**, and the asymmetry is what
makes the flat row expressible. A false means this monitor objects to nothing; it
does not mean an opening is authorized, which is §6 step 4's question and several
other gates'.

**A FLAT instrument withholds nothing**, and this is the one ruling in the table
that is not conservative-by-default. It is forced. A monitor that treated "no
distance" uniformly would veto the opening — and the evidence that would lift the
veto, a position's liquidation price, is produced only **by** opening. That is
`[[evidence-produced-by-the-action-it-gates]]` exactly: the permitted region is
unreachable from a flat book, the system never trades, and every individual
refusal reads as the gate working.

**The mark is screened even on a flat instrument**, and reading it first is what
makes that true. §7.1's staleness rule is about the FEED — the mark stream is
public and answerable for an instrument this system holds nothing on — so
screening it only for held positions would make the block conditional on already
holding the exposure it exists to prevent acquiring.

**An unreadable margin read outranks every mark verdict** and is never flat.
§5.3 forbids re-deriving an absence from an unanswered query, and the cost here
is specific: the trigger silently stops watching a live short.

---

## 5. Task #209 — an unstated liquidation price WITHHOLDS but does not ESCALATE

Record §79 filed this deliberately: the decoder reports absence faithfully, and
what the ladder concludes from it is a policy ruling. The ruling **splits** the
two halves that a single "be conservative" reading fuses.

**It never permits.** Bybit's documented `liqPrice: ""` means the price falls
outside the symbol's band, which really is the well-margined case — so the
absence arrives on the **healthy** path, which is what makes "far from
liquidation" tempting. No distance is computable, so openings are blocked exactly
as a stale mark blocks them.

**It never compels, and the reason is convergence rather than caution.**
Escalating looks like the safe direction until you follow it: §7.1's remedy is a
`PackageDelever` toward reduced notional, which **raises** liq headroom, which
pushes the liquidation price **further outside the band that made the venue
withhold it**. The trigger's own remedy strengthens the trigger. It has no fixed
point except flat, so a rule escalating on this member would de-risk every
well-margined Bybit position to zero, one generation at a time, with the safety
system appearing to work throughout — and each individual de-lever defensible.

This is a shape worth naming: an escalation whose action makes its own precondition
more true is not a trigger. Checking for it is one question — *"what does the
remedy do to the input?"* — and it is not the question a conservatism argument
asks.

**What would change the ruling** is the symbol's own **price band**: Bybit states
the condition as `liqPrice <= minPrice || liqPrice >= maxPrice`, and for a short
only the upper case is reachable, so the band's edge is a **lower bound** on the
distance. If the bound clears the open threshold, absence is safely healthy; if
the band is narrower, absence is uninformative and must block. This system does
not read the band. Until it does, absence is evidence for nothing — and it is
reported as its own state, distinct from a stale mark, precisely because the two
differ on the escalation axis and agree on the withholding one.

`[[an-unverified-split-wants-the-shared-model]]` in its own shape: the coarse
model that is conservative under both readings is *withhold, do not act*, and the
venue-fact verification is the loosening trigger.

---

## 6. Wiring: one adapter is built here and the other cannot be

`oms.Deps` gains four fields, and the asymmetry between two of them is a
dependency fence rather than an inconsistency.

`VenueMargin` is a **venue client**, like every other venue capability on `Deps`
— this module constructs `driven.NewVenueMarginStatus`, because §4 assigns the
ACL to the context. `MarkRisk` is the **built port**, and it has to be: the mark's
adapter is `oms/infrastructure/markstate`, which reads `engine/state`, whose graph
reaches `marketdata` and `strategy` — both of which `app/reconcile`'s fence
forbids in a §7.5 boot binary. A `markstate.Reader` field, or a `*state.Cache`
this file wrapped, would put that graph in every binary constructing a `Module`.
So the field is the interface, the adapter is constructed by the root that owns a
market-data pipeline, and the boot root supplies nothing and drags nothing.

`LiqPolicies` is the switch: empty means this deployment monitors nothing (the
reconcile-only root's shape), non-empty makes the other three required, on
`SpotBalances`' precedent.

**What is NOT delivered and is stated rather than left to be noticed:** a trading
process without this monitor has no liq-distance trigger at all. What forecloses
that is a gate on the rung where trading is permitted, and §7.5's checklist has
no member for it — adding one this root cannot produce would refuse the only
binary there is. It lands with B4, which is also the first thing that would
*drive* the monitor.

---

## 7. The campaign

**36 mutants, two real survivors, one `NO-OP!!` re-aimed.** Both survivors are
one finding.

**`Price.Div`'s zero-divisor guard is redundant with `mulDivMantissa`'s.** Both
return `ErrDivideByZero`, so `o.p == 0` → `o.p < 0` leaves the zero case behaving
identically and the `errors.Is` assertion cannot tell them apart. What the mutant
*does* change is the **negative** divisor, which it starts refusing — and a
negative price is legal (`Price` says so: calendar spreads and settled contracts
trade below zero). Nothing in `fixedpoint` had ever divided by a negative value.

**The mirror is where it got interesting.** `Money.Div` has the same guard, and
the same mutant survives there — on an operation several milestones old, whose
subject is milestone 2's carry-over-capital number. The rule *"a fix applied in
one place gets applied to its mirror"* found a hole the mirror had always had;
`[[evidence-does-not-mirror-when-code-does]]` is the reason it was worth checking
rather than assuming the older operation was covered. Both types now have a
negative-divisor test, and the zero case additionally asserts the **message**,
which is the guard's only contribution over the shared helper's.

**The collapse checklist ran before the campaign and four fixtures exist only
because of it:**

- the `[25%, 30%)` distance — without it `band == Clear` and `PermitsOpening` are
  the same answer;
- **two** venues with **different** mark budgets — without which "this venue's max
  age" and "the only max age" are the same value
  (`[[a-fixture-of-one-collapses]]`);
- `Age ≠ DataAge` in every reading, with one case stale on **each** clock alone —
  without which `Age > b || DataAge > b` is indistinguishable from either half;
- a distance sitting **exactly on** each threshold — without which `<` and `<=`
  are the same predicate.

Two more came from asking what the ordering claims were: a **flat instrument with
a stale mark** (which is the only fixture that can tell "screen the mark
unconditionally" from "screen it for held positions"), and a held position whose
margin read **and** mark are both broken (which pins the precedence rather than
leaving it incidental).

The `NO-OP!!` was a stale line number after an edit. Bookkeeping.

One test was **reordered** because of a mutant: the #209 test called
`Validate()` first, and `Validate` happens to refuse a band beside an incomplete
reason — so an escalating mutant was being caught by the consistency rule rather
than by the ruling. The ruling's own assertions now run first, and both claims
are separately seen to bite.

---

## 8. Rejected alternatives

| Rejected | Why |
|---|---|
| the policy in `oms/domain` | §7.1's single-sourcing sentence names a second consumer in `risk`; the import would later have to run the wrong way |
| a fifth `LiqBand` member for `[25%, 30%)` | the members are a product of two properties, so the missing case is a missing AXIS, not a missing point — and a fifth member would still leave `PermitsOpening` derived from the band |
| one policy + per-symbol overrides | the default is BTC's, which §5.2 calls too loose for an alt; the failure is silent and permissive |
| capping thresholds at 1 | a sub-1× short's liq distance genuinely exceeds 100%; the cap refuses a conservative deployment at startup |
| `\|liq − mark\|` | past liquidation it GROWS with the rally, so the top of the ladder reports improving headroom |
| `RoundTowardZero` | conservative only on the positive side; it understates an already-breached position, which is the case that matters |
| dividing by the liquidation price | a different question, systematically smaller for a short, permissive by that ratio |
| inferring the side from `liq > mark` | a short past liquidation and an ordinary long are the same shape under that test |
| widening #208's codec to project the side now | correct destination, wrong slice: a one-day-old ruling reopened without its own argument, plus an unverified Bybit `side` spelling. Filed as #211 |
| returning the port errors from `Assess` | §7.1 attaches a consequence to not knowing; an ignored error lifts it silently |
| `OpeningPermitted` instead of `OpeningWithheld` | as a permission the flat case must answer "true", which makes a monitor that knows nothing about an instrument the thing AUTHORIZING exposure there |
| a flat instrument withholding openings | deadlock: the evidence is a by-product of the action the gate guards |
| escalating on an unstated liq price (#209) | the remedy raises headroom, which makes the venue withhold the price again — no fixed point except flat |
| reading Bybit's `""` as "far from liquidation" (#209) | true of the venue's reason and unusable without the band that bounds it; absence would become the permissive answer on a healthy path |
| taking `T_blind` / `mark_blind_action` now | they parameterize the escalation this slice does not build, and a budget with no consumer is an unchecked claim (§5.2's own rule) |
| a `markstate.Reader` field on `oms.Deps` | puts `marketdata` and `strategy` in every OMS binary's graph, which `app/reconcile`'s fence forbids |
| leaving `risk/domain` on the single fence exemption list | the gate-keyed lapse check would then fail for a package whose reason had not expired |

---

## 9. What is not delivered

- **Nothing drives the monitor.** `Assess` has no caller outside tests. B4 is the
  loop, and it is also what should gate a trading process on the monitor's
  presence (§6 above).
- **B3** — `MarginOp`, the first producer for `boot.GateMarginOpsResolved` — and
  **B4** — the escalation that finally calls `Deleverer.Open` — are unchanged and
  next.
- **The REST mark failover** §7.1 names for a stale mark is not built. The state
  is reported; the recovery is B4's.
- **Task #211**, filed: the side from the venue's own row.
- **The mark budget is one duration per venue**, applied to both clocks, mirroring
  `marketdata`'s single `Budget`. Its `Blind` half lands with the escalation that
  consumes it.
