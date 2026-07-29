package capture

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// The CaptureSeq and continuity half of §32k's verify, plus the states the writer
// REFUSES to produce.
//
// Those states are why a builder exists here at all. Writer.Data refuses a
// sequence that does not ascend (ErrSeqNotAscending), so "evidence of two
// writers" — §32k's own phrase — cannot be reached through the writer, and a
// verify check for it would be untestable against the only tool that writes
// archives. The builder is a second, deliberately dumb encoder: it writes what it
// is told, including things no writer would write, which is exactly what a
// checker has to be shown to catch.

// archiveBuilder assembles an archive file byte by byte.
type archiveBuilder struct {
	t      *testing.T
	root   string
	key    StreamKey
	window time.Time

	buf     []byte
	at      time.Time
	records uint64
	lastSeq uint64
}

func newArchive(t *testing.T, root string, key StreamKey, window time.Time) *archiveBuilder {
	t.Helper()
	return &archiveBuilder{
		t: t, root: root, key: key, window: window,
		buf: AppendFileHeader(nil),
		at:  window.Add(time.Minute),
	}
}

// emit appends one record, advancing the fixture's clock a minute so the
// timestamps ascend the way a real file's do.
func (b *archiveBuilder) emit(kind RecordKind, seq uint64, payload []byte) {
	b.t.Helper()
	buf, err := AppendRecord(b.buf, Record{
		Kind: kind, CaptureSeq: seq, RecvTime: b.at, Mono: time.Duration(b.records) * time.Minute,
		Payload: payload,
	})
	if err != nil {
		b.t.Fatalf("append %s: %v", kind, err)
	}
	b.buf = buf
	b.records++
	b.at = b.at.Add(time.Minute)
}

func (b *archiveBuilder) marshal(v any) []byte {
	b.t.Helper()
	body, err := json.Marshal(v)
	if err != nil {
		b.t.Fatalf("marshal payload: %v", err)
	}
	return body
}

// open writes an Open record that identifies this file correctly.
func (b *archiveBuilder) open(prevLast uint64) { b.t.Helper(); b.openAs(b.key, b.window, prevLast) }

// openAs writes an Open record identifying some OTHER stream or window, for the
// moved-file check.
func (b *archiveBuilder) openAs(key StreamKey, window time.Time, prevLast uint64) {
	b.t.Helper()
	vi := key.Source.Instrument
	b.emit(RecordKindOpen, b.lastSeq, b.marshal(openPayload{
		Venue: string(vi.Venue), Product: string(vi.Product), Contract: string(vi.Contract),
		Symbol: vi.Symbol, Stream: string(key.Source.Stream), Topic: key.Topic,
		Format: fmt.Sprintf("v%d.%d", FormatMajor, FormatMinor),
		Window: window.Format(time.RFC3339), Host: "fixture", PID: 1,
		PrevLastSeq: prevLast,
		Provenance:  Provenance{Tool: "capture-test", GitSHA: "deadbeef", ConfigHash: "cafe"},
	}))
}

func (b *archiveBuilder) data(seq uint64) {
	b.t.Helper()
	b.lastSeq = seq
	b.emit(RecordKindData, seq, []byte(fmt.Sprintf(`{"seq":%d}`, seq)))
}

func (b *archiveBuilder) alive() { b.t.Helper(); b.emit(RecordKindAlive, b.lastSeq, nil) }

func (b *archiveBuilder) gap(g Gap) {
	b.t.Helper()
	payload, err := g.MarshalPayload()
	if err != nil {
		b.t.Fatalf("marshal gap: %v", err)
	}
	b.emit(RecordKindGap, b.lastSeq, payload)
}

// fillAlive appends exactly the number of ticks a fully covered window should
// hold, so a hand-built fixture does not trip the coverage threshold and bury the
// finding under test.
func (b *archiveBuilder) fillAlive(cfg VerifyConfig) {
	b.t.Helper()
	for range expectedAliveTicks(b.window, cfg.RotationPeriod, cfg.AliveInterval) {
		b.alive()
	}
}

// closeWith writes a Close record carrying whatever the caller says, so a test
// can lie about the digest, the count or the last sequence.
func (b *archiveBuilder) closeWith(lastSeq, count uint64, sha string) {
	b.t.Helper()
	b.emit(RecordKindClose, b.lastSeq, b.marshal(closePayload{
		LastSeq: lastSeq, RecordCount: count, SHA256: sha,
	}))
}

