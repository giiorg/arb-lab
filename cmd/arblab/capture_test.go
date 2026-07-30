package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	enginecapture "github.com/giiorg/arb-lab/internal/engine/capture"
)

// TestExitCodeSpellsEveryClass pins the class→code map. The PRECEDENCE between
// classes is Report.ExitClass's and is deliberately not exercised here — this
// command owes only the spelling, and a test of precedence in this package
// would be a second place the decision appears to live.
func TestExitCodeSpellsEveryClass(t *testing.T) {
	cases := []struct {
		class enginecapture.FindingClass
		want  int
	}{
		{enginecapture.ClassUnspecified, 0},
		{enginecapture.ClassIntegrity, 1},
		{enginecapture.ClassCoverage, 2},
	}
	for _, tc := range cases {
		if got := exitCode(tc.class); got != tc.want {
			t.Errorf("exitCode(%q) = %d, want %d", tc.class, got, tc.want)
		}
	}
}

// TestFormatReportKeepsTheAxesApart: §36i's operator-facing rule — the two
// finding classes are printed separately and never summed into one "problems"
// count. The fixture holds one integrity and two coverage findings, chosen so
// a summed count (3) is a number no honest line contains.
func TestFormatReportKeepsTheAxesApart(t *testing.T) {
	window := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	key := enginecapture.StreamKey{}
	rep := &enginecapture.Report{
		From: window, To: window.Add(time.Hour),
		Windows: []enginecapture.WindowReport{{
			Stream: key, Window: window, Verdict: enginecapture.VerdictGapped,
			Records: 10, DataRecords: 7, AliveTicks: 50, AliveExpected: 60,
			FirstSeq: 1, LastSeq: 7,
			Gaps: map[enginecapture.Cause]uint64{enginecapture.CauseReconnect: 2},
		}},
		Findings: []enginecapture.Finding{
			{Kind: enginecapture.FindingChecksum, Offset: 40000, Path: "x.arbcap", Detail: "crc"},
			{Kind: enginecapture.FindingAliveShort, Offset: -1, Detail: "50/60"},
			{Kind: enginecapture.FindingGapRecord, Offset: -1, Cause: enginecapture.CauseReconnect, Detail: "socket"},
		},
	}
	var out bytes.Buffer
	formatReport(&out, rep, enginecapture.VerifyConfig{Root: "/r", MinAlivePercent: 90})
	got := out.String()

	if !strings.Contains(got, "findings: 1 integrity, 2 coverage") {
		t.Errorf("the report does not state the two axes separately:\n%s", got)
	}
	if strings.Contains(got, "3 findings") || strings.Contains(got, "findings: 3") {
		t.Errorf("the report summed the two axes into one count:\n%s", got)
	}
	// Integrity present → the verdict line must say so, and must be the
	// distrust sentence rather than the thin-archive one.
	if !strings.Contains(got, "INTEGRITY") {
		t.Errorf("no integrity verdict line:\n%s", got)
	}
	if !strings.Contains(got, "gapped") || !strings.Contains(got, "alive 50/60") {
		t.Errorf("the window row lost its verdict or its counts:\n%s", got)
	}
	if !strings.Contains(got, "reconnect×2") {
		t.Errorf("the window row lost its gap tally:\n%s", got)
	}
}

// TestFormatReportCleanVerdict: the healthy case prints "clean" — the positive
// control for the formatter, without which a formatter that printed INTEGRITY
// unconditionally would pass the test above.
func TestFormatReportCleanVerdict(t *testing.T) {
	window := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	rep := &enginecapture.Report{
		From: window, To: window.Add(time.Hour),
		Windows: []enginecapture.WindowReport{{
			Window: window, Verdict: enginecapture.VerdictReplayable,
			Records: 64, DataRecords: 60, AliveTicks: 60, AliveExpected: 60,
			FirstSeq: 1, LastSeq: 60,
		}},
	}
	var out bytes.Buffer
	formatReport(&out, rep, enginecapture.VerifyConfig{Root: "/r", MinAlivePercent: 90})
	got := out.String()
	if !strings.Contains(got, "verdict: clean") {
		t.Errorf("a findingless report does not read clean:\n%s", got)
	}
	if !strings.Contains(got, "findings: 0 integrity, 0 coverage") {
		t.Errorf("the clean report hides its zero counts:\n%s", got)
	}
}

func TestDispatchRefusesMalformedInvocations(t *testing.T) {
	cases := [][]string{
		nil,
		{"capture"},
		{"capture", "replay"}, // not built yet; must refuse, not misroute
		{"verify"},
	}
	for _, args := range cases {
		var out, errOut bytes.Buffer
		if got := dispatch(args, &out, &errOut); got != 64 {
			t.Errorf("dispatch(%q) = %d, want 64", args, got)
		}
		if !strings.Contains(errOut.String(), "usage") {
			t.Errorf("dispatch(%q) printed no usage", args)
		}
	}
}

func TestCaptureVerifyRequiresItsFlags(t *testing.T) {
	var out, errOut bytes.Buffer
	if got := captureVerify(nil, &out, &errOut); got != 64 {
		t.Errorf("captureVerify with no flags = %d, want 64", got)
	}
	if got := captureRun(nil, &errOut); got != 64 {
		t.Errorf("captureRun with no flags = %d, want 64", got)
	}
}
