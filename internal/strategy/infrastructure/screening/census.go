package screening

// The dislocation census (§42b-1): every extreme spread event across
// the WHOLE intersection — mismatched-grid pairs included, matched at
// whatever instants coincide — because §42f-2 prices the broad funding
// tier from this table, and a symbol whose steady state ranks nowhere
// may still dislocate. What the census cannot show is executability
// (the book during the event); §42b-1 states that honestly rather than
// letting the table imply it.

import (
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// Event is one dislocation: consecutive matched settlements whose
// |spread| held at or above CensusLevel with one sign, unbroken by a
// gap wider than the pair's bound.
type Event struct {
	Symbol      string
	Start       time.Time
	End         time.Time
	Settlements int
	// Direction is the spread sign: positive means Binance printed
	// higher through the event.
	Direction int
	Peak      fixedpoint.Rate // largest |spread| in the event
	Sum       fixedpoint.Rate // sum of |spread| over the event
}

// CensusPair extracts a pair's events. maxGap uses the same
// MaxGapFactor × modal-interval bound as the episode model.
func CensusPair(symbol string, matched []Match) []Event {
	if len(matched) == 0 {
		return nil
	}
	modal, _ := modalInterval(matched)
	maxGap := time.Duration(MaxGapFactor) * modal
	if len(matched) == 1 {
		// One matched settlement has no measurable spacing; a lone
		// qualifying print is still one event.
		maxGap = 0
	}

	var events []Event
	var cur *Event
	closeEvent := func() {
		if cur != nil {
			events = append(events, *cur)
			cur = nil
		}
	}
	for i, m := range matched {
		absS := mustAbs(m.Spread)
		sgn := mustSign(m.Spread)
		qualifies := mustCmp(absS, CensusLevel) >= 0 && sgn != 0

		if cur != nil {
			gap := i > 0 && m.At.Sub(matched[i-1].At) > maxGap
			if gap || !qualifies || sgn != cur.Direction {
				closeEvent()
			}
		}
		if !qualifies {
			continue
		}
		if cur == nil {
			cur = &Event{
				Symbol: symbol, Start: m.At, End: m.At,
				Settlements: 1, Direction: sgn, Peak: absS, Sum: absS,
			}
			continue
		}
		cur.End = m.At
		cur.Settlements++
		cur.Sum = mustAdd(cur.Sum, absS)
		if mustCmp(absS, cur.Peak) > 0 {
			cur.Peak = absS
		}
	}
	closeEvent()
	return events
}
