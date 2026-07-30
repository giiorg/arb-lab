package dispatch_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/engine/dispatch"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/events"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// The fixtures here are sized against the collapse checklist BEFORE the
// mutation battery, per the standing discipline:
//
//   - arbitration tests hold MULTIPLE sources ready at once — a fixture whose
//     sources are never simultaneously ready cannot see the arbitration at all;
//   - the safety-prio source sorts LAST ("ZZZ"), so "most urgent wins" and
//     "sorted-first wins" are different answers;
//   - rotation tests use THREE sources — with two, "rotate from the cursor" and
//     "pick the other one" are the same rule (the boot reverse-teardown lesson);
//   - mailboxes are depth 8 with NoCoalescing, so per-source FIFO is a claim
//     about a real queue rather than about a single slot.

var t0 = time.Unix(1_700_000_000, 0).UTC()

func perp(sym string) ids.VenueInstrument {
	return ids.VenueInstrument{
		Venue: "binance", Product: ids.ProductLinearPerp,
		Symbol: sym, Contract: ids.ContractLinear,
	}
}

// markSrc is a market-lane source; ctrlSrc a control-lane one. The private
// streams are not product-constrained, but a perp instrument keeps every
// fixture valid under SourceID.Validate either way.
func markSrc(sym string) events.SourceID {
	return events.SourceID{Instrument: perp(sym), Stream: events.StreamMark}
}

func ctrlSrc(sym string) events.SourceID {
	return events.SourceID{Instrument: perp(sym), Stream: events.StreamAccount}
}

// env builds a PRE-dispatch envelope: AcceptOrdinal stamped (ingestion did),
// IngressSeq zero (dispatch has not happened — a producer that set it would be
// minting the replay key).
func env(s events.SourceID, ordinal uint64, prio events.Prio) events.Envelope {
	return events.Envelope{
		AcceptOrdinal: ordinal,
		Source:        s,
		RecvTime:      t0.Add(time.Duration(ordinal) * time.Millisecond),
		Prio:          prio,
		Payload:       fmt.Sprintf("payload-%d", ordinal),
	}
}

// entry is one sink delivery: exactly one of e / f is set.
type entry struct {
	seq uint64
	e   *events.Envelope
	f   *clock.Fired
}

type recordingSink struct {
	entries  []entry
	envErr   error // returned by OnEnvelope when set
	timerErr error // returned by OnTimer when set
}

func (s *recordingSink) OnEnvelope(e events.Envelope) error {
	s.entries = append(s.entries, entry{seq: e.IngressSeq, e: &e})
	return s.envErr
}

func (s *recordingSink) OnTimer(seq uint64, f clock.Fired) error {
	s.entries = append(s.entries, entry{seq: seq, f: &f})
	return s.timerErr
}

type rig struct {
	t       *testing.T
	clk     *clock.TestClock
	control *events.ControlLane
	market  *events.MarketDataLane
	sink    *recordingSink
	d       *dispatch.Dispatcher
}