// seal writes a truthful Close record: the digest of everything before it and the
// count of the records before it, which is what writer.closeFile does.
func (b *archiveBuilder) seal() {
	b.t.Helper()
	sum := sha256.Sum256(b.buf)
	b.closeWith(b.lastSeq, b.records, hex.EncodeToString(sum[:]))
}

// write flushes the file and its sidecar, and returns the path.
func (b *archiveBuilder) write() string {
	b.t.Helper()
	path := b.key.FilePath(b.root, b.window, verifyPeriod)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		b.t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, b.buf, 0o644); err != nil {
		b.t.Fatalf("write %s: %v", path, err)
	}
	sum := sha256.Sum256(b.buf)
	body := fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), filepath.Base(path))
	if err := os.WriteFile(path+digestExt, []byte(body), 0o644); err != nil {
		b.t.Fatalf("write sidecar: %v", err)
	}
	return path
}

// builtCfg is a verify request over one window of a hand-built root.
func builtCfg(root string, window time.Time) VerifyConfig {
	return VerifyConfig{
		Root: root, From: window, To: window.Add(verifyPeriod),
		RotationPeriod: verifyPeriod, AliveInterval: verifyAlive, MinAlivePercent: 100,
	}
}

func fixtureWindow(t *testing.T, n int) time.Time {
	t.Helper()
	start, err := time.Parse(time.RFC3339, verifyStart)
	if err != nil {
		t.Fatalf("parse start: %v", err)
	}
	return start.Add(time.Duration(n-1) * verifyPeriod)
}

// TestVerifySeparatesExplainedFromUnexplainedHoles is §32a's verdict line as a
// test: a recorded gap is not an error; an unexplained silence is.
//
// The table varies the drop SIZE as well as whether a gap record covers it,
// because §32l's row names a drop of exactly one as the collapse where "hole" and
// "off-by-one" are the same reading.
func TestVerifySeparatesExplainedFromUnexplainedHoles(t *testing.T) {
	cases := []struct {
		name      string
		seqs      []uint64
		gap       *Gap
		want      FindingKind
		wantRange string
	}{{
		name: "a drop of one with no gap record",
		seqs: []uint64{1, 2, 4, 5},
		want: FindingSeqHoleUnexplained, wantRange: "3..3",
	}, {
		name: "a drop of three with no gap record",
		seqs: []uint64{1, 2, 6, 7},
		want: FindingSeqHoleUnexplained, wantRange: "3..5",
	}, {
		name: "a drop of one the queue recorded",
		seqs: []uint64{1, 2, 4, 5},
		gap:  &Gap{Cause: CauseOverflow, From: 3, To: 3, Count: 1},
		want: FindingSeqHole, wantRange: "3..3",
	}, {
		name: "a drop of three the queue recorded",
		seqs: []uint64{1, 2, 6, 7},
		gap:  &Gap{Cause: CauseOverflow, From: 3, To: 5, Count: 3},
		want: FindingSeqHole, wantRange: "3..5",
	}, {
		// A gap that does not COVER the hole must not excuse it. Partial cover
		// is a gap record describing a different event, and accepting it would
		// let one recorded overflow launder every later hole in the file.
		name: "a gap record that covers only part of the hole",
		seqs: []uint64{1, 2, 6, 7},
		gap:  &Gap{Cause: CauseOverflow, From: 3, To: 4, Count: 2},
		want: FindingSeqHoleUnexplained, wantRange: "3..5",
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, window := t.TempDir(), fixtureWindow(t, 1)
			cfg := builtCfg(root, window)
			b := newArchive(t, root, perpKey(), window)
			b.open(0)
			for i, seq := range tc.seqs {
				if i == 2 && tc.gap != nil {
					b.gap(*tc.gap)
				}
				b.data(seq)
			}
			b.fillAlive(cfg)
			b.seal()
			b.write()

			rep := verifyOK(t, cfg)
			holes := append(findingsFor(rep, FindingSeqHole), findingsFor(rep, FindingSeqHoleUnexplained)...)
			if len(holes) != 1 {
				t.Fatalf("%d hole findings, want 1: %s", len(holes), dumpFindings(rep))
			}
			if holes[0].Kind != tc.want {
				t.Errorf("hole reported as %q, want %q: %s", holes[0].Kind, tc.want, holes[0].Detail)
			}
			if !strings.Contains(holes[0].Detail, tc.wantRange) {
				t.Errorf("detail %q does not name the range %s", holes[0].Detail, tc.wantRange)
			}
			// The class is the thing that reaches an operator, through the exit
			// code, and it is what the two kinds exist to separate.
			wantClass := ClassCoverage
			if tc.want == FindingSeqHoleUnexplained {
				wantClass = ClassIntegrity
			}
			if got := rep.ExitClass(); got != wantClass {
				t.Errorf("ExitClass = %q, want %q: %s", got, wantClass, dumpFindings(rep))
			}
		})
	}
}

