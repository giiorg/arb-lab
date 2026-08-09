package formalfresh

import (
	"strings"
	"testing"
)

// The three digests are full 64-hex and pairwise different in their FIRST
// TWELVE characters, and both facts are load-bearing.
//
// Length first: `short()` truncates at 12, so a fixture digest of 12 or fewer
// characters makes "printed in full" and "printed truncated" the SAME string,
// and every assertion below would pass against the very defect #234 names. This
// is the collapse checklist's first dimension and it is the whole point of the
// file — a short fixture here is not a smaller test, it is a vacuous one.
//
// Prefixes second: if two digests shared a 12-character prefix, a message that
// printed the wrong one truncated would be indistinguishable from one that
// printed the right one, so "which digest" would go unchecked while "how long"
// looked covered.
const (
	stampedSHA = "1111111111112222222222223333333333334444444444445555555555556666"
	actualSHA  = "aaaaaaaaaaaabbbbbbbbbbbbccccccccccccddddddddddddeeeeeeeeeeeeffff"
	markerSHA  = "999999999999888888888888777777777777666666666666555555555555aaaa"
)

func init() {
	// A fixture that silently stopped satisfying its own premise would take the
	// assertions down with it without failing anything, so the premise is
	// checked rather than commented.
	for _, d := range []string{stampedSHA, actualSHA, markerSHA} {
		if len(d) != 64 {
			panic("fixture digest is not 64 chars — every assertion below would collapse")
		}
	}
	if stampedSHA[:12] == actualSHA[:12] || actualSHA[:12] == markerSHA[:12] || stampedSHA[:12] == markerSHA[:12] {
		panic("fixture digests share a 12-char prefix — 'which digest' would be unchecked")
	}
}

func fixtureModel() *Model {
	return &Model{
		Name:           "AmbiguousOrder",
		VerifiedAt:     "2026-08-08",
		VerifiedCommit: "310e854",
	}
}

func fixtureSubject() *Subject {
	return &Subject{
		Doc:     "build-spec.md",
		Section: "7.1",
		Role:    RolePremise,
		Why:     "the correspondence map cites the sizing rule",
		SHA256:  stampedSHA,
	}
}

// TestExplainPrintsTheDigestYouMustTranscribe is #234.
//
// Both of these verdicts end in an instruction to write a `sha256Actual` into
// docs/formal/freshness.json, and TestManifestSubjectsAreWellFormed refuses any
// marker whose sha256Actual is not exactly 64 characters. A message that
// truncates the one value it is asking for sends the reader off to recompute a
// digest the gate had in hand — so the full digest is part of the instruction,
// not decoration on it.
func TestExplainPrintsTheDigestYouMustTranscribe(t *testing.T) {
	tests := []struct {
		name    string
		verdict Verdict
		marker  *KnownStale
		// currentLine / stampedLine are the labels the two digests are printed
		// under. They are asserted POSITIONALLY rather than by mere presence,
		// because both slots hold a digest of identical width and character
		// class: swapping the two arguments would print the full digest under
		// the wrong label, and a `strings.Contains(text, full)` assertion cannot
		// see the difference. The message would then be telling the reader to
		// copy the digest they already have.
		currentLine string
		stampedLine string
		stampedSeen string
	}{
		{
			name:        "stale asks for a new marker",
			verdict:     Stale,
			currentLine: "actual:",
			stampedLine: "stamped:",
			stampedSeen: stampedSHA,
		},
		{
			name:    "marker-stale asks for the marker to be updated",
			verdict: MarkerStale,
			marker: &KnownStale{
				Task:   "#231",
				Note:   "core §6 step 7 names a retired producer",
				SHA256: markerSHA,
			},
			currentLine: "document is now:",
			stampedLine: "marker describes:",
			stampedSeen: markerSHA,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := fixtureSubject()
			s.KnownStale = tc.marker

			r := Explain(tc.verdict, fixtureModel(), s, actualSHA)

			if !r.Fails {
				t.Errorf("%v must fail the build; got Fails=false", tc.verdict)
			}

			current := lineWith(t, r.Text, tc.currentLine)
			if !strings.Contains(current, actualSHA) {
				t.Errorf("%v does not print the CURRENT digest in full under %q.\n"+
					"  the reader is told to record a 64-char sha256Actual and handed %d.\n"+
					"  line: %s\n  message:\n%s",
					tc.verdict, tc.currentLine, len(short(actualSHA)), current, r.Text)
			}

			stamped := lineWith(t, r.Text, tc.stampedLine)
			if strings.Contains(stamped, actualSHA) {
				t.Errorf("%v prints the current digest under %q — the two digests are swapped,\n"+
					"  so the value the reader must copy is sitting in the slot they must not:\n%s",
					tc.verdict, tc.stampedLine, r.Text)
			}
			if !strings.Contains(stamped, short(tc.stampedSeen)) {
				t.Errorf("%v: %q does not carry the digest it names:\n%s", tc.verdict, tc.stampedLine, r.Text)
			}

			// It must also SAY that this is the value to record, or a full digest
			// is just a longer number in a list of numbers.
			if !strings.Contains(r.Text, "sha256Actual") {
				t.Errorf("%v prints the digest but never names the field it goes in:\n%s", tc.verdict, r.Text)
			}
		})
	}
}

