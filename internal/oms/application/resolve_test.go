package application_test

import (
	"errors"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/oms/application"
	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// # The fixtures, and the six collapses the checklist closed BEFORE the battery
//
// Each was chosen because the obvious value makes a claim in this file vacuous.
//
//  1. Every anchor carries a NONZERO uncertainty band. A ServerTime built with
//     zero uncertainty has Earliest == Instant == Latest, so no test could tell
//     which edge a gate reads — and §5.3's two time bounds deliberately read
//     OPPOSITE edges (the horizon takes Latest, the retention cliff takes
//     Earliest). Each bound needs its OWN band-edge fixture, and the pair is
//     TestResolveMeasuresTheHorizonFromTheAnchorsLatestEdge and
//     TestResolveMeasuresTheCliffFromTheAnchorsEarliestEdge. This entry once
//     named the pair and cited only the first, which is how the second came to
//     be missing: a nonzero band makes the distinction EXPRESSIBLE, and only a
//     clock placed inside that band makes it OBSERVED.
//  2. The two retentions DIFFER, and one case inverts their order. Equal
//     retentions make min(a,b), a and b the same answer, so the rule that gate 4
//     takes the shorter would be untestable — and reading UnfilledRetention by
//     name happens to be right on both venues today, which is exactly why the
//     inverted fixture exists.
//  3. MaxLanding != SweepMargin. Equal values make "adds both" and "doubles one"
//     the same horizon.
//  4. Sweeps carry MORE THAN ONE row, and the ambiguous row comes FIRST. With
//     one row, "examines every row" and "returns on the first ambiguous one" are
//     the same function; with the ambiguous row last, so are they.
//  5. The sweep here is a single ANSWER, because paging is the adapter's — the
//     multi-page fixture lives in driven/orderresolver_test.go, where three
//     pages with a different watermark each separate "reports the last
//     page's watermark" from "reports the first's" and from "the second's".
//  6. Records are planted at a version that is NOT 1. Every record ApplyPlanned
//     creates is at version 1, so a suite that only ever resolves version-1
//     records cannot tell "threads the record's version" from "assumes the
//     first".

// fakeResolver is ports.OrderResolver at the port.
//
// It counts BOTH methods, because two claims in this file are about a call NOT
// happening: a missing anchor must refuse before spending a venue request, and a
// non-terminal present order must not sweep. Neither is visible from the
// returned Resolution.
type fakeResolver struct {
	status    ports.VenueOrder
	statusErr error

	sweeps    []ports.ExecutionSweep
	sweepErr  error
	resolves  int
	sweepCall int

	// windows records the (since, until) each sweep was asked for, which is the
	// only evidence that the resolver built the window from the anchor's band
	// rather than from its instant.
	windows []sweepWindow
}

type sweepWindow struct {
	since time.Time
	until time.Time
}

func (f *fakeResolver) Resolve(domain.Order) (ports.VenueOrder, error) {
	f.resolves++
	if f.statusErr != nil {
		return ports.VenueOrder{}, f.statusErr
	}
	return f.status, nil
}

func (f *fakeResolver) SweepExecutions(_ domain.Order, since, until time.Time) (ports.ExecutionSweep, error) {
	f.windows = append(f.windows, sweepWindow{since: since, until: until})
	f.sweepCall++
	if f.sweepErr != nil {
		return ports.ExecutionSweep{}, f.sweepErr
	}
	if len(f.sweeps) == 0 {
		return ports.ExecutionSweep{}, nil
	}
	i := min(f.sweepCall-1, len(f.sweeps)-1)
	return f.sweeps[i], nil
}

var _ ports.OrderResolver = (*fakeResolver)(nil)

// The policy every test starts from. The two durations differ (checklist item
// 3), and both are far larger than the millisecond-scale clock bands below so
// that a test about the horizon is not accidentally a test about the band.
const (
	testMaxLanding  = 30 * time.Second
	testSweepMargin = 5 * time.Second

	// testUncertainty is the clock band. Nonzero (item 1) and small relative to
	// the durations above.
	testUncertainty = 40 * time.Millisecond
)

func testPolicy() application.ResolutionPolicy {
	return application.ResolutionPolicy{
		MaxLanding:  testMaxLanding,
		SweepMargin: testSweepMargin,
	}
}

// publishedReach is a venue that states both horizons, with the two DIFFERENT
// (item 2) and the unfilled one shorter — the real venues' shape.
func publishedReach() ports.SurfaceReach {
	return ports.SurfaceReach{
		Surface:           "order_status",
		UnfilledRetention: 3 * time.Hour,
		FilledRetention:   90 * time.Hour,
		Published:         true,
	}
}

// resolvableAt plants an order at a resolvable state with an anchor, at a
// version that is not 1 (item 6), and returns its anchor.
func resolvableAt(t *testing.T, r *fakeOrders, o domain.Order, state domain.OrderState, anchorAt time.Time) clock.ServerTime {
	t.Helper()
	at, err := clock.NewServerTime(anchorAt, testUncertainty)
	if err != nil {
		t.Fatalf("NewServerTime: %v", err)
	}
	r.records[o.ClientOrderID] = ports.OrderRecord{
		Order: o, State: state, Version: 7, AuthorizedAt: at,
	}
	return at
}

// venueNow converts an engine instant into what the venue's clock reads, so a
// test can place "now" relative to an anchor expressed in the venue's frame.
// The offset is testClock's, named once rather than restated.
const testVenueOffset = 250 * time.Millisecond

// newResolver builds the use case over fresh fakes at a chosen engine instant.
func newResolver(t *testing.T, now time.Time, policy application.ResolutionPolicy) (*application.Resolver, *fakeJournal, *fakeOrders, *fakeResolver) {
	t.Helper()
	j := newFakeJournal()
	r := newFakeOrders()
	v := &fakeResolver{}
	clk, err := clock.NewTestClock(now, []clock.Calibration{
		{Venue: perp().Venue, Offset: testVenueOffset, Uncertainty: testUncertainty},
	})
	if err != nil {
		t.Fatalf("NewTestClock: %v", err)
	}
	res, err := application.NewResolver(j, r, v, r, clk, policy)
	if err != nil {
		t.Fatalf("NewResolver: %v", err)
	}
	return res, j, r, v
}

// absentVenue is a venue answering "no such order" with a published reach.
func absentVenue(reach ports.SurfaceReach) ports.VenueOrder {
	return ports.VenueOrder{Presence: ports.VenueHasNoOrder, Reach: reach}
}

// provenSweep is a sweep that reached exactly the horizon and saw the given
// rows. Exactly the horizon rather than past it, because that is the boundary
// the rule compares with Before — a fixture strictly past it cannot tell `<`
// from `<=`.
func provenSweep(through time.Time, rows ...ports.VenueExecution) ports.ExecutionSweep {
	return ports.ExecutionSweep{Executions: rows, ProvenThrough: through}
}

// row builds one execution attributed to a named client order id. An EMPTY id
// is Binance's shape — its trade lists return none — and is what makes a sweep
// inconclusive.
func row(id ids.ClientOrderID, executionID string) ports.VenueExecution {
	return ports.VenueExecution{ExecutionID: ids.ExecutionID(executionID), ClientOrderID: id}
}

// horizonOf is the instant the rule's gate 1 boundary falls on, computed the way
// the rule computes it. It is used to place the CLOCK, never to assert — a test
// that asserted against this would be checking the implementation against
// itself.
func horizonOf(anchor clock.ServerTime, policy application.ResolutionPolicy) time.Time {
	return anchor.Latest().Add(policy.MaxLanding + policy.SweepMargin)
}

// engineTimeFor returns the engine instant at which the venue clock certainly
// reads at or past boundary — the earliest such instant plus a margin, so the
// fixture is not sitting on the band edge by accident.
func engineTimeFor(boundary time.Time) time.Time {
	// CertainlyPassed needs venueNow.Earliest() >= boundary, i.e.
	// engine + offset - uncertainty >= boundary.
	return boundary.Add(testUncertainty - testVenueOffset).Add(time.Millisecond)
}

// engineTimeStraddling returns the engine instant at which the venue clock's
// band CONTAINS boundary — its instant sits exactly on it, so Earliest is before
// and Latest is at or past, and Passed answers Uncertain.
//
// It is the fixture no test in this file had before the battery, and two
// surviving mutants needed it: a gate reading `== CertainlyPassed` instead of
// `!= CertainlyNotPassed` differs from the intended rule on this input ALONE,
// and nowhere else.
func engineTimeStraddling(boundary time.Time) time.Time {
	return boundary.Add(-testVenueOffset)
}

// engineTimeJustBefore returns the LAST engine instant at which the venue clock
// certainly has not reached boundary — its Latest edge one nanosecond short.
//
// The tightest position from which a gate may still conclude, so a rule that
// took the wrong band edge would conclude here too and the assertion pairs with
// the straddle above.
func engineTimeJustBefore(boundary time.Time) time.Time {
	// CertainlyNotPassed needs venueNow.Latest() < boundary, i.e.
	// engine + offset + uncertainty < boundary.
	return boundary.Add(-testVenueOffset - testUncertainty - time.Nanosecond)
}

func TestNewResolverRefusesAMissingPortOrAnUnusablePolicy(t *testing.T) {
	t.Parallel()

	clk, err := clock.NewTestClock(testNow, nil)
	if err != nil {
		t.Fatalf("NewTestClock: %v", err)
	}

	// One omission at a time and a DIFFERENT sentinel per case, so a constructor
	// that answered one blanket refusal to everything does not score five.
	if _, err := application.NewResolver(nil, newFakeOrders(), &fakeResolver{}, newFakeOrders(), clk, testPolicy()); !errors.Is(err, application.ErrJournalMissing) {
		t.Errorf("NewResolver(nil journal) = %v, want ErrJournalMissing", err)
	}
	if _, err := application.NewResolver(newFakeJournal(), nil, &fakeResolver{}, newFakeOrders(), clk, testPolicy()); !errors.Is(err, application.ErrOrdersMissing) {
		t.Errorf("NewResolver(nil orders) = %v, want ErrOrdersMissing", err)
	}
	if _, err := application.NewResolver(newFakeJournal(), newFakeOrders(), nil, newFakeOrders(), clk, testPolicy()); !errors.Is(err, application.ErrResolverMissing) {
		t.Errorf("NewResolver(nil venue) = %v, want ErrResolverMissing", err)
	}
	if _, err := application.NewResolver(newFakeJournal(), newFakeOrders(), &fakeResolver{}, nil, clk, testPolicy()); !errors.Is(err, application.ErrVenueOrderIndexMissing) {
		t.Errorf("NewResolver(nil index) = %v, want ErrVenueOrderIndexMissing", err)
	}
	if _, err := application.NewResolver(newFakeJournal(), newFakeOrders(), &fakeResolver{}, newFakeOrders(), nil, testPolicy()); !errors.Is(err, application.ErrClockMissing) {
		t.Errorf("NewResolver(nil clock) = %v, want ErrClockMissing", err)
	}
	if _, err := application.NewResolver(newFakeJournal(), newFakeOrders(), &fakeResolver{}, newFakeOrders(), clk, application.ResolutionPolicy{}); !errors.Is(err, application.ErrResolutionPolicyUnset) {
		t.Errorf("NewResolver(zero policy) = %v, want ErrResolutionPolicyUnset", err)
	}

	// The positive control. Without it a constructor that refused everything
	// would score five out of five above.
	if _, err := application.NewResolver(newFakeJournal(), newFakeOrders(), &fakeResolver{}, newFakeOrders(), clk, testPolicy()); err != nil {
		t.Errorf("NewResolver with every dependency = %v, want nil", err)
	}
}

// Each duration is zeroed SEPARATELY. A policy with both zeroed would be
// satisfied by a Validate that checked only one, and both removals delete a
// different gate: a zero MaxLanding says a send stops being able to land the
// instant it is authorized, a zero SweepMargin makes SM §2 step 3's "strictly
// later" an equality.
func TestResolutionPolicyValidateNamesEachMissingDuration(t *testing.T) {
	t.Parallel()

	cases := map[string]application.ResolutionPolicy{
		"no landing bound": {SweepMargin: testSweepMargin},
		"no sweep margin":  {MaxLanding: testMaxLanding},
		"negative landing": {MaxLanding: -time.Second, SweepMargin: testSweepMargin},
		"assumed reach of zero": {
			MaxLanding:  testMaxLanding,
			SweepMargin: testSweepMargin,
			AssumedReach: map[application.SurfaceKey]time.Duration{
				{Venue: "binance", Product: ids.ProductSpot}: 0,
			},
		},
	}
	for name, policy := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if err := policy.Validate(); !errors.Is(err, application.ErrResolutionPolicyUnset) {
				t.Errorf("Validate = %v, want ErrResolutionPolicyUnset", err)
			}
		})
	}

	if err := testPolicy().Validate(); err != nil {
		t.Errorf("Validate of a complete policy = %v, want nil", err)
	}
}

