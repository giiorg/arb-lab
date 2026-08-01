package crossvenue

// The persistence-conditioned episode machine (§45e). It restates the
// screening's episode rules (§43's M1) with one structural change: it
// emits WINDOWS instead of accruing inline, because the gate's accrual
// truth lives on two independent settlement grids that the ledger walks
// separately. The interval convention (OpenAt, CloseAt] IS the screening's
// no-lookahead rule made structural — the trigger print is observed, never
// collected, and the exit print is accrued because the position is still
// open when it settles.
//
// The signal here is the decision-time spread ESTIMATE (§45e: what the
// capture's markPrice/tickers streams showed), never the settled rate —
// the screening scored on settled rates and said so; the gate does not get
// that licence.

import (
	"fmt"
	"strconv"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// DecisionPoint is one instant at which entry/exit is evaluated: the
// instant and the decision-time spread estimate, oriented Binance minus
// Bybit like Match.Spread.
type DecisionPoint struct {
	At     time.Time
	Signal fixedpoint.Rate
}

// Window is one episode: open at the trigger's instant (exclusive for
// accrual), closed at the exit print's instant (inclusive). Sign is the
// episode's direction: +1 holds short Binance / long Bybit (the signal
// printed at or above +entry), -1 the opposite. Points counts the decision
// points the episode held through — trigger excluded, exit included — the
// evidence count slice D's floors read.
type Window struct {
	OpenAt  time.Time
	CloseAt time.Time
	Sign    int
	Points  int
}

// Validate rejects the unusable zero value and every malformed direction.
func (w Window) Validate() error {
	if w.Sign != 1 && w.Sign != -1 {
		return fmt.Errorf("window sign %d: %w", w.Sign, ErrWindowInvalid)
	}
	if w.OpenAt.IsZero() || w.CloseAt.IsZero() || w.CloseAt.Before(w.OpenAt) {
		return fmt.Errorf("window %s to %s: %w",
			w.OpenAt.Format(time.RFC3339Nano), w.CloseAt.Format(time.RFC3339Nano), ErrWindowInvalid)
	}
	if w.Points < 0 {
		return fmt.Errorf("window points %d: %w", w.Points, ErrWindowInvalid)
	}
	return nil
}

// Episodes walks the decision series and emits every episode the entry
// level and hysteresis exit admit. The exit level is entry/exitDivisor at
// RoundTowardZero — the screening's own halving, kept bit-identical so the
// two implementations state one rule. Rules, restated from §43's M1:
//
//   - flat: a print with |signal| at or above entry opens a window AT that
//     instant, in the print's direction; the trigger is not counted.
//   - open: a spacing wider than maxGap closes the window at the PREVIOUS
//     instant first (the hole is evidence we do not have), and the current
//     print may then trigger afresh.
//   - open: the print is counted, then a sign flip or |signal| below the
//     exit level closes the window at THIS instant; a flip that lands at
//     or beyond the entry level re-triggers on the same print.
//   - end of data closes any open window at the last instant — the
//     conservative reading of an unfinished episode (it will still be
//     charged a full round trip).
func Episodes(points []DecisionPoint, entry fixedpoint.Rate, exitDivisor int, maxGap time.Duration) ([]Window, error) {
	if len(points) == 0 {
		return nil, ErrNoPoints
	}
	if err := entry.Validate(); err != nil {
		return nil, fmt.Errorf("entry: %w", err)
	}
	if s, err := entry.Sign(); err != nil {
		return nil, fmt.Errorf("entry: %w", err)
	} else if s <= 0 {
		return nil, fmt.Errorf("entry %s: %w", entry.Decimal(), ErrEntryInvalid)
	}
	if exitDivisor < 1 {
		return nil, fmt.Errorf("divisor %d: %w", exitDivisor, ErrExitDivisorInvalid)
	}
	if maxGap <= 0 {
		return nil, fmt.Errorf("max gap %s: %w", maxGap, ErrGapInvalid)
	}
	divisor, err := fixedpoint.ParseRate(strconv.Itoa(exitDivisor))
	if err != nil {
		return nil, fmt.Errorf("divisor %d: %w", exitDivisor, err)
	}
	exit, err := entry.Div(divisor, fixedpoint.RoundTowardZero)
	if err != nil {
		return nil, fmt.Errorf("exit level: %w", err)
	}

	var out []Window
	open := false
	var cur Window

	closeAt := func(at time.Time) {
		cur.CloseAt = at
		out = append(out, cur)
		open = false
		cur = Window{}
	}

	for i, p := range points {
		if err := p.Signal.Validate(); err != nil {
			return nil, fmt.Errorf("point %d at %s: %w", i, p.At.Format(time.RFC3339Nano), err)
		}
		if i > 0 && !p.At.After(points[i-1].At) {
			return nil, fmt.Errorf("point %d at %s after %s: %w",
				i, p.At.Format(time.RFC3339Nano), points[i-1].At.Format(time.RFC3339Nano), ErrSeriesDisordered)
		}

		if open && p.At.Sub(points[i-1].At) > maxGap {
			closeAt(points[i-1].At)
		}

		absS, err := p.Signal.Abs()
		if err != nil {
			return nil, fmt.Errorf("point %d: %w", i, err)
		}
		sgn, err := p.Signal.Sign()
		if err != nil {
			return nil, fmt.Errorf("point %d: %w", i, err)
		}

		if !open {
			if c, err := absS.Cmp(entry); err != nil {
				return nil, fmt.Errorf("point %d: %w", i, err)
			} else if c >= 0 {
				open = true
				cur = Window{OpenAt: p.At, Sign: sgn}
			}
			continue
		}

		cur.Points++

		belowExit, err := absS.Cmp(exit)
		if err != nil {
			return nil, fmt.Errorf("point %d: %w", i, err)
		}
		if sgn != cur.Sign || belowExit < 0 {
			closeAt(p.At)
			// The same print may trigger a fresh episode in its own
			// direction (a sign flip that lands beyond the entry level).
			if sgn != 0 {
				if c, err := absS.Cmp(entry); err != nil {
					return nil, fmt.Errorf("point %d: %w", i, err)
				} else if c >= 0 {
					open = true
					cur = Window{OpenAt: p.At, Sign: sgn}
				}
			}
		}
	}
	if open {
		closeAt(points[len(points)-1].At)
	}
	return out, nil
}
