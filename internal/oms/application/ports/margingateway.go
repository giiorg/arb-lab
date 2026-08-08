package ports

import (
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// MarginGateway is §4's `MarginGateway` narrowed to the one method core §7.1's
// ladder calls: posting additional isolated margin against a perpetual position
// (task #215).
//
// # One method of four, on OrderPlacer's rule and with a sharper reason
//
// §4 declares `AddIsolatedMargin`, `ReduceIsolatedMargin`, `TransferCollateral`
// and `ResolveMarginOp`, and contexts are told to consume "segregated capability
// interfaces". Two of the three absent methods have no consumer at all — §7.1's
// ladder tops up on the Rebalance band and de-risks by delevering, never by
// pulling margin back out, and the staged float belongs to §7.2's unbuilt
// `treasury`. The third is not a gateway method in this system AT ALL, and that
// is the interesting one: `ResolveMarginOp` is `MarginOpResolver`, an application
// use case reading two venue ports, because the resolution is evidence
// reconciliation over a journaled before-state rather than a call to a venue
// (record §81). A port method for it would put §7.1's evidence rule behind an
// adapter, where the rule that decides `Ambiguous` would be venue code.
//
// # It ADDS, and the direction is not a parameter
//
// domain.MarginOp carries a signed delta because the venues' own APIs are signed.
// This interface does not, and the narrowing is where the two facts are allowed
// to differ: a record must be able to describe a reduction (a venue's act history
// reports them, and a resolver must match against them), while the DISPATCH path
// may only perform the direction that has a caller. A signed parameter here would
// declare an act nothing in this system performs and nothing tests — the
// enum-member-with-no-producer shape, arriving as a sign convention.
//
// # It returns no acknowledgement, and that is the venue fact this whole slice
// # is built around
//
// §4's declared signature is `(Ack, error)`. Neither venue supplies an operation
// identity — Bybit answers with a position snapshot, Binance's own history rows
// carry no id (Profiles §4) — so an `Ack` here could only ever be the empty
// value, and a caller reading a field off it would be reading a fact no venue
// states. What replaces it is the journaled before-state: `domain.MarginOp`
// exists precisely because this call cannot answer "which operation was that".
// §4 now declares the same signature for the same reason, and states it for
// `ReduceIsolatedMargin` too — same endpoint, same absent identity — while
// `TransferCollateral` keeps its `Ack` because transfers are the one op with a
// real one (core v0.9.65, task #215, record §84).
//
// A nil error therefore carries the whole acknowledgement: the venue said it
// performed the act, on the market and in the direction and (where the venue
// states it) the amount that was asked for. The adapter checks all of that; see
// engine/venue.MarginWriteCodec for which half each venue lets it check.
type MarginGateway interface {
	// AddIsolatedMargin posts amount of additional isolated margin against the
	// position on instrument, EXACTLY ONCE.
	//
	// §4: state-changing and NON-idempotent, and §7.1 adds the clause that makes
	// it different from every other write here — "blind delta retries are
	// prohibited". An implementation MUST NOT retry internally, for
	// OrderPlacer.Place's reason and one further: an order retried twice can be
	// resolved by asking the venue about its ClientOrderID, and a top-up sent
	// twice cannot be distinguished from one sent once, by anything, ever
	// (§7.1's duplicate same-amount case is exactly this shape arriving from
	// outside).
	//
	// amount is strictly positive and in the instrument's margin asset — the
	// quote, on the linear perpetuals this system trades. An implementation
	// refuses anything else rather than clamping: a zero consumes an evidence
	// window without changing anything, and a negative is the opposite act.
	//
	// # The error MUST carry an apperror.Effect, and the caller's reading of it
	// # is NOT the order path's
	//
	// The adapter is the only layer that knows whether the request left the
	// socket (§5.3), so the effect is recorded there and read here. What differs
	// from `Place` is what each effect licenses: an order at EffectNotApplied
	// goes to `SubmissionUnknown` because two absence terminals must be told
	// apart by evidence class, while a margin op has ONE absence answer, so the
	// same effect concludes `NotApplied` outright. See
	// application.classifyMarginCall, which is where the mapping lives and why.
	//
	// An unclassified error is safe rather than sharp: apperror.EffectOf answers
	// EffectUnknown for anything it does not recognise, and an unknown-effect
	// margin op is left IN FLIGHT for §7.1's evidence path to resolve — the
	// conservative direction, since the dangerous one is concluding that a
	// top-up applied when it did not.
	AddIsolatedMargin(instrument ids.VenueInstrument, amount fixedpoint.Money) error
}
