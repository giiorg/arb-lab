package dispatch_test

// The dispatch log's battery. Fixtures are sized against the collapse
// checklist BEFORE the mutation battery (§39n-8):
//
//   - the round-trip log interleaves BOTH entry kinds — a log holding only
//     envelopes cannot see the k-discriminator or the timer-positions claim;
//   - envelopes appear WITH and WITHOUT a SourceTime, so absent-vs-present is
//     a distinction the fixture contains;
//   - the source set differs pairwise beyond Symbol (venue, product/contract,
//     stream) — the §32c capture set is the realistic accident;
//   - truncation lands MID-record and separately AT a record boundary — the
//     first is TornBytes' case, the second is the seal's, and a fixture with
//     only one cannot tell the two crash signals apart;
//   - vandalism lands in CRC-covered bytes AND in the frameLen field the CRC
//     does not cover;
//   - the WAL-order test drives a FAILING wrapped sink, without which
//     log-before-next and next-before-log are indistinguishable.

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/engine/capture"
	"github.com/giiorg/arb-lab/internal/engine/dispatch"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/events"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// --- fixture vocabulary ---

func bybitPerpSrc(sym string) events.SourceID {
	return events.SourceID{
		Instrument: ids.VenueInstrument{
			Venue: "bybit", Product: ids.ProductLinearPerp,
			Symbol: sym, Contract: ids.ContractLinear,
		},
		Stream: events.StreamMark,
	}
}

func spotBookSrc(sym string) events.SourceID {
	return events.SourceID{
		Instrument: ids.VenueInstrument{
			Venue: "binance", Product: ids.ProductSpot, Symbol: sym,
		},
		Stream: events.StreamBook,
	}
}

// logRecorder is the replay-side mirror of recordingSink.
type logRecorder struct {
	envs   []dispatch.EnvelopeRecord
	timers []dispatch.TimerRecord
	seqs   []uint64 // arrival order across both kinds

	failAtSeq uint64 // when nonzero, refuse the entry at this seq
	failWith  error
}

func (r *logRecorder) OnEnvelopeRecord(rec dispatch.EnvelopeRecord) error {
	if r.failAtSeq != 0 && rec.IngressSeq == r.failAtSeq {
		return r.failWith
	}
	r.envs = append(r.envs, rec)
	r.seqs = append(r.seqs, rec.IngressSeq)
	return nil
}

func (r *logRecorder) OnTimerRecord(rec dispatch.TimerRecord) error {
	if r.failAtSeq != 0 && rec.IngressSeq == r.failAtSeq {
		return r.failWith
	}
	r.timers = append(r.timers, rec)
	r.seqs = append(r.seqs, rec.IngressSeq)
	return nil
}

