package domain_test

// The zero-value battery: every constructed type in the package fails
// Validate when nobody populated it (§11's rule — an unpopulated cost reads
// as free, an unpopulated rate as zero fee, and both compute the economic
// gate in our favour with nothing failing).

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/strategy/domain"
)

func TestZeroValuesAreUnusable(t *testing.T) {
	cases := []struct {
		name string
		v    interface{ Validate() error }
	}{
		{"Side", domain.Side("")},
		{"IntentKind", domain.IntentKind("")},
		{"LegSpec", domain.LegSpec{}},
		{"Intent", domain.Intent{}},
		{"PriceView", domain.PriceView{}},
		{"RealizedSettlement", domain.RealizedSettlement{}},
		{"FundingMaterials", domain.FundingMaterials{}},
		{"MarketView", domain.MarketView{}},
		{"PositionView", domain.PositionView{}},
		{"HoldPlan", domain.HoldPlan{}},
		{"PlanTrigger", domain.PlanTrigger("")},
		{"HoldPlanRevision", domain.HoldPlanRevision{}},
		{"CalibrationRef", domain.CalibrationRef{}},
		{"MeasuredRate", domain.MeasuredRate{}},
		{"MeasuredMoney", domain.MeasuredMoney{}},
		{"SettlementForecast", domain.SettlementForecast{}},
		{"Aggregation", domain.Aggregation("")},
		{"StrategyParams", domain.StrategyParams{}},
		{"PolicyProjection", domain.PolicyProjection{}},
		{"ProvenanceClass", domain.ProvenanceClass("")},
		{"Provenance", domain.Provenance{}},
		{"LegFees", domain.LegFees{}},
		{"LotRules", domain.LotRules{}},
		{"HurdleBenchmark", domain.HurdleBenchmark{}},
		{"CapitalAllocation", domain.CapitalAllocation{}},
		{"ExecTactic", domain.ExecTactic("")},
		{"ExecutionCostRequest", domain.ExecutionCostRequest{}},
		{"ExecutionCostEstimate", domain.ExecutionCostEstimate{}},
		{"BasisTerm", domain.BasisTerm{}},
		{"DecisionKind", domain.DecisionKind("")},
		{"Verdict", domain.Verdict("")},
		{"DwellState", domain.DwellState{}},
		{"DecisionInput", domain.DecisionInput{}},
		{"Decision", domain.Decision{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.v.Validate(); err == nil {
				t.Fatalf("zero %s validated", tc.name)
			}
		})
	}

	// DecisionTerms.Validate takes the kind; its zero value must fail for
	// both.
	if err := (domain.DecisionTerms{}).Validate(domain.KindEntry); err == nil {
		t.Fatal("zero DecisionTerms validated for entry")
	}
	if err := (domain.DecisionTerms{}).Validate(domain.KindExit); err == nil {
		t.Fatal("zero DecisionTerms validated for exit")
	}
}

// TestFixturesAreValid is the positive control: the batteries below mutate
// these fixtures one member at a time, and a broken fixture would make every
// refusal below vacuous — a universally-objecting validator reads as a
// perfect score without this.
func TestFixturesAreValid(t *testing.T) {
	for _, tc := range []struct {
		name string
		v    interface{ Validate() error }
	}{
		{"entry input", entryInput(t)},
		{"exit input", exitInput(t)},
		{"entry decision", entryDecision(t)},
		{"exit decision", exitDecision(t)},
		{"refusal decision", refusalDecision(t)},
	} {
		if err := tc.v.Validate(); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
	}
}

// TestValidateNamesTheField spot-checks that a refusal names the member that
// tripped it — the failure an operator gets must be "a refusal that names
// the field", not a bare sentinel.
func TestValidateNamesTheField(t *testing.T) {
	in := entryInput(t)
	in.Hurdle = domain.HurdleBenchmark{}
	err := in.Validate()
	if err == nil {
		t.Fatal("input with zero hurdle validated")
	}
	if got := err.Error(); !strings.Contains(got, "hurdle") {
		t.Fatalf("error does not name the hurdle: %q", got)
	}

	// And the package's own sentinel is wired for members it owns.
	in = entryInput(t)
	in.At = time.Time{}
	if err := in.Validate(); !errors.Is(err, domain.ErrUnset) {
		t.Fatalf("want ErrUnset for missing instant, got %v", err)
	}
}
