// Package driving holds marketdata's driving adapters: what the §3.5
// dispatcher calls to get an envelope or a fired timer into the use case.
//
// # A driving adapter is a translation, not a second use case
//
// application/ports.go records why FundingIngest.Ingest and OnSilence are
// concrete methods rather than an interface: an interface exists to allow
// substitution, and nothing substitutes the use case itself. What varies is the
// SHAPE of what arrives — an events.Envelope today, an HTTP request or a replay
// record tomorrow — and that variation is what lives here. So this package owns
// exactly two decisions: what a raw funding message must contain, and which
// parts of an arriving envelope are authoritative for what.
//
// # Nothing here loops, and nothing here is a goroutine
//
// §3.5 gives the dispatcher the job of arbitrating between ready sources and
// assigning the replay key. A driving adapter that pumped a channel would be a
// second arbitration, and the published order would stop being a function of
// the dispatch order — which is the property replay depends on. Both entry
// points below take one item and return one answer.
package driving

import (
	"fmt"

	"github.com/giiorg/arb-lab/internal/marketdata/application"
	"github.com/giiorg/arb-lab/internal/marketdata/domain"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/events"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"time"
)

// FundingUpdate is what a venue's funding stream delivers, normalized.
//
// # It carries no provenance and no instrument, and that is the whole design
//
// §4's MarketDataSource describes StreamFunding as carrying "rate, interval,
// nextSettle, cap/floor, source, venueSeq" — the last two of which the Envelope
// already carries, for every stream, as fields whose meaning §3.5 defines. Put
// them in the payload as well and there are two copies of one fact with nothing
// forbidding them to disagree, which is the hazard ids.VenueInstrument's doc
// describes from the other side: a redundant encoding is safe only while
// something rejects the disagreement. Here the redundancy is removed instead,
// so Deliver has no reconciliation to get wrong.
//
// The instrument is absent for the sharper version of the same reason: it is
// the mailbox key. A payload naming an instrument other than the source it
// arrived on is a state no feed can produce and every adapter would have to
// check.
//
// [SPEC EDIT OWED] §4's MarketDataSource comment lists what StreamFunding
// carries as "rate, interval, nextSettle, cap/floor, source, venueSeq", and the
// last two are Envelope fields §3.5 already defines for every stream. Read
// literally it asks for both copies, and nothing says which wins. The unsafe
// direction is reachable without any implementer noticing they chose: gap
// detection reads VenueSeq and freshness reads the timestamps, so a payload
// whose provenance disagrees with its envelope's makes those two questions get
// answered from different sources — a venue whose payload restamps an unchanged
// reading is then a fresh observation with no gap, which is precisely the
// silent-stall §3.4's timer exists to catch. The requirement is that the
// ENVELOPE is authoritative for provenance and a normalized payload carries
// none.
//
// # It is declared here because engine/venue does not exist yet
//
// §4 makes engine/venue the owner of normalized venue DTOs, consumed by
// contexts through thin ACL adapters — this is the ACL. Until that package
// lands, this type IS the contract: the shape a funding stream must be
// normalized into before this context will accept it. When engine/venue
// arrives, the translation into this struct is one function in this package,
// and domain/ and application/ are untouched — which is the property §3.6
// rule 1 exists to protect.
type FundingUpdate struct {
	// Rate is the venue's estimate for the NEXT settlement — per-settlement,
	// never per-hour and never annualized (§2.4).
	Rate fixedpoint.Rate

	// Interval is the live funding interval, read per-symbol on every update
	// because Bybit can flip an alt 8h↔1h mid-position (§2.4).
	Interval time.Duration

	// NextSettle is the venue's next settlement instant, in the venue's own
	// time base.
	NextSettle time.Time

	// Cap and Floor are the venue's live clamps on the rate.
	Cap, Floor fixedpoint.Rate
}

// FundingDispatch adapts §3.5's dispatch surface to marketdata's funding use
// case.
//
// Not safe for concurrent use, deliberately, and it inherits that rather than
// choosing it: §3.4 makes the marketdata reducer single-writer, FundingIngest
// holds unsynchronized maps for exactly that reason, and a mutex here would
// make concurrent use compile while leaving the published order nondeterministic
// under replay. One dispatcher goroutine calls this; the race detector says so
// if that stops being true.
type FundingDispatch struct {
	ingest *application.FundingIngest
}

