# Record §83 — M4 slice B3 part 1c: Binance's act history, and the two rules part 1 had scoped to a one-source world

**Date:** 2026-08-08 · **Task:** #214 · **Docs:** v0.9.64 · **Campaign:** 22
mutants, 2 real survivors (both closed; both fixture collapses in the NEW tests)
· `make ci` green on both modules · `SPEC EDIT OWED` grep zero · **Unblocks:**
#215.

Read record §81 first — this closes the second of the three items its §8 lists
as owed, and §81 §3b is where the reading implemented here was ruled. Read
`docs/build-spec.md` §7.1's act-history bullet for the normative statement and
Profiles §4 for the verified row.

---

## 0. What was owed, and what it cost while it was owed

§7.1 gives Binance two evidence sources for a margin op: the
`positionMargin/history` match **plus** the position-risk margin delta. Part 1
built only the delta, and §81 §3b named the cost rather than hiding it:

> Binance concludes `Ambiguous` where it could conclude `Applied`. Any
> coincident position mutation in the crash window becomes an operator incident
> instead of a resolved op.

That is a **fill** — the ordinary event, not an exotic one. A partial close
inside the window moves the posted margin to a figure matching neither the
before-state nor before+delta, so the delta arm can only say "something happened
that I cannot attribute", while the act log holds one clean row saying exactly
what happened.

---

## 1. The "plus" is not a conjunction, and the division of labour is the ruling

§81 §3b ruled this and this slice implements it. The argument, restated because
it is what the code's shape follows from: the two sources have **different
confounds**. A history row records an **act** — the venue stating that an
adjustment of this amount happened on this symbol at this time. The posted delta
is a **live quantity**, and every fill moves it.

Requiring the two to agree lets the more-confounded source veto the
less-confounded one. So instead each gets the job it is good at:

| | concludes `Applied` | concludes `NotApplied` |
|---|---|---|
| the act history | **yes** — one matching, attributable row | no |
| the posted delta | no | **yes** — nothing moved, past the bound |

and anything else is `Ambiguous`. On Bybit, where no act history exists, the
delta does both — which is not a fallback but the entire contract §7.1 states
for that venue.

---

## 2. The distinction the whole design protects, and it inverts a verdict

**"This venue keeps no act history" and "the act history returned no rows" are
opposite evidence.** Take one observation — the position's posted margin moved
by exactly the op's delta — and ask each venue:

	Bybit    no act history exists anywhere      Applied
	Binance  the act history returned no rows    Ambiguous

Opposite verdicts on identical arithmetic. On Binance the venue has been asked
whether it performed the adjustment and has said it did not, while the margin
moved by that amount anyway — a second mover whose amount coincides, which is
more suspicious than a mismatch rather than less. On Bybit nobody was asked.

The obvious modelling — `[]MarginAct`, nil meaning "no rows" — makes the value a
forgetful caller produces mean the reading that **blocks openings until an
operator clears it**. So:

- `domain.MarginActHistory` has a **three-member unexported state**: unset,
  absent, published. Its zero is `unset` and `Resolve` refuses it by name.
- The two legitimate readings are built by `NoMarginActHistory()` and
  `PublishedMarginActHistory(rows)`, so the pairing that inverts a verdict is
  **unspellable**.
- `ports.VenueMarginActs` returns that **domain type whole**, where every other
  read port in the package returns an ACL mirror. The mirrors exist because §3.6
  rule 1 forbids importing `engine/`; that argument does not apply to a domain
  type, and a port-local struct with an exported row slice would hand the
  collapse back to every adapter.
- `NewMarginOpResolver` **refuses a nil** act port rather than reading it as "no
  history", and `oms.Deps` carries `VenueMarginActs` as its own field rather
  than type-asserting `VenueMargin`. Both are the same refusal one layer out: an
  assertion would let a narrower test double, or a root wired against an older
  client, make a claim about the VENUE by accident.

---

## 3. Composing broke two premises part 1 had discharged correctly

Neither was wrong when it was written. Each rested on the delta being the only
source, and re-deriving them is what the `[[composing-breaks-the-premises-of-earlier-proofs]]`
habit is for.

### 3a. A flat position no longer outranks everything

