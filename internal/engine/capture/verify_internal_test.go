package capture

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/events"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// §32l's fixture-collapse checklist, run BEFORE the mutations (nine occurrences,
// so it is a checklist item and not a war story). What it changed here, and each
// entry is a parameter that was pinned to the value collapsing a distinction the
// code makes:
//
//   - ONE STREAM would collapse "the report's rows are per stream" onto "the
//     report has rows". Three streams, and — this is the part a count alone does
//     not fix — each with a DIFFERENT record density, because three streams fed
//     identically produce three identical rows, and a verify that attributed a
//     row to the wrong stream would pass. Stream A gets a frame every step, B
//     every second, C every third.
//   - ONE WINDOW would collapse "per window" onto "per file", and a range of one
//     window would collapse "windows come from the request" onto "windows come
//     from the files". Three verified windows, with a fourth on each side of the
//     range that exists on disk and must not appear in the report.
//   - PERIOD AND INTERVAL EQUAL, or dividing, would collapse "count the interval
//     boundaries inside the window" onto "period/interval". They are coprime (60
//     and 7), which is also what §32l's own row demands, and the healthy case
//     asserts ticks == expected EXACTLY rather than against the threshold — the
//     threshold would pass a fixture that had quietly lost a tick.
//   - THE THRESHOLD AT ANYTHING BUT 100 in the healthy case would make the
//     Alive assertion vacuous for a fully covered window.
//   - TRUNCATING MID-PAYLOAD ONLY would collapse "torn tail" onto "clean end".
//     Three truncations: mid-payload, mid-header, and EXACTLY on a record
//     boundary — the last is where the two are indistinguishable from the record
//     stream alone, and the sidecar digest is the only thing that separates them.
//   - A HOLE WITH A GAP RECORD ONLY would collapse §32a's verdict line ("a
//     recorded gap is not an error; an unexplained silence is") onto "a hole is
//     reported". Both, asserting the two different classes.
//   - A HOLE MID-FILE ONLY would collapse "hole" onto "the file starts mid-
//     sequence", which is §32l's own row. There is a hole at a file's FIRST Data
//     record, checked against the Open record's prev_last_seq.
//   - A DROP OF EXACTLY ONE would collapse "hole" onto "off-by-one". Drops of 1
//     and of 3.
//   - A PERP-ONLY SOURCE SET would leave §32c's source 4 shape (Binance SPOT
//     book) out, which is the one §35's item 2 is about. See sourceFourKey.

// sourceFourKey is §32c's source 4 — Binance spot partial depth — and it is in
// the healthy fixture for a reason that is worth stating, because §36's RESUME
// note expected it to constrain this slice.
//
// §35's item 2: source 4's payload is `{lastUpdateId, bids, asks}` — no span, no
// back-link, no event time — so it is the one source that CANNOT be gap-checked
// from its own wire bytes. That asymmetry does not reach verify, and the reason is
// §32n's fence rather than a convenience: verify reads no payload a venue wrote.
// Its continuity check is over CaptureSeq, which is OUR reader's counter (§32d)
// and is stamped identically for all seven sources — so source 4 is checked
// exactly as strictly as the other six, and a wire-level check that would have to
// treat it differently is assigned to marketdata at read time (doc.go).
//
// It is in the fixture anyway, so "verify treats every source alike" is a checked
// claim rather than an assumption that happens to hold.
func sourceFourKey() StreamKey {
	return StreamKey{
		Source: events.SourceID{
			Instrument: ids.VenueInstrument{
				Venue:   ids.Venue("binance"),
				Product: ids.ProductSpot,
				Symbol:  "BTCUSDT",
			},
			Stream: events.StreamBook,
		},
		Topic: "btcusdt@depth20@100ms",
	}
}

