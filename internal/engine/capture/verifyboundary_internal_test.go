package capture

import (
	"strings"
	"testing"
	"time"
)

// §71 — WHICH zero a prev_last_seq of zero is.
//
// `restart_boundary` used to be one name over a product of two properties: the
// claim, and whether a sequence existed to lose. On a REST-overlay source the
// second property is permanently false — slice E settled that a Rest record is
// not a Data record, so lastSeq never leaves zero and every hourly rotation
// declares no predecessor truthfully — and the finding fired 24 times a day per
// source, each time marking its window VerdictGapped for an hour in which a
// living writer rotated a file and lost nothing.
//
// The fixtures below are the product table. The first uses the REAL writer,
// because a REST-overlay stream is something the writer produces in production
// and a hand-built one would only test this file's reading of the format. The
// rest are hand-built for the state one test process cannot reach: an archive
// written by two different processes.

// restPayload is one recorded venue ask, shaped the way slice E frames them.
func restPayload(t *testing.T, at time.Time) []byte {
	t.Helper()
	payload, err := EncodeRestPayload(RestPayload{
		Meta: RestMeta{
			Method:  "GET",
			URL:     "https://fapi.binance.com/fapi/v1/fundingInfo",
			Status:  200,
			AskedAt: at,
		},
		Body: []byte(`[{"symbol":"BTCUSDT","adjustedFundingRateCap":"0.02"}]`),
	})
	if err != nil {
		t.Fatalf("encode rest payload: %v", err)
	}
	return payload
}

// runOverlay advances n minutes writing a REST answer and a tick per minute and
// NO venue frames — a `fundingInfo` source, which is the shape §71 is about.
//
// Deliberately through Writer.Rest rather than a hand-built file: the claim is
// that a real overlay stream rotates without a restart, and the writer is what
// decides what its Open records say.
func (h *harness) runOverlay(n int) {
	h.t.Helper()
	for range n {
		h.step++
		h.clock.Advance(time.Minute)
		for _, k := range h.keys {
			w, open := h.writers[k.String()]
			if !open {
				continue
			}
			if err := w.Rest(restPayload(h.t, h.clock.Now())); err != nil {
				h.t.Fatalf("%s Rest: %v", k, err)
			}
		}
		h.writeTicks()
	}
}

// rest appends one recorded venue ask to a hand-built file. Every unsequenced
// fixture holds at least one, and that is not decoration: a file with neither a
// Data nor a Rest record is §32k's dead feed and earns FindingNoData, which
// would flag the window through a second door and make the verdict assertions
// below say nothing.
func (b *archiveBuilder) rest(t *testing.T) {
	b.t.Helper()
	b.emit(RecordKindRest, b.lastSeq, restPayload(t, b.at))
}

