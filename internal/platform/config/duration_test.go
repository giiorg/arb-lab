package config_test

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/config"
)

func TestDurationAcceptsStrings(t *testing.T) {
	tests := []struct {
		in   string
		want time.Duration
	}{
		{`"90s"`, 90 * time.Second},
		{`"5m"`, 5 * time.Minute},
		{`"1h30m"`, 90 * time.Minute},
		{`"250ms"`, 250 * time.Millisecond},
		{`"0"`, 0}, // time.ParseDuration special-cases a bare zero
		{`"0s"`, 0},
	}

	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			var d config.Duration
			if err := json.Unmarshal([]byte(tc.in), &d); err != nil {
				t.Fatalf("Unmarshal(%s) = %v", tc.in, err)
			}
			if d.Duration() != tc.want {
				t.Errorf("got %v, want %v", d.Duration(), tc.want)
			}
			if err := d.Validate(); err != nil {
				t.Errorf("Validate = %v", err)
			}
		})
	}
}

// TestDurationRejectsABareNumber is the point of the type.
//
// A JSON number decoded into a time.Duration is NANOSECONDS. §5.2's T_blind of
// 300 — five minutes to whoever typed it — would become 300ns: a stale-mark
// blind budget that has expired before the first check, firing the §7.1 de-risk
// on the first tick with nothing wrong. Nothing errors and nothing logs, which
// is why the number has to be refused rather than interpreted.
func TestDurationRejectsABareNumber(t *testing.T) {
	for _, in := range []string{`300`, `0`, `1.5`, `-1`, `1e9`} {
		var d config.Duration
		if err := json.Unmarshal([]byte(in), &d); !errors.Is(err, config.ErrDurationMalformed) {
			t.Errorf("Unmarshal(%s) = %v, want ErrDurationMalformed", in, err)
		}
	}
}

// TestDurationBareNumberWouldOtherwiseBeSilent measures what that refusal is
// worth: the same 300 into a plain time.Duration is accepted, and is 300
// nanoseconds.
func TestDurationBareNumberWouldOtherwiseBeSilent(t *testing.T) {
	var plain time.Duration
	if err := json.Unmarshal([]byte(`300`), &plain); err != nil {
		t.Fatalf("encoding/json rejected a number into time.Duration (%v) — "+
			"re-derive config.Duration before deleting it", err)
	}
	if plain != 300*time.Nanosecond {
		t.Fatalf("a bare 300 decoded to %v, want 300ns", plain)
	}
	if plain > time.Second {
		t.Fatal("unreachable, but states the consequence: 300ns is not five minutes")
	}
}

func TestDurationRejectsMalformedStrings(t *testing.T) {
	for _, in := range []string{`""`, `"5"`, `"5 m"`, `"m"`, `"five minutes"`, `"90sec"`, `true`, `null`, `{}`, `["5m"]`} {
		var d config.Duration
		if err := json.Unmarshal([]byte(in), &d); err == nil {
			t.Errorf("Unmarshal(%s) = nil, want an error (got %v)", in, d)
		}
	}
}

// TestDurationRejectsANegative covers both entry points, because they are
// reachable independently: a document goes through UnmarshalJSON, and code
// building a schema in a test goes through NewDuration.
func TestDurationRejectsANegative(t *testing.T) {
	var d config.Duration
	if err := json.Unmarshal([]byte(`"-5m"`), &d); !errors.Is(err, config.ErrDurationNegative) {
		t.Errorf("Unmarshal(\"-5m\") = %v, want ErrDurationNegative", err)
	}

	if err := config.NewDuration(-time.Second).Validate(); !errors.Is(err, config.ErrDurationNegative) {
		t.Errorf("NewDuration(-1s).Validate = %v, want ErrDurationNegative", err)
	}
}

// TestDurationZeroIsValid states the deliberate exception to §11's unusable-zero
// rule, so nobody "fixes" it later.
//
// Zero cost and zero fee compute the economic gate in our favour, which is why
// those must fail. Zero dwell and zero delay do not: "check immediately" and "no
// hysteresis" are settings somebody might genuinely want. A schema for which
// zero is wrong says so in its own Validate, where it can name the field.
func TestDurationZeroIsValid(t *testing.T) {
	if err := config.NewDuration(0).Validate(); err != nil {
		t.Errorf("NewDuration(0).Validate = %v, want nil", err)
	}
	var d config.Duration
	if err := d.Validate(); err != nil {
		t.Errorf("zero config.Duration Validate = %v, want nil", err)
	}
}

func TestDurationJSONRoundTrip(t *testing.T) {
	for _, want := range []time.Duration{0, time.Second, 90 * time.Second, 36 * time.Hour} {
		encoded, err := json.Marshal(config.NewDuration(want))
		if err != nil {
			t.Fatalf("Marshal(%v) = %v", want, err)
		}

		var back config.Duration
		if err := json.Unmarshal(encoded, &back); err != nil {
			t.Fatalf("Unmarshal(%s) = %v — this package must accept its own output", encoded, err)
		}
		if back.Duration() != want {
			t.Errorf("round trip: %v -> %s -> %v", want, encoded, back.Duration())
		}
	}
}