func newRig(t *testing.T) *rig {
	t.Helper()
	clk, err := clock.NewTestClock(t0, nil)
	if err != nil {
		t.Fatalf("NewTestClock: %v", err)
	}
	control, err := events.NewControlLane(16)
	if err != nil {
		t.Fatalf("NewControlLane: %v", err)
	}
	market, err := events.NewMarketDataLane(8, events.NoCoalescing)
	if err != nil {
		t.Fatalf("NewMarketDataLane: %v", err)
	}
	sink := &recordingSink{}
	d, err := dispatch.New(dispatch.Config{
		Control: control, Market: market, Timers: clk, Sink: sink,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return &rig{t: t, clk: clk, control: control, market: market, sink: sink, d: d}
}

func (r *rig) publish(e events.Envelope) {
	r.t.Helper()
	if err := r.market.Publish(e); err != nil {
		r.t.Fatalf("market publish: %v", err)
	}
}

func (r *rig) sendControl(e events.Envelope) {
	r.t.Helper()
	if err := r.control.Send(context.Background(), e); err != nil {
		r.t.Fatalf("control send: %v", err)
	}
}

// drain steps until nothing dispatches, failing the test on any error. Bounded,
// because a dispatcher bug must fail the suite rather than hang it — a test
// that hangs is not a test that fails.
func (r *rig) drain() []entry {
	r.t.Helper()
	for range 1000 {
		ok, err := r.d.Step()
		if err != nil {
			r.t.Fatalf("Step: %v", err)
		}
		if !ok {
			return r.sink.entries
		}
	}
	r.t.Fatal("drain did not terminate in 1000 steps")
	return nil
}

// ordinals extracts the AcceptOrdinal sequence of dispatched envelopes,
// skipping timers — the fixture gives every envelope a distinct ordinal, so
// this names the dispatch order in terms the fixture chose.
func ordinals(entries []entry) []uint64 {
	var out []uint64
	for _, en := range entries {
		if en.e != nil {
			out = append(out, en.e.AcceptOrdinal)
		}
	}
	return out
}

// --- construction ---

func TestNewRejectsEveryMissingInput(t *testing.T) {
	clk, _ := clock.NewTestClock(t0, nil)
	control, _ := events.NewControlLane(1)
	market, _ := events.NewMarketDataLane(1, events.NoCoalescing)
	sink := &recordingSink{}
	full := dispatch.Config{Control: control, Market: market, Timers: clk, Sink: sink}

	// Positive control first: the full config constructs.
	if _, err := dispatch.New(full); err != nil {
		t.Fatalf("New(full config) = %v, want nil", err)
	}

	cases := []struct {
		name string
		mut  func(dispatch.Config) dispatch.Config
		want error
	}{
		{"control", func(c dispatch.Config) dispatch.Config { c.Control = nil; return c }, dispatch.ErrControlLaneNil},
		{"market", func(c dispatch.Config) dispatch.Config { c.Market = nil; return c }, dispatch.ErrMarketLaneNil},
		{"timers", func(c dispatch.Config) dispatch.Config { c.Timers = nil; return c }, dispatch.ErrTimerSourceNil},
		{"sink", func(c dispatch.Config) dispatch.Config { c.Sink = nil; return c }, dispatch.ErrSinkNil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := dispatch.New(tc.mut(full)); !errors.Is(err, tc.want) {
				t.Errorf("New = %v, want %v", err, tc.want)
			}
		})
	}
}

// --- invariant 1: IngressSeq dense, monotonic, from 1, stamped only at dispatch ---

// TestSeqIsDenseAcrossKinds is also the suite's positive control: a well-formed
// mixed run — timers, control, market — dispatches everything, and the one
// counter covers all of it with no hole and no repeat.
func TestSeqIsDenseAcrossKinds(t *testing.T) {
	r := newRig(t)
	r.clk.SetTimer("t-1", t0, 0)
	r.sendControl(env(ctrlSrc("AAAUSDT"), 10, events.PrioNormal))
	r.publish(env(markSrc("AAAUSDT"), 20, events.PrioNormal))
	r.publish(env(markSrc("BBBUSDT"), 21, events.PrioNormal))

	entries := r.drain()
	if len(entries) != 4 {
		t.Fatalf("dispatched %d items, want 4", len(entries))
	}
	for i, en := range entries {
		if want := uint64(i + 1); en.seq != want {
			t.Errorf("entry %d has seq %d, want %d — the sequence must be dense from 1", i, en.seq, want)
		}
	}
	if got := r.d.LastSeq(); got != 4 {
		t.Errorf("LastSeq() = %d, want 4", got)
	}
	// Every dispatched envelope is post-stamp valid — the §3.5 boundary check
	// actually ran where it is placed.
	for i, en := range entries {
		if en.e != nil {
			if err := en.e.Validate(); err != nil {
				t.Errorf("entry %d fails Validate after dispatch: %v", i, err)
			}
		}
	}
}

func TestMintedIngressSeqIsRefusedAndBurnsNothing(t *testing.T) {
	r := newRig(t)
	minted := env(markSrc("AAAUSDT"), 1, events.PrioNormal)
	minted.IngressSeq = 5 // a producer contesting the replay key
	r.publish(minted)
	r.publish(env(markSrc("AAAUSDT"), 2, events.PrioNormal))

	ok, err := r.d.Step()
	if ok || !errors.Is(err, dispatch.ErrIngressSeqMinted) {
		t.Fatalf("Step = (%v, %v), want (false, ErrIngressSeqMinted)", ok, err)
	}
	if got := r.d.LastSeq(); got != 0 {
		t.Fatalf("LastSeq after refusal = %d, want 0 — a refusal must not burn a sequence", got)
	}

	// The offender was consumed (its source is not jammed) and the next
	// envelope takes seq 1: density holds through the refusal.
	ok, err = r.d.Step()
	if !ok || err != nil {
		t.Fatalf("Step after refusal = (%v, %v), want (true, nil)", ok, err)
	}
	if got := r.sink.entries[0].seq; got != 1 {
		t.Errorf("first dispatched seq = %d, want 1", got)
	}
}

// --- invariant 4 (the refusal half): zero AcceptOrdinal on an input-derived envelope ---

func TestZeroAcceptOrdinalIsRefused(t *testing.T) {
	r := newRig(t)
	e := env(markSrc("AAAUSDT"), 0, events.PrioNormal) // ordinal zero: "timer-derived" claimed by a lane input
	r.publish(e)

	ok, err := r.d.Step()
	if ok || !errors.Is(err, events.ErrAcceptOrdinalUnset) {
		t.Fatalf("Step = (%v, %v), want (false, ErrAcceptOrdinalUnset)", ok, err)
	}
	if got := r.d.LastSeq(); got != 0 {
		t.Errorf("LastSeq after refusal = %d, want 0", got)
	}
}

func TestMalformedEnvelopesAreRefusedAtTheDispatchBoundary(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*events.Envelope)
		want error
	}{
		{"no recv time", func(e *events.Envelope) { e.RecvTime = time.Time{} }, events.ErrRecvTimeUnset},
		{"nil payload", func(e *events.Envelope) { e.Payload = nil }, events.ErrPayloadNil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newRig(t)
			e := env(markSrc("AAAUSDT"), 1, events.PrioNormal)
			tc.mut(&e)
			r.publish(e)
			ok, err := r.d.Step()
			if ok || !errors.Is(err, tc.want) {
				t.Errorf("Step = (%v, %v), want (false, %v)", ok, err, tc.want)
			}
		})
	}
}

