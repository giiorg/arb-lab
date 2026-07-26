package events_test

import (
	"strings"
	"testing"

	"github.com/giiorg/arb-lab/internal/platform/events"
)

// TestLaneDeclaredSet pins the set and, more importantly, pins that the zero
// value is NOT one of them.
//
// Lane takes fixedpoint.RoundingMode's documented integer-enum exception to
// §3.6 rule 8, and the whole justification for that exception is the zero value
// being an unusable sentinel. Unlike apperror.EffectUnspecified, LaneUnspecified
// is not safe to act on either — there is no conservative lane to fall back to,
// because routing an unclassified event to market data may drop it and routing
// everything to control may block trading on a metrics backlog.
func TestLaneDeclaredSet(t *testing.T) {
	declared := []events.Lane{
		events.LaneControl, events.LaneMarketData, events.LaneObservability,
	}
	for _, l := range declared {
		if err := l.Validate(); err != nil {
			t.Errorf("declared %v: Validate() = %v, want nil", l, err)
		}
		if s := l.String(); !strings.HasPrefix(s, "Lane") || strings.Contains(s, "(") {
			t.Errorf("String() = %q, want a declared name", s)
		}
	}

	if err := events.LaneUnspecified.Validate(); err == nil {
		t.Error("LaneUnspecified.Validate() = nil; the zero value must be unusable — that is the entire reason this enum is an integer rather than rule 8's string shape")
	}

	// An undeclared value past the end of the set. The check is `> maxLane`, so
	// this is the case that catches a maxLane left behind when a lane is added.
	if err := events.Lane(99).Validate(); err == nil {
		t.Error("Lane(99).Validate() = nil, want an error")
	}
	if got := events.Lane(99).String(); got != "Lane(99)" {
		t.Errorf("Lane(99).String() = %q, want the numeric fallback", got)
	}
}

// TestLaneStringCoversEveryDeclaredValue is the guard the deferred
// exhaustiveswitch gate (§3.6 rule 10) will eventually make unnecessary.
//
// Until then, this walks every value from the sentinel to maxLane and asserts
// none of them falls through to the numeric default — which is exactly what a
// newly added lane would do.
func TestLaneStringCoversEveryDeclaredValue(t *testing.T) {
	for l := events.LaneUnspecified; l <= events.LaneObservability; l++ {
		if got := l.String(); strings.Contains(got, "(") {
			t.Errorf("Lane(%d).String() = %q — a declared lane fell through to the numeric default; add it to the switch", uint8(l), got)
		}
	}
}
