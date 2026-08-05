# M3 task #179 — the spot leg's exposure baseline (§7.5's invariant covers one leg of two)

**Date:** 2026-08-05 (session 103) · **Status:** DESIGN — WIP
**Reads:** core §7.5 (the spot paragraph, edited v0.9.37/v0.9.39), §5.1's hedge
group, §4.1/§4.2, record §56g/§56j (slice F2), record §69 (#183's document
precedent), Profiles §2/§3/§4/§6.
**Review:** round 1 (second-model adversarial, design_doc) returned NO-SHIP
with five majors, ALL validated and folded in: (1) record-time snapshot → the
six-step protocol with quiesce bound and cumulative re-read bracket (§70b);
(2) balance port discarded verbatim evidence → `Words` on every row, one
shared per-venue derivation for adapter and loader (§70d); (3) two priorities,
no carrier → constructor-pinned per call site, zero refused (§70d/§70f);
(4) `OutcomeIncomplete` unrepresentable → `SpotUnconcluded` + explicit
reduction precedence (§70c); (5) required `spot_baseline` circular on a virgin
deployment → optional in schema, absence is the gate's own named refusal
(§70g). Checklist items 12–16 added for the five.
Round 2 returned NO-SHIP with four majors + one minor, ALL validated and
folded in: (1) the virgin deployment PASSED vacuously — journal-derived
coverage set → the configured spot roster threads into `NewJoiner` and Assert
walks it (§70c); (2) the `AuthorizedAt` quiesce anchor was wrong for a
long-lived order and the staleness argument overclaimed — replaced by the
settle-window re-read of BOTH surfaces with the residual venue-staleness
premise named and bounded (§70b); (3) the account label is mutable config —
the document binds the credential FINGERPRINT the spend lock already computes
(§70b); (4) the scalar `ours` was an unverifiable attestation and a
path-pinned document was replaceable in place — per-order rows recomputed at
load and cross-checked against the chain-marked prefix, and the config pin is
`{path, sha256}` (§70b/§70g); (minor) `SpotUnconcluded` rendering goldened
(§70h). Checklist items 17–21 added.
Round 3 returned NO-SHIP with five majors + one minor, ALL validated and
folded in: (1) the Binance account read is weight **20**, not the 10 a
summarized fetch misread — settled by exact grep of the raw doc, profiles
corrected, call-cost fixture + 20→10 mutation added (§70d/§70h.22); (2) a
late fill visible in the wallet before the trade list would falsely LATCH at
boot — the second look: on a non-empty delta the Reconciler waits
`spot_settle`, re-runs the JOIN (idempotent), re-asserts, and only a STABLE
delta latches (§70c/§70h.23); (3) the terminal requirement trapped
`Planned`-only orders with no venue side and no discard record —
`MayExistAtVenue` scopes it (§70b); (4) a baselined asset that is another
spot pair's QUOTE makes consideration flows invisible to `ours` —
default-deny at construction, quote carried on the roster row, the
quote-flow retention alternative recorded as rejected (§70c/§70h.24);
(5) the record protocol's join rides `driven.OrderResolver`'s hard-pinned
Safety — the resolver's priority becomes constructor-pinned like the balance
adapter's (§70f/§70h.13); (minor) the settle bound is NAMED (`spot_settle`,
§5.2) and the stale `AuthorizedAt` fixture in item 12 replaced.

## §70a — what is owed, and why the gap concludes rather than halts

§7.5 asserts "every unit of the venue's *position* is attributable to an order
this system journaled" — per **derivative** instrument. Neither venue has a
position surface for spot (a spot balance is not a position), and the balance
that stands in its place contains coins this system never bought. §56g's ruling:
the spot leg is REPORTED on every boot (`Join.Unasserted`,
`UnassertedNoPositionSurface`) and does not hold the posture, "until a baseline
is defined". This slice defines it.

The failure being closed is the rare CONCLUDING one: a pre-existing 1 BTC reads
as our long, nets against the perp short, the group reads over-hedged, and a
reduction is sized in the wrong direction — from a number every invariant over
the group agrees with.

Three questions were left open by name: recorded **when**? by **whom**?
invalidated by **what**?

## §70b — RULING: the baseline is `balance − ours`, recorded by an operator
## command, bound to the journal's chain

