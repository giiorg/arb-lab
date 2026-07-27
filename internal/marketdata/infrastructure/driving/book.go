package driving

import (
	"fmt"

	"github.com/giiorg/arb-lab/internal/engine/venue"
	"github.com/giiorg/arb-lab/internal/marketdata/application"
	"github.com/giiorg/arb-lab/internal/marketdata/domain"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/events"
)

// BookDispatch adapts §3.5's dispatch surface to marketdata's book
// reconstruction.
//
// Not safe for concurrent use, inherited rather than chosen — FundingDispatch's
// comment states the argument and it does not change here: §3.4 makes the
// marketdata reducer single-writer, and BookReconstruct holds unsynchronized
// per-instrument state for exactly that reason.
type BookDispatch struct {
	reconstruct *application.BookReconstruct
}

// NewBookDispatch binds the adapter to a use case, refusing a nil one.
func NewBookDispatch(r *application.BookReconstruct) (*BookDispatch, error) {
	if r == nil {
		return nil, ErrReconstructNil
	}
	return &BookDispatch{reconstruct: r}, nil
}

// Deliver translates one dispatched book envelope and runs the reconstruction.
//
// # ONE entry point for both payloads, and that is the load-bearing decision
//
// The obvious alternative is two methods — DeliverSnapshot and DeliverDelta —
// and it looks tidier, because the two do call different use-case methods. It is
// wrong at this seam, and the reason is §3.5's: a snapshot and a delta for one
// instrument arrive on ONE SourceID (venue.BookSource makes no distinction), so
// they share one mailbox, and VenueSeq is authoritative WITHIN a SourceID.
//
// Two entry points would put the choice of which to call on the dispatcher,
// which holds only an envelope and would have to type-assert the payload to
// decide — the same assertion, moved somewhere with no obligation to keep the
// order. And the order is the whole of reconstruction: a snapshot applied after
// the delta that followed it rewinds the book to the fetch position, which the
// next delta then gaps against. Keeping the branch here means the dispatcher
// drains the mailbox in sequence and the routing cannot disagree with it.
//
// # Value assertions only, and not both spellings
//
// FundingDispatch's rule: accepting a pointer as well would give one payload two
// spellings, one of which shares memory with its producer, on a lane whose
// contract is that a published envelope is not mutated afterwards.
func (d *BookDispatch) Deliver(env events.Envelope) (application.BookView, error) {
	if err := env.Validate(); err != nil {
		return application.BookView{}, fmt.Errorf("marketdata book deliver: %w", err)
	}
	// Against the stream, not the lane. The lane check would pass: funding and
	// mark ride the market-data lane too, and a funding payload read as a book
	// would land on the payload assertion below rather than here — with an error
	// naming the type instead of the misrouting that caused it.
	if env.Source.Stream != events.StreamBook {
		return application.BookView{}, fmt.Errorf(
			"marketdata book deliver: stream %s: %w", env.Source.Stream, ErrNotBookStream)
	}

	// The ordinal cannot be zero — Validate above refuses an unstamped envelope —
	// which is what keeps application.ErrNoTrigger a claim about hand-built calls
	// rather than about this path. Converted rather than passed, because
	// application declares its own AcceptOrdinal and does not take the
	// transport's types.
	trigger := application.AcceptOrdinal(env.AcceptOrdinal)
	wm := domain.Watermark{
		VenueSeq:   env.VenueSeq,
		SourceTime: env.SourceTime,
		RecvTime:   env.RecvTime,
	}

	switch p := env.Payload.(type) {
	case venue.BookSnapshot:
		// Judged as a venue message before it is translated, so a malformed level
		// names the venue seam rather than this context's reconstruction of it.
		// The redundancy with the domain check inside OnSnapshot is the one
		// FundingDispatch records: what it buys is which boundary an operator is
		// sent to, and a field this translation transposes still reaches the
		// domain check, which is the only one positioned to catch it.
		if err := p.Validate(); err != nil {
			return application.BookView{}, fmt.Errorf("marketdata book deliver: %w", err)
		}
		return d.reconstruct.OnSnapshot(domain.BookSnapshot{
			// From the SOURCE, never the payload: it is the mailbox key, and a
			// payload that named one could contradict the mailbox it arrived on.
			Instrument: env.Source.Instrument,
			Bids:       toDomainLevels(p.Bids),
			Asks:       toDomainLevels(p.Asks),
			Watermark:  wm,
		}, trigger)

	case venue.BookDelta:
		if err := p.Validate(); err != nil {
			return application.BookView{}, fmt.Errorf("marketdata book deliver: %w", err)
		}
		changes, err := toDomainChanges(p.Changes)
		if err != nil {
			return application.BookView{}, fmt.Errorf("marketdata book deliver: %w", err)
		}
		return d.reconstruct.OnDelta(domain.BookDelta{
			Instrument: env.Source.Instrument,
			Changes:    changes,
			FirstSeq:   p.FirstSeq,
			// The span's far end comes from the envelope and its near end from
			// the payload — venue.BookDelta records why, and this is the line
			// that assembles them. A FirstSeq past this value is the one
			// disagreement the split leaves possible, and domain's Validate
			// names it ErrDeltaSeqInverted, which is what it is.
			LastSeq:   env.VenueSeq,
			Watermark: wm,
		}, trigger)
	}

	return application.BookView{}, fmt.Errorf(
		"marketdata book deliver: payload %T: %w", env.Payload, ErrBookPayloadType)
}

