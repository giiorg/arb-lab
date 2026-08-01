package driving

// CrossVenueGate.Run — the §45 procedure commit's governed act, and the ONE
// caller of RunGateSymbols, which is the only door through ruling §45f-2's
// post-cutoff seal. Everything §41e demanded of the calibration derivation
// is demanded here for the same reasons: the execution binding is checked
// before a byte is read and recorded in the artifact after, the artifact
// version is write-once, the datasets it consumed are enumerated with their
// digests, and approval is a detached attestation with a cooling delay.
//
// # The order of operations is the governance
//
// Bind the build, read the reference revision, walk the sealed window,
// ADMIT the symbols, and only then compose a statistic. The admission step
// sits where it does deliberately (§45o): once eight series are weighted
// and summed, a symbol scored on a quarter of its episodes is invisible, so
// both admission rules must have run — and the whole run must have refused
// — before any portfolio number exists.
//
// # Why a failed admission writes NOTHING
//
// The pass shape is a PRE-DECLARED portfolio (ruling §45f-3). A symbol that
// cannot be scored cannot be dropped and the rest scored instead: that
// would score a portfolio nobody declared, chosen by which symbol happened
// to be thin, which is the tuning door the pre-declaration exists to shut.
// So a refused symbol refuses the RUN, by name, and the artifact path stays
// unburned for the run that can score. The operator still sees every
// symbol's evidence — the refusal carries the report — they just do not get
// a verdict out of it.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/config"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/strategy/domain"
	"github.com/giiorg/arb-lab/internal/strategy/domain/cashcarry"
	dcv "github.com/giiorg/arb-lab/internal/strategy/domain/crossvenue"
	"github.com/giiorg/arb-lab/internal/strategy/infrastructure/crossvenue"
	"github.com/giiorg/arb-lab/internal/strategy/infrastructure/driven"
)

// Gate sentinels.
var (
	// ErrGateSymbolsUnfit reports a run whose declared roster is not all
	// scoreable — one or more symbols short of the end rule's floor or
	// past the refusal ceiling. The wrapping message names every one of
	// them with its counts, because "the gate is not ready" is useless and
	// "KAITOUSDT has 96 matched settlements, floor 180" is a date.
	ErrGateSymbolsUnfit = fmt.Errorf("strategy/driving: gate roster not scoreable")

	// ErrGateBoundUnset reports a run with no declared evidence bound. The
	// bound is what makes two runs name different universes explicitly, so
	// defaulting it to the wall clock would silently make the artifact's
	// window a function of when somebody typed the command.
	ErrGateBoundUnset = fmt.Errorf("strategy/driving: gate evidence bound unset")
)

// GateConfig is the gate run's world: every path explicit, the evidence
// bound an ARGUMENT.
type GateConfig struct {
	ArchiveRoot   string
	RestRoot      string
	ReferencePath string
	OutPath       string

	// Before is the declared evidence bound: settled rows and archive days
	// are read up to but not including it.
	Before time.Time
}

// Validate reports whether the run is fully specified.
func (c GateConfig) Validate() error {
	if c.ArchiveRoot == "" || c.RestRoot == "" || c.ReferencePath == "" || c.OutPath == "" {
		return fmt.Errorf("gate config paths: %w", domain.ErrUnset)
	}
	if c.Before.IsZero() {
		return ErrGateBoundUnset
	}
	return nil
}

// GateEvidence is everything the sealed walk and the committed reference
// produced, before any admission or statistic. Split out so the admission
// rules and the scoring can be exercised over a constructed value — the
// alternative is a battery that can only reach them through a filesystem.
type GateEvidence struct {
	Report crossvenue.SimReport
	Series map[string][]fixedpoint.Rate

	BinanceTaker, BybitTaker fixedpoint.Rate
	Hurdle                   domain.HurdleBenchmark

	ReferenceSHA256  string
	Revision         string
	ExecutableSHA256 string
	Before           time.Time
}

