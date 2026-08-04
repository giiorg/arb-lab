package application

import (
	"fmt"

	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// This file is §2.4's verification half, shared by the two use cases that
// enforce it: the Planner (per leg, before anything is journaled) and the
// Executor's preflight (per package, before any fsync or send). ONE
// implementation on purpose — two statements of "conforms to the venue's
// grid" is how a plan-time pass and an execute-time refusal come to disagree
// by one lot step, the drift TargetQty's doc warns about on the sizing side.
//
// The normative sources are §2.4's trading-filter conformance bullet (the
// four members, the two enforcement points, the no-repair rule) and §6 step
// 5's verification clause — landed v0.9.41, the session this file did:
// until then §2.4 mandated only the LOT intersection, and the tick and
// min-notional duties asserted here were two unmarked spec holes.
//
// # Why the Executor re-verifies what the Planner already refused
//
// Plan is the only way an order enters the system TODAY, but the repository
// it fills is also rebuilt by §7.5's replay — and a replayed StatePlanned
// record was conformed by whatever filters revision (or pre-filters build)
// planned it, not by this process's. §5.3's doctrine for such records is
// replan-or-discard, never dispatch; the preflight check is that doctrine
// made structural for the package path, and it is package-ATOMIC where the
// per-leg dispatchers could not be: a legacy off-grid leg refuses the whole
// package before any sibling has been fsynced or sent, which is the
// asymmetric-rejection harm this file exists to prevent, closed at the second
// place it could re-enter.
//
// # Verify, never derive
//
// The DERIVING quantizer — round down to the intersection grid, judge the
// residual against its tolerance — is §6 step 4's and lives with sizing.
// Rounding here instead of refusing would silently repair a disagreement
// between the sizing's idea of the grid and the venue's, which is precisely
// the disagreement worth surfacing; and a repaired quantity would no longer
// be the one the caller's own records describe.

// validateFilters refuses an unusable entry at construction, where the
// message can name the wiring — not at the first real order it would
// misjudge. A nil map is legal: it is the deployment that plans nothing.
func validateFilters(filters map[ids.VenueInstrument]domain.TradingFilters) error {
	for inst, f := range filters {
		if err := f.Validate(); err != nil {
			return fmt.Errorf("trading filters for %s: %w", inst, err)
		}
	}
	return nil
}

// conformsToFilters judges one leg against its instrument's published
// filters. An absent entry refuses — the fail-closed reading, stated on
// ErrFiltersUnknown.
//
// # The notional is judged at the leg's own limit price, rounded toward zero
//
// The venue's filter reads a limit order's price*qty product, so the limit
// price is the exact quantity the venue judges — not a conservative proxy for
// it, as the sizing path's bid-priced check is. RoundTowardZero understates a
// product whose exact value needs more than scale 9, so the disagreement
// window rounding opens is refuse-what-the-venue-accepts, never the reverse.
func conformsToFilters(filters map[ids.VenueInstrument]domain.TradingFilters,
	inst ids.VenueInstrument, quantity fixedpoint.Quantity, limit fixedpoint.Price) error {

	f, ok := filters[inst]
	if !ok {
		return fmt.Errorf("instrument %s: %w", inst, ErrFiltersUnknown)
	}

	if on, err := quantity.OnStep(f.Step); err != nil {
		return fmt.Errorf("lot step: %w", err)
	} else if !on {
		return fmt.Errorf("quantity %s, step %s: %w", quantity, f.Step, ErrQuantityOffLotStep)
	}
	if c, err := quantity.Cmp(f.MinQty); err != nil {
		return fmt.Errorf("min qty: %w", err)
	} else if c < 0 {
		return fmt.Errorf("quantity %s, min %s: %w", quantity, f.MinQty, ErrBelowMinQty)
	}

	if on, err := limit.OnTick(f.Tick); err != nil {
		return fmt.Errorf("tick: %w", err)
	} else if !on {
		return fmt.Errorf("limit price %s, tick %s: %w", limit, f.Tick, ErrPriceOffTick)
	}

	notional, err := fixedpoint.Notional(limit, quantity, fixedpoint.RoundTowardZero)
	if err != nil {
		return fmt.Errorf("notional: %w", err)
	}
	if c, err := notional.Cmp(f.MinNotional); err != nil {
		return fmt.Errorf("min notional: %w", err)
	} else if c < 0 {
		return fmt.Errorf("notional %s at limit %s, min %s: %w",
			notional, limit, f.MinNotional, ErrBelowMinNotional)
	}
	return nil
}
