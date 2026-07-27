// Package marketdata is §3.2's market-data bounded context, and this file is
// its composition seam: the one place that knows which adapter satisfies which
// port.
//
// # Why a seam here rather than wiring in app/
//
// §11 gives each binary a composition root under internal/app/, and those roots
// own what varies BETWEEN binaries — which venues, which stores, whether there
// is an HTTP surface at all. What does not vary between binaries is which
// adapter implements this context's ports: FundingIngest is always driven by
// ConfigBudgets and LanePublisher, whether it is running live or replaying.
// Spreading that into four roots would make it four copies of one fact, and the
// backtest's parity claim (§3.5) rests on the wiring being the same wiring.
//
// So the split is: app/ decides what the store and the lane ARE, this decides
// what they are plugged into.
//
// # Scope: the funding path is WIRED; the book path is built but not yet wired
//
// §3.2 gives this context book reconstruction and mark normalization as well.
// application.BookReconstruct now exists and is tested — the domain
// reconstruction, gap detection on VenueSeq, the hold buffer and automatic
// resnapshot are all built — but Module still has one field, and the reason is
// specific rather than a leftover deferral.
//
// A driving adapter turns an events.Envelope into a use-case call, which means
// type-asserting a NORMALIZED VENUE DTO out of the payload — venue.Funding is
// what FundingDispatch asserts. There is no venue.Book yet, so a book dispatch
// would have to invent one here, in the context that consumes it, which is the
// dependency direction driving/'s doc comment exists to forbid. The book DTO
// belongs to engine/venue (§4), and that is the next slice.
//
// The deferral stays visible in the TYPE rather than in prose elsewhere: Module
// has one field, so a caller cannot reach a book path that is not assembled and
// there is no nil to check. When it lands it is a second field and a second
// Deps entry — the Resnapshotter, which is a port rather than a concrete kernel
// type because its adapter is a venue REST client living outside this context
// entirely (§20a defers those clients, and this is where that deferral surfaces).
package marketdata

import (
	"fmt"

	"github.com/giiorg/arb-lab/internal/marketdata/application"
	"github.com/giiorg/arb-lab/internal/marketdata/infrastructure/driven"
	"github.com/giiorg/arb-lab/internal/marketdata/infrastructure/driving"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/config"
	"github.com/giiorg/arb-lab/internal/platform/events"
)

// Deps is what a composition root supplies.
//
// # These are the outside world, and none of them is a port
//
// A caller passes a clock, a configuration store and an events lane — three
// concrete kernel types — and gets back a context whose inside is ports all the
// way down. That asymmetry is the point: substitution happens at the seams
// application/ declares, and a root that wanted to substitute Budgets itself
// would be reaching past the composition this file exists to perform.
//
// The one substitution a root DOES make is Clock, and it is an interface for
// §3.5's reason rather than for testing: cmd/arblab-backtest wires a TestClock
// and runs the same code, and that interchangeability IS the parity mechanism.
type Deps struct {
	// Clock is §3.5's injected time port. The silence timer is armed on it, so
	// a backtest driving a TestClock drives this context's staleness ladder.
	Clock clock.Clock

	// Config carries this context's §5.2 section. A live store rather than a
	// snapshot, because the budgets in it are hot-reloadable and the whole
	// reason application.Budgets is a port is that an edit made during an
	// incident must reach a use case constructed at boot.
	Config *config.Store[driven.FreshnessConfig]

	// Lane is §3.4's market-data lane. Its depth and coalescing policy are the
	// root's choice, not this context's: the policy is a claim about every
	// source on the lane — the perp mark coalesces adverse-preservingly and a
	// book snapshot does not — and one context cannot answer for the others.
	Lane *events.MarketDataLane
}

// Module is the context's driving surface.
//
// Whatever a root gets back is what the §3.5 dispatcher calls. Nothing else is
// exported from the wiring: the use case, the two adapters and the domain are
// reachable through their own packages for testing, but the assembled context
// hands out exactly the entry points a dispatcher needs.
type Module struct {
	// Funding is the funding-rate path: envelopes in, silence timers in.
	Funding *driving.FundingDispatch
}

// New assembles the context, refusing an incomplete one.
//
// # It adds no refusals of its own, and that is deliberate
//
// Every dependency here already has exactly one constructor that owns its
// refusal — NewConfigBudgets names a missing store, NewLanePublisher a missing
// lane, NewFundingIngest a missing clock — and each of those refusals is
// phrased in terms of the wire it is about. A nil check here would be a second
// statement of the same rule, and the copy that drifts is always the one
// written where the value is merely passing through. What this function
// contributes is ORDER, not validation.
//
// The consequence a caller can rely on: an error from New is one of the
// constructors' sentinels, so a root can branch on ErrStoreNil or
// application.ErrClockNil directly rather than on a wrapped string.
func New(deps Deps) (*Module, error) {
	budgets, err := driven.NewConfigBudgets(deps.Config)
	if err != nil {
		return nil, fmt.Errorf("marketdata: freshness budgets: %w", err)
	}
	publisher, err := driven.NewLanePublisher(deps.Lane)
	if err != nil {
		return nil, fmt.Errorf("marketdata: funding publisher: %w", err)
	}
	ingest, err := application.NewFundingIngest(deps.Clock, publisher, budgets)
	if err != nil {
		return nil, fmt.Errorf("marketdata: funding ingest: %w", err)
	}
	dispatch, err := driving.NewFundingDispatch(ingest)
	if err != nil {
		return nil, fmt.Errorf("marketdata: funding dispatch: %w", err)
	}
	return &Module{Funding: dispatch}, nil
}
