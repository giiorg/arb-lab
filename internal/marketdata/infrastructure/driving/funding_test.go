package driving_test

import (
	"errors"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/marketdata/application"
	"github.com/giiorg/arb-lab/internal/marketdata/domain"
	"github.com/giiorg/arb-lab/internal/marketdata/infrastructure/driving"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/events"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

const venue = ids.Venue("binance")

var base = time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)

// venueSkew is non-zero for the reason application's suite records: with a zero
// offset, Clock.Now and Clock.ServerNow(venue) are the same instant, so the two
// freshness verdicts become the same question and every §3.5 mistake this code
// claims to avoid would pass.
const venueSkew = 90 * time.Second

var budget = domain.Budget{MaxAge: 30 * time.Second, Blind: 5 * time.Minute}

// The four numbers on an inbound envelope are pairwise DISTINCT, and none is a
// small integer that another could be confused with. An adapter that copied
// AcceptOrdinal into VenueSeq, or IngressSeq into either, is only visible if
// they differ.
const (
	acceptOrdinal uint64 = 7
	ingressSeq    uint64 = 19
	venueSeq      uint64 = 4242
)

// The two timestamps differ, so "copies SourceTime" and "copies RecvTime" are
// not the same assignment. sourceAt is EARLIER, which is the ordinary case: a
// venue stamps a message before we receive it.
var (
	sourceAt = base.Add(-2 * time.Second)
	recvAt   = base.Add(-1 * time.Second)
)

func perp() ids.VenueInstrument {
	return ids.VenueInstrument{
		Venue:    venue,
		Product:  ids.ProductLinearPerp,
		Symbol:   "BTCUSDT",
		Contract: ids.ContractLinear,
	}
}

func rate(t *testing.T, s string) fixedpoint.Rate {
	t.Helper()
	r, err := fixedpoint.ParseRate(s)
	if err != nil {
		t.Fatalf("ParseRate(%q): %v", s, err)
	}
	return r
}

// update's three rates are pairwise distinct and the interval is not a round
// multiple of anything else in the fixture, so a field crossed with another
// field is visible rather than absorbed.
func update(t *testing.T) driving.FundingUpdate {
	t.Helper()
	return driving.FundingUpdate{
		Rate:       rate(t, "0.00013"),
		Interval:   8 * time.Hour,
		NextSettle: base.Add(4 * time.Hour),
		Cap:        rate(t, "0.0075"),
		Floor:      rate(t, "-0.0075"),
	}
}

func envelope(t *testing.T, stream events.StreamKind, payload any) events.Envelope {
	t.Helper()
	return events.Envelope{
		AcceptOrdinal: acceptOrdinal,
		IngressSeq:    ingressSeq,
		Source:        events.SourceID{Instrument: perp(), Stream: stream},
		SourceTime:    sourceAt,
		RecvTime:      recvAt,
		VenueSeq:      venueSeq,
		Prio:          events.PrioNormal,
		Payload:       payload,
	}
}

// spyPublisher accepts everything and remembers it, so a test can tell "the use
// case ran" from "the adapter returned something plausible".
type spyPublisher struct{ got []application.FundingSnapshot }

func (s *spyPublisher) Publish(snap application.FundingSnapshot) error {
	s.got = append(s.got, snap)
	return nil
}

type fixedBudgets struct{ b domain.Budget }

func (f fixedBudgets) FundingBudget(ids.Venue) (domain.Budget, bool) { return f.b, true }

type harness struct {
	dispatch *driving.FundingDispatch
	clk      *clock.TestClock
	pub      *spyPublisher
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	clk, err := clock.NewTestClock(base, []clock.Calibration{
		{Venue: venue, Offset: venueSkew, Uncertainty: time.Second},
	})
	if err != nil {
		t.Fatalf("NewTestClock: %v", err)
	}
	pub := &spyPublisher{}
	svc, err := application.NewFundingIngest(clk, pub, fixedBudgets{b: budget})
	if err != nil {
		t.Fatalf("NewFundingIngest: %v", err)
	}
	d, err := driving.NewFundingDispatch(svc)
	if err != nil {
		t.Fatalf("NewFundingDispatch: %v", err)
	}
	return &harness{dispatch: d, clk: clk, pub: pub}
}

// ---------------------------------------------------------------------------
// Construction
// ---------------------------------------------------------------------------

func TestNewFundingDispatchRefusesAMissingUseCase(t *testing.T) {
	if _, err := driving.NewFundingDispatch(nil); !errors.Is(err, driving.ErrIngestNil) {
		t.Fatalf("NewFundingDispatch(nil) = %v, want ErrIngestNil", err)
	}
}

// ---------------------------------------------------------------------------
// Translation: provenance from the envelope, venue data from the payload
// ---------------------------------------------------------------------------