// Timer routes a fired book silence timer to the use case.
//
// It does not filter, for FundingDispatch.Timer's stated reason: the membership
// test already exists once, inside the use case's map of armed timers, and a
// second one written from the ID's prefix would be a copy of a naming rule that
// lives in application/. A foreign ID comes back as application.ErrUnknownTimer.
//
// The two Timer methods are DISTINCT and must stay so. Book and funding arm
// separate ID namespaces (application.bookSilenceTimerID says why), so a
// dispatcher routing a book timer here and a funding timer to the other one is
// the arrangement both namespaces exist to serve; routing either to both would
// make every fired timer produce one real answer and one ErrUnknownTimer.
func (d *BookDispatch) Timer(id clock.TimerID) (application.BookView, error) {
	return d.reconstruct.OnSilence(id)
}

// toDomainLevels converts one side's levels, and does nothing else.
//
// # It cannot fail, and it used to — the difference was measured
//
// This validated each level as it copied it, which read as defence in depth and
// was a third copy of one check: venue.BookSnapshot.Validate had already walked
// every level, and domain.BookSnapshot.Validate walks them again. The cost was
// not the cycles. Bypassing the venue-seam check entirely left every test
// passing — tools/mutate.sh reported SURVIVED — because this function still
// produced the venue sentinel, so nothing could tell the two apart and the
// boundary the check exists to name was untested while looking covered.
//
// A helper called "to domain levels" that also refuses them is doing two jobs,
// and the refusal belongs to the one that owns the rule. Nothing is lost from
// the message either: venue.BookSnapshot.Validate already reports the side and
// the index ("ask level 2"), which is why that error never needed rebuilding
// here.
//
// The result is a fresh allocation, which the type change forces and which is
// also what is wanted: the payload's backing array belongs to an envelope on a
// lane, and a domain book aliasing it would be a published snapshot sharing
// memory with a producer.
func toDomainLevels(levels []venue.BookLevel) []domain.BookLevel {
	if len(levels) == 0 {
		// nil rather than an empty slice, so an empty side is spelled one way.
		// Two spellings of "no levels" is two things for a consumer to test.
		return nil
	}
	out := make([]domain.BookLevel, len(levels))
	for i, l := range levels {
		out[i] = domain.BookLevel{Price: l.Price, Quantity: l.Quantity}
	}
	return out
}

// toDomainChanges converts a delta's instructions.
func toDomainChanges(changes []venue.LevelChange) ([]domain.LevelChange, error) {
	if len(changes) == 0 {
		// An empty delta is a heartbeat and is legal — venue.BookDelta says so.
		return nil, nil
	}
	out := make([]domain.LevelChange, len(changes))
	for i, c := range changes {
		side, err := toDomainSide(c.Side)
		if err != nil {
			return nil, fmt.Errorf("change %d: %w", i, err)
		}
		out[i] = domain.LevelChange{Side: side, Price: c.Price, Quantity: c.Quantity}
	}
	return out, nil
}

// toDomainSide translates the venue seam's side into the domain's.
//
// # This function is why the two BookSide types are worth having
//
// It is a switch over a module-declared enum-like, so §3.6 rule 10 requires it
// to name every member — which makes a side added to venue.BookSide and not to
// domain.BookSide a BUILD failure, here, at the one place the two vocabularies
// meet. A shared type would make the same change a silent pass-through of a
// value the domain has no case for, and the symptom would be a book missing
// every level on the new side rather than a compile error.
//
// The two happen to use identical string values today, so `domain.BookSide(s)`
// would work and is exactly the shortcut that discards the property above.
func toDomainSide(s venue.BookSide) (domain.BookSide, error) {
	switch s {
	case venue.SideBid:
		return domain.SideBid, nil
	case venue.SideAsk:
		return domain.SideAsk, nil
	}
	// Unreachable through Deliver, which validates the payload — and every
	// LevelChange with it — first. Returned rather than assumed, because the
	// alternative default is a side, and defaulting a side to bid writes an ask
	// into the wrong half of the book, where it crosses the spread and reads as
	// an arbitrage.
	if err := s.Validate(); err != nil {
		return "", fmt.Errorf("book change side: %w", err)
	}
	return "", fmt.Errorf("book change side %q: %w", string(s), venue.ErrSideUnknown)
}