// The resolvable set is DERIVED from SM §1 — an order is resolvable exactly when
// the domain would accept proved absence for it — so this table is the
// derivation's consequence rather than a second copy of it. StatePlanned is the
// case worth naming: §5.3 says such an order was never dispatched, so a query
// about it would run the finality contract to its deadline over an order we have
// not sent.
func TestResolveRefusesAnOrderTheDomainWouldNotProveAbsent(t *testing.T) {
	t.Parallel()

	refused := []domain.OrderState{
		domain.StatePlanned,
		domain.StateAcknowledged,
		domain.StatePartiallyFilled,
		domain.StateCancelPending,
		domain.StateCancelUnknown,
		domain.StateFilled,
		domain.StateCanceled,
		domain.StateRejected,
		domain.StateAbsentAfterFinality,
	}
	for _, state := range refused {
		t.Run(state.String(), func(t *testing.T) {
			t.Parallel()

			res, j, r, v := newResolver(t, testNow, testPolicy())
			o := dispatchableOrder(t)
			resolvableAt(t, r, o, state, testNow)

			_, err := res.Resolve(o.ClientOrderID)
			if !errors.Is(err, application.ErrOrderNotResolvable) {
				t.Fatalf("Resolve at %s = %v, want ErrOrderNotResolvable", state, err)
			}
			if v.resolves != 0 {
				t.Errorf("queried the venue %d times about an unresolvable order, want 0", v.resolves)
			}
			if len(j.calls) != 0 {
				t.Errorf("journaled %v, want nothing", j.states())
			}
		})
	}

	// The positive control, and it is what makes the list above a claim rather
	// than a universally-refusing predicate: these three ARE resolvable.
	//
	// StateSubmitted joined them in #242, and it joined by DERIVATION rather than
	// by edit — the gate asks the domain whether proved absence is an answer it
	// would accept, and SM §1's new query-resolution row says it is. That is the
	// derivation working as designed, and it closes a second gap nobody had
	// named: a successfully placed order the venue holds NO record of could not
	// be resolved by §5.3's rule at all, because the one state every such order
	// is in was refused at the door.
	for _, state := range []domain.OrderState{
		domain.StateDispatchAuthorized,
		domain.StateSubmitted,
		domain.StateSubmissionUnknown,
	} {
		t.Run("admitted/"+state.String(), func(t *testing.T) {
			t.Parallel()

			at, err := clock.NewServerTime(testNow, testUncertainty)
			if err != nil {
				t.Fatalf("NewServerTime: %v", err)
			}
			// Past the landing horizon, so that a refusal here can only be about
			// the STATE — which is what this control is asserting.
			res, _, r, v := newResolver(t, engineTimeFor(horizonOf(at, testPolicy())), testPolicy())
			o := dispatchableOrder(t)
			resolvableAt(t, r, o, state, testNow)
			v.status = absentVenue(publishedReach())

			if _, err := res.Resolve(o.ClientOrderID); err != nil {
				t.Fatalf("Resolve at %s = %v, want nil", state, err)
			}
			if v.resolves != 1 {
				t.Errorf("queried the venue %d times, want 1", v.resolves)
			}
		})
	}
}

