package capture_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/giiorg/arb-lab/internal/engine/capture"
)

// producerFor names the test that PRODUCES a finding of this kind.
//
// A kind with no producer is a declared member no code path can reach — which is
// exactly the defect this file exists because of. FindingAliveShort was declared,
// classified, documented and never emitted: verify computed the coverage
// shortfall inline to set the verdict, so the verdict came out right, the finding
// list said nothing about the only thing wrong with the window, and an exit code
// read off that list reported clean. Neither the compiler, nor exhaustiveswitch,
// nor coverage of Class() could see it, because every one of them was satisfied.
//
// A SWITCH rather than the map this replaced, and that is §71e. The map had
// drifted by five members — every manifest kind (§38a/§38c) was added to
// finding.go and to Class() and never here — and nothing could say so, because a
// test that ranges over a map cannot see the member that is not in it. The
// roster's own completeness was the one claim in this file that nothing checked.
// exhaustiveswitch (§3.6 rule 10) is deliberately in scope for a switch in a
// _test.go file over a PRODUCTION enum, so a kind added without a producing test
// now fails the gate at this function.
func producerFor(k capture.FindingKind) string {
	switch k {
	case capture.FindingChecksum:
		return "TestVerifyNamesTheFourFaults/one_corrupt_byte"
	case capture.FindingMalformed:
		return "TestVerifyRejectsRecordsThatArePresentAndWrong"
	case capture.FindingFileHeader:
		return "TestVerifyRejectsRecordsThatArePresentAndWrong"
	case capture.FindingDigestMismatch:
		return "TestVerifyChecksTheCloseRecordsOwnClaims"
	case capture.FindingDigestMissing:
		return "TestVerifyNamesTheFourFaults/a_missing_sidecar"
	case capture.FindingRecordCount:
		return "TestVerifyChecksTheCloseRecordsOwnClaims"
	case capture.FindingSeqNotAscending:
		return "TestVerifyReportsNonAscendingSequencesAsEvidenceOfTwoWriters"
	case capture.FindingSeqHole:
		return "TestVerifySeparatesExplainedFromUnexplainedHoles"
	case capture.FindingSeqHoleUnexplained:
		return "TestVerifySeparatesExplainedFromUnexplainedHoles"
	case capture.FindingCrossFileBreak:
		return "TestVerifyChecksTheCloseRecordsOwnClaims"
	case capture.FindingStrayFile:
		return "TestVerifyReportsAFileWhoseOpenRecordNamesAnotherStream"
	case capture.FindingWindowAbsent:
		return "TestVerifyNamesTheFourFaults/a_deleted_hour"
	case capture.FindingTornTail:
		return "TestVerifyNamesTheFourFaults/truncation"
	case capture.FindingNoClose:
		return "TestVerifyNamesTheFourFaults/truncation"
	case capture.FindingNoData:
		return "TestVerifyReportsAWindowWithNoVenueFrame"
	case capture.FindingAliveShort:
		return "TestVerifyReportsShortAliveCoverage"
	case capture.FindingGapRecord:
		return "TestVerifyReadsARestartTheWayTheWriterRecordsIt"
	case capture.FindingRestartBoundary:
		return "TestVerifyKeepsTheRestartNameWhenAnythingCouldHaveBeenLost"
	case capture.FindingUnsequencedBoundary:
		return "TestVerifyReadsAnOverlayRotationAsTheNonEventItIs"
	case capture.FindingContinuityUnknown:
		return "TestVerifyReportsAContinuityItCannotCheck"
	case capture.FindingManifestMissingFile:
		return "TestManifestPassNamesADeletedFile"
	case capture.FindingManifestMismatch:
		return "TestManifestPassComparesEveryFactItHolds"
	case capture.FindingManifestUnreadable:
		return "TestManifestPassReportsAManifestItCannotRead"
	case capture.FindingUnmanifested:
		return "TestManifestPassReportsAClosedFileWithNoEntry"
	case capture.FindingManifestAbsent:
		return "TestManifestPassReportsADayWithNoManifest"
	case capture.FindingUnspecified:
		// The zero value names no finding, so nothing produces it. Named rather
		// than defaulted, which is the whole point of the switch.
		return ""
	}
	return ""
}

// declaredFindingKinds is the set the assertions below iterate. It is a plain
// list because Go cannot enumerate a constant set — what keeps it honest is
// producerFor above, which the gate forces an author to visit for the same
// member.
var declaredFindingKinds = []capture.FindingKind{
	capture.FindingChecksum, capture.FindingMalformed, capture.FindingFileHeader,
	capture.FindingDigestMismatch, capture.FindingDigestMissing, capture.FindingRecordCount,
	capture.FindingSeqNotAscending, capture.FindingSeqHole, capture.FindingSeqHoleUnexplained,
	capture.FindingCrossFileBreak, capture.FindingStrayFile, capture.FindingWindowAbsent,
	capture.FindingTornTail, capture.FindingNoClose, capture.FindingNoData,
	capture.FindingAliveShort, capture.FindingGapRecord, capture.FindingRestartBoundary,
	capture.FindingUnsequencedBoundary, capture.FindingContinuityUnknown,
	capture.FindingManifestMissingFile, capture.FindingManifestMismatch,
	capture.FindingManifestUnreadable, capture.FindingUnmanifested, capture.FindingManifestAbsent,
}

// TestFindingKindDeclaredSet pins the roster, the classification of every member,
// and that the zero value belongs to neither exit code.
func TestFindingKindDeclaredSet(t *testing.T) {
	var integrity, coverage int
	for _, kind := range declaredFindingKinds {
		if err := kind.Validate(); err != nil {
			t.Errorf("declared %s: Validate() = %v, want nil", kind, err)
		}
		if producer := producerFor(kind); producer == "" {
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

// TestAffectsVerdictIsDecidedPerKind pins every kind's effect on its window's
// verdict (§71c), and both directions are populated on purpose: a method that
// answered false for everything would turn every window replayable while passing
// any test that checked only the new kind.
//
// The count check at the end is what makes the two lists a partition rather than
// two overlapping opinions — a kind decided in neither would otherwise inherit
// whatever the method's trailing return happens to be.
func TestAffectsVerdictIsDecidedPerKind(t *testing.T) {
	neutral := []capture.FindingKind{
		capture.FindingWindowAbsent,
		capture.FindingUnsequencedBoundary,
	}
	for _, k := range declaredFindingKinds {
		want := !slices.Contains(neutral, k)
		if got := k.AffectsVerdict(); got != want {
			t.Errorf("%s.AffectsVerdict() = %v, want %v", k, got, want)
		}
	}
	var flagging int
	for _, k := range declaredFindingKinds {
		if k.AffectsVerdict() {
			flagging++
		}
	}
	if flagging != len(declaredFindingKinds)-len(neutral) {
		t.Errorf("%d of %d kinds flag their window against %d neutral: the two "+
			"directions do not partition the roster",
			flagging, len(declaredFindingKinds), len(neutral))
	}
	// The zero value is not a declared kind and it flags: a finding minted with no
	// kind is a defect in the package, and a window that reads gapped is easier to
	// chase than one that reads clean.
	if !capture.FindingUnspecified.AffectsVerdict() {
		t.Error("an unset kind does not flag its window")
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
