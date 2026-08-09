package application_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/giiorg/arb-lab/internal/oms/application"
	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/apperror"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// This file checks core §7.1's margin-op DISPATCH path (task #215).
//
// # The collapse checklist, run before the assertions were written
//
//   - THE GATEWAY RECORDS THE JOURNAL'S DEPTH AT THE MOMENT IT IS CALLED. Without
//     it, "fsync then send" and "send then fsync" produce identical final states
//     — the record is present either way — and the ordering is the entire safety
//     property of this file.
//   - TWO INSTRUMENTS, because with one "latches THIS position" and "latches
//     everything" are the same behaviour, and §7.1's rule is per position.
//   - THE VERDICT-WRITE FAILURE IS FIXTURED, not only the success. The latch is
//     released on a DURABLE verdict, so a fixture where the write always succeeds
//     makes "release when concluded" and "release when journaled" the same rule.
//   - TWO SEQUENTIAL DISPATCHES ON ONE INSTRUMENT ASSERT DIFFERENT IDS. With one
//     dispatch, "mints an id" and "returns a constant" agree.
//   - THE BEFORE-STATE (100), THE DELTA (137.45) AND THE MAINTENANCE (10) ARE
//     THREE DIFFERENT NUMBERS, none a prefix or multiple of another, so a record
//     built from the wrong field of the reading fails on the value.
//   - THE WATERMARK IS ASSERTED AGAINST VENUE TIME, which the fixture's clock
//     puts testVenueOffset away from local time — the one difference that makes
//     a locally-stamped watermark visible.
//   - EVERY PRE-FSYNC REFUSAL ASSERTS BOTH that the journal is empty AND that the
//     gateway was never called. Either alone is satisfied by a use case that got
//     the order wrong in the other direction.

// fakeMarginGateway is ports.MarginGateway.
//
// It captures `journalDepth` — how many dispatch records existed when the call
// arrived — because that is the only observable difference between journaling
// before the send and journaling after it.
type fakeMarginGateway struct {
	journal *fakeMarginOpJournal
	err     error

	calls        []marginGatewayCall
	journalDepth []int
}

type marginGatewayCall struct {
	instrument ids.VenueInstrument
	amount     fixedpoint.Money
}

var _ ports.MarginGateway = (*fakeMarginGateway)(nil)

func (f *fakeMarginGateway) AddIsolatedMargin(instrument ids.VenueInstrument, amount fixedpoint.Money) error {
	f.calls = append(f.calls, marginGatewayCall{instrument: instrument, amount: amount})
	f.journalDepth = append(f.journalDepth, len(f.journal.dispatched))
	return f.err
}

// topUpDelta is the amount every dispatch below posts. It is not the
// before-state, not the maintenance, and not a round number.
const topUpDelta = "137.45"

// perpETH is the SECOND position. §7.1's serialization is per POSITION, and one
// instrument cannot tell that rule from a global lock.
func perpETH() ids.VenueInstrument {
	return ids.VenueInstrument{
		Venue: "binance", Product: ids.ProductLinearPerp,
		Symbol: "ETHUSDT", Contract: ids.ContractLinear,
	}
}

// marginDispatchIDs builds a deterministic id generator.
//
// A fixed entropy reader and the fixture's own clock, which is what
// ids.Generator documents as the §3.5 parity configuration: the ids are stable
// across runs, and they still DIFFER from each other, because the counter in
// rand_a advances within a millisecond.
func marginDispatchIDs(t *testing.T, clk clock.Clock) *ids.Generator {
	t.Helper()
	entropy := bytes.NewReader(bytes.Repeat([]byte{0xA5}, 4096))
	gen, err := ids.NewGenerator(clk, entropy)
	if err != nil {
		t.Fatalf("NewGenerator: %v", err)
	}
	return gen
}

type dispatchRig struct {
	dispatcher *application.MarginOpDispatcher
	journal    *fakeMarginOpJournal
	margin     *fakeMargin
	gateway    *fakeMarginGateway
}

// newDispatchRig wires a dispatcher over a live position on both instruments.
func newDispatchRig(t *testing.T) *dispatchRig {
	t.Helper()
	j := &fakeMarginOpJournal{}
	m := &fakeMargin{
		statuses: map[ids.VenueInstrument]ports.MarginReading{
			perp():    marginReadingOf(t, "100"),
			perpETH(): marginReadingOfInstrument(t, perpETH(), "250"),
		},
		held: map[ids.VenueInstrument]bool{perp(): true, perpETH(): true},
	}
	g := &fakeMarginGateway{journal: j}
	clk := marginOpClock(t)
	d, err := application.NewMarginOpDispatcher(j, m, g, clk, marginDispatchIDs(t, clk))
	if err != nil {
		t.Fatalf("NewMarginOpDispatcher: %v", err)
	}
	return &dispatchRig{dispatcher: d, journal: j, margin: m, gateway: g}
}

