package crossvenue

// The book-cost walkers (§45e: costs are measured, per symbol, from our
// books — no BTC figure transfers). Each venue's walker samples its book
// stream on the same epoch grid the decision points use and computes the
// standing depth-walked slippage for the committed candidate size, both
// sides, at every sampled instant. Two things come out of one walk:
//
//   - the DISTRIBUTION of walked slips per side, which the assembly reads
//     the p90 of (conditional on this size, this symbol, this venue);
//   - the walked slips AT the wanted instants (episode boundaries), the
//     "this order" half of WorseOf.
//
// A sampled instant whose book cannot fill the candidate is DepthShort:
// excluded from the distribution and counted (§41b's rule — a silent zero
// would put the thinnest books at the flattering end). A wanted instant
// with no measurable book carries its reason out, so the assembly can
// refuse the episode by name rather than default a cost.

import (
	"fmt"
	"time"

	"errors"

	"github.com/giiorg/arb-lab/internal/engine/capture"
	"github.com/giiorg/arb-lab/internal/engine/venue"
	mdomain "github.com/giiorg/arb-lab/internal/marketdata/domain"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	dcv "github.com/giiorg/arb-lab/internal/strategy/domain/crossvenue"
)

// BoundarySample is one wanted instant's walked slips. OK is true only
// when both sides walked; otherwise Reason names the first refusal, and
// the assembly's episode refusal repeats it.
type BoundarySample struct {
	Bid, Ask fixedpoint.Rate
	OK       bool
	Reason   string
}

// CostSeries is one stream's walked-cost evidence.
type CostSeries struct {
	// BidSlips and AskSlips are the walked-slippage distributions across
	// sampled instants, observation order.
	BidSlips []fixedpoint.Rate
	AskSlips []fixedpoint.Rate

	// AtWanted maps a wanted instant (UnixMilli) to its boundary sample.
	// Every wanted instant the walk covered appears; a wanted instant
	// with no fresh book appears with OK=false.
	AtWanted map[int64]BoundarySample

	// Frames counts decoded data frames. Skipped counts grid instants
	// with no fresh (or no anchored) book. DepthShort counts per-side
	// distribution exclusions.
	Frames     int
	Skipped    int
	DepthShort int

	// Accumulator counters, zero on the snapshot-wire venue.
	Breaks        int
	ColdDeltas    int
	DeletesAbsent int
}

// costCollector owns the grid sampling and the walk arithmetic shared by
// both venue walkers; the venue loop feeds it the current book.
type costCollector struct {
	p      SampleParams
	qty    fixedpoint.Quantity
	wanted map[int64]bool

	out      CostSeries
	lastGrid time.Time
}

func newCostCollector(p SampleParams, qty fixedpoint.Quantity, wanted []time.Time) *costCollector {
	w := make(map[int64]bool, len(wanted))
	for _, t := range wanted {
		w[t.UnixMilli()] = true
	}
	return &costCollector{p: p, qty: qty, wanted: w,
		out: CostSeries{AtWanted: map[int64]BoundarySample{}}}
}

// observe advances the grid to just before recv, sampling the held book —
// nil when cold or dropped — at each instant. Errors out of the walk that
// are not depth refusals propagate.
func (c *costCollector) observe(recv time.Time, book func(at time.Time) (mdomain.BookSnapshot, bool)) error {
	var walkErr error
	c.lastGrid = sampleGrid(recv, c.lastGrid, c.p, func(t time.Time) {
		if walkErr != nil {
			return
		}
		b, ok := book(t)
		if !ok {
			c.out.Skipped++
			if c.wanted[t.UnixMilli()] {
				c.out.AtWanted[t.UnixMilli()] = BoundarySample{Reason: "no fresh book at boundary"}
			}
			return
		}
		walkErr = c.walkAt(t, b)
	})
	return walkErr
}

func (c *costCollector) walkAt(t time.Time, b mdomain.BookSnapshot) error {
	var bs BoundarySample
	sides := []struct {
		side mdomain.BookSide
		dst  *[]fixedpoint.Rate
		set  func(fixedpoint.Rate)
	}{
		{mdomain.SideBid, &c.out.BidSlips, func(r fixedpoint.Rate) { bs.Bid = r }},
		{mdomain.SideAsk, &c.out.AskSlips, func(r fixedpoint.Rate) { bs.Ask = r }},
	}
	walked := 0
	for _, s := range sides {
		slip, err := dcv.WalkedFill(b, s.side, c.qty)
		if err != nil {
			if errors.Is(err, dcv.ErrDepthShort) {
				c.out.DepthShort++
				if bs.Reason == "" {
					bs.Reason = fmt.Sprintf("%s: %v", s.side, err)
				}
				continue
			}
			return fmt.Errorf("walk at %s: %w", t.Format(time.RFC3339Nano), err)
		}
		*s.dst = append(*s.dst, slip)
		s.set(slip)
		walked++
	}
	if c.wanted[t.UnixMilli()] {
		bs.OK = walked == 2
		c.out.AtWanted[t.UnixMilli()] = bs
	}
	return nil
}