// The refusal must happen BEFORE a venue request is spent: §5.3 requires
// rate-limit budget to be reserved for recovery, and no answer could supply the
// instant that is missing.
func TestResolveRefusesToProveAbsenceWithNoAnchor(t *testing.T) {
	t.Parallel()

	res, j, r, v := newResolver(t, testNow, testPolicy())
	o := dispatchableOrder(t)
	r.records[o.ClientOrderID] = ports.OrderRecord{
		Order: o, State: domain.StateDispatchAuthorized, Version: 7,
	}

	got, err := res.Resolve(o.ClientOrderID)
	if err != nil {
		t.Fatalf("Resolve = %v, want nil", err)
	}
	if got.Outcome != application.OutcomeUnproved || got.Reason != application.ReasonNoAnchor {
		t.Fatalf("Resolve = %s/%s, want unproved/no_anchor", got.Outcome, got.Reason)
	}
	if v.resolves != 0 {
		t.Errorf("queried the venue %d times with no anchor, want 0", v.resolves)
	}
	if len(j.calls) != 0 || len(r.applied) != 0 {
		t.Errorf("wrote journal=%v applied=%d, want nothing", j.states(), len(r.applied))
	}
}

// TestResolveMeasuresTheHorizonFromTheAnchorsLatestEdge is the band-edge test,
// and it is the reason every anchor here carries a nonzero uncertainty.
//
// The clock is placed in the window where the horizon computed from the anchor's
// INSTANT has certainly passed and the one computed from its LATEST has not. An
// implementation reading the instant proves absence here; the correct one waits.
// The window is exactly the band's width, so no fixture with a zero band could
// contain it.
func TestResolveMeasuresTheHorizonFromTheAnchorsLatestEdge(t *testing.T) {
	t.Parallel()

	anchorAt := testNow
	at, err := clock.NewServerTime(anchorAt, testUncertainty)
	if err != nil {
		t.Fatalf("NewServerTime: %v", err)
	}
	wrongHorizon := at.Instant().Add(testMaxLanding + testSweepMargin)

	// Half a band past the point where the WRONG horizon is certainly passed,
	// which is still short of the right one.
	now := engineTimeFor(wrongHorizon).Add(testUncertainty / 2)

	res, _, r, v := newResolver(t, now, testPolicy())
	o := dispatchableOrder(t)
	resolvableAt(t, r, o, domain.StateDispatchAuthorized, anchorAt)
	v.status = absentVenue(publishedReach())
	v.sweeps = []ports.ExecutionSweep{provenSweep(horizonOf(at, testPolicy()))}

	got, err := res.Resolve(o.ClientOrderID)
	if err != nil {
		t.Fatalf("Resolve = %v, want nil", err)
	}
	if got.Reason != application.ReasonHorizonNotPassed {
		t.Fatalf("Resolve = %s/%s, want unproved/horizon_not_passed — the horizon must be measured from the anchor's LATEST edge",
			got.Outcome, got.Reason)
	}
}

