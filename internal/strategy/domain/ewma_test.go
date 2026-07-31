package domain_test

// The EWMA battery pins BEHAVIOUR with algebraically exact cases; the digit
//-level arithmetic is the oracle's (models_oracle_test.go). The central
// fixture answers §40j's first named suspect by construction: a window of
// ONE settlement is where "EWMA" and "the last value" collapse, so the load-
// bearing fixtures hold two-plus settlements with DISTINCT rates at DISTINCT
// ages — making "last value", "arithmetic mean" and "EWMA" three different
// numbers.

import (
	"errors"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/strategy/domain"
)

func settlement(t *testing.T, at time.Time, rate string) domain.RealizedSettlement {
	t.Helper()
	return domain.RealizedSettlement{
		Instrument: perpInst,
		At:         at,
		Rate:       mustRate(t, rate),
		Watermark:  watermark(at, 5),
	}
}

func TestEWMA_WindowOfOneIsThatRate(t *testing.T) {
	got, err := domain.EWMA([]domain.RealizedSettlement{
		settlement(t, baseAt, "0.00042"),
	}, 72*time.Hour)
	if err != nil {
		t.Fatalf("EWMA: %v", err)
	}
	if want := "0.00042"; got.Decimal() != want {
		t.Errorf("EWMA = %s; want %s", got.Decimal(), want)
	}
}

// A window of identical UNIT rates is exact regardless of the weights —
// Σw·1/Σw — so it isolates the normalization: a swapped numerator and
// denominator, or a weight leaking into only one of the sums, moves this.
func TestEWMA_UnitRatesNormalizeToOne(t *testing.T) {
	got, err := domain.EWMA([]domain.RealizedSettlement{
		settlement(t, baseAt.Add(-16*time.Hour), "1"),
		settlement(t, baseAt.Add(-8*time.Hour), "1"),
		settlement(t, baseAt, "1"),
	}, 8*time.Hour)
	if err != nil {
		t.Fatalf("EWMA: %v", err)
	}
	if want := "1"; got.Decimal() != want {
		t.Errorf("EWMA = %s; want %s", got.Decimal(), want)
	}
}

// Two settlements, distinct rates, the older one a full half-life back: the
// EWMA must land strictly BETWEEN the arithmetic mean and the newest rate.
// Below the mean kills "EWMA is the mean" (recency weighting is real); above
// the newest kills "EWMA is the last value" (history participates at all).
func TestEWMA_LandsBetweenMeanAndNewest(t *testing.T) {
	window := []domain.RealizedSettlement{
		settlement(t, baseAt.Add(-8*time.Hour), "0.00020"),
		settlement(t, baseAt, "0.00010"),
	}
	got, err := domain.EWMA(window, 8*time.Hour)
	if err != nil {
		t.Fatalf("EWMA: %v", err)
	}
	mean := mustRate(t, "0.00015")
	newest := mustRate(t, "0.00010")
	if c, err := got.Cmp(mean); err != nil || c >= 0 {
		t.Errorf("EWMA %s not strictly below the mean %s (cmp %d, %v)", got.Decimal(), mean.Decimal(), c, err)
	}
	if c, err := got.Cmp(newest); err != nil || c <= 0 {
		t.Errorf("EWMA %s not strictly above the newest %s (cmp %d, %v)", got.Decimal(), newest.Decimal(), c, err)
	}
}

// The same two rates further apart in time must sit CLOSER to the newest:
// more elapsed time is less weight on the old print. This is the decay
// DIRECTION — an inverted exponent passes everything above and fails here.
func TestEWMA_OlderHistoryWeighsLess(t *testing.T) {
	near, err := domain.EWMA([]domain.RealizedSettlement{
		settlement(t, baseAt.Add(-8*time.Hour), "0.00020"),
		settlement(t, baseAt, "0.00010"),
	}, 8*time.Hour)
	if err != nil {
		t.Fatalf("EWMA near: %v", err)
	}
	far, err := domain.EWMA([]domain.RealizedSettlement{
		settlement(t, baseAt.Add(-24*time.Hour), "0.00020"),
		settlement(t, baseAt, "0.00010"),
	}, 8*time.Hour)
	if err != nil {
		t.Fatalf("EWMA far: %v", err)
	}
	// Both are above 0.00010; the far window must be strictly closer to it.
	if c, err := far.Cmp(near); err != nil || c >= 0 {
		t.Errorf("far EWMA %s not strictly below near EWMA %s (cmp %d, %v)", far.Decimal(), near.Decimal(), c, err)
	}
}

