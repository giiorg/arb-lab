package persistence

import (
	"fmt"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// The coordinates, as nesting types.
//
// The nesting IS record §103's claim, expressed as a signature rather than as
// a sentence: run length is a function of the entry level alone
// (summarise.py:58 — cost enters only wins and mean_net), break-even and net
// need a cost as well, and annualisation needs a size on top of that. You
// cannot pass a cost to RunLength because EntryAt has no field for one, and you
// cannot call PerSymbol without naming the leg notional that the field name
// annual_pct_on_2000 omits.
//
// The leg notional is the fifth coordinate, and it is in no field and no row of
// the artifact: summarise.py:96 pins CAP = 2000 and LEG = 1000, the field name
// carries the capital, and the per-leg notional scales the numerator with
// nothing recording it. That is why SizeAt carries two Money values.

// EntryAt names the entry level. It is what run length depends on, and nothing
// more.
type EntryAt struct {
	Entry fixedpoint.Rate
}

// Validate reports whether the coordinate was populated.
func (a EntryAt) Validate() error {
	if err := a.Entry.Validate(); err != nil {
		return fmt.Errorf("%w: entry level: %v", ErrCoordinateUnset, err)
	}
	return nil
}

// CostAt adds the round-trip cost, which break-even and net depend on.
type CostAt struct {
	EntryAt
	Cost fixedpoint.Rate
}

// Validate reports whether the coordinate and everything it embeds was
// populated.
func (a CostAt) Validate() error {
	if err := a.EntryAt.Validate(); err != nil {
		return err
	}
	if err := a.Cost.Validate(); err != nil {
		return fmt.Errorf("%w: cost: %v", ErrCoordinateUnset, err)
	}
	return nil
}

// SizeAt adds the two notionals. LegNotional scales the numerator; Capital is
// the denominator the annualised return is expressed against.
type SizeAt struct {
	CostAt
	LegNotional fixedpoint.Money
	Capital     fixedpoint.Money
}

// Validate reports whether the coordinate and everything it embeds was
// populated.
func (a SizeAt) Validate() error {
	if err := a.CostAt.Validate(); err != nil {
		return err
	}
	if err := a.LegNotional.Validate(); err != nil {
		return fmt.Errorf("%w: leg notional: %v", ErrCoordinateUnset, err)
	}
	if err := a.Capital.Validate(); err != nil {
		return fmt.Errorf("%w: capital: %v", ErrCoordinateUnset, err)
	}
	return nil
}

// Hurdle is the benchmark a verdict compares against. It is an argument and
// never a constant.
//
// F45 is the reason. summarise.py's HURDLE_PCT is an illustrative constant: it
// is NOT reference data (record §100 §8c defines that as configs/reference.*.json
// specifically, which carry 0.0369 rather than 0.046), and it is not a
// calibrated benchmark per build-spec §55, whose input is dated and
// haircut-adjusted. A Go constant here would re-commit F45's defect in a second
// language, so Class and Source are required fields: a hurdle that cannot say
// what kind of number it is cannot be rendered honestly beside a count computed
// from it.
//
// Annual is a RATIO, not a percentage: 4.6% is 0.046.
type Hurdle struct {
	Annual fixedpoint.Rate
	Class  string
	Source string
}

// Validate reports whether the hurdle carries its value AND its provenance.
func (h Hurdle) Validate() error {
	if err := h.Annual.Validate(); err != nil {
		return fmt.Errorf("%w: hurdle rate: %v", ErrCoordinateUnset, err)
	}
	if h.Class == "" {
		return fmt.Errorf("%w: hurdle class", ErrCoordinateUnset)
	}
	if h.Source == "" {
		return fmt.Errorf("%w: hurdle source", ErrCoordinateUnset)
	}
	return nil
}
