package ports

import (
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// MarkRiskReader is §4's OMS-owned mark-risk read port — the seam through which
// §7.1's liq-distance trigger reads the mark.
//
// # Why the OMS owns it, and why it is a READ PORT rather than an import
//
// §4's monitor row states both halves: the OMS is driven by private account
// events "and — via an OMS-owned mark-risk read port over engine/state (NOT by
// importing marketdata; the read port keeps the §3.6 hexagonal boundary) — the
// perp mark". Rule 1 forbids this context's application/ from importing
// engine/, and taking marketdata's application types would point the dependency
// outward. So this interface is declared here, in the consumer's own language,
// and engine/state implements it by translation — the same shape and the same
// direction as strategy's MarketStateReader.
//
// # The mark is why the trigger cannot be account-event-driven
//
// §7.1: liq distance "is a function of mark, which moves continuously with no
// account mutation, so the trigger MUST NOT be account-event-only (a pure rally
// would otherwise go undetected until liquidation)". The account stream is the
// OMS's natural input and it is the wrong one for this question — a rally moves
// the position toward liquidation while the account emits nothing at all. This
// port is what makes the other input reachable without breaking the boundary.
//
// # NO freshness verdict crosses, and the SIGNATURE is what enforces it
//
// engine/state's cache doc records why a stored verdict is wrong: an
// observation sitting in a cache ages without anything arriving to re-judge it,
// so a verdict stamped at accept time reads fresh for the entire outage. §7.1's
// whole stale-mark ladder (max-age → T_blind → mark_blind_action) is computed
// from that age, so a frozen verdict keeps the ladder on its top rung through
// exactly the outage it exists to escalate.
//
// So this port returns AGES and takes the instants to measure them against.
// That is stronger than a comment asking callers to re-derive: there is no
// verdict to return stale, and no reading whose age could be computed later
// against a different clock read than the one this call used.
//
// # It returns durations rather than a watermark, and a fence found the reason
//
// The obvious payload is marketdata/domain.Watermark, carried verbatim so the
// consumer can ask it anything. app/reconcile's import fence refuses it: the
// §7.5 boot root imports this ports package, and a Watermark field would put
// marketdata in a boot binary's graph — a context that fence exists to keep out,
// whose one exemption is keyed to producing a declared boot gate, which
// marketdata does not.
//
// Being forced off the watermark improved the seam. The age ARITHMETIC — two
// clocks, and the clock-inversion rule that makes a negative age
// FreshnessUnknown rather than fresh — stays single-sourced in
// marketdata/domain.Watermark, where it is already tested; the adapter computes
// it, because the adapter is a package that may import marketdata. A
// watermark-carrying port would instead have invited every consumer to redo
// that arithmetic, and the half a reader gets wrong is that the DATA age is
// measured against ServerNow(venue) and never Now.
type MarkRiskReader interface {
	// Mark returns the venue's latest mark for an instrument, with its two ages
	// measured at the instants given, and whether there is one.
	//
	// now is engine time, for the transport age — has anything ARRIVED.
	// serverNow is the VENUE's time for that instrument's venue, for the data
	// age — is the venue's own view moving. Two instants rather than one
	// because a feed dies two ways and one clock sees only one of them: a
	// venue-side stall keeps updates arriving on schedule with a frozen
	// SourceTime, so the transport age reads perfectly fresh throughout.
	//
	// The bool is not an error: nothing observed yet is the ordinary state of a
	// freshly booted process. It is also NOT the same answer as a stale mark,
	// and a caller must not collapse them — marketdata's FreshnessUnknown sits
	// below FreshnessBlind precisely because "never observed" and "observed and
	// aged out" call for different operator responses, and because a zero-valued
	// reading returned instead of false would hand §7.1 a mark of zero, which
	// is a plausible-looking denominator.
	//
	// The error is the clock inversion: an instant preceding the one it is
	// measured from. It is returned rather than folded into the bool because
	// the two say opposite things about what to do — absence is "wait, this is
	// boot", inversion is "two clocks disagree and no age computed from them
	// means anything".
	Mark(vi ids.VenueInstrument, now, serverNow time.Time) (MarkReading, bool, error)
}

// MarkReading is one venue mark as this context reads it: the number, and how
// old it was at the instants the caller asked about.
//
// # It is a type of the OMS's own, and the narrowing is deliberate
//
// marketdata/domain.MarkObservation carries the reading and this port could
// have returned it — strategy's MarketStateReader returns
// mdomain.FundingObservation, so the precedent exists. Two things rule it out.
// The fence above is the hard one. The soft one is what the consumers do:
// strategy PRICES against an observation and wants everything it says, while
// §7.1's monitor wants one number and its age, and every additional field is a
// field a liq-distance computation could reach for by accident.
type MarkReading struct {
	// Instrument is the perpetual this reads. Carried rather than left to the
	// caller's own key, so a reading that has been passed through a layer still
	// names what it describes.
	Instrument ids.VenueInstrument

	// Price is the venue's mark, in the instrument's quote asset. It is the
	// VENUE's number: §7.1 computes liq distance from the venue's mark against
	// the venue's own published liquidation price, and there the venue's
	// computation is the definition of the quantity its liquidation engine acts
	// on.
	Price fixedpoint.Price

	// Age is how long since this process took delivery, at the `now` the caller
	// supplied — the transport-liveness question.
	Age time.Duration

	// DataAge is how old the venue's own view was, at the `serverNow` the
	// caller supplied. Separate from Age because a venue-side stall keeps
	// updates arriving on schedule with a frozen SourceTime, so Age alone
	// reports a healthy feed for the whole outage.
	DataAge time.Duration
}
