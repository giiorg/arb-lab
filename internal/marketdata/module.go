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
// # Scope: the funding and book paths are WIRED; mark is not
//
// §3.2 gives this context three jobs and two of them are assembled here. The
// book path landed exactly as this comment previously committed to: engine/venue
// gained the normalized DTOs (venue.BookSnapshot, venue.BookDelta), so
// BookDispatch type-asserts a venue type instead of inventing one in the context
// that consumes it — the dependency direction driving/'s doc exists to protect.
// Module has a second field and Deps has the predicted Resnapshotter entry.
//
// Mark normalization remains unwired, and the reason is the same shape as the
// book path's was: §3.4's mark stream has its own freshness class and §5.2's
// mark_blind_action keys on it, but there is no venue.Mark and no use case, so
// there is nothing here to compose. The deferral stays visible in the TYPE
// rather than in prose elsewhere — a caller cannot reach a mark path that is not
// assembled, and there is no nil to check.
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
// # Three kernel types, and ONE port — the exception is the interesting field
//
// A caller passes a clock, a configuration store and an events lane — concrete
// kernel types — and gets back a context whose inside is ports all the way down.
// That asymmetry is the point: substitution happens at the seams application/
// declares, and a root that wanted to substitute Budgets itself would be
// reaching past the composition this file exists to perform.
//
// Resnapshotter breaks that pattern, and it is worth saying why rather than
// letting it read as an inconsistency. Every other driven port has exactly one
// adapter and that adapter lives in this context's infrastructure/, so this file
// can construct it. A resnapshot is a REST call to a venue, and the venue
// clients live outside this context entirely — §20a defers them — so there is
// nothing here to construct. A root supplies it or the context does not build.
//
// The one substitution a root ALSO makes is Clock, and it is an interface for
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

	// Resnapshotter is how the book path asks a venue for a fresh full book.
	//
	// A port, and supplied from outside for the reason stated above: its adapter
	// is a venue REST client, and §20a defers those. What a root wires here must
	// be IDEMPOTENT under repeat calls — application.BookReconstruct requests on
	// every held delta rather than once, deliberately, because a single dropped
	// request leaves the book held forever with nothing retrying. Only the
	// adapter knows the venue's rate limit (§7.6), so the coalescing belongs
	// there.
	Resnapshotter application.Resnapshotter

	// BookHoldDepth bounds the per-instrument buffer of deltas awaiting a
	// resnapshot.
	//
	// # A Deps field rather than a §5.2 config entry, which is a claim about
	// # WHEN it may change
	//
	// The freshness budgets beside it are hot — an operator widens one
	// mid-incident and it takes effect without a restart. This must not be, and
	// platform/config's default direction is the argument: a field is
	// restart-required until somebody writes down why it is safe to change under
	// load, and nobody can write that here. Shrinking the depth mid-resync
	// discards held deltas that the in-flight snapshot was going to chain to,
	// which converts a recovering book into the non-terminating resync loop the
	// buffer exists to prevent — at precisely the moment an operator is reaching
	// for a knob.
	//
	// Zero is refused rather than defaulted: application.ErrHoldDepthInvalid
	// records that a depth of zero holds nothing, so the REST book always lands
	// behind the live stream and the very next delta gaps, forever.
	BookHoldDepth int
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

	// Book is the order-book path: snapshot and delta envelopes in on one
	// surface, silence timers in.
	//
	// Two fields rather than one dispatcher with a stream switch, and the two
	// Timer methods are why. Each path arms its OWN timer-ID namespace
	// (application.bookSilenceTimerID records that a shared name would make each
	// class's arm cancel the other's), so the routing a caller does here is real
	// routing and not a formality. A merged surface would have to re-derive which
	// use case owns a fired ID from the ID's spelling — a copy of a naming rule
	// that lives in application/, which is the same copy FundingDispatch.Timer
	// declines to make one layer down.
	Book *driving.BookDispatch
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

	// The book path gets its OWN publisher over the SAME lane. Sharing the
	// pointer is correct — one market-data lane, per-SourceID mailboxes inside it
	// — and the two publishers exist because the two ports both declare Publish;
	// driven.BookLanePublisher's doc records that Go forces the split.
	bookPublisher, err := driven.NewBookLanePublisher(deps.Lane)
	if err != nil {
		return nil, fmt.Errorf("marketdata: book publisher: %w", err)
	}
	// budgets is reused rather than reconstructed: one ConfigBudgets satisfies
	// both narrow ports, which is where the sharing belongs (application declares
	// FundingBudgets and BookBudgets separately so neither use case depends on
	// the other's method). A second adapter over the same store would be a second
	// read path to keep in step for nothing.
	reconstruct, err := application.NewBookReconstruct(
		deps.Clock, bookPublisher, budgets, deps.Resnapshotter, deps.BookHoldDepth)
	if err != nil {
		return nil, fmt.Errorf("marketdata: book reconstruction: %w", err)
	}
	bookDispatch, err := driving.NewBookDispatch(reconstruct)
	if err != nil {
		return nil, fmt.Errorf("marketdata: book dispatch: %w", err)
	}

	return &Module{Funding: dispatch, Book: bookDispatch}, nil
}
