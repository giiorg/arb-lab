package application

import (
	"fmt"
	"time"

	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// This file is §2.4's verification half, shared by the three use cases that
// enforce it: the Planner (per leg, before anything is journaled), the
// Executor's preflight (per package, before any fsync or send) and the
// Deleverer's verifyLeg (per ladder leg, same). ONE implementation on purpose
// — two statements of "conforms to the venue's grid" is how a plan-time pass
// and an execute-time refusal come to disagree by one lot step, the drift
// TargetQty's doc warns about on the sizing side.
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

// filterPolicy is the venue's published grid and the deployment's row-age
// tolerance held as ONE value, because §69 makes them one policy: a grid you
// cannot date is a claim about a row nobody has vouched for, and a tolerance
// over no rows is a root declaring a bound for something it never supplied.
// Holding them in one type is what stops a caller from enforcing half of it.
//
// # It is enforced at the caller's ADMISSION UNIT, never inside Dispatch
//
// The unit is the set of legs that must be refused together, and it differs by
// caller: the Executor's is the whole package (§62d — one instant for every
// leg, or a two-leg package can straddle the age bound and refuse
// asymmetrically after a sibling was sent), the Deleverer's is the single leg
// (SM §7 sequences its two deliberately — the spot cap is unknown until the
// perp confirms, so there is no shared instant to stamp). Dispatch cannot know
// which it is being called under, so a check placed there would read its own
// clock per leg inside the executor's fan-out — which is §62d re-opened in the
// one method that cannot see the harm. See Dispatcher.filters, which holds this
// value without reading it, and says why.
type filterPolicy struct {
	rows   map[ids.VenueInstrument]domain.TradingFilters
	maxAge time.Duration
}

// newFilterPolicy refuses a policy no leg could be judged against, at
// construction, where the message names the root.
//
// The half-a-policy refusals are §69's and were the Executor's until the
// policy got a type: a non-empty map with a non-positive bound is a grid
// nothing dates, and a bound over an empty map is a tolerance for rows that do
// not exist. Both are wiring, and the empty/zero pair is legal — it is the
// reconcile-only deployment, which conforms nothing because it plans nothing
// and whose every leg is refused by ErrFiltersUnknown rather than admitted.
func newFilterPolicy(rows map[ids.VenueInstrument]domain.TradingFilters, maxAge time.Duration) (filterPolicy, error) {
	if err := validateFilters(rows); err != nil {
		return filterPolicy{}, err
	}
	if len(rows) > 0 && maxAge <= 0 {
		return filterPolicy{}, fmt.Errorf("filter max age %s with %d filter rows: %w",
			maxAge, len(rows), ErrFilterMaxAgeInvalid)
	}
	if len(rows) == 0 && maxAge != 0 {
		return filterPolicy{}, fmt.Errorf("filter max age %s with no filter rows: %w",
			maxAge, ErrFilterMaxAgeInvalid)
	}
	return filterPolicy{rows: rows, maxAge: maxAge}, nil
}

// admits judges one leg against the grid AND the grid's date, at the instant
// the caller chose. It returns the bare finding; the caller wraps it with the
// context only the caller has (which leg of which package, which rung of which
// ladder).
//
// The two halves run in this order and are not separable by a caller: a leg
// that conforms to a row the venue may have moved past has satisfied a claim
// about the ROW, and §69's whole point is that the row's date is the other half
// of that claim. Splitting them across two layers is how one gets enforced and
// the other forgotten — which is exactly what #184 found on the ladder's path,
// where neither ran.
func (p filterPolicy) admits(inst ids.VenueInstrument, quantity fixedpoint.Quantity,
	limit fixedpoint.Price, now time.Time) error {

	if err := conformsToFilters(p.rows, inst, quantity, limit); err != nil {
		return err
	}

	// The entry is known present because conformsToFilters just consulted it.
	//
	// A FUTURE row refuses first, with its own sentinel: a negative age passes
	// the staleness comparison trivially, and it has a real producer — a
	// backward wall-clock step after boot, exactly the event platform/clock's
	// clamp exists for, met here on a map resolved before the step. The
	// operator's fix differs too: a stale row wants a fresh revision, a future
	// row wants the clock investigated.
	f := p.rows[inst]
	if f.ObservedAt.After(now) {
		return fmt.Errorf("filters for %s observed %s, after the admission instant %s: %w",
			inst, f.ObservedAt.Format(time.RFC3339), now.Format(time.RFC3339), ErrFiltersFromTheFuture)
	}
	// The bound is inclusive — a row aged exactly the tolerance still admits.
	if age := now.Sub(f.ObservedAt); age > p.maxAge {
		return fmt.Errorf("filters for %s observed %s, %s ago against a %s bound: %w",
			inst, f.ObservedAt.Format(time.RFC3339), age.Round(time.Second), p.maxAge,
			ErrFiltersStaleAtAdmission)
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