// TestVerifyReadsAnOverlayRotationAsTheNonEventItIs is §71's motivating case,
// end to end through the real writer: a REST-overlay stream rotating hourly with
// nobody restarting anything.
//
// THREE covered windows rather than two, and the count is the point: with one
// boundary in range, "one finding per boundary" and "one finding per stream"
// give the same answer, and the fixture would pass against a check that fired
// once and stopped.
func TestVerifyReadsAnOverlayRotationAsTheNonEventItIs(t *testing.T) {
	h := newHarness(t, perpKey())
	// The healthy-archive span: four hours and five minutes, so the three windows
	// the request names are each covered from their first minute to their last and
	// no window is short an Alive tick.
	h.runOverlay(4*60 + 5)
	h.stopAll()

	rep := verifyOK(t, h.cfg())

	// The fixture contains the thing under test. Without this the whole test could
	// pass over a stream that simply had no files.
	if len(rep.Windows) != 3 {
		t.Fatalf("%d windows in range, want 3: the fixture is not what the assertions assume", len(rep.Windows))
	}
	for _, w := range rep.Windows {
		if w.RestRecords == 0 {
			t.Fatalf("window %s holds no Rest record, so it is not an overlay stream",
				w.Window.Format(time.RFC3339))
		}
		if w.DataRecords != 0 {
			t.Fatalf("window %s holds %d Data records, so it sequences and §71's case does not apply",
				w.Window.Format(time.RFC3339), w.DataRecords)
		}
	}

	if got := findingsFor(rep, FindingRestartBoundary); len(got) != 0 {
		t.Errorf("%d restart boundaries on a stream nobody restarted: %s", len(got), dumpFindings(rep))
	}
	boundaries := findingsFor(rep, FindingUnsequencedBoundary)
	if len(boundaries) != 3 {
		t.Fatalf("%d unsequenced boundaries, want one per in-range rotation (3): %s",
			len(boundaries), dumpFindings(rep))
	}
	if !strings.Contains(boundaries[0].Detail, "same writer") {
		t.Errorf("detail %q does not say what was concluded", boundaries[0].Detail)
	}

	// The row is REPORTED — this ruling renamed a finding, it did not silence one.
	if got := rep.ExitClass(); got != ClassCoverage {
		t.Errorf("ExitClass = %q, want %q: the boundary must still be on the report", got, ClassCoverage)
	}

	// And the verdict is the honest one: a replay of these hours gets everything
	// the archive holds, which is what VerdictReplayable means.
	for _, w := range rep.Windows {
		if w.Verdict != VerdictReplayable {
			t.Errorf("window %s is %q, want %q — a living writer's rotation costs a replay nothing",
				w.Window.Format(time.RFC3339), w.Verdict, VerdictReplayable)
		}
	}
}

