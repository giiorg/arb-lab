package application_test

import (
	"errors"
	"github.com/giiorg/arb-lab/internal/marketdata/application/ports"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/marketdata/application"
	"github.com/giiorg/arb-lab/internal/marketdata/domain"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// bookBudget is SUB-SECOND, per §3.4, and its bounds differ from the funding
// budget in this package on purpose: a reconstructor reading FundingBudget would
// otherwise produce identical freshness verdicts and fail nothing.
var bookBudget = domain.Budget{MaxAge: 250 * time.Millisecond, Blind: 2 * time.Second}

// holdDepth is 3, which is the smallest depth that distinguishes the three
// buffer outcomes: below capacity, exactly at it, and past it. A depth of 1
// would make "the buffer is full" and "the buffer holds the only delta" the same
// state, and a depth of 2 would make the discarded set and the retained set the
// same size.
const holdDepth = 3

func bprice(t *testing.T, s string) fixedpoint.Price {
	t.Helper()
	p, err := fixedpoint.ParsePrice("BTC", "USDT", s)
	if err != nil {
		t.Fatalf("ParsePrice(%q): %v", s, err)
	}
	return p
}

func bqty(t *testing.T, s string) fixedpoint.Quantity {
	t.Helper()
	q, err := fixedpoint.ParseQuantity("BTC", s)
	if err != nil {
		t.Fatalf("ParseQuantity(%q): %v", s, err)
	}
	return q
}

// bookAt is a valid three-a-side book positioned at a venue sequence.
func bookAt(t *testing.T, seq uint64, recvAt, sourceAt time.Time) domain.BookSnapshot {
	t.Helper()
	lvl := func(p, q string) domain.BookLevel {
		return domain.BookLevel{Price: bprice(t, p), Quantity: bqty(t, q)}
	}
	return domain.BookSnapshot{
		Instrument: perp(),
		Bids:       []domain.BookLevel{lvl("100", "1"), lvl("99", "2"), lvl("98", "3")},
		Asks:       []domain.BookLevel{lvl("101", "4"), lvl("102", "5"), lvl("103", "6")},
		Watermark:  domain.Watermark{VenueSeq: seq, SourceTime: sourceAt, RecvTime: recvAt},
	}
}

// deltaAt is a valid update spanning first..last that touches one bid level.
//
// The span is a parameter rather than a single sequence because
// domain.SeqRelationOverlap is unreachable for a single-sequence delta, and
// overlap is the state every resnapshot lands in.
func deltaAt(t *testing.T, first, last uint64, recvAt, sourceAt time.Time) domain.BookDelta {
	t.Helper()
	return domain.BookDelta{
		Instrument: perp(),
		Changes: []domain.LevelChange{
			{Side: domain.SideBid, Price: bprice(t, "99"), Quantity: bqty(t, "20")},
		},
		FirstSeq:  first,
		LastSeq:   last,
		Watermark: domain.Watermark{VenueSeq: last, SourceTime: sourceAt, RecvTime: recvAt},
	}
}

// spyBookPublisher records what was published and can be told to fail.
type spyBookPublisher struct {
	got  []ports.BookView
	fail error
}

func (s *spyBookPublisher) Publish(v ports.BookView) error {
	s.got = append(s.got, v)
	return s.fail
}

// spyResnapshotter counts requests and can be told to fail.
//
// It counts rather than recording a bool, because the recovery's correctness is
// about how MANY requests a held stream produces: requesting once and never
// again is the failure that cannot recover, and requesting per delta is what
// makes the adapter's idempotence load-bearing.
type spyResnapshotter struct {
	on   []ids.VenueInstrument
	fail error
}

func (s *spyResnapshotter) RequestSnapshot(i ids.VenueInstrument) error {
	s.on = append(s.on, i)
	return s.fail
}

type fixedBookBudgets struct {
	b    domain.Budget
	on   ids.Venue
	gone bool
}

func (f *fixedBookBudgets) BookBudget(v ids.Venue) (domain.Budget, bool) {
	if f.gone || v != f.on {
		return domain.Budget{}, false
	}
	return f.b, true
}

type bookHarness struct {
	svc *application.BookReconstruct
	clk *clock.TestClock
	pub *spyBookPublisher
	bud *fixedBookBudgets
	res *spyResnapshotter
}

func newBookHarness(t *testing.T) *bookHarness {
	t.Helper()
	clk, err := clock.NewTestClock(base, []clock.Calibration{
		{Venue: venue, Offset: venueSkew, Uncertainty: time.Second},
	})
	if err != nil {
		t.Fatalf("NewTestClock: %v", err)
	}
	h := &bookHarness{
		clk: clk,
		pub: &spyBookPublisher{},
		bud: &fixedBookBudgets{b: bookBudget, on: venue},
		res: &spyResnapshotter{},
	}
	h.svc, err = application.NewBookReconstruct(clk, h.pub, h.bud, h.res, holdDepth)
	if err != nil {
		t.Fatalf("NewBookReconstruct: %v", err)
	}
	return h
}

// sync brings the harness to a synced book at seq, with fresh timestamps.
func (h *bookHarness) sync(t *testing.T, seq uint64) {
	t.Helper()
	if _, err := h.svc.OnSnapshot(bookAt(t, seq, base, base.Add(venueSkew)), anyOrdinal); err != nil {
		t.Fatalf("OnSnapshot(seq %d): %v", seq, err)
	}
}

// ---------------------------------------------------------------------------
// Construction
// ---------------------------------------------------------------------------

func TestNewBookReconstructRefusesMissingDependencies(t *testing.T) {
	clk, err := clock.NewTestClock(base, nil)
	if err != nil {
		t.Fatalf("NewTestClock: %v", err)
	}
	pub := &spyBookPublisher{}
	bud := &fixedBookBudgets{b: bookBudget, on: venue}
	res := &spyResnapshotter{}

	// Each case omits exactly ONE dependency, so a constructor that checked only
	// the first is caught by the others.
	cases := []struct {
		name  string
		c     clock.Clock
		p     ports.BookPublisher
		b     ports.BookBudgets
		r     ports.Resnapshotter
		depth int
		want  error
	}{
		{"no clock", nil, pub, bud, res, holdDepth, application.ErrClockNil},
		{"no publisher", clk, nil, bud, res, holdDepth, application.ErrBookPublisherNil},
		{"no budgets", clk, pub, nil, res, holdDepth, application.ErrBookBudgetsNil},
		{"no resnapshotter", clk, pub, bud, nil, holdDepth, application.ErrResnapshotterNil},
		// Zero AND negative. Zero is the dangerous one — it holds nothing, so
		// every resnapshot lands behind the stream and the recovery never
		// converges — and a check written as `< 0` would admit it.
		{"zero hold depth", clk, pub, bud, res, 0, application.ErrHoldDepthInvalid},
		{"negative hold depth", clk, pub, bud, res, -1, application.ErrHoldDepthInvalid},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			_, err := application.NewBookReconstruct(c.c, c.p, c.b, c.r, c.depth)
			if !errors.Is(err, c.want) {
				t.Errorf("NewBookReconstruct = %v, want %v", err, c.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// The four relations
// ---------------------------------------------------------------------------

func TestOnDeltaAppliesAContiguousUpdate(t *testing.T) {
	t.Parallel()

	h := newBookHarness(t)
	h.sync(t, 1000)

	view, err := h.svc.OnDelta(deltaAt(t, 1001, 1005, base, base.Add(venueSkew)), 88)
	if err != nil {
		t.Fatalf("OnDelta = %v, want nil", err)
	}
	if view.Applied != domain.SeqRelationContiguous {
		t.Errorf("Applied = %s, want SeqRelationContiguous", view.Applied)
	}
	if view.Book.Watermark.VenueSeq != 1005 {
		t.Errorf("book seq = %d, want 1005", view.Book.Watermark.VenueSeq)
	}
	// The delta's change actually landed, not merely the sequence: a
	// reconstructor that advanced the watermark without applying would satisfy
	// every sequence assertion and publish a stale book.
	if got := view.Book.Bids[1].Quantity.Decimal(); got != "20" {
		t.Errorf("bid[1] quantity = %s, want 20 — the change must be APPLIED, not just counted", got)
	}
	if view.Trigger != 88 {
		t.Errorf("Trigger = %d, want 88", view.Trigger)
	}
	if len(h.res.on) != 0 {
		t.Errorf("requested %d snapshots on a clean apply, want 0", len(h.res.on))
	}
	if len(h.pub.got) != 2 {
		t.Fatalf("published %d views, want 2 (the snapshot and the delta)", len(h.pub.got))
	}
}

func TestOnDeltaAppliesAnOverlappingUpdate(t *testing.T) {
	t.Parallel()

	h := newBookHarness(t)
	h.sync(t, 1000)

	// Starts AT the book's position and runs past it, which is what the first
	// delta after a REST snapshot looks like. A reconstructor admitting only the
	// contiguous case refuses this and re-requests forever.
	view, err := h.svc.OnDelta(deltaAt(t, 1000, 1005, base, base.Add(venueSkew)), anyOrdinal)
	if err != nil {
		t.Fatalf("OnDelta(overlap) = %v, want nil — overlap is the NORMAL post-resnapshot state", err)
	}
	if view.Applied != domain.SeqRelationOverlap {
		t.Errorf("Applied = %s, want SeqRelationOverlap", view.Applied)
	}
	if len(h.res.on) != 0 {
		t.Errorf("requested %d snapshots on an overlap, want 0", len(h.res.on))
	}
}

func TestOnDeltaDropsAStaleUpdateWithoutResnapshotting(t *testing.T) {
	t.Parallel()

	h := newBookHarness(t)
	h.sync(t, 1000)

	_, err := h.svc.OnDelta(deltaAt(t, 995, 999, base, base.Add(venueSkew)), anyOrdinal)
	if !errors.Is(err, application.ErrSuperseded) {
		t.Fatalf("OnDelta(stale) = %v, want ErrSuperseded", err)
	}
	// The claim that matters: a redelivery is NOT a desync. An at-least-once
	// transport produces these on every reconnect, and requesting a snapshot for
	// each would turn a healthy recovery into a REST storm.
	if len(h.res.on) != 0 {
		t.Errorf("requested %d snapshots for a redelivery, want 0", len(h.res.on))
	}
	if len(h.pub.got) != 1 {
		t.Errorf("published %d views, want 1 — a stale delta publishes nothing", len(h.pub.got))
	}
}

func TestOnDeltaGapHoldsAndResnapshots(t *testing.T) {
	t.Parallel()

	h := newBookHarness(t)
	h.sync(t, 1000)

	// Exactly ONE sequence missing: the minimal gap, which is what separates
	// `FirstSeq > prev+1` from a check with a wider tolerance.
	_, err := h.svc.OnDelta(deltaAt(t, 1002, 1005, base, base.Add(venueSkew)), anyOrdinal)
	if !errors.Is(err, application.ErrSequenceGap) {
		t.Fatalf("OnDelta(gap) = %v, want ErrSequenceGap", err)
	}
	if len(h.res.on) != 1 || h.res.on[0] != perp() {
		t.Fatalf("resnapshot requests = %v, want exactly one for %s", h.res.on, perp())
	}
	if len(h.pub.got) != 1 {
		t.Errorf("published %d views, want 1 — a gapped delta must not publish", len(h.pub.got))
	}

	// And the book is now UNSYNCED: the next delta is held rather than applied,
	// even though it follows the gapped one contiguously. Without this the
	// desync would be a one-message hiccup rather than a state.
	_, err = h.svc.OnDelta(deltaAt(t, 1006, 1008, base, base.Add(venueSkew)), anyOrdinal)
	if !errors.Is(err, application.ErrAwaitingSnapshot) {
		t.Errorf("OnDelta after a gap = %v, want ErrAwaitingSnapshot", err)
	}
}

func TestOnDeltaTreatsADivergentResultAsADesync(t *testing.T) {
	t.Parallel()

	h := newBookHarness(t)
	h.sync(t, 1000)

	// The sequence is perfect and the RESULT is not a book: a bid resting above
	// the best ask with nothing clearing it. Every message arrived, so this is a
	// different diagnosis from a gap even though the recovery is identical.
	d := deltaAt(t, 1001, 1005, base, base.Add(venueSkew))
	d.Changes = []domain.LevelChange{
		{Side: domain.SideBid, Price: bprice(t, "105"), Quantity: bqty(t, "1")},
	}

	_, err := h.svc.OnDelta(d, anyOrdinal)
	if !errors.Is(err, application.ErrBookDesynced) {
		t.Fatalf("OnDelta(crossing) = %v, want ErrBookDesynced", err)
	}
	// The underlying domain refusal is carried, not swallowed: it is what says
	// WHICH invariant broke, and an operator reading only "desynced" learns
	// nothing about whether to look at the transport or the decoder.
	if !errors.Is(err, domain.ErrBookCrossed) {
		t.Errorf("OnDelta(crossing) = %v, want it to carry domain.ErrBookCrossed", err)
	}
	if len(h.res.on) != 1 {
		t.Errorf("resnapshot requests = %d, want 1", len(h.res.on))
	}
	if len(h.pub.got) != 1 {
		t.Errorf("published %d views, want 1 — a divergent book must not reach the lane", len(h.pub.got))
	}
}

// ---------------------------------------------------------------------------
// Resync: the drain is the reason the buffer exists
// ---------------------------------------------------------------------------

// TestResyncDrainsHeldDeltasOntoTheSnapshot is the central claim of this file.
//
// A REST snapshot is taken at a sequence the live stream has already passed. The
// deltas that arrived during the fetch are exactly what carries the book from
// the snapshot's position to the present, so a reconstructor that discarded them
// would publish a book that is already behind, and the NEXT delta would gap
// against it — a loop that never converges while every metric reports a
// connected feed.
//
// The fixture is built so a discard is DETECTABLE: the held deltas move the book
// to sequence 1012, which the snapshot alone (1004) cannot reach.
func TestResyncDrainsHeldDeltasOntoTheSnapshot(t *testing.T) {
	t.Parallel()

	h := newBookHarness(t)
	h.sync(t, 1000)

	// A gap puts the book into recovery and holds the delta that exposed it.
	if _, err := h.svc.OnDelta(deltaAt(t, 1006, 1008, base, base.Add(venueSkew)), anyOrdinal); !errors.Is(err, application.ErrSequenceGap) {
		t.Fatalf("seeding the gap: %v", err)
	}
	// One more arrives while the REST fetch is in flight.
	if _, err := h.svc.OnDelta(deltaAt(t, 1009, 1012, base, base.Add(venueSkew)), anyOrdinal); !errors.Is(err, application.ErrAwaitingSnapshot) {
		t.Fatalf("seeding the second held delta: %v", err)
	}

	// The snapshot comes back positioned BEHIND both held deltas, which is the
	// realistic case and the one that makes the drain necessary.
	view, err := h.svc.OnSnapshot(bookAt(t, 1005, base, base.Add(venueSkew)), 99)
	if err != nil {
		t.Fatalf("OnSnapshot = %v, want nil", err)
	}
	if view.Book.Watermark.VenueSeq != 1012 {
		t.Errorf("book seq = %d, want 1012 — the held deltas must be DRAINED onto the snapshot, not discarded",
			view.Book.Watermark.VenueSeq)
	}
	// Published once for the resync, carrying the drained position — not twice,
	// which would put a known-stale REST book on the lane where latest-value
	// coalescing might keep it.
	if len(h.pub.got) != 2 {
		t.Fatalf("published %d views in total, want 2 (initial sync + resync)", len(h.pub.got))
	}
	if got := h.pub.got[1].Book.Watermark.VenueSeq; got != 1012 {
		t.Errorf("published seq = %d, want 1012", got)
	}
	// A full snapshot REPLACED the book rather than following it, which is
	// exactly what SeqRelationUnknown means.
	if view.Applied != domain.SeqRelationUnknown {
		t.Errorf("Applied = %s, want SeqRelationUnknown — a snapshot replaces, it does not follow", view.Applied)
	}

	// And the stream continues from the DRAINED position, not the snapshot's.
	if _, err := h.svc.OnDelta(deltaAt(t, 1013, 1015, base, base.Add(venueSkew)), anyOrdinal); err != nil {
		t.Errorf("OnDelta after resync = %v, want nil", err)
	}
}

func TestResyncDropsHeldDeltasTheSnapshotAlreadyCovers(t *testing.T) {
	t.Parallel()

	h := newBookHarness(t)
	h.sync(t, 1000)

	if _, err := h.svc.OnDelta(deltaAt(t, 1006, 1008, base, base.Add(venueSkew)), anyOrdinal); !errors.Is(err, application.ErrSequenceGap) {
		t.Fatalf("seeding the gap: %v", err)
	}

	// The snapshot is NEWER than the held delta, so draining must drop it
	// silently — SeqRelationStale, the ordinary case, not a fault. A drain that
	// escalated here would make every fast REST response look like a defect.
	view, err := h.svc.OnSnapshot(bookAt(t, 1020, base, base.Add(venueSkew)), anyOrdinal)
	if err != nil {
		t.Fatalf("OnSnapshot = %v, want nil", err)
	}
	if view.Book.Watermark.VenueSeq != 1020 {
		t.Errorf("book seq = %d, want 1020", view.Book.Watermark.VenueSeq)
	}
}

// TestResyncReHoldsTheSuccessorsOfAGapFoundWhileDraining is the defect that made
// hold take a RUN of deltas rather than one.
//
// If the drain hits a gap it must re-hold the offending delta AND everything
// behind it: the successors are what the next snapshot has to chain to, so
// dropping them guarantees the next drain gaps too. The symptom is not a wrong
// book — it is a recovery loop where every individual step looks correct.
func TestResyncReHoldsTheSuccessorsOfAGapFoundWhileDraining(t *testing.T) {
	t.Parallel()

	h := newBookHarness(t)
	h.sync(t, 1000)

	// THREE held deltas, with the gap in the MIDDLE: 1006..1008, then a hole,
	// then 1010..1012 and 1013..1015.
	//
	// The third one is not padding. With the gap at the last held delta,
	// `held[i:]` and `held[i:i+1]` are the same one-element slice, so the
	// fixture cannot tell "re-hold the run" from "re-hold the offender" — and
	// the version of this test that stopped at two deltas passed against both.
	// The successor is the whole distinction.
	if _, err := h.svc.OnDelta(deltaAt(t, 1006, 1008, base, base.Add(venueSkew)), anyOrdinal); !errors.Is(err, application.ErrSequenceGap) {
		t.Fatalf("seeding the gap: %v", err)
	}
	for _, s := range [][2]uint64{{1010, 1012}, {1013, 1015}} {
		if _, err := h.svc.OnDelta(deltaAt(t, s[0], s[1], base, base.Add(venueSkew)), anyOrdinal); !errors.Is(err, application.ErrAwaitingSnapshot) {
			t.Fatalf("seeding held delta %d..%d: %v", s[0], s[1], err)
		}
	}

	// The first snapshot bridges to 1008 and then gaps on 1010.
	if _, err := h.svc.OnSnapshot(bookAt(t, 1005, base, base.Add(venueSkew)), anyOrdinal); !errors.Is(err, application.ErrSequenceGap) {
		t.Fatalf("OnSnapshot mid-gap = %v, want ErrSequenceGap", err)
	}

	// The second snapshot lands past the hole. Reaching 1015 requires BOTH
	// re-held deltas; re-holding only the offender reaches 1012 and stops.
	view, err := h.svc.OnSnapshot(bookAt(t, 1009, base, base.Add(venueSkew)), anyOrdinal)
	if err != nil {
		t.Fatalf("OnSnapshot after the gap = %v, want nil", err)
	}
	if view.Book.Watermark.VenueSeq != 1015 {
		t.Errorf("book seq = %d, want 1015 — the successors of a mid-drain gap must be RE-HELD",
			view.Book.Watermark.VenueSeq)
	}
}

func TestHoldBufferOverflowIsCountedAndKeepsTheNewest(t *testing.T) {
	t.Parallel()

	h := newBookHarness(t)
	h.sync(t, 1000)

	// Four deltas into a depth-3 buffer: the first gaps, the rest are held, and
	// the fourth pushes past capacity. The spans are contiguous WITH EACH OTHER
	// so the retained set is a chain a later snapshot can actually drain — a
	// fixture with holes between them could not distinguish which three were
	// kept, because every candidate set would gap somewhere.
	spans := [][2]uint64{{1006, 1008}, {1009, 1011}, {1012, 1014}, {1015, 1017}}
	var lastErr error
	for _, s := range spans {
		_, lastErr = h.svc.OnDelta(deltaAt(t, s[0], s[1], base, base.Add(venueSkew)), anyOrdinal)
	}
	if !errors.Is(lastErr, application.ErrHoldOverflow) {
		t.Fatalf("the fourth held delta = %v, want ErrHoldOverflow", lastErr)
	}

	// Recorded, never silent: §3.5 makes a drop on the capture path a RECORDED
	// gap. A reconstructor that quietly shed held deltas presents a resync loop
	// as a slow venue.
	n, ok := h.svc.Overflows(perp())
	if !ok {
		t.Fatal("Overflows reported an unknown instrument")
	}
	if n != 1 {
		t.Errorf("Overflows = %d, want 1", n)
	}

	// The newest THREE were kept — 1009..1017 — and the assertion is built to
	// separate all three candidate policies at once. A snapshot at 1008 drains:
	//
	//   keep newest depth (correct): 1009..1011, 1012..1014, 1015..1017 -> 1017
	//   keep OLDEST depth:           1006..1008 stale, then to 1014     -> 1014
	//   discard all, keep only ds:   1015..1017 gaps against 1008       -> refused
	//
	// The third is what "clear the buffer and start from the new delta" does,
	// and it reads as equivalent to the first until a fixture overflows with a
	// run shorter than the buffer — which is the only way it ever overflows.
	view, err := h.svc.OnSnapshot(bookAt(t, 1008, base, base.Add(venueSkew)), anyOrdinal)
	if err != nil {
		t.Fatalf("OnSnapshot = %v, want nil", err)
	}
	if view.Book.Watermark.VenueSeq != 1017 {
		t.Errorf("book seq = %d, want 1017 — overflow must keep the NEWEST %d held deltas, not one and not the oldest",
			view.Book.Watermark.VenueSeq, holdDepth)
	}
}

func TestHoldRequestsASnapshotForEveryHeldDelta(t *testing.T) {
	t.Parallel()

	h := newBookHarness(t)
	h.sync(t, 1000)

	for _, s := range [][2]uint64{{1006, 1008}, {1009, 1011}} {
		if _, err := h.svc.OnDelta(deltaAt(t, s[0], s[1], base, base.Add(venueSkew)), anyOrdinal); err == nil {
			t.Fatalf("OnDelta(%d..%d) = nil, want a refusal", s[0], s[1])
		}
	}
	// Every held delta re-requests. Requesting once and never again is the
	// failure that cannot recover: a dropped or rate-limited request leaves the
	// book held forever with nothing retrying, and idempotence belongs in the
	// adapter, which is the only thing that knows the venue's §7.6 budget.
	if len(h.res.on) != 2 {
		t.Errorf("resnapshot requests = %d, want 2 — one per held delta", len(h.res.on))
	}
}

func TestHoldReportsAFailedSnapshotRequest(t *testing.T) {
	t.Parallel()

	h := newBookHarness(t)
	h.sync(t, 1000)
	h.res.fail = errors.New("rate limited")

	_, err := h.svc.OnDelta(deltaAt(t, 1002, 1005, base, base.Add(venueSkew)), anyOrdinal)
	// BOTH survive the wrap: the gap is why a snapshot was wanted and the
	// failure is why there will not be one. Reporting either alone leaves a
	// reader unable to tell a feed problem from a REST problem.
	if !errors.Is(err, application.ErrSequenceGap) {
		t.Errorf("OnDelta = %v, want it to carry ErrSequenceGap", err)
	}
	if !errors.Is(err, h.res.fail) {
		t.Errorf("OnDelta = %v, want it to carry the request failure", err)
	}
}

// ---------------------------------------------------------------------------
// Refusals and provenance
// ---------------------------------------------------------------------------

func TestBookRefusesAMissingTrigger(t *testing.T) {
	t.Parallel()

	h := newBookHarness(t)

	// Both entry points, because the sentinel's whole job is to keep zero
	// readable as "derived from no input" — and a check on one of two input
	// paths leaves the other minting the value that only OnSilence may.
	if _, err := h.svc.OnSnapshot(bookAt(t, 1000, base, base.Add(venueSkew)), 0); !errors.Is(err, application.ErrNoTrigger) {
		t.Errorf("OnSnapshot(trigger 0) = %v, want ErrNoTrigger", err)
	}
	h.sync(t, 1000)
	if _, err := h.svc.OnDelta(deltaAt(t, 1001, 1005, base, base.Add(venueSkew)), 0); !errors.Is(err, application.ErrNoTrigger) {
		t.Errorf("OnDelta(trigger 0) = %v, want ErrNoTrigger", err)
	}
}

func TestBookRefusesAVenueWithNoBudget(t *testing.T) {
	t.Parallel()

	h := newBookHarness(t)
	h.bud.on = "bybit" // configured for a different venue

	_, err := h.svc.OnSnapshot(bookAt(t, 1000, base, base.Add(venueSkew)), anyOrdinal)
	if !errors.Is(err, application.ErrNoBookBudget) {
		t.Errorf("OnSnapshot = %v, want ErrNoBookBudget", err)
	}
}

func TestBookRefusesAMalformedInput(t *testing.T) {
	t.Parallel()

	h := newBookHarness(t)

	bad := bookAt(t, 1000, base, base.Add(venueSkew))
	bad.Bids[1] = domain.BookLevel{Price: bprice(t, "101"), Quantity: bqty(t, "2")} // ascending bids
	if _, err := h.svc.OnSnapshot(bad, anyOrdinal); !errors.Is(err, domain.ErrBookUnordered) {
		t.Errorf("OnSnapshot(unordered) = %v, want domain.ErrBookUnordered", err)
	}

	h.sync(t, 1000)
	badDelta := deltaAt(t, 1001, 1005, base, base.Add(venueSkew))
	badDelta.Watermark.VenueSeq = 1004
	if _, err := h.svc.OnDelta(badDelta, anyOrdinal); !errors.Is(err, domain.ErrDeltaSeqDisagrees) {
		t.Errorf("OnDelta(seq disagreement) = %v, want domain.ErrDeltaSeqDisagrees", err)
	}
}

func TestBookReportsAPublishFailureWithTheViewIntact(t *testing.T) {
	t.Parallel()

	h := newBookHarness(t)
	h.pub.fail = errors.New("lane closed")

	view, err := h.svc.OnSnapshot(bookAt(t, 1000, base, base.Add(venueSkew)), anyOrdinal)
	if err == nil {
		t.Fatal("OnSnapshot = nil, want the publish failure")
	}
	// The view is returned ALONGSIDE the error, matching the funding path: the
	// observation happened, and a caller that wants to log what could not be
	// published needs it.
	if view.Book.Watermark.VenueSeq != 1000 {
		t.Errorf("view seq = %d, want 1000 — the view must survive a publish failure", view.Book.Watermark.VenueSeq)
	}
	// And the book stayed synced, so the next delta applies rather than
	// re-triggering a resnapshot. Rolling back on a publish failure would make a
	// lane outage look like a feed desync.
	if _, err := h.svc.OnDelta(deltaAt(t, 1001, 1005, base, base.Add(venueSkew)), anyOrdinal); !errors.Is(err, h.pub.fail) {
		t.Errorf("OnDelta after a failed publish = %v, want only the publish failure", err)
	}
}

// ---------------------------------------------------------------------------
// Freshness and the silence timer
// ---------------------------------------------------------------------------

// TestBookFreshnessUsesBothClocks is the assertion the venue skew exists for.
//
// The book's SourceTime is deliberately NOT advanced with its RecvTime, so the
// venue's own view is frozen while the transport is healthy. Transport reads
// fresh and Data does not — and an implementation measuring both against
// Clock.Now, which is §3.5's forbidden comparison, produces two identical
// verdicts here.
func TestBookFreshnessUsesBothClocks(t *testing.T) {
	t.Parallel()

	h := newBookHarness(t)

	// Received now; the venue stamped it a second of server time ago, which is
	// past the 250ms max-age and past the 2s blind bound only if measured
	// against server time.
	stalled := bookAt(t, 1000, base, base.Add(venueSkew).Add(-time.Second))
	view, err := h.svc.OnSnapshot(stalled, anyOrdinal)
	if err != nil {
		t.Fatalf("OnSnapshot = %v, want nil", err)
	}
	if view.Transport != domain.FreshnessFresh {
		t.Errorf("Transport = %s, want FreshnessFresh — nothing is wrong with the transport", view.Transport)
	}
	if view.Data != domain.FreshnessStale {
		t.Errorf("Data = %s, want FreshnessStale — the venue's own view is a second old", view.Data)
	}
	// The reduction a trade decision reads is the WORSE of the two, which is the
	// whole reason both are carried.
	if got := view.Least(); got != domain.FreshnessStale {
		t.Errorf("Least() = %s, want FreshnessStale", got)
	}
}

// TestBookSilenceTimerRepublishesWithAWorseVerdict is §3.4's "detected by a
// timer, not only by the next update" for the data class that gates execution.
//
// A feed that simply stops emits no event, so a freshness check written only on
// arrival is the check a dead feed never triggers — and §3.4 gives books
// sub-second budgets precisely because an order is priced against them.
func TestBookSilenceTimerRepublishesWithAWorseVerdict(t *testing.T) {
	t.Parallel()

	h := newBookHarness(t)
	h.sync(t, 1000)

	if h.pub.got[0].Transport != domain.FreshnessFresh {
		t.Fatalf("initial Transport = %s, want FreshnessFresh", h.pub.got[0].Transport)
	}

	at, ok := h.clk.AdvanceToNextTimer()
	if !ok {
		t.Fatal("no timer was armed — a book feed that goes quiet would never be noticed")
	}
	if !at.After(base) {
		t.Fatalf("timer fired at %s, want after %s", at, base)
	}
	fired, ok := h.clk.NextDue()
	if !ok {
		t.Fatal("AdvanceToNextTimer reported a timer that NextDue does not")
	}

	view, err := h.svc.OnSilence(fired.ID)
	if err != nil {
		t.Fatalf("OnSilence = %v, want nil", err)
	}
	if view.Transport != domain.FreshnessStale {
		t.Errorf("Transport = %s, want FreshnessStale after the max-age rung", view.Transport)
	}
	// Derived from the ABSENCE of an input, so both provenance fields say so.
	// Reusing the last input's ordinal makes a replay report a phantom duplicate
	// and conclude the feed is fine — inverting the meaning of the one event
	// that fired to say it had gone quiet.
	if view.Trigger != 0 {
		t.Errorf("Trigger = %d, want 0 — a silence verdict inherits no ordinal", view.Trigger)
	}
	if view.Applied != domain.SeqRelationUnknown {
		t.Errorf("Applied = %s, want SeqRelationUnknown", view.Applied)
	}
	// The SAME book, not a fabricated one: nothing new was observed, and that is
	// the news.
	if view.Book.Watermark.VenueSeq != 1000 {
		t.Errorf("book seq = %d, want 1000 unchanged", view.Book.Watermark.VenueSeq)
	}
}

func TestBookSilenceTimerRefusesAForeignID(t *testing.T) {
	t.Parallel()

	h := newBookHarness(t)
	h.sync(t, 1000)

	if _, err := h.svc.OnSilence(clock.TimerID("somebody.elses.timer")); !errors.Is(err, application.ErrUnknownTimer) {
		t.Errorf("OnSilence(foreign) = %v, want ErrUnknownTimer", err)
	}
}

// TestBookAndFundingTimersDoNotCollide is the mirror check: SetTimer replaces by
// ID, so the two data classes sharing an ID for one instrument would make each
// arm cancel the other's — and since the budgets differ, the class that fires
// sooner is the one silently lost.
func TestBookAndFundingTimersDoNotCollide(t *testing.T) {
	t.Parallel()

	clk, err := clock.NewTestClock(base, []clock.Calibration{
		{Venue: venue, Offset: venueSkew, Uncertainty: time.Second},
	})
	if err != nil {
		t.Fatalf("NewTestClock: %v", err)
	}
	books, err := application.NewBookReconstruct(
		clk, &spyBookPublisher{}, &fixedBookBudgets{b: bookBudget, on: venue}, &spyResnapshotter{}, holdDepth)
	if err != nil {
		t.Fatalf("NewBookReconstruct: %v", err)
	}
	funding, err := application.NewFundingIngest(clk, &spyPublisher{}, &fixedBudgets{b: budget, on: venue})
	if err != nil {
		t.Fatalf("NewFundingIngest: %v", err)
	}

	if _, err := books.OnSnapshot(bookAt(t, 1000, base, base.Add(venueSkew)), anyOrdinal); err != nil {
		t.Fatalf("OnSnapshot: %v", err)
	}
	if _, err := funding.Ingest(obs(t, base, base.Add(venueSkew), 1), anyOrdinal); err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	// Two armed timers on ONE instrument. A shared ID leaves one.
	if got := clk.Pending(); got != 2 {
		t.Errorf("pending timers = %d, want 2 — book and funding must not share a timer ID", got)
	}
}
