package driving_test

import (
	"errors"
	"github.com/giiorg/arb-lab/internal/marketdata/application/ports"
	"strings"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/engine/venue"
	"github.com/giiorg/arb-lab/internal/marketdata/application"
	"github.com/giiorg/arb-lab/internal/marketdata/domain"
	"github.com/giiorg/arb-lab/internal/marketdata/infrastructure/driving"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/events"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// The book fixture's sequences, and the SPAN is the point of them.
//
// bookFirstSeq is strictly below bookLastSeq, so the delta covers three venue
// sequences rather than one. That is not realism for its own sake: venue.BookDelta
// carries only FirstSeq and the adapter reads LastSeq off the envelope, so a
// fixture from a single-sequence venue — First == Last, which is what Bybit
// actually sends — makes "LastSeq comes from the envelope" and "LastSeq comes
// from the payload" the same assignment. The whole design of the DTO would be
// untested while looking covered, and domain.BookDelta.Follows already warns
// about the neighbouring version of this trap.
const (
	bookSnapSeq  uint64 = 4242
	bookFirstSeq uint64 = 4243
	bookLastSeq  uint64 = 4245
)

// The book path's own timestamps, chosen so BOTH freshness verdicts land Fresh
// under a sub-second budget. Routing is what this suite is about, and a fixture
// that published Blind views would still pass every assertion here while making
// any later reader think the verdicts were the subject.
//
// bookSourceAt is offset by the venue skew, so DataAge is 100ms against
// ServerNow rather than 90s. A fixture reusing the funding suite's timestamps
// would measure the skew instead of the book.
var (
	bookRecvAt   = base.Add(-50 * time.Millisecond)
	bookSourceAt = base.Add(venueSkew - 100*time.Millisecond)
)

// bookBudget is sub-second, per §3.4's "order books used for execution need
// sub-second budgets". Deliberately unlike the funding budget above: sharing one
// number would make a book path reading the funding budget invisible here.
var bookBudget = domain.Budget{MaxAge: 200 * time.Millisecond, Blind: 2 * time.Second}

const bookHoldDepth = 4

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

func venueLevel(t *testing.T, p, q string) venue.BookLevel {
	t.Helper()
	return venue.BookLevel{Price: bookPrice(t, p), Quantity: bookQty(t, q)}
}

// venueSnap's two halves carry different prices AND different sizes, so a
// translation that assigned asks to Bids is visible as a value rather than only
// as a downstream ordering failure. Two levels a side keeps the ordering real.
func venueSnap(t *testing.T) venue.BookSnapshot {
	t.Helper()
	return venue.BookSnapshot{
		Bids: []venue.BookLevel{venueLevel(t, "43250.10", "0.5"), venueLevel(t, "43250.00", "1.25")},
		Asks: []venue.BookLevel{venueLevel(t, "43251.00", "0.75"), venueLevel(t, "43251.50", "1.5")},
	}
}

// venueDelta touches BOTH sides, which is what makes toDomainSide testable: with
// bid-only changes, "translates the side" and "returns bid unconditionally" give
// the same book.
//
// Neither change is a removal and neither price already exists in venueSnap, so
// both are inserts and both are observable as new levels.
//
// Both prices sit INSIDE the old spread (43250.10 .. 43251.00), which is what
// makes a side-swapping translation fail loudly rather than quietly: swapped,
// the bid change becomes the best ask at 43250.50 and the ask change becomes the
// best bid at 43250.90, and 43250.90 >= 43250.50 is a CROSSED book. Placed
// outside the spread instead, a swap would still produce a perfectly valid book
// and only the value assertions would stand between that and a published
// arbitrage.
func venueDelta(t *testing.T) venue.BookDelta {
	t.Helper()
	return venue.BookDelta{
		FirstSeq: bookFirstSeq,
		Changes: []venue.LevelChange{
			{Side: venue.SideBid, Price: bookPrice(t, "43250.50"), Quantity: bookQty(t, "2")},
			{Side: venue.SideAsk, Price: bookPrice(t, "43250.90"), Quantity: bookQty(t, "3")},
		},
	}
}

func bookEnvelope(inst ids.VenueInstrument, seq uint64, payload any) events.Envelope {
	return events.Envelope{
		// Distinct from every sequence in the fixture, so an adapter that copied
		// a sequence into the ordinal is visible.
		AcceptOrdinal: seq + 11,
		IngressSeq:    seq + 31,
		Source:        events.SourceID{Instrument: inst, Stream: events.StreamBook},
		SourceTime:    bookSourceAt,
		RecvTime:      bookRecvAt,
		VenueSeq:      seq,
		Prio:          events.PrioNormal,
		Payload:       payload,
	}
}

// spyBookPublisher accepts everything and remembers it, so a test can tell "the
// use case ran" from "the adapter returned something plausible".
type spyBookPublisher struct{ got []ports.BookView }

func (s *spyBookPublisher) Publish(v ports.BookView) error {
	s.got = append(s.got, v)
	return nil
}

// fixedBookBudgets is SEPARATE from fixedBudgets rather than a second method on
// it, which is application/ports/'s own argument for splitting the two ports:
// a fake that must implement BookBudget to be handed to FundingIngest is a fake
// asserting a relationship that does not exist.
type fixedBookBudgets struct{ b domain.Budget }

func (f fixedBookBudgets) BookBudget(ids.Venue) (domain.Budget, bool) { return f.b, true }

// spyResnapshotter counts requests. The count is the assertion that matters on
// the recovery paths: BookReconstruct requests on every held delta rather than
// once, and "requested" is not the same claim as "requested again".
type spyResnapshotter struct{ requests []ids.VenueInstrument }

func (s *spyResnapshotter) RequestSnapshot(inst ids.VenueInstrument) error {
	s.requests = append(s.requests, inst)
	return nil
}

type bookHarness struct {
	dispatch *driving.BookDispatch
	clk      *clock.TestClock
	pub      *spyBookPublisher
	resnap   *spyResnapshotter
}

func newBookHarness(t *testing.T) *bookHarness {
	t.Helper()
	clk, err := clock.NewTestClock(base, []clock.Calibration{
		{Venue: binance, Offset: venueSkew, Uncertainty: time.Second},
	})
	if err != nil {
		t.Fatalf("NewTestClock: %v", err)
	}
	pub := &spyBookPublisher{}
	resnap := &spyResnapshotter{}
	svc, err := application.NewBookReconstruct(clk, pub, fixedBookBudgets{b: bookBudget}, resnap, bookHoldDepth)
	if err != nil {
		t.Fatalf("NewBookReconstruct: %v", err)
	}
	d, err := driving.NewBookDispatch(svc)
	if err != nil {
		t.Fatalf("NewBookDispatch: %v", err)
	}
	return &bookHarness{dispatch: d, clk: clk, pub: pub, resnap: resnap}
}

// ---------------------------------------------------------------------------
// Construction
// ---------------------------------------------------------------------------

func TestNewBookDispatchRefusesAMissingUseCase(t *testing.T) {
	if _, err := driving.NewBookDispatch(nil); !errors.Is(err, driving.ErrReconstructNil) {
		t.Fatalf("NewBookDispatch(nil) = %v, want ErrReconstructNil", err)
	}
}

// ---------------------------------------------------------------------------
// Routing: one Deliver, two payloads
// ---------------------------------------------------------------------------

func TestBookDeliverRoutesASnapshotToOnSnapshot(t *testing.T) {
	h := newBookHarness(t)

	view, err := h.dispatch.Deliver(bookEnvelope(perp(), bookSnapSeq, venueSnap(t)))
	if err != nil {
		t.Fatalf("Deliver(snapshot): %v", err)
	}

	// SeqRelationUnknown is what OnSnapshot reports: a full book REPLACED the
	// position rather than following it. Asserting the relation is how this test
	// tells OnSnapshot from OnDelta — both publish a view, and a dispatch that
	// routed a snapshot into OnDelta would fail differently but could equally
	// have been written to succeed.
	if view.Applied != domain.SeqRelationUnknown {
		t.Errorf("Applied = %v, want SeqRelationUnknown — a snapshot replaces, it does not follow", view.Applied)
	}
	if view.Book.Watermark.VenueSeq != bookSnapSeq {
		t.Errorf("book seq = %d, want %d", view.Book.Watermark.VenueSeq, bookSnapSeq)
	}

	// The instrument comes from the SOURCE — the payload carries none — and the
	// levels come from the payload, each half into its own slice.
	if view.Book.Instrument != perp() {
		t.Errorf("Instrument = %v, want the envelope source's %v", view.Book.Instrument, perp())
	}
	assertLevel(t, "best bid", view.Book.Bids[0], "43250.10", "0.5")
	assertLevel(t, "best ask", view.Book.Asks[0], "43251.00", "0.75")

	if len(h.pub.got) != 1 {
		t.Fatalf("%d views published, want 1", len(h.pub.got))
	}
}

// TestBookDeliverAssemblesTheSpanFromBOTHEnds is the central claim of the DTO
// split, and the fixture is what makes it a claim.
//
// The delta covers 4243..4245: FirstSeq from the payload, LastSeq from the
// envelope's VenueSeq. Applied to a book at 4242 that is CONTIGUOUS, and the
// resulting book sits at 4245.
//
// An implementation reading LastSeq from the payload's FirstSeq instead would
// build the span 4243..4243 against a watermark of 4245, which
// domain.BookDelta.Validate refuses as ErrDeltaSeqDisagrees — so the kill is
// loud. That is only true because First != Last here.
func TestBookDeliverAssemblesTheSpanFromBothEnds(t *testing.T) {
	h := newBookHarness(t)
	if _, err := h.dispatch.Deliver(bookEnvelope(perp(), bookSnapSeq, venueSnap(t))); err != nil {
		t.Fatalf("Deliver(snapshot): %v", err)
	}

	view, err := h.dispatch.Deliver(bookEnvelope(perp(), bookLastSeq, venueDelta(t)))
	if err != nil {
		t.Fatalf("Deliver(delta): %v", err)
	}
	if view.Applied != domain.SeqRelationContiguous {
		t.Errorf("Applied = %v, want SeqRelationContiguous — 4243 begins exactly where 4242 ends", view.Applied)
	}
	if view.Book.Watermark.VenueSeq != bookLastSeq {
		t.Errorf("book seq = %d, want %d — the span's far end is the envelope's",
			view.Book.Watermark.VenueSeq, bookLastSeq)
	}
	if len(h.resnap.requests) != 0 {
		t.Errorf("%d resnapshots requested, want 0 — a contiguous delta is the steady state",
			len(h.resnap.requests))
	}
}

// TestBookDeliverRefusesAFirstSeqPastTheEnvelope pins the one disagreement the
// FirstSeq-only split leaves possible, and shows it needs no sentinel of its own.
//
// A payload claiming to begin at 4246 on an envelope whose VenueSeq is 4245 is
// an inverted span, and that is exactly what domain calls it. The alternative
// design — carrying both ends in the payload — would have made this a
// payload-internally-consistent message contradicting its envelope, needing a
// third check nobody would have thought to write.
func TestBookDeliverRefusesAFirstSeqPastTheEnvelope(t *testing.T) {
	h := newBookHarness(t)
	if _, err := h.dispatch.Deliver(bookEnvelope(perp(), bookSnapSeq, venueSnap(t))); err != nil {
		t.Fatalf("Deliver(snapshot): %v", err)
	}

	d := venueDelta(t)
	d.FirstSeq = bookLastSeq + 1
	_, err := h.dispatch.Deliver(bookEnvelope(perp(), bookLastSeq, d))
	if !errors.Is(err, domain.ErrDeltaSeqInverted) {
		t.Fatalf("Deliver = %v, want domain.ErrDeltaSeqInverted", err)
	}
}

// TestBookDeliverTranslatesEachSideToItsOwnHalf is toDomainSide's test, and the
// delta touching BOTH sides is what makes it one.
//
// venueDelta records why both inserted prices sit inside the old spread: a
// change routed to the wrong half does not merely land in the wrong slice, it
// CROSSES the book, so the swap fails even if these value assertions were
// deleted. Both are also the new best on their side, which is what the indices
// below rely on.
func TestBookDeliverTranslatesEachSideToItsOwnHalf(t *testing.T) {
	h := newBookHarness(t)
	if _, err := h.dispatch.Deliver(bookEnvelope(perp(), bookSnapSeq, venueSnap(t))); err != nil {
		t.Fatalf("Deliver(snapshot): %v", err)
	}
	view, err := h.dispatch.Deliver(bookEnvelope(perp(), bookLastSeq, venueDelta(t)))
	if err != nil {
		t.Fatalf("Deliver(delta): %v", err)
	}

	if len(view.Book.Bids) != 3 || len(view.Book.Asks) != 3 {
		t.Fatalf("book is %d bids / %d asks, want 3 / 3", len(view.Book.Bids), len(view.Book.Asks))
	}
	// Each inserted level is the new BEST on its side, because each sits inside
	// the old spread.
	assertLevel(t, "best bid", view.Book.Bids[0], "43250.50", "2")
	assertLevel(t, "best ask", view.Book.Asks[0], "43250.90", "3")
}

// ---------------------------------------------------------------------------
// The refusals
// ---------------------------------------------------------------------------

func TestBookDeliverRefusesAForeignStream(t *testing.T) {
	// A BOOK payload on the FUNDING stream. The lane check one layer down would
	// pass — funding and book share the market-data lane — which is why the
	// boundary here is the stream.
	h := newBookHarness(t)
	env := bookEnvelope(perp(), bookSnapSeq, venueSnap(t))
	env.Source.Stream = events.StreamFunding

	if _, err := h.dispatch.Deliver(env); !errors.Is(err, driving.ErrNotBookStream) {
		t.Fatalf("Deliver = %v, want ErrNotBookStream", err)
	}
}

func TestBookDeliverRefusesAnUnhandledPayload(t *testing.T) {
	h := newBookHarness(t)
	snap := venueSnap(t)

	cases := map[string]any{
		// A funding payload correctly routed to the book stream is the misroute
		// that the stream check cannot see, because the stream is right.
		"funding payload": update(t),

		// A POINTER to the right type. Refused deliberately: accepting both
		// spellings would give one payload two, one of which shares memory with
		// its producer, on a lane whose contract is that a published envelope is
		// not mutated afterwards.
		"pointer to a snapshot": &snap,

		// The domain type, which is what a consumer of this lane receives. It
		// must not be accepted as an input: that would let a republished view
		// re-enter reconstruction as though it were a venue message.
		"domain snapshot": domain.BookSnapshot{},
	}
	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			env := bookEnvelope(perp(), bookSnapSeq, payload)
			_, err := h.dispatch.Deliver(env)
			if !errors.Is(err, driving.ErrBookPayloadType) {
				t.Fatalf("Deliver = %v, want ErrBookPayloadType", err)
			}
			// The %T is the whole diagnosis for this refusal — one sentinel
			// covers both payload types, so the message is what says which
			// wrong one arrived.
			if got := err.Error(); !strings.Contains(got, "payload ") {
				t.Errorf("Deliver = %q, want it to name the payload type", got)
			}
		})
	}
}