func TestResolveProvesAbsenceWhenEveryGatePasses(t *testing.T) {
	t.Parallel()

	anchorAt := testNow
	at, err := clock.NewServerTime(anchorAt, testUncertainty)
	if err != nil {
		t.Fatalf("NewServerTime: %v", err)
	}
	horizon := horizonOf(at, testPolicy())

	res, j, r, v := newResolver(t, engineTimeFor(horizon), testPolicy())
	o := dispatchableOrder(t)
	resolvableAt(t, r, o, domain.StateSubmissionUnknown, anchorAt)
	v.status = absentVenue(publishedReach())
	// Exactly at the horizon: the boundary case, which a fixture proving
	// coverage well past it could not distinguish from a `<=` comparison.
	v.sweeps = []ports.ExecutionSweep{provenSweep(horizon)}

	got, err := res.Resolve(o.ClientOrderID)
	if err != nil {
		t.Fatalf("Resolve = %v, want nil", err)
	}
	if got.Outcome != application.OutcomeResolved || got.State != domain.StateAbsentAfterFinality {
		t.Fatalf("Resolve = %s/%s, want resolved/absent_after_finality", got.Outcome, got.State)
	}
	if !got.Journaled || !got.Applied {
		t.Errorf("Journaled=%v Applied=%v, want both true", got.Journaled, got.Applied)
	}

	want := []domain.OrderState{domain.StateAbsentAfterFinality}
	if got := j.states(); len(got) != 1 || got[0] != want[0] {
		t.Errorf("journaled %v, want %v", got, want)
	}
	// The version the record actually held, not the first one — checklist item 6.
	if len(r.applied) != 1 || r.applied[0].expected != 7 {
		t.Errorf("applied %+v, want one write at expected version 7", r.applied)
	}

	// The sweep starts at the anchor's EARLIEST edge, which cannot be after the
	// venue created the order.
	if len(v.windows) != 1 || !v.windows[0].since.Equal(at.Earliest()) {
		t.Errorf("swept from %v, want the anchor's earliest edge %v", v.windows, at.Earliest())
	}
	if !v.windows[0].until.Equal(horizon) {
		t.Errorf("swept until %v, want the horizon %v", v.windows[0].until, horizon)
	}
}

// TestResolveProvesAbsenceOfAPlacedOrder is the case the derivation opened.
//
// Before #242 a Submitted order was refused at the resolvable-set gate, so the
// one state every successfully placed order occupies was the one state §5.3's
// stable-not-found rule could not be run for. A Place that reached the venue's
// edge and left no record there — the lost-ack-plus-nothing-landed case — had no
// terminal available to it.
func TestResolveProvesAbsenceOfAPlacedOrder(t *testing.T) {
	t.Parallel()

	anchorAt := testNow
	at, err := clock.NewServerTime(anchorAt, testUncertainty)
	if err != nil {
		t.Fatalf("NewServerTime: %v", err)
	}
	horizon := horizonOf(at, testPolicy())

	res, j, r, v := newResolver(t, engineTimeFor(horizon), testPolicy())
	o := dispatchableOrder(t)
	resolvableAt(t, r, o, domain.StateSubmitted, anchorAt)
	v.status = absentVenue(publishedReach())
	v.sweeps = []ports.ExecutionSweep{provenSweep(horizon)}

	got, err := res.Resolve(o.ClientOrderID)
	if err != nil {
		t.Fatalf("Resolve = %v, want nil", err)
	}
	if got.Outcome != application.OutcomeResolved {
		t.Fatalf("Outcome = %v (%v), want resolved", got.Outcome, got.Reason)
	}
	if got.State != domain.StateAbsentAfterFinality {
		t.Errorf("State = %s, want absent_after_finality", got.State)
	}
	if !got.Journaled || !got.Applied {
		t.Errorf("Journaled=%v Applied=%v, want both true", got.Journaled, got.Applied)
	}
	if states := j.states(); len(states) != 1 || states[0] != domain.StateAbsentAfterFinality {
		t.Errorf("journal holds %v, want one absent_after_finality record", states)
	}
}

