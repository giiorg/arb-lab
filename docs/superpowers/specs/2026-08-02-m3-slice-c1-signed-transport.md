# §50 — M3 slice C1: the signed REST transport, and the evidence that is not symmetric

**Date:** 2026-08-02 · **Session:** 76 · **Docs:** v0.9.31
**Task:** #162 (re-scoped) · **Opens:** #170, #171 · **Supersedes nothing.**

---

## §50a — What this slice is, and why slice C was split

§47f's slice C is *"`engine/venue`'s account/order half"*: signed REST for both
venues, `Place`, `OrderByClientID`, `Fills`, Bybit's `order/history` fallback,
attached to §7.6's ladder. Measured against slices A (≈2,000 lines) and B
(≈2,600), that is two to three sessions of work at this repo's bar — a decision
record, a spec edit, the collapse checklist, and a full mutation round with kills.

**The split is by LAYER, not by venue, and the reason is recorded because the
other split looks equally reasonable and is refuted in advance.** §47f's own
standing hazard says: *"A fix applied in one place gets applied to its mirror.
Slice C builds two venues' clients; whatever repetition, bound or isolation trick
one earns, the other gets the same session."* Splitting Binance-then-Bybit puts
the two clients in **different sessions**, which is precisely the failure that
hazard names. Splitting by layer keeps both venues inside each sub-slice.

- **C1 (this record):** credentials, per-venue signature composition, the
  clock-injected timestamp, the §7.6 governor attachment, header folding, and the
  §5.3 effect classification.
- **C2 (task #162's remainder):** `Place`, `OrderByClientID`, `Fills`, the Bybit
  history fallback, and the `ports.OrderPlacer` adapter.

**The user accepted the split 2026-08-02**, on the argument above.

---

## §50b — THE FIRST FINDING: the scope note's premise was false

§47f's slice-C row states the venue facts are *"normative and already recorded
(profiles §1/§4/§7); nothing unverified enters"*.

**There is no §7 in `venue-profiles.md`** — it has ended at §5 throughout. And
across the entire doc set, the only authentication fact was one line in §5 naming
Bybit's four header names. **How either venue composes the string it signs was
recorded nowhere**, which is the single most load-bearing fact a signed REST
client encodes.

This is **§47e's own shape one layer out**. §47e caught a normative claim the code
silently failed (*"§12 says `engine/venue` has an account/order half"*); this is a
**scope note claiming a verification is banked when it is not**. Both are
invisible to `grep -rn "SPEC EDIT OWED"` for the same reason: nothing *departs*
from a clause, the clause is *absent*.

**The tell is transferable and cheap: a citation to a section that does not
exist.** `§1/§4/§7` was written by someone reasoning about what *ought* to be
recorded, and nobody re-derived it — the same mechanism as
*a rejected alternative is an unchecked claim*, applied to a pointer instead of an
argument. **Check that a cited section exists before relying on what it says.**

The verification pass this forced is `venue-profiles.md` §6, new this release.

---

## §50c — THE SECOND FINDING: the evidence does not mirror, even when the code does

**Binance publishes reproducible test vectors. Bybit does not.** Both were chased
to the end:

| | Binance | Bybit |
|---|---|---|
| Published worked example | **three**, two documents, two secrets | one |
| Reproducible | **yes — all three re-computed and matching** | **no — the secret is masked** |
| Fallback probe | n/a | refuted, see below |

Bybit's example gives a string to sign and a signature but masks the secret as
`XXXXXXXXXX`. The one hypothesis that would rescue it — that the secret **is** the
literal placeholder — was tested: it yields `c00720f96c5934ca…`, not the published
`410e0f387baf…`. **Refuted, not assumed.**

### The credential-free differential probe, and why its positive control decided it

The natural substitute: sign a request with an **invented** key/secret twice, once
with the correct composition and once with a deliberately wrong one, and see
whether the venue's error distinguishes them. If it does, the venue is the oracle
and no credential is needed.

**It does not, on either venue.** Binance answered `{"code":-2015,...}` to both;
Bybit answered **HTTP 401 with `content-length: 0`** to both. Each checks the
credential *before* the signature.

**The refutation is only trustworthy because Binance ran first as the positive
control.** On Bybit alone the observation is two identical 401s, which reads
equally well as *"no complaint, so the composition is right"* — and nothing in the
observation arbitrates between the two readings. Running it first on the venue
whose answer is independently known is what converted an ambiguous reading into a
refutation. **This is the "positive controls, always" rule doing load-bearing
work rather than ceremonial work**, and it is worth noticing that the control was
not a second run of the same thing: it was the same probe aimed at a target with
a known answer.

### The consequence, which is the transferable half

**`BinanceSigner` has three independent oracles; `BybitSigner` has none.** The two
will look symmetric in the code — one interface, mirrored implementations,
comparable test counts — while the *evidence* behind them is not symmetric at all.

§47f's hazard covers **fixes** propagating to a mirror. **Its inverse does not
follow from it and is not covered:** writing Bybit's signer "the same way" as
Binance's inherits **none** of Binance's assurance. Passing tests on the two halves
are not comparable evidence, and a reader counting them would conclude they are.
Recorded in `signing.go`'s doc comment at the point of use, not only here. Task
**#171** carries the real-credential validation, which is the only thing that can
close it.

---

## §50d — THE THIRD FINDING: §7.6 is wrong about Bybit's header (spec defect, FIXED)

§7.6 read: *"the reported consumption above (`X-MBX-USED-WEIGHT-*`,
`X-Bapi-Limit-*`) is folded into the local count — but only when it exceeds it."*

