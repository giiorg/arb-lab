package venue

import (
	"context"
	"fmt"

	"github.com/giiorg/arb-lab/internal/engine/venue/transport"
	"github.com/giiorg/arb-lab/internal/platform/apperror"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// This file is §4's Positions surface, venue-side: the enumeration §7.5's
// reconcile-only boot joins the write-ahead journal to.
//
// # It ENUMERATES, and the per-symbol form — which is what §4's signature reads
// # like — cannot answer the question §7.5 asks
//
// §7.5 alerts on "foreign orders or unexplained position deltas". A read keyed on
// an instrument can only ever ask about instruments this system already knows it
// traded, so the position it is structurally unable to see is the one on a symbol
// nothing here has heard of — which is precisely what a foreign order leaves
// behind. The enumeration is therefore the shape that can fail the invariant, and
// a per-symbol read would pass it while the account held a naked leg in a symbol
// nobody asked about.
//
// The cost of the enumeration is that "flat" becomes an ABSENCE from a list, and
// this repo has paid for absences that read as zeros. Two things bound it. The
// query is not scoped by symbol at all, so a symbol missing from the answer is
// missing because the venue says it holds nothing there — not because the caller
// spelled it wrong, which is the accident a symbol-scoped read invites. And an
// unanswerable query is an error rather than an empty list: both venues refuse a
// malformed scope (Binance -1121 BAD_SYMBOL "Invalid symbol."; Bybit refuses a
// linear enumeration with neither symbol nor settleCoin), so a scope this system
// got wrong does not come back looking flat.
//
// # The two venues encode DIRECTION in different layers, which is this slice's
// # first finding and the recurring shape at this seam
//
//	Binance USDⓈ-M   positionAmt is SIGNED, positionSide is "BOTH" in one-way mode
//	Bybit V5         size is "Position size, always positive", side is "Buy"/"Sell"
//	                 and "" for an empty position
//
// A decoder written once against either spelling reads the other venue's short as
// a long: on Binance because the sign is in the number the other venue never
// signs, on Bybit because the direction is in a field Binance does not use for it.
// It is §7.6's header asymmetry one surface over — two venues, one quantity, and
// only the venues' prose says which way it points — and the failure direction is
// the same one: nothing fails, the magnitudes agree, and the net delta this
// system asserts before it resumes trading is wrong by twice the position.
//
// # SPOT HAS NO POSITION SURFACE, and that is a refusal rather than a zero
//
// Neither venue has one: a spot balance is not a position, and both venues'
// position endpoints are derivatives-only (Binance's is /fapi, Bybit's rejects
// category=spot). The honest answer for a spot instrument is therefore
// ErrPositionSurfaceAbsent and not an empty enumeration, because an empty
// enumeration is indistinguishable from a flat account — and §7.5's whole
// question is whether the venue holds something this system cannot account for.
// What a spot leg's exposure has to be asserted against instead is stated at the
// consumer (oms/application.Joiner), because it is a rule and not a venue fact.

// PositionScope is which slice of an account's positions to enumerate.
//
// # The two venues do not enumerate the same width, and the caller's guarantee
// # is the NARROWER one
//
// Binance's /fapi/v3/positionRisk with no symbol returns every symbol on the
// USDⓈ-M family, whatever it settles in. Bybit's /v5/position/list requires a
// settleCoin when no symbol is given, so one call covers one settlement asset.
// Settle is therefore REQUIRED here and ignored by the Binance codec: a field one
// venue does not read is the price of a scope both can express, and the
// alternative — an optional field — would make the Bybit call fail at run time
// for a caller that read the Binance documentation.
//
// The consequence is a limit rather than a defect, and it belongs to whoever
// reasons from a clean enumeration: on Bybit, a position settled in an asset this
// scope did not name is not in the answer and is not evidence of anything.
type PositionScope struct {
	// Product is which derivative family to enumerate. Spot is refused — see the
	// file comment.
	Product ids.ProductCategory

	// Settle is the settlement asset, "USDT". Required, and read only by the
	// venues that scope on it.
	Settle fixedpoint.Asset
}

// Validate reports whether the scope names an enumeration a venue could serve.
func (s PositionScope) Validate() error {
	if err := s.Product.Validate(); err != nil {
		return err
	}
	if s.Settle == "" {
		return ErrPositionSettleUnset
	}
	return nil
}

func (s PositionScope) String() string { return fmt.Sprintf("%s/%s", s.Product, s.Settle) }

// Position is one instrument's net exposure at the venue, normalized.
type Position struct {
	// Instrument is which market. The venue and product come from the client and
	// the scope; only the symbol comes off the row.
	Instrument ids.VenueInstrument

	// Net is the SIGNED position size in the instrument's base asset: positive is
	// long, negative is short, and zero is a venue that reported the row while
	// holding nothing.
	//
	// One signed field rather than a side beside an unsigned size, which inverts
	// the rule OrderRequest and domain.Fill follow — and the inversion is the
	// point. Those types describe an INTENT, whose direction belongs to the
	// command that produced it; this one describes a NET, which is the result of
	// arithmetic over both directions and has no side to carry. A Side field here
	// would have to answer for a flat position, and both answers are wrong.
	Net fixedpoint.Quantity
}

// Validate reports whether the position is one a caller may reason from.
func (p Position) Validate() error {
	if err := p.Instrument.Validate(); err != nil {
		return fmt.Errorf("position instrument: %w", err)
	}
	if err := p.Net.Validate(); err != nil {
		return fmt.Errorf("position %s: %w", p.Instrument.Symbol, err)
	}
	return nil
}

// PositionReport is one enumeration's whole answer.
type PositionReport struct {
	// Positions are the rows whose symbol the caller declared, typed in that
	// instrument's base asset. A declared instrument that is ABSENT from this
	// slice is flat — see the file comment for why that reading is available
	// here and would not be under a symbol-scoped read.
	Positions []Position

	// Unrecognized are the symbols the venue reported that the caller did not
	// declare, in the venue's own spelling, sorted.
	//
	// # It carries no quantity, and the absence is the honest half of the type
	//
	// The money kernel refuses to build a Quantity without a base asset, and the
	// decomposition of a symbol this system does not trade is exactly what nobody
	// configured (see InstrumentAssets: splitting a symbol is a per-venue
	// convention and a wrong split mints a Money whose asset tag is a lie). So
	// the row cannot be typed, and the alternatives are to carry the venue's raw
	// decimal beside a typed field that is sometimes meaningless, or to fail the
	// whole read.
	//
	// Neither is needed, because the QUANTITY is not what the consumer branches
	// on. §7.5 routes a foreign order to operator clearance and does not
	// auto-correct it, so what the alert needs is the symbol; the size is
	// something the operator reads off the venue. Carrying a name for a thing
	// that must not be acted on automatically is the whole requirement.
	//
	// On Binance this slice also catches a foreign ORDER, because V3 returns
	// "only symbol that has position or open orders" — a symbol with a resting
	// order and no position is in this list. Bybit's position endpoint reports
	// positions only, so the same evidence does not exist there, and a consumer
	// must not read an empty Unrecognized as "no foreign orders" on either venue.
	Unrecognized []string
}

// PositionCodec is one venue's translation of the position surface.
//
// Embedded into OrderCodec rather than segregated, on OrderQueryCodec's
// argument: §7.5 makes the position read a precondition of trading resuming, so
// a venue whose derivative positions cannot be read is not a venue this system
// may hold a derivative leg on, and a separate interface would model a
// combination that must not exist.
type PositionCodec interface {
	// EncodePositions renders the enumeration for one scope.
	//
	// It returns ErrPositionSurfaceAbsent for a family the venue offers no
	// position surface for — spot on both venues.
	EncodePositions(scope PositionScope) (transport.Call, error)

	// DecodePositions reads the venue's answer.
	//
	// assets is the caller's declared instrument set: a row whose symbol is in it
	// is typed with that instrument's base asset, and one that is not becomes a
	// PositionReport.Unrecognized entry.
	//
	// An implementation MUST refuse a row that is not one-way — Binance's
	// positionSide other than BOTH, Bybit's positionIdx other than 0. §4.1 pins
	// one-way position mode as a refuse-to-trade posture check, and a hedge-mode
	// account answers this query with a LONG row and a SHORT row for the same
	// symbol, which net to a number no field on the row states.
	DecodePositions(scope PositionScope, assets map[ids.VenueInstrument]InstrumentAssets, resp transport.Response) (PositionReport, error)
}

// Positions enumerates the account's open positions for one scope.
//
// # The urgency is the caller's, and all three classes are legitimate
//
// §7.5's boot join is Safety traffic, a live risk read is Routine, and a
// scheduled reconciliation is neither — OrderByClientID's argument unchanged, and
// for the same reason the write path's narrowing does not carry: the narrowing
// there was a venue fact about orders, not a house style.
//
// # There is no venue check here, and there could not be one
//
// Place, OrderByClientID and Executions all reach route(), which refuses a
// reference for another venue. A PositionScope carries no venue at all — the
// venue comes from the client — so the mismatch this call could suffer is not
// expressible at this seam. It is checked one layer out, where a scope DOES carry
// a venue: oms/infrastructure/driven.VenuePositions, whose own venue must
// therefore be a fact and not a coincidence.
func (c *OrderClient) Positions(ctx context.Context, scope PositionScope, assets map[ids.VenueInstrument]InstrumentAssets, urgency ratelimit.Priority) (PositionReport, error) {
	op := string(c.codec.Venue()) + ".Positions"

	if err := scope.Validate(); err != nil {
		return PositionReport{}, apperror.Precondition(op, "scope", err)
	}
	if err := urgency.Validate(); err != nil {
		return PositionReport{}, apperror.Precondition(op, "urgency", err)
	}
	for instrument, a := range assets {
		if err := a.Validate(); err != nil {
			return PositionReport{}, apperror.Precondition(op, "assets",
				fmt.Errorf("%s: %w", instrument.Symbol, err))
		}
	}
	client, ok := c.transports[scope.Product]
	if !ok {
		return PositionReport{}, apperror.Precondition(op, "route",
			fmt.Errorf("scope %s: %w", scope, ErrOrderFamilyUnrouted))
	}

	call, err := c.codec.EncodePositions(scope)
	if err != nil {
		return PositionReport{}, apperror.Precondition(op, "encode", err)
	}
	call.Priority = urgency

	resp, err := client.Do(ctx, call)
	if err != nil {
		// Unwrapped, on the read path's standing rule: the transport already
		// classified the effect, and an enumeration that was not answered must not
		// be re-derived into one that came back empty.
		return PositionReport{}, err
	}

	report, err := c.codec.DecodePositions(scope, assets, resp)
	if err != nil {
		return PositionReport{}, apperror.Precondition(op, "decode", err)
	}
	for _, p := range report.Positions {
		if err := p.Validate(); err != nil {
			return PositionReport{}, apperror.Precondition(op, "decode", err)
		}
	}
	return report, nil
}

// positionInstrument finds the declared instrument a decoded row names.
//
// Shared by both codecs because the composition is the SCOPE's, not a venue's:
// the venue and product come from the client and the scope, and only the symbol
// is read off the wire. A codec assembling it independently is a codec that can
// disagree about which family a row belongs to, on a query whose whole purpose is
// attributing exposure to instruments.
//
// # It refuses a SECOND match rather than returning the first, and the mutation
// # harness is what settled that
//
// The obvious loop returns on the first entry that matches. Under it, deleting
// the product comparison — so that a perp row could match the spot instrument
// with the same symbol, which every cash-and-carry deployment declares — is
// caught only when Go's randomized map iteration happens to visit spot first.
// A test that fails half the time is the shape this repo has recorded on a
// `select`: the answer is not more trials, it is to remove the coin flip.
//
// The refusal is also a real configuration check rather than an artifact of
// making the mutation deterministic. ids.VenueInstrument carries a ContractType
// the key does not need, and domain.FillKey has already recorded what that
// costs: an unvalidated {linear_perp, inverse} is a DISTINCT map key from the
// {linear_perp, linear} naming the same real instrument. Two such keys in one
// assets table would attribute this row to whichever the iteration reached
// first, and the two carry different base assets exactly when the mistake
// matters.
func positionInstrument(venue ids.Venue, scope PositionScope, symbol string, assets map[ids.VenueInstrument]InstrumentAssets) (ids.VenueInstrument, InstrumentAssets, bool, error) {
	var (
		found      ids.VenueInstrument
		foundAsset InstrumentAssets
		matches    int
	)
	for instrument, a := range assets {
		if instrument.Venue != venue || instrument.Product != scope.Product || instrument.Symbol != symbol {
			continue
		}
		matches++
		found, foundAsset = instrument, a
	}
	switch matches {
	case 0:
		return ids.VenueInstrument{}, InstrumentAssets{}, false, nil
	case 1:
		return found, foundAsset, true, nil
	}
	return ids.VenueInstrument{}, InstrumentAssets{}, false, fmt.Errorf(
		"%d declared instruments match %s %s on %s: %w",
		matches, scope.Product, symbol, venue, ErrPositionInstrumentAmbiguous)
}