// Gate 4, in both of its refusing directions. Uncertain must refuse as loudly as
// certainly-past: §5.3's terminal is PROVED absence, and a band straddling the
// cliff means both readings are consistent with what we know.
func TestResolveRefusesAbsenceOutsideTheSurfacesReach(t *testing.T) {
	t.Parallel()

	// The order is old enough that the unfilled retention has certainly expired.
	anchorAt := testNow
	at, err := clock.NewServerTime(anchorAt, testUncertainty)
	if err != nil {
		t.Fatalf("NewServerTime: %v", err)
	}

	cases := map[string]struct {
		reach ports.SurfaceReach
		// age is how far past the anchor the clock sits.
		age  time.Duration
		want application.UnprovedReason
	}{
		"past the shorter retention": {
			reach: publishedReach(), age: 4 * time.Hour, want: application.ReasonReachExceeded,
		},
		// The INVERTED fixture (checklist item 2): the venue publishes a longer
		// unfilled retention than filled. Reading UnfilledRetention by name
		// would license a proof here; taking the minimum refuses.
		"past the shorter retention, fields inverted": {
			reach: ports.SurfaceReach{
				Surface: "order_status", UnfilledRetention: 90 * time.Hour,
				FilledRetention: 3 * time.Hour, Published: true,
			},
			age: 4 * time.Hour, want: application.ReasonReachExceeded,
		},
		"unpublished with no configured assumption": {
			reach: ports.SurfaceReach{Surface: "order_status"},
			age:   time.Minute, want: application.ReasonReachUnknown,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			now := engineTimeFor(anchorAt.Add(tc.age))
			res, j, r, v := newResolver(t, now, testPolicy())
			o := dispatchableOrder(t)
			resolvableAt(t, r, o, domain.StateDispatchAuthorized, anchorAt)
			v.status = absentVenue(tc.reach)
			v.sweeps = []ports.ExecutionSweep{provenSweep(now.Add(time.Hour))}

			got, err := res.Resolve(o.ClientOrderID)
			if err != nil {
				t.Fatalf("Resolve = %v, want nil", err)
			}
			if got.Reason != tc.want {
				t.Fatalf("Resolve = %s/%s, want unproved/%s", got.Outcome, got.Reason, tc.want)
			}
			// Gate 4 refuses BEFORE the sweep is paid for — §5.3 reserves the
			// rate-limit budget for recovery, and a sweep whose answer could not
			// be used spends it for nothing.
			if v.sweepCall != 0 {
				t.Errorf("swept %d times past the reach, want 0", v.sweepCall)
			}
			if len(j.calls) != 0 {
				t.Errorf("journaled %v, want nothing", j.states())
			}
			// The refusal names what it was measured against.
			if got.Reach.Surface != tc.reach.Surface {
				t.Errorf("Reach.Surface = %q, want %q", got.Reach.Surface, tc.reach.Surface)
			}
		})
	}

	// The positive control: inside the reach, the same fixture proves absence.
	// Without it every assertion above is satisfied by a gate that refuses
	// always.
	t.Run("inside the reach", func(t *testing.T) {
		t.Parallel()

		horizon := horizonOf(at, testPolicy())
		res, _, r, v := newResolver(t, engineTimeFor(horizon), testPolicy())
		o := dispatchableOrder(t)
		resolvableAt(t, r, o, domain.StateDispatchAuthorized, anchorAt)
		v.status = absentVenue(publishedReach())
		v.sweeps = []ports.ExecutionSweep{provenSweep(horizon)}

		got, err := res.Resolve(o.ClientOrderID)
		if err != nil {
			t.Fatalf("Resolve = %v, want nil", err)
		}
		if got.State != domain.StateAbsentAfterFinality {
			t.Errorf("Resolve = %s/%s/%s, want resolved/absent_after_finality", got.Outcome, got.State, got.Reason)
		}
	})

	// And the configured assumption standing in for an unpublished retention,
	// which is §53c's ruling: the carrier is forced, and supplying it works.
	t.Run("unpublished with a configured assumption", func(t *testing.T) {
		t.Parallel()

		policy := testPolicy()
		policy.AssumedReach = map[application.SurfaceKey]time.Duration{
			{Venue: perp().Venue, Product: perp().Product}: 24 * time.Hour,
		}
		horizon := horizonOf(at, policy)
		res, _, r, v := newResolver(t, engineTimeFor(horizon), policy)
		o := dispatchableOrder(t)
		resolvableAt(t, r, o, domain.StateDispatchAuthorized, anchorAt)
		v.status = absentVenue(ports.SurfaceReach{Surface: "order_status"})
		v.sweeps = []ports.ExecutionSweep{provenSweep(horizon)}

		got, err := res.Resolve(o.ClientOrderID)
		if err != nil {
			t.Fatalf("Resolve = %v, want nil", err)
		}
		if got.State != domain.StateAbsentAfterFinality {
			t.Errorf("Resolve = %s/%s/%s, want resolved/absent_after_finality", got.Outcome, got.State, got.Reason)
		}
	})
}

// TestResolveMeasuresTheCliffFromTheAnchorsEarliestEdge is gate 4's band-edge
// test, and it exists because the battery found its absence.
//
// TestResolveMeasuresTheHorizonFromTheAnchorsLatestEdge is the mirror of this
// one, and it was written; this one was not. §5.3's two time bounds read
// OPPOSITE edges of the same band, so a suite that pins one edge and leaves the
// other unpinned has checked half a rule — and the fixture header above claimed
// the pair while citing a single test. Two mutants lived in the gap:
//
//   - the cliff computed from the anchor's LATEST edge, which is one band-width
//     LATER, so an order whose absence is no longer provable reads as provable;
//   - `Passed(cliff) == CertainlyPassed` in place of `!= CertainlyNotPassed`,
//     which admits the Uncertain answer as a proof.
//
// Both differ from the intended rule only while the clock's band contains the
// cliff — a window exactly one band wide, which every other fixture here sits
// hours away from. This is the repo's mirror-rule second axis again: the fix
// (place the clock INSIDE the band) was applied to gate 1 and not carried to
// gate 4.
func TestResolveMeasuresTheCliffFromTheAnchorsEarliestEdge(t *testing.T) {
	t.Parallel()

	anchorAt := testNow
	at, err := clock.NewServerTime(anchorAt, testUncertainty)
	if err != nil {
		t.Fatalf("NewServerTime: %v", err)
	}
	reach := publishedReach()

	// Computed the way the rule computes it — from the EARLIEST edge and the
	// SHORTER retention — and used only to place the clock, never to assert.
	cliff := at.Earliest().Add(reach.UnfilledRetention)

	// The band straddles the cliff: both readings are consistent with what we
	// know, and §5.3's terminal is PROVED absence.
	t.Run("straddling the cliff", func(t *testing.T) {
		t.Parallel()

		res, j, r, v := newResolver(t, engineTimeStraddling(cliff), testPolicy())
		o := dispatchableOrder(t)
		resolvableAt(t, r, o, domain.StateDispatchAuthorized, anchorAt)
		v.status = absentVenue(reach)
		v.sweeps = []ports.ExecutionSweep{provenSweep(horizonOf(at, testPolicy()))}

		got, err := res.Resolve(o.ClientOrderID)
		if err != nil {
			t.Fatalf("Resolve = %v, want nil", err)
		}
		if got.Reason != application.ReasonReachExceeded {
			t.Fatalf("Resolve = %s/%s/%s, want unproved/reach_exceeded — an absence may not be proved from a band that CONTAINS the retention cliff",
				got.Outcome, got.State, got.Reason)
		}
		if v.sweepCall != 0 {
			t.Errorf("swept %d times, want 0 — gate 4 refuses before the sweep is paid for", v.sweepCall)
		}
		if len(j.calls) != 0 {
			t.Errorf("journaled %v, want nothing", j.states())
		}
	})

	// One nanosecond earlier the whole band is short of the cliff, and the same
	// fixture proves. Without this the assertion above is satisfied by a gate
	// that refuses always — and it is one band-width away, so a rule reading the
	// wrong edge cannot pass both.
	t.Run("the whole band short of the cliff", func(t *testing.T) {
		t.Parallel()

		res, _, r, v := newResolver(t, engineTimeJustBefore(cliff), testPolicy())
		o := dispatchableOrder(t)
		resolvableAt(t, r, o, domain.StateDispatchAuthorized, anchorAt)
		v.status = absentVenue(reach)
		v.sweeps = []ports.ExecutionSweep{provenSweep(horizonOf(at, testPolicy()))}

		got, err := res.Resolve(o.ClientOrderID)
		if err != nil {
			t.Fatalf("Resolve = %v, want nil", err)
		}
		if got.State != domain.StateAbsentAfterFinality {
			t.Fatalf("Resolve = %s/%s/%s, want resolved/absent_after_finality", got.Outcome, got.State, got.Reason)
		}
	})
}

