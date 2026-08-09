package venue

import (
	"encoding/json"
	"fmt"

	"github.com/giiorg/arb-lab/internal/engine/venue/transport"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// This file is Bybit's position surface (Profiles §9).
//
// # Bybit states the direction OUTSIDE the number, which is the mirror of
// # Binance and the reason this decoder cannot be shared
//
// Verbatim from the V5 position docs: size is "Position size, always positive",
// side is "'Buy': long, 'Sell': short", and side will "return an empty string ''
// for an empty position". Binance signs positionAmt and puts "BOTH" in its side
// field. Neither venue's row is readable with the other's rule, and both are
// readable with a plausible-looking wrong one: reading Bybit's size alone reports
// every short as a long, and reading Binance's side for a direction finds a
// constant.
//
// # The empty side is a VALUE, and refusing it would refuse the flat account
//
// "" beside a size of 0 is Bybit reporting a symbol it holds nothing in — the
// ordinary answer for a symbol with a settled position, and it arrives whenever
// this system asks about instruments it has closed. A side validator that
// rejected the empty string would turn a flat venue into a failed boot. What is
// refused instead is the CONTRADICTION: an empty side beside a nonzero size is a
// row whose direction is unreadable, and defaulting it to either direction is a
// signed number this decoder would be inventing.
//
// # settleCoin is REQUIRED here because the enumeration has no symbol
//
// Verbatim: for linear, "either symbol or settleCoin is required. symbol has a
// higher priority". This surface deliberately sends no symbol (positions.go), so
// the scope's settlement asset is the parameter that makes the call legal — and a
// scope naming a different settlement asset is answered without error and without
// the positions this system holds, which is the one failure here that produces a
// confident wrong answer rather than a refusal.

// EncodePositions implements PositionCodec.
func (c *BybitOrderCodec) EncodePositions(scope PositionScope) (transport.Call, error) {
	if err := scope.Validate(); err != nil {
		return transport.Call{}, err
	}
	if scope.Product == ids.ProductSpot {
		// Bybit's position endpoint takes category ∈ {linear, inverse, option}
		// and has no spot member: a spot balance is not a position. Refused with
		// the surface-absent sentinel rather than with bybitCategory's routing
		// error, because "spot" IS a legal category on every other endpoint here
		// and a routing diagnosis would send a reader to add it.
		return transport.Call{}, fmt.Errorf("bybit spot: %w", ErrPositionSurfaceAbsent)
	}
	category, err := bybitCategory(scope.Product)
	if err != nil {
		return transport.Call{}, err
	}
	return transport.Call{
		Op:     string(c.venue) + ".Positions",
		Method: "GET",
		Path:   "/v5/position/list",
		Params: transport.Params{
			{Key: "category", Value: category},
			{Key: "settleCoin", Value: string(scope.Settle)},
		},
		Family: scope.Product,

		// Bybit publishes no per-endpoint weight — /v5/position/list is metered
		// at 50 requests per second per UID — so there is nothing to charge and
		// nothing to invent. OrderByClientID's argument, unchanged. RequestOnly
		// is that fact with a carrier: the bare zero Cost is the field nobody
		// filled in, and Cost.Validate refuses it before Acquire can price the
		// call.
		Cost: ratelimit.RequestOnly(),
	}, nil
}

// bybitPositionWire is one row of the position list, narrowed to the three fields
// §7.5 reads.
type bybitPositionWire struct {
	Symbol      string `json:"symbol"`
	Side        string `json:"side"`
	Size        string `json:"size"`
	PositionIdx int    `json:"positionIdx"`
}

// bybitPositionListWire is the envelope. A separate struct from
// bybitListWire because the row type differs and Go's decoder is structural: one
// envelope generic over both would decode an order list into a position list
// without complaint.
type bybitPositionListWire struct {
	Result struct {
		Category string              `json:"category"`
		List     []bybitPositionWire `json:"list"`
	} `json:"result"`
}

// bybitOneWayPositionIdx is positionIdx in one-way mode: "0: One-Way Mode".
const bybitOneWayPositionIdx = 0

// DecodePositions implements PositionCodec.
func (c *BybitOrderCodec) DecodePositions(scope PositionScope, assets map[ids.VenueInstrument]InstrumentAssets, resp transport.Response) (PositionReport, error) {
	var wire bybitPositionListWire
	if err := json.Unmarshal(resp.Body, &wire); err != nil {
		return PositionReport{}, fmt.Errorf("bybit positions: %w", err)
	}

	category, err := bybitCategory(scope.Product)
	if err != nil {
		return PositionReport{}, err
	}
	if wire.Result.Category != category {
		// The envelope echoes which market answered. A mismatch means these rows
		// describe a different family's positions, and attributing them to this
		// one is the same class of error as reading a spot order's state off a
		// futures response.
		return PositionReport{}, fmt.Errorf("bybit positions answer category %q, asked for %q: %w",
			wire.Result.Category, category, ErrPositionsUnreadable)
	}

	var report PositionReport
	seen := make(map[string]struct{}, len(wire.Result.List))
	for i, row := range wire.Result.List {
		if row.Symbol == "" {
			return PositionReport{}, fmt.Errorf("bybit position %d carries no symbol: %w",
				i, ErrPositionsUnreadable)
		}
		if row.PositionIdx != bybitOneWayPositionIdx {
			return PositionReport{}, fmt.Errorf("bybit position %d on %s is positionIdx %d, not %d: %w",
				i, row.Symbol, row.PositionIdx, bybitOneWayPositionIdx, ErrPositionModeNotOneWay)
		}
		if _, dup := seen[row.Symbol]; dup {
			return PositionReport{}, fmt.Errorf("bybit answers twice for %s: %w",
				row.Symbol, ErrPositionsUnreadable)
		}
		seen[row.Symbol] = struct{}{}

		instrument, a, declared, err := positionInstrument(c.venue, scope, row.Symbol, assets)
		if err != nil {
			return PositionReport{}, err
		}
		if !declared {
			report.Unrecognized = append(report.Unrecognized, row.Symbol)
			continue
		}
		net, err := bybitSignedSize(a.Base, row.Size, row.Side)
		if err != nil {
			return PositionReport{}, fmt.Errorf("bybit position %d on %s: %w", i, row.Symbol, err)
		}
		report.Positions = append(report.Positions, Position{Instrument: instrument, Net: net})
	}

	sortPositionReport(&report)
	return report, nil
}

// bybitSignedSize applies a row's side to its unsigned size.
//
// The empty-side arm is checked against the SIZE rather than trusted, because the
// two fields can disagree and only one of the disagreements is safe to absorb: ""
// with size 0 is the documented flat row, while "" with a nonzero size is a
// direction this decoder does not have. Reading the second as flat would report a
// live position as no position — the one answer that makes §7.5's net-delta
// assertion pass over exposure nobody can see.
//
// # Shared with the MARGIN codec, and what that costs the differential test
//
// Task #219 gave the margin surface its own signed quantity, and both Bybit
// codecs now reach this. Taking the strings rather than a wire struct is what
// makes that possible without either surface's narrowing picking up the other's
// fields — the two wire types stay separate and each declares its own tags.
//
// The sharing is deliberate and it removes a failure mode rather than hiding
// one: a rule stated once cannot drift into two readings of "Sell". What it DOES
// remove is the shared-payload differential test's ability to see a sign
// disagreement, because there is no longer a second sign rule to disagree. What
// that test still covers on this venue is which JSON KEYS each narrowing
// declares, which is the live hazard here — `size` sits on a row beside
// `positionValue`, and `positionIM` beside `positionIMByMp`. The Binance half of
// the same test remains genuinely differential: both codecs parse `positionAmt`
// independently, with no helper between them.
func bybitSignedSize(base fixedpoint.Asset, size, side string) (fixedpoint.Quantity, error) {
	q, err := fixedpoint.ParseQuantity(base, size)
	if err != nil {
		return fixedpoint.Quantity{}, fmt.Errorf("size %q: %w", size, err)
	}
	sign, err := q.Sign()
	if err != nil {
		return fixedpoint.Quantity{}, fmt.Errorf("size %q: %w", size, err)
	}
	if sign < 0 {
		// "Position size, always positive". A negative one means the venue has
		// started signing the field, which would make side a second statement of
		// a direction already in the number — and this decoder would then be
		// applying it twice.
		return fixedpoint.Quantity{}, fmt.Errorf("size %q is negative on a surface documented as always positive: %w",
			size, ErrPositionsUnreadable)
	}

	switch side {
	case "Buy":
		return q, nil
	case "Sell":
		return q.Neg()
	case "":
		if sign != 0 {
			return fixedpoint.Quantity{}, fmt.Errorf("size %q carries no side: %w",
				size, ErrPositionSideUnreadable)
		}
		return q, nil
	}
	return fixedpoint.Quantity{}, fmt.Errorf("side %q: %w", side, ErrPositionSideUnreadable)
}

// Compile-time proof that the codec satisfies the position half too (§3.6
// rule 2).
var _ PositionCodec = (*BybitOrderCodec)(nil)
