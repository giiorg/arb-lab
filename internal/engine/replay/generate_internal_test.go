package replay

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/engine/dispatch"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/events"
)

// The canonical Generate run arms three timers around the archive's tail; the
// trio exercises both sides of every boundary the advance rule owns:
//
//	"mid"    at 43.5s — strictly between the chatter (43s) and the last
//	         record (44s): fires between them, and its position in the log
//	         is the §39p-10 "timer between two real messages" suspect.
//	"edge"   at 44.0s — EXACTLY the final record's instant: the timer-wins-
//	         equals tie, and the ≤-vs-< boundary on end-of-archive drain.
//	"beyond" at 44.0s+1h — strictly past the archive: must stay unfired,
//	         because beyond the archive there is no world to re-judge.
const (
	timerMid    = clock.TimerID("batt-mid")
	timerEdge   = clock.TimerID("batt-edge")
	timerBeyond = clock.TimerID("batt-beyond")
)

func armCanonicalTimers(clk *clock.TestClock) {
	clk.SetTimer(timerMid, at(tChat+500*time.Millisecond), 0)
	clk.SetTimer(timerEdge, at(tLast), 0)
	clk.SetTimer(timerBeyond, at(tLast+time.Hour), 0)
}

// runCanonicalGenerate is one fresh Generate world over the canonical archive,
// logging to logPath, with the canonical timers armed.
func runCanonicalGenerate(t *testing.T, root, logPath string) (Report, *recSink, *clock.TestClock, uint64) {
	t.Helper()
	tape, ing, clk, lane := newWorld(t, root, canonicalSpecs())
	sink := &recSink{clk: clk, lane: lane}
	logw, err := dispatch.NewLogWriter(logPath, sink)
	if err != nil {
		t.Fatalf("NewLogWriter: %v", err)
	}
	d := newDispatcher(t, lane, clk, logw)
	armCanonicalTimers(clk)
	rep, runErr := Generate(tape, ing, clk, lane, d, nil)
	if err := logw.Close(); err != nil {
		t.Fatalf("log close: %v", err)
	}
	if runErr != nil {
		t.Fatalf("Generate: %v", runErr)
	}
	return rep, sink, clk, d.LastSeq()
}

// TestGenerateCanonicalRun asserts the whole §39p-6 advance rule over one run:
// counts, ordinal inheritance through the pump, the timer trio's fates, the
// tie batch genuinely contending, and no residue.
func TestGenerateCanonicalRun(t *testing.T) {
	root := writeCanonicalArchive(t)
	logPath := filepath.Join(t.TempDir(), "run.dlog")
	rep, sink, clk, lastSeq := runCanonicalGenerate(t, root, logPath)

	// Seven records consumed, five emissions (§39p-2's gates), two timer
	// firings, zero failed polls: 5 + 2 = 7 dispatches, dense seq to 7.
	if rep.Ingested != 7 || rep.Emitted != 5 || rep.Dispatched != 7 || rep.Ordinary != 0 || rep.RestFailures != 0 {
		t.Errorf("report %+v, want Ingested 7, Emitted 5, Dispatched 7, Ordinary 0, RestFailures 0", rep)
	}
	if lastSeq != 7 {
		t.Errorf("LastSeq = %d, want 7", lastSeq)
	}

	// The delivered ordinals are the raw-arrival positions of the emitting
	// records — 2, 3, 4, 5, 7 — with 6 (the funding-silent chatter) burned
	// and absent. An ordinal of 6 appearing, or 7 arriving as 6, is the
	// skip-non-emitting mutant.
	var ordinals []uint64
	for _, d := range sink.got {
		if d.kind == "env" {
			ordinals = append(ordinals, d.ordinal)
		}
	}
	wantOrdinals := []uint64{2, 3, 4, 5, 7}
	if len(ordinals) != len(wantOrdinals) {
		t.Fatalf("delivered ordinals %v, want %v", ordinals, wantOrdinals)
	}
	// The 42.0s tie pair (ordinals 4 and 5) may arbitrate either way — the
	// arbitration's own tests own that choice — so positions 2 and 3 are
	// compared as a set and everything else exactly.
	if ordinals[0] != 2 || ordinals[1] != 3 || ordinals[4] != 7 {
		t.Errorf("delivered ordinals %v, want %v (tie pair unordered)", ordinals, wantOrdinals)
	}
	if !(ordinals[2] == 4 && ordinals[3] == 5) && !(ordinals[2] == 5 && ordinals[3] == 4) {
		t.Errorf("tie batch delivered ordinals %d,%d, want {4,5}", ordinals[2], ordinals[3])
	}

	// The tie batch must reach the arbitration as a genuine contention: when
	// its first envelope was delivered, the other source's mailbox still held
	// one. Without this witness, ingest-one-drain-fully — §39j's named
	// collapse — passes every other assertion in this test.
	var tieFirst *delivered
	for i := range sink.got {
		if sink.got[i].kind == "env" && sink.got[i].recv.Equal(at(tTie)) {
			tieFirst = &sink.got[i]
			break
		}
	}
	if tieFirst == nil {
		t.Fatal("no delivery at the tie instant")
	}
	if tieFirst.peers != 1 {
		t.Errorf("first tie delivery saw %d queued peers, want 1 — the batch never contended", tieFirst.peers)
	}

	// Every envelope was delivered with the clock AT its batch's instant —
	// §39p-10's SetNow-before-publish suspect: a driver that published first
	// and moved time after would satisfy every count above while every
	// freshness verdict computed in the sink chain read a stale Now.
	for i, d := range sink.got {
		if d.kind == "env" && !d.now.Equal(d.recv) {
			t.Errorf("delivery %d: clock at %v while dispatching an envelope received %v", i, d.now, d.recv)
		}
	}

	// The timer trio. "mid" fired at its own instant with the clock there;
	// "edge" fired at exactly the final instant, BEFORE the final record's
	// envelope; "beyond" stayed pending.
	var timers []delivered
	for _, d := range sink.got {
		if d.kind == "timer" {
			timers = append(timers, d)
		}
	}
	if len(timers) != 2 {
		t.Fatalf("%d timers fired, want 2 (mid, edge): %+v", len(timers), timers)
	}
	if timers[0].timerID != timerMid || !timers[0].firedAt.Equal(at(tChat+500*time.Millisecond)) {
		t.Errorf("first timer %+v, want %s at %v", timers[0], timerMid, at(tChat+500*time.Millisecond))
	}
	if !timers[0].now.Equal(timers[0].firedAt) {
		t.Errorf("mid fired with clock at %v, want %v — the clock must sit at the firing instant", timers[0].now, timers[0].firedAt)
	}
	if timers[1].timerID != timerEdge || !timers[1].firedAt.Equal(at(tLast)) {
		t.Errorf("second timer %+v, want %s at %v", timers[1], timerEdge, at(tLast))
	}
	last := sink.got[len(sink.got)-1]
	if last.kind != "env" || last.ordinal != 7 {
		t.Errorf("final delivery %+v, want the 44.0s envelope (ordinal 7) AFTER the edge timer — timer wins equals", last)
	}
	if got := clk.Pending(); got != 1 {
		t.Errorf("%d timers pending after the run, want 1 (beyond) — the archive's edge is the sim's edge", got)
	}
}