// TestBookDeliverRefusesASnapshotWithNoVenueSequence is the regression test for
// a defect this wiring exposed.
//
// domain.Watermark.Validate deliberately admits a zero VenueSeq, because a
// REST-polled funding read has none. A BOOK at position zero is a different
// thing: it validates, it is marked synced, and then every delta after it
// computes SeqRelationUnknown and is refused as ErrAwaitingSnapshot forever
// while the feed reports connected. The second half of this test is what makes
// that claim rather than the first.
func TestBookDeliverRefusesASnapshotWithNoVenueSequence(t *testing.T) {
	h := newBookHarness(t)

	_, err := h.dispatch.Deliver(bookEnvelope(perp(), 0, venueSnap(t)))
	if !errors.Is(err, domain.ErrBookSeqUnset) {
		t.Fatalf("Deliver(snapshot at seq 0) = %v, want domain.ErrBookSeqUnset", err)
	}

	// Nothing was published and nothing was synced, so a following delta is
	// held awaiting a snapshot rather than applied to a book at position zero.
	if len(h.pub.got) != 0 {
		t.Errorf("%d views published, want 0 — a book with no position must not reach the lane", len(h.pub.got))
	}
	if _, err := h.dispatch.Deliver(bookEnvelope(perp(), bookLastSeq, venueDelta(t))); !errors.Is(err, application.ErrAwaitingSnapshot) {
		t.Errorf("Deliver(delta) = %v, want ErrAwaitingSnapshot", err)
	}
}