// The fixture's clock. A whole-minute start with whole-minute steps, so every
// Alive interval boundary coincides EXACTLY with a step.
//
// That is not tidiness. With a step phase offset from the boundaries, a tick due
// at 14:59:30 is written at 15:00:17 — after the rotation — so it lands in the
// next window's file while expectedAliveTicks counts it in the previous one, and
// the healthy fixture reports one window short and the next one over. The phase
// alignment is the fixture's own precondition, and it is asserted rather than
// assumed: the healthy case requires ticks == expected for every window.
const (
	verifyStart  = "2026-07-29T14:00:00Z"
	verifyPeriod = time.Hour
	verifyAlive  = 7 * time.Minute

	// Larger than the step, because the fake clock advances a minute per step and
	// a five-second sync interval means a REAL fsync per step per stream — which
	// is the whole cost of this fixture and buys verify nothing, since a synced
	// and an unsynced byte are the same byte to a reader.
	verifySync = 13 * time.Minute
)

// harness drives real Writers over a span of fake time, so what verify reads is
// what the writer produces.
//
// Deliberately not a hand-built archive: verify's whole value is that it is a
// SECOND implementation of the archive's rules, and a fixture built by hand
// against the same reading of those rules would test the reading rather than the
// writer. The hand-built files below exist only for states the writer refuses to
// produce, which is exactly where a second implementation has to be trusted.
type harness struct {
	t       *testing.T
	root    string
	clock   *fakeClock
	keys    []StreamKey
	writers map[string]*Writer
	seq     map[string]uint64
	step    int
}

func newHarness(t *testing.T, keys ...StreamKey) *harness {
	t.Helper()
	at, err := time.Parse(time.RFC3339, verifyStart)
	if err != nil {
		t.Fatalf("parse start: %v", err)
	}
	h := &harness{
		t: t, root: t.TempDir(), clock: newFakeClock(at), keys: keys,
		writers: make(map[string]*Writer), seq: make(map[string]uint64),
	}
	for _, k := range keys {
		h.open(k)
	}
	return h
}

func (h *harness) open(k StreamKey) *Writer {
	h.t.Helper()
	w, err := OpenWriter(WriterConfig{
		Root: h.root, Key: k, Clock: h.clock,
		Provenance:     Provenance{Tool: "capture-test", GitSHA: "deadbeef", ConfigHash: "cafe"},
		RotationPeriod: verifyPeriod, AliveInterval: verifyAlive, SyncInterval: verifySync,
	})
	if err != nil {
		h.t.Fatalf("OpenWriter(%s): %v", k, err)
	}
	h.writers[k.String()] = w

	// The reader's CaptureSeq counter is SEEDED from what the writer recovered,
	// and this line is an obligation on internal/app/capture rather than a
	// convenience here.
	//
	// OpenWriter recovers the last Data sequence from an existing hour file and
	// takes it as its own floor (writer.go: "it also bounds what this run may
	// write"), while Writer.Data refuses a sequence that does not ascend. So a
	// process restarting INTO an hour it had already been writing, with a reader
	// counting from one, has every frame refused for the remainder of that hour —
	// and the archive fills with Alive ticks while the process reports errors it
	// has nowhere to put. Seeding from Stats().LastSeq is the whole fix and needs
	// no new API; the composition root must do it at the same point.
	h.seq[k.String()] = w.Stats().LastSeq
	return w
}

// advance runs one minute: a Data record for the streams due one, then a Tick for
// every open writer.
//
// Data before Tick because that is the live order — a frame arrives and the pump
// then services timers — and because it is the order that puts the boundary tick
// in the new file rather than the old one (Writer.Tick's comment).
func (h *harness) advance() {
	h.t.Helper()
	h.step++
	h.clock.Advance(time.Minute)
	h.writeData()
	h.writeTicks()
}

func (h *harness) writeData() {
	h.t.Helper()
	for i, k := range h.keys {
		w, open := h.writers[k.String()]
		if !open {
			continue
		}
		// Every stream, every second step, every third step: three densities, so
		// a row attributed to the wrong stream cannot pass.
		if h.step%(i+1) != 0 {
			continue
		}
		h.seq[k.String()]++
		if err := w.Data(dataRecord(h.seq[k.String()], h.clock, fmt.Sprintf("frame %d", h.step))); err != nil {
			h.t.Fatalf("%s Data(%d): %v", k, h.seq[k.String()], err)
		}
	}
}

