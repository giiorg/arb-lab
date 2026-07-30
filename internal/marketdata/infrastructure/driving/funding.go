// Package driving holds marketdata's driving adapters: what the §3.5
// dispatcher calls to get an envelope or a fired timer into the use case.
//
// # A driving adapter is a translation, not a second use case
//
// application/ports/ records why FundingIngest.Ingest and OnSilence are
// concrete methods rather than an interface: an interface exists to allow
// substitution, and nothing substitutes the use case itself. What varies is the
// SHAPE of what arrives — an events.Envelope today, an HTTP request or a replay
// record tomorrow — and that variation is what lives here.
//
// # It is an anti-corruption layer, and as of engine/venue it is only that
//
// This package used to declare the normalized funding payload itself, as a
// FundingUpdate, because §4's owner of normalized venue DTOs did not exist yet
// and something had to be the contract. engine/venue now exists and owns
// venue.Funding, so what is left here is the translation: venue.Funding plus an
// events.Envelope in, domain.FundingObservation out. That the move cost domain/
// and application/ nothing is the property §3.6 rule 1 exists to protect, and it
// is worth noting it was checked rather than assumed — neither package's
// imports changed.
//
// The direction of the dependency is the point. This context depends on the
// venue seam; the venue seam knows nothing about this context. A venue DTO
// changing shape is a compile error in this one file.
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
	"github.com/giiorg/arb-lab/internal/marketdata/application/ports"

	"github.com/giiorg/arb-lab/internal/engine/venue"
	"github.com/giiorg/arb-lab/internal/marketdata/application"
	"github.com/giiorg/arb-lab/internal/marketdata/domain"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/events"
)

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
func (d *FundingDispatch) Deliver(env events.Envelope) (ports.FundingSnapshot, error) {
	if err := env.Validate(); err != nil {
		return ports.FundingSnapshot{}, fmt.Errorf("marketdata funding deliver: %w", err)
	}
	// Checked against the funding stream rather than against the lane. The lane
	// is the coarser question and it would pass: mark and book ride the same
	// market-data lane, and a book update read as a funding observation is a
	// zero rate published as a venue reading.
	if env.Source.Stream != events.StreamFunding {
		return ports.FundingSnapshot{}, fmt.Errorf(
			"marketdata funding deliver: stream %s: %w", env.Source.Stream, ErrWrongStream)
	}

	// A value assertion, not a pointer one, and not both. Accepting *venue.Funding
	// as well would give one payload two spellings, one of which shares memory
	// with its producer — on a lane whose contract is that a published envelope
	// is not mutated afterwards.
	u, ok := env.Payload.(venue.Funding)
	if !ok {
		return ports.FundingSnapshot{}, fmt.Errorf(
			"marketdata funding deliver: payload %T: %w", env.Payload, ErrPayloadType)
	}

	// The venue reading is judged before it is translated, and the redundancy
	// with the observation check below is deliberate — engine/venue's doc
	// records why the two types exist at all (§3.6 rule 1 forbids domain/ from
	// importing the engine, so the domain cannot BE this type).
	//
	// What the ordering buys is which boundary an operator is sent to. A cap
	// below its floor is the same defect either way, but reported from here it
	// names the venue reading, and reported from the observation it names this
	// context's translation of it. Running the venue check first means the
	// diagnosis is the outer one whenever both apply — and a field this
	// translation transposes still reaches the domain check, which is the only
	// one positioned to catch it.
	if err := u.Validate(); err != nil {
		return ports.FundingSnapshot{}, fmt.Errorf("marketdata funding deliver: %w", err)
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

	// The trigger comes from the envelope and is CONVERTED rather than passed,
	// because application declares its own AcceptOrdinal — that layer does not
	// take the transport's types, and this cast is the seam. Validate above no
	// longer judges the ordinal (zero is §3.5's absence-derived reading, and
	// this adapter's envelopes are input-derived by construction of the route),
	// so Ingest's ErrNoTrigger is the load-bearing producer-side refusal §3.5
	// assigns — no longer a backstop for hand-built calls.
	//
	// The use case's own refusals travel out unwrapped in meaning — a
	// supersede is the ordinary outcome of an at-least-once transport, and a
	// caller counting them must be able to tell one from a feed fault.
	return d.ingest.Ingest(obs, ports.AcceptOrdinal(env.AcceptOrdinal))
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
func (d *FundingDispatch) Timer(id clock.TimerID) (ports.FundingSnapshot, error) {
	return d.ingest.OnSilence(id)
}
