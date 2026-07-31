// Package driving holds the strategy context's driving adapters. Slice A
// of step 4 lands one skeleton: the `arblab calibrate` command's refusal
// frame — the H1/H2/H3 disjointness guards (§41c), the §41h seal guard,
// and the §41e execution binding — with the derivation itself arriving in
// slices B and C.
//
// # Why a driving adapter
//
// The calibrate CLI DRIVES the context: it reads datasets and the archive,
// runs strategy/domain's own derivation math, and writes the context's
// artifact schema — §40d's fetch-writes-a-revision pattern one level up
// (derive-writes-an-artifact). §3.6 rule 3's letter puts it here:
// infrastructure/ contains only driven/ and driving/, so the adapter
// DIRECTION stays visible in the path.
//
// # Refusals, not reminders
//
// Every anti-tuning rule in §41c/§41e is a refusal the machinery executes,
// never a rule the operator remembers — the same default-deny direction
// nofloat and config:"hot" encode, applied to governance. The guards here
// are pure functions over declared constants, so the battery can exercise
// both directions of every boundary.
package driving

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/strategy/infrastructure/driven"
)

// Sentinels.
var (
	// ErrDerivationUnbuilt is the skeleton's honest door: the frame exists,
	// the derivation lands in slices B and C. The refusal names the state
	// rather than pretending an empty derivation is a derivation.
	ErrDerivationUnbuilt = fmt.Errorf("strategy/driving: calibration derivation not built yet (step 4 slices B/C)")

	// ErrHoldoutEra is rule H1's refusal (§41c): a historical input dated
	// on or after the declared boundary belongs to the holdout era,
	// reserved for scoring. The boundary is calendar arithmetic declared in
	// the 2026-07-31 design record — never a value chosen by inspecting
	// the data.
	ErrHoldoutEra = fmt.Errorf("strategy/driving: input on or after the holdout boundary (2026-02-01, §41c H1)")

	// ErrInstantUnset reports a zero instant handed to a guard. A zero
	// would read as "before every boundary" and pass — the forgotten
	// argument wearing a calibration-era date.
	ErrInstantUnset = fmt.Errorf("strategy/driving: instant unset")

	// ErrDayUnmanifested is rule H2's refusal (§41c): derivation reads only
	// archive days whose manifest entries exist when it runs. A day the
	// manifest cannot vouch for is a day the freeze cannot enumerate.
	ErrDayUnmanifested = fmt.Errorf("strategy/driving: archive day absent from the manifest")

	// ErrDatasetGap is one half of rule H3's completeness refusal (§41c):
	// a closed dataset identity is missing a row the venue serves inside
	// the declared range. Omitting adverse calibration rows must be a
	// refusal, not a quieter dataset.
	ErrDatasetGap = fmt.Errorf("strategy/driving: dataset missing a row inside its closed range")

	// ErrDatasetExtra is the other half: a row the grid never scheduled.
	// An inserted row is as much a tampered identity as a deleted one —
	// the "or extra" the review round added.
	ErrDatasetExtra = fmt.Errorf("strategy/driving: dataset carries a row outside its closed grid")

	// ErrDatasetOrder reports rows out of ascending order — a dataset
	// whose identity cannot be checked row-by-row.
	ErrDatasetOrder = fmt.Errorf("strategy/driving: dataset rows not strictly ascending")

	// ErrFetchBeforeSeal is §41h's ordering refusal: a holdout-era fetch
	// dated before the seal would mean the promotion identity was chosen
	// with holdout bytes in hand — the population-selection leak no
	// deterministic end rule repairs afterward.
	ErrFetchBeforeSeal = fmt.Errorf("strategy/driving: holdout-era fetch predates the §41h seal")

	// ErrIdentityUnsealed reports a holdout-era fetch for an identity the
	// seal does not name. The scored population is everything eligible
	// under the SEALED identity — a different venue's holdout data has no
	// consumer this side of a v2 artifact.
	ErrIdentityUnsealed = fmt.Errorf("strategy/driving: holdout-era fetch outside the sealed identity")

	// ErrTreeDirty is §41e's execution-binding refusal: a
	// modified-working-tree build can emit thresholds while naming the
	// last clean commit, which is rule-tuning's open door.
	ErrTreeDirty = fmt.Errorf("strategy/driving: derivation build from a dirty tree")

	// ErrRevisionUnavailable reports a build with no embedded VCS
	// revision — nothing to bind the recorded commit to.
	ErrRevisionUnavailable = fmt.Errorf("strategy/driving: no VCS revision embedded in the build")

	// ErrRevisionMismatch reports an embedded revision disagreeing with
	// the commit about to be recorded in the artifact.
	ErrRevisionMismatch = fmt.Errorf("strategy/driving: build revision disagrees with the recorded commit")

	// ErrArtifactExists is the no-overwrite refusal: versions are
	// append-only (§41d), so an existing artifact version is never
	// rewritten — a change is a v2.
	ErrArtifactExists = fmt.Errorf("strategy/driving: artifact version already exists")

	// ErrDatasetDigestMismatch reports a committed dataset file whose
	// bytes no longer hash to the digest the artifact records.
	ErrDatasetDigestMismatch = fmt.Errorf("strategy/driving: dataset bytes disagree with the recorded digest")
)

