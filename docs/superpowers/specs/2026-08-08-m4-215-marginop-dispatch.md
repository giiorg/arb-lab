# §84 — task #215: the MarginOp DISPATCH path (2026-08-08, session 127b)

**STATUS: the CODE half is landed and committed; the DOCS half is owed.** This
record was written at the code commit so that a later session picks up the
rulings rather than re-deriving them. Section 3 is the remaining work list, and
it is the first thing to read.

Commit `2d4cfbe` (branch `feat/milestone-1-foundation`). `make ci` green both
modules, working tree clean, and the one open `[SPEC EDIT OWED]` marker is
deliberate — it names the §4 signature change the docs commit lands.

## 1. What landed


| File | What |
|---|---|
| `internal/engine/venue/marginwrite.go` | `MarginWriteCodec`, `OrderClient.AddIsolatedMargin`, `checkMarginWriteAmount` |
| `internal/engine/venue/marginwrite_binance.go` | `POST /fapi/v1/positionMargin {type:1}`, weight 20 (unverified), `{amount,type}` echo check |
| `internal/engine/venue/marginwrite_bybit.go` | `POST /v5/position/add-margin`, `positionIdx:0` sent + echoed, `positionIM` deliberately unread |
| `internal/engine/venue/errors.go` | 4 new sentinels (`ErrMarginWrite*`) |
| `internal/oms/application/ports/margingateway.go` | `MarginGateway.AddIsolatedMargin(instrument, amount) error` — **carries the `[SPEC EDIT OWED]` marker** |
| `internal/oms/infrastructure/driven/margingateway.go` | `VenueMarginGateway`, `marginTopUpUrgency = PrioritySafety` (pinned const), `ErrVenueMarginWriterMissing` |
| `internal/oms/application/margindispatch.go` | `MarginOpDispatcher`, `MarginOpDispatch`, `classifyMarginCall`/`classifyMarginEffect` |
| `internal/oms/application/errors.go` | `ErrMarginGatewayMissing`, `ErrIDGeneratorMissing`, `ErrMarginOpInFlight`, `ErrMarginPositionFlat`, `ErrMarginBeforeStateMismatch`; two existing sentinels reworded from "resolver" to "use case" |
| `internal/oms/domain/marginop.go` | `MarginOpPending` now documents TWO producers |
| `internal/oms/module.go` | `Deps.VenueMarginWriter`, `Deps.IDs`, `Module.MarginTopUp`, the wiring branch |

Tests: `marginwrite_test.go` (venue), `margingateway_test.go` (driven),
`margindispatch_test.go` (application), 4 new module wiring tests.
**19 mutants, 0 survivors.**

## 2. The rulings the docs must carry

