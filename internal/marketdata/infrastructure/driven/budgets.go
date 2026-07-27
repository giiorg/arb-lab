package driven

import (
	"fmt"
	"maps"
	"slices"

	"github.com/giiorg/arb-lab/internal/marketdata/domain"
	"github.com/giiorg/arb-lab/internal/platform/config"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// FreshnessConfig is the §5.2 configuration section this context owns.
//
// # It lives here rather than in platform/config, and that is the kernel's rule
//
// platform/config owns the MECHANISM — strict decode, Validate, the per-field
// hot-reload policy, the revision identity §3.5 replays against — and
// deliberately owns no schema. A context's parameters belong to the context
// whose meaning they carry. So this is marketdata's section, in marketdata's
// tree, and the practical test of the split is that adding a venue's budget
// touches this file and nothing under platform/.
//
// # Why the section is a map and not a list of venues
//
// A venue is a key, not an enum-like (see ids.Venue): the set is open and comes
// from configuration. A list would admit two entries for one venue with nothing
// to say which wins, and §5.1's one-spelling rule is the same rule — two
// spellings of an identity are two answers to a question that has one.
type FreshnessConfig struct {
	// Funding is the per-venue budget for funding-rate data.
	//
	// # Hot, and the tag is load-bearing rather than a convenience
	//
	// platform/config denies reload per field by default, and a key-set change
	// counts as STRUCTURAL — so without this tag, onboarding a venue or
	// widening a budget during a degrading feed would need a process restart,
	// and nothing in the code would say so. §3.4's budgets are exactly what an
	// operator reaches for mid-incident; a restart to apply one is a restart
	// taken while a position is open.
	//
	// The tag makes the whole subtree hot, which is what lets both a changed
	// bound and an added venue land.
	//
	// # It is scoped to FUNDING on purpose
	//
	// §3.4 is explicit that freshness is per-data-class and not one threshold:
	// a book used for execution needs a sub-second budget while funding
	// metadata tolerates longer. A single map named "freshness" would invite
	// one number for both, and the number that is right for funding is
	// catastrophic for a book. The order-book path is deferred for this slice
	// (the user's call), and when it lands it gets its own field here rather
	// than sharing this one.
	Funding map[ids.Venue]BudgetSpec `json:"funding" config:"hot"`
}

// BudgetSpec is one venue's funding budget as it is written in a document.
//
// The two bounds are config.Duration rather than time.Duration, and that is the
// difference between "5m" and 300 nanoseconds. A JSON number decoded into a
// time.Duration is nanoseconds, so a T_blind of 300 meaning five minutes gives
// a blind budget that has already expired the first time it is checked — the
// §7.1 de-risk firing on the first tick, with nothing logged and nothing
// failing. config.Duration refuses the number outright.
type BudgetSpec struct {
	// MaxAge is the age past which an observation is FreshnessStale.
	MaxAge config.Duration `json:"max_age"`

	// Blind is the age past which it is FreshnessBlind, firing §5.2's
	// mark_blind_action.
	Blind config.Duration `json:"blind"`
}

// Budget is the domain value this spec describes.
//
// A translation, not a definition: what a valid budget IS belongs to the
// domain, and this type exists only because a domain value should not have to
// carry a wire encoding. Keeping the two apart is what stops a JSON tag from
// becoming a domain concern the day the config format changes.
func (s BudgetSpec) Budget() domain.Budget {
	return domain.Budget{MaxAge: s.MaxAge.Duration(), Blind: s.Blind.Duration()}
}

// Validate defers to the domain rather than restating its rule.
//
// domain.Budget.Validate is where "MaxAge must be strictly below Blind" lives,
// and it refuses a configuration in which the FreshnessStale rung is
// unreachable — the warning that exists to precede the automated de-lever never
// firing, so the first symptom an operator sees IS the de-lever. Restating that
// comparison here would be a second copy of a boundary rule, which is the
// defect §7.6 already paid for once.
func (s BudgetSpec) Validate() error {
	return s.Budget().Validate()
}

// Validate refuses a section that would leave the funding path unable to work.
//
// # The errors name the venue, and they name the SAME one on every run
//
// Go randomises map iteration, so a validator that reported the first bad entry
// it happened to reach would name a different venue on different runs — two
// boot logs produced from one bad file, disagreeing about what is wrong with
// it. Sorting the keys costs one allocation at boot and buys a refusal an
// operator can compare against yesterday's.
func (c FreshnessConfig) Validate() error {
	if len(c.Funding) == 0 {
		return ErrNoVenues
	}
	for _, v := range slices.Sorted(maps.Keys(c.Funding)) {
		if err := v.Validate(); err != nil {
			return fmt.Errorf("funding freshness: venue key %q: %w", string(v), err)
		}
		if err := c.Funding[v].Validate(); err != nil {
			return fmt.Errorf("funding freshness: venue %s: %w", v, err)
		}
	}
	return nil
}

// ConfigBudgets answers application.Budgets from a live configuration store.
//
// # It re-reads on every call, and that is the point of the port existing
//
// application/ports.go states it: reading budgets through a port at decision
// time is what lets a hot-reloaded budget take effect without the use case
// holding a snapshot of config taken at construction. Caching here would
// reintroduce exactly that — a service that keeps applying the budget that was
// in force when the process booted, while an operator watches the file they
// edited have no effect.
//
// The read is cheap by construction: config.Store.Current is an atomic pointer
// load, which is what makes "read it every time" an honest default rather than
// a cost to be optimised away.
type ConfigBudgets struct {
	store *config.Store[FreshnessConfig]
}

// NewConfigBudgets binds the adapter to a store, refusing a nil one.
func NewConfigBudgets(store *config.Store[FreshnessConfig]) (*ConfigBudgets, error) {
	if store == nil {
		return nil, ErrStoreNil
	}
	return &ConfigBudgets{store: store}, nil
}

// FundingBudget reports the configured budget for a venue.
//
// # The returned Budget is a copy, which is what keeps config's one sharp edge
// # out of this context
//
// config.Store.Current returns a copy of the snapshot STRUCT and explicitly
// does not deep-copy: a map field's backing store is shared with every other
// reader. Returning the domain value built from an entry — rather than a
// pointer into that map, or the map itself — means nothing a caller holds can
// reach the shared structure. Treating loaded configuration as immutable is the
// kernel's stated contract; this is the adapter not relying on every caller
// having read it.
//
// A missing venue is (zero, false) rather than an error, per the port's
// contract, and the zero Budget is unusable — domain.Budget.Validate refuses it
// — so a caller that ignored the bool still cannot trade on the result.
func (b *ConfigBudgets) FundingBudget(v ids.Venue) (domain.Budget, bool) {
	spec, ok := b.store.Current().Value.Funding[v]
	if !ok {
		return domain.Budget{}, false
	}
	return spec.Budget(), true
}