**`X-Bapi-Limit-Status` is not consumption.** Verbatim: *"your **remaining**
requests for current endpoint"* — the complement, with the cap in `X-Bapi-Limit`.

Applying an upward-only fold to a *remaining* count fails in **both** directions:

| Venue says | Local count | Fold rule | Result |
|---|---|---|---|
| 90 remaining (plenty of room) | 30 used | 90 > 30 → fold | **count inflated to 90** |
| 2 remaining (nearly banned) | 30 used | 2 < 30 → ignore | **the warning is discarded** |

and **the discarded direction is silent**, which is the direction that ends in the
ban the rule exists to prevent.

**Fixed in §7.6 and implemented in `BybitLimitReader`:** convert first
(`used = X-Bapi-Limit − X-Bapi-Limit-Status`), and derive **no observation at all**
when `X-Bapi-Limit` is absent — a remaining count with no cap cannot become a
consumption figure, and a guessed cap is a budget quietly rescaled.

**Why this was invisible.** Both headers are integers about the same budget, in
similarly-named fields. Nothing in the *values* distinguishes them; only the
venue's prose points them in opposite directions. This is the **format-collapse
family one layer up** — the collision is between two fields' *meanings*, with
types, shapes and plausible ranges all identical. A §7.6 that named one header and
generalised to "the rate-limit headers" would have been right; naming both
specifically is what made it wrong.

---

## §50e — THE DESIGN DECISION: the acceptance window is asymmetric

Both venues state the same server-side test, and the **shape** is the fact:

```
Binance: if (timestamp < serverTime + 1000 && serverTime - timestamp <= recvWindow)
Bybit:   server_time - recv_window <= timestamp < server_time + 1000
```

`recvWindow` widens **only the behind side**. The ahead side is a **fixed 1000 ms**
that no parameter can widen, on either venue.

**Why that is a trap rather than a detail.** The symptom of a mis-set clock is
timing rejections; the documented knob for timing is `recvWindow`; so the natural
operator response repairs one direction and **cannot touch the other**. A host
whose clock runs fast keeps failing intermittently with the fix apparently applied
and the knob apparently maxed.

Encoded as `aheadTolerance`, a named constant with the rule beside it rather than
a config field that would imply it could be raised. And it composes with
`platform/clock`: `ServerNow` carries an **uncertainty band**, so `checkServerTime`
refuses to sign when the band does not fit inside **the tighter of the two sides**
— which is `min(aheadTolerance, recvWindow)`, because `recvWindow` may be set below
1000 ms. Two components each correct alone; the failure lives only in their
composition, which is where this project's findings keep coming from.

