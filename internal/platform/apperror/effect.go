package apperror

import "fmt"

// Effect answers the one question the safety kernel actually asks of a failed
// venue call: did the side effect happen?
//
// This is the axis build-spec §5.3 is built around. Place is non-idempotent,
// transport retries are forbidden, and "not sent" is NEVER inferred — an order
// at DispatchAuthorized or beyond is resolved by query only, because the single
// Place may have executed with its response lost. An error type that reported
// only what went wrong would force the OMS to re-derive that fact from the
// cause, and re-deriving it is exactly the inference the spec forbids. So the
// adapter that made the call — the only layer that knows whether the request
// left the socket — records the answer here.
//
// It is an integer enum rather than §3.6 rule 8's string shape, the same
// documented exception fixedpoint.RoundingMode takes and for the same reason: an
// Effect is an argument, never persisted and never transmitted (what reaches the
// audit row is the resulting order state, not this), and the property that
// matters is that the zero value be an unusable sentinel.
type Effect uint8

const (
	// EffectUnspecified is the zero value and is never a valid argument. A
	// classification that nobody filled in fails Validate.
	//
	// It is nonetheless SAFE to encounter, which is the difference between this
	// sentinel and RoundInvalid's. MayHaveApplied reads it as "yes, it may
	// have", so a spec built with the field forgotten degrades to the
	// conservative reading rather than to a silent "proved not sent". Loud
	// where it can be, safe where it cannot — the same division ServerTime
	// takes when an uncalibrated venue answers Uncertain.
	EffectUnspecified Effect = iota

	// EffectNotApplied means the side effect provably did not happen: the
	// connection was refused before a byte was written, the local governor
	// declined to send, the venue rejected the request on its merits, our own
	// Validate refused it. There is no order at the venue and no query is owed.
	//
	// This is the only value that permits a fresh attempt, and it is a claim
	// about evidence — never a default, never an inference from silence.
	EffectNotApplied

	// EffectUnknown means the request may or may not have executed. A write
	// timeout, a read timeout after the request was sent, a truncated response,
	// a process death mid-call. This is the value that drives the order
	// lifecycle to SubmissionUnknown (§5.3) and hands resolution to
	// OrderByClientID under the venue-specific stable-not-found rule.
	//
	// It is the safe answer in every ambiguous case, and it is what an
	// unclassified error resolves to. Treating it as NotApplied is the specific
	// mistake that produces a duplicate live order.
	EffectUnknown

	// EffectApplied means the side effect definitely happened and the call
	// failed anyway: the venue accepted the order and then the response failed
	// to decode, a margin transfer committed and the acknowledgement was
	// malformed. Rarer than the other two and easy to omit, but it is a
	// different instruction from Unknown — nothing needs resolving, the local
	// record needs correcting.
	EffectApplied
)

// maxEffect is the last declared value. Validate compares against it, so adding
// a value means extending this — which the declared-set test checks.
const maxEffect = EffectApplied

// Validate reports whether e is one of the declared values, excluding the
// unspecified sentinel.
func (e Effect) Validate() error {
	if e == EffectUnspecified || e > maxEffect {
		return fmt.Errorf("effect %d: %w", uint8(e), ErrEffectInvalid)
	}
	return nil
}

// mayHaveApplied reports whether e leaves open the possibility that the side
// effect ran.
//
// Note which values answer true: Unknown, Applied, and the unspecified
// sentinel. Only NotApplied — the one value that is a claim about evidence —
// answers false. Any value this package does not recognise answers true as well,
// so a stray Effect(9) cast cannot open the retry branch.
//
// It is unexported because the useful entry point takes an error and handles
// nil; see MayHaveApplied.
func (e Effect) mayHaveApplied() bool { return e != EffectNotApplied }

// String implements fmt.Stringer.
//
// This switch is the shape the deferred exhaustiveswitch gate (§3.6 rule 10)
// exists to police: a value added above falls through and prints as a number.
// Survivable here; it would not be in a safety state machine.
func (e Effect) String() string {
	switch e {
	case EffectUnspecified:
		return "EffectUnspecified"
	case EffectNotApplied:
		return "EffectNotApplied"
	case EffectUnknown:
		return "EffectUnknown"
	case EffectApplied:
		return "EffectApplied"
	}
	return fmt.Sprintf("Effect(%d)", uint8(e))
}

// resolveEffect combines the effect a caller is claiming with the effect already
// recorded by the error being wrapped.
//
// The rule is one sentence: agreement is preserved, disagreement collapses to
// EffectUnknown. Certainty can never be manufactured by re-wrapping.
//
// The hazard this closes is real and quiet. EffectOf finds the OUTERMOST
// classification in a chain, because that is the one closest to the caller's own
// layer and usually the most specific. But that also means an application layer
// wrapping a transport error could restate a genuinely ambiguous outcome as
// EffectNotApplied — laundering "the order might be live" into "there is no
// order" through nothing more than an error message. With this rule the
// laundering is impossible: the disagreement itself is evidence that the two
// layers do not know, so the pair resolves to the value that sends the OMS to
// the venue to ask.
//
// A caller that has genuinely NEW evidence — a resolving query that proved the
// order absent — does not wrap. It constructs a fresh error, or more usually
// stops holding an error at all, because it now has an answer.
func resolveEffect(claimed, recorded Effect) Effect {
	switch {
	case recorded == EffectUnspecified:
		// Nothing recorded below: the claim stands on its own. An unspecified
		// claim stays unspecified here rather than being silently promoted —
		// New is where that degrades to EffectUnknown, so the degradation
		// happens once and is reported once.
		return claimed
	case claimed == EffectUnspecified:
		// The wrapper said nothing, so it overrides nothing.
		return recorded
	case claimed == recorded:
		return claimed
	default:
		return EffectUnknown
	}
}
