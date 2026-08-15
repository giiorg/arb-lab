package pair

import (
	"github.com/giiorg/arb-lab/internal/app/reconcile"
	"github.com/giiorg/arb-lab/internal/oms/application"
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
