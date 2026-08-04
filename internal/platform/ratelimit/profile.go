package ratelimit

import (
	"fmt"
	"strings"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// Profile is one venue's rate-limit configuration: its API families, its
// budgets, and how long a penalty holds it off.
//
// It is data supplied by the caller. This package holds no venue numbers — see
// the package doc for why that is a stronger rule here than the precedent it
// follows.
type Profile struct {
	// Venue names the venue these limits belong to. One governor, one venue,
	// because ScopeVenue means "shared across this venue's families" and a
	// governor spanning two venues would have no such thing to be.
	Venue ids.Venue

	// Families are the API families requests may name.
	//
	// ids.ProductCategory rather than a free string, and the fit is not a
	// coincidence: on Binance the product families ARE separate API hosts with
	// separate budgets (spot, USDⓈ-M, COIN-M), which is exactly the distinction
	// §5.1 already draws for an instrument. Reusing it means an adapter naming a
	// family and an adapter naming an instrument cannot drift apart.
	//
	// Declared explicitly rather than derived from Limits, so that a family
	// carrying no product budget of its own — only the venue envelope — is
	// still a family a request may legally name, and a typo is not.
	Families []ids.ProductCategory

	// Limits are the budgets, at both scopes.
	Limits []Limit

	// Uncounted is the written escape from the coverage rule below: each entry
	// declares that one family deliberately budgets nothing for one dimension,
	// and why. Same shape as //nofloat:allow and observability's Unbounded —
	// the default is deny, and the person taking on the risk writes down why
	// they are entitled to. There is no wildcard family, because an exemption
	// generalizes over its predicate: one written for the family where the
	// claim holds would silently release the family where it does not.
	//
	// A declaration is a claim about traffic, not a suppression: if a cost
	// charging the waived dimension arrives anyway, the Governor still refuses
	// it (ErrCostUncounted) — the arrival is the claim's falsification,
	// delivered fail-closed before anything is sent.
	Uncounted []Uncounted

	// ThrottlePause is how long a PenaltyThrottle holds the venue off when the
	// venue supplies no hint of its own: §7.6's "hard pause" on a Binance 429 or
	// a Bybit 403+10006.
	ThrottlePause time.Duration

	// BanPause is the same for a PenaltyBan — a 418 or Bybit's unheadered
	// retCode 10018 — and must be at least ThrottlePause, because §7.6 puts a
	// ban in a strictly more serious class.
	//
	// Neither is a maximum. A venue hint longer than the configured pause wins;
	// see Governor.Penalize.
	BanPause time.Duration
}

// Uncounted is one family's written declaration that one dimension is
// deliberately not budgeted. See Profile.Uncounted.
type Uncounted struct {
	// Family is the one family the declaration covers. Must be declared in
	// Families.
	Family ids.ProductCategory

	// Dimension is the dimension left unbudgeted. DimensionRequests is refused
	// here: the venue envelope is not waivable (see ErrProfileEnvelopeMissing).
	Dimension Dimension

	// Reason is why — non-empty, in the operator's words. It is the same
	// artifact as a //nofloat:allow reason: the point is that forgetting
	// produces a refusal naming the dimension, while declining produces a
	// sentence somebody signed.
	Reason string
}

// Validate reports whether the profile can construct a governor.
//
// # The coverage rule (§7.6, task #173)
//
// A charge is applied by walking the budgets this profile declares and taking
// each one's dimension from the cost, so a dimension with no budget is
// silently uncounted — and a governor that counts nothing admits everything.
// Binance's futures order is the worked case: documented at ZERO weight and 1
// on each order-count budget, so a weight-only profile charges literally
// nothing for placing it. Validate therefore refuses a profile that cannot
// count what a cost can charge:
//
//   - every profile declares a venue-scoped REQUESTS limit — §7.6's shared
//     IP-ban envelope. Family-scoped request counters do not satisfy this:
//     each can be inside its own capacity while their sum trips the gateway
//     ban neither reports on. Not waivable — the envelope's capacity is a
//     locally chosen bound, not a venue fact somebody might lack.
//   - every family budgets every other dimension (weight, orders, and any
//     member added later — the loop runs to maxDimension, so a new dimension
//     joins the required set without anyone remembering it) with a
//     FAMILY-scoped limit, or carries a written Uncounted declaration.
//     Venue-scoped limits do not satisfy product-dimension coverage: the
//     header fold addresses (family, name), so a venue-scoped weight or order
//     budget is a fold target nothing can reach — Limit.Validate refuses the
//     construction outright.
func (p Profile) Validate() error {
	if err := p.Venue.Validate(); err != nil {
		return err
	}
	families, err := p.validateFamilies()
	if err != nil {
		return err
	}
	if len(p.Limits) == 0 {
		return fmt.Errorf("venue %q: %w", p.Venue, ErrProfileEmpty)
	}

	seen := make(map[limitKey]struct{}, len(p.Limits))
	counted := make(map[ids.ProductCategory]map[Dimension]bool, len(p.Families))
	envelope := false
	for _, l := range p.Limits {
		if err := l.Validate(); err != nil {
			return fmt.Errorf("venue %q: %w", p.Venue, err)
		}
		if l.Scope == ScopeFamily {
			if _, ok := families[l.Family]; !ok {
				return fmt.Errorf("venue %q limit %q names family %q: %w",
					p.Venue, l.Name, l.Family, ErrProfileFamilyUndeclared)
			}
			if counted[l.Family] == nil {
				counted[l.Family] = make(map[Dimension]bool)
			}
			counted[l.Family][l.Dimension] = true
		}
		if l.Scope == ScopeVenue && l.Dimension == DimensionRequests {
			envelope = true
		}
		k := l.key()
		if _, dup := seen[k]; dup {
			return fmt.Errorf("venue %q declares limit %q twice at %s: %w",
				p.Venue, l.Name, l.Scope, ErrLimitDuplicate)
		}
		seen[k] = struct{}{}
	}
	if !envelope {
		return fmt.Errorf("venue %q: %w", p.Venue, ErrProfileEnvelopeMissing)
	}
	waived, err := p.validateUncounted(families, counted)
	if err != nil {
		return err
	}
	for _, f := range p.Families {
		for d := DimensionUnspecified + 1; d <= maxDimension; d++ {
			if d == DimensionRequests {
				// Counted by the venue envelope required above, for every
				// family, by construction.
				continue
			}
			if counted[f][d] || waived[f][d] {
				continue
			}
			return fmt.Errorf(
				"venue %q family %q: no budget counts %s — declare a family-scoped limit or an Uncounted reason: %w",
				p.Venue, f, d, ErrProfileDimensionUncounted)
		}
	}
	return p.validatePauses()
}

// validateUncounted checks the written declarations and returns them as a
// lookup. counted is consulted for the contradiction case: a declaration for a
// dimension a family-scoped budget already counts is a stale claim that would
// quietly survive the budget's later removal.
func (p Profile) validateUncounted(
	families map[ids.ProductCategory]struct{},
	counted map[ids.ProductCategory]map[Dimension]bool,
) (map[ids.ProductCategory]map[Dimension]bool, error) {
	waived := make(map[ids.ProductCategory]map[Dimension]bool, len(p.Uncounted))
	for _, u := range p.Uncounted {
		if _, ok := families[u.Family]; !ok {
			return nil, fmt.Errorf("venue %q uncounted %s/%s names an undeclared family: %w",
				p.Venue, u.Family, u.Dimension, ErrProfileUncountedInvalid)
		}
		if err := u.Dimension.Validate(); err != nil {
			return nil, fmt.Errorf("venue %q uncounted for family %q: %w: %w",
				p.Venue, u.Family, err, ErrProfileUncountedInvalid)
		}
		if u.Dimension == DimensionRequests {
			return nil, fmt.Errorf("venue %q family %q: the venue request envelope is not waivable: %w",
				p.Venue, u.Family, ErrProfileUncountedInvalid)
		}
		if strings.TrimSpace(u.Reason) == "" {
			return nil, fmt.Errorf("venue %q uncounted %s/%s declares no reason: %w",
				p.Venue, u.Family, u.Dimension, ErrProfileUncountedInvalid)
		}
		if waived[u.Family][u.Dimension] {
			return nil, fmt.Errorf("venue %q uncounted %s/%s is declared twice: %w",
				p.Venue, u.Family, u.Dimension, ErrProfileUncountedInvalid)
		}
		if counted[u.Family][u.Dimension] {
			return nil, fmt.Errorf("venue %q uncounted %s/%s: a declared budget counts it: %w",
				p.Venue, u.Family, u.Dimension, ErrProfileUncountedContradicted)
		}
		if waived[u.Family] == nil {
			waived[u.Family] = make(map[Dimension]bool)
		}
		waived[u.Family][u.Dimension] = true
	}
	return waived, nil
}

func (p Profile) validateFamilies() (map[ids.ProductCategory]struct{}, error) {
	if len(p.Families) == 0 {
		return nil, fmt.Errorf("venue %q declares no API families: %w", p.Venue, ErrProfileFamilies)
	}
	families := make(map[ids.ProductCategory]struct{}, len(p.Families))
	for _, f := range p.Families {
		if err := f.Validate(); err != nil {
			return nil, fmt.Errorf("venue %q: %w", p.Venue, err)
		}
		if _, dup := families[f]; dup {
			return nil, fmt.Errorf("venue %q declares family %q twice: %w",
				p.Venue, f, ErrProfileFamilies)
		}
		families[f] = struct{}{}
	}
	return families, nil
}

func (p Profile) validatePauses() error {
	if p.ThrottlePause <= 0 {
		return fmt.Errorf("venue %q throttle pause %v: %w", p.Venue, p.ThrottlePause, ErrPauseInvalid)
	}
	if p.BanPause < p.ThrottlePause {
		return fmt.Errorf("venue %q ban pause %v is shorter than its throttle pause %v: %w",
			p.Venue, p.BanPause, p.ThrottlePause, ErrPauseInvalid)
	}
	return nil
}
