package crossvenue_test

// Scoring fixtures, checklist-first: the portfolio weights are 0.75/0.25,
// never 0.5/0.5 — equal weights collapse "weighted combination" into
// "plain average" and every wrong weighting passes. The policy-sum
// refusal fixture is off by exactly ONE mantissa unit, the tightest
// wrongness the arithmetic can express.

import (
	"errors"
	"testing"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/strategy/domain/crossvenue"
)

func policy(t *testing.T, pairs ...string) []crossvenue.Allocation {
	t.Helper()
	if len(pairs)%2 != 0 {
		t.Fatalf("policy wants symbol/weight pairs, got %d items", len(pairs))
	}
	var out []crossvenue.Allocation
	for i := 0; i < len(pairs); i += 2 {
		out = append(out, crossvenue.Allocation{Symbol: pairs[i], Weight: rate(t, pairs[i+1])})
	}
	return out
}

func series(t *testing.T, decs ...string) []fixedpoint.Rate {
	t.Helper()
	out := make([]fixedpoint.Rate, len(decs))
	for i, d := range decs {
		out[i] = rate(t, d)
	}
	return out
}

func TestPortfolioReturns_WeightsAreWeights(t *testing.T) {
	// 0.75×0.0004 + 0.25×0.0008 = 0.0005 per period 0; a plain average
	// would say 0.0006.
	got, err := crossvenue.PortfolioReturns(
		policy(t, "LABUSDT", "0.75", "MUSDT", "0.25"),
		map[string][]fixedpoint.Rate{
			"LABUSDT": series(t, "0.0004", "-0.0002"),
			"MUSDT":   series(t, "0.0008", "0"),
		},
	)
	if err != nil {
		t.Fatalf("PortfolioReturns: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("periods %d, want 2", len(got))
	}
	wantRate(t, "period 0", got[0], "0.0005")
	wantRate(t, "period 1", got[1], "-0.00015")
}

func TestPortfolioReturns_PolicyRefusals(t *testing.T) {
	okSeries := map[string][]fixedpoint.Rate{
		"LABUSDT": series(t, "0.0001"),
		"MUSDT":   series(t, "0.0001"),
	}

	if _, err := crossvenue.PortfolioReturns(nil, okSeries); !errors.Is(err, crossvenue.ErrPolicyInvalid) {
		t.Fatalf("empty policy: %v, want ErrPolicyInvalid", err)
	}

	// Sum one mantissa unit under 1.
	under := policy(t, "LABUSDT", "0.75", "MUSDT", "0.249999999")
	if _, err := crossvenue.PortfolioReturns(under, okSeries); !errors.Is(err, crossvenue.ErrPolicyInvalid) {
		t.Fatalf("sum 1 ulp under: %v, want ErrPolicyInvalid", err)
	}
	over := policy(t, "LABUSDT", "0.75", "MUSDT", "0.250000001")
	if _, err := crossvenue.PortfolioReturns(over, okSeries); !errors.Is(err, crossvenue.ErrPolicyInvalid) {
		t.Fatalf("sum 1 ulp over: %v, want ErrPolicyInvalid", err)
	}

	dup := policy(t, "LABUSDT", "0.5", "LABUSDT", "0.5")
	if _, err := crossvenue.PortfolioReturns(dup, okSeries); !errors.Is(err, crossvenue.ErrPolicyInvalid) {
		t.Fatalf("repeated symbol: %v, want ErrPolicyInvalid", err)
	}

	zeroW := policy(t, "LABUSDT", "1", "MUSDT", "0")
	if _, err := crossvenue.PortfolioReturns(zeroW, okSeries); !errors.Is(err, crossvenue.ErrPolicyInvalid) {
		t.Fatalf("zero weight: %v, want ErrPolicyInvalid — an unheld symbol is omitted, not zeroed", err)
	}

	unnamed := []crossvenue.Allocation{{Weight: rate(t, "1")}}
	if _, err := crossvenue.PortfolioReturns(unnamed, okSeries); !errors.Is(err, crossvenue.ErrPolicyInvalid) {
		t.Fatalf("unnamed holding: %v, want ErrPolicyInvalid", err)
	}
}

func TestPortfolioReturns_SeriesRefusals(t *testing.T) {
	pol := policy(t, "LABUSDT", "0.75", "MUSDT", "0.25")

	missing := map[string][]fixedpoint.Rate{"LABUSDT": series(t, "0.0001"), "SOONUSDT": series(t, "0.0001")}
	if _, err := crossvenue.PortfolioReturns(pol, missing); !errors.Is(err, crossvenue.ErrSeriesMisaligned) {
		t.Fatalf("held symbol unmeasured: %v, want ErrSeriesMisaligned", err)
	}

	extra := map[string][]fixedpoint.Rate{
		"LABUSDT": series(t, "0.0001"), "MUSDT": series(t, "0.0001"), "SOONUSDT": series(t, "0.0001"),
	}
	if _, err := crossvenue.PortfolioReturns(pol, extra); !errors.Is(err, crossvenue.ErrSeriesMisaligned) {
		t.Fatalf("measured symbol unheld: %v, want ErrSeriesMisaligned", err)
	}

	ragged := map[string][]fixedpoint.Rate{
		"LABUSDT": series(t, "0.0001", "0.0002"), "MUSDT": series(t, "0.0001"),
	}
	if _, err := crossvenue.PortfolioReturns(pol, ragged); !errors.Is(err, crossvenue.ErrSeriesMisaligned) {
		t.Fatalf("ragged series: %v, want ErrSeriesMisaligned", err)
	}

	empty := map[string][]fixedpoint.Rate{"LABUSDT": {}, "MUSDT": {}}
	if _, err := crossvenue.PortfolioReturns(pol, empty); !errors.Is(err, crossvenue.ErrNoReturns) {
		t.Fatalf("zero periods: %v, want ErrNoReturns", err)
	}
}

func TestLCB_SingleReturnIsItself(t *testing.T) {
	// n=1: every resample path is the single return, every mean equals
	// it, and the bound at ANY confidence is that value — the degenerate
	// anchor that pins nothing about rank but everything about plumbing.
	got, err := crossvenue.StationaryBootstrapLCB(series(t, "0.0007"), 3, 25, rate(t, "0.95"), 42)
	if err != nil {
		t.Fatalf("LCB: %v", err)
	}
	wantRate(t, "single-return LCB", got, "0.0007")
}

func TestLCB_DeterministicUnderSeed(t *testing.T) {
	s := series(t, "0.0001", "-0.0003", "0.0009", "0.0002", "-0.0001")
	a, err := crossvenue.StationaryBootstrapLCB(s, 2, 199, rate(t, "0.95"), 7)
	if err != nil {
		t.Fatalf("LCB: %v", err)
	}
	b, err := crossvenue.StationaryBootstrapLCB(s, 2, 199, rate(t, "0.95"), 7)
	if err != nil {
		t.Fatalf("LCB: %v", err)
	}
	if a.Mantissa() != b.Mantissa() {
		t.Fatalf("same seed disagrees: %s vs %s", a.Decimal(), b.Decimal())
	}
	// Two particular seeds may honestly land the same order statistic in a
	// small discrete value space; twenty seeds all agreeing means the seed
	// never reached the stream.
	distinct := map[int64]bool{a.Mantissa(): true}
	for seed := uint64(0); seed < 20; seed++ {
		c, err := crossvenue.StationaryBootstrapLCB(s, 2, 199, rate(t, "0.95"), seed)
		if err != nil {
			t.Fatalf("LCB seed %d: %v", seed, err)
		}
		distinct[c.Mantissa()] = true
	}
	if len(distinct) < 2 {
		t.Fatalf("twenty-one seeds produced one value (%s) — the seed is not reaching the stream", a.Decimal())
	}
}

func TestLCB_BoundIsBelowTheMean(t *testing.T) {
	// A dispersed series: the 5% LCB of the mean must sit at or below the
	// full-sample mean (0.0002 here) — the direction the bound serves.
	s := series(t, "0.001", "-0.0006", "0.0008", "-0.0004", "0.0002")
	got, err := crossvenue.StationaryBootstrapLCB(s, 2, 499, rate(t, "0.95"), 42)
	if err != nil {
		t.Fatalf("LCB: %v", err)
	}
	mean := rate(t, "0.0002")
	if c, err := got.Cmp(mean); err != nil {
		t.Fatal(err)
	} else if c > 0 {
		t.Fatalf("LCB %s above the sample mean %s", got.Decimal(), mean.Decimal())
	}
}

func TestLCB_Refusals(t *testing.T) {
	s := series(t, "0.0001", "0.0002")
	conf := rate(t, "0.95")

	if _, err := crossvenue.StationaryBootstrapLCB(nil, 2, 100, conf, 1); !errors.Is(err, crossvenue.ErrNoReturns) {
		t.Fatalf("empty series: %v, want ErrNoReturns", err)
	}
	if _, err := crossvenue.StationaryBootstrapLCB(s, 0, 100, conf, 1); !errors.Is(err, crossvenue.ErrBootstrapInvalid) {
		t.Fatalf("mean block 0: %v, want ErrBootstrapInvalid", err)
	}
	if _, err := crossvenue.StationaryBootstrapLCB(s, 2, 0, conf, 1); !errors.Is(err, crossvenue.ErrBootstrapInvalid) {
		t.Fatalf("resamples 0: %v, want ErrBootstrapInvalid", err)
	}
	for _, bad := range []string{"0", "1", "1.5", "-0.5"} {
		if _, err := crossvenue.StationaryBootstrapLCB(s, 2, 100, rate(t, bad), 1); !errors.Is(err, crossvenue.ErrBootstrapInvalid) {
			t.Fatalf("confidence %s: %v, want ErrBootstrapInvalid", bad, err)
		}
	}
	unset := []fixedpoint.Rate{{}}
	if _, err := crossvenue.StationaryBootstrapLCB(unset, 2, 100, conf, 1); err == nil {
		t.Fatal("unset return accepted")
	}
}