// TestVerifyFindsAHoleAtAFilesFirstRecord is §32l's row: a drop at the FIRST
// record of a file, where "hole" and "the file starts mid-sequence" are the same
// reading.
//
// It is the case a per-file check cannot see at all, because within the file
// nothing is missing — the evidence is entirely in the Open record's
// prev_last_seq, which is why §32e put it there.
func TestVerifyFindsAHoleAtAFilesFirstRecord(t *testing.T) {
	root := t.TempDir()
	first, second := fixtureWindow(t, 1), fixtureWindow(t, 2)
	cfg := builtCfg(root, second)

	a := newArchive(t, root, perpKey(), first)
	a.open(0)
	for seq := uint64(1); seq <= 4; seq++ {
		a.data(seq)
	}
	a.fillAlive(cfg)
	a.seal()
	a.write()

	b := newArchive(t, root, perpKey(), second)
	b.open(4) // continues the predecessor…
	for seq := uint64(8); seq <= 10; seq++ {
		b.data(seq) // …but the first frame is 8, so 5..7 never arrived
	}
	b.fillAlive(cfg)
	b.seal()
	b.write()

	rep := verifyOK(t, cfg)
	holes := findingsFor(rep, FindingSeqHoleUnexplained)
	if len(holes) != 1 {
		t.Fatalf("%d unexplained holes, want 1: %s", len(holes), dumpFindings(rep))
	}
	if !strings.Contains(holes[0].Detail, "5..7") {
		t.Errorf("detail %q does not name the missing range 5..7", holes[0].Detail)
	}
	if holes[0].Window.Equal(first) {
		t.Errorf("the hole is attributed to the predecessor window, want %s", second.Format(time.RFC3339))
	}
}

// TestVerifyReportsNonAscendingSequencesAsEvidenceOfTwoWriters covers the state
// Writer.Data refuses, which is the reason this fixture is hand-built.
//
// Two capture processes over one root interleave their sequences into something
// that looks like a healthy dense log from every angle except this one, and §32f's
// flock is what prevents it — so this is §31g rule 4's out-of-process check that
// the prevention worked, rather than a check on the writer.
func TestVerifyReportsNonAscendingSequencesAsEvidenceOfTwoWriters(t *testing.T) {
	cases := []struct {
		name string
		seqs []uint64
	}{
		{name: "a repeated sequence", seqs: []uint64{1, 2, 2, 3}},
		{name: "a reversed sequence", seqs: []uint64{1, 2, 3, 2}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, window := t.TempDir(), fixtureWindow(t, 1)
			cfg := builtCfg(root, window)
			b := newArchive(t, root, perpKey(), window)
			b.open(0)
			for _, seq := range tc.seqs {
				b.data(seq)
			}
			b.fillAlive(cfg)
			// last_seq is the final record's sequence, whatever that turned out
			// to be, so the Close record does not add a second finding.
			b.closeWith(tc.seqs[len(tc.seqs)-1], b.records, closeDigest(b))
			b.write()

			rep := verifyOK(t, cfg)
			if got := findingKinds(rep); !slices.Equal(got, []FindingKind{FindingSeqNotAscending}) {
				t.Fatalf("findings %v, want only a non-ascending sequence: %s", got, dumpFindings(rep))
			}
			if got := rep.ExitClass(); got != ClassIntegrity {
				t.Errorf("ExitClass = %q, want %q", got, ClassIntegrity)
			}
		})
	}
}

// closeDigest is the digest of everything a builder has appended so far, which is
// what a truthful Close record carries.
func closeDigest(b *archiveBuilder) string {
	sum := sha256.Sum256(b.buf)
	return hex.EncodeToString(sum[:])
}