// H1Boundary is rule H1's frozen split instant (§41c, declared in the
// 2026-07-31 design record before any dataset was fetched): everything
// before is calibration era; everything on or after is holdout era,
// reserved for scoring.
var H1Boundary = time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)

// The §41h seal: the step-5 promotion identity, sealed by the user on
// 2026-07-31 — before any holdout-era fetch and before any derivation run,
// the earliest instant H3's both-conditions rule admits. A different fork
// or venue afterward is a v2 artifact plus a complete rerun.
const (
	SealFork  = driven.ForkHybrid
	SealVenue = ids.Venue("binance")
)

// SealedAt dates the seal.
var SealedAt = time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC)

// GuardCalibrationInput is rule H1 (§41c): refuse any historical input
// dated on or after the boundary. The boundary instant itself is holdout —
// the refusal names the record so the operator reads why, not just what.
func GuardCalibrationInput(at time.Time) error {
	if at.IsZero() {
		return fmt.Errorf("calibration input: %w", ErrInstantUnset)
	}
	if !at.Before(H1Boundary) {
		return fmt.Errorf("input at %s: %w", at.Format(time.RFC3339), ErrHoldoutEra)
	}
	return nil
}

// GuardHoldoutFetch is §41h's seal guard, for the step-5 fetchers that DO
// read holdout-era data: the fetch must postdate the seal (a fetch before
// it would mean the identity was chosen with holdout bytes in hand) and
// must be for the sealed identity (holdout data outside it has no lawful
// consumer).
func GuardHoldoutFetch(venue ids.Venue, fetchAt time.Time) error {
	if fetchAt.IsZero() {
		return fmt.Errorf("holdout fetch: %w", ErrInstantUnset)
	}
	if fetchAt.Before(SealedAt) {
		return fmt.Errorf("fetch at %s, sealed %s: %w",
			fetchAt.Format(time.RFC3339), SealedAt.Format(time.RFC3339), ErrFetchBeforeSeal)
	}
	if venue != SealVenue {
		return fmt.Errorf("fetch for %s, sealed venue %s: %w", venue, SealVenue, ErrIdentityUnsealed)
	}
	return nil
}

// GuardArchiveDay is rule H2 (§41c): derivation reads only archive days
// the manifest vouches for. The manifested predicate is the archive's —
// this guard owns only the refusal.
func GuardArchiveDay(day string, manifested func(day string) bool) error {
	if day == "" {
		return fmt.Errorf("archive day: %w", ErrInstantUnset)
	}
	if manifested == nil || !manifested(day) {
		return fmt.Errorf("archive day %s: %w", day, ErrDayUnmanifested)
	}
	return nil
}

// SettlementGrid is a closed dataset identity's expected row grid (§41c
// H3): every instant from Start to End inclusive, Interval apart. The
// completeness rule is mechanical so the tool can refuse a gap OR an
// extra by name.
type SettlementGrid struct {
	Start, End time.Time
	Interval   time.Duration
}

// Validate reports whether the grid describes a closed range.
func (g SettlementGrid) Validate() error {
	if g.Start.IsZero() || g.End.IsZero() {
		return fmt.Errorf("grid range: %w", ErrInstantUnset)
	}
	if g.Interval <= 0 {
		return fmt.Errorf("grid interval %v: %w", g.Interval, ErrInstantUnset)
	}
	if g.End.Before(g.Start) {
		return fmt.Errorf("grid end %s before start %s: %w",
			g.End.Format(time.RFC3339), g.Start.Format(time.RFC3339), ErrDatasetOrder)
	}
	return nil
}

