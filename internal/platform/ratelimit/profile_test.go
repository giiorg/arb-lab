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
			p.Limits = p.Limits[3:] // the venue-scoped envelope alone
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
