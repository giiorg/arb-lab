package capture

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/events"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// §32l's fixture-collapse table for slice B, run BEFORE the mutations. Each row
// names the test that varies the parameter it says collapses:
//
//	one source in a writer test  -> TestTwoStreamsKeepSeparateSequencesAndTrees
//	one record per window        -> TestRotationCoversBusyAndIdleWindows
//	a drop of exactly 1          -> TestQueueDropNewest* (queue_internal_test.go)
//	truncation mid-payload only  -> TestTornTailAtEveryOffsetClass
//	alive_interval == rotation   -> testConfig uses 7m into a 60m window
//	a step smaller than spacing  -> TestClockStepRecordedAtBothMagnitudes
//	two writers both refused     -> TestSecondWriterIsRefusedAndTheFirstKeepsWriting
//
// What running the checklist added beyond that table:
//
//   - Every minted record would have echoed CaptureSeq 0 if the fixture minted
//     before any Data arrived, where "echoes the last data sequence" and "always
//     writes zero" are the same answer.
//   - Every file fitted in one read, where "the scanner reads until the file is
//     exhausted" and "the scanner decodes whatever one read returned" agree.
//     A file larger than scanBufInit separates them, and the wrong one truncates
//     a healthy archive down to its first 64 KiB.
//   - record_count in the Close record would have been the writer's LIFETIME
//     total if no fixture rotated, since the two agree for the first file.

const testStart = "2026-07-29T14:03:17Z"

func startInstant(t *testing.T) time.Time {
	t.Helper()
	at, err := time.Parse(time.RFC3339, testStart)
	if err != nil {
		t.Fatalf("parse start: %v", err)
	}
	return at
}

func TestFreshWriterWritesHeaderAndOpenRecord(t *testing.T) {
	c := newFakeClock(startInstant(t))
	w, err := OpenWriter(testConfig(t, perpKey(), c))
	if err != nil {
		t.Fatalf("OpenWriter: %v", err)
	}
	defer func() { _ = w.Close() }()

	recs := readAllRecords(t, w.Path())
	if got := kindsOf(recs); !slices.Equal(got, []RecordKind{RecordKindOpen}) {
		t.Fatalf("fresh file holds %v, want exactly one Open record", got)
	}

	var open openPayload
	if err := json.Unmarshal(recs[0].Payload, &open); err != nil {
		t.Fatalf("decode open payload: %v", err)
	}
	// The self-identification claim: a file moved out of its directory by a sync
	// script must still name its own stream, because a frame carries no
	// SourceID and the path is otherwise the only record of it.
	key := perpKey()
	if open.Symbol != key.Source.Instrument.Symbol || open.Topic != key.Topic ||
		open.Stream != string(key.Source.Stream) || open.Venue != string(key.Source.Instrument.Venue) {
		t.Errorf("open payload identifies %+v, want the writer's own stream %s", open, key)
	}
	if open.PrevLastSeq != 0 {
		t.Errorf("prev_last_seq = %d on a first file, want 0", open.PrevLastSeq)
	}
	if open.Provenance.Tool != "capture-test" || open.Provenance.GitSHA != "deadbeef" {
		t.Errorf("provenance = %+v, want the config's", open.Provenance)
	}
}

// TestTwoStreamsKeepSeparateSequencesAndTrees varies §32l's first row: one source
// in a writer test cannot tell a per-stream counter from one global counter.
func TestTwoStreamsKeepSeparateSequencesAndTrees(t *testing.T) {
	c := newFakeClock(startInstant(t))
	root := t.TempDir()

	open := func(k StreamKey) *Writer {
		cfg := testConfig(t, k, c)
		cfg.Root = root
		w, err := OpenWriter(cfg)
		if err != nil {
			t.Fatalf("OpenWriter(%s): %v", k, err)
		}
		t.Cleanup(func() { _ = w.Close() })
		return w
	}
	a, b := open(perpKey()), open(spotKey())

	// Interleaved, and each stream's sequence starts at 1 — which is what a
	// shared counter could not produce.
	for i := uint64(1); i <= 4; i++ {
		c.Advance(time.Second)
		if err := a.Data(dataRecord(i, c, "a")); err != nil {
			t.Fatalf("a.Data(%d): %v", i, err)
		}
		if err := b.Data(dataRecord(i, c, "b")); err != nil {
			t.Fatalf("b.Data(%d): %v", i, err)
		}
	}
	if a.Path() == b.Path() {
		t.Fatalf("both streams wrote to %s", a.Path())
	}
	for _, w := range []*Writer{a, b} {
		var seqs []uint64
		for _, r := range readAllRecords(t, w.Path()) {
			if r.Kind == RecordKindData {
				seqs = append(seqs, r.CaptureSeq)
			}
		}
		if want := []uint64{1, 2, 3, 4}; !slices.Equal(seqs, want) {
			t.Errorf("%s holds data sequences %v, want %v", w.Path(), seqs, want)
		}
	}
}

