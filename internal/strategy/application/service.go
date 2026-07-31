package application

import (
	"fmt"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/strategy/application/ports"
	"github.com/giiorg/arb-lab/internal/strategy/domain"
	"github.com/giiorg/arb-lab/internal/strategy/domain/cashcarry"
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

	// ErrTooManyPositions reports more than one held package. The MVP
	// universe is one pair, so a second position is evidence of a caller
	// bug, not a portfolio.
	ErrTooManyPositions = fmt.Errorf("strategy: more than one held position")

	// ErrForeignPosition reports a held package on instruments this service
	// was not wired for — a caller bug, never a refusal: no decision about
	// the configured pair can be evidence about someone else's.
	ErrForeignPosition = fmt.Errorf("strategy: held position is not the configured pair")

	// ErrEntryContinuation reports continuation state supplied to an ENTRY
	// evaluation. There is no prior decision for a position that does not
	// exist; state from one is a caller wiring two evaluations together.
	ErrEntryContinuation = fmt.Errorf("strategy: continuation state on an entry evaluation")

	// ErrSinkFailed wraps a DecisionSink write failure. §40f: a failed sink
	// write returns zero Intents AND a non-nil error, and the runner treats
	// it as run-aborting — a run that silently stopped journaling is a
	// scored sample with missing evidence, worse than a stopped run.
	ErrSinkFailed = fmt.Errorf("strategy: decision sink write failed")
)

// Pre-core refusal rules (§40f: the service constructs these; the core never
// sees an invalid input). Same journal-vocabulary convention as the
// cashcarry constants: a RefusedBy is one of these, optionally ": <detail>".
const (
	// RefuseMarketAbsent reports a read port with no entry for a configured
	// instrument — an empty cache at boot is a state, not a fault, and the
	// decision it forces is a refusal, not an error.
	RefuseMarketAbsent = "service/market_state_absent"

	// RefuseStale reports a leg whose watermark age at the evaluation
	// instant exceeds the configured budget (§3.4 read-time re-judgement).
	RefuseStale = "service/view_stale"

	// RefuseSkew reports cross-leg receive-time skew beyond the configured
	// limit — two fresh legs that describe different instants.
	RefuseSkew = "service/skew_breach"

	// RefusePort reports a driven port that errored during assembly.
	RefusePort = "service/port_failure"

	// RefuseAssembly reports an assembled input that failed validation — an
	// unpopulated member, a legs mismatch, a horizon hole.
	RefuseAssembly = "service/input_invalid"

	// RefuseContinuation reports exit feedback state nobody fed back: the
	// working plan revision or the dwell missing. §40b: the dwell is an
	// input fed back from the prior journaled Decision, and forgetting it
	// must be a refusal naming the field, not a dwell that silently
	// restarts.
	RefuseContinuation = "service/continuation_missing"

	// RefuseCore reports the evaluation core failing on a validated input —
	// arithmetic that could not complete. The refusal keeps the journal
	// hole-free; the detail carries the fault.
	RefuseCore = "service/core_fault"
)