// marginReadingOfInstrument is marginReadingOf for an instrument other than
// perp(). The Instrument field is what the dispatcher checks the answer's scope
// against, so it cannot be hard-coded here.
func marginReadingOfInstrument(t *testing.T, vi ids.VenueInstrument, posted string) ports.MarginReading {
	t.Helper()
	return ports.MarginReading{
		Instrument:     vi,
		Maintenance:    marginTestMoney(t, "10"),
		PostedMargin:   marginTestMoney(t, posted),
		SizingQuantity: marginTestQuantity(t, "-0.25"),
		EntryPrice:     marginTestEntry(t, "64180.41"),
	}
}

// ---------------------------------------------------------------------------
// The happy path — and the ORDER of its steps
// ---------------------------------------------------------------------------

func TestDispatchJournalsTheOpBEFORECallingTheVenue(t *testing.T) {
	t.Parallel()

	rig := newDispatchRig(t)

	got, err := rig.dispatcher.Dispatch(perp(), marginTestMoney(t, topUpDelta))
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	if len(rig.gateway.calls) != 1 {
		t.Fatalf("the venue was called %d times, want exactly 1", len(rig.gateway.calls))
	}
	// The claim. §7.1 fsyncs the record "BEFORE dispatch", and a process that
	// died between the two writes must leave evidence that something was
	// attempted — which is the only thing that makes the op resolvable, since
	// neither venue offers an operation id to ask about.
	if rig.gateway.journalDepth[0] != 1 {
		t.Errorf("the venue was called with %d dispatch records journaled, want 1 — the fsync must precede the send",
			rig.gateway.journalDepth[0])
	}

	if len(rig.journal.dispatched) != 1 {
		t.Fatalf("journaled %d dispatch records, want 1", len(rig.journal.dispatched))
	}
	op := rig.journal.dispatched[0]
	if op.BeforePosted.Decimal() != "100" {
		t.Errorf("BeforePosted = %s, want the reading's POSTED margin 100", op.BeforePosted.Decimal())
	}
	if op.Delta.Decimal() != topUpDelta {
		t.Errorf("Delta = %s, want %s", op.Delta.Decimal(), topUpDelta)
	}
	// Venue time, not local. The fixture's clock puts them testVenueOffset
	// apart, and a watermark on the wrong clock anchors an evidence window that
	// is compared against venue-stamped history rows.
	if !op.Watermark.Equal(marginVenueNow(t)) {
		t.Errorf("Watermark = %v, want the VENUE instant %v", op.Watermark, marginVenueNow(t))
	}

	if got.Status != domain.MarginOpApplied {
		t.Errorf("Status = %s, want applied — the venue acknowledged the act", got.Status)
	}
	if !got.Journaled {
		t.Error("Journaled = false on a verdict the journal accepted")
	}
	if got.InFlight() {
		t.Error("InFlight = true after a durable verdict")
	}
	if len(rig.journal.resolved) != 1 || rig.journal.resolved[0].status != domain.MarginOpApplied {
		t.Errorf("resolutions = %+v, want one applied", rig.journal.resolved)
	}
}

// TestAnAcknowledgedTopUpIsAppliedWITHOUTConsultingTheEvidencePath is the
// slice's central decision, asserted as the absence of a second read.
//
// Running §7.1's evidence contract here would answer `pending` on a venue with an
// act history — the history lags independently of the margin figure, so it is
// silent inside the propagation bound (task #214) — which leaves no verdict, no
// journal write, and the position latched by its own successful remedy.
func TestAnAcknowledgedTopUpIsAppliedWITHOUTConsultingTheEvidencePath(t *testing.T) {
	t.Parallel()

	rig := newDispatchRig(t)
	before := len(rig.margin.reads)

	if _, err := rig.dispatcher.Dispatch(perp(), marginTestMoney(t, topUpDelta)); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	// Exactly ONE margin read: the before-state. A second would be the evidence
	// path running, and its answer inside the bound is `pending`.
	if got := len(rig.margin.reads) - before; got != 1 {
		t.Errorf("the margin surface was read %d times, want 1 — the before-state and nothing else", got)
	}
}

// ---------------------------------------------------------------------------
// §7.1's serialization — per POSITION, and released by DURABILITY
// ---------------------------------------------------------------------------

