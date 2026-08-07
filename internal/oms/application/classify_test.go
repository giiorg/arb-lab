package application_test

import (
	"errors"
	"testing"

	"github.com/giiorg/arb-lab/internal/oms/application"
	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// fakeGroupOrders is the classifier's one dependency.
//
// It does NOT filter by group, deliberately. The real adapter does, and a fake
// that repeated the filter could never produce the record set the classifier's
// own membership check exists for — which is the case where a port
// implementation this package does not own answers with an order from another
// group. A fake that only ever produced correct input would make that check
// unreachable and therefore untested.
type fakeGroupOrders struct {
	records []ports.OrderRecord
	err     error
	calls   int
}

func (f *fakeGroupOrders) OrdersIn(domain.HedgeGroup) ([]ports.OrderRecord, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.records, nil
}

var _ ports.HedgeGroupOrders = (*fakeGroupOrders)(nil)

func hedgeGroup() domain.HedgeGroup {
	return domain.HedgeGroup{Account: "sub-1", Venue: "binance", Symbol: "BTCUSDT"}
}

// leg builds one order record: the instrument, the direction, what was ordered,
// what has executed, and where the lifecycle has got to.
func leg(t *testing.T, n int, instrument ids.VenueInstrument, side domain.Side, ordered, executed string, state domain.OrderState) ports.OrderRecord {
	t.Helper()
	return ports.OrderRecord{
		Order: domain.Order{
			ClientOrderID: mintedID(t, n),
			Instrument:    instrument,
			Side:          side,
			Type:          domain.OrderTypeLimit,
			TimeInForce:   domain.TimeInForceIOC,
			Quantity:      qty(t, ordered),
			LimitPrice:    price(t, "50000"),
		},
		State:      state,
		Cumulative: qty(t, executed),
		Version:    1,
	}
}

func newClassifier(t *testing.T, records ...ports.OrderRecord) (*application.Classifier, *fakeGroupOrders) {
	t.Helper()
	orders := &fakeGroupOrders{records: records}
	classifier, err := application.NewClassifier(orders)
	if err != nil {
		t.Fatalf("NewClassifier: %v", err)
	}
	if err := classifier.Track(hedgeGroup(), "BTC"); err != nil {
		t.Fatalf("Track: %v", err)
	}
	return classifier, orders
}

func classify(t *testing.T, c *application.Classifier) application.Verdict {
	t.Helper()
	v, err := c.Classify(hedgeGroup())
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	return v
}

func assertVerdict(t *testing.T, v application.Verdict, class domain.Classification, residual string) {
	t.Helper()
	if v.Class != class {
		t.Fatalf("class = %s, want %s (residual %s, range %s)", v.Class, class, v.Residual, v.Range)
	}
	if got, want := v.Residual.Decimal(), qtyDecimal(t, residual); got != want {
		t.Fatalf("residual = %s, want %s", v.Residual, residual)
	}
}

func qtyDecimal(t *testing.T, v string) string {
	t.Helper()
	return qty(t, v).Decimal()
}

// The realized delta is SIGNED by each leg's own direction, and the legs are
// deliberately UNEQUAL.
//
// A cash-and-carry's two legs carry the same base quantity, so the fixture that
// reads most naturally — buy 1, sell 1 — nets to zero under "buy minus sell",
// under "sell minus buy", and under "add both magnitudes and negate". Every
// wrong sign convention passes it. Here the spot leg has executed 1 and the perp
// leg 0.75, so the group is genuinely long 0.25 and only one arithmetic gives
// that.
func TestClassifySignsEachLegByItsOwnDirection(t *testing.T) {
	t.Parallel()

	classifier, _ := newClassifier(t,
		leg(t, 1, spot(), domain.SideBuy, "1", "1", domain.StateFilled),
		leg(t, 2, perp(), domain.SideSell, "0.75", "0.75", domain.StateFilled),
	)

	assertVerdict(t, classify(t, classifier), domain.ClassificationDetermined, "0.25")
}

// A fully hedged, fully executed group is Determined at zero — the positive
// control for the test above, without which "signs by direction" and "always
// reports a nonzero residual" would be indistinguishable.
func TestClassifyReportsAFullyHedgedGroupAsDeterminedAndFlat(t *testing.T) {
	t.Parallel()

	classifier, _ := newClassifier(t,
		leg(t, 1, spot(), domain.SideBuy, "1", "1", domain.StateFilled),
		leg(t, 2, perp(), domain.SideSell, "1", "1", domain.StateFilled),
	)

	assertVerdict(t, classify(t, classifier), domain.ClassificationDetermined, "0")
}

// §6.7's ambiguity test keys on the STATE, and Planned is the state that looks
// live and contributes nothing.
//
// Both rows hold the identical order — same instrument, same side, same ordered
// quantity, nothing executed — and differ only in the lifecycle state. That is
// the fixture that separates "contributes a possible delta" from "is not
// terminal": with different quantities, the two rows would differ for a second
// reason and the state's effect would be unattributable.
//
// The Planned row is the one that matters. §5.3 says such an order "was never
// dispatched (no venue order can exist)", and a classifier reading it as
// ambiguous would quiesce it to finality by querying a venue that has never
// heard of it — running to the deadline, holding new trading and alerting an
// operator over an order this system has merely not sent yet.
func TestClassifyTreatsPlannedAsContributingNothing(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		state domain.OrderState
		want  domain.Classification
	}{
		{"planned", domain.StatePlanned, domain.ClassificationDetermined},
		{"authorized to dispatch", domain.StateDispatchAuthorized, domain.ClassificationAmbiguous},
		{"acknowledged", domain.StateAcknowledged, domain.ClassificationAmbiguous},
		{"canceled", domain.StateCanceled, domain.ClassificationDetermined},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			classifier, _ := newClassifier(t,
				leg(t, 1, perp(), domain.SideSell, "1", "0", tc.state),
			)
			assertVerdict(t, classify(t, classifier), tc.want, "0")
		})
	}
}