// TestVerifyChecksTheCloseRecordsOwnClaims covers the three claims a Close record
// makes about the file it ends. Each is redundant with a walk of the file, and
// CHECKING the redundancy is what turns it from an opportunity to disagree into
// evidence.
func TestVerifyChecksTheCloseRecordsOwnClaims(t *testing.T) {
	cases := []struct {
		name string
		lie  func(b *archiveBuilder, lastSeq, count uint64, sha string)
		want FindingKind
	}{{
		name: "a digest that does not cover the bytes",
		lie: func(b *archiveBuilder, lastSeq, count uint64, sha string) {
			b.closeWith(lastSeq, count, "00000000000000000000000000000000"+
				"00000000000000000000000000000000")
		},
		want: FindingDigestMismatch,
	}, {
		name: "a record count that is not the file's",
		lie: func(b *archiveBuilder, lastSeq, count uint64, sha string) {
			b.closeWith(lastSeq, count+7, sha)
		},
		want: FindingRecordCount,
	}, {
		name: "a last sequence the records do not reach",
		lie: func(b *archiveBuilder, lastSeq, count uint64, sha string) {
			b.closeWith(lastSeq+5, count, sha)
		},
		want: FindingCrossFileBreak,
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, window := t.TempDir(), fixtureWindow(t, 1)
			cfg := builtCfg(root, window)
			b := newArchive(t, root, perpKey(), window)
			b.open(0)
			for seq := uint64(1); seq <= 3; seq++ {
				b.data(seq)
			}
			b.fillAlive(cfg)
			tc.lie(b, b.lastSeq, b.records, closeDigest(b))
			b.write()

			rep := verifyOK(t, cfg)
			if got := findingKinds(rep); !slices.Equal(got, []FindingKind{tc.want}) {
				t.Fatalf("findings %v, want only %q: %s", got, tc.want, dumpFindings(rep))
			}
		})
	}
}

// TestVerifyReportsALyingCloseRecordOnce covers which of two disagreeing values
// the cross-file chain carries forward, and it exists because a mutation of that
// line SURVIVED: in every other fixture the Close record's claim and the file's
// last Data sequence agree, so the choice was unobservable.
//
// They disagree exactly when the Close record lies — and the successor's
// prev_last_seq is written from the same value the Close record carries
// (writer.closeFile), so a successor agreeing with a lying Close is the expected
// pairing. Carrying the true sequence forward instead reports one fault twice.
func TestVerifyReportsALyingCloseRecordOnce(t *testing.T) {
	root := t.TempDir()
	first, second := fixtureWindow(t, 1), fixtureWindow(t, 2)
	cfg := builtCfg(root, first)
	cfg.To = second.Add(verifyPeriod) // both windows in range

	a := newArchive(t, root, perpKey(), first)
	a.open(0)
	for seq := uint64(1); seq <= 4; seq++ {
		a.data(seq)
	}
	a.fillAlive(cfg)
	a.closeWith(a.lastSeq+5, a.records, closeDigest(a)) // the lie: claims 9, holds 4
	a.write()

	b := newArchive(t, root, perpKey(), second)
	b.open(9) // agrees with the Close record, which is what the writer would do
	for seq := uint64(10); seq <= 12; seq++ {
		b.data(seq)
	}
	b.fillAlive(cfg)
	b.seal()
	b.write()

	rep := verifyOK(t, cfg)
	breaks := findingsFor(rep, FindingCrossFileBreak)
	if len(breaks) != 1 {
		t.Fatalf("%d cross-file breaks, want exactly 1 — the lie is one fault: %s",
			len(breaks), dumpFindings(rep))
	}
	if !breaks[0].Window.Equal(first) {
		t.Errorf("the break is attributed to %s, want the window whose Close record lied (%s)",
			breaks[0].Window.Format(time.RFC3339), first.Format(time.RFC3339))
	}
	// And no hole: the successor continues from the sequence it claimed.
	if got := findingsFor(rep, FindingSeqHoleUnexplained); len(got) != 0 {
		t.Errorf("the successor was also faulted for a hole: %s", dumpFindings(rep))
	}
}

// TestVerifyReportsAFileWhoseOpenRecordNamesAnotherStream is the check
// writer.openPayload's six structural fields were added for — and they buy
// nothing at all unless something compares them.
func TestVerifyReportsAFileWhoseOpenRecordNamesAnotherStream(t *testing.T) {
	root, window := t.TempDir(), fixtureWindow(t, 1)
	cfg := builtCfg(root, window)
	b := newArchive(t, root, perpKey(), window)
	// Written into the perp stream's tree while naming the spot stream: exactly
	// what one `mv` by a sync script produces, and otherwise undetectable because
	// a frame carries no SourceID.
	b.openAs(spotKey(), window, 0)
	b.data(1)
	b.fillAlive(cfg)
	b.seal()
	b.write()

	rep := verifyOK(t, cfg)
	strays := findingsFor(rep, FindingStrayFile)
	if len(strays) != 1 {
		t.Fatalf("%d stray findings, want 1: %s", len(strays), dumpFindings(rep))
	}
	if !strings.Contains(strays[0].Detail, spotKey().Topic) || !strings.Contains(strays[0].Detail, perpKey().Topic) {
		t.Errorf("detail %q does not name both the record's stream and the path's", strays[0].Detail)
	}
}

