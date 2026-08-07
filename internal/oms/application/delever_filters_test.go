package application_test

import (
	"errors"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/oms/application"
	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// Task #184: SM §7's ladder re-verifies §2.4's grid and §69's row date before
// each leg reaches the venue — the check the Executor's preflight does for the
// package path and that the ladder did not do at all.
//
// # What made the gap invisible, and what these fixtures therefore vary
//
// Every delever fixture in the suite happens to conform: the legs are sized in
// whole multiples of both instruments' steps at a limit price on both ticks. So
// "the check runs" and "the check does not run" gave the same answer everywhere
// — this repo's collapse tell, the fixture not containing the thing under test.
// The batteries below vary the grid, the row's DATE, and the INSTANT the row is
// judged at, one at a time, because those are three different predicates and an
// off-grid case alone would demonstrate only the first.
//
// # Which grid member each rung can actually break, which is a finding
//
// verifyLeg requires the leg's quantity to EQUAL the generation's own figure —
// the target for the perp leg, the authorized size for the spot leg. So the
// quantity members (lot step, min qty, min notional) are reachable from this
// ladder only when the GENERATION fixed an off-grid quantity, and in that world
// no conforming leg exists at all: the rung is stuck whatever this check does,
// because a venue would refuse the same order. The limit PRICE is the member the
// generation does not constrain, so it is the one whose refusal is recoverable —
// and it is therefore what the anti-stranding fixture below has to vary. See
// task #206, filed for the size that fixes itself off-grid.

// spotLegAt is spotLeg with the limit price exposed. The price is the one grid
// member the generation does not pin, so it is where a delever leg can be
// off-grid and STILL be replaceable by a conforming one.
func spotLegAt(t *testing.T, n int, size, limit string) ports.OrderRecord {
	t.Helper()
	rec := spotLeg(t, n, size)
	rec.Order.LimitPrice = price(t, limit)
	return rec
}

// TestTheLadderRefusesAnOffGridPerpLeg is #184 at the first rung, on the LOT
// STEP: 1.01 is not a whole multiple of the perp's 0.05 step, while the price is
// on tick and the notional clears its minimum — so nothing but the step can be
// responsible for the refusal.
//
// The error is asserted with its two consequences, because the error alone is
// satisfied by an implementation that refuses AFTER submitting: nothing reached
// the venue, and nothing reached the generation journal.
func TestTheLadderRefusesAnOffGridPerpLeg(t *testing.T) {
	t.Parallel()

	f := newDeleverer(t, neutralPackage(t)...)
	f.open("1.01")

	leg := f.add(perpLeg(t, 10, "1.01"))
	if _, err := f.deleverer.SubmitPerpReduction(hedgeGroup(), ref(leg)); !errors.Is(err, application.ErrQuantityOffLotStep) {
		t.Fatalf("SubmitPerpReduction(off-grid) = %v, want ErrQuantityOffLotStep", err)
	}
	if f.placer.calls != 0 {
		t.Errorf("placed %d orders, want 0 — the refusal must precede the send", f.placer.calls)
	}
	// The opening record and nothing else: Open wrote `planned`, and the
	// refused rung must have written nothing. Asserted as the whole sequence
	// rather than as "no perp record", so an implementation writing SOME other
	// record on the refused path is caught too.
	if got := f.journal.phases(); len(got) != 1 || got[0] != domain.GenerationPlanned {
		t.Errorf("generation journal holds %v, want [Planned] alone — the refusal must precede the record", got)
	}
	g, ok := f.deleverer.Generation(hedgeGroup())
	if !ok || g.Phase() != domain.GenerationPlanned {
		t.Errorf("generation at %v (live=%t), want Planned — a read-only refusal moves nothing",
			g.Phase(), ok)
	}
}

// TestTheLadderRefusesAnOffTickSpotLegAndStillAdmitsAConformingOne is the
// mirror, and it carries the claim the perp case cannot: a refusal at the SECOND
// rung does not STRAND the ladder.
//
// This is the rung where refusing looks like the shape §7's quiesce rule forbids
// — the perp reduction is confirmed and the ledger entry is reserved, so a
// refusal that ended the ladder here would leave a commitment nothing discharges
// (the 16-state stuttering behaviour TLC returned for a premise-GUARDED
// SubmitSpotReduction). It is not that shape, and the last assertion is why: the
// same rung admits the moment a conforming leg is presented, because everything
// the check does is a read taken before any journal write and before the
// aggregate advances.
func TestTheLadderRefusesAnOffTickSpotLegAndStillAdmitsAConformingOne(t *testing.T) {
	t.Parallel()

	f := newDeleverer(t, neutralPackage(t)...)
	f.open("1")
	f.toSpotAuthorization("1", "0.4")

	// 50010 is off the spot instrument's 25 tick; the quantity is exactly the
	// authorized 0.4 and on the 0.025 step, so the tick is the only member left.
	bad := f.add(spotLegAt(t, 11, "0.4", "50010"))
	if _, err := f.deleverer.SubmitSpotReduction(hedgeGroup(), ref(bad)); !errors.Is(err, application.ErrPriceOffTick) {
		t.Fatalf("SubmitSpotReduction(off-tick) = %v, want ErrPriceOffTick", err)
	}
	// One send, and it is the PERP leg's — so the count distinguishes "the spot
	// leg was refused" from "no leg was ever sent", which a zero would not.
	if f.placer.calls != 1 {
		t.Errorf("placed %d orders, want 1 (the perp leg only)", f.placer.calls)
	}
	for _, c := range f.journal.calls {
		if c.kind == "spot" {
			t.Fatalf("generation journal holds a spot record %v, want none", f.journal.phases())
		}
	}
	g, _ := f.deleverer.Generation(hedgeGroup())
	if g.Phase() != domain.GenerationSpotReduceAuthorized {
		t.Fatalf("generation at %v, want SpotReduceAuthorized — the rung must stay re-presentable", g.Phase())
	}

	// The recoverability claim, and the reason this refusal is admissible where
	// the premise guard was not: the ladder is exactly where it was, so the
	// operator's fix — a leg replanned onto the grid — is presentable against the
	// SAME generation. An implementation that consumed the rung on refusal
	// passes everything above and fails here.
	good := f.add(spotLegAt(t, 12, "0.4", "50000"))
	if _, err := f.deleverer.SubmitSpotReduction(hedgeGroup(), ref(good)); err != nil {
		t.Fatalf("SubmitSpotReduction(conforming, after a refusal) = %v, want sent", err)
	}
	if f.placer.calls != 2 {
		t.Errorf("placed %d orders, want 2", f.placer.calls)
	}
}

// TestTheLadderRefusesALegAgainstAStaleFiltersRow is §69's half, and it is what
// distinguishes the shared policy from half of it.
//
// The leg CONFORMS — same quantity, same price, same grid as every passing
// fixture in the suite. Only the row's observation instant moves. So an
// implementation that called conformsToFilters instead of the policy's admits
// passes every other test in this file and fails only this one.
func TestTheLadderRefusesALegAgainstAStaleFiltersRow(t *testing.T) {
	t.Parallel()

	stale := filtersObservedAt(t, testNow.Add(-48*time.Hour))
	f := newDelevererOn(t, testClock(t), stale, testFilterMaxAge, neutralPackage(t)...)
	f.open("1")

	leg := f.add(perpLeg(t, 10, "1"))
	if _, err := f.deleverer.SubmitPerpReduction(hedgeGroup(), ref(leg)); !errors.Is(err, application.ErrFiltersStaleAtAdmission) {
		t.Fatalf("SubmitPerpReduction(stale row) = %v, want ErrFiltersStaleAtAdmission", err)
	}
	if f.placer.calls != 0 {
		t.Errorf("placed %d orders, want 0", f.placer.calls)
	}

	// The positive control the case above needs: the SAME leg against a FRESH
	// row is sent. Without it, a ladder that refused every leg for any reason
	// would score a pass here.
	fresh := newDeleverer(t, neutralPackage(t)...)
	fresh.open("1")
	if _, err := fresh.deleverer.SubmitPerpReduction(hedgeGroup(), ref(fresh.add(perpLeg(t, 10, "1")))); err != nil {
		t.Fatalf("SubmitPerpReduction(fresh row) = %v, want the leg sent", err)
	}
}

// TestTheLadderJudgesEachLegAtItsOwnInstant is this design's load-bearing
// difference from the package path, and nothing else in the suite can see it.
//
// The Executor stamps ONE instant for a whole package because its legs go out
// together. This ladder cannot: §7 sequences its two legs across a venue round
// trip and a finality wait, precisely because the spot cap is the CONFIRMED perp
// reduction. So a row that was fresh when the perp leg went out can be stale by
// the time the spot leg is sized, and the ladder must judge it again rather than
// reuse the perp leg's verdict.
//
// The fixture puts the row 23 hours old against a 24-hour bound, sends the perp
// leg, then advances the clock. An implementation that stamped one instant per
// GENERATION admits the spot leg at any age; one that reads per leg refuses it
// past the bound. Both branches are asserted, because "refused" alone is also
// what a ladder refusing every second leg would produce.
func TestTheLadderJudgesEachLegAtItsOwnInstant(t *testing.T) {
	t.Parallel()

	ladderTo := func(t *testing.T, wait time.Duration) error {
		t.Helper()
		clk := testClock(t)
		rows := filtersObservedAt(t, testNow.Add(-23*time.Hour))
		f := newDelevererOn(t, clk, rows, testFilterMaxAge, neutralPackage(t)...)
		f.open("1")

		perp := f.add(perpLeg(t, 10, "1"))
		if _, err := f.deleverer.SubmitPerpReduction(hedgeGroup(), ref(perp)); err != nil {
			t.Fatalf("SubmitPerpReduction at row age 23h against a 24h bound = %v, want sent", err)
		}
		if err := f.deleverer.RecordPerpFill(hedgeGroup(), qty(t, "1")); err != nil {
			t.Fatalf("RecordPerpFill: %v", err)
		}
		if err := f.deleverer.ConfirmPerpReduction(hedgeGroup()); err != nil {
			t.Fatalf("ConfirmPerpReduction: %v", err)
		}
		// The finality wait §7's sequencing forces. It is the ONLY thing the two
		// cases below differ in.
		if err := clk.Advance(wait); err != nil {
			t.Fatalf("Advance(%s): %v", wait, err)
		}
		if err := f.deleverer.AuthorizeSpotReduction(hedgeGroup(), qty(t, "1")); err != nil {
			t.Fatalf("AuthorizeSpotReduction: %v", err)
		}
		_, err := f.deleverer.SubmitSpotReduction(hedgeGroup(), ref(f.add(spotLeg(t, 11, "1"))))
		return err
	}

	// Inside the bound at both instants: the ladder completes, so the refusal
	// below is attributable to the wait and not to the second rung.
	if err := ladderTo(t, 30*time.Minute); err != nil {
		t.Fatalf("spot leg at row age 23h30m = %v, want sent", err)
	}
	// The same row, the same leg, two hours later: 25h against a 24h bound.
	if err := ladderTo(t, 2*time.Hour); !errors.Is(err, application.ErrFiltersStaleAtAdmission) {
		t.Fatalf("spot leg at row age 25h = %v, want ErrFiltersStaleAtAdmission", err)
	}
}

// TestTheLadderNamesItsOwnRequirementBeforeTheVenuesGrid pins the ORDER of
// verifyLeg's five checks, which no one-violation-at-a-time fixture can see.
//
// A leg that breaks the generation's size requirement AND the venue's lot step
// has two true findings, and only one of them sends an operator to the right
// place: ErrGenerationLegMissized names the ladder's own arithmetic,
// ErrQuantityOffLotStep names the reference map. Swap the two checks and every
// single-violation fixture in this file still passes.
func TestTheLadderNamesItsOwnRequirementBeforeTheVenuesGrid(t *testing.T) {
	t.Parallel()

	f := newDeleverer(t, neutralPackage(t)...)
	f.open("1")

	// Sized 1.01 against a generation that requires 1: missized AND off-step.
	both := f.add(perpLeg(t, 10, "1.01"))
	_, err := f.deleverer.SubmitPerpReduction(hedgeGroup(), ref(both))
	if !errors.Is(err, application.ErrGenerationLegMissized) {
		t.Fatalf("SubmitPerpReduction(missized and off-grid) = %v, want ErrGenerationLegMissized", err)
	}
	if errors.Is(err, application.ErrQuantityOffLotStep) {
		t.Errorf("the refusal also names the lot step; the grid check must not run first")
	}
}

// TestTheLadderRefusesALegItHasNoFiltersFor is the fail-closed direction, on the
// deployment shape NewDeleverer's doc names: the reconcile-only root wires no
// filters at all, which is a LEGAL construction meaning "plans nothing".
//
// A ladder that admitted an unknown instrument would send a safety leg against a
// grid nobody had ever read — the worst possible moment to discover a wiring
// omission, and the direction §2.4's absent-entry rule exists to refuse.
func TestTheLadderRefusesALegItHasNoFiltersFor(t *testing.T) {
	t.Parallel()

	// The PERP row alone, so the spot leg is the one with no entry. A map that
	// was merely empty would refuse both legs, and could not tell "consults the
	// dispatcher's map" from "refuses everything".
	perpOnly := map[ids.VenueInstrument]domain.TradingFilters{perp(): testFilters(t)[perp()]}
	f := newDelevererOn(t, testClock(t), perpOnly, testFilterMaxAge, neutralPackage(t)...)
	f.open("1")
	f.toSpotAuthorization("1", "1")

	leg := f.add(spotLeg(t, 11, "1"))
	if _, err := f.deleverer.SubmitSpotReduction(hedgeGroup(), ref(leg)); !errors.Is(err, application.ErrFiltersUnknown) {
		t.Fatalf("SubmitSpotReduction(no filters row) = %v, want ErrFiltersUnknown", err)
	}
	// The perp leg went out under the same map, which is what makes the refusal
	// above about the missing ENTRY rather than about the map being partial.
	if f.placer.calls != 1 {
		t.Errorf("placed %d orders, want 1 (the perp leg, whose row exists)", f.placer.calls)
	}
}
