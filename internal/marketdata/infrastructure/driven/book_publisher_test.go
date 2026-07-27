package driven_test

import (
	"errors"
	"github.com/giiorg/arb-lab/internal/marketdata/application/ports"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/marketdata/domain"
	"github.com/giiorg/arb-lab/internal/marketdata/infrastructure/driven"
	"github.com/giiorg/arb-lab/internal/platform/events"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

func bookPrice(t *testing.T, s string) fixedpoint.Price {
	t.Helper()
	p, err := fixedpoint.ParsePrice("BTC", "USDT", s)
	if err != nil {
		t.Fatalf("ParsePrice(%q): %v", s, err)
	}
	return p
}

func bookQty(t *testing.T, s string) fixedpoint.Quantity {
	t.Helper()
	q, err := fixedpoint.ParseQuantity("BTC", s)
	if err != nil {
		t.Fatalf("ParseQuantity(%q): %v", s, err)
	}
	return q
}

func bookLevel(t *testing.T, p, q string) domain.BookLevel {
	t.Helper()
	return domain.BookLevel{Price: bookPrice(t, p), Quantity: bookQty(t, q)}
}

// bookView is the fixture. Its watermark reuses the same three pairwise-distinct
// provenance values the funding fixture uses, for the same reason recorded
// there: equal timestamps make "copies SourceTime" and "copies RecvTime" one
// assignment, and a VenueSeq of 1 collides with the ordinal a wrong
// implementation would stamp.
func bookView(t *testing.T, inst ids.VenueInstrument, trigger ports.AcceptOrdinal) ports.BookView {
	t.Helper()
	return ports.BookView{
		Book: domain.BookSnapshot{
			Instrument: inst,
			Bids:       []domain.BookLevel{bookLevel(t, "43250.10", "0.5"), bookLevel(t, "43250.00", "1.25")},
			Asks:       []domain.BookLevel{bookLevel(t, "43251.00", "0.75"), bookLevel(t, "43251.50", "1.5")},
			Watermark: domain.Watermark{
				VenueSeq:   venueSeq,
				SourceTime: sourceAt,
				RecvTime:   recvAt,
			},
		},
		Transport: domain.FreshnessFresh,
		Data:      domain.FreshnessStale,
		Applied:   domain.SeqRelationContiguous,
		Trigger:   trigger,
	}
}

func newBookPublisher(t *testing.T, lane *events.MarketDataLane) *driven.BookLanePublisher {
	t.Helper()
	p, err := driven.NewBookLanePublisher(lane)
	if err != nil {
		t.Fatalf("NewBookLanePublisher: %v", err)
	}
	return p
}

func TestNewBookLanePublisherRefusesAMissingLane(t *testing.T) {
	// Shared with NewLanePublisher, deliberately: both name the same wire, and a
	// second sentinel would be two names for one missing pointer.
	if _, err := driven.NewBookLanePublisher(nil); !errors.Is(err, driven.ErrLaneNil) {
		t.Fatalf("NewBookLanePublisher(nil) = %v, want ErrLaneNil", err)
	}
}

// TestBookPublishAddressesTheBookMailbox is the routing claim, and consuming
// from the BOOK source is what makes it one.
//
// A publisher that called venue.FundingSource would build a perfectly valid
// envelope — same instrument, same lane, and funding is a market-data stream
// too — and put it on a mailbox no book consumer reads. The symptom is an
// instrument that never publishes a book, which §3.4's silence timer reports as
// a venue outage. Nothing but the mailbox key separates the two.
func TestBookPublishAddressesTheBookMailbox(t *testing.T) {
	lane := newLane(t)
	p := newBookPublisher(t, lane)

	if err := p.Publish(bookView(t, perp(), acceptOrdinal)); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	// The negative control first: nothing may have landed on the funding mailbox.
	if _, ok := lane.Consume(events.SourceID{Instrument: perp(), Stream: events.StreamFunding}); ok {
		t.Error("a book reached the FUNDING mailbox — the source constructor is the wrong one")
	}

	env, ok := lane.Consume(events.SourceID{Instrument: perp(), Stream: events.StreamBook})
	if !ok {
		t.Fatal("nothing reached the book mailbox")
	}
	if env.Source.Stream != events.StreamBook {
		t.Errorf("Stream = %q, want %q", env.Source.Stream, events.StreamBook)
	}
	if env.VenueSeq != venueSeq {
		t.Errorf("VenueSeq = %d, want %d", env.VenueSeq, venueSeq)
	}
	if env.SourceTime != sourceAt {
		t.Errorf("SourceTime = %v, want %v", env.SourceTime, sourceAt)
	}
	if env.RecvTime != recvAt {
		t.Errorf("RecvTime = %v, want %v", env.RecvTime, recvAt)
	}
	if env.Prio != events.PrioNormal {
		t.Errorf("Prio = %d, want PrioNormal — not the zero value, which is PrioSafety", env.Prio)
	}

	view, ok := env.Payload.(ports.BookView)
	if !ok {
		t.Fatalf("payload is %T, want ports.BookView", env.Payload)
	}
	if view.Book.Watermark.VenueSeq != venueSeq {
		t.Errorf("payload book seq = %d, want %d", view.Book.Watermark.VenueSeq, venueSeq)
	}
}

// TestBookPublishInheritsTheOrdinalAndNotTheReplayKey is §3.5's two-sequence
// rule at this seam: one is inherited, the other is the dispatcher's to mint.
func TestBookPublishInheritsTheOrdinalAndNotTheReplayKey(t *testing.T) {
	lane := newLane(t)
	if err := newBookPublisher(t, lane).Publish(bookView(t, perp(), acceptOrdinal)); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	env, ok := lane.Consume(events.SourceID{Instrument: perp(), Stream: events.StreamBook})
	if !ok {
		t.Fatal("nothing reached the book mailbox")
	}
	if env.AcceptOrdinal != uint64(acceptOrdinal) {
		t.Errorf("AcceptOrdinal = %d, want the view's %d — a published book that cannot be traced to its cause",
			env.AcceptOrdinal, acceptOrdinal)
	}
	if env.IngressSeq != 0 {
		t.Errorf("IngressSeq = %d, want 0 — a mailbox holds pre-dispatch envelopes, and minting a replay key here is a second authority over it",
			env.IngressSeq)
	}
}

// TestBookPublishAcceptsATimerDerivedView is the case the probe exists for, and
// it is the one a naive validate-then-publish would refuse.
//
// A view from the book silence timer legitimately carries a zero Trigger —
// derived from no input at all — while events.Envelope.Validate refuses a zero
// AcceptOrdinal. Probing with the REAL value instead of a forced one would
// therefore reject precisely the envelope that reports a dead book feed, and
// §3.4 gives books sub-second budgets because they gate execution. The feed
// dying is the one case that would go unreported.
func TestBookPublishAcceptsATimerDerivedView(t *testing.T) {
	lane := newLane(t)
	if err := newBookPublisher(t, lane).Publish(bookView(t, perp(), 0)); err != nil {
		t.Fatalf("Publish(trigger 0) = %v, want nil — a silence-timer view has no ordinal", err)
	}

	env, ok := lane.Consume(events.SourceID{Instrument: perp(), Stream: events.StreamBook})
	if !ok {
		t.Fatal("nothing reached the book mailbox")
	}
	if env.AcceptOrdinal != 0 {
		t.Errorf("AcceptOrdinal = %d, want 0 — the probe's forced value must not leak onto the published envelope",
			env.AcceptOrdinal)
	}
}

// TestBookPublishAcceptsASpotBook is the asymmetry between the two source
// constructors, asserted at the publisher.
//
// §3.5 gives StreamBook to both product families and StreamFunding to perps
// alone. A publisher copied from LanePublisher with the source call swapped
// would pass every other test here and refuse every spot book — half of every
// cash-and-carry trade, since §4.2 prices the spot leg from exactly this.
func TestBookPublishAcceptsASpotBook(t *testing.T) {
	lane := newLane(t)
	spot := ids.VenueInstrument{Venue: binance, Product: ids.ProductSpot, Symbol: "BTCUSDT"}

	if err := newBookPublisher(t, lane).Publish(bookView(t, spot, acceptOrdinal)); err != nil {
		t.Fatalf("Publish(spot book) = %v, want nil", err)
	}
	if _, ok := lane.Consume(events.SourceID{Instrument: spot, Stream: events.StreamBook}); !ok {
		t.Error("nothing reached the spot book mailbox")
	}
}

func TestBookPublishRefuses(t *testing.T) {
	// Each case breaks exactly one thing, and the cases split across the two
	// checks Publish makes — the envelope probe and the domain validation — so
	// an implementation that dropped either is caught. A refused view must not
	// reach the mailbox at all, which the shared assertion below states.
	cases := map[string]struct {
		mutate func(*ports.BookView)
		want   error
	}{
		"unset instrument": {
			// Caught by the SOURCE constructor, before the probe.
			func(v *ports.BookView) { v.Book.Instrument = ids.VenueInstrument{} },
			ids.ErrVenueUnset,
		},
		"no receive time": {
			// Refused by BOTH checks — the probe requires it on an envelope
			// because it is ours to stamp, and domain.Watermark requires it
			// because freshness is measured from it. Either sentinel is a
			// correct answer, so this asserts the one the ORDER produces: the
			// probe runs first, and if that ever changes this test says so.
			func(v *ports.BookView) { v.Book.Watermark.RecvTime = time.Time{} },
			events.ErrRecvTimeUnset,
		},
		"no source time": {
			// The case only the DOMAIN check can catch: SourceTime is OPTIONAL
			// on an envelope, because not every venue message carries one, while
			// domain.Watermark requires it — an observation with no venue stamp
			// cannot answer DataAge, and the venue-side stall is what that
			// measurement exists to find. Skipping the domain check would let
			// this reach the lane looking perfectly well-formed.
			func(v *ports.BookView) { v.Book.Watermark.SourceTime = time.Time{} },
			domain.ErrWatermarkUnset,
		},
		"crossed book": {
			// The other domain-only case: an envelope cannot see that the best
			// bid is above the best ask, and a zero-or-negative spread is the one
			// number a cost-hurdle comparison treats as free execution.
			func(v *ports.BookView) { v.Book.Bids[0] = bookLevel(t, "43260.00", "1") },
			domain.ErrBookCrossed,
		},
		"no venue sequence": {
			// A book at position zero validates as a lane payload and then never
			// advances: every later delta computes SeqRelationUnknown. See
			// domain.ErrBookSeqUnset — this is the refusal that stops a
			// permanent resync loop reaching consumers as a healthy book.
			func(v *ports.BookView) { v.Book.Watermark.VenueSeq = 0 },
			domain.ErrBookSeqUnset,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			lane := newLane(t)
			v := bookView(t, perp(), acceptOrdinal)
			tc.mutate(&v)

			if err := newBookPublisher(t, lane).Publish(v); !errors.Is(err, tc.want) {
				t.Fatalf("Publish = %v, want %v", err, tc.want)
			}
			// Nothing was delivered. A publisher reporting failure after
			// delivering would leave a consumer reading a record its producer
			// believes it rejected.
			if _, ok := lane.Consume(events.SourceID{Instrument: perp(), Stream: events.StreamBook}); ok {
				t.Error("a refused view reached the mailbox")
			}
		})
	}
}
