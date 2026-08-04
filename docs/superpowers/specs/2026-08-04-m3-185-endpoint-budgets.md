# M3 task #185 — Bybit per-endpoint budgets: ScopeEndpoint (§66)

**Date:** 2026-08-04 (session 96). **Task:** #185. **Status: DESIGN CONVERGED,
IMPLEMENTATION NOT STARTED.** This record is deliberately committed
mid-slice as the handoff artifact: the design survived six adversarial
second-model review rounds (arc below), the implementation spec is Appendix B,
and the next session implements from it. On close this record gains the
implementation findings, the mutation table, and the docs-version note, and
this WIP banner is replaced.

**Follow-ups already filed with full context:** #187 (fold-health: a mapped
Bybit response with missing/malformed rate-limit headers is silent — the
derivation this slice adds is what makes that condition detectable at all),
#188 (package-level budget reservation — legs acquire independently after
fan-out; pre-existing, sharpened by round 5's finding that the losing leg is
post-fsync StateSubmissionUnknown, not a clean unsent).

**Docs owed at close (not yet edited):** build-spec §7.6 endpoint-budget
clause (venue-level shared window, R1/R2/R3, boot roster+dimension+interval
checks, the min-capacity rule as normative operator arithmetic); §624's
"50/s per UID per endpoint" sentence and §626's "#185 owns that axis" tail
updated by reference; venue-profiles dated 2026-08-04 block (the verified
table: create 10/s inverse/linear/option and 20/s spot upgradable, all four
reads 50/s; the verbatim rolling-window and business-lines-independence
sentences; the batch consumption formula; the caveat that the page's "counted
separately for Spot, Linear, Inverse, and Options markets" sentence is about
WEBSOCKET CONNECTION limits and must not be cited for API budget separation;
the named assumption — per-category counting separation — recorded as a
LOOSENING trigger for #171, never a correctness prerequisite); adr-history
finding row; tutorial + ka parity if the tutorial's §7.6 passage is touched.

## §66a — the review arc (six rounds, three rebuttals, two tasks filed)

- **Round 1 (NO-SHIP):** (1) BLOCKER — the design relied on unverified
  per-category counter separation; fixed then (uniqueness rule), superseded in
  round 3 by the venue-level window. (2) A 100ms endpoint interval over-admits
  10x → Bybit CheckFold requires exactly 1s. (3) Declared-name substitution
  (execution_list call wired as order_realtime) survives R1/R3 → the endpoint
  name is now DERIVED from the path via a closed registry in the reader and
  Call.Limit is DELETED — the #186 one-carrier lesson; the wiring mistake
  class became inexpressible, and the "no roster in transport" non-goal was
  recognized as wrong (the dialect readers already ARE venue knowledge).
- **Round 2 (NO-SHIP):** (1a) ACCEPTED — Do read the body before folding
  headers, so a read failure discarded the reading that had already arrived
  (worst on a 429); the fold moves to immediately after client.Do. (1b)
  REBUTTED — "saturate the window on missing headers" contradicts the
  normative derive-nothing ruling (a guessed cap is a budget quietly
  rescaled); self-charging continues and the upward ratchet recovers on the
  next read; filed instead as #187. (2) ACCEPTED — endpoint limits needed the
  family-membership check (mooted by round 3's venue-level design). (3)
  Migration inventory extended (incl. priority_test's past-the-set scope
  fixture, which ScopeEndpoint turns valid — the collapse checklist's
  boundary-fixture tell).
- **Round 3 (NO-SHIP, the design-changing round):** BLOCKER — family-keyed
  endpoint windows + any single-ownership rule make a Bybit spot+perp package
  NON-ATOMIC (executor fans legs concurrently, no governor preflight: one leg
  sends, the sibling refuses deterministically — a naked leg). Fix adopted:
  **endpoint budgets are VENUE-level** — Family refused, keyed ("", name),
  one shared window across categories, conservative under EITHER reading of
  the venue's per-category question; the Bybit reader emits folds at family
  "" (the existing venue-keyspace Observe semantic). The uniqueness rule and
  the single-family carrier dissolve. MAJOR — request-time-only completeness
  strands §6.7 recovery (a profile missing order_history refuses the
  Safety-priority fallback at the recovery moment) → boot roster check at
  NewSignedClient via LimitReader.Endpoints(); also corrected a factual error
  (spot DOES call order_create; position_list is the surface with no spot
  cell). MAJOR — missing-header semantics → filed as #187, not absorbed.
- **Round 4 (NO-SHIP):** BLOCKER — concurrent-leg capacity race → REBUTTED as
  pre-existing (identical race on the venue envelope today; Acquire-per-leg
  is the shipped design; §6 one-leg-failure is the designed consumer), with
  the brief's "atomic" overclaim withdrawn and #188 filed. MAJOR — the boot
  roster check was dimension-blind (order_history declared as Orders passes,
  then R3 refuses the fallback at the recovery moment) → the registry carries
  {path, name, required dimension} and both CheckFold and the completeness
  check enforce it (create→Orders per the venue's batch consumption formula;
  reads→Requests).
- **Round 5 (NO-SHIP):** MAJOR — unequal per-category caps (create: 20/s spot
  vs 10/s linear) under one shared window → the MIN-capacity rule is
  normative, deliberately operator arithmetic (platform holds no venue
  numbers; the venue marks these caps upgradable per account, so a transport
  cap table would refuse legitimately upgraded accounts; precedent: "the
  profile that knows the capacity does the arithmetic once, in the open").
  MAJOR — REJECTED as spurious (a five-question review contract that never
  existed). MINOR — ACCEPTED: the round-4 rebuttal understated the OMS
  consequence — a governor-refused leg is post-fsync and lands
  StateSubmissionUnknown (dispatch.go classifyEffect maps non-venue
  NotApplied there), a durable authorized order needing resolution; #188
  carries the acceptance-test obligation.
- **Round 6 (final): one finding, PRE-EXISTING, confirmed in window.go —**
  freeAt's early return computes `need <= ceiling-used` in uint64 while
  `used > ceiling` is a state the package explicitly allows (the observe
  ratchet; admits' own doc says so): the subtraction wraps, a refused request
  reports NotBefore == now (RetryAfter 0 — "retry immediately", false), and
  worseThan can name the wrong worst limit. Reachable today on family
  windows; routine on 1s endpoint windows. Fix and four test cases are in
  Appendix B §5b. The existing freeAt table stops at used=60/ceiling=100 —
  the collapse checklist's pinned-below-the-boundary tell.

## §66b — the design (the brief as converged after round 6)

## Problem (found in #173's review, filed with full context)

Bybit meters **per endpoint, per UID, per second** (verified 2026-08-04 from
https://bybit-exchange.github.io/docs/v5/rate-limit):
- `POST /v5/order/create`: 10/s (inverse/linear/option), **20/s spot**, upgradable
- `GET /v5/order/realtime`, `/v5/order/history`, `/v5/execution/list`: 50/s, all categories
- `GET /v5/position/list`: 50/s (inverse/linear/option; no spot cell)
- Verbatim: "The API rate limit is based on the rolling time window per second and UID."
- Verbatim: "the configuration of business lines is independent of each other"
  (context: the trade-limit consumption note; the table itself has per-category columns)
- Verbatim batch formula: "Number of consumption = number of requests * number of
  orders included in a single request" (batch APIs; plain create consumes 1)
- IP envelope: "600 requests within a 5-second window per IP" → 403, 10-min ban.
- NOTE: the page's "counted separately for Spot, Linear, Inverse, and Options markets"
  sentence is about WEBSOCKET CONNECTION limits, not API budgets — must not be cited
  for budget separation. Per-category **counting** separation (spot call and linear
  call to the same endpoint decrement different counters) is an inference from the
  configuration-independence quote + per-category table columns. The design
  deliberately does NOT rely on it in either direction: the local endpoint
  window is venue-level and shared, conservative under both readings. #171's
  testnet measurement is recorded as a LOOSENING trigger (split per family for
  throughput), never a correctness prerequisite.

The adapters already carry per-endpoint fold names in `Call.Limit`
(`order_create`, `order_realtime`, `order_history`, `execution_list`,
`position_list`) because Bybit's `X-Bapi-Limit*` headers describe "current
endpoint" without naming it. But `ratelimit.Limit` has no endpoint axis:

- Declaring `position_list` as a family-scoped `DimensionRequests` limit means
  **every same-family call charges it** (`Governor.decide` walks every applicable
  window; `Cost.units(DimensionRequests)` is 1 for every acquire) — an order burst
  exhausts the local position_list count and refuses Safety-priority position
  reads during exactly the §7.1 incident that needs them.
- NOT declaring it means `Governor.Observe` refuses the fold (`ErrLimitUnknown`)
  and `SignedClient.observe()` drops the error by design — the venue's own
  consumption reading silently lost.

## Design

### 1. `Scope` gains a third member: `ScopeEndpoint`

One endpoint's own budget, VENUE-level — one shared window per endpoint across
product categories. **`Limit.Name` doubles as the endpoint discriminator** — the same string the adapter already carries in
`Call.Limit` for the header fold, so the fold address and the charge address are
one carrier (the #186 anchor-sharing lesson applied in advance: no second
derivation to drift).

Validation (in `Limit.Validate`):
- `Family` REFUSED — an endpoint budget is VENUE-level, one shared window per
  endpoint across product categories, keyed `("", name)` (round-3 blocker
  fix). Whether Bybit's per-endpoint UID counter is split per category is an
  unverified inference, and the two candidate models fail differently:
  family-keyed windows are correct if split but ADMIT 2x if shared (the ban
  direction), and with any single-ownership rule they make a Bybit spot+perp
  package NON-ATOMIC — the executor fans legs out concurrently with no
  governor preflight, so one leg sends while the other refuses at R1: a naked
  leg. One venue-level window is conservative under EITHER reading (shared:
  correct; split: slight under-admission, harmless at this system's call
  rates) and keeps every leg STRUCTURALLY ROUTABLE — no configuration can
  make one leg's calls deterministically unroutable while its sibling sends.
  What remains is the capacity race every shared window already has today
  (two legs acquiring concurrently while the envelope holds one unit): that
  is pre-existing and dynamic, and the losing side is refused pre-SEND
  (EffectNotApplied) but post-FSYNC — dispatch has already durably recorded
  DispatchAuthorized, and classifyEffect maps a non-venue NotApplied to
  StateSubmissionUnknown, so the losing leg is a durable authorized order
  the resolution machinery must resolve while its sibling may fill. That is
  the §6 one-leg-failure flow doing its designed job, not a silent naked
  leg — but it is not free, which is why the reservation task is filed
  rather than dismissed. A package-level reservation API (acquire all legs'
  budgets as one operation before fan-out) is filed as its own task rather
  than absorbed — it spans governors for cross-venue packages and belongs to
  the §6 executor's layer, not to the budget axis. The fold reaches it because the Bybit
  reader emits observations at family "" — the EXISTING documented address
  for venue-scoped windows. #171's measurement may justify splitting per
  family later (a recorded loosening, not a fix); the rejected alternative —
  enforce single-family Bybit configs until #171 — is recorded as rejected
  because the single-family claim would need its own carrier and blocks the
  supported package shape for a throughput win nothing needs.
- Ceilings under the shared window (round-5 major): the venue's caps are
  UNEQUAL per category on some endpoints (create: 20/s spot vs 10/s linear),
  so a shared window is conservative only if its ceilings are the MINIMUM
  across the categories the profile enables — 20 would admit a linear burst
  twice the venue's meter. That rule is NORMATIVE in §7.6 and stated on
  LimitConfig, and deliberately NOT machine-enforced: platform holds no venue
  numbers by design, and a transport-side cap table would refuse legitimately
  upgraded accounts (the venue's own table marks create "upgradable"). This
  is the same trust already extended to every declared capacity in the
  system; the rejected alternative — per-category capacities in config with
  a code-derived min — is recorded in the decision record (it moves the
  transcription, not the trust, and the repo's precedent is "the profile
  that knows the capacity does the arithmetic once, in the open").
- `Dimension` must be `DimensionRequests` or `DimensionOrders`.
  `DimensionWeight` refused: no venue meters weight per endpoint; default-deny
  until a venue fact demands it.
  - Why Orders is needed: Bybit's create cost is `Cost{Orders: 1}` and the venue's
    own consumption formula for order endpoints counts ORDERS (batch = N per
    request). `order_create`'s budget declared as `DimensionOrders` charges
    `units = Cost.Orders` — 1 for a plain create, N for a future batch — with the
    existing `units()` switch, no special formula.
  - Read endpoints declare `DimensionRequests` (units = 1 per call).

Profile rules:
- Endpoint limits do NOT satisfy the venue request envelope and do NOT count
  toward the family weight/orders coverage rule (unchanged from #173: coverage is
  family-scoped-or-waived; a Bybit profile waives weight AND orders per family
  with written reasons — "no weight budget exists; orders are metered per
  endpoint and order_create's endpoint budget is the counter").
- Duplicate key: `limitKey{family, name}` unchanged; endpoint limits live in
  the empty-family keyspace, so an endpoint budget and the venue envelope
  cannot share a name, and a family limit MAY reuse an endpoint's name only in
  its own family keyspace — which Bybit's CheckFold then refuses as squatting
  (below), and which on any dialect never receives that endpoint's folds (the
  Bybit reader emits endpoint folds at family "").
- (Round-2 major-2 — family membership — is MOOT under the venue-level
  design: an endpoint limit names no family at all.)

### 2. `ratelimit.Request` gains `Endpoint string`; transport DERIVES it — `Call.Limit` is deleted (round-1 major-2 fix)

`LimitReader` gains `Endpoint(path string) string`: the dialect's closed
path→endpoint-name mapping. Bybit's reader holds the five-entry map
(`/v5/order/create` → `order_create`, `/v5/order/realtime` → `order_realtime`,
`/v5/order/history` → `order_history`, `/v5/execution/list` →
`execution_list`, `/v5/position/list` → `position_list`); Binance's returns ""
for every path. `transport.Do` sets `req.Endpoint` from it, and
`BybitLimitReader.Observations` attributes the fold to the SAME derived name —
`Call.Limit` and the venue-package roster constants are deleted, so an adapter
cannot wire the wrong name to a path: the substitution class is inexpressible,
not checked. (Venue knowledge in transport is the established pattern: the
readers and envelopes already hold header names and retCode semantics.) An
unmapped Bybit path derives "" and is refused pre-send by R2 — a new adapter
endpoint added without extending the map fails loudly on first use.

### 3. Two request-time refusals in `Governor.decide` (both errors, pre-send, no waiting repairs them — the `ErrFamilyUnknown` class)

- **R1**: `r.Endpoint != ""` and no `ScopeEndpoint` window is declared under
  that name (venue-level lookup, family-blind) → refuse
  (`ErrEndpointUnbudgeted`). Closes the
  fold-loss arm: a call either has its declared window (fold lands) or is never
  sent (nothing to lose). Strict and venue-independent: scoping it to
  profiles-with-endpoints would leave today's only expressible Bybit profile
  (zero endpoint budgets) silently lossy — the exact defect this task exists to
  kill.
- **R2**: `r.Endpoint == ""` and the profile declares ≥1 `ScopeEndpoint` limit
  (venue-wide predicate, not per-family) → refuse (`ErrEndpointUnnamed`). Closes
  the unmapped-path hazard: on a venue that meters per endpoint, a call whose
  path the reader cannot map would otherwise be metered venue-side and
  uncounted locally. Venue-wide because "this profile has gone per-endpoint" is
  a dialect fact; per-family would silently admit the unmapped call in the
  family whose budgets weren't declared yet.
- **R3**: `r.Endpoint != ""` and the matched endpoint window would receive ZERO
  units from this cost (e.g. a RequestOnly read matched to an Orders-dimension
  endpoint budget) → refuse. `window.admits` documents that a zero charge is
  always admitted, so without R3 the local gate never refuses while the venue
  meters 50/s — a declared budget that cannot do its job, silently.

### 4. Charging and the cannot-count interaction

- `New` builds `endpoints map[family]map[name]*window` beside `applicable`;
  endpoint windows are excluded from `applicable` (being in it is exactly the
  cross-charging bug).
- `decide`: the matched endpoint window joins the admit/refuse walk (worst
  `freeAt` naming, same as any window) and the charge walk, with units by its
  own declared dimension.
- **#173's cannot-count check is extended per-request**: dimension d is counted
  if some applicable family/venue window counts d OR the matched endpoint
  window's dimension is d. Without this, every Bybit create
  (`Cost{Orders: 1}`) is refused by the rule that was meant to protect it —
  there is no family-wide orders budget on Bybit to declare, and inventing one
  fabricates a budget the venue does not meter. A create matched to an
  orders-dimension `order_create` window is truthfully counted; the same create
  matched to a requests-dimension window (operator mis-declared) is still
  refused (waiver never suppresses; names the dimension; operator fixes the
  declaration).

### 5. `Observe` unchanged; the fold MOVES EARLIER in `Do` (round-2 major-1a fix)

Endpoint windows are registered in `g.windows` under `(family, name)`; the fold
lands. `BybitLimitReader.Observations` attributes to the derived endpoint name.

`SignedClient.Do` currently reads the response body BEFORE folding headers, so
a body-read failure or an oversized body discards the reading that already
arrived — including on a 429, where it matters most. The fold moves to
immediately after `client.Do` returns (headers are complete at that point),
before any body handling. Classification and penalization still require the
body and keep their positions: a Bybit refusal lives in the body, so no penalty
is derivable from a response whose body was lost — but the header fold IS
derivable, and now survives.

REBUTTED (round-2 major-1b): "conservatively saturate or hold the endpoint
window when expected Bybit headers are absent/malformed". This contradicts a
normative ruling the spec already made deliberately (§7.6 + venue-profiles §6,
banked 2026-08-02): with `X-Bapi-Limit` absent, NO observation may be derived —
a remaining count with no cap beside it cannot become a consumption figure, and
a guessed cap is a budget quietly rescaled. Saturating on a missing header
would refuse ALL traffic on that endpoint for the interval on what may be a
response shape that legitimately omits the header. The failure direction is
bounded: self-charging continues regardless (every call charges its window
pre-send), so what is lost on a headerless response is only the
external-consumer correction, and the upward-only ratchet recovers it on the
next successful read.

### 6. `CheckFold` gains the scope parameter

`CheckFold(scope ratelimit.Scope, name string, d ratelimit.Dimension, w time.Duration)`.
`NewSignedClient`'s walk now skips only `ScopeVenue` (envelope — requests-only,
unreachable by any fold) and passes both `ScopeFamily` and `ScopeEndpoint`.

- **Binance**: `ScopeEndpoint` → refuse always. This dialect derives fold names
  from headers; its `Endpoint()` derives "" for every path, so an endpoint
  budget can neither be charged nor folded — dead config that reads as
  coverage. `ScopeFamily` behaviour unchanged.
- **Bybit** (its first real CheckFold body — the home #186 left waiting):
  - `ScopeEndpoint`: the name must be in the reader's closed endpoint map
    (a typo'd endpoint budget is dead config AND the real budget missing —
    refused at boot), and the interval must be EXACTLY 1s (round-1 major-1
    fix: the venue's published per-endpoint windows are all `/s`; a shorter
    local window over-admits multiplicatively — 10-per-100ms admits 100/s
    against a 10/s meter — and header folds cannot repair it because
    observations age out on the same wrong interval).
  - `ScopeFamily`: the name must NOT be in the endpoint map — a family-scoped
    limit squatting on a roster name would capture the endpoint's folds while
    being charged by every family call; now refused at boot rather than only
    argued about at request time.
  - Boot COMPLETENESS (round-3 major fix, round-4 dimension fix): the closed
    registry carries `{path, name, required dimension}` — order_create is
    DimensionOrders (the venue's consumption formula counts orders carried),
    the four reads are DimensionRequests. `LimitReader` exposes it as
    `Endpoints()` (Binance: nil), and `NewSignedClient` requires every roster
    name to be declared as a `ScopeEndpoint` budget OF THE REGISTRY'S
    DIMENSION; Bybit's CheckFold enforces the same dimension per declared
    endpoint limit. Without the dimension check, order_history declared as
    Orders passes a name-only roster check and then R3 refuses the
    Safety-priority history fallback at exactly the §6.7 recovery moment —
    recreating the stranding the roster check exists to kill; the mirror
    (order_create as Requests) would fail the cannot-count check on first
    use instead of at boot. R1/R2/R3 remain as defense in depth. Request-time-only completeness strands RECOVERY:
    a profile missing `order_history` boots and trades, then R1 refuses the
    Safety-priority realtime-to-history fallback exactly when §6.7 needs it
    to resolve an ambiguous order — fail-closed at the wrong time, with new
    trading held for as long as the order stays unresolved. Under the
    venue-level design the roster is family-blind, so all five are required;
    a budget for an endpoint a run never calls costs nothing. (The earlier
    rationale also misstated a venue fact: spot DOES call order_create; it is
    position_list that has no spot surface.) R1/R2 remain as defense in
    depth.

### 7. Config (`internal/app/reconcile/config.go`)

`parseScope` gains `"endpoint"`. No new config field — the limit row's existing
`name` is the discriminator.

### 8. Spec/docs

- build-spec §7.6: endpoint-scoped budget clause + R1/R2.
- venue-profiles: dated 2026-08-04 block with the table facts above, the batch
  formula, the WS-vs-API separation caveat, and the named assumption
  (per-category counting separation → #171).
- Decision record §66; adr-history finding row; tutorial + ka parity if touched.

## Known migration cost inside the repo

- `Call.Limit` deleted: the five venue call sites drop their `Limit:` lines and
  the roster constants move into the Bybit reader's closed map (exported name
  constants stay available in transport for profiles/fixtures). transport's
  `testCall()` fixture loses its stray `Limit: "weight_1m"` (set on a
  Binance-shaped call the reader ignores today).
- Every existing test that drives a Bybit call through `SignedClient.Do` must
  add endpoint budgets to its profile — R1 bites them all (that is the
  enforcement demonstrating itself). `order_test.go`'s `testGovernor` currently
  hands Bybit a Binance-shaped profile; it gains a Bybit-shaped variant:
  envelope + per-family weight/orders `Uncounted` waivers + the endpoint
  budgets for the family under test.
- The #173 composed test extends to the full seam: Acquire (endpoint charge) →
  send → observe fold ratcheting the SAME endpoint window.
- (round-2 minor) Direct codec-level assertions on `call.Limit` that never pass
  through `Do` — order_test.go's EncodePlace assertions, positions_test.go,
  orderquery_test.go's route assertions — migrate to asserting the reader's
  derived name per path (pinning all five path→name tuples exactly, not merely
  distinct). priority_test's "past the set" scope fixture is `ScopeFamily + 1`,
  which `ScopeEndpoint` turns VALID — it moves to the new max+1. CheckFold
  tests gain the scope argument; serialized-config fixtures gain nothing
  (scope "endpoint" is additive) but the scope-string parser test grows the
  third value.

## Round-3 dispositions carried into this revision

- Blocker (non-atomic two-family packages): FIXED by the venue-level window
  (section 1); the uniqueness rule and the single-family carrier are gone.
- Major (request-time-only completeness strands §6.7 recovery): FIXED by the
  boot-time roster check (`LimitReader.Endpoints()`, section 6).
- Major (fold health on missing/malformed headers): FILED as a follow-up task
  rather than absorbed — holding an endpoint on a missing header contradicts
  the spec's explicit derive-nothing ruling and rests on a shared-UID premise
  that does not exist before #171; this slice's derivation change is what
  makes "mapped call, headerless response" detectable at all, and the filed
  task cites that seam. R1/R2/R3 + self-charging remain the primary mechanism;
  the fold stays the correction channel.

## Round-4 dispositions carried into this revision

- Blocker (concurrent-leg capacity race): REBUTTED as pre-existing with the
  overclaim corrected. The race exists today on the venue envelope and every
  family window — Acquire-per-leg is the shipped design, and §6's
  one-leg-failure flow is its designed consumer; the losing leg's refusal is
  pre-send. Round 3's blocker was the DETERMINISTIC structural refusal, which
  the venue-level window removes. "Atomic" is withdrawn from the brief;
  package-level reservation is FILED as its own task (spans governors for
  cross-venue packages; §6-executor layer).
- Major (dimension-blind completeness): FIXED — the registry carries the
  required dimension and both the roster check and CheckFold enforce it.

## Round-5 dispositions carried into this revision

- Major (unequal per-category caps under the shared window): the
  minimum-capacity rule is now normative (§7.6 + LimitConfig doc);
  machine enforcement declined with the argument recorded (no venue numbers
  in platform; upgradable accounts falsify any hardcoded cap table).
- Major ("round-5 questions 3–5 missing"): REJECTED — no five-question
  contract existed; the artifact's question list is its own.
- Minor (SubmissionUnknown, not clean-unsent): ACCEPTED — the rebuttal text
  now states the post-fsync consequence accurately, and task #188 carries it.

## Round-6 question (final round)

Any remaining BLOCKER-severity finding that changes the architecture?
Sub-major polish belongs to implementation review (a code_diff round follows
the build).

## Appendix B — the implementation spec (hand to the implementing agent verbatim)

Read the finalized design first:
/private/tmp/claude-501/-Users-sofo-Projects-misc-arb-lab/dc8c78f3-eee0-4b5d-9276-6c59243064c3/scratchpad/185-design-brief.md
It is authoritative on WHY. This file is the file-by-file WHAT. Match the
repo's comment voice (argued doc comments; state the constraint the code
cannot show; no change-log narration). CLAUDE.md's rules bind: no float64, no
time.Now, exhaustive switches (a switch over Scope must name every member —
no default), zero values unusable.

## 1. internal/platform/ratelimit/priority.go

- Add `ScopeEndpoint` after `ScopeFamily`: one endpoint's own budget,
  VENUE-level — one shared window per endpoint across product categories,
  because whether the venue splits its endpoint meter per category is a venue
  fact this package must not assume, and a shared local window is conservative
  under either answer. The limit's Name doubles as the endpoint discriminator
  a Request must match to charge it.
- `maxScope = ScopeEndpoint`. Extend `Scope.String`.

## 2. internal/platform/ratelimit/errors.go — new sentinels, existing voice

- `ErrLimitEndpointDimension` — endpoint-scoped limit counting weight. No venue
  meters weight per endpoint; default-deny until a venue fact demands it.
- `ErrEndpointUnbudgeted` — a request names an endpoint no ScopeEndpoint
  budget covers (profile-versus-adapter disagreement; the ErrFamilyUnknown
  class, no waiting repairs it).
- `ErrEndpointUnnamed` — a request names no endpoint against a profile that
  budgets per endpoint.
- `ErrCostEndpointUncounted` — the matched endpoint budget would receive zero
  units from this cost (an orders-dimension budget under a request-only cost):
  the declared budget cannot count this call, and a window charged zero never
  refuses (window.admits admits every zero), so the disagreement is refused
  instead.

## 3. internal/platform/ratelimit/limit.go

- `Limit.Name` doc: add the ScopeEndpoint role (fold address AND charge
  discriminator; the Bybit dialect derives it from the call path). Remove the
  stale "until #185" sentence.
- `validateFamily`: `ScopeEndpoint` behaves as `ScopeVenue` — a named family
  is REFUSED (`ErrLimitFamily`), with a doc sentence carrying the argument:
  the endpoint meter's per-category split is unverified, a shared window is
  conservative under either reading, and a family-keyed endpoint budget makes
  a two-family package non-atomic (one leg sends, the other refuses locally —
  a naked leg). Keep the case-bool style exhaustive.
- `Validate`: after the venue-scope/dimension rule add: endpoint scope refuses
  `DimensionWeight` (`ErrLimitEndpointDimension`). Requests and Orders are the
  two shapes a per-endpoint meter takes (Bybit: reads are requests/s; order
  endpoints consume per ORDER carried — batch formula, Profiles §7.6 block).

## 4. internal/platform/ratelimit/profile.go

In `Validate`'s limit walk:
- ScopeEndpoint limits do NOT mark `counted[family][dim]`, do NOT satisfy the
  envelope (the envelope test stays `Scope == ScopeVenue && Dimension ==
  DimensionRequests`). They DO share the `seen` duplicate key space
  (unchanged key(): family "" + name — so an endpoint budget cannot collide
  with the envelope's name, and declaring one endpoint twice is
  `ErrLimitDuplicate`).

## 5. internal/platform/ratelimit/governor.go

- `Request` gains `Endpoint string` field with doc: the per-endpoint budget
  this call charges, derived by the transport dialect from the call path;
  empty for venues that do not meter per endpoint.
- Governor gains `endpoints map[string]*window` (venue-level, keyed by name)
  and `hasEndpoints bool`, built in `New` beside `applicable`. ScopeEndpoint
  windows are EXCLUDED from `applicable` (being walked by every family call is
  exactly the cross-charging defect; note the existing builder predicate
  `Scope == ScopeVenue || Family == f` already excludes them — keep it that
  way on purpose and say so) and from `counted`.
- `decide`, after the family lookup, before the cannot-count loop:
  - R2: `r.Endpoint == "" && g.hasEndpoints` → error `ErrEndpointUnnamed`
    naming venue+family.
  - R1: `r.Endpoint != ""`: `ep := g.endpoints[r.Endpoint]` (family-blind);
    nil → error `ErrEndpointUnbudgeted` naming venue and endpoint.
  - R3: `ep != nil && r.Cost.units(ep.limit.Dimension) == 0` → error
    `ErrCostEndpointUncounted` naming the limit and its dimension.
- Cannot-count loop gains the per-request extension:
  `if r.Cost.units(d) > 0 && !g.counted[r.Family][d] && !(ep != nil && ep.limit.Dimension == d)`
  — with a doc sentence: the matched endpoint window is the venue's own meter
  for its dimension (Bybit's order budget IS the create endpoint's budget), so
  a cost it counts is counted, and Profile-level coverage stays family-scoped.
- The admit/refuse walk and the charge walk append `ep` (when non-nil) to the
  applicable windows; refusal naming/freeAt logic identical.
- `Observe`: unchanged (endpoint windows are in g.windows).

## 5b. internal/platform/ratelimit/window.go — freeAt underflow (round-6 find, PRE-EXISTING)

`freeAt`'s early return computes `need <= ceiling-used` in uint64. `used >
ceiling` is a state this package EXPLICITLY allows (observe ratchets a venue
reading in above any ceiling — admits' own doc says so), and there the
subtraction wraps: a refused request gets `NotBefore == now`, Decision reports
RetryAfter 0 ("retry immediately" — false), and worseThan can name the wrong
worst limit. Reachable TODAY on family windows (weight_1m observed to 1500 >
OpeningCeiling 1400); endpoint windows make it routine (1s windows, folds near
ceiling). Fix: `if used <= ceiling-need { return now }` — safe because the
`need > ceiling` guard above it already bounds `ceiling-need`. Doc-comment the
wrap direction (the admits comparison two functions up is written backwards
for EXACTLY this reason — mirror its argument). Tests (window_internal_test):
(1) observation raises used above Capacity → freeAt returns a real future
instant; (2) Safety spending above the Routine ceiling → same for a Routine
request; (3) a 1s endpoint-style refusal returns the oldest-required expiry,
not now; (4) with multiple refusing windows the governor names the latest
release's limit. The existing table stops at used=60/ceiling=100 — the
collapse checklist's "fixture pinned below the boundary" tell, cite it.

## 6. internal/engine/venue/transport/signed.go

- `LimitReader` gains TWO methods over one closed registry:
  `Endpoint(path string) string` — the dialect's path→endpoint-budget-name
  mapping ("" = this venue does not meter this call per endpoint) — and
  `Endpoints() []EndpointBudget` — every endpoint budget the dialect requires
  a profile to declare (nil for Binance), where
  `type EndpointBudget struct { Name string; Dimension ratelimit.Dimension }`.
  Two views, one registry: they cannot disagree.
- `Call.Limit` field DELETED (its doc's content moves to the Bybit map).
- `Do`:
  - `req := ratelimit.Request{Family: call.Family, Priority: call.Priority,
    Cost: call.Cost, Endpoint: c.cfg.Limits.Endpoint(call.Path)}`.
  - MOVE `c.observe(call, resp.Header)` to immediately after
    `resp, err := c.client.Do(...)` succeeds and `defer resp.Body.Close()` —
    BEFORE `io.ReadAll`. Doc: headers are complete when Do returns; reading
    the body can fail, and a fold discarded on a read failure is discarded on
    exactly the 429 that carried it. Classification still needs the body and
    keeps its position.
- `BinanceLimitReader.Endpoint` returns "" always (headers name their own
  fold; endpoint budgets are refused for this dialect in CheckFold).
- `BybitLimitReader`: package-level
  `bybitEndpoints = map[string]string{ "/v5/order/create": BybitOrderCreateLimit,
  "/v5/order/realtime": BybitOrderRealtimeLimit, "/v5/order/history":
  BybitOrderHistoryLimit, "/v5/execution/list": BybitExecutionListLimit,
  "/v5/position/list": BybitPositionListLimit }` — the five exported name
  constants MOVE HERE from engine/venue (same identifiers, same strings;
  venue imports transport already). Carry over the "separate budgets per
  endpoint — the fallback must not spend the primary's allowance during a
  Bybit restart" argument into the map's doc.
  The registry rows carry the REQUIRED DIMENSION: order_create is
  DimensionOrders (the venue's batch formula counts orders carried; a
  Requests-dim create would under-charge a future batch and already fails
  the cannot-count check on first use), the four reads are DimensionRequests
  (an Orders-dim read budget is charged zero by RequestOnly and never
  refuses — R3's subject — and would strand §6.7's history fallback at the
  recovery moment). `Endpoint(path)` = registry lookup; `Endpoints()` =
  the rows, sorted by name.
  `Observations` uses the derived name instead of call.Limit and emits the
  observation at `Family: ""` — the venue-level keyspace where endpoint
  windows live (the EXISTING documented Observe semantic for an empty
  family). Unmapped path → nil observations, same as today's empty Limit.
- `CheckFold` signature becomes
  `CheckFold(scope ratelimit.Scope, name string, d ratelimit.Dimension, w time.Duration) error`
  on the interface and both readers.
  - `NewSignedClient` walk: switch over `l.Scope` naming every member —
    ScopeVenue skip (envelope, requests-only, unreachable by fold);
    ScopeUnspecified skip (unreachable: the governor validated the profile);
    ScopeFamily, ScopeEndpoint → CheckFold. AFTER the walk, the completeness
    check (round-3 major fix): for every row in `cfg.Limits.Endpoints()`,
    the snapshot must hold a ScopeEndpoint limit under that row's name AND
    with that row's dimension; a missing or dimension-mismatched one refuses
    construction (`ErrEndpointBudgetMissing`, new transport sentinel) naming
    the endpoint and, on mismatch, both dimensions. Rationale in the doc comment:
    request-time-only completeness strands recovery — a profile missing
    order_history boots and trades, then refuses the Safety-priority history
    fallback exactly when §6.7 needs it to resolve an ambiguous order. All
    five are required regardless of which families the run configures: a
    budget for an endpoint never called costs nothing, and the roster is
    venue-level like the windows.
  - Binance: ScopeEndpoint → refuse (`ErrLimitFoldUnaddressable` wrapped: this
    dialect derives fold names from headers and its calls name no endpoint —
    the budget could neither be charged nor folded). ScopeFamily arms
    unchanged. Scope switch exhaustive.
  - Bybit (first real body): ScopeEndpoint → name must be one of the five
    registry names (else refuse: a typo'd endpoint budget is dead config AND
    the real budget missing), `d` must equal the registry row's dimension
    (round-4 fix — order_history declared as Orders would pass a name-only
    check and then R3 refuses the Safety-priority fallback at the §6.7
    recovery moment), AND `w == time.Second` exactly (the venue's published
    per-endpoint windows are all /s; a shorter window over-admits
    multiplicatively and folds age out on the same wrong interval).
    ScopeFamily → name must NOT be one of the map values (a family budget
    squatting on an endpoint name would capture its folds while every family
    call charges it). Else accept. Scope switch exhaustive.

## 7. internal/engine/venue — call sites

- order_bybit.go: delete `Limit: BybitOrderCreateLimit` and the constant
  (moved to transport); keep Cost{Orders: 1} and its doc.
- orderquery_bybit.go: `bybitOrderStatusRoute` returns path only (drop the
  name half and the local constants — moved); executions call site drops
  `Limit:`; adjust docs (the metering argument lives in transport now).
- positions_bybit.go: drop `Limit:` and the constant (moved).

## 8. internal/app/reconcile/config.go

- `parseScope`: add `"endpoint"` → `ratelimit.ScopeEndpoint`; error text
  becomes (want "venue", "family" or "endpoint").
- LimitConfig.Name and .Scope doc comments: describe the third scope (scope
  "endpoint", product OMITTED — the window is venue-level and shared across
  categories); remove the stale "until #185" sentence. The Scope or ceilings
  doc MUST carry the min-capacity rule: where the venue publishes UNEQUAL
  per-category caps for one endpoint (Bybit create: 20/s spot, 10/s linear),
  the shared window's ceilings are stated from the MINIMUM across the
  categories the profile enables — the larger figure admits a burst the
  stricter category's meter refuses. Deliberately a documented operator rule,
  not code: platform holds no venue numbers, and the venue marks these caps
  upgradable per account.

## 9. Tests (write in the repo's table style; run the collapse checklist on
   every new fixture BEFORE mutating)

Platform (limit_test/profile_test/governor_test, black-box like existing):
- Validate: endpoint+weight refused (ErrLimitEndpointDimension);
  endpoint WITH a family refused (ErrLimitFamily — the inversion of the
  family-scope rule, so vary the fixture on scope with family held constant);
  endpoint name colliding with the venue envelope's name → ErrLimitDuplicate;
  the same endpoint declared twice → ErrLimitDuplicate; a family limit reusing
  an endpoint's name in a real family → LEGAL at the platform layer (assert
  it, with the comment pointing at Bybit CheckFold as the layer that refuses
  it); endpoint limits satisfy neither envelope nor family coverage (a
  profile with ONLY endpoint budgets + envelope still needs per-family
  weight/orders waivers).
- Scope table: ScopeEndpoint usable; past-the-set fixture moves to
  ScopeEndpoint + 1. String() case.
- Governor R1/R2/R3: each refusal is an error (not a Decision), names what its
  doc promises; R2 fires only when the profile HAS endpoint budgets (a
  Binance-shaped profile admits endpoint-less requests unchanged).
- Cross-charge regression (THE task's defect): TWO endpoint windows
  (distinct names, DIFFERENT capacities so a mis-landed charge is
  arithmetically visible), plus the envelope. Charge endpoint A to its
  ceiling: A refuses, B still admits, the envelope holds only the request
  counts. Assert the sibling's Snapshot Used is exactly 0.
- Cross-family SHARING (the round-3 ruling made executable): the same
  endpoint charged from TWO families accumulates in ONE window — charge via
  family 1, then a family-2 request against the same endpoint sees the
  consumption (refusal at the shared ceiling). This is the deliberate
  conservative merge; the test comment carries the naked-leg argument.
- Family calls (Endpoint "") on a profile WITHOUT endpoint budgets never touch
  endpoint state; with endpoint budgets they refuse (R2).
- Cannot-count extension: cost {Orders:1}, family waives orders, endpoint
  window Orders-dimension → admitted and charged 1; same cost, endpoint window
  Requests-dimension → ErrCostUncounted (waiver never suppresses); cost
  {Orders:5} charges 5 (the batch formula riding units()).
- R3: RequestOnly cost vs Orders-dimension endpoint window → refused.
- Observe folds into an endpoint window upward-only (ratchet visible in
  Snapshot).
- Priority: endpoint window enforces the ladder (Opening ceiling < capacity
  fixture; Safety reaches capacity).
- Refusal naming: endpoint window as the worst refusal names its Limit and a
  freeAt a retry can succeed at (interval 1s — advance the test clock).

Transport (signed_internal_test.go):
- Endpoint(): all FIVE path→name tuples pinned as literals both sides;
  unmapped path → ""; Binance always "".
- Bybit CheckFold: endpoint scope {roster name, registry dimension, 1s} OK;
  non-roster name refused; wrong dimension refused (order_history as Orders;
  order_create as Requests — BOTH directions); 999ms, 1001ms, 2s refused (1s
  boundary); family scope roster name refused; family scope non-roster name
  OK; venue-scope rows skipped by the walk (existing test extends).
- Completeness: profile missing one of the five → NewSignedClient refuses
  naming it; all five present but one with the wrong dimension → refused.
- Binance CheckFold: endpoint scope refused for any name incl. weight_1m.
- NewSignedClient isolating fixture: Bybit reader + endpoint budget at 2s →
  construction refused naming the limit.
- Do folds headers even when the body read fails mid-stream AND when the body
  exceeds ReadLimit (server sets X-Bapi-Limit headers + oversized body;
  governor Snapshot shows the ratchet; the call still returns its error).
- Composed (extends #173's seam test): Bybit read through Do against a real
  governor whose profile declares the endpoint budget: admitted, endpoint
  window charged 1, response headers ratchet the SAME window.
- Composed R1: Bybit call through Do, profile lacks the endpoint budget →
  Precondition error, zero requests reached the server (counter assertion).
- testCall() loses `Limit:`; add a Bybit-shaped profile/client helper
  (envelope + per-family weight/orders Uncounted waivers + endpoint budgets,
  1s intervals).

Venue (order_test.go, orderquery_test.go, positions_test.go):
- Codec assertions on call.Limit DELETED (Path assertions stay and now carry
  the routing claim; the name claim lives in transport's Endpoint() tuples).
- testGovernor: Bybit variant — envelope + per-family waivers (weight: "bybit
  publishes no per-endpoint weight", orders: "metered per endpoint;
  order_create's budget is the counter") + ALL FIVE endpoint budgets,
  venue-level (no family). All intervals 1s, capacities per the venue table
  (10 create as DimensionOrders, 50 each read as DimensionRequests), roomy
  ceilings below capacity. The completeness check makes all five mandatory.

Config (reconcile): parseScope "endpoint" row; a profile() round-trip with an
endpoint budget row (scope "endpoint", NO product field — a product on an
endpoint row must refuse via ErrLimitFamily surfacing through profile()).

## 10. Verification gates (run yourself, in order)

1. `make ci` — both modules, race. Everything compiles, all suites green.
2. `grep -rn "SPEC EDIT OWED" internal/ tools/` → must be 0 (docs are a
   separate step done by the orchestrator, not you — do NOT edit docs/).
3. Report: files changed, tests added/renamed/deleted, anything in the spec
   you deviated from and why (deviations are fine if argued; silent drift is
   not).

Do NOT touch docs/, do NOT commit. Scope is internal/ only.
