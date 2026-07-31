package domain_test

// The estimator battery. The clamp fixtures answer §40j's second named
// suspect: a cap/floor a forecast never reaches is a clamp
// indistinguishable from no clamp, so BOTH directions get a fixture where
// the clamp visibly BINDS.

import (
	"errors"
	"testing"
	"time"

	mdomain "github.com/giiorg/arb-lab/internal/marketdata/domain"
	"github.com/giiorg/arb-lab/internal/strategy/domain"
)

func fixtureEstimator(t *testing.T) domain.EWMAEstimator {
	t.Helper()
	e, err := domain.NewEWMAEstimator(72*time.Hour, mustRate(t, "0.25"), fixtureDispersion(t))
	if err != nil {
		t.Fatalf("NewEWMAEstimator: %v", err)
	}
	return e
}

// materials: live estimate 0.00011, next settle +2h, clamps ±0.0075,
// realized window [0.00009 @ -14h, 0.00012 @ -6h].
func fixtureMaterials(t *testing.T) domain.FundingMaterials {
	t.Helper()
	return domain.FundingMaterials{
		Observation: fixtureFunding(t),
		Realized: []domain.RealizedSettlement{
			settlement(t, baseAt.Add(-14*time.Hour), "0.00009"),
			settlement(t, baseAt.Add(-6*time.Hour), "0.00012"),
		},
	}
}

func TestNewEWMAEstimator_Refusals(t *testing.T) {
	q := mustRate(t, "0.25")
	d := fixtureDispersion(t)

	if _, err := domain.NewEWMAEstimator(0, q, d); !errors.Is(err, domain.ErrUnset) {
		t.Errorf("zero half-life = %v; want ErrUnset", err)
	}
	if _, err := domain.NewEWMAEstimator(72*time.Hour, mustRate(t, "1"), d); !errors.Is(err, domain.ErrQuantileRange) {
		t.Errorf("quantile 1 = %v; want ErrQuantileRange", err)
	}
	// The class-2 refusal: a dispersion with no artifact cannot be built
	// into an estimator, which is what keeps a made-up distribution out of
	// every forecast downstream.
	d.Artifact = domain.CalibrationRef{}
	if _, err := domain.NewEWMAEstimator(72*time.Hour, q, d); !errors.Is(err, domain.ErrUnset) {
		t.Errorf("artifact-less dispersion = %v; want ErrUnset", err)
	}

	// The zero value is unusable through the method door.
	var zero domain.EWMAEstimator
	if _, err := zero.Estimate(fixtureMaterials(t), []time.Time{baseAt.Add(2 * time.Hour)}); err == nil {
		t.Error("zero estimator estimated")
	}
}

