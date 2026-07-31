package domain_test

// The stratum-classifier battery. The fixtures answer §41i's named suspects
// by construction: candidates sit exactly AT bounds (strictly-inside
// fixtures collapse inclusive into exclusive), the rules OVERLAP where order
// changes the answer (disjoint rules collapse first-match into any-match),
// and one fixture's spread admits while its depth refuses with a LATER rule
// admitting both (otherwise "first spread match wins" and "first full match
// wins" are the same predicate). Every Rate-side case has its Quantity-side
// mirror — the two interval types are duplicated logic, and a fix applied in
// one place gets applied to its mirror.

import (
	"errors"
	"testing"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/strategy/domain"
)

func rate(t *testing.T, s string) fixedpoint.Rate {
	t.Helper()
	r, err := fixedpoint.ParseRate(s)
	if err != nil {
		t.Fatalf("ParseRate(%q): %v", s, err)
	}
	return r
}

func qty(t *testing.T, s string) fixedpoint.Quantity {
	t.Helper()
	q, err := fixedpoint.ParseQuantity("BTC", s)
	if err != nil {
		t.Fatalf("ParseQuantity(%q): %v", s, err)
	}
	return q
}

func rb(t *testing.T, kind domain.BoundKind, v string) domain.RateBound {
	t.Helper()
	if kind == domain.BoundUnbounded {
		return domain.RateBound{Kind: kind}
	}
	return domain.RateBound{Kind: kind, Value: rate(t, v)}
}

func qb(t *testing.T, kind domain.BoundKind, v string) domain.QuantityBound {
	t.Helper()
	if kind == domain.BoundUnbounded {
		return domain.QuantityBound{Kind: kind}
	}
	return domain.QuantityBound{Kind: kind, Value: qty(t, v)}
}

func feats(t *testing.T, spread, depth string) domain.BookFeatures {
	t.Helper()
	return domain.BookFeatures{Spread: rate(t, spread), Depth: qty(t, depth)}
}

// fixtureClassifier: three strata whose boundaries deliberately make
// inclusivity observable and whose order carries information.
//
//	calm:     spread [0, 0.0002]        depth [10, unbounded)
//	stressed: spread (0.0002, 0.002]    depth (1, unbounded)
//	thin:     spread [0, 0.002]         depth (0, 1]     — OVERLAPS calm/stressed
//	          on spread; only depth separates it, and it comes LAST, so a
//	          classifier that stops at the first spread match answers wrong.
func fixtureClassifier(t *testing.T) domain.StratumClassifier {
	t.Helper()
	return domain.StratumClassifier{Rules: []domain.StratumRule{
		{
			Name:   "calm",
			Spread: domain.RateInterval{Lo: rb(t, domain.BoundInclusive, "0"), Hi: rb(t, domain.BoundInclusive, "0.0002")},
			Depth:  domain.QuantityInterval{Lo: qb(t, domain.BoundInclusive, "10"), Hi: qb(t, domain.BoundUnbounded, "")},
		},
		{
			Name:   "stressed",
			Spread: domain.RateInterval{Lo: rb(t, domain.BoundExclusive, "0.0002"), Hi: rb(t, domain.BoundInclusive, "0.002")},
			Depth:  domain.QuantityInterval{Lo: qb(t, domain.BoundExclusive, "1"), Hi: qb(t, domain.BoundUnbounded, "")},
		},
		{
			Name:   "thin",
			Spread: domain.RateInterval{Lo: rb(t, domain.BoundInclusive, "0"), Hi: rb(t, domain.BoundInclusive, "0.002")},
			Depth:  domain.QuantityInterval{Lo: qb(t, domain.BoundExclusive, "0"), Hi: qb(t, domain.BoundInclusive, "1")},
		},
	}}
}

func TestClassifyPlacesAndRefuses(t *testing.T) {
	c := fixtureClassifier(t)
	cases := []struct {
		name          string
		spread, depth string
		want          string
	}{
		// Exactly AT calm's spread hi (inclusive): calm, not stressed.
		{"at calm spread hi", "0.0002", "50", "calm"},
		// One step past it: stressed's exclusive lo admits, calm's hi does not.
		{"past calm spread hi", "0.000200001", "50", "stressed"},
		// Exactly AT calm's depth lo (inclusive).
		{"at calm depth lo", "0.0001", "10", "calm"},
		// Spread admits calm, depth (below 10) does not — falls through to
		// THIN, the later rule: first-full-match, not first-spread-match.
		{"calm spread, thin depth", "0.0001", "0.5", "thin"},
		// Exactly AT stressed's depth lo (exclusive): NOT stressed; thin's
		// hi is inclusive at 1, and thin's spread span admits.
		{"at stressed depth lo", "0.001", "1", "thin"},
		// Inside stressed's span on both features.
		{"stressed both features", "0.001", "2", "stressed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := c.Classify(feats(t, tc.spread, tc.depth))
			if err != nil {
				t.Fatalf("Classify: %v", err)
			}
			if got != tc.want {
				t.Fatalf("Classify = %s, want %s", got, tc.want)
			}
		})
	}
}

