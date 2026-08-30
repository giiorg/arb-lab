package persistence_test

import (
	"errors"
	"testing"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/strategy/infrastructure/persistence"
)

func mustRate(t *testing.T, s string) fixedpoint.Rate {
	t.Helper()
	r, err := fixedpoint.ParseRate(s)
	if err != nil {
		t.Fatalf("ParseRate(%q): %v", s, err)
	}
	return r
}

func mustMoney(t *testing.T, s string) fixedpoint.Money {
	t.Helper()
	m, err := fixedpoint.ParseMoney("USDT", s)
	if err != nil {
		t.Fatalf("ParseMoney(%q): %v", s, err)
	}
	return m
}

// The zero value of every coordinate is refused. An unset entry level would
// otherwise read as 0.0002 rounded down to nothing, and an unset leg notional
// would read as a zero-size position whose annualised return is zero — both
// answer the research question in our favour with nothing failing.
func TestCoordinates_ZeroValueIsRefused(t *testing.T) {
	cases := map[string]interface{ Validate() error }{
		"EntryAt": persistence.EntryAt{},
		"CostAt":  persistence.CostAt{},
		"SizeAt":  persistence.SizeAt{},
		"Hurdle":  persistence.Hurdle{},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			err := c.Validate()
			if !errors.Is(err, persistence.ErrCoordinateUnset) {
				t.Fatalf("Validate() = %v, want ErrCoordinateUnset", err)
			}
		})
	}
}

// Each coordinate validates EVERY field it carries, including the ones it
// inherits by embedding. A CostAt whose Cost is set but whose embedded Entry is
// not is the realistic accident: the caller filled in the field the method name
// mentions and forgot the one the type nests.
func TestCoordinates_EachFieldIsChecked(t *testing.T) {
	entry := mustRate(t, "0.001")
	cost := mustRate(t, "0.0021")
	leg := mustMoney(t, "1000")
	cap2000 := mustMoney(t, "2000")

	t.Run("CostAt missing the embedded entry", func(t *testing.T) {
		c := persistence.CostAt{Cost: cost}
		if err := c.Validate(); !errors.Is(err, persistence.ErrCoordinateUnset) {
			t.Fatalf("Validate() = %v, want ErrCoordinateUnset", err)
		}
	})
	t.Run("CostAt missing its own cost", func(t *testing.T) {
		c := persistence.CostAt{EntryAt: persistence.EntryAt{Entry: entry}}
		if err := c.Validate(); !errors.Is(err, persistence.ErrCoordinateUnset) {
			t.Fatalf("Validate() = %v, want ErrCoordinateUnset", err)
		}
	})
	t.Run("SizeAt missing the capital", func(t *testing.T) {
		s := persistence.SizeAt{
			CostAt:      persistence.CostAt{EntryAt: persistence.EntryAt{Entry: entry}, Cost: cost},
			LegNotional: leg,
		}
		if err := s.Validate(); !errors.Is(err, persistence.ErrCoordinateUnset) {
			t.Fatalf("Validate() = %v, want ErrCoordinateUnset", err)
		}
	})
	t.Run("SizeAt missing the leg notional", func(t *testing.T) {
		s := persistence.SizeAt{
			CostAt:  persistence.CostAt{EntryAt: persistence.EntryAt{Entry: entry}, Cost: cost},
			Capital: cap2000,
		}
		if err := s.Validate(); !errors.Is(err, persistence.ErrCoordinateUnset) {
			t.Fatalf("Validate() = %v, want ErrCoordinateUnset", err)
		}
	})
	t.Run("Hurdle missing its class and source", func(t *testing.T) {
		h := persistence.Hurdle{Annual: mustRate(t, "0.046")}
		if err := h.Validate(); !errors.Is(err, persistence.ErrCoordinateUnset) {
			t.Fatalf("Validate() = %v, want ErrCoordinateUnset", err)
		}
	})
	t.Run("fully populated coordinates pass", func(t *testing.T) {
		// The positive control. Without it, a Validate() that returned an error
		// unconditionally would score a perfect five above.
		s := persistence.SizeAt{
			CostAt:      persistence.CostAt{EntryAt: persistence.EntryAt{Entry: entry}, Cost: cost},
			LegNotional: leg,
			Capital:     cap2000,
		}
		if err := s.Validate(); err != nil {
			t.Fatalf("SizeAt.Validate() = %v, want nil", err)
		}
		h := persistence.Hurdle{
			Annual: mustRate(t, "0.046"),
			Class:  "illustrative_constant",
			Source: "research/persistence/summarise.py HURDLE_PCT",
		}
		if err := h.Validate(); err != nil {
			t.Fatalf("Hurdle.Validate() = %v, want nil", err)
		}
	})
}
