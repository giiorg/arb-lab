package formalfresh

import "fmt"

// Report is what the gate says about one tracked section: the text a person
// reads, and whether it fails the build.
//
// It exists so the messages can be asserted on. They used to be format strings
// inside the gate's t.Errorf calls, which meant no test could read them and
// tools/mutate.sh had no line to aim at — and #234 is what that cost: the
// message telling a reader to record a 64-character sha256Actual printed a
// 12-character digest, and every run of the gate was green about it.
type Report struct {
	Text  string
	Fails bool
}

// Label names a tracked section the way every message spells it.
func (s Subject) Label() string { return fmt.Sprintf("%s §%s", s.Doc, s.Section) }

// Explain renders one verdict as the instruction it implies.
//
// On digest length there is a deliberate asymmetry, and it is the rule rather
// than an inconsistency: a digest the reader must TRANSCRIBE into
// docs/formal/freshness.json is printed in full, and a digest the reader only
// COMPARES stays truncated. The comparison digests are already in the manifest
// at full length, which is the file being edited anyway; printing all of them
// whole would put four 64-character lines in the message and bury the one that
// has to be copied.
//
// The restamp path in the gate is the third case and needs neither: it rewrites
// the manifest itself, so nothing is transcribed there and its log line stays
// short on both sides.
func Explain(v Verdict, model *Model, s *Subject, actual string) Report {
	label := s.Label()

	switch v {
	case Fresh:
		// Silent, and that silence is load-bearing rather than an omission:
		// model (b)'s sections have been unchanged across dozens of commits to
		// the same file, so a green here is the gate agreeing, not declining to
		// look.
		return Report{}

	case MarkedStale:
		return Report{Text: fmt.Sprintf("%s: marked stale, owned by %s — %s",
			label, s.KnownStale.Task, s.KnownStale.Note)}

	case MarkerObsolete:
		return Report{
			Fails: true,
			Text: fmt.Sprintf("%s: carries a %s staleness marker but matches its stamp again.\n"+
				"  remove the marker — left in place it would suppress the next real change",
				label, s.KnownStale.Task),
		}

	case MarkerStale:
		return Report{
			Fails: true,
			Text: fmt.Sprintf("%s: changed AGAIN since %s recorded it.\n"+
				"  marker describes: %s\n"+
				"  document is now:  %s\n"+
				"  the recorded delta is not the actual delta — re-read the change, then set this\n"+
				"  marker's sha256Actual in %s to the full digest above",
				label, s.KnownStale.Task, short(s.KnownStale.SHA256), actual, ManifestPath),
		}

	case Stale:
		return Report{
			Fails: true,
			Text: fmt.Sprintf("%s CHANGED since %s was verified (%s, %s), and nothing owns it.\n"+
				"  stamped: %s\n"+
				"  actual:  %s\n"+
				"  role: %s — %s\n"+
				"  Either re-run `cd docs/formal && ./run.sh` and restamp with\n"+
				"  `make formal-restamp MODEL=%s`, or record a knownStale marker in %s naming\n"+
				"  the task that will — its sha256Actual is the full `actual` digest above",
				label, model.Name, model.VerifiedAt, model.VerifiedCommit,
				short(s.SHA256), actual, s.Role, s.Why, model.Name, ManifestPath),
		}

	default:
		return Report{Fails: true, Text: fmt.Sprintf("%s: unhandled verdict %v", label, v)}
	}
}

// short truncates a digest to the length at which two of them can still be told
// apart by eye. It is only ever applied to a digest the reader compares — see
// Explain's note on the asymmetry — and it leaves an undersized string alone so
// a malformed manifest entry can still be printed by the check that reports it.
func short(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}