// TestVerifyReportsAWindowWithNoVenueFrame is §32k's Binance trap at hour
// granularity: a process that connected, waited politely and captured nothing.
//
// Every byte is valid, the digests match, the Alive ticks are all present. The
// hour differs from a healthy one only in a count nobody compared against zero.
func TestVerifyReportsAWindowWithNoVenueFrame(t *testing.T) {
	h := newHarness(t, perpKey())
	h.runIdle(60)
	h.stopAll()

	rep := verifyOK(t, h.cfgWindow(1))
	if got := findingKinds(rep); !slices.Equal(got, []FindingKind{FindingNoData}) {
		t.Fatalf("findings %v, want only no_data: %s", got, dumpFindings(rep))
	}
	if got := rep.Windows[0].Verdict; got != VerdictGapped {
		t.Errorf("verdict %q, want %q", got, VerdictGapped)
	}
	// Not absent: a file exists and the process was demonstrably alive. The two
	// send an operator to different places — one to the disk, one to the venue.
	if rep.Windows[0].AliveTicks == 0 {
		t.Errorf("the fixture wrote no Alive ticks, so no_data and absent collapse")
	}
	if got := rep.ExitClass(); got != ClassCoverage {
		t.Errorf("ExitClass = %q, want %q", got, ClassCoverage)
	}
}

// TestVerifyReportsShortAliveCoverage drives a writer whose Tick was not called,
// which is a stalled pump — and Writer.Tick deliberately does not backfill the
// missed ticks, precisely so the count comes up short here.
func TestVerifyReportsShortAliveCoverage(t *testing.T) {
	h := newHarness(t, perpKey())
	h.run(10)
	h.runUnticked(35) // a stall long enough to lose several interval boundaries
	h.run(15)
	h.stopAll()

	rep := verifyOK(t, h.cfgWindow(1))
	if got := findingsFor(rep, FindingAliveShort); len(got) != 1 {
		t.Fatalf("%d alive-short findings, want 1: %s", len(got), dumpFindings(rep))
	}
	w := rep.Windows[0]
	if w.AliveTicks >= w.AliveExpected {
		t.Fatalf("%d ticks against %d expected: the stall did not lose one, so the check is vacuous",
			w.AliveTicks, w.AliveExpected)
	}
	if w.AliveTicks == 0 {
		t.Errorf("the fixture lost EVERY tick, so short coverage and no coverage collapse")
	}
	if got := rep.ExitClass(); got != ClassCoverage {
		t.Errorf("ExitClass = %q, want %q", got, ClassCoverage)
	}
}

// TestVerifyReadsARestartTheWayTheWriterRecordsIt covers both shapes a process
// restart takes, because the writer records them differently and only one of them
// is FindingRestartBoundary.
//
// Which shape you get depends on whether the new run lands in an hour the old one
// had already opened, and the difference is not cosmetic: appending to an existing
// file recovers the predecessor's sequence from disk, so continuity is unbroken and
// the restart is a minted gap record; opening a FRESH file cannot recover anything,
// so prev_last_seq is zero and the restart is only visible as that zero beside a
// predecessor that exists.
func TestVerifyReadsARestartTheWayTheWriterRecordsIt(t *testing.T) {
	t.Run("into an hour it had already opened, as a process_restart gap", func(t *testing.T) {
		h := newHarness(t, perpKey())
		h.run(65) // into window 2
		h.stop(perpKey())
		h.open(perpKey()) // same window, existing file
		h.run(50)
		h.stopAll()

		rep := verifyOK(t, h.cfgWindow(2))
		gaps := findingsFor(rep, FindingGapRecord)
		if len(gaps) != 1 || gaps[0].Cause != CauseProcessRestart {
			t.Fatalf("gap findings %v, want one process_restart: %s", gaps, dumpFindings(rep))
		}
		// Continuity is intact: the second Open recovered the sequence from the
		// file, so nothing here is a break or a hole.
		for _, kind := range []FindingKind{
			FindingCrossFileBreak, FindingSeqHoleUnexplained, FindingRestartBoundary,
		} {
			if got := findingsFor(rep, kind); len(got) != 0 {
				t.Errorf("a same-hour restart was reported as %s: %s", kind, dumpFindings(rep))
			}
		}
		if got := rep.Windows[0].Gaps[CauseProcessRestart]; got != 1 {
			t.Errorf("window gap count for %s = %d, want 1", CauseProcessRestart, got)
		}
	})

	t.Run("into a fresh hour, as a declared lack of predecessor", func(t *testing.T) {
		h := newHarness(t, perpKey())
		h.run(65)
		h.stop(perpKey())
		// Down for the rest of window 2 and all of nothing else: the next run
		// opens window 3 fresh, so prevLastSeq starts at zero and the archive's
		// only record of the restart is that zero beside window 2's file.
		h.clock.Advance(60 * time.Minute)
		h.open(perpKey())
		h.run(60)
		h.stopAll()

		rep := verifyOK(t, h.cfgWindow(3))
		if got := findingsFor(rep, FindingRestartBoundary); len(got) != 1 {
			t.Fatalf("%d restart boundaries, want 1: %s", len(got), dumpFindings(rep))
		}
		// The two findings a naive check would produce instead, both wrong here:
		// zero is the documented "no predecessor", not a broken claim, and the
		// reader's counter restarting is not a hole back to sequence one.
		if got := findingsFor(rep, FindingCrossFileBreak); len(got) != 0 {
			t.Errorf("a restart was reported as a cross-file break: %s", dumpFindings(rep))
		}
		if got := findingsFor(rep, FindingSeqHoleUnexplained); len(got) != 0 {
			t.Errorf("a restarted counter was reported as a hole back to one: %s", dumpFindings(rep))
		}
		if got := rep.ExitClass(); got != ClassCoverage {
			t.Errorf("ExitClass = %q, want %q: %s", got, ClassCoverage, dumpFindings(rep))
		}
	})
}

