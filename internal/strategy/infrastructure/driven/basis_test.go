package driven_test

// The conservative-basis battery. The charge fixture's product runs past
// scale 9, so the away-from-zero rounding is a VISIBLE digit — a mutant
// rounding half-even moves the assertion instead of nothing.

import (
	"errors"
	"testing"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/strategy/domain"
	"github.com/giiorg/arb-lab/internal/strategy/infrastructure/driven"
)

func fixtureBasis(t *testing.T, rate string) driven.ConservativeBasis {
	t.Helper()
	b, err := driven.NewConservativeBasis(domain.MeasuredRate{
		Value:    mustRate(t, rate),
		Artifact: modelCalibration("basis"),
	})
	if err != nil {
		t.Fatalf("NewConservativeBasis: %v", err)
	}
	return b
}

func TestConservativeBasis_ChargesTheMeasuredMove(t *testing.T) {
	// qty 0.15 at the spot ASK 64000.50 → notional 9600.075 exactly; times
	// 0.000000123 → 0.001180809225, which is 12 decimal places: away from
	// zero the 9th digit becomes ...810. Half-even would print ...809 — the
	// rounding direction is a visible digit here.
	b := fixtureBasis(t, "0.000000123")
	term, err := b.EstimateBasis(modelView(t), modelPlan(t), mustQty(t, "0.15"))
	if err != nil {
		t.Fatalf("EstimateBasis: %v", err)
	}
	if got, want := term.AdverseMove.Amount.Decimal(), "0.00118081"; got != want {
		t.Errorf("AdverseMove = %s; want %s", got, want)
	}
	if got, want := term.AdverseMove.Artifact, modelCalibration("basis"); got != want {
		t.Errorf("Artifact = %+v; want %+v", got, want)
	}

	// The favorable leg is a DECLARED zero: constructed, right asset, zero
	// magnitude — the §2.4 conservative rule, not an unset field.
	if err := term.FavorableConvergence.Validate(); err != nil {
		t.Errorf("FavorableConvergence is not a constructed value: %v", err)
	}
	if sign, err := term.FavorableConvergence.Sign(); err != nil || sign != 0 {
		t.Errorf("FavorableConvergence sign = %d, %v; want a declared zero", sign, err)
	}
	if got, want := term.FavorableConvergence.Asset(), fixedpoint.Asset("USDT"); got != want {
		t.Errorf("FavorableConvergence asset = %s; want %s", got, want)
	}
}

// The notional is priced at the ASK, not the bid — the higher price, the
// larger charge. The fixture's spread makes the two prices produce different
// charges at scale 9.
func TestConservativeBasis_PricesAtTheAsk(t *testing.T) {
	// 0.15 × ask 64000.50 × 0.0001 = 0.96000750 (from 9600.075);
	// at the bid it would be 0.15 × 64000.00 × 0.0001 = 0.96 exactly.
	b := fixtureBasis(t, "0.0001")
	term, err := b.EstimateBasis(modelView(t), modelPlan(t), mustQty(t, "0.15"))
	if err != nil {
		t.Fatalf("EstimateBasis: %v", err)
	}
	if got, want := term.AdverseMove.Amount.Decimal(), "0.9600075"; got != want {
		t.Errorf("AdverseMove = %s; want %s (the ask-priced charge)", got, want)
	}
}

func TestConservativeBasis_Refusals(t *testing.T) {
	// A negative measured "adverse" move is a credit wearing the wrong sign.
	if _, err := driven.NewConservativeBasis(domain.MeasuredRate{
		Value:    mustRate(t, "-0.0001"),
		Artifact: modelCalibration("basis"),
	}); !errors.Is(err, driven.ErrChargeNegative) {
		t.Errorf("negative adverse = %v; want ErrChargeNegative", err)
	}

	// The class-2 refusal: no artifact, no model.
	if _, err := driven.NewConservativeBasis(domain.MeasuredRate{
		Value: mustRate(t, "0.0001"),
	}); !errors.Is(err, domain.ErrUnset) {
		t.Errorf("artifact-less measure = %v; want ErrUnset", err)
	}

	// A measured ZERO is admissible — declared and artifact-backed is not
	// defaulted.
	if _, err := driven.NewConservativeBasis(domain.MeasuredRate{
		Value:    mustRate(t, "0"),
		Artifact: modelCalibration("basis"),
	}); err != nil {
		t.Errorf("measured zero refused: %v", err)
	}

	b := fixtureBasis(t, "0.0001")

	// The zero value is unusable through the method door.
	var zero driven.ConservativeBasis
	if _, err := zero.EstimateBasis(modelView(t), modelPlan(t), mustQty(t, "0.15")); !errors.Is(err, domain.ErrUnset) {
		t.Errorf("zero adapter = %v; want ErrUnset", err)
	}

	// Candidate quantity must be strictly positive.
	for _, q := range []string{"0", "-0.1"} {
		if _, err := b.EstimateBasis(modelView(t), modelPlan(t), mustQty(t, q)); !errors.Is(err, driven.ErrQtyNotPositive) {
			t.Errorf("qty %s = %v; want ErrQtyNotPositive", q, err)
		}
	}

	// Malformed inputs refuse rather than price.
	if _, err := b.EstimateBasis(domain.MarketView{}, modelPlan(t), mustQty(t, "0.15")); err == nil {
		t.Error("zero view priced")
	}
	if _, err := b.EstimateBasis(modelView(t), domain.HoldPlanRevision{}, mustQty(t, "0.15")); err == nil {
		t.Error("zero plan priced")
	}
}
