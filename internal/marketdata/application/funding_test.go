package application_test

import (
	"errors"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/marketdata/application"
	"github.com/giiorg/arb-lab/internal/marketdata/domain"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

const venue = ids.Venue("binance")

// base is engine time's origin for these tests. Not the zero instant: every
// Watermark field refuses that, so a bug that zeroed one would fail for the
// wrong reason.
var base = time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)

// venueSkew is deliberately NON-ZERO, and it is the fixture parameter that
// keeps the two freshness verdicts from being the same question.
//
// Transport freshness is measured against Clock.Now and data freshness against
// Clock.ServerNow(venue). With a zero offset those are the same instant, so
// every test would pass identically against an implementation that measured
// both against engine time — which is the §3.5 mistake the code's comment
// claims to avoid. A skew makes the claim falsifiable.
const venueSkew = 90 * time.Second

// budget's two bounds are far apart and neither is a multiple of the other, so
// an age landing in the stale band cannot also be read as fresh or blind. The
// gap between them is what makes the three-rung ladder observable at all.
var budget = domain.Budget{MaxAge: 30 * time.Second, Blind: 5 * time.Minute}

// anyOrdinal is the AcceptOrdinal for calls that are not about the ordinal.
//
// It is 77 rather than 1, and that is the fixture-collapse check applied to a
// value that would otherwise get one by default. Every VenueSeq in this file is
// 0, 1, 2, 4, 5, 6, 8 or 9999, so a snapshot stamping AcceptOrdinal from the
// watermark's sequence cannot agree with this by accident; and 1 would let a
// hardcoded 1 pass everywhere. The tests that ARE about the ordinal use their
// own distinctive value and do not read this.
const anyOrdinal = application.AcceptOrdinal(77)

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

// obs builds an observation whose two timestamps are set INDEPENDENTLY.
//
// recvAt and sourceAt are separate parameters for the same reason venueSkew is
// non-zero: a fixture that always sets them equal cannot express the venue-side
// stall — transport healthy, SourceTime frozen — which is the failure
// domain.Watermark's doc comment says is invisible to the check written first.
func obs(t *testing.T, recvAt, sourceAt time.Time, seq uint64) domain.FundingObservation {
	t.Helper()
	return domain.FundingObservation{
		Instrument: perp(),
		Rate:       rate(t, "0.0001"),
		Interval:   8 * time.Hour,
		NextSettle: base.Add(4 * time.Hour),
		Cap:        rate(t, "0.0075"),
		Floor:      rate(t, "-0.0075"),
		Watermark: domain.Watermark{
			VenueSeq:   seq,
			SourceTime: sourceAt,
			RecvTime:   recvAt,
		},
	}
}

// spyPublisher records what was published and can be told to fail.
type spyPublisher struct {
	got  []application.FundingSnapshot
	fail error
}

func (s *spyPublisher) Publish(snap application.FundingSnapshot) error {
	s.got = append(s.got, snap)
	return s.fail
}

// fixedBudgets answers for one venue, so "no budget configured" is reachable by
// asking about any other.
//
// It is a POINTER stub with a gone flag, because §5.2 makes budgets
// hot-reloadable and a venue can therefore stop being configured between an
// ingest and the silence timer it armed. That window is the only way to reach
// OnSilence's error paths, and those are where the timer bookkeeping either
// cleans up or leaks.
type fixedBudgets struct {
	b    domain.Budget
	on   ids.Venue
	gone bool
}

func (f *fixedBudgets) FundingBudget(v ids.Venue) (domain.Budget, bool) {
	if f.gone || v != f.on {
		return domain.Budget{}, false
	}
	return f.b, true
}

type harness struct {
	svc *application.FundingIngest
	clk *clock.TestClock
	pub *spyPublisher
	bud *fixedBudgets
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
	bud := &fixedBudgets{b: budget, on: venue}
	svc, err := application.NewFundingIngest(clk, pub, bud)
	if err != nil {
		t.Fatalf("NewFundingIngest: %v", err)
	}
	return &harness{svc: svc, clk: clk, pub: pub, bud: bud}
}

