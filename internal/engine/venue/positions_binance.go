package venue

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/giiorg/arb-lab/internal/engine/venue/transport"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// This file is Binance's position surface (Profiles §9).
//
// # V2 and V3 are not two spellings of one endpoint, and choosing wrongly makes
// # an ABSENCE mean the opposite thing
//
// Both exist, both are GET, both take the same three parameters, and their rows
// differ by a few fields nothing here reads. What differs is which rows come
// back, and the changelog states it in one line (2024-07-24, verbatim):
// "GET /fapi/v3/positionRisk: Compared to GET /fapi/v2/positionRisk, this
// endpoint only returns symbols that the user has positions or open orders in."
//
// So on V2 a symbol missing from the answer is a symbol that does not exist on
// the family, while on V3 it is a symbol the account is flat in. This surface is
// read to decide whether the venue holds exposure nothing here can account for,
// which makes "flat" a conclusion rather than a gap — and V3 is the version whose
// silence carries it. V2 would have to be read as a list to filter, and every row
// it returns for a symbol we never traded would be a false foreign position.
//
// V3 is also the version that is not deprecation-announced.
//
// # The charged weight is the CONSERVATIVE reading of an unverified figure
//
// This endpoint's published weight is not settled (task #180, Profiles §9).
// THREE readings are in play, none of them read directly — a flat 5, which is
// what was recorded when this code was written, and two CONDITIONAL ones from
// two different AI summarizers on 2026-08-05, "5 with symbol, 20 without" and
// "1 with symbol, 10 without". `developers.binance.com` answers every headless
// fetcher with a WAF challenge, so nobody here has read the line directly.
//
// The readings are not symmetric for us, because this call deliberately sends no
// symbol (see EncodePositions): every conditional reading charges us its
// expensive arm. Undercounting an IP budget, against the limit whose breach is
// the ban §7.6 exists to avoid, is the failure this figure decides — so the cost
// charged is 20, the MAXIMUM over the candidates rather than the newest of them,
// which makes it correct-or-pessimistic under each (exact at 5/20, over by 10 at
// 1/10, over by 15 at a flat 5) once per reconcile pass.
//
// The two summaries disagree by a factor of two on the same line, which is the
// argument for the max rather than against the figure: what they AGREE on is the
// conditional shape, and the shape is the half that binds a symbol-less caller.
//
// Reading the published line is therefore a LOOSENING trigger, never a
// correctness prerequisite: if the venue's own page states a lower figure for
// the symbol-less call, this drops to it with the citation. Nothing else in the
// system depends on which it is, and no further unread candidate below 20 can
// change this line.
//
// The related fact about the SOURCE is recorded in Profiles §9 rather than
// resolved here: the V2 endpoint page and the venue's changelog disagreed for
// two years (2024-08-07, verbatim: "GET /fapi/v2/positionRisk: 5->10", against a
// page that went on saying 5), which is why a figure from either alone is
// treated as a claim rather than as a fact.
//
// # The direction is in the SIGN, and one-way mode is what makes that readable
//
// A row carries positionAmt and positionSide. §4.1 pins one-way position mode as
// a refuse-to-trade posture check, and in one-way mode positionSide is "BOTH" and
// the net is the sign of positionAmt. In hedge mode the same symbol answers with
// a LONG row and a SHORT row, whose net is a subtraction no field states — so a
// decoder that ignored positionSide would report one leg of a hedge-mode account
// as the whole position. This one refuses the row instead, which is the same
// answer §4.1 gives one layer up and arrives here first, because this query runs
// before the posture check has anything to compare against.

// EncodePositions implements PositionCodec.
//
// The symbol parameter is deliberately NOT sent. It is optional on this endpoint,
// and sending it would narrow the answer to instruments this system already knows
// about — which is exactly the set that cannot contain a foreign position. See
// positions.go.
func (c *BinanceOrderCodec) EncodePositions(scope PositionScope) (transport.Call, error) {
	if err := scope.Validate(); err != nil {
		return transport.Call{}, err
	}
	path, cost, err := binancePositionsRoute(scope.Product)
	if err != nil {
		return transport.Call{}, err
	}
	return transport.Call{
		Op:     string(c.venue) + ".Positions",
		Method: "GET",
		Path:   path,
		Family: scope.Product,
		Cost:   cost,
	}, nil
}