// A partially filled leg is ambiguous, and its realized half is in the residual
// while its unexecuted half is in the range.
//
// The two halves are different numbers (0.4 executed of 1 ordered), so an
// implementation that put the whole ordered quantity in either place is visible.
func TestClassifySplitsAPartialFillBetweenResidualAndRange(t *testing.T) {
	t.Parallel()

	classifier, _ := newClassifier(t,
		leg(t, 1, spot(), domain.SideBuy, "1", "1", domain.StateFilled),
		leg(t, 2, perp(), domain.SideSell, "1", "0.4", domain.StatePartiallyFilled),
	)

	v := classify(t, classifier)
	assertVerdict(t, v, domain.ClassificationAmbiguous, "0.6")
	// 0.6 realized, and the remaining 0.6 of the perp sell may still execute.
	if got := v.Range.Lo().Decimal(); got != qtyDecimal(t, "0") {
		t.Fatalf("range low = %s, want 0", v.Range.Lo())
	}
	if got := v.Range.Hi().Decimal(); got != qtyDecimal(t, "0.6") {
		t.Fatalf("range high = %s, want 0.6", v.Range.Hi())
	}
}

// FM-5 at the classifier: reserving moves NOTHING.
//
// The group is genuinely long 0.5 before the reservation, so "the residual is
// unchanged" is a claim about a nonzero number rather than about zero — the
// negative-assertion trap this repo has recorded. A generation booking its
// target here would make the group read SHORT and send the determined branch to
// buy back exposure the generation's own leg is about to acquire.
func TestReserveDoesNotMoveTheResidualAndAmendDoes(t *testing.T) {
	t.Parallel()

	classifier, _ := newClassifier(t,
		leg(t, 1, spot(), domain.SideBuy, "1", "1", domain.StateFilled),
		leg(t, 2, perp(), domain.SideSell, "0.5", "0.5", domain.StateFilled),
	)
	assertVerdict(t, classify(t, classifier), domain.ClassificationDetermined, "0.5")

	reserved, err := classifier.Reserve(hedgeGroup(), "pkg-1", domain.ProducerPackageDeleverSpot)
	if err != nil {
		t.Fatalf("Reserve: %v", err)
	}
	assertVerdict(t, reserved, domain.ClassificationDetermined, "0.5")
	if got := reserved.Reserved.Decimal(); got != qtyDecimal(t, "0") {
		t.Fatalf("reserved = %s, want 0 — a generation that has committed nothing", reserved.Reserved)
	}

	// The producer confirms 0.2 of its perp reduction, so its entry now carries
	// an outstanding intent of 0.2 and the classifier's subtrahend moves.
	amended, err := classifier.Amend(hedgeGroup(), "pkg-1", qty(t, "0.2"))
	if err != nil {
		t.Fatalf("Amend: %v", err)
	}
	assertVerdict(t, amended, domain.ClassificationDetermined, "0.3")
	if got := amended.Reserved.Decimal(); got != qtyDecimal(t, "0.2") {
		t.Fatalf("reserved = %s, want 0.2", amended.Reserved)
	}
}

