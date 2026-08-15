# Cross-Venue Pair Composition Root Implementation Plan (task #236)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `internal/app/pair`, the composition root that boots two single-venue OMS deployments side by side and drives ONE cross-venue perp-perp package across them — opening the second leg only against the first leg's *established* fill, and unwinding the first leg when the second cannot be opened.

**Architecture:** Two `reconcile.Runner`s, two journals, two `oms.Module`s, held open simultaneously by one root. Nothing in the safety kernel is edited: `domain.HedgeGroup` keeps its `(Account, Venue, Symbol)` key, and §6.7's classifier is not called. The pair — the concept record §95 ruling 2 places **above** the hedge group — is a **new type in this root**, composed from two per-venue leg readings, and it is *named and designed* here rather than deferred unnamed. Phase 1 dispatches **sequentially** (leg A, gate on A's established exposure, leg B, unwind A on B's failure), which is record §95 §7 alternative (ii) as the smallest runnable increment of the same root. Phase 2 adds the paired fan-out and fan-in.

**Tech Stack:** Go (root module), `internal/app/reconcile` (the boot), `internal/oms` (the module), `internal/platform/{clock,config,fixedpoint,ids,observability,boot}`, `net/http/httptest` for venue doubles. No new dependencies.

> ## UNBLOCKED — the dependency landed in task #242 (2026-08-15, `d7d2075`)
>
> This plan was blocked because **a leg this system placed reached
> `domain.StateSubmitted` and nothing advanced it**, so D4's `Establish` could
> never establish a leg this root actually placed: every successful run would
> read `VerdictIndeterminate`, leg B would never be sent, D8 would refuse the
> unwind, and a real fill on side A would be left naked and reported as "exposed,
> for a human" — the outcome this slice exists to prevent, produced on the happy
> path.
>
> **Task #242 closed it in ten commits, `e581831..d7d2075`.** The finding turned
> out to be sharper than "nothing calls `ApplyState`": **SM §1 gave `Submitted`
> no query-resolution row**, so a driver written against `CanTransition` would
> have been refused by the domain on the ordinary outcome of every order this
> system sends — `Filled`, `Canceled` and `PartiallyFilled` had no legal target.
> `state-machines.md` v0.9.61 adds the row (`9b87b17`), and `01d27ce` makes
> `Joiner.joinOne` journal and apply the venue's lifecycle reading — the reading
> it has held since slice F2 — as its last step, after the executions are
> admitted and cross-checked.
>
> Decisions, rejected alternatives and the reading pass:
> **`docs/superpowers/specs/2026-08-15-m4-242-order-finality-decisions.md`**.
> The original finding stays in
> `docs/superpowers/specs/2026-08-15-m4-236-order-finality-gap.md`.
>
> **What changes in this plan, and it is two tasks.** `drive.establish` (task 6)
> now reads a terminal state the join actually produces, in place of a polling
> loop that waited on one nothing wrote; and `venueDouble` (task 5) gains the
> venue-status responses the advance consumes — the joiner calls `venue.Resolve`
> and reads `status.State`, so a double that answers only the fill surface will
> leave every leg indeterminate. **Everything else stands**: D4, D5, D7 and D8
> were written against SM §1 as specified, and SM §1 was not wrong — it was
> unimplemented. This plan is **not re-planned here**.
>
> **One correction to D4 beyond the mechanism, and it is the count-word shape.**
> D4's blocking-list enumeration (`Absent` / `Incomplete` / `Unattributed`) was
> total when it was written and is not now: #242 added a fourth per-order finding
> class, **`Contradicted`** — the venue and this system holding lifecycle states
> SM §1 has no edge between — which latches `boot.OutcomeNeedsOperator`. A leg
> whose id appears in it must read **indeterminate**, exactly as the other three
> do. D4 is corrected accordingly.

**Spec:** `docs/superpowers/specs/2026-08-15-m4-236-crossvenue-root-scoping.md` (record §95). Read it before task 1 — in particular §4 (the classifier cannot see a cross-venue pair), §6 (the three rulings) and §7 (the five rejected alternatives). Supporting: `docs/build-spec.md` §5.1, §5.2, §5.3, §6, §6.7, §7.5, §7.6; `docs/state-machines.md` SM §1.

---

## Global Constraints

Copied from `CLAUDE.md` and the spec. Every task's requirements implicitly include this section.

- **No `float64` anywhere under `internal/`.** All money is `platform/fixedpoint` (`Money`/`Quantity`/`Price`/`Rate`). The `nofloat` analyzer denies it by type; the escape is `//nofloat:allow <reason>` and this plan needs none.
- **No `time.Now()`.** Time comes from the injected `*clock.Wall`. Any sleep is an injected `func(time.Duration)` field defaulting to `time.Sleep`, on the precedent of `oms.Deps.SettleWait`.
- **`internal/app/pair` must NOT import `internal/oms/application/ports` or `internal/oms/infrastructure/driven`.** §3.6 rule 6, enforced by the `singlewriter` analyzer, positionally, with `app/` roots explicitly included. It MAY import `internal/oms`, `internal/oms/application` and `internal/oms/domain` — `internal/app/reconcile` already imports all three.
- **A switch over a module-declared enum-like must name every member.** The `exhaustiveswitch` analyzer refuses a `default:` that stands in for a missing one. `pair.Verdict`, `pair.ExitClass` and `pair.Mode` are new enum-likes and every switch over them names all members.
- **The zero value is unusable.** Every constructed type gets a `Validate()` that refuses the unpopulated case.
- **`fixedpoint` comparisons are fallible and there is no `IsZero` or `Equal`.** `Quantity` and `Price` expose `Sign() (int, error)` and `Cmp(o) (int, error)`; both fail on an unpopulated value or an asset-tag mismatch, because — `Quantity.Sign`'s own doc — *"a flat position and a position nobody populated would otherwise be indistinguishable."* Never compare either type with `==`: that compares an unexported mantissa and tag, and succeeds silently across two differently-scaled representations of one number. **Every task in this plan uses these two test helpers rather than writing the comparison inline**, so no assertion re-derives the rule:

  ```go
  // eqQty fails the test unless got and want are the same quantity, INCLUDING
  // the asset tag. An error from Cmp is a failure, not a mismatch — it means one
  // of the two was never populated, which is the case this repo's fixedpoint
  // refuses to let read as zero.
  func eqQty(t *testing.T, got, want fixedpoint.Quantity) {
      t.Helper()
      cmp, err := got.Cmp(want)
      if err != nil {
          t.Fatalf("comparing %v and %v: %v", got, want, err)
      }
      if cmp != 0 {
          t.Errorf("quantity = %v, want %v", got, want)
      }
  }

  // eqPrice is the same for Price.
  func eqPrice(t *testing.T, got, want fixedpoint.Price) {
      t.Helper()
      cmp, err := got.Cmp(want)
      if err != nil {
          t.Fatalf("comparing %v and %v: %v", got, want, err)
      }
      if cmp != 0 {
          t.Errorf("price = %v, want %v", got, want)
      }
  }
  ```

  `internal/app/pair` gets one copy in `pair_test.go`; `internal/oms/legstate_test.go` gets its own (different package).
- **A "flat" assertion is `Sign() == 0`, and its error is a failure.** In implementation code, `q.Sign()` returning an error means the quantity was never populated — never absorb it into "flat".
- **Deviations from the spec are marked, not absorbed:** `[SPEC EDIT OWED]` in code, and the slice-close gate is `grep -rn "SPEC EDIT OWED" internal/ tools/`. This plan introduces **exactly one** owed marker (task 3) and task 14 lands it.
- **Test commands:** `go test -race ./internal/app/pair/...` for the inner loop; `make ci` before the final commit. `make test` pins `RAPID_SEED=20260726`.
- **Commits:** no AI/model attribution trailers, in commit messages or anywhere else.
- **Demo, not testnet.** Every credential is `*_DEMO_*`; hosts are `demo-fapi.binance.com` and `api-demo.bybit.com`. On Bybit demo and testnet are *different environments* (821 vs 809 symbols). Say "demo".

---

## Design Decisions Carried Into This Plan

These were settled by record §95 or by reading the code during planning. They are stated once here so no task re-derives them.

**D1 — The package is `internal/app/pair`, the binary verb is `arblab pair run`.** Not `crossvenue`: `arblab crossvenue {check,fetch,sim,gate}` are all offline research tools, and an operator must not be one word away from confusing an offline simulation with a live order path. `pair` is also ruling 2's own noun.

**D2 — Both legs plan under ONE shared `ids.TradeID`, one leg per side, leg index 0 on each.** `ids.ClientOrderID` encodes `(Trade, Leg, Attempt)` and carries **no venue**, so two single-leg plans under one trade mint the *same* id on both venues. That is safe here and the safety rests on two facts the config **enforces** rather than asserts: the two sides name **different venues**, and the two sides use **different physical journal identities**, not merely different path spellings. No store spans venues, so the two ids never meet in one place; the venue is the disambiguator, and the shared id is what makes the partner fact durable in both journals and on both venues' order pages with no kernel change. **This premise is a tested refusal, not a paragraph** (task 3).

**D3 — One leg per side, for this slice, and the reason is D2.** With two legs on one side, that side's indices are 0 and 1 while the other side's restart at 0, so "leg index" would stop identifying a leg within the pair. Widening needs a leg-index base on `application.TradePlan`, which is a kernel change and therefore not #236's (ruling 3).