func (h *harness) writeTicks() {
	h.t.Helper()
	for _, k := range h.keys {
		if w, open := h.writers[k.String()]; open {
			if err := w.Tick(); err != nil {
				h.t.Fatalf("%s Tick: %v", k, err)
			}
		}
	}
}

// run advances n minutes with both a frame and a tick per minute.
func (h *harness) run(n int) { h.t.Helper(); h.runWith(n, true, true) }

// runIdle advances n minutes writing Alive ticks and NO venue frames — a live
// process on a dead feed, which is §32k's Binance trap.
func (h *harness) runIdle(n int) { h.t.Helper(); h.runWith(n, false, true) }

// runUnticked advances n minutes writing frames and NO ticks — a pump that
// stalled its own timekeeping, which Writer.Tick deliberately does not backfill.
func (h *harness) runUnticked(n int) { h.t.Helper(); h.runWith(n, true, false) }

func (h *harness) runWith(n int, data, tick bool) {
	h.t.Helper()
	for range n {
		h.step++
		h.clock.Advance(time.Minute)
		if data {
			h.writeData()
		}
		if tick {
			h.writeTicks()
		}
	}
}

// stop closes one stream's writer, which is §32k's "kill a stream mid-capture"
// in its clean form: the file it was writing gets a Close record and a sidecar,
// and every later window for that stream is simply absent.
func (h *harness) stop(k StreamKey) {
	h.t.Helper()
	w, open := h.writers[k.String()]
	if !open {
		return
	}
	if err := w.Close(); err != nil {
		h.t.Fatalf("%s Close: %v", k, err)
	}
	delete(h.writers, k.String())
}

// abandon drops a writer WITHOUT closing it — a process killed mid-hour. The
// file keeps its lock file and gets no Close record and no sidecar.
func (h *harness) abandon(k StreamKey) {
	h.t.Helper()
	delete(h.writers, k.String())
}

func (h *harness) stopAll() {
	h.t.Helper()
	for _, k := range h.keys {
		h.stop(k)
	}
}

// cfg is a verify request over the middle three windows.
//
// The run spans five windows and the request names three, so the archive holds a
// file on each side of the range that must not appear in the report — which is
// what makes "the report covers the requested range" a claim and not a tautology.
func (h *harness) cfg() VerifyConfig {
	h.t.Helper()
	start, err := time.Parse(time.RFC3339, verifyStart)
	if err != nil {
		h.t.Fatalf("parse start: %v", err)
	}
	return VerifyConfig{
		Root: h.root,
		From: start.Add(verifyPeriod), To: start.Add(4 * verifyPeriod),
		RotationPeriod: verifyPeriod, AliveInterval: verifyAlive,
		MinAlivePercent: 100,
	}
}

// cfgWindow is a verify request over exactly window n, 1-based from the
// harness's start instant. The focused tests use it so a single covered window
// costs sixty steps rather than five windows' worth.
func (h *harness) cfgWindow(n int) VerifyConfig {
	h.t.Helper()
	cfg := h.cfg()
	start, err := time.Parse(time.RFC3339, verifyStart)
	if err != nil {
		h.t.Fatalf("parse start: %v", err)
	}
	cfg.From = start.Add(time.Duration(n-1) * verifyPeriod)
	cfg.To = cfg.From.Add(verifyPeriod)
	return cfg
}

// healthyArchive is the fixture every fault test starts from: three streams of
// three densities across five windows, cleanly closed.
func healthyArchive(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t, perpKey(), spotKey(), sourceFourKey())
	// 4 hours and 5 minutes from a whole hour, so windows 2, 3 and 4 — the three
	// the request names — are each covered from their first minute to their last.
	h.run(4*60 + 5)
	h.stopAll()
	return h
}

// verifyOK runs Verify and fails on an error, which is never an archive fault.
func verifyOK(t *testing.T, cfg VerifyConfig) *Report {
	t.Helper()
	rep, err := Verify(cfg)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	return rep
}

// findingKinds reduces a report to the set of kinds it holds, sorted and
// deduplicated, which is what the fault assertions are about.
func findingKinds(rep *Report) []FindingKind {
	var out []FindingKind
	for _, f := range rep.Findings {
		if !slices.Contains(out, f.Kind) {
			out = append(out, f.Kind)
		}
	}
	slices.Sort(out)
	return out
}