Part 1's argument, verbatim: *"closing a position returns its isolated margin
whether or not the top-up landed first"*, so the before/after difference has no
meaning. That is a claim about **the delta's evidence**, not about the act's. A
venue that recorded the adjustment recorded it whether or not the position was
closed afterwards.

So on a history-bearing venue flatness blinds the resolution **only once the
history is silent** — and there it blinds it completely, because there is no
live posted margin to difference. `[[a-narrowing-carries-an-argument]]`: the
reason was scoped to the source it was written for, and re-reading the reason is
what found it.

### 3b. The bound guards the history's SILENCE, not the margin's stillness

Part 1 consulted the propagation bound on exactly one observation, on an
explicitly stated ground: *"only 'nothing has moved' is an observation that the
passage of time can legitimately change"*. With a second source that premise is
simply **false**. "The act log has no row yet" is also an observation time can
change, and it lags **independently** of the margin figure.

Keeping the old rule would have made a still-propagating op resolve `Ambiguous`
— so the half built to REDUCE over-ambiguation would have introduced a new
source of it, in the same slice. The rule now: within the bound a silent history
concludes nothing, whatever the margin did; past it, the delta separates
`NotApplied` from `Ambiguous`. A row that **has** arrived is never held for the
bound, because an act the venue recorded is not made more or less true by
waiting — which is part 1's own argument against hoisting the bound, applied to
the source it now belongs to.

---

## 4. `deltaType` — a required component of the match that §7.1 does not name

§7.1 words the match as *"symbol + type + amount + time-window"*, and Profiles
§4 recorded the row as `{amount, asset, symbol, time, type, positionSide}` —
flagged *"corroborated, not verbatim — SPA"*.

The row has a **seventh** field. Binance's documentation site answers a fetcher
with an AWS WAF challenge, so the shape came from the spec-generated connector's
own mocked response for this exact endpoint — a raw-text source, per
`[[a-summarized-fetch-is-not-a-source]]`:

```json
{"symbol":"BTCUSDT","type":1,"deltaType":"USER_ADJUST","amount":"23.36332311",
 "asset":"USDT","time":1578047897183,"positionSide":"BOTH"}
```

**Why it is load-bearing rather than trivia.** The stated match says nothing
about WHY the venue changed the margin — and this system already knows there is
a second reason, because **§4.1's posture check reads `isAutoAddMargin` per
configured symbol precisely because the venue can add margin on its own**. A
match blind to the cause reads such a row as evidence of this system's act, and
that is a **false `Applied`** — the silent direction, where the float ledger is
wrong and nothing reports it.

**Only `USER_ADJUST` is verified.** Rather than guess the enumeration, the decode
is two-valued: the verified value, and everything else. And an unrecognised value
is **reported, not dropped**, which is `platform/config`'s precedent inverted.
Config errors on an unknown tag *"because there the safe direction is also the
silent one"*; here it is not. Dropping an unclassifiable row makes the history
silent, which falls through to the weaker source with nothing anywhere reporting
that a row was seen and discarded. So a matching row with an unattributable cause
resolves `Ambiguous` — §7.1's "coincident manual adjustment" reached from the
other side.

---

## 5. Where things live

| where | what |
|---|---|
| `oms/domain` | `MarginActHistory` (+ its two constructors and the match), `MarginAct`, `MarginActCause`, `MarginOp.Adds`, `Resolve` split into `resolveFromDelta` and `resolveWithActHistory` |
| `oms/application/ports` | `VenueMarginActs` — the one read port returning a domain type, with the reason |
| `engine/venue` | `MarginActCodec` (`PublishesMarginActs` / encode / decode), `MarginAct`, `MarginActWindow`, `OrderClient.MarginActs`; Binance's codec; Bybit's declared absence |
| `oms/infrastructure/driven` | `VenueMarginActs` — the triple-to-three-state translation, both directions merged |
| `oms` | `Deps.VenueMarginActs` (its own field), built in the same arm as the margin-status adapter |

---

## 6. Rejected alternatives

1. **A `[]MarginAct` with nil meaning "no rows".** §2: the value a forgetful
   caller produces would mean the reading that blocks openings.
2. **A port-local ACL mirror of the history type.** §2: the mirrors exist for an
   import rule that does not apply to a domain type, and an exported row slice
   restores the collapse.
