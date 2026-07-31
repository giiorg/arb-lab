// Package ports declares the strategy context's driven dependencies, as §3.6
// rule 2 requires: every driven dependency is an interface here; adapters
// live in infrastructure/driven/ (or, for the read seam, in the engine
// subsystem that holds the data) with a compile-time var _ ports.X
// assertion.
//
// The roster is §40b's, seven in all: the two read ports of the split read
// seam (MarketStateReader, SettlementHistory), the three model ports
// (FundingForecast, BasisForecast, ExecutionCostModel), the consolidated
// ReferenceData seam, and the DecisionSink. Every port is DRIVEN; the driving
// side is the application service itself, concrete, because nothing
// substitutes a use case (marketdata's ports doc records the rule).
//
// Payload types are strategy/domain's: this layer does not take a
// transport's or another context's application types, which is what makes
// the dependency point inward.
package ports

import (
	"time"

	mdomain "github.com/giiorg/arb-lab/internal/marketdata/domain"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/strategy/domain"
)

// MarketStateReader is the latest-value half of §40c's split read seam:
// funding observation and top of book, per instrument, each with its
// watermark.
//
// engine/state implements it by translation over its cache — the direction
// every gate allows, and the consumer-owned-port trigger its doc deferred on.
// The false return is "this instrument has no entry yet", which the service
// turns into a pre-core refusal Decision; it is not an error, because an
// empty cache at boot is a state, not a fault.
//
// No freshness verdict crosses this seam. The entries carry watermarks and
// the ages are derived at the READ against an instant the caller supplies —
// engine/state's cache doc records why a stored verdict would read fresh for
// an entire outage.
type MarketStateReader interface {
	Funding(ids.VenueInstrument) (mdomain.FundingObservation, bool)
	TopOfBook(ids.VenueInstrument) (domain.PriceView, bool)
}

// SettlementHistory is the other half: the realized-settlement window the
// EWMA needs, ascending, oldest first.
//
// # The adapter's ownership is declared here, before anything implements it
//
// The implementor is ONE deterministic accumulator over the DISPATCHED
// funding stream — the same component, fed by the same envelopes, in live
// and replay. §40c records why the latest-value cache is the wrong home (a
// cache holds one observation per source; widening it into a ring buffer for
// one consumer grows a second contract) and why a runner-owned accumulator
// was rejected (replay-specific state makes the forecast's inputs differ
// between the two paths §3.5 requires to be the same engine). It is a
// strategy-context adapter wired by the composition root; the step 5 runner
// is its first instantiation, and this declaration is what the deferral rule
// names as its construct.
//
// How MUCH history is retained is the composition root's wiring (a
// forecast_model concern, §5.2); the port answers with what it holds.
type SettlementHistory interface {
	Window(ids.VenueInstrument) ([]domain.RealizedSettlement, error)
}

// FundingForecast is §2.4's estimator contract on the explicit-materials
// signature (§40e): per-settlement forecasts over the horizon's enumerated
// settlements, from materials the CALLER supplies.
//
// §2.4 writes the contract as Estimate(inst, horizon) and in the same
// paragraph puts the estimator's raw materials in MarketView — an
// implementation honoring the literal two-argument form must hold market
// state internally or fetch it mid-call, which contradicts the pure-core
// rule and makes the estimator untestable as a function. The
// explicit-materials reading preserves the output shape and the horizon rule
// (still keyed by instrument, through the materials' observation); the slice
// C close's reading pass confirms it or converts it to a marked §2.4 edit.
//
// The horizon is the plan's enumerated settlements — never instants the
// estimator invents. The port earns its seam from §2.4's own swappability
// requirement (forecast_model, hot-reloadable, out-of-sample validated), not
// from I/O: the default estimator is pure domain math wired behind it.
type FundingForecast interface {
	Estimate(m domain.FundingMaterials, horizon []time.Time) ([]domain.SettlementForecast, error)
}

// BasisForecast supplies §2.4's basis term. The default adapter is the
// spec's conservative rule — zero favorable convergence, DECLARED, and the
// measured adverse move charged as a class-2 artifact-identified input — so
// "absent a configured estimator" is a wiring state, never a nil check.
type BasisForecast interface {
	EstimateBasis(view domain.MarketView, plan domain.HoldPlanRevision, qty fixedpoint.Quantity) (domain.BasisTerm, error)
}

// ExecutionCostModel prices a candidate execution (§40e): the expected
// per-leg cost for a candidate quantity under a stated tactic and latency
// assumption, with the calibration identity its numbers came from.
//
// The adapter sources its own book state — its wiring, not the strategy
// application's; book plumbing stays out of this context. The estimate is
// never the simulator's realized fill: one is the decision's input, the
// other is the run's output, and conflating them lets the sim grade its own
// homework.
type ExecutionCostModel interface {
	EstimateCost(domain.ExecutionCostRequest) (domain.ExecutionCostEstimate, error)
}

// ReferenceData is the dated-revision seam (§40b/§40d): fees, lot rules and
// the hurdle benchmark as three as-of faces of ONE mechanism — dated
// reference revisions with provenance refusal, one store, one step-5 replay
// applicator. Three one-method ports would be three shallow doors over the
// same seam, and the round-4 review caught the cost of naming them
// separately: the hurdle, required by DecisionInput, had no source at all.
//
// # The asOf parameter is the archive's seam, ahead of the archive
//
// config.Store holds one atomic CURRENT snapshot; a scored sample spanning
// months needs PERIOD-CORRECT revisions, and the append-only revision
// repository + replay applicator that serve that are step 5 work,
// pre-registered in §40k-2. What is fixed now is the port shape those
// arrive behind: every answer is as-of an instant, and an instant for which
// no period-correct revision exists is a REFUSAL NAMING THE INPUT — never a
// silent apply-current.
//
// [SPEC EDIT OWED] §2.4's cost sentence gains two clauses (§40d, taken
// deliberately at this port, landed in the core spec before step 3 closes):
// (i) the composition rule — fetched fees land as dated revisions; the
// decision path (live and replay) reads a revision as-of its instant; a
// revision without provenance is refused; (ii) the milestone-2
// authorization — the pre-capital gate MAY be scored on a sourced, dated,
// published-tier revision, labeled modeled-tier; account-real authenticated
// fee revisions are a precondition of any live order. Without (i), §2.4
// reads as demanding a scoring-time venue call §3.5 forbids; without (ii),
// the milestone-2 gate is non-conforming the day it is scored and the record
// absorbs the violation silently.
type ReferenceData interface {
	Fees(inst ids.VenueInstrument, asOf time.Time) (domain.LegFees, error)
	LotRules(inst ids.VenueInstrument, asOf time.Time) (domain.LotRules, error)
	Hurdle(asOf time.Time) (domain.HurdleBenchmark, error)
}

// DecisionSink is where a Decision becomes durable (§40f/§40h).
//
// # Write returns success ONLY after the decision is flushed and fsynced
//
// The semantics are write-ahead: Decide sinks the Decision before returning
// any Intent derived from it, and a failed write suppresses the Intents AND
// surfaces the error. The container's default Data path appends and fsyncs
// on a periodic tick — right for eight market streams at market rate, and
// wrong here, because torn-tail recovery cannot recover bytes never made
// durable: "write-ahead" would quietly mean "write-eventually". A decision
// stream runs at decision rate, so per-frame fsync costs nothing that
// matters. The milestone-2 adapter is capture-lane framing over
// engine/capture's codec (k:"decision", the §40f wire DTO), owned by the
// step 5 runner; test adapters are in-memory.
type DecisionSink interface {
	Write(domain.Decision) error
}
