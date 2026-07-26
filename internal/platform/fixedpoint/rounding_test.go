package fixedpoint_test

import (
	"errors"
	"testing"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// The zero value must be INVALID, for the same reason Money's is.
//
// Every operation that can lose precision takes a RoundingMode explicitly
// (§3.6 rule 4). If the zero value were a usable mode, a struct field or a
// variable nobody set would silently pick whichever mode happens to be declared
// first — a *policy* decision about money, made by omission.
func TestRoundingMode_ZeroValueIsInvalid(t *testing.T) {
	var mode fixedpoint.RoundingMode

	if mode != fixedpoint.RoundInvalid {
		t.Fatalf("the zero RoundingMode is %v; want RoundInvalid — the sentinel must be 0", mode)
	}
	err := mode.Validate()
	if err == nil {
		t.Fatal("RoundInvalid.Validate() returned nil; want an error")
	}
	if !errors.Is(err, fixedpoint.ErrRoundingMode) {
		t.Fatalf("RoundInvalid.Validate() = %v; want it to wrap ErrRoundingMode", err)
	}
}

// A value cast in from outside the declared set must be rejected too.
// RoundingMode is uint8, so `fixedpoint.RoundingMode(n)` compiles for any n;
// Validate is the only thing standing between that and a silent no-op branch.
func TestRoundingMode_UndeclaredValueIsInvalid(t *testing.T) {
	for _, n := range []uint8{9, 42, 255} {
		mode := fixedpoint.RoundingMode(n)
		err := mode.Validate()
		if err == nil {
			t.Errorf("RoundingMode(%d).Validate() returned nil; want an error", n)
			continue
		}
		if !errors.Is(err, fixedpoint.ErrRoundingMode) {
			t.Errorf("RoundingMode(%d).Validate() = %v; want it to wrap ErrRoundingMode", n, err)
		}
	}
}

// Enumerating the declared range and requiring every member to be *named here*
// is what makes this more than a restatement of the source.
//
// Insert a mode in the middle and the loop reaches it while the map does not,
// so this fails until someone states what the new mode is called. Append one
// past the end and the bound below stops matching, which fails the count check.
// Either way a new rounding policy cannot arrive unannounced.
func TestRoundingMode_DeclaredSet(t *testing.T) {
	named := map[fixedpoint.RoundingMode]string{
		fixedpoint.RoundExact:        "RoundExact",
		fixedpoint.RoundTowardZero:   "RoundTowardZero",
		fixedpoint.RoundAwayFromZero: "RoundAwayFromZero",
		fixedpoint.RoundFloor:        "RoundFloor",
		fixedpoint.RoundCeil:         "RoundCeil",
		fixedpoint.RoundHalfUp:       "RoundHalfUp",
		fixedpoint.RoundHalfEven:     "RoundHalfEven",
	}

	if got, want := len(named), 7; got != want {
		t.Fatalf("the table names %d modes; want %d valid modes plus RoundInvalid", got, want)
	}

	seen := map[string]bool{}
	for mode := fixedpoint.RoundInvalid + 1; ; mode++ {
		if mode.Validate() != nil {
			// First value past the declared range: the set ends here.
			if int(mode) != len(named)+1 {
				t.Fatalf("the declared range ends at %d; want it to end at %d, "+
					"so that every valid mode is named in this test", mode, len(named)+1)
			}
			break
		}
		name, ok := named[mode]
		if !ok {
			t.Fatalf("RoundingMode(%d) validates but is not named in this test — "+
				"a new rounding policy was added without saying what it is for", uint8(mode))
		}
		if got := mode.String(); got != name {
			t.Errorf("RoundingMode(%d).String() = %q; want %q", uint8(mode), got, name)
		}
		if seen[name] {
			t.Errorf("String() returned %q for more than one mode", name)
		}
		seen[name] = true
	}
}

// An undeclared mode must still print something a log reader can act on, rather
// than an empty string that reads as "no mode was involved".
func TestRoundingMode_StringOfUndeclared(t *testing.T) {
	if got, want := fixedpoint.RoundInvalid.String(), "RoundInvalid"; got != want {
		t.Errorf("RoundInvalid.String() = %q; want %q", got, want)
	}
	if got, want := fixedpoint.RoundingMode(200).String(), "RoundingMode(200)"; got != want {
		t.Errorf("RoundingMode(200).String() = %q; want %q", got, want)
	}
}