3. **Deriving the venue capability by type-asserting `VenueMargin`.** §2: it
   lets a narrower double make a claim about the venue.
4. **Reading Bybit's `transaction-log` as a second source.** Profiles §4: its
   `type` enum has no value for a manual position-margin adjustment and whether
   a top-up emits a record at all is undocumented and MUST NOT be relied on. The
   failure direction is the dangerous one — a top-up that applied and emitted
   nothing would resolve `NotApplied`.
5. **Requiring the history and the delta to agree** (the conjunction reading).
   §1, and §81 §3b before it: an ordinary partial fill becomes an incident.
6. **Dropping rows with an unrecognised `deltaType`.** §4: the silent reading is
   the unsafe one here.
7. **Fetching only the op's own direction from the venue.** The direction rule
   would then live in two places, and a wrong narrowing at the adapter returns
   FEWER rows — which reads as a venue that recorded nothing rather than as a
   bug. Both directions are fetched and the domain applies the rule once.
8. **Paging a full history page.** A truncated page can only UNDERCOUNT matching
   rows, turning §7.1's `Ambiguous` duplicate case into a single-row `Applied`.
   Refused instead, at exactly the limit.

---

## 7. The campaign — 22 mutants, and both survivors were the same shape

Both were **fixture collapses in tests written this slice**, and both were a
fixture whose `live` figure made the mutant's fallthrough answer identically.

### 7a. The duplicate-row case could not see its own rule

`len(rows) > 1` → `len(rows) > 2` **SURVIVED**. With the count rule dropped, two
rows fall past the `== 1` branch into the **silent** branch — and the fixture's
`live: "125"` (a moved margin) makes that branch answer `Ambiguous`, which is
what the test expected. So `> 1` and `> 2` agreed.

Fixed by choosing the value where the two branches disagree: `live: "100"`, an
unmoved margin, under which the silent branch answers `NotApplied`. The moved
variant is kept as a second case, but the unmoved one is what carries the rule.

### 7b. The flat guard was invisible to a fixture that passed a legal zero

`if !held` → `if false` in the history arm **SURVIVED**. The test's flat case
passed `marginMoney(t, "0")` — a **valid** zero USDT figure. With the guard
removed, the code differenced it (0 − 100 = −100), got a non-zero movement, and
answered `Ambiguous`: the expected value.

What the resolver actually passes for a flat position is `ports.MarginReading{}`'s
`PostedMargin` — the **unset** `fixedpoint.Money`, which fails `Validate`. The
fixture had substituted a legal value for the degenerate one reality supplies,
which is `[[a-legal-degenerate-shape-is-the-unaudited-fixture]]` at the level of
a single field. Fixed by passing `fixedpoint.Money{}`, under which the mutant
errors instead of concluding.

### 7c. What was caught

The arm dispatch (26 failing tests), the window's open and close edges
separately, the direction match, the magnitude-vs-signed comparison, the
bound's `<`, both silent-branch verdicts, the rung order, the cause mapping,
the page-full `>=` at exactly the limit, Bybit's `false`, the fetch window's
edge order, the adapter's both-directions fetch, and — the one that matters most
— `NoMarginActHistory()` → `PublishedMarginActHistory(nil)` in the adapter,
which is the collapse in its most compact form.

Three `NO-OP!!` results were line drift and were re-aimed. Two `NOBUILD!!`
results named an orphaned local (`opens`, `adds`) after a sed deleted its only
reader — the ordinary bad-mutant case, and each claim is covered by the
neighbouring mutation that flips the predicate instead of deleting it.

---

## 8. What is still owed after this

1. **The dispatch path** (#215) — before-state read, fsync, `AddIsolatedMargin`,
   and §7.1's one-op-in-flight-per-position serialization. **Nothing blocks it
   now.**
2. **The `positionMargin/history` weight** (#217, new) — unverified behind the
   same WAF wall as #180 and charged at 20. Nearly free, because the call is
   issued once per in-flight op at boot, but it is an unverified figure in a
   dated document and it should be closed with #180's browser read.
3. **`deltaType`'s enumeration** — one value verified, the rest conservative. A
   real account (#171's credential territory) is what would loosen it. The
   failure direction is the operator's, not the safety one.
