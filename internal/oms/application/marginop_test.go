package application_test

import (
	"errors"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/oms/application"
	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/boot"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// fakeMarginOpJournal is ports.MarginOpJournal, recording what was made durable.
//
// It records the RESOLUTIONS in order, because the claim that matters most here
// is negative: a `pending` must never reach the log. A fake that only counted
// calls would satisfy an implementation that wrote every status.
type fakeMarginOpJournal struct {
	dispatched []domain.MarginOp
	resolved   []marginVerdictCall
	reconciled []domain.MarginOpReconciliation
	failOn     map[domain.MarginOpID]error

	// failEvery and failDispatch are the DISPATCH path's scripts (task #215).
	//
	// failOn keys on an op id, which the resolver's tests can name because they
	// build the op themselves. The dispatcher MINTS its id, so a test about its
	// verdict write has nothing to key on — hence a script that names no
	// subject. failDispatch is separate because the two failures sit on opposite
	// sides of the venue call and must never be exercised together: one refuses
	// the send, the other happens after it.
	failEvery    error
	failDispatch error
}

type marginVerdictCall struct {
	id     domain.MarginOpID
	status domain.MarginOpStatus
}

var _ ports.MarginOpJournal = (*fakeMarginOpJournal)(nil)

func (f *fakeMarginOpJournal) AppendMarginOpDispatched(op domain.MarginOp) error {
	if f.failDispatch != nil {
		return f.failDispatch
	}
	f.dispatched = append(f.dispatched, op)
	return nil
}

func (f *fakeMarginOpJournal) AppendMarginOpResolved(id domain.MarginOpID, status domain.MarginOpStatus) error {
	if f.failEvery != nil {
		return f.failEvery
	}
	if err := f.failOn[id]; err != nil {
		return err
	}
	f.resolved = append(f.resolved, marginVerdictCall{id: id, status: status})
	return nil
}

// The reconciliation is recorded WHOLE rather than by op id, because the claims
// about it are about the fields core §7.3 requires — who and when — and a fake
// keeping only the subject would satisfy a use case that dropped the actor.
func (f *fakeMarginOpJournal) AppendMarginOpReconciled(rec domain.MarginOpReconciliation) error {
	if err := f.failOn[rec.Op]; err != nil {
		return err
	}
	f.reconciled = append(f.reconciled, rec)
	return nil
}

// The margin-op fixture. Every number disagrees with every other for the reason
// domain's own table states: before (100) ≠ delta (25), and the applied live
// figure (125) is neither.
const (
	marginOpTestBound = 30 * time.Second
	marginOpTestAsset = fixedpoint.Asset("USDT")
)

// marginOpNow is the LOCAL instant the test clock is built at. The watermark is
// venue time, so the two differ by testVenueOffset — which is the point: a
// resolver that measured the age against the local clock would be wrong by
// exactly that offset, and the fixture is what makes the difference visible.
func marginOpNow() time.Time { return time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC) }

func marginOpClock(t *testing.T) clock.Clock {
	t.Helper()
	clk, err := clock.NewTestClock(marginOpNow(), []clock.Calibration{
		{Venue: perp().Venue, Offset: testVenueOffset, Uncertainty: testUncertainty},
	})
	if err != nil {
		t.Fatalf("NewTestClock: %v", err)
	}
	return clk
}

// marginVenueNow is what ServerNow(venue) answers for the fixture's clock — the
// instant every watermark below is positioned relative to.
func marginVenueNow(t *testing.T) time.Time {
	t.Helper()
	return marginOpClock(t).ServerNow(perp().Venue).Instant()
}

func marginTestMoney(t *testing.T, v string) fixedpoint.Money {
	t.Helper()
	m, err := fixedpoint.ParseMoney(marginOpTestAsset, v)
	if err != nil {
		t.Fatalf("ParseMoney(%q): %v", v, err)
	}
	return m
}

