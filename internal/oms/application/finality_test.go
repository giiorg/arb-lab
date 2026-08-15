package application_test

import (
	"errors"
	"testing"

	"github.com/giiorg/arb-lab/internal/oms/application"
	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// This file is the composition the defect was invisible to.
//
// Every other test in this package exercises ONE use case over doubles, and both
// halves were individually right: the dispatcher classified a successful Place as
// StateSubmitted, the join fetched the venue's reading, and neither was wrong.
// What nothing exercised was an order travelling from a real Place to a terminal,
// because no consumer required it — `arblab exercise run` treats StateSubmitted as
// its success state and exits there.
//
// So the fixtures here MUST start at Dispatch. A test that plants a terminal
// record starts on the far side of the bug and would have passed against the tree
// as it stood before #242, which is the only property that makes this file
// evidence rather than coverage.
//
// # What each walk would need in order to pass with the advance deleted
//
// Stated per walk rather than assumed, because "the walk ran" and "the walk
// reached the transition" are the executions-not-coverage pair this repo has
// recorded eleven instances of:
//
//  1. The table: it would need the projection to ALREADY hold the venue's reading
//     when the join is called. It holds StateSubmitted — place() asserts that —
//     and no row's venue reading is Submitted, so every row's want differs from
//     the state a no-op leaves behind. The journal half needs the same fact one
//     store over: the dispatch path writes DispatchAuthorized and Submitted and
//     nothing else, so `recorded(id, want)` cannot be satisfied by a record the
//     dispatch already wrote.
//  2. The unresolved walk asserts the OPPOSITE direction, so a deleted advance
//     passes it by construction. It is not evidence about the write path and does
//     not claim to be: its subject is the refusal, and its positive control is
//     walk 1 — same harness, same Place, an answering venue.
//  3. The round trip: it would need the rebuilt projection to reach StateFilled
//     from a log holding only the dispatch path's records. Replay applies through
//     ApplyState, which cannot invent a state no record names, so a journal
//     without the advance's record replays to Submitted.

// finalityHarness holds one deployment's two use cases over SHARED stores.
//
// Shared is the whole point: the dispatcher's journal and projection are the
// joiner's, so the record the join advances is the record the Place created, with
// the version the dispatch left on it. Two sets of fakes would make this an
// assertion about two independent systems that happen to agree.
type finalityHarness struct {
	dispatch *application.Dispatcher
	join     *application.Joiner
	journal  *fakeJournal
	placer   *fakePlacer
	orders   *fakeOrders
	venue    *fakeJoinVenue

	// placed is how many orders this harness has dispatched, which is what the
	// placer's own count is checked against — a walk that sent one order twice
	// and a walk that sent two orders once are the same total.
	placed int
}

func newFinalityHarness(t *testing.T) *finalityHarness {
	t.Helper()
	j := newFakeJournal()
	p := &fakePlacer{watch: j, ack: ports.Ack{VenueOrderID: "v-1"}}
	r := newFakeOrders()
	v := newFakeJoinVenue()
	clk := testClock(t)

	d, err := application.NewDispatcher(j, p, r, clk, application.NewDispatchAuthority(),
		testFilters(t), testFilterMaxAge)
	if err != nil {
		t.Fatalf("NewDispatcher: %v", err)
	}
	joiner, err := application.NewJoiner(v, newFakePositions(), r, j, clk, joinAccount, application.SpotLeg{})
	if err != nil {
		t.Fatalf("NewJoiner: %v", err)
	}
	return &finalityHarness{dispatch: d, join: joiner, journal: j, placer: p, orders: r, venue: v}
}

// place runs the real dispatch path and asserts it landed where the system
// actually leaves a successful Place. That assertion is not decoration: it is the
// premise every walk below rests on, and if it ever changes these tests are
// measuring something else.
//
// # Both halves of the planning path run, and the projection half is ApplyPlanned
// # rather than plant
//
// plant writes a record straight into the fake's map, and the record it writes
// carries the ZERO Quantity — untagged — where ApplyPlanned writes a zero OF THE
// ORDERED ASSET. Every dispatch test survives the difference because none of them
// does arithmetic on the cumulative; the join does, on every order it walks
// (`rec.Cumulative.Cmp(status.Executed)`), and an untagged operand fails there.
// So the planted shortcut would have turned the three zero-fill rows below into
// tests of fixedpoint's asset guard. ApplyPlanned is the write the real planning
// path makes, which makes this the shorter statement as well as the correct one.
//
// The AppendPlanned is the LOG's half, and §6 writes an order twice. Without it
// the replay round-trip below would meet a state record for an order the rebuilt
// projection has never heard of, and fail with ErrOrderUnknown — a red bar about
// the fixture rather than about the walk.
func (h *finalityHarness) place(t *testing.T) domain.Order {
	t.Helper()
	return h.placeLeg(t, 1, "v-1")
}

// placeLeg is place for a named leg of the fixture trade, with its own venue id.
//
// The id is per-leg because the ack's is: ApplyAck refuses a venue order id that
// already names another order on the same instrument, so a second order taking
// the first's id would dispatch with AckConflict set and no venue id in the
// projection — a fixture broken in a way every assertion below would survive.
func (h *finalityHarness) placeLeg(t *testing.T, leg int, venueOrderID string) domain.Order {
	t.Helper()
	o := dispatchableOrder(t)
	o.ClientOrderID = mintedID(t, leg)
	if _, err := h.orders.ApplyPlanned(o); err != nil {
		t.Fatalf("ApplyPlanned: %v", err)
	}
	if err := h.journal.AppendPlanned(o); err != nil {
		t.Fatalf("AppendPlanned: %v", err)
	}
	h.placer.ack = ports.Ack{VenueOrderID: venueOrderID}

	out, err := h.dispatch.Dispatch(o.ClientOrderID, 1, ports.UrgencyOpening, nil)
	if err != nil {
		t.Fatalf("Dispatch = %v, want nil", err)
	}
	h.placed++
	if h.placer.calls != h.placed {
		t.Fatalf("Place called %d times over %d dispatches, want one apiece", h.placer.calls, h.placed)
	}
	if out.State != domain.StateSubmitted {
		t.Fatalf("a successful Place classified as %s, want submitted", out.State)
	}
	if !out.Applied {
		t.Fatalf("Dispatch reported Applied=false; the walk needs the projection")
	}
	if out.AckConflict {
		t.Fatalf("Dispatch reported an ack conflict; the fixture's venue ids collide")
	}
	if got := stateOf(t, h.orders, o.ClientOrderID); got != domain.StateSubmitted {
		t.Fatalf("projection holds %s after Place, want submitted", got)
	}
	return o
}

// TestAPlacedOrderReachesEveryTerminalTheVenueCanReport is the requirement, as a
// table over the FIVE readings driven.toDomainState can produce.
//
// The five are not a sample: engine/venue normalizes both venues' vocabularies to
// Open / PartiallyFilled / Filled / Terminated / Rejected, and the ACL maps them
// onto exactly these SM §1 members. A sixth row would be a claim about an adapter
// that does not exist.
//
// Fill quantities differ per row so that "advances to the venue's state" and
// "advances to a state derived from the fills" are different answers, and the
// partial row's remainder is what makes the canceled-with-fills case real rather
// than a relabelled zero-fill one.
func TestAPlacedOrderReachesEveryTerminalTheVenueCanReport(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		venueState domain.OrderState
		executed   string
		fill       string // "" = the sweep returns nothing
		wantFills  int
	}{
		{
			// The full-fill IOC: the ordinary success this whole slice exists for.
			name: "full fill", venueState: domain.StateFilled,
			executed: "0.5", fill: "0.5", wantFills: 1,
		},
		{
			// A marketable IOC that took part of the book; the venue cancels the
			// remainder with no request of ours (SM §1's venue-initiated
			// termination row). This is the SHAPE #236's hedge is sized against.
			name: "partial fill then canceled", venueState: domain.StateCanceled,
			executed: "0.3", fill: "0.3", wantFills: 1,
		},
		{
			// An IOC that crossed nothing. A real outcome on a thin book, and the
			// one where a zero cumulative is the TRUTH rather than a reading taken
			// too early — which is only distinguishable because the state is
			// terminal.
			name: "zero fill canceled", venueState: domain.StateCanceled,
			executed: "0", fill: "", wantFills: 0,
		},
		{
			// An explicit venue rejection. Distinct evidence class from proved
			// absence (FLAG-2), and a target Submitted could already reach before
			// this slice widened the row — Acknowledged is the other one OF THE
			// READINGS THIS TABLE DRIVES, and the scoping is the claim: pre-slice
			// successors(Submitted) also held SubmissionUnknown, which no §4 ACL
			// reading produces, so it is legal and undrivable from here. Worth
			// naming because the defect was never "no legal edge"
			// alone. Two of these five rows were legal the whole time and no order
			// took either, because nothing called the write.
			name: "venue rejection", venueState: domain.StateRejected,
			executed: "0", fill: "", wantFills: 0,
		},
		{
			// Still live. The NON-terminal row, and it is what stops this table
			// asserting "the join writes a terminal" when the claim is "the join
			// writes the venue's reading".
			name: "still acknowledged", venueState: domain.StateAcknowledged,
			executed: "0", fill: "", wantFills: 0,
		},
	}

	// The table's own collapse guard, asserted rather than eyeballed once with a
	// -v run. Two rows share StateCanceled on purpose, and what makes them two
	// rows rather than one repeated is the fill count; every other pair has to
	// differ in the state itself. The Submitted clause is the one that keeps the
	// whole table from going vacuous: a row whose venue reading equalled the state
	// a successful Place leaves behind would pass with the advance deleted.
	for i, a := range cases {
		if a.venueState == domain.StateSubmitted {
			t.Errorf("row %q reads back the state Place already leaves; it cannot see an advance", a.name)
		}
		for _, b := range cases[i+1:] {
			if a.venueState == b.venueState && a.wantFills == b.wantFills {
				t.Errorf("rows %q and %q agree on state AND fills — one of them measures nothing new",
					a.name, b.name)
			}
		}
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			h := newFinalityHarness(t)
			o := h.place(t)

			// A SECOND dispatched order, which the venue does not hold. It is what
			// makes three separate readings here mean what they say, and none of
			// the three is about absence:
			//
			//   - Advanced counts WRITES. Over one order, "one order advanced" and
			//     "one write happened" are the same number, so the field's own
			//     doc comment ("it counts writes, not orders") is untested.
			//   - Orders counts the walk. Two, against one advance.
			//   - Every per-id journal reading below has something to exclude. With
			//     one order in the log, a helper whose id filter did nothing
			//     returns the same slice.
			//
			// Absent rather than a second advancing order because the join must end
			// with ONE write for the counts to separate, and §5.3's absence is the
			// realistic way a dispatched order contributes no advance.
			other := h.placeLeg(t, 0, "v-2")

			h.venue.status[o.ClientOrderID] = venueSays(t, tc.venueState, "v-1", tc.executed)
			if tc.fill != "" {
				h.venue.sweeps[o.Instrument] = ports.ExecutionSweep{
					Executions: []ports.VenueExecution{
						execution(t, "e-1", o.ClientOrderID, "v-1", tc.fill),
					},
				}
			}

			report, err := h.join.Join([]ids.ClientOrderID{o.ClientOrderID, other.ClientOrderID})
			if err != nil {
				t.Fatalf("Join = %v, want nil", err)
			}
			if report.Orders != 2 {
				t.Errorf("Orders = %d, want 2", report.Orders)
			}
			if report.Advanced != 1 {
				t.Errorf("Advanced = %d, want 1 — two orders walked, one write", report.Advanced)
			}
			if report.Fills != tc.wantFills {
				t.Errorf("Fills = %d, want %d", report.Fills, tc.wantFills)
			}
			if len(report.Contradicted) != 0 || len(report.Unattributed) != 0 {
				t.Errorf("Contradicted=%v Unattributed=%v, want neither",
					report.Contradicted, report.Unattributed)
			}
			if len(report.Absent) != 1 || report.Absent[0] != other.ClientOrderID {
				t.Errorf("Absent = %v, want just %q", report.Absent, other.ClientOrderID)
			}
			if got := stateOf(t, h.orders, o.ClientOrderID); got != tc.venueState {
				t.Errorf("projection holds %s, want %s", got, tc.venueState)
			}
			// The order the venue does not hold did NOT move: an absence is §5.3's
			// input, not a conclusion, and a join that advanced it would be
			// concluding finality from a not-found.
			if got := stateOf(t, h.orders, other.ClientOrderID); got != domain.StateSubmitted {
				t.Errorf("the absent order holds %s, want submitted", got)
			}

			// The journal, separately from the projection: the log is what §7.5
			// replays, and a state applied but not journaled is lost on the next
			// boot while looking perfect on this one.
			if !h.journal.recorded(o.ClientOrderID, tc.venueState) {
				t.Errorf("journal holds %v, want a %s record for %q",
					h.journal.stateRecordsFor(o.ClientOrderID), tc.venueState, o.ClientOrderID)
			}
			// And it is the dispatch path's two records plus exactly this one, in
			// that order. The membership check above is satisfied by a join that
			// journaled the right state among several; this is what says the log
			// holds one history rather than a superset of it.
			want := []domain.OrderState{domain.StateDispatchAuthorized, domain.StateSubmitted, tc.venueState}
			if got := h.journal.stateRecordsFor(o.ClientOrderID); !equalStates(got, want) {
				t.Errorf("journal recorded %v for %q, want %v", got, o.ClientOrderID, want)
			}
			dispatchOnly := []domain.OrderState{domain.StateDispatchAuthorized, domain.StateSubmitted}
			if got := h.journal.stateRecordsFor(other.ClientOrderID); !equalStates(got, dispatchOnly) {
				t.Errorf("journal recorded %v for the absent order %q, want %v",
					got, other.ClientOrderID, dispatchOnly)
			}
		})
	}
}

