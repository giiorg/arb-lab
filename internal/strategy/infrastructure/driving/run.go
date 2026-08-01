package driving

// Calibrator.Run — §41f's derive-writes-an-artifact, real as of slice C.
// The §41b derivation order is the function's shape: costs and dispersion
// freeze first, the band freezes against the clearing level, and the
// occupancy replay consumes all of them. Every anti-tuning rule fires as
// a refusal on the way: H1 at every dataset row (the dataset doors), H2
// at every archive file (the family-2 collectors), H3 as the committed
// files' closed grids, §41e as the execution binding checked before any
// byte is derived and recorded inside the artifact after.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/giiorg/arb-lab/internal/engine/capture"
	"github.com/giiorg/arb-lab/internal/engine/venue"
	"github.com/giiorg/arb-lab/internal/platform/config"
	"github.com/giiorg/arb-lab/internal/platform/events"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/strategy/domain"
	"github.com/giiorg/arb-lab/internal/strategy/domain/calibrate"
	"github.com/giiorg/arb-lab/internal/strategy/infrastructure/driven"
)

// Run sentinels.
var (
	// ErrDriftTailDominated reports the imminent residual tail more
	// adverse than the EWMA tail at the drift quantile. EntryDrift is
	// pinned family 1 by the §41b roster, so a family-2-dominated tail
	// cannot be frozen without either understating the drift or lying
	// about its provenance — the governance's question, refused by name.
	ErrDriftTailDominated = fmt.Errorf("strategy/driving: imminent tail dominates the EWMA tail at the drift quantile")

	// ErrFrameUnpriced reports a surrogate frame whose declared stratum
	// and candidate have no surviving p90 cell — the frame's frozen cost
	// cannot be drawn from the artifact's own table.
	ErrFrameUnpriced = fmt.Errorf("strategy/driving: no p90 cell for the frame's declared stratum and candidate")

	// ErrClearingUnstable reports the clearing level moving when the
	// frozen band is substituted into the frame — the band-independence
	// of Enter is an invariant this run CHECKS rather than assumes.
	ErrClearingUnstable = fmt.Errorf("strategy/driving: clearing level moved when the band froze")
)

// RunConfig is the derivation run's world: every path explicit, the
// freeze instant an ARGUMENT (the injected-time rule — no wall clock in
// the output path).
type RunConfig struct {
	DatasetsDir   string
	ArchiveRoot   string
	ReferencePath string
	OutPath       string
	Freeze        time.Time
}

// Validate reports whether the run is fully specified.
func (c RunConfig) Validate() error {
	if c.DatasetsDir == "" || c.ArchiveRoot == "" || c.ReferencePath == "" || c.OutPath == "" {
		return fmt.Errorf("run config paths: %w", domain.ErrUnset)
	}
	if c.Freeze.IsZero() {
		return fmt.Errorf("run config freeze: %w", ErrInstantUnset)
	}
	return nil
}

// RunResult is what the governed act produced.
type RunResult struct {
	ArtifactPath string
	ArtifactID   string
	Hash         string
	Bytes        []byte
}

// Calibrator is the `arblab calibrate` command's core. The two seams
// exist so the battery can exercise the §41e refusals without rebuilding
// binaries: nil means the real thing.
type Calibrator struct {
	// ReadRevision overrides the embedded-build-info read; nil uses
	// ReadBuildRevision.
	ReadRevision func() (revision string, dirty bool, err error)

	// ExecutableDigest overrides the self-hash; nil hashes the running
	// executable's bytes.
	ExecutableDigest func() (string, error)
}

// The instruments and assets of the one calibrated pair.
var (
	calibSpotInst = ids.VenueInstrument{Venue: "binance", Product: ids.ProductSpot, Symbol: "BTCUSDT"}
	calibPerpInst = ids.VenueInstrument{
		Venue: "binance", Product: ids.ProductLinearPerp, Symbol: "BTCUSDT", Contract: ids.ContractLinear,
	}
)

const (
	calibBase  = fixedpoint.Asset("BTC")
	calibQuote = fixedpoint.Asset("USDT")
)

