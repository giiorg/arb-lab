package driven_test

import (
	"errors"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/marketdata/application"
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

func snapshot(t *testing.T, verdict domain.Freshness) application.FundingSnapshot {
	t.Helper()
	return application.FundingSnapshot{
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

	got, ok := env.Payload.(application.FundingSnapshot)
	if !ok {
		t.Fatalf("Payload is %T, want application.FundingSnapshot", env.Payload)
	}
	if got != snap {
		t.Errorf("Payload = %+v, want the snapshot published %+v", got, snap)
	}
}

func TestPublishStampsNeitherSequenceNumber(t *testing.T) {
	// §3.5 gives AcceptOrdinal to ingestion and IngressSeq to the dispatcher.
	// This adapter is neither, and a locally-minted value for either would be a
	// second authority over an ordering that has to have exactly one — the same
	// argument the port's doc makes for the snapshot not carrying a sequence.
	lane := newLane(t)
	if err := newPublisher(t, lane).Publish(snapshot(t, domain.FreshnessFresh)); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	env, ok := lane.Consume(fundingSource())
	if !ok {
		t.Fatal("nothing on the funding mailbox")
	}
	if env.AcceptOrdinal != 0 {
		t.Errorf("AcceptOrdinal = %d, want 0 — ingestion stamps it, not this layer", env.AcceptOrdinal)
	}
	if env.IngressSeq != 0 {
		t.Errorf("IngressSeq = %d, want 0 — the dispatcher stamps the replay key", env.IngressSeq)
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
		breakIt func(*application.FundingSnapshot)
		want    error
	}{
		"no instrument — the envelope has no source to address": {
			breakIt: func(s *application.FundingSnapshot) { s.Observation.Instrument = ids.VenueInstrument{} },
			want:    ids.ErrVenueUnset,
		},
		"no receive time — the envelope requires ours": {
			breakIt: func(s *application.FundingSnapshot) { s.Observation.Watermark.RecvTime = time.Time{} },
			want:    events.ErrRecvTimeUnset,
		},
		"no source time — only the domain requires the venue's": {
			breakIt: func(s *application.FundingSnapshot) { s.Observation.Watermark.SourceTime = time.Time{} },
			want:    domain.ErrWatermarkUnset,
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
