# Design record §63 — M3 task #174: persist `Ack.VenueOrderID`, and gate 3 learns to rule out

**Date:** 2026-08-04 (session 93). **Docs:** v0.9.41 → v0.9.42. **Task:** #174
closes. **Evidence:** `make ci` green both modules, 19 mutations / 19 kills,
marker grep 0.

Slice D priced its own refusal honestly (record §53): on Binance ANY execution
row inside the sweep window made an absence `sweep_inconclusive` — a permanent
refusal, `RetryMayLift() == false`, boot `OutcomeNeedsOperator` — because no
Binance trade list carries a client order id and the venue `orderId` is the
only join key. §6 step 6 sends parallel marketable IOCs on two legs, so a
sibling's fill inside the window is the ordinary case. This slice lands the
lever slice D named: the id the Place ack already carried is persisted at
dispatch, and the absent path rules rows out against the instrument's
persisted set.

## §63a — what landed, in one pass

- `ports.Journal.AppendAcked` + `ports.JournalVisitor.Acked` + the `acked`
  record kind (`kindTagAcked = 4`, table `journal_acked`, append-only
  triggers, LEFT JOIN in the replay query). `chainLabel` stays `/v1`: a new
  kind changes no existing preimage; an older build reading a newer journal
  refuses at the kind column, the documented downgrade direction.
- `ports.OrderRecord.VenueOrderID`, produced only by
  `ports.OrderWriteRepository.ApplyAck`: refuses an empty id, a `Planned`
  order (`MayExistAtVenue` false), and — `ErrVenueOrderIDConflict` — an id
  contradicting the store; same-value re-record is a no-op with the version
  left alone (replay meets duplicates legitimately).
- `ports.VenueOrderIndex` (`VenueOrderIDsOn(instrument)`), a narrow read port
  on the `HedgeGroupOrders` precedent, served by the same memory store and
  wired to the Resolver as a second capability of one object.
- `attribute` gains the map and two arms: a row whose id maps to THIS order is
  ours (the absent path's only claim, since the venue that denies the order
  supplies no live id); a row whose id maps to ANOTHER order is ruled out —
  #174's arm. Unmapped ids still refuse. Ours-wins-over-unknown ordering
  unchanged.
- Dispatch: on a non-empty `Ack.VenueOrderID`, `AppendAcked` BEFORE
  `AppendState`, `ApplyAck` before `ApplyState`; `Outcome` gains
  `AckJournaled`, `AckApplied`, `AckConflict`; `HoldsNewTrading` is now
  `!Applied || AckConflict`. Replay's `rebuild` gains `Acked`.
- The Joiner refuses a non-empty store-vs-live id disagreement
  (`ErrVenueOrderIDMismatch`) before attributing anything, and attributes by
  whichever source is non-empty past that check. It does NOT take the index.

## §63b — the unbanked premise, and why the review was right to block on it

The existing different-id arm ("the venue named a different order of its own →
ruled out") needs only at-query-time uniqueness: the venue told us our id in
the same response. The new arm compares a row against an id persisted EARLIER
— it needs non-reuse ACROSS TIME, or a recycled number could belong to the
very order being resolved, and the rule-out would prove a false
`AbsentAfterFinality`, a terminal, journaled. The two premises are `==` on a
string in code; only the provenance of the right-hand side differs. Profiles
§8 held the join-key fact and nothing stronger — the Codex review's blocker,
sustained.

Both statements were then found on primary sources and banked verbatim
(Profiles §8, dated 2026-08-04): USDⓈ-M Query Order — *"orderId is
self-increment for each specific symbol"* (a self-incrementing counter is a
non-reuse statement); SPOT glossary — *"uniquely identifies the order on a
symbol"*. Per SYMBOL, hence: every consumer scopes to the full
`VenueInstrument`, the index is per-instrument, `ApplyAck`'s cross-order check
compares instruments, and the resolver test's scope fixture puts the same
number on the other leg's book and asserts it neither blocks nor claims.

**Rejected alternative, kept on record:** resolving unknown rows by querying
the order BY `orderId` (`GET /fapi/v1/order` accepts it) would answer
authoritatively at query time and need no non-reuse premise — rejected for
this slice because it spends §7.6 budget per unknown row on a path that must
reserve budget for recovery, and adds a venue surface #174 does not need now
that the premise is banked. It remains the hardening if the banked fact is
ever doubted.

## §63c — the conflict design: two halves that compose

A venue handing two of our orders on one instrument the same id (the banked
fact failing, or an adapter mis-threading acks) must never leave a poisoned
index consultable. The first draft refused the second `ApplyAck` and moved on
— which retains the FIRST owner in the index, and rule 6 then rules the second
order's own fills out against it (Codex's blocker sub-point: the unsafe
COMPOSITION of two individually-correct refusals).