// TestAPlacedOrderTheVenueWillNotResolveStaysSubmitted is the fifth case the
// record names, and it is the one whose SAFE answer is "nothing happened".
//
// §5.3 forbids reading an unanswered query as an absence, and the direction here
// is the same: a joiner that advanced on a failed Resolve would be concluding
// finality from a query that did not run. The order stays where it was, the error
// carries the venue's, and the boot posture is what stops trading.
func TestAPlacedOrderTheVenueWillNotResolveStaysSubmitted(t *testing.T) {
	t.Parallel()

	h := newFinalityHarness(t)
	o := h.place(t)
	// A second dispatched order, never joined here: the distractor that keeps the
	// per-id journal reading below a claim about THIS order rather than about a
	// log that happens to hold one.
	h.placeLeg(t, 0, "v-2")
	rateLimited := errors.New("venue: rate limited")
	h.venue.err[o.ClientOrderID] = rateLimited

	if _, err := h.join.Join([]ids.ClientOrderID{o.ClientOrderID}); !errors.Is(err, rateLimited) {
		t.Fatalf("Join = %v, want the venue's error", err)
	}
	if got := stateOf(t, h.orders, o.ClientOrderID); got != domain.StateSubmitted {
		t.Errorf("projection holds %s, want submitted — the query was not answered", got)
	}
	// The dispatch path's two records and nothing more. Spelled as the whole
	// sequence rather than as "no Filled record": an advance concluded from an
	// unanswered query would journal whatever it concluded, and a count aimed at
	// one target state would miss every other spelling of the same mistake.
	want := []domain.OrderState{domain.StateDispatchAuthorized, domain.StateSubmitted}
	if got := h.journal.stateRecordsFor(o.ClientOrderID); !equalStates(got, want) {
		t.Errorf("journal recorded %v, want %v — nothing may be journaled from a query that did not run",
			got, want)
	}
	if got := h.journal.recordCountFor(o.ClientOrderID, domain.StateFilled); got != 0 {
		t.Errorf("journal holds %d terminal records, want 0", got)
	}
}