// FM-6: the third input's mutators return a FRESH verdict, with no separate
// Classify call.
//
// The assertion is on the verdict each mutator RETURNS, which is the whole
// claim: a ledger change that did not re-classify is what TLC's NC19 reaches,
// and its signature is a dead end — every interval collapsed, the verdict still
// stale, and nothing enabled.
func TestLedgerMutatorsReturnAFreshVerdict(t *testing.T) {
	t.Parallel()

	classifier, orders := newClassifier(t,
		leg(t, 1, spot(), domain.SideBuy, "2", "2", domain.StateFilled),
	)

	before := orders.calls
	reserved, err := classifier.Reserve(hedgeGroup(), "floor-1", domain.ProducerIntervalFloor)
	if err != nil {
		t.Fatalf("Reserve: %v", err)
	}
	if orders.calls == before {
		t.Fatal("Reserve did not re-read the group's orders, so its verdict is the caller's last one")
	}
	assertVerdict(t, reserved, domain.ClassificationDetermined, "2")

	amended, err := classifier.Amend(hedgeGroup(), "floor-1", qty(t, "-1.5"))
	if err != nil {
		t.Fatalf("Amend: %v", err)
	}
	assertVerdict(t, amended, domain.ClassificationDetermined, "3.5")

	released, verdict, err := classifier.Release(hedgeGroup(), "floor-1")
	if err != nil {
		t.Fatalf("Release: %v", err)
	}
	// The released amount is asserted against its exact value, not merely
	// against "nonzero": §6.7 row 4 requires the caller to re-enter exactly this
	// exposure as an unresolved order, and a caller handed the wrong magnitude
	// loses the difference with nothing failing.
	if got := released.Decimal(); got != qtyDecimal(t, "-1.5") {
		t.Fatalf("Release returned %s, want -1.5", released)
	}
	assertVerdict(t, verdict, domain.ClassificationDetermined, "2")
}

// The deadline branch, its exit, and the episode reset.
//
// One test rather than three because the claim is about a SEQUENCE: §6.7's
// deadline verdict is not terminal, the late collapse returns Determined, and
// what ends the episode is that report — which is what re-arms the one interval
// floor a later, distinct incident is entitled to.
func TestDeadlineVerdictIsLeftWhenTheIntervalsCollapse(t *testing.T) {
	t.Parallel()

	live := leg(t, 2, perp(), domain.SideSell, "1", "0", domain.StateAcknowledged)
	classifier, orders := newClassifier(t,
		leg(t, 1, spot(), domain.SideBuy, "1", "1", domain.StateFilled),
		live,
	)

	assertVerdict(t, classify(t, classifier), domain.ClassificationAmbiguous, "1")

	blown, err := classifier.DeadlineExceeded(hedgeGroup())
	if err != nil {
		t.Fatalf("DeadlineExceeded: %v", err)
	}
	assertVerdict(t, blown, domain.ClassificationDeadlineExceededAmbiguous, "1")

	// The venue answers: the perp leg was canceled with nothing executed. No
	// fill and no position event — §6.7's own case for "MUST NOT wait for the
	// next fill when order state alone became conclusive".
	orders.records[1] = leg(t, 2, perp(), domain.SideSell, "1", "0", domain.StateCanceled)
	assertVerdict(t, classify(t, classifier), domain.ClassificationDetermined, "1")

	// A second, distinct incident is entitled to its own deadline verdict — the
	// episode ended when Determined was reported.
	orders.records[1] = leg(t, 2, perp(), domain.SideSell, "1", "0", domain.StateAcknowledged)
	reblown, err := classifier.DeadlineExceeded(hedgeGroup())
	if err != nil {
		t.Fatalf("DeadlineExceeded: %v", err)
	}
	assertVerdict(t, reblown, domain.ClassificationDeadlineExceededAmbiguous, "1")
}

// A blown deadline does NOT override a collapsed range — model (a)'s NC14, at
// the use-case layer.
//
// The latched reading is what strands a machine with every interval collapsed, a
// determined naked residual and dispatch budget in hand.
func TestDeadlineExceededOnACollapsedGroupStaysDetermined(t *testing.T) {
	t.Parallel()

	classifier, _ := newClassifier(t,
		leg(t, 1, spot(), domain.SideBuy, "1", "1", domain.StateFilled),
	)

	v, err := classifier.DeadlineExceeded(hedgeGroup())
	if err != nil {
		t.Fatalf("DeadlineExceeded: %v", err)
	}
	assertVerdict(t, v, domain.ClassificationDetermined, "1")
}