// TestVerifyKeepsTheRestartNameWhenAnythingCouldHaveBeenLost is the product
// table, one row per dimension the downgrade depends on.
//
// Row one is the control and every other row differs from it in exactly ONE
// thing, which is what makes each row evidence about its own dimension rather
// than about the conjunction. The verdict is asserted on every row for the same
// reason: "unsequenced_boundary does not flag its window" is only meaningful
// beside a row where a boundary DOES.
func TestVerifyKeepsTheRestartNameWhenAnythingCouldHaveBeenLost(t *testing.T) {
	const (
		otherHost = "another-host"
		otherPID  = 4242
	)
	cases := []struct {
		name string
		// predecessor builds the earlier window's file. The successor is always
		// the same: an unsequenced file declaring no predecessor, written by
		// (fixtureHost, fixturePID).
		predecessor func(t *testing.T, b *archiveBuilder, cfg VerifyConfig)
		want        FindingKind
		wantDetail  string
		wantVerdict Verdict
	}{{
		name: "the same writer rotating an unsequenced stream",
		predecessor: func(t *testing.T, b *archiveBuilder, cfg VerifyConfig) {
			b.open(0)
			b.rest(t)
			b.fillAlive(cfg)
			b.seal()
		},
		want:        FindingUnsequencedBoundary,
		wantDetail:  "same writer",
		wantVerdict: VerdictReplayable,
	}, {
		name: "a different PID on the same host",
		predecessor: func(t *testing.T, b *archiveBuilder, cfg VerifyConfig) {
			b.openAsProcess(b.key, b.window, 0, fixtureHost, otherPID)
			b.rest(t)
			b.fillAlive(cfg)
			b.seal()
		},
		want:        FindingRestartBoundary,
		wantDetail:  "writer fixture/4242 to fixture/1",
		wantVerdict: VerdictGapped,
	}, {
		// The mirror of the row above, and it is not redundant: with only one of
		// them, "compares the pid" and "compares host and pid" are the same
		// predicate, and an archive synced from a second machine is exactly the
		// case that separates them.
		name: "a different HOST with the same PID",
		predecessor: func(t *testing.T, b *archiveBuilder, cfg VerifyConfig) {
			b.openAsProcess(b.key, b.window, 0, otherHost, fixturePID)
			b.rest(t)
			b.fillAlive(cfg)
			b.seal()
		},
		want:        FindingRestartBoundary,
		wantDetail:  "writer another-host/1 to fixture/1",
		wantVerdict: VerdictGapped,
	}, {
		name: "the predecessor held a sequence",
		predecessor: func(t *testing.T, b *archiveBuilder, cfg VerifyConfig) {
			b.open(0)
			for seq := uint64(1); seq <= 5; seq++ {
				b.data(seq)
			}
			b.fillAlive(cfg)
			b.seal()
		},
		want:        FindingRestartBoundary,
		wantDetail:  "has carried a sequence",
		wantVerdict: VerdictGapped,
	}, {
		// The same sequence with NO Close record, which is the ordinary crash: a
		// run that died mid-hour and a fresh one that opened the next. It is a
		// separate row because a sealed file states its last sequence twice — in
		// its records and in its Close payload — so the sealed row above cannot
		// say WHICH of the two the rule read.
		name: "the predecessor held a sequence and never closed",
		predecessor: func(t *testing.T, b *archiveBuilder, cfg VerifyConfig) {
			b.open(0)
			for seq := uint64(1); seq <= 5; seq++ {
				b.data(seq)
			}
			b.fillAlive(cfg)
		},
		want:        FindingRestartBoundary,
		wantDetail:  "has carried a sequence",
		wantVerdict: VerdictGapped,
	}, {
		// The mirror of the chain's own rule: verify carries the CLOSE RECORD'S
		// claim forward rather than the sequence the records reached, because the
		// successor's prev_last_seq is written from that same number. A Close
		// naming a sequence its records never showed is therefore evidence a
		// sequence existed, and the fact this rule reads has to agree with the
		// fact the chain carries.
		name: "the predecessor's Close record claims a sequence its records never showed",
		predecessor: func(t *testing.T, b *archiveBuilder, cfg VerifyConfig) {
			b.open(0)
			b.rest(t)
			b.fillAlive(cfg)
			b.closeWith(5, b.records, closeDigest(b))
		},
		want:        FindingRestartBoundary,
		wantDetail:  "has carried a sequence",
		wantVerdict: VerdictGapped,
	}, {
		// Unread bytes are not empty bytes: the Data record that would have made
		// this stream a sequencing one may be sitting in the tail nobody decoded.
		name: "the predecessor ends in a torn tail",
		predecessor: func(t *testing.T, b *archiveBuilder, cfg VerifyConfig) {
			b.open(0)
			b.rest(t)
			b.fillAlive(cfg)
			b.buf = append(b.buf, 0x00, 0x01, 0x02) // a record that starts and does not finish
		},
		want:        FindingRestartBoundary,
		wantDetail:  "stopped short",
		wantVerdict: VerdictGapped,
	}, {
		// An identity that could not be read must not compare equal to another
		// one that could not be read, which is what a bare struct comparison
		// would do with two zero values.
		name: "the predecessor's Open record does not parse",
		predecessor: func(t *testing.T, b *archiveBuilder, cfg VerifyConfig) {
			b.emit(RecordKindOpen, b.lastSeq, []byte(`{"venue":`))
			b.rest(t)
			b.fillAlive(cfg)
			b.seal()
		},
		want:        FindingRestartBoundary,
		wantDetail:  "writer unknown to fixture/1",
		wantVerdict: VerdictGapped,
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			first, second := fixtureWindow(t, 1), fixtureWindow(t, 2)
			cfg := builtCfg(root, second)

			a := newArchive(t, root, perpKey(), first)
			tc.predecessor(t, a, cfg)
			a.write()

			b := newArchive(t, root, perpKey(), second)
			b.open(0)
			b.rest(t)
			b.fillAlive(cfg)
			b.seal()
			b.write()

			rep := verifyOK(t, cfg)
			got := findingsFor(rep, tc.want)
			if len(got) != 1 {
				t.Fatalf("%d %s findings, want 1: %s", len(got), tc.want, dumpFindings(rep))
			}
			if !strings.Contains(got[0].Detail, tc.wantDetail) {
				t.Errorf("detail %q does not name the reason (%q)", got[0].Detail, tc.wantDetail)
			}
			other := FindingRestartBoundary
			if tc.want == FindingRestartBoundary {
				other = FindingUnsequencedBoundary
			}
			if n := len(findingsFor(rep, other)); n != 0 {
				t.Errorf("%d %s findings beside it: one seam, one name: %s", n, other, dumpFindings(rep))
			}
			// Never integrity: every row here is an honest archive.
			if got := rep.ExitClass(); got == ClassIntegrity {
				t.Errorf("ExitClass = %q on an honest archive: %s", got, dumpFindings(rep))
			}
			if len(rep.Windows) != 1 {
				t.Fatalf("%d windows in range, want 1", len(rep.Windows))
			}
			if v := rep.Windows[0].Verdict; v != tc.wantVerdict {
				t.Errorf("verdict %q, want %q: %s", v, tc.wantVerdict, dumpFindings(rep))
			}
		})
	}
}