// TestClassifyOrderIsPrecedence pins first-match-wins directly: a classifier
// whose rules BOTH admit the same features answers by position, and
// reversing the order flips the answer — the mutant that scans backward or
// picks "best fit" fails here, nowhere else.
func TestClassifyOrderIsPrecedence(t *testing.T) {
	wide := domain.StratumRule{
		Name:   "wide",
		Spread: domain.RateInterval{Lo: rb(t, domain.BoundInclusive, "0"), Hi: rb(t, domain.BoundInclusive, "1")},
		Depth:  domain.QuantityInterval{Lo: qb(t, domain.BoundExclusive, "0"), Hi: qb(t, domain.BoundUnbounded, "")},
	}
	narrow := domain.StratumRule{
		Name:   "narrow",
		Spread: domain.RateInterval{Lo: rb(t, domain.BoundInclusive, "0"), Hi: rb(t, domain.BoundInclusive, "0.001")},
		Depth:  domain.QuantityInterval{Lo: qb(t, domain.BoundExclusive, "0"), Hi: qb(t, domain.BoundUnbounded, "")},
	}
	f := feats(t, "0.0005", "3")

	got, err := domain.StratumClassifier{Rules: []domain.StratumRule{wide, narrow}}.Classify(f)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if got != "wide" {
		t.Fatalf("Classify = %s, want wide (declared first)", got)
	}
	got, err = domain.StratumClassifier{Rules: []domain.StratumRule{narrow, wide}}.Classify(f)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if got != "narrow" {
		t.Fatalf("Classify = %s, want narrow (declared first)", got)
	}
}

func TestClassifyUnclassifiableRefusesByName(t *testing.T) {
	c := fixtureClassifier(t)
	// Spread past every rule's hi.
	if _, err := c.Classify(feats(t, "0.5", "50")); !errors.Is(err, domain.ErrStratumUnclassifiable) {
		t.Fatalf("spread out of range: %v, want ErrStratumUnclassifiable", err)
	}
	// Spread in range, depth zero — thin's lo is exclusive at 0.
	if _, err := c.Classify(feats(t, "0.0001", "0")); !errors.Is(err, domain.ErrStratumUnclassifiable) {
		t.Fatalf("depth out of range: %v, want ErrStratumUnclassifiable", err)
	}
}

func TestClassifyRefusesUnmeasuredFeatures(t *testing.T) {
	c := fixtureClassifier(t)
	if _, err := c.Classify(domain.BookFeatures{Depth: qty(t, "1")}); err == nil {
		t.Fatal("unset spread classified")
	}
	if _, err := c.Classify(domain.BookFeatures{Spread: rate(t, "0.0001")}); err == nil {
		t.Fatal("unset depth classified")
	}
}

func TestClassifierValidateRefusals(t *testing.T) {
	valid := fixtureClassifier(t)
	if err := valid.Validate(); err != nil {
		t.Fatalf("positive control: %v", err)
	}

	t.Run("empty rules", func(t *testing.T) {
		err := domain.StratumClassifier{}.Validate()
		if !errors.Is(err, domain.ErrUnset) {
			t.Fatalf("Validate: %v, want ErrUnset", err)
		}
	})
	t.Run("duplicate name", func(t *testing.T) {
		c := fixtureClassifier(t)
		c.Rules = append(c.Rules, c.Rules[0])
		if err := c.Validate(); !errors.Is(err, domain.ErrStratumDuplicate) {
			t.Fatalf("Validate: %v, want ErrStratumDuplicate", err)
		}
	})
	t.Run("unnamed rule", func(t *testing.T) {
		c := fixtureClassifier(t)
		c.Rules[0].Name = ""
		if err := c.Validate(); !errors.Is(err, domain.ErrUnset) {
			t.Fatalf("Validate: %v, want ErrUnset", err)
		}
	})
}

