package venue

import (
	"encoding/json"
	"fmt"

	"github.com/giiorg/arb-lab/internal/engine/venue/transport"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// This file is Bybit's margin top-up: `POST /v5/position/add-margin` with a
// POSITIVE `margin` (Profiles §4 — "doc slug `manual-add-margin`; path is
// `add-margin`", a known slug/path mismatch, and "`margin` positive adds,
// negative reduces").
//
// # The acknowledgement names the SUBJECT and not the ACT
//
// Profiles §4, verbatim: the response is "`category, symbol, positionIdx,
// riskId, …, positionIM, positionMM, …, updatedTime` (no op id; the only
// resolution signal is the `positionIM` delta vs a before-state)". So this venue
// echoes WHICH position it acted on and never HOW MUCH — the mirror image of
// Binance, and the reason MarginWriteCodec has no shared echo helper.
//
// This decoder therefore checks the market, the family and the one-way pin, and
// cannot check the amount. What that costs is the case Binance's decoder catches:
// a venue that applied a different quantity than we asked for is invisible here
// and surfaces later as §7.1's Ambiguous, since the posted-margin comparison sees
// a movement matching nothing. Late and safe, and the boundary is a venue fact
// rather than a choice.
//
// # `positionIM` IS on this response and is deliberately NOT read
//
// It is the after-state — the exact quantity §7.1's evidence contract compares
// against the journaled before-state — and reading it here is the tempting
// shortcut this file exists to refuse. Three reasons, and the third is the one
// that would not have been obvious:
//
//   - It is a SECOND authority for a figure ports.VenueMarginStatus already owns.
//     §7.1 words the Bybit contract as "the `positionIM` delta vs the journaled
//     before-state", one source, and a second reading of it inside the write path
//     would be two producers of one quantity with no rule for which wins.
//   - It exists on ONE venue. A resolution rule that consumed it would be a rule
//     Binance cannot run, so the two venues' verdicts would rest on different
//     evidence for reasons that have nothing to do with §7.1.
//   - It is only available in the case that needs no evidence at all. The
//     evidence path exists for an op whose outcome was NOT observed — a lost
//     response, a killed process. Here the response arrived; there is nothing to
//     reconstruct.
//
// # `positionIdx` is SENT as well as checked, unlike Binance's `positionSide`
//
// The asymmetry is the venues', not this file's. Bybit's parameter is required
// for a one-way position (0), and the response echoes it, so sending it and
// checking the echo are both available and both cheap. Binance's is optional,
// defaults correctly under the §4.1 pin, and comes back on nothing.

// EncodeAddIsolatedMargin implements MarginWriteCodec.
func (c *BybitOrderCodec) EncodeAddIsolatedMargin(instrument ids.VenueInstrument,
	assets InstrumentAssets, amount fixedpoint.Money) (transport.Call, error) {

	if err := instrument.Validate(); err != nil {
		return transport.Call{}, err
	}
	if err := checkMarginWriteAmount(assets, amount); err != nil {
		return transport.Call{}, err
	}
	if instrument.Product == ids.ProductSpot {
		// EncodeMarginStatus's refusal, unchanged: spot is a legal `category` on
		// every other Bybit endpoint here, so a routing diagnosis would send a
		// reader to configure a surface that exists on neither venue.
		return transport.Call{}, fmt.Errorf("bybit spot: %w", ErrPositionSurfaceAbsent)
	}
	category, err := bybitCategory(instrument.Product)
	if err != nil {
		return transport.Call{}, err
	}

	body := bybitAddMarginWire{
		Category:    category,
		Symbol:      instrument.Symbol,
		Margin:      amount.Decimal(),
		PositionIdx: bybitOneWayPositionIdx,
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return transport.Call{}, fmt.Errorf("bybit margin top-up body: %w", err)
	}

	return transport.Call{
		Op:     string(c.venue) + ".AddIsolatedMargin",
		Method: "POST",
		Path:   "/v5/position/add-margin",
		Body:   raw,
		Family: instrument.Product,

		// No published weight — Bybit meters this family per request — so the
		// request itself is the cost. EncodeMarginStatus's note, unchanged:
		// RequestOnly carries that fact, where a bare zero Cost is the field
		// nobody filled in.
		Cost: ratelimit.RequestOnly(),
	}, nil
}

// bybitAddMarginWire is the add-margin body.
//
// `margin` is a STRING carrying the signed decimal, which is how this venue
// takes every quantity, and it is rendered from fixedpoint rather than formatted
// — the same rule EncodePlace follows for `qty` and `price`. The sign is
// positive here by construction: checkMarginWriteAmount refuses anything else,
// and the reduce direction is a surface with no consumer (marginwrite.go).
//
// `positionIdx` carries no omitempty, for bybitPlaceWire's reason at a different
// field: the value that must be sent is ZERO — Bybit's one-way index, the §4.1
// pin — and omitempty drops exactly zero. An add-margin request with no
// positionIdx is one Bybit answers for whichever leg it chooses on a hedge-mode
// account, which is the account state §4.1's gate refuses and this parameter
// makes unexpressible.
type bybitAddMarginWire struct {
	Category    string `json:"category"`
	Symbol      string `json:"symbol"`
	Margin      string `json:"margin"`
	PositionIdx int    `json:"positionIdx"`
}

// bybitAddMarginAckWire is the acknowledgement envelope, narrowed to the three
// fields that say which position was acted on.
//
// retCode and retMsg are not read: transport.ClassifyResponse has already refused
// a non-zero retCode, which on this venue arrives with HTTP 200 — bybitAckWire's
// rule, and duplicating the check would put a second, weaker copy of it where the
// first one is not visible.
type bybitAddMarginAckWire struct {
	Result struct {
		Category    string `json:"category"`
		Symbol      string `json:"symbol"`
		PositionIdx int    `json:"positionIdx"`
	} `json:"result"`
}

// DecodeAddIsolatedMargin implements MarginWriteCodec.
func (c *BybitOrderCodec) DecodeAddIsolatedMargin(instrument ids.VenueInstrument,
	assets InstrumentAssets, amount fixedpoint.Money, resp transport.Response) error {

	var wire bybitAddMarginAckWire
	if err := json.Unmarshal(resp.Body, &wire); err != nil {
		return fmt.Errorf("bybit margin top-up on %s: %w", instrument.Symbol, err)
	}

	category, err := bybitCategory(instrument.Product)
	if err != nil {
		return err
	}
	if wire.Result.Category != category {
		// DecodeMarginStatus's check on the same envelope field: the category
		// selects which market answered, and BTCUSDT exists in more than one. A
		// top-up acknowledged for another family is margin posted against a
		// position this op does not describe.
		return fmt.Errorf("bybit acknowledges a margin top-up in category %q, asked for %q: %w",
			wire.Result.Category, category, ErrMarginWriteUnacknowledged)
	}
	if wire.Result.Symbol != instrument.Symbol {
		return fmt.Errorf("bybit acknowledges a margin top-up on %s, asked about %s: %w",
			wire.Result.Symbol, instrument.Symbol, ErrMarginWriteUnacknowledged)
	}
	if wire.Result.PositionIdx != bybitOneWayPositionIdx {
		// The §4.1 pin re-asserted against the venue's own account of what it
		// did. A hedge-mode account holds two legs with two margin figures, and a
		// top-up acknowledged for one of them was resolved afterwards against a
		// before-state read for the whole position.
		return fmt.Errorf("bybit acknowledges a margin top-up on %s at positionIdx %d, not %d: %w",
			instrument.Symbol, wire.Result.PositionIdx, bybitOneWayPositionIdx, ErrPositionModeNotOneWay)
	}
	return nil
}

var _ MarginWriteCodec = (*BybitOrderCodec)(nil)
