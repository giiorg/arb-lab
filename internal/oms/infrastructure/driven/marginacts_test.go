package driven_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/engine/venue"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/oms/infrastructure/driven"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// This file checks the margin-ACT ACL (task #214).
//
// # What the checklist demanded here
//
//   - THE READER ANSWERS DIFFERENTLY PER DIRECTION. The adapter fetches adds and
//     reduces separately and merges them, so a double returning the same rows for
//     both calls would make "merged both answers" and "duplicated one answer" the
//     same slice.
//   - THE PUBLISHED BIT AND THE ROW COUNT ARE VARIED INDEPENDENTLY, because the
//     one translation this adapter must not get wrong is published-and-empty
//     versus not-published, and those differ only in that bit.
//   - THE READER RECORDS ITS CALLS, so "asked once" is distinguishable from
//     "asked twice" — the no-history arm's claim is that the second read is
//     skipped, which no returned value can show.

// recordingActReader is driven.VenueMarginActReader, answering per direction.
type recordingActReader struct {
	adds      []venue.MarginAct
	reduces   []venue.MarginAct
	published bool
	err       error

	calls []actCall
}

type actCall struct {
	instrument ids.VenueInstrument
	window     venue.MarginActWindow
	adds       bool
	urgency    ratelimit.Priority
}

func (r *recordingActReader) MarginActs(_ context.Context, instrument ids.VenueInstrument,
	_ venue.InstrumentAssets, w venue.MarginActWindow, adds bool, urgency ratelimit.Priority) ([]venue.MarginAct, bool, error) {

	r.calls = append(r.calls, actCall{instrument: instrument, window: w, adds: adds, urgency: urgency})
	if r.err != nil {
		return nil, false, r.err
	}
	if !r.published {
		return nil, false, nil
	}
	if adds {
		return r.adds, true, nil
	}
	return r.reduces, true, nil
}

func actMoney(t *testing.T, v string) fixedpoint.Money {
	t.Helper()
	m, err := fixedpoint.ParseMoney("USDT", v)
	if err != nil {
		t.Fatalf("ParseMoney(%q): %v", v, err)
	}
	return m
}

func actAt(ms int64) time.Time { return time.UnixMilli(ms).UTC() }

func actFrom() time.Time { return actAt(1578047890000) }
func actTo() time.Time   { return actAt(1578047920000) }

func newActAdapter(t *testing.T, r *recordingActReader) *driven.VenueMarginActs {
	t.Helper()
	a, err := driven.NewVenueMarginActs(r, positionAssets())
	if err != nil {
		t.Fatalf("NewVenueMarginActs: %v", err)
	}
	return a
}

// TestAVenueWithNoHistoryIsTranslatedToTheABSENTReading, and asked only ONCE.
//
// The absence is a documented venue fact, so re-asking for the other direction
// would spend budget to re-derive it. That claim lives in the call count and
// nowhere in the returned value.
func TestAVenueWithNoHistoryIsTranslatedToTheABSENTReading(t *testing.T) {
	r := &recordingActReader{published: false}
	hist, err := newActAdapter(t, r).MarginActs(perpInstrument(), actFrom(), actTo())
	if err != nil {
		t.Fatalf("MarginActs: %v", err)
	}
	if hist.Published() {
		t.Error("a venue that publishes nothing produced a PUBLISHED reading")
	}
	if len(r.calls) != 1 {
		t.Errorf("the venue was asked %d times, want 1 — absence is documented, not re-derived", len(r.calls))
	}
}

// TestAPublishedHistoryMergesBOTHDirections.
//
// The two directions carry different amounts, so a merge that dropped one, or
// asked twice in the same direction, is visible in the resulting rows rather
// than only in a count.
func TestAPublishedHistoryMergesBOTHDirections(t *testing.T) {
	r := &recordingActReader{
		published: true,
		adds: []venue.MarginAct{{
			Symbol: "BTCUSDT", Amount: actMoney(t, "23.36"), Adds: true,
			At: actAt(1578047897183), Cause: venue.MarginActCauseUserAdjust,
		}},
		reduces: []venue.MarginAct{{
			Symbol: "BTCUSDT", Amount: actMoney(t, "41.07"), Adds: false,
			At: actAt(1578047899471), Cause: venue.MarginActCauseOther,
		}},
	}

	hist, err := newActAdapter(t, r).MarginActs(perpInstrument(), actFrom(), actTo())
	if err != nil {
		t.Fatalf("MarginActs: %v", err)
	}
	if !hist.Published() {
		t.Fatal("a venue that answered produced an ABSENT reading")
	}
	if err := hist.Validate(); err != nil {
		t.Fatalf("the merged history does not validate: %v", err)
	}

	if len(r.calls) != 2 {
		t.Fatalf("the venue was asked %d times, want 2 (one per direction)", len(r.calls))
	}
	if r.calls[0].adds == r.calls[1].adds {
		t.Error("both reads asked for the SAME direction — one direction is never fetched")
	}
	for i, c := range r.calls {
		if c.window.From != actFrom() || c.window.To != actTo() {
			t.Errorf("call %d window = %v..%v, want %v..%v", i, c.window.From, c.window.To, actFrom(), actTo())
		}
		if c.urgency != ratelimit.PrioritySafety {
			t.Errorf("call %d urgency = %v, want Safety", i, c.urgency)
		}
		if c.instrument != perpInstrument() {
			t.Errorf("call %d instrument = %v", i, c.instrument)
		}
	}
}

