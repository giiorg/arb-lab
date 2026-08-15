package pair

import (
	"errors"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// qtyStep is qty under the name the call sites read best: a lot GRID value
// rather than an order size. Same asset, because a step in one asset and a
// quantity in another cannot be compared and fixedpoint would refuse them.
func qtyStep(t *testing.T, s string) fixedpoint.Quantity {
	t.Helper()
	return qty(t, s)
}

// writeCrossVenueReferenceNoTick omits the `tick` key on both rows — the
// pre-tick row shape, which is NOT the same row as a declared "0" (the domain
// refuses that one at load, so a "0" fixture would test the domain's refusal
// and not this consumer's).
func writeCrossVenueReferenceNoTick(t *testing.T, observedAt time.Time) string {
	t.Helper()
	return writeCrossVenueReferenceTicks(t, observedAt, "1", "0.1", "", "")
}

// validConfigWithSteps is validConfig over a revision whose two sides carry
// DIFFERENT lot steps.
func validConfigWithSteps(t *testing.T, stepA, stepB string) Config {
	t.Helper()
	c := validConfig(t)
	c.Reference.Path = writeCrossVenueReference(t, timeAgo(t, time.Hour), stepA, stepB)
	return c
}

// perpOn is one side's instrument. The two sides trade the SAME SYMBOL, so the
// venue is the only component that tells the two keys apart — which is exactly
// why the per-side maps below must not hold each other's rows.
func perpOn(venue ids.Venue) ids.VenueInstrument {
	return ids.VenueInstrument{
		Venue: venue, Product: ids.ProductLinearPerp,
		Contract: ids.ContractLinear, Symbol: "LABUSDT",
	}
}

// TestTradingFiltersAreResolvedPerSide holds the shape the boot needs: one map
// per side, each holding ONLY that side's instruments.
//
// The fixture gives both sides a LABUSDT linear-perp row with DIFFERENT steps —
// "1" on side A and "0.1" on side B. Equal steps would make "resolved side A's
// row" and "resolved side B's row" the same observation, which is this repo's
// eleven-times shape: the fixture would not contain the thing under test.
func TestTradingFiltersAreResolvedPerSide(t *testing.T) {
	t.Parallel()
	c := validConfigWithSteps(t, "1", "0.1")
	wall := testWall(t)

	got, err := c.tradingFilters(wall)
	if err != nil {
		t.Fatalf("tradingFilters() = %v, want nil", err)
	}
	if len(got) != 2 {
		t.Fatalf("tradingFilters returned %d maps, want one per side", len(got))
	}
	a, b := perpOn("binance"), perpOn("bybit")
	if _, ok := got[0][b]; ok {
		t.Error("side A's map holds side B's instrument; each side's planner must verify against its own grid")
	}
	if _, ok := got[1][a]; ok {
		t.Error("side B's map holds side A's instrument")
	}
	if _, ok := got[0][a]; !ok {
		t.Fatal("side A's map is missing side A's own instrument")
	}
	if _, ok := got[1][b]; !ok {
		t.Fatal("side B's map is missing side B's own instrument")
	}
	eqQty(t, got[0][a].Step, qtyStep(t, "1"))
	eqQty(t, got[1][b].Step, qtyStep(t, "0.1"))
	// The observation instant is carried through, because the executor re-judges
	// the row's age at every package admission (§69) and a zero here would make
	// every package look infinitely stale.
	if got[0][a].ObservedAt.IsZero() {
		t.Error("ObservedAt is zero; the executor's per-package freshness re-judgement has nothing to read")
	}
}

func TestTradingFiltersRefuseAStaleRow(t *testing.T) {
	t.Parallel()
	c := validConfig(t)
	c.Reference.Path = writeCrossVenueReference(t, timeAgo(t, 48*time.Hour), "1", "0.1")
	c.Reference.MaxAge = dur(t, "24h")
	if _, err := c.tradingFilters(testWall(t)); !errors.Is(err, ErrFilterRowStale) {
		t.Fatalf("tradingFilters() = %v, want %v", err, ErrFilterRowStale)
	}
}

func TestTradingFiltersRefuseATicklessRow(t *testing.T) {
	t.Parallel()
	c := validConfig(t)
	c.Reference.Path = writeCrossVenueReferenceNoTick(t, timeAgo(t, time.Hour))
	if _, err := c.tradingFilters(testWall(t)); !errors.Is(err, ErrFilterTickUnrecorded) {
		t.Fatalf("tradingFilters() = %v, want %v", err, ErrFilterTickUnrecorded)
	}
}

// TestTradingFiltersRefuseAnUnreadableReference is the third refusal, and it is
// the one an operator meets first: a path that does not resolve must be named
// against the FILE, before either boot opens a journal or reaches a venue.
func TestTradingFiltersRefuseAnUnreadableReference(t *testing.T) {
	t.Parallel()
	c := validConfig(t)
	c.Reference.Path = c.Reference.Path + ".missing"
	if _, err := c.tradingFilters(testWall(t)); !errors.Is(err, ErrReferenceUnreadable) {
		t.Fatalf("tradingFilters() = %v, want %v", err, ErrReferenceUnreadable)
	}
}