func TestDeliverTakesProvenanceFromTheEnvelope(t *testing.T) {
	h := newHarness(t)

	snap, err := h.dispatch.Deliver(envelope(t, events.StreamFunding, update(t)))
	if err != nil {
		t.Fatalf("Deliver: %v", err)
	}

	want := domain.Watermark{VenueSeq: venueSeq, SourceTime: sourceAt, RecvTime: recvAt}
	if got := snap.Observation.Watermark; got != want {
		t.Errorf("Watermark = %+v, want the envelope's %+v", got, want)
	}
	// The instrument comes from the SOURCE, which is the only place it exists:
	// FundingUpdate carries none, so a payload cannot name an instrument other
	// than the mailbox it arrived on.
	if got := snap.Observation.Instrument; got != perp() {
		t.Errorf("Instrument = %+v, want the source's %+v", got, perp())
	}
}

func TestDeliverTakesTheVenueReadingFromThePayload(t *testing.T) {
	h := newHarness(t)
	u := update(t)

	snap, err := h.dispatch.Deliver(envelope(t, events.StreamFunding, u))
	if err != nil {
		t.Fatalf("Deliver: %v", err)
	}

	obs := snap.Observation
	sameRate(t, "Rate", obs.Rate, u.Rate)
	sameRate(t, "Cap", obs.Cap, u.Cap)
	sameRate(t, "Floor", obs.Floor, u.Floor)
	if obs.Interval != u.Interval {
		t.Errorf("Interval = %v, want %v", obs.Interval, u.Interval)
	}
	if !obs.NextSettle.Equal(u.NextSettle) {
		t.Errorf("NextSettle = %v, want %v", obs.NextSettle, u.NextSettle)
	}
}

// sameRate compares through Cmp rather than ==. A Rate carries a construction
// flag alongside its mantissa, so == is an assertion about representation where
// Cmp is one about value — and Cmp's error is what catches an unconstructed
// operand instead of silently reporting inequality.
func sameRate(t *testing.T, name string, got, want fixedpoint.Rate) {
	t.Helper()
	c, err := got.Cmp(want)
	if err != nil {
		t.Fatalf("%s: Cmp(%v, %v): %v", name, got, want, err)
	}
	if c != 0 {
		t.Errorf("%s = %v, want %v", name, got, want)
	}
}

func TestDeliverRunsTheUseCase(t *testing.T) {
	// Translating correctly and then not calling anything would satisfy both
	// tests above, since Deliver returns the snapshot it built. The publisher
	// is what proves the observation went through FundingIngest.
	h := newHarness(t)

	if _, err := h.dispatch.Deliver(envelope(t, events.StreamFunding, update(t))); err != nil {
		t.Fatalf("Deliver: %v", err)
	}

	if len(h.pub.got) != 1 {
		t.Fatalf("publisher saw %d snapshots, want 1", len(h.pub.got))
	}
	if got := h.pub.got[0].Transport; got != domain.FreshnessFresh {
		t.Errorf("Transport = %v, want FreshnessFresh", got)
	}
}

// ---------------------------------------------------------------------------
// What it refuses
// ---------------------------------------------------------------------------

func TestDeliverRefusesAForeignStream(t *testing.T) {
	// StreamMark is the case that matters: it is on the SAME market-data lane
	// as funding, so "is this funding?" and "is this on my lane?" are different
	// questions and only a fixture from this lane can tell them apart. A
	// control-lane stream is included so the guard is not accidentally a lane
	// check that happens to work.
	for _, stream := range []events.StreamKind{events.StreamMark, events.StreamBook, events.StreamExecutions} {
		t.Run(string(stream), func(t *testing.T) {
			h := newHarness(t)

			_, err := h.dispatch.Deliver(envelope(t, stream, update(t)))
			if !errors.Is(err, driving.ErrWrongStream) {
				t.Fatalf("Deliver(%s) = %v, want ErrWrongStream", stream, err)
			}
			if len(h.pub.got) != 0 {
				t.Error("a foreign stream reached the use case")
			}
		})
	}
}

func TestDeliverRefusesAPayloadItCannotRead(t *testing.T) {
	// A pointer to the right type is included deliberately. It is the shape a
	// producer avoiding a copy would reach for, and accepting it would give the
	// context two spellings of one payload — one of which shares memory with
	// its producer, on a lane whose whole design is that a queued envelope is
	// not mutated after it is published.
	//
	// A payload that is ABSENT is deliberately not in this table. It is
	// events.ErrPayloadNil, refused by the envelope check before the assertion
	// is reached, and the two belong to different owners: "this envelope is not
	// well-formed" is the transport's complaint, "this envelope is not mine" is
	// the context's. Folding them together would make a wiring bug in the
	// dispatcher read as a venue sending the wrong stream. It has its own case
	// below.
	u := update(t)
	cases := map[string]any{
		"a book payload":                struct{ Bids, Asks int }{1, 2},
		"an observation, not an update": domain.FundingObservation{},
		"a pointer to the right type":   &u,
	}
	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)

			_, err := h.dispatch.Deliver(envelope(t, events.StreamFunding, payload))
			if !errors.Is(err, driving.ErrPayloadType) {
				t.Fatalf("Deliver = %v, want ErrPayloadType", err)
			}
			if len(h.pub.got) != 0 {
				t.Error("an unreadable payload reached the use case")
			}
		})
	}
}

