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
> wall.go — §68f); round 5 NO-SHIP (2B/2M: one blocker and one major
> REBUTTED as pre-existing transport/restart properties this slice
> neither narrows nor widens — §68d items 10/11 — one blocker accepted
> with the venue-scoped lock rekeying, one major accepted as a
> guarantee-narrowing; §68f); round 6 NO-SHIP (1B, accepted and
> code-verified: a malformed endpoint host is the third member of round
> 3's boot-detectable post-fsync genus — canonical base-URL parse at
> NewSignedClient, price seam widened to prepare, §68f); round 7 NO-SHIP
> (3B, all real, all code-verified: the lock rekeyed from the open venue
> alias to the closed SpendDomain; the canonical URL check gains
> `u.Hostname() != ""` — `:443` round-trips byte-for-byte with an empty
> hostname; control bytes refused in credentials at NewCredential — the
> genus's fourth member — with the representative-wire half argued down
> as §68d item 12; §68f); round 8 NO-SHIP (1M, no blockers — the first
> such round: the token's lifetime made structural through the root's
> two-stage lifecycle, four composed root fixtures and the early-release
> mutant named; §68f); round 9 NO-SHIP (1M, no blockers — correcting the
> ROUND-8 FOLD: the retention arm's premise was falsified by the object
> graph, the rule's predicate is now an owned REACHABLE spending
> capability, the oms.New error arm releases, fixture 4 flipped; §68f);
> round 10 NO-SHIP (1B — item 12's own reopening trigger fired: the
> Host header is config-derived and the enumeration omitted it; closed
> by a WriteProxy serialization dry-run at construction, stdlib-only,
> with the direct-path silent-zeroing mechanism corrected into the
> record; §68f); **round 11 SHIP** (zero material findings; convergence
> confirmed against the shipped governor, transport ordering, wiring
> and both roots' lifecycles; items 10–12's reopening triggers checked
> un-fired). **The implementation LANDED (session 100, second-model
> codex session `impl-188-reservation`, two turns — the first cut off
> by a client crash at ~27min with its partial tree preserved and
> resumed): A-1…A-18 complete, zero stated deviations, the §68b.5a
> wall.go constancy caveat re-verified by the builder, exactly one
> [SPEC EDIT OWED] marker (ports.OrderPlacer.Place), `make ci` green on
> both modules VERIFIED BY THE ORCHESTRATOR. Post-write review is
> partially done (platform/transport/venue/oms layers read against
> Appendix A) with TWO QUEUED FINDINGS, not yet fixed: (1) DEFECT —
> `Hold.Consume` (hold.go, the capacity loop) lacks `admits`' `units ==
> 0` guard, so a fold past capacity on a window an entry charges ZERO
> units on refuses the leg — a §68e discrimination-fixture violation;
> fix is `if units == 0 { continue }` plus its fixture; (2) NIT —
> signed.go's WriteProxy dry-run discards the `http.NewRequest` error
> before dereferencing (unreachable, but name the unreachability).
> Next: loop both through the SAME codex session, re-verify, finish the
> post-write review (root/renderer/test files unread), collapse
> checklist, mutation campaign per §68e/A-19, docs to v0.9.47, close
> #188.**

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
   manufactures resolution episodes. Round 6 then found a THIRD organ,
   which falsified round 3's implicit "two": an endpoint host that cannot
   assemble into a parseable URL (`%`, an unclosed IPv6 bracket) passes
   config validation (non-empty is all it checks) and
   `SignedClientConfig.Validate` (a five-character denylist), and then
   `http.NewRequestWithContext` refuses EVERY send from that family —
   post-fsync, post-consume, deterministically (verified against the
   pinned toolchain this session: `https://%/…` fails URL parsing; so
   does `[::1`; and `user@host`/`host:99999` parse but send elsewhere or
   nowhere). Round 7 found the FOURTH: a control byte inside an API key
   passes `NewCredential` and is rejected by Go's transport at send —
   post-consume, post-fsync, on every order. This slice closes all four —
   the race by reservation, the three config-static organs by
   construction-time refusals (§68b.5a) — and the round-6 lesson is
   recorded as the genus's own rule: any config-static fact a send will
   read must be provable at construction, by running the SAME code the
   send runs (checkServerTime, url.Parse) wherever that code is
   reachable, never a restatement — and where the send's own code is NOT
   reachable at construction (Go's header validator, §68d item 12), by
   refusing the input class at its carrier and recording the enumeration
   that makes the refusal complete.

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

**Take the one-spender guard, host-scoped and keyed by the SPEND DOMAIN.**
§67 recorded the premise — exactly one consumer spends this UID at a time —
as operational, with this task as its enforcement home. The deliverable is
a cross-process exclusive lock per spend domain, acquired at composition
time, refused at `NewSignedClient` when absent: two lock-aware processes
spending against one exchange on one host becomes a boot refusal naming
the lock, not a silent split-brain self-charge. The key was narrowed twice
by review, and both corrections point the same way — toward the CLOSED
name for what actually shares a budget. Round 5: the budgets the premise
protects are scoped ABOVE the API key (§7.6's IP-ban envelope is per
egress IP whatever the credential; Bybit's meters are per UID, and a UID
can hold several keys), so a per-credential lock lets two processes with
different keys both boot while splitting one shared envelope's accounting.
Round 7: `ids.Venue` is an OPEN alias a config chooses (`binance` beside
`binance_prod`, both naming one exchange), so a per-venue-alias lock file
lets two processes spending the same exchange acquire two files and both
boot — the alias must not enter the namespace at all. The lock is
therefore keyed by `transport.SpendDomain`, a closed enum the SIGNER
implementations answer (`BinanceSigner` → `SpendDomainBinance`,
`BybitSigner` → `SpendDomainBybit`): compiled constants, not config, so
the namespace cannot be split by spelling. That key is the conservative
shared model: it refuses same-host concurrency under EVERY reading of how
the exchange attributes the traffic, and the deployments it wrongly
refuses — two independent same-exchange accounts, or testnet beside
mainnet, spending from one host — are ones this system does not run, with
the recorded lock-directory split as the deliberate operator override if
one ever must. The credential fingerprint stays, as BOUND METADATA on the
token (round 1's binding check survives beside the domain check). The
limits are stated rather than papered over: flock is HOST-scoped and
COOPERATIVE — a credential spent from a second host, a NAT'd sibling host
sharing the egress IP, an external script, or a binary predating this slice
all sit outside it, covered only by the operational premise and §67's
revisit trigger, both unchanged. The realistic accident — a cron overlap,
an operator double-run, an exercise run beside a reconcile run — is
same-host and lock-aware, and that class becomes inexpressible.

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
  applicable window, one lock hold). Consume is the LAST pre-send instant
  local accounting can stamp (round 5 caught this sentence claiming the
  charge starts "at the send" — consume and the socket write are separate
  events): the charge ages out `interval` after consume, not `interval`
  after the reservation, which removes the unbounded reserve→consume gap
  (a slow fsync) that round 1's blocker named. The residual consume→send
  latency — sign, build, connect — is byte-for-byte today's Acquire→send
  latency (signed.go steps 2→4), stated in §68c as a boundary this slice
  does not move.

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

`SignedClient` gains the preparation seam — round 6 widened round 3's
`price` into `prepare(call) (ratelimit.Request, error)`: the exact
rate-limit request `Do` builds today, PLUS the structural sendability
check (the method is a valid token and the call's path composes with the
construction-validated base into a URL `url.Parse` accepts — the same
library call `Do`'s request construction runs, with a representative
signed-query shape, so "prepared" MEANS "will not refuse at NewRequest").
One factoring serves both callers, so reserve and send cannot drift:
`ReservePlace` prepares every leg BEFORE `Governor.Reserve` (an
unbuildable call refuses the package whole — nothing occupied, nothing
journaled, nothing sent), and `Do` prepares before it consumes or
acquires. `transport.Call` gains `Hold *ratelimit.Hold`:

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

### 5a. Boot-detectable post-fsync refusals become construction refusals
### (round 3's blocker, and round 6's second instance of the genus)

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

**The base URL is the genus's second member (round 6's blocker), closed
the same way: run the send's own code at construction.** `NewSignedClient`
parses `cfg.Scheme + "://" + cfg.Host + "/"` with `url.Parse` — the
library `Do`'s request construction itself goes through — and refuses
construction unless the round-trip is CANONICAL: parse succeeds,
`u.Scheme` equals the configured scheme, `u.Host` equals the configured
host byte-for-byte, `u.User` is nil, the path is exactly `/`, and any
port parses into 1–65535. The five-character denylist in `Validate` stays
(its refusals name themselves better), but the parse is what makes the
check complete: `%` fails as an invalid escape, `[::1` as an unclosed
bracket — both today pass every shipped validation and then refuse EVERY
send from that family at `http.NewRequestWithContext`, post-fsync and
post-consume, deterministically (verified against the pinned toolchain
this session) — while `user@host` and `host:99999` PARSE today and are
worse than a refusal (the request addresses `u.Host` rather than the
configured string, or a port no dial can reach); the equality and range
checks are what catch those two. Round 10 added the check's LAST layer,
because the parser is not the send path's only judge of the host: a
configured `ho"st` or `<host>` passes `url.Parse` AND every canonicality
clause (Go's URL parser deliberately admits quotes and angle brackets in
an authority), and is refused only by `httpguts.ValidHostHeader` inside
request serialization — where the pinned toolchain's behavior FORKS on a
fact outside our config: with a proxy in the environment the write
errors post-consume, and WITHOUT one Go silently ZEROES the Host header
and sends anyway, so the leg dies as a proxy error or a DNS/dial failure
on the garbage authority — post-fsync, deterministically, either way
(both arms verified against request.go this session). The construction
check therefore ends with a serialization DRY-RUN:
`http.NewRequest` against the validated base, then
`req.WriteProxy(io.Discard)` — WriteProxy, deliberately, because the
proxy arm is the one that surfaces `ValidHostHeader`'s verdict as an
error while the direct arm hides it in a zeroed header; the predicate is
the same `ValidHostHeader` call the transport's own write runs, reached
through the stdlib with no network and no new dependency. This is the
#186 move on a URL, completed: derive the canonical form through the
send path's own parser AND its own serializer, and refuse a config
either one rejects. What construction proves, `Do` never re-litigates:
the base is a constant of the client. The remaining dynamic
URL parts are closed by construction elsewhere, and the closure is a
CLAIM this record must not let age silently: paths and methods are codec
static literals (the codec goldens' subject), and the signed query is
`Params.Encode`, which runs every key and value through `url.QueryEscape`
and appends a hex signature — so no canonical base composes with them
into an unparseable URL. §68b.2's prepare seam is where that claim is
exercised per call rather than trusted from this paragraph.

The canonical-parse rule as round 6 wrote it had a hole round 7 found by
probing it: `:443` ROUND-TRIPS — parse succeeds, `u.Host` equals the
configured string byte-for-byte, userinfo is nil, the port is in range —
while `u.Hostname()` is EMPTY, and an empty hostname addresses whatever
listener the resolver finds (a local server received the request, API-key
header and all, in the round's probe) or fails post-consume without one.
So the check requires `u.Hostname() != ""` beside the equality and range
checks: a canonical authority must contain an actual host, not only a
well-formed port.

**The credential is the genus's fourth member (round 7), and its refusal
lands on the CARRIER.** An API key with an embedded LF, CR or NUL passes
`NewCredential` (non-empty is all it checks) and both signers copy the
raw key into a request header, which Go's transport REJECTS at send —
post-consume, post-fsync, on every order, classified `EffectUnknown` by
the non-dial arm. So `NewCredential` refuses a key OR secret containing
any byte below 0x20 or equal to 0x7F: no venue credential legitimately
contains control bytes (the trim in resolveSecret already exists because
a trailing newline is a paste artifact, not a credential character), and
the refusal names the byte's offset without echoing the value. The
remaining header values are closed by their generators — the timestamp
and recvWindow are rendered digits, the signature is hex, the
content-type is a literal — EXCEPT the one round 10 caught this
enumeration omitting: the HTTP Host header is config-derived (it IS the
configured host), and it is exactly what the serialization dry-run above
proves at construction. With Host covered by the dry-run and the API key
by the carrier, the enumeration is complete and recorded HERE rather
than re-proved at runtime; §68d item 12 records the history — why the
fuller representative-wire validation was first rejected, and how its
own reopening trigger fired for precisely this header.

### 5b. The one-spender guard: `transport.SpendLock`, keyed by the closed
### SPEND DOMAIN, BOUND to the credential that spends it

Three review findings shaped the key, each narrowing it toward the same
principle: the lock file's name must be the CLOSED, COMPILED name of what
actually shares a budget, with nothing config-chosen in it. Round 1's
third blocker: a lock whose fingerprint the CALLER supplies proves only
that some file got locked — two roots signing with one key but
fingerprinting differently (or locking in different directories) both
boot; so every identity on the token is DERIVED, on both sides of the
comparison, from the constructed collaborators themselves. Round 5's
blocker: a lock keyed by the credential guards a NARROWER scope than the
budgets the premise protects — §7.6's venue envelope is per egress IP
whatever the key, and Bybit meters per UID, where one UID can hold
several keys — so two processes with two different keys would each take
their own lock, both boot, and split one shared envelope's accounting
between two governors that each look complete. Round 7's blocker: the
round-5 fix keyed the file by `ids.Venue`, which is an OPEN alias config
chooses and `Validate` checks only as non-empty — `binance` beside
`binance_prod` name one exchange, take two files, and both boot, the
fingerprint metadata unable to join namespaces it never names. So the key
is `transport.SpendDomain` — a closed enum declared beside the lock,
answered by the SIGNER implementations as compiled constants
(`BinanceSigner` → `SpendDomainBinance`, `BybitSigner` →
`SpendDomainBybit`; a new dialect arrives with its member, and
`exhaustiveswitch` covers every switch over it). The config alias never
enters the namespace, and the credential fingerprint rides on the token
as bound metadata:

- `transport.Credential` gains `Fingerprint() string` — hex SHA-256 of the
  API key. The key never touches the filesystem; the fingerprint is not
  secret (the key is an identifier the venue sees on every request; its
  hash identifies without revealing).
- `transport.SpendDomain` is a closed string enum with `Validate` over
  its declared members. Filesystem safety is by construction — the
  members are compiled lowercase constants — and `Validate` is what keeps
  a future member honest rather than a runtime charset rule.
- `transport.AcquireSpendLock(dir string, domain SpendDomain, cred Credential) (*SpendLock, error)`
  — an exclusive, non-blocking flock on `<dir>/<domain>.lock`, domain
  validated and fingerprint computed INSIDE from `cred`, both stored on
  the returned token. Held for the process's life; `Close` releases.
- `Signer` gains `Fingerprint() string` (each signer already holds its
  `Credential`; both implementations delegate to it) and
  `SpendDomain() SpendDomain` (a constant per implementation — the
  comparison's other side, compiled rather than configured).
- `SignedClientConfig` gains `Spend *SpendLock`, REQUIRED — and
  `NewSignedClient` refuses a token whose DOMAIN differs from the
  signer's (`Signer.SpendDomain()`) or whose FINGERPRINT differs from the
  signer's (`Signer.Fingerprint()`). A present-but-wrong lock — minted
  for another exchange, or for a credential this signer does not hold —
  is the wiring mistake the two comparisons exist for; "non-nil" alone
  proves nothing, and proving nothing while looking like a guard is worse
  than absence. Both sides of both comparisons are derived (the root's
  resolved credential and its dialect's signer feed the lock; the
  client's own signer feeds the check), so a root that builds the signer
  on one exchange and the lock on another cannot construct a client.

One root acquires once per spend domain and shares the token across that
domain's clients (Binance's two per-family transports, the order and read
clients): the lock is per-domain, not per-client or per-credential, and a
second acquire of the same domain in ONE process is refused too — flock
locks belong to the open file description, so a second open-and-flock
conflicts even in-process (verified on darwin this session; linux
documents the same). A future cross-venue root acquires one lock per
domain it spends. Non-flock platforms (build-tagged, the storage_other.go
precedent) refuse to construct the lock: no platform gets a silent no-op
guard.

**The namespace has a carrier and an owner (round 2):** the lock directory
is a REQUIRED deployment config field (`spend_lock_dir` in each
credentialed root's config schema — reconcile's today, any future
credentialed root's by the same rule), validated at load like the journal
path. The ROOT owns the token, and round 8 made the ownership STRUCTURAL
rather than prose, because the root's lifecycle is two-stage and the
prose named neither stage: `newOrderClient` constructs the spending
clients, then a separately fallible `oms.New` runs, and on ITS failure
the root deliberately stays ALIVE and halted (app.go's watchdog
argument) — so "release on shutdown and on failure paths" had two
implementations that both passed every planned test, one of which
(a `defer Close` inside the wiring helper) releases the lock the moment
wiring RETURNS, leaving the process spending unguarded for its whole
life. The rule that decides every path is: **the token is held exactly
as long as an OWNED, REACHABLE spending capability exists** — round 9
sharpened the predicate after round 8's first cut ("a constructed
spending client exists") misread the object graph: after `startOMS`'s
`oms.New` error arm returns, the client is an unreachable local — no
Runner field stores it, `oms.New` returns no partial module, nothing
owns a goroutine that could touch it — so "was constructed" held while
"can be spent from" did not, and the retained lock protected against no
possible spender while refusing the operator's corrected invocation
until process death. Concretely: `newOrderClient` acquires per spend
domain after resolving the credential and returns the token in its named
wiring result (a carrier, not a deferred release); on a wiring failure
INSIDE it — no client constructed — it releases before returning the
error. The Runner stores the token beside the governor; on an `oms.New`
failure it CLOSES and clears the token in the error arm, joining a close
failure into the recorded cause — the halted-alive root keeps its
diagnosis and its posture, and the domain is free for the corrected run.
The success path's phase closer releases the token AFTER
`module.Close()` — the same LIFO discipline every closer in this root
follows — and the OS's release-on-exit remains the backstop for every
path that dies instead of returning. The
docs-owed inventory at close includes the config schema and its example.
The one deployment a domain-keyed lock refuses that an alias-keyed one
admitted — testnet beside mainnet of one exchange on one host — is
refused DELIBERATELY (their budget separation is a venue claim nobody
verified), and the recorded lock-directory split is the operator
override if that run ever must happen.

**Residuals, stated (rounds 2 and 5):** the guarantee holds among
LOCK-AWARE processes on ONE host sharing ONE lock directory, and each
qualifier is a recorded residual, not fine print. Two roots configured
with different lock directories split the namespace and both boot — a
config error the code cannot see (the same class as pointing two roots at
different journals). A process that never asks — an external script, a
binary built before this slice — spends unguarded; flock is cooperative by
nature, and the mitigation is operational (retire pre-lock builds of the
credentialed binaries when this lands; no credentialed daemon runs today,
so there is no live migration window to drain). Cross-host sharing of a
credential, and a NAT deployment where two hosts share one egress IP's
envelope, likewise stay on §67's operational premise and revisit trigger,
both unchanged — the $20/$55 live-test decision's single-spender
confirmation covers the whole premise, not only the same-host slice of it.

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
- **The admission→send window and the charge's clock.** Today a charge is
  stamped at `Acquire` (signed.go step 2) and the socket write happens at
  step 4; the latency between them — sign, build, connect, plus any
  process stall — is unmetered, and a stall longer than a window's
  interval lets the charge evict locally before the venue counts the call.
  With a hold, `Consume` stamps at the same seam, so the window is
  identical in both modes: this slice moves the charge's clock from
  reserve time (unbounded gap, round 1's blocker) to the last pre-send
  instant, and stops there. §68d item 10 records why it does not chase the
  residue.
- **Restart cold-start.** A fresh process builds a fresh governor
  (wire.go) and learns the venue's live state the way it always has: the
  first response's upward fold, and the first refusal's penalty. The spend
  lock serializes CONCURRENT spenders; it transfers nothing between
  SEQUENTIAL ones, exactly as a plain restart transfers nothing today.
  §68d item 11 records why.

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
   releases the other (§the-exemption lesson); the lock covers every
   venue. (Round 5 then widened the KEY from credential to venue —
   §68b.5b — which subsumes this item's conclusion rather than reopening
   it.)
10. **In-flight charge accounting** (round 5's proposed fix for the
    admission→send latency: a `reserved → in_flight → consumed` ladder
    where the charge is stamped only after `http.Client.Do` returns, on
    ordinary Acquire traffic as well). Rejected as out of this slice's
    scope, not as wrong: the latency it meters is the shipped transport's
    own (step 2 charges, step 4 sends — slice C1's shape, unchanged by
    this design, present identically on the nil-hold path), so it is not
    a defect this slice introduces and not one the pre-fsync contract
    this slice completes says anything about. The failure needs a
    multi-second stall inside a microseconds-wide code region; its blast
    radius is one venue refusal the penalty path already absorbs; and the
    fix would put a second state machine on EVERY governor charge in the
    system to shave it. Consume stamps at the same seam Acquire stamps
    today — parity, claimed as parity (§68b.1, §68c). Reopening trigger:
    evidence of a real over-admission traced to this window, or a
    transport change that widens it.
11. **Persisted governor handoff state** (round 5's proposed fix for
    restart cold-start: crash-safe persistence of live charges and pause
    horizons under the lock, or a startup quarantine spanning the longest
    window). Rejected: pre-existing restart behavior this slice neither
    narrows nor widens, and the mechanism is the wrong authority — the
    venue's headers are the designed channel for consumption this process
    did not see (the upward fold exists for exactly this), and a persisted
    local snapshot is a stored form containing the answer, aging while it
    sits and drifting from the venue's own count the moment any
    non-participant spends. The 1s endpoint windows heal faster than any
    boot; the surviving cost is one request against a still-live pause,
    which re-announces itself and is absorbed by the penalty path — the
    same cost every restart pays today. A startup quarantine would charge
    every clean boot the worst case's price to remove a bounded one-request
    residue.
12. **Representative-wire header validation at construction** (round 7's
    proposed completion of the credential fix: sign a representative
    request at `NewSignedClient` and validate every generated header with
    the transport's own validator before accepting the client). Rejected
    on the seam, not the principle: Go does not export the validator —
    `httpguts.ValidHeaderFieldValue` runs inside the transport at send,
    `http.NewRequest` and `Request.Write` never invoke it — so
    construction could only run a RESTATEMENT of the charset rule, which
    is the copy-that-drifts the genus rule forbids, or the engine module
    could take `golang.org/x/net` as a dependency to re-run a check whose
    only config-derived input the carrier now refuses (§68b.5a: control
    bytes cannot enter a `Credential`). Every other header value is
    generator-typed — digits, hex, a literal — and that enumeration is
    recorded in §68b.5a beside the refusal. Reopening trigger: a header
    value whose source is neither the credential nor a shipped generator
    (an operator-configured header, a venue demanding a config-derived
    field), which arrives with its consumer and re-raises the question at
    the seam that adds it. **The trigger FIRED in round 10, and the item
    is amended rather than deleted:** the HTTP Host header was always
    config-derived and the round-7 enumeration missed it. Its resolution
    also REVISED this item's premise at that one header: a stdlib
    no-network seam DOES exist for the Host — `req.WriteProxy(io.Discard)`
    surfaces the transport's own `ValidHostHeader` verdict as an error —
    so §68b.5a's dry-run covers it with no new dependency. What this item
    continues to reject is general header-VALUE validation at
    construction, for which the round-7 objection still holds (the value
    validator runs only inside the transport), and the trigger continues
    to govern any FUTURE config-derived header.

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
at/above refuse naming both quantities. The base-URL refusal at
NewSignedClient (rounds 6 and 7), one fixture per failure MODE rather
than per string: `%` (invalid escape), `[::1` (unclosed bracket),
`user@host` (non-nil userinfo — parses, refused by equality),
`host:99999` (port out of range — parses, refused by range), `:443` and
`:80` (round 7: round-trip byte-for-byte with an EMPTY hostname —
refused by the `u.Hostname() != ""` check, and the pair matters because
each passes every OTHER check), `ho"st` and `<host>` (round 10: pass the
parse AND every canonicality clause, refused only by the serialization
dry-run — the pair that isolates the WriteProxy layer from the parse
layer), `host/x` (denylist, the shipped refusal's positive control that
the parse did not replace it), and the plain valid host constructing
(positive control). No live-listener
fixture: what a resolvable empty hostname addresses is the platform
resolver's behavior, not this check's — the construction refusal is the
claim, and the fixture asserts the refusal. The credential refusal at
NewCredential (round 7): a key with an embedded LF, a secret with an
embedded NUL, a key with 0x7F — each refused naming the offset and not
the value (assert the error string omits the key) — beside the
plain-alphanumeric positive control; and the refusal is in the CARRIER,
so both signers inherit it without a fixture of their own. And the prepare-seam
refusal pre-reserve: a codec double returning a Call whose path cannot
compose into a parseable URL — `ReservePlace` refuses with ZERO occupancy
(snapshot equality with the pre-call golden), zero journal appends, zero
HTTP requests; the mixed fixture is two legs with the invalid one SECOND,
so "refused whole" and "first leg checked first" are distinguishable —
the ORDER dimension again. (The malformed-HOST variant of that mixed
package needs no executor fixture: construction already refused the
client, so the composition is inexpressible — asserted by the
construction fixture, not re-proved downstream.)

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
refuses a nil SpendLock AND both mismatch axes separately — a token minted
for a different DOMAIN with this signer's credential, and a token minted
for this domain with a DIFFERENT credential (round 1's binding fixture,
round 5's key fixture — two fixtures, because one mismatched-both token
cannot tell which comparison fired) — with the positive control beside
them; AcquireSpendLock refuses a held domain (second fd in-process — flock
semantics make this testable without a second process) in BOTH the
fixtures that distinguish the key from its two rejected predecessors: two
DIFFERENT credentials on one domain (round 5's scenario — under the
credential key this pair boots), and one credential under two DIFFERENT
`ids.Venue` aliases whose signers share a dialect (round 7's scenario —
under the alias key this pair boots; the fixture constructs both signers
and asserts they answer the SAME SpendDomain, which is the property the
whole fix is) — with the positive control releasing and reacquiring;
AcquireSpendLock refuses an undeclared SpendDomain member (Validate's
fixture) with the declared members as positive controls; the lock file is
named by the domain constant, and neither the key, its fingerprint, nor
the config alias appears in the lock dir's file NAMES (assert directory
listing; the fingerprint may sit inside the token's metadata, never in
the namespace). And the ROOT-lifecycle composition (round 8 — the lock
primitives above cannot see a wiring helper that releases too early):
through the real Runner, (1) while the Runner is up, a second
AcquireSpendLock on the same domain FAILS — this is the fixture that
kills the defer-Close-inside-newOrderClient mutant, the campaign's named
early-release mutant, because under that mutant wiring has returned and
the second acquire succeeds; (2) after a normal shutdown (the phase
closer ran), the second acquire SUCCEEDS; (3) after a wiring failure
inside newOrderClient (no client constructed), the second acquire
SUCCEEDS — release-before-error observed; (4) after an `oms.New` failure
(root alive and halted, the client an unreachable local — round 9's
correction of this fixture's first form, which asserted retention on a
capability nothing owned), the second acquire SUCCEEDS immediately —
release-in-the-error-arm observed, with the close error joined into the
recorded cause when it fails, and the root still halted with its
diagnosis intact. Fixtures (3) and (4) release at DIFFERENT sites
(inside newOrderClient; in startOMS's error arm), so both stay —
per-site release mutants need per-site fixtures — and the campaign's
second named lifecycle mutant is deleting the error-arm release, which
only fixture (4) sees. (The OS-releases-on-exit backstop is flock's
documented behavior, recorded rather than subprocess-tested — the
round-7 live-listener rule again.)

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

- **Round 5 (NO-SHIP, second-model design review — the convergence check;
  two findings accepted, two rebutted with recorded arguments):**
  BLOCKER — Consume stamps the charge before the socket write, so a
  stall longer than the interval between consume and send evicts the
  charge before the venue counts the call; proposed a
  `reserved → in_flight → consumed` ladder stamping after
  `http.Client.Do`, applied to ordinary Acquire traffic too → REBUTTED
  as pre-existing: the consume→send window is byte-for-byte the shipped
  Acquire→send window (signed.go steps 2→4, slice C1's shape), the
  reviewer's own second-order note places the same defect on the
  nil-hold path, and the design claims parity there in as many words
  (§68b.1's linearization paragraph). Out of the round-4 absorb
  criterion — the window is post-admission and the pre-fsync contract
  says nothing about it. The proposal is §68d item 10 with its reopening
  trigger; what the finding DID catch is a real prose overclaim ("the
  charge's window starts … at the send"), fixed in place. BLOCKER — the
  lock key (SHA-256 of the API key) guards a narrower scope than the
  budgets the premise protects: §7.6's venue envelope is per egress IP
  and Bybit meters per UID (one UID, several keys), so two same-host
  processes with different keys both boot and split one envelope's
  accounting → ACCEPTED with the reviewer's conservative host variant:
  the lock file is keyed per VENUE (`<dir>/<venue>.lock`, both sides
  derived from construction inputs), the credential fingerprint demoted
  to bound metadata on the token, `NewSignedClient` checking venue and
  fingerprint as two separate refusals; §68d item 9's conclusion is
  subsumed, the two-different-keys fixture named into §68e. The
  full-strength variant (authoritative verified UID and egress
  identities) was NOT taken: both identities are unverifiable locally
  and the venue-key already refuses every same-host composition. MAJOR —
  a sequential lock handoff starts the successor on an empty governor
  while the venue still counts the predecessor's requests or pause;
  proposed persisted handoff state or a startup quarantine → REBUTTED as
  pre-existing restart behavior (every root builds a fresh governor
  today, lock or no lock; the upward fold and the penalty path are the
  designed relearning channel; the residue is one request against a
  still-live pause, the same cost every restart pays now) — §68d item
  11. MAJOR — the boot-refusal claim quietly assumed every spender
  participates, while flock is advisory and a pre-lock binary or
  external script ignores it → ACCEPTED as a guarantee-narrowing:
  §68a/§68b.5b now state the lock-aware/one-host/one-directory scope as
  named residuals with the operational mitigation (retire pre-lock
  builds; no credentialed daemon runs today, so no live migration window
  exists), and the proposed drain/rotation MECHANISM was not taken —
  machinery for a migration this deployment cannot experience.

- **Round 6 (NO-SHIP, second-model design review):** BLOCKER — a
  malformed endpoint host (`%`, `[::1`) passes config validation
  (non-empty is its only host check), passes
  `SignedClientConfig.Validate` (a five-character denylist), and then
  fails `http.NewRequestWithContext` on EVERY send from that family —
  post-fsync, post-consume, deterministically: `EffectNotApplied` +
  non-reject → durable `StateSubmissionUnknown`, possibly beside a
  filled sibling — the third member of round 3's genus, found by probing
  the "closes both" claim → ACCEPTED and verified live this session
  (`%` and `[::1` fail NewRequest under the pinned toolchain;
  `user@host` and `host:99999` PARSE, which is worse — the request
  addresses the parsed host or an undialable port). Absorbed twice, per
  the reviewer's two-part fix: `NewSignedClient` canonical base-URL
  parse-and-compare (§68b.5a — url.Parse itself, scheme/host equality,
  nil userinfo, port range; the send's own code, never a restatement),
  and the `price` seam widened to `prepare` (§68b.2) so `ReservePlace`
  refuses a structurally unbuildable call before `Governor.Reserve` —
  nothing occupied, nothing journaled — closing the codec-produced
  method/path residue the host check cannot see. The query half was
  verified closed by construction (`Params.Encode` QueryEscapes every
  key and value; the signature is hex) and the claim is exercised by the
  prepare seam per call rather than trusted as prose. The round's three
  demanded fixtures are in §68e, the mixed-package one carrying the
  ORDER dimension (invalid leg second) and the malformed-host mixed
  variant recorded as inexpressible past construction rather than
  re-proved downstream. Condition ¶3 now states the genus rule: any
  config-static fact a send will read must be provable at construction
  by running the send's own code.

- **Round 7 (NO-SHIP, second-model design review; three blockers, all
  real, all code-verified, one absorbed at reduced scope):** BLOCKER —
  the round-5 lock key (`ids.Venue`) is an OPEN config alias validating
  only non-empty, so `binance` beside `binance_prod` — one exchange, one
  credential, one host — take two lock files and both boot, the
  fingerprint metadata joining nothing → ACCEPTED with the reviewer's
  fix: the key is `transport.SpendDomain`, a closed enum the signer
  implementations answer as compiled constants, so the namespace cannot
  be split by spelling; the round-6 charset rule became moot (constants)
  and was replaced by `SpendDomain.Validate`; the two-aliases-one-dialect
  fixture joins the two-credentials-one-domain fixture as the pair that
  distinguishes this key from BOTH rejected predecessors. The
  deliberately-refused deployment (testnet beside mainnet, same host) is
  recorded with the lock-directory split as the operator override.
  BLOCKER — the round-6 canonical-parse rule passes `:443`: it
  round-trips byte-for-byte (host equality, nil userinfo, valid port)
  while `u.Hostname()` is EMPTY, and the round's live probe showed an
  empty hostname addressing a local listener, API-key header and all →
  ACCEPTED verbatim: `u.Hostname() != ""` joins the check, `:443`/`:80`
  join the fixtures (each passes every OTHER check — the pair isolates
  the new clause); the demanded live-listener fixture was NOT taken (it
  asserts the platform resolver's behavior, not the refusal — the
  construction fixture is the claim). BLOCKER — an API key with an
  embedded LF/CR/NUL passes `NewCredential` and both signers copy it
  into a header Go's transport rejects at send — post-consume,
  post-fsync, every order, `EffectUnknown` via the non-dial arm — the
  genus's fourth member, config-static in the CREDENTIAL → ACCEPTED at
  the carrier: `NewCredential` refuses control bytes in key or secret
  (offset named, value never echoed), the remaining header values
  recorded as generator-typed (digits, hex, a literal); the fuller
  representative-wire validation is §68d item 12, rejected on the seam —
  Go does not export the transport's validator, so construction could
  run only a restatement or buy an x/net dependency to re-check an input
  the carrier now refuses — with its reopening trigger (a header value
  sourced from neither the credential nor a shipped generator).

- **Round 8 (NO-SHIP, second-model design review; one MAJOR — the first
  round with no blocker):** MAJOR — the token's lifetime discipline was
  prose over a lifecycle the prose never named: the root is two-stage
  (`newOrderClient`, then a separately fallible `oms.New` whose failure
  leaves the process deliberately ALIVE and halted), so "release on
  shutdown and on failure paths" admitted two implementations that both
  pass every planned fixture — a `defer Close` inside the wiring helper
  (the lock releases when wiring RETURNS; the process spends unguarded
  for its whole life; §67's premise silently void) and a no-transfer
  variant (the halted root pins the domain). ACCEPTED with the ownership
  made structural and the ambiguity resolved by a discriminating rule
  rather than either blanket reading: the token is held exactly as long
  as a constructed spending client exists — wiring-internal failures
  release before returning (nothing can spend), an `oms.New` failure
  RETAINS (clients exist; releasing while the capability exists is the
  vacuous-guard direction, and the reviewer's "prevents corrected
  invocations" cost is flock working as designed, bounded by the OS
  releasing on process death), and the success path's phase closer
  releases after `module.Close()`. The carrier is the named wiring
  result; the four composed root fixtures and the
  defer-Close-inside-wiring mutant are §68e's round-8 block. The
  finding's second-order claim (a leaked lock "prevents corrected
  invocations" indefinitely) was CORRECTED in the folding: flock cannot
  outlive its process, so the leak direction is bounded by the halted
  process's own death — recorded as the backstop, not tested from a
  subprocess.

- **Round 9 (NO-SHIP, second-model design review; one MAJOR, no
  blockers — and the finding corrects the ROUND-8 FOLD, not the round-8
  finding):** MAJOR — the fold's retention arm rested on a premise the
  object graph falsifies: after `startOMS`'s `oms.New` error arm
  returns, the client is an unreachable LOCAL (no Runner field stores
  it, `oms.New` returns no partial module, nothing owns an execution
  path that could touch it), so "a constructed spending client exists"
  was true while "anything can spend" was false, the retained lock
  protected against no possible spender, and §68e's fixture 4 ENCODED
  the wrong behavior — a fixture asserting a defect is worse than a
  missing one, because it rejects the correct implementation → ACCEPTED
  in full: the rule's predicate is now an OWNED, REACHABLE spending
  capability; the `oms.New` error arm closes and clears the token
  (close failure joined into the recorded cause, diagnosis and halted
  posture intact); fixture 4 flipped to assert immediate reacquisition;
  the error-arm release joined the campaign as the second named
  lifecycle mutant. The orchestrator's round-8 rationale ("the
  halted-alive root still holds spend-capable clients") is left in the
  round-8 entry as history — this entry is its correction, and the
  lesson is the repo's own: a lifetime rule's predicate must name what
  the OBJECT GRAPH holds, not what a constructor once produced.

- **Round 10 (NO-SHIP, second-model design review; one BLOCKER — §68d
  item 12's own reopening trigger fired):** BLOCKER — the HTTP Host
  header is a config-derived header value the round-7 enumeration
  omitted: `ho"st` and `<host>` pass `url.Parse` and every round-6/7
  canonicality clause (Go's URL parser deliberately admits quotes and
  angle brackets in an authority) and are judged only by
  `httpguts.ValidHostHeader` inside request serialization — post-consume,
  post-fsync, deterministic → ACCEPTED, with the folding CORRECTING the
  mechanism claim on the way in: the reviewer's probe used the proxy
  write path, and the pinned request.go shows the direct path does not
  error — it silently ZEROES the invalid Host and sends, so the
  production failure is a proxy error under HTTPS_PROXY or a DNS/dial
  failure on the garbage authority (both arms verified this session;
  the deterministic post-fsync leg death stands either way). The fix is
  the serialization DRY-RUN at construction —
  `req.WriteProxy(io.Discard)`, deliberately the proxy arm, because it
  alone surfaces `ValidHostHeader`'s verdict as an error — which also
  REVISED item 12's premise at this one header: a stdlib no-network
  seam exists for the Host, so the check costs no new dependency. Item
  12 is amended (trigger fired, Host covered, general header-value
  validation still rejected, trigger still governs future
  config-derived headers), §68b.5a's enumeration now names Host as the
  member it omitted, and the `ho"st`/`<host>` pair joins the fixtures
  as the layer-isolating cases.

## Appendix A — implementation spec (file by file)

Written at round 11's SHIP. The evidence plan is §68e and is not restated
here; each file's tests are named where they are not obvious from it.
Doc comments follow the §68b section that specifies the construct — the
ARGUMENTS recorded there (why occupancy is untimed, why freeAt ignores
it, why the lock is domain-keyed) belong on the constructs, not only in
this record.

### A-1. `internal/platform/ratelimit/window.go`

- `window` gains `reserved uint64` — untimed, never evicted, invisible to
  `observe` (doc comment states the contract from §68b.1: the venue's
  report can never include unsent reservations).
- `admits` counts it, in the non-wrapping direction:
  `units == 0 → true; used := usedAt(now); if used >= ceiling → false;
  avail := ceiling - used; if w.reserved >= avail → false;
  return units <= avail - w.reserved`.
- `freeAt` is UNCHANGED and gains the §68b.1 doc paragraph (occupancy is
  cancellable, so the earliest instant a retry could succeed is the
  charges-only answer; computing over `ceiling - reserved` would overload
  the documented zero).
- New unexported methods, all called only under the governor's lock:
  `reserve(units uint64)` (`reserved += units` — no saturation needed:
  Reserve's overflow-checked admission bounds the sum below capacity),
  `release(units uint64)` (`reserved -= units`; entries are the only
  callers, so underflow is structurally impossible — assert nothing,
  document that), and `consume(now time.Time, units uint64)`
  (`reserved -= units; w.charge(now, units)` — the occupancy→charge
  conversion, one method so no interleaving between the two halves).
- The consume-time CAPACITY check lives in hold.go, not here; window
  contributes `usedAt`.

### A-2. `internal/platform/ratelimit/hold.go` (new file)

- `Hold` — one package's provisional occupancy. Fields: `gov *Governor`,
  `entries []holdEntry`. `holdEntry`: `req Request`,
  `windows []*window` (resolved at reserve time — the profile is
  immutable, so resolution cannot drift), `state holdState`
  (`holdReserved → holdConsumed | holdCanceled`, an unexported uint8
  enum; transitions only under `gov.mu`, no transition out of the two
  terminals — §68b.1's state machine, stated in the type's doc).
- `MintedBy(g *Governor) bool` — pointer identity, the §68b.2
  defense-in-depth answer. No accessor exposes the governor value.
- `Consume(r Request) (Decision, error)`: lock `gov.mu`. Match `r` by
  struct equality against entries in `holdReserved` (first match).
  No match → error (`ErrHoldEntryUnmatched` — wrong request, consumed
  already, or canceled; fail closed). Then re-check ONLY the venue's
  word: a pause in force → `Decision{Reason: ReasonPaused,
  NotBefore: pausedUntil}`, nothing charged; for each of the entry's
  windows, with `used := w.usedAt(now)` and
  `units := r.Cost.units(w.limit.Dimension)`,
  `used > w.limit.Capacity || units > w.limit.Capacity - used` →
  `Decision{Reason: ReasonCeiling}` via the existing `worseThan`
  attribution, nothing charged, entry still reserved (§68b.1: the
  non-wrapping form is MANDATORY — window.go's `admits` documents why
  the sum wraps). On admission: `entry.state = holdConsumed`, then
  `w.consume(now, units)` per window, one lock hold, and return the
  admitted Decision.
- `Cancel() error`: lock `gov.mu`. For every entry in `holdReserved`:
  mark `holdCanceled`, then `w.release(units)` per window. Consumed
  entries untouched. Repeated Cancel: idempotent no-op, nil error.
- Nil-receiver behavior is NOT defined — a nil `*Hold` at a consume site
  means "acquire at send" and never reaches these methods; transport
  branches on nil before calling.

### A-3. `internal/platform/ratelimit/governor.go`

- `Reserve(reqs []Request) (*Hold, Decision, error)` — §68b.1's bullets,
  in order, all under one lock acquisition:
  1. `len(reqs) == 0` → `ErrReserveEmpty`.
  2. Per-request validation IDENTICAL to `decide`'s gates (factor the
     request-validation prefix of `decide` — family known, endpoint
     rules R1/R2/R3, cannot-count — into an unexported
     `admitRequest(r Request) ([]*window, error)` that BOTH paths call,
     so the taxonomy cannot drift; any error refuses the whole batch,
     nothing occupied).
  3. Mixed priorities → `ErrReservePriorityMixed`.
  4. A pause in force → `Decision{Reason: ReasonPaused}`, Hold nil.
  5. Sum the batch per window with overflow-CHECKED accumulation
     (`a > MaxUint64 - b` → `ErrReserveOverflow`, refusing the batch —
     never saturate; §68b.1's a-number-that-lies-twice argument in the
     doc).
  6. Admission on the sum at the batch priority against
     `ceiling - used - reserved` per applicable window (the same
     non-wrapping shape as `admits`); refusal names the worst limit by
     `worseThan`, Hold nil.
  7. Admission: `w.reserve(sum)` per window, build the Hold (one entry
     per request, resolved windows attached), return it with the
     admitted Decision.
- `LimitState` gains `Reserved uint64` (doc: §68b.1's observability
  paragraph); `Snapshot`'s loop copies `w.reserved`.

### A-4. `internal/platform/ratelimit/errors.go`

`ErrReserveEmpty`, `ErrReservePriorityMixed`, `ErrReserveOverflow`,
`ErrHoldEntryUnmatched`. Wording follows the file's existing style
(lowercase, package-prefixed).

### A-5. `internal/engine/venue/transport` — the spend lock

**`spenddomain.go` (new):** `type SpendDomain string`;
`SpendDomainBinance SpendDomain = "binance"`,
`SpendDomainBybit SpendDomain = "bybit"`; `Validate()` over the closed
set (`ErrSpendDomainInvalid`). Doc carries §68b.5b's three-round key
argument in brief.

**`spendlock.go` (new):** `SpendLock` — fields unexported: `domain`,
`fingerprint`, `path`, platform handle. `Domain() SpendDomain`,
`Fingerprint() string`, `Close() error` (idempotent).
`AcquireSpendLock(dir string, domain SpendDomain, cred Credential) (*SpendLock, error)`:
validate domain and `cred.Validate()`; fingerprint computed INSIDE from
`cred` (§A-6); refuse empty dir (`ErrSpendLockDirUnset`); delegate to
the platform half. Doc: the lock file is `<dir>/<domain>.lock`, contains
nothing secret, and is never deleted (deleting a lock file another
process may be flocking is the classic unlink race — the file persists,
the FLOCK is the lock).

**`spendlock_unix.go` (`//go:build darwin || linux`):** open/create the
file 0o644, `syscall.Flock(fd, LOCK_EX|LOCK_NB)`; `EWOULDBLOCK` →
`ErrSpendLockHeld` naming the path and domain. The
same-process-second-fd conflict is flock semantics and is what §68e's
in-process fixtures rely on.

**`spendlock_other.go` (`//go:build !darwin && !linux`):** constructor
returns `ErrSpendLockUnsupported` — the storage_other.go precedent, no
silent no-op guard.

### A-6. `internal/engine/venue/transport/credential.go`

- `Fingerprint() string` — `hex.EncodeToString(sha256.Sum256([]byte(c.key)))`
  slice form; doc: identifies without revealing (§68b.5b bullet 1).
- `NewCredential` (and `Validate`, mirroring) refuses a key OR secret
  containing any byte `< 0x20` or `== 0x7F` —
  `ErrCredentialControlByte`, naming the FIELD and byte OFFSET, never
  echoing the value (§68b.5a round-7 member; the doc names Go's
  transport-level header rejection as the post-fsync failure this
  forecloses).

### A-7. `internal/engine/venue/transport/signing.go`

- `Signer` interface gains `Fingerprint() string` and
  `SpendDomain() SpendDomain`.
- `BinanceSigner`: delegate fingerprint to its `Credential`; return
  `SpendDomainBinance`. `BybitSigner` likewise with `SpendDomainBybit`.
  Constants, not config — §68b.5b's whole point.

### A-8. `internal/engine/venue/transport/signed.go`

- `SignedClientConfig` gains `Spend *SpendLock` (required).
- `Call` gains `Hold *ratelimit.Hold` (doc: nil means acquire-at-send —
  the single-order and read mode, §68b.2).
- New exported `func (c *SignedClient) Prepare(call Call) (ratelimit.Request, error)` —
  THE seam (§68b.2 as widened by round 6): builds the exact
  `ratelimit.Request` Do prices today (Family, `Limits.Endpoint(path)`,
  Priority, Cost), validates it, and dry-builds
  `http.NewRequest(call.Method, base+call.Path, nil)` refusing on error
  (`ErrCallUnpreparable` wrapping). Do calls Prepare; ReservePlace calls
  Prepare — one spelling.
- `Do` step 2 becomes: `req, err := c.Prepare(call)` (error →
  `apperror.Precondition`); then if `call.Hold == nil` → `Acquire(req)`
  exactly as today; else: `call.Hold.MintedBy(c.cfg.Governor)` false →
  `apperror.Precondition` (`ErrHoldForeignGovernor`, pre-send);
  `decision, err := call.Hold.Consume(req)` — error →
  `apperror.Precondition` (mismatch class), refusing decision →
  `apperror.RateLimited` (pause AND ceiling: both venue-initiated,
  both `EffectNotApplied`). No other change to the step order; the
  step-list doc comment gains the consume branch.
- New `func (c *SignedClient) UsesGovernor(g *ratelimit.Governor) bool` —
  identity answer for venue-level construction; no accessor exposes the
  value.
- `NewSignedClient`, after the existing roster checks, in order:
  1. **Base URL** (§68b.5a rounds 6/7/10): `u, err := url.Parse(scheme+"://"+host+"/")`;
     refuse on error, `u.Scheme != cfg.Scheme`, `u.Host != cfg.Host`,
     `u.User != nil`, `u.Path != "/"`, `u.Hostname() == ""`, or a
     present port outside 1–65535 (`ErrHostNotCanonical`, naming which
     clause). Then the serialization dry-run:
     `req, _ := http.NewRequest("GET", scheme+"://"+host+"/", nil)`;
     `req.WriteProxy(io.Discard)` error → `ErrHostUnserializable`
     (doc: WriteProxy deliberately — the proxy arm surfaces
     ValidHostHeader as an error where the direct arm zeroes; §68b.5a).
  2. **Signing readiness** (§68b.5a round 3):
     `checkServerTime(cfg.Clock.ServerNow(cfg.Signer.Venue()), cfg.RecvWindow)`
     — the function itself, never a restatement; error wrapped with a
     construction-context message naming band and bound. Implementation
     re-verifies wall.go's calibration constancy before landing (the
     §68b.5a caveat); if that reading fails, the check degrades to
     early-warning and the record is corrected — do not silently ship a
     completeness claim the clock no longer supports.
  3. **Spend lock** (§68b.5b): `cfg.Spend == nil` → `ErrSpendLockUnset`;
     `cfg.Spend.Domain() != cfg.Signer.SpendDomain()` →
     `ErrSpendLockDomainMismatch`;
     `cfg.Spend.Fingerprint() != cfg.Signer.Fingerprint()` →
     `ErrSpendLockFingerprintMismatch`. Two comparisons, two errors —
     §68e's two-fixture requirement.
- `Validate` gains nothing for these (they need constructed
  collaborators; Validate stays field-shape checks). The five-character
  host denylist STAYS in Validate (its refusals name themselves).

### A-9. `internal/engine/venue/order.go`

- `OrderRequest` gains `Hold *ratelimit.Hold` (doc: threading seam,
  §68b.3; nil = single-order mode).
- `OrderClientConfig` gains `Governor *ratelimit.Governor`, REQUIRED
  (`ErrOrderGovernorUnset`); `NewOrderClient` refuses any family
  transport where `!client.UsesGovernor(cfg.Governor)`
  (`ErrOrderGovernorMismatch`, naming the family) — §68b.2's
  prose-to-refusal conversion, doc quoting the config comment it
  replaces.
- `Place`: after `EncodePlace`, set `call.Hold = r.Hold` — the CODEC
  does not carry the hold; the client sets it post-encode (state that in
  a comment: a codec that saw the hold would be a codec that could
  drop it).
- New `func (c *OrderClient) ReservePlace(reqs []OrderRequest) (*ratelimit.Hold, ratelimit.Decision, error)`:
  empty → error; for each request: `Validate`, venue match, family
  routed (the same three refusals Place makes, same sentinels); encode
  through the codec; `transport.Prepare` through the leg's family
  transport into a `ratelimit.Request`. Then ONE
  `cfg.Governor.Reserve(batch)`. Errors and refusing Decisions pass
  through. Requests' `Hold` fields are ignored here (doc: a reservation
  request carrying a hold is not meaningful; nothing reads it).

### A-10. `internal/oms/application/ports` — the reserver port

**`gateway.go`:** `Place(o domain.Order, u Urgency, hold *ratelimit.Hold) (Ack, error)`
— the [SPEC EDIT OWED] §4 signature; doc gains the §68b.3 nil contract.

**`reserver.go` (new):** `CapacityReserver` interface,
`ReservePlacements(orders []domain.Order, u Urgency) ([]*ratelimit.Hold, error)`
— one hold per leg, index-aligned; §68b.4's doc verbatim (per-leg shape
for §45's cross-venue future; today every element is the same pointer;
an error means nothing reserved, nothing sent). Segregated-capability
argument in the file doc, plus the deliberate NON-split of the wiring
carrier (§68b.4's round-4 paragraph, condensed).

### A-11. `internal/oms/application/dispatch.go`

- `Dispatch(id ids.ClientOrderID, expected int64, urgency ports.Urgency, hold *ratelimit.Hold) (Outcome, error)`
  — the hold is carried untouched to
  `d.placer.Place(o, urgency, hold)` (line ~398). Nil documented at the
  signature: acquire-at-send, the §6.7/resolution mode. No other logic
  changes; the resolver's and any single-order caller's call sites pass
  nil explicitly.

### A-12. `internal/oms/application/execute.go`

- `Executor` gains `reserver ports.CapacityReserver`;
  `NewExecutor(d *Dispatcher, filters map[...]domain.TradingFilters, reserver ports.CapacityReserver)`
  — nil reserver → `ErrReserverMissing` (errors.go).
- `preflight` returns `([]ports.OrderRecord, error)` — the records it
  already loads (§68b.4: reservation prices what preflight admitted, not
  a second read).
- `Execute`, between preflight and dispatcher construction:
  `orders := make([]domain.Order, ...)` from the returned records;
  `holds, err := e.reserver.ReservePlacements(orders, urgency)` — error
  → `Execution{}, err` (the err-means-nothing-attempted contract holds:
  nothing journaled, nothing occupied on a refusal).
  IMMEDIATELY on success, defer the blanket cancel: collect DISTINCT
  holds by pointer identity, `defer func(){ for _, h := range distinct {
  _ = h.Cancel() } }()` — armed before any fsync is possible (§68b.1's
  mandatory release; Cancel cannot touch consumed entries, so this is
  provably harmless to sent legs).
  Then the vector verification (§68b.4, round 3's exactness):
  `len(holds) != len(refs)` (short OR long) or any nil element →
  `Execution{}, ErrHoldVectorInvalid` naming got/want — before any
  dispatcher exists.
- The fan-out threads `holds[i]` into `d.Dispatch(ref.ID, ref.Version,
  urgency, holds[i])`.

### A-13. `internal/oms/infrastructure/driven/ordergateway.go`

- `VenueOrderPlacer` widens:
  `Place(ctx, venue.OrderRequest) (venue.OrderAck, error)` AND
  `ReservePlace([]venue.OrderRequest) (*ratelimit.Hold, ratelimit.Decision, error)`
  — §68b.4's one-provider ruling; the interface doc carries the round-4
  argument (an independent second carrier let reservation and placement
  land on different governors).
- `OrderGateway.Place` gains the hold parameter, threading into
  `venue.OrderRequest.Hold` in `toVenueOrder`'s caller (the translate
  step itself stays hold-blind; set it beside Urgency where the request
  is assembled).
- New `ReservePlacements(orders []domain.Order, u ports.Urgency) ([]*ratelimit.Hold, error)`:
  translate every leg through the existing `toVenueOrder`; ONE
  `client.ReservePlace(reqs)`; a non-nil error passes through; a
  refusing Decision maps to a new error
  (`ErrPackageCapacityRefused`) carrying venue, limit name and
  `NotBefore` in its message — an error is honest, nothing was sent
  (§68b.4). On admission: `holds := make([]*ratelimit.Hold, len(orders))`
  with every element the returned hold. LAST leg included — §68e's
  skip-last-leg mutant is aimed exactly here.
- `var _ ports.CapacityReserver = (*OrderGateway)(nil)` beside the
  existing assertion.

### A-14. `internal/oms/module.go`

- `Deps.VenueOrders` keeps its name and doc; its TYPE is the widened
  `driven.VenueOrderPlacer`. No new Deps field (§68b.4: the root wires
  ONE venue surface; reserve-here-place-there is inexpressible).
- `New`: `executor, err := application.NewExecutor(dispatcher,
  deps.InstrumentFilters, placer)` — the SAME gateway value, narrowed by
  the two port types the two use cases hold. Comment states the round-4
  narrowing in one sentence.

### A-15. `internal/app/reconcile/config.go`

- `VenueConfig` gains `SpendLockDir string` (`spend_lock_dir`), REQUIRED
  beside the credential refs it protects (`ErrConfigFieldUnset` when
  empty; same validation shape as the journal path). Doc: §68b.5b's
  carrier paragraph. The shipped example config gains the field
  (locate by the existing `api_key_ref` example).

### A-16. `internal/app/reconcile/wire.go`

- `newOrderClient` returns a named wiring result (round 8):
  `type venueWiring struct { client *venue.OrderClient;
  governor *ratelimit.Governor; spend *transport.SpendLock }`.
- Order inside: resolve credential → collaborators → profile → governor
  → `transport.AcquireSpendLock(cfg.Venue.SpendLockDir,
  parts.signer.SpendDomain(), cred)` → transports (each
  `SignedClientConfig` gains `Spend: spend` and the construction now
  runs §A-8's checks) → order client (config gains
  `Governor: governor`). EVERY failure after the acquire releases the
  token before returning, joining a close error (`errors.Join`) — §68b.5b's
  release-before-error; the deferred-close-on-success shape is
  FORBIDDEN here (the round-8 mutant), so the success return hands the
  token out in the result instead.

### A-17. `internal/app/reconcile/app.go`

- `Runner` gains `spend *transport.SpendLock`.
- `startOMS`: destructure the wiring result; store
  `r.spend = w.spend` beside `r.governor`. On the `oms.New` error arm
  (round 9): `errors.Join` a `r.spend.Close()` failure into
  `r.reconciliation.Cause`, nil the field, THEN report the gate failure
  — the halted-alive root keeps its diagnosis and the domain is free.
  On success the StopFunc becomes
  `func(context.Context) error { err := module.Close();
  if r.spend != nil { err = errors.Join(err, r.spend.Close()) };
  return err }` — module first, lock second, LIFO (§68b.5b).
- `newOrderClient`-error arm: unchanged (the helper released
  internally).

### A-18. `cmd/arblab/reconcile.go` (+ exercise.go via the shared writer)

- `writeRateLimitStatus` renders, for each limit with `Reserved > 0`
  (profile order, after the fold-miss lines):
  `ratelimit: <venue> <limit>: N unit(s) reserved for an in-flight
  package`. Zero-reserved output is byte-identical to today — the #187
  golden full-report test is the regression net, extended with one
  reserved-non-zero case.

### A-19. Evidence and campaign

§68e is the plan, unabridged: run the collapse checklist BEFORE
mutating; the named mutants are (1) per-call-site hold arguments
(`holds[i]` → nil, per site), (2) `holds[i]` → `holds[0]` recorded
EXPECTED-EQUIVALENT today (single venue, same pointer — record, do not
claim), (3) skip-last-leg in the driven adapter (killed only by the
composed real-adapter test), (4) `Consume` → `Acquire` in `Do`'s hold
branch (killed by exact-`Used` assertions), (5) `defer spend.Close()`
inside `newOrderClient` (killed by root fixture 1), (6) delete the
`oms.New` error-arm release (killed by root fixture 4), plus §68e's
boundary and wrap fixtures. `MUTATE_MODULE` unset; `MUTATE_TIMEOUT=180`
where the exercise suite is in scope. Marker grep zero at close — the
one [SPEC EDIT OWED] (§4 Place signature / §6 step 6) lands with the
docs below.

### A-20. Out of scope, recorded

No metric-export change (`LimitState.Reserved` is the §10 seam); no
`Decision` vocabulary change (§68d item 8); no multi-venue coordinator
(§68d item 4); no TTL (item 5); no in-flight charge ladder (item 10); no
governor-state persistence (item 11); no representative-wire header
validation beyond the Host dry-run (item 12 as amended). Docs owed at
close per §68e's inventory (v0.9.47).
