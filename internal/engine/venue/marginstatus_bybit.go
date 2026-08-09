package venue

import (
	"encoding/json"
	"fmt"

	"github.com/giiorg/arb-lab/internal/engine/venue/transport"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// This file is Bybit's margin-status surface (Profiles §9).
//
// # "No position" is spelled the OPPOSITE way round from Binance, and §7.5's
// # enumeration taught the opposite lesson
//
// Verbatim, on the symbol parameter: "If `symbol` passed, it returns data
// regardless of having position or not". So a flat instrument here is a ROW with
// a size of 0, while on Binance it is an empty array — and §7.5's enumeration
// trained the reading that an absent row means flat, which is true there and
// says nothing about this call. Both codecs consequently decide presence from the
// size rather than from the row count, and the row count is only ever refused
// when it exceeds one.
//
// # `liqPrice` is EMPTY on a healthy position, and that is a documented value
//
// Verbatim: it "is the real price for isolated and cross positions, and keeps
// `""` when liqPrice <= minPrice or liqPrice >= maxPrice". A short whose
// liquidation sits above the symbol's price band — the well-margined case, which
// is the state a correctly sized position is supposed to be in — is answered with
// an empty string. Read as a zero it becomes a liquidation price far BELOW the
// mark, which every distance formula reports as maximum headroom. The shared rule
// is in marginLiqPrice, stated once for both venues.
//
// # `positionBalance` is DEPRECATED, which is why the posted figure is
// # `positionIM`
//
// Verbatim: positionBalance is "Deprecated, can refer to `positionIM` or
// `positionIMByMp` field". The name reads like the obvious answer to §4's
// "available margin" and the venue has retired it. `positionIM` is also the field
// §7.1 already names for the other direction — a margin top-up is resolved by
// "the `positionIM` delta vs the journaled before-state" — so the read side and
// the evidence side name one field, which is what makes a before/after comparison
// a comparison of the same quantity.
//
// Two more fields on this row are deprecated and none of them is declared:
// `bustPrice` ("Deprecated, always `""`") and `tradeMode` ("Deprecated, always
// 0"). `markPrice` is present and deliberately not read — the mark is its own
// class with its own port (record §78).
//
// # An EMPTY margin figure means Portfolio Margin, which §4.1 already refuses
//
// Verbatim, on both positionIM and positionMM: "Portfolio margin mode: returns
// `""`". §4.1's posture gate refuses Portfolio Margin, so an empty figure here is
// not a state to absorb — it means an account reached this trigger that the boot
// gate should have stopped, and the honest answer is a refusal naming the field
// rather than a zero margin requirement that reads as a position needing nothing.

// EncodeMarginStatus implements MarginStatusCodec.
//
// `settleCoin` is not sent and is not needed: it is required only for the
// symbol-less enumeration ("either `symbol` or `settleCoin` is required. `symbol`
// has a higher priority"), which is EncodePositions' call and not this one. That
// also removes this surface's half of the enumeration's standing limit — a
// position settled in an asset the scope did not name — because a symbol names
// its own settlement.
func (c *BybitOrderCodec) EncodeMarginStatus(instrument ids.VenueInstrument) (transport.Call, error) {
	if err := instrument.Validate(); err != nil {
		return transport.Call{}, err
	}
	if instrument.Product == ids.ProductSpot {
		// EncodePositions' refusal, unchanged: spot is a legal category on every
		// other Bybit endpoint here, so a routing diagnosis would send a reader to
		// add a route for a surface that does not exist on either venue.
		return transport.Call{}, fmt.Errorf("bybit spot: %w", ErrPositionSurfaceAbsent)
	}
	category, err := bybitCategory(instrument.Product)
	if err != nil {
		return transport.Call{}, err
	}
	return transport.Call{
		Op:     string(c.venue) + ".MarginStatus",
		Method: "GET",
		Path:   "/v5/position/list",
		Params: transport.Params{
			{Key: "category", Value: category},
			{Key: "symbol", Value: instrument.Symbol},
		},
		Family: instrument.Product,

		// No published weight — /v5/position/list is metered at 50 requests per
		// second per UID — so the request itself is the cost. EncodePositions'
		// note, unchanged: RequestOnly carries that fact, where a bare zero Cost
		// is the field nobody filled in.
		Cost: ratelimit.RequestOnly(),
	}, nil
}

// bybitMarginStatusWire is one position row, narrowed to what §7.1 asks for.
//
// A second narrowing of the row bybitPositionWire narrows, for the reason
// marginstatus.go states.
//
// # `side` IS declared now, and the note it replaces was right when written
//
// Until #219 this struct deliberately omitted `side`, on the argument that "a
// direction is exactly what this consumer does not need". Record §85 §1 changed
// what the consumer needs: §7.1's remedies size from a SIGNED quantity, Bybit
// states an always-positive `size`, and the sign therefore lives in `side` and
// nowhere else on the row. Omitting it now would leave this codec able to
// report a short and a long as the same number — which is the defect
// positions_bybit.go's file comment names in as many words.
//
// The old note's underlying worry is answered by the prohibition in
// marginstatus.go rather than by the omission: what must not happen is §7.5's
// invariant acquiring a second source, and that is a rule about who may READ
// SizingQuantity, not about which keys this struct declares.
//
// `avgPrice` is §85 §3's `e`. It sits on a row beside `positionValue` and
// `markPrice`, both of the same magnitude and character class, which is why the
// fixture carries the full row rather than the declared keys.
type bybitMarginStatusWire struct {
	Symbol      string `json:"symbol"`
	Side        string `json:"side"`
	Size        string `json:"size"`
	AvgPrice    string `json:"avgPrice"`
	PositionIdx int    `json:"positionIdx"`
	LiqPrice    string `json:"liqPrice"`
	PositionMM  string `json:"positionMM"`
	PositionIM  string `json:"positionIM"`
}

// bybitMarginStatusListWire is the envelope.
//
// Its own struct rather than bybitPositionListWire reused, on that type's own
// argument one surface over: Go's decoder is structural, so one envelope generic
// over both row types would decode either list into either shape without
// complaint — and here the two shapes are two narrowings of the SAME venue rows,
// which is the case where a mistaken reuse looks most correct.
type bybitMarginStatusListWire struct {
	Result struct {
		Category string                  `json:"category"`
		List     []bybitMarginStatusWire `json:"list"`
	} `json:"result"`
}

// DecodeMarginStatus implements MarginStatusCodec.
func (c *BybitOrderCodec) DecodeMarginStatus(instrument ids.VenueInstrument, assets InstrumentAssets, resp transport.Response) (MarginStatus, bool, error) {
	var wire bybitMarginStatusListWire
	if err := json.Unmarshal(resp.Body, &wire); err != nil {
		return MarginStatus{}, false, fmt.Errorf("bybit margin status: %w", err)
	}

	category, err := bybitCategory(instrument.Product)
	if err != nil {
		return MarginStatus{}, false, err
	}
	if wire.Result.Category != category {
		// The envelope echoes which market answered, and a mismatch means these
		// margin figures belong to a different family's position — the same
		// class of error as reading a spot order's state off a futures response.
		return MarginStatus{}, false, fmt.Errorf("bybit margin status answers category %q, asked for %q: %w",
			wire.Result.Category, category, ErrPositionsUnreadable)
	}

	rows := wire.Result.List
	if len(rows) == 0 {
		// Bybit documents a row "regardless of having position or not" for a
		// symbol-scoped call, so an empty list is not the documented flat answer
		// — but it says the same thing and there is nothing here to compute
		// from. Reported as absence rather than refused, because the conservative
		// answer for this consumer is "no position to monitor", and a refusal
		// would take the trigger off an instrument on a venue that merely
		// tightened its response.
		return MarginStatus{}, false, nil
	}
	if len(rows) > 1 {
		// One symbol was asked about. A second row is a hedge-mode account,
		// whose two legs carry two margin figures with no combined one stated.
		return MarginStatus{}, false, fmt.Errorf("bybit answers %d rows for %s: %w",
			len(rows), instrument.Symbol, ErrPositionsUnreadable)
	}

	row := rows[0]
	if row.Symbol != instrument.Symbol {
		return MarginStatus{}, false, fmt.Errorf("bybit answers for %s, asked about %s: %w",
			row.Symbol, instrument.Symbol, ErrPositionsUnreadable)
	}
	if row.PositionIdx != bybitOneWayPositionIdx {
		return MarginStatus{}, false, fmt.Errorf("bybit margin status on %s is positionIdx %d, not %d: %w",
			row.Symbol, row.PositionIdx, bybitOneWayPositionIdx, ErrPositionModeNotOneWay)
	}

	// Presence AND the sizing quantity, through §7.5's own rule rather than a
	// second one (bybitSignedSize). Signing before the presence test rather than
	// after is what makes the contradiction reachable here: `side: ""` beside a
	// NONZERO size is a live position whose direction is unreadable, and a
	// presence test that looked only at the size would report it held and then
	// size a remedy off an unsigned number. The documented flat row — `side: ""`
	// with size 0 — passes through this helper unchanged and is caught by the
	// zero check below, so the ordinary flat answer is not disturbed.
	size, err := bybitSignedSize(assets.Base, row.Size, row.Side)
	if err != nil {
		return MarginStatus{}, false, fmt.Errorf("bybit margin status on %s: %w", row.Symbol, err)
	}
	sign, err := size.Sign()
	if err != nil {
		return MarginStatus{}, false, fmt.Errorf("bybit margin status on %s size %q: %w",
			row.Symbol, row.Size, err)
	}
	if sign == 0 {
		return MarginStatus{}, false, nil
	}

	if row.PositionMM == "" || row.PositionIM == "" {
		// Portfolio Margin, which §4.1's posture gate refuses. See the file
		// comment: this is an account that should never have reached the
		// trigger, and the two available absorptions — a zero maintenance or a
		// zero posted margin — are the permissive reading in both directions.
		return MarginStatus{}, false, fmt.Errorf("bybit margin status on %s states positionMM %q positionIM %q: %w",
			row.Symbol, row.PositionMM, row.PositionIM, ErrMarginFigureUnstated)
	}

	// The quote asset, and there is nothing on the row to check it against:
	// Bybit publishes no per-row margin asset where Binance publishes
	// `marginAsset`. The premise (a linear perpetual margins in its quote asset)
	// is therefore asserted here and falsified only on the other venue, which is
	// why that check is worth keeping even though this codec cannot make it.
	maintenance, err := marginFigure(assets.Quote, "positionMM", row.PositionMM)
	if err != nil {
		return MarginStatus{}, false, fmt.Errorf("bybit margin status on %s: %w", row.Symbol, err)
	}
	posted, err := marginFigure(assets.Quote, "positionIM", row.PositionIM)
	if err != nil {
		return MarginStatus{}, false, fmt.Errorf("bybit margin status on %s: %w", row.Symbol, err)
	}
	liq, stated, err := marginLiqPrice(assets.Base, assets.Quote, row.LiqPrice)
	if err != nil {
		return MarginStatus{}, false, fmt.Errorf("bybit margin status on %s: %w", row.Symbol, err)
	}
	entry, err := marginEntryPrice(assets.Base, assets.Quote, row.AvgPrice)
	if err != nil {
		return MarginStatus{}, false, fmt.Errorf("bybit margin status on %s: %w", row.Symbol, err)
	}

	return MarginStatus{
		Instrument:     instrument,
		LiqPrice:       liq,
		LiqPriceStated: stated,
		Maintenance:    maintenance,
		PostedMargin:   posted,
		SizingQuantity: size,
		EntryPrice:     entry,
	}, true, nil
}

// Compile-time proof that the codec satisfies the margin-status half too (§3.6
// rule 2).
var _ MarginStatusCodec = (*BybitOrderCodec)(nil)