// TestGenerateIsDeterministic is the crafted-fixture half of the parity
// probe's claim: two fresh worlds over one archive write byte-identical logs.
// The production fixture proves it on real data; this one proves it on a
// fixture whose ties are FORCED, so a nondeterminism that ns-real data never
// exercises (map order over contending sources) has somewhere to fail.
func TestGenerateIsDeterministic(t *testing.T) {
	root := writeCanonicalArchive(t)
	dir := t.TempDir()
	log1, log2 := filepath.Join(dir, "run1.dlog"), filepath.Join(dir, "run2.dlog")
	runCanonicalGenerate(t, root, log1)
	runCanonicalGenerate(t, root, log2)
	b1, b2 := readFileBytes(t, log1), readFileBytes(t, log2)
	if string(b1) != string(b2) {
		t.Fatalf("two Generate runs wrote different logs (%d vs %d bytes)", len(b1), len(b2))
	}
	// Non-vacuity: the log the comparison rode contains the tie batch and
	// both fired timers.
	entries := readLogEntries(t, log1)
	if len(entries) != 7 {
		t.Fatalf("log holds %d entries, want 7", len(entries))
	}
}

// TestGenerateSinkErrorSplit: the predicate decides survival (§39m-4 — the
// stamp is spent either way). The erroring delivery is MID-RUN (ordinal 3,
// deterministically the second dispatch) rather than the last one, because on
// the last delivery "continue after an ordinary error" and "stop on it"
// produce the same report — the pinned-parameter collapse, avoided at design
// time here.
func TestGenerateSinkErrorSplit(t *testing.T) {
	errBoom := errors.New("boom")
	run := func(t *testing.T, ordinary func(error) bool) (Report, *recSink, error) {
		t.Helper()
		root := writeCanonicalArchive(t)
		tape, ing, clk, lane := newWorld(t, root, canonicalSpecs())
		sink := &recSink{clk: clk, lane: lane}
		sink.errOn = func(e events.Envelope) error {
			if e.AcceptOrdinal == 3 {
				return errBoom
			}
			return nil
		}
		d := newDispatcher(t, lane, clk, sink)
		rep, err := Generate(tape, ing, clk, lane, d, ordinary)
		return rep, sink, err
	}

	t.Run("ordinary continues", func(t *testing.T) {
		rep, sink, err := run(t, func(err error) bool { return errors.Is(err, errBoom) })
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		// No timers armed in this world: five envelope dispatches, the
		// ordinary one counted INSIDE the total — the stamp was spent.
		if rep.Ordinary != 1 || rep.Dispatched != 5 {
			t.Errorf("report %+v, want Ordinary 1 within Dispatched 5", rep)
		}
		if last := sink.got[len(sink.got)-1]; last.ordinal != 7 {
			t.Errorf("last delivery has ordinal %d, want 7 — the run must genuinely continue past the error", last.ordinal)
		}
	})
	t.Run("fatal stops", func(t *testing.T) {
		rep, sink, err := run(t, nil)
		if !errors.Is(err, errBoom) {
			t.Fatalf("err = %v, want errBoom", err)
		}
		if rep.Ordinary != 0 || rep.Dispatched != 1 {
			t.Errorf("report %+v, want Dispatched 1, Ordinary 0 — the run stopped where the ordinary run went on", rep)
		}
		if last := sink.got[len(sink.got)-1]; last.ordinal != 3 {
			t.Errorf("last delivery has ordinal %d, want 3 — nothing may dispatch past a fatal error", last.ordinal)
		}
	})
}