**D4 — Exposure is read, never assumed, and the reading is three-valued.** After a leg's IOC, the root calls that side's `Module.Join.Join([id])` and then a new `Module.LegState(id)`. **`Join` is what ADVANCES the leg's state** — since task #242 (`01d27ce`) it journals and applies the venue's lifecycle reading as its last step — so `LegState(id)` reads a terminal one on the ordinary path rather than a `Submitted` that nothing moves. Exposure is **established** only when the leg's state is terminal for state *and* the id appears in none of the join's `Absent`/`Incomplete`/`Unattributed`/**`Contradicted`** lists (`Contradicted` is #242's fourth finding class — the venue and this system holding states SM §1 has no edge between; it latches `boot.OutcomeNeedsOperator`, so it can never be read as established). Anything else is **indeterminate**, which stops the run. This is the direction that matters: a zero cumulative read too early is indistinguishable from "did not fill", and concluding "did not fill" on a leg that actually filled leaves naked exposure reported as a clean run.

**D5 — Leg B is sized to leg A's ESTABLISHED FILL, not to the configured quantity.** A partial fill on A hedged at the configured size is naked exposure on B. If A's fill fails B's own §2.4 filters, `Plan` refuses — nothing is sent on B — and the run unwinds A. Quantizing A's fill down to B's grid and hedging the remainder is better and is **owed, not built** (task 14 records it).

**D6 — If A's established fill is ZERO, leg B is not sent and the run is clean.** An IOC that did not fill opened nothing, so there is nothing to hedge and nothing to unwind. On a thin demo book with a tight marketable limit this is a *likely* outcome, and it is a real result: it proves the wire contract, the journal path and both boots, and proves nothing about fills.

**D7 — The unwind is planned under the same `TradeID`, same leg index, `Attempt = pair.attempt + 1`, `ReduceOnly: true`, opposite side, sized to A's established fill, at the leg's declared `unwind_limit_price`, with `UrgencySafety`.** The both-legs-flat gate in task 5 is what makes that inverse reduce-only order legal and makes a complete close return the account to its starting position: the opening leg created the entire position it closes. A fresh attempt is what §5.3 requires for a second submission on a leg; a re-run of the same document is refused by the planner's duplicate check, which is the fail-safe direction. `UrgencySafety` because reducing exposure is §7.6's safety class and must stay available under a stop-opening latch.

**D8 — The unwind is permitted ONLY on a `VerdictHalfOpen` pair whose exposed side is side A.** A compensating trade is §6.7's business and §6.7 cannot see this pair (record §95 §4). So the root does not compensate on any reading it cannot fully establish: `VerdictSkewed` and `VerdictIndeterminate` end the run at `ExitExposed` for a human. That refusal is the honest form of the gap, and it is what task 14 marks as owed to #241.

**D9 — One `*clock.Wall`, built from BOTH sides' calibrations.** `clock.NewWall` takes a `[]clock.Calibration` and refuses duplicate venues, so one wall holds both venue frames and the platform backstops D2's distinct-venue rule. `observability.NewRegistry()` is per side: a collector registered by one side must not be able to refuse the other's boot.

**D10 — This plan builds phase 1 and phase 2 only.** What it does not build is listed at the end, with reasons.

---

## File Structure

| File | Responsibility |
|---|---|
| `internal/oms/legstate.go` (create) | `Module.LegState` and `Module.VenuePosition` — the order and venue-position reads a composition root needs, in the package `singlewriter` permits to import `ports`. |
| `internal/oms/legstate_test.go` (create) | Its tests. |
| `internal/app/pair/pair.go` (create) | The **pair concept**: `LegExposure`, `Verdict`, `Compose`. Pure logic, no I/O, no venue. |
| `internal/app/pair/pair_test.go` (create) | The verdict table, with the collapse checklist applied to it. |
| `internal/app/pair/errors.go` (create) | Sentinel errors, on `exercise/errors.go`'s pattern. |
| `internal/app/pair/config.go` (create) | `Config`, `SideConfig`, `PairConfig`, `LegConfig`, `Validate`, `legIntent`, `plan`. |
| `internal/app/pair/config_test.go` (create) | Validation refusals + the positive control. |
| `internal/app/pair/filters.go` (create) | `tradingFilters` across both sides' instrument tables, per side. |
| `internal/app/pair/report.go` (create) | `Report`, `SideReport`, `ExitClass` and its precedence. |
| `internal/app/pair/app.go` (create) | `Run`: two boots, the both-clean-and-venue-flat gate, the sequential drive, the unwind, teardown. |
| `internal/app/pair/app_test.go` (create) | End-to-end over two `httptest` venue doubles. |
| `internal/app/pair/imports_test.go` (create) | This root's own import fence. |
| `cmd/arblab/pair.go` (create) | `arblab pair run -config <file>`: load, run, render, map exit codes. |
| `cmd/arblab/pair_test.go` (create) | Dispatch routing and exit-code mapping. |
| `cmd/arblab/main.go` (modify) | One dispatch arm and the usage block. |
| `configs/pair.demo.example.json` (create) | The operator's starting document. Secret **refs** only, never values. |
| `docs/superpowers/specs/2026-08-15-m4-236-crossvenue-root-scoping.md` (modify) | §10: what was built, and what task 14 marks owed. |
| `docs/HANDOFF.md` (modify) | The session block. |

---

## Test Scaffolding

Every helper the test code below references, with its contract. Build them as the first task that needs them; do not invent a second one later under a different name.

| Helper | Contract | Build from |
|---|---|---|
| `eqQty(t, got, want)` / `eqPrice(t, got, want)` | Fallible-comparison assertions. See Global Constraints. | — |
| `qty(t, "100")` | `fixedpoint.ParseQuantity("LAB", s)`, fatal on error. | task 2 |
| `qtyStep(t, "0.1")` | Same, for a lot-step value. Same asset. | task 4 |
| `testWall(t)` | `clock.NewWall(nil)`, fatal on error. Used where no venue frame is needed. | task 4 |
| `timeAgo(t, d)` | `testWall(t).Now().Add(-d)`. Reference-row freshness is a RELATIVE fact — a literal date would start failing `max_age` some months after it was typed. | task 4 |
| `validConfig(t)` | A `Config` that validates: two sides (binance/bybit), linear-perp `LABUSDT`, one leg each, opposite sides, `mode: "sequential"`, `settle: 10ms`, `settle_attempts: 3`, leg 0's `limit_price` and `unwind_limit_price` **different**. | task 3, adapting `internal/app/exercise/app_test.go:228` |
| `validConfigWithSteps(t, a, b)` | `validConfig` whose reference revision gives the two sides **different** lot steps. | task 4 |
| `writeCrossVenueReference(t, observedAt, stepA, stepB)` | Writes a four-row reference revision (two fee rows, two lot rows) to `t.TempDir()`, returns the path. | task 4, from `exercise/app_test.go:189` |
| `writeCrossVenueReferenceNoTick(t, observedAt)` | Same, omitting the `tick` key — the pre-tick row shape, which is not the same row as a declared `"0"`. | task 4 |
| `runnableConfig(t)` | Returns `(config.Snapshot[Config], [2]*venueDouble)`: a `validConfig` whose two sides' endpoints point at two `httptest` servers, plus the doubles. | task 5, from `exercise/app_test.go:74` and `chaos_test.go` |
| `runnableConfigWithSteps(t, a, b)` | The runnable form of `validConfigWithSteps`, preserving the same two venue doubles while giving each side a distinct lot grid. | task 7 |
| `runnableConfigWithMatchedStartingPosition(t, side, net)` | A runnable config whose selected venue and seeded journal agree on the same non-zero net, so boot is clean while the account is not flat. Without the matching journal seed, boot's disagreement gate stops first and the flat-start gate is never tested. | task 5 |
| `venueDouble` | Per side. Serves a clean boot's reads; records every `Place`. Controls: `refuseVenueJoin()`, `fillIOC(qty)`, `holdOrderNonTerminal()`, `refuseDispatchBeforePlace()`, `rejectPlace()`, `rejectPlaceAfter(n)`, `banOnPlace()`, `position(instrument, net)`. `refuseDispatchBeforePlace()` produces a non-nil `LegResult.Err` and no `Place`; `rejectPlace()` reaches the venue and produces terminal `StateRejected`, so one cannot stand in for the other. Reads: `placeCalls()`, `lastPlace()`, `firstPlace()`, `lastPlaceQuantity()`, `firstJoinAt()`, `sawPlaceBefore(t)`. | task 5 |
| `stubSleep(f)` | Returns a per-test runner that passes `f` to `runWithWait`; it mutates no package state, and `Run` delegates with `time.Sleep`. | task 6 |
| `setMode(&snap, ModePaired)` | Rewrites `snap.Value.Pair.Mode` with the declared enum-like, not a bare string. | task 12 |
| `newPlaceBarrier(t, n, timeout)` / `double.blockInPlace(b)` | A rendezvous of `n` parties with its own bound. Each double blocks inside `Place` until all `n` have arrived, then all proceed; `bothArrived()` reports whether the rendezvous completed. **The timeout is the point** — a sequential implementation can never supply the second party, and without a bound the test would HANG rather than fail, which names no defect. Keep the bound well under the harness's own limit, or a timeout at the harness limit is not a bound. | task 12 |
| `cleanBoot()` / `latchedBoot()` / `incompleteBoot()` | `reconcile.Report` values whose `ExitClass()` is each of the three. | task 8, from `exercise/app_test.go:436` |
| `reportWith(bootA, bootB, verdict, drove)` | A `Report` with two clean-journaled open legs. | task 8 |
| `reportWithLegs(bootA, bootB, verdict, leg)` | Same, with side 0's `Open` replaced by `leg`. | task 8 |
| `unjournaledLeg()` | A `*LegReport` whose `Result.Outcome.Journaled` is false — SENT, and its record needs eyes. | task 8 |
| `refusedLeg()` | A `*LegReport` whose `Refusal` is non-nil and whose `Result` is the zero value — attempted, nothing sent. It must differ from `unjournaledLeg()` in whether Place ran and in nothing else that `ExitClass` reads, or the two rungs it separates collapse. | task 8 |
| `packageDeps(t, importPath)` | `go list -deps` over the package, returning import paths. | task 9, from `reconcile/imports_test.go` |
| `moduleWithOnePlannedLeg(t)` | An `*oms.Module` with one leg planned, plus that leg's id/instrument/side/quantity. | task 1, from `internal/oms/module_test.go` |

**A note on `venueDouble`, because it is the largest piece of scaffolding here and the one most able to lie.** It must be able to distinguish *the order is terminal and filled nothing* from *the order is not terminal yet* — those are `TestSideBIsNotSentWhenSideAFilledNothing` and `TestSideBIsNotSentWhileSideAIsIndeterminate`, and both produce a cumulative of zero. A double that could only express one of them would make one of those two tests vacuous while it passed. Give it an explicit terminal-state control, and assert in a self-test that the two modes produce **different** order-query responses. It must also distinguish a pre-`Place` dispatch refusal from a venue rejection: the former leaves the planned order dispatchable and appears as `LegResult.Err`; the latter crossed the wire and is terminal `StateRejected`. Collapsing them is how both nothing-sent compensation paths disappear behind one passing rejection fixture.

**`fillIOC(q)` must leave the order TERMINAL, and for a PARTIAL fill that means `StateCanceled` with a non-zero cumulative — never `StatePartiallyFilled`.** `domain.OrderState.IsTerminal` returns true for exactly `Filled`, `Canceled`, `Rejected` and `AbsentAfterFinality`; `PartiallyFilled` is explicitly non-terminal, because it means "some fills applied and MORE POSSIBLE". An IOC cannot rest, so its unfilled remainder is canceled and the real venue lands it terminal — but a double that modelled a partial fill as `PartiallyFilled` would be *plausible*, and `Establish` would refuse it, turning `TestSecondLegIsSizedToTheFirstLegsEstablishedFill` — the test D5 exists for — into a `VerdictIndeterminate` run that never sends leg B. The double would be wrong in the direction that looks safe, which is the direction nobody investigates.

**`venueDouble`'s default position answer is FLAT, and it has to be stated rather than left implicit**, because task 5's gate now refuses a non-flat start: a double that defaulted to anything else would refuse every other test in this package, and one that had no position surface at all would make `Module.VenuePosition` error rather than answer. Say so in the helper's own comment. Note what that costs: with a flat default, the positive control's explicit `position(instrument, "0")` cannot distinguish "the gate read the venue" from "the gate read a default" — only the non-flat subtests separate those, which is why both of them exist and why neither may be dropped as redundant.

---

# PHASE 1 — the sequential gated pair (record §95 §7 alternative (ii))

## Task 1: `oms.Module.LegState` and `VenuePosition` — the reads the root cannot otherwise take

**Why this is task 1:** the root needs one leg's state and cumulative filled quantity to decide anything, and it must know the venue was flat before relying on a reduce-only inverse to restore that starting state. `ports.OrderRecord` and `ports.VenuePositions` are behind the `singlewriter` fence. `internal/oms` is on the permitted side of that fence, so both reads belong here as projections into permitted vocabulary — §5.1's own pattern, and the reason `oms/domain` is deliberately out of the gate's scope.

**Files:**
- Create: `internal/oms/legstate.go`
- Modify: `internal/oms/module.go` (retain the already-constructed `ports.VenuePositions` read capability on `Module`)
- Test: `internal/oms/legstate_test.go`

**Interfaces:**
- Consumes: `m.orders` (`*driven.MemoryOrderWriteRepository`), already a field on `Module`.
- Produces:
  ```go
  type LegState struct {
      Instrument ids.VenueInstrument
      Side       domain.Side
      State      domain.OrderState
      Ordered    fixedpoint.Quantity
      Cumulative fixedpoint.Quantity
      Fills      int
  }
  func (m *Module) LegState(id ids.ClientOrderID) (LegState, error)

  type VenuePosition struct {
      Instrument ids.VenueInstrument
      Net        fixedpoint.Quantity
  }
  func (m *Module) VenuePosition(scope application.PositionScope, instrument ids.VenueInstrument) (VenuePosition, error)
  ```

- [ ] **Step 1: Write the failing test**

Add to `internal/oms/legstate_test.go`. Use whatever module-construction helper `internal/oms/module_test.go` already provides — open it first and reuse its fixture rather than writing a second one; if it constructs via `oms.New(deps)` with a temp journal, do the same here.

```go
package oms_test

import (
	"errors"
	"testing"

	"github.com/giiorg/arb-lab/internal/oms"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// TestLegStateReportsAPlannedLeg is the positive control: a leg that exists
// reads back with its instrument, side, state and a ZERO cumulative of the
// ordered asset — which ports.OrderRecord documents as a real answer rather
// than a missing one.
func TestLegStateReportsAPlannedLeg(t *testing.T) {
	t.Parallel()
	m, planned := moduleWithOnePlannedLeg(t) // helper below

	got, err := m.LegState(planned.ID)
	if err != nil {
		t.Fatalf("LegState(%q) = %v, want nil", planned.ID, err)
	}
	if got.Instrument != planned.Instrument {
		t.Errorf("Instrument = %v, want %v", got.Instrument, planned.Instrument)
	}
	if got.Side != planned.Side {
		t.Errorf("Side = %v, want %v", got.Side, planned.Side)
	}
	if got.State != domain.StatePlanned {
		t.Errorf("State = %v, want %v", got.State, domain.StatePlanned)
	}
	eqQty(t, got.Ordered, planned.Quantity)
	// Flat is Sign()==0, and Sign's error is a FAILURE rather than "flat": a
	// cumulative nobody populated is exactly what fixedpoint refuses to let read
	// as zero, and a zero cumulative of the ordered asset is what
	// ports.OrderRecord promises here.
	if sign, err := got.Cumulative.Sign(); err != nil {
		t.Errorf("Cumulative.Sign() = %v; the record must carry a zero OF THE ORDERED ASSET, not an unpopulated value", err)
	} else if sign != 0 {
		t.Errorf("Cumulative = %v, want flat", got.Cumulative)
	}
	if got.Fills != 0 {
		t.Errorf("Fills = %d, want 0", got.Fills)
	}
}

// TestLegStateRefusesAnUnknownID is the negative control. A root asking about
// an id the store never held must get an error, not a zero LegState — a zero
// LegState reads as "planned, nothing filled", which is the flattering
// direction for a leg that may be live at a venue.
func TestLegStateRefusesAnUnknownID(t *testing.T) {
	t.Parallel()
	m, _ := moduleWithOnePlannedLeg(t)

	got, err := m.LegState(ids.ClientOrderID("ab-00000000000000000000000000-00-00"))
	if err == nil {
		t.Fatalf("LegState(unknown) = %+v, nil; want an error", got)
	}
	if got != (oms.LegState{}) {
		t.Errorf("LegState on error = %+v, want the zero value", got)
	}
	_ = errors.Unwrap // keep the import honest if the assertion above changes
	_ = fixedpoint.Quantity{}
}

// TestVenuePositionReportsTheVenuesOwnNet is the positive control. The fresh
// journal is flat while the venue double reports a pre-existing long, so a
// projection accidentally derived from the journal cannot pass.
func TestVenuePositionReportsTheVenuesOwnNet(t *testing.T) {
	t.Parallel()
	m, scope, instrument, want := moduleWithVenuePosition(t, "25")

	got, err := m.VenuePosition(scope, instrument)
	if err != nil {
		t.Fatalf("VenuePosition(%s) = %v, want nil", instrument, err)
	}
	eqQty(t, got.Net, want)
}

// TestVenuePositionRefusesAnUnknownInstrument stops an absent row for an
// instrument this module never declared being promoted into evidence of
// flatness. Absence is flat only inside the configured enumeration.
func TestVenuePositionRefusesAnUnknownInstrument(t *testing.T) {
	t.Parallel()
	m, scope, _, _ := moduleWithVenuePosition(t, "0")
	unknown := ids.VenueInstrument{Venue: "binance", Product: ids.ProductLinearPerp, Contract: ids.ContractLinear, Symbol: "UNKNOWNUSDT"}

	if _, err := m.VenuePosition(scope, unknown); err == nil {
		t.Fatal("VenuePosition(unknown) = nil; want an error")
	}
}
```

Write `moduleWithOnePlannedLeg(t *testing.T) (*oms.Module, plannedLeg)` in the same file, reusing `module_test.go`'s existing construction. `plannedLeg` is a local struct carrying `ID`, `Instrument`, `Side`, `Quantity` — whatever the helper planned. If `module_test.go` is in package `oms` rather than `oms_test`, put this file in the same package and drop the `oms.` qualifiers.

Write `moduleWithVenuePosition` beside it. Its venue reader must return a non-zero row while its fresh journal contains no fills; otherwise the venue-owned and journal-derived readings collapse to the same fixture and the projection can use the wrong authority while the test passes.

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/oms/ -run 'TestLegState|TestVenuePosition' -v`
Expected: FAIL, `m.LegState undefined` and `m.VenuePosition undefined`.

- [ ] **Step 3: Write the minimal implementation**

Create `internal/oms/legstate.go`:

```go
package oms

import (
	"fmt"

	"github.com/giiorg/arb-lab/internal/oms/application"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// LegState is what a composition root may read about one leg it dispatched.
//
// # Why it exists here and not at the root
//
// §3.6 rule 6 bars every app/ root from importing oms/application/ports, by
// position, and ports.OrderRecord is where the state and the cumulative live.
// The rule is right — a root that can name the write port can grow a second
// write path — and the read it forbids is one a cross-venue root genuinely
// needs: it cannot size a hedge against a fill it cannot see. So the read is a
// PROJECTION, minted in the one package the gate permits, into the vocabulary
// oms/domain already speaks. That is §5.1's own pattern and it is the reason
// oms/domain is deliberately outside the gate's scope.
//
// # It is read-only and it is not a verdict
//
// It reports what the current-state projection holds. It does NOT say the
// reading is trustworthy: a Cumulative of zero on a non-terminal order means
// "nothing has been attributed yet", which is not the same claim as "nothing
// filled". The caller composes that judgement from this plus its own join
// report — see internal/app/pair's Compose, which refuses to conclude on any
// leg that is not terminal for state.
type LegState struct {
	// Instrument and Side are the plan's own, fixed when the order was planned.
	Instrument ids.VenueInstrument
	Side       domain.Side

	// State is where the order is in SM §1's lifecycle.
	State domain.OrderState

	// Ordered is the planned size; Cumulative is what the fill ledger has
	// admitted, zero OF THE ORDERED ASSET on a record with no fills.
	Ordered    fixedpoint.Quantity
	Cumulative fixedpoint.Quantity

	// Fills is how many distinct executions were admitted. It is here for the
	// same reason ports.OrderRecord carries it: a cumulative that matches while
	// the count does not is what a double-counted fill plus a missed one looks
	// like from the quantity alone.
	Fills int
}

// LegState reports the projection's current answer for one leg.
//
// An unknown id is an ERROR rather than a zero value, and the direction is the
// point: a zero LegState reads as "planned, nothing filled", which is the
// flattering answer for a leg that may be live at a venue.
func (m *Module) LegState(id ids.ClientOrderID) (LegState, error) {
	rec, err := m.orders.Load(id)
	if err != nil {
		return LegState{}, fmt.Errorf("leg state %q: %w", id, err)
	}
	return LegState{
		Instrument: rec.Order.Instrument,
		Side:       rec.Order.Side,
		State:      rec.State,
		Ordered:    rec.Order.Quantity,
		Cumulative: rec.Cumulative,
		Fills:      rec.Fills,
	}, nil
}

// VenuePosition is the venue-owned position projection for one instrument.
type VenuePosition struct {
	Instrument ids.VenueInstrument
	Net        fixedpoint.Quantity
}

// VenuePosition reports the VENUE'S OWN net position for one declared
// instrument. It does not derive from the journal: a fresh journal is flat no
// matter what the account already holds, which is precisely the non-flat start
// the pair root must refuse before a reduce-only inverse can be trusted.
//
// A declared instrument absent from the venue's complete scope is returned as
// a typed zero. An instrument the module never declared is refused; treating
// those two absences alike turns "we enumerated it and it was absent" into
// "we never asked about it" while reporting both as flat.
func (m *Module) VenuePosition(scope application.PositionScope, instrument ids.VenueInstrument) (VenuePosition, error) {
	assets, ok := m.instrumentAssets[instrument]
	if !ok {
		return VenuePosition{}, fmt.Errorf("venue position %s: instrument is not declared", instrument)
	}
	report, err := m.venuePositions.Positions(scope)
	if err != nil {
		return VenuePosition{}, fmt.Errorf("venue position %s: %w", instrument, err)
	}
	for _, row := range report.Positions {
		if row.Instrument == instrument {
			return VenuePosition{Instrument: instrument, Net: row.Net}, nil
		}
	}
	zero, err := fixedpoint.ParseQuantity(assets.Base, "0")
	if err != nil {
		return VenuePosition{}, fmt.Errorf("venue position %s typed zero: %w", instrument, err)
	}
	return VenuePosition{Instrument: instrument, Net: zero}, nil
}
```

Retain the `positions` adapter already constructed in `oms.New` as `venuePositions ports.VenuePositions` and retain the validated instrument-assets table as `instrumentAssets map[ids.VenueInstrument]venue.InstrumentAssets` on `Module`; do not construct a second adapter in this projection. That keeps the read on the same ACL and venue capability the boot used, while still leaving `ports` unnameable from `internal/app/pair`.

If `domain.Order`'s size field is not named `Quantity`, read `internal/oms/domain/order.go` and use the real name; do not invent one.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test -race ./internal/oms/ -run 'TestLegState|TestVenuePosition' -v`
Expected: PASS, all four tests.

- [ ] **Step 5: Bite-check the claim**

The claim under test is "LegState reports the RECORD's cumulative". Mutate it to report the ordered quantity instead and confirm the suite fails:

```bash
tools/mutate.sh internal/oms/legstate.go <line-of-Cumulative-field> \
  's/Cumulative: rec.Cumulative/Cumulative: rec.Order.Quantity/' \
  'LegState reports the ordered size as the cumulative' ./internal/oms/
tools/mutate.sh internal/oms/legstate.go <line-of-VenuePosition-row-match> \
  's/if row.Instrument == instrument {/if false {/' \
  'VenuePosition discards the venue row and returns the fresh-journal-shaped zero' ./internal/oms/
```

Expected verdicts: `caught`, `caught`. A `SURVIVED` on the first means the positive-control fixture has a zero ordered quantity, where the two are the same number — fix the fixture, not the mutation. A `SURVIVED` on the second means the venue-position control is flat, where the venue and fresh-journal answers collapse — make it non-zero.

- [ ] **Step 6: Commit**

```bash
git add internal/oms/legstate.go internal/oms/legstate_test.go internal/oms/module.go
git commit -m "oms: expose the two venue-root projections

§3.6 rule 6 keeps ports.OrderRecord out of every app/ root, and a
cross-venue root cannot size a hedge against a fill it cannot see. The
reads land as projections in the package the gate permits, into permitted
vocabulary — §5.1's own pattern. VenuePosition reads the venue rather than
the journal because a fresh journal says flat about a non-flat account. An unknown id errors rather
than returning a zero value, because a zero LegState reads as 'planned,
nothing filled' for a leg that may be live at a venue."
```

---

## Task 2: the pair concept — `Verdict` and `Compose`

**Why this task is next and why it is pure:** record §95 ruling 2 places the cross-venue residual in a new concept **above** the hedge group, and ruling 3 keeps the normative edit in #241. So this is where the concept is *named and given a composition rule*, with no I/O, no venue and no kernel edit. Everything after it consumes this vocabulary.

**Files:**
- Create: `internal/app/pair/pair.go`
- Create: `internal/app/pair/errors.go`
- Test: `internal/app/pair/pair_test.go`

**Interfaces:**
- Consumes: `oms.LegState` (task 1), `domain.OrderState`, `fixedpoint.Quantity`.
- Produces:
  ```go
  type LegExposure struct {
      Venue       ids.Venue
      ID          ids.ClientOrderID
      Instrument  ids.VenueInstrument
      Side        domain.Side
      State       domain.OrderState
      Filled      fixedpoint.Quantity
      Established bool
      Because     string
  }
  func Establish(venue ids.Venue, id ids.ClientOrderID, st oms.LegState, joined JoinFacts) LegExposure

  type JoinFacts struct{ Absent, Incomplete, Unattributed bool }

  type Verdict uint8
  const (
      VerdictUnopened Verdict = iota
      VerdictIntact
      VerdictHalfOpen
      VerdictSkewed
      VerdictIndeterminate
  )
  func (v Verdict) String() string
  func Compose(a, b LegExposure) (Verdict, error)
  func Exposed(v Verdict) bool
  ```

**`Compose` is FALLIBLE, and the reason is `fixedpoint`'s own.** `Quantity` has no `IsZero` and no `Equal`. It has `Sign() (int, error)` and `Cmp(Quantity) (int, error)`, and `Sign`'s doc says exactly why it is fallible: *"a flat position and a position nobody populated would otherwise be indistinguishable."* That is this task's hazard word for word — an unpopulated quantity reading as flat is a naked position reading as no position. So `Compose` returns `(VerdictIndeterminate, err)` on any quantity it cannot read, which is the fail-closed direction and gives the report a reason to print.

It is also fallible on two legs with the same `domain.Side`. `Validate` names that document defect, but assuming it here lets two equal buys compose as `VerdictIntact`; the pure boundary repeats the premise because it has callers in two modes and no right to inherit one caller's discipline.

- [ ] **Step 1: Write the failing test**

Create `internal/app/pair/pair_test.go`.

**Before writing the table, apply the collapse checklist to it** — this repo has been caught eleven times, and this table has three collapses waiting in it:

1. **Quantity equality vs non-zero.** If every fixture uses the same filled quantity on both sides, `VerdictIntact` ("equal") and `VerdictSkewed` ("both non-zero") are the same predicate. The table below fills **100 vs 60**.
2. **Filled vs ordered.** If every fixture's filled equals its ordered quantity, a `Compose` that read `Ordered` instead of `Filled` would pass. The table orders 100 on both sides and fills less on at least one.
3. **The α = ½ complement.** Never fill exactly half on one side against a full other side, or `(1−α)·q` and `α·q` are the same number — this is record §89's own trap and `[[a-half-collapses-a-complement]]`. 100/60 is a 0.6 ratio for exactly that reason.

```go
package pair

import (
	"errors"
	"testing"

	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// qty is the fixture's quantity constructor. LAB is the shortlist alt record
// §95 §2 measured trading on both demo venues.
func qty(t *testing.T, s string) fixedpoint.Quantity {
	t.Helper()
	q, err := fixedpoint.ParseQuantity("LAB", s)
	if err != nil {
		t.Fatalf("ParseQuantity(LAB, %q): %v", s, err)
	}
	return q
}

// leg builds an ESTABLISHED exposure. Every field that Compose reads is a
// parameter, so no test below can be right for a reason the fixture supplied.
func leg(t *testing.T, venue string, side domain.Side, filled string) LegExposure {
	t.Helper()
	return LegExposure{
		Venue:       ids.Venue(venue),
		ID:          ids.ClientOrderID("ab-01j0000000000000000000000-00-00"),
		Instrument:  ids.VenueInstrument{Venue: ids.Venue(venue), Product: ids.ProductLinearPerp, Contract: ids.ContractLinear, Symbol: "LABUSDT"},
		Side:        side,
		State:       domain.StateFilled,
		Filled:      qty(t, filled),
		Established: true,
	}
}

func TestComposeVerdicts(t *testing.T) {
	t.Parallel()

	unestablished := func(l LegExposure) LegExposure {
		l.Established, l.Because = false, "state is not terminal"
		l.State = domain.StateSubmissionUnknown
		return l
	}

	cases := []struct {
		name string
		a, b LegExposure
		want Verdict
	}{
		{
			// Nothing filled on either side: the IOC-did-not-fill case, which
			// is the LIKELY outcome on a thin demo book and is a real clean run.
			name: "neither side filled",
			a:    leg(t, "binance", domain.SideSell, "0"),
			b:    leg(t, "bybit", domain.SideBuy, "0"),
			want: VerdictUnopened,
		},
		{
			name: "both filled the same quantity",
			a:    leg(t, "binance", domain.SideSell, "100"),
			b:    leg(t, "bybit", domain.SideBuy, "100"),
			want: VerdictIntact,
		},
		{
			// The one the whole slice exists for: A carries exposure, B does not.
			name: "only side A filled",
			a:    leg(t, "binance", domain.SideSell, "100"),
			b:    leg(t, "bybit", domain.SideBuy, "0"),
			want: VerdictHalfOpen,
		},
		{
			// The mirror. It must NOT collapse into the case above — a rule that
			// only looked at A would answer HalfOpen for both and pass.
			name: "only side B filled",
			a:    leg(t, "binance", domain.SideSell, "0"),
			b:    leg(t, "bybit", domain.SideBuy, "100"),
			want: VerdictHalfOpen,
		},
		{
			// 100 vs 60. NOT 100 vs 50: at one half the remainder equals the
			// filled complement and two different wrong rules agree.
			name: "both filled, unequal",
			a:    leg(t, "binance", domain.SideSell, "100"),
			b:    leg(t, "bybit", domain.SideBuy, "60"),
			want: VerdictSkewed,
		},
		{
			name: "side A not established",
			a:    unestablished(leg(t, "binance", domain.SideSell, "0")),
			b:    leg(t, "bybit", domain.SideBuy, "100"),
			want: VerdictIndeterminate,
		},
		{
			name: "side B not established",
			a:    leg(t, "binance", domain.SideSell, "100"),
			b:    unestablished(leg(t, "bybit", domain.SideBuy, "0")),
			want: VerdictIndeterminate,
		},
		{
			// Indeterminate OUTRANKS everything, including the reading that
			// would otherwise be Intact. An unestablished leg's zero is not a
			// measurement, and a rule that let the other side's number decide
			// would conclude on it.
			name: "unestablished outranks an otherwise-intact reading",
			a:    unestablished(leg(t, "binance", domain.SideSell, "100")),
			b:    unestablished(leg(t, "bybit", domain.SideBuy, "100")),
			want: VerdictIndeterminate,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := Compose(tc.a, tc.b)
			if err != nil {
				t.Fatalf("Compose(%s, %s) error = %v, want nil", tc.a.Venue, tc.b.Venue, err)
			}
			if got != tc.want {
				t.Errorf("Compose(%s, %s) = %s, want %s", tc.a.Venue, tc.b.Venue, got, tc.want)
			}
		})
	}
}

// TestComposeRefusesAnUnpopulatedQuantity is why Compose is fallible.
// fixedpoint.Quantity.Sign is fallible for the same reason it states in its own
// doc — "a flat position and a position nobody populated would otherwise be
// indistinguishable" — and here that difference is a naked position reading as
// no position. The fixture uses the ZERO Quantity, which carries no asset tag,
// against a legitimately-flat one that does.
func TestComposeRefusesAnUnpopulatedQuantity(t *testing.T) {
	t.Parallel()
	a := leg(t, "binance", domain.SideSell, "0") // flat, and tagged LAB
	b := leg(t, "bybit", domain.SideBuy, "0")
	b.Filled = fixedpoint.Quantity{} // nobody populated it

	got, err := Compose(a, b)
	if err == nil {
		t.Fatalf("Compose with an unpopulated quantity = %s, nil; want an error", got)
	}
	if got != VerdictIndeterminate {
		t.Errorf("Compose on error = %s, want VerdictIndeterminate — the fail-closed direction", got)
	}
	// The control: the SAME shape with both quantities populated is a clean
	// VerdictUnopened, so the test above is about the tag and not about zero.
	if got, err := Compose(a, leg(t, "bybit", domain.SideBuy, "0")); err != nil || got != VerdictUnopened {
		t.Fatalf("Compose(flat, flat) = %s, %v; want VerdictUnopened, nil", got, err)
	}
}

// TestComposeRefusesSameSideLegs repeats Validate's opposite-side premise at
// the pure boundary. Compose is reached from two modes and can be called by
// code that never loaded Config; assuming the premise here lets two equal buys
// report VerdictIntact.
func TestComposeRefusesSameSideLegs(t *testing.T) {
	t.Parallel()
	a := leg(t, "binance", domain.SideBuy, "100")
	b := leg(t, "bybit", domain.SideBuy, "100")

	got, err := Compose(a, b)
	if !errors.Is(err, ErrLegSideSame) {
		t.Fatalf("Compose(two buys) = %s, %v; want VerdictIndeterminate, %v", got, err, ErrLegSideSame)
	}
	if got != VerdictIndeterminate {
		t.Errorf("Compose(two buys) = %s, want VerdictIndeterminate", got)
	}
}

// TestExposedNamesEveryVerdict is the completeness check the exhaustiveswitch
// gate cannot make: it asserts what each member MEANS for a running system,
// so a member added later has to choose a side here too.
func TestExposedNamesEveryVerdict(t *testing.T) {
	t.Parallel()
	want := map[Verdict]bool{
		VerdictUnopened:      false,
		VerdictIntact:        false,
		VerdictHalfOpen:      true,
		VerdictSkewed:        true,
		VerdictIndeterminate: true,
	}
	for v, w := range want {
		if got := Exposed(v); got != w {
			t.Errorf("Exposed(%s) = %t, want %t", v, got, w)
		}
	}
	if len(want) != 5 {
		t.Fatalf("the Verdict roster has %d members here; add the new one to this table", len(want))
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/app/pair/ -run 'TestCompose|TestExposed' -v`
Expected: FAIL — the package does not exist.

- [ ] **Step 3: Write the implementation**

Create `internal/app/pair/errors.go`:

```go
package pair

import "errors"

var (
	// ErrPairFieldUnset and ErrPairFieldInvalid report a pair block this root
	// cannot drive — a missing field, and one that parses to no declared
	// value. Kept distinct on reconcile's ErrConfigFieldUnset precedent: the
	// operator's fix differs.
	ErrPairFieldUnset   = errors.New("pair: pair field is unset")
	ErrPairFieldInvalid = errors.New("pair: pair field is invalid")

	// ErrSideCount reports a document that does not describe exactly two
	// sides. One side is the exercise binary; three is a shape nothing in this
	// root's leg-index argument (D2) covers.
	ErrSideCount = errors.New("pair: exactly two sides are required")

	// ErrSideVenueDuplicate reports two sides naming the same venue.
	//
	// It is not a tidiness rule. Both legs plan under ONE TradeID at leg index
	// 0, so they mint the SAME ClientOrderID — safe only because no store spans
	// venues and the venue is what tells the two apart. Two sides on one venue
	// removes the disambiguator and puts two different orders under one id on
	// one venue's order page.
	ErrSideVenueDuplicate = errors.New("pair: both sides name the same venue")

	// ErrSideJournalShared reports two sides pointing at one journal path.
	// §5.1's journal is a single-writer store; two oms.Modules over one file is
	// two writers, and the second half of ErrSideVenueDuplicate's argument —
	// with one journal the two identical ids DO meet.
	//
	// Comparing the configured strings was the shallower check: it missed a
	// relative/absolute alias, a symlinked parent and an existing hard link, all
	// of which put the identical ids in one physical SQLite chain.
	ErrSideJournalShared = errors.New("pair: both sides name the same journal path")

	// ErrLegCount reports a pair that is not exactly one leg per side (D3).
	ErrLegCount = errors.New("pair: exactly one leg per side is required")

	// ErrLegVenueUnknown reports a leg naming a venue no side declares.
	ErrLegVenueUnknown = errors.New("pair: leg names a venue no side declares")

	// ErrLegInstrumentUnknown reports a leg naming an instrument its own side's
	// instrument table does not hold. Refused at Validate rather than at
	// planning, because the row is where the base and quote assets come from.
	ErrLegInstrumentUnknown = errors.New("pair: leg names an instrument its side does not declare")

	// ErrLegBaseMismatch reports two legs whose base assets differ. The pair's
	// whole arithmetic — equal filled quantities means intact — is a comparison
	// of two fixedpoint.Quantity values, which is a runtime asset-tag check;
	// refusing here names the config field instead of failing mid-run.
	ErrLegBaseMismatch = errors.New("pair: the two legs do not share a base asset")

	// ErrLegSideSame reports two legs on the same side of the market. A pair
	// whose legs are both buys is not a hedge, and every verdict below would
	// report it intact.
	ErrLegSideSame = errors.New("pair: the two legs are on the same side of the market")

	// ErrReferenceFieldUnset, ErrReferenceUnreadable, ErrFilterTickUnrecorded
	// and ErrFilterRowStale are exercise's four, restated for this root's own
	// message prefix. Their arguments are unchanged — see
	// internal/app/exercise/errors.go, which states each in full.
	ErrReferenceFieldUnset  = errors.New("pair: reference field is unset")
	ErrReferenceUnreadable  = errors.New("pair: reference revision could not be read")
	ErrFilterTickUnrecorded = errors.New("pair: reference row records no tick, which the order path verifies against")
	ErrFilterRowStale       = errors.New("pair: reference row is older than reference.max_age")

	// ErrBootNotClean reports a drive declined because a side's boot verdict was
	// not ExitClean. Both sides must be clean before either is touched: a
	// dispatch that began against one clean boot and one unknown one is the
	// half-open pair this root exists to avoid producing on purpose.
	ErrBootNotClean = errors.New("pair: a side's boot verdict does not admit execution")

	// ErrPositionNotFlat reports a clean boot whose venue still holds a position
	// on one of this pair's instruments. This root's only compensating trade is
	// a reduce-only inverse; it is legal as restoration only when the opening
	// leg created the entire position it later closes.
	ErrPositionNotFlat = errors.New("pair: a leg's venue position is not flat before execution")

	// ErrHedgeUnsizable reports leg A's established fill failing leg B's own
	// §2.4 filters — below B's min quantity or min notional, or off its lot
	// grid. Nothing is sent on B and the run unwinds A (D5).
	ErrHedgeUnsizable = errors.New("pair: side A's fill cannot be expressed as a leg on side B")

	// ErrUnwindRefused reports an unwind this root declined to attempt. A
	// compensating trade is §6.7's gate's business and §6.7 cannot see a
	// cross-venue pair (record §95 §4), so this root compensates only on the
	// one reading it fully established: a HalfOpen pair whose exposed side is
	// side A. Every other reading ends at ExitExposed, for a human.
	ErrUnwindRefused = errors.New("pair: the pair's verdict does not admit an automatic unwind")
)
```

Create `internal/app/pair/pair.go`:

```go
// Package pair is the composition root for a cross-venue perp-perp package:
// two single-venue OMS deployments, two journals, two boots, and ONE trade
// spanning them.
//
// # The concept this file names, and why it is not in the kernel
//
// Record §95 §4 found that §6.7's classifier cannot see a cross-venue pair:
// domain.HedgeGroup is (Account, Venue, Symbol), the classifier computes its
// residual per hedge group, and a cross-venue pair sits in two groups with no
// group holding both. Inside each venue's own group a correctly-hedged pair
// reads as a filled leg with no pending counter-order — §6 step 7's DETERMINED
// residual, the branch that compensates within five seconds. Driven naively the
// safety kernel would flatten both halves of a healthy position, once per
// venue, in opposite directions.
//
// Ruling 2 places the fix in a NEW concept ABOVE the hedge group and leaves
// §5.1's key alone: the venue in that key is correct for margin, liquidation
// and ADL, and §5.1 keys remedy episodes by hedge group so §7.1's whole ladder
// rides the same object. What is actually needed is not a merged residual but
// the fact that A LEG HAS A PARTNER ELSEWHERE. That fact is what this file
// holds, and ruling 3 puts the normative edit to §5.1/§6.7 in task #241, not
// here.
//
// # What that costs, stated rather than left to be discovered
//
// This root COMPOSES two per-venue readings and refuses to conclude on any it
// could not establish. It does not make the pair a kernel concept, so nothing
// in the safety kernel knows about it, and §6.7 stays the single gate for every
// compensating trade that goes through the kernel. The one compensating trade
// this root issues — the unwind — is therefore deliberately narrow: it runs on
// exactly one verdict, and every other reading is handed to a human. See
// ErrUnwindRefused.
package pair

import (
	"fmt"

	"github.com/giiorg/arb-lab/internal/oms"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// JoinFacts is what one side's join reported about one leg: the three lists
// that mean "this reading is not finished".
//
// They are passed in rather than read here because the join is I/O and this
// file is not. Each is TRUE when the leg's id appeared in that list.
type JoinFacts struct {
	// Absent: the venue says it does not hold the order. Not a failure and not
	// a resolution — §5.3's stable-not-found rule decides what an absence
	// licenses, through four gates this root does not implement.
	Absent bool

	// Incomplete: the executions sweep did not prove coverage through the
	// instant it was asked about. A truncated sweep's silence is not evidence.
	Incomplete bool

	// Unattributed: the venue's own cumulative disagrees with what the join
	// could admit. An execution exists that was not attributed to this order,
	// and the exposure it represents is real whether or not its row can be
	// named.
	Unattributed bool
}

// LegExposure is one side's leg as this root reads it.
type LegExposure struct {
	Venue      ids.Venue
	ID         ids.ClientOrderID
	Instrument ids.VenueInstrument
	Side       domain.Side
	State      domain.OrderState

	// Filled is the established executed quantity, meaningful ONLY when
	// Established is true.
	Filled fixedpoint.Quantity

	// Established reports whether Filled is a measurement. When false, Filled
	// is whatever the projection happened to hold and MUST NOT be read: a zero
	// cumulative on a leg that has not reached a terminal state means "nothing
	// attributed yet", which is not "nothing filled", and the difference is a
	// naked position reported as a clean run.
	Established bool

	// Because names why Established is false, for the operator's report.
	Because string
}

// Establish is the three-valued reading D4 requires: a leg's exposure counts as
// measured only when the order is terminal for STATE and the join reported none
// of its three unfinished-reading facts about it.
//
// The direction is the whole point. Every one of these conditions, taken the
// other way, produces a confident zero — and a confident zero on a leg that
// actually filled is the reading that leaves naked exposure looking clean.
func Establish(venue ids.Venue, id ids.ClientOrderID, st oms.LegState, joined JoinFacts) LegExposure {
	e := LegExposure{
		Venue: venue, ID: id,
		Instrument: st.Instrument, Side: st.Side, State: st.State,
		Filled: st.Cumulative,
	}
	terminal, err := st.State.IsTerminal()
	switch {
	case err != nil:
		e.Because = fmt.Sprintf("state %q: %v", st.State, err)
	case !terminal:
		e.Because = fmt.Sprintf("state %q is not terminal", st.State)
	case joined.Absent:
		e.Because = "the venue does not hold this order; §5.3's absence rule has not been run"
	case joined.Incomplete:
		e.Because = "the executions sweep did not prove coverage"
	case joined.Unattributed:
		e.Because = "the venue's cumulative disagrees with the attributed fills"
	default:
		e.Established = true
	}
	return e
}

// Verdict is the pair's classification — the concept record §95 ruling 2 places
// above the hedge group.
//
// It is NOT §6.7's verdict and must not be confused with one. §6.7 answers
// "determined or ambiguous" about ONE hedge group's residual and authorizes a
// compensating magnitude. This answers "what does this cross-venue pair look
// like from outside both groups", and authorizes nothing on its own.
type Verdict uint8

const (
	// VerdictUnopened : neither leg carries exposure. The IOC-did-not-fill
	// case, and a real result — it proves both boots, both wire contracts and
	// both journal paths, and proves nothing about fills.
	VerdictUnopened Verdict = iota

	// VerdictIntact : both legs carry the same base quantity. The pair is
	// hedged, which is the outcome the strategy wants.
	VerdictIntact

	// VerdictHalfOpen : exactly one leg carries exposure, and its magnitude is
	// established. This is the one reading this root will act on.
	VerdictHalfOpen

	// VerdictSkewed : both legs carry exposure and the quantities differ. The
	// pair is partially hedged; the residual is real and its remedy is a
	// compensating trade on one side, which is §6.7's gate's business and not
	// this root's (record §95 §4). An operator matter.
	VerdictSkewed

	// VerdictIndeterminate : at least one leg's exposure was not established.
	// It OUTRANKS every other reading, including one that would otherwise look
	// intact, because an unestablished leg's quantity is not a measurement and
	// a rule that let the other side decide would conclude on it.
	VerdictIndeterminate
)

// String implements fmt.Stringer.
func (v Verdict) String() string {
	switch v {
	case VerdictUnopened:
		return "VerdictUnopened"
	case VerdictIntact:
		return "VerdictIntact"
	case VerdictHalfOpen:
		return "VerdictHalfOpen"
	case VerdictSkewed:
		return "VerdictSkewed"
	case VerdictIndeterminate:
		return "VerdictIndeterminate"
	}
	return fmt.Sprintf("Verdict(%d)", uint8(v))
}

// Exposed reports whether this verdict leaves exposure a human must account
// for. It is what the exit class is derived from.
func Exposed(v Verdict) bool {
	switch v {
	case VerdictUnopened, VerdictIntact:
		return false
	case VerdictHalfOpen, VerdictSkewed, VerdictIndeterminate:
		return true
	}
	// Named rather than defaulted: a member added above must choose a side
	// here, and choosing "not exposed" by falling through is how a real
	// exposure becomes invisible. exhaustiveswitch enforces the naming; this
	// line is only the compiler's return.
	return true
}

// Compose is the pair's composition rule.
//
// # Indeterminate is checked first, and the order is the safety argument
//
// Every other arm reads a quantity, and a quantity from an unestablished leg is
// not a number this system measured.
//
// # It is fallible because fixedpoint made the same decision one layer down
//
// Quantity.Sign is fallible so that "a flat position and a position nobody
// populated" cannot be confused, and at this layer that confusion is a naked
// position reading as no position. An unreadable quantity therefore returns
// VerdictIndeterminate WITH the error: fail-closed, and the caller has a reason
// to print rather than a bare verdict.
//
// # It repeats the opposite-side premise
//
// Config.Validate is not this function's only possible caller. Two equal buys
// compare equal just as two opposite fills do, so assuming the config check
// here turns a directional package into VerdictIntact.
func Compose(a, b LegExposure) (Verdict, error) {
	if !a.Established || !b.Established {
		return VerdictIndeterminate, nil
	}
	// Validate refuses same-side documents, and Compose repeats that premise
	// rather than inheriting one caller's discipline. Without it two equal buys
	// compare equal and become VerdictIntact — a same-direction package reported
	// as a hedge.
	//
	// It is checked AFTER the Established guard and not before, because Side is
	// a field an UNESTABLISHED reading may not carry: drive.establish's error
	// path returns a LegExposure holding only Venue, ID and Because, so its Side
	// is the zero value. Two such legs would compare equal and report a document
	// defect in place of the unfinished reading that actually stopped the run —
	// a wrong diagnosis on the exact path D4 exists to keep honest.
	if a.Side == b.Side {
		return VerdictIndeterminate, fmt.Errorf("pair legs are both %s: %w", a.Side, ErrLegSideSame)
	}
	aSign, err := a.Filled.Sign()
	if err != nil {
		return VerdictIndeterminate, fmt.Errorf("side %s filled quantity: %w", a.Venue, err)
	}
	bSign, err := b.Filled.Sign()
	if err != nil {
		return VerdictIndeterminate, fmt.Errorf("side %s filled quantity: %w", b.Venue, err)
	}
	aOpen, bOpen := aSign != 0, bSign != 0
	switch {
	case !aOpen && !bOpen:
		return VerdictUnopened, nil
	case aOpen != bOpen:
		return VerdictHalfOpen, nil
	}
	// Cmp is the asset-tag check as well as the comparison: two legs whose bases
	// disagree cannot be compared, and Config.Validate refuses that document at
	// load. Checked here anyway rather than assumed — the config refusal is one
	// caller's discipline, and this function is reached from two modes.
	cmp, err := a.Filled.Cmp(b.Filled)
	if err != nil {
		return VerdictIndeterminate, fmt.Errorf("comparing %s and %s fills: %w", a.Venue, b.Venue, err)
	}
	if cmp == 0 {
		return VerdictIntact, nil
	}
	return VerdictSkewed, nil
}
```

Note what is NOT used: there is no `IsZero` and no `Equal` on `fixedpoint.Quantity`, and `==` on it would compare an unexported mantissa and asset tag while silently succeeding on two differently-scaled representations. `Sign` and `Cmp` are the surface, and both are fallible on purpose.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test -race ./internal/app/pair/ -run 'TestCompose|TestExposed' -v`
Expected: PASS, all eight `TestComposeVerdicts` subtests and `TestExposedNamesEveryVerdict`.

- [ ] **Step 5: Bite-check the two claims the collapse checklist protects**

```bash
# (1) Intact must require EQUAL quantities, not merely two non-zero ones.
tools/mutate.sh internal/app/pair/pair.go <line-of-a.Filled.Equal> \
  's/if cmp == 0 {/if aOpen \&\& bOpen {/' \
  'Intact on any two non-zero fills' ./internal/app/pair/

# (2) Indeterminate must outrank everything.
tools/mutate.sh internal/app/pair/pair.go <line-of-the-Established-guard> \
  's/if !a.Established || !b.Established {/if !a.Established \&\& !b.Established {/' \
  'Indeterminate only when BOTH legs are unestablished' ./internal/app/pair/
tools/mutate.sh internal/app/pair/pair.go <line-of-the-side-opposition-guard> \
  's/if a.Side == b.Side {/if false {/' \
  'Compose assumes Validate enforced opposite sides' ./internal/app/pair/
```

Expected verdicts: `caught`, `caught`, `caught`. Mutation (1) is the one the 100-vs-60 fixture exists for — if the table had used equal quantities everywhere it would `SURVIVE`.

- [ ] **Step 6: Commit**

```bash
git add internal/app/pair/pair.go internal/app/pair/errors.go internal/app/pair/pair_test.go
git commit -m "pair: the cross-venue pair concept, above the hedge group

Record §95 ruling 2: §5.1's (Account, Venue, Symbol) key is right for
margin, liquidation and ADL and stays untouched; what a cross-venue pair
needs is the fact that a leg has a partner elsewhere. Verdict composes
two per-venue readings and Establish makes the reading three-valued —
exposure counts as measured only on a terminal state with a clean join,
because a confident zero on a leg that filled is naked exposure that
reads as a clean run.

The verdict table fills 100 against 60 on purpose: with equal fills on
both sides, 'both non-zero' and 'equal' are one predicate."
```

---

## Task 3: the document — `Config` and its refusals

**Files:**
- Create: `internal/app/pair/config.go`
- Test: `internal/app/pair/config_test.go`

**Interfaces:**
- Consumes: `reconcile.Config` (§5.2's per-OMS block, unchanged — this root holds no second statement of it, exercise's rule), `application.LegIntent`, `application.TradePlan`, `pair` errors from task 2.
- Produces:
  ```go
  type Config struct {
      Sides     []SideConfig    `json:"sides"`
      Pair      PairConfig      `json:"pair"`
      Reference ReferenceConfig `json:"reference"`
  }
  type SideConfig struct{ OMS reconcile.Config `json:"oms"` }
  type ReferenceConfig struct {
      Path   string          `json:"path"`
      MaxAge config.Duration `json:"max_age"`
  }
  type Mode string
  const (
      ModeSequential Mode = "sequential"
      ModePaired     Mode = "paired"
  )
  type PairConfig struct {
      ID      string          `json:"id"`
      Attempt int             `json:"attempt"`
      Mode    Mode            `json:"mode"`
      Settle  config.Duration `json:"settle"`
      SettleAttempts int      `json:"settle_attempts"`
      Legs    []LegConfig     `json:"legs"`
  }
  type LegConfig struct {
      Venue            ids.Venue           `json:"venue"`
      Product          ids.ProductCategory `json:"product"`
      Contract         ids.ContractType    `json:"contract,omitempty"`
      Symbol           string              `json:"symbol"`
      Side             string              `json:"side"`
      Quantity         string              `json:"quantity"`
      LimitPrice       string              `json:"limit_price"`
      UnwindLimitPrice string              `json:"unwind_limit_price"`
  }
  func (c Config) Validate() error
  func (c Config) sideFor(v ids.Venue) (int, bool)
  func (c Config) legIntent(i int) (application.LegIntent, error)
  func (c Config) openPlan(i int, quantity fixedpoint.Quantity) (application.TradePlan, error)
  func (c Config) unwindPlan(i int, filled fixedpoint.Quantity) (application.TradePlan, error)
  ```

- [ ] **Step 1: Write the failing test**

Create `internal/app/pair/config_test.go`. Build `validConfig(t)` by **copying `internal/app/exercise/app_test.go`'s `reconcileConfig`, `writeReferenceTicks`, `ref` and `dur` helpers and adapting them**: two sides (binance + bybit), linear-perp only (no spot leg, so no `SpotSettle` and no `SpotBaseline`), symbol `LABUSDT`, base `LAB`, and `ModeSequential`. Open that file and copy from it rather than inventing a second fixture shape.

```go
// TestValidConfigValidates is the positive control every refusal below needs.
// Without it a universally-objecting Validate reads as a perfect score.
func TestValidConfigValidates(t *testing.T) {
	t.Parallel()
	if err := validConfig(t).Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
}

func TestConfigRefusals(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		bend func(*Config)
		want error
	}{
		{"one side", func(c *Config) { c.Sides = c.Sides[:1] }, ErrSideCount},
		{"three sides", func(c *Config) { c.Sides = append(c.Sides, c.Sides[0]) }, ErrSideCount},
		{
			// D2's premise, as a refusal. Both legs mint the same
			// ClientOrderID; the venue is what tells them apart.
			name: "both sides on one venue",
			bend: func(c *Config) {
				c.Sides[1].OMS.Venue.Venue = c.Sides[0].OMS.Venue.Venue
			},
			want: ErrSideVenueDuplicate,
		},
		{
			// The second half of the same premise: with one journal the two
			// identical ids DO meet, and §5.1's store is single-writer.
			name: "both sides on one journal",
			bend: func(c *Config) { c.Sides[1].OMS.JournalPath = c.Sides[0].OMS.JournalPath },
			want: ErrSideJournalShared,
		},
		{
			// A raw string comparison misses the same path spelled once relative
			// and once absolute; both modules would write one physical database.
			name: "relative and absolute aliases of one journal",
			bend: aliasJournalRelativeAndAbsolute(t),
			want: ErrSideJournalShared,
		},
		{
			// EvalSymlinks is what makes a different parent spelling stop being a
			// different journal identity.
			name: "symlink aliases of one journal",
			bend: aliasJournalThroughSymlink(t),
			want: ErrSideJournalShared,
		},
		{
			// Two existing names may resolve without symlinks and still be one
			// inode. os.SameFile is the check the spelling cannot supply.
			name: "hard-link aliases of one journal",
			bend: aliasJournalThroughHardLink(t),
			want: ErrSideJournalShared,
		},
		{"three legs", func(c *Config) { c.Pair.Legs = append(c.Pair.Legs, c.Pair.Legs[0]) }, ErrLegCount},
		{"one leg", func(c *Config) { c.Pair.Legs = c.Pair.Legs[:1] }, ErrLegCount},
		{
			name: "two legs on one side",
			bend: func(c *Config) { c.Pair.Legs[1].Venue = c.Pair.Legs[0].Venue },
			want: ErrLegCount,
		},
		{"leg on an undeclared venue", func(c *Config) { c.Pair.Legs[1].Venue = ids.Venue("kraken") }, ErrLegVenueUnknown},
		{"leg on an undeclared instrument", func(c *Config) { c.Pair.Legs[1].Symbol = "DOGEUSDT" }, ErrLegInstrumentUnknown},
		{"both legs buy", func(c *Config) { c.Pair.Legs[0].Side = "buy"; c.Pair.Legs[1].Side = "buy" }, ErrLegSideSame},
		{"unset trade id", func(c *Config) { c.Pair.ID = "" }, ErrPairFieldUnset},
		{"malformed trade id", func(c *Config) { c.Pair.ID = "not-a-uuid" }, ErrPairFieldInvalid},
		{"unset mode", func(c *Config) { c.Pair.Mode = "" }, ErrPairFieldUnset},
		{"unknown mode", func(c *Config) { c.Pair.Mode = Mode("parallel-ish") }, ErrPairFieldInvalid},
		{"negative attempt", func(c *Config) { c.Pair.Attempt = -1 }, ErrPairFieldInvalid},
		{
			// D7: the unwind mints Attempt+1, and 99 is ids.maxAttempt.
			name: "attempt at the id format's ceiling leaves no room to unwind",
			bend: func(c *Config) { c.Pair.Attempt = 99 },
			want: ErrPairFieldInvalid,
		},
		{"unset settle", func(c *Config) { c.Pair.Settle = config.Duration{} }, ErrPairFieldUnset},
		{"zero settle attempts", func(c *Config) { c.Pair.SettleAttempts = 0 }, ErrPairFieldUnset},
		{
			// Fail-closed: if you cannot state the price at which you would
			// close a leg, you may not open it.
			name: "leg with no unwind limit price",
			bend: func(c *Config) { c.Pair.Legs[0].UnwindLimitPrice = "" },
			want: ErrPairFieldUnset,
		},
		{"unset reference path", func(c *Config) { c.Reference.Path = "" }, ErrReferenceFieldUnset},
		{"unset reference max_age", func(c *Config) { c.Reference.MaxAge = config.Duration{} }, ErrReferenceFieldUnset},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := validConfig(t)
			tc.bend(&c)
			err := c.Validate()
			if !errors.Is(err, tc.want) {
				t.Fatalf("Validate() = %v, want %v", err, tc.want)
			}
		})
	}
}

// TestLegBaseMismatchIsRefused needs its own fixture rather than a bend,
// because it takes a second instrument row with a different base asset.
func TestLegBaseMismatchIsRefused(t *testing.T) {
	t.Parallel()
	c := validConfig(t)
	// Give side B an ETHUSDT row and point its leg at it: same quote, DIFFERENT
	// base, so the pair's quantity comparison would be an asset-tag mismatch at
	// runtime rather than a named config refusal.
	c.Sides[1].OMS.Instruments = append(c.Sides[1].OMS.Instruments, reconcile.InstrumentConfig{
		Venue: c.Sides[1].OMS.Venue.Venue, Product: ids.ProductLinearPerp,
		Contract: ids.ContractLinear, Symbol: "ETHUSDT", Base: "ETH", Quote: "USDT", Leverage: 5,
	})
	c.Pair.Legs[1].Symbol = "ETHUSDT"
	if err := c.Validate(); !errors.Is(err, ErrLegBaseMismatch) {
		t.Fatalf("Validate() = %v, want %v", err, ErrLegBaseMismatch)
	}
}

// TestOpenPlanUsesTheGivenQuantityAndNotTheDocument holds D5: leg B is sized to
// leg A's established fill. A plan that read the document's quantity would be
// right for leg A and would hedge a partial fill at full size on leg B.
func TestOpenPlanUsesTheGivenQuantityAndNotTheDocument(t *testing.T) {
	t.Parallel()
	c := validConfig(t) // both legs state quantity "100"
	got, err := c.openPlan(1, qty(t, "60"))
	if err != nil {
		t.Fatalf("openPlan(1, 60) = %v, want nil", err)
	}
	if len(got.Legs) != 1 {
		t.Fatalf("openPlan produced %d legs, want 1", len(got.Legs))
	}
	eqQty(t, got.Legs[0].Quantity, qty(t, "60"))
	if got.Legs[0].ReduceOnly {
		t.Error("ReduceOnly = true on an opening leg, want false")
	}
	if got.Attempt != c.Pair.Attempt {
		t.Errorf("Attempt = %d, want %d", got.Attempt, c.Pair.Attempt)
	}
}

// TestUnwindPlanInvertsTheSideAndAdvancesTheAttempt holds D7. The three claims
// are separated deliberately: a plan that got the side right and the
// reduce-only flag wrong opens fresh exposure while looking like a close.
func TestUnwindPlanInvertsTheSideAndAdvancesTheAttempt(t *testing.T) {
	t.Parallel()
	c := validConfig(t) // leg 0 is a SELL at limit_price, unwind at unwind_limit_price
	got, err := c.unwindPlan(0, qty(t, "60"))
	if err != nil {
		t.Fatalf("unwindPlan(0, 60) = %v, want nil", err)
	}
	leg := got.Legs[0]
	if leg.Side != domain.SideBuy {
		t.Errorf("Side = %v, want %v (the opposite of the opening leg's sell)", leg.Side, domain.SideBuy)
	}
	if !leg.ReduceOnly {
		t.Error("ReduceOnly = false, want true — an unwind that is not reduce-only can open a position")
	}
	// The established fill of 60, not the document's 100.
	eqQty(t, leg.Quantity, qty(t, "60"))
	if got.Attempt != c.Pair.Attempt+1 {
		t.Errorf("Attempt = %d, want %d — §5.3 forbids reusing the opening leg's id",
			got.Attempt, c.Pair.Attempt+1)
	}
	wantPrice, err := fixedpoint.ParsePrice("LAB", "USDT", c.Pair.Legs[0].UnwindLimitPrice)
	if err != nil {
		t.Fatalf("ParsePrice: %v", err)
	}
	// The leg's unwind_limit_price, which the fixture makes DIFFER from its
	// limit_price — with the two equal, "uses the limit" and "uses the unwind
	// limit" are the same assertion.
	eqPrice(t, leg.LimitPrice, wantPrice)
}
```

The three alias fixtures create their paths under `t.TempDir()` and clean up through the test. The existing plain-string case passes under all three mutations — relative/absolute, symlink and hard link have different strings — which is why none of them may be replaced by that case.

**The relative/absolute and symlink fixtures must NOT pre-create the database files, and that is the load-bearing detail.** A journal that already exists is the *second* boot; the first boot is the one where it does not, and `filepath.EvalSymlinks` refuses a path whose final component is missing. A fixture that touched the `.db` files first would resolve through a different code path and pass against an implementation that cannot fire in production — this repo's recorded shape of a check whose refusing path is the untested one. The hard-link fixture is the exception and must create both files, because a hard link to a nonexistent file cannot be made and `os.SameFile` needs two `FileInfo`s. Add a fourth case as the negative control: two genuinely distinct journals in one directory must validate, or a `sameJournal` that returned true unconditionally would pass all three.

Note the fixture requirement that makes the last test bite: **leg 0's `limit_price` and `unwind_limit_price` must differ**, or "uses the limit price" and "uses the unwind limit price" are the same assertion. Make them `"0.0840"` and `"0.0860"`.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/app/pair/ -run 'TestValidConfig|TestConfigRefusals|TestLegBase|TestOpenPlan|TestUnwindPlan' -v`
Expected: FAIL — `undefined: Config`, `undefined: validConfig`.

- [ ] **Step 3: Write the implementation**

Create `internal/app/pair/config.go`. Structure it on `internal/app/exercise/config.go` — read that file first; `legIntent` there is the shared half of `Validate` and `plan`, and keeping that property means a document that validated cannot fail to plan for a reason validation could have named. Preserve it here.

```go
package pair

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/giiorg/arb-lab/internal/app/reconcile"
	"github.com/giiorg/arb-lab/internal/oms/application"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/config"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// Config is the cross-venue root's document: TWO §5.2 OMS blocks, and the one
// pair they jointly execute.
//
// # Why two blocks rather than one with two venues
//
// reconcile.Config's own comment settles it: "Venue is §5.2's per-venue block.
// ONE, singular … oms.Deps takes one venue client, so a second block here would
// have nowhere to go. A deployment trading two venues runs two of these,
// against two journals." So a cross-venue package is not a configuration of the
// existing binary (record §95 §3); it is this root, holding two of them.
//
// The schema is reconcile's, unchanged and mounted under a name — exercise's
// rule and its reason: two schemas for one block is the copy that drifts, and
// the drift would surface as a run that reconciles differently from the
// recovery run over the same journal.
//
// [SPEC EDIT OWED] §12 item 7 describes strategy #2's execution half as
// "`strategy/crossvenue` live wiring over the milestone-3 OMS + milestone-4
// controls". "Live wiring" reads as connection work and does not say that the
// composition root is single-venue by design or that the hedge group cannot
// hold two venues. This document shape is the answer to both; §12 item 7 and
// §5.2 need a sentence naming the two-deployment shape. Owned by task #241
// alongside §5.1/§6.7's re-scope — see record §95 §8.
type Config struct {
	// Sides are the two deployments, exactly two, on two venues, over two
	// journals. See ErrSideVenueDuplicate and ErrSideJournalShared: neither
	// rule is tidiness, and both are load-bearing for the shared trade id.
	Sides []SideConfig `json:"sides"`

	// Pair is the package these two sides jointly execute.
	Pair PairConfig `json:"pair"`

	// Reference names the committed reference revision both sides resolve their
	// §2.4 trading filters from, and how old a row may be. ONE revision for
	// both venues, deliberately: the two legs' grids are compared against each
	// other implicitly every time a fill on one is sized as an order on the
	// other, and two revisions of different ages would make that comparison
	// span two observation instants.
	Reference ReferenceConfig `json:"reference"`
}

// SideConfig is one deployment: §5.2's block exactly as `arblab reconcile run`
// reads it.
type SideConfig struct {
	OMS reconcile.Config `json:"oms"`
}

// ReferenceConfig locates the reference revision and bounds its age.
type ReferenceConfig struct {
	Path   string          `json:"path"`
	MaxAge config.Duration `json:"max_age"`
}

// Mode selects the pair's execution surface.
type Mode string

const (
	ModeSequential Mode = "sequential"
	ModePaired     Mode = "paired"
)

// PairConfig is one cross-venue package.
type PairConfig struct {
	// ID is the ids.TradeID BOTH legs plan under, and the sharing is the
	// design (D2). ClientOrderID encodes (Trade, Leg, Attempt) and carries no
	// venue, so both legs mint the same id — which is safe because no store
	// spans venues, and which is what makes "this leg has a partner elsewhere"
	// durable in both journals and on both venues' order pages with no change
	// to the kernel. The two refusals that hold it up are
	// ErrSideVenueDuplicate and ErrSideJournalShared.
	ID string `json:"id"`

	// Attempt distinguishes a replan of this pair. It is bounded BELOW the id
	// format's ceiling because the unwind mints Attempt+1 (D7).
	Attempt int `json:"attempt"`

	// Mode is required because sequential and paired execution have different
	// failure surfaces. A default would make that choice on an absent field.
	Mode Mode `json:"mode"`

	// Settle and SettleAttempts bound the exposure reading. After a leg's IOC
	// this root re-joins that leg to venue truth until the order is terminal
	// for state, waiting Settle between attempts and giving up after
	// SettleAttempts — at which point the leg is INDETERMINATE and the run
	// stops for a human.
	//
	// Both are required and neither may be zero. A zero here would make the
	// first read the only read, and the first read after an IOC is exactly when
	// a venue has not yet published the trade rows — which is the reading that
	// reports a filled leg as unfilled.
	Settle         config.Duration `json:"settle"`
	SettleAttempts int             `json:"settle_attempts"`

	// Legs are the pair's two orders, one per side. The FIRST is the leg this
	// root opens first and the only one it will unwind (D8); order in this
	// slice is therefore a decision, not a presentation.
	Legs []LegConfig `json:"legs"`
}

// LegConfig is one leg, bound to its side by venue.
type LegConfig struct {
	// Venue selects the side. There is no separate side name: the OMS block
	// already carries exactly one venue and the two must differ, so the venue
	// IS the side's identity and a leg cannot be bound to the wrong block by a
	// label going stale.
	Venue    ids.Venue           `json:"venue"`
	Product  ids.ProductCategory `json:"product"`
	Contract ids.ContractType    `json:"contract,omitempty"`
	Symbol   string              `json:"symbol"`

	// Side is "buy" or "sell". The two legs must differ — a pair whose legs are
	// both buys is not a hedge, and every verdict would report it intact.
	Side string `json:"side"`

	// Quantity is the ordered size in the BASE asset for the FIRST leg opened.
	// The second leg is sized to the first's ESTABLISHED FILL (D5), so its
	// quantity here is the operator's statement of intent and the ceiling the
	// document was validated against — never what is actually sent.
	Quantity string `json:"quantity"`

	// LimitPrice is §6 step 6's marketable limit for the opening order.
	LimitPrice string `json:"limit_price"`

	// UnwindLimitPrice is the marketable limit for the reduce-only close of
	// this leg. REQUIRED on every leg, and the requirement is fail-closed: an
	// operator who cannot state the price at which they would close a leg is
	// not ready to open it, and a defaulted unwind price is a compensating
	// order priced by whoever wrote the default.
	UnwindLimitPrice string `json:"unwind_limit_price"`
}
```

Then `Validate`, in this order — sides before pair, because a leg cannot be resolved against a side list that does not hold:

```go
func (c Config) Validate() error {
	if len(c.Sides) != 2 {
		return fmt.Errorf("sides: %d declared: %w", len(c.Sides), ErrSideCount)
	}
	for i := range c.Sides {
		if err := c.Sides[i].OMS.Validate(); err != nil {
			return fmt.Errorf("sides[%d].oms: %w", i, err)
		}
	}
	if c.Sides[0].OMS.Venue.Venue == c.Sides[1].OMS.Venue.Venue {
		return fmt.Errorf("sides both name venue %s: %w", c.Sides[0].OMS.Venue.Venue, ErrSideVenueDuplicate)
	}
	shared, err := sameJournal(c.Sides[0].OMS.JournalPath, c.Sides[1].OMS.JournalPath)
	if err != nil {
		return fmt.Errorf("resolve side journal identity: %w", err)
	}
	if shared {
		return fmt.Errorf("sides name one physical journal (%s, %s): %w",
			c.Sides[0].OMS.JournalPath, c.Sides[1].OMS.JournalPath, ErrSideJournalShared)
	}

	if c.Reference.Path == "" {
		return fmt.Errorf("reference.path: %w", ErrReferenceFieldUnset)
	}
	if c.Reference.MaxAge.Duration() <= 0 {
		return fmt.Errorf("reference.max_age: %w", ErrReferenceFieldUnset)
	}

	if c.Pair.ID == "" {
		return fmt.Errorf("pair.id: %w", ErrPairFieldUnset)
	}
	if _, err := ids.ParseTradeID(c.Pair.ID); err != nil {
		return fmt.Errorf("pair.id: %v: %w", err, ErrPairFieldInvalid)
	}
	// maxAttemptForUnwind is ids' own ceiling less the one the unwind mints.
	// Refused here rather than discovered when the unwind is planned, which is
	// after leg A is live at a venue.
	if c.Pair.Attempt < 0 || c.Pair.Attempt >= 99 {
		return fmt.Errorf("pair.attempt %d (0..98; the unwind mints attempt+1): %w",
			c.Pair.Attempt, ErrPairFieldInvalid)
	}
	if c.Pair.Mode == "" {
		return fmt.Errorf("pair.mode: %w", ErrPairFieldUnset)
	}
	if c.Pair.Mode != ModeSequential && c.Pair.Mode != ModePaired {
		return fmt.Errorf("pair.mode %q: %w", c.Pair.Mode, ErrPairFieldInvalid)
	}
	if c.Pair.Settle.Duration() <= 0 {
		return fmt.Errorf("pair.settle: %w", ErrPairFieldUnset)
	}
	if c.Pair.SettleAttempts <= 0 {
		return fmt.Errorf("pair.settle_attempts: %w", ErrPairFieldUnset)
	}
	if len(c.Pair.Legs) != 2 {
		return fmt.Errorf("pair.legs: %d declared: %w", len(c.Pair.Legs), ErrLegCount)
	}
	if c.Pair.Legs[0].Venue == c.Pair.Legs[1].Venue {
		return fmt.Errorf("pair.legs both name venue %s: %w", c.Pair.Legs[0].Venue, ErrLegCount)
	}
	for i := range c.Pair.Legs {
		if c.Pair.Legs[i].UnwindLimitPrice == "" {
			return fmt.Errorf("pair.legs[%d].unwind_limit_price: %w", i, ErrPairFieldUnset)
		}
		if _, err := c.legIntent(i); err != nil {
			return err
		}
		if _, err := c.unwindIntent(i, fixedpoint.Quantity{}); err != nil && !errors.Is(err, errUnwindQuantityDeferred) {
			return err
		}
	}
	a, err := c.legIntent(0)
	if err != nil {
		return err
	}
	b, err := c.legIntent(1)
	if err != nil {
		return err
	}
	if a.Side == b.Side {
		return fmt.Errorf("pair.legs are both %q: %w", c.Pair.Legs[0].Side, ErrLegSideSame)
	}
	if a.Quantity.Asset() != b.Quantity.Asset() {
		return fmt.Errorf("pair.legs bases %s and %s: %w",
			a.Quantity.Asset(), b.Quantity.Asset(), ErrLegBaseMismatch)
	}
	return nil
}
```

`sameJournal` compares identity, not spelling: call `filepath.Abs` for both paths, apply `filepath.EvalSymlinks` whenever that path resolves, and, when both final paths already exist, compare their `os.FileInfo` values with `os.SameFile` so hard links are refused too. A non-existent final database is still compared by its canonical absolute parent/name. Do not make a missing database an error — clean first boot creates it — but do propagate failures that prevent a path which exists from being identified.

```go
func sameJournal(a, b string) (bool, error) {
	pa, ia, err := journalIdentity(a)
	if err != nil {
		return false, err
	}
	pb, ib, err := journalIdentity(b)
	if err != nil {
		return false, err
	}
	if ia != nil && ib != nil && os.SameFile(ia, ib) {
		return true, nil
	}
	return pa == pb, nil
}

// journalIdentity resolves the PARENT directory and rejoins the base name.
//
// # Resolving the whole path is the version that cannot fire
//
// filepath.EvalSymlinks lstats every component INCLUDING the last, so on a
// database that does not exist yet it returns an IsNotExist error and no
// resolved path — measured, not assumed. A version that swallowed that error
// and kept the unresolved path would leave `<dir>/link/a.db` and
// `<dir>/real/a.db` comparing unequal, so the symlink refusal would never fire
// on a CLEAN FIRST BOOT, which is the only boot where the journal is absent and
// therefore the ordinary case. The gate would pass its own fixture only if that
// fixture pre-created the databases — the test would be green and the rule dead.
//
// The parent must exist, and a missing parent is returned as an error rather
// than absorbed: a journal directory nobody created is a document to fix, and
// treating it as "a path unlike any other path" is how two sides come to share
// one journal because neither could be resolved.
func journalIdentity(path string) (string, os.FileInfo, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", nil, err
	}
	dir, base := filepath.Split(abs)
	resolvedDir, err := filepath.EvalSymlinks(filepath.Clean(dir))
	if err != nil {
		return "", nil, fmt.Errorf("resolve journal directory %s: %w", dir, err)
	}
	final := filepath.Join(resolvedDir, base)
	info, err := os.Stat(final)
	if os.IsNotExist(err) {
		return final, nil, nil // a first boot creates it; that is not an alias
	}
	if err != nil {
		return "", nil, err
	}
	return final, info, nil
}
```

`legIntent(i)` mirrors `exercise.Config.legIntent` with two changes: the instrument's venue comes from `c.Pair.Legs[i].Venue`, and the instrument table searched is **that leg's own side's**. Write `sideFor` first:

```go
// sideFor finds the side index a venue names.
func (c Config) sideFor(v ids.Venue) (int, bool) {
	for i := range c.Sides {
		if c.Sides[i].OMS.Venue.Venue == v {
			return i, true
		}
	}
	return 0, false
}

// legIntent resolves one leg against ITS OWN SIDE's instrument table.
//
// It is the shared half of Validate, openPlan and unwindPlan — exercise's rule,
// which is what makes a document that validated unable to fail at planning for
// a reason validation could have named.
func (c Config) legIntent(i int) (application.LegIntent, error) {
	leg := c.Pair.Legs[i]
	side, ok := c.sideFor(leg.Venue)
	if !ok {
		return application.LegIntent{}, fmt.Errorf("pair.legs[%d] names venue %s: %w",
			i, leg.Venue, ErrLegVenueUnknown)
	}
	instrument := ids.VenueInstrument{
		Venue: leg.Venue, Product: leg.Product, Contract: leg.Contract, Symbol: leg.Symbol,
	}
	var row *reconcile.InstrumentConfig
	for j := range c.Sides[side].OMS.Instruments {
		if c.Sides[side].OMS.Instruments[j].Instrument() == instrument {
			row = &c.Sides[side].OMS.Instruments[j]
			break
		}
	}
	if row == nil {
		return application.LegIntent{}, fmt.Errorf("pair.legs[%d] names %s: %w",
			i, instrument, ErrLegInstrumentUnknown)
	}
	var s domain.Side
	switch leg.Side {
	case "buy":
		s = domain.SideBuy
	case "sell":
		s = domain.SideSell
	default:
		return application.LegIntent{}, fmt.Errorf("pair.legs[%d].side %q (want \"buy\" or \"sell\"): %w",
			i, leg.Side, ErrPairFieldInvalid)
	}
	quantity, err := fixedpoint.ParseQuantity(row.Base, leg.Quantity)
	if err != nil {
		return application.LegIntent{}, fmt.Errorf("pair.legs[%d].quantity: %v: %w", i, err, ErrPairFieldInvalid)
	}
	limit, err := fixedpoint.ParsePrice(row.Base, row.Quote, leg.LimitPrice)
	if err != nil {
		return application.LegIntent{}, fmt.Errorf("pair.legs[%d].limit_price: %v: %w", i, err, ErrPairFieldInvalid)
	}
	return application.LegIntent{
		Instrument: instrument, Side: s, Quantity: quantity, LimitPrice: limit,
	}, nil
}
```

`unwindIntent(i, filled)` is `legIntent(i)` with the side inverted, `ReduceOnly: true`, `LimitPrice` parsed from `UnwindLimitPrice`, and `Quantity` replaced by `filled`. Have it return `errUnwindQuantityDeferred` (an unexported sentinel) when `filled` is the zero `Quantity`, so `Validate` can use it to check the *price* without a quantity in hand.

Then the two plan builders — both single-leg, both under `c.Pair.ID`:

```go
// openPlan is the TradePlan for one leg's OPENING order, sized by the caller.
//
// The quantity is a PARAMETER and not the document's, and that is D5: leg B is
// sized to leg A's established fill, so a plan reading the document would be
// right for the first leg and would hedge a partial fill at full size on the
// second.
func (c Config) openPlan(i int, quantity fixedpoint.Quantity) (application.TradePlan, error) {
	trade, err := ids.ParseTradeID(c.Pair.ID)
	if err != nil {
		return application.TradePlan{}, fmt.Errorf("pair.id: %w", err)
	}
	intent, err := c.legIntent(i)
	if err != nil {
		return application.TradePlan{}, err
	}
	intent.Quantity = quantity
	return application.TradePlan{Trade: trade, Attempt: c.Pair.Attempt, Legs: []application.LegIntent{intent}}, nil
}

// unwindPlan is the TradePlan for the reduce-only close of leg i (D7): same
// trade, same leg index, attempt+1, opposite side, sized to the ESTABLISHED
// fill, at the leg's declared unwind limit.
//
// The attempt advance is §5.3's requirement and it is also the fail-safe: a
// second run of the same document mints ids the journal already holds, and the
// planner refuses the whole plan rather than sending a duplicate.
func (c Config) unwindPlan(i int, filled fixedpoint.Quantity) (application.TradePlan, error) {
	trade, err := ids.ParseTradeID(c.Pair.ID)
	if err != nil {
		return application.TradePlan{}, fmt.Errorf("pair.id: %w", err)
	}
	intent, err := c.unwindIntent(i, filled)
	if err != nil {
		return application.TradePlan{}, err
	}
	return application.TradePlan{Trade: trade, Attempt: c.Pair.Attempt + 1, Legs: []application.LegIntent{intent}}, nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test -race ./internal/app/pair/ -v`
Expected: PASS, every subtest of `TestConfigRefusals` plus the four named tests and task 2's.

- [ ] **Step 5: Bite-check the two refusals D2 rests on**

```bash
tools/mutate.sh internal/app/pair/config.go <line-of-the-venue-duplicate-check> \
  's/if c.Sides\[0\].OMS.Venue.Venue == c.Sides\[1\].OMS.Venue.Venue {/if false {/' \
  'two sides may share a venue' ./internal/app/pair/
tools/mutate.sh internal/app/pair/config.go <line-of-the-shared-journal-check> \
  's/if shared {/if false {/' \
  'two sides may share one canonical journal identity' ./internal/app/pair/
```

Expected: `caught`, `caught`.

- [ ] **Step 6: Commit**

```bash
git add internal/app/pair/config.go internal/app/pair/config_test.go
git commit -m "pair: the two-sided document, and the two refusals its trade id rests on

Both legs plan under one TradeID at leg index 0, so both mint the same
ClientOrderID. That is safe only because no store spans venues — so
'two sides may not share a venue' and 'two sides may not share a
journal' are enforced refusals rather than a comment. The unwind limit
price is required on every leg: an operator who cannot state the price
at which they would close a leg is not ready to open it."
```

---

## Task 4: `tradingFilters` across two sides

**Files:**
- Create: `internal/app/pair/filters.go`
- Test: `internal/app/pair/filters_test.go`

**Interfaces:**
- Consumes: `Config` (task 3), `strategydriven.Document`, `omsdomain.TradingFilters`.
- Produces: `func (c Config) tradingFilters(clk clock.Clock) ([]map[ids.VenueInstrument]omsdomain.TradingFilters, error)` — one map per side, indexed by side.

- [ ] **Step 1: Write the failing test**

```go
// TestTradingFiltersAreResolvedPerSide holds the shape the boot needs: one map
// per side, each holding ONLY that side's instruments.
//
// The fixture gives both sides a LABUSDT linear-perp row with DIFFERENT steps —
// "1" on side A and "0.1" on side B. Equal steps would make "resolved side A's
// row" and "resolved side B's row" the same observation, which is this repo's
// eleven-times shape: the fixture would not contain the thing under test.
func TestTradingFiltersAreResolvedPerSide(t *testing.T) {
	t.Parallel()
	c := validConfigWithSteps(t, "1", "0.1")
	wall := testWall(t)

	got, err := c.tradingFilters(wall)
	if err != nil {
		t.Fatalf("tradingFilters() = %v, want nil", err)
	}
	if len(got) != 2 {
		t.Fatalf("tradingFilters returned %d maps, want one per side", len(got))
	}
	a := ids.VenueInstrument{Venue: "binance", Product: ids.ProductLinearPerp, Contract: ids.ContractLinear, Symbol: "LABUSDT"}
	b := ids.VenueInstrument{Venue: "bybit", Product: ids.ProductLinearPerp, Contract: ids.ContractLinear, Symbol: "LABUSDT"}
	if _, ok := got[0][b]; ok {
		t.Error("side A's map holds side B's instrument; each side's planner must verify against its own grid")
	}
	if _, ok := got[1][a]; ok {
		t.Error("side B's map holds side A's instrument")
	}
	eqQty(t, got[0][a].Step, qtyStep(t, "1"))
	eqQty(t, got[1][b].Step, qtyStep(t, "0.1"))
}

func TestTradingFiltersRefuseAStaleRow(t *testing.T) {
	t.Parallel()
	c := validConfig(t)
	c.Reference.Path = writeCrossVenueReference(t, timeAgo(t, 48*time.Hour), "1", "0.1")
	c.Reference.MaxAge = dur(t, "24h")
	if _, err := c.tradingFilters(testWall(t)); !errors.Is(err, ErrFilterRowStale) {
		t.Fatalf("tradingFilters() = %v, want %v", err, ErrFilterRowStale)
	}
}

func TestTradingFiltersRefuseATicklessRow(t *testing.T) {
	t.Parallel()
	c := validConfig(t)
	c.Reference.Path = writeCrossVenueReferenceNoTick(t, timeAgo(t, time.Hour))
	if _, err := c.tradingFilters(testWall(t)); !errors.Is(err, ErrFilterTickUnrecorded) {
		t.Fatalf("tradingFilters() = %v, want %v", err, ErrFilterTickUnrecorded)
	}
}
```

Write `writeCrossVenueReference(t, observedAt, stepA, stepB string) string` on `exercise`'s `writeReferenceTicks` pattern, emitting **four** rows: binance/linear_perp and bybit/linear_perp fee rows, and binance/linear_perp and bybit/linear_perp lot rows for `LABUSDT` (base `LAB`, quote `USDT`, `min_notional` `"5"`, tick `"0.0001"`). Open `internal/app/exercise/app_test.go:189` and copy its structure.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/app/pair/ -run 'TestTradingFilters' -v`
Expected: FAIL, `c.tradingFilters undefined`.

- [ ] **Step 3: Write the implementation**

Create `internal/app/pair/filters.go` as `internal/app/exercise/filters.go` with an outer loop over sides. Read that file first — its doc comment carries the whole argument for why the adapter lives at the root that plans and not in `app/reconcile`, and that argument is unchanged here. The one addition worth a comment:

```go
// tradingFilters resolves BOTH sides' instruments from ONE reference revision.
//
// One revision for two venues is deliberate. Every time this root sizes a fill
// on one venue as an order on the other, it compares two grids implicitly — and
// two revisions of different ages would put that comparison across two
// observation instants, so a hedge could be refused by a grid that had moved
// while the other side's row still said it had not.
//
// The returned slice is indexed by SIDE, and each map holds only that side's
// instruments: each side's planner and executor verify against their own venue's
// grid, and a map holding the other venue's rows would let a leg validate
// against a filter no venue will judge it by.
func (c Config) tradingFilters(clk clock.Clock) ([]map[ids.VenueInstrument]omsdomain.TradingFilters, error) {
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test -race ./internal/app/pair/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/pair/filters.go internal/app/pair/filters_test.go
git commit -m "pair: per-side trading filters from one reference revision

One revision for two venues, because sizing a fill on one venue as an
order on the other compares two grids implicitly and two revisions of
different ages would span two observation instants. One map per side,
because each side's planner verifies against the grid its own venue
will judge the order by."
```

---

## Task 5: two boots, and the both-clean-and-flat gate

**Files:**
- Create: `internal/app/pair/report.go`
- Create: `internal/app/pair/app.go`
- Test: `internal/app/pair/app_test.go`

**Interfaces:**
- Consumes: `reconcile.NewRunner`, `reconcile.WithInstrumentFilters`, `Runner.Sequence/BootReport/Module/Halted/Checklist/RateLimit`, `Config.tradingFilters`, `Module.VenuePosition` (task 1).
- Produces:
  ```go
  type SideReport struct {
      Venue     ids.Venue
      Boot      reconcile.Report
      // RateLimit is refreshed after all drive and unwind traffic; Boot keeps
      // the pre-drive snapshot used for boot precedence.
      RateLimit ratelimit.Snapshot
      Open      *LegReport
      Unwind    *LegReport
      Exposure  LegExposure
  }
  type LegReport struct {
      ID       ids.ClientOrderID
      Refusal  error
      Result   application.LegResult
  }
  type Report struct {
      Sides   [2]SideReport
      Verdict Verdict
      Drove   bool
      Refusal error
  }
  func Run(ctx context.Context, snap config.Snapshot[Config], reg func() *observability.Registry,
           stderr io.Writer) (Report, error)
  ```

  `Run` builds its own `*clock.Wall` from both sides' calibrations (D9), so it does not take one — the CLI cannot supply a wall that knows only one venue's frame.

  `Attempted` is dropped. A non-nil `*LegReport` is the reader for "this leg was attempted", and `Refusal` distinguishes attempted-but-not-sent from no report because no unwind was tried. A second boolean with no independent state only creates an impossible disagreement.

- [ ] **Step 1: Write the failing test**

Build the venue doubles first. Open `internal/app/exercise/app_test.go:74` (`serveBootReads`) and `internal/app/exercise/chaos_test.go` — one of them already stands up an `httptest.Server` answering a boot's reads, and copying it is far cheaper and far more accurate than writing a second one. Write `twoVenueServers(t)` returning both hosts, and point each side's `Endpoints` at them.

```go
// TestRunRefusesToDriveWhenEitherBootIsUnclean is the gate. It runs TWICE —
// once with side A unclean and once with side B — because a gate that only
// read side A would pass a one-sided test and then open a naked leg on the
// venue whose boot it never consulted. This is the mirror rule, on the side
// axis.
func TestRunRefusesToDriveWhenEitherBootIsUnclean(t *testing.T) {
	t.Parallel()
	for _, unclean := range []int{0, 1} {
		t.Run(fmt.Sprintf("side %d unclean", unclean), func(t *testing.T) {
			t.Parallel()
			snap, servers := runnableConfig(t)
			servers[unclean].refuseVenueJoin() // make that side's boot ExitIncomplete

			rep, err := Run(context.Background(), snap, observability.NewRegistry, io.Discard)
			if err != nil {
				t.Fatalf("Run() error = %v, want nil (a refusal belongs in the report)", err)
			}
			if rep.Drove {
				t.Fatal("Drove = true on an unclean boot; nothing may be sent")
			}
			if !errors.Is(rep.Refusal, ErrBootNotClean) {
				t.Errorf("Refusal = %v, want %v", rep.Refusal, ErrBootNotClean)
			}
			for i := range rep.Sides {
				if rep.Sides[i].Open != nil {
					t.Errorf("side %d attempted an open leg after an unclean boot on side %d", i, unclean)
				}
			}
			// The venue double counts Place calls; zero on BOTH sides is the
			// claim, not just on the unclean one.
			for i, s := range servers {
				if n := s.placeCalls(); n != 0 {
					t.Errorf("side %d received %d Place calls, want 0", i, n)
				}
			}
		})
	}
}

// TestRunRefusesToDriveWhenEitherLegStartsNonFlat is the second half of the
// gate, and it runs once per side. A gate that read only side 0 would pass one
// fixture and then trust a reduce-only inverse on the unread side's pre-existing
// position.
func TestRunRefusesToDriveWhenEitherLegStartsNonFlat(t *testing.T) {
	t.Parallel()
	for _, nonflat := range []int{0, 1} {
		t.Run(fmt.Sprintf("side %d non-flat", nonflat), func(t *testing.T) {
			t.Parallel()
			snap, servers := runnableConfigWithMatchedStartingPosition(t, nonflat, "25")

			rep, err := Run(context.Background(), snap, observability.NewRegistry, io.Discard)
			if err != nil {
				t.Fatalf("Run() = %v, want nil (a gate refusal belongs in the report)", err)
			}
			if rep.Drove {
				t.Fatal("Drove = true from a non-flat start; the inverse unwind may increase that position")
			}
			for i := range rep.Sides {
				if rep.Sides[i].Boot.ExitClass() != reconcile.ExitClean {
					t.Fatalf("side %d boot = %s, want ExitClean; the flat-start gate must be the refusal under test",
						i, rep.Sides[i].Boot.ExitClass())
				}
			}
			if !errors.Is(rep.Refusal, ErrPositionNotFlat) {
				t.Errorf("Refusal = %v, want %v", rep.Refusal, ErrPositionNotFlat)
			}
			for i, s := range servers {
				if n := s.placeCalls(); n != 0 {
					t.Errorf("side %d received %d Place calls from a non-flat start, want 0", i, n)
				}
			}
		})
	}
}

// TestRunBootsBothSidesAndReadsBothFlatBeforeDriving is the positive control:
// with two clean boots and explicit venue-flat answers the run proceeds.
// Reconciliation.Assertion's zero value means "did not run", not flat, so the
// positive control must come from VenuePositions too.
func TestRunBootsBothSidesAndReadsBothFlatBeforeDriving(t *testing.T) {
	t.Parallel()
	snap, servers := runnableConfig(t)
	for leg := range snap.Value.Pair.Legs {
		intent, _ := snap.Value.legIntent(leg)
		side, _ := snap.Value.sideFor(snap.Value.Pair.Legs[leg].Venue)
		servers[side].position(intent.Instrument, "0")
	}
	rep, err := Run(context.Background(), snap, observability.NewRegistry, io.Discard)
	if err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}
	for i := range rep.Sides {
		if got := rep.Sides[i].Boot.ExitClass(); got != reconcile.ExitClean {
			t.Fatalf("side %d boot = %s, want ExitClean", i, got)
		}
	}
	if !rep.Drove {
		t.Fatalf("Drove = false with two clean boots; Refusal = %v", rep.Refusal)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/app/pair/ -run 'TestRun' -v`
Expected: FAIL, `undefined: Run`.

- [ ] **Step 3: Write the implementation**

Create `report.go` with the four types above and no `ExitClass` yet — that is task 8. Create `app.go` with `Run` doing boots and both halves of the gate only; leave the drive as a stub returning `Report{Drove: true}` so this task's positive control passes and task 6 replaces it.

```go
// Run performs one cross-venue pair cycle: two reconcile-only boots, and — if
// and only if BOTH verdicts are ExitClean AND both legs' instruments are flat
// at their own venues — the paired drive.
//
// # The error contract is reconcile.Run's
//
// An error means a boot could not be PERFORMED. Every refusal, including the
// gate below, is in the Report.
//
// # Both boots and both venue-position reads finish before EITHER side is touched
//
// A drive that began against one clean boot and one unknown one is exactly the
// half-open pair this root exists to avoid producing, arrived at on purpose.
// The two boots are sequential rather than concurrent, and the choice is
// deliberate: each acquires a spend lock, opens a journal and makes signed
// venue calls, and running them concurrently would interleave two boots'
// diagnoses on one stderr with no way to tell which venue refused. There is no
// latency argument for a reconcile-only boot.
func Run(ctx context.Context, snap config.Snapshot[Config], newRegistry func() *observability.Registry,
	stderr io.Writer) (Report, error) {

	cfg := snap.Value

	// One wall over BOTH venues' calibrations (D9). clock.NewWall refuses a
	// duplicate venue, which backstops Validate's ErrSideVenueDuplicate — but
	// Validate is what NAMES the config field, so both exist.
	cals := append(cfg.Sides[0].OMS.Calibrations(), cfg.Sides[1].OMS.Calibrations()...)
	wall, err := clock.NewWall(cals)
	if err != nil {
		return Report{}, err
	}

	// The §2.4 filters, resolved BEFORE either boot exists, so a bad reference
	// revision refuses the RUN with the file named — before any venue call,
	// any fsync and any spend lock.
	filters, err := cfg.tradingFilters(wall)
	if err != nil {
		return Report{}, err
	}

	var rep Report
	runners := [2]*reconcile.Runner{}
	// boot.Sequence.Run returns *boot.Running — the concrete type, held per side
	// so the deferred teardown below can reach both.
	var running [2]*boot.Running

	// Teardown is deferred over both sides in reverse, and its failures are
	// reported rather than returned: a journal that will not close is a real
	// problem and it is not a reason to discard the verdicts just established.
	defer func() {
		for i := len(running) - 1; i >= 0; i-- {
			if running[i] == nil {
				continue
			}
			shutdown, _ := running[i].Shutdown(ctx, wall)
			for _, p := range shutdown.Phases {
				if p.Err != nil {
					fmt.Fprintf(stderr, "pair: side %d phase %q close: %v\n", i, p.Name, p.Err)
				}
			}
		}
	}()

	for i := range cfg.Sides {
		rep.Sides[i].Venue = cfg.Sides[i].OMS.Venue.Venue
		bootSnap := config.Snapshot[reconcile.Config]{Value: cfg.Sides[i].OMS, Rev: snap.Rev}
		// A registry PER SIDE: a collector registered by one side must not be
		// able to refuse the other side's boot.
		r, err := reconcile.NewRunner(bootSnap, wall, newRegistry(), stderr,
			reconcile.WithInstrumentFilters(filters[i], cfg.Reference.MaxAge.Duration()))
		if err != nil {
			return rep, err
		}
		runners[i] = r
		live, err := r.Sequence().Run(ctx)
		if err != nil {
			rep.Sides[i].Boot = r.BootReport()
			return rep, err
		}
		running[i] = live
		rep.Sides[i].Boot = r.BootReport()
		rep.Sides[i].RateLimit = r.RateLimit()
	}

	// The gate. Both, before either.
	for i := range rep.Sides {
		if class := rep.Sides[i].Boot.ExitClass(); class != reconcile.ExitClean {
			rep.Refusal = fmt.Errorf("side %d (%s) boot verdict is %s: %w",
				i, rep.Sides[i].Venue, class, ErrBootNotClean)
			return rep, nil
		}
		if runners[i].Halted() || runners[i].Module() == nil {
			rep.Refusal = fmt.Errorf("side %d (%s) boot halted: %w", i, rep.Sides[i].Venue, ErrBootNotClean)
			return rep, nil
		}
	}

	// A clean boot is not evidence of a flat account. Net-delta asserts only
	// disagreement between venue and journal, and Reconciliation.Assertion's
	// zero value also means the assertion did not run. Read each leg's VENUE
	// position directly; the journal-derived figure is flat on a fresh journal
	// regardless of what the account already holds.
	for leg := range cfg.Pair.Legs {
		side, _ := cfg.sideFor(cfg.Pair.Legs[leg].Venue) // Validate made this total.
		intent, _ := cfg.legIntent(leg)                  // Validate made this total.
		scope := positionScopeFor(cfg.Sides[side].OMS, intent.Instrument)
		pos, err := runners[side].Module().VenuePosition(scope, intent.Instrument)
		if err != nil {
			return rep, fmt.Errorf("side %d (%s) starting position: %w", side, rep.Sides[side].Venue, err)
		}
		sign, err := pos.Net.Sign()
		if err != nil {
			return rep, fmt.Errorf("side %d (%s) starting position: %w", side, rep.Sides[side].Venue, err)
		}
		if sign != 0 {
			rep.Refusal = fmt.Errorf("side %d (%s) starts at %s on %s: %w",
				side, rep.Sides[side].Venue, pos.Net, intent.Instrument, ErrPositionNotFlat)
			return rep, nil
		}
	}

	rep.Drove = true
	return rep, nil
}
```

The `running` slice's element type must match whatever `boot.Sequence.Run` returns — read `internal/platform/boot` and use the concrete type rather than the interface sketched above if that is what it returns. `positionScopeFor` must select the configured scope covering the leg's venue/product and must treat no match as a wiring defect; do not infer a scope from the journal or turn a missing scope into flatness. The construct it reads is named rather than left to be found: `reconcile.Config.PositionScopes` is an exported `[]ScopeConfig`, and `ScopeConfig.Scope()` returns an `application.PositionScope` (`internal/app/reconcile/config.go:616`). Match on `Venue` and `Product`. Note that `application.PositionScope` is a **type alias** for `ports.PositionScope`, which is why this root can name the type at all — `singlewriter` polices import paths, and `internal/oms/application` is not one of the two forbidden trees.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test -race ./internal/app/pair/ -run 'TestRun' -v`
Expected: PASS, both boot-refusal subtests, both non-flat-refusal subtests and the positive control.

- [ ] **Step 5: Bite-check the gate's second read**

The mirror-rule risk here is a gate that reads side 0 only. The `for i := range rep.Sides` loop already covers both; mutate it to `for i := 0; i < 1; i++` and confirm the `side 1 unclean` subtest fails:

```bash
tools/mutate.sh internal/app/pair/app.go <line-of-the-gate-loop> \
  's/for i := range rep.Sides {/for i := 0; i < 1; i++ {/' \
  'the gate reads only side 0' ./internal/app/pair/
```

Then make the same mutation to the flat-position loop and confirm the `side 1 non-flat` subtest fails. Expected: `caught`, `caught`. A `SURVIVED` means the corresponding mirror fixture is missing.

- [ ] **Step 6: Commit**

```bash
git add internal/app/pair/app.go internal/app/pair/report.go internal/app/pair/app_test.go
git commit -m "pair: two boots, and the gate that reads both before either is driven

A drive begun against one clean boot and one unknown one is the
half-open pair this root exists to avoid, produced on purpose. The
boots are sequential: each takes a spend lock, opens a journal and
makes signed calls, and concurrent boots would interleave two
diagnoses on one stderr with no way to tell which venue refused."
```

---

## Task 6: the sequential drive — leg A, establish, leg B

**Files:**
- Modify: `internal/app/pair/app.go` (replace task 5's `rep.Drove = true` stub)
- Test: `internal/app/pair/app_test.go`

**Interfaces:**
- Consumes: `Config.openPlan`, `Module.Plan.Plan`, `Module.Execute.Execute`, `Module.Join.Join`, `Module.LegState` (task 1), `Establish`/`Compose` (task 2).
- Produces: `func (d *drive) open(leg int, quantity fixedpoint.Quantity) (*LegReport, error)` and `func (d *drive) establish(leg int, id ids.ClientOrderID) (LegExposure, error)` — both unexported.

The parameters are leg indices, despite the older `side` spelling in the interface sketch: leg-list order decides A/B sequencing, which D8 needs; `LegConfig.Venue` is the binding. Every runner, checklist, filter map and `rep.Sides` access made on behalf of a leg resolves its side through `sideFor` first. Treating leg index as side index is the silent cross-wire the reversed-list control exists to expose.

- [ ] **Step 1: Write the failing tests**

```go
// TestSecondLegIsSizedToTheFirstLegsEstablishedFill is D5. The venue double
// fills side A PARTIALLY — 60 of the ordered 100 — and the assertion is on the
// quantity side B was ASKED for.
//
// 60, not 50: at one half the filled quantity equals its own remainder and a
// hedge sized to the unfilled part would produce the same number.
func TestSecondLegIsSizedToTheFirstLegsEstablishedFill(t *testing.T) {
	t.Parallel()
	snap, servers := runnableConfig(t)
	servers[0].fillIOC("60")  // ordered 100, filled 60
	servers[1].fillIOC("60")

	rep, err := Run(context.Background(), snap, observability.NewRegistry, io.Discard)
	if err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}
	got := servers[1].lastPlaceQuantity()
	if got != "60" {
		t.Fatalf("side B was asked for %q, want the established fill \"60\" and not the document's \"100\"", got)
	}
	if rep.Verdict != VerdictIntact {
		t.Errorf("Verdict = %s, want VerdictIntact", rep.Verdict)
	}
}

// TestSideBIsNotSentWhenSideAFilledNothing is D6. An IOC that did not fill
// opened nothing, so there is nothing to hedge — and sending B would open the
// naked leg this root exists to prevent.
func TestSideBIsNotSentWhenSideAFilledNothing(t *testing.T) {
	t.Parallel()
	snap, servers := runnableConfig(t)
	servers[0].fillIOC("0")

	rep, err := Run(context.Background(), snap, observability.NewRegistry, io.Discard)
	if err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}
	if n := servers[1].placeCalls(); n != 0 {
		t.Fatalf("side B received %d Place calls after side A filled nothing, want 0", n)
	}
	if rep.Verdict != VerdictUnopened {
		t.Errorf("Verdict = %s, want VerdictUnopened", rep.Verdict)
	}
}

// TestSideBIsNotSentWhileSideAIsIndeterminate is D4's whole point. The venue
// double leaves side A's order NON-TERMINAL for every settle attempt, so its
// cumulative reads zero — the same number TestSideBIsNotSentWhenSideAFilledNothing
// produces, from a completely different situation. The two tests together are
// what stop a confident zero standing in for an unfinished read.
func TestSideBIsNotSentWhileSideAIsIndeterminate(t *testing.T) {
	t.Parallel()
	snap, servers := runnableConfig(t)
	servers[0].holdOrderNonTerminal()

	rep, err := Run(context.Background(), snap, observability.NewRegistry, io.Discard)
	if err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}
	if n := servers[1].placeCalls(); n != 0 {
		t.Fatalf("side B received %d Place calls while side A was indeterminate, want 0", n)
	}
	if rep.Verdict != VerdictIndeterminate {
		t.Errorf("Verdict = %s, want VerdictIndeterminate", rep.Verdict)
	}
	if rep.Sides[0].Exposure.Because == "" {
		t.Error("Exposure.Because is empty; an indeterminate leg must name why")
	}
}

// TestTheSettleLoopIsBOUNDED. A mutation that stopped the venue answering
// would otherwise make this test HANG rather than fail, and a hanging test
// names no defect. The injected sleep counts calls and the assertion is on the
// count, so the bound is checked rather than waited out.
func TestTheSettleLoopIsBounded(t *testing.T) {
	t.Parallel()
	snap, servers := runnableConfig(t)
	servers[0].holdOrderNonTerminal()
	var sleeps int
	run := stubSleep(func(time.Duration) { sleeps++ })

	if _, err := run(context.Background(), snap, observability.NewRegistry, io.Discard); err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}
	// settle_attempts is 3 in the fixture, so at most 2 waits BETWEEN 3 reads.
	if sleeps > 2 {
		t.Fatalf("settle loop slept %d times, want at most 2 for settle_attempts=3", sleeps)
	}
}

// TestLegOrderDoesNotSelectTheSideModule reverses the leg list while leaving
// Sides in binance/bybit order. Leg order decides A then B; each leg's Venue
// decides which runner, filter map and report slot receive it.
func TestLegOrderDoesNotSelectTheSideModule(t *testing.T) {
	t.Parallel()
	snap, servers := runnableConfig(t)
	snap.Value.Pair.Legs[0], snap.Value.Pair.Legs[1] = snap.Value.Pair.Legs[1], snap.Value.Pair.Legs[0]
	servers[1].fillIOC("100") // leg A is now bybit
	servers[0].fillIOC("100") // leg B is now binance

	if _, err := Run(context.Background(), snap, observability.NewRegistry, io.Discard); err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}
	if servers[1].placeCalls() != 1 || servers[0].placeCalls() != 1 {
		t.Fatalf("Place calls binance/bybit = %d/%d, want 1/1 on each leg's own module",
			servers[0].placeCalls(), servers[1].placeCalls())
	}
	if got := servers[1].firstPlace().Side; got != "BUY" {
		t.Errorf("bybit module received %s, want reversed leg A's BUY", got)
	}
	if got := servers[0].firstPlace().Side; got != "SELL" {
		t.Errorf("binance module received %s, want reversed leg B's SELL", got)
	}
}
```

Give the fixture `settle_attempts: 3` and `settle: "10ms"`. `Run` calls an unexported `runWithWait(..., time.Sleep)`; `stubSleep(f)` returns the same per-run function with `f`. The wait then lives on that run's `drive`, on `oms.Deps.SettleWait`'s precedent, and parallel tests cannot read a package global another test is mutating.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/app/pair/ -run 'TestSecondLeg|TestSideB|TestTheSettle' -v`
Expected: FAIL — the stub `Run` sends nothing and `rep.Verdict` is the zero `VerdictUnopened` for the wrong reason.

- [ ] **Step 3: Write the implementation**

Replace the stub in `app.go`:

```go
// drive holds what the paired cycle needs from both sides.
type drive struct {
	cfg     Config
	runners [2]*reconcile.Runner
	rep     *Report
	stderr  io.Writer
	wait    func(time.Duration)
}

// Run supplies the production wait. Tests call runWithWait through stubSleep,
// so no parallel test can replace another drive's clock edge.
func Run(/* existing arguments */) (Report, error) {
	return runWithWait(/* existing arguments */, time.Sleep)
}

