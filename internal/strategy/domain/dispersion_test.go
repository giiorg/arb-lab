package domain_test

// The dispersion battery. The fixture answers two of §40j's named checklist
// suspects by construction: the distribution has WIDTH (a zero-width fixture
// collapses "quantile" into "mean" — every selection returns the same shift
// and the machinery is untestable while looking covered), and it is
// ASYMMETRIC (a symmetric fixture collapses p25 into p75: |shift| equal both
// sides means a swapped-tail mutant changes nothing).

import (
	"errors"
	"testing"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/strategy/domain"
)

// fixtureDispersion: five points, strictly ascending Q, non-decreasing
// shifts, asymmetric about the median (|−0.00013| ≠ 0.00009), median shift
// deliberately NONZERO so "shift at the median" and "no shift" are distinct
// answers.
func fixtureDispersion(t *testing.T) domain.Dispersion {
	t.Helper()
	return domain.Dispersion{
		Points: []domain.QuantilePoint{
			{Q: mustRate(t, "0.10"), Shift: mustRate(t, "-0.00030")},
			{Q: mustRate(t, "0.25"), Shift: mustRate(t, "-0.00013")},
			{Q: mustRate(t, "0.50"), Shift: mustRate(t, "0.00002")},
			{Q: mustRate(t, "0.75"), Shift: mustRate(t, "0.00009")},
			{Q: mustRate(t, "0.90"), Shift: mustRate(t, "0.00041")},
		},
		Artifact: fixtureCalibration("dispersion"),
	}
}

func TestDispersion_ValidateRefusals(t *testing.T) {
	cases := []struct {
		name    string
		corrupt func(*domain.Dispersion)
		want    error
	}{
		{"no artifact", func(d *domain.Dispersion) { d.Artifact = domain.CalibrationRef{} }, domain.ErrUnset},
		{"no points", func(d *domain.Dispersion) { d.Points = nil }, domain.ErrUnset},
		{"quantile zero", func(d *domain.Dispersion) { d.Points[0].Q = mustRate(t, "0") }, domain.ErrQuantileRange},
		{"quantile one", func(d *domain.Dispersion) { d.Points[4].Q = mustRate(t, "1") }, domain.ErrQuantileRange},
		{"quantile unset", func(d *domain.Dispersion) { d.Points[2].Q = fixedpoint.Rate{} }, fixedpoint.ErrRateUnset},
		{"shift unset", func(d *domain.Dispersion) { d.Points[2].Shift = fixedpoint.Rate{} }, fixedpoint.ErrRateUnset},
		{"quantiles duplicated", func(d *domain.Dispersion) { d.Points[1].Q = d.Points[0].Q }, domain.ErrDispersionShape},
		{"quantiles descending", func(d *domain.Dispersion) {
			d.Points[0].Q, d.Points[1].Q = d.Points[1].Q, d.Points[0].Q
		}, domain.ErrDispersionShape},
		{"shifts decreasing", func(d *domain.Dispersion) {
			d.Points[3].Shift = mustRate(t, "-0.00050")
		}, domain.ErrDispersionShape},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := fixtureDispersion(t)
			tc.corrupt(&d)
			if err := d.Validate(); !errors.Is(err, tc.want) {
				t.Fatalf("Validate = %v; want %v", err, tc.want)
			}
		})
	}

	// Positive control: the unbroken fixture validates.
	if err := fixtureDispersion(t).Validate(); err != nil {
		t.Fatalf("fixture: %v", err)
	}

	// Equal neighbouring SHIFTS are legal — a flat stretch of a quantile
	// function is a real shape, only a DECREASE is a refusal.
	d := fixtureDispersion(t)
	d.Points[3].Shift = d.Points[2].Shift
	if err := d.Validate(); err != nil {
		t.Fatalf("flat shift stretch refused: %v", err)
	}
}

func TestDispersion_ShiftAtSelectsOutward(t *testing.T) {
	d := fixtureDispersion(t)
	cases := []struct {
		name string
		q    string
		want string
	}{
		// Exact hits return the point, both tails.
		{"exact p25", "0.25", "-0.00013"},
		{"exact median", "0.50", "0.00002"},
		{"exact p75", "0.75", "0.00009"},
		{"exact lower edge", "0.10", "-0.0003"},
		{"exact upper edge", "0.90", "0.00041"},

		// Below the median, selection rounds DOWN to the more extreme lower
		// point — the conservative direction for a lower-tail draw.
		{"lower tail between", "0.20", "-0.0003"},
		{"lower tail near median", "0.40", "-0.00013"},

		// At or above the median it rounds UP to the more extreme upper
		// point.
		{"upper tail between", "0.60", "0.00009"},
		{"upper tail near edge", "0.80", "0.00041"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := d.ShiftAt(mustRate(t, tc.q))
			if err != nil {
				t.Fatalf("ShiftAt(%s): %v", tc.q, err)
			}
			if got.Decimal() != tc.want {
				t.Errorf("ShiftAt(%s) = %s; want %s", tc.q, got.Decimal(), tc.want)
			}
		})
	}
}

// The tail rule's boundary is the median, and it is pinned from BOTH sides
// at ulp distance: exactly 0.5 between two points takes the upper (the `<`
// versus `<=` distinction), one ulp below takes the lower. Together the two
// cases pin the constant itself — a boundary moved anywhere, in either
// direction, fails one of them; a single-sided pin leaves every constant
// between the probe and 0.5 indistinguishable.
func TestDispersion_MedianBoundaryPinnedFromBothSides(t *testing.T) {
	d := domain.Dispersion{
		Points: []domain.QuantilePoint{
			{Q: mustRate(t, "0.40"), Shift: mustRate(t, "-0.00007")},
			{Q: mustRate(t, "0.60"), Shift: mustRate(t, "0.00011")},
		},
		Artifact: fixtureCalibration("dispersion"),
	}
	got, err := d.ShiftAt(mustRate(t, "0.5"))
	if err != nil {
		t.Fatalf("ShiftAt(0.5): %v", err)
	}
	if want := "0.00011"; got.Decimal() != want {
		t.Errorf("ShiftAt(0.5) = %s; want the upper point %s", got.Decimal(), want)
	}

	below, err := d.ShiftAt(mustRate(t, "0.499999999"))
	if err != nil {
		t.Fatalf("ShiftAt(0.499999999): %v", err)
	}
	if want := "-0.00007"; below.Decimal() != want {
		t.Errorf("ShiftAt(0.499999999) = %s; want the lower point %s", below.Decimal(), want)
	}
}

func TestDispersion_ShiftAtRefusals(t *testing.T) {
	d := fixtureDispersion(t)

	// Outside the measured span: refused, never answered from the edge —
	// the edge point would UNDERSTATE a tail the artifact never reached.
	for _, q := range []string{"0.05", "0.95"} {
		if _, err := d.ShiftAt(mustRate(t, q)); !errors.Is(err, domain.ErrQuantileUnsupported) {
			t.Errorf("ShiftAt(%s) = %v; want ErrQuantileUnsupported", q, err)
		}
	}

	// A quantile outside (0,1) is malformed before it is unsupported.
	if _, err := d.ShiftAt(mustRate(t, "0")); !errors.Is(err, domain.ErrQuantileRange) {
		t.Errorf("ShiftAt(0) = %v; want ErrQuantileRange", err)
	}

	// The zero value is unusable through the method door too.
	var zero domain.Dispersion
	if _, err := zero.ShiftAt(mustRate(t, "0.25")); err == nil {
		t.Error("zero Dispersion answered ShiftAt")
	}
}
