package pair

import (
	"fmt"

	"github.com/giiorg/arb-lab/internal/app/reconcile"
	"github.com/giiorg/arb-lab/internal/oms/application"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// SideReport is everything one side did.
type SideReport struct {
	// Venue is which exchange this side is.
	Venue ids.Venue

	// Boot is §7.5's verdict for this side, taken once, before anything was
	// driven — the value the gate reads.
	Boot reconcile.Report

	// RateLimit is refreshed AFTER all drive and unwind traffic, so it reports
	// what the run actually spent. Boot keeps the pre-drive snapshot the boot
	// precedence is derived from, and the two are separate fields because a
	// single refreshed one would silently restate the boot's verdict inputs.
	RateLimit ratelimit.Snapshot

	// Open and Unwind are this side's two possible orders. A NIL pointer means
	// "not attempted" — there is no second boolean saying so, because a boolean
	// with no independent state can only ever disagree with the pointer.
	Open   *LegReport
	Unwind *LegReport

	// Exposure is what this root established about this side's opening leg.
	Exposure LegExposure
}

// LegReport is one dispatched leg.
type LegReport struct {
	// ID is the ClientOrderID this leg planned under. Both sides plan the same
	// one (D2), which is what makes a leg's partner findable from either
	// journal.
	ID ids.ClientOrderID

	// Refusal is non-nil when the leg was ATTEMPTED and nothing was sent — a
	// planning refusal, or a dispatch that never reached the wire. It is the
	// distinction a nil *LegReport cannot make: no report means no attempt.
	Refusal error

	// Result is the executor's own per-leg outcome. Meaningful only when
	// Refusal is nil.
	Result application.LegResult
}

// Report is one cross-venue pair cycle.
type Report struct {
	// Sides is indexed the way Config.Sides is, so side i here is side i there.
	Sides [2]SideReport

	// Verdict is the composed reading of the two legs.
	Verdict Verdict

	// Drove reports whether the gate admitted execution. It says nothing about
	// what was filled.
	Drove bool

	// Refusal is why the run stopped short, if it did.
	Refusal error
}

// ExitClass is what one pair cycle means for the operator driving it.
//
// It is exercise.ExitClass plus one rung, and the new rung is the reason this
// root exists: a run can end with the two venues holding UNBALANCED exposure,
// which is neither "a leg needs eyes" nor "nothing was sent".
type ExitClass uint8

const (
	// ExitClean : both boots clean, and the pair ended intact or unopened with
	// every dispatched leg journaled and applied. An IOC that filled nothing on
	// both sides is clean — it opened nothing.
	ExitClean ExitClass = iota

	// ExitOperator : a latched gate on either side's boot, or a venue IP ban.
	ExitOperator

	// ExitIncomplete : a boot could not finish, the drive refused before any leg
	// was attempted, or a leg was attempted and REFUSED with nothing sent.
	// Nothing reached a venue; a retry may lift it.
	//
	// The third case is here rather than under ExitExecution on the OMS's own
	// contract, and getting it wrong tells an operator the opposite of the
	// truth. application.LegResult states it: "a non-nil Err means NOTHING WAS
	// SENT for this leg and the order is still dispatchable." So a per-leg Err
	// and a Plan refusal are both nothing-sent, and neither is a dispatched
	// record needing eyes. Classing them as ExitExecution would print "a leg was
	// dispatched" about a leg that never left the process.
	ExitIncomplete

	// ExitExecution : a leg was SENT and its own record needs a human — a failed
	// Place, a record that did not journal, an apply whose failure holds new
	// trading, an ack the store contradicts. Exercise's class, per side.
	//
	// The distinguishing fact is that Place ran. A refusal is ExitIncomplete's.
	ExitExecution

	// ExitExposed : the run ended with the pair NOT intact and exposure live —
	// half-open with a refused or failed unwind, skewed, or indeterminate.
	//
	// It outranks ExitExecution deliberately. A lost record is a reading this
	// system can re-derive; an unbalanced cross-venue position is money moving
	// against an account with nothing hedging it, and §6.7 cannot see it to say
	// so (record §95 §4). This is the class the slice was built to report.
	ExitExposed
)

// String implements fmt.Stringer.
func (e ExitClass) String() string {
	switch e {
	case ExitClean:
		return "ExitClean"
	case ExitOperator:
		return "ExitOperator"
	case ExitIncomplete:
		return "ExitIncomplete"
	case ExitExecution:
		return "ExitExecution"
	case ExitExposed:
		return "ExitExposed"
	}
	return fmt.Sprintf("ExitClass(%d)", uint8(e))
}

// ExitClass reduces the report, boot verdicts first.
//
// The boot reports are the PRE-drive snapshots — exercise's rule and its
// reason: re-deriving them here would rank a latch raised by the drive's own
// fan-in as ExitOperator and bury the more specific fact, that THIS run
// dispatched.
func (r Report) ExitClass() ExitClass {
	for i := range r.Sides {
		if r.Sides[i].Boot.ExitClass() == reconcile.ExitOperator {
			return ExitOperator
		}
	}
	for i := range r.Sides {
		if r.Sides[i].Boot.ExitClass() == reconcile.ExitIncomplete {
			return ExitIncomplete
		}
	}
	if !r.Drove {
		return ExitIncomplete
	}
	if Exposed(r.Verdict) {
		return ExitExposed
	}
	// The runtime snapshots are taken after all drive and unwind traffic. An
	// exposed pair still outranks the ban as the exit class, but an intact pair
	// must not hide an execution-time IP ban behind clean boots.
	for i := range r.Sides {
		if r.Sides[i].RateLimit.Bans > 0 {
			return ExitOperator
		}
	}
	// Both leg classes are scanned in FULL before either answers, because a leg
	// that was sent and needs eyes outranks a leg that was refused, and the two
	// can arrive on different sides of the same run. Returning on the first leg
	// examined would make the answer depend on side order.
	refused := false
	for i := range r.Sides {
		for _, leg := range []*LegReport{r.Sides[i].Open, r.Sides[i].Unwind} {
			if leg == nil {
				continue
			}
			// Nothing was sent for this leg — see ExitIncomplete's comment. Note
			// it and keep scanning; a sent-and-broken leg elsewhere wins.
			if leg.Refusal != nil || leg.Result.Err != nil {
				refused = true
				continue
			}
			if leg.Result.Outcome.State != domain.StateSubmitted ||
				!leg.Result.Outcome.Journaled || !leg.Result.Outcome.Applied {
				return ExitExecution
			}
			if leg.Result.Outcome.Ack.VenueOrderID != "" &&
				(!leg.Result.Outcome.AckJournaled || !leg.Result.Outcome.AckApplied) {
				return ExitExecution
			}
		}
	}
	if refused {
		return ExitIncomplete
	}
	return ExitClean
}
