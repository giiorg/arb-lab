package domain

import (
	"fmt"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// This file is the book-state stratum classifier (§8: "p90 slippage
// conditional on order size AND book state"; step-4 design §41b-ii-b).
//
// The classifier's CODE lives here in the domain, under the procedure
// commit; its PARAMETERS — the cut points, boundary inclusivity and rule
// order — arrive frozen inside the hashed calibration artifact. Calibration
// and runtime run the same classifier, which is what makes "the same book
// lands in the same stratum" a property of the artifact rather than a hope
// about two implementations: without frozen boundaries, two conforming
// readers of one approved artifact could put one book in two strata and
// draw different charges from the same governance.
//
// A book the classifier cannot place is a REFUSAL, never a nearest-cell
// guess — the out-of-range direction is the one that must be loud.

// BoundKind says how one end of a stratum interval is declared. It is
// explicit — never inferred from a zero value — because an interval end
// nobody populated must refuse, not silently mean "unbounded": unbounded is
// the permissive reading, and the forgotten field defaulting to permissive
// is the direction every gate in this repo exists to refuse.
type BoundKind string

const (
	// BoundUnbounded declares the end open: no constraint on this side.
	BoundUnbounded BoundKind = "unbounded"

	// BoundInclusive declares a closed end: the boundary value itself is
	// inside the interval.
	BoundInclusive BoundKind = "inclusive"

	// BoundExclusive declares an open end: the boundary value itself is
	// outside the interval.
	BoundExclusive BoundKind = "exclusive"
)

// Validate reports whether k is one of the declared kinds.
func (k BoundKind) Validate() error {
	switch k {
	case BoundUnbounded, BoundInclusive, BoundExclusive:
		return nil
	}
	return fmt.Errorf("bound kind %q: %w", string(k), ErrKindInvalid)
}

func (k BoundKind) String() string { return string(k) }

// RateBound is one end of a rate-valued interval. Value is required for a
// bounded end and must be ABSENT for an unbounded one — a value on an
// unbounded end is a contradiction the artifact meant something by, so it
// refuses rather than being ignored.
type RateBound struct {
	Kind  BoundKind
	Value fixedpoint.Rate
}

// Validate reports whether the bound is coherently declared.
func (b RateBound) Validate() error {
	if err := b.Kind.Validate(); err != nil {
		return err
	}
	if b.Kind == BoundUnbounded {
		if b.Value.Validate() == nil {
			return fmt.Errorf("unbounded end carries value %s: %w", b.Value, ErrStratumShape)
		}
		return nil
	}
	if err := b.Value.Validate(); err != nil {
		return fmt.Errorf("bound value: %w", err)
	}
	return nil
}

// QuantityBound is RateBound's shape for a quantity-valued interval.
type QuantityBound struct {
	Kind  BoundKind
	Value fixedpoint.Quantity
}

// Validate reports whether the bound is coherently declared.
func (b QuantityBound) Validate() error {
	if err := b.Kind.Validate(); err != nil {
		return err
	}
	if b.Kind == BoundUnbounded {
		if b.Value.Validate() == nil {
			return fmt.Errorf("unbounded end carries value %s: %w", b.Value, ErrStratumShape)
		}
		return nil
	}
	if err := b.Value.Validate(); err != nil {
		return fmt.Errorf("bound value: %w", err)
	}
	return nil
}

// RateInterval is a rate-valued interval with per-end declared inclusivity.
type RateInterval struct {
	Lo, Hi RateBound
}

// Validate reports whether the interval is declared and non-empty.
func (i RateInterval) Validate() error {
	if err := i.Lo.Validate(); err != nil {
		return fmt.Errorf("lo: %w", err)
	}
	if err := i.Hi.Validate(); err != nil {
		return fmt.Errorf("hi: %w", err)
	}
	return intervalShape(i.Lo.Kind, i.Hi.Kind, func() (int, error) { return i.Lo.Value.Cmp(i.Hi.Value) })
}

// Contains reports whether v lies inside the interval.
func (i RateInterval) Contains(v fixedpoint.Rate) (bool, error) {
	lo, err := boundAdmits(i.Lo.Kind, true, func() (int, error) { return v.Cmp(i.Lo.Value) })
	if err != nil || !lo {
		return false, err
	}
	return boundAdmits(i.Hi.Kind, false, func() (int, error) { return v.Cmp(i.Hi.Value) })
}

// QuantityInterval is RateInterval's shape for quantities.
type QuantityInterval struct {
	Lo, Hi QuantityBound
}

// Validate reports whether the interval is declared and non-empty.
func (i QuantityInterval) Validate() error {
	if err := i.Lo.Validate(); err != nil {
		return fmt.Errorf("lo: %w", err)
	}
	if err := i.Hi.Validate(); err != nil {
		return fmt.Errorf("hi: %w", err)
	}
	return intervalShape(i.Lo.Kind, i.Hi.Kind, func() (int, error) { return i.Lo.Value.Cmp(i.Hi.Value) })
}

// Contains reports whether v lies inside the interval.
func (i QuantityInterval) Contains(v fixedpoint.Quantity) (bool, error) {
	lo, err := boundAdmits(i.Lo.Kind, true, func() (int, error) { return v.Cmp(i.Lo.Value) })
	if err != nil || !lo {
		return false, err
	}
	return boundAdmits(i.Hi.Kind, false, func() (int, error) { return v.Cmp(i.Hi.Value) })
}

// intervalShape refuses an interval no value can satisfy: lo above hi, or
// lo equal to hi with either end exclusive. A dead rule in a classifier is
// a stratum the artifact NAMES but nothing can ever reach — the declared
// enum member nothing emits, refused at construction instead of discovered
// at scoring.
func intervalShape(lo, hi BoundKind, cmp func() (int, error)) error {
	if lo == BoundUnbounded || hi == BoundUnbounded {
		return nil
	}
	c, err := cmp()
	if err != nil {
		return err
	}
	if c > 0 {
		return fmt.Errorf("lo above hi: %w", ErrStratumShape)
	}
	if c == 0 && (lo == BoundExclusive || hi == BoundExclusive) {
		return fmt.Errorf("empty interval at shared boundary: %w", ErrStratumShape)
	}
	return nil
}

// boundAdmits evaluates one end. cmp compares the candidate against the
// bound's value; isLo says which side the bound guards.
func boundAdmits(kind BoundKind, isLo bool, cmp func() (int, error)) (bool, error) {
	switch kind {
	case BoundUnbounded:
		return true, nil
	case BoundInclusive, BoundExclusive:
		c, err := cmp()
		if err != nil {
			return false, err
		}
		if c == 0 {
			return kind == BoundInclusive, nil
		}
		if isLo {
			return c > 0, nil
		}
		return c < 0, nil
	}
	return false, fmt.Errorf("bound kind %q: %w", string(kind), ErrKindInvalid)
}

// BookFeatures is the classifier's input: the features the procedure
// measures on a book at one instant. The adapter that holds the book
// computes them (one implementation, shared by runtime pricing and the
// calibration derivation); the domain only judges where they land.
type BookFeatures struct {
	// Spread is the relative top-of-book spread, (ask − bid) / mid.
	Spread fixedpoint.Rate

	// Depth is the priced side's total displayed base quantity across the
	// snapshot's levels.
	Depth fixedpoint.Quantity
}

// Validate reports whether the features were actually measured.
func (f BookFeatures) Validate() error {
	if err := f.Spread.Validate(); err != nil {
		return fmt.Errorf("spread: %w", err)
	}
	if err := f.Depth.Validate(); err != nil {
		return fmt.Errorf("depth: %w", err)
	}
	return nil
}

// StratumRule is one frozen stratum: its name and the feature intervals
// that admit a book into it.
type StratumRule struct {
	// Name keys the p90 cells measured for this stratum.
	Name string

	// Spread and Depth are the admitting intervals. Both must hold.
	Spread RateInterval
	Depth  QuantityInterval
}

// Validate reports whether the rule is complete.
func (r StratumRule) Validate() error {
	if r.Name == "" {
		return fmt.Errorf("stratum name: %w", ErrUnset)
	}
	if err := r.Spread.Validate(); err != nil {
		return fmt.Errorf("stratum %s spread: %w", r.Name, err)
	}
	if err := r.Depth.Validate(); err != nil {
		return fmt.Errorf("stratum %s depth: %w", r.Name, err)
	}
	return nil
}

// StratumClassifier is the frozen classifier: ordered rules, first match
// wins. The ORDER is part of the frozen parameters — overlapping rules are
// legal precisely because precedence is declared, not discovered.
type StratumClassifier struct {
	Rules []StratumRule
}

// Validate reports whether the classifier can place anything.
func (c StratumClassifier) Validate() error {
	if len(c.Rules) == 0 {
		return fmt.Errorf("classifier rules: %w", ErrUnset)
	}
	seen := map[string]bool{}
	for n, r := range c.Rules {
		if err := r.Validate(); err != nil {
			return fmt.Errorf("rule %d: %w", n, err)
		}
		if seen[r.Name] {
			return fmt.Errorf("rule %d name %s: %w", n, r.Name, ErrStratumDuplicate)
		}
		seen[r.Name] = true
	}
	return nil
}

// Classify places the features in the first admitting stratum. No stratum
// admitting them is ErrStratumUnclassifiable — the artifact has no number
// for this book state, and the honest answer is a refusal naming that,
// never the nearest cell.
func (c StratumClassifier) Classify(f BookFeatures) (string, error) {
	if err := c.Validate(); err != nil {
		return "", err
	}
	if err := f.Validate(); err != nil {
		return "", fmt.Errorf("classify: %w", err)
	}
	for _, r := range c.Rules {
		inSpread, err := r.Spread.Contains(f.Spread)
		if err != nil {
			return "", fmt.Errorf("stratum %s spread: %w", r.Name, err)
		}
		if !inSpread {
			continue
		}
		inDepth, err := r.Depth.Contains(f.Depth)
		if err != nil {
			return "", fmt.Errorf("stratum %s depth: %w", r.Name, err)
		}
		if inDepth {
			return r.Name, nil
		}
	}
	return "", fmt.Errorf("spread %s depth %s: %w", f.Spread, f.Depth, ErrStratumUnclassifiable)
}