// The interval floor: issued once, sized from the nearest-zero endpoint, and
// counted in the subtrahend from the instant it is authorized.
func TestIssueFloorReservesItsOwnGenerationAndIsIssuedOnce(t *testing.T) {
	t.Parallel()

	// Spot bought 1.4, the perp sell has executed 0.4 of 1 — realized long 1.0,
	// with 0.6 still able to execute. The summed range is [0.4, 1]: the near
	// endpoint is 0.4 and the far one is 1, so "nearest zero" and "worst
	// magnitude" are different answers. That is the NC1 distinction, and a range
	// whose near endpoint were zero could not express it — which is why the two
	// legs are deliberately unequal here.
	classifier, _ := newClassifier(t,
		leg(t, 1, spot(), domain.SideBuy, "1.4", "1.4", domain.StateFilled),
		leg(t, 2, perp(), domain.SideSell, "1", "0.4", domain.StatePartiallyFilled),
	)
	if _, err := classifier.DeadlineExceeded(hedgeGroup()); err != nil {
		t.Fatalf("DeadlineExceeded: %v", err)
	}

	authorization, after, err := classifier.IssueFloor(hedgeGroup(), "floor-1")
	if err != nil {
		t.Fatalf("IssueFloor: %v", err)
	}
	if got := authorization.Change.Decimal(); got != qtyDecimal(t, "-0.4") {
		t.Fatalf("floor change = %s, want -0.4 (worst-endpoint sizing would give -1)", authorization.Change)
	}
	if authorization.Reservation != "floor-1" {
		t.Fatalf("reservation = %q, want floor-1", authorization.Reservation)
	}

	// The reservation is counted BEFORE the order is submitted, which is what
	// stops the still-in-flight floor being re-read as fresh naked exposure.
	//
	// The ENTRY is +0.4 where the order's own change is −0.4, and the sign is
	// the assertion rather than an incidental: the ledger carries the exposure
	// the reduction REMOVES, so the subtraction in §6.7's
	// `realized_delta − outstanding_reserved_reductions` shrinks the residual by
	// the floor's size. Realized here is long 1.0, the floor is selling 0.4, so
	// what is still genuinely naked is 0.6. The opposite sign gives 1.4 —
	// arithmetic that reads the working order as MORE exposure — and that is
	// what this repo shipped until the generation ladder came to reuse it.
	if got := after.Reserved.Decimal(); got != qtyDecimal(t, "0.4") {
		t.Fatalf("reserved after IssueFloor = %s, want 0.4 (the exposure removed, not the order's −0.4 change)", after.Reserved)
	}
	if got := after.Residual.Decimal(); got != qtyDecimal(t, "0.6") {
		t.Fatalf("residual after IssueFloor = %s, want 0.6", after.Residual)
	}

	// FM-7's defence in depth: never twice in one episode.
	if _, _, err := classifier.IssueFloor(hedgeGroup(), "floor-2"); !errors.Is(err, application.ErrFloorAlreadyIssued) {
		t.Fatalf("second IssueFloor = %v, want ErrFloorAlreadyIssued", err)
	}
}

