package ratelimit

import (
	"fmt"
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

// Validate reports whether the profile can construct a governor.
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
	for _, l := range p.Limits {
		if err := l.Validate(); err != nil {
			return fmt.Errorf("venue %q: %w", p.Venue, err)
		}
		if l.Scope == ScopeFamily {
			if _, ok := families[l.Family]; !ok {
				return fmt.Errorf("venue %q limit %q names family %q: %w",
					p.Venue, l.Name, l.Family, ErrProfileFamilyUndeclared)
			}
		}
		k := l.key()
		if _, dup := seen[k]; dup {
			return fmt.Errorf("venue %q declares limit %q twice at %s: %w",
				p.Venue, l.Name, l.Scope, ErrLimitDuplicate)
		}
		seen[k] = struct{}{}
	}
	return p.validatePauses()
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