func TestEstimate_ImminentTakesLiveEstimate(t *testing.T) {
	m := fixtureMaterials(t)
	// The imminent settlement needs no history: an empty window must not
	// refuse a horizon that never reaches past it.
	m.Realized = nil

	fs, err := fixtureEstimator(t).Estimate(m, []time.Time{m.Observation.NextSettle})
	if err != nil {
		t.Fatalf("Estimate: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("got %d forecasts; want 1", len(fs))
	}
	if got, want := fs[0].Expected.Decimal(), "0.00011"; got != want {
		t.Errorf("imminent Expected = %s; want the live estimate %s", got, want)
	}
	// Adverse = live + p25 shift (-0.00013), clamps far away: -0.00002.
	if got, want := fs[0].Adverse.Decimal(), "-0.00002"; got != want {
		t.Errorf("imminent Adverse = %s; want %s", got, want)
	}
	if got, want := fs[0].Quantile.Decimal(), "0.25"; got != want {
		t.Errorf("Quantile = %s; want %s", got, want)
	}
	if got, want := fs[0].Dispersion, fixtureCalibration("dispersion"); got != want {
		t.Errorf("Dispersion = %+v; want %+v", got, want)
	}
	if !fs[0].At.Equal(m.Observation.NextSettle) {
		t.Errorf("At = %s; want %s", fs[0].At, m.Observation.NextSettle)
	}
}

func TestEstimate_LaterSettlementsTakeTheEWMA(t *testing.T) {
	m := fixtureMaterials(t)
	next := m.Observation.NextSettle
	horizon := []time.Time{next, next.Add(8 * time.Hour), next.Add(16 * time.Hour)}

	fs, err := fixtureEstimator(t).Estimate(m, horizon)
	if err != nil {
		t.Fatalf("Estimate: %v", err)
	}
	if len(fs) != 3 {
		t.Fatalf("got %d forecasts; want 3", len(fs))
	}
	// The imminent one is the live estimate; the later two are the SAME
	// trailing EWMA — §2.4's estimator has one number for everything beyond
	// the next settlement.
	if got, want := fs[0].Expected.Decimal(), "0.00011"; got != want {
		t.Errorf("imminent Expected = %s; want %s", got, want)
	}
	ewma, err := domain.EWMA(m.Realized, 72*time.Hour)
	if err != nil {
		t.Fatalf("EWMA: %v", err)
	}
	for n := 1; n < 3; n++ {
		if c, err := fs[n].Expected.Cmp(ewma); err != nil || c != 0 {
			t.Errorf("later Expected[%d] = %s; want the EWMA %s (cmp %d, %v)",
				n, fs[n].Expected.Decimal(), ewma.Decimal(), c, err)
		}
	}
	// And the later expected is NOT the live estimate — the two rules are
	// distinguishable in this fixture (0.00011 live vs an EWMA of
	// 0.00009/0.00012 history).
	if c, err := fs[1].Expected.Cmp(fs[0].Expected); err != nil || c == 0 {
		t.Errorf("later Expected equals the live estimate %s — the fixture collapsed (cmp %d, %v)",
			fs[0].Expected.Decimal(), c, err)
	}
}

// A horizon that never CONTAINS the imminent settlement — the exit shape,
// where the first remaining settlement sits past NextSettle — takes the EWMA
// for every entry. This is what distinguishes "the instant equals
// NextSettle" from "the first horizon entry": every other fixture starts its
// horizon AT NextSettle, where an index-based imminent rule is the same
// program.
func TestEstimate_HorizonWithoutTheImminentIsAllEWMA(t *testing.T) {
	m := fixtureMaterials(t)
	next := m.Observation.NextSettle
	fs, err := fixtureEstimator(t).Estimate(m, []time.Time{next.Add(8 * time.Hour), next.Add(16 * time.Hour)})
	if err != nil {
		t.Fatalf("Estimate: %v", err)
	}
	ewma, err := domain.EWMA(m.Realized, 72*time.Hour)
	if err != nil {
		t.Fatalf("EWMA: %v", err)
	}
	for n := range fs {
		if c, err := fs[n].Expected.Cmp(ewma); err != nil || c != 0 {
			t.Errorf("Expected[%d] = %s; want the EWMA %s (cmp %d, %v)",
				n, fs[n].Expected.Decimal(), ewma.Decimal(), c, err)
		}
	}
	// And in particular NOT the live estimate on the first entry.
	if c, err := fs[0].Expected.Cmp(m.Observation.Rate); err != nil || c == 0 {
		t.Errorf("Expected[0] equals the live estimate — the imminent rule fired on an index (cmp %d, %v)", c, err)
	}
}

// The cap binds: a live estimate ABOVE the live cap forecasts AT the cap,
// and the adverse draw added on top stays inside it too.
func TestEstimate_CapBinds(t *testing.T) {
	m := fixtureMaterials(t)
	m.Observation.Rate = mustRate(t, "0.009") // above the 0.0075 cap
	m.Observation.Cap = mustRate(t, "0.0075")

	fs, err := fixtureEstimator(t).Estimate(m, []time.Time{m.Observation.NextSettle})
	if err != nil {
		t.Fatalf("Estimate: %v", err)
	}
	if got, want := fs[0].Expected.Decimal(), "0.0075"; got != want {
		t.Errorf("Expected = %s; want the cap %s", got, want)
	}
}

// The floor binds on the ADVERSE side: an expected rate near the floor with
// a lower-tail shift would fall through it, and the venue's own clamp is
// what stops it — no settlement can print below the floor.
func TestEstimate_FloorBindsTheAdverse(t *testing.T) {
	m := fixtureMaterials(t)
	m.Observation.Rate = mustRate(t, "-0.00745") // 0.00005 above the floor
	fs, err := fixtureEstimator(t).Estimate(m, []time.Time{m.Observation.NextSettle})
	if err != nil {
		t.Fatalf("Estimate: %v", err)
	}
	if got, want := fs[0].Expected.Decimal(), "-0.00745"; got != want {
		t.Errorf("Expected = %s; want %s (unclamped)", got, want)
	}
	// -0.00745 + (-0.00013) = -0.00758, below the -0.0075 floor.
	if got, want := fs[0].Adverse.Decimal(), "-0.0075"; got != want {
		t.Errorf("Adverse = %s; want the floor %s", got, want)
	}
}

// An upper-tail estimator (q = 0.75) draws ABOVE expected: the shift's sign
// flows through unmolested, which is what makes the machinery usable for
// the payer side too.
func TestEstimate_UpperTailDrawsAbove(t *testing.T) {
	e, err := domain.NewEWMAEstimator(72*time.Hour, mustRate(t, "0.75"), fixtureDispersion(t))
	if err != nil {
		t.Fatalf("NewEWMAEstimator: %v", err)
	}
	m := fixtureMaterials(t)
	fs, err := e.Estimate(m, []time.Time{m.Observation.NextSettle})
	if err != nil {
		t.Fatalf("Estimate: %v", err)
	}
	// 0.00011 + 0.00009 = 0.00020.
	if got, want := fs[0].Adverse.Decimal(), "0.0002"; got != want {
		t.Errorf("Adverse = %s; want %s", got, want)
	}
}

func TestEstimate_Refusals(t *testing.T) {
	e := fixtureEstimator(t)
	m := fixtureMaterials(t)
	next := m.Observation.NextSettle

	// A horizon reaching past the imminent settlement with no history.
	empty := m
	empty.Realized = nil
	if _, err := e.Estimate(empty, []time.Time{next, next.Add(8 * time.Hour)}); !errors.Is(err, domain.ErrWindowEmpty) {
		t.Errorf("later settlement on an empty window = %v; want ErrWindowEmpty", err)
	}

	// A horizon instant before the venue's next settlement — settled, or on
	// a schedule the venue does not hold.
	if _, err := e.Estimate(m, []time.Time{next.Add(-time.Hour)}); !errors.Is(err, domain.ErrHorizonBehind) {
		t.Errorf("horizon behind = %v; want ErrHorizonBehind", err)
	}

	// Order and shape.
	if _, err := e.Estimate(m, []time.Time{next.Add(8 * time.Hour), next}); !errors.Is(err, domain.ErrNotAscending) {
		t.Errorf("descending horizon = %v; want ErrNotAscending", err)
	}
	if _, err := e.Estimate(m, []time.Time{next, next}); !errors.Is(err, domain.ErrNotAscending) {
		t.Errorf("duplicate horizon = %v; want ErrNotAscending", err)
	}
	if _, err := e.Estimate(m, []time.Time{{}}); !errors.Is(err, domain.ErrUnset) {
		t.Errorf("zero horizon instant = %v; want ErrUnset", err)
	}

	// Invalid materials refuse before anything is computed.
	bad := m
	bad.Observation = mdomain.FundingObservation{}
	if _, err := e.Estimate(bad, []time.Time{next}); err == nil {
		t.Error("invalid materials estimated")
	}

	// An empty horizon is a real answer, not a refusal: a hold too short to
	// reach a settlement captures nothing (Enumerate's own rule).
	fs, err := e.Estimate(m, nil)
	if err != nil {
		t.Fatalf("empty horizon: %v", err)
	}
	if len(fs) != 0 {
		t.Errorf("empty horizon produced %d forecasts", len(fs))
	}
}

// Every forecast the estimator emits validates — the port's consumers
// receive constructed values, not raw structs.
func TestEstimate_OutputValidates(t *testing.T) {
	m := fixtureMaterials(t)
	next := m.Observation.NextSettle
	fs, err := fixtureEstimator(t).Estimate(m, []time.Time{next, next.Add(8 * time.Hour)})
	if err != nil {
		t.Fatalf("Estimate: %v", err)
	}
	for n, f := range fs {
		if err := f.Validate(); err != nil {
			t.Errorf("forecast %d: %v", n, err)
		}
	}
}