// --- invariant 2: an unready source never stalls ready ones ---

func TestUnreadySourcesDoNotStallReadyOnes(t *testing.T) {
	r := newRig(t)
	ready := markSrc("BBBUSDT")
	drainedEmpty := markSrc("AAAUSDT") // has a mailbox, currently empty — and sorts FIRST,
	// so a rule that waited on (or scanned into) the empty box would trip here

	r.publish(env(drainedEmpty, 1, events.PrioNormal))
	if _, ok := r.market.Consume(drainedEmpty); !ok {
		t.Fatal("fixture: could not drain AAAUSDT")
	}
	r.publish(env(ready, 2, events.PrioNormal))
	r.publish(env(ready, 3, events.PrioNormal))

	got := ordinals(r.drain())
	want := []uint64{2, 3}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("dispatched ordinals %v, want %v — the empty mailbox must not stall the ready one", got, want)
	}
}

// --- invariant 3: per-source FIFO through dispatch ---

func TestPerSourceFIFOSurvivesInterleaving(t *testing.T) {
	r := newRig(t)
	a, b := markSrc("AAAUSDT"), markSrc("BBBUSDT")
	// Both ready with a backlog — one queued envelope each would make "FIFO
	// within a source" vacuously true whatever the dispatcher did.
	r.publish(env(a, 1, events.PrioNormal))
	r.publish(env(a, 2, events.PrioNormal))
	r.publish(env(a, 3, events.PrioNormal))
	r.publish(env(b, 11, events.PrioNormal))
	r.publish(env(b, 12, events.PrioNormal))
	r.publish(env(b, 13, events.PrioNormal))

	got := ordinals(r.drain())
	// The full interleave is deterministic (round-robin from no cursor:
	// A, B, A, B, ...), and asserting it exactly also pins the rotation.
	want := []uint64{1, 11, 2, 12, 3, 13}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("dispatch order %v, want %v", got, want)
	}
}