func TestBookDeliverRefusesAMalformedVenuePayload(t *testing.T) {
	// Refused at the VENUE seam — venue.ErrLevelQuantityNotPositive, not a
	// domain error — which is what puts the operator at the adapter that decoded
	// the message rather than at this context's translation of it.
	h := newBookHarness(t)
	snap := venueSnap(t)
	snap.Asks = append([]venue.BookLevel(nil), snap.Asks...)
	snap.Asks[1] = venue.BookLevel{Price: bookPrice(t, "43251.50"), Quantity: bookQty(t, "0")}

	if _, err := h.dispatch.Deliver(bookEnvelope(perp(), bookSnapSeq, snap)); !errors.Is(err, venue.ErrLevelQuantityNotPositive) {
		t.Fatalf("Deliver = %v, want venue.ErrLevelQuantityNotPositive", err)
	}
}

// TestBookDeliverRefusesAMalformedVenueDelta is the snapshot case's mirror, and
// it was written because tools/mutate.sh reported the delta's venue-seam check
// as SURVIVED while the snapshot's was covered.
//
// The fault is a NEGATIVE quantity specifically, and that choice is the test. A
// bad SIDE would be caught by toDomainSide during translation and would report
// the same venue sentinel either way, so it cannot tell whether the payload was
// judged at all. A negative quantity is checked in exactly one place before the
// domain sees it — venue.LevelChange.Validate, reached only through
// BookDelta.Validate — so a domain sentinel here means the venue seam was
// skipped, which is the boundary distinction this whole redundancy exists for.
func TestBookDeliverRefusesAMalformedVenueDelta(t *testing.T) {
	h := newBookHarness(t)
	if _, err := h.dispatch.Deliver(bookEnvelope(perp(), bookSnapSeq, venueSnap(t))); err != nil {
		t.Fatalf("Deliver(snapshot): %v", err)
	}

	d := venueDelta(t)
	d.Changes = append([]venue.LevelChange(nil), d.Changes...)
	d.Changes[1].Quantity = bookQty(t, "-1")

	_, err := h.dispatch.Deliver(bookEnvelope(perp(), bookLastSeq, d))
	if !errors.Is(err, venue.ErrChangeQuantityNegative) {
		t.Fatalf("Deliver = %v, want venue.ErrChangeQuantityNegative — a domain sentinel here means "+
			"the payload reached the domain unjudged", err)
	}
	if errors.Is(err, domain.ErrChangeQuantityNegative) {
		t.Error("the refusal is the DOMAIN's, which names this context's translation rather than the venue message")
	}
}