// GateResult is what the governed act produced.
type GateResult struct {
	ArtifactPath string
	ArtifactID   string
	Hash         string
	Bytes        []byte
	Doc          driven.GateArtifactDoc

	// Evidence is populated whenever the walk completed, INCLUDING on an
	// admission refusal — the operator needs the counts to know how far
	// off the gate is, and a refusal that carried nothing would send them
	// to re-run the walk to find out.
	Evidence GateEvidence
}

// CrossVenueGate is the `arblab crossvenue gate` command's core. The two
// seams mirror Calibrator's, for the same reason: the battery must be able
// to exercise the §41e refusals without building binaries.
type CrossVenueGate struct {
	ReadRevision     func() (revision string, dirty bool, err error)
	ExecutableDigest func() (string, error)
}

// AdmitGateSymbols applies both admission rules to a completed walk and
// returns every refusal, in roster order, as its own message. It is a pure
// function over the report so the two rules' interaction — a symbol can
// fail either, or both — is testable without a walk.
//
// Both rules are evaluated for EVERY symbol rather than stopping at the
// first refusal. A gate that named one short symbol per run would take as
// many runs to enumerate the roster as there are thin symbols, and each of
// those runs is a walk over the whole archive.
func AdmitGateSymbols(rep crossvenue.SimReport) []string {
	var refusals []string
	for _, sr := range rep.Symbols {
		if err := dcv.CheckEvidenceFloor(sr.Symbol, sr.Pairing.Matched, ProcCrossVenueMatchedFloor); err != nil {
			refusals = append(refusals, err.Error())
		}
		if err := dcv.CheckRefusalCeiling(sr.Symbol, sr.RefusedN, len(sr.Episodes), ProcCrossVenueRefusalCeiling); err != nil {
			refusals = append(refusals, err.Error())
		}
	}
	return refusals
}

// Run executes the governed gate and writes the artifact.
func (g CrossVenueGate) Run(cfg GateConfig) (GateResult, error) {
	if err := cfg.Validate(); err != nil {
		return GateResult{}, err
	}

	ev, err := g.measure(cfg)
	if err != nil {
		return GateResult{}, err
	}

	// The admission gate, before any portfolio number exists.
	if refusals := AdmitGateSymbols(ev.Report); len(refusals) > 0 {
		return GateResult{Evidence: ev}, fmt.Errorf("%s: %w", strings.Join(refusals, "; "), ErrGateSymbolsUnfit)
	}

	doc, err := g.assemble(cfg, ev)
	if err != nil {
		return GateResult{Evidence: ev}, err
	}
	if err := doc.Validate(); err != nil {
		return GateResult{Evidence: ev}, fmt.Errorf("assembled gate artifact: %w", err)
	}

	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return GateResult{Evidence: ev}, fmt.Errorf("marshal gate artifact: %w", err)
	}
	out = append(out, '\n')

	if err := GuardNewVersion(cfg.OutPath, func(p string) bool {
		_, statErr := os.Stat(p)
		return statErr == nil
	}); err != nil {
		return GateResult{Evidence: ev}, err
	}
	if err := os.MkdirAll(filepath.Dir(cfg.OutPath), 0o755); err != nil {
		return GateResult{Evidence: ev}, fmt.Errorf("write gate artifact: %w", err)
	}
	if err := os.WriteFile(cfg.OutPath, out, 0o644); err != nil {
		return GateResult{Evidence: ev}, fmt.Errorf("write gate artifact: %w", err)
	}

	// The written bytes must load through the production door — the same
	// emitted-artifact-must-load check the calibration run ends with.
	loaded, err := driven.LoadGateArtifactFile(cfg.OutPath)
	if err != nil {
		return GateResult{Evidence: ev}, fmt.Errorf("emitted gate artifact does not load: %w", err)
	}
	return GateResult{
		ArtifactPath: cfg.OutPath,
		ArtifactID:   loaded.Identity.ID,
		Hash:         loaded.Identity.ContentSHA256,
		Bytes:        out,
		Doc:          doc,
		Evidence:     ev,
	}, nil
}