// --- invariant 5: Prio constancy per source ---

func TestPrioEscalationIsRefused(t *testing.T) {
	r := newRig(t)
	s := markSrc("AAAUSDT")
	r.publish(env(s, 1, events.PrioNormal))

	if ok, err := r.d.Step(); !ok || err != nil {
		t.Fatalf("first dispatch = (%v, %v), want (true, nil)", ok, err)
	}

	// The motivating accident: a producer escalates on a WORSE reading, which
	// would reorder this source inside its own mailbox drain.
	r.publish(env(s, 2, events.PrioSafety))
	ok, err := r.d.Step()
	if ok || !errors.Is(err, dispatch.ErrPrioInconstant) {
		t.Fatalf("escalated Step = (%v, %v), want (false, ErrPrioInconstant)", ok, err)
	}
	if got := r.d.LastSeq(); got != 1 {
		t.Errorf("LastSeq after refusal = %d, want 1", got)
	}

	// And the other direction refuses too — constancy, not a floor.
	r.publish(env(s, 3, events.Prio(200)))
	if ok, err := r.d.Step(); ok || !errors.Is(err, dispatch.ErrPrioInconstant) {
		t.Errorf("demoted Step = (%v, %v), want (false, ErrPrioInconstant)", ok, err)
	}
}

func TestARefusedEnvelopeFixesNoPrio(t *testing.T) {
	r := newRig(t)
	s := markSrc("AAAUSDT")

	bad := env(s, 1, events.Prio(7))
	bad.Payload = nil // refused for its own reasons, at Prio 7
	r.publish(bad)
	if ok, err := r.d.Step(); ok || err == nil {
		t.Fatalf("Step on the bad envelope = (%v, %v), want a refusal", ok, err)
	}

	// A refusal never entered the consumed order, so it must bind nothing:
	// the source's constant is fixed by its first DISPATCHED envelope.
	r.publish(env(s, 2, events.Prio(9)))
	if ok, err := r.d.Step(); !ok || err != nil {
		t.Errorf("Step at a different Prio after a refusal = (%v, %v), want (true, nil)", ok, err)
	}
}

// --- arbitration: the kind rank ---