// TestRotationCoversBusyAndIdleWindows varies §32l's second row in both
// directions: several records in one window, and a window with no Data at all.
//
// The idle window is the load-bearing half. It only produces a file because
// Alive ticks are UNCONDITIONAL (§32g) — a tick written "when idle" would be
// indistinguishable here, and a tick written only alongside data would leave the
// hour with no file, which reads at scoring time as a quiet market rather than
// as a capture that was running.
func TestRotationCoversBusyAndIdleWindows(t *testing.T) {
	c := newFakeClock(startInstant(t))
	cfg := testConfig(t, perpKey(), c)
	w, err := OpenWriter(cfg)
	if err != nil {
		t.Fatalf("OpenWriter: %v", err)
	}

	seq := uint64(0)
	// Hour 14: several records, so "rotate on the hour" and "rotate per record"
	// give different answers.
	for range 5 {
		c.Advance(3 * time.Minute)
		seq++
		if err := w.Data(dataRecord(seq, c, "busy")); err != nil {
			t.Fatalf("Data: %v", err)
		}
	}
	firstFile := w.Path()

	// Hour 15: ticks only, no data.
	for range 12 {
		c.Advance(5 * time.Minute)
		if err := w.Tick(); err != nil {
			t.Fatalf("Tick: %v", err)
		}
	}
	idleFile := w.Path()

	// Hour 16: data again. The twelve five-minute ticks above land at 15:18, so
	// this has to clear the rest of hour 15 as well.
	c.Advance(45 * time.Minute)
	seq++
	if err := w.Data(dataRecord(seq, c, "busy again")); err != nil {
		t.Fatalf("Data: %v", err)
	}
	lastFile := w.Path()
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	files := archiveFiles(t, cfg.Root)
	if len(files) != 3 {
		t.Fatalf("rotation produced %d files (%v), want 3 — one per elapsed window", len(files), files)
	}
	for _, p := range []string{firstFile, idleFile, lastFile} {
		if !slices.Contains(files, p) {
			t.Errorf("%s is missing from the archive listing %v", p, files)
		}
	}

	// The idle window's file exists and holds no Data — the distinction the
	// unconditional tick buys.
	idle := readAllRecords(t, idleFile)
	if n := countKind(idle, RecordKindData); n != 0 {
		t.Errorf("the idle window's file holds %d Data records, want 0", n)
	}
	if n := countKind(idle, RecordKindAlive); n == 0 {
		t.Errorf("the idle window's file holds no Alive ticks, so a dead capture "+
			"and a quiet market would be identical: %v", kindsOf(idle))
	}

	// Every rotated file is bracketed: Open first, Close last.
	for _, p := range []string{firstFile, idleFile, lastFile} {
		recs := readAllRecords(t, p)
		if recs[0].Kind != RecordKindOpen {
			t.Errorf("%s begins with %s, want an Open record", p, recs[0].Kind)
		}
		if last := recs[len(recs)-1]; last.Kind != RecordKindClose {
			t.Errorf("%s ends with %s, want a Close record", p, last.Kind)
		}
	}

	// record_count describes the FILE, not the writer's lifetime. The two agree
	// for the first file, so only a rotated one can tell them apart.
	for _, p := range []string{firstFile, idleFile, lastFile} {
		recs := readAllRecords(t, p)
		var body closePayload
		if err := json.Unmarshal(recs[len(recs)-1].Payload, &body); err != nil {
			t.Fatalf("decode close payload of %s: %v", p, err)
		}
		// The Close record itself is not counted: the count is taken before it
		// is written, alongside the digest it accompanies.
		if want := uint64(len(recs) - 1); body.RecordCount != want {
			t.Errorf("%s: close record_count = %d, want %d (the file holds %d records)",
				p, body.RecordCount, want, len(recs))
		}
	}
}

// TestAliveTicksDoNotBackfill pins the decision that a stalled pump comes up
// SHORT rather than catching up.
//
// Backfilling ten ticks stamped with the current instant would produce the
// expected count and erase the evidence that the process was stalled, which is
// the one thing the Alive mechanism exists to make visible.
func TestAliveTicksDoNotBackfill(t *testing.T) {
	c := newFakeClock(startInstant(t))
	w, err := OpenWriter(testConfig(t, perpKey(), c))
	if err != nil {
		t.Fatalf("OpenWriter: %v", err)
	}

	// One tick per interval while the pump keeps up.
	for range 4 {
		c.Advance(7 * time.Minute)
		if err := w.Tick(); err != nil {
			t.Fatalf("Tick: %v", err)
		}
	}
	keptUp := w.Stats().AliveTicks
	if keptUp != 4 {
		t.Fatalf("a pump that kept up wrote %d ticks in 4 intervals, want 4", keptUp)
	}

	// Then a stall of five intervals, resolved by one call.
	c.Advance(5 * 7 * time.Minute)
	if err := w.Tick(); err != nil {
		t.Fatalf("Tick after the stall: %v", err)
	}
	if got := w.Stats().AliveTicks - keptUp; got != 1 {
		t.Errorf("a single Tick after a five-interval stall wrote %d ticks, want 1 — "+
			"backfilling would report the expected count and hide the stall", got)
	}
	_ = w.Close()
}