// measure binds the build, reads the committed reference revision, and
// walks the sealed post-cutoff window.
func (g CrossVenueGate) measure(cfg GateConfig) (GateEvidence, error) {
	// §41e first: nothing is read from a build the artifact cannot name.
	readRev := g.ReadRevision
	if readRev == nil {
		readRev = ReadBuildRevision
	}
	revision, dirty, err := readRev()
	if err != nil {
		return GateEvidence{}, err
	}
	if err := CheckExecutionBinding(revision, dirty, revision); err != nil {
		return GateEvidence{}, err
	}
	execDigest := g.ExecutableDigest
	if execDigest == nil {
		execDigest = selfDigest
	}
	exeSHA, err := execDigest()
	if err != nil {
		return GateEvidence{}, err
	}
	if err := driven.CheckHashText(exeSHA); err != nil {
		return GateEvidence{}, fmt.Errorf("executable digest: %w", err)
	}

	// The committed reference revision: fees and hurdle, read as-of the
	// declared bound rather than pinned in the roster (§2.4/§2.1).
	refData, err := os.ReadFile(cfg.ReferencePath)
	if err != nil {
		return GateEvidence{}, fmt.Errorf("reference: %w", err)
	}
	refStore, err := config.New[driven.Document](cfg.ReferencePath, cfg.Before, refData)
	if err != nil {
		return GateEvidence{}, fmt.Errorf("reference: %w", err)
	}
	ref, err := driven.NewReferenceStore(refStore)
	if err != nil {
		return GateEvidence{}, err
	}
	feeProbe := ProcCrossVenueSymbols[0].Symbol
	bnFees, err := ref.Fees(gatePerpInstrument("binance", feeProbe), cfg.Before)
	if err != nil {
		return GateEvidence{}, fmt.Errorf("binance perp fees: %w", err)
	}
	byFees, err := ref.Fees(gatePerpInstrument("bybit", feeProbe), cfg.Before)
	if err != nil {
		return GateEvidence{}, fmt.Errorf("bybit perp fees: %w", err)
	}
	hurdle, err := ref.Hurdle(cfg.Before)
	if err != nil {
		return GateEvidence{}, fmt.Errorf("hurdle: %w", err)
	}
	refSum := sha256.Sum256(refData)

	p := ProcCrossVenueSimParams(bnFees.Taker, byFees.Taker)
	rep, series, err := crossvenue.RunGateSymbols(cfg.ArchiveRoot, cfg.RestRoot, p, cfg.Before)
	if err != nil {
		return GateEvidence{}, err
	}
	return GateEvidence{
		Report:           rep,
		Series:           series,
		BinanceTaker:     bnFees.Taker,
		BybitTaker:       byFees.Taker,
		Hurdle:           hurdle,
		ReferenceSHA256:  hex.EncodeToString(refSum[:]),
		Revision:         revision,
		ExecutableSHA256: exeSHA,
		Before:           cfg.Before,
	}, nil
}

// gatePerpInstrument names a fee cell. The symbol is the lookup's and does
// not select the row — FeeRow is keyed on (venue, product) because venue
// base-tier schedules are schedule-wide — but it must still be a valid
// instrument, so the roster's first symbol stands in rather than an empty
// string that Validate would refuse.
func gatePerpInstrument(venue, symbol string) ids.VenueInstrument {
	return ids.VenueInstrument{
		Venue:    ids.Venue(venue),
		Product:  ids.ProductLinearPerp,
		Contract: ids.ContractLinear,
		Symbol:   symbol,
	}
}

