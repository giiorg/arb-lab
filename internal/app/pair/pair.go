// Package pair is the composition root for a cross-venue perp-perp package:
// two single-venue OMS deployments, two journals, two boots, and ONE trade
// spanning them.
//
// # The concept this file names, and why it is not in the kernel
//
// Record §95 §4 found that §6.7's classifier cannot see a cross-venue pair:
// domain.HedgeGroup is (Account, Venue, Symbol), the classifier computes its
// residual per hedge group, and a cross-venue pair sits in two groups with no
// group holding both. Inside each venue's own group a correctly-hedged pair
// reads as a filled leg with no pending counter-order — §6 step 7's DETERMINED
// residual, the branch that compensates within five seconds. Driven naively the
// safety kernel would flatten both halves of a healthy position, once per
// venue, in opposite directions.
//
// Ruling 2 places the fix in a NEW concept ABOVE the hedge group and leaves
// §5.1's key alone: the venue in that key is correct for margin, liquidation
// and ADL, and §5.1 keys remedy episodes by hedge group so §7.1's whole ladder
// rides the same object. What is actually needed is not a merged residual but
// the fact that A LEG HAS A PARTNER ELSEWHERE. That fact is what this file
// holds, and ruling 3 puts the normative edit to §5.1/§6.7 in task #241, not
// here.
//
// # What that costs, stated rather than left to be discovered
//
// This root COMPOSES two per-venue readings and refuses to conclude on any it
// could not establish. It does not make the pair a kernel concept, so nothing
// in the safety kernel knows about it, and §6.7 stays the single gate for every
// compensating trade that goes through the kernel. The one compensating trade
// this root issues — the unwind — is therefore deliberately narrow: it runs on
// exactly one verdict, and every other reading is handed to a human. See
// ErrUnwindRefused.
package pair

