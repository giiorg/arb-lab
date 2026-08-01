package crossvenue

// The research simulator (§45a deliverable 2, §45e's frame): walks the
// capture's decision-time streams into episodes, prices each episode's
// accrual from the settled REST store through the phase ledger, charges
// the §8 conservative round trip from our own measured books, and scores
// the pre-declared portfolio at the bootstrap LCB. Every parameter is an
// argument (§41e doctrine — slice D pins the values beside their
// rationale); every exclusion is counted; the post-cutoff seal is
// structural (seal.go).
//
// # The cost composition, stated once
//
// Per venue, per side: the walked cell is the WORSE of the two boundary
// instants' standing walks (the worse side is charged at both
// transitions, so both boundaries are its instants), then WorseOf against
// the venue's p90-conditional-on-size, then WorseSide across the venue's
// two sides. RoundTrip charges each venue's fee and selected bound twice.
// Conservatism enters at those named selections and nowhere else.
//
// # Refusals, never defaults (§45e)
//
// An episode whose boundary book is missing, thin, or unmeasurable is
// REFUSED BY NAME and excluded from the return series — reported beside
// the scored episodes, in EndImbalance's mould: whether a refusal voids
// the run is the procedure commit's rule, and an exploration report that
// silently defaulted a cost would tune parameters against free fills.

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/giiorg/arb-lab/internal/engine/capture"
	"github.com/giiorg/arb-lab/internal/platform/events"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/strategy/domain/calibrate"
	dcv "github.com/giiorg/arb-lab/internal/strategy/domain/crossvenue"
)

// Sim sentinels.
var (
	// ErrLabelUnset reports an exploration run with no label. Ruling
	// §45f-2's trail is "labeled and logged"; an unlabeled run cannot be
	// cited by the procedure commit's informed-not-tuned audit.
	ErrLabelUnset = fmt.Errorf("strategy/crossvenue: exploration run label unset")

	// ErrSimParamsInvalid reports a sim procedure that is not completely
	// declared.
	ErrSimParamsInvalid = fmt.Errorf("strategy/crossvenue: sim parameters incomplete")
)

// SymbolSpec is one walked symbol, its committed candidate size, and its
// committed entry level — per symbol by ruling §45n-4: the screening
// ranked each symbol at its own best-of-four level and carried the
// selection openly, and a common level would measure a different strategy
// from the one the ranking evidenced.
type SymbolSpec struct {
	Symbol string
	Qty    fixedpoint.Quantity
	Entry  fixedpoint.Rate
}

// SimParams is the complete exploration procedure. Values are slice-D
// parameters; this struct only refuses an undeclared one.
type SimParams struct {
	Symbols []SymbolSpec

	// Sample is the shared sampling procedure for signals and books.
	Sample SampleParams

	// Episode rules (dcv.Episodes' arguments; entry is per symbol).
	ExitDivisor int
	MaxGap      time.Duration

	// VoidImbalance, when set (ruling §45n-3, the gate's rule), refuses an
	// episode whose legs settled unequal counts inside its window instead
	// of scoring it — on same-grid pairs an imbalanced end usually marks
	// missing settled rows or a venue interval change, not economics.
	VoidImbalance bool

	// Round is the settlement pairing bucket (dcv.Pair's argument).
	Round time.Duration

	// Fees are per-leg taker rates; P90 is the conditional quantile the
	// distributions are read at.
	BinanceFee fixedpoint.Rate
	BybitFee   fixedpoint.Rate
	P90        fixedpoint.Rate

	// Period is the shared return grid's step.
	Period time.Duration

	// Policy and the bootstrap parameters (ruling §45f-3's statistic).
	Policy     []dcv.Allocation
	MeanBlock  int
	Resamples  int
	Confidence fixedpoint.Rate
	Seed       uint64

	// Label names the exploration run in the audit trail.
	Label string
}