// Gate 3's coverage half. A sweep that stopped short proves nothing about the
// window it did not reach, and SM §2 step 3 collapses a possible-delta interval
// on exactly this watermark.
func TestResolveRefusesAbsenceOnASweepThatStoppedShort(t *testing.T) {
	t.Parallel()

	anchorAt := testNow
	at, err := clock.NewServerTime(anchorAt, testUncertainty)
	if err != nil {
		t.Fatalf("NewServerTime: %v", err)
	}
	horizon := horizonOf(at, testPolicy())

	res, j, r, v := newResolver(t, engineTimeFor(horizon), testPolicy())
	o := dispatchableOrder(t)
	resolvableAt(t, r, o, domain.StateDispatchAuthorized, anchorAt)
	v.status = absentVenue(publishedReach())
	// One nanosecond short. The smallest fixture that can tell `Before` from
	// `!After`, and a fixture short by minutes could not.
	v.sweeps = []ports.ExecutionSweep{provenSweep(horizon.Add(-time.Nanosecond))}

	got, err := res.Resolve(o.ClientOrderID)
	if err != nil {
		t.Fatalf("Resolve = %v, want nil", err)
	}
	if got.Reason != application.ReasonSweepIncomplete {
		t.Fatalf("Resolve = %s/%s, want unproved/sweep_incomplete", got.Outcome, got.Reason)
	}
	if len(j.calls) != 0 {
		t.Errorf("journaled %v, want nothing", j.states())
	}
}

// Gate 3's attribution half, and the fixtures are the checklist's item 4: more
// than one row, with the ambiguous one FIRST.
func TestResolveAttributesEverySweepRowBeforeDeciding(t *testing.T) {
	t.Parallel()

	anchorAt := testNow
	at, err := clock.NewServerTime(anchorAt, testUncertainty)
	if err != nil {
		t.Fatalf("NewServerTime: %v", err)
	}
	horizon := horizonOf(at, testPolicy())
	mine := mintedID(t, 1)
	other := mintedID(t, 2)

	cases := map[string]struct {
		rows       []ports.VenueExecution
		wantReason application.UnprovedReason
		wantState  domain.OrderState
		wantOurs   int
	}{
		"nothing at all": {
			wantState: domain.StateAbsentAfterFinality,
		},
		"only other orders, each named": {
			rows:      []ports.VenueExecution{row(other, "e-1"), row(other, "e-2")},
			wantState: domain.StateAbsentAfterFinality,
		},
		"an unnamed row makes the sweep inconclusive": {
			rows:       []ports.VenueExecution{row(other, "e-1"), row("", "e-2")},
			wantReason: application.ReasonSweepInconclusive,
		},
		// The ordering claim: the ambiguous row comes FIRST, so a function that
		// returned on the first unattributable row would answer inconclusive and
		// hide an execution that is real exposure.
		"an unnamed row BEFORE ours does not mask it": {
			rows:       []ports.VenueExecution{row("", "e-1"), row(mine, "e-2")},
			wantReason: application.ReasonExecutionsFound,
			wantOurs:   1,
		},
		"two of ours are both carried": {
			rows:       []ports.VenueExecution{row(mine, "e-1"), row(other, "e-2"), row(mine, "e-3")},
			wantReason: application.ReasonExecutionsFound,
			wantOurs:   2,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			res, j, r, v := newResolver(t, engineTimeFor(horizon), testPolicy())
			o := dispatchableOrder(t)
			resolvableAt(t, r, o, domain.StateDispatchAuthorized, anchorAt)
			v.status = absentVenue(publishedReach())
			v.sweeps = []ports.ExecutionSweep{provenSweep(horizon, tc.rows...)}

			got, err := res.Resolve(o.ClientOrderID)
			if err != nil {
				t.Fatalf("Resolve = %v, want nil", err)
			}
			if tc.wantState != "" {
				if got.Outcome != application.OutcomeResolved || got.State != tc.wantState {
					t.Fatalf("Resolve = %s/%s/%s, want resolved/%s", got.Outcome, got.State, got.Reason, tc.wantState)
				}
				return
			}
			if got.Outcome != application.OutcomeUnproved || got.Reason != tc.wantReason {
				t.Fatalf("Resolve = %s/%s, want unproved/%s", got.Outcome, got.Reason, tc.wantReason)
			}
			if len(got.Executions) != tc.wantOurs {
				t.Errorf("carried %d executions, want %d", len(got.Executions), tc.wantOurs)
			}
			if len(j.calls) != 0 {
				t.Errorf("journaled %v, want nothing", j.states())
			}
		})
	}
}

// A present order's state is recorded, and a TERMINAL one needs the sweep first
// — SM §2 calls the four-step contract the one canonical boundary, "never a
// weaker cancel-ack-only variant".
//
// The non-terminal case asserts the sweep was NOT run, which is the only
// evidence that the exemption is deliberate rather than a path that happens to
// skip it.
func TestResolveRecordsAPresentOrdersState(t *testing.T) {
	t.Parallel()

	anchorAt := testNow
	at, err := clock.NewServerTime(anchorAt, testUncertainty)
	if err != nil {
		t.Fatalf("NewServerTime: %v", err)
	}
	horizon := horizonOf(at, testPolicy())

	cases := map[string]struct {
		venueState domain.OrderState
		wantSweeps int
	}{
		"live at the venue":         {venueState: domain.StateAcknowledged, wantSweeps: 0},
		"partially filled":          {venueState: domain.StatePartiallyFilled, wantSweeps: 0},
		"terminated unfilled":       {venueState: domain.StateCanceled, wantSweeps: 1},
		"filled":                    {venueState: domain.StateFilled, wantSweeps: 1},
		"rejected with an explicit": {venueState: domain.StateRejected, wantSweeps: 1},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			res, j, r, v := newResolver(t, engineTimeFor(horizon), testPolicy())
			o := dispatchableOrder(t)
			resolvableAt(t, r, o, domain.StateDispatchAuthorized, anchorAt)
			v.status = ports.VenueOrder{
				Presence: ports.VenueHasOrder,
				Reach:    publishedReach(),
				State:    tc.venueState,
			}
			v.sweeps = []ports.ExecutionSweep{provenSweep(horizon)}

			got, err := res.Resolve(o.ClientOrderID)
			if err != nil {
				t.Fatalf("Resolve = %v, want nil", err)
			}
			if got.Outcome != application.OutcomeResolved || got.State != tc.venueState {
				t.Fatalf("Resolve = %s/%s, want resolved/%s", got.Outcome, got.State, tc.venueState)
			}
			if v.sweepCall != tc.wantSweeps {
				t.Errorf("swept %d times, want %d", v.sweepCall, tc.wantSweeps)
			}
			if states := j.states(); len(states) != 1 || states[0] != tc.venueState {
				t.Errorf("journaled %v, want [%s]", states, tc.venueState)
			}
		})
	}
}