// TestVerifyReportsAContinuityItCannotCheck is the anti-vacuity row: a check that
// quietly does not run is indistinguishable from one that passed.
//
// The predecessor is corrupt, so the walk never reaches the sequence it ends on,
// so the successor's prev_last_seq claim cannot be checked. Verify says so.
func TestVerifyReportsAContinuityItCannotCheck(t *testing.T) {
	root := t.TempDir()
	first, second := fixtureWindow(t, 1), fixtureWindow(t, 2)
	cfg := builtCfg(root, second)

	a := newArchive(t, root, perpKey(), first)
	a.open(0)
	for seq := uint64(1); seq <= 4; seq++ {
		a.data(seq)
	}
	a.fillAlive(cfg)
	a.seal()
	pathA := a.write()

	b := newArchive(t, root, perpKey(), second)
	b.open(4)
	for seq := uint64(5); seq <= 7; seq++ {
		b.data(seq)
	}
	b.fillAlive(cfg)
	b.seal()
	b.write()

	// Corrupt the PREDECESSOR, which is outside the verified range — so the only
	// thing the report can say about it is that it cost a check on the file that
	// IS in range.
	corruptPayloadByte(t, pathA)

	rep := verifyOK(t, cfg)
	if got := findingsFor(rep, FindingContinuityUnknown); len(got) != 1 {
		t.Fatalf("%d unknown-continuity findings, want 1: %s", len(got), dumpFindings(rep))
	}
	// The out-of-range predecessor's own corruption is not reported: the request
	// did not ask about that window, and answering about it would make --from and
	// --to advisory.
	if got := findingsFor(rep, FindingChecksum); len(got) != 0 {
		t.Errorf("a fault in an out-of-range window was reported: %s", dumpFindings(rep))
	}
}