// Validate refuses an incomplete procedure. Domain-owned rules (entry
// positivity, policy weights, bootstrap ranges) are refused by the domain
// calls that own them; what is checked here is only what nothing
// downstream would catch.
func (p SimParams) Validate() error {
	if strings.TrimSpace(p.Label) == "" {
		return ErrLabelUnset
	}
	if len(p.Symbols) == 0 {
		return fmt.Errorf("no symbols: %w", ErrSimParamsInvalid)
	}
	if p.Period <= 0 {
		return fmt.Errorf("period %s: %w", p.Period, ErrSimParamsInvalid)
	}
	return p.Sample.Validate()
}

// EpisodeReport is one episode's economics — or its named refusal.
type EpisodeReport struct {
	OpenAt, CloseAt time.Time
	Sign            int
	Points          int

	// Refused, when non-empty, names why this episode was excluded from
	// the return series; the value fields below it are then unset.
	Refused string

	Net          string
	Cost         string
	Return       string
	EndImbalance int
}

// SymbolReport is one symbol's walk evidence.
type SymbolReport struct {
	Symbol string

	BinanceFrames, BybitFrames   int
	BinanceSkipped, BybitSkipped int
	Merge                        MergeStats

	Pairing        dcv.PairStats
	SealedExcluded int

	BinanceBooks, BybitBooks CostSeries `json:"-"`

	// The frozen distribution reads, decimal strings for the report.
	BnP90Bid, BnP90Ask, ByP90Bid, ByP90Ask string

	Episodes []EpisodeReport
	Scored   int
	RefusedN int

	// NonZeroReturns renders the symbol's period series sparsely: period
	// start (RFC3339) → summed return, for every period a scored episode
	// landed in. The full series is zeros elsewhere — idle periods
	// contribute zero BY CONSTRUCTION (the utilization honesty), so only
	// the exceptions are worth bytes, and the attribution instant is
	// visible instead of buried in an index.
	NonZeroReturns map[string]string
}

// SimReport is one labeled exploration run.
type SimReport struct {
	Label     string
	Days      []string
	DayHashes []string

	Symbols []SymbolReport

	// PortfolioReturns' length and the LCB verdict statistic.
	Periods      int
	PortfolioLCB string
}

// RunSim executes the exploration procedure over the pre-cutoff corpus.
// The seal is structural: archive enumeration is bounded by BooksCutoff,
// settled rows at or past SettledCutoff are excluded and counted.
func RunSim(archiveRoot, restRoot string, p SimParams) (SimReport, error) {
	if err := p.Validate(); err != nil {
		return SimReport{}, err
	}
	days, err := LoadManifestDays(archiveRoot, BooksCutoff)
	if err != nil {
		return SimReport{}, err
	}
	rep, series, err := runOver(archiveRoot, restRoot, p, days, sealSettled)
	if err != nil {
		return SimReport{}, err
	}
	rets, err := dcv.PortfolioReturns(p.Policy, series)
	if err != nil {
		return SimReport{}, fmt.Errorf("portfolio: %w", err)
	}
	lcb, err := dcv.StationaryBootstrapLCB(rets, p.MeanBlock, p.Resamples, p.Confidence, p.Seed)
	if err != nil {
		return SimReport{}, fmt.Errorf("portfolio LCB: %w", err)
	}
	rep.Periods = len(rets)
	rep.PortfolioLCB = lcb.Decimal()
	return rep, nil
}

// RunGateSymbols is the procedure commit's door through the §45f-2 seal:
// it walks the POST-cutoff evidence window — archive days in
// [BooksCutoff's day, before's day) and settled rows in [SettledCutoff,
// before) — and returns the per-symbol evidence plus each symbol's period
// series, leaving the verdict composition (ceiling, policy, portfolio
// statistic, hurdle) to the committed gate runner, which must exclude
// ceiling-refused symbols BEFORE any portfolio statistic exists. Nothing
// else reaches past the seal; the exploration CLI has no path here.
func RunGateSymbols(archiveRoot, restRoot string, p SimParams, before time.Time) (SimReport, map[string][]fixedpoint.Rate, error) {
	if err := p.Validate(); err != nil {
		return SimReport{}, nil, err
	}
	if !before.After(BooksCutoff) {
		return SimReport{}, nil, fmt.Errorf("gate bound %s not after the books cutoff: %w",
			before.Format(time.RFC3339), ErrPostCutoffSealed)
	}
	days, err := LoadManifestDaysWindow(archiveRoot, BooksCutoff, before)
	if err != nil {
		return SimReport{}, nil, err
	}
	return runOver(archiveRoot, restRoot, p, days, func(rows []dcv.Settlement) ([]dcv.Settlement, int) {
		return gateSettled(rows, before)
	})
}