// A terminal reading is NOT enough on its own: without the sweep's watermark,
// SM §2 step 3 has not run and the order's interval has not collapsed.
func TestResolveWithholdsAPresentTerminalUntilTheSweepReachesTheHorizon(t *testing.T) {
	t.Parallel()

	anchorAt := testNow
	at, err := clock.NewServerTime(anchorAt, testUncertainty)
	if err != nil {
		t.Fatalf("NewServerTime: %v", err)
	}
	horizon := horizonOf(at, testPolicy())

	res, j, r, v := newResolver(t, engineTimeFor(horizon), testPolicy())
	o := dispatchableOrder(t)
	resolvableAt(t, r, o, domain.StateDispatchAuthorized, anchorAt)
	v.status = ports.VenueOrder{
		Presence: ports.VenueHasOrder, Reach: publishedReach(), State: domain.StateCanceled,
	}
	v.sweeps = []ports.ExecutionSweep{provenSweep(horizon.Add(-time.Nanosecond))}

	got, err := res.Resolve(o.ClientOrderID)
	if err != nil {
		t.Fatalf("Resolve = %v, want nil", err)
	}
	if got.Reason != application.ReasonSweepIncomplete {
		t.Fatalf("Resolve = %s/%s, want unproved/sweep_incomplete", got.Outcome, got.Reason)
	}
	if len(j.calls) != 0 {
		t.Errorf("journaled %v, want nothing — a terminal state with no proven sweep is not finality", j.states())
	}
}

// §5.3 forbids reading an unanswered query as an absence, and the two must leave
// by different returns. This is the assertion that the error path writes nothing
// at all.
func TestResolvePassesAQueryFailureThroughAndWritesNothing(t *testing.T) {
	t.Parallel()

	failure := errors.New("rate limited")
	anchorAt := testNow
	at, err := clock.NewServerTime(anchorAt, testUncertainty)
	if err != nil {
		t.Fatalf("NewServerTime: %v", err)
	}

	res, j, r, v := newResolver(t, engineTimeFor(horizonOf(at, testPolicy())), testPolicy())
	o := dispatchableOrder(t)
	resolvableAt(t, r, o, domain.StateDispatchAuthorized, anchorAt)
	v.statusErr = failure

	got, err := res.Resolve(o.ClientOrderID)
	if !errors.Is(err, failure) {
		t.Fatalf("Resolve = %v, want the query failure", err)
	}
	if got.Outcome != "" {
		t.Errorf("Resolve returned outcome %q alongside an error, want the zero Resolution", got.Outcome)
	}
	if len(j.calls) != 0 || len(r.applied) != 0 {
		t.Errorf("wrote journal=%v applied=%d, want nothing", j.states(), len(r.applied))
	}
}

// Legality is established BEFORE the journal, because the journal is append-only
// and §7.5 replays it: a transition the projection would refuse must never reach
// the log.
//
// The venue answer used here is one the REAL adapter cannot produce — its
// translation maps onto five states, all of them legal from this source — so
// what this guards is the PORT, which any implementation may satisfy. Stated
// rather than left for a reader to wonder whether the branch is reachable.
func TestResolveEstablishesLegalityBeforeJournaling(t *testing.T) {
	t.Parallel()

	anchorAt := testNow
	at, err := clock.NewServerTime(anchorAt, testUncertainty)
	if err != nil {
		t.Fatalf("NewServerTime: %v", err)
	}

	res, j, r, v := newResolver(t, engineTimeFor(horizonOf(at, testPolicy())), testPolicy())
	o := dispatchableOrder(t)
	resolvableAt(t, r, o, domain.StateDispatchAuthorized, anchorAt)
	v.status = ports.VenueOrder{
		Presence: ports.VenueHasOrder, Reach: publishedReach(), State: domain.StateCancelPending,
	}

	if _, err := res.Resolve(o.ClientOrderID); !errors.Is(err, domain.ErrTransitionForbidden) {
		t.Fatalf("Resolve = %v, want ErrTransitionForbidden", err)
	}
	if len(j.calls) != 0 {
		t.Errorf("journaled %v before checking legality, want nothing", j.states())
	}
}

// A journal failure does NOT stop the apply, and the asymmetry is the opposite
// of Dispatch's second apply. There the record was left at a state from which
// every target was forbidden, so the attempt could only fail; here the record is
// untouched and the transition is still legal, so declining to try would hold
// §6.7's classifier on a stale state to punish a journal already reported.
func TestResolveAppliesTheStateEvenWhenTheJournalFailed(t *testing.T) {
	t.Parallel()

	anchorAt := testNow
	at, err := clock.NewServerTime(anchorAt, testUncertainty)
	if err != nil {
		t.Fatalf("NewServerTime: %v", err)
	}
	horizon := horizonOf(at, testPolicy())

	res, j, r, v := newResolver(t, engineTimeFor(horizon), testPolicy())
	o := dispatchableOrder(t)
	resolvableAt(t, r, o, domain.StateDispatchAuthorized, anchorAt)
	v.status = absentVenue(publishedReach())
	v.sweeps = []ports.ExecutionSweep{provenSweep(horizon)}
	j.failOn[domain.StateAbsentAfterFinality] = errors.New("disk full")

	got, err := res.Resolve(o.ClientOrderID)
	if err != nil {
		t.Fatalf("Resolve = %v, want nil", err)
	}
	if got.Journaled {
		t.Errorf("Journaled = true, want false")
	}
	if !got.Applied {
		t.Errorf("Applied = false, want true — the record is untouched and the edge is still legal")
	}
	if rec := r.records[o.ClientOrderID]; rec.State != domain.StateAbsentAfterFinality {
		t.Errorf("record is at %s, want %s", rec.State, domain.StateAbsentAfterFinality)
	}
}