func TestASecondOpOnTheSamePositionIsRefusedWhileTheFirstIsUnresolved(t *testing.T) {
	t.Parallel()

	rig := newDispatchRig(t)
	// An effect that establishes nothing: the op stays in flight.
	rig.gateway.err = apperror.Transport("binance.AddIsolatedMargin", apperror.EffectUnknown, errors.New("timeout"))

	first, err := rig.dispatcher.Dispatch(perp(), marginTestMoney(t, topUpDelta))
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if first.Status != domain.MarginOpPending {
		t.Fatalf("Status = %s, want pending on an unknown effect", first.Status)
	}
	if !first.InFlight() {
		t.Fatal("InFlight = false on an op with no verdict")
	}

	_, err = rig.dispatcher.Dispatch(perp(), marginTestMoney(t, topUpDelta))
	if !errors.Is(err, application.ErrMarginOpInFlight) {
		t.Fatalf("second dispatch: got %v, want ErrMarginOpInFlight", err)
	}
	if len(rig.gateway.calls) != 1 {
		t.Errorf("the venue was called %d times, want 1 — the refusal must be pre-send", len(rig.gateway.calls))
	}
	if len(rig.journal.dispatched) != 1 {
		t.Errorf("journaled %d dispatch records, want 1 — the refusal must be pre-fsync", len(rig.journal.dispatched))
	}

	// The OTHER position is untouched. §7.1's rule is per position, and a global
	// lock would take the remedy away from every instrument at once.
	if _, err := rig.dispatcher.Dispatch(perpETH(), marginTestMoney(t, topUpDelta)); err != nil {
		t.Fatalf("Dispatch on the other position: %v", err)
	}
}

func TestTheLatchIsReleasedONLYWhenTheVerdictIsDURABLE(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name        string
		verdictFail bool
		wantSecond  error
	}{
		{"a journaled verdict frees the position", false, nil},
		{"a verdict the journal refused keeps it held", true, application.ErrMarginOpInFlight},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rig := newDispatchRig(t)

			// The first dispatch's verdict write is the thing under test, so the
			// failure is scripted against whatever id gets minted.
			if tc.verdictFail {
				rig.journal.failEvery = errors.New("journal write refused")
			}

			first, err := rig.dispatcher.Dispatch(perp(), marginTestMoney(t, topUpDelta))
			if err != nil {
				t.Fatalf("Dispatch: %v", err)
			}
			if first.Status != domain.MarginOpApplied {
				t.Fatalf("Status = %s, want applied", first.Status)
			}
			if first.Journaled == tc.verdictFail {
				t.Fatalf("Journaled = %v with verdictFail = %v", first.Journaled, tc.verdictFail)
			}
			if first.InFlight() != tc.verdictFail {
				t.Errorf("InFlight = %v, want %v", first.InFlight(), tc.verdictFail)
			}

			// A second op is what would DESTROY the evidence an un-journaled
			// verdict must be reconstructed from: the posted margin would then
			// have moved by the sum of two deltas and match neither.
			rig.journal.failEvery = nil
			second, err := rig.dispatcher.Dispatch(perp(), marginTestMoney(t, topUpDelta))
			if tc.wantSecond != nil {
				if !errors.Is(err, tc.wantSecond) {
					t.Fatalf("second dispatch: got %v, want %v", err, tc.wantSecond)
				}
				return
			}
			if err != nil {
				t.Fatalf("second dispatch: %v", err)
			}
			// Two dispatches, two DIFFERENT ids. A constant would put two
			// records under one subject, which the replay refuses outright.
			if second.Op.ID == first.Op.ID {
				t.Errorf("both ops minted the id %q", first.Op.ID)
			}
			if second.Op.ID == "" {
				t.Error("the second op has no id")
			}
		})
	}
}

func TestInFlightOnNamesTheOpHoldingThePosition(t *testing.T) {
	t.Parallel()

	rig := newDispatchRig(t)
	rig.gateway.err = apperror.Transport("binance.AddIsolatedMargin", apperror.EffectUnknown, errors.New("timeout"))

	if _, held := rig.dispatcher.InFlightOn(perp()); held {
		t.Fatal("a position holds an op before anything was dispatched")
	}

	got, err := rig.dispatcher.Dispatch(perp(), marginTestMoney(t, topUpDelta))
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	id, held := rig.dispatcher.InFlightOn(perp())
	if !held || id != got.Op.ID {
		t.Errorf("InFlightOn = (%q, %v), want (%q, true)", id, held, got.Op.ID)
	}
	if _, held := rig.dispatcher.InFlightOn(perpETH()); held {
		t.Error("the other position reports an op in flight")
	}
}

// ---------------------------------------------------------------------------
// What each transport effect establishes
// ---------------------------------------------------------------------------