func runOver(archiveRoot, restRoot string, p SimParams, days []ManifestDay,
	seal func([]dcv.Settlement) ([]dcv.Settlement, int)) (SimReport, map[string][]fixedpoint.Rate, error) {
	rep := SimReport{Label: p.Label}
	for _, d := range days {
		rep.Days = append(rep.Days, d.Day)
		rep.DayHashes = append(rep.DayHashes, d.SHA256)
	}

	gridStart, err := time.ParseInLocation("2006-01-02", days[0].Day, time.UTC)
	if err != nil {
		return SimReport{}, nil, fmt.Errorf("grid start: %w", err)
	}
	gridEnd, err := time.ParseInLocation("2006-01-02", days[len(days)-1].Day, time.UTC)
	if err != nil {
		return SimReport{}, nil, fmt.Errorf("grid end: %w", err)
	}
	gridEnd = gridEnd.Add(24 * time.Hour)
	periods := int(gridEnd.Sub(gridStart) / p.Period)
	if gridStart.Add(time.Duration(periods) * p.Period).Before(gridEnd) {
		periods++ // the trailing partial period holds its closes too
	}

	series := map[string][]fixedpoint.Rate{}
	for _, spec := range p.Symbols {
		sr, symSeries, err := runSymbol(archiveRoot, restRoot, days, spec, p, gridStart, periods, seal)
		if err != nil {
			return SimReport{}, nil, fmt.Errorf("symbol %s: %w", spec.Symbol, err)
		}
		rep.Symbols = append(rep.Symbols, sr)
		series[spec.Symbol] = symSeries
	}
	return rep, series, nil
}

