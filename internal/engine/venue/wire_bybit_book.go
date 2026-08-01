package venue

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// This file decodes Bybit's book wire: the v5 `orderbook.<depth>.<sym>` topic,
// both products (the spot and linear frames share one shape — Profiles §5's
// own-archive observation, 2026-08-01).
//
// # Stateless, unlike the ticker — because the wire is
//
// BybitTickerState exists because the ticker's deltas omit unchanged fields,
// so the accumulated state IS the reading. The book wire has no absent-field
// semantics: a snapshot is the whole book and a delta is a complete
// instruction list, each frame decodable on its own bytes. The accumulation a
// book DOES need — applying deltas to a running snapshot, judging whether a
// delta follows — is reconstruction, a consumer judgement, not venue wire
// mechanics; it lives with the consumer (marketdata's domain for the live
// pipeline, the crossvenue walker for the archive), never here.
//
// # One decoder for both products, and that is a measured fact, not lenience
//
// The Binance book file splits spot and futures into two decoders because the
// venue publishes two dialects. Bybit publishes one: the envelope, the data
// object and every value type were observed identical across a linear alt
// stream and the BTCUSDT spot stream (Profiles §5). A product split here
// would be two copies of one shape, drifting.

// BybitBookKind says which of the two message kinds a frame carried, in the
// venue's own vocabulary. Produced only by DecodeBybitOrderbook from a checked
// type word, so it carries no Validate — the decoder is the gate, the same
// absence BinanceBookSnapshot records.
type BybitBookKind string

const (
	// BybitBookSnapshotKind is a full-book restatement: reset the local book
	// (Profiles §5: "a new snapshot means reset").
	BybitBookSnapshotKind BybitBookKind = "snapshot"

	// BybitBookDeltaKind is an incremental instruction list.
	BybitBookDeltaKind BybitBookKind = "delta"
)

// BybitBookMessage is one decoded orderbook frame.
//
// Exactly one of Snapshot/Delta is meaningful, named by Kind. UpdateID is the
// venue's `u` — the single-sequence position, "resets to 1 during service
// restarts" (Profiles §5), so monotonicity is the CONSUMER's judgement with
// the restart caveat in hand; a decoder that judged it would have to hold
// state this file just argued away. Seq is the venue's cross sequence,
// carried as evidence because the venue states it on every frame; nothing
// here interprets it. SourceTime is the envelope's `ts`. The frame's `cts`
// (matching-engine time) is deliberately unread: its semantics are not yet a
// verified Profiles fact, and carrying a timestamp whose meaning is guessed
// is the units trap with a clock.
type BybitBookMessage struct {
	Kind       BybitBookKind
	Snapshot   BookSnapshot
	Delta      BookDelta
	UpdateID   uint64
	Seq        uint64
	SourceTime time.Time
}

// bybitBookWire is the frame envelope; bybitBookDataWire the data object.
// `u` and `seq` are JSON NUMBERS — unlike the ticker topic, whose funding
// fields are all strings (Profiles §5, own-archive observation) — and the
// level arrays are [price, size] STRING pairs. The five data keys share no
// case-insensitive collision, so encoding/json's fallback matching has
// nothing to misroute here (the trap the Binance futures wire had to declare
// FirstID against).
type bybitBookWire struct {
	Topic string          `json:"topic"`
	Type  string          `json:"type"`
	Op    string          `json:"op"`
	TS    int64           `json:"ts"`
	Data  json.RawMessage `json:"data"`
}

type bybitBookDataWire struct {
	Symbol string      `json:"s"`
	Bids   [][2]string `json:"b"`
	Asks   [][2]string `json:"a"`
	Update uint64      `json:"u"`
	Seq    uint64      `json:"seq"`
}

