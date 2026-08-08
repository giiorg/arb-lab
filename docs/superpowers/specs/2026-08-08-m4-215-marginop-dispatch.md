# Record §84 — M4 slice B3 part 2: the MarginOp dispatch path, and a returned call must CONCLUDE

**Date:** 2026-08-08 · **Task:** #215 · **Docs:** v0.9.65 · **Campaign:** 19
mutants, 0 survivors · **Commits:** `2d4cfbe` (code), and the docs commit that
carries this record. Branch `feat/milestone-1-foundation`.

Slice B3's second half. Part 1 (#212, §81) made a margin op a durable record and
resolved it at boot; part 1b (#213, §82) gave its ambiguity an end; part 1c
(#214, §83) gave Binance's evidence its second source. This part is the half that
**performs** the op — and the interesting thing about it is that almost none of
the work was in calling the endpoint.

---

## 0. What was owed, and what shape it arrived in

§7.1 had a resolution path with nothing to resolve. Every `MarginOp` in the
system was written by a test, because the only producer of one is a dispatcher
and there was none: part 1 deliberately landed the reading half first, so that
the ambiguous outcomes dispatch produces had somewhere to go.

What the section asked for is four steps in a fixed order — **read the
before-state, fsync the record, call the venue exactly once, record the
outcome** — and the order is the whole safety content, because a crash between
any two of them must leave the next boot able to tell what happened. That part
was mechanical. The design question was the one the section does not ask:
**after the call returns, who is entitled to say the top-up applied?**

---

## 1. A returned call CONCLUDES, and the evidence path is not consulted

The tidy instinct is to route every op through `domain.MarginOp.Resolve`. §7.1's
evidence rule then has exactly one home, the dispatcher stays thin, and the
boot path and the live path answer identically by construction. It is the design
this slice started with, and it is wrong in a way that only shows up when you
trace what it answers **at the moment it would be called**.

Run immediately after a call that returned, on a venue that keeps an act
history, that path answers **`Pending`**. Not because anything is wrong — because
#214's own re-derived premise (§83 §3b) is that **the act history lags
independently of the margin figure**, so the propagation bound guards the
history's *silence*. A history that has not yet published our row is exactly the
observation the bound exists to hold.

`Pending` is not a verdict. Nothing is journaled. The position stays latched (§4
below). And §7.1's ladder, seeing a top-up that did not conclude within its
budget, escalates to a `PackageDelever` — **reducing a position whose top-up
worked**. The remedy's success and its failure would have produced the same
observable, which is the failure mode this whole protocol exists to prevent.

So the dispatcher journals the verdict **the call itself carries**. Two grounds,
and the second one above is the sharp one; the first is that the evidence path is
also the *weaker* source here. §4 scopes `ResolveMarginOp(MarginOpRef)` to "the
journaled local op" — it is a **reconstruction** of what the response would have
said, built for the case where no response was read. Preferring it over a
response in hand is the same information at lower fidelity, plus the fill
confound §7.1 names.

**The rule is not duplicated by this, and that matters for the single-source
argument that motivated the rejected design.** The evidence path still owns every
case where no answer was read — which is every case the boot resolver sees. What
the dispatcher owns is the one case the evidence path was never built for: a
response.

---

## 2. The two acknowledgements check different halves, and neither venue lets both be checked

The read side has `marginLiqPrice`: one helper, both venues, because both publish
the same figure. The write side looks like it should have the mirror of that, and
it cannot.

| venue | response | what it lets an adapter verify | what it does not |
|---|---|---|---|
| Binance | `{amount, code, msg, type}` | the **ACT** — direction (`type`) and quantity (`amount`), by value | **no symbol**: that the venue acted on the asked-for market |
| Bybit | a position snapshot (`category, symbol, positionIdx, …`) | the **SUBJECT** — market and position index | **no amount**: that it moved the asked-for quantity |

Complementary and disjoint. A shared echo helper could only check the
intersection, which is empty, so each codec checks what its own venue states and
**records in its doc what it structurally cannot**. Both facts were already in
Profiles §4; the slice added the reading, not the facts.

**Bybit's `positionIM` is on the response and is deliberately not read.** It is
tempting — a posted-margin figure, right there, differencing against the
journaled before-state. Three reasons against, and the third is the one that
settles it: it is a **second authority** for a figure `ports.VenueMarginStatus`
already owns; it exists on **one** venue, so anything built on it is not a rule
but a Bybit special case; and it is available only in the case that needs **no
evidence at all**, because the call returned. Reading it would buy a corroboration
of a conclusion already reached, and pay for it with a figure that can disagree
with the one the rest of the system uses.

---

## 3. What each effect licenses — and `EffectApplied` licenses nothing

`apperror.Effect` crosses the adapter boundary because the adapter is the only
layer that knows whether the request left the socket (§5.3). The dispatcher's
mapping (`classifyMarginCall`) is deliberately **not** the order path's:

| effect | order path (`§6.7 classifyEffect`) | margin op | why |
|---|---|---|---|
| `EffectNotApplied` | `SubmissionUnknown` | **concludes `NotApplied`** | the order path has TWO absence terminals that SM §1 separates by evidence class, and the effect alone cannot choose; a margin op has ONE absence answer — the margin did not move — and both causes the effect covers say it |
| `EffectApplied` | terminal-ish | **leaves it IN FLIGHT** | *the request arrived* is not *the venue acted*, and unlike an order there is no second authority to ask — no `OrderByClientID` equivalent exists for a margin op, which is this protocol's founding fact |
| `EffectUnknown` | in flight | in flight | `apperror.EffectOf` answers `EffectUnknown` for anything it does not recognise, so the unclassified error is the conservative one by default |

The asymmetry between the first two rows is the finding. It is easy to read
"a margin op can conclude from its effect" as symmetric and let `EffectApplied`
conclude `Applied` — and that is the dangerous direction, since the harm here is
concluding a top-up landed when it did not. An **unreadable acknowledgement** is
`EffectApplied` for exactly this reason — the venue codec's own
`TestAnUnreadableAcknowledgementIsEffectAPPLIED`: the bytes came back, so the
request arrived, and nothing about them says the venue acted.

The adapter returns a failed top-up's error **unwrapped**, because a wrapped one
whose `Effect` no longer reads through would silently become `EffectUnknown` —
safe, but it would delete the one row above that concludes.

---

## 4. The latch is released by DURABILITY, not by knowledge

§7.1 serializes ops per position. The obvious implementation releases the latch
when the dispatcher knows the answer. The correct one releases it when the
answer is **journaled**, and the difference is a real state: the call returned,
the verdict is known, the journal write failed.

In that state the verdict must be reconstructed from evidence at the next boot —
and **a second op on that position destroys the evidence**, because the posted
margin has then moved by the sum of two deltas and matches neither one. The
before/after difference the whole resolution path rests on stops being a
difference of anything.

This is a different rule from the order path's `Outcome.Journaled`, and the
comparison is worth keeping: there, a lost state record is re-derivable by
**query** — the venue will tell you about the order again. Here the venue has no
memory of the operation to query, so the local record is not a cache of venue
truth, it is the only copy of the premise.

**The in-flight set starts empty at boot, and the premise for that lives in
another package.** An unresolved op makes `MarginResolution.Outcome()` answer
`OutcomeIncomplete`, which puts the process in `PostureReconcileOnly`, and such a
process is not trading — so no top-up can be dispatched while one is
outstanding, and there is nothing to seed. That is a true premise about *today's*
composition roots, not a property of the dispatcher, so **the expiry trigger is
written in the field's own doc**: the day a root drives safety actions through a
reconcile-only boot, the set must be seeded from `MarginResolution.Resolved`.

---

## 5. Three narrowings, each with an argument that is not "we only need one"

**The port is one method of four.** `ReduceIsolatedMargin` and
`TransferCollateral` have no consumer — the ladder tops up on the Rebalance band
and de-risks by delevering, never by pulling margin back out, and the staged
float belongs to §7.2's unbuilt `treasury`. `ResolveMarginOp` is not a gateway
method **in this system at all**: it is `MarginOpResolver`, an application use
case, because a port method for it would put the rule that decides `Ambiguous`
behind an adapter, where it would be venue code.

**The parameter is unsigned where the record is signed.** `domain.MarginOp`
carries a signed delta because the venues' own APIs are signed and a resolver
must match against reductions the act history reports. The dispatch path may only
perform the direction that has a caller. A signed parameter here would declare an
act nothing in this system performs and nothing tests — the
`[[enum-member-with-no-producer-is-an-unverified-classification]]` shape,
arriving as a sign convention.

**The dispatcher mints the id; `Deleverer.Open` takes one.** The asymmetry is
justified, not an inconsistency: a `MarginOpID` never leaves the process, where a
generation id is a subject other components reason about. And the failure of
getting it wrong is severe — a re-used id files two dispatch records under one
subject, which is `ErrMarginOpAlreadyDispatched`, which makes the journal
unreplayable and the **process unbootable**. A derived id over
`(instrument, watermark)` was considered and refused because it collides for
exactly the two ops §7.1 serializes: unreachable live, trivial under a stub clock.

**The urgency is a pinned const where the reads take a constructor argument.**
`marginTopUpUrgency = ratelimit.PrioritySafety`, because §7.6 names "§7.1's
margin top-up" in its `Safety` row and — unlike `marginStatusUrgency`, which has
a second caller at a second rung — there is no second caller to parameterize for.
`PriorityRoutine` is refused for the write on `OrderRequest.Validate`'s
precedent: a state-changing call under stress is not "a read that keeps the risk
view live", and both neighbouring classes are a real decision rather than a
default.

---

## 6. Where things live

| where | what |
|---|---|
| `engine/venue` | `MarginWriteCodec` (encode/decode per venue), `OrderClient.AddIsolatedMargin`, `checkMarginWriteAmount`, four `ErrMarginWrite*` sentinels |
| `engine/venue` (Binance) | `POST /fapi/v1/positionMargin {type:1}`, weight **20 (unverified, ⚠ Profiles §4)**, `{amount, type}` echoed by value |
| `engine/venue` (Bybit) | `POST /v5/position/add-margin`, `positionIdx:0` sent and echoed, each echo checked separately, `positionIM` unread |
| `oms/application/ports` | `MarginGateway.AddIsolatedMargin(instrument, amount) error` — §4 narrowed, unsigned, no `Ack` |
| `oms/infrastructure/driven` | `VenueMarginGateway`, `marginTopUpUrgency` pinned, `ErrVenueMarginWriterMissing`, the unwrapped error |
| `oms/application` | `MarginOpDispatcher`, `MarginOpDispatch`, `classifyMarginCall` / `classifyMarginEffect`, the per-position latch, five new sentinels |
| `oms/domain` | `MarginOpPending` now documents **two** producers |
| `oms` | `Deps.VenueMarginWriter`, `Deps.IDs`, `Module.MarginTopUp`, the wiring branch |

---

## 7. Rejected alternatives

1. **Route every op through the evidence path** (single-sourcing §7.1's rule).
   §1: inside the propagation bound it answers `Pending` on a venue with an act
   history, so a successful top-up is latched until the ladder delevers its
   position. This was the slice's own first design.
2. **Let `EffectApplied` conclude `Applied`,** for symmetry with the row above
   it. §3: *the request arrived* is not *the venue acted*, and there is no second
   authority to ask.
3. **Release the latch when the verdict is KNOWN.** §4: a second op destroys the
   evidence the un-journaled verdict would have to be rebuilt from.
4. **Seed the in-flight set from the journal at boot.** Correct-but-dead today —
   an unresolved op forces reconcile-only, which cannot dispatch. Refused as
   untestable code with a named expiry trigger instead of built and unreachable.
5. **A shared echo helper across both codecs,** mirroring `marginLiqPrice`. §2:
   the two verifiable halves are disjoint, so the shared helper could only check
   their empty intersection.
6. **Read Bybit's `positionIM` as corroboration.** §2: a second authority for a
   figure `VenueMarginStatus` owns, on one venue, in the one case that needs no
   evidence.
7. **A caller-minted `MarginOpID`,** as `Deleverer.Open` takes. §5: the id never
   leaves the process, and a re-used one makes the process unbootable.
8. **A derived id over `(instrument, watermark)`.** §5: it collides for exactly
   the two ops §7.1 serializes.
9. **A signed `delta` on the port,** matching `domain.MarginOp`. §5: it declares
   an act nothing performs and nothing tests.
10. **Keeping §4's `(Ack, error)` return.** Neither venue supplies an operation
    identity, so the `Ack` could only ever be the empty value and a caller
    reading a field off it would read a fact no venue states. Taken as the spec
    edit instead (v0.9.65), extended to `ReduceIsolatedMargin` on the same
    argument, and **not** to `TransferCollateral` — transfers are the one op with
    a real identity, and the contrast is what makes the signature informative.
11. **An internal retry on a failed send.** §7.1 prohibits blind delta retries
    outright, and the reason is sharper than `Place`'s: an order sent twice is
    resolvable by asking about its `ClientOrderID`, and a top-up sent twice
    cannot be distinguished from one sent once, by anything, ever.
12. **`ratelimit.PriorityRoutine` for the write.** §5.

---

## 8. The campaign — 19 mutants, 0 survivors, and one limit of this record

**19 mutants, 0 survivors.** Three verdicts were re-aims rather than results:
two `NOBUILD!!` from a sed that orphaned the mutated branch's only reader (`cmp`,
then `u`) — *"declared and not used"*, which is the ordinary bad-mutant case and
not the assertion-shaped non-build that counts as a kill — and one `NO-OP!!` from
a pattern that matched a duplicate case.

**The limit, stated rather than left to be discovered:** the per-mutant table was
not carried into this record at the code commit, so what survives is the total
and the three re-aims. A later session cannot read off *which* claim each mutant
aimed at. What it can read off is the suite, whose test names are the claims:

- **`TestDispatchJournalsTheOpBEFORECallingTheVenue`** — the order that is the
  slice's whole safety content.
- **`TestAnAcknowledgedTopUpIsAppliedWITHOUTConsultingTheEvidencePath`** — §1,
  as an executable claim rather than a paragraph.
- **`TestASecondOpOnTheSamePositionIsRefusedWhileTheFirstIsUnresolved`** and
  **`TestTheLatchIsReleasedONLYWhenTheVerdictIsDURABLE`** — §4's two halves,
  separately, because the second is the one a knowledge-released latch passes.
- **`TestTheEffectDecidesWhatMayBeConcluded`** — §3's three rows as three cases,
  including *"applied means the request arrived, NOT that the venue acted"*.
- **`TestEveryPreFsyncRefusalSendsAndJournalsNOTHING`** — six refusals (spot
  instrument, no position, unreadable before-state, a read about another market,
  a zero delta, a delta in another asset) all asserted to send nothing, which is
  the property a refusal *ordering* bug breaks.
- **`TestAFailedFsyncRefusesTheSendOutright`** and
  **`TestAnUncalibratedVenueIsRefusedABOVETheFsync`** — the two rungs either side
  of the durability boundary.
- **`TestBinanceDecodeAddIsolatedMarginComparesTheAmountBYVALUE`**,
  **`TestBybitDecodeAddIsolatedMarginChecksEachEchoSEPARATELY`**,
  **`TestAnUnreadableAcknowledgementIsEffectAPPLIED`** — §2 and §3 at the codec.
- **`TestTheAssembledModuleDispatchesAMarginTopUp`** plus three wiring refusals
  (`AMarginWriterWithNoReaderIsRefused`, `AMarginReaderWithNoWriterBuilds`,
  `AMarginWriterWithNoIDSourceIsRefused`) — #212's second survivor was an
  unexercised wiring branch, and the mirror rule says the new branch gets
  fixtures the same session, in both the refusing and the building direction.

---

## 9. What is still owed after this

1. **Nothing DRIVES the dispatcher — slice B4.** All three of §7.1's rungs now
   exist: the trigger (#210), the top-up (#215), the ladder (#177). **Not one is
   driven**, and B4's loop is the only thing between three built rungs and a
   working ladder. This is milestone 4's critical path.
2. **Nothing re-resolves a LIVE in-flight op** (#212's standing gap). In practice
   a failed top-up latches its position until the next boot, which is §7.1's own
   escalation rather than a gap — but B4 is where it stops being the only answer.
3. **A live `Ambiguous` has no in-process reporter.** The dispatch path can leave
   an op unresolved; the next boot resolves it and the gate latches. §82's
   clearance act is what ends it, and it is an operator act by design.
4. **`POST /fapi/v1/positionMargin`'s weight** (#217, now covering two figures) —
   unverified behind #180's WAF wall, charged at 20. The argument was made afresh
   rather than inherited, because unlike the boot-time history read this one is on
   the **incident path**; what makes overcharging affordable is §7.1's
   one-op-in-flight-per-position ceiling.
5. **`ReduceIsolatedMargin`, `TransferCollateral`,** sizing
   (`margin-for-target-liq-distance`), and the trading-rung gate that would refuse
   a process holding no top-up path — all await B4's root.
6. **`docs/tutorial.md` was checked and is not owed an edit.** It has no §7.1
   margin-op walkthrough to extend. The top-up appears in its **Liquidation**
   risk-register bullet as a named mechanism — an active `add-margin` top-up from
   margin staged outside the position, with a reserved rate-limit budget and a
   `PackageDelever` fallback if it is slow — and every clause of that is still
   accurate after this slice. Neither #212, #213 nor #214 touched the file
   either; its last commit is from milestone 3 (`0512f70`, #186).
