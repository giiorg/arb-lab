package capture_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/giiorg/arb-lab/internal/engine/capture"
)

// TestRecordKindDeclaredSet pins the set and, more importantly, pins that the
// zero value is NOT one of them.
//
// RecordKind takes a second documented exception to §3.6 rule 8's string shape
// (see its doc), and the exception's justification is the zero value being an
// unusable sentinel. That matters more here than it does for events.Lane,
// because a RecordKind is read back off a DISK: a zero-filled region — a sparse
// hole, a page the filesystem allocated before the process died — must not
// decode as a valid record. Had RecordKindData been zero, the most common
// corruption pattern on disk would parse cleanly.
func TestRecordKindDeclaredSet(t *testing.T) {
	declared := []capture.RecordKind{
		capture.RecordKindOpen, capture.RecordKindData, capture.RecordKindAlive,
		capture.RecordKindGap, capture.RecordKindRest, capture.RecordKindClose,
		capture.RecordKindClockStep,
	}
	for _, k := range declared {
		if err := k.Validate(); err != nil {
			t.Errorf("declared %v: Validate() = %v, want nil", k, err)
		}
		if s := k.String(); !strings.HasPrefix(s, "RecordKind") || strings.Contains(s, "(") {
			t.Errorf("String() = %q, want a declared name", s)
		}
	}

	if err := capture.RecordKindUnspecified.Validate(); !errors.Is(err, capture.ErrKindUnspecified) {
		t.Errorf("RecordKindUnspecified.Validate() = %v, want %v — the zero value "+
			"must be unusable, which is the entire reason this enum is an integer "+
			"rather than rule 8's string shape", err, capture.ErrKindUnspecified)
	}

	// One past the last declared value. The check is `> maxRecordKind`, so this
	// is the case that catches a maxRecordKind left behind when a kind is added
	// — and it is the boundary at which `>` and `>=` differ, which is why the
	// last declared kind is asserted valid in the loop above rather than only
	// this one asserted invalid.
	past := capture.RecordKindClockStep + 1
	if err := past.Validate(); !errors.Is(err, capture.ErrKindUnknown) {
		t.Errorf("RecordKind(%d).Validate() = %v, want %v", uint8(past), err, capture.ErrKindUnknown)
	}
	if got := past.String(); !strings.Contains(got, "(") {
		t.Errorf("RecordKind(%d).String() = %q, want the numeric fallback", uint8(past), got)
	}
	if err := capture.RecordKind(255).Validate(); !errors.Is(err, capture.ErrKindUnknown) {
		t.Errorf("RecordKind(255).Validate() = %v, want %v", err, capture.ErrKindUnknown)
	}
}

// TestRecordKindStringCoversEveryDeclaredValue walks the whole declared range,
// sentinel included, and asserts none of them falls through to the numeric
// default.
//
// exhaustiveswitch (§3.6 rule 10) already makes a missing case a compile error,
// so this is not that guard — it is the one the gate cannot give: the switch
// could name every member and still return the wrong STRING for one, and the
// trailing numeric fallback is reachable for an undeclared value, which is the
// shape a mistyped case label produces.
func TestRecordKindStringCoversEveryDeclaredValue(t *testing.T) {
	seen := map[string]capture.RecordKind{}
	for k := capture.RecordKindUnspecified; k <= capture.RecordKindClockStep; k++ {
		got := k.String()
		if strings.Contains(got, "(") {
			t.Errorf("RecordKind(%d).String() = %q — a declared kind fell through "+
				"to the numeric default", uint8(k), got)
			continue
		}
		if prior, dup := seen[got]; dup {
			t.Errorf("RecordKind(%d) and RecordKind(%d) both print as %q; a case "+
				"label names the wrong member", uint8(prior), uint8(k), got)
		}
		seen[got] = k
	}
}