// ---------------------------------------------------------------------------
// Construction
// ---------------------------------------------------------------------------

func TestNewFundingIngestRefusesMissingDependencies(t *testing.T) {
	clk, err := clock.NewTestClock(base, nil)
	if err != nil {
		t.Fatalf("NewTestClock: %v", err)
	}
	pub := &spyPublisher{}
	bud := &fixedBudgets{b: budget, on: venue}

	// Each case omits exactly ONE dependency, so a constructor checking only
	// the first would be caught by the other two.
	cases := []struct {
		name string
		c    clock.Clock
		p    application.FundingPublisher
		b    application.FundingBudgets
		want error
	}{
		{"no clock", nil, pub, bud, application.ErrClockNil},
		{"no publisher", clk, nil, bud, application.ErrPublisherNil},
		{"no budgets", clk, pub, nil, application.ErrBudgetsNil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, err := application.NewFundingIngest(tc.c, tc.p, tc.b)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
			if svc != nil {
				t.Fatalf("got a service back alongside the refusal")
			}
		})
	}

	// The positive control. Without it, a constructor that refused everything
	// would score three out of three above.
	if _, err := application.NewFundingIngest(clk, pub, bud); err != nil {
		t.Fatalf("fully wired construction: %v", err)
	}
}

// ---------------------------------------------------------------------------
// The two freshness verdicts
// ---------------------------------------------------------------------------

// TestFreshnessIsTwoIndEPENDENTQuestions is the venue-side-stall case: updates
// keep arriving on schedule while the venue's own view is frozen.
//
// It is the test that separates Transport from Data. Under an implementation
// measuring both against engine time — or both against server time — the two
// verdicts agree here and the stall is invisible, which is exactly what
// domain.Watermark says goes wrong.
func TestFreshnessIsTwoIndependentQuestions(t *testing.T) {
	h := newHarness(t)

	// Delivered just now, but carrying a venue stamp from ten minutes ago.
	// Ten minutes is past Blind, one nanosecond of transport age is not.
	stale := base.Add(venueSkew).Add(-10 * time.Minute)
	snap, err := h.svc.Ingest(obs(t, base, stale, 1), anyOrdinal)
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	if snap.Transport != domain.FreshnessFresh {
		t.Errorf("Transport = %v, want %v — the transport is healthy", snap.Transport, domain.FreshnessFresh)
	}
	if snap.Data != domain.FreshnessBlind {
		t.Errorf("Data = %v, want %v — the venue's view is ten minutes old", snap.Data, domain.FreshnessBlind)
	}
	if got := snap.Least(); got != domain.FreshnessBlind {
		t.Errorf("Least() = %v, want %v — the worse of the two", got, domain.FreshnessBlind)
	}
}

// The mirror: the venue's view is current but nothing has arrived in a while.
// Present because the previous test alone would pass against an implementation
// that had the two fields swapped.
func TestTransportStallIsSeenWhenDataIsCurrent(t *testing.T) {
	h := newHarness(t)

	// Arrived 45s ago — past MaxAge (30s), inside Blind (5m) — while the venue
	// stamped it at what is, in server time, effectively now.
	recv := base.Add(-45 * time.Second)
	snap, err := h.svc.Ingest(obs(t, recv, base.Add(venueSkew), 1), anyOrdinal)
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	if snap.Transport != domain.FreshnessStale {
		t.Errorf("Transport = %v, want %v", snap.Transport, domain.FreshnessStale)
	}
	if snap.Data != domain.FreshnessFresh {
		t.Errorf("Data = %v, want %v", snap.Data, domain.FreshnessFresh)
	}
	if got := snap.Least(); got != domain.FreshnessStale {
		t.Errorf("Least() = %v, want %v", got, domain.FreshnessStale)
	}
}