// WalkBinanceBookCosts walks the futures depth20 stream: every frame is a
// full snapshot, so the held book is simply the last frame, required
// fresh within Slack at each grid instant.
func WalkBinanceBookCosts(root string, days []ManifestDay, key capture.StreamKey, inst ids.VenueInstrument,
	p SampleParams, qty fixedpoint.Quantity, wanted []time.Time) (CostSeries, error) {
	if err := p.Validate(); err != nil {
		return CostSeries{}, err
	}
	base, quote, aerr := usdtPairAssets(inst.Symbol)
	if aerr != nil {
		return CostSeries{}, fmt.Errorf("binance book walk: %v", aerr)
	}
	col := newCostCollector(p, qty, wanted)

	var cur *mdomain.BookSnapshot
	var curRecv time.Time

	err := scanStream(root, key, days, func(cont bool) {
		if !cont {
			cur = nil
		}
	}, func(rec capture.Record) error {
		switch rec.Kind {
		case capture.RecordKindGap:
			cur = nil
			return nil
		case capture.RecordKindData:
		case capture.RecordKindUnspecified, capture.RecordKindOpen, capture.RecordKindAlive,
			capture.RecordKindClose, capture.RecordKindRest, capture.RecordKindClockStep:
			// Framing and liveness records carry no frame to decode.
			return nil
		}
		decoded, derr := venue.DecodeBinanceFuturesDepth20(rec.Payload, base, quote)
		if derr != nil {
			return fmt.Errorf("%s at %s: %w", key, rec.RecvTime.Format(time.RFC3339Nano), derr)
		}
		col.out.Frames++

		if err := col.observe(rec.RecvTime, func(at time.Time) (mdomain.BookSnapshot, bool) {
			if cur == nil || at.Sub(curRecv) > p.Slack {
				return mdomain.BookSnapshot{}, false
			}
			return *cur, true
		}); err != nil {
			return err
		}

		book := mdomain.BookSnapshot{
			Instrument: inst,
			Bids:       toMarketLevels(decoded.Book.Bids),
			Asks:       toMarketLevels(decoded.Book.Asks),
			Watermark: mdomain.Watermark{
				SourceTime: rec.RecvTime,
				RecvTime:   rec.RecvTime,
				VenueSeq:   decoded.LastUpdateID,
			},
		}
		cur, curRecv = &book, rec.RecvTime
		return nil
	})
	if err != nil {
		return CostSeries{}, err
	}
	return col.out, nil
}

// WalkBybitBookCosts walks the orderbook.50 stream through the
// accumulator: the held book is the reconstruction, anchored by
// snapshots, advanced by contiguous deltas, dropped at breaks, gaps and
// file boundaries. Freshness at a grid instant is the last absorbed
// frame's RecvTime — any frame maintains the book, so any frame renews
// the anchor.
func WalkBybitBookCosts(root string, days []ManifestDay, key capture.StreamKey, inst ids.VenueInstrument,
	p SampleParams, qty fixedpoint.Quantity, wanted []time.Time) (CostSeries, error) {
	if err := p.Validate(); err != nil {
		return CostSeries{}, err
	}
	base, quote, aerr := usdtPairAssets(inst.Symbol)
	if aerr != nil {
		return CostSeries{}, fmt.Errorf("bybit book walk: %v", aerr)
	}
	col := newCostCollector(p, qty, wanted)

	acc := NewBookAccumulator()
	var lastRecv time.Time

	err := scanStream(root, key, days, func(cont bool) {
		if !cont {
			acc.Drop()
		}
	}, func(rec capture.Record) error {
		switch rec.Kind {
		case capture.RecordKindGap:
			acc.Drop()
			return nil
		case capture.RecordKindData:
		case capture.RecordKindUnspecified, capture.RecordKindOpen, capture.RecordKindAlive,
			capture.RecordKindClose, capture.RecordKindRest, capture.RecordKindClockStep:
			// Framing and liveness records carry no frame to decode.
			return nil
		}
		msg, emitted, derr := venue.DecodeBybitOrderbook(rec.Payload, base, quote)
		if derr != nil {
			return fmt.Errorf("%s at %s: %w", key, rec.RecvTime.Format(time.RFC3339Nano), derr)
		}
		if !emitted {
			return nil
		}
		col.out.Frames++

		if err := col.observe(rec.RecvTime, func(at time.Time) (mdomain.BookSnapshot, bool) {
			raw, ok := acc.Sample()
			if !ok || at.Sub(lastRecv) > p.Slack {
				return mdomain.BookSnapshot{}, false
			}
			return mdomain.BookSnapshot{
				Instrument: inst,
				Bids:       toMarketLevels(raw.Bids),
				Asks:       toMarketLevels(raw.Asks),
				Watermark: mdomain.Watermark{
					SourceTime: lastRecv,
					RecvTime:   lastRecv,
					VenueSeq:   acc.UpdateID(),
				},
			}, true
		}); err != nil {
			return err
		}

		acc.Apply(msg)
		lastRecv = rec.RecvTime
		return nil
	})
	if err != nil {
		return CostSeries{}, err
	}
	st := acc.Stats()
	col.out.Breaks = st.Breaks
	col.out.ColdDeltas = st.ColdDeltas
	col.out.DeletesAbsent = st.DeletesAbsent
	return col.out, nil
}

// toMarketLevels translates venue wire levels into the market domain's —
// field-identical by design; the translation is the seam where a venue
// shape change becomes a compile error (family2's stated rule, restated
// here because the two packages cannot share the unexported helper).
func toMarketLevels(levels []venue.BookLevel) []mdomain.BookLevel {
	out := make([]mdomain.BookLevel, len(levels))
	for i, l := range levels {
		out[i] = mdomain.BookLevel{Price: l.Price, Quantity: l.Quantity}
	}
	return out
}
