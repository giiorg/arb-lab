package ids

import "fmt"

// ClientOrderID is the OMS-minted identifier carried on a venue order —
// Binance newClientOrderId, Bybit orderLinkId.
//
// # What §5.3 makes it, and what it is not
//
// It is a RECOVERY CORRELATION HANDLE, not a venue idempotency key. The spec is
// emphatic because the difference is a safety property: Binance permits reuse
// once an order is terminal, so sending the same id twice does not deduplicate
// anything. What the id buys is the ability, after a crash, to ask the venue
// "what happened to this?" — which is the ONLY resolution §5.3 permits for an
// order at DispatchAuthorized or beyond. Never resubmit, never infer "not sent".
//
// # Which is why it is derived rather than random
//
// Derivation is a pure function of (TradeID, leg, attempt). Nothing needs to be
// stored for the OMS to re-derive it after a crash, and that matters because
// §11 says the local journal is host-local and lost on host death — the failure
// where knowing the id matters most is the one where a lookup table would be
// gone.
//
// # And why it is reversible rather than hashed
//
// The text carries the TradeID rather than a digest of it, so an operator
// reading a venue's order page during an incident recovers the trade without
// database access. Again: the incident where that is needed is the one where
// the database may be what died. The cost is that the id is not opaque — a
// venue's support staff can see we group orders by trade, which is not a secret
// worth an incident-time lookup.
//
// # Format
//
//	ab-<26 Crockford base32 of the TradeID>-<leg, 2 digits>-<attempt, 2 digits>
//	ab-01J8ZQ4M7K3P9V2XW5R6T8Y0BN-01-00
//
// Fixed at 35 characters, always. Fixed length is worth the two padding digits:
// a truncation is visible as a length check rather than as a parse that happens
// to succeed, and legs and attempts sort correctly as text.
//
// The "ab-" prefix keeps our ids clear of the venue's own. Binance stamps
// system-generated closes with clientOrderId prefixes — "autoclose-" for
// liquidation, "adl_autoclose" for ADL, "settlement_autoclose-" for delisting
// (venue-profiles, R1d-7) — and one of those parsing as ours would attribute a
// liquidation to a trade.
//
// # An unverified venue assumption, stated rather than buried
//
// The 36-character budget and the [A-Za-z0-9_-] character set this format fits
// inside are NOT recorded in venue-profiles.md, which is where this project puts
// verified venue facts. They are the widely-documented limits for both venues
// and this format is designed conservatively inside them, but they have not been
// browser-confirmed to the standard §4 sets. Confirm before milestone 3 places a
// real order. If a venue turns out to allow less, the 26-character TradeID field
// is the only part with any give, and shortening it means hashing — which is the
// alternative this design rejected.
type ClientOrderID string

const (
	// clientOrderIDPrefix distinguishes our ids from the venue's own.
	clientOrderIDPrefix = "ab-"

	// clientOrderIDLen is the exact rendered length: 3 + 26 + 1 + 2 + 1 + 2.
	clientOrderIDLen = 35

	// clientOrderIDBudget is the length limit the format is designed inside.
	// See the type's documentation: this is an unverified venue assumption, and
	// naming it as a constant is what lets a test assert the format stays
	// within it rather than leaving the claim in a comment.
	clientOrderIDBudget = 36

	// maxLeg and maxAttempt are what two decimal digits hold.
	maxLeg     = 99
	maxAttempt = 99
)

// ClientOrderRef is the triple a ClientOrderID encodes.
type ClientOrderRef struct {
	// Trade is the trade the order belongs to.
	Trade TradeID

	// Leg indexes the order within the trade — 0 and 1 for a cash-and-carry
	// package's spot and perp legs.
	Leg int

	// Attempt distinguishes successive submissions for the same leg, and is
	// what makes §5.3's "never reused within the recoverable window" hold. It
	// increments on a replan, never on a retry: §5.3 forbids transport retries
	// outright, so a second id for the same leg means a new decision to submit,
	// not a second attempt at delivering the first.
	Attempt int
}

func (r ClientOrderRef) Validate() error {
	if err := r.Trade.Validate(); err != nil {
		return err
	}
	if r.Leg < 0 || r.Leg > maxLeg {
		return fmt.Errorf("leg %d: %w", r.Leg, ErrLegRange)
	}
	if r.Attempt < 0 || r.Attempt > maxAttempt {
		return fmt.Errorf("attempt %d: %w", r.Attempt, ErrAttemptRange)
	}
	return nil
}

