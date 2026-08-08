package venue

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/giiorg/arb-lab/internal/engine/venue/transport"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// This file is Binance's margin-act history surface: `GET
// /fapi/v1/positionMargin/history` (Profiles §4).
//
// # The response shape is VERBATIM, and it carries a field the profile did not
//
// Binance's own documentation site answers a fetcher with a WAF challenge, so
// this repo's row shape came from the spec-generated connector's mocked response
// for this exact endpoint — a raw-text source rather than a summarized fetch:
//
//	{"symbol":"BTCUSDT","type":1,"deltaType":"USER_ADJUST","amount":"23.36332311",
//	 "asset":"USDT","time":1578047897183,"positionSide":"BOTH"}
//
// `deltaType` is the field, and it is not cosmetic. §7.1 words the match as
// "symbol + type + amount + time-window", which says nothing about WHY the venue
// changed the margin — and this system already knows there is a second reason,
// because §4.1's posture check reads `isAutoAddMargin` per configured symbol
// precisely because the venue can add margin on its own. A match blind to the
// cause would read such a row as evidence of this system's act.
//
// One value of the field is verified and the rest are not, so the decode is
// two-valued: the verified one, and everything else. See MarginActCauseOther for
// why the unrecognised case is REPORTED rather than dropped.
//
// # `type` is a request parameter as well as a response field
//
// So the direction is narrowed at the venue and re-checked on every row. That is
// not belt-and-braces: the parameter narrows what is fetched (and a fetch that
// silently ignored it would hand the consumer reductions to match against a
// top-up), while the row's own field is what the consumer's rule reads.

// EncodeMarginActs implements MarginActCodec.
func (c *BinanceOrderCodec) EncodeMarginActs(instrument ids.VenueInstrument, w MarginActWindow, adds bool) (transport.Call, error) {
	if err := instrument.Validate(); err != nil {
		return transport.Call{}, err
	}
	if err := w.Validate(); err != nil {
		return transport.Call{}, err
	}
	path, cost, err := binanceMarginActsRoute(instrument.Product)
	if err != nil {
		return transport.Call{}, err
	}
	return transport.Call{
		Op:     string(c.venue) + ".MarginActs",
		Method: "GET",
		Path:   path,
		Params: transport.Params{
			{Key: "symbol", Value: instrument.Symbol},
			{Key: "type", Value: fmt.Sprintf("%d", binanceMarginActType(adds))},
			{Key: "startTime", Value: fmt.Sprintf("%d", w.From.UnixMilli())},
			// Sent as the window's OPEN upper edge without adjustment. Binance's
			// endTime is inclusive, so this can fetch one row the consumer's
			// half-open match then excludes — which is the harmless direction:
			// the fetch may be wider than the match, never narrower, because the
			// match re-applies both edges and a row it never received is a row it
			// cannot count.
			{Key: "endTime", Value: fmt.Sprintf("%d", w.To.UnixMilli())},
			{Key: "limit", Value: fmt.Sprintf("%d", binanceMarginActsPageLimit)},
		},
		Family: instrument.Product,
		Cost:   cost,
	}, nil
}

// binanceMarginActType renders the direction as Binance's `type`: 1 adds, 2
// reduces (Profiles §4).
//
// One function for both the request parameter and the response reading, so the
// two cannot disagree about which integer means which direction — the mapping is
// stated once and every use of it is the same statement.
func binanceMarginActType(adds bool) int64 {
	if adds {
		return 1
	}
	return 2
}

// binanceMarginActsRoute is the path and charged cost per family.
//
// Exhaustive over the enum-like (§3.6 rule 10). Spot is refused with the
// surface-absent sentinel rather than the unrouted error, on
// binanceMarginStatusRoute's argument: no venue posts isolated margin against a
// spot holding, so reporting it as a routing gap invites a reader to fix it by
// adding a route.
func binanceMarginActsRoute(p ids.ProductCategory) (string, ratelimit.Cost, error) {
	switch p {
	case ids.ProductSpot:
		return "", ratelimit.Cost{}, fmt.Errorf("binance spot: %w", ErrPositionSurfaceAbsent)
	case ids.ProductLinearPerp:
		return "/fapi/v1/positionMargin/history", ratelimit.Cost{Weight: binanceMarginActsWeight}, nil
	case ids.ProductInversePerp:
		return "", ratelimit.Cost{}, fmt.Errorf("%s: %w", p, ErrOrderFamilyUnrouted)
	}
	return "", ratelimit.Cost{}, fmt.Errorf("%s: %w", p, ids.ErrProductUnknown)
}