**The construct.** A baseline is a per-`(venue, account, asset)` quantity: the
portion of the venue's spot balance NOT attributable to this system, at a named
instant. It is defined as `baseline := B − ours`, where `B` is the venue's
balance read at record time and `ours` is the fee-adjusted net this system's
journaled spot orders account for (§70c's formula), rebuilt through the same
replay + venue join §7.5's boot uses.

**Recorded by whom: an operator running `arblab baseline record`.** Never
automatically, and never at boot — a boot that minted its own baseline would
absorb any unexplained delta into it, laundering exactly the thing the invariant
exists to catch. Two deliberate acts are required before a baseline governs
anything: running the command (which writes a NEW dated document under `O_EXCL`,
#183's shape — no overwrite path exists), and editing the deployment config to
pin the new document's path. The old document stays on disk as history.

**Recorded when: whenever `ours` is exactly determined — and "determined" is a
PROTOCOL, not a predicate** (design-review finding 1: terminal state does not
close the fill ledger — §5.3 admits a post-cancel late fill indefinitely, and
the join's order read, executions sweep, and the new balance read are three
independent calls with no common snapshot). The record protocol:

1. Replay the journal; require every journaled spot order on the configured
   venue **whose state answers true to `MayExistAtVenue`** terminal. A
   `Planned`-only order is excluded EXPLICITLY (round-3 finding: it is
   non-terminal, has no venue side by §5.3's own rule, contributes exact
   zero, and the journal has no discard record kind — a crash between plan
   and dispatch would otherwise make recording permanently impossible).
2. Join: sweep concluded (not `Incomplete`), no pending-`Absent` under §5.3,
   cross-check agreed (not `Unattributed`), no unpinned balance semantics
   (§70d).
3. Read `B`.
4. Wait the **settle bound** — §5.2's `spot_settle`, a required-when-spot
   duration validated positive (for the IOC-only MVP, seconds), shared by
   this protocol and §70c's boot second-look because they are one rule with
   two callers. No activity anchor is derived from `AuthorizedAt` or journal
   state records (round-2 finding: authorization is days old on a long-lived
   order canceled seconds ago, and state records carry no instant) — the
   bound is observed, not inferred.
5. Re-read every spot order's venue cumulative (`Resolve` — cheap) AND `B`,
   and require BOTH unchanged from steps 2–3. A late-reporting execution
   either moved a cumulative across the window (refused here), was already
   counted by the cumulative but missing from the sweep (refused at step 2 as
   `Unattributed`), or moved the balance (refused here). The residual premise
   is NAMED rather than argued away: a venue order surface stale by MORE than
   the settle bound, consistently across both reads, is undetectable — that
   premise is bounded, per-venue, operator-owned via the config field, and it
   is the same kind of claim (not the same claim) as `MaxLanding`: how stale
   the venue's own read side may be about its own matching engine.
6. Write the document.

The command's transport is a `NewSignedClient`, so it CANNOT run without
holding the venue's one-spender lock (v0.9.47) — a second lock-aware spending
process on this host cannot execute trades mid-record; across hosts the
standing operational premise applies, unchanged.

The gauntlet refuses rather than waits in every arm, and it does NOT require
flatness (`ours == 0`):

- *Rejected: flat-only recording.* Fees paid in the base asset make an exactly
  zero net practically unreachable after any trading, and a mid-carry
  re-record (after an operator deposit) would require flattening a healthy
  hedge. Terminal-and-concluded gives `ours` exactly, which is all the
  subtraction needs.
- *Rejected: an `--absorb-delta` flag distinguishing first record from
  re-record.* Recording IS absorption — `B − ours` folds any currently
  unexplained delta into the baseline by construction, deliberately: that is
  what an operator who just deposited BTC needs. The guard is visibility plus
  the two-act rule, not a flag: when a prior baseline is configured, the
  command PRINTS the delta being absorbed (old baseline vs new, per asset), and
  the config edit is a second, diffable act. A flag would be clearance-by-flag
  inside one command — diagnosis and forgiveness in a single keystroke.
- **The `Unattributed` refusal is the non-negotiable one:** an execution the
  cross-check could not name is real exposure with no owner, and recording over
  it folds it into "not ours" permanently, silencing the alert that names it.

**Invalidated by what: any non-journaled balance change, detected rather than
enumerated.** Deposits, withdrawals, transfers, dust conversion, earn/interest,
airdrops, manual trades — the design does not enumerate them, because the
assertion catches the effect of all of them identically: the next boot's
`B_now − baseline ≠ ours_now` is a latched operator alert (§70c). The remedy is
inspect → re-record → re-pin. A baseline has **no age bound**: it is invalidated
by events, never by time, and its ObservedAt is carried for the operator's
diagnosis, not for a staleness gate.

**Bound to the journal's chain, because `ours` is a claim about ONE history.**
The document carries a chain mark — the journal's head `(seq, hash)` at record
time — and the boot loader verifies `HashAt(seq) == hash` before the baseline
governs anything. `ours_record` cannot be recomputed at load (fills are not
journaled), so it is an attestation; the mark pins WHICH history it attested
against. A journal restored from backup, reset, or truncated below the mark
fails the verification and the baseline is REFUSED by name — the operator
re-records — rather than silently subtracted against a history it never
described. A baseline recorded over an empty journal carries `seq 0` and the
verification is vacuous (every journal extends the empty one; `ours` was zero,
so nothing was attested). This needs two read methods on the journal's replay
surface (`Head`, `HashAt`) — reads, on the read port, per the Journal/
JournalReplay split.

**The document stores inputs, not conclusions** (this repo's stored-form rule,
tightened by round-2 finding 4: a stored SCALAR `ours` was still a conclusion —
schema-valid arbitrary bytes with a true chain mark would have loaded). Per
asset: the venue's balance fields VERBATIM (Binance `free`/`locked` strings;
Bybit `walletBalance`/`locked` strings). Per spot ORDER counted by the record
run: `{client order id, side, venue cumulative, fees by asset}` — the rows
`ours` is a sum of. The loader RECOMPUTES: `B` from the verbatim words via
§70d's one shared per-venue function, `ours(S)` from the order rows by §70c's
own formula, `baseline = B − ours` last — and it cross-checks every stored
order id against the replayed journal prefix at the chain mark: a row naming
an order the prefix does not hold is refused, so the rows are bound to the
same history the mark names. What remains attested rather than recomputable is
only each row's `venue cumulative` (fills are not journaled); a late fill
admitted after record now shows up as a DIAGNOSABLE per-order disagreement
between the stored row and the live join, not an opaque asset-level delta.

Top-level bindings, each its own refusal: venue; account label (against the
deployment config); **the spending credential's fingerprint** (round-2 finding
3: the label is mutable configuration, so a credential swapped to another
subaccount under an unchanged label would pass every other binding — the
SHA-256 fingerprint the transport already computes for the spend lock is
recorded at record time and compared against the constructed transport's at
load; key rotation therefore forces a re-record, deliberately and stated);
ObservedAt (venue server time); the chain mark. **And the config pin is
`{path, sha256-of-document}`, not a path alone** (round-2 finding 4: `O_EXCL`
guards creation only, and a path-pinned file edited in place would change the
effective baseline with no second act — with the digest in config, replacing
the document WITHOUT the config edit is a named refusal, so the two-act rule
is enforced by construction rather than by convention). A document row for an
asset that is not the base asset of a configured spot instrument is REFUSED at
load (a row nothing consumes is a claim nobody checks).