func TestDueTimersOutrankEveryEnvelope(t *testing.T) {
	r := newRig(t)
	// The envelope is SAFETY prio and the timers are not — if the rank between
	// kinds were a Prio comparison rather than timers-first, the envelope
	// would win and this fixture would see it.
	r.publish(env(markSrc("AAAUSDT"), 1, events.PrioSafety))
	r.sendControl(env(ctrlSrc("AAAUSDT"), 2, events.PrioSafety))
	// Three due timers with a deliberate (At, Prio, ID) tangle: latest At
	// last, and within the equal-At pair the lower Prio first.
	r.clk.SetTimer("z-late", t0.Add(-time.Second), 0) // earliest At wins despite the "z" id
	r.clk.SetTimer("b-second", t0, 3)
	r.clk.SetTimer("a-third", t0, 7) // higher Prio number loses to b-second despite the earlier id

	entries := r.drain()
	if len(entries) != 5 {
		t.Fatalf("dispatched %d items, want 5", len(entries))
	}
	wantTimers := []clock.TimerID{"z-late", "b-second", "a-third"}
	for i, want := range wantTimers {
		if entries[i].f == nil || entries[i].f.ID != want {
			t.Errorf("entry %d = %+v, want timer %q — all due timers drain first, in (At, Prio, ID) order", i, entries[i], want)
		}
	}
	if entries[3].e == nil || entries[3].e.Source.Stream != events.StreamAccount {
		t.Errorf("entry 3 = %+v, want the control envelope", entries[3])
	}
	if entries[4].e == nil || entries[4].e.Source.Stream != events.StreamMark {
		t.Errorf("entry 4 = %+v, want the market envelope", entries[4])
	}
}

func TestControlOutranksMarketWhateverThePrios(t *testing.T) {
	r := newRig(t)
	// Market head is PrioSafety, control head PrioNormal: a Prio-comparing
	// rank would dispatch the market envelope first. The lane rank must win.
	r.publish(env(markSrc("AAAUSDT"), 1, events.PrioSafety))
	r.sendControl(env(ctrlSrc("BBBUSDT"), 2, events.PrioNormal))

	entries := r.drain()
	if len(entries) != 2 {
		t.Fatalf("dispatched %d, want 2", len(entries))
	}
	if entries[0].e.Source.Stream != events.StreamAccount {
		t.Errorf("first dispatched = %v, want the control envelope", entries[0].e.Source)
	}
}

// --- arbitration: market Prio, then rotation ---

func TestMostUrgentMarketSourceWinsFromTheBackOfTheSortOrder(t *testing.T) {
	r := newRig(t)
	// The safety source sorts LAST. "Most urgent wins" and "sorted-first wins"
	// are different answers here, and the round-robin start would be AAAUSDT
	// too — three wrong rules, one fixture that distinguishes all of them.
	r.publish(env(markSrc("AAAUSDT"), 1, events.PrioNormal))
	r.publish(env(markSrc("MMMUSDT"), 2, events.PrioNormal))
	r.publish(env(markSrc("ZZZUSDT"), 3, events.PrioSafety))
	r.publish(env(markSrc("ZZZUSDT"), 4, events.PrioSafety))

	got := ordinals(r.drain())
	// ZZZ drains fully first (its class is alone at PrioSafety), then the
	// cursor sits at ZZZ and the normal class wraps: AAA, MMM.
	want := []uint64{3, 4, 1, 2}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("dispatch order %v, want %v", got, want)
	}
}

func TestEqualPrioSourcesRotate(t *testing.T) {
	r := newRig(t)
	// THREE sources, not two: with two, "rotate from the cursor" and "pick the
	// one you didn't just pick" are the same rule.
	a, b, c := markSrc("AAAUSDT"), markSrc("BBBUSDT"), markSrc("CCCUSDT")
	for i, s := range []events.SourceID{a, b, c} {
		r.publish(env(s, uint64(10*i+1), events.PrioNormal))
		r.publish(env(s, uint64(10*i+2), events.PrioNormal))
	}

	got := ordinals(r.drain())
	want := []uint64{1, 11, 21, 2, 12, 22}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("dispatch order %v, want %v — equal-Prio sources must rotate, not replay the sort order", got, want)
	}
}

