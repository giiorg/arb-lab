package driven_test

// Shared fixtures for the model-adapter batteries (basis, cost). Shapes obey
// the fixture-collapse checklist: books are ASYMMETRIC between sides (the
// worse-side selection must be a real choice), fee-free numbers differ per
// leg, and every book fixture has MORE THAN ONE level so "best" and "the
// walk" are distinct questions.

import (
	"testing"
	"time"

	mdomain "github.com/giiorg/arb-lab/internal/marketdata/domain"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/strategy/domain"
)

var (
	modelBase = time.Date(2026, 7, 31, 9, 0, 0, 0, time.UTC)

	mSpotInst = ids.VenueInstrument{Venue: "binance", Product: ids.ProductSpot, Symbol: "BTCUSDT"}
	mPerpInst = ids.VenueInstrument{
		Venue: "binance", Product: ids.ProductLinearPerp,
		Symbol: "BTCUSDT", Contract: ids.ContractLinear,
	}
)

func mustRate(t *testing.T, s string) fixedpoint.Rate {
	t.Helper()
	r, err := fixedpoint.ParseRate(s)
	if err != nil {
		t.Fatalf("ParseRate(%q): %v", s, err)
	}
	return r
}

func mustQty(t *testing.T, s string) fixedpoint.Quantity {
	t.Helper()
	q, err := fixedpoint.ParseQuantity("BTC", s)
	if err != nil {
		t.Fatalf("ParseQuantity(%q): %v", s, err)
	}
	return q
}

func mustPrice(t *testing.T, s string) fixedpoint.Price {
	t.Helper()
	p, err := fixedpoint.ParsePrice("BTC", "USDT", s)
	if err != nil {
		t.Fatalf("ParsePrice(%q): %v", s, err)
	}
	return p
}

func mustMoney(t *testing.T, s string) fixedpoint.Money {
	t.Helper()
	m, err := fixedpoint.ParseMoney("USDT", s)
	if err != nil {
		t.Fatalf("ParseMoney(%q): %v", s, err)
	}
	return m
}

func modelCalibration(name string) domain.CalibrationRef {
	return domain.CalibrationRef{
		ID:   domain.FixturePrefix + name,
		Hash: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
	}
}

func modelWatermark(at time.Time, seq uint64) mdomain.Watermark {
	return mdomain.Watermark{VenueSeq: seq, SourceTime: at.Add(-30 * time.Millisecond), RecvTime: at}
}

func modelView(t *testing.T) domain.MarketView {
	t.Helper()
	return domain.MarketView{
		Spot: domain.PriceView{
			Instrument: mSpotInst,
			Bid:        mustPrice(t, "64000.00"),
			Ask:        mustPrice(t, "64000.50"),
			Watermark:  modelWatermark(modelBase, 101),
		},
		Perp: domain.PriceView{
			Instrument: mPerpInst,
			Bid:        mustPrice(t, "64012.00"),
			Ask:        mustPrice(t, "64012.50"),
			Watermark:  modelWatermark(modelBase, 102),
		},
		Funding: mdomain.FundingObservation{
			Instrument: mPerpInst,
			Rate:       mustRate(t, "0.00011"),
			Interval:   8 * time.Hour,
			NextSettle: modelBase.Add(3 * time.Hour),
			Cap:        mustRate(t, "0.0075"),
			Floor:      mustRate(t, "-0.0075"),
			Watermark:  modelWatermark(modelBase, 103),
		},
		Realized: []domain.RealizedSettlement{
			{Instrument: mPerpInst, At: modelBase.Add(-13 * time.Hour), Rate: mustRate(t, "0.00008"), Watermark: modelWatermark(modelBase.Add(-13*time.Hour), 61)},
			{Instrument: mPerpInst, At: modelBase.Add(-5 * time.Hour), Rate: mustRate(t, "0.00013"), Watermark: modelWatermark(modelBase.Add(-5*time.Hour), 73)},
		},
	}
}

func modelPlan(t *testing.T) domain.HoldPlanRevision {
	t.Helper()
	return domain.HoldPlanRevision{
		Num:     1,
		Trigger: domain.TriggerEntry,
		At:      modelBase,
		Plan: domain.HoldPlan{
			Settlements: []time.Time{
				modelBase.Add(3 * time.Hour),
				modelBase.Add(11 * time.Hour),
			},
			Terminal: modelBase.Add(20 * time.Hour),
		},
	}
}

// bookFor builds a two-level book. Levels are (price, qty) pairs best-first;
// the caller states both sides explicitly, so no fixture is accidentally
// symmetric.
func bookFor(t *testing.T, inst ids.VenueInstrument, bids, asks [][2]string) mdomain.BookSnapshot {
	t.Helper()
	mk := func(rows [][2]string) []mdomain.BookLevel {
		out := make([]mdomain.BookLevel, len(rows))
		for i, r := range rows {
			out[i] = mdomain.BookLevel{Price: mustPrice(t, r[0]), Quantity: mustQty(t, r[1])}
		}
		return out
	}
	return mdomain.BookSnapshot{
		Instrument: inst,
		Bids:       mk(bids),
		Asks:       mk(asks),
		Watermark:  modelWatermark(modelBase, 200),
	}
}

// mapBooks is the test BookSource: a map plus a presence bool.
type mapBooks map[ids.VenueInstrument]mdomain.BookSnapshot

func (m mapBooks) Snapshot(inst ids.VenueInstrument) (mdomain.BookSnapshot, bool) {
	b, ok := m[inst]
	return b, ok
}
