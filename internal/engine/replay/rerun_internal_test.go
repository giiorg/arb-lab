package replay

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/engine/capture"
	"github.com/giiorg/arb-lab/internal/engine/dispatch"
	"github.com/giiorg/arb-lab/internal/engine/venue"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/events"
)

// runCanonicalRerun follows a log over an archive root with the canonical
// timers armed (or a caller-chosen arming), wrapping the sink in a fresh
// LogWriter so byte-equality is assertable.
func runCanonicalRerun(t *testing.T, root, follow, rewrite string, arm func(*clock.TestClock)) (Report, dispatch.LogReport, *recSink, error) {
	t.Helper()
	tape, ing, clk, lane := newWorld(t, root, canonicalSpecs())
	sink := &recSink{clk: clk, lane: lane}
	logw, err := dispatch.NewLogWriter(rewrite, sink)
	if err != nil {
		t.Fatalf("NewLogWriter: %v", err)
	}
	if arm != nil {
		arm(clk)
	}
	rep, lr, runErr := Rerun(tape, ing, clk, lane, logw, follow, nil)
	if err := logw.Close(); err != nil && runErr == nil {
		t.Fatalf("log close: %v", err)
	}
	return rep, lr, sink, runErr
}

// TestRerunReproducesTheRun is §39p-7's strongest form on the crafted fixture:
// the directed pump re-drives the generated log to a byte-identical rewrite,
// through the same clock trajectory — including the pre-ingest of the no-news
// chatter batch that sits strictly before the "mid" timer, which is the exact
// unit-level pin the production fixture found by failing (build fact 2: the
// lazy pull left that batch unconsumed and the next demanded batch sat in the
// clock's past).
func TestRerunReproducesTheRun(t *testing.T) {
	root := writeCanonicalArchive(t)
	dir := t.TempDir()
	genLog := filepath.Join(dir, "gen.dlog")
	rerunLog := filepath.Join(dir, "rerun.dlog")

	genRep, genSink, _, _ := runCanonicalGenerate(t, root, genLog)
	rep, lr, sink, err := runCanonicalRerun(t, root, genLog, rerunLog, armCanonicalTimers)
	if err != nil {
		t.Fatalf("Rerun: %v", err)
	}

	b1, b2 := readFileBytes(t, genLog), readFileBytes(t, rerunLog)
	if string(b1) != string(b2) {
		t.Fatalf("rerun rewrote a different log (%d vs %d bytes)", len(b1), len(b2))
	}
	if !lr.Closed || lr.LastSeq != 7 {
		t.Errorf("followed log report %+v, want Closed with LastSeq 7", lr)
	}
	if rep.Ingested != genRep.Ingested || rep.Emitted != genRep.Emitted || rep.Dispatched != genRep.Dispatched {
		t.Errorf("rerun report %+v differs from generate's %+v", rep, genRep)
	}
	// The sink saw the same deliveries in the same order with the clock at
	// the same instants — the parity claim at the observation layer, not
	// just the byte layer.
	if len(sink.got) != len(genSink.got) {
		t.Fatalf("rerun delivered %d, generate %d", len(sink.got), len(genSink.got))
	}
	for i := range sink.got {
		g, r := genSink.got[i], sink.got[i]
		if g.kind != r.kind || g.seq != r.seq || g.ordinal != r.ordinal || g.timerID != r.timerID || !g.now.Equal(r.now) {
			t.Errorf("delivery %d: generate %+v, rerun %+v", i, g, r)
		}
	}
}

// TestMatchRefusesEachFrozenField aims one case at each arm of the
// field-by-field cross-check. The base pair agrees on every field; each case
// then varies exactly one, so a dropped or blinded arm has a case that fails
// for it alone (§39p-10's "each cross-check field blinded" roster row).
func TestMatchRefusesEachFrozenField(t *testing.T) {
	src := events.SourceID{Instrument: bybitPerpInst(), Stream: events.StreamFunding}
	recv := at(10 * time.Second)
	stamp := at(10*time.Second - 15*time.Millisecond)
	base := func() (events.Envelope, dispatch.EnvelopeRecord) {
		e := events.Envelope{
			AcceptOrdinal: 5,
			Source:        src,
			SourceTime:    stamp,
			RecvTime:      recv,
			VenueSeq:      0,
			Prio:          events.PrioNormal,
			Payload:       venue.Funding{},
		}
		r := dispatch.EnvelopeRecord{
			IngressSeq:    3,
			Source:        src,
			AcceptOrdinal: 5,
			VenueSeq:      0,
			SourceTime:    stamp,
			RecvTime:      recv,
			Prio:          events.PrioNormal,
		}
		return e, r
	}

	e0, r0 := base()
	if err := match(e0, r0); err != nil {
		t.Fatalf("agreeing pair refused: %v", err)
	}

	cases := []struct {
		field string
		mut   func(*events.Envelope)
	}{
		{"accept ordinal", func(e *events.Envelope) { e.AcceptOrdinal = 6 }},
		{"recv time", func(e *events.Envelope) { e.RecvTime = recv.Add(time.Millisecond) }},
		{"source time", func(e *events.Envelope) { e.SourceTime = stamp.Add(time.Millisecond) }},
		{"venue seq", func(e *events.Envelope) { e.VenueSeq = 9 }},
		{"prio", func(e *events.Envelope) { e.Prio = events.PrioNormal + 1 }},
	}
	for _, c := range cases {
		e, r := base()
		c.mut(&e)
		err := match(e, r)
		if !errors.Is(err, ErrDiverged) {
			t.Errorf("%s: err = %v, want ErrDiverged", c.field, err)
			continue
		}
		if !strings.Contains(err.Error(), c.field) {
			t.Errorf("%s: refusal %q does not name the field", c.field, err)
		}
	}
}