// The interval-shape refusals, Rate side and Quantity mirror side by side.
func TestIntervalShapeRefusals(t *testing.T) {
	t.Run("rate lo above hi", func(t *testing.T) {
		i := domain.RateInterval{Lo: rb(t, domain.BoundInclusive, "0.002"), Hi: rb(t, domain.BoundInclusive, "0.001")}
		if err := i.Validate(); !errors.Is(err, domain.ErrStratumShape) {
			t.Fatalf("Validate: %v, want ErrStratumShape", err)
		}
	})
	t.Run("quantity lo above hi", func(t *testing.T) {
		i := domain.QuantityInterval{Lo: qb(t, domain.BoundInclusive, "2"), Hi: qb(t, domain.BoundInclusive, "1")}
		if err := i.Validate(); !errors.Is(err, domain.ErrStratumShape) {
			t.Fatalf("Validate: %v, want ErrStratumShape", err)
		}
	})
	t.Run("rate empty at shared boundary", func(t *testing.T) {
		i := domain.RateInterval{Lo: rb(t, domain.BoundExclusive, "0.001"), Hi: rb(t, domain.BoundInclusive, "0.001")}
		if err := i.Validate(); !errors.Is(err, domain.ErrStratumShape) {
			t.Fatalf("Validate: %v, want ErrStratumShape", err)
		}
	})
	t.Run("quantity empty at shared boundary", func(t *testing.T) {
		i := domain.QuantityInterval{Lo: qb(t, domain.BoundInclusive, "1"), Hi: qb(t, domain.BoundExclusive, "1")}
		if err := i.Validate(); !errors.Is(err, domain.ErrStratumShape) {
			t.Fatalf("Validate: %v, want ErrStratumShape", err)
		}
	})
	t.Run("rate point interval is legal", func(t *testing.T) {
		i := domain.RateInterval{Lo: rb(t, domain.BoundInclusive, "0.001"), Hi: rb(t, domain.BoundInclusive, "0.001")}
		if err := i.Validate(); err != nil {
			t.Fatalf("point interval: %v", err)
		}
		in, err := i.Contains(rate(t, "0.001"))
		if err != nil || !in {
			t.Fatalf("point interval Contains its point: %v %v", in, err)
		}
	})
	t.Run("quantity point interval is legal", func(t *testing.T) {
		i := domain.QuantityInterval{Lo: qb(t, domain.BoundInclusive, "1"), Hi: qb(t, domain.BoundInclusive, "1")}
		if err := i.Validate(); err != nil {
			t.Fatalf("point interval: %v", err)
		}
		in, err := i.Contains(qty(t, "1"))
		if err != nil || !in {
			t.Fatalf("point interval Contains its point: %v %v", in, err)
		}
	})
}

// The bound-declaration refusals, both mirrors.
func TestBoundDeclarationRefusals(t *testing.T) {
	t.Run("rate unbounded with value", func(t *testing.T) {
		b := domain.RateBound{Kind: domain.BoundUnbounded, Value: rate(t, "1")}
		if err := b.Validate(); !errors.Is(err, domain.ErrStratumShape) {
			t.Fatalf("Validate: %v, want ErrStratumShape", err)
		}
	})
	t.Run("quantity unbounded with value", func(t *testing.T) {
		b := domain.QuantityBound{Kind: domain.BoundUnbounded, Value: qty(t, "1")}
		if err := b.Validate(); !errors.Is(err, domain.ErrStratumShape) {
			t.Fatalf("Validate: %v, want ErrStratumShape", err)
		}
	})
	t.Run("rate bounded without value", func(t *testing.T) {
		b := domain.RateBound{Kind: domain.BoundInclusive}
		if err := b.Validate(); err == nil {
			t.Fatal("bounded end with no value validated")
		}
	})
	t.Run("quantity bounded without value", func(t *testing.T) {
		b := domain.QuantityBound{Kind: domain.BoundExclusive}
		if err := b.Validate(); err == nil {
			t.Fatal("bounded end with no value validated")
		}
	})
	t.Run("forgotten kind refuses", func(t *testing.T) {
		if err := (domain.RateBound{Value: rate(t, "1")}).Validate(); !errors.Is(err, domain.ErrKindInvalid) {
			t.Fatalf("rate zero kind: %v, want ErrKindInvalid", err)
		}
		if err := (domain.QuantityBound{Value: qty(t, "1")}).Validate(); !errors.Is(err, domain.ErrKindInvalid) {
			t.Fatalf("quantity zero kind: %v, want ErrKindInvalid", err)
		}
	})
}
