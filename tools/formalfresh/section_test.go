package formalfresh

import (
	"errors"
	"strings"
	"testing"
)

// doc builds a line-oriented fixture. Fixtures are written as line slices
// rather than raw strings because a fenced-code fixture has to contain
// backticks, and because line boundaries are the thing under test.
func doc(lines ...string) string { return strings.Join(lines, "\n") }

// The structural fixture is built to break FOUR distinctions at once, because
// each of them collapses under the fixture a person writes by default:
//
//   - 7.1 vs 7.10 — a prefix match passes unless a longer sibling exists.
//   - 7.1 vs 7    — a dot-stripping bug passes unless parent and child both
//     appear and are both asked for.
//   - 7.1 vs 7.1.1 — "ends at the next heading" and "ends at the next heading
//     of the same or shallower level" are the same rule unless a DEEPER
//     heading sits inside a section.
//   - 7.2 vs 8    — and they are the same rule again unless a section is
//     closed by a SHALLOWER heading somewhere too.
//
// A fixture with only `7.1` and `7.2` in it would pass against every one of
// those bugs.
var structural = doc(
	"# Build spec",
	"",
	"Preamble.",
	"",
	"## 7. Risk",
	"",
	"Body of seven.",
	"",
	"### 7.1 Sizing",
	"",
	"Body of seven-one.",
	"",
	"#### 7.1.1 Nested",
	"",
	"Nested body.",
	"",
	"### 7.10 Tenth",
	"",
	"Body of seven-ten.",
	"",
	"### 7.2 Second",
	"",
	"Body of seven-two.",
	"",
	"## 8. Next",
	"",
	"Body of eight.",
)

func TestExtractSectionBoundaries(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		contains []string
		excludes []string
	}{
		{
			name: "child section keeps its own deeper headings",
			id:   "7.1",
			// The nested #### is PART of 7.1 and must be hashed with it.
			contains: []string{"### 7.1 Sizing", "Body of seven-one.", "#### 7.1.1 Nested", "Nested body."},
			// A prefix match would swallow 7.10; a shallow-end bug would stop
			// before the nested heading and silently hash less.
			excludes: []string{"7.10", "Body of seven-ten.", "### 7.2", "## 8."},
		},
		{
			name:     "longer sibling is a different section",
			id:       "7.10",
			contains: []string{"### 7.10 Tenth", "Body of seven-ten."},
			excludes: []string{"Body of seven-one.", "Body of seven-two."},
		},
		{
			name:     "parent section spans its children",
			id:       "7",
			contains: []string{"## 7. Risk", "Body of seven.", "Body of seven-one.", "Body of seven-two."},
			// Closed by the SHALLOWER heading, not by any of its own children.
			excludes: []string{"## 8. Next", "Body of eight."},
		},
		{
			name:     "section closed by a shallower heading",
			id:       "7.2",
			contains: []string{"### 7.2 Second", "Body of seven-two."},
			excludes: []string{"## 8.", "Body of eight.", "Body of seven-ten."},
		},
		{
			name:     "final section runs to end of file",
			id:       "8",
			contains: []string{"## 8. Next", "Body of eight."},
			excludes: []string{"Body of seven."},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ExtractSection(structural, tc.id)
			if err != nil {
				t.Fatalf("ExtractSection(%q) = error %v, want success", tc.id, err)
			}
			// Positive control on the extraction itself: an extractor returning
			// "" would satisfy every `excludes` assertion below and none of
			// these, which is the whole reason both directions are asserted.
			if !strings.HasPrefix(got, "#") {
				t.Errorf("extract %q: does not start with its heading:\n%s", tc.id, got)
			}
			for _, want := range tc.contains {
				if !strings.Contains(got, want) {
					t.Errorf("extract %q: missing %q\n--- got ---\n%s", tc.id, want, got)
				}
			}
			for _, bad := range tc.excludes {
				if strings.Contains(got, bad) {
					t.Errorf("extract %q: leaked %q\n--- got ---\n%s", tc.id, bad, got)
				}
			}
		})
	}
}

// Trailing blank lines are trimmed, so the digest does not move when somebody
// adds a blank line before the next heading. Asserted rather than assumed,
// because it is the one normalisation the gate performs.
func TestExtractSectionTrimsTrailingBlanks(t *testing.T) {
	a, err := ExtractSection(doc("## 1. A", "", "Body.", "", "## 2. B", "", "Other."), "1")
	if err != nil {
		t.Fatal(err)
	}
	b, err := ExtractSection(doc("## 1. A", "", "Body.", "", "", "", "## 2. B", "", "Other."), "1")
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Errorf("trailing blank lines changed the section text:\n%q\nvs\n%q", a, b)
	}
	if Digest(a) != Digest(b) {
		t.Error("trailing blank lines changed the digest")
	}
}

