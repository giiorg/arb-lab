package domain

import (
	"fmt"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// This file is the SECOND half of core §7.1's margin-op evidence contract:
// Binance's `positionMargin/history`, the venue's own record of margin-change
// ACTS, beside the posted-margin delta task #212 built.
//
// # The two sources do not vote — they answer different questions
//
// §7.1 lists Binance's evidence as the history match "plus the position-risk
// margin delta", and record §81 §3b ruled that the "plus" is NOT a conjunction.
// The reason is that the two sources have different confounds. A history row
// records an ACT: the venue is stating that a margin adjustment of this amount
// happened on this symbol at this time. The posted delta is a LIVE QUANTITY, and
// every fill on the position moves it. Requiring the two to agree would let the
// more-confounded source veto the less-confounded one, so an ordinary partial
// fill inside the window — which moves the delta and leaves the act log alone —
// would become a §9 operator incident.
//
// The reading that gives each source a job it is actually good at, and the one
// this file implements:
//
//	the history is the ONLY thing that can conclude Applied
//	the delta is the ONLY thing that can conclude NotApplied
//	the two disagreeing, or either one speaking twice, is Ambiguous
//
// A venue that publishes no act history at all falls back to the delta for both
// verdicts, which is exactly the contract task #212 built and is still the whole
// contract on Bybit.
//
// # "No source" and "the source said nothing" are DIFFERENT, and collapsing them
// # inverts a verdict
//
// This is the distinction MarginActHistory exists to make unforgeable. Take one
// observation — the position's posted margin moved by exactly the op's delta —
// and ask each venue:
//
//	Bybit    no act history exists anywhere      -> Applied
//	Binance  the act history returned no rows    -> Ambiguous
//
// Opposite verdicts on identical arithmetic, because on Binance the venue has
// been asked whether it performed the act and has said it did not, while the
// margin moved by that amount anyway — a second mover. On Bybit nobody was
// asked, so the delta is all there is and it is the contract §7.1 states.
//
// A zero value that read as "no rows" would therefore judge every Bybit op by a
// rule written for a venue that answers. That is why the state is a three-member
// unexported enum whose zero is UNSET and refused, rather than a `[]MarginAct`
// whose nil is quietly the empty case. Profiles §4 is what puts Bybit in the
// no-source arm and it is not a gap to be filled later: its transaction-log
// `type` enum has no value for a manual position-margin adjustment, and whether
// a top-up emits a record at all is undocumented and MUST NOT be relied on.

// MarginActCause is why the venue says a margin change happened.
//
// Binance publishes it as `deltaType` on every `positionMargin/history` row, and
// this system needs it because it already knows margin can move for a second
// reason: §4.1's posture check reads `isAutoAddMargin` per configured symbol
// precisely because the venue can add margin on its own. A rule that matched on
// symbol + type + amount + window alone would read such a row as evidence of
// THIS system's act.
//
// It has two members and the split is deliberately not an enumeration of
// Binance's vocabulary — see MarginActUnattributable.
type MarginActCause string

const (
	// MarginActCauseUnset is the zero value and names nothing. Validate refuses
	// it, so a row that reached this system with no cause decoded cannot be
	// silently counted as one this system caused.
	MarginActCauseUnset MarginActCause = ""

	// MarginActUserAdjust is a manual position-margin adjustment — the act this
	// system performs, Binance `deltaType: "USER_ADJUST"`.
	MarginActUserAdjust MarginActCause = "user_adjust"

	// MarginActUnattributable is every other cause, INCLUDING one this system
	// does not recognise.
	//
	// # An unrecognised value is loud, because here the silent direction is also
	// # the unsafe one
	//
	// The tempting shape is to drop a row whose cause is not recognised, which
	// makes the history silent, which falls through to the delta. That reading is
	// wrong for `platform/config`'s reason, inverted: config denies an unknown
	// tag by ERRORING rather than by silently denying, "because there the safe
	// direction is also the silent one". Here it is the same trap. If Binance
	// adds a `deltaType`, renames one, or starts reporting our own adjustments
	// under a new value, silently dropping the row hides a venue act behind a
	// verdict computed from the weaker source, with nothing anywhere reporting
	// that a row was seen and discarded.
	//
	// So an unrecognised cause is carried as this member, and a matching row
	// bearing it resolves Ambiguous: the venue recorded an act inside this op's
	// window that this system cannot attribute, which is precisely §7.1's
	// "coincident manual adjustment" case reached from the other direction.
	//
	// The enumeration of Binance's OTHER `deltaType` values is not verified — the
	// spec-generated connector's own mocked response is the only verbatim shape
	// this repo holds (Profiles §4), and it carries one value. Rather than guess
	// the rest, everything that is not the one verified value lands here, which
	// is conservative under every reading of what the missing values are.
	MarginActUnattributable MarginActCause = "unattributable"
)

// Validate refuses the zero value and any undeclared cause.
func (c MarginActCause) Validate() error {
	switch c {
	case MarginActUserAdjust, MarginActUnattributable:
		return nil
	case MarginActCauseUnset:
		return ErrMarginActCauseUnset
	}
	return fmt.Errorf("%q: %w", string(c), ErrMarginActCauseInvalid)
}

// String implements fmt.Stringer.
func (c MarginActCause) String() string { return string(c) }

// MarginAct is one row of the venue's margin-change history, normalized.
//
// The venues supply NO identity on these rows (Profiles §4: "records have no
// unique id"), which is the whole reason MarginOp exists and is why every field
// here is part of a match rather than a lookup key.
type MarginAct struct {
	// Amount is the magnitude of the change the venue recorded, always
	// non-negative — Binance publishes `amount` unsigned and carries the
	// direction in `type`.
	//
	// It is a magnitude rather than a signed figure so that the comparison
	// against a MarginOp's SIGNED delta has one obvious spelling. Matching a
	// signed row against a signed delta would agree with an add-vs-reduce
	// confusion at the decoder, because the fixture amounts a person writes are
	// positive and the sign would come from the same field twice.
	Amount fixedpoint.Money

	// Adds is the direction: true for a top-up (Binance `type` 1), false for a
	// reduction (`type` 2).
	//
	// Beside the magnitude rather than folded into its sign because the venue
	// publishes it as its own field, and a decoder that folded them would make a
	// wrong `type` reading indistinguishable from a wrong `amount` reading.
	Adds bool

	// At is the venue's timestamp on the row. VENUE time — the same clock
	// MarginOp.Watermark is on, which is what makes the window comparable.
	At time.Time

	// Cause is why the venue says the change happened.
	Cause MarginActCause
}

// Validate reports whether the row is one a match may be performed against.
func (a MarginAct) Validate() error {
	if err := a.Amount.Validate(); err != nil {
		return fmt.Errorf("margin act amount: %w", err)
	}
	sign, err := a.Amount.Sign()
	if err != nil {
		return fmt.Errorf("margin act amount: %w", err)
	}
	if sign <= 0 {
		// Zero is refused beside negative, and not only for tidiness: a MarginOp
		// with a zero delta is already refused because Applied and NotApplied are
		// indistinguishable under it, and a zero-amount row would match such an
		// op's magnitude if one ever reached here.
		return fmt.Errorf("margin act amount is %s: %w", a.Amount, ErrMarginActAmountNotPositive)
	}
	if a.At.IsZero() {
		// A row with no timestamp cannot be placed inside or outside a window, so
		// it would match every op on the symbol.
		return ErrMarginActInstantUnset
	}
	return a.Cause.Validate()
}

// marginHistoryState is whether a venue publishes an act history, and if so
// whether this reading holds its answer. Unexported: the three cases are
// constructed through the two functions below, so no caller can mint the pairing
// that inverts a verdict.
type marginHistoryState int

const (
	marginHistoryUnset marginHistoryState = iota
	marginHistoryAbsent
	marginHistoryPublished
)

// MarginActHistory is what the venue's act history says about one op's window.
//
// Three states, and the file comment above is the argument for why the third
// exists rather than being spelled "an empty slice":
//
//	UNSET      nobody populated this — a wiring defect, refused by Resolve
//	ABSENT     this venue publishes no act history (Bybit, Profiles §4)
//	PUBLISHED  the venue answered; the rows are its answer, possibly none
//
// The zero value is UNSET and unusable, which is the repo's standing rule doing
// real work rather than ceremony: the value a forgetful caller produces must not
// be the one that reads as "Binance answered and recorded nothing", because that
// is the reading under which a correct top-up resolves Ambiguous and blocks
// openings until an operator clears it.
type MarginActHistory struct {
	state marginHistoryState
	rows  []MarginAct
}

// NoMarginActHistory is the reading for a venue that publishes no usable
// margin-act history.
//
// Bybit, and it is a RULING rather than an unimplemented adapter: Profiles §4
// records that the V5 transaction-log's ~100-value `type` enum contains no value
// for a manual position-margin add or reduce, and that whether a top-up emits a
// record at all is undocumented and MUST NOT be relied on. Building a matcher on
// it would be building on a fact the venue does not state.
func NoMarginActHistory() MarginActHistory {
	return MarginActHistory{state: marginHistoryAbsent}
}

// PublishedMarginActHistory is the reading for a venue that answered.
//
// rows is every row the venue returned for the query, unfiltered: the matching
// rule is this package's, so that it is single-sourced and testable without a
// venue, and so that a row the venue returned but the match rejected is rejected
// by a rule with a name rather than by a query parameter.
//
// An empty rows is a legitimate and meaningful answer — the venue was asked and
// recorded nothing — which is exactly the answer NoMarginActHistory must not be
// confused with.
func PublishedMarginActHistory(rows []MarginAct) MarginActHistory {
	return MarginActHistory{state: marginHistoryPublished, rows: rows}
}

// Published reports whether this venue publishes an act history at all.
func (h MarginActHistory) Published() bool { return h.state == marginHistoryPublished }

// Validate refuses the unset reading and any malformed row.
func (h MarginActHistory) Validate() error {
	switch h.state {
	case marginHistoryAbsent:
		if len(h.rows) != 0 {
			// Unreachable through the constructors and checked anyway, because
			// the rows of an absent history would be silently ignored by every
			// rule below — the shape a struct literal built by a later caller
			// would take.
			return ErrMarginHistoryAbsentWithRows
		}
		return nil
	case marginHistoryPublished:
		for i, row := range h.rows {
			if err := row.Validate(); err != nil {
				return fmt.Errorf("margin act history row %d: %w", i, err)
			}
		}
		return nil
	case marginHistoryUnset:
		return ErrMarginHistoryUnset
	}
	return fmt.Errorf("%d: %w", h.state, ErrMarginHistoryStateInvalid)
}

// matches returns the rows that could be the act this op requested.
//
// # The match is symbol + direction + magnitude + window, and the symbol is not
// # checked here
//
// §7.1 states the match as "symbol + type + amount + time-window". The symbol is
// the one component this rule does NOT re-check, because a history reading is
// fetched per instrument and the adapter refuses an answer for a different one —
// the same placement VenueMarginStatus uses for the same reason. Re-checking it
// here would need the row to carry a symbol whose only source is the query that
// already constrained it, which is a check fed by the thing it checks.
//
// # The window shares MarginOp's anchor and the resolution's bound
//
// [Watermark, Watermark+bound). The lower edge is closed because the watermark is
// read immediately BEFORE the call, so an act at that instant is this op's; the
// upper edge is open for the same reason Resolve's elapsed comparison is `<` —
// a bound states how long the venue may still act, and at its far edge it may no
// longer. One bound serving both uses is deliberate: a separate window duration
// would be a second assumption about one transport, free to drift from the first.
//
// Both edges fail toward silence, and silence is the conservative direction: a
// row wrongly excluded makes the history say nothing, which resolves NotApplied
// or Ambiguous but never Applied.
func (h MarginActHistory) matches(op MarginOp, bound time.Duration) ([]MarginAct, error) {
	adds, err := op.Adds()
	if err != nil {
		return nil, err
	}
	want, err := op.Delta.Abs()
	if err != nil {
		return nil, fmt.Errorf("margin op %q delta: %w", op.ID, err)
	}

	opens := op.Watermark
	closes := op.Watermark.Add(bound)

	var out []MarginAct
	for _, row := range h.rows {
		if row.Adds != adds {
			continue
		}
		if row.At.Before(opens) || !row.At.Before(closes) {
			continue
		}
		cmp, err := row.Amount.Cmp(want)
		if err != nil {
			return nil, fmt.Errorf("margin op %q: %w", op.ID, err)
		}
		if cmp != 0 {
			continue
		}
		out = append(out, row)
	}
	return out, nil
}