// TestRotationDistinguishesSourcesBeyondSymbol is the checklist's find, added
// BEFORE the mutation battery: every other market fixture differs only in
// Symbol, so a comparator collapsed to symbol-only would pass all of them —
// and symbol collision across venues is not hypothetical, it is the actual
// §32c capture set (BTCUSDT on binance and bybit, spot and perp). Four sources
// sharing one symbol, differing pairwise in Venue, Product/Contract and
// Stream; a comparator that merges any pair makes the rotation skip or repeat
// a source, which the exact interleave sees.
func TestRotationDistinguishesSourcesBeyondSymbol(t *testing.T) {
	r := newRig(t)
	binancePerpBook := events.SourceID{Instrument: perp("BTCUSDT"), Stream: events.StreamBook}
	binancePerpMark := events.SourceID{Instrument: perp("BTCUSDT"), Stream: events.StreamMark}
	binanceSpotBook := events.SourceID{
		Instrument: ids.VenueInstrument{Venue: "binance", Product: ids.ProductSpot, Symbol: "BTCUSDT"},
		Stream:     events.StreamBook,
	}
	bybitPerpBook := events.SourceID{
		Instrument: ids.VenueInstrument{
			Venue: "bybit", Product: ids.ProductLinearPerp,
			Symbol: "BTCUSDT", Contract: ids.ContractLinear,
		},
		Stream: events.StreamBook,
	}

	// Sorted order under the five-field compare: venue first (binance < bybit),
	// then product ("linear_perp" < "spot"), then stream ("book" < "mark").
	inSortedOrder := []events.SourceID{binancePerpBook, binancePerpMark, binanceSpotBook, bybitPerpBook}
	for i, s := range inSortedOrder {
		r.publish(env(s, uint64(10*i+1), events.PrioNormal))
		r.publish(env(s, uint64(10*i+2), events.PrioNormal))
	}

	got := ordinals(r.drain())
	want := []uint64{1, 11, 21, 31, 2, 12, 22, 32}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("dispatch order %v, want %v — four same-symbol sources must stay four sources", got, want)
	}
}

// TestControlFIFOThroughDispatch is the mirror of the market FIFO test — the
// standing rule that a claim tested on one side gets tested on its symmetric
// side immediately. The control lane is ONE queue, so its order through
// dispatch is global send order, across sources.
func TestControlFIFOThroughDispatch(t *testing.T) {
	r := newRig(t)
	a, b := ctrlSrc("AAAUSDT"), ctrlSrc("BBBUSDT")
	r.sendControl(env(a, 1, events.PrioNormal))
	r.sendControl(env(b, 2, events.PrioNormal))
	r.sendControl(env(a, 3, events.PrioNormal))

	got := ordinals(r.drain())
	want := []uint64{1, 2, 3}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("control dispatch order %v, want %v — send order, across sources", got, want)
	}
}

// --- invariant 6: the dispatcher never advances time ---

func TestStepNeverAdvancesTime(t *testing.T) {
	r := newRig(t)
	r.clk.SetTimer("future", t0.Add(time.Hour), 0)

	// Nothing dispatchable: the timer is not due and both lanes are empty. A
	// dispatcher that "helped" by advancing to the next timer would dispatch
	// here; the contract is that the DRIVER owns that decision.
	for range 3 {
		ok, err := r.d.Step()
		if ok || err != nil {
			t.Fatalf("Step with nothing due = (%v, %v), want (false, nil)", ok, err)
		}
	}
	if got := r.clk.Now(); !got.Equal(t0) {
		t.Errorf("clock moved to %v, want %v — the dispatcher must never advance time", got, t0)
	}
	if got := r.clk.Pending(); got != 1 {
		t.Errorf("Pending() = %d, want 1 — the undue timer must survive", got)
	}

	// The driver advances; only then does the timer dispatch.
	if _, ok := r.clk.AdvanceToNextTimer(); !ok {
		t.Fatal("AdvanceToNextTimer found no timer")
	}
	if ok, err := r.d.Step(); !ok || err != nil {
		t.Errorf("Step after the driver advanced = (%v, %v), want (true, nil)", ok, err)
	}
}

// --- sink errors: the dispatch happened, the sequence is spent ---

