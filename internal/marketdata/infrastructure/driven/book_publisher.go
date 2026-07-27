package driven

import (
	"fmt"
	"github.com/giiorg/arb-lab/internal/marketdata/application/ports"

	"github.com/giiorg/arb-lab/internal/engine/venue"
	"github.com/giiorg/arb-lab/internal/platform/events"
)

// bookPrio is the priority every reconstructed-book envelope is published at.
//
// # The same constant as funding, and the reasoning is NOT "they are equally
// # important"
//
// §3.4 is explicit that they are not — a book gates execution and "funding
// metadata tolerates longer" — so the tempting move is to publish books more
// urgently. Two things make that the wrong lever.
//
// §3.5's constancy requirement (see fundingPrio, landed as a spec edit in
// v0.9.8) is per-source and would still be satisfied by a constant that differs
// between streams, so it does not settle this on its own. What settles it is
// that the market-data lane does not read Prio at all: it is a per-SourceID
// mailbox set, and ordering WITHIN a mailbox is arrival order. So a book
// published at a lower number would not overtake anything — it would be a
// difference that changes no behaviour, sitting where a reader would take it as
// a promise the lane makes. §3.4's class distinction is real and it is expressed
// where it has teeth: two budgets, two mailboxes, two silence timers.
//
// Not the zero value, for events.Prio's stated reason: PrioSafety is 0 so an
// unclassified envelope is treated as urgent, and taking it here by default
// would put market data ahead of the safety kernel because nobody chose.
const bookPrio = events.PrioNormal

// BookLanePublisher is ports.BookPublisher over §3.4's market-data lane.
//
// # A second type rather than a second method, and Go decides that, not taste
//
// ConfigBudgets satisfies both narrow budget ports from one struct, and the
// natural symmetry would be one publisher satisfying both. It cannot:
// ports.FundingPublisher and ports.BookPublisher both declare
// Publish, differing only in parameter, and one type cannot have both. Naming it
// away — PublishBook — would stop it satisfying the port.
//
// The split it forces is honest anyway. What the two share is a lane pointer and
// nothing else: the source constructor differs, the payload differs, and the
// domain check at the end differs. What must NOT drift between them is the
// envelope-assembly rule — which sequence is inherited and which is left
// unstamped — and that is stated once, in LanePublisher's doc, and referred to
// here rather than restated.
// §3.6 rule 2's compile-time assertion; see LanePublisher's for what it buys.
//
// The two publishers exist because Go forbids one type from having two Publish
// methods, and this assertion is what keeps that split from silently becoming a
// drift: whichever of the two is edited, the one that stopped satisfying its
// port names itself at build time.
var _ ports.BookPublisher = (*BookLanePublisher)(nil)

type BookLanePublisher struct {
	lane *events.MarketDataLane
}

// NewBookLanePublisher binds the adapter to a lane, refusing a nil one.
//
// It shares ErrLaneNil with NewLanePublisher, which is the opposite call from
// the driving package's three new sentinels and is right for the opposite
// reason: there the message named a wire that differed per path, and here both
// constructors name the same wire — the same lane, from the same Deps field. A
// second sentinel would be two names for one missing pointer.
func NewBookLanePublisher(lane *events.MarketDataLane) (*BookLanePublisher, error) {
	if lane == nil {
		return nil, ErrLaneNil
	}
	return &BookLanePublisher{lane: lane}, nil
}

// Publish puts a reconstructed book on its instrument's book mailbox.
//
// # Two checks, two owners, in the order that keeps both reachable
//
// LanePublisher.Publish states the pattern and the argument for each half; what
// follows is only what differs on this path.
//
// The probe forces BOTH sequence numbers so Validate's unstamped checks do not
// fire, and it is discarded — the published envelope keeps IngressSeq zero,
// because a mailbox holds pre-dispatch envelopes, and keeps the AcceptOrdinal it
// inherited. Forcing the ordinal on the probe matters more here than it does on
// the funding path: a view from the book silence timer legitimately carries a
// zero, and §3.4 gives books sub-second budgets precisely because they gate
// execution — so probing with the real value would refuse exactly the envelope
// that reports a dead book feed, which is the one this lane most needs.
//
// The domain check is view.Book.Validate rather than an observation's, and it is
// the one that cannot be skipped: it owns the invariants no envelope can see —
// both sides ordered, uncrossed, one asset pair, and a non-zero VenueSeq. The
// last of those is the reason a book that is merely WELL-FORMED is not enough
// (domain.ErrBookSeqUnset): such a book validates as a lane payload and then
// never advances.
func (p *BookLanePublisher) Publish(view ports.BookView) error {
	w := view.Book.Watermark

	// venue.BookSource rather than a literal, for FundingSource's structural
	// reason rather than a safety one: one place in the codebase writes down
	// which stream carries books. It also differs from the funding constructor in
	// a way worth not re-deciding at a call site — StreamBook is available to
	// SPOT as well as perp, since §4.2 prices the spot leg from its book, so this
	// is the source constructor that must not inherit the perp-only refusal.
	src, err := venue.BookSource(view.Book.Instrument)
	if err != nil {
		return fmt.Errorf("marketdata book publish: %w", err)
	}

	env := events.Envelope{
		// Inherited, not minted. Zero is legitimate and means the view came from
		// a silence timer — from no input at all — which is why it is copied
		// unconditionally rather than guarded.
		AcceptOrdinal: uint64(view.Trigger),

		Source:     src,
		SourceTime: w.SourceTime,
		RecvTime:   w.RecvTime,
		VenueSeq:   w.VenueSeq,
		Prio:       bookPrio,
		Payload:    view,
	}

	probe := env
	probe.AcceptOrdinal, probe.IngressSeq = 1, 1
	if err := probe.Validate(); err != nil {
		return fmt.Errorf("marketdata book publish: %w", err)
	}
	if err := view.Book.Validate(); err != nil {
		return fmt.Errorf("marketdata book publish %s: %w", view.Book.Instrument, err)
	}

	return p.lane.Publish(env)
}