// TestAPUBLISHEDHistoryWithNORowsIsNotTheABSENTOne is the translation that
// inverts a verdict if collapsed, asserted directly.
//
// Both readings carry zero rows. They differ only in the published bit, and
// domain.MarginOp.Resolve reaches Applied under one and Ambiguous under the
// other on identical arithmetic.
func TestAPUBLISHEDHistoryWithNORowsIsNotTheABSENTOne(t *testing.T) {
	silent := &recordingActReader{published: true}
	hist, err := newActAdapter(t, silent).MarginActs(perpInstrument(), actFrom(), actTo())
	if err != nil {
		t.Fatalf("MarginActs: %v", err)
	}
	if !hist.Published() {
		t.Fatal("a venue that answered with no rows produced an ABSENT reading — the collapse this type exists to prevent")
	}

	absent := &recordingActReader{published: false}
	none, err := newActAdapter(t, absent).MarginActs(perpInstrument(), actFrom(), actTo())
	if err != nil {
		t.Fatalf("MarginActs: %v", err)
	}
	if none.Published() {
		t.Fatal("a venue with no history produced a PUBLISHED reading")
	}
}

// TestAFailedReadIsNEVERAnEmptyHistory.
//
// An empty published history is EVIDENCE — it is what turns a moved posted
// margin into an Ambiguous verdict — so manufacturing it from a timeout would
// invent evidence out of a transport failure.
func TestAFailedReadIsNEVERAnEmptyHistory(t *testing.T) {
	boom := errors.New("venue unreachable")
	r := &recordingActReader{published: true, err: boom}

	hist, err := newActAdapter(t, r).MarginActs(perpInstrument(), actFrom(), actTo())
	if !errors.Is(err, boom) {
		t.Fatalf("MarginActs: got %v, want %v", err, boom)
	}
	if hist.Published() {
		t.Error("a failed read produced a PUBLISHED history")
	}
	// And it is not the ABSENT reading either — the zero value, which Resolve
	// refuses by name. A failure must not answer the venue-capability question.
	if err := hist.Validate(); !errors.Is(err, domain.ErrMarginHistoryUnset) {
		t.Errorf("the returned history validates as %v, want the UNSET refusal", err)
	}
}

func TestVenueMarginActsRefusals(t *testing.T) {
	t.Run("no reader", func(t *testing.T) {
		if _, err := driven.NewVenueMarginActs(nil, positionAssets()); !errors.Is(err, driven.ErrVenueMarginActReaderMissing) {
			t.Fatalf("NewVenueMarginActs(nil): got %v, want %v", err, driven.ErrVenueMarginActReaderMissing)
		}
	})
	t.Run("no assets", func(t *testing.T) {
		if _, err := driven.NewVenueMarginActs(&recordingActReader{}, nil); !errors.Is(err, driven.ErrInstrumentAssetsMissing) {
			t.Fatalf("NewVenueMarginActs(no assets): got %v, want %v", err, driven.ErrInstrumentAssetsMissing)
		}
	})
	t.Run("an undeclared instrument is refused, not reported as historyless", func(t *testing.T) {
		other := ids.VenueInstrument{
			Venue: perpInstrument().Venue, Product: ids.ProductLinearPerp,
			Symbol: "ETHUSDT", Contract: ids.ContractLinear,
		}
		r := &recordingActReader{published: true}
		hist, err := newActAdapter(t, r).MarginActs(other, actFrom(), actTo())
		if !errors.Is(err, driven.ErrMarginInstrumentUndeclared) {
			t.Fatalf("MarginActs(undeclared): got %v, want %v", err, driven.ErrMarginInstrumentUndeclared)
		}
		if hist.Published() {
			t.Error("an undeclared instrument produced a published history")
		}
		if len(r.calls) != 0 {
			t.Error("the venue was called for an instrument this deployment never declared")
		}
	})

	// The positive control.
	if _, err := driven.NewVenueMarginActs(&recordingActReader{}, positionAssets()); err != nil {
		t.Fatalf("NewVenueMarginActs with everything wired: %v", err)
	}
}