// TestVerifyJudgesASecondOpenAgainstTheOneBeforeIt is the seam INSIDE a file:
// §38i's shape, where a run died mid-hour and its replacement reopened the file
// it was already writing.
//
// It matters here because that seam takes the same zero claim — an unsequenced
// writer that reopens has nothing to recover — and the identity it must be
// compared against is the Open record above it in the SAME file, not the last
// file's. A rule that only carried identity across files would compare this
// file's second Open against its predecessor file and answer about the wrong
// pair.
func TestVerifyJudgesASecondOpenAgainstTheOneBeforeIt(t *testing.T) {
	build := func(t *testing.T, secondPID int) *Report {
		t.Helper()
		root := t.TempDir()
		window := fixtureWindow(t, 1)
		cfg := builtCfg(root, window)

		b := newArchive(t, root, perpKey(), window)
		b.open(0)
		b.rest(t)
		b.openAsProcess(b.key, b.window, 0, fixtureHost, secondPID)
		b.rest(t)
		b.fillAlive(cfg)
		b.seal()
		b.write()
		return verifyOK(t, cfg)
	}

	t.Run("a replacement process reopened the file", func(t *testing.T) {
		rep := build(t, 9999)
		if got := findingsFor(rep, FindingRestartBoundary); len(got) != 1 {
			t.Fatalf("%d restart boundaries, want 1: a second process reopened this file: %s",
				len(got), dumpFindings(rep))
		}
		if n := len(findingsFor(rep, FindingUnsequencedBoundary)); n != 0 {
			t.Errorf("%d unsequenced boundaries: the writer changed mid-file: %s", n, dumpFindings(rep))
		}
	})

	t.Run("the same process reopened it", func(t *testing.T) {
		rep := build(t, fixturePID)
		if got := findingsFor(rep, FindingUnsequencedBoundary); len(got) != 1 {
			t.Fatalf("%d unsequenced boundaries, want 1: %s", len(got), dumpFindings(rep))
		}
		if n := len(findingsFor(rep, FindingRestartBoundary)); n != 0 {
			t.Errorf("%d restart boundaries: %s", n, dumpFindings(rep))
		}
	})
}

