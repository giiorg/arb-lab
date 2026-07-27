package domain_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/marketdata/domain"
)

// base is an arbitrary non-zero instant. Every test builds from it rather than
// from a literal, so no fixture accidentally uses the zero time — which is the
// value Watermark.Validate refuses, and would turn a freshness assertion into
// an unset-field assertion without saying so.
var base = time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)

// budget is the fixture policy. The two bounds are deliberately NOT multiples
// of each other and neither is a round number of the other's units: a budget of
// 5s/10s would let a mutant that doubled MaxAge land exactly on Blind and
// produce a plausible verdict at every boundary case below.
var budget = domain.Budget{MaxAge: 5 * time.Second, Blind: 12 * time.Second}

func wm(recv time.Time) domain.Watermark {
	return domain.Watermark{VenueSeq: 1, SourceTime: recv, RecvTime: recv}
}

func TestFreshnessValidate(t *testing.T) {
	t.Parallel()

	// The zero value is LEGAL, as in boot.Posture: "nothing observed" is a real
	// answer and the one a zeroed struct must give.
	for _, f := range []domain.Freshness{
		domain.FreshnessUnknown,
		domain.FreshnessBlind,
		domain.FreshnessStale,
		domain.FreshnessFresh,
	} {
		if err := f.Validate(); err != nil {
			t.Errorf("%s: Validate() = %v, want nil", f, err)
		}
	}

	if err := domain.Freshness(4).Validate(); !errors.Is(err, domain.ErrFreshnessInvalid) {
		t.Errorf("Freshness(4).Validate() = %v, want ErrFreshnessInvalid", err)
	}
}

// TestFreshnessOrdering pins the ordering itself, because every other claim in
// this file rests on it. A mutant that reordered the constants would otherwise
// keep the type compiling and flip the meaning of every comparison.
func TestFreshnessOrdering(t *testing.T) {
	t.Parallel()

	ordered := []domain.Freshness{
		domain.FreshnessUnknown,
		domain.FreshnessBlind,
		domain.FreshnessStale,
		domain.FreshnessFresh,
	}
	for i := 1; i < len(ordered); i++ {
		if !(ordered[i-1] < ordered[i]) {
			t.Errorf("%s is not less trusted than %s", ordered[i-1], ordered[i])
		}
	}

	// The zero value must be the least trusted. Stated separately from the
	// ordering above because it is the load-bearing half: a reordering that
	// preserved the sequence but moved the zero would pass the loop.
	if domain.Freshness(0) != domain.FreshnessUnknown {
		t.Errorf("zero value = %s, want FreshnessUnknown", domain.Freshness(0))
	}
}

func TestFreshnessAtLeast(t *testing.T) {
	t.Parallel()

	if !domain.FreshnessFresh.AtLeast(domain.FreshnessStale) {
		t.Error("Fresh.AtLeast(Stale) = false, want true")
	}
	// The equal case, which is where AtLeast's >= differs from a >.
	if !domain.FreshnessStale.AtLeast(domain.FreshnessStale) {
		t.Error("Stale.AtLeast(Stale) = false, want true")
	}
	if domain.FreshnessBlind.AtLeast(domain.FreshnessStale) {
		t.Error("Blind.AtLeast(Stale) = true, want false")
	}
}

func TestLeast(t *testing.T) {
	t.Parallel()

	// Every case carries at least THREE elements with the answer somewhere
	// other than position 0, because with two elements — or with the least one
	// first — "return the least" and "return the first" are the same function.
	// That is boot's three-closers defect, which reversed a slice and swapped
	// only its ends.
	cases := []struct {
		name string
		in   []domain.Freshness
		want domain.Freshness
	}{
		{
			name: "least in the middle",
			in:   []domain.Freshness{domain.FreshnessFresh, domain.FreshnessBlind, domain.FreshnessStale},
			want: domain.FreshnessBlind,
		},
		{
			name: "least last",
			in:   []domain.Freshness{domain.FreshnessFresh, domain.FreshnessStale, domain.FreshnessBlind},
			want: domain.FreshnessBlind,
		},
		{
			name: "least first, so first and least agree — the control",
			in:   []domain.Freshness{domain.FreshnessBlind, domain.FreshnessStale, domain.FreshnessFresh},
			want: domain.FreshnessBlind,
		},
		{
			name: "all equal",
			in:   []domain.Freshness{domain.FreshnessStale, domain.FreshnessStale, domain.FreshnessStale},
			want: domain.FreshnessStale,
		},
		{
			// The load-bearing identity. A fold seeded at FreshnessFresh — which
			// is how one naturally writes a minimum — answers Fresh here, making
			// "no legs were checked" and "every leg is fresh" the same answer.
			name: "empty is the most adverse, not the most permissive",
			in:   nil,
			want: domain.FreshnessUnknown,
		},
		{
			name: "single element",
			in:   []domain.Freshness{domain.FreshnessFresh},
			want: domain.FreshnessFresh,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := domain.Least(c.in...); got != c.want {
				t.Errorf("Least(%v) = %s, want %s", c.in, got, c.want)
			}
		})
	}
}