// Run executes the governed derivation and writes the artifact.
func (c Calibrator) Run(cfg RunConfig) (RunResult, error) {
	if err := cfg.Validate(); err != nil {
		return RunResult{}, err
	}

	// §41e first: nothing is derived from a build the artifact cannot
	// name. The recorded commit IS the embedded revision — recording
	// anything else is what CheckExecutionBinding exists to refuse.
	readRev := c.ReadRevision
	if readRev == nil {
		readRev = ReadBuildRevision
	}
	revision, dirty, err := readRev()
	if err != nil {
		return RunResult{}, err
	}
	if err := CheckExecutionBinding(revision, dirty, revision); err != nil {
		return RunResult{}, err
	}
	execDigest := c.ExecutableDigest
	if execDigest == nil {
		execDigest = selfDigest
	}
	exeSHA, err := execDigest()
	if err != nil {
		return RunResult{}, err
	}
	if err := driven.CheckHashText(exeSHA); err != nil {
		return RunResult{}, fmt.Errorf("executable digest: %w", err)
	}

	// The committed inputs: the three consumed family-1 identities.
	funding, err := loadDataset(cfg.DatasetsDir, "binance", SeriesFunding)
	if err != nil {
		return RunResult{}, err
	}
	spotKlines, err := loadDataset(cfg.DatasetsDir, "binance", SeriesSpotKline1h)
	if err != nil {
		return RunResult{}, err
	}
	markKlines, err := loadDataset(cfg.DatasetsDir, "binance", SeriesMarkKline1h)
	if err != nil {
		return RunResult{}, err
	}

	history, err := FundingSettlements(funding.file, calibPerpInst)
	if err != nil {
		return RunResult{}, fmt.Errorf("funding dataset: %w", err)
	}
	spotK, err := KlinePoints(spotKlines.file, calibBase, calibQuote)
	if err != nil {
		return RunResult{}, fmt.Errorf("spot klines: %w", err)
	}
	markK, err := KlinePoints(markKlines.file, calibBase, calibQuote)
	if err != nil {
		return RunResult{}, fmt.Errorf("mark klines: %w", err)
	}

	// The H2 universe and the family-2 measurements.
	days, err := LoadManifestDays(cfg.ArchiveRoot, cfg.Freeze)
	if err != nil {
		return RunResult{}, err
	}
	spotBooks, err := MeasureBooks(cfg.ArchiveRoot, days, BookStreamSpec{
		Key:        capture.StreamKey{Source: events.SourceID{Instrument: calibSpotInst, Stream: events.StreamBook}, Topic: "btcusdt@depth20@100ms"},
		Instrument: calibSpotInst,
		Decode: func(raw []byte) (venue.BinanceBookSnapshot, error) {
			return venue.DecodeBinanceSpotDepth20(raw, calibBase, calibQuote)
		},
	}, ProcFamily2)
	if err != nil {
		return RunResult{}, fmt.Errorf("spot books: %w", err)
	}
	perpBooks, err := MeasureBooks(cfg.ArchiveRoot, days, BookStreamSpec{
		Key:        capture.StreamKey{Source: events.SourceID{Instrument: calibPerpInst, Stream: events.StreamBook}, Topic: "btcusdt@depth20@100ms"},
		Instrument: calibPerpInst,
		Decode: func(raw []byte) (venue.BinanceBookSnapshot, error) {
			return venue.DecodeBinanceFuturesDepth20(raw, calibBase, calibQuote)
		},
	}, ProcFamily2)
	if err != nil {
		return RunResult{}, fmt.Errorf("perp books: %w", err)
	}
	imminent, err := MeasureImminent(cfg.ArchiveRoot, days,
		capture.StreamKey{Source: events.SourceID{Instrument: calibPerpInst, Stream: events.StreamFunding}, Topic: "btcusdt@markPrice@1s"},
		ProcImminent)
	if err != nil {
		return RunResult{}, fmt.Errorf("imminent: %w", err)
	}
	transport, err := LatencyStats(imminent.Latencies)
	if err != nil {
		return RunResult{}, fmt.Errorf("transport: %w", err)
	}

	// --- Costs freeze first (§41b's order). ---
	cells, dropped, err := freezeCells(spotBooks, perpBooks)
	if err != nil {
		return RunResult{}, err
	}
	moves := append(append([]fixedpoint.Rate{}, spotBooks.Moves...), perpBooks.Moves...)
	adverseFill, err := calibrate.UpperQuantile(moves, ProcMoveQuantile)
	if err != nil {
		return RunResult{}, fmt.Errorf("adverse fill move: %w", err)
	}
	if sign, err := adverseFill.Sign(); err != nil {
		return RunResult{}, err
	} else if sign < 0 {
		// An era whose 99th-percentile move was favorable freezes a
		// declared zero — the conservative model never credits a move,
		// mirroring the basis rule.
		adverseFill = fixedpoint.ZeroRate()
	}

	// --- The dispersion: EWMA residuals, imminent residuals, the merge. ---
	ewmaResiduals, err := calibrate.Residuals(history, ProcEWMAHalfLife)
	if err != nil {
		return RunResult{}, fmt.Errorf("ewma residuals: %w", err)
	}
	ewmaPoints, err := calibrate.Points(ewmaResiduals, ProcResidualGrid)
	if err != nil {
		return RunResult{}, fmt.Errorf("ewma points: %w", err)
	}
	immPoints, err := calibrate.Points(imminent.Residuals, ProcResidualGrid)
	if err != nil {
		return RunResult{}, fmt.Errorf("imminent points: %w", err)
	}
	merged, err := calibrate.MergeAdverse(ewmaPoints, immPoints)
	if err != nil {
		return RunResult{}, fmt.Errorf("merge: %w", err)
	}

	// EntryDrift from the FAMILY-1 tail (the roster's pinning), with the
	// procedure.go invariant CHECKED: if the merged tail is not the EWMA
	// tail, family 2 dominates and the pinning itself is in question.
	entryDrift, err := calibrate.EntryDriftFrom(ewmaPoints, ProcEntryDriftTail, ProcFrameInterval)
	if err != nil {
		return RunResult{}, fmt.Errorf("entry drift: %w", err)
	}
	if err := checkDriftTail(ewmaPoints, merged, ProcEntryDriftTail, ProcFrameInterval); err != nil {
		return RunResult{}, err
	}

	// --- The hold-scale basis move. ---
	basis, err := calibrate.AdverseBasisMove(spotK, markK, ProcBasisHorizon, ProcBasisQuantile)
	if err != nil {
		return RunResult{}, fmt.Errorf("adverse basis: %w", err)
	}

	// --- The committed reference rows. ---
	refData, err := os.ReadFile(cfg.ReferencePath)
	if err != nil {
		return RunResult{}, fmt.Errorf("reference: %w", err)
	}
	refStore, err := config.New[driven.Document](cfg.ReferencePath, cfg.Freeze, refData)
	if err != nil {
		return RunResult{}, fmt.Errorf("reference: %w", err)
	}
	ref, err := driven.NewReferenceStore(refStore)
	if err != nil {
		return RunResult{}, err
	}

	// --- The surrogate frame, the clearing level, the band, the replay. ---
	artifactID := strippedID(cfg.OutPath)
	// Derivation-time identity: the values are being derived BEFORE the
	// file whose hash would name them exists (a file cannot contain its
	// own digest). These refs live only inside the replay; the loader
	// re-mints every persisted ref from the emitted bytes at load.
	preRef := domain.CalibrationRef{ID: artifactID, Hash: "derivation-preimage"}

	frame, err := buildFrame(preRef, ref, cfg.Freeze, spotK, markK, merged, basis, entryDrift, cells, adverseFill)
	if err != nil {
		return RunResult{}, err
	}

	// The band needs the clearing level; Enter does not read the band, and
	// that independence is CHECKED below rather than assumed. The pre-band
	// frame carries the drift as a stand-in width.
	preFrame := frame
	preFrame.Params.ExitBand = domain.MeasuredRate{Value: entryDrift, Artifact: preRef}
	clearing, err := calibrate.ClearingLevel(preFrame)
	if err != nil {
		return RunResult{}, fmt.Errorf("clearing level: %w", err)
	}
	bandWidth, err := calibrate.ExitBandWidth(history, clearing, ProcExitPersist, ProcBandQuantile, ProcFrameInterval)
	if err != nil {
		return RunResult{}, fmt.Errorf("exit band: %w", err)
	}
	frame.Params.ExitBand = domain.MeasuredRate{Value: bandWidth, Artifact: preRef}
	clearingAfter, err := calibrate.ClearingLevel(frame)
	if err != nil {
		return RunResult{}, fmt.Errorf("clearing recheck: %w", err)
	}
	if clearing != clearingAfter {
		return RunResult{}, fmt.Errorf("pre-band %s, post-band %s: %w",
			clearing.Decimal(), clearingAfter.Decimal(), ErrClearingUnstable)
	}

	occ, err := calibrate.Occupancy(history, frame)
	if err != nil {
		return RunResult{}, fmt.Errorf("occupancy: %w", err)
	}
	minUtil, err := occ.Utilization.Mul(ProcMinUtilizationFactor, fixedpoint.RoundFloor)
	if err != nil {
		return RunResult{}, fmt.Errorf("min utilization: %w", err)
	}

	// --- Assemble, validate, emit. ---
	doc, err := assembleDoc(assembly{
		funding: funding, spotKlines: spotKlines, markKlines: markKlines,
		days: days, revision: revision, exeSHA: exeSHA,
		merged: merged, ewmaResidualCount: len(ewmaResiduals),
		entryDrift: entryDrift, bandWidth: bandWidth,
		basis: basis, utilization: occ.Utilization, minUtil: minUtil,
		cells: cells, droppedCells: dropped, adverseFill: adverseFill,
		transport: transport, imminent: imminent,
		spotBooks: spotBooks, perpBooks: perpBooks,
		freeze: cfg.Freeze,
	})
	if err != nil {
		return RunResult{}, err
	}
	if err := doc.Validate(); err != nil {
		return RunResult{}, fmt.Errorf("assembled artifact: %w", err)
	}

	// The emit-time re-hash (§41f): the recorded digests against the
	// files as they are NOW — a dataset edited mid-run must refuse, not
	// ship a digest for bytes nobody can reproduce.
	for _, h := range doc.Datasets.History {
		data, err := os.ReadFile(filepath.Join(cfg.DatasetsDir, "..", h.File))
		if err != nil {
			return RunResult{}, fmt.Errorf("re-hash %s: %w", h.File, err)
		}
		if err := CheckDatasetDigest(h.SHA256, data); err != nil {
			return RunResult{}, fmt.Errorf("re-hash %s: %w", h.File, err)
		}
	}

	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return RunResult{}, fmt.Errorf("marshal artifact: %w", err)
	}
	out = append(out, '\n')

	if err := GuardNewVersion(cfg.OutPath, func(p string) bool {
		_, statErr := os.Stat(p)
		return statErr == nil
	}); err != nil {
		return RunResult{}, err
	}
	if err := os.WriteFile(cfg.OutPath, out, 0o644); err != nil {
		return RunResult{}, fmt.Errorf("write artifact: %w", err)
	}

	// The written bytes must load through the production door.
	loaded, err := driven.LoadArtifactFile(cfg.OutPath)
	if err != nil {
		return RunResult{}, fmt.Errorf("emitted artifact does not load: %w", err)
	}
	return RunResult{
		ArtifactPath: cfg.OutPath,
		ArtifactID:   loaded.Ref.ID,
		Hash:         loaded.Ref.Hash,
		Bytes:        out,
	}, nil
}

