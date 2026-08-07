package venue

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// This file decodes Binance's funding wire: the `<sym>@markPrice` WS frame and
// the `GET /fapi/v1/fundingInfo` REST overlay. The two compose — §39p-2: the
// frame carries rate and settlement instant and NO interval, cap or floor;
// those exist only in the overlay — so neither half alone can produce a
// venue.Funding, and the composition lives here beside the DTO it builds,
// where a Binance shape-change stays a compile error in one package.
//
// Decoders live in engine/venue rather than in the replay driver or the (future)
// live WS client, because both of those are transports: what varies between
// them is HOW bytes arrive, and what must not vary is what the bytes mean.

// BinanceMarkPrice is the funding-relevant half of one markPrice frame,
// decoded and unit-checked but not yet a venue.Funding — see
// FundingInfoOverlay.Compose for the missing half.
//
// # The OTHER half of the frame now has its own decoder, and the split is the
// # data class rather than convenience [M4 #207]
//
// This type is named for the frame and reads only its funding fields. That was
// complete while funding was the frame's only consumer; §7.1's liq-distance
// trigger is a second one, and it reads `p` — the mark — which this type
// deliberately drops. The mark did NOT get added here: it is §3.4's own third
// data class with its own §5.2 freshness budget, and one struct is one budget.
// See Mark for the argument in full, including the Bybit half of it.
//
// So one frame is decoded by two functions that share no state and no return
// type. The cost is a second json.Unmarshal per message, which is a per-symbol
// 1 Hz stream; the alternative costs a mark measured against funding's
// tolerance, in the one computation whose staleness §7.1 escalates on.
type BinanceMarkPrice struct {
	Symbol string
	Rate   fixedpoint.Rate

	// NextSettle is the venue's next settlement instant (`T`, epoch ms,
	// venue time base).
	NextSettle time.Time

	// EventTime is the venue's own stamp on the frame (`E`, epoch ms) — the
	// envelope's SourceTime, surfaced here because the payload is the only
	// place Binance states it.
	EventTime time.Time
}

// binanceMarkPriceWire is the frame as Binance spells it (Profiles §1, §5:
// per-symbol raw stream on the /market prefix). Money fields are strings and
// parsed through fixedpoint; instants are epoch-ms integers.
type binanceMarkPriceWire struct {
	Event      string `json:"e"`
	EventTime  int64  `json:"E"`
	Symbol     string `json:"s"`
	Rate       string `json:"r"`
	NextSettle int64  `json:"T"`
}

// DecodeBinanceMarkPrice decodes one raw markPrice frame.
//
// A frame whose event token is not "markPriceUpdate" is refused as FOREIGN
// rather than skipped: the capture path derives the stream, so a different
// event here means a misfiled file or a mis-subscribed topic — a wiring fact
// someone must see, not chatter to step over.
func DecodeBinanceMarkPrice(raw []byte) (BinanceMarkPrice, error) {
	var w binanceMarkPriceWire
	if err := json.Unmarshal(raw, &w); err != nil {
		return BinanceMarkPrice{}, fmt.Errorf("venue: binance markPrice frame: %w: %v", ErrWireUndecodable, err)
	}
	if w.Event != "markPriceUpdate" {
		return BinanceMarkPrice{}, fmt.Errorf("venue: binance event %q: %w", w.Event, ErrWireForeign)
	}
	if w.Symbol == "" {
		return BinanceMarkPrice{}, fmt.Errorf("venue: binance markPrice: symbol: %w", ErrWireIncomplete)
	}
	rate, err := fixedpoint.ParseRate(w.Rate)
	if err != nil {
		return BinanceMarkPrice{}, fmt.Errorf("venue: binance funding rate %q: %w", w.Rate, err)
	}
	if w.NextSettle <= 0 {
		return BinanceMarkPrice{}, fmt.Errorf("venue: binance markPrice T=%d: %w", w.NextSettle, ErrWireIncomplete)
	}
	if w.EventTime <= 0 {
		return BinanceMarkPrice{}, fmt.Errorf("venue: binance markPrice E=%d: %w", w.EventTime, ErrWireIncomplete)
	}
	return BinanceMarkPrice{
		Symbol:     w.Symbol,
		Rate:       rate,
		NextSettle: time.UnixMilli(w.NextSettle).UTC(),
		EventTime:  time.UnixMilli(w.EventTime).UTC(),
	}, nil
}