// TestAnInFlightFloorIsNotSoldASecondTimeWhenTheIntervalsCollapse is the sign
// convention's BITE, and the reason it is a separate test from the one above.
//
// A residual asserted at the moment the floor is issued is a number nobody
// spends. §6.7's late-resolution row is where it becomes money: the intervals
// collapse while the floor is STILL in flight, the group reports Determined, and
// the determined branch sizes an ExposureReduction from the residual. With the
// entry signed as the order's own change instead of the exposure it removes, the
// group reads long 1.4 against 0.6 genuinely naked, sells 1.4, and lands at
// −0.8 — a naked SHORT minted by reservation-aware sizing, which exists to
// prevent exactly that. Every quantity in that sentence is computed FROM the
// ledger, so no invariant over the residual can object (§46 §8e).
func TestAnInFlightFloorIsNotSoldASecondTimeWhenTheIntervalsCollapse(t *testing.T) {
	t.Parallel()

	classifier, orders := newClassifier(t,
		leg(t, 1, spot(), domain.SideBuy, "1.4", "1.4", domain.StateFilled),
		leg(t, 2, perp(), domain.SideSell, "1", "0.4", domain.StatePartiallyFilled),
	)
	if _, err := classifier.DeadlineExceeded(hedgeGroup()); err != nil {
		t.Fatalf("DeadlineExceeded: %v", err)
	}
	if _, _, err := classifier.IssueFloor(hedgeGroup(), "floor-1"); err != nil {
		t.Fatalf("IssueFloor: %v", err)
	}

	// The perp leg reaches a venue-terminal state with its 0.4 reconciled, so
	// every possible-delta interval has collapsed. The floor's own order has
	// NOT resolved — its entry is still in the ledger, which is the whole
	// window this test is about.
	orders.records[1].State = domain.StateCanceled

	verdict := classify(t, classifier)
	if verdict.Class != domain.ClassificationDetermined {
		t.Fatalf("class = %s, want determined once the intervals collapse", verdict.Class)
	}
	reduction, err := verdict.Reduction()
	if err != nil {
		t.Fatalf("Reduction: %v", err)
	}
	// Realized is long 1.0; the floor is already selling 0.4; 0.6 is naked.
	if got := reduction.Decimal(); got != qtyDecimal(t, "-0.6") {
		t.Fatalf("ExposureReduction = %s, want -0.6 (−1.4 double-sells the floor and lands the group short)", reduction)
	}
}

func TestIssueFloorRefusals(t *testing.T) {
	t.Parallel()

	t.Run("outside the deadline-exceeded branch", func(t *testing.T) {
		t.Parallel()

		// Merely ambiguous, and its range does not straddle — so a guaranteed
		// floor EXISTS and is still refused. That is §6.7's narrowing, and a
		// fixture whose range straddled would be refused for the other reason.
		classifier, _ := newClassifier(t,
			leg(t, 1, spot(), domain.SideBuy, "1", "1", domain.StateFilled),
			leg(t, 2, perp(), domain.SideSell, "1", "0.4", domain.StatePartiallyFilled),
		)
		_, _, err := classifier.IssueFloor(hedgeGroup(), "floor-1")
		if !errors.Is(err, application.ErrFloorOutsideDeadlineBranch) {
			t.Fatalf("IssueFloor = %v, want ErrFloorOutsideDeadlineBranch", err)
		}
	})

	t.Run("a straddling range", func(t *testing.T) {
		t.Parallel()

		// A live buy and a live sell on a flat realized delta: the range spans
		// zero in both directions, so no nonzero compensation is safe under
		// every resolution.
		classifier, _ := newClassifier(t,
			leg(t, 1, spot(), domain.SideBuy, "1", "0", domain.StateAcknowledged),
			leg(t, 2, perp(), domain.SideSell, "1", "0", domain.StateAcknowledged),
		)
		if _, err := classifier.DeadlineExceeded(hedgeGroup()); err != nil {
			t.Fatalf("DeadlineExceeded: %v", err)
		}
		_, _, err := classifier.IssueFloor(hedgeGroup(), "floor-1")
		if !errors.Is(err, domain.ErrIntervalStraddlesZero) {
			t.Fatalf("IssueFloor = %v, want ErrIntervalStraddlesZero", err)
		}
	})

	t.Run("a nearest endpoint of zero", func(t *testing.T) {
		t.Parallel()

		// NC1's own shape: realized long 1 with a pending sell of the whole 1,
		// giving [0, 1]. The pending hedge may already cover the position, so
		// nothing is naked under every resolution — and spending the episode's
		// one floor on a change of zero is what the refusal prevents.
		classifier, _ := newClassifier(t,
			leg(t, 1, spot(), domain.SideBuy, "1", "1", domain.StateFilled),
			leg(t, 2, perp(), domain.SideSell, "1", "0", domain.StateAcknowledged),
		)
		if _, err := classifier.DeadlineExceeded(hedgeGroup()); err != nil {
			t.Fatalf("DeadlineExceeded: %v", err)
		}
		_, _, err := classifier.IssueFloor(hedgeGroup(), "floor-1")
		if !errors.Is(err, application.ErrFloorMagnitudeZero) {
			t.Fatalf("IssueFloor = %v, want ErrFloorMagnitudeZero", err)
		}
	})
}

