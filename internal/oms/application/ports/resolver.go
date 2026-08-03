package ports

import (
	"fmt"
	"time"

	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// OrderResolver is §4's OrderGateway narrowed to the two READ methods §5.3's
// recovery may use, and it is declared here with its consumer for the reason
// OrderPlacer was: a port with no use case is a shape frozen against nothing.
//
// # It is a separate port from OrderPlacer, and the separation is the safety
// # property
//
// §5.3: an order at DispatchAuthorized or beyond "is resolved ONLY by
// OrderByClientID/Fills — never resubmitted, and 'not sent' is never inferred".
// The resolution use case therefore holds this port and not OrderPlacer, so
// "never resubmit" stops being a rule someone must remember and becomes a method
// the resolver does not have. A single wide OrderGateway port would have left
// the resubmission one line away from the code whose whole subject is not
// writing it.
type OrderResolver interface {
	// Resolve reads one order's state at the venue, across every surface that
	// venue offers.
	//
	// The multi-surface walk is the ADAPTER's, because which surfaces exist and
	// which is consulted next are venue facts — §5.3 makes Bybit's normative
	// ("resolution MUST fall back to GET /v5/order/history by orderLinkId …
	// never infer 'not submitted' from realtime alone"). What comes back names
	// the surface that ANSWERED, which for an absence is the last one tried.
	//
	// An ABSENCE is a value on the returned VenueOrder, never an error. An error
	// means the query was not answered — rate-limited, timed out, refused — and
	// §5.3 forbids the two being told apart by inspection at the call site:
	// "'not submitted' is never inferred from a transient not-found", and "if the
	// resolving query is unavailable/rate-limited, new trading stays disabled".
	Resolve(o domain.Order) (VenueOrder, error)

	// SweepExecutions reads the venue's executions for one instrument across a
	// closed window, and reports how far coverage was actually PROVEN.
	//
	// SM §2 step 3 sweeps "through a venue/server watermark strictly LATER than
	// the order's cancellation-effectiveness / max landing horizon", so the
	// window's end is an input and the watermark reached is an output. They are
	// not the same value: an implementation that runs out of pages, budget or
	// patience returns what it proved, and the caller — which owns the policy —
	// decides that a short sweep proves nothing rather than being told a
	// comfortable lie.
	//
	// Paging is the adapter's and MUST be bounded. engine/venue deliberately
	// refuses to loop inside one call ("a client that looped internally would
	// spend an unbounded number of §7.6 acquisitions"), so the loop exists here;
	// an unbounded one would spend the rate-limit reserve that §5.3 requires be
	// held FOR this recovery.
	SweepExecutions(o domain.Order, since, until time.Time) (ExecutionSweep, error)
}

// VenuePresence is whether the venue had a record of the order at all.
//
// It is a value rather than an error for §5.3's reason, and the two venues make
// the case for it from opposite directions: Binance states "no such order"
// inside an error envelope (code -2013) and Bybit inside a success with an empty
// list. Neither spelling is what the rule needs — the rule needs "the venue
// answered, and its answer is: no such order", which is a fact about the ORDER.
// An error is a fact about the QUERY.
type VenuePresence string

const (
	// VenueHasOrder means the venue returned a record.
	VenueHasOrder VenuePresence = "has_order"

	// VenueHasNoOrder means the venue answered, and answered that it holds no
	// such order on any surface it offers.
	//
	// It is NOT "the order was never placed", and nothing in this package may
	// read it that way. What it licenses is decided by the four gates of §5.3's
	// stable-not-found rule — see the Resolver use case.
	VenueHasNoOrder VenuePresence = "has_no_order"
)

// Validate reports whether p is one of the declared readings.
func (p VenuePresence) Validate() error {
	switch p {
	case VenueHasOrder, VenueHasNoOrder:
		return nil
	}
	return fmt.Errorf("%q: %w", string(p), ErrVenuePresenceInvalid)
}

func (p VenuePresence) String() string { return string(p) }

// SurfaceReach is how far back the surface that answered can see, per the
// venue's own published retention.
//
// # Both durations are carried, and the rule that consumes them is NOT here
//
// The venues publish two horizons — Binance USDⓈ-M drops an unfilled
// CANCELED/EXPIRED order after 3 days and everything after 90 — and the
// question "which one bounds an ABSENCE" has an answer that is a policy rather
// than a venue fact. So the translation carries both and the Resolver decides.
// A port that pre-combined them would put a rule in the ACL, where no test
// about the rule would think to look.
type SurfaceReach struct {
	// Surface names which of the venue's surfaces answered — Bybit resolves an
	// order across two with different retention, so "absent from realtime" and
	// "absent from history" are different claims.
	//
	// It is a plain string and it is REPORTED, never branched on: composing the
	// surfaces is the adapter's job (§5.3 makes Bybit's fallback normative), so
	// by the time a value reaches this package the walk is over. Carried so an
	// operator reading a refusal learns which surface produced the absence.
	Surface string

	// UnfilledRetention is how long after creation the surface still returns an
	// order that reached a terminal state with NO execution. FilledRetention is
	// the same for one that executed at least once.
	//
	// Both are zero when Published is false, and zero is not "forever" — see
	// Validate.
	UnfilledRetention time.Duration
	FilledRetention   time.Duration

	// Published reports whether these durations are venue facts at all.
	//
	// False for Binance SPOT, whose Query-order page states no retention. It is
	// carried rather than defaulted because both defaults are unsafe and the
	// unsafety is not symmetric — see the Resolver's assumed-reach
	// configuration, which is where an operator's assumption is required to
	// stand in for the missing fact.
	Published bool
}

// Validate reports whether the reach is one a rule may reason from.
func (r SurfaceReach) Validate() error {
	if r.Surface == "" {
		return ErrSurfaceUnnamed
	}
	if !r.Published {
		if r.UnfilledRetention != 0 || r.FilledRetention != 0 {
			return fmt.Errorf("%s: %w", r.Surface, ErrSurfaceReachContradictory)
		}
		return nil
	}
	if r.UnfilledRetention <= 0 || r.FilledRetention <= 0 {
		return fmt.Errorf("%s: %w", r.Surface, ErrSurfaceReachUnset)
	}
	return nil
}

// VenueOrder is one venue's whole answer about one order.
type VenueOrder struct {
	// Presence is whether the venue had the order. Read this FIRST — State and
	// Executed are meaningful only when it is VenueHasOrder.
	Presence VenuePresence

	// Reach is how far back the answering surface can see, which is what decides
	// whether a VenueHasNoOrder is worth anything.
	Reach SurfaceReach

	// State is the venue's lifecycle reading, already translated into SM §1's
	// vocabulary by the ACL. It is the domain's type rather than the venue's
	// because §4 assigns that translation to the adapter, and because a use case
	// holding a venue's spelling would have to re-derive the mapping every time
	// it wanted to compare against a stored state.
	State domain.OrderState

	// Executed is SM §2 step 2's "authoritative final cumulative executed
	// quantity".
	Executed fixedpoint.Quantity

	// VenueOrderID is the venue's own identifier. It is the ONLY join key
	// between a swept execution and one of our orders on Binance, whose trade
	// lists return no client order id at all — see ExecutionSweep.
	VenueOrderID string
}

// VenueExecution is one fill as the sweep reports it.
//
// It is not domain.Fill: that type carries an Account, which nothing on this
// path can populate, and admitting a fill is a transaction §5.3 requires to
// include the recovery cursor and the outbox. This type exists to answer one
// question — does the sweep contain an execution belonging to this order — and
// carries what answering it needs.
type VenueExecution struct {
	// ExecutionID is the venue's immutable fill id, unique only WITHIN an
	// instrument.
	ExecutionID ids.ExecutionID

	// ClientOrderID is the order this fill belongs to, WHERE THE VENUE REPORTS
	// IT, and it is empty on Binance — neither /fapi/v1/userTrades nor
	// /api/v3/myTrades returns one. That emptiness is not a decoding gap; it is
	// the venue fact that makes attribution a three-outcome question.
	ClientOrderID ids.ClientOrderID

	// VenueOrderID is the venue's identifier for the order this fill belongs to.
	VenueOrderID string

	// Quantity is the executed size, and At is the venue's server time for the
	// execution.
	Quantity fixedpoint.Quantity
	At       time.Time
}

// ExecutionSweep is what one bounded sweep proved.
type ExecutionSweep struct {
	// Executions are the fills observed in the window, oldest first.
	Executions []VenueExecution

	// ProvenThrough is the instant coverage was actually established to — §6.7
	// step 3's watermark.
	//
	// It is the field the rule reads, and it is deliberately NOT the window's
	// end: a sweep that exhausted its page budget proves coverage only through
	// its last row, and a caller comparing this against the horizon it asked for
	// is the one that cannot mistake a truncated sweep for a complete one. An
	// EMPTY sweep still advances it to the requested end — "nothing happened
	// through T" is a positive answer and the commonest one.
	ProvenThrough time.Time
}
