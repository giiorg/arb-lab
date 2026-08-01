package crossvenue_test

// The gate's two admission rules. The collapse checklist ran BEFORE this
// battery and pinned four fixture parameters that would otherwise have
// made real claims unreachable:
//
//   - Every case varies the FLOOR and the CEILING across cases. A battery
//     that only ever passed 180 and "0.25" would pass identically against
//     an implementation that ignored its argument and used the roster's
//     constant — the parameter-is-wired claim is the one the procedure
//     commit rests on, and a single pinned value cannot see it.
//   - Every case varies the SYMBOL. One symbol everywhere makes "the
//     message names THIS symbol" and "the message names a hardcoded
//     symbol" the same assertion.
//   - The ceiling's exactly-at case uses counts whose fraction is exactly
//     representable (2 of 8 against 0.25), so "exceeds" and "reaches" are
//     distinguishable rather than separated by a rounding step.
//   - One ceiling case uses 1 of 3 — a fraction NOT representable at scale
//     9 — against a ceiling of 0.333333333 and again against 0.333333334.
//     Those two answers differ, and they differ only if the comparison is
//     exact; an implementation that divided into a Rate first and rounded
//     gets one of them wrong whichever mode it picked. Without this pair
//     the cross-multiplication is untested.

import (
	"errors"
	"strings"
	"testing"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/strategy/domain/crossvenue"
)

func TestCheckEvidenceFloorBoundary(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		symbol  string
		matched int
		floor   int
		wantErr error
	}{
		// The §45f-1 boundary itself, both sides, at the committed floor.
		{"exactly at the floor is admitted", "LABUSDT", 180, 180, nil},
		{"one short of the floor refuses", "BEATUSDT", 179, 180, crossvenue.ErrEvidenceShort},
		{"comfortably over is admitted", "KAITOUSDT", 421, 180, nil},

		// The same boundary at a DIFFERENT floor: the rule reads its
		// argument, not the roster's number.
		{"exactly at a different floor", "MMTUSDT", 12, 12, nil},
		{"one short of a different floor", "MUSDT", 11, 12, crossvenue.ErrEvidenceShort},

		// Zero evidence is short of any admissible floor.
		{"no evidence at all", "SOONUSDT", 0, 1, crossvenue.ErrEvidenceShort},

		// Bounds the rule cannot apply.
		{"floor of zero admits everything", "ESPORTSUSDT", 0, 0, crossvenue.ErrGateBoundInvalid},
		{"negative floor", "BCHUSDT", 5, -1, crossvenue.ErrGateBoundInvalid},
		{"negative matched count", "LABUSDT", -1, 180, crossvenue.ErrGateBoundInvalid},
		{"unnamed symbol", "", 200, 180, crossvenue.ErrGateBoundInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := crossvenue.CheckEvidenceFloor(tc.symbol, tc.matched, tc.floor)
			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("CheckEvidenceFloor(%q,%d,%d) = %v, want admitted",
						tc.symbol, tc.matched, tc.floor, err)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("CheckEvidenceFloor(%q,%d,%d) = %v, want %v",
					tc.symbol, tc.matched, tc.floor, err, tc.wantErr)
			}
		})
	}
}

// TestCheckEvidenceFloorNamesTheSymbolAndCounts pins the "refused BY NAME"
// half of ruling §45f-1: the refusal is what the artifact records against
// the symbol, so a message that omits the symbol or the counts leaves the
// artifact unable to say which symbol was short, or by how much.
func TestCheckEvidenceFloorNamesTheSymbolAndCounts(t *testing.T) {
	t.Parallel()
	for _, sym := range []string{"LABUSDT", "BCHUSDT"} {
		err := crossvenue.CheckEvidenceFloor(sym, 42, 180)
		if err == nil {
			t.Fatalf("%s: 42 matched against floor 180 was admitted", sym)
		}
		msg := err.Error()
		for _, want := range []string{sym, "42", "180"} {
			if !strings.Contains(msg, want) {
				t.Fatalf("%s: refusal %q does not name %q", sym, msg, want)
			}
		}
	}
}

