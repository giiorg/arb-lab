package persistence

import (
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// daysPerYear is the annualisation factor summarise.py:111 uses. It is a
// calendar convention, not a claim about any venue.
const daysPerYear = 365

// SymbolReturn is one symbol's realised economics at one coordinate.
type SymbolReturn struct {
	Symbol       string
	Shortlist    bool
	Episodes     int
	Days         int
	ModalMinutes int
	First        time.Time
	Last         time.Time

	// Net is the total after cost, in the leg notional's asset, for a position
	// of one LegNotional per leg.
	Net fixedpoint.Money

	// Annual is Net over Capital, annualised across Days — as a RATIO. The
	// artifact's annual_pct_on_2000 is this times 100.
	Annual fixedpoint.Rate
}

// SymbolReturns is the per-symbol table, and Excluded is why it is shorter than
// the file's symbol count.
type SymbolReturns struct {
	// Rows are sorted by Annual descending, ties broken by Symbol ascending.
	//
	// The artifact sorts by the ROUNDED percentage at cost 0.0021 for every
	// block; this sorts by the unrounded ratio at the coordinate asked for.
	// Tests compare by symbol name, never by index, because those two orders
	// are not the same claim.
	Rows []SymbolReturn

	// Excluded names the symbols with no episodes at this entry level, sorted.
	//
	// summarise.py:99 writes `if not e: continue` and records nothing. Measured
	// over the committed file the excluded counts are 0 / 1 / 5 / 12 across the
	// four declared levels, which is why verdict.total_symbols reads 56 against
	// a header declaring symbols_eligible 61 — F60. The denominator is a
	// function of the entry level, and no field in the artifact says so. This
	// one does.
	Excluded []string

	At SizeAt
}

// PerSymbol computes each symbol's net and annualised return at one coordinate.
func (s Set) PerSymbol(at SizeAt) (SymbolReturns, error) {
	if err := at.Validate(); err != nil {
		return SymbolReturns{}, err
	}
	rows, err := s.at(at.EntryAt)
	if err != nil {
		return SymbolReturns{}, err
	}

	out := SymbolReturns{At: at}
	for _, r := range rows {
		if len(r.episodes) == 0 {
			out.Excluded = append(out.Excluded, r.symbol.name)
			continue
		}
		row, err := symbolRow(r, at)
		if err != nil {
			return SymbolReturns{}, err
		}
		out.Rows = append(out.Rows, row)
	}

	sort.Strings(out.Excluded)
	sort.Slice(out.Rows, func(i, j int) bool {
		cmp, err := out.Rows[i].Annual.Cmp(out.Rows[j].Annual)
		if err != nil || cmp == 0 {
			return out.Rows[i].Symbol < out.Rows[j].Symbol
		}
		return cmp > 0
	})
	return out, nil
}

func symbolRow(r symbolLevel, at SizeAt) (SymbolReturn, error) {
	// time.Duration is an int64 count of nanoseconds, so dividing it by
	// another Duration is integer division — no float64 anywhere in this
	// line. It truncates toward zero exactly as Python's timedelta.days does
	// for a non-negative span; a negative span (last before first) makes days
	// negative, which the guard below refuses.
	days := int(r.symbol.last.Sub(r.symbol.first) / (24 * time.Hour))
	if days < 1 {
		// summarise.py:33 writes max(1, days) here, which inflates an
		// annualised return over a day that did not happen and fails nothing.
		return SymbolReturn{}, fmt.Errorf("%w: %s spans %s to %s",
			ErrSpanDegenerate, r.symbol.name,
			r.symbol.first.Format(time.RFC3339), r.symbol.last.Format(time.RFC3339))
	}

	total := fixedpoint.ZeroMoney(at.LegNotional.Asset())
	costPerEpisode, err := at.LegNotional.Mul(at.Cost, fixedpoint.RoundHalfEven)
	if err != nil {
		return SymbolReturn{}, fmt.Errorf("persistence: cost per episode: %w", err)
	}
	for _, e := range r.episodes {
		gross, err := at.LegNotional.Mul(e.Accrued, fixedpoint.RoundHalfEven)
		if err != nil {
			return SymbolReturn{}, fmt.Errorf("persistence: %s gross: %w", r.symbol.name, err)
		}
		net, err := gross.Sub(costPerEpisode)
		if err != nil {
			return SymbolReturn{}, fmt.Errorf("persistence: %s net: %w", r.symbol.name, err)
		}
		total, err = total.Add(net)
		if err != nil {
			return SymbolReturn{}, fmt.Errorf("persistence: %s total: %w", r.symbol.name, err)
		}
	}

	ratio, err := total.Div(at.Capital, fixedpoint.RoundHalfEven)
	if err != nil {
		return SymbolReturn{}, fmt.Errorf("persistence: %s return on capital: %w", r.symbol.name, err)
	}
	year, err := fixedpoint.ParseRate(strconv.Itoa(daysPerYear))
	if err != nil {
		return SymbolReturn{}, fmt.Errorf("persistence: days per year: %w", err)
	}
	scaled, err := ratio.Mul(year, fixedpoint.RoundHalfEven)
	if err != nil {
		return SymbolReturn{}, fmt.Errorf("persistence: %s annualise: %w", r.symbol.name, err)
	}
	span, err := fixedpoint.ParseRate(strconv.Itoa(days))
	if err != nil {
		return SymbolReturn{}, fmt.Errorf("persistence: %s span %d: %w", r.symbol.name, days, err)
	}
	annual, err := scaled.Div(span, fixedpoint.RoundHalfEven)
	if err != nil {
		return SymbolReturn{}, fmt.Errorf("persistence: %s annual: %w", r.symbol.name, err)
	}

	return SymbolReturn{
		Symbol: r.symbol.name, Shortlist: r.symbol.shortlist,
		Episodes: len(r.episodes), Days: days, ModalMinutes: r.symbol.modalMinutes,
		First: r.symbol.first, Last: r.symbol.last,
		Net: total, Annual: annual,
	}, nil
}
