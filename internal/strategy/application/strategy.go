// Package application is the strategy context's use-case layer: §3.3's
// Strategy inbound port, the cashcarry application service whose Decide
// method is the production door (§40f), and the thin adapter between them.
package application

import (
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/strategy/domain"
)

// Strategy is §3.3's context inbound port, verbatim — no core edit in this
// step. The foundation loop consumes this; §5.2's registry enables/weights
// implementations of it.
//
// # The errorless shape is NOT the production path
//
// Evaluate has no error channel, so a fatal journal failure inside it could
// only be swallowed, panicked, or side-channeled — and for an open position,
// silently returning zero Intents can suppress a required close while the
// engine keeps running without journal evidence. The production door is the
// Service's Decide (§40f); Adapter below satisfies THIS interface for the
// compile-checked conformance claim and nothing else. How the milestone-3
// foundation loop consumes the error-bearing door — a §3.3 edit widening
// this interface, or a declared synchronous fatal channel observed before
// the next event — is milestone 3's design decision, pre-registered in
// §40k-5 with the foundation loop as its construct.
type Strategy interface {
	Name() string

	// Universe is canonical (venue, product, symbol, contract); spot and
	// perp are DISTINCT [→ Z1]; static for MVP.
	Universe() []ids.VenueInstrument

	// Evaluate scores entry over a strategy-owned, immutable view — NOT
	// engine/state's type [→ X5].
	Evaluate(v domain.MarketView) []domain.Intent

	// OnPosition scores continue-vs-close for a held package.
	OnPosition(p domain.PositionView, v domain.MarketView) []domain.Intent

	// Params is the typed, validated, hot-reloadable §5.2 face.
	Params() domain.StrategyParams
}

// Adapter satisfies Strategy over the Service's Decide door.
//
// Its sole job is the conformance assertion below (§40f): the step-5 runner
// and every future production caller consume Decide, which can refuse and
// can fail loudly. The error-dropping here is exactly why this shape is not
// consumed in production — kept in one small type so the drop has one
// address.
type Adapter struct {
	S *Service
}

// The §3.6 rule 2 assertion this adapter exists for. Breaking Strategy
// conformance is a NOBUILD, which for this line means CAUGHT — the compiler
// error names the missing method.
var _ Strategy = Adapter{}

func (a Adapter) Name() string                    { return a.S.Name() }
func (a Adapter) Universe() []ids.VenueInstrument { return a.S.Universe() }
func (a Adapter) Params() domain.StrategyParams   { return a.S.Params() }

// Evaluate adapts the entry direction. The view parameter is accepted for
// interface conformance and NOT consumed: Decide assembles its own input
// from the ports (§40b — the application owns assembly), and reconciling a
// caller-supplied view with a port-assembled one would make two authorities
// over one fact.
func (a Adapter) Evaluate(domain.MarketView) []domain.Intent {
	_, intents, err := a.S.Decide(nil)
	if err != nil {
		return nil
	}
	return intents
}

// OnPosition adapts the exit direction, same rules as Evaluate.
func (a Adapter) OnPosition(p domain.PositionView, _ domain.MarketView) []domain.Intent {
	_, intents, err := a.S.Decide([]domain.PositionView{p})
	if err != nil {
		return nil
	}
	return intents
}
