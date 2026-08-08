package driven

import (
	"errors"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// The margin-op journal's fixtures. Every stored number differs from every
// other, which is what makes a swapped column visible: the delta (25) is not the
// before-state (100), and the watermark is neither.
func marginTestInstrument() ids.VenueInstrument {
	return ids.VenueInstrument{
		Venue: "binance", Product: ids.ProductLinearPerp,
		Symbol: "BTCUSDT", Contract: ids.ContractLinear,
	}
}

func marginTestWatermark() time.Time {
	return time.Date(2026, 8, 8, 11, 59, 30, 0, time.UTC)
}

func testMoney(t *testing.T, asset, decimal string) fixedpoint.Money {
	t.Helper()
	m, err := fixedpoint.ParseMoney(fixedpoint.Asset(asset), decimal)
	if err != nil {
		t.Fatalf("ParseMoney(%s, %s): %v", asset, decimal, err)
	}
	return m
}

func testMarginOp(t *testing.T, id domain.MarginOpID) domain.MarginOp {
	t.Helper()
	return domain.MarginOp{
		ID:           id,
		Instrument:   marginTestInstrument(),
		Delta:        testMoney(t, "USDT", "25"),
		BeforePosted: testMoney(t, "USDT", "100"),
		Watermark:    marginTestWatermark(),
	}
}

// TestAMarginOpRoundTripsThroughTheChain is the durability claim: every field of
// core §7.1's record survives a write and a replay, and the chain covers all of
// them.
//
// The values are asserted individually rather than by comparing structs, because
// a struct comparison passes on the one shape that matters here — the delta and
// the before-state swapped. Both are Money in one asset, both non-negative in
// this fixture, so each is legal in the other's slot and only the numbers
// distinguish them.
func TestAMarginOpRoundTripsThroughTheChain(t *testing.T) {
	t.Parallel()
	j, path := openTestJournal(t)

	op := testMarginOp(t, "MOP-1")
	if err := j.AppendMarginOpDispatched(op); err != nil {
		t.Fatalf("AppendMarginOpDispatched: %v", err)
	}
	if err := j.AppendMarginOpResolved("MOP-1", domain.MarginOpAmbiguous); err != nil {
		t.Fatalf("AppendMarginOpResolved: %v", err)
	}

	// The independent oracle re-derives every preimage from the stored columns
	// and rehashes the whole chain. It shares no code with the encoders.
	verifyChain(t, oracleReplay(t, path))

	var v recordingVisitor
	if err := j.Replay(&v); err != nil {
		t.Fatalf("Replay: %v", err)
	}

	got := v.margin(t, kindMarginOpDispatched).op
	if got.ID != "MOP-1" {
		t.Errorf("id replayed as %q, want MOP-1", got.ID)
	}
	if got.Instrument != marginTestInstrument() {
		t.Errorf("instrument replayed as %+v, want %+v", got.Instrument, marginTestInstrument())
	}
	if want := op.Delta; got.Delta.Mantissa() != want.Mantissa() || got.Delta.Asset() != want.Asset() {
		t.Errorf("delta replayed as %s %s, want %s %s",
			got.Delta.Decimal(), got.Delta.Asset(), want.Decimal(), want.Asset())
	}
	if want := op.BeforePosted; got.BeforePosted.Mantissa() != want.Mantissa() {
		t.Errorf("before-state replayed as %s, want %s", got.BeforePosted.Decimal(), want.Decimal())
	}
	if !got.Watermark.Equal(marginTestWatermark()) {
		t.Errorf("watermark replayed as %s, want %s", got.Watermark, marginTestWatermark())
	}

	verdict := v.margin(t, kindMarginOpResolved)
	if verdict.id != "MOP-1" {
		t.Errorf("verdict replayed under %q, want MOP-1", verdict.id)
	}
	if verdict.status != domain.MarginOpAmbiguous {
		t.Errorf("verdict replayed as %s, want %s", verdict.status, domain.MarginOpAmbiguous)
	}
}

// TestAMarginOpResolutionFollowsItsDispatchInTheOneChain is what the single
// chain was chosen for, at this subject.
//
// The two records are ordered by the walk rather than by a discipline this
// package keeps, and the ordering is what lets a replay refuse a verdict for an
// op it has never seen. Asserted with an unrelated record BETWEEN them, so
// "follows" is not the same claim as "is the next record".
func TestAMarginOpResolutionFollowsItsDispatchInTheOneChain(t *testing.T) {
	t.Parallel()
	j, _ := openTestJournal(t)

	if err := j.AppendMarginOpDispatched(testMarginOp(t, "MOP-1")); err != nil {
		t.Fatalf("AppendMarginOpDispatched: %v", err)
	}
	if err := j.AppendGenerationOpened("GEN-1", testGroup("main", "binance", "BTCUSDT"),
		"RES-1", testQty(t, "BTC", "1")); err != nil {
		t.Fatalf("AppendGenerationOpened: %v", err)
	}
	if err := j.AppendMarginOpResolved("MOP-1", domain.MarginOpApplied); err != nil {
		t.Fatalf("AppendMarginOpResolved: %v", err)
	}

	var v recordingVisitor
	if err := j.Replay(&v); err != nil {
		t.Fatalf("Replay: %v", err)
	}
	dispatch := v.margin(t, kindMarginOpDispatched)
	verdict := v.margin(t, kindMarginOpResolved)
	if dispatch.seq >= verdict.seq {
		t.Fatalf("dispatch at seq %d, verdict at seq %d — the verdict must follow",
			dispatch.seq, verdict.seq)
	}
	// The generation record sits between them, which is what makes this an
	// ordering claim rather than an adjacency one.
	if verdict.seq-dispatch.seq != 2 {
		t.Fatalf("verdict is %d records after its dispatch, want 2", verdict.seq-dispatch.seq)
	}
}

// TestAppendMarginOpResolvedRefusesAPending is the port's one non-well-formedness
// check: a pending is not a verdict, and an append-only log turns a momentary
// condition into a permanent record.
//
// Paired with the positive controls the repo's rule requires — all three
// CONCLUDED statuses are accepted — because a method that refused everything
// would satisfy the refusal alone.
func TestAppendMarginOpResolvedRefusesAPending(t *testing.T) {
	t.Parallel()
	j, path := openTestJournal(t)

	for _, id := range []domain.MarginOpID{"MOP-A", "MOP-B", "MOP-C", "MOP-D"} {
		if err := j.AppendMarginOpDispatched(testMarginOp(t, id)); err != nil {
			t.Fatalf("AppendMarginOpDispatched(%s): %v", id, err)
		}
	}

	if err := j.AppendMarginOpResolved("MOP-A", domain.MarginOpPending); !errors.Is(err, ports.ErrMarginOpStatusNotConcluded) {
		t.Fatalf("AppendMarginOpResolved(pending) = %v, want ErrMarginOpStatusNotConcluded", err)
	}
	// The zero value and an undeclared spelling are refused by the status's own
	// Validate, which is a different error and a different fault: one is a caller
	// that reached no verdict, the other a caller that invented one.
	if err := j.AppendMarginOpResolved("MOP-A", domain.MarginOpUnresolved); !errors.Is(err, domain.ErrMarginOpStatusUnset) {
		t.Errorf("AppendMarginOpResolved(zero) = %v, want ErrMarginOpStatusUnset", err)
	}
	if err := j.AppendMarginOpResolved("MOP-A", domain.MarginOpStatus("resolved")); !errors.Is(err, domain.ErrMarginOpStatusInvalid) {
		t.Errorf("AppendMarginOpResolved(undeclared) = %v, want ErrMarginOpStatusInvalid", err)
	}
	if err := j.AppendMarginOpResolved("", domain.MarginOpApplied); !errors.Is(err, domain.ErrMarginOpIDUnset) {
		t.Errorf("AppendMarginOpResolved(no id) = %v, want ErrMarginOpIDUnset", err)
	}

	for i, status := range []domain.MarginOpStatus{
		domain.MarginOpApplied, domain.MarginOpNotApplied, domain.MarginOpAmbiguous,
	} {
		id := []domain.MarginOpID{"MOP-B", "MOP-C", "MOP-D"}[i]
		if err := j.AppendMarginOpResolved(id, status); err != nil {
			t.Errorf("AppendMarginOpResolved(%s): %v", status, err)
		}
	}

	verifyChain(t, oracleReplay(t, path))
}

// TestAppendMarginOpDispatchedRefusesAnUnreplayableRecord pins that the write end
// runs the record's own Validate.
//
// The refusals themselves are domain's and tested there; what this asserts is
// that they are REACHED here, which is the half a journal that validated nothing
// would fail while every domain test stayed green.
func TestAppendMarginOpDispatchedRefusesAnUnreplayableRecord(t *testing.T) {
	t.Parallel()
	j, _ := openTestJournal(t)

	cases := []struct {
		name string
		edit func(*domain.MarginOp)
		want error
	}{
		{"no id", func(o *domain.MarginOp) { o.ID = "" }, domain.ErrMarginOpIDUnset},
		{"zero delta", func(o *domain.MarginOp) { o.Delta = testMoney(t, "USDT", "0") }, domain.ErrMarginOpDeltaZero},
		{"no watermark", func(o *domain.MarginOp) { o.Watermark = time.Time{} }, domain.ErrMarginOpWatermarkUnset},
		{"assets disagree", func(o *domain.MarginOp) {
			o.BeforePosted = testMoney(t, "USDC", "100")
		}, domain.ErrMarginOpAssetsDisagree},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			op := testMarginOp(t, "MOP-BAD")
			tc.edit(&op)
			if err := j.AppendMarginOpDispatched(op); !errors.Is(err, tc.want) {
				t.Fatalf("AppendMarginOpDispatched: got %v, want %v", err, tc.want)
			}
		})
	}

	// The positive control.
	if err := j.AppendMarginOpDispatched(testMarginOp(t, "MOP-OK")); err != nil {
		t.Fatalf("AppendMarginOpDispatched of a well-formed record: %v", err)
	}
}

