package apperror_test

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/apperror"
)

// This package exists because build-spec §5.3 forbids one specific inference:
// "not sent" is never inferred from a failed Place. The order lifecycle has a
// state — SubmissionUnknown — whose whole purpose is to hold an order whose fate
// is genuinely unknown until a query settles it, and the thing that puts an
// order there is an error saying so.
//
// So the tests below are mostly about ONE property, approached from several
// directions: it must be hard to end up believing a venue write did not happen
// when the evidence does not say so. Every default, every wrap rule and every
// nil case is checked against that, including the paths where the caller made a
// mistake — because a taxonomy that is only safe when used correctly is not a
// safety mechanism, it is documentation.

func TestMayHaveApplied(t *testing.T) {
	// Read each case as: the OMS is holding this error after calling Place, and
	// is deciding between "resolve by query" (true) and "nothing to resolve"
	// (false). Getting false wrong creates a duplicate live order.
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{
			// Nothing failed, so nothing is in doubt. This is the only reason
			// MayHaveApplied takes an error rather than being a method on
			// Effect: the nil check has to live somewhere, and spreading it
			// across call sites is how one call site forgets it.
			name: "nil error", err: nil, want: false,
		},
		{
			// The load-bearing default. An error this package never saw — from
			// the standard library, from a dependency, from a branch nobody
			// classified — must read as ambiguous. If this ever flips to false,
			// every unclassified failure becomes a licence to resubmit.
			name: "unclassified error", err: errors.New("something broke"), want: true,
		},
		{
			name: "unclassified stdlib sentinel", err: io.ErrUnexpectedEOF, want: true,
		},
		{
			name: "transport, unknown",
			err:  apperror.Transport("venue.Place", apperror.EffectUnknown, io.ErrUnexpectedEOF),
			want: true,
		},
		{
			// A connection refused before the first byte is the one transport
			// case that IS evidence. It is also the only way to reach false
			// through the transport constructor, which is why that constructor
			// makes the effect an argument.
			name: "transport, proved not applied",
			err:  apperror.Transport("venue.Place", apperror.EffectNotApplied, errors.New("connection refused")),
			want: false,
		},
		{
			name: "rate limited",
			err:  apperror.RateLimited("venue.Place", "429", time.Second, nil),
			want: false,
		},
		{
			name: "venue rejected",
			err:  apperror.VenueRejected("venue.Place", "-2011", "below min notional", nil),
			want: false,
		},
		{
			name: "unauthorized",
			err:  apperror.Unauthorized("venue.Place", "-2015", nil),
			want: false,
		},
		{
			name: "precondition",
			err:  apperror.Precondition("oms.Plan", "quantity failed Validate", nil),
			want: false,
		},
		{
			// An internal defect says nothing about how far the call got.
			name: "internal defect",
			err:  apperror.Internal("oms.Apply", "unreachable branch", nil),
			want: true,
		},
		{
			// Classification must survive an outer fmt.Errorf. Adapters wrap
			// with %w for context constantly, and a taxonomy that a single
			// annotation defeats would be worse than none: it would read as
			// safe right up until someone added a message.
			name: "classified, then wrapped with fmt.Errorf",
			err: fmt.Errorf("submitting leg 1: %w",
				apperror.Transport("venue.Place", apperror.EffectUnknown, nil)),
			want: true,
		},
		{
			name: "not-applied classification survives fmt.Errorf too",
			err: fmt.Errorf("submitting leg 1: %w",
				apperror.RateLimited("venue.Place", "429", 0, nil)),
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := apperror.MayHaveApplied(tc.err); got != tc.want {
				t.Errorf("MayHaveApplied(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

// TestNoLaunderingOfCertainty is the reason resolveEffect exists.
//
// EffectOf reads the OUTERMOST classification, because that is the one nearest
// the caller and usually the most specific. On its own that rule has a hole: an
// application layer wrapping an ambiguous transport failure could restate it as
// certain, and "the order might be live" would become "there is no order"
// through nothing but an error message. These cases assert the hole is closed.
func TestNoLaunderingOfCertainty(t *testing.T) {
	ambiguous := apperror.Transport("venue.Place", apperror.EffectUnknown, io.ErrUnexpectedEOF)

	cases := []struct {
		name  string
		outer *apperror.Error
	}{
		{
			// Every constructor that fixes EffectNotApplied is a potential
			// laundering site, so each one is checked rather than one standing
			// for the others.
			name:  "precondition wrapping an ambiguous transport failure",
			outer: apperror.Precondition("oms.Submit", "state forbids submission", ambiguous),
		},
		{
			name:  "venue reject wrapping it",
			outer: apperror.VenueRejected("oms.Submit", "-2011", "rejected", ambiguous),
		},
		{
			name:  "rate limit wrapping it",
			outer: apperror.RateLimited("oms.Submit", "429", time.Second, ambiguous),
		},
		{
			name:  "auth wrapping it",
			outer: apperror.Unauthorized("oms.Submit", "-2015", ambiguous),
		},
		{
			name:  "not-found wrapping it",
			outer: apperror.NotFound("oms.Resolve", "no such order", ambiguous),
		},
		{
			name: "an explicit Spec claiming certainty",
			outer: apperror.New(apperror.Spec{
				Class:  apperror.ClassVenueReject,
				Effect: apperror.EffectNotApplied,
				Op:     "oms.Submit",
				Cause:  ambiguous,
			}),
		},
		{
			name: "an explicit Spec claiming the opposite certainty",
			outer: apperror.New(apperror.Spec{
				Class:  apperror.ClassInternal,
				Effect: apperror.EffectApplied,
				Op:     "oms.Submit",
				Cause:  ambiguous,
			}),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.outer.Effect(); got != apperror.EffectUnknown {
				t.Errorf("Effect() = %v, want EffectUnknown — an ambiguous cause was laundered into a certain outcome", got)
			}
			if !apperror.MayHaveApplied(tc.outer) {
				t.Error("MayHaveApplied() = false; the OMS would skip the resolving query on an order that may be live")
			}
			// The Class is NOT constrained the same way. Only the safety axis
			// is monotone; an outer layer is entitled to reclassify the cause
			// for routing, which is the whole reason the two axes are separate.
			if tc.outer.Class() == apperror.ClassTransport && tc.name != "transport" {
				t.Logf("class carried through as %v", tc.outer.Class())
			}
		})
	}
}

// TestEffectAgreementIsPreserved is the other half of the wrap rule, and it is
// a positive control for the test above: if collapsing to Unknown were
// unconditional rather than triggered by disagreement, every case in
// TestNoLaunderingOfCertainty would still pass and the rule would be "always
// unknown", which is useless.
func TestEffectAgreementIsPreserved(t *testing.T) {
	notApplied := apperror.VenueRejected("venue.Place", "-2011", "below min notional", nil)

	outer := apperror.Precondition("oms.Submit", "leg refused", notApplied)
	if got := outer.Effect(); got != apperror.EffectNotApplied {
		t.Errorf("Effect() = %v, want EffectNotApplied — two layers agreeing should not produce doubt", got)
	}
	if apperror.MayHaveApplied(outer) {
		t.Error("MayHaveApplied() = true on two agreeing not-applied classifications")
	}

	// And an unclassified cause does not poison a certain claim, because an
	// unclassified CAUSE has no recorded effect to disagree with — the Unknown
	// default belongs to EffectOf's reading of a chain, not to a nil-effect
	// field. Getting this wrong would make every wrapped stdlib error ambiguous
	// and the NotApplied branch would become unreachable in practice.
	overPlain := apperror.VenueRejected("venue.Place", "-2011", "rejected", errors.New("json: cannot unmarshal"))
	if got := overPlain.Effect(); got != apperror.EffectNotApplied {
		t.Errorf("Effect() over an unclassified cause = %v, want EffectNotApplied", got)
	}
}

func TestClassOf(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want apperror.Class
	}{
		{"nil", nil, apperror.ClassInternal},
		{"unclassified", errors.New("boom"), apperror.ClassInternal},
		{"transport", apperror.Transport("venue.Place", apperror.EffectUnknown, nil), apperror.ClassTransport},
		{"rate limit", apperror.RateLimited("venue.Place", "429", 0, nil), apperror.ClassRateLimit},
		{"auth", apperror.Unauthorized("venue.Place", "-2015", nil), apperror.ClassAuth},
		{"venue reject", apperror.VenueRejected("venue.Place", "-2011", "", nil), apperror.ClassVenueReject},
		{"not found", apperror.NotFound("venue.OrderByClientID", "", nil), apperror.ClassNotFound},
		{"precondition", apperror.Precondition("oms.Plan", "", nil), apperror.ClassPrecondition},
		{"internal", apperror.Internal("oms.Apply", "", nil), apperror.ClassInternal},
		{
			"wrapped with fmt.Errorf",
			fmt.Errorf("ctx: %w", apperror.RateLimited("venue.Place", "429", 0, nil)),
			apperror.ClassRateLimit,
		},
		{
			// Two classifications in one chain: the outermost wins, because it
			// is the one nearest the layer asking. Unlike Effect, Class carries
			// no safety obligation, so nothing constrains this.
			"outermost of two classifications wins",
			apperror.Precondition("oms.Submit", "",
				apperror.Transport("venue.Place", apperror.EffectUnknown, nil)),
			apperror.ClassPrecondition,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := apperror.ClassOf(tc.err); got != tc.want {
				t.Errorf("ClassOf() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestEffectOf_NilIsUnspecifiedNotUnknown(t *testing.T) {
	// The distinction matters for the metrics path, which reports the effect of
	// failures. If nil reported EffectUnknown, a caller building a label from
	// every call — including the successful ones — would publish a stream of
	// ambiguous outcomes that never happened.
	if got := apperror.EffectOf(nil); got != apperror.EffectUnspecified {
		t.Errorf("EffectOf(nil) = %v, want EffectUnspecified", got)
	}
	// But the SAFETY path must not read that sentinel as certainty. This is the
	// pair of assertions that pins the difference; either alone permits the
	// wrong design.
	if apperror.EffectUnspecified == apperror.EffectNotApplied {
		t.Fatal("the unspecified sentinel must not equal the certain value")
	}
	if apperror.MayHaveApplied(nil) {
		t.Error("MayHaveApplied(nil) = true")
	}
}

func TestMalformedSpecDegradesSafely(t *testing.T) {
	cases := []struct {
		name string
		spec apperror.Spec
		// wantErrIs is the sentinel the malformed report must wrap, so the
		// degradation names WHICH field was wrong rather than just that one was.
		wantErrIs error
	}{
		{
			name:      "effect left unspecified",
			spec:      apperror.Spec{Class: apperror.ClassTransport, Op: "venue.Place"},
			wantErrIs: apperror.ErrEffectInvalid,
		},
		{
			name: "effect cast out of range",
			spec: apperror.Spec{
				Class: apperror.ClassTransport, Effect: apperror.Effect(200), Op: "venue.Place",
			},
			wantErrIs: apperror.ErrEffectInvalid,
		},
		{
			name: "class misspelled",
			spec: apperror.Spec{
				Class: apperror.Class("ratelimit"), Effect: apperror.EffectNotApplied, Op: "venue.Place",
			},
			wantErrIs: apperror.ErrClassInvalid,
		},
		{
			name: "class left empty",
			spec: apperror.Spec{Effect: apperror.EffectNotApplied, Op: "venue.Place"},
			// An empty Class is not a distinct case in the taxonomy — it fails
			// the same declared-set check a typo does, which is the point of
			// having no "unset" member.
			wantErrIs: apperror.ErrClassInvalid,
		},
		{
			name:      "op omitted",
			spec:      apperror.Spec{Class: apperror.ClassTransport, Effect: apperror.EffectUnknown},
			wantErrIs: apperror.ErrOpUnset,
		},
		{
			// Whitespace is not an operation name. Without TrimSpace this
			// passes the emptiness check and produces a metric series labelled
			// with a space.
			name: "op is whitespace",
			spec: apperror.Spec{
				Class: apperror.ClassTransport, Effect: apperror.EffectUnknown, Op: "   ",
			},
			wantErrIs: apperror.ErrOpUnset,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := apperror.New(tc.spec)

			// Loud: the defect is reported and identifies the field.
			if got.Malformed() == nil {
				t.Fatal("Malformed() = nil on a spec that does not validate")
			}
			if !errors.Is(got.Malformed(), apperror.ErrMalformed) {
				t.Error("Malformed() does not wrap ErrMalformed, so a boot check cannot find it")
			}
			if !errors.Is(got.Malformed(), tc.wantErrIs) {
				t.Errorf("Malformed() = %v, want it to wrap %v", got.Malformed(), tc.wantErrIs)
			}

			// Safe: it still reads as ambiguous rather than as certainty.
			if got.Effect() != apperror.EffectUnknown {
				t.Errorf("Effect() = %v, want EffectUnknown", got.Effect())
			}
			if !apperror.MayHaveApplied(got) {
				t.Error("MayHaveApplied() = false on a malformed classification")
			}
			if got.Class() != apperror.ClassInternal {
				t.Errorf("Class() = %v, want ClassInternal", got.Class())
			}

			// Usable: no panic, no empty string, and never an empty op — an
			// unlabelled metric series is how this failure would hide.
			if got.Op() == "" {
				t.Error("Op() is empty; the degraded error would produce an unlabelled metric series")
			}
			if got.Error() == "" {
				t.Error("Error() is empty")
			}
		})
	}
}

// TestWellFormedSpecIsNotMalformed is the positive control for the test above.
// Without it, a Malformed() that returned non-nil unconditionally would score a
// perfect result on every case there.
func TestWellFormedSpecIsNotMalformed(t *testing.T) {
	specs := []apperror.Spec{
		{Class: apperror.ClassTransport, Effect: apperror.EffectUnknown, Op: "venue.Place"},
		{Class: apperror.ClassRateLimit, Effect: apperror.EffectNotApplied, Op: "venue.Place", Code: "429"},
		{Class: apperror.ClassInternal, Effect: apperror.EffectApplied, Op: "oms.Apply", Detail: "d"},
	}
	for _, s := range specs {
		got := apperror.New(s)
		if got.Malformed() != nil {
			t.Errorf("Malformed() = %v on a valid spec %+v", got.Malformed(), s)
		}
		if got.Class() != s.Class {
			t.Errorf("Class() = %v, want %v", got.Class(), s.Class)
		}
		if got.Effect() != s.Effect {
			t.Errorf("Effect() = %v, want %v", got.Effect(), s.Effect)
		}
	}
}

func TestConstructorClassifications(t *testing.T) {
	// Each constructor's (class, effect) pair, stated once so that a change to
	// any of them is a visible edit here rather than a silent behaviour change.
	// The effects are not arbitrary: everything except Transport and Internal is
	// EffectNotApplied because the request demonstrably did not execute, and
	// Transport is the one that takes the effect as an argument precisely
	// because it cannot be fixed.
	cases := []struct {
		name       string
		err        *apperror.Error
		wantClass  apperror.Class
		wantEffect apperror.Effect
	}{
		{"RateLimited", apperror.RateLimited("op", "429", 0, nil), apperror.ClassRateLimit, apperror.EffectNotApplied},
		{"Unauthorized", apperror.Unauthorized("op", "-2015", nil), apperror.ClassAuth, apperror.EffectNotApplied},
		{"VenueRejected", apperror.VenueRejected("op", "-2011", "", nil), apperror.ClassVenueReject, apperror.EffectNotApplied},
		{"NotFound", apperror.NotFound("op", "", nil), apperror.ClassNotFound, apperror.EffectNotApplied},
		{"Precondition", apperror.Precondition("op", "", nil), apperror.ClassPrecondition, apperror.EffectNotApplied},
		{"Internal", apperror.Internal("op", "", nil), apperror.ClassInternal, apperror.EffectUnknown},
		{"Transport(unknown)", apperror.Transport("op", apperror.EffectUnknown, nil), apperror.ClassTransport, apperror.EffectUnknown},
		{"Transport(not applied)", apperror.Transport("op", apperror.EffectNotApplied, nil), apperror.ClassTransport, apperror.EffectNotApplied},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.err.Malformed() != nil {
				t.Fatalf("constructor produced a malformed error: %v", tc.err.Malformed())
			}
			if got := tc.err.Class(); got != tc.wantClass {
				t.Errorf("Class() = %v, want %v", got, tc.wantClass)
			}
			if got := tc.err.Effect(); got != tc.wantEffect {
				t.Errorf("Effect() = %v, want %v", got, tc.wantEffect)
			}
		})
	}
}

// TestTransportRefusesToPickAnEffect pins the signature decision rather than a
// behaviour: Transport must not have a default effect. If someone later adds one
// by giving the parameter a zero-value meaning, the malformed path catches it.
func TestTransportWithUnspecifiedEffectIsMalformed(t *testing.T) {
	got := apperror.Transport("venue.Place", apperror.EffectUnspecified, nil)
	if got.Malformed() == nil {
		t.Fatal("Transport accepted an unspecified effect silently; the one signature where the effect must be stated has acquired a default")
	}
	if !apperror.MayHaveApplied(got) {
		t.Error("and it did not degrade to the safe reading")
	}
}

func TestErrorsIsAndAsThroughTheChain(t *testing.T) {
	root := io.ErrUnexpectedEOF
	classified := apperror.Transport("venue.Place", apperror.EffectUnknown, root)
	wrapped := fmt.Errorf("submitting leg 1: %w", classified)

	if !errors.Is(wrapped, io.ErrUnexpectedEOF) {
		t.Error("errors.Is could not reach the root cause; Unwrap is not connected")
	}

	var target *apperror.Error
	if !errors.As(wrapped, &target) {
		t.Fatal("errors.As could not find the classification")
	}
	if target.Op() != "venue.Place" {
		t.Errorf("Op() = %q, want %q", target.Op(), "venue.Place")
	}
}

func TestRetryAfterAndCodeAreCarriedVerbatim(t *testing.T) {
	// §7.6 names specific codes as hard pauses and others as immediate operator
	// alerts. This package carries them and interprets nothing — the Class is
	// our interpretation, the Code is the evidence for it, and an operator
	// triaging an alert needs the evidence.
	err := apperror.RateLimited("venue.Place", "10018", 7*time.Second, nil)
	if got := err.Code(); got != "10018" {
		t.Errorf("Code() = %q, want %q", got, "10018")
	}
	if got := err.RetryAfter(); got != 7*time.Second {
		t.Errorf("RetryAfter() = %v, want 7s", got)
	}

	// Zero means the venue said nothing — Bybit's 10018 IP ban is explicitly
	// unheadered (§7.6) — which is different from "retry now". This package does
	// not substitute a default for the silence.
	silent := apperror.RateLimited("venue.Place", "10018", 0, nil)
	if got := silent.RetryAfter(); got != 0 {
		t.Errorf("RetryAfter() = %v on a venue that sent no hint, want 0", got)
	}
}

func TestErrorMessageNamesTheAmbiguity(t *testing.T) {
	// An operator reading a log at 3am should not have to remember which classes
	// are ambiguous, so the effect is spelled out rather than implied by the
	// class.
	ambiguous := apperror.Transport("venue.Place", apperror.EffectUnknown, io.ErrUnexpectedEOF)
	msg := ambiguous.Error()
	for _, want := range []string{"venue.Place", "transport", "MAY HAVE EXECUTED", "unexpected EOF"} {
		if !strings.Contains(msg, want) {
			t.Errorf("Error() = %q, missing %q", msg, want)
		}
	}

	certain := apperror.RateLimited("venue.Place", "429", 0, nil)
	certainMsg := certain.Error()
	if strings.Contains(certainMsg, "MAY HAVE EXECUTED") {
		t.Errorf("Error() = %q; a proved-not-applied failure must not read as ambiguous", certainMsg)
	}
	for _, want := range []string{"rate_limit", "429", "did not execute"} {
		if !strings.Contains(certainMsg, want) {
			t.Errorf("Error() = %q, missing %q", certainMsg, want)
		}
	}
}

func TestClassValidate(t *testing.T) {
	for _, c := range []apperror.Class{
		apperror.ClassTransport, apperror.ClassRateLimit, apperror.ClassAuth,
		apperror.ClassVenueReject, apperror.ClassNotFound, apperror.ClassPrecondition,
		apperror.ClassInternal,
	} {
		if err := c.Validate(); err != nil {
			t.Errorf("Validate() on declared class %q = %v", c, err)
		}
		if got := c.String(); got != string(c) {
			t.Errorf("String() = %q, want %q for a declared class", got, string(c))
		}
	}

	for _, c := range []apperror.Class{"", "Transport", "transport ", "rate-limit", "unknown"} {
		if err := c.Validate(); !errors.Is(err, apperror.ErrClassInvalid) {
			t.Errorf("Validate() on undeclared class %q = %v, want ErrClassInvalid", c, err)
		}
		if got := c.String(); !strings.HasPrefix(got, "Class(") {
			t.Errorf("String() = %q on an undeclared class; it should not pass for a declared one", got)
		}
	}
}

// TestClassWireTokensAreFrozen pins the exact strings, because they are a
// compatibility contract rather than an implementation detail.
//
// This test exists because a mutation renaming "not_found" to "notfound"
// SURVIVED the whole suite. The declared-set test could not see it: the
// `classes` slice names the constants, not the literals, so a rename moves the
// declaration and the lookup table together and they still agree. Every
// behavioural test compares a constant against itself and is likewise blind.
//
// What breaks is outside this program. §10 makes Class a metric label and the
// audit projection stores it, so a renamed token orphans every dashboard query
// and splits a stored series in two, with nothing failing. That is precisely the
// boundary §3.6 rule 8 reserves the string shape for — so the tokens get frozen
// the way any wire format does, and changing one becomes a deliberate edit here.
//
// Effect deliberately gets no equivalent test. It is an argument, never
// persisted and never transmitted, so its String() output is debug text and
// pinning it would be ceremony.
func TestClassWireTokensAreFrozen(t *testing.T) {
	frozen := map[apperror.Class]string{
		apperror.ClassTransport:    "transport",
		apperror.ClassRateLimit:    "rate_limit",
		apperror.ClassAuth:         "auth",
		apperror.ClassVenueReject:  "venue_reject",
		apperror.ClassNotFound:     "not_found",
		apperror.ClassPrecondition: "precondition",
		apperror.ClassInternal:     "internal",
	}
	for c, want := range frozen {
		if string(c) != want {
			t.Errorf("wire token changed: %q, want %q — this breaks stored audit rows and dashboard queries", string(c), want)
		}
	}
}

func TestEffectValidate(t *testing.T) {
	for _, e := range []apperror.Effect{
		apperror.EffectNotApplied, apperror.EffectUnknown, apperror.EffectApplied,
	} {
		if err := e.Validate(); err != nil {
			t.Errorf("Validate() on declared effect %v = %v", e, err)
		}
	}
	for _, e := range []apperror.Effect{apperror.EffectUnspecified, apperror.Effect(4), apperror.Effect(255)} {
		if err := e.Validate(); !errors.Is(err, apperror.ErrEffectInvalid) {
			t.Errorf("Validate() on %v = %v, want ErrEffectInvalid", e, err)
		}
	}
}

// TestUndeclaredEffectCannotOpenTheRetryBranch is a negative control for the
// mayHaveApplied predicate. Effect is a defined uint8, so Effect(n) compiles for
// every n; the predicate is written as "everything except NotApplied" rather
// than "Unknown or Applied or Unspecified" so that a value nobody declared falls
// on the safe side instead of off the end of a list.
func TestUndeclaredEffectCannotOpenTheRetryBranch(t *testing.T) {
	for _, raw := range []apperror.Effect{4, 5, 99, 255} {
		err := apperror.New(apperror.Spec{
			Class: apperror.ClassTransport, Effect: raw, Op: "venue.Place",
		})
		if !apperror.MayHaveApplied(err) {
			t.Errorf("MayHaveApplied() = false for undeclared Effect(%d)", uint8(raw))
		}
	}
}