// Reduction is the NEGATION of the residual, and it is refused on every verdict
// that does not permit compensation.
//
// The residual is nonzero and asymmetric, so "negates" and "returns the
// residual" are different answers — negating zero is the fixture that could not
// tell them apart.
func TestVerdictReductionNegatesTheResidualAndOnlyWhenDetermined(t *testing.T) {
	t.Parallel()

	determined, _ := newClassifier(t,
		leg(t, 1, spot(), domain.SideBuy, "1", "0.75", domain.StateFilled),
	)
	reduction, err := classify(t, determined).Reduction()
	if err != nil {
		t.Fatalf("Reduction: %v", err)
	}
	if got := reduction.Decimal(); got != qtyDecimal(t, "-0.75") {
		t.Fatalf("Reduction = %s, want -0.75", reduction)
	}

	ambiguous, _ := newClassifier(t,
		leg(t, 1, spot(), domain.SideBuy, "1", "0.75", domain.StatePartiallyFilled),
	)
	if _, err := classify(t, ambiguous).Reduction(); !errors.Is(err, application.ErrCompensationBeforeFinality) {
		t.Fatalf("Reduction on an ambiguous verdict = %v, want ErrCompensationBeforeFinality", err)
	}
}

// The classifier refuses a record that is not in the group it was asked about,
// rather than skipping it.
//
// Skipping computes a residual over a SUBSET and reports it as the group's,
// which is the one error here that produces a confident answer instead of a
// refusal. The foreign record differs in SYMBOL only — same venue, same product
// — because a record differing in several components would be rejected by any
// check.
func TestClassifyRefusesAnOrderOutsideTheGroup(t *testing.T) {
	t.Parallel()

	foreign := leg(t, 2, ids.VenueInstrument{
		Venue: "binance", Product: ids.ProductSpot, Symbol: "ETHUSDT",
	}, domain.SideBuy, "1", "1", domain.StateFilled)

	classifier, _ := newClassifier(t,
		leg(t, 1, spot(), domain.SideBuy, "1", "1", domain.StateFilled),
		foreign,
	)

	if _, err := classifier.Classify(hedgeGroup()); !errors.Is(err, domain.ErrOrderOutsideHedgeGroup) {
		t.Fatalf("Classify = %v, want ErrOrderOutsideHedgeGroup", err)
	}
}

// An untracked group is refused rather than created on demand.
//
// The direction is the point: a classifier that tracked on first sight would
// answer an unknown group with Determined, residual zero and no reservations —
// the one verdict that unblocks a compensating trade — for a group whose orders
// and reservations it has never seen.
func TestClassifierRefusesAnUntrackedGroup(t *testing.T) {
	t.Parallel()

	classifier, _ := newClassifier(t)
	other := domain.HedgeGroup{Account: "sub-1", Venue: "binance", Symbol: "ETHUSDT"}

	if _, err := classifier.Classify(other); !errors.Is(err, application.ErrHedgeGroupNotTracked) {
		t.Fatalf("Classify = %v, want ErrHedgeGroupNotTracked", err)
	}
	if _, err := classifier.Reserve(other, "x", domain.ProducerIntervalFloor); !errors.Is(err, application.ErrHedgeGroupNotTracked) {
		t.Fatalf("Reserve = %v, want ErrHedgeGroupNotTracked", err)
	}
}

// Each group's reserved-reduction ledger and episode state are its own.
//
// Two groups rather than one, because with a single group "this group's ledger"
// and "the classifier's ledger" are the same object and every keying mistake
// passes. The second group's residual is a different nonzero number, so a
// classifier that shared one episode would report a visibly wrong figure rather
// than a coincidentally equal one.
func TestGroupsDoNotShareLedgersOrEpisodes(t *testing.T) {
	t.Parallel()

	orders := &fakeGroupOrders{}
	classifier, err := application.NewClassifier(orders)
	if err != nil {
		t.Fatalf("NewClassifier: %v", err)
	}
	if err := classifier.Track(hedgeGroup(), "BTC"); err != nil {
		t.Fatalf("Track: %v", err)
	}
	if err := classifier.Track(hedgeGroup(), "BTC"); !errors.Is(err, application.ErrHedgeGroupAlreadyTracked) {
		t.Fatalf("second Track = %v, want ErrHedgeGroupAlreadyTracked", err)
	}

	orders.records = []ports.OrderRecord{
		leg(t, 1, spot(), domain.SideBuy, "1", "1", domain.StateFilled),
	}
	if _, err := classifier.Reserve(hedgeGroup(), "pkg-1", domain.ProducerPackageDeleverSpot); err != nil {
		t.Fatalf("Reserve: %v", err)
	}
	if _, err := classifier.Amend(hedgeGroup(), "pkg-1", qty(t, "0.4")); err != nil {
		t.Fatalf("Amend: %v", err)
	}

	// A second group on another symbol. The fake answers with the same records
	// for any group, so the ONLY thing that can differ between the two verdicts
	// is the per-group ledger — which is exactly the claim.
	second := domain.HedgeGroup{Account: "sub-1", Venue: "binance", Symbol: "BTCUSDT2"}
	if err := classifier.Track(second, "BTC"); err != nil {
		t.Fatalf("Track: %v", err)
	}
	orders.records = []ports.OrderRecord{
		leg(t, 1, ids.VenueInstrument{
			Venue: "binance", Product: ids.ProductSpot, Symbol: "BTCUSDT2",
		}, domain.SideBuy, "1", "1", domain.StateFilled),
	}
	v, err := classifier.Classify(second)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if got := v.Reserved.Decimal(); got != qtyDecimal(t, "0") {
		t.Fatalf("second group's reserved = %s, want 0 — the first group's ledger leaked", v.Reserved)
	}
	if got := v.Residual.Decimal(); got != qtyDecimal(t, "1") {
		t.Fatalf("second group's residual = %s, want 1", v.Residual)
	}
}