import (
	"fmt"

	"github.com/giiorg/arb-lab/internal/oms"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// JoinFacts is what one side's join reported about one leg: the four lists that
// mean "this reading is not finished".
//
// They are passed in rather than read here because the join is I/O and this
// file is not. Each is TRUE when the leg's id appeared in that list.
//
// # Four rather than three, and the fourth arrived after this plan was written
//
// Task #242 added Contradicted to application.JoinReport, and the plan's D4 was
// corrected for it rather than left to be discovered: a three-field struct here
// would have compiled, passed, and read a leg the join had latched
// boot.OutcomeNeedsOperator over as cleanly established. That is this repo's
// count-word shape — an enumeration that was total when it was written and is
// not now.
type JoinFacts struct {
	// Absent: the venue says it does not hold the order. Not a failure and not
	// a resolution — §5.3's stable-not-found rule decides what an absence
	// licenses, through four gates this root does not implement.
	Absent bool

	// Incomplete: the executions sweep did not prove coverage through the
	// instant it was asked about. A truncated sweep's silence is not evidence.
	Incomplete bool

	// Unattributed: the venue's own cumulative disagrees with what the join
	// could admit. An execution exists that was not attributed to this order,
	// and the exposure it represents is real whether or not its row can be
	// named.
	Unattributed bool

	// Contradicted: the venue's lifecycle reading and this system's are two
	// well-formed states SM §1 has no edge between (task #242). The join
	// latches boot.OutcomeNeedsOperator over it, so a leg carrying it can never
	// be read as established here — the two authorities disagree about what
	// happened to the order, which is the last reading from which to conclude a
	// quantity.
	Contradicted bool
}

// LegExposure is one side's leg as this root reads it.
type LegExposure struct {
	Venue      ids.Venue
	ID         ids.ClientOrderID
	Instrument ids.VenueInstrument
	Side       domain.Side
	State      domain.OrderState

	// Filled is the established executed quantity, meaningful ONLY when
	// Established is true.
	Filled fixedpoint.Quantity

	// Established reports whether Filled is a measurement. When false, Filled
	// is whatever the projection happened to hold and MUST NOT be read: a zero
	// cumulative on a leg that has not reached a terminal state means "nothing
	// attributed yet", which is not "nothing filled", and the difference is a
	// naked position reported as a clean run.
	Established bool

	// Because names why Established is false, for the operator's report.
	Because string
}

// Establish is the three-valued reading D4 requires: a leg's exposure counts as
// measured only when the order is terminal for STATE and the join reported none
// of its unfinished-reading facts about it.
//
// The direction is the whole point. Every one of these conditions, taken the
// other way, produces a confident zero — and a confident zero on a leg that
// actually filled is the reading that leaves naked exposure looking clean.
func Establish(venue ids.Venue, id ids.ClientOrderID, st oms.LegState, joined JoinFacts) LegExposure {
	e := LegExposure{
		Venue: venue, ID: id,
		Instrument: st.Instrument, Side: st.Side, State: st.State,
		Filled: st.Cumulative,
	}
	terminal, err := st.State.IsTerminal()
	switch {
	case err != nil:
		e.Because = fmt.Sprintf("state %q: %v", st.State, err)
	case !terminal:
		e.Because = fmt.Sprintf("state %q is not terminal", st.State)
	case joined.Absent:
		e.Because = "the venue does not hold this order; §5.3's absence rule has not been run"
	case joined.Incomplete:
		e.Because = "the executions sweep did not prove coverage"
	case joined.Unattributed:
		e.Because = "the venue's cumulative disagrees with the attributed fills"
	case joined.Contradicted:
		e.Because = "the venue's lifecycle state and this system's have no edge between them"
	default:
		e.Established = true
	}
	// # What guards a FIFTH finding class, since nothing here can
	//
	// A field added to JoinFacts and forgotten in the switch above falls through
	// to established, and no compiler notices: it is one more bool. The obvious
	// defence — a `joined.unfinished()` backstop below the switch — was written
	// and then removed, because it is unreachable while every field has an arm,
	// so `tools/mutate.sh` replacing it with `if false` SURVIVES. A branch no
	// test can kill is this repo's unreachable-sentinel shape: it reads as
	// handled and checks nothing.
	//
	// The guard is a REFLECTIVE test over this struct's own fields instead —
	// cmd/arblab's TestReconcileReportRendersEveryJoinFindingList, one layer up,
	// on the same finding roster. See TestEstablishNamesEveryJoinFact.
	return e
}

// Verdict is the pair's classification — the concept record §95 ruling 2 places
// above the hedge group.
//
// It is NOT §6.7's verdict and must not be confused with one. §6.7 answers
// "determined or ambiguous" about ONE hedge group's residual and authorizes a
// compensating magnitude. This answers "what does this cross-venue pair look
// like from outside both groups", and authorizes nothing on its own.
type Verdict uint8

const (
	// VerdictUnopened : neither leg carries exposure. The IOC-did-not-fill
	// case, and a real result — it proves both boots, both wire contracts and
	// both journal paths, and proves nothing about fills.
	VerdictUnopened Verdict = iota

	// VerdictIntact : both legs carry the same base quantity. The pair is
	// hedged, which is the outcome the strategy wants.
	VerdictIntact

	// VerdictHalfOpen : exactly one leg carries exposure, and its magnitude is
	// established. This is the one reading this root will act on.
	VerdictHalfOpen

	// VerdictSkewed : both legs carry exposure and the quantities differ. The
	// pair is partially hedged; the residual is real and its remedy is a
	// compensating trade on one side, which is §6.7's gate's business and not
	// this root's (record §95 §4). An operator matter.
	VerdictSkewed

	// VerdictIndeterminate : at least one leg's exposure was not established.
	// It OUTRANKS every other reading, including one that would otherwise look
	// intact, because an unestablished leg's quantity is not a measurement and
	// a rule that let the other side decide would conclude on it.
	VerdictIndeterminate
)

// String implements fmt.Stringer.
func (v Verdict) String() string {
	switch v {
	case VerdictUnopened:
		return "VerdictUnopened"
	case VerdictIntact:
		return "VerdictIntact"
	case VerdictHalfOpen:
		return "VerdictHalfOpen"
	case VerdictSkewed:
		return "VerdictSkewed"
	case VerdictIndeterminate:
		return "VerdictIndeterminate"
	}
	return fmt.Sprintf("Verdict(%d)", uint8(v))
}

// Exposed reports whether this verdict leaves exposure a human must account
// for. It is what the exit class is derived from.
func Exposed(v Verdict) bool {
	switch v {
	case VerdictUnopened, VerdictIntact:
		return false
	case VerdictHalfOpen, VerdictSkewed, VerdictIndeterminate:
		return true
	}
	// Named rather than defaulted: a member added above must choose a side
	// here, and choosing "not exposed" by falling through is how a real
	// exposure becomes invisible. exhaustiveswitch enforces the naming; this
	// line is only the compiler's return.
	return true
}

// Compose is the pair's composition rule.
//
// # Indeterminate is checked first, and the order is the safety argument
//
// Every other arm reads a quantity, and a quantity from an unestablished leg is
// not a number this system measured.
//
// # It is fallible because fixedpoint made the same decision one layer down
//
// Quantity.Sign is fallible so that "a flat position and a position nobody
// populated" cannot be confused, and at this layer that confusion is a naked
// position reading as no position. An unreadable quantity therefore returns
// VerdictIndeterminate WITH the error: fail-closed, and the caller has a reason
// to print rather than a bare verdict.
//
// # It repeats the opposite-side premise
//
// Config.Validate is not this function's only possible caller. Two equal buys
// compare equal just as two opposite fills do, so assuming the config check
// here turns a directional package into VerdictIntact.
func Compose(a, b LegExposure) (Verdict, error) {
	if !a.Established || !b.Established {
		return VerdictIndeterminate, nil
	}
	// Validate refuses same-side documents, and Compose repeats that premise
	// rather than inheriting one caller's discipline. Without it two equal buys
	// compare equal and become VerdictIntact — a same-direction package reported
	// as a hedge.
	//
	// It is checked AFTER the Established guard and not before, because Side is
	// a field an UNESTABLISHED reading may not carry: drive.establish's error
	// path returns a LegExposure holding only Venue, ID and Because, so its Side
	// is the zero value. Two such legs would compare equal and report a document
	// defect in place of the unfinished reading that actually stopped the run —
	// a wrong diagnosis on the exact path D4 exists to keep honest.
	if a.Side == b.Side {
		return VerdictIndeterminate, fmt.Errorf("pair legs are both %s: %w", a.Side, ErrLegSideSame)
	}
	aSign, err := a.Filled.Sign()
	if err != nil {
		return VerdictIndeterminate, fmt.Errorf("side %s filled quantity: %w", a.Venue, err)
	}
	bSign, err := b.Filled.Sign()
	if err != nil {
		return VerdictIndeterminate, fmt.Errorf("side %s filled quantity: %w", b.Venue, err)
	}
	aOpen, bOpen := aSign != 0, bSign != 0
	switch {
	case !aOpen && !bOpen:
		return VerdictUnopened, nil
	case aOpen != bOpen:
		return VerdictHalfOpen, nil
	}
	// Cmp is the asset-tag check as well as the comparison: two legs whose bases
	// disagree cannot be compared, and Config.Validate refuses that document at
	// load. Checked here anyway rather than assumed — the config refusal is one
	// caller's discipline, and this function is reached from two modes.
	cmp, err := a.Filled.Cmp(b.Filled)
	if err != nil {
		return VerdictIndeterminate, fmt.Errorf("comparing %s and %s fills: %w", a.Venue, b.Venue, err)
	}
	if cmp == 0 {
		return VerdictIntact, nil
	}
	return VerdictSkewed, nil
}