### `recvWindow` unset is REFUSED, and the direction is the argument

Both venues default an absent `recvWindow` to 5000 ms, so a forgotten field does
not fail — it **silently adopts the venue's policy**. And the venue's policy is not
this system's: `recvWindow` bounds **how long a delayed, non-idempotent `Place`
can still execute** after we stopped waiting for it. Large converts requests we
gave up on into orders that fill; small makes the venue refuse a stale submission,
which is a clean `EffectNotApplied` rather than an `EffectUnknown` that resolves
to `Applied` later. **The safe direction is small, the venue's default is larger
than a trading client should want, and forgetting must be a refusal that names the
field.**

---

## §50f — The effect classification (§5.3's seam)

Every path out of a signed call leaves through `venueerror.go`, and each states
the evidence its `Effect` rests on.

- **`EffectNotApplied` is granted only where the evidence is structural**:
  `net.OpError.Op == "dial"` (the connection never came up, so no byte was
  written), a local governor refusal, our own `Validate`, or a venue rejection on
  the merits. Everything else — a partial write, a lost response, a TLS error, a
  deadline — is `EffectUnknown`, because Go's HTTP client does not report how far
  a request got.
- **5xx is `EffectUnknown`, and Binance says so in as many words**: on a 503, *"the
  execution status is UNKNOWN and could have been a success."*
- **Bybit states no equivalent anywhere.** The asymmetry does **not** license
  different treatment — it means the conservative default is doing *more* work on
  Bybit, since the venue that documents the hazard is the one where we could in
  principle do better.
- **`ClassifyAckDecodeFailure` is the `EffectApplied` case**, present because it is
  the one that gets forgotten: a 2xx means the venue accepted, so a failed ack
  decode leaves a live order and a broken local record — nothing to resolve, a
  record to correct.

**A Bybit auth failure carries no `retCode` and no body at all** (401,
`content-length: 0`, live probe). A classifier reaching into the body for a code
finds zero bytes on precisely the failure an unattended process is most likely to
meet — an expired or IP-blocked key. Hence `code` is a *refinement* of the
classification rather than the thing switched on, and `codeOr` falls back to the
HTTP status so the error is still distinguishable in a log.

---

## §50g — Method notes

**The collapse checklist ran before the mutations and changed four fixtures for
free**: `Params` ordering had to differ from alphabetical or "preserve order" and
"sort" collapse; the uncertainty bound had to be tested *exactly at* the boundary;
`recvWindow` had to be set below `aheadTolerance` so the `min()` could bind; and
the Bybit POST fixture needed **both** a body and a query, or "sign the body
because the method is POST" and "sign whichever is non-empty" are the same string.

**The sharpest fixture in the slice is `limit=100, remaining=40 → used=60`.** With
`limit=100, remaining=50` — the fixture a person writes without thinking — `used`,
`remaining` and `limit/2` are all 50, and an implementation folding the raw header
value passes while folding exactly the wrong quantity. §50d's defect would have
been reproduced in the code and hidden by the test.

**Three survivors, and they taught two different lessons.**

1. **`Binance: stop signing the body` SURVIVED.** Every Binance fixture carried an
   *empty body*, so `Encode() + string(body)` and `Encode()` were the same
   expression. **The body is a fixture dimension exactly like cardinality and
   format, and the checklist as I ran it did not have it.** Closed by a POST
   fixture that also checks the concatenation *order*.
2. **`hardcode recvWindow to "5000"` SURVIVED.** Every fixture used 5000 ms —
   which is *also both venues' documented default*. **The value that collapses a
   dimension is often the venue's own default, which is exactly the value a
   fixture drifts toward.**
3. **`swap key and secret lengths in the redaction` SURVIVED — and this one is a
   NEW shape.** The **fixture was already correct**: key and secret were given
   different lengths precisely so a swap would show. What collapsed was the
   **assertion** — `strings.Contains(rendered, len(secret))` is still true when the
   two lengths trade places, because containment cannot see which slot a value
   landed in.

   **So the checklist needs a sibling.** Its stated tell is *"the fixture does not
   contain the thing under test"*; beside it belongs *"the assertion does not
   distinguish the thing under test"*. Varying the fixture buys nothing if the
   predicate throws the variation away. Fixed by asserting the exact expected
   string.

