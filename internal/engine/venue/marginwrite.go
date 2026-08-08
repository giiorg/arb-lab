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

// This file is §4's `MarginGateway.AddIsolatedMargin`, venue-side: core §7.1's
// margin TOP-UP, the one state-changing call this system makes that is not an
// order (task #215). Binance `POST /fapi/v1/positionMargin {type:1}`, Bybit
// `POST /v5/position/add-margin` with a positive `margin` (Profiles §4).
//
// # It is the SECOND non-idempotent write in the system, and the first one's
// # discipline is not inherited automatically
//
// Place has a ClientOrderID both sides agree on, so §5.3 can ask the venue "what
// happened to THIS order" afterwards. Neither venue offers an identity for a
// margin operation — Bybit answers with a position snapshot and Binance's own
// history rows carry no id (Profiles §4) — so there is nothing to ask about
// later, and the whole recovery story is the journaled before-state
// oms/domain.MarginOp holds. What that costs THIS layer is a stricter contract
// on the acknowledgement: a Place ack may legitimately be empty, while an
// unreadable margin ack has to be reported as EffectApplied, because the act is
// the thing that cannot be looked up.
//
// # Only the ADD direction exists here, and the omission is a ruling
//
// §4 declares `ReduceIsolatedMargin` and `TransferCollateral` beside it. Neither
// has a consumer: §7.1's ladder tops up on the Rebalance band and de-risks by
// delevering, never by pulling margin back out, and the staged float lives in
// §7.2's unbuilt `treasury`. Encoding them now would be the enum-member-with-no-
// producer shape this repo has already paid for — the compiler is satisfied, the
// codec is fixture-tested against a doc example, and the first real caller finds
// a path no test ever exercised against a live account. They arrive with their
// consumers.
//
// The direction is consequently NOT a parameter. oms/domain.MarginOp carries a
// SIGNED delta because the venues' own APIs are signed, and the narrowing back to
// a magnitude happens here, at the seam that renders it: a `type` selected by a
// sign this layer computed would be one more place the sign convention lives.

// MarginWriteCodec is one venue's translation of the margin top-up surface.
//
// # Embedded into OrderCodec, on MarginStatusCodec's argument rather than
// # MarginActCodec's
//
// The two embedding decisions in this package come apart on whether the missing
// capability describes a supportable deployment. A venue that keeps no margin-act
// history is one whose ops resolve from a single evidence source — ordinary, and
// so MarginActCodec stands alone. A venue that cannot be TOLD to post margin is
// one where §7.1's Rebalance band has no action at all: the ladder's middle rung
// disappears and every breach escalates straight to a PackageDelever. That is not
// a smaller deployment, it is a different safety story, so a separate interface
// would model a combination this system must not run in.
type MarginWriteCodec interface {
	// EncodeAddIsolatedMargin renders the top-up as one signed call.
	//
	// amount is the MAGNITUDE to add, strictly positive and in the instrument's
	// quote asset. The assets are taken so that premise is falsifiable here
	// rather than at the first arithmetic three layers away — the same check
	// marginFigure exists for on the read side, applied to the one figure this
	// system ORIGINATES.
	EncodeAddIsolatedMargin(instrument ids.VenueInstrument, assets InstrumentAssets, amount fixedpoint.Money) (transport.Call, error)

	// DecodeAddIsolatedMargin reads the venue's acknowledgement and reports
	// whether it acknowledges THIS top-up.
	//
	// It returns no value, and that is the venue fact rather than a narrowing:
	// neither response carries an operation identity, so there is nothing to
	// carry out. What it does is CHECK — that the venue echoed the market, the
	// direction and the amount this call asked for — and an implementation MUST
	// make those checks rather than reading the status alone. A top-up that
	// applied to a different amount than we asked for is an act this system
	// cannot attribute, and the acknowledgement is the only place the difference
	// is visible: the posted-margin evidence path would see a movement matching
	// nothing and report §7.1's Ambiguous, which is a §9 incident raised for a
	// fact the response already stated.
	//
	// The request's own amount is passed in for that comparison. It is NOT read
	// back out of the response and it is not defaulted — an echo checked against
	// itself is the shared-constant vacuity this repo has already paid for.
	//
	// # The two venues acknowledge in different vocabularies, and NEITHER lets
	// # both halves be checked
	//
	// There is no shared echo helper here, unlike marginLiqPrice on the read
	// side, and the reason is a venue fact rather than a structuring choice:
	//
	//	Binance answers {amount, code, msg, type} — the ACT, with no symbol
	//	Bybit answers a position snapshot — the SUBJECT, with no amount
	//
	// So Binance can be asked "did you move the amount I named, in the direction
	// I named" and cannot be asked "on which market"; Bybit is the mirror image.
	// Each codec checks what its own venue states, and what each cannot check is
	// recorded in its file rather than left to look like an omission. The one
	// thing a caller must NOT do to close Bybit's half is read `positionIM` off
	// this response as an after-state — see marginwrite_bybit.go.
	DecodeAddIsolatedMargin(instrument ids.VenueInstrument, assets InstrumentAssets, amount fixedpoint.Money, resp transport.Response) error
}

