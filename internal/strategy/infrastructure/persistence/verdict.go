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
	// PerSymbol so a verdict can state its own denominator's provenance.
	Excluded []string

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

	out := Verdict{
		TotalSymbols: len(table.Rows), Excluded: table.Excluded,
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