// run is the sequential cycle: open leg 0, establish it, and open leg 1 only
// against what leg 0 actually took.
//
// # Why the second leg is gated on the FILL and not on the submission
//
// A leg that submitted and filled nothing opened no exposure, so hedging it
// opens the naked position this whole root exists to prevent. A leg whose fill
// could not be established has not been measured, and sizing a hedge from an
// unmeasured zero is the same mistake made confidently. So the gate reads
// Establish's three-valued answer, and two of the three stop the run.
func (d *drive) run() {
	sideA := d.sideForLeg(0)
	sideB := d.sideForLeg(1)
	open0, err := d.open(0, d.mustQuantity(0))
	d.rep.Sides[sideA].Open = open0
	if err != nil {
		// Nothing reached the venue on leg A, so nothing is exposed and leg B
		// is not touched. The verdict stays Unopened and the refusal is the
		// report's.
		d.rep.Refusal = err
	} else {
		d.rep.Sides[sideA].Exposure, err = d.establish(0, open0.ID)
		if err != nil {
			d.rep.Refusal = err
		}
		a := d.rep.Sides[sideA].Exposure
		// flat() is Sign()==0 with the error surfaced, never absorbed: a quantity
		// nobody populated must not read as "filled nothing".
		aFlat, flatErr := flat(a.Filled)
		if !a.Established || flatErr != nil || aFlat {
			// Three stops, one branch, and they are told apart by Compose rather
			// than here: indeterminate (a human), an unreadable quantity (a human),
			// or a clean no-fill (nothing opened). None of them sends leg 1.
			if flatErr != nil {
				d.rep.Refusal = errors.Join(d.rep.Refusal, flatErr)
				a.Established, a.Because = false, flatErr.Error()
				d.rep.Sides[sideA].Exposure = a
			}
			d.rep.Sides[sideB].Exposure = unopenedPartner(d.cfg, 1)
			d.rep.Verdict, err = Compose(a, d.rep.Sides[sideB].Exposure)
			d.rep.Refusal = errors.Join(d.rep.Refusal, err)
		} else {
			open1, openErr := d.open(1, a.Filled)
			d.rep.Sides[sideB].Open = open1
			if openErr != nil {
				// D5's refusal path: side A's fill could not be expressed as a leg on
				// side B — below its min quantity or notional, off its grid, or refused
				// by Execute before Place. Nothing was sent on B. Every such verdict
				// reaches task 7's single compensation tail.
				d.rep.Refusal = openErr
				d.rep.Sides[sideB].Exposure = unopenedPartner(d.cfg, 1)
				d.rep.Verdict, err = Compose(a, d.rep.Sides[sideB].Exposure)
				d.rep.Refusal = errors.Join(d.rep.Refusal, err)
			} else {
				d.rep.Sides[sideB].Exposure, err = d.establish(1, open1.ID)
				if err != nil {
					d.rep.Refusal = errors.Join(d.rep.Refusal, err)
				}
				d.rep.Verdict, err = Compose(a, d.rep.Sides[sideB].Exposure)
				d.rep.Refusal = errors.Join(d.rep.Refusal, err)
			}
		}
	}
	// Task 7 replaces this comment with d.compensate(). There is exactly one
	// tail: no composed verdict can bypass compensation through an early return.
}

