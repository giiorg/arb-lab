# M3 task #173 — the cannot-count refusal, in two layers (§64)

**Date:** 2026-08-04 (session 94). **Task:** #173. **Docs:** v0.9.43.
**Core edit:** §7.6 gains the two-layer clause. **Follow-ups filed:** #185, #186.

§7.6's sentence is "a governor MUST refuse a cost it cannot count." The
mechanism under repair: `Governor.decide()` walks the windows a profile
declares (venue-scoped + the request family's) charging
`Cost.units(limit.Dimension)`, so a dimension with no applicable limit is
charged nowhere — no error, no metric. Binance's `POST /fapi/v1/order` weighs
ZERO and charges 1 on each order-count budget (Profiles §7), so a weight-only
profile admits every futures order for free, against a venue that bans on
`X-MBX-ORDER-COUNT-*`.

## §64a — the trigger had already fired

The task was filed (slice C2a's reading pass, 2026-08-03) under the premise
"NO production ratelimit.Profile is constructed anywhere", naming the first
production profile as the trigger per the deferral rule. Slice G2 then built
exactly that (`app/reconcile/config.go` `VenueConfig.profile()`, a live
governor in `wire.go`) and nobody re-read the debt. This session opened #173
believing the trigger might still be pending and found it two slices stale.

The deferral rule ("name the construct the analyzer reads") worked; what
failed is that a filed task's trigger clause is a claim about the present that
gets re-READ instead of re-CHECKED. Same shape as the memory "a debt that
names a count". The check is one grep and now precedes any "is this due yet"
judgement.

## §64b — the waiver is self-attested, so the runtime layer is primary

Round 1 of the second-model review (Codex) returned NO-SHIP on the v1 design
(construction-only coverage + written waivers): nothing at Profile
construction can see the costs the adapters will present — that knowledge
lives in the venue codecs — so an `Uncounted` declaration with a false reason
passes validation and uncounts the dimension exactly as forgetting did.

Landed as two layers:

1. **Runtime (primary).** `decide()` — Check and Acquire — refuses with an
   ERROR (`ErrCostUncounted`, the misconfiguration channel like
   `ErrFamilyUnknown`, not a budget `Decision`) any request whose cost charges
   a caller-filled dimension the family's applicable window set does not
   count. A waiver never suppresses it: the waiver claims no such cost
   arrives, and the cost in hand is the claim's falsification, delivered
   fail-closed before anything is sent. Per-family covered-dimension sets are
   precomputed at `New()` beside `applicable`, and the doc comment states why
   the two must agree.
2. **Construction (early warning).** For every declared family and every enum
   dimension (iterated numerically to `maxDimension`, so a future member joins
   the required set without anyone remembering it): a FAMILY-scoped limit of
   that dimension, or a written `Uncounted{family, dimension, reason}`.
   Refusal names venue, family and dimension.

**Rejected: a per-family endpoint-cost catalog at the composition boundary**
(the reviewer's other suggested fix). It would be a second copy of venue
knowledge — the codecs already ARE the catalog — and a copy drifts; the
runtime backstop checks the true universe (costs actually presented) at the
moment it matters, with the same fail-closed effect. Rejected: suppressing the
runtime refusal under a waiver — that converts a falsified claim back into
the silent bypass.

## §64c — the live defect: two slices' truths contradicting at an uncomposed seam

Verified against code, then fixed: C2b's Bybit read surfaces
(`positions_bybit.go`, `orderquery_bybit.go` — status, history, executions)
deliberately passed `Cost{}` with the comment "nothing to charge and nothing
to invent" (true: Bybit meters those per request per UID). C2a's
`Cost.Validate` refuses `Weight==0 && Orders==0` as the field nobody filled in
(also true). `SignedClient.Do` validates the request BEFORE `Acquire`, so
every Bybit position/order read through the real client returned
`ErrCostUnset`. Invisible because the builder suite asserts paths/params, the
governor suite asserts admission, and nothing composed them — the
evidence-does-not-mirror shape at a seam.

Fix is a carrier: `Cost` gains an unexported `requestOnly` mark settable only
through `ratelimit.RequestOnly()`. The zero value stays refused; the
contradiction (request-only beside a charge) is refused; `units()` is
unchanged (requests are charged implicitly either way). Three call sites
converted. New tests: `TestBybitReadCostsAreRequestOnlyAndValid` (the builder
side — the cost must survive the validation `Do` actually runs, pinned per
call site), and `TestSignedClientAdmitsARequestOnlyCost` (the composed seam —
through `Do` against a real governor, envelope-only charge asserted).

**Rejected: an exported `RequestOnly bool` field** — a literal can produce it
by accident and the zero-value rule ("the zero value is unusable") loses its
teeth. **Rejected: relaxing `Cost.Validate` to accept the zero** — that
re-opens the field-nobody-filled-in hole §51d closed.

## §64d — venue scope is reserved for the envelope

Round 1's third finding, verified: both `LimitReader`s address a header fold
at `(family, name)` — `BinanceLimitReader` derives the name from the header
key and always attaches the call's family; `BybitLimitReader` uses the call's
`Limit` name and the call's family. A venue-scoped limit is keyed under the
EMPTY family, so its observations report `ErrLimitUnknown`, which the observe
seam deliberately drops (by design: a mislabelled header must not fail a
request the venue already answered). So a venue-scoped weight/order budget
validates, counts locally, and silently never receives the venue's own
reading — losing exactly the correction §7.6's header-awareness exists for.

`Limit.Validate` now refuses `ScopeVenue` with any dimension but
`DimensionRequests` — the one dimension with no header to lose.

## §64e — the envelope is shared, mandatory, and not waivable

Round 2's single remaining major: the v2 draft let a family-scoped requests
limit satisfy requests coverage, but §7.6's envelope is SHARED per venue — the
governor's own cross-family test states why (exhausting spot's envelope must
refuse perp traffic). Two per-family counters can each sit inside their own
capacity while their sum trips the gateway ban neither reports on.

Landed: every profile must declare at least one venue-scoped
`DimensionRequests` limit (`ErrProfileEnvelopeMissing`); family request limits
are additive detail and never satisfy it; there is no `Uncounted` waiver for
requests. The no-waiver ruling holds because the envelope's capacity is a
LOCALLY CHOSEN safety bound (§7.6: "bound combined request rate…"), not a
venue-published fact — so the package doc's "no venue numbers nobody verified"
rule is not violated by requiring it, and "the venue documents no request
limit" is a reason to choose a bound, never to have none. This also removes
the one self-defeating state the runtime layer would otherwise admit (a
waived-requests profile whose every request errors, since every Acquire
charges a request by construction).

## §64f — what is deliberately NOT absorbed

- **#185:** Bybit's per-endpoint request budgets are inexpressible. The
  adapters already carry per-endpoint limit names for the header fold
  (`position_list`, `order_realtime`, `order_history`, order-create), but
  `charge()` walks every applicable window by dimension — so DECLARING those
  as family-scoped requests limits cross-charges them (an order burst drains
  `position_list`'s local count and refuses Safety-priority position reads
  during exactly the incident that needs them), while NOT declaring them
  silently drops their folds (`Observe` refuses, `Do` discards). Needs a
  per-endpoint axis; interacts with #171.
- **#186:** no boot-time check that a profile's limit names agree with the
  adapter's fold-derived names. The reconcile fixture itself had
  fold-unaddressable names (`spot_weight` where the fold derives `weight_1m`)
  — renamed this session to the convention, with the check filed. The observe
  seam's comment has pointed at "a boot-time check" since C2a; #186 makes it
  exist.
- The stale `ErrCostUnset` doc comment (still arguing from the refuted "every
  endpoint weighs at least one" premise §51d corrected in `limit.go`) was
  fixed in passing — the mirror rule applied to a doc.

## §64g — evidence

`make ci` green on both modules. **19 targeted mutations, 19 kills** across
the three layers (profile coverage ×8, limit/cost rules ×3, governor backstop
×3, Bybit call sites ×3, reconcile passthrough ×2). Three initial seds
reported `NOBUILD!!` with "declared and not used" — the discriminator for a
sed that orphaned a reader, not a broken claim — and were re-aimed to keep the
reader live. **One documented expected-equivalent mutant** (the
`DimensionRequests` exemption in the decide-time loop, dead by construction
while §64e's envelope rule holds; kept because its content is the coupling to
that rule, stated in the doc comment at both ends — the `Reserve()`
precedent).

Collapse-checklist notes on the new fixtures: the two uncovered-dimension
cases drop DIFFERENT families' budgets (dropping only perp's proves the loop
walked past a covered spot — "names the uncovered family" vs "names the first
family" would otherwise be the same answer); a third case drops a different
DIMENSION of the same family; spot's and perp's order budgets carry distinct
numbers so cross-family attribution cannot mint identical figures; the
family-requests-don't-satisfy case keeps everything else valid so the envelope
check is the only rule that can object (the two-claims-one-checker guard).