// loadedDataset is one committed file with its exact bytes' digest.
type loadedDataset struct {
	file   DatasetFile
	rel    string
	sha256 string
}

// loadDataset reads the one committed file for (venue, series) by the
// fetcher's own naming convention.
func loadDataset(dir, venueName, series string) (loadedDataset, error) {
	pattern := filepath.Join(dir, venueName, series+".*.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return loadedDataset{}, fmt.Errorf("dataset glob: %w", err)
	}
	if len(matches) != 1 {
		return loadedDataset{}, fmt.Errorf("%s/%s: %d files match, want exactly one: %w",
			venueName, series, len(matches), ErrSeriesUnknown)
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		return loadedDataset{}, fmt.Errorf("dataset %s: %w", matches[0], err)
	}
	var f DatasetFile
	if err := json.Unmarshal(data, &f); err != nil {
		return loadedDataset{}, fmt.Errorf("dataset %s: %w: %w", matches[0], ErrPageMalformed, err)
	}
	sum := sha256.Sum256(data)
	rel := filepath.ToSlash(filepath.Join("datasets", venueName, filepath.Base(matches[0])))
	return loadedDataset{file: f, rel: rel, sha256: hex.EncodeToString(sum[:])}, nil
}

// selfDigest hashes the running executable.
func selfDigest() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("executable path: %w", err)
	}
	data, err := os.ReadFile(exe)
	if err != nil {
		return "", fmt.Errorf("executable bytes: %w", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// strippedID is §41d's identity rule, which driven owns: the artifact
// file's basename without extension. Delegated rather than restated —
// the attestation binds on this ID, so two spellings of it could refuse a
// correct approval.
func strippedID(path string) string { return driven.ArtifactIDFromPath(path) }

// checkDriftTail enforces procedure.go's family-pinning invariant: the
// DRIFT the family-1 tail freezes must not understate the drift the
// merged tail would have frozen. The comparison is between the two
// resulting margins, not the raw shifts — two non-negative tail shifts
// both freeze a zero drift and are equivalent however they order.
func checkDriftTail(ewma, merged []domain.QuantilePoint, tail fixedpoint.Rate, interval time.Duration) error {
	fromEwma, err := calibrate.EntryDriftFrom(ewma, tail, interval)
	if err != nil {
		return fmt.Errorf("drift check (family 1): %w", err)
	}
	fromMerged, err := calibrate.EntryDriftFrom(merged, tail, interval)
	if err != nil {
		return fmt.Errorf("drift check (merged): %w", err)
	}
	c, err := fromMerged.Cmp(fromEwma)
	if err != nil {
		return err
	}
	if c > 0 {
		return fmt.Errorf("family-1 drift %s, merged drift %s at q=%s: %w",
			fromEwma.Decimal(), fromMerged.Decimal(), tail, ErrDriftTailDominated)
	}
	return nil
}

// freezeCells applies the quantile selection and the MinEvidence rule to
// both streams' samples, in one place, and orders the table
// deterministically: venue, product, side, stratum, then MaxQty ascending
// (the Validate-required group order).
func freezeCells(streams ...BookMeasurements) ([]driven.CostCell, uint64, error) {
	var cells []driven.CostCell
	var dropped uint64
	for _, m := range streams {
		for key, samples := range m.Slips {
			if uint64(len(samples)) < ProcMinEvidence {
				dropped++
				continue
			}
			p90, err := calibrate.UpperQuantile(samples, ProcFillQuantile)
			if err != nil {
				return nil, 0, fmt.Errorf("cell %s %s %s: %w", key.Instrument, key.Side, key.Stratum, err)
			}
			if sign, err := p90.Sign(); err != nil {
				return nil, 0, err
			} else if sign < 0 {
				// A cell whose p90 is favorable freezes a declared zero:
				// the runtime refuses a negative charge outright.
				p90 = fixedpoint.ZeroRate()
			}
			cells = append(cells, driven.CostCell{
				Instrument:   key.Instrument,
				Side:         key.Side,
				Stratum:      key.Stratum,
				MaxQty:       key.MaxQty,
				P90:          p90,
				Observations: uint64(len(samples)),
				Method:       procMethodIdentity,
			})
		}
	}
	sort.Slice(cells, func(i, j int) bool {
		a, b := cells[i], cells[j]
		if a.Instrument != b.Instrument {
			if a.Instrument.Venue != b.Instrument.Venue {
				return a.Instrument.Venue < b.Instrument.Venue
			}
			return a.Instrument.Product < b.Instrument.Product
		}
		if a.Side != b.Side {
			return a.Side < b.Side
		}
		if a.Stratum != b.Stratum {
			return a.Stratum < b.Stratum
		}
		return a.MaxQty.Mantissa() < b.MaxQty.Mantissa()
	})
	return cells, dropped, nil
}

// procMethodIdentity names the committed observation per cell (§41b-ii-i).
const procMethodIdentity = "cfwalk-v1: classify book@t (frozen classifier), walk candidate through same side@t+250ms, measure vs best@t; recv-anchored 10s epoch grid, 1s pair slack; gap-spanning pairs discarded; depth-short excluded and counted"

// frameCellP90 selects the WORSE side's p90 for the frame's declared
// stratum and candidate — the frame charges one conservative number for
// both directions.
func frameCellP90(inst ids.VenueInstrument, cells []driven.CostCell) (fixedpoint.Rate, error) {
	var best fixedpoint.Rate
	found := false
	for _, c := range cells {
		if c.Instrument != inst || c.Stratum != ProcSurrogateStratum || c.MaxQty != ProcCandidateQty {
			continue
		}
		if !found {
			best = c.P90
			found = true
			continue
		}
		cmp, err := c.P90.Cmp(best)
		if err != nil {
			return fixedpoint.Rate{}, err
		}
		if cmp > 0 {
			best = c.P90
		}
	}
	if !found {
		return fixedpoint.Rate{}, fmt.Errorf("%s %s %s: %w",
			inst, ProcSurrogateStratum, ProcCandidateQty, ErrFrameUnpriced)
	}
	return best, nil
}

// buildFrame assembles the surrogate: era-anchored frame prices (the
// committed era's FIRST klines — a stated fixed frame, §41n reading ii),
// the frozen reference rows, the merged dispersion, and the frame cost
// drawn from the artifact's own frozen numbers.
func buildFrame(preRef domain.CalibrationRef, ref *driven.ReferenceStore, freeze time.Time,
	spotK, markK []calibrate.KlinePoint, merged []domain.QuantilePoint,
	basis, entryDrift fixedpoint.Rate, cells []driven.CostCell, adverseFill fixedpoint.Rate,
) (calibrate.Surrogate, error) {
	spotFees, err := ref.Fees(calibSpotInst, freeze)
	if err != nil {
		return calibrate.Surrogate{}, fmt.Errorf("spot fees: %w", err)
	}
	perpFees, err := ref.Fees(calibPerpInst, freeze)
	if err != nil {
		return calibrate.Surrogate{}, fmt.Errorf("perp fees: %w", err)
	}
	spotLots, err := ref.LotRules(calibSpotInst, freeze)
	if err != nil {
		return calibrate.Surrogate{}, fmt.Errorf("spot lots: %w", err)
	}
	perpLots, err := ref.LotRules(calibPerpInst, freeze)
	if err != nil {
		return calibrate.Surrogate{}, fmt.Errorf("perp lots: %w", err)
	}
	hurdle, err := ref.Hurdle(freeze)
	if err != nil {
		return calibrate.Surrogate{}, fmt.Errorf("hurdle: %w", err)
	}

	money := func(s string) (fixedpoint.Money, error) { return fixedpoint.ParseMoney(calibQuote, s) }
	spotPrincipal, err := money(ProcCapitalSpotPrincipal)
	if err != nil {
		return calibrate.Surrogate{}, err
	}
	perpMargin, err := money(ProcCapitalPerpMargin)
	if err != nil {
		return calibrate.Surrogate{}, err
	}
	rescue, err := money(ProcCapitalRescue)
	if err != nil {
		return calibrate.Surrogate{}, err
	}
	reserve, err := money(ProcCapitalReserve)
	if err != nil {
		return calibrate.Surrogate{}, err
	}
	zero := fixedpoint.ZeroMoney(calibQuote)

	// The frame's execution cost: per leg, the era-anchored notional times
	// (worse-side p90 + the frozen adverse fill move), away from zero —
	// charge arithmetic, the runtime model's own composition held constant.
	frameCost := func(inst ids.VenueInstrument, price fixedpoint.Price) (fixedpoint.Money, error) {
		p90, err := frameCellP90(inst, cells)
		if err != nil {
			return fixedpoint.Money{}, err
		}
		charge, err := p90.Add(adverseFill)
		if err != nil {
			return fixedpoint.Money{}, err
		}
		notional, err := fixedpoint.Notional(price, ProcCandidateQty, fixedpoint.RoundAwayFromZero)
		if err != nil {
			return fixedpoint.Money{}, err
		}
		return notional.Mul(charge, fixedpoint.RoundAwayFromZero)
	}
	spotCost, err := frameCost(calibSpotInst, spotK[0].Price)
	if err != nil {
		return calibrate.Surrogate{}, fmt.Errorf("frame spot cost: %w", err)
	}
	perpCost, err := frameCost(calibPerpInst, markK[0].Price)
	if err != nil {
		return calibrate.Surrogate{}, fmt.Errorf("frame perp cost: %w", err)
	}

	policyRev := config.Revision{
		Num:      1,
		Hash:     sha256.Sum256([]byte("arblab-calibration-procedure-v1")),
		Source:   "internal/strategy/infrastructure/driving/procedure.go",
		LoadedAt: freeze,
	}

	return calibrate.Surrogate{
		Spot:      calibSpotInst,
		Perp:      calibPerpInst,
		SpotPrice: spotK[0].Price,
		PerpPrice: markK[0].Price,
		Interval:  ProcFrameInterval,
		Cap:       ProcFrameCap,
		Floor:     ProcFrameFloor,
		Params: domain.StrategyParams{
			MinFundingAPR:      ProcMinFundingAPR,
			MinHoldSettlements: ProcMinHoldSettlements,
			ForecastModel:      "ewma.v1",
			EWMAHalfLife:       ProcEWMAHalfLife,
			DecisionQuantile:   ProcDecisionQuantile,
			Aggregation:        domain.AggregationConservative,
			BasisModel:         "conservative.v1",
			EntryDrift:         domain.MeasuredRate{Value: entryDrift, Artifact: preRef},
			// ExitBand is substituted by the caller: first the stand-in,
			// then the frozen width — the ClearingLevel invariant checks
			// the substitution changed nothing Enter reads.
			ExitBand:     domain.MeasuredRate{Value: entryDrift, Artifact: preRef},
			ExitPersist:  ProcExitPersist,
			PerpLeverage: ProcPerpLeverage,
		},
		PolicyRev: policyRev,
		Dispersion: domain.Dispersion{
			Points:   merged,
			Artifact: preRef,
		},
		AdverseBasis: domain.MeasuredRate{Value: basis, Artifact: preRef},
		SpotFees:     spotFees,
		PerpFees:     perpFees,
		SpotLots:     spotLots,
		PerpLots:     perpLots,
		Hurdle:       hurdle,
		Capital: domain.CapitalAllocation{
			SpotPrincipal:      spotPrincipal,
			PerpInitialMargin:  perpMargin,
			RescueMargin:       domain.MeasuredMoney{Amount: rescue, Artifact: preRef},
			FeeSlippageReserve: reserve,
			CommittedFloat:     zero,
		},
		CandidateQty:      ProcCandidateQty,
		ResidualTolerance: procQty("0"),
		ExecCost: domain.ExecutionCostEstimate{
			SpotCost:    spotCost,
			PerpCost:    perpCost,
			Model:       "surrogate.v1",
			Calibration: preRef,
			Tactic:      domain.TacticTakerIOC,
			Qty:         ProcCandidateQty,
		},
		Opportunity:      zero,
		ClosingCostDelta: zero,
		RiskDelta:        zero,
	}, nil
}