// DecodeBybitOrderbook decodes one raw frame. The bool reports whether a
// message was decoded; false covers transport chatter (an `op`
// acknowledgement riding the same socket, exactly as on the ticker stream).
//
// The foreign/incomplete boundary follows the errors.go vocabulary: a frame
// whose topic is not an orderbook topic is FOREIGN (the reader goes to the
// wiring that routed a ticker file here), while an orderbook frame missing a
// field the venue always states — `u`, `seq`, `ts`, the symbol — is
// INCOMPLETE (the reader goes to the Profiles entry). `u` is refused at zero
// because the venue's own floor is 1 ("resets to 1", never 0), so zero is a
// dropped field, not a restart.
func DecodeBybitOrderbook(raw []byte, base, quote fixedpoint.Asset) (BybitBookMessage, bool, error) {
	var w bybitBookWire
	if err := json.Unmarshal(raw, &w); err != nil {
		return BybitBookMessage{}, false, fmt.Errorf("venue: bybit book frame: %w: %v", ErrWireUndecodable, err)
	}

	if w.Topic == "" {
		if w.Op != "" {
			return BybitBookMessage{}, false, nil
		}
		return BybitBookMessage{}, false, fmt.Errorf("venue: bybit frame with no topic and no op: %w", ErrWireForeign)
	}
	if !strings.HasPrefix(w.Topic, "orderbook.") {
		return BybitBookMessage{}, false, fmt.Errorf("venue: bybit topic %q: %w", w.Topic, ErrWireForeign)
	}
	if w.Type != string(BybitBookSnapshotKind) && w.Type != string(BybitBookDeltaKind) {
		return BybitBookMessage{}, false, fmt.Errorf("venue: bybit book type %q: %w", w.Type, ErrWireForeign)
	}
	if w.TS <= 0 {
		return BybitBookMessage{}, false, fmt.Errorf("venue: bybit book ts=%d: %w", w.TS, ErrWireIncomplete)
	}

	var d bybitBookDataWire
	if err := json.Unmarshal(w.Data, &d); err != nil {
		return BybitBookMessage{}, false, fmt.Errorf("venue: bybit book data: %w: %v", ErrWireUndecodable, err)
	}
	if d.Symbol == "" {
		return BybitBookMessage{}, false, fmt.Errorf("venue: bybit book: symbol: %w", ErrWireIncomplete)
	}
	if d.Update == 0 {
		return BybitBookMessage{}, false, fmt.Errorf("venue: bybit book u=0: %w", ErrWireIncomplete)
	}
	if d.Seq == 0 {
		return BybitBookMessage{}, false, fmt.Errorf("venue: bybit book seq=0: %w", ErrWireIncomplete)
	}

	msg := BybitBookMessage{
		Kind:       BybitBookKind(w.Type),
		UpdateID:   d.Update,
		Seq:        d.Seq,
		SourceTime: time.UnixMilli(w.TS).UTC(),
	}

	switch msg.Kind {
	case BybitBookSnapshotKind:
		book, err := decodeBookLevels(d.Bids, d.Asks, base, quote)
		if err != nil {
			return BybitBookMessage{}, false, fmt.Errorf("venue: bybit book snapshot: %w", err)
		}
		msg.Snapshot = book
	case BybitBookDeltaKind:
		delta, err := decodeBybitBookChanges(d, base, quote)
		if err != nil {
			return BybitBookMessage{}, false, err
		}
		msg.Delta = delta
	}
	return msg, true, nil
}

// decodeBybitBookChanges parses a delta's instruction lists. Zero size is
// LEGAL here and means remove (Profiles §5) — which is exactly LevelChange's
// contract and why a delta cannot share the snapshot's level parser: the same
// bytes mean "resting order" in one frame kind and "removal" in the other.
func decodeBybitBookChanges(d bybitBookDataWire, base, quote fixedpoint.Asset) (BookDelta, error) {
	parse := func(side BookSide, pairs [][2]string, into []LevelChange) ([]LevelChange, error) {
		for i, p := range pairs {
			price, err := fixedpoint.ParsePrice(base, quote, p[0])
			if err != nil {
				return nil, fmt.Errorf("venue: bybit book delta %s change %d price %q: %w", side, i, p[0], err)
			}
			qty, err := fixedpoint.ParseQuantity(base, p[1])
			if err != nil {
				return nil, fmt.Errorf("venue: bybit book delta %s change %d qty %q: %w", side, i, p[1], err)
			}
			c := LevelChange{Side: side, Price: price, Quantity: qty}
			if err := c.Validate(); err != nil {
				return nil, fmt.Errorf("venue: bybit book delta %s change %d: %w", side, i, err)
			}
			into = append(into, c)
		}
		return into, nil
	}

	changes := make([]LevelChange, 0, len(d.Bids)+len(d.Asks))
	changes, err := parse(SideBid, d.Bids, changes)
	if err != nil {
		return BookDelta{}, err
	}
	changes, err = parse(SideAsk, d.Asks, changes)
	if err != nil {
		return BookDelta{}, err
	}

	// Bybit is a single-sequence venue: the message covers exactly one venue
	// sequence, so the span BookDelta carries is length one — FirstSeq is `u`,
	// the same value the capture envelope's VenueSeq holds. BookDelta records
	// why that is the span of length one and not a special case.
	delta := BookDelta{Changes: changes, FirstSeq: d.Update}
	if err := delta.Validate(); err != nil {
		return BookDelta{}, fmt.Errorf("venue: bybit book delta: %w", err)
	}
	return delta, nil
}
