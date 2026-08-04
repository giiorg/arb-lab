package domain

import (
	"fmt"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// TradingFilters is one instrument's published order constraints: the lot
// grid, the price grid, and the two floors the venue checks before it will
// hold an order. §2.4 calls the quantity half "the venue lot sizes"; the
// venues' own documents call the whole set trading filters, and the profiles
// document records them under that name.
//
// # Why the ORDER PATH carries these at all, when sizing already quantizes
//
// §6 step 4's sizing derives an on-grid quantity (strategy's TargetQty owns
// the rounding and the residual tolerance). This type exists for the
// verification half: an order that reaches a venue off-grid is not refused
// symmetrically — each leg is judged by its own venue against its own grid —
// so the failure §2.4 exists to prevent is one leg of a package filling while
// the other is rejected, which is naked exposure, not an error message. The
// planner therefore VERIFIES every leg against this type before anything is
// journaled, and a wrong quantity is a refusal that names the filter rather
// than a venue rejection that arrives after the sibling leg was sent.
//
// # It lives in domain/, not application/ports, and the placement is load-bearing
//
// §3.6 rule 6 (singlewriter) forbids every package outside this context —
// composition roots included — from importing oms/application/ports and
// oms/infrastructure/driven. A root must still be able to STATE these filters
// (it is the layer that resolves them from the reference revision), so the
// vocabulary sits in the one package the gate deliberately leaves nameable,
// exactly as the order-state enum does for projections (§5.1's pattern).
type TradingFilters struct {
	// Step is the lot increment: every order quantity is a whole multiple.
	// Binance LOT_SIZE stepSize; Bybit qtyStep (linear) / basePrecision (spot).
	Step fixedpoint.Quantity

	// MinQty is the smallest order the venue admits. A declared zero is legal
	// — a venue may publish one — and distinct from an unset field, which
	// fails Validate.
	MinQty fixedpoint.Quantity

	// Tick is the price increment: every limit price is a whole multiple.
	// Binance PRICE_FILTER tickSize; Bybit priceFilter.tickSize. Every planned
	// order here is a marketable-IOC LIMIT (§6 step 6), so there is no order
	// this system sends that a tick does not bind.
	Tick fixedpoint.Price

	// MinNotional is the venue's minimum order value, judged at the order's
	// own limit price — the price the venue's filter reads for a limit order.
	// A declared zero is legal for MinQty's reason.
	MinNotional fixedpoint.Money
}

// Validate reports whether the filters could verify anything.
//
// The two grids must be strictly positive: a zero step admits every quantity,
// which is the unpopulated field reading as "no constraint" — and the zero
// arrives from real data, not only from forgetting (Binance publishes
// stepSize "0.00000000" on filters that do not apply). The two floors must be
// present but may be declared zeros.
//
// The members must also agree about the instrument's assets. Each field
// carries its own tags, so a filters value assembled from two instruments'
// rows — or from a wrong base/quote split — is expressible, and every
// disagreement it could smuggle into the planner's checks is caught here
// instead, where the message can say which members disagree.
func (f TradingFilters) Validate() error {
	if err := f.Step.Validate(); err != nil {
		return fmt.Errorf("step: %w", err)
	}
	if sign, err := f.Step.Sign(); err != nil {
		return fmt.Errorf("step: %w", err)
	} else if sign <= 0 {
		return fmt.Errorf("step %s: %w", f.Step, ErrFilterStepNotPositive)
	}

	if err := f.MinQty.Validate(); err != nil {
		return fmt.Errorf("min qty: %w", err)
	}
	if sign, err := f.MinQty.Sign(); err != nil {
		return fmt.Errorf("min qty: %w", err)
	} else if sign < 0 {
		return fmt.Errorf("min qty %s: %w", f.MinQty, ErrFilterFloorNegative)
	}

	if err := f.Tick.Validate(); err != nil {
		return fmt.Errorf("tick: %w", err)
	}
	if f.Tick.Mantissa() <= 0 {
		return fmt.Errorf("tick %s: %w", f.Tick, ErrFilterTickNotPositive)
	}

	if err := f.MinNotional.Validate(); err != nil {
		return fmt.Errorf("min notional: %w", err)
	}
	if sign, err := f.MinNotional.Sign(); err != nil {
		return fmt.Errorf("min notional: %w", err)
	} else if sign < 0 {
		return fmt.Errorf("min notional %s: %w", f.MinNotional, ErrFilterFloorNegative)
	}

	if f.MinQty.Asset() != f.Step.Asset() {
		return fmt.Errorf("min qty in %s, step in %s: %w",
			f.MinQty.Asset(), f.Step.Asset(), ErrFilterAssetsDisagree)
	}
	if f.Tick.Base() != f.Step.Asset() {
		return fmt.Errorf("tick prices %s, step counts %s: %w",
			f.Tick.Base(), f.Step.Asset(), ErrFilterAssetsDisagree)
	}
	if f.MinNotional.Asset() != f.Tick.Quote() {
		return fmt.Errorf("min notional in %s, tick quoted in %s: %w",
			f.MinNotional.Asset(), f.Tick.Quote(), ErrFilterAssetsDisagree)
	}
	return nil
}