func TestSinkErrorPropagatesWithTheSequenceSpent(t *testing.T) {
	r := newRig(t)
	boom := errors.New("sink refused")
	r.sink.envErr = boom
	r.publish(env(markSrc("AAAUSDT"), 1, events.PrioNormal))

	ok, err := r.d.Step()
	if !ok || !errors.Is(err, boom) {
		t.Fatalf("Step = (%v, %v), want (true, the sink's error)", ok, err)
	}
	if got := r.d.LastSeq(); got != 1 {
		t.Errorf("LastSeq = %d, want 1 — the item was dispatched; the stamp is spent", got)
	}

	r.sink.envErr = nil
	r.sink.timerErr = boom
	r.clk.SetTimer("t", t0, 0)
	ok, err = r.d.Step()
	if !ok || !errors.Is(err, boom) {
		t.Fatalf("timer Step = (%v, %v), want (true, the sink's error)", ok, err)
	}
	if got := r.d.LastSeq(); got != 2 {
		t.Errorf("LastSeq = %d, want 2", got)
	}
}

// --- coalescing composes: the dispatcher releases what the mailbox resolved ---

func TestDispatchReleasesTheCoalescedEnvelopeWhole(t *testing.T) {
	clk, err := clock.NewTestClock(t0, nil)
	if err != nil {
		t.Fatalf("NewTestClock: %v", err)
	}
	control, err := events.NewControlLane(4)
	if err != nil {
		t.Fatalf("NewControlLane: %v", err)
	}
	// Depth 2, not 1: with one slot, "merge against the newest" and "merge
	// against the oldest" are the same slot — the events-package lesson.
	market, err := events.NewMarketDataLane(2, func(events.SourceID) events.MergeFunc {
		return events.LatestWins
	})
	if err != nil {
		t.Fatalf("NewMarketDataLane: %v", err)
	}
	sink := &recordingSink{}
	d, err := dispatch.New(dispatch.Config{Control: control, Market: market, Timers: clk, Sink: sink})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	s := markSrc("AAAUSDT")
	for _, ord := range []uint64{1, 2, 3} {
		if err := market.Publish(env(s, ord, events.PrioNormal)); err != nil {
			t.Fatalf("publish %d: %v", ord, err)
		}
	}
	// LatestWins collapsed 1..3 into the newest. The dispatcher must release
	// that envelope WHOLE — its AcceptOrdinal is the coalescing lineage slice
	// B will persist, so a dispatcher that touched it would corrupt the trace.
	ok, err := d.Step()
	if !ok || err != nil {
		t.Fatalf("Step = (%v, %v), want (true, nil)", ok, err)
	}
	got := sink.entries[0].e
	if got.AcceptOrdinal != 3 {
		t.Errorf("dispatched ordinal %d, want 3 — the merged envelope, whole", got.AcceptOrdinal)
	}
	if got.IngressSeq != 1 {
		t.Errorf("IngressSeq = %d, want 1", got.IngressSeq)
	}
}

// --- concurrency: producers race the driver; -race is the assertion ---

func TestConcurrentProducersOneDriverIsRaceFree(t *testing.T) {
	r := newRig(t)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := range 200 {
			if err := r.market.Publish(env(markSrc("AAAUSDT"), uint64(i+1), events.PrioNormal)); err != nil {
				t.Errorf("publish: %v", err)
				return
			}
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := range 100 {
			if err := r.control.Send(context.Background(), env(ctrlSrc("BBBUSDT"), uint64(1000+i), events.PrioNormal)); err != nil {
				t.Errorf("send: %v", err)
				return
			}
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := range 50 {
			r.clk.SetTimer(clock.TimerID(fmt.Sprintf("t-%d", i)), t0, 0)
		}
	}()

	// The single driver pumps while producers publish. Bounded: termination
	// here is not the claim, memory safety under -race is.
	for range 5000 {
		if _, err := r.d.Step(); err != nil {
			t.Fatalf("Step: %v", err)
		}
	}
	wg.Wait()
	r.drain()
}
