package venue

import (
	"fmt"

	"github.com/giiorg/arb-lab/internal/engine/venue/transport"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// This file is Bybit's answer to the margin-act history surface, and the answer
// is that there is none. It is a RULING taken in Profiles §4, not an adapter
// nobody has written yet, so it is implemented rather than omitted.
//
// # What the venue does and does not state
//
// `POST /v5/position/add-margin` answers with a position snapshot carrying no
// operation id: the only resolution signal it offers is the `positionIM` delta
// against a before-state, which is exactly the evidence task #212 built.
//
// The tempting second source is `GET /v5/account/transaction-log`, whose records
// DO carry a unique `id`. Profiles §4 refuses it in the venue's own terms: the
// ~100-value `type` enum (`TRANSFER_IN/OUT`, `TRADE`, `SETTLEMENT`,
// `LIQUIDATION`, `ADL`, …) "contains no value for a manual position-margin
// add/reduce", and whether a top-up emits a record at all is "undocumented; MUST
// NOT be relied on".
//
// That last clause is what makes building on it unsafe rather than merely
// unsupported, and the direction is the dangerous one. A matcher over the
// transaction log would read "no record" as "the act did not happen" — so a
// top-up that DID apply and emitted nothing would resolve NotApplied, which is
// §7.1's named worst case: "the dangerous direction is assuming a top-up applied
// when it did not", reached from the mirror side.
//
// # So Bybit resolves from ONE source, and that is the whole contract there
//
// domain.MarginOp's delta arm is not a fallback on this venue; it is what §7.1
// specifies for it. The consumer says so positively — NoMarginActHistory — and
// the reason this file exists at all is to make that a statement something with a
// name makes, rather than an inference drawn from a missing method.

// PublishesMarginActs implements MarginActCodec.
//
// False, and the two encoder/decoder methods below are consequently unreachable
// through OrderClient.MarginActs, which checks this first. They refuse loudly
// anyway: a later caller reaching them directly would otherwise get an empty
// slice, and an empty history from a venue that publishes one is the observation
// that turns a moved posted margin into an operator incident.
func (c *BybitOrderCodec) PublishesMarginActs() bool { return false }

// EncodeMarginActs implements MarginActCodec by refusing.
func (c *BybitOrderCodec) EncodeMarginActs(instrument ids.VenueInstrument, w MarginActWindow, adds bool) (transport.Call, error) {
	return transport.Call{}, fmt.Errorf("bybit %s: %w", instrument.Symbol, ErrMarginActSurfaceAbsent)
}

// DecodeMarginActs implements MarginActCodec by refusing.
//
// It refuses rather than returning an empty slice, which is the whole point:
// nil, nil here would be indistinguishable from "Bybit answered and recorded
// nothing", and that reading resolves a correct top-up as Ambiguous.
func (c *BybitOrderCodec) DecodeMarginActs(instrument ids.VenueInstrument, assets InstrumentAssets, resp transport.Response) ([]MarginAct, error) {
	return nil, fmt.Errorf("bybit %s: %w", instrument.Symbol, ErrMarginActSurfaceAbsent)
}

var _ MarginActCodec = (*BybitOrderCodec)(nil)