func TestBudgetValidate(t *testing.T) {
	t.Parallel()

	if err := budget.Validate(); err != nil {
		// The positive control. Without it, a Validate that refused everything
		// would score a perfect pass on the table below.
		t.Fatalf("the fixture budget must be valid: %v", err)
	}

	cases := []struct {
		name string
		b    domain.Budget
		want error
	}{
		{"zero max age", domain.Budget{MaxAge: 0, Blind: time.Second}, domain.ErrBudgetInvalid},
		{"negative max age", domain.Budget{MaxAge: -time.Second, Blind: time.Second}, domain.ErrBudgetInvalid},
		{"zero blind", domain.Budget{MaxAge: time.Second, Blind: 0}, domain.ErrBudgetInvalid},
		{"negative blind", domain.Budget{MaxAge: time.Second, Blind: -time.Second}, domain.ErrBudgetInvalid},
		{
			// THE case. With only the strictly-greater case below, `>=` and `>`
			// are the same predicate and the stale rung silently collapses.
			name: "max age EQUAL to blind leaves the stale rung unreachable",
			b:    domain.Budget{MaxAge: 5 * time.Second, Blind: 5 * time.Second},
			want: domain.ErrBudgetUnreachable,
		},
		{
			name: "max age above blind",
			b:    domain.Budget{MaxAge: 9 * time.Second, Blind: 5 * time.Second},
			want: domain.ErrBudgetUnreachable,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			err := c.b.Validate()
			if !errors.Is(err, c.want) {
				t.Fatalf("Validate() = %v, want %v", err, c.want)
			}
			// Assert the MESSAGE, not only the sentinel: §8f's lesson is that
			// one sentinel cannot say which check fired, so a fixture failing
			// the check AFTER the one under test would pass on the sentinel
			// alone. Both zero-bound cases return ErrBudgetInvalid.
			if !strings.Contains(err.Error(), "max age") && !strings.Contains(err.Error(), "blind") {
				t.Errorf("message %q names neither bound", err)
			}
		})
	}
}

func TestBudgetAt(t *testing.T) {
	t.Parallel()

	// Every boundary is tested AT the bound and one nanosecond either side.
	// ratelimit's freeAt defect was exactly this: every case sat strictly
	// inside or strictly outside, so `<=` and `<` were the same predicate and
	// a mutant swapping them survived.
	cases := []struct {
		name string
		age  time.Duration
		want domain.Freshness
	}{
		{"zero age", 0, domain.FreshnessFresh},
		{"one ns below max age", budget.MaxAge - 1, domain.FreshnessFresh},
		{"EXACTLY at max age is still fresh", budget.MaxAge, domain.FreshnessFresh},
		{"one ns past max age", budget.MaxAge + 1, domain.FreshnessStale},
		{"one ns below blind", budget.Blind - 1, domain.FreshnessStale},
		{"EXACTLY at blind is still stale", budget.Blind, domain.FreshnessStale},
		{"one ns past blind", budget.Blind + 1, domain.FreshnessBlind},
		{"far past blind", 100 * budget.Blind, domain.FreshnessBlind},
		{
			// A negative age must not read as younger than every budget.
			name: "negative age is unknown, not fresh",
			age:  -time.Nanosecond,
			want: domain.FreshnessUnknown,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := budget.At(c.age); got != c.want {
				t.Errorf("At(%v) = %s, want %s", c.age, got, c.want)
			}
		})
	}
}

