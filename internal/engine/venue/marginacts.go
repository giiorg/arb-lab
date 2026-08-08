package venue

import (
	"context"
	"fmt"
	"time"

	"github.com/giiorg/arb-lab/internal/engine/venue/transport"
	"github.com/giiorg/arb-lab/internal/platform/apperror"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// This file is the venue side of core §7.1's SECOND margin-op evidence source:
// the venue's own record of margin-change ACTS (task #214). Binance publishes
// one at `GET /fapi/v1/positionMargin/history`; Bybit publishes none this system
// may rely on.
//
// # Absence is a DECLARED capability, never an error a caller reinterprets
//
// The obvious shape is to let Bybit's encoder refuse and have the caller read
// that refusal as "no history here". It is the wrong shape for a reason the
// consumer makes sharp: on Binance "the act history returned no rows" is
// EVIDENCE — it is what turns a moved posted margin into an Ambiguous verdict —
// while "this venue keeps no act history" is the absence of evidence, and the
// two produce opposite verdicts on identical arithmetic. A rule that derives the
// second from an error derives it from every error, so one timeout would
// manufacture a venue capability fact.
//
// So PublishesMarginActs is its own method, answered from the venue's
// documentation rather than from a call, and the client never issues a request
// for a venue that answers false. An error stays an error.
//
// # Bybit's false is a RULING, and Profiles §4 is where it was taken
//
// Verbatim: `GET /v5/account/transaction-log` records carry a unique `id`, but
// the ~100-value `type` enum "contains no value for a manual position-margin
// add/reduce", and whether a top-up emits a record at all is "undocumented; MUST
// NOT be relied on". Building a matcher over it would be building over a fact the
// venue does not state, and the failure would be silent in the dangerous
// direction: a top-up that emitted no record would read as an act that did not
// happen.
//
// # The charged weight is UNVERIFIED, and overcharging it is nearly free here
//
// Binance's documentation site answers a fetcher with an AWS WAF challenge
// (Profiles §9's ⚠ block; the same wall #180 is open against), and the
// spec-generated connector that supplies this endpoint's response shape verbatim
// carries no weights at all. So no figure for this path is verified, and rather
// than transcribe a plausible one this charges `binanceMarginActsWeight` — the
// largest weight this repo already charges on any `fapi` read.
//
// That is the standing rule (charge the maximum, because undercounting the IP
// budget is the failure that ends in the §7.6 ban) applied where it costs almost
// nothing: this call is issued once per IN-FLIGHT MARGIN OP AT BOOT, and §7.1
// permits one op in flight per position. A steady-state deployment issues zero
// of them. Overcharging a call that is normally never made spends no budget,
// which is what makes the conservative direction the cheap one here rather than
// a trade.

// binanceMarginActsWeight is the charge for one act-history read — see the file
// comment. Not a verified figure; deliberately the repo's largest `fapi` weight.
const binanceMarginActsWeight = 20

// binanceMarginActsPageLimit is the row cap requested per read.
//
// Binance's parameter defaults to 500 (Profiles §4) and this asks for that
// maximum deliberately: the window is a propagation bound wide — seconds — so a
// page that filled would mean hundreds of margin adjustments on one symbol in
// that time, which is not a page to turn but a state no verdict should be
// reached in. DecodeMarginActs refuses a full page for exactly that reason.
const binanceMarginActsPageLimit = 500

// MarginActCause is why the venue says a margin change happened — Binance's
// `deltaType`.
//
// Two members, and the second is a catch-all rather than an enumeration: only
// one value of this field is verified (Profiles §4), so guessing the rest would
// put an unverified fact in the position that decides whether an act is
// attributed to this system.
type MarginActCause string

const (
	// MarginActCauseUnset is the zero value and names nothing.
	MarginActCauseUnset MarginActCause = ""

	// MarginActCauseUserAdjust is a manual position-margin adjustment — the act
	// this system performs. Binance `deltaType: "USER_ADJUST"`.
	MarginActCauseUserAdjust MarginActCause = "user_adjust"

	// MarginActCauseOther is every other value, recognised or not.
	//
	// It is REPORTED rather than dropped, and the consumer resolves a row
	// carrying it as Ambiguous. A decoder that silently discarded unrecognised
	// causes would hide a venue act behind a verdict computed from the weaker
	// source — and this system already knows margin can move for a second reason,
	// because §4.1's posture check reads `isAutoAddMargin` per configured symbol
	// precisely because the venue can add margin on its own.
	MarginActCauseOther MarginActCause = "other"
)

// binanceUserAdjustDeltaType is the one `deltaType` value this repo holds
// verbatim: the spec-generated connector's own mocked response for this endpoint
// (Profiles §4). Every other value decodes to MarginActCauseOther.
const binanceUserAdjustDeltaType = "USER_ADJUST"

// MarginAct is one margin-change row, normalized.
//
// These rows carry NO unique id on either venue (Profiles §4), which is why
// MarginOp exists at all and why every field here is part of a match rather than
// a key.
type MarginAct struct {
	// Symbol is the market the venue reported the change on. Carried so the
	// caller can check the query's scope took effect rather than trust it.
	Symbol string

	// Amount is the magnitude of the change, non-negative. Binance publishes an
	// unsigned `amount` and carries direction in `type`.
	Amount fixedpoint.Money

	// Adds is the direction: true for a top-up (`type` 1), false for a reduction
	// (`type` 2).
	Adds bool

	// At is the venue's timestamp on the row, in venue time.
	At time.Time

	// Cause is the venue's stated reason.
	Cause MarginActCause
}

// Validate refuses a row a match could be run against wrongly.
func (a MarginAct) Validate() error {
	if a.Symbol == "" {
		return ErrMarginActUnreadable
	}
	if err := a.Amount.Validate(); err != nil {
		return fmt.Errorf("margin act on %s amount: %w", a.Symbol, err)
	}
	sign, err := a.Amount.Sign()
	if err != nil {
		return fmt.Errorf("margin act on %s amount: %w", a.Symbol, err)
	}
	if sign <= 0 {
		return fmt.Errorf("margin act on %s amount is %s: %w", a.Symbol, a.Amount, ErrMarginActUnreadable)
	}
	if a.At.IsZero() {
		return fmt.Errorf("margin act on %s has no timestamp: %w", a.Symbol, ErrMarginActUnreadable)
	}
	switch a.Cause {
	case MarginActCauseUserAdjust, MarginActCauseOther:
		return nil
	case MarginActCauseUnset:
		return fmt.Errorf("margin act on %s has no cause: %w", a.Symbol, ErrMarginActUnreadable)
	}
	return fmt.Errorf("margin act on %s cause %q: %w", a.Symbol, string(a.Cause), ErrMarginActUnreadable)
}

// MarginActWindow is the venue-time interval an act-history read covers,
// half-open: [From, To).
//
// A struct rather than two `time.Time` parameters, on the signature rule a
// swapped pair earned elsewhere in this repo: two adjacent same-typed arguments
// are exchangeable at every call site, and an inverted window here returns no
// rows and no error — which the consumer reads as "the venue recorded nothing",
// the evidence that turns a correct top-up into an operator incident.
type MarginActWindow struct {
	From time.Time
	To   time.Time
}

// Validate refuses a window that could not contain an act.
func (w MarginActWindow) Validate() error {
	if w.From.IsZero() || w.To.IsZero() {
		return fmt.Errorf("margin act window %v..%v: %w", w.From, w.To, ErrMarginActWindowInvalid)
	}
	if !w.From.Before(w.To) {
		// Empty and inverted are refused together. Both fetch nothing, and
		// "nothing" is a load-bearing answer on this surface rather than a
		// harmless empty result.
		return fmt.Errorf("margin act window %v..%v: %w", w.From, w.To, ErrMarginActWindowInvalid)
	}
	return nil
}

// MarginActCodec is one venue's translation of the margin-act history surface.
//
// It is NOT embedded into OrderCodec, and that is the difference from
// MarginStatusCodec one surface over. A venue whose margin status cannot be read
// is a venue §7.1's trigger cannot run on, so embedding models a combination that
// must not exist. A venue that records no margin acts is merely one whose ops
// resolve from a single source — an ordinary, supported deployment.
type MarginActCodec interface {
	// PublishesMarginActs reports whether this venue has a margin-act history
	// this system may rely on. See the file comment: a false is a documented
	// venue fact, not a missing implementation, and the client answers it without
	// issuing a call.
	PublishesMarginActs() bool

	// EncodeMarginActs renders the windowed read.
	//
	// adds selects the venue's direction filter, because Binance's `type` is a
	// request parameter rather than only a response field — narrowing at the
	// venue costs nothing and the consumer re-applies the same rule to every row
	// it is given.
	EncodeMarginActs(instrument ids.VenueInstrument, w MarginActWindow, adds bool) (transport.Call, error)

	// DecodeMarginActs reads the venue's answer.
	//
	// An implementation MUST return every row the venue reported without
	// filtering on amount or time. The match — §7.1's "symbol + type + amount +
	// time-window" — belongs to the consumer, and the duplicate-row case the
	// section calls Ambiguous is precisely one where two rows must both survive
	// to be counted.
	DecodeMarginActs(instrument ids.VenueInstrument, assets InstrumentAssets, resp transport.Response) ([]MarginAct, error)
}

// marginActsReadRef is the placeholder client order id this surface's routing
// check carries — marginStatusReadRef's argument, unchanged.
const marginActsReadRef ids.ClientOrderID = "margin-acts-read"

// MarginActs reads the venue's margin-change history for one instrument.
//
// The bool reports whether this venue publishes such a history at all. It is not
// an error and it is not an empty slice: an empty slice from a venue that DOES
// publish one means the venue was asked and recorded nothing, which is evidence
// §7.1 reasons from, while a false here means nobody can be asked.
//
// # The urgency is the caller's, and today's caller is a boot
//
// MarginStatus's argument. This runs inside §7.5's reconcile boot, resolving ops
// left in flight by a crash, so it is not on the incident path the way a margin
// read is — but it is on the path that decides whether trading may resume, so a
// caller is free to raise it.
func (c *OrderClient) MarginActs(ctx context.Context, instrument ids.VenueInstrument, assets InstrumentAssets,
	w MarginActWindow, adds bool, urgency ratelimit.Priority) ([]MarginAct, bool, error) {

	op := string(c.codec.Venue()) + ".MarginActs"

	codec, ok := c.codec.(MarginActCodec)
	if !ok {
		// A codec that does not implement the surface at all. Distinct from one
		// that implements it and declares no history: this is a wiring defect,
		// and reporting it as "this venue publishes none" would silently downgrade
		// Binance's two-source contract to one source.
		return nil, false, apperror.Precondition(op, "codec", ErrMarginActCodecAbsent)
	}
	if !codec.PublishesMarginActs() {
		return nil, false, nil
	}
	if err := assets.Validate(); err != nil {
		return nil, false, apperror.Precondition(op, "assets", err)
	}
	if err := w.Validate(); err != nil {
		return nil, false, apperror.Precondition(op, "window", err)
	}

	client, err := c.route(op, OrderRef{Instrument: instrument, ClientOrderID: marginActsReadRef}, urgency)
	if err != nil {
		return nil, false, err
	}

	call, err := codec.EncodeMarginActs(instrument, w, adds)
	if err != nil {
		return nil, false, apperror.Precondition(op, "encode", err)
	}
	call.Priority = urgency

	resp, err := client.Do(ctx, call)
	if err != nil {
		// Unwrapped, and the rule bites harder here than on any other read: the
		// transport classified the effect, and an unanswered act history must not
		// become an empty one. An empty history is the observation that turns a
		// moved posted margin into an Ambiguous verdict, so manufacturing it out
		// of a timeout would invent evidence.
		return nil, false, err
	}

	acts, err := codec.DecodeMarginActs(instrument, assets, resp)
	if err != nil {
		return nil, false, apperror.Precondition(op, "decode", err)
	}
	for i, act := range acts {
		if err := act.Validate(); err != nil {
			return nil, false, apperror.Precondition(op, "decode", fmt.Errorf("row %d: %w", i, err))
		}
		if act.Symbol != instrument.Symbol {
			// The query was scoped to a symbol, so a row for another one means the
			// parameter did not take effect — and the rows would then be another
			// market's acts matched against this op's amount.
			return nil, false, apperror.Precondition(op, "decode",
				fmt.Errorf("row %d is %s, asked about %s: %w", i, act.Symbol, instrument.Symbol, ErrMarginActUnreadable))
		}
	}
	return acts, true, nil
}
