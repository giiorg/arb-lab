# Record §97 — two §4.1 defects a live probe found and no test could, and a hazard that was already written down under the wrong heading

**Date:** 2026-08-16 (session 144) · **Task:** #236 demo preparation · **Milestone:** 4 (pre-gate)
**Predecessors:** record §95 (#236's scoping and its not-claimed list), record §96 (#242, order
finality), record §73b/§73c-1/§73o/§73p (§4.1's posture surface, the attestation ruling, the
probe-count and subject-axis findings)
**Venues:** Binance demo (`demo-fapi.binance.com`), Bybit demo (`api-demo.bybit.com`), both venues'
mainnet public hosts. **Read-only probes and one operator attestation. Nothing was placed.**

---

## 1. Why this record exists

#236 landed on 2026-08-15 with `make ci` green and **no order ever sent to a venue**. This session
began the demo run its runbook describes. Before step 1 finished, reading the two accounts' live
posture found a defect that eleven months of tests could not: **Bybit's spot-margin probe calls a
path that does not exist.**

The finding is not interesting because the fix is hard — it is one string. It is interesting because
of **what class of check could have caught it, and the answer is: none of the ones this repo has.**

## 2. The first defect: a doc slug transcribed as an HTTP path

`internal/engine/venue/posture_bybit.go` probed `GET /v5/spot-margin-uta/status`. That path returns
**HTTP 404 with an EMPTY body**, on the demo host *and* on the mainnet host.

The empty body is the diagnostic. Bybit wraps every route it owns — including failing ones — in a
`{"retCode":…,"retMsg":…}` envelope. A 404 with **no envelope** means the request never reached the
API layer, so the route does not exist rather than being refused. A JSON 404 and an empty 404 are
opposite evidence, and only the second one licenses "wrong path".

**Root cause: a documentation page SLUG was transcribed as an endpoint.** Bybit files this call at
`bybit-exchange.github.io/docs/v5/spot-margin-uta/status`, and the page body reads:

> **Get Status And Leverage** — *HTTP Request* — `GET /v5/spot-margin-trade/state`

The URL slug and the HTTP path are different strings, and the slug is the one that got written down.

### 2.1 Where it had spread

| Site | Kind |
|---|---|
| `internal/engine/venue/posture_bybit.go:117` | the probe itself |
| `internal/engine/venue/transport/signed.go:810` | the rate-limit budget roster |
| `internal/engine/venue/posture.go` (`PostureProbeSpotMargin` doc) | comment |
| `internal/risk/domain/check.go` (`bybitUTARules` doc) | restatement of Profiles §2 |
| **`docs/venue-profiles.md:43`** | **normative — the verified-venue-facts table** |
| `internal/app/pair/venuedouble_test.go:432` | the `httptest` double |
| `internal/engine/venue/transport/signed_internal_test.go:1088` | the endpoint-routing table |

Six of the seven were downstream of the seventh. `venue-profiles.md`'s rule is *"Nothing goes in
unverified"*, and this row went in unverified — from a doc URL rather than from a doc body.

## 3. Why nothing caught it, which is the half worth carrying

**The `httptest` double served the same wrong path the code requested.** Its default arm is already
`v.unexpected(w, r)`, so the fixture was never lax — it was **authored from the same source as the
code**. A strict fixture cannot object to a fact both sides state identically.

This is the shape the memory index files as *a fixture that states its fact twice* and *a check fed
only by the trusted implementation*, one layer further out than either: the duplicated fact is not
internal state but **a claim about a system nobody here controls**. No unit test, no mutation, no
formal model and no lint can falsify it, because every one of them runs inside a world the same
wrong string defines. **The only instrument that can falsify a path is a request to the venue.**

That generalises past this row. Every path constant in `signed.go`'s two rosters is an unverified
external claim of exactly this kind, and the repo has no gate that reads any of them.

### 3.1 What this says about #236's not-claimed list

Record §95 §8 listed what a green `make ci` did **not** establish, and was right to. This finding is
the first hard evidence for that list rather than an assertion of it: **the doubles proved the code
consistent with itself and nothing else**, and the very first contact with a real venue produced a
defect that would have refused the Bybit boot outright. #236 was never "done pending a run" — the
run is a distinct verification instrument, and this record is its first return.

## 4. The second defect: a gate no flat account could pass

