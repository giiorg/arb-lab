package capture_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/giiorg/arb-lab/internal/engine/capture"
)

// declaredFindingKinds is the roster §32k's report is built on, and it is also
// the checklist the test below turns into a failure.
//
// Every entry names the test that PRODUCES that finding, and a kind with no
// producer is a declared member no code path can reach — which is exactly the
// defect this file exists because of. FindingAliveShort was declared, classified,
// documented and never emitted: verify computed the coverage shortfall inline to
// set the verdict, so the verdict came out right, the finding list said nothing
// about the only thing wrong with the window, and an exit code read off that list
// reported clean. Neither the compiler, nor exhaustiveswitch, nor coverage of
// Class() could see it, because every one of them was satisfied.
var declaredFindingKinds = map[capture.FindingKind]string{
	capture.FindingChecksum:           "TestVerifyNamesTheFourFaults/one_corrupt_byte",
	capture.FindingMalformed:          "TestVerifyRejectsRecordsThatArePresentAndWrong",
	capture.FindingFileHeader:         "TestVerifyRejectsRecordsThatArePresentAndWrong",
	capture.FindingDigestMismatch:     "TestVerifyChecksTheCloseRecordsOwnClaims",
	capture.FindingDigestMissing:      "TestVerifyNamesTheFourFaults/a_missing_sidecar",
	capture.FindingRecordCount:        "TestVerifyChecksTheCloseRecordsOwnClaims",
	capture.FindingSeqNotAscending:    "TestVerifyReportsNonAscendingSequencesAsEvidenceOfTwoWriters",
	capture.FindingSeqHole:            "TestVerifySeparatesExplainedFromUnexplainedHoles",
	capture.FindingSeqHoleUnexplained: "TestVerifySeparatesExplainedFromUnexplainedHoles",
	capture.FindingCrossFileBreak:     "TestVerifyChecksTheCloseRecordsOwnClaims",
	capture.FindingStrayFile:          "TestVerifyReportsAFileWhoseOpenRecordNamesAnotherStream",
	capture.FindingWindowAbsent:       "TestVerifyNamesTheFourFaults/a_deleted_hour",
	capture.FindingTornTail:           "TestVerifyNamesTheFourFaults/truncation",
	capture.FindingNoClose:            "TestVerifyNamesTheFourFaults/truncation",
	capture.FindingNoData:             "TestVerifyReportsAWindowWithNoVenueFrame",
	capture.FindingAliveShort:         "TestVerifyReportsShortAliveCoverage",
	capture.FindingGapRecord:          "TestVerifyReadsARestartTheWayTheWriterRecordsIt",
	capture.FindingRestartBoundary:    "TestVerifyReadsARestartTheWayTheWriterRecordsIt",
	capture.FindingContinuityUnknown:  "TestVerifyReportsAContinuityItCannotCheck",
}

// TestFindingKindDeclaredSet pins the roster, the classification of every member,
// and that the zero value belongs to neither exit code.
func TestFindingKindDeclaredSet(t *testing.T) {
	var integrity, coverage int
	for kind, producer := range declaredFindingKinds {
		if err := kind.Validate(); err != nil {
			t.Errorf("declared %s: Validate() = %v, want nil", kind, err)
		}
		if producer == "" {
			t.Errorf("%s names no test that produces it: a finding nobody can reach "+
				"is reported as handled", kind)
		}
		switch kind.Class() {
		case capture.ClassIntegrity:
			integrity++
		case capture.ClassCoverage:
			coverage++
		case capture.ClassUnspecified:
			t.Errorf("declared %s has no class, so neither exit code counts it", kind)
		}
		if kind.String() != string(kind) {
			t.Errorf("%s: String() = %q", kind, kind.String())
		}
	}

	// Both classes have members. A roster that had drifted entirely into one of
	// them would make the other exit code unreachable while every per-kind
	// assertion above still passed.
	if integrity == 0 || coverage == 0 {
		t.Errorf("%d integrity and %d coverage kinds: one exit code is unreachable",
			integrity, coverage)
	}

	if err := capture.FindingUnspecified.Validate(); !errors.Is(err, capture.ErrFindingKindUnspecified) {
		t.Errorf("FindingUnspecified.Validate() = %v, want %v", err, capture.ErrFindingKindUnspecified)
	}
	if got := capture.FindingUnspecified.Class(); got != capture.ClassUnspecified {
		t.Errorf("FindingUnspecified.Class() = %q, want the unspecified class", got)
	}
	undeclared := capture.FindingKind("wobbly")
	if err := undeclared.Validate(); !errors.Is(err, capture.ErrFindingKindUnknown) {
		t.Errorf("FindingKind(%q).Validate() = %v, want %v", undeclared, err, capture.ErrFindingKindUnknown)
	}
	if got := undeclared.Class(); got != capture.ClassUnspecified {
		t.Errorf("FindingKind(%q).Class() = %q, want the unspecified class", undeclared, got)
	}
}