func TestBudgetAssessClockInversion(t *testing.T) {
	t.Parallel()

	w := wm(base)

	// The equal case first: now == RecvTime is a zero age, NOT an inversion.
	// Without it, `Before` and `!After` are the same predicate here.
	got, err := budget.Assess(w, base)
	if err != nil {
		t.Fatalf("Assess at exactly RecvTime = %v, want nil", err)
	}
	if got != domain.FreshnessFresh {
		t.Errorf("Assess at exactly RecvTime = %s, want Fresh", got)
	}

	// The inversion returns BOTH a safe verdict and the diagnosis. Assert both,
	// because a mutant returning only one of them is a real regression: dropping
	// the error hides §3.6 rule 5's defect, and dropping the safe verdict makes
	// every ignoring call site invent its own.
	got, err = budget.Assess(w, base.Add(-time.Nanosecond))
	if !errors.Is(err, domain.ErrClockInversion) {
		t.Errorf("Assess before RecvTime error = %v, want ErrClockInversion", err)
	}
	if got != domain.FreshnessUnknown {
		t.Errorf("Assess before RecvTime = %s, want FreshnessUnknown (the safe direction)", got)
	}
}

func TestBudgetNextTransition(t *testing.T) {
	t.Parallel()

	w := wm(base)

	cases := []struct {
		name   string
		now    time.Time
		want   time.Time
		wantOK bool
	}{
		{
			// From fresh, the timer must fire when the observation goes stale.
			// The want is written as RecvTime+MaxAge rather than as a literal,
			// but the assertion below also checks it is NOT RecvTime+Blind —
			// otherwise a mutant returning the wrong bound passes whenever the
			// two happen to be computed from the same field.
			name:   "from fresh, next transition is the stale boundary",
			now:    base.Add(time.Second),
			want:   base.Add(budget.MaxAge),
			wantOK: true,
		},
		{
			name:   "from fresh at EXACTLY the stale boundary, still the stale boundary",
			now:    base.Add(budget.MaxAge),
			want:   base.Add(budget.MaxAge),
			wantOK: true,
		},
		{
			name:   "from stale, next transition is the blind boundary",
			now:    base.Add(budget.MaxAge + time.Nanosecond),
			want:   base.Add(budget.Blind),
			wantOK: true,
		},
		{
			name:   "from stale at EXACTLY the blind boundary, still the blind boundary",
			now:    base.Add(budget.Blind),
			want:   base.Add(budget.Blind),
			wantOK: true,
		},
		{
			// Nowhere further to age, so there is nothing to arm.
			name:   "from blind there is no later transition",
			now:    base.Add(budget.Blind + time.Nanosecond),
			wantOK: false,
		},
		{
			name:   "a clock inversion arms nothing",
			now:    base.Add(-time.Nanosecond),
			wantOK: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got, ok := budget.NextTransition(w, c.now)
			if ok != c.wantOK {
				t.Fatalf("NextTransition ok = %v, want %v", ok, c.wantOK)
			}
			if !ok {
				return
			}
			if !got.Equal(c.want) {
				t.Errorf("NextTransition = %s, want %s", got, c.want)
			}
			// The two bounds are distinct instants, so naming the wrong one is
			// caught rather than coincidentally equal.
			if other := base.Add(budget.MaxAge + budget.Blind - c.want.Sub(base)); got.Equal(other) && !c.want.Equal(other) {
				t.Errorf("NextTransition returned the other bound, %s", got)
			}
		})
	}
}

// TestNextTransitionAgreesWithAssess is the property the silence timer actually
// depends on: at the instant NextTransition names, the verdict must genuinely
// have changed. Two implementations of one boundary rule — the classifier and
// the timer arithmetic — are checked against each other rather than each
// against a literal, which is the only way a shared off-by-one is visible.
func TestNextTransitionAgreesWithAssess(t *testing.T) {
	t.Parallel()

	w := wm(base)

	for _, now := range []time.Time{
		base,
		base.Add(time.Second),
		base.Add(budget.MaxAge),
		base.Add(budget.MaxAge + time.Nanosecond),
		base.Add(budget.Blind),
	} {
		before, err := budget.Assess(w, now)
		if err != nil {
			t.Fatalf("Assess(%s) = %v", now, err)
		}
		at, ok := budget.NextTransition(w, now)
		if !ok {
			t.Fatalf("no transition from %s at %s", before, now)
		}

		// One nanosecond before the named instant, the verdict must be
		// unchanged; at it or just after, it must have degraded. Checking the
		// nanosecond before is what makes this bite: without it, an
		// implementation naming an instant far too early would still see a
		// changed verdict at that instant.
		if got, _ := budget.Assess(w, at.Add(-time.Nanosecond)); got != before {
			t.Errorf("from %s at %s: verdict already %s one ns before the named transition", before, now, got)
		}
		if got, _ := budget.Assess(w, at.Add(time.Nanosecond)); got >= before {
			t.Errorf("from %s at %s: verdict still %s just past the named transition", before, now, got)
		}
	}
}
