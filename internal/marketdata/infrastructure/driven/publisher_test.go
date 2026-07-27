package driven_test

import (
	"errors"
	"github.com/giiorg/arb-lab/internal/marketdata/application/ports"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/marketdata/domain"
	"github.com/giiorg/arb-lab/internal/marketdata/infrastructure/driven"
	"github.com/giiorg/arb-lab/internal/platform/events"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// The three provenance values are pairwise DISTINCT, and none is derivable from
// another. Two equal timestamps would make "copies SourceTime" and "copies
// RecvTime" the same assignment, and a VenueSeq of 1 collides with the
// AcceptOrdinal a wrong implementation would stamp.
var (
	recvAt   = at.Add(3 * time.Second)
	sourceAt = at.Add(1 * time.Second)
)

const venueSeq uint64 = 4242

// acceptOrdinal is the trigger a non-timer snapshot carries, and it is
// deliberately NOT venueSeq.
//
// Both are uint64 and both ride the same envelope, so a fixture using one value
// for both would make "inherited the trigger" and "stamped from the watermark's
// sequence" the same assertion — and the second is a real implementation
// somebody would write, because the watermark is the field already in hand.
const acceptOrdinal = ports.AcceptOrdinal(9001)

func perp() ids.VenueInstrument {
	return ids.VenueInstrument{
		Venue:    binance,
		Product:  ids.ProductLinearPerp,
		Symbol:   "BTCUSDT",
		Contract: ids.ContractLinear,
	}
}

func rate(t *testing.T, s string) fixedpoint.Rate {
	t.Helper()
	r, err := fixedpoint.ParseRate(s)
	if err != nil {
		t.Fatalf("ParseRate(%q): %v", s, err)
	}
	return r
}

func snapshot(t *testing.T, verdict domain.Freshness) ports.FundingSnapshot {
	t.Helper()
	return ports.FundingSnapshot{
		Observation: domain.FundingObservation{
			Instrument: perp(),
			Rate:       rate(t, "0.0001"),
			Interval:   8 * time.Hour,
			NextSettle: at.Add(4 * time.Hour),
			Cap:        rate(t, "0.0075"),
			Floor:      rate(t, "-0.0075"),
			Watermark: domain.Watermark{
				VenueSeq:   venueSeq,
				SourceTime: sourceAt,
				RecvTime:   recvAt,
			},
		},
		Transport:    verdict,
		Data:         verdict,
		SeqAvailable: true,
		Trigger:      acceptOrdinal,
	}
}

// fundingSource is the mailbox a published snapshot must land in. Built here
// independently of the adapter, so a test reads from the source it EXPECTS
// rather than from whatever the adapter chose.
func fundingSource() events.SourceID {
	return events.SourceID{Instrument: perp(), Stream: events.StreamFunding}
}

// newLane uses NoCoalescing so two publishes are two observable envelopes.
// Under LatestWins the second would merge into the first, and a test asserting
// on "what was published" would be asserting on a merge rule that belongs to
// the composition root, not to this adapter.
func newLane(t *testing.T) *events.MarketDataLane {
	t.Helper()
	lane, err := events.NewMarketDataLane(4, events.NoCoalescing)
	if err != nil {
		t.Fatalf("NewMarketDataLane: %v", err)
	}
	return lane
}

func newPublisher(t *testing.T, lane *events.MarketDataLane) *driven.LanePublisher {
	t.Helper()
	p, err := driven.NewLanePublisher(lane)
	if err != nil {
		t.Fatalf("NewLanePublisher: %v", err)
	}
	return p
}

// ---------------------------------------------------------------------------
// Construction
// ---------------------------------------------------------------------------

func TestNewLanePublisherRefusesANilLane(t *testing.T) {
	if _, err := driven.NewLanePublisher(nil); !errors.Is(err, driven.ErrLaneNil) {
		t.Fatalf("NewLanePublisher(nil) = %v, want ErrLaneNil", err)
	}
}

// ---------------------------------------------------------------------------
// What crosses onto the lane
// ---------------------------------------------------------------------------

func TestPublishCarriesTheWatermarkOntoTheEnvelope(t *testing.T) {
	lane := newLane(t)
	snap := snapshot(t, domain.FreshnessFresh)

	if err := newPublisher(t, lane).Publish(snap); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	env, ok := lane.Consume(fundingSource())
	if !ok {
		t.Fatal("nothing on the funding mailbox")
	}
	if got := env.Source; got != fundingSource() {
		t.Errorf("Source = %+v, want %+v", got, fundingSource())
	}
	if !env.RecvTime.Equal(recvAt) {
		t.Errorf("RecvTime = %v, want the watermark's %v", env.RecvTime, recvAt)
	}
	if !env.SourceTime.Equal(sourceAt) {
		t.Errorf("SourceTime = %v, want the watermark's %v", env.SourceTime, sourceAt)
	}
	if env.VenueSeq != venueSeq {
		t.Errorf("VenueSeq = %d, want the watermark's %d", env.VenueSeq, venueSeq)
	}

	got, ok := env.Payload.(ports.FundingSnapshot)
	if !ok {
		t.Fatalf("Payload is %T, want ports.FundingSnapshot", env.Payload)
	}
	if got != snap {
		t.Errorf("Payload = %+v, want the snapshot published %+v", got, snap)
	}
}

// The two sequence numbers get opposite treatment here, and the difference is
// INVENTED versus INHERITED.
//
// §3.5 gives IngressSeq to the dispatcher, in the order the state machine
// consumes an envelope. This adapter does not know that order, so a value
// minted here would be a second authority over the replay key — it stays zero.
// AcceptOrdinal is stamped at ingestion, UPSTREAM, and §3.5 requires the
// dispatched envelope to carry the ordinal of the input that triggered it. The
// snapshot carried it here for exactly that, so writing it invents nothing.
//
// One test rather than two, because a suite that asserted them separately could
// not show they were treated differently — which is the whole claim.
func TestPublishInheritsTheOrdinalAndStampsNoReplayKey(t *testing.T) {
	lane := newLane(t)
	if err := newPublisher(t, lane).Publish(snapshot(t, domain.FreshnessFresh)); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	env, ok := lane.Consume(fundingSource())
	if !ok {
		t.Fatal("nothing on the funding mailbox")
	}
	if env.AcceptOrdinal != uint64(acceptOrdinal) {
		t.Errorf("AcceptOrdinal = %d, want the snapshot's trigger %d — a published snapshot no replay can tie to its cause",
			env.AcceptOrdinal, acceptOrdinal)
	}
	if env.IngressSeq != 0 {
		t.Errorf("IngressSeq = %d, want 0 — the dispatcher stamps the replay key", env.IngressSeq)
	}
}

// A snapshot from §3.4's silence timer carries a ZERO trigger, and it must
// still publish.
//
// This is what forces the probe to fabricate BOTH sequence numbers rather than
// only the replay key. Probing with the real ordinal reads naturally — it is the
// value the envelope actually carries — and it would make Publish refuse
// precisely the envelope whose entire news is that a feed has gone quiet. The
// one failure that must never be silent would be the one this adapter drops.
func TestPublishAcceptsATimerTriggeredSnapshot(t *testing.T) {
	lane := newLane(t)
	snap := snapshot(t, domain.FreshnessStale)
	snap.Trigger = 0

	if err := newPublisher(t, lane).Publish(snap); err != nil {
		t.Fatalf("Publish: %v — a silence snapshot must reach the lane", err)
	}

	env, ok := lane.Consume(fundingSource())
	if !ok {
		t.Fatal("nothing on the funding mailbox")
	}
	if env.AcceptOrdinal != 0 {
		t.Errorf("AcceptOrdinal = %d, want 0 — there was no raw input to inherit from", env.AcceptOrdinal)
	}
}

func TestPublishUsesOnePriorityForEveryVerdict(t *testing.T) {
	// A priority that varied with the freshness verdict would reorder two
	// snapshots of ONE SourceID against each other, where VenueSeq is
	// authoritative (§3.5) — a degrading feed's later reading overtaking its
	// earlier one inside the mailbox it shares.
	lane := newLane(t)
	pub := newPublisher(t, lane)

	// Deliberately the two ends of the ladder: if priority were derived from
	// the verdict at all, Fresh and Blind are where it would differ most.
	for _, verdict := range []domain.Freshness{domain.FreshnessFresh, domain.FreshnessBlind} {
		if err := pub.Publish(snapshot(t, verdict)); err != nil {
			t.Fatalf("Publish(%v): %v", verdict, err)
		}
	}

	first, ok := lane.Consume(fundingSource())
	if !ok {
		t.Fatal("nothing on the funding mailbox")
	}
	second, ok := lane.Consume(fundingSource())
	if !ok {
		t.Fatal("only one envelope on the funding mailbox")
	}
	if first.Prio != second.Prio {
		t.Errorf("Prio %d then %d; priority must not depend on the verdict", first.Prio, second.Prio)
	}
	if first.Prio == events.PrioSafety {
		t.Error("funding publishes at PrioSafety; §3.4 puts funding metadata below the safety path")
	}
}

// ---------------------------------------------------------------------------
// What this layer refuses, which is exactly what it is responsible for
// ---------------------------------------------------------------------------

func TestPublishRefusesAndNamesTheCheckThatFired(t *testing.T) {
	// Both checks refuse, so asserting only "err != nil" would make them the
	// same observation and leave either one deletable. Each case therefore
	// names the SENTINEL, which is what pins which layer owns the mistake.
	//
	// The third case is the one that makes two checks necessary rather than
	// tidy: SourceTime MAY be absent on a lane, because not every venue message
	// carries one, so events.Envelope.Validate accepts it — while
	// domain.Watermark requires it, because an observation with no venue stamp
	// cannot answer DataAge and the venue-side stall is exactly what that
	// measurement exists to catch. Under the envelope check alone it would
	// reach the mailbox looking well-formed.
	cases := map[string]struct {
		breakIt func(*ports.FundingSnapshot)
		want    error
	}{
		"no instrument — the envelope has no source to address": {
			breakIt: func(s *ports.FundingSnapshot) { s.Observation.Instrument = ids.VenueInstrument{} },
			want:    ids.ErrVenueUnset,
		},
		"no receive time — the envelope requires ours": {
			breakIt: func(s *ports.FundingSnapshot) { s.Observation.Watermark.RecvTime = time.Time{} },
			want:    events.ErrRecvTimeUnset,
		},
		"no source time — only the domain requires the venue's": {
			breakIt: func(s *ports.FundingSnapshot) { s.Observation.Watermark.SourceTime = time.Time{} },
			want:    domain.ErrWatermarkUnset,
		},
		// A well-formed instrument that simply has no funding. Before §4's
		// perp-only rule reached events.SourceID this case came out as
		// domain.ErrFundingOnSpot — refused, but by the observation check,
		// three layers in and only because marketdata happens to build a
		// domain object. Now the envelope itself cannot be addressed, which is
		// what covers the producers that build no observation at all.
		//
		// The sentinel is the assertion, not merely that it fails: the two
		// diagnoses name different owners, and this pins which one the
		// transport now answers with.
		"spot instrument — a funding source no venue can produce": {
			breakIt: func(s *ports.FundingSnapshot) {
				s.Observation.Instrument = ids.VenueInstrument{
					Venue: binance, Product: ids.ProductSpot, Symbol: "BTCUSDT",
				}
			},
			want: events.ErrStreamProductMismatch,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			lane := newLane(t)
			snap := snapshot(t, domain.FreshnessFresh)
			tc.breakIt(&snap)

			err := newPublisher(t, lane).Publish(snap)
			if !errors.Is(err, tc.want) {
				t.Fatalf("Publish = %v, want %v", err, tc.want)
			}
			// Refused means NOT delivered. A publisher that reported an error
			// after putting the envelope on the lane would leave the consumer
			// reading a record the producer believes it rejected.
			if _, ok := lane.Consume(fundingSource()); ok {
				t.Error("the refused snapshot reached the mailbox anyway")
			}
		})
	}
}