// findingsFor lists the findings of one kind, for an assertion that needs the
// detail rather than the presence.
func findingsFor(rep *Report, kind FindingKind) []Finding {
	var out []Finding
	for _, f := range rep.Findings {
		if f.Kind == kind {
			out = append(out, f)
		}
	}
	return out
}

// dumpFindings renders a report's findings for a failure message. A test that
// says "wanted 1 finding, got 4" and does not say which four costs a debugging
// session per failure.
func dumpFindings(rep *Report) string {
	if len(rep.Findings) == 0 {
		return "(none)"
	}
	var b strings.Builder
	for _, f := range rep.Findings {
		b.WriteString("\n  " + f.String())
	}
	return b.String()
}

// TestVerifyReportsAHealthyArchiveClean is §32k's positive control, and it comes
// FIRST for the reason §31g rule 4 gives in its own sentence: a monitor that has
// never reported a problem has not been shown able to, and its converse is worse
// — a checker that objects to everything scores perfectly against every fault
// test below while being useless.
func TestVerifyReportsAHealthyArchiveClean(t *testing.T) {
	h := healthyArchive(t)
	rep := verifyOK(t, h.cfg())

	if len(rep.Findings) != 0 {
		t.Fatalf("healthy archive produced %d findings: %s", len(rep.Findings), dumpFindings(rep))
	}
	if got := rep.ExitClass(); got != ClassUnspecified {
		t.Errorf("ExitClass = %q, want the clean class", got)
	}
	if want := 3 * 3; len(rep.Windows) != want {
		t.Fatalf("%d window rows, want %d (three streams x three windows)", len(rep.Windows), want)
	}

	perStream := make(map[string]uint64)
	for _, w := range rep.Windows {
		if w.Verdict != VerdictReplayable {
			t.Errorf("%s %s: verdict %q, want %q",
				w.Stream, w.Window.Format(time.RFC3339), w.Verdict, VerdictReplayable)
		}
		// Exact rather than "at or above the threshold". The threshold is 100%
		// here, so the two agree — and asserting equality is what makes a
		// fixture that quietly lost a tick fail instead of scraping through.
		if w.AliveTicks != w.AliveExpected {
			t.Errorf("%s %s: %d Alive ticks, expected %d",
				w.Stream, w.Window.Format(time.RFC3339), w.AliveTicks, w.AliveExpected)
		}
		if w.AliveExpected < 2 {
			t.Errorf("%s %s: expected tick count is %d, so the coverage check cannot discriminate",
				w.Stream, w.Window.Format(time.RFC3339), w.AliveExpected)
		}
		if w.DataRecords == 0 {
			t.Errorf("%s %s: no Data records", w.Stream, w.Window.Format(time.RFC3339))
		}
		if len(w.Gaps) != 0 {
			t.Errorf("%s %s: gaps %v, want none", w.Stream, w.Window.Format(time.RFC3339), w.Gaps)
		}
		perStream[w.Stream.String()] += w.DataRecords
	}

	// The three densities have to arrive as three different totals, or the
	// fixture cannot tell a row attributed to the wrong stream from a right one.
	totals := make([]uint64, 0, len(perStream))
	for _, n := range perStream {
		totals = append(totals, n)
	}
	slices.Sort(totals)
	for i := 1; i < len(totals); i++ {
		if totals[i] == totals[i-1] {
			t.Fatalf("stream Data totals %v collapse: a mis-attributed row would pass", totals)
		}
	}
}