// A tracked group with no orders is Determined at zero of its base asset.
//
// It is a real state — before the first plan, and after §6 step 9's close — and
// the asset tag is what makes it expressible at all, which is why Track takes
// one.
func TestAnEmptyGroupIsDeterminedAtZero(t *testing.T) {
	t.Parallel()

	classifier, _ := newClassifier(t)
	v := classify(t, classifier)
	assertVerdict(t, v, domain.ClassificationDetermined, "0")
	if v.Residual.Asset() != "BTC" {
		t.Fatalf("residual asset = %q, want BTC", v.Residual.Asset())
	}
}

func TestNewClassifierAndTrackRefusals(t *testing.T) {
	t.Parallel()

	if _, err := application.NewClassifier(nil); !errors.Is(err, application.ErrHedgeGroupOrdersMissing) {
		t.Fatalf("NewClassifier(nil) = %v, want ErrHedgeGroupOrdersMissing", err)
	}

	classifier, err := application.NewClassifier(&fakeGroupOrders{})
	if err != nil {
		t.Fatalf("NewClassifier: %v", err)
	}
	if err := classifier.Track(hedgeGroup(), ""); !errors.Is(err, application.ErrHedgeGroupAssetUnset) {
		t.Fatalf("Track with no asset = %v, want ErrHedgeGroupAssetUnset", err)
	}
	incomplete := domain.HedgeGroup{Venue: "binance", Symbol: "BTCUSDT"}
	if err := classifier.Track(incomplete, "BTC"); !errors.Is(err, domain.ErrHedgeGroupAccountUnset) {
		t.Fatalf("Track with no account = %v, want ErrHedgeGroupAccountUnset", err)
	}
}

// A failure from the order source is passed through, not absorbed into a
// verdict.
//
// The safe-looking absorption would be to classify the group as Ambiguous when
// the records cannot be read, which is wrong in the way that matters: Ambiguous
// is a CONCLUSION about the venue's state, and a caller cannot tell it from one
// drawn on real records.
func TestClassifyPassesAReadFailureThrough(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("repository unavailable")
	orders := &fakeGroupOrders{err: sentinel}
	classifier, err := application.NewClassifier(orders)
	if err != nil {
		t.Fatalf("NewClassifier: %v", err)
	}
	if err := classifier.Track(hedgeGroup(), "BTC"); err != nil {
		t.Fatalf("Track: %v", err)
	}
	if _, err := classifier.Classify(hedgeGroup()); !errors.Is(err, sentinel) {
		t.Fatalf("Classify = %v, want the source's error", err)
	}
}

// An unpopulated cumulative quantity is refused rather than read as zero.
//
// ports.OrderRecord documents Cumulative as "zero OF THE ORDERED ASSET on a
// record with no fills", so an untagged one is a record something built by hand
// — and reading it as zero would silently drop a leg's realized exposure from
// the group's delta.
func TestClassifyRefusesARecordWithAnUnpopulatedCumulative(t *testing.T) {
	t.Parallel()

	broken := leg(t, 1, spot(), domain.SideBuy, "1", "0", domain.StateFilled)
	broken.Cumulative = fixedpoint.Quantity{}

	classifier, _ := newClassifier(t, broken)
	if _, err := classifier.Classify(hedgeGroup()); !errors.Is(err, fixedpoint.ErrAssetUnset) {
		t.Fatalf("Classify = %v, want ErrAssetUnset", err)
	}
}