// binancePositionsRoute is the path and the venue-documented cost per family.
//
// Exhaustive over the enum-like (§3.6 rule 10). Spot is refused with its own
// sentinel rather than with the unrouted error the other families use: a spot
// instrument has no position surface ANYWHERE, on either venue, and reporting
// that as a routing gap would invite a reader to fix it by adding a route.
func binancePositionsRoute(p ids.ProductCategory) (string, ratelimit.Cost, error) {
	switch p {
	case ids.ProductSpot:
		return "", ratelimit.Cost{}, fmt.Errorf("binance spot: %w", ErrPositionSurfaceAbsent)
	case ids.ProductLinearPerp:
		// 20, not 5: the conservative arm of an unverified conditional weight,
		// charged for the symbol-less enumeration this codec always sends. See
		// the file comment — a direct read of the published line loosens it.
		return "/fapi/v3/positionRisk", ratelimit.Cost{Weight: 20}, nil
	case ids.ProductInversePerp:
		return "", ratelimit.Cost{}, fmt.Errorf("%s: %w", p, ErrOrderFamilyUnrouted)
	}
	return "", ratelimit.Cost{}, fmt.Errorf("%s: %w", p, ids.ErrProductUnknown)
}

// binancePositionWire is one row of the V3 answer, narrowed to what §7.5 asks
// for.
//
// The full row carries entryPrice, breakEvenPrice, markPrice, unRealizedProfit,
// liquidationPrice, isolatedMargin, notional, marginAsset, isolatedWallet,
// initialMargin, maintMargin, positionInitialMargin, openOrderInitialMargin, adl,
// bidNotional, askNotional and updateTime. None of them is read: §7.5 asserts a
// net-delta invariant, and every other field is a number this system would then
// have two sources for.
type binancePositionWire struct {
	Symbol       string `json:"symbol"`
	PositionSide string `json:"positionSide"`
	PositionAmt  string `json:"positionAmt"`
}

// binanceOneWayPositionSide is what positionSide reads in one-way mode.
const binanceOneWayPositionSide = "BOTH"

// DecodePositions implements PositionCodec.
func (c *BinanceOrderCodec) DecodePositions(scope PositionScope, assets map[ids.VenueInstrument]InstrumentAssets, resp transport.Response) (PositionReport, error) {
	var wire []binancePositionWire
	if err := json.Unmarshal(resp.Body, &wire); err != nil {
		return PositionReport{}, fmt.Errorf("binance positions: %w", err)
	}

	var report PositionReport
	seen := make(map[string]struct{}, len(wire))
	for i, row := range wire {
		if row.Symbol == "" {
			return PositionReport{}, fmt.Errorf("binance position %d carries no symbol: %w",
				i, ErrPositionsUnreadable)
		}
		if row.PositionSide != binanceOneWayPositionSide {
			// Hedge mode, or a venue that has started answering differently.
			// Either way the net for this symbol is not on this row, and §4.1
			// makes the account posture that produces it refuse-to-trade.
			return PositionReport{}, fmt.Errorf("binance position %d on %s is positionSide %q, not %q: %w",
				i, row.Symbol, row.PositionSide, binanceOneWayPositionSide, ErrPositionModeNotOneWay)
		}
		if _, dup := seen[row.Symbol]; dup {
			// Two one-way rows for one symbol. Summing them would be this
			// decoder inventing an aggregation rule the venue does not
			// document; taking either would silently drop half the exposure.
			return PositionReport{}, fmt.Errorf("binance answers twice for %s: %w",
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
		net, err := fixedpoint.ParseQuantity(a.Base, row.PositionAmt)
		if err != nil {
			return PositionReport{}, fmt.Errorf("binance position %d on %s positionAmt %q: %w",
				i, row.Symbol, row.PositionAmt, err)
		}
		report.Positions = append(report.Positions, Position{Instrument: instrument, Net: net})
	}

	sortPositionReport(&report)
	return report, nil
}

// sortPositionReport orders both halves by symbol.
//
// Shared with the Bybit codec because the reason is the consumer's rather than a
// venue's: §7.5 turns this answer into an operator alert, and an alert that names
// its causes in whatever order the venue listed them is one an operator learns to
// distrust. Recovery.Unresolved sorts for the same reason and says the same
// thing.
func sortPositionReport(r *PositionReport) {
	sort.Slice(r.Positions, func(i, j int) bool { return r.Positions[i].Instrument.Symbol < r.Positions[j].Instrument.Symbol })
	sort.Strings(r.Unrecognized)
}

// Compile-time proof that the codec satisfies the position half too (§3.6
// rule 2).
var _ PositionCodec = (*BinanceOrderCodec)(nil)
