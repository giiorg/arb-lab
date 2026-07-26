package config

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// Revision is the identity of one loaded configuration.
//
// §5.2 requires that "revisions are recorded for deterministic replay (§3.5)",
// and §3.5's parity claim is that a backtest re-applies what the live run
// consumed. Configuration is an input to that: a run whose risk caps changed
// halfway through replays wrongly if the replay uses only the final file. So a
// Revision has to answer two different questions, and it carries a field for
// each.
//
//	WHICH document was this?     Hash, over the exact bytes. Content-addressed
//	                             and reproducible: the same file loaded on
//	                             another host a year later hashes identically,
//	                             which is what lets a replay assert it is
//	                             holding the configuration the run held.
//	WHEN in this run was it in    Num, monotonic within one Store. Dense and
//	force?                       ordered, so "the configuration in force at
//	                             revision 3" is answerable without timestamps.
//
// Neither substitutes for the other. Two Stores that load the same file both
// mint Num 1 over the same Hash; one Store reloading the same bytes twice would
// mint the same Hash at two Nums — except that Reload treats byte-identical
// input as a no-op, so it does not.
//
// Source and LoadedAt are provenance. Source names the document for an operator
// reading an audit trail; LoadedAt is supplied by the caller from a Clock rather
// than read here, so a replay controls it.
type Revision struct {
	// Num is the position of this revision in its Store's history, counting
	// from 1. Zero means unset, which is why Validate rejects it.
	Num uint64

	// Hash is sha256 over the exact bytes loaded — not over the decoded value,
	// so a reformatted file is a different revision even when it decodes
	// identically. That is the honest direction: an operator did edit the file,
	// and an audit trail that hides the edit because the semantics happened not
	// to change is an audit trail that decides what matters on the operator's
	// behalf.
	//
	// The zero array is a reliable "unset" sentinel: sha256 of the empty input
	// is e3b0c442…, not zero, so no real document can produce it.
	Hash [32]byte

	// Source names where the bytes came from — a path, a URL, a well-known
	// name. It is provenance for humans and for replay, never something this
	// package reads from.
	Source string

	// LoadedAt is the instant the configuration took effect, supplied by the
	// caller from a Clock. See the package doc on why it is an argument rather
	// than a time.Now() call.
	LoadedAt time.Time
}

// Validate reports whether the revision identifies anything.
//
// The zero Revision fails on all four counts, which is deliberate: a zero
// revision travelling into a journal would record "configuration unknown, at the
// epoch, from nowhere" as though it were a fact.
func (r Revision) Validate() error {
	if r.Num == 0 {
		return fmt.Errorf("%w: revision number is zero", ErrRevisionInvalid)
	}
	if r.Hash == ([32]byte{}) {
		return fmt.Errorf("%w: content hash unset", ErrRevisionInvalid)
	}
	if r.Source == "" {
		return fmt.Errorf("%w: %w", ErrRevisionInvalid, ErrSourceUnset)
	}
	if r.LoadedAt.IsZero() {
		return fmt.Errorf("%w: %w", ErrRevisionInvalid, ErrInstantUnset)
	}
	return nil
}

// HashString is the content hash as lowercase hex, the form that goes into a
// journal entry or an audit line.
func (r Revision) HashString() string { return hex.EncodeToString(r.Hash[:]) }

// String renders the revision for a log line: the number, an abbreviated hash,
// and the source. The abbreviation is for reading; HashString is for recording.
func (r Revision) String() string {
	return fmt.Sprintf("rev %d sha256:%s %s", r.Num, r.HashString()[:12], r.Source)
}

// hashOf is the content hash of a configuration document. Split out because two
// callers need it to agree exactly — Reload compares the incoming hash against
// the current revision's to detect a re-read of unchanged bytes, and that
// comparison is only sound if both sides hashed the same way.
func hashOf(data []byte) [32]byte { return sha256.Sum256(data) }
