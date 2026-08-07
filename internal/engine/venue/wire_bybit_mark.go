package venue

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// This file decodes the MARK out of Bybit's v5 `tickers.<sym>` topic — the same
// frames wire_bybit.go reads for funding, read for §3.4's other data class.
//
// # Why this is a second decoder over one stream, and not a field on
// # BybitTickerState [M4 #207]
//
// Two reasons, and the second is the one that would not have been guessed.
//
// FIRST, the emission rules are opposites. BybitTickerState emits "exactly when
// the message is a snapshot or a delta carrying at least one funding field",
// because most deltas are bid/ask churn and re-emitting an unchanged rate ~10/s
// "would round-trip the full two-hop dispatch for nothing". Those same ~10/s
// deltas ARE the mark's news: §7.1's liq distance "is a function of mark, which
// moves continuously with no account mutation", and a mark gated on funding
// news would refresh about once a minute. That is §7.1's own stated defect
// reached one class over — the trigger frozen behind a slower event's cadence,
// with "a pure rally... undetected until liquidation".
//
// SECOND, the mark needs no accumulator at all, so giving it one would be
// inventing state. wire_bybit.go's state machine exists because funding is FOUR
// fields arriving at different times, so "a stateless decoder therefore cannot
// exist — the accumulated last-known state IS the reading". A mark is ONE
// field, and one field is a complete reading: there is nothing to carry across
// frames, no snapshot/delta distinction that changes the answer, and no cold
// start to wait out. Folding it in would also have forced asset tags onto every
// existing funding consumer — replay, crossvenue, sim — for a class none of
// them reads, which is how a design pays for a coupling it did not need.
//
// The result is that both venues' mark decoders are pure functions over one
// frame taking the assets their price needs. The asymmetry between the venues
// is funding's, not the mark's.

// bybitTickerMarkWire is the frame envelope read for its mark. It restates
// bybitTickerWire's keys rather than sharing the struct, on the same rule
// binanceMarkWire follows: each decoder requires exactly the keys its own data
// class cannot do without, and a shared struct makes a field mandatory for both
// the moment either needs it.
type bybitTickerMarkWire struct {
	Topic string          `json:"topic"`
	Type  string          `json:"type"`
	Op    string          `json:"op"`
	TS    int64           `json:"ts"`
	Data  json.RawMessage `json:"data"`
}

type bybitTickerMarkDataWire struct {
	// MarkPrice is the reading. indexPrice and lastPrice ride the same object
	// and are deliberately not decoded: §7.1 reads the MARK, because the
	// venue's mark is the number its own liquidation engine uses, and a field
	// nobody reads is an unchecked claim wearing a type annotation.
	MarkPrice string `json:"markPrice"`
}

// BybitMark is one mark reading and the instant the venue stamped on the
// message that produced it (`ts`, epoch ms) — the envelope's SourceTime,
// surfaced because the payload is the only place Bybit states it.
type BybitMark struct {
	Mark       Mark
	SourceTime time.Time
}

// DecodeBybitMark reads one ticker frame as a mark reading.
//
// The bool reports whether the frame carried a mark. False is the ordinary
// case, not an error: it covers transport chatter (an `op` acknowledgement) and
// every delta whose data object states no markPrice. Emitting on presence alone
// is what keeps this honest — a decoder that re-emitted a held mark on a
// mark-less frame would be publishing a reading nothing observed, which is
// precisely the fabrication §3.4's silence timer exists to report instead.
//
// It takes the assets for the reason every price-producing decoder here does:
// fixedpoint.Price carries base and quote so a size in one asset at a price in
// another is an error rather than a plausible number, and splitting a venue
// symbol to guess them is §5.2's SymbolSpec's job — "a wrong split mints money
// whose asset tag is a lie".
//
// # A snapshot is not required first, and that is the mark's difference
//
// BybitTickerState refuses to emit before a snapshot because its reading is
// accumulated and a partial accumulation is a wrong reading. Nothing here is
// accumulated, so a markPrice on a mid-stream delta is a complete, correct mark
// — and waiting for a snapshot would blind §7.1's trigger for the whole window
// after a reconnect, on a rule that protects a property this class does not
// have.
func DecodeBybitMark(raw []byte, base, quote fixedpoint.Asset) (BybitMark, bool, error) {
	var w bybitTickerMarkWire
	if err := json.Unmarshal(raw, &w); err != nil {
		return BybitMark{}, false, fmt.Errorf("venue: bybit mark frame: %w: %v", ErrWireUndecodable, err)
	}

	// The three framing refusals are BybitTickerState.Apply's, restated rather
	// than shared because they are cheap and because a decoder that trusted
	// another decoder to have rejected a frame would be correct only while both
	// are called on every frame — which is a wiring property, not a local one.
	if w.Topic == "" {
		if w.Op != "" {
			return BybitMark{}, false, nil
		}
		return BybitMark{}, false, fmt.Errorf("venue: bybit frame with no topic and no op: %w", ErrWireForeign)
	}
	if w.Type != "snapshot" && w.Type != "delta" {
		return BybitMark{}, false, fmt.Errorf("venue: bybit ticker type %q: %w", w.Type, ErrWireForeign)
	}
	if w.TS <= 0 {
		return BybitMark{}, false, fmt.Errorf("venue: bybit ticker ts=%d: %w", w.TS, ErrWireIncomplete)
	}

	var d bybitTickerMarkDataWire
	if err := json.Unmarshal(w.Data, &d); err != nil {
		return BybitMark{}, false, fmt.Errorf("venue: bybit ticker data: %w: %v", ErrWireUndecodable, err)
	}

	// A SNAPSHOT with no markPrice is refused; a DELTA with none is silence.
	// The split is the venue's own claim about the message: a snapshot states
	// full ticker state, so a perp snapshot without a mark is a stream this
	// seam has not been taught (a pre-listing symbol, a spot topic misrouted
	// here) and someone must look — the same argument wire_bybit.go makes for
	// funding's four fields. A delta states only what changed, so a mark-less
	// one is the ordinary majority of the stream.
	if d.MarkPrice == "" {
		if w.Type == "snapshot" {
			return BybitMark{}, false, fmt.Errorf(
				"venue: bybit ticker snapshot states no markPrice: %w", ErrWireIncomplete)
		}
		return BybitMark{}, false, nil
	}

	price, err := fixedpoint.ParsePrice(base, quote, d.MarkPrice)
	if err != nil {
		return BybitMark{}, false, fmt.Errorf("venue: bybit markPrice %q: %w", d.MarkPrice, err)
	}
	m := Mark{Price: price}
	if err := m.Validate(); err != nil {
		return BybitMark{}, false, fmt.Errorf("venue: bybit mark: %w", err)
	}
	return BybitMark{Mark: m, SourceTime: time.UnixMilli(w.TS).UTC()}, true, nil
}