// flat reports whether q is zero, and NEVER absorbs the error into "yes".
//
// fixedpoint.Quantity.Sign is fallible so that "a flat position and a position
// nobody populated" stay distinguishable, and at this call site that
// distinction is the difference between "leg A opened nothing" and "leg A's
// size was never read" — one of which sends leg B and the other of which must
// not.
func flat(q fixedpoint.Quantity) (bool, error) {
	sign, err := q.Sign()
	if err != nil {
		return false, err
	}
	return sign == 0, nil
}

// mustQuantity is the document's stated size for one leg.
//
// It cannot fail after Validate — legIntent is the shared half of both, which
// is exactly the property exercise's config keeps — and the panic is the
// wiring-defect assertion rather than an error path: a document that validated
// and then failed to state a quantity is a bug in this package, and returning
// an error here would invite a caller to carry on with a zero.
func (d *drive) mustQuantity(leg int) fixedpoint.Quantity {
	intent, err := d.cfg.legIntent(leg)
	if err != nil {
		panic(fmt.Sprintf("pair: leg %d did not resolve after Validate: %v", leg, err))
	}
	return intent.Quantity
}

// sideForLeg binds by venue. Leg-list order decides A/B sequencing; it is not
// a runner index. Treating it as one sends a reversed document through the
// other venue's module, filter map and report slot.
func (d *drive) sideForLeg(leg int) int {
	side, ok := d.cfg.sideFor(d.cfg.Pair.Legs[leg].Venue)
	if !ok {
		panic(fmt.Sprintf("pair: leg %d did not resolve after Validate", leg))
	}
	return side
}