// venueRow builds one execution the way Binance reports it: NO client order id,
// the venue's own orderId only. It is the row shape the persisted-ack index
// exists for.
func venueRow(venueOrderID, executionID string) ports.VenueExecution {
	return ports.VenueExecution{ExecutionID: ids.ExecutionID(executionID), VenueOrderID: venueOrderID}
}

// plantAcked plants a sibling order on an instrument with a persisted venue id
// — a Filled terminal, which is the realistic sibling: the orders that leave
// rows in a sweep window are exactly the ones that executed.
func plantAcked(t *testing.T, r *fakeOrders, leg int, instrument ids.VenueInstrument, venueOrderID string) ids.ClientOrderID {
	t.Helper()
	id := mintedID(t, leg)
	o := dispatchableOrder(t)
	o.ClientOrderID = id
	o.Instrument = instrument
	r.records[id] = ports.OrderRecord{
		Order: o, State: domain.StateFilled, Version: 3, VenueOrderID: venueOrderID,
	}
	return id
}

// TestResolveRulesOutRowsByThePersistedAckIndex is task #174's headline: on
// Binance a swept row names no client order id, so before the index existed
// ANY row in the window was ReasonSweepInconclusive — a permanent refusal.
// With the siblings' acks persisted, a row is ruled out when its venue id
// belongs to another of OUR orders on the instrument, claimed when it is this
// order's own persisted id, and still refused when nobody is recorded under
// it.
func TestResolveRulesOutRowsByThePersistedAckIndex(t *testing.T) {
	t.Parallel()

	anchorAt := testNow
	at, err := clock.NewServerTime(anchorAt, testUncertainty)
	if err != nil {
		t.Fatalf("NewServerTime: %v", err)
	}
	horizon := horizonOf(at, testPolicy())

	cases := map[string]struct {
		ownVenueID string // persisted on THIS order's record when non-empty
		siblingOn  ids.VenueInstrument
		rows       []ports.VenueExecution
		wantReason application.UnprovedReason
		wantState  domain.OrderState
		wantOurs   int
	}{
		// The behaviour change: every row in the window belongs to an
		// identified sibling, so a Binance absence CONCLUDES.
		"every row owned by a sibling proves the absence": {
			siblingOn: perp(),
			rows:      []ports.VenueExecution{venueRow("201", "e-1"), venueRow("202", "e-2")},
			wantState: domain.StateAbsentAfterFinality,
		},
		"a row nobody is recorded under still refuses": {
			siblingOn:  perp(),
			rows:       []ports.VenueExecution{venueRow("201", "e-1"), venueRow("999", "e-2")},
			wantReason: application.ReasonSweepInconclusive,
		},
		// The banked venue fact is per SYMBOL, so a sibling's number on the
		// OTHER instrument is a different namespace and rules out nothing —
		// the fixture that separates instrument-keyed from globally-keyed,
		// in the direction that would otherwise prove a false absence.
		"a sibling's number on another instrument rules out nothing": {
			siblingOn:  spot(),
			rows:       []ports.VenueExecution{venueRow("201", "e-1")},
			wantReason: application.ReasonSweepInconclusive,
		},
		// This order's OWN persisted ack claims the row — exposure, surfaced,
		// with the unknown row FIRST so the ours-wins-over-unknown ordering
		// covers the new arm too.
		"a row matching this orders own persisted id is exposure": {
			ownVenueID: "200",
			siblingOn:  perp(),
			rows:       []ports.VenueExecution{venueRow("999", "e-1"), venueRow("200", "e-2")},
			wantReason: application.ReasonExecutionsFound,
			wantOurs:   1,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			res, j, r, v := newResolver(t, engineTimeFor(horizon), testPolicy())
			o := dispatchableOrder(t)
			resolvableAt(t, r, o, domain.StateDispatchAuthorized, anchorAt)
			if tc.ownVenueID != "" {
				rec := r.records[o.ClientOrderID]
				rec.VenueOrderID = tc.ownVenueID
				r.records[o.ClientOrderID] = rec
			}
			// The siblings hold 201 and 202; the placement instrument is the
			// case's own.
			plantAcked(t, r, 2, tc.siblingOn, "201")
			plantAcked(t, r, 3, tc.siblingOn, "202")

			v.status = absentVenue(publishedReach())
			v.sweeps = []ports.ExecutionSweep{provenSweep(horizon, tc.rows...)}

			got, err := res.Resolve(o.ClientOrderID)
			if err != nil {
				t.Fatalf("Resolve = %v, want nil", err)
			}
			if tc.wantState != "" {
				if got.Outcome != application.OutcomeResolved || got.State != tc.wantState {
					t.Fatalf("Resolve = %s/%s/%s, want resolved/%s", got.Outcome, got.State, got.Reason, tc.wantState)
				}
				return
			}
			if got.Outcome != application.OutcomeUnproved || got.Reason != tc.wantReason {
				t.Fatalf("Resolve = %s/%s, want unproved/%s", got.Outcome, got.Reason, tc.wantReason)
			}
			if len(got.Executions) != tc.wantOurs {
				t.Errorf("carried %d executions, want %d", len(got.Executions), tc.wantOurs)
			}
			if len(j.calls) != 0 {
				t.Errorf("journaled %v, want nothing", j.states())
			}
		})
	}
}

// TestResolveRefusesWhenTheIndexCannotAnswer: an index error is a refusal of
// the whole resolution (err != nil, nothing written), never a degrade to an
// empty map — an empty map is a real answer that quietly waves off every row a
// sibling could explain, and the degraded mode must not be selectable by a
// store hiccup.
func TestResolveRefusesWhenTheIndexCannotAnswer(t *testing.T) {
	t.Parallel()

	anchorAt := testNow
	at, err := clock.NewServerTime(anchorAt, testUncertainty)
	if err != nil {
		t.Fatalf("NewServerTime: %v", err)
	}
	horizon := horizonOf(at, testPolicy())

	res, j, r, v := newResolver(t, engineTimeFor(horizon), testPolicy())
	o := dispatchableOrder(t)
	resolvableAt(t, r, o, domain.StateDispatchAuthorized, anchorAt)
	r.failVenueIDs = errors.New("store sulking")
	v.status = absentVenue(publishedReach())
	v.sweeps = []ports.ExecutionSweep{provenSweep(horizon)}

	if _, err := res.Resolve(o.ClientOrderID); err == nil {
		t.Fatal("Resolve = nil, want the index failure passed through")
	}
	if len(j.calls) != 0 {
		t.Errorf("journaled %v, want nothing — err != nil means nothing was written", j.states())
	}
}