func TestClockInversionIsRefusedNotPublished(t *testing.T) {
	h := newHarness(t)

	// Received in the future relative to engine time: two clocks that disagree.
	_, err := h.svc.Ingest(obs(t, base.Add(time.Minute), base.Add(venueSkew), 1), anyOrdinal)
	if !errors.Is(err, domain.ErrClockInversion) {
		t.Fatalf("err = %v, want %v", err, domain.ErrClockInversion)
	}
	if len(h.pub.got) != 0 {
		t.Fatalf("published %d snapshots on a broken clock, want 0", len(h.pub.got))
	}
}

// ---------------------------------------------------------------------------
// Causal traceability — §3.5's AcceptOrdinal
// ---------------------------------------------------------------------------

// The ordinal of the input that triggered a snapshot travels WITH the snapshot,
// which is what lets a publisher stamp it and a replay tie a dispatched envelope
// back to the raw input that caused it.
//
// The two fixture values are deliberately unequal and neither is 1: an ordinal
// of 4242 against a VenueSeq of 3 is what separates "carried the trigger" from
// the two implementations that would otherwise look identical here — stamping
// from the watermark's sequence, and stamping a constant.
func TestSnapshotCarriesTheTriggeringOrdinal(t *testing.T) {
	h := newHarness(t)

	const trigger = application.AcceptOrdinal(4242)
	snap, err := h.svc.Ingest(obs(t, base, base.Add(venueSkew), 3), trigger)
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	if snap.Trigger != trigger {
		t.Errorf("Trigger = %d, want %d — a published snapshot no replay can tie to its cause", snap.Trigger, trigger)
	}
	// And it must reach the publisher, which is the only consumer that can act
	// on it. A returned-but-not-published trigger would still lose the trace.
	if len(h.pub.got) != 1 {
		t.Fatalf("published %d, want 1", len(h.pub.got))
	}
	if got := h.pub.got[0].Trigger; got != trigger {
		t.Errorf("published Trigger = %d, want %d", got, trigger)
	}
}

// A silence-timer snapshot carries ZERO, and this is the test that fixes the
// meaning of that value.
//
// §3.5's inheritance rule covers an envelope derived from a raw input. This one
// is derived from the ABSENCE of an input, so there is nothing to inherit. The
// preceding Ingest deliberately supplies a distinctive ordinal, because the
// implementation this test exists to refuse is the tempting one: re-using the
// held observation's trigger, which would make the capture log claim one raw
// input produced two dispatched envelopes — a replay reconciling them reports a
// phantom duplicate and concludes the feed is healthy, which is the exact
// opposite of what this timer fired to say.
func TestSilenceCarriesNoTriggeringOrdinal(t *testing.T) {
	h := newHarness(t)

	if _, err := h.svc.Ingest(obs(t, base, base.Add(venueSkew), 3), 4242); err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	fired := advanceAndFire(t, h)
	snap, err := h.svc.OnSilence(fired.ID)
	if err != nil {
		t.Fatalf("OnSilence: %v", err)
	}
	if snap.Trigger != 0 {
		t.Errorf("Trigger = %d, want 0 — a timer-triggered snapshot has no raw input to inherit from", snap.Trigger)
	}
}

