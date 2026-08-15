# Record §95 — task #236: the two-leg demo run is not a configuration, and the safety kernel cannot see the pair

**Date:** 2026-08-15 (session 140) · **Task:** #236 · **Milestone:** 4 (pre-gate, §47's admission)
**Predecessors:** HANDOFF session 139 (the task's ruling and its three named unknowns), record §54
(slice E, the §6.7 classifier), record §47 (milestone 3 admitted pre-gate), §41r (the 2026-08-01 fork
ruling), record §59 (slice G3, parallel-IOC and the chaos test)
**Venues:** Binance demo (`demo-fapi.binance.com`), Bybit demo (`api-demo.bybit.com`) — read-only
probes and one `order/test` validation. **Nothing was placed.**

---

## 1. Why this record exists

Session 139 ruled task #236 to be "a two-leg demo smoke test across both testnets" and named three
unknowns to verify before sizing it. All three are now measured. Two of the three came back the
opposite way round from the handoff's expectation, and verifying them surfaced **two structural
findings that no list held**, either of which changes what #236 is.

This record fixes what was measured, what was decided, and what was rejected — so that a later
session does not re-open the settled part or re-derive the measured part.

---

## 2. The three unknowns, measured

**(1) The testnets DO list the shortlist alts, and the handoff's named trap is refuted.** The
handoff stated the trap up front — *"the testnets are unlikely to list LAB, BEAT, KAITO, MMT, M,
SOON or ESPORTS, so the smoke test will realistically run on BTCUSDT … the fixture will not contain
the thing under test."* Measured 2026-08-15 against both demo hosts:

| Symbol | Binance demo | Bybit demo | Top bid, Binance / Bybit |
|---|---|---|---|
| LABUSDT | TRADING | Trading | 0.0845000 / 0.08450 |
| BEATUSDT | TRADING | Trading | 0.4519100 / 0.45200 |
| KAITOUSDT | TRADING | Trading | 0.382900 / 0.3835 |
| MMTUSDT | **PENDING_TRADING** | Trading | **empty book** / 0.17510 |
| MUSDT | TRADING | Trading | 1.1043000 / 1.1056 |
| SOONUSDT | TRADING | Trading | 0.1970000 / 0.19640 |
| ESPORTSUSDT | TRADING | Trading | 0.0158400 / 0.015850 |
| BCHUSDT | TRADING | Trading | 204.78 / 204.80 |

Seven of eight are tradable with five levels of two-sided book on **both** venues, at prices that
track each other. Minimum notional is **5 USDT** for the alts on both venues, against BTCUSDT's
**50** on Binance — so an alt run is *cheaper* than the BTC run the trap predicted. **MMTUSDT is the
one exception** and is out of scope for a live run until Binance opens it.

The measurement is a **snapshot**, and its limits are stated rather than left to be assumed: it says
these symbols were listed and quoted at one instant on 2026-08-15. It does not say the demo books
persist at other hours, that an IOC fills, or that demo funding follows mainnet intervals. Those are
the run's own evidence to produce.

**(2) Both demo credentials authenticate and carry trade permission, today.** Binance demo answers
`GET /fapi/v2/balance` with **4 908.60 USDT** available, and `POST /fapi/v1/order/test` for
**LABUSDT SELL MARKET 100** returns HTTP 200 — the venue validated an **alt-symbol** order against
these exact keys without placing anything. Bybit demo answers `GET /v5/user/query-api` with
`readOnly=0`, `ContractTrade: [Order, Position]`, and **`expiredAt 2026-11-10T07:03:17Z`**, with
164 776 USDT equity. Record §89's #230 probes had already exercised the write path on both venues on
2026-08-09; this check establishes the keys have not since lapsed, and **pins the expiry date**,
which no record held.

*Beside it, unlooked for:* Bybit's `totalAvailableBalance` came back an **empty string** — the same
empty field #225 is about, reproducing outside #225's probe. Consistent with §94's finding; recorded
here only because a second sighting is worth having.

**(3) `chaos_test.go` drives a FAKE venue, and says so with its reason.**
`internal/app/exercise/chaos_test.go:51` carries the heading *"Why a fake venue and not a testnet
(§59e)"*: a real testnet cannot hold an IOC open while the harness SIGKILLs the client, cannot count
our `Place` calls, and *"needs credentials #171 has not validated."* So there is **no overlap** with
#236 — the chaos test proves our crash boundary against a controllable fake, and #236 would prove the
wire contract against a matching engine. The handoff's "read it before sizing the slice" is
discharged: it neither covers part of #236 nor shrinks it.

**A vocabulary correction falls out of (1) and (2).** The handoff says "testnet" throughout; every
credential in `.env` is `*_DEMO_*` and every probe targets the **demo** hosts. For Binance the
distinction is empty — demo and testnet returned **identical** symbol sets (733 each, zero status
disagreements), confirming profiles §917's note that the legacy host is the same environment. For
**Bybit they are different environments**: 821 symbols on demo against 809 on testnet, 29 demo-only
and 17 testnet-only. One word, two asymmetric realities — the mirror rule's second axis, on the
environment axis. **This record and everything after it says "demo".**

---

## 3. Finding 1 — the composition root is single-venue BY DESIGN, so #236 is not a configuration

The handoff's inventory — *"what it can use, all of it built and mutation-tested"* — is accurate per
venue and silent about the join. `internal/app/reconcile/config.go:58`:

> *"Venue is §5.2's per-venue block. **ONE, singular**, and the singular is a property of what it
> configures rather than a simplification: `oms.Deps` takes one venue client, so a second block here
> would have nowhere to go. **A deployment trading two venues runs two of these, against two
> journals.**"*

`exercise/config.go:82` carries it downstream — *"the venue is the document's one venue"* — and
`legIntent` builds every leg's instrument from `c.OMS.Venue.Venue`, with `ErrVenueMismatch` refusing
any instrument that disagrees. **So no document can express a two-venue package**, and #236 cannot be
delivered by writing a config. It is a new composition root or it is two independent runs.

---

## 4. Finding 2 — §6.7's classifier cannot see a cross-venue pair, and the failure direction CONCLUDES

`domain.HedgeGroup` is `(Account, Venue, Symbol)` — `internal/oms/domain/hedgegroup.go:62` holds the
venue as a first-class component, and `ports/hedgegroup.go:42` restates §5.1's three-part key. §6.7's
classifier computes its residual **per hedge group**. A cross-venue pair therefore places its two
legs in **two different hedge groups, and no group contains both.** Three consequences:

1. The classifier **structurally cannot** compute a cross-venue residual. Each group sees one leg.
2. A correctly-hedged pair reads, inside each venue's own group, as **a filled leg with no pending
   exposure-changing counter-order** — which is §6 step 7's ***determined*** residual, the branch
   that compensates **immediately, ≤5 s**, with an `ExposureReduction`. Driven naively, the safety
   kernel flattens both halves of a healthy position, once per venue, in opposite directions.
3. §2.3's own words are what make this a finding rather than a gap: strategy #2 *"inherits the full
   **two-venue** execution/one-leg-failure machinery (§6) — which is exactly why it is valuable to
   build the foundation for it."* That inheritance is **asserted, not built.** It is true of the
   dispatch half — journal, deterministic ids, parallel IOC, the finality contract, all genuinely
   strategy-agnostic — and false of the classification half, because one is keyed by venue and the
   other is not. The sentence generalizes from the half that generalizes.

**The failure direction is §5.1's own rare one — it CONCLUDES rather than halts.** §5.1 records the
sibling hazard on the account axis in terms: keyed on `(venue, symbol)` alone *"a naked long in one
subaccount nets against a naked short in another to a residual of zero … the group reads flat, no
`ExposureReduction` is sized, both legs stay naked, and every invariant computed over the group
agrees."* This is that argument's neighbour on the **venue** axis, and it runs the other way: the
group does not read flat, it reads **naked**, and the compensation it sizes is real.

**Nothing acts on the verdict today, and this was verified rather than assumed.** `Module.Classify`
is constructed at `internal/oms/module.go:975` and **has no production caller** — the only
`.Classify(` sites in the tree are `transport/stream.go`'s frame classifier and `strategy`'s cost
classifier, neither related. The driver is B4, unbuilt. So a demo run today cannot trigger this
behaviour, and the finding is about the root being designed rather than about running code.

---

## 5. Why this was on no list — the fork reached the roster and not the kernel's grouping

The dates explain it, and every step was correct on its own day:

| When | What |
|---|---|
| pre-2026-08-01 | §12 leads with cash-and-carry. §5.1's hedge group is designed to group *"a trade's spot + perp legs"* — **two legs, one venue**. The venue in the key is right for that shape. |
| 2026-08-01 | The milestone-2 gate **fires**. Cash-and-carry is killed. |
| 2026-08-01 | Fork ruling (c), §41r: the alt/dislocation niche — **strategy #2 arrives early**, and it is **perp-perp across two venues**. |
| 2026-08-02 | Ruling §47 admits milestone 3 pre-gate, on the ground that §3.1 runs every Intent through the same pipeline ***"regardless of strategy"***. |
| 2026-08-03 | Slice E (record §54) builds the §6.7 classifier — **on the hedge-group key designed for the dead strategy**. |

§47's *"regardless of strategy"* is true of the order path and false of the hedge group, because the
group encodes **which legs balance each other**, which is a fact about the trade's leg shape. §47 saw
the category — it hard-gated *"milestone 4's **leg-shape-dependent** half"* and named three members
(`EmergencyPlan` contents, §4.1's posture leg roster, dual-venue margin). **The hedge group is not
among them.**

The roadmap entry that should have carried it is one sentence. §12 item 7 in full:

> **Strategy #2 — execution half:** `strategy/crossvenue` live wiring over the milestone-3 OMS +
> milestone-4 controls, scoped to alts/dislocation.

*"Live wiring"* reads as connection work. It does not say the hedge group cannot hold two venues.

**This is task #238's shape, one layer deeper.** #238 found milestone 6's canary spec still fixing
*"BTC-only scope"* and a *"spot IOC leg"* — cash-and-carry's leg shape, never re-scoped after the
fork. This is the same omission in the safety kernel's grouping. Two sites now; the fork reached the
milestone roster's prose and not the objects the prose is about.

---

## 6. The rulings

**Ruling 1 — #236 is the full cross-venue composition root.** Operator ruling, 2026-08-15, over two
rejected alternatives (§7). Two `reconcile.Runner`s, two journals, two OMS modules, a package that
spans venues with paired dispatch and fan-in.

**Ruling 2 — the cross-venue residual lives in a NEW concept ABOVE the hedge group. §5.1's key is
not touched.** The pair composes two per-venue verdicts; it does not replace them. Three grounds:

- **The venue key is correct for what §5.1 designed it for.** Margin, liquidation and ADL are
  per-venue facts. A group that spans venues hides all three, which is §5.1's own stated hazard one
  axis over.
- **The classifier is not the key's only consumer.** §5.1: *"a **remedy episode** is keyed by a
  **hedge group** — it outlives every generation and every margin op it authorizes."* So §7.1's
  whole escalation machinery — the ladder (#177), the top-up (#215), the trigger (#210) — keys off
  the same object. Removing the venue moves the ground under all of it.
- **What is actually needed is smaller than a merged number.** The pair does not need one residual
  mixing two venues. It needs the classifier to know **a leg has a partner elsewhere**, so that (a) a
  healthy pair does not read as two naked legs, and (b) a failed leg makes its partner genuinely
  naked and that is seen. A merged residual would deliver (a) by destroying the per-venue view (b)
  and §7.1 both need.

**Ruling 3 — the re-scope is its own task, #241, and it is NOT #236.** #236 may proceed under ruling
2 and must **state which model it follows**; the normative edits to §5.1/§6.7 that make the pair a
first-class concept belong to #241, beside #238. Splitting them keeps a live-venue slice from
carrying a spec edit to the safety kernel.

---

## 7. Rejected alternatives, with the reason each was rejected

**On the slice's scope (ruling 1):**

- **(i) Two independent single-venue runs, no join.** Configure the existing `exercise` binary twice,
  run four times (two opens, two reduce-only closes). Zero new production code. **Rejected by the
  operator.** Its real objection is the one session 139 used to reject the single-leg probe: the
  operator is the coordinator, nothing reacts to a half-filled pair, and it is that rejected probe
  twice over. *It remains the fallback if the root proves larger than a slice.*
- **(ii) A thin two-leg driver with an abort rule.** Run venue A, gate on its verdict, run venue B,
  and on B's failure issue a reduce-only close of A. **This was the recorded recommendation, and the
  operator chose (iii) over it.** It is the smallest thing that exercises the two-leg failure case;
  it was declined as too small a step toward a root that is owed anyway. Kept here because it is the
  natural **first phase** of the full root, and the plan should be written so that it is.

**On the pair model (ruling 2):**

- **(iv) Widen `HedgeGroup` — drop the venue from the key.** Both legs then land in one group and the
  existing classifier works unchanged; the smallest code change of the three. **Rejected on blast
  radius and on direction:** it hides per-venue margin, liquidation and ADL inside a merged number,
  it moves the key §7.1's remedy episodes are keyed by, and it edits a settled normative rule whose
  own section documents the analogous netting hazard. The benefit credited to it — "the classifier
  just works" — is real and is **not** the property that matters; the classifier working on the
  wrong grouping is the failure being avoided.
- **(v) Keep it out of the kernel for this slice.** The root reports both per-venue verdicts side by
  side, composes nothing, and marks cross-venue classification owed. **Rejected as the ruling**, but
  note that it is what #236 **actually ships** under ruling 3's split — the difference is that the
  pair concept is *designed and named* rather than deferred unnamed. If #241 slips, the root must not
  silently become (v) with no marker.

---

## 8. What is owed, and what is explicitly NOT claimed

- **Task #241** — re-scope §5.1 and §6.7 for strategy #2's leg shape: the pair concept, its residual
  composition rule, and whether it is §5.1's **fifth** journal record scope. Twin of #238.
- **The design of the root itself** is not in this record. It is the next artifact, and it needs the
  `writing-plans` treatment rather than a paragraph here.
- **Alt-specific live behaviour stays owed until a shortlist symbol runs against a venue.** Finding
  (1) removes the handoff's *reason* to expect a BTCUSDT-only run; it does not by itself put an alt
  through the order path.
- **Not claimed:** that demo books persist, that an IOC fills on a demo alt, that demo funding
  matches mainnet intervals, or that `order/test`'s 200 predicts a real `Place` acceptance —
  `order/test` validates a request, and margin, position mode and leverage constraints are checked
  by the engine it deliberately does not reach.

---

## 9. Method note

Every finding above came from opening the artifact and measuring it — session 139's own closing
lesson, applied to session 139's handoff. Two of the three unknowns and both structural findings
contradicted a documented expectation, and **no document was lying**: the trap was a reasoned guess
stated honestly as a risk, §2.3's inheritance sentence is true of the half it generalizes from, and
§12 item 7's "live wiring" describes work nobody had sized.

**One collapse, caught in this session's own instrument.** The first depth probe reported an empty
book for **every** symbol *including BTCUSDT*. The uniformity was the tell: it was `urllib` failing
TLS with no CA bundle — the exact trap `probe230.py`'s header comment documents and works around, in
a script written without reading it. A probe that fails identically everywhere reads as a finding
about the venues. **A negative result needs its positive control before it is believed**, and here
the control was free: a symbol that certainly has a book.

---

## 10. What was built (2026-08-16, session 143, `internal/app/pair`)

### 1. Ruling 1's shape, delivered

`internal/app/pair` is the composition root: **two `reconcile.Runner`s, two journals, two
`oms.Module`s, held open simultaneously by one process**, driving ONE cross-venue perp-perp package.
`arblab pair run -config <file>` is the verb — not a `crossvenue` subcommand, because every existing
`crossvenue` subcommand is an offline research tool and an operator must not be one word away from
confusing a simulation with a live order path.

Both execution modes are built. **Sequential** (§7 alternative (ii), the plan's phase 1) opens leg A,
establishes its exposure, and sends leg B **sized to A's established fill**; **paired** (phase 2)
sends both before either is established and composes at the fan-in. The mode is a required field with
no default — two failure surfaces, and a defaulted mode is a decision nobody made.

### 2. Ruling 2's concept, and where it lives

`pair.Verdict` and `pair.Compose` are in the **root**, not the kernel. `domain.HedgeGroup` is
untouched, §6.7 is unedited, and `Module.Classify` still has no production caller. The concept has
five members — `Unopened`, `Intact`, `HalfOpen`, `Skewed`, `Indeterminate` — and `Exposed()` is the
predicate the exit class derives from.

The reading that feeds it is **three-valued** (`Establish`): exposure counts as measured only when the
order is terminal for *state* **and** the join reported none of its four per-order finding lists about
it. The fourth is `Contradicted`, which #242 added after this record was written; the plan's own D4
was corrected for it, and what guards a *fifth* is a reflective test over `JoinFacts`' fields rather
than a backstop in the code — the backstop was written, and mutating it to `if false` **survived**,
because it is unreachable while every field has an arm.

### 3. What #236 actually ships is §7's alternative (v), and the markers are on

The root reports both per-venue verdicts side by side and composes **above** them; cross-venue
*classification* stays owed. §7 (v) warned: *"If #241 slips, the root must not silently become (v)
with no marker."* The markers are three, all naming §6.7 and this record's §4 by section:

1. `internal/app/pair/pair.go`'s package comment — why the concept is not in the kernel, and what that
   costs.
2. `pair.Verdict`'s own doc — "It is NOT §6.7's verdict and must not be confused with one."
3. `ErrUnwindRefused`'s doc, and `drive.compensate`'s comment — the one reading this root will act on,
   and the three it hands to a human.

### 4. The one `[SPEC EDIT OWED]`, discharged here

`build-spec.md` **v0.9.70 → v0.9.71**. §5.2's OMS-deployment-block bullet now states that the block
describes one deployment on one venue and that a cross-venue package is **two** of them under one
root, with the two rules that make one shared `TradeID` safe: the sides must name **different venues**
and **different physical journals**. §12 item 7 gains a scope correction naming both halves the phrase
"live wiring" hid — the new root, and §6.7's blindness to the pair.

The marker is **removed from the code**, not carried forward: the slice-close gate is a grep, it
cannot see an unmarked claim, and a marker carried past its slice is worse than either landing it or
routing it. Neither edited section is tracked by `formal-fresh` (it watches §2, §3, §5, §5.3, §6, §7,
§7.1, §7.4, §7.5 and SM §1/§7), so no model's stamp moves and **no new `knownStale` marker is
created**. The count stands at **six, all #229's**.

### 5. Five things owed, with their reasons

- **Task #241**, unchanged: the pair as a kernel concept, its residual composition rule, and whether
  it is §5.1's fifth journal record scope.
- **Quantization (D5).** A fill that fails the partner's grid unwinds the **whole** leg rather than
  hedging the quantized part and unwinding the remainder. Correct and coarse; the better version needs
  a lot-grid rounding decision with an explicit direction, which is a money-rounding argument and
  deserves its own slice. Not urgent while runs are fixed-size.
- **A durable pair record.** The shared `TradeID` correlates the two legs in both journals, which is
  the fact ruling 2 asked for — but **nothing states that the pair *should* have had two legs**. A
  crash between leg A and leg B leaves side A's journal holding a leg whose partner was never planned
  anywhere. That is precisely §5.1's fifth-record-scope question, and it belongs to #241.
- **Half-open on side B.** Sequential mode opens side 0 first, so a side-1-only exposure is a
  contradiction rather than a case; paired mode **can** produce it, and the compensation for it is
  refused (D8) rather than handled.
- **A per-leg dispatch refusal is untested from this level, and it is named rather than deleted.**
  `drive.open` handles the branch where `Execute` returns no aggregate error, `LegResult.Err` is
  non-nil and nothing reached the venue — the branch whose absence recreates the
  `if err != nil { return }` reflex the executor's split signature exists to prevent. No fixture
  reaches it, and the reason is structural: every pre-`Place` refusal inside `Dispatch` is caused by
  **local** state (an OCC conflict, a journal fsync failure, a missing venue calibration), none of
  which a venue double can produce.

### 6. No live run was performed

**This slice ran against `httptest` venue doubles only.** No order reached `demo-fapi.binance.com` or
`api-demo.bybit.com` in session 143, so §8's not-claimed list stands **entirely unchanged** and
nothing here is evidence about demo books, demo fills, demo funding, or the strategy's economics. The
runbook `docs/runbooks/pair-demo-run.md` is what a live run follows, and its §8 restates the same
list operationally. Saying so in a sentence is the point: a record silent about whether the thing ran
is exactly the shape §9's method note is about.

### 7. Five findings the build produced that the plan did not predict

1. **Bybit's §4.1 posture read rides the SPOT family**, so a *perp-only* Bybit deployment must still
   route a spot endpoint — the mirror of Binance, where a spot-only deployment needs no futures route.
   Found by booting it.
2. **Bybit's transport declares a closed EIGHT-endpoint budget roster** and refuses client
   construction when a document leaves any row unbudgeted, so that a restart's fallback query cannot
   spend the primary query's allowance. A cross-venue document is the first one that has to carry both
   venues' metering shapes, and they are not the same shape: Bybit publishes **no per-endpoint
   weight** (profiles §5), so the honest configuration is its real requests-per-second budget plus
   §7.6's written `uncounted` waiver — never Binance's `weight_1m` under a Bybit name.
3. **A guard duplicating a downstream check reads as tested and is not.** Mutating away "leg A filled
   nothing → do not send leg B" SURVIVED: with the gate gone, leg B is planned at quantity zero and
   the **planner's** §2.4 min-quantity rule refuses it, so the Place-count assertion passes for the
   wrong reason. What separates them is the **refusal** — D6's run is clean — and that is now asserted.
4. **An unestablished leg was always also flat in every fixture the plan specified**, so the
   `Established` guard was equivalent to the flat guard and could be deleted with everything green.
   The isolating fixture holds side A's order **live at the venue with 60 already executed against
   it**: non-zero exposure that has not been measured, which is the most dangerous reading in this
   root and the one the guard exists for.
5. **A rendezvous that counts arrivals cannot tell the two modes apart.** Under sequential dispatch
   leg A enters the barrier, waits out its bound, leaves, and leg B arrives as the "second" party — so
   a running total reaches the quorum and reports a fan-out that never happened. What separates the
   modes is **simultaneity**, so the counter goes down again when a party gives up. Its own negative
   control is what found this.
