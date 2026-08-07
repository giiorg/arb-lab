package application_test

import (
	"errors"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/oms/application"
	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// The Deleverer's fixtures, and the two things they deliberately do not share.
//
//  1. The CLASSIFIER reads the hedge group's orders through ports.HedgeGroupOrders
//     and the DELEVERER verifies its legs through ports.OrderWriteRepository.
//     Those are two ports, and the fixture plants into both rather than into one
//     view both read — because a single store would make "verifies the leg
//     against the durable record" and "verifies it against the premise's own
//     fold" the same act, and they are not: only one of them is refreshed by
//     §7.5's replay.
//  2. The premise is dissolved TWO different ways across this file. Premise.Intact
//     is `residual == 0 AND class == Determined`, so a suite that only ever broke
//     both conjuncts at once could not tell a check on the number from a check on
//     the verdict. externalSale breaks the first alone (Determined, nonzero) and
//     externalInFlight breaks the second alone (zero, Ambiguous).

// genCall is one durable generation record, in the order it was written.
//
// It is one flat struct with a field per payload — the optional-field shape
// ports.GenerationJournal refuses in its API — for the reason dispatch_test.go's
// journalCall states: a test asserting the ORDER of records needs them in one
// slice. The `kind` field is what keeps that honest, because two records that
// differ only in which optional field is set are otherwise indistinguishable
// from one record written twice.
type genCall struct {
	kind        string
	id          domain.GenerationID
	phase       domain.GenerationPhase
	group       domain.HedgeGroup
	reservation domain.ReservationID
	amount      fixedpoint.Quantity
	order       ids.ClientOrderID
	path        domain.SupersessionPath

	// placesBefore is how many venue submissions had happened when this record
	// was written. It is FLAG-3's whole claim as an observation about
	// INTERLEAVING rather than an inference from two totals: "durable before the
	// spot dispatch" is false in exactly the world where the two counts still
	// agree at the end.
	placesBefore int
}

type fakeGenerationJournal struct {
	calls []genCall

	// failOn maps a phase to the error its append returns, keyed by phase so a
	// test names the step it is breaking rather than counting calls.
	failOn map[domain.GenerationPhase]error

	// watch is the placer, read at the moment each record is written.
	watch *fakePlacer
}

func newFakeGenerationJournal(p *fakePlacer) *fakeGenerationJournal {
	return &fakeGenerationJournal{failOn: make(map[domain.GenerationPhase]error), watch: p}
}

func (j *fakeGenerationJournal) places() int {
	if j.watch == nil {
		return 0
	}
	j.watch.mu.Lock()
	defer j.watch.mu.Unlock()
	return j.watch.calls
}

func (j *fakeGenerationJournal) record(c genCall) error {
	c.placesBefore = j.places()
	j.calls = append(j.calls, c)
	return j.failOn[c.phase]
}

func (j *fakeGenerationJournal) AppendGenerationOpened(id domain.GenerationID, group domain.HedgeGroup,
	reservation domain.ReservationID, target fixedpoint.Quantity) error {
	return j.record(genCall{
		kind: "opened", id: id, phase: domain.GenerationPlanned,
		group: group, reservation: reservation, amount: target,
	})
}

func (j *fakeGenerationJournal) AppendGenerationPerpSubmitted(id domain.GenerationID, order ids.ClientOrderID) error {
	return j.record(genCall{
		kind: "perp", id: id, phase: domain.GenerationPerpReduceInFlight, order: order,
	})
}

func (j *fakeGenerationJournal) AppendGenerationAuthorized(id domain.GenerationID, size fixedpoint.Quantity) error {
	return j.record(genCall{
		kind: "authorized", id: id, phase: domain.GenerationSpotReduceAuthorized, amount: size,
	})
}

func (j *fakeGenerationJournal) AppendGenerationSpotSubmitted(id domain.GenerationID, order ids.ClientOrderID) error {
	return j.record(genCall{
		kind: "spot", id: id, phase: domain.GenerationSpotReduceInFlight, order: order,
	})
}

func (j *fakeGenerationJournal) AppendGenerationSuperseded(id domain.GenerationID, path domain.SupersessionPath) error {
	return j.record(genCall{
		kind: "superseded", id: id, phase: domain.GenerationSuperseded, path: path,
	})
}

// AppendGenerationPhase refuses the five payload-carrying phases exactly as the
// port requires. A laxer fake would let the use case record an authorization
// with no size or a supersession with no path, which are the two records
// recovery cannot reconstruct — so the refusal is here rather than only in a
// future adapter, on the same argument fakeJournal's AppendAcked states.
func (j *fakeGenerationJournal) AppendGenerationPhase(id domain.GenerationID, phase domain.GenerationPhase) error {
	switch phase {
	case domain.GenerationPlanned, domain.GenerationPerpReduceInFlight,
		domain.GenerationSpotReduceAuthorized, domain.GenerationSpotReduceInFlight,
		domain.GenerationSuperseded:
		return ports.ErrGenerationPhaseNeedsPayload

	// The five this method owns, NAMED rather than left to a default — §3.6
	// rule 10's escape, and the gate caught this switch on its first run. It
	// matters more in a fake than it looks: a phase added later would otherwise
	// fall through to "recorded, no payload", and this file would keep passing
	// while the port's own totality argument had quietly acquired a hole.
	case domain.GenerationPerpReduceFinal, domain.GenerationHeldPerp, domain.GenerationHeldSpot,
		domain.GenerationComplete, domain.GenerationTerminalFlatten:
	}
	return j.record(genCall{kind: "phase", id: id, phase: phase})
}

var _ ports.GenerationJournal = (*fakeGenerationJournal)(nil)

// phases is the record sequence as phases alone, for the ordering assertions.
func (j *fakeGenerationJournal) phases() []domain.GenerationPhase {
	out := make([]domain.GenerationPhase, 0, len(j.calls))
	for _, c := range j.calls {
		out = append(out, c.phase)
	}
	return out
}

func (j *fakeGenerationJournal) find(t *testing.T, kind string) genCall {
	t.Helper()
	for _, c := range j.calls {
		if c.kind == kind {
			return c
		}
	}
	t.Fatalf("no %q record in %v", kind, j.phases())
	return genCall{}
}

type deleverFixture struct {
	t            *testing.T
	deleverer    *application.Deleverer
	classifier   *application.Classifier
	groupOrders  *fakeGroupOrders
	orders       *fakeOrders
	journal      *fakeGenerationJournal
	orderJournal *fakeJournal
	placer       *fakePlacer
	clk          *clock.TestClock
}

// newDeleverer builds the use case over a hedge group already holding `records`.
func newDeleverer(t *testing.T, records ...ports.OrderRecord) *deleverFixture {
	t.Helper()
	return newDelevererOn(t, testClock(t), testFilters(t), testFilterMaxAge, records...)
}

// newDelevererOn is newDeleverer with §2.4/§69's three knobs exposed — the
// clock each leg's admission instant is read from, the filter map the ladder
// judges its legs against, and the row-age bound (#184).
//
// They reach the Deleverer through the DISPATCHER, which is the point: there is
// no parameter on NewDeleverer through which a second policy could arrive, so
// this helper is also the demonstration that the ladder enforces the process's
// one policy rather than a copy of its own.
func newDelevererOn(t *testing.T, clk *clock.TestClock,
	filters map[ids.VenueInstrument]domain.TradingFilters, maxAge time.Duration,
	records ...ports.OrderRecord) *deleverFixture {

	t.Helper()
	groupOrders := &fakeGroupOrders{records: records}
	classifier, err := application.NewClassifier(groupOrders)
	if err != nil {
		t.Fatalf("NewClassifier: %v", err)
	}
	if err := classifier.Track(hedgeGroup(), "BTC"); err != nil {
		t.Fatalf("Track: %v", err)
	}

	oj := newFakeJournal()
	placer := &fakePlacer{watch: oj}
	orders := newFakeOrders()
	dispatcher, err := application.NewDispatcher(oj, placer, orders, clk,
		application.NewDispatchAuthority(), filters, maxAge)
	if err != nil {
		t.Fatalf("NewDispatcher: %v", err)
	}
	gj := newFakeGenerationJournal(placer)
	d, err := application.NewDeleverer(dispatcher, classifier, gj)
	if err != nil {
		t.Fatalf("NewDeleverer: %v", err)
	}

	f := &deleverFixture{
		t: t, deleverer: d, classifier: classifier, groupOrders: groupOrders,
		orders: orders, journal: gj, orderJournal: oj, placer: placer, clk: clk,
	}
	for _, rec := range records {
		f.orders.records[rec.Order.ClientOrderID] = rec
	}
	return f
}

// add puts a record in BOTH stores: the classifier's fold and the repository the
// deleverer verifies legs against.
func (f *deleverFixture) add(rec ports.OrderRecord) ports.OrderRecord {
	f.t.Helper()
	f.groupOrders.records = append(f.groupOrders.records, rec)
	f.orders.records[rec.Order.ClientOrderID] = rec
	return rec
}

// ref is the DispatchRef for a planted record.
func ref(rec ports.OrderRecord) application.DispatchRef {
	return application.DispatchRef{ID: rec.Order.ClientOrderID, Version: rec.Version}
}

// venueSees replaces a record in the CLASSIFIER's view alone, standing in for
// the order-lifecycle events §6.7's inputs 1 and 2 arrive on.
//
// The fixture advances the two stores separately because the system does: a
// dispatch moves the write repository, and what the classifier folds over is the
// projection those events later reach. A helper that moved both would make every
// test below assert a world in which a submitted order is instantly visible to
// the fold, which is the one timing §6.7's whole interval algebra exists for.
func (f *deleverFixture) venueSees(rec ports.OrderRecord) {
	f.t.Helper()
	for i, existing := range f.groupOrders.records {
		if existing.Order.ClientOrderID == rec.Order.ClientOrderID {
			f.groupOrders.records[i] = rec
			return
		}
	}
	f.t.Fatalf("venueSees: no record for %q", rec.Order.ClientOrderID)
}

// perpLeg is the generation's buy-to-reduce order, planted planned-and-unfilled
// so that it contributes an interval to the raw verdict — which is the state §7
// depends on and the state FM-9's netting must see through.
func perpLeg(t *testing.T, n int, size string) ports.OrderRecord {
	t.Helper()
	return leg(t, n, perp(), domain.SideBuy, size, "0", domain.StatePlanned)
}

// spotLeg is the generation's reserved sale.
func spotLeg(t *testing.T, n int, size string) ports.OrderRecord {
	t.Helper()
	return leg(t, n, spot(), domain.SideSell, size, "0", domain.StatePlanned)
}

// externalSale dissolves the premise through its RESIDUAL alone: a filled sale
// by somebody else leaves the group Determined and genuinely short.
func externalSale(t *testing.T, n int, size string) ports.OrderRecord {
	t.Helper()
	return leg(t, n, spot(), domain.SideSell, size, size, domain.StateFilled)
}

// externalInFlight dissolves the premise through its CLASS alone: an unresolved
// order by somebody else contributes an interval while moving no quantity, so
// the residual stays zero and the verdict leaves Determined.
func externalInFlight(t *testing.T, n int, size string) ports.OrderRecord {
	t.Helper()
	return leg(t, n, spot(), domain.SideSell, size, "0", domain.StateSubmitted)
}

func (f *deleverFixture) open(target string) *domain.Generation {
	f.t.Helper()
	g, err := f.deleverer.Open("gen-1", hedgeGroup(), "pkg-1", qty(f.t, target))
	if err != nil {
		f.t.Fatalf("Open: %v", err)
	}
	return g
}

// toSpotAuthorization drives a freshly-opened generation to
// `SpotReduceAuthorized` — the rung a test about the SPOT leg has to start from,
// which is four calls away and which §7's sequencing makes unavoidable: the
// authorized size is capped by the CONFIRMED perp reduction, so the perp leg has
// to be submitted, filled and finalised first.
//
// target is the perp leg's size and its full fill; authorized is the spot
// reduction the caller then fixes, which the aggregate caps at target.
func (f *deleverFixture) toSpotAuthorization(target, authorized string) {
	f.t.Helper()
	perp := f.add(perpLeg(f.t, 10, target))
	if _, err := f.deleverer.SubmitPerpReduction(hedgeGroup(), ref(perp)); err != nil {
		f.t.Fatalf("SubmitPerpReduction: %v", err)
	}
	if err := f.deleverer.RecordPerpFill(hedgeGroup(), qty(f.t, target)); err != nil {
		f.t.Fatalf("RecordPerpFill: %v", err)
	}
	if err := f.deleverer.ConfirmPerpReduction(hedgeGroup()); err != nil {
		f.t.Fatalf("ConfirmPerpReduction: %v", err)
	}
	if err := f.deleverer.AuthorizeSpotReduction(hedgeGroup(), qty(f.t, authorized)); err != nil {
		f.t.Fatalf("AuthorizeSpotReduction(%s): %v", authorized, err)
	}
}

// completeWithNoSpotSale drives the generation to `Complete` the shortest way
// SM §7 admits: the perp leg submitted, confirmed with nothing executed, and a
// spot authorization of ZERO — §7's own honest extreme, "sells no more spot than
// the confirmed perp reduction" where the confirmed reduction is nothing.
//
// It exists so a test whose subject is what happens AFTER an ending can reach an
// ending that is not a supersession. Before the ladder bar, any ending would do
// and the supersession was the cheapest to write; the two are not
// interchangeable now, and a helper is what stops the next fixture reaching for
// the cheap one again.
func (f *deleverFixture) completeWithNoSpotSale() {
	f.t.Helper()
	perp := f.add(perpLeg(f.t, 20, "1"))
	if _, err := f.deleverer.SubmitPerpReduction(hedgeGroup(), ref(perp)); err != nil {
		f.t.Fatalf("SubmitPerpReduction: %v", err)
	}
	if err := f.deleverer.ConfirmPerpReduction(hedgeGroup()); err != nil {
		f.t.Fatalf("ConfirmPerpReduction: %v", err)
	}
	if err := f.deleverer.AuthorizeSpotReduction(hedgeGroup(), qty(f.t, "0")); err != nil {
		f.t.Fatalf("AuthorizeSpotReduction(0): %v", err)
	}
	if _, err := f.deleverer.SubmitSpotReduction(hedgeGroup(), application.DispatchRef{}); err != nil {
		f.t.Fatalf("SubmitSpotReduction(zero): %v", err)
	}
}

// residual is the group's ordinary §6.7 residual — what every OTHER client of
// the classifier sees, which is the quantity the ledger arithmetic is about.
func (f *deleverFixture) residual() string {
	f.t.Helper()
	v, err := f.classifier.Classify(hedgeGroup())
	if err != nil {
		f.t.Fatalf("Classify: %v", err)
	}
	return v.Residual.Decimal()
}

// reserved is the subtrahend §2's sizing removes.
func (f *deleverFixture) reserved() string {
	f.t.Helper()
	v, err := f.classifier.Classify(hedgeGroup())
	if err != nil {
		f.t.Fatalf("Classify: %v", err)
	}
	return v.Reserved.Decimal()
}

func (f *deleverFixture) assertReserved(want string) {
	f.t.Helper()
	if got := f.reserved(); got != qtyDecimal(f.t, want) {
		f.t.Fatalf("outstanding_reserved_reductions = %s, want %s", got, want)
	}
}

func (f *deleverFixture) assertResidual(want string) {
	f.t.Helper()
	if got := f.residual(); got != qtyDecimal(f.t, want) {
		f.t.Fatalf("residual = %s, want %s", got, want)
	}
}

// TestTheLedgerTracksTheGenerationsCommitmentAtEveryPhase is SM §7's "exactly
// backed at every instant" invariant, run as a ladder rather than asserted.
//
// The figures are deliberately UNEQUAL at three points — target 1, perp confirmed
// 0.6, spot authorized 0.4 — because §7's three sentences about the entry's
// amount give the SAME answer whenever those coincide. With target == confirmed
// == authorized, "carries the outstanding net intent", "carries the target" and
// "carries the authorization" are one number, and FM-5 (booking the target) would
// pass every assertion below.
func TestTheLedgerTracksTheGenerationsCommitmentAtEveryPhase(t *testing.T) {
	t.Parallel()

	f := newDeleverer(t, neutralPackage(t)...)
	f.open("1")

	// `Planned`: the entry exists and is ZERO. This is FM-5 itself — the perp
	// leg has bought nothing, so the outstanding net intent is nothing, and
	// booking the target here would make a still-neutral package read as short
	// by 1.
	f.assertReserved("0")
	f.assertResidual("0")

	perp := f.add(perpLeg(t, 10, "1"))
	if _, err := f.deleverer.SubmitPerpReduction(hedgeGroup(), ref(perp)); err != nil {
		t.Fatalf("SubmitPerpReduction: %v", err)
	}
	// Submitted but unfilled: still zero committed.
	f.assertReserved("0")

	if err := f.deleverer.RecordPerpFill(hedgeGroup(), qty(t, "0.6")); err != nil {
		t.Fatalf("RecordPerpFill: %v", err)
	}
	// The confirmed buy raised the group's realized delta by 0.6, and the entry
	// rose with it — so the package still reads neutral, which is the whole
	// purpose of the reservation.
	f.assertReserved("0.6")

	if err := f.deleverer.ConfirmPerpReduction(hedgeGroup()); err != nil {
		t.Fatalf("ConfirmPerpReduction: %v", err)
	}
	f.assertReserved("0.6")

	// The authorization is STRICTLY LESS than the confirmed reduction. §7 says
	// "no MORE spot than the confirmed perp reduction", and "no more" and
	// "exactly" are different predicates.
	if err := f.deleverer.AuthorizeSpotReduction(hedgeGroup(), qty(t, "0.4")); err != nil {
		t.Fatalf("AuthorizeSpotReduction: %v", err)
	}
	f.assertReserved("0.4")

	spot := f.add(spotLeg(t, 11, "0.4"))
	if _, err := f.deleverer.SubmitSpotReduction(hedgeGroup(), ref(spot)); err != nil {
		t.Fatalf("SubmitSpotReduction: %v", err)
	}
	f.assertReserved("0.4")

	if err := f.deleverer.ConfirmSpotReduction(hedgeGroup(), qty(t, "0.4")); err != nil {
		t.Fatalf("ConfirmSpotReduction: %v", err)
	}
	// Complete: the entry is RETIRED, so it subtracts nothing at all.
	f.assertReserved("0")
}

// TestTheAuthorizationIsDurableBeforeTheSpotOrderIsSent is FLAG-3, asserted as an
// interleaving rather than as two totals.
//
// "A durable SpotReduceAuthorized journal record between confirmed perp reduction
// and spot submission … so recovery knows the perp-finality precondition was met
// without re-derivation." The claim is false in exactly the world where the
// record is written after the send — and in that world the counts at the end are
// still one record and one Place, so counting them proves nothing.
func TestTheAuthorizationIsDurableBeforeTheSpotOrderIsSent(t *testing.T) {
	t.Parallel()

	f := newDeleverer(t, neutralPackage(t)...)
	f.open("1")
	perp := f.add(perpLeg(t, 10, "1"))
	if _, err := f.deleverer.SubmitPerpReduction(hedgeGroup(), ref(perp)); err != nil {
		t.Fatalf("SubmitPerpReduction: %v", err)
	}
	if err := f.deleverer.RecordPerpFill(hedgeGroup(), qty(t, "1")); err != nil {
		t.Fatalf("RecordPerpFill: %v", err)
	}
	if err := f.deleverer.ConfirmPerpReduction(hedgeGroup()); err != nil {
		t.Fatalf("ConfirmPerpReduction: %v", err)
	}
	if err := f.deleverer.AuthorizeSpotReduction(hedgeGroup(), qty(t, "1")); err != nil {
		t.Fatalf("AuthorizeSpotReduction: %v", err)
	}

	spot := f.add(spotLeg(t, 11, "1"))
	if _, err := f.deleverer.SubmitSpotReduction(hedgeGroup(), ref(spot)); err != nil {
		t.Fatalf("SubmitSpotReduction: %v", err)
	}

	auth := f.journal.find(t, "authorized")
	if auth.placesBefore != 1 {
		t.Fatalf("authorization record written after %d venue submissions, want 1 (the perp leg only)",
			auth.placesBefore)
	}
	if got := auth.amount.Decimal(); got != qtyDecimal(t, "1") {
		t.Fatalf("authorization record carries %s, want 1", got)
	}
	// The claim is that the record precedes the SPOT send, so the total has to
	// be read too: 1 < 2 is the assertion, and `auth.placesBefore == 1` alone
	// would also hold in a world where the spot leg was never sent at all.
	if f.placer.calls != 2 {
		t.Fatalf("placed %d orders, want 2", f.placer.calls)
	}
	// The control that `placesBefore` measures something that MOVES rather than
	// a constant the fixture never varies. The perp record is written before any
	// send at all, so the three record kinds sit at 0, 1 and 1 — and a counter
	// stuck at whatever it was first read at would fail here.
	if perpRec := f.journal.find(t, "perp"); perpRec.placesBefore != 0 {
		t.Fatalf("perp leg record written after %d submissions, want 0", perpRec.placesBefore)
	}
}

// TestEveryLegTheGenerationSubmitsIsJournaledWithItsOrderID is FM-9's netting
// input made recoverable.
//
// domain.Generation.OwnLegs is the set the premise is evaluated net of, and it
// lives in memory. A generation recovered without its leg ids computes the
// premise over a set that still contains its own perp reduction — which is the
// raw reading model (a)'s NC22 aborts on four states from Init.
func TestEveryLegTheGenerationSubmitsIsJournaledWithItsOrderID(t *testing.T) {
	t.Parallel()

	f := newDeleverer(t, neutralPackage(t)...)
	g := f.open("1")
	perp := f.add(perpLeg(t, 10, "1"))
	if _, err := f.deleverer.SubmitPerpReduction(hedgeGroup(), ref(perp)); err != nil {
		t.Fatalf("SubmitPerpReduction: %v", err)
	}
	if err := f.deleverer.RecordPerpFill(hedgeGroup(), qty(t, "1")); err != nil {
		t.Fatalf("RecordPerpFill: %v", err)
	}
	if err := f.deleverer.ConfirmPerpReduction(hedgeGroup()); err != nil {
		t.Fatalf("ConfirmPerpReduction: %v", err)
	}
	if err := f.deleverer.AuthorizeSpotReduction(hedgeGroup(), qty(t, "1")); err != nil {
		t.Fatalf("AuthorizeSpotReduction: %v", err)
	}
	spot := f.add(spotLeg(t, 11, "1"))
	if _, err := f.deleverer.SubmitSpotReduction(hedgeGroup(), ref(spot)); err != nil {
		t.Fatalf("SubmitSpotReduction: %v", err)
	}

	if got, want := f.journal.find(t, "perp").order, perp.Order.ClientOrderID; got != want {
		t.Errorf("perp record names order %q, want %q", got, want)
	}
	if got, want := f.journal.find(t, "spot").order, spot.Order.ClientOrderID; got != want {
		t.Errorf("spot record names order %q, want %q", got, want)
	}
	// The two records name DIFFERENT orders, which is what makes the assertions
	// above two claims rather than one: a journal that wrote the perp id under
	// both kinds would satisfy either check alone.
	if f.journal.find(t, "perp").order == f.journal.find(t, "spot").order {
		t.Errorf("both leg records name %q", f.journal.find(t, "perp").order)
	}
	if legs := g.OwnLegs(); len(legs) != 2 {
		t.Errorf("OwnLegs = %v, want both legs", legs)
	}
}

// TestADissolvedPremiseSupersedesTheGenerationRatherThanRefusingTheStep is model
// (a)'s NC23 — P5 — and it is the load-bearing test of this file.
//
// "The obligation must be DISCHARGED, not merely permitted: an implementation
// that makes the supersession possible on the next event and never guarantees it
// has a real stuck state." The counterexample is a generation halted at
// `PerpReduceFinal` with a genuinely dissolved premise, the dispatch budget
// spent so §2 cannot compensate either, and the one action that would end it
// available and never taken. It is a LIVENESS requirement, so no invariant finds
// it — which is why it is asserted here as "the generation ENDED", never as "the
// step was refused".
//
// The dissolution is through the RESIDUAL alone: an external filled sale leaves
// the group Determined and short by 0.5.
func TestADissolvedPremiseSupersedesTheGenerationRatherThanRefusingTheStep(t *testing.T) {
	t.Parallel()

	f := newDeleverer(t, neutralPackage(t)...)
	g := f.open("1")
	perp := f.add(perpLeg(t, 10, "1"))
	if _, err := f.deleverer.SubmitPerpReduction(hedgeGroup(), ref(perp)); err != nil {
		t.Fatalf("SubmitPerpReduction: %v", err)
	}
	if err := f.deleverer.RecordPerpFill(hedgeGroup(), qty(t, "1")); err != nil {
		t.Fatalf("RecordPerpFill: %v", err)
	}
	if err := f.deleverer.ConfirmPerpReduction(hedgeGroup()); err != nil {
		t.Fatalf("ConfirmPerpReduction: %v", err)
	}

	// Somebody else's reduction executes. The generation is at PerpReduceFinal —
	// the model's own halting state.
	f.add(externalSale(t, 20, "0.5"))

	err := f.deleverer.AuthorizeSpotReduction(hedgeGroup(), qty(t, "1"))
	if !errors.Is(err, domain.ErrGenerationPremiseDissolved) {
		t.Fatalf("AuthorizeSpotReduction = %v, want ErrGenerationPremiseDissolved", err)
	}
	// The step did not happen AND the generation ended. Asserting only the first
	// is what "merely permitted" looks like from the outside.
	if got := g.Phase(); got != domain.GenerationSuperseded {
		t.Fatalf("phase = %s, want superseded", got)
	}
	if got := g.SupersededBy(); got != domain.SupersededByPremiseLost {
		t.Fatalf("superseded by %s, want premise_lost", got)
	}
	if got := f.journal.find(t, "superseded").path; got != domain.SupersededByPremiseLost {
		t.Fatalf("supersession record carries path %s, want premise_lost", got)
	}
}

// TestReassessDischargesTheSupersessionOnAnEventThatIsNotALadderStep is the same
// obligation reached the other way.
//
// §6.7's three inputs move on events that are not this machine's steps at all —
// another producer releasing an entry, somebody else's fill, an interval
// collapsing — and the generation may be sitting at a phase whose next step is
// nobody's to take. The discharge cannot depend on the ladder being driven.
//
// The dissolution here is through the CLASS alone: an unresolved external order
// contributes an interval, so the residual stays ZERO and only the verdict moves.
// Paired with the test above, that separates the two conjuncts of Premise.Intact.
func TestReassessDischargesTheSupersessionOnAnEventThatIsNotALadderStep(t *testing.T) {
	t.Parallel()

	f := newDeleverer(t, neutralPackage(t)...)
	g := f.open("1")

	f.add(externalInFlight(t, 20, "0.5"))

	r, err := f.deleverer.Reassess(hedgeGroup())
	if err != nil {
		t.Fatalf("Reassess: %v", err)
	}
	if r.Intact {
		t.Fatalf("Reassess reported the premise intact over %s", r.Premise.Class)
	}
	if !r.Superseded {
		t.Fatalf("Reassess did not discharge the supersession: %+v", r)
	}
	// The residual is untouched — this dissolution is entirely in the verdict.
	if got := r.Premise.Residual.Decimal(); got != qtyDecimal(t, "0") {
		t.Fatalf("premise residual = %s, want 0 (this fixture dissolves the CLASS)", got)
	}
	if r.Premise.Class != domain.ClassificationAmbiguous {
		t.Fatalf("premise class = %s, want Ambiguous", r.Premise.Class)
	}
	// The premise-lost row RESUMES the ladder, and the two external rows do not.
	// Reporting it is what lets core §7.1 open a later generation once §2 has
	// compensated the residual — refusing would disable leverage reduction on a
	// neutral package for the rest of the process's life.
	if !r.ResumesLadder {
		t.Errorf("ResumesLadder = false for premise_lost")
	}
	if got := g.Phase(); got != domain.GenerationSuperseded {
		t.Fatalf("phase = %s, want superseded", got)
	}
}

// TestADissolvedPremiseWaitsForTheReservedLegRatherThanStrandingIt is §7's
// quiesce-first rule, and it is the ONE state in which the obligation is
// outstanding.
//
// All three supersession rows "cancel/quiesce the generation's reserved second
// leg to §6.7 finality FIRST", so a premise that dissolves while the spot leg is
// out at the venue must NOT end the generation — abandoning it there strands the
// leg, which is the harm the rule exists for. What makes this bounded rather
// than a stuck state is that the leg is a marketable IOC: the venue resolves it,
// and both phases that can report this reach `Complete` or `TerminalFlatten`.
func TestADissolvedPremiseWaitsForTheReservedLegRatherThanStrandingIt(t *testing.T) {
	t.Parallel()

	f := newDeleverer(t, neutralPackage(t)...)
	g := f.open("1")
	perp := f.add(perpLeg(t, 10, "1"))
	if _, err := f.deleverer.SubmitPerpReduction(hedgeGroup(), ref(perp)); err != nil {
		t.Fatalf("SubmitPerpReduction: %v", err)
	}
	if err := f.deleverer.RecordPerpFill(hedgeGroup(), qty(t, "1")); err != nil {
		t.Fatalf("RecordPerpFill: %v", err)
	}
	if err := f.deleverer.ConfirmPerpReduction(hedgeGroup()); err != nil {
		t.Fatalf("ConfirmPerpReduction: %v", err)
	}
	if err := f.deleverer.AuthorizeSpotReduction(hedgeGroup(), qty(t, "1")); err != nil {
		t.Fatalf("AuthorizeSpotReduction: %v", err)
	}
	spot := f.add(spotLeg(t, 11, "1"))
	if _, err := f.deleverer.SubmitSpotReduction(hedgeGroup(), ref(spot)); err != nil {
		t.Fatalf("SubmitSpotReduction: %v", err)
	}

	f.add(externalSale(t, 20, "0.5"))

	r, err := f.deleverer.Reassess(hedgeGroup())
	if err != nil {
		t.Fatalf("Reassess: %v", err)
	}
	if !r.AwaitingQuiesce {
		t.Fatalf("Reassess = %+v, want AwaitingQuiesce with the spot leg live", r)
	}
	if r.Superseded {
		t.Fatalf("Reassess superseded a generation whose reserved leg is still out")
	}
	if got := g.Phase(); got != domain.GenerationSpotReduceInFlight {
		t.Fatalf("phase = %s, want spot_reduce_in_flight", got)
	}

	// And the leg is not blocked from finishing. §7's boundary (a): sending or
	// completing an already-authorized leg is not premise-guarded, because
	// refusing does not prevent a commitment, it strands one.
	if err := f.deleverer.ConfirmSpotReduction(hedgeGroup(), qty(t, "1")); err != nil {
		t.Fatalf("ConfirmSpotReduction with a dissolved premise: %v", err)
	}
	if got := g.Phase(); got != domain.GenerationComplete {
		t.Fatalf("phase = %s, want complete", got)
	}

	// And the obligation is DISCHARGED by the ending, not left outstanding. A
	// generation at Complete has reached §6.7 finality and there is nothing left
	// to take further, so "the premise dissolved after the generation finished"
	// is an ordinary thing for a package to do — it is what §2's determined
	// branch then compensates. Reassess must report that rather than treat it as
	// a fault, or every completed generation on a moved package becomes an error
	// the caller has no action for.
	after, err := f.deleverer.Reassess(hedgeGroup())
	if err != nil {
		t.Fatalf("Reassess on an ended generation: %v", err)
	}
	if !after.Intact || after.AwaitingQuiesce || after.Superseded {
		t.Fatalf("Reassess on an ended generation = %+v, want nothing owed", after)
	}
}

// TestASupersededGenerationLeavesItsUnmatchedExposureVisible is the difference
// between retiring an entry and amending it to zero.
//
// Both leave the ledger subtracting nothing, and they are not the same act. A
// generation superseded after buying 1 perp and selling no spot has put a real
// +1 on the package — releasing the entry says so, and §2's determined branch
// then compensates it. Amending to zero first would report the same final ledger
// while asserting the exposure had gone somewhere, and the group would read flat
// while holding a leg.
//
// The perp leg is planted FILLED here, because the claim is about realized
// exposure: with an unfilled leg the residual is zero under both readings.
func TestASupersededGenerationLeavesItsUnmatchedExposureVisible(t *testing.T) {
	t.Parallel()

	f := newDeleverer(t, neutralPackage(t)...)
	f.open("1")
	perpRec := f.add(perpLeg(t, 10, "1"))
	if _, err := f.deleverer.SubmitPerpReduction(hedgeGroup(), ref(perpRec)); err != nil {
		t.Fatalf("SubmitPerpReduction: %v", err)
	}
	if err := f.deleverer.RecordPerpFill(hedgeGroup(), qty(t, "1")); err != nil {
		t.Fatalf("RecordPerpFill: %v", err)
	}
	// The venue's own record of the same fill: the buy executed.
	f.venueSees(leg(t, 10, perp(), domain.SideBuy, "1", "1", domain.StateFilled))

	// While the entry stands, the package still reads neutral — that IS the
	// reservation working.
	f.assertResidual("0")
	f.assertReserved("1")

	if err := f.deleverer.Supersede(hedgeGroup(), domain.SupersededByEmergencyClose); err != nil {
		t.Fatalf("Supersede: %v", err)
	}

	f.assertReserved("0")
	// The exposure the generation created is now VISIBLE, which is the whole
	// point: it is genuinely net long by the perp it bought back.
	f.assertResidual("1")
}

// TestOpenRefusesASecondGenerationOnALedgerThisProcessDidNotWrite is SM §7's
// "each generation is a bounded attempt made only after the prior generation
// reaches §6.7 finality", against the half an in-process map cannot see.
//
// The sibling entry here was never created by this Deleverer — it is what a
// previous process, or another wiring, leaves in the shared ledger. The in-memory
// guard is blind to it, and what catches it is Classifier.PremiseFor refusing at
// two PackageDelever entries, which only happens because Open RESERVES BEFORE it
// asks for the premise. Asked the other way round, the sibling would be a count
// of one and its entry would be silently netted out of this generation's premise.
func TestOpenRefusesASecondGenerationOnALedgerThisProcessDidNotWrite(t *testing.T) {
	t.Parallel()

	f := newDeleverer(t, neutralPackage(t)...)
	if _, err := f.classifier.Reserve(hedgeGroup(), "stale-pkg", domain.ProducerPackageDeleverSpot); err != nil {
		t.Fatalf("Reserve: %v", err)
	}

	_, err := f.deleverer.Open("gen-1", hedgeGroup(), "pkg-1", qty(t, "1"))
	if !errors.Is(err, application.ErrPremiseNettingAmbiguous) {
		t.Fatalf("Open = %v, want ErrPremiseNettingAmbiguous", err)
	}
	// And the refusal did not LEAK an entry. One left behind subtracts from
	// every later sizing for the life of the process, on behalf of a generation
	// that does not exist — so the count must be back to the stale one alone.
	if _, err := f.classifier.PremiseFor(hedgeGroup(), nil); err != nil {
		t.Fatalf("after the refusal, PremiseFor = %v, want the stale entry alone", err)
	}
	if len(f.journal.calls) != 0 {
		t.Errorf("a refused Open wrote %d generation records", len(f.journal.calls))
	}
}

// TestOpenRefusesASecondLiveGenerationOnTheSameGroup is the in-process half of
// the same gate. It names the generation and its phase, which the ledger-side
// refusal cannot.
func TestOpenRefusesASecondLiveGenerationOnTheSameGroup(t *testing.T) {
	t.Parallel()

	f := newDeleverer(t, neutralPackage(t)...)
	f.open("1")

	_, err := f.deleverer.Open("gen-2", hedgeGroup(), "pkg-2", qty(t, "1"))
	if !errors.Is(err, application.ErrGenerationAlreadyLive) {
		t.Fatalf("Open = %v, want ErrGenerationAlreadyLive", err)
	}
}

// TestOpenAdmitsANextGenerationOnceTheFirstHasEnded is the positive control for
// both refusals above, and it is core §7.1's ladder continuing: "Complete → the
// escalation ladder continues → Planned (next generation)".
//
// Without it, a Deleverer that refused every second Open would score full marks
// on the two tests above while disabling the mechanism §7 exists to run.
//
// # The ENDING KIND was a collapsed dimension, and #204 is what separated it
//
// This test used to end the first generation with an `EmergencyClosePackage`
// supersession, which was indistinguishable from any other ending for as long as
// nothing acted on SupersessionPath.ResumesLadder — the sentence it quotes is
// about `Complete`, and the fixture reached for whichever ending was cheapest to
// write. It is not indistinguishable now: an external arbitration CLOSES the
// ladder, so that fixture was asserting the opposite of what its own doc claims.
// The ladder-closing case is the test below.
func TestOpenAdmitsANextGenerationOnceTheFirstHasEnded(t *testing.T) {
	t.Parallel()

	f := newDeleverer(t, neutralPackage(t)...)
	f.open("1")
	f.completeWithNoSpotSale()
	if _, err := f.deleverer.Open("gen-2", hedgeGroup(), "pkg-2", qty(t, "1")); err != nil {
		t.Fatalf("Open after the first generation ended: %v", err)
	}
}

// TestOpenRefusesTheNextGenerationAfterAnEXTERNALSupersession is the mirror, and
// it is SM §7's ladder-resumption invariant on the LIVE path.
//
// The rule has two homes only in the sense that it fires in two situations — a
// process that saw the arbitration, and a process that reads it back off the log
// at §7.5 boot — and it is one predicate (`ResumesLadder`) consulted from one
// place (`Deleverer.BarLadder`). A version that held only across a restart would
// be a system that refuses what it has forgotten and permits what it remembers.
//
// Both external rows, because the answer is a property of the PATH: a bar keyed
// off "a supersession happened" passes with one of them present and is wrong
// about the third.
func TestOpenRefusesTheNextGenerationAfterAnExternalSupersession(t *testing.T) {
	t.Parallel()

	for _, path := range []domain.SupersessionPath{
		domain.SupersededByEmergencyClose,
		domain.SupersededByFenceReadyLost,
	} {
		t.Run(path.String(), func(t *testing.T) {
			t.Parallel()
			f := newDeleverer(t, neutralPackage(t)...)
			f.open("1")
			if err := f.deleverer.Supersede(hedgeGroup(), path); err != nil {
				t.Fatalf("Supersede: %v", err)
			}
			_, err := f.deleverer.Open("gen-2", hedgeGroup(), "pkg-2", qty(t, "1"))
			if !errors.Is(err, application.ErrGenerationLadderBarred) {
				t.Fatalf("Open after %s = %v, want ErrGenerationLadderBarred", path, err)
			}
			// The refusal is read BEFORE the reserve, so a barred Open leaves the
			// shared ledger exactly as it found it. A refusal that reserved first
			// would leave an entry behind for a generation that does not exist.
			f.assertReserved("0")
		})
	}
}

// TestASpotOrderMayNotBeSubmittedAsThePerpLeg is §7's perp-short-FIRST
// sequencing, checked where both facts are in hand.
//
// The order is on the right hedge group and carries the right quantity, so
// nothing else in the system objects — and what it does is sell spot before any
// perp reduction has confirmed, driving the transient net SHORT in the rally the
// generation was opened for. That is the one direction the whole ordering exists
// to avoid.
func TestASpotOrderMayNotBeSubmittedAsThePerpLeg(t *testing.T) {
	t.Parallel()

	f := newDeleverer(t, neutralPackage(t)...)
	f.open("1")
	wrong := f.add(spotLeg(t, 10, "1"))

	_, err := f.deleverer.SubmitPerpReduction(hedgeGroup(), ref(wrong))
	if !errors.Is(err, application.ErrGenerationLegWrongProduct) {
		t.Fatalf("SubmitPerpReduction with a spot order = %v, want ErrGenerationLegWrongProduct", err)
	}
	if f.placer.calls != 0 {
		t.Errorf("a refused leg reached the venue %d times", f.placer.calls)
	}
}

// TestALegFromOutsideTheHedgeGroupIsRefused closes the gap a mutation found: the
// group check in verifyLeg was unasserted, because every fixture in this file
// planted its legs on the group by construction.
//
// The harm is FM-9's, reached from the other end. The premise is computed by
// FOLDING the hedge group's orders and excluding this generation's own legs, so
// an order on another symbol is not in the fold at all — naming it as a leg
// therefore excludes nothing, and the generation goes on judging itself against
// a quantity that still contains its real leg. Nothing else objects: the order
// is well-formed, correctly sized and genuinely a perp.
func TestALegFromOutsideTheHedgeGroupIsRefused(t *testing.T) {
	t.Parallel()

	f := newDeleverer(t, neutralPackage(t)...)
	f.open("1")

	other := ids.VenueInstrument{
		Venue: "binance", Product: ids.ProductLinearPerp,
		Symbol: "ETHUSDT", Contract: ids.ContractLinear,
	}
	stray := leg(t, 10, other, domain.SideBuy, "1", "0", domain.StatePlanned)
	f.orders.records[stray.Order.ClientOrderID] = stray

	_, err := f.deleverer.SubmitPerpReduction(hedgeGroup(), ref(stray))
	if !errors.Is(err, domain.ErrOrderOutsideHedgeGroup) {
		t.Fatalf("SubmitPerpReduction with an off-group leg = %v, want ErrOrderOutsideHedgeGroup", err)
	}
	if f.placer.calls != 0 {
		t.Errorf("a refused leg reached the venue %d times", f.placer.calls)
	}
}

// TestTheSpotLegMustBeExactlyTheAuthorizedSize is the missizing rule, and the
// fixture deliberately takes the direction that LOOKS safe.
//
// A spot order SMALLER than the authorization sells less, which reads as
// conservative. It is not: the ledger entry carries the authorization, so a
// smaller order overstates outstanding_reserved_reductions, the residual reads
// more SHORT than the package is, and §2's determined branch BUYS — adding
// exposure in the middle of reducing it.
func TestTheSpotLegMustBeExactlyTheAuthorizedSize(t *testing.T) {
	t.Parallel()

	f := newDeleverer(t, neutralPackage(t)...)
	f.open("1")
	perp := f.add(perpLeg(t, 10, "1"))
	if _, err := f.deleverer.SubmitPerpReduction(hedgeGroup(), ref(perp)); err != nil {
		t.Fatalf("SubmitPerpReduction: %v", err)
	}
	if err := f.deleverer.RecordPerpFill(hedgeGroup(), qty(t, "1")); err != nil {
		t.Fatalf("RecordPerpFill: %v", err)
	}
	if err := f.deleverer.ConfirmPerpReduction(hedgeGroup()); err != nil {
		t.Fatalf("ConfirmPerpReduction: %v", err)
	}
	if err := f.deleverer.AuthorizeSpotReduction(hedgeGroup(), qty(t, "1")); err != nil {
		t.Fatalf("AuthorizeSpotReduction: %v", err)
	}

	small := f.add(spotLeg(t, 11, "0.5"))
	_, err := f.deleverer.SubmitSpotReduction(hedgeGroup(), ref(small))
	if !errors.Is(err, application.ErrGenerationLegMissized) {
		t.Fatalf("SubmitSpotReduction with an undersized leg = %v, want ErrGenerationLegMissized", err)
	}
}

// TestAZeroAuthorizationCompletesWithoutSendingAnything is §7's honest extreme: a
// perp reduction that confirmed nothing authorizes no spot sale.
//
// It is not a shortcut, and the two assertions are separate claims: nothing is
// sent, and the generation still reaches `Complete` — a machine that refused the
// zero case would leave the generation blocking §7's one-at-a-time gate for ever.
func TestAZeroAuthorizationCompletesWithoutSendingAnything(t *testing.T) {
	t.Parallel()

	f := newDeleverer(t, neutralPackage(t)...)
	g := f.open("1")
	perp := f.add(perpLeg(t, 10, "1"))
	if _, err := f.deleverer.SubmitPerpReduction(hedgeGroup(), ref(perp)); err != nil {
		t.Fatalf("SubmitPerpReduction: %v", err)
	}
	// No fill: the leg's interval collapsed with nothing executed.
	if err := f.deleverer.ConfirmPerpReduction(hedgeGroup()); err != nil {
		t.Fatalf("ConfirmPerpReduction: %v", err)
	}
	if err := f.deleverer.AuthorizeSpotReduction(hedgeGroup(), qty(t, "0")); err != nil {
		t.Fatalf("AuthorizeSpotReduction(0): %v", err)
	}

	before := f.placer.calls
	if _, err := f.deleverer.SubmitSpotReduction(hedgeGroup(), application.DispatchRef{}); err != nil {
		t.Fatalf("SubmitSpotReduction on a zero authorization: %v", err)
	}
	if f.placer.calls != before {
		t.Errorf("a zero authorization sent %d orders", f.placer.calls-before)
	}
	if got := g.Phase(); got != domain.GenerationComplete {
		t.Fatalf("phase = %s, want complete", got)
	}
	f.assertReserved("0")
}

// TestAZeroAuthorizationRefusesAnOrderID is FM-9 arriving through the netting
// rather than through the trigger: an id supplied for a leg that was never sent
// enters OwnLegs and removes an order this generation does not own from its own
// premise.
func TestAZeroAuthorizationRefusesAnOrderID(t *testing.T) {
	t.Parallel()

	f := newDeleverer(t, neutralPackage(t)...)
	f.open("1")
	perp := f.add(perpLeg(t, 10, "1"))
	if _, err := f.deleverer.SubmitPerpReduction(hedgeGroup(), ref(perp)); err != nil {
		t.Fatalf("SubmitPerpReduction: %v", err)
	}
	if err := f.deleverer.ConfirmPerpReduction(hedgeGroup()); err != nil {
		t.Fatalf("ConfirmPerpReduction: %v", err)
	}
	if err := f.deleverer.AuthorizeSpotReduction(hedgeGroup(), qty(t, "0")); err != nil {
		t.Fatalf("AuthorizeSpotReduction(0): %v", err)
	}

	stray := f.add(spotLeg(t, 11, "0"))
	_, err := f.deleverer.SubmitSpotReduction(hedgeGroup(), ref(stray))
	if !errors.Is(err, domain.ErrGenerationLegNotSubmitted) {
		t.Fatalf("SubmitSpotReduction with an id against a zero authorization = %v, want ErrGenerationLegNotSubmitted", err)
	}
}

// TestAnExternalSupersessionWaitsForTheReservedLeg is the quiesce-first rule on
// the two rows a caller CAN assert.
//
// The refusal is the rule: the caller's answer is to drive the leg to §6.7
// finality — for a marketable IOC, the venue's own doing — and supersede then.
// The row still fires afterwards against a COMPLETE generation, which is why the
// external paths are not guarded on the generation still being active.
func TestAnExternalSupersessionWaitsForTheReservedLeg(t *testing.T) {
	t.Parallel()

	f := newDeleverer(t, neutralPackage(t)...)
	g := f.open("1")
	perp := f.add(perpLeg(t, 10, "1"))
	if _, err := f.deleverer.SubmitPerpReduction(hedgeGroup(), ref(perp)); err != nil {
		t.Fatalf("SubmitPerpReduction: %v", err)
	}
	if err := f.deleverer.RecordPerpFill(hedgeGroup(), qty(t, "1")); err != nil {
		t.Fatalf("RecordPerpFill: %v", err)
	}
	if err := f.deleverer.ConfirmPerpReduction(hedgeGroup()); err != nil {
		t.Fatalf("ConfirmPerpReduction: %v", err)
	}
	if err := f.deleverer.AuthorizeSpotReduction(hedgeGroup(), qty(t, "1")); err != nil {
		t.Fatalf("AuthorizeSpotReduction: %v", err)
	}
	spot := f.add(spotLeg(t, 11, "1"))
	if _, err := f.deleverer.SubmitSpotReduction(hedgeGroup(), ref(spot)); err != nil {
		t.Fatalf("SubmitSpotReduction: %v", err)
	}

	err := f.deleverer.Supersede(hedgeGroup(), domain.SupersededByFenceReadyLost)
	if !errors.Is(err, domain.ErrGenerationLegNotQuiesced) {
		t.Fatalf("Supersede with the spot leg live = %v, want ErrGenerationLegNotQuiesced", err)
	}

	// Quiesced, the same row fires — against a generation that has COMPLETED.
	if err := f.deleverer.ConfirmSpotReduction(hedgeGroup(), qty(t, "1")); err != nil {
		t.Fatalf("ConfirmSpotReduction: %v", err)
	}
	if err := f.deleverer.Supersede(hedgeGroup(), domain.SupersededByFenceReadyLost); err != nil {
		t.Fatalf("Supersede after the quiesce: %v", err)
	}
	if got := g.SupersededBy(); got != domain.SupersededByFenceReadyLost {
		t.Fatalf("superseded by %s, want fence_ready_lost", got)
	}
}

// TestThePremiseLostRowIsNotSomethingACallerMayAssert keeps the third row this
// machine's own.
//
// It is discharged from the netted quantity that decides it, so admitting it as
// an argument would let a caller cancel a generation on nobody's evidence — at
// the moment its perp short is nearing liquidation, which is why the generation
// exists.
func TestThePremiseLostRowIsNotSomethingACallerMayAssert(t *testing.T) {
	t.Parallel()

	f := newDeleverer(t, neutralPackage(t)...)
	g := f.open("1")

	err := f.deleverer.Supersede(hedgeGroup(), domain.SupersededByPremiseLost)
	if !errors.Is(err, application.ErrPremiseLossNotExternal) {
		t.Fatalf("Supersede(premise_lost) = %v, want ErrPremiseLossNotExternal", err)
	}
	if got := g.Phase(); got != domain.GenerationPlanned {
		t.Fatalf("phase = %s, want planned — the refused row must not have ended it", got)
	}
}

// TestReassessLeavesAnIntactPremiseAlone is the positive control for every
// discharge above: a machine that superseded unconditionally would pass all of
// them.
func TestReassessLeavesAnIntactPremiseAlone(t *testing.T) {
	t.Parallel()

	f := newDeleverer(t, neutralPackage(t)...)
	g := f.open("1")
	perpRec := f.add(perpLeg(t, 10, "1"))
	if _, err := f.deleverer.SubmitPerpReduction(hedgeGroup(), ref(perpRec)); err != nil {
		t.Fatalf("SubmitPerpReduction: %v", err)
	}
	if err := f.deleverer.RecordPerpFill(hedgeGroup(), qty(t, "0.6")); err != nil {
		t.Fatalf("RecordPerpFill: %v", err)
	}
	// The leg is PARTIALLY filled and still out: 0.6 of 1 executed, so 0.4 of
	// interval remains. That is the state §2's PartiallyFilled case describes,
	// and it is the fixture FM-9 needs — a fully executed leg collapses its own
	// interval and a wholly unfilled one moves no quantity, and in either the raw
	// verdict and the netted premise could agree by accident.
	f.venueSees(leg(t, 10, perp(), domain.SideBuy, "1", "0.6", domain.StateSubmitted))

	// The generation's OWN perp leg is live at the venue's view, so the RAW
	// verdict is Ambiguous — §7 depends on that, since it is how §2 refuses every
	// compensating trade while the leg is out. The premise, net of the
	// generation's own legs, is untouched. That is FM-9.
	raw, err := f.classifier.Classify(hedgeGroup())
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if raw.Class != domain.ClassificationAmbiguous {
		t.Fatalf("raw class = %s, want Ambiguous — the fixture does not reach FM-9's condition", raw.Class)
	}

	r, err := f.deleverer.Reassess(hedgeGroup())
	if err != nil {
		t.Fatalf("Reassess: %v", err)
	}
	if !r.Intact {
		t.Fatalf("Reassess dissolved a premise over the generation's own leg: %+v", r)
	}
	if got := g.Phase(); got != domain.GenerationPerpReduceInFlight {
		t.Fatalf("phase = %s, want perp_reduce_in_flight", got)
	}
}

// TestNewDelevererRefusesEveryMissingCollaborator, with its positive control.
func TestNewDelevererRefusesEveryMissingCollaborator(t *testing.T) {
	t.Parallel()

	d, _, _, _ := newDispatcher(t)
	classifier, _ := newClassifier(t)
	journal := newFakeGenerationJournal(nil)

	if _, err := application.NewDeleverer(nil, classifier, journal); !errors.Is(err, application.ErrDispatcherMissing) {
		t.Errorf("NewDeleverer(nil, classifier, journal) = %v, want ErrDispatcherMissing", err)
	}
	if _, err := application.NewDeleverer(d, nil, journal); !errors.Is(err, application.ErrClassifierMissing) {
		t.Errorf("NewDeleverer(dispatcher, nil, journal) = %v, want ErrClassifierMissing", err)
	}
	if _, err := application.NewDeleverer(d, classifier, nil); !errors.Is(err, application.ErrGenerationJournalMissing) {
		t.Errorf("NewDeleverer(dispatcher, classifier, nil) = %v, want ErrGenerationJournalMissing", err)
	}
	if _, err := application.NewDeleverer(d, classifier, journal); err != nil {
		t.Errorf("NewDeleverer(dispatcher, classifier, journal) = %v, want a deleverer", err)
	}
}

// TestALadderStepOnAGroupWithNoGenerationIsRefused. Each step journals, so a
// no-op would write a record for a generation that does not exist.
func TestALadderStepOnAGroupWithNoGenerationIsRefused(t *testing.T) {
	t.Parallel()

	f := newDeleverer(t, neutralPackage(t)...)
	if err := f.deleverer.ConfirmPerpReduction(hedgeGroup()); !errors.Is(err, application.ErrNoLiveGeneration) {
		t.Errorf("ConfirmPerpReduction = %v, want ErrNoLiveGeneration", err)
	}
	if _, err := f.deleverer.Reassess(hedgeGroup()); !errors.Is(err, application.ErrNoLiveGeneration) {
		t.Errorf("Reassess = %v, want ErrNoLiveGeneration", err)
	}
	if len(f.journal.calls) != 0 {
		t.Errorf("refused steps wrote %d generation records", len(f.journal.calls))
	}
}

// TestBothLegsAreDispatchedAsSafety is §7.6's shed class.
//
// `PackageDelever` is core §5.3's foundation-originated command and is "exempt
// from stop-opening" — the same fact from the other side. A leg defaulted to
// Opening sits under the ~70% back-off ceiling and is shed during exactly the
// incident the reserve band exists for.
func TestBothLegsAreDispatchedAsSafety(t *testing.T) {
	t.Parallel()

	f := newDeleverer(t, neutralPackage(t)...)
	f.open("1")
	perp := f.add(perpLeg(t, 10, "1"))
	if _, err := f.deleverer.SubmitPerpReduction(hedgeGroup(), ref(perp)); err != nil {
		t.Fatalf("SubmitPerpReduction: %v", err)
	}
	if err := f.deleverer.RecordPerpFill(hedgeGroup(), qty(t, "1")); err != nil {
		t.Fatalf("RecordPerpFill: %v", err)
	}
	if err := f.deleverer.ConfirmPerpReduction(hedgeGroup()); err != nil {
		t.Fatalf("ConfirmPerpReduction: %v", err)
	}
	if err := f.deleverer.AuthorizeSpotReduction(hedgeGroup(), qty(t, "1")); err != nil {
		t.Fatalf("AuthorizeSpotReduction: %v", err)
	}
	spot := f.add(spotLeg(t, 11, "1"))
	if _, err := f.deleverer.SubmitSpotReduction(hedgeGroup(), ref(spot)); err != nil {
		t.Fatalf("SubmitSpotReduction: %v", err)
	}

	if len(f.placer.urgencies) != 2 {
		t.Fatalf("placed %d orders, want 2", len(f.placer.urgencies))
	}
	for i, u := range f.placer.urgencies {
		if u != ports.UrgencySafety {
			t.Errorf("leg %d dispatched as %s, want safety", i, u)
		}
	}
}

// TestAFailedJournalWriteLeavesTheGenerationWhereItWas is the check-journal-apply
// order, asserted at the step whose record recovery reads.
//
// The journal is append-only, so a record written for a phase the aggregate then
// refuses describes a decision that never took effect. The mirror claim is
// asserted here: when the WRITE fails, the aggregate must not have moved either,
// or the process would be running a generation the log does not know reached
// that phase.
func TestAFailedJournalWriteLeavesTheGenerationWhereItWas(t *testing.T) {
	t.Parallel()

	f := newDeleverer(t, neutralPackage(t)...)
	g := f.open("1")
	perp := f.add(perpLeg(t, 10, "1"))
	if _, err := f.deleverer.SubmitPerpReduction(hedgeGroup(), ref(perp)); err != nil {
		t.Fatalf("SubmitPerpReduction: %v", err)
	}
	if err := f.deleverer.RecordPerpFill(hedgeGroup(), qty(t, "1")); err != nil {
		t.Fatalf("RecordPerpFill: %v", err)
	}
	if err := f.deleverer.ConfirmPerpReduction(hedgeGroup()); err != nil {
		t.Fatalf("ConfirmPerpReduction: %v", err)
	}

	boom := errors.New("disk full")
	f.journal.failOn[domain.GenerationSpotReduceAuthorized] = boom

	if err := f.deleverer.AuthorizeSpotReduction(hedgeGroup(), qty(t, "1")); !errors.Is(err, boom) {
		t.Fatalf("AuthorizeSpotReduction = %v, want the journal's error", err)
	}
	if got := g.Phase(); got != domain.GenerationPerpReduceFinal {
		t.Fatalf("phase = %s, want perp_reduce_final — the aggregate moved past a failed fsync", got)
	}
	if got := g.Authorized().Decimal(); got != qtyDecimal(t, "0") {
		t.Fatalf("authorized = %s, want 0", got)
	}
}