// Ingest refuses a zero ordinal, and that refusal is what makes zero READABLE
// as "timer-triggered" rather than as "somebody forgot".
//
// Without it the two are the same value with two meanings, and the reader who
// cannot tell them apart is a replay reconciling dispatched envelopes against
// raw inputs.
func TestIngestRefusesAnUnstampedOrdinal(t *testing.T) {
	h := newHarness(t)

	if _, err := h.svc.Ingest(obs(t, base, base.Add(venueSkew), 3), 0); !errors.Is(err, application.ErrNoTrigger) {
		t.Fatalf("err = %v, want %v", err, application.ErrNoTrigger)
	}
	if len(h.pub.got) != 0 {
		t.Errorf("published %d snapshots from an untraceable ingest, want 0", len(h.pub.got))
	}
	if n := h.clk.Pending(); n != 0 {
		t.Errorf("Pending() = %d, want 0 — a refused ingest must not arm a timer", n)
	}

	// The positive control. Without it a service refusing every ingest would
	// score a pass above, and the two assertions after it would pass twice over.
	if _, err := h.svc.Ingest(obs(t, base, base.Add(venueSkew), 3), 1); err != nil {
		t.Fatalf("the same observation with an ordinal: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Supersede and sequence gaps
// ---------------------------------------------------------------------------

// The seq-ordered path. Every boundary is its own case, because the predicate
// is `next <= prev` and a suite testing only `next < prev` cannot tell it from
// `next < prev` — the duplicate is the whole difference.
func TestSequenceOrdering(t *testing.T) {
	cases := []struct {
		name     string
		prev     uint64
		next     uint64
		accepted bool
		gap      bool
	}{
		{"consecutive advances with no gap", 5, 6, true, false},
		{"a skip advances AND reports a gap", 5, 8, true, true},
		{"the duplicate is refused", 5, 5, false, false},
		{"a reorder is refused", 5, 4, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			if _, err := h.svc.Ingest(obs(t, base, base.Add(venueSkew), tc.prev), anyOrdinal); err != nil {
				t.Fatalf("seeding Ingest: %v", err)
			}

			snap, err := h.svc.Ingest(obs(t, base, base.Add(venueSkew), tc.next), anyOrdinal)
			if !tc.accepted {
				if !errors.Is(err, application.ErrSuperseded) {
					t.Fatalf("err = %v, want %v", err, application.ErrSuperseded)
				}
				if len(h.pub.got) != 1 {
					t.Fatalf("published %d, want 1 — the refused one must not publish", len(h.pub.got))
				}
				return
			}
			if err != nil {
				t.Fatalf("Ingest: %v", err)
			}
			if snap.GapDetected != tc.gap {
				t.Errorf("GapDetected = %v, want %v", snap.GapDetected, tc.gap)
			}
			if !snap.SeqAvailable {
				t.Error("SeqAvailable = false on a sequenced source")
			}
		})
	}
}

// The arrival-ordered path, used when the venue supplies no sequence.
//
// The EQUAL case is the one that matters: `!next.Before(prev)` and
// `next.After(prev)` differ at exactly that instant and nowhere else, so a
// suite without it cannot tell the implemented rule from the one that silently
// drops a second poll landing in the same clock tick.
func TestArrivalOrderingWhenNoSequence(t *testing.T) {
	cases := []struct {
		name     string
		offset   time.Duration
		accepted bool
	}{
		{"a later arrival advances", time.Second, true},
		{"an equal arrival advances", 0, true},
		{"an earlier arrival is refused", -time.Second, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			first := base.Add(-10 * time.Second)
			if _, err := h.svc.Ingest(obs(t, first, base.Add(venueSkew), 0), anyOrdinal); err != nil {
				t.Fatalf("seeding Ingest: %v", err)
			}

			snap, err := h.svc.Ingest(obs(t, first.Add(tc.offset), base.Add(venueSkew), 0), anyOrdinal)
			if !tc.accepted {
				if !errors.Is(err, application.ErrSuperseded) {
					t.Fatalf("err = %v, want %v", err, application.ErrSuperseded)
				}
				return
			}
			if err != nil {
				t.Fatalf("Ingest: %v", err)
			}
			if snap.SeqAvailable {
				t.Error("SeqAvailable = true on a source that supplied no sequence")
			}
			if snap.GapDetected {
				t.Error("GapDetected = true where no sequence exists to have a gap in")
			}
		})
	}
}

// The first observation for an instrument has no predecessor, so it cannot be
// superseded and cannot report a gap. Present because every other supersede
// test seeds first, which would hide a nil-map or a first-write bug.
func TestFirstObservationIsAlwaysAccepted(t *testing.T) {
	h := newHarness(t)
	snap, err := h.svc.Ingest(obs(t, base, base.Add(venueSkew), 9999), anyOrdinal)
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if snap.GapDetected {
		t.Error("GapDetected = true with no predecessor to have skipped from")
	}
}

// ---------------------------------------------------------------------------
// Budgets
// ---------------------------------------------------------------------------

// Ingest validates the observation before anything else, and an invalid one
// never reaches the publisher.
//
// Absent, and a mutation removing the check survived: every other fixture is a
// valid observation, so "validated" and "not validated" produce identical runs.
// The spot case is the one that matters most — domain refuses it because a spot
// leg reading "funding 0" is indistinguishable from a perp in a flat regime,
// and the carry computation nets the two legs, so the wrong answer would be
// absorbed into the number milestone 2's kill gate reads.
func TestInvalidObservationsAreRefusedBeforePublishing(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*domain.FundingObservation)
		want error
	}{
		{
			name: "a spot instrument has no funding",
			mut: func(o *domain.FundingObservation) {
				o.Instrument.Product = ids.ProductSpot
				o.Instrument.Contract = ""
			},
			want: domain.ErrFundingOnSpot,
		},
		{
			name: "an inverted clamp band",
			mut:  func(o *domain.FundingObservation) { o.Cap, o.Floor = o.Floor, o.Cap },
			want: domain.ErrClampInverted,
		},
		{
			name: "a non-positive funding interval",
			mut:  func(o *domain.FundingObservation) { o.Interval = 0 },
			want: domain.ErrIntervalInvalid,
		},
		{
			name: "an unpopulated receive stamp",
			mut:  func(o *domain.FundingObservation) { o.Watermark.RecvTime = time.Time{} },
			want: domain.ErrWatermarkUnset,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			o := obs(t, base, base.Add(venueSkew), 1)
			tc.mut(&o)

			if _, err := h.svc.Ingest(o, anyOrdinal); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
			if len(h.pub.got) != 0 {
				t.Errorf("published %d snapshots from an invalid observation, want 0", len(h.pub.got))
			}
			if n := h.clk.Pending(); n != 0 {
				t.Errorf("Pending() = %d, want 0 — an invalid observation must not arm a timer", n)
			}
		})
	}
}