// Reporting Determined ENDS the episode, and the evidence for that is what
// happens NEXT — not what happens at the report.
//
// # Both flags survived their first mutation, and the reason is one shape
//
// Deleting either reset left every earlier test green. The deadline flag is
// invisible at the moment of resolution because Classify's precedence already
// returns Determined for a collapsed range whether or not the flag is set; the
// floor flag is invisible because the next call in those tests was
// DeadlineExceeded, which sets the deadline flag itself. In both cases the
// assertion could not see the variation: the observable difference only appears
// on the FOLLOWING incident, which no test had.
//
// So this walks two incidents. The second one must start Ambiguous — a fresh
// ambiguity that inherited a stale deadline verdict would hold new trading and
// admit a floor with no deadline ever having blown — and it must be entitled to
// its own floor, because §6.7 states "never issued twice" inside the
// deadline-exceeded row and a global reading leaves a second incident with no
// admissible safe action at all.
func TestResolvingAnEpisodeRearmsBothTheDeadlineAndTheFloor(t *testing.T) {
	t.Parallel()

	// Incident one: spot long 1.4, perp sell 1 with 0.4 executed. Range [0.4, 1].
	classifier, orders := newClassifier(t,
		leg(t, 1, spot(), domain.SideBuy, "1.4", "1.4", domain.StateFilled),
		leg(t, 2, perp(), domain.SideSell, "1", "0.4", domain.StatePartiallyFilled),
	)
	if _, err := classifier.DeadlineExceeded(hedgeGroup()); err != nil {
		t.Fatalf("DeadlineExceeded: %v", err)
	}
	first, _, err := classifier.IssueFloor(hedgeGroup(), "floor-1")
	if err != nil {
		t.Fatalf("IssueFloor: %v", err)
	}
	if got := first.Change.Decimal(); got != qtyDecimal(t, "-0.4") {
		t.Fatalf("first floor = %s, want -0.4", first.Change)
	}

	// The perp leg is confirmed canceled with its 0.4 executed. Every interval
	// has collapsed, so the episode resolves.
	orders.records[1] = leg(t, 2, perp(), domain.SideSell, "1", "0.4", domain.StateCanceled)
	// Realized is long 1.0 and floor-1's entry is still in the ledger holding
	// the 0.4 it is selling, so the residual is what is genuinely naked: 0.6.
	assertVerdict(t, classify(t, classifier), domain.ClassificationDetermined, "0.6")

	// Incident two: a new counter-leg goes live. Nothing has timed out.
	orders.records = append(orders.records,
		leg(t, 3, perp(), domain.SideSell, "0.5", "0", domain.StateAcknowledged))
	assertVerdict(t, classify(t, classifier), domain.ClassificationAmbiguous, "0.6")

	// And a floor asked for now is refused for the RIGHT reason — the branch,
	// not the spent flag. Without this the row above would be equally satisfied
	// by a classifier that had merely forgotten the group.
	if _, _, err := classifier.IssueFloor(hedgeGroup(), "floor-2"); !errors.Is(err, application.ErrFloorOutsideDeadlineBranch) {
		t.Fatalf("IssueFloor = %v, want ErrFloorOutsideDeadlineBranch", err)
	}

	// This incident's own deadline blows, and it is entitled to its own floor.
	if _, err := classifier.DeadlineExceeded(hedgeGroup()); err != nil {
		t.Fatalf("DeadlineExceeded: %v", err)
	}
	second, _, err := classifier.IssueFloor(hedgeGroup(), "floor-2")
	if err != nil {
		t.Fatalf("second IssueFloor: %v", err)
	}
	// Residual 0.6 with 0.5 still able to sell gives [0.1, 0.6]; the near
	// endpoint is 0.1. Asserted as a value rather than as "no error", because a
	// floor issued at the wrong magnitude is the failure the rule exists for.
	//
	// The magnitude is SMALLER than the first floor's, and that is FM-7's
	// mechanism visible in a number: the second floor is sized net of the
	// first's still-outstanding reservation, which is what "reservation-aware
	// sizing already prevents a second floor reaching the opposite sign" means.
	// Signed as the order's own change, the ledger compounded instead — the
	// second floor came out at 0.9, larger than the first.
	if got := second.Change.Decimal(); got != qtyDecimal(t, "-0.1") {
		t.Fatalf("second floor = %s, want -0.1", second.Change)
	}
}
