package application_test

import (
	"errors"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/marketdata/application"
	"github.com/giiorg/arb-lab/internal/marketdata/application/ports"
	"github.com/giiorg/arb-lab/internal/marketdata/domain"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// markBudget is DELIBERATELY not `budget`, the funding file's value.
//
// The two classes' whole reason to be separate is that their budgets are not
// interchangeable (§5.2, ports.MarkBudgets). A test suite that gave them the
// same numbers could not tell an implementation that reads MarkBudget from one
// that reads FundingBudget, which is the substitution this split exists to make
// impossible. Both bounds differ, and neither is a multiple of funding's.
var markBudget = domain.Budget{MaxAge: 4 * time.Second, Blind: 45 * time.Second}

// markOrdinal is distinct from anyOrdinal for the same reason that one is 77:
// a snapshot that carried the wrong class's ordinal would otherwise agree.
const markOrdinal = ports.AcceptOrdinal(31)

func markPrice(t *testing.T, s string) fixedpoint.Price {
	t.Helper()
	p, err := fixedpoint.ParsePrice("BTC", "USDT", s)
	if err != nil {
		t.Fatalf("ParsePrice(%q): %v", s, err)
	}
	return p
}

func markObs(t *testing.T, recvAt, sourceAt time.Time, seq uint64) domain.MarkObservation {
	t.Helper()
	return domain.MarkObservation{
		Instrument: perp(),
		Price:      markPrice(t, "64000.5"),
		Watermark: domain.Watermark{
			VenueSeq:   seq,
			SourceTime: sourceAt,
			RecvTime:   recvAt,
		},
	}
}

type spyMarkPublisher struct {
	got  []ports.MarkSnapshot
	fail error
}

func (s *spyMarkPublisher) Publish(snap ports.MarkSnapshot) error {
	s.got = append(s.got, snap)
	return s.fail
}

// fixedMarkBudgets mirrors fixedBudgets, including the `gone` flag: §5.2 makes
// budgets hot-reloadable, so a venue can stop being configured between an
// ingest and the silence timer it armed, and that window is the only route to
// OnSilence's error paths.
type fixedMarkBudgets struct {
	b    domain.Budget
	on   ids.Venue
	gone bool
}

func (f *fixedMarkBudgets) MarkBudget(v ids.Venue) (domain.Budget, bool) {
	if f.gone || v != f.on {
		return domain.Budget{}, false
	}
	return f.b, true
}

type markHarness struct {
	svc *application.MarkIngest
	clk *clock.TestClock
	pub *spyMarkPublisher
	bud *fixedMarkBudgets
}

func newMarkHarness(t *testing.T) *markHarness {
	t.Helper()
	clk, err := clock.NewTestClock(base, []clock.Calibration{
		{Venue: venue, Offset: venueSkew, Uncertainty: time.Second},
	})
	if err != nil {
		t.Fatalf("NewTestClock: %v", err)
	}
	pub := &spyMarkPublisher{}
	bud := &fixedMarkBudgets{b: markBudget, on: venue}
	svc, err := application.NewMarkIngest(clk, pub, bud)
	if err != nil {
		t.Fatalf("NewMarkIngest: %v", err)
	}
	return &markHarness{svc: svc, clk: clk, pub: pub, bud: bud}
}

// ---------------------------------------------------------------------------
// Construction
// ---------------------------------------------------------------------------

func TestNewMarkIngestRefusesMissingDependencies(t *testing.T) {
	t.Parallel()

	clk, err := clock.NewTestClock(base, nil)
	if err != nil {
		t.Fatalf("NewTestClock: %v", err)
	}
	pub := &spyMarkPublisher{}
	bud := &fixedMarkBudgets{b: markBudget, on: venue}

	cases := []struct {
		name string
		c    clock.Clock
		p    ports.MarkPublisher
		b    ports.MarkBudgets
		want error
	}{
		{"no clock", nil, pub, bud, application.ErrClockNil},
		{"no publisher", clk, nil, bud, application.ErrPublisherNil},
		{"no budgets", clk, pub, nil, application.ErrBudgetsNil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := application.NewMarkIngest(tc.c, tc.p, tc.b)
			if !errors.Is(err, tc.want) {
				t.Errorf("NewMarkIngest err = %v; want %v", err, tc.want)
			}
			if got != nil {
				t.Error("a refused construction returned a usable value")
			}
		})
	}

	// The positive control, without which every case above passes against a
	// constructor that refuses everything.
	if _, err := application.NewMarkIngest(clk, pub, bud); err != nil {
		t.Errorf("NewMarkIngest with every dependency: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Freshness: two clocks, two questions
// ---------------------------------------------------------------------------

// The mark path's copy of funding's two-clock claim, and it is not redundant
// coverage: the two use cases compute it through the same helper but read the
// verdicts into different structs, and a mark snapshot that filled Data from
// the transport verdict would leave every venue-side stall invisible to §7.1 —
// the one failure mode the mark's data age exists for.
func TestMarkFreshnessIsTwoIndependentQuestions(t *testing.T) {
	t.Parallel()
	h := newMarkHarness(t)

	// Arrived just now by engine time, so transport is fresh. The venue stamped
	// it venueSkew+beyond ago in ITS OWN base, so the data view is old. With a
	// zero skew these would be one question.
	sourceAt := base.Add(venueSkew).Add(-10 * time.Second)
	snap, err := h.svc.Ingest(markObs(t, base, sourceAt, 1), markOrdinal)
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if snap.Transport != domain.FreshnessFresh {
		t.Errorf("Transport = %v; want FreshnessFresh — it arrived at `now`", snap.Transport)
	}
	if snap.Data != domain.FreshnessStale {
		t.Errorf("Data = %v; want FreshnessStale — the venue's view is 10s old against a 4s budget",
			snap.Data)
	}
	if got := snap.Least(); got != domain.FreshnessStale {
		t.Errorf("Least() = %v; want FreshnessStale, the worse of the two", got)
	}
}

// The mark's budget is READ, not funding's. This is the test that fails if
// MarkIngest is handed a FundingBudgets-shaped answer, and it works because
// markBudget and budget disagree at the age chosen: 10s is BLIND against the
// mark's 45s? no — it is stale against mark (4s < 10s <= 45s) and FRESH against
// funding (10s <= 30s). One age, two different verdicts.
func TestMarkReadsItsOwnBudgetNotFundings(t *testing.T) {
	t.Parallel()
	h := newMarkHarness(t)

	// No skew in the data direction: SourceTime is set so the data age is zero,
	// isolating the transport verdict as the thing under test.
	if err := h.clk.Advance(10 * time.Second); err != nil {
		t.Fatalf("Advance: %v", err)
	}
	now := base.Add(10 * time.Second)
	snap, err := h.svc.Ingest(markObs(t, base, now.Add(venueSkew), 1), markOrdinal)
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if snap.Transport != domain.FreshnessStale {
		t.Errorf("Transport = %v; want FreshnessStale — 10s is past the mark's 4s max-age, "+
			"though it would be FRESH under funding's 30s", snap.Transport)
	}
}

// ---------------------------------------------------------------------------
// The silence timer, which is what §7.1 hangs off
// ---------------------------------------------------------------------------

// The load-bearing test of this slice: the mark's silence timer must not share
// an ID with the funding timer for the SAME instrument.
//
// clock.SetTimer replaces by ID across one process-wide queue, so a shared ID
// means the class that armed second silently cancels the first — and the
// survivor looks entirely healthy. Both ingests are driven here for one
// instrument, and both timers must be pending afterwards.
func TestMarkAndFundingTimersDoNotCollideOnOneInstrument(t *testing.T) {
	t.Parallel()

	clk, err := clock.NewTestClock(base, []clock.Calibration{
		{Venue: venue, Offset: venueSkew, Uncertainty: time.Second},
	})
	if err != nil {
		t.Fatalf("NewTestClock: %v", err)
	}
	fIngest, err := application.NewFundingIngest(clk, &spyPublisher{}, &fixedBudgets{b: budget, on: venue})
	if err != nil {
		t.Fatalf("NewFundingIngest: %v", err)
	}
	mIngest, err := application.NewMarkIngest(clk, &spyMarkPublisher{}, &fixedMarkBudgets{b: markBudget, on: venue})
	if err != nil {
		t.Fatalf("NewMarkIngest: %v", err)
	}

	if _, err := fIngest.Ingest(obs(t, base, base.Add(venueSkew), 1), anyOrdinal); err != nil {
		t.Fatalf("funding Ingest: %v", err)
	}
	if _, err := mIngest.Ingest(markObs(t, base, base.Add(venueSkew), 1), markOrdinal); err != nil {
		t.Fatalf("mark Ingest: %v", err)
	}

	// Two timers for ONE instrument, which is the whole claim. A shared ID
	// leaves one, and the survivor looks entirely healthy.
	if got := clk.Pending(); got != 2 {
		t.Fatalf("pending timers = %d; want 2 — one per data class. A shared timer ID "+
			"silently replaces the class that armed first, and the survivor looks healthy", got)
	}
}

// The timer degrades the verdict with no new observation — the mechanism §7.1
// means by "never silently freezing the trigger". Without it a dead mark feed
// leaves the position marked fresh for the whole outage.
func TestMarkSilenceDegradesTheVerdict(t *testing.T) {
	t.Parallel()
	h := newMarkHarness(t)

	if _, err := h.svc.Ingest(markObs(t, base, base.Add(venueSkew), 1), markOrdinal); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if n := h.clk.Pending(); n != 1 {
		t.Fatalf("Pending() = %d; want 1 — nothing would notice this feed dying", n)
	}

	// Past the mark's max-age, inside its blind budget.
	if err := h.clk.Advance(markBudget.MaxAge + time.Second); err != nil {
		t.Fatalf("Advance: %v", err)
	}
	fired, ok := h.clk.NextDue()
	if !ok {
		t.Fatal("no timer became due past the mark's max-age")
	}
	snap, err := h.svc.OnSilence(fired.ID)
	if err != nil {
		t.Fatalf("OnSilence: %v", err)
	}
	if snap.Transport != domain.FreshnessStale {
		t.Errorf("Transport after silence = %v; want FreshnessStale", snap.Transport)
	}
	// Positive: the reading itself is unchanged. The timer reports an ABSENCE;
	// it must never mint a number.
	if snap.Observation.Price != markObs(t, base, base.Add(venueSkew), 1).Price {
		t.Error("OnSilence changed the observed price — it reports an absence, not a reading")
	}
	if snap.Trigger != 0 {
		t.Errorf("Trigger = %d; want 0 — this reading is derived from no input", snap.Trigger)
	}
}

// The ladder reaches Blind and then stops arming. A feed that never returns
// must not schedule a wakeup per rung for ever.
func TestMarkSilenceLadderTerminatesAtBlind(t *testing.T) {
	t.Parallel()
	h := newMarkHarness(t)

	if _, err := h.svc.Ingest(markObs(t, base, base.Add(venueSkew), 1), markOrdinal); err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	// Stale rung.
	if err := h.clk.Advance(markBudget.MaxAge + time.Second); err != nil {
		t.Fatalf("Advance: %v", err)
	}
	stale, ok := h.clk.NextDue()
	if !ok {
		t.Fatal("no timer became due at the stale rung")
	}
	if _, err := h.svc.OnSilence(stale.ID); err != nil {
		t.Fatalf("OnSilence (stale): %v", err)
	}
	if n := h.clk.Pending(); n != 1 {
		t.Fatalf("Pending() = %d; want 1 — the ladder stops one rung early", n)
	}

	// Blind rung: this is the verdict §5.2's mark_blind_action fires on.
	if err := h.clk.Advance(markBudget.Blind); err != nil {
		t.Fatalf("Advance: %v", err)
	}
	blind, ok := h.clk.NextDue()
	if !ok {
		t.Fatal("no timer became due at the blind rung")
	}
	snap, err := h.svc.OnSilence(blind.ID)
	if err != nil {
		t.Fatalf("OnSilence (blind): %v", err)
	}
	if snap.Transport != domain.FreshnessBlind {
		t.Errorf("Transport = %v; want FreshnessBlind", snap.Transport)
	}
	if got := h.clk.Pending(); got != 0 {
		t.Errorf("pending timers = %d; want 0 — a blind reading has nowhere further to age", got)
	}
}

// ---------------------------------------------------------------------------
// Refusals
// ---------------------------------------------------------------------------

func TestMarkIngestRefusesAnUnstampedOrdinal(t *testing.T) {
	t.Parallel()
	h := newMarkHarness(t)

	if _, err := h.svc.Ingest(markObs(t, base, base.Add(venueSkew), 1), 0); !errors.Is(err, application.ErrNoTrigger) {
		t.Errorf("Ingest with a zero ordinal: err = %v; want ErrNoTrigger", err)
	}
	if len(h.pub.got) != 0 {
		t.Error("a refused ingest published")
	}
	// Positive control: the same reading with an ordinal is accepted, so the
	// refusal above is about the ordinal and not about the fixture.
	if _, err := h.svc.Ingest(markObs(t, base, base.Add(venueSkew), 1), markOrdinal); err != nil {
		t.Errorf("Ingest with an ordinal: %v", err)
	}
}

func TestMarkIngestRefusesAnUnconfiguredVenue(t *testing.T) {
	t.Parallel()
	h := newMarkHarness(t)
	h.bud.on = "somewhere-else"

	if _, err := h.svc.Ingest(markObs(t, base, base.Add(venueSkew), 1), markOrdinal); !errors.Is(err, application.ErrNoBudget) {
		t.Errorf("err = %v; want ErrNoBudget", err)
	}
}

// A reading that arrived before the one held is refused, and — the half worth
// asserting separately — the HELD reading is the newer one afterwards.
func TestMarkSupersededReadingIsRefused(t *testing.T) {
	t.Parallel()
	h := newMarkHarness(t)

	// Both arrive at `now`, so the RecvTime fallback cannot decide this and the
	// VENUE SEQUENCE is what refuses it — which is the rule under test. A newer
	// RecvTime here would also trip the clock-inversion guard, which passes for
	// the wrong reason.
	newer := markObs(t, base, base.Add(venueSkew), 5)
	if _, err := h.svc.Ingest(newer, markOrdinal); err != nil {
		t.Fatalf("Ingest (newer): %v", err)
	}
	older := markObs(t, base, base.Add(venueSkew), 4)
	if _, err := h.svc.Ingest(older, markOrdinal); !errors.Is(err, application.ErrSuperseded) {
		t.Errorf("err = %v; want ErrSuperseded", err)
	}
	if len(h.pub.got) != 1 {
		t.Errorf("published %d snapshots; want 1 — the superseded one must not publish", len(h.pub.got))
	}
}

// An invalid observation is refused BEFORE it can become the held baseline, so
// one corrupt reading cannot poison supersede detection afterwards.
func TestMarkInvalidObservationsAreRefusedBeforePublishing(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		mut  func(*domain.MarkObservation)
		want error
	}{
		{
			// The Contract is cleared as well as the Product, and that is the
			// fixture-collapse check rather than tidiness: a spot instrument
			// CARRYING a contract type is refused by ids.VenueInstrument.Validate
			// one line earlier, so the obvious one-field mutation never reaches
			// ErrMarkOnSpot at all — it asserts a rule this file does not own,
			// while reading as though it covered the rule this file adds.
			"spot instrument",
			func(o *domain.MarkObservation) {
				o.Instrument.Product = ids.ProductSpot
				o.Instrument.Contract = ""
			},
			domain.ErrMarkOnSpot,
		},
		{
			"zero mark",
			func(o *domain.MarkObservation) { o.Price = mustPrice(t, "0") },
			domain.ErrMarkNotPositive,
		},
		{
			"negative mark",
			func(o *domain.MarkObservation) { o.Price = mustPrice(t, "-64000.5") },
			domain.ErrMarkNotPositive,
		},
		{
			"unset receive time",
			func(o *domain.MarkObservation) { o.Watermark.RecvTime = time.Time{} },
			domain.ErrWatermarkUnset,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newMarkHarness(t)
			o := markObs(t, base, base.Add(venueSkew), 1)
			tc.mut(&o)
			if _, err := h.svc.Ingest(o, markOrdinal); !errors.Is(err, tc.want) {
				t.Errorf("err = %v; want %v", err, tc.want)
			}
			if len(h.pub.got) != 0 {
				t.Error("a refused observation published")
			}
		})
	}
}

func mustPrice(t *testing.T, s string) fixedpoint.Price {
	t.Helper()
	p, err := fixedpoint.ParsePrice("BTC", "USDT", s)
	if err != nil {
		t.Fatalf("ParsePrice(%q): %v", s, err)
	}
	return p
}