func TestBookDeliverRefusesAnUnstampedEnvelope(t *testing.T) {
	// A market-data mailbox holds PRE-dispatch envelopes, so what reaches a
	// context handler has both sequences stamped. An observation accepted with
	// no IngressSeq is one a replay cannot address.
	h := newBookHarness(t)
	env := bookEnvelope(perp(), bookSnapSeq, venueSnap(t))
	env.IngressSeq = 0

	if _, err := h.dispatch.Deliver(env); !errors.Is(err, events.ErrIngressSeqUnset) {
		t.Fatalf("Deliver = %v, want events.ErrIngressSeqUnset", err)
	}
}

// TestBookDeliverAcceptsASpotBook is the end-to-end half of
// TestBookSourceAcceptsSpot, and it is the case a book path copied from the
// funding path would drop.
//
// §4.2 prices the spot leg from its book, so a dispatch that inherited funding's
// perp-only assumption would silently unsubscribe half of every cash-and-carry
// trade. Nothing else in this file passes a spot instrument.
func TestBookDeliverAcceptsASpotBook(t *testing.T) {
	h := newBookHarness(t)
	spot := ids.VenueInstrument{Venue: binance, Product: ids.ProductSpot, Symbol: "BTCUSDT"}

	view, err := h.dispatch.Deliver(bookEnvelope(spot, bookSnapSeq, venueSnap(t)))
	if err != nil {
		t.Fatalf("Deliver(spot book) = %v, want nil — §4.2 prices the spot leg from this", err)
	}
	if view.Book.Instrument.Product != ids.ProductSpot {
		t.Errorf("Product = %q, want %q", view.Book.Instrument.Product, ids.ProductSpot)
	}
}

