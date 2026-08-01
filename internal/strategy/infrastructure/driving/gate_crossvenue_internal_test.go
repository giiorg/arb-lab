package driving

// The gate's utilization arithmetic, from inside the package — the same
// seam ledger_internal_test.go and ewma_internal_test.go use, for the same
// reason: the claim is about an unexported step, and exporting it just to
// test it would widen the package's surface for the battery's convenience.

import (
	"errors"
	"testing"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	dcv "github.com/giiorg/arb-lab/internal/strategy/domain/crossvenue"
)

// TestAnnualizedMeanCountsIdlePeriods is the utilization honesty in one
// assertion. Four periods, one of which earned 0.01 and three of which were
// idle: the mean is 0.0025, NOT 0.01, and annualizing gives
// 0.0025 x 2190 = 5.475.
//
// The fixture is deliberately mostly zeros, and that is the whole point. On
// a series with no idle periods this test would pass against an
// implementation that divided by the number of TRADING periods — which
// would report a strategy in position a quarter of the time as though it
// were always deployed, quadrupling the headline the hurdle is compared
// against. §8 asks for a utilization-adjusted figure; this division is
// where the adjustment happens, and it happens by counting the zeros.
func TestAnnualizedMeanCountsIdlePeriods(t *testing.T) {
	t.Parallel()
	zero := fixedpoint.ZeroRate()
	earn, err := fixedpoint.ParseRate("0.01")
	if err != nil {
		t.Fatalf("ParseRate: %v", err)
	}

	// 8760h / 4h = 2190 periods per year, derived here rather than read
	// back from the code under test.
	const periodsPerYear = 2190

	got, err := annualizedMean([]fixedpoint.Rate{zero, earn, zero, zero})
	if err != nil {
		t.Fatalf("annualizedMean: %v", err)
	}
	if got.Decimal() != "5.475" {
		t.Fatalf("annualized mean %s, want 5.475 (0.01 over 4 periods, x%d)",
			got.Decimal(), periodsPerYear)
	}

	// The same earnings spread over eight periods must HALVE the figure.
	// This is the claim that the divisor is the period count rather than a
	// constant: the two cases differ only in how many idle periods sit
	// beside the identical earning one.
	got8, err := annualizedMean([]fixedpoint.Rate{zero, earn, zero, zero, zero, zero, zero, zero})
	if err != nil {
		t.Fatalf("annualizedMean: %v", err)
	}
	if got8.Decimal() != "2.7375" {
		t.Fatalf("over eight periods: %s, want 2.7375 — half the four-period figure", got8.Decimal())
	}

	// A wholly idle series is an exact zero, not a refusal: standing down
	// for the entire window is a real measurement and its return is zero.
	idle, err := annualizedMean([]fixedpoint.Rate{zero, zero, zero})
	if err != nil {
		t.Fatalf("annualizedMean over an idle series: %v", err)
	}
	if s, err := idle.Sign(); err != nil || s != 0 {
		t.Fatalf("an idle series annualized to %s (sign %d, err %v), want exactly 0",
			idle.Decimal(), s, err)
	}

	// An EMPTY series is a refusal. It is a different thing from an idle
	// one — no periods means nothing was measured, and a zero there would
	// be a fabricated observation rather than an observed zero.
	if _, err := annualizedMean(nil); !errors.Is(err, dcv.ErrNoReturns) {
		t.Fatalf("an empty series gave %v, want ErrNoReturns", err)
	}
}