// A longer half-life weights the old print MORE — the parameter's own
// direction, distinguishable from the age direction above.
func TestEWMA_LongerHalfLifeRemembersMore(t *testing.T) {
	window := func() []domain.RealizedSettlement {
		return []domain.RealizedSettlement{
			settlement(t, baseAt.Add(-24*time.Hour), "0.00020"),
			settlement(t, baseAt, "0.00010"),
		}
	}
	short, err := domain.EWMA(window(), 8*time.Hour)
	if err != nil {
		t.Fatalf("EWMA short: %v", err)
	}
	long, err := domain.EWMA(window(), 72*time.Hour)
	if err != nil {
		t.Fatalf("EWMA long: %v", err)
	}
	if c, err := long.Cmp(short); err != nil || c <= 0 {
		t.Errorf("72h EWMA %s not strictly above 8h EWMA %s (cmp %d, %v)", long.Decimal(), short.Decimal(), c, err)
	}
}

// The 31-half-life skip is a claim about scale-9 arithmetic, and the claim —
// not the skip's mechanics — is what gets pinned: a print that old carries a
// weight below half an ulp, so INCLUDING it and EXCLUDING it are the same
// program. The probe uses rate 1, the largest plausible magnitude, so this
// is the equivalence at its weakest point; and a print at 29 half-lives
// (weight ≈ 2 ulps) must still visibly participate, which is what stops the
// skip from firing early. The exact 30-vs-31 boundary is deliberately
// unpinned — at 30 half-lives the weight is a single ulp and the difference
// is sub-observable for every realistic rate, a documented near-equivalence.
func TestEWMA_AncientPrintsCannotMoveTheAnswer(t *testing.T) {
	const h = 8 * time.Hour

	without, err := domain.EWMA([]domain.RealizedSettlement{
		settlement(t, baseAt, "0.00010"),
	}, h)
	if err != nil {
		t.Fatalf("EWMA without: %v", err)
	}
	with, err := domain.EWMA([]domain.RealizedSettlement{
		settlement(t, baseAt.Add(-31*h), "1"),
		settlement(t, baseAt, "0.00010"),
	}, h)
	if err != nil {
		t.Fatalf("EWMA with: %v", err)
	}
	if c, err := with.Cmp(without); err != nil || c != 0 {
		t.Errorf("a 31-half-life-old print moved the EWMA: %s vs %s (cmp %d, %v)",
			with.Decimal(), without.Decimal(), c, err)
	}
}

func TestEWMA_OldButRepresentablePrintsParticipate(t *testing.T) {
	const h = 8 * time.Hour
	// Newest rate zero, ancient rate 1: the entire answer IS the ancient
	// print's contribution, so a skip firing at 29 half-lives would zero it.
	got, err := domain.EWMA([]domain.RealizedSettlement{
		settlement(t, baseAt.Add(-29*h), "1"),
		settlement(t, baseAt, "0"),
	}, h)
	if err != nil {
		t.Fatalf("EWMA: %v", err)
	}
	if sign, err := got.Sign(); err != nil || sign <= 0 {
		t.Errorf("EWMA with a 29-half-life-old print = %s; want strictly positive (sign %d, %v)",
			got.Decimal(), sign, err)
	}
}

func TestEWMA_Refusals(t *testing.T) {
	if _, err := domain.EWMA(nil, 8*time.Hour); !errors.Is(err, domain.ErrWindowEmpty) {
		t.Errorf("empty window = %v; want ErrWindowEmpty", err)
	}
	if _, err := domain.EWMA([]domain.RealizedSettlement{
		settlement(t, baseAt, "0.0001"),
	}, 0); !errors.Is(err, domain.ErrUnset) {
		t.Errorf("zero half-life = %v; want ErrUnset", err)
	}
	if _, err := domain.EWMA([]domain.RealizedSettlement{
		settlement(t, baseAt, "0.0001"),
		settlement(t, baseAt.Add(-8*time.Hour), "0.0002"),
	}, 8*time.Hour); !errors.Is(err, domain.ErrNotAscending) {
		t.Errorf("descending window = %v; want ErrNotAscending", err)
	}

	// Two settlements from two different perps in one window: an assembly
	// bug, refused as a leg mismatch rather than averaged across venues.
	mixed := []domain.RealizedSettlement{
		settlement(t, baseAt.Add(-8*time.Hour), "0.0002"),
		settlement(t, baseAt, "0.0001"),
	}
	mixed[1].Instrument = ids.VenueInstrument{
		Venue: "bybit", Product: ids.ProductLinearPerp,
		Symbol: "BTCUSDT", Contract: ids.ContractLinear,
	}
	if _, err := domain.EWMA(mixed, 8*time.Hour); !errors.Is(err, domain.ErrLegsMismatch) {
		t.Errorf("cross-venue window = %v; want ErrLegsMismatch", err)
	}
}