// lineWith returns the single line of text carrying the given label, failing if
// it is absent or ambiguous. A label matching two lines would make every
// positional assertion above depend on which one was found first.
func lineWith(t *testing.T, text, label string) string {
	t.Helper()
	var found []string
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, label) {
			found = append(found, line)
		}
	}
	switch len(found) {
	case 1:
		return found[0]
	case 0:
		t.Fatalf("no line labelled %q in:\n%s", label, text)
	default:
		t.Fatalf("%d lines labelled %q — the positional assertion is ambiguous:\n%s", len(found), label, text)
	}
	return ""
}

// TestExplainKeepsComparisonDigestsShort is the other half, and without it the
// first test is satisfied by printing every digest in full — which would make
// the message four 64-character lines and bury the one that matters.
//
// The asymmetry is the claim: a digest you must TRANSCRIBE is printed whole; a
// digest you only COMPARE stays short, because its full value is already in
// docs/formal/freshness.json where the reader is about to be editing anyway.
func TestExplainKeepsComparisonDigestsShort(t *testing.T) {
	s := fixtureSubject()
	r := Explain(Stale, fixtureModel(), s, actualSHA)

	if strings.Contains(r.Text, stampedSHA) {
		t.Errorf("the stamped digest is printed in full; it is only compared, never retyped:\n%s", r.Text)
	}
	if !strings.Contains(r.Text, stampedSHA[:12]) {
		t.Errorf("the stamped digest is not shown at all, so the reader cannot see WHAT changed:\n%s", r.Text)
	}

	s.KnownStale = &KnownStale{Task: "#231", Note: "n", SHA256: markerSHA}
	r = Explain(MarkerStale, fixtureModel(), s, actualSHA)
	if strings.Contains(r.Text, markerSHA) {
		t.Errorf("the marker's recorded digest is printed in full; it is only compared:\n%s", r.Text)
	}
	if !strings.Contains(r.Text, markerSHA[:12]) {
		t.Errorf("the marker's recorded digest is not shown at all:\n%s", r.Text)
	}
}

// TestExplainCoversEveryVerdict is the positive control. Without it a function
// that returned an empty non-failing Report for everything would satisfy the
// "stays short" test above and fail nothing.
func TestExplainCoversEveryVerdict(t *testing.T) {
	tests := []struct {
		verdict   Verdict
		marker    *KnownStale
		wantFails bool
		wantText  bool
		mentions  []string
	}{
		{
			verdict:   Fresh,
			wantFails: false,
			wantText:  false,
		},
		{
			verdict:   MarkedStale,
			marker:    &KnownStale{Task: "#231", Note: "owned by the retired-producer fix", SHA256: actualSHA},
			wantFails: false,
			wantText:  true,
			mentions:  []string{"#231", "owned by the retired-producer fix"},
		},
		{
			verdict:   MarkerObsolete,
			marker:    &KnownStale{Task: "#231", Note: "n", SHA256: markerSHA},
			wantFails: true,
			wantText:  true,
			mentions:  []string{"#231", "remove the marker"},
		},
		{
			verdict:   Stale,
			wantFails: true,
			wantText:  true,
			// The role and its reason travel with the failure: "re-run the model"
			// and "re-read the justification" are different work, and FM-7 is
			// what conflating them cost.
			mentions: []string{"AmbiguousOrder", "2026-08-08", "310e854", RolePremise, "the correspondence map cites the sizing rule", "formal-restamp MODEL=AmbiguousOrder"},
		},
		{
			verdict:   MarkerStale,
			marker:    &KnownStale{Task: "#231", Note: "n", SHA256: markerSHA},
			wantFails: true,
			wantText:  true,
			mentions:  []string{"#231"},
		},
	}

	if len(tests) != int(MarkerStale)+1 {
		t.Fatalf("this table covers %d verdicts and the type declares %d — a new member is untested",
			len(tests), int(MarkerStale)+1)
	}

	for _, tc := range tests {
		t.Run(tc.verdict.String(), func(t *testing.T) {
			s := fixtureSubject()
			s.KnownStale = tc.marker

			r := Explain(tc.verdict, fixtureModel(), s, actualSHA)

			if r.Fails != tc.wantFails {
				t.Errorf("Fails = %v, want %v", r.Fails, tc.wantFails)
			}
			if got := r.Text != ""; got != tc.wantText {
				t.Errorf("Text non-empty = %v, want %v (text: %q)", got, tc.wantText, r.Text)
			}
			if tc.wantText && !strings.Contains(r.Text, s.Label()) {
				t.Errorf("message does not name the section it is about:\n%s", r.Text)
			}
			for _, want := range tc.mentions {
				if !strings.Contains(r.Text, want) {
					t.Errorf("message omits %q:\n%s", want, r.Text)
				}
			}
		})
	}
}

func TestSubjectLabel(t *testing.T) {
	s := Subject{Doc: "build-spec.md", Section: "7.1"}
	if got, want := s.Label(), "build-spec.md §7.1"; got != want {
		t.Errorf("Label() = %q, want %q", got, want)
	}
}

func TestShortLeavesUndersizedDigestsAlone(t *testing.T) {
	// A malformed short digest must not be padded or panic — the well-formedness
	// test reports the malformation, and this one must still be able to print it.
	if got := short("abc"); got != "abc" {
		t.Errorf("short(%q) = %q, want it unchanged", "abc", got)
	}
	if got, want := short(actualSHA), actualSHA[:12]; got != want {
		t.Errorf("short() = %q, want %q", got, want)
	}
}
