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

	// Censored is how many of this symbol's entries at this level triggered
	// with no settlement after them, and so are in neither Episodes nor Net.
	//
	// It is per-row rather than per-table because the population it shrinks is
	// per-row: summarise.py counts these in its own `episodes` for the symbol, so a row
	// here and the artifact's row for the same symbol are answers over
	// different populations, and Episodes + Censored is the artifact's count.
	// A symbol whose records were ALL censored has no row at all — it appears
	// in Excluded, with its own count, so the level's censoring is fully
	// accounted between the two.
	Censored int

	// Net is the total after cost, in the leg notional's asset, for a position
	// of one LegNotional per leg.
	Net fixedpoint.Money

	// Annual is Net over Capital, annualised across Days — as a RATIO. The
	// artifact's annual_pct_on_2000 is this times 100.
	Annual fixedpoint.Rate
}

// Exclusion is one symbol the level dropped, and WHY it was dropped.
//
// The two reasons are different facts and this type keeps them apart. Censored
// == 0 means the symbol has no records at all at this entry level — the level
// never triggered for it. Censored > 0 means it triggered and every record it
// produced was right-censored (F103), so it has records and no runs. Filing
// both as a bare name would make "never entered" and "entered, never settled"
// indistinguishable, which is the same class of loss the coordinates on these
// results exist to prevent.
type Exclusion struct {
	Symbol   string
	Censored int
}

// SymbolReturns is the per-symbol table, and Excluded is why it is shorter than
// the file's symbol count.
type SymbolReturns struct {
	// Rows are sorted by Annual descending, ties broken by Symbol ascending.
	//
	// The artifact sorts by the ROUNDED percentage at cost 0.0021 for every
	// block; this sorts by the unrounded ratio at the coordinate asked for, so
	// the oracle compares rows by symbol name and never by index — those two
	// orders are not the same claim. The order itself is asserted as an
	// INVARIANT over the rows instead (persymbol_test.go), with a fixture
	// carrying an exact tie, because all 56 annual ratios in the committed file
	// are distinct and the tiebreak has nothing to break against it.
	Rows []SymbolReturn

	// Excluded names the symbols with no RUNS at this entry level, sorted by
	// symbol, each carrying its own censored count so the two reasons for a
	// drop stay apart (see Exclusion).
	//
	// summarise.py:99 writes `if not e: continue` and records nothing. Measured
	// over the committed file the excluded counts are 0 / 1 / 5 / 12 across the
	// four declared levels, which is why verdict.total_symbols reads 56 against
	// a header declaring symbols_eligible 61 — F60. The denominator is a
	// function of the entry level, and no field in the artifact says so. This
	// one does.
	Excluded []Exclusion

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
			out.Excluded = append(out.Excluded, Exclusion{
				Symbol: r.symbol.name, Censored: len(r.censored),
			})
			continue
		}
		row, err := symbolRow(r, at)
		if err != nil {
			return SymbolReturns{}, err
		}
		out.Rows = append(out.Rows, row)
	}

	sort.Slice(out.Excluded, func(i, j int) bool {
		return out.Excluded[i].Symbol < out.Excluded[j].Symbol
	})
	// The Cmp error is CAPTURED, never folded into the tiebreak. Returning the
	// name comparison on an error would answer a different question silently —
	// a mismatched asset tag would render as a symbol-ordered table that no
	// caller could tell from a correctly ranked one. verdict.go's median sort
	// takes the same shape, and a claim checked in one place gets checked in
	// its mirror.
	var sortErr error
	sort.Slice(out.Rows, func(i, j int) bool {
		cmp, err := out.Rows[i].Annual.Cmp(out.Rows[j].Annual)
		if err != nil {
			sortErr = err
			return false
		}
		if cmp == 0 {
			return out.Rows[i].Symbol < out.Rows[j].Symbol
		}
		return cmp > 0
	})
	if sortErr != nil {
		return SymbolReturns{}, fmt.Errorf("persistence: sort per-symbol rows: %w", sortErr)
	}
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
		Episodes: len(r.episodes), Censored: len(r.censored),
		Days: days, ModalMinutes: r.symbol.modalMinutes,
		First: r.symbol.first, Last: r.symbol.last,
		Net: total, Annual: annual,
	}, nil
}