// NewClientOrderID derives the id for a reference.
//
// It is a pure function: the same reference yields the same id on every host, in
// every process, forever. That is the property crash recovery rests on, and it
// is why nothing here consults a clock, a counter or a random source.
//
// The range checks are errors rather than silent widening because the format has
// a hard venue-side length budget. A leg of 100 rendered as three digits would
// produce a 36-character id — still inside the budget today, and one character
// from an id the venue rejects at submission time, on a path that by definition
// only runs when something else has already gone wrong.
func NewClientOrderID(ref ClientOrderRef) (ClientOrderID, error) {
	if err := ref.Validate(); err != nil {
		return "", err
	}
	id := ClientOrderID(fmt.Sprintf("%s%s-%02d-%02d",
		clientOrderIDPrefix, ref.Trade.UUID().Base32(), ref.Leg, ref.Attempt))
	if len(id) != clientOrderIDLen {
		// Unreachable given the checks above, and checked anyway: this is the
		// assertion that the format and the constant have not drifted apart,
		// and it fires at derivation rather than at the venue.
		return "", fmt.Errorf("ids: derived id %q is %d characters, want %d", id, len(id), clientOrderIDLen)
	}
	return id, nil
}

// ParseClientOrderID recovers the reference from an id.
//
// It is strict about everything: exact length, exact prefix, exact separator
// positions, the canonical base32 alphabet, two decimal digits per field, and a
// decoded UUID that is a well-formed v7. Anything else is not one of ours, and
// the ids that are not ours are the ones that matter — a Binance
// "autoclose-..." parsing as a trade id would attribute a liquidation to a trade
// we placed.
func ParseClientOrderID(s string) (ClientOrderRef, error) {
	if len(s) != clientOrderIDLen {
		return ClientOrderRef{}, fmt.Errorf("%q is %d characters, want %d: %w",
			s, len(s), clientOrderIDLen, ErrClientOrderIDSyntax)
	}
	if s[:len(clientOrderIDPrefix)] != clientOrderIDPrefix {
		return ClientOrderRef{}, fmt.Errorf("%q lacks the %q prefix: %w",
			s, clientOrderIDPrefix, ErrClientOrderIDSyntax)
	}

	// Fixed offsets, because the length is fixed. Splitting on '-' would also
	// work until a venue id happened to contain the right number of them.
	const (
		trade    = len(clientOrderIDPrefix) // 3
		sep1     = trade + base32Len        // 29
		legAt    = sep1 + 1                 // 30
		sep2     = legAt + 2                // 32
		attempAt = sep2 + 1                 // 33
	)
	if s[sep1] != '-' || s[sep2] != '-' {
		return ClientOrderRef{}, fmt.Errorf("%q has separators in the wrong place: %w", s, ErrClientOrderIDSyntax)
	}

	raw, err := decodeBase32(s[trade:sep1])
	if err != nil {
		return ClientOrderRef{}, fmt.Errorf("%q: %w", s, err)
	}
	u := UUID(raw)
	if err := u.Validate(); err != nil {
		return ClientOrderRef{}, fmt.Errorf("%q: %w", s, err)
	}

	leg, err := twoDigits(s[legAt : legAt+2])
	if err != nil {
		return ClientOrderRef{}, fmt.Errorf("%q leg: %w", s, err)
	}
	attempt, err := twoDigits(s[attempAt : attempAt+2])
	if err != nil {
		return ClientOrderRef{}, fmt.Errorf("%q attempt: %w", s, err)
	}

	return ClientOrderRef{Trade: TradeID{u: u}, Leg: leg, Attempt: attempt}, nil
}

// twoDigits reads exactly two ASCII digits.
//
// Hand-written rather than strconv.Atoi because Atoi accepts a leading sign and
// leading whitespace: "+5" and " 5" would parse, and the id would round-trip to
// a different string than the one that came in.
func twoDigits(s string) (int, error) {
	if len(s) != 2 {
		return 0, ErrClientOrderIDSyntax
	}
	v := 0
	for i := range 2 {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("%q is not two digits: %w", s, ErrClientOrderIDSyntax)
		}
		v = v*10 + int(c-'0')
	}
	return v, nil
}
