// Package ids mints and parses the identities build-spec §5.1 assigns here:
// UUIDv7, the typed IDs built on it, and the deterministic ClientOrderID
// derivation §5.3 requires.
//
// # Nothing in here reads the wall clock or the system entropy pool by itself
//
// A UUIDv7 embeds a 48-bit millisecond timestamp and 74 bits of randomness, and
// both would break §3.5's backtest-live parity if taken ambiently: a replay that
// minted different ids from the same input is not a replay of the same run.
// So a Generator holds an injected TimeSource and an injected io.Reader, and the
// backtester supplies deterministic ones. Note that §3.6 rule 5 does NOT reach
// platform/ — no gate would have caught an ambient time.Now() here. The
// injection is a design decision, not a compliance one.
//
// # This package imports nothing from the module, on purpose
//
// It sits at the bottom of §1's dependency spine alongside fixedpoint. That
// matters concretely: clock imports THIS package for Venue, so an import in the
// other direction would be a cycle. The time seam is therefore a one-method
// interface declared here (TimeSource) rather than clock.Clock — Go's structural
// interfaces mean clock.Wall and clock.TestClock satisfy it with no import edge
// at all.
//
// # Two shapes of identity, and the difference is who mints them
//
// TradeID and IntentID are OURS: minted here as UUIDv7, valid or absent, with an
// unusable zero value. ExecutionID is the VENUE'S: received, never minted, and
// carrying a uniqueness caveat (§5.3) that has bitten this design before —
// it is unique only within an instrument.
//
// ClientOrderID is neither. It is derived — a pure function of a TradeID, a leg
// and an attempt — because §5.3 makes it a recovery correlation handle: after a
// crash the OMS must re-derive the same id to ask the venue what happened to it.
// A random id would need the journal to survive, and §11 says the journal is
// host-local and lost on host death.
package ids

import "errors"

// Sentinel errors. Callers branch on these with errors.Is, so they are part of
// the package's public contract.
var (
	// ErrVenueUnset reports an empty Venue.
	ErrVenueUnset = errors.New("ids: venue unset")

	// ErrUUIDUnset reports the zero UUID — sixteen zero bytes, which is a
	// legitimate RFC 9562 value (the Nil UUID) and never one this package
	// mints. It is what an unpopulated TradeID field reports.
	ErrUUIDUnset = errors.New("ids: unconstructed uuid")

	// ErrUUIDVersion reports a UUID that is well-formed but not version 7.
	// This package mints only v7 and every derivation depends on the embedded
	// timestamp, so a v4 arriving here is a wiring mistake rather than a
	// tolerable variation.
	ErrUUIDVersion = errors.New("ids: uuid is not version 7")

	// ErrUUIDVariant reports a UUID whose variant bits are not RFC 9562's
	// 0b10. Together with ErrUUIDVersion this is what makes "a valid id" a
	// checkable claim rather than "sixteen bytes".
	ErrUUIDVariant = errors.New("ids: uuid variant is not RFC 9562")

	// ErrUUIDSyntax reports text that is not a canonical UUID.
	ErrUUIDSyntax = errors.New("ids: invalid uuid syntax")

	// ErrTimeUnset reports a TimeSource returning an instant before the Unix
	// epoch, in practice the zero time.Time. A UUIDv7 timestamp is an unsigned
	// 48-bit millisecond count and has nowhere to put a negative one.
	ErrTimeUnset = errors.New("ids: time source is before the unix epoch")

	// ErrTimeRange reports an instant past the 48-bit millisecond ceiling —
	// year 10889. Unreachable in practice and checked anyway, because the
	// alternative is silently truncating the high bits and minting an id that
	// sorts before every id already issued.
	ErrTimeRange = errors.New("ids: instant exceeds the 48-bit millisecond range")

	// ErrEntropy reports a failed read from the injected entropy source.
	ErrEntropy = errors.New("ids: entropy source failed")

	// ErrBase32Syntax reports text outside the strict Crockford alphabet this
	// package encodes with. See decodeBase32 for why the decoder is strict
	// where Crockford's own specification is lenient.
	ErrBase32Syntax = errors.New("ids: invalid base32 syntax")

	// ErrBase32Overflow reports 26 base32 characters that do not fit in 128
	// bits. 26 characters carry 130 bits, so the two spare high bits must be
	// zero; text where they are not is not something this package encoded.
	ErrBase32Overflow = errors.New("ids: base32 value exceeds 128 bits")

	// ErrClientOrderIDSyntax reports text that is not one of our derived
	// client order ids — including, deliberately, the venue's own system ids.
	ErrClientOrderIDSyntax = errors.New("ids: invalid client order id syntax")

	// ErrLegRange and ErrAttemptRange report a leg or attempt outside the two
	// decimal digits the id format allocates. They are errors rather than
	// silent widening because the format has a hard venue-side length budget:
	// widening the field would produce an id the venue rejects, at submission
	// time, on a path that only runs when something has already gone wrong.
	ErrLegRange     = errors.New("ids: leg outside 0..99")
	ErrAttemptRange = errors.New("ids: attempt outside 0..99")
)

// Venue identifies an exchange.
//
// It lived in platform/clock until this package landed — clock's §3.5 signature
// ServerNow(Venue) needed it and the shared kernel may not import a bounded
// context (§3.6 rule 9), so it went to the only place that existed. It belongs
// here: config, ratelimit, oms and engine/venue will all need to name a venue
// without wanting a clock.
//
// It is a key rather than an enum-like, so §3.6 rule 8's declared-set shape does
// not apply — the same reasoning clock.TimerID records. The set of venues is
// open and comes from configuration; Validate can only check that somebody set
// it, and an empty one reaching a calibration lookup is a wiring mistake with a
// silent answer otherwise.
type Venue string

func (v Venue) Validate() error {
	if v == "" {
		return ErrVenueUnset
	}
	return nil
}

func (v Venue) String() string { return string(v) }