// assemble builds the artifact document: the roster echoed, the ledger
// enumerated, the per-symbol evidence, and the verdict.
func (g CrossVenueGate) assemble(cfg GateConfig, ev GateEvidence) (driven.GateArtifactDoc, error) {
	weights := map[string]fixedpoint.Rate{}
	for _, a := range ProcCrossVenuePolicy {
		weights[a.Symbol] = a.Weight
	}
	params := driven.GateParametersDoc{
		MatchedFloor:    ProcCrossVenueMatchedFloor,
		RefusalCeiling:  ProcCrossVenueRefusalCeiling.Decimal(),
		VoidImbalance:   ProcCrossVenueVoidImbalance,
		SampleEvery:     ProcCrossVenueSample.Every.String(),
		SampleSlack:     ProcCrossVenueSample.Slack.String(),
		MaxGap:          ProcCrossVenueMaxGap.String(),
		ExitDivisor:     ProcCrossVenueExitDivisor,
		PairingRound:    ProcCrossVenueRound.String(),
		P90:             ProcCrossVenueP90.Decimal(),
		Period:          ProcCrossVenuePeriod.String(),
		MeanBlock:       ProcCrossVenueMeanBlock,
		Resamples:       ProcCrossVenueResamples,
		Confidence:      ProcCrossVenueConfidence.Decimal(),
		Seed:            ProcCrossVenueSeed,
		NotionalTarget:  ProcCrossVenueNotionalTarget,
		BinanceTakerFee: ev.BinanceTaker.Decimal(),
		BybitTakerFee:   ev.BybitTaker.Decimal(),
		Denominator:     ProcCrossVenueDenominator,
		Holdout:         ProcCrossVenueHoldout,
	}
	for _, s := range ProcCrossVenueSymbols {
		w, ok := weights[s.Symbol]
		if !ok {
			return driven.GateArtifactDoc{}, fmt.Errorf("policy holds no weight for %s: %w",
				s.Symbol, dcv.ErrPolicyInvalid)
		}
		params.Symbols = append(params.Symbols, driven.GateSymbolParamDoc{
			Symbol: s.Symbol,
			Entry:  s.Entry.Decimal(),
			Qty:    s.Qty.Decimal(),
			Weight: w.Decimal(),
		})
	}

	evidence := driven.GateEvidenceDoc{
		SettledCutoff: crossvenue.SettledCutoff.Format(time.RFC3339),
		BooksCutoff:   crossvenue.BooksCutoff.Format(time.RFC3339),
		Before:        ev.Before.UTC().Format(time.RFC3339),
	}
	for i, day := range ev.Report.Days {
		evidence.ArchiveDays = append(evidence.ArchiveDays, driven.GateArchiveDayDoc{
			Day: day, SHA256: ev.Report.DayHashes[i],
		})
	}
	// The settled ledger is deduplicated by path and ordered, so two
	// symbols sharing a store file cite it once and two runs over the same
	// corpus produce byte-identical ledgers.
	seenFile := map[string]bool{}
	var files []driven.LedgerEntryDoc
	for _, sr := range ev.Report.Symbols {
		for _, f := range sr.SettledFiles {
			if seenFile[f.Path] {
				continue
			}
			seenFile[f.Path] = true
			files = append(files, driven.LedgerEntryDoc{File: filepath.ToSlash(f.Path), SHA256: f.SHA256, Refusals: []string{}})
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].File < files[j].File })
	evidence.SettledFiles = files

	var symbols []driven.GateSymbolDoc
	for _, sr := range ev.Report.Symbols {
		annual, err := annualizedMean(ev.Series[sr.Symbol])
		if err != nil {
			return driven.GateArtifactDoc{}, fmt.Errorf("symbol %s: %w", sr.Symbol, err)
		}
		symbols = append(symbols, driven.GateSymbolDoc{
			Symbol:             sr.Symbol,
			MatchedSettlements: sr.Pairing.Matched,
			DecisionPoints:     sr.Merge.Joined,
			BinanceSkipped:     sr.BinanceSkipped,
			BybitSkipped:       sr.BybitSkipped,
			Episodes:           len(sr.Episodes),
			Scored:             sr.Scored,
			Refused:            sr.RefusedN,
			SealedExcluded:     sr.SealedExcluded,
			AnnualizedReturn:   annual.Decimal(),
		})
	}

	verdict, err := gateVerdict(ev)
	if err != nil {
		return driven.GateArtifactDoc{}, err
	}
	return driven.GateArtifactDoc{
		Schema: driven.GateArtifactSchema,
		Procedure: driven.GateProcedureDoc{
			Label:            ProcCrossVenueLabel,
			Commit:           ev.Revision,
			ExecutableSHA256: ev.ExecutableSHA256,
			ReferenceFile:    filepath.ToSlash(cfg.ReferencePath),
			ReferenceSHA256:  ev.ReferenceSHA256,
		},
		Parameters: params,
		Evidence:   evidence,
		Symbols:    symbols,
		Verdict:    &verdict,
	}, nil
}