// TestTheJournaledWalkReplaysToTheSameTerminal is the claim only a round-trip can
// make, and it is the reason each hop has to be a legal edge rather than a
// convenient one.
//
// §7.5's reconcile-only boot rebuilds STATE by replaying the log, and
// replay.State applies every record through ApplyState — which checks
// domain.CanTransition. A journal holding a transition the edge set refuses does
// not degrade: the next boot REFUSES, over a log written by this system on the
// boot before. So "the walk is legal" and "the deployment can boot again" are the
// same property, and this is the test that makes them one.
func TestTheJournaledWalkReplaysToTheSameTerminal(t *testing.T) {
	t.Parallel()

	h := newFinalityHarness(t)
	o := h.place(t)
	// A second dispatched order the venue does not hold, so the replayed log
	// carries two histories that end in DIFFERENT states. Without it the
	// unresolved-set assertion below is a negative satisfied by the degenerate
	// answer: an empty list proves nothing about which orders left the set when
	// no order could have been in it.
	other := h.placeLeg(t, 0, "v-2")
	h.venue.status[o.ClientOrderID] = venueSays(t, domain.StateFilled, "v-1", "0.5")
	h.venue.sweeps[o.Instrument] = ports.ExecutionSweep{
		Executions: []ports.VenueExecution{execution(t, "e-1", o.ClientOrderID, "v-1", "0.5")},
	}
	if _, err := h.join.Join([]ids.ClientOrderID{o.ClientOrderID, other.ClientOrderID}); err != nil {
		t.Fatalf("Join = %v, want nil", err)
	}
	live := stateOf(t, h.orders, o.ClientOrderID)
	if live != domain.StateFilled {
		t.Fatalf("live projection holds %s, want filled — the round-trip's premise", live)
	}

	// A fresh projection, rebuilt from the SCRIPT this run's records make.
	rebuilt := newFakeOrders()
	replayer, err := application.NewReplayer(&fakeReplay{entries: h.journal.replayScript()}, rebuilt)
	if err != nil {
		t.Fatalf("NewReplayer: %v", err)
	}
	recovery, err := replayer.Rebuild()
	if err != nil {
		t.Fatalf("Rebuild = %v, want nil — the log holds a transition the edge set refuses", err)
	}
	if got := stateOf(t, rebuilt, o.ClientOrderID); got != domain.StateFilled {
		t.Errorf("replayed projection holds %s, want filled", got)
	}
	// The same terminal, not merely a terminal: the two projections are the point
	// of the round trip, and asserting the literal alone would still pass if the
	// live one had drifted somewhere else and the fixture were updated to match.
	if got := stateOf(t, rebuilt, o.ClientOrderID); got != live {
		t.Errorf("replayed projection holds %s, live projection holds %s", got, live)
	}
	// The order that reached a terminal has LEFT the unresolved set, and the one
	// still at Submitted has not. That pair is the consequence the boot actually
	// reads: before this slice every placed order stayed in the second group for
	// ever, with nothing reporting an error. Stated as the exact set rather than
	// as an emptiness, because an empty list is also what a rebuild that produced
	// nothing returns.
	wantUnresolved := []ids.ClientOrderID{other.ClientOrderID}
	if len(recovery.Unresolved) != 1 || recovery.Unresolved[0] != wantUnresolved[0] {
		t.Errorf("Unresolved = %v, want %v — the terminal leaves the set, the submitted one stays",
			recovery.Unresolved, wantUnresolved)
	}
	if recovery.Orders != 2 {
		t.Errorf("Orders = %d, want 2 — the replay rebuilt both histories", recovery.Orders)
	}
}
