package venue

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// This file decodes Binance's partial-book-depth wire: the spot
// `<sym>@depth20@100ms` frame and the futures frame of the same topic name.
// They are two dialects, not one lenient shape — the spot frame is a bare
// `{lastUpdateId, bids, asks}` snapshot with NO venue timestamp anywhere in
// the payload, while the futures frame wraps the same twenty levels in a
// `depthUpdate` event envelope carrying the venue's own event time. A single
// tolerant decoder would silently accept a misfiled file from either
// product, which is exactly the wiring fact ErrWireForeign exists to name.
//
// Decoders live here for wire_binance.go's stated reason: the capture path
// and any future live client are transports — what varies is HOW bytes
// arrive, what must not vary is what they mean.

// BinanceBookSnapshot is one decoded partial-depth frame: the venue's top
// twenty levels, best-first on both sides per Binance's published ordering
// (bids descending, asks ascending — copied, not re-sorted; a venue that
// changes its order must fail the domain's ordering check, not be silently
// repaired here).
type BinanceBookSnapshot struct {
	Book BookSnapshot

	// LastUpdateID is the venue's book position for this frame — the spot
	// frame's `lastUpdateId`, the futures frame's final update id `u`. It
	// is the snapshot's VenueSeq: zero is a decoder that dropped the
	// field, refused at decode.
	LastUpdateID uint64

	// EventTime is the venue's own stamp (`E`, epoch ms) — present on the
	// FUTURES dialect only. The spot partial-depth payload carries no
	// venue time at all, so the spot decoder leaves it zero and the
	// consumer's time anchor is the capture envelope's RecvTime; a
	// consumer that needs venue time on spot books is asking the wire for
	// a fact it does not carry.
	EventTime time.Time
}

// binanceSpotDepthWire is the spot partial-depth frame as Binance spells
// it: no event token, no timestamp, levels as [price, qty] string pairs.
type binanceSpotDepthWire struct {
	LastUpdateID uint64      `json:"lastUpdateId"`
	Bids         [][2]string `json:"bids"`
	Asks         [][2]string `json:"asks"`
}

// binanceFuturesDepthWire is the futures partial-depth frame: the
// depthUpdate event envelope around the same level arrays.
type binanceFuturesDepthWire struct {
	Event     string      `json:"e"`
	EventTime int64       `json:"E"`
	Symbol    string      `json:"s"`
	FinalID   uint64      `json:"u"`
	Bids      [][2]string `json:"b"`
	Asks      [][2]string `json:"a"`
}

// DecodeBinanceSpotDepth20 decodes one spot partial-depth frame.
//
// A payload with a zero lastUpdateId is refused as FOREIGN rather than
// incomplete: every real spot depth frame carries one, so its absence
// means the bytes are some other stream's — the depthUpdate dialect lands
// here when a futures file is misfiled as spot, and it decodes to zero
// because its position field is spelled `u`.
func DecodeBinanceSpotDepth20(raw []byte, base, quote fixedpoint.Asset) (BinanceBookSnapshot, error) {
	var w binanceSpotDepthWire
	if err := json.Unmarshal(raw, &w); err != nil {
		return BinanceBookSnapshot{}, fmt.Errorf("venue: binance spot depth frame: %w: %v", ErrWireUndecodable, err)
	}
	if w.LastUpdateID == 0 {
		return BinanceBookSnapshot{}, fmt.Errorf("venue: binance spot depth lastUpdateId=0: %w", ErrWireForeign)
	}
	book, err := decodeBookLevels(w.Bids, w.Asks, base, quote)
	if err != nil {
		return BinanceBookSnapshot{}, err
	}
	return BinanceBookSnapshot{Book: book, LastUpdateID: w.LastUpdateID}, nil
}

// DecodeBinanceFuturesDepth20 decodes one futures partial-depth frame.
//
// The event token must be "depthUpdate" — anything else is a misfiled file
// or a mis-subscribed topic, refused as foreign for wire_binance.go's
// stated reason.
func DecodeBinanceFuturesDepth20(raw []byte, base, quote fixedpoint.Asset) (BinanceBookSnapshot, error) {
	var w binanceFuturesDepthWire
	if err := json.Unmarshal(raw, &w); err != nil {
		return BinanceBookSnapshot{}, fmt.Errorf("venue: binance futures depth frame: %w: %v", ErrWireUndecodable, err)
	}
	if w.Event != "depthUpdate" {
		return BinanceBookSnapshot{}, fmt.Errorf("venue: binance event %q: %w", w.Event, ErrWireForeign)
	}
	if w.Symbol == "" {
		return BinanceBookSnapshot{}, fmt.Errorf("venue: binance futures depth: symbol: %w", ErrWireIncomplete)
	}
	if w.FinalID == 0 {
		return BinanceBookSnapshot{}, fmt.Errorf("venue: binance futures depth u=0: %w", ErrWireIncomplete)
	}
	if w.EventTime <= 0 {
		return BinanceBookSnapshot{}, fmt.Errorf("venue: binance futures depth E=%d: %w", w.EventTime, ErrWireIncomplete)
	}
	book, err := decodeBookLevels(w.Bids, w.Asks, base, quote)
	if err != nil {
		return BinanceBookSnapshot{}, err
	}
	return BinanceBookSnapshot{
		Book:         book,
		LastUpdateID: w.FinalID,
		EventTime:    time.UnixMilli(w.EventTime).UTC(),
	}, nil
}

// decodeBookLevels parses both sides' [price, qty] string pairs. An empty
// side is legal (BookSnapshot's halted-instrument argument); a level that
// parses but could not rest in a book — zero quantity in a snapshot — is
// BookLevel.Validate's refusal, surfaced with its side and index.
func decodeBookLevels(bids, asks [][2]string, base, quote fixedpoint.Asset) (BookSnapshot, error) {
	parse := func(side BookSide, pairs [][2]string) ([]BookLevel, error) {
		levels := make([]BookLevel, 0, len(pairs))
		for i, p := range pairs {
			price, err := fixedpoint.ParsePrice(base, quote, p[0])
			if err != nil {
				return nil, fmt.Errorf("venue: binance depth %s level %d price %q: %w", side, i, p[0], err)
			}
			qty, err := fixedpoint.ParseQuantity(base, p[1])
			if err != nil {
				return nil, fmt.Errorf("venue: binance depth %s level %d qty %q: %w", side, i, p[1], err)
			}
			l := BookLevel{Price: price, Quantity: qty}
			if err := l.Validate(); err != nil {
				return nil, fmt.Errorf("venue: binance depth %s level %d: %w", side, i, err)
			}
			levels = append(levels, l)
		}
		return levels, nil
	}
	b, err := parse(SideBid, bids)
	if err != nil {
		return BookSnapshot{}, err
	}
	a, err := parse(SideAsk, asks)
	if err != nil {
		return BookSnapshot{}, err
	}
	book := BookSnapshot{Bids: b, Asks: a}
	if err := book.Validate(); err != nil {
		return BookSnapshot{}, fmt.Errorf("venue: binance depth snapshot: %w", err)
	}
	return book, nil
}
