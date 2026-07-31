package domain_test

// The joint-rule battery. The fixture's expected and adverse sums are
// DIFFERENT numbers with different digit patterns, so a mutant summing the
// wrong field moves an assertion rather than nothing.

import (
	"errors"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/strategy/domain"
)

func aggForecast(t *testing.T, at time.Time, expected, adverse string) domain.SettlementForecast {
	t.Helper()
	return domain.SettlementForecast{
		At:         at,
		Expected:   mustRate(t, expected),
		Adverse:    mustRate(t, adverse),
		Quantile:   mustRate(t, "0.25"),
		Dispersion: fixtureCalibration("dispersion"),
	}
}

func aggFixture(t *testing.T) []domain.SettlementForecast {
	t.Helper()
	return []domain.SettlementForecast{
		aggForecast(t, baseAt.Add(2*time.Hour), "0.00010", "0.00003"),
		aggForecast(t, baseAt.Add(10*time.Hour), "0.00012", "0.00001"),
		aggForecast(t, baseAt.Add(18*time.Hour), "0.00009", "-0.00002"),
	}
}

func TestAggregate_ConservativeSums(t *testing.T) {
	out, err := domain.Aggregate(domain.AggregationConservative, aggFixture(t))
	if err != nil {
		t.Fatalf("Aggregate: %v", err)
	}
	if got, want := out.Expected.Decimal(), "0.00031"; got != want {
		t.Errorf("Expected sum = %s; want %s", got, want)
	}
	// The adverse sum is every settlement at its adverse quantile
	// SIMULTANEOUSLY — §2.4's one co-moving scenario, not independent draws.
	if got, want := out.Adverse.Decimal(), "0.00002"; got != want {
		t.Errorf("Adverse sum = %s; want %s", got, want)
	}
}

func TestAggregate_Refusals(t *testing.T) {
	// Empty: the no-settlement case is the caller's, not a zero scenario.
	if _, err := domain.Aggregate(domain.AggregationConservative, nil); !errors.Is(err, domain.ErrUnset) {
		t.Errorf("empty = %v; want ErrUnset", err)
	}

	// An undeclared rule is refused by the switch itself — including the
	// zero value, which is a rule nobody chose.
	if _, err := domain.Aggregate(domain.Aggregation("independent_draws"), aggFixture(t)); !errors.Is(err, domain.ErrKindInvalid) {
		t.Errorf("undeclared rule = %v; want ErrKindInvalid", err)
	}
	if _, err := domain.Aggregate(domain.Aggregation(""), aggFixture(t)); !errors.Is(err, domain.ErrKindInvalid) {
		t.Errorf("zero rule = %v; want ErrKindInvalid", err)
	}

	// Order.
	swapped := aggFixture(t)
	swapped[0], swapped[1] = swapped[1], swapped[0]
	if _, err := domain.Aggregate(domain.AggregationConservative, swapped); !errors.Is(err, domain.ErrNotAscending) {
		t.Errorf("descending = %v; want ErrNotAscending", err)
	}

	// One quantile per aggregate: p25 and p10 draws in one sum is a
	// scenario nobody stated.
	mixedQ := aggFixture(t)
	mixedQ[1].Quantile = mustRate(t, "0.10")
	if _, err := domain.Aggregate(domain.AggregationConservative, mixedQ); !errors.Is(err, domain.ErrQuantileMixed) {
		t.Errorf("mixed quantiles = %v; want ErrQuantileMixed", err)
	}

	// One artifact per aggregate — and a differing HASH alone is a
	// different artifact: same ID + new content is exactly the re-measured
	// artifact §40e's one-artifact rule exists to catch.
	mixedA := aggFixture(t)
	mixedA[2].Dispersion.Hash = "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
	if _, err := domain.Aggregate(domain.AggregationConservative, mixedA); !errors.Is(err, domain.ErrArtifactMixed) {
		t.Errorf("mixed artifact hash = %v; want ErrArtifactMixed", err)
	}

	// An invalid forecast refuses.
	invalid := aggFixture(t)
	invalid[1].Quantile = mustRate(t, "0")
	if _, err := domain.Aggregate(domain.AggregationConservative, invalid); !errors.Is(err, domain.ErrQuantileRange) {
		t.Errorf("invalid forecast = %v; want ErrQuantileRange", err)
	}
}