// NewFundingDispatch binds the adapter to a use case, refusing a nil one.
func NewFundingDispatch(ingest *application.FundingIngest) (*FundingDispatch, error) {
	if ingest == nil {
		return nil, ErrIngestNil
	}
	return &FundingDispatch{ingest: ingest}, nil
}

// Deliver translates one dispatched envelope and runs the funding use case.
//
// # The envelope is validated in full, which means the sequence numbers too
//
// A market-data mailbox holds PRE-dispatch envelopes — driven.LanePublisher
// records why it leaves both sequence fields unstamped — and what reaches a
// context handler is POST-dispatch, so both are stamped by the time it gets
// here. Requiring them is therefore not pedantry about a struct: an observation
// accepted with no IngressSeq is one a replay cannot address, and the damage is
// not that it goes missing. It becomes the held reading that the NEXT
// observation's supersede check compares against, so an envelope nobody can
// replay silently changes what a replay accepts.
//
// # Which end of the message each field comes from
//
// Provenance — sequence and both timestamps — comes from the envelope, because
// §3.5 defines those fields for every stream and the transport is what knows
// them. The venue's reading comes from the payload. Neither is stated twice,
// so there is no disagreement to resolve. The instrument comes from the source
// for the strongest version of that: it is the mailbox key, and a payload
// cannot contradict the mailbox it was delivered to if it does not carry one.
func (d *FundingDispatch) Deliver(env events.Envelope) (application.FundingSnapshot, error) {
	if err := env.Validate(); err != nil {
		return application.FundingSnapshot{}, fmt.Errorf("marketdata funding deliver: %w", err)
	}
	// Checked against the funding stream rather than against the lane. The lane
	// is the coarser question and it would pass: mark and book ride the same
	// market-data lane, and a book update read as a funding observation is a
	// zero rate published as a venue reading.
	if env.Source.Stream != events.StreamFunding {
		return application.FundingSnapshot{}, fmt.Errorf(
			"marketdata funding deliver: stream %s: %w", env.Source.Stream, ErrWrongStream)
	}

	// A value assertion, not a pointer one, and not both. Accepting *FundingUpdate
	// as well would give one payload two spellings, one of which shares memory
	// with its producer — on a lane whose contract is that a published envelope
	// is not mutated afterwards.
	u, ok := env.Payload.(FundingUpdate)
	if !ok {
		return application.FundingSnapshot{}, fmt.Errorf(
			"marketdata funding deliver: payload %T: %w", env.Payload, ErrPayloadType)
	}

	obs := domain.FundingObservation{
		Instrument: env.Source.Instrument,
		Rate:       u.Rate,
		Interval:   u.Interval,
		NextSettle: u.NextSettle,
		Cap:        u.Cap,
		Floor:      u.Floor,
		Watermark: domain.Watermark{
			VenueSeq:   env.VenueSeq,
			SourceTime: env.SourceTime,
			RecvTime:   env.RecvTime,
		},
	}

	// The use case's own refusals travel out unwrapped in meaning — a
	// supersede is the ordinary outcome of an at-least-once transport, and a
	// caller counting them must be able to tell one from a feed fault.
	return d.ingest.Ingest(obs)
}

// Timer routes a fired silence timer to the use case.
//
// # It does not filter, and that is a decision rather than an omission
//
// The obvious addition is a Handles(TimerID) bool so a dispatcher can ask
// before routing. It is not here because the membership test already exists,
// once, inside FundingIngest — the map of armed timers — and this package
// cannot see it. A second test written from the ID's prefix would be a copy of
// a naming rule that lives in application/, and the two would agree until
// somebody changed one.
//
// So a foreign ID comes back as application.ErrUnknownTimer, which is a value
// a dispatcher can branch on. What a process-wide dispatcher should DO with a
// timer no context claims is a question for the dispatcher, and it does not
// exist yet; answering it here would be inventing a policy for a caller that
// has not been written.
func (d *FundingDispatch) Timer(id clock.TimerID) (application.FundingSnapshot, error) {
	return d.ingest.OnSilence(id)
}