// TestAliveTickFiresExactlyOnTheBoundary is the collapse the checklist missed
// on its first pass and the mutation plan surfaced: every other alive fixture
// advances by 7 minutes from 14:03:17, so `now` never lands ON aliveDue, and
// `!now.Before(due)` and `now.After(due)` are the same predicate everywhere.
//
// This is the repo's `<=` versus `<` shape for the sixth time (ratelimit's freeAt
// twice, outbox, observability, exhaustiveswitch, boot), and the fix is the same
// one every time: put a fixture exactly on the value the comparison turns on.
func TestAliveTickFiresExactlyOnTheBoundary(t *testing.T) {
	const interval = 7 * time.Minute
	onBoundary := startInstant(t).Truncate(interval)
	c := newFakeClock(onBoundary)
	cfg := testConfig(t, perpKey(), c)
	cfg.AliveInterval = interval
	w, err := OpenWriter(cfg)
	if err != nil {
		t.Fatalf("OpenWriter: %v", err)
	}
	defer func() { _ = w.Close() }()

	// One nanosecond short: no tick yet. Without this half, an implementation
	// that ticked on every call would pass the half below.
	c.Advance(interval - time.Nanosecond)
	if err := w.Tick(); err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if got := w.Stats().AliveTicks; got != 0 {
		t.Fatalf("a tick fired %d times one nanosecond before the boundary, want 0", got)
	}

	// Exactly on it: the tick is due.
	c.Advance(time.Nanosecond)
	if !c.Now().Equal(onBoundary.Add(interval)) {
		t.Fatalf("the fixture is at %s, not on the boundary %s — it does not test "+
			"what it names", c.Now(), onBoundary.Add(interval))
	}
	if err := w.Tick(); err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if got := w.Stats().AliveTicks; got != 1 {
		t.Errorf("a tick exactly on the boundary fired %d times, want 1", got)
	}
}

// TestClockStepRecordedAtBothMagnitudes varies §32l's clock-step row: a step
// smaller than the record spacing and a step larger than it.
//
// A fixture with only the large step cannot tell "detected" from "detected
// because it moved the window", and a fixture with only the small one cannot
// tell "recorded" from "absorbed".
func TestClockStepRecordedAtBothMagnitudes(t *testing.T) {
	c := newFakeClock(startInstant(t))
	w, err := OpenWriter(testConfig(t, perpKey(), c))
	if err != nil {
		t.Fatalf("OpenWriter: %v", err)
	}

	const spacing = 30 * time.Second
	c.Advance(spacing)
	if err := w.Data(dataRecord(1, c, "before")); err != nil {
		t.Fatalf("Data: %v", err)
	}

	// Smaller than the spacing between records — the case that is invisible in
	// the RecvTimes alone, because the clamp keeps them ordered.
	c.Backstep(2*time.Second, spacing)
	if err := w.Tick(); err != nil {
		t.Fatalf("Tick after the small step: %v", err)
	}
	// Larger than the spacing.
	c.Backstep(5*time.Minute, spacing)
	if err := w.Tick(); err != nil {
		t.Fatalf("Tick after the large step: %v", err)
	}
	// A tick with no new step must NOT write another record; otherwise "reports
	// a step" and "reports every poll" are the same behaviour.
	if err := w.Tick(); err != nil {
		t.Fatalf("quiet Tick: %v", err)
	}
	path := w.Path()
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	recs := readAllRecords(t, path)
	steps := make([]clockStepPayload, 0, 2)
	for _, r := range recs {
		if r.Kind != RecordKindClockStep {
			continue
		}
		var p clockStepPayload
		if err := json.Unmarshal(r.Payload, &p); err != nil {
			t.Fatalf("decode clock step payload: %v", err)
		}
		steps = append(steps, p)
	}
	if len(steps) != 2 {
		t.Fatalf("archive holds %d ClockStep records, want exactly 2 (one per step, "+
			"none for the quiet poll): %v", len(steps), kindsOf(recs))
	}
	if steps[0].BackstepCount != 1 || steps[0].BackstepTotalNs != int64(2*time.Second) {
		t.Errorf("first step recorded %+v, want count 1 and 2s absorbed", steps[0])
	}
	// Cumulative, per §33d: both counters are monotonic and never reset, so a
	// caller wanting a delta subtracts two observations.
	if steps[1].BackstepCount != 2 || steps[1].BackstepTotalNs != int64(2*time.Second+5*time.Minute) {
		t.Errorf("second step recorded %+v, want the CUMULATIVE count 2 and total 5m2s", steps[1])
	}
}

