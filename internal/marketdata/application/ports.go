// Package application is marketdata's use-case layer: it drives the domain and
// declares the ports the outside world plugs into.
//
// §3.2 gives this context one job on the funding path — "ingest + normalize
// funding; stamp venue-seq + source/receive time + freshness; publish". The
// domain owns what an observation MEANS (what a valid one is, when its
// settlements fall, how stale it is). This layer owns the sequencing decisions
// that need memory of what came before, and it is where §3.4's two feed-failure
// modes are actually detected.
//
// # Ports are declared here and implemented outward, which is what makes the
// # dependency point inward
//
// Every interface in this file is a DRIVEN port: something this layer needs and
// infrastructure supplies. There is no interface here for the driving side —
// Ingest and OnSilence below are the driving port, and they are concrete
// methods, because an interface exists to allow substitution and nothing
// substitutes the use case itself. §3.6 rule 1 (hexboundary) is the enforcement,
// and as of this slice it is a build failure rather than a convention.
package application

import (
	"github.com/giiorg/arb-lab/internal/marketdata/domain"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// FundingPublisher is where an accepted observation goes.
//
// # It takes a snapshot, not an envelope, and that is the boundary
//
// §3.5 assigns IngressSeq — the replay key — to the DISPATCHED envelope, in the
// order the state machine consumes it. This layer does not know that order and
// must not invent one: stamping a sequence here would put the arbitration that
// decides the replay key in two places, which is the same argument §3.5 makes
// for keeping timer delivery out of Clock. So what crosses is the observation
// and the verdict on it, and the envelope is built by whoever dispatches.
//
// # Publish returns an error, and callers must not treat it as fire-and-forget
//
// §3.4 puts market data on a lossy lane, so it is tempting to make this return
// nothing. That would be wrong at THIS seam: the lane's lossiness is a
// statement about superseded updates being coalesced, not about a publisher
// that cannot accept at all. A publisher failing is a marketdata outage, which
// §7.1 escalates; swallowing it here would make the feed die silently, which is
// precisely the failure the freshness budget exists to catch and would leave it
// as the only thing catching it.
type FundingPublisher interface {
	Publish(FundingSnapshot) error
}

// FundingSnapshot is one accepted observation together with the freshness
// verdict this layer computed for it.
//
// The verdict travels WITH the observation rather than being recomputed
// downstream, because the two would then be computed against different clock
// readings — and §3.4's whole point is that freshness is a property of an
// instant. A consumer that wants to re-derive it later can: the watermark is
// carried, and domain.Budget is a value.
type FundingSnapshot struct {
	// Observation is the accepted reading, already validated.
	Observation domain.FundingObservation

	// Transport is freshness measured against engine time — has anything
	// ARRIVED recently (domain.Watermark.Age).
	Transport domain.Freshness

	// Data is freshness measured against the venue's own server time — is the
	// venue's VIEW recent (domain.Watermark.DataAge).
	//
	// Two fields rather than one, because §3.4's "per-venue max-age budget" is
	// singular and a feed dies two ways. A venue-side stall keeps updates
	// arriving on schedule with a frozen SourceTime, so Transport reads perfectly
	// fresh for the whole outage. Reporting only the transport verdict would make
	// the invisible failure stay invisible; reporting only the minimum of the two
	// would lose which one fired, and the operator response differs.
	Data domain.Freshness

	// GapDetected reports a break in the venue's own sequence for this source
	// (§3.4's sequence-gap detection). Always false for a source that supplies
	// no sequence — see SeqAvailable, which is what stops that from reading as
	// "no gaps".
	GapDetected bool

	// SeqAvailable reports whether the venue supplies a sequence for this
	// source at all. §3.5 makes VenueSeq authoritative only within a SourceID,
	// and a REST-polled funding read carries none — the ordinary case, not an
	// error. A consumer needing gap detection must check this rather than read
	// GapDetected == false as an assurance.
	SeqAvailable bool
}

// Least is the snapshot's overall freshness: the worse of the two verdicts.
//
// A caller deciding whether to trade on this observation wants one answer, and
// the safe reduction is the minimum — the same rule domain.Least applies across
// legs, for the same reason. The two fields stay separate for diagnosis; this
// is the decision.
func (s FundingSnapshot) Least() domain.Freshness {
	return domain.Least(s.Transport, s.Data)
}

// Budgets supplies the freshness budget for a venue's funding data.
//
// A port rather than a map field, because §5.2 makes these configuration and
// §3.4 makes them per-data-class AND per-venue. Reading them through a port at
// decision time is what lets a hot-reloaded budget take effect without this
// layer holding a snapshot of config taken at construction — which is the
// staleness bug one layer up from the one it is measuring.
//
// The bool is not an error: a venue with no configured budget is a
// configuration question, and the caller decides whether that is a refusal or a
// fallback. Returning an error here would make every call site invent one.
type Budgets interface {
	FundingBudget(ids.Venue) (domain.Budget, bool)
}