// marginOpAged builds an in-flight op whose watermark sits `age` before the
// venue's current instant.
func marginOpAged(t *testing.T, id domain.MarginOpID, age time.Duration) domain.MarginOp {
	t.Helper()
	return domain.MarginOp{
		ID:           id,
		Instrument:   perp(),
		Delta:        marginTestMoney(t, "25"),
		BeforePosted: marginTestMoney(t, "100"),
		Watermark:    marginVenueNow(t).Add(-age),
	}
}

// marginReadingOf builds the live margin answer, which for this use case is one
// field: §7.1's evidence is the POSTED figure and nothing else on the row.
func marginReadingOf(t *testing.T, posted string) ports.MarginReading {
	t.Helper()
	return ports.MarginReading{
		Instrument:   perp(),
		Maintenance:  marginTestMoney(t, "10"),
		PostedMargin: marginTestMoney(t, posted),
	}
}

// fakeMarginActs is ports.VenueMarginActs.
//
// It records the WINDOWS it was asked for, not only the count, because the claim
// that the fetch window is derived from the op's own watermark and the
// resolution's own bound is otherwise unobservable — a resolver that passed a
// constant window, or swapped the two edges, would satisfy every verdict
// assertion below while fetching evidence for the wrong interval.
type fakeMarginActs struct {
	hist    domain.MarginActHistory
	err     error
	queries []marginActQuery
}

type marginActQuery struct {
	instrument ids.VenueInstrument
	from, to   time.Time
}

var _ ports.VenueMarginActs = (*fakeMarginActs)(nil)

func (f *fakeMarginActs) MarginActs(instrument ids.VenueInstrument, from, to time.Time) (domain.MarginActHistory, error) {
	f.queries = append(f.queries, marginActQuery{instrument: instrument, from: from, to: to})
	if f.err != nil {
		return domain.MarginActHistory{}, f.err
	}
	return f.hist, nil
}

// noActs is the venue-with-no-history double, which is what every test written
// for task #212 was implicitly using. Naming it makes those tests say which
// venue's contract they assert rather than leaving it as the absence of a
// parameter.
func noActs() *fakeMarginActs { return &fakeMarginActs{hist: domain.NoMarginActHistory()} }

func newMarginOpResolverT(t *testing.T, margin ports.VenueMarginStatus, j ports.MarginOpJournal) *application.MarginOpResolver {
	t.Helper()
	return newMarginOpResolverWithActs(t, margin, j, noActs())
}

func newMarginOpResolverWithActs(t *testing.T, margin ports.VenueMarginStatus, j ports.MarginOpJournal,
	acts ports.VenueMarginActs) *application.MarginOpResolver {

	t.Helper()
	r, err := application.NewMarginOpResolver(j, margin, acts, marginOpClock(t), marginOpTestBound)
	if err != nil {
		t.Fatalf("NewMarginOpResolver: %v", err)
	}
	return r
}

func marginFixture(t *testing.T, posted string, held bool) *fakeMargin {
	t.Helper()
	return &fakeMargin{
		statuses: map[ids.VenueInstrument]ports.MarginReading{perp(): marginReadingOf(t, posted)},
		held:     map[ids.VenueInstrument]bool{perp(): held},
	}
}