// TestTornTailAtEveryOffsetClass varies §32l's truncation row past the two
// obvious cases.
//
// The boundary case is the one that collapses: a file truncated exactly at a
// record boundary is INDISTINGUISHABLE from a clean end, so a recovery that
// reported a torn tail there would write a gap record for a discontinuity that
// never happened — and §32k's verify would report the archive as gapped forever
// after.
func TestTornTailAtEveryOffsetClass(t *testing.T) {
	// The fixture returns the final frame's length, because every offset class
	// below is a position INSIDE that frame. Expressing the cut as "bytes
	// removed from the file" instead was the first draft, and it made the
	// expected torn-byte count a function of a frame length the test never
	// measured — so the assertion was against an arithmetic guess rather than
	// against the format.
	build := func(t *testing.T) (root string, path string, size int64, lastFrame int) {
		t.Helper()
		c := newFakeClock(startInstant(t))
		cfg := testConfig(t, perpKey(), c)
		w, err := OpenWriter(cfg)
		if err != nil {
			t.Fatalf("OpenWriter: %v", err)
		}
		for i := uint64(1); i <= 3; i++ {
			c.Advance(time.Second)
			// Different payload lengths, so "advance by the frame" and "advance
			// by a constant" are different walks.
			if err := w.Data(dataRecord(i, c, strings.Repeat("x", int(i)*17))); err != nil {
				t.Fatalf("Data: %v", err)
			}
		}
		if err := w.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
		st, err := os.Stat(w.Path())
		if err != nil {
			t.Fatalf("stat: %v", err)
		}
		body, err := os.ReadFile(w.Path())
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		frame, err := frameLenOfLastRecord(body)
		if err != nil {
			t.Fatalf("measure the last frame: %v", err)
		}
		return cfg.Root, w.Path(), st.Size(), frame
	}

	// `retain` is how many bytes of the FINAL frame survive the cut, so each row
	// names a position inside that frame rather than an offset from the end of
	// the file.
	//
	// The boundary collapses in TWO distinct ways and both are here, because
	// they differ in what else the recovery should say: an untouched file ends
	// with its Close record and is a clean restart, while a file whose final
	// frame was removed whole also ends on a boundary but has NO Close — a crash
	// that happened to land between records. Neither may report a torn tail, and
	// a fixture with only one of them cannot tell "no torn tail on a boundary"
	// from "no torn tail when a Close is present".
	for _, tc := range []struct {
		name        string
		retain      func(lastFrame int) int
		wantTorn    bool
		wantRestart bool
	}{
		{"untouched, ending on a Close record", func(n int) int { return n }, false, true},
		{"final frame removed whole — a boundary with no Close", func(int) int { return 0 }, false, false},
		{"inside the length field itself", func(int) int { return 2 }, true, false},
		{"mid-header", func(int) int { return recordHeaderLen - 8 }, true, false},
		{"mid-payload", func(n int) int { return n - 4 }, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, path, size, lastFrame := build(t)
			retain := tc.retain(lastFrame)
			if tc.wantTorn && (retain <= 0 || retain >= lastFrame) {
				t.Fatalf("retain %d is not strictly inside the %d-byte final frame, so "+
					"this row does not test the class it names", retain, lastFrame)
			}
			if cut := int64(lastFrame - retain); cut > 0 {
				if err := os.Truncate(path, size-cut); err != nil {
					t.Fatalf("truncate: %v", err)
				}
			}

			c := newFakeClock(startInstant(t))
			cfg := testConfig(t, perpKey(), c)
			cfg.Root = root
			w, err := OpenWriter(cfg)
			if err != nil {
				t.Fatalf("reopen: %v", err)
			}
			defer func() { _ = w.Close() }()

			recs := readAllRecords(t, w.Path())
			var causes []Cause
			for _, r := range recs {
				if r.Kind != RecordKindGap {
					continue
				}
				var g Gap
				if err := json.Unmarshal(r.Payload, &g); err != nil {
					t.Fatalf("decode gap: %v", err)
				}
				causes = append(causes, g.Cause)
				if g.Cause == CauseTornTail && g.Bytes != int64(retain) {
					t.Errorf("torn tail reports %d bytes, want the %d retained bytes of "+
						"the final frame", g.Bytes, retain)
				}
			}
			if got := slices.Contains(causes, CauseTornTail); got != tc.wantTorn {
				t.Errorf("torn_tail gap present = %v, want %v (causes %v)", got, tc.wantTorn, causes)
			}
			// The restart gap is the second axis, and asserting it separately is
			// what keeps the two clean rows from collapsing onto each other:
			// both end on a boundary, and only one ended cleanly.
			if got := slices.Contains(causes, CauseProcessRestart); got != tc.wantRestart {
				t.Errorf("process_restart gap present = %v, want %v (causes %v)",
					got, tc.wantRestart, causes)
			}
		})
	}
}