// TestVerifyRefusesAStaleIdentityAcrossAnUnreadableOpen is the case that decides
// what an unreadable Open record does to the identity carry.
//
// Three files: a readable one, one whose Open payload is garbage, and a third
// declaring no predecessor. Carrying the FIRST file's identity past the middle
// one would compare the third against a process two files away and answer "the
// same writer" about a pair that was never compared. The middle file is the
// whole fixture — with two files there is no stale identity to carry, so the
// unknown and the correct answer coincide.
//
// The RANGE is part of the fixture and was got wrong first time round: verify
// walks exactly one file earlier than the range, so a request naming only the
// third window starts the walk at the middle one, and the identity that was
// supposed to go stale was never read at all. The mutation is what said so — the
// version of this test that named one window passed with the reset deleted. The
// request therefore names windows two and three, which is also why the readable
// control expects TWO boundaries: with the first file walked, the middle file's
// own Open records a seam as well.
func TestVerifyRefusesAStaleIdentityAcrossAnUnreadableOpen(t *testing.T) {
	build := func(t *testing.T, breakMiddle bool) (*Report, time.Time) {
		t.Helper()
		root := t.TempDir()
		w1, w2, w3 := fixtureWindow(t, 1), fixtureWindow(t, 2), fixtureWindow(t, 3)
		cfg := builtCfg(root, w2)
		cfg.To = w3.Add(verifyPeriod)

		a := newArchive(t, root, perpKey(), w1)
		a.open(0)
		a.rest(t)
		a.fillAlive(cfg)
		a.seal()
		a.write()

		b := newArchive(t, root, perpKey(), w2)
		if breakMiddle {
			b.emit(RecordKindOpen, b.lastSeq, []byte(`{"venue":`))
		} else {
			b.open(0)
		}
		b.rest(t)
		b.fillAlive(cfg)
		b.seal()
		b.write()

		c := newArchive(t, root, perpKey(), w3)
		c.open(0)
		c.rest(t)
		c.fillAlive(cfg)
		c.seal()
		c.write()

		return verifyOK(t, cfg), w3
	}

	t.Run("the identity in between could not be read", func(t *testing.T) {
		rep, w3 := build(t, true)
		got := findingsFor(rep, FindingRestartBoundary)
		if len(got) != 1 {
			t.Fatalf("%d restart boundaries, want 1: an unreadable identity is not an equal one: %s",
				len(got), dumpFindings(rep))
		}
		// Named, because the claim is about WHICH seam: the third file's, judged
		// against an identity that could not be read rather than against the first
		// file's.
		if !got[0].Window.Equal(w3) {
			t.Errorf("the restart is reported at %s, want the third window %s",
				got[0].Window.Format(time.RFC3339), w3.Format(time.RFC3339))
		}
		if n := len(findingsFor(rep, FindingUnsequencedBoundary)); n != 0 {
			t.Errorf("%d unsequenced boundaries: the comparison used a stale identity: %s",
				n, dumpFindings(rep))
		}
	})

	t.Run("the same archive with that Open readable", func(t *testing.T) {
		rep, _ := build(t, false)
		if got := findingsFor(rep, FindingUnsequencedBoundary); len(got) != 2 {
			t.Fatalf("%d unsequenced boundaries, want 2 (both in-range seams): %s",
				len(got), dumpFindings(rep))
		}
		if n := len(findingsFor(rep, FindingRestartBoundary)); n != 0 {
			t.Errorf("%d restart boundaries: %s", n, dumpFindings(rep))
		}
	})
}