func TestMarginOpResolverConstructorRefusals(t *testing.T) {
	margin := marginFixture(t, "125", true)
	journal := &fakeMarginOpJournal{}

	cases := []struct {
		name    string
		journal ports.MarginOpJournal
		margin  ports.VenueMarginStatus
		acts    ports.VenueMarginActs
		clk     clock.Clock
		bound   time.Duration
		want    error
	}{
		{"no journal", nil, margin, noActs(), marginOpClock(t), marginOpTestBound, application.ErrMarginOpJournalMissing},
		{"no margin port", journal, nil, noActs(), marginOpClock(t), marginOpTestBound, application.ErrVenueMarginStatusMissing},
		// A nil act port is refused rather than read as "this venue keeps no
		// history". The two are a whole verdict apart, and noActs() above is what
		// the deliberate answer looks like — something with a name.
		{"no act port", journal, margin, nil, marginOpClock(t), marginOpTestBound, application.ErrVenueMarginActsMissing},
		{"no clock", journal, margin, noActs(), nil, marginOpTestBound, application.ErrClockMissing},
		{"zero bound", journal, margin, noActs(), marginOpClock(t), 0, domain.ErrMarginOpBoundNotPositive},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := application.NewMarginOpResolver(tc.journal, tc.margin, tc.acts, tc.clk, tc.bound)
			if !errors.Is(err, tc.want) {
				t.Fatalf("NewMarginOpResolver: got %v, want %v", err, tc.want)
			}
		})
	}

	// The positive control: without it every refusal above is satisfied by a
	// constructor that refuses everything.
	if _, err := application.NewMarginOpResolver(journal, margin, noActs(), marginOpClock(t), marginOpTestBound); err != nil {
		t.Fatalf("NewMarginOpResolver with everything wired: %v", err)
	}
}

func TestMarginOpResolverConcludesAndJournalsTheVerdict(t *testing.T) {
	cases := []struct {
		name       string
		posted     string
		held       bool
		age        time.Duration
		want       domain.MarginOpStatus
		wantWrites int
	}{
		{"applied", "125", true, time.Hour, domain.MarginOpApplied, 1},
		{"not applied", "100", true, time.Hour, domain.MarginOpNotApplied, 1},
		{"ambiguous", "110", true, time.Hour, domain.MarginOpAmbiguous, 1},
		{"flat", "0", false, time.Hour, domain.MarginOpAmbiguous, 1},

		// The one status that is NOT written. It is the whole reason the journal
		// fake records statuses rather than counting calls.
		{"pending", "100", true, time.Second, domain.MarginOpPending, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			journal := &fakeMarginOpJournal{}
			r := newMarginOpResolverT(t, marginFixture(t, tc.posted, tc.held), journal)

			res, err := r.Resolve(application.Recovery{
				InFlightMarginOps: []domain.MarginOp{marginOpAged(t, "op-1", tc.age)},
			})
			if err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			if len(res.Resolved) != 1 {
				t.Fatalf("resolved %d ops, want 1", len(res.Resolved))
			}
			if got := res.Resolved[0].Status; got != tc.want {
				t.Fatalf("status = %s, want %s", got, tc.want)
			}
			if len(journal.resolved) != tc.wantWrites {
				t.Fatalf("journalled %d verdicts, want %d (%v)", len(journal.resolved), tc.wantWrites, journal.resolved)
			}
			if tc.wantWrites == 1 {
				if journal.resolved[0].status != tc.want {
					t.Errorf("journalled %s, want %s", journal.resolved[0].status, tc.want)
				}
				if journal.resolved[0].id != "op-1" {
					t.Errorf("journalled the verdict under %q, want op-1", journal.resolved[0].id)
				}
			}

			// The live reading is carried, because the venue moves on and an
			// operator asked to reconcile a float has no other way to see what the
			// comparison actually saw. Asserted by VALUE and not merely for
			// presence: the posted figure the resolution read is the number the
			// float reconciliation starts from, and a zero there is the one that
			// reads as a position holding no margin at all.
			if got := res.Resolved[0].Held; got != tc.held {
				t.Errorf("Held = %t, want %t", got, tc.held)
			}
			want := marginTestMoney(t, tc.posted)
			if got := res.Resolved[0].LivePosted; got.Mantissa() != want.Mantissa() || got.Asset() != want.Asset() {
				t.Errorf("LivePosted = %s %s, want %s %s",
					got.Decimal(), got.Asset(), want.Decimal(), want.Asset())
			}
		})
	}
}

