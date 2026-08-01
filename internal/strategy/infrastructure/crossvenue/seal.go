package crossvenue

// The post-cutoff seal (ruling §45f-2), IN CODE: pre-cutoff data is the
// legitimate development corpus (already selection-contaminated, §43b);
// post-cutoff data is the gate's evidence, and the sim refuses to consume
// it before the procedure commit exists. The seal is structural, not
// remembered: archive enumeration is bounded by the books cutoff at
// LoadManifestDays, and settled REST rows at or past the settled cutoff
// are excluded HERE, visibly counted, never silently read into a score.
//
// The two instants are §43b's committed selection-cleanliness boundary —
// facts of the record, not tunable parameters, which is why they are
// constants with a citation rather than arguments slice D would pin.

import (
	"fmt"
	"time"

	dcv "github.com/giiorg/arb-lab/internal/strategy/domain/crossvenue"
)

// SettledCutoff is the instant selection-clean settled-funding evidence
// begins (§43b): the screening's fetch cutoff. A settlement AT this
// instant is gate evidence, so the pre-cutoff corpus is strictly before.
var SettledCutoff = time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)

// BooksCutoff is the instant selection-clean book evidence begins (§43b):
// the capture-expansion restart that first recorded the shortlist's books.
var BooksCutoff = time.Date(2026, 8, 1, 11, 13, 0, 0, time.UTC)

// ErrPostCutoffSealed reports an explicit attempt to point the exploration
// sim past the seal — a bound beyond the cutoff is a request to read gate
// evidence, refused by name until the procedure commit exists (§45f-2).
var ErrPostCutoffSealed = fmt.Errorf("strategy/crossvenue: post-cutoff evidence is sealed until the procedure commit (§45f-2)")

// sealSettled drops every settlement at or past SettledCutoff and reports
// how many were excluded. Dropping-and-counting rather than refusing is
// deliberate: the REST store legitimately accumulates post-cutoff rows for
// the gate's later run, and an exploration sim over a store that has begun
// accumulating must exclude them visibly, not become unrunnable.
func sealSettled(rows []dcv.Settlement) (kept []dcv.Settlement, excluded int) {
	for _, r := range rows {
		if r.At.Before(SettledCutoff) {
			kept = append(kept, r)
			continue
		}
		excluded++
	}
	return kept, excluded
}