func TestUnconfiguredVenueIsRefused(t *testing.T) {
	h := newHarness(t)
	other := obs(t, base, base.Add(venueSkew), 1)
	other.Instrument.Venue = ids.Venue("bybit")

	if _, err := h.svc.Ingest(other, anyOrdinal); !errors.Is(err, application.ErrNoBudget) {
		t.Fatalf("err = %v, want %v", err, application.ErrNoBudget)
	}
}

func TestUnreachableStaleRungIsRefused(t *testing.T) {
	clk, err := clock.NewTestClock(base, nil)
	if err != nil {
		t.Fatalf("NewTestClock: %v", err)
	}
	// MaxAge == Blind: the ladder silently has two rungs where it is configured
	// to have three. domain.Budget.Validate refuses it; this asserts the
	// application actually asks.
	bad := domain.Budget{MaxAge: time.Minute, Blind: time.Minute}
	svc, err := application.NewFundingIngest(clk, &spyPublisher{}, &fixedBudgets{b: bad, on: venue})
	if err != nil {
		t.Fatalf("NewFundingIngest: %v", err)
	}

	if _, err := svc.Ingest(obs(t, base, base, 1), anyOrdinal); !errors.Is(err, domain.ErrBudgetUnreachable) {
		t.Fatalf("err = %v, want %v", err, domain.ErrBudgetUnreachable)
	}
}

// ---------------------------------------------------------------------------
// The silence timer — §3.4's "detected by a timer, not only by the next update"
// ---------------------------------------------------------------------------

