# M3 task #186 — boot-time fold-name agreement (§65)

**Date:** 2026-08-04 (session 95). **Task:** #186. **Docs:** v0.9.44.
**Core edit:** §7.6 gains the fold-addressability clause. **Follow-ups:** none
new; #185 unchanged (its axis is deliberately not pre-empted here).

`Governor.Observe` folds a venue-reported reading into a budget addressed by
`(family, name)` — no dimension, no interval. Two producers of those addresses
exist: `BinanceLimitReader` DERIVES the name from the header key
(`X-Mbx-Used-Weight-1M` → `weight_1m`, at the call's family);
`BybitLimitReader` uses the call's `Limit` constant. The observe seam drops
`Observe` errors by design (a mislabelled header must not fail a request the
venue already answered) and its comment has said "worth a boot-time check"
since slice C2a. So a family weight budget named `spot_weight` validates,
counts locally, and silently discards every venue reading — the governor runs
on exactly the local guessing §7.6's header-awareness exists to correct. The
worse shape: a REQUESTS budget named `weight_1m` would have the venue's weight
reading folded INTO it, corrupting the one budget whose local count is correct
by construction.

## §65a — what landed

1. `transport.LimitReader` gains `CheckFold(name, dimension, interval) error`
   — the derivation's inverse, beside `Observations` so both share one set of
   constants and one window parser/renderer and cannot drift.
2. `BinanceLimitReader.CheckFold`: a family weight/orders budget must bear
   EXACTLY the derived name (`weight_`/`orders_` + the canonical rendering of
   its own interval); a requests budget is refused only if its name is
   exactly derivable (round-trip through the shared parser+renderer), which
   is the set of names the canonicalizing runtime can mint — `weight_1m` is
   refused, `weight_guardrail` and `weight_60s` stay legal local names.
3. `BinanceLimitReader.Observations` CANONICALIZES: the header suffix is
   parsed to a duration and re-rendered through the same renderer CheckFold
   uses, so `60S`, `1M` and every equivalent spelling fold to `weight_1m`;
   an unparseable suffix derives no observation. For the spellings Binance
   actually sends (`1M`, `10S`, `1D`) nothing changes.
4. `BybitLimitReader.CheckFold` is deliberately vacuous, argument in its doc
   comment: its fold names are caller-supplied per endpoint from the venue
   package's roster (transport cannot see it — import direction), per-endpoint
   budgets are today inexpressible in the profile vocabulary (#185), and
   refusing non-roster names would refuse the only expressible local budgets.
   The method gets its real body with #185.
5. Enforcement in `NewSignedClient`: walk `Governor.Snapshot().Limits`,
   `CheckFold` every `ScopeFamily` entry, refuse construction on the first
   error (`ErrLimitFoldUnaddressable`, naming the limit and the derived
   name). The check runs where the reader/governor pair is FORMED — the same
   seam whose runtime half drops the errors — so no composition root can
   forget it; `wire.go` gained zero code. Venue-scoped entries are skipped:
   folds always carry a non-empty family and venue scope is keyed under the
   empty family (and is requests-only since §64d).

## §65b — the review reshaped it: six rounds, and the anchor rule twice

Second-model review (Codex), adversarial, against the design before any code.

- **Round 1 (accepted): the check and the runtime must share one anchor.**
  The v1 check rendered a canonical name from the limit's interval while the
  runtime preserved the header's own spelling — two functions that disagree
  the moment Binance spells one minute `60S`. A boot check certifying
  `weight_1m` while the reader minted `weight_60s` would pass while the fold
  still dropped. Fix: canonicalization in `Observations` through the SAME
  renderer (the memory rule "a recomputed check must share the anchor",
  found by the reviewer before the code existed).
- **Round 2 (accepted, both): a prefix-based collision rule falsely refuses
  non-derivable names** (`weight_guardrail` is not a window; refusing it
  turns a legal local guardrail into a boot outage) — the predicate became
  exact-derivability via round trip; **and the scope-filter mutation was
  unkillable via `ip_5s`** (it falls through the nil branch either way) —
  the isolating fixture is a venue envelope legally NAMED `weight_1m` beside
  a family weight budget of the same name (keys differ by empty vs
  non-empty family), which constructs, and stops constructing if the filter
  is dropped.
- **Rounds 3–4 (accepted): the fixture inventory was wrong twice.** The
  claim "all existing profiles conform" was generalized from the sites
  checked; the exercise chaos JSON, then the TYPED `LimitConfig` fixtures
  (exercise `validConfig`, reconcile `spotOnly` — hidden because the file's
  PRIMARY fixture was #173-renamed) all still declared
  `spot_weight`-family names. Three fixture families migrated (pure
  renames; every interval already matched). These are the third-through-
  fifth live instances of the defect class: every exercise run and every
  spot-only reconcile run was silently dropping every venue reading. Also
  round 4: the collision predicate's window-independence needed its own
  discriminating cases (`weight_1m`/requests/**5s** must refuse — Observe
  checks neither dimension nor interval), and the window parser needs
  CHECKED unit multiplication (2^55+1 seconds wraps to exactly 1s under
  naive `time.Duration(n) * time.Second` and would mint `weight_1s`).
- **Round 5 (one accepted, one REBUTTED and the rebuttal upheld in round
  6).** Accepted: `Limit.Name`'s doc ("ours to choose, not the venue's...
  maps onto whatever this profile calls it") and `LimitConfig.Name`'s echo
  became false the moment this landed — both corrected in the same commit,
  plus the telemetry note (names feed Snapshot and future §10 labels; no
  deployed consumer exists — verified, the exporter is still deferred).
  Rebutted: "canonicalization aliases duration-equal limiters" — the
  proposed spelling-tuple identity REINTRODUCES round 1's silent loss,
  which is the observed defect (five instances now), while the
  duration-equal pair appears in no verified roster and, if it ever
  occurred, merges CONSERVATIVELY under the upward-only fold (max wins,
  budget fills faster, refuses sooner; the status quo drops one header
  entirely). A dual-header concurrent test banks the merge behavior.
  Round 6 adjudicated the rebuttal on the merits and shipped.

## §65c — rejected alternatives (recorded so nobody reopens them)

- **Check in wire.go / per-dialect switch in the app layer:** duplicates the
  derivation outside the type that owns it; a future root forgets the call.
- **`FoldableNames(family) []string`:** Binance's producible set is
  unbounded (one name per window the venue chooses to report); the
  judgement is per-(dimension, interval), so an enumeration API cannot
  express it.
- **Bybit roster check via a new `venue.BybitLimitNames()`:** builds API
  #185 replaces, and would forbid legitimate local-only budgets.
- **Check inside `ratelimit.New`:** platform/ratelimit holds no venue
  knowledge by hard rule; the fold convention is venue knowledge.
- **Making observe() stop dropping errors:** rejected in C2a and #173 —
  a mislabelled header must not fail a mid-trade request. This slice is the
  other half of that bargain, kept at last.
- **Spelling-tuple fold identity** (round 5): see §65b.

## §65d — evidence

`make ci` green on both modules (verified locally after the second-model
implementation was manually reviewed file by file — the diff, the conventions
and the collapse-check comments all read; two stale comments the spec did not
cover were corrected by hand: the observe seam's "worth a boot-time check"
now says the check exists, and the reconcile fixture's "until #186 lands"
note now states the enforcement).

**13 targeted mutations: 12 kills + 1 diagnosed expected-equivalent.**
Aimed at: the weight prefix flipped (13 tests fail); the declared-vs-derived
comparison deleted (5); the requests-collision branch made unreachable (4);
the collision coupled to the limit's interval (2 — exactly the
window-independence rows built for it); the round trip degraded to prefix
matching (5 — `weight_guardrail` among them); the overflow bound loosened to
ignore the unit (2 — the 2^55+1 header and name rows); the ScopeFamily
filter dropped (1 — exactly the shared-name venue-envelope fixture, the
round-2 isolating case); Bybit's vacuity replaced with a refusal (3, run
against `./internal/engine/venue/...` so the real Bybit client construction
objects — the positive control that the vacuity is load-bearing); the
check's interval swapped for a constant (5, killed by the venue suite's
`orders_10s`/10s profile); runtime canonicalization reverted to the verbatim
spelling (3); the renderer's input pinned to one minute (3); the fold-check
error swallowed at construction (1). Three `NOBUILD!!` verdicts along the
way were all bad seds ("declared and not used" / "imported and not used" —
the discriminator from §63g's family), each re-aimed to a compiling mutant
and killed; none stands in for a claim.

**The expected-equivalent** (the `Reserve()` precedent): the parser's
`n == 0` refusal survived deletion because both consumers re-render the
parsed window through `binanceWindow`, which refuses a zero window one step
later on every path — no observable difference exists through any exported
surface. Kept, because the guard is the parser holding its own contract ("a
parsed window is a positive renderable duration") rather than leaning on the
renderer's refusal; deleting a fail-closed guard to satisfy a mutation score
would invert the tool's purpose.

Marker grep (`grep -rn "SPEC EDIT OWED" internal/ tools/`): **0** — the §7.6
clause landed directly, same session. Docs at v0.9.44 (build-spec,
adr-history, tutorial + `tutorial.ka.md` regenerated, parity **16/16**;
venue-profiles and state-machines untouched — no venue fact and no machine
changed). Fixture migrations verified by the exercise and reconcile suites
running under `make ci`'s race tests, `TestProcessKillMidIOC` reaching
dispatch through the new check with real wiring.