// TestVerifyRejectsRecordsThatArePresentAndWrong covers errors.go's other side:
// bytes that are there and are not what a writer would have produced.
//
// The three cases are chosen so none of them can be reached by flipping a byte at
// random, which is what makes them distinct from the corrupt-byte case:
//
//   - A wrong FRAME LENGTH is §33c's documented residual — the length field sits
//     OUTSIDE its own checksum, so a wrong length needs no CRC repair and is
//     refused by the bound check before a checksum is ever computed. It is the one
//     corruption that reaches a different sentinel than every other single-byte
//     flip, which is precisely why the residual is documented.
//   - A set RESERVED BIT with the checksum REPAIRED is the forward-compatibility
//     refusal: a v1.1 writer's record reaching a v1.0 reader, which would
//     otherwise return a plausible record with a flag silently dropped. Nothing
//     about those bytes is inconsistent, so only the rule catches it.
//   - A wrong MAGIC is not a record fault at all, and its finding must be the
//     file-level one — "this is not our file" sends a reader to the config, not to
//     an offset.
func TestVerifyRejectsRecordsThatArePresentAndWrong(t *testing.T) {
	cases := []struct {
		name string
		edit func(t *testing.T, body []byte)
		want []FindingKind
	}{{
		name: "a frame length below the header it must contain",
		edit: func(t *testing.T, body []byte) {
			patchFirstDataFrame(t, body, func(frame []byte) {
				binary.BigEndian.PutUint32(frame[offFrameLen:], framedHeaderLen-1)
			})
		},
		want: []FindingKind{FindingMalformed, FindingDigestMismatch},
	}, {
		name: "a reserved flag bit with the checksum repaired",
		edit: func(t *testing.T, body []byte) {
			patchFirstDataFrame(t, body, func(frame []byte) {
				frame[offFlags] = 1
				binary.BigEndian.PutUint32(frame[offChecksum:],
					crc32.Checksum(frame[offKind:], crcTable))
			})
		},
		want: []FindingKind{FindingMalformed, FindingDigestMismatch},
	}, {
		name: "a file that does not begin with the magic",
		edit: func(t *testing.T, body []byte) { copy(body, "NOTCAP") },
		// Nothing else: verify stops at the header, so every later check would
		// be a claim about a file it has not read.
		want: []FindingKind{FindingFileHeader},
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, window := t.TempDir(), fixtureWindow(t, 1)
			cfg := builtCfg(root, window)
			b := newArchive(t, root, perpKey(), window)
			b.open(0)
			for seq := uint64(1); seq <= 3; seq++ {
				b.data(seq)
			}
			b.fillAlive(cfg)
			b.seal()
			path := b.write()

			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			tc.edit(t, body)
			if err := os.WriteFile(path, body, 0o644); err != nil {
				t.Fatalf("write %s: %v", path, err)
			}

			rep := verifyOK(t, cfg)
			want := slices.Clone(tc.want)
			slices.Sort(want)
			if got := findingKinds(rep); !slices.Equal(got, want) {
				t.Fatalf("findings %v, want %v: %s", got, want, dumpFindings(rep))
			}
			// Corruption is never a torn tail — errors.go's header note — and a
			// corrupt file must not also be faulted for the coverage it stopped
			// verify from measuring.
			for _, kind := range []FindingKind{FindingTornTail, FindingAliveShort, FindingNoData} {
				if got := findingsFor(rep, kind); len(got) != 0 {
					t.Errorf("corruption was also reported as %s: %s", kind, dumpFindings(rep))
				}
			}
			if got := rep.ExitClass(); got != ClassIntegrity {
				t.Errorf("ExitClass = %q, want %q", got, ClassIntegrity)
			}
		})
	}
}

// patchFirstDataFrame hands edit the bytes of the first Data record's whole frame,
// located by walking rather than by a hard-coded offset — the record sizes here
// depend on a JSON payload, so an offset literal would drift silently.
func patchFirstDataFrame(t *testing.T, body []byte, edit func(frame []byte)) {
	t.Helper()
	off, err := ParseFileHeader(body)
	if err != nil {
		t.Fatalf("file header: %v", err)
	}
	for off < len(body) {
		rec, used, err := DecodeRecord(body[off:])
		if err != nil {
			t.Fatalf("at %d: %v", off, err)
		}
		if rec.Kind == RecordKindData {
			edit(body[off : off+used])
			return
		}
		off += used
	}
	t.Fatal("the fixture holds no Data record")
}

// TestVerifyRefusesARangeWithNoCompleteWindow is the vacuity refusal, and it
// carries its own positive control: the same request with a whole window in it
// must be accepted, or "refuses" would be indistinguishable from "refuses
// everything".
func TestVerifyRefusesARangeWithNoCompleteWindow(t *testing.T) {
	h := newHarness(t, perpKey())
	h.run(60)
	h.stopAll()
	base := h.cfgWindow(1)

	cases := []struct {
		name string
		edit func(cfg *VerifyConfig)
	}{
		{name: "a range shorter than a window", edit: func(c *VerifyConfig) { c.To = c.From.Add(59 * time.Minute) }},
		{name: "a range straddling a boundary", edit: func(c *VerifyConfig) {
			c.From = c.From.Add(30 * time.Minute)
			c.To = c.From.Add(verifyPeriod)
		}},
		{name: "an empty range", edit: func(c *VerifyConfig) { c.To = c.From }},
		{name: "a backwards range", edit: func(c *VerifyConfig) { c.From, c.To = c.To, c.From }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := base
			tc.edit(&cfg)
			if _, err := Verify(cfg); !errors.Is(err, ErrVerifyRangeEmpty) {
				t.Fatalf("Verify = %v, want %v", err, ErrVerifyRangeEmpty)
			}
		})
	}

	// The positive control.
	if _, err := Verify(base); err != nil {
		t.Fatalf("a range holding one whole window was refused: %v", err)
	}

	t.Run("and a threshold outside 1..100", func(t *testing.T) {
		for _, pct := range []uint{0, 101} {
			cfg := base
			cfg.MinAlivePercent = pct
			if _, err := Verify(cfg); !errors.Is(err, ErrAlivePercentRange) {
				t.Errorf("MinAlivePercent %d: Verify = %v, want %v", pct, err, ErrAlivePercentRange)
			}
		}
	})
}

