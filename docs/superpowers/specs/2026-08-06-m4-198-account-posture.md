# Design record §73 — M4 slice A (task #198): the §4.1 account posture, and the gate that has been declared since slice G2 with nothing to produce it

**Date:** 2026-08-06 · **Session:** 109 · **Docs:** v0.9.50 (spec edits owed at
slice close) · **Task:** #198

**Reads:** core §4 (the `AccountPostureSource` port declaration), core §4.1 (the
safety premise), core §6 step 3 (the pre-trade guard), core §7.5 (the boot
assert), core §12 item 4 (this milestone), `venue-profiles` §2 (the per-venue
posture facts and the assert-or-refuse read paths — the table this slice
implements), `internal/app/reconcile/app.go` (`declaredGates`, `ExitClass`),
`internal/oms/application/ports/balances.go` and
`internal/engine/venue/balances_bybit.go` (task #179's borrow handling, which
names this slice as the owner of the strict reading).

---

## §73a — What this slice is, and why its seam already exists

Milestone 4 opens on **the safety premise rather than on sizing**, and §12 item 4
states the ordering itself: *"the account/margin-mode verification above is what
makes a tier reading mean anything."* A live leverage tier read under the wrong
margin mode or the wrong position mode is a number computed against a different
contract than the one that will be traded, so sizing built first would rest on a
premise nothing had checked.

**The seam is not greenfield.** `platform/boot` has declared `GateAccountPosture`
since slice G2, `app/reconcile`'s `declaredGates()` lists it in §7.5's full
eight-gate roster, and `Report.ExitClass` names it explicitly in the arm for
gates *"with no producer anywhere"*. That arm's comment is the instruction this
slice follows:

> a gate added to §7.5's roster has to choose a side here, and choosing "not
> ours" by falling through is how a real refusal becomes invisible.

So the slice's structural deliverable is precise: **give an already-declared gate
its first producer, and move it across that switch.** The switch is
compiler-checked (§3.6 rule 10), which is what makes the move impossible to
forget — but note it is checked for *exhaustiveness*, not for *correct
membership*. Nothing but this record and a test says the gate now belongs in the
`ExitIncomplete` arm; see §73g.

**Scope boundary.** This slice delivers the posture **verification**. It does not
deliver sizing, the margin top-up protocol, `MarginGateway`, the kill-switch
ladder, `Fence`/`FenceReady`, or the audit projection — those are milestone 4's
other workstreams (§12 item 4), and the actuator half is explicitly
canary-concurrent (`[→ R1d-8]`).

**Credentials are deferred (operator ruling, session 109).** Every venue
interaction here is fixture-driven. A live testnet read is a **loosening
trigger**, not a prerequisite — the same posture the project took on #171 and
#180. What that costs is stated in §73f rather than left implicit.

---

## §73b — The reading pass, before any code

Six findings came out of reading Profiles §2 against the existing code. Four are
this slice's design; two are defects in the specification itself.

### F1 — "Portfolio-Margin OFF" is a required check with NO read path (spec defect)

Profiles §2's Binance assert-or-refuse cell reads:

> `GET /fapi/v1/multiAssetsMargin`==false; `GET /fapi/v1/symbolConfig`.marginType==ISOLATED (or `positionRisk`); **Portfolio-Margin OFF**; `positionRisk.isAutoAddMargin`==false; **one-way** `positionSide/dual`.dualSidePosition==false; configured leverage matches pin

Every item in that cell names a `GET` **except** Portfolio-Margin, and no other
section of either document supplies one. The account-model row (§2, line 36)
states the requirement — *"Portfolio Margin / Unified MUST be OFF — it would fold
spot into futures collateral"* — but a requirement is not a read path.

This is **[[an-absence-claim-is-a-claim]] inverted**: the check reads as covered
because it sits in a list of covered checks, and the missing element is one
column of one cell. It is also the shape §46 §8e names — the artifact that would
reveal the gap is the artifact everyone treats as the answer.

**Why it cannot be defaulted to "passed".** §4.1's whole property is *"the spot
leg cannot be seized to cover the perp loss"*, and Portfolio Margin is the one
setting that folds spot into futures collateral — i.e. the single account state
that falsifies the flagship's safety premise outright. A check whose failure
would invalidate the premise cannot be the check that silently passes.