// unopenedPartner is the exposure of a side this root DECIDED not to send.
//
// It is Established: "we chose not to send it" is a measurement, and it is a
// different fact from "we sent it and could not read the result". Collapsing
// the two is what would let an unreadable leg travel through Compose as a
// confident zero — which is the whole hazard D4 exists for, arriving by the
// back door. The quantity carries the leg's own base asset, because a zero with
// no asset tag is the value fixedpoint refuses to let read as flat.
func unopenedPartner(cfg Config, leg int) LegExposure {
	intent, err := cfg.legIntent(leg)
	if err != nil {
		panic(fmt.Sprintf("pair: leg %d did not resolve after Validate: %v", leg, err))
	}
	side, _ := cfg.sideFor(cfg.Pair.Legs[leg].Venue)
	zero, err := intent.Quantity.Sub(intent.Quantity) // zero, carrying the base asset
	if err != nil {
		panic(fmt.Sprintf("pair: leg %d quantity is not subtractable from itself: %v", leg, err))
	}
	return LegExposure{
		Venue:       cfg.Sides[side].OMS.Venue.Venue,
		Instrument:  intent.Instrument,
		Side:        intent.Side,
		Filled:      zero,
		Established: true,
		Because:     "",
	}
}
```

`unopenedPartner` has no `ID` and no `State`, deliberately: there is no order to name. The renderer (task 10) keys its "exposure" line on a non-empty `ID`, so a side that was never sent prints nothing rather than printing a fabricated flat reading.

`open(leg, quantity)` plans one leg and executes it. It always resolves the runner and report side from `LegConfig.Venue`; leg-list order remains only the A/B sequencing decision. On a not-sent path it returns a non-nil `LegReport` carrying `Refusal`, so the report cannot confuse "an attempt was refused" with "no attempt was made":

```go
func (d *drive) open(leg int, quantity fixedpoint.Quantity) (*LegReport, error) {
	side := d.sideForLeg(leg)
	m := d.runners[side].Module()
	plan, err := d.cfg.openPlan(leg, quantity)
	if err != nil {
		return &LegReport{Refusal: err}, err
	}
	records, err := m.Plan.Plan(plan)
	if err != nil {
		// The planner's §2.4 grid verification lands here, which is D5's
		// refusal: an unhedgeable fill is a refusal to send rather than a
		// rounded-off order.
		//
		// But it is NOT the only thing that lands here, and wrapping everything
		// in ErrHedgeUnsizable would tell an operator the market grid rejected
		// a size when the truth was something else entirely. Plan also returns
		// ErrLegAlreadyPlanned on a repeated (Trade, Leg, Attempt) — which is a
		// re-run of the same document, §5.3's duplicate fail-safe doing its job
		// — plus repository and journal errors. Reported as a sizing problem,
		// an operator would go and edit a quantity that was never wrong.
		//
		// So the sentinel is attached ONLY to the filter refusals and ONLY on
		// leg B, which is the only leg sized to something other than the
		// document. Read conformsToFilters' sentinels in
		// internal/oms/application/planning.go and name them; every other error
		// passes through unchanged.
		refusal := err
		if leg == 1 && isFilterRefusal(err) {
			refusal = fmt.Errorf("side %d (%s): %v: %w",
				side, d.rep.Sides[side].Venue, err, ErrHedgeUnsizable)
		}
		return &LegReport{Refusal: refusal}, refusal
	}
	refs := make([]application.DispatchRef, len(records))
	for i, rec := range records {
		refs[i] = application.DispatchRef{ID: rec.Order.ClientOrderID, Version: rec.Version}
	}
	ex, err := m.Execute.Execute(d.runners[side].Checklist(), refs, application.UrgencyOpening)
	if err != nil {
		return &LegReport{ID: refs[0].ID, Refusal: err}, err // package preflight refused whole: nothing was sent
	}
	if ex.Legs[0].Err != nil {
		// Executor's split signature is deliberate: execute.go says a per-leg
		// Err means THIS leg was not sent and remains dispatchable. Looking only
		// at the aggregate error recreates the `if err != nil { return }` reflex
		// that signature exists to prevent, then establish waits forever on a
		// still-planned order and D8 refuses the unwind as Indeterminate.
		return &LegReport{ID: refs[0].ID, Refusal: ex.Legs[0].Err, Result: ex.Legs[0]}, ex.Legs[0].Err
	}
	return &LegReport{ID: refs[0].ID, Result: ex.Legs[0]}, nil
}
```

`establish(leg, id)` is D4's bounded loop and resolves the side from that leg's venue:

```go
// establish re-joins one leg to venue truth until its state is terminal, then
// reads the projection.
//
// # It is BOUNDED, and the bound is a config field rather than a constant
//
// The loop's exit condition is a venue publishing trade rows, which is
// something this process does not control. An unbounded wait here would be a
// composition root blocking forever on a venue that went quiet, and the honest
// answer after the bound is INDETERMINATE — which stops the run for a human,
// sends nothing, and unwinds nothing.
func (d *drive) establish(leg int, id ids.ClientOrderID) (LegExposure, error) {
	side := d.sideForLeg(leg)
	m := d.runners[side].Module()
	venue := d.rep.Sides[side].Venue
	var last LegExposure
	for attempt := 0; attempt < d.cfg.Pair.SettleAttempts; attempt++ {
		if attempt > 0 {
			d.wait(d.cfg.Pair.Settle.Duration())
		}
		joined, err := m.Join.Join([]ids.ClientOrderID{id})
		if err != nil {
			return LegExposure{Venue: venue, ID: id, Because: err.Error()}, err
		}
		st, err := m.LegState(id)
		if err != nil {
			return LegExposure{Venue: venue, ID: id, Because: err.Error()}, err
		}
		last = Establish(venue, id, st, factsFor(id, joined))
		if last.Established {
			return last, nil
		}
	}
	return last, nil
}

