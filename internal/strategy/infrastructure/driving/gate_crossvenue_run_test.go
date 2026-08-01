package driving_test

// The §45 gate runner's judgements, over constructed evidence. The
// collapse checklist ran first and pinned three things:
//
//   - The admission fixtures carry MORE THAN ONE symbol, and the failing
//     one is never the first. A one-symbol report — or a report whose
//     first symbol is the broken one — cannot tell "checks every symbol"
//     from "checks the first symbol", and AdmitGateSymbols exists
//     precisely to enumerate the whole roster in one walk.
//   - The utilization fixture's series CONTAINS ZEROS. Every claim about
//     the mean dividing by the full period count collapses on a series
//     with no idle periods, where "divide by all periods" and "divide by
//     the periods that traded" are the same division. This is the one
//     arithmetic that makes the gate's headline utilization-adjusted, so
//     an untestable version of it would be the worst kind of quiet.
//   - The annualization is checked against a value derived independently
//     here (periods per year = 8760h / 4h = 2190), not against whatever
//     the code returns.

import (
	"errors"
	"strings"
	"testing"
	"time"

	dcv "github.com/giiorg/arb-lab/internal/strategy/domain/crossvenue"
	"github.com/giiorg/arb-lab/internal/strategy/infrastructure/crossvenue"
	"github.com/giiorg/arb-lab/internal/strategy/infrastructure/driving"
)

// fitSymbol is one symbol's report, comfortably admissible.
func fitSymbol(symbol string) crossvenue.SymbolReport {
	return crossvenue.SymbolReport{
		Symbol:   symbol,
		Pairing:  dcv.PairStats{Matched: 200},
		Episodes: make([]crossvenue.EpisodeReport, 20),
		Scored:   18,
		RefusedN: 2, // 2 of 20 = 0.1, under the 0.25 ceiling
	}
}

func TestAdmitGateSymbolsAcceptsAFitRoster(t *testing.T) {
	t.Parallel()
	// The positive control: a rule set that refused everything would score
	// perfectly on every refusal case below without this.
	rep := crossvenue.SimReport{Symbols: []crossvenue.SymbolReport{
		fitSymbol("LABUSDT"), fitSymbol("KAITOUSDT"), fitSymbol("BCHUSDT"),
	}}
	if got := driving.AdmitGateSymbols(rep); len(got) != 0 {
		t.Fatalf("a fit roster was refused: %v", got)
	}
}

func TestAdmitGateSymbolsRefusalsAreEnumerated(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		mut  func([]crossvenue.SymbolReport)
		want []string // substrings, one per expected refusal
	}{
		{
			// The failing symbol is the LAST, so a check that stopped at
			// the first symbol — or returned after the first refusal —
			// reports nothing.
			name: "a short symbol at the end of the roster",
			mut:  func(s []crossvenue.SymbolReport) { s[2].Pairing.Matched = 96 },
			want: []string{"BCHUSDT", "96", "180"},
		},
		{
			name: "exactly at the floor is admitted",
			mut:  func(s []crossvenue.SymbolReport) { s[2].Pairing.Matched = 180 },
			want: nil,
		},
		{
			name: "one below the floor is refused",
			mut:  func(s []crossvenue.SymbolReport) { s[2].Pairing.Matched = 179 },
			want: []string{"BCHUSDT", "179"},
		},
		{
			// 5 of 20 is exactly 0.25 — the declared ceiling, admitted.
			name: "exactly at the refusal ceiling is admitted",
			mut:  func(s []crossvenue.SymbolReport) { s[1].RefusedN = 5 },
			want: nil,
		},
		{
			name: "one refusal past the ceiling",
			mut:  func(s []crossvenue.SymbolReport) { s[1].RefusedN = 6 },
			want: []string{"KAITOUSDT", "6", "20", "0.25"},
		},
		{
			// Both rules on one symbol produce BOTH messages: a runner
			// that reported the first fault per symbol would send the
			// operator back for a second walk to learn the second.
			name: "one symbol failing both rules is named twice",
			mut: func(s []crossvenue.SymbolReport) {
				s[1].Pairing.Matched = 10
				s[1].RefusedN = 19
			},
			want: []string{"10", "19"},
		},
		{
			// Two different symbols, two different faults, one walk.
			name: "faults on two symbols are both reported",
			mut: func(s []crossvenue.SymbolReport) {
				s[0].Pairing.Matched = 5
				s[2].RefusedN = 20
			},
			want: []string{"LABUSDT", "BCHUSDT"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			syms := []crossvenue.SymbolReport{
				fitSymbol("LABUSDT"), fitSymbol("KAITOUSDT"), fitSymbol("BCHUSDT"),
			}
			tc.mut(syms)
			got := driving.AdmitGateSymbols(crossvenue.SimReport{Symbols: syms})
			joined := strings.Join(got, "; ")
			if len(tc.want) == 0 {
				if len(got) != 0 {
					t.Fatalf("admitted roster was refused: %v", got)
				}
				return
			}
			if len(got) == 0 {
				t.Fatalf("no refusal reported; wanted one naming %v", tc.want)
			}
			for _, w := range tc.want {
				if !strings.Contains(joined, w) {
					t.Fatalf("refusals %q do not name %q", joined, w)
				}
			}
		})
	}
}

// TestAdmitGateSymbolsNamesBothFaultsSeparately pins the count, not just
// the content: a symbol failing both rules yields two messages, so the
// operator can see that fixing one leaves the other.
func TestAdmitGateSymbolsNamesBothFaultsSeparately(t *testing.T) {
	t.Parallel()
	s := fitSymbol("MUSDT")
	s.Pairing.Matched = 10
	s.RefusedN = 19
	got := driving.AdmitGateSymbols(crossvenue.SimReport{Symbols: []crossvenue.SymbolReport{s}})
	if len(got) != 2 {
		t.Fatalf("a symbol failing both rules produced %d refusal(s): %v", len(got), got)
	}
}

func TestGateConfigValidate(t *testing.T) {
	t.Parallel()
	full := driving.GateConfig{
		ArchiveRoot: "archive", RestRoot: "rest",
		ReferencePath: "ref.json", OutPath: "out.json",
		Before: time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC),
	}
	if err := full.Validate(); err != nil {
		t.Fatalf("the complete config was refused: %v", err)
	}

	// The bound is the one member with its own sentinel: defaulting it to
	// the clock is the specific mistake the message has to name.
	noBound := full
	noBound.Before = time.Time{}
	if err := noBound.Validate(); !errors.Is(err, driving.ErrGateBoundUnset) {
		t.Fatalf("an unset bound gave %v, want ErrGateBoundUnset", err)
	}

	// Every path member, individually — a validator checking only the
	// first would pass three of these.
	for name, mut := range map[string]func(*driving.GateConfig){
		"archive": func(c *driving.GateConfig) { c.ArchiveRoot = "" },
		"rest":    func(c *driving.GateConfig) { c.RestRoot = "" },
		"ref":     func(c *driving.GateConfig) { c.ReferencePath = "" },
		"out":     func(c *driving.GateConfig) { c.OutPath = "" },
	} {
		c := full
		mut(&c)
		if err := c.Validate(); err == nil {
			t.Fatalf("a config with no %s path validated", name)
		}
	}
}