// gateVerdict composes the portfolio statistic and compares it to the
// hurdle. Every step here is the pre-registered one: the declared policy,
// the declared bootstrap, the shared annualization, the shared hurdle
// netting.
func gateVerdict(ev GateEvidence) (driven.GateVerdictDoc, error) {
	rets, err := dcv.PortfolioReturns(ProcCrossVenuePolicy, ev.Series)
	if err != nil {
		return driven.GateVerdictDoc{}, fmt.Errorf("portfolio: %w", err)
	}
	lcb, err := dcv.StationaryBootstrapLCB(rets, ProcCrossVenueMeanBlock, ProcCrossVenueResamples,
		ProcCrossVenueConfidence, ProcCrossVenueSeed)
	if err != nil {
		return driven.GateVerdictDoc{}, fmt.Errorf("portfolio LCB: %w", err)
	}
	annual, err := cashcarry.Annualized(lcb, ProcCrossVenuePeriod)
	if err != nil {
		return driven.GateVerdictDoc{}, fmt.Errorf("annualize LCB: %w", err)
	}
	hurdle, err := cashcarry.EffectiveHurdle(ev.Hurdle)
	if err != nil {
		return driven.GateVerdictDoc{}, err
	}
	c, err := annual.Cmp(hurdle)
	if err != nil {
		return driven.GateVerdictDoc{}, fmt.Errorf("verdict comparison: %w", err)
	}
	return driven.GateVerdictDoc{
		Periods:         len(rets),
		PortfolioLCB:    lcb.Decimal(),
		AnnualizedLCB:   annual.Decimal(),
		Hurdle:          hurdle.Decimal(),
		HurdlePublished: ev.Hurdle.AnnualRate.Decimal(),
		HurdleAsOf:      ev.Hurdle.AsOf.UTC().Format(time.RFC3339),
		Clears:          c >= 0,
	}, nil
}

// annualizedMean is one series' mean per-period return on the year basis
// the hurdle is quoted on. The mean divides by the FULL period count,
// idle periods included — that is what makes the figure
// utilization-adjusted rather than an in-position rate, and it is why
// nothing downstream multiplies by a utilization fraction.
func annualizedMean(series []fixedpoint.Rate) (fixedpoint.Rate, error) {
	if len(series) == 0 {
		return fixedpoint.Rate{}, dcv.ErrNoReturns
	}
	sum := fixedpoint.ZeroRate()
	var err error
	for i, r := range series {
		if err = r.Validate(); err != nil {
			return fixedpoint.Rate{}, fmt.Errorf("period %d: %w", i, err)
		}
		if sum, err = sum.Add(r); err != nil {
			return fixedpoint.Rate{}, fmt.Errorf("period %d: %w", i, err)
		}
	}
	n := fixedpoint.RateFromMantissa(int64(len(series)) * 1_000_000_000)
	mean, err := sum.Div(n, fixedpoint.RoundHalfEven)
	if err != nil {
		return fixedpoint.Rate{}, err
	}
	return cashcarry.Annualized(mean, ProcCrossVenuePeriod)
}