*Rejected: baselining the quote/settle asset (USDT).* Quote balances change on
every fee, funding settlement, and §7.1 margin-staging transfer — all
legitimate, most not journaled as spot orders — so a USDT baseline would false-
alarm perpetually or force re-record-per-incident, training the operator to
absorb deltas reflexively. The hedge exposure the invariant protects is the
BASE asset; PnL economics read fills and funding, not balances. Recorded here
so the absence stays visible (§56j's practice).

## §70c — RULING: what is asserted for the spot leg, and at what granularity

**The claim:** per `(venue, account, base asset)` with a recorded baseline,
`B_now − baseline == ours_now` — every unit of spot balance movement since the
baseline is attributable to a journaled order. Equality is exact decimal
arithmetic (`fixedpoint`); there is no tolerance band, because both sides are
sums of venue-published decimal strings.

**`ours` is fee-adjusted, and the fee term is what makes the equality hold at
all:** on both venues a spot BUY's fee is charged in the RECEIVED (base) asset
by default, so the balance credits `qty − fee` while the order surface's
cumulative states gross `qty`. So, for base asset S at the configured
`(venue, account)`:

    ours(S) = Σ signed cumulative over spot orders whose base is S
            − Σ fees with fee.Asset == S over ALL spot orders at the venue

The fee sum ranges over ALL spot orders (any pair), not only S-based ones: a
BNB-discount fee on a BTCUSDT spot trade debits the spot BNB balance, so if BNB
were a baselined asset the fee that moved it was charged on another
instrument's order. It ranges over SPOT orders only: Binance futures fees
(USDT, or BNB under discount) debit the FUTURES wallet, which the spot balance
read never sees — summing them would subtract from a wallet they never touched
— and Bybit UTA perp fees settle in USDT only, which is not a baselined asset
(§70b). The formula is venue-agnostic because `Fill.Fee` carries the venue's
word with its asset tag (§5.1's no-FeeAsset ruling working as designed).

**Fee retention is the enabling change:** `domain.Ledger` currently keeps
cumulative quantity and a fill COUNT and discards `Fill.Fee`. `ledgerOrder`
gains per-asset fee totals, accumulated in `Apply` under the same
mutate-only-after-nothing-can-fail discipline, and `ports.OrderRecord` gains
`Fees []fixedpoint.Money` (sorted by asset — one entry per asset actually
charged). The fee's dimension crossing: netting a fee (Money) against a balance
(Quantity) crosses dimensions once, at one named call site, via the kernel's
existing codec surface (`Mantissa()` → `QuantityFromMantissa`, same asset tag,
same canonical scale — lossless, no rounding decision arises), with the
argument stated at that site: a fee charged in an asset is a flow OF that
asset, and where flows net against holdings the dimension is
quantity-of-asset. No kernel edit; the crossing stays greppable as the one
`QuantityFromMantissa` whose source is a Money.

**Granularity is the ASSET, not the instrument.** Two spot instruments sharing
a base (BTCUSDT, BTCUSDC) share one balance; a delta cannot be attributed
between them. The report is therefore a new `Assertion` field:

    SpotUnexplained []AssetDelta{Venue, Asset, Expected, Observed, Unexplained}

where Observed is `B_now − baseline` — carried pre-subtracted for the same
reason InstrumentDelta carries Unexplained: the operator acts on the number,
and a report that makes the reader subtract is one where a wrong-order
subtraction is invisible. Both signs latch: positive is exposure we cannot
account for, negative is coins the journal says we hold that the venue does
not. Non-empty `SpotUnexplained` is §7.5's "unexplained position delta" — alert
for operator clearance, latching, never auto-corrected — and joins
`Assertion.Holds()`'s disjunction.

**A second new field carries the refusal-to-conclude** (design-review finding
4: an unpinned Bybit row must become `boot.OutcomeIncomplete`, and an
`Assertion` that could express only deltas would force the seam to misfile it
as failed, latched, or passed):

    SpotUnconcluded []SpotUnconcluded{Venue, Asset, Reason}

— a baselined asset whose balance semantics were unpinned this read (§70d's
`locked != 0` / borrow arms). It does not join `Holds()` (latching a timer is
the halts-with-every-light-green failure) and it does not pass. The
reconciliation seam's reduction gains an explicit PRECEDENCE, asserted by
fixture: error → `OutcomeFailed`; `Holds()` → `OutcomeNeedsOperator`;
non-empty `SpotUnconcluded` → `OutcomeIncomplete`; else `OutcomePassed`. The
ordering is the safety claim: a boot with one pinned unexplained delta AND one
unpinned row latches — the incomplete finding must never mask the latching
one — and the both-present fixture is the discriminating one (§70h).

*Rejected: folding spot deltas into `Unexplained []InstrumentDelta` by
attributing to the sole configured instrument with that base.* Right answer
today (MVP trades one pair per base) and a wrong SHAPE: the attribution becomes
load-bearing the day a second pair lands, and nothing would refuse it — the
delta would be pinned on whichever instrument sorted first. The honest
granularity costs one type.

**Where it runs: inside `Joiner.Assert`, same gate — and the spot coverage set
comes from CONFIGURATION, never from the journal** (round-2 finding 1:
`Assert` derives its instrument set from dispatched orders, so a virgin
deployment — configured spot instruments, empty journal, no baseline — had
empty sets everywhere and the gate PASSED: the exact "satisfied on a boot that
issued no query" failure §7.5's G2 paragraph closes for scopes). `NewJoiner`
gains the configured spot instrument roster (the validated `InstrumentAssets`
table the module already holds), and Assert walks THAT roster: every
configured spot instrument is answered for — asserted against its baseline, or
reported `UnassertedBaselineUnrecorded` — whether or not any order ever
existed. The derivative half keeps its config-time coverage rule (config
validation already requires every derivative covered by a scope); the spot
half cannot use config-time enforcement because the baseline is deliberately
optional there (§70g), so the coverage check lives at assert time, in the
report. Otherwise `GateNetDeltaAsserted` covers §7.5's assertion and this
completes it; the ordering (replay → join → resolve absences → assert) is
unchanged and the spot half re-reads the projection at assert time exactly as
the derivative half does.

**A baselined asset MUST NOT be another spot pair's quote, and the refusal is
the constructor's** (round-3 finding: `ours(S)` counts base flows and S-
denominated fees, so a buy on ETHBTC — BTC the quote — debits the wallet's
BTC by the executed consideration while the BTC formula sees only fees; a
fully journaled order would latch a false delta). The roster row carries the
instrument's QUOTE beside its base, and a baseline for an asset that is any
roster row's quote is refused by name at construction. *Rejected: retaining
executed quote consideration and summing signed quote flows into `ours`.*
Right in principle and a whole second retention surface (per-order quote
totals with explicit rounding through the price multiplication) for a
configuration the MVP does not trade; the deny keeps the wrong number
inexpressible instead of computed. The dropped-instrument corner is closed by
the deny's own timing: while the pair was configured, the deny forbade the
baseline, so any pre-existing quote flows are absorbed into a baseline
recorded AFTER the pair left the config — `B − ours` absorbs history by
construction.

**A spot delta LATCHES only when it is STABLE across a second look, and the
second look re-runs the JOIN** (round-3 finding: at boot the balance read and
the sweeps that fed the projection are independent calls, so a late-reporting
fill visible in the wallet but not yet in the trade list would latch a false
operator incident — §7.5's G1 paragraph names freezing a self-clearing
condition into an operator incident as the failure). When Assert yields a
non-empty `SpotUnexplained`, the Reconciler waits `spot_settle`, re-runs the
JOIN over the same dispatched set (idempotent by the fill ledger's dedupe —
re-admitted rows are no-ops, late rows move the cumulative), and re-runs
Assert. An IDENTICAL delta set latches (`OutcomeNeedsOperator`); a changed
one — including a second join that did not conclude — is the world still
moving, `OutcomeIncomplete`, and the next boot re-derives everything. The
balance-only bracket was rejected as the fix because the projection is frozen
between two Asserts: the late fill's wallet effect is ALREADY settled, so
only a re-sweep can move `ours` to meet it. The record protocol's bracket and
this one are one rule — never mint a permanent fact from one read of two
surfaces that can lag each other — with the record command refusing and the
boot downgrading to incomplete, each in its own vocabulary.

**Boot-only, like the rest of the join.** The continuous/admission-time mirror
(re-asserting during the long-running engine's loop) lands with that engine's
own loop, on §69b's deferral shape: the executor's admission gate covers
filters because filters gate each package; the balance invariant is a
reconciliation fact with no per-package consumer yet. Deferred WITH the
argument, not silently.

## §70d — RULING: the balance read — a fourth port, and the venue facts

**A new driven port, not a method on `VenuePositions`:**

    SpotBalances interface {
        Balances(SpotBalanceScope) (BalanceReport, error)
    }
    SpotBalanceScope{Venue ids.Venue}          // Validate: venue set, and that is all
    BalanceReport{
        ObservedAt clock.ServerTime             // the venue clock instant of the read
        Rows       []AssetBalance               // {Asset, Total Quantity, Words []VerbatimField}
        Unpinned   []UnpinnedBalance            // {Asset string, Reason}
    }

Each row carries the venue's OWN fields verbatim (`Words`: Binance
`free`/`locked`, Bybit `walletBalance`/`locked` as strings), beside the typed
`Total` — design-review finding 2: the record command must store the venue's
words and a report that had already summed them would force either a second
decoder or a reconstruction, both of which violate the verbatim-evidence rule.
`Total` is derived from `Words` by ONE exported per-venue function in
`engine/venue`, and the baseline LOADER calls the same function on the stored
words — the live read and the boot-time recomputation cannot disagree about
what the fields mean, because there is one meaning in one place.

**The priority carrier is the CONSTRUCTOR, because this port is the first read
surface with two call sites on different rungs** (design-review finding 3:
§7.6 puts read priority at the call site; the positions adapter pins `Safety`
because reconcile is its only consumer, and this port has two). The driven
adapter takes the priority at construction — boot wiring builds it at
`Safety` (§7.5's reconcile is Safety-class), the record command builds its own
at `Routine` (an operator maintenance read must not spend the emergency
reserve) — and refuses the zero value, so a wiring that forgot to choose does
not silently inherit either rung.

The port split follows VenuePositions' own argument: a balance read is an
assertion about the account whose failure mode is an answer describing a scope
nobody meant — and a use case holding only `SpotBalances` cannot reach for a
per-order query to explain a delta away. The enumeration is UNSCOPED by asset
(§56b's rule, one surface over): the read must be able to answer for an asset
the caller did not name, and per-asset queries are exactly the reads that
cannot surface a coin nobody expected. An error is never an empty report
(§5.3's absence rule, §56b's costing: an empty report reads as a flat wallet,
which is what the invariant passes on).

Only rows for baselined assets are CONSUMED by the assertion; the enumeration
still returns everything the venue reported, and unconsumed rows are simply
unconsumed. A foreign COIN is deliberately not §7.5 "foreign": a coin balance
is not exposure anyone traded into existence on this account model (posture
forbids borrow; spot cannot be short), and pre-existing coins are the baseline
premise itself. Recorded so nobody "fixes" it into the Foreign list later.

**Binance (verified 2026-08-05, binance/binance-spot-api-docs GitHub, master):**
`GET /api/v3/account`, SIGNED, request weight **20** (round-3 finding: a
summarized fetch misread 10, the second model re-read the raw markdown, and
an exact grep settled it — the governor charges exactly the caller-stated
cost, so an understated weight admits past the venue's budget; the call-cost
gets its own fixture and a 20→10 mutation), on the SPOT family's weight
budget (family-scoped, existing profile shape). `omitZeroBalances`
default **false** and the adapter leaves it unset — every asset comes back, so
absence carries no conclusion and none is needed. Verbatim: `free` = "Amount
not currently committed to an open order", `locked` = "Amount currently
committed to an open order". **Total = free + locked** — both fields are
documented as partitions of holdings, so the sum is the venue's word, not an
inference. Rows never unpinned.

**Bybit (verified 2026-08-05, bybit-exchange.github.io/docs/v5):**
`GET /v5/account/wallet-balance`, `accountType=UNIFIED` required, `coin`
OMITTED — verbatim: *"If not passed, it returns non-zero asset info"* — so the
unscoped call is the enumeration kind whose silence carries the conclusion:
an absent coin is zero. (Passing `coin=` would flip the absence semantics to
the uninterpretable kind; the adapter never passes it. Profiles §9's
version-is-part-of-the-requirement rule, arriving as a parameter.)
Per-endpoint budget: the Account table row is **50/s** — a new roster entry
(§70f).

**The Bybit total is an UNVERIFIED SPLIT, and the both-readings rule is the
model** (this repo's recorded shape: enforcing one reading of an unverified
semantic converts uncertainty into a deterministic failure). The docs state
`walletBalance` = "Wallet balance of coin" and `locked` = "Locked balance due
to the Spot open order", and NO sentence states whether walletBalance includes
locked. The equity formula (equity = walletBalance − spotBorrow +
unrealisedPnl + option value) never adds `locked`, which IMPLIES inclusion —
an inference, named, not relied on. The adapter therefore computes the total
under BOTH readings and:

- `locked == 0`: the readings coincide; the row is pinned at `walletBalance`.
- `locked != 0`: the readings differ by exactly `locked`; the row is reported
  in `Unpinned` with the reason naming the unverified fact. The assertion for
  that asset is then a **ran-and-concluded, work-remains** outcome
  (`boot.OutcomeIncomplete`'s cell — reconcile-only, re-reportable): a resting
  spot order completes or cancels, the next attempt pins. MVP's spot flow is
  IOC, so the disagreement window is the exception, not the boot.
- `borrowAmount != 0` or `spotBorrow != 0`: unpinned with a borrow reason —
  borrowed coins inflate walletBalance without being holdings. The §4.1
  posture check refuses borrow separately; this port reports about its OWN
  evidence rather than assuming a sibling gate ran (gates are reported about
  their own evidence, §7.5's own rule).

**#171's testnet session is the LOOSENING trigger** (Profiles §4's shape): a
measured walletBalance-vs-locked relationship pins the unverified split, and
the `locked != 0` arm tightens to arithmetic. Never a correctness
prerequisite.

`arblab baseline record` refuses to mint a baseline from an unpinned row, for
the same reason it refuses `Unattributed`: a baseline is an attestation, and
attesting an ambiguous `B` stores the ambiguity as fact.

## §70e — RULING: the reason taxonomy — one member dies, one is born

`UnassertedNoPositionSurface` (does not hold posture) loses its last producer:
a spot instrument WITH a loadable baseline is asserted and leaves `Unasserted`
entirely; one WITHOUT becomes the new member. A declared member nothing emits
is this repo's recorded false-handled shape, so the member is DELETED, and
`exhaustiveswitch` walks every switch to prove the roster edit reached them.

The new member:

    UnassertedBaselineUnrecorded   // spot instrument whose base asset has no
                                   // loadable baseline row — HOLDS the posture

It holds, and the §56g exemption argument does not apply to it: that argument
("a leg whose invariant can NEVER be asserted would halt a correct system for
ever") was about a permanent venue-imposed gap, and the gap is no longer
permanent — recording a baseline is a one-time setup act on the same footing
as `spend_lock_dir`, and §7.5's own sentence scoped the exemption "until a
baseline is defined". The exemption's membership is now EMPTY: every
`UnassertedReason` holds the posture, `HoldsPosture` keeps its signature and
its per-member argument, and §7.5's spot paragraph is rewritten to say so
(spec edit, §70g). A rejection's reason expired and the rejection goes with
it; recorded here so no later session re-derives the old exemption from the
old prose.

*Rejected: keeping `UnassertedNoPositionSurface` for a deployment that trades
spot but "opts out" of baselines.* There is no such deployment shape: the
baseline document is pinned by config, config is validated at boot, and a spot
instrument without a baseline row is exactly `UnassertedBaselineUnrecorded`.
An opt-out member would be a permissive default wearing a taxonomy entry.

## §70f — transport and rate-limit consequences

**Bybit roster gains one row:** `/v5/account/wallet-balance` →
`wallet_balance`, `DimensionRequests`, 1s window (venue-published 50/s;
ceiling is operator config per §66's minimum-across-categories arithmetic —
here there is one category, the account surface). §66's complete-roster rule
then REQUIRES every Bybit profile to declare `wallet_balance` — existing
configs and fixtures refuse at construction until they do. The refusal is the
feature (§65's compatibility precedent): a profile missing it would boot and
then refuse the Safety-priority balance read at exactly the reconcile moment
that needs it.

**Binance adds no budget shape:** weight 20 on the existing spot family
weight budgets; the cost validator's coverage rule is already satisfied by
family-scoped weight+orders declarations.

**The order-read surface's priority becomes constructor-pinned too** (round-3
finding: the record protocol re-runs the join and `Resolve`, and
`driven.OrderResolver` hard-pins `PrioritySafety` on the argument that its one
caller was recovery — the record command is a second, maintenance caller, and
its sweep can issue up to 32 pages per order, so the pin would spend the
emergency reserve well beyond two balance reads). Same shape as the balance
adapter: `Safety` in boot wiring, `Routine` in the record command's, zero
refused.

**Priority: constructor-pinned per call site** — `Safety` for the boot
adapter (§7.6 puts §7.5's reconcile in the Safety class), `Routine` for the
record command's own adapter, zero value refused (§70d's carrier ruling,
review finding 3).

## §70g — spec edits owed, and what is NOT in this slice

**Spec edits (land before slice close, versioned):**
- §7.5's spot paragraph: the baseline is now DEFINED — rewrite "until a
  baseline is defined" into the construct (recorded by operator command,
  `B − ours`, chain-marked, invalidated by non-journaled change, latching
  delta alert), and retire the must-not-hold exemption (its membership is
  empty; the halts-forever argument's premise expired with the gap's
  permanence).
- §5.2: `spot_baseline` (`{path, sha256}` of the pinned document — the digest
  is the two-act enforcement, §70b) and `spot_settle` (the settle bound, a
  positive duration, required whenever any spot instrument is configured —
  both the record protocol and the boot second-look read it) — `spot_baseline`
  **OPTIONAL in the schema, and the optionality is load-bearing** (design-review finding 5: a
  required field is circular on a virgin deployment — the record command runs
  off the same validated config the field would refuse to load, and a "dummy
  path" workaround trains operators to stage invalid configs). Absence is not
  a config error; it is the INVARIANT's own named refusal: every spot
  instrument reports `UnassertedBaselineUnrecorded`, which holds the posture.
  The first-record flow is therefore: config without the field → boot holds,
  naming the gap → `arblab baseline record` (same config) → add the pin.
  When the field IS present, the document must load, chain-verify, and bind
  (venue/account/rows) or the boot errors — a pinned document that will not
  load is a wiring defect, not a gate refusal.
- Profiles: new dated block — the two balance endpoints' verbatim facts
  (§70d), the Bybit walletBalance/locked NAMED INFERENCE with #171 as
  loosening trigger, the wallet-balance 50/s row beside §4's table.
- adr-history: dated block for #179.

**Not in this slice (recorded so the absences stay visible):**
- **Continuous re-assertion** in the long-running engine (§70c's deferral,
  with the argument).
- **Quote-asset baselines** (§70b's rejection, with the argument).
- **FundingLedger** (§56j's standing deferral, unchanged — settles in quote,
  moves no baselined asset on either venue's wallet the spot read sees).
- **#184's Dispatch conformance** — untouched, still owed to its own task.
- **A baseline EDITOR/lifecycle tooling** beyond record: list/diff/expire are
  operator conveniences with no invariant content; the two-act rule is the
  control.

## §70h — test plan: the collapse checklist run at design time

Fixture dimensions that must NOT collapse (each names the distinction it
protects):

1. **Fee ≠ 0 in the base asset** on at least one buy — or `− fees` and
   "no fee term" are the same formula. And a fee in a NON-base asset (quote or
   BNB-shaped) on the same venue — or "fees with asset==S" and "all fees"
   collapse.
2. **A fee on a DIFFERENT pair debiting a baselined asset** (the BNB shape) —
   or per-order-base fee scoping and per-asset scoping collapse.
3. **Baseline ≠ 0 and ours ≠ 0 simultaneously** — or `B == ours` (no-baseline
   comparison) and `B − baseline == ours` are the same predicate. The §56g
   motivating case IS this fixture: pre-existing coins + our net both nonzero.
4. **Both delta signs**: venue above (unexplained surplus) and venue below
   (missing coins) — a negative-direction mutant must die on its own fixture.
5. **Bybit locked ≠ 0** (unpinned row) beside locked == 0 (pinned) — or the
   both-readings rule is decoration. And borrow ≠ 0 separately.
6. **An absent Bybit coin with a baseline row** (absence-is-zero must produce
   a comparison against zero, not a skip) — cardinality: the balance report
   must also hold ≥2 rows so "first row" ≠ "the row for this asset".
7. **Chain mark mismatch** (hash differs at seq) AND **seq beyond head**
   (truncation) — two different refusals, each named. Plus the vacuous
   `seq 0` acceptance.
8. **Record-time refusals each on their OWN fixture**: a non-terminal spot
   order; an `Incomplete` sweep; an `Unattributed` order; an unpinned balance
   row — four distinct refusals, or the gauntlet collapses into "something
   refused".
9. **Two spot instruments sharing a base** (BTCUSDT + BTCUSDC) — the asset-
   granularity claim needs the fixture that would break instrument
   attribution.
10. **A spot instrument with no baseline row** → `UnassertedBaselineUnrecorded`
    holds the posture; a perp with no scope → `UnassertedScopeNotEnumerated`
    still holds — the two reasons must be distinguished by fixtures, not by
    both holding.
11. **Golden for the renderer** extension (a preservation claim needs a
    golden), including SpotUnexplained lines' ORDER against the existing
    sections.
12. **The record bracket** (round-1 finding 1, re-anchored by round 3): a
    cumulative that moves across the settle window refuses; a `spot_settle`
    of zero or negative refuses at config; the wait is shown to actually
    elapse on the injected clock (a fake clock that never advances would let
    "waited" and "did not wait" collapse); and a journal holding one
    `Planned`-only spot order beside one terminal filled one RECORDS — the
    Planned exclusion's own fixture.
13. **Priority per call site** (review finding 3, extended by round 3): the
    boot adapter and the record adapter constructed at different rungs,
    tested against distinct ceilings — the Safety call admitted where Routine
    is shed, the zero priority refused at construction — and the SAME pair
    asserted on the ORDER-READ surface (`Resolve` and the execution sweep),
    not only on balances, because the record protocol's join is where the
    volume is.
14. **Unpinned + latching in ONE boot** (review finding 4): the precedence
    fixture — `OutcomeNeedsOperator` wins over `OutcomeIncomplete`, shown by
    a boot carrying both.
15. **The virgin path** (review finding 5): a config with spot instruments
    and no `spot_baseline` boots to a HOLDING gate that names
    `UnassertedBaselineUnrecorded` — no placeholder file anywhere in the
    fixture.
16. **Document round-trip** (review finding 2): a written document reloaded
    must re-derive the same totals from the stored words via the shared
    per-venue function — a golden on the document bytes, and a word-mutation
    (free/locked swapped) that the loader's re-derivation must surface as a
    changed total.
17. **The virgin roster** (round-2 finding 1): configured spot instruments +
    EMPTY journal + no baseline → `UnassertedBaselineUnrecorded` for every
    one of them, and the gate holds — the fixture that fails on a
    journal-derived coverage set.
18. **The settle re-read, both surfaces** (round-2 finding 2): a cumulative
    that moves across the settle window refuses; a BALANCE that moves across
    it while every cumulative is stable refuses separately — two fixtures, or
    "re-read both" collapses into "re-read one".
19. **The credential swap** (round-2 finding 3): a document recorded under
    fingerprint A loaded by a transport holding credential B refuses by name,
    with venue, account label, chain mark, and digest all matching.
20. **The digest pin** (round-2 finding 4): document bytes replaced at the
    pinned path with a schema-valid alternative refuse against the config
    digest; the stored-rows recomputation refuses a row naming an order
    outside the chain-marked prefix — two different refusals, each named.
21. **`SpotUnconcluded` rendering** (round-2 minor): a renderer line per
    unpinned reason (locked, borrow), goldened, plus the combined
    latching-plus-unpinned boot's output — the renderer enumerates fields by
    hand, so a new field's absence is silent without its golden.
22. **The Binance call cost** (round-3 finding 1): an exact fixture pinning
    the account read's charge at 20 on the spot weight dimension, and a
    20→10 mutation shown killed — the governor charges the caller's word, so
    the number is a claim, not a constant.
23. **The second look, both visibility orders** (round-3 finding 2): a late
    fill whose wallet effect precedes its trade-list row — first Assert
    reports a delta, the re-JOIN admits the row, the re-Assert's delta set
    changes → `OutcomeIncomplete`, no latch; and the same delta STABLE across
    both looks → `OutcomeNeedsOperator`. Two fixtures, or "re-ran the join"
    and "re-read the balance" collapse.
24. **The quote-overlap deny** (round-3 finding 4): a roster holding BTCUSDT
    (BTC base) and ETHBTC (BTC quote) with a BTC baseline refuses at
    construction, naming the pair; the same roster WITHOUT the baseline
    constructs (the deny is about the baseline, not the pair).

Mutation battery (≥15, scoped seds, run after the checklist): the fee-term
drop, fee-scope widening (all orders incl. perp), fee-scope narrowing (own-
pair only), baseline-subtraction drop, sign flip on Unexplained, both-readings
arm swap (walletBalance+locked vs walletBalance when pinning), absence-skip
(continue instead of zero), chain-mark comparison drop, seq-beyond-head accept,
record-gauntlet arm drops (one per refusal), HoldsPosture flip on the new
member, roster row dimension/window mutations, Total=free (locked dropped) on
Binance.

### Campaign results (2026-08-05, session 105) — 20 runs, zero SURVIVED

Run after the item 1–24 audit filled the application-layer gaps the safe-point
summaries had overstated (assertSpot and the second look had NO tests until
`join_spot_test.go` / `reconcile_spot_test.go` landed — "5–6 partly in
join_test" was a claim the audit falsified).

| # | Mutant | Verdict |
|---|---|---|
| 1 | record: unattributed arm dropped (`>0`→`>99`) | caught |
| 2 | record: incomplete-sweep arm dropped | caught |
| 3 | record: open-order arm dropped (`!terminal`→`terminal && false`) | caught (first sed NOBUILD: orphaned `terminal` — re-aimed) |
| 4 | record: cumulative-moved arm dropped (`!= 0`→`== 99`) | caught (re-aimed after NOBUILD, same cause) |
| 5 | record: balance-moved arm dropped | caught |
| 6 | record: unpinned-balance arm dropped | caught (re-aimed after NOBUILD, same cause) |
| 7 | assertSpot: fee term dropped | caught (re-aimed after NOBUILD: orphaned `feeTotal`) |
| 8 | assertSpot: fee scope widened to derivative orders | caught — needed the perp-with-BTC-fee leg added to the main fixture first |
| 9 | assertSpot: fee scope narrowed to the order's own base | caught |
| 10 | assertSpot: baseline subtraction dropped | caught (12 tests) |
| 11 | assertSpot: Unexplained sign flipped | caught (7 tests) |
| 12 | assertSpot: absent asset skipped instead of compared as zero | caught |
| 13 | HoldsPosture: UnassertedBaselineUnrecorded exempted | caught (3 tests) |
| 14 | second look: `spotDeltasEqual` answers stable for every pair | caught |
| 15 | second look: the holding gate never brackets (`Holds()`→false) | caught (6 tests) |
| 16 | loader: chain-mark comparison dropped | caught (2 tests) |
| 17 | journal: seq-beyond-head answers a zero hash, not the named sentinel | caught (3 tests) |
| 18 | Binance Total = free (locked dropped) | caught (2 tests) |
| 19 | Binance account read charged at the misread 10 | caught |
| 20 | Bybit pinning arm swapped (`locked != 0` pins) | caught (6 tests) |
| 21 | wallet_balance roster row on the wrong dimension | caught (35 tests) |

Four NOBUILD!! verdicts total, every one "declared and not used" — a bad sed
orphaning the condition's only reader, none a compiler-enforced claim; each
re-aimed with a variable-preserving mutation and killed. The Binance
free/locked VALUE swap stays un-run by design: the sum commutes, so it is
behavior-equivalent (§70h.16 names the Bybit value-swap as the loud one, and
it is a standing fixture in baseline_test).
