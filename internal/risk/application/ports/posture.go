// Package ports declares what the risk context needs from the outside world,
// in the risk context's own vocabulary.
//
// §4's port table assigns `AccountPostureSource` to *"risk — the safety-posture
// guard (§4.1)"*. This is that declaration. It speaks `risk/domain`'s types and
// names no venue, no endpoint and no wire shape: the ACL in
// `risk/infrastructure/driven` is where `engine/venue`'s answer becomes one of
// these, and §3.6 rule 1 makes that the only place it can be.
package ports

import (
	"errors"
	"fmt"

	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/risk/domain"
)

// ErrPostureScopeSettingsUnstated reports a posture scope that names no setting
// to establish.
//
// The package's first sentinel, and it is here rather than in domain because
// what it refuses is a malformed REQUEST — a caller that did not say what it
// needs read — rather than anything about an account. See PostureScope.Settings
// for why the empty list cannot be given a meaning.
var ErrPostureScopeSettingsUnstated = errors.New("risk: posture scope names no setting to establish")

// AccountPostureSource reads one venue account's whole §4.1 posture.
//
// # The read is WHOLE or it is nothing
//
// §4.1's premise is a conjunction, so there is no partial answer here. An
// implementation that performed four of five venue calls has established
// nothing about the fifth, and returning what it did learn invites the guard to
// fold over a short list and find no failures in it — which is the vacuous pass
// this context is built to refuse. An error means the posture was NOT read.
//
// # It is never an empty report
//
// §5.3's rule about not inferring an absence from an unanswered query, one
// context over: an empty report reads as an account that answered nothing,
// which the rules turn into named refusals — a very different statement from
// "the read failed", and the two must not be confusable.
//
// # There is no caching seam here, deliberately
//
// §6.3 requires the posture *"fresh within a max-age TTL and re-verified by the
// pre-trade REST read"*, and record §73c-2 reads that conservatively: the TTL
// bounds any cached snapshot and is never a way to skip the read. Position-mode
// changes emit NO venue event on either venue, so a flip inside a TTL window is
// exactly what the re-verify exists to catch. A `Cached()` or `MaxAge` method
// here would be the seam through which the rejected reading returns.
type AccountPostureSource interface {
	// AccountPosture reads the posture for one scope.
	AccountPosture(PostureScope) (domain.Report, error)
}

// PostureScope is which venue's account to read.
//
// It carries the venue and nothing else: the account is the credential's (the
// venue answers for whatever subaccount the signature names), the symbol roster
// belongs to the REQUIREMENT rather than to the read — a read scoped by the
// symbols this system already names cannot surface a configured symbol the
// venue does not know — and there is no freshness dimension, per the type
// above.
type PostureScope struct {
	Venue ids.Venue

	// Settings is every setting this read must establish.
	//
	// # It narrows WHAT IS ASKED, and never how completely it is answered
	//
	// The type above refuses a partial answer, and this field is not a way back
	// to one: it is decided before any call is issued, from the rule table's
	// statement of which settings govern this deployment at all, and every
	// setting in it is established or the read fails (task #199, record §73p).
	// The conjunction is unchanged; what changed is what it ranges over.
	//
	// It exists because a setting that governs a leg this deployment does not
	// trade would otherwise be read from an endpoint the deployment has no
	// reason to route — turning §4.1's gate unanswerable for a shape §5.2
	// actively builds toward (record §73n F20).
	//
	// Empty is REFUSED rather than meaning "everything". The absent list would
	// have to mean one or the other, and "everything" is the reading that lets a
	// caller who forgot to build one read a surface it cannot route; "nothing"
	// is the reading that returns a clean posture nobody checked. Both are worse
	// than a refusal that names the field.
	Settings []domain.Setting
}

// Validate reports whether the scope names a read a venue could serve.
func (s PostureScope) Validate() error {
	if err := s.Venue.Validate(); err != nil {
		return fmt.Errorf("posture scope venue: %w", err)
	}
	if len(s.Settings) == 0 {
		return fmt.Errorf("posture scope for %s: %w", s.Venue, ErrPostureScopeSettingsUnstated)
	}
	for _, set := range s.Settings {
		if err := set.Validate(); err != nil {
			return fmt.Errorf("posture scope for %s: %w", s.Venue, err)
		}
	}
	return nil
}

func (s PostureScope) String() string { return fmt.Sprintf("%s/account-posture", s.Venue) }