func TestIngestArmsTheSilenceTimer(t *testing.T) {
	h := newHarness(t)
	if _, err := h.svc.Ingest(obs(t, base, base.Add(venueSkew), 1), anyOrdinal); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if n := h.clk.Pending(); n != 1 {
		t.Fatalf("Pending() = %d, want 1 — nothing would notice this feed dying", n)
	}

	// It must fire at MaxAge past RECEIPT, not at MaxAge past now. The two are
	// the same only when the observation arrived at this instant, which is why
	// the next test re-checks with an aged one.
	// The priority is a decision, so it is pinned. §3.5 makes lower MORE urgent
	// and the zero value therefore the most urgent of all; a stale-mark check
	// outranking the safety kernel's own timers in a drain would be an omission
	// rather than a choice, and nothing else in the suite would notice.
	//
	// Asserted as non-zero rather than as the literal, because non-zero is the
	// claim the code argues for and the exact rung is free to be re-tuned.
	pending, ok := h.clk.NextDue()
	if ok {
		t.Fatalf("a timer was already due at ingest time: %+v", pending)
	}

	at, ok := h.clk.AdvanceToNextTimer()
	if !ok {
		t.Fatal("AdvanceToNextTimer: no timer")
	}
	if fired, ok := h.clk.NextDue(); !ok {
		t.Fatal("timer did not become due after advancing to it")
	} else if fired.Prio == 0 {
		t.Error("Prio = 0, the most urgent value there is — marketdata must not outrank the safety kernel by default")
	}
	// One tick PAST MaxAge, not at it. Budget.At is inclusive-fresh, so firing
	// AT the bound re-derives FreshnessFresh and re-arms for the same instant —
	// a spin with the ladder frozen. The domain owns that arithmetic; this
	// asserts the application did not re-derive its own copy.
	if want := base.Add(budget.MaxAge + time.Nanosecond); !at.Equal(want) {
		t.Errorf("fired at %s, want %s", at, want)
	}
}

// Re-arming must REPLACE, not accumulate. §3.5 gives SetTimer replace-by-id
// semantics and the ID is derived from the instrument for exactly this reason;
// a per-call ID would leak one timer per update, forever, on a healthy feed.
func TestReArmingReplacesRatherThanAccumulates(t *testing.T) {
	h := newHarness(t)
	for i := uint64(1); i <= 5; i++ {
		at := base.Add(time.Duration(i) * time.Second)
		if err := h.clk.SetNow(at); err != nil {
			t.Fatalf("SetNow: %v", err)
		}
		if _, err := h.svc.Ingest(obs(t, at, at.Add(venueSkew), i), anyOrdinal); err != nil {
			t.Fatalf("Ingest %d: %v", i, err)
		}
	}
	if n := h.clk.Pending(); n != 1 {
		t.Fatalf("Pending() = %d after 5 updates, want 1", n)
	}
}

// The timer fires, nothing has arrived, and the verdict degrades. This is the
// behaviour §3.4 requires and the one an update-triggered check never produces.
func TestSilenceDegradesTheVerdictWithoutANewObservation(t *testing.T) {
	h := newHarness(t)
	if _, err := h.svc.Ingest(obs(t, base, base.Add(venueSkew), 1), anyOrdinal); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if got := h.pub.got[0].Transport; got != domain.FreshnessFresh {
		t.Fatalf("first publish Transport = %v, want %v", got, domain.FreshnessFresh)
	}

	fired := advanceAndFire(t, h)
	snap, err := h.svc.OnSilence(fired.ID)
	if err != nil {
		t.Fatalf("OnSilence: %v", err)
	}
	if snap.Transport != domain.FreshnessStale {
		t.Errorf("Transport = %v, want %v", snap.Transport, domain.FreshnessStale)
	}
	// The SAME observation, re-judged — not a fabricated reading.
	if snap.Observation.Watermark.RecvTime != base {
		t.Errorf("RecvTime = %s, want the original %s", snap.Observation.Watermark.RecvTime, base)
	}
	if len(h.pub.got) != 2 {
		t.Fatalf("published %d, want 2 — silence must publish the new verdict", len(h.pub.got))
	}
}