// Neither spec document contains a heading-like line inside a code fence
// TODAY, so this branch is unreachable from the real corpus and would ship
// untested without a synthetic fixture. Without fence tracking, asking for "4"
// below returns ErrAmbiguous and asking for "3" returns a truncated section —
// two different wrong answers from one bug.
func TestExtractSectionIgnoresFencedHeadings(t *testing.T) {
	fenced := doc(
		"## 3. Configuration",
		"",
		"Intro.",
		"",
		"```sh",
		"# not a heading",
		"## 4. also not a heading",
		"```",
		"",
		"Tail after the fence.",
		"",
		"## 4. Real four",
		"",
		"Four body.",
	)

	three, err := ExtractSection(fenced, "3")
	if err != nil {
		t.Fatalf("§3: %v", err)
	}
	if !strings.Contains(three, "# not a heading") {
		t.Error("§3: fenced content was dropped — the fence ended the section")
	}
	if !strings.Contains(three, "Tail after the fence.") {
		t.Error("§3: section truncated at the fenced heading")
	}
	if strings.Contains(three, "Four body.") {
		t.Error("§3: ran past the real §4 heading")
	}

	four, err := ExtractSection(fenced, "4")
	if err != nil {
		t.Fatalf("§4: %v — a fenced `## 4.` was counted as a real heading", err)
	}
	if !strings.Contains(four, "Four body.") {
		t.Errorf("§4: wrong section extracted:\n%s", four)
	}
}

// state-machines.md writes `## §3.` and build-spec.md writes `## 7.`; a
// citation names both without the marker. Both must parse to the cited id.
func TestExtractSectionAcceptsBothHeadingConventions(t *testing.T) {
	sm := doc("## §2. Classifier", "", "SM body.", "", "## §3. Arbitration", "", "Other.")
	got, err := ExtractSection(sm, "2")
	if err != nil {
		t.Fatalf("§ convention: %v", err)
	}
	if !strings.Contains(got, "SM body.") || strings.Contains(got, "Other.") {
		t.Errorf("§ convention: wrong span:\n%s", got)
	}
}

func TestExtractSectionErrors(t *testing.T) {
	tests := []struct {
		name string
		src  string
		id   string
		want error
	}{
		{
			name: "missing heading is an error, not an empty section",
			src:  structural,
			id:   "9",
			want: ErrNotFound,
		},
		{
			name: "duplicated heading is refused rather than guessed",
			src:  doc("## 5. One", "", "First.", "", "## 5. Two", "", "Second."),
			id:   "5",
			want: ErrAmbiguous,
		},
		{
			name: "heading with no body is an error",
			src:  doc("## 5. Empty", "", "", "## 6. Next", "", "Body."),
			id:   "5",
			want: ErrEmpty,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ExtractSection(tc.src, tc.id)
			if !errors.Is(err, tc.want) {
				t.Errorf("got %v, want %v", err, tc.want)
			}
		})
	}
}

func TestSectionID(t *testing.T) {
	tests := []struct {
		rest string
		want string
	}{
		{"§3. FenceReadyLost arbitration", "3"},
		{"7.1 Sizing for survival", "7.1"},
		{"12. MVP milestones", "12"},
		{"3.0 Resolved architecture decisions", "3.0"},
		{"7.10 A tenth subsection", "7.10"},
		// Non-numbered headings are not sections. Without this the appendix and
		// the document title would be tracked as sections named "Appendix" and
		// "Funding-Rate", and ExtractSection would happily hash them.
		{"Appendix — traceability & history", ""},
		{"Funding-Rate Arbitrage Platform", ""},
		{"", ""},
		// A bare separator has no number in it.
		{". Nothing", ""},
		{"7..1 Malformed", ""},
	}
	for _, tc := range tests {
		if got := sectionID(tc.rest); got != tc.want {
			t.Errorf("sectionID(%q) = %q, want %q", tc.rest, got, tc.want)
		}
	}
}

func TestHeading(t *testing.T) {
	tests := []struct {
		line      string
		wantLevel int
	}{
		{"## 7. Risk", 2},
		{"#### 7.1.1 Nested", 4},
		{"###### 1. Deepest", 6},
		// Seven hashes is not a heading in CommonMark, and a space is required
		// after the run — `#hashtag` is prose.
		{"####### 1. Too deep", 0},
		{"#hashtag", 0},
		{"Not a heading", 0},
		{"", 0},
		{"#", 0},
	}
	for _, tc := range tests {
		if got, _ := heading(tc.line); got != tc.wantLevel {
			t.Errorf("heading(%q) level = %d, want %d", tc.line, got, tc.wantLevel)
		}
	}
}

// The marker is an owned, pinned delta — not an exemption. Each row below is a
// distinct verdict the gate must be able to reach; collapsing any two of them
// would let a real staleness pass as a recorded one.
func TestSubjectCheck(t *testing.T) {
	const stamped, changed, changedAgain = "aaa", "bbb", "ccc"

	tests := []struct {
		name   string
		marker *KnownStale
		actual string
		want   Verdict
	}{
		{"unchanged and unmarked", nil, stamped, Fresh},
		{"changed with nothing owning it", nil, changed, Stale},
		{"changed, owned, and the marker matches", &KnownStale{SHA256: changed}, changed, MarkedStale},
		// The marker covers ONE delta. A second edit to the same section is a
		// new fact the recorded note does not describe.
		{"changed again past the marker", &KnownStale{SHA256: changed}, changedAgain, MarkerStale},
		// A marker on a section that matches again would sit there suppressing
		// the next real staleness, so it has to be an error.
		{"marker outlived its staleness", &KnownStale{SHA256: changed}, stamped, MarkerObsolete},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := Subject{SHA256: stamped, KnownStale: tc.marker}
			if got := s.Check(tc.actual); got != tc.want {
				t.Errorf("Check = %v, want %v", got, tc.want)
			}
		})
	}
}