// TestExpectedAliveTicksUsesTheWritersOwnGrid pins the defect this function was
// written wrong once already.
//
// time.Time.Truncate — which Writer.nextTick uses to anchor ticks — rounds down to
// a multiple of d SINCE THE ZERO TIME, not since the Unix epoch. The epoch sits
// 62135596800 s after the zero time and 62135596800 mod 420 is 60, so a 7-minute
// grid computed over UnixNano is shifted one minute from the writer's, and one
// boundary crosses each hour edge: the count reads one high for one window and one
// low for the next, for a perfectly healthy archive.
//
// The two grids agree exactly when the interval divides 60 s, so a fixture using a
// one-minute Alive interval cannot see this at all. Every case below uses an
// interval that does not divide a minute.
// A second collapse the checklist caught here, before the mutations rather than
// after: the closed-open arithmetic (`(end.Sub(first)-1)/interval + 1`) differs
// from the naive `end.Sub(first)/interval + 1` ONLY when a boundary lands exactly
// on the window's end, and with a 7-minute interval in a 60-minute window that
// never happens — 60 is not a multiple of 7. Every case in the table below would
// have passed with the off-by-one in place. The 20-minute row is the one that
// distinguishes them, because a 20-minute grid is aligned to the hour and so puts
// a boundary on both edges of every window.
func TestExpectedAliveTicksUsesTheWritersOwnGrid(t *testing.T) {
	grids := []struct {
		name             string
		period, interval time.Duration
	}{
		{name: "coprime, the production pair", period: time.Hour, interval: 7 * time.Minute},
		{name: "a boundary on both window edges", period: time.Hour, interval: 20 * time.Minute},
		{name: "interval equal to the period", period: time.Hour, interval: time.Hour},
		{name: "interval longer than the period", period: time.Hour, interval: 90 * time.Minute},
		{name: "a sub-hour period", period: 30 * time.Minute, interval: 7 * time.Minute},
	}
	for _, g := range grids {
		t.Run(g.name, func(t *testing.T) {
			for n := 1; n <= 5; n++ {
				window := fixtureWindow(t, n)
				want := 0
				// The oracle: walk the writer's own grid, one boundary at a
				// time, with no arithmetic in common with the implementation.
				for tick := window.Truncate(g.interval); tick.Before(window.Add(g.period)); tick = tick.Add(g.interval) {
					if !tick.Before(window) {
						want++
					}
				}
				if got := expectedAliveTicks(window, g.period, g.interval); got != uint64(want) {
					t.Errorf("%s: expected %d ticks, the writer's grid holds %d",
						window.Format(time.RFC3339), got, want)
				}
			}
		})
	}

	// The 20-minute grid is only a discriminator if it really does land on the
	// window edges. Asserted, because a grid that did not would make the row
	// above a duplicate of the first one.
	edge := fixtureWindow(t, 2)
	if !edge.Truncate(20 * time.Minute).Equal(edge) {
		t.Errorf("%s is not a 20-minute boundary, so the exact-division case is untested",
			edge.Format(time.RFC3339))
	}

	// The window-start boundary belongs to the window that STARTS there, because
	// Writer.Tick rotates before it writes — so the boundary tick lands in the new
	// file. A window whose start is itself a boundary is where the two readings
	// differ by one.
	aligned := fixtureWindow(t, 2) // 15:00, which is a 7-minute boundary
	if !aligned.Truncate(verifyAlive).Equal(aligned) {
		t.Fatalf("%s is not an interval boundary, so this case does not test what it claims",
			aligned.Format(time.RFC3339))
	}
	if got, prev := expectedAliveTicks(aligned, verifyPeriod, verifyAlive),
		expectedAliveTicks(aligned.Add(-verifyPeriod), verifyPeriod, verifyAlive); got <= prev {
		t.Errorf("a window starting on a boundary expects %d ticks and its predecessor %d; "+
			"the boundary must count for the window it starts", got, prev)
	}
}