// binanceMarginActWire is one history row, whole. Every field the venue publishes
// is declared, because every one of them is read: four make the match, the asset
// falsifies the margin-asset premise, and `positionSide` enforces the one-way pin.
type binanceMarginActWire struct {
	Symbol       string `json:"symbol"`
	Type         int64  `json:"type"`
	DeltaType    string `json:"deltaType"`
	Amount       string `json:"amount"`
	Asset        string `json:"asset"`
	Time         int64  `json:"time"`
	PositionSide string `json:"positionSide"`
}

// DecodeMarginActs implements MarginActCodec.
func (c *BinanceOrderCodec) DecodeMarginActs(instrument ids.VenueInstrument, assets InstrumentAssets, resp transport.Response) ([]MarginAct, error) {
	var wire []binanceMarginActWire
	if err := json.Unmarshal(resp.Body, &wire); err != nil {
		return nil, fmt.Errorf("binance margin acts: %w", err)
	}
	if len(wire) >= binanceMarginActsPageLimit {
		// A full page over a window a propagation bound wide would mean hundreds
		// of margin adjustments on one symbol within seconds. Refused rather than
		// paged: the consumer's rule counts matching rows, and a truncated page
		// can only UNDERCOUNT them — which turns the duplicate case §7.1 calls
		// Ambiguous into a single-row Applied.
		return nil, fmt.Errorf("binance margin acts on %s: %d rows: %w",
			instrument.Symbol, len(wire), ErrMarginActPageFull)
	}

	out := make([]MarginAct, 0, len(wire))
	for i, row := range wire {
		if row.PositionSide != binanceOneWayPositionSide {
			// The §4.1 pin, checked here as well as on the position surface. A
			// hedge-mode account records acts per leg, and a leg's top-up matched
			// against a whole-position op would attribute one leg's act to both.
			return nil, fmt.Errorf("binance margin act %d on %s is positionSide %q, not %q: %w",
				i, row.Symbol, row.PositionSide, binanceOneWayPositionSide, ErrPositionModeNotOneWay)
		}
		if row.Asset != string(assets.Quote) {
			// The same falsifiable premise the margin-status codec checks: this
			// venue states which asset its figures are in, so the linear-perp
			// "margin is the quote asset" premise is checkable rather than
			// assumed. Typing the amount in the declared quote regardless would
			// make a foreign-asset row match this op's magnitude numerically.
			return nil, fmt.Errorf("binance margin act %d on %s is in %q, declared quote is %q: %w",
				i, row.Symbol, row.Asset, assets.Quote, ErrMarginAssetsDisagree)
		}

		var adds bool
		switch row.Type {
		case binanceMarginActType(true):
			adds = true
		case binanceMarginActType(false):
			adds = false
		default:
			// An undeclared direction. Refused rather than defaulted, because both
			// defaults are wrong in a way that matters: read as an add it becomes a
			// candidate match for a top-up, and read as a reduce it silently
			// disappears from a top-up's match set.
			return nil, fmt.Errorf("binance margin act %d on %s has type %d: %w",
				i, row.Symbol, row.Type, ErrMarginActUnreadable)
		}

		amount, err := fixedpoint.ParseMoney(assets.Quote, row.Amount)
		if err != nil {
			return nil, fmt.Errorf("binance margin act %d on %s amount %q: %w", i, row.Symbol, row.Amount, err)
		}
		if row.Time <= 0 {
			// A zero or negative stamp cannot be placed in a window, so the row
			// would be a candidate for every op on the symbol.
			return nil, fmt.Errorf("binance margin act %d on %s has time %d: %w",
				i, row.Symbol, row.Time, ErrMarginActUnreadable)
		}

		out = append(out, MarginAct{
			Symbol: row.Symbol,
			Amount: amount,
			Adds:   adds,
			At:     time.UnixMilli(row.Time).UTC(),
			Cause:  binanceMarginActCause(row.DeltaType),
		})
	}
	return out, nil
}

// binanceMarginActCause maps `deltaType` onto the two-member cause.
//
// Everything that is not the one verified value lands in MarginActCauseOther,
// including an EMPTY string. That last case is the one worth stating: an absent
// `deltaType` is not a manual adjustment with the field omitted, it is a row
// whose cause this system does not know — and the consumer's rule for an
// unattributable act is the conservative one, so the unknown lands where it can
// only refuse to conclude Applied.
func binanceMarginActCause(deltaType string) MarginActCause {
	if deltaType == binanceUserAdjustDeltaType {
		return MarginActCauseUserAdjust
	}
	return MarginActCauseOther
}

// PublishesMarginActs implements MarginActCodec. Binance keeps a margin-change
// history and this system reads it.
func (c *BinanceOrderCodec) PublishesMarginActs() bool { return true }

var _ MarginActCodec = (*BinanceOrderCodec)(nil)