// marginWriteRef is the placeholder client order id this surface's routing check
// carries — marginStatusReadRef's argument, unchanged: route validates per
// order, and a margin op is not about one order.
const marginWriteRef ids.ClientOrderID = "margin-top-up"

// AddIsolatedMargin posts additional isolated margin against one perpetual
// position, EXACTLY ONCE.
//
// # Place's submit-once rule, and here it binds harder
//
// §7.1: "blind delta retries are prohibited — the dangerous direction is
// assuming a top-up applied when it did not". Nothing here retries and
// SignedClient is built without retries or redirects, so a second attempt is a
// decision taken upstream with the journal in hand. The difference from an order
// is that an order can be RESOLVED by asking the venue about its id, and this
// cannot be resolved at all except by the before/after comparison the caller
// journaled first.
//
// # The urgency is the caller's argument, and Routine is refused
//
// OrderRequest.Validate's refusal, applied to its mirror. §7.6's Routine class is
// "the reads that keep the risk view live", and a top-up is not a read under any
// §5.3 command — so Routine here is a value that passes a type check and states
// nothing true, while both neighbours are a real decision: rounding up spends the
// safety reserve and rounding down sheds the remedy at 70% of budget, during
// precisely the incident the reserve exists for. The class is not PINNED here
// because §4 gives this package "raw clients and normalized DTOs" and §7.6's shed
// class is policy; the OMS's adapter states it, once, as a constant.
//
// # Every path out carries an apperror.Effect
//
// Place's rule, and the decode arm is the one that matters most: a 2xx followed
// by an unreadable acknowledgement is EffectApplied — the margin may well have
// moved and our record of it is what is broken — which is what keeps the caller
// from concluding that nothing happened.
func (c *OrderClient) AddIsolatedMargin(ctx context.Context, instrument ids.VenueInstrument,
	assets InstrumentAssets, amount fixedpoint.Money, urgency ratelimit.Priority) error {

	op := string(c.codec.Venue()) + ".AddIsolatedMargin"

	codec, ok := c.codec.(MarginWriteCodec)
	if !ok {
		// A codec that does not implement the surface. ErrMarginActCodecAbsent's
		// case one surface over, and reported as its own sentinel rather than as
		// a routing gap: the fix is a codec, not a transport.
		return apperror.Precondition(op, "codec", ErrMarginWriteCodecAbsent)
	}
	if err := assets.Validate(); err != nil {
		return apperror.Precondition(op, "assets", err)
	}
	if err := urgency.Validate(); err != nil {
		return apperror.Precondition(op, "urgency", err)
	}
	if urgency == ratelimit.PriorityRoutine {
		return apperror.Precondition(op, "urgency",
			fmt.Errorf("margin top-up on %s: %w", instrument.Symbol, ErrMarginWriteUrgencyRoutine))
	}
	if err := checkMarginWriteAmount(assets, amount); err != nil {
		return apperror.Precondition(op, "amount", fmt.Errorf("margin top-up on %s: %w", instrument.Symbol, err))
	}

	client, err := c.route(op, OrderRef{Instrument: instrument, ClientOrderID: marginWriteRef}, urgency)
	if err != nil {
		return err
	}

	call, err := codec.EncodeAddIsolatedMargin(instrument, assets, amount)
	if err != nil {
		return apperror.Precondition(op, "encode", err)
	}
	call.Priority = urgency

	resp, err := client.Do(ctx, call)
	if err != nil {
		// Already classified: SignedClient.Do decides the effect, and §5.3
		// forbids re-deriving it from the cause. The caller's rule for each
		// effect is oms/application's, because what an effect means for a margin
		// op is not what it means for an order.
		return err
	}

	if err := codec.DecodeAddIsolatedMargin(instrument, assets, amount, resp); err != nil {
		// The venue accepted the call and we cannot read what it said about it.
		// EffectApplied, which for this surface means "the margin may have moved
		// and the evidence path must decide" — never "nothing happened".
		return transport.ClassifyAckDecodeFailure(call.Op, err)
	}
	return nil
}

// checkMarginWriteAmount is the rule for the one money figure this system
// ORIGINATES rather than reads.
//
// Both halves fail permissively if absorbed. A non-positive amount is not a
// smaller top-up: Binance's `type` and Bybit's sign are how the direction is
// stated, so a zero is a call that changes nothing while consuming an op's
// evidence window, and a negative one is a REDUCTION rendered as an add — the
// opposite act, on a position whose liq distance provoked it. And an amount typed
// in the base asset is the one substitution available at every call site: it
// validates, it formats, and it sends the number of BITCOIN a caller meant as
// dollars.
func checkMarginWriteAmount(assets InstrumentAssets, amount fixedpoint.Money) error {
	if err := amount.Validate(); err != nil {
		return err
	}
	sign, err := amount.Sign()
	if err != nil {
		return err
	}
	if sign <= 0 {
		return fmt.Errorf("amount is %s: %w", amount, ErrMarginWriteAmountNotPositive)
	}
	if amount.Asset() != assets.Quote {
		return fmt.Errorf("amount is in %s, declared quote is %q: %w",
			amount.Asset(), assets.Quote, ErrMarginAssetsDisagree)
	}
	return nil
}