func TestTheEffectDecidesWhatMayBeConcluded(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name       string
		effect     apperror.Effect
		want       domain.MarginOpStatus
		wantWrite  bool
		wantLatch  bool
		wantReason string
	}{
		{
			name: "not applied concludes, where the order path cannot",
			// A margin op has ONE absence answer, so both causes
			// EffectNotApplied covers — never left the socket, rejected on its
			// merits — say the same thing: the margin did not move.
			effect: apperror.EffectNotApplied, want: domain.MarginOpNotApplied,
			wantWrite: true, wantLatch: false,
		},
		{
			name: "applied means the request arrived, NOT that the venue acted",
			// For an order this is StateSubmitted. Here there is no second
			// authority to ask, so concluding applied would be exactly the
			// assumption §7.1 calls dangerous.
			effect: apperror.EffectApplied, want: domain.MarginOpPending,
			wantWrite: false, wantLatch: true,
		},
		{
			name:   "unknown leaves it to the evidence path",
			effect: apperror.EffectUnknown, want: domain.MarginOpPending,
			wantWrite: false, wantLatch: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rig := newDispatchRig(t)
			rig.gateway.err = apperror.Transport("binance.AddIsolatedMargin", tc.effect, errors.New("scripted"))

			got, err := rig.dispatcher.Dispatch(perp(), marginTestMoney(t, topUpDelta))
			if err != nil {
				t.Fatalf("Dispatch: %v — a failed CALL is not a failed dispatch", err)
			}
			if got.Status != tc.want {
				t.Errorf("Status = %s, want %s", got.Status, tc.want)
			}
			if got.Cause == nil {
				t.Error("Cause = nil on a failed call")
			}
			if wrote := len(rig.journal.resolved) == 1; wrote != tc.wantWrite {
				t.Errorf("verdict written = %v, want %v (a pending must never reach the log)", wrote, tc.wantWrite)
			}
			if got.InFlight() != tc.wantLatch {
				t.Errorf("InFlight = %v, want %v", got.InFlight(), tc.wantLatch)
			}
			if _, held := rig.dispatcher.InFlightOn(perp()); held != tc.wantLatch {
				t.Errorf("the position is held = %v, want %v", held, tc.wantLatch)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Everything that refuses BEFORE the fsync
// ---------------------------------------------------------------------------

func TestEveryPreFsyncRefusalSendsAndJournalsNOTHING(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		setup func(t *testing.T, rig *dispatchRig) (ids.VenueInstrument, fixedpoint.Money)
		want  error
	}{
		{
			name: "a spot instrument has no position to post margin against",
			setup: func(t *testing.T, _ *dispatchRig) (ids.VenueInstrument, fixedpoint.Money) {
				return spot(), marginTestMoney(t, topUpDelta)
			},
			want: domain.ErrMarginOpNotPerp,
		},
		{
			name: "the venue reports no position",
			setup: func(t *testing.T, rig *dispatchRig) (ids.VenueInstrument, fixedpoint.Money) {
				rig.margin.held[perp()] = false
				return perp(), marginTestMoney(t, topUpDelta)
			},
			want: application.ErrMarginPositionFlat,
		},
		{
			name: "the before-state could not be read",
			setup: func(t *testing.T, rig *dispatchRig) (ids.VenueInstrument, fixedpoint.Money) {
				rig.margin.err = errors.New("venue would not answer")
				return perp(), marginTestMoney(t, topUpDelta)
			},
			want: nil, // any error; asserted as non-nil below
		},
		{
			name: "the margin read answered about another market",
			setup: func(t *testing.T, rig *dispatchRig) (ids.VenueInstrument, fixedpoint.Money) {
				rig.margin.statuses[perp()] = marginReadingOfInstrument(t, perpETH(), "100")
				return perp(), marginTestMoney(t, topUpDelta)
			},
			want: application.ErrMarginBeforeStateMismatch,
		},
		{
			name: "a zero delta is an op that moves nothing",
			setup: func(t *testing.T, _ *dispatchRig) (ids.VenueInstrument, fixedpoint.Money) {
				return perp(), marginTestMoney(t, "0")
			},
			want: domain.ErrMarginOpDeltaZero,
		},
		{
			name: "a delta in another asset than the before-state",
			setup: func(t *testing.T, _ *dispatchRig) (ids.VenueInstrument, fixedpoint.Money) {
				amount, err := fixedpoint.ParseMoney("BTC", topUpDelta)
				if err != nil {
					t.Fatalf("ParseMoney: %v", err)
				}
				return perp(), amount
			},
			want: domain.ErrMarginOpAssetsDisagree,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rig := newDispatchRig(t)
			instrument, amount := tc.setup(t, rig)

			_, err := rig.dispatcher.Dispatch(instrument, amount)
			if err == nil {
				t.Fatal("the dispatch was accepted")
			}
			if tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
			// The two halves of "nothing was sent". A journaled record with no
			// call is an op recovery will try to resolve for ever; a call with
			// no record is one nothing can resolve at all.
			if len(rig.journal.dispatched) != 0 {
				t.Errorf("journaled %d dispatch records on a refusal", len(rig.journal.dispatched))
			}
			if len(rig.gateway.calls) != 0 {
				t.Errorf("the venue was called %d times on a refusal", len(rig.gateway.calls))
			}
			if _, held := rig.dispatcher.InFlightOn(instrument); held {
				t.Error("a refused dispatch latched the position")
			}
		})
	}
}

func TestAFailedFsyncRefusesTheSendOutright(t *testing.T) {
	t.Parallel()

	rig := newDispatchRig(t)
	rig.journal.failDispatch = errors.New("fsync refused")

	_, err := rig.dispatcher.Dispatch(perp(), marginTestMoney(t, topUpDelta))
	if err == nil {
		t.Fatal("a top-up was dispatched over a failed fsync")
	}
	// The asymmetry that justifies the ordering: the journal is LOCAL, so a
	// failed write means what it says — unlike the network write below it.
	if len(rig.gateway.calls) != 0 {
		t.Errorf("the venue was called %d times after the record failed to become durable", len(rig.gateway.calls))
	}
	if _, held := rig.dispatcher.InFlightOn(perp()); held {
		t.Error("a dispatch that never happened latched the position")
	}
}

func TestAnUncalibratedVenueIsRefusedABOVETheFsync(t *testing.T) {
	t.Parallel()

	// A clock with no calibration for this venue answers the ZERO ServerTime
	// rather than an error, so the refusal has to be explicit — and it has to be
	// above the fsync, because a record with no watermark cannot bound its own
	// evidence window and the op it describes would already have been sent.
	clk, err := clock.NewTestClock(marginOpNow(), nil)
	if err != nil {
		t.Fatalf("NewTestClock: %v", err)
	}
	j := &fakeMarginOpJournal{}
	m := &fakeMargin{
		statuses: map[ids.VenueInstrument]ports.MarginReading{perp(): marginReadingOf(t, "100")},
		held:     map[ids.VenueInstrument]bool{perp(): true},
	}
	g := &fakeMarginGateway{journal: j}
	d, err := application.NewMarginOpDispatcher(j, m, g, clk, marginDispatchIDs(t, clk))
	if err != nil {
		t.Fatalf("NewMarginOpDispatcher: %v", err)
	}

	if _, err := d.Dispatch(perp(), marginTestMoney(t, topUpDelta)); err == nil {
		t.Fatal("a top-up was dispatched against an uncalibrated venue clock")
	}
	if len(j.dispatched) != 0 || len(g.calls) != 0 {
		t.Errorf("journaled %d records and made %d calls, want none", len(j.dispatched), len(g.calls))
	}
}

// ---------------------------------------------------------------------------
// Construction
// ---------------------------------------------------------------------------

func TestNewMarginOpDispatcherRefusesEveryMissingCollaborator(t *testing.T) {
	t.Parallel()

	j := &fakeMarginOpJournal{}
	m := &fakeMargin{}
	g := &fakeMarginGateway{journal: j}
	clk := marginOpClock(t)
	gen := marginDispatchIDs(t, clk)

	for _, tc := range []struct {
		name    string
		journal ports.MarginOpJournal
		margin  ports.VenueMarginStatus
		gateway ports.MarginGateway
		clk     clock.Clock
		ids     *ids.Generator
		want    error
	}{
		{"no journal", nil, m, g, clk, gen, application.ErrMarginOpJournalMissing},
		{"no margin read", j, nil, g, clk, gen, application.ErrVenueMarginStatusMissing},
		{"no gateway", j, m, nil, clk, gen, application.ErrMarginGatewayMissing},
		{"no clock", j, m, g, nil, gen, application.ErrClockMissing},
		{"no id source", j, m, g, clk, nil, application.ErrIDGeneratorMissing},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := application.NewMarginOpDispatcher(tc.journal, tc.margin, tc.gateway, tc.clk, tc.ids)
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}

	// The positive control: without it every refusal above is satisfied by a
	// constructor that refuses everything.
	if _, err := application.NewMarginOpDispatcher(j, m, g, clk, gen); err != nil {
		t.Fatalf("NewMarginOpDispatcher with everything wired: %v", err)
	}
}
