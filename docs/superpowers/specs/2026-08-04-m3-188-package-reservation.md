# M3 task #188 — package-level budget reservation + the one-spender guard (§68)

> **WIP (session 99).** Design brief drafted; round 1 NO-SHIP (3B/1M/1m,
> all accepted: untimed occupancy, three-state hold entries,
> credential-bound spend lock); orchestrator interim fix (mandatory
> deferred Cancel — occupancy does not self-heal); round 2 NO-SHIP
> (1B/4M, all accepted, two with different fixes than proposed:
> capacity-check-at-consume over epochs, freeAt-ignores-occupancy over a
> Decision vocabulary change); round 3 NO-SHIP (2B/2M/1m, all accepted:
> signing-readiness construction refusal absorbed, overflow-safe
> arithmetic, composed capacity-refusal fixture, headline prose
> de-drifted, N+1 vector); round 4 NO-SHIP (1B/1M/2m, all accepted:
> single widened VenueOrders provider — the reserve/place split made
> inexpressible — hold-governor identity, the composed real-adapter
> test, prose de-drifted; the round verified the clock fix against
> wall.go — §68f). **Next: round 5 convergence check; on SHIP, write
> Appendix A and dispatch implementation.** Nothing in `internal/` has
> been touched.

**Task:** #188 — legs acquire rate-limit capacity independently after fan-out.
Filed during #185's design review (round 4 classified the concurrent-leg
capacity race as pre-existing and REBUTTED the blocker rather than absorbing
the fix; round 5 corrected the rebuttal's understated OMS consequence — the
refused leg is post-fsync and lands `StateSubmissionUnknown` — and assigned
this task the acceptance-test obligation; §66a). #187's close then assigned
this task a SECOND subject: the enforcement home for the one-spender
operational premise (§67a — "package-level reservation is the first slice
that must reason about concurrent spenders").

## The condition, precisely (all three code-verified this session)

1. **The race.** `Executor.Execute` (execute.go) preflights every leg with
   reads, then fans legs out one goroutine per leg; each leg's
   `SignedClient.Do` (signed.go step 2) calls `Governor.Acquire` on its own.
   With any shared window near its ceiling, one leg charges the last unit
   its priority admits while its sibling is refused. The shared windows are
   not exotic: the venue envelope is charged by EVERY leg by construction,
   and since #185 a Bybit spot+perp package charges the SAME `order_create`
   1s endpoint window twice — the two legs of the system's core act are that
   window's largest single consumer.

2. **The consequence.** The refused leg is refused pre-send but POST-FSYNC:
   `DispatchAuthorized` is already durable, `apperror.RateLimited` fixes
   `EffectNotApplied`, and `classifyEffect` maps a non-`ClassVenueReject`
   `EffectNotApplied` to `StateSubmissionUnknown` (dispatch.go). The outcome
   is a durable authorized order the §6.7 resolution machinery must resolve
   — a Safety-priority query spent on an ambiguity we manufactured — while
   the sibling may fill: a naked leg whose unhedged window §6 step 4's
   sizing explicitly budgets for. §6's one-leg-failure flow is the designed
   consumer, so this is not a correctness hole; it is a self-inflicted
   resolution episode at the margin.

3. **The overclaim.** Execute's own contract states the rule this slice
   completes: "every check that can refuse ANY leg runs before ANY leg's
   authorization fsync." The governor's capacity check is a check that can
   refuse a leg, and today it runs after — inside transport, per leg,
   concurrently. Capacity was believed to be the ONE refusable
   precondition the preflight cannot see — review round 3 falsified the
   "one": a `clock_uncertainty` configured at or past the signing
   acceptance bound (`min(recv_window, aheadTolerance)`) passes config
   validation (non-negative is all it checks), passes Dispatch's anchor
   check (populated is all IT checks), and then `checkServerTime` refuses
   EVERY send at signing — post-fsync, deterministically, on every order.
   Same genus, different organ: a boot-detectable configuration that
   manufactures resolution episodes. This slice closes both — the race by
   reservation, the config by a construction-time refusal (§68b.5a).

## §68a — the decision: reserve before fan-out, and check the premise at boot

**Take the reservation.** A package's rate-limit charges are validated
and PROVISIONALLY OCCUPIED atomically, as one governor operation, before
any leg's fsync; each leg's transport CONSUMES its occupancy — becoming a
timed charge only then — instead of acquiring. A package that cannot
reserve is refused whole, with nothing journaled and nothing sent — the
same contract shape as every other preflight refusal. (Round 3 caught this
paragraph still saying "charged as one operation before the fsync" — the
round-1 model this record rejects; a literal implementation of that
sentence recreates both round-1 blockers, so the sentence itself had to
move.)

**The weighing, honestly.** Against: Opening ceilings sit at ~70% of
capacity precisely so routine operation never runs at the margin where this
race lives, and §6's one-leg-failure flow already consumes the refusal
correctly. For, and it prevails: (a) the executor's contract already states
the fsync-side rule, and this slice closes both known violations of it —
the capacity race by reservation, the signing-readiness configuration by
construction refusal — making the prose true rather than adding a new
promise; (b) #185 moved
the margin: 1s endpoint windows refill fast but are SMALL (create: min(10,
20) = 10/s shared across categories), and both legs charge the same one, so
the collision no longer requires an independent co-spender — the package
collides with itself plus any single concurrent Safety read burst; (c) the
cost of losing the race is not an error return but a durable
`SubmissionUnknown` — a resolution episode, a Safety-priority query, and an
unhedged window — which is the most expensive spelling of "retry later" this
system has; (d) §45's cross-venue future has NO shared window between legs,
so no governor can ever see the sibling's refusal coming — preflight
reservation is the only shape that generalizes, and the seam should exist
before the second venue arrives with it.

**Take the one-spender guard, host-scoped.** §67 recorded the premise —
exactly one consumer spends a venue credential at a time — as operational,
with this task as its enforcement home. The deliverable is a cross-process
exclusive lock per credential, acquired at composition time, refused at
`NewSignedClient` when absent: two processes spending one credential on one
host becomes a boot refusal naming the lock, not a silent split-brain
self-charge. The limit is stated rather than papered over: flock is
HOST-scoped, so a credential shared across hosts remains covered only by the
operational premise and §67's revisit trigger, both unchanged. The realistic
accident — a cron overlap, an operator double-run, an exercise run beside a
reconcile run — is same-host, and that class becomes inexpressible.

## §68b — the mechanism

### 1. `platform/ratelimit`: `Reserve`, `Hold`, `Consume`, `Cancel` — and
### why a reservation is OCCUPANCY, not a timed charge

**The model, corrected by review round 1 (two blockers, one root cause):**
the first draft stamped a reservation as an ordinary window charge at
reserve time. That fails twice. A charge is TIMED: on a 1s Bybit endpoint
window it ages out under `window.evict` if anything — a slow fsync, a
SIGSTOP, scheduler pressure — delays the leg past the interval, and the
leg then sends against a window holding no live charge (the venue counts
at SEND time, so a reserve-time stamp also frees too early by exactly the
reserve→send latency, on every reservation). And it poisons the fold:
`observe` ratchets against charge-derived `used`, so a venue report of 10
against 6 reserved-but-unsent units records only the 4-unit excess — which
a later `Cancel` of the 6 turns into `used=4` against a venue that said 10,
and a report BELOW the reserved count is discarded entirely and then
cancelled down to zero. The venue's report can never include our unsent
reservations, so folding them into one number was a category error.

So a reservation is **provisional occupancy, held beside the charges**:
`window` gains `reserved uint64` — untimed, never evicted, invisible to
`observe` (which continues to ratchet against charge-derived `used` only,
now stated as a contract rather than an accident), counted by admission.
`admits` becomes "units ≤ ceiling − used − reserved", written in the
non-underflowing direction (§66 round 6's lesson stands: `used > ceiling`
is a state the ratchet produces, and now `used + reserved` can exceed any
ceiling). `freeAt` IGNORES occupancy — round 2's correction: `NotBefore`
is documented as "the earliest instant a retry COULD succeed" (decision.go
reserves the zero for "not by waiting", and `worseThan` ranks zero as
never), and occupancy can vanish at any moment via Cancel, so the earliest
instant a retry could succeed is when the CHARGES alone would admit.
Computing it over `ceiling − reserved` would either overload the zero into
a false "never" or invent a release instant occupancy does not have; the
charges-only answer is the existing contract's own semantics — a hint that
may be early, never a promise — and it keeps `worseThan`'s attribution
pointed at the window whose release time is real.

`Governor.Reserve(reqs []Request) (*Hold, Decision, error)` — all-or-nothing
under the governor's one lock:

- **Validation is decide's, per request, unchanged in taxonomy.** Every
  request passes the same gates as `decide`: `Validate`, family known,
  R1/R2/R3 endpoint rules, the cannot-count refusal. Any failure is an
  ERROR for the whole batch (a profile-versus-adapter disagreement no
  waiting repairs), and nothing is occupied.
- **Mixed priorities are refused as an error.** A package is one §5.3
  command; its legs share one urgency. A batch naming two priorities is a
  caller bug, and admitting it would make "which ceiling?" a per-window
  accident.
- **Admission is checked on the SUM, and the sum is CHECKED arithmetic.**
  For each window applicable to any request, the total units the whole
  batch would occupy must fit under `ceiling − used − reserved` at the
  batch's priority — comparisons in the non-wrapping form, and the batch
  accumulation itself overflow-checked: a total that does not fit in
  uint64 is an ERROR refusing the batch, never a saturation (round 3: a
  saturated total reads as exactly-at-capacity while the entry totals it
  summarizes disagree with `reserved` — a number that lies twice).
  This is the entire point: two legs that each fit alone but not together
  are refused together, with nothing occupied — and a second package's
  reservation stacks against the first's.
- **A pause refuses the batch** (`ReasonPaused`), as it refuses everything.
- **Refusal names the worst limit** across all refusing windows, by the
  existing `worseThan` rule — the `Decision` is the answer, `Hold` is nil.
- **Admission raises `reserved` on every applicable window** and returns a
  `*Hold` carrying one entry per request.

**Every hold entry is a three-state machine, and the states are the
concurrency answer:** `reserved → consumed | canceled`, transitions only
under the governor's lock, no transition out of the two terminals.
`Consume` and `Cancel` racing is therefore a serialization the lock
decides, and BOTH orders are specified: consume-then-cancel retains the
consumed leg's charge and releases nothing of it; cancel-then-consume
fails the consume closed (nothing sent, `EffectNotApplied`).

`Hold.Consume(r Request) (Decision, error)` — called by transport instead
of `Acquire`:

- Matches `r` by STRUCT EQUALITY against the hold's entries in state
  `reserved` (`Request` is comparable: Family, Endpoint, Priority, Cost).
  No match — wrong request, consumed already, canceled — is an ERROR: fail
  closed, nothing sent, `EffectNotApplied`. Two identical entries are
  interchangeable occupancies; first-reserved-match is sound.
- **Re-checks the venue's word, and ONLY the venue's word — the pause, and
  capacity on the folded count.** A penalty in force when Consume runs
  means the venue has declined; issuing would lengthen the ban. And a
  charge that can no longer fit under CAPACITY on any applicable window
  (charge-derived `used`; written `used > cap || units > cap − used`,
  NEVER `used + units > cap` — the fold accepts the full uint64 range, so
  the sum wraps, and window.go's own `admits` already refuses this exact
  expression for this exact reason) is refused with `ReasonCeiling` —
  round 2's blocker: an upward fold between reserve and consume can
  ratchet `used` to the venue's own report, and §7.6 forbids issuing past
  capacity for EVERY class, reservation or none. The check is
  against capacity, not the priority ceiling, and that scoping is the
  argument: local spending alone can never push `used + reserved` past
  capacity (Reserve and Acquire both bound their admission by it, counting
  occupancy), so a consume-time capacity violation is PROVABLY
  fold-initiated — the venue's word, the same class as the pause — and the
  reservation's own guarantee ("no interleaved local spender refuses a
  reserved leg") survives intact. No other re-admission runs.
  **Consume is the admission linearization point, and the claim stops
  there:** a penalty or fold that lands after Consume returns and before
  the socket write is not caught, exactly as one landing after today's
  `Acquire` is not — the reservation narrows nothing about that window and
  widens nothing either.
- On admission: atomically converts the occupancy into an ordinary charge
  stamped at CONSUME time (`reserved -= units; charge(now, units)` per
  applicable window, one lock hold). The charge's window starts when the
  venue's does — at the send — so it ages out `interval` after the call,
  not `interval` after the reservation.

`Hold.Cancel() error` — for every entry still in state `reserved`: mark
`canceled`, then release its occupancy (`reserved -= units` per window),
one lock hold, so no Consume can interleave between the mark and the
release. Consumed entries are untouched — the venue counted those calls.
Repeated Cancel is an idempotent no-op.

**Cancel is MANDATORY at package end, and the state machine is what makes
that safe.** The occupancy model changed the failure of forgetting: a
charge aged out on its own, but occupancy never does, so a hold abandoned
with entries still `reserved` — a leg whose Dispatch refused pre-Place
after fan-out, a panic, any early return — would pin capacity for the
process's life. The executor therefore cancels every distinct hold on EVERY
exit from Execute (deferred at reserve time, before the first fsync is
possible), and that blanket cancel is provably harmless to sent legs:
Cancel touches only entries still in state `reserved`, and a sent leg's
entry is `consumed` — the release-a-counted-charge hazard that §68d item 3
rejects is not expressible through this API. What §68d item 3 continues to
reject is releasing CONSUMED charges early; releasing never-sent occupancy
is not a throughput optimization but the model's obligation.

**Observability:** `LimitState` gains `Reserved uint64` beside `Used`, and
the roots' existing rate-limit status line renders it only when non-zero —
the consumer arrives with the field. A snapshot taken mid-package shows
capacity genuinely spoken for; a snapshot at rest shows zero.

### 2. `engine/venue`: the venue owns the batch, and the shared-governor
### prose becomes a checked fact

`OrderClient.ReservePlace(reqs []OrderRequest) (*ratelimit.Hold, ratelimit.Decision, error)`
builds each leg's call through the venue codec exactly as `Place` does
(same encode path, same cost, same registry-derived endpoint), prices it
into a `ratelimit.Request` via the leg's family transport, and calls
`Governor.Reserve` ONCE with the batch.

That requires the venue-level governor to be nameable, and naming it
converts a normative comment into a construction refusal:
`OrderClientConfig` gains `Governor *ratelimit.Governor`, required, and
construction refuses any family transport not built on exactly that
governor (`SignedClient` answers identity, it does not expose the value).
"On Binance the two clients MUST share one *ratelimit.Governor*" has been
prose on a config field since slice C2a; after this slice a root that
wires two governors under one venue cannot boot — the #186 move, applied
to the claim this slice's correctness rests on.

`SignedClient` gains the pricing seam (`price(call) (ratelimit.Request,
error)`, the exact request `Do` builds today, factored so reserve and send
cannot drift) and `transport.Call` gains `Hold *ratelimit.Hold`:

- `Do` with `Call.Hold == nil`: `Acquire`, exactly as today. This is the
  single-order dispatch path (§6.7 resolution, a future #184 caller) and
  every read — the class for which acquire-at-send is CORRECT, so the
  degraded mode of a forgotten hold is the shipped, safe behavior.
- `Do` with `Call.Hold != nil`: `Hold.Consume(priced request)`. A consume
  refusal is `apperror.RateLimited` (pause) or `apperror.Precondition`
  (mismatch), both `EffectNotApplied`, both pre-send.
- **No double charge**, and the evidence plan pins it with an exact count:
  after a reserved Place, `Used` equals the reserve-time total, not the
  total plus an acquire.

### 3. The threading, on Urgency's exact precedent

The hold travels the path the shed class already walked (v0.9.32):
`Executor` → `Dispatcher.Dispatch(id, expected, urgency, hold)` →
`ports.OrderPlacer.Place(o, u, hold)` → `venue.OrderRequest.Hold` →
`transport.Call.Hold` → `Do`. Nil at every seam means "acquire at send" —
the single-order path's mode, documented at each seam. `[SPEC EDIT OWED]`
§4's Place signature and §6 step 6 both name the new argument at close.

### 4. `ports`: the reserver is its own segregated capability

`ports.CapacityReserver`:
`ReservePlacements(orders []domain.Order, u Urgency) ([]*ratelimit.Hold, error)`
— one hold PER LEG, index-aligned with the input. Today every leg is one
venue, so every element is the same pointer; the shape is per-leg because
§45's cross-venue package returns per-venue holds and the port must not
need re-deriving then. The driven adapter translates each leg through the
existing `toVenueOrder` and calls `OrderClient.ReservePlace`; a refusal
maps to an error carrying the decision's venue, limit and `NotBefore`
(nothing sent, so an error is honest — Dispatch's own contract shape).

**The wiring is named, not left to be invented (round 2), and the two
capabilities are ONE construction (round 4's blocker):** `Executor` gains
a `reserver ports.CapacityReserver` field and `NewExecutor` gains it as a
REQUIRED argument (nil refused, the constructor's standing pattern) — the
dispatcher cannot supply it, because a Dispatcher deliberately exposes
only the Place capability. But the root does NOT get a second carrier:
round 4 showed that an independent `Deps.VenueOrderReserves` field lets a
root wire reservation to client A and placement to client B — two
internally-valid clients, two governors, and a hold that either consumes
against the governor the send never charges or mismatches post-fsync. So
`Deps.VenueOrders`' interface WIDENS to carry both `Place` and
`ReservePlace` (they are one venue surface — the same `*venue.OrderClient`
has always been the value), ONE driven gateway is constructed from it, and
`oms.Module` narrows that single gateway into the two ports the two use
cases hold. Reserve-here-place-there is inexpressible, not checked — the
NewExecutor/§59b move, applied to the capability pair whose split this
slice creates. (The VenueOrderReads/VenuePositions precedent of
one-field-per-capability is deliberately NOT followed here, and the
difference is the reason it exists there: those splits STOP a use case
from doing more than its capability; this split would let the root wire
one act's two halves to two different venues' accounting.)

**Defense in depth on the hold itself:** a `Hold` knows the governor that
minted it, and `Do` refuses (Precondition, pre-send) a hold whose governor
is not this client's own — so even a future wiring mistake that
resurrects the split fails closed at the last seam that can still refuse
with nothing sent, instead of consuming capacity the send never charges.

`preflight` returns the records it already loaded, so reservation prices
the orders the preflight admitted, not a second read.

The `Executor` consumes it between `preflight` and dispatcher
construction: reserve, DEFER the cancel of every distinct hold (the
mandatory package-end release — §68b.1), then VERIFY the vector — exactly
one non-nil hold per leg, length equal to the refs', refused otherwise
before any dispatcher exists — then fan out with `holds[i]` threaded to
leg i. The verification is round 2's finding: nil means "acquire at send",
so a short or nil-bearing vector from a buggy adapter would silently
revert a leg to the post-fsync race this slice removes, while its
occupancy sat reserved beside it. A failure between the reserve and the
fan-out refuses the package whole with the deferred cancel already armed.
`Dispatcher` remains usable without a reservation (nil hold) — the
single-order path.

### 5a. Signing readiness is a construction refusal (round 3's blocker)

`NewSignedClient` gains one check: it takes a construction-time
`Clock.ServerNow(Signer.Venue())` reading and applies the SAME acceptance
rule signing applies (`checkServerTime` itself, not a restatement — two
spellings of one rule are the copy that drifts). A configuration whose
uncertainty band cannot clear `min(RecvWindow, aheadTolerance)` refuses to
construct, naming the band and the bound — instead of authorizing orders
all day and refusing every one of them at signing, post-fsync,
deterministically. The check is COMPLETE, not merely early, because the
calibration is per-process constant: `Config.Calibrations` feeds the one
`Wall` built before `Run`, and its own doc states the calibration "cannot
be added later" — a band that passes at construction is the band every
later signing sees. (Implementation must re-verify that constancy against
wall.go before relying on it; if a future clock implementation re-bands at
runtime, this check degrades to early-warning and signing's own refusal
remains the backstop — which is today's shipped behavior, not a new
hazard.) The uncalibrated venue is deliberately NOT this check's subject:
a zero calibration claims a perfect clock and passes, exactly as it
reaches signing today, and Dispatch's anchor validation already refuses
the truly-absent frame pre-fsync.

### 5b. The one-spender guard: `transport.SpendLock`, BOUND to the
### credential it protects

Review round 1's third blocker: a lock whose fingerprint the CALLER
supplies proves only that some file got locked — two roots signing with
one key but fingerprinting differently (or locking in different
directories) both boot. The fingerprint must therefore be DERIVED, on both
sides of the comparison, from the credential itself:

- `transport.Credential` gains `Fingerprint() string` — hex SHA-256 of the
  API key. The key never touches the filesystem; the fingerprint is not
  secret (the key is an identifier the venue sees on every request; its
  hash identifies without revealing).
- `transport.AcquireSpendLock(dir string, cred Credential) (*SpendLock, error)`
  — an exclusive, non-blocking flock on `<dir>/<fingerprint>.lock`,
  fingerprint computed INSIDE from `cred`, stored on the returned token.
  Held for the process's life; `Close` releases.
- `Signer` gains `Fingerprint() string` (each signer already holds its
  `Credential`; both implementations delegate to it).
- `SignedClientConfig` gains `Spend *SpendLock`, REQUIRED — and
  `NewSignedClient` refuses a token whose fingerprint differs from the
  SIGNER's. A present-but-wrong lock (minted for another configured
  credential, or for a mistyped one) is the wiring mistake the comparison
  exists for; "non-nil" alone proves nothing, and proving nothing while
  looking like a guard is worse than absence.

One root acquires once per credential and shares the token across that
credential's clients (Binance's two per-family transports, the order and
read clients): the lock is per-credential, not per-client, and a second
acquire of the same fingerprint in ONE process is refused too — flock
locks belong to the open file description, so a second open-and-flock
conflicts even in-process (verified on darwin this session; linux
documents the same). Non-flock platforms (build-tagged, the
storage_other.go precedent) refuse to construct the lock: no platform gets
a silent no-op guard.

**The namespace has a carrier and an owner (round 2):** the lock directory
is a REQUIRED deployment config field (`spend_lock_dir` in each
credentialed root's config schema — reconcile's today, any future
credentialed root's by the same rule), validated at load like the journal
path. The ROOT owns the token: wire.go acquires once per credential after
resolving it, hands the token to every client on that credential, releases
it on shutdown and on every construction-failure path out of wiring — the
same lifetime discipline the journal handle gets. The docs-owed inventory
at close includes the config schema and its example.

**Residual, stated:** two roots configured with DIFFERENT lock directories
on one host split the namespace and both boot — a config error the code
cannot see (the same class as pointing two roots at different journals),
recorded rather than papered over. Cross-host sharing likewise stays on
§67's operational premise and revisit trigger, both unchanged.

## §68c — what this slice does NOT change

- **Acquire's own semantics.** Non-package traffic acquires at send,
  unchanged. The reservation is additive.
- **The refusal taxonomy.** Reserve reuses decide's gates; no new refusal
  class exists for a request; the one new error class is the consume
  mismatch, which is a wiring defect by construction.
- **§67's count-and-alert.** Fold misses, bans, the derive-nothing ruling:
  untouched. The premise §67 recorded becomes checked at one scope
  (host) and stays operational at the other (cross-host), with the revisit
  trigger unchanged.
- **The pause's supremacy.** A venue penalty still refuses reserved legs
  at consume time. The reservation binds our own accounting, never the
  venue's word.

## §68d — rejected alternatives (recorded so nobody reopens them)

1. **`Governor.Check` as the package preflight.** Expressly advisory
   (governor.go says so in those words); invalidated between Check and
   Acquire by any interleaved spender, which is a TOCTOU that narrows the
   race without closing it — and a narrowed race is worse than an open
   one, because the surviving failure is rarer, later, and wearing a test
   suite that passes. Filed against in the task text itself.
2. **Internal keyed matching instead of threading the hold** (governor
   holds reservations keyed by request shape; Do's Acquire consults the
   table first and falls back). Rejected for the fallback: a reservation
   that fails to match SILENTLY degrades to today's behavior, so the
   mechanism can go vacuous with every test passing — the clean-run/
   vacuous-run collapse as an API design. The explicit token fails closed
   at the exact seam the wiring broke.
3. **Releasing CONSUMED charges on any evidence short of the venue's.**
   The first draft rejected all post-fan-out release while reservations
   were timed charges that self-healed by eviction; the occupancy model
   inverted half of that — never-sent occupancy MUST be released (§68b's
   mandatory deferred Cancel), because it no longer ages out. What stays
   rejected is the other half: a consumed entry's charge is a call the
   venue counted, and no local fact — a transport error, a timeout, an
   abandoned outcome — is evidence the venue uncounted it. The state
   machine encodes the split: Cancel cannot reach a consumed entry, so the
   ban-direction release is inexpressible rather than reviewed-for.
4. **The multi-venue prepare/commit coordinator, now.** No consumer: no
   composition wires two venues into one Executor today (module.go takes
   one venue's client), and §45's gate decides whether one ever does. The
   port shape (per-leg holds) and `Cancel` (the rollback verb) are the
   seam; the coordinator lands with its consumer — the
   schema-frozen-before-its-consumer lesson, applied prospectively.
   Deterministic reserve order across venues (by venue name) is recorded
   here for that slice.
5. **A reservation TTL.** Under the occupancy model there is nothing to
   expire: occupancy is untimed by design (round 1's blocker is exactly
   what a timed reservation costs), the charge's clock starts at consume,
   and an abandoned hold is released by the executor's mandatory deferred
   Cancel rather than by a deadline. A TTL would add a clock read to the
   dispatch path — dispatch.go confines clock access to the authorization
   anchor "with the least room" for a second reader — to reintroduce, on
   a timer, the eviction the model exists to remove.
6. **Per-family governors to shrink the shared-window race.** Violates
   §7.6's venue envelope directly ("two governors would be two IP
   envelopes for one IP").
7. **Epoch-based hold invalidation** (round 2's proposed fix for the
   fold-past-capacity blocker: stamp windows with a correction epoch at
   reserve; any upward fold before consume fails the hold closed).
   Rejected as over-broad: it refuses packages on folds that still FIT —
   a reading one unit above the local count would kill a package the
   venue has ample room for, manufacturing exactly the post-fsync
   resolution episode this slice exists to remove, on a signal that
   carries no refusal. The capacity check at consume draws the line where
   §7.6 draws it: the venue's word binds when it says "you are past
   capacity", not when it says anything at all.
8. **Extending `Decision` with a reservation-contention outcome** (round
   2's proposed fix for the zero-`NotBefore` overload). Rejected: a new
   `Reason` member for a transient internal state leaks the mechanism
   into every exhaustive switch over the enum, and the contract already
   has the right semantics — `NotBefore` is "the earliest instant a retry
   COULD succeed", which for a window blocked by cancellable occupancy is
   the charges-only answer. The fix is in `freeAt`'s arithmetic, not in
   the vocabulary.
9. **Scoping the spend lock to Bybit.** The premise was recorded on
   Bybit's UID because #187's fold is Bybit's, but the split-brain
   self-charge it guards against is venue-neutral (two processes each
   holding a complete-looking account of one credential's consumption
   describes Binance identically). An exemption argued from one cause
   releases the other (§the-exemption lesson); the lock is per credential,
   every venue.

## §68e — evidence plan (run the collapse checklist BEFORE mutating)

**Collapse checklist, named up front:**

- The sum-over-shared-window fixture must make "each leg fits alone" and
  "the batch fits together" DISAGREE (ceiling ≥ each leg, < sum). A
  ceiling admitting both is the pinned value that collapses the batch
  check into per-leg Acquire.
- The all-or-nothing fixture must have the refusing window be the SECOND
  (or later) window/leg evaluated, so "charged then rolled back" and
  "never charged" are distinguishable from "first check refused" — the
  ORDER dimension.
- The consume-match fixture needs ≥2 entries with DIFFERENT requests, so
  match-by-equality and match-first are distinguishable — cardinality.
- The no-double-charge assertion is an EXACT `Used` count (a known count
  is an instrument), never `>=` or `!= 0` — the negative-assertion lesson.
- The hold-threading mutants at call sites (holds[i] → nil) are per-site
  argument mutations — the #187 lesson; note that holds[i] → holds[0] is
  expected-equivalent TODAY (single venue, same pointer) and must be
  recorded as such, not claimed as a kill.
- The Cancel test asserts snapshot EQUALITY with the pre-reserve golden,
  not "used decreased".

**Unit (ratelimit):** batch admit/refuse on the sum; all-or-nothing
rollback leaves every window byte-identical in snapshot; mixed-priority
error; pause-at-reserve; pause-at-consume (penalty between reserve and
consume refuses with `ReasonPaused`); consume mismatch error; double
consume error; two-identical-entries consume twice then refuse the third;
cancel releases reserved only; validation-gate parity with decide
(table over R1/R2/R3/cannot-count/family/mixed-priority for Reserve).

**Unit (ratelimit) — the round-1 compositions, each a former blocker:**

- **Occupancy survives the window.** Reserve on a 1s window, advance the
  fake clock PAST the interval, then: (a) an interleaved Acquire that
  would fit absent the reservation is still refused — the occupancy did
  not age out; (b) Consume still succeeds and the resulting charge ages
  out at `consume + interval`, not `reserve + interval` (advance again and
  assert the exact eviction boundary — a known instant is an instrument).
- **Observe is blind to occupancy, in both directions.** Reserve 6, fold a
  venue report of 10: `Used` ratchets to 10, `Reserved` stays 6, and a
  subsequent Cancel leaves `Used == 10` exactly. The below-report case
  needs a pre-existing charge to be non-vacuous (with `used == 0` any
  report folds in): charge 5, reserve 6, fold a report of 4 — discarded,
  and Cancel leaves the charge of 5 intact. Partial-consume variant:
  consume one of two legs, fold, cancel the rest — the consumed charge and
  the fold survive, the occupancy alone is released.
- **The entry state machine, both serialized orders.** Consume-then-cancel:
  the consumed leg's charge survives Cancel. Cancel-then-consume: Consume
  fails closed with nothing charged. Double-cancel: idempotent, snapshot
  unchanged. Plus a barrier-controlled race (two goroutines, one Consume
  one Cancel, run under `-race`): every interleaving lands in one of the
  two specified outcomes, never a third.
- **admits/freeAt in the reserved-heavy corner:** `used + reserved` past
  the ceiling (ratchet + occupancy) neither underflows nor admits — the
  §66-round-6 non-underflow direction, re-checked on the new arithmetic,
  with the fixture ON the boundary (`used + reserved == ceiling`, `== ±1`)
  rather than strictly inside it.
- **The consume-time capacity check, on its boundary AND its wrap
  corner.** Reserve, fold an upward reading, consume: `used + units ==
  Capacity` ADMITS (§7.6 forbids "past", not "at"); `== Capacity + 1`
  REFUSES with `ReasonCeiling` and nothing charged, the entry still
  reserved (the deferred Cancel releases it). The wrap fixtures beside
  them (round 3): fold `MaxUint64`, consume a 1-unit entry — refused, not
  wrapped-to-zero-and-admitted; and a Reserve batch whose cost sum
  overflows uint64 against a `Capacity == MaxUint64` profile — an error,
  nothing occupied. And the discrimination fixture: a fold that raises
  `used` but leaves the charge fitting under capacity does NOT refuse —
  the check reads capacity, not "any fold happened", and this fixture is
  what tells those two rules apart.
- **freeAt ignores occupancy.** A window blocked by occupancy alone
  reports the charges-only `NotBefore` (with a live charge in the fixture
  so the answer is a real future instant, not the degenerate now); the
  same window's `admits` still refuses — the pair in one test, because
  the split IS the design.

**Transport:** Do-with-hold consumes and does NOT acquire (exact Used);
Do-with-nil-hold acquires (existing suite is the positive control);
consume refusal maps to `EffectNotApplied` both ways (pause → RateLimited,
mismatch → Precondition); price/send cannot drift (one seam, asserted by
construction — the factored `price` is what `Do` itself calls). And the
COMPOSED capacity-refusal test (round 3: unit coverage of Hold cannot
prove Do obeys its answer — an implementation handling only ReasonPaused
and sending on ReasonCeiling passes every unit fixture): reserve, fold
past capacity, call Do — zero HTTP requests issued (counted RoundTripper),
`ClassRateLimit` + `EffectNotApplied` returned, the entry still reserved
until Cancel, `Used` exactly unchanged. The signing-readiness refusal at
NewSignedClient: bands immediately below, exactly at, and above
`min(RecvWindow, aheadTolerance)` — below constructs (positive control),
at/above refuse naming both quantities.

**Executor:** reserve refusal → error, zero journal appends, zero Place
calls (fake journal + fake placer counting); reserve runs AFTER preflight
(a preflight-refused package never reserves — order asserted); holds
threaded per leg; the hold-vector verification refuses a SHORT
vector, a nil-bearing vector, AND an N+1 vector (round 3: equality
weakened to `<` survives the first two — the long-vector fixture is what
makes "exact" mean exact), each with the deferred cancel observed on
every distinct returned hold — occupancy back to zero, nothing journaled; the deferred cancel on EVERY exit — the
load-bearing case being a mixed package (one leg consumed, one refused
pre-Place by its own admit): after Execute returns, `Reserved` is exactly
zero and the consumed leg's charge is exactly intact — occupancy neither
leaks nor takes the sent leg's charge with it.

**Acceptance (the task-owed test), claimed at its true width:** a
governor-refused leg REACHES resolution, in BOTH surviving refusal
shapes: a penalty landing before one leg's consume (the pause), and — the
round-3 addition — an upward fold past capacity landing before it (the
capacity check), each venue-initiated, each producing
`EffectNotApplied`/non-reject →
`StateSubmissionUnknown` durable in the journal → the test invokes the
REAL Resolver, which resolves the order to its correct terminal, and the
Execution reports the mixed one-leg result. The classifier, journal,
executor and resolver are the shipped components; the venue is faked; and
the test ITSELF stands where §6's one-leg-failure consumer will — round
2's correction: module.go places that consumer outside the OMS context and
no shipped composition invokes it, so a claim of "the designed consumer
engaged" would be a citation to a component that does not exist. What the
test proves is that every artifact that consumer needs — the durable
state, the resolution, the per-leg report — is produced and consumable;
its invocation seam is the consumer's own future slice.

**Composed (round 4's major — the seams above, tested TOGETHER):** a
two-leg package through the REAL driven adapter, `OrderClient`, codecs,
transports (counted RoundTrippers) and one Governor: distinct priced
requests per leg, both reserved, both consumed, per-window counts EXACT
(envelope +2, each family's windows +its own, the Bybit endpoint window
+2), zero residual occupancy after Execute. The mutation campaign carries
the skip-last-leg mutant in the driven adapter (`len(orders)-1`): the
returned vector still passes the executor's shape checks, so only this
composed test — the omitted leg's consume failing against the real hold —
can kill it. Plus the hold-governor identity refusal: a hold minted by
governor A presented to a client on governor B refuses pre-send with
nothing consumed on either.

**Boot/wiring:** OrderClient refuses a family transport on a different
governor (and the positive control: same governor boots); NewSignedClient
refuses a nil SpendLock AND a mismatched one (a lock minted for a
different credential — the round-1 binding blocker's fixture) with the
positive control beside it; AcquireSpendLock refuses a held fingerprint
(second fd in-process — flock semantics make this testable without a
second process) and the positive control releases and reacquires;
fingerprint is key-derived and the key never appears in the lock dir
(assert file name and content).

**Mutation campaign:** per Appendix (to be written at convergence), with
the §67 additions standing: per-call-site argument mutants, exact-count
assertions, `MUTATE_MODULE` unset (all targets in the root module),
`MUTATE_TIMEOUT` 180s where the exercise suite is in scope.

**Docs owed at close:** build-spec §7.6 gains the package-reservation
clause + the spend-exclusivity sentence scoping §67's premise paragraph
(v0.9.47); §6 step 6 names the reserve-before-fan-out step and the
widened Dispatch/Place arguments (the §4 signature edit); §5.2/§7.6's
transport prose gains the signing-readiness construction refusal (round
3's absorbed blocker — a sentence, beside the recvWindow discussion it
completes); the credentialed roots' config schema gains `spend_lock_dir`
(required) with its example updated; venue-profiles untouched (no new
venue fact); adr-history row; tutorial parity ONLY if tutorial sections
touch §7.6's rate-limit teaching (check at close; ka is generated, never
hand-edited).

## §68f — the review arc

- **Round 1 (NO-SHIP, second-model design review):** BLOCKER — a
  reservation stamped as a timed charge can EXPIRE before Consume (a slow
  fsync or a stall past a 1s window evicts it; the leg then sends against
  no live charge, and the stamp also frees too early by the reserve→send
  latency on every reservation) → ACCEPTED; the reservation became untimed
  provisional occupancy (`window.reserved`), converted to a charge stamped
  at consume time. BLOCKER — `Reserve → Observe → Cancel` composes into
  erased venue readings (the fold ratchets against a used that includes
  unsent reservations, so cancelling subtracts consumption the venue
  reported) → ACCEPTED; same redesign, plus the contract that `observe` is
  blind to occupancy, plus the composition fixtures in §68e. BLOCKER — a
  caller-supplied fingerprint proves a file got locked, not that THE
  credential is exclusively held → ACCEPTED; fingerprint derived from
  `transport.Credential` on both sides and compared at `NewSignedClient`,
  lock-directory residual recorded. MAJOR — Cancel had no terminal-state
  semantics (a literal implementation could consume after cancel and send
  uncharged) → ACCEPTED; the entry is a three-state machine with both
  serialized race orders specified and tested. MINOR — the pause guarantee
  claimed "between reserve and send"; the enforceable claim is "in force
  when Consume runs" → ACCEPTED, wording fixed, Consume named the
  admission linearization point. The round also independently confirmed
  the flock same-process-second-fd premise on darwin and found no layering
  violation in the ports → platform/ratelimit edge.

- **Between rounds (orchestrator, self-found while briefing round 2):**
  the occupancy redesign changed the failure of an ABANDONED hold — a
  timed charge self-healed by eviction, occupancy never does — so §68d's
  round-1 rejection of release-at-fan-in had its premise inverted for the
  never-sent half. Fixed before round 2 ran: Cancel is mandatory at
  package end (deferred at reserve), and the state machine makes the
  blanket cancel provably unable to touch a sent leg's charge; §68d item
  3 now rejects only the release of CONSUMED charges.
- **Round 2 (NO-SHIP, second-model design review):** BLOCKER — an upward
  fold between reserve and consume can ratchet `used` past capacity and
  Consume (pause-only) would issue past the venue's own reported
  capacity, violating §7.6's absolute-capacity rule → ACCEPTED, with a
  different fix than proposed: Consume refuses when the charge cannot fit
  under CAPACITY (provably fold-initiated, since local accounting bounds
  `used + reserved` by capacity); the proposed correction-epoch
  invalidation is §68d item 7, rejected as refusing folds that still fit.
  MAJOR — the zero-`NotBefore`-for-occupancy rule overloaded the
  documented "not by waiting" meaning and poisoned `worseThan`
  attribution → ACCEPTED, fixed in `freeAt` (occupancy ignored; the
  "earliest instant a retry COULD succeed" contract already covers a
  cancellable block); the proposed `Decision` vocabulary extension is
  §68d item 8. MAJOR — the Executor's path to the reserver was
  unspecified and the hold vector unverified (a short/nil vector silently
  reverts a leg to the post-fsync race) → ACCEPTED: required `NewExecutor`
  argument, `Deps.VenueOrderReserves` on the segregated-capability
  precedent, preflight returns its records, exact-length/non-nil
  verification before any fsync. MAJOR — the acceptance test claimed "the
  designed §6 consumer engaged" but module.go places that consumer
  outside the OMS and nothing ships it → ACCEPTED, claim narrowed to what
  the test proves (every artifact the consumer needs is produced; the
  test stands in its place). MAJOR — the spend-lock namespace had no
  config carrier or owner → ACCEPTED: required `spend_lock_dir`, root
  ownership and release discipline, config docs in the close inventory.

- **Round 3 (NO-SHIP, second-model design review):** BLOCKER — the
  "capacity is the ONE missing preflight-refusable precondition" claim was
  FALSE: a `clock_uncertainty` at or past `min(recv_window,
  aheadTolerance)` passes config validation and Dispatch's anchor check,
  then `checkServerTime` refuses every send post-fsync, deterministically
  → ACCEPTED and absorbed (§68b.5a: `NewSignedClient` applies
  `checkServerTime` itself to a construction-time reading; complete
  because calibration is per-process constant). BLOCKER — the specified
  consume check `used + units > Capacity` WRAPS (the fold accepts full
  uint64; window.go's `admits` already refuses this exact expression for
  this exact reason), and Reserve's batch sum could wrap or saturate into
  a total inconsistent with `reserved` → ACCEPTED: non-wrapping
  comparisons, overflow-checked accumulation erroring the batch, MaxUint64
  fixtures — §66 round 6's own lesson recurring on new arithmetic, caught
  by the same reviewer class that caught it there. MAJOR — no composed
  test proved `Do` OBEYS a `ReasonCeiling` consume answer (a
  pause-only implementation passes every unit fixture and sends past
  folded capacity) → ACCEPTED: composed transport fixture + the
  fold-induced acceptance variant. MAJOR — §68a's headline still said
  "charged as ONE operation before the fsync", the round-1 model this
  record rejects → ACCEPTED, reworded in place with the drift noted.
  MINOR — "exact-length" hold verification had no N+1 fixture, so
  equality weakened to `<` survived the plan → ACCEPTED. The round also
  cleared `freeAt`-ignores-occupancy against every shipped caller (no
  retry loop exists to be misled) and upheld the capacity-check-at-consume
  invariant argument.

- **Round 4 (NO-SHIP, second-model design review):** BLOCKER — the round-2
  wiring (`Deps.VenueOrderReserves` as its own field) let a root wire
  reservation and placement to two different clients/governors: the hold
  either consumes against a governor the send never charges, or
  mismatches post-fsync into `StateSubmissionUnknown` → ACCEPTED with the
  reviewer's proposed shape: one widened `Deps.VenueOrders` provider, one
  driven gateway, narrowed into the two ports — the split inexpressible —
  plus governor identity on the Hold refused pre-send at `Do` as defense
  in depth; the one-field-per-capability precedent is recorded as
  deliberately not followed, with the reason. MAJOR — no test composed
  the REAL driven adapter + OrderClient + transports + governor (the
  skip-last-leg adapter mutant survives every planned fixture) →
  ACCEPTED: the composed two-leg exact-counts test and the mutant named
  into the campaign. MINOR ×2 — superseded prose (§68a's "the one
  precondition", §68d item 5's timed-charge reasoning) → fixed. The
  round also VERIFIED §68b.5a against the shipped clock (wall.go copies
  calibrations at construction, no mutation path; notimenow permits the
  injected read) and endorsed the absorb-vs-file criterion ("absorb what
  is required to make this slice's stated pre-fsync contract true without
  adding a new consumer or coordinator").

## Appendix A — implementation spec (file by file)

*To be written when the design converges (post-rounds).*