// TestFindingClassDeclaredSet is the two exit codes, and the zero value that is
// neither.
func TestFindingClassDeclaredSet(t *testing.T) {
	for _, c := range []capture.FindingClass{capture.ClassIntegrity, capture.ClassCoverage} {
		if err := c.Validate(); err != nil {
			t.Errorf("declared %s: Validate() = %v, want nil", c, err)
		}
		if c.String() != string(c) {
			t.Errorf("%s: String() = %q", c, c.String())
		}
	}
	if err := capture.ClassUnspecified.Validate(); !errors.Is(err, capture.ErrFindingClassUnspecified) {
		t.Errorf("ClassUnspecified.Validate() = %v, want %v", err, capture.ErrFindingClassUnspecified)
	}
	if err := capture.FindingClass("urgent").Validate(); !errors.Is(err, capture.ErrFindingClassUnknown) {
		t.Errorf("Validate() = %v, want %v", err, capture.ErrFindingClassUnknown)
	}
}

// TestVerdictDeclaredSet is §32k's three per-(source, hour) judgements.
func TestVerdictDeclaredSet(t *testing.T) {
	declared := []capture.Verdict{
		capture.VerdictReplayable, capture.VerdictGapped, capture.VerdictAbsent,
	}
	for _, v := range declared {
		if err := v.Validate(); err != nil {
			t.Errorf("declared %s: Validate() = %v, want nil", v, err)
		}
		if v.String() != string(v) {
			t.Errorf("%s: String() = %q", v, v.String())
		}
	}
	if len(slices.Compact(slices.Clone(declared))) != len(declared) {
		t.Errorf("the declared verdicts are not distinct: %v", declared)
	}
	if err := capture.VerdictUnspecified.Validate(); !errors.Is(err, capture.ErrVerdictUnspecified) {
		t.Errorf("VerdictUnspecified.Validate() = %v, want %v", err, capture.ErrVerdictUnspecified)
	}
	if err := capture.Verdict("fine").Validate(); !errors.Is(err, capture.ErrVerdictUnknown) {
		t.Errorf("Validate() = %v, want %v", err, capture.ErrVerdictUnknown)
	}
}

// TestFindingStringNamesTheClassKindAndPlace covers the one line an operator
// actually reads.
//
// The offset is the part worth a test: -1 means "about the file as a whole" and
// must not print as `+-1`, which is the shape a bare Sprintf produces and which
// reads as a byte offset that happens to be impossible.
func TestFindingStringNamesTheClassKindAndPlace(t *testing.T) {
	withOffset := capture.Finding{
		Kind: capture.FindingChecksum, Path: "/a/b.arbcap", Offset: 4096, Detail: "bad crc",
	}
	if got, want := withOffset.String(),
		"integrity checksum /a/b.arbcap+4096: bad crc"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	whole := capture.Finding{
		Kind: capture.FindingNoClose, Path: "/a/b.arbcap", Offset: -1, Detail: "crashed",
	}
	if got, want := whole.String(), "coverage no_close /a/b.arbcap: crashed"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}