// TestRerunRefusesDivergence drives the refusals end to end — the log and the
// archive genuinely disagreeing, not a hand-built pair.
func TestRerunRefusesDivergence(t *testing.T) {
	dir := t.TempDir()
	genRoot := writeCanonicalArchive(t)
	genLog := filepath.Join(dir, "gen.dlog")
	runCanonicalGenerate(t, genRoot, genLog)

	t.Run("ordinal shift", func(t *testing.T) {
		// The same world plus ONE extra funding-silent record at 40.5s: every
		// later ordinal shifts by one, and the first envelope the log demands
		// after the insertion arrives carrying the wrong ordinal.
		root := t.TempDir()
		writeStreamFile(t, root, binanceMarkKey(), []capture.Record{
			dataRec(1, at(tMark1), binanceMarkJSON(at(tMark1-10*time.Millisecond), "0.00010")),
			dataRec(2, at(tTie), binanceMarkJSON(at(tTie-10*time.Millisecond), "0.00011")),
		})
		writeStreamFile(t, root, binanceInfoKey(), []capture.Record{
			restRec(1, at(tPoll), restOKPayload(t, at(tPoll-50*time.Millisecond), fundingInfoBodyJSON)),
		})
		writeStreamFile(t, root, bybitTickerKey(), []capture.Record{
			dataRec(1, at(tSnap), bybitSnapJSON(at(tSnap-20*time.Millisecond), "0.00012")),
			dataRec(2, at(tSnap+500*time.Millisecond), bybitQuoteDeltaJSON(at(tSnap+500*time.Millisecond))),
			dataRec(3, at(tTie), bybitRateDeltaJSON(at(tTie-15*time.Millisecond), "0.00013")),
			dataRec(4, at(tChat), bybitChatterJSON()),
			dataRec(5, at(tLast), bybitRateDeltaJSON(at(tLast-15*time.Millisecond), "0.00014")),
		})
		_, _, _, err := runCanonicalRerun(t, root, genLog, filepath.Join(t.TempDir(), "re.dlog"), armCanonicalTimers)
		if !errors.Is(err, ErrDiverged) || !strings.Contains(err.Error(), "accept ordinal") {
			t.Errorf("err = %v, want ErrDiverged naming the accept ordinal", err)
		}
	})

	t.Run("exhausted tape", func(t *testing.T) {
		// The same world minus its final record: the log's last envelope entry
		// demands a head no remaining batch can produce.
		root := t.TempDir()
		writeStreamFile(t, root, binanceMarkKey(), []capture.Record{
			dataRec(1, at(tMark1), binanceMarkJSON(at(tMark1-10*time.Millisecond), "0.00010")),
			dataRec(2, at(tTie), binanceMarkJSON(at(tTie-10*time.Millisecond), "0.00011")),
		})
		writeStreamFile(t, root, binanceInfoKey(), []capture.Record{
			restRec(1, at(tPoll), restOKPayload(t, at(tPoll-50*time.Millisecond), fundingInfoBodyJSON)),
		})
		writeStreamFile(t, root, bybitTickerKey(), []capture.Record{
			dataRec(1, at(tSnap), bybitSnapJSON(at(tSnap-20*time.Millisecond), "0.00012")),
			dataRec(2, at(tTie), bybitRateDeltaJSON(at(tTie-15*time.Millisecond), "0.00013")),
			dataRec(3, at(tChat), bybitChatterJSON()),
		})
		_, _, _, err := runCanonicalRerun(t, root, genLog, filepath.Join(t.TempDir(), "re.dlog"), armCanonicalTimers)
		if !errors.Is(err, ErrDiverged) || !strings.Contains(err.Error(), "exhausted") {
			t.Errorf("err = %v, want ErrDiverged naming the exhausted tape", err)
		}
	})

	t.Run("timer nothing due", func(t *testing.T) {
		// The log froze "mid"; this run never arms it. A sink chain that arms
		// differently is a different run — refused, not repaired.
		arm := func(clk *clock.TestClock) {
			clk.SetTimer(timerEdge, at(tLast), 0)
			clk.SetTimer(timerBeyond, at(tLast+time.Hour), 0)
		}
		_, _, _, err := runCanonicalRerun(t, genRoot, genLog, filepath.Join(t.TempDir(), "re.dlog"), arm)
		if !errors.Is(err, ErrDiverged) || !strings.Contains(err.Error(), "nothing is due") {
			t.Errorf("err = %v, want ErrDiverged: nothing is due", err)
		}
	})

	t.Run("timer wrong identity", func(t *testing.T) {
		// A timer IS due at the frozen instant, but it is not the frozen one.
		arm := func(clk *clock.TestClock) {
			clk.SetTimer(clock.TimerID("someone-else"), at(tChat+500*time.Millisecond), 0)
			clk.SetTimer(timerEdge, at(tLast), 0)
			clk.SetTimer(timerBeyond, at(tLast+time.Hour), 0)
		}
		_, _, _, err := runCanonicalRerun(t, genRoot, genLog, filepath.Join(t.TempDir(), "re.dlog"), arm)
		if !errors.Is(err, ErrDiverged) || !strings.Contains(err.Error(), "someone-else") {
			t.Errorf("err = %v, want ErrDiverged naming the differing timer", err)
		}
	})
}