// TestMarginOpResolverMeasuresTheAgeOnTheVENUEClock is the anchor claim.
//
// # The op has to STRADDLE the bound, and a merely-nonzero skew is not enough
//
// The first version of this test placed the op half a bound old and asserted
// Pending. It SURVIVED a mutation swapping ServerNow(venue) for Now(), and
// correctly: the two clocks are testVenueOffset apart (250ms) against a 30s
// bound, so at half a bound BOTH readings say Pending and the fixture varied the
// right parameter by an amount the predicate cannot see.
//
// So the op is positioned in the window where the two clocks give DIFFERENT
// verdicts — width exactly |skew| — and the expected answer is the VENUE's. A
// resolver reading the host's clock answers the other one, which in the positive
// -skew case is "nothing happened" about a write that may still be landing.
//
// The straddle is computed from the clock rather than assumed, because the
// offset's SIGN decides which side of the bound the venue lands on and this test
// must not silently depend on the fixture's choice of it.
func TestMarginOpResolverMeasuresTheAgeOnTheVENUEClock(t *testing.T) {
	clk := marginOpClock(t)
	skew := clk.ServerNow(perp().Venue).Instant().Sub(clk.Now())
	if skew == 0 {
		t.Fatal("the fixture's venue offset is zero, so the two clocks cannot disagree " +
			"and this test asserts nothing")
	}
	if abs := skew; abs < 0 {
		abs = -abs
		if abs >= marginOpTestBound {
			t.Fatalf("the venue skew (%v) is not smaller than the bound (%v); "+
				"the straddle below assumes it is", abs, marginOpTestBound)
		}
	}

	// age is measured from the VENUE's now, so venueElapsed == age and
	// localElapsed == age - skew. Pick the age that puts exactly one of them past
	// the bound.
	age, want := marginOpTestBound, domain.MarginOpNotApplied
	if skew < 0 {
		age, want = marginOpTestBound+skew, domain.MarginOpPending
	}

	journal := &fakeMarginOpJournal{}
	r := newMarginOpResolverT(t, marginFixture(t, "100", true), journal)

	// Nothing moved, so the only thing deciding between Pending and NotApplied is
	// which clock the age is measured on.
	res, err := r.Resolve(application.Recovery{
		InFlightMarginOps: []domain.MarginOp{marginOpAged(t, "op-1", age)},
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got := res.Resolved[0].Status; got != want {
		t.Fatalf("status = %s, want %s — the age was not measured against ServerNow(venue)", got, want)
	}
}

// marginOpSpec is one in-flight op and the venue answer that decides it.
//
// The age is stated per op rather than derived from the posted figure, because
// deriving it made "nothing moved" and "too recent" the same fixture parameter —
// and those are precisely the two the Pending branch has to tell apart.
type marginOpSpec struct {
	id     domain.MarginOpID
	posted string
	age    time.Duration
}

func TestMarginOpResolverCountsAndFolds(t *testing.T) {
	old, fresh := time.Hour, time.Second

	cases := []struct {
		name        string
		inFlight    []marginOpSpec
		carried     []domain.MarginOpID
		wantAmbig   int
		wantPending int
		wantOutcome boot.Outcome
	}{
		{"nothing at all", nil, nil, 0, 0, boot.OutcomePassed},
		{"all resolved cleanly", []marginOpSpec{
			{"op-applied", "125", old}, {"op-not-applied", "100", old},
		}, nil, 0, 0, boot.OutcomePassed},
		{"one ambiguous", []marginOpSpec{{"op-1", "110", old}}, nil, 1, 0, boot.OutcomeBlocksOpening},
		{"one carried ambiguity and nothing in flight", nil,
			[]domain.MarginOpID{"old-1"}, 1, 0, boot.OutcomeBlocksOpening},
		{"two sources of ambiguity are one count", []marginOpSpec{{"op-1", "110", old}},
			[]domain.MarginOpID{"old-1"}, 2, 0, boot.OutcomeBlocksOpening},

		// The fold's own claim: an ambiguity beside a pending reports the
		// AMBIGUITY, even though OutcomeIncomplete's posture is more restrictive.
		// Reporting the pending would lose the latch and take the safety ladder
		// away during the incident §7.1 says safety actions must continue through.
		{"an ambiguity outranks a pending", []marginOpSpec{
			{"op-1", "110", old}, {"op-2", "100", fresh},
		}, nil, 1, 1, boot.OutcomeBlocksOpening},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// The margin port is keyed by instrument, so one fake cannot answer
			// differently for two ops on the same market. Each op is resolved in
			// its own pass against its own fake and the counts are summed, which
			// is what the real phase does across a list.
			res := application.MarginResolution{
				Carried:   tc.carried,
				Ambiguous: len(tc.carried),
			}
			for _, spec := range tc.inFlight {
				journal := &fakeMarginOpJournal{}
				r := newMarginOpResolverT(t, marginFixture(t, spec.posted, true), journal)
				one, err := r.Resolve(application.Recovery{
					InFlightMarginOps: []domain.MarginOp{marginOpAged(t, spec.id, spec.age)},
				})
				if err != nil {
					t.Fatalf("Resolve(%s): %v", spec.id, err)
				}
				res.Resolved = append(res.Resolved, one.Resolved...)
				res.Ambiguous += one.Ambiguous
				res.Pending += one.Pending
			}

			if res.Ambiguous != tc.wantAmbig {
				t.Errorf("Ambiguous = %d, want %d", res.Ambiguous, tc.wantAmbig)
			}
			if res.Pending != tc.wantPending {
				t.Errorf("Pending = %d, want %d", res.Pending, tc.wantPending)
			}
			if got := res.Outcome(); got != tc.wantOutcome {
				t.Errorf("Outcome() = %s, want %s", got, tc.wantOutcome)
			}
		})
	}
}