// Deps are the Service's wired dependencies: the injected clock, the ID
// generator, and §40b's seven ports.
type Deps struct {
	// Clock is the ONLY time source (§3.6 rule 5). Intent.ValidUntil and
	// every evaluation instant come from here, which is what makes the sim
	// pump and a future live pump the same engine (§3.5).
	Clock clock.Clock

	// IDs mints IntentIDs. Injected for §3.5's reason exactly as the clock
	// is: a defaulted crypto/rand would make a replay non-reproducible.
	IDs *ids.Generator

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
		{"id generator", d.IDs == nil},
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

// Settings are the service's run-policy wiring: the §5.2 values that are
// neither strategy params (those ride the PolicyProjection) nor port data.
// Every member is required — each one's zero value flatters the gate or
// disables a guard, the exact silence the zero-value-unusable rule exists
// for.
type Settings struct {
	// CandidateQty is the explicit candidate the sizing is scored at
	// (§40b: never inferred back out of C_allocated — the core separately
	// refuses a candidate the declared capital cannot fund).
	CandidateQty fixedpoint.Quantity

	// ResidualTolerance is §2.4's residual-delta tolerance. A declared zero
	// is legitimate strictness (only exact grid fits enter).
	ResidualTolerance fixedpoint.Quantity

	// Tactic is the execution pattern priced by the cost model. Taker-IOC
	// is §2.4's default; maker-entry is the documented opt-in, never
	// silent.
	Tactic domain.ExecTactic

	// Latency is the decision-to-venue latency assumption handed to the
	// cost model.
	Latency time.Duration

	// MaxBookAge and MaxFundingAge are the §3.4 freshness budgets the
	// service judges watermarks against at the read.
	MaxBookAge, MaxFundingAge time.Duration

	// SkewLimit is the §3.4 cross-leg skew limit. §5.2's own warning: a
	// zero here admits only two legs delivered in the same instant, so the
	// engine never trades with nothing reporting an error — refused at
	// construction, by name.
	SkewLimit time.Duration

	// IntentTTL bounds how long an emitted Intent may be executed.
	IntentTTL time.Duration
}

// Validate names the first unusable member.
func (s Settings) Validate() error {
	if err := s.CandidateQty.Validate(); err != nil {
		return fmt.Errorf("candidate qty: %w", err)
	}
	if sign, err := s.CandidateQty.Sign(); err != nil {
		return fmt.Errorf("candidate qty: %w", err)
	} else if sign <= 0 {
		return fmt.Errorf("candidate qty %s: %w", s.CandidateQty, domain.ErrUnset)
	}
	if err := s.ResidualTolerance.Validate(); err != nil {
		return fmt.Errorf("residual tolerance: %w", err)
	}
	if err := s.Tactic.Validate(); err != nil {
		return fmt.Errorf("tactic: %w", err)
	}
	for _, c := range []struct {
		name string
		d    time.Duration
	}{
		{"latency", s.Latency},
		{"max book age", s.MaxBookAge},
		{"max funding age", s.MaxFundingAge},
		{"skew limit", s.SkewLimit},
		{"intent ttl", s.IntentTTL},
	} {
		if c.d <= 0 {
			return fmt.Errorf("%s %v: %w", c.name, c.d, domain.ErrUnset)
		}
	}
	return nil
}

// Continuation is the exit direction's fed-back state: the current journaled
// HoldPlanRevision and the prior dwell, both from the last journaled
// Decision (§40f). The runner owns the feedback loop — a core or service
// keeping this across calls would make the dwell a counter that silently
// restarts on redeploy.
type Continuation struct {
	Plan  domain.HoldPlanRevision
	Dwell domain.DwellState
}

func (c Continuation) isZero() bool {
	return c.Dwell == (domain.DwellState{}) &&
		c.Plan.Num == 0 && c.Plan.Trigger == "" && c.Plan.At.IsZero() &&
		c.Plan.Plan.Settlements == nil && c.Plan.Plan.Terminal.IsZero()
}

// Service is the cashcarry application service. Its Decide method is the
// production inbound door (§40f); the §3.3 Strategy face is Adapter's.
//
// It is deliberately CONCRETE: the use case is the driving port, and nothing
// substitutes a use case (the rule marketdata's ports doc records). What
// varies — venues, models, storage — varies behind the driven ports in Deps.
type Service struct {
	deps     Deps
	spot     ids.VenueInstrument
	perp     ids.VenueInstrument
	policy   domain.PolicyProjection
	settings Settings
	capital  domain.CapitalAllocation
}

// NewService wires the service for one spot+perp pair.
//
// The policy projection is construction-fixed for the milestone-2
// single-revision world (hot-reload re-projection is milestone 3's); it
// carries the revision identity every journaled Decision cites. The capital
// decomposition is likewise declared wiring — §7.1's live computation is
// milestone 4, and the gate needs the number as a provenance-carrying input,
// not the machinery.
func NewService(deps Deps, spot, perp ids.VenueInstrument, policy domain.PolicyProjection,
	settings Settings, capital domain.CapitalAllocation) (*Service, error) {
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
	if err := policy.Validate(); err != nil {
		return nil, fmt.Errorf("policy: %w", err)
	}
	if err := settings.Validate(); err != nil {
		return nil, fmt.Errorf("settings: %w", err)
	}
	if err := capital.Validate(); err != nil {
		return nil, fmt.Errorf("capital: %w", err)
	}
	return &Service{deps: deps, spot: spot, perp: perp, policy: policy,
		settings: settings, capital: capital}, nil
}

// Name is the §5.2 registry key.
func (s *Service) Name() string { return "cashcarry" }

// Universe is the configured pair, spot first. A fresh slice per call: the
// universe is static (§3.3) and a caller must not be able to edit it.
func (s *Service) Universe() []ids.VenueInstrument {
	return []ids.VenueInstrument{s.spot, s.perp}
}

// Params is the §3.3 typed-params face.
func (s *Service) Params() domain.StrategyParams { return s.policy.Params }

// Decide is the production door (§40f): assemble the DecisionInput from the
// ports, run the core, sink the Decision write-ahead, then — and only then —
// derive Intents.
//
// held is the caller's position facts: empty for an entry evaluation,
// exactly one for continue-vs-close. cont is the exit direction's fed-back
// state from the prior journaled Decision; it MUST be zero for an entry.
//
// The contract: Decide writes the Decision to the DecisionSink BEFORE
// returning any Intent derived from it; a failed sink write returns zero
// Intents AND a non-nil error the runner treats as run-aborting. Pre-core
// refusals are constructed here, sunk, and returned with no Intents and a
// nil error — the refusal is the gate working, and the journal shows it.
// No Intent without its journaled Decision.
func (s *Service) Decide(held []domain.PositionView, cont Continuation) (domain.Decision, []domain.Intent, error) {
	if len(held) > 1 {
		return domain.Decision{}, nil, fmt.Errorf("%d positions: %w", len(held), ErrTooManyPositions)
	}
	if len(held) == 0 {
		if !cont.isZero() {
			return domain.Decision{}, nil, ErrEntryContinuation
		}
		return s.decideEntry()
	}
	p := held[0]
	if p.Spot != s.spot || p.Perp != s.perp {
		return domain.Decision{}, nil, fmt.Errorf("%s/%s: %w", p.Spot, p.Perp, ErrForeignPosition)
	}
	return s.decideExit(p, cont)
}

// decideEntry assembles and scores the entry direction.
func (s *Service) decideEntry() (domain.Decision, []domain.Intent, error) {
	at := s.deps.Clock.Now()
	in := domain.DecisionInput{Kind: domain.KindEntry, At: at}

	view, rule, detail := s.assembleView(at)
	if rule != "" {
		return s.sinkRefusal(in, rule, detail)
	}
	in.View = view
	in.Policy = s.policy

	// The entry proposal: the minimum policy-admissible hold from the live
	// schedule, revision 1.
	plan, err := cashcarry.ProposeHoldPlan(view.Funding, s.policy.Params.MinHoldSettlements)
	if err != nil {
		return s.sinkRefusal(in, RefusePort, fmt.Sprintf("propose plan: %v", err))
	}
	in.Plan = domain.HoldPlanRevision{Num: 1, Trigger: domain.TriggerEntry, At: at, Plan: plan}

	if rule, detail := s.assembleShared(&in, plan.Settlements, s.settings.CandidateQty); rule != "" {
		return s.sinkRefusal(in, rule, detail)
	}
	if err := in.Validate(); err != nil {
		return s.sinkRefusal(in, RefuseAssembly, err.Error())
	}

	d, err := cashcarry.Enter(in)
	if err != nil {
		return s.sinkRefusal(in, RefuseCore, err.Error())
	}
	return s.sinkAndEmit(d)
}

// decideExit assembles and scores the exit direction over the held package.
func (s *Service) decideExit(p domain.PositionView, cont Continuation) (domain.Decision, []domain.Intent, error) {
	at := s.deps.Clock.Now()
	in := domain.DecisionInput{Kind: domain.KindExit, At: at, Position: p, Dwell: cont.Dwell}

	if cont.Plan.Num == 0 || !cont.Dwell.Declared {
		return s.sinkRefusal(in, RefuseContinuation,
			fmt.Sprintf("plan revision %d, dwell declared %v", cont.Plan.Num, cont.Dwell.Declared))
	}

	view, rule, detail := s.assembleView(at)
	if rule != "" {
		return s.sinkRefusal(in, rule, detail)
	}
	in.View = view
	in.Policy = s.policy

	// §40f's regeneration rule: if the venue's live schedule moved, the
	// working plan revision advances — journaled with its trigger by riding
	// the Decision this evaluation sinks.
	rev, _, err := cashcarry.RegeneratePlan(cont.Plan, view.Funding, at)
	if err != nil {
		return s.sinkRefusal(in, RefuseContinuation, fmt.Sprintf("regenerate: %v", err))
	}
	in.Plan = rev

	var remaining []time.Time
	for _, t := range rev.Plan.Settlements {
		if t.After(at) {
			remaining = append(remaining, t)
		}
	}
	if rule, detail := s.assembleShared(&in, remaining, p.Qty); rule != "" {
		return s.sinkRefusal(in, rule, detail)
	}
	if err := in.Validate(); err != nil {
		return s.sinkRefusal(in, RefuseAssembly, err.Error())
	}

	d, err := cashcarry.Continue(in)
	if err != nil {
		return s.sinkRefusal(in, RefuseCore, err.Error())
	}
	return s.sinkAndEmit(d)
}

// assembleView reads both legs and the settlement window and judges
// freshness and skew at the read (§3.4). A non-empty rule is a refusal.
func (s *Service) assembleView(at time.Time) (domain.MarketView, string, string) {
	funding, ok := s.deps.Market.Funding(s.perp)
	if !ok {
		return domain.MarketView{}, RefuseMarketAbsent, "funding " + s.perp.String()
	}
	spotTop, ok := s.deps.Market.TopOfBook(s.spot)
	if !ok {
		return domain.MarketView{}, RefuseMarketAbsent, "book " + s.spot.String()
	}
	perpTop, ok := s.deps.Market.TopOfBook(s.perp)
	if !ok {
		return domain.MarketView{}, RefuseMarketAbsent, "book " + s.perp.String()
	}

	for _, c := range []struct {
		name   string
		recv   time.Time
		budget time.Duration
	}{
		{"spot book", spotTop.Watermark.RecvTime, s.settings.MaxBookAge},
		{"perp book", perpTop.Watermark.RecvTime, s.settings.MaxBookAge},
		{"funding", funding.Watermark.RecvTime, s.settings.MaxFundingAge},
	} {
		if age := at.Sub(c.recv); age > c.budget {
			return domain.MarketView{}, RefuseStale,
				fmt.Sprintf("%s age %v over budget %v", c.name, age, c.budget)
		}
	}
	skew := spotTop.Watermark.RecvTime.Sub(perpTop.Watermark.RecvTime)
	if skew < 0 {
		skew = -skew
	}
	if skew > s.settings.SkewLimit {
		return domain.MarketView{}, RefuseSkew,
			fmt.Sprintf("cross-leg skew %v over limit %v", skew, s.settings.SkewLimit)
	}

	window, err := s.deps.History.Window(s.perp)
	if err != nil {
		return domain.MarketView{}, RefusePort, fmt.Sprintf("settlement history: %v", err)
	}
	return domain.MarketView{Spot: spotTop, Perp: perpTop, Funding: funding, Realized: window}, "", ""
}

// assembleShared fills the members both directions assemble the same way:
// forecasts over the given horizon, basis, execution cost for the candidate,
// the three reference faces as-of the instant, capital, and the declared
// zeros. A non-empty rule is a refusal.
func (s *Service) assembleShared(in *domain.DecisionInput, horizon []time.Time, candidate fixedpoint.Quantity) (string, string) {
	at := in.At
	forecasts, err := s.deps.Forecast.Estimate(in.View.Materials(), horizon)
	if err != nil {
		return RefusePort, fmt.Sprintf("funding forecast: %v", err)
	}
	in.Forecasts = forecasts

	basis, err := s.deps.Basis.EstimateBasis(in.View, in.Plan, candidate)
	if err != nil {
		return RefusePort, fmt.Sprintf("basis forecast: %v", err)
	}
	in.Basis = basis

	cost, err := s.deps.Cost.EstimateCost(domain.ExecutionCostRequest{
		Spot: s.spot, Perp: s.perp, Qty: candidate,
		Tactic: s.settings.Tactic, Latency: s.settings.Latency,
	})
	if err != nil {
		return RefusePort, fmt.Sprintf("execution cost: %v", err)
	}
	in.ExecCost = cost

	if in.SpotFees, err = s.deps.Reference.Fees(s.spot, at); err != nil {
		return RefusePort, fmt.Sprintf("spot fees: %v", err)
	}
	if in.PerpFees, err = s.deps.Reference.Fees(s.perp, at); err != nil {
		return RefusePort, fmt.Sprintf("perp fees: %v", err)
	}
	if in.SpotLots, err = s.deps.Reference.LotRules(s.spot, at); err != nil {
		return RefusePort, fmt.Sprintf("spot lots: %v", err)
	}
	if in.PerpLots, err = s.deps.Reference.LotRules(s.perp, at); err != nil {
		return RefusePort, fmt.Sprintf("perp lots: %v", err)
	}
	if in.Hurdle, err = s.deps.Reference.Hurdle(at); err != nil {
		return RefusePort, fmt.Sprintf("hurdle: %v", err)
	}

	in.ResidualTolerance = s.settings.ResidualTolerance
	in.CandidateQty = candidate
	in.Capital = s.capital

	// The reserve/opportunity term and the two Δ terms are DECLARED zeros
	// in milestone 2, in the capital's asset: §2.4's own branch-symmetric
	// common case for the Δs, and the hurdle already charges the whole
	// C_allocated — a separate reserve-opportunity line would double-count
	// it. A declared zero with provenance (this code, this reasoning) is
	// the §40b-sanctioned exception; an absent member never is.
	quote := s.capital.SpotPrincipal.Asset()
	in.Opportunity = fixedpoint.ZeroMoney(quote)
	in.ClosingCostDelta = fixedpoint.ZeroMoney(quote)
	in.RiskDelta = fixedpoint.ZeroMoney(quote)
	return "", ""
}

// sinkRefusal journals a pre-core refusal and returns it with no Intents.
// The sink write is still write-ahead: a refusal that failed to journal
// aborts the run exactly as a scored decision would.
func (s *Service) sinkRefusal(in domain.DecisionInput, rule, detail string) (domain.Decision, []domain.Intent, error) {
	refusedBy := rule
	if detail != "" {
		refusedBy = rule + ": " + detail
	}
	d := domain.Decision{
		Kind:      in.Kind,
		Verdict:   domain.VerdictRefuse,
		At:        in.At,
		Input:     in,
		RefusedBy: refusedBy,
	}
	if err := s.deps.Sink.Write(d); err != nil {
		return d, nil, fmt.Errorf("%w: %v", ErrSinkFailed, err)
	}
	return d, nil, nil
}

// sinkAndEmit is the write-ahead tail of the happy path: journal the scored
// Decision, then derive its Intents. Order is the contract — an Intent
// exists only downstream of its journaled Decision.
func (s *Service) sinkAndEmit(d domain.Decision) (domain.Decision, []domain.Intent, error) {
	if err := s.deps.Sink.Write(d); err != nil {
		return d, nil, fmt.Errorf("%w: %v", ErrSinkFailed, err)
	}
	intents, err := s.intentsFor(d)
	if err != nil {
		// The Decision is journaled; the Intent could not be built. That is
		// a fault, not a refusal — surface it so the runner stops rather
		// than reading "no intents" as a scored hold-still.
		return d, nil, err
	}
	return d, intents, nil
}

// intentsFor derives the Intents a verdict implies: an open for Enter, a
// close for Close, nothing otherwise. The open's quantity re-derives the
// core's own TargetQty — the same pure function the verdict priced, so the
// journal and the order can only agree.
func (s *Service) intentsFor(d domain.Decision) ([]domain.Intent, error) {
	var kind domain.IntentKind
	var legs []domain.LegSpec
	var qty fixedpoint.Quantity
	var reason string

	switch d.Verdict {
	case domain.VerdictEnter:
		target, err := cashcarry.TargetQty(d.Input)
		if err != nil {
			return nil, fmt.Errorf("intent target: %w", err)
		}
		kind = domain.IntentOpen
		legs = []domain.LegSpec{
			{Instrument: s.spot, Side: domain.SideBuy},
			{Instrument: s.perp, Side: domain.SideSell},
		}
		qty = target
		reason = "cashcarry entry: quantile carry cleared scaled hurdle + margin"
	case domain.VerdictClose:
		kind = domain.IntentClose
		legs = []domain.LegSpec{
			{Instrument: s.spot, Side: domain.SideSell},
			{Instrument: s.perp, Side: domain.SideBuy},
		}
		qty = d.Input.Position.Qty
		reason = "cashcarry exit: stand-down (band+dwell or plan complete)"
	case domain.VerdictDecline, domain.VerdictContinue, domain.VerdictRefuse:
		return nil, nil
	default:
		return nil, fmt.Errorf("verdict %q: %w", d.Verdict, domain.ErrKindInvalid)
	}

	id, err := s.deps.IDs.NewIntentID()
	if err != nil {
		return nil, fmt.Errorf("mint intent id: %w", err)
	}
	intent := domain.Intent{
		IntentID:   id,
		Revision:   1,
		ValidUntil: d.At.Add(s.settings.IntentTTL),
		Kind:       kind,
		Legs:       legs,
		TargetQty:  qty,
		Reason:     reason,
	}
	if err := intent.Validate(); err != nil {
		return nil, fmt.Errorf("intent: %w", err)
	}
	return []domain.Intent{intent}, nil
}