// TestTornTailIsTruncatedNotOverwritten was found by a surviving mutant, and the
// diagnosis is worth stating because the mutant looked equivalent.
//
// Removing the Truncate call left every fixture passing: the recovery seeks to
// the last good boundary and writes an Open record and a gap record there, which
// together are LONGER than the torn tails those fixtures produce, so the stale
// bytes were entirely overwritten and the file came out identical.
//
// They stop being equivalent when the torn tail is longer than what the new run
// writes before it stops — the realistic case, since a torn tail is a partially
// written record and records carry venue book updates. Then the file ends with
// several kilobytes of stale bytes past the last real record, and the NEXT open
// reads them as corruption: ErrArchiveCorrupt on a file whose recovery had
// already been recorded as clean.
//
// The fixture therefore cuts into a large DATA record rather than the small
// Close record every other torn-tail case cuts into.
func TestTornTailIsTruncatedNotOverwritten(t *testing.T) {
	c := newFakeClock(startInstant(t))
	cfg := testConfig(t, perpKey(), c)
	w, err := OpenWriter(cfg)
	if err != nil {
		t.Fatalf("OpenWriter: %v", err)
	}
	for i := uint64(1); i <= 3; i++ {
		c.Advance(time.Second)
		if err := w.Data(dataRecord(i, c, strings.Repeat("z", 4096))); err != nil {
			t.Fatalf("Data: %v", err)
		}
	}
	path := w.Path()
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	closeFrame, err := frameLenOfLastRecord(body)
	if err != nil {
		t.Fatalf("measure the close record: %v", err)
	}

	// Remove the Close record entirely plus four bytes of the 4 KiB Data record
	// before it, so the retained tail is thousands of bytes rather than tens.
	cut := int64(closeFrame + 4)
	if err := os.Truncate(path, int64(len(body))-cut); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	const wantTorn = 4096 + recordHeaderLen - 4
	if wantTorn < 1024 {
		t.Fatalf("the fixture's torn tail is only %d bytes, which the recovery's own "+
			"records would overwrite — it does not test what it names", wantTorn)
	}

	c2 := newFakeClock(startInstant(t))
	cfg2 := testConfig(t, perpKey(), c2)
	cfg2.Root = cfg.Root
	w2, err := OpenWriter(cfg2)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if err := w2.Close(); err != nil {
		t.Fatalf("Close after recovery: %v", err)
	}

	// The recovered file must decode end to end with nothing left over. A
	// recovery that seeked without truncating leaves the tail's remainder past
	// the last record it wrote.
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = f.Close() }()
	scan, err := scanArchive(f)
	if err != nil {
		t.Fatalf("the recovered file does not scan cleanly: %v", err)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if scan.tornBytes != 0 || scan.goodEnd != st.Size() {
		t.Errorf("after recovery the file is %d bytes with %d good and %d torn — the "+
			"discarded tail was overwritten rather than truncated away",
			st.Size(), scan.goodEnd, scan.tornBytes)
	}
}