// TestVerifyNamesTheFourFaults is §32k's own list: kill a stream mid-capture,
// corrupt one byte, truncate a file mid-record, delete an hour. Each subtest
// starts from the healthy archive above and breaks exactly one thing.
func TestVerifyNamesTheFourFaults(t *testing.T) {
	t.Run("a stream that stops mid-capture leaves absent windows", func(t *testing.T) {
		h := newHarness(t, perpKey(), spotKey(), sourceFourKey())
		h.run(2*60 + 30) // into the third window
		h.stop(spotKey())
		h.run(60 + 95)
		h.stopAll()

		rep := verifyOK(t, h.cfg())
		var absent []time.Time
		for _, w := range rep.Windows {
			if w.Stream.String() == spotKey().String() && w.Verdict == VerdictAbsent {
				absent = append(absent, w.Window)
			}
		}
		if len(absent) != 1 {
			t.Fatalf("%d absent windows for the stopped stream, want 1: %s", len(absent), dumpFindings(rep))
		}
		if got := findingsFor(rep, FindingWindowAbsent); len(got) != 1 {
			t.Fatalf("%d window-absent findings, want 1: %s", len(got), dumpFindings(rep))
		}
		// A stopped stream is thin, not lying: every finding must be coverage.
		if got := rep.ExitClass(); got != ClassCoverage {
			t.Errorf("ExitClass = %q, want %q: %s", got, ClassCoverage, dumpFindings(rep))
		}
		// And the two streams that kept running must still be replayable, or
		// the report is failing the archive for its neighbour's fault.
		for _, w := range rep.Windows {
			if w.Stream.String() != spotKey().String() && w.Verdict != VerdictReplayable {
				t.Errorf("%s %s: verdict %q, want the untouched streams unaffected",
					w.Stream, w.Window.Format(time.RFC3339), w.Verdict)
			}
		}
	})

	t.Run("one corrupt byte is a checksum failure, not a torn tail", func(t *testing.T) {
		h := healthyArchive(t)
		path := archiveFileFor(t, h, perpKey(), 2)
		corruptPayloadByte(t, path)

		rep := verifyOK(t, h.cfg())
		if got := findingsFor(rep, FindingChecksum); len(got) != 1 {
			t.Fatalf("%d checksum findings, want 1: %s", len(got), dumpFindings(rep))
		}
		// The mis-diagnosis errors.go spends its header note on. A corrupt byte
		// reported as a torn tail would be truncated by the writer's recovery at
		// the next open, destroying the evidence.
		if got := findingsFor(rep, FindingTornTail); len(got) != 0 {
			t.Errorf("a corrupt byte was also reported as a torn tail: %s", dumpFindings(rep))
		}
		if got := rep.ExitClass(); got != ClassIntegrity {
			t.Errorf("ExitClass = %q, want %q: %s", got, ClassIntegrity, dumpFindings(rep))
		}
		// The sidecar covers the complete file, so it must disagree too — and
		// its disagreement is what makes the corruption a fact about the bytes
		// on disk rather than about one decoder.
		if got := findingsFor(rep, FindingDigestMismatch); len(got) != 1 {
			t.Errorf("%d sidecar mismatches, want 1: %s", len(got), dumpFindings(rep))
		}
	})

	t.Run("truncation mid-record, mid-header and on a boundary", func(t *testing.T) {
		// §32l's row: truncating mid-payload only collapses "torn tail" onto
		// "clean end". The third case is where they are indistinguishable from
		// the record stream, and the sidecar is what separates them.
		cases := []struct {
			name string
			cut  func(t *testing.T, body []byte) int64
			want []FindingKind
		}{{
			name: "mid-payload",
			cut:  func(t *testing.T, body []byte) int64 { return int64(len(body)) - 2 },
			want: []FindingKind{FindingTornTail, FindingNoClose, FindingDigestMismatch},
		}, {
			name: "mid-header",
			cut: func(t *testing.T, body []byte) int64 {
				n, err := frameLenOfLastRecord(body)
				if err != nil {
					t.Fatalf("last frame length: %v", err)
				}
				// Six bytes into the final record: past its four-byte length
				// field and two bytes into the header, where nothing but the
				// length is readable. n is the WHOLE frame including that field
				// (frameLenOfLastRecord returns DecodeRecord's consumed count,
				// not the header's frameLen) — subtracting an extra 4 here cuts
				// into the record BEFORE the last one, which reports a torn tail
				// for the wrong reason and passes.
				return int64(len(body) - n + 6)
			},
			want: []FindingKind{FindingTornTail, FindingNoClose, FindingDigestMismatch},
		}, {
			name: "exactly on a record boundary",
			cut: func(t *testing.T, body []byte) int64 {
				n, err := frameLenOfLastRecord(body)
				if err != nil {
					t.Fatalf("last frame length: %v", err)
				}
				return int64(len(body) - n)
			},
			// No torn tail at all: the record stream ends cleanly. Only the
			// missing Close record and the sidecar say anything happened, and
			// the sidecar is the one that says the file was DAMAGED rather than
			// crashed.
			want: []FindingKind{FindingNoClose, FindingDigestMismatch},
		}}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				h := healthyArchive(t)
				path := archiveFileFor(t, h, perpKey(), 3)
				body, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("read %s: %v", path, err)
				}
				if err := os.Truncate(path, tc.cut(t, body)); err != nil {
					t.Fatalf("truncate %s: %v", path, err)
				}

				rep := verifyOK(t, h.cfg())
				want := slices.Clone(tc.want)
				slices.Sort(want)
				if got := findingKinds(rep); !slices.Equal(got, want) {
					t.Fatalf("findings %v, want %v: %s", got, want, dumpFindings(rep))
				}
			})
		}
	})

	t.Run("a crashed file's torn tail is coverage, not integrity", func(t *testing.T) {
		// The other side of the truncation pair, and the one that makes the
		// classification discriminate: a file with no Close record AND no
		// sidecar was never closed, so the tail is a crash rather than damage.
		//
		// It has to be the file the writer still HAD OPEN, which is why the run
		// stops mid-window 4 rather than after window 5: rotation closes a file
		// cleanly and writes its sidecar, so every earlier window's file is a
		// closed one and truncating it is the damage case, not the crash case.
		h := newHarness(t, perpKey())
		h.run(3*60 + 30)
		h.abandon(perpKey())

		path := archiveFileFor(t, h, perpKey(), 4)
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if err := os.Truncate(path, int64(len(body))-2); err != nil {
			t.Fatalf("truncate: %v", err)
		}

		rep := verifyOK(t, h.cfg())
		if got := rep.ExitClass(); got != ClassCoverage {
			t.Fatalf("ExitClass = %q, want %q — a crash is honest and thin: %s",
				got, ClassCoverage, dumpFindings(rep))
		}
		if got := findingsFor(rep, FindingTornTail); len(got) != 1 {
			t.Errorf("%d torn tails, want 1: %s", len(got), dumpFindings(rep))
		}
		if got := findingsFor(rep, FindingDigestMissing); len(got) != 0 {
			t.Errorf("a crashed file was faulted for having no sidecar: %s", dumpFindings(rep))
		}
	})

	t.Run("a deleted hour is absent and breaks the chain", func(t *testing.T) {
		h := healthyArchive(t)
		path := archiveFileFor(t, h, perpKey(), 2)
		for _, p := range []string{path, path + digestExt} {
			if err := os.Remove(p); err != nil {
				t.Fatalf("remove %s: %v", p, err)
			}
		}

		rep := verifyOK(t, h.cfg())
		if got := findingsFor(rep, FindingWindowAbsent); len(got) != 1 {
			t.Fatalf("%d window-absent findings, want 1: %s", len(got), dumpFindings(rep))
		}
		// The surviving files now claim a continuity that is not there, which is
		// the half of this fault a coverage count cannot express: the next
		// hour's Open record names a predecessor sequence the archive no longer
		// reaches.
		breaks := findingsFor(rep, FindingCrossFileBreak)
		if len(breaks) != 1 {
			t.Fatalf("%d cross-file breaks, want 1: %s", len(breaks), dumpFindings(rep))
		}
		if got := rep.ExitClass(); got != ClassIntegrity {
			t.Errorf("ExitClass = %q, want %q: %s", got, ClassIntegrity, dumpFindings(rep))
		}
		var absent int
		for _, w := range rep.Windows {
			if w.Verdict == VerdictAbsent {
				absent++
			}
		}
		if absent != 1 {
			t.Errorf("%d absent verdicts, want 1", absent)
		}
	})

	t.Run("a missing sidecar is reported for a closed file", func(t *testing.T) {
		h := healthyArchive(t)
		path := archiveFileFor(t, h, spotKey(), 3) + digestExt
		if err := os.Remove(path); err != nil {
			t.Fatalf("remove %s: %v", path, err)
		}
		rep := verifyOK(t, h.cfg())
		if got := findingKinds(rep); !slices.Equal(got, []FindingKind{FindingDigestMissing}) {
			t.Fatalf("findings %v, want only a missing digest: %s", got, dumpFindings(rep))
		}
		if got := rep.ExitClass(); got != ClassIntegrity {
			t.Errorf("ExitClass = %q, want %q — an unverifiable copy is not merely thin: %s",
				got, ClassIntegrity, dumpFindings(rep))
		}
	})

	t.Run("a stray file in the tree is reported", func(t *testing.T) {
		h := healthyArchive(t)
		dir := filepath.Dir(archiveFileFor(t, h, perpKey(), 2))
		if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("hi"), 0o644); err != nil {
			t.Fatalf("write stray: %v", err)
		}
		rep := verifyOK(t, h.cfg())
		if got := findingKinds(rep); !slices.Equal(got, []FindingKind{FindingStrayFile}) {
			t.Fatalf("findings %v, want only a stray file: %s", got, dumpFindings(rep))
		}
	})

	t.Run("a file moved into another day is reported", func(t *testing.T) {
		// path.go's date-directory check, reached through verify: a sync script
		// that moved a file is otherwise reported under the day its name claims,
		// silently double-counting one day and emptying another.
		h := healthyArchive(t)
		path := archiveFileFor(t, h, perpKey(), 2)
		moved := filepath.Join(filepath.Dir(filepath.Dir(path)), "31", filepath.Base(path))
		if err := os.MkdirAll(filepath.Dir(moved), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.Rename(path, moved); err != nil {
			t.Fatalf("rename: %v", err)
		}
		if err := os.Remove(path + digestExt); err != nil {
			t.Fatalf("remove sidecar: %v", err)
		}

		rep := verifyOK(t, h.cfg())
		if got := findingsFor(rep, FindingStrayFile); len(got) != 1 {
			t.Fatalf("%d stray findings, want 1: %s", len(got), dumpFindings(rep))
		}
		// And the window it used to cover is now absent, which is the fact an
		// operator acts on.
		if got := findingsFor(rep, FindingWindowAbsent); len(got) != 1 {
			t.Errorf("%d window-absent findings, want 1: %s", len(got), dumpFindings(rep))
		}
	})
}

