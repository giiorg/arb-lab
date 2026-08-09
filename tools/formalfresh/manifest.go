package formalfresh

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ManifestPath is the manifest's location relative to the repository root.
const ManifestPath = "docs/formal/freshness.json"

// FormalDir is where the models live, relative to the repository root.
const FormalDir = "docs/formal"

// Manifest records, for every formal model, the document sections it was last
// verified against and the digest each of those sections had at that moment.
type Manifest struct {
	Comment []string `json:"comment"`
	Models  []Model  `json:"models"`
}

// Model is one formal model: its TLA+ modules, its companion artifact, and the
// verification run whose digests are stamped below.
type Model struct {
	Name string `json:"name"`
	// Modules lists every .tla file belonging to this model. The gate asserts
	// that every .tla in docs/formal appears in exactly one model's list, so a
	// model (c) added later cannot escape the gate by not being mentioned —
	// a declared scope is a claim, and this one is checked.
	Modules   []string `json:"modules"`
	Companion string   `json:"companion"`
	// VerifiedAt / VerifiedCommit identify the suite run these digests belong
	// to. They are prose to this program and evidence to a reader: the digests
	// say WHAT was checked, these say WHEN, and a restamp that moves the
	// digests without moving these is visible in the diff.
	VerifiedAt     string    `json:"verifiedAt"`
	VerifiedCommit string    `json:"verifiedCommit"`
	VerifiedNote   string    `json:"verifiedNote"`
	Subjects       []Subject `json:"subjects"`
}

// Role distinguishes what a section's change would invalidate.
const (
	// RoleSubject: the model encodes this section's machine. A change can
	// invalidate the runs themselves — re-run ./run.sh.
	RoleSubject = "subject"
	// RolePremise: the correspondence map cites this section to justify an
	// encoding or an omission. A change can invalidate a recorded
	// justification without touching a single run — re-read the row.
	// FM-7 is this repo's evidence for what a never-re-derived justification
	// costs, which is why premises are tracked and not only subjects.
	RolePremise = "premise"
)

// Subject is one tracked document section.
type Subject struct {
	Doc     string `json:"doc"`
	Section string `json:"section"`
	Role    string `json:"role"`
	Why     string `json:"why"`
	SHA256  string `json:"sha256"`

	// KnownStale marks a section that HAS changed since verification, with the
	// task that owns re-verifying it. It is the SPEC-EDIT-OWED pattern:
	// deviations are marked, not absorbed. (Spelled with hyphens deliberately:
	// the slice-close gate greps code for the marker's real spelling, and prose
	// ABOUT the marker is exactly what that grep must not match.)
	//
	// It is deliberately not an exemption. The marker pins the digest it was
	// stale AGAINST, so it covers exactly one known delta — the next edit to
	// the same section fails the gate again, and a marker whose section has
	// gone back to matching is itself an error, so the markers cannot rot.
	KnownStale *KnownStale `json:"knownStale,omitempty"`
}

// KnownStale names the owner of a recorded, un-discharged staleness.
type KnownStale struct {
	Task   string `json:"task"`
	Note   string `json:"note"`
	SHA256 string `json:"sha256Actual"`
}

// Load reads the manifest rooted at repoRoot.
func Load(repoRoot string) (*Manifest, error) {
	raw, err := os.ReadFile(filepath.Join(repoRoot, ManifestPath))
	if err != nil {
		return nil, err
	}
	var m Manifest
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	// An unknown field is a typo in a hand-edited manifest, and a typo'd
	// `knownStale` would silently become an unmarked staleness — refuse it
	// rather than ignore it.
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("%s: %w", ManifestPath, err)
	}
	return &m, nil
}

// Save writes the manifest back, formatted stably so a restamp produces a
// reviewable diff.
//
// HTML escaping is off: the `why` fields are prose full of `&` and quote the
// section titles verbatim, and & in a file whose entire purpose is to be
// read by a person during a review is noise.
func (m *Manifest) Save(repoRoot string) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(m); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(repoRoot, ManifestPath), buf.Bytes(), 0o644)
}

// Modules returns every .tla file in docs/formal, sorted.
func Modules(repoRoot string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(repoRoot, FormalDir))
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".tla") {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out, nil
}

// SectionText extracts one tracked section's text from the working tree.
func (s Subject) SectionText(repoRoot string) (string, error) {
	raw, err := os.ReadFile(filepath.Join(repoRoot, "docs", s.Doc))
	if err != nil {
		return "", err
	}
	text, err := ExtractSection(string(raw), s.Section)
	if err != nil {
		return "", fmt.Errorf("docs/%s §%s: %w", s.Doc, s.Section, err)
	}
	return text, nil
}

// Verdict is what the gate concluded about one tracked section.
type Verdict int

const (
	// Fresh: the section is byte-identical to what the model was checked
	// against.
	Fresh Verdict = iota
	// MarkedStale: the section changed, and a KnownStale marker names the task
	// that owns re-verification AND matches the current text.
	MarkedStale
	// Stale: the section changed with nothing owning it. This is the failure
	// the gate exists for.
	Stale
	// MarkerObsolete: a KnownStale marker survives on a section that now
	// matches its stamp. Left alone it would suppress a future real staleness,
	// so it is an error rather than a courtesy.
	MarkerObsolete
	// MarkerStale: a KnownStale marker exists but the section has changed
	// AGAIN since the marker was written, so the marker no longer describes
	// what is there. The recorded delta is not the actual delta.
	MarkerStale
)

func (v Verdict) String() string {
	switch v {
	case Fresh:
		return "fresh"
	case MarkedStale:
		return "marked-stale"
	case Stale:
		return "STALE"
	case MarkerObsolete:
		return "MARKER-OBSOLETE"
	case MarkerStale:
		return "MARKER-STALE"
	default:
		return fmt.Sprintf("Verdict(%d)", int(v))
	}
}

// Check classifies one subject against the current document text.
func (s Subject) Check(actual string) Verdict {
	switch {
	case actual == s.SHA256 && s.KnownStale == nil:
		return Fresh
	case actual == s.SHA256:
		return MarkerObsolete
	case s.KnownStale == nil:
		return Stale
	case s.KnownStale.SHA256 == actual:
		return MarkedStale
	default:
		return MarkerStale
	}
}