**One `NOBUILD!!`, and it was a bad sed** — `undefined: derr`, the sed deleting a
declaration while its use remained. That is the mirror of the recorded case
(*"declared and not used"*, a sed orphaning a branch's only reader): same family,
opposite direction. Re-aimed at the branch body; caught.

**A data race in a TEST, caught by `make ci` and not by `go test`.** Two parallel
subtests shared one `SignedRequest` and both assigned `RecvWindow`. The plain run
passed; `-race` aborted the binary and **failed every other test in the package**,
which is why the first `make ci` looked like a catastrophe rather than one bug.
Worth stating for the next reader: a race abort's blast radius is the whole test
binary, so "everything broke" is a plausible symptom of one shared fixture.

**Final: 18 mutations, 18 kills.**

---

## §50h — The reading pass (what the marker grep cannot see)

Zero markers, and the grep could not have found either finding above. The reading
pass found one further gap:

**§7.6 requires an operator alert and nothing delivers it.** *"surface 418 / IP ban
as immediate operator alerts"* is normative. `venueerror.go` maps 418 and Bybit's
`10018`/403 onto `ratelimit.PenaltyBan`; `platform/ratelimit`'s own doc states it
does **not** deliver alerts — *"Snapshot.Bans is the fact the caller alerts on"* —
and **nothing reads `Snapshot.Bans`**. The MUST is unmet end-to-end, carries no
marker (nothing departs from the clause; the consumer is simply absent), and is
therefore exactly the class §47e named. **Task #170.**

Two clauses were checked and hold: §4's duration rule (`RecvWindow` is a
`time.Duration` crossing the seam through `.Milliseconds()`, a conversion that
states its unit, with an out-of-range value refused rather than converted), and
§3.6 rule 2 (`var _ Signer = …` for both signers, `var _ LimitReader = …` for both
readers).

**One constraint handed forward to C2:** §4 requires every per-instrument method
to key on `VenueInstrument`. `Call` keys on `ProductCategory` only, which is right
for a transport-layer call and **not** sufficient for `Place`/`OrderByClientID`/
`Fills` — spot and perp have independent order-ID namespaces.

---

## §50i — Rejected alternatives

**(i) Split slice C by venue.** Rejected: §47f's own mirror hazard names it. See
§50a.

**(ii) Put the signers in a sibling package "for tidiness".** Rejected, and the
boundary chose the layout rather than the other way round: the secret is readable
only inside `transport` because `Credential` exposes no accessor, so a sibling
package would have required **exporting the secret** to sign with it. §7.3's
blast-radius argument becomes a compile-time property instead of a review item.

**(iii) Use `url.Values` for parameters.** Rejected: `Encode()` sorts by key, which
is not the order Binance's published vectors use — so a map-based signer **cannot
be checked against the only oracle this slice has**, to buy a property no venue
asks for. It would also let a layer re-order between signing and sending, and that
failure surfaces as an authentication error, which reads as a credential problem
rather than an encoding one.

**(iv) Fall back to local time when a venue is uncalibrated.** Rejected: it
produces a client that works while the host clock happens to be close and fails
when it drifts — a fault that appears to come and go with no local change. An
unset `ServerTime` refuses and the error names the clock.

**(v) Retry a 503 inside the client.** Rejected: §5.3's *"transport retries
forbidden"* binds the adapter as well as the caller, and a retry here is invisible
to the OMS. 503 is the status most likely to tempt one and is precisely the status
Binance documents as **UNKNOWN execution** — so the retry would be a second
non-idempotent submission of an order that may already exist. `TestSignedClient
SendsExactlyOnce` answers a 503 and asserts a request count of one.

**(vi) Treat Bybit's silence on execution ambiguity as license to classify its
5xx more cheaply.** Rejected: absence of documentation is absence of evidence,
which is the condition `EffectUnknown` exists for.
