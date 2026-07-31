package application

import (
	"fmt"

	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/strategy/application/ports"
	"github.com/giiorg/arb-lab/internal/strategy/domain"
)

// Sentinels.
var (
	// ErrDepNil reports a Deps member nobody wired. Named per field by the
	// wrapping message.
	ErrDepNil = fmt.Errorf("strategy: dependency not wired")

	// ErrUniverseShape reports a universe that is not one spot leg and one
	// perp leg on one symbol — the only shape cashcarry trades (§12.2's
	// one-venue/one-symbol milestone).
	ErrUniverseShape = fmt.Errorf("strategy: universe is not a spot+perp pair")

	// ErrCoreUnbuilt reports the slice-A skeleton: the Decide door exists
	// and refuses, loudly, until slice C lands the assembly and the
	// cashcarry core. A door that silently returned zero Intents would look
	// exactly like a scored Decline — this error is the difference between
	// "not built" and "decided against".
	ErrCoreUnbuilt = fmt.Errorf("strategy: decision core not built (step 3 slice C)")

	// ErrTooManyPositions reports more than one held package. The MVP
	// universe is one pair, so a second position is evidence of a caller
	// bug, not a portfolio.
	ErrTooManyPositions = fmt.Errorf("strategy: more than one held position")
)

// Deps are the Service's wired dependencies: the injected clock and §40b's
// seven ports.
type Deps struct {
	// Clock is the ONLY time source (§3.6 rule 5). Intent.ValidUntil and
	// every evaluation instant come from here, which is what makes the sim
	// pump and a future live pump the same engine (§3.5).
	Clock clock.Clock

	Market    ports.MarketStateReader
	History   ports.SettlementHistory
	Forecast  ports.FundingForecast
	Basis     ports.BasisForecast
	Cost      ports.ExecutionCostModel
	Reference ports.ReferenceData
	Sink      ports.DecisionSink
}

// validate names the first missing dependency. A nil port found at Decide
// time would be a panic in the code path that prices a hold.
func (d Deps) validate() error {
	for _, c := range []struct {
		name string
		nil_ bool
	}{
		{"clock", d.Clock == nil},
		{"market state reader", d.Market == nil},
		{"settlement history", d.History == nil},
		{"funding forecast", d.Forecast == nil},
		{"basis forecast", d.Basis == nil},
		{"execution cost model", d.Cost == nil},
		{"reference data", d.Reference == nil},
		{"decision sink", d.Sink == nil},
	} {
		if c.nil_ {
			return fmt.Errorf("%s: %w", c.name, ErrDepNil)
		}
	}
	return nil
}

// Service is the cashcarry application service. Its Decide method is the
// production inbound door (§40f); the §3.3 Strategy face is Adapter's.
//
// It is deliberately CONCRETE: the use case is the driving port, and nothing
// substitutes a use case (the rule marketdata's ports doc records). What
// varies — venues, models, storage — varies behind the driven ports in Deps.
type Service struct {
	deps   Deps
	spot   ids.VenueInstrument
	perp   ids.VenueInstrument
	params domain.StrategyParams
}

// NewService wires the service for one spot+perp pair.
//
// params arrives validated and construction-fixed for the milestone-2
// single-revision world; the live PolicyProjection assembly (revision
// identity per decision, hot-reload) lands with the Decide implementation in
// slice C, where the projection is read per call rather than held.
func NewService(deps Deps, spot, perp ids.VenueInstrument, params domain.StrategyParams) (*Service, error) {
	if err := deps.validate(); err != nil {
		return nil, err
	}
	if err := spot.Validate(); err != nil {
		return nil, fmt.Errorf("spot: %w", err)
	}
	if err := perp.Validate(); err != nil {
		return nil, fmt.Errorf("perp: %w", err)
	}
	if spot.Product != ids.ProductSpot || !perp.Product.IsPerp() {
		return nil, fmt.Errorf("%s + %s: %w", spot, perp, ErrUniverseShape)
	}
	if spot.Venue != perp.Venue || spot.Symbol != perp.Symbol {
		// Cash-and-carry is same venue, same asset (§2.2); the cross-venue
		// pair is strategy #2's shape, not a misconfiguration this service
		// should quietly trade.
		return nil, fmt.Errorf("%s + %s: %w", spot, perp, ErrUniverseShape)
	}
	if err := params.Validate(); err != nil {
		return nil, fmt.Errorf("params: %w", err)
	}
	return &Service{deps: deps, spot: spot, perp: perp, params: params}, nil
}

// Name is the §5.2 registry key.
func (s *Service) Name() string { return "cashcarry" }

// Universe is the configured pair, spot first. A fresh slice per call: the
// universe is static (§3.3) and a caller must not be able to edit it.
func (s *Service) Universe() []ids.VenueInstrument {
	return []ids.VenueInstrument{s.spot, s.perp}
}

// Params is the §3.3 typed-params face.
func (s *Service) Params() domain.StrategyParams { return s.params }

// Decide is the production door (§40f): assemble the DecisionInput from the
// ports, run the core, sink the Decision write-ahead, then — and only then —
// derive Intents.
//
// held is the caller's position facts: empty for an entry evaluation,
// exactly one for continue-vs-close.
//
// # Slice A: the door exists and refuses
//
// The contract fixed now, which slice C implements and tests: Decide writes
// the Decision to the DecisionSink BEFORE returning any Intent derived from
// it; a failed sink write returns zero Intents AND a non-nil error, and the
// runner treats that error as run-aborting. Pre-core refusals (a port
// errored, an unpopulated input, a stale view) are the service's to
// construct: the core never sees an invalid input — the service builds the
// refusal Decision naming the refusing rule, sinks it, and returns no
// Intents. No Intent without its journaled Decision.
func (s *Service) Decide(held []domain.PositionView) (domain.Decision, []domain.Intent, error) {
	if len(held) > 1 {
		return domain.Decision{}, nil, fmt.Errorf("%d positions: %w", len(held), ErrTooManyPositions)
	}
	return domain.Decision{}, nil, ErrCoreUnbuilt
}
