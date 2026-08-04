package ratelimit_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// binanceShaped is a profile with the structure §7.6 describes — separate
// product weight/order counters plus one shared venue IP envelope — with
// numbers chosen for arithmetic, not copied from a venue. The venue's real
// budgets are not in venue-profiles.md as verified facts, and this package
// deliberately holds no venue table (see the package doc).
func binanceShaped() ratelimit.Profile {
	return ratelimit.Profile{
		Venue:    ids.Venue("binance"),
		Families: []ids.ProductCategory{ids.ProductSpot, ids.ProductLinearPerp},
		Limits: []ratelimit.Limit{
			{
				Name: "weight_1m", Scope: ratelimit.ScopeFamily, Family: ids.ProductSpot,
				Dimension: ratelimit.DimensionWeight, Interval: time.Minute,
				OpeningCeiling: 70, RoutineCeiling: 90, Capacity: 100,
			},
			{
				// The same name under a different family is a different limit.
				Name: "weight_1m", Scope: ratelimit.ScopeFamily, Family: ids.ProductLinearPerp,
				Dimension: ratelimit.DimensionWeight, Interval: time.Minute,
				OpeningCeiling: 140, RoutineCeiling: 180, Capacity: 200,
			},
			{
				Name: "orders_10s", Scope: ratelimit.ScopeFamily, Family: ids.ProductLinearPerp,
				Dimension: ratelimit.DimensionOrders, Interval: 10 * time.Second,
				OpeningCeiling: 7, RoutineCeiling: 9, Capacity: 10,
			},
			{
				// The shared IP-ban envelope: venue-scoped, request-counted,
				// and unheadered, so nothing will ever correct it.
				Name: "ip_5s", Scope: ratelimit.ScopeVenue,
				Dimension: ratelimit.DimensionRequests, Interval: 5 * time.Second,
				OpeningCeiling: 42, RoutineCeiling: 54, Capacity: 60,
			},
			{
				// Spot's own order budget, appended last so the index-addressed
				// cases above it stay stable. Distinct numbers from perp's, so a
				// snapshot or refusal attributing one family's budget to the
				// other cannot produce the same figures.
				Name: "orders_10s", Scope: ratelimit.ScopeFamily, Family: ids.ProductSpot,
				Dimension: ratelimit.DimensionOrders, Interval: 10 * time.Second,
				OpeningCeiling: 14, RoutineCeiling: 18, Capacity: 20,
			},
		},
		ThrottlePause: 30 * time.Second,
		BanPause:      2 * time.Minute,
	}
}

func TestProfileValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(*ratelimit.Profile)
		want    error
		wantMsg string
	}{
		// Positive control — and it carries the same-name-different-family
		// case, so a key that collapsed to the bare name would fail here.
		{"the binance-shaped profile is usable", func(*ratelimit.Profile) {}, nil, ""},

		{"an unset venue is refused", func(p *ratelimit.Profile) { p.Venue = "" },
			ids.ErrVenueUnset, ""},

		{"no limits is refused", func(p *ratelimit.Profile) { p.Limits = nil },
			ratelimit.ErrProfileEmpty, ""},
		{"no families is refused", func(p *ratelimit.Profile) {
			p.Families = nil
			p.Limits = p.Limits[3:4] // the venue-scoped envelope alone
		}, ratelimit.ErrProfileFamilies, "no API famil"},
		{"a duplicate family is refused", func(p *ratelimit.Profile) {
			p.Families = append(p.Families, ids.ProductSpot)
		}, ratelimit.ErrProfileFamilies, "twice"},
		{"an unknown family is refused", func(p *ratelimit.Profile) {
			p.Families = append(p.Families, ids.ProductCategory("futures"))
		}, ids.ErrProductUnknown, ""},

		{"a limit on an undeclared family is refused", func(p *ratelimit.Profile) {
			p.Families = []ids.ProductCategory{ids.ProductSpot}
		}, ratelimit.ErrProfileFamilyUndeclared, "linear_perp"},

		{"a malformed limit is refused", func(p *ratelimit.Profile) { p.Limits[0].Interval = 0 },
			ratelimit.ErrLimitInterval, ""},
		{"the same name twice under one family is refused", func(p *ratelimit.Profile) {
			p.Limits[2].Name = "weight_1m"
		}, ratelimit.ErrLimitDuplicate, "weight_1m"},
		{"the same venue-scoped name twice is refused", func(p *ratelimit.Profile) {
			dup := p.Limits[3]
			p.Limits = append(p.Limits, dup)
		}, ratelimit.ErrLimitDuplicate, "ip_5s"},

		// §7.6's coverage rule (task #173). The two uncovered-dimension cases
		// drop DIFFERENT families' budgets: with both families uncovered, "the
		// refusal names the uncovered family" and "the refusal names the first
		// family" are the same answer, and dropping only perp's proves the loop
		// walked PAST a covered spot.
		{"an uncounted dimension on the first family is refused", func(p *ratelimit.Profile) {
			p.Limits = p.Limits[:4] // drop spot's orders budget
		}, ratelimit.ErrProfileDimensionUncounted, `family "spot": no budget counts DimensionOrders`},
		{"an uncounted dimension on a later family is refused", func(p *ratelimit.Profile) {
			p.Limits = append(p.Limits[:2], p.Limits[3:]...) // drop perp's orders budget
		}, ratelimit.ErrProfileDimensionUncounted, `family "linear_perp": no budget counts DimensionOrders`},
		{"the refusal names the uncovered dimension, not just the family", func(p *ratelimit.Profile) {
			p.Limits = p.Limits[1:] // drop spot's weight budget, keep its orders
		}, ratelimit.ErrProfileDimensionUncounted, "DimensionWeight"},
		{"a missing venue envelope is refused", func(p *ratelimit.Profile) {
			p.Limits = append(p.Limits[:3], p.Limits[4:]...)
		}, ratelimit.ErrProfileEnvelopeMissing, ""},
		{"family-scoped request limits do not satisfy the envelope", func(p *ratelimit.Profile) {
			// Both families fully covered, requests counted per family — and
			// still refused: each counter can be inside its own capacity while
			// their sum trips the shared gateway ban neither reports on.
			p.Limits[3] = ratelimit.Limit{
				Name: "req_5s", Scope: ratelimit.ScopeFamily, Family: ids.ProductSpot,
				Dimension: ratelimit.DimensionRequests, Interval: 5 * time.Second,
				OpeningCeiling: 42, RoutineCeiling: 54, Capacity: 60,
			}
			p.Limits = append(p.Limits, ratelimit.Limit{
				Name: "req_5s", Scope: ratelimit.ScopeFamily, Family: ids.ProductLinearPerp,
				Dimension: ratelimit.DimensionRequests, Interval: 5 * time.Second,
				OpeningCeiling: 42, RoutineCeiling: 54, Capacity: 60,
			})
		}, ratelimit.ErrProfileEnvelopeMissing, ""},
		{"a venue-scoped weight limit is refused", func(p *ratelimit.Profile) {
			p.Limits[3].Dimension = ratelimit.DimensionWeight
		}, ratelimit.ErrLimitScopeDimension, "ip_5s"},

		// The written escape, and its own validation.
		{"an Uncounted reason satisfies coverage", func(p *ratelimit.Profile) {
			p.Limits = p.Limits[:4]
			p.Uncounted = []ratelimit.Uncounted{{
				Family: ids.ProductSpot, Dimension: ratelimit.DimensionOrders,
				Reason: "fixture: spot order flow is not under test here",
			}}
		}, nil, ""},
		{"an Uncounted entry for requests is refused", func(p *ratelimit.Profile) {
			p.Uncounted = []ratelimit.Uncounted{{
				Family: ids.ProductSpot, Dimension: ratelimit.DimensionRequests,
				Reason: "the venue documents no request limit",
			}}
		}, ratelimit.ErrProfileUncountedInvalid, "not waivable"},
		{"an Uncounted entry naming an undeclared family is refused", func(p *ratelimit.Profile) {
			p.Uncounted = []ratelimit.Uncounted{{
				Family: ids.ProductInversePerp, Dimension: ratelimit.DimensionOrders,
				Reason: "no such family here",
			}}
		}, ratelimit.ErrProfileUncountedInvalid, "undeclared"},
		{"an Uncounted entry with no reason is refused", func(p *ratelimit.Profile) {
			p.Limits = p.Limits[:4]
			p.Uncounted = []ratelimit.Uncounted{{
				Family: ids.ProductSpot, Dimension: ratelimit.DimensionOrders,
				Reason: "   ",
			}}
		}, ratelimit.ErrProfileUncountedInvalid, "no reason"},
		{"a duplicate Uncounted entry is refused", func(p *ratelimit.Profile) {
			p.Limits = p.Limits[:4]
			p.Uncounted = []ratelimit.Uncounted{
				{Family: ids.ProductSpot, Dimension: ratelimit.DimensionOrders, Reason: "once"},
				{Family: ids.ProductSpot, Dimension: ratelimit.DimensionOrders, Reason: "twice"},
			}
		}, ratelimit.ErrProfileUncountedInvalid, "twice"},
		{"an Uncounted entry a declared budget contradicts is refused", func(p *ratelimit.Profile) {
			p.Uncounted = []ratelimit.Uncounted{{
				Family: ids.ProductSpot, Dimension: ratelimit.DimensionOrders,
				Reason: "stale: spot orders became budgeted after this was written",
			}}
		}, ratelimit.ErrProfileUncountedContradicted, ""},

		{"a zero throttle pause is refused", func(p *ratelimit.Profile) { p.ThrottlePause = 0 },
			ratelimit.ErrPauseInvalid, "throttle"},
		{"a ban pause shorter than the throttle pause is refused", func(p *ratelimit.Profile) {
			p.BanPause = time.Second
		}, ratelimit.ErrPauseInvalid, "ban"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := binanceShaped()
			tc.mutate(&p)

			err := p.Validate()
			if !errors.Is(err, tc.want) {
				t.Fatalf("Profile.Validate() = %v, want %v", err, tc.want)
			}
			if tc.wantMsg != "" && !strings.Contains(err.Error(), tc.wantMsg) {
				t.Errorf("Profile.Validate() = %q, want the message to contain %q", err, tc.wantMsg)
			}
		})
	}
}