Landed design: live, `Outcome.AckConflict` folds into `HoldsNewTrading`, so
the executor's fan-in files the same latch it files for a stale projection —
no resolution runs after it in that process. Crashed, the journal — a
recorder, deliberately conflict-blind — holds both acked records, and the
replay's `ApplyAck` refuses the second: a log that VERIFIES and does not
COMPOSE, `OutcomeFailed` (not `EvidenceUnusable` — the chain is intact, and
`ChainBroken` answering false on it is asserted). Every path to a resolution
therefore passes through either the latch or the replay refusal.
`TestAnAckConflictLatchesLiveAndRefusesTheNextBoot` drives the whole
composition against the real journal and store.

The ordinary dropped write is deliberately NOT a hold: losing the ack record
degrades gate 3 to the pre-#174 refusal — safe and late, the resolver's
designed failure direction — and holding trading on a store hiccup while the
journal took the record would be a halt with nothing wrong. Reported per leg
(`ack_journaled`/`ack_applied` on the CLI line, `ExitExecution` in the exit
class) because a store dropping writes threatens the next order.

## §63d — the Joiner stays out of the index

Two authorities can now name one order's venue id: the persisted ack and the
live query. Handing the join the index would let rule 3 attribute rows by the
persisted id while rule 2 attributes by the live one — fills from two venue
orders landing on one client order, with the cumulative cross-check
arithmetically satisfiable while wrong (Codex major, accepted). The join
instead REFUSES a non-empty disagreement before any attribution — under the
banked fact there is no innocent reading — and past that check the two sources
agree or one is silent, so it attributes by whichever is non-empty. That
fallback is real behaviour, not tidiness: a surface answering without an id
leaves the persisted ack as the only key that can claim Binance-shaped rows,
and `TestJoinAttributesByThePersistedIdWhenTheSurfaceReturnsNone` pins it.

## §63e — ack-before-state, argued from the crash window

Between `Place` returning and the state record landing there is a window a
SIGKILL can hit. Ack-first: the journal holds `authorized`+`acked`, which
replays into an order at the crash boundary WITH the one join key a Binance
sweep can use. State-first: `authorized`+`state`, the pre-#174 world — and the
ack, unlike the state, is not re-derivable (the resolving query supplies a
state; a lost ack is what §6.7 runs for). Codex confirmed; the ordering is
asserted as an interleaving fact from the fake's single record list
(`ackedBeforeState`) and against the real store's journal rows in
`module_test`.

## §63f — reporting shape: three fields, meaningful-only-when

`AckJournaled`/`AckApplied` mirror `Journaled`/`Applied` one record over, and
are meaningful only when `Ack.VenueOrderID` is non-empty — the gating fact is
a sibling field in the same struct, which is what keeps the
meaningful-only-when honest. `AckConflict` is separate from `AckApplied`
because it is a different KIND of false: a failure loses a reading, a conflict
gains a finding. The consumer landed with the fields (the
schema-field-frozen-before-its-consumer lesson): the exercise CLI prints the
pair only when an id existed (an id-less leg printing `ack_journaled=false`
would read as a failure that never had a write to fail), and `ExitClass` folds
the pair in on `Journaled`'s own terms.

## §63g — evidence

19 mutations, 19 kills: the two `attribute` arms, the index-error refusal, the
two dispatch skip-guards, the `HoldsNewTrading` disjunct, the conflict
reporting, the replay duplicate count and error propagation, the join mismatch
and fallback, the repository's four `ApplyAck` refusals and the index's
instrument scope, the journal's empty-id refusal, the codec tag, the visitor
dispatch, and the exercise exit predicate. Two re-aims, both recorded: an
orphaned-variable `NOBUILD!!` (re-aimed to the always-false form that still
reads the variable), and a NEW harness lesson — `MUTATE_TIMEOUT=180` without
the unit parses as 180 nanoseconds, `go test` refuses the flag, zero `--- FAIL`
lines, and the harness prints `NOBUILD!!` with an EMPTY compiler-error block.
An empty diagnostic block under a `NOBUILD!!` means the runner never reached
the compiler; reproduce by hand before re-aiming anything.

Collapse checklist, run before the battery: the scope fixture (same number,
other instrument) is the one that separates instrument-keyed from
globally-keyed in the direction that would prove a false absence; the
tag-collision fixture uses a venue id that SPELLS a state name, because a
numeric id differs from every state name and would pass with the tags deleted;
the exercise exit predicate has its positive control (both writes landed →
`ExitClean`) so the two failure cases cannot pass against
fail-everything-acked; `TestChainVerifiesAtEveryLength` re-runs its
all-kinds-interleave dimension at four kinds with the cycle completing twice.
