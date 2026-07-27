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

// AcceptOrdinal is the raw arrival order of the input that triggered a
// snapshot: §3.5's RawIngressSeq, stamped once at ingestion across every raw
// input in the process.
//
// # It is a named type rather than a uint64, and that is not decoration
//
// A watermark's VenueSeq is also a uint64 and means something else entirely —
// one is per-source and venue-assigned, the other is process-wide and ours. As
// bare integers a call site swapping them compiles and produces a capture log
// that is wrong in a way no test of either value alone can see. This is the
// same argument fixedpoint makes for Money and Quantity being distinct types.
//
// # It is declared here rather than imported from platform/events
//
// This layer does not take the transport's types — that is what makes the
// dependency point inward, and FundingPublisher below is the same rule applied
// to the envelope. The conversion is one cast in each adapter, which is a place
// a reader can look; an import would be a coupling nobody would look for.
//
// # Zero means TIMER-TRIGGERED, and it is the only thing it may mean
//
// §3.5 states the inheritance rule for an envelope derived from a raw input and
// has no case for one derived from the ABSENCE of an input, which is exactly
// what §3.4's silence timer produces: the one envelope on the lane whose entire
// news is that nothing arrived. There is no raw input to inherit from, so zero
// is the honest answer. What makes zero readable as that answer rather than as
// "somebody forgot" is that Ingest REFUSES it — see ErrNoTrigger.
type AcceptOrdinal uint64

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
// # INVENTED and INHERITED are different, and conflating them lost a rule
//
// The paragraph above is about IngressSeq, and it does NOT generalize to the
// other sequence number. AcceptOrdinal is stamped at ingestion, upstream of
// here, so a snapshot does not INVENT it — it INHERITS it, and §3.5 requires the
// dispatched envelope to carry it for causal traceability. Reading the
// no-sequences rule as covering both is what produced a publisher stamping zero,
// and a published snapshot that could not be traced back to the input that
// caused it. That is why Trigger is on the snapshot: it is the one piece of
// envelope provenance that must survive this boundary, and it survives as a
// value the driving side already holds rather than as an envelope this port
// would then have to accept whole.
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

	// Trigger is the AcceptOrdinal of the raw input this snapshot was derived
	// from, or zero when a silence timer derived it from no input at all.
	//
	// §3.5 requires a dispatched envelope to carry the ordinal of what caused
	// it — causal traceability — so this is what a publisher stamps. It is
	// carried on the snapshot rather than recovered by the publisher because
	// the publisher has no envelope: FundingPublisher's whole shape is that
	// what crosses is the observation and the verdict, and an adapter with
	// nothing to put here would have to either invent an ordinal or drop the
	// trace. Both were tried in the design and both are recorded as rejected.
	Trigger AcceptOrdinal
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

// FundingBudgets supplies the freshness budget for a venue's funding data.
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
//
// # It is per data class, and that is interface segregation rather than taste
//
// The obvious shape is one Budgets port with a method per class, and it was
// rejected: §3.4's classes are independent, so a funding use case declaring a
// dependency it never calls is a dependency §3.7's service split would have to
// carry across a process boundary for nothing. The narrow form also makes the
// test stubs honest — a fake that must implement BookBudget to be handed to
// FundingIngest is a fake asserting a relationship that does not exist. One
// adapter satisfies both, which is where the sharing belongs.
type FundingBudgets interface {
	FundingBudget(ids.Venue) (domain.Budget, bool)
}

// BookBudgets supplies the freshness budget for a venue's order-book data.
//
// Separate from FundingBudgets because §3.4 is explicit that the numbers are
// not interchangeable: "order books used for execution need sub-second budgets
// + a cross-leg skew limit... funding metadata tolerates longer". A single
// budget for both is one of two wrong answers — funding's tolerance applied to a
// book lets execution price against a book from a minute ago, and a book's
// applied to funding reports a healthy 8h feed as permanently blind.
type BookBudgets interface {
	BookBudget(ids.Venue) (domain.Budget, bool)
}

// Resnapshotter asks a venue for a fresh full book.
//
// # It requests and returns; the snapshot arrives as a separate input
//
// The tempting signature returns the book — one call, no state machine. It is
// wrong at this seam for §3.5's reason: a REST call "runs outside the
// state-transition dispatcher and re-enters as a fresh envelope with its own
// IngressSeq". A method that blocked until a book came back would make the
// reconstructor wait inside a dispatch, stalling every other SourceID's
// mailbox on one venue's REST latency — which is exactly the head-of-line block
// §3.5 forbids when it says an unready SourceID never stalls dispatch of ready
// inputs from other sources.
//
// So this returns only whether the request was accepted, and the resulting book
// arrives later through OnSnapshot, carrying its own AcceptOrdinal.
type Resnapshotter interface {
	RequestSnapshot(ids.VenueInstrument) error
}

// BookPublisher is where a reconstructed book goes.
//
// Same boundary as FundingPublisher and for the same reason: what crosses is the
// observation and the verdict on it, never an envelope. §3.5 assigns IngressSeq
// at dispatch, and a producer minting one would be a second authority over the
// replay key.
type BookPublisher interface {
	Publish(BookView) error
}

// BookView is one reconstructed book together with the freshness verdicts and
// the provenance of the input that produced it.
//
// It is the book path's FundingSnapshot, and the two fields that differ are the
// ones worth reading.
type BookView struct {
	// Book is the reconstructed, validated snapshot.
	Book domain.BookSnapshot

	// Transport is freshness against engine time — has anything ARRIVED.
	Transport domain.Freshness

	// Data is freshness against the venue's own server time — is the venue's
	// VIEW moving. Two verdicts rather than one, because a venue-side stall
	// keeps updates arriving on schedule with a frozen SourceTime and Transport
	// reads perfectly fresh for the whole outage.
	Data domain.Freshness

	// Applied is how the input that produced this book related to the book it
	// was applied to.
	//
	// SeqRelationUnknown means a full snapshot REPLACED the book rather than
	// following it, which is precisely what that value means on the domain type
	// — no relation is computable — so it is not a second meaning bolted onto a
	// sentinel. A consumer counting resyncs reads this; there is no separate
	// bool, because a bool beside a relation is a second spelling of one fact.
	Applied domain.SeqRelation

	// Trigger is the AcceptOrdinal of the raw input this view was derived from,
	// or zero when a silence timer derived it from no input at all. Same
	// inheritance rule, same sentinel, same reason as FundingSnapshot.Trigger.
	Trigger AcceptOrdinal
}

// Least is the view's overall freshness: the worse of the two verdicts, which is
// the safe reduction a trade decision needs.
func (v BookView) Least() domain.Freshness {
	return domain.Least(v.Transport, v.Data)
}
