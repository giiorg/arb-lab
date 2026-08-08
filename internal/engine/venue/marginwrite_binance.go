package venue

import (
	"encoding/json"
	"fmt"

	"github.com/giiorg/arb-lab/internal/engine/venue/transport"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// This file is Binance's margin top-up: `POST /fapi/v1/positionMargin` with
// `type` 1 (Profiles §4).
//
// # The acknowledgement names the ACT and not the MARKET
//
// Verbatim from Profiles §4: the endpoint "responds `{amount, code, msg, type}`
// — no op id". There is no `symbol` on it, so this decoder CANNOT check that the
// venue answered about the instrument the call named. What it can check is the
// pair that describes what was done — the direction and the amount — and it
// checks both.
//
// What the missing symbol costs is worth stating rather than leaving implicit.
// The request carries `symbol` as a signed parameter, so a wrong market would
// have to come from this codec's own encoder, and the encode/decode pair is
// tested against one instrument at a time. The realistic accident it cannot
// catch is a venue that ignores the parameter — which would show up as a top-up
// that never appears on the position, resolved by §7.1's evidence path as
// NotApplied after the propagation bound. Late and safe, which is the direction
// this system takes when it cannot check something.
//
// # `code` is NOT read here
//
// bybitAckWire's rule at the other venue: transport.ClassifyResponse has already
// applied BinanceEnvelope, and that reader treats only a NEGATIVE code as an
// error precisely because a non-negative `code` appears on Binance responses
// that mean success — this endpoint's `200` being one of them. A second check
// here would be a weaker copy of a rule that already ran, and the direction it
// would fail in is refusing a call that worked.
//
// # The charged weight is UNVERIFIED, and the ceiling that makes overcharging
// # affordable is §7.1's own serialization
//
// binanceMarginActsWeight's situation (task #214) with different arithmetic. The
// documentation site answers a fetcher with an AWS WAF challenge (Profiles §9's
// ⚠ block, the wall #180 and #217 are open against), and the spec-generated
// connector carries response shapes and no weights, so no figure for this path is
// verified. Rather than transcribe a plausible one this charges the repo's
// largest `fapi` weight.
//
// The act-history read could call that "nearly free" because it is issued once
// per in-flight op at boot and never in steady state. This one IS on the incident
// path, so the argument has to be made again rather than inherited: §7.1 permits
// at most ONE margin op in flight per position, so the worst case is one call per
// configured perpetual per propagation bound — tens of weight against a
// four-figure per-minute budget, from traffic §7.6 already puts in the reserved
// Safety band. Undercounting is what ends in a §7.6 ban; overcounting here costs
// a fraction of a percent of the budget it protects.
const binanceMarginWriteWeight = 20

// binanceMarginAddType is Binance's `type` for the add direction.
//
// It is binanceMarginActType(true) rather than a second literal `1`, and the
// indirection is the point: the request parameter, the history row's direction
// and this echo check are three readings of ONE mapping, and a repo that spells
// it three times has three places for it to drift. binanceMarginActType's own
// doc states the mapping (Profiles §4: 1=add, 2=reduce).
func binanceMarginAddType() int64 { return binanceMarginActType(true) }

// EncodeAddIsolatedMargin implements MarginWriteCodec.
//
// # `positionSide` is not sent, and the omission is §4.1's pin being trusted
// # where it is already asserted
//
// The parameter is optional and defaults to `BOTH` in one-way mode, which §4.1
// pins and §7.5 asserts at boot. Sending it unconditionally is the shape
// EncodePlace already rejected for `reduceOnly`: a hedge-mode account would then
// have EVERY call rejected rather than the boot gate refusing the account, which
// moves the diagnosis from the posture check to a venue error message.
//
// It is also the half this venue's acknowledgement cannot echo back, so there
// would be nothing to check it against. The act-history decoder DOES check
// `positionSide` on every row (marginacts_binance.go), which is where the pin is
// re-asserted against this venue's own record of what it did.
func (c *BinanceOrderCodec) EncodeAddIsolatedMargin(instrument ids.VenueInstrument,
	assets InstrumentAssets, amount fixedpoint.Money) (transport.Call, error) {

	if err := instrument.Validate(); err != nil {
		return transport.Call{}, err
	}
	if err := checkMarginWriteAmount(assets, amount); err != nil {
		return transport.Call{}, err
	}
	path, cost, err := binanceMarginWriteRoute(instrument.Product)
	if err != nil {
		return transport.Call{}, err
	}
	return transport.Call{
		Op:     string(c.venue) + ".AddIsolatedMargin",
		Method: "POST",
		Path:   path,
		Params: transport.Params{
			{Key: "symbol", Value: instrument.Symbol},
			{Key: "amount", Value: amount.Decimal()},
			{Key: "type", Value: fmt.Sprintf("%d", binanceMarginAddType())},
		},
		Family: instrument.Product,
		Cost:   cost,
	}, nil
}

// binanceMarginWriteRoute is the path and charged cost per family.
//
// Exhaustive over the enum-like (§3.6 rule 10). Spot is refused with the
// surface-absent sentinel on binanceMarginActsRoute's argument: no venue posts
// isolated margin against a spot holding, so reporting it as a routing gap
// invites a reader to close it by adding a route.
func binanceMarginWriteRoute(p ids.ProductCategory) (string, ratelimit.Cost, error) {
	switch p {
	case ids.ProductSpot:
		return "", ratelimit.Cost{}, fmt.Errorf("binance spot: %w", ErrPositionSurfaceAbsent)
	case ids.ProductLinearPerp:
		return "/fapi/v1/positionMargin", ratelimit.Cost{Weight: binanceMarginWriteWeight}, nil
	case ids.ProductInversePerp:
		return "", ratelimit.Cost{}, fmt.Errorf("%s: %w", p, ErrOrderFamilyUnrouted)
	}
	return "", ratelimit.Cost{}, fmt.Errorf("%s: %w", p, ids.ErrProductUnknown)
}

// binanceMarginWriteWire is the acknowledgement, narrowed to the two fields that
// describe what the venue did.
//
// `amount` is a json.Number rather than a string or a float. Binance publishes it
// as a JSON NUMBER on this endpoint — unlike every decimal on the read surfaces,
// which are quoted strings — and json.Number keeps the literal text so it reaches
// fixedpoint's parser exactly as the venue wrote it. A float64 would be a
// `nofloat` violation and, worse, a value that has already lost the digits this
// comparison is about.
type binanceMarginWriteWire struct {
	Amount json.Number `json:"amount"`
	Type   int64       `json:"type"`
}

// DecodeAddIsolatedMargin implements MarginWriteCodec.
func (c *BinanceOrderCodec) DecodeAddIsolatedMargin(instrument ids.VenueInstrument,
	assets InstrumentAssets, amount fixedpoint.Money, resp transport.Response) error {

	var wire binanceMarginWriteWire
	if err := json.Unmarshal(resp.Body, &wire); err != nil {
		return fmt.Errorf("binance margin top-up on %s: %w", instrument.Symbol, err)
	}

	if wire.Type != binanceMarginAddType() {
		// The venue says it did the OPPOSITE act. Refused rather than accepted as
		// an acknowledgement, because the evidence path would then look for a
		// top-up in the history and find a reduction of the same magnitude — a
		// row it excludes by direction, which reads as the venue having recorded
		// nothing.
		return fmt.Errorf("binance acknowledges a margin change of type %d on %s, asked for type %d: %w",
			wire.Type, instrument.Symbol, binanceMarginAddType(), ErrMarginWriteUnacknowledged)
	}

	echoed, err := fixedpoint.ParseMoney(assets.Quote, wire.Amount.String())
	if err != nil {
		return fmt.Errorf("binance margin top-up on %s amount %q: %w", instrument.Symbol, wire.Amount.String(), err)
	}
	cmp, err := echoed.Cmp(amount)
	if err != nil {
		return fmt.Errorf("binance margin top-up on %s: %w", instrument.Symbol, err)
	}
	if cmp != 0 {
		// Exact equality, and the failure direction is deliberate. A margin op is
		// a transfer of a stated quantity rather than a computation the venue
		// performs, so a near miss is not rounding — and reporting it here is
		// strictly better than letting the posted-margin comparison discover it,
		// which would raise §7.1's Ambiguous (a §9 incident needing an operator)
		// for a discrepancy the response already named.
		return fmt.Errorf("binance acknowledges %s of margin on %s, asked for %s: %w",
			echoed, instrument.Symbol, amount, ErrMarginWriteUnacknowledged)
	}
	return nil
}

var _ MarginWriteCodec = (*BinanceOrderCodec)(nil)