// binanceMarkWire is the same frame read for its MARK fields. A second struct
// rather than more tags on binanceMarkPriceWire, so that the two decoders stay
// independently readable: what each one requires is exactly the set of keys its
// own data class cannot do without, and a shared struct would make a field
// mandatory for both the moment either needed it.
// # `P` is DECLARED although nothing reads it, and that is not tidiness
//
// The frame states both `p` (mark price) and `P` (estimated settlement price).
// encoding/json prefers an exact tag match, but a key with NO declared field
// falls back to case-INSENSITIVE matching — so a struct declaring only `p`
// lets `P` compete for the same field, and which one lands depends on key
// ORDER in the document. Binance sends `P` after `p`, so the estimated settle
// price wins and the mark is silently the wrong number.
//
// Nothing downstream can catch it: an estimated settlement price is a
// plausible mark, within a dollar of the real one on this fixture. Validate
// passes, the liq distance computes, and the trigger is simply wrong.
//
// Declaring the key is what converts the fallback into an exact match. The
// field is deliberately unused rather than removed — see mark_test.go's
// reordered fixture, which is what keeps this from being re-tidied away.
//
// SECOND INSTANCE IN THIS PACKAGE [M4 #207]. binanceFuturesDepthWire's FirstID
// carries the same note for `U` vs `u`, found by a survived mutation in session
// 52. That note sat on the struct that had the defect, so it said nothing to
// the next single-letter Binance wire written — which is this one, two files
// away, and it went in wrong. The hazard belongs to the SEAM: any struct here
// with single-letter tags must declare every key of the frame that differs from
// a declared one only by case, whether or not anything reads it.
type binanceMarkWire struct {
	Event     string `json:"e"`
	EventTime int64  `json:"E"`
	Symbol    string `json:"s"`
	Mark      string `json:"p"`

	// EstSettle claims `P` so it cannot collide with Mark. Read by nothing.
	EstSettle string `json:"P"`
}

// BinanceMark is one markPrice frame read as §3.4's mark data class.
//
// It carries the reading and the venue's own stamp, the same two things
// BinanceMarkPrice carries, and for the same reason: `E` is where Binance
// states the frame's instant, so the payload is the only place to surface it.
type BinanceMark struct {
	Symbol string
	Mark   Mark

	// EventTime is the venue's own stamp on the frame (`E`, epoch ms) — the
	// envelope's SourceTime.
	EventTime time.Time
}

// DecodeBinanceMark decodes one raw markPrice frame as a mark reading.
//
// # It takes the assets, and that is what makes the result a PRICE
//
// A mark is a price, and fixedpoint.Price carries base and quote so that
// multiplying a BTC size by an ETH price is an error rather than a plausible
// number. The book decoders take the same pair for the same reason; the frame
// states a bare symbol, and a decoder inventing assets from it would be doing
// the venue-symbol split §5.2 gives to SymbolSpec — "a wrong split mints money
// whose asset tag is a lie".
//
// A frame whose event token is not "markPriceUpdate" is refused as FOREIGN,
// DecodeBinanceMarkPrice's rule and its reason: the stream is derived from the
// capture path, so a different event means a misfiled file or a mis-subscribed
// topic.
func DecodeBinanceMark(raw []byte, base, quote fixedpoint.Asset) (BinanceMark, error) {
	var w binanceMarkWire
	if err := json.Unmarshal(raw, &w); err != nil {
		return BinanceMark{}, fmt.Errorf("venue: binance mark frame: %w: %v", ErrWireUndecodable, err)
	}
	if w.Event != "markPriceUpdate" {
		return BinanceMark{}, fmt.Errorf("venue: binance event %q: %w", w.Event, ErrWireForeign)
	}
	if w.Symbol == "" {
		return BinanceMark{}, fmt.Errorf("venue: binance mark: symbol: %w", ErrWireIncomplete)
	}
	if w.Mark == "" {
		return BinanceMark{}, fmt.Errorf("venue: binance mark p: %w", ErrWireIncomplete)
	}
	if w.EventTime <= 0 {
		return BinanceMark{}, fmt.Errorf("venue: binance mark E=%d: %w", w.EventTime, ErrWireIncomplete)
	}
	price, err := fixedpoint.ParsePrice(base, quote, w.Mark)
	if err != nil {
		return BinanceMark{}, fmt.Errorf("venue: binance mark price %q: %w", w.Mark, err)
	}
	m := Mark{Price: price}
	if err := m.Validate(); err != nil {
		return BinanceMark{}, fmt.Errorf("venue: binance mark: %w", err)
	}
	return BinanceMark{
		Symbol:    w.Symbol,
		Mark:      m,
		EventTime: time.UnixMilli(w.EventTime).UTC(),
	}, nil
}