1. **A returned call CONCLUDES `Applied`; the evidence path is not consulted.**
   Two reasons. (a) §4 scopes `ResolveMarginOp(MarginOpRef)` to "the journaled
   local op" — it is a RECONSTRUCTION of what the response would have said,
   built for the case where no response was read, so preferring it is the same
   information at lower fidelity plus #212's confounds. (b) Run inside the
   propagation bound it answers **`Pending`** on a venue with an act history
   (#214: the history lags independently, so the bound guards its silence) —
   no verdict, no journal write, position stays latched, ladder escalates to a
   PackageDelever **on a top-up that worked**. The remedy's success would be
   indistinguishable from its failure.

2. **The two acknowledgements check different halves and NEITHER venue lets
   both be checked.** Binance `{amount, code, msg, type}` — the ACT, no symbol.
   Bybit a position snapshot — the SUBJECT, no amount. Hence no shared echo
   helper (unlike `marginLiqPrice` on the read side); each codec records what
   it cannot check.

3. **Bybit's `positionIM` is on the response and is not read.** A second
   authority for a figure `ports.VenueMarginStatus` owns; available on one
   venue only; and available only in the case that needs no evidence.

4. **`EffectNotApplied` concludes here where `classifyEffect` cannot.** There
   the value cannot choose between `StateRejected` and
   `StateAbsentAfterFinality` (SM §1 / FLAG-2 separates them by evidence
   class); a margin op has ONE absence answer and both causes the effect covers
   say it. **`EffectApplied` does NOT conclude** — "the request arrived" is not
   "the venue acted", and there is no second authority to ask.

5. **The latch is released by DURABILITY, not by knowledge.** A verdict known
   but not journaled must be reconstructed from evidence, and a second op on
   that position destroys that evidence (the posted margin then moved by the
   sum of two deltas and matches neither). Different from `Outcome.Journaled`
   on the order path, where a lost state record is re-derived by query.

6. **The in-flight map starts EMPTY, on a premise living in another package.**
   A `Pending` op at boot makes `MarginResolution.Outcome()` →
   `OutcomeIncomplete` → `PostureReconcileOnly`, and such a process is not
   trading, so no top-up can be dispatched while one is outstanding. **Expiry
   trigger, written in the field's doc:** the day a root drives safety actions
   through a reconcile-only boot, the map must be seeded from
   `MarginResolution.Resolved`.

7. **The dispatcher MINTS the id (`*ids.Generator`), where `Deleverer.Open`
   takes one.** A `MarginOpID` never leaves the process, and a re-used one puts
   two dispatch records under one subject — `ErrMarginOpAlreadyDispatched`,
   which makes the log unreplayable and the process unbootable. A derived id
   over `(instrument, watermark)` collides for exactly the two ops §7.1
   serializes (unreachable live, trivial under a stub clock).

8. **The port is one method of four.** `ReduceIsolatedMargin` /
   `TransferCollateral` have no consumer; `ResolveMarginOp` is not a gateway
   method in this system at all — it is `MarginOpResolver`, because a port
   method would put §7.1's `Ambiguous` rule behind an adapter.

9. **`marginTopUpUrgency` is a pinned CONST** where `marginStatusUrgency` is a
   constructor argument: §7.6 names "§7.1's margin top-up" in the Safety row,
   and unlike the reads there is no second caller at a second rung.

10. **`ratelimit.PriorityRoutine` is refused for the write**, mirroring
    `OrderRequest.Validate` — a state-changing call is not "a read that keeps
    the risk view live", and both neighbouring classes are a real decision.

## 3. The docs work still owed — START HERE

1. **`docs/build-spec.md` → v0.9.65**
   - §4 `MarginGateway`: change `AddIsolatedMargin(VenueInstrument, delta Money) (Ack, error)`
     to return `error` alone, with the reason (neither venue supplies an
     operation identity, so an `Ack` could only be the empty value).
     **This discharges the `[SPEC EDIT OWED]` marker** in
     `ports/margingateway.go:45` — delete the marker line in the same commit.
   - §7.1: the dispatch half is now BUILT. Its two bullets currently say
     "What is NOT built … the dispatch path (the before-state read, the fsync,
     `AddIsolatedMargin`, and this section's one-op-in-flight-per-position
     serialization — part 2 …)" (around line 580) — replace with the landed
     annotation and add ruling 1 (the returned call concludes) plus ruling 5
     (release on durability).
   - §12 milestone-4 roster item 4 (`docs/build-spec.md` line ~832): add the
     "Slice B3 part 2 landed" annotation beside part 1 / 1b / 1c, and note that
     all three rungs of §7.1's ladder now exist and **none is driven** (B4).
   - Status line at the top (line 3): new headline.
2. **`docs/venue-profiles.md` §4**: the two acknowledgements' asymmetry
   (rulings 2 and 3), and the ⚠ for `POST /fapi/v1/positionMargin`'s unverified
   weight (charged 20) beside the existing `positionMargin/history` one.
3. **`docs/HANDOFF.md`**: new status section at the top, session 128, previous
   one demoted to HISTORICAL.
4. **`docs/adr-history.md`**: §84, task #215 closed; note that **B4 is now the
   only thing between the three built rungs and a working ladder**.
5. **THIS record**: rewrite it into the shape §82/§83 have — the rulings above
   as prose with their rejected alternatives worked through
   (evidence-path-always, caller-minted id, reading `positionIM`, a signed port
   parameter, an `Ack` return, a shared echo helper), the mutation campaign
   with its two NOBUILD re-aims, and the "what is still owed" list. It is a
   handoff note today, which is not the form a decision record keeps.
6. **`docs/tutorial.md`** — check whether #212/#213/#214 touched it; if the
   §7.1 margin-op walkthrough exists, extend it. `tutorial.ka.md` is
   GENERATED — never hand-edit; `make` has a target for the assembler.
7. **Task #217**: extend its description to cover BOTH unverified Binance
   figures — `positionMargin/history` (read) and `positionMargin` (write),
   each charged 20 behind #180's WAF wall.
8. **Mark task #215 completed** once the docs commit lands.

## 4. What is NOT built, and each is named

- **Nothing DRIVES the dispatcher.** B4's loop is the trigger (the liq
  monitor's Rebalance band). All three of §7.1's rungs now exist — trigger
  (#210), top-up (#215), ladder (#177) — and nothing connects them.
- **Nothing re-resolves a live in-flight op.** #212's standing gap. In
  practice a failed top-up latches its position until the next boot, which is
  §7.1's escalation rather than a gap — but B4 should give the dispatcher a
  driver, and the doc names it.
- **A live `Ambiguous` has no in-process reporter.** The dispatch path can
  leave an op unresolved; the next boot resolves it and the gate latches.
- `ReduceIsolatedMargin`, `TransferCollateral`, sizing
  (`margin-for-target-liq-distance`), and the trading-rung gate that would
  refuse a process holding no top-up path (lands with B4).