Candidate read paths exist in the assistant's recollection and are **not
recorded here**: `venue-profiles` takes verified facts only, and a summarizer or
a recollection is the channel that has already misread a load-bearing number
twice in this project (`/api/v3/account`'s weight, and #180's three
disagreeing candidates). See [[a-summarized-fetch-is-not-a-source]]. Verification
is blocked by the same AWS WAF challenge that has blocked #180 across three
sessions.

**Ruling (operator, session 109): operator attestation with default refuse.**
See §73c-1.

### F2 — The borrow field has two folds wanting opposite defaults (design trap, already labelled)

`engine/venue/balances_bybit.go:193` reads an **empty** `borrowAmount` /
`spotBorrow` as *"the venue stated no borrow figure"* and lets the row pin. Its
comment names this slice as the reason that is safe:

> the §4.1 posture check owns "borrow must be zero" … a wallet with nothing to
> borrow must not turn every wallet read into a failed one.

Task #179 was right to do that — a balance reader that failed on every account
with no borrow facility would be unusable — and it deliberately did **not**
assume a sibling gate had run (`UnpinnedBorrowOutstanding`'s comment: *"this port
reports about its OWN evidence"*).

But it means the two consumers of one field need **opposite** treatments of
absence:

| Consumer | Absent borrow field means | Because |
|---|---|---|
| `SpotBalances` (#179) | no borrow stated → the row still pins | a total is being derived, and a missing borrow does not corrupt it |
| §4.1 posture (this slice) | **cannot conclude → refuse** | the claim being asserted *is* "borrow is off", and silence is not a zero |

**Therefore the posture check MUST NOT reuse `bybitCoinBorrowed`.** Reusing it
imports the permissive arm into the one gate whose entire purpose is refusing
borrow, and the resulting system passes its borrow check on a venue response that
said nothing about borrow. This is [[two-folds-over-one-record-need-opposite-defaults]]
exactly, and it is the second appearance in two slices of
[[a-checker-consuming-a-value-marked-not-a-conclusion]] (§70i's first major was
the same family).

The posture read gets **its own** predicate with its own empty-string arm, and
that arm is a refusal carrying a named reason.

### F3 — The leverage pin's natural implementation is vacuous

"configured leverage matches pin" has **no config field today and no producer**.
The implementation a session in a hurry writes is: read the venue's configured
leverage at boot, store it as the pin, compare on subsequent reads. That check
compares the venue against itself and **cannot fail on the first boot** — which
is the boot that matters, because it is the one that would catch an account
configured wrong before we ever touched it.

This is [[shared-constant-makes-its-own-check-vacuous]]: one value feeding both
the setter and the verifier stays self-consistent under any mutation. The pin is
an **operator-declared** number in config, stated independently of what the venue
reports, and an **absent** pin is a refusal rather than a default (§3.6 rule 4 —
the zero value is unusable; an unpinned leverage reads as "any leverage is fine",
which computes the safety gate in our favour).

### F4 — The auto-add-margin check is roster-scoped, and the position-scoped version passes vacuously at boot

Profiles §2's Bybit cell: auto-add-margin OFF **"for every configured linear
symbol (not only open positions)"**. The parenthetical is load-bearing and
already anticipates the mistake. An implementation that iterates open positions
checks **zero symbols on a flat account** — which is the state at every cold boot
before the first trade, i.e. precisely when the gate is supposed to speak.

Coverage is answered from the **configured roster**, which is the same ruling
task #179 reached for spot baselines (*"coverage answered from the configured
roster rather than the journal"*). The precedent is followed rather than
re-derived.

### F5 — `unifiedMarginStatus` is an external enum and needs default-deny

Profiles §2 admits `unifiedMarginStatus ∈ {3,4,5,6}`, refuses classic `1`, and
notes 5/6 are expected for new accounts. **`2` is unlisted**, and a value the
venue adds next year is unlisted by construction. Both refuse: the admitted set
is a whitelist, not a blacklist of known-bad values. This is the house default
direction — the failure from forgetting should be a refusal that names the value,
not a posture silently accepted because nobody enumerated it.

### F6 — The pre-trade guard has no caller yet, and must not pretend otherwise

§6 step 3's `risk.Gate` is where the pre-trade re-verify lives, and **nothing in
this system runs a trading loop today** (§12 item 4's own finding from #197:
nothing sizes a trade). So the pre-trade half of "boot *and* before each trade"
has no consumer this slice can wire.

The answer is **one use case with two callers, one of which lands later** — not a
second code path built speculatively. Boot calls it now. The pre-trade caller
lands with the trading loop, and that debt is recorded in §73f and in the task
list rather than left to be rediscovered. The failure mode being avoided is
[[a-missing-consumer-leaves-the-generic-verdict-speaking]]: an unconsumed guard
is not neutral, because the generic path answers in its place.

---

## §73c — The three rulings (operator, session 109)

### §73c-1 — Portfolio Margin: operator attestation, default refuse

No attestation → `GateAccountPosture` fails with a **named** reason
(`PostureUnattestedPortfolioMargin`). The operator records the fact
out-of-band and pins it, and the gate consumes the attestation.

**Shape follows #179's spot baseline**, which is the closest precedent in the
repo for "a fact only a human can establish, consumed by an automated gate":

- an **inputs-not-conclusions** document — it records *what was observed and
  when and by whom*, not the verdict "posture OK";
- pinned by a **two-act** `{path, sha256}` config edit, so writing the document
  and trusting it are separate deliberate acts;
- invalidated by **events, never by time** — an attestation does not silently
  expire into a pass or a fail on a clock tick;
- **absent → refuse**, never absent → pass.

**What the attestation does NOT do:** it does not become a venue fact. Nothing is
written into `venue-profiles` §2's verified table on the strength of an operator
attestation; §2 gains a marked note that the read path is **unknown** and that
the gate is attestation-backed until one is verified. The distinction is the
whole point of Profiles being verified-only.

**Rejected:** treating the sibling checks as implying PM is off. Portfolio Margin
is a different **account type**, not a setting within the classic account, so the
value of a classic-account setting establishes nothing about it — and the
implication would have to be argued from venue behaviour nobody here has read.
That is [[rejected-alternative-is-an-unchecked-claim]] waiting to happen: the
argument would be recorded once and never re-derived.

### §73c-2 — Pre-trade: a REST read before every trade

Every trade performs the full posture read regardless of snapshot age. The TTL is
an **additional upper bound** on any cached snapshot, never a way to skip the
read.

§6.3's own text is the argument: *"position-mode changes emit **no** venue event
on either venue, so a TTL + REST re-verify is the only reliable guard — never
event-presence."* Under the skip-inside-TTL reading, a position-mode flip inside
the window is undetected, which is the exact case the sentence says the re-verify
exists to catch. The conservative reading is correct under both parses of the
`and`; the permissive one is correct only under one. See
[[an-unverified-split-wants-the-shared-model]].

**Cost is affordable and the reason is structural, not a hope:** cash-and-carry
opens are hours apart, not sub-second. This slice records the measured
per-posture-read weight on both venues when the codecs land, so the figure is a
number rather than an assurance.

### §73c-3 — Reads only; posture is asserted, never repaired

The engine never mutates account posture. Profiles §2's `POST` setters are
recorded as **operator** acts and no code path in this slice calls one.

Two grounds. First, an account-wide, effectively irreversible mutation inside an
automated boot path is the wrong default. Second, and stronger: **a posture the
bot can repair is a posture whose violation stops being evidence.** The check's
value is that a wrong margin mode means *something happened that we do not
understand* — an out-of-band change, a wrong account, a compromised key. Silently
correcting it discards exactly that signal and leaves the system trading on an
account whose history it has just erased.

**Rejected:** an `arblab posture set` operator command in this slice. Not wrong
in principle, but it is a new credentialed **write** path, and with credentials
deferred it could not be tested against a venue at all — a write path whose only
evidence is a fixture is the shape this project has repeatedly paid for. It
stays available as a later slice once #171's credential exists.

---

## §73d — Architecture

### The `risk` bounded context is created here

§4's port declaration says `AccountPostureSource` is *"Consumed by risk — the
safety-posture guard (§4.1)"*, and `oms/application/reconcile.go` already states
the ownership: *"the §4.1 account posture is risk's and engine/venue's"*. So
`internal/risk/` is created rather than the check being parked in `oms`.

`CLAUDE.md`'s layout diagram already lists `risk/` among the bounded contexts —
along with `audit/` and `treasury/`, neither of which exists either. **The
diagram is a claim about the domain, not a description of the tree**, and this
slice makes one of the three true. That is worth noting in its own right: it is
the same class of gap #197 found in §12 one directory up, and the same class the
`SPEC EDIT OWED` grep cannot see.

```
internal/risk/
  domain/                  the posture rules — pure predicates, no I/O
  application/             the guard use case; folds rules → a posture verdict
  application/ports/       AccountPostureSource, in risk's own vocabulary
  infrastructure/driven/   ACL over engine/venue's codecs
```

`hexboundary` bites immediately: `risk/domain` and `risk/application` may not
import `infrastructure/` or `engine/`, so the port speaks risk's types and the
driven adapter translates — mirroring `oms/infrastructure/driven/balances.go`.

### Venue side

`engine/venue/posture.go` + `posture_binance.go` + `posture_bybit.go`, following
the `balances.go` / `positions.go` pattern exactly (codec + per-venue wire types
+ a client method stamping `ObservedAt` off `SignedClient.ServerNow`).

New Bybit endpoints join `bybitEndpointRegistry` with their fold names and
dimensions — #186's boot check compares profile limit names against
fold-derived names, so an endpoint added without a roster row is caught there.
Binance weights are recorded per endpoint; **where a weight is unverified, the
conservative candidate is charged and the reasoning is written at the call
site**, which is #180's discipline and its file comment is the template.

### The verdict type

The guard returns a **verdict carrying every failed check by name**, not a
boolean and not the first failure. Two reasons, both paid for elsewhere in this
repo: a first-failure-only checker never demonstrates the deeper claim
([[a-config-listing-two-claims-checks-only-the-shallower]]), and an operator
looking at a refused boot needs the full list to act once rather than
iteratively.

---

## §73e — Rejected alternatives (do not silently reopen)

1. **Park the posture check in `oms` and skip creating `risk`.** Rejected: §4
   assigns the consumer, `reconcile.go` already names the owner, and the
   §7.5 checklist comment argues at length that a context declaring gates it
   cannot report is the dangerous shape. Parking it in `oms` makes `oms` the
   owner of a gate whose inputs it does not own.
2. **One posture read shared between boot and pre-trade via a cached snapshot.**
   Rejected by §73c-2 — the cache is exactly what the REST re-verify exists to
   distrust.
3. **Reuse `bybitCoinBorrowed` for the borrow check.** Rejected by F2. It is the
   single most likely shortcut here and the code it lives in warns against it.
4. **Derive the leverage pin from the venue at first boot.** Rejected by F3 —
   self-consistent under mutation, so the check cannot fail when it matters.
5. **Iterate open positions for auto-add-margin.** Rejected by F4 — vacuous on a
   flat account, which is boot.
6. **Treat `unifiedMarginStatus` as a blacklist (refuse only `1`).** Rejected by
   F5 — default-deny; an unlisted value refuses.
7. **Build the pre-trade path speculatively with no caller.** Rejected by F6 —
   one use case, the second caller lands with the trading loop, the debt is
   recorded.

---

## §73f — What this slice deliberately does NOT do

- **No live venue verification.** Credentials are deferred, so every codec is
  fixture-tested and the posture read has never met a real account. The specific
  exposure: a wire-shape error (a field name, a nesting level, an enum spelling)
  would be invisible to every test here and would surface as a refusal on the
  first credentialed boot. That direction is a **halt**, which is the acceptable
  one, but it is not "verified".
- **No setters** (§73c-3).
- **No pre-trade caller** (F6) — the guard exists and boot calls it.
- **Nothing entered into `venue-profiles` §2's verified table** on the strength
  of an attestation or a recollection (§73c-1).
- **No sizing, no `MarginGateway`, no tier read** — §12 item 4's other clauses.

---

## §73g — Collapse checklist, run BEFORE the tests

Per the house rule that the checklist runs before the mutations, the fixture
parameters pinned to a value that collapses a distinction the code makes:

1. **One configured symbol** collapses F4's roster claim — "every configured
   symbol" and "the first configured symbol" give the same answer. Fixtures use
   **≥2**, with the violation on the **second**.
   ([[fixture-count-of-one-collapses]])
2. **One venue** collapses every per-venue rule into "the rule". Both venues'
   fixtures exist for each shared concept.
3. **An all-failing posture** collapses "reports every failure" into "reports
   something". Fixtures pin exactly **two** simultaneous failures and assert
   both appear, plus a positive control where posture is clean.
4. **An empty borrow string and a zero borrow string** are the same answer under
   the wrong predicate. Both fixtures exist and must give **different** verdicts
   (F2): `""` → refuse-inconclusive, `"0"` → pass.
5. **A leverage pin equal to the venue's reported value in every fixture**
   collapses F3. At least one fixture has pin ≠ venue value in each direction.
6. **`unifiedMarginStatus` fixtures at 3 and 1 only** collapse F5's whitelist
   into a blacklist. Fixtures include the unlisted `2` and an out-of-range
   value, both refusing.
7. **The gate's ExitClass membership** is not checked by the compiler, only its
   exhaustiveness. A test asserts a blocking `GateAccountPosture` now yields
   `ExitIncomplete`, and it is mutated to confirm it bites.

---

## §73i — What the venue slice found once it was built (session 109)

Items 1 of §73h landed: `posture.go`, `posture_binance.go`, `posture_bybit.go`,
the two Bybit roster rows, and `posture_test.go` (13 tests). Two findings came
out of writing it that the design pass had not seen.

### F7 — The registry↔profile agreement is checked in ONE direction only

Adding two endpoints to `bybitEndpointRegistry` left the whole suite green, and
the reason is worth recording: `BybitLimitReader.CheckFold` validates that every
**profile-declared** limit exists in the registry, and never the reverse. So a
registry endpoint that no profile declares passes every check at construction.

It is **not** silent at runtime, which is the part that matters and the part
that was verified rather than assumed: `Governor.Observe` returns
`ErrLimitUnknown` for a name it cannot place, and its comment names this exact
case — *"a header the adapter parses and the profile does not declare — a budget
going entirely uncounted, which is the failure that looks exactly like
everything working."* #185 and #186 built the machinery for precisely this.

**So this slice creates a configuration requirement, not a defect:** every
profile must declare `account_info` and `spot_margin_status` (endpoint scope,
`DimensionRequests`, 1s window) before the first posture read, or the fold is
refused by name. It is recorded as work item 7a rather than left for the first
credentialed boot to discover.

The one-directional check is worth its own look later — the asymmetry means the
registry can grow past the profile without anything objecting until traffic
flows — but widening it belongs with the config work item, not here, and it is
listed as a candidate rather than done.

### F8 — `-run Posture` does not select most of this file's tests

Eleven of the thirteen new tests are named for the venue and the surface
(`TestBinanceDecodeMultiAssets…`), not for the word "Posture", so the obvious
invocation ran **four** of them and reported `ok`. That is
[[a-probe-an-invocation-cannot-select]] in miniature, caught here only because
the pass count was checked against the number of tests written.

No code changed for it — renaming thirteen tests to satisfy a filter is worse
than knowing the filter — but the mutation campaign at slice close runs against
the **package**, never a name pattern, and this record is where that is stated.

### The four mutants run so far (the full ≥15 campaign is item 8)

| # | Mutation | Verdict |
|---|---|---|
| M1 | an empty borrow field reads as `PostureStated` | caught (1) |
| M2 | a **missing** bool decodes as the passing `false` | caught (2) |
| M3 | an empty `symbolConfig` array read as an empty roster | caught (1) |
| M4 | contradicting auto-add-margin spellings accepted | caught (1) |

M2 was **re-aimed before it counted**: the first version replaced the nil guard
with `if false`, which makes the mutant dereference a nil pointer and die by
crash rather than by wrong answer. The harness reported `caught`, but a kill by
crash leaves the claim — *"a missing field must not read as false"* — unchecked.
The counted mutant returns a stated `"false"` instead, and two tests fail on it.
Same trap as `PANIC!!`, one step earlier: the verdict and the outcome are not the
same thing.

---

## §73h — Work items

1. `engine/venue` posture codecs, both venues + roster rows + weights.
2. `internal/risk/domain` — the rules and the verdict type.
3. `internal/risk/application` + ports — the guard use case.
4. `internal/risk/infrastructure/driven` — the ACL adapter.
5. The Portfolio-Margin attestation document + its two-act config pin.
6. The leverage pin config field (absent → refuse).
7. `app/reconcile` wiring; `GateAccountPosture` moves across `ExitClass`.
   - **7a.** Declare `account_info` and `spot_margin_status` in every rate-limit
     profile (endpoint scope, `DimensionRequests`, 1s window) — §73i F7. Without
     them the first posture read's fold is refused by name.
8. Collapse checklist (§73g) as fixtures, then a ≥15-mutant campaign.
9. Spec edits → v0.9.51: §4.1/§6.3/§7.5 annotations, Profiles §2's marked
   unknown read path, §12 item 4's slice annotation, ADR §3's dated block.
10. Final fresh `code_diff` review.
