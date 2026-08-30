package persistence

import (
	"fmt"
	"sort"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// Verdict is the summary answer at one coordinate, against one hurdle.
//
// It is deliberately not called "the kill-gate verdict": build-spec §12.2's
// gate is a comparison against a dated, haircut-adjusted benchmark, and the
// hurdle this takes is whatever the caller supplies. Verdict carries the hurdle
// it used, with the hurdle's own class and source, so a rendered figure and its
// comparand travel together.
type Verdict struct {
	// MedianAnnual is the middle element of the annual returns, ascending, at
	// index len/2 — replicating summarise.py's v[len(v)//2] exactly. For an
	// even population that is the UPPER of the two middle values, not their
	// average. It is a ratio.
	MedianAnnual fixedpoint.Rate

	// PositiveSymbols counts symbols whose NET is strictly positive. This is
	// deliberately the raw net rather than the rounded percentage: three
	// symbols earn a couple of dollars over four years, which is positive and
	// renders as +0.0%/yr.
	PositiveSymbols int

	// BeatHurdle counts symbols whose annual RATIO is strictly greater than the
	// hurdle's. The comparison is on the stored ratio, never on a rounded
	// percentage — and the committed file cannot tell the two apart, so
	// testdata/hurdle-boundary.json is what makes this claim checkable.
	BeatHurdle int

	// TotalSymbols is the population these counts are out of — the level's
	// surviving symbols, not the file's symbol count.
	TotalSymbols int

	// Excluded names the symbols this level dropped, carried through from
	// PerSymbol so a verdict can state its own denominator's provenance. Each
	// entry carries its own censored count, so a symbol dropped because every
	// record it produced was right-censored is distinguishable from one that
	// never entered at all.
	//
	// It is a COPY of PerSymbol's slice, not an alias. The copy is DEFENSIVE
	// and no test can currently kill it: Verdict calls PerSymbol itself, so the
	// table whose slice would be aliased never leaves this function and no
	// caller can hold both ends. Removing the copy is an equivalent mutant —
	// measured, SURVIVED, and diagnosed rather than left as a test gap. It
	// stays because the alias becomes reachable the moment anything hands
	// Verdict a table it did not build, and a shared backing array is invisible
	// in both types' signatures. What it must NOT be credited with is a
	// protection something checks today.
	//
	// There is deliberately no episode-level Censored field on Verdict. Every
	// other count here — PositiveSymbols, BeatHurdle, TotalSymbols — is over
	// SYMBOLS, and an episode count sitting beside them would be a figure whose
	// base is not the one its neighbours share: exactly the "value travelled
	// without its base" defect this package exists to make unrepresentable. The
	// censoring fact AT a verdict's own granularity is a symbol dropped for
	// having no runs, and that is what Excluded carries. A caller wanting the
	// episode-level count asks NetPerEpisode or RunLength at the same
	// coordinate.
	Excluded []Exclusion

	Hurdle Hurdle
	At     SizeAt
}

// Verdict folds the per-symbol table into counts against a hurdle.
func (s Set) Verdict(at SizeAt, h Hurdle) (Verdict, error) {
	if err := h.Validate(); err != nil {
		return Verdict{}, err
	}
	table, err := s.PerSymbol(at)
	if err != nil {
		return Verdict{}, err
	}

	excluded := make([]Exclusion, len(table.Excluded))
	copy(excluded, table.Excluded)
	out := Verdict{
		TotalSymbols: len(table.Rows), Excluded: excluded,
		Hurdle: h, At: at, MedianAnnual: fixedpoint.ZeroRate(),
	}
	if len(table.Rows) == 0 {
		return out, nil
	}

	annuals := make([]fixedpoint.Rate, 0, len(table.Rows))
	for _, r := range table.Rows {
		annuals = append(annuals, r.Annual)

		sign, err := r.Net.Sign()
		if err != nil {
			return Verdict{}, fmt.Errorf("persistence: %s net sign: %w", r.Symbol, err)
		}
		if sign > 0 {
			out.PositiveSymbols++
		}
		cmp, err := r.Annual.Cmp(h.Annual)
		if err != nil {
			return Verdict{}, fmt.Errorf("persistence: %s hurdle comparison: %w", r.Symbol, err)
		}
		if cmp > 0 {
			out.BeatHurdle++
		}
	}

	var sortErr error
	sort.Slice(annuals, func(i, j int) bool {
		cmp, err := annuals[i].Cmp(annuals[j])
		if err != nil {
			sortErr = err
			return false
		}
		return cmp < 0
	})
	if sortErr != nil {
		return Verdict{}, fmt.Errorf("persistence: sort annual returns: %w", sortErr)
	}
	out.MedianAnnual = annuals[len(annuals)/2]
	return out, nil
}