func TestDeliverRefusesAnEmptyEnvelopeAsMalformedNotAsForeign(t *testing.T) {
	h := newHarness(t)

	_, err := h.dispatch.Deliver(envelope(t, events.StreamFunding, nil))
	if !errors.Is(err, events.ErrPayloadNil) {
		t.Fatalf("Deliver(nil payload) = %v, want events.ErrPayloadNil", err)
	}
	if errors.Is(err, driving.ErrPayloadType) {
		t.Error("an envelope with no payload reads as a wrong-type payload; a dispatcher bug would look like a venue one")
	}
}

func TestDeliverRefusesAnUndispatchedEnvelope(t *testing.T) {
	// §3.5 makes IngressSeq the replay key, stamped by the dispatcher on the
	// envelope it consumes. An observation accepted without one is one a replay
	// cannot address — and it would not merely be missing from the replay: it
	// becomes the held reading that supersede detection compares the NEXT
	// observation against, so the gap silently changes what the replay accepts.
	//
	// AcceptOrdinal is checked for the same reason at the other end: it is the
	// capture lane's authority, and an observation with no raw arrival order
	// cannot be tied back to the input it came from.
	cases := map[string]func(*events.Envelope){
		"no ingress seq":    func(e *events.Envelope) { e.IngressSeq = 0 },
		"no accept ordinal": func(e *events.Envelope) { e.AcceptOrdinal = 0 },
		"no receive time":   func(e *events.Envelope) { e.RecvTime = time.Time{} },
	}
	for name, breakIt := range cases {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			env := envelope(t, events.StreamFunding, update(t))
			breakIt(&env)

			if _, err := h.dispatch.Deliver(env); err == nil {
				t.Fatal("Deliver accepted an envelope the dispatcher had not stamped")
			}
			if len(h.pub.got) != 0 {
				t.Error("an unstamped envelope reached the use case")
			}
		})
	}
}

func TestDeliverReportsTheUseCasesOwnRefusal(t *testing.T) {
	// A replayed delivery is the ordinary outcome of an at-least-once
	// transport, and the adapter must pass the diagnosis through rather than
	// flattening it into "bad envelope" — a caller counting supersedes needs to
	// tell them from feed faults.
	h := newHarness(t)
	env := envelope(t, events.StreamFunding, update(t))
	if _, err := h.dispatch.Deliver(env); err != nil {
		t.Fatalf("first Deliver: %v", err)
	}

	if _, err := h.dispatch.Deliver(env); !errors.Is(err, application.ErrSuperseded) {
		t.Fatalf("replayed Deliver = %v, want application.ErrSuperseded", err)
	}
}

// ---------------------------------------------------------------------------
// Timers
// ---------------------------------------------------------------------------

func TestTimerRoutesASilenceTimerBackToTheUseCase(t *testing.T) {
	h := newHarness(t)
	if _, err := h.dispatch.Deliver(envelope(t, events.StreamFunding, update(t))); err != nil {
		t.Fatalf("Deliver: %v", err)
	}

	// Drive the clock the way §3.5's dispatcher does: advance, pull what is
	// due, route it. Nothing here pushes.
	if _, ok := h.clk.AdvanceToNextTimer(); !ok {
		t.Fatal("no silence timer was armed by the ingest")
	}
	fired, ok := h.clk.NextDue()
	if !ok {
		t.Fatal("NextDue reported nothing due after advancing to a timer")
	}

	snap, err := h.dispatch.Timer(fired.ID)
	if err != nil {
		t.Fatalf("Timer(%q): %v", fired.ID, err)
	}
	// The re-assessment is the whole reason the timer exists: same observation,
	// worse verdict. Equal verdicts would mean the timer fired at an instant
	// where nothing had changed — the spin the domain's NextTransition records.
	if snap.Transport != domain.FreshnessStale {
		t.Errorf("Transport = %v, want FreshnessStale one tick past MaxAge", snap.Transport)
	}
	if len(h.pub.got) != 2 {
		t.Fatalf("publisher saw %d snapshots, want 2 (the ingest and the silence re-assessment)", len(h.pub.got))
	}
}

func TestTimerRefusesAnIDItDidNotArm(t *testing.T) {
	// The dispatcher pumps one timer queue for the whole process, so a context
	// that silently accepted a foreign ID would make a mis-routed timer look
	// like a healthy no-op — a safety timer swallowed by marketdata.
	h := newHarness(t)

	if _, err := h.dispatch.Timer(clock.TimerID("oms.recovery.sweep")); !errors.Is(err, application.ErrUnknownTimer) {
		t.Fatalf("Timer(foreign) = %v, want application.ErrUnknownTimer", err)
	}
}