// archiveFileFor returns the archive path for a stream and a 1-based window index
// counted from the harness's start instant.
func archiveFileFor(t *testing.T, h *harness, key StreamKey, window int) string {
	t.Helper()
	start, err := time.Parse(time.RFC3339, verifyStart)
	if err != nil {
		t.Fatalf("parse start: %v", err)
	}
	p := key.FilePath(h.root, start.Add(time.Duration(window-1)*verifyPeriod), verifyPeriod)
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("window %d of %s: %v", window, key, err)
	}
	return p
}

// corruptPayloadByte flips one byte inside the first Data record's payload.
//
// Inside a PAYLOAD specifically, and located by walking rather than by picking an
// offset: a byte flipped in a header field can land on the frame length, which
// the codec refuses as ErrFrameLen before it ever computes a checksum — so a
// fixture that corrupted "some byte in the middle" would assert a checksum
// failure and pass or fail depending on where the middle happened to be.
func corruptPayloadByte(t *testing.T, path string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	off, err := ParseFileHeader(body)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	for off < len(body) {
		rec, used, err := DecodeRecord(body[off:])
		if err != nil {
			t.Fatalf("%s at %d: %v", path, off, err)
		}
		if rec.Kind == RecordKindData && len(rec.Payload) > 0 {
			body[off+recordHeaderLen] ^= 0xff
			if err := os.WriteFile(path, body, 0o644); err != nil {
				t.Fatalf("write %s: %v", path, err)
			}
			return
		}
		off += used
	}
	t.Fatalf("%s holds no Data record with a payload to corrupt", path)
}