// TestAMarginOpAndAnOrderMayShareASpelling is the kind tag's claim at the third
// id space.
//
// A MarginOpID and a ClientOrderID share no namespace, so they can spell the
// same string. Both a state record and a resolution are "a subject followed by
// one length-prefixed string", so without the kind tag in the preimage the two
// would hash alike — and the chain would verify across a swap of one for the
// other.
func TestAMarginOpAndAnOrderMayShareASpelling(t *testing.T) {
	t.Parallel()
	j, path := openTestJournal(t)

	const shared = "APPLIED-1"

	o := testOrder(t, shared, "binance", "BTCUSDT")
	if err := j.AppendPlanned(o); err != nil {
		t.Fatalf("AppendPlanned: %v", err)
	}
	if err := j.AppendMarginOpDispatched(testMarginOp(t, shared)); err != nil {
		t.Fatalf("AppendMarginOpDispatched: %v", err)
	}
	if err := j.AppendMarginOpResolved(shared, domain.MarginOpApplied); err != nil {
		t.Fatalf("AppendMarginOpResolved: %v", err)
	}

	records := oracleReplay(t, path)
	verifyChain(t, records)

	// The two records that share a subject must not share a preimage, which is
	// the property the tag buys and the only one a chain verification cannot
	// report on its own.
	seen := map[string]string{}
	for _, r := range records {
		key := string(r.preimage)
		if other, dup := seen[key]; dup {
			t.Fatalf("records of kind %q and %q hash the same bytes", other, r.kind)
		}
		seen[key] = r.kind
	}
}
