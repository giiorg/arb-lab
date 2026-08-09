package formalfresh

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// repoRoot is where this package sits relative to the repository root.
const repoRoot = "../.."

// restampEnv names the model to restamp, or "all". It is a model NAME rather
// than a boolean on purpose: re-running `./run.sh` filtered to one model and
// then restamping every model would relabel the other model's staleness as
// verified without anything having verified it. The expensive-and-correct
// action and the cheap-and-wrong one must not share a spelling.
const restampEnv = "FORMALFRESH_RESTAMP"

// TestFormalModelsAreFresh is the gate.
//
// It does not run TLC. It asserts that the document sections each model was
// checked against are byte-identical to what they were at the recorded run —
// so that a normative edit which voids a model's coverage fails the build
// instead of waiting to be noticed.
func TestFormalModelsAreFresh(t *testing.T) {
	m, err := Load(repoRoot)
	if err != nil {
		t.Fatalf("loading the manifest: %v", err)
	}
	restamp := os.Getenv(restampEnv)
	if restamp != "" {
		t.Logf("RESTAMP MODE (%s=%q): digests will be rewritten and markers cleared.\n"+
			"This asserts the suite was RE-RUN against the current documents. "+
			"Restamping without re-running is how a staleness gate becomes a formality.",
			restampEnv, restamp)
	}

	// A model that verifies zero sections would pass this gate silently, and so
	// would a manifest with zero models. Both are the vacuous-run shape, so
	// both are refused before any comparison happens.
	if len(m.Models) == 0 {
		t.Fatal("manifest lists no models — the gate would pass vacuously")
	}

	changed := false
	for i := range m.Models {
		model := &m.Models[i]
		t.Run(model.Name, func(t *testing.T) {
			if len(model.Subjects) == 0 {
				t.Fatalf("%s tracks no sections — it would pass this gate vacuously", model.Name)
			}
			for j := range model.Subjects {
				s := &model.Subjects[j]
				label := fmt.Sprintf("%s §%s", s.Doc, s.Section)

				text, err := s.SectionText(repoRoot)
				if err != nil {
					// Renamed, removed or duplicated heading. Distinct from
					// staleness and reported as itself: the manifest names a
					// section the document no longer has.
					t.Errorf("%s: %v\n  the manifest names a section docs/%s no longer resolves; "+
						"fix the manifest or restore the heading", label, err, s.Doc)
					continue
				}
				actual := Digest(text)

				if restamp == "all" || restamp == model.Name {
					if actual != s.SHA256 || s.KnownStale != nil {
						t.Logf("restamped %s: %s -> %s", label, short(s.SHA256), short(actual))
						s.SHA256 = actual
						s.KnownStale = nil
						changed = true
					}
					continue
				}

				switch v := s.Check(actual); v {
				case Fresh:
					// The negative control for this gate, and it is a real one:
					// model (b)'s sections have been unchanged across dozens of
					// commits to the same file, so a green here is not the
					// gate declining to look.
				case MarkedStale:
					t.Logf("%s: marked stale, owned by %s — %s", label, s.KnownStale.Task, s.KnownStale.Note)
				case MarkerStale:
					t.Errorf("%s: changed AGAIN since %s recorded it.\n"+
						"  marker describes: %s\n  document is now:   %s\n"+
						"  the recorded delta is not the actual delta — re-read the change and update the marker",
						label, s.KnownStale.Task, short(s.KnownStale.SHA256), short(actual))
				case MarkerObsolete:
					t.Errorf("%s: carries a %s staleness marker but matches its stamp again.\n"+
						"  remove the marker — left in place it would suppress the next real change",
						label, s.KnownStale.Task)
				case Stale:
					t.Errorf("%s CHANGED since %s was verified (%s, %s), and nothing owns it.\n"+
						"  stamped: %s\n  actual:  %s\n"+
						"  role: %s — %s\n"+
						"  Either re-run `cd docs/formal && ./run.sh` and restamp with "+
						"`make formal-restamp MODEL=%s`, or record a knownStale marker naming the task that will.",
						label, model.Name, model.VerifiedAt, model.VerifiedCommit,
						short(s.SHA256), short(actual), s.Role, s.Why, model.Name)
				default:
					t.Errorf("%s: unhandled verdict %v", label, v)
				}
			}
		})
	}

	if changed {
		if err := m.Save(repoRoot); err != nil {
			t.Fatalf("saving the manifest: %v", err)
		}
		t.Logf("manifest rewritten: %s", ManifestPath)
	}
}

