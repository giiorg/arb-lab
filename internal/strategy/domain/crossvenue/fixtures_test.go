package crossvenue_test

// Shared fixture helpers. The base instant is arbitrary but off every
// venue grid on purpose: fixtures that need grid alignment say so
// explicitly, so an implementation that assumed a shared origin has
// nothing to lean on.

import (
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/strategy/domain/crossvenue"
)

// cvBase is 2026-08-05T00:00:00Z — inside the gate's evidence era, on no
// particular grid.
var cvBase = time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC)

func rate(t testing.TB, s string) fixedpoint.Rate {
	t.Helper()
	r, err := fixedpoint.ParseRate(s)
	if err != nil {
		t.Fatalf("ParseRate(%q): %v", s, err)
	}
	return r
}

func settle(t testing.TB, at time.Time, r string) crossvenue.Settlement {
	t.Helper()
	return crossvenue.Settlement{At: at, Rate: rate(t, r)}
}

func point(t testing.TB, at time.Time, sig string) crossvenue.DecisionPoint {
	t.Helper()
	return crossvenue.DecisionPoint{At: at, Signal: rate(t, sig)}
}

func mustDecimal(t testing.TB, r fixedpoint.Rate) string {
	t.Helper()
	if err := r.Validate(); err != nil {
		t.Fatalf("rate unset: %v", err)
	}
	return r.Decimal()
}

// wantRate fails unless got equals the decimal want exactly.
func wantRate(t testing.TB, what string, got fixedpoint.Rate, want string) {
	t.Helper()
	w := rate(t, want)
	c, err := got.Cmp(w)
	if err != nil {
		t.Fatalf("%s: %v", what, err)
	}
	if c != 0 {
		t.Fatalf("%s: got %s, want %s", what, got.Decimal(), want)
	}
}