// TestMarginOpResolverPendingAloneIsIncomplete is the fold's other arm, split
// out because the table above never has a pending WITHOUT an ambiguity beside
// it — and with one beside it, "reports the ambiguity" and "reports the more
// restrictive" would be the same answer for a reader who did not check.
func TestMarginOpResolverPendingAloneIsIncomplete(t *testing.T) {
	journal := &fakeMarginOpJournal{}
	r := newMarginOpResolverT(t, marginFixture(t, "100", true), journal)

	res, err := r.Resolve(application.Recovery{
		InFlightMarginOps: []domain.MarginOp{marginOpAged(t, "op-1", time.Second)},
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if res.Pending != 1 || res.Ambiguous != 0 {
		t.Fatalf("Pending=%d Ambiguous=%d, want 1 and 0", res.Pending, res.Ambiguous)
	}
	if got := res.Outcome(); got != boot.OutcomeIncomplete {
		t.Fatalf("Outcome() = %s, want OutcomeIncomplete", got)
	}
}

// TestMarginOpResolverCarriesAnAmbiguityNothingRecomputes is the stored
// verdict's whole point: an ambiguity a PREVIOUS process concluded still blocks,
// and this process performs no read to establish it.
//
// The margin port is scripted to answer "applied" — the reading a re-derivation
// would reach — so a resolver that recomputed carried verdicts would report a
// clean boot. Nothing is in flight, so the port must not be consulted at all.
func TestMarginOpResolverCarriesAnAmbiguityNothingRecomputes(t *testing.T) {
	journal := &fakeMarginOpJournal{}
	margin := marginFixture(t, "125", true)
	r := newMarginOpResolverT(t, margin, journal)

	res, err := r.Resolve(application.Recovery{
		AmbiguousMarginOps: []domain.MarginOpID{"old-1", "old-2"},
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if res.Ambiguous != 2 {
		t.Errorf("Ambiguous = %d, want 2", res.Ambiguous)
	}
	if len(res.Carried) != 2 {
		t.Errorf("Carried = %v, want two ids", res.Carried)
	}
	if len(res.Resolved) != 0 {
		t.Errorf("resolved %d ops, want 0 — a carried verdict is read, not recomputed", len(res.Resolved))
	}
	if len(journal.resolved) != 0 {
		t.Errorf("wrote %d verdicts for carried ambiguities, want 0", len(journal.resolved))
	}
	if got := res.Outcome(); got != boot.OutcomeBlocksOpening {
		t.Errorf("Outcome() = %s, want OutcomeBlocksOpening", got)
	}
}

func TestMarginOpResolverFailures(t *testing.T) {
	venueDown := errors.New("venue unreachable")

	t.Run("the margin read fails", func(t *testing.T) {
		journal := &fakeMarginOpJournal{}
		r := newMarginOpResolverT(t, &fakeMargin{err: venueDown}, journal)

		res, err := r.Resolve(application.Recovery{
			InFlightMarginOps: []domain.MarginOp{marginOpAged(t, "op-1", time.Hour)},
		})
		if !errors.Is(err, venueDown) {
			t.Fatalf("Resolve: got %v, want the venue's error", err)
		}
		if len(res.Resolved) != 0 {
			t.Errorf("resolved %d ops over an unreadable venue", len(res.Resolved))
		}
		if len(journal.resolved) != 0 {
			t.Errorf("wrote %d verdicts over an unreadable venue", len(journal.resolved))
		}
	})

	t.Run("the verdict cannot be made durable", func(t *testing.T) {
		// The verdict is reached and cannot be recorded. It must NOT be reported
		// as concluded: the next boot would find the op still in flight and
		// resolve it from a staler window, which is the drift the durable verdict
		// exists to stop — and a caller told the op resolved would have no reason
		// to expect that.
		diskFull := errors.New("journal write failed")
		journal := &fakeMarginOpJournal{failOn: map[domain.MarginOpID]error{"op-1": diskFull}}
		r := newMarginOpResolverT(t, marginFixture(t, "125", true), journal)

		res, err := r.Resolve(application.Recovery{
			InFlightMarginOps: []domain.MarginOp{marginOpAged(t, "op-1", time.Hour)},
		})
		if !errors.Is(err, diskFull) {
			t.Fatalf("Resolve: got %v, want the journal's error", err)
		}
		if len(res.Resolved) != 0 {
			t.Errorf("reported %d resolutions whose verdict was never made durable", len(res.Resolved))
		}
	})

	t.Run("a failure keeps what it built", func(t *testing.T) {
		// Two ops, the second unreadable. The first's verdict is durable and stays
		// so — Rebuild's rule, and here it is load-bearing: re-deriving it at the
		// next boot is exactly what the stored verdict prevents.
		//
		// The fake answers per instrument, so the second op names an instrument it
		// has no entry for, which reads as flat rather than as an error. To fail
		// the SECOND read specifically the fake is swapped after the first — which
		// no fake supports — so this asserts the weaker, checkable half: the ops
		// resolved before a failure are reported and journalled.
		journal := &fakeMarginOpJournal{failOn: map[domain.MarginOpID]error{"op-2": errors.New("nope")}}
		r := newMarginOpResolverT(t, marginFixture(t, "125", true), journal)

		res, err := r.Resolve(application.Recovery{InFlightMarginOps: []domain.MarginOp{
			marginOpAged(t, "op-1", time.Hour),
			marginOpAged(t, "op-2", time.Hour),
		}})
		if err == nil {
			t.Fatal("Resolve returned nil over a journal that refused the second verdict")
		}
		if len(res.Resolved) != 1 || res.Resolved[0].Op.ID != "op-1" {
			t.Fatalf("kept %v, want exactly op-1's resolution", res.Resolved)
		}
		if len(journal.resolved) != 1 || journal.resolved[0].id != "op-1" {
			t.Fatalf("journal holds %v, want exactly op-1's verdict", journal.resolved)
		}
	})
}

// TestMarginOpResolverIssuesNothing is §7.1's prohibition as a test: "blind delta
// retries are prohibited".
//
// The resolver holds a journal and a READ port, so a retry is not expressible —
// which is the claim. Asserted through the ports it was handed rather than
// through behaviour, because the absence of a call is what a behavioural test
// cannot distinguish from a call that happened not to fire.
func TestMarginOpResolverIssuesNothing(t *testing.T) {
	journal := &fakeMarginOpJournal{}
	r := newMarginOpResolverT(t, marginFixture(t, "100", true), journal)

	if _, err := r.Resolve(application.Recovery{
		InFlightMarginOps: []domain.MarginOp{marginOpAged(t, "op-1", time.Hour)},
	}); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	// The only durable write a resolution may perform is the verdict. A DISPATCH
	// record would mean this type had re-fired the op.
	if len(journal.dispatched) != 0 {
		t.Fatalf("the resolver journalled %d dispatches — it re-fired a margin op",
			len(journal.dispatched))
	}
}

// TestTheActFetchWindowIsTheOPsOwnAnchorAndBOUND (task #214).
//
// The window handed to the act port must be [Watermark, Watermark+bound), built
// from the same two quantities the domain's match re-applies. A resolver that
// passed a constant window, the LOCAL clock's instant, or the two edges swapped
// would satisfy every verdict assertion in this file while fetching evidence for
// the wrong interval — and a wrong interval returns no rows, which is the
// observation that turns a correct top-up into an operator incident.
//
// The op is aged deliberately, so the watermark is neither `now` nor the zero
// instant and no edge can be right by coincidence.
func TestTheActFetchWindowIsTheOPsOwnAnchorAndBOUND(t *testing.T) {
	op := marginOpAged(t, "op-window", 5*time.Minute)
	acts := noActs()
	journal := &fakeMarginOpJournal{}
	r := newMarginOpResolverWithActs(t, marginFixture(t, "125", true), journal, acts)

	if _, err := r.Resolve(application.Recovery{InFlightMarginOps: []domain.MarginOp{op}}); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(acts.queries) != 1 {
		t.Fatalf("the act port was asked %d times, want 1", len(acts.queries))
	}

	q := acts.queries[0]
	if !q.from.Equal(op.Watermark) {
		t.Errorf("window opens at %v, want the op's watermark %v", q.from, op.Watermark)
	}
	if want := op.Watermark.Add(marginOpTestBound); !q.to.Equal(want) {
		t.Errorf("window closes at %v, want watermark+bound %v", q.to, want)
	}
	if !q.from.Before(q.to) {
		t.Error("the window is empty or inverted")
	}
	if q.instrument != op.Instrument {
		t.Errorf("window asked about %v, want %v", q.instrument, op.Instrument)
	}
}

// TestAFailedActReadFailsTheRESOLUTION.
//
// The resolver's standing rule: inside a boot a venue that will not answer is an
// error, because the checklist has a member for it and the gate is what decides
// next. Absorbing it into a verdict would resolve the op from one source while
// reporting the boot clean.
func TestAFailedActReadFailsTheRESOLUTION(t *testing.T) {
	boom := errors.New("act history unreachable")
	acts := &fakeMarginActs{err: boom}
	journal := &fakeMarginOpJournal{}
	r := newMarginOpResolverWithActs(t, marginFixture(t, "125", true), journal, acts)

	_, err := r.Resolve(application.Recovery{InFlightMarginOps: []domain.MarginOp{marginOpAged(t, "op-fail", time.Minute)}})
	if !errors.Is(err, boom) {
		t.Fatalf("Resolve: got %v, want %v", err, boom)
	}
	if len(journal.resolved) != 0 {
		t.Errorf("a verdict was journaled despite the failed read: %v", journal.resolved)
	}
}
