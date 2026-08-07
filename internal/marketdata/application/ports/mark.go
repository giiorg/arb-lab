package ports

import (
	"github.com/giiorg/arb-lab/internal/marketdata/domain"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// MarkPublisher is where an accepted mark observation goes.
//
// Same boundary as FundingPublisher and BookPublisher, and for the same reason:
// what crosses is the observation and the verdict on it, never an envelope.
// §3.5 assigns IngressSeq at dispatch, and a producer minting one would be a
// second authority over the replay key.
//
// Publish returns an error for FundingPublisher's reason, and it is sharper
// here: §3.4's lane is lossy about SUPERSEDED updates, not about a publisher
// that cannot accept at all, and a mark publisher failing silently would leave
// §7.1's liq-distance trigger reading a mark nothing is refreshing — with the
// freshness budget as the only thing that would ever notice.
type MarkPublisher interface {
	Publish(MarkSnapshot) error
}

// MarkSnapshot is one accepted mark reading together with the freshness
// verdicts this layer computed for it.
//
// # It carries NO GapDetected and NO SeqAvailable, and the absence is a ruling
//
// FundingSnapshot carries both, and copying them here was the obvious move. It
// is refused because for this class they could only ever be the constants
// (false, false) — and a field pinned to a constant is an unchecked claim
// wearing a type annotation, with the additional cost that a reader seeing
// GapDetected == false reads it as an assurance rather than as an absence.
//
// Neither MVP venue supplies a sequence this class can use, and the Bybit half
// is the one worth writing down because it looks like it does. Binance's
// markPrice frame carries no sequence at all. Bybit's ticker carries `cs`, and
// `cs` genuinely advances — but it counts TICKER MESSAGES, and only some ticker
// messages state a markPrice. So between two consecutive mark readings `cs`
// skips by however many bid/ask-churn deltas fell between them, which is a gap
// on almost every pair. The sequence is real and its granularity is a different
// class's; taking it would report a permanent gap in a healthy feed.
//
// Supersede detection is unaffected: domain.Watermark still carries VenueSeq,
// and the ingest's arrival-order fallback is what handles a source with none.
// What is dropped is only the gap VERDICT, which nothing could compute.
type MarkSnapshot struct {
	// Observation is the accepted reading, already validated.
	Observation domain.MarkObservation

	// Transport is freshness measured against engine time — has anything
	// ARRIVED recently (domain.Watermark.Age).
	Transport domain.Freshness

	// Data is freshness measured against the venue's own server time — is the
	// venue's VIEW recent (domain.Watermark.DataAge).
	//
	// Two fields rather than one, FundingSnapshot's rule and its reason: a
	// venue-side stall keeps updates arriving on schedule with a frozen
	// SourceTime, so Transport reads perfectly fresh for the whole outage. For
	// the mark that outage IS the input §7.1 escalates on, and the operator
	// response to a dead socket differs from the response to a venue whose mark
	// has stopped moving.
	Data domain.Freshness

	// Trigger is the AcceptOrdinal of the raw input this snapshot was derived
	// from, or zero when a silence timer derived it from no input at all. Same
	// inheritance rule, same sentinel, same reason as FundingSnapshot.Trigger.
	Trigger AcceptOrdinal
}

// Least is the snapshot's overall freshness: the worse of the two verdicts.
//
// This is the value §7.1's ladder reads. FreshnessBlind here is what fires
// §5.2's mark_blind_action, and FreshnessUnknown — never observed, or a broken
// clock — is deliberately below it rather than beside it.
func (s MarkSnapshot) Least() domain.Freshness {
	return domain.Least(s.Transport, s.Data)
}

// MarkBudgets supplies the freshness budget for a venue's mark data.
//
// A third port rather than a method on FundingBudgets, on the segregation rule
// this package's doc states: §3.4's classes are independent, and a use case
// declaring a dependency it never calls is one §3.7's service split would carry
// across a process boundary for nothing. One adapter satisfies all three.
//
// The numbers are the ones that must not be shared. §5.2 puts the mark's
// max-age and its T_blind inside the stale-mark escalation policy, because they
// are what §7.1's ladder is built from; funding's budget is metadata tolerance.
// A single budget for both is wrong in whichever direction it is set.
type MarkBudgets interface {
	MarkBudget(ids.Venue) (domain.Budget, bool)
}