// TestCorruptFileIsRefusedNotTruncated is the other half of the sentinel split
// errors.go exists for: a torn tail is truncated and recorded, and everything
// else stops the open.
//
// Truncating on a checksum failure would destroy the evidence of the corruption
// and leave an archive that verifies clean with a hole in it.
func TestCorruptFileIsRefusedNotTruncated(t *testing.T) {
	c := newFakeClock(startInstant(t))
	cfg := testConfig(t, perpKey(), c)
	w, err := OpenWriter(cfg)
	if err != nil {
		t.Fatalf("OpenWriter: %v", err)
	}
	for i := uint64(1); i <= 3; i++ {
		c.Advance(time.Second)
		if err := w.Data(dataRecord(i, c, "payload")); err != nil {
			t.Fatalf("Data: %v", err)
		}
	}
	path := w.Path()
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	// A bad byte in the MIDDLE of the file, so the frame is not the last one and
	// the checksum spans the right bytes and fails — §33c's second row, the one
	// that is diagnosed correctly.
	body[len(body)/2] ^= 0x40
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	c2 := newFakeClock(startInstant(t))
	cfg2 := testConfig(t, perpKey(), c2)
	cfg2.Root = cfg.Root
	if _, err := OpenWriter(cfg2); !errorIs(err, ErrArchiveCorrupt) {
		t.Fatalf("reopening a corrupt file = %v, want ErrArchiveCorrupt", err)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if st.Size() != int64(len(body)) {
		t.Errorf("the corrupt file is now %d bytes, was %d — a refused open must not "+
			"truncate, or the evidence is gone", st.Size(), len(body))
	}
}

// TestSecondWriterIsRefusedAndTheFirstKeepsWriting varies §32l's last row: a
// fixture where both writers are refused cannot tell "the second is refused"
// from "both are".
func TestSecondWriterIsRefusedAndTheFirstKeepsWriting(t *testing.T) {
	c := newFakeClock(startInstant(t))
	cfg := testConfig(t, perpKey(), c)
	first, err := OpenWriter(cfg)
	if err != nil {
		t.Fatalf("the first writer was refused: %v", err)
	}

	second, err := OpenWriter(cfg)
	if !errorIs(err, ErrLockHeld) {
		t.Fatalf("the second writer got %v, want ErrLockHeld", err)
	}
	if second != nil {
		t.Fatal("the refused writer returned a non-nil Writer")
	}
	// The message must name the holder, per §32f — "a second writer refuses to
	// start and says which process holds it".
	if !strings.Contains(err.Error(), "pid") {
		t.Errorf("the refusal %q does not name the holding process", err)
	}

	// The half that a both-refused fixture cannot see.
	c.Advance(time.Second)
	if err := first.Data(dataRecord(1, c, "still writing")); err != nil {
		t.Errorf("the lock holder was disturbed by the refused writer: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// And after the release, a new writer succeeds — the positive control that
	// stops a lock which refuses forever from scoring perfectly.
	third, err := OpenWriter(cfg)
	if err != nil {
		t.Fatalf("after the lock was released, a new writer was still refused: %v", err)
	}
	_ = third.Close()
}

// TestCloseWritesBothDigestsAndTheyDiffer pins the two-digest design.
//
// A test asserting only one of them would pass against an implementation that
// wrote the same digest twice, which is the mistake worth guarding: the Close
// record's digest CANNOT cover the record carrying it, so the two must differ by
// exactly that record.
func TestCloseWritesBothDigestsAndTheyDiffer(t *testing.T) {
	c := newFakeClock(startInstant(t))
	w, err := OpenWriter(testConfig(t, perpKey(), c))
	if err != nil {
		t.Fatalf("OpenWriter: %v", err)
	}
	for i := uint64(1); i <= 3; i++ {
		c.Advance(time.Second)
		if err := w.Data(dataRecord(i, c, "payload")); err != nil {
			t.Fatalf("Data: %v", err)
		}
	}
	path := w.Path()
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	recs := readAllRecords(t, path)
	var body closePayload
	if err := json.Unmarshal(recs[len(recs)-1].Payload, &body); err != nil {
		t.Fatalf("decode close payload: %v", err)
	}

	sidecar, err := os.ReadFile(path + digestExt)
	if err != nil {
		t.Fatalf("read sidecar: %v", err)
	}
	fields := strings.Fields(string(sidecar))
	if len(fields) != 2 || fields[1] != filepath.Base(path) {
		t.Fatalf("sidecar %q is not in sha256sum's format", sidecar)
	}
	if fields[0] == body.SHA256 {
		t.Errorf("both digests are %s; the Close record's digest cannot cover the "+
			"record that carries it, so they must differ", fields[0])
	}

	// Each digest is checked against what it claims to cover, which is what
	// makes "they differ" more than a coincidence.
	full, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read archive: %v", err)
	}
	closeFrameLen, err := frameLenOfLastRecord(full)
	if err != nil {
		t.Fatalf("measure the close record: %v", err)
	}
	if got := sha256Hex(full); got != fields[0] {
		t.Errorf("sidecar digest %s does not match the complete file (%s)", fields[0], got)
	}
	if got := sha256Hex(full[:len(full)-closeFrameLen]); got != body.SHA256 {
		t.Errorf("close record digest %s does not match the file before it (%s)", body.SHA256, got)
	}
}

// TestScannerReadsPastItsBufferBoundary is the collapse the checklist found: in
// every other fixture the whole file fits in one read, where "read until the
// file is exhausted" and "decode whatever one read returned" agree.
//
// They disagree on a file larger than scanBufInit, and the wrong one truncates a
// perfectly healthy archive down to its first 64 KiB while recording a torn-tail
// gap that never happened.
func TestScannerReadsPastItsBufferBoundary(t *testing.T) {
	c := newFakeClock(startInstant(t))
	cfg := testConfig(t, perpKey(), c)
	w, err := OpenWriter(cfg)
	if err != nil {
		t.Fatalf("OpenWriter: %v", err)
	}
	// Payload sizes that are not a divisor of the buffer, so records straddle
	// the read boundary rather than aligning with it.
	const payload = 997
	const records = 400 // ~400 KiB, comfortably past scanBufInit
	for i := uint64(1); i <= records; i++ {
		c.Advance(time.Millisecond)
		if err := w.Data(dataRecord(i, c, strings.Repeat("y", payload))); err != nil {
			t.Fatalf("Data(%d): %v", i, err)
		}
	}
	path := w.Path()
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if st.Size() <= scanBufInit {
		t.Fatalf("the fixture file is %d bytes, which fits in one read (%d) — it does "+
			"not exercise the boundary it exists for", st.Size(), scanBufInit)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = f.Close() }()
	scan, err := scanArchive(f)
	if err != nil {
		t.Fatalf("scanArchive: %v", err)
	}
	if scan.tornBytes != 0 {
		t.Errorf("a healthy %d-byte file reported %d torn bytes", st.Size(), scan.tornBytes)
	}
	if scan.goodEnd != st.Size() {
		t.Errorf("goodEnd = %d, want the whole file (%d)", scan.goodEnd, st.Size())
	}
	if scan.lastSeq != records {
		t.Errorf("lastSeq = %d, want %d", scan.lastSeq, records)
	}
	if !scan.hadClose {
		t.Error("the file ends with a Close record and the scan did not see it")
	}
}

// TestScannerGrowsForARecordLargerThanItsBuffer was found by a surviving mutant:
// removing the buffer's growth ceiling changed nothing, because every fixture's
// records were small enough that the scanner never had to grow at all.
//
// The two are only equivalent while no single record exceeds scanBufInit. One
// that does is not exotic — a venue book snapshot passes 64 KiB routinely, and
// MaxRecord is 8 MiB precisely because records that large are expected. Without
// growth the scanner buffers 64 KiB, cannot decode a record that needs 200, and
// reports a healthy file as a torn tail — after which the recovery TRUNCATES it,
// discarding every record after the first oversized one.
func TestScannerGrowsForARecordLargerThanItsBuffer(t *testing.T) {
	const payload = 200 << 10 // comfortably past scanBufInit, comfortably under MaxRecord
	if payload <= scanBufInit {
		t.Fatalf("the fixture payload is %d bytes, which fits in the initial buffer "+
			"(%d) — it does not exercise growth", payload, scanBufInit)
	}
	c := newFakeClock(startInstant(t))
	cfg := testConfig(t, perpKey(), c)
	w, err := OpenWriter(cfg)
	if err != nil {
		t.Fatalf("OpenWriter: %v", err)
	}
	// A small record on either side, so a scanner that mishandled growth would
	// also lose the record after the big one rather than only the big one.
	for i, size := range []int{64, payload, 64} {
		c.Advance(time.Second)
		if err := w.Data(dataRecord(uint64(i+1), c, strings.Repeat("w", size))); err != nil {
			t.Fatalf("Data(%d bytes): %v", size, err)
		}
	}
	path := w.Path()
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = f.Close() }()
	scan, err := scanArchive(f)
	if err != nil {
		t.Fatalf("scanArchive on a file with a %d-byte record: %v", payload, err)
	}
	if scan.tornBytes != 0 {
		t.Errorf("a healthy file with one oversized record reported %d torn bytes — "+
			"the recovery would truncate away everything after it", scan.tornBytes)
	}
	if scan.lastSeq != 3 {
		t.Errorf("lastSeq = %d, want 3 — the record after the oversized one was lost", scan.lastSeq)
	}
}

// TestMintedRecordsEchoTheLastDataSequence pins the decision §32d forced: writer
// -owned records carry the last Data CaptureSeq rather than one of their own.
//
// A fixture that minted before any Data arrived would see zero either way, which
// is the collapse — "echoes the last data sequence" and "always writes zero"
// give the same answer.
func TestMintedRecordsEchoTheLastDataSequence(t *testing.T) {
	c := newFakeClock(startInstant(t))
	w, err := OpenWriter(testConfig(t, perpKey(), c))
	if err != nil {
		t.Fatalf("OpenWriter: %v", err)
	}
	for i := uint64(1); i <= 3; i++ {
		c.Advance(time.Second)
		if err := w.Data(dataRecord(i, c, "d")); err != nil {
			t.Fatalf("Data: %v", err)
		}
	}
	if err := w.Gap(Gap{Cause: CauseReconnect, Detail: "socket dropped"}); err != nil {
		t.Fatalf("Gap: %v", err)
	}
	path := w.Path()
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	recs := readAllRecords(t, path)
	for _, r := range recs {
		switch r.Kind {
		case RecordKindGap, RecordKindClose:
			if r.CaptureSeq != 3 {
				t.Errorf("%s carries CaptureSeq %d, want the last data sequence 3", r.Kind, r.CaptureSeq)
			}
		case RecordKindOpen:
			if r.CaptureSeq != 0 {
				t.Errorf("the Open record carries CaptureSeq %d, want 0 — no data had "+
					"arrived when it was written", r.CaptureSeq)
			}
		case RecordKindData, RecordKindAlive, RecordKindRest, RecordKindClockStep,
			RecordKindUnspecified:
			// Not part of this claim. Named rather than left to a default, per
			// §3.6 rule 10 — a kind added later lands here as a compile failure
			// instead of silently escaping the assertion.
		}
	}
}

func TestDataRejections(t *testing.T) {
	c := newFakeClock(startInstant(t))
	w, err := OpenWriter(testConfig(t, perpKey(), c))
	if err != nil {
		t.Fatalf("OpenWriter: %v", err)
	}
	defer func() { _ = w.Close() }()

	c.Advance(time.Minute)
	// The positive control first: the well-formed record is accepted, without
	// which every refusal below would be satisfied by a Data that refused all.
	if err := w.Data(dataRecord(5, c, "ok")); err != nil {
		t.Fatalf("the well-formed record was refused: %v", err)
	}

	alive := dataRecord(6, c, "")
	alive.Kind = RecordKindAlive
	if err := w.Data(alive); !errorIs(err, ErrKindNotData) {
		t.Errorf("Data(an Alive record) = %v, want ErrKindNotData", err)
	}

	// Ascent, at both failing relations: equal and lower. A fixture with only
	// one cannot tell `<=` from `<`.
	for _, seq := range []uint64{5, 4, 0} {
		if err := w.Data(dataRecord(seq, c, "x")); !errorIs(err, ErrSeqNotAscending) {
			t.Errorf("Data(seq %d after 5) = %v, want ErrSeqNotAscending", seq, err)
		}
	}

	// A window in the past. Reachable only from a caller — Wall clamps Now so a
	// live reader's stamps never go back — which is why it is tested here rather
	// than assumed unreachable.
	old := dataRecord(6, c, "x")
	old.RecvTime = c.Now().Add(-2 * time.Hour)
	if err := w.Data(old); !errorIs(err, ErrWindowBackwards) {
		t.Errorf("Data(a record two hours old) = %v, want ErrWindowBackwards", err)
	}
}

func TestWriterConfigRefusesTheZeroValue(t *testing.T) {
	c := newFakeClock(startInstant(t))
	good := testConfig(t, perpKey(), c)
	if err := good.Validate(); err != nil {
		t.Fatalf("the populated config was refused: %v", err)
	}
	for _, tc := range []struct {
		name string
		mut  func(*WriterConfig)
		want error
	}{
		{"no root", func(c *WriterConfig) { c.Root = "" }, ErrRootUnset},
		{"no clock", func(c *WriterConfig) { c.Clock = nil }, ErrTimeSourceUnset},
		{"no rotation period", func(c *WriterConfig) { c.RotationPeriod = 0 }, ErrPeriodRange},
		{"negative rotation period", func(c *WriterConfig) { c.RotationPeriod = -time.Hour }, ErrPeriodRange},
		{"no alive interval", func(c *WriterConfig) { c.AliveInterval = 0 }, ErrIntervalRange},
		{"no sync interval", func(c *WriterConfig) { c.SyncInterval = 0 }, ErrIntervalRange},
		{"no topic", func(c *WriterConfig) { c.Key.Topic = "" }, ErrTopicUnset},
		{"unset instrument", func(c *WriterConfig) {
			c.Key.Source.Instrument = ids.VenueInstrument{}
		}, ids.ErrVenueUnset},
		{"unset stream", func(c *WriterConfig) {
			c.Key.Source.Stream = events.StreamKind("")
		}, events.ErrStreamUnset},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := good
			tc.mut(&cfg)
			if err := cfg.Validate(); !errorIs(err, tc.want) {
				t.Errorf("Validate = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestWriterRefusesAfterClose(t *testing.T) {
	c := newFakeClock(startInstant(t))
	w, err := OpenWriter(testConfig(t, perpKey(), c))
	if err != nil {
		t.Fatalf("OpenWriter: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// Idempotent, so a deferred Close after an explicit one is not an error.
	if err := w.Close(); err != nil {
		t.Errorf("the second Close = %v, want nil", err)
	}
	c.Advance(time.Second)
	if err := w.Data(dataRecord(1, c, "x")); !errorIs(err, ErrWriterClosed) {
		t.Errorf("Data after Close = %v, want ErrWriterClosed", err)
	}
	if err := w.Tick(); !errorIs(err, ErrWriterClosed) {
		t.Errorf("Tick after Close = %v, want ErrWriterClosed", err)
	}
	if err := w.Gap(Gap{Cause: CauseReconnect}); !errorIs(err, ErrWriterClosed) {
		t.Errorf("Gap after Close = %v, want ErrWriterClosed", err)
	}
}