Correcting the path let the Bybit boot reach the posture evaluation, where it
refused again:

```
posture SettingPositionMode  [LABUSDT]: ReasonUnobserved (required 0)
posture SettingAutoAddMargin [LABUSDT]: ReasonUnobserved (required 0)
posture SettingLeverage      [LABUSDT]: ReasonUnobserved (required 5)
```

The position probe scoped by `settleCoin`. Measured on Bybit demo:

| Query | Result |
|---|---|
| `category=linear&settleCoin=USDT` | **0 rows** on a flat account |
| `category=linear&symbol=LABUSDT` | 1 row — `size 0`, `leverage 5`, `autoAddMargin 0`, `positionIdx 0` |

**`settleCoin` scope answers only for symbols carrying an open position.** So
§4.1's three per-symbol rows were unobservable on a flat account. All three are
`SubjectDerivativesLeg`, live whenever the deployment configures a derivative —
which this one does, so no coverage rule lapses them away.

**The two gates had no state satisfying both.** The pair run requires both venues
flat; flat means no row means unobserved means the posture gate refuses; not-flat
means the row exists and the **flat-start gate** refuses instead. The Bybit boot
could not pass in any account state, independent of the path defect. This is the
memory index's *evidence produced by the action it gates*: the evidence is a
by-product of the very action being gated.

It also contradicted §4.1's own wording, which record §73b F4 states as
*"auto-add-margin OFF for **every configured linear symbol**, not only open
positions."* `settleCoin` scope cannot express that requirement.

### 4.1 Reversing `PostureScope`'s ruling, with a control rather than an argument

`ports.PostureScope` carried a deliberate ruling against the fix:

> the symbol roster belongs to the REQUIREMENT rather than to the read — a read
> scoped by the symbols this system already names cannot surface a configured
> symbol the venue does not know

A settled question, so it was encoded as a control and put to the venue:

```
GET /v5/position/list?category=linear&symbol=NOTAREALUSDT
  -> {"retCode":10001,"retMsg":"symbol not exist"}
```

**A symbol-scoped read NAMES an unknown symbol.** The `settleCoin` read cannot
express the question at all — it returns the same empty list for "the venue does
not know this symbol" and "the venue knows it and you hold no position", so the
collapse the ruling meant to prevent was the one it shipped.

**The ruling's goal was right and its premise about venue behaviour was untested.**
That is the distinction worth preserving: this is not a reversal of the reasoning,
it is the reasoning applied to a measured fact instead of an assumed one. What did
NOT change is where the roster comes from — still derived from the requirement, so
the rules and the read range over one list.

### 4.2 What it cost

The posture read is now `3 + len(symbols)` requests rather than a flat 4, so
record §73c-2's *"fixed, quotable number"* becomes a quotable **function** of the
roster. All four probes ride per-endpoint request budgets and none is weighted,
so the binding ceiling is `position_list`'s per-second one — untouched at a
realistic roster size.

## 5. The hazard was already written down, under the wrong heading

This is the half most worth carrying. `posture_test.go`'s **Binance** encode test
has said this since record §73o:

> `/fapi/v3/positionRisk` is NOT among these paths, because it does not publish
> the field §4.1 was reading from it. **Re-adding a per-symbol probe against a
> POSITION enumeration would restore a gate no flat account can pass.**

True, general, correct — and filed under Binance. Bybit's posture read *was* a
per-symbol probe against a position enumeration, in the same file, and did
exactly what the sentence predicts. **The note was written about the ENDPOINT
that motivated it rather than about the MECHANISM**, so nothing carried it one
screen down.

The memory index calls this *a hazard named for one input*. The cheap habit it
argues for: when a hazard note is written, check whether its subject is the
input or the mechanism, and if it is the mechanism, go and look at the siblings
immediately — the same move the *fix applied in one place gets applied to its
mirror* rule already makes for tests.

## 6. The word, which went the other way

The same rule expected the literal `"0"` for `spotMarginMode`, and the code said plainly that this
was a guess:

> the word the status endpoint answers with is NOT recorded in either document. `"0"` is this rule's
> best reading, taken from the setter's argument … reading the endpoint's published response is a
> loosening trigger.

The trigger has now been pulled. Bybit's response table reads *`spotMarginMode` — Spot margin
status. `1`: on, `0`: off*, and the live demo account answers `"0"`. **The guess was right and is now
observed.**