func TestCheckRefusalCeilingBoundary(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		symbol   string
		refused  int
		episodes int
		ceiling  string
		wantErr  error
	}{
		// Exactly AT the declared ceiling is admitted; the rule refuses
		// what EXCEEDS it. 2 of 8 is exactly 0.25 — no rounding step
		// separates the two sides of this boundary.
		{"exactly at the ceiling", "LABUSDT", 2, 8, "0.25", nil},
		{"one refusal past the ceiling", "BEATUSDT", 3, 8, "0.25", crossvenue.ErrRefusalCeiling},
		{"well under the ceiling", "KAITOUSDT", 1, 40, "0.25", nil},

		// The same boundary at a DIFFERENT ceiling: the rule reads its
		// argument. 2 of 8 is over 0.2 and under 0.5.
		{"same counts refused by a tighter ceiling", "MMTUSDT", 2, 8, "0.2", crossvenue.ErrRefusalCeiling},
		{"same counts admitted by a looser ceiling", "MUSDT", 2, 8, "0.5", nil},

		// The inexact fraction, both ways. 1/3 = 0.333... exceeds a
		// ceiling of 0.333333333 and does not exceed 0.333333334.
		{"inexact fraction just over", "SOONUSDT", 1, 3, "0.333333333", crossvenue.ErrRefusalCeiling},
		{"inexact fraction just under", "SOONUSDT", 1, 3, "0.333333334", nil},

		// A zero ceiling can fire, and does, on the first refusal.
		{"zero ceiling tolerates no refusal", "ESPORTSUSDT", 1, 20, "0", crossvenue.ErrRefusalCeiling},
		{"zero ceiling with no refusals", "ESPORTSUSDT", 0, 20, "0", nil},

		// Every episode refused is the case a ceiling exists to catch.
		{"every episode refused", "BCHUSDT", 9, 9, "0.25", crossvenue.ErrRefusalCeiling},

		// No episodes passes VACUOUSLY — the signal never triggered, which
		// is standing down, not thin evidence (that is the floor's job).
		{"no episodes at all", "LABUSDT", 0, 0, "0.25", nil},
		{"no episodes under a zero ceiling", "LABUSDT", 0, 0, "0", nil},

		// Bounds and counts the rule cannot apply.
		{"ceiling of exactly one can never fire", "BEATUSDT", 1, 4, "1", crossvenue.ErrGateBoundInvalid},
		{"ceiling above one", "BEATUSDT", 1, 4, "1.5", crossvenue.ErrGateBoundInvalid},
		{"negative ceiling", "KAITOUSDT", 0, 4, "-0.25", crossvenue.ErrGateBoundInvalid},
		{"more refusals than episodes", "MMTUSDT", 5, 4, "0.25", crossvenue.ErrGateBoundInvalid},
		{"negative refusal count", "MUSDT", -1, 4, "0.25", crossvenue.ErrGateBoundInvalid},
		{"negative episode count", "MUSDT", 0, -1, "0.25", crossvenue.ErrGateBoundInvalid},
		{"unnamed symbol", "", 1, 4, "0.25", crossvenue.ErrGateBoundInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := crossvenue.CheckRefusalCeiling(tc.symbol, tc.refused, tc.episodes, rate(t, tc.ceiling))
			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("CheckRefusalCeiling(%q,%d,%d,%s) = %v, want admitted",
						tc.symbol, tc.refused, tc.episodes, tc.ceiling, err)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("CheckRefusalCeiling(%q,%d,%d,%s) = %v, want %v",
					tc.symbol, tc.refused, tc.episodes, tc.ceiling, err, tc.wantErr)
			}
		})
	}
}

// TestCheckRefusalCeilingUnsetCeilingRefuses pins the zero-value rule: an
// unpopulated Rate must refuse rather than read as a ceiling of zero. The
// two are opposite in effect — a zero ceiling refuses on the first
// refusal, while a forgotten argument reading as zero would refuse EVERY
// symbol that ever refused an episode, which looks like a working gate.
func TestCheckRefusalCeilingUnsetCeilingRefuses(t *testing.T) {
	t.Parallel()
	var unset fixedpoint.Rate
	err := crossvenue.CheckRefusalCeiling("LABUSDT", 0, 10, unset)
	if err == nil {
		t.Fatal("an unset ceiling was accepted")
	}
	if errors.Is(err, crossvenue.ErrRefusalCeiling) {
		t.Fatalf("unset ceiling reported as a refusal verdict, not an unset bound: %v", err)
	}
}

// TestCheckRefusalCeilingNamesTheSymbolAndCounts is the ceiling's half of
// the by-name rule (§45n-3 borrows §45f-1's mould).
func TestCheckRefusalCeilingNamesTheSymbolAndCounts(t *testing.T) {
	t.Parallel()
	for _, sym := range []string{"KAITOUSDT", "MMTUSDT"} {
		err := crossvenue.CheckRefusalCeiling(sym, 7, 12, rate(t, "0.25"))
		if err == nil {
			t.Fatalf("%s: 7 of 12 refused against a 0.25 ceiling was admitted", sym)
		}
		msg := err.Error()
		for _, want := range []string{sym, "7", "12", "0.25"} {
			if !strings.Contains(msg, want) {
				t.Fatalf("%s: refusal %q does not name %q", sym, msg, want)
			}
		}
	}
}