// The ladder walks fresh → stale → blind and then STOPS arming. A dead feed
// that re-armed forever would grow the timer queue without bound, which is the
// leak clock.TimerSource.Pending exists to make visible.
func TestSilenceLadderTerminatesAtBlind(t *testing.T) {
	h := newHarness(t)
	if _, err := h.svc.Ingest(obs(t, base, base.Add(venueSkew), 1), anyOrdinal); err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	fired := advanceAndFire(t, h)
	if _, err := h.svc.OnSilence(fired.ID); err != nil {
		t.Fatalf("OnSilence (stale rung): %v", err)
	}
	if n := h.clk.Pending(); n != 1 {
		t.Fatalf("Pending() = %d after the stale rung, want 1 — the blind rung must be armed", n)
	}

	fired = advanceAndFire(t, h)
	snap, err := h.svc.OnSilence(fired.ID)
	if err != nil {
		t.Fatalf("OnSilence (blind rung): %v", err)
	}
	if snap.Transport != domain.FreshnessBlind {
		t.Errorf("Transport = %v, want %v", snap.Transport, domain.FreshnessBlind)
	}
	if n := h.clk.Pending(); n != 0 {
		t.Errorf("Pending() = %d once blind, want 0 — there is nowhere further to age", n)
	}
}

// An observation that arrives ALREADY blind cancels the timer armed by its
// predecessor, and stops the ID being claimable.
//
// This is the only path that reaches arm()'s terminal branch with a timer still
// pending, and its absence let two mutations live. Every other route there goes
// through OnSilence, where the timer has already fired and been consumed — so
// cancelling it is a no-op and deleting it was already done at the top of
// OnSilence. Both cleanups looked covered and neither was: the fixture never
// put the code in the state where they do anything.
func TestAnAlreadyBlindObservationCancelsThePendingTimer(t *testing.T) {
	h := newHarness(t)
	if _, err := h.svc.Ingest(obs(t, base, base.Add(venueSkew), 1), anyOrdinal); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if n := h.clk.Pending(); n != 1 {
		t.Fatalf("Pending() = %d after the first observation, want 1", n)
	}

	// Ten minutes later the feed delivers a reading it took a moment after the
	// first — a venue coming back from an outage with something already blind.
	later := base.Add(10 * time.Minute)
	if err := h.clk.SetNow(later); err != nil {
		t.Fatalf("SetNow: %v", err)
	}
	snap, err := h.svc.Ingest(obs(t, base.Add(time.Second), base.Add(venueSkew+time.Second), 2), anyOrdinal)
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if snap.Transport != domain.FreshnessBlind {
		t.Fatalf("Transport = %v, want %v — the fixture does not reach the terminal branch", snap.Transport, domain.FreshnessBlind)
	}

	if n := h.clk.Pending(); n != 0 {
		t.Errorf("Pending() = %d, want 0 — a blind observation has nowhere further to age, so its timer is a wakeup that changes nothing", n)
	}
	// And the ID must not still route: a timer that was cancelled but left in
	// the map would be claimable by any later delivery of the same ID.
	id := clock.TimerID("marketdata.funding.silence." + perp().String())
	if _, err := h.svc.OnSilence(id); !errors.Is(err, application.ErrUnknownTimer) {
		t.Errorf("OnSilence after cancel err = %v, want %v", err, application.ErrUnknownTimer)
	}
}

// OnSilence drops the fired timer BEFORE anything that can fail, so a
// re-assessment that errors does not leave a dead ID mapped.
//
// Reached by removing the venue's budget between the arm and the fire, which
// §5.2 makes possible — budgets are hot-reloadable. Without the early delete
// the second claim returns the same budget error rather than ErrUnknownTimer,
// which is why the two errors have to be distinguished here and not merely
// counted.
func TestAFailedReassessmentStillConsumesTheTimer(t *testing.T) {
	h := newHarness(t)
	if _, err := h.svc.Ingest(obs(t, base, base.Add(venueSkew), 1), anyOrdinal); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	fired := advanceAndFire(t, h)

	h.bud.gone = true
	if _, err := h.svc.OnSilence(fired.ID); !errors.Is(err, application.ErrNoBudget) {
		t.Fatalf("first claim err = %v, want %v", err, application.ErrNoBudget)
	}
	if _, err := h.svc.OnSilence(fired.ID); !errors.Is(err, application.ErrUnknownTimer) {
		t.Fatalf("second claim err = %v, want %v — the fired timer was not consumed", err, application.ErrUnknownTimer)
	}
}