func runSymbol(archiveRoot, restRoot string, days []ManifestDay, spec SymbolSpec, p SimParams,
	gridStart time.Time, periods int, seal func([]dcv.Settlement) ([]dcv.Settlement, int)) (SymbolReport, []fixedpoint.Rate, error) {
	sym := spec.Symbol
	sr := SymbolReport{Symbol: sym}

	bnInst := ids.VenueInstrument{Venue: "binance", Product: ids.ProductLinearPerp, Contract: ids.ContractLinear, Symbol: sym}
	byInst := ids.VenueInstrument{Venue: "bybit", Product: ids.ProductLinearPerp, Contract: ids.ContractLinear, Symbol: sym}
	lower := strings.ToLower(sym)

	bnFundingKey := capture.StreamKey{Source: events.SourceID{Instrument: bnInst, Stream: events.StreamFunding}, Topic: lower + "@markPrice@1s"}
	bnBookKey := capture.StreamKey{Source: events.SourceID{Instrument: bnInst, Stream: events.StreamBook}, Topic: lower + "@depth20@100ms"}
	byFundingKey := capture.StreamKey{Source: events.SourceID{Instrument: byInst, Stream: events.StreamFunding}, Topic: "tickers." + sym}
	byBookKey := capture.StreamKey{Source: events.SourceID{Instrument: byInst, Stream: events.StreamBook}, Topic: "orderbook.50." + sym}

	// 1. Decision-time signal.
	bn, err := SampleBinanceEstimates(archiveRoot, days, bnFundingKey, p.Sample)
	if err != nil {
		return SymbolReport{}, nil, err
	}
	by, err := SampleBybitEstimates(archiveRoot, days, byFundingKey, p.Sample)
	if err != nil {
		return SymbolReport{}, nil, err
	}
	sr.BinanceFrames, sr.BinanceSkipped = bn.Frames, bn.Skipped
	sr.BybitFrames, sr.BybitSkipped = by.Frames, by.Skipped

	points, mstats, err := MergeDecisionPoints(bn.Points, by.Points)
	if err != nil {
		return SymbolReport{}, nil, err
	}
	sr.Merge = mstats

	// 2. Episodes, at the symbol's own committed entry level (§45n-4).
	windows, err := dcv.Episodes(points, spec.Entry, p.ExitDivisor, p.MaxGap)
	if err != nil {
		return SymbolReport{}, nil, err
	}

	// 3. Boundary instants wanted from both venues' books.
	var wanted []time.Time
	for _, w := range windows {
		wanted = append(wanted, w.OpenAt, w.CloseAt)
	}

	bnBooks, err := WalkBinanceBookCosts(archiveRoot, days, bnBookKey, bnInst, p.Sample, spec.Qty, wanted)
	if err != nil {
		return SymbolReport{}, nil, err
	}
	byBooks, err := WalkBybitBookCosts(archiveRoot, days, byBookKey, byInst, p.Sample, spec.Qty, wanted)
	if err != nil {
		return SymbolReport{}, nil, err
	}
	sr.BinanceBooks, sr.BybitBooks = bnBooks, byBooks

	// 4. Settled truth, sealed.
	bnRows, _, err := LoadSettled(restRoot, "binance", sym)
	if err != nil {
		return SymbolReport{}, nil, err
	}
	byRows, _, err := LoadSettled(restRoot, "bybit", sym)
	if err != nil {
		return SymbolReport{}, nil, err
	}
	bnSettled, bnExcluded := seal(bnRows)
	bySettled, byExcluded := seal(byRows)
	sr.SealedExcluded = bnExcluded + byExcluded

	_, pstats, err := dcv.Pair(bnSettled, bySettled, p.Round)
	if err != nil {
		return SymbolReport{}, nil, err
	}
	sr.Pairing = pstats

	// 5. The frozen distribution reads. Computed lazily: a symbol with no
	// episodes needs no p90, and an empty distribution must refuse only
	// where a cost was actually to be charged.
	type p90s struct {
		bid, ask fixedpoint.Rate
		err      error
	}
	readP90 := func(cs CostSeries) p90s {
		bid, errB := calibrate.UpperQuantile(cs.BidSlips, p.P90)
		if errB != nil {
			return p90s{err: errB}
		}
		ask, errA := calibrate.UpperQuantile(cs.AskSlips, p.P90)
		if errA != nil {
			return p90s{err: errA}
		}
		return p90s{bid: bid, ask: ask}
	}
	var bnP90, byP90 p90s
	if len(windows) > 0 {
		bnP90, byP90 = readP90(bnBooks), readP90(byBooks)
		if bnP90.err == nil {
			sr.BnP90Bid, sr.BnP90Ask = bnP90.bid.Decimal(), bnP90.ask.Decimal()
		}
		if byP90.err == nil {
			sr.ByP90Bid, sr.ByP90Ask = byP90.bid.Decimal(), byP90.ask.Decimal()
		}
	}

	// 6. Episode economics.
	symSeries := make([]fixedpoint.Rate, periods)
	for i := range symSeries {
		symSeries[i] = fixedpoint.ZeroRate()
	}

	venueSlip := func(cs CostSeries, q p90s, w dcv.Window) (fixedpoint.Rate, string, error) {
		if q.err != nil {
			return fixedpoint.Rate{}, fmt.Sprintf("no cost distribution: %v", q.err), nil
		}
		open, okO := cs.AtWanted[w.OpenAt.UnixMilli()]
		closeS, okC := cs.AtWanted[w.CloseAt.UnixMilli()]
		if !okO || !open.OK {
			return fixedpoint.Rate{}, boundaryReason("open", open, okO), nil
		}
		if !okC || !closeS.OK {
			return fixedpoint.Rate{}, boundaryReason("close", closeS, okC), nil
		}
		bidWalked, err := dcv.WorseOf(open.Bid, closeS.Bid)
		if err != nil {
			return fixedpoint.Rate{}, "", err
		}
		askWalked, err := dcv.WorseOf(open.Ask, closeS.Ask)
		if err != nil {
			return fixedpoint.Rate{}, "", err
		}
		bidCell, err := dcv.WorseOf(bidWalked, q.bid)
		if err != nil {
			return fixedpoint.Rate{}, "", err
		}
		askCell, err := dcv.WorseOf(askWalked, q.ask)
		if err != nil {
			return fixedpoint.Rate{}, "", err
		}
		slip, err := dcv.WorseSide(bidCell, askCell)
		if err != nil {
			return fixedpoint.Rate{}, "", err
		}
		return slip, "", nil
	}

	for _, w := range windows {
		er := EpisodeReport{OpenAt: w.OpenAt, CloseAt: w.CloseAt, Sign: w.Sign, Points: w.Points}

		ledger, err := dcv.WindowAccrual(w, bnSettled, bySettled)
		if err != nil {
			return SymbolReport{}, nil, fmt.Errorf("episode %s: %w", w.OpenAt.Format(time.RFC3339), err)
		}
		er.EndImbalance = ledger.EndImbalance

		var refuse string
		if p.VoidImbalance && ledger.EndImbalance != 0 {
			// Ruling §45n-3: an imbalanced end voids the episode by name.
			refuse = fmt.Sprintf("end imbalance %d", ledger.EndImbalance)
			er.Refused = refuse
			sr.RefusedN++
			sr.Episodes = append(sr.Episodes, er)
			continue
		}

		bnSlip, refuse, err := venueSlip(bnBooks, bnP90, w)
		if err != nil {
			return SymbolReport{}, nil, err
		}
		if refuse == "" {
			var bySlip fixedpoint.Rate
			bySlip, refuse, err = venueSlip(byBooks, byP90, w)
			if err != nil {
				return SymbolReport{}, nil, err
			}
			if refuse == "" {
				cost, err := dcv.RoundTrip(p.BinanceFee, bnSlip, p.BybitFee, bySlip)
				if err != nil {
					return SymbolReport{}, nil, fmt.Errorf("episode %s: %w", w.OpenAt.Format(time.RFC3339), err)
				}
				ret, err := ledger.Net.Sub(cost)
				if err != nil {
					return SymbolReport{}, nil, fmt.Errorf("episode %s: %w", w.OpenAt.Format(time.RFC3339), err)
				}
				idx := int(w.CloseAt.Sub(gridStart) / p.Period)
				if idx < 0 || idx >= periods {
					return SymbolReport{}, nil, fmt.Errorf("episode close %s outside the period grid", w.CloseAt.Format(time.RFC3339))
				}
				symSeries[idx], err = symSeries[idx].Add(ret)
				if err != nil {
					return SymbolReport{}, nil, fmt.Errorf("episode %s: %w", w.OpenAt.Format(time.RFC3339), err)
				}
				er.Net, er.Cost, er.Return = ledger.Net.Decimal(), cost.Decimal(), ret.Decimal()
				sr.Scored++
			}
		}
		if refuse != "" {
			er.Refused = refuse
			sr.RefusedN++
		}
		sr.Episodes = append(sr.Episodes, er)
	}

	sort.Slice(sr.Episodes, func(i, j int) bool { return sr.Episodes[i].OpenAt.Before(sr.Episodes[j].OpenAt) })

	sr.NonZeroReturns = map[string]string{}
	for i, r := range symSeries {
		s, err := r.Sign()
		if err != nil {
			return SymbolReport{}, nil, fmt.Errorf("series period %d: %w", i, err)
		}
		if s != 0 {
			sr.NonZeroReturns[gridStart.Add(time.Duration(i)*p.Period).Format(time.RFC3339)] = r.Decimal()
		}
	}
	return sr, symSeries, nil
}

func boundaryReason(which string, bs BoundarySample, present bool) string {
	if !present {
		return fmt.Sprintf("%s boundary not on a sampled instant", which)
	}
	if bs.Reason != "" {
		return fmt.Sprintf("%s boundary: %s", which, bs.Reason)
	}
	return fmt.Sprintf("%s boundary book unmeasured", which)
}