// TestVerifyTreatsAnAdoptedClaimAsEvidenceOfASequence is the narrowed-range case,
// and it is the reason `sequenced` is set from Open records and not from Data
// records alone.
//
// Verify walks at most ONE file earlier than the requested range. When that one
// file is an hour in which the feed was silent, there is no Data record in it to
// see — the only thing saying this stream sequences at all is the prev_last_seq
// it claims. A downgrade keyed on "no Data record was read" would call the next
// seam a non-event on a stream that had been sequencing for weeks.
//
// The two subtests are the same archive but for that one claim, so what is
// varied is exactly what the rule reads.
func TestVerifyTreatsAnAdoptedClaimAsEvidenceOfASequence(t *testing.T) {
	build := func(t *testing.T, silentClaims uint64) *Report {
		t.Helper()
		root := t.TempDir()
		w1, w2, w3 := fixtureWindow(t, 1), fixtureWindow(t, 2), fixtureWindow(t, 3)
		cfg := builtCfg(root, w3)

		// Out of the walk entirely: the range names window 3, so the walk starts at
		// window 2, the last file before it.
		a := newArchive(t, root, perpKey(), w1)
		a.open(0)
		for seq := uint64(1); seq <= 5; seq++ {
			a.data(seq)
		}
		a.fillAlive(cfg)
		a.seal()
		a.write()

		// The silent hour: alive, answering REST, holding not one venue frame.
		//
		// Deliberately NOT sealed. A sealed file states its sequence twice — in its
		// Open claim and in its Close payload — and either one alone is enough to
		// answer §71's question, so a sealed fixture here proves nothing about
		// which of them the rule reads. (Found by mutation: dropping the Open-claim
		// half survived against the sealed version of this fixture.) The Close
		// record's own half is a separate row in the table test above.
		//
		// The writer identity is held at (fixtureHost, fixturePID) throughout for
		// the same reason: with the process varying too, the identity conjunct
		// could be what produced the answer.
		b := newArchive(t, root, perpKey(), w2)
		b.open(silentClaims)
		b.rest(t)
		b.fillAlive(cfg)
		b.write()

		c := newArchive(t, root, perpKey(), w3)
		c.open(0)
		c.rest(t)
		c.fillAlive(cfg)
		c.seal()
		c.write()

		return verifyOK(t, cfg)
	}

	t.Run("the silent hour claims a predecessor, so the seam after it is a restart", func(t *testing.T) {
		rep := build(t, 5)
		if got := findingsFor(rep, FindingRestartBoundary); len(got) != 1 {
			t.Fatalf("%d restart boundaries, want 1: an adopted claim is evidence of a sequence: %s",
				len(got), dumpFindings(rep))
		}
		if n := len(findingsFor(rep, FindingUnsequencedBoundary)); n != 0 {
			t.Errorf("%d unsequenced boundaries: the stream had a sequence to lose: %s", n, dumpFindings(rep))
		}
	})

	t.Run("the same archive with no claim to adopt", func(t *testing.T) {
		rep := build(t, 0)
		if got := findingsFor(rep, FindingUnsequencedBoundary); len(got) != 1 {
			t.Fatalf("%d unsequenced boundaries, want 1: %s", len(got), dumpFindings(rep))
		}
		if n := len(findingsFor(rep, FindingRestartBoundary)); n != 0 {
			t.Errorf("%d restart boundaries: %s", n, dumpFindings(rep))
		}
	})
}

// TestSeenSequenceKeepsIgnoranceApartFromAbsence is the unit half: none() takes
// BOTH fields, because "no sequence here" and "no sequence I could see" are
// opposite answers to §71's question and a single boolean would merge them.
func TestSeenSequenceKeepsIgnoranceApartFromAbsence(t *testing.T) {
	cases := []struct {
		seen seenSequence
		want bool
	}{
		{seenSequence{}, true},
		{seenSequence{any: true}, false},
		{seenSequence{truncated: true}, false},
		{seenSequence{any: true, truncated: true}, false},
	}
	for _, tc := range cases {
		if got := tc.seen.none(); got != tc.want {
			t.Errorf("seenSequence{any:%v, truncated:%v}.none() = %v, want %v",
				tc.seen.any, tc.seen.truncated, got, tc.want)
		}
	}
}

// TestWriterIdentitySameRefusesTheUnknown pins the zero-value trap directly: two
// unreadable identities are not the same writer, however equal their fields are.
func TestWriterIdentitySameRefusesTheUnknown(t *testing.T) {
	known := writerIdentity{host: "h", pid: 7, known: true}
	cases := []struct {
		name string
		a, b writerIdentity
		want bool
	}{
		{"the same process", known, known, true},
		{"a different pid", known, writerIdentity{host: "h", pid: 8, known: true}, false},
		{"a different host", known, writerIdentity{host: "g", pid: 7, known: true}, false},
		{"an unknown beside a known", writerIdentity{}, known, false},
		{"a known beside an unknown", known, writerIdentity{}, false},
		{"two unknowns", writerIdentity{}, writerIdentity{}, false},
		{"an unknown that carries fields", writerIdentity{host: "h", pid: 7}, known, false},
	}
	for _, tc := range cases {
		if got := tc.a.same(tc.b); got != tc.want {
			t.Errorf("%s: same() = %v, want %v", tc.name, got, tc.want)
		}
	}
	if got := (writerIdentity{}).String(); got != "unknown" {
		t.Errorf("unknown identity renders as %q", got)
	}
	if got := known.String(); got != "h/7" {
		t.Errorf("known identity renders as %q, want h/7", got)
	}
}

// The per-kind verdict table lives in finding_test.go, beside the roster it has
// to agree with.