func TestUnknownTimerIsRefused(t *testing.T) {
	h := newHarness(t)
	if _, err := h.svc.OnSilence(clock.TimerID("someone.elses.timer")); !errors.Is(err, application.ErrUnknownTimer) {
		t.Fatalf("err = %v, want %v", err, application.ErrUnknownTimer)
	}
}

// Once the ladder terminates, the ID stops being claimable. A dispatcher that
// re-delivered it would otherwise get an unbounded run of degradations for one
// silence.
//
// The intermediate rung is deliberately NOT asserted unclaimable, and getting
// that wrong is what this test was originally written as: the ID is derived
// from the instrument, so re-arming for the blind rung re-registers the same
// string, and claiming it again is correct while a rung remains. What must
// stop is claiming it after there is nowhere further to age.
func TestATerminalTimerIsNotClaimableAgain(t *testing.T) {
	h := newHarness(t)
	if _, err := h.svc.Ingest(obs(t, base, base.Add(venueSkew), 1), anyOrdinal); err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	stale := advanceAndFire(t, h)
	if _, err := h.svc.OnSilence(stale.ID); err != nil {
		t.Fatalf("OnSilence (stale rung): %v", err)
	}

	blind := advanceAndFire(t, h)
	if blind.ID != stale.ID {
		t.Fatalf("rung IDs differ (%q then %q) — replace-by-id is what stops timers accumulating", stale.ID, blind.ID)
	}
	if _, err := h.svc.OnSilence(blind.ID); err != nil {
		t.Fatalf("OnSilence (blind rung): %v", err)
	}

	if _, err := h.svc.OnSilence(blind.ID); !errors.Is(err, application.ErrUnknownTimer) {
		t.Fatalf("claim past the terminal rung err = %v, want %v", err, application.ErrUnknownTimer)
	}
}

// ---------------------------------------------------------------------------
// Publisher failure
// ---------------------------------------------------------------------------

// A publish failure is reported AND the observation stays held. Rolling back
// would make a publisher outage silently re-open the door to a replayed
// delivery, which the code comments claim and this measures.
func TestPublishFailureIsReportedButTheObservationStillCounts(t *testing.T) {
	h := newHarness(t)
	boom := errors.New("lane closed")
	h.pub.fail = boom

	if _, err := h.svc.Ingest(obs(t, base, base.Add(venueSkew), 5), anyOrdinal); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want %v", err, boom)
	}

	h.pub.fail = nil
	// Seq 5 again: if the failed publish had rolled back the held reading, this
	// duplicate would be accepted as a first observation.
	if _, err := h.svc.Ingest(obs(t, base, base.Add(venueSkew), 5), anyOrdinal); !errors.Is(err, application.ErrSuperseded) {
		t.Fatalf("err = %v, want %v", err, application.ErrSuperseded)
	}
}

// advanceAndFire moves the clock to the next timer and consumes it.
//
// It fails rather than blocks when no timer is due. TestClock's queue is
// pull-based so nothing can hang here, but the check is what turns a mutation
// that stops arming into a named failure instead of a nil-field panic three
// lines later.
func advanceAndFire(t *testing.T, h *harness) clock.Fired {
	t.Helper()
	if _, ok := h.clk.AdvanceToNextTimer(); !ok {
		t.Fatal("no timer armed")
	}
	fired, ok := h.clk.NextDue()
	if !ok {
		t.Fatal("timer did not become due after advancing to it")
	}
	return fired
}
