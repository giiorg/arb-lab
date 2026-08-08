package venue

import (
	"encoding/json"
	"fmt"

	"github.com/giiorg/arb-lab/internal/engine/venue/transport"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// This file is Binance's margin-status surface (Profiles §9).
//
// # It reads V3, and the V3-vs-V2 argument is RE-DERIVED rather than inherited
//
// Profiles §9 records the rule the hard way: "the decision is PER QUESTION,
// which the paragraph above did not have to say and the next consumer needed" —
// §4.1 inherited §7.5's V3 choice, and V3 turned out not to publish the field it
// wanted at all. So this consumer states its own reason, and it is a different
// reason from §7.5's.
//
// §7.5 enumerates with NO symbol and needs a missing row to mean flat, which
// only V3's "only symbol that has position or open orders will be returned"
// gives. This call passes a symbol, where both versions answer, so that argument
// does not apply here at all. What decides it is what each version says about a
// FLAT symbol: V2 returns a row of zeros — a liquidation price of 0, a
// maintenance of 0, an isolated wallet of 0 — and every one of those is a number
// §7.1 computes with. V3 returns an empty array, which this codec reports as
// "no position" and the client turns into a status nobody can compute from.
//
// The two versions therefore agree on the healthy case and disagree exactly on
// the degenerate one, which is the case a monitor meets every time this system
// closes a package.
//
// # The weight is the SAME unverified figure as the enumeration's, on its cheap
// # arm
//
// #180 is still open and three readings are in play (Profiles §9's ⚠ block): a
// flat 5, "5 with `symbol`, 20 without", and "1 with `symbol`, 10 without". This
// call always passes a symbol, so it takes the CHEAP arm of both conditional
// readings, and the maximum over the three candidates is 5 — exact under two of
// them and over by 4 under the third. Same rule as EncodePositions (charge the
// maximum, because undercounting the IP budget is the failure that ends in the
// ban §7.6 exists to avoid), applied to the other arm, so a single browser read
// loosens both lines at once.
//
// Note the asymmetry that makes this worth stating: the enumeration's 20 and
// this call's 5 come from ONE unread line, so they are not independent figures
// and a correction moves both.
//
// # `marginAsset` is checked rather than trusted, and Bybit has no counterpart
//
// The row states which asset its margin figures are in. Binance is the only one
// of the two venues that does, so this is the one place the "margin is quoted in
// the quote asset on a linear perp" premise — the premise MarginStatus.Validate
// enforces, and which the Bybit codec has nothing to check against — is
// falsifiable against the venue rather than assumed from it.

// EncodeMarginStatus implements MarginStatusCodec.
//
// The symbol parameter IS sent, which is the exact inverse of EncodePositions and
// for the inverse reason: that call must not narrow, because the set it would
// narrow away is the only place a foreign position can be; this one asks about
// one position this system knows it holds, and enumerating the family to find it
// would charge the expensive weight arm and hand a per-instrument question a list
// to search.
func (c *BinanceOrderCodec) EncodeMarginStatus(instrument ids.VenueInstrument) (transport.Call, error) {
	if err := instrument.Validate(); err != nil {
		return transport.Call{}, err
	}
	path, cost, err := binanceMarginStatusRoute(instrument.Product)
	if err != nil {
		return transport.Call{}, err
	}
	return transport.Call{
		Op:     string(c.venue) + ".MarginStatus",
		Method: "GET",
		Path:   path,
		Params: transport.Params{
			{Key: "symbol", Value: instrument.Symbol},
		},
		Family: instrument.Product,
		Cost:   cost,
	}, nil
}

// binanceMarginStatusRoute is the path and the charged cost per family.
//
// Exhaustive over the enum-like (§3.6 rule 10), and spot is refused with the
// surface-absent sentinel rather than the unrouted error for
// binancePositionsRoute's reason: no venue has a spot position surface anywhere,
// so reporting it as a routing gap invites a reader to fix it by adding a route.
func binanceMarginStatusRoute(p ids.ProductCategory) (string, ratelimit.Cost, error) {
	switch p {
	case ids.ProductSpot:
		return "", ratelimit.Cost{}, fmt.Errorf("binance spot: %w", ErrPositionSurfaceAbsent)
	case ids.ProductLinearPerp:
		// 5, the maximum over #180's three candidate readings on the arm this
		// call takes — see the file comment. The enumeration charges 20 for the
		// other arm of the same unread line.
		return "/fapi/v3/positionRisk", ratelimit.Cost{Weight: 5}, nil
	case ids.ProductInversePerp:
		return "", ratelimit.Cost{}, fmt.Errorf("%s: %w", p, ErrOrderFamilyUnrouted)
	}
	return "", ratelimit.Cost{}, fmt.Errorf("%s: %w", p, ids.ErrProductUnknown)
}

// binanceMarginStatusWire is one V3 row, narrowed to what §7.1 asks for.
//
// A SECOND narrowing of the same twenty-field row that binancePositionWire
// narrows, and deliberately not a widening of it (marginstatus.go). The overlap
// is the three fields that identify the row — symbol, side and amount — and
// every one of them is read here for a different purpose than there: the amount
// is a PRESENCE test, never a quantity this system reasons from, because §7.5's
// net-delta invariant must keep a single source for the position.
//
// `markPrice` is on this row and is deliberately not declared: the mark has its
// own class, its own budget and its own port (record §78).
type binanceMarginStatusWire struct {
	Symbol           string `json:"symbol"`
	PositionSide     string `json:"positionSide"`
	PositionAmt      string `json:"positionAmt"`
	LiquidationPrice string `json:"liquidationPrice"`
	MaintMargin      string `json:"maintMargin"`
	IsolatedWallet   string `json:"isolatedWallet"`
	MarginAsset      string `json:"marginAsset"`
}

// DecodeMarginStatus implements MarginStatusCodec.
func (c *BinanceOrderCodec) DecodeMarginStatus(instrument ids.VenueInstrument, assets InstrumentAssets, resp transport.Response) (MarginStatus, bool, error) {
	var wire []binanceMarginStatusWire
	if err := json.Unmarshal(resp.Body, &wire); err != nil {
		return MarginStatus{}, false, fmt.Errorf("binance margin status: %w", err)
	}

	if len(wire) == 0 {
		// V3's silence: "only symbol that has position or open orders will be
		// returned". Asked about ONE symbol, an empty array is the venue saying
		// this account holds nothing there and has nothing resting either.
		return MarginStatus{}, false, nil
	}
	if len(wire) > 1 {
		// One symbol was asked about, so a second row is either a hedge-mode
		// account (whose two legs carry two margin figures with no combined one
		// stated) or a venue that ignored the symbol parameter. Taking either row
		// would attribute one leg's margin to the whole position.
		return MarginStatus{}, false, fmt.Errorf("binance answers %d rows for %s: %w",
			len(wire), instrument.Symbol, ErrPositionsUnreadable)
	}

	row := wire[0]
	if row.Symbol != instrument.Symbol {
		// The scope was a symbol, so a row for another one means the parameter
		// did not take effect — and the margin figures would then be a different
		// market's, which is the one failure here that produces a confident
		// number rather than a refusal.
		return MarginStatus{}, false, fmt.Errorf("binance answers for %s, asked about %s: %w",
			row.Symbol, instrument.Symbol, ErrPositionsUnreadable)
	}
	if row.PositionSide != binanceOneWayPositionSide {
		return MarginStatus{}, false, fmt.Errorf("binance margin status on %s is positionSide %q, not %q: %w",
			row.Symbol, row.PositionSide, binanceOneWayPositionSide, ErrPositionModeNotOneWay)
	}

	// The amount decides PRESENCE and is never carried out of this function.
	// V3 returns a row for a symbol with open orders and no position, so a row
	// is not by itself a position — and a zero-amount row's liquidation price
	// and maintenance are the degenerate figures this surface exists to keep
	// away from §7.1.
	amt, err := fixedpoint.ParseQuantity(assets.Base, row.PositionAmt)
	if err != nil {
		return MarginStatus{}, false, fmt.Errorf("binance margin status on %s positionAmt %q: %w",
			row.Symbol, row.PositionAmt, err)
	}
	sign, err := amt.Sign()
	if err != nil {
		return MarginStatus{}, false, fmt.Errorf("binance margin status on %s positionAmt %q: %w",
			row.Symbol, row.PositionAmt, err)
	}
	if sign == 0 {
		return MarginStatus{}, false, nil
	}

	if row.MarginAsset != string(assets.Quote) {
		// See the file comment: this is the only venue that states it, so it is
		// the only place the linear-perp premise is falsifiable. A mismatch means
		// either a non-linear contract routed here or a deployment whose declared
		// quote asset is not what the venue margins in — and typing the figures
		// in either asset would then be this codec asserting the premise instead
		// of checking it.
		return MarginStatus{}, false, fmt.Errorf("binance margins %s in %q, declared quote is %q: %w",
			row.Symbol, row.MarginAsset, assets.Quote, ErrMarginAssetsDisagree)
	}

	maintenance, err := marginFigure(assets.Quote, "maintMargin", row.MaintMargin)
	if err != nil {
		return MarginStatus{}, false, fmt.Errorf("binance margin status on %s: %w", row.Symbol, err)
	}
	posted, err := marginFigure(assets.Quote, "isolatedWallet", row.IsolatedWallet)
	if err != nil {
		return MarginStatus{}, false, fmt.Errorf("binance margin status on %s: %w", row.Symbol, err)
	}
	liq, stated, err := marginLiqPrice(assets.Base, assets.Quote, row.LiquidationPrice)
	if err != nil {
		return MarginStatus{}, false, fmt.Errorf("binance margin status on %s: %w", row.Symbol, err)
	}

	return MarginStatus{
		Instrument:     instrument,
		LiqPrice:       liq,
		LiqPriceStated: stated,
		Maintenance:    maintenance,
		PostedMargin:   posted,
	}, true, nil
}

// Compile-time proof that the codec satisfies the margin-status half too (§3.6
// rule 2).
var _ MarginStatusCodec = (*BinanceOrderCodec)(nil)
