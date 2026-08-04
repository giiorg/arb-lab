package ratelimit_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// The ladder is the package's central claim, so it is asserted as an ordering
// rather than as four independent constants: a reordering that swapped Routine
// and Safety would leave every individual Validate test green.
func TestPriorityOrdersOpeningBelowRoutineBelowSafety(t *testing.T) {
	t.Parallel()

	if !(ratelimit.PriorityOpening < ratelimit.PriorityRoutine) {
		t.Errorf("PriorityOpening (%d) must sort below PriorityRoutine (%d): a venue "+
			"under stress sheds new exposure before it sheds the reads that monitor it",
			ratelimit.PriorityOpening, ratelimit.PriorityRoutine)
	}
	if !(ratelimit.PriorityRoutine < ratelimit.PrioritySafety) {
		t.Errorf("PriorityRoutine (%d) must sort below PrioritySafety (%d): §7.1's reserved "+
			"allocation exists so trading traffic cannot starve a margin top-up",
			ratelimit.PriorityRoutine, ratelimit.PrioritySafety)
	}
}

func TestPriorityValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		p    ratelimit.Priority
		want error
	}{
		// Positive controls. Without these a Validate that rejected everything
		// would score a perfect run on the negative cases alone.
		{"opening is usable", ratelimit.PriorityOpening, nil},
		{"routine is usable", ratelimit.PriorityRoutine, nil},
		{"safety is usable", ratelimit.PrioritySafety, nil},

		{"the zero value is refused", ratelimit.PriorityUnspecified, ratelimit.ErrPriorityInvalid},
		{"a value past the ladder is refused", ratelimit.PrioritySafety + 1, ratelimit.ErrPriorityInvalid},
		{"a wildly out-of-range value is refused", ratelimit.Priority(200), ratelimit.ErrPriorityInvalid},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.p.Validate()
			if !errors.Is(err, tc.want) {
				t.Fatalf("Priority(%d).Validate() = %v, want %v", uint8(tc.p), err, tc.want)
			}
		})
	}
}

// The zero value is refused rather than defaulted, and neither default is safe:
// defaulting to Safety lets a forgotten field bypass the whole throttle and
// earn a ban; defaulting to Opening lets a forgotten flatten be refused. So the
// refusal has to name the field an operator must go and set.
func TestPriorityValidateNamesTheField(t *testing.T) {
	t.Parallel()

	err := ratelimit.PriorityUnspecified.Validate()
	if err == nil {
		t.Fatal("PriorityUnspecified.Validate() = nil, want a refusal")
	}
	if !strings.Contains(err.Error(), "priority") {
		t.Errorf("Validate() = %q, want the message to name the priority field", err)
	}
}

func TestPriorityString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		p    ratelimit.Priority
		want string
	}{
		{ratelimit.PriorityUnspecified, "PriorityUnspecified"},
		{ratelimit.PriorityOpening, "PriorityOpening"},
		{ratelimit.PriorityRoutine, "PriorityRoutine"},
		{ratelimit.PrioritySafety, "PrioritySafety"},
		{ratelimit.Priority(9), "Priority(9)"},
	}

	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			t.Parallel()
			if got := tc.p.String(); got != tc.want {
				t.Errorf("Priority(%d).String() = %q, want %q", uint8(tc.p), got, tc.want)
			}
		})
	}
}

func TestDimensionValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		d    ratelimit.Dimension
		want error
	}{
		{"weight is usable", ratelimit.DimensionWeight, nil},
		{"orders is usable", ratelimit.DimensionOrders, nil},
		{"requests is usable", ratelimit.DimensionRequests, nil},

		{"the zero value is refused", ratelimit.DimensionUnspecified, ratelimit.ErrDimensionInvalid},
		{"a value past the set is refused", ratelimit.DimensionRequests + 1, ratelimit.ErrDimensionInvalid},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := tc.d.Validate(); !errors.Is(err, tc.want) {
				t.Fatalf("Dimension(%d).Validate() = %v, want %v", uint8(tc.d), err, tc.want)
			}
		})
	}
}

func TestDimensionString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		d    ratelimit.Dimension
		want string
	}{
		{ratelimit.DimensionUnspecified, "DimensionUnspecified"},
		{ratelimit.DimensionWeight, "DimensionWeight"},
		{ratelimit.DimensionOrders, "DimensionOrders"},
		{ratelimit.DimensionRequests, "DimensionRequests"},
		{ratelimit.Dimension(7), "Dimension(7)"},
	}

	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			t.Parallel()
			if got := tc.d.String(); got != tc.want {
				t.Errorf("Dimension(%d).String() = %q, want %q", uint8(tc.d), got, tc.want)
			}
		})
	}
}

func TestScopeValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		s    ratelimit.Scope
		want error
	}{
		{"venue is usable", ratelimit.ScopeVenue, nil},
		{"family is usable", ratelimit.ScopeFamily, nil},
		{"endpoint is usable", ratelimit.ScopeEndpoint, nil},

		{"the zero value is refused", ratelimit.ScopeUnspecified, ratelimit.ErrScopeInvalid},
		{"a value past the set is refused", ratelimit.ScopeEndpoint + 1, ratelimit.ErrScopeInvalid},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := tc.s.Validate(); !errors.Is(err, tc.want) {
				t.Fatalf("Scope(%d).Validate() = %v, want %v", uint8(tc.s), err, tc.want)
			}
		})
	}
}

func TestScopeString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		s    ratelimit.Scope
		want string
	}{
		{ratelimit.ScopeUnspecified, "ScopeUnspecified"},
		{ratelimit.ScopeVenue, "ScopeVenue"},
		{ratelimit.ScopeFamily, "ScopeFamily"},
		{ratelimit.ScopeEndpoint, "ScopeEndpoint"},
		{ratelimit.Scope(5), "Scope(5)"},
	}

	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			t.Parallel()
			if got := tc.s.String(); got != tc.want {
				t.Errorf("Scope(%d).String() = %q, want %q", uint8(tc.s), got, tc.want)
			}
		})
	}
}
