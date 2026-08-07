package domain

import "errors"

// The risk domain's refusals. Each is a sentinel so a caller can branch on the
// kind without matching text; every construction site wraps it with the value
// that failed, because one sentinel cannot say WHICH setting or symbol fired.
//
// Note what is NOT here: a sentinel for "the posture is wrong". A posture
// violation is a Verdict with named Failures — data an operator reads and acts
// on — never an error, because an error collapses eleven checks into one string
// and the first-failure-only checker is a shape this repo has already paid for
// (a config listing two claims checks only the shallower).
var (
	// ErrSettingInvalid, ErrScopeInvalid and ErrPresenceInvalid report an
	// observation field that is the zero value or is not a declared member.
	//
	// Three sentinels rather than one, on oms/domain's argument: they send a
	// reader to different files. A bad setting is an ACL translating a venue
	// concern this context does not name; a bad scope is an ACL that lost the
	// subject dimension; a bad presence is an observation nobody stamped —
	// which, because the zero value is refused, is the one a hand-built fixture
	// produces by omission.
	ErrSettingInvalid  = errors.New("risk: posture setting is not a declared value")
	ErrScopeInvalid    = errors.New("risk: posture scope is not a declared value")
	ErrPresenceInvalid = errors.New("risk: posture presence is not a declared value")

	// ErrSubjectInvalid reports a RULE ROW whose subject is the zero value or is
	// not a declared member. Unlike the three above it, this is never an ACL
	// fault and never an account fault: the rule tables are this package's own
	// data, so it is a defect in the table and it is raised by the audit that
	// reads the table rather than by anything on the wire path (record §73p).
	ErrSubjectInvalid = errors.New("risk: posture rule subject is not a declared value")

	// ErrVenueReadOwed reports CheckWithoutVenueRead called for a requirement
	// whose table DOES have settings to read.
	//
	// The precondition it guards is the only thing standing between a legitimate
	// no-read verdict and a general-purpose way to judge any account without
	// looking at it — on Bybit that would drop eight venue checks and report a
	// clean posture. It names the count and the coverage, because the caller's
	// bug is always that one of those two is not what they assumed.
	ErrVenueReadOwed = errors.New("risk: this requirement owes a venue read and may not be judged without one")

	// ErrFailureReasonInvalid reports a Failure carrying no declared reason.
	// A failure that cannot say why is one an operator cannot act on.
	ErrFailureReasonInvalid = errors.New("risk: failure reason is not a declared value")

	// ErrAttestationInvalid reports an attestation value that is not a
	// declared member. The UNSTATED zero is deliberately valid: "no operator
	// has attested" is the ordinary state of a fresh deployment, and it
	// produces a named Failure rather than a configuration error.
	ErrAttestationInvalid = errors.New("risk: attestation is not a declared value")

	// ErrDialectUnknown reports a posture dialect this package has no rule
	// table for.
	//
	// It is an ERROR and never an empty rule set, which is the whole reason it
	// exists: a dialect with no rules evaluates nothing, finds no failures and
	// reports a clean posture — the vacuous pass, arriving through the one
	// field that selects which account model is being checked.
	ErrDialectUnknown = errors.New("risk: posture dialect has no rule table")

	// ErrObservationIncoherent reports an observation whose fields contradict
	// each other — a subject on an account-wide setting, a missing subject on
	// a per-symbol one, or a value carried beside a presence that says there
	// was none. Whichever field a rule reads, the other says otherwise, and the
	// permissive reading is always the one that reads the value.
	ErrObservationIncoherent = errors.New("risk: posture observation contradicts itself")

	// ErrObservationDuplicate reports two observations on one
	// (setting, scope, subject) key.
	//
	// Refused rather than folded even when the two agree: folding needs a rule
	// for which one wins, any such rule is invented here rather than published
	// by the venue, and the case where they DISAGREE is exactly the posture
	// violation being hunted (Bybit returns one position row per side in hedge
	// mode).
	ErrObservationDuplicate = errors.New("risk: posture report states one setting twice")

	// ErrReportUnusable reports a posture report that cannot be judged at all —
	// no venue, or no observed-at instant.
	ErrReportUnusable = errors.New("risk: posture report is unusable")

	// ErrRosterEmpty reports a requirement that claims to trade derivatives and
	// then lists none.
	//
	// The empty roster is the vacuity this whole file is written against: every
	// per-symbol rule folds over it, finds nothing to check, and reports a
	// clean posture for an account nobody looked at.
	//
	// It fires on the CARRIER's word rather than on `len(Symbols)` alone, and
	// the distinction is #198 item 6's finding: a deployment configuring only
	// spot instruments derives an empty roster legitimately, and §5.2's venue
	// block being singular makes that the ordinary shape of a cross-venue
	// carry's spot half. See RosterCoverage.
	ErrRosterEmpty = errors.New("risk: posture requirement trades derivatives and configures no symbols")

	// ErrRosterCoverageInvalid reports a requirement whose roster coverage is
	// the unspecified zero or an undeclared value.
	//
	// The zero is refused rather than read as either meaning, because a
	// Requirement built without thinking about coverage must not get the
	// permissive one by default.
	ErrRosterCoverageInvalid = errors.New("risk: posture requirement roster coverage is not a declared value")

	// ErrRosterCoverageDisagrees reports a requirement stating that no
	// derivative is configured while listing symbols.
	//
	// The mirror refusal of ErrRosterEmpty, and it is the one that keeps the
	// carrier from becoming a flag nobody checks: believing it over the slice
	// would silently drop every per-symbol rule for symbols the caller supplied.
	ErrRosterCoverageDisagrees = errors.New("risk: posture requirement roster coverage disagrees with its symbols")

	// ErrRosterDuplicate reports one symbol configured twice — two pins for one
	// symbol, where the one that loses is decided by iteration order.
	ErrRosterDuplicate = errors.New("risk: posture requirement names a symbol twice")

	// ErrSymbolUnset reports a roster row with no symbol.
	ErrSymbolUnset = errors.New("risk: posture requirement has a roster row with no symbol")

	// ErrLeveragePinUnset reports a roster row whose leverage pin is not
	// positive.
	//
	// §3.6 rule 4 in its most literal form: an unpinned leverage reads as "any
	// leverage is acceptable", which computes the safety gate in our favour
	// with nothing failing. The pin is operator-declared and stated
	// independently of what the venue reports — deriving it from the venue at
	// first boot compares the venue against itself (record §73b F3).
	ErrLeveragePinUnset = errors.New("risk: posture requirement has a roster row with no leverage pin")

	// ErrAttestationNotConsumed reports an attestation pinned on a dialect
	// whose rule table has no check that reads it.
	//
	// Refused on task #179's precedent — a spot baseline pinned on a spotless
	// deployment is refused because a document nothing consumes is an unchecked
	// claim, and an operator who pinned one believes a check is running.
	ErrAttestationNotConsumed = errors.New("risk: attestation pinned where no rule consumes it")

	// ErrVenueMismatch reports a report and a requirement naming different
	// venues — one exchange's answer judged against another's requirements,
	// which is a wiring mistake whose every individual check still evaluates.
	ErrVenueMismatch = errors.New("risk: posture report and requirement name different venues")

	// ErrVerdictVacuous reports a verdict that evaluated no checks being asked
	// whether it is satisfied.
	//
	// "No failures" and "nothing was checked" are the same zero value, and the
	// second must never read as the first. This is the guard that makes
	// Verdict.Satisfied answerable rather than merely true.
	ErrVerdictVacuous = errors.New("risk: verdict evaluated no checks")
)