// FundingInfoOverlay holds the last complete fundingInfo answer: per symbol,
// the interval and the rate clamps. It is a full-replace store — each poll
// returns the venue's whole list, so Apply swaps the previous state out
// rather than merging, and a symbol that disappears from the answer
// disappears from the overlay (its next Compose reports the absence instead
// of serving a reading the venue has withdrawn).
type FundingInfoOverlay struct {
	rows map[string]overlayRow
}

type overlayRow struct {
	cap, floor fixedpoint.Rate
	interval   time.Duration
}

// fundingInfoRowWire is one row as the venue spells it. updateTime is
// deliberately not declared: Profiles §5 measured it NULL for exactly our
// symbol while integer elsewhere, so a typed field is a decoder that parses
// 740 rows and fails on the one we trade — and nothing here reads it.
type fundingInfoRowWire struct {
	Symbol        string `json:"symbol"`
	Cap           string `json:"adjustedFundingRateCap"`
	Floor         string `json:"adjustedFundingRateFloor"`
	IntervalHours int    `json:"fundingIntervalHours"`
}

// NewFundingInfoOverlay returns an empty overlay: every Compose reports
// absence until the first Apply — the "no answer yet" state §39p-2 chose over
// a default table built for no caller.
func NewFundingInfoOverlay() *FundingInfoOverlay {
	return &FundingInfoOverlay{rows: map[string]overlayRow{}}
}

// Applied reports whether an answer has been applied at all — false is the
// cold state, distinguishable from an applied answer that lacks a symbol.
func (o *FundingInfoOverlay) Applied() bool { return o.rows != nil && len(o.rows) > 0 }

// Apply replaces the overlay with one fundingInfo response body.
//
// All-or-nothing: a malformed row refuses the whole answer and keeps the
// previous state, because a half-applied list is a state no poll ever
// produced — the same torn-state refusal the capture codec makes.
func (o *FundingInfoOverlay) Apply(body []byte) error {
	var wire []fundingInfoRowWire
	if err := json.Unmarshal(body, &wire); err != nil {
		return fmt.Errorf("venue: fundingInfo body: %w: %v", ErrWireUndecodable, err)
	}
	next := make(map[string]overlayRow, len(wire))
	for _, r := range wire {
		if r.Symbol == "" {
			return fmt.Errorf("venue: fundingInfo row with no symbol: %w", ErrWireIncomplete)
		}
		capRate, err := fixedpoint.ParseRate(r.Cap)
		if err != nil {
			return fmt.Errorf("venue: fundingInfo %s cap %q: %w", r.Symbol, r.Cap, err)
		}
		floorRate, err := fixedpoint.ParseRate(r.Floor)
		if err != nil {
			return fmt.Errorf("venue: fundingInfo %s floor %q: %w", r.Symbol, r.Floor, err)
		}
		interval, err := FundingIntervalFromHours(r.IntervalHours)
		if err != nil {
			return fmt.Errorf("venue: fundingInfo %s: %w", r.Symbol, err)
		}
		next[r.Symbol] = overlayRow{cap: capRate, floor: floorRate, interval: interval}
	}
	o.rows = next
	return nil
}

// Compose joins one decoded frame with the overlay's answer for its symbol.
//
// The false return is the cold or absent case — no overlay applied yet, or an
// applied answer without this symbol — and per §39p-2 it emits nothing rather
// than defaulting: the "absent ⇒ 8h" rule needs cap/floor companions that are
// per-symbol-class, our symbol is present in measured fact (Profiles §5,
// 2026-07-29), and a default table built now would be built for no caller.
// The construct that makes it buildable is the multi-symbol capture config.
func (o *FundingInfoOverlay) Compose(m BinanceMarkPrice) (Funding, bool, error) {
	row, ok := o.rows[m.Symbol]
	if !ok {
		return Funding{}, false, nil
	}
	f := Funding{
		Rate:       m.Rate,
		Interval:   row.interval,
		NextSettle: m.NextSettle,
		Cap:        row.cap,
		Floor:      row.floor,
	}
	if err := f.Validate(); err != nil {
		return Funding{}, false, fmt.Errorf("venue: binance funding for %s: %w", m.Symbol, err)
	}
	return f, true, nil
}
