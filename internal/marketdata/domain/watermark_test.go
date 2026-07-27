package domain_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/marketdata/domain"
)

func TestWatermarkValidate(t *testing.T) {
	t.Parallel()

	full := domain.Watermark{VenueSeq: 7, SourceTime: base, RecvTime: base}
	if err := full.Validate(); err != nil {
		t.Fatalf("the fixture watermark must be valid: %v", err)
	}

	// VenueSeq zero is LEGAL and this is the case that says so. A REST-polled
	// funding read carries no sequence at all, so refusing zero would make the
	// polled source unrepresentable — and the tempting fix, defaulting it to 1,
	// would manufacture a sequence a consumer could run gap detection against.
	noSeq := domain.Watermark{VenueSeq: 0, SourceTime: base, RecvTime: base}
	if err := noSeq.Validate(); err != nil {
		t.Errorf("a watermark with no venue sequence must be valid: %v", err)
	}

	cases := []struct {
		name  string
		w     domain.Watermark
		names string
	}{
		{"unset source time", domain.Watermark{RecvTime: base}, "source time"},
		{"unset receive time", domain.Watermark{SourceTime: base}, "receive time"},
		{"both unset", domain.Watermark{}, "source time"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			err := c.w.Validate()
			if !errors.Is(err, domain.ErrWatermarkUnset) {
				t.Fatalf("Validate() = %v, want ErrWatermarkUnset", err)
			}
			// One sentinel covers both timestamps, so the message is the only
			// thing that says WHICH is missing — §8f.
			if !strings.Contains(err.Error(), c.names) {
				t.Errorf("message %q does not name %q", err, c.names)
			}
		})
	}
}

// TestAgeAndDataAgeAreDifferentQuestions is the point of having two methods.
// The fixture is the failure that is invisible to the check a person writes
// first: a venue-side feed that stalls while the transport stays healthy.
func TestAgeAndDataAgeAreDifferentQuestions(t *testing.T) {
	t.Parallel()

	// Updates keep arriving on time — RecvTime is current — but every one of
	// them carries the same hour-old venue stamp. The two timestamps therefore
	// differ, which is what makes this fixture able to tell the methods apart:
	// with SourceTime == RecvTime, Age and DataAge are the same function and a
	// mutant swapping the fields survives.
	w := domain.Watermark{
		VenueSeq:   9,
		SourceTime: base.Add(-time.Hour),
		RecvTime:   base,
	}

	got, err := w.Age(base)
	if err != nil {
		t.Fatalf("Age: %v", err)
	}
	if got != 0 {
		t.Errorf("Age = %v, want 0 — the transport is healthy", got)
	}

	// Measured against the venue's own clock, not engine time.
	gotData, err := w.DataAge(base)
	if err != nil {
		t.Fatalf("DataAge: %v", err)
	}
	if gotData != time.Hour {
		t.Errorf("DataAge = %v, want 1h — the venue's view is stale", gotData)
	}

	// And the consequence: the transport check reports fresh for the whole
	// outage while the data check reports blind. If these ever agree, one of
	// the two failure modes has stopped being detectable.
	if f, _ := budget.Assess(w, base); f != domain.FreshnessFresh {
		t.Errorf("Assess = %s, want Fresh", f)
	}
	if f, _ := budget.AssessData(w, base); f != domain.FreshnessBlind {
		t.Errorf("AssessData = %s, want Blind", f)
	}
}

func TestAgeRefusals(t *testing.T) {
	t.Parallel()

	w := domain.Watermark{VenueSeq: 1, SourceTime: base, RecvTime: base}

	// now == then is a zero age, not an inversion: the equal case, where
	// Before and !After would otherwise be the same predicate.
	if got, err := w.Age(base); err != nil || got != 0 {
		t.Errorf("Age at exactly RecvTime = (%v, %v), want (0, nil)", got, err)
	}
	if got, err := w.DataAge(base); err != nil || got != 0 {
		t.Errorf("DataAge at exactly SourceTime = (%v, %v), want (0, nil)", got, err)
	}

	for _, c := range []struct {
		name string
		fn   func(time.Time) (time.Duration, error)
	}{
		{"Age", w.Age},
		{"DataAge", w.DataAge},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			// The inversion check must be on BOTH paths. It is written once in
			// an unexported helper precisely so it cannot be present on one and
			// missing on the other — and this loop is what proves that claim
			// rather than trusting the refactor. Same rule as Receive/Send in
			// platform/events: when a fix earns a test, its mirror gets one.
			if _, err := c.fn(base.Add(-time.Nanosecond)); !errors.Is(err, domain.ErrClockInversion) {
				t.Errorf("%s one ns early = %v, want ErrClockInversion", c.name, err)
			}
		})
	}

	// An unset instant is reported as unset, not as an age of fifty-five years.
	if _, err := (domain.Watermark{RecvTime: base}).DataAge(base); !errors.Is(err, domain.ErrWatermarkUnset) {
		t.Errorf("DataAge with no source time = %v, want ErrWatermarkUnset", err)
	}
	if _, err := (domain.Watermark{SourceTime: base}).Age(base); !errors.Is(err, domain.ErrWatermarkUnset) {
		t.Errorf("Age with no receive time = %v, want ErrWatermarkUnset", err)
	}
}
