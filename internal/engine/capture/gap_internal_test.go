package capture

import (
	"encoding/json"
	"slices"
	"testing"
)

// declaredCauses is the roster this package claims to have. It is written out by
// hand rather than derived, which is the point: a cause added to gap.go and not
// added here fails the walk below, and a cause added here and not to gap.go fails
// Validate. Neither list can drift alone.
var declaredCauses = []Cause{
	CauseReconnect, CauseOverflow, CauseTornTail,
	CauseProcessRestart, CauseNoFirstData, CauseClockStep,
}

func TestCauseValidateAcceptsEveryDeclaredCause(t *testing.T) {
	for _, c := range declaredCauses {
		if err := c.Validate(); err != nil {
			t.Errorf("the declared cause %q was refused: %v", c, err)
		}
		if c.String() != string(c) {
			t.Errorf("String() = %q for %q", c.String(), string(c))
		}
	}
	// The two refusals, split because they come from different mistakes.
	if err := CauseUnspecified.Validate(); !errorIs(err, ErrCauseUnspecified) {
		t.Errorf("the zero cause = %v, want ErrCauseUnspecified", err)
	}
	for _, c := range []Cause{"reconnected", "RECONNECT", " reconnect", "reconnect ", "overflow\n"} {
		if err := c.Validate(); !errorIs(err, ErrCauseUnknown) {
			t.Errorf("Validate(%q) = %v, want ErrCauseUnknown", string(c), err)
		}
	}
}

// TestCauseHasRangeIsTrueForExactlyOne pins which causes carry a sequence range,
// because Gap.Validate's demand for one keys on it.
//
// Asserting the whole partition rather than the one true case is what stops the
// predicate from being quietly widened: a cause that gained a range would
// otherwise start requiring From/To with no test saying so, and every existing
// caller of Gap would begin failing at a call site that had not changed.
func TestCauseHasRangeIsTrueForExactlyOne(t *testing.T) {
	var withRange []Cause
	for _, c := range declaredCauses {
		if c.hasRange() {
			withRange = append(withRange, c)
		}
	}
	if want := []Cause{CauseOverflow}; !slices.Equal(withRange, want) {
		t.Errorf("causes carrying a sequence range = %v, want %v", withRange, want)
	}
}

func TestGapValidate(t *testing.T) {
	// The positive controls come first: without them a Validate that refused
	// everything would satisfy the whole rejection table.
	for _, g := range []Gap{
		{Cause: CauseOverflow, From: 7, To: 7, Count: 1},
		{Cause: CauseOverflow, From: 7, To: 11, Count: 5},
		{Cause: CauseTornTail, Bytes: 143},
		{Cause: CauseTornTail}, // a torn tail of zero bytes is not reachable, but is not malformed
		{Cause: CauseProcessRestart},
		{Cause: CauseReconnect, Detail: "socket closed by peer"},
		// A cause that does not REQUIRE a range may still carry one — a
		// reconnect that knows which sequences it lost is more useful than one
		// that does not, and hasRange only governs what is demanded.
		{Cause: CauseReconnect, From: 3, To: 6, Count: 4},
		{Cause: CauseNoFirstData},
		{Cause: CauseClockStep},
	} {
		if err := g.Validate(); err != nil {
			t.Errorf("the well-formed gap %+v was refused: %v", g, err)
		}
	}

	for _, tc := range []struct {
		name string
		gap  Gap
		want error
	}{
		{"no cause", Gap{From: 1, To: 2, Count: 2}, ErrCauseUnspecified},
		{"unknown cause", Gap{Cause: "wat"}, ErrCauseUnknown},
		{
			// The invariant is a property of drop-newest, not bookkeeping: a
			// gap whose count disagrees with its range is evidence the queue
			// dropped in some other pattern, which would mean the sequence
			// holes elsewhere in the file no longer mean what §32d says.
			"count disagrees with the range",
			Gap{Cause: CauseOverflow, From: 3, To: 6, Count: 3},
			ErrGapRange,
		},
		{"count of one over a range of four", Gap{Cause: CauseOverflow, From: 3, To: 6, Count: 1}, ErrGapRange},
		{"range runs backwards", Gap{Cause: CauseOverflow, From: 6, To: 3, Count: 4}, ErrGapRange},
		{
			// Half-populated: From and To set, Count forgotten. Asking "did the
			// caller fill ANY range field" rather than "is Count > 0" is what
			// makes this an error instead of a silently ignored range.
			"range without a count",
			Gap{Cause: CauseOverflow, From: 3, To: 6},
			ErrGapRange,
		},
		{"count without a range", Gap{Cause: CauseOverflow, Count: 4}, ErrGapRange},
		{
			// The isolating fixture, found by a surviving mutant. Every other
			// half-populated row uses CauseOverflow, where hasRange() refuses
			// it by a SECOND route — so they show that something objected
			// rather than that hasRangeFields did. On a cause that requires no
			// range, hasRangeFields is the only thing standing between a
			// caller who forgot Count and a gap record describing a hole with
			// no size. Same shape as §3.6's singlewriter probe note.
			"a non-range cause with a half-populated range",
			Gap{Cause: CauseReconnect, From: 3, To: 6},
			ErrGapRange,
		},
		{"a non-range cause with a count and no range", Gap{Cause: CauseTornTail, Count: 4}, ErrGapRange},
		{
			// A cause that requires a range and carries none. This is the arm
			// hasRange exists for, and it is the one a fixture without an
			// overflow case would never reach.
			"overflow with nothing populated",
			Gap{Cause: CauseOverflow},
			ErrGapRange,
		},
		{"negative bytes", Gap{Cause: CauseTornTail, Bytes: -1}, ErrGapRange},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.gap.Validate(); !errorIs(err, tc.want) {
				t.Errorf("Validate(%+v) = %v, want %v", tc.gap, err, tc.want)
			}
		})
	}
}

// TestGapPayloadRoundTripsAndOmitsEmptyFields checks the two things the JSON
// shape is for: it survives the archive, and it does not spend bytes saying
// nothing.
func TestGapPayloadRoundTripsAndOmitsEmptyFields(t *testing.T) {
	full := Gap{Cause: CauseOverflow, From: 3, To: 6, Count: 4, Detail: "queue full"}
	body, err := full.MarshalPayload()
	if err != nil {
		t.Fatalf("MarshalPayload: %v", err)
	}
	var back Gap
	if err := json.Unmarshal(body, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back != full {
		t.Errorf("round trip gave %+v, want %+v", back, full)
	}

	// Determinism: the same gap must produce byte-identical payloads, or a
	// golden test and a content digest stop meaning anything.
	again, err := full.MarshalPayload()
	if err != nil {
		t.Fatalf("MarshalPayload (second): %v", err)
	}
	if string(again) != string(body) {
		t.Errorf("two encodings of one gap differ:\n%s\n%s", body, again)
	}

	// A minimal gap carries only its cause.
	minimal, err := Gap{Cause: CauseProcessRestart}.MarshalPayload()
	if err != nil {
		t.Fatalf("MarshalPayload(minimal): %v", err)
	}
	if got, want := string(minimal), `{"cause":"process_restart"}`; got != want {
		t.Errorf("minimal payload = %s, want %s", got, want)
	}

	// And an invalid gap never reaches the archive.
	if _, err := (Gap{Cause: CauseOverflow, From: 3, To: 6, Count: 99}).MarshalPayload(); !errorIs(err, ErrGapRange) {
		t.Errorf("MarshalPayload of an inconsistent gap = %v, want ErrGapRange", err)
	}
}