// TestEveryModelIsTracked closes the gate's own escape hatch: a model added to
// docs/formal and never added to the manifest is unpoliced, and nothing else
// would say so. A declared scope is a claim.
func TestEveryModelIsTracked(t *testing.T) {
	m, err := Load(repoRoot)
	if err != nil {
		t.Fatalf("loading the manifest: %v", err)
	}

	onDisk, err := Modules(repoRoot)
	if err != nil {
		t.Fatalf("reading %s: %v", FormalDir, err)
	}
	if len(onDisk) == 0 {
		t.Fatalf("no .tla modules found under %s — this check would pass vacuously", FormalDir)
	}

	owner := map[string]string{}
	for _, model := range m.Models {
		for _, mod := range model.Modules {
			if prev, dup := owner[mod]; dup {
				t.Errorf("%s is claimed by both %s and %s", mod, prev, model.Name)
			}
			owner[mod] = model.Name
			if _, err := os.Stat(filepath.Join(repoRoot, FormalDir, mod)); err != nil {
				t.Errorf("%s claims %s, which does not exist", model.Name, mod)
			}
		}
		if model.Companion == "" {
			t.Errorf("%s names no companion artifact", model.Name)
			continue
		}
		if _, err := os.Stat(filepath.Join(repoRoot, FormalDir, model.Companion)); err != nil {
			t.Errorf("%s names companion %s, which does not exist", model.Name, model.Companion)
		}
	}

	var untracked []string
	for _, mod := range onDisk {
		if owner[mod] == "" {
			untracked = append(untracked, mod)
		}
	}
	sort.Strings(untracked)
	if len(untracked) > 0 {
		t.Errorf("these TLA+ modules are in %s but no manifest entry claims them, "+
			"so no document change can ever be reported against them:\n  %s",
			FormalDir, strings.Join(untracked, "\n  "))
	}
}

// TestManifestSubjectsAreWellFormed checks the manifest's own shape. A subject
// with an empty role or an unrecognised one is a hand-edit that would still
// hash and compare correctly, so the type system never sees it.
func TestManifestSubjectsAreWellFormed(t *testing.T) {
	m, err := Load(repoRoot)
	if err != nil {
		t.Fatalf("loading the manifest: %v", err)
	}
	for _, model := range m.Models {
		if model.VerifiedAt == "" || model.VerifiedCommit == "" {
			t.Errorf("%s: verifiedAt/verifiedCommit must say which run these digests belong to", model.Name)
		}
		seen := map[string]bool{}
		for _, s := range model.Subjects {
			key := s.Doc + "§" + s.Section
			if seen[key] {
				t.Errorf("%s: %s listed twice", model.Name, key)
			}
			seen[key] = true

			switch s.Role {
			case RoleSubject, RolePremise:
			default:
				t.Errorf("%s: %s has role %q, want %q or %q", model.Name, key, s.Role, RoleSubject, RolePremise)
			}
			if s.Why == "" {
				t.Errorf("%s: %s records no reason for being tracked", model.Name, key)
			}
			if len(s.SHA256) != 64 {
				t.Errorf("%s: %s has a malformed digest %q", model.Name, key, s.SHA256)
			}
			if s.KnownStale != nil {
				if s.KnownStale.Task == "" {
					t.Errorf("%s: %s is marked stale but names no owning task — "+
						"an unowned marker is an exemption, not a marker", model.Name, key)
				}
				if len(s.KnownStale.SHA256) != 64 {
					t.Errorf("%s: %s marker has a malformed sha256Actual %q", model.Name, key, s.KnownStale.SHA256)
				}
			}
		}
	}
}

func short(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}
