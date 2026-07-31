package state

import (
	"github.com/giiorg/arb-lab/internal/marketdata/domain"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	sports "github.com/giiorg/arb-lab/internal/strategy/application/ports"
	sdomain "github.com/giiorg/arb-lab/internal/strategy/domain"
)

// StrategyReader is the strategy context's MarketStateReader, implemented by
// translation over the Cache — the deferral the package doc carried, resolved
// by its named trigger: the first consumer-owned read port exists now, and
// this file is the translation it deferred.
//
// # The translation is a projection, and it stays one
//
// What crosses the seam is the funding observation as accepted, and the top
// of book as a PriceView — bid and ask, never a collapsed mid, because which
// side of the spread a term is priced against is the consumer's conservative
// judgement (§40c). No freshness verdict crosses: the entries carry
// watermarks, and re-judgement happens at the consumer's read against an
// instant its caller supplies, which is the package doc's structural rule
// arriving at its first consumer.
//
// # Absence is false, not an error, and a topless book is absent
//
// An instrument with no cache entry yet — boot, or a symbol nothing
// publishes — answers false, and the strategy service turns that into a
// journaled refusal Decision. A book with an EMPTY SIDE answers false too: a
// top of book with no resting ask is not a wide market, it is half a market,
// and handing the consumer a PriceView with an unset price would put an
// invalid value behind a true.
type StrategyReader struct {
	c *Cache
}

// The §3.6 rule 2 assertion the port requires of its adapter. Breaking
// conformance is a NOBUILD, which for this line means CAUGHT.
var _ sports.MarketStateReader = StrategyReader{}

// NewStrategyReader wraps the cache. A nil cache answers nothing and is a
// wiring bug; it panics here at the composition root rather than at the
// first read on the decision path.
func NewStrategyReader(c *Cache) StrategyReader {
	if c == nil {
		panic("state: NewStrategyReader over a nil cache")
	}
	return StrategyReader{c: c}
}

// Funding is the funding face: the accepted observation, verbatim.
func (r StrategyReader) Funding(vi ids.VenueInstrument) (domain.FundingObservation, bool) {
	e, ok := r.c.Funding(vi)
	if !ok {
		return domain.FundingObservation{}, false
	}
	return e.Observation, true
}

// TopOfBook is the price face: best bid and ask with the book's watermark.
func (r StrategyReader) TopOfBook(vi ids.VenueInstrument) (sdomain.PriceView, bool) {
	e, ok := r.c.Book(vi)
	if !ok {
		return sdomain.PriceView{}, false
	}
	bid, ok := e.Book.BestBid()
	if !ok {
		return sdomain.PriceView{}, false
	}
	ask, ok := e.Book.BestAsk()
	if !ok {
		return sdomain.PriceView{}, false
	}
	return sdomain.PriceView{
		Instrument: vi,
		Bid:        bid.Price,
		Ask:        ask.Price,
		Watermark:  e.Book.Watermark,
	}, true
}