// factsFor turns the join report's three lists into the leg's own three facts.
func factsFor(id ids.ClientOrderID, j application.Join) JoinFacts {
	return JoinFacts{
		Absent:       slices.Contains(j.Absent, id),
		Incomplete:   slices.Contains(j.Incomplete, id),
		Unattributed: slices.Contains(j.Unattributed, id),
	}
}
```

`unopenedPartner(cfg, leg)` returns an **established, zero** `LegExposure` for a leg that was deliberately not sent — established because "we chose not to send it" is a measurement, unlike "we sent it and could not read it". It resolves the report side from the leg's venue. Write it with that sentence as its comment; the distinction is the one `TestSideBIsNotSentWhenSideAFilledNothing` and `TestSideBIsNotSentWhileSideAIsIndeterminate` exist to separate.

Wire `drive` into `runWithWait` where the stub was, set its `wait` field from the argument, and set `rep.Drove = true` before calling `d.run()`. `Run` supplies `time.Sleep`; no package-level `sleep` variable exists. Re-snapshot `rep.Sides[i].RateLimit = runners[i].RateLimit()` for both sides after `d.run()` returns. The timing is deliberate on `internal/app/exercise/app.go:191-195`'s argument: bans are monotone and the drive plus unwind is venue traffic, so a snapshot beside Boot's misses the one §7.6 alert this binary can newly earn.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test -race ./internal/app/pair/ -v`
Expected: PASS, all of it.

- [ ] **Step 5: Bite-check D5 and D6**

```bash
# D5: leg B sized to the document rather than to the fill.
tools/mutate.sh internal/app/pair/app.go <line-of-open(1, a.Filled)> \
  's/d.open(1, a.Filled)/d.open(1, d.mustQuantity(1))/' \
  'leg B sized to the document, not the established fill' ./internal/app/pair/

# D6: the zero-fill gate removed.
tools/mutate.sh internal/app/pair/app.go <line-of-the-fill-gate> \
  's/if !a.Established || flatErr != nil || aFlat {/if !a.Established {/' \
  'leg B sent against a zero fill' ./internal/app/pair/
```

Expected: `caught`, `caught`. Watch for `TIMEOUT!!` on the second — if the venue double never answers, bound the test rather than believing the verdict.

- [ ] **Step 6: Commit**

```bash
git add internal/app/pair/app.go internal/app/pair/app_test.go
git commit -m "pair: the sequential drive — open, establish, hedge the fill

The second leg is gated on the first leg's ESTABLISHED fill, not on its
submission: a leg that filled nothing opened nothing, and a leg whose
fill could not be read has not been measured. Sizing a hedge from an
unmeasured zero is the same mistake made confidently, so two of
Establish's three answers stop the run and only one proceeds. The
settle loop is bounded by a config field because its exit condition is
a venue publishing trade rows, which this process does not control."
```

---

## Task 7: the unwind, and the three readings it refuses

**Files:**
- Modify: `internal/app/pair/app.go`
- Test: `internal/app/pair/app_test.go`

**Interfaces:**
- Consumes: `Config.unwindPlan`, `application.UrgencySafety`, `Compose`.
- Produces: `func (d *drive) unwind(leg int, filled fixedpoint.Quantity) (*LegReport, error)` and `func (d *drive) compensate()`.

- [ ] **Step 1: Write the failing tests**

```go
// TestAnUnsizableHedgeUnwindsSideA is D5's own refusal path. Side A's fill is
// established, then side B's Plan rejects that quantity against side B's §2.4
// lot grid with ErrHedgeUnsizable. Nothing is sent on B, but A must still reach
// the single compensation tail.
func TestAnUnsizableHedgeUnwindsSideA(t *testing.T) {
	t.Parallel()
	snap, servers := runnableConfigWithSteps(t, "1", "100")
	servers[0].fillIOC("60") // valid on A, off B's 100-unit grid

	rep, err := Run(context.Background(), snap, observability.NewRegistry, io.Discard)
	if err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}
	if !errors.Is(rep.Sides[1].Open.Refusal, ErrHedgeUnsizable) {
		t.Fatalf("side B refusal = %v, want %v", rep.Sides[1].Open.Refusal, ErrHedgeUnsizable)
	}
	if servers[1].placeCalls() != 0 || servers[0].placeCalls() != 2 {
		t.Fatalf("Place calls A/B = %d/%d, want 2/0 — A open plus unwind, nothing on B",
			servers[0].placeCalls(), servers[1].placeCalls())
	}
}

// TestAPerLegDispatchRefusalOnSideBUnwindsSideA covers Execute's split
// signature: aggregate error nil, LegResult.Err non-nil, and no Place. Refuse
// the unwind too so the final report preserves the composed HalfOpen verdict;
// Indeterminate would mean establish was incorrectly asked to wait on the
// still-dispatchable planned order.
func TestAPerLegDispatchRefusalOnSideBUnwindsSideA(t *testing.T) {
	t.Parallel()
	snap, servers := runnableConfig(t)
	servers[0].fillIOC("60")
	servers[0].rejectPlaceAfter(1)
	servers[1].refuseDispatchBeforePlace()

	rep, err := Run(context.Background(), snap, observability.NewRegistry, io.Discard)
	if err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}
	if servers[0].placeCalls() != 2 {
		t.Fatalf("side A received %d Place calls, want open plus unwind", servers[0].placeCalls())
	}
	if servers[1].placeCalls() != 0 {
		t.Fatalf("side B received %d Place calls, want 0 for a per-leg refusal", servers[1].placeCalls())
	}
	if rep.Verdict != VerdictHalfOpen {
		t.Fatalf("Verdict = %s, want VerdictHalfOpen, not VerdictIndeterminate", rep.Verdict)
	}
}

// TestAHalfOpenPairUnwindsSideA is the case alternative (ii) exists for: side A
// filled, side B reached Place and returned terminal StateRejected, and the
// exposure is closed rather than left for a human. This is specifically the
// terminal-rejection path, not a Plan refusal or a per-leg dispatch refusal.
func TestAHalfOpenPairUnwindsSideA(t *testing.T) {
	t.Parallel()
	snap, servers := runnableConfig(t)
	servers[0].fillIOC("60")
	servers[1].rejectPlace() // B's venue refuses; nothing opens there

	rep, err := Run(context.Background(), snap, observability.NewRegistry, io.Discard)
	if err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}
	if rep.Sides[0].Unwind == nil {
		t.Fatal("no unwind attempted on a half-open pair")
	}
	// The unwind's three claims, asserted separately: a close that got the side
	// right and reduce-only wrong opens fresh exposure while looking like a close.
	place := servers[0].lastPlace()
	if place.Side != "BUY" {
		t.Errorf("unwind side = %q, want BUY (the opposite of the sell that opened)", place.Side)
	}
	if !place.ReduceOnly {
		t.Error("unwind reduceOnly = false; a non-reduce-only close can open a position")
	}
	if place.Quantity != "60" {
		t.Errorf("unwind quantity = %q, want the established fill \"60\"", place.Quantity)
	}
	if place.ClientOrderID == servers[0].firstPlace().ClientOrderID {
		t.Error("unwind reused the opening leg's ClientOrderID; §5.3 forbids reuse in the recoverable window")
	}
}

// TestASkewedPairIsNotUnwound and TestAnIndeterminatePairIsNotUnwound are D8.
// A compensating trade is §6.7's gate's business and §6.7 cannot see this pair
// (record §95 §4), so this root acts on exactly one reading and hands every
// other to a human. These two are the refusals, and they are separate tests
// because they arrive from different situations and must not share a fixture.
func TestASkewedPairIsNotUnwound(t *testing.T) {
	t.Parallel()
	snap, servers := runnableConfig(t)
	servers[0].fillIOC("100")
	servers[1].fillIOC("60") // partial hedge: both sides exposed, unequally

	rep, err := Run(context.Background(), snap, observability.NewRegistry, io.Discard)
	if err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}
	if rep.Verdict != VerdictSkewed {
		t.Fatalf("Verdict = %s, want VerdictSkewed", rep.Verdict)
	}
	for i := range rep.Sides {
		if rep.Sides[i].Unwind != nil {
			t.Errorf("side %d was unwound on a skewed pair; §6.7 owns that compensation", i)
		}
	}
	if !errors.Is(rep.Refusal, ErrUnwindRefused) {
		t.Errorf("Refusal = %v, want %v naming why nothing was compensated", rep.Refusal, ErrUnwindRefused)
	}
}

func TestAnIndeterminatePairIsNotUnwound(t *testing.T) {
	t.Parallel()
	snap, servers := runnableConfig(t)
	servers[0].fillIOC("60")
	servers[1].holdOrderNonTerminal() // B sent, fate unknown

	rep, err := Run(context.Background(), snap, observability.NewRegistry, io.Discard)
	if err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}
	if rep.Verdict != VerdictIndeterminate {
		t.Fatalf("Verdict = %s, want VerdictIndeterminate", rep.Verdict)
	}
	if rep.Sides[0].Unwind != nil {
		t.Error("side A was unwound while side B's fate was unknown — the unwind could double the exposure it was meant to close")
	}
}

// TestAnUnwindThatFailsLeavesTheReportExposed. The unwind is an order like any
// other and it can be refused; when it is, the pair is still half-open and the
// report must say so rather than reporting the attempt as the outcome.
func TestAnUnwindThatFailsLeavesTheReportExposed(t *testing.T) {
	t.Parallel()
	snap, servers := runnableConfig(t)
	servers[0].fillIOC("60")
	servers[0].rejectPlaceAfter(1) // the open succeeds; the unwind is refused
	servers[1].rejectPlace()

	rep, err := Run(context.Background(), snap, observability.NewRegistry, io.Discard)
	if err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}
	if rep.Verdict != VerdictHalfOpen {
		t.Fatalf("Verdict = %s, want VerdictHalfOpen — the unwind did not close it", rep.Verdict)
	}
	if !Exposed(rep.Verdict) {
		t.Error("Exposed = false on a failed unwind")
	}
	if rep.Sides[0].Unwind == nil || rep.Sides[0].Unwind.Refusal == nil {
		t.Fatal("failed unwind has no LegReport.Refusal; it is indistinguishable from no unwind attempted")
	}
}
```

`rejectPlace()` cannot stand in for either new test. It crosses the wire and records terminal `StateRejected`; an unsizable hedge stops in `Plan`, and a per-leg dispatch refusal leaves the order `StatePlanned` with `LegResult.Err` because nothing was sent. Those are three different authorities and three different failure states.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/app/pair/ -run 'Unwind|Skewed|Indeterminate' -v`
Expected: FAIL — no unwind is attempted.

- [ ] **Step 3: Write the implementation**

Extract the block below into `d.compensate()`, and replace task 6's single tail comment with `d.compensate()`. `run()` keeps exactly one exit: every branch that composes a verdict falls through to that call. The Plan-refusal branch is D5's advertised path and must not be allowed to return immediately before the compensation task added later.

```go
func (d *drive) compensate() {
	// The unwind, and the narrowness is the design (D8).
	//
	// A compensating trade is §6.7's gate's business — "the single gate for
	// every compensating trade" — and record §95 §4 found that §6.7 cannot see
	// a cross-venue pair at all: HedgeGroup is (Account, Venue, Symbol) and the
	// pair sits in two groups with no group holding both. So this root does not
	// get to compensate on a reading it could not fully establish.
	//
	// VerdictHalfOpen with the exposure on leg A is the ONE reading where the
	// answer is unambiguous: A's fill is measured, B opened nothing, and the
	// task-5 flat-start gate means closing A returns the account to where it
	// started. Without that gate, an opening sell that merely reduced a
	// pre-existing long would be followed by a reduce-only buy that increases
	// the position and is rejected or capped. Every other
	// exposed reading ends here for a human:
	//
	//   - VerdictSkewed: both sides carry exposure and the residual is a
	//     per-venue quantity §6.7 exists to size. Unwinding either side by this
	//     root's arithmetic would be that gate's job done outside it.
	//   - VerdictIndeterminate: at least one leg's fate is unknown. An unwind
	//     against an unknown leg can DOUBLE the exposure it meant to close.
	//   - VerdictHalfOpen on leg B: expressible, and out of scope for phase 1 —
	//     the drive opens leg A first, so the only way B is exposed alone
	//     is an A fill that later read as zero, which is a contradiction the
	//     run must not resolve by trading.
	// An unreadable quantity is NOT an unwind. flat's error means leg A's fill
	// was never populated, and Compose has already answered Indeterminate for
	// it — but the guard checks it here rather than resting on that, because a
	// premise about another function's output is exactly the thing that goes
	// stale silently. aFlat stays false on the error path, so the `err == nil`
	// term is what refuses.
	sideA := d.sideForLeg(0)
	aFlat, aErr := flat(d.rep.Sides[sideA].Exposure.Filled)
	if d.rep.Verdict == VerdictHalfOpen && d.rep.Sides[sideA].Exposure.Established &&
		aErr == nil && !aFlat {
		unwound, err := d.unwind(0, d.rep.Sides[sideA].Exposure.Filled)
		d.rep.Sides[sideA].Unwind = unwound
		if err != nil {
			d.rep.Refusal = errors.Join(d.rep.Refusal, err)
		}
		// Re-read leg A and recompose: the report must state where the pair
		// ENDED, not what was attempted. A failed unwind leaves it half-open.
		//
		// The close counts only when the unwind's OWN fill matches the exposure
		// it was sized to. A partial close is still exposure, and a comparison
		// that accepted "some of it filled" would report a half-closed pair as
		// unopened — which is the flattering direction.
		if unwound != nil {
			after, _ := d.establish(0, unwound.ID)
			closed := false
			if after.Established {
				cmp, err := after.Filled.Cmp(d.rep.Sides[sideA].Exposure.Filled)
				closed = err == nil && cmp == 0
			}
			if closed {
				// Exposure is NOT overwritten: it stays the opening reading,
				// which is a fact about what the open did and belongs in the
				// report. What changed is the pair's net position, and the
				// verdict is where that is stated.
				d.rep.Verdict = VerdictUnopened
			}
		}
	} else if Exposed(d.rep.Verdict) {
		d.rep.Refusal = errors.Join(d.rep.Refusal,
			fmt.Errorf("verdict %s: %w", d.rep.Verdict, ErrUnwindRefused))
	}
}
```

`unwind(leg, filled)` is `open` with `unwindPlan` and `application.UrgencySafety`. Like `open`, it returns a non-nil report carrying `Refusal` whenever the attempt was not sent:

```go
// unwind plans and sends the reduce-only close of one leg.
//
// UrgencySafety, not UrgencyOpening: reducing exposure is §7.6's safety class,
// and the difference is load-bearing rather than cosmetic. A stale-projection
// latch raised by the opening leg's own fan-in refuses the next OPENING package
// at the executor's admission read — and if the unwind were classed as one, the
// hold that a failed apply raises would block the very order that closes the
// exposure the hold is about.
func (d *drive) unwind(leg int, filled fixedpoint.Quantity) (*LegReport, error) {
	side := d.sideForLeg(leg)
	m := d.runners[side].Module()
	plan, err := d.cfg.unwindPlan(leg, filled)
	if err != nil {
		return &LegReport{Refusal: err}, err
	}
	records, err := m.Plan.Plan(plan)
	if err != nil {
		return &LegReport{Refusal: err}, err
	}
	refs := []application.DispatchRef{{ID: records[0].Order.ClientOrderID, Version: records[0].Version}}
	ex, err := m.Execute.Execute(d.runners[side].Checklist(), refs, application.UrgencySafety)
	if err != nil {
		return &LegReport{ID: refs[0].ID, Refusal: err}, err
	}
	if ex.Legs[0].Err != nil {
		return &LegReport{ID: refs[0].ID, Refusal: ex.Legs[0].Err, Result: ex.Legs[0]}, ex.Legs[0].Err
	}
	return &LegReport{ID: refs[0].ID, Result: ex.Legs[0]}, nil
}
```

Note that nothing here constructs a "zero quantity" — the close is recorded by the **verdict**, not by overwriting a measured field with a synthesised zero. A literal `fixedpoint.Quantity{}` carries no asset tag and is precisely the value `Sign` refuses; manufacturing one to mean "flat now" would put the unreadable value into the report and let it read as flat downstream.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test -race ./internal/app/pair/ -v`
Expected: PASS.

- [ ] **Step 5: Bite-check the two refusals**

```bash
tools/mutate.sh internal/app/pair/app.go <line-of-the-unwind-guard> \
  's/if d.rep.Verdict == VerdictHalfOpen \&\&/if Exposed(d.rep.Verdict) \&\&/' \
  'unwind on every exposed verdict, including skewed and indeterminate' ./internal/app/pair/
tools/mutate.sh internal/app/pair/app.go <line-of-UrgencySafety> \
  's/application.UrgencySafety/application.UrgencyOpening/' \
  'the unwind is classed as an opening' ./internal/app/pair/
tools/mutate.sh internal/app/pair/app.go <line-of-the-D5-open-error-branch> \
  's/d.rep.Refusal = openErr/d.rep.Refusal = openErr; return/' \
  'D5 returns before the single compensation tail' ./internal/app/pair/
```

Expected: `caught` on the first and third. The second may `SURVIVE` — if it does, that is a **test gap and not a wrong mutant**: add a test that latches `GateNetDeltaAsserted` at `OutcomeProjectionStale` before the unwind and asserts the unwind still sends. `boot.Checklist.Report` is how the fixture latches it; `internal/app/exercise/app_test.go`'s stale-apply cases show the shape.

- [ ] **Step 6: Commit**

