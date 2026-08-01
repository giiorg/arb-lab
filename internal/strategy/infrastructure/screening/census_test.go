package screening

// Census tests. The boundary fixture sits EXACTLY at the census level —
// the print where ≥ and > disagree — and the split fixtures each
// contain the splitting cause (sign flip, gap) between two prints that
// would otherwise merge.

import (
	"testing"
	"time"
)

// A print exactly at the level qualifies (≥, not >), and one scale-unit
// below does not.
func TestCensusLevelBoundary(t *testing.T) {
	events := CensusPair("TESTUSDT", series(t, "0.0025", "0.0001"))
	if len(events) != 1 || events[0].Settlements != 1 {
		t.Fatalf("events %v, want one single-print event at the exact level", events)
	}
	events = CensusPair("TESTUSDT", series(t, "0.002499999", "0.0001"))
	if len(events) != 0 {
		t.Fatalf("events %v, want none below the level", events)
	}
}

// Consecutive qualifying prints with one sign accumulate into one event
// with peak and sum.
func TestCensusAccumulatesOneEvent(t *testing.T) {
	events := CensusPair("TESTUSDT", series(t, "0.003", "0.0026", "0.0004"))
	if len(events) != 1 {
		t.Fatalf("events %d, want 1", len(events))
	}
	e := events[0]
	if e.Settlements != 2 || e.Direction != 1 {
		t.Fatalf("settlements %d direction %d, want 2 and +1", e.Settlements, e.Direction)
	}
	if e.Peak.Decimal() != "0.003" || e.Sum.Decimal() != "0.0056" {
		t.Fatalf("peak %s sum %s, want 0.003 and 0.0056", e.Peak.Decimal(), e.Sum.Decimal())
	}
	if !e.Start.Equal(t0) || !e.End.Equal(t0.Add(4*time.Hour)) {
		t.Fatalf("span %s..%s, want t0..t0+4h", e.Start, e.End)
	}
}

// A sign flip splits an event even with no sub-level print between.
func TestCensusSplitsOnSignFlip(t *testing.T) {
	events := CensusPair("TESTUSDT", series(t, "0.003", "-0.003"))
	if len(events) != 2 {
		t.Fatalf("events %d, want 2 (sign split)", len(events))
	}
	if events[0].Direction != 1 || events[1].Direction != -1 {
		t.Fatalf("directions %d,%d, want +1,-1", events[0].Direction, events[1].Direction)
	}
}

// A hole wider than the gap bound splits an event: two qualifying
// prints 12h apart on a 4h modal grid are separate dislocations.
func TestCensusSplitsOnGap(t *testing.T) {
	matched := []Match{
		{At: t0, Spread: scrRate("0.003")},
		{At: t0.Add(12 * time.Hour), Spread: scrRate("0.003")}, // > 2×4h
		{At: t0.Add(16 * time.Hour), Spread: scrRate("0.0001")},
		{At: t0.Add(20 * time.Hour), Spread: scrRate("0.0001")},
	}
	events := CensusPair("TESTUSDT", matched)
	if len(events) != 2 {
		t.Fatalf("events %d, want 2 (gap split)", len(events))
	}
}