// newLogRig assembles the slice-A rig with a LogWriter wrapped around the
// recording sink, so every dispatched item is both logged and observed live.
func newLogRig(t *testing.T) (*rig, *dispatch.LogWriter, string) {
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
	inner := &recordingSink{}
	path := filepath.Join(t.TempDir(), "dispatch.log")
	w, err := dispatch.NewLogWriter(path, inner)
	if err != nil {
		t.Fatalf("NewLogWriter: %v", err)
	}
	d, err := dispatch.New(dispatch.Config{
		Control: control, Market: market, Timers: clk, Sink: w,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return &rig{t: t, clk: clk, control: control, market: market, sink: inner, d: d}, w, path
}

// pumpMixedRun drives the checklist's canonical run: two timers, one control
// envelope, three market sources differing pairwise beyond Symbol, SourceTime
// present on some envelopes and absent on others, a nonzero VenueSeq on one.
func pumpMixedRun(r *rig) {
	e1 := env(markSrc("BTCUSDT"), 1, events.PrioNormal)
	e1.VenueSeq = 4711
	e1.SourceTime = t0.Add(-3 * time.Millisecond)
	r.publish(e1)

	e2 := env(spotBookSrc("BTCUSDT"), 2, events.PrioNormal) // no SourceTime, VenueSeq zero
	r.publish(e2)

	e3 := env(ctrlSrc("BTCUSDT"), 3, events.PrioSafety)
	e3.SourceTime = t0.Add(-1 * time.Millisecond)
	r.sendControl(e3)

	r.clk.SetTimer("silence-judge", t0.Add(-time.Second), 0)
	r.clk.SetTimer("funding-boundary", t0, 5)

	r.publish(env(bybitPerpSrc("BTCUSDT"), 4, events.PrioNormal))
}

// writeRawLog builds a file from hand-assembled frames — the tests' second,
// independent spelling of the format, so a silent field rename in the payload
// schema fails here rather than round-tripping invisibly.
func writeRawLog(t *testing.T, frames ...capture.Record) string {
	t.Helper()
	body := capture.AppendFileHeader(nil)
	var err error
	for _, rec := range frames {
		body, err = capture.AppendRecord(body, rec)
		if err != nil {
			t.Fatalf("AppendRecord: %v", err)
		}
	}
	path := filepath.Join(t.TempDir(), "raw.log")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func dataFrame(seq uint64, at time.Time, payload string) capture.Record {
	return capture.Record{
		Kind: capture.RecordKindData, CaptureSeq: seq,
		RecvTime: at, Payload: []byte(payload),
	}
}

// envEntryJSON is a valid envelope entry spelled by hand. Field names here are
// the wire format; if this test breaks after a struct-tag change, the format
// changed, and §39n-3 says that is a new `k` token, not a rename.
const envEntryJSON = `{"k":"env","venue":"binance","product":"linear_perp",` +
	`"symbol":"BTCUSDT","contract":"linear","stream":"mark",` +
	`"accept":9,"venue_seq":0,"prio":128}`

// --- the round trip ---

func TestReplayReDrivesTheConsumedOrderExactly(t *testing.T) {
	r, w, path := newLogRig(t)
	pumpMixedRun(r)
	live := r.drain()
	if len(live) != 6 {
		t.Fatalf("live run dispatched %d items, want 6", len(live))
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	rec := &logRecorder{}
	rep, err := dispatch.ReplayLog(path, rec)
	if err != nil {
		t.Fatalf("ReplayLog: %v", err)
	}
	if !rep.Closed || rep.TornBytes != 0 {
		t.Errorf("report = %+v, want Closed with nothing torn", rep)
	}
	if rep.Entries != 6 || rep.LastSeq != 6 {
		t.Errorf("report = %+v, want 6 entries ending at seq 6", rep)
	}
	if len(rec.seqs) != 6 {
		t.Fatalf("consumer saw %d entries, want 6", len(rec.seqs))
	}

	// The replayed stream must match the live sink's view item by item — that
	// is the "identical consumer-visible order" the slice's done-when names.
	ei, ti := 0, 0
	for i, want := range live {
		if rec.seqs[i] != want.seq {
			t.Errorf("entry %d replayed seq %d, live saw %d", i, rec.seqs[i], want.seq)
		}
		switch {
		case want.f != nil:
			got := rec.timers[ti]
			ti++
			if got.IngressSeq != want.seq || got.Fired.ID != want.f.ID ||
				got.Fired.Prio != want.f.Prio || !got.Fired.At.Equal(want.f.At) {
				t.Errorf("timer %d = %+v, live fired %+v at seq %d", i, got, want.f, want.seq)
			}
		default:
			got := rec.envs[ei]
			ei++
			e := want.e
			if got.IngressSeq != e.IngressSeq || got.Source != e.Source ||
				got.AcceptOrdinal != e.AcceptOrdinal || got.VenueSeq != e.VenueSeq ||
				got.Prio != e.Prio {
				t.Errorf("envelope %d = %+v, live dispatched %+v", i, got, e)
			}
			if !got.RecvTime.Equal(e.RecvTime) {
				t.Errorf("envelope %d RecvTime = %v, want %v", i, got.RecvTime, e.RecvTime)
			}
			if got.SourceTime.IsZero() != e.SourceTime.IsZero() ||
				(!e.SourceTime.IsZero() && !got.SourceTime.Equal(e.SourceTime)) {
				t.Errorf("envelope %d SourceTime = %v, want %v — absent and present must both survive",
					i, got.SourceTime, e.SourceTime)
			}
		}
	}
}

func TestTwoIdenticalRunsProduceByteIdenticalLogs(t *testing.T) {
	run := func() []byte {
		r, w, path := newLogRig(t)
		pumpMixedRun(r)
		r.drain()
		if err := w.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	first, second := run(), run()
	if !bytes.Equal(first, second) {
		t.Fatalf("two identical runs wrote different bytes (%d vs %d) — the parity probe's "+
			"precondition is already false at the writer", len(first), len(second))
	}
}

// --- the writer's rules ---

func TestLogIsWrittenAheadOfTheWrappedSink(t *testing.T) {
	r, w, path := newLogRig(t)
	sentinel := errors.New("sink refused")
	r.sink.envErr = sentinel

	r.publish(env(markSrc("BTCUSDT"), 1, events.PrioNormal))
	dispatched, err := r.d.Step()
	if !dispatched || !errors.Is(err, sentinel) {
		t.Fatalf("Step = (%v, %v), want (true, the sink's error) — the stamp is spent", dispatched, err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	rec := &logRecorder{}
	rep, err := dispatch.ReplayLog(path, rec)
	if err != nil {
		t.Fatalf("ReplayLog: %v", err)
	}
	if rep.Entries != 1 || len(rec.envs) != 1 {
		t.Fatalf("the entry the sink refused is not in the log (entries=%d) — "+
			"the write must precede the sink call", rep.Entries)
	}
}

func TestWriterAssertsDensityInBothMethods(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dispatch.log")
	w, err := dispatch.NewLogWriter(path, &recordingSink{})
	if err != nil {
		t.Fatal(err)
	}
	stamped := func(seq uint64) events.Envelope {
		e := env(markSrc("BTCUSDT"), seq, events.PrioNormal)
		e.IngressSeq = seq
		return e
	}
	if err := w.OnEnvelope(stamped(5)); !errors.Is(err, dispatch.ErrLogSeqNotDense) {
		t.Errorf("a log starting at 5 = %v, want ErrLogSeqNotDense — a log that starts mid-run lies about where the order began", err)
	}
	if err := w.OnEnvelope(stamped(1)); err != nil {
		t.Fatalf("seq 1: %v", err)
	}
	if err := w.OnEnvelope(stamped(1)); !errors.Is(err, dispatch.ErrLogSeqNotDense) {
		t.Errorf("a repeated seq = %v, want ErrLogSeqNotDense", err)
	}
	if err := w.OnEnvelope(stamped(3)); !errors.Is(err, dispatch.ErrLogSeqNotDense) {
		t.Errorf("a skipped seq = %v, want ErrLogSeqNotDense", err)
	}
	// The mirror rule: OnTimer shares the assertion.
	if err := w.OnTimer(4, clock.Fired{ID: "x", At: t0, Prio: 0}); !errors.Is(err, dispatch.ErrLogSeqNotDense) {
		t.Errorf("a skipped timer seq = %v, want ErrLogSeqNotDense", err)
	}
	if err := w.OnTimer(2, clock.Fired{ID: "x", At: t0, Prio: 0}); err != nil {
		t.Errorf("timer at seq 2: %v", err)
	}
}

func TestWriterRefusalsNameTheirCases(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dispatch.log")
	if _, err := dispatch.NewLogWriter(path, nil); !errors.Is(err, dispatch.ErrSinkNil) {
		t.Errorf("nil sink = %v, want ErrSinkNil", err)
	}
	w, err := dispatch.NewLogWriter(path, &recordingSink{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := dispatch.NewLogWriter(path, &recordingSink{}); !errors.Is(err, fs.ErrExist) {
		t.Errorf("existing path = %v, want fs.ErrExist — fresh files only (§39n-5)", err)
	}

	// A timer scheduled for the zero time cannot exist (§11); the container's
	// own RecvTime refusal is the door it would hit.
	if err := w.OnTimer(1, clock.Fired{ID: "x", Prio: 0}); !errors.Is(err, capture.ErrRecvTimeUnset) {
		t.Errorf("zero-At timer = %v, want the container's RecvTime refusal", err)
	}

	// A SourceTime outside Unix-nanosecond range would wrap into a plausible
	// instant in the wrong century.
	e := env(markSrc("BTCUSDT"), 1, events.PrioNormal)
	e.IngressSeq = 1
	e.SourceTime = time.Date(3000, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := w.OnEnvelope(e); !errors.Is(err, dispatch.ErrLogTimeRange) {
		t.Errorf("year-3000 SourceTime = %v, want ErrLogTimeRange", err)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := w.Close(); !errors.Is(err, dispatch.ErrLogClosed) {
		t.Errorf("second Close = %v, want ErrLogClosed", err)
	}
	e2 := env(markSrc("BTCUSDT"), 1, events.PrioNormal)
	e2.IngressSeq = 1
	if err := w.OnEnvelope(e2); !errors.Is(err, dispatch.ErrLogClosed) {
		t.Errorf("OnEnvelope after Close = %v, want ErrLogClosed", err)
	}
	if err := w.OnTimer(1, clock.Fired{ID: "x", At: t0}); !errors.Is(err, dispatch.ErrLogClosed) {
		t.Errorf("OnTimer after Close = %v, want ErrLogClosed", err)
	}
}

func TestEmptyLogReplaysAsEmptyAndUnsealed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dispatch.log")
	w, err := dispatch.NewLogWriter(path, &recordingSink{})
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	rec := &logRecorder{}
	rep, err := dispatch.ReplayLog(path, rec)
	if err != nil {
		t.Fatalf("ReplayLog: %v", err)
	}
	if rep.Entries != 0 || rep.Closed || rep.TornBytes != 0 {
		t.Errorf("report = %+v, want empty, unsealed, nothing torn", rep)
	}
	if len(rec.seqs) != 0 {
		t.Errorf("consumer ran %d times on an empty log", len(rec.seqs))
	}
}

// --- the reader's refusals ---

// sealedLog writes a three-entry log through the real writer and returns its
// bytes plus each record's offset and length (header excluded).
func sealedLog(t *testing.T) (body []byte, offs []int64, lens []int) {
	t.Helper()
	r, w, path := newLogRig(t)
	r.publish(env(markSrc("BTCUSDT"), 1, events.PrioNormal))
	r.clk.SetTimer("mid", t0.Add(-time.Second), 0)
	r.publish(env(bybitPerpSrc("BTCUSDT"), 2, events.PrioNormal))
	r.drain()
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	off := int64(capture.FileHeaderLen)
	if _, err := capture.WalkRecords(f, func(_ capture.Record, raw []byte) error {
		offs = append(offs, off)
		lens = append(lens, len(raw))
		off += int64(len(raw))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(offs) != 4 { // three entries and the seal
		t.Fatalf("fixture log holds %d records, want 4", len(offs))
	}
	return body, offs, lens
}

func replayBytes(t *testing.T, body []byte) (dispatch.LogReport, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "replay.log")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	return dispatch.ReplayLog(path, &logRecorder{})
}

func TestReplayRefusesVandalismInsideAndOutsideTheChecksum(t *testing.T) {
	body, offs, lens := sealedLog(t)

	// A payload byte of the second record — inside the CRC's cover.
	vandal := append([]byte(nil), body...)
	vandal[offs[1]+int64(lens[1])-1] ^= 0xFF
	if _, err := replayBytes(t, vandal); !errors.Is(err, capture.ErrArchiveCorrupt) {
		t.Errorf("payload vandalism = %v, want the container's corruption refusal", err)
	}

	// The frameLen field of a NON-final record — the one header field OUTSIDE
	// the checksum (§39n-4's residual). Plausible-but-wrong here makes the CRC
	// span wrong, so the refusal still comes, as corruption rather than a tear.
	vandal = append([]byte(nil), body...)
	vandal[offs[1]+3]++ // low byte of frameLen: off by one, still plausible
	if _, err := replayBytes(t, vandal); !errors.Is(err, capture.ErrArchiveCorrupt) {
		t.Errorf("frameLen vandalism mid-file = %v, want corruption, not a tear", err)
	}
}

func TestReplayRecoversATornTailAndReportsIt(t *testing.T) {
	body, offs, _ := sealedLog(t)

	// Mid-record: seven bytes into the seal.
	rep, err := replayBytes(t, body[:offs[3]+7])
	if err != nil {
		t.Fatalf("a torn tail must recover, got: %v", err)
	}
	if rep.Entries != 3 || rep.TornBytes != 7 || rep.Closed {
		t.Errorf("mid-record tear: report = %+v, want 3 entries, 7 torn bytes, unsealed", rep)
	}

	// At a record boundary: the seal removed whole. TornBytes cannot see this
	// tear — the missing seal is the only crash signal left, which is why the
	// seal exists (§39n-4).
	rep, err = replayBytes(t, body[:offs[3]])
	if err != nil {
		t.Fatalf("a boundary tear must recover, got: %v", err)
	}
	if rep.Entries != 3 || rep.TornBytes != 0 || rep.Closed {
		t.Errorf("boundary tear: report = %+v, want 3 entries, nothing torn, UNSEALED", rep)
	}
}

func TestReplayRefusesRecordsAfterTheSeal(t *testing.T) {
	body, _, _ := sealedLog(t)
	extra, err := capture.AppendRecord(body, dataFrame(4, t0, envEntryJSON))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := replayBytes(t, extra); !errors.Is(err, dispatch.ErrLogRecordAfterSeal) {
		t.Errorf("record after seal = %v, want ErrLogRecordAfterSeal", err)
	}
}

func TestReplayCrossChecksTheSeal(t *testing.T) {
	entry := dataFrame(1, t0, envEntryJSON)
	seal := func(seq uint64, entries string) capture.Record {
		return capture.Record{
			Kind: capture.RecordKindClose, CaptureSeq: seq,
			RecvTime: t0, Payload: []byte(`{"entries":` + entries + `}`),
		}
	}

	cases := []struct {
		name string
		recs []capture.Record
	}{
		{"count disagrees", []capture.Record{entry, seal(1, "2")}},
		{"final seq disagrees", []capture.Record{entry, seal(2, "1")}},
		{"seal over zero entries", []capture.Record{seal(0, "0")}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeRawLog(t, tc.recs...)
			if _, err := dispatch.ReplayLog(path, &logRecorder{}); !errors.Is(err, dispatch.ErrLogSealMismatch) {
				t.Errorf("%s = %v, want ErrLogSealMismatch", tc.name, err)
			}
		})
	}

	// The agreeing seal is the positive control, or every refusal above is a
	// universally-objecting predicate.
	path := writeRawLog(t, entry, seal(1, "1"))
	rep, err := dispatch.ReplayLog(path, &logRecorder{})
	if err != nil || !rep.Closed {
		t.Errorf("an agreeing seal: report=%+v err=%v, want a clean sealed replay", rep, err)
	}
}

func TestReplayRefusesForeignKindsAndAHoleInTheSequence(t *testing.T) {
	alive := capture.Record{Kind: capture.RecordKindAlive, RecvTime: t0}
	path := writeRawLog(t, alive)
	if _, err := dispatch.ReplayLog(path, &logRecorder{}); !errors.Is(err, dispatch.ErrLogKind) {
		t.Errorf("an archive kind in a dispatch log = %v, want ErrLogKind — "+
			"the realistic accident is an archive file handed to the wrong reader", err)
	}

	path = writeRawLog(t, dataFrame(1, t0, envEntryJSON), dataFrame(3, t0, envEntryJSON))
	if _, err := dispatch.ReplayLog(path, &logRecorder{}); !errors.Is(err, dispatch.ErrLogSeqNotDense) {
		t.Errorf("a hole in an in-process log = %v, want ErrLogSeqNotDense — a hole here is never a drop", err)
	}
}

func TestReplayRefusesPayloadsThatAreNotSequenceRecords(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		want    error
	}{
		{"unknown token", `{"k":"quux"}`, dispatch.ErrLogEntryUnknown},
		{"not json", `not json`, dispatch.ErrLogEntryInvalid},
		{"invalid source", `{"k":"env","venue":"","product":"linear_perp","symbol":"BTCUSDT",` +
			`"contract":"linear","stream":"mark","accept":1,"venue_seq":0,"prio":128}`,
			dispatch.ErrLogEntryInvalid},
		{"timer with no id", `{"k":"timer","id":"","prio":0}`, dispatch.ErrLogEntryInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeRawLog(t, dataFrame(1, t0, tc.payload))
			if _, err := dispatch.ReplayLog(path, &logRecorder{}); !errors.Is(err, tc.want) {
				t.Errorf("%s = %v, want %v", tc.name, err, tc.want)
			}
		})
	}

	// Positive control: the hand-spelled valid entry replays. This is also the
	// format pin — a silent struct-tag rename fails HERE, not in a round trip
	// that re-derives both sides from the same renamed code.
	path := writeRawLog(t, dataFrame(1, t0, envEntryJSON))
	rec := &logRecorder{}
	if _, err := dispatch.ReplayLog(path, rec); err != nil {
		t.Fatalf("the hand-spelled entry did not replay: %v", err)
	}
	if len(rec.envs) != 1 || rec.envs[0].AcceptOrdinal != 9 ||
		rec.envs[0].Source != markSrc("BTCUSDT") {
		t.Errorf("hand-spelled entry decoded to %+v", rec.envs)
	}
}

func TestReplayConsumerErrorIsTheConsumersOwn(t *testing.T) {
	r, w, path := newLogRig(t)
	pumpMixedRun(r)
	r.drain()
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	sentinel := errors.New("consumer refused")
	rec := &logRecorder{failAtSeq: 3, failWith: sentinel}
	rep, err := dispatch.ReplayLog(path, rec)
	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want the consumer's own error, distinguishable from corruption", err)
	}
	if errors.Is(err, dispatch.ErrLogEntryInvalid) || errors.Is(err, capture.ErrArchiveCorrupt) {
		t.Errorf("consumer error %v also matches a corruption sentinel", err)
	}
	if rep.Entries != 2 {
		t.Errorf("report.Entries = %d, want the 2 delivered before the refusal", rep.Entries)
	}

	if _, err := dispatch.ReplayLog(path, nil); !errors.Is(err, dispatch.ErrLogConsumerNil) {
		t.Errorf("nil consumer = %v, want ErrLogConsumerNil", err)
	}
}