```bash
git add internal/app/pair/app.go internal/app/pair/app_test.go
git commit -m "pair: the unwind, and the two readings it refuses

§6.7 is the single gate for every compensating trade and it cannot see
a cross-venue pair (record §95 §4), so this root compensates on exactly
one reading: half-open, side A, fill established. Skewed is a residual
§6.7 exists to size; indeterminate is a leg whose fate is unknown,
where an unwind can double the exposure it meant to close. Both end
the run for a human. The unwind is UrgencySafety, so a stale-projection
latch raised by the opening leg cannot block the order that closes it."
```

---

## Task 8: `ExitClass` and its precedence

**Files:**
- Modify: `internal/app/pair/report.go`
- Test: `internal/app/pair/report_test.go` (create)

**Interfaces:**
- Produces:
  ```go
  type ExitClass uint8
  const (
      ExitClean ExitClass = iota
      ExitOperator
      ExitIncomplete
      ExitExecution
      ExitExposed
  )
  func (e ExitClass) String() string
  func (r Report) ExitClass() ExitClass
  ```

- [ ] **Step 1: Write the failing test**

```go
// TestExitClassPrecedence pins the ORDER, which is the whole content of this
// method. Each case is built so that exactly one rung applies and the rungs
// below it would answer differently — a fixture satisfying only one rung shows
// every rung and hides the ladder's order.
func TestExitClassPrecedence(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		rep  Report
		want ExitClass
	}{
		{
			name: "a latched gate on either side outranks everything",
			rep:  reportWith(latchedBoot(), cleanBoot(), VerdictIntact, true),
			want: ExitOperator,
		},
		{
			// The mirror. A precedence that read side 0 only would pass above
			// and fail here.
			name: "a latched gate on side B",
			rep:  reportWith(cleanBoot(), latchedBoot(), VerdictIntact, true),
			want: ExitOperator,
		},
		{
			name: "an unfinished boot with nothing sent",
			rep:  reportWith(incompleteBoot(), cleanBoot(), VerdictUnopened, false),
			want: ExitIncomplete,
		},
		{
			// Exposure outranks a lost record: a live unbalanced position is
			// more urgent than an unjournaled ack.
			name: "exposure outranks a leg needing eyes",
			rep:  reportWithLegs(cleanBoot(), cleanBoot(), VerdictSkewed, unjournaledLeg()),
			want: ExitExposed,
		},
		{
			// A ban earned during dispatch is in the post-drive RateLimit
			// snapshot, not Boot. It is operator work when no exposure outranks it.
			//
			// The leg is UNJOURNALED on purpose. With clean legs the rung below
			// this one would not fire either, so "the ban outranks a leg needing
			// eyes" and "the ban outranks a clean run" would be the same
			// observation — a one-violation fixture shows every rung and hides
			// the ladder's order. Here ExitExecution is genuinely available and
			// the ban has to beat it.
			name: "a runtime venue ban outranks a leg needing eyes",
			rep: func() Report {
				r := reportWithLegs(cleanBoot(), cleanBoot(), VerdictIntact, unjournaledLeg())
				r.Sides[1].RateLimit.Bans = 1
				return r
			}(),
			want: ExitOperator,
		},
		{
			name: "a dispatched leg needing eyes on an unexposed pair",
			rep:  reportWithLegs(cleanBoot(), cleanBoot(), VerdictIntact, unjournaledLeg()),
			want: ExitExecution,
		},
		{
			// A refused leg sent nothing, so it is ExitIncomplete's and not
			// ExitExecution's. Told apart from the case above by exactly one
			// field, which is the point: the two legs differ only in whether
			// Place ran.
			name: "a refused leg sent nothing",
			rep:  reportWithLegs(cleanBoot(), cleanBoot(), VerdictIntact, refusedLeg()),
			want: ExitIncomplete,
		},
		{
			// Both classes present at once, on OPPOSITE sides, so the answer
			// cannot come from whichever side the loop happens to reach first.
			// A sent-and-broken record outranks a leg that never left.
			name: "a refused leg on side A and a broken record on side B",
			rep: func() Report {
				r := reportWithLegs(cleanBoot(), cleanBoot(), VerdictIntact, refusedLeg())
				r.Sides[1].Open = unjournaledLeg()
				return r
			}(),
			want: ExitExecution,
		},
		{
			name: "two clean boots, an intact pair, every leg journaled",
			rep:  reportWith(cleanBoot(), cleanBoot(), VerdictIntact, true),
			want: ExitClean,
		},
		{
			// D6: an IOC that did not fill is CLEAN. It is the likely demo
			// outcome and must not read as a failure.
			name: "nothing filled on either side",
			rep:  reportWith(cleanBoot(), cleanBoot(), VerdictUnopened, true),
			want: ExitClean,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.rep.ExitClass(); got != tc.want {
				t.Errorf("ExitClass() = %s, want %s", got, tc.want)
			}
		})
	}
}
```

Copy `cleanBoot()` from `internal/app/exercise/app_test.go:436` and write `latchedBoot()`/`incompleteBoot()` beside it.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/app/pair/ -run TestExitClassPrecedence -v`
Expected: FAIL, `r.ExitClass undefined`.

- [ ] **Step 3: Write the implementation**

```go
// ExitClass is what one pair cycle means for the operator driving it.
//
// It is exercise.ExitClass plus one rung, and the new rung is the reason this
// root exists: a run can end with the two venues holding UNBALANCED exposure,
// which is neither "a leg needs eyes" nor "nothing was sent".
type ExitClass uint8

const (
	// ExitClean : both boots clean, and the pair ended intact or unopened with
	// every dispatched leg journaled and applied. An IOC that filled nothing on
	// both sides is clean — it opened nothing.
	ExitClean ExitClass = iota

	// ExitOperator : a latched gate on either side's boot, or a venue IP ban.
	// Nothing was dispatched.
	ExitOperator

	// ExitIncomplete : a boot could not finish, the drive refused before any leg
	// was attempted, or a leg was attempted and REFUSED with nothing sent.
	// Nothing reached a venue; a retry may lift it.
	//
	// The third case is here rather than under ExitExecution on the OMS's own
	// contract, and getting it wrong tells an operator the opposite of the
	// truth. execute.go:136 states it: "a non-nil Err means NOTHING WAS SENT for
	// this leg and the order is still dispatchable." So a per-leg Err and a Plan
	// refusal are both nothing-sent, and neither is a dispatched record needing
	// eyes. Classing them as ExitExecution would print "a leg was dispatched"
	// about a leg that never left the process.
	ExitIncomplete

	// ExitExecution : a leg was SENT and its own record needs a human — a failed
	// Place, a record that did not journal, an apply whose failure holds new
	// trading, an ack the store contradicts. Exercise's class, per side.
	//
	// The distinguishing fact is that Place ran. A refusal is ExitIncomplete's,
	// above.
	ExitExecution

	// ExitExposed : the run ended with the pair NOT intact and exposure live —
	// half-open with a refused or failed unwind, skewed, or indeterminate.
	//
	// It outranks ExitExecution deliberately. A lost record is a reading this
	// system can re-derive; an unbalanced cross-venue position is money moving
	// against an account with nothing hedging it, and §6.7 cannot see it to
	// say so (record §95 §4). This is the class the slice was built to be able
	// to report.
	ExitExposed
)

// ExitClass reduces the report, boot verdicts first.
//
// The boot reports are the PRE-drive snapshots — exercise's rule and its
// reason: re-deriving them here would rank a latch raised by the drive's own
// fan-in as ExitOperator and bury the more specific fact, that THIS run
// dispatched.
func (r Report) ExitClass() ExitClass {
	for i := range r.Sides {
		if r.Sides[i].Boot.ExitClass() == reconcile.ExitOperator {
			return ExitOperator
		}
	}
	for i := range r.Sides {
		if r.Sides[i].Boot.ExitClass() == reconcile.ExitIncomplete {
			return ExitIncomplete
		}
	}
	if !r.Drove {
		return ExitIncomplete
	}
	if Exposed(r.Verdict) {
		return ExitExposed
	}
	// The runtime snapshots are taken after all drive and unwind traffic. An
	// exposed pair still outranks the ban as the exit class, but an intact pair
	// must not hide an execution-time IP ban behind clean boots.
	for i := range r.Sides {
		if r.Sides[i].RateLimit.Bans > 0 {
			return ExitOperator
		}
	}
	// Both leg classes are scanned in FULL before either answers, because a leg
	// that was sent and needs eyes outranks a leg that was refused, and the two
	// can arrive on different sides of the same run. Returning on the first leg
	// examined would make the answer depend on side order.
	refused := false
	for i := range r.Sides {
		for _, leg := range []*LegReport{r.Sides[i].Open, r.Sides[i].Unwind} {
			if leg == nil {
				continue
			}
			// Nothing was sent for this leg — see ExitIncomplete's comment. Note
			// it and keep scanning; a sent-and-broken leg elsewhere wins.
			if leg.Refusal != nil || leg.Result.Err != nil {
				refused = true
				continue
			}
			if leg.Result.Outcome.State != domain.StateSubmitted ||
				!leg.Result.Outcome.Journaled || !leg.Result.Outcome.Applied {
				return ExitExecution
			}
			if leg.Result.Outcome.Ack.VenueOrderID != "" &&
				(!leg.Result.Outcome.AckJournaled || !leg.Result.Outcome.AckApplied) {
				return ExitExecution
			}
		}
	}
	if refused {
		return ExitIncomplete
	}
	return ExitClean
}
```

Add the `String()` method naming all five members.

- [ ] **Step 4: Run to verify it passes**

Run: `go test -race ./internal/app/pair/ -v`
Expected: PASS.

- [ ] **Step 5: Bite-check the precedence**

```bash
tools/mutate.sh internal/app/pair/report.go <line-of-the-Exposed-check> \
  's/if Exposed(r.Verdict) {/if false {/' \
  'exposure does not reach the exit class' ./internal/app/pair/
```

Expected: `caught` by the "exposure outranks a leg needing eyes" case.

- [ ] **Step 6: Commit**

```bash
git add internal/app/pair/report.go internal/app/pair/report_test.go
git commit -m "pair: ExitClass, with the rung this root exists to report

Exercise's four plus ExitExposed: a run can end with two venues holding
unbalanced exposure, which is neither 'a leg needs eyes' nor 'nothing
was sent'. It outranks ExitExecution — a lost record is re-derivable,
an unhedged cross-venue position is money moving with nothing against
it and §6.7 cannot see it to say so."
```

---

## Task 9: this root's import fence

**Files:**
- Create: `internal/app/pair/imports_test.go`

- [ ] **Step 1: Write the test**

Copy `internal/app/reconcile/imports_test.go` and adapt. This root's fence is reconcile's **plus** the other two roots it must not become a library of, and **minus** nothing — it constructs two OMS contexts and no others.

```go
// The fence for this composition root.
//
// It is reconcile's list, and the argument transfers unchanged: a root that
// acquires a collaborator from another bounded context has acquired one that
// assumes a posture no phase here has decided. Two additions are this root's
// own:
//
//   - internal/app/exercise. It is the SINGLE-venue driver and it is the
//     nearest neighbour, which is exactly why the edge must not exist:
//     importing it would make one of the two a library, and record §95 §3's
//     whole finding is that the single-venue root cannot express this package.
//     The shared parts are reconcile.Runner and oms.Module, which both roots
//     import directly.
//   - internal/oms/application/ports and internal/oms/infrastructure/driven,
//     named here as well as policed by the singlewriter analyzer. The analyzer
//     is the gate; this entry is the note saying the omission is deliberate,
//     and it is why oms.LegState exists (task 1).
var forbiddenRoots = []string{
	"github.com/giiorg/arb-lab/internal/marketdata",
	"github.com/giiorg/arb-lab/internal/risk",
	"github.com/giiorg/arb-lab/internal/treasury",
	"github.com/giiorg/arb-lab/internal/audit",
	"github.com/giiorg/arb-lab/internal/app/capture",
	"github.com/giiorg/arb-lab/internal/app/sim",
	"github.com/giiorg/arb-lab/internal/app/exercise",
	"github.com/giiorg/arb-lab/internal/engine/dispatch",
	"github.com/giiorg/arb-lab/internal/oms/application/ports",
	"github.com/giiorg/arb-lab/internal/oms/infrastructure/driven",
}
```

`internal/strategy` is **not** on the list, and the omission is deliberate — state it in the file: this root resolves §2.4 filters from the strategy context's dated reference revision, exactly as `app/exercise` does, and `filters.go` carries the argument for why the adapter belongs at the root that plans. Reconcile's own fence has a documented exemption for the same shape of case (`permittedByGate`, task #198), and the general rule there is what this comment restates: the fence is keyed to its reason, not to its argument. Follow reconcile's file for the `go list -deps` mechanics — do not invent a second way to enumerate imports.

Add a **positive control**: assert the enumeration actually returns this package's real imports (a `go list` that silently returned nothing would make every forbidden entry pass).

```go
func TestImportEnumerationIsNotVacuous(t *testing.T) {
	t.Parallel()
	deps := packageDeps(t, "github.com/giiorg/arb-lab/internal/app/pair")
	if len(deps) == 0 {
		t.Fatal("go list returned no dependencies; every fence assertion below would pass vacuously")
	}
	if !slices.Contains(deps, "github.com/giiorg/arb-lab/internal/app/reconcile") {
		t.Error("the dependency list does not contain app/reconcile, which this root demonstrably imports")
	}
}
```

- [ ] **Step 2: Run it**

Run: `go test -race ./internal/app/pair/ -run TestImport -v`
Expected: PASS. A failure here names a real import that should not exist — remove the import, do not widen the list.

- [ ] **Step 3: Commit**

```bash
git add internal/app/pair/imports_test.go
git commit -m "pair: this root's import fence, with a non-vacuous enumeration control

app/exercise is on the list and it is the nearest neighbour, which is
why: importing it would make one of the two roots a library, and record
§95 §3's finding is that the single-venue root cannot express this
package. internal/strategy is deliberately absent — the §2.4 filter
adapter belongs at the root that plans, on filters.go's argument."
```

---

## Task 10: `arblab pair run`

**Files:**
- Create: `cmd/arblab/pair.go`
- Create: `cmd/arblab/pair_test.go`
- Modify: `cmd/arblab/main.go` (one dispatch arm at line ~68, and the usage block)

- [ ] **Step 1: Write the failing test**

Copy `cmd/arblab/exercise_test.go` and adapt. Assert: an absent `-config` returns 64; a config that will not load returns 70; each `ExitClass` maps to its code; an unrecognised class returns 70 rather than 0.

```go
func TestPairRunExitCodes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		class pair.ExitClass
		want  int
	}{
		{pair.ExitClean, 0},
		{pair.ExitOperator, 1},
		{pair.ExitIncomplete, 2},
		{pair.ExitExecution, 3},
		{pair.ExitExposed, 4},
	}
	for _, tc := range cases {
		if got := pairExitCode(tc.class); got != tc.want {
			t.Errorf("pairExitCode(%s) = %d, want %d", tc.class, got, tc.want)
		}
	}
	// A class this switch has not been taught must NOT exit 0 — reconcileRun's
	// own rule, and the reason the switch names every member instead of
	// defaulting.
	if got := pairExitCode(pair.ExitClass(99)); got == 0 {
		t.Error("an unrecognised exit class mapped to 0")
	}
}

// TestABanEarnedDuringDispatchAppearsInPairReport carries exercise's mirror
// fixture into this root. The side starts with Bans==0, its Place response earns
// a ban, and the assertion is on rendered output from the post-drive snapshot.
// Make the pair remain exposed too: ExitExposed may outrank ExitOperator, but
// the operator-visible ban must not disappear with the losing class.
func TestABanEarnedDuringDispatchAppearsInPairReport(t *testing.T) {
	t.Parallel()
	snap, servers := runnablePairConfig(t)
	servers[0].fillIOC("60")
	servers[1].banOnPlace()
	servers[0].rejectPlaceAfter(1) // preserve HalfOpen after the unwind attempt

	rep, err := pair.Run(context.Background(), snap, observability.NewRegistry, io.Discard)
	if err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}
	if rep.ExitClass() != pair.ExitExposed {
		t.Fatalf("ExitClass = %s, want ExitExposed; exposure may outrank the ban", rep.ExitClass())
	}
	var out bytes.Buffer
	writePairReport(&out, rep)
	if !strings.Contains(out.String(), "ban") {
		t.Fatalf("report omitted dispatch-time ban:\n%s", out.String())
	}
}
```

Build `runnablePairConfig` by adapting the same `TestABanDuringDispatchIsSurfaced` fixture in `internal/app/exercise/app_test.go`; its important collapse control is that the boot snapshot has zero bans and only the `Place` response increments the runtime limiter. A boot-time ban does not test the re-snapshot.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./cmd/arblab/ -run TestPairRun -v`
Expected: FAIL, `undefined: pairExitCode`.

- [ ] **Step 3: Write the implementation**

`cmd/arblab/pair.go` mirrors `cmd/arblab/exercise.go`. The exit mapping is its own function so the test can reach it without standing up a run — `exerciseRun` inlines its switch, and inlining it here would leave `TestPairRunExitCodes` unable to select the case it is about:

```go
// pairExitCode maps the report's class to a process exit code.
//
// Every member is named and none defaults, on reconcileRun's rule: a class
// added to the app package and not taught to this switch must not exit 0.
func pairExitCode(class pair.ExitClass) int {
	switch class {
	case pair.ExitClean:
		return 0
	case pair.ExitOperator:
		return 1
	case pair.ExitIncomplete:
		return 2
	case pair.ExitExecution:
		return 3
	case pair.ExitExposed:
		return 4
	}
	return 70
}

// writePairLeg renders one leg, or nothing when it was never attempted.
func writePairLeg(w io.Writer, label string, leg *pair.LegReport) {
	if leg == nil {
		return
	}
	if leg.Refusal != nil {
		fmt.Fprintf(w, "  %s %s: refused, nothing sent: %v\n", label, leg.ID, leg.Refusal)
		return
	}
	o := leg.Result.Outcome
	fmt.Fprintf(w, "  %s %s: %s venue_id=%q journaled=%t applied=%t",
		label, leg.ID, o.State, o.Ack.VenueOrderID, o.Journaled, o.Applied)
	if o.Ack.VenueOrderID != "" {
		fmt.Fprintf(w, " ack_journaled=%t ack_applied=%t", o.AckJournaled, o.AckApplied)
	}
	if o.AckConflict {
		fmt.Fprint(w, " ACK CONFLICT — venue id contradicts the store; new trading held")
	}
	if o.Cause != nil {
		fmt.Fprintf(w, " cause=%v", o.Cause)
	}
	fmt.Fprintln(w)
}
```

`pairRun` then calls `pairExitCode(report.ExitClass())`, printing the unrecognised-class line to stderr before returning 70 exactly as `exerciseRun` does.

Two further differences from the exercise shell:

1. **`pair.Run` builds its own clock** (D9), so this shell does not call `clock.NewWall` for the run — only for the bootstrap config read, exactly as `exerciseRun` does for `loadExerciseConfig`.
2. **The renderer prints per side and then the pair**, and the pair line is last because it is the answer:

```go
func writePairReport(w io.Writer, r pair.Report) {
	for i := range r.Sides {
		fmt.Fprintf(w, "side %d — %s\n", i, r.Sides[i].Venue)
		writeReconcileVerdict(w, r.Sides[i].Boot)
		writeReconcileDiagnosis(w, r.Sides[i].Boot)
		writePairLeg(w, "open  ", r.Sides[i].Open)
		writePairLeg(w, "unwind", r.Sides[i].Unwind)
		if e := r.Sides[i].Exposure; e.Established {
			fmt.Fprintf(w, "  exposure: %s %s (state %s)\n", e.Side, e.Filled, e.State)
		} else if e.ID != "" {
			fmt.Fprintf(w, "  exposure: NOT ESTABLISHED — %s\n", e.Because)
		}
		writeRateLimitStatus(w, r.Sides[i].RateLimit)
	}
	if !r.Drove {
		fmt.Fprintf(w, "pair:     not driven: %v\n", r.Refusal)
		return
	}
	fmt.Fprintf(w, "pair:     %s\n", r.Verdict)
	if pair.Exposed(r.Verdict) {
		fmt.Fprintln(w, "pair:     EXPOSURE IS LIVE AND UNBALANCED. §6.7's classifier cannot see a cross-venue")
		fmt.Fprintln(w, "pair:     pair (record §95 §4), so nothing will compensate this automatically.")
		fmt.Fprintln(w, "pair:     Recovery: `arblab reconcile run` on EACH side, then close by hand.")
	}
	if r.Refusal != nil {
		fmt.Fprintf(w, "pair:     %v\n", r.Refusal)
	}
}
```

`writeRateLimitStatus` is called unconditionally for each side before any exit-class-specific return, and a non-zero `Bans` count must always print. Exposure may outrank the ban in `ExitClass`; precedence reduces a process exit code, not operator-visible facts.

Add to `main.go`'s `dispatch`, beside the exercise arm:

```go
	if len(args) >= 2 && args[0] == "pair" && args[1] == "run" {
		return pairRun(args[2:], stdout, stderr)
	}
```

And to `usage`, after the `exercise run` line:

```
  arblab pair run         -config <file>
```

plus this paragraph, after the `exercise run` paragraph:

```
pair run drives ONE cross-venue package across TWO deployments: two
§5.2 blocks, two journals, two reconcile-only boots, and — only if both
verdicts are clean — leg A, then leg B sized to leg A's ESTABLISHED
fill. A leg that filled nothing opens no hedge; a leg whose fill could
not be read stops the run. On a half-open pair it issues one
reduce-only close of leg A. Exit codes: reconcile's three, plus 3 (a
leg was dispatched and needs a human) and 4 — THE PAIR IS EXPOSED: the
two venues hold unbalanced positions. §6.7's classifier cannot see a
cross-venue pair, so nothing compensates a 4 automatically; the
recovery path is `reconcile run` on each side and a manual close.
```

- [ ] **Step 4: Run the tests**

Run: `go test -race ./cmd/arblab/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/arblab/pair.go cmd/arblab/pair_test.go cmd/arblab/main.go
git commit -m "arblab: pair run

Its own verb rather than a crossvenue subcommand: every existing
'crossvenue' subcommand is an offline research tool, and an operator
must not be one word away from confusing a simulation with a live
order path. Exit 4 is the one the usage block spends a paragraph on —
nothing compensates an exposed pair automatically."
```

---

## Task 11: the operator's document and the demo runbook

**Files:**
- Create: `configs/pair.demo.example.json`
- Create: `docs/runbooks/pair-demo-run.md`

- [ ] **Step 1: Write the example document**

`configs/pair.demo.example.json`: two sides, `binance` (`demo-fapi.binance.com`) and `bybit` (`api-demo.bybit.com`), linear-perp only, `LABUSDT`, journals at `./run/pair/binance.db` and `./run/pair/bybit.db`. **Secret refs only** — `env:BINANCE_DEMO_API_KEY` and friends — never a value; `platform/config`'s `SecretRef` has no field for one and an inline credential fails to parse.

Copy the §5.2 block's field set from an existing working document (there is one in `internal/app/exercise/app_test.go`'s `reconcileConfig` if `configs/` has none) and fill the rate-limit rows from `docs/venue-profiles.md` rather than from memory — a limit row is a constant asserting an external fact, and it is its own fixture.

The pair block contains a syntactically valid UUIDv7 so the committed example itself validates. **It is an example value, not a reusable run id: replace it with a freshly minted UUIDv7 before every run, exactly as runbook step 4 requires.**

```json
"pair": {
  "id": "018f0c1a-2b3c-7d4e-8f90-1a2b3c4d5e6f",
  "attempt": 0,
  "mode": "sequential",
  "settle": "3s",
  "settle_attempts": 5,
  "legs": [
    {"venue": "binance", "product": "linear_perp", "contract": "linear", "symbol": "LABUSDT",
     "side": "sell", "quantity": "100", "limit_price": "0.0840", "unwind_limit_price": "0.0860"},
    {"venue": "bybit", "product": "linear_perp", "contract": "linear", "symbol": "LABUSDT",
     "side": "buy", "quantity": "100", "limit_price": "0.0855", "unwind_limit_price": "0.0835"}
  ]
}
```

Prices are placeholders and the runbook says so — record §95 §2's LAB quote was `0.0845 / 0.08450` at one instant on 2026-08-15, and a marketable limit must be set from a fresh book read, not from a document written days earlier.

- [ ] **Step 2: Write the runbook**

`docs/runbooks/pair-demo-run.md`, covering in order:

