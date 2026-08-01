package crossvenue

// The funding-phase ledger (§45e): each leg accrues at its OWN
// venue-published instants, and a window where one leg has settled and the
// other has not carries the unhedged-accrual term explicitly — a ledger,
// not a footnote. The term needs no cross-leg pairing to define: at any
// instant where both legs have settled equally often the position is
// hedged, so the unhedged term is the running net's deviation from its
// value at the LAST count-balanced instant. Legs settling at one shared
// instant are processed atomically, which is why a phase-aligned grid
// collapses the term to exactly zero rather than approximately.

import (
	"fmt"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// WindowLedger is one episode window priced against the two legs' settled
// series. All rates are oriented by the window's Sign: +1 holds short
// Binance / long Bybit, so a Binance print adds +rate to Net and a Bybit
// print adds -rate; -1 reverses both.
type WindowLedger struct {
	// BinancePrints and BybitPrints count each leg's settlements inside
	// (OpenAt, CloseAt].
	BinancePrints int
	BybitPrints   int

	// BinanceAccrued and BybitAccrued are each leg's raw rate sums over
	// the window, venue orientation (no episode sign applied) — the
	// per-leg evidence the report renders.
	BinanceAccrued fixedpoint.Rate
	BybitAccrued   fixedpoint.Rate

	// Net is the window's signed accrual: Sign × (Binance − Bybit) over
	// the prints above. Costs are NOT netted here — the round-trip charge
	// is the cost assembly's job, applied once per episode.
	Net fixedpoint.Rate

	// PhaseMax and PhaseMin are the extremes of the running unhedged
	// accrual: the signed net recognized since the last instant at which
	// both legs had settled equally often. Zero both ways on a
	// phase-aligned grid; a Binance-first positive print pushes PhaseMax
	// up under Sign +1, a Bybit-first one pushes PhaseMin down.
	PhaseMax fixedpoint.Rate
	PhaseMin fixedpoint.Rate

	// EndImbalance is BinancePrints − BybitPrints at close. Non-zero means
	// the window closed inside a phase gap (or across a hole in one leg's
	// history) — reported, not refused, because whether that voids the
	// episode is the procedure commit's rule, not this ledger's.
	EndImbalance int
}

// WindowAccrual prices one window from the two legs' settled series. Each
// series must be strictly ascending inside the window — a duplicated
// instant on one leg is refused (ErrDuplicateSettlement), because accrual
// truth with a doubled settlement has no honest reading. Prints AT OpenAt
// are excluded (the trigger is observed, not collected); prints AT CloseAt
// are included (the position is still open when the exit print settles).
func WindowAccrual(w Window, binance, bybit []Settlement) (WindowLedger, error) {
	if err := w.Validate(); err != nil {
		return WindowLedger{}, err
	}
	bn, err := windowSlice("binance", binance, w)
	if err != nil {
		return WindowLedger{}, err
	}
	by, err := windowSlice("bybit", bybit, w)
	if err != nil {
		return WindowLedger{}, err
	}

	led := WindowLedger{
		BinancePrints:  len(bn),
		BybitPrints:    len(by),
		BinanceAccrued: fixedpoint.ZeroRate(),
		BybitAccrued:   fixedpoint.ZeroRate(),
		Net:            fixedpoint.ZeroRate(),
		PhaseMax:       fixedpoint.ZeroRate(),
		PhaseMin:       fixedpoint.ZeroRate(),
		EndImbalance:   len(bn) - len(by),
	}

	// Merge the two legs in time order, processing every print that shares
	// one instant — across legs and within a leg's grid neighbour on the
	// other side — as ONE atomic event before the phase term is read.
	i, j := 0, 0
	balance := 0                    // binance prints minus bybit prints so far
	anchor := fixedpoint.ZeroRate() // Net at the last count-balanced event
	for i < len(bn) || j < len(by) {
		var at time.Time
		switch {
		case i >= len(bn):
			at = by[j].At
		case j >= len(by):
			at = bn[i].At
		case bn[i].At.Before(by[j].At):
			at = bn[i].At
		default:
			at = by[j].At
		}

		// At most ONE print per leg can share an instant — windowSlice
		// refused duplicates — so an event is one optional print from each
		// leg, processed together before the phase term is read.
		if i < len(bn) && bn[i].At.Equal(at) {
			led.BinanceAccrued, err = led.BinanceAccrued.Add(bn[i].Rate)
			if err != nil {
				return WindowLedger{}, fmt.Errorf("binance accrual at %s: %w", at.Format(time.RFC3339Nano), err)
			}
			led.Net, err = addSigned(led.Net, bn[i].Rate, w.Sign)
			if err != nil {
				return WindowLedger{}, fmt.Errorf("net at %s: %w", at.Format(time.RFC3339Nano), err)
			}
			balance++
			i++
		}
		if j < len(by) && by[j].At.Equal(at) {
			led.BybitAccrued, err = led.BybitAccrued.Add(by[j].Rate)
			if err != nil {
				return WindowLedger{}, fmt.Errorf("bybit accrual at %s: %w", at.Format(time.RFC3339Nano), err)
			}
			led.Net, err = addSigned(led.Net, by[j].Rate, -w.Sign)
			if err != nil {
				return WindowLedger{}, fmt.Errorf("net at %s: %w", at.Format(time.RFC3339Nano), err)
			}
			balance--
			j++
		}

		if balance == 0 {
			anchor = led.Net
			continue
		}
		unhedged, err := led.Net.Sub(anchor)
		if err != nil {
			return WindowLedger{}, fmt.Errorf("phase term at %s: %w", at.Format(time.RFC3339Nano), err)
		}
		if c, err := unhedged.Cmp(led.PhaseMax); err != nil {
			return WindowLedger{}, err
		} else if c > 0 {
			led.PhaseMax = unhedged
		}
		if c, err := unhedged.Cmp(led.PhaseMin); err != nil {
			return WindowLedger{}, err
		} else if c < 0 {
			led.PhaseMin = unhedged
		}
	}
	return led, nil
}

// windowSlice selects a leg's prints inside (OpenAt, CloseAt], validating
// order, rates, and the no-duplicate rule as it goes. Prints outside the
// window are not validated beyond order — the ledger prices a window, it
// does not audit a history.
func windowSlice(leg string, rows []Settlement, w Window) ([]Settlement, error) {
	var out []Settlement
	for k, r := range rows {
		if k > 0 && r.At.Before(rows[k-1].At) {
			return nil, fmt.Errorf("%s row %d at %s after %s: %w",
				leg, k, r.At.Format(time.RFC3339Nano), rows[k-1].At.Format(time.RFC3339Nano), ErrSeriesDisordered)
		}
		if !r.At.After(w.OpenAt) || r.At.After(w.CloseAt) {
			continue
		}
		if err := r.Rate.Validate(); err != nil {
			return nil, fmt.Errorf("%s row %d at %s: %w", leg, k, r.At.Format(time.RFC3339Nano), err)
		}
		if len(out) > 0 && r.At.Equal(out[len(out)-1].At) {
			return nil, fmt.Errorf("%s at %s: %w", leg, r.At.Format(time.RFC3339Nano), ErrDuplicateSettlement)
		}
		out = append(out, r)
	}
	return out, nil
}

// addSigned adds rate (sign +1) or its negation (sign -1) to acc.
func addSigned(acc, rate fixedpoint.Rate, sign int) (fixedpoint.Rate, error) {
	if sign == 1 {
		return acc.Add(rate)
	}
	neg, err := rate.Neg()
	if err != nil {
		return fixedpoint.Rate{}, err
	}
	return acc.Add(neg)
}