// ---------------------------------------------------------------------------
// Timers
// ---------------------------------------------------------------------------

// TestBookTimerRoutesToTheBookUseCase also shows the two Timer methods are
// distinct namespaces rather than one queue read twice: the ID armed by a book
// delivery is unknown to the funding dispatch, and that is what stops each
// class's arm from cancelling the other's.
func TestBookTimerRoutesToTheBookUseCase(t *testing.T) {
	h := newBookHarness(t)
	if _, err := h.dispatch.Deliver(bookEnvelope(perp(), bookSnapSeq, venueSnap(t))); err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if h.clk.Pending() != 1 {
		t.Fatalf("%d timers armed, want 1", h.clk.Pending())
	}

	if _, ok := h.clk.AdvanceToNextTimer(); !ok {
		t.Fatal("no timer to advance to")
	}
	fired, ok := h.clk.NextDue()
	if !ok {
		t.Fatal("NextDue reported nothing due")
	}

	view, err := h.dispatch.Timer(fired.ID)
	if err != nil {
		t.Fatalf("Timer(%q): %v", fired.ID, err)
	}
	// The book is UNCHANGED and the verdict has worsened. That pair is the
	// honest statement a silence timer makes: nothing new was observed, and that
	// is the news.
	if view.Book.Watermark.VenueSeq != bookSnapSeq {
		t.Errorf("book seq = %d, want %d unchanged", view.Book.Watermark.VenueSeq, bookSnapSeq)
	}
	if view.Transport == domain.FreshnessFresh {
		t.Errorf("Transport = %v after the silence timer, want a worse rung", view.Transport)
	}
	// Trigger zero: derived from no input at all. Reusing the last input's
	// ordinal would make a replay report a phantom duplicate and conclude the
	// feed was fine — inverting the meaning of the one event that fired to say
	// it had gone quiet.
	if view.Trigger != 0 {
		t.Errorf("Trigger = %d, want 0 — a timer-derived view inherits no ordinal", view.Trigger)
	}
}

func TestBookTimerRefusesAnIDItDidNotArm(t *testing.T) {
	h := newBookHarness(t)
	if _, err := h.dispatch.Timer(clock.TimerID("someone.elses.timer")); !errors.Is(err, application.ErrUnknownTimer) {
		t.Fatalf("Timer(foreign) = %v, want application.ErrUnknownTimer", err)
	}
}

// assertLevel compares NUMERICALLY rather than by String(), which also checks
// the asset tags survived: Cmp refuses two operands whose tags disagree, so a
// translation that dropped or crossed a tag fails here rather than producing a
// string that merely looks different.
func assertLevel(t *testing.T, what string, l domain.BookLevel, price, qty string) {
	t.Helper()
	if cmp, err := l.Price.Cmp(bookPrice(t, price)); err != nil || cmp != 0 {
		t.Errorf("%s price = %s, want %s (cmp %d, err %v)", what, l.Price, price, cmp, err)
	}
	if cmp, err := l.Quantity.Cmp(bookQty(t, qty)); err != nil || cmp != 0 {
		t.Errorf("%s quantity = %s, want %s (cmp %d, err %v)", what, l.Quantity, qty, cmp, err)
	}
}
