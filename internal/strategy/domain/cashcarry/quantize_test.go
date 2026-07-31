package cashcarry_test

import (
	"errors"
	"testing"

	"github.com/giiorg/arb-lab/internal/strategy/domain/cashcarry"
)

// TestTargetQtyIntersectionIsTheLCM uses steps whose LCM is NEITHER step —
// 0.003 and 0.005 intersect at 0.015 — so a mutant that quantizes to one
// leg's grid (or to the larger step) produces a different target. The
// checklist's shape: with the production 0.00001/0.001 pair the LCM equals
// the perp step and "use the coarser step" is indistinguishable from the
// rule.
func TestTargetQtyIntersectionIsTheLCM(t *testing.T) {
	in := flatEntry(t)
	in.SpotLots.Step = mustQty(t, "0.003")
	in.SpotLots.MinQty = mustQty(t, "0.003")
	in.PerpLots.Step = mustQty(t, "0.005")
	in.PerpLots.MinQty = mustQty(t, "0.005")
	in.CandidateQty = mustQty(t, "0.1")
	in.ExecCost.Qty = in.CandidateQty
	in.ResidualTolerance = mustQty(t, "0.01")

	got, err := cashcarry.TargetQty(in)
	if err != nil {
		t.Fatal(err)
	}
	// 0.1 floored onto the 0.015 grid is 0.09 — six lots. The perp grid
	// alone would answer 0.1 (twenty lots of 0.005), the spot grid 0.099.
	if got.Decimal() != "0.09" {
		t.Fatalf("target = %s, want 0.09", got.Decimal())
	}
}

func TestTargetQtyResidualBoundary(t *testing.T) {
	in := flatEntry(t)
	in.SpotLots.Step = mustQty(t, "0.003")
	in.SpotLots.MinQty = mustQty(t, "0.003")
	in.PerpLots.Step = mustQty(t, "0.005")
	in.PerpLots.MinQty = mustQty(t, "0.005")
	in.CandidateQty = mustQty(t, "0.1")

	// Residual is exactly 0.01: a tolerance of exactly that passes — the
	// rule is "beyond tolerance", not "at" — and one ulp less refuses.
	in.ResidualTolerance = mustQty(t, "0.01")
	if _, err := cashcarry.TargetQty(in); err != nil {
		t.Fatalf("residual at tolerance refused: %v", err)
	}
	in.ResidualTolerance = mustQty(t, "0.009999999")
	if _, err := cashcarry.TargetQty(in); !errors.Is(err, cashcarry.ErrResidualExceeded) {
		t.Fatalf("want ErrResidualExceeded, got %v", err)
	}
}

func TestTargetQtyMinQtyBoundary(t *testing.T) {
	in := flatEntry(t)
	// Target quantizes to exactly 0.1; a per-leg MinQty of exactly 0.1
	// passes, one ulp more refuses — checked per leg, so the perp's floor
	// alone must be able to refuse.
	in.PerpLots.MinQty = mustQty(t, "0.1")
	if _, err := cashcarry.TargetQty(in); err != nil {
		t.Fatalf("target at min qty refused: %v", err)
	}
	in.PerpLots.MinQty = mustQty(t, "0.100000001")
	if _, err := cashcarry.TargetQty(in); !errors.Is(err, cashcarry.ErrBelowMinQty) {
		t.Fatalf("want ErrBelowMinQty, got %v", err)
	}
}

// TestTargetQtyMinNotionalIsJudgedAtTheBid separates the bid from the ask:
// the target's bid notional sits one cent under the venue floor while its
// ask notional sits over, so an ask-priced mutant admits an order the venue
// can reject.
func TestTargetQtyMinNotionalIsJudgedAtTheBid(t *testing.T) {
	in := flatEntry(t)
	in.View.Perp.Bid = mustPrice(t, "99999.90")
	in.View.Perp.Ask = mustPrice(t, "100000.10")
	// Target 0.1: bid notional 9999.99, ask notional 10000.01.
	in.PerpLots.MinNotional = mustMoney(t, "10000")
	if _, err := cashcarry.TargetQty(in); !errors.Is(err, cashcarry.ErrBelowMinNotional) {
		t.Fatalf("want ErrBelowMinNotional, got %v", err)
	}
	// At the bid notional exactly, it passes.
	in.PerpLots.MinNotional = mustMoney(t, "9999.99")
	if _, err := cashcarry.TargetQty(in); err != nil {
		t.Fatalf("at-bid-notional refused: %v", err)
	}
}

func TestTargetQtyCandidateSmallerThanOneStep(t *testing.T) {
	in := flatEntry(t)
	in.CandidateQty = mustQty(t, "0.0004")
	in.ExecCost.Qty = in.CandidateQty
	in.ResidualTolerance = mustQty(t, "0.0004")
	// Quantized to zero on the 0.001 intersection grid; zero is under the
	// perp's MinQty, so the refusal names the floor rather than emitting a
	// zero-quantity order.
	if _, err := cashcarry.TargetQty(in); !errors.Is(err, cashcarry.ErrBelowMinQty) {
		t.Fatalf("want ErrBelowMinQty, got %v", err)
	}
}

// TestTargetQtyLCMOverflowRefuses drives two near-2^32 coprime step
// mantissas whose LCM exceeds int64 — no order quantity satisfies both
// grids, and the answer is a named refusal, not a wrapped integer.
func TestTargetQtyLCMOverflowRefuses(t *testing.T) {
	in := flatEntry(t)
	in.SpotLots.Step = mustQty(t, "4.294967291")
	in.PerpLots.Step = mustQty(t, "4.294967279")
	if _, err := cashcarry.TargetQty(in); !errors.Is(err, cashcarry.ErrLotIncompatible) {
		t.Fatalf("want ErrLotIncompatible, got %v", err)
	}
}