Keep the two halves apart when reading this row: **the word was an admitted inference that held; the
path was an unmarked transcription that did not.** The inference was cheap to check precisely
because it had been written down *as* an inference, with its own loosening trigger. The path was
expensive to find because it was written down as a fact.

## 7. What was decided, and what was rejected

**D1 — correct the path at all seven sites, and correct `venue-profiles.md` as a normative edit
rather than a code-only fix.** The verified-facts table is the upstream source; leaving it wrong
would re-seed the defect on the next reader.

**D2 — keep the retired path as a NAMED MARKER in the endpoint-routing table, not as a guard.**
*Rejected: calling it a regression guard.* The bite-check refused that claim. The mutation that
would prove it a guard is a **resurrection** — the old path re-added beside the new — and no
single-line edit reaches the row, because two earlier gates fire first: re-adding it under any
declared limit name is refused by `ratelimit` as *"limit declared twice at ScopeEndpoint"*, and
re-adding it under a fresh name is refused as an undeclared limit. It is also a specific instance of
the table's existing `{"/v5/order/unknown", ""}` row. **What it adds is findability, and the comment
says so** rather than implying coverage it does not have.

**D3 — do not build a live path-verification gate this session.** *Rejected: a `make venue-verify`
target probing every rostered path.* It is the right instrument and §3 argues for it, but it needs
credentials in CI, it makes the build depend on two exchanges' uptime, and its failure mode
(exchange down ⇒ red build) is one this repo has no ruling on. **Routed to a task, not absorbed** —
see §7.

**D4 — correct the runbook's demo-vs-mainnet paragraph in the same slice.** It was measured wrong in
the same session, by the same instrument, and leaving it would have left a false reassurance beside
a corrected one.

## 8. The runbook correction

`docs/runbooks/pair-demo-run.md` §2 read *"For Binance that is the same environment. For Bybit it is
not"*, citing §95 §2's demo-vs-**testnet** symbol counts — a true measurement of a **different pair
of environments**, generalised to this one. Measured directly on `LABUSDT`:

| | Bybit | Binance |
|---|---|---|
| demo vs mainnet grid | **byte-identical** | **differs** |
| `tickSize` | `0.0001` both | `0.0001` main, `0.00001` demo |
| `stepSize` | `1` both | `1` main, `0.1` demo |
| `quantityPrecision` | — | `0` main, `1` demo |

**Backwards, and the harmful half was the reassuring one** — the sentence told an operator not to
check Binance. The direction happens to be safe today: mainnet's grid is the **coarser** one, so a
price and size valid under the reference are also valid on demo, and the error can only over-refuse.
That is a property of today's LABUSDT rows and not a guarantee, and the corrected paragraph says so.

Same shape as the memory index's *a reassuring claim generalized from one input*, and the general
lesson is one this repo already carries: **a sentence that tells the reader not to look is the one
that must be measured.**

## 9. Owed

1. **A live path-verification instrument** (D3). Every path in `signed.go`'s rosters is an unverified
   external claim; this session falsified one by hand. Needs a ruling on CI credentials and on what a
   venue outage should do to a build.
2. **The other rostered paths are still unverified.** This slice checked exactly one, because exactly
   one was in the way. **An absence of further findings here is an absence of looking, not evidence.**
3. `docs/superpowers/specs/2026-08-06-m4-198-account-posture.md` and
   `docs/review/2026-07-18-06-…` still name the retired path. **Left as-is deliberately:** they are
   historical records of what was decided and believed then, and rewriting history to match today
   would destroy the traceability the records exist for. This record is the correction's home.

## 10. What this session did NOT establish

Unchanged from §95 §8, and worth restating because this record contains the first real venue
evidence in the project and could be misread as more:

- **No order was placed.** Every probe was a signed read.
- Nothing here is evidence about demo books, demo fills, demo funding, or the strategy's economics.
- The corrected path is verified as **the documented one that answers 200 with the expected field**.
  It is **not** yet verified that the §4.1 posture phase passes end-to-end against Bybit demo — that
  needs the boot, which is the next action.
- The Portfolio-Margin attestation written this session records the **operator's own statement**
  (`observed: off`), whose `method` field states exactly what was and was not looked at. §4.1's
  limit is unchanged: its whole strength is the operator's word plus the deliberateness of the two
  acts.