1. **Preconditions.** `.env` holds `BINANCE_DEMO_*` and `BYBIT_DEMO_*`. **The Bybit demo key expires 2026-11-10T07:03:17Z** (record §95 §2). Both keys were confirmed to carry trade permission on 2026-08-15; that is a fact with a date, not a standing guarantee.
2. **Fetch a fresh reference revision** covering both venues' `LABUSDT` linear-perp rows: `arblab reference fetch -reference <in.json> -out configs/reference.crossvenue.<date>.json`. **Named limitation:** `referencePublicConfig` in `cmd/arblab/reference.go` targets the **mainnet** public hosts (`fapi.binance.com`, `api.bybit.com`), not the demo hosts. For Binance that is the same environment. **For Bybit it is not** — record §95 §2 measured 821 demo symbols against 809 testnet, and demo is a distinct environment. So a Bybit demo grid may differ from the mainnet grid this revision records, and a mismatch shows up as a `Plan` refusal or a venue rejection rather than as a wrong fill. Check the refusal against the demo `instruments-info` before widening anything.
3. **Read both books** and set the two `limit_price` values marketable and the two `unwind_limit_price` values marketable in the opposite direction. State the rule: a limit that is not marketable turns an IOC into a no-fill, which is a *clean* run under D6 and proves nothing about fills.
4. **Mint a fresh `pair.id` per run.** §5.3 forbids ClientOrderID reuse within the recoverable window and the planner refuses a repeat — which is the fail-safe, and it means a re-run with the same id fails rather than double-sending.
5. **Dry checks before the live run:** `arblab reconcile run -config <side A>` and again for side B, both expecting exit 0. If either is not 0, `pair run` will refuse anyway, and the single-side command names the problem more directly.
6. **The run:** `arblab pair run -config configs/pair.demo.<date>.json`.
7. **Reading the exit code**, including the one that matters: **exit 4 means the two venues hold unbalanced positions and nothing will fix it automatically.** Recovery is `arblab reconcile run` on each side, then a manual reduce-only close.
8. **What a green run proves and what it does not.** It proves: both wire contracts against real matching engines, both signed transports, both journal paths, the plan→journal→dispatch path, the two-leg gate, and the paired report. It does **not** prove: that demo books persist at other hours, that an IOC fills on a demo alt, that demo funding matches mainnet intervals, that mainnet grids match demo grids, or that anything here is evidence about the strategy's economics. Record §95 §8 states the same list and this section is its operational form.

- [ ] **Step 3: Verify the example document actually validates**

Add a test in `cmd/arblab/pair_test.go` that loads `configs/pair.demo.example.json` and calls `Validate()`. This is the check that stops the committed example rotting silently — a document nobody loads is prose.

```go
func TestExampleDocumentValidates(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("../../configs/pair.demo.example.json")
	if err != nil {
		t.Fatalf("read example: %v", err)
	}
	wall, err := clock.NewWall(nil)
	if err != nil {
		t.Fatalf("NewWall: %v", err)
	}
	store, err := config.New[pair.Config]("pair.demo.example.json", wall.Now(), data)
	if err != nil {
		t.Fatalf("the committed example does not parse: %v", err)
	}
	if err := store.Current().Value.Validate(); err != nil {
		t.Fatalf("the committed example does not validate: %v", err)
	}
}
```

If `Validate` needs the secret env vars set, set them in the test with `t.Setenv` to obviously-fake values — the document holds refs, and resolving them is a boot phase, not validation.

- [ ] **Step 4: Run**

Run: `go test -race ./cmd/arblab/ -v`
Expected: PASS, including `TestExampleDocumentValidates`.

- [ ] **Step 5: Commit**

```bash
git add configs/pair.demo.example.json docs/runbooks/pair-demo-run.md cmd/arblab/pair_test.go
git commit -m "pair: the demo document and its runbook, with the example under test

The committed example is loaded and validated by a test, because a
document nobody loads is prose. The runbook names two limits rather
than leaving them to be discovered: `reference fetch` reads the
MAINNET public hosts, which is the same environment on Binance and a
different one on Bybit; and a non-marketable limit turns an IOC into a
no-fill, which is a CLEAN run that proves nothing about fills."
```

---

# PHASE 2 — the paired fan-out

Phase 1 is sequential: leg B is never in flight while leg A's fate is unknown, which is what makes its failure cases small. That safety costs the thing the strategy actually wants — the time between the two legs is time the pair is unhedged, and a dislocation can close inside it.

Phase 2 sends both legs at once and composes the verdict at a fan-in. It does **not** change what happens after the verdict: the same `Compose`, the same `ExitClass`, the same D8 refusal.

## Task 12: paired fan-out and fan-in

**Files:**
- Modify: `internal/app/pair/app.go`
- Test: `internal/app/pair/app_test.go`

**Interfaces:**
- Consumes: everything from phase 1.
- Produces: paired behavior for task 3's `ModePaired`. The declared `Mode` type, its two constants, validation refusal, `validConfig` value and example field already exist in phase 1, so adding behavior here cannot invalidate every earlier fixture.

- [ ] **Step 1: Write the failing tests**

```go
// TestPairedModeSendsBothLegsBeforeEitherIsEstablished is the mode's defining
// property. The venue doubles record the ORDER of the calls they saw, and the
// assertion is that side B's Place arrived before side A's first join —
// otherwise this is sequential mode wearing a different name.
func TestPairedModeSendsBothLegsBeforeEitherIsEstablished(t *testing.T) {
	t.Parallel()
	snap, servers := runnableConfig(t)
	setMode(&snap, ModePaired)
	servers[0].fillIOC("100")
	servers[1].fillIOC("100")

	if _, err := Run(context.Background(), snap, observability.NewRegistry, io.Discard); err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}
	if !servers[1].sawPlaceBefore(servers[0].firstJoinAt()) {
		t.Fatal("side B's Place arrived after side A's first join; that is sequential mode")
	}
}

// TestPairedModeHoldsBothPlacesOpenAtOnce is the assertion the test above
// CANNOT make, and without it the mode's defining property is unchecked.
//
// "B's Place happened before A's first join" is satisfied by a purely serial
// Place(A) → Place(B) → Join(A), which is not concurrent fan-out at all: it
// restores one venue round-trip of leg-time while leaving the test green. The
// only fixture that separates the two is one where BOTH requests must be in
// flight simultaneously for either to complete, so a serial implementation
// deadlocks and the bound reports it rather than hanging.
func TestPairedModeHoldsBothPlacesOpenAtOnce(t *testing.T) {
	t.Parallel()
	snap, servers := runnableConfig(t)
	setMode(&snap, ModePaired)

	// Each double blocks inside Place until both have entered. A sequential
	// implementation can never satisfy the second arrival, so the barrier
	// times out and names the failure instead of hanging the suite — a test
	// that hangs is not a test that fails.
	barrier := newPlaceBarrier(t, 2, time.Second)
	servers[0].blockInPlace(barrier)
	servers[1].blockInPlace(barrier)
	servers[0].fillIOC("100")
	servers[1].fillIOC("100")

	if _, err := Run(context.Background(), snap, observability.NewRegistry, io.Discard); err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}
	if !barrier.bothArrived() {
		t.Fatal("the two Place calls were never in flight together; that is sequential dispatch wearing paired mode's name")
	}
}

// TestPairedModeSizesBothLegsFromTheDocument. Sequential mode sizes B from A's
// fill; paired mode CANNOT — nothing is established when B is sent. Stating it
// as a test stops the two modes' sizing rules being conflated later.
func TestPairedModeSizesBothLegsFromTheDocument(t *testing.T) {
	t.Parallel()
	snap, servers := runnableConfig(t)
	setMode(&snap, ModePaired)
	servers[0].fillIOC("60") // a partial fill that sequential mode WOULD have used
	servers[1].fillIOC("100")

	if _, err := Run(context.Background(), snap, observability.NewRegistry, io.Discard); err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}
	if got := servers[1].lastPlaceQuantity(); got != "100" {
		t.Errorf("side B was asked for %q, want the document's \"100\" — nothing is established at fan-out", got)
	}
}

// TestPairedModeSkewIsRefusedNotCompensated. Paired mode's characteristic
// outcome is a skewed pair, and D8 is unchanged: this root does not compensate
// one, because sizing that residual is §6.7's job and §6.7 cannot see the pair.
func TestPairedModeSkewIsRefusedNotCompensated(t *testing.T) {
	t.Parallel()
	snap, servers := runnableConfig(t)
	setMode(&snap, ModePaired)
	servers[0].fillIOC("100")
	servers[1].fillIOC("60")

	rep, err := Run(context.Background(), snap, observability.NewRegistry, io.Discard)
	if err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}
	if rep.Verdict != VerdictSkewed {
		t.Fatalf("Verdict = %s, want VerdictSkewed", rep.Verdict)
	}
	for i := range rep.Sides {
		if rep.Sides[i].Unwind != nil {
			t.Errorf("side %d was compensated on a skewed pair", i)
		}
	}
	if rep.ExitClass() != ExitExposed {
		t.Errorf("ExitClass = %s, want ExitExposed", rep.ExitClass())
	}
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/app/pair/ -run TestPairedMode -v`
Expected: FAIL — `ModePaired` validates but `Run` is still sequential unconditionally.

- [ ] **Step 3: Write the implementation**

Rename phase 1's `run` body to `runSequential`, then switch on task 3's declared `Mode` in the `run` entry point. Name both constants and do not add a `default`; the type exists so `exhaustiveswitch` can refuse a new mode whose execution behavior was not written:

```go
switch d.cfg.Pair.Mode {
case ModeSequential:
	d.runSequential()
case ModePaired:
	d.runPaired()
}
```

```go
// runPaired sends both legs before either is established.
//
// # What it buys and what it costs, both stated
//
// It buys leg-time: the interval between the two sends is the interval the pair
// is unhedged, and in sequential mode that interval includes a bounded settle
// loop against a venue. It costs the sequential mode's whole safety story —
// leg B is sized from the DOCUMENT because nothing is established when it goes
// out, so a partial fill on either side produces a SKEWED pair, which this root
// will not compensate (D8).
//
// # The fan-out is concurrent and the fan-in is not
//
// Two goroutines, one per side, each doing plan-then-execute against its own
// module. Nothing is shared between them: two modules, two journals, two venue
// clients, two checklists. The fan-in joins both and composes ONE verdict, and
// it runs on this goroutine, because composing is where a second writer would
// be a defect rather than a slowdown.
func (d *drive) runPaired() {
	var wg sync.WaitGroup
	var opened [2]*LegReport
	var errs [2]error
	for i := range d.cfg.Pair.Legs {
		wg.Add(1)
		go func(leg int) {
			defer wg.Done()
			opened[leg], errs[leg] = d.open(leg, d.mustQuantity(leg))
		}(i)
	}
	wg.Wait()

	for i := range opened {
		side := d.sideForLeg(i)
		d.rep.Sides[side].Open = opened[i]
		if errs[i] != nil {
			d.rep.Refusal = errors.Join(d.rep.Refusal, errs[i])
			d.rep.Sides[side].Exposure = unopenedPartner(d.cfg, i)
			continue
		}
		var err error
		d.rep.Sides[side].Exposure, err = d.establish(i, opened[i].ID)
		if err != nil {
			d.rep.Refusal = errors.Join(d.rep.Refusal, err)
		}
	}
	var cerr error
	sideA, sideB := d.sideForLeg(0), d.sideForLeg(1)
	d.rep.Verdict, cerr = Compose(d.rep.Sides[sideA].Exposure, d.rep.Sides[sideB].Exposure)
	d.rep.Refusal = errors.Join(d.rep.Refusal, cerr)
	d.compensate() // the same D8 block phase 1's run() ends with, extracted
}
```

Task 7 already extracted phase 1's unwind block into `d.compensate()` and put it at the sequential path's single tail. Paired mode calls that same method after fan-in; two copies is how the modes' compensation rules come to differ silently.

- [ ] **Step 4: Run all tests with the race detector**

Run: `go test -race -count=5 ./internal/app/pair/ -v`
Expected: PASS. `-count=5` because the fan-out is concurrent and a data race that shows up one run in five is the whole reason `make test` is race-enabled.

- [ ] **Step 5: Bite-check the mode split**

```bash
tools/mutate.sh internal/app/pair/app.go <line-of-the-mode-switch> \
  's/case ModePaired:/case Mode("paired_DISABLED"):/' \
  'paired mode falls through to sequential' ./internal/app/pair/
```

Expected: `caught`, from the behavioural tests — **not** `NOBUILD!!`.

The `NOBUILD!!` expectation would be wrong, and the reason is worth stating because it is a standing trap for every gate bite-check in this repo: `tools/mutate.sh` runs `go test -C "${MUTATE_MODULE:-.}"` and nothing else (`tools/mutate.sh:147`). It never invokes the custom analyzers. The mutated switch is still valid Go — `Mode("paired_DISABLED")` compiles fine — so the build succeeds and only the behaviour changes. `exhaustiveswitch` reports when *the analyzer runs*, which is `make lint`, not `go test`.

So the gate needs its own bite-check, run separately, and it is worth having because `Mode`'s whole reason for being a declared type rather than a bare `string` is that this analyzer covers it:

```bash
# The analyzer's own bite-check: apply the mutant, run the LINT, restore.
git stash list >/dev/null   # ensure a clean tree first
sed -i '' 's/case ModePaired:/case Mode("paired_DISABLED"):/' internal/app/pair/app.go
make lint    # expect: exhaustiveswitch names the missing ModePaired arm
git checkout -- internal/app/pair/app.go
```

A `make lint` that stays green here means `Mode` is not being read as an enum-like — check that it has at least `minMembers` (2) declared constants of the type and that the switch tag really is a `pair.Mode`.

- [ ] **Step 6: Commit**

```bash
git add internal/app/pair/app.go internal/app/pair/app_test.go
git commit -m "pair: paired mode — both legs out before either is established

It buys leg-time, which is the interval the pair is unhedged, and it
costs sequential mode's safety story: leg B is sized from the document
because nothing is established at fan-out, so a partial fill on either
side produces a skewed pair. D8 is unchanged and the two modes share
one copy of it — sizing that residual is §6.7's job, and §6.7 cannot
see this pair. Mode is required with no default: two failure surfaces,
and a defaulted mode is a decision nobody made."
```

---

## Task 13: `make ci` green, both modules

- [ ] **Step 1: Run the whole gate**

Run: `make ci`
Expected: green. It runs `fmt-check`, `tutorial-ka-check`, `formal-fresh`, `vet`, `lint` (the six custom analyzers) and race tests, over **both** modules.

- [ ] **Step 2: Check the two gates most likely to bite this slice**

`singlewriter` fires if anything under `internal/app/pair` reached `oms/application/ports` or `oms/infrastructure/driven` — task 1 exists so it does not have to. `exhaustiveswitch` fires on any switch over `Verdict`, `ExitClass` or `Mode` that stopped naming a member.

Run: `go vet ./... && ./bin/arblab-lint ./...` (or whatever `make lint` invokes — read the Makefile target rather than guessing).

- [ ] **Step 3: Confirm `formal-fresh` is untouched**

Run: `make formal-fresh`
Expected: green with no new `knownStale` markers. This slice edits no tracked spec section — the one `[SPEC EDIT OWED]` marker from task 3 is a *code* marker naming §12 item 7, not a normative edit. If `formal-fresh` reports a change, something outside this plan's file list was touched; find it before continuing.

- [ ] **Step 4: Confirm the OWED grep says exactly one**

Run: `grep -rn "SPEC EDIT OWED" internal/ tools/`
Expected: exactly one hit — `internal/app/pair/config.go`, the §12 item 7 marker. Task 14 is what discharges it.

- [ ] **Step 5: Commit any formatting fixes**

```bash
git add -A
git commit -m "pair: make ci green, both modules"
```

---

## Task 14: the record and the handoff

**Files:**
- Modify: `docs/superpowers/specs/2026-08-15-m4-236-crossvenue-root-scoping.md` (append §10)
- Modify: `docs/HANDOFF.md` (new session block at the top)

- [ ] **Step 1: Append §10 to record §95**

Cover, in this order:

1. **What was built**, against ruling 1: `internal/app/pair`, two `reconcile.Runner`s, two journals, two `oms.Module`s, sequential mode and paired mode, `arblab pair run`.
2. **What was built against ruling 2**, and where the pair concept lives: `pair.Verdict` and `pair.Compose`, in the root and not in the kernel. `domain.HedgeGroup` is unchanged and `Module.Classify` still has no production caller.
3. **What #236 actually ships, named as §95 §7's alternative (v)** — the root reports both per-venue verdicts side by side and composes above them, with cross-venue classification **owed**. §95 §7 (v) warned: *"If #241 slips, the root must not silently become (v) with no marker."* The markers are: `ErrUnwindRefused`'s doc, the `Verdict` type's doc, and `pair.go`'s package comment, all three naming §6.7 and record §95 §4 by section.
4. **The one `[SPEC EDIT OWED]`**, discharged here: §12 item 7's *"live wiring"* and §5.2's single-venue sentence both need a clause naming the two-deployment shape. Land the spec edit in this step and remove the marker, or — if the edit belongs with #241's larger re-scope — move the marker's text into #241's task entry and say so explicitly. **Do not leave it in the code past this slice**; the slice-close gate is the grep and it cannot see an unmarked claim, so a marker carried forward silently is worse than either option.
5. **Four things owed, with their reasons:**
   - **#241** (unchanged): the pair as a kernel concept, its residual composition rule, and whether it is §5.1's fifth journal record scope.
   - **Quantization (D5):** a fill that fails the partner's grid unwinds the whole leg rather than hedging the quantized part and unwinding the remainder. Owed; not urgent while runs are fixed-size.
   - **A durable pair record:** the shared `TradeID` correlates the two legs in both journals, which is the fact ruling 2 asked for — but nothing states that the pair *should* have had two legs. A crash between leg A and leg B leaves side A's journal holding a leg whose partner was never planned anywhere. That is precisely §5.1's fifth-record-scope question, and it belongs to #241.
   - **Half-open on side B:** phase 1 opens side 0 first, so a side-1-only exposure is a contradiction rather than a case. Paired mode can produce it; the compensation for it is refused (D8) rather than handled.
6. **What the demo run proved, if one was performed**, with its date, its symbol, its two exit codes and its limits — record §95 §8's not-claimed list, restated against the actual run rather than in the abstract. **If no live run was performed, say so in one sentence.** A record that is silent about whether the thing ran is the shape §95 §9's own method note is about.

- [ ] **Step 2: Write the HANDOFF block**

Lead with what a fresh session must not re-derive: the three rulings are settled, the three unknowns are measured (§95 §2, dated 2026-08-15), and the pair concept lives in the root by ruling 3's split. Then the next action, then the still-open list carried forward from session 140's block (#225, #226, #235, #237, #238, #239, #240, #241, the five `knownStale` markers, the capture still down by the 2026-08-15 decision).

- [ ] **Step 3: Final verification**

```bash
make ci
grep -rn "SPEC EDIT OWED" internal/ tools/
git status
```

Expected: `make ci` green, the grep at **zero** (step 1 item 4 discharged the one marker), tree clean after the commit below.

- [ ] **Step 4: Commit**

```bash
git add docs/
git commit -m "m4 #236: the cross-venue root is built — and what it ships is (v) with the markers on

Ruling 1's shape: two runners, two journals, two modules, one package
spanning venues. Ruling 2's concept: pair.Verdict, in the root, with
§5.1's key untouched. Ruling 3's split holding: §6.7 is not edited and
Classify still has no production caller.

What #236 actually ships is §95 §7's alternative (v), which §95 itself
predicted — and the warning attached to it was that the root must not
silently BECOME (v) with no marker. Three markers name §6.7 and §95 §4
by section, and #241 carries the normative half."
```

---

## What This Plan Does Not Build, And Why

Each of these was considered and left out on a stated ground, so a later session does not read the absence as an oversight.

- **Any edit to `domain.HedgeGroup`, §5.1 or §6.7.** Record §95 ruling 3: the normative re-scope is task #241. Splitting them keeps a live-venue slice from carrying a spec edit to the safety kernel.
- **A cross-venue call to `Module.Classify`.** §6.7 computes a residual per hedge group and a cross-venue pair sits in two groups; driving it would flatten both halves of a healthy position, once per venue, in opposite directions (record §95 §4). It has no production caller today and this plan does not give it one.
- **A durable pair record in the journal.** Whether the pair is §5.1's **fifth** journal record scope is explicitly #241's question (record §95 §8). The shared `TradeID` gives the correlation without one; what it does not give is a statement that a pair *was intended*, which is the gap task 14 records.
- **Quantizing leg A's fill to leg B's grid.** D5 refuses instead, which is correct and coarse. Hedging the quantized part and unwinding the remainder is better and needs a lot-grid rounding decision with an explicit direction — a money-rounding argument that deserves its own slice.
- **Position-aware restoration from a non-flat start.** The only compensation built here is a reduce-only inverse of the opening leg. Against a pre-existing position that inverse can increase exposure instead of restoring it, so the task-5 flat-start gate refuses the coarse case rather than pretending the journal's zero or an absent reconciliation assertion proves flatness.
- **Live fill streaming.** Both sides read fills by re-joining, bounded by `settle_attempts`. A websocket user-data stream is milestone 7's, and the bounded re-join is what makes this root a one-shot tool rather than a loop.
- **B4, the loop that drives this root repeatedly.** #225 and #226 remain its only blockers and neither blocks this plan. This root is the thing B4 will drive.
- **More than two sides.** D3: leg indices restart at 0 per side, so a third side would need a leg-index base on `application.TradePlan`, which is a kernel change.
- **Anything that reads the demo run as evidence about the strategy.** The run proves wire contracts, transports, journals and the two-leg gate. Record §95 §8's not-claimed list stands unchanged: demo books may not persist, an IOC may not fill, demo funding may not match mainnet intervals.

---

## Self-Review

**Spec coverage.** Record §95 §6's three rulings: ruling 1 → tasks 3, 5, 6, 12 (two runners, two journals, two modules, a package spanning venues, paired dispatch and fan-in). Ruling 2 → task 2 (the concept above the hedge group; §5.1's key untouched, verified by task 9's fence and by the absence of any `hedgegroup.go` edit in the file list). Ruling 3 → task 14 (#241 carries the normative edit; #236 states which model it follows). §95 §7 (ii) as the **first phase** → tasks 5–8 are exactly (ii): venue A, gate on verdict, venue B, reduce-only unwind of A on B's failure. §95 §7 (v)'s warning about becoming (v) unmarked → task 14 step 1 item 3 names the three markers. §95 §8's four owed items → task 14 step 1 item 5. §95 §2's measured unknowns are consumed, not re-verified: LABUSDT is the fixture symbol throughout, the Bybit key expiry is in the runbook, and `chaos_test.go` is not touched.

**Placeholder scan.** Every code step carries real code. Four steps say "read file X and copy its helper rather than inventing a second one" — tasks 1, 3, 4, 5, 9, 10 — and each names the exact file and, where it exists, the line. That is a deliberate instruction to reuse an existing fixture, not a deferred decision: writing a second `reconcileConfig` or a second `go list` walker is how two fixtures come to disagree. Three steps say "if the real signature differs, use the real one" (tasks 1, 2, 7) — those are guards against my having mis-read a field name during planning, and each names where to look.

**Type consistency.** `LegExposure` is produced by `Establish` (task 2) and consumed by `Compose` (task 2), `drive.establish` (task 6), the unwind guard (task 7) and the renderer (task 10) — same field names throughout. `LegReport` is produced by `drive.open`/`drive.unwind` (tasks 6, 7) and consumed by `Report.ExitClass` (task 8) and the renderer (task 10) — `Refusal` carries every attempted-but-not-sent path and `Result application.LegResult` carries a sent result, so no unused attempted flag can disagree with them. `oms.LegState` is consumed by `Establish`; `oms.VenuePosition` is consumed by the flat-start gate (task 5), and both projections are task 1's permitted bridge over the `ports` fence. `Config.openPlan`/`unwindPlan` (task 3) are the only plan builders and both take a `fixedpoint.Quantity` parameter — the sizing decision has exactly one home. `Verdict` is switched over in `String` and `Exposed` (task 2) and compared in tasks 6, 7, 8, 12; `Mode` is switched over at the drive entry point; `exhaustiveswitch` covers all three switches.

**One gap found and closed during review.** Task 8's `ExitClass` reads `r.Sides[i].Unwind` as well as `r.Sides[i].Open`, which task 7 introduces. Task 8 comes after task 7, so the field exists — but task 5 declares `SideReport` with both fields from the start, which is why `Open` and `Unwind` are both in task 5's `Produces` block rather than appearing later.