// CheckComplete is rule H3's completeness refusal: rows must be exactly
// the grid — strictly ascending, no instant missing, none extra. The tool
// refuses a dataset with a missing OR extra row, naming the first
// offender; "no missing row" alone would let an inserted row thicken the
// sample unrefused.
func CheckComplete(g SettlementGrid, rows []time.Time) error {
	if err := g.Validate(); err != nil {
		return err
	}
	for n := 1; n < len(rows); n++ {
		if !rows[n-1].Before(rows[n]) {
			return fmt.Errorf("row %d %s after row %d %s: %w",
				n-1, rows[n-1].Format(time.RFC3339), n, rows[n].Format(time.RFC3339), ErrDatasetOrder)
		}
	}
	i := 0
	for expected := g.Start; !expected.After(g.End); expected = expected.Add(g.Interval) {
		if i >= len(rows) {
			return fmt.Errorf("missing %s: %w", expected.Format(time.RFC3339), ErrDatasetGap)
		}
		if rows[i].Before(expected) {
			return fmt.Errorf("row %s: %w", rows[i].Format(time.RFC3339), ErrDatasetExtra)
		}
		if rows[i].After(expected) {
			return fmt.Errorf("missing %s: %w", expected.Format(time.RFC3339), ErrDatasetGap)
		}
		i++
	}
	if i < len(rows) {
		return fmt.Errorf("row %s: %w", rows[i].Format(time.RFC3339), ErrDatasetExtra)
	}
	return nil
}

// CheckExecutionBinding is §41e's execution binding as a pure judgement:
// the build's embedded revision and dirty flag against the commit the
// artifact is about to record. Split from the build-info read so both
// directions of every refusal are testable.
func CheckExecutionBinding(revision string, dirty bool, recording string) error {
	if revision == "" {
		return ErrRevisionUnavailable
	}
	if dirty {
		return fmt.Errorf("revision %s: %w", revision, ErrTreeDirty)
	}
	if recording == "" {
		return fmt.Errorf("recorded commit: %w", ErrRevisionUnavailable)
	}
	if revision != recording {
		return fmt.Errorf("build %s, recording %s: %w", revision, recording, ErrRevisionMismatch)
	}
	return nil
}

// ReadBuildRevision reads the running binary's embedded VCS revision and
// dirty flag (the stdlib's build info — standard library only, the
// kernelpurity-compatible seam §41e names). A binary built outside VCS
// stamping reports ErrRevisionUnavailable through CheckExecutionBinding's
// empty-revision arm.
func ReadBuildRevision() (revision string, dirty bool, err error) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "", false, ErrRevisionUnavailable
	}
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if revision == "" {
		return "", false, ErrRevisionUnavailable
	}
	return revision, dirty, nil
}

// GuardNewVersion is §41d's append-only refusal: an artifact version is
// never overwritten. The exists predicate is the filesystem's; the guard
// owns the refusal.
func GuardNewVersion(path string, exists func(string) bool) error {
	if path == "" {
		return fmt.Errorf("artifact path: %w", ErrInstantUnset)
	}
	if exists != nil && exists(path) {
		return fmt.Errorf("artifact %s: %w", path, ErrArtifactExists)
	}
	return nil
}

// CheckDatasetDigest refuses a committed dataset whose bytes no longer
// hash to the recorded digest — the emit-time re-hash §41f names. The
// recorded text must already satisfy §41d's encoding.
func CheckDatasetDigest(recorded string, data []byte) error {
	if err := driven.CheckHashText(recorded); err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	if got := hex.EncodeToString(sum[:]); got != recorded {
		return fmt.Errorf("bytes hash %s, recorded %s: %w", got, recorded, ErrDatasetDigestMismatch)
	}
	return nil
}

// Calibrator is the `arblab calibrate` command's core. The frame above is
// real; the derivation is not — Run refuses by name until slices B and C
// land the fetchers, the derivation math and the freeze.
type Calibrator struct{}

// Run refuses: the derivation is unbuilt. The refusal is the same honesty
// as step 3's ErrCoreUnbuilt door — a skeleton that admits it is one,
// rather than an empty derivation emitting an empty artifact.
func (Calibrator) Run() error { return ErrDerivationUnbuilt }
