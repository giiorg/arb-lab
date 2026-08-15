package pair

import "errors"

var (
	// ErrPairFieldUnset and ErrPairFieldInvalid report a pair block this root
	// cannot drive — a missing field, and one that parses to no declared
	// value. Kept distinct on reconcile's ErrConfigFieldUnset precedent: the
	// operator's fix differs.
	ErrPairFieldUnset   = errors.New("pair: pair field is unset")
	ErrPairFieldInvalid = errors.New("pair: pair field is invalid")

	// ErrSideCount reports a document that does not describe exactly two
	// sides. One side is the exercise binary; three is a shape nothing in this
	// root's leg-index argument (D2) covers.
	ErrSideCount = errors.New("pair: exactly two sides are required")

	// ErrSideVenueDuplicate reports two sides naming the same venue.
	//
	// It is not a tidiness rule. Both legs plan under ONE TradeID at leg index
	// 0, so they mint the SAME ClientOrderID — safe only because no store spans
	// venues and the venue is what tells the two apart. Two sides on one venue
	// removes the disambiguator and puts two different orders under one id on
	// one venue's order page.
	ErrSideVenueDuplicate = errors.New("pair: both sides name the same venue")

	// ErrSideJournalShared reports two sides pointing at one journal path.
	// §5.1's journal is a single-writer store; two oms.Modules over one file is
	// two writers, and the second half of ErrSideVenueDuplicate's argument —
	// with one journal the two identical ids DO meet.
	//
	// Comparing the configured strings was the shallower check: it missed a
	// relative/absolute alias, a symlinked parent and an existing hard link, all
	// of which put the identical ids in one physical SQLite chain.
	ErrSideJournalShared = errors.New("pair: both sides name the same journal path")

	// ErrLegCount reports a pair that is not exactly one leg per side (D3).
	ErrLegCount = errors.New("pair: exactly one leg per side is required")

	// ErrLegVenueUnknown reports a leg naming a venue no side declares.
	ErrLegVenueUnknown = errors.New("pair: leg names a venue no side declares")

	// ErrLegInstrumentUnknown reports a leg naming an instrument its own side's
	// instrument table does not hold. Refused at Validate rather than at
	// planning, because the row is where the base and quote assets come from.
	ErrLegInstrumentUnknown = errors.New("pair: leg names an instrument its side does not declare")

	// ErrLegBaseMismatch reports two legs whose base assets differ. The pair's
	// whole arithmetic — equal filled quantities means intact — is a comparison
	// of two fixedpoint.Quantity values, which is a runtime asset-tag check;
	// refusing here names the config field instead of failing mid-run.
	ErrLegBaseMismatch = errors.New("pair: the two legs do not share a base asset")

	// ErrLegSideSame reports two legs on the same side of the market. A pair
	// whose legs are both buys is not a hedge, and every verdict below would
	// report it intact.
	ErrLegSideSame = errors.New("pair: the two legs are on the same side of the market")

	// ErrReferenceFieldUnset, ErrReferenceUnreadable, ErrFilterTickUnrecorded
	// and ErrFilterRowStale are exercise's four, restated for this root's own
	// message prefix. Their arguments are unchanged — see
	// internal/app/exercise/errors.go, which states each in full.
	ErrReferenceFieldUnset  = errors.New("pair: reference field is unset")
	ErrReferenceUnreadable  = errors.New("pair: reference revision could not be read")
	ErrFilterTickUnrecorded = errors.New("pair: reference row records no tick, which the order path verifies against")
	ErrFilterRowStale       = errors.New("pair: reference row is older than reference.max_age")

	// ErrBootNotClean reports a drive declined because a side's boot verdict was
	// not ExitClean. Both sides must be clean before either is touched: a
	// dispatch that began against one clean boot and one unknown one is the
	// half-open pair this root exists to avoid producing on purpose.
	ErrBootNotClean = errors.New("pair: a side's boot verdict does not admit execution")

	// ErrPositionNotFlat reports a clean boot whose venue still holds a position
	// on one of this pair's instruments. This root's only compensating trade is
	// a reduce-only inverse; it is legal as restoration only when the opening
	// leg created the entire position it later closes.
	ErrPositionNotFlat = errors.New("pair: a leg's venue position is not flat before execution")

	// ErrHedgeUnsizable reports leg A's established fill failing leg B's own
	// §2.4 filters — below B's min quantity or min notional, or off its lot
	// grid. Nothing is sent on B and the run unwinds A (D5).
	ErrHedgeUnsizable = errors.New("pair: side A's fill cannot be expressed as a leg on side B")

	// ErrUnwindRefused reports an unwind this root declined to attempt. A
	// compensating trade is §6.7's gate's business and §6.7 cannot see a
	// cross-venue pair (record §95 §4), so this root compensates only on the
	// one reading it fully established: a HalfOpen pair whose exposed side is
	// side A. Every other reading ends at ExitExposed, for a human.
	ErrUnwindRefused = errors.New("pair: the pair's verdict does not admit an automatic unwind")
)
